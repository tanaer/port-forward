# Repository Guidelines

## Project Structure & Module Organization
- `main.go` coordinates config loading and spins up forwarders via `forward.Run`.
- Runtime settings and shared primitives belong in `conf/`; keep new globals here.
- Tunnelling logic sits in `forward/`, persistence in `sql/sql.go` (writes `goForward.db`), and the admin UI lives in `web/` with templates under `assets/templates/`.
- Reusable helpers go in `utils/utils.go`; favour focused functions over copy-paste.

## Build, Test, and Development Commands
- `go build -o goForward .` produces the CLI binary used in deployments.
- `go run . -port 8899 -pass 123456` spins up the dashboard locally with explicit credentials.
- `go test ./...` executes package tests; narrow to `./forward` or `./conf` while iterating.

## Coding Style & Naming Conventions
- Run `gofmt -w` (tabs, newline at EOF) and `goimports` before pushing.
- Exported APIs use PascalCase with short doc comments; keep internals camelCase and package-scoped.
- Keep logs terse (`[component] event`) and prefer early returns to nested branching.

## Testing Guidelines
- Co-locate `_test.go` files with the code under test and use table-driven cases for different port scenarios.
- Bind mock listeners to `127.0.0.1:0`, assert on `forward.ConnectionStats`, and document any concurrency flakiness in the PR.

## Commit & Pull Request Guidelines
- Match the existing imperative subjects (`Update forward.go`) while adding context when relevant (`Adjust UDP idle timeout`).
- PRs should list behavior changes, reference `go build`/`go test` output, and link issues or screenshots for UI or API shifts.
- Flag config defaults or schema updates in both the description and `readme.md` when they affect deployments.

## Security & Configuration Tips
- Do not commit production copies of `goForward.db`; generate fixtures through helpers in `sql/`.
- Pass dashboard secrets via flags or environment variables and scrub them from logs before sharing debug output.
