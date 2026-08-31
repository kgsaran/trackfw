"""homedir.py — resolves the user's home directory consistently across platforms.

Mirrors internal/homedir/homedir.go (Go, canonical source of truth) and
npm/src/homedir.js (Node.js). See that file's doc comment for the full rationale.

Why it exists: os.path.expanduser("~") reads $HOME on Linux and macOS, but
%USERPROFILE% on Windows. Tests and gates isolate the home directory with
HOME=<tempdir>, which on Windows isolates nothing — the process keeps reading and
writing the developer's real home.

home_dir() makes Windows behave like the other platforms: $HOME first,
expanduser("~") as the fallback — **on Windows only**. See the function's own
docstring for why the platform guard is there. Where $HOME is unset nothing
changes.

The empty string does NOT count as set: HOME="" would resolve to "" and every
derived path would silently become relative.

Two families, and both matter:

  home_dir()      "give me the home directory"
  expand_path(p)  "expand the ~ in this path" — used for adr_dirs in trackfw.yaml,
                  which also resolved through %USERPROFILE% before this fix.
"""

import os
import sys


def home_dir() -> str:
    """The user's home directory. Prefers $HOME **on Windows only**.

    On Linux and macOS os.path.expanduser("~") already reads $HOME, so preferring
    the variable there fixes nothing — and it BREAKS the home isolation of several
    tests in this repository, which isolate by patching the FUNCTION rather than
    the variable:

        monkeypatch.setattr("os.path.expanduser",
                            lambda p: str(home) if p == "~" else os.path.expanduser(p))

    Reading os.environ["HOME"] first bypasses that patch, and production walks to
    the runner's REAL home. Measured on a Linux CI run: three tests failing —
    test_identity_wizard.py::test_agents_install_with_existing_identity_and_no_flag_does_not_invoke,
    test_scope_resolution.py::test_targets_flag_with_tty_and_no_scope_still_triggers_scope_resolver
    and test_thirdparty.py::test_install_global_scope_requires_its_own_confirmation.
    One of them fails with "OSError: pytest: reading from stdin while output is
    captured!" — production found the wrong home, did not find the identity the
    test had written, and went on to prompt.

    The platform guard keeps Linux and macOS byte-for-byte identical to the
    previous behavior and fixes only where the defect exists.

    Go and Node.js need no such guard: os.UserHomeDir() and os.homedir() already
    read $HOME on POSIX, so preferring the variable there is a no-op, and both
    suites isolate by environment variable rather than by patching a function.
    """
    if sys.platform == "win32":
        from_env = os.environ.get("HOME")
        if from_env:
            return from_env
    return os.path.expanduser("~")


def expand_path(path):
    """Expand a leading `~` using home_dir(). Mirrors config.ExpandPath (Go).

    Returns the value untouched when it does not start with `~`, or is not a str.
    """
    if not path or not isinstance(path, str):
        return path
    if path == "~":
        return home_dir()
    if path.startswith("~/") or path.startswith("~" + chr(92)):
        return os.path.join(home_dir(), path[2:])
    return path
