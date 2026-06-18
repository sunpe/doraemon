package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDumpTOML(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "system.toml"), `
[server]
listen = "127.0.0.1:8765"

[storage]
path = "`+filepath.Join(dir, "doraemon.db")+`"
`)

	var out bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--config-dir", dir, "config", "dump", "--format", "toml"})
	cmd.SetOut(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config dump returned error: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "[system.server]") {
		t.Fatalf("expected TOML output to contain [system.server], got:\n%s", got)
	}
	if strings.HasPrefix(strings.TrimSpace(got), "{") {
		t.Fatalf("expected TOML output, got JSON:\n%s", got)
	}
}

func writeTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
