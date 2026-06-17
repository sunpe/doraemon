package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sunpe/doraemon/internal/config"
)

func writeFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMergesCommandAndRuleDirsButNotSystemD(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "system.toml"), `
[server]
listen = "127.0.0.1:8765"

[storage]
path = "`+filepath.Join(dir, "doraemon.db")+`"
`)
	writeFile(t, filepath.Join(dir, "system.d", "10-ignored.toml"), `
[server]
listen = "0.0.0.0:9999"
`)
	writeFile(t, filepath.Join(dir, "commands.d", "10-kubectl.toml"), `
[executors.kubectl]
binary = "/usr/bin/kubectl"

[tools."k8s.pods.list"]
executor = "kubectl"
argv = ["get", "pods", "-n", "{{namespace}}"]
`)
	writeFile(t, filepath.Join(dir, "rules.d", "10-roles.toml"), `
[roles.readonly]
tools = ["k8s.pods.list"]
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.System.Server.Listen != "127.0.0.1:8765" {
		t.Fatalf("system.d was loaded or system config changed: %q", cfg.System.Server.Listen)
	}
	if _, ok := cfg.Commands.Executors["kubectl"]; !ok {
		t.Fatal("commands.d executor was not loaded")
	}
	if _, ok := cfg.Rules.Roles["readonly"]; !ok {
		t.Fatal("rules.d role was not loaded")
	}
}

func TestLoadRejectsDuplicateToolsAcrossCommandFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "system.toml"), `
[server]
listen = "127.0.0.1:8765"
[storage]
path = "`+filepath.Join(dir, "doraemon.db")+`"
`)
	writeFile(t, filepath.Join(dir, "commands.toml"), `
[executors.kubectl]
binary = "/usr/bin/kubectl"
[tools."k8s.pods.list"]
executor = "kubectl"
argv = ["get", "pods"]
`)
	writeFile(t, filepath.Join(dir, "commands.d", "20-duplicate.toml"), `
[tools."k8s.pods.list"]
executor = "kubectl"
argv = ["get", "pods", "-A"]
`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected duplicate tool error")
	}
	if !strings.Contains(err.Error(), "duplicate tool") {
		t.Fatalf("expected duplicate tool error, got: %v", err)
	}
}

func TestLoadRejectsHighRiskAllowWithoutExpiry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "system.toml"), `
[server]
listen = "127.0.0.1:8765"
[storage]
path = "`+filepath.Join(dir, "doraemon.db")+`"
`)
	writeFile(t, filepath.Join(dir, "commands.toml"), `
[executors.systemctl]
binary = "/usr/bin/systemctl"
[tools."host.service.restart"]
executor = "systemctl"
risk = "high"
argv = ["restart", "{{service}}"]
`)
	writeFile(t, filepath.Join(dir, "rules.toml"), `
[roles.ops]
tools = ["host.service.restart"]

[[high_risk.allow]]
name = "missing-expiry"
tool = "host.service.restart"
users = ["alice"]
`)

	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected high risk expiry validation error")
	}
	if !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("expected expires_at error, got: %v", err)
	}
}

func TestLoadAcceptsValidHighRiskAllow(t *testing.T) {
	dir := t.TempDir()
	expires := time.Now().Add(time.Hour).Format(time.RFC3339)
	writeFile(t, filepath.Join(dir, "system.toml"), `
[server]
listen = "127.0.0.1:8765"
[storage]
path = "`+filepath.Join(dir, "doraemon.db")+`"
`)
	writeFile(t, filepath.Join(dir, "commands.toml"), `
[executors.systemctl]
binary = "/usr/bin/systemctl"
[tools."host.service.restart"]
executor = "systemctl"
risk = "high"
argv = ["restart", "{{service}}"]
`)
	writeFile(t, filepath.Join(dir, "rules.toml"), `
[roles.ops]
tools = ["host.service.restart"]

[[high_risk.allow]]
name = "restart-nginx"
tool = "host.service.restart"
users = ["alice"]
expires_at = "`+expires+`"

[high_risk.allow.params]
service = "nginx"
`)

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(cfg.Rules.HighRisk.Allow) != 1 {
		t.Fatalf("expected one high risk allow, got %d", len(cfg.Rules.HighRisk.Allow))
	}
}
