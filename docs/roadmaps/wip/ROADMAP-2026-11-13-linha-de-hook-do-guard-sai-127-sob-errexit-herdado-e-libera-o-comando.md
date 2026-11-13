---
status: wip
date: 2026-11-13
req: "docs/req/REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md"
squad: "hades-tf, apolo-tf, artemis-tf, hefesto-tf, zeus-tf"
---

# Roadmap: linha de hook do guard sai 127 sob errexit herdado e libera o comando

> Created: 2026-11-13 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md -->
REQ: docs/req/REQ-2026-11-13-linha-de-hook-do-guard-sai-127-sob-errexit-herdado-e-libera-o-comando.md
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md (adendo de 2026-11-13, D18)
Origem: resíduo R4 da REQ-2026-10-20, não corrigido. Varredura de issues em 2026-11-13: zero abertas.

A linha de hook PS/POSIX tem segmentos que, em sh/bash/dash, falham por desenho (`emulate`, `$LASTEXITCODE=2`)
ou expandem variável não definida (`$LASTEXITCODE`). Com `errexit` ou `nounset` herdados, o shell sai 127 antes
do guard e o comando passa. Medição do arquiteto na REQ (macOS): 127 em `bash -e`, `bash -u`, `sh -e`, `sh -u`,
`dash -e` e `BASH_ENV` com `set -e`; 2 em `bash`, `zsh -e`, `zsh -u` e `SHELLOPTS=errexit`.

Sítio canônico da forma esperada: `guardExpectedLine` em `internal/validator/validator_guard_binary_probe.go`.
O literal `emulate -R sh` aparece também em `internal/generators/agentfiles.go`,
`internal/guardforms/guard_generations.go`, `internal/validator/validator_credential_guard.go` e
`internal/validator/validator_git_branch_guard.go`. Matriz existente: `internal/generators/guard_line_shell_test.go`.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] AC1 — inventário e matriz medida da forma atual (ML-0A)
- [ ] AC2 — deny sai 2 sob as opções herdadas (ML-1A)
- [ ] AC3 — allow sai 0 sob as opções herdadas (ML-1A)
- [ ] AC4 — PowerShell 5.1 e 7 no Windows (ML-1B)
- [ ] AC5 — matriz de teste com contra-braço (ML-1A)
- [ ] AC6 — geração legada, aviso e migração sem duplicata (ML-1A)
- [ ] AC7 — configs deste repositório migradas (ML-1A)
- [ ] AC8 — `make quality` verde (ML-2B)

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model e inventário (AC1)
> Dependencies: none. Blocks all implementation.

### ML-0A — Inventário das formas de linha de hook, matriz de opções herdadas e candidatos
**Status:** ⬜ Pending
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-13-wave0-guard-errexit-herdado.md` (único arquivo escrito)

**Actions:**
1. Inventário: toda forma de linha de hook que o trackfw gera ou aceita como corrente (família PS/POSIX,
   POSIX-only, nativa de Windows, invólucro `/bin/bash -c`, registro da D12 em `guard_generations.go`). Busque o
   literal das formas no repositório inteiro, não só nos arquivos citados no Context, e confira a contagem por
   um segundo caminho.
2. Matriz da forma atual: para cada forma, rc com guard falso que nega (2) e que permite (0), sob `bash`,
   `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e` (se houver `dash`), `zsh -e`, `zsh -u`, `BASH_ENV` com
   `set -e` e com `set -u`, e `SHELLOPTS=errexit`. Meça só em diretório de `mktemp -d` com `trap` de limpeza e
   `PATH` apontando para o guard falso; nada de `git` no repositório (se precisar de git, use um teste Go em
   `t.TempDir()`). Diga, pela documentação de cada CLI de agente, com que shell ele executa o hook e se herda o
   ambiente do usuário.
3. Candidatos: pelo menos duas formas que neutralizem `errexit` e `nounset` antes de qualquer segmento que
   falhe (por exemplo `set +eu` no início), medidas na mesma matriz POSIX. Para cada uma, diga o que se espera
   no PowerShell 5.1 e 7 (parse e stderr); a medição no Windows é do ML-1B.
4. Threat model: como a forma nova poderia liberar o comando (guard ausente do PATH, rc fora de 0/2, `set -e`
   religado depois, alias ou função chamada `set`) e como poderia bloquear o legítimo.
5. Residual declarado.

**Acceptance criteria:**
- [ ] Inventário com contagem conferida por segundo caminho e lista final dos sítios do ML-1A
- [ ] Matriz da forma atual e dos candidatos com rc medido, não presumido
- [ ] Nenhuma linha de implementação escrita; `git diff --cached` vazio ao fim

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-13-wave0-guard-errexit-herdado.md
```

## Wave 1 — Implementação (AC2–AC7)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B são sequenciais: o ML-1B mede no Windows a forma que o ML-1A
> fixar, e os dois podem tocar `guard_line_shell_test.go`.

### ML-1A — Forma nova da linha, geração legada, migração e matriz POSIX
**Status:** ⬜ Pending
**Squad:** apolo-tf
**Files affected:** definidos pela auditoria do ML-0A (base: os cinco arquivos Go do Context, os testes deles e
as configs de hook versionadas deste repositório).

**Actions:** definidas pela auditoria do ML-0A, com a forma escolhida pela D18.

**Acceptance criteria:**
- [ ] AC2, AC3, AC5, AC6 e AC7 da REQ
- [ ] `go build ./...` e os testes dos pacotes tocados verdes

**Wave gates:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ ./internal/guardforms/
```

### ML-1B — PowerShell 5.1 e 7 na VM Windows
**Status:** ⬜ Pending
**Squad:** artemis-tf
**Files affected:** `docs/portabilidade/2026-11-13-guard-errexit-windows.md`

**Actions:** definidas pela auditoria do ML-1A.

**Acceptance criteria:**
- [ ] AC4 da REQ, medido na VM e confirmado no CI de Windows do PR

## Wave 2 — Red team e qualidade (AC8)
> Dependencies: Wave 1 completa.

### ML-2A — Red team da forma nova
**Status:** ⬜ Pending
**Squad:** hades-tf, hefesto-tf
**Files affected:** `docs/seguranca/2026-11-13-wave2-redteam-guard-errexit-herdado.md`

**Actions:** reimplementar a matriz a partir da leitura, sem conferir o diff, e tentar liberar um comando negado.

**Acceptance criteria:**
- [ ] Parecer com cada tentativa e o rc medido; achado de mesma causa vira ML nesta REQ

### ML-2B — `make quality` com a máquina ociosa
**Status:** ⬜ Pending
**Squad:** zeus-tf

**Acceptance criteria:**
- [ ] `make quality` rc=0
