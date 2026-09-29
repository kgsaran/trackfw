---
status: Accepted
date: 2026-09-29
author: "trackfw_architect"
---

# ADR: quando uma REQ deve ter roadmap, e o casamento REQ↔roadmap não depende de `req_id` no roadmap

> Date: 2026-09-29 | Status: Accepted

## Context

`traceid_orphan_req` reprova repositório em **estado correto**, por **duas causas distintas**. Ambas
reproduzidas pelo arquiteto na **v9.1.0** em fixture própria:

```
✗ traceid_orphan_req: req "...-sem-roadmap.md" ...        ← REQ recém-criada, sem roadmap ainda
✗ traceid_orphan_req: req "...-par-completo-teste.md" ... ← par REQ↔roadmap VÁLIDO e pareado
```

| causa | o estado correto que a regra reprova |
|---|---|
| **C1** | REQ que **ainda não começou** — o roadmap só nasce quando o trabalho começa |
| **C2** | par **existente e pareado**, invisível porque `roadmap new` **não escreve `req_id:`** no frontmatter do roadmap (medido: `grep -c '^req_id:'` = **0**) |

🔴 **As duas se parecem no log e diferem no mecanismo.** O reportante avisou disso explicitamente:
quem corrigir só C1 vê a regra continuar reprovando e conclui que a correção não funcionou.

### O custo medido, que é o que torna isto grave

No repositório consumidor há **14 ocorrências de `orphan_req` congeladas** no
`.trackfw-baseline.json`. **O baseline virou o mecanismo de convivência com uma regra que dispara em
estado correto** — e o preço é concreto: **toda REQ nova deixa o CI vermelho** até alguém regenerar
o baseline, o que **apaga o passivo de todos**, não só o novo.

🔴 Um controle que obriga o usuário a silenciá-lo em massa não está protegendo nada: está
**treinando** o usuário a ignorá-lo.

### 🔴 A sugestão do issue não pode ser adotada como está

O issue propõe *"não emitir quando a REQ está em `backlog/`"*. Isso **usaria a pasta da REQ** — e a
**ADR-2026-09-03, D1** declara o invariante oposto:

> *"REQ **não** tem dimensão de estado. `backlog`/`analyzing`/`wip`/… são conceito de **roadmap**.
> REQ tem `status` no frontmatter (`Open`/`Done`), não pasta de estado."*

Adotar a sugestão literal escreveria no produto exatamente a suposição que aquela ADR existe para
extinguir — *"um leitor supondo um nível que não existe"*. O recorte tem que ser **semântico**.

### O precedente que mostra que o problema já era conhecido

Existe uma **regra vizinha**, `req_has_roadmap`, que mede o **mesmo fato** ("esta REQ não tem
roadmap") e também é `error` por default. Ela ganhou um **cutoff temporal de grandfathering**
(`validator_req_roadmap_cutoff.go`): REQs criadas antes de uma data são isentas.

🔴 **Isso é a admissão, já no código, de que reprovar "REQ sem roadmap" produz falso positivo** — mas
resolvida com **data**, não com semântica. Uma data isenta o passivo e **não** isenta a REQ que você
cria amanhã, que é exatamente o caso do issue.

## Decision

**D1 — O recorte é semântico, nunca por pasta da REQ.** Nenhuma regra decide "esta REQ deveria ter
roadmap?" olhando o diretório em que a REQ está. O discriminante é o **`status:` do frontmatter** —
que é o campo que a ADR-2026-09-03 D1 estabelece como a dimensão legítima da REQ.

**D1-bis — Por que `status:` e não `e.state`, medido.** O `reqIndex` **tem** um campo `state`
(`validator_traceid.go:136`, `state = parent`), e a regra vizinha `traceid_state_mismatch` **já o
consome** (linha 289). Seria tentador usá-lo. **Medido, ele não serve como fonte única:**

```
validator_traceid.go:77    state: ""        ← layout PLANO: vazio
validator_traceid.go:136   state = parent   ← subpastas: nome do diretório
validator_traceid.go:289   "Só compara se ambos têm estado definido"
```

🔴 **Em layout plano — o deste repositório — `e.state` é vazio, e a regra ficaria inerte.** O
`status:` do frontmatter está **sempre** disponível. E o próprio precedente já reconhece a lacuna, ao
só comparar quando ambos os estados existem.

**D2 — Uma REQ `Open` sem roadmap NÃO é órfã; é uma REQ que ainda não começou.** É o protocolo que o
próprio produto recomenda (`req new` → `roadmap new` → `roadmap move wip`). Reprovar ali **inverte o
fluxo**: ou o usuário cria roadmaps vazios para satisfazer o validate, ou convive com violação
permanente. O sinal legítimo é **REQ `Done` sem roadmap** — aí sim, algo se perdeu.

**D2-bis — `Superseded` e `Closed` NÃO disparam.** A Wave 0 achou **14 `Superseded` + 2 `Closed`**
no corpus, sem comportamento declarado — um limbo que bloquearia o ML-1A.

**Decidido:** só **`Done`** dispara. `Superseded` e `Closed` são desfechos em que **a ausência de
roadmap é esperada** — uma REQ substituída ou encerrada sem execução não deveria ter roadmap, e
exigi-lo recriaria o defeito desta REQ noutro estado. 🔴 E o caso deixou de ser hipotético: esta
própria REQ vai marcar outra como `Superseded` na consolidação.

**D3 — O casamento REQ↔roadmap não pode depender de `req_id` no roadmap.** O `roadmap new` não o
escreve, e **milhares de roadmaps existentes não o têm**. A regra passa a casar também pelo vínculo
que o produto **de fato** grava — o campo `req:` do frontmatter do roadmap.

🔴 **Corrigir apenas o gerador (passar a escrever `req_id:`) é insuficiente por construção:** não
alcança nenhum roadmap já existente, e as 14 ocorrências congeladas continuariam vivas. O casamento
por vínculo real corrige o passivo **sem migração**.

**D4 — As regras vizinhas param de discordar. São QUATRO, não duas.** A Wave 0 varreu todos os
`applyRule*(` e encontrou quatro que opinam sobre o vínculo REQ↔roadmap:

| regra | critério | filtra por estado? |
|---|---|---|
| `req_has_roadmap` | REQ sem `roadmap:` apontando para `.md` real | **não** |
| `traceid_orphan_req` | REQ com `req_id` sem roadmap de mesmo id | **não** |
| `ref_targets_exist` | REQ com `Roadmap:` apontando para arquivo inexistente | **não** |
| `req_roadmap_lifecycle` | REQ `Open` com roadmap em `done/` | **sim** — só `Open` |

🔴 **A primeira versão desta ADR nomeava duas.** A quarta já filtra por estado — logo o precedente de
"regra consciente de estado" **já existe no produto**, e o que falta é consistência, não invenção.

As quatro aplicam o mesmo critério de "esta REQ deveria ter roadmap?". Duas regras com vereditos diferentes sobre a
mesma pergunta é a mesma classe de defeito do #450 (`context` × `status`), noutra superfície.

## Consequences

**Positivas**
- O consumidor deixa de ter CI vermelho ao criar REQ, que é o fluxo normal do produto.
- O `.trackfw-baseline.json` deixa de ser usado para esconder estado correto.
- O casamento passa a funcionar para roadmaps **já existentes**, sem pedir migração a ninguém.

**Limite de D3, medido e declarado (Wave 0)** — casar por `req:` **não** fecha C2 para todos:

| bucket | roadmaps | razão |
|---|---|---|
| `req:` **stale** | 32 | 17 apontam para `docs/requisições/` (dir migrado), 7 sem `.md`, 8 outros |
| `req:` **vazio/null** | 8 | `req: ""` ou `req: ~` |
| **sem** campo `req:` | 18 | gerações antigas |

**58 roadmaps** neste corpus continuam disparando após D3. Não é refutação — é o **alcance** de D3, e
está escrito para que ninguém declare C2 fechada sem rodar o discriminador no corpus alvo.

⚠️ **E D3 deve chamar `normalizeRefSeparator`** (`validator.go:3285`): medido, um `req:` gravado com
`\` (Windows) não casa sem isso.

**Negativas e aceitas**
- **A contagem de violações cai** em repositórios que hoje as têm. É a correção aparecendo — e entra
  nas release notes, porque número que muda sozinho entre versões gera issue.
- **Uma REQ `Open` abandonada há meses sem roadmap deixa de ser sinalizada como violação.** Aceito:
  o sinal de abandono é `status`/idade, não ausência de roadmap, e emitir violação por isso foi
  exatamente o que produziu este defeito.
- 🔴 **RETRATAÇÃO (Wave 0, 2026-09-29).** A primeira versão desta ADR afirmava que *"o cutoff
  temporal de `req_has_roadmap` pode tornar-se redundante sob D1/D2"*. **REFUTADO por medição:** dos
  13 warnings grandfathered no corpus, **3 são `Done`** — sob D2 elas voltariam a ser **violations**
  se o cutoff sumisse. O cutoff **não** é redundante; permanece, e removê-lo continua sendo decisão
  própria.

## Alternatives Considered

**Excluir REQ em `backlog/` (a sugestão literal do issue)** — rejeitado por conflito direto com a
**ADR-2026-09-03 D1**. Reintroduziria no produto a suposição de que REQ tem pasta de estado.

**Rebaixar `traceid_orphan_req` para warning** — rejeitado como solução única. Silencia o sintoma nas
**duas** causas e deixa o falso positivo de pé; um warning permanente treina o mesmo hábito de
ignorar que o baseline já treinou. Rebaixar pode ser **parte** da resposta para casos residuais, mas
não substitui o recorte correto.

**Fazer `roadmap new` escrever `req_id:`** — rejeitado como solução única (é **parte** de D3, não o
todo): não alcança roadmap existente nenhum, e o passivo medido no consumidor continuaria.

**Regenerar o baseline** — rejeitado. É o que o consumidor já faz, e apaga o passivo de todos para
acomodar um caso novo. Trata o instrumento, não o defeito.
