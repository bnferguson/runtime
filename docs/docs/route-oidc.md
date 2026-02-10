---
sidebar_position: 8
---

# Route OIDC Authentication

:::info Labs Feature
Route OIDC authentication is a [labs feature](/labs) and is disabled by default. Enable it with `--labs routeoidc` or `MIREN_LABS=routeoidc` when starting the server.
:::

Route OIDC authentication lets you protect your applications with single sign-on at the routing layer. Unauthenticated requests are redirected to an OIDC provider for login, and after authentication, JWT claims are injected as HTTP headers before the request reaches your app.

Your app receives identity information as plain HTTP headers (e.g., `X-User-Email`) — no in-app auth code required.

## Authentication vs Authorization

This feature handles **authentication** — verifying _who_ a user is — not **authorization** — deciding _what_ they can access.

After authentication, your app receives claims as headers and decides what to do with them. For example, if you configure Google as your OIDC provider, your app receives the user's email address and can check the domain for basic access control.

## Trust Model

Your app trusts the claim headers (e.g., `X-User-Email`) because Miren is the only network path into the sandbox — external clients cannot reach your app directly. Miren validates the JWT from the OIDC provider and sets the headers before proxying the request.

This "trust the proxy" model is the standard approach used by tools like OAuth2 Proxy, Traefik ForwardAuth, and nginx `auth_request`. It's simple and works well when the platform controls the network topology, which Miren does via sandbox isolation.

Your app does not need to verify signatures or validate tokens — it can treat the claim headers as trusted input from the platform.

## How Claim Mappings Work

The `--claim-header` option maps JWT claims from the OIDC provider to HTTP headers that your app receives:

```bash
miren route oidc enable myapp.example.com \
  --claim-header email:X-User-Email \
  --claim-header sub:X-User-ID \
  --claim-header name:X-User-Name
```

- Each `--claim-header` takes the form `CLAIM:HEADER`
- Multiple mappings can be specified
- Claims not present in the JWT are silently skipped
- Your app receives these as regular request headers

## Example Provider Setups

### Google

Google's OIDC provider works well when you want to restrict access by email domain.

```bash
miren route oidc enable myapp.example.com \
  --provider-url https://accounts.google.com \
  --client-id $GOOGLE_CLIENT_ID \
  --client-secret $GOOGLE_CLIENT_SECRET \
  --scope openid email profile \
  --claim-header email:X-User-Email \
  --claim-header name:X-User-Name
```

Your app can then check the `X-User-Email` header's domain for basic authorization (e.g., only allow `@yourcompany.com`).

### GitHub (via federation)

GitHub doesn't expose a native OIDC provider endpoint. To use GitHub for authentication, you'll need a federated OIDC provider like [Dex](https://dexidp.io/) that can use GitHub as an upstream identity source and encode org and team membership as JWT claims.

### GitLab

GitLab has a built-in OIDC provider — no federation layer needed. Register an application in your GitLab instance and point directly at it.

```bash
miren route oidc enable myapp.example.com \
  --provider-url https://gitlab.com \
  --client-id $GITLAB_CLIENT_ID \
  --client-secret $GITLAB_CLIENT_SECRET \
  --scope openid email profile \
  --claim-header email:X-User-Email \
  --claim-header name:X-User-Name
```

### Keycloak (self-hosted)

[Keycloak](https://www.keycloak.org/) is an open-source identity provider you can run yourself. It supports user federation, identity brokering, and fine-grained claim configuration.

```bash
miren route oidc enable myapp.example.com \
  --provider-url https://keycloak.example.com/realms/myrealm \
  --client-id $KEYCLOAK_CLIENT_ID \
  --client-secret $KEYCLOAK_CLIENT_SECRET \
  --scope openid email profile \
  --claim-header email:X-User-Email \
  --claim-header preferred_username:X-User-Name
```

## Reusing Providers Across Routes

The examples above create a new OIDC provider inline for each route. If you use the same provider for multiple routes, you can reference an existing provider by name instead of repeating the configuration:

```bash
miren route oidc enable another-app.example.com \
  --provider google-oauth \
  --claim-header email:X-User-Email
```

Providers created inline are preserved when you disable OIDC on a route, so they can be reused later.

## Default Route Support

All `miren route oidc` commands support the `--default` flag for the default route (which has no static hostname). When used with the default route, the OAuth2 redirect URL is derived from the request's `Host` header at runtime.

```bash
miren route oidc enable --default \
  --provider-url https://accounts.google.com \
  --client-id $GOOGLE_CLIENT_ID \
  --client-secret $GOOGLE_CLIENT_SECRET \
  --claim-header email:X-User-Email
```

## Managing OIDC on Routes

```bash
# Show current OIDC configuration for a route
miren route oidc show myapp.example.com

# Disable OIDC on a route (provider is preserved for reuse)
miren route oidc disable myapp.example.com
```

See the [CLI reference](/cli-reference#route-oidc-authentication) for the full list of options.
