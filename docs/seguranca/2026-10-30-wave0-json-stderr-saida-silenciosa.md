# Wave 0 — Modelo de ameaça: erro tipado de saída silenciosa (`--json` limpo depois do documento)

> Roadmap: `ROADMAP-2026-09-08-validate-suprime-usage-em-erro-de-runtime-e-o-gate-que-provaria-isso.md` · Wave 0 · ML-0A
> Data: 2026-10-30 · Revisor: hades-tf

---

## Seção 1 — Completude da enumeração

### 1.1 Todos os comandos com `--json` no código de produção

Enumeração sem truncamento de `internal/commands/*.go` (excluindo `_test.go`):

| arquivo | flag registrada | padrão de escrita |
|---|---|---|
| `validate.go:73` | `--json` | `fmt.Fprintln(OutOrStdout, data)` → `return fmt.Errorf(...)` → **pós-documento** |
| `update.go:92` | `--json` | `fmt.Fprintln(OutOrStdout, out)` → `return fmt.Errorf(...)` → **pós-documento** |
| `update_harness.go:113` | `--json` | `fmt.Fprintln(OutOrStdout, out)` → `return fmt.Errorf(...)` → **pós-documento** |
| `barrier.go:122` | `--json` | `os.Exit(1)` direto em `runBarrier` → não devolve erro ao root.go → **limpo** |
| `doctor.go:81` | `--json` | `return encoder.Encode(findings)` — a escrita É o valor de retorno → **pré-documento** |
| `audit_surface.go:79` | `--json` | `return enc.Encode(report)` — a escrita É o valor de retorno → **pré-documento** |
| `roadmap.go:157` (show) | `--json` | `return enc.Encode(doc)` via `ShowRoadmapJSON` — escrita IS retorno → **pré-documento** |
| `integrations_flags.go:119` | `--json` | `return printLifecycleOutput(...)` → `return encoder.Encode(output)` — escrita IS retorno → **pré-documento** |

**`context.go` — excluído da tabela:** `context.go` usa `--format=json` (string), não `--json` (bool). `trackfw context --json` produz `rc=1, stderr=176 B (unknown flag: --json)` — erro de flag-parse antes de qualquer documento. O arquivo `windows-probe.yml:922` que menciona `context --json` refere-se provavelmente a um engano de workflow; a flag registrada é `--format`. Medido: `trackfw context --format=json` → `rc=0, stdout JSON, stderr=0 B`. `context.go` não pertence à enumeração de sítios `--json` e não tem o padrão pós-documento.

**Conclusão:** a tabela da REQ ("Revisão de escopo — 2026-10-30") é fechada. Exatamente três sítios têm o padrão pós-documento: `validate.go:38`, `update.go:85`, `update_harness.go:106`. Nenhum outro sítio foi encontrado. `context.go` explicitamente classificado como fora do escopo (flag distinta; comportamento medido).

**Critério de distinção pré/pós:** o padrão pós-documento é `fmt.Fprintln(OutOrStdout, ...)` na linha N, seguido de `return fmt.Errorf(...)` na linha N+k. O padrão pré-documento é `return encoder.Encode(...)` ou `return json.Marshal(...)` — o erro e a escrita são atômicos; se a escrita falha, o documento está incompleto e o erro é o correto a reportar.

### 1.2 Todos os escritores diretos de `os.Stderr` acessíveis pelos três comandos

Enumeração por `grep -rn 'os\.Stderr\|\.ErrOrStderr\|log\.Print\|log\.Fatal' internal/ --include='*.go'` filtrada para os três caminhos de execução (excluindo `_test.go` e código morto para esses comandos):

| escritor | arquivo:linha | comando alcançável | momento | após o fix |
|---|---|---|---|---|
| `root.go:125` — `err.Error()` sem prefixo | `commands/root.go` | `validate` | pós-documento | **removido** |
| `root.go:127` — `"Error: " + err.Error()` | `commands/root.go` | `update`, `update harness` | pós-documento | **removido** |
| `pathguard.RejectAndReport` (224/245) | `pathguard/pathguard.go` | `update` com symlink target | durante computação | **permanece** (§3.10) |
| `config.MalformedConfigMessage` (295) | `config/config.go` | `validate`, `update` | pre-exit: `config.Load()` chama `osExit(1)` imediatamente — processo aborta ANTES de qualquer documento | permanece (fora do escopo — não há documento a proteger) |
| `harnessCatalogTarget` (2671) — `harnessWarnMsg` | `generators/update.go` | `update harness` com targets de catálogo (`<tool>-agents`/`<tool>-skills`) | durante computação (dentro de `UpdateHarness`, ANTES do marshal), mas **o documento ainda é escrito depois** — coexiste com o documento | **permanece** (§3.11 — classe a declarar em residual) |

**Observação sobre `integrations_flags.go`:** as escritas em os.Stderr nesse arquivo (linhas 230, 250, 288, 316, 380) são do caminho `trackfw agents install/update --json`, não de `trackfw update --json` nem `trackfw update harness --json`. Escopo disjunto — não afetam AC1/AC1-bis.

**Observação sobre `update_harness.go`:** diferentemente de `update.go`, o `update harness` NÃO usa `silenceStdout()` ao chamar `UpdateHarness()`. O `harnessWarnMsg` (linha 2671) escreve em `os.Stderr` durante a avaliação de cada target de catálogo (`<tool>-agents`/`<tool>-skills`) e faz isso ANTES do marshal — mas o documento É escrito depois, então coexiste. **Este write não é coberto pelo fix**: após o fix, a linha de root.go some mas as linhas de `harnessWarnMsg` permanecem. Medido em §3.11.

**`config.MalformedConfigMessage`:** `config.Load()` chama `osExit(1)` imediatamente quando trackfw.yaml é inválido. O processo aborta antes de qualquer documento — nenhum stdout, só o `MalformedConfigMessage` + exit 1. Fora do escopo desta REQ (mesma categoria do erro pré-documento).

---

## Seção 2 — Modelo de ameaça

**Adversário primário:** o implementador apressado. Não um atacante externo — a superfície aqui é o próprio código do produto.

### Ameaça T1 — Fail-open via código 0

O constructor do erro tipado que não rejeita `Code == 0` permite que uma instância com `Code: 0` chame `os.Exit(0)` via root.go → o processo reporta sucesso mesmo com falha real.

**Quem é afetado:** qualquer chamada ao constructor com argumento zero literal ou variável não inicializada.
**Onde entra:** no constructor público do tipo, se não houver validação.
**Que gate pega:** um test que afirme `Code != 0` no constructor (`t.Errorf` se Code == 0). Sem esse teste, passa.

### Ameaça T2 — Fail-open via `errors.As` + `errors.Join`

Medido:

```
Test5  direct assertion on errors.Join(real,base):  false  (não fail-open)
Test6  errors.As on errors.Join(real,base):          true  (FAIL-OPEN)
Test7  direct assertion on errors.Join(base,real):  false  (não fail-open)
Test8  errors.As on errors.Join(base,real):          true  (FAIL-OPEN)
```

Se root.go reconhece o tipo via `errors.As`, então `errors.Join(realErr, silentErr)` é reconhecido como saída silenciosa, `realErr` é engolido e o processo sai silenciosamente. O implementador que quiser "também sinalizar o código correto" com `errors.Join` introduz o bug.

### Ameaça T3 — Fail-open via `%w`

Medido:

```
Test3  direct assertion on %w:  false  (não fail-open)
Test4  errors.As on %w:          true  (FAIL-OPEN)
```

`fmt.Errorf("context: %w", silentErr)` com `errors.As` → silencia. Com direct assertion → o erro "context: silent exit 1" é impresso como erro normal. Ameaça real apenas se root.go usar `errors.As`.

### Ameaça T4 — Captura ampla nos ramos pré-documento

`update.go` tem dois caminhos de erro que precedem a escrita do documento:
- linha 62–64: `silenceErr` de `silenceStdout` (falha ao abrir `/dev/null`)
- linha 75–76: `marshalErr` de `json.Marshal`

Se o ML-1A substituir o `return fmt.Errorf(...)` em `update.go:85` por um padrão e, por descuido, também substituir o `return marshalErr` ou `return silenceErr`, esses erros ficam silenciosos para o consumidor. O documento não foi escrito ainda, então o consumidor recebe `rc=1`, stdout vazio, stderr vazio — condição indistinguível de um resultado legítimo sem violações (exceto pelo `rc`).

**Onde entra:** no próprio diff do ML-1A, se a busca-e-troca for por padrão em vez de por sítio.

### Ameaça T5 — Ausência de contra-braço funcional nos testes existentes

Os testes atuais (`validate_json_test.go`) chamam `newValidateCmd().Execute()` e usam `cmd.SetErr(&bytes.Buffer{})`. O fluxo de reconhecimento do erro tipado está em `root.go:Execute()`, que:
1. nunca é chamado por esses testes (chamam `cmd.Execute()` direto, não `Execute()`)
2. escreve em `os.Stderr` diretamente via `fmt.Fprintln(os.Stderr, ...)`, não via `cmd.OutOrStderr()`

Consequência: um teste que assira `errBuf.Len() == 0` passa hoje (antes do fix) e passa depois do fix — e passaria de novo se a regressão fosse introduzida. O contra-braço especificado no AC do roadmap (`revertido o reconhecimento no root.go, os testes novos reprovam`) é impossível com o padrão atual de teste em processo.

**Onde entra:** no próprio arquivo de teste do ML-1A, se usar o mesmo padrão `cmd.SetErr`.

### Ameaça T6 — D7 (guard → exit 2) ignorado

D7 (ADR-2026-10-04): toda falha sob `trackfw guard` sai com código 2.

Medido:
```
$ trackfw guard write-scope --nonexistent-flag
rc=2
```

Se um comando sob guard retornar o erro tipado com `Code: 1`, e root.go reconhecer o tipo antes de verificar `isCommandUnderGuard(cmd)`, o processo sai com 1 em vez de 2. O hook do git que chama `trackfw guard ...` lê o código de saída para decidir allow/deny; 1 != 2 pode confundir ferramentas que usam o código 2 como sinal de "guard ativo negou" vs "trackfw error".

Risco real é baixo porque os três sítios afetados (`validate`, `update`, `update harness`) não estão sob guard. Mas a ordem de verificação deve estar documentada.

---

## Seção 3 — Confirmação por medição

Binário compilado em `$BUILDDIR/tfw` fora da árvore; fixtures em diretórios mktemp. §3.1–3.11 rodaram com `env -i HOME="$FAKEHOME" PATH="$BUILDDIR:/usr/bin:/bin" LANG=C` sem exceções. §3.9 e §3.10 foram reexecutados com `env -i` durante a finalização deste documento e confirmaram os números documentados. Runs de `context` em §1.1 foram usados apenas para classificação de flag (nota em §1.1); não medem bytes de resultado e não estão na tabela de §3.

**Nota — Incidente Item 0 (executar sem `cd` para o fixture):** duas execuções de `update --json` iniciais rodaram contra a árvore real (`/Users/kgsaran/.../trackfw`) em vez de um fixture. Método de bounding: `find <repo> -newermt "2026-10-30 00:00:00" -type f -not -path '*/.git/*'` retornou 26 arquivos; `git status --porcelain` mostra apenas `?? docs/seguranca/2026-10-30-wave0-json-stderr-saida-silenciosa.md`. Os arquivos `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` foram reescritos com bytes idênticos (git não vê diff). As entradas `.trackfw-log-entries/20261030T1*Z-*.entry` que aparecem no `find` estão confirmadas como comprometidas no HEAD (commit `2cb2c3b3`). `.git/hooks/` não foi modificado: todos os arquivos têm mtime de Jun 11 (confirmado por `ls -l .git/hooks/`). Dano funcional: nenhum. Causa: falta de `cd` antes do comando no primeiro ciclo de medição.

### 3.1 `validate --json` com violação (AC1)

Fixture: `trackfw.yaml` (strict), roadmap em `wip/` sem REQ, `git symbolic-ref HEAD refs/heads/feat/x`.

```
$ env -i HOME=$FAKEHOME1 PATH=... LANG=C tfw validate --json
rc=1
stdout: 1091 bytes (JSON válido — 4 violations, 1 warning)
stderr: 21 bytes
```

stderr (xxd):
```
00000000: 3420 7669 6f6c 6174 696f 6e28 7329 2066  4 violation(s) f
00000010: 6f75 6e64 0a                             ound.
```

Mecanismo confirmado: `validate.go:22` (`cmd.SilenceErrors = true`) faz root.go entrar no ramo `cmd.SilenceErrors` (linha 124–125) e imprimir apenas `err.Error()` sem o prefixo `"Error: "`. Resultado: `"4 violation(s) found\n"` (21 bytes).

### 3.2 `validate` modo texto com violação (AC7 baseline)

```
rc=1
stdout: 687 bytes (texto com ✗ linhas)
stderr: 28 bytes — "Error: 4 violation(s) found\n"
```

Modo texto não seta `cmd.SilenceErrors` → root.go imprime `cmd.ErrPrefix()` + `err.Error()` = `"Error: "` + `"4 violation(s) found"` + `"\n"`. Deve permanecer idêntico após o fix.

### 3.3 `validate --json` com modo lenient ativo

Fixture: `trackfw.yaml` com `governance_mode: lenient` e `lenient_until: 2027-06-30` (dentro do horizonte máximo de 730 dias — `config.LenientHorizonDays`). Mesmo conjunto de violações do §3.1.

**Nota sobre o horizonte:** `lenient_until: 2099-12-31` é tratado como strict por construção (além do horizonte de 730 dias de `isLenientFor`). A data deve ficar dentro de `now + 730 days` para que o modo seja ativado.

```
$ env -i HOME=$FAKEHOME PATH=... LANG=C tfw validate --json
rc=0
stdout: 1093 bytes (JSON válido — mode="lenient", exit_code=0, 5 warnings, 0 violations)
stderr: 0 bytes
```

```
$ env -i HOME=$FAKEHOME PATH=... LANG=C tfw validate
rc=0
stdout: "[LENIENT MODE] Governance violations treated as warnings until 2027-06-30\n..."
stderr: 0 bytes
```

Em modo lenient, as violações tornam-se avisos e o processo sai com `rc=0` — o caminho de `return fmt.Errorf(...)` pós-documento nunca é atingido. O defeito não afeta o caminho lenient. AC7 ("lenient não muda") confirmado: a saída já é correta antes e depois do fix.

### 3.4 `update --json` com `TargetFailed` (AC1-bis)

Fixture: `trackfw.yaml`, `CLAUDE.md` com chmod 444. Corrida com `cd $F2` + `--targets agent-rules`.

```
rc=1
stdout: 501 bytes (JSON com state="failed" e summary.failed=1)
stderr: 42 bytes
```

stderr (xxd):
```
00000000: 4572 726f 723a 2074 7261 636b 6677 2075  Error: trackfw u
00000010: 7064 6174 653a 2031 2074 6172 6765 7428  pdate: 1 target(
00000020: 7329 2066 6169 6c65 640a                 s) failed.
```

`"Error: "` prefixo presente porque `cmd.SilenceErrors` não é setado no ramo `--json` de `update.go` (diferente de `validate.go` que o seta explicitamente no ramo json). `root.go:127` imprime `cmd.ErrPrefix() + err.Error()`.

### 3.5 `update` modo texto com `TargetFailed` (AC7 baseline)

```
rc=1
stdout: texto com ✗ e detalhes do erro
stderr: "Error: trackfw update: 1 target(s) failed\n"
```

Deve permanecer idêntico após o fix.

### 3.6 `update harness --json` com `TargetFailed` (AC1-bis)

Fixture: `FAKEHOME3` com `.trackfw/` em modo 555. `--targets git-branch-guard-script`.

```
rc=1
stdout: 410 bytes (JSON com state="failed", path="~/.trackfw/scripts/trackfw-git-branch-guard.sh")
stderr: 50 bytes
```

stderr (xxd):
```
00000000: 4572 726f 723a 2074 7261 636b 6677 2075  Error: trackfw u
00000010: 7064 6174 6520 6861 726e 6573 733a 2031  pdate harness: 1
00000020: 2074 6172 6765 7428 7329 2066 6169 6c65   target(s) faile
00000030: 640a                                     d.
```

Todos os paths resolvidos dentro do FAKEHOME3 (confirmado pelo campo `message` do JSON).

### 3.7 `update harness` modo texto com `TargetFailed` (AC7 baseline)

```
rc=1
stdout: texto com ✗ e detalhes do mkdir
stderr: "Error: trackfw update harness: 1 target(s) failed\n"
```

### 3.8 Guard com argumento inválido (D7 baseline)

```
$ trackfw guard write-scope --nonexistent-flag
rc=2
```

D7 confirmado. O caminho guard→root.go verifica `isCommandUnderGuard(cmd)` e usa `exitCode = 2`.

### 3.9 `validate --json` com stdout fechado (`1>&-`) — P3 base

Antes do fix, o que acontece quando stdout é fechado antes da chamada:

```
$ env HOME=$FAKEHOME ... tfw validate --json 1>&-
rc=1
stdout: 0 bytes (inferred — fd1 fechado, Fprintln falha silenciosamente, documento perdido)
stderr: 21 bytes — "4 violation(s) found\n"  ← medido
```

A ausência de stdout é **inferida**: fd1 estava fechado no experimento e não foi capturado. O stderr de 21 bytes foi medido diretamente. O `fmt.Fprintln(cmd.OutOrStdout(), string(data))` falha silenciosamente (erro ignorado, documento perdido). A linha `return fmt.Errorf(...)` ainda é executada, então root.go ainda imprime a mensagem no stderr.

**Impacto pós-fix (sem a guarda de P3):** com o erro tipado sem mensagem, a mesma situação produziria `rc=1, stdout=0 B, stderr=0 B` — o consumidor recebe falha sem documento e sem mensagem. Isso é ambíguo (igual a um `rc=1` com zero violações em contextos que não produzem documento). P3 deve incluir uma guarda: se o `Fprintln` falhar, retornar um `fmt.Errorf` normal (ruidoso), não o erro tipado silencioso.

### 3.10 `update --json` com `CLAUDE.md` como symlink — ADR-2026-09-18 D3

Fixture: `CLAUDE.md` é um symlink (`ln -s real-claude.md CLAUDE.md`). `pathguard.RejectAndReport` recusa a escrita e escreve diretamente no stderr (não via root.go), além do retorno de erro.

```
rc=1
stdout: 1326 bytes (JSON com state="failed")
stderr: 432 bytes total, 2 linhas:
```

Linha 1 (pathguard, direto em stderr, ~388 B):
```
trackfw: refusing write to <path>/CLAUDE.md: refusing reparse-point path "<path>/CLAUDE.md" (mode Lrwxr-xr-x)
```

Linha 2 (root.go, 42 B — removida pelo fix):
```
Error: trackfw update: 1 target(s) failed
```

**AC1-bis não é plenamente atingível para a classe de recusa de symlink.** A linha 1 vem de `RejectAndReport` que escreve em `os.Stderr` independentemente do ramo `--json`. Após o fix, o stderr cai de 432 B para ~388 B (linha 2 removida), mas não chega a 0 B. O AC1-bis ("stderr 0 B") aplica-se apenas à classe TargetFailed por I/O (chmod, mkdir); a classe de recusa de pathguard mantém a linha de aviso de segurança no stderr — comportamento intencional por ADR-2026-09-18 D3 ("recusa audível").

### 3.11 `update harness --json` com targets de catálogo em HOME sem config global

Fixture: HOME vazio (sem `~/.trackfw/trackfw.yaml`), sem `--targets` (todos os 36 targets avaliados). Executado com `env -i`.

```
$ env -i HOME=$H_FRESH PATH=... LANG=C tfw update harness --json
rc=0
stdout: 2948 bytes (JSON válido — summary: updated=2, skipped=0, missing=34, failed=0)
stderr: 3320 bytes (20 linhas, todas idênticas — harnessWarnMsg por cada target de catálogo)
```

Linha de stderr (repetida 20 vezes):
```
trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml — usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões.
```

**Contagem:** 10 tools × 2 targets (agents + skills) = 20 targets de catálogo = 20 linhas. Os 2 "updated" são `git-branch-guard-script` e `credential-guard-script` (targets de script, não catálogo — não passam por `harnessCatalogTarget`). Os 34 "missing" = 20 catálogo + claude-skill + 13 de wiring.

**rc=0** — root.go não escreve nada. As 3320 B são puramente de `harnessWarnMsg` (durante avaliação dos targets, antes do documento).

Com `--install-missing` (env -i, mesmo fixture): rc=0, stdout=2948 B (summary: updated=36), stderr=3320 B (mesmo). `harnessWarnMsg` dispara independentemente de instalar ou não.

**Verificação de escape de home (`--install-missing`):** `find $H_IM -type f | while read f; do [ -e $HOME/$f ] && stat -f '%Sm %N' $HOME/$f; done` encontrou ~230 correspondências de nome. Todos os arquivos correspondentes têm mtime de 2026-10-24 ou anterior — pré-existem de uma instalação anterior via `trackfw update harness`. Verificação de mtime para hoje: `find $HOME -newermt "2026-10-30" -path "*/.trackfw*" -o -path "*/.claude/agents*" ...` = zero arquivos modificados em 2026-10-30 dentro dos caminhos que `update harness` escreve. Conclui-se que `update harness --install-missing` com `env -i HOME=$H_IM` confineu todas as escritas ao `$H_IM`. `go-homedir` honrou `$HOME` da variável de ambiente — nenhum escape para o home real.

`agent_models` NÃO aparece no JSON stdout (`grep -c agent_models out-harness-i = 0`). A mensagem não tem destino no documento atual.

**Cenário de AC1-bis com catalog target failure (inferido — não medido diretamente):** se um target de catálogo falhar (TargetFailed) em ambiente sem config global, o stderr teria: N linhas de `harnessWarnMsg` + a linha de root.go. Após o fix: N linhas de `harnessWarnMsg` apenas. AC1-bis ("stderr 0 B") não seria atingível.

O §3.6 usou `--targets git-branch-guard-script` (target não-catálogo). `harnessWarnMsg` não disparou. Esse é o fixture correto para o teste de AC1-bis.

---

## Seção 4 — Consumidores

Busca por `"violation(s) found"`, `"target(s) failed"`, `validate.violations` em `scripts/`, `.github/`, `internal/` (sem `_test.go` para os dois primeiros):

**`scripts/check-agent-namespace-union.sh:528–533`:**
```bash
# stdout-only (NOT run_*, which merges stderr via 2>&1 — the "N violation(s) found" line goes to
# stderr and would corrupt the JSON payload if merged ...)
case "$runtime" in
  go)  validate_json_stdout=$(cd "$P7" && "$GO_BIN" validate --json 2>/dev/null) ;;
```
O script usa explicitamente `2>/dev/null` e seu comentário cita o defeito pelo nome. É um consumidor que contorna o bug. Após o fix, `2>/dev/null` descarta nada em vez de descartar a linha ruim — compatibilidade mantida.

**`scripts/check-update-parity.sh:113–118, 323–326`:**
`run_update` captura `UPDATE_STDERR=$(cat "$err_file")`. A variável é usada apenas em mensagens diagnósticas de falha (linha 326: `"stderr: $S3_GO_ERR"` quando o exit code não é zero). Nenhum teste analisa o conteúdo de `UPDATE_STDERR` para validação. Após o fix, o valor será vazio nos cenários de sucesso — compatibilidade mantida.

**`.github/workflows/windows-probe.yml:922` (label `validate-json`):** `Start-Process` com `args=@("validate","--json")` captura stdout em arquivo; `fc.exe /b` compara nativo vs shim. Após o fix, o documento JSON é idêntico (o fix não altera o conteúdo do documento). Compatibilidade mantida. Este é o único consumer real de `validate --json` no arquivo.

**`.github/workflows/windows-probe.yml:924–925` — achado separado (fora do escopo desta REQ):** o arquivo contém `@{ label = "context-json"; args = @("context","--json") }` e `@{ label = "validate-strict"; args = @("validate","--strict") }`. Ambas as flags são inválidas — `context` usa `--format=json` (string), `validate` não tem `--strict`. Ambas produzem `rc=1, stdout=0 B` (unknown flag, flag-parse error); `fc.exe /b` compara dois arquivos de 0 bytes → `BYTE_IDENTICAL`. O loop não falha no exit não-zero (apenas reporta `EXIT_MATCH`). Os dois testes produzem um label enganoso: `BYTE_IDENTICAL` para dois arquivos de 0 bytes (duas falhas de flag-parse idênticas), não correspondência de funcionalidade. **Severidade: LOW** — `context-json` e `validate-strict` não são citados em nenhum doc como evidência de paridade (grep em `docs/` confirmado); o label enganoso não mascara paridade documentada. O dado real (`native_exit=1 native_bytes=0`) está visível no log para quem ler, mas o label `BYTE_IDENTICAL` sugere sucesso de paridade quando a execução sequer chegou à lógica do comando. Fix: corrigir para `context --format=json` e remover `validate-strict` (ou usar uma flag válida). Causa: flags erradas no workflow, não o defeito de stream sujo. Não bloqueia AC6 desta REQ.

**`internal/i18n/locales/en-US.json:50`:** `"violations": "{{count}} violation(s) found"` — fonte da mensagem. Usada em `validate.go:38` e `:69`. O fix não altera a i18n.

**`internal/commands/validate_json_test.go`:** nenhuma linha fixa `"violation(s) found"` no stderr. Nenhum `_test.go` encontrado com `"target(s) failed"`.

**Conclusão AC6 (vigente):** nenhum gate, teste ou workflow depende das linhas no stderr que serão suprimidas.

---

## Seção 5 — Alvos de falsificação nas duas direções

### AC1 — `validate --json` com violação: stderr 0 B

**Direção falso-negativo (teste passa, bug regrediu):**
- Sítio de entrada: `json_exit_test.go`, se o teste usar `cmd.SetErr(&bytes.Buffer{})` + `newValidateCmd().Execute()` e afirmar `errBuf.Len() == 0`.
- Mecanismo: root.go escreve em `os.Stderr` (não em `cmd.OutOrStderr()`). A asserção mede o canal errado e passa com ou sem o fix.
- Gate que deveria pegar: teste de subprocess (via `exec.Command` como em `root_test.go`) com `cmd.Stderr = &buf` + `assert buf.Len() == 0`, ou extração de `Execute()` para aceitar um `io.Writer` de stderr.
- Verificação do contra-braço: se o reconhecimento no root.go for revertido (linha de recognição comentada), o teste subprocess deve reprovar. Medido: com o código atual (sem fix), o stderr tem 21 bytes → um teste que afirme 0 bytes FALHARIA. Portanto o teste subprocess é um contra-braço real.

**Direção falso-positivo (teste falha, fix estava correto):**
- Sítio de entrada: root.go, se a recognição usar `errors.As` em vez de type assertion direta.
- Mecanismo: `fmt.Errorf("%w", silentErr)` passado acidentalmente por algum outro caminho seria reconhecido como saída silenciosa → stderr vazio quando deveria ter uma mensagem.
- Gate que deveria pegar: um teste que retorne `fmt.Errorf("wrapped: %w", &ExitCodeError{...})` do RunE e afirme que stderr CONTÉM a mensagem de erro, não está vazio.

### AC1-bis — `update --json` e `update harness --json` com `TargetFailed`: stderr 0 B

**Escopo de AC1-bis (redigido sob decisão D-1 opção (a); se arquiteto escolher (b), este escopo é ampliado):** aplica-se à classe de falha por I/O (chmod, permissão de escrita no diretório) em targets não-catálogo. **Não** se aplica a duas outras classes (ver D-1):
- **Classe pathguard (`update --json` com symlink):** `RejectAndReport` escreve ~388 B em `os.Stderr` por ADR-2026-09-18 D3 ("recusa audível") — veja §3.10 e residual 5.
- **Classe harnessWarnMsg (`update harness --json` com targets de catálogo sem config global):** `harnessCatalogTarget` escreve `harnessWarnMsg` em `os.Stderr` durante avaliação (§3.11). Após o fix, essa classe produz `rc≠0, stdout=JSON, stderr=N×harnessWarnMsg` — a linha de root.go é removida mas o `harnessWarnMsg` permanece.

Consequência para os testes: AC1-bis deve usar fixtures de falha por I/O (chmod 444) em targets não-catálogo (ex: `git-branch-guard-script`, `claude-skill`). Fixtures de catálogo ou de symlink produzem stderr não-zero por classes fora do escopo desta REQ.

**Direção falso-negativo:** mesmos padrões que AC1. `update.go` não usa `cmd.SetErr` em seus testes existentes; um teste em processo que capture `cmd.OutOrStderr()` sofre do mesmo problema — root.go escreve em `os.Stderr`.

**Direção falso-positivo:** como AC1.

### AC7 — modo texto e guard inalterados

**Direção falso-negativo (texto silenciado acidentalmente):**
- Sítio: `validate.go:69` (ramo texto). Se o fix erroneamente substituir o `return fmt.Errorf(...)` do ramo texto pelo erro tipado, o modo texto passa a sair silencioso.
- Gate que deveria pegar: teste subprocess de modo texto que afirme `stderr != ""` (ou que contenha `"Error: "`).

**Direção falso-positivo (guard silenciado):**
- Sítio: root.go, se a recognição do erro tipado rodar sem verificar `isCommandUnderGuard(cmd)`.
- Mecanismo: um comando sob guard retorna `*ExitCodeError{Code: 1}` → exit 1 em vez de exit 2.
- Gate que deveria pegar: teste subprocess de um comando guard que afirme `exitCode == 2`.

---

## Seção 6 — Prescrições para o ML-1A e residual declarado

### Prescrições

**P1 — Nome e localização do tipo:** `ExitCodeError` (pacote `commands`), em `internal/commands/root.go` ou em arquivo novo `internal/commands/exit_code_error.go`. Não exportar para fora do pacote `commands`.

```go
// ExitCodeError signals to Execute() that the command already wrote its
// output document and only the exit code is needed. Execute() exits with
// Code without printing anything. Code must not be 0.
type ExitCodeError struct {
    Code int
    Msg  string // human message, returned by Error()
}

func (e *ExitCodeError) Error() string { return e.Msg }

// newExitCodeError is the only constructor. It panics on Code == 0 (see T1).
func newExitCodeError(code int, msg string) *ExitCodeError {
    if code == 0 {
        panic("ExitCodeError: code must not be 0")
    }
    return &ExitCodeError{Code: code, Msg: msg}
}
```

A `Error()` retorna a mensagem humana: se o tipo for acidentalmente embrulhado (`fmt.Errorf("%w", ...)`), root.go imprime a mensagem em vez de silenciar — comportamento ruidoso pelo caminho errado (T3 mitigado).

**P2 — Reconhecimento em `root.go` por type assertion direta (não `errors.As`):**

Medido: `errors.As` é fail-open para `errors.Join` e `%w` (T2, T3). Direct assertion não atravessa wrapping.

```go
// Em Execute(), antes do bloco de impressão de erro:
if !isCommandUnderGuard(cmd) {
    if exitErr, ok := err.(*ExitCodeError); ok {
        os.Exit(exitErr.Code)
    }
}
// ... impressão de erro normal + os.Exit(exitCode)
```

A guarda `!isCommandUnderGuard(cmd)` preserva D7: mesmo que um comando guard retorne `*ExitCodeError`, root.go cai no caminho normal e sai com 2 (T6 mitigado).

**Ordenação no fluxo de `Execute()` (`root.go:96–141`):** o bloco de recognição entra DEPOIS de `formatUnknownCommandError` e ANTES da impressão. Isso é seguro porque `*ExitCodeError` só pode ser devolvido por `RunE` — erros de unknown-command e flag-parse nunca chegam ao `RunE`, portanto nunca são `*ExitCodeError`. A verificação `isCommandUnderGuard(cmd)` verifica o comando; para comandos guard (que usam `os.Exit` direto), o `cmd` no argumento de `Execute()` já é o subcomando guard, então a guarda funciona corretamente.

**P3 — Três sítios de substituição — somente pós-documento, com guarda de escrita:**

Cada sítio deve seguir o padrão de **duas etapas em sequência**: (a) escrever o documento e checar o erro de escrita antes de retornar o erro tipado; (b) só retornar o erro tipado silencioso se a escrita teve sucesso. Se a escrita falhou, retornar um `fmt.Errorf` normal (ruidoso) — o documento está perdido e o consumidor precisa saber.

```go
// Padrão correto (pós-documento com guarda):
if _, writeErr := fmt.Fprintln(cmd.OutOrStdout(), string(data)); writeErr != nil {
    return fmt.Errorf("validate: failed to write JSON document: %w", writeErr)
}
return newExitCodeError(1, i18n.T("validate.violations", ...))
```

Sítios:
- `validate.go:35+38` — substituição de `Fprintln` + `return fmt.Errorf` pelo padrão acima
- `update.go:78+85` — idem, mensagem `"trackfw update: %d target(s) failed"`
- `update_harness.go:99+106` — idem, mensagem `"trackfw update harness: %d target(s) failed"`

As linhas `marshalErr` (update.go:75, update_harness.go:94) e `silenceErr` (update.go:62-64) NÃO são substituídas — são pré-documento (T4 mitigado).

**Inferido a partir de §3.9:** sem a guarda, `validate --json 1>&-` produziria `rc=1, stdout=0 B, stderr=0 B` pós-fix — condição ambígua. Com a guarda, produziria `rc=1, stdout=0 B, stderr=N B (write error)` — condição ruidosa e diagnosticável.

**P4 — Testes em `json_exit_test.go` via subprocess:**

Usar o padrão de `root_test.go` (`rootBinary(t)` + `exec.Command`). Estrutura de cada teste:

```go
// AC1: validate --json com violação → rc=1, stderr 0 bytes
cmd := exec.Command(bin, "validate", "--json")
cmd.Dir = violationFixture
var stderr bytes.Buffer
cmd.Stderr = &stderr
runErr := cmd.Run()
// assert: exitErr.ExitCode() == 1
// assert: stderr.Len() == 0  ← este é o pin que falha hoje e passa após o fix
// assert: stdout é JSON válido
```

O contra-braço (P5) garante que o teste não é vacuoso.

**P5 — Contra-braço: regressão deve reprovar:**

O contra-braço já tem evidência empírica: o binário atual (sem o fix) produz 21, 42 e 50 bytes no stderr para os três comandos (§3.1, §3.4, §3.6). Um teste subprocess que afirme `stderr.Len() == 0` FALHARIA contra esse binário. Portanto, o teste é um contra-braço real por construção.

Prescição para ML-1A: durante o desenvolvimento, rodar os testes subprocess novos contra um build com o bloco de recognição em root.go removido e verificar que falham. Documentar o resultado no commit message. ML-2A re-verificará a partir do diff.

**P6 — Teste de vácuo do constructor (T1):**

```go
func TestExitCodeError_ZeroCodePanics(t *testing.T) {
    defer func() {
        if r := recover(); r == nil {
            t.Error("expected panic for Code == 0")
        }
    }()
    _ = newExitCodeError(0, "msg")
}
```

**P7 — Contagem de sítios por grep (não por offset de linha):**

Para evitar o padrão "busca e troca ampla" do T4, adicionar no próprio arquivo de teste um assertion que conta as chamadas a `newExitCodeError` no pacote `commands`:

```go
// TestExitCodeError_SiteCount — pins the number of call sites so a broad
// search-and-replace cannot silently add a pre-document site.
func TestExitCodeError_SiteCount(t *testing.T) {
    // Expected: 3 (validate.go, update.go, update_harness.go)
    // Each new site must be justified with a comment explaining why it is post-document.
    ...
}
```

Implementação: `grep -rn "newExitCodeError" internal/commands/ | grep -v "_test.go" | wc -l` como referência, ou embedding do count como constante auditada.

**P8 — `docs/cli-parity.md` (AC8) — redigido sob D-1 (a):**

Se o arquiteto escolher (b), o segundo parágrafo abaixo sai e o contrato passa a ler "stderr vazio" para todos os comandos com `--json` (exceto erros pré-documento).

Adicionar próximo à seção de `validate --json`:

```
## --json stream contract (post-document)

After the result document has been written to stdout, the process exits with
the document's own exit_code. The root.go error printer writes nothing to
stderr (the result is communicated through the exit code only, not through a
message from root.go).

In-run diagnostics that fire WHILE computing the result (pathguard refusals,
agent-models warnings) may still appear on stderr — they are described in the
command's own documentation and are listed in docs/seguranca/.

This contract applies to: validate --json, update --json, update harness --json.
Errors that occur BEFORE the document is written (marshal failure, config
parse error) are not covered and may still print to stderr.
```

### Residual declarado

1. **`errors.Join(realErr, silentErr)` não é detectável** pela direct assertion sem envolver o tipo de `ExitCodeError`. Se um desenvolvedor escrever `errors.Join(realErr, newExitCodeError(...))`, root.go imprimirá o erro completo (não silenciará) — comportamento correto, mas a intenção do desenvolvedor ("quero sair com este código") será ignorada silenciosamente. Documentar na docstring de `ExitCodeError` que ele não deve ser combinado com outros erros.

2. **Comandos guard não usam `ExitCodeError`** e não deveriam. D7 está preservado pela guarda em P2. Se um futuro ML adicionar `--json` a algum comando guard, essa wave deve ser revista.

3. **Testes em processo (`cmd.Execute()`)** continuam sem capturar o stderr de root.go. O estado anterior ao fix (21 bytes no stderr para `validate --json`) não é detectável pelos testes existentes. Isso é um gap de cobertura pré-existente, não introduzido pelo fix.

4. **Modo texto não é testado por bytes de stderr** em nenhum teste existente. AC7 (modo texto inalterado) é verificado apenas pelo contra-braço dos testes subprocess novos, não por um pin independente.

**D-1 — Decisão do arquiteto (zeus-tf): classes de stderr que persistem após o fix.**

Duas classes de writes em `os.Stderr` coexistem com o documento e persistem após o fix (root.go é removido, elas não):

| classe | comando | fonte | bytes medidos | no documento? |
|---|---|---|---|---|
| pathguard refusal | `update --json` com symlink | `pathguard.RejectAndReport` — ADR-2026-09-18 D3 | ~388 B (§3.10) | sim — campo `message` já contém o texto |
| harnessWarnMsg | `update harness --json` com catalog targets sem config global | `harnessCatalogTarget` linha 2671 | 3320 B / 20 linhas (§3.11) | **não** — `agent_models` ausente do schema JSON |

**Adversário desta decisão (Wave 0):** o arquiteto otimista que fecha AC1-bis como verde com um fixture escolhido a dedo (chmod 444, `git-branch-guard-script`). Enquanto isso, o default de `update harness --json` — sem `--targets`, em qualquer máquina sem `agent_models` configurado globalmente — ainda emite 3320 B no stderr com `rc=0`. O consumidor de máquina que espera "stderr vazio" e recebe bytes encontra esse comportamento na primeira execução real, não na fixture de teste. O fix de root.go não piora isso — o comportamento pré-existe — mas a escolha entre (a) e (b) determina se o contrato P8 é honesto sobre os limites ou não.

**Opções para o arquiteto:**

- **(a) Restringir AC1-bis à classe de falha de I/O em targets não-catálogo (§5 e P8 redigidos sob esta opção):** os testes usam fixtures chmod 444 com `git-branch-guard-script`. P8 lê "root.go não escreve nada; diagnósticos em-run (listados em §1.2) podem aparecer." AC1-bis aprovado com essa qualificação.
- **(b) Eliminar ambas as classes sob `--json`:** `RejectAndReport` e `harnessCatalogTarget` não escrevem em `os.Stderr` quando o comando roda com `--json`. Custos assimétricos: para pathguard, o texto já está no campo `message` (redundância). Para `harnessWarnMsg`, o campo não existe no schema; adicionar implica atualizar o test de pinagem de chaves JSON (`TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract`), o `cli-parity.md` e o ML-1A. (b) é possível mas mais invasivo. Altera ADR-2026-09-18 D3 para pathguard.

**Regra Dura:** o fix de root.go fecha apenas 21/42/50 B. Pathguard e harnessWarnMsg têm mecanismo distinto (não passam por root.go) — separar é permitido. Se o arquiteto quiser (b), é ML adicional nesta REQ, não nova REQ.

**Recomendação hades-tf:** opção (a), com rationale por classe:

- **Classe pathguard (`update --json` com symlink):** opção (a). O campo `message` do documento já contém o texto da recusa. Suprimir o stderr seria eliminar a segunda via de um aviso de segurança (ADR-2026-09-18 D3), para ganho zero de informação.
- **Classe harnessWarnMsg (`update harness --json` com catalog targets sem config global):** opção (a). O stderr é o único canal desta mensagem — `agent_models` não existe no schema JSON. Silenciar sob `--json` descarta informação a menos que o schema cresça, que é o custo de (b). O aviso orienta o usuário a configurar `trackfw.yaml` global; suprimi-lo degrada a experiência sem equivalente no documento.

6. **Stdout fechado (`1>&-`) pós-fix sem guarda: condição ambígua.** Mitigado pela guarda de escrita em P3. Se a guarda não for implementada, `validate --json 1>&-` produz `rc=1, stdout=0 B, stderr=0 B` — indistinguível de uma execução com zero violações e documento vazio, mas com código 1. A guarda converte isso em erro ruidoso.

7. **ver D-1 acima.**
