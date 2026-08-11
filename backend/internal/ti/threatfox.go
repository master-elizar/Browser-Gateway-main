package ti

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type threatFoxProvider struct{ s *Service }

func (threatFoxProvider) ID() string { return "threatfox" }

func (p threatFoxProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	_ = kind // search_ioc accepts host/ip/url-ish terms
	payload, _ := json.Marshal(map[string]string{
		"query":       "search_ioc",
		"search_term": indicator,
	})
	headers := map[string]string{"Content-Type": "application/json"}
	if apiKey != "" {
		headers["Auth-Key"] = apiKey
	}
	code, body, err := p.s.doJSON(ctx, http.MethodPost, "https://threatfox-api.abuse.ch/api/v1/", bytes.NewReader(payload), headers)
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if code >= 300 {
		return nil, fmt.Errorf("threatfox http %d: %s", code, truncate(string(body), 180))
	}

	var resp struct {
		QueryStatus string `json:"query_status"`
		Data        []any  `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	res := &Result{
		Provider:  "threatfox",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://threatfox.abuse.ch/browse.php?search=" + indicator,
	}
	switch resp.QueryStatus {
	case "ok":
		res.Verdict = "malicious"
		res.Malicious = len(resp.Data)
		if res.Malicious == 0 {
			res.Malicious = 1
		}
	case "no_result", "no_results":
		res.Verdict = "clean"
		res.Harmless = 1
	default:
		res.Verdict = "unknown"
		res.Undetected = 1
	}
	return res, nil
}

func readLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, 2<<20))
}
