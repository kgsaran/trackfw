# Wave 0 — Modelo de Ameaça: extrator JSON do guard de branch

> Data: 2026-10-02 | Agente: hades-tf | ML: ML-0A
> Roadmap: `ROADMAP-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md`
> ADR: `docs/adr/ADR-2026-10-02-o-guard-de-branch-extrai-o-comando-do-payload-por-um-parser-json-de-verdade-e-falha-fechado-quando-nao-consegue.md`
> Issue: #507

Script medido: `scripts/trackfw-git-branch-guard.sh` em `3b2eff09`.
Confirmação de identidade:
```
$ git diff 3b2eff09 HEAD -- scripts/trackfw-git-branch-guard.sh
(saída vazia)
```

Ambiente sem `jq`: PATH curado com symlinks para `sed`, `awk`, `bash`, `cat`, `head`, `tr`, `wc`, `date` — sem `jq`. Confirmado: `command -v jq` no PATH curado retorna vazio.
Diretório de execução: projeto trackfw minimal (`fake_project/trackfw.yaml` presente).
Protótipo awk: cópia da guard com linhas 163-177 substituídas por extrator awk com `hex2dec()`.

---

## 1. Completude da enumeração

O roadmap declara duas formas de falha aberta no fallback por `sed`. Cada uma é confirmada por efeito. A medição encontrou mais duas.

### Forma A — `\n` não desescapado (multilinha JSON)

Payload — bytes `6f 69 5c 6e 67 69 74` = `oi\ngit` (os dois bytes `5c 6e`, não newline real):
```json
{"tool_name": "Bash", "tool_input": {"command": "echo oi\ngit push origin main"}}
```

```
+jq=2   -jq_sed=0   -jq_awk=2
```

sed extrai `echo oi\ngit push origin main` como um segmento único; `quote_aware_split` não quebra em `\n` literal; verbo = `echo` → rc=0.
awk decodifica `\n` → newline real → dois segmentos; linha 2 = `git push` → rc=2.

### Forma B — `\"` truncando a extração

```json
{"tool_name": "Bash", "tool_input": {"command": "echo \"hello\" && git push origin main"}}
```

```
+jq=2   -jq_sed=0   -jq_awk=2
```

sed extrai `echo \` (truncado no `\` antes do `"`): sem verbo git → rc=0.
awk decodifica `\"` → `"` → extrai `echo "hello" && git push origin main` → bloqueia.

### Forma C — `\t` não desescapado (NOVA)

```json
{"tool_input": {"command": "git\tpush origin main"}}
```

```
+jq=2   -jq_sed=0   -jq_awk=2
```

sed não decodifica `\t`; token = `git\tpush` (não `git`) → rc=0. jq e awk decodificam → `git` + `push` → bloqueiam.

### Forma D — `\u000a` (mesma causa que Forma A)

`json.dumps({"command":"echo oi\u000agit push"})` — `\u000a` = newline:
```
+jq=2   -jq_sed=0   -jq_awk=2
```
Mesmo mecanismo da Forma A.

### Caso NÃO-falha-aberta: `}` em valor de chave irmã

`}` em valor de chave antes de `command`: o padrão `[^}]*` do primeiro sed falha, mas o último fallback (sem restrição `[^}]*`) encontra `"command"` → não é falha aberta. Confirmado com p06 (rc=2 em todos os caminhos).

**Lista fechada de formas de falha aberta no sed:** Forma A (`\n`), B (`\"`), C (`\t`), D (`\u000a`).

---

## 2. Modelo de ameaça do extrator awk

Cada caso abaixo foi executado em três vias: `orig+jq / orig-nojq (sed) / awk-nojq`.

### 2.1 `RS=""` é modo parágrafo — REGRESSÃO vs sed

`RS=""` em awk usa modo parágrafo: linhas em branco separam registros. Se o JSON tiver linha em branco, o awk processa apenas o primeiro registro.

Payload com linha em branco entre `tool_name` e `tool_input`:
```json
{"tool_name": "Bash",

"tool_input": {"command": "git push origin main"}}
```

```
+jq=2   -jq_sed=2   -jq_awk=0   [ISSUE-AWK: regressão vs sed]
```

awk: registro 1 = `{"tool_name": "Bash",` — sem `command` → sai 0. sed: processa linha a linha, encontra `"command"` na linha 3 → rc=2.

**Restrição obrigatória para ML-1A:** não usar `RS=""`. Acumular em `{_acc = _acc $0 "\n"}` e processar em `END`.

### 2.2 Chave duplicada — divergência jq vs awk

Payload raw (json.dumps não suporta duplicatas):
```json
{"tool_input":{"command":"echo safe","command":"git push origin evil"}}
```

```
+jq=2   -jq_sed=2   -jq_awk=0   [ISSUE-AWK: first-wins vs last-wins]
```

jq usa last-wins → `git push origin evil` → bloqueia. sed greedy (`.*`) também encontra o segundo `"command"` → bloqueia. awk first-wins → `echo safe` → sai 0.

**Restrição obrigatória para ML-1A:** implementar last-wins OU negar em chave duplicada (extensão de D2).

### 2.3 `strtonum()` ausente em BWK awk (macOS)

```
$ awk 'BEGIN{print strtonum("0xff")}'
awk: calling undefined function strtonum
 source line number 1
rc=2
```

`strtonum()` é extensão gawk. O extrator deve usar `hex2dec()` portável, validada em macOS BWK awk `version 20200816`:

```awk
function hex2dec(h,   v,i,c,d) {
    h=tolower(h); v=0
    for(i=1;i<=length(h);i++){c=substr(h,i,1);d=index("0123456789abcdef",c)-1;if(d<0)return -1;v=v*16+d}
    return v
}
```

### 2.4 Prioridade de busca — protótipo diverge do ADR D1

ADR D1: `tool_input.command → flat command → tool_info.command_line → hook_input.command` (mesma prioridade de jq: `.tool_input.command // .command // .tool_info.command_line // .hook_input.command`).

Protótipo: `tool_input.command → tool_info.command_line → hook_input.command → flat command`. `flat command` é 2ª no jq/ADR, 4ª no protótipo.

Caso discriminante (derivado por inspeção — não medido com o guard diretamente, porque jq e protótipo divergem só quando tool_info.command_line existe E flat command é `git push`):
`{"command":"git push","tool_info":{"command_line":"echo safe"}}` → jq: flat command na 2ª posição → `git push` → bloqueia; protótipo: tool_info na 2ª posição → `echo safe` → sai 0.

**Restrição obrigatória para ML-1A:** implementar a ordem do ADR D1.

### 2.5 Escape desync (`\\` em valor de string irmão)

```json
{"tool_input": {"description": "x\\\\", "command": "git push origin main"}}
```
(description = `x\\` no JSON = um `\` decodificado)

```
+jq=2   -jq_sed=2   -jq_awk=2   [sem regressão]
```

O extrator awk processa `\\` como escaped-backslash e termina a string corretamente. Encontra `"command"` como chave separada. ✓

### 2.6 Backslash final em valor de comando (D2 não dispara indevidamente)

```json
{"tool_input": {"command": "echo foo\\\\"}}
```
(valor decodificado: `echo foo\`)

```
+jq=0   -jq_sed=0   -jq_awk=0   [sem falso positivo]
```

String válida terminando em `\`. D2 não dispara. Comando `echo foo\` é permitido. ✓

### 2.7 Chaves aninhadas (`tool_input` em sub-objeto)

```json
{"meta":{"tool_input":{"command":"echo safe"}},"tool_input":{"command":"git push origin main"}}
```

```
+jq=2   -jq_sed=2   -jq_awk=2   ✓
```

jq `.tool_input.command` extrai somente do nível raiz. awk e sed: idem. ✓

```json
{"meta":{"command":"echo safe"},"command":"git push origin main"}
```

```
+jq=2   -jq_sed=2   -jq_awk=2   ✓
```

### 2.8 `\u000d` (CR)

```json
{"tool_input": {"command": "echo oi\rgit push origin main"}}
```
(jq e awk decodificam `\u000d` → CR)

```
+jq=0   -jq_sed=0   -jq_awk=0
```

CR (`\r`) não é separador de comando em bash. `quote_aware_split` não quebra em `\r`. rc=0 correto. ✓

### 2.9 Literal `\\n` no argumento — sem falso positivo (AC3)

```json
{"tool_input": {"command": "echo \"a\\\\ngit push origin main\""}}
```
(decodificado: `echo "a\ngit push origin main"` — `\n` é backslash+n, não newline)

```
+jq=0   -jq_sed=0   -jq_awk=0   ✓
```

Nenhum extrator over-decoda `\\n` como newline. AC3 satisfeito. ✓

### 2.10 `\ud83d` (surrogate isolado)

```
+jq=2   -jq_sed=2   -jq_awk=2   ✓
```

jq falha → `|| true` → CMD_RAW vazio → sed fallback encontra literal → bloqueia. awk converte para marcador `\002` → bloqueia.

### 2.11 `\u0000` (NUL) — TODAS as formas falham abertas (resíduo pré-existente)

```
+jq=0   -jq_sed=0   -jq_awk=0
```

bash `$()` descarta NUL → `git pushorigin main` → `pushorigin` ≠ `push` → rc=0. Este limite é da shell, não do extrator. Pré-existente (não é regressão desta REQ).

---

## 3. Diferenças entre AWK: BWK/macOS, gawk, mawk

| recurso | BWK macOS `20200816` | gawk | mawk |
|---|---|---|---|
| `strtonum()` | ERRO rc=2 — medido | disponível | disponível (não medido) |
| `split(s, a, sep)` explícito | ✓ medido | ✓ | ✓ (não medido) |
| `printf "%c"` com número | ✓ medido | ✓ | ✓ (não medido) |
| `RS=""` parágrafo | ✓ — mas regressão em JSON com blank line (medido) | ✓ | ✓ (não medido) |
| `tolower`, `substr`, `index`, `length` | ✓ medido | ✓ | ✓ (não medido) |
| interval expressions `{n}` | ✓ medido | ✓ | ✓ (não medido) |

`command -v gawk mawk` retorna vazio nesta máquina. Colunas gawk/mawk derivadas de documentação POSIX awk — **não medidas diretamente**. ML-1A deve executar a tabela em CI Linux onde gawk está disponível.

**Nota sobre `/usr/bin/jq` no macOS:**
```
$ ls -l /usr/bin/jq
.rwxr-xr-x@ 2.2M root 24 Sep 04:10 /usr/bin/jq
```
macOS (14.x+) ships jq em `/usr/bin/jq`. O PATH curado dos testes de no-jq deve explicitamente excluir `/usr/bin`, caso contrário o braço sem-jq silenciosamente usa jq e não testa o fallback.

---

## 4. Custo de extração (macOS, `bash 5.3.20`, `awk version 20200816`)

| cenário | sed | awk | jq |
|---|---|---|---|
| payload típico (~80 bytes) | ~10 ms | ~10 ms | ~23 ms |
| 200 KB, fields pequenos, command de 21 bytes | ~4 ms | ~323 ms | ~4 ms |
| 200 KB, um value de 199 KB (AAAA...\ngit push) | ~7 ms | ~1 700 ms | ~4 ms |
| 200 KB, flat command (4 varreduras) | ~7 ms | ~6 800 ms | ~4 ms |

O custo quadrático do awk (`out = out c` por caractere) aparece em **valores longos**. Para payloads típicos (<1 KB): custo comparável ao sed.

Extrapolação MSYS2 — declarada como inferência, não medida: a nota do vault documenta latência de pipe ~90× maior no MSYS2 para throughput I/O; o awk é CPU-bound, então o fator MSYS2 é desconhecido. ML-1A pode aceitar o O(n²) e documentar o limite no ADR, ou usar concatenação por chunks.

---

## 5. Resíduo declarado

| # | item | tipo |
|---|---|---|
| R1 | `\u0000` em subcomando — todos os caminhos falham abertos (bash `$()` descarta NUL) | pré-existente; REQ separada se corrigido |
| R2 | Chave duplicada — restrição de ML-1A (last-wins ou deny) | implementação |
| R3 | `RS=""` regressão — restrição de ML-1A (acumulação em `END`) | implementação |
| R4 | `strtonum()` ausente em BWK awk — restrição de ML-1A (`hex2dec()`) | implementação |
| R5 | Prioridade diverge do ADR D1 — restrição de ML-1A | implementação |
| R6 | Custo quadrático em command longo (200 KB) | declarado no ADR |
| R7 | `\uXXXX` não-ASCII → marcador `\002` | declarado no ADR D1 |
| R8 | gawk/mawk não medidos nesta VM | CI |
| R9 | `/usr/bin/jq` em macOS — precondição de teste | implementação |

---

## Tabela de casos obrigatórios para ML-1A

Pré-condição do braço `-jq_awk`: `command -v jq` dentro do PATH curado deve retornar vazio. Razão: macOS ships `/usr/bin/jq`; PATH incluindo `/usr/bin` usa jq silenciosamente.

| id | payload (intenção) | +jq | -jq_awk | exp |
|---|---|---|---|---|
| C01 | `git push origin main` (baseline block) | 2 | 2 | 2 |
| C02 | `echo oi\ngit push` (JSON `\n`) — Forma A | 2 | 2 | 2 |
| C03 | `echo "hello" && git push` (JSON `\"`) — Forma B | 2 | 2 | 2 |
| C04 | `git\tpush` (JSON `\t`) — Forma C | 2 | 2 | 2 |
| C05 | `echo oi\u000agit push` (`\u000a`) — Forma D | 2 | 2 | 2 |
| C06 | `echo "a\\ngit push"` (JSON `\\n` = literal `\n`) — sem falso positivo | 0 | 0 | 0 |
| C07 | `git commit -m "a\\nb"` (literal `\\n`) | 2 | 2 | 2 |
| C08 | description=`git push`, command=`echo safe` — command em valor de string | 0 | 0 | 0 |
| C09 | dup: `{"command":"echo safe","command":"git push"}` | 2 | 2 | 2 |
| C10 | JSON com linha em branco antes de `command` | 2 | 2 | 2 |
| C11 | `{"command":"echo safe","tool_input":{"command":"git push"}}` | 2 | 2 | 2 |
| C12 | `{"command":"git push","tool_info":{"command_line":"echo"}}` | 2 | 2 | 2 |
| C13 | `git push \ud83d` (surrogate isolado) | 2 | 2 | 2 |
| C14 | `git push` (evasão unicode `g`) | 2 | 2 | 2 |
| C15 | `echo hello world` (controle: permitido) | 0 | 0 | 0 |
| C16 | `git status` (controle: read-only) | 0 | 0 | 0 |
| C17 | payload sem chave `command` (tipo Read tool) | 0 | 0 | 0 |
| C18 | string não-terminada (decode error) | 2 | 2 | 2 |
| C19 | nested: `{"meta":{"tool_input":{"command":"echo safe"}},"tool_input":{"command":"git push"}}` | 2 | 2 | 2 |
| C20 | `echo foo\\` (trailing backslash — D2 não dispara) | 0 | 0 | 0 |
| C21 | `echo oi\u000dgit push` (CR — não é separador) | 0 | 0 | 0 |

**Resíduo não obrigatório (declarado, sem caso de teste obrigatório):** `\u0000` (NUL) — todos os caminhos falham abertos por limite de bash `$()`. Pré-existente; não é regressão desta REQ. Se corrigido, REQ separada.

---

## Gates da Wave 0

```bash
test -s docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md
grep -q "Veredito" docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md
```

Ambos passam com este arquivo.

---

## Veredito

**APROVA COM AJUSTES**

O design do ADR (D1–D4) é som. O protótipo awk tem quatro restrições de implementação que ML-1A deve satisfazer:

| restrição | defeito correspondente |
|---|---|
| R3 — não usar `RS=""`, usar acumulação em `END` | JSON com linha em branco → awk=0, sed=2 (regressão) |
| R2 — last-wins OU deny-on-duplicate para chave duplicada | chave duplicada → awk=0, jq=2 (divergência) |
| R4 — `hex2dec()` portável, sem `strtonum()` | erro de runtime em BWK awk/macOS |
| R5 — prioridade exatamente como ADR D1 | `flat command` na 4ª posição vs 2ª no jq/ADR |

Nenhuma invalida o design: são constraints de implementação, não contradições de arquitetura. R1, R6, R7 são residuais declarados no ADR. ML-1A pode avançar com estas restrições explicitamente declaradas na tabela de testes.
