package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	"github.com/sunpe/doraemon/internal/config"
	"github.com/sunpe/doraemon/internal/mcp"
	"github.com/sunpe/doraemon/internal/store"
	"github.com/sunpe/doraemon/internal/tools"
)

func NewRootCommand() *cobra.Command {
	var configDir string
	root := &cobra.Command{
		Use:   "doraemon",
		Short: "Minimal MCP operations gateway",
	}
	root.PersistentFlags().StringVar(&configDir, "config-dir", "/etc/doraemon", "configuration directory")

	root.AddCommand(serveCommand(&configDir))
	root.AddCommand(checkConfigCommand(&configDir))
	root.AddCommand(configCommand(&configDir))
	root.AddCommand(userCommand(&configDir))
	root.AddCommand(tokenCommand(&configDir))
	root.AddCommand(auditCommand(&configDir))
	root.AddCommand(policyCommand(&configDir))
	root.AddCommand(&cobra.Command{Use: "version", Run: func(cmd *cobra.Command, args []string) { fmt.Fprintln(cmd.OutOrStdout(), "doraemon dev") }})
	return root
}

func serveCommand(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start MCP HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			server := mcp.Server{Store: db, Tools: tools.Service{Config: cfg, Store: db}}
			httpServer := &http.Server{Addr: cfg.System.Server.Listen, Handler: server.Handler()}
			cmd.Printf("listening on %s\n", cfg.System.Server.Listen)
			slog.Info("server starting", "listen", cfg.System.Server.Listen, "storage_path", cfg.System.Storage.Path)
			if err := httpServer.ListenAndServe(); err != nil {
				slog.Error("server stopped", "error", err)
				return err
			}
			return nil
		},
	}
}

func checkConfigCommand(configDir *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check-config",
		Short: "Validate configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := config.Load(*configDir); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "config ok")
			return nil
		},
	}
}

func configCommand(configDir *string) *cobra.Command {
	c := &cobra.Command{Use: "config"}
	var format string
	dump := &cobra.Command{
		Use:   "dump",
		Short: "Print effective configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configDir)
			if err != nil {
				return err
			}
			if format != "json" && format != "toml" {
				return fmt.Errorf("unsupported format %q", format)
			}
			if format == "toml" {
				var buf bytes.Buffer
				if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
					return err
				}
				fmt.Fprint(cmd.OutOrStdout(), buf.String())
				return nil
			}
			buf, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(buf))
			return nil
		},
	}
	dump.Flags().StringVar(&format, "format", "json", "output format")
	c.AddCommand(dump)
	return c
}

func userCommand(configDir *string) *cobra.Command {
	c := &cobra.Command{Use: "user"}
	var roles string
	create := &cobra.Command{
		Use:   "create <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Create user",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			return db.CreateUser(args[0], splitCSV(roles))
		},
	}
	create.Flags().StringVar(&roles, "roles", "", "comma-separated roles")
	list := &cobra.Command{
		Use:   "list",
		Short: "List users",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			users, err := db.ListUsers()
			if err != nil {
				return err
			}
			return printJSON(cmd, users)
		},
	}
	disable := &cobra.Command{
		Use:   "disable <name>",
		Args:  cobra.ExactArgs(1),
		Short: "Disable user",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			return db.DisableUser(args[0])
		},
	}
	c.AddCommand(create, list, disable)
	return c
}

func tokenCommand(configDir *string) *cobra.Command {
	c := &cobra.Command{Use: "token"}
	var user, name, ttl string
	create := &cobra.Command{
		Use:   "create",
		Short: "Create token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if user == "" || name == "" || ttl == "" {
				return fmt.Errorf("--user, --name and --ttl are required")
			}
			d, err := time.ParseDuration(ttl)
			if err != nil {
				return err
			}
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			created, err := db.CreateToken(user, name, time.Now().Add(d))
			if err != nil {
				return err
			}
			return printJSON(cmd, created)
		},
	}
	create.Flags().StringVar(&user, "user", "", "user")
	create.Flags().StringVar(&name, "name", "", "token name")
	create.Flags().StringVar(&ttl, "ttl", "", "token TTL, e.g. 720h")
	list := &cobra.Command{
		Use:   "list",
		Short: "List tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			tokens, err := db.ListTokens(user)
			if err != nil {
				return err
			}
			return printJSON(cmd, tokens)
		},
	}
	list.Flags().StringVar(&user, "user", "", "user")
	revoke := &cobra.Command{
		Use:   "revoke <token-id>",
		Args:  cobra.ExactArgs(1),
		Short: "Revoke token",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			return db.RevokeToken(args[0])
		},
	}
	var rotateTTL string
	rotate := &cobra.Command{
		Use:   "rotate <token-id>",
		Args:  cobra.ExactArgs(1),
		Short: "Revoke token and create a replacement",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rotateTTL == "" {
				return fmt.Errorf("--ttl is required")
			}
			d, err := time.ParseDuration(rotateTTL)
			if err != nil {
				return err
			}
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			created, err := db.RotateToken(args[0], time.Now().Add(d))
			if err != nil {
				return err
			}
			return printJSON(cmd, created)
		},
	}
	rotate.Flags().StringVar(&rotateTTL, "ttl", "", "token TTL, e.g. 720h")
	c.AddCommand(create, list, revoke, rotate)
	return c
}

func auditCommand(configDir *string) *cobra.Command {
	c := &cobra.Command{Use: "audit"}
	var sinceRaw string
	list := &cobra.Command{
		Use:   "list",
		Short: "List audit events",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, db, err := loadRuntime(*configDir)
			if err != nil {
				return err
			}
			defer db.Close()
			var since time.Time
			if sinceRaw != "" {
				d, err := time.ParseDuration(sinceRaw)
				if err != nil {
					return err
				}
				since = time.Now().Add(-d)
			}
			events, err := db.ListAudit(since, 100)
			if err != nil {
				return err
			}
			return printJSON(cmd, events)
		},
	}
	list.Flags().StringVar(&sinceRaw, "since", "", "duration, e.g. 1h")
	c.AddCommand(list)
	return c
}

func policyCommand(configDir *string) *cobra.Command {
	c := &cobra.Command{Use: "policy"}
	var tool, input string
	test := &cobra.Command{
		Use:   "test",
		Short: "Validate that tool and input are parseable",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(*configDir)
			if err != nil {
				return err
			}
			if _, ok := cfg.Commands.Tools[tool]; !ok {
				return fmt.Errorf("unknown configured command tool %q", tool)
			}
			body, err := os.ReadFile(input)
			if err != nil {
				return err
			}
			var params map[string]string
			if err := json.Unmarshal(body, &params); err != nil {
				return err
			}
			return printJSON(cmd, params)
		},
	}
	test.Flags().StringVar(&tool, "tool", "", "tool name")
	test.Flags().StringVar(&input, "input", "", "input JSON file")
	c.AddCommand(test)
	return c
}

func loadRuntime(configDir string) (config.Config, *store.DB, error) {
	cfg, err := config.Load(configDir)
	if err != nil {
		return config.Config{}, nil, err
	}
	if !filepath.IsAbs(cfg.System.Storage.Path) {
		cfg.System.Storage.Path = filepath.Join(configDir, cfg.System.Storage.Path)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.System.Storage.Path), 0o700); err != nil {
		return config.Config{}, nil, err
	}
	db, err := store.Open(cfg.System.Storage.Path)
	if err != nil {
		return config.Config{}, nil, err
	}
	return cfg, db, nil
}

func printJSON(cmd *cobra.Command, value any) error {
	buf, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(buf))
	return nil
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func RunForTest(ctx context.Context, args ...string) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}
