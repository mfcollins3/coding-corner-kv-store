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
	"errors"
	"mime"
	"net/http"
	"strings"

	"go.michaelfcollins3.dev/kvstore/internal/kvstore"
)

func GetValue(store kvstore.Store) http.Handler {
	validateAcceptHeader := func(w http.ResponseWriter, header string) bool {
		if header == "" {
			return true
		}

		acceptLower := strings.ToLower(header)
		parts := strings.Split(acceptLower, ",")
		for _, part := range parts {
			contentType := strings.TrimSpace(part)
			mediaType, params, err := mime.ParseMediaType(contentType)
			if err != nil {
				http.Error(
					w,
					"The Content-Type header is invalid",
					http.StatusBadRequest,
				)
				return false
			}

			if mediaType == "*/*" {
				return true
			}

			if mediaType != "text/plain" {
				http.Error(
					w,
					"Accept header must include text/plain",
					http.StatusNotAcceptable,
				)
				return false
			}

			charset := strings.ToLower(params["charset"])
			if charset != "" && charset != "utf-8" {
				http.Error(
					w,
					"Unsupported charset; only UTF-8 is allowed",
					http.StatusNotAcceptable,
				)
				return false
			}
		}

		return true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !validateAcceptHeader(w, r.Header.Get("Accept")) {
			return
		}

		key := r.PathValue("key")
		value, err := store.Get(key)
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(value))
	})
}
