package handlers

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/config"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/netmon"
	"github.com/browser-gateway/backend/internal/orchestrator"
	"github.com/browser-gateway/backend/internal/ratelimit"
	"github.com/browser-gateway/backend/internal/sessions"
	"github.com/browser-gateway/backend/internal/signaling"
	"github.com/browser-gateway/backend/internal/store"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Handler struct {
	cfg      *config.Config
	st       *store.Store
	orch     *orchestrator.Orchestrator
	auth     *auth.Service
	tokens   *auth.TokenService
	sessions *sessions.Service
	netmon   *netmon.Hub
	signal   *signaling.Hub
	limiter  *ratelimit.Limiter
	ti       *ti.Service
}

func New(
	cfg *config.Config,
	st *store.Store,
	orch *orchestrator.Orchestrator,
	tokens *auth.TokenService,
	authSvc *auth.Service,
	sess *sessions.Service,
	hub *netmon.Hub,
	signal *signaling.Hub,
	limiter *ratelimit.Limiter,
	tiSvc *ti.Service,
) *Handler {
	return &Handler{
		cfg: cfg, st: st, orch: orch, tokens: tokens, auth: authSvc,
		sessions: sess, netmon: hub, signal: signal, limiter: limiter, ti: tiSvc,
	}
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	apiErr := APIError{Code: "INTERNAL", Message: err.Error()}
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		apiErr.Message = e.Message
		switch code {
		case fiber.StatusBadRequest:
			apiErr.Code = "BAD_REQUEST"
		case fiber.StatusUnauthorized:
			apiErr.Code = "UNAUTHORIZED"
		case fiber.StatusForbidden:
			apiErr.Code = "FORBIDDEN"
		case fiber.StatusConflict:
			apiErr.Code = "CONFLICT"
		case fiber.StatusNotFound:
			apiErr.Code = "NOT_FOUND"
		case fiber.StatusTooManyRequests:
			apiErr.Code = "RATE_LIMIT"
		case fiber.StatusNotImplemented:
			apiErr.Code = "NOT_IMPLEMENTED"
		}
	}
	return c.Status(code).JSON(fiber.Map{"error": apiErr})
}

func (h *Handler) Register(app *fiber.App) {
	app.Get("/healthz", h.Healthz)
	app.Get("/readyz", h.Readyz)

	api := app.Group("/api")
	api.Get("/version", h.Version)
	api.Get("/setup/status", h.SetupStatus)
	api.Post("/setup/complete", h.SetupComplete)

	api.Post("/auth/register", h.AuthRegister)
	api.Post("/auth/login", h.AuthLogin)
	api.Post("/auth/refresh", h.AuthRefresh)
	api.Post("/auth/logout", h.AuthLogout)

	authed := api.Group("", auth.Middleware(h.tokens, h.auth))
	authed.Get("/auth/me", h.AuthMe)
	authed.Post("/auth/password", h.AuthChangePassword)

	authed.Get("/browser/launch-options", h.BrowserLaunchOptions)
	authed.Post("/browser/create", h.BrowserCreate)
	authed.Get("/browser/list", h.BrowserList)
	authed.Get("/browser/:id", h.BrowserGet)
	authed.Post("/browser/:id/start", h.BrowserStart)
	authed.Post("/browser/:id/stop", h.BrowserStop)
	authed.Delete("/browser/:id", h.BrowserDelete)
	authed.Post("/browser/:id/clipboard", h.BrowserClipboard)
	authed.Post("/browser/:id/upload", h.BrowserUpload)
	authed.Get("/browser/:id/downloads", h.BrowserDownloads)
	authed.Get("/browser/:id/downloads/:fileId", h.BrowserDownloadFile)
	authed.Get("/browser/:id/network/events", h.BrowserNetworkEvents)
	authed.Delete("/browser/:id/network/events", h.BrowserClearNetworkEvents)
	authed.Post("/browser/:id/network/enrich", h.BrowserNetworkEnrich)
	authed.Post("/ti/lookup", h.TILookup)
	authed.Get("/webrtc/ice", h.WebRTICE)
	authed.Get("/viewer/features", h.ViewerFeatures)

	authed.Get("/history", h.HistoryList)
	authed.Get("/history/:id", h.HistoryGet)
	authed.Get("/history/:id/frames/:eventId", h.HistoryFrame)
	authed.Delete("/history/:id", h.HistoryDelete)

	internal := app.Group("/internal")
	internal.Post("/sessions/:id/netmon", h.InternalNetmon)
	internal.Post("/sessions/:id/health", h.InternalHealth)
	internal.Post("/sessions/:id/history/frame", h.InternalHistoryFrame)

	admin := authed.Group("/admin", auth.RequireRole(domain.RoleSuperAdmin))
	admin.Get("/users", h.AdminListUsers)
	admin.Post("/users", h.AdminCreateUser)
	admin.Patch("/users/:id", h.AdminPatchUser)
	admin.Delete("/users/:id", h.AdminDeleteUser)
	admin.Get("/sessions", h.AdminListSessions)
	admin.Post("/sessions/:id/stop", h.AdminStopSession)
	admin.Get("/settings", h.AdminGetSettings)
	admin.Put("/settings", h.AdminPutSettings)
	admin.Get("/audit", h.AdminListAudit)
	admin.Get("/audit/export", h.AdminExportAudit)
	admin.Get("/tls", h.AdminGetTLS)
	admin.Put("/tls", h.AdminPutTLS)
	admin.Post("/tls/apply", h.AdminApplyTLS)
	admin.Get("/health", h.AdminSystemHealth)
	admin.Get("/updates", h.AdminCheckUpdates)
	admin.Post("/updates/apply", h.AdminApplyUpdate)
	admin.Post("/updates/clear", h.AdminClearUpdate)
	admin.Post("/network/apply", h.AdminApplyNetwork)
	admin.Get("/network/status", h.AdminNetworkStatus)
}

func (h *Handler) Healthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

func (h *Handler) Readyz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	if err := h.st.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not_ready",
			"error":  err.Error(),
		})
	}

	dockerOK := h.orch.Ping(ctx) == nil
	return c.JSON(fiber.Map{
		"status":   "ready",
		"postgres": true,
		"redis":    true,
		"docker":   dockerOK,
	})
}

func (h *Handler) Version(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"name":    "browser-gateway",
		"stage":   15,
		"version": "0.17.7",
		"env":     h.cfg.AppEnv,
	})
}

func (h *Handler) NotImplemented(feature string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
			"error": APIError{
				Code:    "NOT_IMPLEMENTED",
				Message: feature + " arrives in a later stage",
			},
		})
	}
}

func (h *Handler) Bootstrap() {
	// Prefer first-run setup key flow. Env bootstrap remains as optional escape hatch.
	if strings.TrimSpace(h.cfg.BootstrapAdminEmail) == "" || strings.TrimSpace(h.cfg.BootstrapAdminPass) == "" {
		return
	}
	if err := h.auth.BootstrapAdmin(h.cfg.BootstrapAdminEmail, h.cfg.BootstrapAdminPass); err != nil {
		log.Printf("bootstrap admin: %v", err)
	}
}
