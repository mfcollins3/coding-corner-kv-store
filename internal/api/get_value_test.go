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
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.michaelfcollins3.dev/kvstore/internal/api"
	"go.michaelfcollins3.dev/kvstore/internal/kvstore"
)

func TestGetValueSucceeds(t *testing.T) {
	store := new(mockStore)
	store.On("Get", "hello").Return("world", nil)

	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "text/plain; charset=utf-8")
	handler.ServeHTTP(recorder, request)

	t.Run("it returns the correct value", func(t *testing.T) {
		assert.Equal(
			t,
			"world",
			recorder.Body.String(),
			"it returns the correct value",
		)
	})

	t.Run("it returns a 200 status code", func(t *testing.T) {
		assert.Equal(
			t,
			http.StatusOK,
			recorder.Code,
			"it returns a 200 status code",
		)
	})

	t.Run("the content type is text/plain", func(t *testing.T) {
		contentType := recorder.Header().Get("Content-Type")
		mimeType, params, err := mime.ParseMediaType(contentType)
		assert.NoError(t, err)
		assert.Equal(t, mimeType, "text/plain")
		assert.Equal(t, params["charset"], "utf-8")
	})
}

func TestGetValueSucceedsIfNoAcceptHeaderIsPresent(t *testing.T) {
	store := new(mockStore)
	store.On("Get", "hello").Return("world", nil)

	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetValueFailsIfAcceptHeaderIsNotSupported(t *testing.T) {
	store := new(mockStore)
	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "application/json")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotAcceptable, recorder.Code)
}

func TestGetValueSucceedsIfNoCharsetIsSet(t *testing.T) {
	store := new(mockStore)
	store.On("Get", "hello").Return("world", nil)

	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestGetValueFailsIfAcceptIsNotParseable(t *testing.T) {
	store := new(mockStore)
	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "text/plain; charset=\"utf-8")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestGetValueFailsIfCharsetIsNotUTF8(t *testing.T) {
	store := new(mockStore)
	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "text/plain; charset=iso-8859-1")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotAcceptable, recorder.Code)
}

func TestGetValueFailsIfKeyIsNotFound(t *testing.T) {
	store := new(mockStore)
	store.On("Get", "hello").Return("", kvstore.ErrKeyNotFound)

	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestGetValueSucceedsIfAcceptHasAnyContentType(t *testing.T) {
	store := new(mockStore)
	store.On("Get", "hello").Return("world", nil)

	handler := api.GetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/kv/hello", nil)
	request.SetPathValue("key", "hello")
	request.Header.Set("Accept", "*/*")
	handler.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
}
