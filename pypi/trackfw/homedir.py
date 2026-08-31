"""homedir.py — resolves the user's home directory consistently across platforms.

Mirrors internal/homedir/homedir.go (Go, canonical source of truth) and
npm/src/homedir.js (Node.js). See that file's doc comment for the full rationale.

Why it exists: os.path.expanduser("~") reads $HOME on Linux and macOS, but
%USERPROFILE% on Windows. Tests and gates isolate the home directory with
HOME=<tempdir>, which on Windows isolates nothing — the process keeps reading and
writing the developer's real home.

home_dir() makes Windows behave like the other platforms: $HOME first,
expanduser("~") as the fallback. Where $HOME is unset nothing changes.

The empty string does NOT count as set: HOME="" would resolve to "" and every
derived path would silently become relative.

Two families, and both matter:

  home_dir()      "give me the home directory"
  expand_path(p)  "expand the ~ in this path" — used for adr_dirs in trackfw.yaml,
                  which also resolved through %USERPROFILE% before this fix.
"""

import os


def home_dir() -> str:
    """The user's home directory, preferring $HOME when set and non-empty."""
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
