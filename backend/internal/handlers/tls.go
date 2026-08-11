package handlers

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/domain"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/pkcs12"
)

const (
	certFileName  = "cert.pem"
	keyFileName   = "key.pem"
	chainFileName = "chain.pem"
	pendingFlag   = ".reload-pending"
)

type tlsStatus struct {
	Configured     bool     `json:"configured"`
	Subject        string   `json:"subject,omitempty"`
	NotBefore      string   `json:"notBefore,omitempty"`
	NotAfter       string   `json:"notAfter,omitempty"`
	DNSNames       []string `json:"dnsNames,omitempty"`
	HasChain       bool     `json:"hasChain"`
	PendingRestart bool     `json:"pendingRestart"`
	CertsDir       string   `json:"certsDir"`
}

type tlsPutBody struct {
	Format         string `json:"format"` // pem | pkcs12
	CertificatePEM string `json:"certificatePem"`
	PrivateKeyPEM  string `json:"privateKeyPem"`
	ChainPEM       string `json:"chainPem"`
	PKCS12Base64   string `json:"pkcs12Base64"`
	PKCS12Password string `json:"pkcs12Password"`
	ApplyNow       bool   `json:"applyNow"`
}

func (h *Handler) certsDir() string {
	d := strings.TrimSpace(h.cfg.CertsDir)
	if d == "" {
		return "/opt/browser-gateway/data/certs"
	}
	return d
}

func (h *Handler) pendingPath() string {
	return filepath.Join(h.certsDir(), pendingFlag)
}

func (h *Handler) AdminGetTLS(c *fiber.Ctx) error {
	st, err := h.readTLSStatus()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	return c.JSON(st)
}

func (h *Handler) AdminPutTLS(c *fiber.Ctx) error {
	actor, _ := c.Locals("user").(*domain.User)
	var body tlsPutBody
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	format := strings.ToLower(strings.TrimSpace(body.Format))
	if format == "" {
		format = "pem"
	}

	var certPEM, keyPEM, chainPEM []byte
	switch format {
	case "pem":
		certPEM = []byte(strings.TrimSpace(body.CertificatePEM))
		keyPEM = []byte(strings.TrimSpace(body.PrivateKeyPEM))
		chainPEM = []byte(strings.TrimSpace(body.ChainPEM))
		if len(certPEM) == 0 || len(keyPEM) == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "certificatePem and privateKeyPem are required")
		}
	case "pkcs12":
		raw, err := decodeBase64Flexible(body.PKCS12Base64)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid pkcs12Base64")
		}
		key, cert, err := pkcs12.Decode(raw, body.PKCS12Password)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "pkcs12 decode failed: "+err.Error())
		}
		keyPEM, err = encodePrivateKey(key)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		chainPEM = nil
	default:
		return fiber.NewError(fiber.StatusBadRequest, "format must be pem or pkcs12")
	}

	fullCert := append([]byte{}, certPEM...)
	if len(chainPEM) > 0 {
		if !strings.HasSuffix(string(fullCert), "\n") {
			fullCert = append(fullCert, '\n')
		}
		fullCert = append(fullCert, chainPEM...)
	}
	if _, err := tls.X509KeyPair(fullCert, keyPEM); err != nil {
		if _, err2 := tls.X509KeyPair(certPEM, keyPEM); err2 != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid certificate/key pair: "+err2.Error())
		}
		fullCert = certPEM
	}

	dir := h.certsDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	tmpCert := filepath.Join(dir, certFileName+".tmp")
	tmpKey := filepath.Join(dir, keyFileName+".tmp")
	if err := os.WriteFile(tmpCert, fullCert, 0o644); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := os.WriteFile(tmpKey, keyPEM, 0o600); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := os.Rename(tmpCert, filepath.Join(dir, certFileName)); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if err := os.Rename(tmpKey, filepath.Join(dir, keyFileName)); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	chainPath := filepath.Join(dir, chainFileName)
	if len(chainPEM) > 0 {
		_ = os.WriteFile(chainPath, chainPEM, 0o644)
	} else {
		_ = os.Remove(chainPath)
	}

	if actor != nil {
		_ = h.auth.WriteAudit(actor.ID, "admin.tls.update", "TLS certificate updated")
	}

	if body.ApplyNow {
		if err := h.applyTLSRestart(c.Context()); err != nil {
			_ = os.WriteFile(h.pendingPath(), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
			return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
				"ok":             true,
				"pendingRestart": true,
				"applyError":     err.Error(),
			})
		}
		_ = os.Remove(h.pendingPath())
		st, _ := h.readTLSStatus()
		return c.JSON(fiber.Map{"ok": true, "pendingRestart": false, "tls": st})
	}

	_ = os.WriteFile(h.pendingPath(), []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o644)
	st, _ := h.readTLSStatus()
	return c.JSON(fiber.Map{"ok": true, "pendingRestart": true, "tls": st})
}

func (h *Handler) AdminApplyTLS(c *fiber.Ctx) error {
	actor, _ := c.Locals("user").(*domain.User)
	if err := h.applyTLSRestart(c.Context()); err != nil {
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}
	_ = os.Remove(h.pendingPath())
	if actor != nil {
		_ = h.auth.WriteAudit(actor.ID, "admin.tls.apply", "Traefik restarted to apply TLS")
	}
	st, _ := h.readTLSStatus()
	return c.JSON(fiber.Map{"ok": true, "pendingRestart": false, "tls": st})
}

func (h *Handler) applyTLSRestart(ctx context.Context) error {
	if err := h.orch.RestartComposeService(ctx, "traefik"); err == nil {
		return nil
	}
	name := strings.TrimSpace(h.cfg.TraefikContainer)
	if name == "" {
		name = "browser-gateway-traefik-1"
	}
	return h.orch.RestartByName(ctx, name)
}

func (h *Handler) readTLSStatus() (tlsStatus, error) {
	dir := h.certsDir()
	st := tlsStatus{
		CertsDir:       dir,
		PendingRestart: fileExists(h.pendingPath()),
		HasChain:       fileExists(filepath.Join(dir, chainFileName)),
	}
	certPath := filepath.Join(dir, certFileName)
	keyPath := filepath.Join(dir, keyFileName)
	if !fileExists(certPath) || !fileExists(keyPath) {
		return st, nil
	}
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return st, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return st, fmt.Errorf("no PEM certificate found")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return st, err
	}
	st.Configured = true
	st.Subject = cert.Subject.String()
	st.NotBefore = cert.NotBefore.UTC().Format(time.RFC3339)
	st.NotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
	st.DNSNames = cert.DNSNames
	return st, nil
}

func (h *Handler) AdminSystemHealth(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	pgOK := h.st.Ping(ctx) == nil
	dockerOK := h.orch.Ping(ctx) == nil
	traefikName := strings.TrimSpace(h.cfg.TraefikContainer)
	if traefikName == "" {
		traefikName = "browser-gateway-traefik-1"
	}
	trRun, trStatus, trErr := h.orch.InspectHealthByName(ctx, traefikName)
	tlsSt, _ := h.readTLSStatus()

	overall := "ok"
	if !pgOK || !dockerOK || trErr != nil || !trRun {
		overall = "degraded"
	}

	return c.JSON(fiber.Map{
		"status": overall,
		"checks": fiber.Map{
			"postgres": fiber.Map{"ok": pgOK},
			"redis":    fiber.Map{"ok": true},
			"docker":   fiber.Map{"ok": dockerOK},
			"traefik": fiber.Map{
				"ok":     trErr == nil && trRun,
				"status": trStatus,
				"error":  errString(trErr),
				"name":   traefikName,
			},
			"tls": tlsSt,
		},
		"checkedAt": time.Now().UTC().Format(time.RFC3339),
	})
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty")
	}
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil {
		return raw, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func encodePrivateKey(key any) ([]byte, error) {
	switch k := key.(type) {
	case *rsa.PrivateKey:
		b, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}), nil
	case *ecdsa.PrivateKey:
		b, err := x509.MarshalPKCS8PrivateKey(k)
		if err != nil {
			return nil, err
		}
		return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: b}), nil
	default:
		return nil, fmt.Errorf("unsupported private key type %T", key)
	}
}
