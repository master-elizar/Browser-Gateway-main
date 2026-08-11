package workers

import (
	"context"
	"log"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/sessions"
	"gorm.io/gorm"
)

// StartPolicyWorker enforces idle timeout and max session duration.
func StartPolicyWorker(db *gorm.DB, sess *sessions.Service, every time.Duration) {
	if every <= 0 {
		every = 30 * time.Second
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for range t.C {
			enforcePolicies(db, sess)
		}
	}()
}

func enforcePolicies(db *gorm.DB, sess *sessions.Service) {
	var settings domain.AppSettings
	if err := db.First(&settings).Error; err != nil {
		return
	}
	now := time.Now()

	// Max duration hard stop
	if settings.MaxSessionDurationSec > 0 {
		cutoff := now.Add(-time.Duration(settings.MaxSessionDurationSec) * time.Second)
		var rows []domain.BrowserSession
		_ = db.Where("status IN ? AND started_at IS NOT NULL AND started_at < ?",
			[]domain.SessionStatus{domain.StatusRunning, domain.StatusIdle}, cutoff).
			Find(&rows).Error
		for _, row := range rows {
			log.Printf("policy: max duration stop session %s", row.ID)
			systemStop(db, sess, row.ID, "max session duration exceeded")
		}
	}

	// Idle: no viewer activity
	if settings.IdleTimeoutSec > 0 {
		idleCut := now.Add(-time.Duration(settings.IdleTimeoutSec) * time.Second)
		var rows []domain.BrowserSession
		_ = db.Where("status = ? AND ((last_active_at IS NOT NULL AND last_active_at < ?) OR (last_active_at IS NULL AND started_at IS NOT NULL AND started_at < ?))",
			domain.StatusRunning, idleCut, idleCut).
			Find(&rows).Error
		for _, row := range rows {
			_ = db.Model(&domain.BrowserSession{}).Where("id = ? AND status = ?", row.ID, domain.StatusRunning).
				Update("status", domain.StatusIdle).Error
			log.Printf("policy: session %s → IDLE", row.ID)
		}

		// Stop sessions that stayed IDLE past another idle window (or half)
		stopIdleCut := now.Add(-time.Duration(settings.IdleTimeoutSec) * time.Second)
		var idleRows []domain.BrowserSession
		_ = db.Where("status = ? AND updated_at < ?", domain.StatusIdle, stopIdleCut).Find(&idleRows).Error
		for _, row := range idleRows {
			log.Printf("policy: idle stop session %s", row.ID)
			systemStop(db, sess, row.ID, "idle timeout")
		}
	}
}

func systemStop(db *gorm.DB, sess *sessions.Service, id, reason string) {
	var row domain.BrowserSession
	if err := db.First(&row, "id = ?", id).Error; err != nil {
		return
	}
	admin := &domain.User{ID: row.OwnerID, Role: domain.RoleSuperAdmin}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _ = sess.Stop(ctx, admin, id)
	_ = db.Model(&domain.BrowserSession{}).Where("id = ?", id).Update("error_reason", reason).Error
}
