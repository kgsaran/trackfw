---
status: Done
date: 2026-08-19
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
---

# REQ: guard não bloqueia comandos destrutivos de working tree em repo compartilhado por agentes

> Date: 2026-08-19 | Status: Open

## Motivação

Levantado por KG: **agentes não devem conseguir rodar `git stash`** — num repositório compartilhado,
um agente que faz stash tira as alterações não commitadas **de todos os outros** do working tree.

O pressuposto foi verificado, não presumido: `git worktree list` mostra **um único worktree**. Os
subagentes despachados em paralelo compartilham de fato o mesmo diretório de trabalho. O risco é
real, não hipotético.

### O `stash` é o caso **menos** grave da classe

Medido: o guard (`scripts/trackfw-git-branch-guard.sh`) bloqueia `commit`, `push`, `checkout -b`,
`switch -c`, `branch` de criação/renomeação e `worktree add -b`. Bloqueia **zero** comandos
destrutivos de working tree. Nenhum ADR, REQ ou nota de vault menciona a classe.

| comando | efeito no worktree compartilhado | recuperável? |
|---|---|---|
| `git stash` / `stash push` | remove alterações não commitadas de **todos** | sim, via `stash pop` |
| `git stash clear` / `drop` | apaga o stash | **não** |
| `git checkout -- <path>` / `git restore <path>` | descarta alterações não commitadas | **não** |
| `git reset --hard` | descarta tudo não commitado | **não** |
| `git clean -f` / `-fd` / `-x` | apaga arquivos não rastreados | **não** |

Bloquear só o `stash` seria de novo **"condição estreita demais"** — o padrão já nomeado quatro
vezes nesta série. O agente que quebraria o trabalho alheio com `stash` o quebra **pior** com
`reset --hard`, e este é irrecuperável.

**Decisão de KG: bloquear a classe inteira.**

## 🔴 O risco dominante é o oposto do óbvio: super-bloquear

O próprio guard já registra esse julgamento, na regra do `git branch`: *"bloquear leitura seria pior
que a brecha"*. Vale igual aqui, e com um caso concreto que **quebraria o fluxo desta sessão**:

- **`git reset --soft HEAD~1` é o contorno padrão** para empurrar trabalho já commitado via
  `trackfw ship` (que exige algo staged). Bloquear `git reset` inteiro inviabiliza o próprio trilho
  governado. **Só `--hard` entra.**
- **`git checkout <branch>` precisa continuar funcionando.** Distinguir branch de caminho sem `--`
  é genuinamente ambíguo; adivinhar produz falso-positivo. Bloquear apenas a **forma explícita de
  caminho**.
- **Leitura nunca bloqueia:** `stash list`, `stash show`, `clean -n`/`--dry-run`, `restore --staged`
  (não toca o working tree).

Guard que atrapalha é guard que o usuário desliga — e guard desligado protege zero. Isso já está
escrito no `ADR-2026-08-17` e vale aqui inteiro.

## Escopo

Bloquear, com mensagem que **nomeia a alternativa**:

```
git stash | git stash push | git stash save
git stash clear | git stash drop
git reset --hard              (qualquer posição do token)
git clean -f | -fd | -x | -X  (mas NÃO -n / --dry-run)
git restore <path>            (mas NÃO --staged sozinho)
git checkout -- <path> | git checkout .
```

Liberar explicitamente, e **provar por cenário** que continuam livres:

```
git stash list | git stash show
git reset (sem --hard) — inclui --soft e --mixed
git clean -n | --dry-run
git restore --staged
git checkout <branch> | git switch <branch>
```

## O que **não** é escopo

- **Worktree isolado por subagente.** É a defesa estrutural, e é melhor que a tripwire — mas é
  mudança de orquestração, não de produto. Fica registrada como recomendação, não como entrega.
- Bloquear `git rm`, `git filter-branch`, `git gc --prune` — outra classe, sem incidente medido.
- Afrouxar qualquer regra existente do guard.

## Consequência declarada, coerente com o `ADR-2026-08-12`

Isto é **tripwire, não fronteira de segurança**. Um agente induzido contorna com `python -c
"shutil.rmtree(...)"`, com `rm -rf`, ou com qualquer coisa que não se pareça com `git`. O guard
existe para tornar o acidente improvável e o ato deliberado **visível**, não para impedi-lo. Não
prometer o que não se entrega.

## Acceptance Criteria

- [ ] AC1 — Todos os comandos da lista de bloqueio são recusados, com mensagem que nomeia a alternativa.
- [ ] AC2 — Todos os comandos da lista de liberação **continuam funcionando** — provado por cenário,
      não por leitura. Em especial **`git reset --soft`**, do qual o próprio trilho depende.
- [ ] AC3 — Cobre as formas de evasão já conhecidas do guard: prefixo `env`/`command`, flag fora da
      primeira posição de token, `git${IFS}stash`. As brechas anteriores foram corrigidas assim.
- [ ] AC4 — Script **byte-idêntico** entre os 3 CLIs e entre escopos de projeto e global.
- [ ] AC5 — No-op fora de projeto trackfw preservado (`ADR-2026-08-17`).
- [ ] AC6 — Cenário P4 por comando bloqueado **e** por comando liberado — o falso-positivo é o risco
      dominante, então precisa de braço de detecção próprio.
- [ ] AC7 — Seção no `docs/cli-parity.md` **nomeando o gate** que protege.
- [ ] AC8 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **A fonte canônica do script é o literal `gitBranchGuardScript` em
  `internal/generators/scaffold.go`** (+ espelhos Node/Python). Existem 7 cópias em disco geradas a
  partir dele — **nunca editar as cópias à mão**.
- **Não quebrar o no-op nem o dreno de stdin.** Um ML anterior desta série introduziu EPIPE ao mexer
  na ordem e foi reprovado por isso.
- **Cuidado com o binário do `PATH`** — está desatualizado e `--version` não distingue o build.

## Linked ADR
ADR: <!-- avaliar: pode caber como emenda ao ADR-2026-08-12 em vez de ADR novo -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md` (Wave 3)
