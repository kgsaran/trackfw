# Wave 0 — Threat Model: o estado que governa a branch

> Roadmap: `ROADMAP-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md`
> Parecer: hades-tf | 2026-10-01 | Branch: `fix/estado-que-governa-a-branch`

---

## 1. Completude da Enumeração

### 1.1 Enumeração dos sítios

O Context do roadmap lista 7 sítios. O `grep -rn` em `internal/`, `scripts/` e `internal/generators/` confirma os seguintes consumidores que decidem se uma branch é governada:

| # | Sítio | Estado atual | Artefato |
|---|---|---|---|
| 1 | `internal/commands/branch.go:runBranchNew` (~:155) | `matchSlug(slug, wip, done)` | muda para `wip` only (D1) |
| 2 | `internal/commands/commit.go` (~:356) | `matchSlug + branchLink(wip, done)` | resolução nova (D2) |
| 3 | `internal/validator/validator.go:validateBranchHasWIPRoadmap` (~:3939) | `ResolveBranchRoadmap(wip, done)` | resolução nova (D2) |
| 4 | `internal/validator/branchlink.go:BranchLinkFor` | `InScope` em `wip∪done` | escopo `wip∪blocked∪done` (D4) |
| 5 | `internal/validator/branchlink.go:RecordBranchLink` | `MatchRoadmapsForBranchSlug(wip, done)` | só `wip` (D1) |
| 6 | `internal/validator/branchlink.go:ResolveBranchRoadmap` | inferência `wip∪done` | D2/D3 |
| 7 | `CheckShipGovernance` → `push`/`ship` | via `validateBranchHasWIPRoadmap` | mesma resolução do validate |

Scripts com texto hardcoded que muda:
- `scripts/check-validate-rule-pins.sh` PIN3/PIN4: marcadores `MARKER_NOMATCH="no roadmap is in wip/ nor done/"` e `MARKER_DIFF="no matching roadmap in wip/ nor done/"` (linhas 328 e 342)

Geradores: nenhum que decida governança diretamente. `internal/generators/scaffold.go` (~:2466-2494) gera o passo do `trackfw-gate.yml` que faz `git fetch --depth=1 origin "+refs/heads/main:refs/remotes/origin/main"` antes de `trackfw validate`. Esse passo é confirmado e cobre a maioria dos cenários de CI.

### 1.2 Sítio ausente da tabela — LACUNA

**`GovernanceViolation.Missing []string`** (validator.go) e **`CheckShipGovernance`** (validator.go:4113-4125) têm comportamento relevante para o D3:

```go
// validator.go:4117
branchViolations, _, _ := validateBranchHasWIPRoadmap()
```

O segundo retorno (warnings) é **descartado**. `GovernanceViolation` tem apenas o campo `Missing []string`. O `defaultCheckGovernance` em `ship.go:242-248` retorna `gv.Missing` — sem campo para warnings. O mesmo `defaultCheckGovernance` é usado por `push.go:96`.

Consequência: **o aviso D3 (`branch_done_scope_unverifiable`) é silenciosamente descartado em `push` e `ship`**. O princípio "Não fica silencioso" do D3 no ADR é violado por construção para as duas portas hard gate.

### 1.3 Veredito de enumeração

A tabela do Context está **substancialmente completa** para os sítios de código. A lacuna é na **plumagem do sinal de retorno**: `CheckShipGovernance`/`GovernanceViolation` não têm campo `Warnings`, tornando o D3 mudo nas portas de push e ship. Esse é um defeito estrutural que deve entrar no escopo do ML-1A.

---

## 2. Threat Model

O adversário no Wave 0 é o implementador apressado e o arquiteto otimista — alguém que obtém "branch governada" sem roadmap legítimo ou que torna o gate mudo sem quebrar nenhuma regra escrita.

### Vector (a): `origin/main` forjada ou desatualizada via `git update-ref`

**Mecanismo**: `deriveOriginDefaultBranch()` lê `refs/remotes/origin/` via `git for-each-ref`. O arquivo `.git/refs/remotes/origin/main` é um arquivo texto simples de 41 bytes. Qualquer processo com acesso ao filesystem do `.git/` pode sobrescrevê-lo.

**Medição**:
```
$ ls -la .git/refs/remotes/origin/main
-rw-r--r--@ 41 kgsaran  1 Oct 15:02  .git/refs/remotes/origin/main
$ cat .git/refs/remotes/origin/main
e104a7f7742daaac6f87f6a8b3730af08e14b5b3
```

**Ataque local**: Aponta `refs/remotes/origin/main` para um commit antigo (antes de o roadmap existir). `ls-tree <old-commit> -- done/` retorna vazio → roadmap parece "movido por esta branch" → D2 item 3 passa.

**Em CI**: NÃO explorável. O passo gerado em `scaffold.go:2488` executa `git fetch --depth=1 origin "+refs/heads/main:refs/remotes/origin/main"` ANTES de `trackfw validate`. O servidor GitHub retorna o valor real; o autor do PR não controla o que o servidor entrega.

**Risco local adicional (stale)**: Em `trackfw push`, a governança roda no Step 2 e o fetch (`git fetch origin --prune`) roda no Step 3 (push.go:217-222). Logo, push avalia a governança contra `origin/main` local, que pode estar desatualizado. Um roadmap mergeado para main após o último fetch local apareceria como "ausente de done/ na base" — passando quando deveria bloquear. Direção frouxa, sem impacto de segurança grave.

**Severidade**: LOCAL only. Requer acesso de escrita ao `.git/`. Não é explorável em CI via PR.

---

### Vector (b): `roadmap_dir` hostil no `trackfw.yaml`

A proposta D2 chama `git ls-tree <ref> -- <done_dir>` onde `done_dir = resolveStateDirs(diskCfg, "done")[i]` = `cfg.RoadmapDir + "/done"`. `resolveStateDirs` não valida o conteúdo de `cfg.RoadmapDir`. Medições:

**`..` no caminho**:
```bash
$ git ls-tree -r --name-only origin/main -- "../../etc/done" > /dev/null 2>&1
$ echo $?
128   # fatal: '../../etc/done' is outside repository
```
rc=128 → `err != nil` → D3 dispara com aviso. CONTIDO.

**Caminho absoluto**:
```bash
$ git ls-tree -r --name-only origin/main -- "/tmp/evil/done" > /dev/null 2>&1
$ echo $?
128   # fatal: Invalid path '/private/tmp/evil'
```
rc=128 → D3 dispara. CONTIDO.

**Caminho iniciando com `-`**:
```bash
$ git ls-tree -r --name-only origin/main -- "-evil/done" > /dev/null 2>&1
$ echo $?
0     # rc=0, saída vazia
```
**rc=0 com saída vazia.** D3 NÃO dispara. `err == nil`. O código interpreta: "nenhum arquivo em done/ na base" → todos os roadmaps em done/ no disco parecem "movidos por esta branch". ATENÇÃO: para que haja um bypass real, os roadmaps também precisam existir em disco no diretório `-evil/done/`. Como esse diretório não existe, a inferência retorna vazio e a branch fica sem governança. Efeito líquido: **a governança fica quebrada, não bypassada**.

**Caminho com espaço** (ex. `"docs my roadmaps"`): passado como argumento único via `exec.Command` → sem splitting de shell → sem injeção. `git ls-tree` retrata o diretório com espaço como pathspec → rc=0, resultado correto.

**Conclusão para (b)**: Todos os casos injetáveis que levam a rc=128 são contidos pelo D3. O único caso rc=0 (dash prefix) não gera bypass útil porque os roadmaps precisam existir no diretório errado para a inferência encontrá-los.

---

### Vector (c): nome de arquivo com caractere especial / quoting do `ls-tree`

**Contexto**: Git com `core.quotepath=true` (padrão — não configurado explicitamente neste repo) cita os nomes de arquivo com caracteres não-ASCII ou de controle no output de `ls-tree`. Medição via `git status`:

```
$ git status --short (num repo temporário com arquivo não-ASCII)
A  "done/ROADMAP-2026-09-01-com-\303\247-teste.md"   # com ç = \xc3\xa7
A  done/ROADMAP-2026-09-01-normal.md
```

O arquivo com `ç` aparece com aspas e sequência de escape. Sem `-z`, `ls-tree --name-only` usa o mesmo mecanismo: `filepath.Base("\"done/ROADMAP-2026-09-01-com-\\303\\247-teste.md\"")` retornaria `"\\303\\247-teste.md"` — NÃO casa com o nome real. O roadmap pareceria ausente da base → passa como "movido por esta branch".

**Com `-z`**: separação por NUL, sem citação. Bytes reais. `os.SplitAfter(out, '\x00')` → basename correto.

```bash
# od do output com -z (dois primeiros paths):
0000000    d   o   c   s   /   r   o   a   d   m   a   p   s   /   d   o
0000020    n   e  \0   ...    # \0 como separador, sem aspas
```

**Sítio adicional (mesma causa, diferente função)**: `mdBasenamesInGitTree` (validator.go:406) e `auditsurface.go:288` também chamam `ls-tree` sem `-z`. São sítios da família `scopeRedirectViolations`, fora do escopo desta REQ, mas com o mesmo mecanismo. Per Regra Dura de Causa Raiz, devem ser incluídos no ML-1A ou em ML adicional nesta REQ.

**Conclusão para (c)**: A chamada ls-tree proposta para D2 **deve usar `-z`** e NUL-split. Sem isso, roadmaps com nomes não-ASCII ou com `\n` (POSIX permite, raro na prática) seriam tratados como ausentes da base, passando silenciosamente.

---

### Vector (d): forçar o D3 removendo `origin`

**Mecanismo**: `git remote remove origin` apaga `refs/remotes/origin/`. `deriveOriginDefaultBranch()` → `git for-each-ref refs/remotes/origin/` → saída vazia → retorna `ok=false` → D3 dispara.

**D3 com aviso — mas aviso DESCARTADO em push/ship**: Como demonstrado na Seção 1.2, `CheckShipGovernance` descarta o segundo retorno de `validateBranchHasWIPRoadmap`. O aviso `branch_done_scope_unverifiable` que D3 deveria emitir **não chega ao usuário** nas portas de push e ship.

**Comportamento real**:
- `trackfw validate`: aviso visível (retorno do `ValidateUnfiltered`)
- `trackfw commit`: aviso propagado (se o ML-1A propagar corretamente)
- `trackfw push`/`ship`: aviso silenciosamente descartado

**Ataque**: remover `origin` localmente → D3 degrada para `wip∪done` frouxa, sem nenhum aviso visível em push. O gate hard de push passa como se tudo estivesse correto.

**Em CI**: origin sempre configurado pelo checkout do GitHub Actions. Não explorável em CI via PR.

**Requer**: acesso local ao `.git/config`. Mesmo limite de trust de (a).

---

### Vector (e): mover roadmap alheio para `done/` para "reivindicá-lo"

**Mecanismo**: Qualquer autor de PR que tenha branch com nome que case por slug com um roadmap em `wip/` pode mover esse roadmap para `done/` na sua branch. O D2 item 3 verifica:
- roadmap em `done/` no disco ✓
- casa o slug da branch ✓
- AUSENTE de `done/` em `origin/main` ✓ (estava em `wip/`, não em `done/`)

Todos os três critérios são satisfeitos → "movido por esta branch" → governance passa.

**Sem nenhum acesso especial**: o atacante apenas cria a branch com nome que case por substring ou tokens com o roadmap alvo, move o arquivo com `git mv`, e faz commit.

**Defesa existente**: o PR diff mostra o arquivo movido de `wip/` para `done/`. O dono do roadmap vê o movimento. Code review é a única barreira.

**Não há defesa técnica no gate**: o gate não tem conceito de propriedade de roadmap.

**Variante com roadmap novo**: criar um arquivo `ROADMAP-2026-10-01-meu-slug.md` em `done/` na própria branch — ausente da base por construção. Isso é o fluxo legítimo da Definition of Done; é também o caminho para qualquer roadmap fake com slug certo.

---

### Vector (f): forja do arquivo de vínculo (`.trackfw-branch-links.json`)

**Arquivo é gitignored**:
```bash
$ git check-ignore -v "docs/roadmaps/.trackfw-branch-links.json"
.gitignore:21:docs/roadmaps/.trackfw-branch-links.json	docs/roadmaps/.trackfw-branch-links.json
```

**NÃO commitado**:
```bash
$ git ls-tree -r origin/main -- "docs/roadmaps/.trackfw-branch-links.json"
(saída vazia — arquivo não rastreado em origin/main)
```

**Em CI**: `readBranchLinks()` retorna nil (arquivo ausente). D1 (written-link) não atua em CI. Apenas inferência. NÃO explorável em CI via PR normal.

**Localmente**: quem tem write access ao diretório pode editar o arquivo JSON e apontar qualquer branch para qualquer roadmap em `done/`. D1 aceitaria sem ls-tree check.

**Residual**: se alguém fizer `git add -f docs/roadmaps/.trackfw-branch-links.json`, o arquivo entra no commit e CI passaria a usá-lo. Não há gate que rejeite um link file comprometido no PR.

---

### Vector (g): base auto-referente (`deriveOriginDefaultBranch` resolve a própria branch)

**Mecanismo**: quando o repo tem apenas UMA branch em `refs/remotes/origin/` (sem `origin/main` ou `origin/master`), `deriveOriginDefaultBranch` usa o fallback de "branch única" (validator_credential_guard_integrity.go:354-356). Se a branch de feature foi empurrada e é a única em origin, ela se torna a "base".

**Consequência para D2**: `ls-tree origin/fix/my-branch -- done/` incluiria o roadmap que a própria branch moveu para done/ → o roadmap aparece "presente na base" → D2 item 3 falha → branch BLOQUEADA mesmo para a DoD legítima.

**Direção do erro**: FALSO POSITIVO (bloqueia quando deveria passar) — direção restrita erroneamente.

**Em CI**: `trackfw-gate.yml` faz `git fetch --depth=1 origin "+refs/heads/main:refs/remotes/origin/main"` antes de validate. Se o repo tem `main`, origin/main é preferido. Só se o repo não tiver main/master E a branch for a única em origin é que o (g) dispara. Cenário degenerate, mas possível em novos repositórios.

---

## 3. Alvos de Falsificação nas Duas Direções

### Sítio 1: D1 — criação consulta só `wip/`

**Frouxo (regredir)**: `RecordBranchLink` volta a resolver contra `wip∪done`. Branch criada com casamento em `done/` recebe vínculo escrito para roadmap alheio concluído.
- Onde entra: `branchlink.go:RecordBranchLink`, linha `MatchRoadmapsForBranchSlug(slug, wipDirs, doneDirs)` onde `doneDirs` é incluído
- Gate que pega: AC3 (teste: `wip/` + `done/` com casamento único → vínculo aponta para `wip/`)

**Restrito (sobre-apertar)**: `runBranchNew` não emite mensagem nomeando roadmaps em `done/`. Usuário não sabe que precisa de `roadmap move wip`.
- Onde entra: `branch.go:166-168`, remoção do texto com roadmaps de `done/`
- Gate que pega: AC2 (mensagem deve citar roadmaps de `done/` e orientar `roadmap move wip`)

---

### Sítio 2: D2 — branch existente aceita `blocked/` e `done/` restrito

**Frouxo (regredir)**: `ResolveBranchRoadmap` usa `wip∪done` sem verificação de `ls-tree`. Todo `done/` passa.
- Onde entra: `branchlink.go:186`, uso de `doneDirs` sem ls-tree check
- Gate que pega: AC5(b) (roadmap em `done/` em origin/main bloqueia)

**Restrito (sobre-apertar)**: `ls-tree` retorna corretamente que o roadmap ESTÁ em `done/` em origin/main (colocado por um PR anterior), mas o branch atual também casou por slug e é legítimo.
- Onde entra: lógica invertida no parser de saída do `ls-tree` (ausente ↔ presente trocados)
- Gate que pega: AC5(a) (roadmap movido pela própria branch, ausente de done/ em origin/main, passa)

**Restrito (over-tighten a DoD)**: O passo `trackfw roadmap move <name> done` que fecha o roadmap no mesmo PR — o roadmap vai de `wip/` para `done/` nesta branch. `ls-tree origin/main -- done/` não o encontra (estava em `wip/`). Isso é exatamente o DoD legítimo. Se a lógica for invertida ("presente na base → passa"), o DoD seria bloqueado.
- Gate que pega: AC5(a) medido no próprio roadmap desta REQ

---

### Sítio 3: D3 — base inverificável degrada com aviso

**Frouxo (D3 silencioso)**: O aviso `branch_done_scope_unverifiable` é descartado em push/ship por `CheckShipGovernance`. Degradação ocorre sem notificação ao usuário.
- Onde entra: `validator.go:4117` (`branchViolations, _, _ := validateBranchHasWIPRoadmap()`)
- Gate que pega: **NENHUM GATE HOJE**. AC6 requer "aviso emitido" mas não especifica que push/ship devem propagá-lo. Essa é a lacuna.

**Restrito (D3 vira violação)**: D3 passa de aviso para violação. Bloqueia commit/push quando origin está inacessível (offline, worktree sem fetch recente).
- Onde entra: `validateBranchHasWIPRoadmap` retorna `branchViolations` em vez de `warnings` para o caso D3
- Gate que pega: AC6 (sem origin → aviso, nunca violação)

---

### Sítio 4: `BranchLinkStaleWarning` (D4)

**Frouxo (stale silencioso)**: `BranchLinkStaleWarning` removida ou não emitida quando link aponta para roadmap fora de `wip∪blocked∪done`.
- Onde entra: remoção da chamada em `ResolveBranchRoadmap`
- Gate que pega: PIN4 do `check-validate-rule-pins.sh`

**Restrito (stale vira violação)**: `BranchLinkStaleWarning` vira erro de violação. Uma branch com roadmap em `blocked/` bloquearia por stale.
- Onde entra: emissão via `violations` em vez de `warnings`
- Gate que pega: AC4 (commit com roadmap em blocked/ passa sem violação de stale)

---

## 4. Resíduo Declarado

### 4.1 Resíduos aceitos pelo design (ADR já declara)

**R1 — Base inverificável (D3)**: Sem `origin` resolvível, `done/` usa inferência frouxa. Documentado no ADR.

**R2 — PR contra base não-default**: "movido por esta branch" é medido contra a default de `origin`, não contra a base real do PR. Erro na direção frouxa (o roadmap pode estar ausente da default e presente na base real). Documentado no ADR.

**R3 — Inferência contra `wip/∪blocked/` continua frouxa**: A relação de casamento (substring + tokens) não muda (D6). Com `wip/` pequeno o risco é baixo. Documentado no ADR.

### 4.2 Resíduos novos identificados neste Wave 0

**R4 — Vetor (e): reivindicação de roadmap alheio de `wip/`**: Qualquer PR autor com branch de slug compatível pode mover um roadmap de `wip/` para `done/` na sua branch e passar D2. Detectável apenas por code review. NÃO documentado no ADR. **Deve ser adicionado como resíduo declarado.**

**R5 — Aviso D3 descartado em push/ship**: `GovernanceViolation` não tem campo `Warnings`. O aviso D3 é silenciado nos gates hard (push, ship). O ADR afirma "Não fica silencioso" — contradição por construção. **Deve ser corrigido no ML-1A (não é resíduo aceitável).**

**R6 — Sítio irmão `mdBasenamesInGitTree` sem `-z`**: `validator.go:406` e `auditsurface.go:288` chamam `ls-tree` sem `-z`. Mesma causa (ausência de `-z`) em função diferente, propósito diferente (`scopeRedirectViolations`). Per Regra Dura de Causa Raiz, deve entrar no escopo desta REQ ou em ML adicional.

**R7 — Vínculo escrito em `done/` bypassa ls-tree check (D1 sobre D2)**: D2 item 1 (vínculo escrito) aceita roadmap em `done/` sem verificar "movido por esta branch". Um vínculo apontando para roadmap alheio em `done/` (escrito localmente ou por `git add -f` do link file) passa sem ls-tree. Em CI o link file não existe (gitignored, não commitado), então não há exploração via PR.

### 4.3 Opinião explícita sobre D3 no `push` (hard gate)

**Pergunta**: O D3 fail-open com aviso é aceitável para o `push`?

**Contexto medido**:
- Step order em `push.go`: Step 2 (governance) precede Step 3 (fetch). Push avalia contra `origin/main` local potencialmente stale.
- `CheckShipGovernance` descarta o aviso D3 hoje.
- Em CI: `trackfw-gate.yml` faz fetch ANTES de validate. Push local pode usar ref stale.

**Argumento para fail-open**:
- A degradação D3 ainda requer que um roadmap case o slug em `wip∪done` — não é bypass arbitrário.
- Fail-closed bloquearia push offline ou em worktrees sem fetch recente (mesma razão do ADR-2026-09-26).
- A degradação é um estágio mais frouxo, não uma porta aberta.

**Argumento contra fail-open-silencioso (o problema real)**:
- O ADR diz explicitamente "Não fica silencioso". A implementação atual descarta o aviso.
- Um push com D3 degradado e sem aviso aparece como "Governance: OK" — comunicação falsa para o usuário.
- O argumento do ADR ("não deadloca o commit da correção") se aplica a `commit` e `validate`, não a `push`. Push precisa da rede de qualquer forma (para empurrar o commit). Se push já requer rede, a justificativa offline é mais fraca para push do que para commit.

**Veredito para D3 em push**: O fail-open é **aceitável** como comportamento. O silêncio é **inaceitável** e precisa ser corrigido. A solução não é mudar o comportamento (fail-closed), mas garantir que o aviso chegue ao usuário: `GovernanceViolation` deve ter um campo `Warnings []string`, e `defaultCheckGovernance`/`runPush` devem imprimir os warnings (ex. precedidos de `"Governance: degraded (origin unavailable): ..."`) mesmo quando `violations` está vazio.

---

## Veredito: APROVA COM AJUSTES

O design do ADR é sólido. Os vetores mais graves são contidos pela construção (ls-tree rc=128 → D3, link file não commitado em CI). Os ajustes abaixo devem ser endereçados antes ou durante o ML-1A — não bloqueiam o design, mas bloqueiam a implementação que ignore qualquer um deles.

### Ajustes obrigatórios

**A1 — Uso explícito de `-z` no ls-tree D2 (ML-1A item 4)**
O ADR menciona `git ls-tree -z` no texto de D2, mas o ML-1A item 4 lista apenas `gitCommand(... "ls-tree", "-z", "--name-only", ...)` sem especificar que o parsing deve usar `bytes.Split(out, []byte{0})` em vez de `strings.Split`. O ML-1A deve declarar isso explicitamente, e o mesmo fix deve cobrir `mdBasenamesInGitTree` (validator.go:406) como sítio irmão da mesma causa.

**A2 — `GovernanceViolation.Warnings` e propagação do D3 em push/ship (ML-1A + roadmap)**
`CheckShipGovernance` em `validator.go:4117` usa `branchViolations, _, _ := validateBranchHasWIPRoadmap()`, descartando o aviso D3. `GovernanceViolation` não tem campo `Warnings`. O ADR afirma "Não fica silencioso" — essa garantia é violada por construção para push e ship hoje. O ML-1A deve incluir: (i) adicionar `Warnings []string` a `GovernanceViolation`, (ii) propagar no `CheckShipGovernance`, (iii) imprimir em `runPush`/`defaultCheckGovernance` quando presente. O roadmap deve refletir esse escopo adicional.

**A3 — Declarar vetor (e) como resíduo no ADR**
O ADR não nomeia a possibilidade de mover um roadmap alheio de `wip/` para `done/` para reivindicá-lo. Deve ser adicionado à seção "Resíduo declarado": "A inferência de 'movido por esta branch' não tem conceito de propriedade de roadmap. Qualquer branch com slug compatível pode mover um roadmap de wip/ para done/ e passar D2 item 3; a defesa é code review, não o gate."
