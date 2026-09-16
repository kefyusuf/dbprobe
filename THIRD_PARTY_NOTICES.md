# Third-Party Notices

`dbprobe` is licensed under the Apache License 2.0. The compiled application
also depends on third-party Go modules that remain subject to their own
licenses and copyright notices.

Official release archives include this notice plus the upstream root-level
license, copying, notice, and copyright materials discovered for every
resolved non-main Go module under a collision-free directory key:

```text
THIRD_PARTY_LICENSES/hex-<lowercase-hex(module-path UTF-8 bytes + NUL + version UTF-8 bytes)>/
```

The release packager appends a **revision-resolved Go module inventory** to
the archived copy of this file from the same `go list -m all` data used to
collect those legal materials. This avoids treating a hand-maintained list as
the authoritative transitive dependency graph.

## Key production dependencies

The following modules are direct production dependencies in the v0.1 release
line. Their exact upstream legal files included in a release archive are
authoritative.

| Module | Version | Upstream / module information |
|---|---|---|
| `github.com/go-sql-driver/mysql` | `v1.10.0` | https://github.com/go-sql-driver/mysql/tree/v1.10.0 |
| `github.com/spf13/cobra` | `v1.10.2` | https://github.com/spf13/cobra/tree/v1.10.2 |
| `modernc.org/sqlite` | `v1.57.0` | https://pkg.go.dev/modernc.org/sqlite@v1.57.0 |

## MPL-covered component

`github.com/go-sql-driver/mysql v1.10.0` is distributed under the Mozilla
Public License 2.0. The corresponding upstream source for that exact version
is available at:

https://github.com/go-sql-driver/mysql/tree/v1.10.0

`dbprobe` does not maintain a modified copy of the upstream MPL-covered module
source in this repository. Official release archives preserve the module's
upstream legal material under `THIRD_PARTY_LICENSES/`.

## Release integrity

The release acceptance contract compares the archived inventory with the
current resolved Go module graph and requires legal material for every
resolved non-main module. The collision-free directory key is derived from the
module path and version together, so distinct module identities cannot alias
the same legal-material directory. A module-graph change therefore cannot
silently produce a release whose archived inventory omits that module.
