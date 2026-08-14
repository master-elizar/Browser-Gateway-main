package sessions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/orchestrator"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrNotFound       = errors.New("session not found")
	ErrForbidden      = errors.New("forbidden")
	ErrLimitExceeded  = errors.New("session limit exceeded")
	ErrInvalidState   = errors.New("invalid session state")
)

type Service struct {
	db   *gorm.DB
	orch *orchestrator.Orchestrator
	ti   *ti.Service
}

func New(db *gorm.DB, orch *orchestrator.Orchestrator, tiSvc *ti.Service) *Service {
	return &Service{db: db, orch: orch, ti: tiSvc}
}

type CreateInput struct {
	OwnerID    string
	Name       string
	StartURL   string
	Browser    string
	DnsMode    string
	DnsServers string
	DnsDohUrl  string
	MemoryMB   int
	CPUs       float64
	Resolution string
	// NetworkEventLimit: 0/unset -> default (500), -1 -> unlimited, N>0 -> exactly N.
	NetworkEventLimit int
}

type SessionView struct {
	ID           string               `json:"id"`
	OwnerID      string               `json:"ownerId"`
	Name         string               `json:"name"`
	Status       domain.SessionStatus `json:"status"`
	ContainerID  string               `json:"containerId,omitempty"`
	StartURL     string               `json:"startUrl,omitempty"`
	Browser      string               `json:"browser,omitempty"`
	DnsMode      string               `json:"dnsMode,omitempty"`
	DnsServers   string               `json:"dnsServers,omitempty"`
	DnsDohUrl    string               `json:"dnsDohUrl,omitempty"`
	MemoryMB     int                  `json:"memoryMb,omitempty"`
	CPUs         float64              `json:"cpus,omitempty"`
	Resolution   string               `json:"resolution,omitempty"`
	NetworkEventLimit int             `json:"networkEventLimit"`
	ErrorReason  string               `json:"errorReason,omitempty"`
	StartedAt    *time.Time           `json:"startedAt,omitempty"`
	StoppedAt    *time.Time           `json:"stoppedAt,omitempty"`
	CreatedAt    time.Time            `json:"createdAt"`
	// Network taint summary (Stage 18) -- populated asynchronously shortly after the session
	// stops, checking every domain seen in this session's traffic against Spamhaus/URLhaus.
	NetTaintChecked bool     `json:"netTaintChecked"`
	NetTaintTotal   int      `json:"netTaintTotal,omitempty"`
	NetTaintFlagged int      `json:"netTaintFlagged,omitempty"`
	NetTaintDomains []string `json:"netTaintDomains,omitempty"`
	// PcapAvailable/PcapSizeBytes reflect the capture file on disk right now -- true/nonzero
	// as soon as the sidecar has flushed its first packets, well before the session stops.
	PcapAvailable bool                 `json:"pcapAvailable"`
	PcapSizeBytes int64                `json:"pcapSizeBytes,omitempty"`
	DurationSec  int64                `json:"durationSec,omitempty"`
	SignalingURL string               `json:"signalingUrl,omitempty"`
	NetmonURL    string               `json:"netmonUrl,omitempty"`
	ControlURL   string               `json:"controlUrl,omitempty"`
	StreamURL    string               `json:"streamUrl,omitempty"`
	StreamType   string               `json:"streamType,omitempty"`
}

type LaunchOptions struct {
	Browsers []LaunchBrowserOption `json:"browsers"`
	Defaults LaunchDefaults        `json:"defaults"`
	Limits   LaunchLimits          `json:"limits"`
}

type LaunchBrowserOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type LaunchDefaults struct {
	Browser           string  `json:"browser"`
	StartURL          string  `json:"startUrl"`
	DnsMode           string  `json:"dnsMode"`
	DnsServers        string  `json:"dnsServers"`
	DnsDohUrl         string  `json:"dnsDohUrl"`
	MemoryMB          int     `json:"memoryMb"`
	CPUs              float64 `json:"cpus"`
	Resolution        string  `json:"resolution"`
	NetworkEventLimit int     `json:"networkEventLimit"`
}

type LaunchLimits struct {
	MemoryMBMin        int      `json:"memoryMbMin"`
	MemoryMBMax        int      `json:"memoryMbMax"`
	CPUsMin            float64  `json:"cpusMin"`
	CPUsMax            float64  `json:"cpusMax"`
	Resolutions        []string `json:"resolutions"`
	// NetworkEventLimits: preset choices for the launch wizard; -1 means unlimited.
	NetworkEventLimits []int    `json:"networkEventLimits"`
}

func toView(s domain.BrowserSession) SessionView {
	v := SessionView{
		ID:          s.ID,
		OwnerID:     s.OwnerID,
		Name:        s.Name,
		Status:      s.Status,
		ContainerID: s.ContainerID,
		StartURL:    s.StartURL,
		Browser:     s.Browser,
		DnsMode:     s.DnsMode,
		DnsServers:  s.DnsServers,
		DnsDohUrl:   s.DnsDohUrl,
		MemoryMB:    s.MemoryMB,
		CPUs:        s.CPUs,
		Resolution:  s.Resolution,
		NetworkEventLimit: s.NetworkEventLimit,
		ErrorReason: s.ErrorReason,
		StartedAt:   s.StartedAt,
		StoppedAt:   s.StoppedAt,
		CreatedAt:   s.CreatedAt,
		NetTaintChecked: s.NetTaintChecked,
		NetTaintTotal:   s.NetTaintTotal,
		NetTaintFlagged: s.NetTaintFlagged,
	}
	if s.NetTaintDomains != "" {
		v.NetTaintDomains = strings.Split(s.NetTaintDomains, ",")
	}
	if s.StartedAt != nil {
		end := time.Now()
		if s.StoppedAt != nil {
			end = *s.StoppedAt
		}
		v.DurationSec = int64(end.Sub(*s.StartedAt).Seconds())
	}
	if s.Status == domain.StatusRunning || s.Status == domain.StatusIdle || s.Status == domain.StatusStarting {
		v.SignalingURL = fmt.Sprintf("/ws/sessions/%s/signaling", s.ID)
		v.NetmonURL = fmt.Sprintf("/ws/sessions/%s/netmon", s.ID)
		v.ControlURL = fmt.Sprintf("/ws/sessions/%s/control", s.ID)
		v.StreamURL = fmt.Sprintf("/ws/sessions/%s/vnc", s.ID)
		v.StreamType = "webrtc"
	}
	return v
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*SessionView, error) {
	if err := s.admit(in.OwnerID); err != nil {
		return nil, err
	}
	if in.Name == "" {
		in.Name = "Browser"
	}
	if in.StartURL == "" {
		in.StartURL = "https://example.com"
	}
	browser := strings.ToLower(strings.TrimSpace(in.Browser))
	switch browser {
	case "", "chromium", "chrome":
		browser = "chromium"
	case "firefox":
		browser = "firefox"
	default:
		return nil, fmt.Errorf("%w: unsupported browser %q", ErrInvalidState, in.Browser)
	}
	resolution := strings.TrimSpace(in.Resolution)
	if resolution == "" {
		resolution = "1280x800x24"
	}
	networkEventLimit := in.NetworkEventLimit
	switch {
	case networkEventLimit == 0:
		networkEventLimit = 500
	case networkEventLimit < 0:
		networkEventLimit = -1
	case networkEventLimit > 50000:
		networkEventLimit = 50000
	}
	agentToken, err := auth.RandomToken(24)
	if err != nil {
		return nil, err
	}
	row := domain.BrowserSession{
		ID:                uuid.NewString(),
		OwnerID:           in.OwnerID,
		Name:               in.Name,
		Status:             domain.StatusCreating,
		StartURL:           in.StartURL,
		Browser:            browser,
		DnsMode:            strings.TrimSpace(in.DnsMode),
		DnsServers:         strings.TrimSpace(in.DnsServers),
		DnsDohUrl:          strings.TrimSpace(in.DnsDohUrl),
		MemoryMB:           in.MemoryMB,
		CPUs:               in.CPUs,
		Resolution:         resolution,
		NetworkEventLimit:  networkEventLimit,
		AgentToken:         agentToken,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	s.audit(in.OwnerID, row.ID, "session.creating", "session created")

	go s.boot(row.ID)

	v := toView(row)
	s.enrichPcap(&v)
	return &v, nil
}

func (s *Service) LaunchOptions() LaunchOptions {
	st := s.settings()
	return LaunchOptions{
		Browsers: []LaunchBrowserOption{
			{ID: "chromium", Name: "Chromium", Description: "Open-source Chrome engine with CDP + DoH flags"},
			{ID: "firefox", Name: "Firefox", Description: "Firefox ESR with TRR / DoH prefs"},
		},
		Defaults: LaunchDefaults{
			Browser:    "chromium",
			StartURL:   "https://example.com",
			DnsMode:    firstNonEmpty(st.DnsMode, "docker"),
			DnsServers: firstNonEmpty(st.DnsServers, "8.8.8.8,1.1.1.1"),
			DnsDohUrl:  firstNonEmpty(st.DnsDohUrl, "https://cloudflare-dns.com/dns-query"),
			MemoryMB:          1536,
			CPUs:              1.5,
			Resolution:        "1280x800x24",
			NetworkEventLimit: 500,
		},
		Limits: LaunchLimits{
			MemoryMBMin: 512,
			MemoryMBMax: 8192,
			CPUsMin:     0.5,
			CPUsMax:     8,
			Resolutions: []string{
				"1280x800x24",
				"1366x768x24",
				"1440x900x24",
				"1600x900x24",
				"1920x1080x24",
			},
			// -1 = unlimited (matches CreateInput/BrowserSession.NetworkEventLimit convention).
			NetworkEventLimits: []int{200, 500, 1000, 2000, 5000, -1},
		},
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Service) boot(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", sessionID).Error; err != nil {
		return
	}

	_ = s.db.Model(&row).Update("status", domain.StatusStarting).Error
	s.audit(row.OwnerID, row.ID, "session.starting", "starting container")

	st := s.settings()
	dnsMode := firstNonEmpty(row.DnsMode, st.DnsMode, "docker")
	dnsServers := firstNonEmpty(row.DnsServers, st.DnsServers, "8.8.8.8,1.1.1.1")
	dnsDoh := firstNonEmpty(row.DnsDohUrl, st.DnsDohUrl, "https://cloudflare-dns.com/dns-query")
	res, err := s.orch.CreateAndStart(ctx, orchestrator.CreateParams{
		SessionID:  row.ID,
		OwnerID:    row.OwnerID,
		AgentToken: row.AgentToken,
		StartURL:   row.StartURL,
		Browser:    firstNonEmpty(row.Browser, "chromium"),
		DnsMode:    dnsMode,
		DnsServers: dnsServers,
		DnsDohUrl:  dnsDoh,
		MemoryMB:   row.MemoryMB,
		CPUs:       row.CPUs,
		Resolution: firstNonEmpty(row.Resolution, "1280x800x24"),
	})
	if err != nil {
		s.fail(row.ID, err.Error())
		return
	}
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"container_id": res.ContainerID,
		"status":       domain.StatusStarting,
	}).Error

	if st.PcapEnabled {
		// Best-effort: a capture failure (missing image, Docker API hiccup) must not take
		// the session down -- pcap is a bonus artifact, not core functionality.
		if pcapID, perr := s.orch.CreatePcapSidecar(ctx, row.ID, res.ContainerID); perr != nil {
			log.Printf("session %s pcap sidecar: %v", row.ID, perr)
		} else {
			_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).
				Update("pcap_container_id", pcapID).Error
		}
	}

	if err := s.orch.WaitHealthy(ctx, res.ContainerID, 120*time.Second); err != nil {
		_ = s.orch.Destroy(context.Background(), res.ContainerID)
		s.fail(row.ID, "boot health timeout: "+err.Error())
		return
	}

	now := time.Now()
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":         domain.StatusRunning,
		"started_at":     now,
		"last_active_at": now,
	}).Error
	s.audit(row.OwnerID, row.ID, "session.running", "session running")
	log.Printf("session %s running container %s", row.ID, short(res.ContainerID))
}

func (s *Service) fail(sessionID, reason string) {
	now := time.Now()
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"status":       domain.StatusDestroyed,
		"error_reason": reason,
		"stopped_at":   now,
	}).Error
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", sessionID).Error; err == nil {
		s.audit(row.OwnerID, row.ID, "session.failed", reason)
	}
	log.Printf("session %s failed: %s", sessionID, reason)
}

func (s *Service) Get(user *domain.User, id string) (*SessionView, error) {
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if user.Role != domain.RoleSuperAdmin && row.OwnerID != user.ID {
		return nil, ErrForbidden
	}
	v := toView(row)
	s.enrichPcap(&v)
	return &v, nil
}

// PcapFilePath returns the on-disk path of a session's capture file, after the same
// ownership check as Get -- used by the download handler so it never has to duplicate that
// authorization logic. Returns an error if the session doesn't exist/isn't owned by user, or
// if no capture file exists yet (never started, still starting, or capture was disabled).
func (s *Service) PcapFilePath(user *domain.User, id string) (string, error) {
	v, err := s.Get(user, id)
	if err != nil {
		return "", err
	}
	if !v.PcapAvailable {
		return "", ErrNotFound
	}
	return filepath.Join(s.orch.PcapDir(), id+".pcap"), nil
}

// enrichPcap stats the capture file on disk -- PcapAvailable/PcapSizeBytes reflect whatever
// has been flushed so far, which is why this is a live stat rather than a stored DB column
// (the file keeps growing for the whole session, well past whenever it was last written to
// the row).
func (s *Service) enrichPcap(v *SessionView) {
	info, err := os.Stat(filepath.Join(s.orch.PcapDir(), v.ID+".pcap"))
	if err != nil || info.IsDir() || info.Size() == 0 {
		return
	}
	v.PcapAvailable = true
	v.PcapSizeBytes = info.Size()
}

func (s *Service) List(user *domain.User, all bool) ([]SessionView, error) {
	q := s.db.Order("created_at desc")
	if !(all && user.Role == domain.RoleSuperAdmin) {
		q = q.Where("owner_id = ?", user.ID)
	}
	q = q.Where("status <> ?", domain.StatusDestroyed)
	var rows []domain.BrowserSession
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]SessionView, 0, len(rows))
	for _, r := range rows {
		v := toView(r)
		s.enrichPcap(&v)
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) Stop(ctx context.Context, user *domain.User, id string) (*SessionView, error) {
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if user.Role != domain.RoleSuperAdmin && row.OwnerID != user.ID {
		return nil, ErrForbidden
	}
	if row.Status == domain.StatusDestroyed || row.Status == domain.StatusStopping {
		v := toView(row)
		s.enrichPcap(&v)
		return &v, nil
	}

	_ = s.db.Model(&row).Update("status", domain.StatusStopping).Error
	s.audit(row.OwnerID, row.ID, "session.stopping", "stopping session")

	if row.ContainerID != "" {
		if err := s.orch.Destroy(ctx, row.ContainerID); err != nil {
			log.Printf("destroy %s: %v", row.ContainerID, err)
		}
	}
	if row.PcapContainerID != "" {
		// The capture file itself lives on the shared data volume, not inside this
		// container, so destroying it (stopping capture) doesn't lose what was recorded.
		if err := s.orch.Destroy(ctx, row.PcapContainerID); err != nil {
			log.Printf("destroy pcap sidecar %s: %v", row.PcapContainerID, err)
		}
	}
	now := time.Now()
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":     domain.StatusDestroyed,
		"stopped_at": now,
	}).Error
	s.audit(row.OwnerID, row.ID, "session.destroyed", "session destroyed")
	go s.runNetworkTaintCheck(row.ID)

	_ = s.db.First(&row, "id = ?", id)
	v := toView(row)
	s.enrichPcap(&v)
	return &v, nil
}

// runNetworkTaintCheck runs asynchronously after a session stops (called from Stop) so it
// never delays the stop response or an idle/max-duration reaper tick -- Spamhaus/URLhaus are
// live network calls per domain, and a chatty session can have hundreds of unique hosts.
// Its result lands on the BrowserSession row a little after the session actually stops; the
// frontend picks it up on its next poll/fetch of the session or history detail.
func (s *Service) runNetworkTaintCheck(sessionID string) {
	if s.ti == nil {
		return
	}
	var settings domain.AppSettings
	if err := s.db.First(&settings).Error; err != nil || !settings.TiEnabled {
		return
	}
	var events []domain.NetworkEvent
	if err := s.db.Where("session_id = ?", sessionID).Find(&events).Error; err != nil {
		return
	}
	seen := map[string]struct{}{}
	domains := make([]string, 0, len(events))
	for _, ev := range events {
		for _, d := range extractTaintDomains(ev.Payload) {
			if _, ok := seen[d]; ok {
				continue
			}
			seen[d] = struct{}{}
			domains = append(domains, d)
		}
	}
	if len(domains) == 0 {
		_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", sessionID).Updates(map[string]any{
			"net_taint_checked": true,
			"net_taint_total":   0,
			"net_taint_flagged": 0,
		}).Error
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	flagged, total := s.ti.CheckDomainsAgainstOpenBlocklists(ctx, settings, domains)
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", sessionID).Updates(map[string]any{
		"net_taint_checked": true,
		"net_taint_total":   total,
		"net_taint_flagged": len(flagged),
		"net_taint_domains": strings.Join(flagged, ","),
	}).Error
}

// extractTaintDomains pulls hostnames out of a netmon event payload -- the same url/query
// fields as handlers.extractIndicators, duplicated locally rather than shared cross-package
// since it's a small, private parsing detail of this one feature.
func extractTaintDomains(payloadJSON string) []string {
	out := []string{}
	if i := strings.Index(payloadJSON, `"url"`); i >= 0 {
		if v := jsonStringAfterTaint(payloadJSON, i); v != "" {
			if u, err := url.Parse(v); err == nil && u.Hostname() != "" {
				out = append(out, u.Hostname())
			}
		}
	}
	if i := strings.Index(payloadJSON, `"query"`); i >= 0 {
		if v := jsonStringAfterTaint(payloadJSON, i); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func jsonStringAfterTaint(s string, from int) string {
	rest := s[from:]
	colon := strings.Index(rest, ":")
	if colon < 0 {
		return ""
	}
	rest = strings.TrimLeft(rest[colon+1:], " \t\n\r")
	if !strings.HasPrefix(rest, `"`) {
		return ""
	}
	rest = rest[1:]
	end := strings.Index(rest, `"`)
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func (s *Service) Delete(ctx context.Context, user *domain.User, id string) error {
	_, err := s.Stop(ctx, user, id)
	return err
}

// ResolveStreamTarget returns container IP for an authorized running session.
func (s *Service) ResolveStreamTarget(ctx context.Context, userID string, role domain.Role, sessionID string) (ip string, err error) {
	row, err := s.loadAuthorized(userID, role, sessionID)
	if err != nil {
		return "", err
	}
	if row.Status != domain.StatusRunning && row.Status != domain.StatusIdle {
		return "", ErrInvalidState
	}
	if row.ContainerID == "" {
		return "", ErrInvalidState
	}
	return s.orch.ContainerIP(ctx, row.ContainerID)
}

func (s *Service) loadAuthorized(userID string, role domain.Role, sessionID string) (*domain.BrowserSession, error) {
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if role != domain.RoleSuperAdmin && row.OwnerID != userID {
		return nil, ErrForbidden
	}
	return &row, nil
}

// AuthenticateAgent validates the per-session agent token.
func (s *Service) AuthenticateAgent(sessionID, token string) (*domain.BrowserSession, error) {
	if token == "" {
		return nil, ErrForbidden
	}
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if row.AgentToken != token {
		return nil, ErrForbidden
	}
	return &row, nil
}

func (s *Service) AgentBaseURL(ctx context.Context, sessionID string) (string, error) {
	var row domain.BrowserSession
	if err := s.db.First(&row, "id = ?", sessionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	if row.Status != domain.StatusRunning && row.Status != domain.StatusIdle {
		return "", ErrInvalidState
	}
	if row.ContainerID == "" {
		return "", ErrInvalidState
	}
	ip, err := s.orch.ContainerIP(ctx, row.ContainerID)
	if err != nil {
		return "", err
	}
	return "http://" + ip + ":8090", nil
}

func (s *Service) admit(ownerID string) error {
	var settings domain.AppSettings
	if err := s.db.First(&settings).Error; err != nil {
		return err
	}
	active := []domain.SessionStatus{
		domain.StatusCreating, domain.StatusStarting, domain.StatusRunning, domain.StatusIdle, domain.StatusStopping,
	}
	var global int64
	if err := s.db.Model(&domain.BrowserSession{}).Where("status IN ?", active).Count(&global).Error; err != nil {
		return err
	}
	if int(global) >= settings.MaxConcurrentSessionsGlobal {
		return ErrLimitExceeded
	}
	var mine int64
	if err := s.db.Model(&domain.BrowserSession{}).
		Where("owner_id = ? AND status IN ?", ownerID, active).
		Count(&mine).Error; err != nil {
		return err
	}
	if int(mine) >= settings.MaxConcurrentSessionsPerUser {
		return ErrLimitExceeded
	}
	return nil
}

func (s *Service) TouchActivity(sessionID string) {
	now := time.Now()
	_ = s.db.Model(&domain.BrowserSession{}).
		Where("id = ? AND status IN ?", sessionID, []domain.SessionStatus{domain.StatusRunning, domain.StatusIdle}).
		Updates(map[string]any{
			"last_active_at": now,
			"status":         domain.StatusRunning,
		}).Error
}

func (s *Service) settings() domain.AppSettings {
	var settings domain.AppSettings
	_ = s.db.First(&settings).Error
	return settings
}

func (s *Service) audit(userID, sessionID, typ, msg string) {
	st := s.settings()
	if strings.HasPrefix(typ, "session.") && !st.LogSessionLifecycle {
		return
	}
	if strings.HasPrefix(typ, "control.") && !st.LogControlActions {
		return
	}
	if typ == "keystroke" && !st.LogKeystrokes {
		return
	}
	if typ == "download" && !st.LogDownloads {
		return
	}
	if typ == "url.visit" && !st.LogVisitedURLs {
		return
	}
	ev := domain.AuditEvent{
		ID:        uuid.NewString(),
		Type:      typ,
		Message:   msg,
		CreatedAt: time.Now(),
	}
	if userID != "" {
		uid := userID
		ev.UserID = &uid
	}
	if sessionID != "" {
		sid := sessionID
		ev.SessionID = &sid
	}
	_ = s.db.Create(&ev).Error
}

func (s *Service) AuditKeystroke(userID, sessionID, msg string) {
	s.audit(userID, sessionID, "keystroke", msg)
}

func (s *Service) AuditControl(userID, sessionID, msg string) {
	s.audit(userID, sessionID, "control.action", msg)
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
