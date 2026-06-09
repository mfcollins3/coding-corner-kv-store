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

// Store is an interface for a key-value store. It defines the operations for
// writing values to or reading values from the store.
type Store interface {
	// Get reads a value from the key-value store given the unique key that the
	// value is associated with. On success, the value associated with the key
	// is returned.
	//
	// The second return value indicates whether a value for the key exists in
	// the store. If the key exists in the store, the second return value is
	// true. If the key is not found in the store, the returned value is false.
	Get(key string) (string, bool, error)

	// Set writes a value to the key-value store and associates the value with
	// the key. If the key already exists in the store, the existing value is
	// overwritten with the new value.
	Set(key, value string) error
}

// NewStore creates a new key-value store. The created store is an in-memory
// store.
//
// The returned store is safe to be used concurrently by multiple threads.
// Access to the thread is protected by a [sync.Mutex] allowing only one thread
// to access the key-value store at a time.
func NewStore() (Store, error) {
	memtable, err := newMemtable()
	if err != nil {
		return nil, err
	}

	return &singleThreadedStore{
		store: memtable,
	}, nil
}
