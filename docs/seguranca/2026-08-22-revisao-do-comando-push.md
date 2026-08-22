# Revisao de Seguranca: `trackfw push` (ML-3B)

**Data:** 2026-08-22
**Revisor:** Hades (Security)
**Branch auditada:** `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`
**Roadmap:** `ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md`
**ADR de referencia:** `ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`

---

## VEREDICTO

**APROVADO COM RESSALVAS**

Todas as 5 perguntas da barreira recebem resposta positiva com base em evidencia medida. Uma ressalva nomeada — ausencia de cenario de falsificacao ponta-a-ponta para o gate `--force-with-lease` — e registrada como debito tecnico declarado, nao como bloqueador. Nenhum vetor de bypass reachable foi encontrado em nenhum dos 3 runtimes.

---

## Metodo

Cada achado distingue:
- **Medido:** grep com saida literal, leitura direta de arquivo+linha, execucao de comando com saida observada.
- **Inferido:** raciocinio estrutural a partir de codigo lido, sem execucao de caminho especifico.

---

## Q1 — Algum gate do `ship` foi perdido no `push`?

**Resposta: Nao. Todos os 4 gates estao presentes na ordem correta.**

| Gate | Arquivo `ship.go` (referencia) | Arquivo `push.go` | Node `push/runner.js` | Python `push/runner.py` |
|---|---|---|---|---|
| 1. main/master bloqueado | linha 270 | linha 127-131 | medido | medido |
| 2. `isShipBranch` | linha 729 | linha 134-140 | importado de ship/runner.js | `is_ship_branch` importado de ship.runner |
| 2. `isGatedShipBranch` + `CheckShipGovernance` | linha 740 + 234-243 | linha 147-170 | `isGatedShipBranch` + `checkShipGovernance` importados | `is_gated_ship_branch` + `check_ship_governance` importados |
| 2.5 force-with-lease gate | linhas 310-420 | linhas 172-207 | presente | presente |
| 3. `detectPendingSquashMerges` (advisory) | presente | linha 217 | importado | `_detect_pending_squash_merges` importado |

**Evidencia medida — Go:**
- `push.go:90`: `checkGovernance: defaultCheckGovernance`
- `ship.go:234-243`: `defaultCheckGovernance` chama `validator.CheckShipGovernance()` — mesma funcao que ship usa.
- `push.go:147`: `if !isGatedShipBranch(branch)` — funcao definida em `ship.go:740`, chamada sem modificacao.

**Diferenca documentada — `push` e mais estrito que `ship` em um ponto:**
`ship` tem caminho `allDocOnly` (arquivos staged todos em `docs/`) que permite avanco sem governance para feat/fix/refactor. `push` nao le o index (`git diff --cached`) e portanto nao tem essa excecao.
- **Direcao:** `push` e mais restritivo, nao mais permissivo. Nao e uma perda de gate.
- **Racional correto:** `push` opera sobre commits ja criados, nao sobre staged files. A excecao doc-only de `ship` serve ao caso "vou commitar so docs" — que em `push` nao existe.

---

## Q2 — `--force-with-lease` ainda exige PR/MR aberto e ha algum caminho de bypass?

**Resposta: Sim, ainda exige. Nenhum bypass reachable encontrado nos 3 runtimes.**

### Caminho Go (`push.go:172-207`)

```
forge.Resolve(Input{FlagForge: "", ConfigForge: ..., RemoteURL: ..., RepoDir: ...})
  → se Forge == "manual" || !adapter.Available  → erro hard (pushForceLeaseNoForgeCLIMsg)
  → checkPROpen(adapter, branch)
      → se erro                                 → erro hard (pushForceLeaseCannotVerifyFmt)
      → se !open                                → erro hard (pushForceLeaseNoPROpenFmt)
  → prossegue apenas se PR/MR confirmado aberto
```

**Evidencia medida:**
- `push.go:183`: `FlagForge: ""` — campo fixo em string vazia. `push` nao tem flag `--forge`. Ship tem (`ship.go:99`); push deliberadamente nao tem (ADR, secao "sem --forge em push").
- `forge/resolve.go`: `Resolve` retorna `{Forge: "manual"}` sem erro quando nenhum forge e detectavel. Esse caso e capturado explicitamente em `push.go:190`: `resolution.Forge == "manual"` → erro. O caminho `resErr != nil` sozinho nao seria suficiente — mas nao e o unico check.
- `adapter.go:24`: `if os.Getenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS") == "1"` → `availFn` retorna `false` → `Available = false` (linhas 47, 55, 63) → gate recusa. A variavel de ambiente e fail-closed para o gate de force.

### Caminho Node (`npm/src/push/runner.js`)

`defaultCheckPROpen` (definida em `npm/src/ship/runner.js:47-88`, importada por push) usa `spawnSync` e lanca excecao em qualquer erro (`result.status !== 0` ou `!Array.isArray(parsed) || parsed.length === 0`). O gate em push/runner.js segue o mesmo padrao de ship: `if (!open)` → retorno 1. Nenhuma flag `--forge` no wrapper `npm/src/commands/push.js`.

### Caminho Python (`pypi/trackfw/push/runner.py`)

`default_check_pr_open` (definida em `pypi/trackfw/ship/runner.py:54-93`, importada) usa `subprocess.run` e levanta `RuntimeError` em qualquer erro. Gate segue o mesmo padrao. `allow_abbrev=False` em `pypi/trackfw/commands/push.py` impede que `--force` seja aceito como abreviacao de `--force-with-lease` (Go e Node nao abreviam por padrao).

### Vetor de env var TRACKFW_DISABLE_EXTERNAL_COMMANDS

**Medido:** `adapter.go:24` — quando `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`, o `availFn` retornado faz `os.LookPath` retornar erro, resultando em `Available=false`. O gate em `push.go:190` testa `!adapter.Available` antes de chamar `checkPROpen`. Resultado: force-with-lease e recusado quando a variavel esta ativa. **Fail-closed.**

### Ressalva de cobertura (nao um bypass)

`check-push-force-parity.sh` **nao existe** (`check-ship-force-parity.sh` existe e usa bare-origin real). O `cli-parity.md` linha 1209 declara explicitamente: `partial=` "o push real, o caminho --force-with-lease e a deteccao de squash-merges com fetch real nao sao exercitados ponta a ponta."

Nenhum cenario de parity testa `--force-with-lease` end-to-end. Isso nao e um bypass — o gate existe e e fail-closed — mas significa que uma refatoracao futura que remova o gate silenciosamente nao seria detectada pela parity gate. Ver secao Debito Tecnico.

---

## Q3 — A string REASON atualizada do guard ensina algum caminho nao governado?

**Resposta: Nao. Todos os caminhos listados no REASON tem verificacao mais alta que o `git push` bruto que substituem.**

**Evidencia medida — `scripts/trackfw-git-branch-guard.sh`:**

- **REASON para `git push`** (linha ~528): lista `trackfw push` (governance + branch-name gate), `trackfw ship` (commit+push+PR), `trackfw release tag` (tag governada). Todos exigem gates antes de escrever no remoto.
- **REASON para `git reset --hard`** (linha ~534): lista `trackfw ship -m "..."` — corretamente nao lista `trackfw push`. Motivo: apos `reset --soft` o trabalho esta staged, sem commit; `trackfw push` empurra commits ja criados e nao operaria sobre trabalho staged. A escolha de vocabulario e tecnicamente correta.
- **Comentario interno** (linha ~347, dentro do bloco reset): tambem cita `trackfw ship -m "..."`. Correto pelo mesmo motivo.

**Nenhuma "escada de downgrade":** o REASON nao cita `git push --force-with-lease` direto, nem variantes sem verificacao. O caminho de escalada (force-with-lease) esta documentado dentro do `--help` de `trackfw push`, nao no REASON do guard. O REASON do guard continua direcionando para comandos governados.

---

## Q4 — O guard ainda e fail-closed apos edicao em 7 arquivos?

**Resposta: Sim, com uma qualificacao sobre a contagem de arquivos.**

### Contagem medida

Grep de `"trackfw push"` nos locais esperados para o REASON string:

```
scripts/trackfw-git-branch-guard.sh                              (canonical)
internal/validator/validator_git_branch_guard_reference.go       (referencia Go)
pypi/trackfw/validator.py                                        (referencia Python)
internal/generators/scaffold.go                                  (generator Go)
npm/src/generators/hooks.js                                      (generator Node)
pypi/trackfw/generators/init_gen.py                             (generator Python)
```

**Resultado:** 6 arquivos distintos encontrados, 7 ocorrencias (um arquivo tem 2 mencoes). O `npm/src/validator/index.js` nao contem referencia ao script do guard (ele valida artefatos de governance: roadmaps, REQs — nao hash de hook). A contagem "7 arquivos" da narrativa do roadmap provavelmente conta as 7 ocorrencias, nao 7 arquivos distintos.

**Conclusao:** a copia canonica e todas as copias que o harness instala (via generators + validator de referencia) foram atualizadas. O `npm/src/validator/index.js` nao e uma copia do script — nao e relevante para este contagem.

### Copia global `~/.trackfw/scripts/` (intencional)

A copia em `~/.trackfw/scripts/trackfw-git-branch-guard.sh` **nao foi atualizada** por esta ML. Ela ainda exibe REASON citando `trackfw ship` para `git push`, em vez de listar `trackfw push` como alternativa.

**Impacto:** funcional — a copia ainda bloqueia `git push` com exit 2. O efeito de seguranca (bloquear push bruto) e preservado. O impacto e pedagogico: o usuario instalado verifica uma mensagem ligeiramente menos util ate executar `trackfw update harness`. O `trackfw validate` emite `+1 git_branch_guard_script_integrity` warning detectando a divergencia — mecanismo de deteccao funcionando corretamente.

**Remediacao:** `trackfw update harness` (decisao de KG, fora do escopo desta ML).

### Fail-closed

O guard bloqueia `git push` e `git reset --hard` via exit 2, independentemente do estado das copias instaladas. A logica de fall-through para `exit 0` (tokens nao reconhecidos) e design intencional para comandos git nao governados — nao e uma porta de saida para push ou reset-hard, que sao testados antes do fall-through.

---

## Q5 — `push` nunca commita e nunca abre PR?

**Resposta: Correto. Verificado nos 3 runtimes.**

### Evidencia medida — grep negativo nos 5 arquivos de push

Grep de `'"--force"'` (flag de force bruto) em todos os 5 arquivos de push: **nenhum resultado** (exit=1).

Grep de `'pr create|openPR|create_pr|CreatePR|openMR|create_mr|pr_create'` em todos os 5 arquivos de push: **nenhum resultado** (exit=1).

Arquivos verificados:
- `internal/commands/push.go`
- `npm/src/push/runner.js`
- `pypi/trackfw/push/runner.py`
- `npm/src/commands/push.js`
- `pypi/trackfw/commands/push.py`

### Confirmacao estrutural

- Go `push.go`: nao tem flag `-m`, nao tem chamada a `runCommit` ou equivalente, nao tem chamada a qualquer funcao de abertura de PR. O unico `git` write que alcanca o remoto e `push` na Step 4.
- Node `npm/src/push/runner.js`: importa 6 simbolos de `ship/runner.js` (`isShipBranch`, `isGatedShipBranch`, `isGitWriteCmd`, `checkShipGovernance`, `detectPendingSquashMerges`, `defaultCheckPROpen`). Nenhum deles escreve commits ou abre PR. `buildPushArgs` e `defaultExecGit` sao reimplementados localmente — nenhum abre PR.
- Python `push/runner.py`: importa `_detect_pending_squash_merges` e `_build_push_args` de `trackfw.ship.runner` (simbolos privados). Nenhum abre PR.
- `npm/src/commands/push.js`: flags parseadas sao apenas `--dry-run` e `--force-with-lease`. Sem `-m`, sem flag de PR.
- `pypi/trackfw/commands/push.py`: mesmas duas flags. `allow_abbrev=False` explicito.

### Falsificacao (cenario 162)

O script `scripts/check-gates-falsify.sh` cenario 162 modifica `push.go` para emitir uma chamada de abertura de PR e confirma que `check-push-parity.sh` detecta a divergencia. Isso prova que o gate de parity e sensivel a introducao de PR-opening em Go. (Cobertura de Node e Python e indireta — via `check-push-parity.sh` que compara saida dos 3 runtimes byte-a-byte em 5 cenarios.)

---

## Debito Tecnico Declarado (nao bloqueia entrega)

### DT-1 — Ausencia de `check-push-force-parity.sh` com bare-origin real

**Severidade:** Media.

**Estado atual:** `check-ship-force-parity.sh` existe e usa um repositorio bare real para testar force-with-lease end-to-end em Go + Node + Python. O `push` nao tem equivalente.

**Risco concreto:** uma refatoracao futura que remova ou inverta o check `if !open` em qualquer dos 3 runtimes nao seria detectada automaticamente. Os cenarios existentes usam `--dry-run`; o gate 2.5 roda antes de qualquer write e tambem em `--dry-run` — mas o cenario de parity nao testa `forceWithLease=true` explicitamente.

**Remediacao recomendada:** criar `scripts/check-push-force-parity.sh` com o mesmo padrao de `check-ship-force-parity.sh` — bare-origin, cenario `forge-nao-verificavel` e cenario `pr-nao-aberto`. Adicionar como cenario 163 em `check-gates-falsify.sh`.

**Referencia:** `docs/cli-parity.md` linha 1209, anotacao `partial=`.

### DT-2 — Pedagogia desatualizada na copia global `~/.trackfw/scripts/`

**Severidade:** Baixa.

O REASON de `git push` na copia instalada ainda cita so `trackfw ship`, sem mencionar `trackfw push`. Funcionalidade (bloqueio) preservada. Detectado pelo warning `git_branch_guard_script_integrity`. Remediacao: `trackfw update harness` (decisao do usuario).

---

## Achados Fora de Escopo

### OOE-1 — `buildPushArgs` e `defaultExecGit` nao exportados por `npm/src/ship/runner.js`

**Severidade:** Baixa (observacao arquitetural).

O `npm/src/push/runner.js` reimplementa `buildPushArgs` e `defaultExecGit` localmente porque `ship/runner.js` nao os exporta. Duplicacao controlada, mas se a logica de `buildPushArgs` evoluir (ex: suporte a remote nao-`origin`) ambas as copias precisam ser atualizadas em sincronia. O roadmap registra que isso sera endereçado por `hefesto-tf` em ML futuro. Nenhuma acao de seguranca requerida agora.

**Proprietario do fix:** `hefesto-tf` (codigo de produto).

### OOE-2 — `trackfw push origin main` aceita args posicionais silenciosamente

**Severidade:** Baixa (UX, nao seguranca).

cobra/pflag, commander e argparse aceitam argumentos posicionais extras sem erro quando nao ha parametro obrigatorio. `trackfw push origin main` e ignorado silenciosamente — o push ainda vai para `origin/<branch-atual>` derivado de `symbolic-ref`. Nao e um bypass de seguranca (o refspec e derivado do estado do repo, nao do input do usuario), mas pode confundir o usuario. Verificar se os 3 runtimes concordam neste comportamento e considerar adicionar `Args: cobra.NoArgs` (Go) equivalente.

**Proprietario do fix:** `apolo-tf` ou `hefesto-tf` (codigo de produto).

---

## Resumo Executivo

| Pergunta | Resposta | Nivel de confianca |
|---|---|---|
| Q1 — Gates perdidos? | Nenhum. `push` e mais estrito que `ship` (sem excecao doc-only) | Alto (medido) |
| Q2 — Bypass de force-with-lease? | Nenhum. Fail-closed inclusive via env var DISABLE | Alto (medido) |
| Q3 — REASON ensina caminho nao governado? | Nao. Todos os caminhos no REASON exigem gates iguais ou maiores | Alto (medido) |
| Q4 — Guard fail-closed apos edicao? | Sim. 6 arquivos confirmados; copia global intencional e detectada | Alto (medido) |
| Q5 — Nunca commita, nunca abre PR? | Correto nos 3 runtimes. Grep negativo + falsificacao via cenario 162 | Alto (medido) |

**Ressalva unica:** ausencia de `check-push-force-parity.sh` (DT-1). Nao e um bypass hoje — e uma lacuna de deteccao futura. Remediacao concreta indicada.
