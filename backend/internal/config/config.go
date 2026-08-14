package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv   string
	HTTPAddr string

	DatabaseURL string
	RedisURL    string

	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	BootstrapAdminEmail string
	BootstrapAdminPass  string

	DockerHost          string
	BrowserImage        string // chromium engine (compat: BROWSER_IMAGE)
	BrowserImageFirefox string
	BrowserNetwork      string
	MaxSessionsGlobal   int
	MaxSessionsPerUser  int
	SessionIdleTimeout  time.Duration
	SessionMaxDuration  time.Duration
	InternalAgentSecret string
	BrowserMemoryMB     int
	BrowserCPUs         float64
	TURNURLs           string
	TURNUsername       string
	TURNPassword       string
	CertsDir           string
	TraefikContainer   string
	SetupKeyFile       string
	UpdateMarkerFile   string
	NetworkMarkerFile  string
	GitHubRepo         string
	// PcapImage is the sidecar packet-capture container image (see /pcap-agent) -- joined to
	// a session's browser container via NetworkMode: container:<id> so only this narrow,
	// capability-minimal helper ever needs CAP_NET_RAW, never the browser-engine container
	// itself.
	PcapImage string
	// PcapDir mirrors historyRoot()'s derive-from-the-shared-data-volume convention
	// (internal/workers/retention.go) rather than introducing a second base-path knob. This
	// is the path as seen INSIDE the backend container (used for os.Stat/os.Remove here).
	PcapDir string
	// PcapHostDir is the same directory's path on the Docker HOST, not inside this
	// container -- required because the orchestrator bind-mounts it into a container *it*
	// creates (the pcap sidecar) via the Docker Engine API, which always resolves bind
	// sources against the host filesystem. Set via docker-compose.yml from HOST_DATA_DIR
	// (see deploy/.env.example) -- only diverges from PcapDir when installed somewhere
	// other than the default /opt/browser-gateway.
	PcapHostDir string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:              getEnv("APP_ENV", "development"),
		HTTPAddr:            getEnv("HTTP_ADDR", ":8080"),
		DatabaseURL:         getEnv("DATABASE_URL", "postgres://bg:bg@postgres:5432/browser_gateway?sslmode=disable"),
		RedisURL:            getEnv("REDIS_URL", "redis://redis:6379/0"),
		JWTSecret:           getEnv("JWT_SECRET", ""),
		AccessTokenTTL:      getDurationEnv("ACCESS_TOKEN_TTL", 15*time.Minute),
		RefreshTokenTTL:     getDurationEnv("REFRESH_TOKEN_TTL", 168*time.Hour),
		BootstrapAdminEmail: getEnv("BOOTSTRAP_ADMIN_EMAIL", ""),
		BootstrapAdminPass:  getEnv("BOOTSTRAP_ADMIN_PASSWORD", ""),
		DockerHost:          getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		BrowserImage:        getEnv("BROWSER_IMAGE", "browser-gateway/browser-engine:local"),
		BrowserImageFirefox: getEnv("BROWSER_IMAGE_FIREFOX", "browser-gateway/browser-engine-firefox:local"),
		BrowserNetwork:      getEnv("BROWSER_NETWORK", "browser-gateway_browser-net"),
		MaxSessionsGlobal:   getIntEnv("MAX_SESSIONS_GLOBAL", 15),
		MaxSessionsPerUser:  getIntEnv("MAX_SESSIONS_PER_USER", 3),
		SessionIdleTimeout:  getDurationEnv("SESSION_IDLE_TIMEOUT", 30*time.Minute),
		SessionMaxDuration:  getDurationEnv("SESSION_MAX_DURATION", 4*time.Hour),
		InternalAgentSecret: getEnv("INTERNAL_AGENT_SECRET", ""),
		BrowserMemoryMB:     getIntEnv("BROWSER_MEMORY_MB", 1536),
		BrowserCPUs:         getFloatEnv("BROWSER_CPUS", 1.5),
		TURNURLs:            getEnv("TURN_URLS", "turn:localhost:3478"),
		TURNUsername:        getEnv("TURN_USERNAME", "bg"),
		TURNPassword:        getEnv("TURN_PASSWORD", "bgturn"),
		CertsDir:            getEnv("CERTS_DIR", "/opt/browser-gateway/data/certs"),
		TraefikContainer:    getEnv("TRAEFIK_CONTAINER_NAME", "browser-gateway-traefik-1"),
		SetupKeyFile:        getEnv("SETUP_KEY_FILE", "/opt/browser-gateway/data/setup.bootstrap"),
		UpdateMarkerFile:    getEnv("UPDATE_MARKER_FILE", "/opt/browser-gateway/data/update.requested"),
		NetworkMarkerFile:   getEnv("NETWORK_MARKER_FILE", "/opt/browser-gateway/data/network.requested"),
		GitHubRepo:          getEnv("GITHUB_REPO", "master-elizar/Browser-Gateway-main"),
	}
	cfg.PcapImage = getEnv("PCAP_IMAGE", "browser-gateway/pcap-agent:local")
	cfg.PcapDir = getEnv("PCAP_DIR", filepath.Join(filepath.Dir(cfg.UpdateMarkerFile), "pcaps"))
	// Falls back to PcapDir (the in-container path) so a deploy that hasn't picked up the
	// HOST_DATA_DIR env var yet still works when installed at the default location --
	// wrong only for a customized, not-yet-updated BG_INSTALL_DIR, same failure mode as any
	// other env var this codebase hasn't been told about yet.
	cfg.PcapHostDir = getEnv("PCAP_HOST_DIR", cfg.PcapDir)

	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getFloatEnv(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
