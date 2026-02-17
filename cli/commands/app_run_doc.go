package commands

const appRunDescription = `This command creates a temporary sandbox using your app's configuration (image, environment variables, working directory) and connects you to an interactive shell. The sandbox is automatically cleaned up when you exit.

This is useful for:
- Debugging application issues in an isolated environment
- Running one-off commands with your app's configuration
- Exploring the container filesystem
- Testing changes before deploying

### How It Works

1. Miren fetches your app's active version configuration
2. Creates an ephemeral sandbox with the same image, environment variables, and working directory as your deployed app
3. Waits for the sandbox to become ready
4. Connects your terminal to an interactive shell inside the sandbox
5. Cleans up the sandbox automatically when you disconnect

:::tip
The ephemeral sandbox runs independently from your production sandboxes. Any changes you make (files created, packages installed) are discarded when you exit.
:::

:::note
If you need to run commands in an existing production sandbox, use ` + "`" + `miren sandbox exec` + "`" + ` instead.
:::`
