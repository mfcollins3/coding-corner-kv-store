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
)

type operation string

func (op operation) String() string {
	return string(op)
}

const (
	operationPut operation = "put"
)

type logEntry struct {
	Operation string `json:"op"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type writeAheadLog struct {
	io.Closer

	file *os.File
}

func newWriteAheadLog(path string) (*writeAheadLog, error) {
	f, err := openFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating or opening write-ahead log file (%s): %w",
			path,
			err,
		)
	}

	return &writeAheadLog{
		file: f,
	}, nil
}

func replayWriteAheadLog(path string, memtable memtable) error {
	if _, err := statFile(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf(
			"error checking if file %s exists: %w",
			path,
			err,
		)
	}

	file, err := openRead(path)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", path, err)
	}

	defer func() {
		_ = file.Close()
	}()

	scanner := bufio.NewScanner(file)
	var entry logEntry
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		count++
		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			return fmt.Errorf(
				"error unmarshalling line %d in %s: %w",
				count,
				line,
				err,
			)
		}

		memtable.set(entry.Key, entry.Value)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning file (%s): %w", path, err)
	}

	return nil
}

func (wal *writeAheadLog) Close() error {
	return wal.file.Close()
}

func (wal *writeAheadLog) log(
	op operation,
	key string,
	value string,
) error {
	entry := logEntry{
		Operation: string(op),
		Key:       key,
		Value:     value,
	}
	b, err := marshalJSON(entry)
	if err != nil {
		return fmt.Errorf("error marshalling log entry: %w", err)
	}

	_, err = fmt.Fprintln(wal.file, string(b))
	if err != nil {
		return fmt.Errorf("error writing log entry: %w", err)
	}

	if err := syncFile(wal.file); err != nil {
		return fmt.Errorf("unable to sync write-ahead log file: %w", err)
	}

	return nil
}

func (wal *writeAheadLog) truncate() error {
	if err := truncateFile(wal.file, 0); err != nil {
		return fmt.Errorf("error truncating write-ahead log file: %w", err)
	}

	if err := syncFile(wal.file); err != nil {
		return fmt.Errorf("unable to sync write-ahead log file: %w", err)
	}

	return nil
}
