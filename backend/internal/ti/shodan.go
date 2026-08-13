package ti

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type shodanProvider struct{ s *Service }

func (shodanProvider) ID() string { return "shodan" }

// suspiciousShodanTags are Shodan's own free-text tags that occasionally flag a host as
// compromised/malicious infrastructure. Shodan itself has no malicious/clean verdict --
// this is a weak heuristic on top of informational data, never counted in the aggregate
// malicious/total ratio (see Result.Informational).
var suspiciousShodanTags = map[string]bool{
	"malware": true, "c2": true, "botnet": true, "compromised": true, "phishing": true,
}

func (p shodanProvider) Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error) {
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}
	var ips []string
	switch kind {
	case KindIP:
		ips = []string{indicator}
	case KindDomain:
		resolved, err := net.DefaultResolver.LookupHost(ctx, indicator)
		if err != nil || len(resolved) == 0 {
			return &Result{
				Provider:      "shodan",
				Kind:          string(kind),
				Indicator:     indicator,
				Verdict:       "unknown",
				Informational: true,
				Detail:        "domain did not resolve, nothing to look up in Shodan",
				Permalink:     "https://www.shodan.io/search?query=hostname:" + url.QueryEscape(indicator),
			}, nil
		}
		// Cap fan-out: a domain can resolve to many IPs (CDNs), Shodan is queried once per IP.
		if len(resolved) > 3 {
			resolved = resolved[:3]
		}
		ips = resolved
	default:
		return nil, ErrSkip
	}

	type hostInfo struct {
		ip    string
		ports []int
		org   string
		tags  []string
	}
	hosts := make([]hostInfo, 0, len(ips))
	suspicious := false
	for _, ip := range ips {
		u := "https://api.shodan.io/shodan/host/" + url.PathEscape(ip) + "?key=" + url.QueryEscape(apiKey)
		code, body, err := p.s.doJSON(ctx, http.MethodGet, u, nil, nil)
		if err != nil || code == http.StatusNotFound {
			continue // not indexed by Shodan -- not an error, just nothing known
		}
		if code == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		}
		if code >= 300 {
			continue
		}
		var payload struct {
			Ports []int    `json:"ports"`
			Org   string   `json:"org"`
			Tags  []string `json:"tags"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			continue
		}
		for _, t := range payload.Tags {
			if suspiciousShodanTags[strings.ToLower(t)] {
				suspicious = true
			}
		}
		hosts = append(hosts, hostInfo{ip: ip, ports: payload.Ports, org: payload.Org, tags: payload.Tags})
	}

	res := &Result{
		Provider:      "shodan",
		Kind:          string(kind),
		Indicator:     indicator,
		Verdict:       "unknown",
		Informational: true,
	}
	if kind == KindIP {
		res.Permalink = "https://www.shodan.io/host/" + url.PathEscape(indicator)
	} else {
		res.Permalink = "https://www.shodan.io/search?query=hostname:" + url.QueryEscape(indicator)
	}
	if len(hosts) == 0 {
		res.Detail = "not indexed by Shodan"
		return res, nil
	}
	if suspicious {
		res.Verdict = "suspicious"
	}
	portSet := map[int]bool{}
	orgs := map[string]bool{}
	var tags []string
	for _, h := range hosts {
		for _, port := range h.ports {
			portSet[port] = true
		}
		if h.org != "" {
			orgs[h.org] = true
		}
		tags = append(tags, h.tags...)
	}
	ports := make([]int, 0, len(portSet))
	for port := range portSet {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	portStrs := make([]string, 0, len(ports))
	for _, port := range ports {
		portStrs = append(portStrs, strconv.Itoa(port))
	}
	orgList := make([]string, 0, len(orgs))
	for o := range orgs {
		orgList = append(orgList, o)
	}
	sort.Strings(orgList)

	detail := fmt.Sprintf("%d host(s) indexed, %d open port(s)", len(hosts), len(ports))
	if len(portStrs) > 0 {
		detail += " (" + strings.Join(portStrs, ",") + ")"
	}
	if len(orgList) > 0 {
		detail += ", org: " + strings.Join(orgList, ", ")
	}
	if len(tags) > 0 {
		detail += ", tags: " + strings.Join(tags, ", ")
	}
	res.Detail = detail
	return res, nil
}
