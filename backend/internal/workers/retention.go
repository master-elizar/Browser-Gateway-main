package workers

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"gorm.io/gorm"
)

// StartRetentionWorker deletes oldest audit/network/history when over budget,
// and purges history older than HistoryRetentionDays.
func StartRetentionWorker(db *gorm.DB, every time.Duration) {
	if every <= 0 {
		every = 5 * time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for range t.C {
			enforceHistoryAge(db)
			enforceRetention(db)
		}
	}()
}

func historyRoot() string {
	if v := os.Getenv("HISTORY_DIR"); v != "" {
		return v
	}
	marker := os.Getenv("UPDATE_MARKER_FILE")
	if marker == "" {
		marker = "/opt/browser-gateway/data/update.requested"
	}
	return filepath.Join(filepath.Dir(marker), "history")
}

func enforceHistoryAge(db *gorm.DB) {
	var settings domain.AppSettings
	if err := db.First(&settings).Error; err != nil {
		return
	}
	days := settings.HistoryRetentionDays
	if days <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	var old []domain.BrowserSession
	if err := db.Where("status = ? AND COALESCE(stopped_at, created_at) < ?", domain.StatusDestroyed, cutoff).
		Limit(50).Find(&old).Error; err != nil {
		return
	}
	for _, s := range old {
		_ = db.Where("session_id = ?", s.ID).Delete(&domain.SessionTimelineEvent{}).Error
		_ = db.Where("session_id = ?", s.ID).Delete(&domain.NetworkEvent{}).Error
		_ = db.Where("session_id = ?", s.ID).Delete(&domain.AuditEvent{}).Error
		_ = os.RemoveAll(filepath.Join(historyRoot(), s.ID))
		_ = db.Delete(&s).Error
		log.Printf("retention: purged history session %s (older than %d days)", s.ID, days)
	}
}

func enforceRetention(db *gorm.DB) {
	var settings domain.AppSettings
	if err := db.First(&settings).Error; err != nil || settings.RetentionBytes <= 0 {
		return
	}
	budget := settings.RetentionBytes

	for {
		var auditBytes, netBytes, histBytes int64
		_ = db.Raw(`SELECT COALESCE(SUM(pg_column_size(a)),0) FROM audit_events a`).Scan(&auditBytes).Error
		_ = db.Raw(`SELECT COALESCE(SUM(pg_column_size(n)),0) FROM network_events n`).Scan(&netBytes).Error
		_ = db.Raw(`SELECT COALESCE(SUM(pg_column_size(t)),0) FROM session_timeline_events t`).Scan(&histBytes).Error
		total := auditBytes + netBytes + histBytes
		if total <= budget {
			return
		}
		// Prefer deleting oldest timeline frames (and their files), then network, then audit.
		var frames []domain.SessionTimelineEvent
		if err := db.Order("created_at asc").Limit(50).Find(&frames).Error; err == nil && len(frames) > 0 {
			for _, f := range frames {
				if f.ScreenshotPath != "" {
					_ = os.Remove(f.ScreenshotPath)
				}
				_ = db.Delete(&f).Error
			}
			log.Printf("retention: trimmed %d timeline frames (approx used %d / %d)", len(frames), total, budget)
			continue
		}
		res := db.Exec(`DELETE FROM network_events WHERE id IN (
			SELECT id FROM network_events ORDER BY created_at ASC LIMIT 500
		)`)
		if res.Error != nil {
			log.Printf("retention network: %v", res.Error)
			return
		}
		if res.RowsAffected == 0 {
			res = db.Exec(`DELETE FROM audit_events WHERE id IN (
				SELECT id FROM audit_events ORDER BY created_at ASC LIMIT 200
			)`)
			if res.Error != nil || res.RowsAffected == 0 {
				log.Printf("retention: cannot shrink further (used≈%d budget=%d)", total, budget)
				return
			}
		}
		log.Printf("retention: trimmed rows (approx used %d / %d)", total, budget)
	}
}
