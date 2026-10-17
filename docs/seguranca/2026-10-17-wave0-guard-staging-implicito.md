# Wave 0 — Threat Model: Guard de Staging com Escopo Implícito

> Reviewer: `hades-tf` | Data: 2026-10-17
> REQ: `docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md`
> ADR: `docs/adr/ADR-2026-09-05-staging-com-escopo-implicito-e-bloqueado-porque-ninguem-audita-o-que-nao-enumerou.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md`

---

## Veredito

**APROVA COM RESSALVAS.**

O desenho do ADR é sólido e a implementação é direta. A seção 1 fecha a lista de superfícies e
encontra três sites além do que o roadmap nomeia. A medição de semântica git (Seção 3) reverte a
recomendação de D-U1: a forma `-A <dir>` tem o mesmo escopo efetivo que `<dir>` sozinho, então
bloquear `-A docs/` enquanto passa `docs/` seria falso positivo puro — violação de AC3. A regra
certa é baseada em escopo, não em flag. As ressalvas são detalhadas abaixo.

---

## Decisões que precisam do usuário

| # | Questão | Minha recomendação | Justificativa baseada em medição |
|---|---|---|---|
| **D-U1** | Bare `git add` (sem operando nenhum) — bloqueia ou passa? | **BLOQUEIA** | `git add -n` sozinho retorna "Nothing specified, nothing added." (exit 0). Não estageia nada, mas o ADR nomeia explicitamente a "forma sem operando". Bloquear é didático; o usuário aprende a usar a forma correta. |
| **D-U2** | `git add -A <dir>` e `git add -u <dir>` — bloqueiam ou passam? | **PASSAM** | Medição (temp repo, `git add -n`): `-A docs/` e `docs/` produzem exatamente os mesmos arquivos staged. `-A` sem pathspec = tudo; `-A <dir>` = equivalente a `<dir>`. Bloquear `-A docs/` enquanto passa `docs/` é falso positivo puro (AC3). Ver Seção 3.1. |
| **D-U3** | `git stage` (alias de `git add`) — adicionar `case "stage":` ou declarar resíduo? | **ADICIONAR** | Bypass direto de custo zero para o agente. Medição: `git stage -A` → guard exit 0 hoje. Incluir `case "stage":` com lógica idêntica a `case "add":`. |
| **D-U4** | `:/` e `:(top)` (pathspec magic de root) — bloqueiam ou residual? | **BLOQUEIAM** | Medição: `git add -n :/` produz os mesmos arquivos que `git add -n -A`. É "tudo da raiz" em outra grafia. Custo: dois strings adicionais no laço. Sem eles, um agente que leia a documentação do git tem bypass em uma linha. |
| **D-U5** | A regra nova entra por padrão ou atrás de config? | **Por padrão** | Todos os guards existentes são default-on. O guard só roda dentro de projeto trackfw (`RunGitBranch:86-88`), não globalmente. Blast radius: apenas usuários do trackfw com projeto ativo. |

---

## Segundo e terceiro sítios — mesma causa, esta REQ (Regra Dura de Causa Raiz)

### Sítio B — `internal/generators/scaffold.go:732`

O skill template gerado para Claude Code (função `generateClaudeCommandsInner`) emite:

```
git add -A
git commit -m "feat(<escopo>): <descrição do ML>"
```

Após a implementação, esse texto ensina uma forma que o guard bloqueia — o mesmo padrão do
`release tag` que sugeria `git stash` (`cli-parity.md:1661`). Linha 753 do mesmo arquivo emite
`git add docs/roadmaps/` (forma correta com caminho explícito) e continua válida. A linha 732 deve
substituir `git add -A` por `git add <arquivos-exatos>` ou instrução de enumeração via `git status
--short`.

### Sítio C — `internal/generators/agentfiles.go:1949`

Amazon Q Developer CLI tem um denylist nativo além do hook. O padrão atual:

```go
const gitDenyPattern = `^git (commit|push|checkout -b)`
```

Este padrão não bloqueia `git add -A` nativo (a bypass via denylist persiste mesmo quando o hook
funciona). A consistência com os outros CLIs exige estender o padrão para cobrir os comandos de
staging implícito. Ação: ML no roadmap desta REQ para atualizar `gitDenyPattern`.

---

## 1. Completude da enumeração

### 1.1 Lista de superfícies de código

| Superfície | Arquivo | Estado atual | Ação |
|---|---|---|---|
| Regra `git add` em `applyRule` | `internal/guard/gitbranch.go:applyRule` | Ausente — `case "add":` top-level não existe (o `case "add":` de linha 454 está aninhado em `case "worktree":`) | Adicionar |
| Razão de bloqueio | `internal/guard/gitbranch.go` consts linhas 13–54 | Ausente (`reasonAddImplicit` não existe) | Adicionar (linha única, JSON-safe) |
| Despacho da razão | `internal/guard/gitbranch.go:RunGitBranch` linhas 130–164 | Ausente (`case "add-implicit":`) | Adicionar |
| Regra `git stage` | `internal/guard/gitbranch.go:applyRule` | Ausente | Adicionar (D-U3) |
| Testes Go | `internal/guard/gitbranch_test.go` | Ausente | Adicionar (AC4) |
| Cenários de falsificação | `scripts/check-gates-falsify.sh` | Ausente para add | Estender (AC8) |
| Contrato de paridade | `docs/cli-parity.md` | Não lista add/stage entre casos de bloqueio | Atualizar (AC8) |
| Template de skill (Sítio B) | `internal/generators/scaffold.go:732` | `git add -A` no template | Corrigir |
| Denylist Amazon Q (Sítio C) | `internal/generators/agentfiles.go:1949` | Regex omite formas de add implícito | Estender |

### 1.2 Baseline medido (`bin/trackfw guard git-branch`, HEAD)

```
exit 0 — não bloqueado hoje (todos são alvos do ADR D1):
  git add -A          git add --all       git add .
  git add -u          git add --update    git add (bare)
  git -C sub add -A   git -c a=b add .
  git stage -A        git add :/          git add -- .

exit 2 — já bloqueado por regra existente (não precisam de novo código):
  git commit -a -m msg      (case "commit": incondicional, linha 307)
  git commit --all -m msg   (idem)

exit 0 — passam e devem continuar passando:
  git add internal/guard/gitbranch.go      (caminho explícito)
  git add -p        git add --patch        (interativo)
  git add -A docs/  git add -u docs/       (pathspec explícita — ver D-U2)
```

### 1.3 Verificação de paridade v8 (D4/AC8)

`gitBranchGuardScript` (`scaffold.go:1699-1712`) é um thin wrapper de 4 linhas:

```bash
if [ $# -gt 0 ]; then exec trackfw guard git-branch --command "$*"; fi
exec trackfw guard git-branch
```

Não há lógica de bloqueio no bash. Uma única implementação em Go. D4/AC8 significa:
- `case "add":`/`case "stage":` + razão no Go.
- Contrato atualizado em `docs/cli-parity.md`.
- Cenários de falsificação em `scripts/check-gates-falsify.sh`.

**Concordo** com a reinterpretação pós-v8: D4/AC8 = Go + cli-parity.md + falsify gate.

### 1.4 Escopo do guard: apenas dentro de projeto trackfw

`RunGitBranch:86-88` encontra o `trackfw.yaml` subindo o diretório. Se não encontra, retorna 0
(permite). Hooks globais (`~/.claude/settings.json` etc.) também passam por este check.
Consequência: o guard só dispara dentro de projetos trackfw ativos — não para um usuário que digita
`git add -A` num terminal fora de qualquer projeto. O blast radius é o conjunto de usuários com
projeto trackfw, não todos os usuários de git.

### 1.5 Mecanismo de hooks — Q2 respondido

O guard dispara em `PreToolUse` / `BeforeTool` / hook equivalente de **7 CLIs**: Claude Code,
Codex, Gemini CLI, GitHub Copilot, Cursor, Windsurf, Amazon Q Developer. Todos são global deny
(decisão 2026-08-14). Git não tem hook `pre-add`; um humano digitando `git add -A` num terminal
nunca é afetado. Não há chave de desligamento por regra individual; o disable é all-or-nothing por
CLI (`disableAllHooks`, `hooksConfig.enabled`, etc. — vault note 2026-10-10; ADR-2026-10-10 D7
já trata como violação).

---

## 2. Modelo de ameaça

**Adversário: o implementador apressado e o arquiteto otimista.**

### Ameaça A — Implementação baseada em flags, não em escopo

O implementador escreve:
```go
case "add":
    for _, t := range rest {
        if t == "-A" || t == "--all" || t == "." || t == "-u" || t == "--update" {
            return "add-implicit"
        }
    }
    return ""
```

Falhas:
- `git add` bare: `rest` é nil; laço não itera; retorna `""` — não bloqueado.
- `git add :/`: token `:/` não está na lista — não bloqueado (ver D-U4).
- `git add ..`: token `..` não está na lista — não bloqueado.
- `git add -- .`: o `--` chega como token; `.` também; se o implementador só checa o primeiro
  positional e não pula `--`, pode passar.
- `git stage -A`: `case "stage":` ausente.
- Ameaça B abaixo.

Mitigação (AC3): o não-afrouxamento exige um manifesto explícito de todas as formas testadas, com
um teste por forma. O gate de vacuidade (AC4) no `check-gates-falsify.sh` captura a ausência do
`case` inteiro, mas não subsets.

### Ameaça B — Vacuidade dos testes Go (AC4)

O implementador adiciona testes de bloqueio sem o braço que prova que, com a regra ausente, os
mesmos vetores passam. O pattern correto é um cenário de sabotagem em `check-gates-falsify.sh`
(padrão do Cenário 73): compilar uma cópia isolada sem o `case "add":` e afirmar que os vetores
de bloqueio retornam `""`.

**Armadilha crítica de AC5:** o teste que usa input `git commit -m "git add -A"` não testa a
regra de `add`. O subcomando é `"commit"`, bloqueado por `case "commit":` antes de qualquer
análise de `add`. O teste assertaria `"add-implicit"`, falharia, e o testador pensaria que a
regra quebrou o AC5 — quando na verdade o teste estava mal construído. Vetores de AC5 devem
usar tokens não-git como primeiro comando: `trackfw commit -m "…git add -A…"`, `echo "git add -A"`,
`gh pr create --body "…"`.

Caso fail-closed declarado (não é bug): um heredoc nunca fechado (`<< EOF` sem terminador) causa
`stripHeredocBodies` a retornar a string original. Se o corpo contiver `git add -A` em linha
separada, o guard bloqueia. Comportamento correto; declarar nos limites.

### Ameaça C — cli-parity.md e falsify gate não atualizados (AC8)

O implementador entrega Go + testes sem:
- atualizar `docs/cli-parity.md` com os novos casos de bloqueio;
- adicionar cenários de sabotagem em `check-gates-falsify.sh`.

`check-git-branch-guard-hook-schema.sh` só pina o schema JSON de saída, que não muda para regras
novas. O ML pode ser marcado `✅` com o contrato de paridade descoberto.

### Ameaça D — Mensagem multi-linha quebra o JSON de saída (R-6)

`denyOut` (`gitbranch.go:174`) insere a razão em `"permissionDecisionReason":"%s"` com `%s` e sem
escape. Uma razão com `\n`, `"` ou `\` produz JSON inválido, que o Claude Code pode rejeitar ou
interpretar de forma inesperada. Medição: `bin/trackfw guard git-branch --command "git reset
--hard"` já produz JSON inválido por `—` em `reasonResetHard` — causa pré-existente, separada.
A razão de `add-implicit` deve ser linha única sem `"`, sem `\n`, sem `\` soltos. Usar `'` para
as aspas internas (pattern dos outros reasons).

### Ameaça E — Sítios B e C não corrigidos na mesma REQ

Ver seção introdutória. O skill template e o denylist do Amazon Q são enforcement sites da mesma
decisão do ADR.

---

## 3. Alvos de falsificação em ambas as direções

### 3.1 Tabela de semântica git medida (temp repo, `git add -n`)

Configuração: repo com arquivo tracked modificado, arquivo tracked deletado e arquivo untracked
em `docs/` e `sub/`. Resultado de `git add -n <vetor>`:

| Vetor | Arquivos staged (medido) | Escopo | Veredito guard |
|---|---|---|---|
| bare `git add` | nenhum ("Nothing specified") | nulo | **BLOQUEIA** (ADR D1; ver D-U1) |
| `git add -A` | tudo em docs/ + sub/ (mod+del+new) | whole-tree | **BLOQUEIA** |
| `git add .` | tudo em docs/ + sub/ (mod+del+new) | cwd | **BLOQUEIA** |
| `git add -u` | mod+del em docs/ + sub/ (sem untracked) | whole-tree tracked | **BLOQUEIA** |
| `git add --all` | = `-A` | whole-tree | **BLOQUEIA** |
| `git add --update` | = `-u` | whole-tree tracked | **BLOQUEIA** |
| `git add :/` | tudo, igual `-A` | whole-tree | **BLOQUEIA** (D-U4) |
| `git add ..` (de sub/) | tudo em docs/ + sub/ (mod+del+new) | whole-tree via parent | **BLOQUEIA** (ver R-7) |
| `git add -A docs/` | docs/tracked.txt + docs/untracked.txt | = `docs/` | **PASSA** (D-U2) |
| `git add docs/` | docs/tracked.txt + docs/untracked.txt | = `-A docs/` | **PASSA** |
| `git add -u docs/` | docs/tracked.txt (somente mod) | ⊂ `docs/` | **PASSA** (D-U2) |
| `git add . ` (de sub/) | sub/tracked.txt + sub/untracked.txt | = `sub/` | **PASSA** |
| `git add <arquivo>` | o arquivo nomeado | explícito | **PASSA** |
| `git add f1 f2` | os dois arquivos | explícito | **PASSA** |
| `git add -p` | (interativo) | controlado pelo usuário | **PASSA** |
| `git add --patch` | idem | idem | **PASSA** |
| `git add -i` | idem | idem | **PASSA** |
| `git add -n <arq>` | dry-run, não estageia | zero | **PASSA** |
| `git add -A -n` | dry-run, não estageia | zero | **PASSA** (exceção `-n`) |
| `git add -f <arq>` | o arquivo (força add de .gitignore) | explícito | **PASSA** |
| `git add -N <arq>` | intent-to-add no arquivo | explícito | **PASSA** |
| `'*.go'` glob | cada .go no cwd | padrão explícito | **PASSA** |
| `--pathspec-from-file=<f>` | o que o arquivo disser | guard não lê o arquivo | **PASSA** (residual) |

### 3.2 Regra derivada (baseada em escopo, não em flags)

A medição mostra que o discriminante correto é o **escopo efetivo**, não a presença de flag:

```
BLOQUEIA se:
  (1) nenhum positional E nenhuma flag interativa (-p/-i/--patch/--interactive)
       E não é dry-run (-n/--dry-run)
  (2) algum positional E todo positional em {., ./, .., ../, :/, :(top)}

PASSA se:
  (-p/-i/--patch/--interactive) presente, ou
  (-n/--dry-run) presente, ou
  algum positional E pelo menos um fora dos whole-tree tokens acima
```

Nota: `-A <dir>` e `-u <dir>` chegam ao laço de positionals como `["<dir>"]`; se `<dir>` não for
`.`/`..`/`:/`, caem na regra PASSA. Correto pela medição.

### 3.3 Direção falso-negativo (FN) — gate que deve pegar

Sabotagem: remover o `case "add":` de `applyRule`. Vetores que devem falhar:

| Vetor | Gate |
|---|---|
| `git add -A` → exit 0 | Cenário sabotado `check-gates-falsify.sh` |
| `git add .` → exit 0 | Idem, vetor separado |
| `git add` bare → exit 0 | Idem |
| `git stage -A` → exit 0 | Cenário para `stage` (D-U3) |

Guarda de vacuidade (AC4): cada teste de FN precisa do braço com a regra ausente. `MatchSubcommand`
não tem toggle; a prova de vacuidade exige o binário sabotado (`check-gates-falsify.sh`).

### 3.4 Direção falso-positivo (FP) — a direção mais cara (AC3)

| Vetor | Esperado |
|---|---|
| `git add internal/guard/gitbranch.go` | `""` (passa) |
| `git add docs/` | `""` (passa) |
| `git add -A docs/` | `""` (passa) |
| `git add -u docs/` | `""` (passa) |
| `git add -p` | `""` (passa) |
| `git add -n file` | `""` (passa) |
| `git add -A -n` | `""` (passa) |
| `echo "git add -A"` | `""` (passa, AC5) |
| `trackfw commit -m "…git add -A…"` | `""` (passa, AC5) |

---

## 4. Resíduo declarado

| Forma | Por que fica fora |
|---|---|
| `git${IFS}add -A` | Expansão de `$IFS` é preprocessing do shell |
| `{git,add} .` | Brace expansion é preprocessing do shell |
| `git config alias.aa 'add -A'` + `git aa` | Guard tokeniza o comando; não lê `.gitconfig` |
| `bash script.sh` onde o script chama `git add -A` | Guard não executa o script |
| Combined short flag `-AN` (sem pathspec) | Laço compara tokens inteiros; `-AN` ≠ `-A`; ver R-1 |
| `--pathspec-from-file=<arq>` | Guard não lê o arquivo de pathspecs |
| `'*.go'` (glob restrito) | Padrão deliberado e restrito; PASSA |
| Hook desligado via `disableAllHooks` etc. | Coberto por ADR-2026-10-10 D7 (outra regra) |
| `git add -A` fora de projeto trackfw | `FindProjectRoot` falha; guard retorna 0 |
| `$(git add -A)` dentro de outro comando | Guard não expande subshells |
| `cmd \| xargs git add`, `find … -exec git add` | Guard não executa pipelines internos |
| Heredoc aberto sem fechamento | `stripHeredocBodies` retorna string original; corpo de `git add -A` bloqueia — fail-closed deliberado |

---

## 5. Riscos para o implementador (R-n)

**R-1 — Combined short flags (`-AN`, `-uN`)**

`applyRule` itera tokens inteiros: `-AN` ≠ `-A`. Os guards existentes já declaram combined flags
como resíduo. **Recomendação:** declarar resíduo, consistente com `isPushBranchDelete` (`gitbranch.go:503`).
Se D-U2 for aprovado (flags + pathspec passam), o único risco real de combined flags é `git add -AN`
sem pathspec — que fica como resíduo declarado.

**R-2 — Bare `git add` exige check antes do laço**

`len(rest) == 0` verifica antes do laço de tokens. Sem este check, `git add` sozinho passa (laço não
itera). Testar separadamente com `MatchSubcommand("git add")`.

**R-3 — `git add -p` / `-i` com `-A` misturado**

Se D-U2 for aprovado, a regra PASSA quando pathspec explícita está presente. Para o caso de
`git add -Ap` (sem pathspec), a presença de `-p` deve garantir PASSA independentemente de `-A`.
Verificar que a detecção de flag interativa precede a detecção de flag de escopo implícito.

**R-4 — `git add -- .`**

O `--` é terminador de flags. O `.` após `--` chega como positional. Deve bloquear. Verificar que
o laço de positionals ignora o token `--` mas processa o que vem depois.

**R-5 — `git stage` (medido: exit 0 hoje)**

Medição: `echo '{"tool_name":"Bash","tool_input":{"command":"git stage -A"}}' | bin/trackfw guard git-branch` → exit 0. O `case "stage":` não existe em `applyRule`. Adicionar se D-U3 for aprovado.

**R-6 — Mensagem de razão deve ser linha única e JSON-safe**

`denyOut` (`gitbranch.go:174`) insere a razão em `"permissionDecisionReason":"%s"` sem escape. A
razão deve ser linha única sem `"`, `\n`, `\` soltos. Observação: `reasonResetHard` já contém `'`
(apóstrofo) que é seguro em JSON; e `—` (em dash) que é string Unicode válida; a causa do JSON
inválido observado é provavelmente `\` em outros contextos. A razão nova deve seguir o pattern
`'trackfw: git add com escopo implícito bloqueado — use git status --short para enumerar e git add
<arquivo1> <arquivo2> ... para estagiar por nome. Depois trackfw commit -m ...'` — sem quebras de
linha. Verificar com `python3 -c 'import json; json.loads(resultado)'` após implementar.

**R-7 — `git add ..` bloqueia (medido)**

Medição: `git add -n ..` de `sub/` produz exatamente os mesmos arquivos que `git add -n -A` da
raiz. Token `..` deve entrar no conjunto de whole-tree positionals que bloqueiam.

**R-8 — Gate de vacuidade AC4 no Go não tem toggle**

`MatchSubcommand` não tem modo "regra desligada". O padrão de vacuidade exige compilar uma cópia
do binário sem o `case "add":` e verificar que os vetores de bloqueio retornam `""`. Ver padrão
do Cenário 73 em `check-gates-falsify.sh`.

---

## 6. Gate placeholder do roadmap (Wave 0)

O roadmap tem `exit 1` como placeholder. **Gate substituto para o ML-0A** (execute cada linha
separadamente — ParseGates não suporta continuações `\`):

```bash
test -f docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md
grep -q "^## 1\." docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md
grep -q "^## 2\." docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md
grep -q "^## 3\." docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md
grep -q "^## 4\." docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md
grep -qv "case .add.: // NOT YET" internal/guard/gitbranch.go
```

A última linha verifica a ausência de implementação (usa comentário sentinela que o implementador
remove quando começa). Alternativa mais simples: verificar que `bin/trackfw guard git-branch
--command "git add -A"` ainda retorna 0 (pré-implementação). Verificar qual abordagem o
arquiteto prefere.

---

## Apêndice: Conformidade com os quatro critérios mandatórios do ML-0A

| Critério | Evidência |
|---|---|
| **1. Completude da enumeração** | Seção 1: 9 superfícies de código tabeladas; baseline medido (16 vetores); verificação do bash wrapper; análise de escopo do guard fora de projeto; hook survey dos 7 CLIs; dois sítios extras encontrados (scaffold.go:732, agentfiles.go:1949). |
| **2. Modelo de ameaça** | Seção 2: cinco ameaças concretas (A–E), cada uma com mecanismo preciso e mitigação. Adversário: implementador apressado + arquiteto otimista. |
| **3. Falsificação em ambas as direções** | Seção 3: medição de semântica git (temp repo + `git add -n`) para 19 vetores; regra derivada de escopo; subseção FN (gate que captura) e FP (a direção mais cara, AC3). |
| **4. Resíduo declarado** | Seção 4: 12 formas explicitamente fora do alcance com justificativa. |
