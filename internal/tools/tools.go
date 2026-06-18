package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"runtime"
	"sort"
	"strconv"
	"time"

	"github.com/sunpe/doraemon/internal/audit"
	"github.com/sunpe/doraemon/internal/config"
	"github.com/sunpe/doraemon/internal/executor"
	"github.com/sunpe/doraemon/internal/pathguard"
	"github.com/sunpe/doraemon/internal/policy"
	"github.com/sunpe/doraemon/internal/store"
)

type Service struct {
	Config config.Config
	Store  *store.DB
}

type Response struct {
	Content any `json:"content"`
}

func (s Service) List() []string {
	builtins := config.BuiltinToolNames()
	names := make([]string, 0, len(s.Config.Commands.Tools)+len(builtins))
	for name := range s.Config.Commands.Tools {
		names = append(names, name)
	}
	names = append(names, builtins...)
	sort.Strings(names)
	return names
}

func (s Service) Call(ctx context.Context, principal store.Principal, toolName string, params map[string]string) (Response, error) {
	start := time.Now()
	event := audit.Event{Timestamp: start.UTC(), User: principal.User, TokenID: principal.TokenID, TokenName: principal.TokenName, Tool: toolName}
	writeAudit := func(decision, reason string, extra func(*audit.Event)) {
		event.Decision = decision
		event.Reason = reason
		event.DurationMS = time.Since(start).Milliseconds()
		if extra != nil {
			extra(&event)
		}
		_ = s.Store.WriteAudit(event)
	}

	if config.IsBuiltinTool(toolName) {
		if !policy.RoleAllows(s.Config.Rules.Roles, principal.Roles, toolName) {
			writeAudit("deny", "tool_not_allowed", nil)
			return Response{}, errors.New("tool_not_allowed")
		}
		out, err := s.callBuiltin(toolName, params)
		if err != nil {
			writeAudit("deny", err.Error(), nil)
			return Response{}, err
		}
		writeAudit("allow", "", nil)
		return Response{Content: out}, nil
	}

	decision, err := policy.Authorize(s.Config, principal, toolName, params, time.Now())
	if err != nil {
		writeAudit("error", err.Error(), nil)
		return Response{}, err
	}
	if !decision.Allowed {
		writeAudit("deny", decision.Reason, nil)
		return Response{}, errors.New(decision.Reason)
	}
	timeout := parseDuration(s.Config.System.Limits.DefaultTimeout, 10*time.Second)
	result := executor.Run(ctx, timeout, decision.Command, decision.Args, s.Config.System.Limits.MaxStdoutBytes, s.Config.System.Limits.MaxStderrBytes)
	writeAudit("allow", "", func(e *audit.Event) {
		e.Command = decision.Command
		e.Args = decision.Args
		e.ExitCode = result.ExitCode
		e.StdoutBytes = len(result.Stdout)
		e.StderrBytes = len(result.Stderr)
		e.DurationMS = result.DurationMS
		e.HighRiskAllow = decision.HighRiskAllow
	})
	return Response{Content: map[string]any{
		"stdout":    string(result.Stdout),
		"stderr":    string(result.Stderr),
		"exit_code": result.ExitCode,
	}}, nil
}

func (s Service) callBuiltin(name string, params map[string]string) (any, error) {
	switch name {
	case "host.status.get":
		return hostStatus()
	case "host.disk.list":
		return hostDisk()
	case "host.process.list":
		return hostProcess()
	case "file.read":
		return s.fileRead(params)
	case "file.list":
		return s.fileList(params)
	case "audit.query":
		return s.auditQuery(params)
	default:
		return nil, errors.New("unknown_tool")
	}
}

func (s Service) fileRead(params map[string]string) (any, error) {
	path := params["path"]
	resolved, err := pathguard.ResolveRead(path, readRoots(s.Config))
	if err != nil {
		return nil, err
	}
	body, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	return map[string]any{"path": resolved, "content": string(body)}, nil
}

func (s Service) fileList(params map[string]string) (any, error) {
	path := params["path"]
	resolved, err := pathguard.ResolveRead(path, readRoots(s.Config))
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		out = append(out, map[string]any{"name": e.Name(), "dir": e.IsDir(), "size": size})
	}
	return out, nil
}

func (s Service) auditQuery(params map[string]string) (any, error) {
	limit := 100
	if v := params["limit"]; v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, err
		}
		limit = n
	}
	var since time.Time
	if v := params["since"]; v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, err
		}
		since = time.Now().Add(-d)
	}
	return s.Store.ListAudit(since, limit)
}

func hostStatus() (any, error) {
	hostname, _ := os.Hostname()
	return map[string]any{
		"hostname":   hostname,
		"goos":       runtime.GOOS,
		"goarch":     runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
		"goroutines": runtime.NumGoroutine(),
	}, nil
}

func hostDisk() (any, error) {
	return map[string]any{"note": "disk details are platform specific in MVP"}, nil
}

func hostProcess() (any, error) {
	return map[string]any{"note": "process listing is intentionally minimal in MVP"}, nil
}

func readRoots(cfg config.Config) []string {
	roots := make([]string, 0, len(cfg.Rules.Paths.Read))
	for _, p := range cfg.Rules.Paths.Read {
		roots = append(roots, p.Root)
	}
	return roots
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}
	return d
}

func ParamsFromJSON(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&m); err != nil {
		return out
	}
	for k, value := range m {
		switch typed := value.(type) {
		case string:
			out[k] = typed
		case json.Number:
			out[k] = typed.String()
		case bool:
			out[k] = strconv.FormatBool(typed)
		default:
			rawValue, err := json.Marshal(typed)
			if err == nil {
				out[k] = string(rawValue)
			}
		}
	}
	return out
}
