package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sunpe/doraemon/internal/config"
	"github.com/sunpe/doraemon/internal/store"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

type Decision struct {
	Allowed       bool
	Reason        string
	Command       string
	Args          []string
	HighRiskAllow string
}

func Authorize(cfg config.Config, principal store.Principal, toolName string, params map[string]string, now time.Time) (Decision, error) {
	tool, ok := cfg.Commands.Tools[toolName]
	if !ok {
		return Decision{Allowed: false, Reason: "unknown_tool"}, nil
	}
	if !roleAllows(cfg.Rules.Roles, principal.Roles, toolName) {
		return Decision{Allowed: false, Reason: "tool_not_allowed"}, nil
	}
	allowName := ""
	if strings.EqualFold(tool.Risk, "high") {
		name, ok := highRiskAllowed(cfg.Rules.HighRisk.Allow, principal, toolName, params, now)
		if !ok {
			return Decision{Allowed: false, Reason: "high_risk_not_allowed"}, nil
		}
		allowName = name
	}
	executor, ok := cfg.Commands.Executors[tool.Executor]
	if !ok {
		return Decision{}, fmt.Errorf("tool %q references missing executor %q", toolName, tool.Executor)
	}
	args, err := renderArgs(tool.Argv, params)
	if err != nil {
		return Decision{Allowed: false, Reason: "invalid_input"}, nil
	}
	if denied(executor.Binary, args, cfg.Rules.Deny) {
		return Decision{Allowed: false, Reason: "deny_token"}, nil
	}
	if reason := validateParams(cfg, toolName, params); reason != "" {
		return Decision{Allowed: false, Reason: reason}, nil
	}
	return Decision{Allowed: true, Command: executor.Binary, Args: args, HighRiskAllow: allowName}, nil
}

func validateParams(cfg config.Config, toolName string, params map[string]string) string {
	for key, value := range params {
		if value == "" {
			return "invalid_input"
		}
		if key != "path" && key != "since" && key != "limit" && key != "tail" && !safeName.MatchString(value) {
			return "invalid_input"
		}
	}
	if strings.HasPrefix(toolName, "k8s.") {
		ns := params["namespace"]
		if ns != "" && len(cfg.Rules.Policy.K8s.AllowedNamespaces) > 0 && !contains(cfg.Rules.Policy.K8s.AllowedNamespaces, ns) {
			return "namespace_not_allowed"
		}
		if tail := params["tail"]; tail != "" {
			n, err := strconv.Atoi(tail)
			if err != nil || n < 0 {
				return "invalid_tail"
			}
			if cfg.Rules.Policy.K8s.MaxLogTail > 0 && n > cfg.Rules.Policy.K8s.MaxLogTail {
				return "tail_too_large"
			}
		}
		for _, denied := range cfg.Rules.Policy.K8s.DeniedResources {
			if strings.Contains(strings.ToLower(toolName), strings.ToLower(denied)) {
				return "resource_denied"
			}
		}
	}
	if strings.HasPrefix(toolName, "docker.") {
		if container := params["container"]; container != "" && len(cfg.Rules.Policy.Docker.AllowedContainers) > 0 && !contains(cfg.Rules.Policy.Docker.AllowedContainers, container) {
			return "container_not_allowed"
		}
		if tail := params["tail"]; tail != "" {
			n, err := strconv.Atoi(tail)
			if err != nil || n < 0 {
				return "invalid_tail"
			}
			if cfg.Rules.Policy.Docker.MaxLogTail > 0 && n > cfg.Rules.Policy.Docker.MaxLogTail {
				return "tail_too_large"
			}
		}
	}
	if strings.HasPrefix(toolName, "host.service.") {
		if service := params["service"]; service != "" && len(cfg.Rules.Policy.Host.AllowedServices) > 0 && !contains(cfg.Rules.Policy.Host.AllowedServices, service) {
			return "service_not_allowed"
		}
	}
	return ""
}

func roleAllows(roles map[string]config.Role, principalRoles []string, toolName string) bool {
	for _, roleName := range principalRoles {
		role, ok := roles[roleName]
		if !ok {
			continue
		}
		for _, tool := range role.Tools {
			if tool == toolName {
				return true
			}
		}
	}
	return false
}

func highRiskAllowed(allows []config.HighRiskAllow, principal store.Principal, toolName string, params map[string]string, now time.Time) (string, bool) {
	for _, allow := range allows {
		if allow.Tool != toolName {
			continue
		}
		if !now.Before(allow.ExpiresAt) {
			continue
		}
		if len(allow.Users) > 0 && !contains(allow.Users, principal.User) {
			continue
		}
		if len(allow.TokenIDs) > 0 && !contains(allow.TokenIDs, principal.TokenID) {
			continue
		}
		matches := true
		for key, want := range allow.Params {
			if params[key] != want {
				matches = false
				break
			}
		}
		if matches {
			return allow.Name, true
		}
	}
	return "", false
}

func renderArgs(argv []string, params map[string]string) ([]string, error) {
	args := make([]string, len(argv))
	for i, arg := range argv {
		rendered := arg
		for {
			start := strings.Index(rendered, "{{")
			if start < 0 {
				break
			}
			end := strings.Index(rendered[start:], "}}")
			if end < 0 {
				return nil, fmt.Errorf("unterminated template")
			}
			end += start
			name := rendered[start+2 : end]
			value, ok := params[name]
			if !ok {
				return nil, fmt.Errorf("missing param %q", name)
			}
			rendered = rendered[:start] + value + rendered[end+2:]
		}
		args[i] = rendered
	}
	return args, nil
}

func denied(binary string, args []string, deny config.Deny) bool {
	values := []string{binary}
	values = append(values, args...)
	for _, v := range values {
		base := v
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		for _, rule := range deny.Argv {
			for _, forbidden := range rule.Values {
				if base == forbidden || v == forbidden {
					return true
				}
			}
		}
		for _, rule := range deny.Tokens {
			for _, forbidden := range rule.Values {
				if strings.Contains(v, forbidden) {
					return true
				}
			}
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
