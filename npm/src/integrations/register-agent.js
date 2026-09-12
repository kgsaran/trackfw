'use strict'

// registerAgentInConfig — ML-2B (Node)
//
// When `trackfw agents install` runs inside a project configured with
// `roadmap_namespacing: by_agent`, the installed catalog item ID (e.g.
// "architect") is registered as a namespace entry in the `agents:` list of
// `trackfw.yaml`, enabling `trackfw req new` / `roadmap new` to route
// artefacts to that agent's subdirectory.
//
// Scope gate: only project-scoped installs write the key. Global installs
// (scope === "global") install files into the user's home directory and do
// NOT own any project's trackfw.yaml — we must not write it.
//
// Format preservation: we use yaml.parseDocument / doc.toString() which is
// the documented round-trip that preserves comments, key order and blank
// lines. We do NOT re-serialise the whole document through yaml.dump so that
// a diff of trackfw.yaml only shows the added line(s) in the agents: block
// (AC3). See the AC3 test for the exact assertion.
//
// Idempotency: if the agent name is already in the list we return early
// without touching the file (AC1).
//
// flat mode: if roadmap_namespacing is not "by_agent" we do nothing at all —
// the agents: key MUST NOT be created (AC2).
//
// Inline-flow guard: if agents: is written in inline/flow style (e.g.
// `agents: [alpha, beta]`) we do NOT rewrite the file. Rewriting would change
// the user's formatting without their consent. Instead we emit a warning to
// stderr naming the file path and the item, and return — the persona install
// itself succeeds. Mirrors Go behaviour (the reference runtime).

const fs = require('node:fs')
const path = require('node:path')
const { parseDocument } = require('yaml')

const CONFIG_FILE = 'trackfw.yaml'

/**
 * Return true if the raw YAML text has an `agents:` key written in inline/flow
 * style on the same line (e.g. `agents: [alpha, beta]`), ignoring comment lines.
 *
 * We test on the raw text before parsing so that we never mutate a document
 * whose style we cannot faithfully reproduce.
 */
function isAgentsInlineFlow(raw) {
  for (const line of raw.split('\n')) {
    const trimmed = line.trimStart()
    if (trimmed.startsWith('#')) continue
    // Non-comment line that starts the `agents:` mapping key and has `[` on it
    if (/^agents\s*:.*\[/.test(trimmed)) return true
  }
  return false
}

/**
 * Register `agentName` in the `agents:` list of `<projectRoot>/trackfw.yaml`.
 *
 * No-op when:
 *  - trackfw.yaml does not exist in projectRoot (nothing to update)
 *  - roadmap_namespacing is absent or not "by_agent"         (AC2)
 *  - agentName is already present in the list                (AC1 idempotency)
 *  - agents: is in inline/flow format — warns to stderr, skips write
 *
 * @param {string} projectRoot  Absolute path to the project directory.
 * @param {string} agentName    Catalog item ID (e.g. "architect").
 */
function registerAgentInConfig(projectRoot, agentName) {
  const cfgPath = path.join(projectRoot, CONFIG_FILE)
  if (!fs.existsSync(cfgPath)) return

  const raw = fs.readFileSync(cfgPath, 'utf8')
  const doc = parseDocument(raw)

  // AC2 gate: only act when roadmap_namespacing is exactly "by_agent"
  const namespacing = doc.get('roadmap_namespacing')
  if (namespacing !== 'by_agent') return

  // Inline-flow guard: refuse to rewrite a file the user authored in flow style.
  // Mirrors Go: "warning: could not register agent ... trackfw.yaml has agents:
  // in inline-flow format; edit <path> manually to add <item>"
  if (isAgentsInlineFlow(raw)) {
    process.stderr.write(
      `warning: could not register agent "${agentName}" in trackfw.yaml:\n` +
      `trackfw.yaml has agents: in inline-flow format; edit ${cfgPath} manually to add "${agentName}"\n`
    )
    return
  }

  // Collect current agents list (may be absent)
  const agentsNode = doc.getIn(['agents'], true)

  if (agentsNode) {
    // List present — check for idempotency (AC1)
    const existing = agentsNode.items.map(item => item.value)
    if (existing.includes(agentName)) return
    agentsNode.items.push(doc.createNode(agentName))
  } else {
    // List absent — create it (AC2 already passed: namespacing === 'by_agent')
    doc.set('agents', doc.createNode([agentName]))
  }

  fs.writeFileSync(cfgPath, doc.toString(), 'utf8')
}

module.exports = { registerAgentInConfig }
