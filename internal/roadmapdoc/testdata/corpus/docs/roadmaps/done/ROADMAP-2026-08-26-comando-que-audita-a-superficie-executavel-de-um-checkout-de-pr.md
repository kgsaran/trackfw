---
status: done
date: 2026-08-26
req: "docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: comando que audita a superficie executavel de um checkout de PR

> Created: 2026-08-26 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md`
ADR: `docs/adr/ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md`

Um checkout de PR hostil executa hook na máquina do mantenedor **sem exigir comando nenhum do
trackfw** — mais amplo que o vetor do `#208`, que exigia rodar `barrier`. **Medido:** `validate` fica
em silêncio diante de hook novo apontando para script novo.


## Acceptance Criteria

- [ ] AC1–AC11 da REQ, integralmente
- [ ] 🔴 **AC5 e AC6 decidem a entrega:** o comando **nomeia o que executa**, não julga se é hostil, e
      **não acusa** wiring legítimo
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça da superfície executável
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md`

**A pergunta que decide o escopo é sua: o que, de fato, executa a partir de um checkout?** O ADR lista
hooks de agente, scripts referenciados, `Makefile` e CI. **Essa lista é a hipótese, não a resposta** —
e a lista dada já errou três vezes na REQ do pin de modelo. Enumere por busca.

Considere, sem se limitar: `.claude/settings.json` e equivalentes dos **6 runtimes** · hooks de git
versionados (`.githooks/`, `core.hooksPath`) · `direnv`/`.envrc` · `package.json` scripts
(`preinstall`, `postinstall`) · `pyproject.toml`/`setup.py` · devcontainer · `.vscode/tasks.json` ·
arquivos de skill e de agente que o CLI de agente lê e interpreta.

**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
grep -q "Completude de enumera" docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
grep -q "Residual declarado" docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
```

### Auditoria do ML-0A — aprovada; **6 ACs novos, e um deles apaga um resíduo do ADR**

**O achado que muda o desenho:** auditar um ref **sem checkout**, via `git show <ref>:<path>`.
O ADR declarava como limite estrutural que o comando *"não protege quem já abriu o repositório"* — a
Wave 0 mostrou que **não precisa ser assim**: sem checkout, a janela vai a **zero**. Virou **AC12**.
É a segunda vez nesta série que uma Wave 0 derruba um resíduo que o ADR dava por inevitável.

**A enumeração corrigiu um número meu:** escopo de **projeto** tem **8** runtimes — medido em
`scripts/check-agent-hooks-parity.sh:90` (`claude codex gemini copilot cursor kiro windsurf amazonq`).
Os 6 do ADR são o escopo **harness/global**. E a instrução que importa: **varrer por padrão de path,
não por presença** — *ausência é informação, não exclusão*. Virou **AC13**.

**As três variantes de diff limpo** que ele nomeou são o coração do AC14:

```
A) so o conteudo do script muda      -> diff do settings.json e ZERO
B) wiring reaponta para outro script -> parece correcao de path
C) matcher "Bash" -> "*"             -> um token muda, o script nao
```

Por isso a unidade reportada tem de ser a **tupla (trigger, matcher, caminho, digest)** — qualquer
componente alterado é superfície.

**E ele achou o falso-positivo antes de existir código, com fixture grátis:** um `grep` por caminho
literal acusaria `docs/cli-parity.md` e `internal/generators/agentfiles.go`, que **mencionam** os
paths sem serem wiring. Discriminante: estar no path do runtime **e** ter estrutura de wiring.
Virou **AC16**.

**Distinção que eu não tinha feito:** arquivos de instrução (`CLAUDE.md`, `AGENTS.md`, slash commands)
**não executam — instruem**, com efeito em comando futuro. Rótulo próprio no relatório (**AC15**).

---

## Wave 1 — O comando

> Dependências: ML-0A auditado. **ML único:** os 3 stacks saem byte-idênticos.

### ML-1A — Comando de auditoria nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

Escopo conforme a enumeração fechada pelo ML-0A. Reusa `validate`/`doctor` para integridade de
artefato gerenciado. **Informa, não acusa; nomeia, não julga.**

**Critérios de aceite:** AC1–AC8 da REQ · `make quality` exit 0 medido

---

### Auditoria do ML-1A — aprovada, **com uma correção minha antes do commit**

#### 🔴 Ele criou um gate-fantasma, e eu troquei por `gap`

`scripts/check-audit-surface.sh` é um **stub que faz `exit 0` sem verificar nada** — e três anotações
do `cli-parity.md` apontavam para ele como `gate=`. Resultado: o
`check-parity-contract-coverage.sh`, que é **exatamente** o controle criado na REQ #196 para impedir
"contrato afirmado sem gate", passava **satisfeito por um chamariz**.

Pior que declarar `gap`, porque `gap` é honesto. Troquei as três para `gap` com o motivo; viram
`gate=` quando os cenários FN-1..5 e FP-1..2 existirem, no ML-2A. `check-parity-contract-coverage.sh`
segue exit 0.

#### O comando, medido por mim

```
$ trackfw audit-surface HEAD~1
9 hook tuple(s)
  hook   [claude] .claude/settings.json PreToolUse/Bash …/trackfw-git-branch-guard.sh sha256:f2e80b0f…
  absent [copilot] .github/hooks/trackfw-attention.json      <- ausencia como informacao
  absent [cursor]  .cursor/hooks.json

git status antes/depois        IDENTICO      <- AC12: sem checkout, worktree intacto
cli-parity/agentfiles no report  0 ocorrencias <- AC16: sem falso-positivo
go == node == py               texto e --json
make quality (CI-exata, minha) exit 0
```

**A variante A eu validei contra a história real do repositório**, em vez de aceitar o fixture dele:

```
615f8f9  git-branch-guard  sha256:bd144a3f85c1ab0f
7132fc5  git-branch-guard  sha256:f2e80b0fa9a48fcc
         mesmo wiring, digest diferente
```

É o ataque em que o diff do `settings.json` é **zero** — e o digest na tupla pega.

**Erro de relatório dele, sem efeito:** apontou a branch como
`fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`, que foi apagada há dias. Os arquivos
estão na branch certa; a citação é que estava velha.

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC9, AC10, AC11 da REQ

---

### Auditoria do ML-2A — aprovada; e a costura de autoteste resolve o problema de fundo da série

**Em vez de sabotar código de produto numa cópia isolada** — que é onde os cenários 170 e 171 se
enrolaram —, ele pôs a costura **no próprio gate**. Rodei as duas:

```
AUDIT_SURFACE_SELFTEST_BREAK=A  -> binario com digest constante
  FAIL [audit-surface/fn-2/digest-changes-when-script-changes]: digest did not change
AUDIT_SURFACE_SELFTEST_BREAK=B  -> binario que estende os paths de instrucao
  FAIL [audit-surface/fp-1/cli-parity-absent]: docs/cli-parity.md appeared in output
sem a variavel -> exit 0

Makefile:47   dentro do alvo parity          <- confirmado por LEITURA
cli-parity    3 anotacoes de volta em gate=
make quality (CI-exata, minha)  exit 0, 174 cenarios
validate                        16 warnings, 0 violations
```

**O gate prova a própria capacidade de detecção nas duas direções** — falso-negativo e
falso-positivo — com mensagens **específicas do defeito**, não genéricas. É exatamente o que faltou
nas duas tentativas anteriores desta série, em que o `assert_fails_with` recusou o padrão por mirar a
mensagem errada.

**Dado de custo, relevante para a decisão de protocolo do KG:** este ML consumiu **44.255 tokens /
95 tool uses**, contra 276k e 290k dos dois maiores da sessão — mesmo tipo de trabalho (gate +
falsificação + paridade nos 3 CLIs). A diferença foi **escopo estreito e alvos já enumerados pela
Wave 0**, não linguagem mais curta.

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem enumerou verifica se a implementação cobre o que ele enumerou. **Veredito explícito.**

---

### Auditoria do ML-3A — REPROVADO parcialmente em AC14; três achados; dois HIGH

**Veredito:** REPROVADO em AC14 para duas formas de comando. Os achados não bloqueiam o repositório
trackfw (as formas de comando usadas aqui são cobertas), mas devem ser registrados como resíduos e
corrigidos numa REQ própria antes de promover `audit-surface` como ferramenta genérica.

**F1 (HIGH) — `normalizeCommand` whitelist:** extensão `.bash`, comando com argumento
(`scripts/hook.sh --strict`), prefixo de interpretador (`bash scripts/hook.sh`) → digest
permanentemente `unresolvable`. Variante A não detectável. Medido: saída idêntica em REF1/REF2
com conteúdo diferente. Handoff: `internal/auditsurface/auditsurface.go::normalizeCommand`.

**F2 (HIGH) — symlink como script:** `git show <ref>:scripts/link.sh` retorna a string do target
(`real.sh`), não o conteúdo. Digest = `sha256("real.sh")` = imutável quando `real.sh` muda.
Medido: `sha256:d7b138374e...` idêntico em REF1 e REF2 com `real.sh` modificado.
Handoff: `internal/auditsurface/auditsurface.go::gitShow` — detectar objeto do tipo `blob`
que é um symlink e resolver o conteúdo do target.

**F3 (MEDIUM) — `validateRef` aceita SHA de outro repositório:** `git rev-parse --verify <sha>`
sem `^{commit}` aceita SHA de 40 hex sem confirmar existência do objeto → relatório "0 hook tuples"
+ exit 0 para um ref estrangeiro. Medido: SHA de HEAD~5 do trackfw aceito em repo vazio.
Fix: `git rev-parse --verify <ref>^{commit}`.

**Gaps de inventário:** `.vscode/tasks.json` prescrito pelo modelo de ameaça Wave 0 não
implementado; `npm/package.json` hardcoded (não raiz `package.json`). Adicionar à REQ de F1–F3.

**Artefato escrito:** `docs/seguranca/2026-08-27-barreira-do-audit-surface.md`

---

### Auditoria do ML-3A — **REPROVADO parcialmente**; três achados, dois graves

Parecer: `docs/seguranca/2026-08-27-barreira-do-audit-surface.md`.

**A pergunta que eu pedi para ele atacar era "o comando mente em algum caso?" — e a resposta é sim,
em dois.**

**F1 (HIGH) — `normalizeCommand` só resolve uma forma estreita.** Para qualquer comando que não seja
`<path>.sh|.py|.js` **sem argumentos e sem prefixo de interpretador**, o digest é fixado em
`"unresolvable"` **para sempre** — independente do conteúdo do script mudar. Medido em três formas
comuns: `.bash`, `cmd --arg`, `bash cmd`. **A variante A fica indetectável nessas formas.**

**F2 (HIGH) — symlink como script.** `git show <ref>:scripts/link.sh` devolve a **string do alvo**
(`real.sh`), não o conteúdo do arquivo real. O digest reportado é o hash do **nome**. Quando
`real.sh` muda entre dois refs, o digest de `link.sh` **fica idêntico**.

> *"Presente-e-estável engana mais que ausente."*

É exatamente a classe que abriu esta série inteira: o `validate` dando verde enquanto o guard estava
inerte.

**F3 (MEDIUM) — `validateRef` aceita SHA de outro repositório.** Confirmei por medição própria:

```
$ trackfw audit-surface 0000000000000000000000000000000000000000
trackfw audit-surface: 0 hook tuple(s) at 000000…
absent [claude] .claude/settings.json
exit=0
```

Relatório **tranquilizador** para um ref que não existe. Correção de uma linha:
`git rev-parse --verify <ref>^{commit}`.

**O que passou:** os 8 runtimes, 7 arquivos de instrução, slash commands, lifecycle hooks e
`.husky/pre-commit` cobertos · variantes B e C detectadas · AC16 sem falso-positivo além das duas
fixtures · AC6 medido (`settings.json` só com `permissions` → `no_hooks`, não acusado).

**Sobre a costura de autoteste (a superfície nova que mandei ele atacar):** risco baixo, **sem
bypass** — a variável só faz **falhar**, nunca passar falso; valor não reconhecido comporta-se como
"sem seam"; não está definida em CI nem no `Makefile`; binário sabotado limpo por `trap`.

**Dois gaps de inventário, não bloqueantes:** `.vscode/tasks.json` prescrito pela Wave 0 e não
implementado · `npm/package.json` com path fixo — em repositório externo com `package.json` na raiz,
imprime três linhas `lifecycle [absent]` **tranquilizadoras** varrendo o path errado.

---

## Wave 4 — Correção pós-barreira

### ML-1B — Os dois casos em que o relatório mente
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

| # | Onde | Correção |
|---|---|---|
| F1 | `internal/auditsurface/auditsurface.go::normalizeCommand` | resolver `.bash`, comando com argumentos e prefixo de interpretador — ou, quando não resolver, **dizer que não resolveu** em vez de fixar digest constante |
| F2 | `internal/auditsurface/auditsurface.go::gitShow` | detectar **symlink** (modo `120000` no `git ls-tree`) e reportar como tal — **nunca** hash do nome do alvo |
| F3 | `internal/commands/audit_surface.go::validateRef` | `git rev-parse --verify <ref>^{commit}` |
| — | inventário | `.vscode/tasks.json` · `package.json` por descoberta, não path fixo |

**Critérios de aceite:**
- [ ] F1: variante A detectada nas 3 formas medidas (`.bash`, `cmd --arg`, `bash cmd`)
- [ ] F2: symlink reportado como symlink; digest **não** é o hash do nome do alvo
- [ ] F3: ref inexistente → erro, **não** relatório vazio com exit 0
- [ ] Cenários novos no `check-audit-surface.sh` para F1, F2 e F3, com guard de vacuidade
- [ ] `make quality` CI-exata **exit 0** · `validate` 16 warnings

---

### Auditoria do ML-1B — aprovada; os três casos em que o relatório mentia estão fechados

**F3 confirmado por medição minha:**

```
$ trackfw audit-surface 0000000000000000000000000000000000000042
Error: audit-surface: ref "000000…42" does not resolve: fatal: Needed a single revision
(antes: "0 hook tuple(s)" + exit 0)
```

**F1 e F2, com a evidência dele:**

```
F1  .bash · --arg · bash cmd  ->  digest MUDA entre refs   (antes: "unresolvable" fixo)
    genuinamente irresoluvel (-c, pipes) segue marcado como tal
F2  symlink->real.sh|sha256:8bb7138 -> sha256:335a931  + marcador de symlink
```

O F2 é o que importava: o relatório **diz que é symlink** e o digest acompanha o conteúdo real, em vez
de hashear o nome do alvo e parecer estável para sempre.

**Gaps de inventário fechados:** `.vscode/tasks.json` presente · `package.json` **por descoberta**,
não path fixo.

```
check-audit-surface        17/17 cenarios, byte-identicos nos 3 CLIs
SELFTEST_BREAK=A / =B      falham nos cenarios designados
make quality (CI-exata, minha)  exit 0
validate                   16 warnings, 0 violations
```

**Dado de custo, para a decisão de protocolo:** este ML custou **124.436 tokens**, contra **44.255**
do ML-2A. O protocolo de economia (sem `make quality` no agente, sem dump de log, ponteiros
`arquivo::símbolo`) ajudou, mas **o escopo domina**: este mexeu em código de produto nos 3 CLIs mais o
gate; o outro era só gate. A alavanca é o **tamanho do microlote**, não a forma da comunicação.

---

---

### ML-3B — Reverificação pós-ML-1B
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem reprovada levanta o bloqueio — ou o mantém com evidência.

**Veredito:** APROVADO. Os três achados originais (F1 em 3 formas, F2 básico, F3) estão fechados.
Bloqueio levantado. Quatro resíduos novos declarados (R1–R4), nenhum bloqueia o trackfw.

**Achados originais:**
- F1 (.bash, cmd --arg, bash cmd): FECHADO. Gate FN-F1a/b/c 3/3.
- F2 (link→real.sh): FECHADO. Digest segue conteúdo real; marcador `symlink->` presente.
- F3 (validateRef): FECHADO. `^{commit}` confirmado no código + gate FN-F3 3/3.

**Resíduos novos descobertos na reverificação:**
- R1 (MEDIUM): `env VAR=x script.sh` → unresolvable estável. FN mas marcado.
- R2 (MEDIUM): path com espaço no nome → unresolvable estável. FN mas marcado.
- R3 (HIGH): cadeia de symlinks (link→middle→real) → digest estável quando real muda. FN silencioso.
- R4 (MEDIUM): symlink circular → sha256 do nome do target, não de conteúdo real.

**Q3 — package.json discovery sem FP:** root com postinstall encontrado; node_modules e fixtures ausentes. ✅

**Regressão:** 17/17 ✅

**Artefato:** `docs/seguranca/2026-08-27-barreira-do-audit-surface.md` — seção "Reverificação ML-3B".

---

### ML-1C — R3 (HIGH): cadeia de symlinks — resolução recursiva com limite e detecção de ciclo
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

Barreira ML-3B declarou R3 (HIGH): cadeia `link.sh → middle.sh → real.sh` produz digest estável
quando `real.sh` muda, porque o código segue apenas 1 nível e hasheia o blob de `middle.sh`
(que é outra string de symlink target). Mesmo modo de falha do F2 original, um nível mais fundo.

| # | Onde | Correção |
|---|---|---|
| R3 | resolução de symlink em todos os 3 CLIs | loop com visited + depth limit: seguir a cadeia até blob real, ciclo ou profundidade excedida |
| R4 | mesmo loop | detectar ciclo (resolved == visited path) → `\|circular-not-supported` |
| — | `getSymlinkTarget` (Go) | `cmd.Dir = gitRoot` ausente — corrigir no mesmo passe |
| — | Python `posixpath.join` | usar `posixpath.normpath(posixpath.join(...))` para normalizar `..` |

**Arquivos afetados:**
- `internal/auditsurface/auditsurface.go` — novo `resolveScriptDigest()` com loop + `cmd.Dir` fix
- `npm/src/commands/audit-surface.js` — mesmo loop no bloco de digest
- `pypi/trackfw/commands/audit_surface.py` — mesmo loop + `posixpath.normpath`
- `scripts/check-audit-surface.sh` — 4 novos cenários com guard de vacuidade duplo

**Formato de marcador:** `symlink-><first_target>|<outcome>` onde outcome é:
- `sha256:<hex>` — conteúdo real do arquivo final
- `circular-not-supported` — ciclo detectado
- `chain-not-supported` — profundidade excedida (limit=8)
- `not-supported` — alvo absoluto (sem regressão do ML-1B)
- `not-found` — alvo ausente (sem regressão do ML-1B)

**Critérios de aceite:**
- [x] Cadeia 2 níveis: digest muda quando arquivo final muda — REF1: `symlink->middle.sh|sha256:7e9e48f252767a210ce30377e834683000c9b6b16869fc5360d72c99ffa9eaba` / REF2: `symlink->middle.sh|sha256:cf9850002cd0d4ecd9e70cba267a5f592dae532cc14d748f17e57053139ca0e1`
- [x] Ciclo: `symlink->link.sh|circular-not-supported`, sem `sha256:` ✅
- [x] Profundidade excedida: `symlink->b.sh|chain-not-supported`, sem `sha256:` ✅
- [x] `not-supported` e `not-found` sem regressão — `symlink->/etc/passwd|not-supported` · `symlink->nonexistent.sh|not-found` ✅
- [x] 4 cenários novos no gate com guard de vacuidade duplo (FN-R3-chain, FN-R3-cycle, FN-R3-depth, FN-R3-nonreg)
- [x] Byte-idêntico nos 3 CLIs — parity confirmada nos 4 cenários novos
- [x] `go build ./...` EXIT:0 · `go test ./internal/auditsurface/...` EXIT:0 · `bash scripts/check-audit-surface.sh` 21/21 EXIT:0

**R1 e R2 declarados como resíduo** — não corrigidos neste ML (fora de escopo da barreira).

---

### Auditoria do ML-3B e do ML-1C — **APROVADO**, e o R3 fechado antes do PR

**A barreira aprovou o ML-1B, mas achou o mesmo defeito um nível mais fundo — R3 (HIGH):** a
resolução de symlink parava no **primeiro** nível, então numa cadeia `link → middle → real` o código
hasheava a **string** que estava dentro de `middle`:

```
REF1: symlink->middle.sh|sha256:d7b138374e…
REF2: symlink->middle.sh|sha256:d7b138374e…    <- IDENTICO, com real.sh alterado
```

O prefixo `symlink->` alertava, mas o `sha256` **parecia conteúdo e nunca mudava**. Mesmo modo de
falha do F2: *presente-e-estável engana mais que ausente*. Confirmei no código
(`auditsurface.go:151-165`, um nível só) e mandei fechar antes do PR.

**Depois do ML-1C:**

```
cadeia 2 niveis   symlink->middle.sh|sha256:7e9e48f2… -> sha256:cf985000…   MUDA
ciclo             symlink->link.sh|circular-not-supported                    sem sha256 ficticio
profundidade      symlink->b.sh|chain-not-supported                          9 hops, limite 8
nao-regressao     symlink->/etc/passwd|not-supported · symlink->nonexistent.sh|not-found

check-audit-surface  21/21, paridade nos 3 CLIs em cada cenario novo
make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

**O critério que separa o que corrigi do que declarei:** onde o comando não consegue resolver, ele
**diz que não conseguiu** (`circular-not-supported`, `chain-not-supported`, `unresolvable`) em vez de
emitir um `sha256:` que parece conteúdo. **R1** (`env VAR=x script.sh`) e **R2** (path com espaço)
ficam como resíduo justamente por já serem honestos: admitem que não sabem. **R4** (circular) foi
coberto de graça pela detecção de ciclo.

**A barreira mediu por conta própria, sem confiar no meu resumo** — inclusive formas que eu não
listei (`./script.sh`, caminho absoluto, `env VAR=x`), e verificou que `package.json` por descoberta
**não** reporta `node_modules/**` nem fixture.

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
