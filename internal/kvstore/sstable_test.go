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

func TestNewSSTable(t *testing.T) {
	const numEntries = 2000

	t.Run("it can be created from a memtable", func(t *testing.T) {
		mt := newMemtable()
		for i := range numEntries {
			mt.set(fmt.Sprintf("key-%d", i), strconv.Itoa(i))
		}

		sst := newSSTable(mt)

		expected := make([]sstableEntry, mt.len())
		for i := range numEntries {
			expected[i] = sstableEntry{
				Key:   fmt.Sprintf("key-%d", i),
				Value: strconv.Itoa(i),
			}
		}

		sort.Slice(expected, func(i, j int) bool {
			return expected[i].Key < expected[j].Key
		})

		assert.Equal(t, sstable(expected), sst)
	})
}

func TestOpenSSTable(t *testing.T) {
	const numEntries = 2000

	t.Run("it can be loaded from a file", func(t *testing.T) {
		tempDir := t.TempDir()
		sstFilename := path.Join(tempDir, "sst-1.json")
		entries := make([]sstableEntry, numEntries)
		for i := range numEntries {
			entries[i] = sstableEntry{
				Key:   fmt.Sprintf("key-%d", i),
				Value: strconv.Itoa(i),
			}
		}

		{
			file, err := os.Create(sstFilename)
			assert.NoError(t, err)
			defer func() {
				_ = file.Close()
			}()
			assert.NoError(t, json.NewEncoder(file).Encode(entries))
		}

		sst, err := openSSTable(sstFilename)

		assert.NoError(t, err)
		assert.Equal(t, sstable(entries), sst)
	})

	t.Run(
		"it reports an error if the file cannot be opened",
		func(t *testing.T) {
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			openRead = func(path string) (*os.File, error) {
				return nil, errors.New("error")
			}

			_, err := openSSTable("does-not-exist")

			assert.ErrorContains(t, err, "unable to open sstable")
		},
	)

	t.Run("it reports an error if the file is invalid", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "sst-1.json")
		{
			file, err := os.Create(filename)
			assert.NoError(t, err)
			defer func() {
				_ = file.Close()
			}()
			_, err = file.Write([]byte("invalid"))
			assert.NoError(t, err)
		}

		_, err := openSSTable(filename)

		assert.ErrorContains(t, err, "unable to decode sstable")
	})
}

func TestSaveSSTable(t *testing.T) {
	const numEntries = 2000

	t.Run("it saves successfully", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "sst-1.json")
		entries := make([]sstableEntry, numEntries)
		for i := range numEntries {
			entries[i] = sstableEntry{
				Key:   fmt.Sprintf("key-%d", i),
				Value: strconv.Itoa(i),
			}
		}

		err := sstable(entries).Save(filename)

		assert.NoError(t, err)
		sst, err := openSSTable(filename)
		assert.NoError(t, err)
		assert.Equal(t, sstable(entries), sst)
	})

	t.Run(
		"it reports an error if the file cannot be created",
		func(t *testing.T) {
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(
				name string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				return nil, errors.New("error")
			}

			st := newSSTable(newMemtable())
			err := st.Save(path.Join(t.TempDir(), "sst-1.json"))

			assert.ErrorContains(t, err, "unable to create sstable file")
		},
	)

	t.Run(
		"it reports an error if the sstable cannot be written",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "sst-1.json")
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(
				name string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				file, err := os.Create(filename)
				assert.NoError(t, err)
				assert.NoError(t, file.Close())
				return file, nil
			}

			st := newSSTable(newMemtable())
			err := st.Save(filename)

			assert.ErrorContains(t, err, "unable to serialize sstable data")
		},
	)

	t.Run(
		"it reports an error if the file cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "sst-1.json")
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("injected error")
			syncFile = func(f *os.File) error {
				return injectedError
			}

			st := newSSTable(newMemtable())
			err := st.Save(filename)

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the parent directory cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "sst-1.json")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("injected error")
			openRead = func(path string) (*os.File, error) {
				return nil, injectedError
			}

			st := newSSTable(newMemtable())
			err := st.Save(filename)

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the parent directory cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "sst-1.json")
			orig := syncDir
			t.Cleanup(func() { syncDir = orig })
			injectedError := errors.New("injected error")
			syncDir = func(f *os.File) error {
				return injectedError
			}

			st := newSSTable(newMemtable())
			err := st.Save(filename)

			assert.ErrorIs(t, err, injectedError)
		},
	)
}

func TestSSTableGet(t *testing.T) {
	mt := newMemtable()
	mt.set("hello", "world")
	mt.set("goodbye", "world")
	mt.delete("goodbye")
	sst := newSSTable(mt)

	t.Run("it returns the sstable value", func(t *testing.T) {
		value, err := sst.Get("hello")

		assert.NoError(t, err)
		assert.Equal(t, value, "world")
	})

	t.Run("it returns an error if the value is not found", func(t *testing.T) {
		_, err := sst.Get("oy")

		assert.ErrorIs(t, err, ErrKeyNotFound)
	})

	t.Run("it returns an error if the key was deleted", func(t *testing.T) {
		_, err := sst.Get("goodbye")

		assert.ErrorIs(t, err, ErrKeyDeleted)
	})
}
