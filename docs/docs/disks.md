
# Persistent Storage

Miren provides two options for persistent storage: **Local Shared Storage** (recommended for most use cases) and **Miren Disks** (experimental, for advanced scenarios).

## Local Shared Storage

Every Miren app automatically gets persistent storage at `/miren/data/local`. This is the simplest way to persist data across restarts and deployments.

### How It Works

- **Automatic**: No configuration needed—just write to `/miren/data/local`
- **Persistent**: Data survives container restarts and redeployments
- **Shared**: All containers within your app share the same storage
- **Host-local**: Data lives on the server's filesystem

### Example: SQLite Database

```javascript
const sqlite3 = require('sqlite3');
const path = require('path');

// Database persists at /miren/data/local/app.db
const dbPath = path.join('/miren/data/local', 'app.db');
const db = new sqlite3.Database(dbPath);
```

### Example: File Uploads

```python
import os

UPLOAD_DIR = '/miren/data/local/uploads'
os.makedirs(UPLOAD_DIR, exist_ok=True)

def save_upload(file):
    path = os.path.join(UPLOAD_DIR, file.filename)
    file.save(path)
```

### When to Use Local Shared Storage

- SQLite databases
- File uploads and user content
- Application cache
- Session storage
- Any data that needs to persist across restarts

### Limitations

- **Host-local**: Data is tied to the server. If you move your app to a different server, you'll need to migrate the data manually.
- **No automatic backups**: Unlike Miren Disks, local storage isn't replicated to Miren Cloud.
- **Shared access**: All containers in your app can read/write simultaneously—your application needs to handle concurrent access (SQLite handles this well when configured with `PRAGMA journal_mode=WAL`).

---

## Miren Disks

:::info Experimental Feature
We're excited about Miren Disks—cloud-synced storage that travels with your app is a powerful capability we're actively building toward. That said, we're still working out the kinks, so for data you care about today, [Local Shared Storage](#local-shared-storage) is the safer choice.

We'd love to have you try Disks and share your feedback as we work toward making this a production-grade feature!
:::

Miren Disks provide cloud-synced persistent storage with automatic replication to Miren Cloud. This enables data portability across clusters but adds complexity.

### Why Use Disks?

- **Portable across clusters**: Your disk data is automatically synced to Miren Cloud and can be restored on any cluster
- **Automatic backups**: Data is replicated to Miren Cloud, giving you peace of mind
- **Configurable size and filesystem**: Specify exactly what you need

### How Disks Work

When you configure a disk for your application:

1. **Miren creates the disk** with the size and filesystem you specify
2. **Your app instance acquires a lease** on the disk (exclusive access)
3. **The disk is mounted** at the path you specified in your container
4. **Data is replicated** to Miren Cloud in the background

When your app stops or restarts:
- The lease is released
- Data remains on the disk
- Your next instance can acquire the lease and continue where it left off

### How Much Storage Does Miren Provide?

During the Developer Preview, we're providing unmetered storage. The intention is to implement a free tier
and usage-based pricing on the storage. We'll be sure to communicate often and clearly how we intend
to proceed.

The feature is designed to keep our costs low, and our intention is to pass that low cost on to our users.

### Configuring Disks

Add a disk to your application by including a `disks` section in your service configuration in `.miren/app.toml`:

```toml
[services.web]
image = "myapp:latest"

[[services.web.disks]]
name = "my-app-data"
mount_path = "/data"
size_gb = 10
filesystem = "ext4"
```

#### Configuration Options

| Option | Required | Description |
|--------|----------|-------------|
| `name` | Yes | Unique name for the disk (alphanumeric, hyphens allowed) |
| `mount_path` | Yes | Where to mount the disk in your container |
| `size_gb` | Yes* | Size in gigabytes (required for auto-creation) |
| `filesystem` | No | Filesystem type: `ext4` (default), `xfs`, or `btrfs` |
| `read_only` | No | Mount as read-only (default: false) |

*`size_gb` is required when the disk doesn't already exist. If the disk exists, this field is ignored.

### Example: PostgreSQL with Persistent Storage

```toml
[services.db]
image = "postgres:16"

[[services.db.env]]
key = "POSTGRES_PASSWORD"
value = "secret"

[[services.db.env]]
key = "PGDATA"
value = "/var/lib/postgresql/data/pgdata"

[[services.db.disks]]
name = "myapp-postgres"
mount_path = "/var/lib/postgresql/data"
size_gb = 20
filesystem = "ext4"
```

### Example: File Upload Storage

```toml
[services.web]
image = "myapp:latest"

[[services.web.disks]]
name = "myapp-uploads"
mount_path = "/app/uploads"
size_gb = 50
```

### Disk Lifecycle

#### Creation

Disks are automatically created when your app first deploys with a volume configuration that includes `size_gb`. The disk is provisioned with the specified size and filesystem.

#### Reuse

If you deploy an app with a `name` that matches an existing disk, Miren will attach that disk instead of creating a new one. This allows you to:
- Share data between app versions
- Preserve data across complete redeployments
- Reference disks created by other apps

#### Deletion

Disks are **not** automatically deleted when you delete an app. This is intentional - your data is precious. To delete a disk:

```bash
miren debug disk delete -i <disk-id>
```

### Viewing Disks in Miren Cloud

When connected to Miren Cloud, you can view and monitor your disks:

1. **Dashboard**: See all disks across your clusters with their status and usage
2. **Data sync status**: Monitor replication progress to the cloud
3. **Disk history**: View when disks were created, attached, and modified

Visit [miren.cloud](https://miren.cloud) and navigate to your cluster to view disk details.

### Inspecting Disks via CLI

List all disks:

```bash
miren debug disk list
```

Check a specific disk's status:

```bash
miren debug disk status -i <disk-id>
```

View active disk leases:

```bash
miren debug disk lease-list
```

See [CLI Reference - Disk Commands](/command/debug-disk) for complete command documentation.

### Important Considerations

#### One Instance per Disk

Disks use exclusive leasing - only one app instance can mount a disk at a time. This ensures data consistency but means:

- Multiple replicas of your app cannot share the same disk
- If you need shared storage, use separate disks per instance or external storage

#### Disk Sizing

- Disks use thin provisioning, so storage is only allocated as needed
- Choose a size that accommodates growth

#### Filesystem Choice

- **ext4**: Best general-purpose choice, widely compatible
- **xfs**: Better for large files and high-throughput workloads

**NOTE:** Your server must have the mkfs tools to format the disk types.

### Next Steps

- [app.toml Reference — Disks](/app-toml#disks) — Complete field reference for disk configuration (including `lease_timeout`)
- [Services](/services) — Define services that use persistent storage
- [Getting Started](/getting-started) — Deploy your first app
- [CLI Reference - Disk Commands](/command/debug-disk) — Complete disk CLI reference
- [Working with Miren Cloud](/working-with-miren-cloud) — Set up cloud features
