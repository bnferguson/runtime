# Changelog

All notable changes to Miren Runtime will be documented in this file.

## Unreleased
*main*

**Features**
- **Smarter `miren init`** - `miren init` now scans your project for the environment variables your app actually needs and pre-sets them on the app before the first deploy, the same as if you'd run `miren config set` yourself. Generated secrets (Rails `SECRET_KEY_BASE`), file-backed keys (`RAILS_MASTER_KEY`), framework defaults, and source-detected reads across Python/Node/Bun/Go/Ruby/Rust are recognized; the ones we can resolve are picked up automatically by the first build. See [What `miren init` Does for You](/app-configuration#what-miren-init-does-for-you). ([#567](https://github.com/mirendev/runtime/pull/567))

---

## v0.7.1
*2026-04-22*

**Improvements**
- **Cut steady-state log noise** - Roughly 76% reduction in server log volume during normal operation. Removed per-request auth success logs, redundant controller reconcile lines, heartbeat spam, and other content-free chatter that drowned out useful signal. ([#767](https://github.com/mirendev/runtime/pull/767))

**Bug Fixes**
- **Fixed Valkey addon causing app connection errors** - The valkey sandbox was starting without `--requirepass` while apps received a URL containing the password, so every client got `ERR AUTH called without any password configured`. Password is now baked directly into the server command at provision time. ([#771](https://github.com/mirendev/runtime/pull/771))
- **Fixed short IDs not resolving for several commands** - `sandbox exec`, `logs sandbox`, and `deploy cancel` weren't resolving short IDs like you'd expect from seeing them in other output. Worst case was `deploy cancel` silently no-op'ing while reporting success, leaving the deploy lock stuck until its 30-minute timeout. ([#769](https://github.com/mirendev/runtime/pull/769))
- **Fixed Rust source-only deploys shipping stale binaries** - Changing only source files in a Rust app could silently ship the previously built binary. The workspace crate now force-rebuilds on every deploy while the dependency cache stays intact. ([#768](https://github.com/mirendev/runtime/pull/768))
- **Fixed activator fail-fast killing healthy replacement deploys** - When a crash-looping version was replaced with a fix, the activator counted dead sandboxes from the broken version and fail-fasted the fix before it got to boot a sandbox of its own. Fail-fast accounting is now scoped to the requesting version. ([#766](https://github.com/mirendev/runtime/pull/766))
- **Fixed silent app.toml parse errors during build** - A parse error in `app.toml` used to be logged and discarded, and the build continued as if no config existed — services, env vars, and addons silently gone. Parse errors now fail the build and surface the full diagnostic (file, line, suggestions) to the client. ([#765](https://github.com/mirendev/runtime/pull/765))

---

## v0.7.0
*2026-04-14*

**Breaking Changes**
- **Local shared storage is now opt-in** - Apps that need persistent local storage now declare it explicitly as a disk with `provider = "local"` in app.toml instead of getting an automatic bind mount at `/miren/data/local`. This makes storage dependencies visible to the scheduler, which needs to know about them as we build out multi-node cluster support. If your app has existing data in local storage, Miren detects it automatically, keeps mounting it, and shows a warning during deploy with a link to add the config. No data loss, just a nudge to make the dependency explicit at your own pace. See [Migrating from automatic local storage](https://miren.md/disks#migrating-from-automatic-local-storage) for details. ([#700](https://github.com/mirendev/runtime/pull/700), [#719](https://github.com/mirendev/runtime/pull/719))

**Features**
- **Managed addons** - Miren now provisions and manages backing services alongside your apps. Add a database or cache with `miren addon add`, and Miren handles the container lifecycle, networking, and credential injection. Launch includes PostgreSQL, MySQL, Valkey, Memcache, and RabbitMQ, with version selection and custom OCI image support. ([#688](https://github.com/mirendev/runtime/pull/688), [#706](https://github.com/mirendev/runtime/pull/706), [#720](https://github.com/mirendev/runtime/pull/720), [#726](https://github.com/mirendev/runtime/pull/726), [#727](https://github.com/mirendev/runtime/pull/727), [#743](https://github.com/mirendev/runtime/pull/743), [#755](https://github.com/mirendev/runtime/pull/755), [#758](https://github.com/mirendev/runtime/pull/758), [#760](https://github.com/mirendev/runtime/pull/760))
- **Short entity IDs** - Entities now get short 3-8 character identifiers that work anywhere a full ID does. CLI tables are easier to read and IDs are easy to type from memory. ([#696](https://github.com/mirendev/runtime/pull/696), [#721](https://github.com/mirendev/runtime/pull/721), [#741](https://github.com/mirendev/runtime/pull/741))
- **CLI aliases** - Define custom command shortcuts in `.miren/app.toml` under `[aliases]`. Supports multi-word names and appends extra arguments. ([#693](https://github.com/mirendev/runtime/pull/693))
- **Disk undelete** - Deleted disks are now soft-deleted with a 7-day retention window. `disk undelete` restores them, and `disk list-deleted` shows what's recoverable. ([#694](https://github.com/mirendev/runtime/pull/694))
- **`miren app restart`** - New command to restart an app immediately. Deploys also now reset crash cooldown, so a new deploy isn't blocked by stale backoff from a previous crash. ([#702](https://github.com/mirendev/runtime/pull/702))

**Improvements**
- **Better config error messages** - Typos and validation errors in `.miren/app.toml` now show file location, underline the problem, and suggest corrections with color output. ([#709](https://github.com/mirendev/runtime/pull/709))
- **Faster `app list`** - App list aggregation moved server-side, significantly reducing round trips for clusters with many apps. ([#701](https://github.com/mirendev/runtime/pull/701))
- **Disk-aware pool draining** - Services with disks now drain old pools before starting new ones, preventing data conflicts during deploys. ([#725](https://github.com/mirendev/runtime/pull/725))
- **Hardened embedded etcd** - Defenses against bbolt freelist bloat that could cause etcd to slow down over time. ([#733](https://github.com/mirendev/runtime/pull/733))
- **App name in sandbox list** - `sandbox list` now shows which app each sandbox belongs to. ([#757](https://github.com/mirendev/runtime/pull/757))
- **Dual-stack IP discovery** - Cluster address reporting now discovers both IPv4 and IPv6 public addresses, and TLS certificates include public IP SANs automatically. ([#708](https://github.com/mirendev/runtime/pull/708), [#712](https://github.com/mirendev/runtime/pull/712))

**Bug Fixes**
- **Fixed deploy from subdirectory using wrong source directory** ([#716](https://github.com/mirendev/runtime/pull/716))
- **Friendlier TLS certificate behavior when DNS isn't ready yet** - ACME challenges now back off gracefully instead of burning through rate limits when a domain's DNS isn't fully propagated. ([#710](https://github.com/mirendev/runtime/pull/710), [#730](https://github.com/mirendev/runtime/pull/730))
- **Fixed overlay IP allocator assigning duplicate IPs after restart** ([#707](https://github.com/mirendev/runtime/pull/707))
- **Fixed sandbox hostname resolution for addon sub-containers** ([#728](https://github.com/mirendev/runtime/pull/728))
- **Fixed stale pool reuse when volume/mount config changes** ([#718](https://github.com/mirendev/runtime/pull/718))
- **Self-healing loop-backed volumes after unclean shutdown** - After a SIGKILL, miren now detects and cleans up stale loop devices instead of mounting a second one and corrupting the filesystem. ([#756](https://github.com/mirendev/runtime/pull/756))
- **Fixed release bundle detection for CLI-only installs** ([#759](https://github.com/mirendev/runtime/pull/759))

---

## v0.6.1
*2026-03-24*

**Improvements**
- **Faster system log queries** - `miren logs system` now returns results in under a second instead of ~10 seconds by using VictoriaLogs' native `limit` parameter instead of server-side sorting. ([#681](https://github.com/mirendev/runtime/pull/681))
- **JSON output for more CLI commands** - `debug netdb list`, `debug netdb status`, and `doctor config` now support `--format json`. Added `--json` as a shorthand for `--format json` on all commands that support it. ([#687](https://github.com/mirendev/runtime/pull/687))

**Bug Fixes**
- **Fixed delta deploys failing when cached and changed files share directories** - Deploying after a cached delta could fail with `mkdir: file exists` when both the cached and changed file sets contained the same directory. ([#689](https://github.com/mirendev/runtime/pull/689))
- **Fixed CLI commands ignoring per-app cluster selection** - Commands like `route list`, `sandbox list`, and `app list` now respect the cluster chosen via `cluster switch` in an app directory, instead of silently falling back to the global active cluster. ([#683](https://github.com/mirendev/runtime/pull/683))

**Documentation**
- Added a top-level Deployment docs page covering the full deploy lifecycle. ([#684](https://github.com/mirendev/runtime/pull/684))

---

## v0.6.0
*2026-03-17*

**Features**
- **Disk storage system overhaul** - The disk subsystem has been rewritten with a new Universal mode backed by loop devices, replacing the previous LSVD/NBD architecture. Existing LSVD volumes are migrated automatically on first boot. Includes disk backup/restore support and a new cloud accelerator mode for segment upload/replay. ([#639](https://github.com/mirendev/runtime/pull/639))
- **System logs accessible from the CLI** - `miren logs system` queries server logs directly from the CLI — same interface as app logs, with `--follow`, `--last`, and `--format json` all working. Under the hood, server logs are ingested into VictoriaLogs through a structured handler, so all attributes (module, level, error fields) are preserved and queryable. `miren logs` is also restructured into proper subcommands (`app`, `sandbox`, `build`, `system`) — bare `miren logs` still works as shorthand for `miren logs app`. ([#645](https://github.com/mirendev/runtime/pull/645), [#662](https://github.com/mirendev/runtime/pull/662), [#679](https://github.com/mirendev/runtime/pull/679))
- **Delta file transfer for deploys** - The deploy client now sends a file manifest first; the server compares it against the cached source from the previous deploy and the client only uploads what changed. Subsequent deploys of unchanged code upload nothing. ([#670](https://github.com/mirendev/runtime/pull/670))
- **Wildcard routes** - Route all subdomains of a domain to a single app with `miren route set *.example.com myapp`. Wildcard routes match any subdomain and the bare domain, with exact routes taking priority. TLS certificates are provisioned automatically for each matching subdomain. ([#659](https://github.com/mirendev/runtime/pull/659))

**Improvements**
- **Upload progress bar and caching summary** - Deploy uploads show a real progress bar with percentage and speed. After upload, a summary shows how many files were cached from the previous deploy and estimated time saved. ([#680](https://github.com/mirendev/runtime/pull/680))
- **Eager TLS certificate provisioning** - Adding a route now triggers certificate provisioning immediately in the background. HTTPS connections no longer stall 30–90 seconds on first access. ([#661](https://github.com/mirendev/runtime/pull/661), [#664](https://github.com/mirendev/runtime/pull/664))
- **Deploy progress tracks build steps** - The progress bar now tracks build steps instead of layer downloads, so it stays active on cached-image deploys. ([#665](https://github.com/mirendev/runtime/pull/665))
- **JSON output for log and app commands** - `miren logs` (all subcommands), `miren app`, `miren app status`, and `miren app history` all support `--format json` for scripting and automation. ([#675](https://github.com/mirendev/runtime/pull/675), [#673](https://github.com/mirendev/runtime/pull/673))
- **Group help for CLI discovery** - `miren help --commands` lists all commands with synopses (supports `--format json`). `miren help app.list version sandbox.stop` shows help for multiple commands at once using dot notation. ([#676](https://github.com/mirendev/runtime/pull/676))
- **System requirements check at install** - The installer now verifies at least 4 GB memory and 50 GB storage before proceeding. ([#663](https://github.com/mirendev/runtime/pull/663))

**Bug Fixes**
- **Fixed rapid deploys causing orphaned sandboxes** - Deploying the same app multiple times in quick succession no longer leaves orphaned sandboxes running or stale routing entries. ([#672](https://github.com/mirendev/runtime/pull/672))
- **Fixed disk config silent failures** - Using `size` instead of `size_gb` in disk config no longer leaves sandboxes stuck in PENDING forever; unknown fields now surface as errors. ([#666](https://github.com/mirendev/runtime/pull/666))
- **Fixed app TUI showing wrong concurrency** - The `miren app` details view now correctly shows fixed instance counts instead of "auto" for services with explicit concurrency settings. ([#667](https://github.com/mirendev/runtime/pull/667))
- **Fixed env var file values including trailing newlines** - `KEY=@filepath` now trims trailing line endings from file contents. ([#671](https://github.com/mirendev/runtime/pull/671))

---

## v0.5.0
*2026-03-10*

**Features**
- **Multi-port and non-HTTP services** - Apps can now expose multiple ports with different protocols (TCP, UDP, HTTP) using `[[services.<name>.ports]]` in app.toml. Node ports are validated at deploy time to prevent conflicts. Enables use cases like IRC servers alongside HTTP endpoints. ([#641](https://github.com/mirendev/runtime/pull/641))
- **CI deployments with GitHub Actions** - Deploy from GitHub Actions and other CI systems using short-lived identity tokens instead of long-lived secrets. Bind repos to apps with `miren auth ci add --github OWNER/REPO -a APP`, and GitHub Actions will authenticate automatically. Target clusters from CI with the `MIREN_CLUSTER` env var. ([#631](https://github.com/mirendev/runtime/pull/631), [#633](https://github.com/mirendev/runtime/pull/633), [#635](https://github.com/mirendev/runtime/pull/635))
- **Required environment variables** - Declare env vars as `required = true` in app.toml and deploys will fail early with a clear message listing what's missing. Vars can also be marked `sensitive` and given a `description` that appears in `miren env list`. ([#638](https://github.com/mirendev/runtime/pull/638))

**Improvements**
- **Zero-config app initialization** - Running any app command without an `app.toml` now offers to run `miren init` for you interactively, inferring the app name from your directory. In CI, the error message tells you what to do. ([#654](https://github.com/mirendev/runtime/pull/654))
- **Bun runtime detection without lockfile** - Bun apps without dependencies are now detected via `bunfig.toml`, the `packageManager` field in package.json, or `bun`/`bunx` commands in package.json scripts. ([#656](https://github.com/mirendev/runtime/pull/656))
- **`env set`/`env delete` show deployment progress** - Environment variable changes now go through the deployment service and show activation progress and routes, matching the UX of `deploy` and `rollback`. ([#630](https://github.com/mirendev/runtime/pull/630))

**Bug Fixes**
- **Fixed 502 errors during deployment rollovers** - The HTTP ingress now retries on a stale connection instead of immediately returning 502, and the launcher waits for new sandboxes to be ready before scaling down old ones. ([#637](https://github.com/mirendev/runtime/pull/637))
- **Fixed streaming responses being buffered** - SSE, chunked downloads, and long-polling now work correctly. The previous timeout implementation buffered entire responses in memory before sending; timeouts are now handled at the transport level so data streams incrementally. ([#634](https://github.com/mirendev/runtime/pull/634))
- **Fixed `miren exec` panic after command finishes** - `miren exec` and `miren app run` no longer panic when the remote command completes. ([#655](https://github.com/mirendev/runtime/pull/655))
- **Fixed `app run` panic when stdin isn't a terminal** - `miren app run` no longer panics in non-interactive contexts like piped input or CI. ([#632](https://github.com/mirendev/runtime/pull/632))
- **Fixed firewall rules leaking on sandbox teardown** - Port forwarding rules are now cleaned up when sandboxes stop, preventing stale rules from making node ports unreachable after redeployment. ([#644](https://github.com/mirendev/runtime/pull/644))

---

## v0.4.1
*2026-02-18*

**Bug Fixes**

- **Fixed app startup directory regression** - Apps deployed before the WORKDIR fix (v0.4.0) would boot with CWD `/` instead of `/app`, causing crashes. The `/app` default is now restored as a fallback for existing app versions without a stored `start_directory`. ([#621](https://github.com/mirendev/runtime/pull/621), [#623](https://github.com/mirendev/runtime/pull/623))
- **Fixed noisy RPC error logs** - Deploying to a server without telemetry capabilities no longer produces a spurious `[ERROR] error resolving capability` message. Optional capability lookups now degrade quietly at Debug level. ([#622](https://github.com/mirendev/runtime/pull/622))

---

## v0.4.0
*2026-02-17*

**Features**

- **First-class rollbacks** - Redeploy a previous app version without rebuilding. Use `miren rollback` for an interactive picker that shows your recent versions, or `miren deploy --version <id>` to deploy a specific version directly. Each rollback creates a full deployment record with provenance tracking. ([#590](https://github.com/mirendev/runtime/pull/590))

- **OpenTelemetry tracing** - Comprehensive distributed tracing across the Miren runtime. Traces cover HTTP ingress, deploy/build pipelines, containerd operations, and CLI commands. CLI spans are shipped through the server via a proxy exporter. Cluster identity is included in trace resource attributes. See [Observability](/observability) for configuration details. ([#595](https://github.com/mirendev/runtime/pull/595), [#601](https://github.com/mirendev/runtime/pull/601), [#602](https://github.com/mirendev/runtime/pull/602), [#609](https://github.com/mirendev/runtime/pull/609))

**Improvements**

- **Per-app cluster pinning** - The CLI now remembers which cluster to use for each app. The first time you use `-C <cluster>` with a command, that cluster is saved locally for the app. Resolution order: explicit `-C` flag > per-app pin > global active cluster. Use `miren cluster current` to see which cluster the CLI will target. ([#596](https://github.com/mirendev/runtime/pull/596))
- **Better app.toml error messages** - Invalid `app.toml` files now surface the actual parse error instead of the confusing "app is required" message. ([#614](https://github.com/mirendev/runtime/pull/614))
- **Exclude dead sandboxes from listing** - `miren sandbox list` now hides dead sandboxes by default, showing only active ones. Use `--all` to see everything. ([#585](https://github.com/mirendev/runtime/pull/585))
- **Redact secrets from debug bundles** - `miren debug bundle` now redacts sensitive information and includes guidance on sharing bundles safely. ([#612](https://github.com/mirendev/runtime/pull/612))
- **`-v` flag moved to `-V`** - The version flag is now `-V`/`--version`, freeing `-v` for `--verbose` across all commands. ([#593](https://github.com/mirendev/runtime/pull/593), [#594](https://github.com/mirendev/runtime/pull/594))
- **Disk mount robustness** - Improved disk mount lifecycle handling with better error recovery and integration test coverage. ([#597](https://github.com/mirendev/runtime/pull/597))

**Bug Fixes**

- **Fixed Dockerfile WORKDIR ignored** - App containers now honor the `WORKDIR` directive from your Dockerfile instead of always defaulting to `/`. ([#617](https://github.com/mirendev/runtime/pull/617))
- **Fixed nested .gitignore files ignored in deploy** - Deploy tarballs now respect `.gitignore` files in subdirectories, not just the root. This prevents things like `web/node_modules/` from inflating your deploy upload. ([#608](https://github.com/mirendev/runtime/pull/608))
- **Fixed disk space GC** - Garbage collection for BuildKit cache and registry blobs now works correctly, preventing disk space from being consumed by stale build artifacts. ([#588](https://github.com/mirendev/runtime/pull/588))
- **Fixed activator pool cache lockout** - The activator pool cache no longer permanently locks out at MaxPoolSize, which was preventing scale-to-zero apps from recovering after hitting the pool size limit. ([#616](https://github.com/mirendev/runtime/pull/616))
- **Fixed disk lease leak in sandbox cleanup** - Periodic sandbox cleanup now properly releases disk leases before deleting sandboxes. ([#613](https://github.com/mirendev/runtime/pull/613))
- **Fixed empty host in route commands** - `miren route set` and `miren route show` now validate that a host is provided instead of accepting empty strings. ([#591](https://github.com/mirendev/runtime/pull/591))
- **Fixed deployment cluster filter** - Deployments are now correctly filtered by cluster, and `miren app history` defaults are improved. ([#589](https://github.com/mirendev/runtime/pull/589))

---

## v0.3.1
*2026-02-06*

**Features**

- **Build-time environment variables** - Environment variables are now available during the build process, so build commands like `npm run build` can access API keys, database URLs, and other configuration. Variables from `app.toml`, existing config, and `--env`/`--secret` CLI flags are all injected at build time. ([#581](https://github.com/mirendev/runtime/pull/581))

**Improvements**

- **Preserve disk mounts during server restart** - The LSVD disk manager now survives server restarts, keeping disk mounts active. Use `systemctl reload miren` for a soft restart that preserves mounts, or `systemctl restart miren` for a full restart. This significantly reduces disruption when updating the miren server. ([#554](https://github.com/mirendev/runtime/pull/554))

  **For existing installations:** To enable this feature, either re-run `sudo miren server install --force` to regenerate the systemd unit file, or manually add the following line to `/etc/systemd/system/miren.service` under the `[Service]` section and run `systemctl daemon-reload`:
  ```
  ExecReload=/bin/kill -USR1 $MAINPID
  ```

- **Batch env var setting** - Setting multiple environment variables now creates a single app version instead of N intermediate versions. ([#578](https://github.com/mirendev/runtime/pull/578))
- **Smarter deploy coalescing** - Rapid successive deploys are now coalesced so only the latest version is launched, avoiding unnecessary churn from intermediate sandbox pools. ([#579](https://github.com/mirendev/runtime/pull/579))
- **Default app name from app.toml** - CLI commands like `app delete`, `route set`, and `route set-default` now infer the app name from `.miren/app.toml` when not specified. ([#562](https://github.com/mirendev/runtime/pull/562))
- **Clearer scaling display** - Instance counts now show `(auto)` or `(fixed)` suffix, and sandbox pool listings include a MODE column. ([#566](https://github.com/mirendev/runtime/pull/566))
- **Deploy validation for services** - Deploys now fail early with a clear error when no services are defined, instead of silently deploying an unservable app. ([#563](https://github.com/mirendev/runtime/pull/563))
- **Smarter `miren upgrade`** - Upgrade now checks permissions upfront, offers interactive sudo vs user-directory install picker, and supports a `--user` flag for non-root installation. ([#564](https://github.com/mirendev/runtime/pull/564))

**Bug Fixes**

- **Fixed build log retention** - Build logs are now properly persisted when using the central BuildKit daemon, restoring the ability to retrieve build output after a deploy with `miren logs --build`. ([#561](https://github.com/mirendev/runtime/pull/561), [#570](https://github.com/mirendev/runtime/pull/570))
- **Fixed orphaned container shims** - Containerd shims are no longer left behind when app containers crash. ([#577](https://github.com/mirendev/runtime/pull/577))
- **Fixed env vars wiped by app.toml** - Adding the first `[[env]]` entry to `app.toml` no longer silently drops existing environment variables. ([#580](https://github.com/mirendev/runtime/pull/580))
- **Fixed DNS during sandbox startup** - DNS no longer returns empty responses during sandbox startup; unknown source IPs are now lazily resolved via entity store lookup. ([#576](https://github.com/mirendev/runtime/pull/576))
- **Fixed nil panic in `miren env list`** - Running `miren env list` when an app isn't deployed to the current cluster no longer crashes. ([#572](https://github.com/mirendev/runtime/pull/572))

---

## v0.3.0
*2026-01-27*

**Features**

- **Automatic image garbage collection** - Container images are now automatically garbage collected to prevent disk exhaustion. Images are kept if less than 30 days old or within the last 20 releases per app. Garbage collection runs weekly or immediately when disk usage exceeds 80%. ([#544](https://github.com/mirendev/runtime/pull/544))
- **Deploy-time environment variables** - Set environment variables atomically with your deployment using `miren deploy -e KEY=VALUE` or `-s SECRET=value` for sensitive values. Supports reading from files with `@file` syntax and interactive prompts. ([#521](https://github.com/mirendev/runtime/pull/521))
- **Graceful shutdown during redeploy** - Apps now receive proper graceful shutdown time during redeploy instead of being force-killed. Configure per-service with `shutdown_timeout` in app.toml (default 10 seconds). ([#520](https://github.com/mirendev/runtime/pull/520))
- **`miren deploy cancel`** - Cancel stuck in-progress deployments without waiting for the 30-minute lock expiry. ([#517](https://github.com/mirendev/runtime/pull/517))
- **`miren debug bundle`** - New diagnostic command that collects system info, logs, container state, and process info into a tar.gz archive for troubleshooting. ([#531](https://github.com/mirendev/runtime/pull/531))
- **Domain allow list for TLS** - Automatic TLS certificate provisioning is now restricted to domains with explicitly configured routes, preventing certificate issuance for arbitrary domains pointed at the server. ([#542](https://github.com/mirendev/runtime/pull/542))

**Improvements**

- **Better app history display** - The `miren app history` command now shows deployment IDs (useful for `deploy cancel`) and improved status formatting. ([#529](https://github.com/mirendev/runtime/pull/529), [#532](https://github.com/mirendev/runtime/pull/532))

**Bug Fixes**

- **Fixed server restart killing all sandboxes** - Sandbox recovery no longer incorrectly kills all sandboxes when the server restarts. ([#546](https://github.com/mirendev/runtime/pull/546))
- **Fixed disk lease transfers** - Disk leases are now properly released when sandboxes stop, allowing new sandboxes to acquire them. ([#516](https://github.com/mirendev/runtime/pull/516))
- **Fixed sandbox exec issues** - Fixed panic when running `sandbox exec` without a TTY and fixed wrong entrypoint being applied to service containers. ([#515](https://github.com/mirendev/runtime/pull/515), [#518](https://github.com/mirendev/runtime/pull/518))
- **Fixed deployment panic handling** - Panics during deployment are now properly marked as failed instead of leaving deployments stuck. ([#513](https://github.com/mirendev/runtime/pull/513))
- **Fixed WebSocket 502 errors** - WebSocket connections no longer fail with 502 Bad Gateway. ([#507](https://github.com/mirendev/runtime/pull/507))
- **Fixed cluster switch for multi-identity setups** - Improved identity handling and UX when switching between clusters. ([#535](https://github.com/mirendev/runtime/pull/535))

---

## v0.2.1
*2025-12-19*

**Improvements**

- **Improved `miren doctor`** - The diagnostic command now suggests commands for you to run instead of running them automatically. Includes better guidance for config, auth, and server issues, plus clickable routes in app listings. ([#491](https://github.com/mirendev/runtime/pull/491))
- **Smarter install defaults** - `miren server install` and `miren download` now default to the version matching your binary instead of always using `main`. ([#500](https://github.com/mirendev/runtime/pull/500), [#502](https://github.com/mirendev/runtime/pull/502))

**Bug Fixes**

- **Fixed scale-to-zero pool deletion** - Pools for apps at scale-to-zero are no longer prematurely deleted after being idle for an hour, which was causing "pool has reached maximum size" errors when traffic resumed. ([#497](https://github.com/mirendev/runtime/pull/497))
- **Fixed activator cache cleanup** - The activator now proactively cleans up cached pool references when pools are deleted, preventing stale cache errors. ([#498](https://github.com/mirendev/runtime/pull/498))
- **Fixed disk debug commands** - `miren debug disk status` and related commands now correctly parse disk IDs. ([#499](https://github.com/mirendev/runtime/pull/499))

---

## v0.2.0
*2025-12-17*

**Features**

- **`miren app run`** - Run commands in a one-off sandbox with your app's configuration. Great for debugging, migrations, or one-off tasks. ([#489](https://github.com/mirendev/runtime/pull/489))
- **Persistent BuildKit daemon** - Builds are now significantly faster thanks to a persistent BuildKit daemon that maintains layer caching across builds. No more cold starts! ([#490](https://github.com/mirendev/runtime/pull/490))
- **`miren doctor` command** - New diagnostic command to help troubleshoot your Miren setup. Includes `miren doctor apps` to check app status and `miren doctor auth` to verify authentication. ([#484](https://github.com/mirendev/runtime/pull/484))
- **`miren deploy --analyze`** - Preview what Miren will detect about your app before actually building it. Great for understanding how your project will be configured. ([#485](https://github.com/mirendev/runtime/pull/485))
- **Rust and uv support** - Miren now auto-detects Rust projects and Python projects using uv, and builds them appropriately. ([#485](https://github.com/mirendev/runtime/pull/485))
- **Log filtering** - Filter logs by service name with `miren logs --service <name>` and by content with `miren logs -g <pattern>`. Also includes a faster chunked log streaming API under the hood. ([#487](https://github.com/mirendev/runtime/pull/487), [#466](https://github.com/mirendev/runtime/pull/466))
- **Debug networking commands** - New `miren debug netdb` commands for inspecting IP allocations and cleaning up orphaned leases. Helpful for advanced troubleshooting. ([#478](https://github.com/mirendev/runtime/pull/478))

**Bug Fixes**

- **Fixed IP address leaks** - Resolved several issues where IP addresses could leak during sandbox lifecycle events, container cleanup, and entity patch failures. ([#479](https://github.com/mirendev/runtime/pull/479))
- **Fixed stale pool reference** - Deleting and recreating an IP pool no longer causes "error acquiring lease" failures. ([#483](https://github.com/mirendev/runtime/pull/483))
- **Fixed LSVD write handling** - LSVD now uses proper Go file writes instead of raw unix calls, improving reliability. ([#477](https://github.com/mirendev/runtime/pull/477))
- **Fixed deployment cancellation race** - Cancelling a deploy with Ctrl-C no longer causes a race condition between the main and UI goroutines. ([#482](https://github.com/mirendev/runtime/pull/482))
- **Fixed authentication bypass** - Local/non-cloud mode now properly requires client certificates. ([#469](https://github.com/mirendev/runtime/pull/469))
- **Fixed entity revision check** - Entity patches no longer incorrectly enforce revision checks when `fromRevision` is 0. ([#470](https://github.com/mirendev/runtime/pull/470))
- **Fixed IPv6 environments** - VictoriaMetrics and VictoriaLogs now listen on IPv6, fixing issues in environments with IPv6 enabled. ([#481](https://github.com/mirendev/runtime/pull/481))

**Documentation**

- Updated system requirements to 4GB RAM and 20GB disk ([#480](https://github.com/mirendev/runtime/pull/480))
- Improved getting started documentation ([#471](https://github.com/mirendev/runtime/pull/471))
- Fixed missing pages in docs sidebar navigation ([#467](https://github.com/mirendev/runtime/pull/467))

---

## v0.1.0
*2025-12-09*

Initial preview release.
