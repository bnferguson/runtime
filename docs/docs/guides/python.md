---
title: Python on Miren
description: Deploy Python apps — FastAPI, Django, Flask — on Miren with automatic build detection.
keywords: [python, fastapi, django, flask, gunicorn, uvicorn, uv, poetry, pipenv, deploy]
---

import CliCommand from '@site/src/components/CliCommand';

# Python on Miren

Miren auto-detects Python apps and builds a container image for you — no Dockerfile
required. It recognizes pip, Pipenv, Poetry, and uv, and configures a start command
for common web frameworks.

:::tip[Let your agent do this]
Ask your AI coding agent to "set up this Python app on Miren" after installing the
[Miren agent skills](/agent-skills). It detects your framework and package manager,
proposes a start command, wires up environment variables, and deploys — using this
page as its reference.
:::

## Do you need a Dockerfile?

No. Miren detects Python from a `requirements.txt`, `Pipfile`, `pyproject.toml`, or
`uv.lock` and builds the image automatically. The default Python version is **3.11**;
override it in [`.miren/app.toml`](/app-configuration) if you need another.

Provide a `Dockerfile.miren` only if your build needs custom system packages or steps
that don't fit detection — see [Using Dockerfile.miren](/languages#using-dockerfilemiren).

## Set up the app

From your project root, initialize and deploy:

<CliCommand context="client">
```miren
miren init
miren deploy
```
</CliCommand>

`miren init` scaffolds `.miren/app.toml` and scans your project for the environment
variables it needs. `miren deploy` uploads your code, builds the image, and activates
the new version. Preview what Miren detects — stack, package manager, and start
command — without building:

<CliCommand context="client">
```miren
miren deploy --analyze
```
</CliCommand>

### Package managers

Miren picks the install command from the files in your repo:

| File | Package manager | Install command |
|------|-----------------|-----------------|
| `Pipfile` | pipenv | `pipenv install --deploy` |
| `uv.lock` | uv | `uv sync --frozen` |
| `pyproject.toml` | poetry | `poetry install --no-root` |
| `requirements.txt` | pip | `pip install -r requirements.txt` |

### Start command

Miren detects your web framework and configures a start command. You can always
override it with a `Procfile` or the `command` field in `.miren/app.toml`.

| Framework | Detected start |
|-----------|----------------|
| FastAPI | `fastapi run` |
| Django | `gunicorn` or `uvicorn` |
| Flask | `gunicorn` |
| Gunicorn / Uvicorn | `gunicorn` / `uvicorn` |

Your server must bind to `0.0.0.0` on `$PORT` — Miren injects `PORT` and routes
traffic to it. A `Procfile` makes this explicit:

```procfile
# gunicorn (Flask / Django / WSGI)
web: gunicorn app:app --bind 0.0.0.0:$PORT

# uvicorn (FastAPI / Starlette / ASGI)
web: uvicorn main:app --host 0.0.0.0 --port $PORT

# uv
web: uv run gunicorn app:app --bind 0.0.0.0:$PORT

# Celery background worker
worker: celery -A tasks worker --loglevel=info
```

See [Services](/services) for running a worker alongside your web process.

## Environment variables

Set variables with `miren env set`. Use `-e` for plain values and `-s` for secrets
(masked in output and logs):

<CliCommand context="client">
```miren
miren env set -e LOG_LEVEL=info
miren env set -s DATABASE_URL=postgres://user:pass@host/db
miren env set -s SECRET_KEY
```
</CliCommand>

`miren env set -s SECRET_KEY` (no value) prompts with masked input. You can also
declare variables in `.miren/app.toml`:

```toml
[[env]]
key = "DATABASE_URL"
value = ""
required = true
sensitive = true
description = "Postgres connection string"
```

See [App Configuration — Environment Variables](/app-configuration#environment-variables).

## Agent quick reference

- **Detection:** `requirements.txt`, `Pipfile`, `pyproject.toml`, or `uv.lock`
- **Default version:** Python 3.11 (override via `[build] version` in `.miren/app.toml`)
- **Install:** pipenv / uv / poetry / pip, chosen by manifest (see table above)
- **Start command:** bind `0.0.0.0:$PORT`; FastAPI/Django/Flask auto-detected, else set a `Procfile`
- **Env vars:** `miren env set -e KEY=VALUE`, `-s` for secrets, or `[[env]]` in `app.toml`
- **Dockerfile:** not needed; add `Dockerfile.miren` only for custom builds

## Next steps

- [Supported Languages — Python](/languages#python) — full build detail
- [App Configuration](/app-configuration) — customize `.miren/app.toml`
- [Services](/services) — web + workers
- [Deployment](/deployment) — how deploys build and activate
