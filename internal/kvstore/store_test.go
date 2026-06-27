package kvstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStoreReturnsInMemoryStore(t *testing.T) {
	store := NewStore()
	assert.IsType(t, inMemoryStore{}, store)
}
