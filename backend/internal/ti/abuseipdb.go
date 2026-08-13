package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type abuseIPDBProvider struct{ s *Service }

func (abuseIPDBProvider) ID() string { return "abuseipdb" }

func (p abuseIPDBProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	if kind != KindIP {
		return nil, ErrSkip
	}
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	u := "https://api.abuseipdb.com/api/v2/check?ipAddress=" + url.QueryEscape(indicator) + "&maxAgeInDays=90&verbose"
	code, body, err := p.s.doJSON(ctx, http.MethodGet, u, nil, map[string]string{
		"Key":    apiKey,
		"Accept": "application/json",
	})
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if code >= 300 {
		return nil, fmt.Errorf("abuseipdb http %d: %s", code, truncate(string(body), 180))
	}

	var payload struct {
		Data struct {
			AbuseConfidenceScore int    `json:"abuseConfidenceScore"`
			TotalReports         int    `json:"totalReports"`
			IPAddress            string `json:"ipAddress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	score := payload.Data.AbuseConfidenceScore
	res := &Result{
		Provider:  "abuseipdb",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://www.abuseipdb.com/check/" + url.PathEscape(indicator),
		Detail:    fmt.Sprintf("confidence score %d, %d report(s)", score, payload.Data.TotalReports),
	}
	switch {
	case score >= 75 || payload.Data.TotalReports >= 10:
		res.Verdict = "malicious"
		res.Malicious = 1
	case score >= 25 || payload.Data.TotalReports > 0:
		res.Verdict = "suspicious"
		res.Suspicious = 1
	default:
		res.Verdict = "clean"
		res.Harmless = 1
	}
	return res, nil
}
