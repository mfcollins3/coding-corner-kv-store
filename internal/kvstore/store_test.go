package kvstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStoreReturnsLSMTreeStore(t *testing.T) {
	store, err := newLSMTreeStore(t.TempDir())
	assert.NoError(t, err)
	assert.IsType(t, &lsmTreeStore{}, store)
}
