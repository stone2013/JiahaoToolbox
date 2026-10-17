package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestDefaultsAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	v := s.Get()
	if v != Defaults() {
		t.Fatal("defaults changed")
	}
	if v.MonitorAnimation != "smooth" {
		t.Fatal("monitor animation default should be smooth")
	}
	if v.CheckUpdatesOnStartup || v.AutoDownloadUpdates || v.AllowBetaUpdates { t.Fatal("update preferences must default off") }
	v.Theme = "light"
	v.IntervalMS = 500
	v.Maximized = true
	if e = s.Update(v); e != nil {
		t.Fatal(e)
	}
	s2, e := Open(dir)
	if e != nil || s2.Get() != v {
		t.Fatal("roundtrip failed", e)
	}
	if _, e = os.Stat(filepath.Join(dir, "settings.json.tmp")); !os.IsNotExist(e) {
		t.Fatal("temporary file retained")
	}
}
func TestLegacyPreferencesDefaultMonitorAnimation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"light","intervalMS":1000,"animations":true,"units":"iec","disk":"","interface":"","width":1536,"height":1000,"maximized":false}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Get().MonitorAnimation != "smooth" {
		t.Fatalf("legacy config defaulted to %q", s.Get().MonitorAnimation)
	}
}
func TestValidation(t *testing.T) {
	for _, ms := range []int{500, 1000, 2000, 5000} {
		s := Defaults()
		s.IntervalMS = ms
		if Validate(s) != nil {
			t.Fatal(ms)
		}
	}
	for _, mode := range []string{"off", "standard", "smooth"} {
		s := Defaults()
		s.MonitorAnimation = mode
		if Validate(s) != nil {
			t.Fatal("monitor animation mode rejected", mode)
		}
	}
	bad := []func(*Settings){func(s *Settings) { s.Theme = "pink" }, func(s *Settings) { s.Units = "bits" }, func(s *Settings) { s.MonitorAnimation = "turbo" }, func(s *Settings) { s.IntervalMS = 1 }, func(s *Settings) { s.Width = 1 }, func(s *Settings) { s.Height = 99999 }, func(s *Settings) { s.Disk = string(make([]byte, 17)) }, func(s *Settings) { s.Interface = string(make([]byte, 257)) }}
	for _, f := range bad {
		s := Defaults()
		f(&s)
		if Validate(s) == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
func TestCorruptConfigPreserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	os.WriteFile(path, []byte("not json"), 0600)
	s, e := Open(dir)
	if e == nil || s.Get() != Defaults() {
		t.Fatal("must return defaults with warning")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "not json" {
		t.Fatal("corrupt original overwritten silently")
	}
}
func TestInvalidUpdateNoMutation(t *testing.T) {
	s, _ := Open(t.TempDir())
	v := s.Get()
	v.Theme = "invalid"
	if s.Update(v) == nil || s.Get() != Defaults() {
		t.Fatal("invalid state stored")
	}
}
func TestConcurrentPreferences(t *testing.T) {
	s, _ := Open(t.TempDir())
	var w sync.WaitGroup
	for i := 0; i < 12; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			v := s.Get()
			v.Theme = "system"
			if e := s.Update(v); e != nil {
				t.Error(e)
			}
		}()
	}
	w.Wait()
}

func TestExclusiveProfileLock(t *testing.T) {
	dir := t.TempDir()
	a, e := LockProfile(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := LockProfile(dir)
	if e == nil {
		b.Close()
		t.Fatal("duplicate profile lock accepted")
	}
	a.Close()
	c, e := LockProfile(dir)
	if e != nil {
		t.Fatal(e)
	}
	c.Close()
}
