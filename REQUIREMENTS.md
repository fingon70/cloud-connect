
# HiDrive CLI Requirements

This document captures the prerequisites and setup needed before developing or using the HiDrive CLI.

## Accounts & Access
- Active Strato HiDrive account with API access enabled.
- HiDrive API credentials from `https://developer.hidrive.com/`.
- Note which auth flow is used (likely OAuth 2.0) and confirm redirect URI(s).

## Local Environment
- Git installed and working.
- Go toolchain installed (recommended: latest stable).
- A local directory chosen as the sync target (for example: `~/hidrive-sync/`).

## Configuration
- A local config file location decided (for example: `~/.config/hidrive-cli/config.json`).
- Environment variables planned for secrets (example: `HIDRIVE_CLIENT_ID`, `HIDRIVE_CLIENT_SECRET`).
- Token storage strategy chosen (encrypted file or OS keychain).

## API Scopes & Permissions
- Confirm required scopes for:
  - Listing remote files/folders.
  - Downloading and uploading.
  - Creating/deleting remote items if sync is bidirectional.
- Record any rate limits or size limits from the API docs.

## Upload API Notes
- Remote folder creation: `POST /dir?path=...`
- File upload: `POST /file?dir=<dir>&name=<filename>` (create, no overwrite)
- File overwrite: `PUT /file?dir=<dir>&name=<filename>` (create or replace)
- Upload body is raw file bytes; use `Content-Type: application/octet-stream` (or a specific MIME type).
- Upload size limit: 2 GB per request body (see HiDrive API docs).

## Sync Behavior
- Define whether sync is:
  - One-way (remote -> local) or bidirectional.
  - Full mirroring or selective paths.
- Decide conflict handling rules (newer wins, keep both, prompt).
- Safety goals:
  - Protect local changes from being overwritten by remote updates.
  - Support cross-device edits (e.g., Windows HiDrive client) while syncing to Linux.
  - Keep sensitive files (Cryptomator vaults, KeePass databases) consistent without forced overwrites.
- Open decisions:
  - Hash-based verification (remote-change detection vs full integrity check).
  - Default conflict policy when local and remote both change.
  - Whether to implement full HiDrive hash algorithm locally or rely on remote hashes for remote->local sync.

## Verification Checklist
- Can authenticate via API and obtain an access token.
- Can list root directory (`/`) and nested paths.
- Can download a single file to the local target.
- Can sync a remote folder to a local root with hash-based change detection.
- Can upload a local file to a remote path.
- Can auto-create missing remote directories during upload.
- Can generate a report JSON and optionally delete local files missing on remote.

## Hash Strategy (MVP)
HiDrive exposes content and metadata hashes that can be requested via `fields` on `GET /dir` or `GET /meta`.
Use those remote hashes to detect changes for remote -> local sync without generating local hashes yet.

Recommended fields for listing:
- `members.path,members.type,members.size,members.mtime,members.chash,members.mhash,members.nhash`

Decision logic:
- If remote `chash` or `mhash` changed since last sync and local file unchanged, download.
- If remote changed and local changed too, skip + report (avoid overwriting multi-machine changes).
- If remote unchanged, skip.

Future considerations:
- For local -> remote, decide whether to implement local hash generation or upload entire files on change.

## Current Implementation Notes
- Sync state stored at `~/.config/hidrive-cli/state.json`.
- `--report` produces a JSON summary with counts plus `conflicts_list` and `errors_list`.
- `--delete` prunes local files and directories that are missing remotely.
- Sync continues on per-file errors and returns a final error if any occurred.
