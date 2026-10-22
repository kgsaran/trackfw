---
date: 2026-10-22
author: hades-tf
ml: ML-0A
roadmap: ROADMAP-2026-09-22-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md
---

# Wave 0 — Threat Model: log por entrada (ADR-2026-10-22)

> Sections 1–6 do ML-0A mapeados nas quatro seções obrigatórias do Wave 0.
> Evidência: arquivo:linha ou saída de comando medida na branch
> `fix/trackfw-log-conflita-em-toda-branch-paralela`
> (HEAD commitado, `git -C /private/tmp/trackfw-toml-escape status --short` vazio).

---

## Seção A — Enumeração completa (ML-0A §1 + §3)

### A1. Escritores do `.trackfw-log`

| Sítio | O que faz | O que muda |
|---|---|---|
| `internal/generators/roadmap.go:1180-1208` (`appendTransitionLogEntry`) | Abre `<roadmapDir>/.trackfw-log` em `O_APPEND` e escreve uma linha | Grava `<roadmapDir>/.trackfw-log.d/<AAAAMMDDTHHMMSSZ>-<h8>.log` (nova função) |
| `internal/generators/req.go:513-525` (`appendREQTransitionLogEntry`) | Idêntico, para `<reqDir>/.trackfw-log` | Mesma mudança em `reqDir` |
| `internal/commands/discover.go:175` (`--bootstrap-log`) | Abre `<roadmapDir>/.trackfw-log` em `O_APPEND|O_CREATE`, escreve entradas retroativas | Deve gravar no diretório `.d/`; a dedup de entradas existentes precisa ler o legado |

O comentário em `scaffold.go:2053` (`gitAttributesBlock`) diz "append-only — every write lands on the last line" e precisa ser atualizado para descrever o formato por entrada.

### A2. Leitores do `.trackfw-log`

Há **dois parsers duplicados** hoje (confirmado: `roadmap.Context` linha 24). Ambos precisam ser eliminados em favor de um único ponto de leitura.

| Sítio | Parser | Tipo de open | Fuso | Situação |
|---|---|---|---|---|
| `internal/metrics/metrics.go:47-80` (`ParseLog`) | `bufio.Scanner` sobre `os.Open` | `os.Open` — sem verificação de tipo | `time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)` | **Eliminar; usar reader unificado** |
| `internal/serve/metrics_log.go:25-54` (`ParseLog`) | `bufio.Scanner` sobre `os.Open` | `os.Open` — sem verificação de tipo | `time.Parse("2006-01-02 15:04", m[1])` → UTC zero | **Eliminar; usar reader unificado** |
| `internal/serve/api_metrics.go:40-41` | Chama `serve.ParseLog` | — | — | Atualizar chamada para reader unificado |
| `internal/validator/validator.go:2873-2895` (`latestWIPTransitionTime`) | Chama `readRegularFile` (FIFO-safe) | `readRegularFile` — seguro | `time.ParseInLocation("...", time.Local)` | **Atualizar para reader unificado** |
| `internal/commands/log.go:27-54` (`runLog`) | `bufio.Scanner` direto | `os.Open` — sem verificação de tipo | n/a (só imprime linhas) | Atualizar para reader unificado |
| `internal/commands/metrics.go:20-22` | Chama `metrics.ParseLog` com caminho hardcoded `"docs/roadmaps/.trackfw-log"` | via `metrics.ParseLog` | — | Bug pré-existente + atualizar chamada |
| `internal/discover/discover.go:543` | `fileExists` (bool apenas) | — | — | Atualizar para checar existência do `.d/` também |
| `internal/commands/discover.go:106-110` | UI: imprime aviso se `!HasTrackfwLog` | — | — | Atualizar mensagem para incluir `.d/` |

**Fuso — problema existente nos dois parsers a eliminar**: `metrics.go` usa `time.Local`; `metrics_log.go` usa `time.Parse` (UTC zero). Eles divergem para qualquer projeto com fuso `!= UTC`. O reader unificado deve usar `time.Local` para linhas do legado (retrocompatibilidade com os 454 registros existentes) e o timestamp do nome do arquivo (UTC) como chave de ordenação para entradas de `.d/`. Desempate pelo nome do arquivo (conforme ADR D1).

### A3. `docs/agents-working-context.md`

| Sítio | O que faz | O que muda |
|---|---|---|
| `internal/generators/agentfiles.go:92-93` | Instrução "Before starting: read `docs/agents-working-context.md`" e "After finishing: update `docs/agents-working-context.md`" | Instruções apontam para `docs/agents-working-context.d/` |
| 12 assets em `internal/integrations/assets/agents/*.md` | "Append an entry to `docs/agents-working-context.md`" (medido em todos os 12) | Trocar por "Write a file to `docs/agents-working-context.d/<AAAAMMDDTHHMMSSZ>-<agente>.md`" |
| 4 goldens: `internal/integrations/testdata/architect.agent-directory.golden.md`, `architect.subagent.golden.md`, `backend.agent-directory.golden.md`, `backend.codex-toml.golden.toml` | Goldened sobre o texto dos assets | Devem acompanhar a mudança |
| CLAUDE.md linhas 11, 209 | "Leia `docs/agents-working-context.md`" e "Atualizar `docs/agents-working-context.md`" | Atualizar texto |
| AGENTS.md linhas 11, 59, 153 | Mesmo | Atualizar |
| GEMINI.md linhas 15-16 | Mesmo | Atualizar |
| `internal/integrations/legacy_test.go:16` (`legacyBackendTemplateBytes`) | Fingerprint do template antigo para `agents update` detectar e atualizar | **Não tocar** — é a âncora de detecção do legado; a atualização vem de um novo template padrão |

**Nota de escopo negativo para o corpus pinado**: `internal/roadmapdoc/testdata/corpus` e `scripts/testdata/` não contêm nenhum arquivo `.trackfw-log` (medido: grep sem saída). O `CORPUS_HASH` (`scripts/check-roadmap-barrier-contract.sh:484` = `6a69da211b80...`) é calculado sobre conteúdo de roadmaps, não sobre o log. Nenhuma mudança aqui.

### A4. Contratos pinados em `scripts/`

| Sítio | Contrato | Impacto |
|---|---|---|
| `scripts/check-ref-separator-portability.sh:92-97` | Asserta a linha literal `logBasename = agent + "/" + filepath.Base(src)` em `internal/generators/roadmap.go` | ML-1A deve **preservar** esta linha ou **atualizar o gate**. A lógica de construir o basename por concatenação explícita (nunca `filepath.Join`) continua correta para o novo format |
| `docs/cli-parity.md:218-279` (seção `.gitattributes`) | Contrato de que `merge=union` é gerado byte-idêntico | Seção precisa ser reescrita para descrever o formato por entrada (ML-1A) |
| `docs/cli-parity.md:795-796` | "`trackfw log` reads only the roadmap log, so REQ transitions do not appear" | Deve continuar verdadeiro após a unificação |

---

## Seção B — Modelo de ameaça (ML-0A §2 + §4 + §5)

O adversário aqui é o implementador que fecha o ML sem fechar todos os furos.

### B1. Travessia de diretório

**flat mode** (o modo deste repositório):

`roadmapCandidateFiles` (`roadmap.go:1049-1074`) lê apenas subdirs de estado (`backlog/`, `wip/`, etc.) — `.trackfw-log.d/` fica no root de `roadmap_dir`, nunca é iterado. Medido: `tf validate` num projeto de teste com `.trackfw-log.d/` populated não retornou nenhuma entrada espúria nos estados.

**by_agent mode** — PROBLEMA MEDIDO:

```
$ tf validate  (projeto com roadmap_namespacing: by_agent, agents: [zeus])
⚠  dot-prefixed directory ".trackfw-log.d" found in roadmap_dir, req_dir
   is treated as an agent namespace (fully enumerated, not declared in agents:)
   — declare it in trackfw.yaml if intentional, or remove it if it is
   leftover tooling
```

`resolveAgentNamespaces` (`validator.go:1903-1933`) admite qualquer `IsDir()`. `isInfraDirName` (`validator.go:1968-1970`) filtra só `node_modules`. `.trackfw-log.d/` aciona `agent_namespace_hidden` warning em todos os projetos by_agent após a instalação.

**Fix obrigatório em ML-1A**: adicionar `.trackfw-log.d` a `isInfraDirName`, com justificativa análoga à do `node_modules` (infraestrutura inequívoca, nenhum operador usaria como namespace de agente). Mesma adição para `docs/agents-working-context.d` se ML-2A o cria dentro de `req_dir`/`roadmap_dir` — mas o D2 usa `docs/` direto, fora de `roadmap_dir`, então o risco não existe para ele.

**discover.go** (`internal/discover/discover.go:510-524`): detecta by_agent verificando se subdirs contêm state-dirs. `.trackfw-log.d/` não tem `wip/`, `backlog/` etc. dentro; não aciona a heurística by_agent. Seguro sem alteração.

### B2. `.gitignore` — vetor de silenciamento bloqueante

**Medido** num repositório temporário:

```
echo "*.log" > .gitignore
git check-ignore -v docs/roadmaps/.trackfw-log.d/20261022T100000Z-aabbccdd.log
# .gitignore:1:*.log	docs/roadmaps/.trackfw-log.d/20261022T100000Z-aabbccdd.log  (exit 0)

echo "*.d" > .gitignore
git check-ignore -v docs/roadmaps/.trackfw-log.d
# .gitignore:1:*.d	docs/roadmaps/.trackfw-log.d  (exit 0)
```

`*.log` é padrão comum em templates de gitignore para projetos Node.js e Python (onde `npm debug.log`, `yarn-error.log` etc. são gerados). Se um adotante tiver `*.log` no `.gitignore`, todos os arquivos de `.trackfw-log.d/` são silenciosamente ignorados pelo git — nunca commitados, nunca chegam à main, e o conflito que a feature deveria eliminar volta a existir (os binários antigos continuam escrevendo no `.trackfw-log` legado, que conflita).

O `.gitignore` deste repositório (`/private/tmp/trackfw-toml-escape/.gitignore`) não tem `*.log` nem `*.d`. E `scaffold.go:generateGitIgnore` só emite o padrão `.trackfw-branch-links.json`, não `*.log`.

**Fix obrigatório em ML-1A** (pré-condição do AC6): o `gitAttributesBlock` (ou um segundo bloco de `.gitignore`) deve acrescentar a negação explícita:
```
!<roadmapDir>/.trackfw-log.d/
```
Com o mesmo mecanismo idempotente de append já usado em `generateGitIgnore`. Sem esta negação, a feature quebra silenciosamente em qualquer projeto com `*.log` no gitignore. **Esta lacuna não está na ADR atual** — é um ajuste obrigatório antes de liberar ML-1A.

Alternativa mais simples: mudar a extensão de `.log` para `.txt` (sem precedente de gitignore cobrindo `*.txt` em projetos de software). Esta decisão pertence ao KG.

### B3. FIFO no caminho do log legado

`metrics.go:50`, `metrics_log.go:28` e `log.go:28` usam `os.Open` sem verificação de tipo. Um `mkfifo` no caminho do `.trackfw-log` legado trava os três indefinidamente (comportamento documentado em `regularfile.go` para outros casos análogos já fechados). `validator.go:2874` usa `readRegularFile` e está seguro.

**Fix obrigatório em ML-1A**: o reader unificado deve usar `readRegularFile` (ou `openRegularFileNonblock`) para o arquivo legado. Para cada arquivo dentro de `.d/`, iterar com `os.ReadDir` e chamar `readRegularFile` por entrada. Pular entradas não-regulares sem falhar (log de diagnóstico opcional).

### B4. Nome dos arquivos (ADR D1/D2)

**Colisão no mesmo segundo**: mesma roadmap, mesma transição, mesmo segundo → mesmo nome e mesmo conteúdo → git merge idempotente (medido, Teste 3). Roadmaps diferentes, mesmo segundo → hashes SHA-256 diferentes → nomes diferentes → sem conflito (medido, Teste 2).

**Caracteres proibidos no Windows**: O formato `20261022T100000Z` não contém `:`, `<`, `>`, `"`, `\`, `/`, `|`, `?`, `*`. ADR D1 explicitamente documenta "sem `:` (que o Windows não aceita)". Conforme.

**MAX_PATH**: nome do arquivo = 29 chars (`20261022T100000Z-aabbccdd.log`); caminho completo `docs/roadmaps/.trackfw-log.d/20261022T100000Z-aabbccdd.log` = 58 chars. Com prefixo de projeto de até 200 chars, total < 260 (limite histórico do Windows antes de long paths opt-in). Sem risco prático.

**Symlink no lugar de `.trackfw-log.d/`**: um symlink para fora da árvore no lugar do diretório `.d/` abriria escrita fora do projeto. O writer deve fazer `Lstat` antes de `MkdirAll` e recusar se a entrada não for `ModeDir` pura. A mesma proteção já existe em `pathguard.RejectAndReport` para o arquivo legado.

**Arquivo malformado ou de terceiro em `.d/`**: o reader pula linhas que não casam com `lineRe` (comportamento atual de todos os parsers). Sem risco de crash. Risco de dados forjados: idêntico ao formato atual (qualquer processo com acesso ao FS pode plantar uma linha).

**Ordenação com fuso e linhas do legado com `-0300`**: 454 linhas no log real, 101 com sufixo `-0300` (medido). `lineRe` captura apenas `\d{4}-\d{2}-\d{2} \d{2}:\d{2}`, sem o offset. O sufixo é ignorado. O reader unificado deve:
- Para linhas do legado: parse com `time.Local` (compat com comportamento existente de `metrics.go` e `validator.go`).
- Para entradas de `.d/`: usar o timestamp UTC do nome do arquivo como chave de ordenação (parse: `"20060102T150405Z"` → `time.UTC`).
- Desempate: nome do arquivo (string sort, que é cronológico dado UTC + padding fixo).

**`<h8>` para D2 (contexto de trabalho)**: Os agentes são LLMs que não podem computar SHA-256 sem ferramentas. Para D1 (log de transições), o `<h8>` é calculado pelo código Go (não pelo agente). Para D2, o conteúdo é um bloco multi-linha gerado pelo agente. **Amendamento obrigatório na ADR para D2**: o nome do arquivo deve ser `<AAAAMMDDTHHMMSSZ>-<agente>.md` (sem hash), pois nenhum agente pode computar SHA-256 no ato de escrever. Dois arquivos do mesmo agente no mesmo segundo são improvavéis, e a colisão de conteúdo é exatamente o caso idempotente desejado. Se unicidade extra for necessária, um nonce aleatório curto (não SHA-256) é mais viável para LLMs.

### B5. Contexto de trabalho — leitura pelos agentes (ML-0A §5)

`trackfw context` (`internal/commands/context.go:8-21`, `internal/generators/context.go:34`) hoje não lê `docs/agents-working-context.md`. O arquivo é lido apenas pelos agentes que seguem as instruções geradas.

**Recomendação**: estender `trackfw context` para incluir as N entradas mais recentes de `docs/agents-working-context.d/` na saída (lidas por nome de arquivo, últimas N por sort lexicográfico = cronológico por UTC). Sem novo subcomando, sem nova flag obrigatória. Um `--no-context-log` pode suprimir se for barulhento. Esta decisão pertence ao KG; o ML-2A deve registrar a escolha e implementá-la.

**O que muda no CLAUDE.md do projeto**:
- Linha 11: `"Leia docs/agents-working-context.md"` → `"Leia as entradas mais recentes de docs/agents-working-context.d/"`
- Linha 209: `"Atualizar docs/agents-working-context.md"` → `"Escreva um arquivo em docs/agents-working-context.d/<AAAAMMDDTHHMMSSZ>-<agente>.md"`
- As mesmas linhas em AGENTS.md e GEMINI.md.
- A instrução deve especificar N (sugestão: 10 entradas mais recentes) para evitar que agentes leiam 47k linhas.

---

## Seção C — Alvos de falsificação nas duas direções (ML-0A §6)

### C1. Falsificação medida — repositório temporário sem `.gitattributes`

Medido em `/private/tmp/claude-501/.../scratchpad/w0-log/gitconflict-test2/` (repositório git temporário sem `.gitattributes`):

**Teste 1 — Formato antigo conflita:**
```
Branch A: printf "..  ROADMAP-A.md  backlog → wip\n" >> .trackfw-log
Branch B: printf "..  ROADMAP-B.md  backlog → wip\n" >> .trackfw-log
git merge branch-b
# CONFLICT (content): Merge conflict in docs/roadmaps/.trackfw-log
# Automatic merge failed  ← PASS: comportamento esperado
```

**Teste 2 — Formato novo (arquivo por entrada) não conflita:**
```
Branch C: cria 20261022T100000Z-f92eaf46.log (ROADMAP-C)
Branch D: cria 20261022T100000Z-8cd8741f.log (ROADMAP-D)
git merge branch-d → "Merge made by the 'ort' strategy" (rc=0)  ← PASS
Entradas em .d/: dois arquivos, conteúdos corretos
```

**Teste 3 — Entrada idêntica nas duas branches (idempotente):**
```
Branches E e F: criam o mesmo 20261022T100000Z-<mesmo_hash>.log com mesmo conteúdo
git merge branch-f → "Merge made by the 'ort' strategy" (rc=0)  ← PASS
```

Todos os três testes passaram sem `.gitattributes`. D3 da ADR é cumprível.

### C2. Falsificação em sentido contrário — `.gitignore` com `*.log`

Medido no mesmo repositório temporário:
```
echo "*.log" > .gitignore
git check-ignore -v docs/roadmaps/.trackfw-log.d/20261022T100000Z-aabbccdd.log
# exit 0 — arquivo ignorado pelo git
```

Resultado: se o projeto tiver `*.log` no `.gitignore`, o merge limpo do Teste 2 é uma ilusão — os arquivos existem localmente mas nunca são commitados. O conflito no forge volta a ocorrer porque os binários antigos continuam appendando no `.trackfw-log` legado e o formato novo é invisível para o git. **Esta é a falsificação do AC6 que ML-1A deve endereçar** (ver B2 acima).

### C3. Falsificação by_agent: aviso espúrio

Medido com `tf validate` em modo by_agent:
```
⚠  dot-prefixed directory ".trackfw-log.d" found in roadmap_dir...
   is treated as an agent namespace
```
Este aviso não é um falso positivo de conflito de conteúdo, mas é um sinal errado que polui a saída do `validate` e pode fazer o adotante remover o `.d/` manualmente, quebrando o mecanismo. Requer correção em `isInfraDirName` (ver B1).

---

## Seção D — Resíduo declarado (ML-0A §4 + §6 + §5)

O que este design aceita não cobrir, dito com precisão:

1. **Janela de transição das instruções instaladas**: Agentes com cópias instaladas dos assets (via `trackfw agents install`) continuam mandando escrever no `.md` até um `trackfw agents update` após a release. Durante essa janela, conflitos no `docs/agents-working-context.md` continuam para agentes que usam instruções antigas. Isso não é regressão — é o estado pré-existente. Declarado em ADR.Consequences.

2. **`merge=union` permanece para o legado**: `scaffold.go:2053` continua gerando `.trackfw-log merge=union`. Isso protege o arquivo legado durante a transição (binários antigos ainda escrevem nele). Comentário deve ser atualizado para dizer que a regra não funciona no forge e por quê (ADR D1).

3. **D2 sem hash**: A ADR atual especifica `<h8>` para D2. LLMs não computam SHA-256 sem ferramentas. A ADR deve ser emendada para D2 usar `<AAAAMMDDTHHMMSSZ>-<agente>.md` sem hash. Esta mudança não afeta D1 (onde o hash é computado pelo código Go).

4. **Ordenação do legado permanece imprecisa**: 353 das 454 linhas do legado têm precisão de minuto sem UTC offset reconhecido pelo parser (são parsadas em `time.Local`). O reader unificado não melhora nem piora essa precisão. A ordenação das entradas antigas continua sendo heurística para timestamps no mesmo minuto.

5. **Arquivo forjado dentro de `.d/`**: qualquer processo com acesso ao FS pode criar `<dir>/.trackfw-log.d/<qualquer_nome>.log` com conteúdo de log plausível. Esse risco é idêntico ao do formato atual (append no arquivo). O controle é o mesmo: `pathguard.RejectAndReport` na escrita, read-only para os leitores.

6. **`commands/metrics.go:20` hardcoded path bug**: pré-existente; ML-1A deve corrigir como parte da unificação dos leitores (mesma causa, mesma REQ).

---

## Lista de arquivos por ML — resultado desta Wave 0

### ML-1A — apolo-tf (log por entrada, D1)

Arquivos afetados (lista final):

| Arquivo | Operação |
|---|---|
| `internal/generators/roadmap.go` (writer `appendTransitionLogEntry`) | Mudar de O_APPEND para criação de arquivo por entrada |
| `internal/generators/req.go` (writer `appendREQTransitionLogEntry`) | Idem |
| `internal/commands/discover.go` (`--bootstrap-log`) | Mudar escrita para `.d/` |
| `internal/metrics/metrics.go` | Eliminar parser; usar reader unificado (novo pacote ou função em `metrics`) |
| `internal/serve/metrics_log.go` | Eliminar parser; usar reader unificado |
| `internal/serve/api_metrics.go` | Atualizar chamada para reader unificado |
| `internal/validator/validator.go:2873-2895` | Atualizar para reader unificado |
| `internal/commands/log.go` | Atualizar para reader unificado |
| `internal/commands/metrics.go:20` | Fix hardcoded path + usar reader unificado |
| `internal/discover/discover.go:543` | Atualizar `HasTrackfwLog` para também verificar `.d/` |
| `internal/commands/discover.go:108-110` | Atualizar mensagem de UI |
| `internal/generators/scaffold.go:2053` (gitAttributesBlock) | Atualizar comentário; adicionar `.gitignore` block para `!<roadmapDir>/.trackfw-log.d/` |
| `internal/validator/validator.go:1968-1970` (`isInfraDirName`) | Adicionar `.trackfw-log.d` |
| `docs/cli-parity.md:218-279` | Reescrever seção `.gitattributes` para descrever formato por entrada |
| `docs/cli-parity.md:795-796` | Atualizar referência a `.trackfw-log` |
| `scripts/check-ref-separator-portability.sh:92-97` | Atualizar ou confirmar que o assert ainda vale pós-mudança |
| Testes existentes que mockam `.trackfw-log` | Atualizar para `.d/` |

### ML-1B — ares-tf (gates de encoding, AC1–AC4)

Arquivos afetados (sem mudança desta Wave 0):
- `scripts/check-roadmap-barrier-contract.sh` (`:612` `errors="replace"`)
- `scripts/check-output-encoding-declared.sh` (`:113-127` allowlist, `:163-168` DECL_RE)

### ML-2A — prometeu-tf (contexto de trabalho por entrada, D2)

Arquivos afetados:
- `internal/generators/agentfiles.go:92-93`
- 12 assets em `internal/integrations/assets/agents/*.md`
- 4 goldens em `internal/integrations/testdata/`
- `internal/generators/context.go` (estender `trackfw context` para ler `.d/`)
- `CLAUDE.md` linhas 11, 209 (e seção `## Working context` do `trackfw_architect` no projeto)
- `AGENTS.md` linhas 11, 59, 153
- `GEMINI.md` linhas 15-16
- `docs/agents-working-context.md` (adicionar cabeçalho de redirecionamento)
- **NÃO tocar**: `internal/integrations/legacy_test.go:16` (`legacyBackendTemplateBytes`) — é âncora de detecção de legado para `agents update`

---

## Ajustes necessários na ADR antes de liberar ML-1A

1. **D1 — `.gitignore`**: Acrescentar: "O `scaffold.go` emite também um bloco de `.gitignore` com `!<roadmapDir>/.trackfw-log.d/` para garantir que padrões `*.log` presentes no projeto do adotante não silencem os arquivos de entrada."

2. **D1 — `isInfraDirName`**: Acrescentar: "O validator em modo `by_agent` deve reconhecer `.trackfw-log.d` como infraestrutura (filtrado por `isInfraDirName`) para não acionar `agent_namespace_hidden` em projetos que o adotam."

3. **D2 — formato do nome do arquivo**: Substituir `<AAAAMMDDTHHMMSSZ>-<agente>-<h8>.md` por `<AAAAMMDDTHHMMSSZ>-<agente>.md`. Justificativa: LLMs não computam SHA-256 sem ferramentas; a colisão de conteúdo (mesmo agente, mesmo segundo) é o caso idempotente desejado e git a trata como "mesmo arquivo".

---

## Veredito

**LIBERADO COM AJUSTES OBRIGATÓRIOS NA ADR (três itens acima) ANTES DO INÍCIO DO ML-1A.**

O mecanismo central (arquivo por entrada → sem conflito de merge) está confirmado por medição. Os três ajustes são pré-requisitos de correção, não melhorias opcionais: sem o item 1 (gitignore), o AC6 pode falhar silenciosamente em metade dos projetos adotantes; sem o item 2 (by_agent), todo projeto by_agent gera um aviso espúrio após a migração; sem o item 3 (D2 hash), o ML-2A não pode ser implementado conforme a ADR. Nenhum deles exige nova onda — todos cabem como adendos à ADR atual antes do handoff ao apolo-tf.
