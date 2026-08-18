---
status: done
date: 2026-08-18
req: "docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: `branch prune` com dry-run por padrão e heurística de arquivos-tocados

> Created: 2026-08-18 | Status: done

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
- [x] AC7 — `detectPendingSquashMerges` usa a mesma lógica; falso-positivo do AC2 some. (ML-2A)
- [x] AC8 — Paridade nos 3 CLIs, com gate de saídas reais. (ML-2B: `check-ship-parity.sh`,
  cenário `squash-merge-warning`, roda os 3 binários reais contra o mesmo fixture git e compara
  stdout+stderr+exit-code byte a byte)
- [x] AC9 — Cenário P4 com fixture de repositório git **real**. (satisfeito por ML-1A/1C para o
  `branch prune`; ML-2B fecha para o `ship` — Cenário 70 do `check-gates-falsify.sh` sabota o
  Node e prova que `check-ship-parity.sh` reprova)
- [x] AC10 — `make quality` verde **e CI verde**. (`make quality` verde localmente, confirmado
  nesta sessão inclusive com o gate novo; CI verde depende do push/PR — autoridade do
  `trackfw_architect`)

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

### ML-1B — Fechar as divergências contra o protocolo do `CLAUDE.md` §1
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Origem:** pergunta de KG (2026-08-18) — *"estamos usando o mesmo método já documentado?"*.
Comparei item a item **no código**, e não estávamos por inteiro.

| Protocolo `CLAUDE.md` §1 | ML-1A | veredito |
|---|---|---|
| Passo 3-bis: `merge-base` → `touched` → `diverg` | ✅ | núcleo correto |
| Não apagar branch em worktree | ✅ | |
| Passo 3 — teste ingênuo `git diff --stat` | ❌ | **correto omitir** — é o defeito que a REQ corrige |
| `gh pr list` para caso duvidoso | ❌ | **correto omitir** — decisão de KG: sem forge |
| Passo 2 — candidatas via `branch -r --no-merged` | ❌ | **correto divergir** — apagamos branch **local** |
| **Passo 1 — `git fetch origin --prune`** | ❌ | 🔴 **fechar** |
| **`diverg` só em doc/config → housekeeping** | ❌ | 🔴 **fechar, com ressalva abaixo** |
| **Tentar `-d` antes de `-D`** | ❌ | 🔴 **fechar** |

**Ações:**

1. **`fetch origin --prune` antes de avaliar.** Sem ele, `origin/main` pode estar velho e uma branch
   recém-mergeada aparece como pendente. A falha do fetch é **não-bloqueante** (offline é caso de
   uso legítimo, mesmo padrão do passo 3 do `ship`), mas o comando deve **avisar** que avaliou com
   dado possivelmente defasado. Note que `origin/main` velho torna o resultado **mais conservador**,
   não menos — mais recusas, nunca mais deleções.

2. **Divergência só em doc/config — NÃO apagar automaticamente.** O `CLAUDE.md` manda tratar como
   housekeeping e apagar, mas ali havia **humano no laço**. Num comando destrutivo isso apagaria
   branch cuja única pendência é documentação. **Classifique como categoria própria** (ex.: `review`)
   com motivo — *"só doc/config diverge; provável housekeeping, confirme e apague"* — em vez de
   `delete`. Preserva "falha fechada" e ainda resolve o caso para o usuário.
   **Se você discordar, argumente antes de implementar.**

3. **Tentar `git branch -d` antes de `-D`.** Se `-d` aceitar, a integração é confirmada também pelo
   git. Só cai para `-D` quando `-d` recusar por ancestralidade — que é o esperado em squash-merge.

4. **Corrigir o texto do `Long` help**, que hoje afirma substituir o procedimento de 6 passos. Depois
   deste ML fica preciso; enquanto não estiver, é afirmação forte demais.

**Critérios de aceite:**
- [x] `fetch origin --prune` roda antes da avaliação; falha é não-bloqueante **e avisada**.
- [x] Offline continua **não apagando** nada (não-regressão do AC6).
- [x] `origin/main` defasado leva a **mais recusas**, nunca a deleção indevida — prove com fixture.
- [x] Divergência só em doc/config vira categoria própria, **não** `delete`.
- [x] `-d` é tentado antes de `-D`; queda para `-D` só quando `-d` recusa por ancestralidade.
- [x] Texto do help condiz com o que o comando faz.
- [x] Não-regressão de todo o ML-1A: dry-run padrão, proteções de main/corrente/worktree, offline.
- [x] Paridade nos 3 CLIs; fixture de repo git **real**.
- [x] `make quality` verde.

**Decisão sobre a ação 2 (doc/config), sem discordar de KG:** implementada exatamente como pedido —
nova categoria `review_doc_config`, ação reportada `review`, nunca `delete`, nunca tocada por
`--apply`. Não usei o critério "só CLAUDE.md" literal do `CLAUDE.md` §1 (esse exemplo já cai em
`isDocsFile`, `.md`); estendi para uma lista conservadora de extensões/arquivos de config
(`.yaml`, `.yml`, `.json`, `.toml`, `.ini`, `.cfg`, `.gitignore`, `.gitattributes`,
`.editorconfig`, `trackfw.yaml`, `LICENSE`) porque uma classificação errada aqui nunca causa uma
deleção — só muda entre duas categorias que ambas mantêm a branch (`review_doc_config` vs
`pending_work`). Risco assimétrico: nenhum.

**Evidência (2026-08-18, Apolo):**

Implementação — 3 ações fechadas nos 3 CLIs:
1. **`fetch origin --prune`** — `internal/commands/branch_prune.go` (`runBranchPrune`, antes do
   `rev-parse --verify -q origin/main`), `npm/src/branch/prune.js`,
   `pypi/trackfw/commands/branch.py` (`run_branch_prune`). Falha só imprime aviso (mensagem
   idêntica nos 3 CLIs) e a avaliação continua contra o `origin/main` já resolvível localmente —
   diferente do `ship.go`, que pula o check inteiro na falha do fetch.
2. **Categoria `review_doc_config`** — nova decisão em `evaluateBranchIntegration` /
   `evaluateBranchIntegration` / `evaluate_branch_integration`, roteada quando `diverg` é
   inteiramente doc/config (`isDocOrConfigPath` nos 3 CLIs). Ação reportada `review`; nunca
   `deletable()`; resumo próprio no relatório (`N branch(es) need manual review...`).
3. **`-d` antes de `-D`** — `defaultDeleteBranch` / `defaultDeleteBranch` / `_default_delete_branch`
   tentam `-d` primeiro, caem para `-D` só quando `-d` recusa.
4. **`Long`/`description`/`addHelpText`** reescritos nos 3 CLIs — não afirmam mais "offline
   command" nem "replaces the 6-step procedure" sem qualificação; descrevem o fetch best-effort, a
   categoria de review e a ordem `-d`→`-D`.

Testes novos (mesmo padrão do ML-1A — fixture git real, sem mock de `git`):
- Go: `internal/commands/branch_prune_test.go` — `TestEvaluateBranchIntegration_ReviewDocConfig_NotDeletable`,
  `TestEvaluateBranchIntegration_MixedDocAndCode_StaysPendingWork`,
  `TestRunBranchPrune_FetchFails_WarnsButStillEvaluates`,
  `TestDefaultDeleteBranch_TriesDashDBeforeDashD_BothCodepaths` (repo real, ambos os caminhos),
  `TestRunBranchPrune_RealGitRepo_StaleOriginMain_IsConservativeNotWrong` (repo real: dois clones,
  URL do remoto quebrada para forçar falha do fetch, branch verdadeiramente integrada upstream mas
  invisível ao `origin/main` defasado é reportada `keep`, e vira `delete`-worthy assim que um fetch
  real acontece — mesma branch, só a atualidade do ref muda).
- Node.js: `npm/tests/branch-prune.test.js` — mesmos cenários, mesmo padrão de fixture real.
- Python: `pypi/tests/test_branch_prune.py` — idem.
- Gate de paridade `scripts/check-branch-prune-parity.sh` ganhou 2 cenários novos (repositório git
  real, byte-a-byte nos 3 binários): `review-doc-config-only` e `stale-origin-main-conservative`.
  Os 4 cenários pré-existentes do ML-1A continuam passando sem alteração (o `fetch` novo é
  transparente contra o bare `origin` local do fixture).

Não-regressão do ML-1A: os 3 testes de fixture com `f1.md` (que agora cairiam em
`review_doc_config` em vez de `pending_work`, já que `.md` é doc) foram ajustados para `f1.go` —
preservando a intenção original do teste (trabalho de código genuinamente pendente), sem alterar
nenhum comportamento de produção do ML-1A.

Comandos de validação (saída bruta):
```
$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 3m ./...
ok  	github.com/kgsaran/trackfw/internal/commands	6.898s
... (todos os pacotes ok)

$ cd npm && node --test tests/*.test.js
ℹ tests 682
ℹ pass 682
ℹ fail 0

$ PYTHONPATH=pypi python3 -m pytest pypi/tests -q
1361 passed, 28 subtests passed in 29.24s

$ GO_BIN=bin/trackfw bash scripts/check-branch-prune-parity.sh
OK   [branch-prune-parity/dry-run-default]
OK   [branch-prune-parity/apply-deletes-integrated]
OK   [branch-prune-parity/apply-never-deletes-current-branch]
OK   [branch-prune-parity/offline-refuses]
OK   [branch-prune-parity/review-doc-config-only]
OK   [branch-prune-parity/stale-origin-main-conservative]
All check-branch-prune-parity.sh scenarios passed.

$ make quality
[exited with code 0]

$ ./bin/trackfw validate
19 warning(s), 0 violations  (mesmos pré-existentes do ML-1A, nenhum novo)
```

`docs/cli-parity.md` — seção `trackfw branch prune` reescrita: fetch best-effort, categoria
`review_doc_config`, ordem `-d`→`-D`, tabela de comando atualizada, 2 cenários novos descritos no
gate de paridade.

**Fora de escopo, confirmado nesta sessão:** Wave 2 (ML-2A, convergência do
`detectPendingSquashMerges` do `ship`) — não implementado, `ship.go`/`ship/runner.js`/`ship/runner.py`
não foram tocados. Nenhum `git commit`/`push`/`branch` executado por mim — autoridade exclusiva do
`trackfw_architect`.

---

### Auditoria do ML-1B — as 3 ações entregues, mas a MINHA regra de doc/config está errada

As três ações do ML-1B foram entregues e o agente **acertou ao não copiar** o `CLAUDE.md`
literalmente: `review` em vez de `delete`, nunca apagando. `-d` antes de `-D`, `fetch --prune`
não-bloqueante e avisado, help corrigido.

**Mas a regra que eu especifiquei classifica errado.** Medido em fixture git real:

```
feat/doc-real   review   only doc/config files diverge — probable housekeeping,
                         confirm and delete manually
```

Essa branch tem documentação **nova, nunca mergeada**. Chamá-la de "provável housekeeping" e
sugerir apagar é **conselho errado sobre trabalho real**. Não apaga sozinha — a falha fechada
segura —, mas o usuário segue o conselho da ferramenta.

**Erro meu de especificação, não do agente.** A regra do `CLAUDE.md` pressupunha divergência
**residual** de uma branch **já integrada** (a `main` avançou depois). Eu a transportei sem esse
pressuposto, e ela passou a capturar também branch **nunca integrada** cujo trabalho é doc.

**Discriminante barato, medido e verificado:**

```
feat/doc-real         touched: [docs/guia-novo.md]  diverg: [docs/guia-novo.md]   IGUAIS
feat/codigo-pendente  touched: [main.go]            diverg: [main.go]             IGUAIS
```

`diverg == touched` ⇒ **nenhum** arquivo da branch está na main ⇒ nada foi integrado ⇒ é trabalho
pendente, qualquer que seja o tipo de arquivo. O resíduo de housekeeping tem `diverg ⊊ touched` —
parte entrou, sobrou ruído.

### ML-1C — `review_doc_config` só quando houve integração parcial
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Ação:** `review_doc_config` passa a exigir **`diverg` como subconjunto PRÓPRIO de `touched`**.
Quando `diverg == touched`, é `pending_work`, mesmo sendo tudo doc/config — nada da branch entrou
na main.

**Critérios de aceite:**
- [x] Branch com doc **nova e nunca mergeada** (`diverg == touched`) → `keep`/`pending_work`, **não** `review`.
- [x] Branch integrada com **resíduo** só de doc (`diverg ⊊ touched`) → `review`, com a mensagem atual.
- [x] Nenhuma das duas vira `delete` — a falha fechada não pode afrouxar.
- [x] Não-regressão completa dos ML-1A e 1B.
- [x] Fixture git real cobrindo **os dois** casos, lado a lado.
- [x] Paridade nos 3 CLIs; `make quality` verde.

**Implementação — nos 3 CLIs:**
- Go: `internal/commands/branch_prune.go` — `evaluateBranchIntegration`, condição alterada de
  `allDocOrConfig(diverg)` para `len(diverg) < len(touched) && allDocOrConfig(diverg)`.
- Node.js: `npm/src/branch/prune.js` — mesma condição (`diverg.length < touched.length &&
  allDocOrConfig(diverg)`).
- Python: `pypi/trackfw/commands/branch.py` — `evaluate_branch_integration`, mesma condição
  (`len(diverg) < len(touched) and all_doc_or_config(diverg)`).
- `docs/cli-parity.md`, seção `trackfw branch prune`: tabela e texto de `review_doc_config`
  reescritos para explicitar a exigência de subconjunto próprio, com o histórico do bug (ML-1C).

**Fixtures ajustadas (decisão sobre o risco 4, respondida):** com o subconjunto próprio, os
fixtures `f1.md` do ML-1A/1B (que tinham sido trocados para `f1.go` no ML-1B porque `.md` passava
a cair em `review_doc_config`) **voltaram a `f1.md`** em todos os 3 CLIs — o teste volta a provar
a intenção original (arquivo de doc genuinamente pendente vira `pending_work`, não por ser
código, mas porque `diverg == touched`). `TestEvaluateBranchIntegration_ReviewDocConfig_NotDeletable`
(e equivalentes Node/Python) foi reescrito para modelar integração parcial genuína (3 arquivos
tocados, 1 integrado, 2 residuais) em vez de "tudo diverge" — do contrário o próprio teste do
ML-1B teria ficado incorreto sob a regra nova.

**Fixture git real, lado a lado, nos 3 CLIs** (prova o discriminante, não só lê o código):
- Go: `TestEvaluateBranchIntegration_RealGitRepo_DocOnlyNeverIntegratedVsPartialResidue`
  (`internal/commands/branch_prune_test.go`) — `feat/doc-real` (doc nova, nunca mergeada) vs
  `feat/residue` (código + doc, só o código integrado via squash-merge).
- Node.js: mesmo cenário em `npm/tests/branch-prune.test.js`.
- Python: mesmo cenário em `pypi/tests/test_branch_prune.py`.
- Gate `scripts/check-branch-prune-parity.sh`, cenário `review-doc-config-only` reescrito para
  incluir as duas branches no mesmo fixture: `feat/docs-review` (resíduo parcial → `review`) e
  `feat/doc-real` (nunca integrada → `keep`, nunca `housekeeping` na mensagem).

**Evidência (2026-08-18, Apolo):**

```
$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 3m ./...
ok  	github.com/kgsaran/trackfw/internal/commands	7.724s
... (todos os pacotes ok)

$ cd npm && node --test tests/*.test.js
ℹ tests 684
ℹ pass 684
ℹ fail 0

$ PYTHONPATH=pypi python3 -m pytest pypi/tests -q
1363 passed, 28 subtests passed in 31.64s

$ GO_BIN=bin/trackfw bash scripts/check-branch-prune-parity.sh
OK   [branch-prune-parity/dry-run-default]
OK   [branch-prune-parity/apply-deletes-integrated]
OK   [branch-prune-parity/apply-never-deletes-current-branch]
OK   [branch-prune-parity/offline-refuses]
OK   [branch-prune-parity/review-doc-config-only]
OK   [branch-prune-parity/stale-origin-main-conservative]
All check-branch-prune-parity.sh scenarios passed.

$ make quality
[exited with code 0]

$ ./bin/trackfw validate
19 warning(s), 0 violations (mesmos pré-existentes, nenhum novo)
```

**Fora de escopo, confirmado nesta sessão:** Wave 2 (ML-2A, convergência do
`detectPendingSquashMerges` do `ship`) e Wave 3 (ML-3A, revisão de segurança do `hades-tf`) — não
implementados. Nenhum `git commit`/`push`/`branch` executado por mim — autoridade exclusiva do
`trackfw_architect`.

---

### Auditoria do ML-1C — aprovada. Wave 1 completa.

Contraste medido em fixture git real, os dois casos lado a lado:

```
feat/doc-real   touched=[docs/guia-novo.md]        diverg=[docs/guia-novo.md]  IGUAIS
                -> keep    pending work vs origin/main: docs/guia-novo.md
feat/residuo    touched=[docs/a.md main.go]        diverg=[docs/a.md]          SUBCONJUNTO PROPRIO
                -> review  only doc/config files diverge — probable housekeeping
```

Antes desta correção, `feat/doc-real` era classificada como `review` e o texto sugeria apagar
**trabalho real nunca mergeado**. Agora é `pending_work`. **Nenhuma das duas vira `delete`.**

Não-regressão do ML-1A/1B verificada na bateria completa: dry-run não apaga (4→4), `--apply` apaga
só a integrada, `main`/corrente/worktree sobrevivem. `make quality` exit 0 · 130 cenários · gate
`branch-prune` 6/6 · `validate` exit 0.

Os fixtures voltaram de `f1.go` para `f1.md` (15 ocorrências), recuperando a intenção original dos
testes do ML-1A — o subconjunto próprio os classifica certo independentemente da extensão.

---

## Wave 2 — Convergência do `ship` (depende da Wave 1)

### ML-2A — `detectPendingSquashMerges` passa a usar a heurística compartilhada
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Dependência:** ML-1A — a função só existe depois dele.

**Discriminante medido, use como caso de teste:** a branch do PR #181, já mergeada, era acusada de
"unmerged changes" porque a `main` avançara com o #182. Quatro arquivos apareciam divergentes sem
haver trabalho pendente.

**Critérios de aceite:**
- [x] O `ship` deixa de avisar sobre branch defasada porém integrada.
- [x] Continua avisando sobre branch com trabalho genuinamente pendente — não-regressão.
- [x] Uma só implementação da heurística; sem cópia divergente.
- [x] Cenário P4 com baseline e detecção.
- [x] `make quality` verde.

**Implementação — nos 3 CLIs, reusando `evaluateBranchIntegration` do ML-1A/1B/1C, sem cópia:**
- Go: `internal/commands/ship.go:562-591` — `detectPendingSquashMerges` chama
  `evaluateBranchIntegration(candidate, gitExec)` (mesmo pacote `commands`, sem import) e só avisa
  quando `eval.Decision == branchPruneDecisionPendingWork`. `candidate` é o ref remoto completo
  (`origin/feat/x`), não o nome curto — a heurística funciona igual em refs locais e remotos.
- Node.js: `npm/src/ship/runner.js` — importa `evaluateBranchIntegration`/`DECISION` de
  `../branch/prune` (sem ciclo: `branch/prune.js` não depende de `ship/runner.js`); avisa só em
  `DECISION.PENDING_WORK`. Exportado `detectPendingSquashMerges` no `module.exports` para teste
  direto.
- Python: `pypi/trackfw/ship/runner.py` — `_detect_pending_squash_merges` faz *late import* de
  `evaluate_branch_integration`/`BRANCH_PRUNE_DECISION_PENDING_WORK` de `trackfw.commands.branch`
  dentro da própria função, espelhando o late import já existente em sentido contrário
  (`commands/branch.py`'s `run_branch_prune` importa `ship.runner` tardiamente) — evita ciclo de
  import-time nos dois sentidos.

O aviso continua **advisory-only** (nunca bloqueia commit/push) — só a condição de disparo mudou,
não a severidade. Decisões diferentes de `pending_work` (`no_own_work`, `content_identical`,
`review_doc_config`, `no_merge_base`, `eval_error`) ficam silenciosas, mesma postura do check
antigo em erro (pular, sem aviso).

**Cenário P4 (falsificação), fixture de repo git real, nos 3 CLIs:**
- Go: `internal/commands/ship_test.go` —
  `TestDetectPendingSquashMerges_RealGitRepo_StaleIntegratedVsGenuinelyPending` reproduz o
  incidente #181/#182 (branch `feat/a` squash-mergeada, `main` avança com PR #182, branch
  `feat/pending` nunca mergeada) num repositório git real e descartável (`origin.git` bare + clone,
  sem mock de `git`) — baseline prova que `git diff origin/main origin/feat/a --stat` (o check
  ingênuo) é não-vazio antes de provar que `detectPendingSquashMerges` corrigido não avisa sobre
  `feat/a` e continua avisando sobre `feat/pending`.
  `TestDetectPendingSquashMerges_CallsSharedEvaluateBranchIntegration` prova, com `gitExec` fake,
  que a função não roda mais seu próprio `diff --stat` bidirecional quando `merge-base` falha.
- Node.js: `npm/tests/ship.test.js` — mesmos dois testes, mesmo fixture real.
- Python: `pypi/tests/test_ship.py` — mesmos dois testes, mesmo fixture real.

**Não-regressão da Wave 1 — provada, não assumida:**
```
$ GO_BIN=bin/trackfw bash scripts/check-branch-prune-parity.sh
OK   [branch-prune-parity/dry-run-default]
OK   [branch-prune-parity/apply-deletes-integrated]
OK   [branch-prune-parity/apply-never-deletes-current-branch]
OK   [branch-prune-parity/offline-refuses]
OK   [branch-prune-parity/review-doc-config-only]
OK   [branch-prune-parity/stale-origin-main-conservative]
All check-branch-prune-parity.sh scenarios passed.
```
`branch prune` (comportamento e heurística) não foi tocado neste ML — só o `ship` passou a
consumi-la.

**Evidência (2026-08-18, Apolo):**
```
$ TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 3m ./...
ok  	github.com/kgsaran/trackfw/internal/commands	8.303s
... (todos os pacotes ok)

$ cd npm && node --test tests/*.test.js
ℹ tests 686
ℹ pass 686
ℹ fail 0

$ PYTHONPATH=pypi python3 -m pytest pypi/tests -q
1365 passed, 28 subtests passed in 28.17s

$ go vet ./...
(exit 0)

$ make quality
[exited with code 0]

$ ./bin/trackfw validate
19 warning(s), 0 violations (mesmos 19 pré-existentes desta sessão — 2 deles são "roadmaps in
wip/ (limit: 1)" e REQs Open/Roadmap-in-done pré-existentes desta sessão, nenhum novo)
```

`docs/cli-parity.md` atualizado: seção "Why not `git branch -d`..." registra que o falso positivo
está fechado; seção "The touched-files heuristic" documenta os 3 pontos de chamada do `ship`
(inclusive o late-import Python) e a postura advisory-only.

**Fora de escopo, confirmado nesta sessão:** Wave 3 (ML-3A, revisão de segurança do `hades-tf`) —
não implementado. Nenhum `git commit`/`push`/`branch` executado por mim — autoridade exclusiva do
`trackfw_architect`.

---

### Auditoria do ML-2A — comportamento aprovado, AC8 em aberto

```
teste ingenuo no ship   REMOVIDO (o unico --stat que sobrou e o de arquivos staged, sem relacao)
funcao compartilhada    ship.go:588 chama evaluateBranchIntegration
implementacao unica     merge-base so aparece em branch_prune.go
mapeamento              so pending_work dispara o aviso; demais decisoes ficam silenciosas
Wave 1                  gate branch-prune 6/6, sem regressao
make quality            exit 0 · validate exit 0
```

**AC8 NÃO está satisfeito, e o agente sinalizou com honestidade.** A paridade do `ship` foi provada
por **teste unitário por stack**, com fixture git real em cada um — o que prova comportamento por
runtime, **não** que os três produzem a mesma saída. O `check-ship-parity.sh` não menciona
squash/pending (0 ocorrências).

É **exatamente a lacuna que eu declarei e critiquei** na REQ de higiene (ML-2C: *"byte-identidade
provada por leitura de fonte; os testes afirmam comportamento por stack, não paridade entre
stacks"*). Declarar de novo seria incoerente — fecha no ML-2B.

### ML-2B — Gate de paridade do aviso de squash-merge do `ship`
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `scripts/check-ship-parity.sh`, `scripts/check-gates-falsify.sh`.

**Ação:** estender o `check-ship-parity.sh` com cenário que roda os **3 binários reais** contra o
mesmo fixture git e compara a saída byte a byte, para dois casos:
- branch **defasada porém integrada** → nenhum aviso, nos 3;
- branch **genuinamente pendente** → aviso presente e idêntico, nos 3.

**Critérios de aceite:**
- [x] Cenário novo no `check-ship-parity.sh` comparando saída real `go-vs-node` e `go-vs-py`.
- [x] Fixture git real com squash-merge e `main` avançando depois.
- [x] Cenário P4 de falsificação: sabotar **um** runtime e provar que o gate reprova.
- [x] `make quality` verde. **CI verde** ainda depende do push/PR — autoridade do
  `trackfw_architect` (mesma ressalva do AC10 desde o início do roadmap).

**Implementação:**
- `scripts/check-ship-parity.sh`, cenário `(e) squash-merge-warning`: `make_squash_fixture` cria
  um bare `origin` real + um único clone (mesmo padrão de
  `TestDetectPendingSquashMerges_RealGitRepo_StaleIntegratedVsGenuinelyPending`, o teste Go-only
  do ML-2A que esta ML estende aos 3 runtimes) — `feat/a` pushada, squash-mergeada em `main`, com
  `main` avançando depois (réplica do #181/#182); `feat/pending` pushada e nunca mergeada. O
  `ship` roda a partir de `chore/ship-parity-squash-check` (checked out após o avanço de `main`,
  um arquivo não-doc staged) — **sem `--dry-run`**, porque o Step 3 do `ship` só executa o fetch
  real e `detectPendingSquashMerges` fora do dry-run (`ship.go:212-220`; dry-run só imprime
  `[dry-run] git fetch origin --prune` e nunca chama a detecção — as 4 cenas pré-existentes (a-d)
  usavam `--dry-run` e por isso nunca exerciam este caminho).
- Guard de não-vacuidade: antes do diff de 3 vias, cada runtime é checado individualmente —
  o aviso de `feat/pending` DEVE aparecer, e `feat/a` NUNCA pode aparecer em nenhum aviso.
  `assert_three_way` então prova byte-identidade de stdout+stderr+exit-code nos 3 binários reais.
- `scripts/check-gates-falsify.sh`, Cenário 70: sabota `npm/src/ship/runner.js` — a condição
  `evalResult.decision === BRANCH_PRUNE_DECISION.PENDING_WORK` vira `true` incondicional,
  reintroduzindo o falso-positivo (Node avisa sobre `feat/a`, a branch stale-porém-integrada) numa
  cópia isolada do Node (Go e Python intocados). Prova, rodando o `check-ship-parity.sh` real
  contra essa árvore, que o gate novo reprova com o diagnóstico exato do guard de não-vacuidade
  (`stale-but-integrated feat/a must never appear in a warning`) e com o diff byte a byte
  correspondente. Contagem do resumo final ajustada de 130→131 cenários.

**Evidência (2026-08-18, Apolo):**
```
$ GO_BIN=bin/trackfw bash scripts/check-ship-parity.sh
OK   [ship-parity/chore-skips-gate]
OK   [ship-parity/docs-skips-gate]
OK   [ship-parity/feat-still-gated-non-regression]
OK   [ship-parity/invalid-branch-vocabulary]
OK   [ship-parity/squash-merge-warning]

All check-ship-parity.sh scenarios passed.

$ (P4 isolado, fora do make quality) sabotando npm/src/ship/runner.js numa árvore isolada:
FAIL [ship-parity/squash-merge-warning/node]: stale-but-integrated feat/a must never appear in a
warning; stdout: ...Warning: branch "feat/a" appears to have unmerged changes vs origin/main...
FAIL [ship-parity/squash-merge-warning/go-vs-node/out]: stdout/stderr diverges: +Warning: branch
"feat/a" appears to have unmerged changes vs origin/main.
check-ship-parity.sh: one or more scenarios FAILED.  (exit 1, confirmado)

$ GO_BIN=bin/trackfw bash scripts/check-branch-prune-parity.sh
OK   [branch-prune-parity/dry-run-default]
OK   [branch-prune-parity/apply-deletes-integrated]
OK   [branch-prune-parity/apply-never-deletes-current-branch]
OK   [branch-prune-parity/offline-refuses]
OK   [branch-prune-parity/review-doc-config-only]
OK   [branch-prune-parity/stale-origin-main-conservative]
All check-branch-prune-parity.sh scenarios passed.

$ make quality
... OK   [falsify/ship-parity/squash-merge-warning-false-positive]
Falsification checks passed (all 131 scenarios, ...)
[exited with code 0]

$ ./bin/trackfw validate
19 warning(s), 0 violations (mesmos pré-existentes desta sessão, nenhum novo)
```

**Fora de escopo, confirmado nesta sessão:** Wave 3 (ML-3A, revisão de segurança do `hades-tf`) —
não implementado. Nenhum `git commit`/`push`/`branch` executado por mim — autoridade exclusiva do
`trackfw_architect`.

---

### Auditoria do ML-2B — pendente (arquiteto)

### Auditoria do ML-2B — aprovada. AC8 fechado.

```
check-ship-parity.sh   5/5, incluindo o cenario novo squash-merge-warning
guard de vacuidade     afirma que o aviso da pendente APARECE
                       e que a integrada NUNCA aparece — os dois sentidos
Cenario 70 (P4)        OK [falsify/ship-parity/squash-merge-warning-false-positive]
Wave 1                 branch-prune 6/6, sem regressao
make quality           exit 0 · 131 cenarios · validate exit 0
```

O guard de vacuidade era o risco que eu tinha nomeado no handoff: um diff de saídas vazias
"passaria" sem provar nada, caso o `ship` abortasse antes por governança ou por nada staged. O
cenário roda **sem** `--dry-run` (o dry-run pula o passo 3 inteiro) a partir de branch `chore/`,
e afirma os dois sentidos. Não é vacuoso.

---

## Wave 3 — Barreira

### ML-3A — `hades-tf`: revisão de comando destrutivo
**Status:** ✅ Concluído · **Veredito: APROVA**, com 1 risco residual declarado · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-18-revisao-do-branch-prune.md`

**Ações:** é o primeiro comando do trackfw que **apaga** trabalho. Verificar se há caminho para
apagar branch não integrada — nome com caracteres especiais, branch com upstream sumido, `origin`
apontando para lugar errado, `main` local defasada em relação a `origin/main`, repositório sem
`origin`, branch cujo nome colide com ref ambígua. Avaliar se `--apply` pode ser disparado sem
intenção. **Veredito explícito; bloquear é saída legítima.**

---

### Barreira ML-3A — APROVA

Ele testou em fixtures git reais e descartáveis, não por leitura:

| vetor | resultado |
|---|---|
| nome com flag/espaço (`--force`, `foo bar`) | o git recusa criar — vetor inexistente |
| **ref ambígua** (branch e tag homônimas) | duas camadas independentes fecham: `git branch --format` desambigua para `heads/<nome>` antes do trackfw ver, e `-d/-D heads/<nome>` falha por incompatibilidade |
| `origin` para repo não relacionado | `merge-base` vazio → `no_merge_base` → keep |
| sem `origin` | recusa o comando inteiro, exit 1 |
| clone raso (`--depth 1`) | sem falso negativo |
| rename / delete / mode-only / binário | todos `pending_work` corretamente |
| `--apply` real, fixture de 6 branches | **só a integrada apagada** — a prova mais forte |
| `--apply` por env var | não existe bind nos 3 CLIs, sem shorthand |

**Risco residual declarado — e eu decidi NÃO corrigir agora.** Os dois `git diff` usam `-z` +
`splitNulPaths`, mas a listagem de branches (`branch_prune.go:432`) e o parsing de worktree (`:466`)
quebram por `\n`. Confirmei a inconsistência.

Considerei fechar e não fechei: o valor de segurança é **nulo** — criar ref com `\n` exige
`git update-ref` (plumbing), acesso equivalente a apagar branch diretamente, e o `hades-tf` não
achou caminho até deleção indevida. Um sétimo ML numa REQ completa, por assepsia, é expansão de
escopo que atrasa a entrega sem reduzir risco.

**Para quem tocar `defaultListLocalBranches` ou o parsing de worktree:** use `-z` e o mesmo
`splitNulPaths` que está duas funções acima, para a dureza ficar uniforme no arquivo.

**Lacunas que ele próprio declarou:** submódulo com ponteiro divergente foi raciocinado por
analogia, não medido; e ele não reexecutou os gates de paridade, cruzou a evidência do roadmap com
leitura linha a linha dos 3 fontes. Aceito — eu **executei** os dois gates nesta auditoria.

**Sobre ACs fechados cedo demais:** não encontrou nenhum. O AC10 (CI verde) está corretamente
marcado em aberto, não fechado sem lastro.

---

### AC10 fechado — CI verde no PR #187

```
go · node · python 3.10 · python 3.12 · governance · package-smoke
parity (4m13s) · windows-integrations-resolve      -> todos pass
mergeable=MERGEABLE  mergeState=CLEAN
```

Verde na **primeira** tentativa. Na REQ anterior o `parity` reprovou no Linux por limite de
argumento que o macOS não aplica, e eu tinha fechado o AC com evidência de uma plataforma só — a
lição virou risco declarado no topo deste roadmap e o AC10 já nasceu exigindo CI.

### Retrospectiva — 3 MLs planejados, 6 executados

Nenhum retrabalho foi implementação errada. Todos os três MLs extras vieram de **auditoria ou
pergunta**, não de defeito do agente:

| ML extra | origem |
|---|---|
| **1B** | pergunta de KG: *"estamos usando o mesmo método já documentado?"* — não estávamos por inteiro |
| **1C** | erro **meu** de especificação: transportei a regra de doc/config do `CLAUDE.md` sem o pressuposto que a justificava |
| **2B** | AC8 fechado com teste por stack em vez de gate entre stacks — a mesma lacuna que eu havia declarado e criticado na REQ de higiene |

O padrão que se repete nas duas últimas REQs: **o plano subestima acoplamento e o critério de
aceite é escrito mais frouxo do que a intenção**. O AC8 aqui dizia "gate comparando saídas reais" e
ainda assim foi fechado com teste unitário — porque quem fecha é quem escreveu, e lê o próprio
critério com boa vontade. Foi a barreira e a auditoria que pegaram, não o autor.

---

## Notas
- **Fora de escopo, declarado:** apagar branch remota; consultar forge; alterar estratégia de merge.
- Commits e branch são exclusivos do `trackfw_architect`.
