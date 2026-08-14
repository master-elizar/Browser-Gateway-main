package workers

import (
	"context"
	"log"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/orchestrator"
	"github.com/browser-gateway/backend/internal/sessions"
	"gorm.io/gorm"
)

// StartReconcile cleans orphan containers and DB rows without containers.
func StartReconcile(db *gorm.DB, orch *orchestrator.Orchestrator, sess *sessions.Service, every time.Duration) {
	if every <= 0 {
		every = 60 * time.Second
	}
	go func() {
		// run once shortly after boot
		time.Sleep(5 * time.Second)
		reconcile(db, orch, sess)
		t := time.NewTicker(every)
		defer t.Stop()
		for range t.C {
			reconcile(db, orch, sess)
		}
	}()
}

func reconcile(db *gorm.DB, orch *orchestrator.Orchestrator, sess *sessions.Service) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	ids, err := orch.ListManagedContainerIDs(ctx)
	if err != nil {
		log.Printf("reconcile list: %v", err)
		return
	}
	known := map[string]struct{}{}
	for _, id := range ids {
		known[id] = struct{}{}
	}

	var rows []domain.BrowserSession
	_ = db.Where("status IN ?", []domain.SessionStatus{
		domain.StatusCreating, domain.StatusStarting, domain.StatusRunning, domain.StatusIdle, domain.StatusStopping,
	}).Find(&rows).Error

	for _, row := range rows {
		// A pcap sidecar (row.PcapContainerID) is also labeled bg.managed=true -- as long as
		// it's still present, mark it known too, or the orphan sweep below would destroy an
		// actively-capturing sidecar every cycle (it's never referenced by ContainerID).
		if row.PcapContainerID != "" {
			delete(known, row.PcapContainerID)
		}
		if row.ContainerID == "" {
			continue
		}
		if _, ok := known[row.ContainerID]; ok {
			delete(known, row.ContainerID)
			continue
		}
		// DB says active but container gone
		log.Printf("reconcile: marking missing container session %s destroyed", row.ID)
		now := time.Now()
		_ = db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Updates(map[string]any{
			"status":       domain.StatusDestroyed,
			"stopped_at":   now,
			"error_reason": "container missing (reconcile)",
		}).Error
	}

	// Orphan containers with no matching active session (browser or pcap sidecar)
	for cid := range known {
		var n int64
		_ = db.Model(&domain.BrowserSession{}).
			Where("(container_id = ? OR pcap_container_id = ?) AND status IN ?", cid, cid, []domain.SessionStatus{
				domain.StatusCreating, domain.StatusStarting, domain.StatusRunning, domain.StatusIdle, domain.StatusStopping,
			}).Count(&n).Error
		if n > 0 {
			continue
		}
		log.Printf("reconcile: destroying orphan container %s", cid[:12])
		_ = orch.Destroy(ctx, cid)
	}
	_ = sess // keep for future stop helpers
}
