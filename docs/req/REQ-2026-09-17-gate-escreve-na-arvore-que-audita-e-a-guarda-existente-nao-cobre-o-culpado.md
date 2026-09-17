---
status: Done
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md"
---

# REQ: gate escreve na arvore que audita e a guarda existente nao cobre o culpado

> Date: 2026-09-17 | Status: Done
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

      🔴 **Emendado após a Wave 0 — a guarda como estava seria satisfeita vacuamente.**
      `git status --porcelain` reporta **estado, não conteúdo**. Num path que já está sujo, a saída é
      byte-idêntica antes e depois de nova escrita. Reproduzido pelo arquiteto no arquivo real:

      ```
      before: [ M docs/agents-working-context.md]
      after : [ M docs/agents-working-context.md]
      ```

      Localmente `make parity` roda `parity-rest` **antes** de `parity-falsify` no mesmo worktree, e
      é o `parity-rest` que contém o gate culpado. Logo o Cenário 18 tira o *before* de uma árvore
      **já suja pelo próprio dano** e conclui que nada mudou. Em CI não ocorre porque os jobs têm
      checkout independente — mas essa garantia é **acidental, não invariante**.

      Duas exigências independentes, porque uma sozinha não basta:
      1. **A comparação inclui conteúdo**, não só o código de status — digest de `git diff HEAD` (ou
         equivalente) junto do `--porcelain`. Uma guarda que não vê conteúdo não mede mutação.
      2. **A falsificação parte de árvore comprovadamente limpa** — `git status --porcelain` vazio
         verificado antes de começar, ou worktree próprio com checkout limpo. Se a árvore não estiver
         limpa, o cenário **falha nomeando**, em vez de medir de um ponto de partida contaminado.
- [ ] **AC4** — 🔴 Varredura dos demais gates que invocam `init`/`discover`/`update`: quantos sujavam
      a árvore antes desta REQ? O número entra no relatório. Se algum além do
      `check-tty-detection.sh` sujar, é **mesma causa, mesma REQ** — corrige aqui.
- [ ] **AC5** — O `GEMINI.md` da raiz tem destino decidido e escrito: ou é artefato legítimo do
      projeto, ou sai. Hoje ele está rastreado por acidente (entrou no commit `4c8f1b04`).


## Residuais nomeados pela Wave 0 (fora dos ACs, registrados para não sumirem)

- **R4 — execução de código do PR durante o próprio CI.** `make parity-rest` depende de `build`
  (`Makefile:27`), que compila `bin/trackfw` a partir do código do PR. Um PR que altere
  `internal/generators/scaffold.go` executa **a sua própria versão** do `Scaffold()` durante a
  rodada, sem isolamento de `cwd`. É visível no diff — não é encoberto —, mas nenhum AC desta REQ o
  cobre, e a correção de `cwd` num único gate não o fecha.
- **R5 — inversão intencional de política por um token.** Acrescentar `--brownfield` à linha 43 do
  `check-tty-detection.sh` produziria `governance_mode: lenient` a três camadas do efeito. O AC1
  fecha o caminho **acidental**; o intencional segue coberto só por revisão humana.
- **R6 — a lição do precedente.** O ML-6I (2026-07-29) codificou *"os gates hoje conhecidos como
  limpos"* em vez de eliminar a propriedade que permite um gate novo nascer fora da cobertura. O AC2
  conserta o mecanismo para os gates **existentes**; não há garantia contínua para gates **futuros**.
  🔴 Se a correção do AC2 for outra lista — ainda que gerada — o R6 continua aberto e esta REQ terá
  repetido o erro que documenta.

### Medições da Wave 0 que valem registro

- Seis gates invocam `init`/`discover`/`update`; **exatamente um** não isola o `cwd`. A contagem da
  REQ estava correta.
- O `init` escreve na raiz: `trackfw.yaml`, `CLAUDE.md`, `GEMINI.md`, `.claude/commands/*.md` e cinco
  scripts em `scripts/` (0755). **Os cinco scripts gerados são idênticos aos commitados** — ou seja,
  sobrescrita já aconteceu antes e foi commitada sem ninguém notar.
- Com o `trackfw.yaml` sobrescrito, `trackfw validate` sai **RC=0 com 156 violações** — a confirmação
  direta do comportamento de três camadas que motivou esta REQ.

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
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
