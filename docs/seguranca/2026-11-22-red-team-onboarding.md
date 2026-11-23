# Red Team — ML-4B — Onboarding Verificável

**REQ:** REQ-2026-11-22  
**Branch:** fix/onboarding-verificavel  
**HEAD:** e6694c06  
**Data:** 2026-11-22  
**Revisor:** Hades (security)  
**Veredito:** REPROVA — um achado BLOQUEANTE em escopo (F-9); F-5 pré-existente ao PR (guard não foi tocado por esta branch), escalado ao write-scope guard owner

---

## Incidente de Execução no Repositório Real

Durante o Target 1 (D2), um `trackfw init --brownfield < /dev/null` foi executado acidentalmente
com cwd no repositório real (ambiente `HOME` não isolado nessa chamada específica).

**Verificação pós-incidente:**

```bash
git -C <repo> status --porcelain
# ?? docs/qualidade/2026-11-22-onboarding-review.md
# ?? docs/seguranca/2026-11-22-red-team-onboarding.md

find ~/.claude/agents/ -newer ~/.claude/settings.json -type f
# (vazio — nenhum arquivo de agente modificado hoje)

git diff HEAD -- .claude/settings.json
# (vazio — conteúdo idêntico ao commit)
```

**Arquivos adjacentes (mtime nesta sessão):**

| Arquivo | Último mtime | Avaliação |
|---|---|---|
| `~/.gemini/settings.json` | Nov 21 17:53 (ontem) | Anterior à sessão; sem relação |
| `~/.codex/logs_2.sqlite-wal` | Nov 22 22:21 (hoje) | WAL SQLite atualiza em leituras; causação indeterminada |
| `~/.codex/models_cache.json` | Nov 22 22:21 (hoje) | Idem |
| `~/.trackfw/integrations-manifest.json` | Nov 22 20:32 (hoje) | Fora das variáveis isoladas; causação indeterminada |

**Dano:** nenhum ao repositório. O D2 guard detectou o `trackfw.yaml` existente → `skipGovernance=true` → governança
não escrita. Os scripts já existiam; merge foi no-op. Nenhum arquivo tracked foi modificado.
`~/.gemini/settings.json` não alterado nesta sessão. Arquivos `~/.codex` e `~/.trackfw` mostram mtime
desta noite mas WAL SQLite atualiza em qualquer transação; acesso causado por esta sessão não confirmado
nem descartado.

---

## Escopo

Falsificar quatro propriedades do contrato (ADR-2026-11-22):

| Alvo | Descrição |
|---|---|
| D1 TTY | `--ai-tools` e `--brownfield` chegam ao caminho TTY sem wizard |
| D2 | Burlas da detecção "arquivo existe" (symlink, dir, .yml, subdir, race condition) |
| D2a | Grafias YAML que contornam o detector de chaves (`presentTopLevelKeys`) |
| D4 | Edições no README/site que mantêm o teste verde mas quebram a UX |

Ambiente: `mktemp -d` isolado, `HOME`/`USERPROFILE`/`CODEX_HOME`/`XDG_CONFIG_HOME` isolados, bash.

---

## Resultados por Alvo

### D1 — Caminho TTY com flags explícitas

**Método:** driver Python `pty.fork()`. Flags `--scope global --identity-preset none`.

#### D1-1: `--ai-tools claude` no caminho TTY

```
TTY_RESULT: SUCCESS mode=aitools   (12 agentes instalados)
```

**CONFORME.** `WithHideFunc` ocultou o grupo de wizard; flag lida corretamente. Encerra residual 9.

#### D1-2: `--brownfield` no caminho TTY — novo projeto

```
TTY_RESULT: SUCCESS mode=brownfield
# trackfw.yaml: governance_mode: lenient / lenient_until: 2026-12-22
```

**CONFORME.** Governance lenient escrita em projeto novo via caminho TTY. Encerra residual 10.

---

### D2 — Detecção "arquivo existe" em `applyBrownfieldOrWarn`

Implementação usa `os.Lstat` (detecta symlinks sem seguir).

#### D2-1 e D2-2: Symlink (F-1) e Broken Symlink (F-2)

```bash
ln -s "$REAL_YAML" "$T/trackfw.yaml"
bin/trackfw init --brownfield < /dev/null
# "governance keys not modified" (D2 guard OK)
# "refusing reparse-point path .../trackfw.yaml (mode Lrwxr-xr-x)" (scaffold pre-check)
# Error: scaffold pre-check: refusing write to ...
# rc=1; nenhum agente instalado
```

**ATRITO (rc=1, não rc=0).** D2 guard ativa corretamente; segundo controle independente
(scaffold pre-check para reparse-points) aborta com rc=1. Fail-closed. Contrato rc=0 violado.

#### D2-3: Diretório com nome `trackfw.yaml` (F-3)

```bash
mkdir "$T/trackfw.yaml"
bin/trackfw init --brownfield < /dev/null
# "governance keys not modified" (D2 guard OK)
# ✓ docs/adr, ... (scaffold cria subdiretórios)
# Error: reading trackfw.yaml: read trackfw.yaml: is a directory
# rc=1; nenhum agente instalado
```

**ATRITO (rc=1).** D2 guard ativa; scaffold cria diretórios; leitura do dir-como-arquivo falha.
Fail-closed.

#### D2-4: `trackfw.yml` (extensão incorreta) (F-4)

```bash
cp existing.yaml "$T/trackfw.yml"
bin/trackfw init --brownfield < /dev/null
# rc=0 — cria trackfw.yaml novo (governance escrita)
```

**LOW.** `applyBrownfieldOrWarn` só verifica `trackfw.yaml`; config reader também ignora `.yml`.
Ambos concordam (nenhum vê o `.yml`). Não é downgrade de governança; a config em `.yml` nunca
foi carregada. Usuário perde configuração existente sem aviso. Declarado no Wave 0 como residual.

#### D2-5: `cwd` em subdiretório do projeto (F-5) — PRÉ-EXISTENTE, escalado

**Status:** Esta branch não modificou `internal/guard/` (confirmado via `git log origin/main..HEAD -- internal/guard/` = vazio). O mecanismo existe no main desde antes desta REQ; a causa raiz (write-scope guard) é distinta da causa desta REQ (init subdir). Reportado para escalação; não conta como REPROVA desta branch.

**Passo 1 — bolha governance:**

```bash
cd "$PARENT/inner"; bin/trackfw init --brownfield < /dev/null
# governance_mode: lenient em inner/trackfw.yaml

bin/trackfw validate   # from inner/
# [LENIENT MODE] Governance violations treated as warnings until 2026-12-22

bin/trackfw validate   # from parent/
# ✓ No violations found.
```

**Passo 2 — write-scope bypass (medido nesta sessão):**

Discriminating test: frozen_paths inclui `.claude/settings.json` em `parent/trackfw.yaml`.

```bash
# Baseline: Write .claude/settings.json → DENIED (rc=2, frozen path)
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},"agent_id":"evil"}' \
  "$PARENT" | bin/trackfw guard write-scope
# denied: ".claude/settings.json" is a frozen path → rc=2

# Passo C.1 — Write .claude/trackfw.yaml (rc=0; não está em frozen_paths)
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/trackfw.yaml"},"agent_id":"evil"}' \
  "$PARENT" | bin/trackfw guard write-scope
# (vazio) → rc=0

# Simulando o write bem-sucedido; agora .claude/trackfw.yaml existe
echo "adr_dirs: [docs/adr]" > "$PARENT/.claude/trackfw.yaml"

# Passo C.2 — Write .claude/settings.json novamente
printf '{"tool_name":"Write","tool_input":{"file_path":"%s/.claude/settings.json"},"agent_id":"evil"}' \
  "$PARENT" | bin/trackfw guard write-scope
# (vazio) → rc=0  ← bypass: .claude/trackfw.yaml re-enraíza o guard em .claude/
```

**Causa raiz:** `findProjectRoot` em `writescope.go:504` caminha a partir do caminho do
arquivo-alvo e retorna o PRIMEIRO ancestral com `trackfw.yaml`. Uma vez que `.claude/trackfw.yaml`
exista, writes para `.claude/settings.json` usam a frozen list de `.claude/trackfw.yaml` (que tem
`write_scope` vazio). Os `frozen_paths` do parent são ignorados.

**Impacto medido:** um subagente pode remover seus próprios hooks Claude Code em dois Write calls —
`Write .claude/trackfw.yaml` (rc=0) e `Write .claude/settings.json` (rc=0 após o primeiro).

**ADR D3 (ADR-2026-10-23)** documenta o design: "a raiz que vale é a do projeto do alvo (o ancestral
com `trackfw.yaml`)". A vulnerabilidade é uma consequência não declarada desse contrato.

**Escalação:** guard owner (`internal/guard/writescope.go`) — `findProjectRoot` ou lógica de freeze
de nested `trackfw.yaml`. Precisa REQ própria (causa diferente da desta REQ).

#### D2-6: Empty file e comments-only file

```bash
touch trackfw.yaml      # rc=0, D2 guard correto
echo "# comment" > trackfw.yaml  # rc=0, D2 guard correto
```

**CONFORME.**

#### D2-7: Análise de race condition (TOCTOU) — por leitura

Sequência: `os.Lstat` → `skipGovernance` decision → `writeTrackfwConfig`.

No sentido de downgrade (strict → lenient): `BrownfieldMode=true` só é definido quando `--brownfield`
é passado como flag, antes do `Lstat`. A janela de race não afeta a decisão de governance mode.

No sentido de DoS: um atacante pode criar `trackfw.yaml` entre o Lstat e o write, forçando merge.
O merge preserva chaves existentes sem downgrade.

**CONFORME por análise de leitura** (race não explorável para downgrade de governança).

---

### D2a — Grafias YAML em `presentTopLevelKeys` e `normalizeYAMLTopLevelKey`

Duas funções em uso: `presentTopLevelKeys` (yaml.Node — `writeTrackfwConfig`) e
`normalizeYAMLTopLevelKey` (text — `generateLefthookHook`).

| Grafia | Função | Resultado |
|---|---|---|
| Chave com aspas duplas: `"adr_dirs":` | presentTopLevelKeys yaml.Node | CONFORME |
| Chave com aspas simples: `'adr_dirs':` | presentTopLevelKeys yaml.Node | CONFORME |
| Escape YAML unicode: `"adr_dirs":` | presentTopLevelKeys yaml.Node decodifica | CONFORME |
| Mapeamento flow: `{adr_dirs: [custom], wip_limit: 7}` | presentTopLevelKeys: detectado; validate usa `custom/adr` | CONFORME |
| BOM primeira linha | Corrigido ML-1C/1D | CONFORME |
| BOM mid-file: `\xef\xbb\xbf adr_dirs:` | yaml.Node retorna `﻿adr_dirs` ≠ `adr_dirs` → duplicado (F-6) | LOW |
| Multi-doc com `---` | `hasMultipleDocuments` detecta; `osExit(1)` | CONFORME |
| Multi-doc com `...` | `hasMultipleDocuments` não detecta; yaml.v3 lê 1º doc | INFO (F-7) |
| Tab em valor | YAML válido; yaml.Node detecta | CONFORME |
| Tab como indentação (malformed) | Parsing YAML falha antes de `applyBrownfieldOrWarn`; rc=1, 0 agentes | ATRITO (fail-closed, antes do D2 guard) |
| Anchor/merge-key `<<: *base` | `presentTopLevelKeys` vê `<<`, não as chaves expandidas → appends defaults (F-9) | BLOQUEANTE |
| Lefthook escape: `"pre-commit":` | `normalizeYAMLTopLevelKey` não decodifica → guard não inserido (F-10) | LOW |

**Detalhe — BOM mid-file (F-6):**
O yaml.v3 não remove BOM de chaves não-iniciais. Chave lida como `﻿adr_dirs` ≠ `adr_dirs` →
não detectada → chave duplicada adicionada. `config.Load()` também falha: escritor e leitor
concordam (ambos erram). Pré-existente; severidade: LOW.

**Detalhe — Anchor/merge-key bypass (F-9) — BLOQUEANTE:**

```yaml
# Input trackfw.yaml com anchor:
_govern: &govern
  require_req_in_commit: true  ← controle de governança explícito
  wip_limit: 3

<<: *govern
adr_dirs: [docs/adr]
req_dir: docs/req
roadmap_dir: docs/roadmaps
```

```
# Após init --brownfield (apende defaults ausentes):
  require_req_in_commit: true      ← merged (nó anchor)
require_req_in_commit: false       ← APPENDED pelo init (template default)
wip_limit: 1                       ← APPENDED (template default; override de wip_limit: 3)
```

Medição via `config.Load()` (zz_ test em `internal/config`):

```
RequireReqInCommit = false  (anchor: true, explicit: false)
WipLimit = 1                (anchor: 3, explicit: 1)
```

Resultado via `trackfw validate` (2 wip roadmaps):

```
⚠  2 roadmaps in wip/ (limit: 1) — confirma wip_limit=1
```

**Causa raiz:** `presentTopLevelKeys` itera o `MappingNode` via `.Value`. Para o merge key `<<`, o
`.Value` é `<<` — as chaves expandidas não são enumeradas. Portanto:
- `require_req_in_commit` não entra no mapa de chaves detectadas
- O template default `require_req_in_commit: false` é adicionado como chave explícita
- O yaml.v3 resolve chave explícita > chave merged → `config.Load()` lê `false`

**Impacto:** um admin que usa YAML anchors para compor `require_req_in_commit: true`, `wip_limit: N`,
ou `rules.*: error` tem esses controles silenciosamente revertidos ao default do template em qualquer
re-execução de `trackfw init --brownfield`. O D2 warning `"governance keys not modified"` é
enganoso: as chaves *de governança* não foram modificadas, mas as chaves derivadas via anchor foram
sobrepostas.

**Severidade: BLOQUEANTE.** `require_req_in_commit: false` desabilita silenciosamente a validação
de commits — `RequireReqInCommit` é consumido em scaffold.go (linhas 154, 159, 1089, 1826) para
gerar ou suprimir o hook `commit-msg`. Downgrade confirmado também para `wip_limit` (anchor: 3 →
explicit: 1 medido via `trackfw validate`). `rules.*` não medido explicitamente.

**Correção sugerida:** `presentTopLevelKeys` deve expandir merge keys (`<<`) antes de retornar o
mapa. Para `<<: *anchor`, expandir o alias e adicionar as chaves ao mapa. Para `<<: [*a, *b]`,
expandir todos os aliases. Pode ser feito iterando `node.Content[i]` onde `Content[i-1].Value == "<<"`.

**Detalhe — Lefthook escape (F-10):**

```
# Medido via zz_ test em internal/generators:
"pre-commit":           → normalizeYAMLTopLevelKey → strips quotes → "pre-commit" → WIRED ✓
"pre-commit":      → normalizeYAMLTopLevelKey → strips quotes → "pre-commit" ≠ "pre-commit"
                        → inPreCommit never true → "no commands: section" warning → NOT WIRED ✗
```

`presentTopLevelKeys` (yaml.Node) detecta corretamente o `pre-commit` block; mas `normalizeYAMLTopLevelKey`
(text-based) não decodifica o escape → `inPreCommit` nunca é verdadeiro → guard hook não inserido.
Resultado: warning enganoso "pre-commit block has no 'commands:' section". Severidade: LOW.
**Nota de alcançabilidade:** F-10 só é atingível pelo wizard TTY (que pergunta hooks e chama
`generateLefthookHook`). Não existe flag `--hooks lefthook` — sem TTY, `generateLefthookHook`
não é chamado.

---

### D4 — Teste de pins de documentação

#### D4-1: Pin do bloco de comandos no README

**CONFORME.** `findQuickStartRun` exige bloco contíguo exato (5 linhas, posição-agnóstica mas
estruturalmente rígida).

#### D4-2: Pin do site guide — execução do bypass (F-8)

Localização de "2 warning(s)" no site guide:
- Linha 62: `...exits with rc=0 and prints \`2 warning(s)\`` (prosa)
- Linha 71: `2 warning(s)` (dentro de código fence — saída esperada do validate)
- Linha 323: `...with \`2 warning(s)\`` (referência posterior)

Teste com `zz_d4_redteam_test.go` (criado e deletado):

```go
modified := strings.ReplaceAll(original, "2 warning(s)", "<!-- 2 warning(s) -->")
blockFound := findQuickStartRun(modifiedLines) >= 0   // true (bloco intacto)
outputStringFound := strings.Contains(modified, "2 warning(s)")  // true (dentro de <!-- -->)
// → PASS
```

**PASS confirmado.** O check 2 (`strings.Contains`) só verifica presença em qualquer lugar do arquivo.
Após substituição, "2 warning(s)" sobrevive dentro de `<!-- 2 warning(s) -->` em três locais —
o test passa enquanto o código fence na linha 71 agora exibiria `<!-- 2 warning(s) -->` literalmente
ao usuário em vez da saída esperada.

Bypass mais direto (não testado explicitamente): remover apenas as linhas 64-72 (o código fence
de saída) → linha 62 e 323 ainda contêm "2 warning(s)" → check 2 passa → `findQuickStartRun`
continua encontrando o bloco de comandos (linhas 54-59) → test passa; usuário vê comandos sem
saída esperada.

**Severidade: LOW (F-8).** Regressão do tipo ML-2B (output errado após quick start) passaria
no test se o texto "2 warning(s)" sobrevivesse em prosa. Por aritmética: linha 62 e linha 323
satisfazem `strings.Contains` por conta própria — o bloco de saída (linha 71) pode ser removido
integralmente sem fazer o check falhar. Contra-braço `_CounterArm_SiteGuide` protege contra
edição dentro do bloco de comandos (check 1) mas não contra edição do bloco de saída mantendo
o texto âncora em qualquer outro local.

---

## Sumário de Achados

| ID | Severidade | Contrato | Descrição |
|---|---|---|---|
| F-1 | ATRITO | D2 rc=0 | Symlink → rc=1; scaffold pre-check "reparse-point"; nenhum agente instalado |
| F-2 | ATRITO | D2 rc=0 | Broken symlink → rc=1 (mesmo padrão) |
| F-3 | ATRITO | D2 rc=0 | Directory → rc=1; scaffold cria dirs; leitura dir-como-arquivo falha |
| F-4 | LOW | D2 | `trackfw.yml` não detectado; config original perdida sem aviso |
| F-5 | BLOQUEANTE (pré-existente, fora de escopo desta branch) | D2 | Subdir init cria `inner/trackfw.yaml` → `findProjectRoot` re-enraíza write-scope guard → qualquer `frozen_paths` bypassado em dois Write calls; inclui `.claude/settings.json`; guard owner, REQ separada |
| F-6 | LOW | D2a | BOM mid-file: `﻿adr_dirs` não detectado; config reader também falha; sem downgrade |
| F-7 | INFO | D2a | `...` multi-doc não detectado; yaml.v3 lê 1º doc corretamente |
| F-8 | LOW | D4 | Site guide check 2 (`strings.Contains`) laxo; saída errada no código fence passa se "2 warning(s)" sobrevive em prosa (linhas 62/323 satisfazem por conta própria) |
| F-9 | BLOQUEANTE | D2a | Anchor/merge-key `<<: *base`: chaves expandidas invisíveis a `presentTopLevelKeys`; `require_req_in_commit: true` → `false` (confirmado via `config.Load()`); controle de governança silenciosamente desabilitado |
| F-10 | LOW | D2a | Lefthook `"pre-commit":`: `normalizeYAMLTopLevelKey` não decodifica escape → guard hook não inserido; warning enganoso |

---

## Achados Bloqueantes — Requer Ação antes de Merge

### F-9 — Governance control downgrade via YAML anchor/merge-key

**Mecanismo:** `presentTopLevelKeys` enumera nós de um `MappingNode` via `.Value`. O merge key `<<`
tem `.Value == "<<"` — as chaves que ele injeta (do alias) não aparecem na iteração. Init appends
defaults ausentes para essas chaves invisíveis; yaml.v3 resolve chave explícita > merged → controle
revertido.

**Downgrade confirmado (config.Load):**
```
require_req_in_commit: true (anchor) → false (explicit, appended by init)
```

**Fix:** `presentTopLevelKeys` deve expandir merge keys antes de retornar o mapa. Iterando
`root.Content`: quando `Content[i].Value == "<<"`, seguir o alias e adicionar cada chave
expandida ao mapa.

---

## Achados Pré-Existentes — Escalação para Guard Owner

### F-5 — Write-scope bypass via nested trackfw.yaml

**Esta branch não introduziu este defeito.** `git log origin/main..HEAD -- internal/guard/` = vazio.

**Mecanismo:** `findProjectRoot` (writescope.go:504) usa o primeiro ancestral com `trackfw.yaml`.
ADR D3 (ADR-2026-10-23) documenta esse design. Consequência não declarada: um subagente pode criar
`<subdir>/trackfw.yaml` (não está em nenhum `frozen_paths`) para re-enraizar o guard e acessar
qualquer arquivo sob `<subdir>/`.

**Discriminating test medido nesta sessão:**

```bash
# parent/trackfw.yaml tem: write_scope.frozen_paths: [.claude/settings.json]

# 1. Write .claude/trackfw.yaml → rc=0 (não está em frozen_paths)
# 2. Write .claude/settings.json → rc=0 (re-enraizado em .claude/; .claude/trackfw.yaml vazio)
# ← .claude/settings.json bypassado em 2 writes
```

**Impacto:** um subagente pode remover seus próprios hooks Claude Code em dois Write calls.
Este defeito está em `internal/guard/writescope.go` → causa diferente, precisa REQ separada.

---

## Residuais Declarados pelo Red Team

**R-1 (estende residual #1 do Wave 0 — subdir governance):**
O Wave 0 declara "init cria yaml local em subdir" como aceito. Este red team adiciona: `config.Load()`
nos comandos `validate`/`status`/`req`/`roadmap` usa a config lenient local; o write-scope guard
pode ser bypassado (F-5). O residual precisaria ser re-nomeado para cobrir o efeito de segurança.

**R-2 (D4 site guide check laxa — não declarado no Wave 0):**
O check 2 (`strings.Contains`) garante apenas que a string existe em algum lugar do arquivo.
Correção trivial: usar busca ancorada próxima ao bloco de comandos.

---

## Fechamentos de Residuais do Wave 0

| Residual | Status | Evidência |
|---|---|---|
| Residual 9: D1 TTY `--ai-tools` | FECHADO | TTY_RESULT: SUCCESS; 12 agentes instalados |
| Residual 10: D1 TTY `--brownfield` | FECHADO | TTY_RESULT: SUCCESS; governance_mode: lenient escrita |

---

## Estado Final

```bash
git diff --cached --stat        # vazio
git status --porcelain          # ?? docs/qualidade/... ?? docs/seguranca/... (apenas este relatório)
pgrep -fl 'trackfw|test_tty'    # Chromium (trackfw serve board — processo pré-existente do usuário)
```

Nenhum `zz_` file remanescente. Nenhum processo trackfw ou test_tty em background.

---

## Notas para o Arquiteto

**Vault note recomendada:** `presentTopLevelKeys` não expande merge keys YAML (`<<: *anchor`);
chaves invisíveis recebem defaults do template em qualquer re-init; afeta `require_req_in_commit`,
`wip_limit`, `rules.*`.

**Roteamento dos achados:**
- F-9 (anchor bypass — BLOQUEANTE, em escopo): `internal/generators/scaffold.go` — `presentTopLevelKeys` deve expandir merge keys
- F-5 (write-scope bypass — pré-existente, escalação): `internal/guard/writescope.go` — `findProjectRoot` ou lógica de freeze de nested `trackfw.yaml`; ADR D3 precisa declarar o residual ou ser corrigido
