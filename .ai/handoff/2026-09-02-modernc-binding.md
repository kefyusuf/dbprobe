# dbprobe modernc SQLite binding handoff — 2026-09-02

## Scope

This branch binds the existing driver-independent SQLite store and history-aware CLI orchestration to `modernc.org/sqlite` at the CLI composition root.

## Required evidence

The branch must not be integrated unless all of the following pass on the final tree:

- exact module pin and stable `go mod tidy`;
- live on-disk snapshot save, close, reopen, `Latest`, `Previous`, and `List`;
- duplicate-save idempotency and conflicting same-ID payload rejection;
- production `inspect -> inspect -> diff` command flow;
- repository-wide formatting, vet, normal tests, race tests, build, and smoke tests under Go 1.25;
- CGo-free Linux amd64, Windows amd64, macOS amd64, and macOS arm64 builds;
- MySQL 8.0.46 and MySQL 8.4.11 Docker acceptance.

## Boundary

The concrete driver import is permitted only in `cmd/dbprobe/sqlite_history.go`. `internal/platform/sqlite`, application services, core packages, and SDK contracts remain driver-independent.

This handoff records the required gate; it is not itself evidence that the gate passed. The authoritative result is the final GitHub Actions run and the integration merge produced only after that run succeeds.
