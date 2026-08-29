---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md"
squad: "apolo-tf"
---

# Roadmap: discover --init nao gera os scripts de attention hooks em Go e Node (quebra de paridade com Python)

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-discover-init-nao-gera-os-scripts-de-attention-hooks-em-go-e-node-quebra-de-paridade-com-python.md`

`trackfw discover --init` injeta hooks em `.claude/settings.json`/`.gemini/settings.json` apontando
para `scripts/trackfw-attention-signal.sh`/`cleanup.sh`, mas só o Python realmente gera esses dois
arquivos nesse fluxo (`pypi/trackfw/commands/discover.py:497-500`). Go e Node.js pulam essa chamada —
bug de paridade, não decisão de design. O fix é puramente "chamar a função que já existe, no lugar
certo, na ordem certa" nos dois runtimes que faltam; a função de geração em si já existe nos três
(usada por `trackfw init`).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Go e Node.js chamam a geração dos scripts de attention antes de injetar os hooks em
      `discover --init`, na mesma ordem do Python
- [x] Idempotência preservada (rodar `discover --init` duas vezes não corrompe/duplica)
- [x] Teste de regressão nos 3 runtimes garantindo que os dois scripts existem no disco após
      `discover --init`
- [x] `make quality` verde (99/99 cenários de falsificação passaram)

## Wave 1 — Fechar o gap em Go e Node (arquivos disjuntos, paralelo)
> Dependencies: none

### ML-1A — Go: gerar scripts de attention em discover --init
**Status:** ✅ Concluído
**Files affected:**
- `internal/generators/scaffold.go` (exportar `generateAttentionScripts` → `GenerateAttentionScripts`, ou criar wrapper exportado)
- `internal/discover/discover.go` (`InstallGates`, por volta da linha 49-64)
**Actions:**
1. Exportar (ou expor via wrapper) a função `generateAttentionScripts()` de `internal/generators/scaffold.go:682`, sem alterar o conteúdo gerado dos scripts (mesmo texto que `trackfw init` já produz).
2. Em `InstallGates` (`internal/discover/discover.go`), chamar essa função **antes** de
   `generators.InjectHooksDetected(rootDir)` — mesma posição relativa usada em
   `pypi/trackfw/commands/discover.py:497-500` (depois de `inject_rules_detected`, antes de
   `inject_hooks_detected`).
3. Atualizar todos os call sites que hoje chamam `generateAttentionScripts()` sem argumento, se a
   assinatura mudar ao exportar.
**Acceptance criteria:**
- [x] `go build ./...` sem erros
- [x] Teste novo/atualizado em `internal/discover/*_test.go` ou `internal/commands/discover_test.go`
      confirmando que `discover --init` num diretório temporário produz
      `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` no disco,
      com o mesmo conteúdo de `trackfw init`
- [x] `go test ./internal/...` verde

> Achado extra durante a implementação (registrado em `vault/notes/go-generateattentionscripts-cwd-vs-rootdir-2026-08-04.md`):
> a função original escrevia em `"scripts"` relativo ao cwd do processo, não ao `rootDir` recebido por
> `InstallGates` — corrigido para escrever em `filepath.Join(rootDir, "scripts")`, mantendo a mensagem
> impressa como `scripts/...` relativo (paridade com o texto fixo do Node).

### ML-1B — Node.js: gerar scripts de attention em discover --init
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/commands/discover.js` (bloco `opts.init`, próximo à linha 426-445)
**Actions:**
1. Importar `generateAttentionScripts` de `../generators/hooks` (já exportada, `npm/src/generators/hooks.js:122`).
2. Chamar `generateAttentionScripts(cfg, cwd)` **antes** de `injectHooksDetected(cwd)`, mesma posição
   relativa do Python. Confirmar que `cfg` disponível nesse ponto do fluxo de `discover.js` é
   suficiente para a assinatura da função (comparar com o call site em `npm/src/generators/init.js:35`).
**Acceptance criteria:**
- [x] Teste novo/atualizado em `npm/tests/*.test.js` confirmando que `discover --init` num diretório
      temporário produz os dois scripts no disco, com o mesmo conteúdo de `trackfw init`
- [x] `npm test` verde (359/359)

## Wave 2 — Validação cruzada
> Dependencies: Wave 1 completa

### ML-2A — Confirmar paridade e fechar a REQ
**Status:** ✅ Concluído
**Files affected:** nenhum (só validação)
**Actions:**
1. Rodar `discover --init` num fixture idêntico nos 3 runtimes e comparar byte-a-byte o conteúdo de
   `scripts/trackfw-attention-signal.sh`/`cleanup.sh` gerado por cada um.
2. Confirmar que rodar `discover --init` duas vezes seguidas não altera os arquivos na segunda vez
   (idempotência).
**Acceptance criteria:**
- [x] `make quality` verde (99/99 cenários de falsificação passaram)
- [x] `trackfw validate` sem violações

> Auditoria manual (trackfw_architect): rodei `discover --init` de verdade nos três binários (Go
> compilado, `node npm/bin/trackfw`, `python3 -m trackfw`) contra fixtures git novos e independentes.
> Os três geram `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh`,
> executáveis (0755). Divergência de conteúdo Go↔Node (comentário PT vs EN, formatação do `sed`) é
> **pré-existente na main antes desta branch** — confirmado via `git show main:...` — e fora do escopo
> desta REQ, que trata apenas de *gerar* os scripts, não de paridade de conteúdo entre runtimes.
