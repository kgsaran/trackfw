# `trackfw ship` tem duas divergências pré-existentes entre CLIs no caminho de erro de governança/padrão de branch — não cobertas por nenhum gate de paridade até esta ML

> 2026-08-16 · descoberto ao escrever `scripts/check-ship-parity.sh` para
> `docs/roadmaps/wip/ROADMAP-2026-08-16-trackfw-ship-aceita-branches-chore-e-docs-sem-gate-de-roadmap.md`

## Sintoma

Um `check-ship-parity.sh` inicial, escrito seguindo o padrão de `check-branch-new-parity.sh`
(diff -u byte-a-byte de stdout **e** stderr entre os 3 runtimes), falhava em dois cenários que
não têm nada a ver com a mudança desta ML (exemplo `chore`/`docs` já passava limpo):

1. Branch `feat/<slug>` sem roadmap em `wip/` → mensagem de violação diverge:
   - Go: `"...no roadmap is in wip/ nor done/ — create governance artifacts first..."`
   - Node/Python: `"...no roadmap is in wip/ — create governance artifacts first..."`
     (sem menção a `done/`, e usa `"<title>"` com os colchetes, enquanto Go usa `"title"` sem)
2. Branch fora do vocabulário (ex.: `hotfix/x`) → o texto da mensagem de erro é
   **byte-idêntico**, mas o **stream** onde ela cai diverge:
   - Go: `stderr`, prefixada com `Error: ` (cobra imprime automaticamente porque `ship.go` nunca
     seta `cmd.SilenceErrors = true`)
   - Node/Python: `stdout`, prefixada com `error: ` (escrita explícita via `writeln`)

## Causa raiz

**(1)** `internal/validator/validateBranchHasWIPRoadmap` (Go) e `checkShipGovernance`/
`check_ship_governance` (Node/Python) são **implementações duplicadas e independentes** da mesma
regra — não há uma fonte única. Foram escritas em momentos diferentes e divergiram no texto
exato da mensagem sem que nenhum gate detectasse, porque **nenhum script de paridade cobria
`trackfw ship` além do floor de `check-cli-parity.sh`** (que só verifica que o comando existe nos
3 runtimes, não o comportamento).

**(2)** `internal/commands/branch.go` e `internal/commands/commit.go` setam
`cmd.SilenceErrors = true` no `RunE` — por isso escrevem sua própria mensagem de erro via
`deps.out` e cobra nunca imprime nada por cima. `internal/commands/ship.go` seta apenas
`cmd.SilenceUsage = true`, **não** `SilenceErrors` — então toda vez que `RunE` retorna um erro,
cobra imprime `Error: <mensagem>` no stderr do comando **além** da mensagem que o próprio `ship`
já escreveu (quando escreve). Nos casos onde `ship` só faz `return fmt.Errorf(...)` sem escrever
nada em `deps.out` antes (ex.: branch fora do padrão), o único texto que sai é esse `Error: ...`
do cobra — no stderr. Node/Python nunca tiveram esse comportamento de framework: sempre escrevem
a mensagem explicitamente via `writeln` no stdout.

## Por que passou despercebido

Os testes unitários de `ship` (Go/Node/Python) sempre verificam apenas **substring** dentro da
mensagem de erro/saída combinada — nunca comparam byte-a-byte entre os 3 CLIs, nem verificam em
qual stream a mensagem cai. Isso é suficiente para garantir que cada CLI individualmente se
comporta como o esperado, mas não pega divergência **entre** runtimes.

## Impacto

Nenhum impacto funcional imediato — a mensagem chega ao usuário em ambos os casos, só que por
streams diferentes (o que pode quebrar scripts externos que capturam apenas stdout ou apenas
stderr do `ship`) e com texto ligeiramente diferente na violação de `branch_has_wip_roadmap`.

## Contorno usado nesta ML

`scripts/check-ship-parity.sh` usa `diff -u` completo (via `assert_three_way`, herdado de
`check-branch-new-parity.sh`) **apenas** nos dois cenários novos desta ML (chore/docs branch-type
skip — `Governance: skipped (chore/docs branch)`), onde não há essa divergência pré-existente.
Para os cenários que tocam o caminho antigo (`feat` sem roadmap; branch fora do vocabulário), o
script usa `assert_exit_equal` + `grep` em conteúdo específico (função nova, sem diff completo de
stream) — prova o que **esta ML** mudou sem falhar por causa de um gap pré-existente e fora de
escopo.

## Correção sugerida (vira REQ própria — fora do escopo desta ML)

1. Unificar a mensagem de violação de `branch_has_wip_roadmap` entre os 3 runtimes (escolher uma
   fonte de verdade textual, replicar byte-a-byte — mesmo padrão que `validator.
   BranchGovernanceOrientation` já resolve para `trackfw branch new`/`trackfw commit`).
2. Decidir se `ship.go` deve setar `cmd.SilenceErrors = true` como `branch.go`/`commit.go` (stdout
   uniforme) ou se Node/Python devem passar a escrever no stderr para casar com o cobra — e então
   escrever um `check-ship-parity.sh` mais amplo cobrindo `assert_three_way` para todos os
   cenários, não só os dois novos.

## Links

- `internal/commands/ship.go`, `npm/src/ship/runner.js`, `pypi/trackfw/ship/runner.py`
- `internal/validator/validateBranchHasWIPRoadmap` (`internal/validator/validator.go`)
- `scripts/check-ship-parity.sh` (cenários `feat-still-gated-non-regression` e
  `invalid-branch-vocabulary`)
- Ver também [[branch-new-exit-code-leak-vs-propagation-2026-08-04]] para o padrão correto de
  `SilenceErrors` + propagação literal de exit code.

## Resolução (ML-1B do ROADMAP-2026-08-16-higiene-sete-debitos-acumulados..., 2026-08-16)

Os dois achados descritos acima foram corrigidos. **Nenhuma das duas correções sugeridas foi
aplicada como estava escrita** — a investigação mais funda (ver `advisor()` na sessão da ML)
mostrou que a causa raiz do achado 1 era duplicação de lógica, não só de texto, e que o achado 2
já descrevia o comportamento canônico do Go (a decisão do arquiteto foi adotar exatamente o que o
Go já fazia).

**Achado 1 — wording.** `checkShipGovernance`/`check_ship_governance` em Node e Python
reimplementavam a regra `branch_has_wip_roadmap` do zero, com texto próprio e **sem escanear
`done/` nenhuma vez** — não era só uma questão de wording, o comportamento também divergia (uma
branch com roadmap só em `done/` passaria no Go e falharia no Node/Python). A correção foi
**eliminar a duplicação**: as duas funções agora delegam para `validator.validateBranchHasWIPRoadmap`
+ `validator.validateWIPHasREQ` (Node: `npm/src/validator/index.js`; Python:
`pypi/trackfw/validator.py`) — as mesmas funções que `trackfw branch new`/`trackfw commit` já
usavam e que já eram byte-idênticas ao Go (`internal/validator/validator.go`
`BranchGovernanceOrientation`/`BranchNoMatchingRoadmapMessage`). `checkShipGovernance()` em Node
e `check_ship_governance()` em Python perderam toda a lógica de leitura de diretório/normalização
de slug — hoje têm o mesmo formato "sem argumentos" que `validator.CheckShipGovernance()` no Go.

**Achado 2 — stream/prefixo.** A decisão do arquiteto foi adotar o comportamento do Go como
canônico: erro no **stderr**, prefixo `Error: `. Comparando com `internal/commands/root.go`
(`Execute()`, linhas ~89-111), esse já era o comportamento real do Go **sem nenhuma mudança
necessária** — `ship.go` não precisou setar `SilenceErrors`; o root wrapper já imprime
`Error: <msg>` no stderr para qualquer `RunE` que não silencie erros (comportamento diferente de
`branch.go`/`commit.go`, que silenciam de propósito para imprimir sem prefixo). A correção ficou
inteiramente do lado de Node/Python: `runShip`/`run_ship` ganharam um parâmetro `writeErr`/
`write_err` (default: escreve `Error: <msg>\n` em stderr) usado só para a linha-resumo final de
cada caminho de aborto; todo o detalhe anterior (lista de violações, dicas de remediação,
bloco "Note: ...") continua no stdout via `writeln`, exatamente como o Go já fazia via `deps.out`.

**Prova**: `scripts/check-ship-parity.sh` — os cenários `feat-still-gated-non-regression` e
`invalid-branch-vocabulary` passaram a usar `assert_three_way` (diff -u completo de stdout **e**
stderr) em vez dos helpers `assert_exit_equal`/`assert_message_byte_identical`, que foram
removidos do script junto com a normalização que descartava a divergência de prefixo. Os quatro
cenários passam com saída byte-idêntica nos três runtimes.
