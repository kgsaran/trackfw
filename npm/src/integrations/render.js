'use strict'

const { lookup: lookupIdentity, agentName } = require('../identity')

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

// toolsFor retorna SET_ARCH para o agente canônico "architect" (item.id do
// catálogo, não o nome renderizado — que pode ser customizado pela
// identidade), SET_IMPL para os demais. IDs proibidos (edit_file, read_file,
// find, view_code_item, view_file_outline, call_mcp_tool) nunca fazem parte
// de nenhum dos conjuntos.
function toolsFor(itemId) {
  return itemId === 'architect' ? SET_ARCH : SET_IMPL
}

// greetingLine monta a primeira linha injetada no corpo do agente quando há
// identidade configurada. Sem apelido configurado, apenas o display name do
// agente é mencionado. Espelha internal/integrations/render.go:greetingLine.
function greetingLine(displayName, nickname) {
  if (!nickname) return `Você é ${displayName}.`
  return `Você é ${displayName}. Trate o usuário como ${nickname}.`
}

// insertBodyPrefix insere prefix como a nova primeira linha da seção de
// corpo de um markdown cru (frontmatter + corpo), seguida de linha em
// branco. Se source não tiver frontmatter reconhecível, prefix é inserido no
// topo. Reusa a mesma detecção de fronteira do frontmatter usada por
// markdownParts, para que Rota A e Rota B concordem sobre onde o corpo
// começa. Espelha internal/integrations/render.go:insertBodyPrefix.
function insertBodyPrefix(source, prefix) {
  const trimmed = String(source).trim()
  if (!prefix) return trimmed
  if (!trimmed.startsWith('---\n')) return `${prefix}\n\n${trimmed}`
  const end = trimmed.indexOf('\n---', 4)
  if (end < 0) return `${prefix}\n\n${trimmed}`
  const insertAt = end + 4
  const head = trimmed.slice(0, insertAt)
  const rest = trimmed.slice(insertAt).replace(/^\n+/, '')
  if (rest === '') return `${head}\n\n${prefix}`
  return `${head}\n\n${prefix}\n\n${rest}`
}

// rewriteFrontmatterFields substitui as linhas "name:" e "description:" do
// frontmatter de um markdown cru por name e description, preservando toda
// outra linha do frontmatter byte a byte (ordem, espaçamento, estilo de
// aspas) e deixando o corpo intocado. Usado pela Rota B (o branch default de
// render) para que representações que consomem o frontmatter cru —
// principalmente "subagent" (claude, gemini, cursor, copilot, kiro-ide,
// windsurf) — recebam a identidade customizada. Se o frontmatter não tiver
// "name:" ou "description:", a chave é simplesmente deixada ausente — esta
// função nunca inventa uma chave que não existia. Se source não tiver
// frontmatter reconhecível, source é retornado sem alteração (trimado).
// Espelha internal/integrations/render.go:rewriteFrontmatterFields.
function rewriteFrontmatterFields(source, name, description) {
  const trimmed = String(source).trim()
  if (!trimmed.startsWith('---\n')) return trimmed
  const end = trimmed.indexOf('\n---', 4)
  if (end < 0) return trimmed
  const frontmatter = trimmed.slice(4, end)
  const rest = trimmed.slice(end) // começa com "\n---", seguido do corpo

  const lines = frontmatter.split('\n').map(line => {
    const separator = line.indexOf(':')
    if (separator < 0) return line
    const key = line.slice(0, separator).trim()
    let replacement
    if (key === 'name') replacement = name
    else if (key === 'description') replacement = description
    else return line
    const value = line.slice(separator + 1).trim()
    const quoted = value.length >= 2 && value.startsWith('"') && value.endsWith('"')
    return quoted ? `${key}: "${replacement}"` : `${key}: ${replacement}`
  })

  return `---\n${lines.join('\n')}${rest}`
}

// frontmatterName extrai apenas o campo "name" de um frontmatter YAML
// delimitado por ---, sem os valores default aplicados por markdownParts.
// Retorna undefined quando o arquivo não tem frontmatter reconhecível ou não
// declara "name". Usado pela detecção de colisão em manager.js. Espelha
// internal/integrations/render.go:frontmatterName.
function frontmatterName(content) {
  const text = String(content).trim()
  if (!text.startsWith('---\n')) return undefined
  const end = text.indexOf('\n---', 4)
  if (end < 0) return undefined
  const frontmatter = text.slice(4, end)
  for (const line of frontmatter.split('\n')) {
    const separator = line.indexOf(':')
    if (separator < 0) continue
    const key = line.slice(0, separator).trim()
    if (key !== 'name') continue
    let value = line.slice(separator + 1).trim()
    value = value.replace(/^"+/, '').replace(/"+$/, '')
    if (value === '') return undefined
    return value
  }
  return undefined
}

// render converte um item canônico do catálogo para a representação nativa
// declarada por uma surface alvo. Quando identity carrega uma identidade
// customizada para item.id, o name/description/body renderizados são
// personalizados (ADR ADR-2026-07-25-identidade-personalizavel-de-agentes,
// seções D1/D2/D6).
//
// render tem duas rotas de saída:
//   - Rota A: "custom-agent-toml", "cli-agent-json", "agent-json" e
//     "agent-directory" trabalham a partir de name/description/body já
//     separados do frontmatter por markdownParts.
//   - Rota B: o branch default, usado pela representação "subagent" (claude,
//     gemini, cursor, copilot, kiro-ide, windsurf), retorna o source cru
//     normalizado com o frontmatter ainda anexado. Quando há identidade
//     configurada, suas linhas "name:"/"description:" são reescritas no
//     lugar (ver rewriteFrontmatterFields) — necessário porque a seleção de
//     subagent do Claude Code lê apenas o frontmatter, nunca o corpo.
//
// Ambas as rotas devem receber a injeção de identidade. Quando não há
// identidade configurada para item.id, name/description/body ficam
// exatamente como markdownParts produziu e o branch default retorna
// normalize(content) — a mesma expressão usada antes de existir suporte a
// identidade — então a saída sem identidade é garantida byte a byte
// inalterada por construção, não por coincidência.
function render({ kind, content, capability, item, identity: cfg }) {
  if (kind === 'skills') return normalize(content)

  const id = item && cfg ? lookupIdentity(cfg, item.id) : undefined
  const parts = markdownParts(content)
  let { name, description, body } = parts
  let greeting = ''
  if (id) {
    greeting = greetingLine(id.display_name, (cfg && cfg.user_nickname) || '')
    name = agentName(id.slug)
    description = `${id.display_name} — ${description}`
    body = `${greeting}\n\n${body}`
  }

  if (capability.representation === 'custom-agent-toml') {
    return `name = ${JSON.stringify(name.replaceAll('-', '_'))}\ndescription = ${JSON.stringify(description)}\ndeveloper_instructions = ${JSON.stringify(body)}\n`
  }
  if (capability.representation === 'cli-agent-json' || capability.representation === 'agent-json') {
    return `${JSON.stringify({ name, description, prompt: body }, null, 2)}\n`
  }
  if (capability.representation === 'agent-directory') {
    // Reconstrói o frontmatter para o Antigravity CLI (agy):
    // - mapeia model canônico para o tier aceito (opus→pro, sonnet→flash)
    // - injeta tools: SET_IMPL ou SET_ARCH dependendo do item.id (não do
    //   nome renderizado, que pode ser customizado pela identidade)
    // - omite campos não suportados pelo agy
    const mappedModel = resolveModel(parts.model)
    const tools = toolsFor(item && item.id)
    let out = `---\nname: ${name}\ndescription: ${description}\n`
    if (mappedModel) out += `model: ${mappedModel}\n`
    out += 'tools:\n'
    for (const tool of tools) out += `  - ${tool}\n`
    out += '---\n'
    if (body) out += `${body}\n`
    return out
  }

  if (!id) return normalize(content)
  const withBody = insertBodyPrefix(content, greeting)
  const withFrontmatter = rewriteFrontmatterFields(withBody, name, description)
  return normalize(withFrontmatter)
}

module.exports = { render, markdownParts, frontmatterName, greetingLine, insertBodyPrefix, rewriteFrontmatterFields }
