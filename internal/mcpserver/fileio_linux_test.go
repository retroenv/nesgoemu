package mcpserver

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/retroenv/retrogolib/assert"
	"github.com/retroenv/retrogolib/mcp"
)

func TestCreateMachineRefusesFIFO(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe.nes")
	assert.NoError(t, syscall.Mkfifo(fifo, 0o600))
	server := newTestServer(t, Options{
		Roots: []string{root},
	})
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := server.createMachine(ctx, mcp.ToolRequest{}, createMachineArgs{
			ROM: fifo,
		})
		done <- err
	}()
	blocked := false
	select {
	case err := <-done:
		assert.ErrorContains(t, err, "not a regular file")
	case <-time.After(100 * time.Millisecond):
		blocked = true
	}
	if blocked {
		release, err := os.OpenFile(fifo, os.O_RDWR|syscall.O_NONBLOCK, 0)
		assert.NoError(t, err)
		_, err = release.Write(make([]byte, 16))
		assert.NoError(t, err)
		assert.NoError(t, release.Close())
		select {
		case err := <-done:
			assert.Error(t, err)
		case <-time.After(time.Second):
			t.Fatal("FIFO load did not finish")
		}
	}
	assert.False(t, blocked, "canceled create_machine remains blocked on a FIFO and holds the load slot")
}
