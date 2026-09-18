// Package qbittorrent loads the vendored build manifest and validates the
// install options (qBittorrent version, libtorrent branch, optional libtorrent
// patch) supplied by the API.
package qbittorrent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/autobrr/brrewery/internal/paths"
)

// AppID is the catalog id this validator applies to.
const AppID = "qbittorrent"

// Libtorrent branch identifiers.
const (
	BranchRC12 = "RC_1_2"
	BranchRC20 = "RC_2_0"
)

// Manifest mirrors ansible/roles/qbittorrent_build/files/qbittorrent/manifest.yml.
type Manifest struct {
	Lines []Line `yaml:"lines"`
}

// Line is the complete, self-contained build profile for one qBittorrent
// release line (e.g. 5.2). Every dependency version is pinned per line
// (lockfile-style) rather than resolved from upstream at build time.
type Line struct {
	Version       string `yaml:"version"`
	CxxStd        string `yaml:"cxx_std"`
	BuildSystem   string `yaml:"build_system"`
	CompilerFlags string `yaml:"compiler_flags"`
	// Qt pins the exact Qt release for this line, e.g. 6.11.1.
	Qt string `yaml:"qt"`
	// QtMin specifies the minimum Qt version required for this line (e.g. 6.5).
	QtMin string `yaml:"qt_min,omitempty"`
	// Zlib pins the zlib release for this line, e.g. 1.3.2.
	Zlib string `yaml:"zlib"`
	// Openssl pins the OpenSSL 3.x release for this line, e.g. 3.6.3.
	Openssl    string         `yaml:"openssl"`
	Libtorrent LibtorrentSpec `yaml:"libtorrent"`
}

// LibtorrentSpec lists the branches a line can build against.
type LibtorrentSpec struct {
	Default  string                          `yaml:"default"`
	Branches map[string]LibtorrentBranchSpec `yaml:"branches"`
}

// LibtorrentBranchSpec pins the libtorrent tag and its compatible Boost for a
// branch. Boost is per-branch because RC_1_2 and RC_2_0 require different Boost
// versions (see the manifest header).
type LibtorrentBranchSpec struct {
	Tag string `yaml:"tag"`
	// Boost pins the Boost release (underscore form, e.g. 1_86_0) this branch builds against.
	Boost string `yaml:"boost"`
}

var (
	manifestOnce sync.Once
	manifest     *Manifest
	manifestErr  error
)

// LoadManifest reads and caches the vendored manifest from the qBittorrent
// vendor root.
func LoadManifest() (*Manifest, error) {
	manifestOnce.Do(func() {
		manifest, manifestErr = loadManifestFrom(paths.ResolveVendorQBittorrentRoot())
	})
	return manifest, manifestErr
}

func loadManifestFrom(root string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, "manifest.yml"))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrManifestUnavailable, err)
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrManifestUnavailable, err)
	}
	if len(m.Lines) == 0 {
		return nil, fmt.Errorf("%w: no qBittorrent lines defined", ErrManifestUnavailable)
	}
	return &m, nil
}

// LineForVersion returns the build profile for a release line (e.g. "5.2").
func (m *Manifest) LineForVersion(version string) (Line, bool) {
	for _, line := range m.Lines {
		if line.Version == version {
			return line, true
		}
	}
	return Line{}, false
}

// ResolveSelection maps the install UI version line to a manifest line.
func (m *Manifest) ResolveSelection(version string) (Line, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return Line{}, fmt.Errorf("%w: empty version line", ErrUnknownVersion)
	}
	line, ok := m.LineForVersion(version)
	if !ok {
		return Line{}, fmt.Errorf("%w: %q", ErrUnknownVersion, version)
	}
	return line, nil
}

// AllowsBranch reports whether the line can build against the given libtorrent branch.
func (l *Line) AllowsBranch(branch string) bool {
	_, ok := l.Libtorrent.Branches[branch]
	return ok
}
