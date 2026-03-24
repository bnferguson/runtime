# Addons

Addons are managed backing services that Miren provisions and operates for your app. Instead of running your own database as a service, you declare an addon and Miren handles the infrastructure — creating the server, injecting connection credentials, and cleaning up when you're done.

## Addons vs. Services

| | Addons | Services |
|---|--------|----------|
| **Setup** | One line in app.toml | Configure image, env, disks, scaling |
| **Management** | Miren provisions and manages | You manage the process |
| **Credentials** | Automatically injected as env vars | You configure manually |
| **Best for** | Production databases, managed infrastructure | Custom software, full control |

If you just need a PostgreSQL database for your app, use an addon. If you need custom PostgreSQL extensions, a specific version, or full control over the configuration, run it as a [service](/services).

## Available Addons

| Addon | Description |
|-------|-------------|
| `miren-postgresql` | Managed PostgreSQL database |

List available addons on your cluster:

```bash
miren addon list-available
```

## Adding an Addon

### Via app.toml (recommended)

Declare addons in your `.miren/app.toml`. They're provisioned automatically on deploy:

```toml
name = "myapp"

[services.web]
command = "npm start"

[addons.miren-postgresql]
variant = "small"
```

### Via CLI

Attach an addon to an existing app:

```bash
miren addon create miren-postgresql:small -a myapp
```

## Environment Variables

When an addon is provisioned, Miren injects connection credentials as environment variables into your app. These are available to all services in the app.

For PostgreSQL, the following variables are injected:

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | Full connection URL (sensitive) | `postgres://user:pass@host:5432/dbname` |
| `PGHOST` | Database hostname | `10.10.0.196` |
| `PGPORT` | Database port | `5432` |
| `PGUSER` | Database username | `myapp` |
| `PGPASSWORD` | Database password (sensitive) | — |
| `PGDATABASE` | Database name | `myapp` |

Most frameworks and ORMs connect automatically using `DATABASE_URL`. You can verify the variables are set:

```bash
miren env list -a myapp
```

## Variants

Each addon offers variants that control the resource allocation and architecture:

```bash
miren addon variants miren-postgresql
```

### PostgreSQL Variants

| Variant | Description | Use case |
|---------|-------------|----------|
| `small` | Dedicated PostgreSQL server (1 GB storage) | Production apps needing isolation |
| `shared` | Multi-app shared PostgreSQL server | Development, staging, or small apps |

**Dedicated** (`small`): Each app gets its own PostgreSQL instance with dedicated storage. Best for production workloads where you need isolation and predictable performance. Start here if your app might grow.

**Shared** (`shared`): Multiple apps share a single PostgreSQL server, each with their own database and credentials. The shared server does not isolate workloads — a heavy query in one app can affect others on the same server. This variant is designed for small internal tools, staging environments, and apps you know will stay lightweight.

If no variant is specified, the default (`small`) is used.

:::note Changing variants
There is currently no way to migrate between variants (e.g. upgrading from `shared` to `small`). If you need to switch, you would need to back up your data, destroy the addon, recreate it with the new variant, and restore. We plan to add in-place variant upgrades in a future release.
:::

## Addon Lifecycle

### Provisioning

When you deploy an app with addons or run `addon create`, Miren:

1. Creates the addon association (status: **pending**)
2. Provisions the backing infrastructure (status: **provisioning**)
3. Injects environment variables into your app configuration
4. Marks the addon as ready (status: **active**)
5. Starts your app with the injected credentials

Provisioning typically takes 1–2 minutes for a new dedicated server (longer if the PostgreSQL image needs to be pulled for the first time).

Your app won't start until addon provisioning completes — Miren holds off launching your app's processes until all addons reach active status.

### Checking Status

List addons attached to your app:

```bash
miren addon list -a myapp
```

### Removing an Addon

Remove an addon and delete its data:

```bash
miren addon destroy miren-postgresql -a myapp
```

:::warning
Destroying an addon permanently deletes the database and all its data. This cannot be undone.
:::

To remove an addon via app.toml, delete the `[addons.miren-postgresql]` section and redeploy. Miren detects the removal and deprovisions the addon.

## Example: Bun + PostgreSQL

A simple web server that tracks page visits using PostgreSQL:

**`.miren/app.toml`**:
```toml
name = "my-bun-app"

[services.web]
command = "bun run index.ts"

[addons.miren-postgresql]
variant = "shared"
```

**`index.ts`**:
```typescript
import { SQL } from "bun";

const sql = new SQL({ url: process.env.DATABASE_URL });

await sql`
  CREATE TABLE IF NOT EXISTS visits (
    id SERIAL PRIMARY KEY,
    visited_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  )
`;

const server = Bun.serve({
  port: process.env.PORT || 3000,
  async fetch(req) {
    await sql`INSERT INTO visits DEFAULT VALUES`;
    const [{ count }] = await sql`SELECT COUNT(*) as count FROM visits`;
    return new Response(`Visits: ${count}\n`);
  },
});
```

Deploy:
```bash
miren deploy
```

Miren provisions PostgreSQL, injects `DATABASE_URL`, and starts your app once the database is ready.

## Backing Up and Restoring

:::info Early Version
Addon backup and restore uses the general-purpose disk backup system. We plan to add addon-aware backup commands in a future release that will simplify this workflow. For now, the steps below work reliably for PostgreSQL addon data.

The `disk backup` and `disk restore` commands must be run directly on the server (via SSH or `miren ssh`), not from your local machine. Remote backup support is planned.
:::

Each PostgreSQL addon stores its data on a Miren disk. You can back up and restore this disk using the `miren disk backup` and `miren disk restore` commands.

### Finding the Disk Name

List disks to find the one belonging to your addon:

```bash
miren debug disk list
```

Addon disks are named with a `pg-` prefix. For dedicated (`small`) addons, the name includes your app name (e.g. `pg-pg-myapp-s...-data`). For shared addons, it starts with `pg-shared-data-`.

### Creating a Backup

Back up the disk to a compressed snapshot file. This must be run on the server:

```bash
miren disk backup -n <disk-name>
```

This creates a timestamped `.miren.zst` file in the current directory. If the disk is currently in use, the backup will be crash-consistent (safe for PostgreSQL, which uses write-ahead logging).

Example:

```bash
miren disk backup -n pg-pg-myapp-sCZDabc123-data
# Output: pg-pg-myapp-sCZDabc123-data-20260324-120000.miren.zst
```

You can specify a custom output path with `-o`:

```bash
miren disk backup -n pg-pg-myapp-sCZDabc123-data -o /backups/myapp-db.miren.zst
```

### Restoring from a Backup

To restore, provide the snapshot file. This must also be run on the server:

```bash
miren disk restore -s <snapshot-file>
```

The restore recreates the disk with the original name. If the disk already exists, use `--force` to overwrite:

```bash
miren disk restore -s myapp-db.miren.zst --force
```

To restore to a different disk name:

```bash
miren disk restore -s myapp-db.miren.zst -n new-disk-name
```

After restoring, restart your app to pick up the restored data:

```bash
miren app restart myapp
```

### Backup Recommendations

- **Schedule regular backups** for production databases, especially before destructive operations like `addon destroy`
- **Store backups off-server** — copy snapshot files to external storage
- **Test restores periodically** to verify your backups are valid
- Backups are compressed with zstd and include checksum verification
