package mcpserver

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

const (
	debugFlag = "-debug"
	rootFlag  = "-root"
)

func TestExtractRoots(t *testing.T) {
	roots, rest, err := extractRoots([]string{rootFlag, "/a", debugFlag, "--root=/b", "-root=/c"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"/a", "/b", "/c"}, roots)
	assert.Equal(t, []string{debugFlag}, rest)

	roots, rest, err = extractRoots(nil)
	assert.NoError(t, err)
	assert.Empty(t, roots)
	assert.Empty(t, rest)
}

func TestExtractRootsRejectsMissingDirectory(t *testing.T) {
	for _, args := range [][]string{
		{rootFlag},
		{debugFlag, "--root"},
		{"-root="},
		{rootFlag, ""},
		{rootFlag, "  "},
	} {
		_, _, err := extractRoots(args)
		assert.ErrorContains(t, err, "needs a directory", "args %v", args)
	}
}

func TestMainRejectsPositionalArguments(t *testing.T) {
	err := Main([]string{"/roms"}, "test")
	assert.ErrorContains(t, err, "unexpected argument")
	err = Main([]string{"-transport", "carrier"}, "test")
	assert.ErrorContains(t, err, "unknown transport")
}
