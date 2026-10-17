package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLogRotationAndClear(t *testing.T) {
	dir := t.TempDir()
	w, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	p := bytes.Repeat([]byte("x"), 512<<10)
	for i := 0; i < 10; i++ {
		if _, e = w.Write(p); e != nil {
			t.Fatal(e)
		}
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 3 {
		t.Fatal("must keep only 3 log files", len(files))
	}
	for _, f := range files {
		s, _ := f.Info()
		if s.Size() > 1<<20 {
			t.Fatal("unbounded log", s.Size())
		}
	}
	if e = w.Clear(); e != nil {
		t.Fatal(e)
	}
	b, _ := w.Read()
	if len(b) != 0 {
		t.Fatal("clear failed")
	}
	if _, e = os.Stat(filepath.Join(dir, "app.log.1")); !os.IsNotExist(e) {
		t.Fatal("rotation not removed")
	}
	w.Close()
	w.Close()
	if _, e = w.Write([]byte("x")); e == nil {
		t.Fatal("closed writer accepted write")
	}
}
func TestConcurrentLogging(t *testing.T) {
	w, _ := Open(t.TempDir())
	defer w.Close()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				if _, e := w.Write([]byte("entry\n")); e != nil {
					t.Error(e)
				}
			}
		}()
	}
	wg.Wait()
}

func TestOversizedWriteStillBounded(t *testing.T) {
	dir := t.TempDir()
	w, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer w.Close()
	p := bytes.Repeat([]byte("x"), 5<<20)
	n, e := w.Write(p)
	if e != nil || n != len(p) {
		t.Fatal(n, e)
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 3 {
		t.Fatal(len(files))
	}
	for _, f := range files {
		s, _ := f.Info()
		if s.Size() > 1<<20 {
			t.Fatal(s.Size())
		}
	}
}
