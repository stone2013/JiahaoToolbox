package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSelectedDirectoryPreservesSpacesAndShellMetacharacters(t *testing.T) {
	root := t.TempDir()
	selected := filepath.Join(root, "folder with spaces & parentheses")
	if err := os.Mkdir(selected, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := selectedDirectory(selected)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Clean(selected) {
		t.Fatalf("path changed: %q != %q", got, selected)
	}
	if _, err = selectedDirectory("relative folder"); err == nil {
		t.Fatal("relative path must be rejected")
	}
	if _, err = selectedDirectory(filepath.Join(root, "missing folder")); err == nil {
		t.Fatal("missing directory must be rejected")
	}
}
