package auth

import (
	"testing"
)

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "correct-horse-battery") {
		t.Fatal("expected verify ok")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("expected verify fail")
	}
}
