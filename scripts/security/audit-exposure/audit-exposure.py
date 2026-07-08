#!/usr/bin/env python3
#
# Post-incident review helper for the :8443 cert-auth bypass
# (GHSA-8fh7-7q4q-cq52). Once you've closed the exposure (upgrade to the
# patched release and restrict :8443 ingress), this helps you look over
# what the bypass could have touched on a cluster. See the README in this
# directory for the full walkthrough.
#
# Read-only -- it only calls `miren debug entity list`. Stdlib Python 3,
# no deps. Run it from a host with the `miren` CLI configured for the
# cluster.
#
# What it shows:
#   A. standing access grants (oidc_binding, runner_invite, auth
#      providers) to look over -- an entry you don't recognize is worth
#      a closer look.
#   B. workloads/routes created or changed in the window, to cross-check
#      against your own deploys.
#   C. a checklist of secrets that were reachable via the bypass, for
#      rotation.
#
# What it can't show: whether anything was actually read. Reads leave no
# trace, so a quiet result isn't a clean bill of health -- just the
# absence of anything left behind. If :8443 was reachable from untrusted
# networks during the window, it's prudent to presume the secrets in
# Section C may have been read, and rotate them.
#
# This is a stopgap: it scrapes the CLI's human-readable output, so it's
# brittle to changes in that format. Treat it as best-effort incident
# tooling, not a stable interface.
#
# USAGE
#   audit-exposure.py --since 2026-06-01T00:00:00Z [--until ...] [--miren miren]
"""audit-exposure: post-incident review helper for GHSA-8fh7-7q4q-cq52."""
import argparse
import datetime as dt
import re
import subprocess
import sys

# Section A: small, fully-enumerable kinds that grant standing access or route
# traffic -- review every entry by hand.
STANDING_KINDS = [
    "oidc_binding",       # standing OIDC access (e.g. from a GitHub repo)
    "runner_invite",      # lets a (rogue) runner join the cluster
    "oidc_provider",      # route auth provider
    "password_provider",  # route auth provider
    "waf_profile",        # web-app-firewall rules (weakening = tamper)
    "http_route",         # ingress routing (a rogue route can redirect traffic)
]

# Section B: the workload chain an operator actually authors. app -> app_version
# is the meaningful unit of change; config_version / deployment / artifact /
# sandbox_pool / sandbox are subordinate byproducts and aren't listed here
# (config_version is still read for the Section C rotation list).
WORKLOAD_KINDS = ["app", "app_version"]

CREATED = "db/entity.created"
UPDATED = "db/entity.updated"

# Redact secret-bearing fields when dumping specs so the report is safe to
# share. Matches keys like client_secret, password_hash, code_hash, api_key,
# access_key, *_token, *_credential, and anything ending in _key (but not a
# bare `key:`, which shows up in non-secret places like claim_conditions).
SECRET_KEY_RE = re.compile(
    r"^-?\s*[A-Za-z0-9_.-]*"
    r"(?:secret|password|passwd|hash|token|credential|bearer|apikey|_key)"
    r"[A-Za-z0-9_.-]*\s*:",
    re.I,
)


def redact_spec(lines):
    """Redact secret-bearing fields in a spec block for display.

    Handles multi-line block scalars: when a secret key's value is a `|`
    block (e.g. a private_key PEM), the header line AND the indented body
    beneath it are dropped, not just the header. Values are never printed.
    Returns stripped display lines.
    """
    out, block_indent = [], None
    for raw in lines:
        indent = len(raw) - len(raw.lstrip())
        stripped = raw.strip()
        if block_indent is not None:
            if not stripped:
                continue  # blank line inside the block scalar; stays dropped
            if indent > block_indent:
                continue  # still inside the redacted block scalar
            block_indent = None
        if SECRET_KEY_RE.match(stripped):
            out.append(f"{stripped.split(':', 1)[0]}: <redacted>")
            block_indent = indent
            continue
        out.append(stripped)
    return out


def run_list(miren, kind):
    """Return the CLI's stdout for a kind, or None if the query failed.

    A failed query must NOT be mistaken for an empty result -- otherwise a
    partial scan reads as clean, the exact false-negative this tool exists to
    avoid. Callers surface None as an explicit failure. The timeout keeps a
    hung CLI from stalling the scan mid-incident.
    """
    try:
        r = subprocess.run(
            [miren, "debug", "entity", "list", "-k", kind],
            capture_output=True, text=True, timeout=60,
        )
    except (subprocess.TimeoutExpired, OSError) as e:
        sys.stderr.write(f"  ! list -k {kind} errored: {e}\n")
        return None
    if r.returncode != 0:
        sys.stderr.write(f"  ! list -k {kind} failed: {r.stderr.strip()[:200]}\n")
        return None
    return r.stdout


def split_records(text):
    """Split `entity list` output into per-entity blocks on the `id: ` marker."""
    records, cur = [], None
    for line in text.splitlines():
        if line.startswith("id: "):
            if cur is not None:
                records.append(cur)
            cur = [line]
        elif cur is not None:
            cur.append(line)
    if cur is not None:
        records.append(cur)
    return records


def attr_map(lines):
    """Pair `- id: X` with the following `value: Y` into a dict."""
    attrs = {}
    for i, l in enumerate(lines):
        m = re.match(r"\s*- id:\s*(\S+)\s*$", l)
        if m and i + 1 < len(lines):
            vm = re.match(r"\s*value:\s*(.*)$", lines[i + 1])
            if vm:
                attrs[m.group(1)] = vm.group(1).strip()
    return attrs


def spec_block(lines):
    """Return the indented lines under `spec:` up to the next top-level key."""
    out, in_spec = [], False
    for l in lines:
        if l.startswith("spec:"):
            in_spec = True
            continue
        if in_spec:
            if l and not l[0].isspace():   # next top-level key (attrs:, etc.)
                break
            out.append(l.rstrip())
    return [l for l in out if l.strip()]


def sensitive_var_keys(lines):
    """Names of variables flagged sensitive:true in a record (never values).

    Backward-search heuristic: for each `sensitive: true`, take the nearest
    preceding `key:`. Catches both top-level variables and per-service env
    without depending on exact nesting. Values are never read or emitted.
    """
    keys = set()
    for i, l in enumerate(lines):
        if re.match(r"\s*sensitive:\s*true\s*$", l):
            for j in range(i, -1, -1):
                m = re.match(r"\s*-?\s*key:\s*(\S+)", lines[j])
                if m:
                    keys.add(m.group(1))
                    break
    return keys


def record_app(lines):
    for l in lines:
        m = re.match(r"\s+app:\s*(app/\S+)", l)
        if m:
            return m.group(1)
    return "?"


def parse_ts(s):
    if not s:
        return None
    s = s.strip().replace("Z", "+00:00")
    try:
        ts = dt.datetime.fromisoformat(s)
    except ValueError:
        return None
    # A bare timestamp (no offset) parses naive; stamp UTC so it stays
    # comparable to the always-aware entity timestamps in in_window().
    if ts.tzinfo is None:
        ts = ts.replace(tzinfo=dt.timezone.utc)
    return ts


def entity_id(lines):
    return lines[0][len("id: "):].strip() if lines else "?"


def summarize(miren, kind):
    """List of entity dicts for a kind, or None if the query failed."""
    text = run_list(miren, kind)
    if text is None:
        return None
    out = []
    for r in split_records(text):
        a = attr_map(r)
        out.append({
            "id": entity_id(r),
            "name": a.get("dev.miren.core/metadata.name", ""),
            "created": parse_ts(a.get(CREATED)),
            "updated": parse_ts(a.get(UPDATED)),
            "spec": spec_block(r),
        })
    return out


def main():
    ap = argparse.ArgumentParser(description="Post-incident exposure review (GHSA-8fh7-7q4q-cq52).")
    ap.add_argument("--since", required=True, help="exposure-window start, ISO8601 (e.g. 2026-06-01T00:00:00Z)")
    ap.add_argument("--until", default=None, help="exposure-window end, ISO8601 (default: now)")
    ap.add_argument("--miren", default="miren", help="path to miren CLI (default: miren)")
    args = ap.parse_args()

    since = parse_ts(args.since)
    if since is None:
        sys.exit(f"bad --since: {args.since!r}")
    if args.until:
        until = parse_ts(args.until)
        if until is None:
            sys.exit(f"bad --until: {args.until!r}")
    else:
        until = dt.datetime.now(dt.timezone.utc)

    def in_window(ts):
        return ts is not None and since <= ts <= until

    failures = []  # kinds whose query failed; non-empty means a PARTIAL scan

    print("=" * 74)
    print(" miren post-incident review  (GHSA-8fh7-7q4q-cq52)")
    print(f" window: {since.isoformat()}  ..  {until.isoformat()}")
    print("=" * 74)
    print(" This reviews what the bypass could have left behind or changed.")
    print(" It can't tell you what was read -- reads leave no trace -- so a")
    print(" quiet result isn't a clean bill of health. If :8443 was reachable")
    print(" from untrusted networks in this window, it's prudent to presume")
    print(" the secrets in Section C may have been read, and rotate them.")

    # -- Section A: standing-access artifacts (review ALL, any age) --
    print("\n" + "#" * 74)
    print("# A. STANDING ACCESS & ROUTING  --  worth looking over by hand.")
    print("#    These grant ongoing access or route traffic. An entry you")
    print("#    don't recognize (an OIDC subject/repo that isn't yours, an")
    print("#    invite you didn't cut, a route you didn't add) is worth a")
    print("#    closer look.")
    print("#" * 74)
    for kind in STANDING_KINDS:
        rows = summarize(args.miren, kind)
        if rows is None:
            failures.append(kind)
            print(f"\n-- {kind}  (FAILED TO QUERY -- result unreliable) "
                  + "-" * max(0, 25 - len(kind)))
            continue
        print(f"\n-- {kind}  ({len(rows)}) " + "-" * (60 - len(kind)))
        for e in rows:
            flag = "  <-- created in window" if in_window(e["created"]) else ""
            c = e["created"].isoformat() if e["created"] else "?"
            print(f"  {e['id']}   created={c}{flag}")
            for l in redact_spec(e["spec"]):
                print(f"      {l}")

    # -- Section B: routing + workloads changed during the window --
    print("\n" + "#" * 74)
    print("# B. WORKLOAD CHANGES  --  new or changed apps and versions.")
    print("#    app -> app_version is the unit that matters. Subordinate")
    print("#    byproducts (deployments, config versions, artifacts, pools)")
    print("#    aren't listed. Cross-check these against your own deploys.")
    print("#" * 74)
    hits = []
    for kind in WORKLOAD_KINDS:
        res = summarize(args.miren, kind)
        if res is None:
            failures.append(kind)
            print(f"\n  ! FAILED to query {kind} -- this section is incomplete")
            continue
        for e in res:
            if in_window(e["created"]):
                hits.append((e["created"], kind, "created", e["id"], e["name"]))
            elif in_window(e["updated"]):
                hits.append((e["updated"], kind, "updated", e["id"], e["name"]))
    hits.sort(key=lambda h: h[0])
    if not hits:
        print("\n  (nothing created/updated in the window)")
    else:
        print(f"\n  {len(hits)} entities touched in window (oldest first):\n")
        for ts, kind, what, eid, name in hits:
            print(f"  {ts.isoformat()}  {what:7}  {kind:15} {eid}  {name}")
    # -- Section C: rotation worklist (secrets an attacker could have read) --
    print("\n" + "#" * 74)
    print("# C. ROTATION CHECKLIST  --  secrets that were reachable via the")
    print("#    bypass. Not 'what was taken' (reads leave no trace) -- just")
    print("#    what's prudent to rotate, since you can't confirm it wasn't")
    print("#    read. (variable names only; values are never printed.)")
    print("#" * 74)
    by_app = {}
    cfg_text = run_list(args.miren, "config_version")
    if cfg_text is None:
        failures.append("config_version")
        print("\n  ! FAILED to query config_version -- rotation list is INCOMPLETE")
    else:
        for r in split_records(cfg_text):
            keys = sensitive_var_keys(r)
            if keys:
                by_app.setdefault(record_app(r), set()).update(keys)
        if by_app:
            print("\n  app env vars flagged sensitive (rotate these):\n")
            for app in sorted(by_app):
                print(f"  {app}")
                for k in sorted(by_app[app]):
                    print(f"      - {k}")
        else:
            print("\n  (no variables flagged sensitive found)")
    print("\n  ALSO rotate (stored readable in the entity store, see Section A):")
    print("    - oidc_provider client_secret(s)")
    print("    - password_provider password_hash(es)")
    print("    - any addon (postgres/mysql/valkey/...) credentials")
    print("    - app config values NOT flagged sensitive but secret in practice")

    print("\n" + "=" * 74)
    if failures:
        uniq = ", ".join(sorted(set(failures)))
        print(f" INCOMPLETE: could not query {len(set(failures))} kind(s): {uniq}")
        print(" Some queries failed, so this run did NOT cover everything. Treat it")
        print(" as unreliable, not clean, and re-run once the CLI/cluster is healthy.")
        print("-" * 74)
    print(" done. Section A: look it over. Section B: cross-check your deploys.")
    print(" Section C: rotate as a precaution. A quiet result isn't proof that")
    print(" nothing happened -- reads leave no trace -- so if :8443 was reachable")
    print(" from untrusted networks in the window, presuming a leak and rotating")
    print(" is the safe call.")
    print("=" * 74)


if __name__ == "__main__":
    main()
