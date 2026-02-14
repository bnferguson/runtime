#!/usr/bin/env python3
"""
Measure test execution time for each Go package individually.

Runs each package with tests through hack/it and records wall-clock time.
Also measures harness overhead by running hack/it with a no-test package.

Output: JSON file with per-package timings for CI optimization analysis.
"""

import argparse
import json
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def find_test_packages():
    """Return list of Go packages that contain test files."""
    result = subprocess.run(
        ["go", "list", "-f",
         "{{.ImportPath}} {{len .TestGoFiles}} {{len .XTestGoFiles}}",
         "./..."],
        capture_output=True, text=True, cwd=ROOT,
    )
    pkgs = []
    for line in result.stdout.strip().splitlines():
        parts = line.split()
        if len(parts) == 3 and (int(parts[1]) > 0 or int(parts[2]) > 0):
            pkgs.append(parts[0])
    return sorted(pkgs)


def load_existing(path):
    """Load an existing test-times.json file."""
    with open(path) as f:
        return json.load(f)


def run_package(pkg, timeout=600):
    """Run tests for a single package via hack/it and return timing info."""
    print(f"  running: {pkg} ...", end=" ", flush=True)
    start = time.monotonic()
    try:
        proc = subprocess.run(
            ["./hack/it", pkg, "-count=1"],
            capture_output=True, text=True, cwd=ROOT, timeout=timeout,
        )
        elapsed = time.monotonic() - start
        passed = proc.returncode == 0
    except subprocess.TimeoutExpired:
        elapsed = timeout
        passed = False
        proc = None

    status = "pass" if passed else "fail"
    if proc is None:
        status = "timeout"

    print(f"{elapsed:.1f}s [{status}]")
    return {
        "package": pkg,
        "elapsed_s": round(elapsed, 2),
        "status": status,
        "returncode": proc.returncode if proc else -1,
    }


def run_harness_overhead(timeout=300):
    """Measure iso/harness startup overhead by running the root package (no real tests)."""
    print("  running: harness overhead (hack/it .) ...", end=" ", flush=True)
    start = time.monotonic()
    try:
        proc = subprocess.run(
            ["./hack/it", "."],
            capture_output=True, text=True, cwd=ROOT, timeout=timeout,
        )
        elapsed = time.monotonic() - start
        passed = proc.returncode == 0
    except subprocess.TimeoutExpired:
        elapsed = timeout
        passed = False
        proc = None

    status = "pass" if passed else ("timeout" if proc is None else "fail")
    print(f"{elapsed:.1f}s [{status}]")
    return {
        "package": ".",
        "elapsed_s": round(elapsed, 2),
        "status": status,
        "returncode": proc.returncode if proc else -1,
    }


def update_packages(existing, new_results):
    """Merge new results into existing data, replacing any matching packages."""
    by_pkg = {r["package"]: r for r in existing["packages"]}
    for r in new_results:
        by_pkg[r["package"]] = r

    all_pkgs = sorted(by_pkg.values(), key=lambda r: -r["elapsed_s"])
    total_test_time = sum(r["elapsed_s"] for r in all_pkgs)
    passed = sum(1 for r in all_pkgs if r["status"] == "pass")
    failed = sum(1 for r in all_pkgs if r["status"] != "pass")

    existing["packages"] = all_pkgs
    existing["summary"]["total_test_time_s"] = round(total_test_time, 2)
    existing["summary"]["package_count"] = len(all_pkgs)
    existing["summary"]["passed"] = passed
    existing["summary"]["failed"] = failed
    return existing


def main():
    parser = argparse.ArgumentParser(description="Measure test execution times")
    parser.add_argument("output", nargs="?", default="test-times.json",
                        help="Output file (default: test-times.json)")
    parser.add_argument("--update", nargs="+", metavar="PKG",
                        help="Measure only these packages and merge into existing output file")
    args = parser.parse_args()

    output_file = args.output

    if args.update:
        pkgs = args.update
        print(f"Measuring {len(pkgs)} package(s)...\n")

        results = []
        for i, pkg in enumerate(pkgs, 1):
            print(f"[{i}/{len(pkgs)}]")
            r = run_package(pkg)
            results.append(r)

        existing = load_existing(output_file)
        output = update_packages(existing, results)

        with open(output_file, "w") as f:
            json.dump(output, f, indent=2)

        print(f"\nUpdated {len(pkgs)} package(s) in {output_file}")
        return

    print("Discovering test packages...")
    pkgs = find_test_packages()
    print(f"Found {len(pkgs)} packages with tests.\n")

    results = []

    # Measure harness overhead first
    print("[0/{n}] Measuring harness overhead".format(n=len(pkgs)))
    overhead = run_harness_overhead()
    results.append(overhead)

    # Measure each package
    for i, pkg in enumerate(pkgs, 1):
        print(f"[{i}/{len(pkgs)}]")
        r = run_package(pkg)
        results.append(r)

    # Summary
    pkg_results = [r for r in results if r["package"] != "."]
    total_test_time = sum(r["elapsed_s"] for r in pkg_results)
    passed = sum(1 for r in pkg_results if r["status"] == "pass")
    failed = sum(1 for r in pkg_results if r["status"] != "pass")

    summary = {
        "harness_overhead_s": overhead["elapsed_s"],
        "total_test_time_s": round(total_test_time, 2),
        "package_count": len(pkg_results),
        "passed": passed,
        "failed": failed,
    }

    output = {
        "summary": summary,
        "packages": sorted(pkg_results, key=lambda r: -r["elapsed_s"]),
    }

    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    print(f"\nResults written to {output_file}")
    print(f"  Harness overhead: {overhead['elapsed_s']:.1f}s")
    print(f"  Total test time:  {total_test_time:.1f}s (sum of {len(pkg_results)} packages)")
    print(f"  Passed: {passed}, Failed: {failed}")
    print(f"\n  Top 10 slowest packages:")
    for r in output["packages"][:10]:
        short = r["package"].removeprefix("miren.dev/runtime/")
        print(f"    {r['elapsed_s']:7.1f}s  {short}")


if __name__ == "__main__":
    main()
