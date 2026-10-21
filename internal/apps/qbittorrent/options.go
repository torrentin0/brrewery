package qbittorrent

import (
	"github.com/autobrr/brrewery/internal/apps/extravars"
	"github.com/autobrr/brrewery/internal/apps/model"
)

func branchLabel(branch string) string {
	switch branch {
	case BranchRC21:
		return "libtorrent 2.1"
	case BranchRC20:
		return "libtorrent 2.0"
	default:
		return "libtorrent 1.2"
	}
}

// InstallOptions builds the catalog install options for qBittorrent from the
// vendored manifest: a version picker (one choice per minor line) and a
// libtorrent branch picker shown only for versions that support multiple branches.
// It returns nil when the manifest cannot be loaded.
func InstallOptions() []model.InstallOption {
	m, err := LoadManifest()
	if err != nil {
		return nil
	}

	versionChoices := make([]model.InstallOptionChoice, 0, len(m.Lines))
	branchVersions := make([]string, 0, len(m.Lines))
	availableBranches := make(map[string]bool)
	for _, line := range m.Lines {
		versionChoices = append(versionChoices, model.InstallOptionChoice{
			Value: line.Version,
			Label: line.Version,
		})
		if len(line.Libtorrent.Branches) > 1 {
			branchVersions = append(branchVersions, line.Version)
			for b := range line.Libtorrent.Branches {
				availableBranches[b] = true
			}
		}
	}

	options := []model.InstallOption{{
		Key:     extravars.QbittorrentVersion,
		Label:   "qBittorrent version",
		Type:    "select",
		Choices: versionChoices,
	}}

	if len(branchVersions) > 0 {
		branchOrder := []string{BranchRC12, BranchRC20, BranchRC21}
		branchChoices := make([]model.InstallOptionChoice, 0, len(branchOrder))
		for _, b := range branchOrder {
			if availableBranches[b] {
				branchChoices = append(branchChoices, model.InstallOptionChoice{
					Value: b,
					Label: branchLabel(b),
				})
			}
		}

		options = append(options, model.InstallOption{
			Key:     extravars.LibtorrentBranch,
			Label:   "libtorrent version",
			Type:    "select",
			Choices: branchChoices,
			When: &model.InstallOptionWhen{
				Key:   extravars.QbittorrentVersion,
				OneOf: branchVersions,
			},
		})
	}

	return options
}
