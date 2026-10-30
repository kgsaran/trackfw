# Red Team — Wave 2, ML-2A: saída silenciosa `--json` pós-documento

> Roadmap: `ROADMAP-2026-09-08-validate-suprime-usage-em-erro-de-runtime-e-o-gate-que-provaria-isso.md` · Wave 2 · ML-2A
> Data: 2026-10-30 · Revisor: hades-tf
> Wave 0: `docs/seguranca/2026-10-30-wave0-json-stderr-saida-silenciosa.md`

---

## Veredito geral

**REPROVA para marcação de ML-1A como ✅.** Comportamento de produção correto. Dois artefatos de teste falham a Regra Dura de Reconciliação do projeto: F3 (teste declara cobrir root.go mas nunca execita Execute()) e P6 (sentença declarada é falsificada pela medição de Code=256). F1 é escopo novo derivado da prescrição P1 incompleta da Wave 0 — não é uma falha do implementador; entra como ML adicional. O arquiteto decide o merge.

---

## Metodologia

Binário compilado em `/private/tmp/claude-501/tfw-current` de `git archive HEAD` (cópia isolada). Mutações em cópia separada. Fixtures em `mktemp` com `HOME=<fakehome>` explícito. Limpeza completa antes do handoff.

**Incidentes de execução acidental na árvore real (ambos read-only):**

1. Primeira invocação de `validate --json` sem `cd` para o fixture: rodou contra o CWD do shell, retornou rc=0 (163 warnings em modo lenient do repo real).
2. Segunda tentativa com path com espaço inicial (`/ private/tmp/...`): `cd` falhou; binário rodou novamente contra CWD com mesmo resultado.

`git status --porcelain` confirmou árvore limpa antes e depois. Dano: nenhum.

**Tentativa de "sujar stdout/stderr" — invocações padrão:**

Dois comandos executados com binário atual (sem `--targets` reduzido):

- `update --json` (sem `--targets`) em fixture mínimo: rc=0, stdout=915 B JSON, stderr=0 B. Documento válido; stdout limpo.
- `update harness --json --install-missing` (env -i): rc=0, stdout=2948 B JSON, stderr=3320 B (20 linhas de `harnessWarnMsg`). Todas as linhas de stderr pertencem à classe D-1(a) declarada na Wave 0 §3.11. Documento válido; nenhuma linha nova de root.go.

Nenhum write não declarado encontrado nos dois caminhos padrão.

---

## Prescrições P1–P6, P8

### P1 — Constructor panics on Code=0 — PARCIAL ⚠

O construtor implementado é idêntico à prescrição P1 da Wave 0 (`if code == 0 { panic(...) }`). **O gap deriva da prescrição P1 incompleta, não do implementador.**

**Finding 1a — Code=256 via construtor normal (Finding F1 na tabela):**

`newExitCodeError(256, ...)` passa o pânico (256 ≠ 0). `os.Exit(256)` no Unix (macOS, medido) retorna rc=0 (baixos 8 bits: 256 mod 256 = 0). Em Windows, exit codes são 32-bit e o resultado seria 256. `Execute()` não valida `exitErr.Code` antes de `os.Exit(exitErr.Code)`.

```
$ go build -o tfw-mutant-256 (newExitCodeError(256, "code-256") em validate.go)
$ cd fixture && HOME=fakehome tfw-mutant-256 validate --json
rc=0  stderr=0 B  (4 violations presentes — fail-open confirmado)
```

**Finding 1b — Struct literal Code=0 (zero-value) dentro do pacote:**

A forma mais provável do implementador apressado: `&ExitCodeError{Msg: "..."}` — campo `Code` não inicializado = 0. `Execute()` chama `os.Exit(0)`.

```
$ go build -o tfw-mutant-t1 (&ExitCodeError{Code: 0} em validate.go)
$ cd fixture && HOME=fakehome tfw-mutant-t1 validate --json
rc=0  stderr=0 B  (4 violations presentes — fail-open confirmado)
```

**Severidade: LOW.** 3 sítios atuais; todos com `newExitCodeError(1, ...)`. Brecha estrutural, não produção imediata.

**Correção unificada (parte do `decideExit` — ver F1/F2/F3):**

```go
// Dentro de decideExit:
if !isCommandUnderGuard(cmd) {
    if exitErr, ok := err.(*ExitCodeError); ok {
        if exitErr.Code >= 1 && exitErr.Code <= 255 {
            return exitErr.Code, ""
        }
        // Code fora de [1,255]: cai no caminho ruidoso
    }
}
```

**Entra como ML adicional em REQ-2026-09-07 (Regra Dura de Causa Raiz).**

---

### P2 — Direct type assertion — PASSA ✓

`root.go:129`: `if exitErr, ok := err.(*ExitCodeError); ok`. Direct assertion, não `errors.As`. Guarda `!isCommandUnderGuard(cmd)` em root.go:128 existe no código.

---

### P3 — Write-failure guard — PASSA para EBADF real; gap de plataforma informacional

Código implementado corretamente nos três sítios. Medido:

| comando | `1</dev/null` (EBADF real) | veredito |
|---|---|---|
| `validate --json` | rc=1, stderr=80 B ("validate: failed to write JSON document: …") | ✓ |
| `update --json` | rc=1, stderr=93 B ("trackfw update: failed to write JSON document: …") | ✓ |
| `update harness --json` | rc=1, stderr=101 B ("trackfw update harness: failed to write JSON document: …") | ✓ |

**Caso `1>&-` (closed fd):** Go 1.25.2 (macOS) reabre fd 1 fechado como `/dev/null` (char device, medido via `os.Stdout.Stat()`). Escrita "tem sucesso" (`writeErr=nil`). P3 não dispara. Equivale a `/dev/null` intencional — comportamento correto para descarte.

**Retratação da Wave 0 §3.9:** "Fprintln falha silenciosamente" era uma inferência incorreta. P3 está correto para erros reais de escrita. Residual 6 da Wave 0 é retratado para o caso `1>&-`.

---

### P4 — Testes via subprocess — PARCIAL ⚠

Os testes de AC1, AC1-bis (validate, update, update harness), AC7 e lenient usam `exec.Command` via `runBin()`. O binário é compilado pela suite via `rootBinary(t)` a partir do módulo do projeto. **Esses testes estão corretos.**

**Dois testes são in-process, não subprocess:**

1. `TestJSONExit_NewExitCodeError_ZeroCodePanics` — in-process; chama o construtor diretamente. Correto para esse propósito (função pura; não requer Execute()).
2. `TestJSONExit_WrappedExitCodeError_NotSilenced` — in-process; verifica `wrapped.(*ExitCodeError)`. Ver F3.

O cabeçalho do arquivo afirma "Todos os testes aqui usam exec.Command" — falso para esses dois testes.

---

### P5 — Contra-braço: regressão deve reprovar — PASSA ✓

Mutação A: bloco de reconhecimento de `*ExitCodeError` substituído por no-op em root.go:

```
FAIL TestJSONExit_ValidateJSON_ViolationSilentStderr   — stderr 21 B: "4 violation(s) found\n"
FAIL TestJSONExit_UpdateJSON_TargetFailedSilentStderr   — stderr 42 B
FAIL TestJSONExit_UpdateHarnessJSON_TargetFailedSilentStderr — stderr 50 B
PASS (4 testes restantes)
```

Os três testes de AC1/AC1-bis falham com o binário sem reconhecimento. Contra-braço real e efetivo.

---

### P6 — Teste de pânico no construtor (Code=0) — SENTENÇA FALSA ⚠

O teste `TestJSONExit_NewExitCodeError_ZeroCodePanics` está correto: chama `newExitCodeError(0, ...)` e verifica o pânico. O teste passa.

**Sentença declarada falsificada pela medição:**

O comentário do teste diz: "tornando impossível criar um *ExitCodeError que saia com rc=0 pelo caminho normal." A medição mostra que `newExitCodeError(256, ...)` — caminho normal, sem pânico — produz rc=0 via `os.Exit(256)`. A sentença é falsa.

Esta é a Regra Dura de Reconciliação para P6: o teste passa, mas a conclusão declarada é contraditada pela medição do mesmo ML.

**Fix:** atualizar o comentário do teste para refletir que o pânico impede apenas Code=0; o range [1,255] é verificado em `Execute()` (após a extração de `decideExit`).

---

### P8 — `docs/cli-parity.md` — PASSA ✓

Seção `### --json stream contract (post-document)` adicionada (linha 794). O comment `trackfw-contract:` declara escopo D-1(a). Texto alinhado com comportamento medido.

---

## Verificação dos três comandos (medição direta)

| comando | rc | stdout | stderr | veredito |
|---|---|---|---|---|
| `validate --json` com violações | 1 | 1091 B JSON (violations=4) | 0 B | AC1 ✓ |
| `update --json` com chmod 444 CLAUDE.md | 1 | JSON (failed=1) | 0 B | AC1-bis ✓ |
| `update harness --json` com chmod 555 .trackfw | 1 | JSON (failed=1) | 0 B | AC1-bis ✓ |

---

## D7 — Comando sob guard continua saindo 2

**Caminho medido (flag-parse):**

```
$ trackfw guard write-scope --nonexistent-flag
rc=2  ✓
```

Flag-parse chega ao bloco D7 de `Execute()` independentemente do bloco de reconhecimento de `*ExitCodeError`. D7 está correto para esse caminho.

**Caminho RunE com `*ExitCodeError` (verificado por leitura apenas):** a guarda `!isCommandUnderGuard(cmd)` está no código (root.go:128). Nenhum comando guard usa `newExitCodeError` hoje. Ver F2.

---

## Achados

### F1 — T1 parcial: Code=256 + struct literal (LOW)

Ver P1 acima. Dois caminhos produzem `os.Exit(0)` com violations presentes: (a) `newExitCodeError(256, ...)` via construtor normal; (b) `&ExitCodeError{Msg: "..."}` via struct literal (zero-value). `Execute()` não valida range antes de `os.Exit`.

**Severidade: LOW.** **Correção:** range check [1,255] em `Execute()` via `decideExit` (ver F2/F3).

---

### F2 — D7 ungated: Mutação C passa (LOW)

Mutação C: `!isCommandUnderGuard(cmd)` removido de root.go. Suite `TestJSONExit`: **todos passam**. Não há teste que execute um comando guard cujo `RunE` retorna `*ExitCodeError{Code: 1}` e confirme rc=2.

**Causa compartilhada com F3:** a lógica de decisão de `Execute()` não é isolável do processamento de comandos reais sem extração.

**Severidade: LOW.** **Entra como ML adicional em REQ-2026-09-07.**

---

### F3 — T3 sem gate de subprocess: bloqueia ✅ ML-1A (LOW)

Mutação B: `err.(*ExitCodeError)` trocado por `errors.As(err, &exitErr)` em root.go. Suite `TestJSONExit`: **todos passam**, incluindo `TestJSONExit_WrappedExitCodeError_NotSilenced`.

O teste verifica `fmt.Errorf("%w", inner).(*ExitCodeError)` — propriedade de Go, não de `Execute()`. Com `errors.As` em root.go, um erro embrulhado seria silenciado; o teste não detectaria.

**Regra Dura de Reconciliação:** o comentário do teste declara "a direct assertion de root.go (T3 mitigation) não atravessa wrapping" como conclusão do ML. O teste nunca exercita `Execute()`. Regra: "se não for possível escrever a frase para um teste, o teste não deveria existir."

**Correção unificada para F1/F2/F3 (um ML):** extrair a lógica de decisão de `Execute()` em uma função pura:

```go
// decideExit(cmd, err) → (code int, msg string)
// code != 0 e msg == "" → saída silenciosa
// code != 0 e msg != "" → root.go imprime msg e sai com code
```

Table tests in-process cobrem sem comandos reais:

| entrada | esperado |
|---|---|
| `*ExitCodeError{Code: 1}` (não-guard) | code=1, msg="" |
| `*ExitCodeError{Code: 0}` (zero-value) | code=1, msg=<mensagem> (range inválido, caminho ruidoso) |
| `*ExitCodeError{Code: 256}` | code=1, msg=<mensagem> (range inválido, caminho ruidoso) |
| `fmt.Errorf("%w", exitErr)` | code=1, msg="wrapped: …" (não silenciado) |
| `*ExitCodeError{Code: 1}` + cmd sob guard | code=2, msg=<mensagem> (D7) |

Para a linha de guard: um comando cobra minimal (`root → guard → sub`) embutido no teste é suficiente para `CommandPath()` retornar `"trackfw guard sub"`.

**Severidade: LOW.** Bloqueia ✅ ML-1A. **Entra como ML adicional em REQ-2026-09-07.**

---

### F4 — P3 gap de plataforma para closed-fd (Informacional)

Go 1.25.2 (macOS) reabre fd 1 fechado como `/dev/null`. A escrita "sucede" (`writeErr=nil`). O caso `1>&-` equivale a redirecionamento intencional para `/dev/null` — comportamento correto, não uma falha do fix. Ver P3 acima.

Wave 0 §3.9 estava errado sobre o mecanismo ("Fprintln falha silenciosamente"). Residual 6 da Wave 0 é retratado para esse caso específico.

---

### F5 — Docstring errors.Join: "is recognised" é inverso da realidade (VERY LOW)

`exit_code_error.go:8-9`: "errors.Join(real, *ExitCodeError) is recognised by Execute() as a silent exit."

Com direct assertion: `errors.Join(real, exitErr).(*ExitCodeError)` → false. Não reconhecido. A afirmação descreve o risco com `errors.As` (T2 da Wave 0), não o comportamento atual.

**Fix:** reformular para "If Execute() is ever changed to use errors.As, errors.Join(real, *ExitCodeError) would be silenced."

---

### F6 — Guard windows-probe: rc=0 com 0 bytes passa (VERY LOW)

A condição `$nExit -ne 0` implica que dois arquivos de 0 bytes com rc=0 chegam ao `fc.exe /b` como BYTE_IDENTICAL. O comentário "dois vazios nao podem valer como BYTE_IDENTICAL" é mais amplo que a condição implementada. Aceitável para os 4 labels atuais (nenhum produz rc=0 com 0 bytes em operação normal). **Fix (se desejado):** `if ($nBytes -eq 0 -or $sBytes -eq 0)`.

---

## `windows-probe.yml` — revisão por leitura

Workflow acionado por `workflow_dispatch` apenas (linha 34) — não bloqueia PRs. O job de regressão que bloqueia merge é `windows-full-suites` em `quality.yml`.

**Mudanças corretas:**
- `context --json` → `context --format=json` (flag válida) ✓
- `validate-strict` removido ✓
- Guard adicionado (rc!=0 com 0 bytes → `exit 1`) ✓

**Falso-positivo:** não. Os 4 labels não produzem rc!=0 com 0 bytes em operação normal no fixture trackfw-nul. `exit 1` aborta o loop do probe sob demanda — custo aceitável.

---

## Resumo dos achados

| # | Descrição | Severidade | Ação |
|---|---|---|---|
| F1 | T1: Code=256 via construtor normal + struct literal Code=0; os.Exit sem range check | LOW | ML adicional REQ-2026-09-07 (`decideExit`) |
| F2 | D7 ungated: Mutação C passa; `!isCommandUnderGuard` sem pin de regressão | LOW | ML adicional REQ-2026-09-07 (`decideExit`) |
| F3 | T3 ungated: Mutação B passa; teste in-process nunca exercita Execute() | LOW (bloqueia ✅) | ML adicional REQ-2026-09-07 (`decideExit`) |
| F4 | P3 gap de plataforma: Go 1.25.2/macOS reabre fd 1 como /dev/null; Wave 0 §3.9 retratado | Informacional | Documentar; sem fix prático |
| F5 | Docstring: "is recognised" é inverso do comportamento atual com direct assertion | VERY LOW | Fix de comentário |
| F6 | Guard windows-probe: dois rc=0 empties passam; comentário mais amplo que condição | VERY LOW | Aceitável para 4 labels atuais |

**P6:** sentença declarada falsificada pela medição de Code=256. Atualizar comentário do teste após `decideExit`.

---

## Residual declarado

1. **1>&- pós-fix:** Go reabre fd 1 fechado como /dev/null; P3 não captura esse caso; Wave 0 §3.9 retratado.
2. **T3 ungated:** até extração de `decideExit`, direct assertion não tem gate de regressão para mudança para errors.As.
3. **D7 ungated para ExitCodeError:** guarda existe; sem teste que a pina para o caminho RunE.
4. **Code fora de [1,255]:** fechável com range check em `decideExit`/`Execute()`.

**Nota de vault (pendente — não escrita por restrição de escopo):** Go 1.25.2 (macOS) reabre closed fd 0–2 como /dev/null ao startup; usar `1</dev/null` para EBADF real em testes, não `1>&-`. O arquiteto pode registrar em `vault/notes/`.

**Erratum Wave 0:** §3.9 (premissa de write failure com `1>&-`) e residual 6 ("mitigado pela guarda de P3" para closed-fd) estão incorretos. O arquiteto decide se atualiza o Wave 0 ou deixa o erratum neste documento.
