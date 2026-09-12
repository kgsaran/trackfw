---
status: backlog
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
**Status:** ⬜ Pendente
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
- [ ] Veredito sobre onde a régua cai, com os quatro casos concretos respondidos
- [ ] Veredito sobre detectabilidade hoje, com evidência
- [ ] O risco de "REQ decorativa" endereçado
- [ ] Nenhuma linha de `CONTRIBUTING.md` escrita
- [ ] Parecer em `docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md`

**Gates da wave:**
```bash
test -f docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
! grep -qi "placeholder" docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
grep -q "Residual" docs/qualidade/2026-09-01-custo-de-adocao-da-regra-de-governanca.md
```

## Wave 1 — Escrever o documento
> Dependências: Wave 0. `atena-tf` — é documento de **entrada**, e o público é quem nunca viu o
> projeto. Clareza aqui vale mais que completude.

## Verificação

Não há CI que feche isto. A verificação é a **próxima contribuição externa chegar no formato certo**
— foi assim que soubemos que o contrato de gates funcionou: o terceiro gate nasceu ligado.

## Barreira final

`hefesto-tf` e o arquiteto. **Sem `hades-tf`** — não há superfície de ataque nesta mudança, e
convocá-lo por reflexo diluiria o sinal das barreiras onde ele importa.
