---
sidebar_position: 6
---

# Admin Commands

Commands for calling admin functions on your applications.

The admin interface allows you to execute custom administrative functions in your running application—useful for user management, cache clearing, database operations, and other maintenance tasks.

## miren admin

Call an admin method on an application.

### Usage

```bash
miren admin [flags] <method> [key=value ...]
```

### Flags

- `--app, -a` - Application name (or use `$MIREN_APP` environment variable)
- `--list, -l` - List available admin methods
- `--json, -j` - Output as highlighted JSON (default for non-TTY)
- `--pretty, -p` - Render output in a human-friendly format (default for TTY)
- `--params-file, -f` - Read params as JSON from file (use `-` for stdin)
- `--no-validate` - Skip method/parameter validation
- `--cluster, -C` - Cluster name

### Examples

```bash
# List available admin methods
miren admin --list

# Call an admin method with no parameters
miren admin get-stats

# Call a method with parameters
miren admin get-user user_id=user-123

# Call a method with complex JSON parameter
miren admin update-config settings='{"debug": true, "timeout": 30}'

# Force JSON output (useful for scripting)
miren admin --json get-stats

# Force pretty output
miren admin --pretty get-user user_id=user-123

# Read params from a JSON file
miren admin update-config -f config.json

# Read params from stdin
echo '{"user_id": "user-123"}' | miren admin get-user -f -
```

## Listing Methods

Use the `--list` flag to discover what admin methods your application exposes:

```bash
$ miren admin --list

Admin methods for go-admin

  clear-cache
  │ Clear the application cache

  delete-user
  │ Delete a user by ID
  └ user_id string

  get-stats
  │ Get application statistics

  get-user
  │ Get a specific user by ID
  └ user_id string

  list-users
  │ List all users in the system
  ├ limit number
  └ offset number

Usage: miren admin -a go-admin <method> [key=value ...]
```

## Parameter Validation

By default, `miren admin` validates method names and parameters against the application's introspection data before making the call. This catches typos and missing required parameters early.

To skip validation (e.g., if your app doesn't support introspection), use `--no-validate`:

```bash
miren admin --no-validate some-method
```

## Output Formats

The admin command automatically chooses the output format based on context:

- **TTY (terminal)**: Uses a human-friendly pretty format by default
- **Non-TTY (pipes, scripts)**: Uses highlighted JSON by default

You can override this behavior:

```bash
# Force JSON output for scripting
miren admin --json get-stats | jq '.total_users'

# Force pretty output even when piping
miren admin --pretty get-user user_id=123
```

## Error Handling

If the admin call fails, the command exits with a non-zero status and displays the error:

```bash
$ miren admin get-user user_id=nonexistent
admin call failed (code -32001): user not found
```

Error codes follow JSON-RPC conventions:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- Custom codes (negative numbers): Application-specific errors

## Next Steps

- [Admin Interface Guide](/admin-interface) - Learn how to implement admin methods in your app
- [CLI Reference](/cli-reference) - See all available commands
