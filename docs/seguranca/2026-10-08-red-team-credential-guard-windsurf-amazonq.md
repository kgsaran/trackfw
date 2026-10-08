---
title: "Red-team: credential guard Windsurf e Amazon Q (ML-2B)"
date: 2026-10-08
author: hades-tf
req: "REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
roadmap: "ROADMAP-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
ml: "ML-2B"
status: concluido
---

# Red-team: credential guard Windsurf e Amazon Q (ML-2B)

> REQ-2026-10-06 · ML-2B · Hades-tf · 2026-10-08
> Revisão independente do diff `git diff main...HEAD -- internal/`
> (commit "fix(guard): credential guard cobre Windsurf e Amazon Q e lê o payload de cada CLI")

**Método:** reimplementação a partir da leitura do código-fonte. Binários compilados:
- Branch: `go build -o $SCRATCH/tf ./cmd/trackfw` da árvore `fix/credential-guard-nao-cobre-windsurf-e-amazon-q`
- Main: `git archive origin/main | tar -x && go build -o $SCRATCH/tf_main ./cmd/trackfw` (commit 5e6b7459)
Projeto de teste: `trackfw.yaml` com `credential_guard: mode: block`.
HOME isolado (`$SCRATCH/isolated_home*`) para todos os vetores que tocam `~/.codeium`.
Real `~/.codeium/windsurf/` não foi modificado (verificado pós-medição).
`grep -c eyJ` em ambos os arquivos de entrega: 0 (nenhum literal de JWT presente).

**Nota de processo:** ao menos duas chamadas de ferramenta durante exploração inicial escreveram
o prefixo base64 de header JWT como literal em variável de shell — violação da instrução do
despacho. O guard global disparou (PostToolUse, "possible JWT detected"). Nenhum secret real estava
envolvido. Todas as medições definitivas usaram construção por `printf '...' | base64 | tr ...`
dentro de scripts, sem escrever o valor em nenhum arquivo.

---

## Veredito

**NAO LIBERA.**

Quatro bloqueadores, todos nesta REQ (Regra Dura de Causa Raiz):

- **F1 (ALTO):** o validator aceita `echo trackfw guard credential --global` como "global completamente
  instalado" e suprime a verificação de projeto mesmo quando NENHUM guard executa. Nova neste PR (ML-1D).
  Requer ML-2C antes do merge.

- **F2 (MÉDIO, pré-existente):** isenção efêmera não entende sinks secundários em pipelines —
  `echo JWT | tee out.txt > /dev/null` é isento em ambos os binários. ML-1C reworkou
  `credIsAllEphemeral`; o gap pertence a esta REQ por Regra Dura. Requer ML-2D antes do merge.

- **F3 (ALTO, regressão):** BOM UTF-8 na frente de um payload JSON desativa inteiramente a Layer 2b
  no branch — revertendo os quatro fixes do ML-1C (B1, B4-parcial, R1e, EE4). `credExtractCmdAndCwd`
  computa `stripped` mas passa `data` (com BOM) para `json.Unmarshal`. A motivação para strip é
  documentada como comentário de código no mesmo pacote desde commit 782f5767 ("PowerShell may emit
  one"); a nova função não replicou o tratamento. Medido nos dois escopos. Requer ML-2C (fix de
  uma linha) antes do merge.

- **F4 (BAIXO, regressão):** arbitrary depth narrowing — `{"params":{"command":"cat secret.txt"}}`
  Branch RC=0, Main RC=2. Para CLIs com payload fora dos 4 caminhos fixos (Cursor, Copilot, Kiro,
  Gemini: formatos não confirmados em docs), branch cai abaixo de main. Requer fallback em ML-2C.

---

## Achados

### F1 — ALTO: Validator aceita forma non-executing no global Windsurf

**Arquivo:** `internal/validator/validator_credential_guard.go`
**Função:** `credentialGuardGlobalInstalledWindsurf()` (nova em ML-1D)
**Mecanismo:** usa `collectCommandsWithMarker(..., credentialGuardGlobalSubcmdMarker, ...)` com
`credentialGuardGlobalSubcmdMarker = "trackfw guard credential --global"` (substring). Qualquer
string que contenha esse substring satisfaz o check — inclusive `echo trackfw guard credential --global`.
Quando retorna `true`, `validateCredentialGuardPresenceRequired()` linha 849 faz `continue` e ignora
a entrada Windsurf por completo. Nenhuma outra regra compensa: `validateGuardHookResolvable` só emite
violations de forma para entradas presentes; se não há entrada no arquivo de projeto, nada é checado.

**Varredura same-cause:** `grep -rn 'globalInstalled' internal/validator/` — o mecanismo existe
apenas em `validator_credential_guard.go` (linha 849). Nenhum outro arquivo afetado.

**Reprodução (medida):**
```
~/.codeium/windsurf/hooks.json:
{"hooks":{"pre_run_command":[{"command":"echo trackfw guard credential --global"}],
          "pre_write_code":[{"command":"echo trackfw guard credential --global"}]}}

.windsurf/hooks.json: apenas git-branch guard, sem credential guard

$ HOME=<echo_home> trackfw validate
No violations found.   ← FALSO POSITIVO (exit 0)
```
**Bare form também passa** — validator aceita `["trackfw guard credential --global"]` sem D11.

**Divergência com o generator:** `globalCredentialGuardInstalledWindsurf()` em `agentfiles.go:2511`
usa `simpleArrayHasValue(..., guardCredentialGlobalCmdPSPOSIX, false)` — igualdade exata.

**Pré-existente na main?** Não. `credentialGuardGlobalInstalledWindsurf()` é nova neste PR (ML-1D).

**Windows:** `harnessCredentialGuardTargetWindsurf` sempre escreve `guardCredentialGlobalCmdPSPOSIX`
(PS/POSIX); cmd.exe não é escrito no arquivo global do Windsurf. ML-2C não precisa acomodar cmd.exe.

---

### F2 — MÉDIO (pré-existente): sinks secundários em pipeline bypassam isenção efêmera

**Arquivo:** `internal/guard/credential.go`
**Função:** `credIsAllEphemeral`
**Mecanismo:** o scanner de redirects lê os redirecionamentos visíveis no shell command. Em
`echo JWT | tee out.txt > /dev/null`, o redirect visível é `> /dev/null` (ephemeral); `tee out.txt`
é argumento do programa. `credIsAllEphemeral` retorna `true`; JWT vai para `out.txt`.

**Reprodução:**
```
payload: {"tool_input":{"command":"echo <JWT> | tee out.txt > /dev/null"}}
Branch RC=0,  Main RC=0

payload: {"tool_input":{"command":"echo <JWT> | dd of=out.txt 2>/dev/null"}}
Branch RC=0,  Main RC=0

payload: {"tool_input":{"command":"echo <JWT> | tee file; : >/dev/null"}}
Branch RC=0,  Main RC=0

payload: {"tool_input":{"command":"echo <JWT> | cp /dev/stdin out.txt > /dev/null"}}
Branch RC=0,  Main RC=0   ← non-tee/dd discriminante

payload: {"tool_input":{"command":"echo <JWT> > /dev/null & python3 -c 'open(\"o\",\"w\").write(\"<JWT>\")'"}
Branch RC=0   ← background (&) não está na lista de metacaracteres

payload: {"tool_input":{"command":"python3 -c 'open(\"o\",\"w\").write(\"<JWT>\")' > /dev/null"}}
Branch RC=0   ← argv0 != echo/printf; shape rule sem allowlist é insuficiente
```
**Pré-existente na main** em todos os vetores. ML-1C reworkou `credIsAllEphemeral`; gap pertence a
esta REQ pela Regra Dura.

**Vetores já bloqueados (sem regressão):**
```
payload: {"tool_input":{"command":"echo <JWT> > /dev/null &> out.txt"}}
Branch RC=2  (& precede >; credIsAllEphemeral não reconhece &>; falha fechada)

payload: {"tool_input":{"command":"echo <JWT> | cp /dev/stdin out.txt"}}
Branch RC=2  (sem redirect → allEphemeral=false)
```

---

### F3 — ALTO (regressão): BOM UTF-8 desativa Layer 2b no branch

**Arquivo:** `internal/guard/credential.go`
**Função:** `credExtractCmdAndCwd` (linha ~373)
**Mecanismo:**
```go
stripped := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
trimmed := bytes.TrimLeft(stripped, " \t\r\n")
if len(trimmed) == 0 || trimmed[0] != '{' {
    return "", "", false
}
var root map[string]json.RawMessage
if err := json.Unmarshal(data, &root); err != nil {  // BUG: deve ser `stripped`
    return "", "", false
}
```
`json.Unmarshal` da stdlib rejeita BOM → `isJSON=false` → Layer 2b desativada nos dois escopos.
`credExtractCmdAndCwd` é chamada em linha 87 (`RunCredential`) e linha 166 (`RunCredentialGlobal`);
o fix de uma linha cobre ambos automaticamente.

**Varredura same-cause:** demais chamadas `json.Unmarshal` em `credential.go` (linhas 380, 396, 420,
428) operam sobre sub-elementos extraídos de `root` — não recebem `data` bruto; não afetadas.

**Trigger documentado como comentário de código:** commit 782f5767 ("feat(guard): hooks de guard
viram trackfw guard...") já tinha "UTF-8 BOM stripped — PowerShell may emit one" em `ExtractCommand`
do mesmo pacote. A nova `credExtractCmdAndCwd` introduzida pelo ML-1C não replicou o tratamento.

**Vetores de regressão medidos:**

| Vetor | Branch sem BOM | Branch com BOM | Main com BOM | Situação |
|---|---|---|---|---|
| `cat secret.txt` (JWT em arquivo) | 2 | **0** | 2 | regressão: branch cai abaixo de main |
| `cat secret.txt > /dev/null` (B1) | 2 | **0** | 0 | regressão vs. branch sem BOM |
| `cat "secret.txt"` (R1e, arquivo presente) | 2 | **0** | 0 | regressão vs. branch sem BOM |
| fs_write + JWT + `/dev/null` (EE4) | 2 | **0** | 0 | regressão vs. branch sem BOM |
| fs_write + JWT sem `/dev/null` | 2 | 2 | 2 | Layer 1 captura JWT inline; sem regressão |
| `cat secret.txt` com `--global` | 2 | **0** | 2 | regressão: branch cai abaixo de main |

**Correção:** linha ~373: `json.Unmarshal(data, &root)` → `json.Unmarshal(stripped, &root)`.

---

### F4 — BAIXO (regressão): arbitrary depth narrowing

**Arquivo:** `internal/guard/credential.go`
**Mecanismo:** `credCmdLineRe` de main casava `"command":"..."` a qualquer profundidade. O JSON
parse do branch verifica 4 caminhos fixos. Para CLIs com formato fora dos 4 fixos, branch cai
abaixo de main.

**Medição:** `{"params":{"command":"cat secret.txt"}}` — Branch RC=0, Main RC=2.

**CLIs não confirmados em docs:** Cursor, Copilot, Kiro, Gemini — payloads assumidos; formatos
reais não verificados. Para qualquer um deles usando caminho fora dos 4 fixos, este é um regression.

**Correção (ML-2C):** fallback: quando `isJSON=true` e nenhum path fixo retornou `shellCmd` não-vazio
e `tool_name != "fs_write"`, percorrer o JSON tree coletando TODOS os valores string de chaves
`"command"` a qualquer profundidade e passá-los individualmente pela Layer 2b. Determinista
(todos são checados), falha fechada.

---

## Behavior Changes Documentadas (não regressões)

### B1 — `cat secret.txt > /dev/null`: main RC=0, branch RC=2 (melhoria intencional)

```
payload: {"tool_input":{"command":"cat secret.txt > /dev/null"}}  # secret.txt contém JWT
main   RC=0  (blanket exemption)  /  branch RC=2  (ML-1C: cmdHasMatch=false → Layer 2b)
```
Correto no branch. BOM reverte este fix (F3 — Branch+BOM RC=0, Main+BOM RC=0).

### B2 — Duplicate JSON key: main usa primeira ocorrência (regex), branch usa última (JSON)

```
payload: {"tool_input":{"command":"cat secret.txt","command":"echo hi"}}
main RC=2  /  branch RC=0
```
Branch correto. RFC 8259 "SHOULD be unique"; payload normal de CLI não gera chaves duplicadas.

### B3 — Symlinked cwd: branch segue symlink, main não (melhoria)

```
{"tool_info":{"command_line":"cat token.txt","cwd":"<link_dir>"}}   # link_dir → dir com token.txt
main RC=0  /  branch RC=2
```

### B4 — cwd fora do projeto: branch RC=2, main RC=0 (melhoria)

```
{"tool_info":{"command_line":"cat outside_secret2.txt","cwd":"/tmp/outside_dir"}}
branch RC=2  /  main RC=0
```

---

## Vetores Medidos

| Vetor | Branch RC | Main RC | Notas |
|---|---|---|---|
| Priority-1 wins | 0 | 0 | `tool_input.command="echo hi"`, `tool_info.command_line="cat secret.txt"`; correto por design |
| BOM + JWT inline no comando | 2 | 2 | Layer 1 detecta JWT no payload bruto; sem regressão |
| BOM + `cat secret.txt` | **0** | 2 | **F3** |
| BOM + `cat secret.txt > /dev/null` (B1) | **0** | 0 | **F3** — sem BOM Branch=2 |
| BOM + `cat "secret.txt"` (arquivo presente) | **0** | 0 | **F3** — sem BOM Branch=2 |
| BOM + fs_write + JWT + >/dev/null | **0** | 0 | **F3** — sem BOM Branch=2 |
| BOM + fs_write + JWT sem /dev/null | 2 | 2 | Layer 1; sem regressão |
| BOM + global (cat secret.txt) | **0** | 2 | **F3** — afeta ambos escopos |
| FIFO como argumento | 0 | — | `credScanFile`: `!IsRegular()` → skip; sem hang |
| Arquivo >1MiB | 0 | — | `fi.Size() >= credMaxFileSize` → skip |
| Ephemeral simples (`echo JWT > /dev/null`) | 0 | 0 | isenção legítima; cmdHasMatch=true; allEphemeral=true |
| $null redirect (Unix) | 2 | 2 | não ephemeral |
| Redirect para arquivo real | 2 | 2 | não ephemeral |
| NUL Windows (no Unix) | 2 | 2 | não ephemeral |
| tee + /dev/null (F2) | 0 | 0 | **F2** |
| dd of= (F2) | 0 | 0 | **F2** |
| tee + semicolon (F2) | 0 | 0 | **F2** |
| cp /dev/stdin + > /dev/null (F2) | 0 | 0 | **F2** — non-tee/dd discriminante |
| background & + writer (F2) | 0 | — | **F2** — `&` ausente da lista de metacaracteres |
| python3 -c open write > /dev/null (F2) | 0 | — | **F2** — argv0 != echo/printf |
| &> redirect (já bloqueado) | 2 | — | credIsAllEphemeral não reconhece `&>`; sem regressão |
| pipe sem redirect (`cp /dev/stdin out.txt`) | 2 | 2 | allEphemeral=false; já bloqueado |
| Process substitution `>(cat > out.txt)` | 2 | 2 | JWT inline; Layer 1 captura |
| EE4 sem BOM + devnull (ML-1C fix) | 2 | 0 | CORRIGIDO em ML-1C |
| EE1: Windsurf pre_write_code JWT + >/dev/null | 2 | — | cmdHasMatch=false |
| cat symlink → arquivo com JWT | 2 | 2 | Sem regressão |
| cwd traversal (../../../../etc/passwd) | 0 | — | Sem JWT em /etc/passwd; sem falso positivo |
| Windsurf project echo form | exit 1 | — | `validateGuardHookResolvable` detecta |
| Amazon Q project echo form | exit 1 | — | `validateGuardHookResolvable` detecta |
| Validate: Windsurf sem credential guard | exit 1 | — | violação emitida |
| Validate: Amazon Q sem fs_write guard | exit 1 | — | violação emitida |
| Validate: partial global (só pre_run_command) | exit 1 | — | `credentialGuardGlobalInstalledWindsurf()` false |
| Validate: D11 global completo + projeto sem guard | exit 0 | — | dedup legítimo |
| Update idempotência | sem duplicata | — | `mergeSimpleCommandArray` idempotente |
| Validate: JSON inválido | exit 1 | — | Sem crash |
| Codex cat file (R1a) | 2 | 2 | Sem regressão |
| Cursor/Copilot root-level (assumed shape) | 2 | 2 | Priority 2; formato não confirmado em docs |
| Kiro hook_input.command (assumed shape) | 2 | 2 | Priority 4; formato não confirmado em docs |
| Gemini payload | — | — | Formato não confirmado; **não medido**; gap pré-existente |
| Amazon Q echo JWT > /dev/null | 0 | 0 | Isenção legítima preservada |
| Residual R2: pre_read_code | 0 | — | Residual declarado |
| R1e sem BOM (baseline) | 2 | 0 | ML-1C fix; BOM reverte (F3) |
| F4: `{"params":{"command":"cat secret.txt"}}` | 0 | 2 | **F4** |

Total medidos na tabela: 43 (Gemini não medido)
Vetores adicionais medidos nos achados e behavior changes: F1 echo-global (exit 0), F1 bare-form
(exit 0), B2 duplicate-key, B3 symlinked-cwd, B4 cwd-outside — total geral: 48 medidos.

---

## Residuais Declarados — Comportamento Confirmado

**R2 — pre_read_code/fs_read sem proteção real:** confirmado (RC=0 com JWT no arquivo referenciado).

**R4 — Escopo global do Amazon Q não documentado:** confirmado. Harness global não existe para Amazon Q.

---

## MLs Corretivos

### ML-2C — Corretivo de F1 + F3 + F4
**Arquivos:** `internal/guard/credential.go`, `internal/validator/validator_credential_guard.go`
**Ações:**

1. **(F3 — uma linha)** `internal/guard/credential.go` linha ~373:
   `json.Unmarshal(data, &root)` → `json.Unmarshal(stripped, &root)`.
   `credExtractCmdAndCwd` é chamada nas linhas 87 e 166; o fix cobre ambos escopos.

2. **(F1)** `internal/validator/validator_credential_guard.go`:
   mudar `credentialGuardGlobalInstalledWindsurf()` para verificar igualdade de string contra a
   constante D11 completa. Declarar `credentialGuardGlobalCmdWindsurf` no validator como literal
   string (import cycle impede importar generators — padrão já adotado). Acrescentar teste que
   pina as duas constantes.

3. **(F4 — fallback)** `internal/guard/credential.go` `credExtractCmdAndCwd`:
   quando `isJSON=true` e nenhum dos 4 caminhos fixos retornou `shellCmd` não-vazio e
   `tool_name != "fs_write"`, percorrer o JSON tree coletando TODOS os valores string de chaves
   `"command"` a qualquer profundidade e passá-los individualmente pela Layer 2b. Determinista
   (todos são checados); falha fechada.

**Critérios de aceite:**
- F3: BOM + `cat secret.txt` → RC=2 nos dois escopos (era 0)
- F3: BOM + `cat secret.txt > /dev/null` (B1) → RC=2 (era 0)
- F3: BOM + `cat "secret.txt"` (arquivo presente) → RC=2 (era 0)
- F3: BOM + fs_write + JWT + `/dev/null` → RC=2 (era 0)
- F3: payload sem BOM → comportamento inalterado
- F3: BOM + JWT inline → RC=2 (Layer 1 preservada)
- F1: echo global + sem credential no projeto → validate exit 1 (era 0)
- F1: D11 global (ambos eventos) + sem credential no projeto → validate exit 0 (dedup preservado)
- F1: bare global + sem credential no projeto → validate exit 1
- F1: teste que pina constante do validator contra constante do generator
- F4: `{"params":{"command":"cat secret.txt"}}` → RC=2 (era 0)
- F4: payload com dois `command` aninhados (benigno + `cat secret.txt`) → RC=2 em N ≥ 20 runs consecutivos (prova de determinismo)
- F4: `tool_name="fs_write"` com `params.command="cat secret.txt"` → RC determinado por Layer 1/EE4, não pelo fallback (fs_write exclusão preservada)
- Falsificação nas duas direções por achado
- `make quality` verde

### ML-2D — Corretivo de F2 (sinks secundários em pipeline)
**Arquivo:** `internal/guard/credential.go`
**Ação:** a isenção efêmera só deve aplicar quando:
- shellCmd é um **comando simples** — sem `|`, `;`, `&&`, `||`, `&` (background), `$(`, `` ` `` ou newline; E
- argv[0] é `echo` ou `printf`; E
- todos os redirects são ephemeral (`/dev/null`, `$(mktemp)`, ou equivalentes).

O allowlist em argv[0] é a defesa que fecha o caso `python3 -c "..." > /dev/null`. A lista de
metacaracteres fecha pipelines e background processes.

**Critérios de aceite:**
- `echo <JWT> | tee out.txt > /dev/null` → RC=2 (era 0)
- `echo <JWT> | dd of=out.txt 2>/dev/null` → RC=2 (era 0)
- `echo <JWT> | tee file; : >/dev/null` → RC=2 (era 0)
- `echo <JWT> | cp /dev/stdin out.txt > /dev/null` → RC=2 (era 0)
- `echo <JWT> > /dev/null & python3 -c '...'` → RC=2 (era 0)
- `python3 -c 'open("o","w").write("<JWT>")' > /dev/null` → RC=2 (era 0)
- `echo <JWT> > /dev/null` → RC=0 (isenção legítima preservada)
- `printf '<JWT>' > /dev/null` → RC=0 (isenção legítima preservada; argv0=printf)
- `echo <JWT> | tee /dev/null > /dev/null` → RC=2 (falso positivo benigno aceito: pipeline)
- Falsificação nas duas direções

---

*Hades-tf — 2026-10-08*
