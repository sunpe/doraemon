package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sunpe/doraemon/internal/config"
	doraemonmcp "github.com/sunpe/doraemon/internal/mcp"
	"github.com/sunpe/doraemon/internal/store"
	"github.com/sunpe/doraemon/internal/tools"
)

func TestSDKClientCanListAndCallTools(t *testing.T) {
	db, token := testDB(t)
	cfg := testConfig()
	svc := tools.Service{Config: cfg, Store: db}
	httpServer := httptest.NewServer(doraemonmcp.Server{Tools: svc, Store: db}.Handler())
	defer httpServer.Close()

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "doraemon-test-client", Version: "dev"}, nil)
	session, err := client.Connect(t.Context(), &sdkmcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/mcp",
		HTTPClient: &http.Client{
			Transport: bearerTransport{token: token, base: http.DefaultTransport},
		},
	}, nil)
	if err != nil {
		t.Fatalf("Connect returned error: %v", err)
	}
	defer session.Close()

	list, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}
	if !slices.ContainsFunc(list.Tools, func(tool *sdkmcp.Tool) bool {
		return tool.Name == "host.status.get"
	}) {
		t.Fatalf("host.status.get not listed: %+v", list.Tools)
	}

	call, err := session.CallTool(t.Context(), &sdkmcp.CallToolParams{Name: "host.status.get"})
	if err != nil {
		t.Fatalf("CallTool returned error: %v", err)
	}
	if len(call.Content) != 1 {
		t.Fatalf("expected one content item, got %+v", call.Content)
	}
	text, ok := call.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", call.Content[0])
	}
	if text.Text == "" {
		t.Fatal("expected non-empty JSON text content")
	}
}

func TestUnauthorizedRequestWritesAudit(t *testing.T) {
	db, _ := testDB(t)
	svc := tools.Service{Config: testConfig(), Store: db}
	httpServer := httptest.NewServer(doraemonmcp.Server{Tools: svc, Store: db}.Handler())
	defer httpServer.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, httpServer.URL+"/mcp", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	events, err := db.ListAudit(time.Time{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Decision != "deny" || events[0].Reason != "unauthorized" {
		t.Fatalf("expected unauthorized audit event, got %+v", events)
	}
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	return t.base.RoundTrip(req)
}

func testDB(t *testing.T) (*store.DB, string) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "doraemon.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	if err := db.CreateUser("alice", []string{"readonly"}); err != nil {
		t.Fatal(err)
	}
	created, err := db.CreateToken("alice", "test", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return db, created.Plaintext
}

func testConfig() config.Config {
	return config.Config{
		Rules: config.Rules{
			Roles: map[string]config.Role{
				"readonly": {Tools: []string{"host.status.get"}},
			},
		},
	}
}

var _ context.Context
