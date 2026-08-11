package ti

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db     *gorm.DB
	client *http.Client

	vtMu     sync.Mutex
	vtLast   time.Time
	vtMinGap time.Duration
}

func New(db *gorm.DB) *Service {
	return &Service{
		db: db,
		client: &http.Client{
			Timeout: 12 * time.Second,
		},
		vtMinGap: 16 * time.Second,
	}
}

func NormalizeKind(kind, value string) (Kind, string, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return "", "", fmt.Errorf("%w: empty value", ErrUnsupported)
	}
	k := Kind(strings.ToLower(strings.TrimSpace(kind)))
	switch k {
	case KindDomain, KindIP, KindURL:
		// ok
	case "":
		if strings.Contains(v, "://") || strings.HasPrefix(strings.ToLower(v), "www.") {
			k = KindURL
		} else if ip := net.ParseIP(strings.Trim(v, "[]")); ip != nil {
			k = KindIP
		} else {
			k = KindDomain
		}
	default:
		return "", "", fmt.Errorf("%w: %s", ErrUnsupported, kind)
	}
	if k == KindURL && !strings.Contains(v, "://") {
		v = "http://" + v
	}
	if k == KindDomain {
		v = strings.TrimSuffix(strings.ToLower(v), ".")
		if h, err := url.Parse("http://" + v); err == nil && h.Hostname() != "" {
			v = h.Hostname()
		}
	}
	if k == KindIP {
		v = strings.Trim(v, "[]")
	}
	return k, v, nil
}

type enabledProvider struct {
	p      providerLookup
	apiKey string
}

func (s *Service) enabledProviders(settings domain.AppSettings) []enabledProvider {
	out := make([]enabledProvider, 0, 6)
	if settings.TiVirusTotalEnabled && strings.TrimSpace(settings.TiAPIKey) != "" {
		out = append(out, enabledProvider{p: virusTotalProvider{s: s}, apiKey: strings.TrimSpace(settings.TiAPIKey)})
	}
	if settings.TiURLHausEnabled {
		out = append(out, enabledProvider{p: urlhausProvider{s: s}})
	}
	if settings.TiThreatFoxEnabled {
		out = append(out, enabledProvider{p: threatFoxProvider{s: s}, apiKey: strings.TrimSpace(settings.TiThreatFoxAPIKey)})
	}
	if settings.TiAbuseIPDBEnabled && strings.TrimSpace(settings.TiAbuseIPDBAPIKey) != "" {
		out = append(out, enabledProvider{p: abuseIPDBProvider{s: s}, apiKey: strings.TrimSpace(settings.TiAbuseIPDBAPIKey)})
	}
	if settings.TiOTXEnabled && strings.TrimSpace(settings.TiOTXAPIKey) != "" {
		out = append(out, enabledProvider{p: otxProvider{s: s}, apiKey: strings.TrimSpace(settings.TiOTXAPIKey)})
	}
	if settings.TiSpamhausEnabled {
		out = append(out, enabledProvider{p: spamhausProvider{}})
	}
	return out
}

func (s *Service) Lookup(ctx context.Context, settings domain.AppSettings, kind, value string) (*Result, error) {
	if !settings.TiEnabled {
		return nil, ErrDisabled
	}
	k, indicator, err := NormalizeKind(kind, value)
	if err != nil {
		return nil, err
	}

	providers := s.enabledProviders(settings)
	// Stage 13 compat: only VT key configured, flags not backfilled yet.
	if len(providers) == 0 && strings.TrimSpace(settings.TiAPIKey) != "" {
		providers = []enabledProvider{{p: virusTotalProvider{s: s}, apiKey: strings.TrimSpace(settings.TiAPIKey)}}
	}
	if len(providers) == 0 {
		return nil, ErrNoProviders
	}

	type slot struct {
		res *Result
	}
	slots := make([]slot, len(providers))
	var wg sync.WaitGroup
	for i, ep := range providers {
		wg.Add(1)
		go func(i int, ep enabledProvider) {
			defer wg.Done()
			if cached, ok := s.fromCache(ep.p.ID(), string(k), indicator); ok {
				cached.Cached = true
				slots[i].res = cached
				return
			}
			res, err := ep.p.Lookup(ctx, k, indicator, ep.apiKey)
			if err != nil {
				if errors.Is(err, ErrSkip) {
					return
				}
				slots[i].res = &Result{
					Provider:  ep.p.ID(),
					Kind:      string(k),
					Indicator: indicator,
					Verdict:   "unknown",
					Error:     err.Error(),
					CheckedAt: time.Now().UTC(),
				}
				return
			}
			if res == nil {
				return
			}
			s.saveCache(res)
			slots[i].res = res
		}(i, ep)
	}
	wg.Wait()

	parts := make([]Result, 0, len(slots))
	for _, sl := range slots {
		if sl.res != nil {
			parts = append(parts, *sl.res)
		}
	}
	if len(parts) == 0 {
		return &Result{
			Provider:  "multi",
			Kind:      string(k),
			Indicator: indicator,
			Verdict:   "unknown",
			CheckedAt: time.Now().UTC(),
			Providers: []Result{},
		}, nil
	}
	return aggregate(string(k), indicator, parts), nil
}

func (s *Service) LookupMany(ctx context.Context, settings domain.AppSettings, values []string) []Result {
	out := make([]Result, 0, len(values))
	seen := map[string]struct{}{}
	for _, raw := range values {
		k, ind, err := NormalizeKind("", raw)
		if err != nil {
			continue
		}
		key := string(k) + "|" + ind
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		r, err := s.Lookup(ctx, settings, string(k), ind)
		if err != nil {
			out = append(out, Result{
				Provider:  "multi",
				Kind:      string(k),
				Indicator: ind,
				Verdict:   "unknown",
				Error:     err.Error(),
				CheckedAt: time.Now().UTC(),
			})
			continue
		}
		out = append(out, *r)
	}
	return out
}

func aggregate(kind, indicator string, parts []Result) *Result {
	agg := &Result{
		Provider:  "multi",
		Kind:      kind,
		Indicator: indicator,
		CheckedAt: time.Now().UTC(),
		Providers: parts,
	}
	anyCached := true
	for _, p := range parts {
		if !p.Cached {
			anyCached = false
		}
		if p.Error != "" {
			continue
		}
		switch p.Verdict {
		case "malicious":
			agg.Malicious++
		case "suspicious":
			agg.Suspicious++
		case "clean":
			agg.Harmless++
		default:
			agg.Undetected++
		}
	}
	agg.Cached = anyCached && len(parts) > 0
	switch {
	case agg.Malicious > 0:
		agg.Verdict = "malicious"
	case agg.Suspicious > 0:
		agg.Verdict = "suspicious"
	case agg.Harmless > 0:
		agg.Verdict = "clean"
	default:
		agg.Verdict = "unknown"
	}
	// Prefer a permalink from the first malicious/suspicious hit.
	for _, p := range parts {
		if (p.Verdict == "malicious" || p.Verdict == "suspicious") && p.Permalink != "" {
			agg.Permalink = p.Permalink
			break
		}
	}
	if agg.Permalink == "" {
		for _, p := range parts {
			if p.Permalink != "" {
				agg.Permalink = p.Permalink
				break
			}
		}
	}
	return agg
}

func (s *Service) waitVTRate(ctx context.Context) error {
	s.vtMu.Lock()
	defer s.vtMu.Unlock()
	wait := s.vtMinGap - time.Since(s.vtLast)
	if wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	s.vtLast = time.Now()
	return nil
}

func (s *Service) fromCache(provider, kind, indicator string) (*Result, bool) {
	var row domain.TICacheEntry
	err := s.db.Where("provider = ? AND kind = ? AND indicator = ? AND expires_at > ?", provider, kind, indicator, time.Now()).
		First(&row).Error
	if err != nil {
		return nil, false
	}
	return &Result{
		Provider:   row.Provider,
		Kind:       row.Kind,
		Indicator:  row.Indicator,
		Verdict:    row.Verdict,
		Malicious:  row.Malicious,
		Suspicious: row.Suspicious,
		Harmless:   row.Harmless,
		Undetected: row.Undetected,
		Permalink:  row.Permalink,
		CheckedAt:  row.CheckedAt,
	}, true
}

func (s *Service) saveCache(r *Result) {
	if r == nil || r.Provider == "multi" || r.Error != "" {
		return
	}
	now := time.Now().UTC()
	row := domain.TICacheEntry{
		ID:         uuid.NewString(),
		Provider:   r.Provider,
		Kind:       r.Kind,
		Indicator:  r.Indicator,
		Verdict:    r.Verdict,
		Malicious:  r.Malicious,
		Suspicious: r.Suspicious,
		Harmless:   r.Harmless,
		Undetected: r.Undetected,
		Permalink:  r.Permalink,
		CheckedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	_ = s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "provider"}, {Name: "kind"}, {Name: "indicator"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"verdict", "malicious", "suspicious", "harmless", "undetected", "permalink", "checked_at", "expires_at",
		}),
	}).Create(&row).Error
	r.CheckedAt = now
}

func (s *Service) doJSON(ctx context.Context, method, rawURL string, body io.Reader, headers map[string]string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	return resp.StatusCode, b, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func FingerprintAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:4])
}

func MaskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "••••••••"
	}
	return key[:4] + strings.Repeat("•", 8) + key[len(key)-4:]
}

func IsMaskedKey(key string) bool {
	key = strings.TrimSpace(key)
	return key == "" || strings.Contains(key, "•") || strings.Contains(key, "*")
}
