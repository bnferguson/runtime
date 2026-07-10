---
title: Agent Skills
description: Use AI coding agents to deploy, diagnose, and manage your Miren infrastructure with pre-built skills for Claude Code, Codex, Amp, and more.
keywords: [miren, agent skills, claude code, codex, amp, ai, llm]
---

# Agent Skills

You shouldn't have to context-switch out of your editor to deploy an app or check why something's unhealthy. Miren's agent skills let your AI coding agent operate your infrastructure directly — deploy, diagnose, and manage apps without leaving the conversation.

The skills work with [Claude Code](https://claude.ai/code), [Codex](https://github.com/openai/codex), [Amp](https://ampcode.com), [Pi](https://pi.dev), and [OpenCode](https://github.com/opencode-ai/opencode). Source and setup instructions are at [github.com/mirendev/miren-skills](https://github.com/mirendev/miren-skills).

:::note[Docs are the source of truth]
Skills make these docs faster to act on — your agent can read a page about scaling and immediately run the commands — but the docs remain the authoritative reference. When in doubt, the docs are the source of truth.
:::

## Installation

### Claude Code

```bash
/plugin marketplace add mirendev/miren-skills
/plugin install miren@miren
```

### Codex CLI

```bash
git clone https://github.com/mirendev/miren-skills
cp -r miren-skills/.agents/skills/* ~/.agents/skills/
```

### Amp

From the command palette (`Ctrl+O` in CLI, `Cmd+Shift+P` in VS Code):

```text
skill: add https://github.com/mirendev/miren-skills
```

### Pi

```bash
pi install git:github.com/mirendev/miren-skills
```

### OpenCode

```bash
git clone https://github.com/mirendev/miren-skills
cp -r miren-skills/.agents/skills/* ~/.config/opencode/skills/
```

## What's included

### `use-miren`

The core skill. Once installed, your agent knows how to use the `miren` CLI — it discovers commands via `miren help`, targets clusters with `-C`, and uses `--json` output for reliable parsing. You don't need to teach it anything; just mention Miren and it kicks in.

### `app-setup`

Getting a new app onto Miren means figuring out what it needs — env vars, databases, services, build config — and wiring it all up. This agent does the detective work for you. Point it at your source code and it walks you through the whole setup, from stack detection to a working `.miren/app.toml`.

Try asking:
- "Help me set up this app on Miren"
- "What does this app need to run?"

### `app-health`

Instead of piecing together app status from multiple commands, ask your agent to check on an app. It pulls together service states, deployment history, logs, and diagnostics into a single report with actionable recommendations. Defaults to the app in your current directory.

Try asking:
- "How's this app doing?"
- "Check the health of myapp"

### `cluster-health`

Same idea, but across your whole cluster. Surveys every app and service, then gives you a prioritized breakdown — what's healthy, what needs attention, and what to do about it.

Try asking:
- "How's the cluster looking?"
- "Give me a health check on garden"

## Commands reference

The skills teach agents to discover commands on their own via `miren help`:

```bash
miren help --commands             # list all commands
miren help app list               # help for a specific command
```

Most commands accept `-C <cluster>` to target a specific cluster.
