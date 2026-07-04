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

func TestMemtableSet(t *testing.T) {
	t.Run("it inserts a key-value pair into the store", func(t *testing.T) {
		mt := newMemtable()

		mt.set("hello", "world")

		value, err := mt.get("hello")
		assert.NoError(t, err, "the key was not found")
		assert.Equal(t, "world", value, "the value was not expected")
	})

	t.Run("it updates a key-value pair", func(t *testing.T) {
		mt := newMemtable()

		mt.set("foo", "bar")
		mt.set("foo", "baz")

		value, err := mt.get("foo")
		assert.NoError(t, err, "the key was not found")
		assert.Equal(t, "baz", value, "the value was not expected")
	})
}

func TestMemtableGet(t *testing.T) {
	t.Run("it returns a value from the store", func(t *testing.T) {
		mt := newMemtable()
		mt.set("hello", "world")

		value, err := mt.get("hello")

		assert.NoError(t, err, "the key was not found")
		assert.Equal(t, "world", value, "the value was not expected")
	})

	t.Run("it returns an error if the key is not found", func(t *testing.T) {
		mt := newMemtable()

		_, err := mt.get("hello")

		assert.ErrorIs(t, err, ErrKeyNotFound, "the key was found")
	})

	t.Run("it returns an error if the key was deleted", func(t *testing.T) {
		mt := newMemtable()

		mt.delete("hello")
		_, err := mt.get("hello")

		assert.ErrorIs(t, err, ErrKeyDeleted)
	})
}

func TestMemtableClear(t *testing.T) {
	t.Run("it clears all the entries in the store", func(t *testing.T) {
		mt := newMemtable()
		mt.set("a", "b")
		mt.set("c", "d")
		mt.set("e", "f")

		mt.clear()

		assert.Empty(t, mt)
	})
}

func TestMemtableDelete(t *testing.T) {
	t.Run("it inserts a tombstone into the store", func(t *testing.T) {
		mt := newMemtable()

		mt.delete("hello")

		entry, ok := mt["hello"]
		assert.True(t, ok, "the key was not found")
		assert.True(t, entry.deleted, "the entry was not deleted")
		assert.Equal(t, "", entry.value, "the value was not expected")
	})
}
