# Third-Party Notices

`dbprobe` is licensed under the Apache License 2.0. The compiled application
also depends on third-party Go modules that remain subject to their own
licenses and copyright notices.

The table below records the production module graph resolved by `go.mod` for
the v0.1 release line. SPDX identifiers are provided as a navigation aid; the
upstream license files included in official release archives are authoritative.
The release packager copies root-level upstream license, copying, notice, and
copyright files from every resolved non-main Go module into:

```text
THIRD_PARTY_LICENSES/<module-path-with-slashes-replaced-by-__>@<version>/
```

| Module | Version | License | Upstream / module information |
|---|---|---|---|
| `github.com/go-sql-driver/mysql` | `v1.10.0` | MPL-2.0 | https://github.com/go-sql-driver/mysql/tree/v1.10.0 |
| `github.com/spf13/cobra` | `v1.10.2` | Apache-2.0 | https://github.com/spf13/cobra/tree/v1.10.2 |
| `modernc.org/sqlite` | `v1.57.0` | BSD-3-Clause | https://pkg.go.dev/modernc.org/sqlite@v1.57.0 |
| `filippo.io/edwards25519` | `v1.2.0` | BSD-3-Clause | https://pkg.go.dev/filippo.io/edwards25519@v1.2.0 |
| `github.com/dustin/go-humanize` | `v1.0.1` | MIT | https://github.com/dustin/go-humanize/tree/v1.0.1 |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause | https://github.com/google/uuid/tree/v1.6.0 |
| `github.com/inconshreveable/mousetrap` | `v1.1.0` | Apache-2.0 | https://github.com/inconshreveable/mousetrap/tree/v1.1.0 |
| `github.com/mattn/go-isatty` | `v0.0.24` | MIT | https://github.com/mattn/go-isatty/tree/v0.0.24 |
| `github.com/ncruces/go-strftime` | `v1.0.0` | MIT | https://github.com/ncruces/go-strftime/tree/v1.0.0 |
| `github.com/remyoudompheng/bigfft` | `v0.0.0-20230129092748-24d4a6f8daec` | BSD-3-Clause | https://pkg.go.dev/github.com/remyoudompheng/bigfft@v0.0.0-20230129092748-24d4a6f8daec |
| `github.com/spf13/pflag` | `v1.0.9` | BSD-3-Clause | https://github.com/spf13/pflag/tree/v1.0.9 |
| `golang.org/x/sys` | `v0.47.0` | BSD-3-Clause | https://pkg.go.dev/golang.org/x/sys@v0.47.0 |
| `modernc.org/libc` | `v1.74.4` | BSD-3-Clause | https://pkg.go.dev/modernc.org/libc@v1.74.4 |
| `modernc.org/mathutil` | `v1.7.1` | BSD-3-Clause | https://pkg.go.dev/modernc.org/mathutil@v1.7.1 |
| `modernc.org/memory` | `v1.11.0` | BSD-3-Clause | https://pkg.go.dev/modernc.org/memory@v1.11.0 |

## MPL-covered component

`github.com/go-sql-driver/mysql v1.10.0` is distributed under the Mozilla
Public License 2.0. The corresponding upstream source for that exact version
is available at:

https://github.com/go-sql-driver/mysql/tree/v1.10.0

`dbprobe` does not maintain a modified copy of the upstream MPL-covered module
source in this repository. Official release archives preserve the module's
upstream license material under `THIRD_PARTY_LICENSES/`.

## Release integrity

The release acceptance contract compares this inventory with the current Go
module graph and requires legal material for every resolved non-main module.
If the module graph changes, the notice and packaging contract must be updated
before release CI can pass.
