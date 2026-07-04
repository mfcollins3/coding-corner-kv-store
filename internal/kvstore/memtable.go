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

type memtableEntry struct {
	value   string
	deleted bool
}

type memtable map[string]memtableEntry

func newMemtable() memtable {
	return memtable{}
}

func (s memtable) delete(key string) {
	s[key] = memtableEntry{value: "", deleted: true}
}

func (s memtable) get(key string) (string, error) {
	if entry, ok := s[key]; ok {
		if entry.deleted {
			return "", ErrKeyDeleted
		}

		return entry.value, nil
	}

	return "", ErrKeyNotFound
}

func (s memtable) set(key, value string) {
	s[key] = memtableEntry{value: value, deleted: false}
}

func (s memtable) len() int {
	return len(s)
}

func (s memtable) clear() {
	clear(s)
}
