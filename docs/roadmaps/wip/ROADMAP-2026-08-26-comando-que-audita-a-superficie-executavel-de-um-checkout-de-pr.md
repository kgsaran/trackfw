---
status: wip
date: 2026-08-26
req: "docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: comando que audita a superficie executavel de um checkout de PR

> Created: 2026-08-26 | Status: wip

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
**Status:** 🔄 Em andamento · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem enumerou verifica se a implementação cobre o que ele enumerou. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
