# Wave 0 — Threat Model: Primeira experiência

> Roadmap: ROADMAP-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md
> Hades (hades-tf) · 2026-11-02 · branch feat/primeira-experiencia
> Binário: `/opt/homebrew/bin/trackfw` 11.2.5
> Convenção: `$SCRATCH` = `/private/tmp/claude-501/…/scratchpad/hades-pe/` (scratchpad da sessão,
>             fora do repositório); `…` = saída abreviada quando o restante é repetição de padrão.

---

## 1. Completude da Enumeração

### Varredura A — `§` em não-test `.go` dentro de `internal/` e `cmd/`

```
$ grep -rn '§' internal/ cmd/ --include='*.go' \
    | grep -v '_test.go:' \
    | grep -v ':[0-9]*:[[:space:]]*//' \
    > $SCRATCH/sect.txt && wc -l $SCRATCH/sect.txt
21 $SCRATCH/sect.txt
```

Resultado completo (21 linhas):

```
internal/integrations/render.go:299:    … "YAML 1.2 §5.1 c-printable" …
internal/commands/branch_prune.go:336:  … CLAUDE.md §1 …
internal/commands/agents_models.go:56:  … ADR-2026-08-21 §4 …
internal/commands/commit.go:346:        … Ver CLAUDE.md §1.
internal/guard/gitbranch.go:23–68:      (17 linhas, cada uma com "Ver CLAUDE.md §1.")
```

Total: 1 allowlisted (`YAML 1.2 §5.1`) + 1 ADR ref (`ADR-2026-08-21 §4`) + 1 PT+§ fora de guard
(`commit.go`) + 1 EN+§ no `Long:` (`branch_prune.go`) + 17 PT+§ em guard (`gitbranch.go`).
`cmd/` não tem hits.

### Varredura B — padrão ampliado (scripts, workflows, non-`.go` de `internal/`)

```
$ grep -rln 'bruto bloqueado\|RECUSADO\|CLAUDE\.md §\|ADR-2026-08-21 §\
\|não é permitido\|one active branch at a time' \
  . --exclude-dir=.git --exclude-dir=seguranca
```

10 arquivos relevantes; hit total não contado (grep -l conta arquivos, não linhas):

| Arquivo | Tipo | O que acerta | Actualização? |
|---|---|---|---|
| `internal/guard/gitbranch.go` | produto | 17×§1 + 3×PT sem § | SIM — ML-1A |
| `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh` | fixture bash congelado | 15 `REASON=` com `§1` | SIM — ML-1A re-pina SHA256 |
| `internal/pathguard/testdata/corpus-pre-fix/…/scaffold.go.txt` | corpus estado histórico | reason strings PT | NÃO — pathguard conta write-containment violations, não texto |
| `.github/workflows/quality.yml:868` | comentário YAML `#` | `CLAUDE.md §Protocolo de Release` | NÃO — comentário, não emitido |
| `internal/commands/branch_prune_forge_test.go` | doc-comment de test | `§"Regra Dura de Reconciliação"` | NÃO — comentário |
| `internal/commands/branch_prune_remote_test.go` | doc-comment de test | mesma citação | NÃO — comentário |
| `internal/commands/commit.go:346` | produto | `Ver CLAUDE.md §1` | SIM — ML-1A |
| `internal/commands/branch_prune.go:336` | produto, Long: help | `CLAUDE.md §1` (já em EN) | SIM — ML-1A remove a citação |
| `internal/commands/agents_models.go:56` | produto, Long: help | `ADR-2026-08-21 §4` | SIM — ML-1A |
| `internal/commands/ship.go` | produto | `RECUSADO` | confirmado: hits apenas em comentários `//` — NÃO |

`ship.go` verificado: `grep -n 'RECUSADO\|§' internal/commands/ship.go | grep -v ':[0-9]*:[[:space:]]*//'` → 0 hits.

### Varredura C — `§` em não-`.go` dentro de `internal/`

```
$ grep -rn '§' internal/ --exclude='*.go'
```

Resultado: apenas o fixture bash (`git-branch-guard.sh`) e o corpus (`scaffold.go.txt`) — ambos
classificados em Varredura B.

### Varredura D — português em não-test `internal/guard/*.go`

```
$ grep -n '[áàâãéêíóôõúçÁÉÍÓÚÇÃÕ]\|RECUSADO\|nao \|bloquead' \
    internal/guard/gitbranch.go internal/guard/payload.go \
    internal/guard/credential.go internal/guard/writescope.go \
  | grep -v '_test.go' | grep -v ':[0-9]*:[[:space:]]*//'
```

Por arquivo:

- `gitbranch.go`: 20 linhas (3 constantes sem `§` + 17 constantes com `§`)
- `payload.go:161`: 1 linha — `fmt.Errorf("payload JSON inválido: %w", err)`
- `credential.go`: 0 hits — mensagens já em inglês
- `writescope.go`: 0 hits — mensagens já em inglês

### Varredura E — confirmação do escopo

AC1 nomeia explicitamente `git-branch, credential e write-scope`. Varredura D confirma: `credential.go`
e `writescope.go` já estão em inglês. Fora dos guards, os três sítios com `§` (Varredura A) também
entram no escopo de AC2 (P1).

### Varredura F — português em todo o produto (escopo ampliado)

```
$ grep -rn --include='*.go' \
    '[áàâãéêíóôõúçÁÉÍÓÚÇÃÕ]\|RECUSADO\|nao \|bloquead' \
    internal/guard internal/commands internal/validator internal/generators cmd \
  | grep -v '_test.go:' | grep -v ':[0-9]*:[[:space:]]*//' \
  | cut -d: -f1 | sort | uniq -c | sort -rn
```

27 arquivos no total; 1 deles é guard em escopo de AC1/AC2 (gitbranch.go); payload.go aparece na
lista mas o erro é descartado antes de alcançar o usuário (ver §1 payload.go e R5); 25 estão fora
do escopo (sem `§`):

| Contagem | Arquivo | Em escopo de AC1/AC2? |
|---|---|---|
| 147 | `internal/generators/scaffold.go` | NÃO — wizard de scaffold |
| 46 | `internal/commands/help.go` | NÃO — UI strings |
| 20 | `internal/guard/gitbranch.go` | SIM — ML-1A |
| 16 | `internal/generators/roadmap.go` | NÃO — gerador |
| 10 | `internal/validator/validator_git_branch_guard.go` | NÃO — mensagem de lint |
| 9 | `internal/validator/validator_credential_guard.go` | NÃO — mensagem de lint |
| 8 | `internal/validator/validator.go` | NÃO — mensagem de lint |
| 8 | `internal/generators/update.go` | NÃO — wizard |
| 8 | `internal/commands/configure.go` | NÃO — wizard |
| 7 | `internal/commands/identity_wizard.go` | NÃO — wizard |
| 5 | `internal/validator/validator_traceid.go` | NÃO — mensagem de lint |
| 5 | `internal/generators/probes.go` | NÃO — gerador |
| 5 | `internal/commands/branch.go` | NÃO — UI string |
| 4 | `internal/commands/metrics.go` | NÃO — UI string |
| 3 | `internal/generators/note.go` | NÃO — gerador |
| 3 | `internal/commands/req.go` | NÃO — wizard strings do assistente interativo; sem `§`, fora de AC1/AC2 |
| 3 | `internal/commands/baseline.go` | NÃO — UI string |
| 3 | `internal/commands/agents_models.go` | NÃO para linhas 18,19,191; SIM linha 56 (§4, coberta por Varredura A) |
| 2 | `internal/generators/claudemd.go` | NÃO — `GlobalADRsDirective` (ver R7) |
| 2 | `internal/commands/integrations_flags.go` | NÃO — UI string |
| 1 | `internal/guard/payload.go` | NÃO emitido ao user (gitbranch.go:117–130 descarta; ver R5) |
| 1 | `internal/validator/validator_req_done_criteria.go` | NÃO — mensagem de lint |
| 1 | `internal/generators/nextsteps.go` | NÃO — gerador |
| 1 | `internal/generators/agentfiles.go` | NÃO — gerador |
| 3 | `internal/commands/commit.go` | NÃO para linhas 321,324 (sugestão de commit, fora de AC1); SIM linha 346 (coberta por §1) |
| 1 | `internal/commands/init.go` | NÃO — wizard string |
| 1 | `internal/commands/adr.go` | NÃO — wizard string |

AC1 delimita: "mensagens de bloqueio emitidas pelos guards". Os 25 arquivos fora dos guards estão
fora do escopo desta REQ (R1–R3).

**Contradição sinalizando ao arquiteto:** REQ linha 34 afirma "O resto do CLI é em inglês". A
Varredura F mostra português em 25 arquivos de produto fora dos guards. A premissa é incorreta;
nenhuma outra REQ a corrige. Não bloqueia esta REQ — declarado em R3.

### As 20 constantes `reason*` de `gitbranch.go` — classificação

```
$ grep -c 'Nada antes deste comando foi executado' internal/guard/gitbranch.go
17
```

`reasonTruncated` (linha 17) NÃO contém "Nada antes" — termina com "Reenvie o comando, ou use
`trackfw ship` / `trackfw branch new`." As 17 constantes com `§1` contêm "Nada antes deste
comando foi executado".

| Constante | Linha | Tem `§1`? | Tem "Nada antes"? | Tem alternativa `trackfw`? |
|---|---|---|---|---|
| `reasonTruncated` | 17 | NÃO | NÃO | SIM (`trackfw ship`, `trackfw branch new`) |
| `reasonNUL` | 19 | NÃO | NÃO | NÃO — payload corrompido, sem recuperação |
| `reasonIndecodeable` | 21 | NÃO | NÃO | NÃO — payload ilegível, sem recuperação |
| `reasonCheckoutB` | 23 | SIM | SIM | SIM (`trackfw branch new`) |
| `reasonSwitchC` | 25 | SIM | SIM | SIM (`trackfw branch new`) |
| `reasonBranchCreate` | 27 | SIM | SIM | SIM (`trackfw branch new`) |
| `reasonWorktreeAddB` | 29 | SIM | SIM | SIM (`trackfw branch new`) |
| `reasonCommit` | 31 | SIM | SIM | SIM (`trackfw commit`) |
| `reasonPush` | 33 | SIM | SIM | SIM (`trackfw push`, `trackfw ship`, `trackfw release tag`) |
| `reasonPushDelete` | 38 | SIM | SIM | SIM (`trackfw branch prune`) |
| `reasonStash` | 40 | SIM | SIM | SIM (`trackfw branch new`) |
| `reasonResetHard` | 42 | SIM | SIM | SIM (`trackfw ship`) |
| `reasonAddImplicit` | 49 | SIM | SIM | SIM (`trackfw commit`) |
| `reasonCleanForce` | 51 | SIM | SIM | NÃO — `git clean -n` |
| `reasonRestorePath` | 53 | SIM | SIM | NÃO — `git restore --staged` |
| `reasonCheckoutPath` | 55 | SIM | SIM | NÃO — `git checkout <branch>` |
| `reasonUpdateRef` | 57 | SIM | SIM | NÃO — menciona `trackfw release tag` como alvo forjado, não como alternativa |
| `reasonWorktreeRemoveForce` | 59 | SIM | SIM | NÃO — `git worktree remove` sem force |
| `reasonRmForce` | 61 | SIM | SIM | NÃO — mesma classe de `git clean` |
| `reasonWorktreeAddImplicit` | 68 | SIM | SIM | SIM (`trackfw branch new`) |

12 constantes com alternativa `trackfw`; 2 sem alternativa por design (NUL, Indecodeable); 5 sem
alternativa trackfw mas com alternativa git (CleanForce, RestorePath, CheckoutPath,
WorktreeRemoveForce, RmForce); UpdateRef sem alternativa (menção informativa); reasonTruncated sem
"Nada antes" (é sobre timeout de leitura, não sobre comando composto).

### `internal/guard/payload.go:161` — erro descartado

```go
161: return "", fmt.Errorf("payload JSON inválido: %w", err)
```

`gitbranch.go:117–130` descarta `.Error()` de `ExtractCommand()` e seleciona `reasonNUL` ou
`reasonIndecodeable`. O texto `"payload JSON inválido"` nunca chega ao stdout/stderr. Tradução
opcional — ver R5.

### Testes que afirmam texto atual das constantes (devem ser atualizados em ML-1A)

| Arquivo | Linha | Afirma | Quebra com |
|---|---|---|---|
| `internal/generators/git_branch_guard_dedup_test.go` | 321 | `strings.Contains(stderr, "git push bruto bloqueado")` | tradução de `reasonPush` |
| `internal/generators/guard_parity_helper_test.go` | 322 | `c22AwkNulSentinel` (AWK prefix literal do fixture) | tradução do prefixo AWK no fixture |
| `internal/generators/guard_parity_helper_test.go` | 326,331 | `c22IndecodeableReason`, `c22NULReason` — cópias byte-a-byte | tradução dessas constantes |
| `internal/generators/guard_parity_helper_test.go` | 341 | `assertGuardParity` Go vs fixture bash byte-a-byte | qualquer divergência Go/fixture |
| `internal/generators/guard_parity_helper_test.go` | 418 | pin SHA256 `"71445ebf…"` | qualquer mudança no fixture |
| `internal/commands/commit_test.go` | 50, 64 | `"commit direto em \"main\" não é permitido"` | tradução de `commit.go:346` |

### `internal/generators/req.go` — ausente do ML-1B

Linhas 22 (`LinkedADR string`), 135 (`adr: ""`), 151 (`ADR: %s`). ML-1B deve atualizar
`internal/commands/req.go` E `internal/generators/req.go`.

### Fixture bash: `REASON=` está indentado 4 espaços

```
$ sed -n '755,757p' \
    internal/generators/testdata/guard-sh-reference/git-branch-guard.sh
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. … Ver CLAUDE.md §1."
    ;;
```

Linhas `REASON=` têm 4 espaços de recuo. O filtro de P8 deve ser `^[+-][[:space:]]*REASON=`.

### Reconciliação: 20 Go vs 15 fixture `REASON="trackfw…"`

```
$ F=internal/generators/testdata/guard-sh-reference/git-branch-guard.sh
$ grep -c 'REASON=' $F
18   # total: 15 com prefixo "trackfw" + 3 sentinelas _TRACKFW_*_REASON=
$ grep -c '^[[:space:]]*REASON="trackfw' $F
15   # linhas de reason reais no fixture
$ grep -c 'escopo implicito\|sem flag de branch' $F
0    # confirma: reasonAddImplicit e reasonWorktreeAddImplicit AUSENTES do fixture
```

17 constantes Go com `§1` vs 15 linhas `REASON="trackfw…"` no fixture. Duas constantes ausentes:
`reasonAddImplicit` e `reasonWorktreeAddImplicit` — adicionadas ao Go num ML posterior ao fixture.
Tradução dessas duas sem gate de paridade byte-a-byte; cobertura apenas pela Camada A. R4.

### Fechamento

Superfície completamente enumerada. Quatro lacunas de roadmap encontradas:
1. `internal/commands/commit.go:346` — ausente do ML-1A
2. `internal/commands/branch_prune.go:336` — ausente do ML-1A
3. `internal/commands/agents_models.go:56` — ausente do ML-1A
4. `internal/generators/req.go` — ausente do ML-1B

---

## 2. Modelo de Ameaça

O adversário aqui não é um atacante externo: é o implementador apressado que fecha cada AC
isoladamente sem verificar interações, e o arquiteto otimista que marca ✅ com base em "testes
passando" em vez de "as conclusões da medição estão representadas nos artefatos".

Resposta às três perguntas do roadmap:

- **Q1 (flag `--adr`):** sim — traversal, injeção de newline, basename sem path, ambiguidade,
  symlink para fora da árvore. (T1, T2, T3, T10)
- **Q2 (escrita do vínculo corrompe frontmatter existente?):** não há caminho de edição in-place
  — `req new` recria o arquivo inteiro. O risco real é sobrescrita silenciosa de REQ existente
  (T6) e injeção via `--adr` com `\n` (T1).
- **Q3 (tradução remove orientação do agente?):** sim. (T7)

### Ameaça T1 — Injeção de newline via `--adr <nome>`

**Vetor:** `--adr "foo\n## Blocked by ADRs\nADR: evil.md"` forja linhas extras no body do REQ;
no frontmatter, `adr: "foo\n"` corrompe o bloco YAML.

**Gate:** P5 — rejeitar `\n`, `\r`, NUL, `"`, backtick. Resolução ADR antes de qualquer escrita.

**Severidade:** MEDIUM.

### Ameaça T2 — Traversal via `--adr <path>`

**Vetor:** `--adr "../../other"` contorna `ResolveADRFiles`.

**Gate:** P5 — rejeitar `/`, `\`, `..`.

**Severidade:** LOW.

### Ameaça T3 — Ambiguidade silenciosa

**Vetor:** dois ADRs com mesmo basename em `adr_dirs` distintos; `findADRFile` retorna o primeiro
sem aviso.

**Gate:** P6 — usar `ResolveADRFiles`, erro quando `len > 1`.

**Severidade:** LOW.

### Ameaça T4 — Path absoluto gravado no REQ, quebra em outra máquina

**Vetor:** `config.go:503` faz `ExpandPath(s)` ao carregar — `ResolveADRFiles` recebe path já
expandido (`/Users/…`). ML-1B escrevendo esse path cria vínculo máquina-específico:
`referenceExists()` falha com `ref_targets_exist` em outra máquina.

**Gate:** P4 — path relativo ao project root, ou `~/` para ADRs em HOME.

**Severidade:** MEDIUM.

### Ameaça T5 — `extractRefPath` early-return quando frontmatter `adr:` é unquoted e vazio

**Cenários medidos (binário 11.2.5 em `$SCRATCH/quickstart-proj/`):**

```
# C4: frontmatter  adr:  (sem aspas)  +  body ADR: docs/adr/ADR-foo.md
$ trackfw validate → ✗ req_has_adr FAILS  (early-return em val="")

# C5: body  ADR: ADR-2026-11-02-foo.md  (basename sem prefixo)
$ trackfw validate → ✗ ref_targets_exist
```

C1 (frontmatter preenchido), C2 (body preenchido, frontmatter `""`), C3 (ambos): todos passam.
Frontmatter tem precedência. Edições manuais no body `ADR:` após `req new --adr` são ignoradas
silenciosamente.

**Severidade:** LOW — C4 não ocorre com template que usa `adr: ""` (quoted).

### Ameaça T6 — `req new` sobrescreve REQ existente sem aviso

**Medido (`$SCRATCH/overwritetest/`, binário 11.2.5):**

```
$ trackfw req new "overwrite-test" --no-roadmap
$ echo "MARKER" >> docs/req/REQ-2026-11-02-overwrite-test.md
$ trackfw req new "overwrite-test" --no-roadmap   # sem aviso
$ grep -c "MARKER" docs/req/REQ-2026-11-02-overwrite-test.md
0   # MARKER DESTROYED
```

Mesma causa que `roadmap new sobrescreve` (REQ-2026-09-09 AC8). ROADMAP-2026-09-09-req-nasce-orfa
está em `done/`; Regra Dura exige movê-lo para `wip/` antes de adicionar o ML. ML-1B não deve
criar workaround.

**Escalação ao arquiteto.**

**Severidade:** MEDIUM (pré-existente).

### Ameaça T7 — Tradução remove orientação que o agente usa para se corrigir

**Vetor:** tradução pode perder: (1) o comando alternativo (`trackfw push` etc.) ou (2) a frase
"nada antes deste comando foi executado" (crucial: agente sabe que não precisa fazer rollback).

**Gate:** Camada A verifica ambos os elementos nos conjuntos corretos. Prescrição P2.

**Severidade:** MEDIUM.

### Ameaça T8 — Fixture congelado: mudanças de lógica peguem carona

**Vetor:** ML-1A edita o fixture, re-pina SHA256, e o diff toca linhas de lógica sem gate.

**Gate:** P8 verifica que o diff toca apenas linhas `REASON=`.

**Severidade:** MEDIUM.

### Ameaça T9 — Três sítios com `§` sobrevivem à tradução

**Vetor:** ML-1A traduz `gitbranch.go` e fixture mas não `commit.go:346`, `branch_prune.go:336`,
`agents_models.go:56`. Teste restrito a `internal/guard/` não detecta.

**Gate:** Camada B sobre `./internal/` inteiro. Prescrição P2.

**Severidade:** LOW.

### Ameaça T10 — Symlink para arquivo externo é enumerado como ADR

**Medido (`$SCRATCH/symlinktest/`, binário 11.2.5):**

```
# Symlink para arquivo fora da árvore: enumerado (count = 2)
# Symlink de diretório: NOT enumerado (os.Stat retorna isDir=true)
$ trackfw context | grep 'ADRs'
## ADRs (2)   # apenas real.md e symlink-to-outside.md
```

Para P4: `EvalSymlinks` sobre o path completo resolve o symlink ao target externo; `Rel(root, target)`
= `../…`, nega path. P4 deve aplicar `EvalSymlinks` apenas ao diretório pai.

**Severidade:** LOW.

### Ameaça T11 — HOME isolation no teste AC4

**Medido:**

```
# validate: idêntico com HOME real e HOME falso ($SCRATCH/fakehome-t11a)
$ diff <(trackfw validate 2>&1) \
       <(HOME=$SCRATCH/fakehome-t11a trackfw validate 2>&1) && echo "IDENTICAL"
IDENTICAL

# init: 30 arquivos gravados em $SCRATCH/fakehome-t11b
$ HOME=$SCRATCH/fakehome-t11b trackfw init --ai-tools claude --forge github
$ find $SCRATCH/fakehome-t11b -type f | wc -l
30   # 1 manifest + 12 agent files + 17 skill files
```

Verificação de escrita fora do escopo neste dispatch:

```
$ stat -f "%Sm %N" -t "%Y-%m-%d" \
    ~/.trackfw/integrations-manifest.json \
    ~/.claude/agents/trackfw-*.md \
    ~/.claude/skills/trackfw-*/SKILL.md \
  | sort | uniq -c
17 2026-07-29   # ~/.claude/skills/trackfw-*/SKILL.md — julho
12 2026-11-01   # ~/.claude/agents/trackfw-*.md — ontem (sessão anterior)
 1 2026-11-01   # ~/.trackfw/integrations-manifest.json — ontem
```

Todos os 30 arquivos têm mtime anterior a hoje (2026-11-02). Nenhuma escrita fora do escopo neste
dispatch.

Padrão de isolamento confirmado em `init_test.go:initFixture`:

```go
func initFixture(t *testing.T) (project, home string) {
    project = t.TempDir()
    home = t.TempDir()
    oldHome := os.Getenv("HOME")
    oldWD, _ := os.Getwd()
    _ = os.Setenv("HOME", home)
    _ = os.Chdir(project)
    t.Cleanup(func() {
        _ = os.Chdir(oldWD)
        _ = os.Setenv("HOME", oldHome)
    })
    return project, home
}
```

**Nota de segurança do cleanup:** `initFixture` usa `os.Setenv` (restaura manualmente). Se ML-1B
usar `os.Setenv("USERPROFILE", …)`, o cleanup deve restaurar o valor original (não `Unsetenv`) —
`Unsetenv` em Windows deixa `os.UserHomeDir()` sem fallback para os testes subsequentes. P7
prescreve `t.Setenv` (salva e restaura automaticamente) para todas as três variáveis.

---

## 3. Alvos de Falsificação

### AC1 — Guard em inglês

| Direção | Onde entra | Gate |
|---|---|---|
| FN — português com `§` em sítios fora de guard | `commit.go:346`, `branch_prune.go:336`, `agents_models.go:56` | Camada B sobre `./internal/` inteiro |
| FN — português accent-free (`reasonTruncated`/`reasonNUL`/`reasonIndecodeable`) | invisível ao scanner de `§` | Camada C: RECUSADO + stopwords em literais nomeados |
| FN — comando alternativo perdido (12 constantes) | constante traduzida sem `trackfw X` | Camada A check (1): table de substrings esperados |
| FN — "nada antes" perdido (17 constantes §1) | constante traduzida sem equivalent | Camada A check (2): table de substrings esperados |
| FP — detector acusa comentário `//` | acessa linhas de comentário | `go/ast BasicLit` retorna apenas string literals |

### AC2 — Nenhuma referência privada

| Direção | Onde entra | Gate |
|---|---|---|
| FN — `§` sobrevive em sítios fora de guard | `commit.go:346`, `branch_prune.go:336`, `agents_models.go:56` | Camada B (go/ast, scope `./internal/`, allowlist `YAML 1.2 §`) |
| FN — accent-free não detectado | `reasonTruncated`/`reasonNUL`/`reasonIndecodeable` | Camada C em literais nomeados em `gitbranch.go` |
| FP — `YAML 1.2 §` acusado | `render.go:299` | allowlist explícita em Camada B |

### AC3 — `req new --adr <ADR>`

| Direção | Onde entra | Gate |
|---|---|---|
| FN — injeção de newline | `--adr "foo\nline2"` | P5 + teste |
| FN — path traversal | `--adr "../outside"` | P5: rejeitar `/`, `\`, `..` |
| FN — ambiguidade silenciosa | `findADRFile` em vez de `ResolveADRFiles` | P6: `len > 1` → erro |
| FN — path absoluto | `ExpandPath` antes de escrever | P4: project-relative ou `~/` |
| FN — EvalSymlinks no path completo | resolve symlink ao target externo | P4: EvalSymlinks apenas no dir pai |
| FN — `req.go` generator não atualizado | ML-1B só edita `commands/req.go` | P3 + AC4 round-trip |
| FN — basename sem path (C5) | `ADR: ADR-foo.md` | `ref_targets_exist` falha |
| FN — frontmatter unquoted (C4) | `adr:` sem aspas → early-return | preservar `adr: ""` quoted |
| FN — falha em `--adr` cria REQ ou roadmap parcial | validação depois de algum write | P3 + P5: ADR resolution antes de qualquer write |
| FN — sem `--adr`, output diverge do atual | ML-1B muda geração sem o flag | teste: saída sem `--adr` byte-idêntica ao 11.2.5 (exceto data) |
| FN — erro de `--adr` emite mensagem em PT | nova mensagem de erro em PT | Camada C inclui os literais de erro do `--adr` |
| FP — ADR com espaço no nome rejeitado | P5 bloqueia espaço | P5 aceita espaço |

### AC4 — Quick start de ponta a ponta

**Baseline medida** (`$SCRATCH/qs-final2/`, binário 11.2.5, sem `init` real — apenas `trackfw.yaml`
mínimo + manualmente simulated `--adr` write):

```
$ HOME=$SCRATCH/fakehome-qs2 trackfw validate
⚠  write_scope_not_wired: .claude/settings.json absent
1 warning(s)
rc=0
```

`blocked_by_draft_adr` NÃO dispara: `adr new` cria ADR com status "Proposed" (não "Draft") —
medido em $SCRATCH/qs-final2 (binário 11.2.5): `validate` retornou rc=0 com o ADR de "Proposed"
presente.
O aviso `write_scope_not_wired` aparece porque o baseline foi executado sem `init` real. A
hipótese é que `init` cria `.claude/settings.json` e o aviso desaparece; não foi medido neste
dispatch (T11 mediu `init` em HOME falso, mas não verificou `validate` depois). O AC4 test
deve verificar que `validate` retorna `err == nil` após a sequência completa incluindo `init`.

| Direção | Onde entra | Gate |
|---|---|---|
| FN — e2e passa, produção falha | `exec.Command("trackfw", …)` (PATH binary, sem `--adr`) | cobra `SetArgs` in-process — P7 |
| FN — validate passa com frontmatter incorreto | ML-1B não atualiza `req.go` | validate pós-geração; C2 e C4 |
| FN — contra-braço ausente | e2e sem `--adr` não verifica violação | teste separado: sem `--adr` → err!=nil |
| FN — init polui HOME do desenvolvedor | HOME não isolado para `init` | `t.Setenv("HOME", t.TempDir())` |
| FN — teste não-serial corrompe HOME | `t.Parallel()` com `t.Setenv` | `t.Setenv` panics on `t.Parallel()` — serial por construção |
| FN — macOS `/var` → `/private/var` diverge | EvalSymlinks inconsistente | P4: EvalSymlinks no projectRoot também |
| FN — XDG lê HOME antigo no Linux | `XDG_CONFIG_HOME` não isolado | P7: `t.Setenv("XDG_CONFIG_HOME", …)` |
| FN — README e teste têm sequências divergentes | README atualizado sem reescrever o teste | revisão do arquiteto: diff literal dos argv strings vs README |

---

## 4. Residual Declarado

**R1 — Português em `internal/generators/scaffold.go` (147 hits)**
Wizard de scaffold — sem `§`, fora do escopo de AC1/AC2.

**R2 — Português em `internal/commands/help.go` (46 hits) e outros 14 arquivos fora dos guards**
UI strings, wizard, configuração — sem `§`, fora do escopo.

**R3 — Português em `internal/validator/validator*.go` e `internal/generators/roadmap.go` e outros**
Mensagens de lint e geração. Sem `§`. Fora do escopo. A REQ delimita: "mensagens de bloqueio
emitidas pelos guards". A premissa REQ linha 34 ("O resto do CLI é em inglês") é incorreta —
sinalizando ao arquiteto sem bloquear esta REQ.

**R4 — `reasonAddImplicit` e `reasonWorktreeAddImplicit` ausentes do fixture bash**
17 Go `§1` constants vs 15 fixture `REASON=` (linhas medidas). Tradução dessas duas constantes
tem cobertura apenas na Camada A (substrings), sem gate de paridade byte-a-byte no fixture.

**R5 — `payload.go:161`: `"payload JSON inválido"` — tradução opcional**
O erro nunca alcança o usuário (gitbranch.go:117–130 descarta). Camada C escaneará esse literal
se ML-1A não o traduzir. P2 Camada C prescreve allowlist explícita com comentário citando
gitbranch.go:117–130 — ou tradução da linha com teste correspondente.

**R6 — Frontmatter `adr:` tem precedência sobre body `ADR:`**
Edições manuais no body `ADR:` após `req new --adr` são silenciosamente ignoradas.
Comportamento documentado (first match wins).

**R7 — `internal/generators/claudemd.go:12` gera `GlobalADRsDirective` em português**
```go
const GlobalADRsDirective = "Obrigatório: Inspecione e respeite todos os ADRs globais…"
```
Emitido em `trackfw init` → `CLAUDE.md` do consumidor. Sem `§`, fora de AC1/AC2. Relevante para
ML-2A (README mostra output do quick start). Declarado como residual desta REQ.

**R8 — `req new` sobrescreve REQ existente sem aviso (T6)**
Pré-existente. Escalado ao arquiteto: sítio irmão de REQ-2026-09-09 AC8. ROADMAP-2026-09-09-
req-nasce-orfa em `done/`; deve voltar para `wip/`.

---

## Prescrições

**P1 — ML-1A: lista de arquivos afetados inclui quatro sítios adicionais**

Acrescentar ao ML-1A:
- `internal/commands/commit.go` (linha 346) + `internal/commands/commit_test.go` (linhas 50, 64)
- `internal/commands/branch_prune.go` (linha 336 — texto do `Long:`)
- `internal/commands/agents_models.go` (linha 56 — texto do `Long:`, cita `ADR-2026-08-21 §4`)

AC2 não está satisfeito enquanto qualquer um destes quatro sítios emitir `§` não allowlisted.

**P2 — ML-1A: três camadas de teste para AC1 e AC2**

Camada A — AC1, orientação preservada (escopo: `internal/guard/gitbranch.go`,
`package guard`, acesso às constantes pelo nome — o compilador detecta rename):

```go
// package guard  (zz_orientation_test.go ou similar — *_test.go em package guard)
//
// A tabela usa os identificadores Go diretamente; qualquer rename quebra a compilação.
var wantSubstrings = []struct {
    c    string
    want []string
}{
    {reasonCheckoutB,           []string{"trackfw branch new"}},
    {reasonSwitchC,             []string{"trackfw branch new"}},
    {reasonBranchCreate,        []string{"trackfw branch new"}},
    {reasonWorktreeAddB,        []string{"trackfw branch new"}},
    {reasonCommit,              []string{"trackfw commit"}},
    {reasonPush,                []string{"trackfw push", "trackfw ship", "trackfw release tag"}},
    {reasonPushDelete,          []string{"trackfw branch prune"}},
    {reasonStash,               []string{"trackfw branch new"}},
    {reasonResetHard,           []string{"trackfw ship"}},
    {reasonAddImplicit,         []string{"trackfw commit"}},
    {reasonWorktreeAddImplicit, []string{"trackfw branch new"}},
    {reasonTruncated,           []string{"trackfw ship", "trackfw branch new"}},
    // reasonUpdateRef: SEM check — menciona "trackfw release tag" como alvo forjado, não alternativa
    // reasonCleanForce, reasonRestorePath, reasonCheckoutPath, reasonWorktreeRemoveForce,
    // reasonRmForce: SEM alternativa trackfw, NÃO entram na tabela
    // reasonNUL, reasonIndecodeable: sem alternativa por design, NÃO entram
}

// Para "Nada antes" — 17 constantes com §1 (excluindo reasonTruncated):
// A frase exata em inglês é entregável do ML-1A (ex: "Nothing before this command was executed").
var wantNothingBefore = []string{
    reasonCheckoutB, reasonSwitchC, reasonBranchCreate, reasonWorktreeAddB,
    reasonCommit, reasonPush, reasonPushDelete, reasonStash, reasonResetHard,
    reasonAddImplicit, reasonCleanForce, reasonRestorePath, reasonCheckoutPath,
    reasonUpdateRef, reasonWorktreeRemoveForce, reasonRmForce, reasonWorktreeAddImplicit,
}
// Para cada entrada em wantSubstrings: assert c contains each want substring.
// Para cada em wantNothingBefore: assert c contains o English equivalent da frase.

// Mutation counter-arm (obrigatório):
// 1. cp internal/guard/gitbranch.go $SCRATCH/gb.orig
// 2. Mutar reasonBranchCreate no ARQUIVO REAL — remover "trackfw branch new"
// 3. go test ./internal/guard/ -count=1 → FAILS
// 4. Restaurar: cp $SCRATCH/gb.orig internal/guard/gitbranch.go
// 5. cmp $SCRATCH/gb.orig internal/guard/gitbranch.go  → exit 0
// 6. go test ./internal/guard/ -count=1 → PASSES
```

Camada B — AC2, ausência de `§` privado (escopo: `./internal/` inteiro, não só guard):

```go
// Usar go/ast para enumerar todas BasicLit strings em arquivos não-test de ./internal/.
// Pular testdata/ e corpus-pre-fix/.
// Falha se qualquer string literal contém "§" exceto quando prefixada por "YAML 1.2 §".
//
// Mutation counter-arm:
// 1. cp internal/guard/gitbranch.go $SCRATCH/gb.orig
// 2. Inserir "Ver CLAUDE.md §1" em uma constante — go test → FAILS
// 3. Restaurar; cmp; go test → PASSES
```

Camada C — AC2, stopwords + acentos (escopo dividido por package):

**Parte C-guard** (`package guard`, acesso por nome — `zz_orientation_test.go` ou arquivo
separado no mesmo package):

```go
// Escopo: as 20 constantes reason* de gitbranch.go, acessadas por identificador.
// Para cada constante, falha se contém:
//   RECUSADO  |  nao   |  bloquead  |  implicito  |  [áàâãéêíóôõúçÁÉÍÓÚÇÃÕ]
//
// Para payload.go:161 ("payload JSON inválido") — constante ou literal inline:
//   OPÇÃO A: ML-1A traduz a linha; o teste a lê por nome e inclui no escopo.
//   OPÇÃO B: ML-1A não traduz; allowlist explícita:
//     // Justificativa: gitbranch.go:117-130 descarta esse erro antes de emiti-lo.
//
// Mutation counter-arm:
// 1. Com código atual (pré-tradução): ao menos uma constante reprova (reasonTruncated tem "RECUSADO")
//    → confirmar antes de rodar ML-1A.
// 2. Inserir "RECUSADO" num literal traduzido — go test → FAILS
// 3. Restaurar; go test → PASSES
```

**Parte C-commands** (`package commands`, para os três sítios de `internal/commands/`):

```go
// Escopo: os três sítios com § em internal/commands/
//
// (a) commit.go:346 — os testes existentes commit_test.go:50,64 AFIRMAM a presença do texto PT
//     atual ("commit direto em \"main\" não é permitido"). Eles quebram quando ML-1A traduzir
//     a linha. ML-1A deve atualizar esses dois testes para afirmar o novo texto em inglês.
//     Camada B (go/ast, escopo ./internal/) já cobre a ausência de § nesse literal após a tradução.
//
// (b) branch_prune.go:336 — Long: help text:
//     cmd := newBranchPruneCmd()
//     require.NotContains(t, cmd.Long, "§")
//
// (c) agents_models.go:56 — Long: help text:
//     cmd := newAgentModelsCmd()   // nome medido: agents_models.go:38
//     require.NotContains(t, cmd.Long, "§")
//
// Os hits extras em commit.go (linhas 321, 324) e agents_models.go (linhas 18, 19, 191)
// NÃO entram no escopo — são wizard strings, sem §, fora de AC1/AC2.
//
// Mutation counter-arm (para b e c):
// 1. Inserir "§2" em newBranchPruneCmd().Long — go test → FAILS
// 2. Restaurar; go test → PASSES
```

**P3 — ML-1B: lista de arquivos afetados inclui `internal/generators/req.go`**

O implementador deve:
1. Adicionar `FrontmatterADR string` em `REQContent` (`generators/req.go:22`).
2. No template: `adr: "` + `content.FrontmatterADR` + `"` quando não-vazio; `adr: ""` quando vazio.
   Não usar `%q`. Aplicar `filepath.ToSlash`.
3. Linha body `ADR:` com path relativo completo (não basename — C5 falha).
4. **Ordem obrigatória em `runReqNew`:** validação (P5) → `ResolveADRFiles` → `NewREQWithResult`
   → `chainRoadmapForREQ`. Write order medido: `NewREQWithResult` executa `os.MkdirAll` +
   `os.WriteFile` (req.go:57–135); `chainRoadmapForREQ` é chamada depois. A resolução do ADR deve
   preceder a primeira chamada de `NewREQWithResult`.
5. Testes obrigatórios:
   - Round-trip: `req new --adr` escreve → `validate` lê sem violação.
   - Erro sem arquivo: `--adr` inválido → nenhum arquivo REQ E nenhum arquivo roadmap.
   - Saída sem flag: `req new` sem `--adr` → output byte-idêntico ao 11.2.5 (exceto data).
   - Mensagens de erro do `--adr` em inglês: `--adr "foo/../bar"` → erro contém `invalid` ou
     similar em inglês; Camada C assina esse literal.

**P4 — ML-1B: path portável para ADRs**

```go
// hit é o resultado de ResolveADRFiles.
absHit := hit
if !filepath.IsAbs(absHit) {
    absHit = filepath.Join(absRoot, absHit)
}

// EvalSymlinks apenas no diretório pai (preserva nome lógico de symlink-arquivo):
hitDir, hitBase := filepath.Split(absHit)
canonDir, err := filepath.EvalSymlinks(hitDir)
if err != nil { canonDir = filepath.Clean(hitDir) }
absHitCanon := filepath.Join(canonDir, hitBase)

// Normalizar projectRoot da mesma forma (macOS: /var → /private/var):
canonRoot, err := filepath.EvalSymlinks(absRoot)
if err != nil { canonRoot = filepath.Clean(absRoot) }

sep := string(os.PathSeparator)
rel, err := filepath.Rel(canonRoot, absHitCanon)
if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+sep) {
    return filepath.ToSlash(rel), nil
}
// Fallback: caminho relativo a HOME
home, _ := os.UserHomeDir()
canonHome, _ := filepath.EvalSymlinks(home)
if canonHome == "" { canonHome = home }
relHome, err2 := filepath.Rel(canonHome, absHitCanon)
if err2 == nil && relHome != ".." && !strings.HasPrefix(relHome, ".."+sep) {
    return "~/" + filepath.ToSlash(relHome), nil
}
return "", fmt.Errorf("ADR outside project root and HOME; use a project-local ADR")
```

**P5 — ML-1B: validar o valor do flag `--adr` antes de qualquer resolução**

Rejeitar: `/`, `\`, `..`, NUL (`\x00`), `\n`, `\r`, `"`, backtick. Espaço aceito.
Saída não-zero sem criar nenhum arquivo (REQ nem roadmap).

**P6 — ML-1B: erro explícito quando `ResolveADRFiles` retorna múltiplos ou nenhum resultado**

Usar `ResolveADRFiles`, não `findADRFile`. Predicado de match: `base == arg || base == arg+".md"`
(comparação de string exata — macOS é case-insensitive no FS, mas o produto não realiza case-fold).
Vetores de teste:
- `--adr "auth"` com dois ADRs cujo basename começa com "auth" → não encontrado (sem match exato)
- `--adr "ADR-foo"` + `--adr "ADR-foo.md"` → ambos resolvem para o mesmo arquivo
- Quando `len > 1`: erro listando os ADRs encontrados
- Quando `len == 0`: erro "ADR not found: <arg>"
- Case-variant: `--adr "adr-foo.md"` (lowercase) com ADR de nome `ADR-foo.md` — com a comparação
  `base == arg || base == arg+".md"` (string exata contra o nome enumerado), o resultado é "not
  found" em todo OS, incluindo macOS. FS case-insensitivity afeta `os.Open`/`os.Stat`, não a
  comparação de strings. Um implementador que faça `os.Stat(filepath.Join(dir, arg))` em vez de
  enumerar encontraria o arquivo em macOS e gravaria o path em minúsculas no REQ — `ref_targets_exist`
  falharia em Linux CI. O teste deve confirmar que a comparação é por string, não por `os.Stat`.

**P7 — ML-1B: teste AC4 usa cobra SetArgs in-process; HOME isolado com t.Setenv**

Padrão confirmado em `internal/commands/init_test.go` (cobra SetArgs) e `adr_test.go` (newRootCmd):

```go
// Sequência AC4 — Quick Start (5 passos, em um único TestQuickStartEndToEnd serial):

// Passo 1: init
cmd := newInitCmd()
cmd.SetArgs([]string{"--ai-tools", "claude", "--forge", "github"})
require.NoError(t, cmd.Execute())

// Passo 2: adr new
root := newRootCmd()
root.SetArgs([]string{"adr", "new", "My Decision"})
require.NoError(t, root.Execute())
// Ler o nome do ADR gerado: glob "docs/adr/ADR-*.md"

// Passo 3: req new --adr
root = newRootCmd()
root.SetArgs([]string{"req", "new", "My Requirement", "--adr", adrBasename})
require.NoError(t, root.Execute())

// Passo 4: roadmap move … wip
// req new cria o roadmap em backlog/ — mover para wip antes de validate.
// Obter o nome exato: glob "docs/roadmaps/backlog/ROADMAP-*-my-requirement*.md"
root = newRootCmd()
root.SetArgs([]string{"roadmap", "move", roadmapBasename, "wip"})
require.NoError(t, root.Execute())

// Passo 5: validate
// ATENÇÃO: validate.go escreve violações via fmt.Printf (stdout global), NÃO via cmd.OutOrStdout().
// SetOut(buf) NÃO captura violações — a asserção correta é no retorno de Execute():
//   rc = 0  →  err == nil
//   rc != 0 →  err != nil (cobra retorna o erro propagado por RunE)
root = newRootCmd()
root.SetArgs([]string{"validate"})
err := root.Execute()
require.NoError(t, err)  // positive path: 0 violations

// Contra-braço (função de teste separada, NÃO t.Parallel):
//   Fazer req new sem --adr → validate --json → assert err != nil + rule ID:
//
//   Medido em $SCRATCH/qs-counterarm (adr: "" + "ADR: " vazio):
//     validate → stdout: `✗ req "…" has no linked ADR (...)`, stderr: `Error: 1 violation(s) found`
//     validate --json → stdout (via cmd.OutOrStdout()):
//       {"violations":[{"rule":"req_has_adr","file":"REQ-…","message":"…"}],...}
//
//   O modo --json escreve via cmd.OutOrStdout(), capturável por SetOut:
//
//   buf := &strings.Builder{}
//   rootCA := newRootCmd()
//   rootCA.SetArgs([]string{"validate", "--json"})
//   rootCA.SetOut(buf)
//   errV := rootCA.Execute()
//   assert.Error(t, errV)
//   assert.Contains(t, buf.String(), "req_has_adr")
```

HOME isolation para `init` — usar `t.Setenv` (salva/restaura automaticamente, panic em
`t.Parallel()` — enforça execução serial):

```go
home := t.TempDir()
t.Setenv("HOME", home)
t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))  // Linux CI
if runtime.GOOS == "windows" {
    t.Setenv("USERPROFILE", home)
}
// t.Chdir requer Go 1.24+. Usar os.Chdir + t.Cleanup:
orig, _ := os.Getwd()
project := t.TempDir()
_ = os.Chdir(project)
t.Cleanup(func() { _ = os.Chdir(orig) })
// validate não requer git init — medido em $SCRATCH/qs-final2 (plain dir, sem .git): rc=0.
// validate não usa HOME (T11: diff=0 com HOME real vs HOME falso), mas init sim.
```

`t.Setenv` é incompatível com `t.Parallel()` por construção — serial garantido.

NÃO usar `exec.Command("trackfw", …)` — executa o binário do PATH (11.2.5, sem `--adr`).

**Falsificação "mesma sequência que o README" (AC5 bridge):**
Os argv literals do teste em passo 1–5 acima devem ser byte-idênticos aos comandos do Quick Start
do README novo (ML-2A). Arquiteto verifica: `grep -A1 '```' README.md | grep 'trackfw'` vs os
`SetArgs` do teste.

**P8 — ML-1A: diff do fixture restrito a linhas de reason**

```bash
# Rodar após ML-1A commitar, da raiz do repositório:
git diff origin/main -- \
    internal/generators/testdata/guard-sh-reference/git-branch-guard.sh \
  | grep '^[+-]' | grep -v '^---\|^+++' \
  | grep -v '_TRACKFW_.*_REASON\|^[+-][[:space:]]*REASON=' \
  | head -5
# → deve retornar 0 linhas (braço limpo)

# Mutation counter-arm:
# 1. Inserir temporariamente "# comment" no fixture
# 2. Confirmar que o comando retorna >= 1 linha
# 3. Remover e confirmar que volta a 0 linhas
```

`REASON=` está indentado com 4 espaços; o padrão usa `[[:space:]]*`.
Linha 259 (AWK sentinel) não é `REASON=` e não deve ser traduzida.
