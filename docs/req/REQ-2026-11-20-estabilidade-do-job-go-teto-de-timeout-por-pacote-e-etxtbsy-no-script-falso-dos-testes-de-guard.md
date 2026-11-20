---
status: Open
date: 2026-11-20
author: "zeus-tf (decisões do KG em 2026-11-20)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md"
---

# REQ: Estabilidade do job go: teto de timeout por pacote e ETXTBSY no script falso dos testes de guard

> Date: 2026-11-20 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
Issue #690 (Lourival, 2026-11-18). Depois do #687/#688, o `internal/generators` roda na linha do `go test -timeout 2m ./...`
do job `go`. O veredito passou a depender do runner sorteado: o mesmo head sai `ok` ou `FAIL`.

Medido no nosso CI (PR #692, job `go`): `internal/generators` levou de 90,8 s a 119,1 s, e a run 44765077726 (d83ca83a,
commit só de docs) terminou em `panic: test timed out after 2m0s`.

Mesmo sintoma, outra causa, medida nas runs 44635171831 (3ff02a8b) e 44614230783 (121b8ce5):
- `TestGuardLineShellMatrix*` reprovou com `/usr/bin/dash: 1: trackfw: Text file busy` e com exit -1 e stdin vazio.
- `makeFakeTrackfw` (`internal/generators/guard_line_shell_test.go:364`) grava o script falso com `os.WriteFile(..., 0755)`
  e o executa em subtestes `t.Parallel()`.
- Um fork concorrente herda o descritor de escrita, e o kernel recusa o exec (`ETXTBSY`, golang/go#22315).
- Subir o teto não fecha esta causa. Pela Regra Dura, ela fica no mesmo roadmap, em ML separado.

## Decisões do KG (2026-11-20)
- Uma REQ, dois MLs: teto de timeout e `ETXTBSY`.
- Teto por pacote (opção 2 do #690): `internal/generators` com teto próprio de **5m** (2,5x o cluster lento de ~121 s); os
  demais pacotes mantêm **2m** como detector de regressão.
- Ponto único: o alvo `test` do `Makefile` define os dois tetos, e o job `go` do `quality.yml` e o `release.yml` passam a
  chamar `make test`, em vez de repetir o comando.

## Acceptance Criteria
- [ ] `make test` roda `internal/generators` com `-timeout 5m` e todos os demais pacotes com `-timeout 2m`. O conjunto
  de pacotes testados é igual ao do `go test ./...` (contagem medida antes e depois).
- [ ] O job `go` do `quality.yml` e o passo de teste do `release.yml` chamam `make test`. Não sobra `go test -timeout 2m ./...`
  fora dos módulos de sonda do ML-1A (`quality.yml:361/383`) e do corpus de testdata.
- [ ] Nenhum helper de teste grava executável e o executa sob paralelismo sem proteção contra `ETXTBSY`. Todos os sítios
  vêm da enumeração da Wave 0. A correção é medida em Linux com repetição (`-count`), antes e depois.
- [ ] A falha com exit -1 e stdin vazio (121b8ce5) tem causa medida: ou é a mesma do `ETXTBSY`, ou fica escrita como
  outra causa.
- [ ] `make quality` rc=0 e CI verde, com o job `go` rodando os dois tetos.

## Negative scope
- Não divide o pacote `internal/generators` (opção 3 do #690).
- Não muda o `-timeout 20m` do `windows-full-suites`. A margem dele é só medida e registrada (14m13s no PR #692).
- Não altera os módulos de sonda do ML-1A (`quality.yml:361/383`), que testam um pacote sintético.
- Não reduz cobertura: nenhum teste é removido, pulado ou desparalelizado inteiro para caber no teto.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-20-estabilidade-do-job-go-teto-de-timeout-por-pacote-e-etxtbsy-no-script-falso-dos-testes-de-guard.md
