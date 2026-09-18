package qbittorrent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/brrewery/internal/apps/qbittorrent"
)

func TestLoadManifest_pinsBuildDependenciesPerLine(t *testing.T) {
	t.Parallel()

	m, err := qbittorrent.LoadManifest()
	require.NoError(t, err)

	line, ok := m.LineForVersion("5.2")
	require.True(t, ok)
	assert.Equal(t, "6.11.1", line.Qt)
	assert.Equal(t, "6.5", line.QtMin)
	assert.Equal(t, "1.3.2", line.Zlib)
	assert.Equal(t, "3.6.3", line.Openssl)
	assert.Equal(t, "-O3 -mtune=native", line.CompilerFlags)
	assert.Equal(t, "1_86_0", line.Libtorrent.Branches["RC_1_2"].Boost)
	assert.Equal(t, "1_91_0", line.Libtorrent.Branches["RC_2_0"].Boost)
}
