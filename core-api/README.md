# core-api

I built this as the Django + Django REST Framework API for PipelineOps:
job/alert-rule CRUD, auth, the Django admin, and the Celery worker/beat
that detects missed heartbeats and fires alerts.

## Endpoints

| Method | Path | |
|---|---|---|
| GET | `/api/auth/me/` | Current session's user, `401` if not logged in. Also primes the `csrftoken` cookie. |
| POST | `/api/auth/login/` | `{username, password}` → `{username}`, sets an httpOnly session cookie |
| POST | `/api/auth/logout/` | Clears the session |
| GET/POST | `/api/jobs/` | List / create jobs |
| GET/PATCH/DELETE | `/api/jobs/:id/` | Job detail (includes recent heartbeats), update, delete |
| GET | `/api/heartbeats/?job=:id` | Heartbeat history (read-only — written by `ingestion-service`) |
| GET/POST | `/api/alert-rules/` | List / create alert rules |
| DELETE | `/api/alert-rules/:id/` | Remove a rule |
| GET | `/api/alert-events/?job=:id&status=open` | Alert history |
| GET | `/healthz` | Liveness check |
| — | `/admin/` | Django admin |

## Auth

I used session-cookie auth, not a bearer token: `/api/auth/login/` sets an
httpOnly `sessionid` cookie, so there's nothing a browser-side XSS payload
could read out of `localStorage` and exfiltrate. All `/api/` endpoints
require that session. State-changing requests (`POST`/`PUT`/`PATCH`/`DELETE`)
additionally require an `X-CSRFToken` header matching the `csrftoken`
cookie — Django's standard double-submit-cookie CSRF check, enforced by
`SessionAuthentication` (see `pipelineops/authentication.py` for why I made
it a thin subclass rather than using the DRF default: the default returns
`403` for "not logged in", which is indistinguishable from a CSRF failure or
an authenticated-but-forbidden request; I wanted a clean `401` instead).
The `csrftoken` cookie is intentionally *not* httpOnly — the frontend has
to read it to echo it back in the header, which is the whole point of the
double-submit pattern.

Because the frontend and API are different origins even in local dev
(different ports), this needs `CORS_ALLOW_CREDENTIALS = True` and the
frontend origin listed in both `CORS_ALLOWED_ORIGINS` and
`CSRF_TRUSTED_ORIGINS`.

## Alerting

I run `alerts.tasks.check_missed_heartbeats` on a Celery beat schedule
(`HEARTBEAT_CHECK_INTERVAL_SECONDS`, default 30s). For each active job whose
`computed_status` is `late` or `failed`, it opens an `AlertEvent` (I skip
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
pip install -r requirements-dev.txt   # adds ruff on top of requirements.txt
ruff check .
python manage.py check
python manage.py makemigrations --check --dry-run
python manage.py test
```
