---
status: done
date: 2026-10-07
req: "docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: trackfw init nao instala os hooks de Gemini e Kiro na primeira execucao porque detecta os CLIs antes de criar seus arquivos

> Created: 2026-10-07 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md -->
REQ: docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md

Causa lida no código pelo arquiteto (2026-10-07): `internal/commands/init.go` chama `generators.Scaffold(cfg)` (~369),
que chama `InjectHooksDetected(cwd)` (`internal/generators/scaffold.go:198`), e **só depois** `installAITools(...)` (~386),
que cria os arquivos que a detecção procura. A detecção (`internal/generators/hooks.go:12`) olha `GEMINI.md`/`.gemini`
para o Gemini e só `.kiro` para o Kiro — que o `installAITools` não cria. Resultado reproduzido: 1ª execução sem hooks
de Gemini e Kiro; 2ª com Gemini; Kiro nunca. O mesmo `InjectHooksDetected` é chamado por `update` (`update.go:69`, `:2428`)
e `discover` (`commands/discover.go:161`).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Uma única execução de `trackfw init --ai-tools <lista>` instala o hook de guard de cada CLI pedido (AC2 da REQ)
- [x] `update` e `discover` varridos: mesma causa corrigida aqui, com teste (AC3)
- [x] Falsificação: voltar a ordem antiga reprova o teste (AC4)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md`
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML
      ✅ Parecer do hades-tf (`docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md`): **libera a Wave 1**.
      🔴 O defeito é maior que a REQ dizia: na 1ª execução de `init --ai-tools <cli>` (HOME isolado), **7 de 8 CLIs** ficam
      sem guard — codex, gemini, kiro, copilot, cursor, windsurf, amazonq; só o claude funciona, por acaso (o `CLAUDE.md`
      é criado antes da detecção). Kiro fica sem guard também na 2ª execução (`.kiro/` nunca é criado). Mesma causa no
      `install` de agents/skills (`integrations_flags.go`) → ML-1B. O `validate` não acusa guard ausente (observação).

**Gates da wave:**
```bash
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md
grep -q "Veredito" docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: Wave 0 auditada.

### ML-1A — O `init` instala o hook de cada CLI pedido em `--ai-tools`, numa execução só
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/commands/init.go`, `internal/generators/hooks.go`, `internal/generators/scaffold.go` (+ testes); `discover`/`update` só se a varredura achar a mesma causa
**Acceptance criteria:**
- [x] Teste por CLI de `--ai-tools` para os 8 CLIs, a partir de diretório vazio (`git init`, `t.Setenv("HOME", t.TempDir())` por CLI), uma execução: o arquivo de hook do CLI existe e contém a linha de guard da D11
- [x] Kiro em modo não interativo, sem `.kiro/` pré-existente: guard instalado na 1ª execução; o relatório diz qual mecanismo fecha esse caso (a reordenação sozinha não cria `.kiro/`)
- [x] `trackfw update --targets agent-hooks` num projeto Kiro iniciado pelo `init` corrigido reconhece o Kiro — **AC reescrito na auditoria**: logo depois do `init` o hook já existe e o `update` reporta `skipped` (nada a mudar, correto). O teste falsificável é o de reparo: hook removido, `.kiro/` mantido → `updated=1`; no código antigo (sem `.kiro/`), `skipped`
- [x] Corrigir o comentário de `internal/generators/agentfiles.go` (~2422) que diz que o `InjectKiroHooks` nunca instala o git-branch-guard (instala)
- [x] Varredura de `update` e `discover` com resultado escrito; mesma causa → corrigida aqui com teste — `update` e `discover` rodam depois que os arquivos existem (detecção funciona); `discover --init` chama duas vezes de forma idempotente (Wave 0); a mesma causa estava no `install` → ML-1B
- [x] Falsificação: voltar a ordem antiga reprova o teste dos 7 CLIs afetados (o claude passa na ordem antiga e não serve de prova)
- [x] `go test ./internal/commands/ ./internal/generators/ -count=1` e `make quality` (arquiteto, sem `~/.local/bin` no PATH) verdes
      Auditoria (2026-10-07): mecanismo = despacho por nome (`InjectHooksForTools`, no fim do `installAITools`), não
      reordenação — a reordenação não cobriria o Kiro com escopo global. Tabela remedida com o binário: guard na 1ª
      execução nos 8 CLIs (eram 1). 15 subtestes conferidos pelo arquiteto. Observação: o `agents install` num projeto
      sem `init` cria o hook de attention que aponta para `scripts/trackfw-attention-*.sh` ainda inexistentes (o guard
      chama o `trackfw` direto e funciona). `make quality` pelo arquiteto (sem `~/.local/bin` no PATH): exit 0, 0 GUARDA.

### ML-1B — `install` de agents/skills: mesma causa (Wave 0)
**Status:** ✅ Concluído
**Squad:** apolo-tf (mesmo despacho do ML-1A, em sequência)
**Por que o escopo original não previa:** achado da Wave 0 — `internal/commands/integrations_flags.go` cria os arquivos
de instrução do CLI (ex.: `GEMINI.md`) e nunca chama a injeção de hooks depois. Mesma causa → mesma REQ.
**Acceptance criteria:**
- [x] Teste: o caminho de `install` que cria os arquivos de um CLI deixa o hook de guard desse CLI instalado, numa execução
- [x] Falsificação: remover a injeção reprova o teste

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ ./internal/generators/ -count=1
```
