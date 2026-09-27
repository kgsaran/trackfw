# `merge=union` não vale no merge do servidor do GitHub — e mover **um** roadmap de `wip/` para `done/` arrasta os vizinhos

> 2026-09-26 · PR #446 (`REQ-2026-09-09`), logo após o merge do #442
> Arquivos: `.gitattributes` (raiz), `docs/roadmaps/.trackfw-log`, qualquer roadmap em `docs/roadmaps/wip/`

## Sintoma

O GitHub exibiu no PR:

```
This branch has conflicts that must be resolved
  docs/roadmaps/.trackfw-log
```

E o botão de merge ficou desabilitado — apesar de o `.gitattributes` deste repositório declarar,
desde 2026-09-02, exatamente a defesa que deveria impedir isso:

```
.trackfw-log merge=union
```

## Causa 1 — o driver de merge é **local**, e o merge do servidor não o usa

🔴 **`merge=union` é um driver de merge, e drivers de merge só existem na máquina que faz o merge.**
O GitHub resolve o merge do PR no servidor dele, com a configuração dele — **não lê o `merge=`
do seu `.gitattributes`**. Então:

| onde | resultado no mesmo arquivo |
|---|---|
| `git merge origin/main` local | `Auto-merging docs/roadmaps/.trackfw-log` — **sem conflito**, 349 + 349 → 350 linhas |
| botão de merge do GitHub | **CONFLICT**, merge bloqueado |

**Consequência prática:** todo PR desta casa que toque o `.trackfw-log` ao mesmo tempo que a `main`
vai aparecer conflitado no GitHub, e a resolução **tem de ser feita localmente** (`git merge
origin/main` na branch e push) — nunca pelo editor web, que resolveria à mão um arquivo que o
driver resolve sozinho e correto.

⚠️ Isto **não invalida** o `merge=union`; ele continua fazendo o trabalho para quem mergeia local, que
é o caminho desta casa. O que a nota corrige é a expectativa de que o `.gitattributes` protegesse
também o botão.

Complementa [[merge-union-preserva-linhas-mas-nao-ordem-e-metrics-depende-de-posicao-2026-09-02]],
que mede o comportamento **local** do driver — inclusive a perda de ordem cronológica, que é o
verdadeiro custo de ele nunca conflitar.

## Causa 2 — o conflito que o GitHub **não** mostrou, e que teria passado despercebido

O merge local expôs um segundo conflito, invisível na interface:

```
CONFLICT (file location): docs/roadmaps/wip/ROADMAP-2026-09-09-….md
  added in HEAD inside a directory that was renamed in origin/main,
  suggesting it should perhaps be moved to docs/roadmaps/done/ROADMAP-2026-09-09-….md
```

O que aconteceu: o PR #442 moveu **um** roadmap de `wip/` para `done/`. Como naquele momento `wip/`
tinha **poucos** arquivos, a heurística de **detecção de renomeação de diretório** do git concluiu que
`docs/roadmaps/wip/` **inteiro** virou `docs/roadmaps/done/` — e moveu junto o roadmap da branch, que
**ainda não tinha fechado**.

🔴 **O git executou a movimentação e seguiu.** Se ninguém conferir, o merge entrega um roadmap
`wip` sentado em `done/`, com o `status:` do frontmatter dizendo `wip`. O `trackfw validate` acusaria
a divergência depois — mas dentro do PR já mergeado, que é tarde.

**Por que é especialmente provável neste projeto:** o fluxo de governança faz exatamente o movimento
que arma a heurística — roadmaps saem de `wip/` para `done/` um a um, e `wip/` costuma ter 1 ou 2
arquivos. Quanto **menos** arquivos sobram em `wip/`, **mais forte** fica a inferência de que o
diretório foi renomeado.

## Como resolver, na ordem

```bash
git fetch origin
git merge origin/main            # o union resolve o log; leia a saída inteira
ls docs/roadmaps/wip/            # 🔴 confira que o SEU roadmap continua aqui
```

Se ele tiver sido arrastado para `done/`:

```bash
N=<basename do roadmap>
git show HEAD:"docs/roadmaps/wip/$N" > "docs/roadmaps/wip/$N"
git rm --cached "docs/roadmaps/done/$N" && rm -f "docs/roadmaps/done/$N"
git add "docs/roadmaps/wip/$N"
```

⚠️ `git checkout -- <path>` está **bloqueado pelo guard** desta casa (descarta trabalho não
commitado de forma irreversível). `git show <ref>:<path> > <path>` faz o mesmo sem esse risco, e é o
caminho a usar.

## O que conferir antes de commitar o merge

- `grep -c '^<<<<<<<' docs/roadmaps/.trackfw-log` → **0**
- contagem de linhas do log **≥** a de cada lado (união, não substituição)
- `ls docs/roadmaps/wip/` contém o roadmap da REQ que ainda não fechou
- o `status:` do frontmatter concorda com a pasta

## Armadilha de medição registrada junto

Ao investigar por que os checks do PR não apareciam, medi `gh pr view --json statusCheckRollup` e
obtive **1** entrada, contra 21 de um PR do dia anterior, e concluí que *"nenhum evento
`pull_request` disparou"*. **Errado.** Os checks estavam sendo criados; a medição foi feita cedo
demais e o `gh pr checks --watch` **retornou `exit 0` sobre um único check**, concordando com a
premissa errada em vez de esperar.

🔴 **`--watch` termina quando os checks que ele conhece terminam — ele não sabe quantos deveriam
existir.** Régua correta: comparar a contagem com a de um PR recente do mesmo repositório **antes**
de afirmar ausência, e tratar `state=BLOCKED` com `mergeable=MERGEABLE` como *"faltam checks"*, não
como *"os checks falharam"*.
