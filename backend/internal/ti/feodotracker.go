package ti

import (
	"context"
	"net"
)

type feodoProvider struct{ s *Service }

func (feodoProvider) ID() string { return "feodo" }

// feodoProvider checks abuse.ch's Feodo Tracker C2 (botnet command-and-control) IP
// blocklist -- no API key, fetched/cached by Service.feodoBlocklist rather than queried
// live per lookup.
func (p feodoProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	var ips []string
	switch kind {
	case KindIP:
		ips = []string{indicator}
	case KindDomain:
		resolved, err := net.DefaultResolver.LookupHost(ctx, indicator)
		if err != nil {
			return &Result{
				Provider:  "feodo",
				Kind:      string(kind),
				Indicator: indicator,
				Verdict:   "clean",
				Harmless:  1,
				Detail:    "domain did not resolve",
				Permalink: "https://feodotracker.abuse.ch/browse/",
			}, nil
		}
		ips = resolved
	default:
		return nil, ErrSkip
	}

	blocklist, err := p.s.feodoBlocklist(ctx)
	if err != nil {
		return nil, err
	}
	res := &Result{
		Provider:  "feodo",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://feodotracker.abuse.ch/browse/",
	}
	for _, ip := range ips {
		if malware, ok := blocklist[ip]; ok {
			res.Verdict = "malicious"
			res.Malicious = 1
			if malware != "" {
				res.Detail = ip + " is an active C2 (" + malware + ")"
			} else {
				res.Detail = ip + " is a known active C2 IP"
			}
			return res, nil
		}
	}
	res.Verdict = "clean"
	res.Harmless = 1
	res.Detail = "not in the Feodo Tracker C2 blocklist"
	return res, nil
}
