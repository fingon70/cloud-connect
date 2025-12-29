# HiDrive CLI

Command-line tool to browse and sync Strato HiDrive files to a local folder.

## Status
Planned: initial MVP with OAuth login, `ls`, and download-only `sync`.

## Architecture Overview
- `cmd/hidrive`: CLI entrypoint.
- `internal/auth`: OAuth 2.0 flow and token persistence.
- `internal/config`: config file and env overrides.
- `internal/api`: HiDrive REST client and pagination.
- `internal/sync`: download-only mirroring logic.
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

## Planned Commands
Auth:
- `hidrive auth login` – open browser, complete OAuth, store token.
- `hidrive auth status` – show login state and token expiry.

Browse:
- `hidrive ls /` – list items in a remote path.
- `hidrive ls /Photos --long` – include sizes and timestamps.

Sync:
- `hidrive sync /Photos ~/hidrive-sync` – download-only mirror.
- Flags: `--dry-run`, `--exclude`, `--include`, `--delete` (optional).

## Development
Go 1.22+ recommended.

Common workflows (once commands exist):
- `go test ./...`
- `go run ./cmd/hidrive ls /`
