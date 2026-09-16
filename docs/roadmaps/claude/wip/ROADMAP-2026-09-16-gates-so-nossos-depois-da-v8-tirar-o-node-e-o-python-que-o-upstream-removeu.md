---
status: wip
date: 2026-09-16
req: "docs/requisições/claude/REQ-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md"
squad: "claude"
---

# Roadmap: gates só nossos depois da v8: tirar o Node e o Python que o upstream removeu

> Created: 2026-09-16 | Status: wip

## Context

REQ: docs/requisições/claude/REQ-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md

O sync da v8 (`f979c01`) removeu `npm/src` e `pypi/trackfw`. Três gates só nossos reprovam, dois
carregam baseline de arquivo que não existe mais, o workflow do fork instala runtimes que nenhum gate
usa, e o `CLAUDE.md` afirma fatos sobre eles.

## Acceptance Criteria

- [ ] AC1 — agregador 0 falhas sem Node/Python; pré-condição do `bin/trackfw` falsificada
- [ ] AC2 — `check-subcommand-parity.sh` retirado com motivo
- [ ] AC3 — `check-slug-inventory.sh` Go-only, falsificado nos dois sentidos
- [ ] AC4 — instrumentos de predicado de SO com escopo `internal cmd`, 0 obsoleta, baseline conferido
- [ ] AC5 — `local-gates.yml` sem runtimes removidos, verde no CI
- [ ] AC6 — `CLAUDE.md` sem afirmação falsa sobre os runtimes removidos
- [ ] AC7 — `validate` 0; CI por nome contra o upstream em `66b7ad8`
- [ ] AC8 — `docs/cli-parity.md` trazido e mantido pelo sync, com prova e falsificação

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** nenhum (medição)
**Actions:**

1. **Enumeração.** `grep` por `npm/src`, `pypi/trackfw`, `npm/bin/trackfw`, `python3 -m trackfw`,
   `pypi/tests` e `npm/tests` nos scripts só nossos e no `local-gates.yml`: **7 arquivos** —
   `run-local-gates.sh`, `check-subcommand-parity.sh`, `check-slug-inventory.sh`,
   `check-upstream-content.sh` (só comentário), `check-os-predicate-classification.sh`,
   `measure-os-predicate-sites.sh`, `local-gates.yml`. Cada gate rodado isolado na árvore do merge;
   resultado na tabela da REQ.
2. **Quem esvazia isto sem quebrar regra escrita.** Três atalhos, cada um recusado:
   - apagar a pré-condição inteira do agregador — perde a falha nomeada do `bin/trackfw`;
   - regravar o baseline do `measure` sem ler os nomes — é o que o próprio script proíbe;
   - reduzir o inventário de slug a `exit 0` — gate que não pode reprovar.
3. **Falsificação nas duas direções**, por ML: pré-condição sem `bin/trackfw` reprova nomeando;
   slug novo e slug sumido reprovam; sítio de classificação novo em `internal/` reprova.
4. **Residual.** A [#366](https://github.com/kgsaran/trackfw/issues/366) segue aberta: rodar
   `parity-rest` na raiz reescreve o `trackfw.yaml` do fork. Nenhum gate nosso o chama.

**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
bash scripts/run-local-gates.sh
```

## Wave 1 — Gates e workflow

### ML-1A — agregador, subcommand-parity e slug-inventory
**Status:** ✅ Concluído
**Files affected:** `scripts/run-local-gates.sh`, `scripts/check-subcommand-parity.sh`, `scripts/check-slug-inventory.sh`
**Acceptance criteria:**
- [x] AC1, AC2 e AC3 medidos, com falsificação

**Evidência — 2026-09-16.**

| caso | resultado |
|---|---|
| agregador, árvore real | `9 executado(s) · 0 falha(s)` (eram 10; saiu o `check-subcommand-parity`) |
| agregador, worktree **sem `bin/`** | rc=1, `FALHA — pre-requisito(s) indisponivel(is): bin/trackfw` |
| agregador, worktree com o gate retirado ainda no disco | rc=1, `COMPLETUDE: 'scripts/check-subcommand-parity.sh' e so nosso e nao esta nem em EXECUTAR nem em FORA` |
| slug, árvore real | rc=0, `1 implementacao declarada` |
| slug, `func toSlug` novo em `internal/generators/` | rc=1, diff nomeia `zz_falsif_slug.go:toSlug` |
| slug, `toSlug` do `adr.go` renomeado para `slugDoADR` | rc=1, diff nomeia `adr.go:toSlug` como sumido |

🔴 **Duas falsificações saíram erradas na primeira tentativa, e as duas pelo instrumento:**

- **Slug "sumido" deu rc=0.** Eu tinha renomeado para `toSlugX`, e `^func toSlug` casa `toSlugX` por
  prefixo. Refeito com um nome que não compartilha prefixo.
- **Agregador "sem binário" rodou os gates.** Movi `bin/trackfw` para fora, mas no Git Bash
  `[ -x bin/trackfw ]` resolve para `bin/trackfw.exe`, que ficou. Pior: o `mv` de volta **sobrescreveu
  o `.exe` e apagou o sem-extensão** — `dir` do cmd mostrou só `trackfw.exe`. Os dois tinham o mesmo
  build (19.967.488 bytes), recompilados em seguida. A falsificação válida foi num worktree sem `bin/`.

Achado lateral, não corrigido aqui: o `|| true` no `grep` do slug existe porque, sem ele, a
implementação sumida faria o `pipefail` matar o script dentro da atribuição, reprovando **sem dizer o
que sumiu**. O gate antigo tinha a mesma forma, mascarada por três `grep` em sequência.

**Frase por teste (Regra de Reconciliação):** a falsificação do worktree afirma que a pré-condição
continua nomeando o binário ausente depois de perder Node e Python; as duas do slug afirmam que o gate
reduzido ao Go ainda reprova nos dois sentidos.

### ML-1B — instrumentos de predicado de SO
**Status:** ✅ Concluído
**Files affected:** `scripts/check-os-predicate-classification.sh`, `scripts/measure-os-predicate-sites.sh`, `scripts/testdata/os-predicate-sites-baseline.txt`
**Acceptance criteria:**
- [x] AC4 medido: nomes que saem conferidos contra arquivos removidos; guardas de pé

**Evidência — 2026-09-16.**

**Lint.** Escopo `internal cmd`, 7 declarações removidas do `BASELINE` (os espelhos Node e Python e as
duas leituras inline de plataforma), todas de arquivo que a v8 apagou. Depois:
`204 sitios · 116 com teste · 57 D1 · 2 D3 · 25 comentario · 4 D2 em 2 arquivos declarados`, **zero**
aviso de obsolescência. Os cinco somam 204.

| falsificação | resultado |
|---|---|
| `runtime.GOOS == "windows"` plantado e **rastreado** em `internal/pathanchor/` | rc=1, pede resolvedor canônico ou baseline com motivo |
| escopo `cmd` (nenhum predicado) | rc=1, guarda de vacuidade: `zero sitios varridos` |
| escopo `internal/pathanchor` (12 predicados, nenhum `os.IsNotExist`) | rc=1, guarda **D1**: `zero sitios classificados como D1` |

**Measure — o baseline não foi regravado às cegas.** Contra o baseline de 11/09: `29 novos · 65
sumidos`. Por topo: 40 dos sumidos são `npm/src` (19) e `pypi/trackfw` (21), apagados pela v8. O resto
foi reconciliado por `arquivo:predicado`, contando ocorrências antes e agora — só **cinco** pares mudaram
de contagem, e **nenhum pela v8**: todos já estavam iguais em `a873897`, a `main` antes do sync.

| arquivo:predicado | baseline → agora | origem |
|---|---|---|
| `internal/config/config_agents_register.go:os.IsNotExist` | 0 → 1 | #330, 12/09 — recebe `err`, é D1 |
| `internal/serve/api_file.go:os.IsNotExist` | 1 → 0 | #332, 12/09 |
| `internal/{discover,serve}/symlink_helper_test.go:runtime.GOOS` | 0 → 1 cada | arquivo de teste (D5) |
| `internal/validator/symlink_helper_test.go:runtime.GOOS` | 0 → 2 | arquivo de teste (D5) |

Os demais nomes de `internal/` mudaram só de **número de linha**, com a contagem por arquivo e
predicado idêntica. Regravado: `204 sítio(s)`, e a comparação seguinte dá `novos 0 · sumidos 0`.

🔴 **Achado lateral:** o baseline do `measure` estava defasado desde 12/09. O script **mede e não
reprova** — sai `rc=0` com "a superfície MUDOU" —, então o agregador o mostrava `ok` a cada push.
É a forma declarada dele, não defeito novo; fica registrado porque foi a v8 que obrigou a olhar.

**Frase por teste:** o sítio plantado afirma que o lint com escopo reduzido ainda reprova
classificação nova em `internal/`; as duas guardas afirmam que o escopo menor não virou verde vazio.

### ML-1C — workflow do fork
**Status:** 🔄 Em andamento
**Files affected:** `.github/workflows/local-gates.yml`
**Acceptance criteria:**
- [ ] AC5 verde no CI

## Wave 2 — Documentação

### ML-2A — CLAUDE.md
**Status:** ✅ Concluído
**Files affected:** `CLAUDE.md`, `scripts/check-upstream-content.sh` (comentário)
**Acceptance criteria:**
- [x] AC6: cada afirmação caducada corrigida com o fato que mudou

**Evidência — 2026-09-16.** Enumeração por `grep` no `CLAUDE.md` por `npm/src`, `pypi/`, `node`,
`python (3`, `três runtimes`, `3 runtimes`, `tri-runtime`. Cada ocorrência lida e classificada:

| seção | veredito | fato que mudou |
|---|---|---|
| Gate de layout de REQ — `docs/req` como fixture | **atualizada** | fixture segue viva só no Go (`validator_test.go`, conferido por `git grep`) |
| Branch `feat/fix/refactor` custa 3 vermelhos | **caducou**, marcada no topo | jobs `python` removidos; `validator.py` apagado; #261 fechada |
| Ponto cego dos 3 gates de PATH curado | **atualizada** | 2 dos 3 gates removidos; o de `release-tag` segue com `ln -s python3`, **não re-medido** |
| Lint de predicado de SO — números | **atualizada** | 240 → 204, 16 D2 em 9 arquivos → 4 em 2 |
| Ratchet de Windows — três suítes | **atualizada** | `entries` com 14 nomes, todos `go` |
| Divergência local — lista de scripts só nossos | **atualizada** | `check-subcommand-parity` retirado |
| `req new --agent` "nos três runtimes" | mantida | afirmação datada de 12/09, verdadeira na data |
| Gate de REQ herdada — "`req list` dos três runtimes" | mantida | idem, histórica |
| **nova:** não rodar `parity-rest` na raiz | acrescentada | #366 aberta; `check-tty-detection.sh` ainda roda `init` no cwd |

Nada foi apagado: toda seção caducada ou desatualizada ganhou um bloco datado com o fato. Os dois
comentários do `check-upstream-content.sh` que citavam `npm/src` e `pypi/tests` como vivos também.

## Wave 3 — O contrato de produto retido em docs/

> Por que o escopo original não previa: o ML-0A enumerou só os sítios que **citam** os runtimes
> removidos. O `docs/cli-parity.md` não os cita por caminho; ele cita **gates** que a v8 apagou, e o
> leitor é um gate do upstream que o nosso agregador não roda. Apareceu no CI da PR #135.

### ML-3A — upstream-sync traz docs/cli-parity.md
**Status:** ✅ Concluído
**Files affected:** `scripts/upstream-sync.sh`, `scripts/check-upstream-sync-falsify.sh`, `docs/cli-parity.md`, `docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md`, `CLAUDE.md`
**Acceptance criteria:**
- [x] AC8 medido

**Evidência — 2026-09-16.**

**O defeito, no CI da PR (run `35138855115`).** `parity-other-gates` parou em `Makefile:35` com
`FAIL: ## Version output (linha 36): gate nomeado não existe no disco: scripts/check-cli-parity.sh` e
dezenas iguais; no run anterior da `main` ele parava no barrier. O `windows-defect-reproduction` ficou
de `19:10:14` a `19:29:51` sem uma linha de saída e foi cancelado com o depurador parado em
`run.ps1:148`, `$stderr = $p.StandardError.ReadToEnd()` — leitura de stdout antes de stderr, que
trava quando a saída de erro enche o buffer. No upstream, com o contrato em dia, o mesmo job passou em
53 s.

**Medição do alcance.** Arquivos de `docs/` citados por caminho em `scripts`, `Makefile`, `.github`,
`internal` e `cmd` do upstream, e que diferem do nosso: só `docs/cli-parity.md` (337 citações) e
`docs/agents-working-context.md`, que é nosso handoff e fica retido. O resto da lista são ADRs e REQs
dele que não importamos, por decisão.

**Mudança.** `PRODUTO_EM_DOCS="docs/cli-parity.md"` no sync: depois de restaurar `docs/` da base, o
arquivo volta do REF; a prova de retenção exclui a entrada, e uma segunda prova exige que ela seja
igual ao REF. A falsificação lê a lista do próprio sync e ganhou a invariante 4.

| caso | resultado |
|---|---|
| falsificação, sync novo | OK — caso 2 (`01086b5...6b3ba49`) toca o arquivo: `produto em docs/ identico ao REF (o merge tocou 1 dele)` |
| **controle:** sync sem a exceção, mesma falsificação | rc=1 — `produto NAO trazido: docs/cli-parity.md` e `produto em docs/ difere do REF` |
| `check-parity-contract-coverage.sh` com o arquivo trazido | rc=0, `nenhuma anotação inválida e nenhuma seção sem anotação`; `trackfw.yaml` intacto |
| blob `docs/cli-parity.md` | `a8aaa06708` = `upstream/main` |

**Frase por teste:** a invariante 4 afirma que o sync deixa o produto em `docs/` igual ao REF; o
controle afirma que ela reprova quando a exceção some.

**Achado do upstream, relatado à parte:** o travamento do `run.ps1:148` existe independente de nós —
qualquer item cuja saída de erro passe do buffer trava o job até o timeout e aparece como
`cancelled`, não como falha.
