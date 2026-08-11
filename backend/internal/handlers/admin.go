package handlers

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type adminPatchUserReq struct {
	Role     *domain.Role `json:"role"`
	Active   *bool        `json:"active"`
	Password *string      `json:"password"`
}

func (h *Handler) AdminPatchUser(c *fiber.Ctx) error {
	var req adminPatchUserReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	actor := auth.CurrentUser(c)
	user, err := h.auth.AdminPatchUser(c.Params("id"), actor.ID, req.Role, req.Active, req.Password)
	if err != nil {
		return mapAuthErr(err)
	}
	return c.JSON(user)
}

func (h *Handler) AdminDeleteUser(c *fiber.Ctx) error {
	actor := auth.CurrentUser(c)
	if err := h.auth.AdminDeleteUser(c.Params("id"), actor.ID); err != nil {
		return mapAuthErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) AdminStopSession(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	view, err := h.sessions.Stop(c.Context(), user, c.Params("id"))
	if err != nil {
		return mapSessionErr(err)
	}
	return c.JSON(view)
}

func (h *Handler) AdminGetSettings(c *fiber.Ctx) error {
	var s domain.AppSettings
	if err := h.st.DB.First(&s).Error; err != nil {
		return err
	}
	return c.JSON(h.settingsDTO(s))
}

func (h *Handler) AdminPutSettings(c *fiber.Ctx) error {
	var incoming domain.AppSettings
	if err := c.BodyParser(&incoming); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	var s domain.AppSettings
	if err := h.st.DB.First(&s).Error; err != nil {
		return err
	}
	if incoming.MaxConcurrentSessionsGlobal > 0 {
		s.MaxConcurrentSessionsGlobal = incoming.MaxConcurrentSessionsGlobal
	}
	if incoming.MaxConcurrentSessionsPerUser > 0 {
		s.MaxConcurrentSessionsPerUser = incoming.MaxConcurrentSessionsPerUser
	}
	if incoming.IdleTimeoutSec > 0 {
		s.IdleTimeoutSec = incoming.IdleTimeoutSec
	}
	if incoming.MaxSessionDurationSec > 0 {
		s.MaxSessionDurationSec = incoming.MaxSessionDurationSec
	}
	if incoming.RetentionBytes > 0 {
		s.RetentionBytes = incoming.RetentionBytes
	}
	s.LogSessionLifecycle = incoming.LogSessionLifecycle
	s.LogControlActions = incoming.LogControlActions
	s.LogVisitedURLs = incoming.LogVisitedURLs
	s.LogDownloads = incoming.LogDownloads
	s.LogNetworkDNS = incoming.LogNetworkDNS
	s.LogNetworkHTTP = incoming.LogNetworkHTTP
	s.LogKeystrokes = incoming.LogKeystrokes
	s.AllowRegistration = incoming.AllowRegistration
	if incoming.PasswordMinLength > 0 {
		s.PasswordMinLength = incoming.PasswordMinLength
	}
	s.PasswordRequireComplexity = incoming.PasswordRequireComplexity
	if incoming.DnsMode != "" {
		s.DnsMode = incoming.DnsMode
	}
	if incoming.DnsServers != "" {
		s.DnsServers = incoming.DnsServers
	}
	if incoming.DnsDohUrl != "" {
		s.DnsDohUrl = incoming.DnsDohUrl
	}
	s.TiEnabled = incoming.TiEnabled
	s.TiAutoEnrich = incoming.TiAutoEnrich
	s.TiVirusTotalEnabled = incoming.TiVirusTotalEnabled
	s.TiURLHausEnabled = incoming.TiURLHausEnabled
	s.TiThreatFoxEnabled = incoming.TiThreatFoxEnabled
	s.TiAbuseIPDBEnabled = incoming.TiAbuseIPDBEnabled
	s.TiOTXEnabled = incoming.TiOTXEnabled
	s.TiSpamhausEnabled = incoming.TiSpamhausEnabled
	if incoming.TiProvider != "" {
		s.TiProvider = incoming.TiProvider
	} else if s.TiProvider == "" {
		s.TiProvider = "multi"
	}
	applyTIKey := func(dst *string, incoming string) {
		key := strings.TrimSpace(incoming)
		if key != "" && !ti.IsMaskedKey(key) {
			*dst = key
		}
	}
	applyTIKey(&s.TiAPIKey, incoming.TiAPIKey)
	applyTIKey(&s.TiThreatFoxAPIKey, incoming.TiThreatFoxAPIKey)
	applyTIKey(&s.TiAbuseIPDBAPIKey, incoming.TiAbuseIPDBAPIKey)
	applyTIKey(&s.TiOTXAPIKey, incoming.TiOTXAPIKey)

	s.ViewerWebRTCEnabled = incoming.ViewerWebRTCEnabled
	s.ViewerNoVNCEnabled = incoming.ViewerNoVNCEnabled
	s.ViewerFitEnabled = incoming.ViewerFitEnabled
	s.ViewerStretchEnabled = incoming.ViewerStretchEnabled
	s.ViewerClipboardEnabled = incoming.ViewerClipboardEnabled
	s.ViewerUploadEnabled = incoming.ViewerUploadEnabled
	s.ViewerDownloadsEnabled = incoming.ViewerDownloadsEnabled
	s.ViewerNetworkEnabled = incoming.ViewerNetworkEnabled
	if s.ViewerUIVersion < 1 {
		s.ViewerUIVersion = 1
	}
	s.HistoryRetentionDays = incoming.HistoryRetentionDays
	if s.HistoryRetentionDays < 0 {
		s.HistoryRetentionDays = 0
	}
	applyTIKey(&s.DownloadZipPasswordDefault, incoming.DownloadZipPasswordDefault)

	if err := h.st.DB.Save(&s).Error; err != nil {
		return err
	}
	actor := auth.CurrentUser(c)
	_ = h.auth.WriteAudit(actor.ID, "admin.settings.update", "settings updated")
	return c.JSON(h.settingsDTO(s))
}

func applyAuditFilters(q *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if v := c.Query("userId"); v != "" {
		q = q.Where("user_id = ?", v)
	}
	if v := c.Query("sessionId"); v != "" {
		q = q.Where("session_id = ?", v)
	}
	if v := c.Query("type"); v != "" {
		q = q.Where("type ILIKE ?", "%"+v+"%")
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q = q.Where("created_at <= ?", t)
		}
	}
	return q
}

func (h *Handler) AdminListAudit(c *fiber.Ctx) error {
	q := applyAuditFilters(h.st.DB.Model(&domain.AuditEvent{}).Order("created_at desc"), c)

	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	offset := 0
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return err
	}
	var items []domain.AuditEvent
	if err := q.Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return err
	}

	userIDs := map[string]struct{}{}
	for _, ev := range items {
		if ev.UserID != nil && *ev.UserID != "" {
			userIDs[*ev.UserID] = struct{}{}
		}
	}
	users := map[string]domain.User{}
	if len(userIDs) > 0 {
		ids := make([]string, 0, len(userIDs))
		for id := range userIDs {
			ids = append(ids, id)
		}
		var rows []domain.User
		_ = h.st.DB.Select("id", "email", "display_name", "role").Where("id IN ?", ids).Find(&rows).Error
		for _, u := range rows {
			users[u.ID] = u
		}
	}

	type auditView struct {
		domain.AuditEvent
		UserEmail       string `json:"userEmail,omitempty"`
		UserDisplayName string `json:"userDisplayName,omitempty"`
		UserRole        string `json:"userRole,omitempty"`
		Summary         string `json:"summary"`
	}
	out := make([]auditView, 0, len(items))
	for _, ev := range items {
		v := auditView{AuditEvent: ev}
		if ev.UserID != nil {
			if u, ok := users[*ev.UserID]; ok {
				v.UserEmail = u.Email
				v.UserDisplayName = u.DisplayName
				v.UserRole = string(u.Role)
			}
		}
		sid := ""
		if ev.SessionID != nil {
			sid = *ev.SessionID
		}
		v.Summary = humanAuditSummary(ev.Type, ev.Message, v.UserDisplayName, v.UserEmail, sid)
		out = append(out, v)
	}
	return c.JSON(fiber.Map{"items": out, "total": total, "limit": limit, "offset": offset})
}

func humanAuditSummary(typ, message, displayName, email, sessionID string) string {
	who := displayName
	if who == "" {
		who = email
	}
	if who == "" {
		who = "Someone"
	}
	switch {
	case strings.HasPrefix(typ, "auth.login"):
		return fmt.Sprintf("%s signed in", who)
	case strings.HasPrefix(typ, "auth.register"):
		return fmt.Sprintf("%s registered an account", who)
	case strings.HasPrefix(typ, "auth.setup"):
		return fmt.Sprintf("%s completed initial setup as administrator", who)
	case strings.HasPrefix(typ, "auth.password"):
		return fmt.Sprintf("%s changed their password", who)
	case strings.HasPrefix(typ, "auth.refresh"):
		return fmt.Sprintf("%s refreshed a session token", who)
	case strings.Contains(typ, "session") && strings.Contains(typ, "create"):
		return fmt.Sprintf("%s created a browser session", who)
	case strings.Contains(typ, "session") && strings.Contains(typ, "stop"):
		return fmt.Sprintf("%s stopped a browser session", who)
	case strings.HasPrefix(typ, "admin.tls"):
		return fmt.Sprintf("%s updated TLS settings", who)
	case strings.HasPrefix(typ, "admin.update"):
		return fmt.Sprintf("%s requested an application update", who)
	case strings.HasPrefix(typ, "admin.user"):
		return fmt.Sprintf("%s changed user administration: %s", who, message)
	case strings.HasPrefix(typ, "admin.settings"):
		return fmt.Sprintf("%s updated application settings", who)
	default:
		if sessionID != "" && sessionID != "-" {
			return fmt.Sprintf("%s — %s (session %s)", who, message, shortID(sessionID))
		}
		if message != "" {
			return fmt.Sprintf("%s — %s", who, message)
		}
		return fmt.Sprintf("%s performed %s", who, typ)
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func (h *Handler) AdminExportAudit(c *fiber.Ctx) error {
	q := applyAuditFilters(h.st.DB.Model(&domain.AuditEvent{}).Order("created_at desc"), c)
	var items []domain.AuditEvent
	if err := q.Limit(10000).Find(&items).Error; err != nil {
		return err
	}
	format := c.Query("format", "csv")
	if format == "json" {
		c.Set("Content-Disposition", "attachment; filename=audit-export.json")
		return c.JSON(fiber.Map{"items": items, "total": len(items)})
	}
	if format == "rfc5424" || format == "syslog" {
		c.Set("Content-Type", "text/plain; charset=utf-8")
		c.Set("Content-Disposition", "attachment; filename=audit-export.rfc5424.log")
		var b strings.Builder
		users := map[string]domain.User{}
		ids := []string{}
		for _, ev := range items {
			if ev.UserID != nil {
				ids = append(ids, *ev.UserID)
			}
		}
		if len(ids) > 0 {
			var rows []domain.User
			_ = h.st.DB.Select("id", "email", "display_name", "role").Where("id IN ?", ids).Find(&rows).Error
			for _, u := range rows {
				users[u.ID] = u
			}
		}
		for _, ev := range items {
			actor := "-"
			if ev.UserID != nil {
				if u, ok := users[*ev.UserID]; ok && u.Email != "" {
					actor = u.Email
				} else {
					actor = *ev.UserID
				}
			}
			sid := "-"
			if ev.SessionID != nil && *ev.SessionID != "" {
				sid = *ev.SessionID
			}
			line := fmt.Sprintf("<%d>1 %s browser-gateway %s - %s [actor=\"%s\" session=\"%s\"] %s\n",
				14,
				ev.CreatedAt.UTC().Format(time.RFC3339),
				ev.Type,
				ev.ID,
				actor,
				sid,
				ev.Message,
			)
			b.WriteString(line)
		}
		return c.SendString(b.String())
	}
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=audit-export.csv")
	w := csv.NewWriter(c)
	_ = w.Write([]string{"id", "createdAt", "type", "message", "userId", "sessionId"})
	for _, ev := range items {
		uid, sid := "", ""
		if ev.UserID != nil {
			uid = *ev.UserID
		}
		if ev.SessionID != nil {
			sid = *ev.SessionID
		}
		_ = w.Write([]string{ev.ID, ev.CreatedAt.Format(time.RFC3339), ev.Type, ev.Message, uid, sid})
	}
	w.Flush()
	return nil
}
