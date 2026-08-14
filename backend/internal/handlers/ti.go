package handlers

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
)

type tiLookupReq struct {
	Kind      string `json:"kind"`
	Value     string `json:"value"`
	SessionID string `json:"sessionId,omitempty"`
}

type tiEnrichReq struct {
	Values []string `json:"values"`
}

type settingsView struct {
	domain.AppSettings
	TiAPIKeySet                   bool   `json:"tiApiKeySet"`
	TiAPIKey                      string `json:"tiApiKey,omitempty"`
	TiThreatFoxAPIKeySet          bool   `json:"tiThreatfoxApiKeySet"`
	TiThreatFoxAPIKey             string `json:"tiThreatfoxApiKey,omitempty"`
	TiAbuseIPDBAPIKeySet          bool   `json:"tiAbuseipdbApiKeySet"`
	TiAbuseIPDBAPIKey             string `json:"tiAbuseipdbApiKey,omitempty"`
	TiOTXAPIKeySet                bool   `json:"tiOtxApiKeySet"`
	TiOTXAPIKey                   string `json:"tiOtxApiKey,omitempty"`
	TiShodanAPIKeySet              bool   `json:"tiShodanApiKeySet"`
	TiShodanAPIKey                 string `json:"tiShodanApiKey,omitempty"`
	TiSafeBrowsingAPIKeySet        bool   `json:"tiSafebrowsingApiKeySet"`
	TiSafeBrowsingAPIKey           string `json:"tiSafebrowsingApiKey,omitempty"`
	TiMalwareBazaarAPIKeySet       bool   `json:"tiMalwarebazaarApiKeySet"`
	TiMalwareBazaarAPIKey          string `json:"tiMalwarebazaarApiKey,omitempty"`
	DownloadZipPasswordDefaultSet bool   `json:"downloadZipPasswordDefaultSet"`
	DownloadZipPasswordDefault    string `json:"downloadZipPasswordDefault,omitempty"`
}

func (h *Handler) settingsDTO(s domain.AppSettings) settingsView {
	v := settingsView{AppSettings: s}
	v.AppSettings.TiAPIKey = ""
	v.AppSettings.TiThreatFoxAPIKey = ""
	v.AppSettings.TiAbuseIPDBAPIKey = ""
	v.AppSettings.TiOTXAPIKey = ""
	v.AppSettings.TiShodanAPIKey = ""
	v.AppSettings.TiSafeBrowsingAPIKey = ""
	v.AppSettings.TiMalwareBazaarAPIKey = ""
	v.AppSettings.DownloadZipPasswordDefault = ""

	if strings.TrimSpace(s.TiAPIKey) != "" {
		v.TiAPIKeySet = true
		v.TiAPIKey = ti.MaskAPIKey(s.TiAPIKey)
	}
	if strings.TrimSpace(s.TiThreatFoxAPIKey) != "" {
		v.TiThreatFoxAPIKeySet = true
		v.TiThreatFoxAPIKey = ti.MaskAPIKey(s.TiThreatFoxAPIKey)
	}
	if strings.TrimSpace(s.TiAbuseIPDBAPIKey) != "" {
		v.TiAbuseIPDBAPIKeySet = true
		v.TiAbuseIPDBAPIKey = ti.MaskAPIKey(s.TiAbuseIPDBAPIKey)
	}
	if strings.TrimSpace(s.TiOTXAPIKey) != "" {
		v.TiOTXAPIKeySet = true
		v.TiOTXAPIKey = ti.MaskAPIKey(s.TiOTXAPIKey)
	}
	if strings.TrimSpace(s.TiShodanAPIKey) != "" {
		v.TiShodanAPIKeySet = true
		v.TiShodanAPIKey = ti.MaskAPIKey(s.TiShodanAPIKey)
	}
	if strings.TrimSpace(s.TiSafeBrowsingAPIKey) != "" {
		v.TiSafeBrowsingAPIKeySet = true
		v.TiSafeBrowsingAPIKey = ti.MaskAPIKey(s.TiSafeBrowsingAPIKey)
	}
	if strings.TrimSpace(s.TiMalwareBazaarAPIKey) != "" {
		v.TiMalwareBazaarAPIKeySet = true
		v.TiMalwareBazaarAPIKey = ti.MaskAPIKey(s.TiMalwareBazaarAPIKey)
	}
	if strings.TrimSpace(s.DownloadZipPasswordDefault) != "" {
		v.DownloadZipPasswordDefaultSet = true
		v.DownloadZipPasswordDefault = ti.MaskAPIKey(s.DownloadZipPasswordDefault)
	}
	return v
}

func (h *Handler) loadSettings() domain.AppSettings {
	var s domain.AppSettings
	_ = h.st.DB.First(&s).Error
	return s
}

func (h *Handler) TILookup(c *fiber.Ctx) error {
	if h.ti == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "threat intelligence unavailable")
	}
	var req tiLookupReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	st := h.loadSettings()
	actor := auth.CurrentUser(c)
	actorID := ""
	if actor != nil {
		actorID = actor.ID
	}
	ctx, cancel := context.WithTimeout(c.Context(), 45*time.Second)
	defer cancel()
	res, err := h.ti.Lookup(ctx, st, actorID, req.Kind, req.Value)
	if err != nil {
		return mapTIErr(err)
	}
	if actor != nil {
		_ = h.auth.WriteAudit(actor.ID, "ti.lookup", res.Kind+":"+res.Indicator+" → "+res.Verdict)
		if sid := strings.TrimSpace(req.SessionID); sid != "" {
			if _, err := h.sessions.Get(actor, sid); err == nil {
				h.persistTIResult(sid, *res)
			}
		}
	}
	return c.JSON(res)
}

func (h *Handler) BrowserNetworkEnrich(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerNetworkEnabled, "network"); err != nil {
		return err
	}
	if h.ti == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "threat intelligence unavailable")
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	var req tiEnrichReq
	_ = c.BodyParser(&req)
	values := req.Values
	if len(values) == 0 {
		var rows []domain.NetworkEvent
		_ = h.st.DB.Where("session_id = ?", id).Order("created_at desc").Limit(80).Find(&rows).Error
		seen := map[string]struct{}{}
		for _, row := range rows {
			for _, v := range extractIndicators(row.Payload) {
				if _, ok := seen[v]; ok {
					continue
				}
				seen[v] = struct{}{}
				values = append(values, v)
				if len(values) >= 12 {
					break
				}
			}
			if len(values) >= 12 {
				break
			}
		}
	}
	if len(values) > 12 {
		values = values[:12]
	}
	st := h.loadSettings()
	ctx, cancel := context.WithTimeout(c.Context(), 120*time.Second)
	defer cancel()
	results := h.ti.LookupMany(ctx, st, user.ID, values)
	for _, r := range results {
		h.persistTIResult(id, r)
	}
	_ = h.auth.WriteAudit(user.ID, "ti.enrich", "session "+id+" indicators="+strconv.Itoa(len(results)))
	return c.JSON(fiber.Map{"items": results, "total": len(results)})
}

func extractIndicators(payloadJSON string) []string {
	out := []string{}
	if i := strings.Index(payloadJSON, `"url"`); i >= 0 {
		if v := jsonStringAfter(payloadJSON, i); v != "" {
			if u, err := url.Parse(v); err == nil && u.Hostname() != "" {
				out = append(out, u.Hostname())
			}
		}
	}
	if i := strings.Index(payloadJSON, `"query"`); i >= 0 {
		if v := jsonStringAfter(payloadJSON, i); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func jsonStringAfter(s string, from int) string {
	rest := s[from:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t\n\r")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func mapTIErr(err error) error {
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), ti.ErrDisabled.Error()):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case strings.Contains(err.Error(), ti.ErrNoProviders.Error()):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), ti.ErrNoAPIKey.Error()):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), ti.ErrRateLimited.Error()):
		return fiber.NewError(fiber.StatusTooManyRequests, err.Error())
	case strings.Contains(err.Error(), ti.ErrUnsupported.Error()):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case strings.Contains(err.Error(), ti.ErrInvalidIndicator.Error()):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	default:
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}
}
