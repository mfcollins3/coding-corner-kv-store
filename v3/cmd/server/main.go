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

// Server implements the Key-Value Storage Engine service. Server will expose
// an HTTP API through which clients can read or write values to the store or
// query metrics.
//
// By default, the server will listen on port 8080. The TCP/IP port can be
// changed using the PORT environment variable.
//
// Then the server is running, the following HTTP operations are available:
//
//   - PUT /kv/{key}: writes the value from the body of the request into the
//     Key-Value Store using the specified key. The PUT operation will return
//     a 201 HTTP status code on success.
//   - GET /kv/{key}: reads the value of the requested key. If the key does not
//     exist in the store, the server will return a 404 HTTP status code.
//   - GET /metrics: returns the P50, P95, and P99 latency percentiles for
//     key/value operations. The returned metrics are in milliseconds.
package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.michaelfcollins3.dev/kvstore/v3/internal/kvstore"
	"go.michaelfcollins3.dev/kvstore/v3/internal/metrics"
)

const histogramSize = 10_000

var logLevel slog.LevelVar

func main() {
	configureLogging()

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	histogram := metrics.NewHistogram(histogramSize)
	store, err := kvstore.NewStore()
	if err != nil {
		slog.ErrorContext(ctx, "make_mux_failed", "error", err)
		os.Exit(1)
	}

	addr := getListenAddress()
	srv := &http.Server{
		Addr: addr,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: makeMux(store, histogram),
	}

	srvErr := make(chan error, 1)
	go func() {
		slog.InfoContext(
			ctx,
			"server_listening",
			"addr",
			addr,
		)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		slog.ErrorContext(
			ctx,
			"server_error",
			"error",
			err,
		)
		os.Exit(1)

	case <-ctx.Done():
		stop()
	}

	slog.InfoContext(ctx, "server_shutdown")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.ErrorContext(
			ctx,
			"server_shutdown_error",
			"error",
			err,
		)
		os.Exit(1)
	}
}

func configureLogging() {
	if len(os.Args) > 1 && os.Args[1] == "--debug" {
		logLevel.Set(slog.LevelDebug)
	} else {
		logLevel.Set(slog.LevelInfo)
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: &logLevel,
	})
	slog.SetDefault(slog.New(handler))
}

func makeMux(
	store kvstore.Store,
	histogram metrics.Histogram,
) *http.ServeMux {
	mux := http.NewServeMux()
	latency := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			h.ServeHTTP(w, r)
			histogram.Record(time.Since(start))
		})
	}
	mux.Handle("GET /metrics", getMetrics(histogram))
	mux.Handle("PUT /kv/{key}", latency(putValue(store)))
	mux.Handle("GET /kv/{key}", latency(getValue(store)))

	return mux
}

func getListenAddress() string {
	addr := ":" + os.Getenv("PORT")
	if addr == ":" {
		addr = ":8080"
	}

	return addr
}

func getMetrics(histogram metrics.Histogram) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.ErrorContext(
				r.Context(),
				"encode_metrics_failed",
				"error",
				err,
			)
		}
	})
}

func putValue(store kvstore.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		defer func() {
			_ = r.Body.Close()
		}()
		value, err := io.ReadAll(r.Body)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"read_request_body_failed",
				"error",
				err,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = store.Set(key, string(value))
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"store_set_failed",
				"error",
				err,
			)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusCreated)
		_, err = w.Write(value)
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"write_put_response_body_failed",
				"error",
				err,
			)
		}
	})
}

func getValue(store kvstore.Store) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.PathValue("key")
		if key == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		value, ok, err := store.Get(key)
		if err != nil {
			slog.ErrorContext(r.Context(), "get_value_failed", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(value))
		if err != nil {
			slog.ErrorContext(
				r.Context(),
				"write_get_response_body_failed",
				"error",
				err,
			)
		}
	})
}
