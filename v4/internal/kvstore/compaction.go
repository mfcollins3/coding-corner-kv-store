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
	"container/heap"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

type mergeItem struct {
	kv           keyValue
	ssTableIndex int
}

type mergeHeap []mergeItem

func (h *mergeHeap) Len() int {
	return len(*h)
}

func (h *mergeHeap) Less(i, j int) bool {
	if (*h)[i].kv.Key != (*h)[j].kv.Key {
		return (*h)[i].kv.Key < (*h)[j].kv.Key
	}

	return (*h)[i].ssTableIndex > (*h)[j].ssTableIndex
}

func (h *mergeHeap) Swap(i, j int) {
	(*h)[i], (*h)[j] = (*h)[j], (*h)[i]
}

func (h *mergeHeap) Push(x any) {
	*h = append(*h, x.(mergeItem))
}

func (h *mergeHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type sstIterator struct {
	file    *os.File
	decoder *json.Decoder
	peeked  *keyValue
}

func newSSTIterator(path string) (*sstIterator, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening sst file %s: %w", path, err)
	}

	it := &sstIterator{
		file:    f,
		decoder: json.NewDecoder(f),
	}
	if err := it.advance(); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("error reading sst file %s: %w", path, err)
	}

	return it, nil
}

func (it *sstIterator) current() (keyValue, bool) {
	if it.peeked == nil {
		return keyValue{}, false
	}

	return *it.peeked, true
}

func (it *sstIterator) advance() error {
	var kv keyValue
	if err := it.decoder.Decode(&kv); err != nil {
		if !errors.Is(err, io.EOF) {
			return fmt.Errorf(
				"error decoding sst file %s: %w",
				it.file.Name(),
				err,
			)
		}

		it.peeked = nil
		return nil
	}

	it.peeked = &kv
	return nil
}

func (it *sstIterator) close() error {
	return it.file.Close()
}

func compact(mt *memtable) error {
	if len(mt.ssTables) == 0 {
		return nil
	}

	iters := make([]*sstIterator, len(mt.ssTables))
	for i, filename := range mt.ssTables {
		it, err := newSSTIterator(filename)
		if err != nil {
			return err
		}

		iters[i] = it
	}

	defer func() {
		for _, it := range iters {
			_ = it.close()
		}
	}()

	h := &mergeHeap{}
	heap.Init(h)
	for i, it := range iters {
		if kv, ok := it.current(); ok {
			heap.Push(h, mergeItem{kv: kv, ssTableIndex: i})
		}
	}

	var (
		emitted     []keyValue
		newSSTables []string
		nextNewID   = mt.nextSSTableID
	)

	flushEmitted := func() error {
		if len(emitted) == 0 {
			return nil
		}

		filename := fmt.Sprintf("sst-%d.json", nextNewID)
		nextNewID++
		if err := writeSSTable(filename, emitted); err != nil {
			return fmt.Errorf("compaction: write SSTable: %w", err)
		}

		newSSTables = append(newSSTables, filename)
		emitted = emitted[:0]
		return nil
	}

	for h.Len() > 0 {
		winner := heap.Pop(h).(mergeItem)
		winnerKey := winner.kv.Key
		if err := iters[winner.ssTableIndex].advance(); err != nil {
			return fmt.Errorf("compaction: advance: %w", err)
		}

		if kv, ok := iters[winner.ssTableIndex].current(); ok {
			heap.Push(h, mergeItem{kv: kv, ssTableIndex: winner.ssTableIndex})
		}

		for h.Len() > 0 && (*h)[0].kv.Key == winnerKey {
			loser := heap.Pop(h).(mergeItem)
			if err := iters[loser.ssTableIndex].advance(); err != nil {
				return fmt.Errorf("compaction: advance: %w", err)
			}

			if kv, ok := iters[loser.ssTableIndex].current(); ok {
				heap.Push(
					h,
					mergeItem{kv: kv, ssTableIndex: loser.ssTableIndex},
				)
			}
		}

		if winner.kv.Deleted {
			continue
		}

		emitted = append(emitted, winner.kv)
		if len(emitted) >= maxTableSize {
			if err := flushEmitted(); err != nil {
				return err
			}
		}
	}

	if err := flushEmitted(); err != nil {
		return err
	}

	if err := syncDirectory(); err != nil {
		return fmt.Errorf(
			"compaction: sync directory after writing new SSTables: %w",
			err,
		)
	}

	oldSSTables := mt.ssTables
	mt.ssTables = newSSTables
	mt.nextSSTableID = nextNewID
	mt.negativeCache = make(map[string]struct{})

	if err := writeManifest(mt); err != nil {
		mt.ssTables = oldSSTables
		mt.nextSSTableID = nextNewID - len(newSSTables)
		return fmt.Errorf("compaction: write manifest: %w", err)
	}

	for _, filename := range oldSSTables {
		_ = os.Remove(filename)
	}

	if err := syncDirectory(); err != nil {
		return fmt.Errorf(
			"compaction: sync directory after removing old SSTables: %w",
			err,
		)
	}

	return nil
}
