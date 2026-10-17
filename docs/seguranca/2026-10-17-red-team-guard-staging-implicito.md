# Red-team Wave 1 — Guard de Staging com Escopo Implícito

> Reviewer: `hades-tf` | Data: 2026-10-17
> Branch: `feat/o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito`
> REQ: `docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md`
> ADR: `docs/adr/ADR-2026-09-05-staging-com-escopo-implicito-e-bloqueado-porque-ninguem-audita-o-que-nao-enumerou.md`
> Binário: `go build -o /tmp/tfw-rt ./cmd/trackfw` (HEAD da branch)

---

## Veredito

**NÃO FECHA.**

Dois bloqueantes — um falso positivo (B-1) e um falso negativo de bypass (B-2) — exigem
correção antes de abrir o PR. Todos os outros achados são não-bloqueantes.

---

## Bloqueantes

### B-1 — Falso positivo: combined short flags com dry-run ou interativo

**Descrição:** `applyRule` compara tokens de flag exatamente (`"-n"`, `"--dry-run"`, `"-p"`,
etc.). Flags combinadas como `-An` (= `-A -n`) ou `-Ap` (= `-A -p`) chegam como token único
e não batem em nenhum case — caem fora dos checks de dryRun/interactive. Para `-An` sem
positional: `hasPositional = false` → bloqueia como bare add. Para `-un .`: `-un` é ignorado
como flag desconhecida, `.` é positional whole-tree root → bloqueia.

**Medido:**

| Comando | Semântica | Decisão | Binário |
|---|---|---|---|
| `git add -An` | dry-run + intent-to-add | deve PASSAR (dry-run) | exit 2 — FP |
| `git add -nA` | dry-run + -A | deve PASSAR (dry-run) | exit 2 — FP |
| `git add -un .` | dry-run + tracked + dot | deve PASSAR (dry-run) | exit 2 — FP |
| `git add -nv .` | dry-run + verbose + dot | deve PASSAR (dry-run) | exit 2 — FP |
| `git add -Ap` | interactive patch + -A | deve PASSAR (interativo) | exit 2 — FP |
| `git add -pA` | interactive patch + -A | deve PASSAR (interativo) | exit 2 — FP |

As decisões (Wave 0 e D-U2) declaram: "simulação (`-n`, `--dry-run`) → passam" e
"interativos (`-p`/`-i`/`--patch`/`--interactive`) → passam". A implementação não honra
essa declaração para combined flags.

Nota: o `cli-parity.md` diz "flags curtas combinadas sem pathspec (ex.: -AN bare) bloqueiam
(fail-closed)." O exemplo `-AN` é correto (dry-run não envolvido). Mas a generalização para
TODO combined flag está errada — `-An` contém dry-run e deve passar. A documentação e o código
precisam ser restritos a "combined flags que não incluem `-n`/`-p` reconhecidos bloqueiam;
combined flags com dry-run ou interativo passam, ou são declaradas resíduo explícito."

**Severidade:** BLOQUEANTE. Por critério explícito do escopo: "Qualquer forma legítima
bloqueada é ressalva bloqueante."

---

### B-2 — Falso negativo: mixed root + explicit positionals — bypass de um token

**Descrição:** `applyRule` bloqueia quando `allPositionalsRoot = true` (todo positional é
whole-tree root). Quando há pelo menos um positional fora de `wholeTreeRoots`, retorna `""`.
Mas qualquer `.` ou `:/` entre os positionals faz o git estagiar a whole-tree
independentemente dos outros positionals — a union prevalece. Um agente pode burlar a regra
acrescentando qualquer nome de arquivo após `.`.

**Medido em temp-repo (3 arquivos modificados + 1 untracked):**

```
git add -n . docs/a.txt   → add 'docs/a.txt', 'sub/b.txt', 'docs/new.txt', 'sub/new.txt'
git add -n .              → add 'docs/a.txt', 'sub/b.txt', 'docs/new.txt', 'sub/new.txt'
```

Ambos produzem exatamente os mesmos 4 arquivos. `.` subsume `docs/a.txt` — a union
é a whole-tree. O guard passa `git add . docs/a.txt` (exit 0) mesmo assim.

| Comando | Staging efetivo | Binário |
|---|---|---|
| `git add . docs/a.txt` | 4 arquivos (whole-tree) | exit 0 — FN bypass |
| `git add -A docs/ .` | 4 arquivos (whole-tree) | exit 0 — FN bypass |

**Causa raiz:** o Wave 0 incluiu o vetor `"git add -- . file.go" → "R-4: mixed: root + explicit
→ explicit wins"` na tabela de PASSES. Esse julgamento estava errado — a medição de `.` em
positional misto produz sempre whole-tree. O teste `_Passes` bloqueou esse comportamento errado.

**Correção recomendada ao implementador:** mudar a condição de `allPositionalsRoot` para
`anyPositionalRoot` — se QUALQUER positional for whole-tree root, bloqueia, porque a union
é sempre whole-tree.

**Verificação de ausência de novo FP com "any root → block":**
- `git add docs/ src/a.go` — nem `docs/` nem `src/a.go` são root → passa. OK.
- `git add .github/x.yml docs/` — nenhum é root → passa. OK.
- `git add -- -A file.go` — `-A` após `--` é positional mas não é root → passa. OK.

**Severidade:** BLOQUEANTE. Bypass de um token; qualquer agente que saiba da regra pode
burlar com `git add . <qualquer-arquivo>`.

---

## Item 1 — Falsos negativos: tabela completa

Medição: `printf '{"tool_name":"Bash","tool_input":{"command":"<cmd>"}}' | /tmp/tfw-rt guard git-branch; echo "exit:$?"`.

| Comando | Deveria bloquear? | Binário | Resultado |
|---|---|---|---|
| `git add --all .` | Sim (`--all` é flag; `.` é root positional) | exit 2 | OK |
| `git add -A -- :/` | Sim (`:/` após `--` é positional root) | exit 2 | OK |
| `git add "."` | Sim (tokenizer strip quotes → `.`) | exit 2 | OK |
| `git add './'` | Sim (`./` é root) | exit 2 | OK |
| `git add :(top)` | Sim (D-U4) | exit 2 | OK |
| `git add :/:` | `:/:` não está no conjunto decidido; forma não-standard | exit 0 | Resíduo não declarado (RT-1) |
| `git add :!x` | Sim — whole-tree minus x; semanticamente implícito; tokenizer já tem o token | exit 0 | FN — resíduo não declarado (RT-2) |
| `git add :(exclude)x` | Sim — idem `:!x`; medido: 3 de 4 arquivos staged | exit 0 | FN — resíduo não declarado (RT-2) |
| `git add ./.` | Sim — `git add -n . ./.` produz mesma saída que `git add -n .`; token `./.` não está em wholeTreeRoots | exit 0 | FN — resíduo não declarado (RT-3) |
| `git add -A .//` | `.//` = `./` normalizado; não está em wholeTreeRoots | exit 0 | FN — resíduo não declarado (RT-3) |
| `git --git-dir=x add -A` | Sim (`--git-dir` stripped) | exit 2 | OK |
| `git -c core.x=y add .` | Sim (`-c` stripped) | exit 2 | OK |
| `git add --ignore-removal` | Sim (bare, sem positional) | exit 2 | OK |
| `env X=1 git add -A` | Sim (`env`/KEY=val stripped; `git add -A` é o comando efetivo) | exit 2 | OK |
| `sudo git add -A` | `sudo` não é stripped; first token ≠ `git` | exit 0 | Resíduo não declarado (RT-4) |
| `git add -A ; ls` | Sim (split em `;`) | exit 2 | OK |
| `(git add -A)` | Não — resíduo de subshell (não declarado em Wave 0 explicitamente, mas mesmo gap de `$(...)`) | exit 0 | Resíduo (RT-5) |
| `$(git add -A)` | Não — declared residual | exit 0 | OK — resíduo declarado |
| newline embutido | Sim (split em newline; primeiro segmento bloqueia) | exit 2 | OK |
| `git add . docs/a.txt` | Sim — `.` subsume tudo (B-2) | exit 0 | **BLOQUEANTE B-2** |

---

## Item 2 — Falsos positivos: tabela completa (AC3 — critério principal)

| Comando | Deveria passar? | Binário | Resultado |
|---|---|---|---|
| `git add .github/x.yml` | Sim (caminho explícito com ponto, não root) | exit 0 | OK |
| `git add .gitignore` | Sim (idem) | exit 0 | OK |
| `git add ./src/a.go` | Sim (`./src/a.go` não é root) | exit 0 | OK |
| `git add ../outro/a.go` | Sim (`../outro/a.go` não é root) | exit 0 | OK |
| `git add -- -A` (filename literal) | Sim (`-A` após `--` é positional, não root) | exit 0 | OK |
| `git add -A docs/ .` | Não — `.` presente stages whole-tree (B-2) | exit 0 | **BLOQUEANTE B-2** |
| `trackfw commit -m 'git add .'` | Sim (primeiro token é `trackfw`) | exit 0 | OK |
| `git add -An` | Sim (dry-run combinado) | exit 2 | **BLOQUEANTE B-1** |
| `git add -Ap` | Sim (interativo combinado) | exit 2 | **BLOQUEANTE B-1** |

---

## Item 3 — Regex Amazon Q vs guard: divergências

Padrão: `^git (add|stage)( +(-A|--all|-u|--update|--|\.|\./|\.\.|\.\./|:/|:\(top\)))* *$`

**Concordâncias principais:** `git add -A`, `git add .`, bare `git add`, `git add ..`, `git add
:/`, `git add :(top)`, `git add -- .`, `git stage -A`, `git add -A .`, `git add -A docs/`,
`git add file.go`, `git add .github/x`, `git add -p`, todos os dry-run forms — todos concordam.

**Divergências medidas:**

| Vetor | Guard | Regex | Avaliação |
|---|---|---|---|
| `git add -AN` (combined, sem dry-run) | BLOQUEIA (exit 2) | PASSA | Aceitável — hook é camada primária; RT-6 (declarar em cli-parity.md) |
| `git add -An` (combined com dry-run) | BLOQUEIA (FP) | PASSA | Parcialmente mascarada: o hook bloqueia incorretamente, mas o deniedCommands deixa passar — conflito interno entre camadas |
| `git add . docs/a.txt` | PASSA (FN bypass) | PASSA | Ambas falham — B-2 |

A divergência de `-An` expõe uma tensão: a segunda camada (regex Amazon Q) é mais correta que
a primeira (guard) para combined flags com dry-run. Isso não resolve B-1, mas é evidência
adicional de que o guard está errado nesse ponto.

---

## Item 4 — Template scaffold.go

A mudança de `git add -A` para `git add <arquivos-do-ml>` e `trackfw commit -m` é coerente.
`trackfw commit` existe (confirmado por `trackfw commit --help`). Nenhum emitter restante ativo
(scripts CI usam `git add -A` diretamente em bash, fora do guard de tool). Correto.

---

## Item 5 — R-6: reasonResetHard e TestAllReasonsProduceValidJSON

**Fix confirmado:** `reasonResetHard` trocou `\"...\"` (aspas duplas não escapadas → JSON
inválido) por `'...'` (single quotes → JSON válido). Verificado com `python3 json.load`.

**Teste reprova com regressão:** Sabotagem `sed 's/ship -m '"'"'...'"'"'/ship -m \"...\"/'`
→ `TestAllReasonsProduceValidJSON/reset-hard` FAIL com "invalid character '.' after object
key:value pair". Correto.

**Limitação declarada (RT-7):** o map de 19 reasons é hardcoded. Quando um reason novo for
adicionado, o desenvolvedor deve manualmente inserir uma entrada no map — o teste não auto-detecta.
`denyOut` ainda usa `%s` sem escape; a proteção é ao nível da string da constante. Essa limitação
estrutural é não-bloqueante para Wave 1, mas deve ser declarada no comentário do teste.

---

## Item 6 — Cenário 201: vacuidade

O cenário faz o pattern single-delta correto:

1. Baseline: compila limpo, verifica `git add -A` → exit 2.
2. Sabotagem: corrompe apenas `case "add", "stage":` → `case "__never-matches-s201__":`.
3. Detecção: `git add -A`, `git add .`, `git stage -A` → exit 0 no binário sabotado.
4. Auto-discriminação: `git push` → exit 2 no sabotado (a regra do push não foi tocada).

Confirmado: o braço de vacuidade de fato reprova sem a regra. Correto.

---

## Item 7 — Regra Dura de Reconciliação: testes novos

| Teste | Frase do que afirma | Reprova sem a regra? |
|---|---|---|
| `TestMatchSubcommand_AddImplicit_Blocks` | Todas as formas de escopo implícito do ADR/Wave 0 retornam `"add-implicit"` em `MatchSubcommand`. | Sim — FAIL quando `case "add","stage"` é removido (14 subtests falham) |
| `TestMatchSubcommand_AddImplicit_Passes` | Caminhos explícitos, flags interativas e dry-run retornam `""` — sem falso positivo nas formas legítimas do ADR. | Não reprova com remoção do case (retorna `""` para tudo) — testa apenas que o guard não over-blocks, correto por construção |
| `TestMatchSubcommand_AddImplicit_AC5` | O guard não bloqueia "git add -A" quando é texto em mensagem (primeiro token não é `git`). | Não reprova com remoção do case — todos os 3 vetores têm `trackfw`/`echo`/`gh` como primeiro token; o add rule nunca é alcançado. Testa o parser base, não a regra nova. Válido, mas a frase deve ser: "o guard não interpreta prosa como add quando o primeiro token não é git." |
| `TestRunGitBranch_AddImplicit_ExitAndReason` | O caminho completo `RunGitBranch → case "add-implicit" → reasonAddImplicit` emite exit 2 + JSON correto. | Sim — FAIL quando o case no RunGitBranch é removido |
| `TestAllReasonsProduceValidJSON` | Todos os 19 reasons atuais produzem JSON parseável. | Sim — FAIL quando `reasonResetHard` tem `\"` não-escapado (verificado por sabotagem) |

**Nota sobre AC5:** os três vetores (`trackfw commit`, `echo`, `gh pr create`) passam sem a
regra nova porque o primeiro token nunca é `git`. Testado com JSON corretamente escapado:
`printf '{"tool_name":"Bash","tool_input":{"command":"trackfw commit -m \"fix; git add .\""}}' | /tmp/tfw-rt guard git-branch` → exit 0. Correto. O teste é válido, mas a frase do que afirma deve refletir que ele testa a camada de tokenização, não a regra `add` em si.

---

## Item adicional: git diff --cached

Confirmado limpo antes de qualquer medição: `git diff --cached --stat` → sem saída.

---

## Ressalvas não-bloqueantes (RT-n)

| # | Descrição | Ação recomendada |
|---|---|---|
| RT-1 | `:/:` não declarado como resíduo | Adicionar a `docs/cli-parity.md` |
| RT-2 | `:!x` / `:(exclude)x` não declarados (staging whole-tree-minus-x, medido) | Declarar como resíduo em `docs/cli-parity.md` com medição |
| RT-3 | `./.` / `-A .//` não declarados (path normalization não feita) | Declarar como resíduo |
| RT-4 | `sudo git add -A` passa — `sudo` não é stripped (mesma classe de `(git add -A)`) | Declarar como resíduo junto com `(git add -A)` |
| RT-5 | `(git add -A)` não declarado explicitamente como resíduo no Wave 0 (só `$(...)`) | Declarar junto com `sudo` e `(...)` |
| RT-6 | Divergência regex AQ vs guard para `-AN` bare (guard bloqueia; regex passa) | Declarar em `docs/cli-parity.md` que combined flags são cobertas pelo hook, não pela regex |
| RT-7 | `TestAllReasonsProduceValidJSON` hardcoded — não auto-detecta reasons futuros | Adicionar comentário no teste: "ao criar novo reason, adicione aqui" |

---

## Resumo executivo

**NÃO FECHA.** Dois bloqueantes impedem o PR:

- **B-1 (FP):** combined flags com dry-run ou interativo bloqueiam (`-An`, `-Ap`). O guard
  compara tokens exatos; combined flags não são decompostos. Fix: decompor combined short flags
  na análise de dryRun/interactive, ou declarar como resíduo com restrição explícita ao `-AN`/`-uN`
  (sem dry-run envolvido). A segunda opção é mais simples mas exige corrigir o cli-parity.md para
  não generalizar para todo combined flag.

- **B-2 (FN):** `git add . docs/a.txt` bypassa a regra mas estageia whole-tree. Medição: 4
  arquivos staged, idêntico a `git add .`. Fix: mudar `allPositionalsRoot` para `anyPositionalRoot`
  — se QUALQUER positional for root, bloqueia. Sem falso positivo com essa mudança: `git add docs/
  src/a.go` passa, `git add .github/x.yml docs/` passa.
