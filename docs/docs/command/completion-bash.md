---
title: "miren completion bash"
sidebar_label: "completion bash"
description: "Output a bash completion script"
---

# miren completion bash

Output a bash completion script

## Usage

```bash
miren completion bash [flags]
```

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Load completion in the current shell:**

```bash
source <(miren completion bash)
```

**Install for all sessions:**

```bash
miren completion bash > /etc/bash_completion.d/miren
```

## See also

- [`miren completion`](/command/completion)
