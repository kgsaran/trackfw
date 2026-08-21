# Windsurf/Amazon Q não têm harness-scope — check-harness-hooks-parity.sh nunca poderá cobri-los

**Contexto:** ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md, ML-1B.

## O achado

`docs/cli-parity.md` afirmava (seção "Caminhos confirmados — Windsurf e Amazon Q") que a correção de
caminho/schema do wiring desses dois CLIs tinha "byte-identidade confirmada via
`check-agent-hooks-parity.sh` **e** `check-harness-hooks-parity.sh`". A segunda metade era **falsa
de forma estrutural, não só não-verificada**: não é que faltasse rodar o gate, é que o gate **não
tem como** verificar essa alegação, nunca teve.

## Por quê

`check-harness-hooks-parity.sh` compara arquivos de **global-scope** (`~/.claude/settings.json`,
`~/.codex/hooks.json`, etc.), escritos por `trackfw update harness --targets <tool>-credential-guard,
<tool>-git-branch-guard`. Os dois caminhos corrigidos para Windsurf/Amazon Q
(`.windsurf/hooks.json`, `.amazonq/cli-agents/q_cli_default.json`) são de **project-scope**, escritos
por `discover --init` — arquivo, gate e escopo totalmente diferentes.

Mais fundo: nem existe um alvo de harness para esses dois CLIs em nenhum dos 3 stacks.
`internal/generators/update.go`, `buildHarnessTargetIDs()` (e os espelhos `npm/src/commands/
update-harness.js`, `pypi/trackfw/commands/update_harness.py`) só inserem o par
`<tool>-credential-guard`/`<tool>-git-branch-guard` para `claude`/`codex`/`gemini`/`cursor`/
`copilot`/`kiro` — nunca para `windsurf`/`amazonq`, que só ganham `<tool>-agents`/`<tool>-skills`.
O comentário do próprio código documenta a decisão: "Windsurf has no native hook mechanism and stays
out per the ADR" (`harnessCatalogTargetOrder`, `internal/generators/update.go`); Amazon Q
simplesmente nunca recebeu esse par.

## Consequência prática

Se um agente futuro ler essa alegação (ou uma REQ de auditoria a citar) e tentar "fechar a lacuna"
estendendo `CLIS` em `check-harness-hooks-parity.sh` para incluir `windsurf`/`amazonq`, vai bater
numa parede: não existe `~/.windsurf/...`/`~/.amazonq/...` gerado por `update harness` para
comparar, porque `update harness` nunca escreve nada lá para esses dois. Estender o gate exigiria
primeiro inventar um harness target novo — **mudança de produto**, fora do escopo de "fechar
cobertura de gate".

## O que foi corrigido

- `docs/cli-parity.md`: a menção falsa a `check-harness-hooks-parity.sh` removida da seção
  "Caminhos confirmados"; a anotação `trackfw-contract` da seção-mãe ("Git branch guard por
  runtime") passou a nomear explicitamente os dois gates com escopos distintos (8 CLIs project-scope
  via `check-agent-hooks-parity.sh`, 6 CLIs global-scope via `check-harness-hooks-parity.sh`) e por
  quê os outros 2 ficam fora por design.
- `scripts/check-harness-hooks-parity.sh`: header ganhou parágrafo explícito explicando a exclusão
  estrutural, para que o próximo agente não precise rederivar isso do zero lendo `update.go`.

## O que o ML-1B efetivamente cobriu

`check-agent-hooks-parity.sh` (project-scope, o gate certo para esses dois caminhos) foi estendido
para os 8 CLIs — isso sim era uma lacuna real de gate, fechada com `CLIS`/`marker_for`/`hookfile_for`
+ Cenário 78 de `check-gates-falsify.sh` (baseline+detecção). Ver
[[credential-guard-hook-resolvable-nao-detecta-script-ausente-2026-08-15]] para o histórico da
mesma família de gates.
