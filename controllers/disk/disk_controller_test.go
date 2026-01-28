package disk

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiskController_New(t *testing.T) {
	log := slog.Default()
	controller := NewDiskController(log, nil, "test-node")

	assert.NotNil(t, controller)
	assert.NotNil(t, controller.Log)
	assert.Equal(t, "/var/lib/miren/disks", controller.mountBasePath)
	assert.Equal(t, "test-node", controller.NodeId)
}
