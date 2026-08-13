package domain

import "time"

type Role string

const (
	RoleSuperAdmin Role = "SUPER_ADMIN"
	RoleUser       Role = "USER"
)

type SessionStatus string

const (
	StatusCreating  SessionStatus = "CREATING"
	StatusStarting  SessionStatus = "STARTING"
	StatusRunning   SessionStatus = "RUNNING"
	StatusIdle      SessionStatus = "IDLE"
	StatusStopping  SessionStatus = "STOPPING"
	StatusDestroyed SessionStatus = "DESTROYED"
)

type User struct {
	ID           string    `json:"id" gorm:"type:uuid;primaryKey"`
	Email        string    `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash string    `json:"-" gorm:"not null"`
	DisplayName  string    `json:"displayName"`
	Role         Role      `json:"role" gorm:"not null"`
	Active       bool      `json:"active" gorm:"not null;default:true"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (User) TableName() string { return "users" }

type BrowserSession struct {
	ID           string        `json:"id" gorm:"type:uuid;primaryKey"`
	OwnerID      string        `json:"ownerId" gorm:"type:uuid;index;not null"`
	Name         string        `json:"name"`
	Status       SessionStatus `json:"status" gorm:"not null;index"`
	ContainerID  string        `json:"containerId,omitempty"`
	StartURL     string        `json:"startUrl,omitempty"`
	// Launch options (per-session; empty DNS fields fall back to AppSettings at boot).
	Browser      string  `json:"browser,omitempty" gorm:"default:chromium"` // chromium | firefox
	DnsMode      string  `json:"dnsMode,omitempty"`
	DnsServers   string  `json:"dnsServers,omitempty"`
	DnsDohUrl    string  `json:"dnsDohUrl,omitempty"`
	MemoryMB     int     `json:"memoryMb,omitempty"`
	CPUs         float64 `json:"cpus,omitempty"`
	Resolution   string  `json:"resolution,omitempty"` // e.g. 1280x800x24
	// NetworkEventLimit caps how many network-panel events GET /network/events returns
	// and the frontend keeps in memory. 0/unset -> default (500), -1 -> unlimited, N>0 -> N.
	// Normalized by sessions.Service.Create; see that package for the canonical convention.
	NetworkEventLimit int           `json:"networkEventLimit" gorm:"column:network_event_limit;default:500"`
	AgentToken   string        `json:"-" gorm:"not null"`
	ErrorReason  string        `json:"errorReason,omitempty"`
	StartedAt    *time.Time    `json:"startedAt,omitempty"`
	StoppedAt    *time.Time    `json:"stoppedAt,omitempty"`
	LastActiveAt *time.Time    `json:"lastActiveAt,omitempty"`
	// Network taint summary (Stage 18): every distinct domain seen in this session's traffic
	// checked against Spamhaus/URLhaus once the session stops. NetTaintDomains is a
	// comma-joined list of the flagged domains (capped, see ti.CheckDomainsAgainstOpenBlocklists).
	NetTaintChecked bool   `json:"netTaintChecked" gorm:"column:net_taint_checked"`
	NetTaintTotal   int    `json:"netTaintTotal,omitempty" gorm:"column:net_taint_total"`
	NetTaintFlagged int    `json:"netTaintFlagged,omitempty" gorm:"column:net_taint_flagged"`
	NetTaintDomains string `json:"-" gorm:"column:net_taint_domains"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func (BrowserSession) TableName() string { return "browser_sessions" }

type AppSettings struct {
	ID                         uint   `json:"-" gorm:"primaryKey"`
	// InstanceName overrides the "Browser Gateway" brand text shown in the sidebar/title
	// once set; empty means "use the default". Set from the setup wizard or admin settings.
	InstanceName               string `json:"instanceName,omitempty"`
	MaxConcurrentSessionsGlobal int   `json:"maxConcurrentSessionsGlobal"`
	MaxConcurrentSessionsPerUser int  `json:"maxConcurrentSessionsPerUser"`
	IdleTimeoutSec             int    `json:"idleTimeoutSec"`
	MaxSessionDurationSec      int    `json:"maxSessionDurationSec"`
	RetentionBytes             int64  `json:"retentionBytes"`
	LogSessionLifecycle        bool   `json:"logSessionLifecycle"`
	LogControlActions          bool   `json:"logControlActions"`
	LogVisitedURLs             bool   `json:"logVisitedUrls"`
	LogDownloads               bool   `json:"logDownloads"`
	LogNetworkDNS              bool   `json:"logNetworkDns"`
	LogNetworkHTTP             bool   `json:"logNetworkHttp"`
	LogKeystrokes              bool   `json:"logKeystrokes"`
	AllowRegistration          bool   `json:"allowRegistration"`
	PasswordMinLength          int    `json:"passwordMinLength"`
	PasswordRequireComplexity  bool   `json:"passwordRequireComplexity"`
	// DnsMode: docker | custom | doh | custom_doh
	DnsMode                    string `json:"dnsMode"`
	DnsServers                 string `json:"dnsServers"` // comma-separated for custom / custom_doh (via dnsmasq)
	DnsDohUrl                  string `json:"dnsDohUrl"`  // DoH template URL for doh / custom_doh
	// Threat intelligence (Stage 13+)
	TiEnabled    bool `json:"tiEnabled"`
	TiAutoEnrich bool `json:"tiAutoEnrich"` // enrich hosts from netmon (rate-limited)
	// Legacy single-provider field (kept for older clients); lookups use multi-provider flags below.
	TiProvider string `json:"tiProvider"`
	// VirusTotal — TiAPIKey is the VT key (compat with Stage 13).
	TiVirusTotalEnabled bool   `json:"tiVirusTotalEnabled" gorm:"column:ti_virustotal_enabled"`
	TiAPIKey            string `json:"tiApiKey,omitempty" gorm:"column:ti_api_key"`
	// Open / free providers (Stage 14)
	TiURLHausEnabled   bool   `json:"tiUrlhausEnabled" gorm:"column:ti_urlhaus_enabled"`
	TiThreatFoxEnabled bool   `json:"tiThreatfoxEnabled" gorm:"column:ti_threatfox_enabled"`
	TiThreatFoxAPIKey  string `json:"tiThreatfoxApiKey,omitempty" gorm:"column:ti_threatfox_api_key"`
	TiAbuseIPDBEnabled bool   `json:"tiAbuseipdbEnabled" gorm:"column:ti_abuseipdb_enabled"`
	TiAbuseIPDBAPIKey  string `json:"tiAbuseipdbApiKey,omitempty" gorm:"column:ti_abuseipdb_api_key"`
	TiOTXEnabled       bool   `json:"tiOtxEnabled" gorm:"column:ti_otx_enabled"`
	TiOTXAPIKey        string `json:"tiOtxApiKey,omitempty" gorm:"column:ti_otx_api_key"`
	TiSpamhausEnabled  bool   `json:"tiSpamhausEnabled" gorm:"column:ti_spamhaus_enabled"`
	// Domain-check tab additions (Stage 18)
	TiShodanEnabled        bool   `json:"tiShodanEnabled" gorm:"column:ti_shodan_enabled"`
	TiShodanAPIKey         string `json:"tiShodanApiKey,omitempty" gorm:"column:ti_shodan_api_key"`
	TiSafeBrowsingEnabled  bool   `json:"tiSafebrowsingEnabled" gorm:"column:ti_safebrowsing_enabled"`
	TiSafeBrowsingAPIKey   string `json:"tiSafebrowsingApiKey,omitempty" gorm:"column:ti_safebrowsing_api_key"`
	TiCrtShEnabled         bool   `json:"tiCrtshEnabled" gorm:"column:ti_crtsh_enabled"`
	TiFeodoEnabled         bool   `json:"tiFeodoEnabled" gorm:"column:ti_feodo_enabled"`
	TiMalwareBazaarEnabled bool   `json:"tiMalwarebazaarEnabled" gorm:"column:ti_malwarebazaar_enabled"`
	TiMalwareBazaarAPIKey  string `json:"tiMalwarebazaarApiKey,omitempty" gorm:"column:ti_malwarebazaar_api_key"`

	// Session viewer UI toggles (0.16.4). ViewerUIVersion=0 means "not migrated yet".
	ViewerUIVersion          int  `json:"-" gorm:"column:viewer_ui_version;default:0"`
	ViewerWebRTCEnabled      bool `json:"viewerWebrtcEnabled" gorm:"column:viewer_webrtc_enabled"`
	ViewerNoVNCEnabled       bool `json:"viewerNovncEnabled" gorm:"column:viewer_novnc_enabled"`
	ViewerFitEnabled         bool `json:"viewerFitEnabled" gorm:"column:viewer_fit_enabled"`
	ViewerStretchEnabled     bool `json:"viewerStretchEnabled" gorm:"column:viewer_stretch_enabled"`
	ViewerClipboardEnabled   bool `json:"viewerClipboardEnabled" gorm:"column:viewer_clipboard_enabled"`
	ViewerUploadEnabled      bool `json:"viewerUploadEnabled" gorm:"column:viewer_upload_enabled"`
	ViewerDownloadsEnabled   bool `json:"viewerDownloadsEnabled" gorm:"column:viewer_downloads_enabled"`
	ViewerNetworkEnabled     bool `json:"viewerNetworkEnabled" gorm:"column:viewer_network_enabled"`

	// 0.17 — history + download zip
	HistoryRetentionDays       int    `json:"historyRetentionDays" gorm:"column:history_retention_days;default:30"`
	DownloadZipPasswordDefault string `json:"downloadZipPasswordDefault,omitempty" gorm:"column:download_zip_password_default"`
}

func (AppSettings) TableName() string { return "app_settings" }

// SessionTimelineEvent is a captured click/navigate frame for session history.
type SessionTimelineEvent struct {
	ID             string    `json:"id" gorm:"type:uuid;primaryKey"`
	SessionID      string    `json:"sessionId" gorm:"type:uuid;index;not null"`
	UserID         string    `json:"userId" gorm:"type:uuid;index;not null"`
	Kind           string    `json:"kind" gorm:"index;not null"` // click | navigate
	URL            string    `json:"url,omitempty"`
	ScreenshotPath string    `json:"-" gorm:"column:screenshot_path"`
	MetaJSON       string    `json:"meta,omitempty" gorm:"type:jsonb"`
	CreatedAt      time.Time `json:"createdAt" gorm:"index"`
}

func (SessionTimelineEvent) TableName() string { return "session_timeline_events" }

// TICacheEntry stores VirusTotal (or other TI) lookup results.
type TICacheEntry struct {
	ID         string    `json:"id" gorm:"type:uuid;primaryKey"`
	Provider   string    `json:"provider" gorm:"uniqueIndex:idx_ti_ind;not null"`
	Kind       string    `json:"kind" gorm:"uniqueIndex:idx_ti_ind;not null"` // domain | ip | url
	Indicator  string    `json:"indicator" gorm:"uniqueIndex:idx_ti_ind;not null"`
	Verdict    string    `json:"verdict"`
	Malicious  int       `json:"malicious"`
	Suspicious int       `json:"suspicious"`
	Harmless   int       `json:"harmless"`
	Undetected    int       `json:"undetected"`
	Detail        string    `json:"detail,omitempty"`
	Informational bool      `json:"informational,omitempty"`
	Permalink     string    `json:"permalink,omitempty"`
	CheckedAt     time.Time `json:"checkedAt"`
	ExpiresAt     time.Time `json:"expiresAt" gorm:"index"`
}

func (TICacheEntry) TableName() string { return "ti_cache" }

// UserTIKey is a per-user personal API key for a threat-intel provider (e.g. "virustotal",
// "shodan") that overrides the project-wide default key from AppSettings for that user's
// own lookups. No row for a given (UserID, Provider) means "use the project default".
type UserTIKey struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    string    `json:"userId" gorm:"type:uuid;not null;uniqueIndex:idx_user_ti_key"`
	Provider  string    `json:"provider" gorm:"not null;uniqueIndex:idx_user_ti_key"`
	APIKey    string    `json:"-" gorm:"column:api_key;not null"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (UserTIKey) TableName() string { return "user_ti_keys" }

type AuditEvent struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    *string   `json:"userId,omitempty" gorm:"type:uuid;index"`
	SessionID *string   `json:"sessionId,omitempty" gorm:"type:uuid;index"`
	Type      string    `json:"type" gorm:"index;not null"`
	Message   string    `json:"message"`
	MetaJSON  *string   `json:"meta,omitempty" gorm:"type:jsonb"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

func (AuditEvent) TableName() string { return "audit_events" }

type NetworkEvent struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	SessionID string    `json:"sessionId" gorm:"type:uuid;index;not null"`
	Type      string    `json:"type" gorm:"index;not null"` // dns | http
	Payload   string    `json:"payload" gorm:"type:jsonb;not null"`
	CreatedAt time.Time `json:"createdAt" gorm:"index"`
}

func (NetworkEvent) TableName() string { return "network_events" }

type RefreshToken struct {
	ID        string    `json:"id" gorm:"type:uuid;primaryKey"`
	UserID    string    `json:"userId" gorm:"type:uuid;index;not null"`
	TokenHash string    `json:"-" gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"index;not null"`
	Revoked   bool      `json:"revoked" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"createdAt"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
