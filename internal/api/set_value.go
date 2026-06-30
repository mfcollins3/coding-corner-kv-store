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
	"io"
	"mime"
	"net/http"
	"strings"

	"go.michaelfcollins3.dev/kvstore/internal/kvstore"
)

func SetValue(store kvstore.Store) http.Handler {
	validateContentTypeHeader := func(
		w http.ResponseWriter,
		contentType string,
	) bool {
		if contentType == "" {
			http.Error(
				w,
				"The Content-Type header is required",
				http.StatusBadRequest,
			)
			return false
		}

		mediaType, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			http.Error(
				w,
				"The Content-Type header is invalid",
				http.StatusBadRequest)
			return false
		}

		if mediaType != "text/plain" {
			http.Error(
				w,
				"Content-Type must be text/plain",
				http.StatusUnsupportedMediaType,
			)
			return false
		}

		charset := strings.ToLower(params["charset"])
		if charset != "" && charset != "utf-8" {
			http.Error(
				w,
				"Unsupported charset; only UTF-8 is allowed",
				http.StatusUnsupportedMediaType,
			)
			return false
		}

		return true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if !validateContentTypeHeader(w, contentType) {
			return
		}

		key := r.PathValue("key")
		defer func() {
			_ = r.Body.Close()
		}()
		value, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(
				w,
				"The request body could not be read",
				http.StatusInternalServerError,
			)
			return
		}

		if err := store.Set(key, string(value)); err != nil {
			http.Error(
				w,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError,
			)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})
}
