package commands

const entitySectionDescription = `Entities are the low-level objects stored in Miren's entity system. Most users won't need to use these commands directly. They're primarily useful for debugging and advanced use cases.

## What are Entities?

Entities are flexible metadata objects stored in Miren's etcd-backed entity store. Everything in Miren is an entity:

- **Apps** - Application definitions
- **Sandboxes** - Running containers
- **Versions** - Immutable app configurations
- **Clusters** - Cluster registrations
- **Users** - User accounts`

const entityPutDescription = `:::warning
This is an advanced command. Use the higher-level commands like ` + "`" + `miren deploy` + "`" + ` instead when possible.
:::`
