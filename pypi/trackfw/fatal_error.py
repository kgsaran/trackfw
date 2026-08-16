"""fatal_error.py — global backstop for uncaught exceptions in trackfw.cli:main().

Defense in depth (REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-
caminhos-absolutos-e-versao-do-runtime). The measured leak was in the Node CLI:
commands that catch internally (e.g. trackfw.integrations.command.run(), which
already prints "trackfw <kind> <action>: <error>" for IntegrationError/OSError/
ValueError) print clean; anything that escapes uncaught would otherwise let
Python's default excepthook dump the full traceback, including absolute
source-file paths and line numbers. This module is installed once, at the
single dispatch point in cli.py:main() — not per command — for the same
reason the Node fix lives in bin/trackfw and not in every command file: the
leak was per-command, and a per-entrypoint handler closes every future gap at
once.

TRACKFW_DEBUG=1 restores the full traceback — this must never blind someone
who genuinely needs to debug a crash.
"""

from __future__ import annotations

import os
import sys
import traceback


def report_fatal_error(error: BaseException, command: str | None = None) -> None:
    """Prints a clean, single-line error to stderr — or the full traceback when
    TRACKFW_DEBUG=1. Deliberately mirrors npm/src/lib/fatal-error.js:
    TRACKFW_DEBUG prints the traceback (which already includes the exception
    message as its own first/last line) instead of the traceback AND the
    clean message, to avoid printing the message twice.
    """
    if os.environ.get("TRACKFW_DEBUG") == "1":
        traceback.print_exc(file=sys.stderr)
        return
    message = str(error) or error.__class__.__name__
    prefix = f"trackfw {command}" if command else "trackfw"
    print(f"{prefix}: {message}", file=sys.stderr)
