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

package api

import (
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"go.michaelfcollins3.dev/kvstore/internal/metrics"
)

func GetMetrics(histogram metrics.Histogram) http.Handler {
	validateAcceptHeader := func(w http.ResponseWriter, accept string) bool {
		if accept == "" {
			return true
		}

		types := strings.Split(accept, ",")
		for _, t := range types {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(t))
			if err != nil {
				http.Error(
					w,
					"The Accept header is malformed",
					http.StatusBadRequest,
				)
				return false
			}

			if mediaType == "*/*" {
				return true
			}

			if mediaType == "application/json" {
				charset, ok := params["charset"]
				if !ok {
					return true
				}

				if charset != "utf-8" {
					http.Error(
						w,
						"The charset must be UTF-8",
						http.StatusNotAcceptable,
					)
					return false
				}

				return true
			}
		}

		http.Error(
			w,
			"Accept header must include application/json",
			http.StatusNotAcceptable,
		)
		return false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		accept := r.Header.Get("Accept")
		if !validateAcceptHeader(w, accept) {
			return
		}

		p := histogram.Percentiles()
		response := struct {
			P50 float64 `json:"p50"`
			P95 float64 `json:"p95"`
			P99 float64 `json:"p99"`
		}{
			P50: p.P50,
			P95: p.P95,
			P99: p.P99,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})
}
