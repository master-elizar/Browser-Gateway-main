package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type crtShProvider struct{ s *Service }

func (crtShProvider) ID() string { return "crtsh" }

// crtShProvider is purely informational (certificate-transparency history) -- it has no
// concept of a malicious/clean verdict, see Result.Informational.
func (p crtShProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	if kind != KindDomain {
		return nil, ErrSkip
	}
	u := "https://crt.sh/?q=" + url.QueryEscape(indicator) + "&output=json"
	code, body, err := p.s.doJSON(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Provider:      "crtsh",
		Kind:          string(kind),
		Indicator:     indicator,
		Verdict:       "unknown",
		Informational: true,
		Permalink:     "https://crt.sh/?q=" + url.QueryEscape(indicator),
	}
	if code == http.StatusNotFound {
		res.Detail = "no certificates found in CT logs"
		return res, nil
	}
	if code >= 300 {
		return nil, fmt.Errorf("crt.sh http %d: %s", code, truncate(string(body), 180))
	}

	var entries []struct {
		IssuerName  string `json:"issuer_name"`
		NotBefore   string `json:"not_before"`
		CommonName  string `json:"common_name"`
		NameValue   string `json:"name_value"`
	}
	if err := json.Unmarshal(body, &entries); err != nil || len(entries) == 0 {
		res.Detail = "no certificates found in CT logs"
		return res, nil
	}

	issuers := map[string]bool{}
	names := map[string]bool{}
	var mostRecent string
	for _, e := range entries {
		if e.IssuerName != "" {
			// Issuer names are long DN strings ("C=US, O=Let's Encrypt, CN=R3") -- keep just
			// the org/CN-ish tail so the summary stays readable.
			issuers[shortIssuer(e.IssuerName)] = true
		}
		for _, n := range strings.Split(e.NameValue, "\n") {
			if n = strings.TrimSpace(n); n != "" {
				names[n] = true
			}
		}
		if e.NotBefore > mostRecent {
			mostRecent = e.NotBefore
		}
	}
	issuerList := make([]string, 0, len(issuers))
	for i := range issuers {
		issuerList = append(issuerList, i)
	}
	sort.Strings(issuerList)

	detail := fmt.Sprintf("%d certificate(s) in CT logs, %d distinct name(s)", len(entries), len(names))
	if mostRecent != "" {
		detail += ", most recent " + strings.SplitN(mostRecent, "T", 2)[0]
	}
	if len(issuerList) > 0 {
		if len(issuerList) > 3 {
			issuerList = issuerList[:3]
		}
		detail += ", issuers: " + strings.Join(issuerList, ", ")
	}
	res.Detail = detail
	return res, nil
}

func shortIssuer(dn string) string {
	for _, part := range strings.Split(dn, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "O=") {
			return strings.TrimPrefix(part, "O=")
		}
	}
	return dn
}
