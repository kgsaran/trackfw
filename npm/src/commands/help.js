'use strict'
const { Command } = require('commander')

const configDocs = {
  adr_dirs: {
    type: 'list of strings',
    default: '["docs/adr"]',
    description: 'Diretórios onde os ADRs são armazenados.',
    example: 'adr_dirs:\n  - docs/adr\n  - docs/adr/zeus',
    impact: 'Todos os diretórios listados são varridos na validação de ADRs.'
  },
  req_dir: {
    type: 'string',
    default: '"docs/req"',
    description: 'Diretório onde as REQs são armazenadas.',
    example: 'req_dir: docs/requisicoes',
    impact: 'Altera onde o trackfw busca e cria arquivos de REQ.'
  },
  roadmap_dir: {
    type: 'string',
    default: '"docs/roadmaps"',
    description: 'Diretório raiz dos roadmaps.',
    example: 'roadmap_dir: docs/roadmaps',
    impact: 'Subtrees backlog/, wip/, blocked/, done/, abandoned/ são relativos a este diretório.'
  },
  roadmap_namespacing: {
    type: 'flat|by_agent',
    default: '"flat"',
    description: 'Estratégia de namespacing dos roadmaps.',
    example: 'roadmap_namespacing: by_agent',
    impact: 'Com by_agent, roadmaps ficam em subpastas por agente (ex: wip/apolo/RM-001.md).'
  },
  agents: {
    type: 'list of strings',
    default: '[]',
    description: 'Lista de agentes ativos no projeto.',
    example: 'agents:\n  - apolo\n  - afrodite',
    impact: 'Agentes listados têm suas subpastas criadas automaticamente em roadmap_dir.'
  },
  governance_mode: {
    type: 'string',
    default: '""',
    description: 'Modo de governança (strict, lenient).',
    example: 'governance_mode: strict',
    impact: 'strict bloqueia commits/merges com violations; lenient emite apenas avisos.'
  },
  lenient_until: {
    type: 'date (YYYY-MM-DD)',
    default: '""',
    description: 'Data até quando o modo lenient está ativo.',
    example: 'lenient_until: 2026-12-31',
    impact: 'Após a data, o modo volta automaticamente para strict.'
  },
  wip_limit: {
    type: 'integer',
    default: '1',
    description: 'Limite de itens WIP simultâneos.',
    example: 'wip_limit: 3',
    impact: 'Aumentar reduz a frequência de bloqueio.'
  },
  wip_by_squad: {
    type: 'boolean',
    default: 'false',
    description: 'Aplicar limite WIP por squad individualmente.',
    example: 'wip_by_squad: true',
    impact: 'Cada squad tem seu próprio contador de WIP em vez de um limite global.'
  },
  require_req_in_commit: {
    type: 'boolean',
    default: 'false',
    description: 'Exigir referência de REQ em mensagens de commit.',
    example: 'require_req_in_commit: true',
    impact: 'Commits sem menção a uma REQ são rejeitados pelo hook de pre-commit.'
  },
  'link_fields.req': {
    type: 'list of strings',
    default: '["REQ:"]',
    description: 'Marcadores que identificam link a REQ.',
    example: 'link_fields:\n  req:\n    - "REQ:"\n    - "Requisito:"',
    impact: 'Qualquer marcador listado é aceito para detectar vínculo com REQ.'
  },
  'link_fields.adr': {
    type: 'list of strings',
    default: '["ADR:"]',
    description: 'Marcadores que identificam link a ADR.',
    example: 'link_fields:\n  adr:\n    - "ADR:"\n    - "Decision:"',
    impact: 'Qualquer marcador listado é aceito para detectar vínculo com ADR.'
  },
  'link_fields.roadmap': {
    type: 'list of strings',
    default: '["Roadmap:"]',
    description: 'Marcadores que identificam link a Roadmap.',
    example: 'link_fields:\n  roadmap:\n    - "Roadmap:"',
    impact: 'Qualquer marcador listado é aceito para detectar vínculo com Roadmap.'
  },
  acceptance_markers: {
    type: 'list of strings',
    default: '["## Acceptance Criteria", "## Critérios de Aceite"]',
    description: 'Marcadores de critério de aceite.',
    example: 'acceptance_markers:\n  - "## Acceptance Criteria"\n  - "## AC"',
    impact: 'Roadmaps WIP sem nenhum desses marcadores disparam a regra wip_acceptance.'
  },
  'rules.wip_has_req': {
    type: 'off|warning|error',
    default: '"error"',
    description: 'Severidade: WIP sem REQ linkada.',
    example: 'rules:\n  wip_has_req: warning',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.wip_acceptance': {
    type: 'off|warning|error',
    default: '"error"',
    description: 'Severidade: WIP sem critérios de aceite.',
    example: 'rules:\n  wip_acceptance: warning',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.wip_limit': {
    type: 'off|warning|error',
    default: '"error"',
    description: 'Severidade: excesso de itens WIP.',
    example: 'rules:\n  wip_limit: warning',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.stale_wip': {
    type: 'off|warning|error',
    default: '"warning"',
    description: 'Severidade: WIP sem atualização recente.',
    example: 'rules:\n  stale_wip: error',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.adr_orphan': {
    type: 'off|warning|error',
    default: '"warning"',
    description: 'Severidade: ADR sem REQ vinculada.',
    example: 'rules:\n  adr_orphan: error',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.ref_targets_exist': {
    type: 'off|warning|error',
    default: '"warning"',
    description: 'Severidade: referências com destino inexistente.',
    example: 'rules:\n  ref_targets_exist: error',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.folder_status': {
    type: 'off|warning|error',
    default: '"warning"',
    description: 'Severidade: coerência entre pasta e status do arquivo.',
    example: 'rules:\n  folder_status: error',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.filename_uniqueness': {
    type: 'off|warning|error',
    default: '"error"',
    description: 'Severidade: nomes de arquivo duplicados.',
    example: 'rules:\n  filename_uniqueness: warning',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  },
  'rules.blocked_by_draft_adr': {
    type: 'off|warning|error',
    default: '"error"',
    description: 'Severidade: REQ bloqueada por ADR em rascunho.',
    example: 'rules:\n  blocked_by_draft_adr: warning',
    impact: 'error bloqueia; warning apenas reporta; off desativa a regra.'
  }
}

/**
 * Retorna string com a listagem tabular de todas as keys configuráveis.
 * @returns {string}
 */
function listKeys() {
  const COL_KEY = 28
  const COL_DEFAULT = 36
  const header = 'KEY'.padEnd(COL_KEY) + 'DEFAULT'.padEnd(COL_DEFAULT) + 'DESCRIÇÃO'
  const sep = '─'.repeat(COL_KEY + COL_DEFAULT + 40)
  const lines = [header, sep]
  for (const [key, doc] of Object.entries(configDocs)) {
    lines.push(
      key.padEnd(COL_KEY) +
      doc.default.padEnd(COL_DEFAULT) +
      doc.description
    )
  }
  return lines.join('\n')
}

/**
 * Retorna string com a documentação detalhada de uma key.
 * Retorna null se a key não existir.
 * @param {string} key
 * @returns {string|null}
 */
function describeKey(key) {
  const doc = configDocs[key]
  if (!doc) return null
  return [
    key,
    `  Type:    ${doc.type}`,
    `  Default: ${doc.default}`,
    `  Desc:    ${doc.description}`,
    `  Example:`,
    ...doc.example.split('\n').map(l => `    ${l}`),
    `  Impact:  ${doc.impact}`
  ].join('\n')
}

const cmd = new Command('help')
cmd.description('Exibe documentação das keys configuráveis do trackfw.yaml')
cmd.argument('[key]', 'key específica para detalhar')
cmd.action((key) => {
  if (!key) {
    console.log(listKeys())
    return
  }
  const output = describeKey(key)
  if (!output) {
    console.error(`chave desconhecida: ${key}`)
    process.exit(1)
  }
  console.log(output)
})

module.exports = cmd
module.exports.listKeys = listKeys
module.exports.describeKey = describeKey
