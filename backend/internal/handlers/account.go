package handlers

import (
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/browser-gateway/backend/internal/domain"
	"github.com/browser-gateway/backend/internal/ti"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// accountTIKeyProviders are the providers a user can supply a personal API key for --
// exactly the key-requiring/key-accepting providers in backend/internal/ti (spamhaus,
// urlhaus, crt.sh, feodo need no key at all, so they're not listed here).
var accountTIKeyProviders = []string{
	"virustotal", "threatfox", "abuseipdb", "otx", "shodan", "safebrowsing", "malwarebazaar",
}

func isAccountTIKeyProvider(id string) bool {
	for _, p := range accountTIKeyProviders {
		if p == id {
			return true
		}
	}
	return false
}

type accountTIKeyView struct {
	Provider  string `json:"provider"`
	KeySet    bool   `json:"keySet"`
	KeyMasked string `json:"keyMasked,omitempty"`
}

// AccountListTIKeys reports, for every provider a personal key can be set for, whether this
// user has one configured -- the raw key is never returned, only a masked preview.
func (h *Handler) AccountListTIKeys(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	var rows []domain.UserTIKey
	_ = h.st.DB.Where("user_id = ?", user.ID).Find(&rows).Error
	byProvider := make(map[string]string, len(rows))
	for _, r := range rows {
		byProvider[r.Provider] = r.APIKey
	}
	out := make([]accountTIKeyView, 0, len(accountTIKeyProviders))
	for _, p := range accountTIKeyProviders {
		v := accountTIKeyView{Provider: p}
		if key := byProvider[p]; key != "" {
			v.KeySet = true
			v.KeyMasked = ti.MaskAPIKey(key)
		}
		out = append(out, v)
	}
	return c.JSON(fiber.Map{"items": out})
}

type accountSetTIKeyReq struct {
	APIKey string `json:"apiKey"`
}

// AccountSetTIKey upserts the caller's personal API key for one provider. An empty/masked
// key is rejected -- use DELETE to clear a key back to the project default, this endpoint is
// only for setting a real one.
func (h *Handler) AccountSetTIKey(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Params("provider")))
	if !isAccountTIKeyProvider(provider) {
		return fiber.NewError(fiber.StatusBadRequest, "unknown provider")
	}
	var req accountSetTIKeyReq
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	key := strings.TrimSpace(req.APIKey)
	if key == "" || ti.IsMaskedKey(key) {
		return fiber.NewError(fiber.StatusBadRequest, "apiKey required")
	}

	var existing domain.UserTIKey
	err := h.st.DB.Where("user_id = ? AND provider = ?", user.ID, provider).First(&existing).Error
	if err == nil {
		existing.APIKey = key
		if err := h.st.DB.Save(&existing).Error; err != nil {
			return err
		}
	} else {
		row := domain.UserTIKey{
			ID:        uuid.NewString(),
			UserID:    user.ID,
			Provider:  provider,
			APIKey:    key,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}
		if err := h.st.DB.Create(&row).Error; err != nil {
			return err
		}
	}
	_ = h.auth.WriteAudit(user.ID, "account.ti_key.set", provider)
	return c.JSON(fiber.Map{"ok": true, "provider": provider, "keySet": true, "keyMasked": ti.MaskAPIKey(key)})
}

// AccountDeleteTIKey removes the caller's personal key for one provider -- lookups for that
// user then fall back to the project-wide default key from AppSettings, if any.
func (h *Handler) AccountDeleteTIKey(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	if user == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "unauthorized")
	}
	provider := strings.ToLower(strings.TrimSpace(c.Params("provider")))
	if !isAccountTIKeyProvider(provider) {
		return fiber.NewError(fiber.StatusBadRequest, "unknown provider")
	}
	if err := h.st.DB.Where("user_id = ? AND provider = ?", user.ID, provider).Delete(&domain.UserTIKey{}).Error; err != nil {
		return err
	}
	_ = h.auth.WriteAudit(user.ID, "account.ti_key.delete", provider)
	return c.JSON(fiber.Map{"ok": true, "provider": provider, "keySet": false})
}
