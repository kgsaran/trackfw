#!/usr/bin/env python3
"""verify-pypi-channel.py — checks that a trackfw version is live on PyPI.

D6 fix: normalizes the version string to PEP 440 before querying the releases dict.
npm/git format: 8.0.0-rc1
PyPI normalized: 8.0.0rc1  (what appears as the key in the releases JSON)

Retry with exponential backoff and a hard deadline.
Measured 2026-09-14 (rc2 release): PyPI propagated with similar latency to npm.
Deadline 900s matches verify-npm-channels.sh; exhausting it is always a FAIL.

Usage:
  python3 verify-pypi-channel.py <version>
  python3 verify-pypi-channel.py <version> --retry

  --retry         Enable retry loop (for use in release workflow after publishing)
  VERIFY_DEADLINE Override deadline in seconds (default: 900)
  PUBLISH_PYPI_RESULT  Set to the needs.publish-pypi.result from workflow context;
                       if not 'success', report publish failure instead of CDN delay.

Exits 0 if found, 1 if not found within deadline, or on publish failure.
"""
import sys
import os
import time
import urllib.request
import json


def normalize_version(raw_version: str) -> str:
    """D6: normalize version to PEP 440 canonical form. 8.0.0-rc1 → 8.0.0rc1."""
    try:
        from packaging.version import Version
        return str(Version(raw_version))
    except ImportError:
        print(
            f"warn: 'packaging' module not available — using raw version '{raw_version}' "
            f"for PyPI lookup; pre-release normalization (8.0.0-rc1 → 8.0.0rc1) skipped",
            file=sys.stderr,
        )
        return raw_version
    except Exception as e:
        print(
            f"verify-pypi-channel: could not normalize version '{raw_version}': {e}",
            file=sys.stderr,
        )
        sys.exit(1)


def query_pypi(py_version: str, raw_version: str) -> bool:
    """Query PyPI JSON API. Returns True if version is live, False if not found."""
    url = "https://pypi.org/pypi/trackfw/json"
    try:
        data = json.loads(urllib.request.urlopen(url, timeout=15).read())
    except Exception as e:
        print(f"FAIL: PyPI query failed: {e}", file=sys.stderr)
        return False

    releases = data.get("releases", {})
    if py_version in releases:
        return True

    available = list(releases.keys())[-5:] if releases else []
    print(
        f"wait: PyPI trackfw {py_version} not found (raw: {raw_version}); "
        f"last 5 available: {available}",
        file=sys.stderr,
    )
    return False


def main():
    if len(sys.argv) < 2:
        print("verify-pypi-channel: usage: verify-pypi-channel.py <version> [--retry]", file=sys.stderr)
        sys.exit(1)

    raw_version = sys.argv[1]
    retry_mode = "--retry" in sys.argv

    py_version = normalize_version(raw_version)

    # Check publish-side outcome if set by workflow needs context
    publish_result = os.environ.get("PUBLISH_PYPI_RESULT", "")
    if publish_result and publish_result != "success":
        print(
            f"FAIL: PyPI publish job result: '{publish_result}' — package may not have been published.",
            file=sys.stderr,
        )
        print(
            "  Action: check publish-pypi job logs for the root cause.",
            file=sys.stderr,
        )
        sys.exit(1)

    if not retry_mode:
        # Single-shot mode (legacy / local use)
        if query_pypi(py_version, raw_version):
            print(f"ok: PyPI trackfw {py_version} is live")
            sys.exit(0)
        else:
            print(
                f"FAIL: PyPI trackfw {py_version} not found (raw version: {raw_version})",
                file=sys.stderr,
            )
            sys.exit(1)

    # --- Retry mode: used by verify-channels in release.yml ------------------
    # Deadline: 900s (matches verify-npm-channels.sh; same measured propagation baseline).
    deadline = int(os.environ.get("VERIFY_DEADLINE", "900"))
    poll_interval = 30
    poll_max = 60

    start = time.monotonic()

    print(
        f"Verifying PyPI trackfw {py_version} (deadline: {deadline}s, "
        f"measured propagation: similar to npm CDN on 2026-09-14)"
    )

    while True:
        elapsed = int(time.monotonic() - start)

        if elapsed >= deadline:
            print("", file=sys.stderr)
            print(
                f"FAIL: deadline exhausted after {elapsed}s — PyPI trackfw {py_version} did not "
                f"propagate within {deadline}s.",
                file=sys.stderr,
            )
            print(
                "  This is a CDN propagation timeout, NOT a confirmed publish failure.",
                file=sys.stderr,
            )
            print(
                f"  Real elapsed: {elapsed}s (deadline: {deadline}s)",
                file=sys.stderr,
            )
            sys.exit(1)

        if query_pypi(py_version, raw_version):
            elapsed = int(time.monotonic() - start)
            print(f"ok: PyPI trackfw {py_version} is live  (+{elapsed}s)")
            sys.exit(0)

        # Don't sleep past the deadline.
        remaining = deadline - elapsed
        actual_sleep = min(poll_interval, remaining)
        if actual_sleep > 0:
            print(f"  polling again in {actual_sleep}s...  (+{elapsed}s)")
            time.sleep(actual_sleep)
        poll_interval = min(poll_interval * 2, poll_max)


if __name__ == "__main__":
    main()
