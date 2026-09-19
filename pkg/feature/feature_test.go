package feature

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestSetInventory(t *testing.T) {
	set := NewSet()
	set.Declare(CHRBanking)
	set.Declare(Mirroring)
	set.Declare(PRGBanking)

	set.Mark(CHRBanking)

	usage := set.Features()
	assert.Equal(t, []Usage{
		{
			ID:   CHRBanking,
			Name: "CHR banking",
			Used: true,
		},
		{
			ID:   Mirroring,
			Name: "Mirroring",
		},
		{
			ID:   PRGBanking,
			Name: "PRG banking",
		},
	}, usage)
}

func TestSetIgnoresDuplicatesAndUnknownIDs(t *testing.T) {
	set := NewSet()
	set.Declare(PRGRAM)
	set.Declare(PRGRAM)
	set.Declare(invalid)
	set.Declare(count)
	set.Mark(invalid)
	set.Mark(count)

	usage := set.Features()
	assert.Equal(t, []Usage{
		{
			ID:   PRGRAM,
			Name: "PRG RAM",
		},
	}, usage)
}

func TestSetEmpty(t *testing.T) {
	set := NewSet()

	assert.Empty(t, set.Features())
}

func TestAllIDsHaveNames(t *testing.T) {
	for id := ID(1); id < count; id++ {
		assert.NotEmpty(t, names[id], "feature %d must have a name", id)
	}
}
