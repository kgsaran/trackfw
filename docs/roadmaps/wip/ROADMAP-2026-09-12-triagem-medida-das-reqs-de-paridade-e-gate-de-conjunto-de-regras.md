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
- [ ] AC9 — gate de paridade de **subcomandos**, comparando conjuntos nos dois sentidos (#298)
- [ ] AC10 — falsificar a direção **sobrando**, que o relator do #298 não falsificou
- [ ] AC11 — a lista de comandos com subcomando não se descobre sozinha: derivar, ou declarar o sítio
- [ ] AC12 — `known_divergences` com motivo escrito por entrada
- [ ] AC13 — `docs/cli-parity.md` ganha a tabela de subcomandos

## 🔴 FRENTE PARADA em 2026-09-12 — decisão do KG

**Estado: MLs restantes marcados ❌ Bloqueado; o roadmap fica em `wip/`.**

> 🔴 **Defeito encontrado ao executar esta própria parada:** mover o roadmap para `blocked/` torna a
> branch `fix/` não-conforme e **impede commitar o ato de bloquear** — `branch_has_wip_roadmap`
> aceita só `wip/` e `done/`, apesar de `blocked` ser estado documentado do ciclo
> (`backlog / analyzing / wip / blocked / done / abandoned`). Catch-22: para registrar o bloqueio é
> preciso não bloquear. Por isso o roadmap permanece em `wip/` com os MLs marcados ❌. **Registrado
> como achado, não contornado em silêncio.**
 Não é abandono e não é pausa por falta de gente: é **decisão de não construir
o que já se sabe que será apagado**.

### O que a levou a parar

`ADR-2026-09-12-estrategia-de-distribuicao` foi **aceita**, e a direção é a **opção D — um binário,
muitos canais**: uma implementação em Go distribuída por npm e pip com o binário dentro do pacote,
no lugar da reimplementação tripla.

Os dois entregáveis restantes desta frente são **infraestrutura de paridade**:

```
ML-2A     check-rule-set-parity.sh      compara o conjunto de REGRAS entre 3 runtimes
ML-2A-b   check-subcommand-parity.sh    compara o conjunto de SUBCOMANDOS entre 3 runtimes
```

Com um runtime, os dois **não passam a passar — deixam de ter objeto.**

### 🔴 E o ML-2A trouxe a demonstração, não a projeção

O agente de QA mediu e **bloqueou, corretamente**: não existe superfície executável que enumere as
regras implementadas em runtime nenhum.

```
--list-rules / --rules                       unknown flag nos 3
audit-surface · configure · doctor · context  nenhum expõe o conjunto
validate --json                               só mostra regra que DISPARA
```

E o argumento que fecha a questão:

> *"Uma regra implementada mas não acionada pelo fixture é byte-a-byte indistinguível de uma regra
> não-implementada. Construir um fixture que dispare TODAS as regras exigiria saber o conjunto a
> priori — que é exatamente o que o gate deveria produzir. É circular."*

As duas saídas fáceis foram recusadas pelos motivos certos: os *severity maps* dão conjunto
**parcial por construção** (só listam regras cujo default não é `error`), e grep de fonte mede
**ortografia, não implementação** — a classe de gate vácuo que este projeto já pagou quatro vezes.

Sobra uma única opção viável: **adicionar `--list-rules` aos três CLIs.**

🔴 **Ou seja: para construir o detector do imposto de paridade, seria preciso pagar o imposto de
paridade** — implementar um comando novo três vezes, para criar um gate que a opção D apaga.

### O que esta frente JÁ entregou, e vale independentemente da v8

| ML | entrega | sobrevive a D? |
|---|---|---|
| ML-1A | triagem medida: 0 entregues, 2 parciais, 4 pendentes | ✅ sim |
| ML-1B | vereditos aplicados; parciais registram que a evidência original era falsa | ✅ sim |
| — | **o Go é a expressão da verdade** no `CLAUDE.md` e no `cli-parity.md`, com o porquê | ✅ sim, e vira central |
| — | três notas de vault do byte NUL consolidadas em uma | ✅ sim |
| — | absorção do #298 e do #310, com a convergência decidida | ✅ a decisão sim |

### O que reabre esta frente

A decisão da v8, que depende de `ROADMAP-2026-09-12-validar-um-binario-muitos-canais`:

- **v8 adota D** ⇒ ML-2A e ML-2A-b são **abandonados**, e os ACs de gate das REQs parciais fecham
  por desaparecimento da causa. Registrar isso nas REQs, não deixá-las abertas.
- **v8 não adota D** ⇒ esta frente **volta para `wip`**, e o ML-2A começa pela decisão de contrato:
  criar `--list-rules` nos três CLIs.

🔴 **Em nenhum dos dois casos as REQs parciais ficam como estão.** Elas hoje esperam um gate; se o
gate deixar de ser necessário, quem fecha é o registro da causa desaparecida.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 1 — Medir antes de concluir
> Dependências: nenhuma.

### ML-1A — **AC1** — triagem medida das REQs de ausência
**Status:** ✅ Concluído
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
**Status:** ✅ Concluído
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
**Status:** ❌ Bloqueado — ver "FRENTE PARADA" acima
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

### ML-2A-b — **AC9 + AC10 + AC11 + AC12** — gate de paridade de subcomandos (#298)
**Status:** ❌ Bloqueado — ver "FRENTE PARADA" acima
**Arquivos afetados:** novo `scripts/check-subcommand-parity.sh`, `Makefile`.
🔴 **Não alterar `scripts/check-cli-parity.sh`** — ele mede o primeiro nível e continua válido para
isso. Este gate desce um nível.

**Mesma causa do ML-2A, superfície ao lado:** lá é o conjunto de **regras**, aqui é o conjunto de
**subcomandos**. Os dois são "gate que verifica itens, não o conjunto".

**Contexto medido:** `check-cli-parity.sh` enumera só o primeiro nível (lista literal na linha 34,
comparação via `--help` da raiz na linha 48). Doze subcomandos sem gate: `adr` 3, `req` 4,
`roadmap` 5. O comentário do gate atual registra que isso **já mordeu**: *"`req move` faltou nos três
e `req list` faltou no Python sem nenhum gate avisar."*

**Ações:**
1. Descer um nível a partir do `--help` de cada comando com subcomando; comparar conjuntos **nos
   dois sentidos** — faltando **e** sobrando.
2. `known_divergences` no formato `<comando>:<runtime>:<subcomando>:<faltando|sobrando>`, **cada
   entrada exigindo motivo escrito**. Nasce vazia.
3. 🔴 Resolver o item 2 dos limites do relator: ou derivar a lista de comandos-com-subcomando do
   `--help` em vez de hardcodar, ou deixar **escrito na entrada** que é sítio de manutenção e o que
   acontece quando um quarto comando aparecer.

**Critérios de aceite:**
- [ ] Braço A (o que o relator falsificou): remover subcomando de um runtime ⇒ reprova nomeando
      comando, runtime e subcomando
- [ ] 🔴 Braço B (**o que ele NÃO falsificou**): plantar subcomando **sobrando** num runtime ⇒
      reprova. A direção existe no desenho dele mas nunca foi provada por sabotagem
- [ ] Contra-braço: árvore intacta ⇒ passa. *(Um gate que reprovasse sempre também "pegaria" o caso
      plantado — é a ressalva do próprio relator.)*
- [ ] `known_divergences` sem motivo escrito ⇒ o gate recusa a entrada
- [ ] Wired no `Makefile`, executado por `make quality`
**Reconciliação:** cada teste novo declara qual conclusão do ML ele afirma.

### ML-2B — **AC7 + AC8 + AC13** — documentação e fechamento
**Status:** ❌ Bloqueado — ver "FRENTE PARADA" acima
**Arquivos afetados:** `docs/cli-parity.md`.
**Critérios de aceite:**
- [ ] Seção nomeando `check-rule-set-parity.sh` e o que ele cobre
- [ ] Seção nomeando `check-subcommand-parity.sh` e o que ele cobre
- [ ] 🔴 Tabela de **subcomandos** por runtime em `docs/cli-parity.md` — hoje só existe a de
      primeiro nível, e é essa ausência que deixou os 12 sem cobertura documental
- [ ] `make quality` verde e CI verde

---

## Escopo negativo

- **Não** implementar as paridades apontadas como genuinamente pendentes — cada uma tem sua REQ, e
  misturar implementação com triagem impede saber qual mudança produziu qual efeito.
- **Não** tocar `internal/validator/`, `internal/generators/roadmap.go`, `branch new`, `commit` ou
  `roadmap move` — 🔴 são da REQ de REQ órfã, **em execução paralela neste momento**.
- **Não** alterar `scripts/check-rules-parity.sh`.
- **Não** corrigir o `grep` do ambiente — é config de shell do usuário, não do produto.
