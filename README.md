# myapp

`myapp` is a small REST API service for a DevOps pet project stack.

## Features

- `GET /healthz` for liveness
- `GET /readyz` for strict readiness (`PostgreSQL SELECT 1` + `Redis PING`)
- `GET /metrics` for Prometheus scraping
- `GET /version` for running app version
- `POST /api/notes` creates a note in PostgreSQL
- `GET /api/notes` lists notes with Redis cache
- startup migration for `notes` table

`/healthz` is intentionally process-only and does not check external dependencies.
`/readyz` checks dependencies and returns `503` if PostgreSQL or Redis is unavailable.

## Configuration

Copy `.env.example` and set real values.

Required:

- `DATABASE_URL`

Optional:

- `HTTP_PORT` (default `8080`)
- `REDIS_ADDR` (default `localhost:6379`)
- `REDIS_PASSWORD` (optional)
- `REDIS_DB` (default `0`)
- `DB_TIMEOUT` (default `2s`)
- `REDIS_TIMEOUT` (default `2s`)
- `CACHE_TTL` (default `45s`)
- `LOG_LEVEL` (`debug|info|warn|error`, default `info`)
- `SHUTDOWN_TIMEOUT` (default `10s`)
- `DB_MAX_CONNS` (default `20`)
- `DB_MAX_IDLE` (default `5`)
- `STARTUP_RETRIES` (default `10`)
- `STARTUP_RETRY_GAP` (default `2s`)

## Local run

```bash
go test ./...
go run ./cmd/myapp
```

## Docker

```bash
docker build -t myapp:local .
docker run --rm -p 8080:8080 --env-file .env myapp:local
```

## Docker Compose

```bash
docker compose up --build
```

App: `http://localhost:8080`  
PostgreSQL: `localhost:5432`  
Redis: `localhost:6379`

## Kubernetes probes (recommended)

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  periodSeconds: 10
  timeoutSeconds: 1

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  periodSeconds: 5
  timeoutSeconds: 1

startupProbe:
  httpGet:
    path: /readyz
    port: 8080
  failureThreshold: 30
  periodSeconds: 2
  timeoutSeconds: 1
```

## Migrations strategy

Current startup migration uses `CREATE TABLE IF NOT EXISTS`, which is safe for initial bootstrap.
For schema evolution, move to a dedicated migration tool (for example `golang-migrate`) and run it as a separate Kubernetes init job.
