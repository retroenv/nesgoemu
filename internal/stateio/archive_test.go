package stateio

import (
	"bytes"
	"testing"

	"github.com/retroenv/retrogolib/assert"
)

func TestRestoreKeepsMemoryAliases(t *testing.T) {
	const (
		storedValue  = 0x34
		updatedValue = 0x56
	)

	var stored bytes.Buffer
	assert.NoError(t, NewWriter(&stored).Bytes([]byte{0x12, storedValue}))
	memory := make([]byte, 2)
	alias := memory[1:]
	reader := NewReader(&stored)
	assert.NoError(t, reader.Bytes(memory))
	assert.Equal(t, byte(storedValue), alias[0])
	alias[0] = updatedValue
	assert.Equal(t, byte(updatedValue), memory[1])
	assert.NoError(t, reader.End())
}

func TestRestoreRejectsWrongMemorySizeWithoutCopy(t *testing.T) {
	var stored bytes.Buffer
	assert.NoError(t, NewWriter(&stored).Bytes([]byte{1, 2, 3}))
	original := []byte{0x12, 0x34}
	memory := bytes.Clone(original)
	assert.ErrorContains(t, NewReader(&stored).Bytes(memory), "memory size")
	assert.Equal(t, original, memory)
}

func TestRestoreRejectsExtraRecords(t *testing.T) {
	var stored bytes.Buffer
	writer := NewWriter(&stored)
	assert.NoError(t, writer.Values(true, false))
	reader := NewReader(&stored)
	var value bool
	assert.NoError(t, reader.Values(&value))
	assert.True(t, value)
	assert.ErrorContains(t, reader.End(), "extra record")
}
