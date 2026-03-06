package saga

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEACStorageNewNilLogger verifies that NewEACStorage handles a nil logger.
func TestEACStorageNewNilLogger(t *testing.T) {
	s := NewEACStorage(nil, nil)
	assert.NotNil(t, s)
	assert.NotNil(t, s.log)
}

// TestEACStorageImplementsStorage verifies the interface compliance at compile time.
func TestEACStorageImplementsStorage(t *testing.T) {
	var _ Storage = (*EACStorage)(nil)
}
