#!/usr/bin/env python3
"""verify-pypi-channel.py — checks that a trackfw version is live on PyPI.

D6 fix: normalizes the version string to PEP 440 before querying the releases dict.
npm/git format: 8.0.0-rc1
PyPI normalized: 8.0.0rc1  (what appears as the key in the releases JSON)

Usage: python3 verify-pypi-channel.py <version>
Exits 0 if found, 1 if not found or on error.
"""
import sys
import os
import urllib.request
import json

def main():
    if len(sys.argv) < 2:
        print("verify-pypi-channel: usage: verify-pypi-channel.py <version>", file=sys.stderr)
        sys.exit(1)

    raw_version = sys.argv[1]

    # D6: normalize version to PEP 440 canonical form.
    # 8.0.0-rc1 → 8.0.0rc1 (the key used in PyPI's releases JSON dict).
    try:
        from packaging.version import Version
        py_version = str(Version(raw_version))
    except ImportError:
        # packaging not installed — fall back and warn (D6 risk accepted as "cannot prove")
        py_version = raw_version
        print(f"warn: 'packaging' module not available — using raw version '{raw_version}' "
              f"for PyPI lookup; pre-release normalization (8.0.0-rc1 → 8.0.0rc1) skipped",
              file=sys.stderr)
    except Exception as e:
        print(f"verify-pypi-channel: could not normalize version '{raw_version}': {e}",
              file=sys.stderr)
        sys.exit(1)

    url = "https://pypi.org/pypi/trackfw/json"
    try:
        data = json.loads(urllib.request.urlopen(url, timeout=15).read())
    except Exception as e:
        print(f"FAIL: PyPI query failed: {e}", file=sys.stderr)
        sys.exit(1)

    releases = data.get("releases", {})
    if py_version in releases:
        print(f"ok: PyPI trackfw {py_version} is live")
        sys.exit(0)
    else:
        available = list(releases.keys())[-5:] if releases else []
        print(f"FAIL: PyPI trackfw {py_version} not found "
              f"(raw version: {raw_version})", file=sys.stderr)
        print(f"Last 5 available versions: {available}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
