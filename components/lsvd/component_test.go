package lsvd

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentNewComponent(t *testing.T) {
	log := slog.Default()
	dataPath := "/var/lib/test"

	comp := NewComponent(log, dataPath)

	assert.NotNil(t, comp)
	assert.Equal(t, dataPath, comp.dataPath)
	assert.False(t, comp.IsRunning())
}

func TestComponentStopNotRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	ctx := context.Background()
	err := comp.Stop(ctx)

	assert.NoError(t, err)
}

func TestComponentIsRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	assert.False(t, comp.IsRunning())
}

func TestComponentPIDNotRunning(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	_, err := comp.PID()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestComponentClose(t *testing.T) {
	log := slog.Default()
	tempDir := t.TempDir()

	comp := NewComponent(log, tempDir)

	err := comp.Close()
	assert.NoError(t, err)
}
