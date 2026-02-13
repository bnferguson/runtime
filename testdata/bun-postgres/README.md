# Bun + PostgreSQL Example

A simple visit counter built with [Bun](https://bun.sh) and the shared PostgreSQL addon.

## What it does

- `GET /` — records a visit and displays the total visit count
- `GET /visits` — returns the 20 most recent visits as JSON
- `GET /health` — checks database connectivity

## Deploy

```bash
m app create bun-postgres
m deploy
```

The `miren-postgresql` addon in `.miren/app.toml` automatically provisions a shared PostgreSQL database and injects `DATABASE_URL` and `PG*` environment variables.

## Environment variables

The addon provides:

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | Full connection string |
| `PGHOST` | PostgreSQL host |
| `PGPORT` | PostgreSQL port |
| `PGUSER` | Database user |
| `PGPASSWORD` | Database password |
| `PGDATABASE` | Database name |
