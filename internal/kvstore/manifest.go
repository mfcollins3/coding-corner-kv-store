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
	sstables := append([]string{path.Base(filename)}, m.sstables...)
	return m.save(sstables)
}

func (m *manifest) getSSTables() []string {
	return m.sstables
}

func (m *manifest) save(sstables []string) error {
	tempFilename := fmt.Sprintf(
		"%s.tmp",
		strings.TrimSuffix(m.filename, path.Ext(m.filename)),
	)

	writeTemporaryManifest := func() error {
		f, err := openFile(
			tempFilename,
			os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return fmt.Errorf("unable to create manifest file: %w", err)
		}

		defer func() {
			_ = f.Close()
		}()

		for i := len(sstables) - 1; i >= 0; i-- {
			_, err = fmt.Fprintln(f, sstables[i])
			if err != nil {
				return fmt.Errorf("unable to write updated manifest: %w", err)
			}
		}

		if err := syncFile(f); err != nil {
			return fmt.Errorf("unable to sync new manifest: %w", err)
		}

		return nil
	}
	replaceManifest := func() error {
		if err := renameFile(tempFilename, m.filename); err != nil {
			return fmt.Errorf("unable to replace manifest file: %w", err)
		}

		manifestFile, err := openRead(m.filename)
		if err != nil {
			return fmt.Errorf("unable to open manifest file: %w", err)
		}

		defer func() {
			_ = manifestFile.Close()
		}()

		if err := syncFile(manifestFile); err != nil {
			return fmt.Errorf("unable to sync manifest file: %w", err)
		}

		return nil
	}
	syncDirectory := func() error {
		dir, err := openRead(path.Dir(m.filename))
		if err != nil {
			return fmt.Errorf("unable to open directory: %w", err)
		}

		defer func() {
			_ = dir.Close()
		}()

		if err := syncDir(dir); err != nil {
			return fmt.Errorf("unable to sync directory: %w", err)
		}

		return nil
	}

	if err := writeTemporaryManifest(); err != nil {
		return err
	}

	if err := replaceManifest(); err != nil {
		return err
	}

	if err := syncDirectory(); err != nil {
		return err
	}

	m.sstables = sstables
	return nil
}
