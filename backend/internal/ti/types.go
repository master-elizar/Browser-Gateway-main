package ti

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDisabled    = errors.New("threat intelligence is disabled")
	ErrNoAPIKey    = errors.New("threat intelligence API key is not configured")
	ErrNoProviders = errors.New("no threat intelligence providers enabled")
	ErrUnsupported = errors.New("unsupported indicator kind")
	ErrRateLimited = errors.New("threat intelligence rate limited")
	ErrSkip        = errors.New("provider skipped for this indicator")
)

type Kind string

const (
	KindDomain Kind = "domain"
	KindIP     Kind = "ip"
	KindURL    Kind = "url"
	KindHash   Kind = "hash" // MD5/SHA1/SHA256 file hash (MalwareBazaar)
)

// Result is a single-provider or aggregated multi-provider lookup.
type Result struct {
	Provider   string    `json:"provider"`
	Kind       string    `json:"kind"`
	Indicator  string    `json:"indicator"`
	Verdict    string    `json:"verdict"` // clean | suspicious | malicious | unknown
	Malicious  int       `json:"malicious"`
	Suspicious int       `json:"suspicious"`
	Harmless   int       `json:"harmless"`
	Undetected int       `json:"undetected"`
	// Detail is a short human-readable summary of this provider's actual response (e.g.
	// "16/94 engines flagged", "listed in 4 pulses", "3 open ports, no risk tags") -- the
	// Malicious/Suspicious/Harmless/Undetected ints mean different things per provider
	// (engine counts for VirusTotal, pulse counts for OTX, ...) so they alone aren't enough
	// for an advanced/per-source UI view.
	Detail string `json:"detail,omitempty"`
	// Informational marks providers that don't assert a malicious/clean verdict at all
	// (Shodan, crt.sh) -- they're shown as context in an advanced view but must not count
	// toward a malicious/total-sources ratio.
	Informational bool      `json:"informational,omitempty"`
	Permalink     string    `json:"permalink,omitempty"`
	Cached        bool      `json:"cached"`
	CheckedAt     time.Time `json:"checkedAt"`
	Error         string    `json:"error,omitempty"`
	Providers     []Result  `json:"providers,omitempty"`
}

type providerLookup interface {
	ID() string
	// Lookup returns ErrSkip when the provider does not apply to this kind.
	Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error)
}
