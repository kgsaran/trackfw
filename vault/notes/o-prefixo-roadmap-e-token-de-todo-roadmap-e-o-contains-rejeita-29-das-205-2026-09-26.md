---
title: O prefixo `ROADMAP-` é token de TODO roadmap — e o `Contains` de hoje rejeita 29 das 205 branches governadas
date: 2026-09-26
ml: ML-3A + ML-3B (REQ-2026-09-09) · ADR-2026-09-26 · fecha o #273
---

# O prefixo `ROADMAP-` é token de todo roadmap — e o `Contains` rejeita 29 das 205

Medido em 2026-09-26, sobre o acervo real deste repositório: **201 roadmaps** em `wip/`+`done/`
(228 no total) e **205 branches** históricas `feat|fix|refactor` (união de `gh pr list --state all`,
merges e refs remotas).

## 1. 🔴 A direção "restrito demais" é MAIOR do que o #273 mediu, e é medida aqui

O handoff e o ADR falam de *"109 de 111, regressão 0"*. Sobre a população de 205:

```
branches que o strings.Contains de hoje ACEITA    176
branches que ele REJEITA                           29      ← 14%, contra os ~9% do #273
```

As 29 não são todas branch mal nomeada: **14** casam um roadmap por sobreposição de ≥2 tokens de
conteúdo, e a leitura par a par confirma que são o **mesmo trabalho** (ex.:
`fix/windows-integrations-resolve` × `ROADMAP-2026-07-24-fix-windows-path-resolve-em-integrations-…`,
3 tokens; `fix/attention-hooks-pos-auditoria` × 6 roadmaps de attention hooks).

⚠️ **E os 176 são população SOBREVIVENTE.** O autor do #273 renomeou uma branch para caber na regra;
este repositório provavelmente fez o mesmo sem registrar. `Contains` aceitar 86% é em parte
**seleção**, não acerto — o que reforça o fix, não o dispensa.

## 2. 🔴 Tokenizar o nome do arquivo cru dá um token GRÁTIS a quem escreve "roadmap" no slug

`normalizeBranchSlug("ROADMAP-2026-09-09-titulo.md")` = `roadmap-2026-09-09-titulo-md`. Logo
`roadmap` é token de **todo** arquivo do acervo:

```
slug "roadmap", casamentos por sobreposição de 1 token:
   tokenização CRUA        175 de 201
   com prefixo REMOVIDO     16 de 201
```

No nível do **veredito** o efeito aparece em uma branch concreta: `feat/roadmap-list` é aceita sob
tokenização crua por `ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md`, com tokens
compartilhados `[roadmap, list]` — e `roadmap` ali é o **prefixo estrutural**, não o assunto. É
aceitação espúria.

🔴 **O remédio é remover o prefixo posicionalmente (`ROADMAP-`/`REQ-`/`ADR-` + data ISO + `.md`), não
proibir a palavra.** Lista negra quebraria o caso legítimo: um roadmap cujo **título** é "roadmap
move com nome vazio…" deve manter o token `roadmap`. Medido no mesmo par: o `req` de
`…-req-move-list-…` é **título**, sobrevive à remoção, e é o que impede a remoção de virar um
`TrimPrefix` de todos os tokens conhecidos.

## 3. O limiar 2 é FORÇADO pelos dois lados — não é preferência

| N | das 29 rejeitadas, quantas passam | regressão | cardinalidade do slug genérico de 1 token |
|---|---|---|---|
| 1 | **29 (todas)** | 0 | `req` 18 · `gate` 20 · `guard` 14 de 201 |
| **2** | **14** | 0 | **0** |
| 3 | 5 | 0 | 0 |
| 4 | 2 | 0 | 0 |

- **Teto:** o par do #273 (`feat/adrs-retroativas-da-divida-do-acervo` ×
  `ROADMAP-2026-09-05-divida-de-governanca-do-acervo-…`) compartilha **exatamente 2** tokens
  (`divida`, `acervo`). `N≥3` reabre o falso-negativo que o issue reporta.
- **Piso:** com `N=1` a relação aceita **205 de 205** — deixa de discriminar — e um único termo
  genérico admite ~10% do acervo.

## 4. 🔴 "Falso positivo" tem DOIS sentidos aqui, e medir o errado dá falso verde

A função responde *"algum roadmap casou?"* e a relação é um **OR**. Então, para uma branch que já
passava por substring, tokens extras **não mudam veredito nenhum**.

- **Nível de veredito** (o que este portão de fato aplica): quantas branches viram rejeita→aceita. É
  o número do AC15. Regressão aceita→rejeita é **0 por construção** (o braço substring é preservado
  verbatim) e foi **confirmada** empiricamente nas 205, nos 4 limiares e nas 2 tokenizações.
- **Nível de cardinalidade** (quantos roadmaps cada slug casa): é o sinal da **etapa 2** (restritiva)
  e o único lugar onde o limiar se paga.

Reportar só cardinalidade mede algo **ortogonal** ao portão deste ML; reportar só veredito perde a
evidência da etapa 2.

## 5. O slug vazio é exceção declarada, e tem de vir ANTES do OR

`strings.Contains(x, "")` é sempre verdadeiro: a branch `feat/` era relatada como **governada** por
qualquer acervo. O carve-out é a única restrição da etapa aditiva (a razão está escrita na D4 do
ADR). Medido: nenhuma das 205 branches normaliza para slug vazio, logo a restrição é **inerte** neste
acervo — ela fecha o braço vazio de `BranchSlugMatchesRoadmap` que o `ML-1C` deixou apontado.

## 6. ⚠️ Armadilha de gate: `check-validate-rule-pins.sh` reusa `pin6`/`pin7`/`pin8` em OUTRO bloco

Ao acrescentar pins ao **Bloco 2** (`branch_has_wip_roadmap`), continuar a numeração local — `pin6`,
`pin7`, `pin8` — **colide** com os rótulos do **Bloco 3** (credential guard), que já os usa. O gate
**passou com os rótulos duplicados**: nada no script detecta colisão, e o resumo final imprime
`Block 3: pin6-pin20` como se nada tivesse acontecido. Rótulo duplicado é invisível e
inauditável — os pins novos foram renomeados para `pin2b`/`pin2c`/`pin2d`.

## 7. O que NÃO fica resolvido

- A direção **frouxa** continua aberta: `fix/roadmap` segue casando 175 de 201 pelo braço substring.
  É preço escrito da D4 (aditivo primeiro) e depende do gate de corpus do `ML-3C`.
- O vínculo **escrito** (`<roadmap_dir>/.trackfw-branch-links.json`) é estado por checkout: clone,
  fork e CI não o têm e caem na inferência — que agora é **superconjunto** do que era, então é seguro.
  Vínculo obsoleto **avisa** (`branch_link_stale`) e nunca vira violação: promovê-lo quebraria a
  ordem aditiva.

Relacionada: [[nome-vazio-move-o-primeiro-e-nome-completo-move-o-irmao-maior-2026-09-26]] — mesma
família (`strings.Contains` como resolvedor), outra superfície.
