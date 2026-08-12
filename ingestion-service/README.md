# ingestion-service

High-throughput HTTP surface for job heartbeats. Runs independently of the
Django core-api — it shares only the Postgres schema Django's migrations
own (`jobs_job`, `jobs_heartbeat`), reading/writing it directly with `pgx`
for speed rather than going through the REST API.

## Endpoints

### `POST /v1/heartbeat`

```json
{
  "job": "nightly-etl",
  "status": "success",
  "duration_ms": 4231,
  "metadata": { "rows_processed": 18234 }
}
```

- `job` — the job's name or UUID.
- `status` — `"success"` (default) or `"failure"`.
- `duration_ms`, `metadata` — optional.

Writes the heartbeat row and the job's denormalized `last_heartbeat_at` /
`last_heartbeat_status` in one transaction, then caches the result in Redis.
Returns `404` if the job isn't registered in core-api yet.

### `GET /v1/jobs/:job/heartbeat/latest`

Redis-first read of a job's last heartbeat (`X-Cache: HIT`/`MISS` header
shows whether it came from cache or was a cold response).

### `GET /healthz`

Checks both the Postgres pool and Redis connection.

### `GET /metrics`

Prometheus metrics: `pipelineops_heartbeats_total{status}`,
`pipelineops_heartbeats_rejected_total{reason}`,
`pipelineops_heartbeat_ingest_duration_seconds`.

## Running locally

```bash
go run ./cmd/server
```

Reads `DATABASE_URL`, `REDIS_URL`, `PORT` (default `8080`), and `GIN_MODE`
from the environment — see `docker-compose.yml` at the repo root for the
values used there.

## Tests / checks

```bash
go vet ./...
go build ./...
```
