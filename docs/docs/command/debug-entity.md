---
title: "miren debug entity"
sidebar_label: "debug entity"
description: "Entity store debug commands"
---

# miren debug entity

Entity store debug commands

Entities are the low-level objects stored in Miren's entity system. Most users won't need to use these commands directly. They're primarily useful for debugging and advanced use cases.

## What are Entities?

Entities are flexible metadata objects stored in Miren's etcd-backed entity store. Everything in Miren is an entity:

- **Apps** - Application definitions
- **Sandboxes** - Running containers
- **Versions** - Immutable app configurations
- **Clusters** - Cluster registrations
- **Users** - User accounts

## Usage

```bash
miren debug entity [flags]
```

## Subcommands

- [`miren debug entity create`](/command/debug-entity-create) — Create a new entity
- [`miren debug entity delete`](/command/debug-entity-delete) — Delete an entity
- [`miren debug entity ensure`](/command/debug-entity-ensure) — Ensure an entity exists
- [`miren debug entity get`](/command/debug-entity-get) — Get an entity
- [`miren debug entity list`](/command/debug-entity-list) — List entities
- [`miren debug entity patch`](/command/debug-entity-patch) — Patch an existing entity
- [`miren debug entity put`](/command/debug-entity-put) — Put an entity
- [`miren debug entity replace`](/command/debug-entity-replace) — Replace an existing entity

## See also

- [`miren debug`](/command/debug)
