'use strict'

const fs = require('fs')
const os = require('os')
const path = require('path')
const crypto = require('crypto')
const { gitOutput } = require('./git-exec')
const config = require('../config')
const { checkTraceIds } = require('./traceid')
const { loadProvenance } = require('../thirdparty/provenance')
const { homedir } = require('../homedir')
const { normalizeRefSeparator } = require('../lib/pathfmt')

// _platform is seeded from process.platform at module load time. Tests override
// it via _setPlatformForTest to exercise the Windows guard on any host.
//
// Why the mode checks need it: on Windows the POSIX execute bit is not
// representable on NTFS. fs.statSync().mode & 0o111 is 0 for every regular file,
// even immediately after fs.chmodSync(path, 0o755). A check written as "the
// script is not executable" is therefore ALWAYS true there, and no action the
// user takes can make it false.
//
// Mirrors internal/validator/goos.go (Go, canonical) and
// pypi/trackfw/validator.py (Python).
let _platform = process.platform

function _setPlatformForTest(plat) {
  const prev = _platform
  _platform = plat
  return () => { _platform = prev }
}

const STALE_WIP_DAYS = 7
let staleWipNowMs = () => Date.now()

// listDir retorna array de nomes de arquivo (não-diretórios) em dir.
// Retorna [] se o diretório não existir.
function listDir(dir) {
  const expanded = config.expandPath ? config.expandPath(dir) : dir
  try {
    return fs.readdirSync(expanded).filter(name => {
      try {
        return !fs.statSync(path.join(expanded, name)).isDirectory()
      } catch (_) {
        return false
      }
    })
  } catch (_) {
    return []
  }
}

// tryListDir tenta listar o diretório distinguindo "não existe" de outros erros.
// Retorna { entries: string[], readError: Error|null }.
// readError é null tanto no caso de sucesso quanto quando o diretório não existe (ENOENT) —
// diretório ausente é esperado para estados que o projeto não usa.
// readError não-null indica que o diretório EXISTE mas não pôde ser lido (ENOTDIR, EPERM…).
function tryListDir(dir) {
  const expanded = config.expandPath ? config.expandPath(dir) : dir
  try {
    const entries = fs.readdirSync(expanded).filter(name => {
      try { return !fs.statSync(path.join(expanded, name)).isDirectory() } catch (_) { return false }
    })
    return { entries, readError: null }
  } catch (err) {
    if (err && err.code === 'ENOENT') return { entries: [], readError: null }
    return { entries: [], readError: err }
  }
}

function inspectionDiagnostic(rule, target, err) {
  const cause = err && err.message ? err.message : String(err)
  return `${rule}: could not inspect "${target}": ${cause}`
}

function listDirForRule(rule, dir, messages) {
  const { entries, readError } = tryListDir(dir)
  if (readError) messages.push(inspectionDiagnostic(rule, dir, readError))
  return entries
}

function readFileForRule(rule, filePath, messages) {
  try {
    return fs.readFileSync(filePath, 'utf8')
  } catch (err) {
    messages.push(inspectionDiagnostic(rule, filePath, err))
    return null
  }
}

// isInsideDir retorna true se childPath estiver contido ou for igual a parentDir.
function isInsideDir(parentDir, childPath) {
  if (!parentDir || !childPath) return false
  const rel = path.relative(path.resolve(parentDir), path.resolve(childPath))
  return rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel))
}

// walkDirMdWithPaths retorna { name, fullPath } de todos .md recursivamente dentro de dir.
function walkDirMdWithPaths(dir) {
  return walkDirMdWithPathsForRule(null, dir, null)
}

function walkDirMdWithPathsForRule(rule, dir, messages) {
  const results = []
  const expandedDir = config.expandPath ? config.expandPath(dir) : dir
  function walk(d) {
    let entries
    try {
      entries = fs.readdirSync(d)
    } catch (err) {
      if (messages && err && err.code !== 'ENOENT') messages.push(inspectionDiagnostic(rule, d, err))
      return
    }
    for (const name of entries) {
      const full = path.join(d, name)
      try {
        if (fs.statSync(full).isDirectory()) { walk(full) }
        else if (name.endsWith('.md')) { results.push({ name, fullPath: full }) }
      } catch (err) {
        if (messages) messages.push(inspectionDiagnostic(rule, full, err))
      }
    }
  }
  walk(expandedDir)
  return results
}

// walkDirMd retorna basenames de todos .md recursivamente dentro de dir.
function walkDirMd(dir) {
  return walkDirMdWithPaths(dir).map(item => item.name)
}

// findAdrFile busca o basename recursivamente em todos os adrDirs configurados.
// Retorna o caminho completo se encontrado, ou null.
function findAdrFile(basename) {
  const cfg = config.load()
  const adrDirs = (cfg.adrDirs || []).map(d => config.expandPath ? config.expandPath(d) : d)
  for (const adrDir of adrDirs) {
    function search(d) {
      let entries
      try { entries = fs.readdirSync(d) } catch (_) { return null }
      for (const name of entries) {
        const full = path.join(d, name)
        try {
          if (fs.statSync(full).isDirectory()) {
            const r = search(full)
            if (r) return r
          } else if (name === basename) {
            return full
          }
        } catch (_) {}
      }
      return null
    }
    const found = search(adrDir)
    if (found) return found
  }
  return null
}

// gitLastModifiedTime retorna o timestamp (ms) do último commit que tocou o arquivo via git log.
// Retorna null em caso de erro ou se não houver commits.
function gitLastModifiedTime(filePath) {
  try {
    const out = gitOutput('.', ['log', '-1', '--format=%ct', '--', filePath]).trim()
    if (out) return parseInt(out, 10) * 1000  // converter para ms
  } catch (_) {}
  return null
}

// resolveAgentNamespaces é o resolvedor canônico de namespaces em modo by_agent — o ÚNICO lugar do
// módulo onde a lista `agents:` do trackfw.yaml é lida ao lado do disco. Devolve a UNIÃO entre
// cfg.agents (na ordem declarada, deduplicada) e os subdiretórios de primeiro nível encontrados em
// dir (ordenados). Todo outro ponto do módulo que precisar enumerar agentes DEVE chamar esta função
// — nunca reimplementar "if (!agents.length) { ler disco }": esse padrão SUBSTITUÍA o disco em vez
// de complementá-lo, deixando invisível qualquer namespace em disco não declarado em agents:
// (REQ-2026-08-29). O padrão `!agents.length`/`agents.length === 0` só pode existir aqui dentro.
//
// Segurança — NÃO segue symlink (AC12/AC13, bloqueante): usa fs.readdirSync(dir, {withFileTypes:
// true}) + dirent.isDirectory(), que reflete o tipo da própria entrada do diretório, não o alvo do
// link — um namespace que é symlink para fora do projeto nunca é tratado como diretório aqui. NÃO
// trocar por fs.statSync (que SEGUE symlink e reproduziu ao vivo escrita fora do projeto via
// `roadmap move` — ver ADR-2026-08-29, decisão 5, e
// vault/notes/update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md).
function resolveAgentNamespaces(cfg, dir) {
  const declared = cfg.agents || []
  const seen = new Set()
  const ordered = []
  for (const a of declared) {
    if (!a || seen.has(a)) continue
    seen.add(a)
    ordered.push(a)
  }

  let entries = []
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true })
  } catch (_) {
    return ordered
  }
  const fromDisk = entries
    .filter(e => e.isDirectory()) // symlinks retornam false aqui — nunca seguidos (AC12/AC13)
    .map(e => e.name)
    .filter(name => !isInfraDirName(name)) // ML-2A: nunca vira namespace, ver comentário na função
    .sort()
  for (const name of fromDisk) {
    if (seen.has(name)) continue
    seen.add(name)
    ordered.push(name)
  }
  return ordered
}

// isInfraDirName decide, no ponto único de leitura de disco do resolvedor, se uma entrada é
// COMPROVADAMENTE infraestrutura e nunca um namespace de agente — filtrada da união (decisão 1 do
// ADR) e portanto invisível a todo consumidor.
//
// CORREÇÃO (ML-4A, achado 1 do parecer hades-tf 2026-08-30, REPROVA original — espelha
// internal/validator/validator.go, ver o comentário lá para a justificativa completa): esta lista já
// incluiu "qualquer nome iniciando com '.'", reabrindo a invisibilidade que a REQ existe para fechar
// (um namespace real ".ghost" desaparecia de union, status, wip limit e `move` sem sinal algum — um
// canal de ocultação deliberada). A lista fechada agora tem UMA entrada:
//   - "node_modules": artefato de tooling JS. Nenhum operador digita isto como nome de agente por
//     acidente ou por design — ruído inequívoco, sem a ambiguidade de um nome iniciado por ponto.
// Nomes iniciados por "." NÃO são mais filtrados aqui — continuam entrando na união (nunca
// invisíveis) — mas o sinal de "não declarado" é rebaixado de violação para aviso de baixo ruído
// (ver isDotPrefixedName / hiddenNamespaceWarnings), nunca zero sinal.
function isInfraDirName(name) {
  return name === 'node_modules'
}

// isDotPrefixedName reporta se name começa com "." — sinal ambíguo (pode ser um namespace legítimo,
// ou tooling que nunca deveria estar dentro de roadmapDir/reqDir), não mais um filtro de
// invisibilidade (ver isInfraDirName). Só rebaixa "não declarado em agents:" de violação para aviso.
function isDotPrefixedName(name) {
  return name.startsWith('.')
}

// AGENT_NAMESPACE_STATE_NAMES replica, só para esta regra, os 6 nomes de estado reservados de
// roadmap/REQ. Um diretório com um desses nomes no topo de roadmap_dir/req_dir é, na prática, resto
// de migração incompleta flat→by_agent (ex.: "wip" órfão) — não um agente. A união (decisão 1 do
// ADR) continua enumerando esses diretórios normalmente — nada fica invisível —, mas eles NÃO
// disparam validateAgentNamespaceUndeclared: pedir para declarar "wip" como agente em agents: seria
// ruído confuso, não uma correção real (ML-0A, seção 3, item 3; recomendação adotada sem alteração).
// Esta exclusão vive só aqui, não no resolvedor — a colisão de nome não é "comprovadamente
// infraestrutura" como isInfraDirName, é uma inferência sobre o significado do nome, então só afeta
// a violação, nunca a união/enumeração.
const AGENT_NAMESPACE_STATE_NAMES = new Set(['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned'])

// undeclaredNamespacesOnDisk devolve, a partir do resolvedor canônico (que já filtra infra e não
// segue symlink), os nomes de namespace presentes em dir e ausentes de agents:, excluindo colisões
// com nome de estado reservado (AGENT_NAMESPACE_STATE_NAMES).
function undeclaredNamespacesOnDisk(cfg, dir, declared) {
  const out = []
  for (const name of resolveAgentNamespaces(cfg, dir)) {
    if (declared.has(name) || AGENT_NAMESPACE_STATE_NAMES.has(name) || isDotPrefixedName(name)) continue
    out.push(name)
  }
  return out
}

// dotPrefixedUndeclaredOnDisk é o espelho de undeclaredNamespacesOnDisk para o caso ambíguo (nome
// iniciado por "."): mesmo resolvedor canônico, mesma exclusão de nomes já declarados, mas mantendo
// exatamente os nomes que undeclaredNamespacesOnDisk descarta por causa do ponto.
function dotPrefixedUndeclaredOnDisk(cfg, dir, declared) {
  const out = []
  for (const name of resolveAgentNamespaces(cfg, dir)) {
    if (declared.has(name) || !isDotPrefixedName(name)) continue
    out.push(name)
  }
  return out
}

// validateAgentNamespaceUndeclared implementa a regra "agent_namespace_undeclared"
// (ADR-2026-08-29, decisão 2 / REQ AC4, AC5, AC9): em modo by_agent, um namespace presente em disco
// (roadmapDir e/ou reqDir — AC2 estende a união às duas árvores, e esta violação segue) e ausente de
// agents: é VIOLAÇÃO, não aviso — usa o mesmo default 'error' de toda regra sem entrada em
// RULE_DEFAULTS (diskRuleSeverity).
//
// A união já garante (Wave 1) que o namespace continua sendo ENUMERADO por todo consumidor mesmo com
// esta violação ativa — esta função só ADICIONA o sinal de configuração incompleta, nunca CONDICIONA
// a enumeração a ele (AC5-b).
//
// Deduplicação por namespace, não por árvore: o caso motivador (cmdb, "zeus" ausente de agents: e em
// disco em roadmapDir E reqDir ao mesmo tempo) produziria duas violações quase-idênticas se o laço
// fosse por árvore — ruído no caso comum, não no caso raro. Uma violação por nome, nomeando todas as
// árvores onde ele foi encontrado.
function validateAgentNamespaceUndeclared(cfg) {
  if (cfg.roadmapNamespacing !== config.NAMESPACING_BY_AGENT) return []
  const declared = new Set(cfg.agents || [])

  const roadmapNames = undeclaredNamespacesOnDisk(cfg, cfg.roadmapDir, declared)
  const reqNames = undeclaredNamespacesOnDisk(cfg, cfg.reqDir || cfg.req_dir || '', declared)

  const inRoadmap = new Set(roadmapNames)
  const inReq = new Set(reqNames)

  const names = []
  const seen = new Set()
  for (const n of [...roadmapNames, ...reqNames]) {
    if (seen.has(n)) continue
    seen.add(n)
    names.push(n)
  }
  names.sort()

  const msgs = []
  for (const name of names) {
    const trees = []
    if (inRoadmap.has(name)) trees.push('roadmap_dir')
    if (inReq.has(name)) trees.push('req_dir')
    msgs.push(`agent namespace "${name}" exists in ${trees.join(', ')} but is not declared in agents: — add it to trackfw.yaml`)
  }
  return msgs
}

// hiddenNamespaceWarnings implementa a regra "agent_namespace_hidden" — o contraponto de baixo ruído
// de validateAgentNamespaceUndeclared para nomes iniciados por "." (ML-4A, achado 1 do parecer
// hades-tf 2026-08-30). Um diretório oculto/ambíguo em disco (roadmapDir e/ou reqDir), ausente de
// agents:, NÃO é filtrado da união (resolveAgentNamespaces mantém) e NÃO dispara a violação plena —
// mas também não é silêncio total: esta função emite um aviso nomeando explicitamente o diretório.
function hiddenNamespaceWarnings(cfg) {
  if (cfg.roadmapNamespacing !== config.NAMESPACING_BY_AGENT) return []
  const declared = new Set(cfg.agents || [])

  const roadmapNames = dotPrefixedUndeclaredOnDisk(cfg, cfg.roadmapDir, declared)
  const reqNames = dotPrefixedUndeclaredOnDisk(cfg, cfg.reqDir || cfg.req_dir || '', declared)

  const inRoadmap = new Set(roadmapNames)
  const inReq = new Set(reqNames)

  const names = []
  const seen = new Set()
  for (const n of [...roadmapNames, ...reqNames]) {
    if (seen.has(n)) continue
    seen.add(n)
    names.push(n)
  }
  names.sort()

  const msgs = []
  for (const name of names) {
    const trees = []
    if (inRoadmap.has(name)) trees.push('roadmap_dir')
    if (inReq.has(name)) trees.push('req_dir')
    msgs.push(`dot-prefixed directory "${name}" found in ${trees.join(', ')} is treated as an agent namespace (fully enumerated, not declared in agents:) — declare it in trackfw.yaml if intentional, or remove it if it is leftover tooling`)
  }
  return msgs
}

// REQ_LAYOUT_STATES é a lista fechada de nomes de pasta de ESTADO reconhecidos nos layouts LEGADOS
// de REQ. Existe só para o leitor tolerar árvores antigas: pelo invariante D1 da ADR-2026-09-03,
// REQ NÃO tem dimensão de estado. Nada aqui decide onde ESCREVER uma REQ.
const REQ_LAYOUT_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']

// listReqMdFiles lista os .md diretamente dentro de dir (sem recursão, sem seguir para subpastas).
// Não usa glob: nomes de agente vêm do disco e metacaracteres ("*", "[") corromperiam a contagem.
function listReqMdFiles(dir) {
  try {
    return fs.readdirSync(dir)
      .filter(n => n.endsWith('.md'))
      .filter(n => { try { return !fs.statSync(path.join(dir, n)).isDirectory() } catch (_) { return false } })
      .map(n => path.join(dir, n))
  } catch (_) { return [] }
}

// reqWriteDir é o PONTO ÚNICO que decide ONDE uma REQ nova é gravada (ADR-2026-09-03, D2/D4):
//   flat     → req_dir/
//   by_agent → req_dir/<agente>/   (primeiro de agents:, ou "default" se a lista é vazia)
// Consumido pelo gerador (newREQ); a união de leitura abaixo contém este diretório por construção.
function reqWriteDir(cfg) {
  const reqDir = cfg.reqDir || cfg.req_dir || ''
  if (!reqDir) return ''
  const namespacing = cfg.roadmapNamespacing || cfg.roadmap_namespacing || ''
  if (namespacing === 'by_agent') {
    const agents = (cfg.agents || []).filter(a => a)
    const agent = agents.length > 0 ? agents[0] : 'default'
    return path.join(reqDir, agent)
  }
  return reqDir
}

// resolveReqFiles é o PONTO ÚNICO de LEITURA de REQ (ADR-2026-09-03, D3/D4): devolve os paths de
// todos os .md de REQ como UNIÃO dos 4 layouts suportados, nunca como escolha exclusiva:
//   req_dir/*.md                    flat legado
//   req_dir/<estado>/*.md           por-estado legado (apesar de D1)
//   req_dir/<agente>/*.md           CANÔNICO em by_agent
//   req_dir/<agente>/<estado>/*.md  legado
// Os dois últimos só valem em by_agent — fora dele não há noção de namespace de agente.
//
// Deduplicação é OBRIGATÓRIA: resolveAgentNamespaces devolve agents: ∪ disco, então um
// req_dir/backlog/ real entra na lista de agentes e o caso <agente>/*.md emite os mesmos paths do
// caso <estado>/*.md — sem o Set, toda REQ em layout por-estado seria contada (e violada) em dobro.
// Ordenação final: determinismo igual ao Go/Python (readdirSync não é ordenado por contrato).
function resolveReqFiles(cfg) {
  const reqDir = cfg.reqDir || cfg.req_dir || ''
  if (!reqDir) return []
  const namespacing = cfg.roadmapNamespacing || cfg.roadmap_namespacing || ''
  const seen = new Set()
  const files = []
  const add = (paths) => {
    for (const p of paths) {
      const clean = path.normalize(p)
      if (seen.has(clean)) continue
      seen.add(clean)
      files.push(clean)
    }
  }

  // 🔴 §4 (hades-tf 2026-09-03): a dedup por STRING não vê req_dir/Backlog ≡ req_dir/backlog em
  // filesystem case-INSENSITIVE (APFS, NTFS) — "Backlog" entra na lista de agentes pelo disco e
  // emite req_dir/Backlog/*.md, enquanto o laço de estados emite req_dir/backlog/*.md hardcoded em
  // minúscula. Mesmo diretório, strings diferentes: toda REQ contada em DOBRO (medido em APFS:
  // 2 REQs e 4 violações para 1 arquivo real). Verde no CI Linux, vermelho na máquina do dev.
  //
  // MECANISMO: só enumeramos um candidato de subdiretório se o nome existir VERBATIM na listagem do
  // pai — a grafia do disco é a autoridade, medimos o disco em vez de presumir a propriedade do FS.
  //   - NÃO troca dupla contagem por SUPRESSÃO: em FS case-SENSITIVE o readdir lista "Backlog" E
  //     "backlog" (dois diretórios reais distintos) e ambos seguem enumerados. Lowercase colapsaria.
  //   - NÃO usa ino/dev: fs.statSync().ino não tem contrato medido em NTFS, e ino colidente
  //     colapsaria arquivos distintos — supressão, a direção proibida.
  //   - Cobre também NFC/NFD, que case-folding não cobre.
  //   - FALLBACK É JOIN CEGO, NUNCA LISTA VAZIA: pai ilegível → não filtra nada (dupla contagem
  //     benigna). Devolver vazio seria supressão.
  //   - Não filtra por TIPO: um <estado> que seja symlink segue enumerado como antes (§3 é outro
  //     escopo).
  // Paridade: mesma lógica em internal/validator/validator.go e pypi/trackfw/validator.py.
  const childCache = new Map()
  const hasChildVerbatim = (parent, name) => {
    if (!childCache.has(parent)) {
      try { childCache.set(parent, new Set(fs.readdirSync(parent))) } catch (_) { childCache.set(parent, null) }
    }
    const children = childCache.get(parent)
    return children === null ? true : children.has(name)
  }
  const addChild = (parent, name) => {
    if (!hasChildVerbatim(parent, name)) return
    add(listReqMdFiles(path.join(parent, name)))
  }

  add(listReqMdFiles(reqDir))
  for (const state of REQ_LAYOUT_STATES) addChild(reqDir, state)

  if (namespacing === 'by_agent') {
    for (const agent of resolveAgentNamespaces(cfg, reqDir)) {
      addChild(reqDir, agent)
      for (const state of REQ_LAYOUT_STATES) addChild(path.join(reqDir, agent), state)
    }
  }

  files.sort()
  return files
}

// resolveStateDirs retorna todos os diretórios de um estado (ex: 'wip', 'done') conforme o modo de
// namespacing. É a fonte única de resolução de caminho por estado — resolveWIPDirs e resolveDoneDirs
// são wrappers finos sobre esta função. Duplicar a lógica aqui foi a causa raiz de defeitos
// anteriores (roadmap_dir divergente entre runtimes).
function resolveStateDirs(cfg, state) {
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    return agents.map(agent => cfg.roadmapDir + '/' + agent + '/' + state)
  }
  return [cfg.roadmapDir + '/' + state]
}

// resolveWIPDirs retorna todos os diretórios wip/ conforme o modo de namespacing.
function resolveWIPDirs(cfg) {
  return resolveStateDirs(cfg, 'wip')
}

// resolveDoneDirs retorna todos os diretórios done/ conforme o modo de namespacing.
function resolveDoneDirs(cfg) {
  return resolveStateDirs(cfg, 'done')
}

// parseBlockedADRs extrai basenames de ADRs da seção "## Blocked by ADRs" de um arquivo REQ.
function parseBlockedADRs(filePath) {
  let content
  try {
    content = fs.readFileSync(filePath, 'utf8')
  } catch (_) {
    return []
  }
  const lines = content.split('\n')
  const adrs = []
  let inSection = false
  for (const line of lines) {
    if (line === '## Blocked by ADRs') {
      inSection = true
      continue
    }
    if (inSection) {
      if (line.startsWith('## ')) break
      if (line.startsWith('- ')) {
        const item = line.slice(2).trim()
        const parts = item.split(/\s+/)
        if (parts.length > 0 && parts[0].endsWith('.md')) {
          adrs.push(parts[0])
        }
      }
    }
  }
  return adrs
}

// contentHasMarker retorna true se o conteúdo contém algum dos markers sem espaço em branco após.
// P3: verifica tanto "\n" quanto "\r\n" para detectar campos vazios em arquivos CRLF.
//
// Usada apenas para markers de "existência de bloco" (ex: acceptanceMarkers, um heading de
// seção) — não para markers de link (REQ:/ADR:/Roadmap:), que usam contentHasMarkerValue.
function contentHasMarker(content, markers) {
  for (const marker of markers) {
    if (content.includes(marker) && !content.includes(marker + ' \n') && !content.includes(marker + ' \r\n')) {
      return true
    }
  }
  return false
}

// contentHasMarkerValue retorna true se algum dos markers aparece em content seguido de
// conteúdo não-branco na mesma linha — isto é, o campo tem um valor real, não apenas o marker.
//
// issue #278: contentHasMarker (acima) detectava "vazio" só pela grafia literal
// "MARKER + um espaço + \n"/"\r\n" — 5 de 7 grafias naturais de campo vazio escapavam
// ("MARKER:\n" sem espaço, dois espaços, tab, CRLF sem espaço, três espaços). Esta função
// decide por VALOR: pega o resto da linha após o marker, descarta \r/\t/espaços das duas
// pontas, e só considera "tem valor" se sobrar algo. Indiferente a CRLF, tabs e contagem de
// espaços — cobre as 7 grafias medidas na triagem, não apenas a literal do template.
function contentHasMarkerValue(content, markers) {
  for (const marker of markers) {
    let start = 0
    while (true) {
      const idx = content.indexOf(marker, start)
      if (idx === -1) break
      const pos = idx + marker.length
      let rest = content.slice(pos)
      const nl = rest.indexOf('\n')
      if (nl !== -1) rest = rest.slice(0, nl)
      if (rest.endsWith('\r')) rest = rest.slice(0, -1)
      if (rest.trim() !== '') return true
      start = pos
    }
  }
  return false
}

// extractAdrHeaderStatus extrai o valor declarado na linha de cabeçalho
// ("> Date: ... | Status: X") de um ADR. Ancorado à linha — não é substring livre sobre o
// documento inteiro — para não confundir texto de prosa que mencione "Status: Draft"/"Proposed"
// (ex: citações de código, exemplos) com o status real do artefato. Retorna '' se a linha não
// existir.
function extractAdrHeaderStatus(content) {
  const marker = '| Status: '
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(marker)
    if (idx >= 0) {
      let rest = trimmed.slice(idx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim()
    }
  }
  return ''
}

// extractFrontmatterField extrai o valor de um campo do bloco frontmatter YAML
// (delimitado por "---" ... "---" no início do arquivo). Retorna '' se o frontmatter
// estiver ausente ou o campo não existir/estiver vazio.
function extractFrontmatterField(content, field) {
  if (!content.startsWith('---')) return ''
  const rest = content.slice(3)
  const end = rest.indexOf('\n---')
  if (end < 0) return ''
  const block = rest.slice(0, end)
  for (const line of block.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith(field + ':')) {
      let val = trimmed.slice(field.length + 1).trim()
      val = val.replace(/^["']|["']$/g, '')
      return val
    }
  }
  return ''
}

// resolveAdrStatus extrai o valor bruto do status de um ADR: frontmatter `status:`
// primeiro — é o campo machine-readable canônico, o mesmo que os geradores (`adr new`,
// NewADRDraft) escrevem e que a regra folder_status já usa como fonte de verdade — com
// fallback para a linha de cabeçalho ("> Date: ... | Status: X") somente se o
// frontmatter estiver ausente ou sem o campo (cobre ADRs legados sem frontmatter). Em um
// ADR bem formado os dois concordam. ML-1D (2026-08-01): alinha o Node ao Go e ao
// Python, que já liam frontmatter-first (ADR-2026-08-01-nocao-canonica-de-adr-nao-aceito).
function resolveAdrStatus(content) {
  const fm = extractFrontmatterField(content, 'status')
  if (fm) return fm
  return extractAdrHeaderStatus(content)
}

// adrNotAcceptedStatusForRule é o helper canônico de "ADR não aceito": único lugar que conhece o
// vocabulário Draft/Proposed. Verdadeiro para ADR cujo status seja "Draft" ou "Proposed"; qualquer
// outro status (Accepted, Superseded, Deprecated, Rejected, ...) conta como aceito por exclusão —
// não há allowlist fechada de status aceitos.
//
// A fonte do status é resolveAdrStatus (frontmatter-first, fallback de cabeçalho — ver acima).
// Crucial: o fallback de cabeçalho extrai o valor de UMA linha específica
// (extractAdrHeaderStatus), não faz content.includes('Status: Draft') sobre o documento
// inteiro — esse era o defeito do código herdado (adrDraftStatusForRule original): um ADR
// com status real "Accepted" mas cuja prosa cita literalmente a string "Status: Draft"
// (ex: este próprio ADR, que documenta o bug) seria classificado como não aceito. Ver
// vault/notes para o caso concreto que expôs isso.
function adrNotAcceptedStatusForRule(rule, basename, messages) {
  const p = findAdrFile(basename)
  if (!p) return { notAccepted: false, status: '', inspected: true }
  try {
    const content = fs.readFileSync(p, 'utf8')
    const status = resolveAdrStatus(content)
    const notAccepted = status.toLowerCase() === 'draft' || status.toLowerCase() === 'proposed'
    return { notAccepted, status, inspected: true }
  } catch (err) {
    if (messages) messages.push(inspectionDiagnostic(rule, p, err))
    return { notAccepted: false, status: '', inspected: false }
  }
}

// adrIsDraft verifica se <adrBasename> está em status não aceito (Draft ou Proposed) buscando
// recursivamente nas adrDirs. Nome mantido por compatibilidade (chamadores existentes); delega no
// helper canônico adrNotAcceptedStatusForRule.
function adrIsDraft(basename) {
  return adrDraftStatusForRule(basename, null).draft
}

function adrDraftStatusForRule(basename, messages) {
  const result = adrNotAcceptedStatusForRule('blocked_by_draft_adr', basename, messages)
  return { draft: result.notAccepted, inspected: result.inspected }
}

// validateWIPHasREQ — roadmaps em wip/ sem marker REQ no conteúdo → violation
// Suporta modo by_agent via resolveWIPDirs.
function validateWIPHasREQ() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const violations = []
  for (const wipDir of wipDirs) {
    const entries = listDirForRule('wip_has_req', wipDir, violations)
    for (const name of entries) {
      const content = readFileForRule('wip_has_req', path.join(wipDir, name), violations)
      if (content === null) continue
      if (!contentHasMarkerValue(content, cfg.linkFields.req)) {
        violations.push(`roadmap "${name}" is in wip but has no linked REQ`)
      }
    }
  }
  return violations
}

// validateREQsHaveADR — REQs em <reqDir>/ sem marker ADR no conteúdo → violation
function validateREQsHaveADR() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!contentHasMarkerValue(content, cfg.linkFields.adr)) {
        violations.push(`req "${path.basename(filePath)}" has no linked ADR`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateBlockedHasREQ — roadmaps em <roadmapDir>/blocked/ sem marker REQ → violation
function validateBlockedHasREQ() {
  const cfg = config.load()
  const violations = []
  for (const blockedDir of resolveStateDirs(cfg, 'blocked')) {
    const entries = listDirForRule('blocked_has_req', blockedDir, violations)
    for (const name of entries) {
      const content = readFileForRule('blocked_has_req', path.join(blockedDir, name), violations)
      if (content === null) continue
      if (!contentHasMarkerValue(content, cfg.linkFields.req)) {
        violations.push(`roadmap "${name}" is in blocked but has no linked REQ`)
      }
    }
  }
  return violations
}

// validateREQsHaveRoadmap — REQs sem marker Roadmap → violation
function validateREQsHaveRoadmap() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!contentHasMarkerValue(content, cfg.linkFields.roadmap)) {
        violations.push(`req "${path.basename(filePath)}" has no linked Roadmap`)
      }
    } catch (_) {
      // ignorar
    }
  }
  return violations
}

// validateADRDirsExist — verifica se todos adrDirs existem.
// Retorna { violations: [], warnings: [] } respeitando strictCiPaths.
function validateADRDirsExist() {
  const cfg = config.load()
  const violations = []
  const warnings = []
  for (const adrDir of cfg.adrDirs || []) {
    const expanded = config.expandPath ? config.expandPath(adrDir) : adrDir
    const absDir = path.resolve(expanded)
    if (!fs.existsSync(absDir)) {
      const msg = `adr_dir "${adrDir}" does not exist`
      if (cfg.strictCiPaths) {
        violations.push(msg)
      } else {
        warnings.push(msg)
      }
    }
  }
  return { violations, warnings }
}

// validateADRsAreReferenced — ADRs em adrDirs não referenciados em nenhuma REQ → violation.
// ADRs localizados fora do projeto local (cwd) são isentos desta validação.
function validateADRsAreReferenced() {
  const cfg = config.load()
  const cwd = process.cwd()
  const violations = []
  let adrs = []
  for (const adrDir of cfg.adrDirs || []) {
    const expanded = config.expandPath ? config.expandPath(adrDir) : adrDir
    const absDir = path.resolve(expanded)
    // Isentar diretórios fora do cwd local
    if (!isInsideDir(cwd, absDir)) {
      continue
    }
    for (const item of walkDirMdWithPathsForRule('adr_orphan', absDir, violations)) {
      if (isInsideDir(cwd, item.fullPath)) {
        adrs.push(item.name)
      }
    }
  }

  const reqFiles = resolveReqFiles(cfg)
  let combined = ''
  for (const filePath of reqFiles) {
    const content = readFileForRule('adr_orphan', filePath, violations)
    if (content !== null) combined += content
  }

  for (const adr of adrs) {
    if (!combined.includes(adr)) {
      violations.push(`adr "${adr}" is not referenced by any REQ`)
    }
  }
  return violations
}

// validateWIPHasAcceptanceCriteria — roadmaps wip sem bloco de critérios de aceite → violation
// Suporta modo by_agent via resolveWIPDirs.
function validateWIPHasAcceptanceCriteria() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const violations = []
  for (const wipDir of wipDirs) {
    const entries = listDirForRule('wip_acceptance', wipDir, violations)
    for (const name of entries) {
      const content = readFileForRule('wip_acceptance', path.join(wipDir, name), violations)
      if (content === null) continue
      const hasBlock = contentHasMarker(content, cfg.acceptanceMarkers)
      if (!hasBlock) {
        violations.push(`roadmap "${name}" is in wip but has no acceptance criteria block`)
      }
    }
  }
  return violations
}

// wipConfigFrom deriva { limit, bySquad } a partir do ProjectConfig já normalizado por
// config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
function wipConfigFrom(cfg) {
  return { limit: cfg.wipLimit > 0 ? cfg.wipLimit : 1, bySquad: !!cfg.wipBySquad }
}

// parseSquadFromFrontmatter extrai o valor do campo "squad:" de um arquivo markdown.
// Retorna string vazia se ausente ou vazio.
function parseSquadFromFrontmatter(filePath) {
  let content
  try {
    content = fs.readFileSync(filePath, 'utf8')
  } catch (_) {
    return ''
  }
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    if (trimmed.startsWith('squad:')) {
      return trimmed.slice('squad:'.length).trim()
    }
  }
  return ''
}

// validateWIPLimit — verifica o WIP limit por agente, por squad ou global conforme trackfw.yaml.
// Retorna { violations: [], warnings: [] }.
function validateWIPLimit() {
  const cfg = config.load()
  const violations = []
  const warnings = []

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    const limit = cfg.wipLimit > 0 ? cfg.wipLimit : 1
    for (const agent of agents) {
      const entries = listDir(cfg.roadmapDir + '/' + agent + '/wip')
      if (entries.length > limit) {
        warnings.push(`${entries.length} roadmaps in wip/ for agent "${agent}" (limit: ${limit}) — consider focusing`)
      }
    }
    return { violations, warnings }
  }

  // modo flat (global ou por squad)
  let files = []
  try {
    files = fs.readdirSync(path.join(cfg.roadmapDir, 'wip'))
      .filter(f => { try { return !fs.statSync(path.join(cfg.roadmapDir, 'wip', f)).isDirectory() } catch (_) { return false } })
      .map(f => path.join(cfg.roadmapDir, 'wip', f))
  } catch (_) {
    return { violations, warnings }
  }

  const wipCfg = wipConfigFrom(cfg)

  if (!wipCfg.bySquad) {
    if (files.length > wipCfg.limit) {
      warnings.push(`${files.length} roadmaps in wip/ (limit: ${wipCfg.limit}) — consider focusing`)
    }
    return { violations, warnings }
  }

  const bySquad = {}
  for (const f of files) {
    let squad = parseSquadFromFrontmatter(f)
    if (!squad) squad = '(no squad)'
    if (!bySquad[squad]) bySquad[squad] = []
    bySquad[squad].push(path.basename(f))
  }
  for (const [squad, items] of Object.entries(bySquad)) {
    if (items.length > wipCfg.limit) {
      warnings.push(`squad "${squad}" has ${items.length} roadmaps in wip/ (limit: ${wipCfg.limit})`)
    }
  }
  return { violations, warnings }
}

// validateSingleWIP — alias retrocompatível de validateWIPLimit (modo flat)
function validateSingleWIP() {
  return validateWIPLimit()
}

// validateStaleWIP — roadmaps wip com mtime >= 7 dias → warning
// Suporta modo by_agent via resolveWIPDirs.
function roadmapLogIdentity(cfg, filePath) {
  const basename = path.basename(filePath)
  if (cfg.roadmapNamespacing !== config.NAMESPACING_BY_AGENT) return basename
  const agent = path.basename(path.dirname(path.dirname(filePath)))
  return `${agent}/${basename}`
}

function parseTransitionLogLine(line) {
  const fields = String(line).trim().split(/\s+/).filter(Boolean)
  if (fields.length < 5) return null
  const timestamp = Date.parse(`${fields[0]}T${fields[1]}:00`)
  if (Number.isNaN(timestamp)) return null
  const arrow = fields.findIndex((field, index) => index >= 3 && (field === '→' || field === '->'))
  if (arrow < 0 || arrow + 1 >= fields.length) return null
  return { timestamp, name: fields[2], toState: fields[arrow + 1] }
}

function latestWipTransitionTime(cfg, filePath) {
  let content
  const logPath = path.join(cfg.roadmapDir, '.trackfw-log')
  try {
    content = fs.readFileSync(logPath, 'utf8')
  } catch (err) {
    return { time: null, diagnostics: err && err.code === 'ENOENT' ? [] : [inspectionDiagnostic('stale_wip', logPath, err)] }
  }
  const identity = roadmapLogIdentity(cfg, filePath)
  let latest = null
  const diagnostics = []
  for (const line of content.split('\n')) {
    if (!line.trim()) continue
    const parsed = parseTransitionLogLine(line)
    if (!parsed) {
      diagnostics.push(`stale_wip: invalid support line in "${logPath}": "${line}"`)
      continue
    }
    if (parsed.name !== identity || parsed.toState !== 'wip') continue
    if (latest === null || parsed.timestamp > latest) latest = parsed.timestamp
  }
  return { time: latest, diagnostics }
}

function validateStaleWIP() {
  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const warnings = []
  const now = staleWipNowMs()
  const thresholdDays = cfg.staleWipDays > 0 ? cfg.staleWipDays : STALE_WIP_DAYS

  for (const wipDir of wipDirs) {
    const files = listDirForRule('stale_wip', wipDir, warnings)
      .filter(f => f.endsWith('.md'))
      .map(f => path.join(wipDir, f))

    for (const filePath of files) {
      try {
        const stat = fs.statSync(filePath)
        const logResult = latestWipTransitionTime(cfg, filePath)
        warnings.push(...logResult.diagnostics)
        const refTime = logResult.time !== null ? logResult.time : stat.mtimeMs
        const ageMs = now - refTime
        const days = Math.floor(ageMs / (1000 * 60 * 60 * 24))
        if (days >= thresholdDays) {
          const lastModified = new Date(refTime).toISOString().slice(0, 10)
          const basename = path.basename(filePath)
          warnings.push(
            `roadmap/wip/${basename} has been in WIP for ${days} days (last modified ${lastModified})`
          )
        }
      } catch (err) {
        warnings.push(inspectionDiagnostic('stale_wip', filePath, err))
      }
    }
  }
  return warnings
}

function setStaleWipNowForTests(fn) {
  staleWipNowMs = fn || (() => Date.now())
}

// validateREQsNotBlockedByDraftADRs — REQs Open com ADRs Draft na seção "## Blocked by ADRs" → violation
function validateREQsNotBlockedByDraftADRs() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    const content = readFileForRule('blocked_by_draft_adr', filePath, violations)
    if (content === null) continue
    if (!content.includes('Status: Open')) continue

    const blockedADRs = parseBlockedADRs(filePath)
    for (const adrBasename of blockedADRs) {
      if (adrDraftStatusForRule(adrBasename, violations).draft) {
        violations.push(`REQ ${path.basename(filePath)} is blocked by not-accepted ADR: ${adrBasename}`)
      }
    }
  }
  return violations
}

// reqStatusEquals compara o status de uma REQ (case-insensitive) contra o valor esperado.
// Casa tanto o frontmatter ("status: X") quanto a linha de cabeçalho ("> Date: ... | Status: X"),
// mesma lógica de detecção usada por reqStatusIsOpen — duplicada aqui (em vez de generalizar
// reqStatusIsOpen) para não alterar o comportamento de req_roadmap_lifecycle, que já a consome.
function reqStatusEquals(content, status) {
  const target = String(status).toLowerCase()
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(':')
    if (idx >= 0 && trimmed.slice(0, idx).trim().toLowerCase() === 'status') {
      return trimmed.slice(idx + 1).trim().replace(/^["']|["']$/g, '').toLowerCase() === target
    }
    const marker = '| Status: '
    const markerIdx = trimmed.indexOf(marker)
    if (markerIdx >= 0) {
      let rest = trimmed.slice(markerIdx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim().toLowerCase() === target
    }
  }
  return false
}

// validateADRAcceptedWhenREQDone — REQ Done cujo ADR vinculado (campo "ADR:") não está aceito
// (Draft ou Proposed, via o helper canônico adrNotAcceptedStatusForRule) → violation. Fecha a
// lacuna que deixou um ADR Proposed atravessar sete REQs Done sem nenhum gate detectar.
function validateADRAcceptedWhenREQDone() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const violations = []
  for (const filePath of files) {
    const content = readFileForRule('adr_accepted_when_req_done', filePath, violations)
    if (content === null) continue
    if (!reqStatusEquals(content, 'done')) continue

    const adrRef = extractRefPath(content, 'ADR')
    if (!adrRef) continue
    const adrBasename = path.basename(adrRef)
    const status = adrNotAcceptedStatusForRule('adr_accepted_when_req_done', adrBasename, violations)
    if (status.notAccepted) {
      const reqBasename = path.basename(filePath)
      violations.push(
        `REQ "${reqBasename}" is Done but linked ADR "${adrBasename}" is not accepted (status: ${status.status})`
      )
    }
  }
  return violations
}

// blockedREQs retorna mapa de reqBasename → [adrBasenames Draft] para uso em getStatus()
function blockedREQs() {
  const cfg = config.load()
  const files = resolveReqFiles(cfg)
  const result = {}
  for (const filePath of files) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      continue
    }
    if (!content.includes('Status: Open')) continue

    const adrNames = parseBlockedADRs(filePath)
    const draftADRs = adrNames.filter(a => adrIsDraft(a))
    if (draftADRs.length > 0) {
      result[path.basename(filePath)] = draftADRs
    }
  }
  return result
}

// governanceModeFrom deriva { mode, lenientUntil } a partir do ProjectConfig já normalizado por
// config.load() — nenhuma releitura de trackfw.yaml acontece aqui. cfg.governanceMode chega como
// o valor bruto do campo (string vazia se ausente); cfg.lenientUntil chega como a data literal
// (ex.: "2026-08-02"), convertida aqui para Date.
function governanceModeFrom(cfg) {
  const mode = cfg.governanceMode ? cfg.governanceMode : 'strict'
  let lenientUntil = null
  if (cfg.lenientUntil) {
    const d = new Date(cfg.lenientUntil)
    if (!isNaN(d.getTime())) lenientUntil = d
  }
  return { mode, lenientUntil }
}

// isLenient retorna true se o projeto está em modo lenient e o prazo não expirou.
function isLenient() {
  const gm = governanceModeFrom(config.load())
  if (gm.mode !== 'lenient') return false
  if (!gm.lenientUntil) return true
  return new Date() < gm.lenientUntil
}

// lenientUntilDate retorna a data de expiração formatada (YYYY-MM-DD) ou ''.
function lenientUntilDate() {
  const gm = governanceModeFrom(config.load())
  if (gm.mode !== 'lenient' || !gm.lenientUntil) return ''
  return gm.lenientUntil.toISOString().slice(0, 10)
}

// validateFrontmatterPresence — verifica presença de frontmatter em ADRs e REQs
function validateFrontmatterPresence() {
  const cfg = config.load()
  const violations = []

  for (const adrDir of cfg.adrDirs) {
    for (const f of walkDirMd(adrDir)) {
      const fullPath = findAdrFile(f)
      if (!fullPath) continue
      try {
        const content = fs.readFileSync(fullPath, 'utf8')
        if (!content.startsWith('---')) {
          violations.push(`adr "${f}" has no frontmatter block`)
        }
      } catch (_) {}
    }
  }

  const reqFilePaths = resolveReqFiles(cfg)
  for (const filePath of reqFilePaths) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!content.startsWith('---')) {
        violations.push(`req "${path.basename(filePath)}" has no frontmatter block`)
      }
    } catch (_) {}
  }

  return violations
}

// extractRefPath extrai o valor de um campo (ex: "REQ", "ADR", "Roadmap") que aponta para .md
function extractRefPath(content, field) {
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(':')
    if (idx !== -1 && trimmed.slice(0, idx).trim().toLowerCase() === field.toLowerCase()) {
      let val = trimmed.slice(idx + 1).trim()
      if (!val || val === '—' || val === '-' || val === '–') return null
      val = val.split(/\s+/)[0]
      val = val.replace(/^["'`]|["'`]$/g, '')
      if (val.endsWith('.md')) return val
    }
  }
  return null
}

// validateRefTargetsExist — verifica se os arquivos referenciados em REQ:, ADR: e Roadmap: existem
function validateRefTargetsExist() {
  const cfg = config.load()
  const warnings = []

  // Roadmaps em wip e blocked: verificar REQ:
  const dirs = [...resolveWIPDirs(cfg), ...resolveStateDirs(cfg, 'blocked')]
  for (const dir of dirs) {
    for (const name of listDirForRule('ref_targets_exist', dir, warnings)) {
      const content = readFileForRule('ref_targets_exist', path.join(dir, name), warnings)
      if (content === null) continue
      const ref = extractRefPath(content, 'REQ')
      if (ref && !referenceExists(ref)) {
        warnings.push(`roadmap "${name}" links to REQ "${ref}" which does not exist`)
      }
    }
  }

  // REQs: verificar ADR: e Roadmap:
  for (const filePath of resolveReqFiles(cfg)) {
    const content = readFileForRule('ref_targets_exist', filePath, warnings)
    if (content === null) continue
    const name = path.basename(filePath)
    const adrRef = extractRefPath(content, 'ADR')
    if (adrRef && !referenceExists(adrRef)) {
      warnings.push(`req "${name}" links to ADR "${adrRef}" which does not exist`)
    }
    const roadmapRef = extractRefPath(content, 'Roadmap')
    if (roadmapRef && !referenceExists(roadmapRef)) {
      warnings.push(`req "${name}" links to Roadmap "${roadmapRef}" which does not exist`)
    }
  }

  return warnings
}

function referenceExists(ref) {
  const expandedRef = config.expandPath ? config.expandPath(ref) : ref
  if (fs.existsSync(expandedRef)) return true
  return false
}

function validateREQRoadmapLifecycle() {
  const cfg = config.load()
  const warnings = []
  for (const filePath of resolveReqFiles(cfg)) {
    try {
      const content = fs.readFileSync(filePath, 'utf8')
      if (!reqStatusIsOpen(content)) continue
      const ref = extractRefPath(content, 'Roadmap')
      if (!ref) continue
      const expandedRef = config.expandPath ? config.expandPath(ref) : ref
      if (!fs.existsSync(expandedRef)) continue
      if (path.basename(path.dirname(expandedRef)) === 'done') {
        warnings.push(`req "${path.basename(filePath)}" is Open but linked Roadmap "${ref}" is in done/`)
      }
    } catch (_) {}
  }
  return warnings
}

function reqStatusIsOpen(content) {
  for (const line of content.split('\n')) {
    const trimmed = line.trim()
    const idx = trimmed.indexOf(':')
    if (idx >= 0 && trimmed.slice(0, idx).trim().toLowerCase() === 'status') {
      return trimmed.slice(idx + 1).trim().replace(/^["']|["']$/g, '').toLowerCase() === 'open'
    }
    const marker = '| Status: '
    const markerIdx = trimmed.indexOf(marker)
    if (markerIdx >= 0) {
      let rest = trimmed.slice(markerIdx + marker.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) rest = rest.slice(0, pipeIdx)
      return rest.trim().toLowerCase() === 'open'
    }
  }
  return false
}

// FOLDER_TO_STATUS mapeia pasta de estado para os valores válidos de status no frontmatter
const FOLDER_TO_STATUS = {
  wip:       ['WIP', 'wip', 'In Progress'],
  backlog:   ['Backlog', 'backlog'],
  analyzing: ['Analyzing', 'analyzing'],
  blocked:   ['Blocked', 'blocked'],
  done:      ['Done', 'done'],
  abandoned: ['Abandoned', 'abandoned'],
}

// validateFolderStatusCoherence — verifica se o status declarado no frontmatter condiz com a pasta
function validateFolderStatusCoherence() {
  const cfg = config.load()
  const warnings = []
  const states = ['wip', 'backlog', 'analyzing', 'blocked', 'done', 'abandoned']

  let dirs = []
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    for (const agent of agents) {
      for (const state of states) {
        dirs.push({ dir: path.join(cfg.roadmapDir, agent, state), state })
      }
    }
  } else {
    for (const state of states) {
      dirs.push({ dir: path.join(cfg.roadmapDir, state), state })
    }
  }

  for (const { dir, state } of dirs) {
    // P2: distinguir "diretório ausente" (esperado) de outros erros (reportar).
    const { entries, readError } = tryListDir(dir)
    if (readError) {
      warnings.push(`folder_status: could not read directory "${dir}": ${readError.message}`)
      continue
    }
    for (const name of entries.filter(f => f.endsWith('.md'))) {
      try {
        const content = fs.readFileSync(path.join(dir, name), 'utf8')
        // Extrair status do frontmatter
        let declared = ''
        if (content.startsWith('---')) {
          const end = content.indexOf('\n---', 3)
          if (end > 0) {
            for (const line of content.slice(3, end).split('\n')) {
              const t = line.trim()
              if (t.startsWith('status:')) {
                declared = t.slice('status:'.length).trim().replace(/['"]/g, '')
                break
              }
            }
          }
        }
        if (!declared) continue
        const expected = FOLDER_TO_STATUS[state] || []
        if (!expected.some(e => e.toLowerCase() === declared.toLowerCase())) {
          warnings.push(`roadmap "${name}": folder is "${state}" but status declares "${declared}"`)
        }
      } catch (_) {}
    }
  }
  return warnings
}

// validateFilenameUniqueness — verifica que o mesmo filename não aparece em múltiplos estados
function validateFilenameUniqueness() {
  const cfg = config.load()
  const states = ['wip', 'backlog', 'analyzing', 'blocked', 'done', 'abandoned']
  const seen = {}  // filename → [states]

  const listErrors = []
  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    for (const agent of agents) {
      for (const state of states) {
        const dir = path.join(cfg.roadmapDir, agent, state)
        const { entries, readError } = tryListDir(dir)
        if (readError) {
          listErrors.push(`filename_uniqueness: could not read directory "${dir}": ${readError.message}`)
          continue
        }
        for (const name of entries) {
          const key = agent + '/' + name
          if (!seen[key]) seen[key] = []
          seen[key].push(state)
        }
      }
    }
  } else {
    for (const state of states) {
      const dir = path.join(cfg.roadmapDir, state)
      const { entries, readError } = tryListDir(dir)
      if (readError) {
        listErrors.push(`filename_uniqueness: could not read directory "${dir}": ${readError.message}`)
        continue
      }
      for (const name of entries) {
        if (!seen[name]) seen[name] = []
        seen[name].push(state)
      }
    }
  }

  const violations = [...listErrors]
  // P3: ordenar os estados dentro de cada mensagem e as mensagens pelo nome
  // para garantir saída determinística independente de ordem de inserção.
  const sortedNames = Object.keys(seen).sort()
  for (const name of sortedNames) {
    const stateList = seen[name]
    if (stateList.length > 1) {
      const sortedStates = [...stateList].sort()
      violations.push(`roadmap "${name}" appears in multiple states: [${sortedStates.join(', ')}]`)
    }
  }
  return violations
}

// branchSlugMatchesRoadmap verifica se branchSlug (já normalizado via normalizeBranchSlug) casa com o
// nome de algum roadmap .md encontrado em wipDirs ou doneDirs. Reutilizada por
// validateBranchHasWIPRoadmap e pelo comando `trackfw branch new` — nunca duplicar esta lógica.
//
// Espelha internal/validator/validator.go:BranchSlugMatchesRoadmap. Retorna { matched, candidates }:
// matched indica se algum candidato casou com o slug; candidates lista todos os roadmaps .md
// encontrados em wipDirs+doneDirs (para diagnóstico/mensagem de orientação quando matched é false).
function branchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs) {
  const dirs = [...wipDirs, ...doneDirs]
  const candidates = []
  let matched = false
  for (const dir of dirs) {
    const files = listDir(dir).filter(f => f.endsWith('.md'))
    candidates.push(...files)
    if (files.some(file => normalizeBranchSlug(file).includes(branchSlug))) matched = true
  }
  return { matched, candidates }
}

// branchGovernanceOrientation is the guidance message printed when a feat/fix/refactor branch has
// no roadmap in wip/ nor done/ at all (candidates is empty). Shared by validateBranchHasWIPRoadmap
// and `trackfw branch new` — never duplicate this string. Byte-identical to Go's
// BranchGovernanceOrientation.
function branchGovernanceOrientation(branch) {
  return `branch "${branch}" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — create governance artifacts first:\n  trackfw req new "title"\n  trackfw roadmap new "title"\n  trackfw roadmap move <name> wip`
}

// branchNoMatchingRoadmapMessage is the guidance message printed when roadmaps exist in wip/ or
// done/ but none of them match the branch's slug. Shared by validateBranchHasWIPRoadmap and
// `trackfw branch new` — never duplicate this string. Byte-identical to Go's
// BranchNoMatchingRoadmapMessage. Does not mutate candidates.
function branchNoMatchingRoadmapMessage(branch, candidates) {
  // P3: sort for deterministic output regardless of filesystem ordering.
  const sorted = [...candidates].sort()
  const display = sorted.slice(0, 3)
  const suffix = sorted.length > 3 ? `, e mais ${sorted.length - 3}` : ''
  return `branch "${branch}" has no matching roadmap in wip/ nor done/ (found: ${display.join(', ')}${suffix}) — include the branch slug in the roadmap filename or set TRACKFW_BRANCH explicitly in CI`
}

// validateBranchHasWIPRoadmap — verifica que branch feat/fix/refactor tem ao menos um roadmap em
// wip/ ou done/ cujo slug case com a branch. Aceita done/ para permitir encerramento do roadmap na
// própria branch, conforme a Definition of Done, sem reprovar o gate.
function validateBranchHasWIPRoadmap() {
  let branch = process.env.TRACKFW_BRANCH || ''
  if (!branch && isGitWorktree(process.cwd())) {
    try {
      branch = gitOutput(process.cwd(), ['symbolic-ref', '--short', 'HEAD']).trim()
    } catch {
      branch = ''
    }
    if (!branch) {
      branch = process.env.GITHUB_HEAD_REF || process.env.CI_COMMIT_REF_NAME || process.env.GITHUB_REF_NAME || ''
    }
  }
  if (!branch) return []
  if (!branch.startsWith('feat/') && !branch.startsWith('fix/') && !branch.startsWith('refactor/')) {
    return []
  }

  const cfg = config.load()
  const wipDirs = resolveWIPDirs(cfg)
  const doneDirs = resolveDoneDirs(cfg)
  const branchSlug = normalizeBranchSlug(branch.split('/', 2)[1])
  const { matched, candidates } = branchSlugMatchesRoadmap(branchSlug, wipDirs, doneDirs)
  if (matched) return []

  if (candidates.length === 0) {
    return [branchGovernanceOrientation(branch)]
  }
  return [branchNoMatchingRoadmapMessage(branch, candidates)]
}

function normalizeBranchSlug(value) {
  return String(value || '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

/**
 * Detecta notas em vault/notes/ não referenciadas pelo index.md.
 * index.md não conta como nota órfã. Projeto sem vault/ retorna [].
 * Aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`.
 * @param {string} [cwd]
 * @returns {string[]}
 */
function validateNoteOrphan(cwd) {
  const root = cwd || process.cwd()
  const vaultDir = path.join(root, 'vault', 'notes')
  if (!fs.existsSync(vaultDir)) return []

  const indexPath = path.join(vaultDir, 'index.md')
  let indexContent = ''
  if (fs.existsSync(indexPath)) {
    indexContent = fs.readFileSync(indexPath, 'utf8')
  }

  const notes = fs.readdirSync(vaultDir).filter((f) => f.endsWith('.md') && f !== 'index.md')
  const msgs = []
  for (const filename of notes) {
    const nameWithoutExt = filename.replace(/\.md$/, '')
    const referenced =
      indexContent.includes(`(${filename})`) ||
      indexContent.includes(`[[${nameWithoutExt}]]`) ||
      indexContent.includes(`[[${filename}]]`)
    if (!referenced) {
      msgs.push(`note "${filename}" is not referenced in vault/notes/index.md`)
    }
  }
  return msgs
}

// CREDENTIAL_GUARD_SCRIPT_MARKER é o nome do script que a regra credential_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto.
const CREDENTIAL_GUARD_SCRIPT_MARKER = 'trackfw-credential-guard.sh'

// GIT_BRANCH_GUARD_SCRIPT_MARKER é o nome do script que a regra git_branch_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto (ROADMAP-2026-08-15-trackfw-validate-deve-
// detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-2A — port de
// internal/validator/validator_git_branch_guard.go's gitBranchGuardScriptMarker). Mesmo padrão de
// CREDENTIAL_GUARD_SCRIPT_MARKER — só o nome do arquivo muda.
const GIT_BRANCH_GUARD_SCRIPT_MARKER = 'trackfw-git-branch-guard.sh'

// CREDENTIAL_GUARD_HOOK_FILES é a lista fechada dos arquivos de hook de PROJETO que o trackfw
// gera hoje e que podem conter uma entrada de credential-guard (ROADMAP-2026-08-12-mitigacao-do
// -fail-open-do-credential-guard, ML-1A). Hooks de escopo GLOBAL (~/.trackfw/..., trackfw update
// harness) ficam fora — caso distinto, fora do repositório do usuário, e a checagem de dedup
// globalCredentialGuardInstalled*() já os pula de propósito nas entradas de projeto.
//
// requiresCommandType (ROADMAP-2026-08-17 ML-4B, port of Go's credentialGuardHookFile.
// requiresCommandType): true for every CLI whose writer always emits a sibling "type":"command"
// field (Claude/Codex/Gemini, GitHub Copilot CLI, Kiro) — a command match found WITHOUT that
// sibling is a structurally malformed entry the CLI silently never executes (hades-tf ML-4A
// barrier finding), not merely "absent". false only for Cursor, whose schema
// ({"command":...}) never carries a "type" field at all.
//
// requiresVarOrShellPrefix (ROADMAP-2026-08-21 ML-1B, port of Go's
// credentialGuardHookFile.requiresVarOrShellPrefix): true for Claude/Codex/Gemini — the
// ADR-2026-08-11 requires these CLIs to anchor the hook path with $VAR/... or "$(git ...)/..."
// because their hooks run from the agent's cwd, not necessarily the project root. A bare
// relative path ("scripts/...") silently fails from any subdirectory (REQ-2026-08-17 root
// cause). false for Cursor/Copilot/Kiro, where the bare relative path IS the correct form —
// flagging them would be a false-positive (the dominant risk of this REQ).
const CREDENTIAL_GUARD_HOOK_FILES = [
  { relPath: '.claude/settings.json', cli: 'Claude Code', requiresCommandType: true, requiresVarOrShellPrefix: true },
  { relPath: '.codex/hooks.json', cli: 'Codex CLI', requiresCommandType: true, requiresVarOrShellPrefix: true },
  { relPath: '.gemini/settings.json', cli: 'Gemini CLI', requiresCommandType: true, requiresVarOrShellPrefix: true },
  { relPath: '.cursor/hooks.json', cli: 'Cursor', requiresCommandType: false, requiresVarOrShellPrefix: false },
  { relPath: '.github/hooks/trackfw-attention.json', cli: 'GitHub Copilot CLI', requiresCommandType: true, requiresVarOrShellPrefix: false },
  { relPath: '.kiro/hooks/trackfw-attention.json', cli: 'Kiro', requiresCommandType: true, requiresVarOrShellPrefix: false },
]

// resolveCredentialGuardHookPath resolve o valor bruto de um comando de hook (string extraída do
// JSON) para um caminho de arquivo absoluto, usando exatamente as 3 formas de prefixo que o
// trackfw emite hoje (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de
// projeto, por CLI"):
//   1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
//      runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
//   2. '"$(git rev-parse --show-toplevel)/…"' — substituição de shell entre aspas literais
//      (Codex). As aspas fazem parte do valor emitido e são removidas antes de resolver contra a
//      raiz do projeto.
//   3. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro) — resolvido diretamente
//      contra a raiz do projeto.
// Qualquer valor que não bata em nenhuma das 3 formas retorna null — o chamador NÃO deve tratar
// isso como violação. Não é função desta regra adivinhar wiring próprio do usuário fora dos
// formatos que o trackfw gera.
// pathIsAnchoredForHookConfig reporta se raw é um caminho ANCORADO para os fins de um comando de
// hook lido de config de CLI de agente e executado por bash — não se é absoluto para o SO host.
// Decisão: ADR-2026-09-04-caminho-posix-ancorado-num-config-lido-por-cli-de-agente-e-absoluto-
// independente-do-so-host. Quem interpreta esse caminho é o bash (ou o próprio CLI de agente),
// nunca o process.platform do runtime Node — por isso este predicado é a UNIÃO de duas formas,
// nunca a definição de "absoluto" de um único SO:
//   - POSIX: prefixo "/" — ancorado em qualquer host, inclusive rodando em Windows via Git Bash.
//   - Windows: letra de unidade ("C:\..." / "C:/...") ou UNC ("\\servidor\share") — ancorado em
//     qualquer host, inclusive medido a partir de macOS/Linux (D1 da ADR: uma letra de unidade
//     também é ancorada, independente de onde o validator roda).
//
// 🔴 Requisito duro da ADR: ZERO chamada dependente de SO aqui — nada de path.isAbsolute,
// path.sep, ou qualquer predicado do módulo "path" que responda com base no process.platform.
// `path.isAbsolute("/opt/foo")` é `false` no Windows — essa é exatamente a lacuna que motivou
// esta função. Invariante por construção, verificável por grep.
//
// D2 da ADR: este predicado NÃO substitui path.isAbsolute nos sítios de travessia real de
// sistema de arquivos (ex.: linha 95 e npm/src/integrations/manager.js) — ali o SO É a
// autoridade certa. Uso restrito aos sítios de classificação de config de hook.
function pathIsAnchoredForHookConfig(raw) {
  if (!raw) return false
  if (raw[0] === '/') return true
  // UNC: \\servidor\share\... — exige um segmento de SERVIDOR não vazio e diferente de "." ou
  // ".." (não são hostname válido), seguido de um separador, seguido de um segmento de SHARE não
  // vazio que não comece com outra barra invertida (evita componente vazio quando há barra dupla
  // no meio). "\\" e "\\x" sozinhos (sem separador de share), "\\.\x" / "\\..\evil" (server "."
  // ou ".."), e "\\..\\evil" (barra dupla no meio produz share vazio) NÃO são UNC válido — são
  // POSIX cwd-dependent (barra invertida não é separador em POSIX), e aceitá-los como ancorado
  // seria o afrouxamento inverso que este predicado existe para evitar (ressalva do parecer
  // hades-tf de 2026-09-04 sobre o ML-3A; a notação exata do exemplo "\\..\\evil" no parecer é
  // ambígua entre 1 e 2 barras antes de "evil" — a checagem de server !== "."/".." fecha as duas
  // leituras, não só uma). A forma POSIX equivalente, "//servidor/share", já é coberta pelo braço
  // raw[0]==='/' acima — este braço cobre só a forma com barra invertida.
  if (raw.length >= 2 && raw[0] === '\\' && raw[1] === '\\') {
    const rest = raw.slice(2)
    const sepIdx = rest.indexOf('\\')
    if (sepIdx > 0) {
      const server = rest.slice(0, sepIdx)
      const share = rest.slice(sepIdx + 1)
      if (server !== '.' && server !== '..' && share.length > 0 && share[0] !== '\\') return true
    }
  }
  // Letra de unidade: C:\... ou C:/...
  if (raw.length >= 3 && isASCIIDriveLetter(raw[0]) && raw[1] === ':' && (raw[2] === '\\' || raw[2] === '/')) {
    return true
  }
  return false
}

// isASCIIDriveLetter reporta se ch é uma letra ASCII (a-z, A-Z) — reconhecimento de letra de
// unidade do Windows em pathIsAnchoredForHookConfig.
function isASCIIDriveLetter(ch) {
  return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

function resolveCredentialGuardHookPath(raw, root) {
  const claudePrefix = '$CLAUDE_PROJECT_DIR/'
  const geminiPrefix = '$GEMINI_PROJECT_DIR/'
  const codexPrefix = '"$(git rev-parse --show-toplevel)/'

  if (raw.startsWith(claudePrefix)) {
    return path.join(root, raw.slice(claudePrefix.length))
  }
  if (raw.startsWith(geminiPrefix)) {
    return path.join(root, raw.slice(geminiPrefix.length))
  }
  if (raw.startsWith(codexPrefix) && raw.endsWith('"')) {
    const inner = raw.slice(codexPrefix.length, raw.length - 1)
    return path.join(root, inner)
  }
  if (!raw.startsWith('$') && !raw.startsWith('"') && !pathIsAnchoredForHookConfig(raw) && !raw.startsWith('~/')) {
    // Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
    // (campo "bash"), Kiro (action.command).
    // ~/… é excluído: é classe 1 (tilde expande para $HOME — ancorado) mas o validator não expande
    // o til; ok=false (null) silencia sem acusar.
    return path.join(root, raw)
  }
  return null
}

// HOOK_ANCHORAGE_CLASS_* classifica a semântica de ancoragem de um valor de comando de hook.
// Avaliada apenas para CLIs com requiresVarOrShellPrefix=true. Decisão: ADR-2026-08-22.
const HOOK_ANCHORAGE_CLASS_ANCHORED      = 1
const HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT = 2
const HOOK_ANCHORAGE_CLASS_UNDECIDABLE   = 3

// stripOuterQuotesForClassify remove aspas duplas envolventes de raw, se presentes. Necessário
// porque "$PWD/scripts/…" entre aspas (achado D.3) deve receber o mesmo veredito que $PWD/…
// sem aspas: as aspas são sintaxe, não semântica de ancoragem.
function stripOuterQuotesForClassify(raw) {
  if (raw.length >= 2 && raw[0] === '"' && raw[raw.length - 1] === '"') {
    return raw.slice(1, raw.length - 1)
  }
  return raw
}

// hookValueWasQuoted reporta se raw tinha aspas duplas externas que stripOuterQuotesForClassify
// removeria. Usado para distinguir ~/… (tilde expande para $HOME — classe 1) de "~/…" (tilde
// NÃO expande dentro de aspas duplas — classe 2).
function hookValueWasQuoted(raw) {
  return raw.length >= 2 && raw[0] === '"' && raw[raw.length - 1] === '"'
}

// classifyHookAnchorage retorna a classe de ancoragem de rawStripped (com aspas externas já
// removidas). wasQuoted indica se o valor original tinha aspas externas (obtido com
// hookValueWasQuoted antes de chamar stripOuterQuotesForClassify). Ver HOOK_ANCHORAGE_CLASS_*
// e ADR-2026-08-22.
function classifyHookAnchorage(rawStripped, wasQuoted) {
  // Classe 1 — ancora na raiz do projeto.
  if (
    rawStripped.startsWith('$CLAUDE_PROJECT_DIR/') ||
    rawStripped.startsWith('$GEMINI_PROJECT_DIR/') ||
    rawStripped.startsWith('$(git rev-parse --show-toplevel)/') ||
    pathIsAnchoredForHookConfig(rawStripped)
  ) {
    return HOOK_ANCHORAGE_CLASS_ANCHORED
  }
  // ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX — semanticamente ancorado.
  // "~/…" com aspas: tilde NÃO expande dentro de aspas duplas, logo a forma quebra — classe 2.
  if (rawStripped.startsWith('~/') && !wasQuoted) {
    return HOOK_ANCHORAGE_CLASS_ANCHORED
  }
  // Classe 2 — expande a partir do cwd.
  // ${PWD}/… tem a mesma semântica de $PWD/… (PWD é mandado pelo POSIX, sempre o cwd).
  if (
    rawStripped.startsWith('$PWD/') ||
    rawStripped.startsWith('${PWD}/') ||
    rawStripped.startsWith('./') ||
    rawStripped.startsWith('../') ||
    (!rawStripped.startsWith('$') && !pathIsAnchoredForHookConfig(rawStripped))
  ) {
    return HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT
  }
  // Classe 3 — indecidível; silêncio declarado.
  return HOOK_ANCHORAGE_CLASS_UNDECIDABLE
}

// cwdDependentReason retorna o sufixo de mensagem específico por forma para violações de
// classe 2, iniciando em "with a …". Formatos: formas contendo $PWD ou ${PWD} (em qualquer
// posição, inclusive em wrappers sh -c / env) recebem a mensagem do $PWD; "~/…" (D4 da
// ADR-2026-09-04) recebe "quoted tilde path"; "~usuario/…" recebe "named-user tilde path";
// demais recebem "bare relative path". Usa includes (não startsWith) para cobrir
// sh -c "$PWD/…" e env FOO=x $PWD/….
//
// 🔴 D4 da ADR-2026-09-04: achado do ML-2B — "~usuario/" e "\"~/\"" caíam no catch-all e
// recebiam "bare relative path", que não é o motivo real: ~user/ EXPANDE em POSIX (indecidível
// sem executar shell, não relativo), e "~/" só falha por causa das ASPAS. O invariante que
// permite reconhecer "~/" aqui sem receber wasQuoted como parâmetro: classifyHookAnchorage já
// classifica "~/…" SEM aspas como classe 1 (ancorado) — logo todo "~/…" que chega aqui (classe
// 2) só pode ter vindo de um valor originalmente citado; ver stripOuterQuotesForClassify.
function cwdDependentReason(rawStripped) {
  if (rawStripped.includes('$PWD') || rawStripped.includes('${PWD}')) {
    return 'with a $PWD path — $PWD expands to the current working directory, not the project root; run `trackfw update` to fix it'
  }
  if (rawStripped.startsWith('~/')) {
    return 'with a quoted tilde path — double quotes prevent shell tilde expansion, so this never resolves to $HOME; run `trackfw update` to fix it'
  }
  if (rawStripped.startsWith('~')) {
    return "with a named-user tilde path — ~user/ expands to that user's home directory, not the project root, and this validator cannot resolve it without running a shell; run `trackfw update` to fix it"
  }
  return "with a bare relative path — this command only resolves from the project root and will silently fail when the agent's cwd is a subdirectory; run `trackfw update` to fix it"
}

// collectCommandsWithMarker percorre recursivamente um valor JSON já decodificado e coleta todo
// valor-string que contém marker, independentemente do nome do campo que o contém — junto com um
// sinal estrutural (typeIsCommand) de que o objeto imediato que o contém também tem
// "type":"command" como campo irmão (ROADMAP-2026-08-17 ML-4B, port de guardCommandMatch em
// internal/validator/validator_credential_guard.go). Cada entrada de `out` é
// {raw, typeIsCommand}.
//
// Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
// Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
// de por caminho de chave evita acoplar esta regra à forma exata de cada schema. Todo schema aqui
// coloca "type" como IRMÃO do campo do comando dentro do MESMO objeto — nunca aninhado mais fundo
// — então basta ler "type" do objeto que collectCommandsWithMarker já está visitando; não é
// necessária uma travessia schema-aware separada.
//
// Generalizado (ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
// desatualizados, ML-2A, port de collectCommandsWithMarker em
// internal/validator/validator_git_branch_guard.go) para aceitar qualquer marker — originalmente
// collectCredentialGuardCommands, hardcoded para CREDENTIAL_GUARD_SCRIPT_MARKER; reusado agora
// também para GIT_BRANCH_GUARD_SCRIPT_MARKER, sem duplicar a travessia recursiva.
function collectCommandsWithMarker(value, marker, out) {
  if (typeof value === 'string') {
    // Top-level/loose string match, outside any enclosing object — defensive fallback, not
    // expected to fire since every hook file read here is rooted at a JSON object. No enclosing
    // object means no "type" sibling to read.
    if (value.includes(marker)) out.push({ raw: value, typeIsCommand: false })
    return
  }
  if (Array.isArray(value)) {
    for (const item of value) collectCommandsWithMarker(item, marker, out)
    return
  }
  if (value && typeof value === 'object') {
    const typeIsCommand = value.type === 'command'
    for (const key of Object.keys(value)) {
      const val = value[key]
      if (typeof val === 'string') {
        if (val.includes(marker)) out.push({ raw: val, typeIsCommand })
        continue
      }
      collectCommandsWithMarker(val, marker, out)
    }
  }
}

// collectCredentialGuardCommands é o wrapper específico de credential-guard sobre
// collectCommandsWithMarker — preservado para compatibilidade de assinatura (exportada
// historicamente com 2 argumentos).
function collectCredentialGuardCommands(value, out) {
  collectCommandsWithMarker(value, CREDENTIAL_GUARD_SCRIPT_MARKER, out)
}

// validateGuardHookResolvable é a implementação genérica compartilhada pelas regras
// "credential_guard_hook_resolvable" e "git_branch_guard_hook_resolvable": para cada arquivo de
// hook de PROJETO que existir, extrai os comandos que referenciam scriptMarker, resolve o caminho
// e verifica que o script existe e é executável.
//
// Generalizado a partir da antiga validateCredentialGuardHookResolvable (ROADMAP-2026-08-15-
// trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-2A, port de
// validateGuardHookResolvable em internal/validator/validator_credential_guard.go) — a lógica de
// resolução de caminho por CLI é idêntica para os 2 scripts, só o marker e o texto da mensagem
// mudam.
//
// Riscos de regressão mapeados no roadmap:
//   - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo
//     (guard global instalado via `trackfw update harness`) — nunca é violação por si só.
//   - Arquivo de hook ausente é pulado em silêncio.
//   - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
//     JSON não é escopo desta regra.
//
// process.cwd() no Node já retorna o caminho FÍSICO (resolve symlinks via getcwd(3) diretamente),
// diferente de os.Getwd() do Go — ver o comentário sobre EvalSymlinks em
// internal/validator/validator_git_branch_guard.go's validateGuardHookResolvable equivalente
// (validator_credential_guard.go). Nenhuma resolução extra de symlink é necessária aqui.
function validateGuardHookResolvable(scriptMarker, cwd) {
  const root = cwd || process.cwd()
  const msgs = []

  for (const hf of CREDENTIAL_GUARD_HOOK_FILES) {
    const fullPath = path.join(root, hf.relPath)
    let content
    try {
      content = fs.readFileSync(fullPath, 'utf8')
    } catch (e) {
      if (e.code === 'ENOENT') continue
      continue
    }

    let parsed
    try {
      parsed = JSON.parse(content)
    } catch (_) {
      continue
    }

    const commands = []
    collectCommandsWithMarker(parsed, scriptMarker, commands)

    const seen = new Set()
    for (const m of commands) {
      const seenKey = `${m.raw} ${m.typeIsCommand}`
      if (seen.has(seenKey)) continue
      seen.add(seenKey)

      // ADR-2026-08-22: classificar por ancoragem ANTES de resolver. Aspas externas são
      // sintaxe, não semântica — removê-las antes garante que "$PWD/…" receba o mesmo veredito.
      // wasQuoted distingue ~/… (classe 1) de "~/…" (classe 2).
      const wasQuoted = hookValueWasQuoted(m.raw)
      const rawStripped = stripOuterQuotesForClassify(m.raw)
      const anchorageClass = classifyHookAnchorage(rawStripped, wasQuoted)
      if (hf.requiresVarOrShellPrefix && anchorageClass === HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} ${cwdDependentReason(rawStripped)}`)
        continue
      }

      // Classe 1 (ancorado) e classe 3 (indecidível) prosseguem para a resolução existente.
      const resolved = resolveCredentialGuardHookPath(m.raw, root)
      if (resolved === null) continue

      // ROADMAP-2026-08-17 ML-4B: a command that resolves to a real path but sits inside a
      // structurally malformed entry (missing/wrong "type" where this CLI's schema requires it —
      // hades-tf ML-4A barrier finding) will NEVER be executed by the CLI, regardless of whether
      // the script itself exists and is executable. Reported instead of the exists/executable
      // checks below, which assume a structurally valid entry.
      if (hf.requiresCommandType && !m.typeIsCommand) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} resolved to "${resolved}", but the hook entry is missing "type":"command" (or has an invalid type) — ${hf.cli} will silently never execute it; run \`trackfw update\` to regenerate it`)
        continue
      }

      let stat = null
      try {
        stat = fs.statSync(resolved)
      } catch (_) {
        stat = null
      }

      if (!stat) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} resolved to "${resolved}", but the script does not exist — run \`trackfw update\` to regenerate it`)
      } else if (_platform !== 'win32' && (stat.mode & 0o111) === 0) {
        msgs.push(`${hf.relPath} (${hf.cli}) references ${scriptMarker} resolved to "${resolved}", but the script is not executable — run \`trackfw update\` to regenerate it`)
      }
    }
  }

  return msgs
}

// validateCredentialGuardHookResolvable é a regra "credential_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
function validateCredentialGuardHookResolvable(cwd) {
  return validateGuardHookResolvable(CREDENTIAL_GUARD_SCRIPT_MARKER, cwd)
}

// validateGitBranchGuardHookResolvable é a regra "git_branch_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
function validateGitBranchGuardHookResolvable(cwd) {
  return validateGuardHookResolvable(GIT_BRANCH_GUARD_SCRIPT_MARKER, cwd)
}

function isGitWorktree(dir) {
  try {
    const out = gitOutput(dir, ['rev-parse', '--is-inside-work-tree'])
    return String(out).trim() === 'true'
  } catch {
    return false
  }
}

// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-
// resposta-e-deteccao-ancorada-no-git.md (Emenda 1: âncora POR ALVO, decidida na Barreira B0).
// Ver internal/validator/validator_credential_guard_integrity.go para o raciocínio completo
// (âncoras, severidades, e por que "trackfw.yaml sem a chave" é lido como HEAD sem a chave, não
// disco sem a chave) — replicado aqui byte-a-byte na semântica, não na forma.

// CREDENTIAL_GUARD_SCRIPT_REFERENCE is a validator-local copy of the same template composed in
// generators/hooks.js (CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_PROJECT_GUARD + CG_DETECTION_CORE
// + CG_PROJECT_TAIL). Not required directly from generators/hooks.js because
// generateCredentialGuardScript() writes to disk AND console.log()s a success line on every call
// -- calling it from inside `trackfw validate` would leak a stray "  \u2713
// scripts/trackfw-credential-guard.sh" line into validate's output on every run, corrupting the
// exact success message Cenario 29 fixes byte-for-byte. Kept as a literal copy (same choice made
// in internal/validator/validator_credential_guard_integrity_reference.go for Go, for the same
// reason) instead -- drift is caught by test/validator-credential-guard-script-reference.test.js,
// which regenerates the real script via generateCredentialGuardScript() into a temp dir and
// asserts byte-equality against this constant.
const CREDENTIAL_GUARD_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \\" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\\$\\{?[A-Za-z_][A-Za-z0-9_]*\\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\\$\\{?([A-Za-z_][A-Za-z0-9_]*)\\}?$/\\1/')
    pattern="*\${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=\${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="\${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=\${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// validateCredentialGuardScriptIntegrity é a regra "credential_guard_script_integrity": compara
// scripts/trackfw-credential-guard.sh em disco contra o template que esta versão do trackfw
// geraria. Silenciosa quando o script não existe — ausência é escopo de
// credential_guard_hook_resolvable, não desta regra.
function validateCredentialGuardScriptIntegrity(cwd) {
  const root = cwd || process.cwd()
  const relPath = 'scripts/trackfw-credential-guard.sh'
  let content
  try {
    content = fs.readFileSync(path.join(root, relPath), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return []
    return [inspectionDiagnostic('credential_guard_script_integrity', relPath, e)]
  }

  if (content === CREDENTIAL_GUARD_SCRIPT_REFERENCE) return []

  return [
    `${relPath} content diverges from the template this version of trackfw generates — ` +
    'if you did not edit this file by hand, run `trackfw update` to regenerate it',
  ]
}

// ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
// ML-2A: port of internal/validator/validator_git_branch_guard.go — adds git-branch-guard
// coverage to the two existing credential-guard checks (existence/executability via
// validateGuardHookResolvable above, and content-drift integrity via
// validateGitBranchGuardScriptIntegrity below), plus the GLOBAL-scope check that was missing for
// BOTH guards before this ML (same gap the Go implementation closed first).

// GBG_REF_BACKTICK — validator-local copy of the same escaped-backtick helper hooks.js's
// GBG_BACKTICK uses (npm/src/generators/hooks.js), to embed REASON's `` `cmd` `` inside the
// script's double-quoted shell string without breaking this JS template literal.
const GBG_REF_BACKTICK = '\\`'

// GIT_BRANCH_GUARD_SCRIPT_REFERENCE is a validator-local copy of the
// scripts/trackfw-git-branch-guard.sh template composed in npm/src/generators/hooks.js
// (GIT_BRANCH_GUARD_SCRIPT const). Not required directly from generators/hooks.js: like
// CREDENTIAL_GUARD_SCRIPT_REFERENCE above, generateGitBranchGuardScript() writes to disk AND
// console.log()s a success line on every call — calling it from inside `trackfw validate` would
// leak a stray "  ✓ scripts/trackfw-git-branch-guard.sh" line into validate's output. There is no
// import-cycle constraint in Node (unlike Go's internal/validator ↔ internal/generators cycle —
// see gitBranchGuardScriptReference's doc comment in
// internal/validator/validator_git_branch_guard_reference.go) but the console.log side effect is
// reason enough on its own to keep a local copy here, matching the choice already made for
// credential-guard in this same file.
//
// Unlike CREDENTIAL_GUARD_SCRIPT_REFERENCE, GIT_BRANCH_GUARD_SCRIPT_REFERENCE is used VERBATIM for
// both the project scope and the global scope (generateGitBranchGuardScript /
// generateGlobalGitBranchGuardScript write the exact same GIT_BRANCH_GUARD_SCRIPT constant) — so
// this single reference constant covers both git_branch_guard_script_integrity (project,
// scripts/trackfw-git-branch-guard.sh) and the global integrity check
// (~/.trackfw/scripts/trackfw-git-branch-guard.sh), no second reference constant needed.
//
// Drift between this copy and the real generator is caught by
// "GIT_BRANCH_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateGitBranchGuardScript emite"
// (npm/tests/git_branch_guard.test.js): it regenerates the script via generateGitBranchGuardScript
// into a temp dir and asserts byte-equality against this constant.
const GIT_BRANCH_GUARD_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b/branch/worktree add -b
# brutos por subagente
#
# TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA: detecta o caso óbvio — comando git literal, sem
# indireção de shell — não é defesa contra um agente adversário competente. Evasões que
# exigem tokenizar como o bash (ex.: git\${IFS}push, {git,push}, g""it push) permanecem
# abertas por decisão: ver docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-
# com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md. O stripping de
# env/command abaixo reconhece as formas SEM argumentos antes de git (env git ...,
# command git ...) e o env seguido de uma sequência de atribuições CHAVE=valor
# (env FOO=bar git ..., env FOO=bar BAZ=qux git ...) — env com FLAGS (env -i git ...,
# env --ignore-environment git ...) e command com flags (command -p git ...) continuam
# evadindo; declarado, não fechado (ver AC5 do ML que adicionou esse stripping). A
# segmentação abaixo
# (quote_aware_split) evita falso-positivo em texto citado — não deve ser lida como imune a
# evasão por citação/tokenização do shell.
set -euo pipefail
set -f

# --- 0. Drena o stdin ANTES de qualquer saída antecipada (ML-1B, ROADMAP-2026-08-17-guard-
# global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md): sem isso,
# quem escreve o payload JSON no pipe recebe EPIPE quando o no-op abaixo sai com 0 antes de ler
# — reprodutível em 100% das chamadas fora de projeto trackfw, não é corrida de timing. Só drena
# se stdin não for um terminal interativo (-t 0): em invocação manual sem pipe, "cat" bloquearia
# esperando EOF (Ctrl-D). O valor lido é reaproveitado no passo 1 abaixo — nunca há uma segunda
# leitura.
_TRACKFW_STDIN=""
[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)

# --- 0b. No-op fora de projeto trackfw (ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-trackfw.md): sobe diretórios a partir do cwd FÍSICO (pwd -P, resolve symlink) até
# achar trackfw.yaml na raiz do projeto. Sem trackfw.yaml em nenhum ancestral, o guard não se
# aplica — fora de projeto trackfw não há trackfw ship como alternativa, e bloquear ali é custo
# sem contrapartida. Custo medido: só parameter expansion e test -f por nível, nenhum fork de
# processo; limitado pela profundidade do caminho.
_TRACKFW_ROOT_DIR=$(pwd -P)
_TRACKFW_FOUND=0
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then
    _TRACKFW_FOUND=1
    break
  fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then
    break
  fi
  _TRACKFW_ROOT_DIR="\${_TRACKFW_ROOT_DIR%/*}"
  if [ -z "$_TRACKFW_ROOT_DIR" ]; then
    _TRACKFW_ROOT_DIR="/"
  fi
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT="$_TRACKFW_STDIN"
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \\{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="\${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Pré-processamento anti-falso-positivo: neutraliza separadores reais (';', '&&',
# '||', '|', quebra de linha) que estão DENTRO de aspas ou de corpo de heredoc, para que
# conteúdo de mensagem (ex.: ` + "`" + `-m "linha 1\\nlinha 2"` + "`" + `) nunca seja fatiado em pseudo-segmentos
# e lido como comando -------------------------------------------------------------------
#
# strip_heredoc_bodies: remove o CORPO de blocos heredoc (<<DELIM ... DELIM), preservando a
# linha de abertura e a linha terminadora — cobre o padrão ` + "`" + `git commit -F- <<'EOF' ... EOF` + "`" + `
# (heredoc não citado, fora do escopo de quote_aware_split abaixo). Heurística por linha, não
# sintaxe completa de shell: só remove o corpo quando encontra a linha terminadora
# correspondente. Se o heredoc nunca fecha (terminador ausente ou não localizado), devolve o
# texto ORIGINAL sem qualquer alteração — lado seguro: mais restritivo é preferível a esconder
# um comando real atrás de um heredoc mal-formado.
strip_heredoc_bodies() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      in_heredoc = 0
      delim = ""
      ok = 1
    }
    {
      raw = raw $0 "\\n"
      if (in_heredoc) {
        trimmed = $0
        sub(/^[ \\t]+/, "", trimmed)
        sub(/[ \\t]+$/, "", trimmed)
        if (trimmed == delim) {
          in_heredoc = 0
          out = out $0 "\\n"
        }
        next
      }
      if (match($0, /<<-?[ \\t]*[^ \\t]+/)) {
        d = substr($0, RSTART, RLENGTH)
        sub(/^<<-?[ \\t]*/, "", d)
        gsub(dq, "", d)
        gsub(sq, "", d)
        if (d != "") {
          delim = d
          in_heredoc = 1
        }
      }
      out = out $0 "\\n"
    }
    END {
      if (in_heredoc) ok = 0
      if (ok) { printf "%s", out } else { printf "%s", raw }
    }
  '
}

# quote_aware_split: emite o texto com ';' isolado, '&&', '||' e '|' isolado convertidos em
# quebra de linha — EXCETO quando ocorrem dentro de uma string entre aspas simples ou duplas,
# caso em que são preservados como texto e uma quebra de linha real dentro das aspas vira
# espaço (nunca gera um novo pseudo-segmento). Substitui o antigo ` + "`" + `sed` + "`" + ` cego, que não
# distinguia texto citado de sintaxe de comando — a causa raiz do falso-positivo de linha de
# mensagem de commit iniciada por "git ...". Aspas não fechadas até o fim da entrada
# permanecem "abertas" até o fim — mesma semântica do shell real: uma aspa não fechada nunca
# deixa o texto seguinte executar como comando novo, só torna o restante parte da mesma
# string.
quote_aware_split() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      bs = sprintf("%c", 92)
      nl = sprintf("%c", 10)
    }
    { s = (NR == 1) ? $0 : s nl $0 }
    END {
      n = length(s)
      q = ""
      out = ""
      i = 1
      while (i <= n) {
        c = substr(s, i, 1)
        if (q != "") {
          if (q == dq && c == bs && i < n) {
            nx = substr(s, i + 1, 1)
            out = out c (nx == nl ? " " : nx)
            i += 2
            continue
          }
          if (c == q) {
            q = ""
            out = out c
            i++
            continue
          }
          out = out (c == nl ? " " : c)
          i++
          continue
        }
        if (c == dq || c == sq) {
          q = c
          out = out c
          i++
          continue
        }
        if (substr(s, i, 2) == "&&" || substr(s, i, 2) == "||") {
          out = out nl
          i += 2
          continue
        }
        if (c == ";" || c == "|") {
          out = out nl
          i++
          continue
        }
        out = out c
        i++
      }
      printf "%s", out
    }
  '
}

# match_subcommand — casa contra "git (commit|push|checkout -b|switch -c)", segmento por
# segmento. Cada segmento é um comando real, obtido depois do pré-processamento acima
# (strip_heredoc_bodies + quote_aware_split), que converte ';', '&&', '||', '|' fora de aspas
# em quebra de linha e neutraliza os mesmos separadores quando aparecem dentro de
# aspas/heredoc. "git" só conta se for o PRIMEIRO token do segmento (por basename, então
# /usr/bin/git também casa) — nunca uma ocorrência solta em qualquer posição da string
# inteira. Isso evita: (a) o segundo comando de uma cadeia escapar da checagem, (b) um path
# absoluto para o git escapar por comparação de igualdade exata, e (c) texto de prosa —
# inclusive linha de mensagem de commit que COMEÇA com "git <sub>" (ex.: uma tabela
# documentando comandos bloqueados) — ser tratado como comando, porque esse texto agora nunca
# produz um novo segmento. ` + "`" + `git switch -c/-C/--create` + "`" + ` (forma alternativa a ` + "`" + `checkout -b` + "`" + `
# para criar branch) é reconhecido varrendo TODOS os tokens após o subcomando, não só o
# primeiro — cobre ` + "`" + `git switch --track -c feat/x` + "`" + ` (flag antes de -c).
# checkout -b é reconhecido do mesmo jeito: varre TODOS os tokens até achar -b/-B/--orphan,
# não só o primeiro. Prefixos env e command antes de git são descartados antes da checagem do
# basename — cobre env git push/command git push sem exigir tokenizar como o bash.
match_subcommand() {
  normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="\${first##*/}"
    while [ "$base" = "env" ] || [ "$base" = "command" ]; do
      is_env="$base"
      shift
      [ "$#" -gt 0 ] || break
      if [ "$is_env" = "env" ]; then
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -*)
              break
              ;;
            *=*)
              shift
              ;;
            *)
              break
              ;;
          esac
        done
        [ "$#" -gt 0 ] || break
      fi
      first="$1"
      base="\${first##*/}"
    done
    [ "$base" = "git" ] || continue
    shift

    sub=""
    while [ "$#" -gt 0 ]; do
      tok="$1"
      case "$tok" in
        -C|-c|--work-tree|--git-dir|--namespace)
          if [ "$#" -ge 2 ]; then shift 2; else shift; fi
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          sub="$tok"
          shift
          break
          ;;
      esac
    done

    case "$sub" in
      commit)
        echo "commit"
        return 0
        ;;
      push)
        echo "push"
        return 0
        ;;
      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
        # git checkout -- <path> | git checkout . descarta alterações não commitadas do
        # caminho indicado, de forma irreversível, no worktree compartilhado — bloqueia
        # quando '--' aparece em qualquer posição (forma explícita de pathspec) ou quando
        # '.' aparece como token isolado. 'git checkout <branch>' sem nenhum dos dois
        # segue liberado por decisão (distinguir branch de caminho sem '--' é ambíguo, e
        # adivinhar produziria falso-positivo).
        checkout_path=0
        for tok2 in "$@"; do
          case "$tok2" in
            --|.)
              checkout_path=1
              ;;
          esac
        done
        if [ "$checkout_path" = "1" ]; then
          echo "checkout-path"
          return 0
        fi
        ;;
      switch)
        for tok2 in "$@"; do
          case "$tok2" in
            -c|-C|--create|--create=*|--force-create|--force-create=*)
              echo "switch-c"
              return 0
              ;;
          esac
        done
        ;;
      stash)
        # git stash: liberado só para leitura (list/show) — bloqueia a forma bare
        # (equivale a "push"), push, save, clear e drop. Decisão de KG: bloquear a
        # classe inteira, não só os literais medidos (ver REQ). Repositório com um único
        # worktree compartilhado entre subagentes paralelos — um stash de um agente
        # remove as alterações não commitadas de todos os outros.
        stash_sub="\${1:-}"
        case "$stash_sub" in
          list|show)
            ;;
          *)
            echo "stash"
            return 0
            ;;
        esac
        ;;
      reset)
        # Só --hard bloqueia, em qualquer posição de token — --soft/--mixed (inclusive
        # sem flag, que é --mixed implícito) seguem liberados: --soft é o contorno
        # padrão para reempurrar trabalho staged via ` + "`" + `trackfw ship -m "..."` + "`" + ` (ainda falta commitar após --soft).
        for tok2 in "$@"; do
          case "$tok2" in
            --hard)
              echo "reset-hard"
              return 0
              ;;
          esac
        done
        ;;
      clean)
        # Bloqueia qualquer forma com force (-f, -fd, -fx, --force) ou -x/-X, EXCETO
        # quando -n/--dry-run também está presente (dry-run nunca apaga nada).
        clean_dry=0
        clean_force=0
        for tok2 in "$@"; do
          case "$tok2" in
            -n|--dry-run)
              clean_dry=1
              ;;
            -f*|--force|--force=*|-x|-X)
              clean_force=1
              ;;
          esac
        done
        if [ "$clean_dry" != "1" ] && [ "$clean_force" = "1" ]; then
          echo "clean-force"
          return 0
        fi
        ;;
      restore)
        # git restore --staged SOZINHO nunca toca o working tree (mexe só no
        # index), então segue liberado mesmo com path. Mas --worktree/-W (com ou
        # sem --staged junto) SEMPRE afeta o working tree — inclusive
        # "--staged --worktree", que restaura os dois — então bloqueia sempre que
        # --worktree/-W aparecer, e também no caso padrão (sem --staged em
        # nenhuma forma) com um argumento posicional (o path).
        restore_staged=0
        restore_worktree=0
        restore_positional=0
        for tok2 in "$@"; do
          case "$tok2" in
            --staged)
              restore_staged=1
              ;;
            --worktree|-W)
              restore_worktree=1
              ;;
            -*)
              ;;
            *)
              restore_positional=1
              ;;
          esac
        done
        if [ "$restore_positional" = "1" ]; then
          if [ "$restore_worktree" = "1" ] || [ "$restore_staged" != "1" ]; then
            echo "restore-path"
            return 0
          fi
        fi
        ;;
      branch)
        # git branch é majoritariamente leitura (sem args, -a, -r, -l, --list, -v/-vv,
        # --show-current, --contains, --no-contains, --merged, --no-merged, --sort=,
        # --format=, --points-at, -d/-D/--delete) — bloquear leitura seria pior que a
        # brecha. Só bloqueia: (a) -c/-C/-m/-M/--copy/--move (cria/renomeia branch,
        # qualquer posição de token) ou (b) um argumento posicional puro (nome da branch a
        # criar), a menos que -d/-D/--delete também esteja presente (delete tem
        # posicional legítimo — o nome a apagar). Flags de valor conhecidas (--contains,
        # --no-contains, --sort, --format, --points-at, --merged, --no-merged) têm seu
        # valor seguinte pulado quando vem em token separado, para não ser lido como
        # posicional de criação.
        branch_action=0
        has_delete=0
        saw_positional=0
        skip_next=0
        for tok2 in "$@"; do
          if [ "$skip_next" = "1" ]; then
            skip_next=0
            continue
          fi
          case "$tok2" in
            -c|-C|-m|-M|--copy|--copy=*|--move|--move=*)
              branch_action=1
              ;;
            -d|-D|--delete|--delete=*)
              has_delete=1
              ;;
            --contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged)
              skip_next=1
              ;;
            -*)
              ;;
            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then
          if [ "$branch_action" = "1" ] || [ "$saw_positional" = "1" ]; then
            echo "branch-create"
            return 0
          fi
        fi
        ;;
      worktree)
        if [ "\${1:-}" = "add" ]; then
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -b|-B)
                echo "worktree-add-b"
                return 0
                ;;
            esac
          done
        elif [ "\${1:-}" = "remove" ]; then
          # git worktree remove SEM -f/--force já recusa sozinho quando há alteração não
          # commitada no worktree indicado — só a forma com force é irreversível o bastante
          # para bloquear aqui.
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -f|--force)
                echo "worktree-remove-force"
                return 0
                ;;
            esac
          done
        fi
        ;;
      update-ref)
        # git update-ref reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o
        # objeto apontado nem exigir push — foi o mecanismo que tornou alcançável o exploit
        # descrito no ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md
        # (Emenda 1): forjar origin/<base> localmente para desviar o commit-alvo de trackfw
        # release tag. Sem forma de leitura equivalente a bloquear seletivamente — a
        # subcommand inteira é escrita — bloqueia sempre, sem exceção de token.
        echo "update-ref"
        return 0
        ;;
      rm)
        # git rm -f/--force apaga do working tree e do index de forma irreversível, mesma
        # classe de git clean -f/git reset --hard já bloqueados acima — sem exceção para
        # --cached (destrancar do index sem -f já segue liberado por não precisar de force).
        for tok2 in "$@"; do
          case "$tok2" in
            -f*|--force|--force=*)
              echo "rm-force"
              return 0
              ;;
          esac
        done
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_REF_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  switch-c)
    REASON="trackfw: git switch -c bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_REF_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  branch-create)
    REASON="trackfw: git branch bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_REF_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-add-b)
    REASON="trackfw: git worktree add -b bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw branch new <type>/<slug>` + GBG_REF_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw commit -m '<mensagem>'` + GBG_REF_BACKTICK + `. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use ` + GBG_REF_BACKTICK + `trackfw push` + GBG_REF_BACKTICK + ` (para empurrar commits já criados), ` + GBG_REF_BACKTICK + `trackfw ship` + GBG_REF_BACKTICK + ` (para commit+push+PR em uma etapa) ou ` + GBG_REF_BACKTICK + `trackfw release tag` + GBG_REF_BACKTICK + ` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  stash)
    REASON="trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. ` + GBG_REF_BACKTICK + `git stash list` + GBG_REF_BACKTICK + `/` + GBG_REF_BACKTICK + `git stash show` + GBG_REF_BACKTICK + ` seguem liberados; para guardar trabalho em progresso, use uma branch própria via ` + GBG_REF_BACKTICK + `trackfw branch new` + GBG_REF_BACKTICK + ` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  reset-hard)
    REASON="trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. ` + GBG_REF_BACKTICK + `git reset --soft` + GBG_REF_BACKTICK + `/` + GBG_REF_BACKTICK + `--mixed` + GBG_REF_BACKTICK + ` seguem liberados (ex.: ` + GBG_REF_BACKTICK + `git reset --soft HEAD~1` + GBG_REF_BACKTICK + ` é o caminho padrão; use ` + GBG_REF_BACKTICK + `trackfw ship -m "..."` + GBG_REF_BACKTICK + ` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  clean-force)
    REASON="trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. ` + GBG_REF_BACKTICK + `git clean -n` + GBG_REF_BACKTICK + `/` + GBG_REF_BACKTICK + `--dry-run` + GBG_REF_BACKTICK + ` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  restore-path)
    REASON="trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. ` + GBG_REF_BACKTICK + `git restore --staged` + GBG_REF_BACKTICK + ` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  checkout-path)
    REASON="trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. ` + GBG_REF_BACKTICK + `git checkout <branch>` + GBG_REF_BACKTICK + `/` + GBG_REF_BACKTICK + `git switch <branch>` + GBG_REF_BACKTICK + ` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  update-ref)
    REASON="trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que ` + GBG_REF_BACKTICK + `trackfw release tag` + GBG_REF_BACKTICK + ` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-remove-force)
    REASON="trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. ` + GBG_REF_BACKTICK + `git worktree remove` + GBG_REF_BACKTICK + ` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  rm-force)
    REASON="trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de ` + GBG_REF_BACKTICK + `git clean -f` + GBG_REF_BACKTICK + `/` + GBG_REF_BACKTICK + `git reset --hard` + GBG_REF_BACKTICK + ` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\\n' "$REASON"
echo "$REASON" >&2
exit 2
`

// validateGitBranchGuardScriptIntegrity é a regra "git_branch_guard_script_integrity": compara
// scripts/trackfw-git-branch-guard.sh em disco contra o template que esta versão do trackfw
// geraria. Espelha validateCredentialGuardScriptIntegrity exatamente — mesmo contrato de silêncio
// na ausência (existência é responsabilidade de git_branch_guard_hook_resolvable, não desta regra).
function validateGitBranchGuardScriptIntegrity(cwd) {
  const root = cwd || process.cwd()
  const relPath = 'scripts/trackfw-git-branch-guard.sh'
  let content
  try {
    content = fs.readFileSync(path.join(root, relPath), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return []
    return [inspectionDiagnostic('git_branch_guard_script_integrity', relPath, e)]
  }

  if (content === GIT_BRANCH_GUARD_SCRIPT_REFERENCE) return []

  return [
    `${relPath} content diverges from the template this version of trackfw generates — ` +
    'if you did not edit this file by hand, run `trackfw update` to regenerate it',
  ]
}

// CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE is a validator-local copy of the GLOBAL-scope
// ~/.trackfw/scripts/trackfw-credential-guard.sh template composed in npm/src/generators/hooks.js
// (GLOBAL_CREDENTIAL_GUARD_SCRIPT = CG_HEADER + CG_DETECTION_CORE + CG_GLOBAL_TAIL). This is a
// DIFFERENT template than CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project-scope variant): the
// global variant omits the project-guard block ("no-op outside a trackfw.yaml project") and
// defaults credential_guard.mode to "block" instead of "warn" — mirrors Go's
// credentialGuardGlobalScriptReference
// (internal/validator/validator_credential_guard_global_reference.go). Comparing the global
// on-disk script against CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project template) would be a
// guaranteed false positive for every user with the global harness installed.
const CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE = `#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \\" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\\$\\{?[A-Za-z_][A-Za-z0-9_]*\\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\\$\\{?([A-Za-z_][A-Za-z0-9_]*)\\}?$/\\1/')
    pattern="*\${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=\${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\\([^"]*\\)".*/\\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="\${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="block"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\\000-\\037' | sed 's/\\\\/\\\\\\\\/g; s/"/\\\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\\n' \\
  "$MSG_ESC" \\
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
`

// globalGuardConfigFile associates a GLOBAL (per-CLI, $HOME-rooted) hook/settings config file with
// the CLI that consumes it, for the global-scope guard checks below. Distinct from
// CREDENTIAL_GUARD_HOOK_FILES, whose relPath is rooted at the PROJECT root, not $HOME.
//
// GLOBAL_GUARD_CONFIG_FILES is the closed list of GLOBAL hook/settings files `trackfw update
// harness` can write a guard entry into — the global-scope counterpart of
// CREDENTIAL_GUARD_HOOK_FILES. Paths and CLI labels ported from Go's globalGuardConfigFiles
// (internal/validator/validator_git_branch_guard.go) — note this list DIFFERS from
// CREDENTIAL_GUARD_HOOK_FILES for Copilot and Kiro (global scope uses .copilot/settings.json and
// .kiro/hooks/trackfw-credential-guard.json, not the project-scope attention-hook paths).
// requiresCommandType (ROADMAP-2026-08-17 ML-4B) mirrors CREDENTIAL_GUARD_HOOK_FILES'
// requiresCommandType above — true for every CLI except Cursor.
const GLOBAL_GUARD_CONFIG_FILES = [
  { relPath: '.claude/settings.json', cli: 'Claude Code', requiresCommandType: true },
  { relPath: '.codex/hooks.json', cli: 'Codex CLI', requiresCommandType: true },
  { relPath: '.gemini/settings.json', cli: 'Gemini CLI', requiresCommandType: true },
  { relPath: '.cursor/hooks.json', cli: 'Cursor', requiresCommandType: false },
  { relPath: '.copilot/settings.json', cli: 'GitHub Copilot CLI', requiresCommandType: true },
  { relPath: '.kiro/hooks/trackfw-credential-guard.json', cli: 'Kiro', requiresCommandType: true },
]

// globalGuardConfigPath resolves the actual on-disk path (relative to $HOME) that
// validateGuardGlobalHookResolvable must read for a given (GLOBAL_GUARD_CONFIG_FILES entry,
// scriptMarker) pair. Port of Go's globalGuardConfigPath (internal/validator/
// validator_git_branch_guard.go) — see that function's doc comment for the full rationale
// (5 CLIs share one merge-based file across both guards; Kiro is the sole exception, with a
// dedicated file per guard because its writer rewrites the whole document wholesale).
//
// ROADMAP-2026-08-17 ML-3B: before this function existed, GLOBAL_GUARD_CONFIG_FILES only ever
// pointed Kiro at trackfw-credential-guard.json for BOTH guards, so git_branch_guard_hook_resolvable
// never inspected ~/.kiro/hooks/trackfw-git-branch-guard.json at all.
function globalGuardConfigPath(gf, scriptMarker) {
  if (gf.cli === 'Kiro' && scriptMarker === GIT_BRANCH_GUARD_SCRIPT_MARKER) {
    return '.kiro/hooks/trackfw-git-branch-guard.json'
  }
  return gf.relPath
}

// validateGuardGlobalHookResolvable is the GLOBAL-scope counterpart of
// validateGuardHookResolvable: for each of the 6 GLOBAL_GUARD_CONFIG_FILES that exists AND
// references scriptMarker, verifies the referenced script exists and is executable. Port of Go's
// validateGuardGlobalHookResolvable (internal/validator/validator_git_branch_guard.go) — see that
// function's doc comment for the full trigger-condition and fail-open rationale.
//
// Global entries are written by npm/src/generators (harnessCredentialGuardTarget*-equivalent code
// in agentfiles-equivalent generators) as fully resolved absolute paths, never a placeholder like
// $CLAUDE_PROJECT_DIR — so, unlike the project-scope resolveCredentialGuardHookPath, no
// prefix-stripping is needed here: any matched command that is NOT already an absolute path is
// skipped (never treated as a violation).
//
// Fail-open: unresolvable $HOME, unreadable file, or invalid JSON all skip that file in silence —
// same contract validateGuardHookResolvable already has for project-scope files.
function validateGuardGlobalHookResolvable(scriptMarker) {
  const home = homedir()
  if (!home) return []

  const msgs = []
  for (const gf of GLOBAL_GUARD_CONFIG_FILES) {
    const relPath = globalGuardConfigPath(gf, scriptMarker)
    const fullPath = path.join(home, relPath)
    let content
    try {
      content = fs.readFileSync(fullPath, 'utf8')
    } catch (e) {
      if (e.code === 'ENOENT') continue
      continue
    }

    let parsed
    try {
      parsed = JSON.parse(content)
    } catch (_) {
      continue
    }

    const commands = []
    collectCommandsWithMarker(parsed, scriptMarker, commands)

    const seen = new Set()
    for (const m of commands) {
      const seenKey = `${m.raw} ${m.typeIsCommand}`
      if (seen.has(seenKey)) continue
      seen.add(seenKey)

      // ADR-2026-09-04-caminho-posix-ancorado-...: pathIsAnchoredForHookConfig classifica por
      // ancoragem (POSIX "/", letra de unidade, UNC), NÃO por path.isAbsolute — que no Windows
      // devolve false para "/opt/foo/guard.sh" e faria este `continue` pular a entrada inteira.
      if (!pathIsAnchoredForHookConfig(m.raw)) continue

      // ROADMAP-2026-08-17 ML-4B: reproduced by hades-tf (ML-4A barrier) — a global config entry
      // with the correct absolute command but missing/wrong "type" (hand-edited, an older trackfw
      // version, another tool's merge) is silently never executed by the CLI, even though the
      // script itself exists and is fine. Reported instead of the exists/executable checks below,
      // which assume the entry is structurally valid.
      if (gf.requiresCommandType && !m.typeIsCommand) {
        msgs.push(`~/${relPath} (${gf.cli}, global scope) references ${scriptMarker} resolved to "${m.raw}", but the hook entry is missing "type":"command" (or has an invalid type) — ${gf.cli} will silently never execute it; run \`trackfw update harness\` to regenerate it`)
        continue
      }

      let stat = null
      try {
        stat = fs.statSync(m.raw)
      } catch (_) {
        stat = null
      }

      if (!stat) {
        msgs.push(`~/${relPath} (${gf.cli}, global scope) references ${scriptMarker} resolved to "${m.raw}", but the script does not exist — run \`trackfw update harness\` to regenerate it`)
      } else if (_platform !== 'win32' && (stat.mode & 0o111) === 0) {
        msgs.push(`~/${relPath} (${gf.cli}, global scope) references ${scriptMarker} resolved to "${m.raw}", but the script is not executable — run \`trackfw update harness\` to regenerate it`)
      }
    }
  }

  return msgs
}

// validateGuardGlobalScriptIntegrity is the GLOBAL-scope counterpart of
// validateCredentialGuardScriptIntegrity/validateGitBranchGuardScriptIntegrity: verifies the
// content of ~/.trackfw/scripts/<scriptFileName> (the fixed location the global generators write
// to) against referenceContent byte-for-byte. Port of Go's validateGuardGlobalScriptIntegrity
// (internal/validator/validator_git_branch_guard.go).
//
// ROADMAP-2026-08-17 (guard global cabeado com no-op / integridade independente de fiação), ML-3A:
// deliberately triggers on ARTIFACT EXISTENCE, not on any GLOBAL_GUARD_CONFIG_FILES entry
// referencing scriptMarker — the old per-config trigger meant a script trackfw itself wrote (via
// `trackfw update harness`) but that no config yet pointed at could rot indefinitely with
// `validate` green. "If trackfw wrote the script, trackfw verifies the script" — existence is the
// only precondition; wiring is irrelevant to whether the artifact itself has drifted.
//
// Fail-open on absence: a script trackfw never wrote (user never ran `update harness`) is not an
// error.
//
// Single evaluation per script (not per referencing config): caps the message count at 1
// regardless of how many (or how few — including zero) configs reference the same on-disk path —
// this is what prevents double-reporting now that git-branch-guard has both global wiring (Wave 2)
// and a global artifact.
//
// scriptMarker doubles as the script's own filename (trackfw-credential-guard.sh /
// trackfw-git-branch-guard.sh) in both call sites below — same equivalence the Go port relies on.
function validateGuardGlobalScriptIntegrity(scriptFileName, referenceContent) {
  const home = homedir()
  if (!home) return []

  const scriptPath = path.join(home, '.trackfw', 'scripts', scriptFileName)
  let content
  try {
    content = fs.readFileSync(scriptPath, 'utf8')
  } catch (_) {
    // Not installed is not a violation — same contract as every other *_script_integrity check
    // (project-scope and global) in this file.
    return []
  }

  if (content === referenceContent) return []

  return [`${scriptPath} (global scope) content diverges from the template this version of trackfw generates — if you did not edit this file by hand, run \`trackfw update harness\` to regenerate it`]
}

// validateCredentialGuardGlobalHookResolvable / validateCredentialGuardGlobalScriptIntegrity /
// validateGitBranchGuardGlobalHookResolvable / validateGitBranchGuardGlobalScriptIntegrity are the
// 4 thin wrappers wired in validateUnfiltered below — each folds its messages into the SAME rule
// name as its project-scope counterpart (credential_guard_hook_resolvable,
// credential_guard_script_integrity, git_branch_guard_hook_resolvable,
// git_branch_guard_script_integrity respectively), so no new rules: entries in trackfw.yaml are
// needed. Port of the 4 equivalent wrappers in
// internal/validator/validator_git_branch_guard.go.
function validateCredentialGuardGlobalHookResolvable() {
  return validateGuardGlobalHookResolvable(CREDENTIAL_GUARD_SCRIPT_MARKER)
}

function validateCredentialGuardGlobalScriptIntegrity() {
  return validateGuardGlobalScriptIntegrity(CREDENTIAL_GUARD_SCRIPT_MARKER, CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE)
}

function validateGitBranchGuardGlobalHookResolvable() {
  return validateGuardGlobalHookResolvable(GIT_BRANCH_GUARD_SCRIPT_MARKER)
}

function validateGitBranchGuardGlobalScriptIntegrity() {
  return validateGuardGlobalScriptIntegrity(GIT_BRANCH_GUARD_SCRIPT_MARKER, GIT_BRANCH_GUARD_SCRIPT_REFERENCE)
}
// NOTE: CREDENTIAL_GUARD_SCRIPT_MARKER === 'trackfw-credential-guard.sh' and
// GIT_BRANCH_GUARD_SCRIPT_MARKER === 'trackfw-git-branch-guard.sh' — both markers are already the
// literal script filenames, so they double as scriptFileName above with no extra constant needed
// (same reuse the Go port relies on).

// CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES mirrors the shell script's own resolution of
// credential_guard.mode (`grep -A 5 '^credential_guard:'`): the mode key is found on the matched
// line or within the 5 lines following it.
const CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES = 5

// extractCredentialGuardMode replica a leitura leve (grep/sed) que o próprio script faz —
// deliberadamente não um parser YAML completo, para que a noção desta regra de "para o que
// credential_guard.mode resolve" bata com o que roda de fato no hook.
function extractCredentialGuardMode(content) {
  const lines = content.split('\n')
  const start = lines.findIndex(l => l.startsWith('credential_guard:'))
  if (start === -1) return { mode: '', ok: false }

  const end = Math.min(start + 1 + CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES, lines.length)
  for (const line of lines.slice(start, end)) {
    const trimmed = line.trim()
    if (!trimmed.includes('mode:')) continue
    let rest = trimmed.startsWith('mode:') ? trimmed.slice('mode:'.length) : trimmed
    rest = rest.trim()
    const hashIdx = rest.indexOf('#')
    if (hashIdx >= 0) rest = rest.slice(0, hashIdx).trim()
    rest = rest.replace(/^["']+|["']+$/g, '')
    return { mode: rest, ok: true }
  }
  return { mode: '', ok: false }
}

// headTrackfwYAML retorna o conteúdo de trackfw.yaml no HEAD do git, resolvido relativo a cwd
// (não necessariamente a raiz do repo — `trackfw validate` pode rodar de um subdiretório). ok é
// false sempre que não há âncora usável: não é worktree git, sem commits, ou trackfw.yaml não
// versionado no HEAD — todos "sem âncora, silêncio", nunca erro.
function headTrackfwYAML(cwd) {
  const root = cwd || process.cwd()
  if (!isGitWorktree(root)) return { content: '', ok: false }
  try {
    gitOutput(root, ['rev-parse', '--verify', 'HEAD'])
  } catch {
    return { content: '', ok: false }
  }
  try {
    const out = gitOutput(root, ['show', 'HEAD:./trackfw.yaml'])
    return { content: out, ok: true }
  } catch {
    return { content: '', ok: false }
  }
}

// validateCredentialGuardModeDowngrade é a regra "credential_guard_mode_downgrade": dispara apenas
// quando credential_guard.mode era explicitamente "block" no HEAD e o trackfw.yaml atual em disco
// não resolve mais para "block" (warn explícito, valor não reconhecido, ou chave/arquivo
// ausente — todos os quais o próprio script resolveria como "warn", o DEFAULT_MODE da variante de
// projeto).
//
// Silenciosa sempre que HEAD não é "block": isso é "sem âncora para detectar downgrade", não
// "nada errado". A ausência da chave em DISCO nunca é tratada como silêncio — é exatamente a via
// que esta regra existe para cobrir.
function validateCredentialGuardModeDowngrade(cwd) {
  const root = cwd || process.cwd()
  const head = headTrackfwYAML(root)
  if (!head.ok) return []

  const headMode = extractCredentialGuardMode(head.content)
  if (headMode.mode !== 'block') return []

  let diskContent
  try {
    diskContent = fs.readFileSync(path.join(root, 'trackfw.yaml'), 'utf8')
  } catch (e) {
    if (e.code === 'ENOENT') return [credentialGuardModeDowngradeMessage()]
    return [inspectionDiagnostic('credential_guard_mode_downgrade', 'trackfw.yaml', e)]
  }

  const diskMode = extractCredentialGuardMode(diskContent)
  if (diskMode.mode === 'block') return []

  return [credentialGuardModeDowngradeMessage()]
}

function credentialGuardModeDowngradeMessage() {
  return 'trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the ' +
    'current file does not resolve to block — if this was intentional, commit the change; ' +
    'otherwise investigate before treating the credential guard as active'
}

// _itemMeta: mapa de message → { rule, file } para enriquecer saída JSON.
// Populado em applyRule e nos pushs diretos do validateUnfiltered.
// Permanece em memória apenas durante a execução de uma chamada validate*.
const _itemMeta = new Map()

// _setMeta registra metadados de rule/file para uma mensagem.
function _setMeta(msg, ruleName) {
  const m = /"([^"]+)"/.exec(msg)
  _itemMeta.set(msg, { rule: ruleName, file: m ? m[1] : '' })
}

// getItemMeta retorna { rule, file } para uma mensagem, ou { rule: '', file: '' } se ausente.
function getItemMeta(msg) {
  return _itemMeta.get(msg) || { rule: '', file: '' }
}

// resetMeta limpa o mapa entre execuções (usado internamente).
function resetMeta() {
  _itemMeta.clear()
}

// RULE_DEFAULTS mapeia regras cujo default NÃO é 'error'.
const RULE_DEFAULTS = {
  note_orphan: 'warning',
  // ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A,
  // ADR-2026-08-12 Emenda 3: the script carries no version marker, so this rule cannot tell
  // legitimate drift (trackfw not updated yet) from tampering — kept a warning, never an error.
  // credential_guard_mode_downgrade is deliberately absent: it falls through to ruleSeverity's
  // 'error' default.
  credential_guard_script_integrity: 'warning',
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A: same rationale as credential_guard_script_integrity above — the script carries no
  // version marker, so this rule cannot tell legitimate drift from tampering.
  // git_branch_guard_hook_resolvable is deliberately absent from this map (falls through to
  // 'error'), mirroring credential_guard_hook_resolvable.
  git_branch_guard_script_integrity: 'warning',
  // ML-4A (REQ-2026-08-29, achado 1 do parecer hades-tf 2026-08-30): namespace oculto/ambíguo (nome
  // iniciado por ".") é sinal de baixo ruído por natureza, não um defeito de configuração como
  // agent_namespace_undeclared ('error'). Nunca 'off' por default — silêncio total é o defeito que
  // esta REQ existe para fechar.
  agent_namespace_hidden: 'warning',
}

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md.
//
// As 3 regras abaixo resolvem severidade de forma DIFERENTE de todas as outras ~38: comparam HEAD
// contra disco e adotam a MAIS ESTRITA das duas, em vez de ler só o disco. Deliberado, não bug —
// sem isso, estas 3 regras podem ser desligadas pela mesma edição NÃO COMMITADA que elas deveriam
// denunciar (`rules: credential_guard_mode_downgrade: off` em trackfw.yaml, nunca commitado). Toda
// outra regra continua passando por diskRuleSeverity, byte-idêntico a antes deste ADR.
const CREDENTIAL_GUARD_ANCHORED_RULES = new Set([
  'credential_guard_hook_resolvable',
  'credential_guard_script_integrity',
  'credential_guard_mode_downgrade',
])

// credentialGuardSeverityRank ordena severidades da menos para a mais estrita, para a comparação
// "mais estrita vence" de credentialGuardRuleSeverity. Qualquer valor fora de 'off'/'warning' só
// significa 'error' na prática — applyRule já trata qualquer valor não reconhecido como violation,
// então este ranking espelha esse mesmo fallback em vez de introduzir um contrato mais rígido.
function credentialGuardSeverityRank(s) {
  if (s === 'off') return 0
  if (s === 'warning') return 1
  return 2
}

// credentialGuardStricterSeverity retorna a mais estrita entre a e b ('error' > 'warning' > 'off').
function credentialGuardStricterSeverity(a, b) {
  return credentialGuardSeverityRank(a) >= credentialGuardSeverityRank(b) ? a : b
}

// credentialGuardDefaultSeverity é o mesmo fallback "RULE_DEFAULTS > error" que diskRuleSeverity
// usa quando trackfw.yaml não tem rules: <name> — extraído para credentialGuardRuleSeverity poder
// aplicá-lo igualmente ao lado HEAD (que não tem equivalente de RULE_DEFAULTS próprio, já que
// config.parseRulesFromContent só devolve o que rules: em si contém).
function credentialGuardDefaultSeverity(name) {
  return RULE_DEFAULTS[name] || 'error'
}

// credentialGuardRuleSeverity resolve a severidade de uma das 3 CREDENTIAL_GUARD_ANCHORED_RULES
// como a MAIS ESTRITA entre HEAD e disco — direcional, não "ignora disco e usa só HEAD" (ver o
// parecer §2 e o ADR — o caso comum, HEAD sem menção à regra, precisa resolver para o default, ou
// seja o valor mais estrito possível, senão o disco venceria de volta silenciosamente sempre).
//
// Sem HEAD (não é git worktree, sem commits, ou trackfw.yaml não versionado no HEAD —
// headTrackfwYAML's 3 casos de "sem âncora"): cai no disco puro, igual a qualquer outra regra.
// ADR ponto de decisão 4: limite aceito, não um bypass acionável por adversário — nenhum desses 3
// casos é alcançável por uma edição não commitada de trackfw.yaml sozinha.
function credentialGuardRuleSeverity(name) {
  const diskSeverity = diskRuleSeverity(name)

  const head = headTrackfwYAML()
  if (!head.ok) return diskSeverity

  const headRules = config.parseRulesFromContent(head.content)
  const headSeverity = headRules[name] || credentialGuardDefaultSeverity(name)

  return credentialGuardStricterSeverity(headSeverity, diskSeverity)
}

// ruleSeverity retorna a severidade configurada para uma regra ('error'|'warning'|'off').
// Prioridade: trackfw.yaml rules: > RULE_DEFAULTS > 'error'.
//
// Para as 3 CREDENTIAL_GUARD_ANCHORED_RULES, delega a credentialGuardRuleSeverity acima — ver o
// comentário logo antes dessa constante para o porquê. Toda outra regra segue para
// diskRuleSeverity, textualmente idêntico ao corpo desta função antes do ADR-2026-08-12.
function ruleSeverity(name) {
  if (CREDENTIAL_GUARD_ANCHORED_RULES.has(name)) return credentialGuardRuleSeverity(name)
  return diskRuleSeverity(name)
}

// diskRuleSeverity é a resolução ordinária, só-disco, usada por toda regra exceto as 3
// CREDENTIAL_GUARD_ANCHORED_RULES: trackfw.yaml rules: (CWD) > RULE_DEFAULTS > 'error'.
function diskRuleSeverity(name) {
  const cfg = config.load()
  if (cfg.rules[name]) return cfg.rules[name]
  if (RULE_DEFAULTS[name]) return RULE_DEFAULTS[name]
  return 'error'
}

// applyRule distribui msgs para violations ou warnings conforme a severidade configurada.
// Se severidade for 'off', descarta silenciosamente.
// Também popula _itemMeta com rule/file para cada mensagem aceita.
function applyRule(ruleName, msgs, violations, warnings) {
  if (!msgs || msgs.length === 0) return
  const severity = ruleSeverity(ruleName)
  if (severity === 'off') return
  if (severity === 'warning') {
    for (const msg of msgs) { _setMeta(msg, ruleName); warnings.push(msg) }
  } else {
    for (const msg of msgs) { _setMeta(msg, ruleName); violations.push(msg) }
  }
}

const BASELINE_FILE = '.trackfw-baseline.json'

// loadBaseline carrega o baseline do arquivo .trackfw-baseline.json.
// Retorna null se o arquivo não existir.
function loadBaseline() {
  try {
    const data = fs.readFileSync(BASELINE_FILE, 'utf8')
    return JSON.parse(data)
  } catch (e) {
    if (e.code === 'ENOENT') return null
    throw new Error(`Erro ao ler baseline: ${e.message}`)
  }
}

// saveBaseline salva snapshot de violations e warnings em .trackfw-baseline.json.
function saveBaseline(violations, warnings) {
  const bf = {
    created: new Date().toISOString(),
    violations,
    warnings,
  }
  fs.writeFileSync(BASELINE_FILE, JSON.stringify(bf, null, 2), 'utf8')
}

function sha256Hex(buf) {
  return crypto.createHash('sha256').update(buf).digest('hex')
}

// THIRDPARTY_ORIGIN é o valor de Claim.origin que marca um artefato do manifest como instalado
// por `third-party install` (ADR-2026-08-15 D11).
const THIRDPARTY_ORIGIN = 'thirdparty'

// validateThirdPartyArtifactHasProvenance implementa a regra "thirdparty_artifact_has_provenance"
// (ADR-2026-08-15 D2) — a detecção real, ancorada em git, por trás do guardrail
// TRACKFW_ORCHESTRATOR_SESSION. NUNCA faz fetch de rede (D6): lê só
// .trackfw/integrations-manifest.json, .trackfw/thirdparty-provenance.json e
// .trackfw/thirdparty-quarantine/<checksum>.json, todos já em disco (e, por convenção deste
// projeto, versionados no repositório).
//
// Duas ramificações, ambas fatais (error — a regra está deliberadamente ausente de RULE_DEFAULTS):
//   1. um artifact do manifest carrega um claim com origin === "thirdparty" mas
//      thirdparty-provenance.json não tem entrada chaveada por aquele destino;
//   2. existe entrada de proveniência, mas seu installed_sha256 não pode ser reconciliado com o que
//      de fato está em disco no destino declarado.
//
// Sobre a ramificação 2 (ADR-2026-08-15 D2-bis, ML-3B) — checksum_sha256 é sha256 dos bytes BRUTOS
// (D6), mas o arquivo instalado é sempre NormalizeThirdPartyContent(raw) — não é a função
// identidade em geral, então comparar checksum_sha256 direto contra sha256(arquivo instalado)
// produz falso-positivo em toda instalação legítima cujo conteúdo bruto não fosse já exatamente
// TrimSpace+newline único. A resolução ML-3A usava o registro de quarentena como ponte entre os
// dois domínios — correta, mas tornava um artefato de ESTÁGIO (.trackfw/thirdparty-quarantine/,
// destinado a ser podado) dependência obrigatória de um gate PERMANENTE. D2-bis resolve isso com
// um segundo campo na entrada de proveniência, installed_sha256 = sha256(bytes NORMALIZADOS),
// calculado no momento do install pelo mesmo código que grava o arquivo (npm/src/commands/thirdparty.js).
// checksum_sha256 permanece intocado, é a âncora de aprovação D8c. A ramificação 2 agora compara
// sha256(arquivo instalado) diretamente contra entry.installed_sha256 — dois domínios já
// normalizados, sem ponte via quarentena. A ausência da quarentena deixou de ser erro desta regra.
function validateThirdPartyArtifactHasProvenance(cwd) {
  const root = cwd || process.cwd()

  const manifestPath = path.join(root, '.trackfw', 'integrations-manifest.json')
  let manifest
  try {
    const raw = fs.readFileSync(manifestPath, 'utf8')
    manifest = JSON.parse(raw)
  } catch (err) {
    if (err.code === 'ENOENT') return []
    throw new Error(`thirdparty_artifact_has_provenance: read ${manifestPath}: ${err.message}`)
  }

  const destinations = []
  for (const [destination, artifact] of Object.entries(manifest.artifacts || {})) {
    const claims = artifact.claims || []
    if (claims.some(claim => claim.origin === THIRDPARTY_ORIGIN)) destinations.push(destination)
  }
  if (destinations.length === 0) return []
  destinations.sort()

  let prov
  try {
    prov = loadProvenance(root)
  } catch (err) {
    throw new Error(`thirdparty_artifact_has_provenance: ${err.message}`)
  }

  const msgs = []
  for (const destination of destinations) {
    // Provenance keys are NOT the manifest's absolute destination —
    // verified empirically against the real install command
    // (npm/src/commands/thirdparty.js): verifyApproval/upsertProvenanceEntry
    // are called with the project-root-relative (or "~/"-prefixed,
    // global-scope) destination string BEFORE IntegrationManager.resolve()
    // joins it against root to produce the absolute manifest key. Every
    // claim reached here came from the PROJECT manifest, so its scope is
    // always "project" (a global-scope claim lives in the home manifest
    // instead, which this rule intentionally never reads). path.relative
    // inverts resolve()'s path.resolve(root, relative) exactly. Mirrors
    // internal/validator/validator_thirdparty_provenance.go.
    //
    // ML-2A (ADR-2026-09-04, D1 categoria 2 — "chave de dicionario ou identificador"):
    // path.relative devolve separador NATIVO, e a chave gravada em
    // thirdparty-provenance.json e sempre "/" (resolveThirdPartySkillDestination monta
    // o destino por concatenacao explicita com "/", nos 3 runtimes). Sem normalizar, no
    // Windows a chave nunca casa e TODO artefato de terceiro e reportado como sem
    // entrada — falso positivo em massa. Go (validator_thirdparty_provenance.go:160) e
    // Python (validator.py:3530) ja normalizavam; o Node era a divergencia (D4).
    //
    // 🔴 So a CHAVE DERIVADA e normalizada. `destination` continua cru e e o que aparece
    // nas mensagens e no manifest; nada aqui altera caminho passado a syscall (ADR D2).
    const provenanceKey = normalizeRefSeparator(path.relative(root, destination))
    const entry = (prov.entries || {})[provenanceKey]
    if (!entry) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" is claimed as a third-party artifact but has no ` +
        'entry in .trackfw/thirdparty-provenance.json — obtain a favorable hades-tf review and record an ' +
        'approved provenance entry for this destination before this can pass validate (D2 branch i)'
      )
      continue
    }

    let installed
    try {
      installed = fs.readFileSync(destination)
    } catch (err) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" is claimed as a third-party artifact with an ` +
        `approved provenance entry, but the destination file could not be read (${err.message})`
      )
      continue
    }

    // entry.installed_sha256 may be `undefined` (key absent — e.g. an
    // approver-authored entry from before `install` ever ran, or a partial
    // install that failed between manager.Install and the installed_sha256
    // upsert). Coerce to '' so the comparison and the message text match
    // Go (zero-value "") and Python (.get(..., "")) byte-for-byte — do NOT
    // interpolate entry.installed_sha256 directly, that renders the
    // JS-only literal "undefined" into the violation message.
    const installedSHA256 = entry.installed_sha256 || ''
    if (sha256Hex(installed) !== installedSHA256) {
      msgs.push(
        `thirdparty_artifact_has_provenance: "${destination}" — installed content does not match ` +
        `installed_sha256 ${installedSHA256} recorded in .trackfw/thirdparty-provenance.json — the ` +
        'artifact was modified after approval or installed outside the fetch/install flow (D2 branch ii, D2-bis)'
      )
    }
  }

  return msgs
}

// validateUnfiltered executa todas as validações e retorna { violations, warnings } sem ratchet.
async function validateUnfiltered() {
  resetMeta()
  const wipLimitResult = validateWIPLimit()
  const violations = []
  const warnings = []

  // Regras com severidade configurável via applyRule (popula _itemMeta automaticamente)
  applyRule('wip_has_req',          validateWIPHasREQ(),                   violations, warnings)
  applyRule('wip_acceptance',       validateWIPHasAcceptanceCriteria(),    violations, warnings)
  applyRule('wip_limit',            wipLimitResult.violations,             violations, warnings)
  applyRule('adr_orphan',           validateADRsAreReferenced(),           violations, warnings)
  applyRule('stale_wip',            validateStaleWIP(),                    violations, warnings)
  applyRule('ref_targets_exist',    validateRefTargetsExist(),             violations, warnings)
  for (const msg of validateREQRoadmapLifecycle()) { _setMeta(msg, 'req_roadmap_lifecycle'); warnings.push(msg) }
  applyRule('folder_status',        validateFolderStatusCoherence(),       violations, warnings)
  applyRule('filename_uniqueness',  validateFilenameUniqueness(),          violations, warnings)
  applyRule('branch_has_wip_roadmap', validateBranchHasWIPRoadmap(),      violations, warnings)
  applyRule('blocked_by_draft_adr', validateREQsNotBlockedByDraftADRs(),  violations, warnings)
  applyRule('adr_accepted_when_req_done', validateADRAcceptedWhenREQDone(), violations, warnings)
  applyRule('note_orphan',           validateNoteOrphan(),                 violations, warnings)
  applyRule('credential_guard_hook_resolvable', validateCredentialGuardHookResolvable().concat(validateCredentialGuardGlobalHookResolvable()), violations, warnings)
  applyRule('credential_guard_script_integrity', validateCredentialGuardScriptIntegrity().concat(validateCredentialGuardGlobalScriptIntegrity()), violations, warnings)
  applyRule('credential_guard_mode_downgrade', validateCredentialGuardModeDowngrade(), violations, warnings)
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A — port de internal/validator/validator.go's wiring das 4 regras/wrappers em
  // validator_git_branch_guard.go.
  applyRule('git_branch_guard_hook_resolvable', validateGitBranchGuardHookResolvable().concat(validateGitBranchGuardGlobalHookResolvable()), violations, warnings)
  applyRule('git_branch_guard_script_integrity', validateGitBranchGuardScriptIntegrity().concat(validateGitBranchGuardGlobalScriptIntegrity()), violations, warnings)

  // ADR-2026-08-15-gate-de-duas-fases-..., ML-3A (D2): detecção ancorada em git por trás do
  // guardrail TRACKFW_ORCHESTRATOR_SESSION.
  applyRule('thirdparty_artifact_has_provenance', validateThirdPartyArtifactHasProvenance(), violations, warnings)

  // Regras configuráveis via applyRule (popula _itemMeta automaticamente)
  applyRule('req_has_adr',          validateREQsHaveADR(),          violations, warnings)
  applyRule('blocked_has_req',      validateBlockedHasREQ(),        violations, warnings)
  applyRule('req_has_roadmap',      validateREQsHaveRoadmap(),      violations, warnings)

  // Regra direta (sem configuração de severidade): violation sempre
  for (const msg of validateFrontmatterPresence())  { _setMeta(msg, 'frontmatter_presence'); violations.push(msg) }

  // Validação de existência dos adr_dirs (retorna violations se strictCiPaths, senão warnings)
  const adrDirsExistResult = validateADRDirsExist()
  for (const msg of adrDirsExistResult.violations) { _setMeta(msg, 'adr_dir_exists'); violations.push(msg) }
  for (const msg of adrDirsExistResult.warnings) { _setMeta(msg, 'adr_dir_exists'); warnings.push(msg) }

  // warnings diretos do WIP limit (não configuráveis)
  for (const msg of wipLimitResult.warnings) { _setMeta(msg, 'wip_limit'); warnings.push(msg) }

  // Verificação bidirecional de trace ID (somente se traceIdField configurado)
  const cfg = config.load()

  // ML-2A (REQ-2026-08-29): namespace de agente em disco e não declarado em agents: — violação, não
  // aviso (ver comentário em validateAgentNamespaceUndeclared).
  applyRule('agent_namespace_undeclared', validateAgentNamespaceUndeclared(cfg), violations, warnings)

  // ML-4A (achado 1, hades-tf 2026-08-30): contraponto de baixo ruído para nomes ocultos/ambíguos
  // (iniciados por ".") — aviso, nunca silêncio total, nunca erro (RULE_DEFAULTS abaixo).
  applyRule('agent_namespace_hidden', hiddenNamespaceWarnings(cfg), violations, warnings)

  if (cfg.traceIdField) {
    for (const msg of checkTraceIds(cfg.reqDir, cfg.roadmapDir, cfg.traceIdField)) {
      // O prefixo da mensagem traceid já carrega o nome da regra (ex: "traceid_orphan_roadmap: ...")
      const ruleName = msg.split(':')[0].trim()
      _setMeta(msg, ruleName)
      violations.push(msg)
    }
  }

  return { violations, warnings }
}

// validate executa todas as validações, aplica ratchet (baseline) e modo lenient.
// Retorna { violations, warnings }.
//
// ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-
// e-disco: carve-out do baseline — violations/warnings de uma das 3
// CREDENTIAL_GUARD_ANCHORED_RULES NUNCA são toleradas via .trackfw-baseline.json, não importa o
// que o arquivo contenha para elas. Mecanismo DIFERENTE do HEAD-vs-disco em
// credentialGuardRuleSeverity: .trackfw-baseline.json é .gitignore'd DE PROPÓSITO ("baseline local
// de violations toleradas (nao versionado)"), então não há HEAD desse arquivo para comparar —
// "exigir commit" simplesmente não se aplica a um arquivo que o projeto decidiu nunca versionar. A
// única forma de fechar esse canal é excluir estas 3 regras da elegibilidade de ratchet, por nome,
// independente do conteúdo da mensagem — daí a checagem via getItemMeta(msg).rule abaixo.
async function validate() {
  const result = await validateUnfiltered()
  let { violations, warnings } = result

  // Ratchet: filtrar violations e warnings que já estavam no baseline
  const baseline = loadBaseline()
  if (baseline) {
    const baselineSet = new Set(baseline.violations || [])
    violations = violations.filter(v => !baselineSet.has(v) || CREDENTIAL_GUARD_ANCHORED_RULES.has(getItemMeta(v).rule))
    const baselineWarnSet = new Set(baseline.warnings || [])
    warnings = warnings.filter(w => !baselineWarnSet.has(w) || CREDENTIAL_GUARD_ANCHORED_RULES.has(getItemMeta(w).rule))
  }

  // Modo lenient: mover violations para warnings, exit code 0
  if (isLenient()) {
    warnings = [...warnings, ...violations]
    violations = []
  }

  return { violations, warnings }
}

// ROADMAP_STATES enumera os 6 estados de roadmap na ordem exibida pelo bloco Inventory.
const ROADMAP_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']

// buildInventorySection monta o bloco "📊 Inventory" — contagem agregada de ADRs, REQs
// (discriminadas por status real via reqStatusEquals) e roadmaps (pelos 6 estados, incluindo
// "analyzing", historicamente omitido). Namespacing-agnóstico: agrega através de
// resolveStateDirs()/resolveReqFiles(), que já resolvem flat vs by_agent.
function buildInventorySection(cfg) {
  let adrCount = 0
  for (const adrDir of cfg.adrDirs || []) {
    adrCount += walkDirMdWithPathsForRule('status_inventory', adrDir, []).length
  }

  const reqFiles = resolveReqFiles(cfg)
  let reqOpen = 0
  let reqDone = 0
  let reqClosed = 0
  let reqOther = 0
  for (const filePath of reqFiles) {
    let content
    try {
      content = fs.readFileSync(filePath, 'utf8')
    } catch (_) {
      continue
    }
    if (reqStatusEquals(content, 'open')) reqOpen++
    else if (reqStatusEquals(content, 'done')) reqDone++
    else if (reqStatusEquals(content, 'closed')) reqClosed++
    else reqOther++
  }

  const roadmapCounts = {}
  let roadmapTotal = 0
  for (const state of ROADMAP_STATES) {
    let count = 0
    for (const dir of resolveStateDirs(cfg, state)) {
      count += listDir(dir).length
    }
    roadmapCounts[state] = count
    roadmapTotal += count
  }

  let section = '\n📊 Inventory\n'
  section += `   ${'ADRs'.padEnd(12)}${adrCount}\n`
  // Toda grafia fora de open/done/closed cai em reqOther. Sem este bucket o total
  // bate e a quebra some com a diferenca EM SILENCIO: um acervo com
  // approved/backlog/abandoned mostra "53 (8 Open · 36 Done · 0 Closed)" sem
  // indicar que 9 existem e nao estao em lugar nenhum da conta.
  // O Python ja fazia isto (pypi/trackfw/commands/status.py:58,199).
  let reqDetail = `${reqOpen} Open · ${reqDone} Done · ${reqClosed} Closed`
  if (reqOther > 0) reqDetail += ` · ${reqOther} Other`
  section += `   ${'REQs'.padEnd(12)}${reqFiles.length}  (${reqDetail})\n`
  section += `   ${'Roadmaps'.padEnd(12)}${roadmapTotal}\n`
  section += `     backlog ${roadmapCounts.backlog} · analyzing ${roadmapCounts.analyzing} · wip ${roadmapCounts.wip}\n`
  section += `     blocked ${roadmapCounts.blocked} · done ${roadmapCounts.done} · abandoned ${roadmapCounts.abandoned}\n`
  return section
}

// getStatus retorna string formatada com o status de governança do projeto
async function getStatus() {
  const cfg = config.load()
  let out = '── trackfw status ──────────────────────\n'
  out += buildInventorySection(cfg)

  if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agents = resolveAgentNamespaces(cfg, cfg.roadmapDir)
    out += '\n⚙ WIP by Agent\n'
    for (const agent of agents) {
      const wip = listDir(cfg.roadmapDir + '/' + agent + '/wip')
      if (wip.length > 0) {
        out += `  [${agent}] WIP (${wip.length})\n`
        wip.forEach(f => { out += `    ${f}\n` })
      }
    }
  } else {
    const wip = listDir(cfg.roadmapDir + '/wip')
    const blocked = listDir(cfg.roadmapDir + '/blocked')
    const done = listDir(cfg.roadmapDir + '/done')

    out += `\n🔄 WIP (${wip.length})\n`
    for (const f of wip) out += `   ${f}\n`

    const wipCfg = wipConfigFrom(cfg)
    if (wipCfg.bySquad && wip.length > 0) {
      const bySquad = {}
      for (const f of wip) {
        let squad = parseSquadFromFrontmatter(path.join(cfg.roadmapDir, 'wip', f))
        if (!squad) squad = '(no squad)'
        bySquad[squad] = (bySquad[squad] || 0) + 1
      }
      out += `\n⚙ WIP by Squad (limit: ${wipCfg.limit} per squad)\n`
      for (const [squad, count] of Object.entries(bySquad)) {
        const status = count > wipCfg.limit ? '⚠' : '✓'
        const noun = count === 1 ? 'roadmap' : 'roadmaps'
        out += `   ${(squad + ':').padEnd(20)} ${count} ${noun}  ${status}\n`
      }
    }

    out += `\n❌ Blocked (${blocked.length})\n`
    for (const f of blocked) out += `   ${f}\n`

    const staleWIPs = validateStaleWIP()
    if (staleWIPs.length > 0) {
      out += `\n⚠  Stale WIP (${staleWIPs.length})\n`
      for (const w of staleWIPs) out += `   ${w}\n`
    }

    // Seção: REQs bloqueadas por ADRs não aceitos (Draft ou Proposed). O status exibido por
    // ADR é resolvido via adrNotAcceptedStatusForRule (helper canônico) em vez de hardcodar
    // "Draft" — blockedREQs() cobre ambos os status desde que delega em adrIsDraft/
    // adrDraftStatusForRule, e um rótulo fixo "(Draft)" mentiria para um ADR Proposed.
    const blockedByDraft = blockedREQs()
    const blockedKeys = Object.keys(blockedByDraft)
    if (blockedKeys.length > 0) {
      out += `\n⏳ REQs blocked by not-accepted ADRs (${blockedKeys.length})\n`
      for (const reqFile of blockedKeys) {
        out += `   ${reqFile}\n`
        for (const adr of blockedByDraft[reqFile]) {
          const { status } = adrNotAcceptedStatusForRule('blocked_by_draft_adr', adr, null)
          out += `     → ${adr} (${status})\n`
        }
      }
    }

    out += `\n✅ Done (last 5)\n`
    const last5 = done.length > 5 ? done.slice(done.length - 5) : done
    for (const f of last5) out += `   ${f}\n`
  }

  out += '\n────────────────────────────────────────\n'
  return out
}

module.exports = {
  _setPlatformForTest,
  validate,
  validateUnfiltered,
  loadBaseline,
  saveBaseline,
  getStatus,
  isLenient,
  lenientUntilDate,
  // exportadas para testes unitários
  validateWIPHasREQ,
  validateREQsHaveADR,
  validateBlockedHasREQ,
  validateREQsHaveRoadmap,
  validateADRsAreReferenced,
  validateWIPHasAcceptanceCriteria,
  validateWIPLimit,
  validateSingleWIP,
  validateStaleWIP,
  setStaleWipNowForTests,
  validateREQsNotBlockedByDraftADRs,
  parseBlockedADRs,
  adrIsDraft,
  listDir,
  tryListDir,
  resolveReqFiles,
  reqWriteDir,
  resolveStateDirs,
  resolveWIPDirs,
  resolveDoneDirs,
  resolveAgentNamespaces,
  parseSquadFromFrontmatter,
  validateFrontmatterPresence,
  // novas funções ML-1B
  walkDirMd,
  findAdrFile,
  gitLastModifiedTime,
  extractRefPath,
  validateRefTargetsExist,
  validateREQRoadmapLifecycle,
  validateFolderStatusCoherence,
  validateFilenameUniqueness,
  validateBranchHasWIPRoadmap,
  // novas funções — trackfw branch new (extraídas do gate branch_has_wip_roadmap)
  branchSlugMatchesRoadmap,
  branchGovernanceOrientation,
  branchNoMatchingRoadmapMessage,
  normalizeBranchSlug,
  // novas funções ML-2B
  contentHasMarker,
  ruleSeverity,
  applyRule,
  // novas funções ML-1B (v2.5.1)
  getItemMeta,
  resetMeta,
  // novas funções ML-2B (governança global)
  isInsideDir,
  walkDirMdWithPaths,
  validateADRDirsExist,
  // novas funções ML-1B (2026-08-01 — adr_accepted_when_req_done)
  extractAdrHeaderStatus,
  adrNotAcceptedStatusForRule,
  reqStatusEquals,
  validateADRAcceptedWhenREQDone,
  // ML-1D (2026-08-01 — reconciliação de paridade: frontmatter-first)
  extractFrontmatterField,
  resolveAdrStatus,
  // ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A
  resolveCredentialGuardHookPath,
  // ADR-2026-09-04 (ML-3A): predicado de ancoragem invariante por GOOS
  pathIsAnchoredForHookConfig,
  // ADR-2026-08-22 (ML-1A): classificação por ancoragem
  HOOK_ANCHORAGE_CLASS_ANCHORED,
  HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT,
  HOOK_ANCHORAGE_CLASS_UNDECIDABLE,
  stripOuterQuotesForClassify,
  hookValueWasQuoted,
  classifyHookAnchorage,
  cwdDependentReason,
  collectCredentialGuardCommands,
  validateCredentialGuardHookResolvable,
  // ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A
  validateCredentialGuardScriptIntegrity,
  validateCredentialGuardModeDowngrade,
  extractCredentialGuardMode,
  CREDENTIAL_GUARD_SCRIPT_REFERENCE,
  // ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
  // ML-2A
  collectCommandsWithMarker,
  validateGuardHookResolvable,
  validateGitBranchGuardHookResolvable,
  validateGitBranchGuardScriptIntegrity,
  GIT_BRANCH_GUARD_SCRIPT_REFERENCE,
  CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE,
  GLOBAL_GUARD_CONFIG_FILES,
  validateGuardGlobalHookResolvable,
  validateGuardGlobalScriptIntegrity,
  // ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas, ML-3A
  validateThirdPartyArtifactHasProvenance,
  validateCredentialGuardGlobalHookResolvable,
  validateCredentialGuardGlobalScriptIntegrity,
  validateGitBranchGuardGlobalHookResolvable,
  validateGitBranchGuardGlobalScriptIntegrity,
}
