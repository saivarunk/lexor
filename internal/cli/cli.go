package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/varun/lexor/internal/app"
	"github.com/varun/lexor/internal/config"
	"github.com/varun/lexor/internal/registry"
	"github.com/varun/lexor/internal/server"
	"github.com/varun/lexor/internal/syncgit"
)

func New() *cobra.Command {
	var workspace, host string
	var port int
	root := &cobra.Command{Use: "lexor", Short: "Your local coding practice workbench", Long: banner + "\nYour local coding practice workbench.", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&workspace, "workspace", ".", "Workspace directory")
	open := func(cmd *cobra.Command) (*app.App, error) {
		c, err := config.Load(workspace)
		if err != nil {
			return nil, err
		}
		if cmd.Flags().Changed("host") {
			c.Server.Host = host
		}
		if cmd.Flags().Changed("port") {
			c.Server.Port = port
		}
		return app.Open(c)
	}
	run := func(fn func(*cobra.Command, *app.App, []string) error) func(*cobra.Command, []string) error {
		return func(cmd *cobra.Command, args []string) error {
			a, err := open(cmd)
			if err != nil {
				return err
			}
			defer a.Close()
			return fn(cmd, a, args)
		}
	}
	root.AddCommand(&cobra.Command{Use: "init", Short: "Initialize a workspace and original starter library without overwriting data", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		if err := a.Seed(); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Workspace ready:", a.Config.Root)
		return nil
	})})
	serve := &cobra.Command{Use: "serve", Short: "Serve the embedded workbench on localhost", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		// Seed only a pristine library; existing workspaces may intentionally be empty.
		marker := a.Config.Path(".lexor/seeded")
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			if len(a.Library.List()) == 0 && len(a.Library.Diagnostics()) == 0 {
				if err = a.Seed(); err != nil {
					return err
				}
			}
			if err = os.WriteFile(marker, []byte("1\n"), 0600); err != nil {
				return err
			}
		}
		address := net.JoinHostPort(a.Config.Server.Host, fmt.Sprint(a.Config.Server.Port))
		origin := "http://" + address
		httpServer := &http.Server{Addr: address, Handler: server.New(a, origin), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		listener, err := net.Listen("tcp", address)
		if err != nil {
			return err
		}
		defer listener.Close()
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		httpServer.BaseContext = func(net.Listener) context.Context { return ctx }
		go func() {
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdown)
		}()
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n  Your local coding practice workbench.\n\n  Open       %s\n  Workspace  %s\n\n", banner, origin, a.Config.Root)
		slog.Info("Lexor ready", "url", origin, "workspace", a.Config.Root)
		err = httpServer.Serve(listener)
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	})}
	serve.Flags().StringVar(&host, "host", "127.0.0.1", "Bind address")
	serve.Flags().IntVar(&port, "port", 8080, "HTTP port")
	root.AddCommand(serve)
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "Check workspace and optional runtime prerequisites", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		status := a.Status(cmd.Context())
		for _, d := range status.Diagnostics {
			label := "OK"
			if !d.OK {
				label = "ACTION"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-7s %-20s %s\n", label, d.Name, d.Message)
		}
		for _, issue := range status.Issues {
			fmt.Fprintln(cmd.OutOrStdout(), "  "+issue)
		}
		return nil
	})})
	problem := &cobra.Command{Use: "problem", Short: "Manage portable JSON problems"}
	problem.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		items, err := a.Problems(cmd.Context())
		if err != nil {
			return err
		}
		for _, pr := range items {
			fmt.Fprintf(cmd.OutOrStdout(), "%-38s %-7s %-10s %s\n", pr.ID, pr.Difficulty, pr.Status, pr.Title)
		}
		return nil
	})})
	problem.AddCommand(&cobra.Command{Use: "validate", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		issues := a.Library.Rescan()
		for _, issue := range issues {
			fmt.Fprintln(cmd.OutOrStdout(), issue)
		}
		if len(issues) > 0 {
			return fmt.Errorf("%d invalid problem files", len(issues))
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Validated %d problems against schema 1.0.0\n", len(a.Library.List()))
		return nil
	})})
	problem.AddCommand(&cobra.Command{Use: "export <id>", Args: cobra.ExactArgs(1), RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		pr, err := a.Library.Get(args[0])
		if err != nil {
			return err
		}
		return printJSON(cmd, pr.Problem)
	})})
	var gitSource, apply bool
	imp := &cobra.Command{Use: "import <source>", Short: "Preview a local/HTTPS JSON package or local Git clone; --apply installs it", Args: cobra.ExactArgs(1), RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		resolver := &registry.Resolver{Library: a.Library}
		var pack registry.Package
		var err error
		if gitSource {
			pack, err = resolver.ResolveGit(cmd.Context(), args[0])
		} else {
			pack, err = resolver.Resolve(cmd.Context(), args[0])
		}
		if err != nil {
			return err
		}
		for _, pr := range pack.Problems {
			fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", pr.ID, pr.Title)
		}
		if !apply {
			fmt.Fprintln(cmd.OutOrStdout(), "Preview only. Add --apply to install these problems.")
			return nil
		}
		ids, err := resolver.Install(pack)
		if err != nil {
			return err
		}
		if err = a.Store.RecordInstallation(cmd.Context(), pack.Source, pack.Digest, ids); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Installed %d problems\n", len(ids))
		return nil
	})}
	imp.Flags().BoolVar(&gitSource, "git", false, "Import from a Git repository")
	imp.Flags().BoolVar(&apply, "apply", false, "Install the validated problems")
	problem.AddCommand(imp)
	root.AddCommand(problem)
	root.AddCommand(&cobra.Command{Use: "stats", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		stats, err := a.Store.Stats(cmd.Context(), a.Library.List())
		if err != nil {
			return err
		}
		return printJSON(cmd, stats)
	})})
	plugin := &cobra.Command{Use: "plugin"}
	plugin.AddCommand(&cobra.Command{Use: "list", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		return printJSON(cmd, map[string]any{"apiVersion": "1.0.0", "languages": a.Languages.List(), "sandbox": a.Sandbox.ID(), "evaluators": []string{"exact", "float", "unordered"}})
	})})
	root.AddCommand(plugin)
	var commit, push, preview bool
	sync := &cobra.Command{Use: "sync", Short: "Export allowlisted progress to the configured Git working tree", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		e := &syncgit.Exporter{Store: a.Store, Directory: a.Config.Path(a.Config.Git.Directory)}
		plan, err := e.Preview(cmd.Context())
		if err != nil {
			return err
		}
		if err = printJSON(cmd, plan); err != nil {
			return err
		}
		if preview {
			return nil
		}
		plan, err = e.Export(cmd.Context())
		if err != nil {
			return err
		}
		if push && !commit {
			return fmt.Errorf("--push requires --commit")
		}
		if commit || a.Config.Git.AutoCommit {
			return e.Commit(cmd.Context(), plan.Files, push)
		}
		return nil
	})}
	sync.Flags().BoolVar(&commit, "commit", false, "Commit only exported files")
	sync.Flags().BoolVar(&push, "push", false, "Push using existing Git credentials (requires --commit)")
	sync.Flags().BoolVar(&preview, "preview", false, "List files without writing")
	sync.AddCommand(&cobra.Command{Use: "init", Args: cobra.NoArgs, RunE: run(func(cmd *cobra.Command, a *app.App, args []string) error {
		e := &syncgit.Exporter{Store: a.Store, Directory: a.Config.Path(a.Config.Git.Directory)}
		if err := e.Init(cmd.Context()); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Git export repository ready:", e.Directory)
		return nil
	})})
	root.AddCommand(sync)
	root.AddCommand(&cobra.Command{Use: "openapi", Short: "Print the generated OpenAPI specification", Hidden: true, RunE: func(cmd *cobra.Command, args []string) error { return printJSON(cmd, server.OpenAPI()) }})
	return root
}
func printJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
