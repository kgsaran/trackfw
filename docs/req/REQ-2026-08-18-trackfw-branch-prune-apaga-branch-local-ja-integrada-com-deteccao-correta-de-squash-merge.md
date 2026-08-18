---
status: Open
date: 2026-08-18
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-18-branch-prune-com-dry-run-por-padrao-e-heuristica-de-arquivos-tocados.md"
---

# REQ: `trackfw branch prune` apaga branch local já integrada, com detecção correta de squash-merge

> Date: 2026-08-18 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Pedido de KG em 2026-08-17: falta um comando para apagar a branch local depois do merge.

Hoje é **procedimento manual documentado** no `CLAUDE.md` (§1, "Uma branch ativa por vez") com 6
passos, dois modos de falha conhecidos e uma heurística de desempate. Executei esse procedimento
**cinco vezes** entre 2026-08-16 e 2026-08-18. Procedimento repetido, com regra determinística e
alto custo de erro, é candidato natural a virar comando — mesma tese do `ship` e do `branch new`.

### O problema não é apagar — é **decidir** se pode apagar

`git branch -d` recusa branch não mergeada por ancestralidade. Como a estratégia do projeto é
**squash-merge**, a ancestralidade nunca existe: `-d` recusa **toda** branch integrada, e o usuário
aprende a usar `-D`, que apaga sem verificar nada. **O hábito que o git induz aqui é o inseguro.**

### Defeito existente que esta REQ corrige de passagem

`detectPendingSquashMerges` (`internal/commands/ship.go:564`) já faz detecção parecida, com o teste
**ingênuo**:

```go
diff, derr := gitExec("diff", "origin/main", candidate, "--stat")
if strings.TrimSpace(diff) != "" {
    // avisa "appears to have unmerged changes"
}
```

Só é confiável se a branch estiver **atualizada** com a `main`. Numa branch defasada, o diff
bidirecional reflete **a evolução da main**, não trabalho pendente.

Medido em 2026-08-17: a branch do PR #181, **já mergeada**, aparecia com 4 arquivos divergentes —
todos porque a `main` avançara com o #182. O aviso do `ship` teria dito "unmerged changes" sobre
uma branch integrada.

**Regra escrita em prosa num `CLAUDE.md` não é gate.** Está sujeita a ser esquecida, mal executada,
ou seguida por um agente que não a leu. Virar código a torna verificável.

## Decisões tomadas por KG (2026-08-18)

1. **`--dry-run` é o padrão.** Sem flag, o comando apenas relata. Apagar exige `--apply` explícito.
   Motivo: apagar é destrutivo e nunca deve surpreender, mesmo o comando já recusando o duvidoso.
2. **Fonte de verdade é só o git**, pela heurística de **arquivos-tocados**. Sem consulta a forge:
   o comando precisa funcionar offline e ser determinístico. Caso raro que a heurística não resolva
   é **recusado com motivo**, não adivinhado.

## Escopo

Subcomando `trackfw branch prune` que:

1. **Decide** integração pela heurística de arquivos-tocados, **não** pelo diff bidirecional ingênuo;
2. **Relata** a decisão por branch, com motivo, sempre;
3. **Apaga** apenas com `--apply`, e apenas o comprovadamente integrado;
4. **Recusa** branch com trabalho pendente, a branch corrente, e branch presa em worktree.

A heurística, já documentada no `CLAUDE.md`:

```
mb=$(git merge-base origin/main <branch>)
touched=$(git diff --name-only "$mb" <branch>)                    # o que a branch tocou
diverg=$(git diff --name-only origin/main <branch> -- $touched)   # o que ainda difere
```

- `touched` vazio → sem trabalho próprio → apagar;
- `diverg` vazio → conteúdo final idêntico ao da main **nos arquivos que ela tocou** → integrada;
- `diverg` não-vazio → **recusar e explicar**.

Corrigir `detectPendingSquashMerges` para usar a mesma lógica faz parte: o valor está em ter **uma**
implementação correta, não duas divergentes.

### Escopo negativo — declarado

- **Não apagar branch remota.** Só local. Remoto é destrutivo e compartilhado; se for desejado, é
  REQ própria.
- **Não consultar forge** — decisão 2 acima.
- **Não fazer merge, rebase ou checkout** além do necessário para sair de uma branch a ser apagada.
- **Não mexer na estratégia de merge** do repositório.

## Acceptance Criteria

- [ ] AC1 — Branch integrada por **squash-merge** (sem ancestralidade) é identificada como integrada.
      É o caso que o `git branch -d` erra.
- [ ] AC2 — Branch **defasada e já integrada**, com a `main` avançada por outros PRs, é identificada
      como integrada. É o falso-positivo atual do `ship` — caso de teste discriminante.
- [ ] AC3 — Branch com trabalho **genuinamente pendente** não é apagada, e o motivo é dito.
- [ ] AC4 — Branch **corrente** e branch presa em **worktree** nunca são apagadas.
- [ ] AC5 — Sem `--apply`, **nada é apagado** — nem no caso claramente integrado.
- [ ] AC6 — Sem remoto / offline: degrada com mensagem clara e **não apaga**. Falha fechada.
- [ ] AC7 — `detectPendingSquashMerges` do `ship` passa a usar a mesma lógica; o falso-positivo do
      AC2 deixa de ser emitido.
- [ ] AC8 — Paridade nos 3 CLIs, com gate comparando **saídas reais**, não por leitura de fonte.
- [ ] AC9 — Cenário de falsificação (P4), baseline + detecção, com fixture de repositório **real**
      (git init + commits + squash-merge simulado). Precedente: o Cenário 50 já cria repo git de
      verdade em fixture.
- [ ] AC10 — `make quality` verde **e CI verde** — não fechar AC com evidência de uma só plataforma.

## Riscos para quem executar

- **É um comando destrutivo.** O risco dominante é apagar trabalho não integrado. Todo caso duvidoso
  deve **recusar e explicar**, nunca apagar. Falha fechada é a postura correta.
- **Não testar por leitura de código.** O critério exige fixture com repositório git real e
  squash-merge de verdade; teste com mock de `git` provaria só que o mock concorda com o código.
- **Cuidado com o binário do `PATH`** — pode estar velho, e `--version` não distingue o build.
  Compilar e usar `./bin/trackfw`.

## Linked ADR
ADR: <!-- nenhum; as duas decisões de desenho estão registradas nesta REQ -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-18-branch-prune-com-dry-run-por-padrao-e-heuristica-de-arquivos-tocados.md`
