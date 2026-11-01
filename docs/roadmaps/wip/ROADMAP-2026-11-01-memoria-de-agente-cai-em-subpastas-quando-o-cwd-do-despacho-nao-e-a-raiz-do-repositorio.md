---
status: wip
date: 2026-11-01
req: "docs/req/REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Memória de agente cai em subpastas quando o cwd do despacho não é a raiz do repositório

> Created: 2026-11-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
Issue: #646

Medição do arquiteto (2026-11-01, detalhada na REQ): o Claude Code cria
`<cwd de quem despacha>/.claude/agent-memory/<agente>/` vazio no ato do despacho e injeta esse caminho
no prompt do agente; a sessão principal segue o mesmo cwd. O `cd` do agente não é o mecanismo.

Varredura de issues (2026-11-01): abertos #646 e #649; só o #646 trata do mecanismo.

## Acceptance Criteria
- [ ] AC1–AC7 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-01. P-1 a P-8 aceitas, com duas correções do arquiteto: (1) P-5 cita `scripts/check-doctor-parity.sh`, que não existe (`ls scripts | grep -i doctor` vazio); o novo tipo de achado entra só no `printDoctorReport` e nos testes; (2) P-2 aprovada como emenda ao AC1 (o harness carrega o `MEMORY.md` do caminho injetado, logo sem leitura explícita a memória da raiz não volta). Padrão `*/**/.claude/agent-memory/` remedido pelo arquiteto com `git check-ignore -v --no-index` num repositório do scratchpad: raiz não ignorada; `pypi/...`, `a/b/c/d/...` e o aninhado `.claude/agent-memory/zeus-tf/.claude/agent-memory/...` ignorados. O `doctor` sai 0 com achados (RunE retorna nil), então o aviso não muda o exit code.
**Files affected:** `docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md`
**Actions:**
1. Completude da enumeração: todos os sítios que geram prompt de agente com `memory:` (assets, golden files, renderização por destino em `internal/integrations/render.go`, o agente arquiteto), o mecanismo de regras de `.gitignore` (`generateGitIgnore`, `internal/generators/scaffold.go`; chamado por `init` e por `update` em `internal/generators/update.go`), o `doctor` (`internal/commands/doctor.go`), e o conjunto de orquestração do barrier (`buildMLScopeOrchestrationSet`, `internal/commands/barrier.go`, que só aceita `.claude/agent-memory/` na raiz).
2. Modelo de ameaça: instrução no prompt que contradiz o caminho injetado pelo harness; padrão de `.gitignore` que ignore a memória versionada da raiz (medir com `git check-ignore -v` num repositório em `t.TempDir()` ou no scratchpad, nunca neste repositório); `doctor` varrendo árvore grande, symlink para fora da raiz, `node_modules`.
3. Alvos de falsificação nos dois sentidos para cada AC.
4. Residual declarado.
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando e resultado)
- [x] Prescrições em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md
grep -q '^## Prescrições' docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md
```

## Wave 1 — Implementação (2 executores em paralelo, pacotes disjuntos)
> Dependencies: Wave 0 auditada. ML-1A só em `internal/integrations/` e no asset do arquiteto; ML-1B e ML-1C no mesmo executor, em sequência, porque ambos tocam `docs/cli-parity.md`.

### ML-1A — Instrução de memória no renderizador do Claude Code e de despacho no arquiteto (apolo-tf) — AC1, AC2
**Status:** ✅ Concluído — auditado em 2026-11-01. Seção `## Memory location` (escrita e leitura na raiz) acrescentada pelo renderizador só para o destino `claude`, depois da reescrita de `model:` e antes da identidade; a reescrita da assinatura procura de trás para a frente a última linha `— nome, título`, então continua funcionando com o sufixo. `## Dispatch cwd` no corpo de `architect.md`. Dois goldens do arquiteto refeitos. Testes rodados por nome em `-v` pelo arquiteto: presença no `claude`, ausência em gemini, cursor, opencode, codex e antigravity, e `Dispatch cwd` presente; contra-braço do executor (sem a injeção, a presença reprova).
**Files affected:** `internal/integrations/render.go`, `internal/integrations/render_test.go`, `internal/integrations/assets/agents/architect.md`, `internal/integrations/testdata/`
**Actions:**
1. P-1/P-2: no renderizador, só para o destino `claude`, acrescentar ao corpo a instrução de memória do AC1 (escrita e leitura na raiz do repositório).
2. P-3: instrução de despacho do AC2 no corpo de `architect.md`.
3. Goldens atualizados; contra-braço: texto do AC1 ausente em codex, opencode, agent-directory e nos demais destinos.
**Acceptance criteria:**
- [x] AC1 e AC2 da REQ; contra-braço por destino
- [x] pacote `internal/integrations` verde

### ML-1B — Regra de `.gitignore` para memória em subpasta (apolo-tf) — AC3
**Status:** ✅ Concluído — auditado em 2026-11-01. `gitIgnoreAgentMemoryEntry = "*/**/.claude/agent-memory/"`, emissão e detecção derivadas da mesma constante (o executor achou e corrigiu a divergência entre literal e constante que deixava o contra-braço verde pelo motivo errado). Detecção por linha exata. Quatro vetores com `git check-ignore` e idempotência rodados por nome em `-v` pelo arquiteto; contra-braço do executor com `**/`: a raiz passa a ser ignorada e o teste reprova.
**Files affected:** `internal/generators/scaffold.go`, `internal/generators/gitignore_test.go`, `docs/cli-parity.md`
**Actions:**
1. P-4: `generateGitIgnore` garante `*/**/.claude/agent-memory/` com detecção por linha exata.
2. P-7: testes com os quatro vetores da REQ medidos por `git check-ignore` e idempotência.
**Acceptance criteria:**
- [x] AC3 da REQ; contra-braço: regra ausente → subpasta não ignorada; padrão `**/` → raiz ignorada
- [x] pacote `internal/generators` verde

### ML-1C — `doctor` aponta memória fora da raiz (apolo-tf, depois do ML-1B) — AC4
**Status:** ✅ Concluído — auditado em 2026-11-01. `scanStrayAgentMemory` (`internal/commands/doctor.go`), tipo `stray-agent-memory`, exit code inalterado. Sete testes rodados por nome em `-v` pelo arquiteto. Contra-braço refeito pelo arquiteto: com o filtro de repositório aninhado desligado (`&& false`), `TestScanStrayAgentMemory_RepositorioAninhado` reprova; restaurado e conferido com `cmp`. O executor mediu que o guarda de symlink do laço externo é redundante (o `WalkDir` não segue symlink); fica como defesa.
**Files affected:** `internal/commands/doctor.go`, `internal/commands/doctor_test.go`, `internal/integrations/doctor.go`, `docs/cli-parity.md`, `.claude/agent-memory/apolo-tf/`
**Actions:**
1. P-5 (sem o script inexistente): raiz por `git rev-parse --show-toplevel`; ignora `.git`, `node_modules`, symlink e repositório aninhado; conta arquivos; novo tipo no `printDoctorReport`.
**Acceptance criteria:**
- [x] AC4 da REQ; contra-braço: sem diretório fora da raiz, nenhuma linha; com repositório aninhado, nenhuma linha
- [x] pacote `internal/commands` verde

### ML-1D — Este repositório (arquiteto) — AC5
**Status:** ✅ Concluído — arquiteto, 2026-11-01. Regra no `.gitignore`; os três diretórios vazios removidos (tinham `zeus-tf/` vazio dentro). `find . -path ./node_modules -prune -o -type d -name agent-memory -print` → só `./.claude/agent-memory`; `git check-ignore -v` ignora `pypi/.claude/agent-memory/x` e o aninhado, e não ignora `.claude/agent-memory/zeus-tf/MEMORY.md`.
**Files affected:** `.gitignore`
**Actions:** P-6: regra no `.gitignore` e remoção dos três diretórios vazios (não rastreados).
**Acceptance criteria:**
- [x] `find . -path ./node_modules -prune -o -type d -name agent-memory -print` retorna só `./.claude/agent-memory`

**Gates da wave:**
```bash
go build ./...
go vet ./internal/integrations/ ./internal/generators/ ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/integrations/ ./internal/generators/ ./internal/commands/ -count=1
make quality
```

## Wave 2 — Red team (hades-tf)
> Dependencies: Wave 1 auditada.

### ML-2A — Red team da Wave 1 (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-01. Reprova a Wave 1 com três achados MEDIUM em código novo e um LOW; todos viram a Wave 3 (defeito conhecido se corrige no mesmo PR). A-1: uma negação `!` do consumidor para `.claude/agent-memory/` é ignorada pela detecção e o append posterior a sobrescreve (last match wins). A-2: o remédio do `doctor` sugere `rm -rf <rel>` sem aspas e relativo à raiz, e com espaço no caminho apaga outro diretório (confirmado no código: `"remove with: rm -rf " + rel`). A-3: o `WalkDir` interno ignora erro de leitura, e uma subpasta ilegível com arquivos é declarada vazia, com sugestão de apagar (confirmado no código: `if innerErr != nil { return nil }`). A-4: o texto `## Dispatch cwd` vai para todos os destinos e afirma comportamento do harness do Claude Code. Ataques que passaram: assinatura com identidade, idempotência do `agents update`, nenhum `hand-modified` depois do upgrade, worktree, laço de symlink, 6000 diretórios em 116 ms.
**Files affected:** `docs/seguranca/2026-11-01-redteam-memoria-de-agente-em-subpasta.md`, `vault/notes/`
**Actions:**
1. Reimplementar a partir da leitura, não do diff: cada prescrição P-1 a P-7 com veredito e evidência.
2. Atacar o renderizador (vazamento do texto para outro destino, efeito na assinatura e na identidade), a regra de `.gitignore` (padrões preexistentes do consumidor, negações, CRLF) e o `doctor` (custo em árvore grande, `rm -rf` sugerido com caminho que contém espaço ou metacaractere, repositório aninhado, symlink).
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-redteam-memoria-de-agente-em-subpasta.md
```

---

## Wave 3 — Corretivos do red team (1 ML)
> Dependencies: Wave 2 auditada.

### ML-3A — Achados A-1 a A-4 do red team (apolo-tf) — AC7
**Status:** ⬜ Pendente
**Files affected:** `internal/generators/scaffold.go`, `internal/generators/gitignore_test.go`, `internal/commands/doctor.go`, `internal/commands/doctor_test.go`, `internal/integrations/assets/agents/architect.md`, `internal/integrations/testdata/architect.subagent.golden.md`, `internal/integrations/testdata/architect.agent-directory.golden.md`, `internal/integrations/render_test.go`, `docs/cli-parity.md`, `.claude/agent-memory/apolo-tf/`
**Actions:**
1. A-1: uma linha `!` do consumidor cujo padrão termina em `.claude/agent-memory/` (ou `.claude/agent-memory`) é tratada como decisão dele: não acrescenta a regra e avisa uma linha.
2. A-2: o remédio do `doctor` usa o caminho absoluto, entre aspas simples com escape de `'` interno.
3. A-3: erro de leitura dentro do diretório fora da raiz não vira "vazio": o achado diz que há conteúdo ilegível e não sugere apagar.
4. A-4: `## Dispatch cwd` qualifica que o comportamento é do Claude Code.
**Acceptance criteria:**
- [ ] AC7 da REQ; contra-braço por item (A-1 com a negação; A-2 com espaço e `'` no nome; A-3 com subpasta `chmod 000`, pulado no Windows e como root)
- [ ] pacotes `internal/generators`, `internal/commands` e `internal/integrations` verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/generators/ ./internal/commands/ ./internal/integrations/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/generators/ ./internal/commands/ ./internal/integrations/ -count=1
make quality
```
