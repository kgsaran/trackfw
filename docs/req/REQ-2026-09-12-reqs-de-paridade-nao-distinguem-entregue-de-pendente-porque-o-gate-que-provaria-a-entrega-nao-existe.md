---
status: Open
date: 2026-09-12
author: ""
adr: ""
roadmap: ""
---

# REQ: REQs de paridade não distinguem entregue de pendente porque o gate que provaria a entrega não existe

> Date: 2026-09-12 | Status: Open

## Motivação

Em **2026-09-12**, ao triar uma frente paralela, medi duas REQs abertas de paridade e descobri que
**as duas já estavam implementadas**:

```
REQ-2026-08-20  note_orphan "ausente do CLI Node"
   → npm/src/validator/index.js: 3 sítios, incluindo applyRule('note_orphan', ...)

REQ-2026-09-01  thirdparty_artifact_has_provenance "ausente no validator do Node"
   → npm/src/validator/index.js: 7 sítios, incluindo a implementação completa
```

Eu ia despachar um agente para reimplementar as duas.

### 🔴 Por que ninguém sabia que estavam prontas

Olhe os ACs da REQ-2026-08-20:

```
AC1 — regra implementada no Node, com paridade de comportamento   ✅ ENTREGUE
AC3 — gate comparando as TRÊS SAÍDAS REAIS, não por leitura       ❌ NÃO EXISTE
```

**O código foi entregue; o gate que provaria a entrega, não.** Sem ele, o único jeito de saber se
uma paridade está fechada é ler os três fontes à mão — que é exatamente o que ninguém faz, e o que
o AC3 existia para evitar.

E `scripts/check-rules-parity.sh` **passa verde**. Ele compara o bloco de regras de **artefatos
gerados** (4 arquivos × 3 runtimes), não o conjunto de regras **implementadas** no validator. Mais um
gate cujo nome promete o que ele não mede.

### A medição foi contaminada por um defeito de ferramenta

A leitura original de "ausente no Node" veio de `grep`, que neste ambiente é `ugrep -I` e **omite
silenciosamente** arquivos com NUL byte. `npm/src/validator/index.js` (190 KB, o maior fonte do CLI
Node) usa `\0` como separador de chave composta e é invisível. Detalhe completo em
`vault/notes/grep-do-ambiente-pula-arquivo-com-nul-2026-09-12.md`.

🔴 **Isto agrava o problema em vez de explicá-lo:** significa que a evidência de várias REQs de
paridade abertas pode ser **falsa por construção**, e não temos gate que contradiga.

## Por que NÃO é a mesma causa da REQ de REQ órfã

A Regra Dura de Causa Raiz põe o ônus em quem quer separar. Aplicando o teste prescrito:

> *"Se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma outra."*

**Corrigir o vínculo REQ↔roadmap (a REQ órfã) fecha estas REQs obsoletas? Não.** Uma REQ de paridade
com roadmap perfeitamente vinculado continua indistinguível entre entregue e pendente, porque o que
falta é o **gate de conjunto**, não o elo de governança. Mecanismos diferentes, remédios diferentes.

O que as duas compartilham é o **efeito** — backlog que cresce e não fecha —, não a causa.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Re-triagem medida** de toda REQ aberta cuja evidência seja ausência num runtime.
      Por REQ, o veredito é um de três: **entregue** · **parcial (código sim, gate não)** ·
      **pendente**. Medição por `/usr/bin/grep` (ou `git grep`) **e** execução real dos três
      binários — nunca pelo `grep` do ambiente, e nunca só por leitura de fonte.
- [ ] **AC2** — REQs verificadas como **entregues** vão para `Done` com a evidência escrita no
      próprio artefato: arquivo, linha e a saída que prova.
- [ ] **AC3** — REQs **parciais** têm o AC pendente isolado e nomeado. 🔴 Não fechar REQ parcial:
      fechar com o gate faltando é o que produziu este problema.
- [ ] **AC4** — 🔴 **Gate de conjunto**: um check que enumera as regras **implementadas** em cada
      runtime e reprova quando os três conjuntos divergem. Executa os binários, não lê fonte.
- [ ] **AC5** — Falsificação do AC4 nas duas direções: remover uma regra de um runtime ⇒ gate
      reprova nomeando a regra e o runtime; três conjuntos iguais ⇒ gate passa (contra-braço).
- [ ] **AC6** — 🔴 **Guarda de vacuidade** no gate do AC4: se a enumeração devolver conjunto vazio em
      qualquer runtime, o gate **reprova** em vez de comparar dois vazios e declarar paridade.
- [ ] **AC7** — `docs/cli-parity.md` atualizado, **nomeando o gate**.
- [ ] **AC8** — `make quality` verde **e CI verde**.

## Escopo negativo

- **Não** implementar as paridades que a triagem apontar como genuinamente pendentes — cada uma tem
  sua REQ, e misturar implementação com triagem impede saber qual mudança produziu qual efeito.
- **Não** tocar `validator`/`roadmap`/`branch new` — são da REQ de REQ órfã, em execução paralela.
- **Não** corrigir o `grep` do ambiente: é configuração de shell do usuário, não do produto.

## Linked ADR
ADR: <!-- não requer -->

---

## Absorção do issue #298 (subcomandos sem gate) — 2026-09-12

Decisão do KG: *"absorve o #298 também"*.

### Por que é a mesma causa

O AC4 desta REQ pede um gate que enumere as **regras implementadas** por runtime e reprove na
divergência. O #298 relata exatamente o mesmo buraco, uma superfície ao lado: `check-cli-parity.sh`
compara **comandos de primeiro nível** e **12 subcomandos** (`adr` 3, `req` 4, `roadmap` 5) não são
comparados entre runtimes.

Mesmo mecanismo: **gate de paridade que verifica itens, não o conjunto.** O teste da Regra Dura
fecha — corrigir "o gate não enumera o conjunto" fecha as duas superfícies; nenhuma delas fecha
sozinha pela correção da outra.

### 🔴 Já mordeu — não é hipótese

O próprio comentário do gate atual registra o que motivou escrevê-lo:

> *"`req move` faltou nos três e `req list` faltou no Python sem nenhum gate avisar."*

### Medição do relator, verificada por nós

Ele falsificou removendo `req list` **só do Node**:

```
[defeito plantado]  cmd.command('list') → cmd.command('list-REMOVIDO-PELA-SONDA')
check-cli-parity.sh         exit 0  "CLI parity smoke checks passed"   ← CEGO
check-subcommand-parity.sh  exit 1  "✗ req: 'list' faltando no runtime node"

[controle, árvore intacta]
check-cli-parity.sh         exit 0
check-subcommand-parity.sh  exit 0
```

Confirmado por nós em 2026-09-12: `scripts/check-cli-parity.sh` enumera só o primeiro nível
(lista literal na linha 34, comparação via `--help` da raiz na linha 48). O
`scripts/check-subcommand-parity.sh` que ele cita **não existe neste repositório**.

### 🔴 Limites que o relator declarou, e que os ACs abaixo precisam fechar

Ele foi explícito sobre o que **não** mediu — respeitar isso é obrigação, não cortesia:

1. Falsificou **uma** direção do conjunto — subcomando **faltando**. A direção **sobrando** está
   implementada no gate dele, mas **não provada por sabotagem**.
2. Cobre `adr`, `req` e `roadmap`. Um quarto comando com subcomando **não é descoberto sozinho** —
   a lista é literal. É sítio de manutenção, e isso deve estar escrito na própria entrada.
3. Medido só em Windows/Git Bash. Não mediu Linux nem macOS.

### Quem implementa

🔴 **Nós.** O relator ofereceu abrir PR (*"posso abrir PR com ele, se interessar"*) e **não há PR
aberto**. Registrado como comentário no #298 para que ele não duplique trabalho.

### Critérios acrescentados

- [ ] **AC9** — gate de paridade de **subcomandos**: desce um nível a partir do `--help` de cada
      comando que tenha subcomando e compara os conjuntos **nos dois sentidos** — faltando **e**
      sobrando.
- [ ] **AC10** — 🔴 **falsificar a direção que o relator não falsificou**: plantar subcomando
      **sobrando** num runtime ⇒ gate reprova nomeando comando, runtime e subcomando. Mais o braço
      dele (faltando) e o contra-braço (árvore intacta ⇒ passa).
- [ ] **AC11** — 🔴 **a lista de comandos com subcomando não se descobre sozinha.** Ou o gate a
      deriva do `--help` em vez de hardcodar, ou a entrada literal carrega, escrito, que é sítio de
      manutenção e o que acontece quando um quarto comando aparecer.
- [ ] **AC12** — `known_divergences` no formato `<comando>:<runtime>:<subcomando>:<faltando|sobrando>`,
      **cada entrada exigindo motivo escrito**. Congela o conhecido **sem esconder** — mesmo
      princípio do baseline. Hoje a lista nasce vazia.
- [ ] **AC13** — `docs/cli-parity.md` ganha a tabela de **subcomandos** por runtime, que hoje não
      existe (só há a de primeiro nível).
