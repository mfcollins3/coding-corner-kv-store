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

// Package metrics implements metric collection and reporting for the key-value
// storage engine. Specifically, package metrics captures latency metrics for
// using the APIs to read and write to the key-value store. Package metrics
// then allows reporting of the p50, p95, and p99 latency percentiles for the
// APIs.
package metrics

import (
	"math"
	"sort"
	"sync"
	"time"
)

// PercentileResult captures the p50, p95, and p99 latency percentiles for
// the APIs and is used to return those metrics to callers.
//
// The measurements in PercentileResult are specified in milliseconds.
type PercentileResult struct {
	// The p50 latency percentile for the APIs. This value indicates that 50%
	// of the API requests were faster than the reported measurement, in
	// milliseconds.
	P50 float64

	// The p95 latency percentile for the APIs. This value indicates that 95%
	// of the API requests were faster than the reported measurement, in
	// milliseconds.
	P95 float64

	// The p99 latency percentile for the APIs. This value indicates that 99%
	// of the API requests were faster than the reported measurement, in
	// milliseconds.
	P99 float64
}

// Histogram is an interface for the latency collection and reporting engine.
// Histogram allows the application to record the latency of API requests and
// then to take a snapshot and return performance metrics of the APIs.
type Histogram interface {
	// Record records a latency measurement for a single API request.
	Record(d time.Duration)

	// Percentiles takes a snapshot of the current latency measurements and
	// calculates the p50, p95, and p99 latency percentiles for analysis.
	Percentiles() PercentileResult
}

type histogram struct {
	mu       sync.RWMutex
	ringSize int
	samples  []float64
	head     int
	count    int
}

// NewHistogram creates a Histogram implementation and returns a reference
// to it. The caller can then use the Histogram implementation to capture
// latency measurements and to report the p50, p95, and p99 latency
// percentiles for API calls.
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

func (h *histogram) Percentiles() PercentileResult {
	h.mu.RLock()
	n := h.count
	snapshot := make([]float64, n)
	copy(snapshot, h.samples[:n])
	h.mu.RUnlock()

	if n == 0 {
		return PercentileResult{}
	}

	sort.Float64s(snapshot)

	rank := func(p float64) float64 {
		i := int(math.Ceil(p/100*float64(n))) - 1
		if i < 0 {
			i = 0
		}

		return snapshot[i]
	}

	return PercentileResult{
		P50: rank(50),
		P95: rank(95),
		P99: rank(99),
	}
}
