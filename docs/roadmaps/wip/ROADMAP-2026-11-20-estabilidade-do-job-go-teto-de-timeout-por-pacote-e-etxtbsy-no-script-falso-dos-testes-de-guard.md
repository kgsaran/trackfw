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
- [x] AC1 — `make test`: `internal/generators` em 5m, demais pacotes em 2m, conjunto de pacotes igual ao `./...`
- [x] AC2 — `quality.yml` (job `go`) e `release.yml` chamam `make test`
- [x] AC3 — nenhum helper grava e executa binário sob paralelismo sem proteção contra `ETXTBSY`, medido em Linux com `-count`
- [x] AC4 — causa da falha exit -1 / stdin vazio medida e escrita
- [x] AC5 — `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat model (hades-tf)
> Dependências: nenhuma. Bloqueia a Wave 1.

### ML-0A — Threat model e enumeração
**Status:** ✅ Done — auditado pelo zeus: tetos = `Makefile:11`, `quality.yml:40`, `release.yml:43`; ETXTBSY: S1 confirmado
(`makeFakeTrackfw`), S2–S4 sem `t.Parallel()` (conferido); `RLock` no escritor correto (fork toma `Lock`, `forkpipe2.go:45`).
Residual R1 (escrita de produto exec'd por teste paralelo) vira item de medição do ML-1B, pela Regra de Causa Raiz
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
- [x] as quatro seções respondidas com evidência (comando e saída), não com afirmação
- [x] nenhuma linha de implementação

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-20-wave0-estabilidade-job-go-threat-model.md
```

## Wave 1 — Implementação (1A ∥ 1B; arquivos disjuntos)
> Dependências: Wave 0 auditada.

### ML-1A — Teto por pacote em ponto único (ares-tf)
**Status:** ✅ Done — auditado: 31 = 30 + 1 pacotes; falsificação nas duas direções (2s reprova, 5s passa, módulo
sintético); nenhum gate pina a linha antiga; `make parity-rest` rc=0. Custo: os dois conjuntos rodam em sequência
**Files affected:** `Makefile`, `.github/workflows/quality.yml` (só o job `go`), `.github/workflows/release.yml`
**Actions:**
1. Alvo `test` do `Makefile`: `internal/generators` com `-timeout 5m`, demais pacotes (`go list ./...` sem ele) com
   `-timeout 2m`, ambos com `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`. Falha de qualquer um reprova o alvo.
2. `quality.yml:40` e `release.yml:43`: trocar o `go test -timeout 2m ./...` por `make test`, mantendo o `env`.
3. Os demais sítios que a Wave 0 marcar como "entra".
**Acceptance criteria:**
- [x] contagem de pacotes: `go list ./... | wc -l` igual à soma dos dois conjuntos do alvo
- [x] falsificação: um pacote não-generators com teste que dorme 150 s reprova em 2m; o mesmo sono no generators passa
  (medido em módulo temporário ou com `-run`, sem commitar o teste)
- [x] `make parity-rest` rc=0 (pins de recipe do `Makefile`)

### ML-1B — `ETXTBSY` nos helpers de teste (artemis-tf)
**Status:** ✅ Done — medido no estresse (ML-1C, 3 rodadas). Código entregue e auditado: `writeExecutableForTest` sob `ForkLock.RLock()` em S1–S4;
nenhum teste removido, pulado ou desparalelizado; R1 medido (risco zero: os testes de produto executam `bash script`).
**Falta:** o braço "antes" não reproduziu (Docker com 1 CPU: 0 falhas em count=50, parallel=64). AC4 corrigido: o
exit -1 vem do caminho PowerShell (`runInteractive`, sinal), não do POSIX; a causa segue sem medição
**Files affected:** os `_test.go` enumerados na Wave 0 (começando por `internal/generators/guard_line_shell_test.go`)
**Actions:**
1. Gravar o executável sem janela de descritor de escrita aberto durante um fork concorrente. Ponto de partida:
   escrita e fechamento sob `syscall.ForkLock.Lock()`, num helper único usado por todos os sítios.
2. Medir em Linux, antes e depois: `go test -count=N -run 'TestGuardLineShellMatrix' ./internal/generators/`, com
   contêiner ou VM. Mostrar o `Text file busy` antes e a ausência depois.
3. Investigar a falha com exit -1 e stdin vazio (121b8ce5) e escrever a causa medida.
**Acceptance criteria:**
- [x] antes/depois em Linux com contagem de falhas: `Text file busy` em 3 de 12 shards do `before` (uma ocorrência em cada
  rodada) e 0 nos `after`
- [x] nenhum teste removido, pulado ou desparalelizado
- [x] Regra de Reconciliação: uma frase por teste novo ou alterado

### ML-1C — Job temporário de estresse no CI para AC3/AC4 (ares-tf; decisão do KG em 2026-11-20)
**Status:** ✅ Done — run 44826888100 (PR #694), `ubuntu-latest` com 4 núcleos, 4 shards por braço:

| braço | `Text file busy` | `obteve -1` | `--- FAIL` |
|---|---|---|---|
| before (`origin/main`) | 1 (shard 3) | 105 (shard 1) | 10 |
| after (HEAD) | 0 | 46 (shard 2) | 5 |

O ETXTBSY reproduziu no `before` e sumiu no `after`, com amostra pequena (1 em 4 contra 0 em 4). O exit -1 aparece
**nos dois braços**, então é outra causa: o `pwsh` aborta na inicialização com `System.IO.FileLoadException: The given
assembly name was invalid`, e os nomes de assembly vêm embaralhados (`'System.Linq, Version=10.0.0.System.Runtime...'`).
A falha restante do `after` (`PSInheritedState ... guard não rodou`) é da mesma família `pwsh`.
> O Docker local tem 1 CPU e não reproduz. A medição vai para o `ubuntu-latest`, onde a falha apareceu.
**Files affected:** `.github/workflows/tmp-etxtbsy-stress.yml` (novo; **removido antes do merge**)
- [x] braço `before` (árvore de `origin/main`) e braço `after` (HEAD), no mesmo tipo de runner, com contagem de
  `Text file busy`, `obteve -1` e FAIL por braço
- [x] o `before` reproduz pelo menos uma falha; se não reproduzir, isso fica escrito e o AC3 não é marcado por ausência
- [x] o arquivo sai da branch antes do merge: removido depois da 3ª rodada

### ML-1D — `pwsh` abortando na inicialização sob paralelismo (artemis-tf) — causa do exit -1 (AC4)
**Status:** ✅ Done (fechado com o ML-1D2)
> Mesmo sintoma, causa medida diferente do ETXTBSY. Pela Regra Dura, fica neste roadmap.
> Hipótese: as instâncias paralelas de `pwsh` compartilham o cache de perfil de inicialização (`StartupProfileData-*` sob
> `$XDG_CACHE_HOME/powershell` ou `~/.cache/powershell`), e uma delas lê o arquivo enquanto outra o grava.
- [x] onde o cache é gravado, pela fonte do PowerShell: `ConsoleHost.cs` → `ProfileOptimization.StartProfile("StartupProfileData-*")`
  em `Platform.CacheDirectory` = `$XDG_CACHE_HOME/powershell` (ou `~/.cache/powershell`), conferido no Mac. Os testes não
  isolavam esse diretório. O Mac (32 pwsh paralelos) não reproduziu a falha; a confirmação vem do estresse no CI
- [x] correção só nos testes: `newPwshCacheEnv` dá um `XDG_CACHE_HOME` próprio aos 5 pontos que lançam pwsh em
  `guard_line_shell_test.go`, sem retry, skip nem perda de paralelismo. Pontos verificados pelo zeus e deixados como estão:
  `fail_closed_d11_behavior_test.go:190` (arquivo sem `t.Parallel()`; os testes paralelos só retomam depois dos
  sequenciais) e `internal/guard` (`pwsh` só como texto). Desvio do executor: escreveu nota no vault fora do escopo
  autorizado; o conteúdo foi mantido
- [x] job de estresse rodado de novo: `obteve -1` = 0 e `Unhandled exception` = 0 em todos os shards do `after` (44833943635 e
  44842568713)

### ML-1D2 — Cache do pwsh aquecido e só para leitura (artemis-tf)
**Status:** ✅ Done — estresse 44842568713 (4dff64a8): os 4 shards do `after` com `Text file busy`, `obteve -1`,
`Unhandled exception`, `já estourou o prazo`, FAIL e panic = 0. O `before` voltou a mostrar 1 `Text file busy`
> Estresse 44833943635 (ef68cce3): no `after`, `obteve -1` = 0 e `Unhandled exception` = 0 nos 4 shards (no `before`,
> 690 no shard 1). Mas o cache vazio por chamada põe todo `pwsh` em partida a frio: aquecimento de 0,12 s para 1,48 s.
> O shard 1 do `after` estourou o prazo de 15 s por chamada (`não executado: /usr/bin/pwsh já estourou o prazo`) e o teto
> de 20 m da carga focada. A correção trocou um flake por outro.
- [x] (feito por cópia: o modelo é aquecido uma vez e cada chamada recebe uma cópia gravável, sem chmod; limpo no
  `TestMain`) um único `pwsh` aquece um diretório de cache por processo de teste; depois o diretório fica sem escrita e todas as
  invocações leem dele (sem escrita concorrente e sem partida a frio), ou alternativa medida melhor
- [x] medido no Mac (10 execuções cada): frio 230 ms, quente+cópia 171 ms, quente só leitura 172 ms; só leitura sai 0,
  sem erro e sem mudar o arquivo; 32 paralelos sem falha, e o contra-braço (cache gravável compartilhado) muda o sha. Itens: tempo de partida frio × quente × quente só para leitura, e que o `pwsh` não falha quando não consegue gravar
  o perfil
- [x] estresse de novo: `obteve -1` = 0, `Unhandled exception` = 0, nenhum `já estourou o prazo` e nenhum `panic` no `after`

## Wave 2 — Revisão (hefesto-tf)
### ML-2A — Revisão de qualidade do diff da Wave 1
**Status:** ✅ Done — APROVA todos os ACs, sem bloqueante. F2 (comentário desatualizado) e F3 (`RLock` na leitura do
mapa) corrigidos (`-race` rc=0). F1 (pipeline sem `pipefail` no `Makefile`) fica registrado: a guarda de vacuidade
cobre a falha total do `go list`, e ele não falha pela metade
**Files affected:** `docs/qualidade/2026-11-20-estabilidade-job-go-review.md`
- [x] parecer com APROVA/REPROVA por AC

## Fechamento
### ML-FB — `make quality` e CI verde
**Status:** ✅ Done
- [x] `make quality` MQ_RC=0 lido pelo zeus no log (`internal/generators` 89,1 s)
- [x] CI verde no PR #694 (24/24 em 32d11c35); o log do job `go` mostra `-timeout 5m ./internal/generators/` e `-timeout 2m $pkgs`
