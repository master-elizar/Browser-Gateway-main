package handlers

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
)

type domainCheckReq struct {
	Value string `json:"value"`
	Kind  string `json:"kind,omitempty"`
}

type domainCheckResp struct {
	Kind      string        `json:"kind"`
	Indicator string        `json:"indicator"`
	// Tier is the simple-mode verdict: safe | suspicious | malicious | unknown ("unknown"
	// only when Total is 0 -- no source could be queried at all, not the same as "safe").
	Tier      string        `json:"tier"`
	Malicious int           `json:"malicious"` // M -- providers whose own verdict was "malicious"
	Total     int           `json:"total"`      // N -- providers successfully queried (excludes no-key-skipped, errored, informational)
	Providers []ti.Result   `json:"providers"`   // every provider actually queried, including informational/errored, for advanced mode
	Whois     *ti.WhoisInfo `json:"whois,omitempty"`
	WhoisError string       `json:"whoisError,omitempty"`
	Cached    bool          `json:"cached"`
	CheckedAt time.Time     `json:"checkedAt"`
}

// domainCheckCacheTTL is deliberately much shorter than ti.Service's own 24h per-provider DB
// cache -- this caches the whole assembled tab response (including the WHOIS/RDAP call,
// which has no cache of its own) just long enough to absorb someone re-checking the same
// value a few times in a row, without serving hour-old-feeling results.
const domainCheckCacheTTL = 12 * time.Minute

type domainCheckCacheEntry struct {
	resp    domainCheckResp
	expires time.Time
}

var (
	domainCheckCacheMu     sync.Mutex
	domainCheckCache       = map[string]domainCheckCacheEntry{}
	domainCheckCacheWrites int
)

// domainCheckCacheKey includes userID because personal API keys (checkpoint 2) can change
// which providers actually run for a given user, so two users checking the same domain can
// legitimately get different Total/Providers.
func domainCheckCacheKey(userID, kind, indicator string) string {
	return userID + "|" + kind + "|" + indicator
}

// TICheck powers the domain-check tab: runs the indicator through every enabled provider
// (via the same ti.Service.Lookup used elsewhere), fetches WHOIS/RDAP for domains, and
// reduces the result to a three-tier verdict alongside the full per-provider detail needed
// for the tab's advanced view.
func (h *Handler) TICheck(c *fiber.Ctx) error {
	if h.ti == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "threat intelligence unavailable")
	}
	var req domainCheckReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if strings.TrimSpace(req.Value) == "" {
		return fiber.NewError(fiber.StatusBadRequest, "value required")
	}
	k, indicator, err := ti.NormalizeKind(req.Kind, req.Value)
	if err != nil {
		return mapTIErr(err)
	}

	actor := auth.CurrentUser(c)
	userID := ""
	if actor != nil {
		userID = actor.ID
	}

	cacheKey := domainCheckCacheKey(userID, string(k), indicator)
	domainCheckCacheMu.Lock()
	entry, hit := domainCheckCache[cacheKey]
	domainCheckCacheMu.Unlock()
	if hit && time.Now().Before(entry.expires) {
		resp := entry.resp
		resp.Cached = true
		return c.JSON(resp)
	}

	st := h.loadSettings()
	ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
	defer cancel()
	res, err := h.ti.Lookup(ctx, st, userID, string(k), indicator)
	if err != nil {
		return mapTIErr(err)
	}

	// aggregate() in ti/service.go already excludes errored and Informational providers from
	// these four counts, and Malicious/Suspicious/Harmless/Undetected each count providers
	// (one increment per provider), not raw per-provider detection numbers -- so this sum is
	// exactly N (providers that were actually queried and returned some verdict), and
	// res.Malicious is exactly M.
	total := res.Malicious + res.Suspicious + res.Harmless + res.Undetected
	resp := domainCheckResp{
		Kind:      res.Kind,
		Indicator: res.Indicator,
		Malicious: res.Malicious,
		Total:     total,
		Providers: res.Providers,
		CheckedAt: res.CheckedAt,
		Tier:      domainCheckTier(res.Malicious, res.Suspicious, total),
	}

	if k == ti.KindDomain {
		whoisCtx, whoisCancel := context.WithTimeout(c.Context(), 10*time.Second)
		whois, werr := h.ti.LookupRDAP(whoisCtx, indicator)
		whoisCancel()
		if werr != nil {
			resp.WhoisError = werr.Error()
		} else {
			resp.Whois = whois
		}
	}

	domainCheckCacheMu.Lock()
	domainCheckCache[cacheKey] = domainCheckCacheEntry{resp: resp, expires: time.Now().Add(domainCheckCacheTTL)}
	domainCheckCacheWrites++
	// No background goroutine for this -- piggyback a sweep of expired entries onto every
	// 200th write so the map can't grow unbounded over a long uptime, without needing a
	// ticker/lifecycle to manage.
	if domainCheckCacheWrites%200 == 0 {
		now := time.Now()
		for key, v := range domainCheckCache {
			if now.After(v.expires) {
				delete(domainCheckCache, key)
			}
		}
	}
	domainCheckCacheMu.Unlock()

	if actor != nil {
		_ = h.auth.WriteAudit(actor.ID, "ti.check", resp.Kind+":"+resp.Indicator+" → "+resp.Tier)
	}
	return c.JSON(resp)
}

// domainCheckTier reduces (M malicious-verdict providers, S suspicious-verdict providers,
// N total successfully-queried providers) to the tab's three-tier simple-mode verdict.
// Rationale (from the approved plan): a flat "X% of sources" ratio alone is misleading
// because N varies a lot per deployment/user (how many API keys are configured), so a small
// number of hits from an otherwise-large N would round down to "safe" even though 2-3
// independent confirmations is a real signal on its own -- hence the M>=3 absolute
// threshold alongside the ratio one.
func domainCheckTier(malicious, suspicious, total int) string {
	if total == 0 {
		return "unknown"
	}
	ratio := float64(malicious) / float64(total)
	switch {
	case malicious >= 3 || ratio > 0.20:
		return "malicious"
	case malicious > 0 || suspicious >= 2:
		return "suspicious"
	default:
		return "safe"
	}
}
