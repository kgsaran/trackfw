---
status: wip
date: 2026-11-20
req: "docs/req/REQ-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md"
squad: "hades-tf, ares-tf, artemis-tf, hefesto-tf"
---

# Roadmap: Estabilidade do job go: teto de timeout por pacote e ETXTBSY no script falso dos testes de guard

> Created: 2026-11-20 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md -->
REQ: docs/req/REQ-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md

## Diagnóstico
Ver a REQ: duas causas para o mesmo sintoma (job `go` intermitente nos testes trazidos pelo #687/#688).
1. **Teto:** `internal/generators` em 91–121 s contra `-timeout 2m`. Sítios: `quality.yml:40`, `release.yml:43` e
   `Makefile:11`.
2. **`ETXTBSY`:** `makeFakeTrackfw` grava o script falso e o executa sob `t.Parallel()` (golang/go#22315).
Issue: #690 (o PR usa `Closes #690`).

## Acceptance Criteria
- [ ] AC1 — `make test`: `internal/generators` em 5m, demais pacotes em 2m, conjunto de pacotes igual ao `./...`
- [ ] AC2 — `quality.yml` (job `go`) e `release.yml` chamam `make test`
- [ ] AC3 — nenhum helper grava e executa binário sob paralelismo sem proteção contra `ETXTBSY`, medido em Linux com `-count`
- [ ] AC4 — causa da falha exit -1 / stdin vazio medida e escrita
- [ ] AC5 — `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat model (hades-tf)
> Dependências: nenhuma. Bloqueia a Wave 1.

### ML-0A — Threat model e enumeração
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-20-wave0-estabilidade-job-go-threat-model.md` (novo, único arquivo escrito)
**Actions:**
1. Enumeração dos tetos: todo `-timeout` de `go test` em `.github/`, `Makefile` e `scripts/`, dizendo qual entra e qual fica
   fora, com o motivo.
2. Enumeração do `ETXTBSY`: todo `_test.go` que grava um arquivo executável (`0755`, `0o755`, `0700`, `os.Chmod`) e depois o
   executa, direta ou indiretamente (PATH, shell, `exec.Command`), e se roda sob `t.Parallel()` ou em pacote com testes
   paralelos.
3. Ameaças: quem esvazia o AC sem quebrar regra escrita? Exemplos: teto alto que esconde regressão real; `make test` que
   testa menos pacotes que `./...`; proteção que serializa a suíte inteira; `t.Skip` ou retry que mascara falha real.
4. Alvos de falsificação nas duas direções para AC1–AC3, e o residual declarado.
**Acceptance criteria:**
- [ ] as quatro seções respondidas com evidência (comando e saída), não com afirmação
- [ ] nenhuma linha de implementação

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-20-wave0-estabilidade-job-go-threat-model.md
```

## Wave 1 — Implementação (1A ∥ 1B; arquivos disjuntos)
> Dependências: Wave 0 auditada.

### ML-1A — Teto por pacote em ponto único (ares-tf)
**Status:** ⬜ Pending
**Files affected:** `Makefile`, `.github/workflows/quality.yml` (só o job `go`), `.github/workflows/release.yml`
**Actions:**
1. Alvo `test` do `Makefile`: `internal/generators` com `-timeout 5m`, demais pacotes (`go list ./...` sem ele) com
   `-timeout 2m`, ambos com `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`. Falha de qualquer um reprova o alvo.
2. `quality.yml:40` e `release.yml:43`: trocar o `go test -timeout 2m ./...` por `make test`, mantendo o `env`.
3. Os demais sítios que a Wave 0 marcar como "entra".
**Acceptance criteria:**
- [ ] contagem de pacotes: `go list ./... | wc -l` igual à soma dos dois conjuntos do alvo
- [ ] falsificação: um pacote não-generators com teste que dorme 150 s reprova em 2m; o mesmo sono no generators passa
  (medido em módulo temporário ou com `-run`, sem commitar o teste)
- [ ] `make parity-rest` rc=0 (pins de recipe do `Makefile`)

### ML-1B — `ETXTBSY` nos helpers de teste (artemis-tf)
**Status:** ⬜ Pending
**Files affected:** os `_test.go` enumerados na Wave 0 (começando por `internal/generators/guard_line_shell_test.go`)
**Actions:**
1. Gravar o executável sem janela de descritor de escrita aberto durante um fork concorrente. Ponto de partida:
   escrita e fechamento sob `syscall.ForkLock.Lock()`, num helper único usado por todos os sítios.
2. Medir em Linux, antes e depois: `go test -count=N -run 'TestGuardLineShellMatrix' ./internal/generators/`, com
   contêiner ou VM. Mostrar o `Text file busy` antes e a ausência depois.
3. Investigar a falha com exit -1 e stdin vazio (121b8ce5) e escrever a causa medida.
**Acceptance criteria:**
- [ ] antes/depois em Linux com contagem de falhas
- [ ] nenhum teste removido, pulado ou desparalelizado
- [ ] Regra de Reconciliação: uma frase por teste novo ou alterado

## Wave 2 — Revisão (hefesto-tf)
### ML-2A — Revisão de qualidade do diff da Wave 1
**Status:** ⬜ Pending
**Files affected:** `docs/qualidade/2026-11-20-estabilidade-job-go-review.md`
- [ ] parecer com APROVA/REPROVA por AC

## Fechamento
### ML-FB — `make quality` e CI verde
**Status:** ⬜ Pending
- [ ] `make quality` MQ_RC=0 lido pelo zeus no log
- [ ] CI verde no PR, com o log do job `go` mostrando os dois tetos
