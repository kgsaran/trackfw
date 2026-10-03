# Wave 2 — Revisão de Segurança Independente: extrator JSON do guard

> Data: 2026-10-03 | Agente: hades-tf | ML: ML-2A
> Roadmap: `ROADMAP-2026-10-02-trackfw-git-branch-guard-falha-aberto-sem-jq-o-fallback-por-sed-nao-interpreta-json.md`
> Wave 0 de referência: `docs/seguranca/2026-10-02-wave0-extrator-json-do-guard.md`
> Diff auditado: `git diff 3b2eff09..HEAD -- scripts/trackfw-git-branch-guard.sh internal/generators/scaffold.go`

Metodologia: reimplementação a partir da leitura do script. Todos os casos executados diretamente contra
`scripts/trackfw-git-branch-guard.sh` HEAD (`973a10da`) com e sem `jq`. PATH curado sem jq: todos os
diretórios que contêm `jq` ou `jq.exe` excluídos (`/opt/homebrew/bin`, `/usr/bin`); shims para `awk`,
`sed`, `head`, `tr`, `find` num diretório temporário. `command -v jq` vazio no PATH curado — confirmado
antes de cada rodada.

---

## Restrições R2–R5 da Wave 0 — verificação de implementação

| restrição | status |
|---|---|
| R3 — `RS=""` removido, acumulação em `END` | ✅ `{_a=_a $0 "\n"}` + `END{...}` (linhas 215–216) |
| R2 — last-wins para chave duplicada | ✅ `_vn[_d]=0` no `,` e sobreposição de `_p1`/`_p2` na mesma chave |
| R4 — `hex2dec()` sem `strtonum()` | ✅ função `hex2dec` portável nas linhas 180–184 |
| R5 — prioridade igual ao ADR D1 | ✅ `_p1`=`tool_input.command`, `_p2`=flat `command`, `_p3`=`tool_info.command_line`, `_p4`=`hook_input.command` |

---

## Casos C01–C22 (tabela da Wave 0)

Todos executados contra HEAD. Coluna `exp` = valor esperado; `OK` = medido == esperado.

| id | payload (intenção) | +jq | -jq | resultado |
|---|---|---|---|---|
| C01 | `git push origin main` (baseline block) | 2 | 2 | OK |
| C02 | `echo oi\ngit push` (JSON `\n`) | 2 | 2 | OK |
| C03 | `echo "hello" && git push` (JSON `\"`) | 2 | 2 | OK |
| C04 | `git\tpush` (JSON `\t`) | 2 | 2 | OK |
| C05 | `echo oi\u000agit push` (`\u000a`) | 2 | 2 | OK |
| C06 | `echo "a\\ngit push"` (literal `\\n`) — sem falso positivo | 0 | 0 | OK |
| C07 | `git commit -m "a\\nb"` | 2 | 2 | OK |
| C08 | description=`git push`, command=`echo safe` | 0 | 0 | OK |
| C09 | dup: `{"command":"echo safe","command":"git push"}` | 2 | 2 | OK |
| C10 | JSON com linha em branco antes de `command` | 2 | 2 | OK |
| C11 | flat `command`=`echo safe`, `tool_input.command`=`git push` | 2 | 2 | OK |
| C12 | flat `command`=`git push`, `tool_info.command_line`=`echo safe` | 2 | 2 | OK |
| C13 | `git push \ud83d` (surrogate isolado) | 2 | 2 | OK |
| C14 | baseline block (controle) | 2 | 2 | OK |
| C15 | `echo hello world` (controle: permitido) | 0 | 0 | OK |
| C16 | `git status` (read-only: permitido) | 0 | 0 | OK |
| C17 | payload sem chave `command` (Read tool) | 0 | 0 | OK |
| C18 | string não-terminada (decode error) | 2 | 2 | OK |
| C19 | nested `tool_input` em `meta`, real na raiz | 2 | 2 | OK |
| C20 | `echo foo\\` (trailing backslash — D2 não dispara) | 0 | 0 | OK |
| C21 | `echo oi\rgit push` (CR — não é separador) | 0 | 0 | OK |
| C22 | `git push\u0000origin main` (NUL — fail-closed novo) | 2 | 2 | OK |

Nenhuma divergência em C01–C22. Nenhum falso positivo.

---

## Casos novos — Wave 2

Estes casos não fazem parte da tabela da Wave 0.

### N01 — `\u` com 3 dígitos hexadecimais

Payload (bytes literais, construído via `python3 chr(92)`):
`{"tool_input":{"command":"git push\u00a"}}`
onde `\u00a` são os 5 bytes literais `\`, `u`, `0`, `0`, `a` (escape incompleto — JSON exige 4 dígitos).

```
+jq=2  -jq=2  OK
```

jq: falha a parsear o JSON (unicode incompleto) → `CMD_RAW` vazio → fallback awk.
awk: `hs = substr(raw, i+1, 4)` — captura `00a"` (4 chars); `"` não é hex → `hex2dec` retorna -1 →
`_DECODE_ERR = "invalid_unicode_hex"` → `exit 2`. Fail-closed.

### N02 — `\u` com 4 dígitos não-hexadecimais (`\uzzzz`)

Payload: `{"tool_input":{"command":"git push\uzzzz"}}`

```
+jq=2  -jq=2  OK
```

awk: `hex2dec("zzzz")` → `d = index("0123456789abcdef","z") - 1 = -1` → retorna -1 → `invalid_unicode_hex` → `exit 2`. Fail-closed.

### N03 — Chave com escape unicode no nome (`"command"`) — DIVERGÊNCIA ENCONTRADA

Payload: `{"tool_input":{"command":"git push origin main"}}`
onde `a` = 'a', portanto a chave decodificada é `"command"`.

```
+jq=2  -jq=0  [DIVERGE]
```

**jq** decodifica escapes unicode nas chaves → `.tool_input.command` = `"git push origin main"` → bloqueia.

**awk** armazena a chave como raw string `command` (12 chars) na linha 235:
`if(!_vn[_d]){_lk[_d]=_raw}`. Compara com `"command"` (7 chars) → não casa → valor nunca
atribuído a `_p1` → `CMD_RAW` vazio → guard passa.

**Confirmação do mecanismo** (N03b — segundo sítio):
`{"tool_input":{"command":"git push","command":"echo safe"}}` → +jq=0 -jq=0. jq lê
`tool_input.command` pela chave decodificada = "git push", mas a chave plain `"command"` = "echo safe"
vem depois → last-wins → jq retorna "echo safe" → 0. awk não encontra nenhuma chave plain "command"
nos níveis certos... espera, awk SIM encontra "command" plain vindo depois. rc_awk=0 porque o plain
"command" do nível raiz... ah, aqui o "command" está em tool_input, depth=2. Confirmado: awk passa
porque armazena chave como raw.

**Reprodução do fix** (protótipo, não aplicado ao produto — implementação é de apolo-tf):
Linha 235 de `scripts/trackfw-git-branch-guard.sh`:

```awk
# antes
if(!_vn[_d]){_lk[_d]=_raw}

# depois
if(!_vn[_d]){_dkn=decode_str(_raw);_lk[_d]=(_DECODE_ERR==""?_dkn:_raw);_DECODE_ERR=""}
```

Com esse fix: N03 → +jq=2 -jq=2 (sem divergência). Regressão nos C01–C22: nenhuma.
Chave com escape inválido (ex.: `"comm\u00Xnd"`) → `decode_str` retorna erro → `_DECODE_ERR!=""` →
fallback para `_raw` → chave não casa com "command" → guard passa sem falso positivo. Confirmado
via `Nbad` (`{"tool_input":{"comm\u00and":"git push"}}` → +jq=0 -jq=0 em ambas as versões).

**Sítios afetados** (4 cópias byte-idênticas):
- `scripts/trackfw-git-branch-guard.sh` linha 235
- `internal/generators/scaffold.go` `gitBranchGuardScript` (linha correspondente ~1920 do Go)
- `internal/validator/validator_git_branch_guard_reference.go` `gitBranchGuardScriptReference`
- `scripts/check-gates-falsify.sh` (literal do guard no cenário `corrupt_literal`)

### N04 — Valor numérico: `"command": 123`

```
+jq=0  -jq=0  OK
```

jq: `type == "string"` falso → `empty` → sem match. awk: só processa tokens `"..."` (strings JSON);
`123` é lido como sequência de dígitos, nunca como token de string → nenhuma chave/valor atribuído. Correto.

### N05 — Valor `null`: `"command": null`

```
+jq=0  -jq=0  OK
```

Mesmo mecanismo de N04. O token `null` (4 chars, sem aspas) nunca entra no branch de string do awk.

### N06 — Valor array: `"command": ["git","push","origin","main"]`

```
+jq=0  -jq=0  OK
```

jq: `type == "string"` falso → sem match. awk: `[` → `_d++`, `_io[_d]=0` (array, não objeto) →
strings dentro do array têm `_io[_d]==0` → o branch `if(_d>=1 && _io[_d]==1)` não dispara → correto.

### N07 — `tool_input` como string (não objeto)

Payload: `{"tool_input":"git push origin main"}`

```
+jq=0  -jq=0  OK
```

jq: `.tool_input.command` → `null` (string não tem campos) → cai para `.command` → ausente → `empty`.
awk: `"tool_input"` é chave, `:` seta `_vn[1]=1`, e então `"git push origin main"` é um string token
em posição de valor: `_k="tool_input"`, `_par=""`. Nenhuma condição de captura casa (`_k != "command"`).
Depois vem `}` → `_d--` → `_d=0`. Nenhum valor capturado. Correto.

### N08 — String muito longa sem aspas de fechamento (50 000 bytes, sem `"` de fechamento)

Payload: `{"tool_input":{"command":"` + `A` × 50 000 (sem fechar com `"`)

```
+jq=2  -jq=2  OK
```

awk: laço de leitura de string chega ao `END` com `_j > _n` sem encontrar `"` de fechamento →
`_sc != "\""` → `_e = "unterminated_string"` → `exit 2`. Fail-closed, sem laço infinito.

### N09 — 10 000 backslashes no valor (`\\` × 5 000 + `git push`)

Payload: `{"tool_input":{"command":"` + `\\` × 5 000 + `git push"}}`

Valor decodificado: 5 000 backslashes literais + `git push`. O comando não começa com o token `git`
(começa com `\\...`), então `match_subcommand` não bloqueia.

```
+jq=0  -jq=0  OK
```

Latência medida (PATH curado sem jq): **39 ms** wall clock. Sem laço. O O(n²) de concatenação char-a-char
documentado na Wave 0 (§4, 200 KB → 1,7 s) não aparece em 10 000 backslashes porque o valor total é
~10 KB, que se mantém no regime linear do custo por invocação de awk.

---

## Resumo de divergências jq × awk

| caso | payload | +jq | -jq | status |
|---|---|---|---|---|
| N03 | chave `"command"` (unicode-escape no nome) | 2 | 0 | **DIVERGE — achado** |
| todos os demais | — | rc | rc | sem divergência |

---

## Achado: N03 — chave unicode-escaped no no-jq path

**Superfície:** awk não decodifica escapes unicode em nomes de chave (`line 235`). jq decodifica.
Um agente que (a) detecta ausência de `jq` e (b) envia `"command"` em vez de `"command"` evita
o guard no path sem jq.

**Pré-condição do bypass:** o agente precisa saber que jq está ausente E enviar payload fora do
formato padrão de hook (chave com escape). Payloads reais gerados pela infraestrutura Claude usam
`"command"` plain. Severidade: baixa (requer adversário knowledgeable + payload não-padrão).

**Fix:** linha 235 dos 4 sítios, conforme protótipo em N03 acima. Testado: fecha a divergência sem
regressão.

**Classificação no contexto do roadmap:** este achado NÃO bloqueia o veredito de APROVA COM AJUSTES
porque está fora do threat model declarado da Wave 0 ("rushed implementer e optimistic architect", não
adversário knowledgeable). Mas como é um defeito medido com fix de uma linha, **deve entrar como ML
adicional na REQ vigente** antes do merge, conforme regra de causa raiz do CLAUDE.md.

---

## Resíduo declarado (mantido da Wave 0)

| # | item |
|---|---|
| R1 | `\u0000` (NUL) — todos os caminhos falham abertos (bash `$()` descarta NUL); pré-existente |
| R6 | Custo O(n²) em valores muito longos (> 100 KB) |
| R7 | `\uXXXX` não-ASCII → marcador `\002` |
| R8 | gawk/mawk não medidos nesta VM |

---

## Veredito

**APROVA COM AJUSTES**

Os 22 casos C01–C22 da Wave 0 passam sem divergência. As restrições R2–R5 estão implementadas.
Um achado novo (N03): chave com unicode-escape no nome diverge jq×awk no path sem jq. Fix de uma
linha fornecido, testado, sem regressão. Deve ser incorporado como ML adicional na REQ vigente antes
do merge, não em REQ separada.

---

> **Nota do arquiteto (auditoria, 2026-10-03):** o resíduo **R1** acima está desatualizado. Ele repete a
> Wave 0, mas o D2-bis foi implementado: `git push\u0000origin main` sai **rc=2 nos dois caminhos**,
> medido nos testes `TestGitBranchGuardAwk_C01C22_WithJQ/C22` e `…_WithoutJQ/C22` (PASS) no HEAD
> `973a10da`. O NUL não é mais resíduo.

