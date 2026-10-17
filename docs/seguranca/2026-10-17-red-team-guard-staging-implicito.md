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

---

## Rodada 2 — Hades-tf

> Reviewer: `hades-tf` | Data: 2026-10-17 (segunda sessão)
> Binário branch: `/tmp/tfw-rt2` (HEAD feat branch, commit `59fc9ea7`)
> Binário r1: `/tmp/tfw-r1` (estado após commit `084ba721` — antes do fix, round-1 code)
> Binário main: `/tmp/tfw-main` (origin/main == local main `3e9d4c6d`, confirmado por `git rev-parse main origin/main`)
> Harness: `python3 -c 'import json,sys;print(json.dumps({"tool_name":"Bash","tool_input":{"command":sys.argv[1]}}))'`
> Motivação: testar o código acrescentado nos commits `ccccedc8`/`59fc9ea7`.

---

### Veredito Rodada 2

**BLOQUEIA.**

Dois bloqueantes novos, introduzidos pelo código desta rodada:

- **B-3** — A decomposição de combined flags (B-1 fix) set `dryRun=true` para o char `n` em
  `-An`, mas não reconhece `--no-dry-run` como negação. Git usa last-flag-wins: `git add -An
  --no-dry-run` estageia tudo. Medido: 2 arquivos staged em repo de teste. Apenas o vetor
  `-An --no-dry-run` é novo desta rodada; os outros (`-n --no-dry-run .`, `-p --no-patch .`,
  `--dry-run --no-dry-run .`) eram já miss na round 1.
- **B-4** — o strip de `(` deixa `)` no último token. O comentário do código afirma que isso
  "does not change blocking decisions." É falso para allowlists de subcomando e flags de
  bloqueio: `(git stash list)` → "list)" não está na allowlist → bloqueia comando legítimo.

---

### B-3 — Bypass via negação de flag: --no-dry-run / --no-patch

**Descrição:** `applyRule` reconhece `--dry-run` e `-n` (via decomposição B-1) como sinais
de dry-run, e passa o comando. Mas git parse-options aceita `--no-dry-run` como negação de
`--dry-run`/`-n` com last-flag-wins. O guard não reconhece a negação.

**Atribuição:** só `git add -An --no-dry-run` é novo nesta rodada (no r1, `-An` era
unrecognized combined flag → blocked bare add; agora passa por dryRun=true + não-negado).
As outras formas (`-n --no-dry-run .`, `-p --no-patch .`, `--dry-run --no-dry-run .`) eram
miss na round 1 também — confirmado medindo r1: r1=exit:0 para todos esses vetores.

**Medido em /tmp/tgit (2 arquivos modificados):**

```
git add -An               → stderr: "add 'test.txt'\nadd 'other.txt'" / diff --cached: vazio (dry-run OK)
git add -An --no-dry-run  → staged: other.txt, test.txt (whole-tree staging)
git add -An --no-dry      → staged: other.txt, test.txt (forma abreviada, mesmo resultado)
git add -n --no-dry-run test.txt → staged: test.txt (--no-dry-run cancela -n)
```

**Tabela B-3:**

| Comando | Semântica git | Deveria | branch | r1 | Novo desta rodada? |
|---|---|---|---|---|---|
| `git add -An --no-dry-run` | stages whole-tree | BLOQUEIA | exit:0 FN | exit:2 (blocked bare) | **SIM** |
| `git add -An --no-dry` | stages whole-tree (abrev.) | BLOQUEIA | exit:0 FN | exit:2 | **SIM** |
| `git add -n --no-dry-run .` | stages `.` | BLOQUEIA | exit:0 FN | exit:0 FN | Não (pre-existente) |
| `git add -p --no-patch .` | stages `.` | BLOQUEIA | exit:0 FN | exit:0 FN | Não (pre-existente) |
| `git add --dry-run --no-dry-run .` | stages `.` | BLOQUEIA | exit:0 FN | exit:0 FN | Não (pre-existente) |
| `git add --no-dry-run .` | stages `.` | BLOQUEIA | exit:2 OK | exit:2 OK | Já correto |
| `git add -An` | dry-run, nada staged | PASSA | exit:0 OK | exit:2 FP (era BUG) | B-1 fix correto |

**Causa raiz:** `applyRule` não modela last-flag-wins para os pares dry-run/interactive.
`--no-dry-run`, `--no-patch`, `--no-interactive`, `--no-dry` (abreviação aceita por git)
devem cancelar os booleans correspondentes.

**Correção recomendada:** reconhecer os negativos na lógica de flags — usando last-wins em
vez de OR acumulativo:
- `--no-dry-run` / `--no-dry` → `dryRun = false`
- `--no-patch` / `--no-interactive` → `interactive = false`
- E para char decomposition: tratar `N` (maiúsculo) como `--no-dry-run` é arriscado (não é
  flag git documentada para `add`); mais seguro: só os long-form negatives.
- Atenção: `--no-dry-run -n .` seria FP se dryRun for sobrescrito em vez de last-wins.
  Implementar como var + sobrescrita sequencial, não como OR.

**Severidade:** BLOQUEANTE. `git add -An --no-dry-run` é o vetor novo — bypass de um token
adicionado após o B-1 fix. Os outros três rows pré-existentes agravam mas não são a causa da
nova superfície.

---

### B-4 — False positive: RT-5 paren strip corrompe token final (novo em RT-5 fix)

**Descrição:** `MatchSubcommand` faz strip do `(` inicial do segmento, deixando o `)` final
preso no último token. O código comenta: "does not change blocking decisions because it
attaches to a flag or filename." Esse invariante é falso para dois casos:

**(a) Allowlists de subcomando:** `(git stash list)` → strip → tokens `["git","stash","list)"]`.
A regra de stash bloqueia "stash" e tem allowlist de subcomandos seguros (`list`, `show`,
etc.). `"list)"` ≠ `"list"` → não está na allowlist → bloqueia. Mas `git stash list` sem
parênteses passa. Resultado: FP — comando legítimo bloqueado pelo strip do `(`.

**(b) Flags de bloqueio (RT-NEW-1, não-bloqueante):** `(git reset --hard)` → tokens `["git",
"reset", "--hard)"]`. `"--hard)"` ≠ `"--hard"` → reset rule não detecta `--hard` → passa.
Mas `git reset --hard` sem parênteses bloqueia. Resultado: FN — a `)` impede o bloqueio.

**Tabela B-4 (FPs — branch bloqueia; sem parênteses passa):**

| Comando | Sem parênteses (branch) | Com parênteses (branch) | Tipo |
|---|---|---|---|
| `(git stash list)` | exit:0 PASSA | exit:2 BLOQUEIA | **FP → B-4 BLOQUEANTE** |
| `(git stash show)` | exit:0 PASSA | exit:2 BLOQUEIA | **FP → B-4 BLOQUEANTE** |

**Tabela RT-NEW-1/RT-NEW-2 (FNs — branch passa; sem parênteses bloqueia):**

| Comando | Sem parênteses (branch) | Com parênteses (branch) | Tipo |
|---|---|---|---|
| `(git reset --hard)` | exit:2 BLOQUEIA | exit:0 PASSA | FN (RT-NEW-1, não-bloqueante) |
| `(git add .)` | exit:2 BLOQUEIA | exit:0 PASSA | FN (RT-NEW-2, ver abaixo) |
| `(git add :/)` | exit:2 BLOQUEIA | exit:0 PASSA | FN (RT-NEW-2) |

**Adicionalmente:** `( git add -A )` (espaço antes de git) passa no branch E no main — a
espaço antes de `git` faz o `(` ficar no próprio segment de `(`, que então é trimmed, e `)`
final vira token separado. `cli-parity.md` declara `((…))` como resíduo mas afirma que
`(...)` simples está coberto — contradito pela evidência (RT-NEW-6, mesma raiz de B-4).

**Causa raiz:** a análise que gerou o código RT-5 cobriu flags (ok — `"-A)"` sem `n/p/i`
→ fail-closed) e filenames (ok — `"file.go)"` não é root token). Não cobriu (a) allowlists
onde a keyword precisa de match exato e (b) flags de bloqueio onde o `")"` impede o match.

**Correção recomendada:** strip simétrico: se `(` foi removido do início do segmento, remover
também o `)` do final do segmento (antes do shellTokenize). Isso é robusto para o paren de
subshell `(cmd)`. Casos `(cd x && git add .)` são tratados pelo split em `&&`, e a `)` fica
no último segmento: `git add .)` → strip `)` → `git add .` → blocks. Atenção: o strip de `)` 
final do segmento, não do token — aplicar em `seg` antes de `shellTokenize(seg)`.

**Severidade:** BLOQUEANTE por B-4 (FP). RT-NEW-1/2 são FNs não-bloqueantes (classe RT-5
que o código declara como residual, mas o comentário sobre o invariante está factualmente
errado).

---

### Mudanças fora da regra `add` introduzidas pelo código desta rodada

O strip de `sudo` e `(` afeta TODOS os guards, não só o add. Comportamentos novos vs main:

| Comando | main | r1 | branch | Avaliação |
|---|---|---|---|---|
| `sudo git add -A` | exit:0 | exit:0 | exit:2 | Novo bloco correto |
| `sudo git commit -m fix` | exit:2 (commit guard) | exit:2 | exit:2 | Consistente |
| `sudo git push --force` | exit:2 (push guard) | exit:2 | exit:2 | Consistente |
| `sudo git reset --hard` | exit:2 (reset guard) | exit:2 | exit:2 | Consistente |
| `sudo git stash` | exit:2 (stash guard) | exit:2 | exit:2 | Consistente |
| `(git push origin main)` | exit:0 (push=main sem guard) | exit:2 | exit:2 | Novo bloco correto (push guard em main não cobre paren) |
| `(git stash list)` | exit:0 | exit:0 | exit:2 | **FP — B-4** |
| `(git stash show)` | exit:0 | exit:0 | exit:2 | **FP — B-4** |

Nota: `cli-parity.md` declara `sudo` e `(...)` apenas sob a regra de `git add`; o impacto
nos outros guards (commit, push, stash) não está declarado nesses guards.

---

### Tabela Completa Rodada 2

Nota sobre coluna `main`: binário de origin/main `3e9d4c6d` (commit docs-only, sem guard de
add; tem guards de commit/push/stash/reset — seus exit:2 são dessas regras, não da ausência
de guard).

| Comando | Deveria | branch | r1 | Tipo |
|---|---|---|---|---|
| `git add -Nf` | BLOQUEIA (bare) | exit:2 OK | exit:2 OK | OK |
| `git add -fA` | BLOQUEIA (bare) | exit:2 OK | exit:2 OK | OK |
| `git add -e` | BLOQUEIA (bare, -e não é interativo declarado) | exit:2 OK | exit:2 OK | OK |
| `git add -An -- .` | PASSA (dryRun=true fires antes de anyPositionalRoot) | exit:0 OK | exit:2 FP (era bug r1) | OK |
| `git add --chmod=+x .` | BLOQUEIA (root positional) | exit:2 OK | exit:2 OK | OK |
| `git add --chmod=+x file.go` | PASSA (explicit path) | exit:0 OK | exit:0 OK | OK |
| `git add -An` | PASSA (B-1 fix — dry-run correto) | exit:0 OK | exit:2 FP (era bug r1) | OK |
| `git add -An --no-dry-run` | BLOQUEIA (--no-dry-run cancela -n) | exit:0 FN | exit:2 OK | **B-3 BLOQUEANTE** |
| `git add -An --no-dry` | BLOQUEIA (abreviação de --no-dry-run) | exit:0 FN | exit:2 OK | **B-3 BLOQUEANTE** |
| `git add -n --no-dry-run .` | BLOQUEIA | exit:0 FN | exit:0 FN (pre-existente) | B-3 (pre-existente) |
| `git add -p --no-patch .` | BLOQUEIA | exit:0 FN | exit:0 FN (pre-existente) | B-3 (pre-existente) |
| `git add --dry-run --no-dry-run .` | BLOQUEIA | exit:0 FN | exit:0 FN (pre-existente) | B-3 (pre-existente) |
| `sudo -u root -n git add -A` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-4 fix) |
| `sudo --user=x git add -A` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-4 fix) |
| `sudo -- git add -A` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-4 fix) |
| `sudo --user x git add -A` | BLOQUEIA | exit:0 FN | exit:0 FN | RT-NEW-3 (pre-existente) |
| `sudo -n git add file.go` | PASSA (sudo's -n, file.go explicit) | exit:0 OK | exit:0 OK | OK |
| `(git add -A)` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-5 fix) |
| `(git add .)` | BLOQUEIA | exit:0 FN | exit:0 FN | RT-NEW-2 (invariant errado) |
| `(git add :/)` | BLOQUEIA | exit:0 FN | exit:0 FN | RT-NEW-2 |
| `(git reset --hard)` | BLOQUEIA | exit:0 FN | exit:0 FN | RT-NEW-1 |
| `(git stash list)` | PASSA | exit:2 FP | exit:0 OK | **B-4 BLOQUEANTE** |
| `(git stash show)` | PASSA | exit:2 FP | exit:0 OK | **B-4 BLOQUEANTE** |
| `(git add src/file.go)` | PASSA | exit:0 OK | exit:0 OK | OK |
| `(cd x && git add -A)` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-5 fix) |
| `git add .\.` (json-encoded) | BLOQUEIA (.\ → ./ via ReplaceAll → root) | exit:2 OK | exit:2 OK | OK |
| `git add .\\` (json-encoded) | BLOQUEIA | exit:2 OK | exit:2 OK | OK |
| `git add ..\..` (json-encoded) | BLOQUEIA (isAllDotDot) | exit:2 OK | exit:2 OK | OK |
| `git add :(top,glob)*` | BLOQUEIA (glob whole-tree magic) | exit:0 FN | exit:0 FN | RT-NEW-4 |
| `git add :(glob)**` | BLOQUEIA | exit:0 FN | exit:0 FN | RT-NEW-4 |
| `git add *` | INDETERMINADO (shell expande antes; guard vê `*`) | exit:0 | exit:0 | RT-NEW-4-b (ver nota) |
| `echo "(git add -A)"` | PASSA (first token = echo) | exit:0 OK | exit:0 OK | OK |
| `git add -foo` | BLOQUEIA (bare, flag desconhecida) | exit:2 OK | exit:2 OK | OK |
| `sudo -u -n git add -A` | BLOQUEIA | exit:2 OK | exit:0 | OK (sudo -u consome -n como valor) |
| `git add :!x :!y` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-2 fix) |
| `git add :!x src/` | PASSA | exit:0 OK | exit:0 OK | OK |
| `git add ./.` | BLOQUEIA | exit:2 OK | exit:0 | OK (RT-3 fix) |
| `git add .//file.go` | PASSA | exit:0 OK | exit:0 OK | OK |
| `git add docs/ src/a.go` | PASSA | exit:0 OK | exit:0 OK | OK |
| Regressão: `git commit -m fix` | BLOQUEIA | exit:2 OK | exit:2 OK | OK |
| Regressão: `git push --force` | BLOQUEIA | exit:2 OK | exit:2 OK | OK |
| Regressão: `git stash` | BLOQUEIA | exit:2 OK | exit:2 OK | OK |
| Regressão: `git reset --hard` | BLOQUEIA | exit:2 OK | exit:2 OK | OK |
| RT-7 sabotagem reasonSabotageNew | Teste FAIL | FAIL confirmado | — | OK |

**Nota RT-NEW-4-b (`*`):** o hook recebe o texto do comando antes da expansão shell. `git add *`
e `git add '*'` chegam ao guard como o token `*`. `isAddRootToken("*")` → false (path.Clean("*")
= `"*"`, não `"."`). Decidir se `*` deve ser bloqueado é decisão arquitetural (fora do alcance
desta rodada); anotar como resíduo em `docs/cli-parity.md`. `:(glob)**` e `:(top,glob)*` são
pathspec magic words git-nativas e são in-scope — confirmado: `git add -n ":(glob)**"` estageia
toda a árvore em /tmp/tgit.

---

### Ressalvas não-bloqueantes Rodada 2

| # | Descrição | Ação recomendada |
|---|---|---|
| RT-NEW-1 | `(git reset --hard)` → FN: `"--hard)"` ≠ `"--hard"`, `)` corrompe flag de bloqueio. Mesma raiz de B-4. | Corrigir junto com B-4: strip `)` do final de `seg` antes de shellTokenize |
| RT-NEW-2 | `(git add .)`, `(git add :/)` → FN: `".)"`/`":/)"`  não são root tokens. O comentário do código afirma "não muda blocking decisions" — factualmente errado para root tokens e flags de bloqueio. | Corrigir junto com B-4; atualizar o comentário para ser preciso |
| RT-NEW-3 | `sudo --user x git add -A` → FN: long-form sudo flags com valor separado por espaço não são consumidas. O value `x` torna-se o "comando" na lógica de strip. `sudo --user=x` (equals-form) funciona. | Declarar em cli-parity.md como resíduo; ou fix: `case "--user", "--group": consume next` |
| RT-NEW-4 | `git add :(glob)**`, `git add :(top,glob)*` → FN: git magic pathspecs de glob whole-tree passam. Medido: `:(glob)**` é aceito pelo git e estageia toda a árvore. | Declarar como resíduo em cli-parity.md (mesmo RT-4 de exclusion) |
| RT-NEW-5 | `git add -e`/`--edit` não são reconhecidos como interativos. Bare `git add -e` bloqueia (correto — sem positional). `git add -e .` bloqueia (correto — root). Não é FP. | Declarar que -e/--edit não são considerados interativos pelo guard |
| RT-NEW-6 | `( git add -A )` (espaço) passa no branch e no r1. cli-parity.md diz `(...)` simples estão cobertos mas esse form não é coberto. Mesma raiz de B-4 (`)` como token separado). | Corrigir junto com B-4; atualizar declaração em cli-parity.md |
| RT-NEW-7 | `TestAllReasonsProduceValidJSON/auto-discovery` usa `strings.Contains(testText, name)` — passa se o nome aparecer apenas em comentário, sem estar no map de JSON. Verificado: sabotagem `reasonSabotageNew` não adicionada ao map → FAIL confirmado. Vulnerabilidade residual: se um dev adicionar o nome em comentário sem o map entry, o teste passa mas a cobertura JSON está ausente. | Adicionar comentário no teste alertando sobre isso; ou mudar a verificação para checar só dentro da função `reasons := map[string]string{...}` |

---

### Falsify Cenário 201

Tentativa de execução: `bash scripts/check-gates-falsify.sh 201` esgotou o timeout (>120s) —
o script inicializa todos os cenários predecessores antes do 201. Os testes unitários que
cobrem o mesmo invariante (`TestMatchSubcommand_AddImplicit_Blocks`,
`TestRunGitBranch_AddImplicit_ExitAndReason`, `TestAllReasonsProduceValidJSON`) passam —
confirmado por `go test ./internal/guard/ -v`.

---

### Resumo Rodada 2

**BLOQUEIA.** Dois novos bloqueantes — ambos introduzidos pelo código desta rodada:

- **B-3 (FN):** `git add -An --no-dry-run` e `git add -An --no-dry` passam o guard mas
  estagiaram a árvore inteira em /tmp/tgit (medido). O B-1 fix set `dryRun=true` via `-n` em
  `-An`, mas `--no-dry-run`/`--no-dry` não são reconhecidos como negação. Apenas os dois
  vetores com combined flag são novos desta rodada; `-n --no-dry-run .` etc. eram miss na r1.

- **B-4 (FP):** `(git stash list)` e `(git stash show)` bloqueiam (exit:2) embora ambos
  passem sem parênteses (exit:0). A `)` presa em "list)" impede o match na allowlist do stash.
  Comando legítimo bloqueado = critério de bloqueante de Round 1.

**Seis ressalvas não-bloqueantes** (RT-NEW-1 a 7 excluindo dup): todos ou pre-existentes
ou consequências da mesma raiz de B-4 já apontada.

**Código correto verificado esta rodada:**
- B-1 fix: `-An`, `-Ap`, `-nA` passam; `-AN` bloqueia. OK.
- B-2 fix: `git add . docs/a.txt` bloqueia. `git add docs/ src/a.go` passa. OK.
- RT-2 fix: `:!x :!y` bloqueia; `:!x src/` passa. OK.
- RT-3 fix: `./.` bloqueia; `.//file.go` passa. OK.
- RT-4 fix (sudo): `sudo git add -A`, `sudo -u root`, `sudo --` bloqueiam. Long-form `--user x` gap = RT-NEW-3.
- RT-5 fix (paren): `(git add -A)`, `(cd x && git add -A)` bloqueiam. FP para stash list/show = B-4.
- RT-7 auto-discovery: `reasonSabotageNew` sem map entry → FAIL. OK.
- Regressão add guard: sem regressão.
- Regressão outros guards (commit/push/stash/reset): sem regressão.

**Nota de papel:** hades-tf não commita. O arquivo está em
`docs/seguranca/2026-10-17-red-team-guard-staging-implicito.md`.

---

## Rodada 3 — Hades-tf

> Reviewer: `hades-tf` | Data: 2026-10-17 (terceira sessão — rodada final)
> Binário branch: `/tmp/tfw-r3` (HEAD da branch, commit `81c91606`)
> Binário main: `/tmp/tfw-main` (origin/main `3e9d4c6d`, mesmo da Rodada 2)
> Testes: `go test ./internal/guard/ ./internal/generators/` — verde.
> Critério ADR D5/AC7: guard é tripwire contra dedo escorregando, não defesa
> contra adversário. BLOQUEANTE = (a) FP em comando legítimo plausível, ou
> (b) bypass natural. RESÍDUO = forma exótica ou só possível com intenção de
> contornar. Regressão em guard diferente de add = sempre BLOQUEANTE.

---

### Veredito Rodada 3

**BLOQUEIA.**

Dois bloqueantes novos, introduzidos pelo código desta rodada:

- **B-5 (FP):** `git add :(top)src/a.go` e `git add ':(top)src/a.go'` bloqueiam
  quando deveriam passar. `isAddRootToken` trata qualquer `:(top)X` como raiz
  independentemente do padrão `X`.
- **B-6 (FP / Regressão no stash guard):** `(git stash list) > /tmp/x`,
  `(git stash show) 2>&1`, `(git stash list) # comment`, `(git stash list) )`,
  `(git stash list) 2>/dev/null` — bloqueiam no branch (exit:2), passavam no
  main (exit:0). Regressão introduzida pelo B-4 fix: o per-segment strip remove
  `(` mas não remove `)` quando a `)` fechadora não é o último caractere do
  segmento (há redirect/comentário após). `"list)"` não casa com a allowlist do
  stash → stash guard dispara.

---

### B-5 — Falso positivo: `:(top)X` com caminho explícito bloqueia

**Descrição:** `isAddRootToken` retorna `true` para qualquer pathspec mágico
cujos attrs contêm `"top"`, independentemente do padrão que o segue. O `:(top)`
sozinho é de fato raiz (equivalente a `:/`). Mas `:(top)src/a.go` significa
"src/a.go relativo à raiz do repo, mesmo que CWD seja subdiretório" — caminho
explícito. O guard o bloqueia como escopo implícito.

Discriminador: `git add ':/src/a.go'` (sinônimo git, diferente grafia) →
branch exit:0, main exit:0. O `:/src/a.go` não entra no ramo `:(` de
`isAddRootToken`. A assimetria confirma que o bug é a cobertura excessiva do
`"top"` attr, não uma propriedade do pathspec mágico em geral.

**Medido (branch=exit:2 ≡ BLOQUEIA; exit:0 ≡ PASSA):**

| Comando | Deveria | branch | main | Tipo |
|---|---|---|---|---|
| `git add ':(top)src/a.go'` | PASSA | exit:2 | exit:0 | **B-5 FP** |
| `git add :(top)src/a.go` | PASSA | exit:2 | exit:0 | **B-5 FP** |
| `git add ':(top,icase)src/a.go'` | PASSA | exit:2 | exit:0 | **B-5 FP** |
| `(git add :(top)src/a.go)` | PASSA | exit:2 | exit:0 | **B-5 FP** |
| `git add :(top)` | BLOQUEIA | exit:2 | exit:0 | OK (main sem regra add) |
| `git add ':(top).'` | BLOQUEIA | exit:2 | exit:0 | OK |
| `git add ':(top,glob)*'` | BLOQUEIA | exit:2 | exit:0 | OK |
| `git add ':/src/a.go'` | PASSA | exit:0 | exit:0 | OK (discriminador) |

**Causa raiz:** em `isAddRootToken` (linha ~700):

```go
if strings.TrimSpace(a) == "top" {
    return true   // ← ignora o padrão após :(top)
}
```

**Correção recomendada:** só retornar `true` quando o padrão for raiz ou vazio
(`isAddRootToken("")` = true via `path.Clean("") = "."`):

```go
if strings.TrimSpace(a) == "top" {
    if isAddRootToken(pattern) {
        return true
    }
}
```

Verificação que `:(top,glob)*` não regride: `isAddRootToken("*")` = false →
"top" check NÃO retorna true; o ramo "glob" ainda captura porque
`isGlobWildcardOnly("*")` = true.

Consequência declarada do fix: `:(top,exclude)x` passa a retornar `false` por
`isAddRootToken` (pattern=`"x"`, não raiz) e também `false` por
`isExclusionPathspec` (não começa com `:!`, `:^` ou `:(exclude)`). O token
seria classificado como positional não-raiz → `allImplicit=false` → PASSA. Mas
`:(top,exclude)x` estágia todo o repo exceto `x` — é escopo implícito. Esse
vetor deve ser adicionado à lista de resíduos da mesma família de `:!x`/RT-2.

**Severidade:** BLOQUEANTE — FP em comando legítimo que o arquiteto declarou
explicitamente como "deve passar".

---

### B-6 — Regressão no stash guard: `(cmd) > redirect` e formas similares

**Descrição:** o fix de B-4 (Rodada 2) adicionou per-segment strip de `(` +
`)` simétricos. O código só remove a `)` final se ela for o ÚLTIMO caractere
do segmento. Quando há conteúdo após a `)` fechadora (redirect `>`, `2>&1`,
`# comment`, extra `)`), a `)` fica presa no token anterior: `list)` em vez
de `list`. A allowlist do stash case não casa → bloqueia.

`quoteAwareSplit` não divide em `>` (não é separador), então
`(git stash list) > /tmp/x` chega inteiro como segmento ao per-segment strip.

Código problemático (`MatchSubcommand`, per-segment strip, linhas ~269–276):

```go
if len(seg) > 0 && seg[0] == '(' {
    seg = strings.TrimLeft(seg[1:], " \t")
    if len(seg) > 0 && seg[len(seg)-1] == ')' {    // ← só strip se último char
        seg = strings.TrimRight(seg[:len(seg)-1], " \t")
    }
}
```

**Medido:**

| Comando | Deveria | branch | main | Tipo |
|---|---|---|---|---|
| `(git stash list) > /tmp/x` | PASSA | exit:2 | exit:0 | **B-6 REGRESSÃO** |
| `(git stash show) 2>&1` | PASSA | exit:2 | exit:0 | **B-6 REGRESSÃO** |
| `(git stash list) # comment` | PASSA | exit:2 | exit:0 | **B-6 REGRESSÃO** |
| `(git stash list) )` | PASSA | exit:2 | exit:0 | **B-6 REGRESSÃO** |
| `(git stash list) 2>/dev/null` | PASSA | exit:2 | exit:0 | **B-6 REGRESSÃO** |
| `(git stash list)` | PASSA | exit:0 | exit:0 | OK (B-4 fix intacto) |
| `(git stash show)` | PASSA | exit:0 | exit:0 | OK (B-4 fix intacto) |

Nota: `(git add -A) > /dev/null` passa no branch (exit:0) e no main (exit:0).
Não é regressão, mas é FN no add rule (o `>` vira positional não-raiz,
`allImplicit=false` → passa). Resíduo da mesma classe de `$(git add -A)` —
I/O redirects não são modeled pelo guard; declarado junto com D5.

**Correção recomendada:** aplicar `stripOuterParens` ao segmento em vez do
check `seg[len-1]==)`, ou usar a lógica de profundidade para encontrar a `)` 
balanceada dentro do segmento e truncar ali:

```go
if len(seg) > 0 && seg[0] == '(' {
    inner := seg[1:]
    // Find balanced ) inside inner (same depth logic as stripOuterParens)
    depth := 1
    for i, c := range inner {
        if c == '(' { depth++ }
        if c == ')' {
            depth--
            if depth == 0 {
                seg = strings.TrimSpace(inner[:i])
                break
            }
        }
    }
}
```

**Severidade:** BLOQUEANTE — regressão em guard diferente de add (stash guard),
critério sempre-bloqueante por definição.

---

### Tabela completa Rodada 3

| Comando | Deveria | branch | main | Categoria |
|---|---|---|---|---|
| `git add ':(top)src/a.go'` | PASSA | exit:2 | exit:0 | **B-5 BLOQUEANTE** |
| `git add :(top)src/a.go` | PASSA | exit:2 | exit:0 | **B-5 BLOQUEANTE** |
| `git add ':(top,icase)src/a.go'` | PASSA | exit:2 | exit:0 | **B-5 BLOQUEANTE** |
| `(git add :(top)src/a.go)` | PASSA | exit:2 | exit:0 | **B-5 BLOQUEANTE** |
| `(git stash list) > /tmp/x` | PASSA | exit:2 | exit:0 | **B-6 BLOQUEANTE** |
| `(git stash show) 2>&1` | PASSA | exit:2 | exit:0 | **B-6 BLOQUEANTE** |
| `(git stash list) # comment` | PASSA | exit:2 | exit:0 | **B-6 BLOQUEANTE** |
| `(git stash list) )` | PASSA | exit:2 | exit:0 | **B-6 BLOQUEANTE** |
| `(git stash list) 2>/dev/null` | PASSA | exit:2 | exit:0 | **B-6 BLOQUEANTE** |
| `git add :(top)` | BLOQUEIA | exit:2 | exit:0 | OK |
| `git add ':(top).'` | BLOQUEIA | exit:2 | exit:0 | OK |
| `git add ':(top,glob)*'` | BLOQUEIA | exit:2 | exit:0 | OK |
| `git add ':/src/a.go'` | PASSA | exit:0 | exit:0 | OK (discriminador B-5) |
| `(git stash list)` | PASSA | exit:0 | exit:0 | OK (B-4 fix intacto) |
| `(git stash show)` | PASSA | exit:0 | exit:0 | OK (B-4 fix intacto) |
| `git add ':(icase)*'` | BLOQUEIA (whole-tree) | exit:0 | exit:0 | RT-R3-1 (resíduo) |
| `git add ':(top,glob)src/*.go'` | PASSA | exit:2 | exit:0 | RT-R3-2 (mesma raiz B-5) |
| `git add --dry .` | PASSA (abrev.) | exit:2 | exit:0 | RT-R3-3 (resíduo) |
| `git add --dry-run=false .` | BLOQUEIA (git rejeita) | exit:2 | exit:0 | OK |
| `git add --pat .` | PASSA (abrev. --patch) | exit:2 | exit:0 | RT-R3-4 (resíduo) |
| `git add --patch .` | PASSA | exit:0 | exit:0 | OK |
| `git add -A -- -n` | PASSA (-A+path → só -n) | exit:0 | exit:0 | OK (correto — -A com pathspec é scoped) |
| `git add ':(attr:executable)'` | PASSA (seletivo) | exit:0 | exit:0 | OK |
| `(git add -A 'f(x)')` | PASSA (explícito) | exit:0 | exit:0 | OK |
| `(git add -A) > /dev/null` | BLOQUEIA | exit:0 | exit:0 | RT-R3-5 (FN, resíduo I/O redirect) |
| `((git add -A))` | BLOQUEIA | exit:2 | exit:0 | OK |
| `(git add f1 && git add -A)` | BLOQUEIA | exit:2 | exit:0 | OK |
| `(git add -A)` | BLOQUEIA | exit:2 | exit:0 | OK (B-4 fix) |
| `git log --format='(%h %s)'` | PASSA | exit:0 | exit:0 | OK |
| `trackfw commit -m 'fix (scope): x'` | PASSA | exit:0 | exit:0 | OK |
| `gh pr create --title 'x (y)'` | PASSA | exit:0 | exit:0 | OK |
| `git add -An --no-dry-run` | BLOQUEIA | exit:2 | exit:0 | OK (B-3 fix) |
| `git add -An` | PASSA (dry-run) | exit:0 | exit:0 | OK (B-1 fix) |
| `git add -n --no-d .` | PASSA (--no-d não atinge min 8) | exit:0 | exit:0 | OK (guard correto) |
| `git add 'unfinished` | PASSA | exit:0 | exit:0 | OK (unclosed quote — explicit token) |
| Regressão: `git commit -m fix` | BLOQUEIA | exit:2 | exit:2 | OK |
| Regressão: `git push --force` | BLOQUEIA | exit:2 | exit:2 | OK |
| Regressão: `git stash` | BLOQUEIA | exit:2 | exit:2 | OK |
| Regressão: `git reset --hard` | BLOQUEIA | exit:2 | exit:2 | OK |

---

### Ressalvas não-bloqueantes (Resíduo Rodada 3)

| # | Descrição | Ação / texto para `cli-parity.md` |
|---|---|---|
| RT-R3-1 | `:(icase)*` → FN whole-tree. `:(icase)` não reconhecido. Exótico (pathspec magic). | Adicionar ao parágrafo de resíduos junto com `git add '*'`: `:(icase)*` (glob case-insensitive inteiro — guard não reconhece o atributo `icase`; usar `git add .` que é bloqueado). |
| RT-R3-2 | `:(top,glob)src/*.go` → FP (mesma raiz de B-5). Mais exótico. | Fechado pelo fix de B-5. |
| RT-R3-3 | `git add --dry .` → FP (abrev. de `--dry-run`). Git aceita `--dry`; guard aceita só `--dry-run` exato. Raro. | Adicionar: "O guard aceita `-n` e `--dry-run` exatos; abreviações como `--dry` não são reconhecidas (fail-closed). Usar `--dry-run`." |
| RT-R3-4 | `git add --pat .` → FP (abrev. de `--patch`). Guard aceita só `--patch` exato. Raro. | Adicionar junto com RT-R3-3: "Mesma classe para `--patch`/`--interactive`: só a forma exata é reconhecida." |
| RT-R3-5 | `(git add -A) > /dev/null` → FN. `>` vira positional após strip do `(`, `allImplicit=false`. | Adicionar ao bloco D5: "`(git add -A) > redir` — o `>` vira positional, escapa a regra; mesma classe de `$(git add -A)` (I/O redirect não modelado pelo guard)." |
| RT-R3-6 | `:(top,exclude)x` → consequência do fix de B-5. Após o fix, `isAddRootToken(":(top,exclude)x")` = false e `isExclusionPathspec` também = false (não começa com `:!`/`:^`/`:(exclude)`) → PASSA. Mas estágia todo o repo menos `x`. | Adicionar à lista RT-2 de exclusões: "`:(top,exclude)x` — `:(top)` combinado com `exclude` não é capturado pelo guard (resíduo do fix B-5)." |

---

### Análise `stripOuterParens` — aspas e formas especiais

`stripOuterParens` não respeita aspas ao contar `(` e `)`. Para os vetores
testados, isso é benigno:

- **`(` dentro de aspas → depth inflada:** a função não encontra `depth==0` no
  último `)` esperado → retorna `s` inalterado → fail-safe (o per-segment
  processa normalmente).
- **`)` dentro de aspas → depth decrementada:** pode causar retorno prematuro
  com `s` inalterado. Nenhum vetor testado causou strip incorreto.

Casos testados e confirmados:
- `(git add -A 'f(x)')` → strip via coincidência de depth (o `(` na string
  de argumento infla depth para 2; os dois `)` finais retornam depth a 0 em
  last pos) → `git add -A 'f(x)'` → path explícito → PASSA. Correto.
- `(git commit -m "fix (scope)")` → idem → `git commit ...` → BLOQUEIA commit
  guard. Correto.
- `(git stash list) > /tmp/x` → NÃO stripa (primeiro `)` não está em last
  pos) → per-segment processa → **B-6** (ver acima).

Nenhum FP/FN novo atribuível a `stripOuterParens` além de B-6
(per-segment, não `stripOuterParens` diretamente) e B-5 (em `isAddRootToken`).

---

### Testes

`go test ./internal/guard/ ./internal/generators/` — verde em todos os
subtestes (confirmado HEAD commit `81c91606`).

Falsify cenário 201 — novos vetores adicionados por `git diff HEAD~1 --
scripts/check-gates-falsify.sh`:
- `baseline-blocks-An-no-dry-run`: `git add -An --no-dry-run` → exit:2 em base
  binary. Medido: `/tmp/tfw-r3` → exit:2. ✓
- `detection-catches-bypass-An-no-dry-run`: `git add -An --no-dry-run` → exit:0
  em binário sabotado (case "add","stage" removido). Não medido diretamente
  (requer build separado); padrão idêntico ao vetor 200 já validado em rounds
  anteriores. A estrutura é correta; `FALSIFY_SUCCESS_FLOOR` bumped de 246→248
  confirma que os 2 novos braços foram contados.

Vetores B-5 e B-6 não têm cobertura de teste atual — ausentes do suite.

---

### Resumo Rodada 3

**BLOQUEIA.** Dois bloqueantes novos:

- **B-5 (FP add guard):** `isAddRootToken` trata `:(top)X` como raiz
  independentemente de `X`. Fix: mudar para `isAddRootToken(pattern)` no ramo
  "top" (empty pattern cai em `path.Clean("")="."` → true; path explícito →
  false). Consequência declarada: `:(top,exclude)x` passa a resíduo (RT-R3-6).
  Vetores do mesmo ML: `:(top)src/a.go`, `:(top,icase)src/a.go`,
  `:(top,glob)src/*.go` e formas parenthesized.

- **B-6 (FP stash guard / regressão):** per-segment strip remove `(` mas não
  a `)` correspondente quando há conteúdo após ela (redirect/comment/extra `)`)
  no mesmo segmento (porque `>` não é split char em `quoteAwareSplit`).
  Fix: usar lógica de profundidade para localizar a `)` balanceada dentro do
  segmento em vez de checar `seg[len-1]==)`.

**Texto de resíduo para `cli-parity.md`** (paste-ready, estilo do arquivo):

No parágrafo de resíduos declarados (linha ~6279), acrescentar após
`git add '*'`:

```
`:(icase)*` (atributo icase não reconhecido pelo guard — usar `git add .`
que é bloqueado), `(git add -A) > redir` (I/O redirect vira positional,
escapa a regra — mesma classe de `$(git add -A)`).
```

Adicionar ao parágrafo de Passa, após a nota de `--no-interactive`:

```
O guard aceita `-n` e `--dry-run` exatos para dry-run, e `--patch`/`--interactive`
exatos para interativo; abreviações git válidas como `--dry` e `--pat` não são
reconhecidas (fail-closed): bloqueiam quando haveria positional root.
```

Adicionar à lista RT-2 de exclusões:

```
`:(top,exclude)x` — `:(top)` combinado com `exclude` não é capturado pelo
guard após o fix de B-5 (resíduo declarado).
```
