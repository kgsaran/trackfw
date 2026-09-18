---
status: done
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md"
squad: ""
---

# Roadmap: Suportar lista YAML inline nas chaves de config dos tres CLIs

> Created: 2026-08-02 | Status: done

## Context

REQ: docs/req/REQ-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md
ADR: docs/adr/ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis.md

**Último item da fila.** KG pediu fechar antes da tag e antes do merge do PR #105, para aceitar
tudo de uma vez.

Os três CLIs ignoram `agents: [zeus, apolo]` em silêncio. Consistente entre CLIs, logo não é
paridade — é config válida descartada sem sinal.

### Estrutura — executor único, deliberadamente

Este projeto mostrou em **todos** os ciclos que três implementações paralelas divergem: fonte de
dado, texto de mensagem, raio de alcance da mudança. Aqui a exigência é **semântica idêntica** em
nove casos de parsing. Um executor com a tabela na mão é mais seguro que coordenar três.

## Critérios de Aceite

- [ ] Tabela de 9 casos idêntica nos 3 CLIs, em **cada** chave de lista
- [ ] Caso 8 (vírgula dentro de aspas) tratado — é o que quebra separação ingênua
- [ ] Cobertura por chave: `adr_dirs`, `agents`, `acceptance_markers`, sub-listas de `link_fields`
- [ ] Formas em bloco (indentada e não indentada) não regridem
- [ ] `status` em `by_agent` com inline: 3 saídas byte-idênticas, ordem **declarada**
- [ ] Cenário de falsificação por caso, com corrupção **determinística**
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Parsing inline (1 ML, executor único nos 3 CLIs)
> Dependências: nenhuma

### ML-1A — Aceitar lista inline nos três parsers
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo (executor **único**)
**Arquivos afetados:** `internal/config/config.go`, `npm/src/config/index.js`,
`pypi/trackfw/config.py` + testes dos três

**Contrato (do ADR) — esta tabela é a definição, não sugestão:**

| # | Entrada | Resultado |
|---|---|---|
| 1 | `[a, b]` | `[a, b]` |
| 2 | `[a,b]` | `[a, b]` |
| 3 | `[ a , b ]` | `[a, b]` |
| 4 | `["a", "b"]` | `[a, b]` |
| 5 | `['a', 'b']` | `[a, b]` |
| 6 | `[a]` | `[a]` |
| 7 | `[]` | lista **vazia**, não default |
| 8 | `["a, b", "c"]` | **dois** itens: `a, b` e `c` |
| 9 | `["## Acceptance Criteria", "## Critérios de Aceite"]` | os dois |

**Acceptance criteria:**
- [x] Tabela reproduzida nos 3 CLIs — **56 casos de teste por CLI**
- [x] Caso 8 tratado com scanner char-a-char que rastreia aspas
- [x] Testado por chave: `adr_dirs`, `agents`, `acceptance_markers`, sub-listas de `link_fields`
- [x] Bloco indentado e não indentado não regridem
- [x] Suítes verdes nos 3
- [x] Escopo respeitado

**Verificação independente de Zeus:** reexecutei 7 casos da tabela em Python e Node lado a lado
— idênticos, inclusive o caso 8 (`['a, b', 'c']` vs `["a, b","c"]`). O Go conferido pelo
comportamento real do `status`, que passou a respeitar a **ordem declarada** em vez do fallback
alfabético.

**Conferência extra que ele fez sem eu pedir:** mesma chave em bloco seguida de inline no mesmo
arquivo — os três devolvem o mesmo resultado. O ADR declara esse caso indefinido; bom saber que
não diverge.

---

## Wave 2 — Barreira (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Paridade e seam por caso
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Ártemis

**Ações:**
1. Gates de paridade passam; `make quality` exit 0; `validate` verde nos 3.
2. Confirmar que os **78** cenários existentes seguem passando. **Rodar, não presumir** — o
   parser de config mudou, e cenários que escrevem `trackfw.yaml` podem ser afetados.
3. Cenário novo cobrindo a tabela, com braço de detecção **determinístico**.
4. Contador e linha final atualizados.

**Acceptance criteria:**
- [x] Gates passam; `make quality` exit 0
- [x] 78 herdados **rodados antes de qualquer edição** — nenhum quebrou com o refactor da Wave 1
- [x] Cenário 35 com fixture inline incluindo o **caso 8**; corrupção determinística
- [x] Contador **78 → 82**, calibrado por `grep -c`, não estimado
- [x] `git status --porcelain` sem resíduo

**Ela verificou a armadilha herdada antes de tudo.** O ciclo anterior teve um cenário quebrado por
refactor do alvo. Como a Wave 1 refatorou os **três** parsers de config, ela buscou
explicitamente por `corrupt_literal` apontando para eles — só o Cenário 34 toca, e o literal
`continuesOpenList` sobreviveu intacto. Nada a reparar, e a verificação foi feita **antes** de
editar.

**O achado mais fino do ciclo — ela pegou vacuidade no próprio cenário dela.** A primeira versão
configurava exatamente os agentes presentes no disco. Traçando "e se alguém reverter o ML-1A
inteiro, não só o trecho corrompido?", confirmou empiricamente com `git apply -R` que a saída
ficava **byte-idêntica ao pinado** — o cenário seria **cego** a essa classe de regressão, e
morreria no setup assim que as funções fossem apagadas.

Corrigido acrescentando um agente **presente no disco mas fora da lista configurada**. Agora:
reversão **total** → o agente extra reaparece e a contagem diverge; reversão **pontual** → o item
com vírgula some. As duas classes cobertas.

**Regra generalizada** em `vault/notes/falsificacao-fixture-vacua-contra-reversao-total-vs-parcial-2026-08-02.md`:
em cenário sobre mecanismo com **fallback**, a fixture precisa conter algo no conjunto de
fallback que **não** esteja no conjunto configurado — senão fica cega a "componente inteiro
removido", mesmo com o braço de detecção pontual funcionando.

---

## Fechamento

Concluído e auditado em 2026-08-02. `make quality` exit 0; falsificação **78 → 82**.

**A fila zerou.** Este era o último item.

**O que fecha:** os três CLIs deixam de descartar `agents: [zeus, apolo]` em silêncio. Vale para
`adr_dirs`, `agents`, `acceptance_markers` e as sub-listas de `link_fields`.

**O que NÃO fecha, e está registrado no ADR:** o parser continua sendo um subconjunto de YAML.
Listas aninhadas inline (`[[a],[b]]`), mapas inline (`{a: 1}`) e âncoras seguem sem suporte **e
sem aviso**. A classe foi reduzida, não eliminada. A solução a prazo é adotar biblioteca YAML —
barata no Go (`yaml.v3` já é indirect), mas dependência de runtime nova no Node e no Python, o
que é mudança de política e merece ADR próprio.
