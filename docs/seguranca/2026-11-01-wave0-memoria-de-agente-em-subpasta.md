# Wave 0 — Threat Model: Memória de Agente em Subpastas

> Roadmap: ROADMAP-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
> REQ: REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
> Issue: #646
> Analista: hades-tf | 2026-11-01

---

## 1. Completude da Enumeração

### 1.1 Assets de agente (`internal/integrations/assets/agents/`)

Todos os 12 arquivos têm `memory: project` no frontmatter. Nenhum tem instrução de caminho de memória no corpo.

```
grep -rn "memory:" internal/integrations/assets/agents/*.md
# 12 linhas: "memory: project" na linha 5 de cada arquivo

grep -rn "agent-memory\|rev-parse\|show-toplevel" internal/integrations/assets/agents/*.md
# (vazio) — GAP confirmado para AC1
```

Arquivo | `memory: project` | Instrução AC1 no corpo
---|---|---
architect.md | sim | NÃO
backend.md | sim | NÃO
code-quality.md | sim | NÃO
data.md | sim | NÃO
dba.md | sim | NÃO
frontend.md | sim | NÃO
iac.md | sim | NÃO
infra.md | sim | NÃO
qa.md | sim | NÃO
security.md | sim | NÃO
tooling.md | sim | NÃO
ux.md | sim | NÃO

### 1.2 Golden files (`internal/integrations/testdata/`)

```
ls internal/integrations/testdata/
# architect.agent-directory.golden.md   ← sem memory:
# architect.subagent.golden.md          ← memory: project (destino claude/cli)
# backend.agent-directory.golden.md     ← sem memory:
# backend.codex-toml.golden.toml        ← sem memory:
```

Arquivos que o PR precisa tocar:
- `architect.subagent.golden.md`: atualizar com o texto AC1 injetado (ver §1.3).
- `architect.agent-directory.golden.md`: atualizar se o body de `architect.md` mudar para AC2 (ver P-3-R e §2a).

Arquivos que o PR **não** toca: `backend.*` (sem `memory:`), `architect.agent-directory.golden.md` apenas se AC2 for render-time.

### 1.3 Renderização por destino (`internal/integrations/render.go`)

Tabulação destino → representação → mantém `memory:` (fonte: `assets/catalog.json`):

Destino/Superfície | Representação | Mantém `memory:`? | Interpreta `memory:`
---|---|---|---
claude/cli | subagent | **SIM** (passa source inteiro) | **SIM** (Claude Code)
gemini/cli | agent-markdown | SIM (passa source inteiro) | NÃO (ignora)
cursor/ide | agent-markdown | SIM (passa source inteiro) | NÃO (ignora)
kiro/ide | agent-markdown | SIM (passa source inteiro) | NÃO (ignora)
antigravity/current | agent-directory | NÃO (reconstrói frontmatter, omite) | —
antigravity/legacy-cli | agent-json | NÃO (body em JSON, omite) | —
codex/cli | custom-agent-toml | NÃO (body em `developer_instructions`) | —
opencode/cli | opencode-agent | NÃO (descarta explicitamente, render.go:394) | —
amazonq/cli | cli-agent-json | NÃO (body em JSON) | —
kiro/cli | agent-json | NÃO (body em JSON) | —

**Conclusão crítica para AC1:** somente `claude/cli` interpreta `memory: project`. Mas o corpo do asset é passado **para todos** os targets — codex inclui o body em `developer_instructions`, opencode e agent-directory incluem o body após o frontmatter reconstruído. Se o texto AC1 for colocado no corpo do asset, ele chegará a Codex, OpenCode e Antigravity, contrariando o escopo negativo ("Não toca os destinos sem `memory:`").

**Sítio correto para AC1: `render.go`, bloco `default`, condicionado a `targetID == "claude"`** — body é manipulado ali para o modelo pinado; a instrução AC1 deve ser concatenada ao body no mesmo bloco. Golden files de `codex`, `opencode-agent` e `agent-directory` devem afirmar AUSÊNCIA do texto.

Para AC2 (instrução de despacho no arquiteto): essa instrução é comportamental e não menciona paths de memória. A REQ não a proíbe para outros destinos; contudo, qualquer mudança no body de `architect.md` afeta `architect.agent-directory.golden.md`. O PR deve listar esse arquivo como afetado. Ver P-3.

### 1.4 `generateGitIgnore` (`internal/generators/scaffold.go:2291`)

```
grep -rn "generateGitIgnore(" internal/ --include="*.go" | grep -v "_test.go"
# internal/generators/scaffold.go:145
# internal/generators/update.go:115
```

Arquivo de teste existente: `internal/generators/gitignore_test.go` — cobre os três ramos da função atual (arquivo ausente, sem regra, com regra). O PR deve adicionar casos de teste para a nova regra do `agent-memory`.

Regras atuais gerenciadas: `<roadmap_dir>/.trackfw-branch-links.json` e `.trackfw/leases/`. Regra para `agent-memory`: ausente (GAP para AC3).

### 1.5 `doctor` (`internal/commands/doctor.go`, `runDoctor`)

```
grep -n "WalkDir\|filepath.Walk\|os.ReadDir\|Lstat" internal/commands/doctor.go
# (vazio) — sem varredura de árvore
```

O `runDoctor` atual é puramente baseado em catálogo. AC4 exige nova varredura. `manager.ProjectRoot` vem de `os.Getwd()` (`integrations_flags.go:437`). Se o doctor for chamado com cwd em `scripts/`, a varredura partiria de `scripts/`, não da raiz do repositório — falso negativo para stray dirs fora de `scripts/`. A varredura deve obter a raiz via `git rev-parse --show-toplevel`, independentemente de `manager.ProjectRoot`.

`printDoctorReport` (`doctor.go:157`) tem format string com 12 kinds explícitos; um novo kind `stray-agent-memory` requer acréscimo nessa linha e em `scripts/check-doctor-parity.sh`.

Doctor exit code: `RunE` retorna `nil` quando há findings (só imprime); exit 0 com findings. A nova modalidade de "aviso" não quebrará CI dos consumidores.

### 1.6 `buildMLScopeOrchestrationSet` (`internal/commands/barrier.go:1038`)

Pattern `.claude/agent-memory/` na linha 1047, matching por `strings.HasPrefix`. Comportamento:

```
# Caminho stray: não começa com ".claude/agent-memory/"
scripts/.claude/agent-memory/zeus-tf/note.md → FORA DO ESCOPO (correto)

# Caminho aninhado: começa com ".claude/agent-memory/"
.claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md → DENTRO DO ESCOPO (gap pré-existente)
```

`docs/cli-parity.md:3241` documenta `.claude/agent-memory/` no conjunto de orquestração.

### 1.7 Outros sítios

```
grep -rn "agent-memory" internal/ | grep -v "barrier.go:1047\|roadmapdoc\|testdata"
# internal/integrations/render_test.go:520,556,1492 — testes que afirmam memory: AUSENTE em opencode/agent-directory
# internal/commands/agents_skills_test.go:137 — memória ausente em alguns destinos
```

```
grep -rn "memory:" docs/ scripts/ | grep -v "docs/agents-working-context\|docs/roadmaps\|docs/req\|scripts/testdata\|docs/adr\|docs/analises"
# docs/cli-parity.md:1123 — opencode descarta memory: (confirma §1.3)
# docs/cli-parity.md:3241 — barrier orchestration set documenta .claude/agent-memory/
```

Nenhum script em `scripts/` gera ou referencia `agent-memory` além de fixtures de teste.

**Lista fechada de sítios para o PR:**

1. `internal/integrations/render.go` — AC1: injetar texto no bloco `targetID == "claude"` do default case
2. `internal/integrations/assets/agents/architect.md` — AC2: adicionar instrução de despacho no body
3. `internal/integrations/testdata/architect.subagent.golden.md` — atualizar com texto AC1
4. `internal/integrations/testdata/architect.agent-directory.golden.md` — atualizar se AC2 muda o body (ver P-3)
5. `internal/generators/scaffold.go:generateGitIgnore` e nova `hasGitIgnoreAgentMemoryRule` — AC3
6. `internal/generators/gitignore_test.go` — novos casos de teste para AC3
7. `internal/commands/doctor.go:runDoctor` — AC4: nova varredura
8. `internal/commands/doctor.go:printDoctorReport:157` — novo kind para summary line
9. `scripts/check-doctor-parity.sh` — novo kind no parity check
10. `.gitignore` do repositório trackfw — AC5: regra nova
11. `pypi/.claude/` (todo o diretório) — AC5: remover
12. `docs/roadmaps/wip/.claude/` (todo o diretório) — AC5: remover
13. `.claude/agent-memory/zeus-tf/.claude/` (diretório aninhado dentro da raiz legítima) — AC5: remover

---

## 2. Modelo de Ameaça

### 2a. AC1: instrução de prompt contradiz o caminho injetado pelo harness

**Mecanismo:** O Claude Code injeta "Persistent Agent Memory" **após** o corpo do agente, com path absoluto e instrução "write directly, don't check." Evidência primária: meu próprio system prompt como hades-tf mostra a seção após o corpo, com:

> "This directory already exists — write to it directly... MEMORY.md is always loaded into your conversation context"

**Qual vence:** o harness. Por posição (posterior = mais recente), especificidade (path absoluto vs. instrução genérica) e a formulação "write directly." A instrução AC1 no body é uma diretriz de fallback, não uma garantia.

**Lado de leitura (incontornável):** a seção do harness diz que `MEMORY.md is always loaded` — esse carregamento ocorre no momento do dispatch, com o path do cwd de dispatch. Se o agente escrever na raiz correta (seguindo AC1), mas o MEMORY.md do path errado estiver carregado, ele lê memória vazia ou desatualizada. **Esse comportamento é do harness e não pode ser corrigido por instrução no body.** P-2 abaixo prescreve um mitigador pelo lado de leitura (leitura explícita do MEMORY.md correto via Read tool), que deve ser declarado como **emenda ao AC1 da REQ** (amplia o escopo além do texto original).

**`<nome-do-agente>` no texto:** o texto AC1 não deve fixar o nome do agente nem pedir `git rev-parse` para descobri-lo — o nome renderizado está no próprio frontmatter e o harness já embute no path injetado. O texto deve instruir o agente a verificar se o path injetado está sob `$(git rev-parse --show-toplevel)`, e se não estiver, substituir o prefixo.

**Auditabilidade:** não é mensurável por teste automático que o agente vai obedecer AC1. Auditável só se o texto está presente no golden file `architect.subagent.golden.md` para `claude` e AUSENTE nos outros.

### 2b. AC3: padrão de `.gitignore`

**Medições** (scratchpad `w0-646/repo1`, `git check-ignore -v`):

Padrão | Raiz `.claude/agent-memory/zeus-tf/user_role.md` | `lib/.claude/agent-memory/x/note.md` | `deep/sub/.claude/agent-memory/x/note.md` | `.claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md`
---|---|---|---|---
`**/.claude/agent-memory/` | **IGNORADO** (errado) | IGNORADO ✓ | IGNORADO ✓ | IGNORADO ✓
`/.claude/agent-memory/` | IGNORADO (errado) | **NÃO IGNORADO** (errado) | **NÃO IGNORADO** (errado) | NÃO IGNORADO (errado)
`*/**/.claude/agent-memory/` | **NÃO IGNORADO** ✓ | IGNORADO ✓ | IGNORADO ✓ | IGNORADO ✓

**Padrão correto: `*/**/.claude/agent-memory/`** — requer ao menos um componente de caminho antes de `.claude/`.

**Interação com memória raiz versionada (143 arquivos):** verificado com `--no-index`. Arquivos rastreados são imunes a `.gitignore` (gitignore só afeta untracked). Confirmado: `echo ".claude/agent-memory/hades-tf/user.md" | git check-ignore --no-index --stdin -v` → "NOT IGNORED." Verificado também que nenhum arquivo com `/\.claude/agent-memory/` em subpath está rastreado (`git ls-files | grep '/\.claude/agent-memory/'` → vazio).

**Idempotência:** `hasGitIgnoreRule` atual usa `path.Base(pattern) == gitIgnoreRuleTarget`. Para `agent-memory`, `path.Base("*/**/.claude/agent-memory/")` = `"agent-memory"` — mas também casaria com `# */**/.claude/agent-memory/` (comentário), `lib/.claude/agent-memory/` e `**/.claude/agent-memory/`. A função `hasGitIgnoreAgentMemoryRule` deve:
- Iterar linha por linha
- `TrimSpace`, pular se vazio ou se começa com `#`
- Pular se começa com `!` (negação = decisão explícita do projeto, não tocar)
- Comparar igualdade exata com `*/**/.claude/agent-memory/`

Residual de migração: se projeto já tiver `**/.claude/agent-memory/` (padrão errado), a nova função não o reconhece como "correto" e faz append. Dois padrões no arquivo. O comportamento final fica dominado pelo mais restritivo existente — a raiz continua ignorada. Correção manual necessária.

### 2c. Doctor varrendo árvore grande

**Comportamento de `filepath.WalkDir` medido** (scratchpad `w0-646/walktest`):

Cenário | Comportamento observado
---|---
Diretório chmod 000 | `ERROR at <path>: permission denied`, walk continua (return nil no callback)
Symlink de diretório (`symlink_to_etc`) | `d.Type() & fs.ModeSymlink != 0`, IsDir() = false — não traversado
Laço de symlink | Inaplicável: WalkDir usa Lstat, não segue symlinks de diretório
`.git/` sem skip | Traversado (lento). Deve ser skipado via `filepath.SkipDir`
`node_modules/` sem skip | Traversado (muito lento em monorepos). Deve ser skipado
FIFO dentro de stray dir | `os.ReadDir` lista sem abrir → não bloqueia

**Contagem de arquivos no diretório stray:** `os.ReadDir("stray/agent-memory/")` retorna entradas incluindo subdiretórios. Os três strays têm `zeus-tf/` como única entrada — 1 entrada de diretório, 0 arquivos. Uma contagem simples de `ReadDir` reportaria 1 entry e marcaria erroneamente como "memória perdida." A contagem deve ser **recursiva sobre entradas não-diretório** (WalkDir dentro do stray), nunca abrindo arquivos.

```
# Verificação no repositório atual:
find pypi/.claude/agent-memory -not -type d | wc -l  → 0
find docs/roadmaps/wip/.claude/agent-memory -not -type d | wc -l  → 0
find .claude/agent-memory/zeus-tf/.claude/agent-memory -not -type d | wc -l  → 0
```

**Root definition — risco de false negative:** `integrationsManager().ProjectRoot = os.Getwd()`. Se `trackfw doctor` for executado de `scripts/`, `ProjectRoot = scripts/` e a varredura partiria de `scripts/`, perdendo `pypi/.claude/agent-memory/` (stray fora de `scripts/`). A varredura deve resolver a raiz git via `git rev-parse --show-toplevel` independentemente de `ProjectRoot`.

**Nested repo — falso positivo:** `git rev-parse --show-toplevel` em `outer/nested/x/` retorna `outer/nested/` (medido no scratchpad). Um agente operando em submódulo tem `outer/nested/.claude/agent-memory/` como raiz correta. O doctor do repositório `outer/` a reportaria como stray. A detecção de nested repo deve checar se o pai de `.claude/` é ele mesmo raiz de um repositório git: `os.Lstat(filepath.Join(dir, ".git"))` exists (qualquer tipo — é arquivo em submódulos).

### 2d. Barrier: diretório de memória em subpasta

O barrier usa `strings.HasPrefix(filePath, ".claude/agent-memory/")` para o pattern `".claude/agent-memory/"`:

- `scripts/.claude/agent-memory/zeus-tf/note.md` → NÃO começa com `.claude/agent-memory/` → FORA DO ESCOPO. Correto: stray aparece como out-of-scope no diff, sinaliza o problema antes do commit.
- `.claude/agent-memory/zeus-tf/.claude/agent-memory/nested/note.md` → COMEÇA com `.claude/agent-memory/` → DENTRO DO ESCOPO. Falso positivo de segurança (aceita stray aninhado). Pré-existente, não agrava o defeito do AC3.

**Desejabilidade:** sim. O barrier é segunda linha de defesa. Stray em subpasta travaria o `trackfw barrier` por out-of-scope, alertando antes de qualquer commit. Não é substituto do doctor.

**Interação com AC3:** após gitignore aplicado, stray não entra no índice → não aparece em `git diff HEAD` → barrier não o vê. O barrier é efetivo somente enquanto AC3 não estiver em vigor no projeto do consumidor.

---

## 3. Alvos de Falsificação nos Dois Sentidos

### AC1 — Instrução de caminho no output renderizado para `claude`

Falso negativo (proteção falha) | Falso positivo (quebra uso legítimo)
---|---
Texto AC1 não injetado no bloco `targetID == "claude"` de `render.go` | Texto AC1 presente em golden files de codex/opencode/agent-directory
Harness vence por recência → agente ignora instrução e segue cwd injected | Instrução pede `git rev-parse` em cwd errado → resolve inner root em nested repo
Lado de leitura não corrigido → memória escrita na raiz nunca lida (residual declarado) | —

**Como medir:** golden file `architect.subagent.golden.md` deve conter o texto; golden files de `codex-toml` e `architect.agent-directory` NÃO devem.

### AC2 — Instrução de despacho no arquiteto

Falso negativo | Falso positivo
---|---
Texto AC2 ausente de `architect.md` ou do output renderizado | AC2 em body de `architect.md` → `architect.agent-directory.golden.md` falha se não atualizado

**Como medir:** grep em `architect.md` instalado; golden de `subagent` e de `agent-directory` atualizados.

### AC3 — Regra de `.gitignore`

Falso negativo (stray entra no índice) | Falso positivo (memória raiz ignorada)
---|---
Padrão emitido diferente de `*/**/.claude/agent-memory/` | `*/**/.claude/agent-memory/` alterado para `**/.claude/agent-memory/` por erro de typo
`hasGitIgnoreAgentMemoryRule` usa basename, aceita padrão errado como "presente" | Linha emitida duplicada por bug de idempotência — harmless mas confuso
Projeto com `**/.claude/agent-memory/` pré-existente — função detecta como presente (residual) | —
`generateGitIgnore` com cwd errado escreve `.gitignore` no lugar errado | —

**Como medir:** `git check-ignore -v` nos 4 casos da REQ. Teste de idempotência (2ª execução no-op). Teste com regra errada pré-existente (append documentado como residual).

### AC4 — Doctor lista stray agent-memory dirs

Falso negativo (stray não reportado) | Falso positivo (legítimo reportado como stray)
---|---
Walk parte de `os.Getwd()` em vez de `git rev-parse --show-toplevel` | Nested repo: `outer/nested/.claude/agent-memory/` é raiz legítima para agentes de `nested/`
Walk sem skip de `.git` → `.git/modules/X/.claude/agent-memory/` reportado (FP ou FN) | Worktree adicional: `outer/.git/worktrees/wt/.claude/agent-memory/` — FP
Contagem com `ReadDir` (inclui subdiretórios) → stray vazio reportado como "memória perdida" | —
Stray dentro de nested repo com `.git` como arquivo (submódulo) — detecção errada | —

**Como medir:** teste com árvore sintética: stray plano, nested repo (`.git/` dir e `.git` arquivo), symlink, chmod-000.

### AC5 — Repositório trackfw com stray removidos

Falso negativo | Falso positivo
---|---
Remoção de `zeus-tf/` mas não do pai `.claude/` → doctor reporta `.claude/agent-memory/` vazia | Regra nova ignora root `.claude/agent-memory/` — verificado: NÃO ocorre com `*/**/.claude/agent-memory/`
Novo stray criado após remoção (sem gitignore em vigor) | —

**Como medir:** `find . -path "*/node_modules" -prune -o -path "*/.git" -prune -o -type d -name "agent-memory" -print` → apenas `./.claude/agent-memory`. `git check-ignore --no-index .claude/agent-memory/hades-tf/user.md` → NOT IGNORED.

### AC6 — `make quality` e CI

Falso negativo | Falso positivo
---|---
Golden files não atualizados → tests falham | Novo kind de finding em `printDoctorReport` não adicionado em `check-doctor-parity.sh` — CI falha

---

## 4. Residual Declarado

1. **Lado de leitura do harness (AC1):** `MEMORY.md is always loaded` do path do harness. Nenhuma instrução no body pode corrigir a leitura automática. A emenda proposta em P-2 (Read explícita do MEMORY.md correto) mitiga mas não elimina o problema — requer que o agente execute uma Read extra no início de cada sessão.

2. **Instrução AC1 não verificável em execução:** não há como testar automaticamente se o agente vai seguir AC1 em vez do harness. Auditável apenas pela presença do texto no golden file.

3. **Nested repositories:** agentes em submódulos têm raiz legítima em `inner/.claude/agent-memory/`. O doctor do repositório pai os reportaria como stray. AC4 deve excluir nested repos, mas essa lógica (`os.Lstat(<dir>/.git) exists`) não está especificada na REQ. Declarado como residual de escopo do PR atual.

4. **Migração de padrão errado em `.gitignore`:** projetos com `**/.claude/agent-memory/` pré-existente recebem append de `*/**/.claude/agent-memory/` em vez de substituição. Dois padrões coexistem; o comportamento final fica dominado pelo mais restritivo existente (raiz continua ignorada). Correção manual necessária.

5. **Windows junctions:** `go.mod >= 1.23` → `ModeIrregular` em vez de `ModeSymlink` para junctions (ver nota de vault `project_winsymlink_gomod_junction_mode`). O callback do WalkDir deve checar `ModeSymlink|ModeIrregular`.

6. **Barrier — nested path:** `.claude/agent-memory/zeus-tf/.claude/agent-memory/...` aceito como in-scope pelo barrier (prefixo match). Pré-existente.

---

## Prescrições

**P-1 (AC1, BLOQUEANTE):** Injetar o texto AC1 em `render.go`, no bloco `default`, condicionado a `targetID == "claude"` — não no corpo do asset. O texto instrui o agente a (a) verificar se o path injetado pelo harness está sob `$(git rev-parse --show-toplevel)` e, se não estiver, substituir o prefixo pela raiz git. Adicionar golden counter-arm: texto presente em `architect.subagent.golden.md`; texto AUSENTE em `architect.agent-directory.golden.md`, `backend.codex-toml.golden.toml` e qualquer output de opencode.

**P-2 (AC1, emenda ao AC1 da REQ, BLOQUEANTE):** O texto AC1 deve cobrir também o lado de leitura: instruir o agente a, quando detectar que o path do harness não está sob a raiz git, executar `Read(<root>/.claude/agent-memory/<nome>/MEMORY.md)` explicitamente no início da sessão. O arquiteto deve aprovar essa emenda antes de Wave 1, pois amplia o AC1 da REQ.

**P-3 (AC2, BLOQUEANTE com observação):** Adicionar instrução de despacho no body de `assets/agents/architect.md`. Como o body vai para TODOS os destinos, `architect.agent-directory.golden.md` deve ser atualizado. Isso é aceitável: a instrução é comportamental (não menciona `memory:`), logo não viola o escopo negativo. O PR deve listar `architect.agent-directory.golden.md` como arquivo afetado.

**P-4 (AC3, BLOQUEANTE):** Em `generateGitIgnore`: adicionar bloco para `*/**/.claude/agent-memory/`, com função `hasGitIgnoreAgentMemoryRule` de correspondência exata por linha: `TrimSpace` → pular linha vazia ou iniciada com `#` → pular iniciada com `!` (decisão do projeto) → comparar igualdade com `"*/**/.claude/agent-memory/"`. Documentar residual: padrão errado pré-existente não é substituído, apenas coexiste.

**P-5 (AC4, BLOQUEANTE):** O `runDoctor` deve chamar nova função que: (a) obtém a raiz via `git rev-parse --show-toplevel`, não via `manager.ProjectRoot`; (b) usa `filepath.WalkDir` com skip explícito de `.git` e `node_modules` via `filepath.SkipDir`; (c) retorna `nil` (continue) em erros de permissão; (d) identifica `d.Name() == "agent-memory" && d.IsDir()` com pai nomeado `.claude` não na raiz; (e) exclui nested repos: verificar `os.Lstat(filepath.Join(parentDir, ".git"))` — se existir (qualquer tipo, incluindo arquivo de submódulo), não é stray; (f) conta arquivos recursivamente dentro do stray via WalkDir interno, nunca abrindo entradas; (g) usa `d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0` para detectar symlinks/junctions. O novo kind deve ser adicionado ao format string em `printDoctorReport` e em `scripts/check-doctor-parity.sh`.

**P-6 (AC5, BLOQUEANTE):** PR deve remover integralmente: `pypi/.claude/` (contém apenas `agent-memory/`), `docs/roadmaps/wip/.claude/` (contém apenas `agent-memory/`), `.claude/agent-memory/zeus-tf/.claude/` (contém apenas `agent-memory/zeus-tf/`). Adicionar `*/**/.claude/agent-memory/` ao `.gitignore` do trackfw. Confirmar que `find . -path "*/node_modules" -prune -o -path "*/.git" -prune -o -type d -name "agent-memory" -print` retorna apenas `./.claude/agent-memory`.

**P-7 (AC3, testes de idempotência):** `gitignore_test.go` deve incluir: (a) arquivo sem regra → regra adicionada; (b) regra correta presente → no-op; (c) regra errada `**/.claude/agent-memory/` presente → append (documentado como residual); (d) segunda execução → no-op. Cada caso verifica o conteúdo final do arquivo com `git check-ignore -v` nos 4 vetores da REQ.

**P-8 (barrier, não-bloqueante, informativo):** O gap de matching aninhado (`.claude/agent-memory/zeus-tf/.claude/agent-memory/...` aceito como in-scope) é pré-existente. Sem correção neste PR. `docs/cli-parity.md:3241` já documenta `.claude/agent-memory/` no conjunto de orquestração.
