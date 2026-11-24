# Red Team — Write-scope com raiz aninhada (ML-2B)

**REQ:** REQ-2026-11-23  
**Branch:** `fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado`  
**Commits auditados:** `d25e3b02` → `7613c035` (HEAD: 7613c035)  
**Go runtime:** go1.25.2 darwin/arm64  
**Binário pré-fix:** compilado de `3ae8d29c` via `git archive 3ae8d29c | tar -x -C $SCRATCH; go build`  
**Revisor:** Hades (hades-tf)  
**Data:** 2026-11-24  
**Veredito:** APROVA COM RESSALVAS

---

## Resumo executivo

Os contratos D1, D2 e D3 do ADR-2026-11-24 foram falsificados em ambas as direções (bypass e falso
positivo) contra binários compilados da branch; todos os vetores F-5A step 1, F-5A step 2 (Forma
III), F-5B, F-5C, F-5-FIFO do Wave 0 passam; F-5D write-path passa; F-5D lease-path permanece
aberto via R-1 (Bash plant). D4 (modo do guard de credencial) foi verificado por leitura + suíte;
F-5-cred não foi re-executado (razão: §D4).

**Ressalva — ASSIMETRIA ENTRE D2 E ADR-2026-10-23 D4 (leases) — F-LEASE-MONO:**

ADR-2026-10-23 (linhas 109 e 114) define a raiz do alvo como o ancestral mais próximo do target
com `trackfw.yaml`, e a lease em `<raiz do alvo>/.trackfw/leases/`. A Wave 4 verifica as "duas
raízes" (sessão e alvo, linha 118): seu objetivo declarado é impedir que um projeto falso com lease
ampla sobreponha a lease restrita do projeto legítimo. O ADR é silente sobre leases de ancestrais
além das duas raízes.

Em um monorepo legítimo com `services/api/trackfw.yaml`, quando o cwd da sessão resolve para
`services/api/`, ambas as raízes resolvem para `services/api/` e uma lease criada em `MONO/` está
fora do escopo documentado de ambas: o guard não a consulta, conforme especificado (linhas 109, 114).

No entanto, esta REQ tornou frozen paths monotônicos (D2): frozen paths do yaml pai cascateiam para
projetos aninhados. ADR-2026-10-23 D4 (leases) mantém nearest-root. A assimetria resultante — frozen
paths cascateiam, leases não — não está explicitamente documentada nos ADRs. O mecanismo é diferente
do que D1–D3 fecham (ver §F-LEASE-MONO): F-LEASE-MONO requer uma raiz aninhada legítima que o
subagente não criou, combinada com uma lease fora do escopo documentado. Os vetores D1–D3 não fecham
esse cenário.

**Ação recomendada:** emendar ADR-2026-10-23 (doc-only, §7) para declarar explicitamente que leases
são nearest-root por design e não cascateiam para projetos aninhados, em contraste com frozen paths
monotônicos (ADR-2026-11-24). Esta emenda deve andar no mesmo PR (artefato doc-only).

---

## Nota de numeração — ADR D3/D4 vs ADR-2026-11-24 D1–D4

O Wave 0 (`2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md`) segue a numeração do
ADR-2026-10-23 onde D3 = frozen list e D4 = leases (write-scope). O ADR-2026-11-24 redefine
D1 = EqualFold freeze, D2 = monotonic union, D3 = ErrNotRegularFile, D4 = credential mode.
Neste relatório: D1–D4 refere-se sempre ao ADR-2026-11-24. Quando citando o Wave 0, nomes são usados
explicitamente.

---

## Ambiente de medição

```
Plataforma:  macOS APFS (case-insensitive, case-preserving)
Go runtime:  go1.25.2 darwin/arm64
Binário pós-fix:  bin/trackfw compilado de HEAD 7613c035  (TFFIX)
Binário pré-fix:  compilado de 3ae8d29c                   (TFPRE)
Fixtures:    /private/tmp/claude-501/.../scratchpad/ (fora da árvore do repo)
Interface:   printf '{"tool_name":"Write","tool_input":{"file_path":"..."},"agent_id":"...","cwd":"..."}' \
             | HOME=/tmp/h1tf USERPROFILE=/tmp/h1tf CODEX_HOME=/tmp/ch1tf XDG_CONFIG_HOME=/tmp/xdg1tf \
               <BINARY> guard write-scope
KELVIN_COUNT (U+212A, bytes e2 84 aa, alias no APFS): 1  (mesmo inode que trackfw.yaml)
```

Nota SIGALRM: `perl -e 'alarm 5; exec @ARGV'` não delimita binários Go sem `signal.Notify` —
o runtime Go captura SIGALRM e não termina o processo por padrão. O hang pré-fix no D3-2 foi
observado além de 30s e terminado externamente; o pós-fix retornou rc=2 em 0s.

---

## Tabela de Formas (falsificação de implementação)

| Forma | Mecanismo | Detectada por | Resultado fix |
|---|---|---|---|
| I | Freeze de caminho literal `.claude/trackfw.yaml` apenas; variantes B/C sobrevivem | F-5A sub-B/C | D1 EqualFold + hasTrackfwYAMLComponent fecha todas as sub-variantes |
| II | `IsRegular()` como única correção; `TRACKFW.YAML` é regular → aceito; symlink legítimo → perde raiz | F-5B via TRACKFW.YAML | D3 ReadRegularFile segue symlink; D1 fecha UPPERCASE (medido) |
| III | (d) monotônico aplica `writeScopeSubagentFrozenRelPaths` só na raiz mais próxima; outer yaml sem `write_scope` → frozen list efetiva vazia | F-5A step 2 | D2 buildFrozenList hardcoded por ancestral (medido) |
| IV | Fix marcado ✅, 12/12 agentes têm Bash | Declarado R-1 | Coberto por R-1 |
| V | `ReadRegularFile` drop-in mas `err != nil → nil`; dir aceito | F-5B via dir | D3 separa IsNotExist de outros erros; rc=2 medido |
| VI (R-8) | Trailing `.`/espaço/`::$DATA` em Windows | R-8 pré-declarado | Trailing-dot e trailing-space: D1 fecha via normalizePathComponent (medido); `::$DATA`: aberto (R-8b) |

---

## Resultados por contrato

### D1 — Subagente não pode escrever em caminho com componente `trackfw.yaml`

**Contrato:** `hasTrackfwYAMLComponent` com EqualFold incondicional sobre cada componente do
caminho alvo, antes de qualquer consulta de raiz ou lease. `normalizePathComponent` strip de
trailing dots e spaces antes do EqualFold. Não aplica a main sessions.

#### D1-1: raiz `trackfw.yaml` — NÃO discrimina pré-fix (referência)

```
TFPRE: denied: "…/trackfw.yaml" is a frozen path (trackfw.yaml)  rc=2
TFFIX: denied: "…/trackfw.yaml" targets a trackfw.yaml component (D1)  rc=2
```

A raiz `trackfw.yaml` já estava na frozen list hardcoded pré-fix. Não discrimina.

#### D1-2a (F-5A step 1): `trackfw.yaml` em subdiretório — DISCRIMINA

```
TFPRE: rc=0        ← plant permitido pré-fix
TFFIX: denied: "…/.claude/trackfw.yaml" targets a trackfw.yaml component (D1)  rc=2  PASS
```

#### D1-3a: lease com `allow=[.claude/trackfw.yaml]`; D1 ainda bloqueia

```
TFPRE: rc=0   (lease carrega e permite .claude/trackfw.yaml; sem D1)
TFFIX: denied: "…/.claude/trackfw.yaml" targets a trackfw.yaml component (D1)  rc=2  PASS
```

#### D1-3b: app.go em lease — allow (D1 não dispara)

```
TFPRE: rc=0   TFFIX: rc=0  PASS
```

#### D1-3c: other.go não em lease — deny (prova que lease está viva)

```
TFPRE: denied: "…/other.go" is not in the allow list of lease …/a1.json for "a1"  rc=2
TFFIX: denied: "…/other.go" is not in the allow list of lease …/a1.json for "a1"  rc=2  PASS
```

#### D1-4K: Kelvin sign U+212A (bytes `e2 84 aa`)

```
TFPRE: rc=0
TFFIX: denied: "…" targets a trackfw.yaml component (D1)  rc=2  PASS
```

KELVIN_COUNT=1 (APFS: mesmos bytes = mesmo inode). EqualFold Unicode fecha o caso.

#### D1-4T: trailing dot `trackfw.yaml.` — R-8a FECHADO

```
TFPRE: rc=0
TFFIX: denied: "…/trackfw.yaml." targets a trackfw.yaml component (D1)  rc=2  PASS
```

#### D1-4S: trailing space `trackfw.yaml ` — R-8a FECHADO

```
TFPRE: rc=0
TFFIX: denied: "…/trackfw.yaml " targets a trackfw.yaml component (D1)  rc=2  PASS
```

#### D1-FP1: main session escreve trackfw.yaml

```
TFPRE: rc=0   TFFIX: rc=0  PASS (D1 condicionado a payloadHasAgentID)
```

#### D1-FP2: subagente escreve testdata/trackfw.yaml (FP novo de D1)

Fixture: `D1FP2F/trackfw.yaml` (outer yaml regular), `D1FP2F/testdata/` (dir vazio; `testdata/trackfw.yaml` AUSENTE).

```
TFPRE: rc=0   (testdata/trackfw.yaml ausente; pré-fix não tem D1; allow)
TFFIX: denied: "…/testdata/trackfw.yaml" targets a trackfw.yaml component (D1)  rc=2
```

FP novo de D1: subagentes não podem criar `trackfw.yaml` em nenhum subdiretório.
ADR-2026-11-24 §Consequences aceita explicitamente: "subagente não cria projeto aninhado".
Repo: `git ls-files | grep -iE '(^|/)trackfw\.yaml$' | grep -v '^trackfw.yaml$' | wc -l` = 0.

Nota: com `testdata/trackfw.yaml` JÁ existente no fixture, o pré-fix re-enraíza no testdata/ e
a frozen list do root-yaml casa o alvo (rc=2). A discriminação de D1 requer fixture fresco.

#### F-5C: `TRACKFW.YAML` (uppercase ASCII) — fresh fixture

```
(Fixture fresco: .claude/TRACKFW.YAML ausente)
TFPRE: rc=0        ← TRACKFW.YAML não reconhecido pré-fix
TFFIX: denied: "…/TRACKFW.YAML" targets a trackfw.yaml component (D1)  rc=2  PASS
```

Nota APFS: com TRACKFW.YAML já existente no fixture, o pré-fix retorna rc=2 via "is a frozen path
(trackfw.yaml)" (APFS alias = mesmo inode). Usar fixture fresco discrimina D1.

**D1 Windows separators:** `hasTrackfwYAMLComponent` (writescope.go:874) usa `strings.FieldsFunc` com
split em `/` e `\`. Após `normalizeTargetPath` converter `C:\proj\.claude\trackfw.yaml` para forma
nativa, a separação em `\` extrai corretamente cada componente. Verificado por leitura; não medido
contra binário Windows. Os testes de D1 em `writescope_nested_root_test.go` não contêm skip por
`runtime.GOOS` ou build tag para Windows (apenas um comentário sobre `Mkfifo` indisponível).

**D1 Windows — R-8b (`::$DATA`): ABERTO** — `hasTrackfwYAMLComponent` não reconhece o sufixo
`::$DATA`. Ambas as variantes medidas em 2026-11-24 retornam rc=0 pós-fix:
- Raiz: target=`PROJ/trackfw.yaml::$DATA` → pré=0, pós=0.
- Aninhado: target=`PROJ/.claude/trackfw.yaml::$DATA` → pré=0, pós=0.
No Windows, o default data stream `::$DATA` é o mesmo arquivo — um subagente pode reescrever
`trackfw.yaml` (incluindo `frozen_paths` e credential mode do projeto) via Write sem ser bloqueado.

**D1: PASS (darwin); Windows: `::$DATA` contorna D1 (R-8b)**
11 vetores; FP declarado no ADR; 0 nested yamls no repo.
Regra Dura: R-8a e R-8b falham na mesma função (`normalizePathComponent`); a decisão para o
arquiteto é novo ML na REQ vigente ou linha de residual declarado no ADR-2026-11-24. Nota sobre 8dot3
(`TRACKF~1.YAM`): mecanismo diferente — requer lookup de filesystem, não strip de string; manter
separado.

---

### D2 — União monotônica de frozen lists

**Contrato:** `buildFrozenList` coleta de `projectRoot` e todos os yamls ancestrais, incluindo
`writeScopeSubagentFrozenRelPaths` hardcoded por ancestral. A raiz é a do cwd; nenhum re-root.

#### D2-1b (F-5A step 2, Forma III): outer yaml sem `write_scope.frozen_paths`; hardcoded re-aplicado

```
D2FIII/trackfw.yaml (adr_dirs: [docs/adr], SEM write_scope.frozen_paths)
D2FIII/.claude/trackfw.yaml (plantado)
TFPRE: rc=0  ← re-root; writeScopeSubagentFrozenRelPaths não reaplicado do outer
TFFIX: denied: "…/.claude/settings.json" is a frozen path (.claude/settings.json)  rc=2  PASS
```

TFFIX rc=2 vem da lista hardcoded do yaml ancestral. Este é o resultado central da Forma III.

#### D2-2a: outer yaml com `frozen_paths` explícito + plant (subagente)

```
TFPRE: rc=0  ← re-root; outer frozen_paths ignorados
TFFIX: denied: "…/.claude/settings.json" is a frozen path (.claude/settings.json)  rc=2  PASS
```

#### D2-3m: main session, outer yaml com `frozen_paths` explícito + plant

```
TFPRE: rc=0   TFFIX: denied rc=2  PASS
```

#### D2-4: subagente escreve arquivo legítimo (não frozen) — counter-arm

```
TFPRE: rc=0   TFFIX: rc=0  PASS
```

#### D2-FP2 (FP novo de D2): yaml ancestral inválido nega toda escrita abaixo

```
D2FP2/trackfw.yaml  (INVALID YAML, acima do projeto)
D2FP2/PROJ/trackfw.yaml  (válido, adr_dirs: [])
TFPRE: rc=0   (pré-fix só lê o yaml da raiz do alvo; yaml ancestral inválido não alcançado)
TFFIX: denied: trackfw.yaml: invalid YAML …  rc=2
```

FP declarado em ADR-2026-11-24 §Consequences linha 86: "um yaml quebrado num diretório acima do
projeto nega toda escrita de subagente abaixo dele".

#### D2-5a e D2-5b (F-5D Write-path): D2 bloqueia mesmo com lease `allow=[.claude/]`

```
F5DW: trackfw.yaml, .claude/trackfw.yaml (plant), .trackfw/leases/a1.json (allow:[F5DW/.claude/])

D2-5a (target=.claude/settings.json, cwd=F5DW):
TFPRE: rc=0  ← frozen list re-enraíza em .claude/ (miss); outer lease carregada via cwdRoot
               mas allow=[.claude/] ⊃ settings.json → lease permite; resultado: allow
TFFIX: denied: "…/.claude/settings.json" is a frozen path (.claude/settings.json)  rc=2  PASS

D2-5b counter-arm (lease allow=[src/], target=.claude/settings.json, cwd=f5dfx2):
TFPRE: denied: "…/f5dfx2/.claude/settings.json" is not in the allow list of lease a1.json  rc=2
TFFIX: denied: "…/.claude/settings.json" is a frozen path (.claude/settings.json)  rc=2
```

D2-5a isola o bypass do pré-fix: a frozen list re-enraíza em `.claude/` (perde o hardcoded do outer),
mas se a lease permite o target, o pré-fix deixa passar. O pós-fix bloqueia via D2 independente da
lease (ADR-2026-10-23 linha 117 confirmado: "caminhos congelados da D3 continuam negando mesmo
dentro do `allow`").

Nota sobre F5C-2 (do log de medição): vetor `D2 step2: TRACKFW.YAML already planted; expect pre=0,
post=2` → `pre=0  post=2  PASS`. Em macOS APFS, TRACKFW.YAML com arquivo já existente é alias
(mesmo inode que `trackfw.yaml`); o vetor confirma que D2's frozen path re-aplicado via outer yaml
bloqueia quando o plant já existe. Equivalente a D2-1b em resultado; listado como vetor adicional.

**D2: PASS** — 7 vetores (D2-1b, D2-2a, D2-3m, D2-4, D2-FP2, D2-5a/5b, F5C-2); ambos os FPs declarados no ADR

---

### D3 — ErrNotRegularFile nega sem travar

#### D3-1a (F-5B): diretório `.claude/trackfw.yaml/` plantado

```
TFPRE: rc=0  ← dir aceito por Lstat, re-root para .claude/
TFFIX: denied: findProjectRoot: trackfw.yaml at "…/.claude": not a regular file  rc=2  PASS
```

#### D3-1b: target path contém `trackfw.yaml/x.go`

```
TFPRE: rc=0
TFFIX: denied: "…/trackfw.yaml/x.go" targets a trackfw.yaml component (D1)  rc=2  PASS
```

#### D3-2 (F-5-FIFO): FIFO nomeado `trackfw.yaml`

```
TFPRE: HUNG > 30s  (os.Open bloqueia; Go runtime não termina em SIGALRM sem signal.Notify)
TFFIX: denied: findProjectRoot: trackfw.yaml at "…/d3fifo": not a regular file  rc=2  elapsed=0s  PASS
```

`fileutil.ReadRegularFile` abre com `O_NONBLOCK`: FIFO sem writer → `open(2)` retorna imediatamente,
fstat vê FIFO → `ErrNotRegularFile`.

#### D3-FP1 (Forma II): symlink trackfw.yaml → arquivo regular, escreve app.go

```
D3FP/real.yaml (regular), D3FP/trackfw.yaml → real.yaml (symlink sem mkdir)
TFPRE: rc=0   TFFIX: rc=0  PASS (symlink seguido; raiz encontrada; app.go não frozen)
```

#### D3-FP2 (Forma II): symlink, escreve .claude/settings.json

```
TFPRE: rc=2   TFFIX: rc=2  PASS (raiz encontrada via symlink; settings.json frozen)
```

Forma II fechada: `ReadRegularFile` segue symlink para arquivo regular; proteção preservada.

**D3: PASS** — 5 vetores

---

### D4 — Modo do guard de credencial é o mais estrito da cadeia

**Verificado por leitura de fonte + suíte da branch; não falsificado de forma independente por medição viva.**

**F-5-cred não foi re-executado.** Razão: o credential guard bloqueia qualquer payload com padrão
reconhecível (JWT incluindo padrões `eyJ...` sintéticos, AWS keys, GH tokens) e payloads genéricos
não acionam padrões — logo rc=0 não prova que D4 passa; prova que nenhum padrão foi acionado. As
restrições de medição proíbem tokens reais ou sintéticos reconhecíveis.

| Ponto | Localização | O que o código diz |
|---|---|---|
| Global always blocks | linha 371 (`RunCredentialGlobal`) | sem walk, sem warn path; "The warn branch and attention-write are unreachable" |
| Project: strictest-mode walk | linha 341 (`RunCredential`) | `credWalkStrictestMode(projectCwd, "warn")` |
| Non-regular yaml → "block" | linha 1503 (`credWalkStrictestMode`) | `ErrNotRegularFile → return "block"` |

Suíte `internal/guard/credential_nested_mode_test.go`:
(`go test ./internal/guard/ -run 'TestCredNested_' -v -count=1` → 8 PASS)

| Teste | Assertiva |
|---|---|
| `TestCredNested_GlobalPlantedWarnBlocks` | global blocks mesmo com yaml "warn" plantado |
| `TestCredNested_ProjectPlantedWarnUnderBlockRootBlocks` | mode walk retorna "block" do ancestral |
| `TestCredNested_ProjectWarnOnlyCounterArm` | rc=warn quando só "warn" existe |
| `TestCredNested_DirectoryYamlCountsAsBlock` | dir named trackfw.yaml → "block" |

**D4: não falsificado de forma independente; verificado por leitura + suíte do implementador.**

---

### F-5D — Write-path e Lease-path

#### Write-path (D2 bloqueia mesmo com lease `allow=[.claude/]`)

Ver D2-5a/5b acima. Resumo:

- D2-5a: plant + lease `allow=[.claude/]`, cwd=outer → pré-fix rc=0, pós-fix rc=2 (D2 Forma III)
- D2-5b: plant + lease `allow=[src/]`, cwd=outer → ambos rc=2 (lease deny)
- ADR-2026-10-23 linha 117 confirmado: frozen nega mesmo dentro do `allow`.
- Write-path de F-5D: **PASS**

#### Lease-path (bypass de lease via cwd=projeto aninhado)

```
F5DFIX: trackfw.yaml, .claude/trackfw.yaml (plant), .trackfw/leases/a1.json (allow:[F5DFIX/src/])

Bypass (cwd=F5DFIX/.claude):
printf '{"…F5DFIX/.claude/notes.md","agent_id":"a1","agent_type":"a1","cwd":"F5DFIX/.claude"}'
TFPRE: rc=0   TFFIX: rc=0  ← collectLeaseRoots(target=.claude, cwd=.claude) = (.claude, .claude)
                               outer lease F5DFIX/.trackfw/leases/ não carregada

Counter-arm (cwd=F5DFIX):
printf '{"…F5DFIX/.claude/notes.md","agent_id":"a1","agent_type":"a1","cwd":"F5DFIX"}'
TFPRE: rc=2   TFFIX: rc=2  ← lease carregada do outer; notes.md não em [src/]; negado
```

Este bypass requer `.claude/trackfw.yaml` plantado (via Bash = R-1, ou via Write que D1 agora
fecha). Pós-fix: planting via Write → D1-2a/F5C-1f bloqueiam (rc=2). Lease-path de F-5D:
**residual R-1** (via Bash).

---

## Achado — INFORMATIVO: Assimetria D2/ADR-2026-10-23 D4 (leases) — F-LEASE-MONO

**Severidade:** Informativo (design by-design per ADR-2026-10-23; nenhum ML bloqueante)

**Mecanismo (diferente de D1–D3):** F-LEASE-MONO requer uma raiz aninhada legítima que o subagente
não criou (services/api/trackfw.yaml pré-existente) combinada com uma lease criada pelo arquiteto
fora do escopo documentado (MONO/.trackfw/leases/). Corrigir leases para serem monotônicas não
fecharia nenhuma das falhas medidas em D1–D3; são causas distintas.

**ADR-2026-10-23 (linhas relevantes):**
- Linha 109: "a raiz que vale é a do projeto do **alvo** (o ancestral com `trackfw.yaml`)"
- Linha 114: "a permissão de escrita fica em `<raiz do projeto do alvo>/.trackfw/leases/<agent_type>.json`"
- Linha 118: "toda permissão encontrada **nas duas raízes** (a da sessão e a do alvo) precisa liberar o alvo."
- O ADR é silente sobre leases de ancestrais além das duas raízes.

**Medição:**

```
MONO/trackfw.yaml, MONO/.trackfw/leases/a1.json (allow:[MONO/services/web/])
MONO/services/api/trackfw.yaml (legítimo, sem plant), MONO/services/api/src/x.go (alvo)

Bypass — cwd da sessão resolve para services/api/:
printf '{"…MONO/services/api/src/x.go","agent_id":"a1","agent_type":"a1","cwd":"MONO/services/api"}'
TFPRE: rc=0   TFFIX: rc=0  ← lease de MONO não consultada; alvo root=session root=services/api/

Counter-arm — cwd=MONO:
printf '{"…MONO/services/api/src/x.go","agent_id":"a1","agent_type":"a1","cwd":"MONO"}'
TFPRE: rc=2   TFFIX: rc=2  denied: not in allow list of lease MONO/.trackfw/leases/a1.json

cwd ausente, process cwd=scratchpad ($SCRATCH, sem trackfw.yaml em nenhum ancestral) — medido:
printf '{"…MONO/services/api/src/x.go","agent_id":"a1","agent_type":"a1"}'
TFPRE: rc=0   TFFIX: rc=0  (alvo root=services/api/; nenhuma lease lá; allow por design)
```

Medição do arm cwd-absent re-confirmada em 2026-11-24 com `cd "$SCRATCH"` antes do pipe: `pre=0, post=0`.

**Precondição:** payload deve incluir `agent_type` correspondente ao nome do arquivo de lease.
Dispatch sem `agent_type` não carrega lease.

**Assimetria pós-REQ:** D2 fez frozen paths monotônicos; os do yaml pai cascateiam para projetos
aninhados. ADR-2026-10-23 D4 (leases) mantém nearest-root conforme especificado (linhas 109, 114);
o ADR é silente sobre cascata de leases. O mesmo projeto aninhado legítimo herda frozen paths do
pai (D2) mas não herda a lease do pai (ADR-2026-10-23 D4). Esta assimetria não está documentada.

**Ação recomendada:** emendar ADR-2026-10-23 (doc-only, §7 do CLAUDE.md global). A emenda deve
andar no mesmo PR.

---

## R-8 — Windows: trailing-dot, trailing-space, `::$DATA`

### R-8a: trailing-dot e trailing-space — FECHADOS por D1

```
D1-4T: TFPRE rc=0, TFFIX rc=2 (trackfw.yaml.)
D1-4S: TFPRE rc=0, TFFIX rc=2 (trackfw.yaml )
```

Medidos. R-8a está fechado.

### R-8b: `::$DATA` (NTFS default data stream) — ABERTO

Medido em 2026-11-24 (payload construído via python3 para preservar literal `::$DATA`):
- Raiz: `target=PROJ/trackfw.yaml::$DATA` → `TFPRE: rc=0  TFFIX: rc=0`
- Aninhado: `target=PROJ/.claude/trackfw.yaml::$DATA` → `TFPRE: rc=0  TFFIX: rc=0`

`EqualFold("trackfw.yaml::$DATA","trackfw.yaml")` = `false` → D1 não bloqueia.
O componente `trackfw.yaml::$DATA` não passa por nenhuma frozen check. No APFS, arquivo com esse
nome não existe (default stream é conceito NTFS); rc=0 confirma que nem D1 nem frozen check
intercede. No Windows: `::$DATA` é o default data stream = mesmo conteúdo que o arquivo base; um
subagente pode reescrever `trackfw.yaml` incluindo `frozen_paths` e credential mode.

**Decisão para o arquiteto (Regra Dura):** R-8a e R-8b falham na mesma função
(`normalizePathComponent`); corrigir `normalizePathComponent` para strip de `::…` fecha ambos. Nova
decisão: (a) novo ML nesta REQ vigente, ou (b) linha de residual declarado no ADR-2026-11-24. Esta
REQ não mergeia até a decisão ser tomada e registrada.

Nota 8dot3 (`TRACKF~1.YAM`): requer lookup de filesystem para resolver o short name — mecanismo
diferente do strip de string. Manter separado; medição Windows não realizada.

---

## Residuais confirmados (pré-declarados)

| ID Wave 0 | Descrição | Status |
|---|---|---|
| R-1 | Bash escapa de D3/D4 (12/12 agentes têm Bash) | CONFIRMADO; write-scope é tool-call-only |
| R-6 | OpenCode `tool_name:"write"` (lowercase) → rc=0 | CONFIRMADO; Step 4 compara PascalCase |
| R-MultiEdit | Não coberto por guard de write-scope | CONFIRMADO por leitura |
| R-8a | Windows trailing-dot/space | FECHADO por D1 nesta branch (medido) |
| R-8b | Windows `::$DATA` default stream | CONFIRMADO ABERTO; medido rc=0; pré-declarado |

---

## Suíte de testes da branch — verificação cruzada

```
go test ./internal/guard/... -count=1
ok  github.com/kgsaran/trackfw/internal/guard  3.162s
```

Nenhuma regressão no pacote `internal/guard`.

---

## Veredito detalhado

| Contrato | Resultado | Vetores medidos |
|---|---|---|
| D1 (EqualFold trackfw.yaml, antes de lease) | PASS (darwin); Windows: `::$DATA` contorna D1 (R-8b) | 11 vetores (D1-1, D1-2a, D1-3a/3b/3c, D1-4K, D1-4T, D1-4S, D1-FP1, D1-FP2, F5C-1) |
| D2 (monotonic frozen list, Forma III) | PASS | 7 vetores (D2-1b, D2-2a, D2-3m, D2-4, D2-FP2, D2-5a/5b, F5C-2) |
| D3 (non-regular file → deny, sem hang) | PASS | 5 vetores (D3-1a, D3-1b, D3-2 FIFO, D3-FP1, D3-FP2) |
| D4 (cred mode strictest) | Não falsificado de forma independente | Leitura + 8 testes |
| F-5D Write-path | PASS | D2-5a pre=0/post=2; D2-5b arm pre=2/post=2 |
| F-5D Lease-path | Residual R-1 | bypass 0/0; arm 2/2; via Write: D1 fecha (darwin only; R-8b abre em Windows) |
| F-LEASE-MONO (assimetria D2/ADR-2026-10-23 D4 leases) | INFORMATIVO | bypass 0/0; arm 2/2; abs 0/0 |

**VEREDITO GLOBAL: APROVA COM RESSALVAS**

Todos os contratos D1–D3 passam em darwin. D4 verificado por leitura + suíte. F-5D Write-path
fechado; lease-path permanece residual R-1 (via Bash plant, que D1 fecha para Write em darwin; R-8b
abre o Write-path de volta em Windows via `::$DATA`). F-LEASE-MONO é informativo: ADR-2026-10-23 D4
(linhas 109 e 114) define leases no nearest-root por design; o ADR é silente sobre leases de
ancestrais; assimetria com D2 não documentada; emenda ADR-2026-10-23 recomendada neste PR (doc-only).
F-5-cred permanece aberto do Wave 0: D4 não foi independentemente falsificado.

**Condição de merge:** esta REQ não mergeia até que R-8b seja tratado — novo ML na REQ vigente
(corrigir `normalizePathComponent` para strip de `::…` além de `.` e espaço) ou linha de residual
declarado em ADR-2026-11-24. Decisão para o arquiteto.
