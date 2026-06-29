package tools_test

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sunpe/doraemon/internal/config"
	"github.com/sunpe/doraemon/internal/store"
	"github.com/sunpe/doraemon/internal/tools"
)

func TestCallLogsDecisionWithoutResponseContent(t *testing.T) {
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})

	db, err := store.Open(filepath.Join(t.TempDir(), "doraemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	svc := tools.Service{
		Store: db,
		Config: config.Config{
			Rules: config.Rules{
				Roles: map[string]config.Role{
					"readonly": {Tools: []string{"host.status.get"}},
				},
			},
		},
	}
	principal := store.Principal{User: "alice", Roles: []string{"readonly"}, TokenID: "tok_test", TokenName: "local"}

	if _, err := svc.Call(t.Context(), principal, "host.status.get", nil); err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	logs := logBuf.String()
	for _, want := range []string{
		`"msg":"tool call completed"`,
		`"tool":"host.status.get"`,
		`"user":"alice"`,
		`"token_id":"tok_test"`,
		`"decision":"allow"`,
		`"duration_ms":`,
	} {
		if !strings.Contains(logs, want) {
			t.Fatalf("expected logs to contain %s, got:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, "hostname") || strings.Contains(logs, "goroutines") {
		t.Fatalf("logs must not include response content, got:\n%s", logs)
	}
}

func TestCallLogsCommandExecutionDetailsWhenEnabled(t *testing.T) {
	logs := captureSlog(t)
	db := testDB(t)
	svc := commandService(db, true)
	principal := store.Principal{User: "alice", Roles: []string{"ops"}, TokenID: "tok_test", TokenName: "local"}

	if _, err := svc.Call(t.Context(), principal, "host.echo", map[string]string{"message": "hello"}); err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	got := logs.String()
	for _, want := range []string{
		`"msg":"command executed"`,
		`"tool":"host.echo"`,
		`"command":"/bin/echo"`,
		`"args":["hello"]`,
		`"duration_ms":`,
		`"exit_code":0`,
		`"stdout_bytes":6`,
		`"stderr_bytes":0`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected logs to contain %s, got:\n%s", want, got)
		}
	}
}

func TestCallDoesNotLogCommandExecutionDetailsWhenDisabled(t *testing.T) {
	logs := captureSlog(t)
	db := testDB(t)
	svc := commandService(db, false)
	principal := store.Principal{User: "alice", Roles: []string{"ops"}, TokenID: "tok_test", TokenName: "local"}

	if _, err := svc.Call(t.Context(), principal, "host.echo", map[string]string{"message": "hello"}); err != nil {
		t.Fatalf("Call returned error: %v", err)
	}

	if strings.Contains(logs.String(), `"msg":"command executed"`) {
		t.Fatalf("expected command execution details to be disabled, got:\n%s", logs.String())
	}
}

func captureSlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logBuf bytes.Buffer
	originalLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, nil)))
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})
	return &logBuf
}

func testDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "doraemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func commandService(db *store.DB, logCommandExecution bool) tools.Service {
	return tools.Service{
		Store: db,
		Config: config.Config{
			System: config.System{
				Logging: config.LoggingConfig{CommandExecution: logCommandExecution},
			},
			Commands: config.Commands{
				Executors: map[string]config.Executor{
					"echo": {Binary: "/bin/echo"},
				},
				Tools: map[string]config.Tool{
					"host.echo": {Executor: "echo", Argv: []string{"{{message}}"}},
				},
			},
			Rules: config.Rules{
				Roles: map[string]config.Role{
					"ops": {Tools: []string{"host.echo"}},
				},
			},
		},
	}
}
