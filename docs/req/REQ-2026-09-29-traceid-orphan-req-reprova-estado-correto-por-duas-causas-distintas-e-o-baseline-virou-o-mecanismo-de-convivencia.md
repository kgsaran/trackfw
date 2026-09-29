---
status: Abandoned
date: 2026-09-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-09-29-quando-uma-req-deve-ter-roadmap-e-o-casamento-req-roadmap-nao-depende-de-req-id-no-roadmap.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas.md"
---

# REQ: `traceid_orphan_req` reprova estado correto por duas causas distintas

> 🔴 **ABANDONADA em 2026-09-29 — DUPLICATA criada por falha de varredura minha.**
>
> A REQ vigente para esta causa é **`REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md`**, escrita por mim **quatro dias antes** e aberta o tempo
> todo. Varri os **issues** abertos, como a regra manda, e **não varri as REQs abertas** — que a
> Regra Dura coloca no mesmo peso: *"absorvê-las em vez de abrir trabalho paralelo"*.
>
> Todo o conteúdo útil (a segunda causa C2, a ADR, a Wave 0) foi **consolidado na vigente**. Este
> arquivo fica como registro do erro, não como trabalho paralelo — e é deliberado: apagá-lo
> esconderia que uma REQ minha, aberta, ficou invisível na minha própria varredura.

> Date: 2026-09-29 | Status: Abandoned
| Linear Issue:
| Jira Issue:

Origem: **#435**, consumidor externo, a partir de **uso real** — e um comentário de cross-link que
identificou a **segunda** causa, que o issue original não continha.

## Motivation

Ambas as causas reproduzidas pelo arquiteto na **v9.1.0**, em fixture própria:

```
✗ traceid_orphan_req: req "...-sem-roadmap.md" ...         ← REQ recém-criada, ainda sem roadmap
✗ traceid_orphan_req: req "...-par-completo-teste.md" ...  ← par REQ↔roadmap VÁLIDO e pareado
```

| causa | estado correto que a regra reprova |
|---|---|
| **C1** | REQ que **ainda não começou** — o roadmap nasce quando o trabalho começa |
| **C2** | par **existente e pareado**, invisível porque `roadmap new` **não escreve `req_id:`** — medido: `grep -c '^req_id:'` no roadmap gerado = **0** |

🔴 **As duas se parecem no log e diferem no mecanismo**, e o reportante avisou: quem corrigir só C1
vê a regra continuar reprovando e conclui que a correção falhou.

### O custo medido

No consumidor há **14 ocorrências congeladas** no `.trackfw-baseline.json`. **O baseline virou o
mecanismo de convivência com uma regra que dispara em estado correto.** Preço concreto: **toda REQ
nova deixa o CI vermelho** até alguém regenerar o baseline — o que **apaga o passivo de todos**.

🔴 Um controle que obriga o usuário a silenciá-lo em massa não protege: **treina o usuário a
ignorá-lo**.

### A sugestão do issue não pode ser adotada como está

O issue propõe *"não emitir quando a REQ está em `backlog/`"*. Isso usaria a **pasta** da REQ, e a
**ADR-2026-09-03 D1** declara o oposto: *"REQ não tem dimensão de estado… REQ tem `status` no
frontmatter, não pasta de estado."* O recorte tem que ser **semântico** — ver D1/D2 da ADR desta REQ.

### O precedente que mostra que o problema já era conhecido

`req_has_roadmap` mede o **mesmo fato**, também é `error`, e ganhou um **cutoff temporal de
grandfathering**. Isso é a admissão, já no código, de que reprovar "REQ sem roadmap" produz falso
positivo — resolvida com **data** em vez de semântica. Uma data isenta o passivo e **não** isenta a
REQ criada amanhã, que é o caso do issue.

## Acceptance Criteria

- [ ] **Enumeração real** das regras que decidem "esta REQ deveria ter roadmap?", com o critério de
      cada uma e onde divergem — entregável da Wave 0
- [ ] 🔴 **C1 fechada pelo recorte SEMÂNTICO** (`status:` do frontmatter), **nunca pela pasta da REQ**.
      Um teste deve falhar se alguém reintroduzir decisão por diretório
- [ ] 🔴 **C2 fechada pelo casamento por vínculo real** (`req:` do roadmap), e **não** apenas fazendo
      `roadmap new` escrever `req_id:` — que não alcançaria roadmap existente nenhum
- [ ] **Braço do passivo:** um par REQ↔roadmap **já existente**, sem `req_id` no roadmap, deixa de
      disparar **sem que nenhum arquivo seja alterado**
- [ ] **Contra-braço — a regra continua servindo para algo:** REQ **`Done`** sem roadmap **ainda**
      dispara. Se nada mais dispara, a correção virou remoção da regra, e isso seria outra decisão
- [ ] `req_has_roadmap` e `traceid_orphan_req` aplicam **o mesmo critério** (ADR D4)
- [ ] **Delta medido de violações** no corpus deste repositório, antes/depois, com a razão de cada
      uma que sair
- [ ] `make quality` e **CI** verdes

## Negative scope — o que esta REQ NÃO faz

- **Não** remove o cutoff temporal de `req_has_roadmap`. Ele pode ficar redundante sob D1/D2, mas
  removê-lo é mudança de comportamento própria — e medir antes de mexer é o ponto.
- **Não** regenera nem altera `.trackfw-baseline.json` de ninguém. O objetivo é que o baseline deixe
  de ser **necessário** para esconder estado correto, não reescrevê-lo.
- **Não** migra roadmap de ninguém para ganhar `req_id:`. **D3 existe exatamente para evitar isso** —
  o casamento passa a funcionar com o que o produto já grava.
- **Não** trata o **#273** (citado como parente no issue). Mesma família — "regra de rastreabilidade
  que erra por não considerar estado" — mas a causa precisa ser **medida** antes de absorver: a
  Regra Dura pede medição escrita para separar **e** para juntar. A Wave 0 decide.

## Linked ADR
ADR: docs/adr/ADR-2026-09-29-quando-uma-req-deve-ter-roadmap-e-o-casamento-req-roadmap-nao-depende-de-req-id-no-roadmap.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-29-traceid-orphan-req-reprova-estado-correto-por-duas-causas-distintas.md
