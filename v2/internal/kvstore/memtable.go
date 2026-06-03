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
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const maxTableSize = 2000

type keyValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type memtable struct {
	data          map[string]string
	nextSSTableID int
	negativeCache map[string]struct{}
	ssTables      []string
}

func newMemtable() (*memtable, error) {
	mt := &memtable{
		data:          make(map[string]string),
		negativeCache: make(map[string]struct{}),
		ssTables:      make([]string, 0),
	}

	if err := initSSTables(mt); err != nil {
		return nil, err
	}

	return mt, nil
}

func (m *memtable) Set(key, value string) error {
	m.data[key] = value
	delete(m.negativeCache, key)
	if len(m.data) < maxTableSize {
		return nil
	}

	kvs := make([]keyValue, 0, len(m.data))
	for k, v := range m.data {
		kvs = append(kvs, keyValue{Key: k, Value: v})
	}

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].Key < kvs[j].Key
	})

	m.data = make(map[string]string)

	filename := fmt.Sprintf("sst-%d.json", m.nextSSTableID)
	m.nextSSTableID++
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

	manifest, err := os.OpenFile("MANIFEST", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening MANIFEST file: %w", err)
	}

	defer func() {
		_ = manifest.Close()
	}()

	if _, err := manifest.WriteString(filename + "\n"); err != nil {
		return fmt.Errorf("error writing to MANIFEST file: %w", err)
	}

	if err := manifest.Sync(); err != nil {
		return fmt.Errorf("error syncing MANIFEST file: %w", err)
	}

	m.ssTables = append(m.ssTables, filename)
	return nil
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
