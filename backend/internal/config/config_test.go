package config_test

import (
	"testing"

	"github.com/browser-gateway/backend/internal/config"
)

func TestLoadRequiresJWTSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when JWT_SECRET missing")
	}
}

func TestLoadOK(t *testing.T) {
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("HTTP_ADDR", ":9090")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Fatalf("addr=%s", cfg.HTTPAddr)
	}
}
