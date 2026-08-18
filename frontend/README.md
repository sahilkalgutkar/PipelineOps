# frontend

I built this as the React + TypeScript dashboard for PipelineOps: job health,
alert rules, and heartbeat history, talking to `core-api` over a session
cookie (see the root README's
[Auth section](../README.md#auth-what-changed-and-why)).

## Running locally

```bash
npm install
cp .env.example .env   # then point VITE_API_BASE_URL at a running core-api
npm run dev
```

Normally you'd just use `docker compose up` from the repo root instead of
running this by hand.

## Checks

```bash
npm run lint    # eslint
npm test        # vitest
npm run build   # tsc -b && vite build
```
