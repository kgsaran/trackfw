---
status: Accepted
date: 2026-10-01
author: "trackfw_architect"
---

# ADR: estado que governa a branch — criação exige `wip/`, branch existente aceita `blocked/`, e `done/` só pelo vínculo ou pela própria branch

> Date: 2026-10-01 | Status: Accepted

REQ: `docs/req/REQ-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md`
Origem: **#494** (com medição de consumidor externo no comentário) · **#490**
Emenda: `ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir.md` — o escopo
de `InScope` do D1 daquele ADR (`wip∪done`) é substituído pelo D1/D2 deste.
Substitui: o AC da `REQ-2026-08-04` que exige teste de "slug com match em `done/`" no `branch new`.

## Context

O gate branch↔roadmap tem **uma** relação de casamento (`validator.MatchRoadmapsForBranchSlug`,
substring ∪ sobreposição de ≥2 tokens) e **um** conjunto de estados que ela consulta: `wip/ ∪ done/`.
Os quatro portões passam por ela: `branch new`, `commit`, `validate` (`branch_has_wip_roadmap`) e,
via `CheckShipGovernance`, `push` e `ship`.

O defeito não está na relação, e sim no **conjunto de estados**, que é o mesmo para perguntas
diferentes:

| pergunta | quem pergunta | o que o conjunto deveria ser |
|---|---|---|
| "posso **começar** trabalho nesta branch?" | `branch new` | só o que está em andamento: `wip/` |
| "esta branch **existente** ainda é governada?" | `commit`, `validate`, `push`, `ship` | o dono da branch em qualquer estado não terminal (`wip/`, `blocked/`) e, em `done/`, só o roadmap que **esta** branch concluiu |

### Medição (binário da `main` `e104a7f7`, 2026-10-01)

**#490, reproduzido em repositório temporário:** branch `fix/cache-de-sessao` criada por `branch new`
(vínculo escrito gravado), `roadmap move … blocked` → `commit` rc=1, `validate` reprova, `push
--dry-run` reprova. O vínculo escrito vira **stale** porque `blocked/` não está no escopo, e o aviso
`branch_link_stale` manda "recriar o vínculo", o que é a orientação errada.

**#494, acervo deste repositório (217 branches `feat|fix|refactor` históricas × 211 roadmaps em
`done/`, medido com as funções reais do pacote via `go test -overlay`, sem tocar a árvore):**

```
casam done/ por substring            180
casam done/ SÓ por sobreposição       22   (k = nº de roadmaps casados: até 9)
não casam nada                        15
casam MAIS DE UM roadmap de done/    126
```

🔴 **A inferência contra `done/` é frouxa nas duas pernas, não só na de tokens.** 126 de 217 branches
casam mais de um roadmap concluído: substring de slug curto também casa assunto alheio. Apertar só a
sobreposição para `done/` não fecharia o #494. O consumidor do comentário mediu o mesmo num segundo
acervo: 25 de 69 roadmaps têm ≥1 alheio, e com `wip/` vazio **100%** dos casamentos vêm de `done/`.

**Braço contrário já satisfeito por construção** (medido no comentário do #494 e conferido em
`branch.go:155`): `chore/` e `docs/` não passam pelo guard, logo o fechamento pós-merge
(`chore/fecha-req-*`) não depende de `done/`.

## Decision

### D1 — Criação consulta só `wip/`.

`branch new` (guard) e `RecordBranchLink` resolvem contra **`wip/` apenas**. Correção tardia de REQ
concluída segue a Regra Dura de Causa Raiz: o roadmap volta para `wip/` antes. Não há fluxo
legítimo de `feat|fix|refactor` que precise de `done/` no instante da criação.

Quando o slug não casa nada em `wip/` mas casaria em `done/`, a mensagem de bloqueio **nomeia** os
roadmaps de `done/` (até 3, ordem determinística) e orienta `trackfw roadmap move <nome> wip` para
reabrir. A mensagem orienta; o gate não cede.

`RecordBranchLink` passa a gravar só com **exatamente um** casamento em `wip/`. Hoje, um único
casamento espúrio em `done/` vira vínculo escrito permanente: é o FP do #494 promovido a fonte de
verdade.

### D2 — Branch existente: `wip/ ∪ blocked/` por inferência; `done/` só por vínculo ou pela própria branch.

Para `commit` e `validate` (e, por eles, `push`/`ship`), um roadmap governa a branch quando:

1. **vínculo escrito** aponta para ele e ele está em `wip/`, `blocked/` ou `done/`; **ou**
2. ele está em `wip/` ou `blocked/` e casa o slug (relação inalterada); **ou**
3. ele está em `done/`, casa o slug **e** foi movido para `done/` **por esta branch**.

**"Movido por esta branch"** = o arquivo está em `done/` na árvore de trabalho e **não** está em
`done/` na **ponta da base** (`git ls-tree --name-only <base> -- <done_dir>/`). A base é a mesma
âncora que a regra de severidade já usa: `deriveOriginDefaultBranch` (`origin/main` → `origin/master`
→ único ref de `origin`). No CI, o `trackfw-gate.yml` gerado já faz o fetch de `origin/main` para
essa âncora. Não se cria um segundo resolvedor de base. PR contra uma base que não seja a default é
resíduo declarado.

Por que a **ponta da base**, e não o `merge-base`: o checkout raso de CI (profundidade 1 nas duas
pontas) não tem ancestral comum, e o `merge-base` falharia exatamente onde o gate gerado
(`trackfw-gate.yml`) roda. A árvore da ponta da base exige só o fetch de 1 commit, que o workflow
gerado já faz (custo de 0,09 s medido, `scaffold.go`). Se o `main` concluiu o roadmap depois que a
branch divergiu, o arquivo já está em `done/` na base, e a branch não o reivindica.

Por que exigir **também** o casamento de slug no item 3: uma branch que conclui um roadmap alheio não
passa a ser governada por ele.

### D3 — Base inverificável degrada com aviso nomeado, nunca em silêncio e nunca em deadlock.

Sem base resolvível (sem `origin`, ref ausente, `ls-tree` falha), o item 3 do D2 usa a inferência de
hoje (casa o slug em `done/`) **e** emite o aviso `branch_done_scope_unverifiable`, com a causa
(qual ref, qual rc). Não vira violação, pela mesma razão do D4 do ADR-2026-09-26: um portão que
reprova por falta de rede paralisa o commit da própria correção. Não fica silencioso, pela regra que
esse ADR já escreveu para o vínculo stale.

### D4 — O vínculo escrito cobre `blocked/`; o aviso stale nomeia os três estados.

`BranchLinkFor` passa a ter escopo `wip ∪ blocked ∪ done`. O texto de `BranchLinkStaleWarning`
lista os três estados. Mover para `blocked/` deixa de produzir "stale", que é o sintoma do #490.

### D5 — Mensagens em fonte única, parametrizadas por consumidor.

`BranchGovernanceOrientation` e `BranchNoMatchingRoadmapMessage` continuam sendo a fonte única em
`validator`. O texto de criação diz "in wip/". O de branch existente diz "in wip/, blocked/ nor
done/". Proibido duplicar a string num comando. Os pinos (`scripts/check-validate-rule-pins.sh`
PIN3/PIN4) e `docs/cli-parity.md` mudam no mesmo PR.

### D6 — A relação de casamento não muda.

`MatchRoadmapsForBranchSlug` (braços, limiar 2, comprimento mínimo 3) fica intocada. O gate do corpus
(AC14 do ADR-2026-09-26) não deve mudar de veredito. Se mudar, a mudança vazou do escopo.

## Consequences

- O #494 fecha por construção na criação: `done/` sai do conjunto.
- O #490 fecha: `blocked/` governa a branch existente, por vínculo e por inferência.
- A mesma causa fecha nas portas de branch existente: um `git checkout -b` cru fora do Claude Code
  (sem o hook) não consegue mais commitar apoiado num `done/` alheio.
- **Custo:** o `validate` passa a ler uma árvore de git (`ls-tree` na base). É uma chamada, só para
  branch `feat|fix|refactor` e só quando nenhum roadmap de `wip/`/`blocked/` casou.
- **Resíduo declarado:** a inferência contra `wip/ ∪ blocked/` continua frouxa (a relação não muda,
  D6). Com `wip/` pequeno o risco é baixo, mas não é zero.
- **Resíduo declarado:** com base inverificável, o item 3 degrada para a inferência de hoje, com
  aviso.
- **Resíduo declarado:** em branch cuja base de PR não é a default de `origin`, "movido por esta
  branch" é medido contra a default. O erro é na direção frouxa: o roadmap pode estar ausente da
  default e presente na base real.

## Alternatives Considered

- **(b) do #494: avisar e pedir `--allow-done`.** Rejeitada. O comentário mediu que o aviso nomeia 3
  candidatos, e a confirmação vira escolha às cegas. A flag também reabre o gate a quem digitar a flag.
- **(c) do #494: comparar datas de REQ e roadmap.** Rejeitada. Data em nome de arquivo não diz quem
  concluiu o quê. O sinal "movido por esta branch" responde à pergunta diretamente.
- **Apertar a relação só para `done/` (substring ∪ vínculo).** Rejeitada pela medição: substring
  também casa assunto alheio (126/217 com mais de um casamento).
- **Só o `branch new` (deixar os gates de branch existente como estão).** Rejeitada pelo KG em
  2026-10-01 (Regra Dura: mesma causa em outra porta é a mesma REQ).
- **`merge-base` em vez da ponta da base.** Rejeitada: falha no checkout raso de CI, que é onde o gate
  gerado roda.

## Linked REQ

`docs/req/REQ-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md`
