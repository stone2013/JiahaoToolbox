//go:build windows && amd64

package native

import (
	"io"
	"log"
	"strings"
	"testing"
)

func TestMissingRuntimeIsExplicit(t *testing.T) {
	w := &host{o: Options{DataDir: t.TempDir(), Log: log.New(io.Discard, "", 0)}}
	err := w.createEnvironmentWithRuntime(nil, "")
	if err == nil || !strings.Contains(err.Error(), "未找到 x64 Microsoft Edge WebView2 Runtime") {
		t.Fatalf("unexpected runtime error: %v", err)
	}
}
func TestStartupReadinessRequiresNavigationAndHandshake(t *testing.T) {
	w := &host{o: Options{Log: log.New(io.Discard, "", 0)}}
	w.loaded = true
	w.checkReady()
	if w.ready {
		t.Fatal("navigation alone must not mark ready")
	}
	w.mounted = true
	w.checkReady()
	if !w.ready {
		t.Fatal("both stages must mark ready")
	}
	closed := &host{closed: true, loaded: true, mounted: true}
	closed.checkReady()
	if closed.ready {
		t.Fatal("closed window must not revive")
	}
}
func TestExplicitStartupThemes(t *testing.T) {
	if StartupTheme("light") != "light" || StartupTheme("dark") != "dark" {
		t.Fatal("saved themes ignored")
	}
}
