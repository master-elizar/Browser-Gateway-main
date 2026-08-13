package ti

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type safeBrowsingProvider struct{ s *Service }

func (safeBrowsingProvider) ID() string { return "safebrowsing" }

func (p safeBrowsingProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	var checkURL string
	switch kind {
	case KindURL:
		checkURL = indicator
	case KindDomain:
		checkURL = "http://" + indicator + "/"
	default:
		return nil, ErrSkip
	}

	reqBody, _ := json.Marshal(map[string]any{
		"client": map[string]string{
			"clientId":      "browser-gateway",
			"clientVersion": "1.0.0",
		},
		"threatInfo": map[string]any{
			"threatTypes":      []string{"MALWARE", "SOCIAL_ENGINEERING", "UNWANTED_SOFTWARE", "POTENTIALLY_HARMFUL_APPLICATION"},
			"platformTypes":    []string{"ANY_PLATFORM"},
			"threatEntryTypes": []string{"URL"},
			"threatEntries":    []map[string]string{{"url": checkURL}},
		},
	})
	u := "https://safebrowsing.googleapis.com/v4/threatMatches:find?key=" + apiKey
	code, body, err := p.s.doJSON(ctx, http.MethodPost, u, bytes.NewReader(reqBody), nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if code >= 300 {
		return nil, fmt.Errorf("safebrowsing http %d: %s", code, truncate(string(body), 180))
	}

	var payload struct {
		Matches []struct {
			ThreatType string `json:"threatType"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	res := &Result{
		Provider:  "safebrowsing",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://transparencyreport.google.com/safe-browsing/search?url=" + checkURL,
	}
	if len(payload.Matches) == 0 {
		res.Verdict = "clean"
		res.Harmless = 1
		res.Detail = "no threats found"
		return res, nil
	}
	types := make([]string, 0, len(payload.Matches))
	seen := map[string]bool{}
	for _, m := range payload.Matches {
		if !seen[m.ThreatType] {
			seen[m.ThreatType] = true
			types = append(types, m.ThreatType)
		}
	}
	res.Verdict = "malicious"
	res.Malicious = len(types)
	res.Detail = "flagged: " + strings.Join(types, ", ")
	return res, nil
}
