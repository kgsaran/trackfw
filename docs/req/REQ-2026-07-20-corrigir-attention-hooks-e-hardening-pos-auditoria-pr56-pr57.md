---
status: Open
date: 2026-07-20
author: Zeus
adr: ""
roadmap: ""
---

# REQ: corrigir attention-hooks e hardening pos-auditoria pr56 pr57

> Date: 2026-07-20 | Status: Open

## Motivation

Auditoria de qualidade e segurança dos PRs **#56** (`feat(governance): ADRs globais compartilhados`)
e **#57** (`feat(attention-hooks): hooks nativos dos 7 CLIs`), já mergeados, conduzida por três
frentes independentes (análise estática arquitetural + revisão de segurança + revisão de qualidade
com reprodução empírica). O PR #56 ficou em padrão aceitável; o **PR #57 apresenta defeitos que
desativam a feature-bandeira na configuração mais comum** e viola a *Regra Dura de Paridade — 3 CLIs*.

Todos os defeitos passaram pela suíte de testes porque os testes validam a **implementação gerada**
(a chave existe no JSON, o script tem o conteúdo esperado) e **não o contrato externo** (o script roda
até o fim sob `pipefail`; o evento de hook corresponde ao spec de cada CLI). Esta REQ formaliza as
correções e — crucialmente — exige **testes de contrato** que capturem essa classe de falha.

### Evidências da auditoria (fonte de verdade: `docs/visao-projeto/VISION.md` e a REQ
`REQ-2026-06-20-attention-hooks-agent-clis.md`, cujos critérios de aceite a implementação Go/Node viola)

**🔴 Críticos (PR #57)**

- **C1 — Scripts de attention abortam silenciosamente (`set -euo pipefail` + `grep` sem match) —
  afeta os 3 CLIs.** Em `scaffold.go`, `npm/src/generators/hooks.js` e `pypi/.../init_gen.py`, o
  bloco `ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml | ... | head -1)` seguido de
  `ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}` nunca alcança o fallback: com `pipefail`, `grep` sem
  match sai 1 e `set -e` mata o script antes. Reproduzido ao vivo (exit 1, arquivo nunca escrito).
  O fluxo de retrofit (`trackfw update`/`discover`) não escreve `roadmap_dir:` no YAML, e o próprio
  `trackfw.yaml` deste repositório não o tem → a feature falha silenciosamente em todo tool call.

- **C2 — Hook do Claude no CLI Go diverge do spec e dos demais CLIs.**
  `internal/generators/agentfiles.go` usa `PermissionRequest`; Node e Python usam
  `PreToolUse[AskUserQuestion]`, que é o que a `VISION.md` e a REQ original documentam. Regressão de
  refactor (troca acidental de chave com o Codex ao realocar a função). Observação: `PermissionRequest`
  **é** um evento válido do Claude Code (doc oficial jul/2026), então não é config morta — mas muda a
  condição de disparo e quebra paridade/spec.

- **C3 — Divergência de paridade tripla no evento do Codex.** O PR mudou a chave do Codex de
  `PermissionRequest`→`PreToolUse` em Go e Node, mas não sincronizou no Python (`hooks.py` mantém
  `PermissionRequest`). Impossível as 3 estarem corretas ao mesmo tempo.

  | Alvo | Spec (VISION/REQ) | Go | Node | Python |
  |------|-------------------|-----|------|--------|
  | Claude — signal | `PreToolUse[AskUserQuestion]` | ❌ `PermissionRequest` | ✅ `PreToolUse` | ✅ `PreToolUse` |
  | Codex — signal | `PermissionRequest` | ❌ `PreToolUse[.*]` | ❌ `PreToolUse[.*]` | ✅ `PermissionRequest` |

**🟠 Médios**

- **C4 — Path traversal em `roadmap_dir` (segurança).** Os scripts shell resolvem `roadmap_dir` do
  YAML sem contenção ao `cwd` (ao contrário do que o próprio #56 fez para `adr_dirs`). Impacto
  limitado (nome de arquivo fixo, sem sobrescrita arbitrária), mas escreve fora da árvore do projeto.
- **C5 — JSON escaping incompleto + regressão (segurança).** O `sed` escapa só `"`, não `\`; o #57
  removeu o `| tr -d '\n'` que existia antes. `MSG` malformado corrompe o `.trackfw-attention.json`.
- **C6 — Erro engolido:** `discover.py` usa `except Exception: pass` totalmente silencioso ao gerar
  scripts (os demais handlers ao menos logam aviso).
- **C7 — Isenção `adr_orphan` diverge em granularidade (#56):** Go filtra por arquivo, Node por
  dir+arquivo, Python só por diretório → diverge com symlink apontando para fora do `cwd`.
- **C8 — `inject_hooks_detected` do Python não cobre `windsurf`** (Go/Node cobrem).
- **C9 — Expansão `~user/` (#56):** Python (`expanduser`) suporta; Go/Node só tratam `~`/`~/`.
- **C10 — DRY:** a diretiva de ADRs globais está hardcoded em 6 lugares, com drift de numeração.

**🟡 Baixos**

- **C11 — Vars mortas em Go** (`hooks.go` — aliases minúsculos órfãos).
- **C12 — Kiro/Copilot fazem overwrite total** (correto, mas inconsistente e sem comentário em Go/Node).
- **C13 — Testes de idempotência de Kiro/Copilot** só checam `len==2`, não comparam conteúdo.

### Pontos fortes confirmados (não regredir)

- Path handling sem bug de prefixo (`/foo` ⊄ `/foobar`) nos 3 CLIs (`filepath.Rel` / `path.relative`
  / `os.path.commonpath`). Merge JSON idempotente e deduplicado. Sem RCE / command injection / supply
  chain. Isenção `adr_orphan` para paths externos é design deliberado e documentado (não é bypass).

## Acceptance Criteria

### Bloqueio da feature #57 (críticos)

- [ ] **C1** — Scripts `trackfw-attention-signal.sh`/`cleanup.sh` (Go, Node, Python) executam até
      escrever/remover o arquivo mesmo quando `trackfw.yaml` **não** contém `roadmap_dir:`
      (remover `pipefail` do trecho ou usar `grep ... || true` / default resiliente).
- [ ] **C2** — CLI Go gera hook do Claude como `PreToolUse[AskUserQuestion]` (signal) +
      `PostToolUse[AskUserQuestion]` (cleanup) em `.claude/settings.json`, conforme `VISION.md`.
- [ ] **C3** — Chave de evento do Codex idêntica nos 3 CLIs, alinhada ao spec da `VISION.md`
      (`PermissionRequest`); Go, Node e Python produzem o mesmo `.codex/hooks.json` funcional.
- [ ] Paridade de wiring de hooks validada nos 3 CLIs para os 7 alvos (Claude, Codex, Gemini, Kiro,
      Copilot, Cursor, Windsurf) — mesma semântica de evento por alvo.

### Hardening de segurança (médios)

- [ ] **C4** — Os 3 scripts contêm/normalizam `ROADMAP_DIR` ao `cwd` antes de `mkdir -p`/escrita
      (reaproveitar a garantia de contenção já existente para `adr_dirs`).
- [ ] **C5** — Construção do JSON escapa `\` além de `"` e trata newlines (restaurar `tr -d '\n'`
      ou migrar para `jq -n` / `python3 -c json.dumps` com fallback seguro) nos 3 CLIs.

### Consistência e higiene (médios/baixos)

- [ ] **C6** — `discover.py` loga aviso ao falhar geração de scripts (não `except: pass` silencioso).
- [ ] **C7** — Isenção `adr_orphan` com a mesma granularidade nos 3 CLIs (definir por-arquivo como
      padrão e alinhar Python).
- [ ] **C8** — `inject_hooks_detected` (Python) cobre `windsurf` como Go/Node.
- [ ] **C9** — Expansão de path com comportamento equivalente para `~user/` nos 3 CLIs (documentar a
      decisão se optar por não suportar).
- [ ] **C10** — Diretiva de ADRs globais extraída para constante única por CLI (eliminar as 6 cópias).
- [ ] **C11/C12/C13** — Remover vars mortas em Go; documentar overwrite intencional de Kiro/Copilot em
      Go/Node; reforçar asserção dos testes de idempotência para comparar conteúdo.

### Testes de contrato (obrigatório — impede a reincidência da classe de falha)

- [ ] Teste que executa cada script gerado com um `trackfw.yaml` **sem** `roadmap_dir:` e verifica que
      `.trackfw-attention.json` é criado e depois removido (cobre C1) — nos 3 CLIs.
- [ ] Teste que assere o **evento de hook == spec da VISION** por alvo e por CLI (cobre C2/C3),
      substituindo/corrigindo o teste atual que assere `PermissionRequest` para o Claude.
- [ ] Teste de escaping com `MSG` contendo `"`, `\` e `\n` produzindo JSON válido e parseável (cobre C5).
- [ ] `make quality` (Go + Node.js + Python + contratos de paridade) verde.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
