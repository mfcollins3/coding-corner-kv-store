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
	}, nil
}

func openManifest(filename string) (*manifest, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open manifest file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	nextSSTableID, err := getNextSSTableID(file)
	if err != nil {
		return nil, err
	}

	return &manifest{
		filename:      filename,
		nextSSTableID: nextSSTableID,
	}, nil
}

func getNextSSTableID(r io.Reader) (int, error) {
	maxID := 0
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "sst-") &&
			strings.HasSuffix(line, ".json") {
			idStr := strings.TrimSuffix(
				strings.TrimPrefix(line, "sst-"),
				".json",
			)
			id, err := strconv.Atoi(idStr)
			if err != nil {
				return 0, fmt.Errorf(
					"unable to parse sst id from %q: %w",
					line,
					err,
				)
			}

			maxID = max(maxID, id)
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("failed to read manifest: %w", err)
	}

	return maxID + 1, nil
}

func (m *manifest) nextSSTableFilename() string {
	filename := fmt.Sprintf("sst-%d.json", m.nextSSTableID)
	m.nextSSTableID++
	return path.Join(path.Dir(m.filename), filename)
}

func (m *manifest) addSSTable(filename string) error {
	f, err := os.OpenFile(
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

	return nil
}

func (m *manifest) sstables() ([]string, error) {
	file, err := openRead(m.filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open manifest file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	var sstables []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		sstables = append(sstables, line)
	}

	slices.Reverse(sstables)
	return sstables, nil
}
