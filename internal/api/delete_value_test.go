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
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.michaelfcollins3.dev/kvstore/internal/api"
)

func TestDeleteValue(t *testing.T) {
	t.Run("it deletes the value from the store", func(t *testing.T) {
		store := new(mockStore)
		store.On("Delete", "hello").Return(nil).Once()
		handler := api.DeleteValue(store)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/kv/hello", nil)
		request.SetPathValue("key", "hello")

		handler.ServeHTTP(recorder, request)

		t.Run("it returns 204", func(t *testing.T) {
			assert.Equal(t, http.StatusNoContent, recorder.Code)
		})

		t.Run("it deletes the value from the store", func(t *testing.T) {
			store.AssertExpectations(t)
		})
	})

	t.Run(
		"it reports an error if the store returns an error",
		func(t *testing.T) {
			injectedError := errors.New("injected error")
			store := new(mockStore)
			store.On("Delete", "hello").Return(injectedError).Once()
			handler := api.DeleteValue(store)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodDelete, "/kv/hello", nil)
			request.SetPathValue("key", "hello")

			handler.ServeHTTP(recorder, request)

			t.Run("it returns 500", func(t *testing.T) {
				assert.Equal(t, http.StatusInternalServerError, recorder.Code)
			})
		},
	)
}
