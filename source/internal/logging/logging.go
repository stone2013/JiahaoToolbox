package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Writer rotates at 1 MiB and retains two generations, bounding disk growth.
type Writer struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

func Open(dir string) (*Writer, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	w := &Writer{path: filepath.Join(dir, "app.log")}
	e := w.open()
	return w, e
}
func (w *Writer) open() error {
	f, e := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	w.f = f
	if s, e := f.Stat(); e == nil {
		w.size = s.Size()
	}
	return nil
}
func (w *Writer) rotate() error {
	if w.f != nil {
		if e := w.f.Close(); e != nil {
			w.f = nil
			return e
		}
		w.f = nil
	}
	if e := os.Remove(w.path + ".2"); e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := os.Rename(w.path+".1", w.path+".2"); e != nil && !os.IsNotExist(e) {
		return e
	}
	if e := os.Rename(w.path, w.path+".1"); e != nil && !os.IsNotExist(e) {
		return e
	}
	w.size = 0
	return w.open()
}
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return 0, os.ErrClosed
	}
	const limit = 1 << 20
	total := 0
	for len(p) > 0 {
		if w.size >= limit {
			if e := w.rotate(); e != nil {
				return total, e
			}
		}
		part := len(p)
		if int64(part) > limit-w.size {
			part = int(limit - w.size)
		}
		n, e := w.f.Write(p[:part])
		total += n
		w.size += int64(n)
		p = p[n:]
		if e != nil {
			return total, e
		}
		if n != part {
			return total, io.ErrShortWrite
		}
	}
	return total, nil
}
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	e := w.f.Close()
	w.f = nil
	return e
}
func (w *Writer) Read() ([]byte, error) { w.mu.Lock(); defer w.mu.Unlock(); return os.ReadFile(w.path) }
func (w *Writer) Clear() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return os.ErrClosed
	}
	if e := w.f.Close(); e != nil {
		w.f = nil
		return e
	}
	w.f = nil
	for _, suffix := range []string{".1", ".2"} {
		if e := os.Remove(w.path + suffix); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	if e := os.WriteFile(w.path, nil, 0600); e != nil {
		return e
	}
	w.size = 0
	return w.open()
}
func New(w io.Writer) *log.Logger { return log.New(w, "", log.Ldate|log.Ltime|log.LUTC) }
