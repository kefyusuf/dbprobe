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
- acceptance commit: `6cd8ad9975439886ecbf25389d810f0993f371a3`
- workflow run: `33647790603`
- benchmark artifact: `sqlite-driver-comparison`, artifact ID `9853550603`
- artifact ZIP SHA-256: `cc15b3ac7896923009afabe310382c0b8ebda4e6cfd5aec2efaa32d6ca018f68`

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

## Timing boundaries

The corrected comparison separates normal performance phases from correctness probes:

- `write_ns` measures exactly the configured 250 normal snapshot saves;
- `duplicate_check_ns` measures the additional idempotent duplicate save separately;
- `initial_close_ns` measures the first owned-store close separately;
- `reopen_read_ns` covers reopen plus `Latest`, `Previous` and `List`, and stops before the conflicting write;
- `conflict_check_ns` measures conflicting same-ID rejection separately;
- `reopen_close_ns` measures the final close separately;
- `process_elapsed_ns` is measured outside the candidate process and includes process startup and all phases;
- `total_ns` is the candidate's internal end-to-end measurement.

This separation prevents duplicate lookup, collision rollback and close costs from being misreported as normal snapshot-write or reopen/read latency.

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
- graceful inspect degradation when history cannot resolve, open or close;
- full normal and race test suites.

## Results

Values are medians of seven measured runs. Binary sizes use stripped, CGo-free comparison binaries built with `-trimpath -ldflags='-s -w'`.

| Metric | modernc v1.57.0 | ncruces v0.35.3 | Interpretation |
|---|---:|---:|---|
| Comparison binary | 6,693,048 B / 6.383 MiB | 9,121,976 B / 8.699 MiB | modernc is 2.316 MiB, or 26.6%, smaller |
| Full process elapsed | 850.364 ms | 816.795 ms | modernc is 33.569 ms, or 4.1%, slower |
| Open + migrate | 2.636 ms | 2.574 ms | 0.062 ms difference |
| Write 250 normal snapshots | 793.949 ms | 774.863 ms | modernc is 19.087 ms, or 2.5%, slower |
| Write per normal snapshot | 3.176 ms | 3.099 ms | modernc adds 0.076 ms per snapshot |
| Duplicate-idempotency check | 0.249 ms | 0.215 ms | both remain sub-millisecond |
| Initial close | 0.748 ms | 0.259 ms | both remain sub-millisecond |
| Reopen + read checks | 34.336 ms | 30.556 ms | modernc is 3.780 ms slower |
| Conflicting-payload check | 0.216 ms | 0.183 ms | both remain sub-millisecond |
| Reopened-store close | 0.253 ms | 0.101 ms | both remain sub-millisecond |
| Internal total | 841.487 ms | 808.579 ms | modernc is 32.908 ms, or 4.1%, slower |

## Decision interpretation

ncruces is faster on this bounded write-heavy microbenchmark, but the corrected phase boundaries show a small absolute difference for dbprobe's actual persistence unit:

- normal writes differ by approximately 0.076 ms per persisted snapshot;
- full process elapsed differs by approximately 33.6 ms across 250 snapshots;
- open/migrate differs by approximately 0.062 ms;
- both correctness probes and close phases remain sub-millisecond;
- modernc produces a 2.316 MiB smaller comparison binary.

A normal dbprobe inspection is dominated by opening and querying the target database, capability discovery, collector execution and counter sampling rather than by one local snapshot append. The observed persistence difference is therefore non-material for the default CLI.

modernc is selected because:

1. both drivers satisfy correctness and CGo-free platform requirements;
2. modernc is a stable v1 module with the stronger maturity and adoption signal considered in ADR-013;
3. the comparison binary is 26.6% smaller;
4. open/migrate latency is effectively equivalent for this workload;
5. the corrected normal-write penalty is approximately 0.076 ms per snapshot;
6. the current `NewConnector` API maps directly onto dbprobe's existing `sql.OpenDB` composition boundary.

`github.com/ncruces/go-sqlite3` remains a validated fallback. Reconsider the decision if modernc/libc compatibility becomes operationally difficult, if distribution size reverses materially, or if production profiles show local history persistence becoming a meaningful share of end-to-end inspection latency.

## Limitations

- GitHub-hosted runners are noisy shared infrastructure; several individual runs contained visible scheduler or filesystem outliers.
- Medians and alternating candidate order reduce, but do not eliminate, shared-runner noise.
- The benchmark measures one Linux filesystem and does not establish Windows or macOS runtime performance.
- Cross-build success proves compilation portability, not native runtime behavior on every target OS.
- The comparison binaries isolate driver footprint; they are not identical to the final dbprobe release binary.
- Results should be rerun when upgrading either driver or changing the SQLite schema, transaction strategy, codec or trend extraction volume.
