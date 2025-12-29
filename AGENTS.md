# Repository Guidelines

## Project Structure & Module Organization
This repository is currently a minimal skeleton. The only tracked file is `LICENSE`, and there is no application source yet. As code is added, keep top-level directories conventional and easy to scan (for example: `src/`, `tests/`, `docs/`, `scripts/`). If you introduce assets, group them under a clear path such as `assets/` or `public/`.

## Build, Test, and Development Commands
No build, test, or run commands are defined in the repository at this time. When you add tooling, document the exact commands here (for example: `npm run build`, `make test`) and keep them in sync with any package scripts or Makefile targets.

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

## Commit & Pull Request Guidelines
Git history only includes a single "Initial commit", so no commit convention is established. Adopt a simple, consistent pattern (for example: `feat: add api client` or `fix: handle empty config`) and keep commits scoped.

For pull requests:
- Include a short description of changes and rationale.
- Link related issues if applicable.
- Add screenshots or logs for user-facing or behavioral changes.
- Call out follow-ups or known limitations.

## Security & Configuration Tips
If configuration or secrets are introduced, document expected environment variables and provide a safe example file (for example: `.env.example`). Never commit real secrets.
