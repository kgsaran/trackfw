---
status: done
date: 2026-08-29
req: "docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: A lista `agents:` complementa o disco, e namespace não declarado vira violação

> Created: 2026-08-29 | Status: done

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
**Status:** ✅ Concluído
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
- [x] AC4, AC5, AC9
- [x] Os artefatos do namespace não declarado continuam sendo **enumerados** mesmo com a violação
      ativa — a união não depende da declaração
- [x] Suítes dos 3 verdes


#### Resultado do ML-2A (apolo-tf, 2026-08-29)

Regra `agent_namespace_undeclared`, com dedupe por nome entre `roadmap_dir` e `req_dir`, filtro de
infraestrutura (`isInfraDirName`: nome iniciado por ponto, `node_modules`) e exclusão dos 6 nomes de
estado reservados. Mensagem **byte-idêntica** nos 3:

```
agent namespace "zeus" exists in roadmap_dir, req_dir but is not declared in agents: — add it to trackfw.yaml
```

Ele também fechou a pendência de paridade que eu tinha herdado do ML-1A: o `_list_by_agent` do
Python usava `sorted(agents)` puro; agora respeita a ordem do resolvedor.

**Auditoria do arquiteto — 3 CLIs reais:**

| verificação | go | node | py |
|---|---|---|---|
| violação com mensagem idêntica | ✓ | ✓ | ✓ |
| `.git`, `node_modules`, `wip` órfão acusados | 0 | 0 | 0 |
| **enumeração independente da violação** (`status` vê o namespace com a violação ativa) | ✓ | ✓ | ✓ |
| declarar o namespace zera a violação | ✓ | ✓ | ✓ |
| ordenação declarado-primeiro (`zulu` antes de `alfa`) | ✓ | ✓ | ✓ |

`check-gates-falsify`: **181 cenários, 0 FAIL**. `validate` **deste** repositório (que é `flat`):
16 warnings, 0 violations — inalterado.

A independência entre enumeração e violação é o critério que define a REQ: se o artefato do
namespace não declarado deixasse de ser lido quando a violação dispara, a união teria virado refém da
declaração e não teríamos corrigido nada.

**Observação, não bloqueia:** o formato de saída do `roadmap list` do Python difere de Go/Node —
`[zulu]` + `[backlog] arquivo` contra `[zulu/backlog]` + arquivo. Divergência de **formatação**,
pré-existente e fora do escopo desta REQ; a **ordenação**, que era o que importava, está alinhada.

## Wave 3 — Gate e contrato
> Dependências: Waves 1 e 2 concluídas.

### ML-3A — Gate falsificável e `docs/cli-parity.md`
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-agent-namespace-union.sh` (novo), `docs/cli-parity.md`, `Makefile`.
**Actions:**
1. Gate cobrindo AC1, AC4 e AC5 nos 3 runtimes, com projeto de sonda em `by_agent` e um namespace
   não declarado. Falsificação nas duas direções.
2. Guarda de vacuidade; contagem de cenários.
3. Seção em `docs/cli-parity.md` anotada com `gate=`.
4. Registrar no `Makefile`.
**Critérios de aceite:**
- [x] AC10, AC11
- [x] Gate exit 0 com contagem; vacuidade provada
- [x] **Rodar no ambiente empobrecido** antes de declarar pronto: locale `C` e `en_US.UTF-8`, e sem
      `node`/`python3` no PATH quando aplicável. Ver
      `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`
- [x] `check-parity-contract-coverage.sh` → 0

> **Formato do bloco de gate:** cada linha é um **comando independente** — não é script, não há
> estado entre linhas. Ver `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md`.


#### Resultado do ML-3A (artemis-tf, 2026-08-30)

`scripts/check-agent-namespace-union.sh`, **35 cenários**, exit 0. Cobre AC1, AC4, AC5, AC12, filtro
de infraestrutura e `flat` intocado, nos 3 runtimes, com falsificação em ambas as direções.

**Uma exclusão que ela documentou em vez de esconder:** a direção B2 (regressão do AC12 — voltar a
seguir symlink) cobre só Node e Python. O Go é **imune por desenho da API** — `entry.IsDir()` nunca
segue symlink, então não há regressão a injetar. Registrado no cabeçalho do gate e no
`cli-parity.md`. É a diferença entre "não testei" e "não é testável, e eis por quê".

**Retarget dos cenários 34 e 35** do `check-gates-falsify.sh`: saíram de ordenação para o sinal da
violação `agent_namespace_undeclared`. Ordem era o melhor discriminante disponível quando o
`apolo-tf` a escolheu no ML-1A; a Wave 2 criou um melhor. Ordem é detalhe de apresentação que uma
refatoração de UI muda sem ninguém perceber que quebrou um gate.

**Auditoria do arquiteto:** injetei a regressão da substituição no `resolveAgentNamespaces` do Go
(`if len(ordered) > 0 { return ordered }` antes do `os.ReadDir`) — o gate **reprova**. Restaurado,
volta a 35/35. `check-parity-contract-coverage.sh` verde. `git status` sem nenhum arquivo de produto.

`make quality` completo: exit 0, 43 gates de `parity`, `check-gates-falsify` 181/181, npm 839,
pytest 1555.

## Wave 4 — Corretivas da barreira final

### ML-4A — Filtro fechado e glob seguro (REPROVAÇÃO do `hades-tf`)
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

**Achado 1, crítico — o filtro recriava a cegueira que a REQ existe para fechar.** `isInfraDirName`
suprimia **qualquer** nome iniciado por ponto, da união **e** da violação, sem sinal. Reproduzido por
mim: `docs/roadmaps/.ghost/wip/ROADMAP-...md` → violação 0 e `status` 0 nos 3. Byte-a-byte o defeito
do cmdb, só que atrás de um ponto no nome — e pior, **canal de ocultação deliberada**.

O `hades-tf` retratou a própria recomendação do ML-0A; **eu a endossei** no despacho do ML-2A,
apoiado no `ADR-2026-08-17`. O erro é dos dois.

**A solução dele é melhor que a que eu especifiquei.** Eu mandei usar lista fechada de nomes de
infraestrutura. Ele reduziu a lista a **uma entrada** (`node_modules`) e criou a regra
`agent_namespace_hidden` — aviso, default `warning`, para diretório com ponto, que passa a ser
**enumerado normalmente** e apenas **sinalizado**. Separou **visibilidade** de **severidade**; eu
tinha tratado as duas como a mesma coisa, e foi por isso que aprovei um filtro largo demais.

**Critérios de aceite:**
- [x] `.ghost` enumerado e sinalizado nos 3 CLIs, nunca em silêncio
- [x] Lista fechada de infraestrutura documentada, com uma única entrada
- [x] `[abc` não quebra o `validate`; `*` conta corretamente; `--json` válido
- [x] Sem regressão da união, do não-seguir-symlink, da violação e da ordenação
- [x] Gate 35 → 60 cenários; `flat` intocado; 16 warnings neste repositório

**Achado 2, alto — metacaractere de glob.** Nome vindo do disco alimentava `filepath.Glob`.
Namespace `[abc` matava o `validate` do Go inteiro; `*` inflava contagem de WIP. Corrigido com
`ListMDFiles`/`_list_md_files` (`ReadDir` + filtro), sem glob.

**Auditoria do arquiteto, 3 CLIs:**

| | antes | depois |
|---|---|---|
| `.ghost/` | invisível | enumerado + avisado |
| namespace `[abc` | `validate` morre, exit 1 | não quebra |
| namespace `*` | contagem inflada | conta 2, correto |
| `--json` | vazava texto cru | JSON válido |

### ML-4B — Contrato da regra nova e cobertura de ordenação
**Status:** ✅ Concluído · **Agente:** `artemis-tf`

**Achado meu na auditoria:** a seção "Ordenação declarado-primeiro — load-bearing para gate" estava
anotada `gate=check-agent-namespace-union.sh`, e o gate **não asseria ordenação em lugar nenhum**
(`grep` = 0). Anotação prometendo cobertura inexistente — o mesmo erro que ela própria corrigiu na
REQ do pin de CI, ao reintroduzir um `partial=` que tinha removido.

Ela escolheu **acrescentar os cenários** em vez de rebaixar a anotação, e justificou: o contrato é
vivo, observável por CLI, e **já divergiu uma vez** (o `roadmap list` do Python, que o ML-2A alinhou).
Gate 60 → **66 cenários**, com Direção C corrompendo o resolvedor de cada runtime.

**E corrigiu uma imprecisão minha:** eu repeti do relatório do ML-4A que `agent_namespace_hidden` é
"nunca `off`". Ela mediu ao vivo — `rules: agent_namespace_hidden: off` **é honrado nos 3**. "Nunca
off" descreve o **default**, não uma trava. O `cli-parity.md` agora diz o que o código faz.

**Critérios de aceite:**
- [x] Nenhuma anotação `gate=` prometendo o que o gate não faz
- [x] `agent_namespace_hidden` e o achado de glob documentados com anotação coerente
- [x] Gate 60 → 66 cenários, com falsificação da ordenação nas duas direções
- [x] `check-parity-contract-coverage.sh` → exit 0
- [x] Nenhum arquivo de produto tocado neste ML

**Auditoria do arquiteto:** injetei `sort.Strings(ordered)` no resolvedor do Go — o gate **reprova**.
Restaurado, volta a 66/66. `check-parity-contract-coverage` verde.

## Barreira final
Revisão `hefesto-tf` e `hades-tf` sobre o diff entregue, auditoria do arquiteto e
`trackfw barrier --wave 3`. **Só declarar concluído com o CI verde**, não com o verde local.

#### Parecer de qualidade da barreira final (hefesto-tf, 2026-08-30)

**Veredito: APROVA.**

Método: li `git diff origin/main...HEAD` completo (40 arquivos), a REQ (AC1–AC13), este roadmap e
verifiquei cada afirmação por leitura direta de código — não me apoiei em nenhuma alegação do diff
sem confirmar o call site. Não rodei os gates de novo (já rodados pelo autor); rastreei chamadores.

**1. Duplicação — acabou de verdade.**
Rastreei todo call site de `cfg.Agents`/`cfg.agents`/`cfg["agents"]` e todo `os.ReadDir`/`readdirSync`/
`os.scandir` nos três runtimes que pudesse enumerar namespace de agente. Todos os consumidores —
`internal/generators/context.go`, `internal/generators/req.go` (`resolveREQFiles`),
`internal/generators/roadmap.go` (`findRoadmap`, `ListRoadmaps`), `internal/serve/api_board.go`,
`internal/serve/api_metrics.go`, `internal/commands/barrier.go` (`resolveBarrierRoadmap`, que
explicitamente comenta ter sido apontado pelo ML-0A como um dos dois pontos de divergência e reusa
`validator.ResolveWIPDirs`/`ResolveDoneDirs`) — passam pelo resolvedor canônico
(`resolveAgentNamespaces`/`ResolveAgentNamespaces` em Go; `resolveAgentNamespaces` exportado de
`npm/src/validator/index.js`, consumido por `context.js`, `api_board.js`, `api_chain.js`,
`generators/roadmap.js`, `generators/req.js`, `api_metrics.js`; `resolve_agent_namespaces` em
`pypi/trackfw/config.py`, consumido por `context.py`, `traceid.py`, `commands/roadmap.py`,
`commands/status.py`, `generators/roadmap.py`, `serve/api_board.py`, `generators/req.py`,
`traceid.py`, `serve/api_chain.py`, `serve/api_metrics.py`, e reexportado por `validator.py`).
`grep` confirma zero ocorrências do padrão de substituição (`len(agents) == 0` /
`!agents.length`/`agents.length === 0` / `not agents`) fora do próprio resolvedor nos três runtimes.
`internal/generators/roadmap.go:108` e `npm/src/generators/roadmap.js:72` (`if len(cfg.Agents) > 0 {
agent = cfg.Agents[0] }`) são o **caminho de escrita** — escolher o agente default ao criar um roadmap
novo sem `--agent` — não o de leitura/enumeração; não é o mesmo defeito e não deveria passar pelo
resolvedor (a REQ ADR-2026-08-29 sobre o mecanismo de escrita, commit `35b79f9`, trata esse caso à
parte). Confirmei que não é um bypass disfarçado lendo o contexto ao redor.

Sobre se a estrutura empurra para reuso ou só espera boa vontade: empurra, com dois reforços reais,
não só o comentário. (a) O contrato de paridade (`docs/cli-parity.md`, `<!-- trackfw-contract:
gate=scripts/check-agent-namespace-union.sh -->`) e o próprio gate tornam qualquer reintrodução do
padrão antigo **failável por gate**, não só revisável por humano — a Direção A da falsificação em
`check-agent-namespace-union.sh` corrompe exatamente esse ponto e prova que o gate reprova. (b) Os
wrappers exportados (`ResolveWIPDirs`, `ResolveDoneDirs`, `ResolveAgentNamespaces` em Go) e o único
ponto de export em Node/Python tornam "escrever de novo" estritamente mais trabalho que "importar e
chamar" — a fricção estrutural favorece o caminho correto. O comentário no topo da função (linhas
1002–1014 de `validator.go`, espelhado em Node/Python) é reforço, não a única defesa.

**2. Resolvedor do Python em `config.py`.**
Defensável, não é gambiarra escondendo problema de camadas. Confirmei a cadeia de import real:
`pypi/trackfw/validator.py` importa `from .traceid import check_traceid` (linha 15), e
`pypi/trackfw/traceid.py` importa `from . import config as _config` (linha 19) — ambos os módulos que
precisam do resolvedor (`validator.py` e `traceid.py`) já dependem de `config.py`; `config.py` não
depende de nenhum dos dois. É o ancestral comum correto pela topologia de import já existente, não uma
escolha arbitrária. Confirma-se por comparação: em Go, `validator_traceid.go` está no MESMO pacote
`validator` (chama `resolveAgentNamespaces` sem import extra) — não há cycle porque não há fronteira de
pacote; em Node, `traceid.js` é um arquivo separado mas requerido por `validator/index.js` na mesma
direção do Python (`index.js` → `require('./traceid')`), e o próprio `resolveAgentNamespaces` vive em
`index.js`, então Node também não tem o cycle porque o resolvedor está do lado que já importa traceid,
não do lado importado. As três soluções são coerentes com a topologia de cada runtime, não
just-Python-different-by-accident. `validator.py:434` reexporta `resolve_agent_namespaces` por
compatibilidade — comportamento idêntico ao original em todo call site que já importava de lá.

**3. Testes vacuous — não encontrei nenhum, mas na primeira passada só tinha lido o cabeçalho de
`check-agent-namespace-union.sh` (linhas 1–50) e aceitei a alegação do próprio script sobre o que ele
cobre, em vez das 537 linhas. É a única evidência de AC1/AC4/AC5/AC12 nos 3 runtimes (não há teste
unitário Go), e a nota de trabalho já registrava que o arquiteto tinha encontrado 2 asserções
decorativas nesse mesmo arquivo antes desta linha (AC5(b) sem asserção própria; Direção A e os filtros
de infra/flat comparando só ausência sem âncora de presença). Reli o arquivo inteiro (588 linhas) para
confirmar se sobrou uma terceira.**
- Auditei as 35 chamadas de `ok(...)`: todas estão atrás de um `if ... fail ...` que testa uma condição
  real sobre a saída capturada — nenhuma `ok()` incondicional após um `; true` que engoliria uma saída
  vazia. As duas lacunas que o arquiteto encontrou e corrigiu (AC5(b) e a âncora de presença de
  Direção A/infra-filter/flat) estão de fato corrigidas: AC5(b) hoje é uma conjunção única (violação de
  bob presente E evidência de que o arquivo de bob foi escaneado — linhas 289–298); Direção A verifica
  `alice` (declarado) presente antes de checar `bob` ausente (linhas 428–433, e os pares Node/Python
  espelhados); infra-filter e flat-untouched têm âncora de vivacidade (violação de `alice`/roadmap-flat
  presente) antes de checar ausência da violação indevida (linhas 329–330, 362–363). Não encontrei uma
  terceira lacuna do mesmo padrão.
- Todo `corrupt_literal` (Direções A, B1, B2, e os retargets 34/35 em `check-gates-falsify.sh`) valida
  `count != 1` antes de escrever — uma corrupção que deixasse de casar por retarget futuro do código-alvo
  falha alto (`SystemExit`), nunca vira no-op silencioso.
- `pypi/tests/test_validator.py::test_modo_by_agent_agents_configurados_complementados_pelo_disco`
  cria `zeus/` em disco SEM declará-lo em `agents:` e afirma que aparece no resultado — reverter a
  correção (voltar à substituição) quebra esta asserção de verdade, não é fixture que coincide por
  acaso.
- Os retargets 34/35 de `check-gates-falsify.sh` seguem o mesmo padrão "corrompe a implementação, nunca
  a asserção" com prova de não-vacuidade em duas pontas (braço limpo E corrompido) — auditei os cenários
  34 e 35 na íntegra (linhas 2603–3047) e a violação `agent_namespace_undeclared` como discriminante é
  byte-específica (mensagem cita o nome do namespace), não posicional.
- Não há testes unitários Go dedicados ao resolvedor ou à violação (`grep` não encontra
  `resolveAgentNamespaces`/`agent_namespace_undeclared` em nenhum `_test.go`) — cobertura em Go vive
  inteiramente nos gates de shell. Isto é consistente com o padrão já estabelecido neste repositório
  (REQs anteriores também dependem primariamente de `check-gates-falsify.sh`/gates dedicados em vez de
  testes unitários Go para regras de `validate`) e a REQ pede explicitamente "gate falsificável" como
  critério (AC10), não teste unitário — não é uma lacuna introduzida por este ML, é debt pré-existente
  do projeto. Registro como observação, não bloqueio.

**4. Retarget dos Cenários 34/35 — ainda prova o que deveriam provar.**
Segue provando parsing de `agents:`, não virou teste de outra coisa com nome antigo. A cadeia completa
(presença/ausência → ordem → violação `agent_namespace_undeclared`) é honestamente documentada nos
próprios comentários do gate como degradação progressiva do discriminante à medida que a união e depois
a violação tornaram os discriminantes anteriores vácuos — e cada retarget é comprovado empiricamente
(rodado ao vivo antes de codar, conforme o comentário RETARGET 3) e mede exatamente
`cfg.Agents = items` / `cfg.agents = items` / `cfg["agents"] = items`, o ponto de atribuição final do
valor já parseado pela biblioteca YAML — não um proxy indireto. É o ponto genérico mais próximo que
ainda preserva a intenção operacional ("a chave `agents:` é lida, não descartada") depois que
`isListItem`/`splitTopLevelCommas` deixaram de existir (substituídos por `yaml.v3`/`yaml` 2.x em Wave
anterior). Concordo com a leitura do autor: não há mais nada mais específico para corromper nesses
runtimes porque o parsing propriamente dito é responsabilidade da biblioteca, não de código do
projeto.

**5. Legibilidade — boa, comentários carregam o porquê.**
Os quatro pontos sutis (união, filtro de infra, não-seguir-symlink, ordenação declarado-primeiro) têm
cada um seção própria em `docs/cli-parity.md` e comentário correspondente no código-fonte. Confirmei
especificamente que o comentário de `entry.IsDir()` (linhas 1010–1014 de `validator.go`) explica por
que não pode virar `os.Stat`: `entry.IsDir()` reflete o `dirent` da própria entrada do diretório (via
Lstat interno), nunca o alvo do link, enquanto `os.Stat` segue symlink e reintroduziria o vetor que
hoje escreve fora da árvore em Node/Python — motivo, não só a regra. A tabela de primitivas por runtime
em `cli-parity.md` reforça isso com uma coluna "por que não segue symlink" nomeando o mecanismo
(`dirent.d_type` vs `stat()` do alvo) para os três runtimes.

**Achado de escopo, não bloqueante — registrar como dívida (reformulado após reconciliação).**
Framing inicial ("symlink-follow em `walkMd`") era a versão mais fraca do achado — `walkMd` só *lê*,
então não toca a garantia literal de AC12 (`roadmap move` escrever fora da árvore). O achado correto,
mais específico, é uma **assimetria de paridade de filtro de infraestrutura na indexação de traceid,
alargada por este diff**: antes do ML-1A, nenhum dos 3 runtimes filtrava `.git`/`node_modules` na
indexação de traceid em modo `by_agent`. Depois do ML-1A, Go (`collectTraceIdEntriesByAgent`, que passou
a chamar `resolveAgentNamespaces`) e Python (`_index_reqs_by_agent`/`_index_roadmaps_by_agent`, que
passaram a chamar `resolve_agent_namespaces`) filtram infra automaticamente, porque herdam o filtro do
resolvedor canônico — efeito colateral correto do ML-1A, não intencional para traceid especificamente.
`npm/src/validator/traceid.js::walkMd` (não tocado por este diff, nem por nenhum ML anterior) continua
recursando `reqDir`/`roadmapDir` inteiro sem filtro de infra e sem noção de `agents:`/`by_agent` — se
`trace_id_field` estiver configurado e existir um `.md` com esse campo dentro de `.git`/`node_modules`,
Node o indexa e Go/Python não. É uma divergência de paridade (AC9-adjacente) **entre os 3 runtimes**,
não apenas um detalhe de symlink de um arquivo isolado — e ficou mais larga com este diff, ainda que
não introduzida por ele. Debt real, condição de disparo estreita (exige `trace_id_field` configurado +
arquivo dentro de dir de infra) — não bloqueia. Sugiro REQ curta para alinhar `walkMd` ao filtro de
infra e à primitiva `withFileTypes`+`dirent.isDirectory()` (esta segunda parte por consistência com
AC12, ainda que a garantia de escrita não esteja em jogo aqui).

**Nit de doc, não bloqueante.** `docs/cli-parity.md`, seção "Ordenação declarado-primeiro — load-bearing
para gate", termina com "não removê-la nem alterar sua semântica de exibição no `serve` sem reavaliar os
dois cenários citados [34/35]". Isso ficou desatualizado pela própria narrativa do parágrafo anterior:
depois do RETARGET 3 (ML-3A), os Cenários 34/35 não dependem mais de ordem — dependem da violação
`agent_namespace_undeclared`. A instrução de "reavaliar 34/35" antes de mexer na ordenação já não é o
guard-rail correto; o texto deveria dizer que a ordenação continua sendo o contrato ativo do `roadmap
list`, sem mais depender dos Cenários 34/35 para isso.

**Resumo do que bloqueia vs. dívida:**
- Bloqueia o PR: nada encontrado.
- Dívida a registrar (não bloqueante): (a) assimetria de paridade — `npm/src/validator/traceid.js::walkMd`
  não filtra `.git`/`node_modules` nem segue a mesma primitiva não-symlink que Go/Python passaram a usar
  via `resolveAgentNamespaces` — REQ curta separada; (b) ausência de testes unitários Go dedicados ao
  resolvedor/violação — padrão pré-existente do projeto, não introduzido aqui, mas vale avaliar em algum
  momento se a superfície cresce; (c) nit de doc em `cli-parity.md` (seção de ordenação) referenciando os
  Cenários 34/35 como guard-rail depois que RETARGET 3 os desacoplou de ordem — corrigir o texto na
  próxima edição dessa seção, não vale um ML dedicado.

#### Parecer de segurança da barreira final (hades-tf, 2026-08-30)

**Veredito: REPROVA.** Dois achados diretamente introduzidos por esta REQ reproduzem, ou pioram, exatamente o defeito que o ADR nomeia como motivação ("nada em disco fica invisível, em nenhum modo, por nenhuma configuração" / "atestado de saúde sobre o que nunca abriu é pior que quebrar"). Um terceiro achado, pré-existente e mais amplo que `by_agent`, foi reproduzido ao vivo dentro do vetor que o próprio ADR pediu para atacar ("symlink dentro de um namespace legítimo") e é reportado para rotear a uma REQ própria, não para bloquear esta.

Toda evidência abaixo foi executada ao vivo nos 3 CLIs (binário Go compilado desta branch, `npm/bin/trackfw`, `PYTHONPATH=pypi python3 -m trackfw.cli`), não inferida.

##### 1. BLOQUEIA — Namespace com nome iniciado por "." atinge invisibilidade total, nos 3 runtimes

`isInfraDirName` (`internal/validator/validator.go:1061`, espelhado em Node/Python) filtra incondicionalmente qualquer nome iniciado por "." **tanto da união quanto da violação**. Isso não é o mesmo tratamento dado a `node_modules` ou a colisão com nome de estado (`wip/` órfão) — esses dois casos continuam **enumerados** (só não disparam a violação); o prefixo "." os remove dos dois, e portanto não deixa nenhum sinal.

Reprodução: `docs/roadmaps/.ghost/wip/RM-ghost.md` (frontmatter válido, `status: wip`), projeto `by_agent` com `agents: [alfa]`.

```
go     validate -> "No violations found."      (exit 0)
node   validate -> "No violations found."      (exit 0)
py     validate -> "No violations found."      (exit 0)

go     status   -> Roadmaps 0, wip 0 (todas as colunas)
node   status   -> idem
py     status   -> idem

go     roadmap move RM-ghost done -> "not found in any state directory"
node   roadmap move RM-ghost done -> idem
py     roadmap move RM-ghost done -> "nao encontrado em nenhum estado"
```

E byte-a-byte a manifestação original do cmdb (REQ, Motivation): `move` falha "not found" para um arquivo que existe, e `validate` reporta limpo sobre o que nunca enumerou — só que agora **reintroduzida pelo próprio código que fecha o defeito**, atrás de uma condição trivial (nomear o namespace com "." na frente) em vez de `agents:` incompleto. Qualquer agente ou colaborador — malicioso ou só copiando um padrão de nome oculto (`.cache`, `.tmp`) para um namespace de verdade — reabre a cegueira sem que nada avise.

**Isto é o residual que o próprio ML-0A declarou e a recomendação que eu mesmo dei** (seção 4: "excluir nomes iniciando com '.' da união e da violação incondicionalmente — nenhum agente legítimo começa com ponto"). A premissa era estreita demais: cobre `.git`/`.trackfw`/`.DS_Store` (nomes que o SO ou uma ferramenta cria, nunca um humano digitando um namespace), mas não distingue esses de um namespace real batizado com ponto por engano ou por design deliberado de ocultação. A recomendação deveria ter sido: excluir da **violação** (não gerar ruído pedindo para declarar `.git`), mas **manter na união e emitir um sinal de baixo ruído** (aviso, não erro) nomeando o diretório oculto ignorado — o mesmo padrão que o ML-2A corretamente aplicou à colisão de nome de estado.

##### 2. BLOQUEIA — Metacaractere de glob no nome do namespace corrompe a contagem do Go, silenciosamente quando e "*", com crash quando e "["

`resolveAgentNamespaces` (Wave 1) alimenta `agent`, agora derivado de qualquer nome de diretório em disco, direto em `filepath.Glob(filepath.Join(roadmapDir, agent, "wip", "*.md"))` sem escapar (`validator.go:223`, e o mesmo padrão em `generators/req.go:148`). Antes da união, `agent` só vinha de string digitada em `agents:` pelo operador; a união faz qualquer nome de diretório em disco chegar ao mesmo `Glob` sem validação de formato — diferente de nome de arquivo de REQ/roadmap, que precisa casar `TYPE-YYYY-MM-DD-slug.md` antes de entrar em qualquer violação.

**Caso "*" — corrupção silenciosa da contagem, confirmada ao vivo:**

Fixture: `alfa/wip/` com 3 arquivos, `beta/wip/` com 1 arquivo (não declarado), `*/wip/` (nome literal "*") com 1 arquivo, `wip.limit: 1`.

```
3 roadmaps in wip/ for agent "alfa" (limit: 1)
5 roadmaps in wip/ for agent "*" (limit: 1)      <- deveria ser 1
```

`filepath.Glob("docs/roadmaps/*/wip/*.md")` — o padrão que o `Glob` recebe quando `agent` é literalmente "*" — casa com **todos** os `wip/` de todos os namespaces (alfa, beta e o próprio "*"), não com o diretório "*" sozinho. O aviso de WIP limit atribuído ao namespace "*" soma artefatos de outros agentes; ninguém lendo "5 roadmaps in wip/ for agent *" sabe que é contagem cruzada. É exatamente o vetor que o ADR nomeia e o ML-0A pediu para vigiar ("passa a enumerar qualquer coisa... ou a violação vira tão barulhenta") — só que na direção pior: não é ruído, é **número plausível e errado**, o oposto do "quebra alguém percebe" que o próprio ADR usa como critério de desenho.

**Caso "[" (ou qualquer classe de caractere não fechada) — DoS total do `validate`, confirmado ao vivo:**

```
$ trackfw validate    (namespace "unmatched[bracket" em disco)
Error: syntax error in pattern
Usage:
  trackfw validate [flags]
...
exit=1
```

`ErrBadPattern` do pacote `path/filepath` sobe cru como erro de `validateWIPLimit`, sem passar pela regra `agent_namespace_undeclared` pretendida — o usuário vê um erro de "Usage" de CLI, não a violação de configuração que a REQ existe para produzir. **`--json` também vaza texto puro no canal que deveria ser JSON**: `trackfw validate --json` no mesmo fixture imprime `syntax error in pattern` sem estrutura, quebrando qualquer consumidor que faça `json.loads` da saída.

Python (`glob.glob`) degrada graciosamente no mesmo fixture — confirmado ao vivo, continua enumerando e emite a violação `agent_namespace_undeclared` pretendida, sem crash. Node não usa padrão de glob nesse caminho (`readdir`-based), não está exposto a esta classe. **É um defeito específico do Go**, mas o Go é a implementação de referência do projeto.

**Por que isto bloqueia:** os dois casos ("*" e "[") são o mesmo defeito raiz (string de disco não validada em posição de padrão de `Glob`) manifestando de dois jeitos — um barulhento (crash), um silencioso (contagem errada). Corrigir só o crash sem corrigir o "*" deixaria o pior dos dois de pé.

##### 3. NÃO BLOQUEIA ESTA REQ, roteia para REQ própria — symlink de ARQUIVO dentro de namespace legítimo, reproduzido ao vivo, pré-existente e mais amplo que `by_agent`

Isto é resposta direta ao pedido de atacar "symlink dentro de um namespace legítimo (não o namespace em si)". AC12 só cobre symlink no **diretório** de namespace (`entry.IsDir()`/`dirent.isDirectory()`/`is_dir(follow_symlinks=False)`); nada nos 3 runtimes verifica se o **arquivo** `.md` dentro de um namespace já validado é, ele mesmo, um symlink.

Fixture: `docs/roadmaps/alfa/wip/RM-leak.md` é um symlink para um arquivo com frontmatter válido **fora do projeto** (`$SCRATCH/victim.txt`, com `status: wip` e `| Status: wip |`). Comando: `trackfw roadmap move RM-leak done` — uso normal, não uma chamada adversária de baixo nível.

```
go    -> victim.txt (fora do projeto): status: wip -> status: done   MUTADO
node  -> victim.txt (fora do projeto): status: wip -> status: done   MUTADO
py    -> nao muta o victim.txt, mas GRAVA seu conteudo INTEIRO dereferenciado
         dentro de docs/roadmaps/alfa/done/RM-leak.md (arquivo real, rastreavel por git)
```

**Precisão sobre o que Go/Node realmente fazem — não é "arbitrary file write" genérico:** `os.Rename(src, dst)` sobre um symlink move o link em si (não segue para renomear o alvo); a mutação acontece no passo seguinte, `os.ReadFile(dst)` + `os.WriteFile(dst, ...)` (`internal/generators/roadmap.go` em torno de `MoveRoadmap`, equivalente em Node), que abrem `dst` — agora o link, já realocado para dentro de `done/` — sem O_NOFOLLOW, e por isso leem/escrevem através dele. O escritor (`rewriteRoadmapStatus`) só toca a linha `status:` do frontmatter e o `| Status: |` do corpo — é uma **mutação de conteúdo preservado, restrita a essas duas linhas**, não conteúdo arbitrário do atacante, e não muda permissão do arquivo-alvo (só se aplica em criação). Ainda assim é integridade violada em um arquivo fora da árvore do projeto, disparada por um comando de rotina — e o link em si fica realocado dentro de `done/`, rastreável por git se commitado.
**Variante Python, e por que pode ser pior:** não muta o alvo, mas **copia o conteúdo dereferenciado inteiro** para um arquivo novo e real dentro da árvore rastreada (`done/RM-leak.md`). Isso é um primitivo de exfiltração de conteúdo local — sobrevive a um patch que só feche a escrita (Go/Node), porque a causa em Python é a leitura seguindo o link, não a escrita.
**Hardlink não é coberto por uma checagem `islink`/`Lstat` em `src`:** um `.md` hardlinkado é indistinguível de um arquivo comum para o sistema de arquivos — qualquer remediação baseada só em detectar symlink não fecha esse caso; precisa ser dito explicitamente para quem implementar, para a correção não ser declarada completa cedo demais.
**Reproduzido também em modo `flat`** (sem `by_agent`, sem união, sem este ADR) — não é causado nem alargado pela Wave 1/2 desta REQ; é pré-existente em `MoveRoadmap`/`moveRoadmap`/`move_roadmap` e mais amplo que o escopo desta REQ. Sigo o precedente que o próprio `hefesto-tf` registrou nesta auditoria para o `walkMd` do Node (achado de symlink fora do escopo literal, roteado para REQ curta separada em vez de reabrir esta): recomendo o mesmo aqui — **REQ própria, bloqueante para `roadmap move` antes do próximo release**, não bloqueante para este PR. Registro aqui, e não em `docs/agents-working-context.md` nem em nota de vault — o escopo desta tarefa pede para não tocar em nenhum outro arquivo; a lacuna fica anotada nesta frase para quem processar o handoff.

##### 4. Confirmado — `roadmap_dir == req_dir` não duplica nem pior

Fixture com as duas chaves apontando para o mesmo diretório físico produz **uma única violação deduplicada**, nomeando as duas árvores ('agent namespace "zeta" exists in roadmap_dir, req_dir but is not declared...'). Confirmado como projetado — o residual do ML-0A ("config-error pré-existente, não piora nem resolve") permanece correto sem alteração.

##### 5. Reconfirmação/correção dos residuais que eu declarei no ML-0A

- **"Caractere de controle não testado" — testado agora, e o resultado é pior do que eu havia sinalizado.** Não é só corrupção visual: um "[" desbalanceado (que nem é caractere de controle, é ASCII imprimível comum) **derruba o `validate` inteiro** (achado 2). Byte de controle puro (ESC/OSC/CR) sem colchete não derruba nada, mas — confirmado ao vivo — grava sequência de escape crua no stdout em texto puro (`internal/commands/validate.go:67`, `fmt.Printf("- %s\n", v)`): um OSC de título de terminal (ESC ] 0 ; ... BEL) seta o título do terminal, e um CR (retorno de carro) seguido de texto forjado sobrescreve a linha visível com um "tudo certo" falso — enquanto o exit code continua 1 e `--json` escapa corretamente todos os bytes de controle (confirmado ao vivo: aparecem como sequências `\u00XX` no JSON), então só engana quem olha o terminal ao vivo ou um log renderizado ingenuamente, não quem checa exit code ou `--json`. Não é vetor novo — nomes de arquivo já entravam sem sanitização em outras violações deste mesmo arquivo (`validator.go:1254`, `1722`, `2368`) e há precedente registrado em `vault/notes/rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21.md` — mas esta REQ **amplia** a superfície: antes, o universo de nomes que aparecem em `agent_namespace_undeclared` era limitado a `agents:` declarado pelo operador; agora é qualquer nome de diretório em disco, sem validação de formato. Severidade: Média — impacto é encenação/UX enganosa em terminal, não execução de código nem perda de dado; não bloqueia esta barreira, mas deveria constar como residual explícito no ADR/REQ (não consta hoje).
- **"`..`/caminho absoluto não é vetor real" — reconfirmo, sem mudança.** Continua não construível: o SO recusa "/" ou ".." literal como nome de entrada de diretório.
- **TOCTOU entre enumeração e escrita do `move` — permanece residual nomeado, não elevo a severidade porque não reproduzi, só raciocinei.** `resolveAgentNamespaces` faz Lstat no momento da enumeração; `MoveRoadmap`/`moveRoadmap`/`move_roadmap` fazem `MkdirAll`+`Rename` (ou leitura+escrita) minutos ou milissegundos depois, sem reter descritor nem relock — resolução de caminho por componente sempre segue symlink para diretórios intermediários no SO, então um componente do caminho trocado por symlink entre os dois passos escaparia da checagem inicial. Pré-condição real: escritor concorrente com acesso de escrita ao mesmo worktree correndo contra o `move` — não é ataque passivo (checar um PR malicioso não dispara isso sozinho), mas é o modelo operacional do próprio projeto (múltiplos subagentes com escrita concorrente no mesmo checkout). Não fechável portavelmente nos 3 runtimes sem primitivas `*at` (ex.: `openat2(RESOLVE_NO_SYMLINKS)`, só Linux). Documentar, não prometer fechar aqui.
- **Junction/reparse point do Windows — não verificável neste ambiente (Darwin), sem máquina Windows.** As primitivas do AC12 (`entry.IsDir()`/`dirent.isDirectory()`/`is_dir(follow_symlinks=False)`) são cientes de symlink POSIX; o comportamento delas contra reparse point NTFS é desconhecido daqui. Cross-referência: issue #216, já fora de escopo por instrução desta tarefa.
- **Enumeração Python não fechada linha a linha — sem mudança, meus achados 1 e 2 não abriram novo ponto fora dos ~16 confirmados.**

##### Resumo — bloqueia vs. roteia vs. não bloqueia

- **Bloqueia o PR:** achado 1 (invisibilidade total por prefixo ".", 3 runtimes) e achado 2 (metacaractere de glob no Go — "*" corrompe contagem em silêncio, "[" derruba `validate` e vaza texto puro no canal `--json`).
- **Roteia para REQ própria, não bloqueia esta:** achado 3 (symlink de arquivo dentro de namespace legítimo em `roadmap move`, pré-existente, mais amplo que `by_agent`, reproduzido em Go/Node/Python com variante de mutação em Go/Node e variante de exfiltração de conteúdo em Python; sem cobertura por checagem só de symlink devido a hardlink).
- **Não bloqueia, registrar como residual explícito:** injeção de escape de terminal/CR em nome de namespace (achado 5, severidade Média, superfície ampliada por esta REQ mesmo não sendo nova), TOCTOU do `move` (sem reprodução, só raciocínio), junction/reparse Windows (não verificável aqui).

**Microlote corretivo mínimo, se a squad optar por consertar em vez de reabrir Wave 2:**
- **ML-4A (bloqueante, 3 runtimes):** decidir e implementar o tratamento de namespace "."-prefixado como aviso de baixo ruído nomeando o diretório ignorado (mesma classe do tratamento já dado à colisão de nome de estado), nunca como "zero sinal, zero enumeração" — fecha achado 1.
- **ML-4B (bloqueante, Go — verificar Node/Python para o mesmo padrão em `req.go`/equivalentes):** trocar `filepath.Glob(join(dir, agent, "wip", "*.md"))` por `os.ReadDir(join(dir, agent, "wip"))` + filtro de sufixo ".md" em código, ou escapar os metacaracteres de glob ("*?[\\") em `agent` antes de montar o padrão — fecha achado 2 nas duas manifestações (crash e contagem silenciosa), sem depender de `agent` nunca conter esses caracteres.
- **ML-4C (não bloqueante, recomendado):** REQ curta e independente para o symlink de arquivo em `roadmap move`/`move_roadmap`/`moveRoadmap` (achado 3) — Lstat em `src` antes de ler/mover, recusar se for symlink, e nomear explicitamente que isso não cobre hardlink.
- **ML-4D (não bloqueante, recomendado):** declarar no ADR/REQ, como residual explícito, a superfície de injeção de escape de terminal em nome de namespace (achado 5) — não corrigir aqui necessariamente, mas parar de deixá-la implícita.

**Nota de processo:** a instrução desta tarefa pede para não tocar em nenhum arquivo além deste roadmap; por isso o achado 3 (symlink de arquivo) não gerou nota de vault nem entrada em `docs/agents-working-context.md` apesar de ambos serem, em circunstância normal, obrigatórios para um achado desta natureza — quem processar este handoff deve criar os dois.
