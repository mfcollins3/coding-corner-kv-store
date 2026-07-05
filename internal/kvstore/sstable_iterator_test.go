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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSTableIterator(t *testing.T) {
	entries := sstable{
		{
			Key:     "hello",
			Value:   "world",
			Deleted: false,
		},
		{
			Key:     "fruit",
			Value:   "apple",
			Deleted: false,
		},
		{
			Key:     "car",
			Value:   "",
			Deleted: true,
		},
	}

	t.Run("newSSTableIterator", func(t *testing.T) {
		t.Run("it successfully opens the SSTable", func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "sstable.json")
			{
				file, err := os.Create(filename)
				assert.NoError(t, err)
				encoder := json.NewEncoder(file)
				assert.NoError(t, encoder.Encode(entries))
			}

			it, err := newSSTableIterator(filename)

			assert.NoError(t, err)
			assert.NotNil(t, it)
			current, ok := it.current()
			assert.True(t, ok)
			assert.Equal(t, "hello", current.Key)
		})

		t.Run("it fails if the file cannot be opened", func(t *testing.T) {
			tempDir := t.TempDir()
			filename := filepath.Join(tempDir, "sstable.json")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("injected error")
			openRead = func(name string) (*os.File, error) {
				return nil, injectedError
			}

			_, err := newSSTableIterator(filename)

			assert.ErrorIs(t, err, injectedError)
		})
	})

	t.Run("current", func(t *testing.T) {
		t.Run("it returns the current item", func(t *testing.T) {
			it := sstableIterator{
				sstable: entries,
				index:   0,
			}

			value, ok := it.current()

			assert.True(t, ok)
			assert.Equal(t, "world", value.Value)
		})

		t.Run(
			"it returns false if the iterator is past the end",
			func(t *testing.T) {
				it := sstableIterator{
					sstable: entries,
					index:   4,
				}

				_, ok := it.current()

				assert.False(t, ok)
			},
		)
	})

	t.Run("next", func(t *testing.T) {
		t.Run("it returns the next item", func(t *testing.T) {
			it := sstableIterator{
				sstable: entries,
				index:   0,
			}

			value, ok := it.next()

			assert.True(t, ok)
			assert.Equal(t, "apple", value.Value)
		})

		t.Run(
			"it returns false if the iterator is past the end",
			func(t *testing.T) {
				it := sstableIterator{
					sstable: entries,
					index:   2,
				}

				_, ok := it.next()

				assert.False(t, ok)
			},
		)
	})
}
