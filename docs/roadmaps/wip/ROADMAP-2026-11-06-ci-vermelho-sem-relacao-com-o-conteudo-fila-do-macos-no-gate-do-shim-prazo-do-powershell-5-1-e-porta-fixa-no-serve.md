---
status: wip
date: 2026-11-06
req: "docs/req/REQ-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md"
squad: ""
---

# Roadmap: CI vermelho sem relação com o conteúdo: fila do macOS no gate do shim, prazo do PowerShell 5.1 e porta fixa no serve

> Created: 2026-11-06 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md

Três causas, um ML para cada. O diagnóstico está no ADR-2026-11-06. Varredura de issues: a única aberta é a #665, absorvida aqui; a #664 é Discussion.

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-06-wave0-ci-vermelho-por-infraestrutura.md`
**Acceptance criteria:**
- [x] o conjunto fechado de caminhos inertes definido e justificado: nenhum deles altera o que o `shim-byte-identity` compara (critério do ADR D1; o critério original, "nenhum entra no wheel nem em script de gate", encolheria o conjunto a `.claude/agent-memory/` e foi emendado na auditoria da Wave 0)
- [x] os modos de evasão da classificação de diff analisados (rename, symlink, base forjada, `pull_request_target`, clone raso, evento diferente), cada um com a defesa correspondente
- [x] o retry de porta e o prazo derivado analisados quanto a mascarar defeito real

## Wave 1 — Implementação (ML-1A e ML-1B em paralelo; ML-1C depois)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos. O ML-1C vem depois para respeitar o teto de 2 agentes simultâneos.

### ML-1A — Gate do shim com classificação de diff (ares-tf) — AC1, AC2
**Status:** ✅ Concluído
**Files affected:** `.github/workflows/quality.yml`, `scripts/check-shim-diff-applies.sh`, `docs/cli-parity.md`
**Requisitos obrigatórios:** os 11 itens do fim do threat model da Wave 0; a matriz `shim-byte-identity` usa `!cancelled()`, não `always()`. Varredura de contrato feita na auditoria: só o próprio `quality.yml` afirma "o gate só aceita `success`"; `scripts/check-workflow-yaml.py` confere que todo `needs` aponta para job existente.
**Gates:** `bash scripts/check-shim-diff-applies.sh --self-test`, `python3 scripts/check-workflow-yaml.py`, `python3 scripts/check-required-status-checks.py --self-test`, `make parity-rest`.
**Acceptance criteria:**
- [x] o script classifica o diff do PR por lista fechada de caminhos inertes, com `git diff --name-only --no-renames` contra o merge-base e checkout `fetch-depth: 0`, e tem `--self-test` cobrindo os casos do AC2; o self-test roda no CI
- [x] o gate passa com `não aplicável` só em `pull_request` contra a branch padrão com diff inteiramente dentro do conjunto, e exige `success` em todos os outros casos
- [x] o comentário do ML-4E no `quality.yml` passa a explicar a nova regra
**Auditoria:** a primeira entrega derivava o repositório de `SCRIPT_DIR`, e a cópia da BASE em `$RUNNER_TEMP` caía sempre em D2 (dispensa morta). O corretivo usa `git rev-parse --show-toplevel` do cwd, e o caso T10 roda o script fora do repositório; o contra-braço reprova T10a.

### ML-1B — Prazo do PowerShell derivado do aquecimento (artemis-tf) — AC3
**Status:** ✅ Concluído
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Acceptance criteria:**
- [x] função de prazo com teste unitário dos três regimes (aquecimento curto → 15 s, médio → 4×, longo → 60 s)
- [x] mensagem de estouro cita o aquecimento e o prazo
- [x] depois do primeiro estouro de prazo de um binário, as chamadas seguintes desse binário reprovam sem executar, citando a chamada que estourou (emenda D3); pior caso de cerca de 60 s por binário, dentro do `-timeout 20m`
- [x] `go test ./internal/generators/` verde

### ML-1C — Porta livre no teste de endereço do serve (apolo-tf) — AC4
**Status:** ✅ Concluído
**Files affected:** `scripts/check-serve-address-parity.sh`
**Acceptance criteria:**
- [x] porta obtida do sistema; retry só em `address already in use`, até 3 vezes
- [x] contra-braço: com a porta ocupada de propósito, o cenário passa; com o retry desligado, reprova
- [x] `make parity-rest` verde

### ML-1D — `grep -q` sob `pipefail` dá 141 intermitente (ares-tf) — AC6
**Status:** ✅ Concluído
**Origem:** achado na auditoria do ML-1A. Em `set -o pipefail`, `produtor | grep -q P` reprova com 141 quando o `grep -q` sai cedo e o produtor ainda escreve (SIGPIPE). Medido: `grep -v '^\s*#' .github/workflows/quality.yml | grep -q check-shim-diff-applies` deu `141 141 0 0 0 0 141 141` em 8 execuções. É o mesmo efeito desta REQ (vermelho sem relação com o conteúdo), por isso entra aqui. Paralelo ao ML-1C: arquivos disjuntos.
**Files affected:** `scripts/check-adr-enumeration-single-point.sh`, `scripts/check-channels-content.sh`, `scripts/check-ci-workflow-pin-parity.sh`, `scripts/check-crlf-normalize-capture.sh`, `scripts/check-falsify-shard-coverage.sh`, `scripts/check-gates-falsify.sh`, `scripts/check-git-branch-guard-hook-schema.sh`, `scripts/check-goreleaser-prerelease.sh`, `scripts/check-init-preserves-user-config.sh`, `scripts/check-install-checksum.sh`, `scripts/check-orphan-gates.sh`, `scripts/check-platform-matrix-parity.sh`, `scripts/check-raw-read-ban.sh`, `scripts/check-release-tag-parity.sh`, `scripts/check-req-roadmap-rule-is-status-aware.sh`, `scripts/check-roadmap-slug-matching.sh`, `scripts/check-serve-api-file-security.sh`, `scripts/check-serve-browser-security.sh`, `scripts/check-shim-diff-applies.sh`, `scripts/check-symlink-privilege-guard.sh`, `scripts/check-update-parity.sh`, `scripts/check-wheel-metadata-readme.sh`, `scripts/check-write-containment.sh`, `scripts/run-gates-falsify-parallel.sh`, `scripts/run-gates-falsify-shard.sh`, `scripts/check-parity-call-site-pins.sh`, `scripts/check-usage-silencing.sh`, `scripts/check-no-grep-q-under-pipefail.sh`, `Makefile`
**Acceptance criteria:**
- [x] nenhum `| grep -q` resta nos scripts com `pipefail`; a troca é `| grep P >/dev/null` (lê a entrada inteira, mesmo código de saída)
- [x] `scripts/check-no-grep-q-under-pipefail.sh` reprova script com `pipefail` e `| grep -q`, tem `--self-test` com contra-braço, e roda no `parity-rest`
- [x] o laço de 20 execuções do pipeline medido acima dá 0 em todas depois da troca no `check-orphan-gates.sh`
- [x] `make parity-rest` verde
**Auditoria:** 96 trocas, só em linhas de `grep`. Dois arquivos entraram fora da lista inicial pela mesma causa: `check-parity-call-site-pins.sh` (`| grep -vqE`, que a varredura por `| *grep -q` não casava) e `check-usage-silencing.sh` (sem `pipefail`, troca preventiva). Medido: forma antiga 18/20 com 141, forma nova 0/20.

## Wave 2 — Red team
> Dependencies: Wave 1 completa.

### ML-2A — Red team da classificação de diff (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-06-redteam-ci-vermelho-por-infraestrutura.md`
**Acceptance criteria:**
- [x] cada modo de evasão da Wave 0 tentado contra o script, com o resultado medido em fixture local (`git write-tree` em diretório temporário, sem PR nem branch)
- [x] T8 (retarget) respondido pela lista de gatilhos do `quality.yml`, sem PR real; o primeiro PR só de documentação depois do merge fica declarado como a medição real da dispensa
- [x] veredito escrito
**Veredito:** REPROVA, com 3 bloqueantes. Os três viram o ML-2B; o ML-2C reverifica.
**Auditoria:** o relatório não cita uma entrada `internal/x.go` que apareceu no índice do repositório real durante o ML-2A. Foi removida pelo arquiteto com `git rm --cached`, sem commit nem branch.

### ML-2B — Corretivo dos bloqueantes do red team (ares-tf)
**Status:** ✅ Concluído
**Files affected:** `.github/workflows/quality.yml`, `.github/workflows/windows-census.yml`, `.github/workflows/release.yml`, `scripts/check-shim-diff-applies.sh`, `scripts/check-no-grep-q-under-pipefail.sh`
**Acceptance criteria:**
- [x] B1: no job `classify`, a classificação pela BASE roda antes do `--self-test` do HEAD; nenhum passo do HEAD roda antes dela
- [x] B2: todo nome de arquivo impresso pelo classificador tem os caracteres de controle trocados por `?`; o self-test prova que nome com `\n` não produz linha começando com `::`
- [x] B3: nenhum `| grep -q` resta em bloco `run:` de `.github/workflows/*.yml`, e o gate de `grep -q` passa a varrer os workflows
- [x] `make parity-rest` verde
**Auditoria:** a premissa "o bash do Actions é `-eo pipefail` por padrão", que veio do handoff do arquiteto, era falsa: sem `shell:` o padrão é `bash -e {0}`. A regra do gate continua uniforme em workflow, e a justificativa nos comentários foi corrigida. `release.yml:83` entrou pela mesma causa.

### ML-2C — Reverificação do red team (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-06-redteam-ci-vermelho-por-infraestrutura.md`
**Acceptance criteria:**
- [x] B1, B2 e B3 reatacados contra o código do ML-2B, com a medição escrita
- [x] veredito final escrito
**Veredito:** APROVA COM RESSALVAS. B1, B2 e B3 fechados por medição independente; dois residuais LOW declarados (R11: ::set-output root *.md com 2 espaços no inert_list; R12: braço YAML do gate sem anti-vacuidade). `make parity-rest` rc=0, shim self-test 27/27.
**Auditoria:** o executor marcou o próprio ML no roadmap, que estava fora do escopo dele. O texto foi conferido e mantido. O índice do repositório estava limpo nos três pontos medidos. O R11 é benigno: o valor seria `true.md`, com indentação, e o runner ignora `::set-output`. O R12 é defeito do gate e vira o ML-2D.

### ML-2D — Guarda de vacuidade no braço YAML do gate de `grep -q` (ares-tf)
**Status:** ✅ Concluído
**Files affected:** `scripts/check-no-grep-q-under-pipefail.sh`
**Acceptance criteria:**
- [x] o gate reprova quando não encontra nenhum `*.yml` no diretório de workflows varrido, como já faz com `scripts/*.sh`
- [x] o `--self-test` tem um caso para diretório de workflows vazio, que reprova
- [x] `make parity-rest` verde

### ML-2E — Nova tentativa do PowerShell só com sinal de máquina parada (artemis-tf) — AC3
**Status:** ✅ Concluído
**Origem:** CI do PR #669, run 42253593127. A premissa da D3 foi falsificada; ver a segunda emenda da D3 no ADR. É a mesma causa da #665.
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Acceptance criteria:**
- [x] depois de um estouro, uma sonda `exit 0` sem cache decide: sonda lenta leva a uma nova tentativa com prazo derivado dela; sonda rápida reprova sem nova tentativa
- [x] o registro de fail-fast só marca o binário depois que a nova tentativa também estoura
- [x] orçamento de uma nova tentativa por binário por processo; esgotado o orçamento, o próximo estouro reprova e marca o binário
- [x] um aquecimento que estoura tem uma nova tentativa; se ela também estourar, a mensagem atribui a falha à máquina
- [x] testes com dublês, sem PowerShell: estouro + sonda lenta + nova tentativa que passa; estouro + sonda rápida reprova com `run` chamado uma vez; estouro + sonda lenta + nova tentativa que estoura reprova e registra; contra-braço sem a condição reprova o caso da sonda rápida
- [x] `go test ./internal/generators/` verde
**Auditoria:** o executor apontou que a nova tentativa que passa não marcava o binário, o que dava pior caso de cerca de 48 min com paradas repetidas. Entrou o orçamento de uma nova tentativa por binário, e o ADR foi emendado. Contra-braços: sem a condição da sonda, sem a nova tentativa e sem o consumo do orçamento, cada um reprova o seu teste.
