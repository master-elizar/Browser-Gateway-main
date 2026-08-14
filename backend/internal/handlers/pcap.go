package handlers

import (
	"github.com/browser-gateway/backend/internal/auth"
	"github.com/gofiber/fiber/v2"
)

// BrowserPcapDownload serves a session's capture file straight off local disk (written by
// the per-session pcap sidecar, see internal/orchestrator.CreatePcapSidecar) -- available as
// soon as the sidecar has flushed its first packets, not only after the session stops.
// sessions.Service.PcapFilePath already applies the same ownership check as any other
// session sub-resource.
func (h *Handler) BrowserPcapDownload(c *fiber.Ctx) error {
	user := auth.CurrentUser(c)
	id := c.Params("id")
	path, err := h.sessions.PcapFilePath(user, id)
	if err != nil {
		return mapSessionErr(err)
	}
	c.Set("Content-Type", "application/vnd.tcpdump.pcap")
	c.Set("Content-Disposition", `attachment; filename="`+id+`.pcap"`)
	return c.SendFile(path)
}
