package ti

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type virusTotalProvider struct{ s *Service }

func (virusTotalProvider) ID() string { return "virustotal" }

func (p virusTotalProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	if err := p.s.waitVTRate(ctx); err != nil {
		return nil, err
	}
	path := ""
	permalink := ""
	switch kind {
	case KindDomain:
		path = "https://www.virustotal.com/api/v3/domains/" + url.PathEscape(indicator)
		permalink = "https://www.virustotal.com/gui/domain/" + url.PathEscape(indicator)
	case KindIP:
		path = "https://www.virustotal.com/api/v3/ip_addresses/" + url.PathEscape(indicator)
		permalink = "https://www.virustotal.com/gui/ip-address/" + url.PathEscape(indicator)
	case KindURL:
		id := base64.RawURLEncoding.EncodeToString([]byte(indicator))
		path = "https://www.virustotal.com/api/v3/urls/" + id
		permalink = "https://www.virustotal.com/gui/url/" + id
	default:
		return nil, ErrSkip
	}

	code, body, err := p.s.doJSON(ctx, http.MethodGet, path, nil, map[string]string{"x-apikey": apiKey})
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if code == http.StatusNotFound {
		return &Result{
			Provider:  "virustotal",
			Kind:      string(kind),
			Indicator: indicator,
			Verdict:   "unknown",
			Permalink: permalink,
		}, nil
	}
	if code >= 300 {
		return nil, fmt.Errorf("virustotal http %d: %s", code, truncate(string(body), 180))
	}

	var payload struct {
		Data struct {
			Attributes struct {
				LastAnalysisStats struct {
					Malicious  int `json:"malicious"`
					Suspicious int `json:"suspicious"`
					Harmless   int `json:"harmless"`
					Undetected int `json:"undetected"`
				} `json:"last_analysis_stats"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	stats := payload.Data.Attributes.LastAnalysisStats
	verdict := "clean"
	switch {
	case stats.Malicious > 0:
		verdict = "malicious"
	case stats.Suspicious > 0:
		verdict = "suspicious"
	case stats.Malicious+stats.Suspicious+stats.Harmless+stats.Undetected == 0:
		verdict = "unknown"
	}
	return &Result{
		Provider:   "virustotal",
		Kind:       string(kind),
		Indicator:  indicator,
		Verdict:    verdict,
		Malicious:  stats.Malicious,
		Suspicious: stats.Suspicious,
		Harmless:   stats.Harmless,
		Undetected: stats.Undetected,
		Permalink:  permalink,
	}, nil
}
