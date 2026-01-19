# HiDrive CLI

Command-line tool to browse and sync Strato HiDrive files to a local folder.

## Status
MVP: OAuth login, `ls`, download, and bidirectional sync (download + upload).

## Architecture Overview
- `cmd/hidrive`: CLI entrypoint.
- `internal/auth`: OAuth 2.0 flow and token persistence.
- `internal/config`: config file and env overrides.
- `internal/api`: HiDrive REST client and pagination.
- `internal/sync`: download + upload mirroring logic.
- `internal/model`: shared types and errors.

## Configuration
Environment variables (preferred for secrets):
- `HIDRIVE_CLIENT_ID`
- `HIDRIVE_CLIENT_SECRET`
- `HIDRIVE_REDIRECT_URI` (default: `http://localhost:8888/callback`)
- `HIDRIVE_SCOPES` (default: `rw`)

Config file path:
- `~/.config/hidrive-cli/config.json`

Token storage:
- `~/.config/hidrive-cli/token.json`

Sync state:
- `~/.config/hidrive-cli/state.json`

## Commands
Auth:
- `hidrive auth login` – open browser, complete OAuth, store token.
- `hidrive auth status` – show login state and token expiry.

Browse:
- `hidrive ls /` – list items in a remote path.
- `hidrive ls /Photos --long` – include sizes and timestamps.

Download:
- `hidrive download /Photos/shot.jpg ~/Downloads/shot.jpg` – download a single file.

Sync:
- Download: `hidrive sync /Photos ~/hidrive-sync`
- Upload: `hidrive sync ~/hidrive-sync/Photos /users/<alias>/Photos`
- Flags: `--dry-run`, `--delete`, `--report`, `--upload`

## Sync Safety (MVP)
The HiDrive API exposes hashes for file content and metadata. For the first release we use remote hashes
(`chash`, `mhash`) returned by `GET /dir` or `GET /meta` to detect remote changes without generating local hashes.

Behavior:
- Remote changed + local unchanged since last sync: download.
- Remote changed + local changed: skip + report (avoid overwriting multi-machine edits).
- Remote unchanged: skip.
- Local changed + remote unchanged: upload.
- Local changed + remote changed: skip + report (avoid overwriting remote changes).
- Local layout mirrors the remote path under the chosen local root (for example `/Photos` becomes `~/hidrive-sync/Photos`).
- Paths under `/users/<alias>` are treated as the user root, so `/users/<alias>/00_INBOX` becomes `~/hidrive-sync/00_INBOX`.
- `--delete` removes local files and directories not present in the remote listing.
- Sync continues on per-file errors and reports them in the summary/report.

Reporting:
- `--report /path/to/report.json` writes a JSON summary with counts and conflict paths.
  - Includes `deleted` and `errors` counts plus `conflicts_list` and `errors_list` arrays when relevant.

## Development
Go 1.22+ recommended.

Build from repo:
- `make build` (writes `bin/hidrive`)
- `make test` (runs `go test ./...`)
- `go run ./cmd/hidrive ls /` (run without building)

## Usage Examples
- Authenticate: `hidrive auth login`
- List a folder: `hidrive ls /users/<alias>/Documents`
- Sync a remote folder to a local directory: `hidrive sync /users/<alias> ~/HiDrive`
- Download a single file: `hidrive download /users/<alias>/Documents/report.pdf ~/HiDrive/report.pdf`
- Upload a local folder to remote: `hidrive sync ~/HiDrive/Projects /users/<alias>/Projects`

## Shell Completion
Generate completion scripts:
- Bash: `hidrive completion bash`
- Zsh: `hidrive completion zsh`
- Fish: `hidrive completion fish`

Manual setup:
- Bash:
  - `mkdir -p ~/.local/share/bash-completion/completions`
  - `hidrive completion bash > ~/.local/share/bash-completion/completions/hidrive`
  - Ensure bash completion is enabled (commonly `[[ -r /usr/share/bash-completion/bash_completion ]] && . /usr/share/bash-completion/bash_completion`).
- Zsh:
  - `mkdir -p ~/.zsh/completions`
  - `hidrive completion zsh > ~/.zsh/completions/_hidrive`
  - Add `fpath=(~/.zsh/completions $fpath)` and `autoload -U compinit && compinit` to `~/.zshrc`.
- Fish:
  - `mkdir -p ~/.config/fish/completions`
  - `hidrive completion fish > ~/.config/fish/completions/hidrive.fish`

Installer note:
- If you package/install the tool, drop the generated files into the standard completion directories above so users do not need to edit shell config manually.

Helper script:
- `sh scripts/install-completions.sh` installs bash, zsh, and fish completions into the standard user locations.

Makefile:
- `make install-completions` runs the helper script.
- `make build` builds `bin/hidrive`.
- `make test` runs the test suite.

## Current State (Remote -> Local)
- Sync uses HiDrive `chash`/`mhash` to detect remote changes; no local hash generation yet.
- Sync state stored at `~/.config/hidrive-cli/state.json`.
- Paths under `/users/<alias>` map to the local root (for example `/users/<alias>/00_INBOX` -> `~/hidrive-sync/00_INBOX`).
- `--report` writes a JSON summary including conflicts and errors.
- `--delete` removes local entries missing on the remote path.
- Syncing a single file path downloads it to the local root using the remote file name.

## Current State (Local -> Remote)
- Upload sync detects local changes based on size/mtime and avoids overwriting remote changes.
- Missing remote folders are created automatically before uploads.
- Use `hidrive sync <local-path> <remote-path>` or force with `--upload` if the direction is ambiguous.
- If the local path is inside a folder named `hidrive-sync`, the relative path under that folder is mirrored remotely.
- Optional marker: place `.hidrive-sync-root` in a local root folder to force the same mirroring behavior.
