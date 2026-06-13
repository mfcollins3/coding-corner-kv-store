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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type keyValue struct {
	Key     string `json:"key"`
	Value   string `json:"value,omitempty"`
	Deleted bool   `json:"deleted,omitzero"`
}

func createManifest() error {
	file, err := os.Create("MANIFEST")
	if err != nil {
		return fmt.Errorf("unable to create MANIFEST: %w", err)
	}

	_ = file.Close()
	return nil
}

func getSSTableInfo() ([]string, int, error) {
	if _, err := os.Stat("MANIFEST"); os.IsNotExist(err) {
		if err = createManifest(); err != nil {
			return []string{}, 1, err
		}

		return []string{}, 1, nil
	} else if err != nil {
		return []string{}, 1, fmt.Errorf("unable to access MANIFEST: %w", err)
	}

	ssTables, maxID, err := loadSSTables()
	if err != nil {
		return ssTables, maxID, err
	}

	return ssTables, maxID + 1, nil
}

func loadSSTables() ([]string, int, error) {
	var ssTables []string

	file, err := os.Open("MANIFEST")
	if err != nil {
		return ssTables, 0, fmt.Errorf("unable to open MANIFEST: %w", err)
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
			ssTables = append(ssTables, line)

			idStr := strings.TrimSuffix(strings.TrimPrefix(base, "sst-"), ".json")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return ssTables, 0, fmt.Errorf(
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

	return ssTables, maxID, nil
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

func searchSSTable(ssTableFilename, key string) (string, bool, bool, error) {
	file, err := os.Open(ssTableFilename)
	if err != nil {
		return "", false, false, fmt.Errorf(
			"error opening SST table %s: %w",
			ssTableFilename,
			err,
		)
	}

	defer func() {
		_ = file.Close()
	}()

	reader := bufio.NewReader(file)
	line := 1
	for {
		data, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return "", false, false, fmt.Errorf(
				"error reading SST table %s: %w",
				ssTableFilename,
				err,
			)
		}

		var kv keyValue
		if err := json.Unmarshal(data, &kv); err != nil {
			return "", false, false, fmt.Errorf(
				"error parsing SST table %s, line %d: %w",
				ssTableFilename,
				line,
				err,
			)
		}

		line++

		if key == kv.Key {
			if kv.Deleted {
				return "", true, true, nil
			}

			return kv.Value, true, false, nil
		}
	}

	return "", false, false, nil
}

func makeSSTable(data map[string]value) []keyValue {
	kvs := make([]keyValue, 0, len(data))
	for k, v := range data {
		kvs = append(
			kvs,
			keyValue{Key: k, Value: v.Value, Deleted: v.Deleted},
		)
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

	encoder := json.NewEncoder(file)
	for _, kv := range kvs {
		if err := encoder.Encode(kv); err != nil {
			return fmt.Errorf("error writing sst file %s: %w", filename, err)
		}
	}

	if err = file.Sync(); err != nil {
		return fmt.Errorf("error syncing sst file %s: %w", filename, err)
	}

	return nil
}

func flushTable(m *memtable) error {
	kvs := makeSSTable(m.data)
	m.data = make(map[string]value)

	filename := fmt.Sprintf("sst-%d.json", m.nextSSTableID)
	m.nextSSTableID++
	if err := writeSSTable(filename, kvs); err != nil {
		return fmt.Errorf("error writing SSTable: %w", err)
	}

	m.ssTables = append(m.ssTables, filename)

	if err := writeManifest(m); err != nil {
		return fmt.Errorf("error writing manifest: %w", err)
	}

	if err := m.wal.Truncate(); err != nil {
		return fmt.Errorf("error truncating WAL file: %w", err)
	}

	return nil
}

func writeManifest(m *memtable) error {
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
