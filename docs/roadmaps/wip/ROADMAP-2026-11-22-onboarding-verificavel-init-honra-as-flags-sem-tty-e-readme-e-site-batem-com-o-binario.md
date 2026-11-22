---
status: wip
date: 2026-11-22
req: "docs/req/REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md"
squad: "hades-tf, apolo-tf, afrodite-tf, atena-tf, hefesto-tf"
---

# Roadmap: Onboarding verificável: init honra as flags sem TTY e README e site batem com o binário

> Created: 2026-11-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md -->
REQ: docs/req/REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md

A auditoria `docs/qualidade/2026-11-22-auditoria-onboarding.md` registrou 25 achados (B.x, A.x, C.x, D.x). O ADR-2026-11-22
fixa D1–D6. Varredura de issues abertos em 2026-11-22: nenhum aberto.

## Acceptance Criteria
- [ ] AC1: `init` honra cada flag com e sem TTY; limites do brownfield (D2) afirmados por teste
- [ ] AC2: teste do Quick start confere README e guia do site em inglês contra os comandos que executa
- [ ] AC3: site em inglês na raiz e pt-BR em `/pt/`; build do site e capturas headless verdes
- [ ] AC4: bloqueantes e atritos da auditoria corrigidos ou justificados um a um
- [ ] AC5: README com Quick start contíguo, Windows fora de *Installation*, sem âncora quebrada
- [ ] AC6: `make quality` rc=0 e CI verde no PR

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model, flag sweep and link sweep (hades-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md` (novo; único arquivo)
**Actions:**
1. Enumeration completeness:
   - every `trackfw init` flag (`init.go:184-188` and any other) and the path that reads it (TTY, non-TTY or both),
     with the reading line;
   - whether `--ai-tools` is ignored in a real TTY (measured with a pty, e.g. `script -q /dev/null ...`, in a temp
     dir with isolated `HOME`/`USERPROFILE`/`CODEX_HOME`/`XDG_CONFIG_HOME`);
   - every inbound link that the site locale move (ADR D3) and the README heading moves (ADR D5) can break: grep the
     repository for `github.io/trackfw`, `/en/guide`, `/guide/` and `README.md#` in `internal/`, `npm/`, `pypi/`,
     `scripts/`, `site/`, `README.md`, `.github/` and `docs/cli-parity.md`;
   - scripts and tests that pin README or site content (`check-wheel-metadata-readme.sh`, `check-channels-content.sh`,
     `check-roadmap-barrier-contract.sh`, `check-shim-diff-applies.sh`, `quickstart_e2e_test.go`, any other).
2. Threat model for ADR D2:
   - what `init --brownfield` does on a project that already has `trackfw.yaml` (creates, extends or keeps
     `lenient_until`; downgrades an existing `governance_mode`), measured on the TTY path today;
   - whether the guards stop an agent from running `trackfw init` in a governed project;
   - a recommended D2 with the measurement behind it.
3. Falsification targets in both directions: flag ignored on one path (regression of D1); flag applied where it
   should not (brownfield relaxing a project already strict).
4. Declared residual.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence (commands and output), not a one-line assertion
- [ ] No implementation line written for this ML; no file outside the one above

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-22-wave0-onboarding-threat-model.md
```

## Wave 1 — Implementation: init and site locale (2 MLs in parallel)
> Dependencies: Wave 0 audited and ADR D2 written. ML-1A and ML-1B touch disjoint files.

### ML-1A — `init` honra as flags nos dois caminhos (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/commands/init.go`; `internal/commands/init_flags_both_paths_test.go` (novo);
`docs/cli-parity.md` (seção de `init`)
**Actions:**
1. ADR D1: every flag listed by ML-0A has the same effect with and without TTY; on the TTY path, an explicit flag wins
   over the wizard or skips the matching question.
2. ADR D2 as written after Wave 0.
3. Tests with arm and counter-arm per path (non-TTY in process; TTY via the existing injection point, or the one the
   ML creates).
4. Update tests and `scripts/check-gates-falsify.sh` scenarios that pin the old behavior, if ML-0A finds any.
**Acceptance criteria:**
- [ ] AC1 da REQ
- [ ] `go build ./...` e `go test ./internal/commands/` verdes

**Wave gates:**
```bash
go build ./...
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m ./internal/commands/
```

### ML-1B — Inglês na raiz do site (afrodite-tf)
**Status:** ⬜ Pending
**Files affected:** `site/.vitepress/config.mts`; `site/index.md`; `site/guide/getting-started.md`;
`site/guide/commands.md`; `site/guide/ai-agents.md`; `site/pt/index.md`; `site/pt/guide/getting-started.md`;
`site/pt/guide/commands.md`; `site/pt/guide/ai-agents.md`; `site/en/` (removido); plus every inbound link file
ML-0A lists outside `internal/`
**Actions:**
1. Move the English pages to the root and the pt-BR pages to `site/pt/` (`site/en/*` → `site/*`,
   `site/guide/*` and `site/index.md` → `site/pt/...`). Content unchanged in this ML.
2. Update `config.mts`: `root` locale en-US, `pt` locale pt-BR, nav, sidebar and OG tags.
3. Update the inbound links ML-0A listed.
**Acceptance criteria:**
- [ ] `npm ci && npm run build` em `site/` rc=0 (o VitePress reprova link morto)
- [ ] Captura em Chrome headless de `/trackfw/` (inglês) e `/trackfw/pt/` (pt-BR) sem asset quebrado
- [ ] Nenhum arquivo em `internal/`

**Wave gates:**
```bash
test -f site/guide/getting-started.md
test -f site/pt/guide/getting-started.md
test ! -e site/en
```

## Wave 2 — Content (2 MLs in parallel)
> Dependencies: Wave 1 audited. ML-2A touches only `site/`; ML-2B touches only `README.md`.

### ML-2A — Conteúdo do site (afrodite-tf)
**Status:** ⬜ Pending
**Files affected:** `site/index.md`; `site/guide/getting-started.md`; `site/guide/commands.md`;
`site/guide/ai-agents.md`; `site/pt/index.md`; `site/pt/guide/getting-started.md`; `site/pt/guide/commands.md`;
`site/pt/guide/ai-agents.md`
**Actions:** fix in both locales the audit findings B.1–B.8, B.10–B.17 and D.2, with every command run against
`bin/trackfw` in an isolated temp dir. Output blocks that change every release (`status`) become a functional
description instead of a literal snapshot. The English getting-started uses the same command lines as the README
Quick start, so ML-3A can pin both.
**Acceptance criteria:**
- [ ] Cada achado listado corrigido, ou justificado no relatório
- [ ] `npm ci && npm run build` em `site/` rc=0; capturas headless das duas raízes

**Wave gates:**
```bash
test -f site/guide/getting-started.md
```

### ML-2B — README contíguo (atena-tf)
**Status:** ⬜ Pending
**Files affected:** `README.md`
**Actions:**
1. ADR D5: `## Quick start, continued` right after `## Quick start`; `### Windows support (partial)` leaves
   *Installation* for its own top-level section after them. No content removed.
2. Update every internal anchor affected and fix B.19 (anchors that point to bold paragraphs).
3. Fix A.2: the `validate` line promises the output the Quick start really produces.
4. Fix B.18: `trackfw branch new feat/<slug>` before `ship` in *Quick start, continued*.
**Acceptance criteria:**
- [ ] AC5 da REQ; lista das âncoras antes/depois no relatório
- [ ] `bash scripts/check-wheel-metadata-readme.sh` rc=0

**Wave gates:**
```bash
bash scripts/check-wheel-metadata-readme.sh
```

## Wave 3 — Docs pinned by test
> Dependencies: Wave 2 audited (the test pins the final text).

### ML-3A — Teste do Quick start confere README e site (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/commands/quickstart_e2e_test.go`
**Actions:** ADR D4: the test keeps the list of commands it runs and asserts that each one appears literally in
`README.md` and `site/guide/getting-started.md`, and that the promised `validate` output in both documents is what
the run produces. No markdown parsing: a line-presence check. Counter-arm: removing or editing one command line
in either document makes the test fail.
**Acceptance criteria:**
- [ ] AC2 da REQ
- [ ] contra-braço medido: editar uma linha do Quick start em cada documento reprova o teste

**Wave gates:**
```bash
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 5m -run 'TestQuickStart' ./internal/commands/
```

## Wave 4 — Review and red team (2 MLs in parallel)
> Dependencies: Wave 3 audited.

### ML-4A — Revisão de qualidade (hefesto-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/qualidade/2026-11-22-onboarding-review.md`
**Acceptance criteria:**
- [ ] Parecer escrito; achados bloqueantes corrigidos por ML corretivo

### ML-4B — Red team (hades-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-22-red-team-onboarding.md`
**Acceptance criteria:**
- [ ] Reimplementa a falsificação de D1, D2 e D4 a partir do contrato, não do diff; residual declarado

**Wave gates:**
```bash
test -s docs/qualidade/2026-11-22-onboarding-review.md
test -s docs/seguranca/2026-11-22-red-team-onboarding.md
```

## Wave 5 — Fechamento
> Dependencies: Wave 4 audited.

### ML-FB — make quality e CI (zeus-tf)
**Status:** ⬜ Pending
**Acceptance criteria:**
- [ ] AC6: `make quality` rc=0 com a máquina ociosa; CI verde no PR

**Wave gates:**
```bash
go build ./...
```
