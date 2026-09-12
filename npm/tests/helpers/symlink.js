'use strict'
/**
 * symlinkOrSkip — canonical helper for symlink-creation in test files.
 *
 * Single authoritative implementation; extracted so that
 * update_discover_symlink_guard.test.js and roadmap_move.test.js (which use
 * different harnesses) share the same detection logic without diverging copies.
 *
 * Wraps fs.symlinkSync so that:
 *   - on success: returns true
 *   - if privilege is lacking (Windows without Developer Mode —
 *     EPERM/EACCES): calls onPrivilegeError(err) and returns false
 *   - on any other error: rethrows (caller fails, not skips — the guard
 *     discriminates "no privilege" from "failed for another reason")
 *
 * Detection is on the failed syscall's error code, not on process.platform:
 * a Windows runner with Developer Mode enabled lets symlinkSync succeed and
 * the test executes normally.
 *
 * Callers supply onPrivilegeError to adapt to their harness:
 *   node:test  → err => t.skip(`msg: ${err.message}`)
 *   custom     → err => { throw new SymlinkPrivilegeSkip(err.message) }
 *
 * SymlinkPrivilegeSkip is exported so custom harnesses can catch it as a
 * distinct sentinel — separate from genuine test failures.
 */

const fs = require('node:fs')

class SymlinkPrivilegeSkip extends Error {
  constructor (msg) {
    super(msg)
    this.name = 'SymlinkPrivilegeSkip'
  }
}

/**
 * @param {string} target
 * @param {string} link
 * @param {(err: NodeJS.ErrnoException) => void} onPrivilegeError
 * @returns {boolean} true if symlink was created; false if test was skipped
 */
function symlinkOrSkip (target, link, onPrivilegeError) {
  try {
    fs.symlinkSync(target, link)
    return true
  } catch (err) {
    if (err && (err.code === 'EPERM' || err.code === 'EACCES')) {
      onPrivilegeError(err)
      return false
    }
    // Any other error (ENOENT, EEXIST, …) is a genuine test failure — rethrow.
    throw err
  }
}

module.exports = { symlinkOrSkip, SymlinkPrivilegeSkip }
