---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md"
squad: "ares-tf"
---

# Roadmap: job parity do CI so roda 4 de 14 scripts do make parity (falsificacao e identity-parity nunca rodam em CI)

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md`

10 dos 14 scripts de `make parity` (incluindo os 100 cenários de falsificação P4 de
`check-gates-falsify.sh` e `check-identity-parity.sh`) nunca rodam em CI — só se alguém lembrar de
`make quality` local. `.github/workflows/quality.yml`'s job `parity` lista 4 scripts manualmente em
vez de chamar `make parity`, então todo gate novo nasce fora do CI por padrão.

**Dependência bloqueante explícita**: `check-identity-parity.sh` está vermelho no `main` agora
(REQ-2026-08-04-json-marshalindent-do-go-escapa-html...). Ligar esse script no CI **antes** desse
bug ser corrigido deixaria o CI permanentemente vermelho — a ordem de merge importa: a REQ do
HTML-escaping precisa fechar primeiro.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Job `parity` roda `make parity` (ou equivalente sincronizado) em vez de listar scripts
- [x] Tempo de execução medido no ambiente real do GitHub Actions, decisão de job separado/paralelo
      tomada com base na medição, não em suposição
- [x] `release.yml` reavaliado e a decisão documentada
- [x] Nenhum gate liga vermelho — REQ do HTML-escaping fechada antes desta Wave 1

## Wave 1 — Ligar make parity completo ao CI
> Dependencies: REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-...
> precisa estar Done antes de iniciar (bloqueante — ver Context)

### ML-1A — Trocar a lista manual de scripts pelo `make parity` no job `parity`
**Status:** ✅ Concluído
**Files affected:**
- `.github/workflows/quality.yml` (job `parity`)
**Actions:**
1. Confirmar que a REQ-2026-08-04-json-marshalindent-do-go-escapa-html... está `Done` antes de
   começar — se não estiver, este ML fica bloqueado (não iniciar antes).
2. Trocar as 4 linhas `- run: scripts/check-*.sh` do job `parity` por `- run: make parity` (o job já
   tem Go/Node/npm/Python configurados; confirmar que `make parity` não precisa de nenhuma etapa de
   setup adicional além do que o job já faz, ex: `npm ci`, `pip install pypi/`).
3. Rodar o workflow (via PR de teste ou `act`/execução manual) e medir o tempo total do job `parity`
   completo, especialmente a contribuição de `check-gates-falsify.sh`.
4. Se o tempo total for razoável para feedback de PR (julgamento do implementador com base na
   medição — não há número mágico pré-definido nesta REQ, ver P1 em
   `docs/gate-design-principles.md`), manter sequencial. Se for excessivo, separar
   `check-gates-falsify.sh` (e talvez `check-identity-parity.sh`, que também é pesado) num job
   paralelo próprio, documentando a decisão e o tempo medido que a motivou.
**Acceptance criteria:**
- [x] Workflow roda verde de ponta a ponta com `make parity` completo — confirmado no run real da
      PR #132 (`gh api .../jobs/92262047832`): job `parity` completou em **2m46s** (mais rápido que
      a estimativa local de 4m15s-4m52s, runner do GitHub Actions com mais paralelismo/cache),
      `conclusion: success`, e o log confirma os 100 cenários de falsificação (`grep -c "OK   \[falsify"` = 100) mais a linha final "Falsification checks passed (all 100 scenarios...)" — não é
      um atalho, os 14 scripts rodaram de fato
- [x] Tempo de execução registrado nesta seção do roadmap
- [x] `trackfw validate` sem violações

**Medição local (2026-08-05, Apple Silicon, referência — não substitui o run real em
`ubuntu-latest`):**
- `time make parity` completo: **4m15s** (user 173s / sys 92s / cpu 103%)
- `check-gates-falsify.sh` isolado: **3m05s** — sozinho responde por ~73% do tempo total
- Os outros 13 scripts somados: ~1m10s

**Decisão: manter tudo sequencial em `make parity`, sem job paralelo.** Motivo: 4m15s não é
"minutos de dois dígitos" (o critério do roadmap para justificar fragmentação). Além disso o job
`parity` já depende de `needs: [go, node, python, package-smoke,
windows-integrations-resolve]` — ou seja, roda depois desses jobs em paralelo, não é o gargalo
crítico do feedback de PR. Bloco de comentário com essa justificativa e a medição foi adicionado
diretamente em `.github/workflows/quality.yml` acima do `run: make parity`, para não depender só
deste roadmap.

**Mudança aplicada:** as 4 linhas `- run: scripts/check-*.sh` do job `parity` foram substituídas
por `- run: make parity` (com `env: TRACKFW_DISABLE_EXTERNAL_COMMANDS: "1"`, mantendo o mesmo guard
que já existia em `check-integration-assets.sh` — `check-barrier.sh` unseta essa var internamente,
então não há conflito). Nenhum setup adicional foi necessário: o job já tinha Go/Node/Python +
`npm ci --ignore-scripts` + `pip install pypi/`; `make parity` depende de `build`, que compila
`bin/trackfw` via `go build`, já coberto pelo `setup-go` existente no job.

### ML-1B — Reavaliar release.yml
**Status:** ✅ Concluído
**Files affected:**
- `.github/workflows/release.yml`
**Actions:** Decidir e documentar (comentário no `.yml` ou nesta seção do roadmap) se o subconjunto
reduzido de 3 scripts em `release.yml` é intencional (ex: release só roda após `quality.yml` já ter
passado no mesmo commit, então redundância completa seria desperdício) ou se precisa do mesmo
tratamento do ML-1A.
**Acceptance criteria:**
- [x] Decisão registrada, com justificativa
- [x] Se decidido expandir, `release.yml` atualizado e testado (worfklow_dispatch manual ou
      equivalente, sem precisar cortar uma tag real só para testar)

**Decisão: manter o subconjunto reduzido de 3 scripts em `release.yml`, sem expandir para
`make parity` completo.** Justificativa: `release.yml` só dispara em `push: tags: v*`
(`.github/workflows/release.yml` linhas 3-6). Pelo "Protocolo de Release" documentado em
`CLAUDE.md` do projeto, tags só são criadas a partir de um commit já mergeado na `main` via PR —
e `quality.yml` roda em todo `push: branches: [main]` (linha 4-5 de `quality.yml`), incluindo
agora `make parity` completo (14 scripts, ML-1A). Logo, no fluxo normal, o commit que a tag aponta
já passou pelos 14 scripts verdes antes da tag existir — rodar tudo de novo em `release.yml` seria
redundância completa (~4min medidos) sem ganho de sinal, só atraso na publicação da release.

O subconjunto de 3 scripts que já existia (`check-cli-parity.sh`, `check-validate-parity.sh`,
`check-static-assets.sh`) foi mantido como smoke check de defesa contra o caso excepcional de
alguém empurrar uma tag manualmente sobre um commit/branch que nunca passou por `quality.yml`
(branch protection não garante isso — é convenção documentada, não bloqueio técnico do GitHub).
Não expandido porque não há evidência de que a redundância completa seja necessária; comentário
com essa justificativa foi adicionado diretamente em `.github/workflows/release.yml` acima dos 3
`run:`. Não foi possível testar via workflow_dispatch real (o job `release` do workflow depende de
`goreleaser` + secrets de publicação só disponíveis em push de tag real) — a mudança feita foi
apenas de comentário/documentação, sem alteração de comportamento executável, então o risco de
regressão é nulo; validado apenas via `yaml.safe_load` (sintaxe) nesta sessão.
