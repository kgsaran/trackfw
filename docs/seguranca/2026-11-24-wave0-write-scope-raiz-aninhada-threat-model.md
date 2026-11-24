# Wave 0 — Threat Model: Guard write-scope re-enraizado por trackfw.yaml aninhado

**REQ:** REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md  
**Roadmap:** ROADMAP-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md  
**Revisor:** Hades (security), ML-0A  
**Data:** 2026-11-24  
**Binário medido:** `go build -o $TMPDIR/tf ./cmd/trackfw` em `fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado`, HEAD `b75794c9`

**HOME isolation pre-check (antes de toda medição):**

```
printf 'x' | env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H/.x" XDG_CONFIG_HOME="$H/.c" \
  sh -c 'echo HOME=$HOME'
→ HOME=/var/folders/js/z4mqvnnd3sj773g8tcntk1000000gn/T/tmp.QIIG3hS1Yj/home
```

Sintaxe em todas as chamadas: `printf '{"tool_name":"Write",...}' | env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H/.x" XDG_CONFIG_HOME="$H/.c" "$TF" guard write-scope`. Nunca `$PROJ` dentro de aspas simples — sempre via `%s` expandido.

---

## 1. Completude da Enumeração

> *A lista de superfícies no roadmap está completa? Nomear o que está ausente, ou mostrar que a lista está fechada.*

### Tabela de achados (por severidade)

| ID | Severidade | Mecanismo | rc medido | Status |
|---|---|---|---|---|
| F-5A | BLOQUEANTE | Arquivo regular `.claude/trackfw.yaml` → `findProjectRoot` re-enraíza; Step 1 rc=0, Step 2 rc=0; também reachable sob lease=[.claude/] mesmo com cwd normal (D3 bypass independe de cwd); e via Write sob lease=[docs/] para frozen paths dentro de docs/ (baseline=2, step1=0, arm=0 medidos) | 0, 0 | Não corrigido |
| F-5B | BLOQUEANTE | Write a `.../trackfw.yaml/x` → Lstat aceita dir; `ErrNotRegularFile` nunca checado; Step 2 (dir planted na fixture) → rc=0 | 0 (step 1); 0 (step 2 medido) | Não corrigido |
| F-5C | BLOQUEANTE (macOS/Windows) | `TRACKFW.YAML` regular; Lstat case-insensitive aceita | 0 | Não corrigido |
| F-5D (D4 Bash-only) | RESIDUAL | Bash plant + cwd=.claude/ → outer lease não carregada; D4 bypass exige DUAS condições: alvo E cwd no ancestral plantado (medido: A(arm=0,counter-arm=2) condição cwd; B(arm=2) condição target); via Write pré-fix = F-5A sob lease pertinente; via Bash = R-1 | A(arm=0,counter-arm=2); B(arm=2) | R-1 |
| F-5-FIFO | MÉDIO | FIFO nomeado `trackfw.yaml` → `ReadFile` bloqueia indefinidamente; planta requer Bash (mkfifo) | HUNG medido | Não corrigido |
| F-5-cred | MÉDIO (mesmo mecanismo; MÉDIO e não BLOQUEANTE porque F-5A abre um Write path a `.claude/settings.json` que é congelado — controle intacto derrotado pela primeira vez; F-5-cred derrota o controle de credential global, mas Write a `~/.claude/settings.json` já é aberto via R-10 — controle com bypass pré-existente. MÉDIO condicional: se Claude Code bloquear Write em `~/.claude/` nativamente, reavaliar.) | `RunCredentialGlobal` (`--global`, linha 361): planta `cwd/trackfw.yaml` com `mode: warn` → downgrade de "block" para "warn"; medido: pre=2, arm=0 (binário, yaml na fixture, cwd=sub; mesmo code path que NOPROJ por L365+L448); sessão fora de projeto: write-scope é projeto-level (por leitura do sítio de instalação — agentfiles.go:266-278; nenhum wiring global) → downgrade via Write não fechado por (a) → Wave 1 ML em credential.go | pre=2, arm=0 (binário, yaml na fixture; transferência para NOPROJ por leitura L365+L448) | AC3 desta REQ; Wave 1 ML: credential.go monotonidade read-side + normalização Windows |
| R-6 | MÉDIO (pré-existente) | OpenCode `tool_name:"write"` lowercase bypassa Step 4; rc=0; gap sem ADR de residual | 0 medido | Fora de escopo |
| MultiEdit | LOW (pré-existente) | MultiEdit não em `writeScopeWriteTools`; rc=0 | 0 | Fora de escopo |
| R-Bash | RESIDUAL (pre-declared) | 12/12 agentes têm Bash; escapa de D3/D4 | — | ADR-2026-10-23 |

### Ação 1 — Reprodução do F-5

#### Mecanismo comum

`findProjectRoot` (`writescope.go:504`) caminha para cima a partir do diretório do arquivo-alvo
e retorna o **primeiro** ancestral que tem `trackfw.yaml` via `os.Lstat`:

```go
// writescope.go:516–523
yamlPath := filepath.Join(dir, "trackfw.yaml")
if _, err := os.Lstat(yamlPath); err == nil {
    return dir, true
}
```

Nenhuma verificação de tipo no objeto encontrado. Qualquer objeto com esse nome (arquivo
regular, diretório, FIFO, symlink) faz o guard re-enraizar naquele ancestral. `buildFrozenList`
inclui `writeScopeSubagentFrozenRelPaths` hardcoded, mas cada entrada é resolvida
relativamente à `projectRoot` retornada. A nova sub-raiz distorce todas as comparações.

#### Sub-variante A: arquivo regular plantado em subdiretório (D3 bypass)

**Medição — baseline (outer yaml sem `write_scope.frozen_paths`):**

```
H=$(mktemp -d)/home; mkdir -p "$H"; PROJ=$(mktemp -d); mkdir -p "$PROJ/.claude"
printf 'adr_dirs: [docs/adr]\n' > "$PROJ/trackfw.yaml"

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ denied: ".../.claude/settings.json" is a frozen path (.claude/settings.json)
rc=2  ✓ (proteção vem exclusivamente de writeScopeSubagentFrozenRelPaths, não do yaml)
```

**Medição — Step 1 (planta `.claude/trackfw.yaml`, sem `write_scope.frozen_paths`):**

```
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/trackfw.yaml"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

Planta executada: `printf 'adr_dirs: []\n' > "$PROJ/.claude/trackfw.yaml"`

**Medição — Step 2 (re-enraizado em `.claude/`):**

```
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**rc medidos:** baseline=2, step1=0, step2=0.

**Raiz do bypass:** após re-enraizamento (`projectRoot = .claude/`), `.claude/settings.json`
de `writeScopeSubagentFrozenRelPaths` torna-se `proj/.claude/.claude/settings.json`
≠ alvo `proj/.claude/settings.json`.

#### Sub-variante A — contra-braço sessão principal

**Medição — baseline com `write_scope.frozen_paths: [.claude/settings.json]`:**

```
printf 'adr_dirs: [docs/adr]\nwrite_scope:\n  frozen_paths: [.claude/settings.json]\n' \
  > "$PROJ2/trackfw.yaml"

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "cwd":"%s"}' "$PROJ2" "$PROJ2" |   # sem agent_id → sessão principal
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ denied: ".../.claude/settings.json" is a frozen path (.claude/settings.json)   rc=2  ✓

Após plantar .claude/trackfw.yaml (vazio):
→ (vazio)   rc=0
```

**rc medidos:** baseline=2, step2=0. F-5 afeta a sessão principal quando `write_scope.frozen_paths`
contém caminhos com componentes de diretório. No regime sem yaml frozen_paths, a sessão principal
pode editar o `trackfw.yaml` da raiz diretamente sem passar pelo re-enraizamento. O impacto
prático do F-5 sobre a sessão principal é portanto **baixo**.

#### Sub-variante A — contra-braço worktree legítimo (medido)

```
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"wt-agent","cwd":"%s"}' "$MAIN" "$WT" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ denied: ".../.claude/settings.json" is a frozen path (.claude/settings.json)   rc=2  ✓
```

**rc medido:** 2. `findProjectRoot($MAIN/.claude/settings.json)` retorna `MAIN/` —
o yaml aninhado de `WT/` não interfere.

#### Sub-variante B: diretório nomeado `trackfw.yaml`

`findProjectRoot` usa `os.Lstat`, que tem sucesso para diretório.
`readWriteScopeFromYAML` chama `os.ReadFile` naquele diretório, que retorna `is a directory`.
O código trata **qualquer** erro de ReadFile como "ausente" (`return nil, nil, nil`):

```go
// writescope.go:615–628
data, readErr := os.ReadFile(filepath.Join(projectRoot, "trackfw.yaml"))
if readErr != nil {
    return nil, nil, nil  // IsNotExist e "is a directory" tratados igual — LACUNA
}
```

**Medição — Step 1 guarded (escrever arquivo dentro de `.claude/trackfw.yaml/`):**

```
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/trackfw.yaml/x"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**Medição — Step 2 (diretório `.claude/trackfw.yaml/` plantado diretamente na fixture; Write a `.claude/settings.json`):**

```
mkdir -p "$PROJ/.claude/trackfw.yaml"
# ls: drwxr-xr-x  trackfw.yaml  (diretório)

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**rc medidos (step 1 e step 2):** 0, 0. Step 2 confirma o bypass completo do D3 após o
diretório ser plantado — `ErrNotRegularFile` em `readWriteScopeFromYAML` é swallowed,
tornando a frozen list vazia.

#### Sub-variante C: variante de maiúsculas em APFS (macOS)

`os.Lstat("trackfw.yaml")` em APFS (case-insensitive) encontra `TRACKFW.YAML`.
`TRACKFW.YAML` é **arquivo regular** — `IsRegular()` retorna `true`.
Portanto uma verificação de `IsRegular()` em `findProjectRoot` **não fecha sub-variante C**.

**Medição:**

```
mkdir -p "$PROJ/.claude" && printf 'adr_dirs: []\n' > "$PROJ/.claude/TRACKFW.YAML"
# ls -la: .rw-r--r--@  TRACKFW.YAML  (arquivo regular)

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**rc medido:** 0. Escopo: macOS (APFS) e Windows (NTFS). Linux (ext4 sem casefold) não é afetado.

#### Sub-variante D: D3 bypass também sob lease restritiva (F-5D folded into F-5A)

**Medição — F-5D arm (lease=[.claude/], yaml plantado na fixture, cwd=$PROJ normal):**

```
H=$(mktemp -d)/home; mkdir -p "$H"; PROJ=$(mktemp -d)
mkdir -p "$PROJ/.claude" "$PROJ/.trackfw/leases"
printf 'adr_dirs: []\n' > "$PROJ/trackfw.yaml"
EXPIRY=$(date -u -v+1H '+%Y-%m-%dT%H:%M:%SZ')
printf '{"version":1,"agent_type":"evil","expires":"%s","allow":["%s/.claude/"]}' \
  "$EXPIRY" "$PROJ" > "$PROJ/.trackfw/leases/evil.json"

# Plant yaml in fixture directly (not via Write guard)
printf 'adr_dirs: []\n' > "$PROJ/.claude/trackfw.yaml"

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","agent_type":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H/.x" XDG_CONFIG_HOME="$H/.c" \
  "$TF" guard write-scope 2>&1
→ (vazio)   rc=0  ← D3 bypass; cwd normal ($PROJ) — cwd condition NOT needed for D3 bypass
```

**Medição — F-5D counter-arm (mesma lease, sem planta, cwd=$PROJ normal):**

```
rm "$PROJ/.claude/trackfw.yaml"

printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","agent_type":"evil","cwd":"%s"}' "$PROJ" "$PROJ" |
  env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H/.x" XDG_CONFIG_HOME="$H/.c" \
  "$TF" guard write-scope 2>&1
→ denied: ".../.claude/settings.json" is a frozen path (.claude/settings.json)
  rc=2  ✓ (D3 protege sem a planta)
```

**rc medidos:** arm=0, counter-arm=2.

**Consequência — F-5D é F-5A sob lease que permite a planta:** o D3 bypass ocorre com cwd
normal (`$PROJ`). A condição `cwd=.claude/` era só necessária para o D4 component (outer lease
não carregada). Mas como a lease já inclui `.claude/`, o D4 component não adiciona bypass
independente — a outer lease já permitia aquele diretório. F-5D via Write é portanto F-5A
com lease que cobre o subdiretório alvo, e tem severidade BLOQUEANTE como F-5A.

**Plant via Write: gating depende do escopo da lease:**

O plant de `P/trackfw.yaml` via Write só é possível quando L autoriza P/ (prefix, trailing `/`) **ou**
o arquivo exato `P/trackfw.yaml` (exact-file entry, sem trailing `/`). Quando nenhuma dessas
condições é satisfeita, a lease bloqueia o plant antes de Step 7 (rc=2). Quando P ⊂ L (prefix),
o plant passa (rc=0 pré-fix) e D3 bypass ocorre. Quando o entry é exact-file (medido abaixo),
o plant também passa (rc=0) mas o D4 escape segue caminho diferente (see exact-file block).

```
Lease=[src/]: P=.claude/ ∉ src/ →
  plant via Write a .claude/trackfw.yaml → rc=2  ✓ (bloqueado pela lease)

Lease=[.claude/]: P=.claude/ ⊂ .claude/ →
  plant via Write a .claude/trackfw.yaml → rc=0  (permitido; depois D3 bypass via F-5A)
```

**rc medidos:** plant(lease=[src/])=2, plant(lease=[.claude/])=0.

**Lease schema é allow-only** (`writescope.go:34`, `json:"allow"` — sem campo `Deny`).
Consequência: plant via Write só é possível quando lease autoriza P/ (prefix) ou o arquivo
exato `P/trackfw.yaml` (exact-file entry); depois, (a) bloqueia o plant, (d) fecha D3 se
plant ocorrer por outro meio.

**AND policy do lease lookup (ML-3C/ML-4C, `writescope.go:202-218`):** a lookup coleta leases de
DOIS roots — `projectRoot` (do alvo, Step 7) e `cwdProjectRoot` (do cwd, Step 11a). Cada lease
encontrada deve INDEPENDENTEMENTE autorizar o alvo. Uma lease permissiva no cwd-root não cancela
uma lease restritiva no target-root.

**D4 bypass via Write sob prefix matching (trailing `/`) — impossível para alvos não-frozen:**

Proof (restrito a entradas de prefix com `/` no final, `writescope.go:309-325`, `pathHasPrefix`):
para o bypass funcionar via Write, o yaml plantado precisa estar em P onde P ∈ allow list (para o
plant ser permitido) E P está no caminho ancestral do alvo T (para re-enraizar Step 7). O allow
entry é prefix (`allow=[P/]`): P/x ∈ allow list para qualquer x ⊂ P. Portanto T ∈ P/** implica
T ∈ allow list → T já está autorizado pela lease antes da planta. D4 bypass via Write é
estruturalmente impossível para alvos não-frozen sob **prefix matching** (trailing `/`).

**Exact-file lease (sem trailing `/`, `pathEqual`) — residual estreito, fechado por (a):**

Entradas exact-file (`allow=["P/trackfw.yaml"]`, sem `/`) usam `pathEqual` — só autorizam o
arquivo exato. O plant `P/trackfw.yaml` via Write está em allow (rc=0 pré-fix). Após a planta,
o guard re-enraíza Step 7 para P/. O lease lookup para a nova raiz P/ busca em
`P/.trackfw/leases/` — arquivo inexistente → sem lease → D3-only. Se o alvo T ∈ P/** não está
em D3 frozen (relativo a P/), T passa.

```
# Medição — exact-file lease `allow=["$PROJ/sub/trackfw.yaml"]` (sem trailing slash)
Pre (cwd=sub, Write sub/x.md, sem plant):                rc=2  (sub/x.md ∉ allow list)
Plant (Write sub/trackfw.yaml, exact-file match):        rc=0  ← plant permitido
Arm (plant presente, Write sub/x.md, cwd=sub):           rc=0  ← re-root→sub/; .trackfw/leases/ ∉ sub/
Counter-arm (cwd=PROJ, plant presente):                  rc=2  ← Step 11a cwd=PROJ → outer lease carregada
```

**rc medidos:** pre=2, plant=0, arm=0, counter-arm=2.

(a) fecha: Write a `sub/trackfw.yaml` → componente EqualFold → rc=2 pós-fix. (d) é D3-only e não
cobre este D4 escape via exact-file lease — a proteção aqui é estritamente (a).

**Restrição cwd:** o counter-arm acima usa `cwd=PROJ` — a Step 11a re-enraíza para PROJ e carrega
a outer lease. Para o arm funcionar (rc=0), o `cwd` do payload deve estar sob P/ (sub/).

**Windows + exact-file lease — ver R-8 e §4 para o residual declarado.**

Para alvos **frozen** dentro de P: o alvo T ∈ P/** é autorizado pela lease (D4), mas a frozen
check (D3, Step 10) bloqueia. A planta de P/trackfw.yaml re-enraíza Step 10 para falhar
(relativo a P, o caminho congelado vira P/frozen-path → `P/frozen-path` ≠ T). É F-5A/D3
bypass — a lease é secundária. Medido abaixo com lease=[docs/].

**D4 bypass via Bash — DUAS condições necessárias (medido):**

O Bash-plant escapa do guard (lease não gata a escrita). Para que o D4 bypass ocorra (outer lease
não carregada), o yaml plantado deve estar no **caminho ancestral** de AMBOS Step 7 (alvo) e Step
11a (cwd). Se qualquer uma das duas condições falhar, o root correspondente permanece PROJ e a
outer lease é carregada.

Evidência das três medições:

```
# Medição A: .claude/ plant + cwd=.claude/ + target=.claude/notes.md (não-frozen)
# lease=[src/]
Arm (cwd=.claude/, .claude/trackfw.yaml planted):
  rc=0  ← AMBAS as condições satisfeitas: Step 7 (.claude/notes.md → .claude/ root)
           E Step 11a (.claude/ cwd → .claude/ root); após dedup: 1 root=.claude/;
           .claude/.trackfw/leases/evil.json não existe → sem lease → rc=0

Counter-arm A (cwd=PROJ, .claude/trackfw.yaml planted):
  rc=2  ← Step 11a (cwd=PROJ) → PROJ root → outer lease carregada
           Prova: condição cwd-ancestral é NECESSÁRIA

# Medição B: src/ plant + cwd=src/ + target=docs/x.md (não-frozen)
# lease=[src/]
Arm (cwd=src/, src/trackfw.yaml planted):
  rc=2  ← Step 7 (docs/x.md → walk-up → PROJ; src/ NÃO é ancestral de docs/) → PROJ lease carregada
          Prova: condição target-ancestral é NECESSÁRIA
```

**rc medidos:** A(arm)=0, A(counter-arm)=2; B(arm)=2.

**Medição completa A (Bash plant, cwd=.claude/, lease=[src/]):**

```
# lease=[src/]; target=.claude/notes.md (não-frozen)
Pre-plant (cwd=.claude/, agent_type=evil, sem yaml em .claude/):
  → denied: ".../.claude/notes.md" is not in the allow list of lease .../evil.json for "evil"
  rc=2  ✓ (findProjectRoot(.claude/)→walk-up→PROJ→lease=[src/] carregada)

Bash plant: printf 'adr_dirs: []\n' > "$PROJ/.claude/trackfw.yaml"

Arm (cwd=.claude/, yaml planted, agent_type=evil):
  → (vazio)   rc=0  (AMBOS Step 7 e Step 11a re-enraizam para .claude/; .claude/.trackfw/leases/ não existe)

Counter-arm (cwd=PROJ, yaml planted):
  → denied: ".../.claude/notes.md" is not in the allow list of lease .../evil.json for "evil"
  rc=2  ✓ (Step 11a usa cwd=PROJ → outer lease PROJ encontrada)
```

Conclusão: D4 bypass via Bash exige yaml no **ancestral comum de alvo e cwd**. Via Bash = R-1.

**D3 bypass via Write com lease=[docs/] — alvo frozen dentro do dir autorizado (medido):**

```
# lease=[docs/]; target=docs/agents-working-context/x.md (FROZEN, linha 63 writescope.go)
Baseline (sem plant):
  → denied: "docs/agents-working-context/x.md" is a frozen path (docs/agents-working-context/)
  rc=2  ✓ (D3 freeze relativo a PROJ)

Step 1 plant via Write (docs/trackfw.yaml, lease=[docs/]):
  → (vazio)   rc=0  ← lease=[docs/] autoriza docs/trackfw.yaml; docs/trackfw.yaml NÃO está na
                      frozen list hardcoded (somente "trackfw.yaml" root-level, linha 64)

Arm (yaml planted, Write docs/agents-working-context/x.md):
  → (vazio)   rc=0  ← D3 bypass: target root=docs/ (docs/trackfw.yaml encontrado);
                      frozen list relativa a docs/: "docs/agents-working-context/" vira
                      "docs/docs/agents-working-context/" ≠ target → D3 miss;
                      D4: target ∈ docs/ → allow → rc=0
```

**rc medidos:** baseline=2, step1=0, arm=0.

Este é F-5A via Write: o alvo `docs/agents-working-context/x.md` já estava autorizado pela
lease (`docs/`), mas bloqueado pelo D3 freeze. A planta de `docs/trackfw.yaml` re-enraíza D3
para falhar. A lease (D4) é secundária — o bypass é F-5A. Fix: (a) bloqueia o plant antes de
Step 7; (d) fecha D3 monotonicamente.

#### FIFO nomeado `trackfw.yaml` (medido)

```
mkfifo "$PROJ/.claude/trackfw.yaml"
# Rodou em background com timeout de kill após 3s
→ HUNG: processo "$TF guard write-scope" ainda vivo após 3s
Processo terminado manualmente; rc nunca impresso.
```

`readWriteScopeFromYAML` chama `os.ReadFile` no FIFO. Sem writer, `ReadFile` bloqueia.
Com design (d) lendo cada ancestral, a superfície de hang cresce para todos os ancestrais.
Planta de FIFO requer Bash (`mkfifo`) — não alcançável via Write/Edit.

#### Frozen paths vulneráveis

| Frozen path | Planta possível | Sub-raiz criada |
|---|---|---|
| `.claude/settings.json` | `.claude/trackfw.yaml` | `.claude/` |
| `.claude/settings.local.json` | idem | idem |
| `.codex/hooks.json` | `.codex/trackfw.yaml` | `.codex/` |
| `.gemini/settings.json` | `.gemini/trackfw.yaml` | `.gemini/` |
| `.cursor/hooks.json` | `.cursor/trackfw.yaml` | `.cursor/` |
| `.windsurf/hooks.json` | `.windsurf/trackfw.yaml` | `.windsurf/` |
| `.github/hooks/` | `.github/trackfw.yaml` | `.github/` |
| `.trackfw/leases/` | `.trackfw/trackfw.yaml` | `.trackfw/` |
| `.opencode/plugins/` | `.opencode/trackfw.yaml` | `.opencode/` |
| `.agents/hooks.json` | `.agents/trackfw.yaml` | `.agents/` |
| `docs/agents-working-context/` | `docs/trackfw.yaml` | `docs/` |
| `trackfw.yaml` (na raiz) | não aplicável — já frozen (linha 64) | — |
| `opencode.json` / `opencode.jsonc` | não aplicável — arquivos na raiz | — |

### Ação 2 — Enumeração completa das superfícies (AC3)

**Limitação do grep por literal `"trackfw\.yaml"`:**

A aspa antes de `trackfw` cega para shell (`"$dir/trackfw.yaml"`) e JS (`"file_path": "..."` com
path montado por variável). Prova: `printf '[ -f "$d/trackfw.yaml" ]\n' | grep -c '"trackfw\.yaml"'`
retorna 0, enquanto `grep -c 'trackfw\.yaml'` retorna 1. Para cobrir non-Go, usar:

```
grep -rln 'trackfw\.yaml' internal/ scripts/ npm/ pypi/ 2>/dev/null | grep -v '\.go$'
```

**Varredura de non-Go por walk-up loops (executada):**

```
grep -rn 'while\|dirname\|Split-Path -Parent\|path\.dirname' \
  internal/generators/testdata/guard-sh-reference/ \
  scripts/trackfw-credential-guard.sh \
  scripts/windows-repro/guard-global-home.ps1 \
  internal/opencodeplugin/trackfw-guard.js 2>/dev/null | grep -v "^Binary"
```

Achados relevantes:
- `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh:88,110`: walk-up `while :; do` para `trackfw.yaml` a partir de `pwd -P` (cwd do processo). Mesmo padrão de `gitbranch.go:FindProjectRoot`. **Este é o arquivo de referência legado (testdata).** A versão em produção é `gitBranchGuardScript` em `internal/generators/scaffold.go:1646` — um shim de uma linha que apenas chama `trackfw guard git-branch`. O walk-up está implementado em Go em `gitbranch.go:FindProjectRoot`; a cópia de testdata está superseded. Classificado abaixo.
- `scripts/check-gates-falsify.sh:999` (linha ~999, bloco `for roadmap in ...`): usa `dirname` apenas para extrair o diretório pai de um path fixo da forma `docs/req/<REQ>.md` — extração de path, não walk-up para `trackfw.yaml`. Sem derivação de política.
- `internal/opencodeplugin/trackfw-guard.js:111`: `realpathOrParent(nodePath.dirname(p))` — para resolver symlinks de targets e do `__filename` do plugin, NÃO para walk-up de `trackfw.yaml`. `projectDir` vem do framework OpenCode, não de walk-up.
- `credential-guard-global.sh`, `credential-guard-project.sh`: apenas `while IFS= read -r line` — loops de leitura de linha, sem walk-up de diretório.
- `scripts/windows-repro/`, `scripts/check-*.sh` (exceto `check-gates-falsify.sh:999` classificado acima): `dirname` para resolver `${BASH_SOURCE[0]}` (path fixo), não walk-up.

**Nenhum arquivo non-Go** faz walk-up de `trackfw.yaml` para derivar política (frozen lists, allow lists). O único walk-up non-Go de `trackfw.yaml` é o `git-branch-guard.sh` de testdata (superseded), classificado a seguir.

**Grep completo Go:**

```
grep -rn '"trackfw\.yaml"\|findProjectRoot\|FindProjectRoot' internal/ scripts/ \
  --include="*.go" --include="*.sh" --include="*.ps1" | grep -v "_test.go"
```

**Probe de walk-up loops (idioma `parent == dir` — encontra ambos os sítios conhecidos e todos os demais):**

```
grep -rn 'parent == dir\|dir == parent\|filepath\.Dir(dir)' internal/ --include="*.go" \
  | grep -v "_test.go"
→ internal/validator/validator.go:3614   (resolvePhysical — walk por path, não por trackfw.yaml)
→ internal/validator/validator.go:3615
→ internal/winskips/winskips.go:100      (ModuleRoot — walk por go.mod, não por trackfw.yaml)
→ internal/winskips/winskips.go:101
→ internal/commands/audit_surface.go:114 (findGitRootFromPath — walk por .git)
→ internal/commands/audit_surface.go:115
→ internal/guard/gitbranch.go:230        (FindProjectRoot — walk por trackfw.yaml; root descartado)
→ internal/guard/gitbranch.go:231
→ internal/guard/writescope.go:520       (findProjectRoot — SUPERFÍCIE F-5)
→ internal/guard/writescope.go:521
```

Cinco walk-up loops em `internal/`. Classificados por relevância para F-5:

**`writescope.go:520` — SUPERFÍCIE F-5 (★ único sítio afetado):**
Dentro de `findProjectRoot(p)`, chamada em dois pontos:
- Linha 165: `findProjectRoot(resolved)` — alvo do payload. Determina `projectRoot` para frozen lists. Ponto principal do F-5.
- Linha 226: `findProjectRoot(cwdResolved)` — campo `cwd` do payload (session cwd, fornecido pelo harness). Determina `cwdProjectRoot` para lease lookup. F-5D condicional (ver acima).

**`gitbranch.go:224-237`, `git-branch-guard.sh:110` (testdata/referência — superseded) e shim de produção em `scaffold.go:1646` — walk-up para `trackfw.yaml`, root descartado, direção só pode ativar o guard:**

```go
// gitbranch.go:224
func FindProjectRoot(cwd string) (string, bool) {
    dir := cwd
    for {
        if _, err := os.Stat(filepath.Join(dir, "trackfw.yaml")); err == nil {
            return dir, true
        }
        parent := filepath.Dir(dir)
        if parent == dir { return "", false }
        dir = parent
    }
}
// chamada em gitbranch.go:122:
_, found := FindProjectRoot(cwd)   // root descartado — só "found" importa
```

`git-branch-guard.sh:110`:
```sh
_TRACKFW_ROOT_DIR=$(pwd -P)
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then _TRACKFW_FOUND=1; break; fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then break; fi
  _TRACKFW_ROOT_DIR="${_TRACKFW_ROOT_DIR%/*}"
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0   # root não é usado para frozen lists
```

O shim de produção (`scaffold.go:1646`) chama `trackfw guard git-branch` sem walk-up próprio;
o walk-up está inteiramente em `gitbranch.go:FindProjectRoot`. A cópia de testdata
(`git-branch-guard.sh:110`) é referência legada e está superseded pela implementação Go.

Ambos usam **cwd do processo guard** (não campo `cwd` do payload). Argumento de direção:
plantar um yaml aninhado só pode flipped `found` de `false` para `true` (guard ativa),
nunca de `true` para `false` (guard desativa). O ataque F-5 requer DESLIGAR proteções; a
direção de impacto de um yaml novo é o oposto. `os.Stat` em FIFO não bloqueia. Sem frozen
lists, sem lease lookup. Não é superfície para F-5.

**`validator.go:3614` (`resolvePhysical`) — walk-up de path, não de trackfw.yaml:**
Sobe para encontrar o ancestral mais profundo que existe (EvalSymlinks). Não procura
`trackfw.yaml`. Não é superfície para F-5.

**`winskips.go:100` (`ModuleRoot`) — walk-up para `go.mod`:**
Sobe para encontrar `go.mod`. Não procura `trackfw.yaml`. Não é superfície para F-5.

**`audit_surface.go:114` (`findGitRootFromPath`) — walk-up para `.git`:**
Sobe para encontrar `.git`. Não procura `trackfw.yaml`. Não é superfície para F-5.

**Saída completa classificada (demais sítios de leitura de `trackfw.yaml`):**

```
1 NÍVEL A PARTIR DO CWD DO PROCESSO (não walk-up; segundo sítio de downgrade via plant):
  internal/guard/credential.go:233:  os.Stat(filepath.Join(projectCwd,"trackfw.yaml"))
  internal/guard/credential.go:334/448: credReadMode/credReadRoadmapDir

  Argumento de direção — OS DOIS ESCOPOS DIFEREM quando não há yaml em cwd:

  PROJECT-SCOPE (`RunCredential`, linha 213): early-exit em linha 233 `os.Stat → err != nil →
  return 0` — "not inside a trackfw project". Yaml ausente = guard desativado. Planta COM
  `mode: warn` = guard ativado em warn (aumento de proteção). Planta COM `mode: block` = aumento.
  Sem downgrade possível a partir de "ausente" (escopo de projeto). Via Write: (a) fecha pós-fix. Via Bash = R-1 (sobrescrita do yaml raiz; sem piso no escopo de projeto).

  GLOBAL-SCOPE (`RunCredentialGlobal`, linha 361): função SEPARADA, invocada pelo flag `--global`.
  SEM early-exit os.Stat (linha 365: "No trackfw.yaml guard"). Chama diretamente
  `credReadModeWithDefault(filepath.Join(cwd,"trackfw.yaml"), "block")` em linha 448. Yaml ausente
  → ReadFile falha → retorna "block" (defaultMode global). Guard bloqueia mesmo sem yaml.
  Planta COM `mode: warn` → **downgrade** de "block" para "warn". Planta COM `mode: block` = sem
  mudança. Via Write em sessão de projeto: (a) fecha. Via Write fora de projeto: R-9. Via Bash:
  pós-fix read-side floor, Bash-planted yaml com `mode: warn` recebe "block" (credential guard IS
  o Bash hook) — R-1 de write-scope não se aplica a este vetor pós-fix.

  Medição global-scope (flag `--global`, cwd=sub sem yaml raiz em cwd):
  ```
  cwd=sub, sem yaml:                        rc=2  ✓ (blocked — defaultMode "block")
  plant yaml com credential_guard.mode=warn: rc=0  ← downgrade block→warn confirmado
  ```
  rc medidos: pre=2, arm=0. Precondição cwd: o yaml é lido de `filepath.Join(cwd,"trackfw.yaml")`;
  se cwd aponta para o projeto raiz onde um yaml legítimo já existe, o plant sobrescreve — Write
  a `cwd/trackfw.yaml` já é bloqueado pelo freeze root-level (linha 64) e por (a) pós-fix.

  Windows gap — credential guard (`--global` scope): (a) no write-scope testa o componente raw;
  EqualFold(`trackfw.yaml.`, `trackfw.yaml`) = false → Write `cwd/trackfw.yaml.` não bloqueado
  por (a). O Win32 filesystem descarta o trailing dot **na criação do arquivo** (não no read);
  o arquivo resulta em `cwd/trackfw.yaml` em disco; `credReadModeWithDefault` lê `cwd/trackfw.yaml`
  → downgrade block→warn. (d) é D3-only — sem equivalente em credential guard. Por leitura, não
  medido (plataforma ausente). Ver R-8 estendido.

  **Veredicto AC3:** o mecanismo de downgrade via plant de `trackfw.yaml` existe em
  `RunCredentialGlobal` — mesma causa, mesma REQ per Regra Dura.

  Cobertura de (a) em write-scope: parcial. Credential global hook usa matcher `Bash|PowerShell`
  (update.go:1119, `claudeShellMatcher = "Bash|PowerShell"` agentfiles.go:534) — Write tool nunca
  chega a `RunCredentialGlobal`. (a) em write-scope bloqueia o Write apenas quando o hook
  write-scope está ativo (sessão em projeto). Write-scope é projeto-level (agentfiles.go:266-278,
  escreve em `.claude/settings.json`; nenhum wiring global em update.go — por leitura). Fora de
  projeto: Write a `cwd/trackfw.yaml` não interceptado por write-scope. (a) seria rc=2 se
  invocado (pre-fix rc=0 medido em $NOPROJ — evidência de comportamento do binário chamado
  diretamente, não de invocação via hook).

  **Fix deve ser no LADO DA LEITURA de `credential.go`, no sítio de chamada L448.**
  `credReadModeWithDefault` tem dois callers: L334 (`RunCredential`, default="warn") e L448
  (`RunCredentialGlobal`, default="block"). O piso de "block" deve ser aplicado SOMENTE no
  sítio L448, ou equivalentemente como `max(defaultMode, yaml_mode)` dentro da função (nunca o
  literal `"block"` dentro da função — L334 também a chama). Enum tem dois valores: "warn" e
  "block". Pós-fix no sítio L448: yaml mode=warn → "block" (piso); yaml ausente → "block"
  (default inalterado). Em prática: o cwd yaml é ignorado para `--global` scope pós-fix.
  Fecha: Write (em e fora de projeto); **Bash-planted yaml** (credential guard IS o Bash hook
  — Bash-planted yaml com `mode: warn` recebe "block" na próxima chamada; R-1 não se aplica
  a este vetor pós-fix); Windows trailing-dot. **FP declarado:** `credential_guard: mode: warn`
  no `trackfw.yaml` do próprio projeto, lido pelo hook global via cwd, perde efeito (opt-in
  explícito — `init` não scaffolda este campo). Requer decisão no ADR. **Recomendação Wave 1
  ML desta REQ** (Regra Dura — mesma causa): piso no sítio L448 de `RunCredentialGlobal`.
  Counter-arm: `RunCredential` (projeto, L334) + yaml `mode: warn` deve continuar rc=0 pós-fix
  (ver §3 FP row). Risco de implementação: aplicar `max("block", ...)` dentro da função compartilhada
  eleva project scope para block — test obrigatório.

CWD FIXO — sem walk-up:
  internal/commands/serve.go:  config.Load() (cwd fixo via serve.Start())
  internal/commands/barrier.go:133,1224 → config.Load()  (cwd fixo)
  internal/commands/commit.go:106 → config.Load()  (cwd fixo)
  internal/commands/push.go:101 → config.Load()  (cwd fixo)
  internal/commands/ship.go:122 → config.Load()  (cwd fixo)
  internal/config/config.go:236/278/379/389: os.ReadFile([cwd+]"trackfw.yaml")
  internal/config/config.go:210: LoadGlobalAgentModels → ~/.trackfw/trackfw.yaml (path fixo global)
  internal/commands/attention.go:66: pathguard.ResolveRoot(cwd) + filepath.Join(root,"trackfw.yaml")
    (pathguard.ResolveRoot = filepath.Abs + filepath.EvalSymlinks, sem walk;
     declaração: internal/pathguard/pathguard.go:316–328)
  internal/validator/validator.go:920/1514: readRegularFile("trackfw.yaml") ← cwd fixo
  internal/validator/validator_credential_guard_integrity.go:136: readRegularFile("trackfw.yaml")
  internal/commands/configure.go:33/157/162/167: cwd ou configureRoot fixo
  internal/commands/init.go:235: os.Lstat(filepath.Join(cwd,"trackfw.yaml"))
  internal/generators/update.go:49/330/332/3366/3686: cwd ou guardRoot fixo
  internal/generators/scaffold.go:106: const trackfwYAMLPath = "trackfw.yaml" (cwd fixo)
  internal/generators/scaffold_doctor.go:279: filepath.Join(projectRoot,"trackfw.yaml") ← cwd fixo
  internal/commands/integrations_flags.go:334: filepath.Join(manager.ProjectRoot,"trackfw.yaml")
  internal/commands/discover.go:138: filepath.Join(resolvedCwd,"trackfw.yaml")

ROOT DIR INJETADO (não walk-up do alvo):
  internal/discover/discover.go:433/780: filepath.Join(rootDir,"trackfw.yaml")

FROZEN ENTRY:
  internal/guard/writescope.go:64:  "trackfw.yaml"  ← freeze root-level apenas
  internal/commands/branch_prune.go:241: "trackfw.yaml": true ← lista de ignore do prune
```

**OpenCode plugin:**

`projectDir` vem de `ctx.location.project.directory` (V2) ou `input.worktree || input.directory`
(V1) — fornecido pelo framework OpenCode, não derivado de walk-up baseado no path-alvo.

Medição com payload real:

```
printf '{"hook_source":"opencode","tool_name":"write","tool_input":{"file_path":"%s/.claude/settings.json"},
  "agent_id":"evil","cwd":"%s"}' "$PROJ" "$PROJ" | env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**rc medido:** 0. `writeScopeWriteTools = {"Write","Edit","NotebookEdit"}` — Step 4 retorna allow
para `"write"` lowercase antes de Step 7. F-5 não é alcançável via OpenCode por este canal.

**Gap de lowercase:** ADR-2026-11-19 cobre o mecanismo de integração do plugin, mas **não declara
o gap de lowercase como residual**. Gap medido separadamente (ML-5-0, 2026-11-19). Verificar
issues/REQs abertas antes de abrir novo ADR.

**Antigravity:** o guard write-scope processa payloads de Antigravity pelo mesmo caminho de Claude
(Step 3-7). `.agents/hooks.json` já está em `writeScopeSubagentFrozenRelPaths` (linha 74). O campo
`cwd` do payload de Antigravity é usado somente em Step 11a (D4 leases), não em Step 7.

**Censo de yamls aninhados no repositório real:**

```
find . -name "trackfw.yaml" -not -path "./trackfw.yaml" -not -path "./.git/*"
→ (vazio — zero yamls aninhados pré-existentes)
```

**Lista fechada:** F-5 é específico de `writescope.go:findProjectRoot` (linha 165 e 226 — a
mesma função, chamada em dois pontos). Segundo sítio de downgrade via plant: `credential.go`
escopo global (`RunCredentialGlobal`, linha 361) — planta com `mode: warn` downgrades de "block"
para "warn"; fix principal: piso no sítio L448 de `RunCredentialGlobal` (ou `max(defaultMode, yaml_mode)` na função — não o literal "block"; L334 tem default=warn) — fecha qualquer canal (Write, Bash); (a) em write-scope é gate
secundário para Write em sessões dentro de projetos trackfw (write-scope é projeto-level — por
leitura do sítio de instalação, agentfiles.go:266-278; nenhum wiring global em update.go);
residual Windows (trailing-dot) em R-8; residual R-10 (Write a `~/.claude/settings.json`). `gitbranch.go:FindProjectRoot` e
`git-branch-guard.sh:110` têm a mesma estrutura de walk-up mas: (a) usam cwd do processo (não
target path), (b) descartam/não usam o root para frozen lists, (c) a direção de impacto de um
yaml novo é só "guard ativa" — nunca "guard desativa". Os demais três walk-ups Go
(`validator`, `winskips`, `audit_surface`) procuram outros artefatos, não `trackfw.yaml`. Nenhum
arquivo non-Go faz walk-up de `trackfw.yaml` para derivar política. `serve`, `barrier`,
`commit`, `push`, `ship` usam `config.Load()` com cwd fixo — não afetados.

---

## 2. Modelo de Ameaça

> *Quem esvazia esta Wave 0 sem quebrar nenhuma regra escrita, e como — o adversário aqui é o implementador apressado e o arquiteto otimista, não um atacante externo.*

**O adversário desta Wave 0 tem seis formas:**

**Forma I — implementador fecha apenas a sub-variante A via freeze de caminho literal:**  
Adiciona `".claude/trackfw.yaml"` à frozen list. Sub-variante B (`.../x` tem basename `x`),
sub-variante C (`TRACKFW.YAML` casa com Lstat case-insensitive) e outros subdiretórios sobrevivem.

**Forma II — implementador adiciona `IsRegular()` a `findProjectRoot` como única correção:**  
`IsRegular()` fecha sub-variante B (diretório). Mas `TRACKFW.YAML` é **arquivo regular** —
`IsRegular()` retorna `true`. Sub-variante C sobrevive.

Além disso: `IsRegular()` retorna `false` para symlink (Lstat retorna ModeSymlink sem
`O_NOFOLLOW`). Se a correção for "skip" (walk continua), o projeto legítimo cujo `trackfw.yaml`
é um symlink para arquivo regular **perde toda proteção** — medido hoje: rc=2 com symlink para
arquivo regular → se skip, não encontra mais a raiz → allow. Se a correção for "deny" para
symlink, o projeto legítimo também é quebrado.

A primitiva correta é `fileutil.ReadRegularFile` (já existe em `internal/fileutil/fileutil.go`)
que: abre com O_NONBLOCK (FIFO sem writer → `ErrNotRegularFile` sem bloquear), fstata o fd
(não o path — imune a TOCTOU), segue symlinks para o alvo (`O_RDONLY` sem `O_NOFOLLOW`),
distingue `ErrNotRegularFile` de `os.IsNotExist`. Desta forma:
- Arquivo regular → aceito ✓
- Symlink para arquivo regular → seguido → fstat regular → aceito ✓ (preserva comportamento)
- Diretório → `ErrNotRegularFile` → deny ✓
- FIFO → O_NONBLOCK → imediato → fstat FIFO → `ErrNotRegularFile` → deny ✓ (sem hang)
- Ausente → ENOENT/`os.IsNotExist` → walk continua ✓

**Forma III — design monotônico (d) implementado como union de yaml frozen_paths apenas:**  
Implementação ingênua: une `yaml.write_scope.frozen_paths` de cada ancestral, mas aplica
`writeScopeSubagentFrozenRelPaths` somente à raiz mais próxima.

Evidência concreta: o baseline de F-5 usa um outer yaml com `adr_dirs: [docs/adr]` e
**sem `write_scope.frozen_paths`** e obtém rc=2. A proteção vem exclusivamente de
`writeScopeSubagentFrozenRelPaths` hardcoded, não do yaml declarado. Uma implementação que
une apenas `yaml.frozen_paths` declarados e aplica os hardcoded só à raiz mais próxima
produziria rc=0 pós-fix (outer = vazio + `.claude/` = vazio → frozen list efetiva = vazia).

**Forma IV — fechamento marcado ✅ enquanto Bash torna o fix irrelevante para 12/12 agentes:**  
O ML passa nos critérios de aceite mas todos os 12 agentes do catálogo têm Bash (medido).

**Forma V — `ErrNotRegularFile` virado em "ausente" pelo drop-in:**  
Um implementador substitui `os.ReadFile` por `fileutil.ReadRegularFile` mas mantém
`if readErr != nil { return nil, nil, nil }`. `ErrNotRegularFile` torna-se equivalente a
"ausente" — e então um **diretório** nomeado `trackfw.yaml` continua sendo aceito por
`findProjectRoot` (Lstat ok) e depois produz `ReadRegularFile err → nil` em
`readWriteScopeFromYAML`. A frozen list fica vazia. F-5B sobrevive ao fix.

A correção correta: `if os.IsNotExist(readErr) { return nil, nil, nil }` / por qualquer outro
erro: `return nil, nil, readErr`. Apenas `os.IsNotExist` é "ausente"; `ErrNotRegularFile`,
`EACCES` e outros erros negam.

**Forma VI — (a) implementado no componente do path antes de normalização (Windows):**  
`normalizeTargetPath` (`writescope.go:717`) converte apenas `/c/` → `C:\` e `C:/` → `C:\`.
Não strip trailing `.` ou espaços. `filepath.Clean` também não remove trailing dot em
componentes. Portanto `trackfw.yaml.` e `trackfw.yaml ` não casam com `EqualFold` na string
raw, mas no Windows ambos criam o arquivo `trackfw.yaml`. (d) é o backstop D3 para Windows
até que a normalização de componentes seja implementada.

### Ação 3 — Desenhos candidatos

#### Desenho (a): freeze de qualquer componente de caminho `trackfw.yaml` para subagentes, ANTES do Step 7

**Mecanismo:** no início de `RunWriteScope`, **antes de Step 7**, verificar se qualquer
componente do caminho alvo é `trackfw.yaml` (EqualFold **incondicional** — GOOS é proxy errado:
APFS pode ser case-sensitive configurado; ext4 casefold e volumes exFAT/NTFS montados em Linux
criam case-insensitivity; o falso-positivo de EqualFold num filesystem case-sensitive é custo
praticamente zero). Aplicado sobre o path APÓS `filepath.Clean` e `normalizeTargetPath`.
`agent_id` já foi parseado antes de Step 7 (Step 1-3); (a) deve ser condicionado a
`payloadHasAgentID(root)`.

Deve ocorrer antes do Step 7 porque: frozenHit é chamado apenas quando Step 7 encontra uma
raiz. Se Step 7 retornar not-found (alvo fora de qualquer projeto) → allow imediato, sem
chamar frozenHit.

**Medição — write `trackfw.yaml` em diretório sem projeto (pré-fix):**

```
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/trackfw.yaml"},"agent_id":"evil","cwd":"%s"}' \
  "$NOPROJECT" "$NOPROJECT" | env HOME="$H" ... "$TF" guard write-scope 2>&1
→ (vazio)   rc=0
```

**rc medido:** 0. Pós-fix com (a) antes do Step 7: deve retornar rc=2.

**Sub-variante A:** `.claude/trackfw.yaml` — componente final é `trackfw.yaml` → nega. ✓  
**Sub-variante B:** `.../trackfw.yaml/x` — componente penúltimo é `trackfw.yaml` → nega. ✓  
**Sub-variante C:** `.../TRACKFW.YAML` — `EqualFold` incondicional casa → nega. ✓  
**Root-level dentro de projeto:** já coberto pelo frozen list hardcoded (medido rc=2).

**Windows bypass (Forma VI — R-8):** `trackfw.yaml.`, trailing space, `::$DATA` não casam com
EqualFold na string raw após `normalizeTargetPath`. (d) é o backstop D3.

**FP direction de (a):** nenhum subagente pode criar `trackfw.yaml` em nenhum diretório.
AC2 deve declarar no ADR que subagentes não criam yamls aninhados e que `trackfw init` em
subdiretório requer sessão principal.

#### Desenho (b1/b2/c): rejeitados — análise por leitura, não medido no binário

(b1) âncora no yaml mais externo: quebra monorepo. (b2) âncora no cwd: reverte ADR-2026-10-23.
(c) requer `git cat-file` no guard: adiciona superfície de processo externo. Todos rejeitados.

#### Desenho (d): união de buildFrozenList completo de todos os ancestrais

**Mecanismo:** em `buildFrozenList`, após determinar a raiz mais próxima, continuar o walk-up
e chamar `buildFrozenList(ancestorRoot)` **completo** — incluindo `writeScopeSubagentFrozenRelPaths`
— para cada ancestral encontrado. Cada entrada é resolvida **contra `ancestorRoot`** (não contra
a raiz mais próxima). Nega se qualquer ancestral congelar o caminho.

**Propriedade-chave — monotonicidade D3:** plantar um novo `trackfw.yaml` em um subdiretório
só pode adicionar à lista total (os ancestrais externos permanecem na cadeia).

**Bypass neutralizado — análise por leitura:**

```
Alvo: proj/.claude/settings.json
Raízes: .claude/ (plantada) e proj/ (externa)
buildFrozenList(.claude/): hardcoded relativo a .claude/ → proj/.claude/.claude/settings.json ≠ alvo
buildFrozenList(proj/):    hardcoded relativo a proj/   → proj/.claude/settings.json = alvo → rc=2 ✓
```

**Monorepo legítimo com yamls pré-existentes (medido no binário atual):**

```
printf '...svc/.claude/settings.json...svc-agent...' | env HOME="$H" ... "$TF" guard write-scope
→ rc=2  ✓

printf '...mono/.claude/settings.json...svc-agent...' | env HOME="$H" ... "$TF" guard write-scope
→ rc=2  ✓
```

**rc medidos:** 2, 2.

**Crítico — Forma III:** design (d) deve chamar `buildFrozenList(ancestorRoot)` completo.
O baseline rc=2 vem exclusivamente dos hardcoded, não do yaml declarado.

**Isolamento de testes — camada (a) vs (d):** (a) bloqueia a Write do yaml antes de Step 7.
Os testes de (d) **devem plantar o yaml diretamente na fixture** (não via Write guard) para
exercer (d) independentemente de (a). Sem isso, a Forma III passa em CI.

**FP direction de (d) — yaml inválido ou não-regular em ancestral acima do projeto:**
Se um yaml inválido ou não-regular existe em qualquer diretório ancestral ao projeto (incluindo
diretórios fora de qualquer projeto, como `~/projects/trackfw.yaml`), (d) com fix(e) retorna
deny para toda escrita abaixo dele. A política de boundary — até onde a walk sobe — deve ser
declarada no ADR como decisão explícita.

#### Fix (e): `fileutil.ReadRegularFile` em cada leitura de yaml

**Mecanismo:** substituir `os.ReadFile` por `fileutil.ReadRegularFile` em `readWriteScopeFromYAML`
e em cada passo de leitura da walk de (d). Em `findProjectRoot`, usar equivalente para distinguir:
- `ErrNotRegularFile` → **deny** (rc=2), não "walk continua"
- `os.IsNotExist` → walk continua (ausente)
- Sem erro → raiz encontrada

E em `readWriteScopeFromYAML`, trocar:
```go
if readErr != nil { return nil, nil, nil }  // atual — LACUNA (Forma V)
```
por:
```go
if os.IsNotExist(readErr) { return nil, nil, nil }   // ausente → ok
return nil, nil, readErr                             // ErrNotRegularFile, EACCES → deny
```

**Política de symlinks (declarada):** `fileutil.ReadRegularFile` não usa `O_NOFOLLOW`.
Symlink para arquivo regular: seguido → fstat regular → aceito. Comportamento atual
preservado (medido hoje: symlink `trackfw.yaml → real.yaml` → rc=2 pré-fix = correto).

#### Recomendação: (a) + (d) + fix (e)

Três layers independentes:
- **(a):** freeze de componente EqualFold incondicional, antes de Step 7. Fecha Step 1 de F-5A/B/C.
  Fecha o prerequisite de F-5D via Write. Não suficiente sozinho (Forma VI no Windows).
- **(d):** `buildFrozenList` completo por ancestral, D3-only. Fecha Step 2 de F-5A/B/C se plant
  ocorreu por outro meio (Bash residual R-1). Backstop D3 para R-8 (write-scope). Propriedade de
  monotonicidade D3. **Não** cobre o D4 exact-file escape (item acima) — esse é estritamente (a).
- **Fix (e):** `fileutil.ReadRegularFile`. Fecha F-5B (dir) e FIFO. Preserva symlinks.

F-5D (D3 component) é F-5A: o D3 bypass ocorre com cwd normal; o cwd=.claude/ só era necessário
para o D4 component. Via Write sob lease=[.claude/]: (a) bloqueia o plant antes de Step 7; (d)
fecha D3 se plant ocorreu por outro meio. F-5D D4 via Bash = R-1 (já residual).

### Texto para ADR-2026-10-23 (Adendo Wave 0)

> **Adendo Wave 0 (REQ-2026-11-23) — D3, proteção contra re-enraizamento.**
>
> **Efeito não declarado em D3:** `findProjectRoot` retorna o ancestral mais próximo; qualquer
> `trackfw.yaml` plantado num subdiretório pelo subagente re-enraíza o guard e anula as frozen
> lists da raiz original — incluindo as hardcoded de `writeScopeSubagentFrozenRelPaths`.
> Medido: baseline=2, step1(plant via Write)=0, step2(re-enraizado)=0.
>
> **F-5D via Write:** a lease é allow-only (`writescope.go:34`, campo `json:"allow"` apenas).
> Plant via Write a `P/trackfw.yaml` só é possível quando L autoriza P/ (prefix, trailing `/`)
> **ou** o arquivo exato `P/trackfw.yaml` (exact-file entry, sem trailing `/`).
> Quando P ∉ L, a lease bloqueia o plant (medido: lease=[src/], plant .claude/trackfw.yaml → rc=2).
> Quando P ⊂ L, plant via Write → rc=0 pré-fix e D3 bypass para frozen paths dentro de P — este é
> F-5A sob lease que autoriza o subdir da planta (medido: lease=[.claude/], arm=0).
>
> **Exact-file lease (sem trailing `/`):** `allow=["P/trackfw.yaml"]` autoriza só o yaml plant;
> após a planta, o lease lookup re-enraíza para P/ onde `.trackfw/leases/` não existe →
> sem lease → D3-only → allow para alvos não-frozen em P/ (medido: pre=2, plant=0, arm=0,
> counter-arm=2). (a) fecha: `P/trackfw.yaml` → componente EqualFold → rc=2 pós-fix. (d) não cobre
> este D4 exact-file escape (d é D3-only). F-5D via Bash = R-1 (já residual).
>
> **Fix três partes:**
>
> (a) **Freeze de componente, antes de Step 7:** para subagentes, qualquer Write em path com
> componente `trackfw.yaml` (EqualFold **incondicional** — GOOS é proxy errado; APFS pode ser
> case-sensitive configurado; ext4 casefold e volumes exFAT/NTFS montados em Linux criam
> case-insensitivity; FP de EqualFold num FS case-sensitive é custo praticamente zero) é negado
> antes de Step 7. Implica: subagentes não podem criar nem sobrescrever arquivos com componente
> de caminho `trackfw.yaml`; `trackfw init` em subdiretório requer sessão principal. No Windows,
> `trackfw.yaml.` e `trackfw.yaml ` não casam com EqualFold na string raw — (d) é o backstop
> D3 (R-8) até que normalização de componente Windows seja adicionada.
>
> (d) **União monotônica completa (D3):** `buildFrozenList` coleta a frozen list **completa**
> — incluindo `writeScopeSubagentFrozenRelPaths` — de cada `trackfw.yaml` encontrado ao
> caminhar para cima. Cada entrada é resolvida contra **sua própria raiz**. Monotonicidade D3:
> plantar um novo yaml apenas adiciona à proteção total. Implementar como
> `buildFrozenList(ancestorRoot)` completo — nunca como union de apenas
> `yaml.write_scope.frozen_paths`. Testes de (d) devem plantar o yaml diretamente na fixture
> (não via Write guard) para exercer (d) independentemente de (a).
> **FP declarado:** yaml inválido/não-regular em ancestral acima do projeto causa deny-all;
> política de boundary é decisão de impl.
>
> (e) **`fileutil.ReadRegularFile` em toda leitura de yaml:** substituir `os.ReadFile` por
> `fileutil.ReadRegularFile` em `readWriteScopeFromYAML` e em cada passo da walk de (d).
> Em `findProjectRoot`, `ErrNotRegularFile` → deny (rc=2); `os.IsNotExist` → walk continua.
> Em `readWriteScopeFromYAML`: `if os.IsNotExist(readErr) { return nil, nil, nil }`;
> caso contrário `return nil, nil, readErr`. Nunca `if err != nil { return nil, nil, nil }`.
> Symlinks para arquivo regular continuam funcionando (O_RDONLY sem O_NOFOLLOW). FIFOs → deny.
>
> **Credential guard (`RunCredentialGlobal`):** mesmo mecanismo — planta `cwd/trackfw.yaml`
> com `credential_guard: {mode: warn}` → downgrade de "block" para "warn" (pre=2, arm=0 medido,
> sessão em projeto, cwd=sub sem yaml raiz). Credential guard usa matcher `Bash|PowerShell`
> (update.go:1119) — Write nunca chega a `RunCredentialGlobal`. Fix deve ser no **lado da
> leitura**, no sítio L448 (ou `max(defaultMode, yaml_mode)` na função — nunca o literal
> `"block"` na função; L334 tem default="warn"). Pós-fix: `mode: warn` → "block"; yaml ausente
> → "block" (inalterado). Fecha: Write (em e fora de projeto); Bash-planted yaml (Bash IS o
> hook; R-1 de write-scope não se aplica a este vetor); Windows trailing-dot. FP: `credential_guard:
> mode: warn` no `trackfw.yaml` do próprio projeto, lido pelo hook global via cwd, perde efeito
> (opt-in explícito; `init` não scaffolda) — decisão para o ADR. Counter-arm obrigatório:
> `RunCredential` (projeto, L334) + yaml `mode: warn` deve continuar rc=0. `roadmap_dir` não é
> lido do yaml em `RunCredentialGlobal` (hardcoded "docs/roadmaps", L461) — `mode` único campo.
> **Recomendação Wave 1 ML desta REQ** (mesma causa, Regra Dura).
>
> **Residual mantido:** escrita por Bash escapa de D3 e D4. 12/12 agentes do catálogo
> têm Bash. R-1 permanece do ADR original.

---

## 3. Alvos de Falsificação em Ambas as Direções

> *Para cada superfície: onde a sabotagem entra, qual gate deveria pegar, em qual direção — falso negativo e falso positivo.*

| Superfície / Cenário | Direção | Comportamento atual | Gate que deve pegar | Medido |
|---|---|---|---|---|
| Sub-variante A: arquivo `.claude/trackfw.yaml` | FN | rc=0 (Step 1 e Step 2) | (a) antes Step 7; (d) Step 2 | rc=0, 0 ✓ |
| Sub-variante B: `.../trackfw.yaml/x` via Write | FN | rc=0 | (a) componente; fix(e) ErrNotRegularFile | rc=0 ✓ |
| Sub-variante B: diretório planted na fixture → settings.json | FN | rc=0 | (d) frozen de ancestral; fix(e) ErrNotRegularFile | rc=0 ✓ |
| Sub-variante C: `TRACKFW.YAML` regular em APFS | FN | rc=0 | (a) EqualFold incondicional; (d) outer root | rc=0 ✓ |
| F-5A sob lease=[.claude/], cwd normal, fixture plant | FN | rc=0 medido (D3 bypass independe de cwd) | (a) fecha plant; (d) fecha D3 pós-plant | arm=0, counter-arm=2 ✓ |
| F-5A via Write + lease=[docs/]: plant `docs/trackfw.yaml` → `docs/agents-working-context/x.md` rc=0 | FN | baseline=2, step1=0, arm=0 medidos | (a) fecha o plant (componente EqualFold); (d) fecha D3 | rc=0 ✓ |
| F-5A via Write + lease=[src/]: plant `.claude/trackfw.yaml` → gated (fora de L) | esperado rc=2 | rc=2 medido | lease bloqueia plant de `.claude/trackfw.yaml` quando lease=[src/] | rc=2 ✓ |
| Exact-file lease `allow=["P/trackfw.yaml"]` (sem `/`): plant via Write → rc=0; arm → D3-only bypass (cwd sob P/ necessário) | FN | pre=2, plant=0, arm=0, counter-arm=2 medidos | (a) fecha plant antes de Step 7; (d) D3-only — não cobre este D4 escape | plant=0, arm=0 ✓ |
| F-5-cred (sessão em projeto): `RunCredentialGlobal` + Write `cwd/trackfw.yaml` com `mode: warn` → downgrade block→warn | FN | pre=2, arm=0 medidos (--global, binário, yaml na fixture — independente do canal) | piso no sítio L448 de `RunCredentialGlobal` (primário — fecha qualquer canal; não adicionar literal "block" na função compartilhada); (a) em write-scope também fecha via Write quando hook ativo em projeto | pre=2, arm=0 ✓ |
| F-5-cred (sessão fora de projeto): Write `cwd/trackfw.yaml` em $NOPROJ → write-scope inativo; downgrade possível | FN | por leitura do sítio de instalação (nenhum wiring em update.go); pre=2, arm=0 medidos em cwd=sub; transferência para $NOPROJ por leitura L365+L448; medição direta do `RunCredentialGlobal` em $NOPROJ negada pelo classificador (write-scope rc=0 em $NOPROJ medido) | piso L448 em `RunCredentialGlobal` = Wave 1 ML desta REQ; (a) em write-scope NÃO cobre | medido em cwd=sub; transferência $NOPROJ por leitura ✓ (ver §4 R-9) |
| F-5-cred via Bash: plant `cwd/trackfw.yaml mode: warn` → pós-fix RunCredentialGlobal deve bloquear chamada Bash seguinte (rc=2) | FN | pre=2, arm=0 (binário, yaml na fixture — canal independente; Bash produz mesmo arquivo) | piso L448 em `RunCredentialGlobal`; credential guard IS o Bash hook — Bash-planted yaml recebe "block" | Não medido pós-fix; Wave 2 (red team) deve cobrir |
| Windows exact-file lease: `P/trackfw.yaml.` lease=`["P/trackfw.yaml"]` — lease bloqueia por pathEqual? | FN incerto (Windows) | Não medido | por leitura: `EvalSymlinks` non-exist branch retorna sem normalização; pathEqual false → lease bloqueia; `normalizeTargetPath` ML fecha de qualquer forma | Não medido — ver R-8 §4 |
| FIFO `.../trackfw.yaml` plantado (via Bash/mkfifo) | FN + hang | ReadFile bloqueia | fix(e) `fileutil.ReadRegularFile` | HUNG medido (3s, killed) ✓ |
| Forma II: `IsRegular()` → skip; symlink regular perde raiz | FN (legítimo) | rc=2 atual (correto) → rc=0 se skip | fix(e) `ReadRegularFile` que segue symlink | rc=2 atual ✓ |
| Forma III: (d) com yaml frozen_paths apenas | FN pós-fix | rc=0 se outer sem `write_scope` | Teste AC1 com outer yaml SEM `write_scope.frozen_paths`; yaml na fixture, não via Write | Lógica; baseline rc=2 vem dos hardcoded (medido) ✓ |
| Forma V: drop-in `ReadRegularFile` mas `err != nil → nil` | FN sub-B pós-fix | F-5B via dir fixture = rc=0 (medido) | Troca da condição: `IsNotExist → nil`, outros → deny | rc=0 medido demonstra a lacuna ✓ |
| Forma VI / R-8: `trackfw.yaml.` no Windows | FN Windows | rc=0 (normalizeTargetPath não stripa trailing dot) | (d) é backstop D3; normalização de componente Windows fecha (a) | Não medido (plataforma ausente) |
| (a) bloqueia `services/novo/trackfw.yaml` por subagente | FP | rc=0 atual | AC2 — declarar no ADR | Lógica ✓ |
| Credential global floor pós-fix: projeto com `credential_guard: mode: warn` no `trackfw.yaml` lido pelo hook global via cwd → rc=2 (pré-fix: arm=0; opt-in explícito — `init` não scaffolda; documentado credential.go:357-358) | FP | arm=0 (medido via cwd=sub, yaml warn — documenta feature perdida) | Decisão no ADR (AC3): anotar perda; teste asserting rc=2 pós-fix | arm=0 ✓ |
| Credential project scope counter-arm pós-fix: `RunCredential` (L334, default="warn") + yaml `mode: warn` → deve continuar rc=0 pós-fix (piso de "block" não deve propagar para escopo de projeto) | FP (risco de implementação) | rc=0 atual (correto) | Gate: test caminho L334; falha se piso entrar na função compartilhada com literal "block" | rc=0 atual ✓; pós-fix Não medido — Wave 1 obrigatório |
| (d) + fix(e): yaml inválido/não-regular em ancestral acima | FP | deny-all para escritas abaixo | Decisão de boundary no ADR | Lógica ✓ |
| (a) Write `.../TRACKFW.YAML` (subagente) → rc=2 | FN se (a) não usa EqualFold | rc=0 atual | (a) string EqualFold — independe do FS; teste não pode ser pulado em CI case-sensitive | Não medido com fix |
| `TRACKFW.YAML` fixture → (d) Walk via fixture APFS → rc=2 | FN se (d) não usa EqualFold | rc=0 atual | (d) `buildFrozenList` via `ReadRegularFile`; CI deve ter FS case-insensitive para esta variante | Não medido com fix |
| Worktree legítimo pós-fix | esperado rc=2 | rc=2 atual | AC1 contra-braço | rc=2 ✓ |
| Monorepo yamls pré-existentes pós-fix | esperado rc=2 | rc=2 atual | AC1 monorepo | rc=2 ✓ |
| Symlink `trackfw.yaml` → arquivo regular pós-fix | esperado rc=2 | rc=2 atual | AC1: symlink não quebrado pelo fix | rc=2 ✓ |

### Vetores de teste obrigatórios para AC1

| Vetor | O que afirma |
|---|---|
| Write a `.../trackfw.yaml` (subagente, sem lease ou lease=[.claude/]) → rc=2 | (a) fecha Step 1 de sub-variante A; executar sem lease ou com lease=[.claude/] (plant pré-fix = rc=0 medido nesses regimes) — com lease=[src/] o plant já era rc=2 pré-fix pela lease (medido), tornando o vetor decorativo para (a) |
| Write a `.../trackfw.yaml/x` (subagente) → rc=2 | (a) verifica componente, não basename |
| Exact-file lease `allow=["P/trackfw.yaml"]` (sem trailing slash); Write `P/trackfw.yaml` (subagente) → rc=2 pós-fix | (a) fecha plant via exact-file lease; pré-fix plant=rc=0 medido (pre=2, plant=0) |
| Write a `.../TRACKFW.YAML` (subagente) → rc=2 | (a) usa EqualFold incondicional; teste é comparação de string — não depende do FS; não pode ser pulado em CI case-sensitive |
| `TRACKFW.YAML` plantado na fixture (não via Write); Write a `settings.json` → rc=2 | (d) fecha sub-variante C; **este vetor** depende de FS case-insensitive; CI deve pular em FS case-sensitive ou usar setup APFS |
| Outer yaml SEM `write_scope.frozen_paths`, yaml plantado na fixture; Write a `settings.json` → rc=2 | (d) usa buildFrozenList completo, não yaml frozen_paths |
| Diretório `trackfw.yaml` plantado na fixture (não via Write); Write a `settings.json` → rc=2 | fix(e) ErrNotRegularFile → deny; não "walk continua" |
| FIFO `.../trackfw.yaml` plantado na fixture (via mkfifo); guard deve sair rc=2 sem travar | fix(e) `fileutil.ReadRegularFile` |
| Symlink `trackfw.yaml → real.yaml`; Write a `settings.json` → rc=2 | fix(e) não quebra symlinks válidos |
| Write `trackfw.yaml` em dir sem projeto → rc=2 | (a) age antes do Step 7 not-found allow |
| Worktree contra-braço → rc=2 | fix não regride o worktree |
| Monorepo yamls pré-existentes → rc=2 | fix não regride monorepo |

---

## 4. Residual Declarado

> *O que este design aceita não cobrir, dito com clareza.*

### Ação 4 — Modelo de ameaça diante do residual de Bash

**Todos os 12 agentes no catálogo têm Bash (output completo medido):**

```
grep -n "^tools:" internal/integrations/assets/agents/*.md

internal/integrations/assets/agents/code-quality.md:6:tools: Read, Grep, Glob, Bash, WebSearch, AskUserQuestion, Write, Edit
internal/integrations/assets/agents/backend.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/data.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/architect.md:6:tools: Agent, Read, Edit, Write, Bash, Grep, Glob, WebSearch, WebFetch, AskUserQuestion, EnterPlanMode, ExitPlanMode, TaskCreate, TaskGet, TaskList, TaskUpdate, TaskStop, TaskOutput
internal/integrations/assets/agents/infra.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/iac.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/dba.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/frontend.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/ux.md:6:tools: Read, Grep, Glob, Bash, WebSearch, AskUserQuestion, Write, Edit
internal/integrations/assets/agents/qa.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/tooling.md:6:tools: Read, Edit, Write, Bash, Grep, Glob, AskUserQuestion
internal/integrations/assets/agents/security.md:6:tools: Read, Grep, Glob, Bash, WebSearch, AskUserQuestion, Write, Edit
```

12/12 agentes do catálogo padrão têm Bash.

**O que esta REQ fecha:**

1. Projetos que removem Bash da lista de ferramentas (ou CLIs sem ferramenta de shell) ganham
   proteção real via Write/Edit.
2. Profundidade de defesa: mesmo com Bash, Write/Edit não é uma via alternativa aberta.
3. Com F-5 fechado, qualquer bypass futuro via Write/Edit requer nova exploração.

**O que fica aberto (residual pré-declarado no ADR-2026-10-23):**

> "Resíduo: escrita por Bash (`echo >>`, `sed -i`, `python -c`, `cp`) escapa da D3 e da D4.
> Só a D2 pega, na auditoria." — ADR-2026-10-23, Consequences.

### Ação 5 — Residuais declarados

**R-1: Bash escapa de D3/D4 (declarado no ADR-2026-10-23).**
F-5 via Write/Edit fechado por esta REQ. Canal Bash permanece. 12/12 agentes têm Bash.

**R-2: Sub-variantes B e C são do mesmo mecanismo.** Dentro desta REQ.

**R-3: Escrita de yaml aninhado por subagente — FECHADO POR (a).**
Design (a) nega qualquer Write com componente `trackfw.yaml` antes de Step 7.
Declara no ADR: subagentes não podem criar nem sobrescrever arquivos com componente
de caminho `trackfw.yaml`; `trackfw init` em subdiretório requer sessão principal.

**R-4: F-5A é alcançável via Write sob qualquer lease L para caminhos frozen dentro de L:**

`writescope.go:196-199`: `agent_type` ausente, vazio ou inválido como slug → D3-only (nenhuma lease
carregada). `agentfiles.go:936`: payload nativo de PreToolUse não inclui `agent_type`; leases são
criadas manualmente pelo arquiteto. D3-only é o regime típico para subagentes Claude Code.

A lease controla apenas **qual diretório pode hospedar a planta**. Se L autoriza P/, o agente pode
Write `P/trackfw.yaml` (pré-fix, rc=0 medido com lease=[docs/]). Depois: D3 freeze para alvos
dentro de P é contornado. Para alvos frozen fora de L: plant bloqueado pela lease → rc=2 (medido:
`.claude/trackfw.yaml` com lease=[src/]).

- **Regime D3-only (padrão de facto):** plant `.claude/trackfw.yaml` via Write → rc=0 pré-fix
  (`.claude/trackfw.yaml` não está na frozen list hardcoded). F-5A BLOQUEANTE.
- **Regime com lease=[L]:** plant via Write em qualquer P ⊂ L → rc=0 pré-fix para P/trackfw.yaml
  → D3 bypass para todos os frozen paths dentro de P (medido: lease=[docs/], baseline=2, step1=0,
  arm=0). F-5A BLOQUEANTE para frozen paths dentro de L.
- **Frozen paths fora de L:** plant bloqueado pela lease (rc=2 medido: lease=[src/],
  plant=.claude/trackfw.yaml). D3 bypass via Write não alcançável para esses caminhos.
  F-5A via Bash = R-1.

Fix em todos os regimes prefix (trailing `/`): (a) bloqueia Write a qualquer `P/trackfw.yaml`
antes de Step 7; (d) fecha D3 se plant ocorrer por outro meio (Bash residual R-1).
Fix para exact-file lease (sem trailing `/`): estritamente (a). (d) não cobre D4 exact-file
escape (D3-only). Exact-file bypass medido: pre=2, plant=0, arm=0, counter-arm=2 (cwd precondition
sob P/ para arm funcionar).

**R-5: MultiEdit não está em `writeScopeWriteTools` (rc=0, pré-existente).**
Mecanismo diferente; fora de escopo desta REQ.

**R-6: OpenCode lowercase tool_name bypassa Step 4 (gap sem ADR de residual).**
F-5 não é o vetor relevante para OpenCode (Step 4 retorna 0 antes de Step 7, medido).
Gap de lowercase é conhecido (ML-5-0, 2026-11-19) mas sem ADR de residual ainda.
Verificar issues/REQs abertas antes de abrir novo ADR.

**R-7: FIFO em ancestral (design d). FECHADO POR fix(e).**
`fileutil.ReadRegularFile` fecha em cada passo do walk sem hang.

**R-8: Windows bypass vectors para (a) (`trackfw.yaml.`, trailing space, `::$DATA`) — write-scope D3 + credential guard.**
`normalizeTargetPath` e `filepath.Clean` não removem trailing dot ou espaços em componentes.
Para **write-scope**: (d) é o backstop D3 no Windows. Se a lease autoriza P/ e frozen paths estão
dentro de P/, `P/trackfw.yaml.` escapa de (a) → D3 bypass para frozen paths dentro de P; (d) fecha
D3. Se frozen paths estão fora de L: plant bloqueado pela lease → rc=2. Exact-file lease
(`allow=["P/trackfw.yaml"]`) + `P/trackfw.yaml.`: por leitura, `resolveWriteScopePath` em
target inexistente usa ramo non-exist (`writescope.go:462-478`) → retorna `P\trackfw.yaml.` sem
normalização → `pathEqual("P\trackfw.yaml.", "P\trackfw.yaml")` = EqualFold(Clean, Clean) = false
→ lease BLOQUEIA plant (rc=2 esperado) independentemente de (a). Por leitura, não medido; se
leitura estiver errada, nem (a) nem (d) cobrem (d é D3-only). `normalizeTargetPath` ML fecha de
qualquer forma.

Para **credential guard** (trailing-dot, Write em sessão de projeto): (a) em write-scope não
bloqueia `trackfw.yaml.` (EqualFold false na string raw). Win32 cria `trackfw.yaml` sem dot →
credential guard lê e downgrade. Wave 1 ML fecha via piso no sítio L448 de `RunCredentialGlobal`
(ou `max(defaultMode, yaml_mode)` na função — não o literal "block") — sem depender de
`normalizeTargetPath`.

Sobrescrita de alvo EXISTENTE (`P/trackfw.yaml` já existe) para exact-file lease no Windows: não
analisada (EvalSymlinks no ramo de arquivo existente retorna path normalizado; lease pode ou não
casar). `normalizeTargetPath` ML fecha de qualquer forma.

CI Windows deve cobrir estes vetores.

**R-9: Credential global — downgrade via Write em qualquer sessão (projeto ou não).**
Credential guard usa matcher `Bash|PowerShell` — Write nunca interceptado pelo hook. Write-scope
é projeto-level (nenhum wiring global em update.go — por leitura do sítio de instalação). Em
qualquer sessão (dentro ou fora de projeto), (a) não protege `RunCredentialGlobal` via Write,
porque: (1) write-scope não atinge `RunCredentialGlobal` pelo matcher; (2) fora de projeto,
write-scope não está wired. pre=2 (sem yaml), arm=0 (yaml mode=warn) medidos em cwd=sub;
transferência para NOPROJ por leitura (L365 sem walk-up, L448 único sítio — ver Medição NOPROJ
abaixo); medição direta do `RunCredentialGlobal` em $NOPROJ negada pelo classificador de
ambiente (write-scope em $NOPROJ foi medido — rc=0, ramo found=false). **Recomendação Wave 1 ML desta REQ** (Regra Dura — mesma causa):
read-side monotonicity em `credReadModeWithDefault` para `--global` scope (ver Veredicto AC3).
Pós-fix, Bash-planted yaml com `mode: warn` TAMBÉM recebe "block" na próxima chamada Bash
(credential guard IS o Bash hook) — R-1 de write-scope não se aplica a este vetor pós-fix.
Medição NOPROJ: `RunCredentialGlobal` nunca faz walk-up (L365) e lê apenas
`filepath.Join(cwd,"trackfw.yaml")` (L448) — cwd=sub/ sem yaml raiz exercita o mesmo code path
que cwd=NOPROJ. pre=2 (sem yaml), arm=0 (yaml warn) medidos em cwd=sub; transferência para
$NOPROJ por leitura (L365+L448 são o único sítio de derivação; ausência de project root não
muda o fluxo). Medição direta em $NOPROJ negada pelo classificador de ambiente.

Nota de cwd (hipótese, não verificada): `os.Getwd()` (credential.go:444) é o process cwd do
hook, não o payload `cwd`. Medição usou process cwd=sub/ explicitamente. Como Claude Code spawna
o hook não foi verificado. Dois ramos: (ramo 1) se process cwd = session root, yaml lido é o
real (raiz do projeto) — in-project vector fecha antes de qualquer fix; (ramo 2) se process cwd
segue o cwd corrente da sessão (`cd` persiste), vetor in-project permanece aberto. R-9 é
relevante em ambos os ramos para sessões fora de projeto (nenhum project root → yaml de cwd é o
único lido).

**R-10 (residual distinto — pré-existente, ML-FA 2026-11-17): Write a `~/.claude/settings.json` via subagente.**
Frozen list é projeto-relative (writescope.go:65-66 — `.claude/settings.json` relativo à raiz
do projeto). `~/.claude/settings.json` está fora de qualquer project root → `findProjectRoot`
retorna `found=false` → write-scope permite. Evidência: (1) medido nesta sessão — target
`$NOPROJ/trackfw.yaml` rc=0 (mesmo ramo: found=false → allow); (2) ML-FA 2026-11-17: lido para
`~/.codex/hooks.json` (mesma função, mesmo ramo); `~/.claude/settings.json` transferência por
leitura — fora dos limites de medição desta sessão. Nenhum yaml envolvido: o mecanismo de
re-rooting (REQ negativa: "credential guard entra somente se a varredura achar o mesmo
mecanismo") não se aplica — residual declarado, não ML desta REQ. Se Claude Code's permission
layer bloqueia este Write: não verificado. Via Bash: já aberto por R-1.
