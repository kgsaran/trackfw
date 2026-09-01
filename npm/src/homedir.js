'use strict'

/**
 * homedir.js — resolves the user's home directory consistently across platforms.
 *
 * Mirrors internal/homedir/homedir.go (Go, canonical source of truth) and
 * pypi/trackfw/homedir.py (Python). See that file's doc comment for the full
 * rationale.
 *
 * Why it exists: os.homedir() reads $HOME on Linux and macOS, but %USERPROFILE%
 * on Windows. Tests and gates isolate the home directory with HOME=<tempdir>,
 * which on Windows isolates nothing — the process keeps reading and writing the
 * developer's real home.
 *
 * homedir() makes Windows behave like the other platforms: $HOME first,
 * os.homedir() as the fallback. Where $HOME is unset nothing changes.
 *
 * The empty string does NOT count as set: HOME="" would resolve to "" and every
 * derived path would silently become relative.
 */

const os = require('os')

function homedir() {
  const fromEnv = process.env.HOME
  if (fromEnv) return fromEnv
  return os.homedir()
}

module.exports = { homedir }
