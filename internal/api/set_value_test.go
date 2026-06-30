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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.michaelfcollins3.dev/kvstore/internal/api"
)

func TestSetValue(t *testing.T) {
	store := new(mockStore)
	store.On("Set", "hello", "world").Return(nil)

	handler := api.SetValue(store)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.SetPathValue("key", "hello")
	request.Header.Set("Content-Type", "text/plain; charset=utf-8")
	handler.ServeHTTP(recorder, request)

	t.Run("it sets the value in the store", func(t *testing.T) {
		store.AssertExpectations(t)
	})

	t.Run("it returns the Created status code", func(t *testing.T) {
		assert.Equal(
			t,
			http.StatusCreated,
			recorder.Code,
			"returned http status should be created",
		)
	})
}

func TestSetValueReturnsBadRequestIfContentTypeNotSpecified(t *testing.T) {
	store := new(mockStore)
	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusBadRequest,
		recorder.Code,
		"returned http status should be bad request",
	)
}

func TestSetValueReturnsUnsupportedContentType(t *testing.T) {
	store := new(mockStore)
	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusUnsupportedMediaType,
		recorder.Code,
		"returned http status should be unsupported media type",
	)
}

func TestSetValueFailsForUnsupportedCharset(t *testing.T) {
	store := new(mockStore)
	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.Header.Set("Content-Type", "text/plain; charset=utf-16")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusUnsupportedMediaType,
		recorder.Code,
		"returned http status should be unsupported media type",
	)
}

func TestSetValueReturnsBadRequestIfContentTypeIsUnparseable(t *testing.T) {
	store := new(mockStore)
	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.Header.Set("Content-Type", "text/plain; charset=\"utf-8")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusBadRequest,
		recorder.Code,
		"returned http status should be bad request",
	)
}

func TestSetValueSucceedsIfCharsetNotSpecified(t *testing.T) {
	store := new(mockStore)
	store.On("Set", "hello", "world").Return(nil)

	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.SetPathValue("key", "hello")
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusCreated,
		recorder.Code,
		"returned http status should be created",
	)
}

func TestSetValueFailsIfBodyCannotBeRead(t *testing.T) {
	reader := new(mockReader)
	reader.On("Close").Return(nil)
	reader.On("Read", mock.Anything).Return(
		0,
		fmt.Errorf("read error"),
	)

	store := new(mockStore)
	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		reader,
	)
	request.SetPathValue("key", "hello")
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusInternalServerError,
		recorder.Code,
		"returned http status should be internal server error",
	)
}

func TestSetValueFailsIfStoreReturnsAnError(t *testing.T) {
	store := new(mockStore)
	store.On("Set", "hello", "world").
		Return(fmt.Errorf("error"))

	handler := api.SetValue(store)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/kv/hello",
		strings.NewReader("world"),
	)
	request.SetPathValue("key", "hello")
	request.Header.Set("Content-Type", "text/plain")
	handler.ServeHTTP(recorder, request)

	assert.Equal(
		t,
		http.StatusInternalServerError,
		recorder.Code,
		"returned http status should be internal server error",
	)
}
