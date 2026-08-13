package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WhoisInfo is registration data fetched via RDAP -- purely informational, not wired into
// the provider registry/aggregation (no malicious/clean concept applies to it). Reserved
// for the domain-check tab's advanced-mode view.
type WhoisInfo struct {
	Domain      string    `json:"domain"`
	Registrar   string    `json:"registrar,omitempty"`
	Registered  time.Time `json:"registered,omitempty"`
	Expires     time.Time `json:"expires,omitempty"`
	Nameservers []string  `json:"nameservers,omitempty"`
}

type rdapResponse struct {
	LdhName     string `json:"ldhName"`
	Nameservers []struct {
		LdhName string `json:"ldhName"`
	} `json:"nameservers"`
	Events []struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	} `json:"events"`
	Entities []struct {
		Roles      []string `json:"roles"`
		VcardArray []any    `json:"vcardArray"`
	} `json:"entities"`
}

// LookupRDAP fetches registration data (registrar, registration/expiry dates,
// nameservers) via rdap.org, a public RDAP bootstrap redirector that resolves the right
// registry RDAP server for any TLD without this service maintaining its own IANA bootstrap
// list. No API key required.
func (s *Service) LookupRDAP(ctx context.Context, domainName string) (*WhoisInfo, error) {
	u := "https://rdap.org/domain/" + url.PathEscape(domainName)
	code, body, err := s.doJSON(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, fmt.Errorf("no RDAP record found for %s", domainName)
	}
	if code >= 300 {
		return nil, fmt.Errorf("rdap http %d: %s", code, truncate(string(body), 180))
	}

	var resp rdapResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	info := &WhoisInfo{Domain: domainName}
	if resp.LdhName != "" {
		info.Domain = resp.LdhName
	}
	for _, ns := range resp.Nameservers {
		if ns.LdhName != "" {
			info.Nameservers = append(info.Nameservers, strings.ToLower(ns.LdhName))
		}
	}
	for _, ev := range resp.Events {
		t, err := time.Parse(time.RFC3339, ev.EventDate)
		if err != nil {
			continue
		}
		switch ev.EventAction {
		case "registration":
			info.Registered = t
		case "expiration":
			info.Expires = t
		}
	}
	for _, e := range resp.Entities {
		isRegistrar := false
		for _, r := range e.Roles {
			if r == "registrar" {
				isRegistrar = true
				break
			}
		}
		if isRegistrar {
			if name := vcardFullName(e.VcardArray); name != "" {
				info.Registrar = name
				break
			}
		}
	}
	return info, nil
}

// vcardFullName extracts the "fn" (full name) property from an RDAP jCard-format
// vcardArray: ["vcard", [["version",{},"text","4.0"], ["fn",{},"text","Example Registrar"], ...]].
func vcardFullName(vcardArray []any) string {
	if len(vcardArray) != 2 {
		return ""
	}
	props, ok := vcardArray[1].([]any)
	if !ok {
		return ""
	}
	for _, raw := range props {
		prop, ok := raw.([]any)
		if !ok || len(prop) < 4 {
			continue
		}
		name, ok := prop[0].(string)
		if !ok || name != "fn" {
			continue
		}
		if value, ok := prop[3].(string); ok {
			return value
		}
	}
	return ""
}
