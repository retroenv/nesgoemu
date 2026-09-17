package sprites

import (
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestDataReturnsPrimaryOAMCopy(t *testing.T) {
	sprites := &Sprites{}
	sprites.SetAddress(0)
	for _, value := range []byte{12, 34, 56, 78} {
		sprites.Write(value)
	}

	data := sprites.Data()
	assert.Equal(t, [4]byte{12, 34, 56, 78}, [4]byte(data[:4]))

	data[0] = 99
	assert.Equal(t, byte(12), sprites.Data()[0])
}
