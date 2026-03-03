---
sidebar_position: 3
---

# App Configuration

Miren uses a **convention over configuration** approach. Most apps deploy with zero configuration—Miren detects your language, builds your image, and runs it with sensible defaults. When you need to customize, you add a `.miren/app.toml` file.

## When You Don't Need app.toml

If your app is a single web service with a standard language stack, Miren handles everything:

- **Language and build**: Detected from your project files (`package.json`, `go.mod`, `Gemfile`, etc.) — see [Supported Languages](/languages)
- **Start command**: Detected from your framework or `Procfile`
- **Scaling**: Web services autoscale based on traffic by default

You can deploy with just:

```bash
miren init
miren deploy
```

## When You Need app.toml

Create `.miren/app.toml` when you need to:

- **Run multiple services** — web server plus workers, databases, or caches
- **Set environment variables** — configuration your app reads at runtime
- **Tune scaling** — adjust concurrency thresholds or use fixed instance counts
- **Attach persistent disks** — for databases or file storage
- **Customize builds** — specify a Dockerfile, language version, or extra build steps
- **Configure addons** — like object storage

## Configuration Sections

Here's how the sections of `app.toml` map to your application's lifecycle:

### Build

The `[build]` section controls how Miren builds your container image. Override the detected language version, point to a custom Dockerfile, or add post-build steps.

```toml
[build]
version = "3.12"
onbuild = ["npm run build"]
```

See [Supported Languages](/languages) for build details per language.

### Services

The `[services.<name>]` sections define the processes your app runs. Each service gets its own command, scaling configuration, and optionally its own container image.

```toml
[services.web]
command = "node server.js"

[services.worker]
command = "node worker.js"
```

See [Services](/services) for patterns like running databases alongside your app.

### Scaling

Each service has a `[services.<name>.concurrency]` section that controls how it scales. Web services default to autoscaling; everything else defaults to a single fixed instance.

```toml
[services.web.concurrency]
mode = "auto"
requests_per_instance = 20

[services.worker.concurrency]
mode = "fixed"
num_instances = 3
```

See [Application Scaling](/scaling) for tuning guidance.

### Persistent Storage

Services can attach disks for data that needs to survive restarts. Disks use exclusive leasing and require fixed concurrency with a single instance.

```toml
[services.db.concurrency]
mode = "fixed"
num_instances = 1

[[services.db.disks]]
name = "postgres-data"
mount_path = "/var/lib/postgresql/data"
size_gb = 20
```

See [Persistent Storage](/disks) for local shared storage and Miren Disks.

### Environment Variables

Environment variables are declared with `[[env]]` at the top level (available to all services) or `[[services.<name>.env]]` for a specific service. Service-level env vars are merged with global ones.

```toml
# Available to all services
[[env]]
key = "DATABASE_URL"
value = "postgres://db.app.miren:5432/myapp"

# Only for the worker service
[[services.worker.env]]
key = "WORKER_CONCURRENCY"
value = "5"
```

#### Env Var Metadata

Each env var supports optional metadata fields for documentation and validation:

```toml
[[env]]
key = "API_KEY"
value = ""
required = true
sensitive = true
description = "Third-party API key for payment processing"
```

| Field | Type | Description |
|-------|------|-------------|
| `key` | string | Variable name (required) |
| `value` | string | Variable value |
| `required` | bool | If `true`, deploy will fail when this variable has no value |
| `sensitive` | bool | If `true`, the value is masked in CLI output and logs |
| `description` | string | Human-readable explanation of what this variable is for |

The `required` flag is useful for variables whose values differ per environment—declare them in `app.toml` with an empty value and `required = true`, then set the actual value with `miren env set` before deploying. The `sensitive` flag ensures secrets aren't accidentally exposed in terminal output.

## Complete Example

```toml
name = "myapp"

[[env]]
key = "DATABASE_URL"
value = "postgres://user:pass@postgres.app.miren:5432/myapp"

[[env]]
key = "SECRET_KEY"
required = true
sensitive = true
description = "Application secret for session signing"

[build]
version = "3.12"

[services.web]
command = "gunicorn app:app --bind 0.0.0.0:8000"
port = 8000

[services.web.concurrency]
mode = "auto"
requests_per_instance = 20

[services.worker]
command = "celery -A app worker"

[services.worker.concurrency]
mode = "fixed"
num_instances = 2

[services.postgres]
image = "postgres:16"

[[services.postgres.env]]
key = "PGDATA"
value = "/var/lib/postgresql/data/pgdata"

[services.postgres.concurrency]
mode = "fixed"
num_instances = 1

[[services.postgres.disks]]
name = "myapp-pgdata"
mount_path = "/var/lib/postgresql/data"
size_gb = 20
```

## Reference

For a complete field-by-field listing of every `app.toml` option, see the [app.toml Reference](/app-toml).
