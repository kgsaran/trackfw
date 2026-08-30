---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `barrier` executa gate de roadmap não confiável porque `roadmapTrustForGates` falha aberto em todo caminho de erro

> Date: 2026-08-30 | Status: Open

## Motivation

O `ADR-2026-08-23` criou o trust-check para impedir que um roadmap chegado por **PR de terceiro**
execute shell na máquina do mantenedor. **Ele falha aberto.** Reproduzido com o binário:

```bash
# repositório sem remote origin, roadmap nunca commitado, gate hostil
**Gates da wave:**
  touch /tmp/EXECUTOU && echo EXECUTOU

$ trackfw barrier <roadmap> --wave 1      # SEM --trust-local-gates
✓ gates: passed
$ ls /tmp/EXECUTOU                        # → existe. O gate rodou.
```

`internal/commands/barrier.go:568-618` devolve `trusted: true` em **todos** os caminhos de erro:
não é repositório git · `rev-parse --show-toplevel` falha · `filepath.Abs` falha · `filepath.Rel`
falha · `git show` falha com qualquer mensagem que não seja uma de **duas substrings em inglês**. O
comentário admite: *"Any other failure (origin not configured, ref not fetched) → fail-open"*.

**Condições corriqueiras que liberam:** sem remote `origin` · `origin/main` não fetchado · remote com
outro nome · branch padrão `master` · e — hipótese não confirmada, mas o desenho permite — `git` com
saída localizada, já que o casamento é por substring em inglês.

**O cenário que o controle deveria cobrir é exatamente onde ele falha:** quem clona um fork para
revisar um PR normalmente **não tem `origin/main`** daquele fork fetchado.

É defeito de **harness em produção** — atinge todo usuário do trackfw, não este repositório.

## Acceptance Criteria

- [ ] **AC1** — Postura invertida: **fecha por padrão**, abre só quando conseguir **provar** que o
      roadmap está em `origin/main` byte a byte. Ausência de prova é ausência de confiança.
- [ ] **AC2** — Cada condição hoje fail-open passa a `not_evaluated` com **razão nomeada**: sem
      remote, ref não fetchada, não é repositório git, caminho não resolvível.
- [ ] **AC3** — A distinção não pode depender de casamento de substring de mensagem do `git`. Use
      código de saída e comandos que respondam a pergunta diretamente (`git rev-parse --verify`,
      `git cat-file -e`).
- [ ] **AC4** — `--trust-local-gates` continua sendo a saída explícita e auditável, e é a única.
- [ ] **AC5** — Falsificação nas duas direções: roadmap idêntico a `origin/main` → gates **executam**;
      qualquer outra situação → `not_evaluated`, **nunca** execução.
- [ ] **AC6** — Paridade nos 3 CLIs; gate falsificável cobrindo as condições de AC2.
- [ ] **AC7** — Não quebrar o fluxo legítimo do arquiteto neste repositório, que usa
      `--trust-local-gates`.
- [ ] **AC8** — `make quality` exit 0 **e CI verde**.

## Negative Scope

- **Não** remover `--trust-local-gates`.
- **Não** tratar aqui o `roadmap move` seguindo symlink — REQ própria.
- **Não** mudar quais checks o `barrier` roda.

## Observação

Achado por acaso pela `artemis-tf` construindo outro gate; nota em
`vault/notes/barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29.md`, que atribui a
`$TMPDIR` simbólico. A investigação mostrou que o symlink é **um** dos caminhos — a causa é o desenho
fail-open.

## Linked ADR
<!-- Emenda ao ADR-2026-08-23: inverter a postura padrão. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
