# Red Team ML-2C — write-scope / credential guard (delta 28c03d8c → 249804fb)

> Data: 2026-11-24 | Autor: hades-tf | REQ: REQ-2026-11-23

## Veredito global

**APROVA COM RESSALVAS**

Todos os contratos D1, D3, D5 e D6 passam nos testes medidos em darwin (macOS APFS). Uma ressalva de implementação e uma residual Windows declarados:

1. **R-8dot3-HASH** (LOW): o regex `^trackf~[0-9]+\.yam$` cobre formas TRACKF~1 a TRACKF~N, mas o Windows gera nomes curtos com hash para muitas colisões no mesmo diretório (ex.: `TRACKFBF~1.YAM`). Essa variante não é coberta. Declarar como residual no ADR-2026-11-24.

2. **D3-FP** (INFO/aceito): um `trackfw.yaml` não regular (diretório, FIFO) no caminho ancestral bloqueia TODAS as escritas, inclusive da sessão principal. O ADR aceita o fail-closed. Comportamento correto por design.

---

## Binários construídos

| ID | Commit | Binário |
|---|---|---|
| PRE | 28c03d8c | `$SCRATCH/tfpre` via `git archive | go build` |
| FIX | 249804fb (ML-2C) | `$SCRATCH/tffix` via `git archive | go build` |

Todos os testes rodaram com `HOME`, `USERPROFILE`, `XDG_CONFIG_HOME` e `CODEX_HOME` apontando para tmp isolado. O único desvio foi o trace `bash -x` inicial (cwd com `trackfw.yaml` real); esse resultado não foi registrado como vetor — foi diagnóstico de ambiente.

Confirmado: `git diff --cached --stat` vazio; `docs/roadmaps/.trackfw-attention.json` não criado.

---

## Tabela de vetores

### D3 — Guard de credencial: yaml não regular na cadeia

| Vetor | Payload/comando | rc PRE | rc FIX | Esperado |
|---|---|---|---|---|
| dir `trackfw.yaml` na cadeia de cwd | `sh trackfw-credential-guard.sh`; stub tfpre/tffix; dir no cwd | 0 | 2 | PASS |
| chmod-000 `trackfw.yaml` na cadeia | `sh trackfw-credential-guard.sh`; chmod 000 no projeto | 0 | 2 | PASS |
| sem yaml na cadeia | `sh trackfw-credential-guard.sh`; sem yaml | 0 | 0 | PASS |
| **counter-arm: yaml regular, alvo benigno** | `{"tool_name":"Write","tool_input":{"file_path":"$root/notes.txt"},"agent_id":"claude"}` | 0 | 0 | PASS |

Mecanismo: `credFindNearestProjectRoot` antes pulava qualquer erro (incluindo `ErrNotRegularFile`); agora só pula `os.IsNotExist`, bloqueando todo o resto.

**Nota sobre D3 falso positivo (aceito):** quando um `trackfw.yaml` não regular existe no ancestral, a sessão principal também recebe rc=2. Comportamento esperado pelo ADR (fail-closed).

---

### Wrapper `scripts/trackfw-credential-guard.sh` — testado por execução com `sh`

| Vetor | rc antes | stub chamado antes | rc depois | stub chamado depois |
|---|---|---|---|---|
| dir `trackfw.yaml` no cwd | 0 | NÃO | 0 | SIM (`guard --help` + `guard credential`) |
| symlink quebrado `trackfw.yaml` no cwd | 0 | NÃO | 0 | SIM (`guard --help` + `guard credential`) |

Mecanismo: `[ -f dir ]` = false; `[ -e dir ] || [ -L dir ]` = true para dir e para symlink quebrado. `[ -L broken_symlink ]` retorna true mesmo com alvo ausente.

**Consistência dos três sítios:** `scaffold.go` (generator) e `validator_credential_guard_integrity_reference.go` têm o mesmo diff `-f` → `-e || -L`. Verificado via `diff` nos dois arquivos.

**Nota de ambiente:** hangs anteriores eram causados por `> "$CALLLOG"` em zsh ativando `$NULLCMD=cat`. Corrigido com `: > "$CALLLOG"` durante medição. O shebang do wrapper é `#!/usr/bin/env bash`, não zsh.

---

### R-8b / 8dot3 — D1 write-scope (ADS e nome curto)

Payload base: `{"tool_name":"Write","tool_input":{"file_path":"<vetor>"},"agent_id":"claude"}`

| Vetor (file_path) | rc PRE | rc FIX | Esperado |
|---|---|---|---|
| `$root/trackfw.yaml::$DATA` | 0 | 2 | PASS |
| `$root/trackfw.yaml:` | 0 | 2 | PASS |
| `$root/trackfw.yaml:x:$DATA` | 0 | 2 | PASS |
| `$root/trackfw.yaml. ::$DATA` | 0 | 2 | PASS |
| `$root/TRACKF~1.YAM` | 0 | 2 | PASS |
| `$root/TRACKF~1.YAM.` | 0 | 2 | PASS |
| `$root/trackf~1.yam::$DATA` | 0 | 2 | PASS |
| `$root/TRACKF~10.YAM` (vetor defensivo; > 8 chars não é short name válido) | 0 | 2 | PASS — regex `~[0-9]+` captura igualmente |
| `$root/trackfw.yaml.bak` (legítimo) | 0 | 0 | PASS (não bloqueado) |
| `$root/tracKfw.yaml` (KELVIN SIGN U+212A→k) | 2 | 2 | PASS — EqualFold captura; APFS mesmo inode |
| `proj\trackfw.yaml` (backslash POSIX, via Python payload) | 2 | 2 | PASS — `hasTrackfwYAMLComponent` split em `\` |
| `$root/Ｔrackfw.yaml` (fullwidth T, U+FF34) | 0 | 0 | PASS — APFS não resolve fullwidth; arquivo distinto; não é ataque |

**KELVIN SIGN:** `strings.EqualFold("tracKfw.yaml", "trackfw.yaml")` = true. APFS resolve ao mesmo inode (medido via Python `os.path.samefile`). Ambas as versões bloqueiam.

**LONG S (U+017F):** `trackfw.yaml` não contém `s`, portanto LONG S não é aplicável ao D1. É capturado via `pathEqual(EqualFold)` no D6 (`.claude/ſettings.json`).

**Fullwidth:** EqualFold não faz normalização de compatibilidade Unicode; APFS não resolve fullwidth para ASCII → arquivo distinto → não é caminho de ataque real.

**R-8dot3-HASH (residual):** o regex cobre `TRACKF~N.YAM`. No Windows, quando há muitas colisões, o OS pode gerar hashed short names como `TRACKFBF~1.YAM`. Sem teste em Windows; declarar como residual.

**Nota darwin vs. Windows:** testes de ADS (`::$DATA`) e trailing-dot em APFS provam que o matcher foi alterado (rc muda de 0 para 2). Não provam que o OS criaria arquivos com esses nomes — em APFS não cria. O valor real desses vetores é Windows-only.

---

### D6 — caminhos congelados: variantes normalizadas

Projeto com `frozen_paths: [.claude/settings.json]`.
Payload base: `{"tool_name":"Write","tool_input":{"file_path":"$root/<vetor>"},"agent_id":"claude"}`

| Vetor (sufixo relativo ao root) | rc PRE | rc FIX | Esperado |
|---|---|---|---|
| `.claude/settings.json` (baseline) | 2 | 2 | PASS |
| `.claude/settings.json.` | 0 | 2 | PASS |
| `.claude/settings.json ` | 0 | 2 | PASS |
| `.claude/settings.json::$DATA` | 0 | 2 | PASS |
| `.claude./settings.json` | 0 | 2 | PASS |
| `.claude/ſettings.json` (LONG S U+017F→s, APFS mesmo inode) | 2 | 2 | PASS — `pathEqual(EqualFold)` captura |
| Sessão principal (sem agent_id) | 0 | 0 | PASS (sessão principal não bloqueada) |

**Windows — por leitura de código (VM não ligada):**
`normalizePathComponents` preserva o volume via `filepath.VolumeName("C:")` = `"C:"` (não processado por `normalizePathComponent`). `resolveWriteScopePath` termina com `filepath.Clean` que converte `/` para `\` no Windows. Portanto, quando `normalizePathComponents` recebe `resolved`, o caminho já usa separadores nativos e `FieldsFunc` divide corretamente. Sem residual de caminho misto. D6 não tem winskips nos testes do ML-2C.

---

### D5 — lease no ancestral restringe projeto aninhado

Leases com `version:1, agent_type:claude, expires: +24h`.
Payload: `{"tool_name":"Write","tool_input":{"file_path":"<alvo>"},"agent_id":"claude","agent_type":"claude"}`

| Vetor | Configuração | rc PRE | rc FIX | Esperado |
|---|---|---|---|---|
| Alvo em inner/src; inner lease permite inner/src; outer lease permite só outer/src | 2 yamls aninhados; 2 leases | 0 | 2 | PASS |
| Alvo em outer/src (dentro do allow outer) | idem | 0 | 0 | PASS (counter-arm) |
| Sem lease em nenhum ancestral | 2 yamls; sem leases | 0 | 0 | PASS |
| Lease inválido no outer, alvo em inner | lease sem campo `expires` | 0 | 2 | PASS |
| Monorepo: subagente em pkg-b, alvo em pkg-b/src (fora do allow da raiz) | raiz + pkg-b yamls; lease na raiz | 0 | 2 | PASS |
| Monorepo: sessão principal escreve em pkg-b/src | sem agent_id | 0 | 0 | SEM REGRESSÃO |
| D3 counter-arm: yaml regular, alvo benigno sem lease | yaml regular; sem leases | 0 | 0 | PASS |

**Output do multi-lease (FIX):**
```
denied: ".../outer/inner/src/x.txt" is not in the allow list
        of lease .../outer/.trackfw/leases/claude.json for "claude"
rc=2
```
Nota: mktemp cria em `/var/folders/...`; `resolveWriteScopePath` resolve via `EvalSymlinks` para `/private/var/...` tanto o target quanto os allow entries. Matching funciona.

**D3 em `collectLeaseRoots` (redundância defensiva):** `buildFrozenList` (passo 9) já caminha os ancestrais com D3. A adição de D3 em `collectLeaseRoots` (passo 11b) é defensiva em profundidade. O PRE já bloqueava D5-inválido-ancestor com rc=2 (via `buildFrozenList`), não via `collectLeaseRoots`.

---

### Regressões

| Vetor | rc FIX | Esperado |
|---|---|---|
| Sessão principal escreve livremente (sem agent_id) | 0 | SEM REGRESSÃO |
| Worktree prunable em `/private/tmp/trackfw-toml-escape` | N/A | SEM IMPACTO — não é nested na árvore principal |

---

## Residuais declarados

| ID | Descrição | Severidade | Ação recomendada |
|---|---|---|---|
| R-8dot3-HASH | 8dot3 hashed short names (`TRACKFBF~1.YAM` etc.) não cobertos pelo regex | LOW | Declarar no ADR-2026-11-24 |
| D3-FP | D3 bloqueia todas as sessões quando yaml não regular existe | INFO/aceito | ADR D3 aceita fail-closed |
| R-1 | Bash/shell bypassa write-scope (pre-existente) | MÉDIO | Pre-declarado no Wave 0 |
| R-6 | OpenCode lowercase `write` bypassa write-scope (pre-existente) | MÉDIO | Pre-declarado no Wave 0 |

---

## git diff --cached --stat

```
(empty)
```

## git status --porcelain

```
?? docs/seguranca/2026-11-24-red-team-ml-2c-write-scope.md
?? docs/seguranca/2026-11-24-revisao-ml-2c-write-scope.md
```

(segundo arquivo é do hefesto-tf em paralelo; hades-tf não modificou código nem adicionou ao staging)

---

## Propostas de nota para o vault (a registrar pelo arquiteto)

1. **zsh NULLCMD hang em bare redirect:** `> file` em zsh ativa `$NULLCMD=cat`, pendurando o processo em espera de stdin. Fix: `: > file`. Aplicável a qualquer script de teste que rode em zsh. (Ver nota existente em MEMORY.md.)

2. **darwin prova só o matcher, não o OS:** testes de ADS (`::$DATA`) e trailing-dot em APFS provam que o guard normaliza corretamente o payload (rc muda de 0 para 2), mas os nomes com ADS/trailing-dot não existem como arquivos distintos em APFS. O valor real é Windows-only. Notar no ADR-2026-11-24 que esses vetores testam o parser de caminho, não o filesystem.
