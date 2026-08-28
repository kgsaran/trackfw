# Barreira de Segurança — bit de execução dos artefatos de scaffold

> Produzido por: `hades-tf` | Data: 2026-08-28
> REQ: `docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold.md`
> Escopo: reverificação final pós-ML-2A. Bloqueia release 7.3.0.

---

**VEREDITO: APROVADO com dois resíduos nomeados e aceitos, ambos já registrados no roadmap.**

O entregue fecha o ciclo completo: detecção → finding `scaffold-wrong-mode` → remédio `trackfw update` → bit restaurado. O cenário 181 prova que remover o `os.Chmod` do update quebra o ciclo — a barreira está armada na direção certa.

---

## Pergunta 1 — AC9 fechado nos 5 pontos de Go e Node?

**Medido, não amostrado.**

### Go (`internal/generators/scaffold.go`)

| Artefato | WriteFile | Chmod |
|---|---|---|
| `scripts/trackfw-validate.sh` | L739 `WriteFile(path, content, 0755)` | L747 `os.Chmod(path, 0755)` |
| `scripts/trackfw-attention-signal.sh` | L833 `WriteFile(signalPath, ..., 0755)` | L834 `os.Chmod(signalPath, 0755)` |
| `scripts/trackfw-attention-cleanup.sh` | L843 `WriteFile(cleanupPath, ..., 0755)` | L848 `os.Chmod(cleanupPath, 0755)` |
| `scripts/trackfw-credential-guard.sh` | L880 `WriteFile(path, ..., 0755)` | L884 `os.Chmod(path, 0755)` |
| `scripts/trackfw-git-branch-guard.sh` | L1204 `WriteFile(path, ..., 0755)` | L1208 `os.Chmod(path, 0755)` |

Resultado do `grep -n "Chmod" scaffold.go`: exatamente 5 linhas (747, 834, 848, 884, 1208). Cada uma segue imediatamente o `WriteFile` correspondente. Nenhum ponto faltando.

### Node (`npm/src/generators/init.js` + `hooks.js`)

| Artefato | writeFileSync | chmodSync |
|---|---|---|
| `scripts/trackfw-validate.sh` | init.js:L125 `{mode:0o755}` | init.js:L129 `chmodSync(scriptPath, 0o755)` |
| `scripts/trackfw-git-branch-guard.sh` | hooks.js:L1049 `{mode:0o755}` | hooks.js:L1051 `chmodSync(scriptPath, 0o755)` |
| `scripts/trackfw-credential-guard.sh` | hooks.js:L1086 `{mode:0o755}` | hooks.js:L1090 `chmodSync(scriptPath, 0o755)` |
| `scripts/trackfw-attention-signal.sh` | hooks.js:L1491 `{mode:0o755}` | hooks.js:L1498 `chmodSync(signalPath, 0o755)` |
| `scripts/trackfw-attention-cleanup.sh` | hooks.js:L1494 `{mode:0o755}` | hooks.js:L1503 `chmodSync(cleanupPath, 0o755)` |

Resultado do `grep -n "chmodSync" init.js hooks.js`: exatamente 5 linhas. AC9 fechado nos dois runtimes.

### Existe um sexto ponto de escrita?

Sim. `internal/discover/discover.go:writeValidateScript` (função local, chamada em L51 por `InstallGates`):

```go
// discover/discover.go:L77–84
func writeValidateScript(rootDir string) error {
    content := "#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n"
    dest := filepath.Join(scriptsDir, "trackfw-validate.sh")
    if err := os.WriteFile(dest, []byte(content), 0755); err != nil { ... }
    return nil  // sem os.Chmod posterior
}
```

O conteúdo é idêntico ao `pythonValidateScriptForm` do doctor (`scaffold_doctor.go:L39`) — o doctor aceitaria. Mas numa reescrita sobre arquivo existente com modo errado, o bit não é restaurado. Os outros quatro artefatos emitidos por `InstallGates` (`GenerateAttentionScripts`, `GenerateCredentialGuardScript`, `GenerateGitBranchGuardScript`) usam as funções centrais de `generators/`, que já têm o `os.Chmod`.

**Status:** este sexto ponto foi nomeado no roadmap como resíduo aceito (auditoria ML-1A: "discover.go:83 escreve o validate script sem Chmod posterior"). A cadeia de remédio funciona: `doctor` detecta `scaffold-wrong-mode` → usuário roda `trackfw update` → `generateValidateScript` em `generators/scaffold.go` inclui o `os.Chmod`. O gap em `discover --init` não é invisível: o doctor o captura na próxima execução.

---

## Pergunta 2 — Os dois resíduos nomeados continuam aceitáveis?

### (a) Guarda de Windows com teste unitário só no Go

**Medido:**

Go (`scaffold_doctor_test.go`):
- `TestWindowsPlatformGuard` (linha ~238): injeta `CurrentGOOS = "windows"` com defer de restauração; verifica supressão do modo.
- `TestWindowsPlatformGuardNonExecutable` (linha ~268): idem para artefatos com `execBit=false`.

Node (`npm/src/integrations/scaffold_doctor.js`):
- `_setPlatformForTest` existe (linha ~77–86) — função exportada que troca `_platform` e retorna restaurador.
- `grep -rn "_setPlatformForTest" npm/tests/` → **sem resultado**. Nenhum teste chama esta função.
- Os testes de Node existentes só têm `process.platform !== 'win32'` como guard de skip para execução nativa em Windows; não simulam Windows no Linux/macOS.

**Avaliação:** o resíduo é real. A implementação da guarda em Node é correta (`_platform !== 'win32'` em scaffold_doctor.js:198 e :257), mas a cobertura de teste é mais fraca que em Go. O parity script (`check-doctor-parity.sh`) valida o comportamento funcional nos três runtimes via fixtures, incluindo o cenário `scaffold-wrong-mode-detected`. Um teste unitário isolado que invoca `_setPlatformForTest('win32')` seria o reforço correto — mas não é condição de bloqueio desta release. O resíduo está nomeado; a barreira o registra aqui, não o eleva.

### (b) `discover.go:83` escreve validate script sem Chmod posterior

Confirmado em medição (ver Pergunta 1). A cadeia de remédio fecha o loop:

```
discover --init (arquivo existente, modo errado)
    → trackfw-validate.sh fica com modo 0644
    → trackfw doctor → scaffold-wrong-mode finding
    → usuário: trackfw update
    → generateValidateScript (scaffold.go:L739 + L747 os.Chmod) → 0755 restaurado
    → trackfw doctor → no mismatches
```

O `discover --init` não é o caminho de remédio — é um caminho de inicialização brownfield. O resíduo é aceito porque o ciclo de detecção e correção está intacto.

---

## Pergunta 3 — A verificação de modo abriu falso-positivo novo?

**Resposta: não.**

Todos os casos de interesse medidos ou demonstrados por lógica direta:

| Modo | `mode & 0o100` | Doctor | Esperado |
|---|---|---|---|
| `0755` | `0100 != 0` | silencioso | correto |
| `0750` | `0100 != 0` | silencioso | correto — umask 027 produz 0750 |
| `0700` | `0100 != 0` | silencioso | correto — **medido**, parity scenario `scaffold-0700-mode-accepted-ac10` passou |
| `0555` | `0100 != 0` | silencioso | correto — bit de execução do dono presente |
| `04755` (setuid) | `0100 != 0` | silencioso | correto — setuid não interfere no bit de execução |
| `0644` | `0` | `scaffold-wrong-mode` | correto — **medido**, parity scenario `scaffold-wrong-mode-detected` passou |
| `0444` | `0` | `scaffold-wrong-mode` | correto — sem execução |

**Arquivo de outro dono:** `stat().Mode()` retorna os bits do inode independente do chamador. Um arquivo com `0644` pertencente a outro usuário seria acusado de `scaffold-wrong-mode` — correto, porque `./scripts/trackfw-validate.sh` não funcionaria sem o bit de execução do dono.

**`noexec` mount:** `stat()` retorna `0755` corretamente; doctor fica silencioso. O script não executaria, mas a verificação de flags de montagem está declarada no Residual-1 do modelo de ameaça e não é coberta.

**`core.fileMode=false`:** doctor lê `stat()` no filesystem — enxerga o modo real independente da config do git. Não afeta a detecção. O residual-4 diz respeito à entrada silenciosa no histórico git, não à detecção.

---

## Pergunta 4 — `scaffold-wrong-mode` pode mascarar divergência de conteúdo?

**Não. O código é explícito na ordem de verificação.**

Go (`scaffold_doctor.go:L313–325`):
```go
if !bytes.Equal(actual, expected) {
    // Content diverges — takes precedence over any mode issue
    // (at most one finding per artifact; update fixes both content and mode anyway).
    return &DoctorFinding{FindingKind: DoctorScaffoldDivergent, ...}
}
// Content matches. Check the execute bit when required.
if execBit && CurrentGOOS != "windows" && !execBitPresent(path) {
    return &DoctorFinding{FindingKind: DoctorScaffoldWrongMode, ...}
}
```

Node (`scaffold_doctor.js:L244–257`) tem a mesma sequência com o mesmo comentário.

**Consequência para a pergunta:** arquivo com conteúdo errado E bit ausente → emite `scaffold-divergent`, não `scaffold-wrong-mode`. Não há mascaramento: o pior dos dois é o que aparece. E o remédio `trackfw update` restaura tanto o conteúdo (`WriteFile`) quanto o modo (`os.Chmod`), portanto a resolução de `scaffold-divergent` conserta os dois. O usuário nunca fica com conteúdo correto e modo ainda errado após um update bem-sucedido.

---

## Pergunta 5 — O `doctor` mente em algum caso novo?

**Nenhuma nova mentira encontrada.**

A única mudança comportamental introduzida é: arquivo com conteúdo correto e bit ausente antes era silencioso (falso-negativo). Agora emite `scaffold-wrong-mode` (verdadeiro-positivo). Esta é a correção pretendida.

Casos verificados em que o doctor poderia potencialmente mentir:

- **Falso-positivo de modo:** eliminado pelo discriminante `& 0o100` em vez de `== 0755`. Medido via `scaffold-0700-mode-accepted-ac10`.
- **`scaffold-wrong-mode` em artefato não-executável:** impossível — `execBit=false` é hard-coded para os 12 artefatos que o gerador escreve em `0644`; a verificação de modo é guardada por `if execBit && ...`.
- **Silêncio indevido em arquivo correto:** o parity script `scaffold-backend-go-no-false-positive` confirma que scaffold íntegro (conteúdo correto + modo 0755) resulta em "no mismatches found".
- **Falso-divergente em arquivo ilegível:** Go e Node retornam `scaffold-divergent` em erro de leitura. Comportamento pré-existente, sem mudança.

```
bash scripts/check-doctor-parity.sh  →  All check-doctor-parity.sh scenarios passed.
181 cenários (make quality — medidos pelo arquiteto antes da entrega)
trackfw validate: 0 violations, 16 warnings pré-existentes
```

---

## Resumo executivo

| Pergunta | Resultado |
|---|---|
| AC9 nos 5 pontos de Go | FECHADO — 5 `os.Chmod` medidos |
| AC9 nos 5 pontos de Node | FECHADO — 5 `fs.chmodSync` medidos |
| Sexto ponto (discover.go) | NOMEADO como resíduo aceito; cadeia de remédio funciona |
| Guarda de Windows Go | Teste unitário presente (`TestWindowsPlatformGuard`) |
| Guarda de Windows Node | Implementação correta; teste unitário ausente — resíduo aceito |
| Falsos-positivos de modo | Nenhum novo (0700, 0750, 0555, setuid aceitos) |
| Mascaramento conteúdo por modo | Impossível — conteúdo tem precedência, código explícito |
| Nova mentira do doctor | Nenhuma encontrada |

**Resíduos aceitos e registrados (não são bloqueantes para a release 7.3.0):**
1. `discover/discover.go:writeValidateScript` sem `os.Chmod` posterior — cadeia de remédio fecha o loop.
2. Guarda de Windows no Node sem teste unitário que injete `_setPlatformForTest('win32')` — implementação correta, cobertura de teste mais fraca que Go.
