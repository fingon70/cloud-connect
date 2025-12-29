package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fingon70/cloud-connect/internal/api"
	"github.com/fingon70/cloud-connect/internal/auth"
	"github.com/fingon70/cloud-connect/internal/config"
	"github.com/fingon70/cloud-connect/internal/sync"
	"github.com/fingon70/cloud-connect/internal/ui"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "auth":
		handleAuth(os.Args[2:])
	case "ls":
		handleLs(os.Args[2:])
	case "sync":
		handleSync(os.Args[2:])
	case "whoami":
		handleWhoAmI(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  hidrive auth login")
	fmt.Fprintln(os.Stderr, "  hidrive auth status")
	fmt.Fprintln(os.Stderr, "  hidrive ls <remote-path> [--long] [--json] [--recursive]")
	fmt.Fprintln(os.Stderr, "  hidrive sync <remote-path> <local-dir> [--dry-run]")
	fmt.Fprintln(os.Stderr, "  hidrive whoami")
}

func handleAuth(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "auth requires a subcommand: login or status")
		os.Exit(2)
	}

	switch args[0] {
	case "login":
		cfg, err := config.Load()
		if err != nil {
			ui.Errorf("failed to load config: %v", err)
			os.Exit(1)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		token, err := auth.Login(ctx, cfg)
		if err != nil {
			ui.Errorf("login failed: %v", err)
			os.Exit(1)
		}
		if err := auth.SaveToken(token); err != nil {
			ui.Errorf("failed to save token: %v", err)
			os.Exit(1)
		}
		ui.Infof("login complete, token saved")
	case "status":
		token, err := auth.LoadToken()
		if err != nil {
			ui.Errorf("failed to load token: %v", err)
			os.Exit(1)
		}
		if err := auth.HasValidToken(token, time.Now()); err != nil {
			ui.Errorf("token invalid: %v", err)
			os.Exit(1)
		}
		ui.Infof("token valid until %s", token.Expiry.Format(time.RFC3339))
	default:
		fmt.Fprintf(os.Stderr, "unknown auth subcommand: %s\n", args[0])
		os.Exit(2)
	}
}

func handleLs(args []string) {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	long := fs.Bool("long", false, "show detailed output")
	jsonOut := fs.Bool("json", false, "show JSON output")
	recursive := fs.Bool("recursive", false, "list recursively")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	path := "/"
	if fs.NArg() >= 1 {
		path = fs.Arg(0)
	}

	if *recursive {
		ui.Errorf("recursive listing is not implemented yet")
		os.Exit(2)
	}

	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)

	path = resolvePath(context.Background(), client, path)
	entries, err := client.ListDir(context.Background(), path)
	if err != nil {
		ui.Errorf("list failed: %v", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(entries); err != nil {
			ui.Errorf("failed to encode JSON: %v", err)
			os.Exit(1)
		}
		return
	}

	for _, entry := range entries {
		display := entry.Path
		if display == "" {
			display = entry.Name
		}
		if *long {
			fmt.Printf("%-6s %10d %s %s\n", entry.Type, entry.Size, entry.UpdatedAt.Format(time.RFC3339), display)
		} else {
			fmt.Printf("%s\n", display)
		}
	}
}

func handleSync(args []string) {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "preview changes without writing files")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "sync requires <remote-path> and <local-dir>")
		os.Exit(2)
	}

	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)
	remotePath := resolvePath(context.Background(), client, fs.Arg(0))
	syncer := sync.Syncer{Client: client}
	if err := syncer.Sync(context.Background(), remotePath, fs.Arg(1), sync.Options{DryRun: *dryRun}); err != nil {
		ui.Errorf("sync failed: %v", err)
		os.Exit(1)
	}
}

func handleWhoAmI(args []string) {
	if len(args) != 0 {
		fmt.Fprintln(os.Stderr, "whoami does not take arguments")
		os.Exit(2)
	}

	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)
	user, err := client.GetUser(context.Background())
	if err != nil {
		ui.Errorf("whoami failed: %v", err)
		os.Exit(1)
	}
	if user.Alias == "" {
		ui.Infof("no user information returned")
		return
	}
	ui.Infof("%s (%s)", user.Alias, user.Home)
}

func loadValidToken() auth.Token {
	token, err := auth.LoadToken()
	if err != nil {
		ui.Errorf("failed to load token: %v", err)
		os.Exit(1)
	}
	if err := auth.HasValidToken(token, time.Now()); err != nil {
		ui.Errorf("token invalid: %v (run: hidrive auth login)", err)
		os.Exit(1)
	}
	return token
}

func resolvePath(ctx context.Context, client *api.Client, path string) string {
	if path == "/" {
		return path
	}
	if strings.HasPrefix(path, "/") {
		trimmed := strings.TrimPrefix(path, "/")
		if trimmed == "" {
			return path
		}
		parts := strings.SplitN(trimmed, "/", 2)
		alias := parts[0]
		if alias == "users" || alias == "public" || alias == ".appdata" {
			return path
		}
		user, err := client.GetUser(ctx)
		if err != nil {
			return path
		}
		if user.Alias == alias {
			if len(parts) == 1 {
				return "/users/" + alias
			}
			return "/users/" + alias + "/" + parts[1]
		}
	}
	return path
}
