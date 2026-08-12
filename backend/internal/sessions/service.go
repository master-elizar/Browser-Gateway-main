package sessions

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/orchestrator"
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
}

func New(db *gorm.DB, orch *orchestrator.Orchestrator) *Service {
	return &Service{db: db, orch: orch}
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
	return &v, nil
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
		out = append(out, toView(r))
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
		return &v, nil
	}

	_ = s.db.Model(&row).Update("status", domain.StatusStopping).Error
	s.audit(row.OwnerID, row.ID, "session.stopping", "stopping session")

	if row.ContainerID != "" {
		if err := s.orch.Destroy(ctx, row.ContainerID); err != nil {
			log.Printf("destroy %s: %v", row.ContainerID, err)
		}
	}
	now := time.Now()
	_ = s.db.Model(&domain.BrowserSession{}).Where("id = ?", row.ID).Updates(map[string]any{
		"status":     domain.StatusDestroyed,
		"stopped_at": now,
	}).Error
	s.audit(row.OwnerID, row.ID, "session.destroyed", "session destroyed")

	_ = s.db.First(&row, "id = ?", id)
	v := toView(row)
	return &v, nil
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
