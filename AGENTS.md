# Repository Guidelines

## Project Structure & Module Organization
This repository is currently a minimal skeleton. The only tracked file is `LICENSE`, and there is no application source yet. As code is added, keep top-level directories conventional and easy to scan (for example: `src/`, `tests/`, `docs/`, `scripts/`). If you introduce assets, group them under a clear path such as `assets/` or `public/`.

## Build, Test, and Development Commands
Build and test commands:
- `make build` (builds `bin/hidrive`)
- `make test` (runs `go test ./...`)
- `make install-completions` (installs bash/zsh/fish completion scripts)

## Coding Style & Naming Conventions
There are no established style or linting rules yet. When introducing code, standardize on:
- Indentation: 2 or 4 spaces per the chosen language.
- Naming: `snake_case` for files where idiomatic, `PascalCase` for types/classes, and `camelCase` for functions/variables.
- Formatting: add a formatter (for example: Prettier, Black, gofmt) and note the command to run it.

## Testing Guidelines
No test framework is configured yet. When tests are added:
- Use a clear naming convention (`*_test`, `*.spec`, or `*.test` depending on language).
- Keep unit tests near source or in a dedicated `tests/` directory.
- Document how to run the suite and any coverage thresholds.

## Sync Behavior Notes
- Sync supports download (remote -> local) and upload (local -> remote).
- `hidrive sync` auto-detects direction based on path existence and hints; use `--upload` to force local -> remote when ambiguous.
- Uploads use HiDrive `POST /file` for new files and `PUT /file` for overwrites; remote folders are created with `POST /dir`.
- Missing remote directories are created automatically when uploading.
- Local root mirroring: if a local path is inside `hidrive-sync`, or a `.hidrive-sync-root` marker exists in an ancestor, the relative path under that root is mirrored remotely.
- Sync state is stored at `~/.config/hidrive-cli/state.json` and is used to detect conflicts in both directions.

## Commit & Pull Request Guidelines
Git history only includes a single "Initial commit", so no commit convention is established. Adopt a simple, consistent pattern (for example: `feat: add api client` or `fix: handle empty config`) and keep commits scoped.

For pull requests:
- Include a short description of changes and rationale.
- Link related issues if applicable.
- Add screenshots or logs for user-facing or behavioral changes.
- Call out follow-ups or known limitations.

## Security & Configuration Tips
If configuration or secrets are introduced, document expected environment variables and provide a safe example file (for example: `.env.example`). Never commit real secrets.
