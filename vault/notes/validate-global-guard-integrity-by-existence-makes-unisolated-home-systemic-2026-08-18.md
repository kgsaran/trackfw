# `git_branch_guard_script_integrity`/`credential_guard_script_integrity` triggering on artifact EXISTENCE (not config wiring) made every un-isolated `$HOME` in tests/gates a systemic failure source — 2026-08-18

## Contexto

ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao,
ML-3A. Antes desta ML, `validateGuardGlobalScriptIntegrity` (Go: `internal/validator/validator_git_branch_guard.go`;
Node: `npm/src/validator/index.js`; Python: `pypi/trackfw/validator.py`) só avaliava
`~/.trackfw/scripts/<guard>.sh` quando um dos 6 configs globais (`~/.claude/settings.json` etc.)
**referenciava** o script. Sem fiação, a regra nunca rodava — foi assim que o script real de KG
ficou 3 versões atrasado com `validate` verde (motivação da REQ).

O fix trocou o gatilho para "o arquivo existe em `~/.trackfw/scripts/`", avaliado uma única vez por
script, independente de fiação.

## Causa raiz do efeito colateral

Essa mudança tornou `Validate()`/`ValidateTagged()` (e portanto **qualquer** teste ou gate que
chame `trackfw validate` / `discover --init` / rode a suíte de testes dos 3 stacks) sensível ao
`$HOME` **real** de quem executa, mesmo em cenários que não têm nada a ver com guards. Antes, só
gates que especificamente exercitavam fiação global (poucos, já isolados por precedente — ver
`check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`) corriam esse risco. Depois
desta ML, **qualquer** `trackfw validate` sem `$HOME` isolado passou a arriscar pegar o estado real
do harness global instalado na máquina.

Na máquina de KG, o `~/.trackfw/scripts/trackfw-git-branch-guard.sh` real estava genuinamente
desatualizado (o próprio bug que motivou a REQ) — então isso não é hipotético, quebrou de verdade:

- Go: `TestValidate_Clean` (`internal/validator/validator_test.go`) — 1 warning inesperado.
- Python: `test_json_sem_violations_summary_counts`, `test_sem_violations_projeto_vazio`.
- Shell: `scripts/check-artifact-parity.sh` (`assert_quoted_status_validate`, que exige
  `violations==0 AND warnings==0`), `scripts/check-barrier.sh` (evidência textual do gate
  `validate` embutido em `trackfw barrier` divergindo entre Go e Node — Go pegou o warning real,
  Node ainda não tinha a mesma regra implementada no momento do teste, expondo também um gap de
  paridade temporário), e várias dezenas de cenários em `scripts/check-gates-falsify.sh` que usam
  `scaffold_adr_req_project` + `bin/trackfw validate` esperando saída limpa pinada
  (`✓ No violations found.`).
- Node não quebrou sozinho (`npm test` passou 651/651) — não porque estivesse isolado, mas porque
  nenhum teste Node fazia asserção estrita de zero warnings sobre `validateUnfiltered()` sem
  controlar `$HOME`. Não presumir que "os testes passam" significa "está isolado" — pode só
  significar que ninguém escreveu a asserção que expõe o vazamento ainda.

## Fix

Isolamento de `$HOME` movido de "por-teste, quando alguém lembra" para "por-runner/gate, uma vez,
por padrão":

- Go: `internal/validator/main_test.go` — `TestMain` seta `$HOME` para um dir temporário antes de
  `m.Run()`, cobrindo TODOS os testes do pacote `validator` (interno e externo, mesmo binário).
  Testes que já isolavam `$HOME` pontualmente via `t.Setenv` continuam funcionando sem alteração —
  eles salvam/restauram em torno do valor do `TestMain`, não do real.
- Python: `pypi/tests/conftest.py` (novo) — fixture `scope="session", autouse=True` fazendo o
  mesmo para a suíte inteira.
- Shell: `scripts/check-artifact-parity.sh`, `scripts/check-barrier.sh` — `export HOME="$WORK/home"`
  logo após criar `$WORK`. `scripts/check-gates-falsify.sh` — mesmo padrão, mas com
  `GOPATH`/`GOCACHE`/`GOMODCACHE` fixados nos valores REAIS **antes** de isolar `$HOME` (ver
  armadilha abaixo).

## Armadilha: isolar `$HOME` quebra `go build` se GOPATH/GOCACHE não forem fixados antes

`scripts/check-gates-falsify.sh` compila binários Go isolados em vários cenários (25+). `go build`
resolve `GOPATH`/`GOCACHE`/`GOMODCACHE` a partir de `$HOME` quando essas variáveis não estão
setadas explicitamente. Isolar só `$HOME` (sem fixar as três) faz cada execução do gate:

1. Redownloadar o módulo inteiro num cache novo (lento — a run subiu de segundos para minutos na
   primeira tentativa).
2. Falhar no cleanup (`trap rm -rf "$WORK"` do `EXIT`) com `Permission denied` em vários arquivos —
   o cache de módulos do Go grava artefatos **read-only** por design, e `rm -rf` genérico não lida
   com isso sem `chmod` prévio.

Fix: capturar `GOPATH`/`GOCACHE`/`GOMODCACHE` reais via `go env` **antes** da linha que troca
`$HOME`, e exportá-los explicitamente. `go build` então usa o cache real (rápido, sem
redownload) mesmo com `$HOME` sintético.

## Por que importa para MLs futuras

- Qualquer regra de `validate` que passe a depender de **existência de arquivo em `$HOME`** (em vez
  de conteúdo de config referenciando algo) muda a classe de risco: deixa de ser "só os gates que
  testam fiação global precisam isolar `$HOME`" e passa a ser "qualquer coisa que rode `validate`
  precisa isolar `$HOME`". Ao adicionar uma regra desse tipo, rodar a suíte completa
  (`go test ./...`, `npm test`, `pytest`, `make quality`) **antes** de declarar a ML pronta — um
  `$HOME` "limpo" de CI pode não expor o problema; a máquina real de um desenvolvedor com o harness
  instalado expõe.
- Sintoma de reconhecimento: uma mudança em `internal/validator/*.go` que só toca uma regra de
  guard, mas quebra testes/gates **sem relação nenhuma** com guards (ex: `TestValidate_Clean`,
  `check-artifact-parity.sh`) — é o padrão clássico de vazamento de `$HOME` não-isolado, não uma
  regressão lógica na regra em si. Ver também
  `check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md` (mesma família de bug,
  instância anterior, escopo mais restrito).
- Se isolar `$HOME` num script que compila Go, sempre fixar `GOPATH`/`GOCACHE`/`GOMODCACHE`
  primeiro — do contrário o sintoma parece "gate ficou lento e não limpa `/tmp`", que não aponta
  obviamente para a causa.

## Addendum 2026-08-19 (ML-3B, ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release)

Instância remanescente da mesma família, achada depois que o guard global foi de fato cabeado
nesta máquina via `trackfw update harness`: dois testes de
`npm/tests/git_branch_guard.test.js` (`injectCodexHooks`, `injectCopilotHooks`) liam `$HOME` real
sem usar o helper `withIsolatedHome` já existente no mesmo arquivo (usado pelos 3 testes vizinhos
`injectClaudeHooks`/`injectGeminiHooks`/`injectCursorHooks`). Sintoma: verde no CI, vermelho na
máquina com o harness instalado — mesmo padrão descrito acima, mas em `npm test`, que a nota
original registrou como "não quebrou sozinho... não porque estivesse isolado". Fix: envolver os
dois blocos de teste em `withIsolatedHome`. Go (`t.Setenv` em todas as 17 funções de teste
relevantes) e Python (`setUp`/`_isolated_home`) já isolavam corretamente — só o npm tinha o gap.

## Referências

- `internal/validator/validator_git_branch_guard.go` (`validateGuardGlobalScriptIntegrity`)
- `internal/validator/main_test.go` (novo)
- `pypi/tests/conftest.py` (novo)
- `scripts/check-artifact-parity.sh`, `scripts/check-barrier.sh`, `scripts/check-gates-falsify.sh`
  (isolamento de `$HOME`, `GOPATH`/`GOCACHE`/`GOMODCACHE`)
- `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md` — mesma
  família de bug, escopo mais restrito (só gates que exercitavam fiação global)
- `docs/roadmaps/wip/ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`
  (ML-3A)
