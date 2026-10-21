---
status: wip
date: 2026-10-20
req: "docs/req/REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md"
squad: "zeus-tf, backend, security, code-quality"
---

# Roadmap: Hooks de guard gerados falham sob zsh com exit 1 e deixam de executar a protecao

> Created: 2026-10-20 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md -->
REQ: docs/req/REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] allow=0 e deny/ausente/erro=2 na matriz de shells suportados.
- [ ] Migração preserva customizações e os gates passam.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** docs/roadmaps/wip/ROADMAP-2026-10-20-*.md (this file only)
**Actions:** see four sections below.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

#### 1. Enumeração completa

**Sítios de emissão** (onde a linha canônica `$LASTEXITCODE=2 2>${null-/dev/null}; ...` é gerada ou validada):

`git grep -F '$LASTEXITCODE=2'` no repositório (fonte: medição direta):

| Arquivo | Linha | Papel |
|---------|-------|-------|
| `internal/generators/agentfiles.go` | 490–492 | 3 constantes: `guardGitBranchCmdPSPOSIX`, `guardCredentialCmdPSPOSIX`, `guardCredentialGlobalCmdPSPOSIX` — fonte de verdade dos 3 guards |
| `internal/validator/validator_guard_binary_probe.go` | 83 | `guardExpectedLine()` — forma canônica usada pelo `validate` para comparar hooks instalados |
| `internal/validator/validator_guard_binary_probe.go` | 94 | `guardD11LegacyLine()` — forma D11 pré-ML-6C (sem 4º trecho); também começa com `$LASTEXITCODE=2 2>...`; reconhece legado |
| `docs/cli-parity.md` | 6195, 6197, 6387 | Documentação normativa — contrato público |
| `scripts/check-validate-rule-pins.sh` | 563 | PIN de fixture inline do Codex hooks JSON |

**Artefatos versionados que contêm a linha** (confirmado por `git grep`):

| Arquivo | Ocorrências | Observação |
|---------|-------------|------------|
| `.claude/settings.json` | 7 entradas de comando | Hooks do projeto — executados por Claude Code |
| `.codex/hooks.json` | 5 entradas de comando | Hooks do projeto — executados por Codex |
| `.gemini/settings.json` | 7 entradas de comando | Hooks do projeto — executados por Gemini |

Esses arquivos são versionados (ADR D2 — a string commitada serve ao time misto). O ML-1A deve migrar tanto os artefatos gerados quanto esses arquivos versionados.

**Sítios de asserção de teste** (mesma causa, mesmo PR — serão atualizados junto com os constantes):
- `internal/generators/update_test.go` (9 ocorrências de `wantCmd`)
- `internal/commands/update_harness_test.go` (≥9 ocorrências de `wantCmd`)
- `internal/validator/validator_credential_guard_test.go`, `validator_git_branch_guard_test.go`, `validator_guard_binary_probe_test.go`, `validator_guard_binary_probe_windsurf_amazonq_test.go`

**Validador que detecta divergência da forma esperada:**
`guardExpectedLine()` em `validator_guard_binary_probe.go:83` é chamado pelo validador ao comparar hooks instalados com a forma canônica. Com o workaround `/bin/bash -c '...'` no HOME real, `TestValidateJSONFlag` e `TestValidateTextUnchanged` falham localmente (passam com HOME isolado). Evidência em `docs/agents-working-context.md` (2026-10-21, zeus-tf).

**Tabela CLI × shell executor × fonte:**

| CLI | Família | Shell em macOS (executor do hook) | Shell no Windows | Fonte |
|-----|---------|----------------------------------|-----------------|-------|
| Codex | PS/POSIX | **zsh** (`SHELL=/bin/zsh`) | PowerShell | vault note + REQ (medido) |
| Claude Code | PS/POSIX | **zsh confirmado**: `$ZSH_VERSION=5.9` impresso pelo tool "Bash" desta sessão; shell executor dos hooks = SHELL env do processo | bash via `C:\Program Files\Git\bin\bash.exe` (hardcoded) | portabilidade/2026-10-20-jornadas-agentes-windows.md:A7; ZSH_VERSION medido nesta sessão |
| Gemini | PS/POSIX | inferência: bash | PowerShell | cli-parity.md:3891 |
| Cursor | PS/POSIX | inferência: bash | inferência: PowerShell | cli-parity.md:3893 |
| Copilot | PS/POSIX | varia (campo `command`) | PowerShell | ADR D4 |
| Windsurf | PS/POSIX | inferência: bash | inferência: PowerShell | agentfiles.go |
| Kiro | cmd.exe | não aplicável macOS | cmd.exe | ADR D2 rev. |
| Amazon Q | cmd.exe | não disponível macOS (inferência) | cmd.exe | ADR D2 rev. |

Nota: Kiro e Amazon Q usam `trackfw guard <nome> || exit 2` — sem `$LASTEXITCODE`, sem EQUALS, sem risco zsh.

**Enumeração fechada:** as 3 constantes em `agentfiles.go:490–492` são os únicos pontos de emissão. `validator_guard_binary_probe.go:83` produz a forma esperada para comparação. Todos os sítios confirmados por `git grep -F '$LASTEXITCODE=2'`.

#### 2. Threat model

**Medição da linha atual** — via `PATH="$D" /bin/<shell> -c "$LINE"` com fake `trackfw` (allow=0 / deny=2 / absent / err1=1); `$D` = dir de scratchpad com os stubs; `$LINE` = constante exata de `agentfiles.go:490`:

| Shell | allow | deny | absent | err1 | Erro observado |
|-------|-------|------|--------|------|----------------|
| `/bin/sh -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ | — |
| `/bin/bash -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ | — |
| `/bin/dash -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ | — |
| `/bin/zsh -c` | **1 ✗** | **1 ✗** | **1 ✗** | **1 ✗** | `zsh:1: 2 not found` |
| `/bin/zsh -f -c` | **1 ✗** | — | — | — | `zsh:1: 2 not found` |
| `/bin/zsh -l -c` | **1 ✗** | — | — | — | `zsh:1: 2 not found` |

**Dois pontos de quebra no zsh (confirmados por isolamento):**

1. **Segmento 1 — EQUALS expansion:** `$LASTEXITCODE` está vazio → palavra `=2` → EQUALS expansion (padrão ON em zsh 5.x) tenta `which 2` → `zsh:1: 2 not found`, abort exit 1. `zsh -f` reproduz: não é startup file, é comportamento padrão do shell.
2. **Segmento 4 — NOMATCH:** `$LASTEXITCODE=2*!!$LASTEXITCODE` → depois que `LASTEXITCODE` é setado como variável de shell (pelo segmento 3), expande para `N=2*!!N` → `*` é glob não-citado em posição de comando → zsh tenta expandir o glob, acha zero arquivos → `zsh:1: no matches found: N=2*!!N`, abort exit 1. O `!!` sobrevive literal em shell não-interativo (não é expansão de histórico).

**Quem esvazia esta Wave 0 sem violar regra escrita, e como:**

- **T1 — Fix remove o seed `$LASTEXITCODE=2` sem substituição → fail-open em PS para trackfw ausente.** Sem o seed, `$LASTEXITCODE` permanece 0 quando trackfw está ausente do PATH → `exit $LASTEXITCODE` = `exit 0` = aprovação silenciosa. Regressão à forma D2. Gate: PS com PATH sem trackfw → deve sair 2.

- **T2 — Fix usa wrapper `/bin/bash -c '...'` como nova forma canônica.** O workaround local funciona em macOS. Adotá-lo como fix viola ADR D2 (string sem caminho fixo) e quebra em Alpine, Windows sem Git Bash no PATH fixo. O `validate` passaria a aceitar uma forma que falha em outros ambientes.

- **T3 — `unsetopt EQUALS` como prefixo é insuficiente: dois pontos de quebra, não um.** Medido: `unsetopt EQUALS 2>/dev/null` resolve o segmento 1, mas o segmento 4 ainda causa `no matches found: N=2*!!N` (NOMATCH). Uma correção parcial passa em sh/bash mas ainda falha em zsh.

  **Candidato verificado:** `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; ` prefixado à linha atual resolve AMBOS os breakpoints:

  | Shell | allow | deny | absent | err1 |
  |-------|-------|------|--------|------|
  | `/bin/sh -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ |
  | `/bin/bash -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ |
  | `/bin/dash -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ |
  | `/bin/zsh -c` | 0 ✓ | 2 ✓ | 2 ✓ | 2 ✓ |
  | `/bin/zsh -f -c` | 0 ✓ | — | — | — |

  Comportamento do prefixo: em sh/bash/dash, `unsetopt` é CommandNotFound (exit 127, stderr swallowed por `2>${null-/dev/null}`), shell continua. Em zsh, `unsetopt EQUALS NOMATCH` desabilita ambas as expansões; `$LASTEXITCODE=2` vira `=2` — agora CommandNotFound (não EQUALS abort), stderr swallowed; `N=2*!!N` vira glob sem matches — agora CommandNotFound (não NOMATCH abort), stderr swallowed. PS: `unsetopt` é CommandNotFoundException (não define `$LASTEXITCODE`), seed `$LASTEXITCODE=2` funciona normalmente. **PS não foi medido diretamente nesta Wave 0 — ver R1.**

- **T4 — `trackfw update` não reconhece o workaround → duplicata ou não-migração.** As listas `legacyD2*`, `legacyD11*` em `agentfiles.go:513–551` são o registro de formas reconhecidas. O workaround `/bin/bash -c '...'` não está nessa lista. Antes da correção do gerador, `trackfw update` preserva o workaround como "hook desconhecido" E injeta a nova forma → duplicata.

- **T5 — Validador não atualizado junto com o gerador → violação espúria.** Se `guardExpectedLine()` em `validator_guard_binary_probe.go:83` retornar a forma antiga após a correção do gerador, `trackfw validate` reportará violação para hooks corretos. O PIN de `check-validate-rule-pins.sh:563` deve ser atualizado no mesmo PR.

#### 3. Falsificação por superfície (ambas as direções)

**Superfície A: Constantes em `agentfiles.go:490–492`**

| Direção | O que quebra | Gate que detecta |
|---------|-------------|-----------------|
| Regressão ao defeito | `/bin/zsh -c "$LINE"` com allow stub → exit 1 | Teste subprocess + gate ML-1A (zsh matrix) |
| Corrige demais — deny-all | `/bin/zsh -c "$LINE"` com allow stub → exit ≠ 0 | Mesmo teste, braço allow |
| Perde seed PS | `powershell -Command "..."` sem trackfw → exit 0 (fail-open) | Gate ML-1A: PS matrix (VM Windows) |
| Perde normalização PS | PS com fake trackfw exit 1 → exit 1 | Gate ML-1A: PS matrix (VM Windows) |
| Stdin JSON não chega ao guard | `/bin/zsh -c "$LINE"` com stub que lê stdin → stdin vazio | Teste subprocess com stdin JSON |
| Fix silencia deny reason no stderr | `/bin/zsh -c "$LINE"` nega mas stderr ausente | Teste subprocess: captura stderr |

**Superfície B: `validator_guard_binary_probe.go:83`**

| Direção | O que quebra | Gate que detecta |
|---------|-------------|-----------------|
| Não atualizado (forma antiga) | `trackfw validate` viola hooks corretos | `scripts/check-validate-rule-pins.sh` (PIN deve ser atualizado junto) |
| Atualizado para forma errada | `trackfw validate` aprova hooks quebrados | PIN com forma correta |

**Superfície C: Asserções de teste**

| Direção | O que quebra | Gate que detecta |
|---------|-------------|-----------------|
| `wantCmd` antigo, gerador corrigido | `go test` falha | `make test` |
| `wantCmd` novo, gerador não corrigido | testes passam, hooks broken em zsh | Reconciliação: cada `wantCmd` declara a constante que afirma |

**Superfície D: Arquivos versionados (`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`)**

| Direção | O que quebra | Gate que detecta |
|---------|-------------|-----------------|
| Não migrados pelo ML-1A | hooks versionados permanecem broken em zsh | `git grep -F '$LASTEXITCODE=2' .claude/settings.json .codex/hooks.json .gemini/settings.json` não deve retornar a forma antiga após o fix |
| Migrados sem dedup | duplicata de hook no arquivo | Teste de migração com fixture versionada + dedup check |

#### 4. Resíduo declarado

**R1 — PS 5.1 e PS 7 não medidos diretamente nesta Wave 0 para o candidato `unsetopt EQUALS NOMATCH ...`.** A ADR D11 anota que PS 5.1 não tem `||`/`&&` (medição anterior que descartou `|| exit 2`), então o seed `$LASTEXITCODE=2` permanece o mecanismo de fail-closed para PS. O comportamento do prefixo `unsetopt EQUALS NOMATCH` em PS — CommandNotFoundException para `unsetopt`, sem efeito em `$LASTEXITCODE` — é o esperado por raciocínio sobre o modelo PS, mas não foi medido na VM. O ML-1A **deve** medir o candidato em `powershell -NoProfile -Command "..."` nos 4 casos (allow/deny/absent/err1) antes de adotar.

**R2 — Claude Code macOS: shell executor não confirmado independentemente.** Evidência desta sessão: `$ZSH_VERSION=5.9` impresso pelo tool "Bash" indica zsh. Se Claude Code invoca hooks via o mesmo shell, `.claude/settings.json` também é executado em zsh → mesma falha. Não medido em processo separado de hook real; registrado como maior prioridade de medição no ML-1A.

**R3 — Gemini, Cursor, Copilot, Windsurf: shell executor não confirmado.** Classificados como "inferência: bash" na tabela. Se algum herdar `SHELL=/bin/zsh` em macOS, o defeito se replica. Esta Wave 0 não mede esses CLIs.

**R4 — `set -e` / `ERR_EXIT` via `.zshenv` ou `BASH_ENV` quebra o candidato.** Medido: `PATH="$D" /bin/zsh -c "set -e; $FIXED_LINE"` → exit 127 (fail-open) porque `unsetopt` falha em sh/bash. Se o usuário tem `set -e` em `.zshenv` (carregado em zsh não-interativo), o CommandNotFound do prefixo aborta antes do guard. Esta é a classe de defeito do design polyglot inteiro (pre-existente à D11), não específica desta correção. Resíduo declarado: não coberto.

**R5 — Workaround local do KG não é migrado automaticamente até o ML-1A ser executado com `trackfw update`.** O backup em `~/.codex/hooks/backups/20261020T110406812571Z/` e o JSON global com workaround estão fora do repositório e não afetam os artefatos versionados. O ML-1A deve incluir fixture com workaround e verificar migração sem duplicata.

**Gates da wave:**
```bash
grep -qF 'agentfiles.go' docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
grep -qF 'validator_guard_binary_probe.go' docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
grep -qF 'unsetopt EQUALS NOMATCH' docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
grep -qF 'zsh deny=2' docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
grep -q '#### 4. Resíduo declarado' docs/roadmaps/wip/ROADMAP-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md
```

### Auditoria do arquiteto da Wave 0 (zeus-tf, 2026-10-21)

- **Medição confirmada no host (macOS)**, com um `trackfw` falso saindo 0/2/1: a linha atual devolve `0/2/2` em `/bin/sh` e bash, e `1/1/1` em zsh. 🔴 No zsh, o **deny também sai 1**, que não é bloqueante: o guard deixa passar (fail-open), não só "não executa". O candidato `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; ` + linha devolve `0/2/2` nos três, e `2` com o trackfw ausente no zsh.
- 🔴 **Correção da tabela CLI × shell:** "Claude Code no macOS executa hooks em zsh" **não se sustenta**. O `$ZSH_VERSION` medido veio do shell da ferramenta Bash, não do executor de hooks. Contraprova direta nesta sessão: o hook global `~/.claude/settings.json` (`... trackfw guard git-branch ...`, a mesma linha polyglot) **negou** `git worktree remove --force` com a mensagem do guard. Em zsh, ele teria abortado com exit 1. A superfície `.claude/settings.json` continua no escopo do ML-1A porque a constante é compartilhada, mas **não** como sítio quebrado medido no macOS. Afetados medidos: Codex. Gemini: shell do executor não medido (inferência).
- O ML-1A exige a matriz em PowerShell 5.1 e cmd na VM Windows (resíduo R1) antes de adotar o candidato.

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

## Evidência e retomada

Reprodução 2026-10-20: todas as nove entradas de guard instaladas falham zsh (exit 1) e passam Bash (exit 0). Semente isolada `=2 2>/dev/null; printf reached`: zsh aborta sem imprimir; Bash imprime reached. Workaround local envolve só strings de comandos de guard em Bash explícito, com backup.

Retomada em 2026-10-21: o #590 foi mergeado (81130826) e não bloqueia mais. Roadmap em wip, branch `fix/hooks-de-guard-gerados-falham-sob-zsh` criada a partir de origin/main (3c0cc1f2). Sem implementação de produto ainda.

Indício adicional (2026-10-21): com o workaround `/bin/bash -c` no `~/.codex/hooks.json` global, `TestValidateJSONFlag`/`TestValidateTextUnchanged` falham localmente também em origin/main, e passam com HOME isolado. Isso mostra que o validador (regra de forma esperada do hook global) e os testes leem o HOME real. A migração precisa levar em conta a forma do workaround.

### ML-1A — Definir e implementar geração compatível e migração
**Status:** ✅ Concluído
**Arquivos afetados:** internal/generators/agentfiles.go, internal/generators/agentfiles_test.go, internal/generators/update_test.go; localizar consumidores de validação antes do handoff.
**Ações:** depois da Wave 0, decidir variante por shell/plataforma com base em prova real, migrar as três constantes de guard e todas as entradas legadas reconhecidas; preservar hooks desconhecidos. Não transportar o wrapper local macOS para Windows sem validação.
**Critérios de aceite:** matriz zsh/sh/bash allow/deny/ausente/erro e regressão Windows; ausência de duplicação; testes de preservação.
**Comandos de validação:** `go test ./internal/generators ./internal/validator ./internal/guard`; `go build ./...`; `make quality`.

**Auditoria (zeus, 2026-10-21):** prefixo `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; `. `TestGuardLineShellMatrix` com 60 subtestes (sh/bash/zsh/zsh -f × allow/deny/erro/ausente, stdin e motivo preservados) + braço do defeito antigo; o teste reprova numa cópia com a correção revertida (`zsh:1: 2 not found`). Migração medida no binário sobre uma cópia do `~/.codex/hooks.json` real: 4 wrappers `/bin/bash -c` → forma nova, 6 entradas, 0 duplicatas, personalizados preservados, idempotente. Windows (VM ARM64, `docs/portabilidade/2026-10-21-guard-zsh-candidato-windows.md`): PS 5.1 e Git Bash dão o mesmo resultado na linha atual e na candidata (0/2/2/2). O cmd.exe não recebe a polyglot (ADR D11, linha 160: Kiro/Amazon Q usam `trackfw guard <nome>`), então o rc=1 medido ali não corresponde a artefato emitido. pwsh não estava na VM. Nota: o trackfw v10.0.0 instalado reporta `guard_wiring_removed` para a forma nova (não a conhece). O CI compila a partir do código do PR.

### ML-1B — Corretivo do red-team: `emulate -R sh`, ordem migração→merge, validador e cobertura
**Status:** 🔄 Em andamento (falta o teste discriminante do B-1)
**Squad:** apolo-tf (implementação) · artemis-tf (matriz Windows da nova forma) · auditoria zeus-tf
**Origem:** ML-2A. O Hades reprovou com o B-1 (duplicata do guard de credencial em `InjectCodexHooks`/`InjectGeminiHooks`, confirmada pelo arquiteto nos configs commitados) e achou resíduos do zsh não declarados. O Hefesto aprovou com A1 (rótulo "ML-7A") e A2 (sem teste da migração do wrapper `/bin/bash -c`). Pela regra de "defeito conhecido corrige agora", tudo entra aqui.
**Decisão (arquiteto, medida no host com o script de matriz em bash):** trocar o prefixo `unsetopt EQUALS NOMATCH` por `emulate -R sh 2>${null-/dev/null}; `. Matriz allow/deny/erro/ausente:
| shell | antiga | unsetopt | emulate -R sh |
|---|---|---|---|
| zsh, zsh -f | 1/1/1/1 | 0/2/2/2 | 0/2/2/2 |
| zsh -o errexit | 1/1/1/1 | 127/127/127/127 | 0/2/2/2 |
| zsh -o nounset | 1/1/1/1 | 1/1/1/1 (fail-open) | 0/2/2/2 |
| zsh -o cshnullglob / nullglob | 1/1/1/1 | ausente: `cat` lê o stdin | 0/2/2/2 |
| dash, /bin/sh, bash | — | 0/2/2/2 | 0/2/2/2 |
A forma `unsetopt` nunca foi publicada, então não vira legado.
**Arquivos afetados:** `internal/generators/agentfiles.go`, `internal/generators/update.go`, `internal/generators/guard_line_shell_test.go`, `internal/generators/*_test.go`, `internal/validator/validator_guard_binary_probe.go`, `internal/validator/validator_guard_wiring.go`, testes do validador, `scripts/check-validate-rule-pins.sh`, `.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `docs/cli-parity.md`, ADR-2026-10-04 (adendo), README se citar a string.
**Critérios de aceite:**
- [ ] forma nova com `emulate -R sh`; matriz por subprocess inclui dash, zsh -o errexit, -o nounset, -o cshnullglob, além de sh/bash/zsh/zsh -f; braço que mostra a forma `unsetopt` falhando sob `-o nounset`
- [ ] Codex/Gemini: todas as migrações antes dos merges; testes de regressão "forma legada → 1 entrada por matcher" para Codex e Gemini; configs versionados sem duplicata
- [ ] `guardWiringTypeFromCmd` reconhece as formas do wrapper `/bin/bash -c`; teste
- [ ] teste da migração do wrapper `/bin/bash -c` (A2)
- [ ] rótulo estável `REQ-2026-10-20` no lugar de "ML-7A" (A1)
- [ ] Windows: PS 5.1 e Git Bash 0/2/2/2 com a nova forma

**Auditoria (zeus, 2026-10-21):** `emulate -R sh` aplicado; matriz com dash/errexit/nounset/cshnullglob e o braço `UnsetoptInsuficiente` PASS; Windows (Artemis, rodada 2) 0/2/2/2 em PS 5.1 e Git Bash; configs sem duplicata; pins e validate OK. 🔴 **O teste do B-1 não discrimina:** `TestInjectCodexHooks/Gemini_LegacyMigrationNoDuplicate` passa também com o `agentfiles.go` anterior (mutação feita pelo arquiteto numa cópia da árvore). O caminho que produziu as duplicatas commitadas no ML-1A ainda não está identificado. As tentativas do arquiteto com o binário anterior não exercitaram o config de projeto (a entrada nem foi migrada), então não valem como medição. Achado A6 da Artemis: no PS 5.1 o redirect `2>${null-/dev/null}` não suprime o CommandNotFound; o prefixo acrescenta uma segunda linha de erro no stderr; aguarda decisão do KG.

## Wave 2 — Auditoria final

### ML-2A — Segurança, qualidade e contrato
**Status:** ⬜ Pendente
**Arquivos afetados:** docs/cli-parity.md, ADR-2026-10-04, CHANGELOG.md; relatório de revisão.
**Ações:** revisar que nenhum erro sai fail-open, nenhuma política foi removida e o artefato gerado foi executado nos shells declarados.
**Critérios de aceite:** revisão independente, gates verdes e prova reproduzível.
**Comandos de validação:** `make quality`; `trackfw validate`.
