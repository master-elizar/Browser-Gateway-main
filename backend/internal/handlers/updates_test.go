package handlers

import (
	"testing"
	"time"
)

func TestVersionNewer(t *testing.T) {
	if !versionNewer("0.12.1", "0.12.0") {
		t.Fatal("expected 0.12.1 > 0.12.0")
	}
	if versionNewer("0.12.0", "0.12.1") {
		t.Fatal("expected 0.12.0 not > 0.12.1")
	}
	if versionNewer("0.12.0", "0.12.0") {
		t.Fatal("equal should be false")
	}
	if !versionNewer("v1.0.0", "0.9.5") {
		t.Fatal("expected v1.0.0 > 0.9.5")
	}
}

func TestSameCommit(t *testing.T) {
	if !sameCommit("5d169a8b26ea", "5d169a8b26eacb0646eee70260697bf93e2d5278") {
		t.Fatal("prefix should match")
	}
	if sameCommit("abc", "def") {
		t.Fatal("different commits")
	}
	if sameCommit("", "abc") {
		t.Fatal("empty should not match")
	}
	if !sameCommit("9bcb68b2b004", "9bcb68b2b004216636ac2ef4b998192a6473ef38") {
		t.Fatal("short installed sha should match remote")
	}
}

func TestVersionNewerEqual(t *testing.T) {
	if versionNewer("0.12.1", "0.12.1") {
		t.Fatal("equal versions must not be newer")
	}
	if !versionNewer("0.12.2", "0.12.1") {
		t.Fatal("0.12.2 should be newer than 0.12.1")
	}
}

func TestReleaseTagCurrent(t *testing.T) {
	if !releaseTagCurrent("0.16.5", "0.16.5") {
		t.Fatal("equal release should be current")
	}
	if releaseTagCurrent("0.16.6", "0.16.5") {
		t.Fatal("newer release is not current")
	}
	if releaseTagCurrent("", "0.16.5") {
		t.Fatal("empty tag is not current")
	}
	if releaseTagCurrent("main@abc", "0.16.5") {
		t.Fatal("main tip label is not a release current check")
	}
}

func TestProgressStale(t *testing.T) {
	if !progressStale(&updateProgress{Percent: 10, Phase: "build", Done: false}) {
		t.Fatal("missing updatedAt should be stale")
	}
	fresh := &updateProgress{
		Percent:   10,
		Phase:     "build",
		Done:      false,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if progressStale(fresh) {
		t.Fatal("fresh progress should not be stale")
	}
	old := &updateProgress{
		Percent:   10,
		Phase:     "build",
		Done:      false,
		UpdatedAt: time.Now().UTC().Add(-time.Hour).Format(time.RFC3339),
	}
	if !progressStale(old) {
		t.Fatal("old progress should be stale")
	}
}
