package ti

import (
	"context"
	"sort"
	"sync"

	"github.com/browser-gateway/backend/internal/domain"
)

// maxTaintConcurrency bounds how many blocklist checks run at once for a single session's
// traffic sweep -- Spamhaus is just a DNS query (cheap) but URLhaus is a live HTTP call to
// abuse.ch, and a chatty session can easily have hundreds of unique domains.
const maxTaintConcurrency = 10

// maxTaintDomains caps how many unique domains from one session get checked, so a very
// chatty session can't turn this into an unbounded background sweep.
const maxTaintDomains = 200

// CheckDomainsAgainstOpenBlocklists checks each domain against Spamhaus DNSBL and URLhaus
// only -- the two no-API-key providers -- regardless of which other (possibly paid/
// rate-limited) providers are enabled, since this runs automatically over an entire
// session's traffic rather than a single user-initiated lookup. Still respects TiEnabled as
// a master off switch, and each individual provider's own TiXEnabled flag, same as any other
// use of this package. Returns the sorted list of domains found on either blocklist and the
// number of domains actually checked (<= len(domains), capped by maxTaintDomains).
func (s *Service) CheckDomainsAgainstOpenBlocklists(ctx context.Context, settings domain.AppSettings, domains []string) ([]string, int) {
	if !settings.TiEnabled || len(domains) == 0 {
		return nil, 0
	}
	var providers []providerLookup
	if settings.TiSpamhausEnabled {
		providers = append(providers, spamhausProvider{})
	}
	if settings.TiURLHausEnabled {
		providers = append(providers, urlhausProvider{s: s})
	}
	if len(providers) == 0 {
		return nil, 0
	}
	if len(domains) > maxTaintDomains {
		domains = domains[:maxTaintDomains]
	}

	type job struct {
		domain string
		p      providerLookup
	}
	jobs := make(chan job)
	flaggedSet := map[string]bool{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for w := 0; w < maxTaintConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var res *Result
				if cached, ok := s.fromCache(j.p.ID(), string(KindDomain), j.domain); ok {
					res = cached
				} else {
					r, err := j.p.Lookup(ctx, KindDomain, j.domain, "")
					if err != nil {
						continue
					}
					s.saveCache(r)
					res = r
				}
				if res.Verdict == "malicious" {
					mu.Lock()
					flaggedSet[j.domain] = true
					mu.Unlock()
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, d := range domains {
			for _, p := range providers {
				select {
				case jobs <- job{domain: d, p: p}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	wg.Wait()

	flagged := make([]string, 0, len(flaggedSet))
	for d := range flaggedSet {
		flagged = append(flagged, d)
	}
	sort.Strings(flagged)
	return flagged, len(domains)
}
