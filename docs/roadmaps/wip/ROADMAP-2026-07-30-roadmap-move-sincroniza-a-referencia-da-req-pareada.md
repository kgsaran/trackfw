---
status: wip
date: 2026-07-30
req: "REQ-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada"
squad: ""
---

# Roadmap: roadmap move sincroniza a referencia da REQ pareada

> Created: 2026-07-30 | Status: wip

## Contexto

REQ: `docs/req/REQ-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

`trackfw roadmap move` sincroniza pasta e `status:` do roadmap, mas deixa o `roadmap:` da REQ pareada
apontando para o estado anterior — o comando de governança produz um estado que o validador reprova
(`ref_targets_exist`). Constatado quatro vezes em duas sessões consecutivas.

## Critérios de Aceite

- [ ] `roadmap move` atualiza o `roadmap:` do frontmatter de toda REQ que aponte para o roadmap movido.
- [ ] Linha `Roadmap:` do corpo também atualizada, preservando backticks.
- [ ] Descoberta por varredura do `req_dir` casando basename, em layout flat e `by_agent`.
- [ ] Zero REQs → no-op silencioso; múltiplas → todas atualizadas; outra REQ → não tocada.
- [ ] Idempotente: mover duas vezes não altera bytes.
- [ ] Falha de escrita → diagnóstico nomeando a REQ + exit não-zero, sem desfazer o move.
- [ ] Paridade nos 3 CLIs com cenário byte-a-byte em `make quality`.
- [ ] `make quality` exit 0 e `validate --json` 0 violações.

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier — os três runtimes implementam contra o contrato congelado
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier — exige os três concluídos
Wave 3 — ML-3A (auditoria de paridade + não-vacuidade)
```

### Lições dos dois roadmaps anteriores, aplicadas aqui

O ML-1A dos dois roadmaps anteriores falhou **duas vezes pelo mesmo padrão**: pinou a *forma* e deixou o
*comportamento* à interpretação.

| Roadmap | O que foi pinado | O que faltou | Custo |
|---|---|---|---|
| skip de artefato desatualizado | **nomes** dos parâmetros do observador | os **valores** de cada um | 1 wave corretiva, 3 respostas divergentes |
| rótulo de wave com sufixo | os dois **regexes** | **quando** a validação roda | 1 wave corretiva, bug que o teste não pegava |

Regra derivada, aplicada ao ML-1A abaixo: **pinar sempre a ordem das operações e os valores observáveis,
não apenas as estruturas e assinaturas.** Em particular, este ML-1A precisa pinar *quando* a escrita da
REQ ocorre no fluxo do move, *o que* acontece em cada cardinalidade (zero, uma, várias), e *o texto
literal* de cada linha de saída.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar o contrato de sincronização da referência
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:**
- `docs/cli-parity.md` — nova seção sob a governança de referências canônicas

**Seção escrita:** `### roadmap move synchronizes the paired REQ reference`, sob
`## Canonical governance references` em `docs/cli-parity.md`.

**Pinado:**
1. **Direção e momento.** A sincronização é unidirecional (corrige quem aponta para o roadmap) e ocorre
   **após** o rename bem-sucedido, no mesmo ponto onde o `status:` do roadmap já é reescrito.
2. **Fonte de descoberta.** Varredura do `req_dir` — flat **e** `by_agent` — casando o **basename** do
   roadmap no `roadmap:` da REQ. Explicitamente **não** usar o `req:` do roadmap, que é frequentemente
   vazio.
3. **Qual campo é normativo.** Frontmatter é o que o validador lê (`extractRefPath` ignora a forma com
   backticks do corpo); o corpo é atualizado por coerência com o leitor humano, preservando formatação.
4. **Cardinalidade, caso a caso.** Zero → no-op silencioso exit 0. Uma → atualiza. Várias → atualiza
   todas. Aponta para outro roadmap → não toca. Já correta → **nenhuma escrita** (idempotência
   byte-a-byte).
5. **Texto literal de cada linha de saída**, incluindo o caso de falha, e em qual stream sai.
6. **Comportamento em erro:** o move **não** é desfeito; diagnóstico nomeia a REQ; exit não-zero.

**Critérios de aceite:**
- [x] Momento da escrita no fluxo pinado explicitamente — **após** rename bem-sucedido, no mesmo ponto
      onde o `status:` do roadmap já é reescrito, "nunca antes, para que um rename falho não deixe edição
      pendurada".
- [x] Fonte de descoberta pinada, com `by_agent` coberto e o `req:` do roadmap **explicitamente excluído**
      (é `""` recém-criado e slug sem caminho nos existentes).
- [x] Campo normativo vs. campo de coerência distinguidos, com a razão: `extractRefPath` trima aspas mas
      **não** backticks, então a forma do corpo é invisível ao validador — "uma implementação que atualiza
      só o corpo não corrige nada".
- [x] **Cinco** cardinalidades pinadas em tabela (zero, uma, várias, aponta para outro, já correta), não
      quatro como eu havia previsto: separei "aponta para outro roadmap" de "já correta".
- [x] Textos de saída pinados literalmente com stream: `✓ synced <req> → <path>` em stdout;
      `trackfw roadmap move: failed to sync <req>: <cause>` em stderr.
- [x] Comportamento em erro pinado: **não** desfaz o move, tenta as REQs restantes, exit não-zero ao fim
      — "um arquivo não-gravável não esconde os demais".

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Arquivos disjuntos — **spawn simultâneo**.

### ML-2A — Go
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `internal/generators/roadmap.go` (`MoveRoadmap`, linha ~326, após o
`rewriteRoadmapStatus` da linha ~372), testes correspondentes

**Critérios de aceite:**
- [ ] Todas as cardinalidades conforme o contrato.
- [ ] Idempotência provada por comparação de bytes após dois moves.
- [ ] `by_agent` coberto por teste.
- [ ] `go build ./...`, `go test ./...`, `go vet ./...` passam.

### ML-2B — Node.js
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `npm/src/generators/roadmap.js` (`moveRoadmap`), testes em `npm/tests/`

**Critérios de aceite:**
- [ ] Comportamento e textos equivalentes ao Go.
- [ ] Idempotência e `by_agent` cobertos.
- [ ] `cd npm && npm test` passa.

### ML-2C — Python
**Status:** ⬜ Pendente
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/commands/roadmap.py` (`_cmd_move`, linha ~99) e o gerador
correspondente, testes em `pypi/tests/`

**Critérios de aceite:**
- [ ] Comportamento e textos equivalentes ao Go e Node.
- [ ] Idempotência e `by_agent` cobertos.
- [ ] Suíte Python passa.

---

## Wave 3 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — ML-2A, ML-2B e ML-2C concluídos.

### ML-3A — Auditar paridade e provar não-vacuidade
**Status:** ⬜ Pendente
**Agente:** Artemis

**Ações:**
1. Cenário de paridade executando os **três** CLIs sobre a mesma árvore, comparando **bytes** da saída
   e do conteúdo resultante das REQs, nas quatro cardinalidades. Com vacuity-guard.
2. Cenário de falsificação provando que o gate **detecta** a regressão — seam que corrompe a fixture,
   nunca a asserção, seguindo o padrão de `BARRIER_SELFTEST_BREAK`.
3. Cenário `by_agent`, que é onde a varredura tende a divergir entre runtimes.
4. Encadear em `make quality`.

**Critérios de aceite:**
- [ ] Quatro cardinalidades cobertas nos três runtimes, byte-a-byte.
- [ ] Vacuity-guard presente; seam de falsificação prova poder de reprovação.
- [ ] `by_agent` coberto.
- [ ] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.
