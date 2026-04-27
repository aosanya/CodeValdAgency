# CodeValdAgency — Configuration

> Part of the split architecture. Index: [architecture.md](architecture.md)

Runtime configuration is loaded by [`internal/config/config.go`](../../internal/config/config.go)
from environment variables. There is no YAML file — every setting is an env
var, with sensible defaults for everything except `CODEVALDAGENCY_GRPC_PORT`
(which is required and fails fast on startup).

---

## 1. Environment Variables

| Name | Required | Default | Purpose |
|---|---|---|---|
| `CODEVALDAGENCY_GRPC_PORT` | ✅ yes | — | Port the gRPC server listens on. Service exits at startup if unset. |
| `CODEVALDAGENCY_AGENCY_ID` | ⚠️ effectively required | `""` | The single agency ID injected at startup. Sent in every Cross `Register` heartbeat and used as the scope key on every `entitygraph.DataManager` call. Empty string is accepted at startup but every CRUD call will resolve to a no-data scope. |
| `AGENCY_ARANGO_ENDPOINT` | no | `http://localhost:8529` | ArangoDB HTTP endpoint. |
| `AGENCY_ARANGO_USER` | no | `root` | ArangoDB username. |
| `AGENCY_ARANGO_PASSWORD` | no | `""` | ArangoDB password. |
| `AGENCY_ARANGO_DATABASE` | no | `codevaldagency` | ArangoDB database name. The collections (`agency_entities`, `agency_relationships`, `agency_schemas`, `agency_snapshots`, `agency_publications`) live inside this database — see [architecture-storage.md](architecture-storage.md). |
| `CROSS_GRPC_ADDR` | no | `""` | CodeValdCross gRPC address for registration heartbeats. **Empty string disables registration entirely** — the service runs standalone. |
| `AGENCY_GRPC_ADVERTISE_ADDR` | no | `:${CODEVALDAGENCY_GRPC_PORT}` | Address that CodeValdCross dials back on. Override when running behind a NAT or a sidecar. |
| `CROSS_PING_INTERVAL` | no | `20s` | Heartbeat cadence to CodeValdCross. Parsed as a Go `time.Duration`. |
| `CROSS_PING_TIMEOUT` | no | `5s` | Per-RPC timeout for each `Register` call. |

---

## 2. Integration-Test-Only Variables

These are read directly by test files, not by the production binary:

| Name | Read by | Purpose |
|---|---|---|
| `AGENCY_ARANGO_ENDPOINT` | [`storage/arangodb/storage_test.go`](../../storage/arangodb/storage_test.go) | When unset, ArangoDB-backed integration tests are **skipped** rather than failing. The test harness does not start its own database. |

`AGENCY_ARANGO_USER`, `AGENCY_ARANGO_PASSWORD`, and `AGENCY_ARANGO_DATABASE`
are reused with the same defaults as the production binary.

---

## 3. Boot Sequence

`cmd/main.go` does the following in order:

1. `config.Load()` — reads env vars, panics if `CODEVALDAGENCY_GRPC_PORT` is missing.
2. `arangodb.NewBackend(cfg)` — connects to ArangoDB and ensures every required
   collection + the named graph exist. This is idempotent and safe to call on
   every boot.
3. `entitygraph.SeedSchema(ctx, schemaManager, schema.DefaultAgencySchema())` —
   writes the pre-delivered `Schema` if the current version is missing.
   Idempotent; safe to call on every boot.
4. `codevaldagency.NewAgencyManager(dm, sm, publisher, agencyID)` — constructs
   the manager facade.
5. `registrar.New(cfg, ...)` and `Run(ctx)` — starts the Cross heartbeat loop.
   When `CROSS_GRPC_ADDR == ""` the loop is a no-op.
6. `grpc.Serve` on `CODEVALDAGENCY_GRPC_PORT`.

Failures at step 1 or 2 are fatal (the service cannot start). Failures at step
5 are non-fatal — the service runs without Cross registration so a Cross
outage does not take agency CRUD offline.

---

## 4. Local Development

Minimum viable `.env` to run the service against a local ArangoDB:

```bash
CODEVALDAGENCY_GRPC_PORT=50061
CODEVALDAGENCY_AGENCY_ID=agency-local
AGENCY_ARANGO_ENDPOINT=http://localhost:8529
AGENCY_ARANGO_PASSWORD=
# CROSS_GRPC_ADDR omitted → registration disabled
```

To run integration tests against the same database:

```bash
export AGENCY_ARANGO_ENDPOINT=http://localhost:8529
go test -race ./...
```

When `AGENCY_ARANGO_ENDPOINT` is unset, `storage/arangodb/storage_test.go` and
the gRPC integration tests in `internal/server/integration_test.go` will skip
themselves cleanly — `go test ./...` still exits 0.
