---
title: o ambiente do desenvolvedor é sistematicamente mais rico que o do CI — três defeitos no mesmo dia
tags: [ci, metodo, gotcha, paridade]
date: 2026-08-29
related: [[paridade-cross-runtime-dentro-do-go-test-quebra-o-job-go-2026-08-29]]
---

## O padrão

Em 2026-08-29, **três** defeitos consecutivos do PR #217 tiveram a mesma forma: verde na máquina do
dev, vermelho no CI. Nenhum era o mesmo mecanismo; todos eram a mesma **causa estrutural**.

| ML | o que a máquina do dev tinha e o CI não | como se manifestou |
|---|---|---|
| ML-3F | `node` e `python3` no `PATH` | testes de paridade cross-runtime dentro do `go test` — o job `go` é Go puro. `NODE stdout is not valid JSON` |
| ML-3G | história do git (clone completo) | congelamento do corpus via `git show <sha>` — `actions/checkout` usa `fetch-depth: 1`. `fatal: Not a valid object name` |
| ML-3H | locale `en_US.UTF-8` (macOS) | `sort` é dependente de locale; o runner Linux usa `C`. Hash do pin diferente, `diff` com o mesmo conteúdo em ordem diferente |

## Por que isso se repete

A máquina de quem desenvolve acumula ferramentas, história e configuração. O CI parte do zero a cada
execução — e é **essa pobreza que o torna representativo** de quem clona o repositório pela primeira
vez. Rodar o `make quality` local e ver verde não é evidência de que o CI vai passar; é evidência de
que passa **num ambiente que só existe na sua máquina**.

## A regra prática

Para qualquer verificação que vá virar gate, o critério de aceite não é *"passou aqui"*. É **rodar
no ambiente empobrecido**:

```bash
# sem os outros runtimes no PATH (cuidado: no macOS, /usr/bin/python3 existe
# como stub das Xcode CLT e um PATH ingênuo não o exclui)
S=$(mktemp -d); mkdir -p $S/bin; ln -s /usr/bin/git $S/bin/git
env -i HOME=$HOME PATH="$S/bin:/bin:$(dirname $(which go))" go test ./...

# sem história do git
git clone --depth 1 file://$PWD /tmp/probe

# nos dois locales
LC_ALL=C bash scripts/<gate>.sh
LC_ALL=en_US.UTF-8 bash scripts/<gate>.sh
```

Se os três derem o mesmo resultado, o gate é hermético. Se algum divergir, você achou o defeito antes
do CI.

## A generalização que dói

O mesmo viés produziu, no mesmo dia, um **erro de governança**: o arquiteto moveu o roadmap para
`done/` e fechou a REQ com o `barrier --wave 3` e o `make quality` **locais** verdes — enquanto o PR
estava com o CI vermelho. Dois microlotes corretivos nasceram depois do fechamento, e a `artemis-tf`
recusou o handoff seguinte apontando `wip/` vazio.

*Verde local não é conclusão. A conclusão é o CI verde.* A máquina do dev não é oráculo — nem para
código, nem para processo.

## Detalhe específico de `sort`

`LC_ALL=C sort` **prefixado por invocação**, não `export LC_ALL=C` global: um export vaza para os
subprocessos (os 3 CLIs invocados pelo gate) e pode mascarar regressão de i18n textual. É a
convenção que `check-integration-assets.sh`, `check-static-assets.sh` e `check-identity-parity.sh`
já seguiam — a agente encontrou o precedente em vez de inventar.
