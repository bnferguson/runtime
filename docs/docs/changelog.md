# Changelog

All notable changes to Miren Runtime will be documented in this file.

## Unreleased
*main*

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
