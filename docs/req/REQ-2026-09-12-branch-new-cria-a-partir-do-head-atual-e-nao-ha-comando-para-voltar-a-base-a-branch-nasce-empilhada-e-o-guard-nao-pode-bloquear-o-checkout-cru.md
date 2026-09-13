---
status: Open
date: 2026-09-12
author: ""
adr: ""
roadmap: ""
---

# REQ: `branch new` cria a partir do HEAD atual, e não há comando para voltar à base

> Date: 2026-09-12 | Status: Open

## Motivação

O fluxo governado de criar branch tem **três passos, e o do meio não tem comando**:

```bash
git checkout main && git pull origin main    # 🔴 git cru — nenhum comando cobre
trackfw branch new <type>/<slug>             # cria a partir do HEAD atual
```

`trackfw branch new` executa `git checkout -b <branch>` (`internal/commands/branch.go:124`) a partir
de onde o worktree estiver, e **não tem `--from`**.

🔴 **Se o primeiro passo for pulado, a branch nasce empilhada** sobre commits de outra branch — em
silêncio. Ela parece normal; o **PR** é que chega com commits alheios, quando já tem revisor olhando.

### Já aconteceu neste repositório

**O PR #333 nasceu com 20 commits alheios.** A branch foi criada com `trackfw branch new` dentro de
um worktree que estava na branch do H-01. O comando fez exatamente o que promete; o passo que
faltava não tinha comando.

### 🔴 E é justamente onde o guard não protege

O guard bloqueia `git checkout -b`, `git branch`, `git commit`, `git push`, `git stash`,
`git checkout -- <path>`. **Deixa passar `git checkout <branch>` e `git pull`** — porque não há
alternativa governada para oferecer no lugar.

O agente fica **sem trilho exatamente no passo que, se errado, empilha a branch**. Todo o resto do
fluxo é guiado; este não.

### Verificado que não existe

Três buscas, para não escrever isto por impressão:

```
git grep '"checkout"|"pull"|"switch"' internal/   (sem testes)
   → internal/commands/branch.go:124  git checkout -b   ← o único

git log --all -S'git", "checkout", "main'          → vazio
git log --all --diff-filter=D -- '*housekeep*' '*sync*'  → vazio
```

`housekeeping` aparece em docs, ADRs e CHANGELOG **sempre** no sentido de *"tipo `chore`/`docs`
isento do gate de roadmap"* (`REQ-2026-08-16`) — nunca como comando.

## O molde já existe no próprio comando irmão

`trackfw branch prune` (#187) **faz o fetch sozinho** e, quando falha, avisa em vez de decidir com
dado velho:

> *"Warning: could not fetch origin (offline, no remote, or fetch failed) — evaluating with possibly
> stale data; a branch merged upstream since the last fetch may still be reported as pending."*

Esta REQ pede o mesmo comportamento no `branch new`.

## Acceptance Criteria

- [ ] **AC1** — `trackfw branch new <type>/<slug>` cria a partir de **`origin/main`** por padrão,
      independentemente do HEAD atual.
- [ ] **AC2** — `--from <ref>` cria a partir da ref dada. `--from HEAD` reproduz o comportamento
      atual, para quem empilha **de propósito**.
- [ ] **AC3** — o comando **faz `fetch` de `origin`** antes de resolver a base e, se o fetch falhar,
      **avisa** em vez de seguir com dado velho — molde literal do `branch prune`.
- [ ] **AC4** — 🔴 **falsificação:** estando numa branch com commits próprios, `branch new` sem
      `--from` produz branch com **zero** commits além de `origin/main`. Contra-braço: com
      `--from HEAD`, os commits são preservados.
- [ ] **AC5** — 🔴 **BREAKING declarado.** Hoje o comando cria do HEAD; quem depende disso muda de
      comportamento. Vai no CHANGELOG como break, não como melhoria.
- [ ] **AC6** — o guard passa a **bloquear `git checkout <branch>` e `git pull` crus**, apontando
      para o caminho governado. 🔴 Só depois do AC1 — bloquear antes deixaria o agente sem saída.
- [ ] **AC7** — 🔴 **contra-braço do AC6:** `git checkout` continua permitido onde não há
      alternativa — entrar em worktree, `--detach` para inspeção, voltar para `main` **sem** criar
      branch. Um guard que bloqueia tudo é desligado pelo usuário; é a lição do `ADR-2026-08-17`.
- [ ] **AC8** — paridade nos 3 CLIs.
      ⚠️ Ver ressalva de sequenciamento abaixo.
- [ ] **AC9** — `--dry-run` (já existente) informa **qual base seria usada**, não só se criaria ou
      bloquearia.

## Escopo negativo

- **Não** mexer no gate de governança do `branch new` — ele funciona e não é o assunto.
- **Não** criar comando novo. A capacidade cabe no `branch new`; um `trackfw sync` separado
  adicionaria passo em vez de remover.
- **Não** bloquear `git checkout` antes do AC1 estar entregue.

## ⚠️ Ressalva de sequenciamento

O AC8 (paridade nos 3 CLIs) é afetado pela **`REQ-2026-09-12-v8-um-binario-muitos-canais`**: se a v8
entrar antes, existe **um** runtime e o AC8 vira trivial. Se esta REQ entrar antes, custa 3×.

🔴 **Nenhuma das duas ordens é errada** — mas a decisão é do arquiteto e depende do cronograma da v8.
Por isso o roadmap nasce em `backlog/`.

## Linked ADR
ADR: <!-- não requer: a decisão de base padrão é comportamento de comando, não de arquitetura -->
