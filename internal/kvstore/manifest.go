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
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
)

type manifest struct {
	filename      string
	nextSSTableID int
	sstables      []string
}

func newManifest(filename string) (*manifest, error) {
	file, err := openFile(
		filename,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create manifest file: %w", err)
	}

	_ = file.Close()

	return &manifest{
		filename:      filename,
		nextSSTableID: 1,
		sstables:      []string{},
	}, nil
}

func openManifest(filename string) (*manifest, error) {
	file, err := openRead(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open manifest file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	sstables, nextSSTableID, err := parseManifest(file)
	if err != nil {
		return nil, err
	}

	return &manifest{
		filename:      filename,
		nextSSTableID: nextSSTableID,
		sstables:      sstables,
	}, nil
}

func parseManifest(r io.Reader) ([]string, int, error) {
	maxID := 0
	sstables := []string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		sstables = append(sstables, line)
		idStr := strings.TrimSuffix(
			strings.TrimPrefix(line, "sst-"),
			".json",
		)
		id, err := strconv.Atoi(idStr)
		if err != nil {
			return []string{}, 0, fmt.Errorf(
				"unable to parse sst id from %q: %w",
				line,
				err,
			)
		}

		maxID = max(maxID, id)
	}

	if err := scanner.Err(); err != nil {
		return []string{}, 0, fmt.Errorf("failed to read manifest: %w", err)
	}

	slices.Reverse(sstables)
	return sstables, maxID + 1, nil
}

func (m *manifest) nextSSTableFilename() string {
	filename := fmt.Sprintf("sst-%d.json", m.nextSSTableID)
	m.nextSSTableID++
	return path.Join(path.Dir(m.filename), filename)
}

func (m *manifest) addSSTable(filename string) error {
	f, err := openFile(
		m.filename,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return fmt.Errorf("error opening manifest file: %w", err)
	}

	defer func() {
		_ = f.Close()
	}()

	_, err = fmt.Fprintln(f, path.Base(filename))
	if err != nil {
		return fmt.Errorf("unable to append sstable to manifest: %w", err)
	}

	m.sstables = append([]string{path.Base(filename)}, m.sstables...)
	return nil
}

func (m *manifest) getSSTables() []string {
	return m.sstables
}
