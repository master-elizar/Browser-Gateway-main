package httpserver

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/config"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/handlers"
	"github.com/browser-gateway/backend/internal/metrics"
	"github.com/browser-gateway/backend/internal/netmon"
	"github.com/browser-gateway/backend/internal/orchestrator"
	"github.com/browser-gateway/backend/internal/ratelimit"
	"github.com/browser-gateway/backend/internal/sessions"
	"github.com/browser-gateway/backend/internal/signaling"
	"github.com/browser-gateway/backend/internal/store"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/browser-gateway/backend/internal/workers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

type Server struct {
	app    *fiber.App
	http   *http.Server
	store  *store.Store
	orch   *orchestrator.Orchestrator
	cfg    *config.Config
	handle *handlers.Handler
}

func New(cfg *config.Config) (*Server, error) {
	st, err := store.Open(cfg.DatabaseURL, cfg.RedisURL)
	if err != nil {
		return nil, err
	}
	if err := st.AutoMigrate(); err != nil {
		_ = st.Close()
		return nil, err
	}
	if err := st.EnsureDefaultSettings(
		cfg.MaxSessionsGlobal,
		cfg.MaxSessionsPerUser,
		int(cfg.SessionIdleTimeout.Seconds()),
		int(cfg.SessionMaxDuration.Seconds()),
	); err != nil {
		_ = st.Close()
		return nil, err
	}
	_ = st.BackfillDNSDefaults()
	_ = st.BackfillTIDefaults()
	_ = st.BackfillViewerUIDefaults()
	_ = st.BackfillFeedDefaults()
	_ = st.BackfillPcapDefaults()

	orch, err := orchestrator.New(cfg)
	if err != nil {
		_ = st.Close()
		return nil, err
	}

	tokens := auth.NewTokenService(cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	authSvc := auth.NewService(st.DB, tokens)
	tiSvc := ti.New(st.DB)
	tiSvc.StartFeedRefresher(func() domain.AppSettings {
		var s domain.AppSettings
		_ = st.DB.First(&s).Error
		return s
	})
	sess := sessions.New(st.DB, orch, tiSvc)
	hub := netmon.NewHub()
	sig := signaling.NewHub()
	limiter := ratelimit.New(st.Redis)
	h := handlers.New(cfg, st, orch, tokens, authSvc, sess, hub, sig, limiter, tiSvc)
	h.Bootstrap()

	app := fiber.New(fiber.Config{
		AppName:      "browser-gateway",
		ReadTimeout:  0,
		WriteTimeout: 0,
		BodyLimit:    64 * 1024 * 1024,
		ErrorHandler: handlers.ErrorHandler,
	})
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(metrics.FiberMiddleware())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
	}))

	h.Register(app)
	metrics.StartCollector(st.DB, orch, 15*time.Second)
	workers.StartPolicyWorker(st.DB, sess, 30*time.Second)
	workers.StartRetentionWorker(st.DB, 5*time.Minute)
	workers.StartReconcile(st.DB, orch, sess, 60*time.Second)
	workers.StartPcapSizeGuard(st.DB, orch, 2*time.Minute)

	return &Server{app: app, store: st, orch: orch, cfg: cfg, handle: h}, nil
}

func (s *Server) Listen(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws/", s.handle.ServeWS)
	mux.Handle("/metrics", metrics.Handler())
	mux.Handle("/", adaptor.FiberApp(s.app))

	s.http = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("browser-gateway backend listening on %s", addr)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if s.http != nil {
		if err := s.http.Shutdown(ctx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}
	_ = s.orch.Close()
	return s.store.Close()
}
