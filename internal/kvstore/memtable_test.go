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

		value, ok := mt["hello"]
		assert.True(t, ok, "the key was not found")
		assert.Equal(t, "world", value, "the value was not expected")
	})

	t.Run("it updates a key-value pair", func(t *testing.T) {
		mt := newMemtable()

		mt.set("foo", "bar")
		mt.set("foo", "baz")

		value, ok := mt["foo"]
		assert.True(t, ok, "the key was not found")
		assert.Equal(t, "baz", value, "the value was not expected")
	})
}

func TestMemtableGet(t *testing.T) {
	t.Run("it returns a value from the store", func(t *testing.T) {
		mt := newMemtable()
		mt.set("hello", "world")

		value, ok := mt.get("hello")

		assert.True(t, ok, "the key was not found")
		assert.Equal(t, "world", value, "the value was not expected")
	})

	t.Run("it returns false if the key is not found", func(t *testing.T) {
		mt := newMemtable()

		_, ok := mt.get("hello")

		assert.False(t, ok, "the key was found")
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
