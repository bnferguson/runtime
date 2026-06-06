---
title: "miren completion fish"
sidebar_label: "completion fish"
description: "Output a fish completion script"
---

# miren completion fish

Output a fish completion script

## Usage

```bash
miren completion fish [flags]
```

## Global Options

- `--options` — Path to file containing options
- `--server-address` — Server address to connect to (default: `127.0.0.1:8443`)
- `--verbose, -v` — Enable verbose output

## Examples

**Load completion in the current shell:**

```bash
miren completion fish | source
```

**Install for all sessions:**

```bash
miren completion fish > ~/.config/fish/completions/miren.fish
```

## See also

- [`miren completion`](/command/completion)
