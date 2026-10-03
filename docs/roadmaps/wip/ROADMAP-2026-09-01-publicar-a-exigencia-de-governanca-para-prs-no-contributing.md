---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing.md"
squad: "atena-tf, hefesto-tf"
---

# Roadmap: Publicar a exigência de governança para PRs no `CONTRIBUTING`

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-projeto-nao-publica-a-exigencia-de-governanca-para-prs-e-nao-tem-contributing.md`

**O trackfw é um framework de governança cujo repositório não publica a regra que ele existe para
vender.** Não há `CONTRIBUTING.md`, nem template de PR, e em lugar nenhum está escrito que um PR
precisa de REQ e roadmap. O `README` descreve a cadeia como *o que o produto faz*; o `CLAUDE.md` é o
harness dos agentes.

Isto veio à tona ao pedir governança a um contribuidor externo (PRs #238 e #240) **por uma regra
nunca publicada** — o mesmo padrão que já custou dois microlotes corretivos com os gates dele.

**Este roadmap é escrito sob a cadeia que ele documenta.** Um guia de governança criado fora da
governança se refutaria sozinho.

## Acceptance Criteria

- [ ] `CONTRIBUTING.md` com a cadeia, os comandos e **quando** cada um é exigido
- [ ] A **exceção de trivialidade** publicada junto — sem ela a regra é arbitrária
- [ ] O **contrato de gates** movido para onde o contribuidor olha
- [ ] Template de PR com REQ, roadmap e **falsificação nas duas direções**
- [ ] A regra é **falsificável** — PR sem REQ é detectável
- [ ] O documento declara que a regra vale **para os mantenedores também**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — O que a regra pode quebrar
> Dependências: nenhuma. Bloqueia a escrita.

### ML-0A — Custo de adoção e falsificabilidade da regra
**Status:** ✅ Concluído
**Agente:** `hefesto-tf`
**Files affected:** nenhum (documento em `docs/qualidade/`)
**Por que esta Wave 0 não é sobre segurança:** o risco aqui é **social e de processo**, não de
ataque. Uma regra publicada mal calibrada afasta contribuição boa — e o projeto acabou de receber
quatro PRs de alta qualidade de fora.
**Actions:**
1. 🔴 **Onde a régua deve cair.** A exceção de trivialidade interna (`~/.claude/CLAUDE.md` §7) é a
   base, mas foi escrita para **mantenedores com contexto**. Julgue se ela serve como está para
   quem chega de fora, ou se a fronteira precisa ser redesenhada. Casos concretos para testar a
   régua: correção de uma linha num gate; correção de typo em mensagem de erro **visível ao
   usuário**; port de um teste; correção de documentação que **descreve comportamento errado**.
2. **Falsificabilidade da regra (AC5).** Um PR sem REQ ligada é detectável hoje? Com `trackfw
   validate`, com o job `governance`, ou com nada? **Se a resposta for "com nada", isso é o achado**
   — estaríamos publicando um contrato sem gate, exatamente o que este projeto acabou de criticar
   por escrito no `docs/cli-parity.md`.
3. 🔴 **Falsificação nas duas direções.** Direta: a regra publicada não é seguida e ninguém percebe.
   **Simétrica, e é a que me preocupa:** a regra é seguida ao pé da letra e produz **REQ vazia de
   conteúdo** — artefato criado para satisfazer o gate, sem critério de aceite real. Isso é pior que
   ausência de REQ, porque **parece rastreabilidade**. Nomeie como distinguir.
4. **Residual declarado.**
**Critérios de aceite:**
- [x] Veredito sobre onde a régua cai, com os quatro casos concretos respondidos
- [x] Veredito sobre detectabilidade hoje, com evidência
- [x] O risco de "REQ decorativa" endereçado
- [x] Nenhuma linha de `CONTRIBUTING.md` escrita
- [x] Parecer em `docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md`

      ✅ Parecer existente: `docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md` (régua dos 4 casos, detectabilidade parcial, risco de REQ decorativa, resíduo). Auditado em 2026-10-03.
**Gates da wave:**
```bash
test -f docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
! grep -qi "placeholder" docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
grep -q "Residual" docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
```

## Wave 1 — Completar o documento (reabertura de 2026-10-03)
> Dependências: Wave 0. `atena-tf`: documento de **entrada**, para quem nunca viu o projeto. Clareza vale
> mais que completude.

### ML-1A — O que falta no `CONTRIBUTING.md`, no template de PR e no `README`
**Status:** ⬜ Pendente
**Squad:** atena-tf
**Files affected:** `CONTRIBUTING.md`, `.github/PULL_REQUEST_TEMPLATE.md`, `README.md` (só o apontamento)
**Actions** (cada item é um AC da REQ; o texto atual tem 154 linhas, e **não** se reescreve o que já está certo):
1. **AC2 (completar):** junto da lista "Dispensam REQ+roadmap", a orientação do parecer da Wave 0 (§1):
   *tamanho do diff não decide trivialidade; a pergunta é se o arquivo participa de uma decisão de pass/fail
   em algum gate, teste ou CI*, com `scripts/`, `.github/workflows/` e os geradores de gate como exemplos de
   "nunca trivial".
2. **AC3:** o **contrato de gates**: um gate novo tem de estar **ligado** (`Makefile`/CI) e **reprovar quando
   não mede nada**, com a anotação `trackfw-contract` no `docs/cli-parity.md` quando descrever contrato.
3. **AC4:** o template de PR ganha os campos REQ ligada, roadmap ligado e **"Falsificação nas duas
   direções"** (incluindo o controle). O comentário em inglês da linha `Closes #` fica **intacto**.
4. **AC6:** o `README.md` aponta para o `CONTRIBUTING.md` de forma visível (uma linha, perto do topo ou da
   seção de contribuição, se existir).
5. **AC7:** uma frase dizendo que a regra **vale para os mantenedores também**, e que este documento foi
   escrito sob a cadeia (REQ-2026-09-01).
6. **AC8:** uma seção curta, por exemplo "Antes de implementar uma issue", com:
   (a) o que é a label `req-aberta` e que issue com ela **ou** em backlog pede uma **Discussion** antes do
   código, e por quê (dois trabalhos paralelos sobre a mesma causa);
   (b) **gate vermelho não mergeia**, mesmo correto: a `main` exige os status checks, e quem abre o PR
   acompanha o CI até ficar verde;
   (c) PR que colide com trabalho em andamento é **fechado**, com o motivo e com crédito pelo que trouxe.
   Tom: o mesmo do documento. Fatos, sem culpar quem contribuiu: a falha de não ter avisado foi nossa.
**Acceptance criteria:**
- [ ] AC2 (completar), AC3, AC4, AC6, AC7 e AC8 da REQ presentes, cada um apontável por linha
- [ ] Nenhuma seção existente perdeu conteúdo (diff mostra só acréscimos, salvo ajuste justificado)
- [ ] `scripts/check-pr-closing-keyword.sh` continua verde com o template novo
**Gates da wave:**
```bash
test -f CONTRIBUTING.md
grep -q "req-aberta" CONTRIBUTING.md
grep -q "CONTRIBUTING" README.md
```

## Wave 2 — Barreira
> Dependências: Wave 1 auditada

### ML-2A — Revisão de clareza e coerência
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-03-revisao-contributing.md`
**Actions:** confrontar cada regra escrita com o que o repositório **de fato** faz (a `main` exige quais checks?
a label existe? os comandos citados existem?). Regra publicada que o repositório não cumpre é pior que regra
ausente.
**Acceptance criteria:**
- [ ] Cada afirmação factual do texto novo conferida contra o repositório
- [ ] Veredito explícito

**Gates da wave:**
```bash
test -s docs/qualidade/2026-10-03-revisao-contributing.md
```

## Verificação

Não há CI que feche isto. A verificação é a **próxima contribuição externa chegar no formato certo**
— foi assim que soubemos que o contrato de gates funcionou: o terceiro gate nasceu ligado.

## Barreira final

`hefesto-tf` e o arquiteto. **Sem `hades-tf`** — não há superfície de ataque nesta mudança, e
convocá-lo por reflexo diluiria o sinal das barreiras onde ele importa.
