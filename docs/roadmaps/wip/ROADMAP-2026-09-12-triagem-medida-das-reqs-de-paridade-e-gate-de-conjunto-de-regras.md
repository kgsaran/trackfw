---
status: wip
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-reqs-de-paridade-nao-distinguem-entregue-de-pendente-porque-o-gate-que-provaria-a-entrega-nao-existe.md"
squad: ""
---

# Roadmap: Triagem medida das REQs de paridade e gate de conjunto de regras

> Created: 2026-09-12 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-12-reqs-de-paridade-nao-distinguem-entregue-de-pendente-porque-o-gate-que-provaria-a-entrega-nao-existe.md

Duas REQs abertas de paridade foram medidas em 2026-09-12 e **já estavam implementadas**. O código
foi entregue; o gate que provaria a entrega, não. Sem ele, saber se uma paridade fechou exige ler
três fontes à mão — que é o que ninguém faz.

> 🔴 Este bloco de ACs foi preenchido **à mão**. O `roadmap new --req` gerou um ML genérico e deixou
> os ACs vazios — é o AC7 da REQ de REQ órfã, em execução paralela. Registrado como evidência viva.

## Acceptance Criteria
- [ ] AC1 — re-triagem medida de toda REQ aberta com evidência de ausência num runtime
- [ ] AC2 — REQs entregues vão para `Done` com evidência escrita (arquivo, linha, saída)
- [ ] AC3 — REQs parciais têm o AC pendente isolado; **não fechar parcial**
- [ ] AC4 — gate de conjunto: enumera regras **implementadas** por runtime e reprova na divergência
- [ ] AC5 — falsificação do AC4 nas duas direções
- [ ] AC6 — guarda de vacuidade: conjunto vazio ⇒ reprova, não "paridade"
- [ ] AC7 — `docs/cli-parity.md` nomeia o gate
- [ ] AC8 — `make quality` verde e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 1 — Medir antes de concluir
> Dependências: nenhuma.

### ML-1A — **AC1** — triagem medida das REQs de ausência
**Status:** ⬜ Pendente
**Arquivos afetados:** somente `docs/qualidade/2026-09-12-triagem-medida-das-reqs-de-paridade.md` (novo).
Nenhum arquivo de produto neste ML.

🔴 **Ferramenta obrigatória:** `/usr/bin/grep` ou `git grep`. **NUNCA** o `grep` do shell — ele é
`ugrep -I` e omite silenciosamente arquivos com NUL byte, incluindo `npm/src/validator/index.js`
(190 KB, o maior fonte do CLI Node). Foi esse defeito que produziu a evidência falsa das duas REQs.
Ver `vault/notes/grep-do-ambiente-pula-arquivo-com-nul-2026-09-12.md`.

**Candidatas** (REQs abertas com evidência de ausência num runtime):
```
REQ-2026-08-20-note-orphan-existe-em-go-e-python-e-esta-ausente-do-cli-node
REQ-2026-09-01-regra-thirdparty-artifact-has-provenance-existe-em-go-e-python-mas-nao-no-validator-do-node
REQ-2026-08-20-validate-json-do-python-nao-rotula-a-regra-branch-has-wip-roadmap
REQ-2026-08-28-cli-python-nao-oferece-superficie-de-ci-e-git-hooks-no-init-e-nao-declara-git-hooks-como-alvo-do-update
REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-status-do-python-conta-reqs-flat-e-walkmd-do-node-indexa-sem-filtro
REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade
```
🔴 **A lista acima é ponto de partida, não escopo fechado.** Varrer `docs/req/*.md` com
`status: Open` e reportar qualquer outra que caiba, com o critério usado.

**Ações, por REQ:**
1. Medir a presença com `/usr/bin/grep` nos 3 fontes.
2. **Executar os três binários** e comparar a saída real — presença no fonte não prova comportamento.
3. Ler os ACs da REQ e dizer, **por AC**, se está entregue.
4. Emitir o veredito: **entregue** · **parcial (código sim, gate não)** · **pendente**.

**Critérios de aceite:**
- [ ] Um veredito por REQ, com arquivo, linha e a saída que o prova
- [ ] Nenhum veredito baseado só em leitura de fonte
- [ ] Divergência entre "fonte tem" e "binário faz" reportada explicitamente, não resolvida sozinha

**Reconciliação:** o relatório declara, por veredito, qual medição o sustenta.

### ML-1B — **AC2 + AC3** — aplicar os vereditos
**Status:** ⬜ Pendente
**Arquivos afetados:** os `docs/req/*.md` triados no ML-1A.
**Ações:**
1. Veredito **entregue** ⇒ `trackfw req move <nome> Done`, e escrever a evidência **no artefato**.
2. Veredito **parcial** ⇒ 🔴 **não fechar.** Marcar no artefato quais ACs faltam e por quê.
3. Veredito **pendente** ⇒ não tocar.
**Critérios de aceite:**
- [ ] Toda REQ fechada carrega a evidência no próprio arquivo
- [ ] Nenhuma REQ parcial fechada
- [ ] `trackfw validate` RC=0

---

## Wave 2 — O gate que faltava
> Dependências: Wave 1 (a triagem diz o que o gate precisa cobrir).

### ML-2A — **AC4 + AC5 + AC6** — gate de conjunto de regras
**Status:** ⬜ Pendente
**Arquivos afetados:** novo `scripts/check-rule-set-parity.sh`, `Makefile`.
🔴 **Nome distinto de `check-rules-parity.sh`, que já existe e mede outra coisa** — o bloco de regras
de artefatos gerados (4 arquivos × 3 runtimes), não as regras implementadas. Não alterar aquele gate.
**Ações:**
1. Enumerar as regras **implementadas** em cada runtime, **executando o binário**. Se não houver
   superfície que liste as regras, reportar ao arquiteto antes de inventar uma.
2. Comparar os três conjuntos; divergência ⇒ reprova **nomeando a regra e o runtime**.
3. Guarda de vacuidade: conjunto vazio em qualquer runtime ⇒ **reprova**. Dois vazios não são
   paridade.
**Critérios de aceite:**
- [ ] Braço: remover uma regra de um runtime ⇒ reprova nomeando regra e runtime
- [ ] Contra-braço: três conjuntos iguais ⇒ passa
- [ ] Vacuidade: enumeração vazia ⇒ reprova, com mensagem própria
- [ ] Wired no `Makefile` e executado por `make quality`
**Reconciliação:** cada teste novo declara qual conclusão do ML ele afirma.

### ML-2B — **AC7 + AC8** — documentação e fechamento
**Status:** ⬜ Pendente
**Arquivos afetados:** `docs/cli-parity.md`.
**Critérios de aceite:**
- [ ] Seção nomeando `check-rule-set-parity.sh` e o que ele cobre
- [ ] `make quality` verde e CI verde

---

## Escopo negativo

- **Não** implementar as paridades apontadas como genuinamente pendentes — cada uma tem sua REQ, e
  misturar implementação com triagem impede saber qual mudança produziu qual efeito.
- **Não** tocar `internal/validator/`, `internal/generators/roadmap.go`, `branch new`, `commit` ou
  `roadmap move` — 🔴 são da REQ de REQ órfã, **em execução paralela neste momento**.
- **Não** alterar `scripts/check-rules-parity.sh`.
- **Não** corrigir o `grep` do ambiente — é config de shell do usuário, não do produto.
