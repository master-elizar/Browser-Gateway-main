package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) WebRTICE(c *fiber.Ctx) error {
	if err := h.requireViewerFeature(h.loadSettings().ViewerWebRTCEnabled, "webrtc"); err != nil {
		return err
	}
	urls := []string{}
	for _, u := range strings.Split(h.cfg.TURNURLs, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	servers := []fiber.Map{}
	for _, u := range urls {
		entry := fiber.Map{"urls": u}
		if strings.HasPrefix(u, "turn:") || strings.HasPrefix(u, "turns:") {
			if h.cfg.TURNUsername != "" {
				entry["username"] = h.cfg.TURNUsername
				entry["credential"] = h.cfg.TURNPassword
			}
		}
		servers = append(servers, entry)
	}
	// No public STUN/CDN ICE — only configured local TURN/STUN (TURN_URLS).
	return c.JSON(fiber.Map{"iceServers": servers})
}
