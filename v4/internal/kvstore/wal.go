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
	"os"
	"syscall"
)

type operation string

const (
	opDelete operation = "delete"
	opPut    operation = "put"
)

type logEntry struct {
	Op    string `json:"op"`
	CRC32 uint32 `json:"crc32"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type writeAheadLog struct {
	file    *os.File
	encoder *json.Encoder
}

func openWriteAheadLog(path string) (*writeAheadLog, error) {
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND|syscall.O_DSYNC,
		0644,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	if err := syncDirectory(); err != nil {
		_ = file.Close()
		return nil, err
	}

	return &writeAheadLog{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

func (wal *writeAheadLog) Close() error {
	if err := wal.file.Close(); err != nil {
		return fmt.Errorf("error closing write-ahead log: %w", err)
	}

	return nil
}

func (wal *writeAheadLog) Log(op operation, key, value string) error {
	hash := crc32.NewIEEE()
	_, err := hash.Write([]byte(fmt.Sprintf("%s,%s,%s", op, key, value)))
	if err != nil {
		return fmt.Errorf("failed to calculate CRC32: %w", err)
	}

	logEntry := logEntry{
		Op:    string(op),
		CRC32: hash.Sum32(),
		Key:   key,
		Value: value,
	}
	if err := wal.encoder.Encode(logEntry); err != nil {
		return fmt.Errorf("failed to write log entry: %w", err)
	}

	return nil
}

func (wal *writeAheadLog) Truncate() error {
	if err := wal.file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate write-ahead log file: %w", err)
	}

	if err := wal.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync write-ahead log file: %w", err)
	}

	return nil
}

func replayLog() (map[string]value, error) {
	store := make(map[string]value)

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
		store[entry.Key] = value{
			Value:   entry.Value,
			Deleted: false,
		}
	}

	return store, nil
}
