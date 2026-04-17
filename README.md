# myapp

`myapp` - devops stack

## Features

- `GET /healthz` for liveness (process only)
- `GET /readyz` for strict readiness (`PostgreSQL SELECT 1` + `Redis PING`)
- `GET /metrics` for prometheus scraping
- `GET /version` for running app version
- `POST /api/notes` creates a note in PostgreSQL
- `GET /api/notes` lists notes with Redis cache
- startup migration for `notes` table

`/healthz` is intentionally process-only and does not check external dependencies.  
`/readyz` checks dependencies and returns `503` if PostgreSQL or Redis is unavailable.

---

## Runtime (Kubernetes)

The application is designed to run in Kubernetes and is deployed via Helm.

It is part of a GitOps-based setup:

- deployed via Argo CD
- exposed via ingress-nginx
- monitored with Prometheus (via `/metrics`)
- autoscaled via HPA

---

## Configuration

copy `.env.example` and set real values.

### Required

- `DATABASE_URL`

### Optional

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

---

## Secrets (Kubernetes)

In Kubernetes, `DATABASE_URL` is not provided via plain environment variables.

Instead it is injected via:

Vault → External Secrets Operator → Kubernetes Secret → myapp

- secrets are stored in Vault
- External Secrets Operator syncs them into Kubernetes
- the application consumes them via `secretKeyRef`

Note: updating a secret requires restarting the pods to take effect.

---

## Local run

```bash
go test ./...
go run ./cmd/myapp
```

---

## Docker

```bash
docker build -t myapp:local .
docker run --rm -p 8080:8080 --env-file .env myapp:local
```

---

## Docker Compose

```bash
docker compose up --build
```

App: http://localhost:8080  
PostgreSQL: localhost:5432  
Redis: localhost:6379

---

## Migrations strategy

current startup migration uses `CREATE TABLE IF NOT EXISTS` which is safe for initial bootstrap.

for schema evolution, move to a dedicated migration tool (for example `golang-migrate`) and run it as a separate Kubernetes init job.
