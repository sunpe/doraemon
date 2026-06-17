package pathguard_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sunpe/doraemon/internal/pathguard"
)

func TestResolveReadAllowsPathInsideRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "app.log")
	if err := os.WriteFile(file, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := pathguard.ResolveRead(file, []string{root})
	if err != nil {
		t.Fatalf("ResolveRead returned error: %v", err)
	}
	want, err := filepath.EvalSymlinks(file)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != want {
		t.Fatalf("unexpected path: %q", resolved)
	}
}

func TestResolveReadRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.log")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := pathguard.ResolveRead(filepath.Join(root, "..", filepath.Base(outside), "secret.log"), []string{root}); err == nil {
		t.Fatal("expected traversal outside root to be rejected")
	}

	link := filepath.Join(root, "escape")
	if err := os.Symlink(outsideFile, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("symlink not available")
		}
		t.Fatal(err)
	}
	if _, err := pathguard.ResolveRead(link, []string{root}); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}
