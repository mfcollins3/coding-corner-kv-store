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
	"hash/crc32"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewWriteAheadLog(t *testing.T) {
	t.Run("it succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "wal.db")

		wal, err := newWriteAheadLog(filename)
		assert.NoError(t, err)
		_ = wal.Close()

		assert.FileExists(t, filename)
	})

	t.Run(
		"it returns an error if the file could not be created",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := openFile
			t.Cleanup(func() { openFile = orig })
			injectedError := errors.New("injected error")
			openFile = func(
				name string,
				flag int,
				perm os.FileMode,
			) (*os.File, error) {
				return nil, injectedError
			}

			_, err := newWriteAheadLog(filename)

			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(
				t,
				err,
				"error creating or opening write-ahead log file",
			)
		},
	)
}

func TestReplayWriteAheadLog(t *testing.T) {
	entries := []logEntry{
		{
			Operation: operationPut.String(),
			Key:       "fruit",
			Value:     "apple",
			Checksum:  crc32.ChecksumIEEE([]byte("put,fruit,apple")),
		},
		{
			Operation: operationPut.String(),
			Key:       "model",
			Value:     "ferrari",
			Checksum:  crc32.ChecksumIEEE([]byte("put,model,ferrari")),
		},
		{
			Operation: operationPut.String(),
			Key:       "dog",
			Value:     "bella",
			Checksum:  crc32.ChecksumIEEE([]byte("put,dog,bella")),
		},
		{
			Operation: operationPut.String(),
			Key:       "child",
			Value:     "alexandra",
			Checksum:  crc32.ChecksumIEEE([]byte("put,child,alexandra")),
		},
	}
	writeSampleData := func(filename string) {
		file, err := os.Create(filename)
		assert.NoError(t, err)
		defer func() {
			_ = file.Close()
		}()
		encoder := json.NewEncoder(file)
		for _, entry := range entries {
			err := encoder.Encode(entry)
			assert.NoError(t, err)
		}
	}

	t.Run("it succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "wal.db")
		writeSampleData(filename)

		mt := newMemtable()
		assert.NoError(t, replayWriteAheadLog(filename, mt))
		assert.Equal(t, len(entries), mt.len())
		for _, entry := range entries {
			value, ok := mt.get(entry.Key)
			assert.True(t, ok)
			assert.Equal(t, entry.Value, value)
		}
	})

	t.Run(
		"it returns an empty memtable if the WAL does not exist",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := statFile
			t.Cleanup(func() { statFile = orig })
			statFile = func(name string) (os.FileInfo, error) {
				return nil, os.ErrNotExist
			}

			mt := newMemtable()
			assert.NoError(t, replayWriteAheadLog(filename, mt))
			assert.Equal(t, 0, mt.len())
		},
	)

	t.Run(
		"it returns an error if it cannot determine in the file exists",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := statFile
			t.Cleanup(func() { statFile = orig })
			injectedError := errors.New("injected error")
			statFile = func(name string) (os.FileInfo, error) {
				return nil, injectedError
			}

			mt := newMemtable()
			err := replayWriteAheadLog(filename, mt)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(
				t,
				err,
				fmt.Sprintf("error checking if file %s exists", filename),
			)
		},
	)

	t.Run(
		"it returns an error if the WAL cannot be opened",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			injectedError := errors.New("injected error")
			openRead = func(name string) (*os.File, error) {
				return nil, injectedError
			}
			writeSampleData(filename)

			mt := newMemtable()
			err := replayWriteAheadLog(filename, mt)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "error opening file")
		},
	)

	t.Run(
		"it returns an error if the file cannot be read",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := openRead
			t.Cleanup(func() { openRead = orig })
			openRead = func(name string) (*os.File, error) {
				file, err := os.Open(name)
				assert.NoError(t, err)
				assert.NoError(t, file.Close())
				return file, nil
			}
			writeSampleData(filename)

			mt := newMemtable()
			err := replayWriteAheadLog(filename, mt)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "error scanning file")
		},
	)

	t.Run(
		"it returns an error if the file is unparseable",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			{
				file, err := os.Create(filename)
				assert.NoError(t, err)
				defer func() {
					_ = file.Close()
				}()
				_, err = fmt.Fprintln(
					file,
					"four score and seven years ago...",
				)
			}

			mt := newMemtable()
			err := replayWriteAheadLog(filename, mt)
			assert.Error(t, err)
			assert.ErrorContains(t, err, "error unmarshalling line")
		},
	)

	t.Run(
		"it stops if a checksum mismatch occurs",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			entries := []logEntry{
				{
					Operation: operationPut.String(),
					Key:       "fruit",
					Value:     "apple",
					Checksum:  crc32.ChecksumIEEE([]byte("put,fruit,apple")),
				},
				{
					Operation: operationPut.String(),
					Key:       "model",
					Value:     "ferrari",
					Checksum:  crc32.ChecksumIEEE([]byte("put,model,ferrari")),
				},
				{
					Operation: operationPut.String(),
					Key:       "dog",
					Value:     "bella",
					Checksum:  1,
				},
				{
					Operation: operationPut.String(),
					Key:       "child",
					Value:     "alexandra",
					Checksum:  crc32.ChecksumIEEE([]byte("put,child,alexandra")),
				},
			}
			{
				file, err := os.Create(filename)
				assert.NoError(t, err)
				defer func() {
					_ = file.Close()
				}()
				encoder := json.NewEncoder(file)
				for _, entry := range entries {
					err := encoder.Encode(entry)
					assert.NoError(t, err)
				}
			}

			mt := newMemtable()
			err := replayWriteAheadLog(filename, mt)
			
			assert.NoError(t, err)
			assert.Equal(t, 2, mt.len())
		},
	)
}

func TestWALLog(t *testing.T) {
	t.Run("it succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "wal.db")
		{
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()
			assert.NoError(t, wal.log(operationPut, "fruit", "apple"))
			assert.NoError(t, wal.log(operationPut, "car", "ferrari"))
			assert.NoError(t, wal.log(operationPut, "child", "alexandra"))
		}

		mt := newMemtable()
		assert.NoError(t, replayWriteAheadLog(filename, mt))
		assert.Equal(t, 3, mt.len())
		value, ok := mt.get("fruit")
		assert.True(t, ok)
		assert.Equal(t, "apple", value)
	})

	t.Run(
		"it returns an error if the entry cannot be serialized",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := marshalJSON
			t.Cleanup(func() { marshalJSON = orig })
			injectedError := errors.New("injected error")
			marshalJSON = func(interface{}) ([]byte, error) {
				return []byte{}, injectedError
			}

			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()

			err = wal.log(operationPut, "fruit", "\"apple")
			assert.Error(t, err)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "error marshalling log entry")
		},
	)

	t.Run(
		"it returns an error if the log entry cannot be written",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := writeFile
			t.Cleanup(func() { writeFile = orig })
			injectedError := errors.New("injected error")
			writeFile = func(f *os.File, b []byte) (int, error) {
				return 0, injectedError
			}
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()

			err = wal.log(operationPut, "fruit", "apple")

			assert.Error(t, err)
			assert.ErrorContains(t, err, "error writing log entry")
		},
	)

	t.Run(
		"it returns an error if the record terminator cannot be written",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := writeFile
			t.Cleanup(func() { writeFile = orig })
			injectedError := errors.New("injected error")
			writeFile = func(f *os.File, b []byte) (int, error) {
				if len(b) == 1 && b[0] == recordTerminator[0] {
					return 0, injectedError
				}

				return f.Write(b)
			}
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()

			err = wal.log(operationPut, "fruit", "apple")

			assert.Error(t, err)
			assert.ErrorContains(t, err, "error writing log entry")
		},
	)

	t.Run(
		"it returns an error if the file cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("injected error")
			syncFile = func(f *os.File) error {
				return injectedError
			}
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()

			err = wal.log(operationPut, "fruit", "apple")

			assert.Error(t, err)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to sync write-ahead log file")
		},
	)
}

func TestWALTruncate(t *testing.T) {
	t.Run("it succeeds", func(t *testing.T) {
		tempDir := t.TempDir()
		filename := path.Join(tempDir, "wal.db")
		wal, err := newWriteAheadLog(filename)
		assert.NoError(t, err)
		defer func() {
			_ = wal.Close()
		}()

		err = wal.truncate()

		assert.NoError(t, err)
	})

	t.Run(
		"it returns an error if truncation fails",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()
			orig := truncateFile
			t.Cleanup(func() { truncateFile = orig })
			injectedError := errors.New("injected error")
			truncateFile = func(f *os.File, size int64) error {
				return injectedError
			}

			err = wal.truncate()

			assert.Error(t, err)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(
				t,
				err,
				"error truncating write-ahead log file",
			)
		},
	)

	t.Run(
		"it returns an error if the file cannot be synced",
		func(t *testing.T) {
			tempDir := t.TempDir()
			filename := path.Join(tempDir, "wal.db")
			wal, err := newWriteAheadLog(filename)
			assert.NoError(t, err)
			defer func() {
				_ = wal.Close()
			}()
			orig := syncFile
			t.Cleanup(func() { syncFile = orig })
			injectedError := errors.New("injected error")
			syncFile = func(f *os.File) error {
				return injectedError
			}

			err = wal.truncate()

			assert.Error(t, err)
			assert.ErrorIs(t, err, injectedError)
			assert.ErrorContains(t, err, "unable to sync write-ahead log file")
		},
	)
}
