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

**Repetido em 2026-11-02 (#655):** o `TestQuickStartEndToEnd` roda `init` e depois `validate` de verdade.
Passou no `make quality` local e reprovou no job `go`. O gatilho não é só "fixture com linha de guard": é
**qualquer teste que chame `validate` (ou `Validate`) num projeto criado por `init`**, porque o `init` gera
os hooks e a sonda passa a rodar. Exija o PATH sem trackfw no handoff de qualquer teste de ponta a ponta,
e rode você mesmo, antes do push, os pacotes tocados com `env PATH="$(dirname $(command -v go)):/usr/bin:/bin"`.

🔴 **O PATH restrito tem efeito colateral (2026-11-05):** `/usr/bin:/bin` troca o bash 5 do Homebrew pelo
**bash 3.2** do macOS. Script com `declare -A` (`check-serve-address-parity.sh`) reprova, e o executor
chamou de "pré-existente". Use o PATH restrito **só no `go test`**; o `make parity-rest`/`make quality`
roda com o PATH normal. Ou monte `$(dirname $(command -v go)):$(dirname $(command -v bash)):/usr/bin:/bin`
se o `trackfw` não estiver no mesmo diretório do bash (aqui os dois estão em `/opt/homebrew/bin`).
