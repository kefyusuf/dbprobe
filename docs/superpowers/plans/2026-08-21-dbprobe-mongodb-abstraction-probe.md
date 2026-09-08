# dbprobe MongoDB Abstraction Probe

**Goal:** Validate that dbprobe's shared runtime and temporal intelligence can represent non-relational database semantics before stabilizing the adapter SDK.

**Scope:** This is a test-only semantic probe, not a production MongoDB adapter. It deliberately avoids adding a MongoDB driver while dependency access is unavailable.

## What the probe must prove

- Core accepts MongoDB-native capabilities such as `activity.operations`, `schema.collections`, `mongodb.wiredtiger`, `mongodb.replica_set`, and `mongodb.oplog` without SQL/RDBMS branches.
- Object references may use `mongodb.instance` and `mongodb.query_shape`; core does not require table/relation object types.
- Adapter-native deterministic rules execute through the shared finding engine.
- Counter collection/delta sampling works for MongoDB query-shape telemetry.
- Successful inspections persist through the generic temporal `Store` port.
- Cross-snapshot query regression uses caller-supplied metric keys, so the temporal core does not hard-code MySQL latency signals.
- Temporal events preserve MongoDB-native object kinds.

## Implemented test

`test/architecture/mongodb_probe_test.go` defines a private test adapter/runtime with:

```text
engine: mongodb
capabilities:
  activity.operations
  workload.query_summary
  schema.collections
  schema.indexes
  storage.cache
  replication.status
  mongodb.wiredtiger
  mongodb.replica_set
  mongodb.oplog

objects:
  mongodb.instance
  mongodb.query_shape

signals:
  mongodb.wiredtiger.cache_pressure_ratio
  core.query.calls
  mongodb.query.total_latency_ms
  mongodb.query.shape

finding:
  mongodb.wiredtiger_cache_pressure
```

Two inspections are recorded to the in-memory temporal store. The second inspection increases WiredTiger cache pressure and query-shape mean latency. The generic diff service must then produce a MongoDB query regression and `query_regression` event without any engine-specific temporal branch.

## Verification

The semantic probe has been run locally with the dependency-free harness:

```bash
go test ./test/architecture -run TestMongoDBSemanticProbeValidatesNonRelationalArchitecture -v
go test -race ./test/architecture -run TestMongoDBSemanticProbeValidatesNonRelationalArchitecture
```

Both passed.

## SDK stability consequence

Passing this test is evidence that the current contracts are not relational-only, but it is **not sufficient to freeze Adapter Contract v1.0**. A real MongoDB client/runtime must still validate connection lifecycle, privilege/capability discovery, BSON/document metadata handling, query-shape sources, WiredTiger/serverStatus telemetry, and replica-set/oplog behavior. The SDK remains `v0.1` until that production-facing spike and later Cassandra-level distributed semantics validation.
