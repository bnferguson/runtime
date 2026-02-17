---
sidebar_position: 7
---

# Admin Interface

:::info Labs Feature
The admin interface is a [labs feature](/labs) and is disabled by default. Enable it with `--labs adminapi` or `MIREN_LABS=adminapi` when starting the server.
:::

The admin interface allows you to expose custom administrative functions in your application that can be called from the CLI or other tooling. This is useful for user management, cache clearing, database operations, and other maintenance tasks.

## How It Works

Your application exposes admin methods via a JSON-RPC 2.0 endpoint at a well-known path. When you run `miren admin`, the CLI:

1. Looks up your app and retrieves the admin token
2. Sends a JSON-RPC request to your app's web service
3. Returns the response (or error) to you

```text
miren admin delete-user user_id=123 --app myapp
         |
         v
    JSON-RPC POST to /.well-known/miren/admin
    Authorization: Bearer <ADMIN_TOKEN>
    {"jsonrpc":"2.0","method":"delete-user","params":{"user_id":"123"},"id":1}
         |
         v
    Your app processes the request and returns a response
```

## Implementing the Admin Endpoint

### Endpoint Requirements

Your web service must expose:

| Requirement | Value |
|-------------|-------|
| Path | `/.well-known/miren/admin` |
| Method | POST |
| Content-Type | `application/json` |
| Protocol | JSON-RPC 2.0 |

### Security

Admin calls are authenticated using a bearer token that Miren generates for your app. Your app receives this token via the `ADMIN_TOKEN` environment variable.

**You must validate this token on every request:**

```go
func authMiddleware(token string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if token != "" {
            auth := r.Header.Get("Authorization")
            if !strings.HasPrefix(auth, "Bearer ") {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            if strings.TrimPrefix(auth, "Bearer ") != token {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

The admin endpoint also receives an `X-Miren-Access` header:
- `internal` — Request is from Miren's admin system (trusted)
- `public` — Request is from an external client (Miren strips any client-provided value)

You can use this header for additional access control if your endpoint is accidentally exposed to the internet.

### JSON-RPC 2.0 Format

Requests follow the standard JSON-RPC 2.0 format:

```json
{
  "jsonrpc": "2.0",
  "method": "method-name",
  "params": {"key": "value"},
  "id": 1
}
```

Successful responses:

```json
{
  "jsonrpc": "2.0",
  "result": {"any": "data"},
  "id": 1
}
```

Error responses:

```json
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32001,
    "message": "user not found",
    "data": {"user_id": "123"}
  },
  "id": 1
}
```

### Standard Error Codes

| Code | Meaning |
|------|---------|
| -32700 | Parse error (invalid JSON) |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |
| < 0 | Application-specific errors |

## Method Introspection

When you run `miren admin --list`, Miren sends a JSON-RPC request with the reserved method name `$methods` to your admin endpoint. If your app handles this method, it should return an array of objects describing the available admin methods. This is optional — if your app doesn't handle `$methods`, the `--list` command will report an error, but regular method calls still work.

The `$methods` request has no params:

```json
{
  "jsonrpc": "2.0",
  "method": "$methods",
  "id": 1
}
```

Response:

```json
{
  "jsonrpc": "2.0",
  "result": [
    {
      "name": "list-users",
      "description": "List all users in the system",
      "category": "users",
      "params": {"limit": "number", "offset": "number"}
    },
    {
      "name": "get-user",
      "description": "Get a specific user by ID",
      "category": "users",
      "params": {"user_id": "string"}
    },
    {
      "name": "clear-cache",
      "description": "Clear the application cache",
      "category": "maintenance"
    }
  ],
  "id": 1
}
```

Method metadata fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Method name |
| `description` | No | Human-readable description |
| `category` | No | Grouping for display (e.g., "users", "maintenance") |
| `params` | No | Parameter definitions as `{"name": "type"}` |

## Complete Example (Go)

Here's a complete example using the `jsonrpc3` library:

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "strings"

    "miren.dev/jsonrpc3/go/jsonrpc3"
)

func main() {
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }

    adminToken := os.Getenv("ADMIN_TOKEN")
    if adminToken == "" {
        log.Println("WARNING: ADMIN_TOKEN not set")
    }

    // Create the admin handler with method definitions
    adminMethods := jsonrpc3.NewMethodMap()

    // Register admin methods with introspection metadata
    adminMethods.Register("list-users", listUsers,
        jsonrpc3.WithDescription("List all users in the system"),
        jsonrpc3.WithParams(map[string]string{
            "limit":  "number",
            "offset": "number",
        }),
        jsonrpc3.WithCategory("users"),
    )

    adminMethods.Register("get-user", getUser,
        jsonrpc3.WithDescription("Get a specific user by ID"),
        jsonrpc3.WithParams(map[string]string{
            "user_id": "string",
        }),
        jsonrpc3.WithCategory("users"),
    )

    adminMethods.Register("clear-cache", clearCache,
        jsonrpc3.WithDescription("Clear the application cache"),
        jsonrpc3.WithCategory("maintenance"),
    )

    // Create the HTTP handler for JSON-RPC
    rpcHandler := jsonrpc3.NewHTTPHandler(adminMethods)

    // Wrap with auth middleware
    authHandler := authMiddleware(adminToken, rpcHandler)

    // Mount admin endpoint at well-known path
    http.Handle("/.well-known/miren/admin", authHandler)

    log.Printf("Starting server on port %s", port)
    log.Fatal(http.ListenAndServe(":"+port, nil))
}

func authMiddleware(token string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if token != "" {
            auth := r.Header.Get("Authorization")
            if !strings.HasPrefix(auth, "Bearer ") {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
            if strings.TrimPrefix(auth, "Bearer ") != token {
                http.Error(w, "Unauthorized", http.StatusUnauthorized)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}

// Sample data
var users = map[string]map[string]any{
    "user-1": {"id": "user-1", "name": "Alice", "email": "alice@example.com"},
    "user-2": {"id": "user-2", "name": "Bob", "email": "bob@example.com"},
}

type listUsersParams struct {
    Limit  int `json:"limit"`
    Offset int `json:"offset"`
}

func listUsers(ctx context.Context, params jsonrpc3.Params, caller jsonrpc3.Caller) (any, error) {
    p := listUsersParams{Limit: 10, Offset: 0}
    if params != nil {
        _ = params.Decode(&p)
    }

    var result []map[string]any
    for _, user := range users {
        result = append(result, user)
    }

    return map[string]any{
        "users": result,
        "total": len(users),
    }, nil
}

type userIDParams struct {
    UserID string `json:"user_id"`
}

func getUser(ctx context.Context, params jsonrpc3.Params, caller jsonrpc3.Caller) (any, error) {
    var p userIDParams
    if params == nil {
        return nil, jsonrpc3.NewInvalidParamsError("user_id is required")
    }
    if err := params.Decode(&p); err != nil {
        return nil, jsonrpc3.NewInvalidParamsError("invalid params")
    }
    if p.UserID == "" {
        return nil, jsonrpc3.NewInvalidParamsError("user_id is required")
    }

    user, ok := users[p.UserID]
    if !ok {
        return nil, jsonrpc3.NewError(-32001, "user not found", nil)
    }

    return user, nil
}

func clearCache(ctx context.Context, params jsonrpc3.Params, caller jsonrpc3.Caller) (any, error) {
    // Your cache clearing logic here
    return map[string]any{
        "cleared": true,
    }, nil
}
```

## Other Languages

The admin interface is language-agnostic. Any language that can:

1. Handle HTTP POST requests
2. Parse and generate JSON
3. Implement the JSON-RPC 2.0 protocol

can expose admin methods. The key requirements are:

- Listen on `/.well-known/miren/admin`
- Validate the `Authorization: Bearer <token>` header against `ADMIN_TOKEN`
- Handle JSON-RPC requests and return proper responses
- Optionally implement `$methods` for introspection

### Python Example

```python
from flask import Flask, request, jsonify
import os

app = Flask(__name__)
ADMIN_TOKEN = os.environ.get('ADMIN_TOKEN', '')

@app.route('/.well-known/miren/admin', methods=['POST'])
def admin_endpoint():
    # Validate token
    auth = request.headers.get('Authorization', '')
    if ADMIN_TOKEN and auth != f'Bearer {ADMIN_TOKEN}':
        return 'Unauthorized', 401

    data = request.json
    method = data.get('method')
    params = data.get('params', {})
    req_id = data.get('id')

    # Handle introspection
    if method == '$methods':
        return jsonify({
            'jsonrpc': '2.0',
            'result': [
                {'name': 'get-stats', 'description': 'Get app statistics'},
            ],
            'id': req_id
        })

    # Handle your methods
    if method == 'get-stats':
        return jsonify({
            'jsonrpc': '2.0',
            'result': {'users': 42, 'requests': 1000},
            'id': req_id
        })

    return jsonify({
        'jsonrpc': '2.0',
        'error': {'code': -32601, 'message': 'Method not found'},
        'id': req_id
    })
```

## Calling Admin Methods

Once your app exposes the admin interface, use the CLI to call methods:

```bash
# List available methods
miren admin --list

# Call a method
miren admin get-user user_id=user-1

# Call with complex parameters
miren admin update-config settings='{"debug": true}'

# Output as JSON (for scripting)
miren admin get-stats --json | jq '.total'
```

See [Admin Commands](/command/admin) for full CLI documentation.

## Next Steps

- [Admin Commands](/command/admin) — CLI reference for `miren admin`
- [Services](/services) — Configure your app's web service
- [Getting Started](/getting-started) — Deploy your first app
