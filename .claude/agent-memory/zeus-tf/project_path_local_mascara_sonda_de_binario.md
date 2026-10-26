---
name: path-local-mascara-sonda-de-binario
description: make quality e go test locais passam com teste que depende do trackfw instalado no PATH; o CI não tem o binário e reprova (#623, TestValidate_Clean)
metadata:
  type: project
---

Fixture com a linha `trackfw guard …` aciona a sonda de binário do validador (`guardBinaryProbeOnce`), que exige
`trackfw` no PATH. Nesta máquina o `trackfw` instalado (`/opt/homebrew/bin`) está no PATH do `go test` e do
`make quality` (o `PATH=$S/venv/bin:$PATH` herda o resto). Teste verde local, vermelho no CI (`go` e
`windows-full-suites`) — medido em 2026-10-25 no #623.

**Why:** o HOME isolado não isola o PATH, e a sonda é real (LookPath + `trackfw guard --help`).

**How to apply:** handoff que mexe em fixture com linha de guard → exigir também
`env PATH=<PATH sem trackfw> go test <pacote>` (comando no handoff do corretivo do #623). Teste usa as costuras
`guardLookupBinary`/`guardRunProbe` com `t.Cleanup`. Relacionado: [[o-instrumento-mente]], [[medir-com-a-regra-nao-com-grep]].
