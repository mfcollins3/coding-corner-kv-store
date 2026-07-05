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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSSTableItemHeap(t *testing.T) {
	t.Run("Len", func(t *testing.T) {
		t.Run("it returns the length of the heap", func(t *testing.T) {
			h := sstableItemHeap{}
			heap.Init(&h)
			heap.Push(&h, sstableItem{})
			heap.Push(&h, sstableItem{})
			heap.Push(&h, sstableItem{})

			assert.Equal(t, 3, h.Len())
		})

		t.Run("it returns zero if the heap is empty", func(t *testing.T) {
			h := sstableItemHeap{}
			heap.Init(&h)

			assert.Equal(t, 0, h.Len())
		})
	})

	t.Run("Less", func(t *testing.T) {
		t.Run("it returns the lesser of two keys", func(t *testing.T) {
			h := sstableItemHeap{}
			heap.Init(&h)
			heap.Push(&h, sstableItem{
				key:          "fruit",
				value:        "apple",
				deleted:      false,
				sstableIndex: 1,
			})
			heap.Push(&h, sstableItem{
				key:          "car",
				value:        "ferrari",
				deleted:      false,
				sstableIndex: 2,
			})

			minEntry := heap.Pop(&h).(sstableItem)

			assert.Equal(t, minEntry.key, "car")
		})

		t.Run(
			"it returns the entry from the newest SSTable if the keys are equal",
			func(t *testing.T) {
				h := sstableItemHeap{}
				heap.Init(&h)
				heap.Push(&h, sstableItem{
					key:          "fruit",
					value:        "apple",
					deleted:      false,
					sstableIndex: 3,
				})
				heap.Push(&h, sstableItem{
					key:          "fruit",
					value:        "orange",
					deleted:      false,
					sstableIndex: 1,
				})

				minEntry := heap.Pop(&h).(sstableItem)

				assert.Equal(t, minEntry.value, "orange")
			},
		)
	})
}
