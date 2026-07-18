'use strict'

function normalize(content) {
  return `${content.trim()}\n`
}

function markdownParts(content) {
  const text = content.trim()
  let name = 'trackfw-agent'
  let description = 'trackfw specialist'
  let body = text
  if (text.startsWith('---\n')) {
    const end = text.indexOf('\n---', 4)
    if (end >= 0) {
      const frontmatter = text.slice(4, end)
      body = text.slice(end + 4).trim()
      for (const line of frontmatter.split('\n')) {
        const separator = line.indexOf(':')
        if (separator < 0) continue
        const key = line.slice(0, separator).trim()
        const value = line.slice(separator + 1).trim().replace(/^['"]|['"]$/g, '')
        if (key === 'name') name = value
        if (key === 'description') description = value
      }
    }
  }
  return { name, description, body }
}

function render({ kind, content, capability }) {
  if (kind === 'skills') return normalize(content)
  const parts = markdownParts(content)
  if (capability.representation === 'custom-agent-toml') {
    return `name = ${JSON.stringify(parts.name.replace(/^trackfw-/, ''))}\ndescription = ${JSON.stringify(parts.description)}\ndeveloper_instructions = ${JSON.stringify(parts.body)}\n`
  }
  if (capability.representation === 'cli-agent-json' || capability.representation === 'agent-json') {
    return `${JSON.stringify({ name: parts.name, description: parts.description, prompt: parts.body }, null, 2)}\n`
  }
  return normalize(content)
}

module.exports = { render, markdownParts }
