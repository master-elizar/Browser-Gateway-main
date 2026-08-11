package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type otxProvider struct{ s *Service }

func (otxProvider) ID() string { return "otx" }

func (p otxProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	section := ""
	switch kind {
	case KindDomain:
		section = "domain"
	case KindIP:
		section = "IPv4"
		if looksIPv6(indicator) {
			section = "IPv6"
		}
	case KindURL:
		section = "url"
	default:
		return nil, ErrSkip
	}
	raw := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/%s/%s/general", section, url.PathEscape(indicator))
	code, body, err := p.s.doJSON(ctx, http.MethodGet, raw, nil, map[string]string{
		"X-OTX-API-KEY": apiKey,
	})
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if code == http.StatusNotFound {
		return &Result{
			Provider:  "otx",
			Kind:      string(kind),
			Indicator: indicator,
			Verdict:   "unknown",
			Permalink: "https://otx.alienvault.com/",
		}, nil
	}
	if code >= 300 {
		return nil, fmt.Errorf("otx http %d: %s", code, truncate(string(body), 180))
	}

	var payload struct {
		PulseInfo struct {
			Count int `json:"count"`
		} `json:"pulse_info"`
		Validation []any `json:"validation"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	res := &Result{
		Provider:  "otx",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: fmt.Sprintf("https://otx.alienvault.com/indicator/%s/%s", section, url.PathEscape(indicator)),
	}
	n := payload.PulseInfo.Count
	switch {
	case n >= 3:
		res.Verdict = "malicious"
		res.Malicious = n
	case n >= 1:
		res.Verdict = "suspicious"
		res.Suspicious = n
	default:
		res.Verdict = "clean"
		res.Harmless = 1
	}
	return res, nil
}

func looksIPv6(s string) bool {
	return len(s) > 0 && (containsColon(s))
}

func containsColon(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return true
		}
	}
	return false
}
