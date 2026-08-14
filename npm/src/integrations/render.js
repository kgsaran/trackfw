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

// Mapa de model: nomes canônicos do catálogo → ID de modelo aceito pelo Codex
// CLI. Fonte: documentação Codex CLI pesquisada em 2026-08-14 (ver ADR
// ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-
// codex-e-cursor). IDs de modelo Codex são versionados e mudam com o ciclo de
// release da OpenAI.
const CODEX_MODEL_MAP = { opus: 'gpt-5.4', sonnet: 'gpt-5.4-mini' }

// resolveModelCodex converte o modelo canônico para o ID de modelo aceito
// pelo Codex CLI. Retorna o valor mapeado, ou string vazia se a linha model
// deve ser omitida (valor desconhecido ou ausente). Espelha
// internal/integrations/render.go:mapModelCodex.
function resolveModelCodex(model) {
  if (!model) return ''
  return CODEX_MODEL_MAP[model] || ''
}

// Mapa de model: nomes canônicos do catálogo → valor aceito pela Cursor
// (fonte: cursor.com/docs/subagents, ver ADR ADR-2026-08-14-roteamento-de-
// model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor).
const CURSOR_MODEL_MAP = { opus: 'claude-opus-5[effort=high]', sonnet: 'composer-2.5[fast=true]' }

// mapModelCursor converte o modelo canônico para o valor aceito pela Cursor.
// Retorna o valor mapeado, ou string vazia se a linha "model:" deve ser
// removida do frontmatter (Cursor cai no default "inherit"/Auto). Espelha
// internal/integrations/render.go:mapModelCursor.
function mapModelCursor(model) {
  if (!model) return ''
  return CURSOR_MODEL_MAP[model] || ''
}

// rewriteFrontmatterModelLine substitui a linha "model:" do frontmatter de um
// markdown cru por value, preservando toda outra linha do frontmatter e o
// corpo byte a byte. Se o frontmatter não tiver linha "model:", uma é
// anexada como última linha do bloco de frontmatter. Se source não tiver
// frontmatter reconhecível, source é retornado sem alteração (trimado) —
// espelha a detecção de fronteira usada por rewriteFrontmatterFields,
// escopada à chave única "model". Espelha
// internal/integrations/render.go:rewriteFrontmatterModelLine.
function rewriteFrontmatterModelLine(source, value) {
  const trimmed = String(source).trim()
  if (!trimmed.startsWith('---\n')) return trimmed
  const end = trimmed.indexOf('\n---', 4)
  if (end < 0) return trimmed
  const frontmatter = trimmed.slice(4, end)
  const rest = trimmed.slice(end) // começa com "\n---", seguido do corpo

  const lines = frontmatter.split('\n')
  let found = false
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const separator = line.indexOf(':')
    if (separator < 0 || line.slice(0, separator).trim() !== 'model') continue
    const rawValue = line.slice(separator + 1).trim()
    const quoted = rawValue.length >= 2 && rawValue.startsWith('"') && rawValue.endsWith('"')
    lines[i] = quoted ? `model: "${value}"` : `model: ${value}`
    found = true
    break
  }
  if (!found) lines.push(`model: ${value}`)

  return `---\n${lines.join('\n')}${rest}`
}

// removeFrontmatterModelLine remove a linha "model:" do frontmatter de um
// markdown cru, se presente, preservando toda outra linha do frontmatter e o
// corpo byte a byte. Se source não tiver linha "model:" ou frontmatter
// reconhecível, source é retornado sem alteração (trimado). Espelha
// internal/integrations/render.go:removeFrontmatterModelLine.
function removeFrontmatterModelLine(source) {
  const trimmed = String(source).trim()
  if (!trimmed.startsWith('---\n')) return trimmed
  const end = trimmed.indexOf('\n---', 4)
  if (end < 0) return trimmed
  const frontmatter = trimmed.slice(4, end)
  const rest = trimmed.slice(end) // começa com "\n---", seguido do corpo

  const lines = frontmatter.split('\n')
  const kept = lines.filter(line => {
    const separator = line.indexOf(':')
    return !(separator >= 0 && line.slice(0, separator).trim() === 'model')
  })
  if (kept.length === lines.length) return trimmed

  return `---\n${kept.join('\n')}${rest}`
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
  if (!nickname) return `You are ${displayName}.`
  return `You are ${displayName}. Address the user as ${nickname}.`
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

// rewriteSignatureLine reescreve a última linha da seção de corpo de um
// markdown cru (frontmatter + corpo) que casa com o padrão de assinatura
// `^— <nome>, <título>$` (travessão em-dash U+2014, espaço, nome, vírgula,
// espaço, título). Apenas o grupo de nome (primeiro) é substituído por
// displayName; o título (segundo grupo) é preservado byte a byte.
//
// Escopo: a função opera somente no corpo (após o fechamento do frontmatter).
// Uma linha de assinatura dentro do frontmatter nunca é tocada — a detecção
// de fronteira espelha rewriteFrontmatterFields exatamente.
//
// Se nenhuma linha do corpo casar com o padrão, source é retornado inalterado.
// Se displayName for vazio, source é retornado inalterado. A função nunca
// inventa uma assinatura que não estava presente.
//
// Espelha internal/integrations/render.go:rewriteSignatureLine.
function rewriteSignatureLine(source, displayName) {
  if (!displayName) return source
  const trimmed = String(source).trim()

  // Localiza o início do corpo — espelha a detecção de fronteira de
  // rewriteFrontmatterFields para que o escopo de ambas as funções coincida.
  let bodyStart = 0
  if (trimmed.startsWith('---\n')) {
    const end = trimmed.indexOf('\n---', 4)
    if (end >= 0) {
      bodyStart = end + 4 // aponta para o char imediatamente após "\n---"
    }
  }
  const head = trimmed.slice(0, bodyStart)
  const bodySection = trimmed.slice(bodyStart)

  const lines = bodySection.split('\n')
  // Percorre de trás para frente para encontrar a ÚLTIMA linha candidata.
  for (let i = lines.length - 1; i >= 0; i--) {
    const line = lines[i]
    if (!line.startsWith('— ')) continue
    const rest = line.slice('— '.length) // pula "— "
    const commaIdx = rest.indexOf(', ')
    if (commaIdx < 0) continue
    const title = rest.slice(commaIdx + 2)
    if (!title) continue
    lines[i] = `— ${displayName}, ${title}`
    return head + lines.join('\n')
  }
  // Nenhuma linha de assinatura encontrada — retorna source inalterado.
  return source
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
//     lugar (ver rewriteFrontmatterFields) e a última linha de assinatura do
//     corpo com padrão `^— <nome>, <título>$` é reescrita com o display name
//     da identidade (ver rewriteSignatureLine) — necessário porque a seleção
//     de subagent do Claude Code lê apenas o frontmatter, nunca o corpo.
//
// Ambas as rotas devem receber a injeção de identidade. Quando não há
// identidade configurada para item.id, name/description/body ficam
// exatamente como markdownParts produziu e o branch default retorna
// normalize(content) — a mesma expressão usada antes de existir suporte a
// identidade — então a saída sem identidade é garantida byte a byte
// inalterada por construção, não por coincidência.
function render({ kind, content, capability, item, identity: cfg, target }) {
  if (kind === 'skills') return normalize(content)

  const id = item && cfg ? lookupIdentity(cfg, item.id) : undefined
  const parts = markdownParts(content)
  let { name, description, body } = parts
  let source = content
  let greeting = ''
  if (id) {
    greeting = greetingLine(id.display_name, (cfg && cfg.user_nickname) || '')
    name = agentName(id.slug)
    description = `${id.display_name} — ${description}`
    body = `${greeting}\n\n${body}`
  }

  if (capability.representation === 'custom-agent-toml') {
    const lines = [
      `name = ${JSON.stringify(name.replaceAll('-', '_'))}`,
      `description = ${JSON.stringify(description)}`,
    ]
    const mappedModel = resolveModelCodex(parts.model)
    if (mappedModel) lines.push(`model = ${JSON.stringify(mappedModel)}`)
    lines.push(`developer_instructions = ${JSON.stringify(body)}`)
    return `${lines.join('\n')}\n`
  }
  if (capability.representation === 'cli-agent-json' || capability.representation === 'agent-json') {
    // Ordem alfabética das chaves (description, name, prompt) é obrigatória:
    // Go serializa via json.MarshalIndent(map[string]string{...}), cujo encoder
    // ordena as chaves alfabeticamente, e Python usa sort_keys. JSON.stringify
    // preserva a ordem de inserção, então a ordem é fixada aqui à mão. Sem
    // isso, instalar por um CLI e listar por outro reporta falso `modified`
    // (o manifest indexa artefatos por sha256 do conteúdo).
    return `${JSON.stringify({ description, name, prompt: body }, null, 2)}\n`
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

  if (capability.representation === 'opencode-agent') {
    // Reconstrói o frontmatter para o OpenCode CLI (opencode.ai), seguindo o
    // mesmo padrão de reconstrução-do-zero do case "agent-directory".
    // Decisão registrada na Wave 1 do roadmap
    // ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai (achado #3,
    // pesquisa contra o binário real 1.18.13):
    //   - "tools:" é uma chave RESERVADA no schema de agente do OpenCode
    //     (espera um objeto de overrides por-ferramenta, ex. {bash: false},
    //     não uma lista de nomes estilo Claude Code) — reutilizar o
    //     frontmatter original faz o OpenCode recusar o carregamento INTEIRO
    //     do projeto ("Configuration is invalid"), não só daquele agente. Por
    //     isso "tools:" nunca é emitido aqui.
    //   - sem "mode:" explícito, o OpenCode assume mode "all" (agente
    //     selecionável como persona primária de chat) — os agentes trackfw
    //     devem ser sempre subagentes puros, nunca primários, para paridade
    //     com o comportamento nos demais targets. Por isso "mode: subagent" é
    //     sempre fixo, nunca omitido.
    //   - "model:" é deliberadamente OMITIDO (decisão de produto do
    //     orquestrador, não uma limitação técnica): o OpenCode espera
    //     "provider/model-id" (ex. "anthropic/claude-sonnet-4-5"), não os
    //     aliases curtos do catálogo canônico ("opus"/"sonnet"), e mapear
    //     para um provider fixo contradiria a motivação de negócio do REQ
    //     (permitir que o usuário roteie os agentes trackfw para o modelo
    //     open-source/local que ele já configurou em opencode.json). Omitir
    //     deixa o OpenCode resolver pelo default já configurado pelo usuário.
    //   - "memory:" também não faz sentido no schema do OpenCode e é
    //     descartado junto com "tools:".
    let out = `---\ndescription: ${description}\nmode: subagent\n---\n`
    if (body) out += `${body}\n`
    return out
  }

  if (target === 'cursor') {
    const mappedModel = mapModelCursor(parts.model)
    source = mappedModel ? rewriteFrontmatterModelLine(source, mappedModel) : removeFrontmatterModelLine(source)
  }
  if (!id) return normalize(source)
  const withBody = insertBodyPrefix(source, greeting)
  const withFrontmatter = rewriteFrontmatterFields(withBody, name, description)
  const withSignature = rewriteSignatureLine(withFrontmatter, id.display_name)
  return normalize(withSignature)
}

module.exports = { render, markdownParts, frontmatterName, greetingLine, insertBodyPrefix, rewriteFrontmatterFields, rewriteSignatureLine }
