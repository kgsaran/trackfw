---
status: wip
date: 2026-11-12
req: "docs/req/REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md"
squad: "hades-tf, ares-tf, zeus-tf"
---

# Roadmap: Guard do parity reprova lista de valores ruins e deixa passar resultado de needs nao documentado

> Created: 2026-11-12 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md -->
REQ: docs/req/REQ-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md
ADR: docs/adr/ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md
Origem: Discussion #664, comentário de 2026-11-06 (lourivalgarciajunior). Nenhuma issue aberta trata o mesmo
mecanismo (varredura de 2026-11-12: zero issues abertas).

O primeiro passo do `parity` reprova uma lista de valores ruins de `needs.*.result`; com `abandoned`, valor não
documentado que o fork observou, ele não dispara. A ADR troca para lista de permitidos (`success`) num script
testável fora do CI.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1 — inventário medido (ML-0A)
- [ ] AC2 — `scripts/check-needs-success.sh` com `--self-test` (ML-1A)
- [ ] AC3 — `parity` e demais sítios do AC1 usando o script (ML-1A)
- [ ] AC4 — comentários de `quality.yml` coerentes (ML-1A)
- [ ] AC5 — self-test no `parity-rest`, com contra-braço (ML-1A)
- [ ] AC6 — log do `parity` no CI do PR (ML-3A)
- [ ] AC7 — `make quality` verde (ML-2B)
- [ ] AC8 — resposta na Discussion #664 (ML-3A)

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model e inventário (AC1)
> Dependencies: none. Blocks all implementation.

### ML-0A — Inventário dos consumidores de resultado de job e threat model
**Status:** ✅ Done
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-12-wave0-agregador-needs-success.md`

**Actions:**
1. Inventário (AC1): todo consumidor de `needs.*.result`, `needs.<job>.result`, `contains(needs`, `success()`,
   `failure()`, `cancelled()` e variável `*_RESULT` em `.github/workflows/*.yml` e `scripts/`. Classifique cada
   um como permitidos, proibidos ou diagnóstico (sem efeito em merge), com a contagem conferida por um segundo
   caminho. Diga, para cada job com `if: always()`, o que decide o veredito dele.
2. A segunda linha de defesa do `parity`: leia os passos seguintes ao da guarda e diga, por leitura, se um
   `needs` com `abandoned` e sem artefato seria reprovado. Ela não justifica manter a guarda atual (ADR,
   alternativas), mas o resultado calibra a gravidade.
3. Threat model do script do ADR D2: como ele aprova sem verificar (por exemplo, `toJSON(needs)` vazio quando o
   job não tem `needs`, um job ausente do JSON, `null`, diferença de maiúsculas, `jq` ausente no runner) e como
   reprova o que é legítimo.
4. Residual declarado.
**Acceptance criteria:**
- [x] Inventário com contagem conferida por um segundo caminho e lista final de sítios do ML-1A
- [x] Nenhuma linha de implementação escrita neste ML

**Auditoria (Zeus, 2026-11-12):** aprovado, com três correções para o ML-1A. Conferido no relatório e no
`quality.yml`: o único sítio de lista de proibidos é `quality.yml:1010`. Os scripts de release já exigem
`success`, e `windows-census.yml` é só `workflow_dispatch`. A §2 também confere: depois da guarda, o `parity`
só tem `checkout`, `setup-python`, o download dos artefatos de shard e `check-falsify-shard-coverage.sh`.
`parity-other-gates` (795) e `check-required-checks` não sobem artefato, então `abandoned` neles hoje dá
`parity` verde sem verificação. `git status` mostra só o relatório, e o índice está vazio. Correções:
1. **R2 não fica como residual (defeito conhecido se corrige agora).** O script recebe os nomes esperados como
   argumentos e exige igualdade de conjunto com as chaves de `NEEDS_JSON`: falta ou sobra de job reprova. Uma
   mudança em `needs:` sem atualizar a chamada falha fechada. Isso entra como braços `missing-expected-job` e
   `unexpected-job`.
2. **R5 não é residual novo.** Num PR, quem pode trocar o script também pode trocar o `quality.yml`. A classificação
   pela BASE do ADR-2026-09-17 existe porque o `classify` *dispensa* trabalho; esta guarda não dispensa nada.
3. **O contra-braço da §3.5 não vale como está escrito.** `sed -i` sem sufixo quebra no BSD/macOS, e
   `/tmp/sabotado.sh` fixo viola `check-mktemp-cleanup.sh`. O contra-braço vai dentro do `--self-test`, com
   `mktemp` e trap na forma canônica da ADR-2026-11-11, uma sabotagem portátil (`sed '…' > arquivo`) e uma
   verificação de que a sabotagem mudou o arquivo (se o `sed` não casar, o braço reprova em vez de ficar
   inerte).

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-12-wave0-agregador-needs-success.md
```

## Wave 1 — Implementação (AC2, AC3, AC4, AC5)
> Dependencies: Wave 0 auditada. Um ML só: o script, o workflow e o `Makefile` são uma mudança.

### ML-1A — `check-needs-success.sh` e troca da guarda do `parity`
**Status:** ⬜ Pending
**Squad:** ares-tf
**Files affected:**
- `scripts/check-needs-success.sh` (novo)
- `.github/workflows/quality.yml`
- `Makefile`

**Actions:**
1. Script do ADR D2 em bash que delega o parse a `python3` (ML-0A §3.1). Contrato:
   `NEEDS_JSON=... scripts/check-needs-success.sh <job> [<job>...]`. Reprova (rc≠0) se a variável faltar ou
   não for um objeto JSON, se o conjunto de chaves diferir dos argumentos, ou se algum `result` não for
   exatamente a string `success`. Sem argumentos, reprova. Ao reprovar, imprime `::error::` com cada job e
   resultado, escapados por `%`→`%25`, `\r`→`%0D`, `\n`→`%0A`. Nunca ecoa `outputs`.
   `--self-test` tem os 23 braços da tabela §3.4 do relatório ML-0A, mais `missing-expected-job`,
   `unexpected-job`, `no-args` e o contra-braço (correção 3 da auditoria do ML-0A).
2. No `parity`, remover o passo com `contains(needs.*.result, …)` e pôr, logo depois de `checkout` e
   `setup-python`, antes do download de artefatos:
   `env: NEEDS_JSON: ${{ toJSON(needs) }}` +
   `run: scripts/check-needs-success.sh parity-falsify-shard parity-other-gates check-required-checks`.
3. Reescrever o comentário de `quality.yml` (~948–955) e o do `parity` (~854–859) para o estado novo.
4. `Makefile`, alvo `parity-rest`: uma linha `scripts/check-needs-success.sh --self-test`.
**Acceptance criteria:**
- [ ] `--self-test` verde, em bash e em `/bin/bash` 3.2, com cada braço nomeado
- [ ] Contra-braço: com a lista de proibidos no lugar da de permitidos, o braço `abandoned` reprova
- [ ] `python3 scripts/check-workflow-yaml.py` rc=0
- [ ] Frase de reconciliação por braço novo do self-test (Regra Dura)

**Wave gates:**
```bash
bash scripts/check-needs-success.sh --self-test
python3 scripts/check-workflow-yaml.py
```

## Wave 2 — Red team e qualidade (AC7)
> Dependencies: Wave 1 auditada. ML-2A e ML-2B em sequência (o ML-2B mede o que o ML-2A deixar).

### ML-2A — Red team do script e da troca
**Status:** ⬜ Pending
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-12-wave2-redteam-agregador-needs-success.md`

**Actions:** reimplementar a leitura do contrato a partir da ADR, sem olhar o self-test primeiro; procurar
entrada que aprove sem verificar. Medir só em `t.TempDir`/scratch, sem tocar o índice do git.
**Acceptance criteria:**
- [ ] Relatório com cada vetor tentado e o resultado
- [ ] `git diff --cached` vazio ao final

### ML-2B — `make quality`
**Status:** ⬜ Pending
**Squad:** ares-tf
**Files affected:** nenhum
**Actions:** `make quality` com a máquina ociosa, saída completa em arquivo, rc lido sem pipe.
**Acceptance criteria:**
- [ ] `make quality` rc=0

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-12-wave2-redteam-agregador-needs-success.md
```

## Wave 3 — Fechamento (AC6, AC8)
> Dependencies: Wave 2 auditada e PR aberto.

### ML-3A — Medição no CI e resposta na Discussion
**Status:** ⬜ Pending
**Squad:** zeus-tf
**Files affected:** este roadmap
**Actions:** registrar o log do primeiro passo do `parity` no CI do PR (jobs avaliados e resultado); responder
na Discussion #664 com o link do PR.
**Acceptance criteria:**
- [ ] Log do `parity` citado aqui, com o run id
- [ ] Comentário publicado na Discussion #664
