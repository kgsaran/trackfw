# Wave 0 — Enumeração e modelo de ameaça: CRLF de `python3` consumido por bash

> Produzido por: `hades-tf` · 2026-09-23  
> Roadmap: `ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md`  
> REQ: `REQ-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md`

---

## Seção 1 — Completude da enumeração

### Escopo varrido

| sítio | comando | resultado |
|---|---|---|
| `scripts/*.sh` | `find scripts -name '*.sh' -exec grep -l 'python3\|PY_BIN\|\$PYTHON\b' {} \;` | 32 arquivos |
| `.github/workflows/*.yml` | `grep -rln 'python3\|PY_BIN\|\$PYTHON\b' .github/workflows/*.yml` | 5 arquivos |
| `Makefile` | `grep -n 'python3\|PY_BIN\|\$PYTHON\b' Makefile` | 6 linhas |

**32 scripts encontrados — 2 são falsos positivos por comentário:**

```bash
grep -n 'python3' scripts/check-write-containment.sh
→ 41: # Portability: bash + grep/sed only. No python3, no mapfile with interpolated

grep -n 'python3' scripts/check-serve-address-parity.sh
→ 30: # python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
    (nenhuma invocação real no arquivo)
```

**População real: 30 scripts** com invocação real de Python.

---

### Fontes não-Python de CRLF

Examinado no corpus: `git`, binários `.exe` nativos invocados por bash, e arquivos com CRLF sendo lidos. Nenhum desses cria o mecanismo desta REQ — o problema é stdout do Python em modo texto, não conteúdo de arquivo. Comportamento de `git ls-files --eol` em Git Bash no Windows não foi medido neste corpus. Aceito como residual (Seção 5).

---

## Seção 2 — Classificação por sítio

### Critério (objetivamente verificável por terceiro)

Um sítio é categoria **(a) — DEFEITO** quando as três condições são simultaneamente verdadeiras:

1. **Captura:** bash recebe stdout de Python como dado — via `$(...)`, `cmd | while IFS= read -r`, `done < <(python3 ...)`, ou redirecionamento para arquivo lido com `while read`.
2. **Emissão de `\n`:** o Python emite pelo menos um `\n` nesse stream — por `print()`, `sys.stdout.write(s)` com `\n` em `s`, ou `writelines`.
3. **Sem normalização:** nada strip `\r` entre Python e a comparação/uso subsequente.

**Consequência:** se condição 2 falha — Python usa `sys.stdout.write(s)` sem `\n` em `s` — o sítio não é (a) independentemente das outras condições. Um `sys.stdout.write(hashlib.sha256(...).hexdigest())` não emite `\n` e o modo texto nunca tem oportunidade de inserir `\r`.

Categoria **(b)** = condições 1+2 verdadeiras, condição 3 falsa (normaliza). **Medido: 0 scripts.**

Categoria **(c)** = condição 1 falsa (não captura). Fora da população.

---

### Medição inicial do arquiteto: refutada em um ponto

Declaração: "normalizam \\r: 1 (check-gates-falsify.sh)".

`check-gates-falsify.sh` linha 20: `export PYTHONIOENCODING=utf-8`. Isso controla o **codec** de stdio, **não** a tradução de newline. A tradução `\n` → `\r\n` depende de `os.linesep` e do parâmetro `newline` do `io.TextIOWrapper` — independente de `PYTHONIOENCODING`:

```bash
$ python3 -c "import os; print(repr(os.linesep))"
'\n'  # macOS/Linux. Windows retorna '\r\n' mesmo com PYTHONIOENCODING=utf-8.
```

**Conclusão: 0 scripts em categoria (b), não 1.**

---

### Contagem final

| categoria | scripts | ativo em CI Windows | dormente CI (ubuntu) |
|---|---|---|---|
| **(a) DEFEITO** | **19** | **2** | 17 |
| **(b) normaliza** | **0** | — | — |
| **(c) fora da população** | **11** | — | — |
| **Total** | **30** | | |

---

### 2.1 Scripts (a) — ATIVOS em CI Windows

#### `scripts/check-no-literal-nul-in-source.sh` — OCORRÊNCIA 1 DA REQ

**Sítio principal: linhas 51-62 (função `_text_files_from_repo`) + linha 326 (consumo)**

```bash
_text_files_from_repo() {
  local root="$1"
  git -C "$root" ls-files --eol 2>/dev/null | python3 -c '
import sys
for line in sys.stdin:
    line = line.rstrip("\n")
    if "\t" not in line:
        continue
    attrs_part, path = line.rsplit("\t", 1)
    path = path.strip()
    if "text=" in attrs_part:
        print(path)         # <- print() em modo texto -> CRLF no Windows
'
}
```

Consumo no gate principal:

```bash
while IFS= read -r rel; do
  [[ -z "$rel" ]] && continue
  full_path="$REPO_ROOT/$rel"
  ...
  if [[ ! -f "$full_path" ]]; then       # "path\r" -> arquivo nao existe
    skipped=$((skipped + 1))
    ...
  fi
done < <(_text_files_from_repo "$REPO_ROOT")   # linha 326
```

Condição 1: `done < <(...)` — atendida. Condição 2: `print(path)` — atendida. Condição 3: nenhuma normalização — atendida.

Em Windows: `rel = "path\r"` → `[[ -f "$REPO_ROOT/path\r" ]]` → **FALSO** → todos os arquivos são pulados como "não-regular" → guarda de vacuidade (linha 329) reporta "0 arquivos examinados" e **exit 1**.

Medição da REQ (Ocorrência 1): 671 caminhos, 671 com `\r`, `[[ -f ]]` → 0.

**Sítio secundário: linha 312**

```bash
offsets=$(python3 - "$full_path" << 'PYEOF'
...
print(', '.join(offs[:5]) + more)
PYEOF
)
```

Captura via `$(...)` com `print()`. Em Windows: `offsets = "0, 12, ...\r"` — corromperia a mensagem de diagnóstico mas não o exit code.

**CI:** `quality.yml`, `runs-on: ubuntu-latest`. **Dormente.**

---

#### `scripts/run-gates-falsify-shard.sh:85`

```bash
$PY_BIN "$GEN" "$SCRIPT" "$WORKDIR" "$SHARD_COUNT" > "$WORKDIR/manifest.txt"
```

**Condição 2 confirmada em `scripts/gen-falsify-chunks.py:610`:**

```python
for l in label_lines:
    print(l)    # -> "\r\n" em texto no Windows
```

**Consumo (linhas 118-131):**

```bash
while IFS= read -r line; do
  [[ "$line" == "chunk=$SHARD_INDEX label="* ]] || continue
  expected="${line#chunk=$SHARD_INDEX label=}"
  if ! grep -qxF "$expected" "$ACTUAL_LABELS"; then   # linha 118 - exige linha exata
    echo "AUSENTE: $expected"
  fi
done < "$WORKDIR/manifest.txt"
```

`expected` = `"nome\r"` → `grep -qxF "nome\r" actual` → nunca casa → **AUSENTE**.

Linha 127 usa `grep -qF` (substring) — casa mesmo com `\r` → sobrevive.

**Plataforma:** `windows-census.yml`, job `censo`, `runs-on: windows-latest`. **ATIVO.**
**Prova:** run `35856380122` — 8/8 shards reprovados, 146 rótulos acusados ausentes, incluindo rótulos emitidos no mesmo run.

---

#### `scripts/check-gates-falsify.sh:3915,4431`

`check-gates-falsify.sh` não é invocado diretamente no Windows, mas `run-gates-falsify-shard.sh` usa `gen-falsify-chunks.py` para materializar chunks desse script — que rodam no Windows. Medição:

```bash
python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh /tmp/cgf 8 > /tmp/m.txt
grep -ln 'PROSE_PAYLOAD=\|T65_BIG_PAYLOAD=' /tmp/cgf/*.sh
```

Resultado:
```
/tmp/cgf/chunk_0.sh    <- PROSE_PAYLOAD (linha 3915 do script original)
/tmp/cgf/chunk_3.sh    <- T65_BIG_PAYLOAD (linha 4431 do script original)
```

Ambos os chunks rodam na VM `windows-latest`.

**Linha 3915:**
```bash
PROSE_PAYLOAD=$("$PY_BIN" -c '
...
print(json.dumps({"tool_input": {"command": cmd}}))
')
```

**Linha 4431:**
```bash
T65_BIG_PAYLOAD=$("$PY_BIN" -c "import json; print(json.dumps({...}))")
```

Condições 1+2+3 atendidas. Em Windows: `PROSE_PAYLOAD = '{"tool_input": ...}\r'` e `T65_BIG_PAYLOAD = '{"..."}\r'`. Passados como argumentos ao guard script. JSON aceita `\r` como whitespace após `}`, portanto parsing pode não falhar — mas qualquer comparação bash posterior `[[ "$PROSE_PAYLOAD" == '{"..."}'  ]]` falha. Impacto prático incerto; o sítio é (a) pelos critérios mecânicos.

**Plataforma:** chunks 0 e 3 rodam em `windows-latest` via `run-gates-falsify-shard.sh`. **ATIVO.**

---

### 2.2 Scripts (a) — dormentes em CI, ativos local Windows

Scripts invocados por `make parity-rest` ou `make parity-falsify` — sem guarda de plataforma no Makefile (evidência na Seção 2.5). Um desenvolvedor Windows rodando esses alvos encontra os defeitos abaixo.

#### `scripts/run-gates-falsify-parallel.sh:129`

```bash
$PY_BIN "$GEN" "$SCRIPT" "$WORKDIR" "$JOBS" > "$WORKDIR/manifest.txt"
```

Mesmo mecanismo: `gen-falsify-chunks.py:610` usa `print(l)`. Consumo: linhas 191-204 com `grep -qxF`.

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-falsify-shard-coverage.sh:68`

```bash
$PY_BIN "$ROOT_DIR/scripts/gen-falsify-chunks.py" "$SCRIPT" "$WORKDIR" "$SHARD_COUNT" > "$WORKDIR/manifest.txt"
```

Mesmo mecanismo. Consumo: linhas 97-115 com `grep -qxF`.

**CI:** `quality.yml:953`, `ubuntu-latest`. Dormente.

---

#### `scripts/check-barrier.sh` — 9 sítios

Padrão: `GATES_STATUS14=$(python3 -c "import json,sys; ... print(r[0] if r else 'MISSING')" "$BARRIER_STDOUT")` → `[[ "$GATES_STATUS14" == "not_evaluated" ]]`.

Em Windows: `"not_evaluated\r" == "not_evaluated"` → FALSO.

| linha | variável | uso |
|---|---|---|
| 989 | `GO_NORM11` | comparação de parity |
| 1198 | `GATES_STATUS14` | `[[ == "not_evaluated" ]]` |
| 1200 | `GATES_FAIL14` | `[[ == "$EXPECTED_NOT_COMMITTED" ]]` |
| 1207 | `GO_S14` | parity diff |
| 1227 | `GATES_STATUS15` | `[[ == "passed" ]]` |
| 1237 | `GO_S15` | parity diff |
| 1258 | `GATES_STATUS16` | `[[ == "passed" ]]` |
| 1265 | `GO_S16` | parity diff |
| 1290 | `GATES_STATUS17` | `[[ == "not_evaluated" ]]` |

**CI:** `make parity-rest`, `ubuntu-latest`. Dormente.

---

#### `scripts/check-roadmap-barrier-contract.sh` — 16 sítios

Padrão: `mls_status=$(doc_check_json ... | python3 -c "import json,sys; print(json.loads(sys.stdin.read()))")` → `[[ "$mls_status" == "pass" ]]`.

Sítios: linhas 127, 789, 827, 852, 925, 926, 1006, 1047, 1071, 1120, 1121, 1143, 1173, 1198, 1230, 1263.

```bash
grep -c 'python3 -c.*print\|python3.*print' scripts/check-roadmap-barrier-contract.sh
→ 16
```

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-update-parity.sh` — 17 sítios

Padrão: `count=$(python3 -c "import json,sys; ... print(...)")`.

Sítios: linhas 202, 251, 279, 320, 352, 377, 406, 448, 480, 534, 544, 586, 596, 631, 639, 689, 699.

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-manifest-version-gate.sh:77,130`

```bash
manifest_version=$(python3 -c "...print(...)")   # linha 77
NPM_VERSION=$(python3 -c "...print(...)")          # linha 130
```

Usadas em comparações de string. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-goreleaser-prerelease.sh:60`

```bash
result=$(python3 - "$yaml_file" <<'PYEOF'
...
print(str(v))
PYEOF
)
```

Usado em `[[ "$result" == "NO_RELEASE_BLOCK" ]]`. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-install-restriction.sh:209,284,286`

```bash
C3_LOCK_COUNT=$(python3 - "$NPM_LOCK" <<'PY'...PY)  # linha 209
MY_REL="$(realpath ... || python3 -c ...)"           # linhas 284/286
```

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-platform-matrix-parity.sh:93`

```bash
GORELEASER_SLUGS=$(python3 - "$GORELEASER_YAML" <<'PYEOF'...print(slug)...PYEOF)
```

Consumido via `while IFS= read -r slug; do ... done <<< "$GORELEASER_SLUGS"`. Em Windows: `slug` teria `\r`. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-wheel-filename.sh:53,88,163`

```bash
result=$(python3 - <<PYEOF...PYEOF)
```

Cada captura usada em `[[ "$result" == "..." ]]`. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-release-tag-parity.sh` — 7+ sítios via `json_field()`

`json_field()` (linha 428) chama `python3 - "$file" "$key" <<'PYEOF'...print(v)...PYEOF` e é invocada como `tags_object=$(json_field ...)`. Em Windows: `tags_object = "sha256_value\r"` → comparação falha.

Sítios de uso: linhas 940, 941, 942, 943, 944, 1274, 1279 (e mais: 1470, 1479, 1667, 1676).

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-thirdparty-parity.sh:165`

```bash
installed_sha256=$(python3 -c "import json,sys; print(json.load(open(sys.argv[1]))['entries'][...]['installed_sha256'])" ...)
if [[ "$installed_sha256" == "$INSTALLED_SHA256" ]]; then
```

`INSTALLED_SHA256` (linha 85) usa `sys.stdout.write()` sem `\n` — não tem `\r`. `installed_sha256` usa `print()` → tem `\r` em Windows → `"hash\r" == "hash"` → FALSO.

Linhas 82, 83, 85 do mesmo script usam `sys.stdout.write()` sem `\n` — condição 2 falha → **não são (a)**.

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-git-branch-guard-hook-schema.sh:181,301,327`

```bash
decode_out=$(printf '%s' "$out" | decode_shape)  # decode_shape() chama python3 "$DECODER"
```

Python escreve com `print()`. `decode_out` usado em comparações. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-channels-content.sh:455`

```bash
PY_VERSION=$(python3 -c "...print(str(Version('$VERSION')))..." 2>/dev/null || echo "$VERSION")
```

Usado em `pip download "trackfw==$PY_VERSION"`. Em Windows: `"8.0.0\r"` → URL inválida. **CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/check-serve-browser-security.sh:93`

```bash
ZONE_CMDLINE=$(python3 -c "...print(subprocess.list2cmdline(argv))...")
```

Condições 1+2+3 atendidas → **(a)**. Uso subsequente: `echo "$ZONE_CMDLINE" | grep -q '&'` — substring, casa mesmo com `\r`. Inerte para uso atual. **Se mudar para igualdade, vira falha ativa.**

**CI:** `ubuntu-latest`. Dormente.

---

#### `scripts/trackfw-attention-signal.sh:14,15`

```bash
TOOL=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "...print(d.get('tool_name',''))...")
MSG=$(echo "$INPUT" | PYTHONIOENCODING=utf-8 python3 -c "...print(...)...")
```

`PYTHONIOENCODING=utf-8` não suprime tradução de newline. Condições 1+2+3 → **(a)**. Não invocado por `make quality`. **Dormente em CI.** Ativo se hook disparar em Windows.

---

### 2.3 Scripts (c) — condição 1 falsa (não capturam stdout como dado bash)

| script | padrão de invocação |
|---|---|
| `check-agent-models-parity.sh:927,928` | `sys.stdout.write(hexdigest())` — condição 2 falha (sem `\n`); linhas 821,882,952,1012 usam `>/dev/null` |
| `check-agent-namespace-union.sh:154,535` | standalone; exit code only |
| `check-ci-workflow-binary-provenance.sh:17` | `exec python3 ...` — substitui o processo bash |
| `check-install-checksum.sh:133` | standalone |
| `check-install-version-pin.sh:425` | standalone |
| `check-output-encoding-declared.sh:147` | standalone |
| `check-parity-contract-coverage.sh:64` | standalone |
| `check-pr-closing-keyword.sh:208,336` | exit code only |
| `check-serve-api-file-security.sh:86` | standalone |
| `check-validate-rule-pins.sh` | standalone e exit code only |
| `smoke-integration-packages.sh:109` | define variável `PYTHON_BIN`; não executa Python |

**Nota sobre `check-thirdparty-parity.sh:82,83,85`:** capturam via `$(...)` mas condição 2 falha — `sys.stdout.write()` sem `\n`. São (c) por condição 2.

---

### 2.4 Reconciliação

19 (a) + 11 (c) = 30 ✓
Dos 19 (a): 2 ativos + 17 dormentes = 19 ✓

---

### 2.5 Workflows e Makefile — veredito por sítio

**`.github/workflows/windows-census.yml`** — único job `runs-on: windows-latest` relevante:

| arquivo:linha | conteúdo | veredito |
|---|---|---|
| `windows-census.yml:360` | `runs-on: windows-latest` | contexto |
| `windows-census.yml:404` | `run: scripts/run-gates-falsify-shard.sh` | plataforma de execução dos sítios (a) ativos |

**Outros 4 workflows** (`quality.yml`, `release.yml`, `pr-check.yml`, `pr-roadmap-check.yml`):

Todos os jobs relevantes têm `runs-on: ubuntu-latest`. Invocações de Python são autônomas ou exit code only — sem captura bash. **(c)** para todos.

**`Makefile` — 6 linhas com Python:**

| Makefile:linha | conteúdo | veredito |
|---|---|---|
| `parity-rest:40` | `scripts/check-barrier.sh` | invoca script (a) dormente — sem guarda OS |
| `parity-rest:43` | `scripts/check-update-parity.sh` | invoca script (a) dormente — sem guarda OS |
| `parity-rest:44` | `scripts/check-release-tag-parity.sh` | invoca script (a) dormente — sem guarda OS |
| `parity-rest:45` | `scripts/check-git-branch-guard-hook-schema.sh` | invoca script (a) dormente — sem guarda OS |
| demais | invocações autônomas de Python sem captura bash | **(c)** para o Makefile em si |

```bash
grep -n 'ifeq.*Windows\|OSTYPE\|uname' Makefile
# → (vazio) — sem guarda de plataforma
```

---

## Seção 3 — Modelo de ameaça

### O adversário: o implementador apressado

O argumento mais plausível para esvaziar esta Wave 0 sem quebrar regra escrita:

> "Todos os scripts exceto `run-gates-falsify-shard.sh` têm `runs-on: ubuntu-latest`. São POSIX-only. Basta corrigir o shard runner."

**O teste que desfaz esse argumento — três perguntas:**

**P1: há job `runs-on: windows-latest` que invoca o script?**

Apenas `run-gates-falsify-shard.sh` e os chunks de `check-gates-falsify.sh` têm job Windows direto. Os outros 17 dormentes não. Esta pergunta sozinha não basta.

**P2: o script é invocado por alvo Makefile sem guarda de plataforma?**

```bash
grep -n 'check-barrier\|check-update-parity\|check-release-tag\|check-git-branch-guard\|run-gates-falsify-parallel' Makefile | grep -v '#'
# → linhas 40, 43, 44, 45 do alvo parity-rest — sem ifeq/uname
```

Scripts invocados diretamente em qualquer plataforma. Não são POSIX-only pelo critério de uso.

**P3: a documentação do projeto declara que o script não suporta Windows?**

Não. Os scripts têm mecanismos explícitos de suporte a Windows: `PYTHONIOENCODING`, `py -3`, `PY_BIN` shim, comentários citando `cp1252`. A intenção de suporte Windows está presente.

**Conclusão:** para qualificar como POSIX-only, o script precisa P1=não, P2=não, P3=sim. Nenhum dos 17 dormentes passa no triplo teste. Escopo correto: **todos os 19 scripts (a)**.

### O segundo vetor: ponto único mal-escopo

O roadmap exige "ponto único, não `tr -d '\r'` espalhado". O implementador apressado pode aplicar o helper apenas nos 3 sítios manifest.txt — corrigindo o CI ativo e deixando 16 sítios de variável capturada intocados.

O gate ML-1B precisa cobrir as duas formas: `$PY_BIN ... > arquivo` lido com `grep -qxF`, e `$(python3 -c "... print(...)")` em comparação de igualdade.

---

## Seção 4 — Falsificação nas duas direções

### Superfície 1: `manifest.txt` (shard, parallel, coverage)

**Direção falso-negativo (defeito não capturado):**

| elemento | valor |
|---|---|
| Onde a sabotagem entra | `gen-falsify-chunks.py:610`: `print(l)` em modo texto |
| Gate que deveria pegar | ML-1B: detectar que novo `$PY_BIN ... > arquivo` lido com `grep -qxF` nasceu sem ponto único |
| Sinal observável | rótulo emitido aparece acusado AUSENTE no mesmo run |

**Direção falso-positivo — delimitador vs conteúdo:**

Os labels do manifesto têm a forma `chunk=N label=falsify/nome`. Nenhum contém `\r` como conteúdo de dados:

```bash
grep -P '\r' scripts/gen-falsify-chunks.py
# → (vazio)
```

`tr -d '\r'` e `sed 's/\r$//'` produzem resultado idêntico para este corpus. A distinção é obrigatória para o design:

- `tr -d '\r'` remove `\r` em **qualquer posição**
- `sed 's/\r$//'` remove apenas `\r` **terminal de linha** — o artefato de modo texto
- binary mode no Python (`sys.stdout.buffer.write(...)`) não produz o problema na origem

**Exemplo de `\r` como conteúdo (fora da população de stdout capturado):**

`scripts/check-roadmap-barrier-contract.sh:1091`, função `write_fixture_crlf`:

```python
data = sys.stdin.buffer.read().decode('utf-8')
data = data.replace('\r\n', '\n').replace('\n', '\r\n')
with open(sys.argv[1], 'wb') as f:
    f.write(data.encode('utf-8'))
```

Esta função escreve em arquivo (não em stdout capturado por bash) — está fora da população (a). Ela produz roadmaps com CRLF intencionais para verificar que o CLI lê corretamente arquivos CRLF (issue #216). Um helper que operasse a nível de leitura de arquivo (em vez de a nível de stdout capturado) destruiria essa verificação silenciosamente.

**Consequência para o ML-1A:** o ponto único deve operar entre Python e bash (stdout capturado), não sobre conteúdo de arquivo. `sed 's/\r$//'` ou binary mode — não `tr -d '\r'`.

---

### Superfície 2: variável bash capturada via `$(python3 -c "... print(...)")`

**Direção falso-negativo:**

| elemento | valor |
|---|---|
| Onde a sabotagem entra | qualquer `VAR=$(python3 -c "... print(x)")` em script rodado no Windows |
| Mecanismo | `print()` → `x\r\n` → bash captura `x\r` → `[[ "$VAR" == "x" ]]` → FALSO |
| Gate que deveria pegar | ML-1B: enumerar todos os `$(python3 -c "... print(...)")` — se condições 1+2+3 sem normalização → REPROVADO |

**Direção falso-positivo:**

Medição: dos sítios de variável capturada neste corpus, zero usam o valor capturado como dado que deva preservar `\r`. Os valores são hashes, versões semânticas, status JSON, contagens. A normalização `${VAR%$'\r'}` é safe para todos os sítios existentes.

**Conclusão:** `${VAR%$'\r'}` é a forma segura para variáveis capturadas. `tr -d '\r'` sobre stdout completo seria errado pelo mesmo motivo da Superfície 1.

---

## Seção 5 — Residual declarado

### 5.1 `check-gates-falsify.sh` — impacto prático incerto

Dois sítios confirmados (linhas 3915, 4431), chunk_0 e chunk_3 confirmados via medição. PROSE_PAYLOAD e T65_BIG_PAYLOAD com `\r` trailing: JSON aceita `\r` como whitespace, parsing pode não falhar. Declarado (a) pelos critérios mecânicos. Impacto prático: não reproduzido.

### 5.2 Fontes não-Python de CRLF

Comportamento de `git ls-files --eol` em Git Bash no Windows não medido. Binários `.exe` nativos não identificados no corpus. **Aceito como residual.**

### 5.3 `trackfw-attention-signal.sh`

Classificado (a) — fora de `make quality` e sem job CI. Registrado como risco prospectivo se hooks seguirem o mesmo padrão.

### 5.4 PYTHONUTF8=1 e Python -X utf8

UTF-8 mode muda o codec mas não o parâmetro `newline` do `io.TextIOWrapper`. Em Windows, `print()` em UTF-8 mode ainda emite `\r\n`. Não testado em VM Windows. **Aceito como residual** — confirmação vem do ML-2A.

---

## Apêndice — comandos de verificação

```bash
# Contar scripts com invocação de Python (32 total, 2 falsos positivos)
find scripts -name '*.sh' -exec grep -l 'python3\|PY_BIN\|\$PYTHON\b' {} \; | wc -l

# Confirmar ausência de normalização de \r
grep -rln 'tr.*-d.*\\r\|sed.*s/\\r\|dos2unix' scripts/*.sh
# -> (vazio)

# gen-falsify-chunks.py usa print() para labels
grep -n '^    print(l)' scripts/gen-falsify-chunks.py
# -> 610:     print(l)

# Sítios capturados em check-gates-falsify.sh
grep -n '=\$("\$PY_BIN"' scripts/check-gates-falsify.sh
# -> 3915: PROSE_PAYLOAD=...
# -> 4431: T65_BIG_PAYLOAD=...

# Chunks que contêm os sítios (medir antes de afirmar)
python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh /tmp/cgf 8 > /tmp/m.txt
grep -ln 'PROSE_PAYLOAD=\|T65_BIG_PAYLOAD=' /tmp/cgf/*.sh
# -> chunk_0.sh, chunk_3.sh

# make parity-rest sem guarda de plataforma
grep -n 'ifeq.*Windows\|OSTYPE\|uname' Makefile
# -> (vazio)

# Shard ativo em Windows
grep -n 'run-gates-falsify-shard\|runs-on.*windows' .github/workflows/windows-census.yml | head -5
# -> 360: runs-on: windows-latest
# -> 404: run: scripts/run-gates-falsify-shard.sh

# Consumo de _text_files_from_repo (Ocorrência 1 da REQ)
grep -n '_text_files_from_repo\|print(path)' scripts/check-no-literal-nul-in-source.sh
# -> 60:         print(path)
# -> 326: done < <(_text_files_from_repo "$REPO_ROOT")
```
