package workers

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/orchestrator"
	"gorm.io/gorm"
)

// StartPcapSizeGuard periodically stops (only) the capture sidecar for any RUNNING session
// whose .pcap file has grown past settings.PcapMaxMB -- the browser session itself keeps
// running untouched, this only ends capture, bounding worst-case disk usage per session
// without vanilla tcpdump's own -C/-W rotation (which grows file *count* unboundedly instead
// of enforcing a hard total-size cap, and would leave multiple files to reassemble for one
// download instead of one clean file per session).
func StartPcapSizeGuard(db *gorm.DB, orch *orchestrator.Orchestrator, every time.Duration) {
	if every <= 0 {
		every = 2 * time.Minute
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for range t.C {
			enforcePcapSize(db, orch)
		}
	}()
}

func enforcePcapSize(db *gorm.DB, orch *orchestrator.Orchestrator) {
	var settings domain.AppSettings
	if err := db.First(&settings).Error; err != nil {
		return
	}
	maxMB := settings.PcapMaxMB
	if maxMB <= 0 {
		return
	}
	maxBytes := int64(maxMB) * 1024 * 1024

	var rows []domain.BrowserSession
	if err := db.Where("status = ? AND pcap_container_id <> ''", domain.StatusRunning).Find(&rows).Error; err != nil {
		return
	}
	for _, row := range rows {
		info, err := os.Stat(pcapPath(row.ID))
		if err != nil || info.Size() < maxBytes {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		err = orch.Destroy(ctx, row.PcapContainerID)
		cancel()
		if err != nil {
			log.Printf("pcap guard: stop sidecar for session %s: %v", row.ID, err)
			continue
		}
		_ = db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Update("pcap_container_id", "").Error
		log.Printf("pcap guard: capture for session %s reached %d MB cap, stopped (session unaffected)", row.ID, maxMB)
	}
}
