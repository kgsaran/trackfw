---
status: wip
date: 2026-09-09
req: "docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md"
squad: ""
---

# Roadmap: REQ nasce orfa porque criar REQ e criar roadmap sao dois comandos e o segundo se esquece

> Created: 2026-09-09 | Reestruturado: 2026-09-12 (absorção da REQ-2026-08-20) | Status: wip

## Context
REQ: docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md

**A tese única deste roadmap:** o vínculo entre artefatos de governança é hoje **inferido** — por
comparação de string, por proximidade de nome, por leitura de um campo entre dois que discordam. Todo
defeito abaixo é uma face disso. A correção é **escrever o vínculo no instante em que ele existe sem
ambiguidade**, e recusar quando não existe.

> 🔴 O bloco "Acceptance Criteria" abaixo estava **vazio** (`- [ ]`, `- [ ]`) porque foi gerado por
> `roadmap new --from-req`, que não consolida os ACs da REQ. **Esse é o AC7 deste próprio roadmap.**
> Preenchido à mão em 2026-09-12; quando o AC7 fechar, deixa de precisar.

## Acceptance Criteria
- [ ] AC1 — criar REQ e roadmap deixa de exigir dois comandos (atrito medido, não escolhido por gosto)
- [ ] AC2 — severidade endurecida por **data de corte**, não por contagem
- [ ] AC3 — grandfathering **visível** no relatório
- [ ] AC4 — decisão escrita sobre onde bloqueia (`validate` e/ou `push`)
- [ ] AC5 — re-triagem das REQs sem roadmap (o classificador heurístico não serve como escopo)
- [ ] AC6 — paridade nos 3 CLIs
- [ ] AC7 — `roadmap new --from-req` escreve o vínculo de volta na REQ e consolida os ACs
- [ ] AC8 — `roadmap move` com nome vazio ou não-exato **recusa e nomeia**
- [ ] AC9 — **uma** noção de "vinculada": frontmatter ou corpo, escolher e escrever
- [ ] AC10 — contra-braço: criar REQ **sem** roadmap continua possível quando é deliberado
- [ ] AC11 — **ADR** da precisão do vínculo branch↔roadmap, com a medição que falsifica o candidato 1
- [ ] AC12 — `findRoadmap` e `BranchSlugMatchesRoadmap` param de aceitar vazio e de escolher por proximidade
- [ ] AC13 — retomada legítima de roadmap concluído continua funcionando
- [ ] AC14 — a medição das 111 branches históricas vira **gate**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat model
> Dependências: nenhuma. **Bloqueia toda implementação.**

### ML-0A — Threat model deste roadmap
**Status:** ⬜ Pendente
**Arquivos afetados:** somente este roadmap.
**Ações:**
1. **Completude da enumeração** — buscar no repositório **todo** sítio que infere vínculo entre
   artefatos por comparação de nome. Já conhecidos: `internal/generators/roadmap.go:632`
   (`findRoadmap`), `internal/validator/validator.go:2870` (`BranchSlugMatchesRoadmap`), e os
   espelhos em `npm/src/` e `pypi/trackfw/`. 🔴 **Não parar nesses três** — grep por
   `Contains`/`includes`/`in ` sobre nomes de arquivo de governança, nos 3 runtimes.
2. **Threat model** — quem esvazia esta Wave sem quebrar regra escrita?
3. **Falsificação nas duas direções**, por sítio: o que quebra quando o comportamento regride, e o
   que quebra quando regride ao contrário (aperta demais).
4. **Residual declarado.**
**Critérios de aceite:**
- [ ] As quatro seções respondidas com evidência, não asserção de uma linha
- [ ] Nenhuma linha de implementação escrita neste ML
**Gate da wave:** substituir este placeholder por um check específico antes de fechar o ML-0A.
```bash
exit 1  # placeholder falha fechado até o ML-0A o substituir
```

---

## Wave 1 — A fonte de verdade do vínculo
> Dependências: Wave 0. 🔴 **Precede tudo**: enquanto houver duas noções de "vinculada", toda
> contagem e todo gate deste roadmap medem coisas diferentes.

### ML-1A — **AC9** — uma noção de "vinculada"
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/validator/validator.go`, `npm/src/validator/index.js`,
`pypi/trackfw/validator.py`, e todo comando que **escreve** REQ (`req new`, `roadmap new --from-req`,
`roadmap move`) nos 3 runtimes.
**Contexto medido:** `roadmap: ""` no frontmatter → 27 REQs; `"no linked Roadmap"` no `validate` →
57. Duas contagens, duas fontes. Vincular 4 REQs pelo frontmatter **não** as tirou da lista do
`validate`. Mesma causa do issue **#306** (`req list` lê `status:` do corpo, `validate` lê o
frontmatter).
**Ações:**
1. **Decidir e registrar** qual é a fonte de verdade. Presunção a confirmar: frontmatter, por
   consistência com `status:` e com o `serve`. 🔴 **Decisão, não inferência** — escrever o porquê.
2. Decidir o que fazer quando os dois divergem: silêncio, aviso ou violação. **Corrigir a leitura sem
   decidir isto troca um defeito por outro.**
3. Todo comando que escreve REQ escreve **os dois**, ou o gerador para de emitir o marcador de corpo
   como placeholder vazio — que é o que cria a órfã silenciosa.
**Critérios de aceite:**
- [ ] Fonte de verdade escrita no artefato, com o motivo
- [ ] Frontmatter preenchido + corpo vazio ⇒ comportamento decidido em (2)
- [ ] Os dois preenchidos e **divergentes** ⇒ idem
- [ ] Os dois iguais ⇒ passa (contra-braço)
- [ ] Paridade nos 3 CLIs, com diff de saída real
**Reconciliação:** cada teste novo declara, em uma frase, qual conclusão deste ML ele afirma.

### ML-1B — **AC7** — `--from-req` fecha o laço
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py`.
**Contexto medido:** `roadmap new --from-req` gera os MLs a partir dos ACs, mas **não** grava
`roadmap:` na REQ e deixa o bloco "Acceptance Criteria" do roadmap **vazio**. O `roadmap move` já faz
o sync (`✓ synced REQ ... → roadmap`) — 🔴 **a capacidade existe num comando e falta no outro.**
**Ações:**
1. `--from-req` grava o vínculo de volta na REQ, no formato que o `validate` de fato lê (decidido no
   ML-1A). Uma operação, dois lados do elo.
2. O bloco consolidado de ACs do roadmap deixa de sair vazio quando a REQ tem ACs.
**Critérios de aceite:**
- [ ] Criar REQ + roadmap pelo caminho integrado ⇒ `req_has_roadmap` **não** dispara (falsificação)
- [ ] Bloco de ACs do roadmap reflete os ACs da REQ
- [ ] Paridade nos 3 CLIs

### ML-1C — **AC8 + AC12 (parte 1 de 2)** — recusar o nome vazio
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/roadmap.go` (`findRoadmap`, linha ~632),
`npm/src/generators/roadmap.js`, `pypi/trackfw/generators/roadmap.py`.
🔴 **Este ML vem ANTES do ML-3A por decisão de ordem:** recusar nome vazio é **estritamente aditivo**
— não existe consumidor legítimo de `roadmap move ""`. Apertar o matcher compartilhado (ML-3A) não é
aditivo e pode paralisar quem depende dele.
**Contexto medido:** `containsIgnoreCase(nome, "")` é sempre verdadeiro ⇒ `roadmap move ""` retorna o
**primeiro** roadmap do primeiro diretório de estado e o move. Aconteceu de verdade em 2026-09-12,
por variável de shell vazia.
**Ações:**
1. Nome vazio ⇒ **erro que nomeia o problema**, nos 3 CLIs.
2. Nome que não casa exatamente ⇒ recusar e **listar os candidatos**, em vez de escolher um.
   🔴 Decisão do KG de 2026-08-29: *"controle que não reconhece rejeita e avisa, em vez de adivinhar"*.
**Critérios de aceite:**
- [ ] Nome vazio ⇒ erro, nenhum arquivo movido (falsificação)
- [ ] Nome exato ⇒ move (contra-braço)
- [ ] Nome parcial ambíguo ⇒ recusa nomeando os candidatos
- [ ] Paridade nos 3 CLIs, mensagens byte-idênticas

---

## Wave 2 — A decisão arquitetural
> Dependências: Wave 1 (a fonte de verdade precisa estar decidida).

### ML-2A — **AC11** — ADR da precisão do vínculo branch↔roadmap
**Status:** ⬜ Pendente
**Arquivos afetados:** novo ADR em `docs/adr/`.
**Insumo obrigatório — a medição já feita em 2026-09-12, contra 185 roadmaps e 111 branches reais:**

| | substring | fronteira |
|---|---|---|
| casamentos das 111 branches históricas | 109 | **109** |
| regressão | — | **0** |
| 20 slugs curtos genéricos | 326 | 266 (−18%) |

```
fix/roadmap   substring 159   fronteira 159   ← 86% do corpus, INALTERADO
fix/guard              16              14
fix/ci                 28               2     ← só aqui funciona
```

🔴 **A medição falsifica o candidato 1 da REQ absorvida**, que o texto dela chamava de
*"provavelmente suficiente"*. Fronteira só remove palavra-dentro-de-palavra; quando o token curto é
uma palavra legítima do nome, ela não ajuda.
**Ações:**
1. Registrar a decisão **com os candidatos descartados e o porquê**, citando a medição.
2. Responder: o vínculo branch↔roadmap passa a ser **escrito** (o `branch new` sabe qual roadmap está
   em `wip` no instante em que cria a branch), inferido com regra mais estrita, ou os dois?
3. 🔴 **Risco dominante herdado da REQ absorvida:** este portão é atravessado por **todo** `branch
   new`, `commit` e `ship`. Falso-positivo aqui **paralisa**, não irrita. A ADR declara como o risco é
   contido.
**Critérios de aceite:**
- [ ] ADR escrita, com candidatos descartados e a medição citada
- [ ] Decisão explícita sobre escrever vs. inferir
- [ ] Contenção do risco de falso-positivo declarada

---

## Wave 3 — O matcher compartilhado
> Dependências: **ML-2A** (não mexer no matcher antes da ADR). 🔴 Enquanto esta wave estiver aberta,
> frente paralela deve usar o `trackfw` **instalado**, não `bin/trackfw` reconstruído desta árvore.

### ML-3A — **AC12 (parte 2 de 2)** — `BranchSlugMatchesRoadmap`
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/validator/validator.go:2864`, espelhos em `npm/src/validator/`,
`pypi/trackfw/validator.py`. Consumidores: `validate`, `branch new` (`commands/branch.go:100`),
`commit` (`commands/commit.go:103`).
**Ações:** implementar o que a ADR do ML-2A decidiu.
**Critérios de aceite:**
- [ ] Comportamento decidido na ADR, nos 3 CLIs
- [ ] 🔴 O gate do ML-2A da REQ antiga fixa o comportamento **atual** — atualizar junto, **nunca
      afrouxar para caber**

### ML-3B — **AC13** — retomada legítima continua funcionando
**Status:** ⬜ Pendente
**Ações:** cenário provando que retomar trabalho de um roadmap em `done/` continua permitido.
**Critérios de aceite:**
- [ ] Cenário de retomada legítima passa (contra-braço do ML-3A)
- [ ] Cenário de slug curto espúrio reprova (braço)

### ML-3C — **AC14** — a medição vira gate
**Status:** ⬜ Pendente
**Arquivos afetados:** novo `scripts/check-roadmap-slug-matching.sh`, wired no `Makefile`.
**Ações:** o corpus de 185 roadmaps e as 111 branches históricas viram fixture. Mudança no matcher que
altere o veredito de qualquer das 111 reprova.
**Critérios de aceite:**
- [ ] Gate roda nos 3 runtimes e compara saídas reais
- [ ] Falsificação: mutação no matcher ⇒ gate reprova
- [ ] Contra-braço: matcher correto ⇒ gate passa

---

## Wave 4 — Prevenção e severidade
> Dependências: Wave 1. Independente da Wave 3.

### ML-4A — **AC1 + AC10** — um comando, com contra-braço
**Status:** ⬜ Pendente
**Ações:** criar REQ e roadmap deixa de exigir dois comandos. 🔴 **Medir o atrito de cada forma**
(flag em `req new`, prompt, `req new` chamando `--from-req`) — não escolher por gosto. **E** criar
REQ sem roadmap continua possível quando é deliberado: *atrito onde é engano, caminho livre onde é
intenção.*
**Critérios de aceite:**
- [ ] Caminho integrado produz REQ **não órfã** (falsificação)
- [ ] Caminho deliberado sem roadmap continua disponível (contra-braço)
- [ ] Paridade nos 3 CLIs

### ML-4B — **AC2 + AC3** — corte por data, grandfathering visível
**Status:** ⬜ Pendente
**Ações:** REQ criada a partir de `<corte>` sem roadmap ⇒ **error**; anterior ⇒ warning. Corte
declarado no artefato. O relatório diz **quantas** estão isentas e **desde quando**.
🔴 *Isenção que não se vê vira permanente.* E inverter a severidade sem corte faz o `validate` falhar
em 32 REQs — alguém configura `lenient` e perdemos a regra **e** o aviso.
**Critérios de aceite:**
- [ ] REQ pós-corte sem roadmap ⇒ error; pré-corte ⇒ warning (os dois braços)
- [ ] Relatório mostra contagem de isentas e a data de corte
- [ ] Paridade nos 3 CLIs

### ML-4C — **AC4** — onde bloqueia
**Status:** ⬜ Pendente
**Ações:** decisão escrita: só `validate`, ou também `push`. 🔴 O `push` hoje exige REQ+roadmap **da
branch**, não de toda REQ — são coisas diferentes e a decisão precisa dizer **qual** muda.
**Critérios de aceite:**
- [ ] Decisão escrita, com o impacto de cada opção

---

## Wave 5 — Fechamento
> Dependências: Waves 1–4.

### ML-5A — **AC5** — re-triagem das REQs sem roadmap
**Status:** ⬜ Pendente
**Ações:** quantas **legitimamente** não têm roadmap (decisão pura, fechada sem implementação).
🔴 O classificador heurístico da REQ **não serve** como escopo: ele casa palavras-chave e testa esse
ramo primeiro, então REQ que tenha as duas coisas cai em "decisão".
**Critérios de aceite:**
- [ ] Contagem por leitura, não por heurística, com a lista

### ML-5B — **AC6** — paridade e fechamento
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [ ] `make quality` verde **e CI verde**
- [ ] `trackfw validate` sem violation nova

---

## Escopo negativo

- **Não** criar roadmap automático para as REQs órfãs existentes — roadmap vazio gerado em massa é
  pior que REQ órfã: **parece cobertura e não é.**
- **Não** endurecer `req_has_adr` no mesmo movimento — 103 avisos, causa distinta, ADR de ratchet
  própria para o acervo.
- **Não** remover a aceitação de `done/` no `branch_has_wip_roadmap`. Ela existe por um motivo
  (retomar trabalho concluído) e a `REQ-2026-07-26` a decidiu. Este roadmap ajusta a **precisão**, não
  a política.
