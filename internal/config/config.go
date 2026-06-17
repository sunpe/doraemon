package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	System   System
	Commands Commands
	Rules    Rules
}

type System struct {
	Server  ServerConfig  `toml:"server"`
	Storage StorageConfig `toml:"storage"`
	Audit   AuditConfig   `toml:"audit"`
	Limits  LimitsConfig  `toml:"limits"`
}

type ServerConfig struct {
	Listen       string `toml:"listen"`
	ReadTimeout  string `toml:"read_timeout"`
	WriteTimeout string `toml:"write_timeout"`
}

type StorageConfig struct {
	Type string `toml:"type"`
	Path string `toml:"path"`
}

type AuditConfig struct {
	Enabled       bool `toml:"enabled"`
	RetentionDays int  `toml:"retention_days"`
	StoreStdout   bool `toml:"store_stdout"`
	StoreStderr   bool `toml:"store_stderr"`
}

type LimitsConfig struct {
	DefaultTimeout string `toml:"default_timeout"`
	MaxStdoutBytes int64  `toml:"max_stdout_bytes"`
	MaxStderrBytes int64  `toml:"max_stderr_bytes"`
}

type Commands struct {
	Executors map[string]Executor `toml:"executors"`
	Tools     map[string]Tool     `toml:"tools"`
}

type Executor struct {
	Binary  string `toml:"binary"`
	Timeout string `toml:"timeout"`
}

type Tool struct {
	Executor string   `toml:"executor"`
	Risk     string   `toml:"risk"`
	Argv     []string `toml:"argv"`
}

type Rules struct {
	Roles    map[string]Role `toml:"roles"`
	Deny     Deny            `toml:"deny"`
	Paths    Paths           `toml:"paths"`
	Policy   Policy          `toml:"policy"`
	HighRisk HighRisk        `toml:"high_risk"`
}

type Role struct {
	Tools []string `toml:"tools"`
}

type Deny struct {
	Tokens []DenyRule `toml:"tokens"`
	Argv   []DenyRule `toml:"argv"`
}

type DenyRule struct {
	Name   string   `toml:"name"`
	Reason string   `toml:"reason"`
	Values []string `toml:"values"`
}

type Paths struct {
	Read []PathRule `toml:"read"`
}

type PathRule struct {
	Root string `toml:"root"`
}

type Policy struct {
	K8s    K8sPolicy    `toml:"k8s"`
	Docker DockerPolicy `toml:"docker"`
	Host   HostPolicy   `toml:"host"`
}

type K8sPolicy struct {
	AllowedNamespaces []string `toml:"allowed_namespaces"`
	DeniedResources   []string `toml:"denied_resources"`
	MaxLogTail        int      `toml:"max_log_tail"`
}

type DockerPolicy struct {
	AllowedContainers []string `toml:"allowed_containers"`
	MaxLogTail        int      `toml:"max_log_tail"`
}

type HostPolicy struct {
	AllowedServices []string `toml:"allowed_services"`
}

type HighRisk struct {
	Allow []HighRiskAllow `toml:"allow"`
}

type HighRiskAllow struct {
	Name      string            `toml:"name"`
	Tool      string            `toml:"tool"`
	Users     []string          `toml:"users"`
	TokenIDs  []string          `toml:"token_ids"`
	ExpiresAt time.Time         `toml:"expires_at"`
	Params    map[string]string `toml:"params"`
}

func Load(dir string) (Config, error) {
	var cfg Config
	if err := loadSystem(filepath.Join(dir, "system.toml"), &cfg.System); err != nil {
		return Config{}, err
	}
	cfg.Commands.Executors = map[string]Executor{}
	cfg.Commands.Tools = map[string]Tool{}
	cfg.Rules.Roles = map[string]Role{}

	for _, path := range orderedFiles(dir, "commands.toml", "commands.d") {
		var next Commands
		if err := decodeFile(path, &next); err != nil {
			return Config{}, err
		}
		if err := mergeCommands(&cfg.Commands, next, path); err != nil {
			return Config{}, err
		}
	}
	for _, path := range orderedFiles(dir, "rules.toml", "rules.d") {
		var next Rules
		if err := decodeFile(path, &next); err != nil {
			return Config{}, err
		}
		if err := mergeRules(&cfg.Rules, next, path); err != nil {
			return Config{}, err
		}
	}
	defaults(&cfg)
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadSystem(path string, out *System) error {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("system config %s: %w", path, err)
		}
		return err
	}
	return decodeFile(path, out)
}

func orderedFiles(dir, mainFile, dName string) []string {
	var out []string
	main := filepath.Join(dir, mainFile)
	if st, err := os.Stat(main); err == nil && !st.IsDir() {
		out = append(out, main)
	}
	d := filepath.Join(dir, dName)
	entries, err := os.ReadDir(d)
	if err != nil {
		return out
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".toml") || strings.HasSuffix(name, "~") {
			continue
		}
		out = append(out, filepath.Join(d, name))
	}
	sort.Strings(out[1:])
	return out
}

func decodeFile(path string, out any) error {
	if _, err := toml.DecodeFile(path, out); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func mergeCommands(dst *Commands, src Commands, path string) error {
	if dst.Executors == nil {
		dst.Executors = map[string]Executor{}
	}
	if dst.Tools == nil {
		dst.Tools = map[string]Tool{}
	}
	for name, executor := range src.Executors {
		if _, exists := dst.Executors[name]; exists {
			return fmt.Errorf("%s: duplicate executor %q", path, name)
		}
		dst.Executors[name] = executor
	}
	for name, tool := range src.Tools {
		if _, exists := dst.Tools[name]; exists {
			return fmt.Errorf("%s: duplicate tool %q", path, name)
		}
		dst.Tools[name] = tool
	}
	return nil
}

func mergeRules(dst *Rules, src Rules, path string) error {
	if dst.Roles == nil {
		dst.Roles = map[string]Role{}
	}
	for name, role := range src.Roles {
		if _, exists := dst.Roles[name]; exists {
			return fmt.Errorf("%s: duplicate role %q", path, name)
		}
		dst.Roles[name] = role
	}
	if err := appendDeny(&dst.Deny.Tokens, src.Deny.Tokens, path); err != nil {
		return err
	}
	if err := appendDeny(&dst.Deny.Argv, src.Deny.Argv, path); err != nil {
		return err
	}
	dst.Paths.Read = append(dst.Paths.Read, src.Paths.Read...)
	dst.HighRisk.Allow = append(dst.HighRisk.Allow, src.HighRisk.Allow...)
	if err := mergePolicy(&dst.Policy, src.Policy, path); err != nil {
		return err
	}
	return nil
}

func appendDeny(dst *[]DenyRule, src []DenyRule, path string) error {
	seen := map[string]struct{}{}
	for _, rule := range *dst {
		if rule.Name != "" {
			seen[rule.Name] = struct{}{}
		}
	}
	for _, rule := range src {
		if rule.Name != "" {
			if _, exists := seen[rule.Name]; exists {
				return fmt.Errorf("%s: duplicate deny rule %q", path, rule.Name)
			}
			seen[rule.Name] = struct{}{}
		}
		*dst = append(*dst, rule)
	}
	return nil
}

func mergePolicy(dst *Policy, src Policy, path string) error {
	if len(src.K8s.AllowedNamespaces) > 0 {
		if len(dst.K8s.AllowedNamespaces) > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.k8s.allowed_namespaces", path)
		}
		dst.K8s.AllowedNamespaces = src.K8s.AllowedNamespaces
	}
	if len(src.K8s.DeniedResources) > 0 {
		if len(dst.K8s.DeniedResources) > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.k8s.denied_resources", path)
		}
		dst.K8s.DeniedResources = src.K8s.DeniedResources
	}
	if src.K8s.MaxLogTail > 0 {
		if dst.K8s.MaxLogTail > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.k8s.max_log_tail", path)
		}
		dst.K8s.MaxLogTail = src.K8s.MaxLogTail
	}
	if len(src.Docker.AllowedContainers) > 0 {
		if len(dst.Docker.AllowedContainers) > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.docker.allowed_containers", path)
		}
		dst.Docker.AllowedContainers = src.Docker.AllowedContainers
	}
	if src.Docker.MaxLogTail > 0 {
		if dst.Docker.MaxLogTail > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.docker.max_log_tail", path)
		}
		dst.Docker.MaxLogTail = src.Docker.MaxLogTail
	}
	if len(src.Host.AllowedServices) > 0 {
		if len(dst.Host.AllowedServices) > 0 {
			return fmt.Errorf("%s: duplicate policy field policy.host.allowed_services", path)
		}
		dst.Host.AllowedServices = src.Host.AllowedServices
	}
	return nil
}

func defaults(cfg *Config) {
	if cfg.System.Server.Listen == "" {
		cfg.System.Server.Listen = "127.0.0.1:8765"
	}
	if cfg.System.Storage.Type == "" {
		cfg.System.Storage.Type = "bbolt"
	}
	if cfg.System.Limits.DefaultTimeout == "" {
		cfg.System.Limits.DefaultTimeout = "10s"
	}
	if cfg.System.Audit.RetentionDays == 0 {
		cfg.System.Audit.RetentionDays = 30
	}
}

func validate(cfg Config) error {
	if cfg.System.Storage.Path == "" {
		return errors.New("storage.path is required")
	}
	for name, executor := range cfg.Commands.Executors {
		if executor.Binary == "" || !filepath.IsAbs(executor.Binary) {
			return fmt.Errorf("executor %q binary must be an absolute path", name)
		}
		if isShellExecutable(executor.Binary) {
			return fmt.Errorf("executor %q binary must not be a shell", name)
		}
	}
	for name, tool := range cfg.Commands.Tools {
		if _, ok := cfg.Commands.Executors[tool.Executor]; !ok {
			return fmt.Errorf("tool %q references unknown executor %q", name, tool.Executor)
		}
		if strings.EqualFold(tool.Risk, "high") && tool.Executor == "" {
			return fmt.Errorf("high risk tool %q must reference an executor", name)
		}
		for _, arg := range tool.Argv {
			if containsShellRisk(arg) || isShellExecutable(arg) {
				return fmt.Errorf("tool %q argv contains forbidden shell token %q", name, arg)
			}
		}
	}
	for name, role := range cfg.Rules.Roles {
		for _, tool := range role.Tools {
			if _, ok := cfg.Commands.Tools[tool]; !ok && !isBuiltinTool(tool) {
				return fmt.Errorf("role %q references unknown tool %q", name, tool)
			}
		}
	}
	highRiskNames := map[string]struct{}{}
	for _, allow := range cfg.Rules.HighRisk.Allow {
		if allow.Name == "" {
			return errors.New("high_risk.allow name is required")
		}
		if _, exists := highRiskNames[allow.Name]; exists {
			return fmt.Errorf("duplicate high_risk.allow %q", allow.Name)
		}
		highRiskNames[allow.Name] = struct{}{}
		if allow.Tool == "" {
			return fmt.Errorf("high_risk.allow %q tool is required", allow.Name)
		}
		if allow.ExpiresAt.IsZero() {
			return fmt.Errorf("high_risk.allow %q expires_at is required", allow.Name)
		}
		if _, ok := cfg.Commands.Tools[allow.Tool]; !ok && !isBuiltinTool(allow.Tool) {
			return fmt.Errorf("high_risk.allow %q references unknown tool %q", allow.Name, allow.Tool)
		}
	}
	for _, p := range cfg.Rules.Paths.Read {
		if !filepath.IsAbs(p.Root) {
			return fmt.Errorf("paths.read root %q must be absolute", p.Root)
		}
	}
	return nil
}

func containsShellRisk(s string) bool {
	for _, token := range []string{"|", "&&", "||", ";", "`", "$(", ">", "<"} {
		if strings.Contains(s, token) {
			return true
		}
	}
	return false
}

func isBuiltinTool(name string) bool {
	switch name {
	case "host.status.get", "host.disk.list", "host.process.list", "file.read", "file.list", "audit.query":
		return true
	default:
		return false
	}
}

func isShellExecutable(s string) bool {
	base := filepath.Base(s)
	switch strings.ToLower(base) {
	case "sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh":
		return true
	default:
		return false
	}
}
