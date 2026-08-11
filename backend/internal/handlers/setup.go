package handlers

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) SetupStatus(c *fiber.Ctx) error {
	n, err := h.auth.CountUsers()
	if err != nil {
		return err
	}
	needs := n == 0
	keyPresent := false
	if needs {
		if raw, err := os.ReadFile(h.cfg.SetupKeyFile); err == nil && strings.TrimSpace(string(raw)) != "" {
			keyPresent = true
		}
	}
	return c.JSON(fiber.Map{
		"needsSetup": needs,
		"keyPresent": keyPresent,
	})
}

type setupCompleteReq struct {
	SetupKey    string `json:"setupKey"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
}

func (h *Handler) SetupComplete(c *fiber.Ctx) error {
	var req setupCompleteReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	pair, err := h.auth.CompleteSetup(req.SetupKey, req.Email, req.Password, req.DisplayName, h.cfg.SetupKeyFile)
	if err != nil {
		return mapAuthErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(pair)
}
