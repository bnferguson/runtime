---
title: Protecting Routes
description: Add single sign-on to your app's HTTP routes using OIDC, with identity passed as headers.
keywords: [route protection, oidc, sso, authentication, single sign-on, identity provider]
---

import CliCommand from '@site/src/components/CliCommand';

# Protecting Routes

:::tip Looking for CI/CD OIDC?
If you want to **deploy from GitHub Actions or other CI systems** using OIDC tokens (no stored secrets), see [CI/CD Deployment with OIDC](/ci-deploy). This page covers a different feature — protecting your app's HTTP routes with single sign-on.
:::

Route protection lets you put single sign-on in front of an application at the routing layer. Unauthenticated requests are redirected to an OIDC identity provider for login, and after authentication, JWT claims are injected as HTTP headers before the request reaches your app.

Your app receives identity information as plain HTTP headers (e.g., `X-User-Email`) — no in-app auth code required. OIDC is the underlying mechanism, so any standards-compliant identity provider works.

## Quick Start

**Step 1: Add an identity provider**

<CliCommand context="client">
```miren
miren auth provider add oidc my-google \
  --provider-url https://accounts.google.com \
  --client-id $GOOGLE_CLIENT_ID \
  --client-secret $GOOGLE_CLIENT_SECRET \
  --scope email --scope profile
```
</CliCommand>

**Step 2: Protect a route**

<CliCommand context="client">
```miren
miren route protect myapp.example.com \
  --provider my-google \
  --claim-header email:X-User-Email \
  --claim-header name:X-User-Name
```
</CliCommand>

That's it. Unauthenticated requests to `myapp.example.com` are now redirected to Google for login. After authentication, your app receives `X-User-Email` and `X-User-Name` headers.

## Authentication vs Authorization

This feature handles **authentication** — verifying _who_ a user is — not **authorization** — deciding _what_ they can access.

After authentication, your app receives claims as headers and decides what to do with them. For example, if you configure Google as your identity provider, your app receives the user's email address and can check the domain for basic access control.

## Trust Model

Your app trusts the claim headers (e.g., `X-User-Email`) because Miren is the only network path into the sandbox — external clients cannot reach your app directly. Miren validates the JWT from the identity provider and sets the headers before proxying the request.

This "trust the proxy" model is the standard approach used by tools like OAuth2 Proxy, Traefik ForwardAuth, and nginx `auth_request`. It's simple and works well when the platform controls the network topology, which Miren does via sandbox isolation.

Your app does not need to verify signatures or validate tokens — it can treat the claim headers as trusted input from the platform.

## How Claim Mappings Work

The `--claim-header` option maps JWT claims from the identity provider to HTTP headers that your app receives:

<CliCommand context="client">
```miren
miren route protect myapp.example.com \
  --provider my-google \
  --claim-header email:X-User-Email \
  --claim-header sub:X-User-ID \
  --claim-header name:X-User-Name
```
</CliCommand>

- Each `--claim-header` takes the form `CLAIM:HEADER`
- Multiple mappings can be specified
- Claims not present in the JWT are silently skipped
- Your app receives these as regular request headers

## Example Provider Setups

### Google

Google's identity provider works well when you want to restrict access by email domain.

<CliCommand context="client">
```miren
miren auth provider add oidc my-google \
  --provider-url https://accounts.google.com \
  --client-id $GOOGLE_CLIENT_ID \
  --client-secret $GOOGLE_CLIENT_SECRET \
  --scope email --scope profile
```
</CliCommand>

Your app can then check the `X-User-Email` header's domain for basic authorization (e.g., only allow `@yourcompany.com`).

### GitHub

GitHub doesn't expose a native OIDC endpoint, so Miren talks to it through a connector instead of the OIDC discovery path. You still get the same end result: identity arrives at your app as plain HTTP headers.

Register an OAuth App in GitHub (Settings → Developer settings → OAuth Apps) with the callback URL `https://your-route.example.com/.well-known/miren/oidc/callback`, then add the provider with `add github` and the orgs you want to allow:

<CliCommand context="client">
```miren
miren auth provider add github my-github \
  --client-id $GITHUB_CLIENT_ID \
  --client-secret $GITHUB_CLIENT_SECRET \
  --org mirendev
```
</CliCommand>

To restrict by team membership, suffix the org with one or more team slugs:

<CliCommand context="client">
```miren
miren auth provider add github my-github \
  --client-id $GITHUB_CLIENT_ID \
  --client-secret $GITHUB_CLIENT_SECRET \
  --org mirendev:engineering,platform
```
</CliCommand>

Repeat `--org` for multiple organizations. Attach the provider to a route as usual:

<CliCommand context="client">
```miren
miren route protect myapp.example.com \
  --provider my-github \
  --claim-header email:X-User-Email \
  --claim-header preferred_username:X-User-Login \
  --claim-header groups:X-User-Groups
```
</CliCommand>

The connector exposes the user's GitHub login as `preferred_username`, and any org/team memberships as `groups` (in the form `org:team`). The full set of claims available is `sub`, `email`, `email_verified`, `name`, `preferred_username`, and `groups`. Mix and match `--claim-header` mappings to surface the bits your app cares about.

A wrinkle worth knowing: the `groups` claim only carries entries for *teams* the user belongs to within configured orgs. If you set `--org NAME` with no team suffix, the user is authorized (they must be in NAME to get in) but `groups` is empty because there are no team memberships to surface. For org membership signal, rely on the fact that the request reached your app at all, or configure team filters with `--org NAME:team1,team2` to populate `groups`.

### GitLab

GitLab has a built-in OIDC provider — no federation layer needed. Register an application in your GitLab instance and point directly at it.

<CliCommand context="client">
```miren
miren auth provider add oidc my-gitlab \
  --provider-url https://gitlab.com \
  --client-id $GITLAB_CLIENT_ID \
  --client-secret $GITLAB_CLIENT_SECRET \
  --scope email --scope profile
```
</CliCommand>

### Keycloak (self-hosted)

[Keycloak](https://www.keycloak.org/) is an open-source identity provider you can run yourself. It supports user federation, identity brokering, and fine-grained claim configuration.

<CliCommand context="client">
```miren
miren auth provider add oidc my-keycloak \
  --provider-url https://keycloak.example.com/realms/myrealm \
  --client-id $KEYCLOAK_CLIENT_ID \
  --client-secret $KEYCLOAK_CLIENT_SECRET \
  --scope email --scope profile
```
</CliCommand>

## Reusing Providers Across Routes

Once you've added a provider, you can use it to protect any number of routes:

<CliCommand context="client">
```miren
miren route protect app1.example.com --provider my-google --claim-header email:X-User-Email
miren route protect app2.example.com --provider my-google --claim-header email:X-User-Email
```
</CliCommand>

## Default Route Support

`route protect` and `route unprotect` support the `--default` flag for the default route (which has no static hostname). When used with the default route, the OAuth2 redirect URL is derived from the request's `Host` header at runtime.

<CliCommand context="client">
```miren
miren route protect --default \
  --provider my-google \
  --claim-header email:X-User-Email
```
</CliCommand>

## Managing Identity Providers

<CliCommand context="client">
```miren
# List all providers
miren auth provider list

# Show details of a provider
miren auth provider show my-google

# Remove a provider
miren auth provider remove my-google
```
</CliCommand>

## Removing Route Protection

<CliCommand context="client">
```miren
# Remove protection from a route (provider is preserved for reuse)
miren route unprotect myapp.example.com

# Check route status including protection info
miren route show myapp.example.com
```
</CliCommand>

See the [CLI reference](/command/route-protect) for the full list of options.
