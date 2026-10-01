# Wave 3 — Revisão Independente: o estado que governa a branch

> Roadmap: `ROADMAP-2026-10-01-o-estado-que-governa-a-branch-...`
> Parecer: hades-tf | 2026-10-01 | Branch: `fix/estado-que-governa-a-branch`
> Diff revisado: `git diff e104a7f7..HEAD -- internal/ scripts/ docs/cli-parity.md`

---

## 1. Confronto com A1, A2, A3

### A1 — `-z` em todos os leitores de árvore do caminho de decisão

**Verificação em `internal/validator/validator.go` (caminho D2):**

```
$ grep -n "ls-tree\|bytes.Split.*0\|-z" internal/validator/validator.go
...
406: // A1 (ADR-2026-10-01): uses `-z` (NUL-delimited output) and bytes.Split so that filenames with
407: // non-ASCII or special characters are never quoted by git's core.quotepath mechanism. Without `-z`,
411: func mdBasenamesInGitTreeWithError(ref, dirPrefix string) (map[string]bool, error) {
412:     out, err := gitCommand(".", "ls-tree", "-r", "-z", "--name-only", ref, "--", dirPrefix).Output()
417:     for _, entry := range bytes.Split(out, []byte{0}) {
433: func mdBasenamesInGitTree(ref, dirPrefix string) map[string]bool {
434:     set, _ := mdBasenamesInGitTreeWithError(ref, dirPrefix)  // delegates to -z reader
```

`mdBasenamesInGitTreeWithError` usa `-z` + `bytes.Split(out, []byte{0})`. `mdBasenamesInGitTree`
delega a ela. CONFORME.

**Verificação em `internal/auditsurface/auditsurface.go` (sítio irmão A1):**

```
$ grep -n "-z\|NUL\|bytes.Split" internal/auditsurface/auditsurface.go
287: // The -z flag requests NUL-terminated output so that file names containing non-ASCII
291:     cmd := exec.Command("git", "ls-tree", "-r", "-z", "--name-only", ref, "--", dir)
298:     for _, entry := range bytes.Split(out, []byte{0}) {
```

`gitLsTree` em `auditsurface.go:291` também usa `-z` + `bytes.Split`. CONFORME.

**Teste vivo (vetor c):**

Repositório temporário com roadmap `ROADMAP-2026-10-01-çomplexo-test-slug.md` em `done/`:

```
$ ls docs/roadmaps/done/
ROADMAP-2026-10-01-çomplexo-test-slug.md

$ git ls-tree origin/main -- docs/roadmaps/done/
(vazio — arquivo ausente da base)

$ ${TFW} validate
⚠  adr_dir "docs/adr" does not exist
1 warning(s)
```

Nenhuma violação de `branch_has_wip_roadmap`. O leitor `-z` reconheceu o nome acentuado
corretamente e tratou o roadmap como "movido por esta branch". A1 VERIFICADO.

---

### A2 — `GovernanceViolation.Warnings` e propagação do D3 em push/ship

**Verificação em `internal/validator/validator.go`:**

```
$ grep -n "Warnings\|GovernanceViolation\|CheckShipGovernance\|branchWarnings" \
    internal/validator/validator.go | grep -E "4177|4188|4190|4211|4215|4217"

4177: // GovernanceViolation holds the messages from a CheckShipGovernance call.
4179: // A2 (ADR-2026-10-01): Warnings carries non-fatal advisory messages ...
4188:     // A non-nil GovernanceViolation with empty Missing but non-empty Warnings means governance
4190:     Warnings []string
4211: func CheckShipGovernance() *GovernanceViolation {
4215:     branchViolations, branchWarnings, _ := validateBranchHasWIPRoadmap()
4217:     warnings = append(warnings, branchWarnings...)
```

`CheckShipGovernance` agora captura `branchWarnings` e os adiciona a `warnings`. CONFORME.

**Verificação em `internal/commands/push.go`:**

```
$ grep -n "Governance.*degraded\|GovernanceViolation\|Warnings" internal/commands/push.go
163:            for _, w := range gv.Warnings {
164:                fmt.Fprintf(deps.out, "Governance: degraded: %s\n", w)
180:    if gv != nil && len(gv.Warnings) > 0 {
182:        for _, w := range gv.Warnings {
183:            fmt.Fprintf(deps.out, "Governance: degraded: %s\n", w)
```

**Teste vivo (vetor d — sem origin):**

```
$ git remote remove origin
$ ${TFW} validate
⚠  branch_done_scope_unverifiable: cannot verify done/ scope — no resolvable origin ref; \
   accepting match in done/ with degraded confidence
⚠  adr_dir "docs/adr" does not exist
2 warning(s)
```

D3 é aviso (não violação). CORRETO.

```
$ ${TFW} push --dry-run
Governance: degraded: branch_done_scope_unverifiable: cannot verify done/ scope — \
no resolvable origin ref; accepting match in done/ with degraded confidence
```

Exit code: 0. O aviso D3 chega ao `push --dry-run` em vez de "Governance: OK" silencioso. A2 VERIFICADO.

---

### A3 — Vetor (e) declarado como resíduo no ADR

```
$ grep -n "resíduo.*vetor.e\|vetor.e.*resíduo\|alheio.*wip.*done\|mover.*alheio" \
    docs/adr/ADR-2026-10-01-*.md

142: - **Resíduo declarado (vetor (e) do threat model da Wave 0):** uma branch cujo slug casa
143:   um roadmap alheio em `wip/` pode movê-lo para `done/` e passar no item 3 do D2, porque
144:   ele estará ausente da base. A defesa é a revisão do diff (o PR mostra o `git mv`), não o gate.
```

ADR linha 142–144: texto exato exigido pelo A3 presente. A3 VERIFICADO.

---

## 2. Vetores (a)–(e) reimplementados a partir da leitura

### Vetor (a): `origin/main` desatualizado

Repositório temporário: `origin/main` aponta para o commit `init` (sem roadmaps). A branch
`fix/test-slug` tem o roadmap `ROADMAP-2026-10-01-çomplexo-test-slug.md` em `done/` (ausente da
base).

```
$ cat .git/refs/remotes/origin/main   # aponta para init (stale)
9fb20a93afe61dfa9f3d955965ff95ef8df0081a

$ git ls-tree origin/main -- docs/roadmaps/done/
(vazio)

$ ${TFW} validate
⚠  adr_dir "docs/adr" does not exist
1 warning(s)   ← sem violação de branch_has_wip_roadmap
```

Resultado: com `origin/main` stale, o roadmap em `done/` na branch aparece ausente da base e
governa. Direção frouxa (o roadmap pode estar em done/ na main real, invisível pelo stale). É o
resíduo R2 declarado no ADR ("no push, governança roda antes do fetch"). NÃO é exploração nova.

---

### Vetor (b): `roadmap_dir` começando com `-`

```
# trackfw.yaml com roadmap_dir: -evildir/roadmaps
$ ${TFW} validate
✗ branch "fix/test-slug" is a feat/fix/refactor branch but no roadmap is in wip/, blocked/ nor done/
Error: 1 violation(s) found
```

A violação é emitida corretamente. Não há bypass porque o dir disco não existe e o ls-tree recebe
`-evildir/roadmaps/done/` como argumento posicional após `--` — git trata como path inexistente,
rc=0, saída vazia. D3 não dispara. Sem roadmaps no disco, nenhuma inferência governa. CONTIDO.

---

### Vetor (c): nome de roadmap não-ASCII (veja §1 — A1)

Testado com `ROADMAP-2026-10-01-çomplexo-test-slug.md`. O `-z` + `bytes.Split([]byte{0})` lê
corretamente o nome acentuado. CONTIDO.

---

### Vetor (d): remover `origin` para forçar D3 degradado (veja §1 — A2)

```
$ git remote remove origin
$ ${TFW} validate
⚠  branch_done_scope_unverifiable: ... accepting match in done/ with degraded confidence
2 warning(s)   ← nunca violação
```

O D3 degrada com aviso, não bloqueia. Aviso chega ao `push --dry-run`. CONTIDO per ADR (R1).

---

### Vetor (e): roadmap alheio movido de `wip/` para `done/`

Reprodução: adicionado roadmap `ROADMAP-2026-10-01-alheio-fixed.md` a `origin/main/done/` via
plumbing do bare repo (outro "autor"), depois criado link file apontando `fix/test-slug` para ele:

```
$ cat docs/roadmaps/.trackfw-branch-links.json
{"links":[{"branch":"fix/test-slug","roadmap":"ROADMAP-2026-10-01-alheio-fixed.md",
"state":"done","recordedAt":"2026-10-01T00:00:00Z"}]}

$ git ls-tree origin/main -- docs/roadmaps/done/
100644 blob ... docs/roadmaps/done/ROADMAP-2026-10-01-alheio-fixed.md   ← presente na base

$ ${TFW} validate
⚠  adr_dir "docs/adr" does not exist
1 warning(s)   ← sem violação! link D1 bypassa ls-tree check (R7)
```

Confirmação do resíduo R7 do Wave 0: D1 (vínculo escrito) aceita o roadmap alheio sem ls-tree.
Em CI o link file não existe (gitignored + não commitado). Localmente, quem tem write access ao
diretório pode forjar. Comportamento esperado e declarado.

---

## 3. Novos vetores investigados (fora do Wave 0)

### 3.1 Pathspec mágico no `done_dir` — `--literal-pathspecs` ausente

**Observação:** `mdBasenamesInGitTreeWithError` e `gitLsTree` não passam `--literal-pathspecs`
para o comando git. O `--` que separa opções de paths previne injeção de flags, mas não
desativa pathspec magic (`:(glob)`, `:(exclude)`, `:(icase)`).

**Medição:**

```
$ git ls-tree -r -z --name-only origin/main -- ":(glob)docs/roadmaps/done/"
fatal: :(glob)docs/roadmaps/done/: pathspec magic not supported by this command: 'glob'
rc=128   ← D3 dispara (erro detectado)

$ git ls-tree -r -z --name-only origin/main -- ":(exclude)docs/roadmaps/done/"
fatal: ...: pathspec magic not supported by this command: 'exclude'
rc=0     ← atenção: fatal vai para stderr, rc=0 para stdout (vazio)

$ git ls-tree -r -z --name-only origin/main -- ":(icase)docs/roadmaps/done/"
fatal: ...: pathspec magic not supported by this command: 'icase'
rc=0     ← igual: stderr fatal, rc=0
```

Para `:(exclude)` e `:(icase)`, o rc=0 faz `err == nil` em Go. O código interpreta: "nenhum
arquivo em done/ na base" sem disparar D3. Se o `roadmap_dir` no `trackfw.yaml` começar com
`:(icase)` ou `:(exclude)`:

1. A varredura de disco procura em `:(icase)docs/roadmaps/done/` (literal — dir com esse nome)
2. Se o atacante CRIOU esse dir literal com roadmaps (possível no macOS/Linux — `mkdir ":(icase)docs/roadmaps/done"` OK), o ls-tree retorna vazio silenciosamente + D3 não dispara
3. Os roadmaps no dir aparecem como "movidos por esta branch" → branch governada

**Avaliação de risco:**
- Para explorar: o `trackfw.yaml` precisa ter `roadmap_dir: :(icase)...` visível no PR diff
- E roadmaps precisam existir em disco num dir com nome de magic pathspec
- O ataque não é mais eficaz do que qualquer outro `roadmap_dir` inválido: qualquer caminho
  inventado sem correspondência em origin/main produziria o mesmo resultado
- Em CI: o workflow gerado por scaffold faz `git fetch --depth=1 origin ...` ANTES de validate,
  mas origin/main não teria o dir com magic pathspec, portanto o bypass via link file não se aplica
  aqui; e sem link file, a inferência depende do token-match + done/ no disco

**Severidade: BAIXA.** Não exploita novo vetor além dos residuais já declarados (R4 em done/,
R7 via link). A defense-in-depth correta seria adicionar `--literal-pathspecs` ao git env ou
substituir `gitCommand(".", "ls-tree", ...)` por `gitCommand(".", "-c", "core.quotepath=false",
"--literal-pathspecs", "ls-tree", ...)`. Não bloqueia o PR, mas deve ser registrado como dívida.

---

### 3.2 Vínculo escrito para roadmap alheio em `done/` (veja vetor e acima)

Confirmado: D1 (link file) governa mesmo que o roadmap esteja em `done/` em `origin/main` (alheio).
Em CI: link file é gitignored e não commitado → não explorável em PR. Resíduo aceitável (R7).

---

### 3.3 `branch new` com casamento em `wip/` alheio — D6 residual

**Teste:**

```
# wip/ contém ROADMAP-2026-10-01-test-slug-migration.md (tokens "test","slug")
# branch fix/test-slug também tem tokens "test","slug"

$ NO_COLOR=1 ${TFW} branch new fix/test-slug --dry-run
[dry-run] would create branch "fix/test-slug" (git checkout -b fix/test-slug)
```

O branch new permite criação por casamento de 2 tokens com roadmap alheio. Comportamento
esperado — D6 resíduo declarado no ADR linha 138–139:

```
- **Resíduo declarado:** a inferência contra `wip/ ∪ blocked/` continua frouxa (a relação não
  muda, D6).
```

CONFIRMADO COMO RESÍDUO.

---

## 4. Conformidade dos pinos e contratos

```
# Scripts/pinos — verificado pela auditoria da Wave 1:
$ GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh
EXIT=0 (30/30 pinos — confirmado em ML-1B audit)

# Mensagem "nor done/" não aparece fora de barrier.go
$ grep -rn "nor done/" internal/commands/*.go
internal/commands/barrier.go:...   ← único match esperado, fora de escopo
```

Pinos e textos conformes.

---

## 5. Resíduos novos identificados nesta revisão

**RN1 — `--literal-pathspecs` ausente nos leitores de árvore**
`:(exclude)` e `:(icase)` em `roadmap_dir` retornam rc=0 com saída vazia sem disparar D3. Não
explorável além dos residuais já declarados. Dívida técnica (defense-in-depth). Arquivos:
`internal/validator/validator.go:412`, `internal/auditsurface/auditsurface.go:291`.

Nenhum resíduo novo com severidade suficiente para bloquear o PR.

---

## Veredito: APROVA COM AJUSTES

A1, A2 e A3 estão implementados e verificados contra o binário real.

Os vetores (a)–(e) do Wave 0 produzem os comportamentos esperados: os contidos estão contidos,
os residuais estão declarados no ADR e se comportam como declarados.

O único achado novo é RN1 (`--literal-pathspecs` ausente), de severidade baixa e sem bypass
explorável além do residual de `roadmap_dir` arbitrário.

### Ajustes

1. **Dívida técnica RN1** — não bloqueia o PR, mas deve ser registrada como ML adicional
   ou issue de follow-up: adicionar `--literal-pathspecs` (ou `git -c pathspec.literal=true`) ao
   `gitCommand` em `internal/validator/validator.go:412` e
   `internal/auditsurface/auditsurface.go:291`. Sem essa flag, paths com `:(icase)`/`:(exclude)`
   retornam rc=0 silenciosamente em vez de disparar D3.
