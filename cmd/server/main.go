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

package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"

	"go.michaelfcollins3.dev/kvstore/internal/api"
	"go.michaelfcollins3.dev/kvstore/internal/kvstore"
	"go.michaelfcollins3.dev/kvstore/internal/metrics"
)

const histogramSize = 10_000

func main() {
	var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")
	var memProfile = flag.String(
		"memprofile",
		"",
		"write memory profile to file",
	)
	flag.Parse()

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}

		defer func() {
			_ = f.Close()
		}()

		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}

		defer pprof.StopCPUProfile()
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	histogram := metrics.NewHistogram(histogramSize)
	latency := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			h.ServeHTTP(w, r)
			histogram.Record(time.Since(start))
		})
	}

	mu := sync.Mutex{}
	singleThreaded := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			h.ServeHTTP(w, r)
			mu.Unlock()
		})
	}

	store, err := kvstore.NewStore(".")
	if err != nil {
		log.Fatalf("unable to create the store: %v", err)
	}
	defer func() {
		_ = store.Close()
	}()

	kvMux := http.NewServeMux()
	kvMux.Handle("GET /{key}", api.GetValue(store))
	kvMux.Handle("PUT /{key}", api.SetValue(store))

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", api.GetMetrics(histogram))
	mux.Handle("/kv/", http.StripPrefix("/kv", latency(singleThreaded(kvMux))))

	srv := &http.Server{
		Addr: ":8080",
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
		Handler: mux,
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		log.Fatalf("server error: %v", err)
	case <-ctx.Done():
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	if *memProfile != "" {
		f, err := os.Create(*memProfile)
		if err != nil {
			log.Fatalf("could not create memory profile: %v", err)
		}

		defer func() {
			_ = f.Close()
		}()

		runtime.GC()
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatalf("could not write memory profile: %v", err)
		}
	}
}
