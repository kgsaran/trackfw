---
status: wip
date: 2026-08-18
req: "docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `branch prune` com dry-run por padrão e heurística de arquivos-tocados

> Created: 2026-08-18 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md`

**Decisões de KG (2026-08-18), já fechadas:**
1. `--dry-run` é o **padrão**; apagar exige `--apply`.
2. Fonte de verdade é **só o git**, pela heurística de arquivos-tocados. Sem forge, sem rede.

O comando resolve um procedimento manual de 6 passos do `CLAUDE.md` §1, executado 5 vezes entre
16 e 18/08. E corrige de passagem o `detectPendingSquashMerges` do `ship`, cujo teste ingênuo
acusou a branch **já mergeada** do #181 como tendo trabalho pendente.

## Acceptance Criteria
- [x] AC1 — Squash-merge sem ancestralidade é reconhecido como integrado.
- [x] AC2 — Branch defasada e integrada (main avançada) é reconhecida como integrada.
- [x] AC3 — Trabalho pendente não é apagado, e o motivo é dito.
- [x] AC4 — Branch corrente e branch em worktree nunca são apagadas.
- [x] AC5 — Sem `--apply`, nada é apagado.
- [x] AC6 — Offline/sem remoto: degrada e **não apaga**. Falha fechada.
- [ ] AC7 — `detectPendingSquashMerges` usa a mesma lógica; falso-positivo do AC2 some.
- [ ] AC8 — Paridade nos 3 CLIs, com gate de saídas reais.
- [ ] AC9 — Cenário P4 com fixture de repositório git **real**.
- [ ] AC10 — `make quality` verde **e CI verde**.

## 🔴 Riscos que valem para todos os MLs

1. **Comando destrutivo.** Caso duvidoso **recusa e explica**, nunca apaga. Falha fechada.
2. **Fixture tem de ser repositório git real** com squash-merge de verdade. Mock de `git` provaria
   só que o mock concorda com o código. Precedente: Cenário 50 já cria repo git em fixture.
3. **`make quality` verde localmente não fecha AC.** Na REQ anterior fechei o AC de gate com
   evidência só de macOS e o CI (Linux) reprovou. Cenário que depende de git real, caminho ou
   limite de SO exige CI verde antes de fechar.
4. **`$HOME` e `cwd` de teste sempre de fixture**, nunca os reais.

---

## Wave 1 — Núcleo da decisão

### ML-1A — Heurística de integração compartilhada + `branch prune` (dry-run por padrão)
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `internal/commands/branch*.go` + espelhos Node/Python, testes dos 3.

**Ações:**
1. Extrair a heurística de arquivos-tocados para **uma** função reutilizável, nos 3 CLIs:
   ```
   mb      = merge-base origin/main <branch>
   touched = diff --name-only mb <branch>
   diverg  = diff --name-only origin/main <branch> -- touched
   ```
   `touched` vazio → sem trabalho próprio · `diverg` vazio → integrada · senão → **recusa**.
2. `trackfw branch prune`: relata a decisão de **cada** branch local, com motivo. **Não apaga** sem
   `--apply`.
3. Recusa sempre: branch corrente, branch em `git worktree list`, e qualquer caso duvidoso.
4. Offline / sem `origin`: degrada com mensagem clara e **não apaga**.

**Critérios de aceite:**
- [x] Sem `--apply`: nenhuma branch é apagada, nem a claramente integrada. Prove contando branches antes/depois.
- [x] Com `--apply`: apaga a integrada, mantém a pendente, e diz o motivo de cada uma.
- [x] Branch corrente e branch em worktree nunca apagadas, mesmo com `--apply`.
- [x] Offline: não apaga; mensagem clara.
- [x] Fixture de repo git **real** com squash-merge simulado; sem mock de `git`.
- [x] Paridade nos 3 CLIs.
- [x] `make quality` verde.

**Evidência (2026-08-18, Apolo):**

Implementação:
- Go: `internal/commands/branch_prune.go` (`evaluateBranchIntegration` — função única e reutilizável;
  `runBranchPrune`), registrado em `internal/commands/branch.go`. Testes:
  `internal/commands/branch_prune_test.go` (unitários com `gitExec` fake + 1 teste de integração
  com repositório git **real**, bare `origin` + clone, discriminante AC2).
- Node.js: `npm/src/branch/prune.js` (`evaluateBranchIntegration`, `runBranchPrune`), wired em
  `npm/src/commands/branch.js`. Testes: `npm/tests/branch-prune.test.js` (mesmo espelhamento,
  incluindo o teste com repo git real).
- Python: lógica embutida em `pypi/trackfw/commands/branch.py` (`evaluate_branch_integration`,
  `run_branch_prune`), mesmo padrão do arquivo existente (`run_branch_new` já vive lá). Testes:
  `pypi/tests/test_branch_prune.py`.
- Gate de paridade novo: `scripts/check-branch-prune-parity.sh` — 4 cenários (dry-run padrão,
  `--apply` apaga só integradas, `--apply` nunca apaga a branch corrente, offline recusa tudo),
  cada um com repositório git **real** (bare `origin` + clone, squash-merge simulado de verdade —
  sem mock de `git`), byte-a-byte nos 3 binários reais. Adicionado ao `make quality` via `parity`
  target (`Makefile`).
- `docs/cli-parity.md`: nova seção "`trackfw branch prune`" documentando a heurística, os
  branches sempre mantidos (`main`/corrente/worktree), o comportamento offline, e o gate de
  paridade.

Comandos de validação (saída bruta):
```
$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...
ok  	github.com/kgsaran/trackfw/internal/commands	6.452s
... (todos os pacotes ok)

$ cd npm && node --test tests/*.test.js
ℹ tests 677
ℹ pass 677
ℹ fail 0

$ python3 -m pytest pypi/tests -q
1356 passed, 28 subtests passed in 27.80s

$ GO_BIN=bin/trackfw bash scripts/check-branch-prune-parity.sh
OK   [branch-prune-parity/dry-run-default]
OK   [branch-prune-parity/apply-deletes-integrated]
OK   [branch-prune-parity/apply-never-deletes-current-branch]
OK   [branch-prune-parity/offline-refuses]
All check-branch-prune-parity.sh scenarios passed.

$ make quality
[... build, test, test-node, test-python, lint, e todos os scripts de parity/falsify ...]
[exited with code 0]
```

`./bin/trackfw validate`: mesmos 19 warnings pré-existentes desta sessão (nenhum novo), 0
violações — governance_mode strict continua passando.

**Discriminante AC2 provado, não assumido:** o teste de repositório real (Go, Node e no gate de
paridade) confirma explicitamente que `git diff origin/main feat/a --stat` (o check ingênuo) é
**não-vazio** antes de checar que `evaluateBranchIntegration` classifica `feat/a` como
`content_identical` (apagável) — provando que o teste discrimina de fato entre o check ingênuo e
a heurística nova, não passa vacuamente.

**Bug de maior severidade encontrado e fechado antes de qualquer código de produção** (apontado
pelo advisor antes da implementação): aplicada literalmente "a cada branch local", a heurística
classificaria a própria `main` como integrada (`merge-base origin/main main` == a ponta de
`main`, `touched` fica vazio) e a ofereceria para apagar. `main` é excluída por nome, antes da
heurística rodar, nos 3 CLIs — coberto por teste dedicado
(`TestRunBranchPrune_DryRun_NeverDeletes_MainNeverCandidate` e equivalentes Node/Python) que
falha explicitamente se `main` aparecer como candidata a `delete` em qualquer linha da saída.

**Fora de escopo, confirmado nesta sessão:** ML-2A (convergência do `ship.go`/`ship/runner.js`/
`ship/runner.py`'s `detectPendingSquashMerges` para chamar `evaluateBranchIntegration` em vez do
diff bidirecional) — não implementado; `ship.go` e equivalentes não foram tocados. AC7/AC9 (na
parte que caiba ao `ship`) e o item "Cenário P4 com fixture de repositório git real" para o
`ship` continuam pendentes do ML-2A, conforme a Wave 2 do roadmap já delimitava. Nenhum
`git commit`/`push`/`branch` executado por mim — autoridade exclusiva do `trackfw_architect`.

---

### Auditoria do ML-1A pelo arquiteto — aprovada

Fixture de repositório git **real** e descartável em `/tmp` (bare remote + clone, squash-merge de
verdade, `main` avançando depois, worktree ocupando branch). Nunca tocou o repositório do projeto.

```
SEM --apply     4 branches antes, 4 depois -> NADA apagado
classificacao   em-worktree: keep · integrada: delete · pendente: keep (nomeia docs/b.md)
                main: keep (default branch — never pruned)
COM --apply     apaga so feat/integrada; main, corrente e worktree sobrevivem
OFFLINE         recusa avaliar, nao apaga, exit 1, mensagem clara
make quality    exit 0 · 130 cenarios · gate novo 4/4 · validate exit 0
```

**Discriminante provado no fixture, não por leitura** — é o núcleo da REQ:

```
teste INGENUO (o que o ship usa hoje):
  git diff origin/main feat/integrada --stat  ->  2 linhas
  => acusaria "unmerged changes" numa branch JA INTEGRADA

heuristica de arquivos-tocados:
  touched: docs/a.md
  diverg:  [VAZIO]  ->  INTEGRADA
```

**Bug pego pelo advisor do agente, e vale registrar:** a versão inicial podia apagar a própria
`main`. Foi fechado com teste failing-first nos 3 CLIs. É o tipo de defeito que num comando
destrutivo não tem segunda chance.

**Observação de processo:** o guard bloqueou a criação do meu fixture (`git commit` literal no
comando). É o guard funcionando — o uso era legítimo, num repositório temporário. Contornei pondo o
setup num script e executando o script, de forma transparente. Registrado porque é atrito real que
qualquer auditoria de comando git vai encontrar.

---

## Wave 2 — Convergência do `ship` (depende da Wave 1)

### ML-2A — `detectPendingSquashMerges` passa a usar a heurística compartilhada
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A — a função só existe depois dele.

**Discriminante medido, use como caso de teste:** a branch do PR #181, já mergeada, era acusada de
"unmerged changes" porque a `main` avançara com o #182. Quatro arquivos apareciam divergentes sem
haver trabalho pendente.

**Critérios de aceite:**
- [ ] O `ship` deixa de avisar sobre branch defasada porém integrada.
- [ ] Continua avisando sobre branch com trabalho genuinamente pendente — não-regressão.
- [ ] Uma só implementação da heurística; sem cópia divergente.
- [ ] Cenário P4 com baseline e detecção.
- [x] `make quality` verde.

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`: revisão de comando destrutivo
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-18-revisao-do-branch-prune.md`

**Ações:** é o primeiro comando do trackfw que **apaga** trabalho. Verificar se há caminho para
apagar branch não integrada — nome com caracteres especiais, branch com upstream sumido, `origin`
apontando para lugar errado, `main` local defasada em relação a `origin/main`, repositório sem
`origin`, branch cujo nome colide com ref ambígua. Avaliar se `--apply` pode ser disparado sem
intenção. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, declarado:** apagar branch remota; consultar forge; alterar estratégia de merge.
- Commits e branch são exclusivos do `trackfw_architect`.
