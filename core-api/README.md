# core-api

Django + Django REST Framework API for PipelineOps: job/alert-rule CRUD,
auth, the Django admin, and the Celery worker/beat that detect missed
heartbeats and fire alerts.

## Endpoints

| Method | Path | |
|---|---|---|
| POST | `/api/auth/token/` | Obtain an auth token (`{username, password}` → `{token}`) |
| GET/POST | `/api/jobs/` | List / create jobs |
| GET/PATCH/DELETE | `/api/jobs/:id/` | Job detail (includes recent heartbeats), update, delete |
| GET | `/api/heartbeats/?job=:id` | Heartbeat history (read-only — written by `ingestion-service`) |
| GET/POST | `/api/alert-rules/` | List / create alert rules |
| DELETE | `/api/alert-rules/:id/` | Remove a rule |
| GET | `/api/alert-events/?job=:id&status=open` | Alert history |
| GET | `/healthz` | Liveness check |
| — | `/admin/` | Django admin |

All `/api/` endpoints (except token auth) require `Authorization: Token <token>`.

## Alerting

`alerts.tasks.check_missed_heartbeats` runs on a Celery beat schedule
(`HEARTBEAT_CHECK_INTERVAL_SECONDS`, default 30s). For each active job whose
`computed_status` is `late` or `failed`, it opens an `AlertEvent` (skipping
jobs that already have one open, so a single outage doesn't spam) and
dispatches through the job's `AlertRule`s — Slack, email, or SMS — falling
back to Slack if no rule is configured. Alerts auto-resolve once heartbeats
resume.

## Running locally

```bash
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
cp .env.example .env   # then point DATABASE_URL at a running Postgres
python manage.py migrate
python manage.py runserver
```

In another shell:

```bash
celery -A pipelineops worker -l info
celery -A pipelineops beat -l info
```

Normally you'd just use `docker compose up` from the repo root instead of
running these by hand.

## Checks

```bash
python manage.py check
python manage.py makemigrations --check --dry-run
```
