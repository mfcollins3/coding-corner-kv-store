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

type sstableItem struct {
	sstableIndex int
	key          string
	value        string
	deleted      bool
}

type sstableItemHeap []sstableItem

func (h sstableItemHeap) Len() int {
	return len(h)
}

func (h sstableItemHeap) Less(i, j int) bool {
	if h[i].key == h[j].key {
		// SSTable array is in reverse order
		return h[i].sstableIndex < h[j].sstableIndex
	}

	return h[i].key < h[j].key
}

func (h sstableItemHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *sstableItemHeap) Push(x any) {
	*h = append(*h, x.(sstableItem))
}

func (h *sstableItemHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[0 : n-1]
	return item
}
