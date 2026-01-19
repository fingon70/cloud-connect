package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fingon70/cloud-connect/internal/api"
	"github.com/fingon70/cloud-connect/internal/auth"
	"github.com/fingon70/cloud-connect/internal/config"
	"github.com/fingon70/cloud-connect/internal/model"
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
	case "download":
		handleDownload(os.Args[2:])
	case "completion":
		handleCompletion(os.Args[2:])
	case "__complete":
		handleComplete(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, "  hidrive sync <remote-path> <local-dir> [--dry-run] [--delete] [--report <path>]")
	fmt.Fprintln(os.Stderr, "  hidrive sync <local-path> <remote-path> [--upload] [--dry-run] [--report <path>]")
	fmt.Fprintln(os.Stderr, "  hidrive download <remote-file> <local-path>")
	fmt.Fprintln(os.Stderr, "  hidrive completion <bash|zsh|fish>")
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
		if updated, err := config.PromptMissing(&cfg, os.Stdin, os.Stdout); err != nil {
			ui.Errorf("failed to read config values: %v", err)
			os.Exit(1)
		} else if updated {
			if err := config.Save(cfg); err != nil {
				ui.Errorf("failed to save config: %v", err)
				os.Exit(1)
			}
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
	parsed, err := parseSyncArgs(args)
	if err != nil {
		ui.Errorf("sync argument error: %v", err)
		os.Exit(2)
	}

	if len(parsed.Positional) < 2 {
		fmt.Fprintln(os.Stderr, "sync requires <remote-path> and <local-dir>")
		os.Exit(2)
	}

	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)
	direction := syncDownload
	if parsed.Upload {
		direction = syncUpload
	} else {
		detected, ok := detectSyncDirection(parsed.Positional[0], parsed.Positional[1])
		if !ok {
			ui.Errorf("unable to determine sync direction; use --upload to force local-to-remote sync")
			os.Exit(2)
		}
		direction = detected
	}

	var remotePath string
	var localPath string
	if direction == syncUpload {
		localPath = parsed.Positional[0]
		remotePath = resolvePath(context.Background(), client, parsed.Positional[1])
	} else {
		remotePath = resolvePath(context.Background(), client, parsed.Positional[0])
		localPath = parsed.Positional[1]
	}
	expandedLocalPath, err := expandPath(localPath)
	if err != nil {
		ui.Errorf("invalid local path: %v", err)
		os.Exit(2)
	}
	syncer := sync.Syncer{Client: client}
	expandedReportPath, err := expandPath(parsed.ReportPath)
	if err != nil {
		ui.Errorf("invalid report path: %v", err)
		os.Exit(2)
	}
	if direction == syncUpload {
		if err := syncer.Upload(context.Background(), expandedLocalPath, remotePath, sync.Options{
			DryRun:     parsed.DryRun,
			Delete:     parsed.Delete,
			ReportPath: expandedReportPath,
		}); err != nil {
			ui.Errorf("sync failed: %v", err)
			os.Exit(1)
		}
		return
	}
	if err := syncer.Sync(context.Background(), remotePath, expandedLocalPath, sync.Options{
		DryRun:     parsed.DryRun,
		Delete:     parsed.Delete,
		ReportPath: expandedReportPath,
	}); err != nil {
		ui.Errorf("sync failed: %v", err)
		os.Exit(1)
	}
}

func handleDownload(args []string) {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "download requires <remote-file> and <local-path>")
		os.Exit(2)
	}

	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)
	remotePath := resolvePath(context.Background(), client, fs.Arg(0))
	entry, err := client.GetMeta(context.Background(), remotePath)
	if err != nil {
		ui.Errorf("download failed: %v", err)
		os.Exit(1)
	}
	if entry.Path == "" {
		entry.Path = remotePath
	}
	if entry.Type == model.EntryFolder {
		ui.Errorf("download failed: %s is a directory", entry.Path)
		os.Exit(2)
	}

	dest, err := resolveDownloadDest(remotePath, fs.Arg(1))
	if err != nil {
		ui.Errorf("invalid destination: %v", err)
		os.Exit(2)
	}

	if err := downloadFile(context.Background(), client, entry, dest); err != nil {
		ui.Errorf("download failed: %v", err)
		os.Exit(1)
	}
	ui.Infof("downloaded %s", dest)
}

func handleCompletion(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "completion requires a shell: bash, zsh, or fish")
		os.Exit(2)
	}
	switch args[0] {
	case "bash":
		fmt.Print(bashCompletion)
	case "zsh":
		fmt.Print(zshCompletion)
	case "fish":
		fmt.Print(fishCompletion)
	default:
		fmt.Fprintf(os.Stderr, "unknown shell: %s\n", args[0])
		os.Exit(2)
	}
}

func handleComplete(args []string) {
	if len(args) < 1 {
		return
	}
	token := loadValidToken()
	client := api.NewClient(api.DefaultBaseURL(), token.AccessToken)
	paths, err := remoteCompletions(context.Background(), client, args[0])
	if err != nil {
		return
	}
	for _, item := range paths {
		fmt.Fprintln(os.Stdout, item)
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
	now := time.Now()
	if err := auth.HasValidToken(token, now); err == nil {
		return token
	}

	cfg, err := config.Load()
	if err != nil {
		ui.Errorf("failed to load config: %v", err)
		os.Exit(1)
	}

	refreshed, err := auth.Refresh(context.Background(), cfg, token)
	if err != nil {
		ui.Errorf("token invalid: %v (run: hidrive auth login)", err)
		os.Exit(1)
	}

	if err := auth.SaveToken(refreshed); err != nil {
		ui.Errorf("failed to save refreshed token: %v", err)
		os.Exit(1)
	}

	return refreshed
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

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if path == "~" {
		return os.UserHomeDir()
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

func resolveDownloadDest(remotePath, destArg string) (string, error) {
	if destArg == "" {
		return "", errors.New("destination path is required")
	}
	dest, err := expandPath(destArg)
	if err != nil {
		return "", err
	}
	base := path.Base(remotePath)
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return filepath.Join(dest, base), nil
	}
	if strings.HasSuffix(destArg, string(os.PathSeparator)) {
		return filepath.Join(dest, base), nil
	}
	return dest, nil
}

func downloadFile(ctx context.Context, client *api.Client, entry model.Entry, dest string) error {
	if entry.Path == "" {
		return errors.New("missing remote file path")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tempFile := dest + ".part"
	out, err := os.Create(tempFile)
	if err != nil {
		return err
	}
	if err := client.DownloadFile(ctx, entry.Path, out); err != nil {
		out.Close()
		_ = os.Remove(tempFile)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tempFile)
		return err
	}
	if err := os.Rename(tempFile, dest); err != nil {
		_ = os.Remove(tempFile)
		return err
	}
	if !entry.UpdatedAt.IsZero() {
		_ = os.Chtimes(dest, time.Now(), entry.UpdatedAt)
	}
	return nil
}

func remoteCompletions(ctx context.Context, client *api.Client, prefix string) ([]string, error) {
	if prefix == "" {
		prefix = "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	dir := prefix
	base := ""
	if !strings.HasSuffix(prefix, "/") {
		dir = path.Dir(prefix)
		if dir == "." {
			dir = "/"
		}
		base = path.Base(prefix)
	}
	entries, err := client.ListDir(ctx, dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		candidate := entry.Path
		if candidate == "" {
			candidate = path.Join(dir, entry.Name)
		}
		if base != "" && !strings.HasPrefix(path.Base(candidate), base) {
			continue
		}
		if entry.Type == model.EntryFolder {
			candidate += "/"
		}
		paths = append(paths, candidate)
	}
	return paths, nil
}

type syncArgs struct {
	DryRun     bool
	Delete     bool
	Upload     bool
	ReportPath string
	Positional []string
}

func parseSyncArgs(args []string) (syncArgs, error) {
	parsed := syncArgs{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			parsed.DryRun = true
		case arg == "--delete":
			parsed.Delete = true
		case arg == "--upload":
			parsed.Upload = true
		case arg == "--report":
			if i+1 >= len(args) {
				return parsed, fmt.Errorf("missing value for --report")
			}
			parsed.ReportPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--report="):
			parsed.ReportPath = strings.TrimPrefix(arg, "--report=")
		case strings.HasPrefix(arg, "-"):
			return parsed, fmt.Errorf("unknown flag %s", arg)
		default:
			parsed.Positional = append(parsed.Positional, arg)
		}
	}
	return parsed, nil
}

type syncDirection int

const (
	syncDownload syncDirection = iota
	syncUpload
)

func detectSyncDirection(first, second string) (syncDirection, bool) {
	firstExists := localPathExists(first)
	secondExists := localPathExists(second)
	if firstExists && !secondExists {
		return syncUpload, true
	}
	if secondExists && !firstExists {
		return syncDownload, true
	}
	if firstExists && secondExists {
		return syncDownload, false
	}
	firstLocalHint := isLocalHint(first)
	secondLocalHint := isLocalHint(second)
	if firstLocalHint && !secondLocalHint {
		return syncUpload, true
	}
	if secondLocalHint && !firstLocalHint {
		return syncDownload, true
	}
	firstRemoteHint := strings.HasPrefix(first, "/")
	secondRemoteHint := strings.HasPrefix(second, "/")
	if firstRemoteHint && !secondRemoteHint {
		return syncDownload, true
	}
	if secondRemoteHint && !firstRemoteHint {
		return syncUpload, true
	}
	return syncDownload, false
}

func isLocalHint(path string) bool {
	return strings.HasPrefix(path, "~") || strings.HasPrefix(path, ".")
}

func localPathExists(path string) bool {
	if path == "" {
		return false
	}
	expanded, err := expandPath(path)
	if err != nil {
		return false
	}
	_, err = os.Stat(expanded)
	return err == nil
}

const bashCompletion = `# bash completion for hidrive
_hidrive_complete() {
  local cur cmd prev
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  cmd="${COMP_WORDS[1]}"

  if [[ $COMP_CWORD -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "auth ls sync download whoami completion" -- "$cur") )
    return 0
  fi

  case "$cmd" in
    auth)
      COMPREPLY=( $(compgen -W "login status" -- "$cur") )
      ;;
    completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      ;;
    ls)
      if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "--long --json --recursive" -- "$cur") )
      else
        if [[ -z "$cur" && -n "$prev" ]]; then
          cur="$prev"
        fi
        COMPREPLY=( $(hidrive __complete "$cur" 2>/dev/null) )
      fi
      ;;
    sync)
      if [[ "$cur" == -* ]]; then
        COMPREPLY=( $(compgen -W "--dry-run --delete --report --upload" -- "$cur") )
      elif [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(hidrive __complete "$cur" 2>/dev/null) )
      else
        COMPREPLY=( $(compgen -f -- "$cur") )
      fi
      ;;
    download)
      if [[ $COMP_CWORD -eq 2 ]]; then
        COMPREPLY=( $(hidrive __complete "$cur" 2>/dev/null) )
      else
        COMPREPLY=( $(compgen -f -- "$cur") )
      fi
      ;;
  esac
}
complete -F _hidrive_complete hidrive
`

const zshCompletion = `#compdef hidrive

_hidrive_remote() {
  local cur="$1"
  if [[ -z $cur ]]; then
    cur="$words[CURRENT]"
  fi
  if [[ -z $cur && CURRENT -gt 2 ]]; then
    cur="$words[CURRENT-1]"
  fi
  local -a candidates
  candidates=("${(@f)$(hidrive __complete "$cur" 2>/dev/null)}")
  if (( ${#candidates[@]} )); then
    compadd -Q -a candidates
  else
    _message 'no remote matches'
  fi
}

_hidrive() {
  if (( CURRENT == 2 )); then
    _values 'command' auth ls sync download whoami completion
    return
  fi

  local cmd=$words[2]
  case $cmd in
    auth)
      _values 'auth' login status
      ;;
    completion)
      _values 'shell' bash zsh fish
      ;;
    ls)
      if [[ $words[CURRENT] == -* ]]; then
        _values 'flag' --long --json --recursive
      else
        _hidrive_remote "$words[CURRENT]"
      fi
      ;;
    sync)
      if [[ $words[CURRENT] == -* ]]; then
        _values 'flag' --dry-run --delete --report --upload
      elif (( CURRENT == 3 )); then
        _hidrive_remote "$words[CURRENT]"
      else
        _files
      fi
      ;;
    download)
      if (( CURRENT == 3 )); then
        _hidrive_remote "$words[CURRENT]"
      else
        _files
      fi
      ;;
  esac
}

compdef _hidrive hidrive
`

const fishCompletion = `function __hidrive_remote_paths
  set -l cur (commandline -ct)
  if test -z "$cur"
    set -l tokens (commandline -opc)
    if test (count $tokens) -ge 2
      set cur $tokens[-1]
    end
  end
  hidrive __complete "$cur" 2>/dev/null
end

complete -c hidrive -n 'not __fish_seen_subcommand_from auth ls sync download whoami completion' -a 'auth ls sync download whoami completion'
complete -c hidrive -n '__fish_seen_subcommand_from auth' -a 'login status'
complete -c hidrive -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'

complete -c hidrive -n '__fish_seen_subcommand_from ls; and string match -q -- "--*" (commandline -ct)' -l long -l json -l recursive
complete -c hidrive -n '__fish_seen_subcommand_from sync; and string match -q -- "--*" (commandline -ct)' -l dry-run -l delete -l report -l upload

complete -c hidrive -n '__fish_seen_subcommand_from ls' -a '(__hidrive_remote_paths)'
complete -c hidrive -n '__fish_seen_subcommand_from sync; and test (count (commandline -opc)) -eq 2' -a '(__hidrive_remote_paths)'
complete -c hidrive -n '__fish_seen_subcommand_from download; and test (count (commandline -opc)) -eq 2' -a '(__hidrive_remote_paths)'
`
