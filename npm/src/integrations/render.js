'use strict'

// Mapa de model: nomes canônicos do catálogo → tier aceito pelo Antigravity CLI (agy)
const MODEL_MAP = { opus: 'pro', sonnet: 'flash' }
// Valores que já são tiers válidos do agy e devem passar sem transformação
const MODEL_PASSTHROUGH = new Set(['flash_lite', 'flash', 'pro'])

// SET_IMPL — conjunto base de 10 ferramentas para agentes de implementação
const SET_IMPL = [
  'view_file',
  'list_dir',
  'grep_search',
  'search_web',
  'read_url_content',
  'write_to_file',
  'replace_file_content',
  'run_command',
  'command_status',
  'generate_image',
]

// SET_ARCH — SET_IMPL + 4 ferramentas de orquestração (total 14) para agentes arquitetos
const SET_ARCH = [
  ...SET_IMPL,
  'send_message',
  'define_subagent',
  'invoke_subagent',
  'schedule',
]

function normalize(content) {
  return `${content.trim()}\n`
}

function markdownParts(content) {
  const text = content.trim()
  let name = 'trackfw-agent'
  let description = 'trackfw specialist'
  let model = ''
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
        if (key === 'model') model = value
      }
    }
  }
  return { name, description, model, body }
}

// resolveModel converte o modelo canônico para o tier aceito pelo agy.
// Retorna o valor mapeado, ou string vazia se a linha model deve ser omitida.
function resolveModel(model) {
  if (!model) return ''
  if (MODEL_PASSTHROUGH.has(model)) return model
  return MODEL_MAP[model] || ''
}

// toolsFor retorna SET_ARCH para agentes cujo nome termina em "architect", SET_IMPL para os demais.
// IDs proibidos (edit_file, read_file, find, view_code_item, view_file_outline, call_mcp_tool)
// nunca fazem parte de nenhum dos conjuntos.
function toolsFor(name) {
  return name.endsWith('architect') ? SET_ARCH : SET_IMPL
}

function render({ kind, content, capability }) {
  if (kind === 'skills') return normalize(content)
  const parts = markdownParts(content)
  if (capability.representation === 'custom-agent-toml') {
    return `name = ${JSON.stringify(parts.name.replaceAll('-', '_'))}\ndescription = ${JSON.stringify(parts.description)}\ndeveloper_instructions = ${JSON.stringify(parts.body)}\n`
  }
  if (capability.representation === 'cli-agent-json' || capability.representation === 'agent-json') {
    return `${JSON.stringify({ name: parts.name, description: parts.description, prompt: parts.body }, null, 2)}\n`
  }
  if (capability.representation === 'agent-directory') {
    // Reconstrói o frontmatter para o Antigravity CLI (agy):
    // - mapeia model canônico para o tier aceito (opus→pro, sonnet→flash)
    // - injeta tools: SET_IMPL ou SET_ARCH dependendo do nome do agente
    // - omite campos não suportados pelo agy
    const mappedModel = resolveModel(parts.model)
    const tools = toolsFor(parts.name)
    let out = `---\nname: ${parts.name}\ndescription: ${parts.description}\n`
    if (mappedModel) out += `model: ${mappedModel}\n`
    out += 'tools:\n'
    for (const tool of tools) out += `  - ${tool}\n`
    out += '---\n'
    if (parts.body) out += `${parts.body}\n`
    return out
  }
  return normalize(content)
}

module.exports = { render, markdownParts }
