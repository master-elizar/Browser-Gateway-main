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
// for the domain-check tab's advanced-mode view. Domain-kind lookups populate Domain/
// Registrar/Nameservers/Registered/Expires plus the contact/status fields below; IP-kind
// lookups instead populate NetworkName/NetworkRange/Country/RIR and reuse the same contact
// fields (a network allocation has entities too, just no registrar/nameservers/expiry).
type WhoisInfo struct {
	Domain    string `json:"domain,omitempty"`
	Registrar string `json:"registrar,omitempty"`
	// Pointers, not time.Time -- encoding/json's omitempty never treats a struct (including
	// the zero time.Time) as empty, so a plain time.Time field would always serialize even
	// when RDAP had no registration/expiration event, sending the frontend a fake
	// "0001-01-01" date instead of just omitting the field.
	Registered  *time.Time `json:"registered,omitempty"`
	Expires     *time.Time `json:"expires,omitempty"`
	Nameservers []string   `json:"nameservers,omitempty"`

	// Expanded contact/status fields (Stage 19), from RDAP entities the earlier version left
	// unparsed. Org falls back to the entity's full name (vCard "fn") when no "org" property
	// is present -- either way it's the best single human-readable label RDAP gives us.
	RegistrantOrg string   `json:"registrantOrg,omitempty"`
	AdminOrg      string   `json:"adminOrg,omitempty"`
	TechOrg       string   `json:"techOrg,omitempty"`
	AbuseOrg      string   `json:"abuseOrg,omitempty"`
	AbuseEmail    string   `json:"abuseEmail,omitempty"`
	Status        []string `json:"status,omitempty"`
	DNSSEC        bool     `json:"dnssec,omitempty"`

	// IP-kind fields (network allocation, not a domain).
	NetworkName  string `json:"networkName,omitempty"`
	NetworkRange string `json:"networkRange,omitempty"`
	Country      string `json:"country,omitempty"`
	// RIR is a best-effort guess (ARIN/RIPE NCC/APNIC/LACNIC/AfriNIC) read from the RDAP
	// response's own links/port43 fields, since the network object itself doesn't declare
	// "which registry answered this" as a dedicated field.
	RIR string `json:"rir,omitempty"`
}

// rdapEntity mirrors RDAP's recursive entity shape -- an entity (e.g. the registrar) can
// itself contain sub-entities (e.g. an "abuse" contact nested inside the registrar), so this
// has to be a named recursive type rather than an inline anonymous struct.
type rdapEntity struct {
	Roles      []string     `json:"roles"`
	VcardArray []any        `json:"vcardArray"`
	Entities   []rdapEntity `json:"entities"`
}

type rdapDomainResponse struct {
	LdhName     string `json:"ldhName"`
	Nameservers []struct {
		LdhName string `json:"ldhName"`
	} `json:"nameservers"`
	Events []struct {
		EventAction string `json:"eventAction"`
		EventDate   string `json:"eventDate"`
	} `json:"events"`
	Entities  []rdapEntity `json:"entities"`
	Status    []string     `json:"status"`
	SecureDNS struct {
		DelegationSigned bool `json:"delegationSigned"`
	} `json:"secureDNS"`
}

type rdapIPResponse struct {
	Handle       string       `json:"handle"`
	StartAddress string       `json:"startAddress"`
	EndAddress   string       `json:"endAddress"`
	Name         string       `json:"name"`
	Country      string       `json:"country"`
	Port43       string       `json:"port43"`
	Entities     []rdapEntity `json:"entities"`
	Links        []struct {
		Href string `json:"href"`
	} `json:"links"`
}

// LookupRDAP fetches registration data (registrar, registration/expiry dates, nameservers,
// contacts, status, DNSSEC) via rdap.org, a public RDAP bootstrap redirector that resolves
// the right registry RDAP server for any TLD without this service maintaining its own IANA
// bootstrap list. No API key required.
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

	var resp rdapDomainResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	info := &WhoisInfo{Domain: domainName, Status: resp.Status, DNSSEC: resp.SecureDNS.DelegationSigned}
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
			registered := t
			info.Registered = &registered
		case "expiration":
			expires := t
			info.Expires = &expires
		}
	}
	walkRDAPEntities(resp.Entities, func(roles []string, vcard []any) {
		label := entityLabel(vcard)
		if hasRDAPRole(roles, "registrar") && info.Registrar == "" {
			info.Registrar = label
		}
		if hasRDAPRole(roles, "registrant") && info.RegistrantOrg == "" {
			info.RegistrantOrg = label
		}
		if hasRDAPRole(roles, "administrative") && info.AdminOrg == "" {
			info.AdminOrg = label
		}
		if hasRDAPRole(roles, "technical") && info.TechOrg == "" {
			info.TechOrg = label
		}
		if hasRDAPRole(roles, "abuse") {
			if info.AbuseOrg == "" {
				info.AbuseOrg = label
			}
			if info.AbuseEmail == "" {
				info.AbuseEmail = vcardProp(vcard, "email")
			}
		}
	})
	return info, nil
}

// LookupRDAPIP fetches network allocation data (org/network name, range, country, abuse
// contact) for an IP address via the same rdap.org redirector used for domains.
func (s *Service) LookupRDAPIP(ctx context.Context, ip string) (*WhoisInfo, error) {
	u := "https://rdap.org/ip/" + url.PathEscape(ip)
	code, body, err := s.doJSON(ctx, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, err
	}
	if code == http.StatusNotFound {
		return nil, fmt.Errorf("no RDAP record found for %s", ip)
	}
	if code >= 300 {
		return nil, fmt.Errorf("rdap http %d: %s", code, truncate(string(body), 180))
	}

	var resp rdapIPResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	info := &WhoisInfo{
		NetworkName: resp.Name,
		Country:     resp.Country,
	}
	switch {
	case resp.StartAddress != "" && resp.EndAddress != "":
		info.NetworkRange = resp.StartAddress + " - " + resp.EndAddress
	case resp.Handle != "":
		info.NetworkRange = resp.Handle
	}
	info.RIR = guessRIR(resp.Port43, linkHosts(resp.Links))
	walkRDAPEntities(resp.Entities, func(roles []string, vcard []any) {
		label := entityLabel(vcard)
		if hasRDAPRole(roles, "registrant") && info.RegistrantOrg == "" {
			info.RegistrantOrg = label
		}
		if hasRDAPRole(roles, "technical") && info.TechOrg == "" {
			info.TechOrg = label
		}
		if hasRDAPRole(roles, "abuse") {
			if info.AbuseOrg == "" {
				info.AbuseOrg = label
			}
			if info.AbuseEmail == "" {
				info.AbuseEmail = vcardProp(vcard, "email")
			}
		}
	})
	return info, nil
}

func linkHosts(links []struct {
	Href string `json:"href"`
}) string {
	var b strings.Builder
	for _, l := range links {
		b.WriteString(l.Href)
		b.WriteString(" ")
	}
	return b.String()
}

// guessRIR infers which of the five Regional Internet Registries answered an IP RDAP query.
// Neither field RDAP returns is a declared "registry name" -- port43 (the legacy WHOIS
// server) and the response's own self-link hostname are the closest available signals, so
// this is a best-effort substring match rather than an authoritative field.
func guessRIR(port43, linkText string) string {
	haystack := strings.ToLower(port43 + " " + linkText)
	switch {
	case strings.Contains(haystack, "arin"):
		return "ARIN"
	case strings.Contains(haystack, "ripe"):
		return "RIPE NCC"
	case strings.Contains(haystack, "apnic"):
		return "APNIC"
	case strings.Contains(haystack, "lacnic"):
		return "LACNIC"
	case strings.Contains(haystack, "afrinic"):
		return "AfriNIC"
	default:
		return ""
	}
}

// walkRDAPEntities visits every entity in the tree, including nested sub-entities (e.g. an
// "abuse" contact nested inside a "registrar" entity), calling fn once per entity with its
// roles and raw vCard array.
func walkRDAPEntities(entities []rdapEntity, fn func(roles []string, vcard []any)) {
	for _, e := range entities {
		fn(e.Roles, e.VcardArray)
		walkRDAPEntities(e.Entities, fn)
	}
}

func hasRDAPRole(roles []string, want string) bool {
	for _, r := range roles {
		if r == want {
			return true
		}
	}
	return false
}

// entityLabel picks the best single human-readable label for an entity: its vCard
// organization name if present, falling back to its full name.
func entityLabel(vcardArray []any) string {
	if org := vcardProp(vcardArray, "org"); org != "" {
		return org
	}
	return vcardProp(vcardArray, "fn")
}

// vcardProp extracts a named property's text value from an RDAP jCard-format vcardArray:
// ["vcard", [["version",{},"text","4.0"], ["fn",{},"text","Example Registrar"], ...]]. Most
// properties carry a plain string value, but "org" in particular is sometimes a jCard
// structured-value array (organization name plus unit names) instead -- handled by joining
// its string elements rather than dropping the value.
func vcardProp(vcardArray []any, propName string) string {
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
		if !ok || name != propName {
			continue
		}
		switch value := prop[3].(type) {
		case string:
			return value
		case []any:
			parts := make([]string, 0, len(value))
			for _, v := range value {
				if s, ok := v.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			return strings.Join(parts, ", ")
		}
	}
	return ""
}
