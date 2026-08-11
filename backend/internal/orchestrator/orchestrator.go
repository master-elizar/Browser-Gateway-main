package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/config"
	"github.com/browser-gateway/backend/internal/metrics"
)

type Orchestrator struct {
	cfg    *config.Config
	client *http.Client
}

type CreateParams struct {
	SessionID  string
	OwnerID    string
	AgentToken string
	StartURL   string
	Browser    string // chromium | firefox
	DnsMode    string
	DnsServers string
	DnsDohUrl  string
	MemoryMB   int
	CPUs       float64
	Resolution string
}

type CreateResult struct {
	ContainerID string
	Name        string
}

func New(cfg *config.Config) (*Orchestrator, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", "/var/run/docker.sock")
		},
	}
	return &Orchestrator{
		cfg: cfg,
		client: &http.Client{
			Transport: transport,
			Timeout:   60 * time.Second,
		},
	}, nil
}

func (o *Orchestrator) Close() error { return nil }

func (o *Orchestrator) Ping(ctx context.Context) error {
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		return fmt.Errorf("docker socket: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/_ping", nil)
	if err != nil {
		return err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docker ping status %d", resp.StatusCode)
	}
	return nil
}

func (o *Orchestrator) CreateAndStart(ctx context.Context, p CreateParams) (*CreateResult, error) {
	name := fmt.Sprintf("browser-session-%s", shortID(p.SessionID))
	_ = o.removeByName(ctx, name)

	browser := strings.ToLower(strings.TrimSpace(p.Browser))
	if browser == "" {
		browser = "chromium"
	}
	image := o.cfg.BrowserImage
	if browser == "firefox" {
		image = o.cfg.BrowserImageFirefox
		if image == "" {
			image = "browser-gateway/browser-engine-firefox:local"
		}
	}

	dnsMode := strings.ToLower(strings.TrimSpace(p.DnsMode))
	if dnsMode == "" {
		dnsMode = "docker"
	}
	dnsServers := strings.TrimSpace(p.DnsServers)
	if dnsServers == "" {
		dnsServers = "8.8.8.8,1.1.1.1"
	}
	dnsDoh := strings.TrimSpace(p.DnsDohUrl)
	if dnsDoh == "" {
		dnsDoh = "https://cloudflare-dns.com/dns-query"
	}
	resolution := strings.TrimSpace(p.Resolution)
	if resolution == "" {
		resolution = "1280x800x24"
	}

	memMB := p.MemoryMB
	if memMB <= 0 {
		memMB = o.cfg.BrowserMemoryMB
	}
	if memMB < 512 {
		memMB = 512
	}
	if memMB > 8192 {
		memMB = 8192
	}
	cpus := p.CPUs
	if cpus <= 0 {
		cpus = o.cfg.BrowserCPUs
	}
	if cpus < 0.5 {
		cpus = 0.5
	}
	if cpus > 8 {
		cpus = 8
	}

	env := []string{
		"SESSION_ID=" + p.SessionID,
		"AGENT_TOKEN=" + p.AgentToken,
		"OWNER_ID=" + p.OwnerID,
		"GATEWAY_INTERNAL_URL=" + getenv("GATEWAY_INTERNAL_URL", "http://backend:8080"),
		"DNS_MODE=" + dnsMode,
		"DNS_SERVERS=" + dnsServers,
		"DNS_DOH_URL=" + dnsDoh,
		"BROWSER_ENGINE=" + browser,
		"RESOLUTION=" + resolution,
	}
	if p.StartURL != "" {
		env = append(env, "START_URL="+p.StartURL)
	}

	hostConfig := map[string]any{
		"CapDrop":        []string{"ALL"},
		"CapAdd":         []string{"NET_BIND_SERVICE"},
		"SecurityOpt":    []string{"no-new-privileges:true"},
		"ReadonlyRootfs": false,
		"NetworkMode":    o.cfg.BrowserNetwork,
		"Memory":         int64(memMB) * 1024 * 1024,
		"NanoCpus":       int64(cpus * 1_000_000_000),
		"ShmSize":        512 * 1024 * 1024,
		"AutoRemove":     false,
	}
	switch dnsMode {
	case "custom", "custom_doh", "both":
		// Force queries through local dnsmasq; upstream = user servers (must be reachable).
		hostConfig["Dns"] = []string{"127.0.0.1"}
	case "doh", "docker":
		// Leave Docker embedded DNS for bootstrap / default resolution.
	default:
		// treat unknown as docker
	}

	body := map[string]any{
		"Image": image,
		"Env":   env,
		"Labels": map[string]string{
			"bg.managed":    "true",
			"bg.session_id": p.SessionID,
			"bg.owner_id":   p.OwnerID,
			"bg.browser":    browser,
		},
		"HostConfig": hostConfig,
	}

	var created struct {
		ID string `json:"Id"`
	}
	if err := o.doJSON(ctx, http.MethodPost, "/containers/create?name="+name, body, &created); err != nil {
		return nil, fmt.Errorf("container create (%s): %w", image, err)
	}
	if created.ID == "" {
		return nil, fmt.Errorf("container create: empty id")
	}

	if err := o.doJSON(ctx, http.MethodPost, "/containers/"+created.ID+"/start", nil, nil); err != nil {
		_ = o.Destroy(ctx, created.ID)
		return nil, fmt.Errorf("container start: %w", err)
	}
	metrics.ContainersCreated.Inc()
	return &CreateResult{ContainerID: created.ID, Name: name}, nil
}

func (o *Orchestrator) WaitHealthy(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ip, err := o.ContainerIP(ctx, containerID)
		if err == nil && ip != "" {
			if err := probeHealth(ctx, ip); err == nil {
				return nil
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("health timeout")
}

func (o *Orchestrator) ContainerIP(ctx context.Context, containerID string) (string, error) {
	var insp struct {
		NetworkSettings struct {
			Networks map[string]struct {
				IPAddress string `json:"IPAddress"`
			} `json:"Networks"`
		} `json:"NetworkSettings"`
	}
	if err := o.doJSON(ctx, http.MethodGet, "/containers/"+containerID+"/json", nil, &insp); err != nil {
		return "", err
	}
	if ep, ok := insp.NetworkSettings.Networks[o.cfg.BrowserNetwork]; ok && ep.IPAddress != "" {
		return ep.IPAddress, nil
	}
	for _, ep := range insp.NetworkSettings.Networks {
		if ep.IPAddress != "" {
			return ep.IPAddress, nil
		}
	}
	return "", fmt.Errorf("no ip")
}

func (o *Orchestrator) Stop(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	err := o.doJSON(ctx, http.MethodPost, "/containers/"+containerID+"/stop?t=10", nil, nil)
	if err != nil && !isGone(err) {
		return err
	}
	return nil
}

func (o *Orchestrator) Destroy(ctx context.Context, containerID string) error {
	if containerID == "" {
		return nil
	}
	_ = o.Stop(ctx, containerID)
	err := o.doJSON(ctx, http.MethodDelete, "/containers/"+containerID+"?force=1&v=1", nil, nil)
	if err != nil && !isGone(err) {
		return err
	}
	metrics.ContainersDestroyed.Inc()
	return nil
}

func (o *Orchestrator) ListManagedContainerIDs(ctx context.Context) ([]string, error) {
	var list []struct {
		ID string `json:"Id"`
	}
	filters := url.QueryEscape(`{"label":["bg.managed=true"]}`)
	path := "/containers/json?all=1&filters=" + filters
	if err := o.doJSON(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list))
	for _, c := range list {
		if c.ID != "" {
			out = append(out, c.ID)
		}
	}
	return out, nil
}

func (o *Orchestrator) RestartByName(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("empty container name")
	}
	return o.doJSON(ctx, http.MethodPost, "/containers/"+name+"/restart?t=15", nil, nil)
}

func (o *Orchestrator) RestartComposeService(ctx context.Context, service string) error {
	var list []struct {
		ID    string   `json:"Id"`
		Names []string `json:"Names"`
	}
	filters := url.QueryEscape(fmt.Sprintf(`{"label":["com.docker.compose.service=%s"]}`, service))
	if err := o.doJSON(ctx, http.MethodGet, "/containers/json?all=1&filters="+filters, nil, &list); err != nil {
		return err
	}
	if len(list) == 0 {
		return fmt.Errorf("no container for compose service %q", service)
	}
	return o.doJSON(ctx, http.MethodPost, "/containers/"+list[0].ID+"/restart?t=15", nil, nil)
}

func (o *Orchestrator) InspectHealthByName(ctx context.Context, name string) (running bool, status string, err error) {
	var info struct {
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
			Health  *struct {
				Status string `json:"Status"`
			} `json:"Health"`
		} `json:"State"`
	}
	if err := o.doJSON(ctx, http.MethodGet, "/containers/"+name+"/json", nil, &info); err != nil {
		return false, "", err
	}
	st := info.State.Status
	if info.State.Health != nil && info.State.Health.Status != "" {
		st = info.State.Health.Status
	}
	return info.State.Running, st, nil
}

func (o *Orchestrator) removeByName(ctx context.Context, name string) error {
	return o.doJSON(ctx, http.MethodDelete, "/containers/"+name+"?force=1", nil, nil)
}

func (o *Orchestrator) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://localhost"+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("docker %s %s: %s", method, path, msg)
	}
	if out == nil || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, out)
}

func shortID(id string) string {
	id = strings.ReplaceAll(id, "-", "")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func isGone(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "404") || strings.Contains(s, "No such container")
}

func probeHealth(ctx context.Context, ip string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:8090/healthz", ip), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}
