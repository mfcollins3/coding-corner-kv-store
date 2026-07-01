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
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
)

func TestNewManifest(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "MANIFEST")

	t.Run("success", func(t *testing.T) {
		manifest, err := newManifest(filename)

		t.Run("it creates the MANIFEST file", func(t *testing.T) {
			assert.NoError(t, err)
			_, err = os.Stat(filename)
			assert.NoError(t, err)
		})

		t.Run("it sets the next SSTable ID to 1", func(t *testing.T) {
			assert.Equal(t, 1, manifest.nextSSTableID)
		})

		t.Run("it has no SSTables", func(t *testing.T) {
			assert.Empty(t, manifest.sstables)
		})
	})

	t.Run(
		"it reports an error if the manifest cannot be created",
		func(t *testing.T) {
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(
				path string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				return nil, errors.New("some error")
			}

			_, err := newManifest(filename)

			assert.ErrorContains(t, err, "unable to create manifest file")
		},
	)
}

func TestOpenManifest(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "MANIFEST")

	t.Run("it successfully loads the manifest", func(t *testing.T) {
		{
			file, err := os.Create(filename)
			assert.NoError(t, err)
			defer func() {
				_ = file.Close()
			}()
			_, err = file.Write(
				[]byte(
					`sst-1.json
sst-2.json
sst-3.json`,
				),
			)
			assert.NoError(t, err)
		}

		manifest, err := openManifest(filename)

		assert.NoError(t, err)
		assert.Equal(t, 3, len(manifest.sstables))
		assert.Equal(t, 4, manifest.nextSSTableID)
		assert.Equal(
			t,
			[]string{"sst-3.json", "sst-2.json", "sst-1.json"},
			manifest.sstables,
		)
	})

	t.Run(
		"it reports an error if the manifest cannot be opened",
		func(t *testing.T) {
			injectedError := errors.New("some error")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			openRead = func(name string) (*os.File, error) {
				return nil, injectedError
			}

			_, err := openManifest(filename)

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the manifest is invalid",
		func(t *testing.T) {
			{
				file, err := os.Create(filename)
				assert.NoError(t, err)
				defer func() {
					_ = file.Close()
				}()
				_, err = file.Write(
					[]byte("sst-1.json\ninvalid-sstable-name\nsst-2.json"),
				)
				assert.NoError(t, err)
			}

			_, err := openManifest(filename)

			assert.ErrorContains(t, err, "unable to parse sst id")
		},
	)
}

func TestSSTableParseManifest(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		manifestContent := `sst-1.json
sst-2.json
sst-3.json
sst-4.json`
		sstables, nextID, err := parseManifest(
			strings.NewReader(manifestContent),
		)

		t.Run("it returns the sstables", func(t *testing.T) {
			expected := []string{
				"sst-4.json",
				"sst-3.json",
				"sst-2.json",
				"sst-1.json",
			}
			assert.Equal(t, expected, sstables)
		})

		t.Run("it returns the next SSTable ID", func(t *testing.T) {
			assert.Equal(t, 5, nextID)
		})

		t.Run("it does not report an error", func(t *testing.T) {
			assert.NoError(t, err)
		})
	})

	t.Run(
		"returns an error if the manifest cannot be parsed",
		func(t *testing.T) {
			manifestContent := `sst-1.json
sst-test.json
sst-2.json`

			_, _, err := parseManifest(strings.NewReader(manifestContent))

			assert.ErrorContains(t, err, "unable to parse sst id")
		},
	)

	t.Run(
		"it reports an error if the manifest cannot be read",
		func(t *testing.T) {
			injectedErr := errors.New("read failed")

			_, _, err := parseManifest(iotest.ErrReader(injectedErr))

			assert.ErrorIs(t, err, injectedErr)
		},
	)
}

func TestSSTableNextSSTableFilename(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "MANIFEST")
	{
		manifestContent := `sst-1.json
sst-2.json
sst-3.json
sst-4.json`
		file, err := os.Create(filename)
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()
		_, err = file.Write([]byte(manifestContent))
		assert.NoError(t, err)
	}

	manifest, err := openManifest(filename)
	assert.NoError(t, err)

	t.Run("it returns the SSTable filename", func(t *testing.T) {
		assert.Equal(
			t,
			path.Join(tempDir, "sst-5.json"),
			manifest.nextSSTableFilename(),
		)
	})

	t.Run("it increments the next SSTable ID", func(t *testing.T) {
		assert.Equal(t, 6, manifest.nextSSTableID)
	})
}

func TestManifestAddSSTable(t *testing.T) {
	tempDir := t.TempDir()
	filename := filepath.Join(tempDir, "MANIFEST")

	{
		manifestContent := `sst-1.json
sst-2.json
sst-3.json
sst-4.json
`
		file, err := os.Create(filename)
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()
		_, err = file.Write([]byte(manifestContent))
		assert.NoError(t, err)
	}

	manifest, err := openManifest(filename)
	assert.NoError(t, err)

	t.Run("it appends the SSTable to the manifest file", func(t *testing.T) {
		err = manifest.addSSTable(path.Join(tempDir, "sst-5.json"))
		assert.NoError(t, err)

		file, err := os.Open(filename)
		assert.NoError(t, err)
		content, err := io.ReadAll(file)
		assert.NoError(t, err)
		assert.Equal(
			t,
			`sst-1.json
sst-2.json
sst-3.json
sst-4.json
sst-5.json
`,
			string(content),
		)
	})

	t.Run(
		"it reports an error if the manifest cannot be opened",
		func(t *testing.T) {
			injectedError := errors.New("some error")
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(
				name string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				return nil, injectedError
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the SSTable cannot be appended to the file",
		func(t *testing.T) {
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			openFile = func(
				name string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				file, err := os.OpenFile(name, flag, perm)
				assert.NoError(t, err)
				assert.NoError(t, file.Close())
				return file, nil
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorContains(
				t,
				err,
				"unable to write updated manifest",
			)
		},
	)

	t.Run(
		"it reports an error if the temporary manifest file cannot be synced",
		func(t *testing.T) {
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("some error")
			syncFile = func(f *os.File) error {
				return injectedError
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the manifest file cannot be renamed",
		func(t *testing.T) {
			orig := renameFile
			t.Cleanup(func() { renameFile = orig })
			injectedError := errors.New("some error")
			renameFile = func(oldpath, newpath string) error {
				return injectedError
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the manifest file cannot be opened",
		func(t *testing.T) {
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("some error")
			openRead = func(name string) (*os.File, error) {
				if name == manifest.filename {
					return nil, injectedError
				}

				return orig(name)
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the manifest file cannot be synced",
		func(t *testing.T) {
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("some error")
			syncFile = func(f *os.File) error {
				if f.Name() == manifest.filename {
					return injectedError
				}

				return orig(f)
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the directory cannot be opened",
		func(t *testing.T) {
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("some error")
			openRead = func(name string) (*os.File, error) {
				if name == path.Dir(manifest.filename) {
					return nil, injectedError
				}

				return orig(name)
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)

	t.Run(
		"it reports an error if the directory cannot be synced",
		func(t *testing.T) {
			orig := syncDir
			t.Cleanup(func() { syncDir = orig })
			injectedError := errors.New("some error")
			syncDir = func(f *os.File) error {
				return injectedError
			}

			err := manifest.addSSTable(path.Join(tempDir, "sst-5.json"))

			assert.ErrorIs(t, err, injectedError)
		},
	)
}
