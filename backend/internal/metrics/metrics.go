package metrics

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/gofiber/fiber/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

type DockerPinger interface {
	Ping(ctx context.Context) error
}

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "browser_gateway",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests handled by the API",
	}, []string{"method", "path", "status"})

	HTTPDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "browser_gateway",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency",
		Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"method", "path"})

	Sessions = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "browser_gateway",
		Name:      "sessions",
		Help:      "Browser sessions by status",
	}, []string{"status"})

	UsersActive = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "browser_gateway",
		Name:      "users_active",
		Help:      "Active users",
	})

	DockerUp = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "browser_gateway",
		Name:      "docker_up",
		Help:      "1 if Docker engine is reachable",
	})

	ContainersCreated = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "browser_gateway",
		Name:      "containers_created_total",
		Help:      "Browser containers created",
	})

	ContainersDestroyed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "browser_gateway",
		Name:      "containers_destroyed_total",
		Help:      "Browser containers destroyed",
	})

	VNCProxyConnections = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "browser_gateway",
		Name:      "vnc_proxy_connections_total",
		Help:      "VNC websocket proxy upgrades",
	})

	VNCProxyErrors = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "browser_gateway",
		Name:      "vnc_proxy_errors_total",
		Help:      "VNC websocket proxy failures",
	})
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func FiberMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		path := c.Route().Path
		if path == "" {
			path = c.Path()
		}
		status := strconv.Itoa(c.Response().StatusCode())
		HTTPRequests.WithLabelValues(c.Method(), path, status).Inc()
		HTTPDuration.WithLabelValues(c.Method(), path).Observe(time.Since(start).Seconds())
		return err
	}
}

func StartCollector(db *gorm.DB, docker DockerPinger, every time.Duration) {
	if every <= 0 {
		every = 15 * time.Second
	}
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		collect(db, docker)
		for range t.C {
			collect(db, docker)
		}
	}()
}

func collect(db *gorm.DB, docker DockerPinger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := db.WithContext(ctx).Model(&domain.BrowserSession{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&rows).Error; err != nil {
		log.Printf("metrics sessions: %v", err)
	} else {
		for _, st := range []domain.SessionStatus{
			domain.StatusCreating, domain.StatusStarting, domain.StatusRunning,
			domain.StatusIdle, domain.StatusStopping, domain.StatusDestroyed,
		} {
			Sessions.WithLabelValues(string(st)).Set(0)
		}
		for _, r := range rows {
			Sessions.WithLabelValues(r.Status).Set(float64(r.Count))
		}
	}

	var users int64
	if err := db.WithContext(ctx).Model(&domain.User{}).Where("active = true").Count(&users).Error; err != nil {
		log.Printf("metrics users: %v", err)
	} else {
		UsersActive.Set(float64(users))
	}

	if docker != nil && docker.Ping(ctx) == nil {
		DockerUp.Set(1)
	} else {
		DockerUp.Set(0)
	}
}
