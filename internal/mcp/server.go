package mcp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/sunpe/doraemon/internal/audit"
	"github.com/sunpe/doraemon/internal/store"
	"github.com/sunpe/doraemon/internal/tools"
)

type Server struct {
	Tools tools.Service
	Store *store.DB
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id,omitempty"`
	Result  any     `json:"result,omitempty"`
	Error   *errObj `json:"error,omitempty"`
}

type errObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handle)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (s Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, response{JSONRPC: "2.0", Error: &errObj{Code: -32700, Message: "parse_error"}})
		return
	}
	principal, err := s.authenticate(r)
	if err != nil {
		_ = s.Store.WriteAudit(audit.Event{Timestamp: time.Now().UTC(), Decision: "deny", Reason: "unauthorized"})
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: &errObj{Code: -32001, Message: "unauthorized"}})
		return
	}
	switch req.Method {
	case "tools/list":
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: map[string]any{"tools": s.Tools.List()}})
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments any    `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: &errObj{Code: -32602, Message: "invalid_params"}})
			return
		}
		res, err := s.Tools.Call(r.Context(), principal, p.Name, tools.ParamsFromAny(p.Arguments))
		if err != nil {
			writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: &errObj{Code: -32000, Message: err.Error()}})
			return
		}
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Result: res})
	default:
		writeJSON(w, response{JSONRPC: "2.0", ID: req.ID, Error: &errObj{Code: -32601, Message: "method_not_found"}})
	}
}

func (s Server) authenticate(r *http.Request) (store.Principal, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return store.Principal{}, errors.New("missing bearer token")
	}
	return s.Store.AuthenticateToken(strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")), time.Now())
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
