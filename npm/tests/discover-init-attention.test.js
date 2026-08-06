'use strict'

// ML-1B (ROADMAP-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md)
//
// `trackfw discover --init` must generate scripts/trackfw-attention-signal.sh
// and scripts/trackfw-attention-cleanup.sh — same content trackfw init writes
// — before injecting hooks that reference them. See npm/src/generators/hooks.js
// (generateAttentionScripts) and the Python reference (pypi/trackfw/commands/
// discover.py, _generate_attention_scripts called before inject_hooks_detected).

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const bin = path.resolve(__dirname, '../bin/trackfw')

const { SIGNAL_SCRIPT, CLEANUP_SCRIPT, GUARD_SCRIPT } = (() => {
  const hooks = require('../src/generators/hooks')
  // The scripts module does not export the raw constants, so derive the
  // expected content the same way `trackfw init` does: by generating into a
  // throwaway directory and reading the result back.
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-attn-ref-'))
  hooks.generateAttentionScripts({}, tmp)
  hooks.generateCredentialGuardScript(tmp)
  const SIGNAL_SCRIPT = fs.readFileSync(path.join(tmp, 'scripts', 'trackfw-attention-signal.sh'), 'utf8')
  const CLEANUP_SCRIPT = fs.readFileSync(path.join(tmp, 'scripts', 'trackfw-attention-cleanup.sh'), 'utf8')
  const GUARD_SCRIPT = fs.readFileSync(path.join(tmp, 'scripts', 'trackfw-credential-guard.sh'), 'utf8')
  fs.rmSync(tmp, { recursive: true, force: true })
  return { SIGNAL_SCRIPT, CLEANUP_SCRIPT, GUARD_SCRIPT }
})()

function scratch() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-discover-init-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot, { recursive: true })
  fs.mkdirSync(homeRoot, { recursive: true })
  return { base, projectRoot, homeRoot }
}

function run(args, cwd, homeRoot) {
  return spawnSync(process.execPath, [bin, ...args], {
    cwd,
    env: { ...process.env, HOME: homeRoot },
    encoding: 'utf8',
  })
}

function signalPath(projectRoot) {
  return path.join(projectRoot, 'scripts', 'trackfw-attention-signal.sh')
}

function cleanupPath(projectRoot) {
  return path.join(projectRoot, 'scripts', 'trackfw-attention-cleanup.sh')
}

function guardPath(projectRoot) {
  return path.join(projectRoot, 'scripts', 'trackfw-credential-guard.sh')
}

test('discover --init generates trackfw-attention-signal.sh and trackfw-attention-cleanup.sh', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.ok(fs.existsSync(signalPath(projectRoot)), 'scripts/trackfw-attention-signal.sh should exist')
  assert.ok(fs.existsSync(cleanupPath(projectRoot)), 'scripts/trackfw-attention-cleanup.sh should exist')
})

test('discover --init generates trackfw-credential-guard.sh (same lifecycle as the attention scripts)', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  assert.ok(fs.existsSync(guardPath(projectRoot)), 'scripts/trackfw-credential-guard.sh should exist')

  const guardContent = fs.readFileSync(guardPath(projectRoot), 'utf8')
  assert.equal(guardContent, GUARD_SCRIPT)

  const guardMode = fs.statSync(guardPath(projectRoot)).mode & 0o777
  assert.equal(guardMode & 0o100, 0o100, 'credential guard script should be executable (owner +x)')
})

test('discover --init attention scripts are executable and byte-identical to trackfw init output', () => {
  const { projectRoot, homeRoot } = scratch()
  const result = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(result.status, 0, result.stderr)

  const signalContent = fs.readFileSync(signalPath(projectRoot), 'utf8')
  const cleanupContent = fs.readFileSync(cleanupPath(projectRoot), 'utf8')
  assert.equal(signalContent, SIGNAL_SCRIPT)
  assert.equal(cleanupContent, CLEANUP_SCRIPT)

  const signalMode = fs.statSync(signalPath(projectRoot)).mode & 0o777
  const cleanupMode = fs.statSync(cleanupPath(projectRoot)).mode & 0o777
  assert.equal(signalMode & 0o100, 0o100, 'signal script should be executable (owner +x)')
  assert.equal(cleanupMode & 0o100, 0o100, 'cleanup script should be executable (owner +x)')
})

test('discover --init is idempotent — running twice does not fail or corrupt the scripts', () => {
  const { projectRoot, homeRoot } = scratch()
  const first = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(first.status, 0, first.stderr)

  const beforeSignal = fs.readFileSync(signalPath(projectRoot), 'utf8')
  const beforeCleanup = fs.readFileSync(cleanupPath(projectRoot), 'utf8')

  // trackfw.yaml already exists after the first run, so discover --init
  // skips yaml (re)generation and gate installation on the second run —
  // exercise that path directly to confirm it does not error and does not
  // corrupt the already-written scripts.
  const second = run(['discover', '--init'], projectRoot, homeRoot)
  assert.equal(second.status, 0, second.stderr)

  const afterSignal = fs.readFileSync(signalPath(projectRoot), 'utf8')
  const afterCleanup = fs.readFileSync(cleanupPath(projectRoot), 'utf8')
  assert.equal(afterSignal, beforeSignal)
  assert.equal(afterCleanup, beforeCleanup)
})
