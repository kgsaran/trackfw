---
status: Done
date: 2026-07-20
author: Zeus
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md"
---

# REQ: hardening de qualidade attention-hooks pos-PR59 (Q1-Q8)

> Date: 2026-07-20 | Status: Done

## Motivation

Reanálise de **qualidade de código** do PR #59 (`fix(attention-hooks): correções e hardening
pós-auditoria dos PRs #56/#57`) — que fecha os 13 achados C1–C13 com testes verdes. A correção
funcional está correta, mas a reanálise (leitura de código + reprodução empírica + comparação
lado-a-lado dos 3 CLIs) revelou **8 problemas de qualidade/robustez/paridade** que os testes verdes
escondem. A causa-raiz reincide: **testar a forma gerada, não o comportamento externo** — a mesma
classe de falha que originou o retrabalho dos PRs #56/#57.

Esta REQ formaliza o hardening de qualidade, mantendo a **Regra Dura de Paridade — 3 CLIs**
(Go, Node.js, Python com semântica idêntica).

### Evidências (verificadas no código / reproduzidas empiricamente)

**🔴 Alta**

- **Q1 — Teste de contrato do Go não testa o contrato, só a forma.** `internal/generators/scaffold_test.go`
  (`TestGenerateAttentionScripts`, ~L108-144) apenas assere que o `.sh` existe, é não-vazio, tem permissão
  de execução e **contém a string do cabeçalho** — **nunca executa o script**. Node
  (`npm/tests/generators.test.js`, `execSync` ~5×) e Python (`pypi/tests/test_generators_init.py`,
  `subprocess.run` ~4×) executam o script gerado e validam C1/C4/C5 de fato. O Go está a um refactor de
  reintroduzir silenciosamente C1 (pipefail), C4 (traversal) e C5 (escaping) com a suíte verde. Viola a
  paridade de teste tri-CLI e reabre o anti-padrão que a Wave 3 dos PRs #56/#57 deveria eliminar.

**🟠 Média**

- **Q2 — Escaping de JSON incompleto nos 3 CLIs (mesma classe do C5).** Reproduzido: o escaping trata `\`,
  `"` e remove `\n`, mas **não escapa os demais caracteres de controle** (U+0000–U+001F). Um TAB ou CR
  literal em `MSG` gera JSON inválido — `jq` e `python json.loads` **ambos rejeitam**
  (`Invalid control character`). Como `MSG` vem de `tool_input.question`/`command` (texto arbitrário do
  agente), um tab basta para o board falhar o parse e o banner **não aparecer**.
- **Q3 — Contenção de path traversal DIVERGE entre CLIs (viola paridade dura).** Go relativiza
  absolutos-sob-CWD e usa `*..*` amplo; Node/Python rejeitam **todo** absoluto e usam patterns por segmento
  (`/*|../*|*/../*|*/..|..`). Consequências reais: Go aceita `/repo/atual/docs/...` (relativiza) mas
  Node/Python descartam; Go **rejeita por engano** diretório legítimo `v1..2` que Node/Python aceitam.
- **Q4 — `tr -d '\n'` (Go/Python) vs `tr -d '\r\n'` (Node).** Divergência real: entrada CRLF deixa `\r`
  residual em Go/Python → caractere de controle → JSON inválido (ligado ao Q2).
- **Q5 — Fallback sem `jq` nunca testado.** O ramo `python3` só executa quando `jq` está ausente e nenhum
  dos 3 CLIs cobre esse caminho. Código real sem teste.

**🟡 Baixa**

- **Q6 — Parsing de YAML frágil:** `grep '^roadmap_dir:' | awk '{print $2}'` quebra com `roadmap_dir:valor`
  (sem espaço), valores com aspas/espaços ou comentário inline.
- **Q7 — DRY entre linguagens:** o script shell (~40 linhas) é triplicado como string literal — foi por isso
  que Q3/Q4 divergiram. Falta um teste golden de paridade comparando os 3 scripts gerados.
- **Q8 — Pressuposto de cwd não documentado:** `[ -f trackfw.yaml ] || exit 0` transforma o hook em no-op
  silencioso fora da raiz do projeto (por design, mas sem comentário/documentação).

## Acceptance Criteria

### Bloqueantes de qualidade (Alta/Média)

- [ ] **Q1** — Go ganha testes que **executam** o `.sh` gerado (via `exec.Command("bash", ...)`) cobrindo:
      (a) `trackfw.yaml` sem `roadmap_dir:` cria/remove `.trackfw-attention.json` no fallback; (b)
      `roadmap_dir:` com `..`/absoluto externo é contido; (c) `MSG` com `"`,`\`,`\n`,TAB,CR → JSON parseável.
      Paridade de teste de contrato entre os 3 CLIs (Go alcança Node/Python).
- [ ] **Q2** — Os 3 scripts escapam/removem toda a faixa de controle U+0000–U+001F (ex.: `tr -d '\000-\037'`
      ou encoder real `jq -n --arg`/`python3 -c json.dumps`); `MSG` com TAB/CR produz JSON válido e parseável.
- [ ] **Q3** — Algoritmo de contenção de path traversal **idêntico** nos 3 CLIs (mesmos casos aceitos/rejeitados).
- [ ] **Q4** — Tratamento de newline/CR uniforme nos 3 CLIs (resolvido junto com Q2).
- [ ] **Q5** — Teste (nos 3 CLIs ou ao menos onde há execução real) que exercita o fallback com `jq` ausente do `PATH`.

### Reforço (Baixa)

- [ ] **Q6** — Parsing de `roadmap_dir` tolerante a `chave:valor` sem espaço e a comentário inline (ou documentar limitação).
- [ ] **Q7** — Teste golden de paridade comparando os 3 scripts gerados (normalizando só o quoting de linguagem),
      pegando divergências futuras automaticamente.
- [ ] **Q8** — Comentar/documentar o pressuposto de cwd (`[ -f trackfw.yaml ] || exit 0`).

### Gate

- [x] `make quality` (Go + Node.js + Python + contratos de paridade) verde.
- [x] `trackfw validate` sem violações.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-07-20-hardening-qualidade-attention-hooks-pos-pr59.md
