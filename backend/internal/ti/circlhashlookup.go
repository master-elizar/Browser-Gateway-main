package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// circlHashlookupProvider queries CIRCL's free, no-key Hashlookup service
// (hashlookup.circl.lu), a database primarily of known/legitimate files (NSRL and vendor
// software sets) that also cross-references a subset of entries against known-malicious
// tagging. Because a "not found" result says nothing about maliciousness (this is
// fundamentally a goodware allowlist, not a malware feed) and the exact response shape can't
// be verified from this environment, this provider only ever asserts a verdict when it finds
// an explicit malicious tag -- every other outcome (found-but-clean, not-found, error) is
// reported as Informational so it can't skew the aggregation ratio with a guess.
type circlHashlookupProvider struct{ s *Service }

func (circlHashlookupProvider) ID() string { return "circlhashlookup" }

func (p circlHashlookupProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	if kind != KindHash {
		return nil, ErrSkip
	}
	algo := ""
	switch len(indicator) {
	case 32:
		algo = "md5"
	case 40:
		algo = "sha1"
	case 64:
		algo = "sha256"
	default:
		return nil, ErrSkip
	}
	rawURL := fmt.Sprintf("https://hashlookup.circl.lu/lookup/%s/%s", algo, indicator)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := readLimited(resp.Body)

	res := &Result{
		Provider:  "circlhashlookup",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://hashlookup.circl.lu/",
	}
	if resp.StatusCode == http.StatusNotFound {
		res.Informational = true
		res.Detail = "not found in CIRCL Hashlookup"
		return res, nil
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("circl hashlookup http %d: %s", resp.StatusCode, truncate(string(body), 180))
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if malicious, ok := payload["KnownMalicious"]; ok && !isEmptyJSONValue(malicious) {
		res.Verdict = "malicious"
		res.Malicious = 1
		res.Detail = fmt.Sprintf("flagged known-malicious by CIRCL Hashlookup: %v", malicious)
		return res, nil
	}
	res.Informational = true
	if name, ok := payload["FileName"].(string); ok && name != "" {
		res.Detail = "known file in CIRCL Hashlookup (NSRL/vendor set): " + name
	} else {
		res.Detail = "known file in CIRCL Hashlookup (NSRL/vendor set)"
	}
	return res, nil
}

func isEmptyJSONValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	default:
		return false
	}
}
