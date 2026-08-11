package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browser-gateway/backend/internal/auth"
	"github.com/gofiber/fiber/v2"
)

const localVersion = "0.17.7"

// Markers older than this are treated as stale so a failed host apply cannot
// permanently lock the Update button.
const updateMarkerStaleAfter = 10 * time.Minute

type updateStatus struct {
	CurrentVersion  string          `json:"currentVersion"`
	LatestTag       string          `json:"latestTag,omitempty"`
	LatestName      string          `json:"latestName,omitempty"`
	LatestCommit    string          `json:"latestCommit,omitempty"`
	InstalledCommit string          `json:"installedCommit,omitempty"`
	UpdateAvailable bool            `json:"updateAvailable"`
	CheckedAt       string          `json:"checkedAt"`
	HTMLURL         string          `json:"htmlUrl,omitempty"`
	UpdatePending   bool            `json:"updatePending"`
	PendingStale    bool            `json:"pendingStale,omitempty"`
	Error           string          `json:"error,omitempty"`
	Source          string          `json:"source,omitempty"` // release | commit | raw
	CheckFailed     bool            `json:"checkFailed,omitempty"`
	CanForce        bool            `json:"canForce,omitempty"`
	Progress        *updateProgress `json:"progress,omitempty"`
}

type updateProgress struct {
	Percent   int    `json:"percent"`
	Phase     string `json:"phase"`
	Message   string `json:"message"`
	UpdatedAt string `json:"updatedAt,omitempty"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
}

func (h *Handler) AdminCheckUpdates(c *fiber.Ctx) error {
	st := h.checkGitHubUpdate()
	return c.JSON(st)
}

func (h *Handler) AdminApplyUpdate(c *fiber.Ctx) error {
	st := h.checkGitHubUpdate()
	force := c.Query("force") == "1" || c.Query("force") == "true"
	if !st.UpdateAvailable && !force && !st.PendingStale {
		return fiber.NewError(fiber.StatusConflict, "already on the latest version")
	}
	actor := auth.CurrentUser(c)
	marker := h.updateMarkerPath()
	if err := os.MkdirAll(filepath.Dir(marker), 0o775); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	// Remove first so systemd PathChanged re-fires even if a previous request stuck.
	_ = os.Remove(marker)
	body := fmt.Sprintf(
		"requestedAt=%s\nby=%s\nlatestCommit=%s\nforce=%v\n",
		time.Now().UTC().Format(time.RFC3339),
		actor.Email,
		st.LatestCommit,
		force,
	)
	if err := os.WriteFile(marker, []byte(body), 0o664); err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "cannot write update marker: "+err.Error())
	}
	_ = h.writeUpdateProgress(&updateProgress{
		Percent:   2,
		Phase:     "queued",
		Message:   "Waiting for host apply…",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Done:      false,
	})
	_ = h.auth.WriteAudit(actor.ID, "admin.update.request", "application update requested from UI")
	return c.JSON(fiber.Map{
		"ok":      true,
		"message": "Update requested. The host will pull main and restart shortly.",
		"marker":  marker,
	})
}

func (h *Handler) AdminClearUpdate(c *fiber.Ctx) error {
	marker := h.updateMarkerPath()
	_ = os.Remove(marker)
	_ = os.Remove(h.updateProgressPath())
	actor := auth.CurrentUser(c)
	if actor != nil {
		_ = h.auth.WriteAudit(actor.ID, "admin.update.clear", "cleared stuck update marker")
	}
	return c.JSON(fiber.Map{"ok": true, "message": "Update pending cleared."})
}

func (h *Handler) updateMarkerPath() string {
	marker := h.cfg.UpdateMarkerFile
	if marker == "" {
		return "/opt/browser-gateway/data/update.requested"
	}
	return marker
}

func (h *Handler) updateProgressPath() string {
	return filepath.Join(filepath.Dir(h.updateMarkerPath()), "update.progress")
}

func (h *Handler) writeUpdateProgress(p *updateProgress) error {
	if p == nil {
		return nil
	}
	path := h.updateProgressPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o775); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o664)
}

func (h *Handler) readUpdateProgress() *updateProgress {
	b, err := os.ReadFile(h.updateProgressPath())
	if err != nil {
		return nil
	}
	var p updateProgress
	if err := json.Unmarshal(b, &p); err != nil {
		return nil
	}
	if p.Phase == "" && p.Percent == 0 && !p.Done {
		return nil
	}
	return &p
}

func (h *Handler) checkGitHubUpdate() updateStatus {
	marker := h.updateMarkerPath()
	pending, stale := markerPendingState(marker)
	st := updateStatus{
		CurrentVersion:  localVersion,
		CheckedAt:       time.Now().UTC().Format(time.RFC3339),
		UpdatePending:   pending && !stale,
		PendingStale:    stale,
		InstalledCommit: h.readInstalledCommit(),
		Progress:        h.readUpdateProgress(),
		CanForce:        true, // admin may always force-pull main
	}
	if stale {
		st.Error = "previous update request is stale (host apply may have failed) — clear or retry"
	}
	// After a failed host apply, unlock Update even if commit tip already matches.
	if st.Progress != nil && st.Progress.Done && (st.Progress.Phase == "failed" || st.Progress.Error != "") {
		st.UpdateAvailable = true
		if st.Error == "" {
			st.Error = "last update failed — retry to rebuild from main"
		}
	}
	if st.Progress != nil && !st.UpdatePending && !st.Progress.Done {
		if progressStale(st.Progress) {
			st.Progress = nil
		}
	}
	if st.UpdatePending && st.Progress == nil {
		st.Progress = &updateProgress{
			Percent:   2,
			Phase:     "queued",
			Message:   "Waiting for host apply…",
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			Done:      false,
		}
	}
	repo := strings.TrimSpace(h.cfg.GitHubRepo)
	if repo == "" {
		repo = "TaskMaster329/Browser-Gateway"
	}

	gotRelease := false
	gotCommit := false

	// 1) GitHub Releases API
	rel, relErr, relSoft := fetchLatestRelease(repo)
	if rel == nil {
		// 2) Fallback: HTML redirect /releases/latest (no API quota)
		rel, _ = fetchLatestReleaseRedirect(repo)
	}
	if rel != nil {
		gotRelease = true
		st.LatestTag = strings.TrimPrefix(rel.TagName, "v")
		st.LatestName = rel.Name
		st.HTMLURL = rel.HTMLURL
		if versionNewer(st.LatestTag, localVersion) {
			st.UpdateAvailable = true
			st.Source = "release"
		}
	} else if relErr != "" && !relSoft && st.Error == "" {
		st.Error = relErr
	}

	// 3) Fallback: version const on main via raw.githubusercontent.com
	if st.LatestTag == "" || (!st.UpdateAvailable && !gotRelease) {
		if rawVer, rawURL, rawErr := fetchMainVersionRaw(repo); rawVer != "" {
			if st.LatestTag == "" {
				st.LatestTag = rawVer
			}
			if st.HTMLURL == "" {
				st.HTMLURL = rawURL
			}
			if versionNewer(rawVer, localVersion) {
				st.UpdateAvailable = true
				if st.Source == "" {
					st.Source = "raw"
				}
				// Prefer showing the newer raw tip when release API missed it.
				if !gotRelease || !versionNewer(st.LatestTag, localVersion) {
					st.LatestTag = rawVer
				}
			} else if st.LatestTag == "" {
				st.LatestTag = rawVer
			}
		} else if rawErr != "" && st.Error == "" && !gotRelease {
			st.Error = rawErr
		}
	}

	// 4) main tip commit (optional — used when installed.commit is present)
	sha, htmlURL, commitErr, commitSoft := fetchMainCommit(repo)
	if sha != "" {
		gotCommit = true
		st.LatestCommit = shortSHA(sha)
		if st.HTMLURL == "" {
			st.HTMLURL = htmlURL
		}
		if st.LatestTag == "" {
			st.LatestTag = "main@" + st.LatestCommit
		}
		local := st.InstalledCommit
		if local == "" {
			// No installed marker: trust version comparison only (already applied above).
		} else if !sameCommit(local, sha) {
			st.UpdateAvailable = true
			if st.Source == "" {
				st.Source = "commit"
			}
		}
	} else if commitErr != "" && !commitSoft && st.Error == "" {
		st.Error = commitErr
	}

	if !gotRelease && !gotCommit && st.LatestTag == "" {
		st.CheckFailed = true
		if st.Error == "" {
			st.Error = "could not reach GitHub to check for updates"
		}
		// Do NOT mark updateAvailable — that made the button always-on with Latest "—".
	}

	if !stale && st.UpdateAvailable {
		// Keep failed-progress / stale messages; clear soft API noise when we have a clear tip.
		if gotRelease || gotCommit || st.Source == "raw" {
			if st.Progress == nil || !(st.Progress.Done && (st.Progress.Phase == "failed" || st.Progress.Error != "")) {
				if !strings.Contains(st.Error, "failed") && !strings.Contains(st.Error, "stale") {
					st.Error = ""
				}
			}
		}
	}

	_ = relSoft
	_ = commitSoft
	return st
}

func releaseTagCurrent(latestTag, current string) bool {
	tag := strings.TrimSpace(latestTag)
	if tag == "" || strings.HasPrefix(tag, "main@") {
		return false
	}
	return !versionNewer(tag, current)
}

func progressStale(p *updateProgress) bool {
	if p == nil || p.Done {
		return false
	}
	if strings.TrimSpace(p.UpdatedAt) == "" {
		return true
	}
	ts, err := time.Parse(time.RFC3339, p.UpdatedAt)
	if err != nil {
		return true
	}
	return time.Since(ts) > updateMarkerStaleAfter
}

func markerPendingState(path string) (pending, stale bool) {
	info, err := os.Stat(path)
	if err != nil {
		return false, false
	}
	age := time.Since(info.ModTime())
	if age > updateMarkerStaleAfter {
		return true, true
	}
	return true, false
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

func fetchLatestRelease(repo string) (rel *ghRelease, errMsg string, soft bool) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	data, status, err := githubGET(url)
	if err != nil {
		return nil, err.Error(), false
	}
	if status == http.StatusNotFound {
		return nil, "", true
	}
	// Unauthenticated GitHub often returns 403 rate-limit — treat as soft miss.
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return nil, "", true
	}
	if status >= 300 {
		return nil, fmt.Sprintf("github releases api %s", http.StatusText(status)), false
	}
	var out ghRelease
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err.Error(), false
	}
	if strings.TrimSpace(out.TagName) == "" {
		return nil, "empty release tag", false
	}
	return &out, "", false
}

// fetchLatestReleaseRedirect follows /releases/latest without the REST API (avoids rate limits).
func fetchLatestReleaseRedirect(repo string) (*ghRelease, error) {
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(http.MethodGet, "https://github.com/"+repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "browser-gateway-updater")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" && (resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently || resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == 303) {
		return nil, fmt.Errorf("empty redirect")
	}
	// Some CDNs return 200 with HTML; try Location first, else parse body link.
	tag := ""
	if loc != "" {
		if i := strings.LastIndex(loc, "/tag/"); i >= 0 {
			tag = strings.TrimPrefix(loc[i+5:], "v")
			tag = strings.TrimPrefix(tag, "/")
			if j := strings.IndexAny(tag, "?#"); j >= 0 {
				tag = tag[:j]
			}
		}
	}
	if tag == "" {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
		s := string(body)
		marker := "/releases/tag/"
		if i := strings.Index(s, marker); i >= 0 {
			rest := s[i+len(marker):]
			end := strings.IndexAny(rest, "\"'?#> \t\n")
			if end < 0 {
				end = len(rest)
			}
			tag = strings.TrimPrefix(rest[:end], "v")
		}
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, fmt.Errorf("could not parse latest release tag")
	}
	return &ghRelease{
		TagName: "v" + strings.TrimPrefix(tag, "v"),
		Name:    tag,
		HTMLURL: "https://github.com/" + repo + "/releases/tag/v" + strings.TrimPrefix(tag, "v"),
	}, nil
}

// fetchMainVersionRaw reads localVersion from updates.go on main (no API token needed).
func fetchMainVersionRaw(repo string) (version, htmlURL, errMsg string) {
	url := "https://raw.githubusercontent.com/" + repo + "/main/backend/internal/handlers/updates.go"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", "", err.Error()
	}
	req.Header.Set("User-Agent", "browser-gateway-updater")
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err.Error()
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return "", "", fmt.Sprintf("raw version %s", http.StatusText(resp.StatusCode))
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<18))
	s := string(data)
	const needle = `const localVersion = "`
	i := strings.Index(s, needle)
	if i < 0 {
		return "", "", "localVersion not found on main"
	}
	rest := s[i+len(needle):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return "", "", "localVersion parse error"
	}
	ver := strings.TrimSpace(rest[:j])
	if ver == "" {
		return "", "", "empty localVersion on main"
	}
	return ver, "https://github.com/" + repo + "/blob/main/backend/internal/handlers/updates.go", ""
}

func fetchMainCommit(repo string) (sha, htmlURL, errMsg string, soft bool) {
	url := "https://api.github.com/repos/" + repo + "/commits/main"
	data, status, err := githubGET(url)
	if err != nil {
		return "", "", err.Error(), false
	}
	if status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return "", "", "", true
	}
	if status >= 300 {
		return "", "", fmt.Sprintf("github commits api %s", http.StatusText(status)), false
	}
	var commit struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(data, &commit); err != nil {
		return "", "", err.Error(), false
	}
	if commit.SHA == "" {
		return "", "", "empty commit sha", false
	}
	return commit.SHA, commit.HTMLURL, "", false
}

func githubGET(url string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "browser-gateway-updater")
	tok := strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	if tok == "" {
		tok = strings.TrimSpace(os.Getenv("GH_TOKEN"))
	}
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return data, resp.StatusCode, nil
}

func (h *Handler) readInstalledCommit() string {
	paths := []string{}
	if h.cfg.UpdateMarkerFile != "" {
		paths = append(paths, filepath.Join(filepath.Dir(h.cfg.UpdateMarkerFile), "installed.commit"))
	}
	paths = append(paths, "/opt/browser-gateway/data/installed.commit")
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		s := strings.TrimSpace(string(b))
		if s != "" {
			return shortSHA(s)
		}
	}
	return ""
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func sameCommit(a, b string) bool {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if a == "" || b == "" {
		return false
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n < 7 {
		return a == b
	}
	return a[:n] == b[:n] || strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func versionNewer(latest, current string) bool {
	lp := parseVer(latest)
	cp := parseVer(current)
	for i := 0; i < 3; i++ {
		if lp[i] > cp[i] {
			return true
		}
		if lp[i] < cp[i] {
			return false
		}
	}
	return false
}

func parseVer(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+@"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		fmt.Sscanf(parts[i], "%d", &n)
		out[i] = n
	}
	return out
}
