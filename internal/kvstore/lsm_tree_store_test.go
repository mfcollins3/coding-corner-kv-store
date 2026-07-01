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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLSMTreeStore(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("it creates a new LSM tree store", func(t *testing.T) {
		orig := statFile
		t.Cleanup(func() { statFile = orig })
		statFile = func(name string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		}

		store, err := newLSMTreeStore(tempDir)
		assert.NoError(t, err)
		defer func() {
			_ = store.Close()
		}()

		assert.NotNil(t, store)
		assert.Empty(t, store.mt)
		assert.Equal(t, tempDir, store.path)
		assert.Equal(t, 1, store.manifest.nextSSTableID)
		assert.NotNil(t, store.negativeCache)
	})

	t.Run("it opens an existing LSM tree store", func(t *testing.T) {
		{
			file, err := os.Create(path.Join(tempDir, "MANIFEST"))
			assert.NoError(t, err)
			defer func() {
				_ = file.Close()
			}()
			_, err = file.Write([]byte("sst-1.json\nsst-2.json\nsst-3.json\n"))
			assert.NoError(t, err)
		}

		createFile := func(name string) {
			file, err := os.Create(path.Join(tempDir, name))
			assert.NoError(t, err)
			assert.NoError(t, file.Close())
		}
		createFile("sst-1.json")
		createFile("sst-2.json")
		createFile("sst-3.json")

		t.Run("it succeeds", func(t *testing.T) {
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			assert.NotNil(t, store)
			assert.Empty(t, store.mt)
			assert.Equal(t, tempDir, store.path)
			assert.Equal(t, 4, store.manifest.nextSSTableID)
			assert.NotNil(t, store.negativeCache)
		})

		t.Run(
			"it returns an error if the MANIFEST cannot be opened",
			func(t *testing.T) {
				injectedError := errors.New("injected error")
				orig := openRead
				t.Cleanup(func() { openRead = orig })
				openRead = func(name string) (*os.File, error) {
					if strings.HasSuffix(name, "MANIFEST") {
						return nil, injectedError
					}

					return orig(name)
				}

				_, err := newLSMTreeStore(tempDir)

				assert.ErrorIs(t, err, injectedError)
			},
		)

		t.Run(
			"it returns an error if the WAL cannot be created",
			func(t *testing.T) {
				injectedError := errors.New("injected error")
				orig := openFile
				t.Cleanup(func() { openFile = orig })
				openFile = func(
					name string,
					flag int,
					perm os.FileMode,
				) (*os.File, error) {
					if strings.HasSuffix(name, "wal.db") {
						return nil, injectedError
					}

					return orig(name, flag, perm)
				}

				_, err := newLSMTreeStore(tempDir)

				assert.ErrorIs(t, err, injectedError)
				assert.ErrorContains(t, err, "unable to open write-ahead log")
			},
		)

		t.Run(
			"it returns an error if the WAL cannot be replayed",
			func(t *testing.T) {
				injectedError := errors.New("injected error")
				orig := statFile
				t.Cleanup(func() { statFile = orig })
				statFile = func(name string) (os.FileInfo, error) {
					if strings.HasSuffix(name, "wal.db") {
						return nil, injectedError
					}

					return orig(name)
				}

				_, err := newLSMTreeStore(tempDir)

				assert.ErrorIs(t, err, injectedError)
				assert.ErrorContains(t, err, "failed to replay log")
			},
		)

		t.Run(
			"it returns an error of dangling SSTables cannot be deleted",
			func(t *testing.T) {
				injectedError := errors.New("injected error")
				orig := findFiles
				t.Cleanup(func() { findFiles = orig })
				findFiles = func(path string) ([]string, error) {
					return []string{}, injectedError
				}

				_, err := newLSMTreeStore(tempDir)

				assert.ErrorIs(t, err, injectedError)
				assert.ErrorContains(
					t,
					err,
					"unable to clear dangling sstable files",
				)
			},
		)
	})
}

func TestLSMTreeStoreGet(t *testing.T) {
	tempDir := t.TempDir()

	t.Run(
		"it returns an error if the key is in the negative cache",
		func(t *testing.T) {
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			store.negativeCache["test"] = struct{}{}

			_, err = store.Get("test")

			assert.ErrorIs(t, err, ErrKeyNotFound)
		},
	)

	t.Run(
		"it returns the value if the key is in the memtable",
		func(t *testing.T) {
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			store.mt.set("hello", "world")

			val, err := store.Get("hello")

			assert.NoError(t, err)
			assert.Equal(t, "world", val)
		},
	)

	t.Run(
		"it returns the value if the key is found in an SSTable",
		func(t *testing.T) {
			{
				manifestFilename := path.Join(tempDir, "MANIFEST")
				file, err := os.Create(manifestFilename)
				assert.NoError(t, err)
				defer func() {
					_ = file.Close()
				}()
				_, err = file.Write([]byte("sst-1.json\n"))
				assert.NoError(t, err)

				sstableFilename := path.Join(tempDir, "sst-1.json")
				sstableFile, err := os.Create(sstableFilename)
				assert.NoError(t, err)
				defer func() {
					_ = sstableFile.Close()
				}()
				_, err = sstableFile.Write([]byte(
					"[{\"key\":\"hello\",\"value\":\"world\"}]",
				))
				assert.NoError(t, err)
			}

			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			val, err := store.Get("hello")

			assert.NoError(t, err)
			assert.Equal(t, "world", val)
		},
	)

	t.Run(
		"it returns an error if the sstable cannot be opened",
		func(t *testing.T) {
			{
				manifestFilename := path.Join(tempDir, "MANIFEST")
				file, err := os.Create(manifestFilename)
				assert.NoError(t, err)
				defer func() {
					_ = file.Close()
				}()
				_, err = file.Write([]byte("sst-1.json\n"))
				assert.NoError(t, err)

				sstableFilename := path.Join(tempDir, "sst-1.json")
				sstableFile, err := os.Create(sstableFilename)
				assert.NoError(t, err)
				defer func() {
					_ = sstableFile.Close()
				}()
				_, err = sstableFile.Write([]byte(
					"[{\"key\":\"hello\",\"value\":\"world\"}]",
				))
				assert.NoError(t, err)
			}

			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			injectedError := errors.New("injected error")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			openRead = func(name string) (*os.File, error) {
				if strings.HasSuffix(name, "sst-1.json") {
					return nil, injectedError
				}

				return orig(name)
			}

			_, err = store.Get("test")

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it adds the key to the negative cache if not found",
		func(t *testing.T) {
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			_, err = store.Get("test")

			assert.ErrorIs(t, err, ErrKeyNotFound)
			_, ok := store.negativeCache["test"]
			assert.True(t, ok)
		},
	)
}

func TestLSMTreeStoreSet(t *testing.T) {
	t.Run("it adds the value to the memstore", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := newLSMTreeStore(tempDir)
		assert.NoError(t, err)
		defer func() {
			_ = store.Close()
		}()

		err = store.Set("test", "passed")
		assert.NoError(t, err)
		val, ok := store.mt["test"]
		assert.True(t, ok)
		assert.Equal(t, "passed", val)
	})

	t.Run("it flushes the memtable to an SSTable", func(t *testing.T) {
		tempDir := t.TempDir()
		store, err := newLSMTreeStore(tempDir)
		assert.NoError(t, err)
		defer func() {
			_ = store.Close()
		}()

		for i := range maxMemtableEntries {
			assert.NoError(
				t,
				store.Set(fmt.Sprintf("key-%d", i), strconv.Itoa(i)),
			)
		}

		assert.FileExists(t, path.Join(tempDir, "sst-1.json"))

		manifestFilename := path.Join(tempDir, "MANIFEST")
		manifestFile, err := os.Open(manifestFilename)
		assert.NoError(t, err)
		defer func() {
			_ = manifestFile.Close()
		}()
		data, err := io.ReadAll(manifestFile)
		assert.NoError(t, err)
		assert.Equal(t, "sst-1.json\n", string(data))
	})

	t.Run(
		"it returns an error if the memtable cannot be flushed",
		func(t *testing.T) {
			tempDir := t.TempDir()
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			for i := range maxMemtableEntries - 1 {
				assert.NoError(
					t,
					store.Set(fmt.Sprintf("key-%d", i), strconv.Itoa(i)),
				)
			}

			injectedError := errors.New("injected error")
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
				return nil, injectedError
			}

			err = store.Set("test", "failed")

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it returns an error if the manifest cannot be updated",
		func(t *testing.T) {
			tempDir := t.TempDir()
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			for i := range maxMemtableEntries - 1 {
				assert.NoError(
					t,
					store.Set(fmt.Sprintf("key-%d", i), strconv.Itoa(i)),
				)
			}

			injectedError := errors.New("injected error")
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
				if !strings.HasSuffix(name, "MANIFEST.tmp") {
					return os.OpenFile(name, flag, perm)
				}

				return nil, injectedError
			}

			err = store.Set("test", "failed")

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "failed to add sstable to manifest")
		},
	)

	t.Run(
		"it returns an error if the WAL log fails",
		func(t *testing.T) {
			tempDir := t.TempDir()
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			injectedError := errors.New("injected error")
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			syncFile = func(f *os.File) error {
				if f == store.wal.file {
					return injectedError
				}

				return orig(f)
			}

			err = store.Set("test", "failed")

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to write key test to the log")
		},
	)

	t.Run(
		"it returns an error if the WAL cannot be truncated",
		func(t *testing.T) {
			tempDir := t.TempDir()
			store, err := newLSMTreeStore(tempDir)
			assert.NoError(t, err)
			defer func() {
				_ = store.Close()
			}()

			for i := range maxMemtableEntries - 1 {
				assert.NoError(
					t,
					store.Set(fmt.Sprintf("key-%d", i), strconv.Itoa(i)),
				)
			}

			injectedError := errors.New("injected error")
			orig := truncateFile
			t.Cleanup(func() { truncateFile = orig })
			truncateFile = func(f *os.File, size int64) error {
				return injectedError
			}

			err = store.Set("test", "failed")

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "failed to truncate write-ahead log")
		},
	)
}

func TestClearDanglingSSTables(t *testing.T) {
	createFile := func(name string) {
		file, err := os.Create(name)
		assert.NoError(t, err)
		_ = file.Close()
	}

	t.Run("it returns an error if the file search fails", func(t *testing.T) {
		tempDir := t.TempDir()
		orig := findFiles
		t.Cleanup(func() { findFiles = orig })
		injectedError := errors.New("injected error")
		findFiles = func(dir string) ([]string, error) {
			return nil, injectedError
		}

		err := clearDanglingSSTables(
			tempDir,
			[]string{"sst-1.json", "sst-2.json", "sst-3.json"},
		)

		assert.ErrorIs(t, err, injectedError)
		assert.ErrorContains(t, err, "unable to find SST tables")
	})

	t.Run(
		"it returns an error if a file cannot be removed",
		func(t *testing.T) {
			tempDir := t.TempDir()
			orig := removeFile
			t.Cleanup(func() { removeFile = orig })
			injectedError := errors.New("injected error")
			removeFile = func(name string) error {
				return injectedError
			}
			createFile(path.Join(tempDir, "sst-1.json"))
			createFile(path.Join(tempDir, "sst-2.json"))

			err := clearDanglingSSTables(tempDir, []string{"sst-1.json"})

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to remove dangling SST table")
		},
	)

	t.Run(
		"it returns an error if the directory cannot be opened",
		func(t *testing.T) {
			tempDir := t.TempDir()
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("injected error")
			openRead = func(name string) (*os.File, error) {
				return nil, injectedError
			}
			createFile(path.Join(tempDir, "sst-1.json"))
			createFile(path.Join(tempDir, "sst-2.json"))

			err := clearDanglingSSTables(tempDir, []string{"sst-1.json"})

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to open directory")
		},
	)

	t.Run(
		"it returns an error if the directory cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("injected error")
			syncFile = func(f *os.File) error {
				if f.Name() == tempDir {
					return injectedError
				}

				return orig(f)
			}
			createFile(path.Join(tempDir, "sst-1.json"))
			createFile(path.Join(tempDir, "sst-2.json"))

			err := clearDanglingSSTables(tempDir, []string{"sst-1.json"})

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to sync directory")
		},
	)
}
