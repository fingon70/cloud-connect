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

## Sync Behavior
- Define whether sync is:
  - One-way (remote -> local) or bidirectional.
  - Full mirroring or selective paths.
- Decide conflict handling rules (newer wins, keep both, prompt).

## Verification Checklist
- Can authenticate via API and obtain an access token.
- Can list root directory (`/`) and nested paths.
- Can download a single file to the local target.
