---
status: Open
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md"
---

# REQ: gate escreve na arvore que audita e a guarda existente nao cobre o culpado

> Date: 2026-09-17 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **#366**. Um gate do `make quality` escreve no `trackfw.yaml` **da raiz do repositório** — não
numa fixture. A config regenerada perde `governance_mode: lenient`, `ci: github-actions`,
`forge: github` e o bloco de versão de modelo por tier (ADR-2026-08-21).

🔴 **Sem o `lenient`, warning vira erro.** O `trackfw validate` local continua com RC=0, mas o CI
reprova `governance-go-install` acusando **26 REQs** e um ADR que ninguém tocou — com a `main` verde
nos mesmos artefatos. Sintoma a três camadas da causa.

**Custo já pago (2026-09-16):** quebrou o CI do PR #365 e obrigou a restaurar o arquivo antes de cada
uma das três tags (rc2, rc3, GA).

### Culpado isolado por bissecção

`scripts/check-tty-detection.sh:43`:

```bash
HOME="$WORK/home" "$GO_BIN" init --ai-tools gemini >/dev/null 2>&1 </dev/null
```

Isola o **`HOME`** num temporário, mas **não isola o `cwd`**. O `trackfw init` roda na raiz do
repositório. O mesmo `init` também cria um `GEMINI.md` na raiz.

### 🔴 O achado que muda o remédio

**A guarda que impediria isso JÁ EXISTE** — `scripts/check-gates-falsify.sh`, Cenário 18
(`no-repo-mutation`), que captura `git status --porcelain` antes e depois de rodar gates a partir do
`ROOT_DIR`. Ela é **anterior** a este defeito: veio do ML-6I, motivado pela nota de vault
`update-parity-gate-writes-real-claude-md-2026-07-29.md` — ou seja, **este repositório já pagou por
esta mesma classe antes, criou a guarda, e a guarda não pegou desta vez**.

O motivo é o allowlist:

```bash
GATES_MUTATION_CHECK=(
  scripts/check-update-parity.sh
  scripts/check-barrier.sh
  scripts/check-slash-parity.sh
  scripts/check-rules-parity.sh
)
```

Quatro gates. Dos seis candidatos que invocam `init`/`discover`, **três ficam de fora** — incluindo o
culpado. **Uma lista fixa deixa gate novo nascer fora da cobertura por omissão**, que é a mesma forma
do defeito original: assumir por omissão em vez de falhar nomeando.

## Acceptance Criteria

- [ ] **AC1** — `check-tty-detection.sh` isola o `cwd` além do `HOME`; rodá-lo não altera a árvore.
- [ ] **AC2** — 🔴 A cobertura do Cenário 18 deixa de ser lista fixa: passa a **enumerar** os gates,
      de modo que um gate novo nasça **dentro** da cobertura. Se houver exclusão, ela é declarada com
      motivo por item, nunca por omissão.
- [ ] **AC3** — Falsificação: um gate propositalmente sujo é **reprovado** pela guarda, e o mesmo
      gate limpo passa. Sem isso não se sabe se a guarda mede.
- [ ] **AC4** — 🔴 Varredura dos demais gates que invocam `init`/`discover`/`update`: quantos sujavam
      a árvore antes desta REQ? O número entra no relatório. Se algum além do
      `check-tty-detection.sh` sujar, é **mesma causa, mesma REQ** — corrige aqui.
- [ ] **AC5** — O `GEMINI.md` da raiz tem destino decidido e escrito: ou é artefato legítimo do
      projeto, ou sai. Hoje ele está rastreado por acidente (entrou no commit `4c8f1b04`).

## Negative Scope

- ❌ **Não** alterar `governance_mode` nem qualquer política de governança para "resolver" o sintoma.
- ❌ **Não** adicionar o `trackfw.yaml` ao `.gitignore` — isso esconderia o dano em vez de impedi-lo.
- ❌ **Não** tratar o #376 aqui: o gerador aplicar template de consumidor ao produtor é **outra
  causa**, com REQ própria.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
