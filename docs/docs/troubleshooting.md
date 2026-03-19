
# Troubleshooting

A step-by-step guide to diagnosing issues with your Miren applications and server.

## Quick health check

Start with `miren doctor` to verify your environment is set up correctly:

```bash
miren doctor
```

This checks your configuration, server connectivity, and authentication. It provides context-aware suggestions when it detects issues. You can also run the subcommands individually:

```bash
miren doctor config   # Check cluster configuration
miren doctor server   # Check server connectivity
miren doctor auth     # Check authentication
```

## App not starting

If your app is deployed but not responding:

**1. Check the app status**

```bash
miren app status -a myapp
```

This shows the current deployment state, version, configuration, and any error messages.

**2. Check the logs**

```bash
# Recent logs
miren logs -a myapp

# Live tail
miren logs -a myapp -f

# Filter for errors
miren logs -a myapp -g error

# Logs from a specific service
miren logs -a myapp --service web
```

**3. Check sandbox state**

```bash
miren sandbox list
```

Look for sandboxes that are stuck in `pending` or `not_ready`, or that have gone `dead`. Use `--all` to include dead sandboxes in the output.

## Deploy failed

**1. Find the failed deployment**

```bash
miren app history -a myapp
```

Failed deployments are marked with a red `✗`. Use `--detailed` for more info including error messages and git SHAs.

**2. Check build logs**

```bash
miren logs build -a myapp VERSION
```

Replace `VERSION` with the version from the deployment history. This shows the build output so you can see where things went wrong. See [Logs](/logs) for more on filtering and following logs.

## Server-level issues

If you suspect the Miren server itself is having problems:

**1. Check server logs on the host**

For systemd installations:

```bash
sudo journalctl -u miren -f
```

For Docker installations:

```bash
docker logs -f miren
```

**2. Test connectivity**

```bash
miren debug connection
```

This tests RPC and HTTP connectivity to the server and reports the server version and auth status.

## Gathering a debug bundle

If you've worked through the steps above and need further help, collect a debug bundle to share:

```bash
sudo miren debug bundle
```

This creates a `miren-debug.tar.gz` archive containing system info, container state, process lists, and server logs.

:::warning Review bundles before sharing
Debug bundles collect diagnostic data that may include sensitive information:

- **Process command lines** — arguments passed to running processes may contain tokens or credentials
- **Application logs** — error messages and stack traces can include request data or internal details

Environment variable values are automatically redacted from container inspect output, but logs and process arguments are included as-is. Review the bundle contents and remove anything sensitive before sharing, especially in public channels like GitHub Issues.
:::

:::tip Use sudo for a complete bundle
Without sudo, the command still runs but produces a partial bundle. Root access is needed for:
- **Containerd socket** — the primary source of container state
- **System journal** — miren server logs via journalctl
- **Docker socket** — unless your user is in the `docker` group
:::

### Bundle options

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output` | Output file path | `miren-debug.tar.gz` |
| `-s, --since` | Include logs since this time | `1 day ago` |
| `-d, --docker-container` | Docker container name | `miren` |

```bash
# Include logs from the last week
sudo miren debug bundle --since "7 days ago"

# Save to a specific path
sudo miren debug bundle -o /tmp/miren-debug.tar.gz
```

## Getting help

If you're stuck, share your debug bundle and what you've tried:

- **Discord** — [miren.dev/discord](https://miren.dev/discord) for community help and questions
- **GitHub Issues** — [File a bug report](https://github.com/mirendev/runtime/issues/new?template=bug_report.yml) and attach your debug bundle
- **Feature Requests** — [Miren Roadmap](https://github.com/mirendev/roadmap/issues) for ideas and suggestions

Remember to review debug bundles for sensitive data before attaching them to public issues.
