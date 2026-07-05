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
	"container/heap"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"slices"
)

const maxMemtableEntries = 2000
const maxOperations = 10_000

type lsmTreeStore struct {
	io.Closer

	mt             memtable
	path           string
	manifest       *manifest
	negativeCache  map[string]struct{}
	wal            *writeAheadLog
	operationCount int
}

func newLSMTreeStore(dir string) (*lsmTreeStore, error) {
	memtable := newMemtable()

	logFilename := path.Join(dir, "wal.db")
	err := replayWriteAheadLog(logFilename, memtable)
	if err != nil {
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

	store := &lsmTreeStore{
		mt:             memtable,
		path:           dir,
		manifest:       manifest,
		negativeCache:  make(map[string]struct{}),
		wal:            wal,
		operationCount: memtable.len(),
	}

	return store, nil
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

func (s *lsmTreeStore) Delete(key string) error {
	if err := s.wal.log(operationDelete, key, ""); err != nil {
		return fmt.Errorf("unable to write key %s to the log: %w", key, err)
	}

	s.operationCount++

	s.mt.delete(key)
	delete(s.negativeCache, key)
	if err := s.flush(); err != nil {
		return fmt.Errorf("failed to flush the memtable: %w", err)
	}

	if err := s.compact(); err != nil {
		return fmt.Errorf("failed to compact the store: %w", err)
	}

	return nil
}

func (s *lsmTreeStore) Get(key string) (string, error) {
	if _, ok := s.negativeCache[key]; ok {
		return "", fmt.Errorf("key %s not found: %w", key, ErrKeyNotFound)
	}

	value, err := s.mt.get(key)
	switch {
	case errors.Is(err, ErrKeyDeleted):
		return "", fmt.Errorf("key %s was deleted: %w", key, ErrKeyDeleted)

	case errors.Is(err, ErrKeyNotFound):
		break

	default:
		return value, err
	}

	sstables := s.manifest.getSSTables()

	for _, filename := range sstables {
		sstable, err := openSSTable(path.Join(s.path, filename))
		if err != nil {
			return "", fmt.Errorf("unable to load sstable: %w", err)
		}

		value, err = sstable.Get(key)
		switch {
		case errors.Is(err, ErrKeyDeleted):
			return "", fmt.Errorf("key %s was deleted: %w", key, ErrKeyDeleted)

		case errors.Is(err, ErrKeyNotFound):
			continue

		default:
			return value, err
		}
	}

	s.negativeCache[key] = struct{}{}
	return "", fmt.Errorf("key %s not found: %w", key, ErrKeyNotFound)
}

func (s *lsmTreeStore) Set(key, value string) error {
	if err := s.wal.log(operationPut, key, value); err != nil {
		return fmt.Errorf("unable to write key %s to the log: %w", key, err)
	}

	s.operationCount++

	s.mt.set(key, value)
	delete(s.negativeCache, key)
	if err := s.flush(); err != nil {
		return fmt.Errorf("failed to flush the memtable: %w", err)
	}

	if err := s.compact(); err != nil {
		return fmt.Errorf("failed to compact the store: %w", err)
	}

	return nil
}

func (s *lsmTreeStore) flush() error {
	if s.mt.len() < maxMemtableEntries {
		return nil
	}

	sst := newSSTable(s.mt)
	filename := s.manifest.nextSSTableFilename()
	log.Printf("Flushing memtable entries to %s\n", path.Base(filename))
	if err := sst.Save(filename); err != nil {
		return fmt.Errorf("failed to save sst: %w", err)
	}

	if err := s.manifest.addSSTable(filename); err != nil {
		return fmt.Errorf("failed to add sstable to manifest: %w", err)
	}

	s.mt.clear()

	log.Println("Truncating the write-ahead log")
	if err := s.wal.truncate(); err != nil {
		return fmt.Errorf("failed to truncate write-ahead log: %w", err)
	}

	log.Println("Flush complete")
	return nil
}

func (s *lsmTreeStore) compact() error {
	if s.operationCount < maxOperations {
		return nil
	}

	log.Println("Compacting the SSTables")

	var newSSTables []string

	sstables := make([]*sstableIterator, len(s.manifest.sstables))
	for i, filename := range s.manifest.sstables {
		sstable, err := newSSTableIterator(path.Join(s.path, filename))
		if err != nil {
			return fmt.Errorf("unable to open sstable %s: %w", filename, err)
		}

		sstables[i] = sstable
	}

	h := sstableItemHeap{}
	heap.Init(&h)
	for index, it := range sstables {
		if item, ok := it.current(); ok {
			heap.Push(&h, sstableItem{
				sstableIndex: index,
				key:          item.Key,
				value:        item.Value,
				deleted:      item.Deleted,
			})
		}
	}

	newSSTable := sstable{}
	for h.Len() > 0 {
		// Get the smallest key from the heap
		item := heap.Pop(&h).(sstableItem)
		if newItem, ok := sstables[item.sstableIndex].next(); ok {
			heap.Push(&h, sstableItem{
				sstableIndex: item.sstableIndex,
				key:          newItem.Key,
				value:        newItem.Value,
				deleted:      newItem.Deleted,
			})
		}

		// Drain other heap entries with the same key and push their next
		// key-value pairs onto the heap.
		if h.Len() > 0 {
			for item.key == h[0].key {
				x := heap.Pop(&h).(sstableItem)
				if newItem, ok := sstables[x.sstableIndex].next(); ok {
					heap.Push(&h, sstableItem{
						sstableIndex: x.sstableIndex,
						key:          newItem.Key,
						value:        newItem.Value,
						deleted:      newItem.Deleted,
					})
				}

				if h.Len() == 0 {
					break
				}
			}
		}

		// Dispose the key if it's deleted.
		if !item.deleted {
			newSSTable = append(newSSTable, sstableEntry{
				Key:     item.key,
				Value:   item.value,
				Deleted: false,
			})
		}

		// SSTables should remain a maximum number of entries. If we hit that
		// limit, then save the SSTable and start writing a new SSTable.
		if len(newSSTable) == maxMemtableEntries {
			filename := s.manifest.nextSSTableFilename()
			log.Printf("Creating new SSTable %s\n", path.Base(filename))
			if err := newSSTable.Save(filename); err != nil {
				return fmt.Errorf(
					"failed to save compacted sstable %s: %w",
					filename,
					err,
				)
			}

			newSSTables = append(newSSTables, filename)
			newSSTable = sstable{}
		}
	}

	if len(newSSTable) > 0 {
		filename := s.manifest.nextSSTableFilename()
		log.Printf("Creating new SSTable %s\n", path.Base(filename))
		if err := newSSTable.Save(filename); err != nil {
			return fmt.Errorf(
				"failed to save compacted sstable %s: %w",
				filename,
				err,
			)
		}

		newSSTables = append(newSSTables, filename)
	}

	oldSSTables := s.manifest.sstables
	slices.Reverse(newSSTables)
	filenames := make([]string, len(newSSTables))
	for i, filename := range newSSTables {
		filenames[i] = path.Base(filename)
	}

	log.Println("Updating manifest")
	if err := s.manifest.save(filenames); err != nil {
		return fmt.Errorf("unable to save updated manifest: %w", err)
	}

	for _, filename := range oldSSTables {
		if err := removeFile(path.Join(s.path, filename)); err != nil {
			return fmt.Errorf("unable to remove sstable %s: %w", filename, err)
		}
	}

	s.operationCount = 0
	log.Println("Compaction complete")
	return nil
}

func clearDanglingSSTables(dir string, ssTables []string) error {
	log.Println("Clearing dangling SSTables")

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

		log.Printf("Removing SSTable %s\n", path.Base(filename))
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

	log.Println("SSTable cleanup finished")
	return nil
}
