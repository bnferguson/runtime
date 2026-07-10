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
import math
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


# -- Unmarked-secret heuristics (Section C2) --------------------------------
# Section C lists only variables explicitly flagged sensitive:true. But that
# flag is display-only (a reader saw every value regardless), and people forget
# to set it constantly -- so unflagged-but-secret config is a silent hole in
# the rotation list. This pass re-scans ALL variables and flags ones that look
# secret but were NOT marked sensitive, by two independent signals: the key
# NAME, and the VALUE shape (which catches misnamed secrets a name check would
# miss). Values are inspected to compute a signal but, as everywhere else in
# this tool, never printed -- output is key name + reason only. It's a review
# list, not an assertion: entropy heuristics have false positives, so an
# operator confirms, then rotates AND sets sensitive:true.

# Key names that read as secret-bearing...
UNMARKED_NAME_RE = re.compile(
    r"(secret|password|passwd|token|api[_-]?key|private[_-]?key|credential|"
    r"access[_-]?key|client[_-]?secret|passphrase|bearer|signing[_-]?key|"
    r"encryption[_-]?key|_key$|_dsn$)", re.I)
# ...minus names that merely reference a secret without being one: public keys,
# ids (client_id, key_id), endpoints/urls, file paths, usernames, toggles.
NAME_DENY_RE = re.compile(
    r"(public|client[_-]?id|_id$|_url$|_uri$|_endpoint$|_host$|_name$|"
    r"_file$|_path$|_user$|_username$|_enabled$|_timeout$|_ttl$|_sid$|"
    r"_count$|key_id)", re.I)
# Identifier-ish keys whose values are high-entropy but not secret (account /
# org / billing / rule IDs, SIDs). Suppresses ONLY the weak entropy signal for
# these; a strong shape (PEM/JWT/url-creds/provider-prefix) still fires.
ENTROPY_KEY_DENY_RE = re.compile(
    r"(_id$|_sid$|_uid$|_guid$|_number$|account|organization|billing)", re.I)

# Strong value shapes: unambiguous credential material.
PROVIDER_TOKEN_RE = re.compile(
    r"(gh[opsu]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|"
    r"xox[baprs]-[A-Za-z0-9-]{10,}|sk_(live|test)_[A-Za-z0-9]{16,}|"
    r"AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_\-]{35}|glpat-[A-Za-z0-9_\-]{16,}|"
    r"dop_v1_[a-f0-9]{40,}|SG\.[A-Za-z0-9_\-]{16,})")
URL_CRED_RE = re.compile(r"://[^/\s:@]+:[^/\s@]+@")
JWT_RE = re.compile(r"^eyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]+$")
HIGH_ENTROPY_CHARSET_RE = re.compile(r"^[A-Za-z0-9+/=_\-.]{24,}$")


def _looks_non_secret(v):
    """Cheap excluders so the entropy check doesn't flag urls/emails/numbers/hosts/paths."""
    return bool(
        re.match(r"^https?://", v)
        or re.match(r"^[\w.+-]+@[\w-]+\.[\w.-]+$", v)          # email
        or re.match(r"^-?\d+(\.\d+)?$", v)                      # number
        or re.match(r"^\d{4}-\d\d-\d\d[T ]", v)                # timestamp
        or re.match(r"^([A-Za-z0-9-]+\.){2,}[A-Za-z]{2,}$", v)  # dotted hostname
        or v.startswith("/")                                    # path
    )


def _shannon(s):
    counts = {}
    for c in s:
        counts[c] = counts.get(c, 0) + 1
    n = len(s)
    return -sum((k / n) * math.log2(k / n) for k in counts.values()) if n else 0.0


def value_signal(v):
    """Reason string if a value LOOKS like a credential, else None. Never emits the value."""
    if v is None:
        return None
    v = v.strip().strip('"').strip("'")
    if not v:
        return None
    if "-----BEGIN" in v:
        return "PEM/key material"
    if JWT_RE.match(v):
        return "JWT-shaped value"
    if URL_CRED_RE.search(v):
        return "URL with embedded credentials"
    if PROVIDER_TOKEN_RE.search(v):
        return "known provider token prefix"
    if HIGH_ENTROPY_CHARSET_RE.match(v) and not _looks_non_secret(v) and _shannon(v) >= 3.8:
        return "high-entropy value"
    return None


def name_signal(key):
    if NAME_DENY_RE.search(key):
        return None
    return "name looks secret" if UNMARKED_NAME_RE.search(key) else None


def iter_vars(spec_lines):
    """Yield (key, sensitive_bool, values) for each config variable / service env entry
    in a config_version spec. `values` is the list of every non-None value seen for that
    key (a key can recur across services' env with different values, and any one of them
    could be the secret, so we keep them all rather than collapsing to the first).
    Line-based like the rest of this scraper; relies on `key:` preceding `sensitive:`/
    `value:` within an item, which holds in the CLI output. A `value: |` block scalar
    (e.g. a PEM) is captured (for shape-testing only, never printed)."""
    items, cur = [], None
    block_capture, block_indent = False, None
    for raw in spec_lines:
        stripped = raw.strip()
        indent = len(raw) - len(raw.lstrip())
        if block_capture:
            if stripped and indent > block_indent:
                cur["value"] = (cur["value"] or "") + "\n" + stripped
                continue
            block_capture = False
        km = re.match(r"-?\s*key:\s*(\S+)\s*$", stripped)
        if km:
            if cur:
                items.append(cur)
            cur = {"key": km.group(1), "sensitive": False, "value": None}
            continue
        if cur is None:
            continue
        sm = re.match(r"sensitive:\s*(true|false)\s*$", stripped)
        if sm:
            cur["sensitive"] = sm.group(1) == "true"
            continue
        vm = re.match(r"value:\s?(.*)$", stripped)
        if vm:
            rest = vm.group(1).strip()
            if rest in ("|", "|-", "|+", ">", ">-", ">+", ""):
                cur["value"], block_capture, block_indent = "", True, indent
            else:
                cur["value"] = rest
    if cur:
        items.append(cur)
    # A key can recur across services' env; merge sensitive as OR (marked anywhere ->
    # treated sensitive) and keep every value so all of them get shape-tested.
    merged = {}
    for it in items:
        k = it["key"]
        m = merged.setdefault(k, {"key": k, "sensitive": False, "values": []})
        m["sensitive"] = m["sensitive"] or it["sensitive"]
        if it["value"] is not None:
            m["values"].append(it["value"])
    return [(m["key"], m["sensitive"], m["values"]) for m in merged.values()]


def unmarked_candidates(record):
    """(key, reasons, strong) for vars NOT flagged sensitive that look secret.
    strong=True means a value-shape signal fired (high confidence)."""
    out = []
    for key, sensitive, values in iter_vars(spec_block(record)):
        if sensitive:
            continue
        reasons, strong = [], False
        for value in values:  # any one of a key's values could be the secret
            vs = value_signal(value)
            if vs == "high-entropy value" and ENTROPY_KEY_DENY_RE.search(key):
                vs = None  # identifier, not a credential -- drop the weak signal
            if vs:
                reasons.append(vs)
                strong = True
                break
        ns = name_signal(key)
        if ns:
            reasons.append(ns)
        if reasons:
            out.append((key, reasons, strong))
    return out


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
        unmarked = {}  # app -> {key: (reasons, strong)}
        for r in split_records(cfg_text):
            app = record_app(r)
            keys = sensitive_var_keys(r)
            if keys:
                by_app.setdefault(app, set()).update(keys)
            for key, reasons, strong in unmarked_candidates(r):
                slot = unmarked.setdefault(app, {})
                prev_reasons, prev_strong = slot.get(key, ([], False))
                slot[key] = (sorted(set(prev_reasons + reasons)), prev_strong or strong)
        if by_app:
            print("\n  app env vars flagged sensitive (rotate these):\n")
            for app in sorted(by_app):
                print(f"  {app}")
                for k in sorted(by_app[app]):
                    print(f"      - {k}")
        else:
            print("\n  (no variables flagged sensitive found)")

        # C2: variables that look secret but were NOT flagged sensitive. Heuristic
        # review list (values never printed). '!' marks a value-shape hit (high
        # confidence); an unmarked line means a name-only hit (worth a glance).
        print("\n  MAYBE-MISSED SECRETS -- these look like secrets but nobody marked")
        print("  them sensitive, so the list above skipped them. Look at each one:")
        print("  if it really is a secret, rotate it too (and mark it sensitive so")
        print("  it's not missed next time). A \"!\" means the value itself looks like")
        print("  a credential; otherwise it's the name that looks secret. As always,")
        print("  the actual values are never shown.")
        if unmarked:
            for app in sorted(unmarked):
                rows = unmarked[app]
                print(f"\n  {app}")
                for key in sorted(rows, key=lambda k: (not rows[k][1], k)):
                    reasons, strong = rows[key]
                    print(f"    {'!' if strong else ' '} {key}  ({', '.join(reasons)})")
        else:
            print("\n  (none detected)")
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
