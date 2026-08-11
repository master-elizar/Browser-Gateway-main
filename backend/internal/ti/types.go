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
	Permalink  string    `json:"permalink,omitempty"`
	Cached     bool      `json:"cached"`
	CheckedAt  time.Time `json:"checkedAt"`
	Error      string    `json:"error,omitempty"`
	Providers  []Result  `json:"providers,omitempty"`
}

type providerLookup interface {
	ID() string
	// Lookup returns ErrSkip when the provider does not apply to this kind.
	Lookup(ctx context.Context, kind Kind, indicator, apiKey string) (*Result, error)
}
