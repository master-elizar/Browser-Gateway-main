package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type urlhausProvider struct{ s *Service }

func (urlhausProvider) ID() string { return "urlhaus" }

func (p urlhausProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	form := url.Values{}
	endpoint := ""
	permalink := "https://urlhaus.abuse.ch/"
	switch kind {
	case KindDomain:
		endpoint = "https://urlhaus-api.abuse.ch/v1/host/"
		form.Set("host", indicator)
		permalink = "https://urlhaus.abuse.ch/host/" + url.PathEscape(indicator) + "/"
	case KindIP:
		endpoint = "https://urlhaus-api.abuse.ch/v1/host/"
		form.Set("host", indicator)
		permalink = "https://urlhaus.abuse.ch/host/" + url.PathEscape(indicator) + "/"
	case KindURL:
		endpoint = "https://urlhaus-api.abuse.ch/v1/url/"
		form.Set("url", indicator)
	default:
		return nil, ErrSkip
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := p.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := readLimited(resp.Body)
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("urlhaus http %d: %s", resp.StatusCode, truncate(string(body), 180))
	}

	var payload struct {
		QueryStatus string `json:"query_status"`
		URLCount    any    `json:"url_count"`
		URLs        []any  `json:"urls"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	res := &Result{
		Provider:  "urlhaus",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: permalink,
	}
	switch payload.QueryStatus {
	case "ok":
		res.Verdict = "malicious"
		res.Malicious = 1
		if n := len(payload.URLs); n > 0 {
			res.Malicious = n
		}
	case "no_results", "no_result":
		res.Verdict = "clean"
		res.Harmless = 1
	default:
		res.Verdict = "unknown"
		res.Undetected = 1
	}
	return res, nil
}
