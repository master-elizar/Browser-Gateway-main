package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func Open(databaseURL, redisURL string) (*Store, error) {
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Store{DB: db, Redis: rdb}, nil
}

func (s *Store) AutoMigrate() error {
	return s.DB.AutoMigrate(
		&domain.User{},
		&domain.BrowserSession{},
		&domain.AppSettings{},
		&domain.AuditEvent{},
		&domain.RefreshToken{},
		&domain.NetworkEvent{},
		&domain.TICacheEntry{},
		&domain.SessionTimelineEvent{},
		&domain.UserTIKey{},
	)
}

func (s *Store) EnsureDefaultSettings(cfgMaxGlobal, cfgMaxPerUser, idleSec, maxDurSec int) error {
	var count int64
	if err := s.DB.Model(&domain.AppSettings{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	settings := domain.AppSettings{
		MaxConcurrentSessionsGlobal:  cfgMaxGlobal,
		MaxConcurrentSessionsPerUser: cfgMaxPerUser,
		IdleTimeoutSec:               idleSec,
		MaxSessionDurationSec:        maxDurSec,
		RetentionBytes:               10 * 1024 * 1024 * 1024,
		LogSessionLifecycle:          true,
		LogControlActions:            true,
		LogVisitedURLs:               true,
		LogDownloads:                 true,
		LogNetworkDNS:                true,
		LogNetworkHTTP:               true,
		LogKeystrokes:                false,
		AllowRegistration:            false,
		PasswordMinLength:            8,
		PasswordRequireComplexity:    false,
		DnsMode:                      "docker",
		DnsServers:                   "8.8.8.8,1.1.1.1",
		DnsDohUrl:                    "https://cloudflare-dns.com/dns-query",
		HistoryRetentionDays:         30,
		PcapEnabled:                  true,
		PcapMaxMB:                    500,
		PcapUIVersion:                1,
	}
	return s.DB.Create(&settings).Error
}

// BackfillDNSDefaults fills empty DNS fields on the singleton settings row.
func (s *Store) BackfillDNSDefaults() error {
	var settings domain.AppSettings
	if err := s.DB.First(&settings).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if settings.DnsMode == "" {
		updates["dns_mode"] = "docker"
	}
	if settings.DnsServers == "" {
		updates["dns_servers"] = "8.8.8.8,1.1.1.1"
	}
	if settings.DnsDohUrl == "" {
		updates["dns_doh_url"] = "https://cloudflare-dns.com/dns-query"
	}
	if len(updates) == 0 {
		return nil
	}
	return s.DB.Model(&settings).Updates(updates).Error
}

// BackfillTIDefaults enables VirusTotal when a legacy API key exists without Stage-14 flags.
func (s *Store) BackfillTIDefaults() error {
	var settings domain.AppSettings
	if err := s.DB.First(&settings).Error; err != nil {
		return err
	}
	updates := map[string]any{}
	if strings.TrimSpace(settings.TiAPIKey) != "" && !settings.TiVirusTotalEnabled &&
		!settings.TiURLHausEnabled && !settings.TiThreatFoxEnabled &&
		!settings.TiAbuseIPDBEnabled && !settings.TiOTXEnabled && !settings.TiSpamhausEnabled {
		updates["ti_virustotal_enabled"] = true
	}
	if settings.TiProvider == "" {
		updates["ti_provider"] = "multi"
	}
	if len(updates) == 0 {
		return nil
	}
	return s.DB.Model(&settings).Updates(updates).Error
}

// BackfillViewerUIDefaults turns all session-viewer controls on once for existing installs.
func (s *Store) BackfillViewerUIDefaults() error {
	var settings domain.AppSettings
	if err := s.DB.First(&settings).Error; err != nil {
		return err
	}
	if settings.ViewerUIVersion >= 1 {
		return nil
	}
	return s.DB.Model(&settings).Updates(map[string]any{
		"viewer_ui_version":        1,
		"viewer_webrtc_enabled":    true,
		"viewer_novnc_enabled":     true,
		"viewer_fit_enabled":       true,
		"viewer_stretch_enabled":   true,
		"viewer_clipboard_enabled": true,
		"viewer_upload_enabled":    true,
		"viewer_downloads_enabled": true,
		"viewer_network_enabled":   true,
	}).Error
}

// BackfillFeedDefaults turns every local bulk threat-intel feed on once for existing
// installs, matching the "works out of the box, no key setup" goal — mirrors
// BackfillViewerUIDefaults's version-guard pattern so it never re-enables a feed a user
// deliberately turned back off.
func (s *Store) BackfillFeedDefaults() error {
	var settings domain.AppSettings
	if err := s.DB.First(&settings).Error; err != nil {
		return err
	}
	if settings.FeedsUIVersion >= 1 {
		return nil
	}
	return s.DB.Model(&settings).Updates(map[string]any{
		"feeds_ui_version":                 1,
		"ti_feed_phishingdb_enabled":       true,
		"ti_feed_openphish_enabled":        true,
		"ti_feed_blocklistproject_enabled": true,
		"ti_feed_hagezi_enabled":           true,
		"ti_feed_ipsum_enabled":            true,
		"ti_feed_firehol_enabled":          true,
		"ti_feed_blocklistde_enabled":      true,
		"ti_feed_spamhausdrop_enabled":     true,
		"ti_feed_cinsarmy_enabled":         true,
		"ti_feed_etcompromised_enabled":    true,
		"ti_feed_greensnow_enabled":        true,
		"ti_circlhashlookup_enabled":       true,
	}).Error
}

// BackfillPcapDefaults turns PCAP capture on once for existing installs, matching the
// "automatic for every session" default -- same version-guard pattern as
// BackfillViewerUIDefaults/BackfillFeedDefaults.
func (s *Store) BackfillPcapDefaults() error {
	var settings domain.AppSettings
	if err := s.DB.First(&settings).Error; err != nil {
		return err
	}
	if settings.PcapUIVersion >= 1 {
		return nil
	}
	updates := map[string]any{
		"pcap_ui_version": 1,
		"pcap_enabled":    true,
	}
	if settings.PcapMaxMB <= 0 {
		updates["pcap_max_mb"] = 500
	}
	return s.DB.Model(&settings).Updates(updates).Error
}

func (s *Store) Ping(ctx context.Context) error {
	sqlDB, err := s.DB.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis: %w", err)
	}
	return nil
}

func (s *Store) Close() error {
	if s.Redis != nil {
		_ = s.Redis.Close()
	}
	if s.DB != nil {
		sqlDB, err := s.DB.DB()
		if err == nil {
			return sqlDB.Close()
		}
	}
	return nil
}
