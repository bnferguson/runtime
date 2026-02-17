package commands

const sandboxSectionDescription = `Sandboxes are the underlying execution environments for your applications. Most of the time you'll work with apps directly, but these commands are useful for debugging and advanced use cases.`

const sandboxExecDescription = `This command connects to an existing sandbox and runs a command inside it. Unlike ` + "`" + `miren app run` + "`" + ` which creates a new ephemeral sandbox, this connects to a sandbox that's already running (typically one serving production traffic).

### Finding Sandbox IDs

Use ` + "`" + `miren sandbox list` + "`" + ` to find the ID of a running sandbox:

` + "```" + `bash
$ miren sandbox list
ID                          APP       SERVICE   STATUS    NODE
sandbox/myapp-web-abc123    myapp     web       RUNNING   node-1
sandbox/myapp-web-def456    myapp     web       RUNNING   node-2
` + "```" + `

:::warning
When you exec into a production sandbox, you're connecting to a live instance that may be serving traffic. Be careful with commands that could affect the running application.
:::

:::tip
For debugging or one-off tasks without affecting production, use ` + "`" + `miren app run` + "`" + ` to create an isolated ephemeral sandbox instead.
:::`
