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
	"bufio"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const maxTableSize = 2000

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type logEntry struct {
	Op    string `json:"op"`
	CRC32 uint32 `json:"crc32"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type memtable struct {
	data          map[string]string
	nextSSTableID int
	negativeCache map[string]struct{}
	ssTables      []string
	wal           *os.File
	encoder       *json.Encoder
}

func newMemtable() (*memtable, error) {
	data, err := replayLog()
	if nil != err {
		return nil, fmt.Errorf("failed to replay log: %w", err)
	}

	wal, err := os.OpenFile(
		"wal.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND|syscall.O_DSYNC,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	if err := syncDirectory(); err != nil {
		return nil, err
	}

	encoder := json.NewEncoder(wal)

	mt := &memtable{
		data:          data,
		negativeCache: make(map[string]struct{}),
		ssTables:      make([]string, 0),
		wal:           wal,
		encoder:       encoder,
	}

	if err := initSSTables(mt); err != nil {
		return nil, err
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
	hash := crc32.NewIEEE()
	_, err := hash.Write([]byte(fmt.Sprintf("put,%s,%s", key, value)))
	if err != nil {
		return fmt.Errorf("failed to calculate CRC32: %w", err)
	}

	logEntry := logEntry{
		Op:    "put",
		CRC32: hash.Sum32(),
		Key:   key,
		Value: value,
	}
	if err := m.encoder.Encode(logEntry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	m.data[key] = value
	delete(m.negativeCache, key)
	if len(m.data) < maxTableSize {
		return nil
	}

	return m.flushTable()
}

func (m *memtable) Get(key string) (string, bool, error) {
	if _, ok := m.negativeCache[key]; ok {
		slog.Debug("negative_cache_hit", "key", key)
		return "", false, nil
	}

	if value, ok := m.data[key]; ok {
		return value, true, nil
	}

	if len(m.ssTables) == 0 {
		return "", false, nil
	}

	for i := len(m.ssTables) - 1; i >= 0; i-- {
		value, ok, err := searchSSTable(m.ssTables[i], key)
		if err != nil {
			return "", false, err
		}

		if ok {
			return value, true, nil
		}
	}

	slog.Debug("negative_cache_insert", "key", "key")
	m.negativeCache[key] = struct{}{}
	return "", false, nil
}

func clearDanglingSSTables(ssTables []string) error {
	set := make(map[string]struct{})
	for _, ssTable := range ssTables {
		set[filepath.Base(ssTable)] = struct{}{}
	}

	matches, err := filepath.Glob("sst-*.json")
	if err != nil {
		return fmt.Errorf("unable to find SST tables in current directory: %w", err)
	}

	removedAny := false
	for _, filename := range matches {
		if _, ok := set[filepath.Base(filename)]; ok {
			continue
		}

		if err := os.Remove(filename); err != nil {
			return fmt.Errorf(
				"unable to remove dangling SST table %q: %w",
				filename,
				err,
			)
		}
		removedAny = true
	}

	if removedAny {
		if err := syncDirectory(); err != nil {
			return err
		}
	}

	return nil
}

func initSSTables(mt *memtable) error {
	if _, err := os.Stat("MANIFEST"); os.IsNotExist(err) {
		if err = createManifest(); err != nil {
			return err
		}

		mt.nextSSTableID = 1
	} else if err == nil {
		maxID, err := loadSSTables(mt)
		if err != nil {
			return err
		}

		mt.nextSSTableID = maxID + 1
	}

	return nil
}

func createManifest() error {
	file, err := os.Create("MANIFEST")
	if err != nil {
		return fmt.Errorf("unable to create MANIFEST: %w", err)
	}

	_ = file.Close()
	return nil
}

func loadSSTables(mt *memtable) (int, error) {
	file, err := os.Open("MANIFEST")
	if err != nil {
		return 0, fmt.Errorf("unable to open MANIFEST: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	maxID := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		base := filepath.Base(line)
		if strings.HasPrefix(base, "sst-") && strings.HasSuffix(base, ".json") {
			mt.ssTables = append(mt.ssTables, line)

			idStr := strings.TrimSuffix(strings.TrimPrefix(base, "sst-"), ".json")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return 0, fmt.Errorf(
					"unable to parse sst-id from %q: %w",
					line,
					err,
				)
			}
			if id > maxID {
				maxID = id
			}
		}
	}

	return maxID, nil
}

func searchSSTable(ssTableFilename, key string) (string, bool, error) {
	file, err := os.Open(ssTableFilename)
	if err != nil {
		return "", false, fmt.Errorf(
			"error opening SST table %s: %w",
			ssTableFilename,
			err,
		)
	}

	defer func() {
		_ = file.Close()
	}()

	var table []keyValue
	if err = json.NewDecoder(file).Decode(&table); err != nil {
		return "", false, fmt.Errorf(
			"error reading SST table %s: %w",
			ssTableFilename,
			err,
		)
	}

	for _, kv := range table {
		if key == kv.Key {
			return kv.Value, true, nil
		}
	}

	return "", false, nil
}

func (m *memtable) flushTable() error {
	kvs := makeSSTable(m.data)
	m.data = make(map[string]string)

	filename := fmt.Sprintf("sst-%d.json", m.nextSSTableID)
	m.nextSSTableID++
	if err := writeSSTable(filename, kvs); err != nil {
		return fmt.Errorf("error writing SSTable: %w", err)
	}

	m.ssTables = append(m.ssTables, filename)

	if err := m.writeManifest(); err != nil {
		return fmt.Errorf("error writing manifest: %w", err)
	}

	if err := m.wal.Truncate(0); err != nil {
		return fmt.Errorf("error truncating WAL file: %w", err)
	}

	if err := m.wal.Sync(); err != nil {
		return fmt.Errorf("error syncing truncated WAL file: %w", err)
	}

	return nil
}

func makeSSTable(data map[string]string) []keyValue {
	kvs := make([]keyValue, 0, len(data))
	for k, v := range data {
		kvs = append(kvs, keyValue{Key: k, Value: v})
	}

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Key < kvs[j].Key
	})

	return kvs
}

func writeSSTable(filename string, kvs []keyValue) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating sst file %s: %w", filename, err)
	}

	defer func() {
		_ = file.Close()
	}()

	if err = json.NewEncoder(file).Encode(kvs); err != nil {
		return fmt.Errorf("error writing sst file %s: %w", filename, err)
	}

	if err = file.Sync(); err != nil {
		return fmt.Errorf("error syncing sst file %s: %w", filename, err)
	}

	return nil
}

func (m *memtable) writeManifest() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current working directory: %w", err)
	}

	manifest, err := os.OpenFile(
		"MANIFEST.tmp",
		os.O_TRUNC|os.O_CREATE|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("error opening MANIFEST file: %w", err)
	}

	defer func() {
		_ = manifest.Close()
	}()

	for _, filename := range m.ssTables {
		if _, err := manifest.WriteString(filename + "\n"); err != nil {
			return fmt.Errorf("error writing manifest file %s: %w", filename, err)
		}
	}

	if err := manifest.Sync(); err != nil {
		return fmt.Errorf("error syncing MANIFEST file: %w", err)
	}

	if err := os.Rename("MANIFEST.tmp", "MANIFEST"); err != nil {
		return fmt.Errorf("error renaming MANIFEST file: %w", err)
	}

	if err := syncDirectory(); err != nil {
		return fmt.Errorf("error syncing directory %s: %w", wd, err)
	}

	return nil
}

func replayLog() (map[string]string, error) {
	store := make(map[string]string)

	file, err := os.Open("wal.log")
	if err != nil {
		if os.IsNotExist(err) {
			return store, nil
		}

		return store, fmt.Errorf(
			"failed to open WAL file for playback: %w",
			err,
		)
	}

	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)
	hash := crc32.NewIEEE()
	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			return store, fmt.Errorf("failed to read log entry: %w", err)
		}

		var entry logEntry
		if err := json.Unmarshal(data, &entry); err != nil {
			return store, fmt.Errorf("failed to unmarshal log entry: %w", err)
		}

		hash.Reset()
		_, err = hash.Write([]byte(fmt.Sprintf(
			"%s,%s,%s",
			entry.Op,
			entry.Key,
			entry.Value,
		)))
		if err != nil {
			return store, fmt.Errorf("CRc32 check failed: %w", err)
		}

		hash := hash.Sum32()
		if entry.CRC32 != hash {
			return store, fmt.Errorf(
				"CRC32 check failed: expected %d, got %d",
				hash,
				entry.CRC32,
			)
		}
		store[entry.Key] = entry.Value
	}

	return store, nil
}

func syncDirectory() error {
	dir, err := os.Open(".")
	if err != nil {
		return fmt.Errorf("failed to open WAL directory: %w", err)
	}

	defer func() {
		_ = dir.Close()
	}()

	if err := dir.Sync(); nil != err {
		return fmt.Errorf("failed to sync WAL directory: %w", err)
	}

	return nil
}
