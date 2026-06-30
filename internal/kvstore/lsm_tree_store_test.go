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
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLSMSetStoresValueInMemtable(t *testing.T) {
	store, err := newLSMTreeStore(t.TempDir())
	assert.NoError(t, err)
	assert.NoError(t, store.Set("hello", "world"))

	value, ok := store.mt["hello"]

	assert.True(t, ok, "the key was not found")
	assert.Equal(t, "world", value)
}

func TestLSMGetRetrievesValueFromMemtable(t *testing.T) {
	store, err := newLSMTreeStore(t.TempDir())
	assert.NoError(t, err)
	store.mt["hello"] = "world"

	value, err := store.Get("hello")
	assert.NoError(t, err)
	assert.Equal(t, "world", value)
}

func TestLSMGetReturnsKeyNotFoundErrorIfKeyIsMissing(t *testing.T) {
	store, err := newLSMTreeStore(t.TempDir())
	assert.NoError(t, err)

	_, err = store.Get("hello")

	assert.ErrorIs(t, err, ErrKeyNotFound)
}

func TestLSMSetFlushIfMemtableGetsTooBig(t *testing.T) {
	tempDir := t.TempDir()
	store, err := newLSMTreeStore(tempDir)
	assert.NoError(t, err)

	for i := range 2000 {
		assert.NoError(t, store.Set(fmt.Sprintf("key%d", i), strconv.Itoa(i)))
	}

	t.Run("it empties the memtable", func(t *testing.T) {
		assert.Empty(t, store.mt)
	})

	t.Run("it creates the SSTable", func(t *testing.T) {
		assert.FileExists(t, path.Join(tempDir, "sst-1.json"))
	})

	t.Run("it writes the key-value pairs to the SSTable", func(t *testing.T) {
		var entries []sstableEntry
		file, err := os.Open(path.Join(tempDir, "sst-1.json"))
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()
		assert.NoError(t, json.NewDecoder(file).Decode(&entries))

		expected := make([]sstableEntry, len(entries))
		for i := range 2000 {
			expected[i] = sstableEntry{
				Key:   fmt.Sprintf("key%d", i),
				Value: strconv.Itoa(i),
			}
		}

		sort.Slice(expected, func(i, j int) bool {
			return expected[i].Key < expected[j].Key
		})
		assert.Equal(t, expected, entries)
	})
}

func TestNewLSMTreeStoreFailsIfStatReturnsError(t *testing.T) {
	orig := statFile
	t.Cleanup(func() { statFile = orig })
	statFile = func(name string) (os.FileInfo, error) {
		return nil, errors.New("test error")
	}

	_, err := newLSMTreeStore(t.TempDir())

	assert.Error(t, err)
}

func TestLSMCreateOrLoadManifestReadsExistingManifest(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")
	assert.NoError(t, createTestManifest(manifestPath))

	store, err := newLSMTreeStore(tempDir)
	assert.NoError(t, err)
	assert.Equal(t, 4, store.manifest.nextSSTableID)
}

func TestLSMGetReturnsErrorIfManifestCannotBeRead(t *testing.T) {
	tempDir := t.TempDir()
	manifestPath := path.Join(tempDir, "MANIFEST")
	assert.NoError(t, createTestManifest(manifestPath))

	store, err := newLSMTreeStore(tempDir)
	assert.NoError(t, err)

	for i := range maxMemtableEntries {
		err = store.Set(fmt.Sprintf("key%d", i), strconv.Itoa(i))
		assert.NoError(t, err)
	}

	orig := openRead
	t.Cleanup(func() { openRead = orig })
	openRead = func(path string) (*os.File, error) {
		return nil, errors.New("test error")
	}
	_, err = store.Get("key3")
	assert.Error(t, err)
}

func createTestManifest(filename string) error {
	manifestFile, err := os.Create(filename)
	if err != nil {
		return err
	}

	defer func() {
		_ = manifestFile.Close()
	}()

	_, err = fmt.Fprintln(manifestFile, "sst-1.json")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(manifestFile, "sst-2.json")
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(manifestFile, "sst-3.json")
	if err != nil {
		return err
	}

	return nil
}
