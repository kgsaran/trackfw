'use strict'

const fs = require('node:fs')

// fatal-error.js — global backstop for errors that escape every commander
// action (sync or async, since npm/bin/trackfw drives the program via
// parseAsync()). Installed ONCE at the entrypoint, never per-command:
// the real leak pattern (REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-
// stack-trace-caminhos-absolutos-e-versao-do-runtime) was per-command —
// commands that catch internally (e.g. `roadmap move`) already print a
// clean message via console.error + process.exitCode, but any command that
// doesn't catch let Node's own uncaught-exception/unhandled-rejection
// printer dump the full stack, absolute install path and runtime version.
// A single handler at the entrypoint closes every one of those gaps at
// once instead of relying on every future command remembering a try/catch.
//
// TRACKFW_DEBUG=1 restores the full stack — this must never blind someone
// who genuinely needs to debug a crash.
//
// Writes with fs.writeSync(2, ...) — NOT process.stderr.write() — on
// purpose: installGlobalHandlers() below calls process.exit() right after
// this returns, and when stderr is a pipe (exactly what every spawnSync-
// based test in this repo uses, and what a piped CI log is) Node's stream
// writes are asynchronous; process.exit() can drop a still-pending write
// before the OS ever sees it, silently eating the message (failure mode
// #2 in the REQ: "engolir a mensagem" — worse than the leak this handler
// exists to fix). fs.writeSync is a raw, synchronous syscall — it cannot
// race process.exit().
function reportFatalError(err) {
  // TRACKFW_DEBUG=1: print the full stack (its first line already is
  // "Error: <message>", so this is the debug-only superset of the clean
  // path below, not a duplicate of it).
  if (process.env.TRACKFW_DEBUG === '1' && err && err.stack) {
    fs.writeSync(2, `${err.stack}\n`)
    return
  }
  const message = err && err.message !== undefined ? err.message : String(err)
  fs.writeSync(2, `Error: ${message}\n`)
}

// installGlobalHandlers — defense in depth beyond the parseAsync().catch()
// in bin/trackfw. Covers errors that reach the process outside that single
// promise chain (e.g. a rejection from a detached/floating promise started
// by a command, or a genuine uncaughtException). Per Node's own contract,
// once a listener is attached Node no longer prints its default trace and
// no longer auto-exits — the listener owns both the message and the exit.
// Both listeners call process.exit() immediately after reportFatalError()
// returns — safe here specifically because reportFatalError writes
// synchronously (see comment above); process.exitCode (which waits for the
// event loop to drain) would be wrong here regardless, since long-running
// commands (e.g. `serve`) hold the loop open and the process would hang
// instead of exiting on a genuinely fatal error.
function installGlobalHandlers() {
  process.on('unhandledRejection', (err) => {
    reportFatalError(err)
    process.exit(1)
  })
  process.on('uncaughtException', (err) => {
    reportFatalError(err)
    process.exit(1)
  })
}

module.exports = { reportFatalError, installGlobalHandlers }
