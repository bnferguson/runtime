---
title: "miren runner"
sidebar_label: "runner"
description: "Runner management commands"
---

# miren runner

Runner management commands

## Usage

```bash
miren runner [flags]
```

## Subcommands

- [`miren runner cordon`](/command/runner-cordon) — Mark a runner unschedulable without stopping its sandboxes
- [`miren runner drain`](/command/runner-drain) — Cordon a runner and evict its sandboxes onto other nodes
- [`miren runner install`](/command/runner-install) — Install systemd service for miren runner
- [`miren runner join`](/command/runner-join) — Join this machine to a coordinator as a runner
- [`miren runner list`](/command/runner-list) — List all registered runners
- [`miren runner remove`](/command/runner-remove) — Remove a registered runner and clean up resources
- [`miren runner service-status`](/command/runner-service-status) — Show miren-runner systemd service status
- [`miren runner start`](/command/runner-start) — Start this machine as a distributed runner
- [`miren runner status`](/command/runner-status) — Show runner health and configuration
- [`miren runner token`](/command/runner-token) — Manage join tokens
- [`miren runner uncordon`](/command/runner-uncordon) — Make a cordoned runner eligible for scheduling again
- [`miren runner uninstall`](/command/runner-uninstall) — Remove systemd service for miren runner
- [`miren runner upgrade`](/command/runner-upgrade) — Upgrade miren runner to the latest or specified version
