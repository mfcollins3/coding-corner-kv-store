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
	"fmt"
	"log/slog"
)

const maxTableSize = 2000
const maxOperations = 10_000

type value struct {
	Value   string
	Deleted bool
}

type memtable struct {
	data           map[string]value
	nextSSTableID  int
	negativeCache  map[string]struct{}
	ssTables       []string
	wal            *writeAheadLog
	operationCount int
}

func newMemtable() (*memtable, error) {
	data, err := replayLog()
	if nil != err {
		return nil, fmt.Errorf("failed to replay log: %w", err)
	}

	ssTables, nextSSTableID, err := getSSTableInfo()
	if err != nil {
		return nil, err
	}

	wal, err := openWriteAheadLog("wal.json")
	mt := &memtable{
		data:          data,
		negativeCache: make(map[string]struct{}),
		ssTables:      ssTables,
		nextSSTableID: nextSSTableID,
		wal:           wal,
	}

	if err := clearDanglingSSTables(mt.ssTables); err != nil {
		return nil, fmt.Errorf("failed to clear dangling SST tables: %w", err)
	}

	return mt, nil
}

func (m *memtable) Close() error {
	return m.wal.Close()
}

func (m *memtable) Set(key, value string) error {
	if err := m.updateMemtable(opPut, key, value, false); err != nil {
		return fmt.Errorf("failed to update memtable: %w", err)
	}

	delete(m.negativeCache, key)
	if len(m.data) < maxTableSize {
		return nil
	}

	if err := flushTable(m); err != nil {
		return fmt.Errorf("failed to flush table: %w", err)
	}

	return m.compact()
}

func (m *memtable) Get(key string) (string, bool, error) {
	if _, ok := m.negativeCache[key]; ok {
		slog.Debug("negative_cache_hit", "key", key)
		return "", false, nil
	}

	if value, ok := m.data[key]; ok {
		if value.Deleted {
			return "", false, nil
		}

		return value.Value, true, nil
	}

	if len(m.ssTables) == 0 {
		return "", false, nil
	}

	for i := len(m.ssTables) - 1; i >= 0; i-- {
		value, ok, deleted, err := searchSSTable(m.ssTables[i], key)
		if err != nil {
			return "", false, err
		}

		if ok {
			if deleted {
				return "", false, nil
			}

			return value, true, nil
		}
	}

	slog.Debug("negative_cache_insert", "key", "key")
	m.negativeCache[key] = struct{}{}
	return "", false, nil
}

func (m *memtable) Delete(key string) error {
	if err := m.updateMemtable(opDelete, key, "", true); err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}

	return m.compact()
}

func (m *memtable) updateMemtable(
	op operation,
	key, v string,
	deleted bool,
) error {
	if err := m.wal.Log(op, key, v); err != nil {
		return fmt.Errorf(
			"failed to write operation to write-ahead log: %w",
			err,
		)
	}

	m.data[key] = value{
		Value:   v,
		Deleted: deleted,
	}

	m.operationCount++

	return nil
}

func (m *memtable) compact() error {
	if m.operationCount < maxOperations {
		return nil
	}

	if err := compact(m); err != nil {
		return fmt.Errorf("failed to compact SSTables: %w", err)
	}

	m.operationCount = 0

	return nil
}
