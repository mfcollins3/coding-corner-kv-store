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
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type sstable []sstableEntry

func newSSTable(mt memtable) sstable {
	sst := make([]sstableEntry, mt.len())
	i := 0
	for k, v := range mt {
		sst[i] = sstableEntry{
			Key:   k,
			Value: v,
		}
		i++
	}

	sort.Slice(sst, func(a, b int) bool {
		return sst[a].Key < sst[b].Key
	})

	return sst
}

func openSSTable(filename string) (sstable, error) {
	f, err := openRead(filename)
	if err != nil {
		return nil, fmt.Errorf("unable to open sstable %q: %w", filename, err)
	}

	defer func() {
		_ = f.Close()
	}()

	var sstable []sstableEntry
	if err := json.NewDecoder(f).Decode(&sstable); err != nil {
		return nil, fmt.Errorf(
			"unable to decode sstable %q: %w",
			filename,
			err,
		)
	}

	return sstable, nil
}

func (s sstable) Save(filename string) error {
	f, err := openFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf(
			"unable to create sstable file at %s: %w",
			filename,
			err,
		)
	}

	defer func() {
		_ = f.Close()
	}()

	if err := json.NewEncoder(f).Encode(s); err != nil {
		return fmt.Errorf(
			"unable to serialize sstable data to %s: %w",
			filename,
			err,
		)
	}

	return nil
}

func (s sstable) Get(key string) (string, bool) {
	for _, entry := range s {
		if entry.Key == key {
			return entry.Value, true
		}
	}

	return "", false
}
