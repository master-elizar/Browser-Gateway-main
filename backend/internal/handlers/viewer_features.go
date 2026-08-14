package handlers

import (
	"strings"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) viewerFeaturesDTO(s domain.AppSettings) fiber.Map {
	return fiber.Map{
		"instanceName":                    strings.TrimSpace(s.InstanceName),
		"viewerWebrtcEnabled":             s.ViewerWebRTCEnabled,
		"viewerNovncEnabled":              s.ViewerNoVNCEnabled,
		"viewerFitEnabled":                s.ViewerFitEnabled,
		"viewerStretchEnabled":            s.ViewerStretchEnabled,
		"viewerClipboardEnabled":          s.ViewerClipboardEnabled,
		"viewerUploadEnabled":             s.ViewerUploadEnabled,
		"viewerDownloadsEnabled":          s.ViewerDownloadsEnabled,
		"viewerNetworkEnabled":            s.ViewerNetworkEnabled,
		"pcapEnabled":                     s.PcapEnabled,
		"downloadZipPasswordDefaultSet":   strings.TrimSpace(s.DownloadZipPasswordDefault) != "",
	}
}

// ViewerFeatures is available to any authenticated user (safe UI feature flags).
func (h *Handler) ViewerFeatures(c *fiber.Ctx) error {
	return c.JSON(h.viewerFeaturesDTO(h.loadSettings()))
}

func (h *Handler) requireViewerFeature(enabled bool, name string) error {
	if enabled {
		return nil
	}
	return fiber.NewError(fiber.StatusForbidden, name+" is disabled by admin")
}
