# audit-exposure

A read-only helper for reviewing what the `:8443` cert-auth bypass
(GHSA-8fh7-7q4q-cq52) could have touched on a Miren cluster, and for building
a rotation checklist afterward.

## First, the honest part

This reviews what the bypass could have **left behind or changed**. It
**cannot** tell you whether anything was *read*: reads leave no trace in the
entity store, and the highest-value target (user-set app environment
variables) is exactly what would be read quietly. So a quiet result is the
absence of anything left behind, not a clean bill of health.

If your cluster's `:8443` was reachable from untrusted networks while it was
on an affected version, the safe assumption is that the readable secrets may
have been seen. Upgrade, restrict ingress, then rotate the secrets in Section
C (and the provider secrets in Section A) as a precaution.

## Requirements

- The `miren` CLI, configured and authenticated against the cluster you want
  to review. The script only calls `miren debug entity list`.
- Python 3, standard library only (no dependencies).

## Usage

Run it once per cluster. `--since` is the start of your exposure window: when
the cluster's `:8443` first became reachable from untrusted networks on an
affected version. If you're unsure, err earlier.

```bash
./audit-exposure.py --since 2026-06-01T00:00:00Z
./audit-exposure.py --since 2026-06-01T00:00:00Z --until 2026-07-08T18:00:00Z
```

Timestamps are ISO 8601; a bare timestamp is treated as UTC. Point at a
specific CLI with `--miren /path/to/miren` if it isn't on your `PATH`.

## What it reports

**A. Standing access & routing.** Every OIDC binding, runner invite, auth
provider, and HTTP route, listed for you to eyeball. These grant ongoing
access or route traffic, so an entry you don't recognize (an OIDC subject or
repo that isn't yours, an invite you didn't create, a route you didn't add)
is worth a closer look. Secret values in this section are redacted.

**B. Workload changes.** New or changed apps and app versions within the
window, so you can cross-check against your own deploys. A deploy you didn't
make is the thing to catch. (Config versions, deployments, artifacts, and
pools are byproducts of a deploy and aren't listed separately.)

**C. Rotation checklist.** The per-app environment variables flagged
sensitive, by name, plus the provider secrets stored in the entity store.
These were readable via the bypass, so rotate them. Values are never printed,
only names.

Section C also flags **maybe-missed secrets**. The `sensitive` flag is
display-only and easy to forget, so the checklist runs a second pass for
variables that *weren't* marked sensitive but still look like secrets, judged
by the key name and by the shape of the value (a PEM block, a `user:pass@host`
URL, a known token prefix, or a high-entropy string). A `!` marks a value that
looks like a credential; otherwise it's the name that looks secret. This is a
review list, not a verdict, so expect the occasional false positive (an ID or
a hash) — confirm each one, rotate the real ones, and mark them sensitive so
they stop hiding. Values are inspected to make the guess but, as everywhere
else, never printed.

If a query fails, the affected section says so and the whole run is flagged
INCOMPLETE, so a partial scan is never mistaken for a clean one.

## Note

This is a stopgap that scrapes the CLI's human-readable output, so it's
brittle to changes in that format. Treat it as best-effort incident tooling
rather than a stable interface.
