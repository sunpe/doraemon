package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sunpe/doraemon/internal/audit"
	"github.com/sunpe/doraemon/internal/store"
	"github.com/sunpe/doraemon/internal/tools"
)

type principalKey struct{}

type Server struct {
	Tools tools.Service
	Store *store.DB
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/mcp", s.authMiddleware(s.streamableHandler()))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (s Server) streamableHandler() http.Handler {
	return sdkmcp.NewStreamableHTTPHandler(func(r *http.Request) *sdkmcp.Server {
		principal, _ := r.Context().Value(principalKey{}).(store.Principal)
		return s.sdkServer(principal)
	}, &sdkmcp.StreamableHTTPOptions{
		JSONResponse: true,
		Stateless:    true,
	})
}

func (s Server) sdkServer(principal store.Principal) *sdkmcp.Server {
	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "doraemon",
		Version: "dev",
	}, nil)
	for _, name := range s.Tools.List() {
		toolName := name
		server.AddTool(&sdkmcp.Tool{
			Name:        toolName,
			Description: toolName,
			InputSchema: map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		}, func(ctx context.Context, req *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			args := map[string]string{}
			if req.Params != nil && len(req.Params.Arguments) > 0 {
				args = tools.ParamsFromJSON(req.Params.Arguments)
			}
			res, err := s.Tools.Call(ctx, principal, toolName, args)
			if err != nil {
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: err.Error()}},
					IsError: true,
				}, nil
			}
			body, err := json.Marshal(res)
			if err != nil {
				return nil, err
			}
			return &sdkmcp.CallToolResult{
				Content:           []sdkmcp.Content{&sdkmcp.TextContent{Text: string(body)}},
				StructuredContent: map[string]any{"content": res.Content},
			}, nil
		})
	}
	return server
}

func (s Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := s.authenticate(r)
		if err != nil {
			_ = s.Store.WriteAudit(audit.Event{Timestamp: time.Now().UTC(), Decision: "deny", Reason: "unauthorized"})
			slog.Warn("request unauthorized", "path", r.URL.Path, "method", r.Method, "remote_addr", r.RemoteAddr, "reason", "unauthorized")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), principalKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s Server) authenticate(r *http.Request) (store.Principal, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return store.Principal{}, errors.New("missing bearer token")
	}
	return s.Store.AuthenticateToken(strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")), time.Now())
}
