---
status: backlog
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md"
squad: "apolo-tf"
---

# Roadmap: json.MarshalIndent do Go escapa HTML e diverge de Node/Python em 3 targets do catalogo (kiro amazonq antigravity-legacy)

> Created: 2026-08-04 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md`

`internal/integrations/render.go:57` usa `json.MarshalIndent` puro, que faz HTML-escaping por
padrão — Node.js (`JSON.stringify`) e Python (`json.dumps`) não escapam. Isso diverge nos 3 targets
que usam a representação `agent-json`/`cli-agent-json` (kiro/cli, amazonq/cli,
antigravity/legacy-cli) sempre que o conteúdo do prompt contém `<`, `>` ou `&` — hoje exposto pelo
placeholder literal `<slug>` no texto do "Dispatch contract" do Architect. Fix pontual +
auditoria dos demais pontos de serialização JSON do Go que têm contrato de paridade byte-a-byte com
Node/Python.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `render.go:57` corrigido, `check-identity-parity.sh` com 0 falhas
- [ ] Teste de regressão com caractere `<`/`>`/`&` no conteúdo de origem
- [ ] Auditoria dos demais `json.Marshal*` com contrato de paridade cross-runtime concluída
- [ ] `make quality` verde, `trackfw validate` sem violações

## Wave 1 — Fix pontual + regressão
> Dependencies: none

### ML-1A — Corrigir render.go:57 e adicionar teste de regressão
**Status:** pending
**Files affected:**
- `internal/integrations/render.go` (linha ~57, `case "cli-agent-json", "agent-json":`)
- `internal/integrations/render_test.go` (novo teste, ou extensão de existente)
**Actions:**
1. Trocar `json.MarshalIndent(map[string]string{...}, "", "  ")` por um `json.Encoder` com
   `SetEscapeHTML(false)` e `SetIndent("", "  ")`, escrevendo num `bytes.Buffer`. Atenção:
   `Encoder.Encode` já adiciona `\n` ao final — conferir se o `append(data, '\n')` logo depois no
   código atual precisa ser removido para não duplicar a quebra de linha (comparar byte-a-byte com
   a saída anterior menos o caractere escapado, para garantir que só o escaping mudou).
2. Adicionar um teste (fixture ou golden) que injeta um valor com `<`, `>` e `&` no source antes de
   `Render()` e confirma que a saída para `cli-agent-json`/`agent-json` não contém `<`,
   `>` nem `&`.
**Acceptance criteria:**
- [ ] `go build ./...`, `go test ./internal/integrations/...` verdes
- [ ] `GO_BIN=bin/trackfw scripts/check-identity-parity.sh` com 0 falhas (hoje: 6)
- [ ] `go build -o bin/trackfw ./cmd/trackfw && bin/trackfw agents list --targets kiro,amazonq,antigravity --json` inspecionado manualmente — nenhum `<`/`>`/`&` na saída

## Wave 2 — Auditoria dos demais pontos de serialização
> Dependencies: Wave 1 completa

### ML-2A — Auditar os demais json.Marshal* com contrato de paridade cross-runtime
**Status:** pending
**Files affected:** nenhum a priori — auditoria primeiro, fix só se confirmado
**Actions:**
1. Revisar `internal/generators/agentfiles.go` (6 call sites de `json.MarshalIndent`, geram
   `.claude/settings.json` e equivalentes de outros CLIs) e a saída `--json` de `trackfw
   validate`/`barrier`/`update` (`internal/commands/validate.go`, `barrier.go`, `update.go`,
   `update_harness.go`) — para cada um, checar se o conteúdo de origem pode conter `<`/`>`/`&` na
   prática (ex: mensagens de commit, texto de roadmap/REQ do usuário) e se existe algum gate de
   paridade Go×Node×Python que exercitaria essa saída.
2. Onde houver risco real confirmado (não só teórico) e gate existente, aplicar o mesmo fix do
   ML-1A. Onde for só risco teórico sem gate cobrindo, documentar a decisão (corrigir preventivamente
   é opcional, mas registrar por quê) — não é obrigatório blindar tudo se não há evidência de uso
   real com esses caracteres.
3. Chamadas Go-only sem equivalente cross-runtime (`internal/serve/`, `internal/sync/`) ficam de
   fora — não há contrato de paridade byte-a-byte pra elas, confirmar isso e não tocar.
**Acceptance criteria:**
- [ ] Auditoria registrada nesta seção do roadmap (atualizar com o resultado por arquivo)
- [ ] Todo fix aplicado (se houver) com teste de regressão equivalente ao ML-1A
- [ ] `make quality` verde
- [ ] `trackfw validate` sem violações
