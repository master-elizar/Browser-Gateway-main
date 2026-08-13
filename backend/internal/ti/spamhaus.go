package ti

import (
	"context"
	"net"
	"strings"
)

type spamhausProvider struct{}

func (spamhausProvider) ID() string { return "spamhaus" }

func (spamhausProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	var q string
	switch kind {
	case KindIP:
		ip := net.ParseIP(indicator)
		if ip == nil {
			return nil, ErrSkip
		}
		ip4 := ip.To4()
		if ip4 == nil {
			// Spamhaus ZEN is primarily IPv4; skip IPv6 quietly.
			return nil, ErrSkip
		}
		// Reverse octets for zen.spamhaus.org
		q = reverseIPv4(ip4.String()) + ".zen.spamhaus.org"
	case KindDomain:
		host := strings.TrimSuffix(strings.ToLower(indicator), ".")
		if host == "" {
			return nil, ErrSkip
		}
		q = host + ".dbl.spamhaus.org"
	default:
		return nil, ErrSkip
	}

	res := &Result{
		Provider:  "spamhaus",
		Kind:      string(kind),
		Indicator: indicator,
		Permalink: "https://check.spamhaus.org/",
	}

	// Context-aware resolver
	r := &net.Resolver{}
	addrs, err := r.LookupHost(ctx, q)
	if err != nil {
		// NXDOMAIN / no list hit → clean
		if dnsErr, ok := err.(*net.DNSError); ok && (dnsErr.IsNotFound || dnsErr.Err == "no such host") {
			res.Verdict = "clean"
			res.Harmless = 1
			res.Detail = "not listed"
			return res, nil
		}
		// Other DNS failures → unknown (do not fail the whole multi-lookup)
		res.Verdict = "unknown"
		res.Undetected = 1
		res.Error = err.Error()
		return res, nil
	}
	if len(addrs) == 0 {
		res.Verdict = "clean"
		res.Harmless = 1
		res.Detail = "not listed"
		return res, nil
	}
	// Any spamhaus return code in 127.0.0.0/8 is a hit.
	res.Verdict = "malicious"
	res.Malicious = 1
	res.Detail = "listed (" + strings.Join(addrs, ", ") + ")"
	return res, nil
}

func reverseIPv4(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}
	return parts[3] + "." + parts[2] + "." + parts[1] + "." + parts[0]
}
