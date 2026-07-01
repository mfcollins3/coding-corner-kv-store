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

package kvstore

import (
	"encoding/json"
	"os"
)

// These "API wrappers" are used for unit testing and injecting error scenarios
// for unit tests. Because my implementation makes use of several functions in
// the Go standard library that are hard to mock out, I have created these API
// wrapper functions for those standard library functions so that I can change
// out the implementation in unit tests to validate failure scenarios.

var marshalJSON = json.Marshal
var openFile = os.OpenFile
var openRead = os.Open
var renameFile = os.Rename
var statFile = os.Stat
var syncFile = func(f *os.File) error {
	return f.Sync()
}
var truncateFile = func(f *os.File, size int64) error {
	return f.Truncate(size)
}
