---
status: wip
date: 2026-07-20
req: "docs/req/REQ-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md"
squad: ""
---

# Roadmap: hardening qualidade attention-hooks pos-pr59

> Created: 2026-07-20 | Status: wip

## Context

Implementa o hardening de qualidade do PR #59 (ver REQ). Os 13 achados C1–C13 estão corretos, mas a
reanálise de qualidade encontrou 8 problemas (Q1–Q8) de robustez, paridade tri-CLI e — sobretudo —
**testes que validam a forma, não o comportamento** (Q1). A causa-raiz é a mesma do retrabalho anterior,
por isso a **Wave 3 é uma barrier de contrato/paridade**.

REQ: docs/req/REQ-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md

### Decisões canônicas (OBRIGATÓRIAS — garantem paridade idêntica nos 3 CLIs)

> Os 3 CLIs devem gerar scripts com **comportamento byte-idêntico** nestes pontos. Adotar exatamente:

1. **Contenção de path traversal (Q3):** adotar o padrão segment-aware do Node/Python como canônico e
   **remover a lógica de relativização de absolutos do Go**. Bloco único, idêntico nos 3, em signal e cleanup:
   ```sh
   ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}
   case "$ROADMAP_DIR" in
     /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
   esac
   ```
   Efeito: rejeita todo caminho absoluto e todo componente `..`, aceita nomes legítimos como `v1..2`.

2. **Sanitização de controle + escaping (Q2 + Q4):** substituir `tr -d '\n'` / `tr -d '\r\n'` por remoção de
   **toda a faixa de controle** ANTES do escaping, idêntico nos 3 (aplicado a `TOOL` e `MSG`):
   ```sh
   ... | tr -d '\000-\037' | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'
   ```
   (`tr -d '\000-\037'` remove `\n`, `\r`, `\t` e demais U+0000–U+001F; a ordem do `sed` — barra antes de aspa —
   permanece.)

3. **Parsing de `roadmap_dir` (Q6):** tolerar `chave:valor` sem espaço e comentário inline:
   ```sh
   ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"'"'"'"'"'" || true)
   ```
   (Se preferir manter `awk`, documentar a limitação no lugar — mas a versão tolerante é a recomendada.)

4. **Comentário de cwd (Q8):** adicionar comentário acima de `[ -f "trackfw.yaml" ] || exit 0` explicando que
   o hook é intencionalmente no-op fora da raiz do projeto.

### Mapa de dependências
```
Wave 1 (scripts: Q2/Q3/Q4/Q6/Q8 — 3 MLs paralelos por CLI) ─┐
                                                             ├→ barrier → Wave 3 (make quality + validate)
Wave 2 (testes: Q1/Q5/Q7 — depende de Wave 1) ──────────────┘
```
> Wave 2 depende da Wave 1 (os testes asseram o comportamento já corrigido).

---

## Wave 1 — Robustez e paridade dos scripts shell (3 MLs paralelos, por CLI)
> Dependencies: none

### ML-1A — Go: aplicar decisões canônicas no script embutido
**Status:** pending
**Files affected:** `internal/generators/scaffold.go` (funções `signalScript`/`cleanupScript`)
**Actions:**
1. (Q3) Substituir os DOIS blocos `case` de path traversal (o `/*` com relativização + o `*..*`) pelo bloco
   único canônico da Decisão 1 — em signal E cleanup.
2. (Q2+Q4) Trocar `tr -d '\n'` por `tr -d '\000-\037'` na sanitização de `TOOL` e `MSG` (Decisão 2).
3. (Q6) Adotar o parsing tolerante de `roadmap_dir` da Decisão 3.
4. (Q8) Adicionar o comentário da Decisão 4.
**Acceptance criteria:**
- [ ] `go build ./...` e `go vet ./...` sem erros
- [ ] `go test ./internal/generators/...` verde
- [ ] Script gerado idêntico em semântica ao de Node/Python nos 4 pontos canônicos

### ML-1B — Node: aplicar decisões canônicas no template
**Status:** pending
**Files affected:** `npm/src/generators/hooks.js` (template do script)
**Actions:**
1. (Q3) Confirmar/manter o bloco `case` canônico (Node já usa o padrão segment-aware) em signal e cleanup.
2. (Q2+Q4) Trocar `tr -d '\r\n'` por `tr -d '\000-\037'` (Decisão 2).
3. (Q6) Adotar o parsing tolerante de `roadmap_dir` (Decisão 3).
4. (Q8) Adicionar o comentário de cwd (Decisão 4).
**Acceptance criteria:**
- [ ] `node --test` verde
- [ ] Script gerado idêntico em semântica ao de Go/Python nos 4 pontos canônicos

### ML-1C — Python: aplicar decisões canônicas nos templates
**Status:** pending
**Files affected:** `pypi/trackfw/generators/init_gen.py` (`_ATTENTION_SIGNAL_SH`/`_ATTENTION_CLEANUP_SH`)
**Actions:**
1. (Q3) Confirmar/manter o bloco `case` canônico (Python já usa o padrão segment-aware) em signal e cleanup.
2. (Q2+Q4) Trocar `tr -d '\n'` por `tr -d '\000-\037'` (Decisão 2).
3. (Q6) Adotar o parsing tolerante de `roadmap_dir` (Decisão 3).
4. (Q8) Adicionar o comentário de cwd (Decisão 4).
**Acceptance criteria:**
- [ ] `pytest pypi/tests/` verde
- [ ] Script gerado idêntico em semântica ao de Go/Node nos 4 pontos canônicos

---

## Wave 2 — Testes de contrato e paridade (3+1 MLs)
> Dependencies: Wave 1 completa (os testes asseram o comportamento corrigido)

### ML-2A — Go: testes de contrato que EXECUTAM o script (Q1) + fallback sem jq (Q5)
**Status:** pending
**Files affected:** `internal/generators/scaffold_test.go`
**Actions:**
1. (Q1) Adicionar teste(s) que executam o `.sh` gerado via `exec.Command("bash", signalPath)` cobrindo:
   (a) `trackfw.yaml` SEM `roadmap_dir:` → cria `.trackfw-attention.json` no fallback `docs/roadmaps` e o
   cleanup remove; (b) `roadmap_dir:` com `..` e absoluto externo → contido em `docs/roadmaps`;
   (c) `MSG` (via stdin JSON) com `"`, `\`, `\n`, TAB e CR → `.trackfw-attention.json` é JSON parseável
   (validar com `encoding/json.Unmarshal`).
2. (Q5) Teste que executa o signal com `jq` removido do `PATH` (env `PATH` reduzido) exercitando o fallback
   `python3` → JSON válido gerado.
**Acceptance criteria:**
- [ ] Go executa o script gerado (não só string-contains) nos 3 cenários C1/C4/C5
- [ ] Teste de fallback sem `jq` verde
- [ ] `go test ./internal/generators/...` verde

### ML-2B — Node: fallback sem jq (Q5)
**Status:** pending
**Files affected:** `npm/tests/generators.test.js`
**Actions:**
1. (Q5) Adicionar teste que executa o signal com `jq` mascarado do `PATH`, validando o fallback `python3`
   e JSON parseável. (Node já executa o script para C1/C4/C5 — apenas complementar o caso sem `jq`.)
**Acceptance criteria:**
- [ ] `node --test` verde com o novo caso de fallback

### ML-2C — Python: fallback sem jq (Q5)
**Status:** pending
**Files affected:** `pypi/tests/test_generators_init.py`
**Actions:**
1. (Q5) Adicionar teste `subprocess.run` com `jq` mascarado do `PATH`, validando o fallback `python3` e
   `json.loads` do arquivo gerado. (Python já executa o script para C1/C4/C5.)
**Acceptance criteria:**
- [ ] `pytest pypi/tests/` verde com o novo caso de fallback

### ML-2D — QA: teste golden de paridade dos 3 scripts (Q7)
**Status:** pending
**Files affected:** novo teste (ex.: `internal/generators/scaffold_parity_test.go` ou script em
`scripts/` chamado por `make quality`)
**Actions:**
1. (Q7) Gerar os scripts dos 3 CLIs e comparar os 4 pontos canônicos (bloco de traversal, sanitização de
   controle, parsing de `roadmap_dir`, comentário de cwd), normalizando apenas o quoting específico de
   linguagem. Falhar se divergirem. Objetivo: pegar divergências futuras automaticamente.
**Acceptance criteria:**
- [ ] Teste golden compara os 3 scripts e passa
- [ ] Divergência proposital em 1 CLI faz o teste falhar (validado localmente)

---

## Wave 3 — BARRIER: qualidade + validação (1 ML)
> Dependencies: Wave 1 E Wave 2 completas

### ML-3A — Gate final
**Status:** pending
**Files affected:** — (execução de gates)
**Actions:**
1. Rodar `make quality` (Go + Node.js + Python + contratos de paridade).
2. Rodar `trackfw validate`.
3. Atualizar este roadmap para `done` e marcar todos os MLs concluídos.
**Acceptance criteria:**
- [ ] `make quality` verde
- [ ] `trackfw validate` sem violações
- [ ] `MSG` com TAB/CR gera JSON parseável nos 3 CLIs (Q2/Q4 comprovado por teste)
- [ ] Paridade de contenção de traversal idêntica nos 3 CLIs (Q3 comprovado por teste golden)
