---
status: wip
date: 2026-08-29
req: "docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: A lista `agents:` complementa o disco, e namespace não declarado vira violação

> Created: 2026-08-29 | Status: wip

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Context

REQ: `REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`
ADR: `ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md`

Em `roadmap_namespacing: by_agent`, a lista `agents:` **substitui** o disco. Diretório não declarado
fica invisível — e o `validate` reporta `No violations found` sobre o que nunca enumerou. No projeto
cmdb, `docs/roadmaps/zeus/` e `docs/requisições/zeus/` estavam fora de tudo.

A regra está duplicada em **6 funções** só no `validator.go` e o modo aparece em 9 arquivos no Go,
11 sítios no Node e 24 no Python.

## Acceptance Criteria

Consolidado — AC1 a AC11 da REQ.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça e enumeração da superfície
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap. Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração — vá pelo CONSUMIDOR, não pelo padrão de texto.** Liste **todos** os
   pontos, nos 3 runtimes, que resolvem diretório de estado ou de REQ em modo `by_agent`. Já
   conhecidos em Go: `validator.go` (`validateWIPLimit:221`, `GetStatus:912`, `resolveStateDirs:1020`,
   `resolveREQFiles:1071`, `validateFolderStatusCoherence:1959`, `validateFilenameUniqueness:2036`),
   mais `validator_traceid.go`, `commands/barrier.go`, `generators/roadmap.go`, `generators/req.go`,
   `generators/context.go`, `serve/api_board.go`, `serve/api_metrics.go`. Confirme e complete para
   Node e Python.
   > **A Wave 0 da REQ do pin declarou enumeração fechada sobre um padrão de busca que eu dei, e
   > perdeu metade da superfície.** Não repita: derive os pontos de quem **consome** o caminho, e
   > justifique por que a lista fecha.
2. **Modelo de ameaça.** A união amplia o que a ferramenta lê. Quem se aproveita disso? Cubra no
   mínimo: diretório com nome que escapa do `roadmap_dir` (`..`, caminho absoluto, symlink apontando
   para fora — lembre que ontem achamos escrita fora do projeto por symlink); nome de diretório com
   separador ou caractere de controle; diretório oculto (`.git`, `.DS_Store`, `node_modules`);
   e o caso de `roadmap_dir` e `req_dir` apontando para o mesmo lugar.
3. **Alvos de falsificação nas duas direções.** O que quebra se regredir (volta a substituir o disco)
   **e** se regredir para o lado oposto (passa a enumerar qualquer coisa, inclusive o que não é
   namespace; ou a violação vira tão barulhenta que o usuário desliga a regra — ver `ADR-2026-08-17`).
4. **Residual declarado.** O que o desenho aceita não cobrir.
**Critérios de aceite:**
- [ ] As quatro seções com evidência medida, não asserção
- [ ] A enumeração cobre os 3 runtimes e justifica o fechamento
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/adr/ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md
```

#### Resultado do ML-0A (hades-tf, 2026-08-29)

**Método.** Enumerei por consumidor, não por padrão de texto: para cada runtime, localizei todo ponto
que referencia `RoadmapNamespacing`/`roadmapNamespacing`/`roadmap_namespacing` OU `cfg.Agents`/`.agents`
(Go/Node/Python), depois abri cada um para confirmar se é (a) um ponto de **leitura/enumeração** com o
padrão "se `agents:` vazio, cai no disco; senão só o declarado" (o defeito), (b) já **seguro** (lê disco
incondicionalmente, sem checar `agents:`), (c) um **escritor de config** (`discover`, popula `agents:` a
partir do disco — não pode esconder nada, é a fonte, não o consumidor) ou (d) **falso positivo** — um
`Agents` de outro domínio (catálogo de agentes de IA/identidade, não namespace de roadmap). Dois vetores
do modelo de ameaça (symlink e paridade Node/Python de `serve`) foram **reproduzidos ao vivo**, não
apenas inferidos — ver evidência abaixo.

##### 1. Enumeração da superfície

**Go — fechada por função+linha (14 pontos com o defeito, 3 já seguros, 2 escritores, 3 falsos positivos):**

Com o defeito (agents: substitui disco quando não-vazio):
1. `internal/validator/validator.go:220` `validateWIPLimit`
2. `internal/validator/validator.go:911` `GetStatus`
3. `internal/validator/validator.go:1019` `resolveStateDirs`
4. `internal/validator/validator.go:1070` `resolveREQFiles`
5. `internal/validator/validator.go:1958` `validateFolderStatusCoherence`
6. `internal/validator/validator.go:2035` `validateFilenameUniqueness`
7. `internal/validator/validator_traceid.go:91` `collectTraceIdEntriesByAgent` (compartilhada por REQ e roadmap — os 2 call-sites em `validateTraceId` não duplicam)
8. `internal/commands/barrier.go:125` `resolveBarrierRoadmap` — **duplica em vez de reusar `validator.resolveStateDirs`** (ver achado arquitetural abaixo)
9. `internal/generators/roadmap.go:484` `findRoadmap`
10. `internal/generators/roadmap.go:591` `ListRoadmaps`
11. `internal/generators/roadmap.go:669` `scanREQFiles`
12. `internal/generators/req.go:144` `listREQFiles` (ramo by_agent)
13. `internal/generators/context.go:59` (seção REQ)
14. `internal/generators/context.go:108` (seção roadmap)

Já seguros hoje (leem disco incondicionalmente via `os.ReadDir`/`filepath.Glob`, nunca checam `cfg.Agents`
— não têm o defeito, mas Wave 1 deve migrá-los para o resolvedor canônico por consistência de AC9, não
por urgência):
- `internal/serve/api_board.go:46` `boardHandler`
- `internal/serve/api_metrics.go:208` `countStateDistribution`
- `internal/generators/roadmap.go:551` `ShowRoadmap` (usa `filepath.Glob(roadmapDir/*/*/*name*.md)`)

Escritores de config, não consumidores (populam `Agents`/`agents:` a partir do disco; não podem
esconder nada porque são a fonte):
- `internal/discover/discover.go` (`discover.Scan`)
- `internal/config/config.go:405,409` (parse do YAML)

Falsos positivos confirmados (mesmo nome, domínio diferente — catálogo de identidade/integrações de
agentes de IA, não namespace de roadmap):
- `internal/commands/agents_models.go`, `internal/commands/identity_wizard.go`,
  `internal/commands/integrations_flags.go`

**Node — fechada por função+linha (15 pontos com o defeito, 1 já seguro por delegação):**

1. `npm/src/validator/index.js:155` `resolveReqFiles`
2. `npm/src/validator/index.js:192` `resolveStateDirs`
3. `npm/src/validator/index.js:528` `validateWIPLimit` — **duplica em vez de chamar `resolveStateDirs`**
4. `npm/src/validator/index.js:934` `validateFolderStatusCoherence` — idem
5. `npm/src/validator/index.js:994` `validateFilenameUniqueness` — idem
6. `npm/src/validator/index.js:3131` `buildInventorySection` (alimenta `status`/`context`)
7. `npm/src/commands/context.js:107` (seção REQ)
8. `npm/src/commands/context.js:128` (seção roadmap)
9. `npm/src/generators/req.js:45`
10. `npm/src/generators/roadmap.js:92` `listRoadmaps`
11. `npm/src/generators/roadmap.js:670` `findRoadmapMatches` (alimenta `showRoadmap` **e** `move`)
12. `npm/src/serve/api_board.js:70`
13. `npm/src/serve/api_chain.js:104` (REQ)
14. `npm/src/serve/api_chain.js:124` (Roadmap)
15. `npm/src/serve/api_metrics.js:36`

Já seguro por delegação (não tem lógica própria — herda a correção de `resolveStateDirs` automaticamente):
- `npm/src/commands/barrier.js:resolveRoadmapFile` — chama `validator.resolveWIPDirs`/`resolveDoneDirs`

> **Achado de sweep, não de segurança**: `npm/src/validator/index.js` — o arquivo que concentra os 6
> pontos mais graves acima — é classificado como `data` por `file(1)` (bytes não-ASCII em algum ponto)
> e é **pulado em silêncio** por `grep -rln "by_agent" npm/src/` sem `-a`. O primeiro sweep ingênuo
> fechou em 10 arquivos, plausivelmente completo, sem o arquivo mais importante. Nota de vault:
> `vault/notes/serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29.md`.

**Python — fechada a nível de arquivo (14 arquivos tocam `by_agent`), com os pontos de duplicação
central confirmados por inspeção; não abri linha a linha todo callsite como em Go/Node.** Confirmados:
`validator.py` (`resolve_req_files:429`, `_resolve_state_dirs:455` — **compartilhada** por
`resolve_wip_dirs`/`resolve_done_dirs`, `validate_wip_limit:855` duplica em vez de reusar
`_resolve_state_dirs`, `validate_folder_status_coherence:1266` duplica, `validate_filename_uniqueness:1321`
duplica), `commands/status.py:_get_agents:107` (6º ponto — equivalente a `GetStatus`/`buildInventorySection`),
`commands/context.py` (2 blocos), `commands/roadmap.py:_list_by_agent:36` (compartilhada por `_cmd_list`
**e** `_find_file`, que por sua vez alimenta `_cmd_show` e — via `generators/roadmap.py:_find_roadmap_matches`
— `move`), `generators/req.py:196`, `generators/roadmap.py:_find_roadmap_matches:183`, `traceid.py`
(`_index_reqs_by_agent:95`, `_index_roadmaps_by_agent:159`), `serve/api_board.py:70`,
`serve/api_chain.py` (2 blocos), `serve/api_metrics.py:34`. Isso fecha ~16 pontos identificados; resto
dos 14 arquivos não foi aberto ponto a ponto — risco residual declarado abaixo.

**Por que a lista fecha (e onde ela pode não fechar).** Todo caminho que lê um diretório de estado ou de
REQ precisa, em algum ponto da cadeia de chamada, (a) referenciar a constante de modo
(`RoadmapNamespacing`/`roadmapNamespacing`/`roadmap_namespacing`) ou (b) chamar um helper que o faça.
Varri (a) diretamente nos 3 runtimes (greps confirmados por arquivo, com o cuidado de `-a` após o achado
acima) e rastreei (b) para as camadas finas de comando que não tocam o modo diretamente
(`internal/commands/roadmap.go`, `req.go`, `status.go` → delegam a `generators`/`validator`;
`commands/status.py`, `commands/req.py` → idem; `barrier.js`/`barrier.py` → idem). **Residual**: um
consumidor que resolvesse um caminho de 2 níveis hardcoded sem nunca tocar a constante de modo escaparia
deste sweep — não encontrei nenhum, mas não é uma prova exaustiva de ausência. Python especificamente
carrega esse risco em maior grau porque não abri os 14 arquivos linha a linha.

##### 2. Modelo de ameaça

**A união não é neutra: ela transforma todo subdiretório de `roadmap_dir`/`req_dir` em algo que o
processo abre e cujo conteúdo ele lê incondicionalmente — antes, isso só acontecia se `agents:` estivesse
vazio ou listasse aquele nome.** Depois da Wave 1, acontece sempre, em todo projeto `by_agent`,
independente de config. É exatamente a frase do ADR — "cobertura maior é superfície maior" — mas
quantificada: o gatilho deixa de ser condicional (config incompleta ou vazia) e passa a ser
incondicional (qualquer entrada de diretório).

- **`..`/caminho absoluto no nome do diretório — não é um vetor real.** Testado: o SO recusa criar uma
  entrada de diretório cujo nome contenha `/` ou seja literalmente `..` (esses são reservados pelo
  próprio filesystem, não pela aplicação). Não há como um atacante fazer `os.ReadDir`/`fs.readdirSync`/
  `os.listdir` devolver uma entrada `..` ou `a/b` — o vetor que a Wave 0 do ADR sugeriu não é
  construtível. Vale a correção porque o ADR pediu para testar, não assumir.

- **Symlink apontando para fora — vetor real, GRAVE, REPRODUZIDO AO VIVO em Node e Python; Go imune por
  acidente de API, não por desenho.** Testei diretamente: `os.ReadDir` (Go) + `entry.IsDir()` retorna
  **`false`** para um symlink mesmo que aponte para um diretório (o bit vem do `dirent` do `readdir`,
  não segue o link). `fs.statSync().isDirectory()` (Node) e `os.path.isdir()` (Python) **seguem** o
  symlink e retornam `true`. Confirmado com teste isolado (`symtest/`, removido após).
  **Consequência reproduzida ao vivo, não inferida**: criei `docs/roadmaps/evil → $OUT` (fora do
  projeto) com `$OUT/wip/RM-leak.md` dentro, e rodei `trackfw roadmap move RM-leak done` — em **Node e
  Python**, o arquivo foi renomeado para `$OUT/done/RM-leak.md`, **fora do diretório do projeto**; em
  **Go**, o comando falhou com "not found" e nada foi escrito. Isso é a mesma classe de defeito do
  vault note `update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md`, em um subsistema diferente
  (enumeração de namespace em vez de `update`/`discover`), e a Wave 1 **repete o padrão de código que
  causou aquele defeito** (`os.path.isdir`/`fs.statSync`) se não for corrigida explicitamente.

  **Precondição medida, e por que a Wave 1 piora especificamente isto (não é pré-existente estático):**
  rodei os dois cenários. Com `agents: [alice]` **declarado** (estado de hoje, antes da união), Node
  respondeu `roadmap "RM-leak" not found in any state directory` — o próprio defeito da REQ (lista
  substitui disco) **blindava** o escape sem querer, porque `evil` nunca era varrido. Só com
  `agents: []` (o caminho de fallback de hoje, que é exatamente o que a união da Wave 1 torna
  **incondicional para todo projeto `by_agent`, com config completa ou não**) o `move` seguiu o symlink
  e escreveu fora. Ou seja: **hoje o escape só dispara se a config estiver vazia; depois da união da
  Wave 1, dispara sempre, independente de `agents:` estar completo.** É a Wave 1 que transforma um vetor
  condicional (config vazia, incomum) em incondicional (todo projeto `by_agent`) — a frase do ADR
  "cobertura maior é superfície maior" aplicada ao caso concreto.

  **Isto bloqueia a Wave 1**: o resolvedor canônico não pode usar `fs.statSync().isDirectory()` (Node)
  nem `os.path.isdir()` (Python) para decidir se uma entrada é um namespace — precisa de
  `fs.readdirSync(dir, {withFileTypes:true})` + `dirent.isDirectory()` (Node, testado: não segue
  symlink) e `os.scandir(dir)` + `entry.is_dir(follow_symlinks=False)` (Python, testado: não segue
  symlink) — o equivalente exato do que Go já faz com `os.ReadDir`+`IsDir()`. **Uma refatoração "de
  limpeza" que trocasse o `os.ReadDir` do Go por algo baseado em `os.Stat` introduziria o vetor no único
  runtime hoje imune** — deve ser proibição nomeada na Wave 1, não um detalhe implícito.

  **Por que corrigir só o resolvedor fecha também o caminho de escrita do `move` (verificado, não
  suposto):** em Go (`MoveRoadmap`, `internal/generators/roadmap.go:412`), Node (`moveRoadmap`,
  `npm/src/generators/roadmap.js:240`) e Python (`move_roadmap`,
  `pypi/trackfw/generators/roadmap.py:581`), o `src` movido vem **exclusivamente** de
  `findRoadmap`/`findRoadmapMatches`/`_find_roadmap_matches` — as mesmas funções já listadas acima como
  pontos do resolvedor. Não há um segundo caminho que redescubra arquivos sob `evil/` depois que o
  resolvedor deixar de incluir `evil` na lista de agentes; o diretório-alvo do `move`
  (`agentStateDir`/`agentStateDir` equivalente) é derivado do diretório do `src` encontrado, não de
  entrada do usuário. Logo: resolvedor corrigido ⇒ `evil` nunca entra na lista de agentes ⇒ `find*`
  nunca encontra `RM-leak.md` sob `evil/` ⇒ `move` nunca tem `src` para mover. Fechar a Wave 1 fecha
  também este vetor de escrita, sem ação adicional em `move` — confirmado lendo as 3 implementações.

- **Nome com separador ou caractere de controle** — mesma conclusão do `..`: o filesystem não permite
  `/` no nome de uma entrada. Caractere de controle (ex. newline) **é** permitido em nomes de arquivo
  Unix, mas não testei um `\n` em nome de diretório neste ciclo — resíduo declarado abaixo.

- **Diretório oculto (`.git`, `.trackfw`, `node_modules`)** — a união vai enumerá-los como "agente" se
  `roadmap_dir` alguma vez contiver um. Não é read-exposure (o conteúdo de `.git/wip/` não existe, então
  não há arquivo `.md` para vazar), mas **é** o gatilho mais provável do vetor da seção 3 (violação
  virar ruído e o usuário desligar a regra) — decisão de design pendente, ver Wave 2.

- **`roadmap_dir` e `req_dir` apontando para o mesmo lugar** — não é um vetor novo desta REQ. Hoje
  (`flat` ou `by_agent`) já produz classificação cruzada (um roadmap aparece como REQ e vice-versa) se
  configurado assim. A união não piora nem resolve isso — é config-error pré-existente, fora de escopo.

- **Nome de diretório que colide com um estado** (`docs/roadmaps/wip/` num projeto `by_agent`) — cenário
  real de migração incompleta flat→by_agent: um `wip/` órfão no topo de `roadmap_dir` vira "agente
  `wip`" após a união, gerando busca por `roadmap_dir/wip/wip/`, `roadmap_dir/wip/done/` etc.
  (tipicamente vazio, inofensivo) e uma violação "namespace `wip` não declarado" genuinamente confusa —
  mesma família do vetor de diretório oculto.

##### 3. Alvos de falsificação nas duas direções

**Regressão para trás (volta a substituir o disco):** projeto com `agents: [a, b]` e `docs/roadmaps/c/wip/RM.md`
em disco → `validate`/`status`/`context`/`serve`/`roadmap move RM` devem enxergar `c`. Se a lista voltar
a substituir, `c` desaparece de todas as 5 superfícies silenciosamente — é o defeito original, cenário
mínimo de Wave 3.

**Regressão para o outro lado (enumera qualquer diretório, inclusive não-namespace):**
1. `.git`/`.trackfw`/`node_modules` sob `roadmap_dir` viram "namespace não declarado" → violação que o
   usuário não sabe resolver (não vai adicionar `.git` a `agents:`). Cenário de gate: projeto com um
   diretório oculto sob `roadmap_dir`, `validate` **não deve** emitir violação de namespace para ele —
   decisão de Wave 2, seção "1" acima cobre a justificativa.
2. Symlink para fora tratado como namespace legítimo (a regressão que este ML-0A bloqueia) — cenário de
   gate: `docs/roadmaps/evil → /tmp/fora`, `roadmap move` **não deve** escrever fora do projeto nem
   `validate` deve enumerar `evil` como namespace.
3. Diretório cujo nome colide com um estado (`wip/`, `done/` etc. soltos no topo de `roadmap_dir`) vira
   "agente" — mesmo ruído do item 1, categoria "guard que atrapalha é guard que o usuário desliga"
   (`ADR-2026-08-17`). Cenário de gate: projeto com `wip/` órfão no topo, sem `agents:` declarando
   `wip`, `validate` não deve pedir para declarar `wip` como agente.

##### 4. Residual declarado

- Nome de diretório com caractere de controle (ex. `\n`) não foi testado neste ciclo — comportamento de
  `os.ReadDir`/`fs.readdirSync`/`os.listdir` e do formatador de mensagem de violação (AC4) com esse nome
  fica sem cobertura.
- Enumeração Python não foi fechada linha a linha como Go/Node — os ~16 pontos confirmados cobrem os
  locais de maior risco (regras de `validate`, `status`, `serve`, `move`/`show`), mas um consumidor
  Python isolado que hardcode caminho de 2 níveis sem tocar `roadmap_namespacing` não seria pego por
  este sweep.
- `roadmap_dir` == `req_dir` (classificação cruzada REQ/roadmap) é um config-error pré-existente, comum
  a `flat` e `by_agent`, que este REQ não piora nem resolve — deixado como está.
- Decisão de tratamento de diretório oculto/nome-colide-com-estado (seção 2/3, item 1 e 3) é
  **pendente**, delegada à Wave 2 conforme a ação 3 do ML-2A original — a recomendação deste ML-0A é
  **excluir nomes iniciando com `.` da união e da violação incondicionalmente** (nenhum agente
  legítimo começa com ponto), e tratar colisão com nome de estado como aviso silencioso opcional, não
  violação — mas a decisão final e sua justificativa por escrito ficam com quem implementa a Wave 2.

## Wave 1 — Resolvedor canônico e união (ML único)
> Dependências: Wave 0 aprovada. **ML único e sequencial**: a mesma regra nos 3 runtimes. Agentes em
> paralelo produziram divergência de comportamento três vezes no ciclo anterior.

### ML-1A — Um resolvedor por runtime, união de `agents:` com o disco
**Status:** ✅ Concluído (implementação e evidência completas; aguardando auditoria do
`trackfw_architect` antes de marcar ✅ — ver protocolo de conclusão de microlote)
**Agente:** `apolo-tf`
**Files affected (fechado pelo ML-0A — ver "Resultado do ML-0A" acima para linha exata e justificativa):**
- Go: `internal/validator/validator.go` (6 pontos: `validateWIPLimit`, `GetStatus`, `resolveStateDirs`,
  `resolveREQFiles`, `validateFolderStatusCoherence`, `validateFilenameUniqueness`),
  `internal/validator/validator_traceid.go` (`collectTraceIdEntriesByAgent`),
  `internal/commands/barrier.go` (`resolveBarrierRoadmap` — hoje duplica, precisa passar a **chamar**
  `validator.resolveStateDirs`, não só ganhar um resolvedor em paralelo),
  `internal/generators/roadmap.go` (`findRoadmap`, `ListRoadmaps`, `scanREQFiles` — **não** `ShowRoadmap`,
  já seguro via glob incondicional, mas migrar por consistência de AC9),
  `internal/generators/req.go`, `internal/generators/context.go` (2 blocos), e
  `internal/serve/api_board.go`/`api_metrics.go` (já seguros, migrar por consistência, não por urgência)
  — mais testes correspondentes.
- Node: `npm/src/validator/index.js` (6 pontos: `resolveReqFiles`, `resolveStateDirs`,
  `validateWIPLimit`, `validateFolderStatusCoherence`, `validateFilenameUniqueness` — hoje duplicam em
  vez de chamar `resolveStateDirs`, apesar do comentário dizer o contrário; `buildInventorySection`),
  `npm/src/commands/context.js` (2 blocos), `npm/src/generators/req.js`, `npm/src/generators/roadmap.js`
  (`listRoadmaps`, `findRoadmapMatches`), `npm/src/serve/api_board.js`, `npm/src/serve/api_chain.js`
  (2 blocos), `npm/src/serve/api_metrics.js` — mais testes. `npm/src/commands/barrier.js` **não precisa
  de mudança de código** (já delega a `resolveWIPDirs`/`resolveDoneDirs`), mas precisa de teste de
  regressão confirmando que herda a correção.
- Python: `pypi/trackfw/validator.py` (5 pontos: `resolve_req_files`, `_resolve_state_dirs`,
  `validate_wip_limit`, `validate_folder_status_coherence`, `validate_filename_uniqueness` — as 3
  últimas duplicam em vez de chamar `_resolve_state_dirs`), `pypi/trackfw/commands/status.py`
  (`_get_agents` — 6º ponto), `pypi/trackfw/commands/context.py`, `pypi/trackfw/commands/roadmap.py`
  (`_list_by_agent`), `pypi/trackfw/generators/req.py`, `pypi/trackfw/generators/roadmap.py`
  (`_find_roadmap_matches`), `pypi/trackfw/traceid.py` (`_index_reqs_by_agent`,
  `_index_roadmaps_by_agent`), `pypi/trackfw/serve/api_board.py`, `pypi/trackfw/serve/api_chain.py`
  (2 blocos), `pypi/trackfw/serve/api_metrics.py` — mais testes. `pypi/trackfw/commands/req.py` não
  precisa de mudança (já delega). **Enumeração Python não foi fechada linha a linha pelo ML-0A** —
  antes de codar, `apolo-tf` deve confirmar com um novo grep (`grep -a -n "agents" <arquivo>` em cada um
  dos 14 arquivos listados no "Resultado do ML-0A") que não há ponto adicional fora desta lista.
**Actions:**
1. Criar **um** resolvedor canônico por runtime que devolva a **união** entre `agents:` e os
   diretórios existentes, para `roadmap_dir` **e** `req_dir` (AC1, AC2).
2. Substituir **todos** os pontos duplicados por chamadas a ele (AC6). O `grep` por `len(agents) == 0`
   e equivalentes só pode casar dentro do resolvedor.
3. Modo `flat` **intocado** (AC8).
4. **Bloqueante de segurança, não opcional** — a detecção de "esta entrada de disco é um namespace"
   **não pode seguir symlink**. Reproduzido ao vivo no ML-0A: `fs.statSync().isDirectory()` (Node) e
   `os.path.isdir()` (Python) seguem symlink e fizeram `trackfw roadmap move` escrever fora do projeto
   através de um diretório de namespace symlinkado; Go (`os.ReadDir`+`entry.IsDir()`) não segue e é
   imune. O resolvedor Node deve usar `fs.readdirSync(dir, {withFileTypes:true})` +
   `dirent.isDirectory()`; o resolvedor Python deve usar `os.scandir(dir)` +
   `entry.is_dir(follow_symlinks=False)`; o resolvedor Go deve **preservar** `os.ReadDir`+`IsDir()` —
   **não trocar por `os.Stat`** sob pretexto de "simplificação", isso reintroduziria o vetor no único
   runtime hoje imune.
**Critérios de aceite:**
- [x] AC1, AC2, AC3, AC6, AC8
- [x] AC7 — não-regressão: `validate`, `status` e `context` sobre este repositório produzem saída
      idêntica à de antes. Compare byte a byte.
- [x] `go build ./...` → 0 · `go test ./...` → 0 · `npm test --prefix npm` → 0 ·
      `PYTHONPATH=pypi python3 -m pytest pypi/tests` → 0


#### Resultado do ML-1A (apolo-tf, 2026-08-29)

Um resolvedor por runtime: `resolveAgentNamespaces` (`validator.go:1006`, exportado para
`generators` e `serve` para evitar ciclo de import), `resolveAgentNamespaces`
(`npm/src/validator/index.js`), e `resolve_agent_namespaces` — este último em
`pypi/trackfw/config.py`, **não** no validator, para evitar o ciclo `validator → traceid → validator`.
Decisão dele, e é a certa.

**Auditoria do arquiteto — os dois pontos centrais, 3 CLIs reais:**

```
symlink apontando para fora do projeto    go: não escapou  node: não escapou  py: não escapou
união (agents: só "alfa", zeus em disco)  go: enxerga      node: enxerga      py: enxerga
check-gates-falsify                       181 cenários, 0 FAIL
grep "len(agents) == 0" em internal/      2 — ambas no COMENTÁRIO do resolvedor
                                            avisando para nunca reimplementar o padrão
```

**O que ele fez com os gates que a própria união tornou vácuos, e por que aceito.** Os cenários 34 e
35 provavam o parsing de `agents:` verificando se um item **aparecia** na saída. Com a união o item
aparece de qualquer forma — no 35 de modo estrutural, porque o diretório de teste tem o mesmo nome
literal do item declarado, então a união o acha mesmo com `cfg.Agents` inteiramente apagado.

Ele retargetou de **presença** para **ordem** (declarados primeiro), que ainda discrimina, e nomeou a
perda em `vault/notes/uniao-disco-agents-mascara-gate-por-presenca-2026-08-29.md` em vez de deixar o
gate verde e mudo. É a conduta certa.

**Duas pendências que EU assumo a partir disso:**

1. **A ordenação declarado-primeiro vale em Go e Node; o `roadmap list` do Python mantém
   alfabética.** Ele preservou por ser divergência pré-existente — mas ela agora é **load-bearing
   para um gate**, e sob a regra de paridade não pode ficar. Entra no ML-2A.
2. **A Wave 2 devolve o sinal perdido, e melhor.** Com a violação de namespace não declarado, um
   `agents:` que falha ao parsear faz **todos** os namespaces virarem não declarados — violações em
   massa. Isso é discriminação por presença de novo, mais forte que ordem. **O ML-3A deve retargetar
   os cenários 34 e 35 sobre o sinal da violação**, aposentando o retarget por ordem.

**Defeitos pré-existentes que ele achou e não corrigiu, corretamente reportados:**
- `pypi/trackfw/commands/status.py:_count_reqs_by_status` nunca soube de `by_agent` — conta REQs por
  listagem flat, e o Inventory do `status` mostra **0** em projeto `by_agent`. Divergente de Go/Node.
- `npm/src/commands/context.js:136` não dá `await` num `validate()` assíncrono — `trackfw context`
  **sempre** falha no Node com `Cannot read properties of undefined`, inclusive em projeto `flat`.
  Grave e independente desta REQ.
- `pypi/trackfw/commands/status.py:_list_dirs` virou código morto.

## Wave 2 — Violação de namespace não declarado (ML único)
> Dependências: Wave 1 concluída. A violação só é segura de emitir depois que a união existe.

### ML-2A — Violação nomeando o namespace não declarado
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** `internal/validator/validator.go` (+ testes), `npm/src/validator/index.js`
(+ testes), `pypi/trackfw/validator.py` (+ testes) — o resolvedor canônico da Wave 1 nos 3 runtimes,
mais o ponto que formata a mensagem de violação (AC4, AC9).
**Actions:**
1. Namespace em disco e ausente de `agents:` → **violação**, nomeando o diretório e instruindo a
   acrescentá-lo (AC4).
2. Mensagem byte-idêntica nos 3 (AC9).
3. Decidir e documentar o tratamento de diretório oculto e de nome que não parece namespace — o ML-0A
   traz a lista de casos (`.git`/`.trackfw`/`node_modules`, diretório cujo nome colide com um estado
   como `wip/` órfão) e a recomendação: **excluir incondicionalmente nomes iniciando com `.` da união e
   da violação** (nenhum agente legítimo começa com ponto); colisão com nome de estado deve virar aviso
   silencioso opcional, não violação de namespace — decisão final e justificativa por escrito ficam com
   quem implementa este ML, não é obrigação seguir a recomendação sem reavaliar.
3. **Paridade de ordenação** (herdada do ML-1A): Go e Node passaram a devolver os namespaces
   declarados primeiro; o `roadmap list` do Python mantém ordenação alfabética. A ordem virou
   **load-bearing para um gate** — alinhe os 3.
**Critérios de aceite:**
- [ ] AC4, AC5, AC9
- [ ] Os artefatos do namespace não declarado continuam sendo **enumerados** mesmo com a violação
      ativa — a união não depende da declaração
- [ ] Suítes dos 3 verdes

## Wave 3 — Gate e contrato
> Dependências: Waves 1 e 2 concluídas.

### ML-3A — Gate falsificável e `docs/cli-parity.md`
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-agent-namespace-union.sh` (novo), `docs/cli-parity.md`, `Makefile`.
**Actions:**
1. Gate cobrindo AC1, AC4 e AC5 nos 3 runtimes, com projeto de sonda em `by_agent` e um namespace
   não declarado. Falsificação nas duas direções.
2. Guarda de vacuidade; contagem de cenários.
3. Seção em `docs/cli-parity.md` anotada com `gate=`.
4. Registrar no `Makefile`.
**Critérios de aceite:**
- [ ] AC10, AC11
- [ ] Gate exit 0 com contagem; vacuidade provada
- [ ] **Rodar no ambiente empobrecido** antes de declarar pronto: locale `C` e `en_US.UTF-8`, e sem
      `node`/`python3` no PATH quando aplicável. Ver
      `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`
- [ ] `check-parity-contract-coverage.sh` → 0

> **Formato do bloco de gate:** cada linha é um **comando independente** — não é script, não há
> estado entre linhas. Ver `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md`.

## Barreira final
Revisão `hefesto-tf` e `hades-tf` sobre o diff entregue, auditoria do arquiteto e
`trackfw barrier --wave 3`. **Só declarar concluído com o CI verde**, não com o verde local.
