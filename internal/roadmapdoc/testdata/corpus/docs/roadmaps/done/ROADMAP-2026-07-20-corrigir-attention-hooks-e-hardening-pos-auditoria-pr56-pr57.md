---
status: done
date: 2026-07-20
req: "docs/req/REQ-2026-07-20-corrigir-attention-hooks-e-hardening-pos-auditoria-pr56-pr57.md"
squad: ""
---

# Roadmap: corrigir attention-hooks e hardening pos-auditoria pr56 pr57

> Created: 2026-07-20 | Status: done

## Context

Implementa as correções da auditoria dos PRs #56/#57 (ver REQ). O PR #57 tem defeitos que desativam
a feature de attention-hooks na configuração mais comum e viola a Regra Dura de Paridade — 3 CLIs.
A causa-raiz é testes que validam a implementação, não o contrato externo — por isso a **Wave 3 é uma
barrier obrigatória de testes de contrato** que só executa após todas as correções.

REQ: docs/req/REQ-2026-07-20-corrigir-attention-hooks-e-hardening-pos-auditoria-pr56-pr57.md
squad:

### Mapa de dependências
```
Wave 1 (críticos, spawn paralelo por CLI) ──┐
Wave 2 (hardening seg + higiene, paralelo) ─┤→ barrier → Wave 3 (testes de contrato + make quality)
```
> Wave 1 e Wave 2 tocam arquivos majoritariamente disjuntos por CLI e podem rodar juntas; a Wave 3
> depende de ambas concluídas.

---

## Wave 1 — Correções críticas de hooks (3 MLs paralelos, por CLI)
> Dependencies: none

### ML-1A — Go: alinhar eventos de hook ao spec + resiliência do script
**Status:** done
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/scaffold.go`, `internal/generators/agentfiles_test.go`
**Actions:**
1. (C2) Em `InjectClaudeHooks`: trocar a chave `PermissionRequest` por `PreToolUse` com matcher
   `AskUserQuestion` para o signal; manter `PostToolUse[AskUserQuestion]` no cleanup.
2. (C3) Em `InjectCodexHooks`: alinhar o evento do Codex ao spec da VISION → `PermissionRequest`
   (signal) + `PostToolUse` (cleanup), idêntico ao que Node/Python devem produzir.
3. (C1) No corpo do `signalScript`/`cleanupScript` em `scaffold.go`: tornar a resolução de
   `ROADMAP_DIR` resiliente à ausência de `roadmap_dir:` sob `set -euo pipefail`
   (ex.: `ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d '"'"'"'"'"'"'"' | head -1 || true)`), garantindo que o fallback `docs/roadmaps` seja alcançado.
4. Corrigir `agentfiles_test.go` para assertar `PreToolUse[AskUserQuestion]` (Claude) e
   `PermissionRequest` (Codex) — remover a asserção que cristaliza o valor errado.
**Acceptance criteria:**
- [x] `.claude/settings.json` gerado tem signal em `PreToolUse[AskUserQuestion]`
- [x] `.codex/hooks.json` gerado tem signal em `PermissionRequest`
- [x] `go test ./...` verde
- [x] `go vet ./...` sem warnings

### ML-1B — Node: alinhar Codex ao spec + resiliência do script
**Status:** done
**Files affected:** `npm/src/generators/hooks.js`, `npm/tests/generators.test.js`
**Actions:**
1. (C3) `injectCodexHooks`: trocar `data.hooks.PreToolUse`/`.*` por `PermissionRequest` (signal) +
   `PostToolUse` (cleanup), conforme spec da VISION. Confirmar que `injectClaudeHooks` permanece
   em `PreToolUse[AskUserQuestion]` (já correto).
2. (C1) No template dos scripts shell: tornar a resolução de `ROADMAP_DIR` resiliente ao `grep` sem
   match sob `pipefail` (mesma abordagem do ML-1A).
**Acceptance criteria:**
- [x] `.codex/hooks.json` gerado idêntico em semântica ao do Go (`PermissionRequest`)
- [x] `node --test` verde

### ML-1C — Python: resiliência do script (Codex já correto)
**Status:** done
**Files affected:** `pypi/trackfw/generators/init_gen.py`, `pypi/trackfw/generators/hooks.py`, `pypi/tests/test_generators_init.py`
**Actions:**
1. (C1) Nos templates `_ATTENTION_SIGNAL_SH`/`_ATTENTION_CLEANUP_SH`: resiliência ao `grep` sem match
   sob `pipefail` (mesma abordagem do ML-1A).
2. (C3) Confirmar que `inject_codex_hooks` permanece em `PermissionRequest` (é a referência correta) e
   que `inject_claude_hooks` permanece em `PreToolUse[AskUserQuestion]`.
**Acceptance criteria:**
- [x] Script executa até escrever o arquivo com YAML sem `roadmap_dir:`
- [x] `pytest pypi/tests/` verde

---

## Wave 2 — Hardening de segurança + higiene (3 MLs paralelos, por CLI)
> Dependencies: none (paralela à Wave 1; arquivos disjuntos dos MLs 1A/1B/1C onde possível — coordenar
> edições no mesmo arquivo de script via ordem 1x→2x se necessário)

### ML-2A — Go: contenção de path + escaping + higiene
**Status:** done
**Files affected:** `internal/generators/scaffold.go`, `internal/generators/hooks.go`, `internal/generators/claudemd.go`, `internal/generators/agentfiles.go`
**Actions:**
1. (C4) Nos scripts: normalizar/conter `ROADMAP_DIR` ao `cwd` antes de `mkdir -p`/escrita.
2. (C5) No JSON dos scripts: escapar `\` além de `"` e tratar `\n` (restaurar `tr -d '\n'` ou migrar
   para `jq -n`/`python3 -c json.dumps` com fallback).
3. (C10) Extrair a diretiva de ADRs globais para uma constante única (eliminar cópias em
   `claudemd.go`/`scaffold.go`/`agentfiles.go`).
4. (C11) Remover as vars mortas (aliases minúsculos) em `hooks.go`.
5. (C12) Comentar o overwrite intencional de Kiro/Copilot.
**Acceptance criteria:**
- [x] `go test ./...` e `go vet ./...` verdes; diretiva em constante única

### ML-2B — Node: contenção de path + escaping + higiene
**Status:** done
**Files affected:** `npm/src/generators/hooks.js`, `npm/src/generators/init.js`
**Actions:**
1. (C4) Conter `ROADMAP_DIR` ao `cwd` nos scripts.
2. (C5) Escaping de `\`/`\n` no JSON dos scripts.
3. (C10) Diretiva de ADRs globais em constante única.
**Acceptance criteria:**
- [x] `node --test` verde

### ML-2C — Python: contenção de path + escaping + higiene + gaps de paridade
**Status:** done
**Files affected:** `pypi/trackfw/generators/init_gen.py`, `pypi/trackfw/generators/hooks.py`, `pypi/trackfw/commands/discover.py`, `pypi/trackfw/validator.py`, `pypi/trackfw/config.py`
**Actions:**
1. (C4) Conter `ROADMAP_DIR` ao `cwd` nos scripts.
2. (C5) Escaping de `\`/`\n` no JSON dos scripts.
3. (C6) `discover.py`: logar aviso ao falhar geração de scripts (remover `except: pass` silencioso).
4. (C7) Alinhar granularidade da isenção `adr_orphan` para por-arquivo (como Go/Node).
5. (C8) `inject_hooks_detected` cobrir `windsurf`.
6. (C9) Decidir/alinhar comportamento de `~user/` (documentar se não suportar).
7. (C10) Diretiva de ADRs globais em constante única.
**Acceptance criteria:**
- [x] `pytest pypi/tests/` verde; paridade de `adr_orphan`/windsurf com Go/Node

---

## Wave 3 — BARRIER: testes de contrato + paridade (1 ML)
> Dependencies: Wave 1 E Wave 2 completas

### ML-3A — QA: testes de contrato externos (impedem reincidência)
**Status:** done
**Files affected:** testes nos 3 CLIs (`internal/generators/*_test.go`, `npm/tests/*`, `pypi/tests/*`)
**Actions:**
1. Teste que executa o script gerado com `trackfw.yaml` **sem** `roadmap_dir:` e verifica criação +
   remoção de `.trackfw-attention.json` (cobre C1) — nos 3 CLIs.
2. Teste que assere **evento de hook == spec da VISION** por alvo/CLI (cobre C2/C3).
3. Teste de escaping com `MSG` contendo `"`, `\` e `\n` → JSON válido/parseável (cobre C5).
4. Teste que reforça idempotência de Kiro/Copilot comparando **conteúdo** (cobre C13).
**Acceptance criteria:**
- [x] Todos os testes de contrato verdes nos 3 CLIs
- [x] `make quality` (Go + Node.js + Python + contratos de paridade) verde
- [x] `trackfw validate` sem violações

---

## ✅ RESOLVIDO — C13 paridade de teste de idempotência por conteúdo (2026-07-20)

> **Status:** ✅ Concluído
> **Severidade:** 🟡 Baixa

### Diagnóstico & Resolução
A cobertura de idempotência por conteúdo para Kiro e Copilot foi padronizada entre os 3 CLIs:

| Alvo | Arquivo / teste | Estado pós-correção |
|------|-----------------|---------------------|
| **Go — Kiro** | `internal/generators/agentfiles_test.go` → `TestInjectKiroHooks` | ✅ Roda 2× + `bytes.Equal(content1, content2)` |
| **Go — Copilot** | `internal/generators/agentfiles_test.go` → `TestInjectCopilotHooks` | ✅ Roda 2× + `bytes.Equal(content1, content2)` |
| **Python — Kiro** | `pypi/tests/test_generators_init.py` → `test_inject_kiro_hooks` | ✅ Roda 2× + `self.assertEqual(data, data2)` |
| **Python — Copilot** | `pypi/tests/test_generators_init.py` → `test_inject_copilot_hooks` | ✅ Roda 2× + `self.assertEqual(data, data2)` |
| **Node — Kiro/Copilot** | `npm/tests/generators.test.js` | ✅ Roda 2× + `assert.deepStrictEqual(data1, data2)` |

### Critérios de aceite do C13
- [x] Go: Kiro e Copilot comparam conteúdo pós-2ª injeção (não só `len`)
- [x] Python: `test_inject_kiro_hooks` compara conteúdo (`assertEqual(data, data2)`)
- [x] Node: Kiro e Copilot rodam 2× e comparam conteúdo (`deepStrictEqual`)
- [x] `make quality` (Go + Node + Python + paridade) verde
- [x] Paridade tripla de teste de idempotência por conteúdo confirmada

### Observação de governança (separada, não faz parte do C13)
`trackfw validate` acusa `branch_has_wip_roadmap` porque este roadmap foi movido para `done/`
enquanto a branch `fix/attention-hooks-pos-auditoria` ainda não foi mergeada (`wip/` vazio).
Alinhar o estado (manter em `wip/` até o merge) ou aceitar a violação como esperada pré-merge.
