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

Sync state:
- `~/.config/hidrive-cli/state.json`

## Planned Commands
Auth:
- `hidrive auth login` – open browser, complete OAuth, store token.
- `hidrive auth status` – show login state and token expiry.

Browse:
- `hidrive ls /` – list items in a remote path.
- `hidrive ls /Photos --long` – include sizes and timestamps.

Sync:
- `hidrive sync /Photos ~/hidrive-sync` – download-only mirror.
- Flags: `--dry-run`, `--report`, `--exclude`, `--include`, `--delete` (optional).

## Sync Safety (MVP)
The HiDrive API exposes hashes for file content and metadata. For the first release we use remote hashes
(`chash`, `mhash`) returned by `GET /dir` or `GET /meta` to detect remote changes without generating local hashes.

Behavior:
- Remote changed + local unchanged since last sync: download.
- Remote changed + local changed: skip + report (avoid overwriting multi-machine edits).
- Remote unchanged: skip.
- Local layout mirrors the remote path under the chosen local root (for example `/Photos` becomes `~/hidrive-sync/Photos`).
- Paths under `/users/<alias>` are treated as the user root, so `/users/<alias>/00_INBOX` becomes `~/hidrive-sync/00_INBOX`.

Reporting:
- `--report /path/to/report.json` writes a JSON summary with counts and conflict paths.

## Development
Go 1.22+ recommended.

Common workflows (once commands exist):
- `go test ./...`
- `go run ./cmd/hidrive ls /`
