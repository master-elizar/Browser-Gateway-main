package ti

import (
	"errors"
	"strings"
	"testing"
)

// TestNormalizeKindRejectsDataURI is a regression test for a real production incident: a
// data:image/png;base64,... favicon URL reached NormalizeKind (auto-detected as "domain"
// since it doesn't contain "://"), hostname extraction silently failed, and the entire
// multi-KB base64 string was returned unchanged as the "indicator" -- which then blew past
// PostgreSQL's btree index row-size limit on the very first cache write.
func TestNormalizeKindRejectsDataURI(t *testing.T) {
	dataURI := "data:image/png;base64," + strings.Repeat("iVBORw0KGgo", 500)

	if _, _, err := NormalizeKind("", dataURI); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("auto-detect kind: expected ErrInvalidIndicator, got %v", err)
	}
	if _, _, err := NormalizeKind("domain", dataURI); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("explicit domain kind: expected ErrInvalidIndicator, got %v", err)
	}
	if _, _, err := NormalizeKind("url", dataURI); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("explicit url kind: expected ErrInvalidIndicator, got %v", err)
	}
}

func TestNormalizeKindRejectsOverlongValue(t *testing.T) {
	huge := strings.Repeat("a", maxIndicatorLen+1)
	if _, _, err := NormalizeKind("", huge); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("expected ErrInvalidIndicator for overlong value, got %v", err)
	}
}

func TestNormalizeKindRejectsGarbageWithExplicitKind(t *testing.T) {
	if _, _, err := NormalizeKind("ip", "not-an-ip"); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("explicit ip kind with garbage: expected ErrInvalidIndicator, got %v", err)
	}
	if _, _, err := NormalizeKind("hash", "not-a-hash"); !errors.Is(err, ErrInvalidIndicator) {
		t.Fatalf("explicit hash kind with garbage: expected ErrInvalidIndicator, got %v", err)
	}
}

// TestNormalizeKindStillAcceptsRealIndicators guards against the new validation being too
// strict and rejecting legitimate input.
func TestNormalizeKindStillAcceptsRealIndicators(t *testing.T) {
	cases := []struct {
		kind, value, wantKind, wantValue string
	}{
		{"", "example.com", "domain", "example.com"},
		{"domain", "WWW.Example.com.", "domain", "www.example.com"},
		{"", "https://example.com/path", "url", "https://example.com/path"},
		{"", "1.2.3.4", "ip", "1.2.3.4"},
		{"ip", "2001:db8::1", "ip", "2001:db8::1"},
		{"", strings.Repeat("a", 32), "hash", strings.Repeat("a", 32)},
	}
	for _, c := range cases {
		k, v, err := NormalizeKind(c.kind, c.value)
		if err != nil {
			t.Fatalf("NormalizeKind(%q, %q): unexpected error: %v", c.kind, c.value, err)
		}
		if string(k) != c.wantKind || v != c.wantValue {
			t.Fatalf("NormalizeKind(%q, %q) = (%q, %q), want (%q, %q)", c.kind, c.value, k, v, c.wantKind, c.wantValue)
		}
	}
}

func TestFilterPlausibleDomainsDropsGarbage(t *testing.T) {
	garbage := "data:image/png;base64," + strings.Repeat("A", 3000)
	got := filterPlausibleDomains([]string{garbage, "Example.COM", "sub.example.org"})
	want := []string{"example.com", "sub.example.org"}
	if len(got) != len(want) {
		t.Fatalf("filterPlausibleDomains = %v, want %v (garbage indicator leaked through)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("filterPlausibleDomains = %v, want %v", got, want)
		}
	}
}
