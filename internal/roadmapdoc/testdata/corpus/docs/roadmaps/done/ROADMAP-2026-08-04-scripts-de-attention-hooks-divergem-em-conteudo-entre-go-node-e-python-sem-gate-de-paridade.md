---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md"
squad: "apolo-tf"
---

# Roadmap: scripts de attention hooks divergem em conteudo entre Go Node e Python (sem gate de paridade)

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md`

Os scripts `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh`, gerados
pelos três runtimes, têm o mesmo comportamento mas texto diferente (comentário PT/EN/PT-diferente,
espaçamento, estilo de `sed`). Sem gate cobrindo isso, a divergência cresceu sem ser detectada. Fix é
puramente textual — nenhuma lógica de execução muda — mas precisa ser feito por uma única mão para
garantir que os três fiquem literalmente iguais (evitar 3 agentes paralelos decidindo o texto cada um
por conta própria e produzindo um novo desalinhamento de 3 formas diferentes).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Os dois scripts (signal e cleanup) byte-idênticos entre Go, Node.js e Python
- [x] Gate de paridade novo cobrindo esses dois scripts, integrado a `make quality`
- [x] `docs/cli-parity.md` documenta o gate
- [x] `trackfw init`/`update`/`discover --init` sem regressão em nenhum runtime

## Wave 1 — Unificar o texto canônico nos 3 runtimes (uma mão só, sequencial por design)
> Dependencies: none

### ML-1A — Escolher texto canônico e sincronizar Go, Node.js e Python
**Status:** ✅ Concluído
**Files affected:**
- `internal/generators/scaffold.go` (`signalScript`/`cleanupScript` dentro de `GenerateAttentionScripts`)
- `npm/src/generators/hooks.js:60` (`SIGNAL_SCRIPT`) e `:97` (`CLEANUP_SCRIPT`)
- `pypi/trackfw/generators/init_gen.py:779`, `:815` (dentro de `_generate_attention_scripts`)
- Goldens/fixtures de teste existentes que comparam o conteúdo desses scripts nos 3 runtimes (buscar
  antes: `grep -rln "Script e intencionalmente\|Script is intentionally\|Script é intencionalmente"
  internal npm/tests pypi/tests` e ajustar o que precisar)
**Actions:**
1. Definir o texto canônico das três divergências documentadas na REQ: (a) o comentário "no-op fora
   da raiz" — escolher uma única frase, em um único idioma (recomendo inglês, já que o restante do
   comentário de cabeçalho `# trackfw attention signal — PreToolUse/... hook` já está em inglês nos
   três); (b) presença/ausência da linha em branco após `ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}`
   — escolher presente ou ausente, uma vez; (c) estilo do `sed` em `TOOL_ESC`/`MSG_ESC` no signal
   script — escolher um dos dois estilos já usados (ambos funcionam identicamente).
2. Aplicar o texto canônico nos três literais-fonte listados acima, byte-a-byte idêntico.
3. Rodar os três runtimes de verdade (binário Go compilado, `node npm/bin/trackfw`, `python3 -m
   trackfw`) contra fixtures `git init` novos e comparar a saída com `diff` — confirmar zero diff
   entre os três pares (Go×Node, Go×Python, Node×Python) para os dois scripts.
**Acceptance criteria:**
- [x] `diff` vazio entre os três runtimes para os dois scripts (confirmado empiricamente, não só por
      leitura de código)
- [x] `go build ./...`, `go test ./internal/...` verdes
- [x] `npm test` verde
- [x] `python3 -m pytest` verde (a partir de `pypi/`)
- [x] `trackfw validate` sem violações

> Achado extra durante a implementação (fora da lista original da REQ): uma 4ª divergência — linha
> em branco entre `TIMESTAMP=...` e `TOOL_ESC=...` no script de signal (Go/Python já tinham, Node.js
> não) — só apareceu rodando os três binários reais, não por leitura de código isolada. Corrigida
> junto com as outras três.

## Wave 2 — Gate de paridade
> Dependencies: Wave 1 completa

### ML-2A — Criar gate de paridade para os scripts de attention
**Status:** ✅ Concluído
**Files affected:**
- Novo script em `scripts/` (ex: `scripts/check-attention-scripts-parity.sh`, seguindo o padrão de
  `scripts/check-integration-assets.sh`) ou extensão de um gate existente que já rode os 3 runtimes
- `Makefile` (alvo `quality`, incluir o novo gate)
- `docs/cli-parity.md` (nova seção documentando o gate)
**Actions:**
1. Gate roda os três binários (`trackfw init` ou `discover --init` num fixture temporário, um por
   runtime) e compara byte-a-byte os dois scripts gerados entre os três — falha com diff explícito se
   divergirem (P2: sem degradação silenciosa).
2. Integrar ao `make quality`.
3. Documentar em `docs/cli-parity.md`.
**Acceptance criteria:**
- [x] Gate falha de propósito quando um dos três textos é alterado manualmente (prova de
      falsificabilidade — P4, seguindo `docs/gate-design-principles.md`) — cenário 43 em
      `check-gates-falsify.sh`, corrompe o comentário do Python e confirma que
      `check-attention-scripts-parity.sh` detecta
- [x] Gate passa no estado corrigido da Wave 1
- [x] `make quality` verde
- [x] `trackfw validate` sem violações

> Auditoria manual (trackfw_architect): revalidei tudo de forma independente — build Go,
> `check-attention-scripts-parity.sh` isolado (verde), `go test ./...`/`npm test`/`python3 -m
> pytest` completos (todos verdes), e `check-gates-falsify.sh` completo (101/101, incluindo o
> cenário 43 novo). Conferi os diffs dos três literais-fonte linha a linha — convergem
> corretamente. O agente também atualizou (corretamente, comment-only, sem mudar comportamento) as
> contagens desatualizadas "14 scripts/100 cenários" em `.github/workflows/quality.yml` e
> `release.yml` para "15/101", já que o novo gate elevou a contagem.
