package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type netmonBatchReq struct {
	Events []map[string]any `json:"events"`
}

func agentTokenFromRequest(c *fiber.Ctx) string {
	if t := c.Get("X-Agent-Token"); t != "" {
		return t
	}
	authz := c.Get("Authorization")
	if len(authz) > 7 && authz[:7] == "Bearer " {
		return authz[7:]
	}
	return ""
}

func (h *Handler) InternalNetmon(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	if _, err := h.sessions.AuthenticateAgent(sessionID, agentTokenFromRequest(c)); err != nil {
		return mapSessionErr(err)
	}
	var req netmonBatchReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	var settings domain.AppSettings
	_ = h.st.DB.First(&settings).Error
	now := time.Now()
	accepted := 0
	for _, ev := range req.Events {
		typ, _ := ev["type"].(string)
		if typ == "" {
			typ = "http"
		}
		if typ == "dns" && !settings.LogNetworkDNS {
			continue
		}
		if (typ == "http" || typ == "ip") && !settings.LogNetworkHTTP {
			continue
		}
		raw, _ := json.Marshal(ev)
		row := domain.NetworkEvent{
			ID:        uuid.NewString(),
			SessionID: sessionID,
			Type:      typ,
			Payload:   string(raw),
			CreatedAt: now,
		}
		if err := h.st.DB.Create(&row).Error; err != nil {
			return err
		}
		h.netmon.Publish(sessionID, ev)
		accepted++
		if settings.TiEnabled && settings.TiAutoEnrich && h.ti != nil {
			go h.autoEnrichEvent(sessionID, ev, settings)
		}
	}
	return c.JSON(fiber.Map{"ok": true, "accepted": accepted})
}

func (h *Handler) autoEnrichEvent(sessionID string, ev map[string]any, settings domain.AppSettings) {
	if h.ti == nil {
		return
	}
	candidates := []string{}
	if q, ok := ev["query"].(string); ok && q != "" {
		candidates = append(candidates, q)
	}
	if u, ok := ev["url"].(string); ok && u != "" {
		candidates = append(candidates, u)
	}
	if len(candidates) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// Only first indicator to stay within free-tier rate limits.
	results := h.ti.LookupMany(ctx, settings, candidates[:1])
	for _, r := range results {
		h.persistTIResult(sessionID, r)
	}
}

// persistTIResult stores a TI verdict on the session (and publishes to live netmon)
// so History can show the same Check results after the session is closed.
func (h *Handler) persistTIResult(sessionID string, r ti.Result) {
	if sessionID == "" || r.Error != "" || strings.TrimSpace(r.Indicator) == "" {
		return
	}
	payload := map[string]any{
		"type":       "ti",
		"ts":         time.Now().UTC().Format(time.RFC3339),
		"provider":   r.Provider,
		"kind":       r.Kind,
		"indicator":  r.Indicator,
		"verdict":    r.Verdict,
		"malicious":  r.Malicious,
		"suspicious": r.Suspicious,
		"harmless":   r.Harmless,
		"undetected": r.Undetected,
		"permalink":  r.Permalink,
		"cached":     r.Cached,
		"providers":  r.Providers,
	}
	raw, _ := json.Marshal(payload)
	row := domain.NetworkEvent{
		ID:        uuid.NewString(),
		SessionID: sessionID,
		Type:      "ti",
		Payload:   string(raw),
		CreatedAt: time.Now(),
	}
	_ = h.st.DB.Create(&row).Error
	h.netmon.Publish(sessionID, payload)
}

func (h *Handler) InternalHealth(c *fiber.Ctx) error {
	sessionID := c.Params("id")
	if _, err := h.sessions.AuthenticateAgent(sessionID, agentTokenFromRequest(c)); err != nil {
		return mapSessionErr(err)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) BrowserClipboard(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerClipboardEnabled, "clipboard"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	base, err := h.sessions.AgentBaseURL(c.Context(), id)
	if err != nil {
		return mapSessionErr(err)
	}
	var req struct {
		Direction string `json:"direction"`
		Text      string `json:"text"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	client := &http.Client{Timeout: 20 * time.Second}
	switch req.Direction {
	case "fromRemote":
		resp, err := client.Get(base + "/clipboard")
		if err != nil {
			return fiber.NewError(fiber.StatusBadGateway, "clipboard unavailable")
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var out map[string]any
		_ = json.Unmarshal(body, &out)
		return c.JSON(out)
	case "toRemote":
		payload, _ := json.Marshal(map[string]any{"text": req.Text, "paste": true})
		resp, err := client.Post(base+"/clipboard", "application/json", bytes.NewReader(payload))
		if err != nil {
			return fiber.NewError(fiber.StatusBadGateway, "clipboard unavailable")
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var out map[string]any
		if err := json.Unmarshal(body, &out); err != nil || out == nil {
			return c.JSON(fiber.Map{"ok": true, "pasted": true})
		}
		return c.JSON(out)
	default:
		return fiber.NewError(fiber.StatusBadRequest, "direction must be toRemote or fromRemote")
	}
}

func (h *Handler) BrowserUpload(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerUploadEnabled, "upload"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	base, err := h.sessions.AgentBaseURL(c.Context(), id)
	if err != nil {
		return mapSessionErr(err)
	}
	file, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "file required")
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	var buf bytes.Buffer
	mw := multiparter(&buf, file.Filename, src)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(base+"/upload", mw, &buf)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "upload unavailable")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	if out == nil {
		out = map[string]any{"ok": true}
	}
	return c.JSON(out)
}

func multiparter(buf *bytes.Buffer, filename string, r io.Reader) string {
	boundary := "----bgboundary" + uuid.NewString()
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"" + filename + "\"\r\n")
	buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	_, _ = io.Copy(buf, r)
	buf.WriteString("\r\n--" + boundary + "--\r\n")
	return "multipart/form-data; boundary=" + boundary
}

func (h *Handler) BrowserDownloads(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerDownloadsEnabled, "downloads"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	base, err := h.sessions.AgentBaseURL(c.Context(), id)
	if err != nil {
		return mapSessionErr(err)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(base + "/downloads")
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "downloads unavailable")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	return c.JSON(out)
}

func (h *Handler) BrowserDownloadFile(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerDownloadsEnabled, "downloads"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	fileID := c.Params("fileId")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	base, err := h.sessions.AgentBaseURL(c.Context(), id)
	if err != nil {
		return mapSessionErr(err)
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(base + "/downloads/" + fileID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadGateway, "download unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fiber.NewError(fiber.StatusNotFound, "file not found")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	name := fileID
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if i := strings.Index(strings.ToLower(cd), "filename="); i >= 0 {
			name = strings.Trim(cd[i+9:], `" `)
		}
	}
	format := strings.ToLower(c.Query("format", "file"))
	if format == "zip" {
		password := c.Query("password")
		if password == "" {
			password = h.loadSettings().DownloadZipPasswordDefault
		}
		if password == "" {
			return fiber.NewError(fiber.StatusBadRequest, "zip password required")
		}
		zipped, zerr := zipFilePassword(name, data, password)
		if zerr != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "zip failed: "+zerr.Error())
		}
		c.Set("Content-Type", "application/zip")
		c.Set("Content-Disposition", `attachment; filename="`+name+`.zip"`)
		return c.Send(zipped)
	}
	ctype := resp.Header.Get("Content-Type")
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	c.Set("Content-Type", ctype)
	c.Set("Content-Disposition", `attachment; filename="`+name+`"`)
	return c.Send(data)
}

func (h *Handler) BrowserNetworkEvents(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerNetworkEnabled, "network"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	q := h.st.DB.Where("session_id = ?", id).Order("created_at desc")
	if t := c.Query("type"); t != "" {
		q = q.Where("type = ?", t)
	}
	limit := 500
	var rows []domain.NetworkEvent
	if err := q.Limit(limit).Find(&rows).Error; err != nil {
		return err
	}
	items := make([]map[string]any, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		var payload map[string]any
		_ = json.Unmarshal([]byte(rows[i].Payload), &payload)
		if payload == nil {
			payload = map[string]any{}
		}
		payload["id"] = rows[i].ID
		payload["type"] = rows[i].Type
		items = append(items, payload)
	}
	return c.JSON(fiber.Map{"items": items, "total": len(items)})
}

func (h *Handler) BrowserClearNetworkEvents(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerNetworkEnabled, "network"); err != nil {
		return err
	}
	user := auth.CurrentUser(c)
	id := c.Params("id")
	if _, err := h.sessions.Get(user, id); err != nil {
		return mapSessionErr(err)
	}
	q := h.st.DB.Where("session_id = ?", id)
	switch t := c.Query("type"); t {
	case "", "all":
		// delete all for session
	case "dns", "http":
		q = q.Where("type = ?", t)
	default:
		return fiber.NewError(fiber.StatusBadRequest, "type must be all, dns, or http")
	}
	res := q.Delete(&domain.NetworkEvent{})
	if res.Error != nil {
		return res.Error
	}
	return c.JSON(fiber.Map{"ok": true, "deleted": res.RowsAffected})
}
