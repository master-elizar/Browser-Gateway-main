package handlers

import (
	"errors"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/ratelimit"
	"github.com/gofiber/fiber/v2"
)

type registerReq struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

func (h *Handler) AuthRegister(c *fiber.Ctx) error {
	var req registerReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	pair, err := h.auth.Register(req.Email, req.Password, req.DisplayName)
	if err != nil {
		return mapAuthErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(pair)
}

func (h *Handler) AuthLogin(c *fiber.Ctx) error {
	var req loginReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	ctx := c.UserContext()
	ip := c.IP()
	email := strings.ToLower(strings.TrimSpace(req.Email))
	identity := email
	if identity == "" {
		identity = ip
	}

	if locked, ttl, _ := h.limiter.IsLocked(ctx, identity); locked {
		return fiber.NewError(fiber.StatusTooManyRequests, "account temporarily locked, retry in "+ttl.Round(time.Second).String())
	}
	ok, _ := h.limiter.Allow(ctx, ratelimit.ClientKey("login", ip), 30, time.Minute)
	if !ok {
		return fiber.NewError(fiber.StatusTooManyRequests, "too many login attempts")
	}

	pair, err := h.auth.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			_ = h.limiter.RecordLoginFail(ctx, identity, 8, 15*time.Minute, 15*time.Minute)
		}
		return mapAuthErr(err)
	}
	h.limiter.ClearLoginFails(ctx, identity)
	return c.JSON(pair)
}

func (h *Handler) AuthRefresh(c *fiber.Ctx) error {
	var req refreshReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	pair, err := h.auth.Refresh(req.RefreshToken)
	if err != nil {
		return mapAuthErr(err)
	}
	return c.JSON(pair)
}

func (h *Handler) AuthLogout(c *fiber.Ctx) error {
	var req refreshReq
	_ = c.BodyParser(&req)
	_ = h.auth.Logout(req.RefreshToken)
	return c.JSON(fiber.Map{"ok": true})
}

func (h *Handler) AuthMe(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	return c.JSON(user)
}

type changePasswordReq struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (h *Handler) AuthChangePassword(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	var req changePasswordReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if err := h.auth.ChangePassword(user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		return mapAuthErr(err)
	}
	return c.JSON(fiber.Map{"ok": true})
}

type adminCreateUserReq struct {
	Email       string      `json:"email"`
	Password    string      `json:"password"`
	DisplayName string      `json:"displayName"`
	Role        domain.Role `json:"role"`
}

func (h *Handler) AdminListUsers(c *fiber.Ctx) error {
	users, err := h.auth.ListUsers()
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": users, "total": len(users)})
}

func (h *Handler) AdminCreateUser(c *fiber.Ctx) error {
	var req adminCreateUserReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	user, err := h.auth.AdminCreateUser(req.Email, req.Password, req.DisplayName, req.Role)
	if err != nil {
		return mapAuthErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(user)
}

func mapAuthErr(err error) error {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return fiber.NewError(fiber.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, auth.ErrUserInactive):
		return fiber.NewError(fiber.StatusForbidden, "user inactive")
	case errors.Is(err, auth.ErrRegistrationClosed):
		return fiber.NewError(fiber.StatusForbidden, "registration is closed")
	case errors.Is(err, auth.ErrWeakPassword):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrEmailTaken):
		return fiber.NewError(fiber.StatusConflict, "email already registered")
	case errors.Is(err, auth.ErrInvalidRefresh):
		return fiber.NewError(fiber.StatusUnauthorized, "invalid refresh token")
	case errors.Is(err, auth.ErrUserNotFound):
		return fiber.NewError(fiber.StatusNotFound, "user not found")
	case errors.Is(err, auth.ErrCannotModifySelf):
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case errors.Is(err, auth.ErrLastSuperAdmin):
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case errors.Is(err, auth.ErrSetupRequired):
		return fiber.NewError(fiber.StatusForbidden, "initial setup required")
	case errors.Is(err, auth.ErrInvalidSetupKey):
		return fiber.NewError(fiber.StatusUnauthorized, "invalid setup key")
	case errors.Is(err, auth.ErrSetupComplete):
		return fiber.NewError(fiber.StatusConflict, "setup already completed")
	case errors.Is(err, auth.ErrWrongPassword):
		return fiber.NewError(fiber.StatusUnauthorized, "current password is incorrect")
	default:
		if strings.Contains(err.Error(), "email required") || strings.Contains(err.Error(), "invalid role") {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return err
	}
}
