package qbittorrent

import (
	"errors"
	"fmt"
	"strings"

	"github.com/autobrr/brrewery/internal/apps/extravars"
	"github.com/autobrr/brrewery/internal/apps/libtorrent"
)

var (
	// ErrManifestUnavailable indicates the vendored build manifest could not be read.
	ErrManifestUnavailable = errors.New("qBittorrent build manifest unavailable")
	// ErrUnknownVersion indicates the requested version is not a manifest entry.
	ErrUnknownVersion = errors.New("unsupported qBittorrent version")
	// ErrBranchNotAllowed indicates the libtorrent branch is not valid for the version.
	ErrBranchNotAllowed = errors.New("libtorrent branch not supported for this qBittorrent version")
)

// Validate enforces the qBittorrent install options when appID is the
// qBittorrent catalog id. Other apps pass through unchanged.
func Validate(appID string, extra map[string]string) error {
	if appID != AppID {
		return nil
	}

	version := strings.TrimSpace(extra[extravars.QbittorrentVersion])
	branch := strings.TrimSpace(extra[extravars.LibtorrentBranch])
	if err := ValidateInstallOptions(version, branch); err != nil {
		return err
	}

	return libtorrent.ValidatePatch(extra[extravars.LibtorrentPatch])
}

// ValidateInstallOptions checks the version exists in the manifest and the
// libtorrent branch (when supplied) is allowed for that version. An empty
// branch is accepted; Ansible falls back to the line default (RC_1_2).
func ValidateInstallOptions(version, branch string) error {
	m, err := LoadManifest()
	if err != nil {
		return err
	}

	line, err := m.ResolveSelection(version)
	if err != nil {
		return err
	}

	if branch == "" {
		return nil
	}
	if branch != BranchRC12 && branch != BranchRC20 && branch != BranchRC21 {
		return fmt.Errorf("%w: %q", ErrBranchNotAllowed, branch)
	}
	if !line.AllowsBranch(branch) {
		return fmt.Errorf("%w: %s with qBittorrent %s", ErrBranchNotAllowed, branch, line.Version)
	}
	return nil
}
