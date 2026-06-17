package policy_test

import (
	"testing"
	"time"

	"github.com/sunpe/doraemon/internal/config"
	"github.com/sunpe/doraemon/internal/policy"
	"github.com/sunpe/doraemon/internal/store"
)

func TestAuthorizeRequiresRoleAndRejectsDenyTokens(t *testing.T) {
	cfg := config.Config{
		Commands: config.Commands{
			Executors: map[string]config.Executor{"kubectl": {Binary: "/usr/bin/kubectl"}},
			Tools: map[string]config.Tool{
				"k8s.pods.list": {Executor: "kubectl", Argv: []string{"get", "pods", "-n", "{{namespace}}"}},
			},
		},
		Rules: config.Rules{
			Roles:  map[string]config.Role{"readonly": {Tools: []string{"k8s.pods.list"}}},
			Deny:   config.Deny{Tokens: []config.DenyRule{{Name: "compose", Values: []string{";", "&&"}}}},
			Policy: config.Policy{K8s: config.K8sPolicy{AllowedNamespaces: []string{"default"}}},
		},
	}
	principal := store.Principal{User: "alice", Roles: []string{"readonly"}, TokenID: "tok_1"}

	decision, err := policy.Authorize(cfg, principal, "k8s.pods.list", map[string]string{"namespace": "default"}, time.Now())
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allow, got %+v", decision)
	}
	if decision.Command != "/usr/bin/kubectl" {
		t.Fatalf("unexpected command: %q", decision.Command)
	}

	decision, err = policy.Authorize(cfg, principal, "k8s.pods.list", map[string]string{"namespace": "default;rm"}, time.Now())
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "deny_token" {
		t.Fatalf("expected deny_token decision, got %+v", decision)
	}

	decision, err = policy.Authorize(cfg, principal, "k8s.pods.list", map[string]string{"namespace": "prod"}, time.Now())
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "namespace_not_allowed" {
		t.Fatalf("expected namespace_not_allowed decision, got %+v", decision)
	}
}

func TestAuthorizeHighRiskRequiresUnexpiredMatchingAllow(t *testing.T) {
	cfg := config.Config{
		Commands: config.Commands{
			Executors: map[string]config.Executor{"systemctl": {Binary: "/usr/bin/systemctl"}},
			Tools: map[string]config.Tool{
				"host.service.restart": {Executor: "systemctl", Risk: "high", Argv: []string{"restart", "{{service}}"}},
			},
		},
		Rules: config.Rules{
			Roles: map[string]config.Role{"ops": {Tools: []string{"host.service.restart"}}},
			HighRisk: config.HighRisk{Allow: []config.HighRiskAllow{{
				Name:      "restart-nginx",
				Tool:      "host.service.restart",
				Users:     []string{"alice"},
				TokenIDs:  []string{"tok_1"},
				ExpiresAt: time.Now().Add(time.Hour),
				Params:    map[string]string{"service": "nginx"},
			}}},
		},
	}
	principal := store.Principal{User: "alice", Roles: []string{"ops"}, TokenID: "tok_1"}

	decision, err := policy.Authorize(cfg, principal, "host.service.restart", map[string]string{"service": "nginx"}, time.Now())
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if !decision.Allowed || decision.HighRiskAllow != "restart-nginx" {
		t.Fatalf("expected high risk allow, got %+v", decision)
	}

	decision, err = policy.Authorize(cfg, principal, "host.service.restart", map[string]string{"service": "docker"}, time.Now())
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "high_risk_not_allowed" {
		t.Fatalf("expected high_risk_not_allowed, got %+v", decision)
	}

	decision, err = policy.Authorize(cfg, principal, "host.service.restart", map[string]string{"service": "nginx"}, time.Now().Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Authorize returned error: %v", err)
	}
	if decision.Allowed || decision.Reason != "high_risk_not_allowed" {
		t.Fatalf("expected expired high risk allow to deny, got %+v", decision)
	}
}
