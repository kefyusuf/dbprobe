# SQLite Driver Selection Benchmark — 2026-09-02

## Purpose

This benchmark records the evidence used to choose the default CGo-free SQLite driver for dbprobe local temporal history. It compares `modernc.org/sqlite v1.57.0` and `github.com/ncruces/go-sqlite3 v0.35.3` through the same dbprobe `ConnectorFactory`, migration, codec, store and reopen path.

The benchmark is a selection aid, not a general SQLite performance claim or service-level objective.

## Reproduction

```bash
make test-sqlite-drivers
make cross-build-sqlite-drivers
RUNS=7 SNAPSHOTS=250 make compare-sqlite-drivers
```

Implementation:

- `test/acceptance/sqlite-drivers/probe/probe.go`
- `test/acceptance/sqlite-drivers/compare.sh`
- isolated nested module under `test/acceptance/sqlite-drivers`

The nested module keeps ncruces out of the production dependency graph.

## Environment

- GitHub-hosted runner: Ubuntu 24.04.4, `linux/amd64`
- Go: `go1.25.14`
- `CGO_ENABLED=0`
- `GOMAXPROCS=2`
- acceptance commit: `4c633a38bb38bb44c29c0acb57966c56b5091736`
- workflow run: `33646041100`
- benchmark artifact: `sqlite-driver-comparison`, artifact ID `9852885941`

## Workload

Each candidate receives the same workload:

- one fresh database and migration per run;
- 250 snapshots;
- 32 numeric observations per snapshot;
- 16 sampled deltas per snapshot;
- transactional snapshot and trend-row writes;
- duplicate snapshot idempotency check;
- close and reopen;
- `Latest`, `Previous` and unbounded `List` reads;
- conflicting same-ID payload rejection;
- seven measured runs after warm-up;
- alternating candidate order between runs.

Both candidates produced the same 5,455,872-byte SQLite database for the workload.

## Correctness and portability

Both candidates passed:

- the same dbprobe persistence contract test;
- create/migrate/save/close/reopen behavior;
- duplicate idempotency and conflicting-payload rejection;
- current-schema reads through `Latest`, `Previous` and `List`;
- CGo-free builds for:
  - Linux amd64;
  - Windows amd64;
  - macOS amd64;
  - macOS arm64.

The production modernc binding additionally passed:

- `PRAGMA foreign_keys = 1`;
- `PRAGMA busy_timeout = 5000`;
- `PRAGMA user_version = 1`;
- transaction rollback after a forced trend-row insert failure;
- foreign-key cascade cleanup;
- private database-file permissions where supported;
- default CLI `inspect -> inspect -> diff` persistence;
- full normal and race test suites.

## Results

Values are medians of seven measured runs. Binary sizes use stripped, CGo-free comparison binaries built with `-trimpath -ldflags='-s -w'`.

| Metric | modernc v1.57.0 | ncruces v0.35.3 | Interpretation |
|---|---:|---:|---|
| Comparison binary | 6,693,048 B / 6.38 MiB | 9,121,976 B / 8.70 MiB | modernc is 2.32 MiB, or 26.6%, smaller |
| Full process elapsed | 1,060.421 ms | 940.012 ms | modernc is 120.409 ms slower across 250 snapshots |
| Open + migrate | 2.519 ms | 2.522 ms | effectively identical |
| Write 250 snapshots | 1,001.647 ms | 885.127 ms | modernc is 13.2% slower |
| Write per snapshot | 4.007 ms | 3.541 ms | modernc adds 0.466 ms per snapshot |
| Reopen + read checks | 46.636 ms | 38.676 ms | modernc is 7.960 ms slower |
| Internal total | 1,050.225 ms | 929.470 ms | modernc is 13.0% slower |

## Decision interpretation

ncruces is faster on this bounded write-heavy microbenchmark. The difference is measurable but small in absolute dbprobe terms: the median write difference is approximately 0.47 ms per persisted snapshot. A real inspection is dominated by opening and querying the target database, capability probing and counter sampling rather than by this local append.

modernc is selected because:

1. both drivers satisfy correctness and CGo-free platform requirements;
2. modernc is a stable v1 module with the stronger maturity and adoption signal considered in ADR-013;
3. the comparison binary is 2.32 MiB smaller;
4. open/migrate latency is effectively equal;
5. the observed persistence penalty is below one millisecond per snapshot and does not justify choosing the pre-v1 fallback for the default distribution;
6. the current `NewConnector` API maps directly onto dbprobe's existing `sql.OpenDB` composition boundary.

`github.com/ncruces/go-sqlite3` remains a validated fallback. Reconsider the decision if modernc/libc compatibility becomes operationally difficult, if distribution size reverses materially, or if production profiles show local history persistence becoming a meaningful share of end-to-end inspection latency.

## Limitations

- GitHub-hosted runners are noisy shared infrastructure.
- The benchmark measures one Linux filesystem and does not establish Windows or macOS runtime performance.
- Candidate order is alternated and warm-up is applied, but the test is not a full statistical performance study.
- The comparison binaries isolate driver footprint; they are not identical to the final dbprobe release binary.
- Results should be rerun when upgrading either driver or changing the SQLite schema, transaction strategy, codec or trend extraction volume.
