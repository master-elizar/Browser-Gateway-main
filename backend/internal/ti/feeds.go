package ti

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
)

// feeds.go implements local bulk threat-intel feed ingestion: free, no-API-key flat-file
// blocklists (domains/IPs) downloaded periodically and held in memory for instant O(1)/
// CIDR-tree matching, as opposed to the rest of this package's live per-lookup API queries.
// A feed with hundreds of thousands of entries would be absurd to re-download on every
// domain-check click, so these are fetched on a background schedule (see StartFeedRefresher)
// and queried purely from memory -- no network round-trip, no rate limit, always available
// once the first fetch succeeds.

type feedFormat int

const (
	// feedFormatPlainList is one indicator per line; blank lines and '#'/'!' comment lines
	// are skipped, and only the first whitespace-separated token on a line is taken (this
	// also transparently handles "value<TAB>metadata" formats like IPsum's "ip\tcount").
	feedFormatPlainList feedFormat = iota
	// feedFormatHostsFile is "0.0.0.0 example.com" style -- the domain is field 2.
	feedFormatHostsFile
	// feedFormatURLHost is one full URL per line -- the domain is the URL's hostname.
	feedFormatURLHost
	// feedFormatCIDRList is one CIDR or bare IP per line (bare IPs become /32 or /128);
	// trailing "; comment" annotations (Spamhaus DROP/EDROP) are dropped by the same
	// first-token rule as feedFormatPlainList.
	feedFormatCIDRList
)

// feedDef describes one bulk feed source. WildcardURL is optional (domain feeds only) and,
// when set, is fetched as a second plain list whose entries match as suffixes (apex + all
// subdomains) rather than exact hostnames -- used by Phishing.Database's wildcard file.
type feedDef struct {
	ID   string
	Name string
	Kind Kind
	URL  string
	// ExtraURLs are additional sources of the same Format merged into this feed's single
	// exact-match set -- used when one logical source (e.g. blocklistproject's several
	// category files) ships as multiple separate downloads. They must stay merged into one
	// feed rather than registered as separate feedDefs, or a single logical source would
	// count as several entries in the malicious/total-sources aggregation ratio.
	ExtraURLs   []string
	WildcardURL string
	Format      feedFormat
	TTL         time.Duration
	Permalink   string
	Enabled     func(domain.AppSettings) bool
}

// loadedFeed holds one feed's current in-memory data plus refresh bookkeeping. A feed with
// a zero fetchedAt has never completed a successful fetch yet (still starting up, or its
// first attempt failed) -- feedProvider.Lookup skips participating in aggregation until then
// rather than asserting a false "clean" verdict from empty data.
type loadedFeed struct {
	mu        sync.RWMutex
	exact     map[string]struct{} // domain kind: exact hostnames
	wildcard  []string            // domain kind: suffix-match roots
	cidrs     []*net.IPNet        // ip kind
	fetchedAt time.Time
	lastErr   error
	count     int
}

func (f *loadedFeed) snapshot() (fetchedAt time.Time, loaded bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.fetchedAt, !f.fetchedAt.IsZero()
}

func (f *loadedFeed) matchDomain(host string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if _, ok := f.exact[host]; ok {
		return true
	}
	for _, root := range f.wildcard {
		if host == root || strings.HasSuffix(host, "."+root) {
			return true
		}
	}
	return false
}

func (f *loadedFeed) matchIP(ip net.IP) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, n := range f.cidrs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// FeedManager owns every registered feedDef and its current loadedFeed data.
type FeedManager struct {
	client *http.Client
	defs   []feedDef
	loaded map[string]*loadedFeed
}

func newFeedManager(client *http.Client) *FeedManager {
	fm := &FeedManager{client: client, loaded: map[string]*loadedFeed{}}
	fm.defs = builtinFeedDefs()
	for _, d := range fm.defs {
		fm.loaded[d.ID] = &loadedFeed{}
	}
	return fm
}

// builtinFeedDefs is the curated source list. Every URL here is a well-known, long-standing
// public feed, but none can be reached from this sandboxed dev environment to verify -- if a
// specific one 404s or changes format after deploy, that's real evidence to act on, same as
// any other bug in this codebase.
func builtinFeedDefs() []feedDef {
	return []feedDef{
		{
			ID:          "feed_phishingdb",
			Name:        "Phishing.Database",
			Kind:        KindDomain,
			URL:         "https://raw.githubusercontent.com/Phishing-Database/Phishing.Database/master/phishing-domains-ACTIVE.txt",
			WildcardURL: "https://raw.githubusercontent.com/Phishing-Database/Phishing.Database/master/phishing-domains-NEW-today.txt",
			Format:      feedFormatPlainList,
			TTL:         6 * time.Hour,
			Permalink:   "https://github.com/Phishing-Database/Phishing.Database",
			Enabled:     func(s domain.AppSettings) bool { return s.TiFeedPhishingDBEnabled },
		},
		{
			ID:        "feed_spamhausdrop",
			Name:      "Spamhaus DROP/EDROP",
			Kind:      KindIP,
			URL:       "https://www.spamhaus.org/drop/drop.txt",
			Format:    feedFormatCIDRList,
			TTL:       6 * time.Hour,
			Permalink: "https://www.spamhaus.org/drop/",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedSpamhausDropEnabled },
		},
		{
			ID:        "feed_openphish",
			Name:      "OpenPhish community feed",
			Kind:      KindDomain,
			URL:       "https://openphish.com/feed.txt",
			Format:    feedFormatURLHost,
			TTL:       time.Hour,
			Permalink: "https://openphish.com/",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedOpenPhishEnabled },
		},
		{
			ID:   "feed_blocklistproject",
			Name: "The Block List Project (malware/phishing/ransomware/scam)",
			Kind: KindDomain,
			URL:  "https://raw.githubusercontent.com/blocklistproject/Lists/master/malware.txt",
			ExtraURLs: []string{
				"https://raw.githubusercontent.com/blocklistproject/Lists/master/phishing.txt",
				"https://raw.githubusercontent.com/blocklistproject/Lists/master/ransomware.txt",
				"https://raw.githubusercontent.com/blocklistproject/Lists/master/scam.txt",
			},
			Format:    feedFormatHostsFile,
			TTL:       6 * time.Hour,
			Permalink: "https://github.com/blocklistproject/Lists",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedBlocklistProjectEnabled },
		},
		{
			ID:        "feed_hagezi",
			Name:      "HaGeZi DNS Blocklist (Pro)",
			Kind:      KindDomain,
			URL:       "https://raw.githubusercontent.com/hagezi/dns-blocklists/main/domains/pro.txt",
			Format:    feedFormatPlainList,
			TTL:       6 * time.Hour,
			Permalink: "https://github.com/hagezi/dns-blocklists",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedHaGeziEnabled },
		},
		{
			ID:        "feed_ipsum",
			Name:      "IPsum (multi-source aggregate, confidence >= 3)",
			Kind:      KindIP,
			URL:       "https://raw.githubusercontent.com/stamparm/ipsum/master/levels/3.txt",
			Format:    feedFormatCIDRList,
			TTL:       12 * time.Hour,
			Permalink: "https://github.com/stamparm/ipsum",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedIPsumEnabled },
		},
		{
			ID:   "feed_firehol",
			Name: "FireHOL (level1+2)",
			Kind: KindIP,
			URL:  "https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/firehol_level1.netset",
			ExtraURLs: []string{
				"https://raw.githubusercontent.com/firehol/blocklist-ipsets/master/firehol_level2.netset",
			},
			Format:    feedFormatCIDRList,
			TTL:       12 * time.Hour,
			Permalink: "https://github.com/firehol/blocklist-ipsets",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedFireHOLEnabled },
		},
		{
			ID:        "feed_blocklistde",
			Name:      "blocklist.de",
			Kind:      KindIP,
			URL:       "https://lists.blocklist.de/lists/all.txt",
			Format:    feedFormatCIDRList,
			TTL:       3 * time.Hour,
			Permalink: "https://www.blocklist.de/",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedBlocklistDeEnabled },
		},
		{
			ID:        "feed_cinsarmy",
			Name:      "CINS Army List",
			Kind:      KindIP,
			URL:       "https://cinsscore.com/list/ci-badguys.txt",
			Format:    feedFormatCIDRList,
			TTL:       3 * time.Hour,
			Permalink: "https://cinsscore.com/#list",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedCINSArmyEnabled },
		},
		{
			ID:        "feed_etcompromised",
			Name:      "Emerging Threats compromised IPs",
			Kind:      KindIP,
			URL:       "https://rules.emergingthreats.net/blockrules/compromised-ips.txt",
			Format:    feedFormatCIDRList,
			TTL:       12 * time.Hour,
			Permalink: "https://rules.emergingthreats.net/",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedETCompromisedEnabled },
		},
		{
			ID:        "feed_greensnow",
			Name:      "GreenSnow",
			Kind:      KindIP,
			URL:       "https://blocklist.greensnow.co/greensnow.txt",
			Format:    feedFormatCIDRList,
			TTL:       3 * time.Hour,
			Permalink: "https://greensnow.co/",
			Enabled:   func(s domain.AppSettings) bool { return s.TiFeedGreenSnowEnabled },
		},
	}
}

// StartFeedRefresher launches the background refresh loop for the calling Service's
// FeedManager. getSettings is polled on every refresh pass so newly-enabled feeds start
// downloading without a server restart. Safe to call once per Service; the initial fetch of
// every enabled feed runs in its own goroutine so server boot is never blocked on it.
func (s *Service) StartFeedRefresher(getSettings func() domain.AppSettings) {
	go s.feeds.run(getSettings)
}

func (fm *FeedManager) run(getSettings func() domain.AppSettings) {
	fm.refreshAll(getSettings, true)
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		fm.refreshAll(getSettings, false)
	}
}

// maxFeedConcurrency bounds parallel downloads so a full refresh pass doesn't open a dozen
// simultaneous multi-megabyte connections at once.
const maxFeedConcurrency = 4

func (fm *FeedManager) refreshAll(getSettings func() domain.AppSettings, initial bool) {
	settings := getSettings()
	sem := make(chan struct{}, maxFeedConcurrency)
	var wg sync.WaitGroup
	for _, d := range fm.defs {
		if !d.Enabled(settings) {
			continue
		}
		lf := fm.loaded[d.ID]
		if !initial {
			if fetchedAt, loaded := lf.snapshot(); loaded && time.Since(fetchedAt) < d.TTL {
				continue
			}
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(d feedDef, lf *loadedFeed) {
			defer wg.Done()
			defer func() { <-sem }()
			fm.refreshOne(d, lf)
		}(d, lf)
	}
	wg.Wait()
}

func (fm *FeedManager) refreshOne(d feedDef, lf *loadedFeed) {
	switch d.Format {
	case feedFormatCIDRList:
		cidrs, err := fm.fetchCIDRList(d.URL)
		if err != nil {
			lf.mu.Lock()
			lf.lastErr = err
			lf.mu.Unlock()
			return
		}
		for _, extraURL := range d.ExtraURLs {
			more, err := fm.fetchCIDRList(extraURL)
			if err != nil {
				continue // one source being briefly down shouldn't drop the rest
			}
			cidrs = append(cidrs, more...)
		}
		lf.mu.Lock()
		lf.cidrs = cidrs
		lf.fetchedAt = time.Now()
		lf.lastErr = nil
		lf.count = len(cidrs)
		lf.mu.Unlock()
	default:
		exact, err := fm.fetchDomainList(d.URL, d.Format)
		if err != nil {
			lf.mu.Lock()
			lf.lastErr = err
			lf.mu.Unlock()
			return
		}
		for _, extraURL := range d.ExtraURLs {
			more, err := fm.fetchDomainList(extraURL, d.Format)
			if err != nil {
				continue // one category source being briefly down shouldn't drop the rest
			}
			for h := range more {
				exact[h] = struct{}{}
			}
		}
		var wildcard []string
		if d.WildcardURL != "" {
			if wc, err := fm.fetchDomainList(d.WildcardURL, feedFormatPlainList); err == nil {
				wildcard = make([]string, 0, len(wc))
				for h := range wc {
					wildcard = append(wildcard, h)
				}
			}
		}
		lf.mu.Lock()
		lf.exact = exact
		if wildcard != nil {
			lf.wildcard = wildcard
		}
		lf.fetchedAt = time.Now()
		lf.lastErr = nil
		lf.count = len(exact) + len(lf.wildcard)
		lf.mu.Unlock()
	}
}

// maxFeedBodyBytes caps a single feed download -- generous enough for any list in the
// curated set (largest is a few tens of MB) while bounding memory if a URL misbehaves.
const maxFeedBodyBytes = 64 << 20

func (fm *FeedManager) fetchBody(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "browser-gateway-ti-feeds/1.0 (+https://github.com/browser-gateway)")
	resp, err := fm.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s: http %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxFeedBodyBytes))
}

func newLineScanner(body []byte) *bufio.Scanner {
	sc := bufio.NewScanner(bytes.NewReader(body))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	return sc
}

func (fm *FeedManager) fetchCIDRList(rawURL string) ([]*net.IPNet, error) {
	body, err := fm.fetchBody(rawURL)
	if err != nil {
		return nil, err
	}
	var out []*net.IPNet
	sc := newLineScanner(body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		field := fields[0]
		if !strings.Contains(field, "/") {
			ip := net.ParseIP(field)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				field += "/32"
			} else {
				field += "/128"
			}
		}
		_, n, err := net.ParseCIDR(field)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

func (fm *FeedManager) fetchDomainList(rawURL string, format feedFormat) (map[string]struct{}, error) {
	body, err := fm.fetchBody(rawURL)
	if err != nil {
		return nil, err
	}
	out := map[string]struct{}{}
	sc := newLineScanner(body)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		var host string
		switch format {
		case feedFormatHostsFile:
			f := strings.Fields(line)
			if len(f) < 2 {
				continue
			}
			host = f[1]
		case feedFormatURLHost:
			f := strings.Fields(line)
			if len(f) == 0 {
				continue
			}
			u, err := url.Parse(f[0])
			if err != nil || u.Hostname() == "" {
				continue
			}
			host = u.Hostname()
		default: // feedFormatPlainList
			f := strings.Fields(line)
			if len(f) == 0 {
				continue
			}
			host = f[0]
		}
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		if host == "" || host == "localhost" || host == "0.0.0.0" || host == "broadcasthost" {
			continue
		}
		out[host] = struct{}{}
	}
	return out, nil
}

// feedProvider adapts one loaded feed to the providerLookup interface used by the rest of
// this package's aggregation logic. A domain feed only answers domain-kind lookups; an IP
// feed answers ip-kind lookups directly and also resolves a domain-kind lookup's hostname to
// IPs first (mirroring feodoProvider's behavior) so it still contributes to a domain check.
type feedProvider struct {
	def feedDef
	fm  *FeedManager
}

func (p feedProvider) ID() string { return p.def.ID }

func (p feedProvider) Lookup(ctx context.Context, kind Kind, indicator, _ string) (*Result, error) {
	lf := p.fm.loaded[p.def.ID]
	if _, loaded := lf.snapshot(); !loaded {
		return nil, ErrSkip
	}
	switch p.def.Kind {
	case KindDomain:
		if kind != KindDomain {
			return nil, ErrSkip
		}
		return p.result(kind, indicator, lf.matchDomain(indicator), ""), nil
	case KindIP:
		switch kind {
		case KindIP:
			ip := net.ParseIP(indicator)
			if ip == nil {
				return nil, ErrSkip
			}
			return p.result(kind, indicator, lf.matchIP(ip), indicator), nil
		case KindDomain:
			ips, err := net.DefaultResolver.LookupHost(ctx, indicator)
			if err != nil {
				return p.result(kind, indicator, false, ""), nil
			}
			for _, raw := range ips {
				ip := net.ParseIP(raw)
				if ip != nil && lf.matchIP(ip) {
					return p.result(kind, indicator, true, raw), nil
				}
			}
			return p.result(kind, indicator, false, ""), nil
		default:
			return nil, ErrSkip
		}
	default:
		return nil, ErrSkip
	}
}

func (p feedProvider) result(kind Kind, indicator string, hit bool, viaIP string) *Result {
	r := &Result{Provider: p.def.ID, Kind: string(kind), Indicator: indicator, Permalink: p.def.Permalink}
	if hit {
		r.Verdict = "malicious"
		r.Malicious = 1
		if viaIP != "" && viaIP != indicator {
			r.Detail = "resolved IP " + viaIP + " is listed in " + p.def.Name
		} else {
			r.Detail = "listed in " + p.def.Name
		}
	} else {
		r.Verdict = "clean"
		r.Harmless = 1
		r.Detail = "not listed in " + p.def.Name
	}
	return r
}
