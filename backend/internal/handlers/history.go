package handlers

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func (h *Handler) historyRoot() string {
	base := filepath.Dir(h.updateMarkerPath())
	return filepath.Join(base, "history")
}

func (h *Handler) canAccessHistorySession(user *domain.User, sess *domain.BrowserSession) bool {
	if user == nil || sess == nil {
		return false
	}
	if user.Role == domain.RoleSuperAdmin {
		return true
	}
	return sess.OwnerID == user.ID
}

// InternalHistoryFrame accepts a JPEG screenshot from the session agent.
func (h *Handler) InternalHistoryFrame(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	sess, err := h.sessions.AuthenticateAgent(sessionID, agentTokenFromRequest(c))
	if err != nil {
		return mapSessionErr(err)
	}
	var req struct {
		Kind       string         `json:"kind"`
		URL        string         `json:"url"`
		ImageBase64 string        `json:"imageBase64"`
		Meta       map[string]any `json:"meta"`
		TS         string         `json:"ts"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "click"
	}
	if req.ImageBase64 == "" {
		return fiber.NewError(fiber.StatusBadRequest, "imageBase64 required")
	}
	raw := req.ImageBase64
	if i := strings.Index(raw, ","); i >= 0 {
		raw = raw[i+1:]
	}
	img, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid image")
	}
	var count int64
	_ = h.st.DB.Model(&domain.SessionTimelineEvent{}).Where("session_id = ?", sessionID).Count(&count).Error
	if count >= 500 {
		return c.JSON(fiber.Map{"ok": true, "skipped": true, "reason": "cap"})
	}

	dir := filepath.Join(h.historyRoot(), sessionID)
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	id := uuid.NewString()
	ts := time.Now().UTC()
	if req.TS != "" {
		if t, e := time.Parse(time.RFC3339, req.TS); e == nil {
			ts = t
		}
	}
	fname := ts.Format("20060102T150405.000") + "_" + kind + "_" + id[:8] + ".jpg"
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, img, 0o664); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	meta := ""
	if req.Meta != nil {
		b, _ := json.Marshal(req.Meta)
		meta = string(b)
	}
	row := domain.SessionTimelineEvent{
		ID:             id,
		SessionID:      sessionID,
		UserID:         sess.OwnerID,
		Kind:           kind,
		URL:            req.URL,
		ScreenshotPath: path,
		MetaJSON:       meta,
		CreatedAt:      ts,
	}
	if err := h.st.DB.Create(&row).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true, "id": id})
}

type historyListItem struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Browser      string     `json:"browser"`
	Status       string     `json:"status"`
	StartURL     string     `json:"startUrl,omitempty"`
	OwnerID      string     `json:"ownerId"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	StoppedAt    *time.Time `json:"stoppedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	FrameCount   int64      `json:"frameCount"`
	TiVerdict    string     `json:"tiVerdict,omitempty"`
	DurationSec  int64      `json:"durationSec,omitempty"`
}

func (h *Handler) HistoryList(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	q := h.st.DB.Model(&domain.BrowserSession{}).Where("status = ?", domain.StatusDestroyed)
	if user.Role != domain.RoleSuperAdmin {
		q = q.Where("owner_id = ?", user.ID)
	}
	if v := c.Query("name"); v != "" {
		q = q.Where("name ILIKE ?", "%"+v+"%")
	}
	if v := c.Query("browser"); v != "" {
		q = q.Where("browser = ?", v)
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("COALESCE(stopped_at, created_at) >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("COALESCE(stopped_at, created_at) <= ?", t)
		}
	}
	var rows []domain.BrowserSession
	if err := q.Order("COALESCE(stopped_at, created_at) desc").Limit(200).Find(&rows).Error; err != nil {
		return err
	}
	tiFilter := strings.ToLower(c.Query("tiVerdict"))
	items := make([]historyListItem, 0, len(rows))
	for _, r := range rows {
		var frames int64
		_ = h.st.DB.Model(&domain.SessionTimelineEvent{}).Where("session_id = ?", r.ID).Count(&frames).Error
		verdict := h.worstTIVerdict(r.ID)
		if tiFilter != "" && !strings.Contains(strings.ToLower(verdict), tiFilter) {
			continue
		}
		item := historyListItem{
			ID:         r.ID,
			Name:       r.Name,
			Browser:    r.Browser,
			Status:     string(r.Status),
			StartURL:   r.StartURL,
			OwnerID:    r.OwnerID,
			StartedAt:  r.StartedAt,
			StoppedAt:  r.StoppedAt,
			CreatedAt:  r.CreatedAt,
			FrameCount: frames,
			TiVerdict:  verdict,
		}
		if r.StartedAt != nil && r.StoppedAt != nil {
			item.DurationSec = int64(r.StoppedAt.Sub(*r.StartedAt).Seconds())
		}
		items = append(items, item)
	}
	return c.JSON(fiber.Map{"items": items, "total": len(items)})
}

func (h *Handler) worstTIVerdict(sessionID string) string {
	var rows []domain.NetworkEvent
	_ = h.st.DB.Where("session_id = ? AND type = ?", sessionID, "ti").Order("created_at desc").Limit(50).Find(&rows).Error
	rank := map[string]int{"malicious": 4, "suspicious": 3, "unknown": 2, "harmless": 1, "clean": 1}
	best := ""
	bestR := 0
	for _, r := range rows {
		var p map[string]any
		_ = json.Unmarshal([]byte(r.Payload), &p)
		v, _ := p["verdict"].(string)
		v = strings.ToLower(v)
		if rank[v] > bestR {
			bestR = rank[v]
			best = v
		}
	}
	return best
}

func (h *Handler) HistoryGet(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	id := c.Params("id")
	var sess domain.BrowserSession
	if err := h.st.DB.First(&sess, "id = ?", id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "session not found")
	}
	if !h.canAccessHistorySession(user, &sess) {
		return fiber.NewError(fiber.StatusForbidden, "forbidden")
	}

	var frames []domain.SessionTimelineEvent
	_ = h.st.DB.Where("session_id = ?", id).Order("created_at asc").Find(&frames).Error
	frameItems := make([]map[string]any, 0, len(frames))
	for _, f := range frames {
		item := map[string]any{
			"id":        f.ID,
			"kind":      f.Kind,
			"url":       f.URL,
			"createdAt": f.CreatedAt,
			"hasImage":  f.ScreenshotPath != "",
			"type":      "frame",
		}
		if f.MetaJSON != "" {
			var meta map[string]any
			_ = json.Unmarshal([]byte(f.MetaJSON), &meta)
			item["meta"] = meta
		}
		frameItems = append(frameItems, item)
	}

	var nets []domain.NetworkEvent
	_ = h.st.DB.Where("session_id = ?", id).Order("created_at asc").Limit(2000).Find(&nets).Error
	netItems := make([]map[string]any, 0, len(nets))
	for _, n := range nets {
		var p map[string]any
		_ = json.Unmarshal([]byte(n.Payload), &p)
		if p == nil {
			p = map[string]any{}
		}
		p["id"] = n.ID
		p["type"] = n.Type
		p["createdAt"] = n.CreatedAt
		p["timelineType"] = "network"
		netItems = append(netItems, p)
	}

	var audits []domain.AuditEvent
	_ = h.st.DB.Where("session_id = ?", id).Order("created_at asc").Limit(500).Find(&audits).Error
	auditItems := make([]map[string]any, 0, len(audits))
	for _, a := range audits {
		auditItems = append(auditItems, map[string]any{
			"id":           a.ID,
			"type":         a.Type,
			"message":      a.Message,
			"createdAt":    a.CreatedAt,
			"timelineType": "audit",
		})
	}

	var netTaintDomains []string
	if sess.NetTaintDomains != "" {
		netTaintDomains = strings.Split(sess.NetTaintDomains, ",")
	}
	return c.JSON(fiber.Map{
		"session": fiber.Map{
			"id":              sess.ID,
			"name":            sess.Name,
			"browser":         sess.Browser,
			"status":          sess.Status,
			"startUrl":        sess.StartURL,
			"ownerId":         sess.OwnerID,
			"startedAt":       sess.StartedAt,
			"stoppedAt":       sess.StoppedAt,
			"createdAt":       sess.CreatedAt,
			"dnsMode":         sess.DnsMode,
			"memoryMb":        sess.MemoryMB,
			"cpus":            sess.CPUs,
			"resolution":      sess.Resolution,
			"errorReason":     sess.ErrorReason,
			"netTaintChecked": sess.NetTaintChecked,
			"netTaintTotal":   sess.NetTaintTotal,
			"netTaintFlagged": sess.NetTaintFlagged,
			"netTaintDomains": netTaintDomains,
		},
		"frames":  frameItems,
		"network": netItems,
		"audit":   auditItems,
	})
}

func (h *Handler) HistoryFrame(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	id := c.Params("id")
	eventID := c.Params("eventId")
	var sess domain.BrowserSession
	if err := h.st.DB.First(&sess, "id = ?", id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "session not found")
	}
	if !h.canAccessHistorySession(user, &sess) {
		return fiber.NewError(fiber.StatusForbidden, "forbidden")
	}
	var ev domain.SessionTimelineEvent
	if err := h.st.DB.First(&ev, "id = ? AND session_id = ?", eventID, id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "frame not found")
	}
	if ev.ScreenshotPath == "" {
		return fiber.NewError(fiber.StatusNotFound, "no image")
	}
	return c.SendFile(ev.ScreenshotPath)
}

func (h *Handler) HistoryDelete(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil || user.Role != domain.RoleSuperAdmin {
		return fiber.NewError(fiber.StatusForbidden, "admin only")
	}
	id := c.Params("id")
	var sess domain.BrowserSession
	if err := h.st.DB.First(&sess, "id = ?", id).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "session not found")
	}
	_ = h.st.DB.Where("session_id = ?", id).Delete(&domain.SessionTimelineEvent{}).Error
	_ = h.st.DB.Where("session_id = ?", id).Delete(&domain.NetworkEvent{}).Error
	_ = h.st.DB.Where("session_id = ?", id).Delete(&domain.AuditEvent{}).Error
	_ = os.RemoveAll(filepath.Join(h.historyRoot(), id))
	_ = h.st.DB.Delete(&sess).Error
	_ = h.auth.WriteAudit(user.ID, "admin.history.delete", "deleted history "+id)
	return c.JSON(fiber.Map{"ok": true})
}
