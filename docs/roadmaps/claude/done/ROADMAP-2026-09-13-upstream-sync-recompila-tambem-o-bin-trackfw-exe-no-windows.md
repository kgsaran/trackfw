---
status: done
date: 2026-09-13
req: "docs/requisições/claude/REQ-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md"
squad: "claude"
---

# Roadmap: upstream-sync recompila também o bin/trackfw.exe no Windows

> Created: 2026-09-13 | Status: done

## Context

REQ: docs/requisições/claude/REQ-2026-09-13-upstream-sync-recompila-tambem-o-bin-trackfw-exe-no-windows.md

O `upstream-sync.sh` só recompila `bin/trackfw`; no Windows o `bin/trackfw.exe` — o que o PowerShell
executa — fica uma versão atrás a cada sync. Decisão do usuário em 2026-09-13: corrigir no script
(opção b), em vez de recompilar à mão a cada sync.

## Acceptance Criteria

- [x] AC1 — `.exe` e binário sem extensão com a mesma versão depois do sync, medido com controle
- [x] AC2 — extensão vinda de `go env GOEXE`, sem predicado de SO; nada muda onde `GOEXE` é vazio
- [x] AC3 — baseline e pós-merge cobertos
- [x] AC4 — `check-upstream-sync-falsify.sh` OK, com o limite do `--skip-verify` declarado
- [x] AC5 — `validate` 0 e `run-local-gates.sh` 0 falhas

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** nenhum (medição e leitura)
**Actions:**

1. **Enumeração.** Busca por todo sítio rastreado que compila ou chama o binário da árvore
   (`git grep` por `go build ... -o ... bin/trackfw` e por `./bin/trackfw`, fora de `docs/` e `vault/`),
   com a origem de cada arquivo conferida contra `upstream/main` usando `MSYS_NO_PATHCONV=1` — sem isso
   o Git Bash converte `upstream/main:.github/...` e o `git cat-file` dá falso "só nosso":

   | sítio | de quem | compila nesta máquina? |
   |---|---|---|
   | `scripts/upstream-sync.sh:80` (baseline) e `:151` (pós-merge) | nosso | **sim**, só `bin/trackfw` |
   | `scripts/run-local-gates.sh` | nosso | não — só exige `bin/trackfw` (133) e imprime a instrução (139) |
   | `Makefile:12` (`make build`) | upstream | sim, só `bin/trackfw` — fora do escopo |
   | `local-gates.yml` (nosso), `quality.yml`, `trackfw-validate.yml`, `windows-census.yml` (upstream) | — | não: runner descartável |

   Nenhum hook do Claude Code deste projeto chama o binário. **Lista fechada**: o único sítio nosso que
   grava o binário nesta máquina é o `upstream-sync.sh`, em dois pontos.

2. **Quem esvazia isto sem quebrar regra escrita.** Qualquer um que apague a linha nova: o
   `check-upstream-sync-falsify.sh` roda o sync com `--skip-verify` (linha 62) e **nunca passa pelos
   builds**, então nada reprova. É o residual principal, declarado abaixo.

3. **Falsificação nas duas direções.**
   - **Regressão (a linha some):** o `.exe` fica na versão anterior. Alvo do AC1 — worktree atrás do
     upstream, `bin/` vazio: sem a mudança o `.exe` não aparece; com a mudança aparece e iguala a versão.
   - **Regressão oposta (build a mais onde não deve):** medido o `GOEXE` que decide:
     `go env GOEXE` = `'.exe'` aqui · `GOOS=linux go env GOEXE` = `''` · `GOOS=darwin go env GOEXE` = `''`.
     Com `GOEXE` vazio a guarda `[ -n ]` não dispara, e o comportamento em Linux/macOS é idêntico ao atual.

4. **Residual declarado.**
   - Nenhum gate automático protege a linha nova (item 2).
   - `make build` e `go build` manual continuam escrevendo só `bin/trackfw`.
   - O `trackfw` do PATH é `npm link`, fora deste escopo.

**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0: a invariante do sync (retido ⊆ docs/ ∪ vault/) continua de pé.
# Limite: roda com --skip-verify e não exercita os builds — ver ML-0A, item 2.
bash scripts/check-upstream-sync-falsify.sh
```

## Wave 1 — O build do .exe no upstream-sync
> Dependencies: ML-0A

### ML-1A — upstream-sync recompila também o bin/trackfw.exe no Windows
**Status:** ✅ Concluído
**Files affected:** `scripts/upstream-sync.sh`
**Actions:**
1. Nos dois sítios (80 e 151), depois do `go build -o bin/trackfw`, se `$(go env GOEXE)` não for vazio,
   construir também `bin/trackfw$(go env GOEXE)`, com a mesma mensagem de falha.
2. Medir por efeito num worktree atrás do upstream, com controle sem a mudança.
3. Rodar `check-upstream-sync-falsify.sh`, `run-local-gates.sh` e `validate`.
**Acceptance criteria:**
- [x] AC1 medido com controle
- [x] AC2 e AC3 conferidos no diff
- [x] AC4 e AC5 verdes

## Evidência — 2026-09-13

**AC1 — medido por efeito, com controle.** Dois worktrees em `9f4c0f1` (2 commits atrás de
`upstream/main`), `bin/` vazio, o mesmo sync nos dois:

| worktree | script | `bin/trackfw` | `bin/trackfw.exe` | `./bin/trackfw` no PowerShell |
|---|---|---|---|---|
| controle | `upstream-sync.sh` antigo | `8.0.0-rc1` | **AUSENTE** | não resolve |
| mudança | `upstream-sync.sh` desta branch | `8.0.0-rc1` | **`8.0.0-rc1`** | **`8.0.0-rc1`** |

Os dois syncs trouxeram o mesmo conteúdo (`go build` exit 0, `validate` 0 antes · 0 depois); a única
diferença é o `.exe`. Confirmado antes de rodar que o sync opera no cwd, não no caminho do próprio
script — é o que permite rodar o script da branch dentro do worktree.

**AC2 e AC3 — no diff.** A extensão vem de `GOEXE_EXT="$(go env GOEXE)"`; o build extra é
`[ -z "$GOEXE_EXT" ] || go build -o "bin/trackfw$GOEXE_EXT" ... || die`, nos dois sítios — baseline
(linhas 84–85) e pós-merge (157, com `${GOEXE_EXT:-}`). Os dois ficam dentro de
`if [ "$SKIP_VERIFY" = "0" ]`, então a variável sempre existe quando o pós-merge roda. `GOEXE` medido:
`.exe` aqui, vazio em `GOOS=linux` e `GOOS=darwin` — onde nada muda.

**AC4.** `check-upstream-sync-falsify.sh`: `OK`. Limite mantido e declarado: ele usa `--skip-verify`
e não exercita os builds; a prova do AC1 é a tabela acima, não este gate.

**AC5.** `trackfw validate`: 0 violações (5 warnings pré-existentes, em REQs de 2026-08-29 que esta
branch não toca). `scripts/run-local-gates.sh`: 10 executados · 0 falhas.
