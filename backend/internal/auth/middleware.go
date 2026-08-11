package auth

import (
	"strings"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/gofiber/fiber/v2"
)

const LocUser = "authUser"
const LocClaims = "authClaims"

func Middleware(tokens *TokenService, svc *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		hdr := c.Get("Authorization")
		if hdr == "" || !strings.HasPrefix(hdr, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "missing bearer token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(hdr, "Bearer "))
		claims, err := tokens.ParseAccess(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid access token")
		}
		user, err := svc.GetUser(claims.UserID)
		if err != nil || !user.Active {
			return fiber.NewError(fiber.StatusUnauthorized, "user not found or inactive")
		}
		c.Locals(LocUser, user)
		c.Locals(LocClaims, claims)
		return c.Next()
	}
}

func RequireRole(roles ...domain.Role) fiber.Handler {
	allowed := map[domain.Role]struct{}{}
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals(LocUser).(*domain.User)
		if !ok || user == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
		}
		if _, ok := allowed[user.Role]; !ok {
			return fiber.NewError(fiber.StatusForbidden, "forbidden")
		}
		return c.Next()
	}
}

func CurrentUser(c *fiber.Ctx) *domain.User {
	user, _ := c.Locals(LocUser).(*domain.User)
	return user
}
