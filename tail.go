// Copyright 2019 Santhosh Kumar Tekuri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tail

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Open opens the named file for tailing with follow-name in mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*Reader, error) {
	return OpenFile(name, 250*time.Millisecond, 10*time.Second)
}

// OpenFile is the generalized Open with config options.
// poll is the polling interval used looking for changes.
// wait is the amount time it waits for the file to appear, before returning io.EOF.
func OpenFile(name string, poll, wait time.Duration) (*Reader, error) {
	r := &Reader{poll: poll, wait: wait}
	if err := r.open(name); err != nil {
		return nil, err
	}
	return r, nil
}

type Reader struct {
	fi os.FileInfo
	f  *os.File
	n  int64
	w  time.Time
	m  time.Time

	// options
	poll time.Duration
	wait time.Duration
}

func (r *Reader) open(name string) error {
	f, err := os.Open(name)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	if r.f != nil {
		_ = r.f.Close()
	}
	r.fi, r.f, r.n, r.w, r.m = fi, f, 0, time.Time{}, fi.ModTime()
	return nil
}

func (r *Reader) Stat() os.FileInfo {
	return r.fi
}

func (r *Reader) Read(p []byte) (n int, err error) {
	for {
		n, err := r.f.Read(p)
		r.n += int64(n)
		if n > 0 {
			if !r.w.IsZero() {
				r.w = time.Now()
			}
			return n, nil
		}
		if err == io.EOF {
			if !r.w.IsZero() {
				if err := r.open(r.f.Name()); err != nil {
					if os.IsNotExist(err) {
						if time.Now().Sub(r.w) >= r.wait {
							fmt.Println("timedout")
							return 0, io.EOF
						}
						time.Sleep(r.poll)
						continue
					}
					return 0, err
				}
				fmt.Println("new file found")
				continue
			}
		L:
			for {
				time.Sleep(r.poll)
				if !r.w.IsZero() {
					break
				}
				s, err := r.status()
				if err != nil {
					return 0, err
				}
				switch s {
				case nochange:
					continue
				case modified:
					break L
				case truncated:
					fmt.Println("file truncated")
					r.n = 0
					r.f.Seek(0, io.SeekStart)
					break L
				case moved:
					fmt.Println("file moved")
					r.w = time.Now()
					break L
				}
			}
			continue
		}
		return n, err
	}
}

func (r *Reader) Seek(offset int64, whence int) (int64, error) {
	n, err := r.f.Seek(offset, whence)
	if err == nil {
		r.n = n
	}
	return n, err
}

func (r *Reader) Offset() int64 {
	return r.n
}

func (r *Reader) Close() error {
	return r.f.Close()
}

type status int

const (
	nochange status = iota
	modified
	moved
	truncated
)

func (r *Reader) status() (status, error) {
	fi, err := os.Stat(r.f.Name())
	if err != nil {
		if os.IsNotExist(err) {
			return moved, nil
		}
		return 0, err
	}
	if !os.SameFile(r.fi, fi) {
		return moved, nil
	}
	size := fi.Size()
	switch {
	case size == r.n:
		if fi.ModTime().After(r.m) {
			r.m = fi.ModTime()
			return truncated, nil
		}
		return nochange, nil
	case size > r.n:
		r.m = fi.ModTime()
		return modified, nil
	default:
		r.m = fi.ModTime()
		return truncated, nil
	}
}
