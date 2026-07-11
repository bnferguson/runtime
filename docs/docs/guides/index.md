---
title: Language Guides
description: Step-by-step guides for getting Python, JavaScript, Go, Ruby, Elixir, and Gleam apps running on Miren.
keywords: [guides, languages, python, javascript, node, bun, go, ruby, elixir, gleam, deploy]
---

# Language Guides

These guides take you from a project on your laptop to a running app on Miren, one
language at a time. Each one covers the same three things: **how to set up the app**,
**how to set environment variables**, and **whether you need a Dockerfile**.

:::tip[Let your agent do this]
If you use an AI coding agent (Claude Code, Codex, Amp, and others), you don't have to
follow these guides by hand. Install the [Miren agent skills](/agent-skills) and ask
your agent to "set up this app on Miren" — it reads your project, detects the stack,
wires up environment variables, and deploys. These guides double as the reference your
agent works from. See [Agent Skills](/agent-skills) for setup.
:::

## Pick your language

| Guide | Auto-detected? | You provide |
|-------|----------------|-------------|
| [Python](/guides/python) | Yes | `requirements.txt` / `pyproject.toml` / `Pipfile` / `uv.lock` |
| [JavaScript (Node & Bun)](/guides/javascript) | Yes | `package.json` + a lockfile |
| [Go](/guides/go) | Yes | `go.mod` |
| [Ruby](/guides/ruby) | Yes | `Gemfile` |
| [Rust](/guides/rust) | Yes | `Cargo.toml` |
| [Elixir](/guides/elixir) | No | `Dockerfile.miren` |
| [Gleam](/guides/gleam) | No | `Dockerfile.miren` |
| [Crystal](/guides/crystal) | No | `Dockerfile.miren` |
| [Zig](/guides/zig) | No | `Dockerfile.miren` |
| [Deno](/guides/deno) | No | `Dockerfile.miren` |
| [Nim](/guides/nim) | No | `Dockerfile.miren` |
| [C](/guides/c) | No | `Dockerfile.miren` |
| [C++](/guides/cpp) | No | `Dockerfile.miren` |
| [Objective-C](/guides/objc) | No | `Dockerfile.miren` |
| [.NET / C#](/guides/dotnet) | No | `Dockerfile.miren` |
| [F#](/guides/fsharp) | No | `Dockerfile.miren` |
| [Java / JVM](/guides/java) | No | `Dockerfile.miren` |
| [Kotlin](/guides/kotlin) | No | `Dockerfile.miren` |
| [Scala](/guides/scala) | No | `Dockerfile.miren` |
| [Clojure](/guides/clojure) | No | `Dockerfile.miren` |
| [Erlang](/guides/erlang) | No | `Dockerfile.miren` |
| [PHP](/guides/php) | No | `Dockerfile.miren` |
| [Perl](/guides/perl) | No | `Dockerfile.miren` |
| [Raku](/guides/raku) | No | `Dockerfile.miren` |
| [OCaml](/guides/ocaml) | No | `Dockerfile.miren` |
| [Haskell](/guides/haskell) | No | `Dockerfile.miren` |
| [Swift](/guides/swift) | No | `Dockerfile.miren` |
| [Dart](/guides/dart) | No | `Dockerfile.miren` |
| [JRuby](/guides/jruby) | No | `Dockerfile.miren` |
| [TruffleRuby](/guides/truffleruby) | No | `Dockerfile.miren` |
| [Julia](/guides/julia) | No | `Dockerfile.miren` |
| [R](/guides/r) | No | `Dockerfile.miren` |
| [Lua](/guides/lua) | No | `Dockerfile.miren` |
| [Common Lisp](/guides/commonlisp) | No | `Dockerfile.miren` |
| [COBOL](/guides/cobol) | No | `Dockerfile.miren` |
| [Bash](/guides/bash) | No | `Dockerfile.miren` |
| [Static sites & SPAs](/guides/static) | No | `Dockerfile.miren` |

## Auto-detected vs. Dockerfile

For most languages, Miren detects your stack from your project files and builds a
container image for you — **no Dockerfile required**. You run `miren init` once and
`miren deploy`, and Miren figures out the rest. See [Supported Languages](/languages)
for exactly what's detected and how.

Every other language here — from Elixir and Gleam to Kotlin, Swift, Julia, and even
COBOL — isn't auto-detected, so its guide shows you a `Dockerfile.miren` you
can drop into your project. Miren builds from that Dockerfile instead of guessing. This
is the same escape hatch available to every language when you need full control over the
build — see [Using Dockerfile.miren](/languages#using-dockerfilemiren).

Every Dockerfile-based guide also notes one Miren-specific rule: even with a
`Dockerfile.miren`, you must define at least one service (a `Procfile` or a
`[services.web]` block) — Miren doesn't use the image's `CMD` as the start command.

## What every guide assumes

- You've installed Miren and can reach a cluster. If not, start with
  [Getting Started](/getting-started).
- Your web service **binds to `0.0.0.0` on the port in the `PORT` environment
  variable**. Miren injects `PORT` at runtime and routes traffic to it. An app that
  hardcodes `localhost` or a fixed port won't receive traffic.

## Next steps

- [Deployment](/deployment) — how `miren deploy` builds and activates versions
- [App Configuration](/app-configuration) — customize with `.miren/app.toml`
- [Services](/services) — run workers and multiple processes
- [Agent Skills](/agent-skills) — let your agent operate Miren for you
