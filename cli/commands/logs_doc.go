package commands

const logsDescription = `## Subcommands

` + "`" + `miren logs` + "`" + ` has subcommands for different log sources:

` + "```" + `bash
miren logs app       # Application logs (default)
miren logs sandbox   # Sandbox logs
miren logs build     # Build logs
miren logs system    # System/server logs
` + "```" + `

Running ` + "`" + `miren logs` + "`" + ` without a subcommand shows app logs (backward compatible).

## Time Range

By default, logs show the last 100 lines. Use ` + "`" + `--last` + "`" + ` to specify a time range:

` + "```" + `bash
# Show logs from the last 5 minutes
miren logs --last 5m

# Show logs from the last hour
miren logs --last 1h

# Show logs from the last 24 hours
miren logs --last 24h
` + "```" + `

## Following Logs

Use ` + "`" + `--follow` + "`" + ` (or ` + "`" + `-f` + "`" + `) to stream logs in real-time:

` + "```" + `bash
# Follow logs as they arrive
miren logs -f

# Follow logs for a specific app
miren logs app -a myapp -f
` + "```" + `

## Filtering by Service

Use ` + "`" + `--service` + "`" + ` to filter logs by service name (app logs only):

` + "```" + `bash
# Show only logs from the web service
miren logs app --service web

# Show worker service logs containing "error"
miren logs app --service worker -g error
` + "```" + `

## Filtering Logs

Use the ` + "`" + `--grep` + "`" + ` (or ` + "`" + `-g` + "`" + `) flag to filter log output. The filter supports multiple syntax options for flexible searching.

### Filter Syntax

| Syntax | Description | Example |
|--------|-------------|---------|
| ` + "`" + `word` + "`" + ` | Match logs containing "word" (case-insensitive) | ` + "`" + `error` + "`" + ` |
| ` + "`" + `"phrase"` + "`" + ` | Match logs containing exact phrase | ` + "`" + `"connection failed"` + "`" + ` |
| ` + "`" + `'phrase'` + "`" + ` | Match logs containing exact phrase (alternate) | ` + "`" + `'connection failed'` + "`" + ` |
| ` + "`" + `/regex/` + "`" + ` | Match logs matching regex pattern | ` + "`" + `/err(or)?/` + "`" + ` |
| ` + "`" + `-term` + "`" + ` | Exclude logs matching term | ` + "`" + `-debug` + "`" + ` |
| ` + "`" + `term1 term2` + "`" + ` | Match logs containing ALL terms (AND) | ` + "`" + `error timeout` + "`" + ` |

### Filter Details

- **Case-insensitive**: All word and phrase matches are case-insensitive
- **AND logic**: Multiple terms must all match for a log line to be included
- **Negation**: Prefix any term with ` + "`" + `-` + "`" + ` to exclude matching lines
- **Quotes**: Use double (` + "`" + `"` + "`" + `) or single (` + "`" + `'` + "`" + `) quotes for phrases with spaces
- **Regex**: Enclose patterns in forward slashes (` + "`" + `/pattern/` + "`" + `) for regex matching

## Log Output Format

Log entries are displayed with the following format:

` + "```" + `
S 2024-01-15 10:30:45: [source] Log message here
` + "```" + `

- **Stream prefix**: ` + "`" + `S` + "`" + ` (stdout), ` + "`" + `E` + "`" + ` (stderr), ` + "`" + `ERR` + "`" + ` (error), ` + "`" + `U` + "`" + ` (user-oob)
- **Timestamp**: Date and time when the log was generated
- **Source**: Optional source identifier (sandbox ID, truncated if long)
- **Message**: The actual log content`
