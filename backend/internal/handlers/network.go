package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/gofiber/fiber/v2"
)

type networkApplyReq struct {
	TURNURLs string `json:"turnUrls"`
}

type networkStatus struct {
	Pending  bool            `json:"pending"`
	Progress *updateProgress `json:"progress,omitempty"`
}

func (h *Handler) networkMarkerPath() string {
	marker := h.cfg.NetworkMarkerFile
	if marker == "" {
		return "/opt/browser-gateway/data/network.requested"
	}
	return marker
}

func (h *Handler) networkProgressPath() string {
	return filepath.Join(filepath.Dir(h.networkMarkerPath()), "network.progress")
}

func (h *Handler) writeNetworkProgress(p *updateProgress) error {
	if p == nil {
		return nil
	}
	path := h.networkProgressPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o664)
}

func (h *Handler) readNetworkProgress() *updateProgress {
	b, err := os.ReadFile(h.networkProgressPath())
	if err != nil {
		return nil
	}
	var p updateProgress
	if err := json.Unmarshal(b, &p); err != nil {
		return nil
	}
	if p.Phase == "" && p.Percent == 0 && !p.Done {
		return nil
	}
	return &p
}

// AdminApplyNetwork requests the host apply a new TURN_URLS value: writes a marker file that
// a host-side systemd path unit picks up (same request/apply/progress protocol as updates.go's
// self-update flow), edits deploy/.env, and recreates the backend so it hands the new ICE
// server address(es) to clients. Cannot be done from inside this container -- deploy/.env is
// read by docker-compose on the host, not mounted into any container.
func (h *Handler) AdminApplyNetwork(c *fiber.Ctx) error {
	var req networkApplyReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	urls := strings.TrimSpace(req.TURNURLs)
	if urls == "" {
		return fiber.NewError(fiber.StatusBadRequest, "turnUrls is required")
	}
	for _, u := range strings.Split(urls, ",") {
		u = strings.TrimSpace(u)
		if u == "" || (!strings.HasPrefix(u, "turn:") && !strings.HasPrefix(u, "turns:")) {
			return fiber.NewError(fiber.StatusBadRequest, "each entry must start with turn: or turns: (got "+u+")")
		}
	}

	actor := auth.CurrentUser(c)
	marker := h.networkMarkerPath()
	if err := os.MkdirAll(filepath.Dir(marker), 0o775); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Remove first so systemd PathChanged re-fires even if a previous request stuck.
	_ = os.Remove(marker)
	body := fmt.Sprintf(
		"requestedAt=%s\nby=%s\nturnUrls=%s\n",
		time.Now().UTC().Format(time.RFC3339),
		actor.Email,
		urls,
	)
	if err := os.WriteFile(marker, []byte(body), 0o664); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "cannot write network marker: "+err.Error())
	}
	_ = h.writeNetworkProgress(&updateProgress{
		Percent:   2,
		Phase:     "queued",
		Message:   "Waiting for host apply…",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Done:      false,
	})
	_ = h.auth.WriteAudit(actor.ID, "admin.network.request", "TURN_URLS change requested: "+urls)
	return c.JSON(fiber.Map{
		"ok":      true,
		"message": "Network settings requested. The host will update deploy/.env and restart the backend shortly.",
	})
}

// AdminNetworkStatus reports progress of a pending AdminApplyNetwork request. The frontend
// polls this while the backend container itself may be mid-restart -- once it's back up,
// the freshly-recreated process picks up right where the previous one left off reading this
// same file from the shared data/ volume.
func (h *Handler) AdminNetworkStatus(c *fiber.Ctx) error {
	p := h.readNetworkProgress()
	return c.JSON(networkStatus{
		Pending:  fileExists(h.networkMarkerPath()),
		Progress: p,
	})
}
