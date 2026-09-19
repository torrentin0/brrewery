// Package selfupdate checks GitHub for newer brrewery releases and installs
// them in place, mirroring what install.sh does on a fresh host.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	"github.com/autobrr/brrewery/internal/buildinfo"
)

const (
	// DefaultRepo is the GitHub repository releases are fetched from. The Go
	// module path (autobrr/brrewery) is not where releases are published.
	DefaultRepo          = "torrentin0/brrewery"
	DefaultCheckInterval = 6 * time.Hour

	githubAPIBase = "https://api.github.com"
)

// RepoFromEnv resolves the release repository slug: BRREWERY_UPDATE_REPO
// ("owner/name") wins, then BRREWERY_REPO_URL (the git URL install.sh also
// honours), then DefaultRepo.
func RepoFromEnv() string {
	if repo := strings.TrimSpace(os.Getenv("BRREWERY_UPDATE_REPO")); repo != "" {
		return repo
	}
	if url := strings.TrimSpace(os.Getenv("BRREWERY_REPO_URL")); url != "" {
		slug := strings.TrimPrefix(url, "https://github.com/")
		slug = strings.TrimSuffix(slug, ".git")
		if slug != url && slug != "" {
			return slug
		}
	}
	return DefaultRepo
}

// Status is the cached result of the last release check, served to the UI.
type Status struct {
	CurrentVersion  string     `json:"current_version"`
	LatestVersion   string     `json:"latest_version,omitempty"`
	LatestTag       string     `json:"latest_tag,omitempty"`
	UpdateAvailable bool       `json:"update_available"`
	CheckedAt       *time.Time `json:"checked_at,omitempty"`
	Error           string     `json:"error,omitempty"`
	// RestartPending is not part of the release check: the update handler
	// overlays it from Updater.RestartPending, so the UI knows an installed
	// update is waiting for its restart.
	RestartPending bool `json:"restart_pending"`
}

// Checker polls the GitHub releases API and caches the newest release. A
// failed check keeps the previous result and only records the error, so the
// periodic ticker can never regress a known-good status.
type Checker struct {
	repo           string
	apiBase        string
	currentVersion string
	client         *http.Client

	mu     sync.Mutex
	status Status
}

func NewChecker(repo string) *Checker {
	return &Checker{
		repo:           repo,
		apiBase:        githubAPIBase,
		currentVersion: buildinfo.Version,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

// Status returns the cached result of the last check.
func (c *Checker) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status.CurrentVersion == "" {
		return Status{CurrentVersion: c.currentVersion}
	}
	return c.status
}

// Refresh queries GitHub for the newest non-draft release (pre-releases
// included, matching install.sh) and updates the cache.
func (c *Checker) Refresh(ctx context.Context) (Status, error) {
	tag, err := c.latestTag(ctx)

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC()
	if err != nil {
		c.status.CurrentVersion = c.currentVersion
		c.status.Error = err.Error()
		return c.status, err
	}

	c.status = Status{
		CurrentVersion:  c.currentVersion,
		LatestVersion:   strings.TrimPrefix(tag, "v"),
		LatestTag:       tag,
		UpdateAvailable: isNewer(c.currentVersion, tag),
		CheckedAt:       &now,
	}
	return c.status, nil
}

// Run checks immediately, then on every tick until ctx is cancelled. Errors
// are already recorded on the cached status, so they are ignored here.
func (c *Checker) Run(ctx context.Context, interval time.Duration) {
	_, _ = c.Refresh(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = c.Refresh(ctx)
		}
	}
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
}

// latestTag returns the tag of the newest non-draft release. GitHub lists
// releases newest-first; /releases/latest is not used because it excludes
// pre-releases (see install.sh fetch_release).
func (c *Checker) latestTag(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=10", c.apiBase, c.repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", fmt.Errorf("build release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", buildinfo.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch releases: unexpected status %s", resp.Status)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return "", fmt.Errorf("decode releases: %w", err)
	}

	for _, release := range releases {
		if release.Draft {
			continue
		}
		if strings.TrimSpace(release.TagName) == "" {
			continue
		}
		return release.TagName, nil
	}
	return "", fmt.Errorf("no published releases found for %s", c.repo)
}

// IsDevBuild reports whether this binary was built without a release version
// (make dev / go build without ldflags). Dev builds never self-update.
func IsDevBuild(version string) bool {
	return version == "" || version == "0.0.0-dev"
}

// isNewer reports whether latestTag is a strictly newer semver than the
// running version. Dev builds always report false so a checkout never offers
// to overwrite itself with a release.
func isNewer(currentVersion, latestTag string) bool {
	if IsDevBuild(currentVersion) {
		return false
	}
	current := "v" + strings.TrimPrefix(currentVersion, "v")
	latest := latestTag
	if !strings.HasPrefix(latest, "v") {
		latest = "v" + latest
	}
	if !semver.IsValid(current) || !semver.IsValid(latest) {
		return false
	}
	return semver.Compare(current, latest) < 0
}
