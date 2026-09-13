---
status: Open
date: 2026-09-09
author: ""
adr: ""
roadmap: "docs/roadmaps/blocked/ROADMAP-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md"
---

# REQ: REQ nasce orfa porque criar REQ e criar roadmap sao dois comandos e o segundo se esquece

> Date: 2026-09-09 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

**Seis REQs órfãs encontradas em três dias**, todas por acaso, durante outro trabalho. O
`req_has_roadmap` avisa — e o aviso não impediu nenhuma delas.

### Medição 1 — o tamanho e a natureza do passivo

```
warnings de req_has_roadmap          56
REQs Open sem roadmap                32
   das quais abertas em setembro     18
```

Das 32, por classificação **heurística** (não medição):

```
com critérios de aceite, pedem implementação    ~14
aparentam decisão/registro puro                 ~18
```

🔴 **Declaro o limite do classificador:** ele casa palavras-chave (*"decisão"*, *"ADR própria"*) e
testa esse ramo **primeiro** — então REQ que tenha as duas coisas cai em "decisão". O número real de
"pedem implementação" é **≥ 14**, provavelmente maior. Não use estes números como escopo sem
re-triagem.

**O que a medição sustenta com segurança:** **18 das 32 nasceram em setembro** — o passivo não é
histórico, está sendo produzido **agora**, no ritmo atual, com o aviso ativo.

### Medição 2 — 🔴 a ferramenta já existe e o fluxo não a usa

```
trackfw roadmap new --from-req <REQ>
  "Generate roadmap with ML stubs from REQ acceptance criteria"

trackfw req new
  (nenhuma opção de criar o roadmap junto)
```

**A capacidade existe. O caminho inverso não.** Quem cria a REQ precisa lembrar de um segundo comando
— e é esse segundo comando que se esquece.

🔴 **Mesmo padrão do achado A1 desta campanha:** o `pathIsAnchoredForHookConfig` existia e o sítio não
o usava; o `--from-req` existe e o fluxo não o usa. **O defeito não é ausência de capacidade, é
ausência de ligação.**

## Por que a inversão direta da severidade NÃO funciona

Virar `req_has_roadmap` para `error` hoje faz o `validate` falhar em **32 REQs**. Consequência
previsível: alguém configura `lenient` ou baixa a severidade — e aí perdemos **a regra e o aviso**.

É o que já acontece com o `windows-full-suites`: job que nasce vermelho vira ruído que se aprende a
ignorar. E é a crítica que o autor do issue #275 fez sobre contagem.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Prevenção antes de gate:** criar REQ e roadmap deixa de exigir dois comandos.
      Forma a decidir (flag em `req new`, prompt, ou `req new` chamando `--from-req`) — **medir o
      atrito de cada uma**, não escolher por gosto.
- [ ] **AC2** — severidade endurecida **por data de corte**, não por contagem: REQ criada a partir de
      `<corte>` sem roadmap ⇒ **error**; anterior ⇒ **warning**, grandfathered. Corte declarado no
      artefato.
- [ ] **AC3** — 🔴 **O grandfathering é visível, não silencioso.** O relatório diz quantas REQs estão
      isentas e desde quando. Isenção que não se vê vira permanente.
- [ ] **AC4** — decisão escrita sobre **onde bloqueia**: só `validate`, ou também `push`. 🔴 O `push`
      hoje exige REQ+roadmap **da branch**, não de toda REQ — são coisas diferentes e a decisão precisa
      dizer qual muda.
- [ ] **AC5** — re-triagem das 32: quantas **legitimamente** não têm roadmap (decisão pura, fechada sem
      implementação). O classificador heurístico acima **não serve** como escopo.
- [ ] **AC6** — paridade nos 3 CLIs.

## Negative Scope

- **Não** criar roadmap automático para as 32 existentes — roadmap vazio gerado em massa é pior que
  REQ órfã: parece cobertura e não é.
- **Não** endurecer `req_has_adr` no mesmo movimento — são **103** avisos, causa distinta, e existe
  ADR de ratchet própria para o acervo.
- **Não** alterar o gate de branch do `push` sem a decisão do AC4.


## 🔴 Medição 3 — o diagnóstico acima está INCOMPLETO, e eu descobri escrevendo este roadmap

Criei o roadmap desta REQ **usando o `--from-req`**, justamente a ferramenta que a Medição 2 diz
existir e não ser usada. Resultado:

```
✓  6 ACs  →  6 MLs (ML-1A..1F), um por critério, com o texto do AC
✓  + ML-0A de threat model
✗  bloco "Acceptance Criteria" do roadmap: VAZIO
✗  REQ → roadmap: continua  roadmap: ""     ← A REQ NASCE ÓRFÃ MESMO ASSIM
```

🔴 **Mesmo pelo caminho correto, a REQ permanece órfã.** O vínculo é de **mão única**: o roadmap
aponta para a REQ no frontmatter; a REQ **não** aponta de volta.

**Isto reescreve o alvo desta REQ.** O título diz *"criar REQ e criar roadmap são dois comandos e o
segundo se esquece"* — verdadeiro, mas insuficiente. O defeito real é:

> **nem o comando integrado fecha o laço.** Quem faz tudo certo — cria a REQ e roda `--from-req` —
> **ainda produz uma REQ órfã**, e o `req_has_roadmap` a acusa.

Isso explica por que o aviso não corrige o comportamento: **ele acusa quem seguiu o processo.** Aviso
que dispara sobre o caminho correto ensina a ignorar o aviso.

**Consequência para o AC1:** prevenção não é só "um comando em vez de dois" — é **o vínculo
bidirecional ser escrito por quem cria**. E o `roadmap move` já faz isso (`✓ synced REQ ... → roadmap`),
então **a capacidade existe num comando e falta no outro**. Terceira ocorrência do mesmo padrão nesta
REQ.

### AC7 — o `--from-req` escreve o vínculo de volta na REQ
- [ ] `roadmap new --from-req` grava `roadmap:` no frontmatter da REQ, como o `roadmap move` já faz
- [ ] o bloco consolidado de "Acceptance Criteria" do roadmap deixa de sair vazio quando a REQ tem ACs
- [ ] falsificação: criar REQ+roadmap pelo caminho integrado ⇒ `req_has_roadmap` **não** dispara

---

## Evidência acumulada em 2026-09-12 — a decisão do KG de priorizar esta REQ

> *"coloque a de req nasce órfã antes das demais, assim já fechamos o buraco que cada vez mais estamos
> cavando"*

### O número, medido hoje

```
37 REQs abertas
 4 em PR aberto
 3 com roadmap ativo
34 SEM roadmap nenhum      ← 92%
```

🔴 **O backlog não é fila priorizada — é depósito.** E a auditoria independente do Codex, em
2026-09-11, devolveu **5 achados, e os 5 já tinham REQ nossa**, escrita, verificada, com arquivo e
linha. Paradas. **O gargalo não é detecção; é fechamento** — e a raiz do fechamento que não acontece é
o trabalho nascer sem plano.

### Três defeitos NOVOS do mesmo mecanismo, medidos hoje

Todos são faces de **"o vínculo REQ↔roadmap é manual e frágil"**:

**1. `roadmap move ""` casa um roadmap ARBITRÁRIO e o move.**
```
$ trackfw roadmap move "" wip
✓ synced REQ-2026-08-31-guarda-de-folha-... → docs/roadmaps/wip/ROADMAP-2026-08-31-...
```
Aconteceu comigo: uma variável vazia por erro de shell, e o comando **adivinhou** em vez de recusar.
🔴 É a decisão do KG de 2026-08-29 (*"rejeita e avisa, em vez de adivinhar"*) violada **pelo próprio
comando que governa o ciclo**.

**2. `roadmap new --from-req` cria o roadmap e NÃO escreve o vínculo de volta na REQ.**
O elo tem de ser feito à mão depois — e é exatamente o passo que "o segundo comando se esquece",
como o título desta REQ diz.

**3. Duas noções de "vinculada" que discordam.**
O `validate` lê o **marcador no corpo**; o frontmatter `roadmap:` é outro campo. Contei **27** REQs
órfãs pelo frontmatter enquanto o produto enxergava **57**. 🔴 Duas fontes de verdade para o mesmo
vínculo, e nenhuma delas é autoritativa.

**4. E o vínculo não sobrevive a rebase.** Hoje, ao rebasear os PRs #331 e #332, o roadmap ficou
apontando para uma REQ que não existia mais naquela branch — porque REQ e roadmap foram commitados em
branches diferentes. Tive que restaurar os dois arquivos e refazer o elo à mão.

### Critérios acrescentados

- [ ] **AC7** — `roadmap move` com nome **vazio** ou que não casa exatamente **recusa e nomeia**, nos 3
      CLIs. 🔴 Nunca escolher um roadmap por proximidade. Falsificação: nome vazio ⇒ erro; nome exato ⇒
      move.
- [ ] **AC8** — `roadmap new --from-req` escreve o vínculo **de volta na REQ**, no formato que o
      `validate` de fato lê. Uma operação, dois lados do elo.
- [ ] **AC9** — 🔴 **uma** noção de "vinculada". Ou o corpo é autoritativo e o frontmatter é derivado,
      ou o inverso — **escolher e escrever**. Enquanto houver duas, toda contagem de órfãs é uma
      opinião. Gate que prove que as duas concordam, ou que só uma existe.
- [ ] **AC10** — 🔴 **contra-braço do AC1:** criar REQ **sem** roadmap continua possível quando é
      deliberado (decisão pura, REQ fechada sem implementação). O remédio não pode ser proibir — o AC5
      já reconhece que parte das 34 é legítima. **Atrito onde é engano, caminho livre onde é intenção.**

---

## Absorção da `REQ-2026-08-20` (casamento por substring) — 2026-09-12

Decisão do KG: *"absorve como ML na órfã"*. A `REQ-2026-08-20-branch-has-wip-roadmap-casa-por-substring-num-corpus-de-done-que-so-cresce`
passa a `Superseded` e seu conteúdo entra aqui.

### Por que é a mesma causa, medido no código

Dois sítios, duas implementações, **um mecanismo**: substring, primeiro-que-casar-vence, sem exigir
casamento exato.

```go
// internal/generators/roadmap.go:632 — consumido por `roadmap move`
if containsIgnoreCase(e.Name(), name) { return primeiro_match }

// internal/validator/validator.go:2870 — consumido por validate, branch new, commit
if strings.Contains(normalizeBranchSlug(name), branchSlug) { matched = true }
```

🔴 **`strings.Contains(x, "")` é sempre verdadeiro** — é por isso que o `roadmap move ""` do AC7 moveu
um roadmap arbitrário. O bug do nome vazio **é** a fraqueza de substring, no outro sítio.

### 🔴 Medição que FALSIFICA o candidato preferido da REQ absorvida

A REQ-2026-08-20 propunha três candidatos e dizia do primeiro (casamento por fronteira) que era
*"mais estrito, e provavelmente suficiente"*. **Não é.** Medido contra o corpus real — 185 roadmaps
em `done/`+`wip/` e as 111 branches `feat|fix|refactor` de PRs mergeados:

| | substring | fronteira |
|---|---|---|
| casamentos das 111 branches históricas | 109 | **109** |
| branches que perderiam casamento (regressão) | — | **0** |
| casamentos de 20 slugs curtos genéricos | 326 | 266 (−18%) |

Ou seja: **fronteira não regride nada e também não resolve nada** para o uso real, porque os slugs
históricos são longos e descritivos. Onde deveria resolver, não resolve:

```
fix/roadmap    substring 159   fronteira 159    ← 86% do corpus, inalterado
fix/req                 17              16
fix/guard               16              14
fix/gate                19              15
fix/ci                  28               2      ← só aqui funciona
```

**Por que falha:** quando o token curto é uma palavra *legítima* do nome do roadmap, a fronteira o
encontra igual. Fronteira só remove o caso de palavra-dentro-de-palavra (`ci` em `precisao`), que é a
minoria.

### O que a medição sustenta

🔴 **Nenhum dos três candidatos da REQ absorvida resolve o problema, porque os três continuam
INFERINDO o vínculo a partir do nome.** O remédio é o mesmo que esta REQ já defende no AC8:

> **o vínculo branch↔roadmap deve ser ESCRITO por quem cria a branch, não adivinhado por
> comparação de string depois.**

O `trackfw branch new` sabe qual roadmap está em `wip` no momento em que cria a branch. Esse é o
instante em que o elo existe sem ambiguidade — e é o instante em que ele não é gravado.

**Isto é a quarta ocorrência do padrão desta REQ:** a capacidade existe num comando e falta no outro.

### Critérios acrescentados

- [ ] **AC11** — 🔴 **ADR obrigatório** (herdado do AC1 da REQ absorvida): decisão registrada sobre
      precisão do vínculo branch↔roadmap, com os candidatos descartados **e a medição acima**, que
      falsifica o candidato 1. A ADR precisa dizer se o vínculo passa a ser escrito, inferido com
      regra mais estrita, ou os dois.
      > Nota: `adr_accepted_when_req_done` **não** exige ADR (`adrRef == "" → continue`); só valida
      > ADR já linkado. Esta obrigação é da REQ, não do gate — registrar para não evaporar.
- [ ] **AC12** — `findRoadmap` (`roadmap move`) e `BranchSlugMatchesRoadmap` (`validate`/`branch
      new`/`commit`) param de aceitar nome vazio e param de escolher por proximidade, **nos 3 CLIs**.
      🔴 Ordem obrigatória: o caso do nome vazio primeiro — ele é estritamente aditivo e não tem
      consumidor legítimo.
- [ ] **AC13** — retomada legítima de roadmap concluído **continua funcionando**, provada por cenário
      (AC3 da REQ absorvida). 🔴 Risco dominante herdado: este portão é atravessado por **todo**
      `branch new`, `commit` e `ship`; falso-positivo aqui **paralisa**, não irrita.
- [ ] **AC14** — a medição dos 185 roadmaps vira **gate**, não nota de rodapé: mudança no matcher que
      altere o veredito de qualquer das 111 branches históricas reprova.

### Risco de execução registrado

🔴 **Apertar `BranchSlugMatchesRoadmap` durante a sessão quebra qualquer frente paralela**, porque
`branch new` e `commit` de outras frentes atravessam o mesmo binário. Frente paralela deve fixar o
binário instalado, não reconstruir a partir da árvore desta REQ.
