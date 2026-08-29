---
title: teste de paridade cross-runtime dentro do `go test` quebra o job `go` do CI — e o gate que parecia cobrir não cobria
tags: [ci, paridade, testes, gotcha, barrier]
date: 2026-08-29
related: [[barrier-crlf-divergencia-node-regex-2026-08-29]]
---

## Sintoma

`make quality` verde na máquina do dev, job `go` **vermelho** no CI:

```
--- FAIL: TestBarrierParity_TildeFenceEvasion
    NODE stdout is not valid JSON: unexpected end of JSON input
        stdout:
```

Nove testes, todos `TestBarrierParity_*`.

## Causa Raiz

Os testes faziam **shell-out para `node` e `python3`** de dentro do `go test`, para comparar o
veredito dos 3 runtimes. O job `go` do CI é **Go puro** — não tem Node instalado. Stdout vazio,
JSON inválido, falha.

Na máquina do dev passa, porque a máquina do dev tem os três runtimes. **É a classe de defeito que
só aparece no ambiente mais pobre**, e o ambiente mais pobre é justamente o CI.

Pior: o job `parity`, onde os três runtimes existem, ficou `skipping` em cascata porque depende do
`go` passar. O gate que de fato cobre paridade **nem chegou a rodar**.

## Fronteira correta

**Paridade cross-CLI mora no gate de paridade, não no teste unitário de um runtime.** Um teste que
precisa dos outros dois runtimes dentro do job de um só é dependência que aquele job não tem.

## As duas armadilhas de quem for corrigir

**1. Não resolva com `t.Skip` quando o runtime não existe.** É a correção óbvia e transforma os
testes em nada dentro do CI — some o vermelho e some a verificação junto. Só é seguro **remover**
se o gate cobrir de verdade, e o gate tiver guarda de vacuidade.

**2. Confira se o gate cobre MESMO — não pelo nome do cenário.** O arquiteto afirmou, ao despachar,
que os cenários `fence-phantom-*` do gate já cobriam cross-runtime. **Não cobriam:** chamavam só
`run_cli go`. As únicas coberturas cross-runtime daquele achado eram dois dos nove testes que a
tarefa mandava remover. Remover confiando no nome teria deixado o achado do ML-1B sem nenhum sinal
cross-runtime no CI.

## Armadilha de reprodução, específica do macOS

`env PATH=/usr/bin:/bin go test ./...` **não** reproduz o CI: o macOS expõe um `python3` funcional
em `/usr/bin/python3` (stub das Xcode Command Line Tools). Um PATH ingênuo mantém o Python vivo e
mascara metade do defeito.

Reprodução fiel: PATH contendo **só** um diretório com `git` (necessário para
`roadmapTrustForGates`), `/bin` para `sh`, e o diretório do `go`. Confirme com
`command -v node; command -v python3` vazios **antes** de rodar.

```bash
S=$(mktemp -d); mkdir -p $S/bin; ln -s /usr/bin/git $S/bin/git
env -i HOME=$HOME PATH="$S/bin:/bin:$(dirname $(which go))" go test ./internal/commands/...
```

## Detalhe de implementação que custou tempo

Comparar campos JSON entre runtimes por **string crua** dá falso-negativo: `python3 -c
"json.dumps(...)"` escapa `⬜` como `⬜` enquanto os outros emitem o caractere. A comparação
tem que ser **estrutural** (helper `json_field_equals` no gate), não textual.
