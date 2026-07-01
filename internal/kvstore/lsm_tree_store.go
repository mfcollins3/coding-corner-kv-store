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
	"path/filepath"
)

const maxMemtableEntries = 2000

type lsmTreeStore struct {
	io.Closer

	mt            memtable
	path          string
	manifest      *manifest
	negativeCache map[string]struct{}
	wal           *writeAheadLog
}

func newLSMTreeStore(dir string) (*lsmTreeStore, error) {
	memtable := newMemtable()

	logFilename := path.Join(dir, "wal.db")
	if err := replayWriteAheadLog(logFilename, memtable); err != nil {
		return nil, fmt.Errorf("failed to replay log: %w", err)
	}

	wal, err := newWriteAheadLog(logFilename)
	if err != nil {
		return nil, fmt.Errorf("unable to open write-ahead log: %w", err)
	}

	manifest, err := createOrLoadManifest(path.Join(dir, "MANIFEST"))
	if err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf("unable to load manifest: %w", err)
	}

	if err := clearDanglingSSTables(dir, manifest.sstables); err != nil {
		_ = wal.Close()
		return nil, fmt.Errorf(
			"unable to clear dangling sstable files: %w",
			err,
		)
	}

	return &lsmTreeStore{
		mt:            memtable,
		path:          dir,
		manifest:      manifest,
		negativeCache: make(map[string]struct{}),
		wal:           wal,
	}, nil
}

func createOrLoadManifest(filename string) (*manifest, error) {
	if _, err := statFile(filename); errors.Is(err, os.ErrNotExist) {
		return newManifest(filename)
	}

	return openManifest(filename)
}

func (s *lsmTreeStore) Close() error {
	return s.wal.Close()
}

func (s *lsmTreeStore) Get(key string) (string, error) {
	if _, ok := s.negativeCache[key]; ok {
		return "", fmt.Errorf("key %s not found: %w", key, ErrKeyNotFound)
	}

	value, ok := s.mt[key]
	if ok {
		return value, nil
	}

	sstables := s.manifest.getSSTables()

	for _, filename := range sstables {
		sstable, err := openSSTable(path.Join(s.path, filename))
		if err != nil {
			return "", fmt.Errorf("unable to load sstable: %w", err)
		}

		value, ok = sstable.Get(key)
		if ok {
			return value, nil
		}
	}

	s.negativeCache[key] = struct{}{}
	return "", fmt.Errorf("key %s not found: %w", key, ErrKeyNotFound)
}

func (s *lsmTreeStore) Set(key, value string) error {
	if err := s.wal.log(operationPut, key, value); err != nil {
		return fmt.Errorf("unable to write key %s to the log: %w", key, err)
	}

	s.mt.set(key, value)
	delete(s.negativeCache, key)
	if s.mt.len() < maxMemtableEntries {
		return nil
	}

	if err := s.flush(); err != nil {
		return fmt.Errorf("failed to flush the memtable: %w", err)
	}

	return nil
}

func (s *lsmTreeStore) flush() error {
	sst := newSSTable(s.mt)
	filename := s.manifest.nextSSTableFilename()
	if err := sst.Save(filename); err != nil {
		return fmt.Errorf("failed to save sst: %w", err)
	}

	if err := s.manifest.addSSTable(filename); err != nil {
		return fmt.Errorf("failed to add sstable to manifest: %w", err)
	}

	s.mt.clear()

	if err := s.wal.truncate(); err != nil {
		return fmt.Errorf("failed to truncate write-ahead log: %w", err)
	}

	return nil
}

func clearDanglingSSTables(dir string, ssTables []string) error {
	set := make(map[string]struct{})
	for _, ssTable := range ssTables {
		set[ssTable] = struct{}{}
	}

	matches, err := findFiles(path.Join(dir, "sst-*.json"))
	if err != nil {
		return fmt.Errorf("unable to find SST tables in %s: %w", dir, err)
	}

	removedAny := false
	for _, filename := range matches {
		if _, ok := set[filepath.Base(filename)]; ok {
			continue
		}

		if err := removeFile(filename); err != nil {
			return fmt.Errorf(
				"unable to remove dangling SST table %q: %w",
				filename,
				err,
			)
		}

		removedAny = true
	}

	if removedAny {
		directory, err := openRead(dir)
		if err != nil {
			return fmt.Errorf("unable to open directory %s: %w", dir, err)
		}

		defer func() {
			_ = directory.Close()
		}()

		if err := syncDir(directory); err != nil {
			return fmt.Errorf("unable to sync directory %s: %w", dir, err)
		}
	}

	return nil
}
