// Copyright 2026 Michael F. Collins, III
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISONG
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package kvstore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetInsertsKeyValuePairIntoStore(t *testing.T) {
	store := newInMemoryStore()

	store.Set("hello", "world")

	value, ok := store["hello"]
	assert.True(t, ok, "the key was not found")
	assert.Equal(t, "world", value, "the value was not expected")
}

func TestSetUpdatesValueInStore(t *testing.T) {
	store := newInMemoryStore()

	store.Set("foo", "bar")
	store.Set("foo", "baz")

	value, ok := store["foo"]
	assert.True(t, ok, "the key was not found")
	assert.Equal(t, "baz", value, "the value was not expected")
}

func TestGetReturnsValueFromStore(t *testing.T) {
	store := newInMemoryStore()
	store.Set("hello", "world")

	value, ok := store.Get("hello")

	assert.True(t, ok, "the key was not found")
	assert.Equal(t, "world", value, "the value was not expected")
}

func TestGetReturnsFalseIfKeyIsNotFound(t *testing.T) {
	store := newInMemoryStore()

	_, ok := store.Get("hello")

	assert.False(t, ok, "the key was found")
}
