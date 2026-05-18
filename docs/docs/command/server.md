---
title: "miren server"
sidebar_label: "server"
description: "Start the miren server"
---

# miren server

Start the miren server

## Usage

```bash
miren server [flags]
```

## Flags

- `--acme-dns-provider` — DNS provider for ACME DNS-01 challenges (e.g., cloudflare, route53, exec). When set, uses DNS challenge instead of HTTP challenge. See https://go-acme.github.io/lego/dns/ for available providers.
- `--acme-email` — Email address for ACME account registration (recommended for account recovery and notifications)
- `--address, -a` — Address to listen on (host:port). For IPv6 use brackets, e.g. "[::1]:8443".
- `--buildkit-gc-duration` — How long to keep BuildKit cache entries (e.g., 7d, 24h)
- `--buildkit-gc-storage` — Maximum BuildKit layer cache size (e.g., 10GB, 50GB)
- `--buildkit-socket` — Path to external BuildKit Unix socket (for distributed mode)
- `--buildkit-socket-dir` — Directory for embedded BuildKit Unix socket (defaults to data_path/buildkit/socket)
- `--config` — Path to configuration file
- `--config-cluster-name, -C` — Name of the cluster in client config
- `--containerd-binary` — Path to containerd binary
- `--containerd-socket` — Path to containerd socket
- `--data-path, -d` — Data path
- `--disk-mode` — Disk I/O mode: auto (default, detect from hardware), universal (loop devices), or accelerator (lbd devices)
- `--dns-names` — Additional DNS names assigned to the server cert
- `--etcd, -e` — Etcd endpoints
- `--etcd-client-port` — Etcd client port
- `--etcd-http-client-port` — Etcd HTTP client port
- `--etcd-peer-port` — Etcd peer port
- `--etcd-prefix, -p` — Etcd prefix
- `--http-request-timeout` — HTTP request timeout in seconds
- `--ingress-address` — Optional bind override. Replaces the mode's default bind entirely (interface and port). Rejected by validation in tls-autoprovision (where :443 + :80 is structural). Reserved unix:/path prefix is not yet supported.
- `--ingress-mode` — Ingress mode: tls-autoprovision (default, :443 + :80 with ACME or self-signed), behind-proxy-http (plain HTTP for use behind a TLS-terminating proxy), behind-proxy-https (TLS terminated by Miren; certs come from self-signed or DNS-01 ACME, since :80 isn't bound for HTTP-01)
- `--ips` — Additional IPs assigned to the server cert
- `--labs` — Comma-separated list of Miren Labs features to enable/disable. Prefix with - to disable.
- `--mode, -m` — Server mode: standalone (default), distributed (experimental)
- `--network-backend` — Network backend for sandbox connectivity: vxlan (default) or wireguard
- `--release-path` — Path to release directory containing binaries
- `--runner-address` — Runner address (host:port). For IPv6 use brackets, e.g. "[::1]:8444".
- `--runner-id, -r` — Runner ID
- `--self-signed-tls` — Use self-signed certificates for TLS (for development/testing only)
- `--serve-tls` — Deprecated and ignored. Retained as a no-op so existing systemd unit files, env vars, and config files from pre-RFD-84 installs still parse. Use ingress.mode to pick the deployment shape.
- `--skip-client-config` — Skip writing client config file to clientconfig.d
- `--start-buildkit` — Start embedded BuildKit daemon for container image builds
- `--start-containerd` — Start embedded containerd daemon
- `--start-etcd` — Start embedded etcd server
- `--start-victorialogs` — Start embedded VictoriaLogs server
- `--start-victoriametrics` — Start embedded VictoriaMetrics server
- `--stop-sandboxes-on-shutdown` — Stop all sandboxes when server shuts down (useful in development)
- `--victorialogs-addr` — VictoriaLogs address (when not using embedded)
- `--victorialogs-http-port` — VictoriaLogs HTTP port in embedded mode
- `--victorialogs-retention` — VictoriaLogs retention period (e.g. 30d, 2w, 1y)
- `--victoriametrics-addr` — VictoriaMetrics address (when not using embedded)
- `--victoriametrics-http-port` — VictoriaMetrics HTTP port in embedded mode
- `--victoriametrics-retention` — VictoriaMetrics retention period in months

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Start in standalone mode:**

```bash
miren server --mode standalone
```

## Subcommands

- [`miren server config`](/command/server-config) — Server configuration management commands
- [`miren server docker`](/command/server-docker) — Docker-based server management commands
- [`miren server install`](/command/server-install) — Install systemd service for miren server
- [`miren server register`](/command/server-register) — Register this cluster with miren.cloud
- [`miren server status`](/command/server-status) — Show miren service status
- [`miren server uninstall`](/command/server-uninstall) — Remove systemd service for miren server
- [`miren server upgrade`](/command/server-upgrade) — Upgrade miren server
