package handlers

import (
	"errors"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/ratelimit"
	"github.com/browser-gateway/backend/internal/sessions"
	"github.com/gofiber/fiber/v2"
)

type createBrowserReq struct {
	Name       string  `json:"name"`
	StartURL   string  `json:"startUrl"`
	Browser    string  `json:"browser"`
	DnsMode    string  `json:"dnsMode"`
	DnsServers string  `json:"dnsServers"`
	DnsDohUrl  string  `json:"dnsDohUrl"`
	MemoryMB   int     `json:"memoryMb"`
	CPUs       float64 `json:"cpus"`
	Resolution string  `json:"resolution"`
}

func (h *Handler) BrowserLaunchOptions(c *fiber.Ctx) error {
	return c.JSON(h.sessions.LaunchOptions())
}

func (h *Handler) BrowserCreate(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	ok, _ := h.limiter.Allow(c.Context(), ratelimit.ClientKey("session-create", user.ID), 10, time.Minute)
	if !ok {
		return fiber.NewError(fiber.StatusTooManyRequests, "too many session create requests")
	}
	var req createBrowserReq
	_ = c.BodyParser(&req)
	view, err := h.sessions.Create(c.Context(), sessions.CreateInput{
		OwnerID:    user.ID,
		Name:       req.Name,
		StartURL:   req.StartURL,
		Browser:    req.Browser,
		DnsMode:    req.DnsMode,
		DnsServers: req.DnsServers,
		DnsDohUrl:  req.DnsDohUrl,
		MemoryMB:   req.MemoryMB,
		CPUs:       req.CPUs,
		Resolution: req.Resolution,
	})
	if err != nil {
		return mapSessionErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(view)
}

func (h *Handler) BrowserList(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	all := c.Query("all") == "true"
	items, err := h.sessions.List(user, all)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": items, "total": len(items)})
}

func (h *Handler) BrowserGet(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	view, err := h.sessions.Get(user, c.Params("id"))
	if err != nil {
		return mapSessionErr(err)
	}
	return c.JSON(view)
}

func (h *Handler) BrowserStop(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	view, err := h.sessions.Stop(c.Context(), user, c.Params("id"))
	if err != nil {
		return mapSessionErr(err)
	}
	return c.JSON(view)
}

func (h *Handler) BrowserDelete(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if err := h.sessions.Delete(c.Context(), user, c.Params("id")); err != nil {
		return mapSessionErr(err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Handler) BrowserStart(c *fiber.Ctx) error {
	// v1 destroy-on-stop; start is create-only. Keep endpoint for API compatibility.
	return c.Status(fiber.StatusConflict).JSON(fiber.Map{
		"error": APIError{Code: "INVALID_STATE", Message: "use POST /browser/create for new temporary sessions"},
	})
}

func (h *Handler) AdminListSessions(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	items, err := h.sessions.List(user, true)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": items, "total": len(items)})
}

func mapSessionErr(err error) error {
	switch {
	case errors.Is(err, sessions.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "session not found")
	case errors.Is(err, sessions.ErrForbidden):
		return fiber.NewError(fiber.StatusForbidden, "forbidden")
	case errors.Is(err, sessions.ErrLimitExceeded):
		return fiber.NewError(fiber.StatusTooManyRequests, "session limit exceeded")
	case errors.Is(err, sessions.ErrInvalidState):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	default:
		return err
	}
}
