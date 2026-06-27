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

package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

type Histogram interface {
	Record(d time.Duration)
	Percentiles() Percentiles
}

type histogram struct {
	mu       sync.RWMutex
	ringSize int
	samples  []float64
	head     int
	count    int
}

func NewHistogram(size int) Histogram {
	return &histogram{
		ringSize: size,
		samples:  make([]float64, size),
	}
}

func (h *histogram) Record(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples[h.head] = float64(d.Milliseconds())
	h.head = (h.head + 1) % h.ringSize
	if h.count < h.ringSize {
		h.count++
	}
}

func (h *histogram) Percentiles() Percentiles {
	h.mu.RLock()
	n := h.count
	snapshot := make([]float64, n)
	copy(snapshot, h.samples[:n])
	h.mu.RUnlock()

	if n == 0 {
		return Percentiles{}
	}

	sort.Float64s(snapshot)

	rank := func(p float64) float64 {
		i := int(math.Ceil(p/100*float64(n))) - 1
		return snapshot[i]
	}

	return Percentiles{
		P50: rank(50),
		P95: rank(95),
		P99: rank(99),
	}
}
