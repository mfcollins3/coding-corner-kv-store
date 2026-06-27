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

package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.michaelfcollins3.dev/kvstore/internal/api"
	"go.michaelfcollins3.dev/kvstore/internal/metrics"
)

func TestGetMetricsSucceeds(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	histogram.Record(10 * time.Millisecond)
	histogram.Record(20 * time.Millisecond)
	histogram.Record(30 * time.Millisecond)
	histogram.Record(40 * time.Millisecond)

	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/json; charset=utf-8")
	handler.ServeHTTP(recorder, request)

	t.Run("it returns a 200 status code", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("it returns JSON content", func(t *testing.T) {
		assert.Equal(
			t,
			"application/json",
			recorder.Header().Get("Content-Type"),
		)
	})

	t.Run("the JSON content is valid", func(t *testing.T) {
		var percentiles struct {
			P50 float64 `json:"p50"`
			P95 float64 `json:"p95"`
			P99 float64 `json:"p99"`
		}
		err := json.Unmarshal(recorder.Body.Bytes(), &percentiles)
		assert.NoError(t, err)
	})
}

func TestGetMetricsSucceedsIfAcceptHeaderIsMissing(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetMetricsSucceedsIfClientAcceptsAnyContentType(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "*/*")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetMetricsFailsIfAcceptHeaderIsUnacceptable(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotAcceptable, recorder.Code)
}

func TestGetMetricsFailsIfCharsetIsNotSupported(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/json; charset=iso-8859-1")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotAcceptable, recorder.Code)
}

func TestGetMetricsFailsIfAcceptHeaderIsUnparseable(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/json; charset=\"utf-8")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetMetricsFailsIfNoAcceptContentTypesAreAcceptable(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "text/plain, application/xml")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotAcceptable, recorder.Code)
}

func TestGetMetricsFailsIfCharsetIsNotSpecified(t *testing.T) {
	histogram := metrics.NewHistogram(10)
	handler := api.GetMetrics(histogram)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	request.Header.Set("Accept", "application/json")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}
