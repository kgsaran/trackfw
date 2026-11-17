# Revisão de Qualidade — ML-3D: `windows_sh_missing` (REQ-IMP-9)

> Autor: hefesto-tf | Data: 2026-11-16
> Escopo: internal/validator/validator_windows_sh_missing_rule.go,
>         internal/validator/validator_windows_sh_missing_rule_test.go,
>         internal/commands/doctor_windows_sh_missing_test.go,
>         diff de internal/validator/validator.go,
>         diff de internal/commands/doctor.go,
>         diff de internal/integrations/doctor.go,
>         diff de scripts/check-validate-rule-pins.sh,
>         diff de docs/cli-parity.md

## Veredito

**REPROVA COM CORRETIVOS** — dois achados de severidade MEDIUM (F2 e F3) devem ser corrigidos antes do commit.

Evidência de build e testes:

```
go build ./...                          → rc=0
go vet ./internal/validator/ ./internal/commands/ ./internal/integrations/  → rc=0
go test ./internal/validator/ ./internal/commands/ ./internal/winskips/ -count=1 -timeout 4m
  ok github.com/kgsaran/trackfw/internal/validator   39.324s
  ok github.com/kgsaran/trackfw/internal/commands    63.832s
  ok github.com/kgsaran/trackfw/internal/winskips    0.159s
make build && GO_BIN=bin/trackfw bash scripts/check-validate-rule-pins.sh → rc=0
  "validate-rule-pins: all 40 pins pass"
  pin32 OK: GOOS=darwin → windows_sh_missing silent (GOOS guard)
```

Testes e build passam. Os dois problemas bloqueantes estão no que os testes NÃO exercitam.

---

## Achados

### F1 — LOW/INFO | Q1: `.husky/*` — listagem per-spec, mas premissa não medida

**Questão:** git hooks no Windows são executados pelo `sh` embutido do Git for Windows,
independente do PATH do usuário? Se sim, listar `.husky/pre-commit` e `.husky/commit-msg` é
aviso falso.

**Evidência coletada:**

`.lefthook/commit-msg/trackfw-req-check.sh` está **corretamente listado**: o `lefthook.yml`
gerado tem `runner: sh` (scaffold.go:1834:
`"\ncommit-msg:\n  scripts:\n    \"trackfw-req-check.sh\":\n      runner: sh\n"`).
Lefthook chama `sh` do PATH diretamente como processo de usuário — não está dentro do
subshell que o git cria para rodar o hook. Se `sh` não está no PATH do processo de
usuário, `runner: sh` falha.

`.husky/pre-commit` e `.husky/commit-msg`: gerados com `#!/bin/sh` (scaffold.go:1887).
O vault note `path-curado-no-windows-tres-medicoes-que-invertem-o-diagnostico-2026-09-24.md`
§2 mede que `git fetch` em remoto local morre com `0xC0000005` quando `sh` está ausente
do PATH — porque git spawna o transporte via shell. Mas hook execution via shebang é
mecanismo diferente: o wrapper `Git/bin/bash.exe` do Git for Windows seta PATH incluindo
`usr\bin` antes de chamar o interpretador shebang, o que tornaria sh disponível mesmo
sem estar no PATH interativo do usuário.

Busca primária (deepwiki lefthook, TIL post, gitforwindows.org/git-wrapper): resultados
conflitantes. A documentação confirma que o wrapper Git for Windows seta PATH, mas não
há medição direta de `#!/bin/sh` hook sem `sh` no PATH do usuário.

**REQ-IMP-5 (threat model §554):** "conveniência (attention-*.sh, trackfw-validate.sh,
lefthook runner sh, husky) — aceito declarar via validate/doctor que não executa sem Git
Bash." A listagem atual é per-spec.

**Corretivo:** Wave 4 VM deve medir se `.husky/pre-commit` executa em Windows com
`sh` ausente do PATH interativo do usuário (Git for Windows instalado mas sem
`usr\bin` no PATH). Se executar → remover `.husky/pre-commit` e `.husky/commit-msg` de
`shConvenienceArtifacts` (validator_windows_sh_missing_rule.go:28-35) e atualizar
cli-parity.md. Sem essa medição, não há base para remover nem para confirmar.

---

### F2 — MEDIUM | Q4: `TestDoctorWindowsShMissing_FindingsEmitted` é vacuoso (Regra Dura de Reconciliação)

**Arquivo:** `internal/commands/doctor_windows_sh_missing_test.go`
**Função:** `TestDoctorWindowsShMissing_FindingsEmitted`
**Conclusão declarada (comentário do teste):** "CheckWindowsShMissingForDoctor drives the
doctor findings correctly."

**A conclusão é falsa.** O teste nunca chama `runDoctor`. Ele chama
`validator.CheckWindowsShMissingForDoctor(root)` (que é testar o pacote validator, já
coberto por `TestWindowsShMissing_Warning_NamesArtifact`), depois **constrói os findings
manualmente com seu próprio loop**:

```go
var findings []integrations.DoctorFinding
for _, a := range artifacts {
    findings = append(findings, integrations.DoctorFinding{
        FindingKind: integrations.DoctorWindowsShMissing,
        Destination: a,
        Remedy:      "sh not found in PATH ...",
    })
}
```

As asserções seguintes (`len(findings) != 2`, `FindingKind`, `Remedy`) testam a
construção do próprio teste, não o código de produto.

**Prova de vacuidade:** deletar `doctor.go:124-132` (o loop `for _, rel := range
validator.CheckWindowsShMissingForDoctor(...)`) e mudar o `Remedy` para texto diferente
em `doctor.go:131` — o teste permanece verde em ambos os casos.

Violação da Regra Dura de Reconciliação: o artefato entregue (teste) não sustenta a
conclusão declarada.

O `trackfw.yaml` no fixture (`makeDoctorShFixture`) é setup morto — `config.Load()`
nunca é chamado pelo teste.

**Corretivo exato:**
Arquivo: `internal/commands/doctor_windows_sh_missing_test.go`
Função: `TestDoctorWindowsShMissing_FindingsEmitted`

Substituir a lógica atual por uma chamada a `runDoctor(false)` a partir de um
fixture que seja um root de git válido. O padrão está em `doctor_test.go` (teste do
stray-agent-memory usa `makeGitRoot` + `runDoctor`). O fixture já está parcialmente
correto (`makeDoctorShFixture` cria os artefatos). O que falta:
1. O root deve ser um repositório git (`git init $root`) para que `manager.ProjectRoot`
   seja resolvido.
2. Chamar `runDoctor(false)` em vez de `validator.CheckWindowsShMissingForDoctor`.
3. Assertar nos findings retornados: pelo menos um com `FindingKind ==
   integrations.DoctorWindowsShMissing`, `Destination` = o artefato presente, e `Remedy`
   contendo "sh not found in PATH".

O teste `TestDoctorWindowsShMissing_SilentOnNonWindows` e
`TestDoctorWindowsShMissing_PrintReport` não são vacuosos — podem permanecer.

---

### F3 — MEDIUM | Q5: Nenhum sítio de fiação em `validator.go` é exercitado pelo stack completo

**Achado:** deletar as duas linhas abaixo de `validator.go` deixaria todos os 40 pins e
todos os testes existentes verdes:

```go
// validator.go:1326
applyRuleWarnOnly("windows_sh_missing", validateWindowsShMissing(), &warnings)

// validator.go:1884
applyRuleWarnOnlyTagged("windows_sh_missing", validateWindowsShMissing(), &warnings)
```

**Por quê:** os testes unitários existentes chamam `checkWindowsShMissing()` e
`validateWindowsShMissing()` diretamente — não `ValidateUnfiltered()` nem o caminho
`--json`. Os dois pins novos (PIN32, PIN33) verificam apenas silêncio; ambos passam
quando a regra não existe.

**Precedente disponível:** `TestADRReqHasContent_VetorJ_FiacaoValidateUnfiltered`
(`internal/validator/validator_adr_req_has_content_test.go`) chama `ValidateUnfiltered()`
com seams injetados e asserta que a regra aparece nos warnings. É o padrão exato
necessário.

**Corretivo exato (mínimo):**
Arquivo: `internal/validator/validator_windows_sh_missing_rule_test.go`
Adicionar função:

```go
// TestWindowsShMissing_FiacaoValidateUnfiltered — verifica que a regra está fiada nos
// dois sítios de validator.go (ValidateUnfiltered e validateUnfilteredTagged).
// (ML-3D conclusão: a regra chega ao chamador via o stack completo do validator.)
func TestWindowsShMissing_FiacaoValidateUnfiltered(t *testing.T) {
    origGOOS := CurrentGOOS
    origFn := ShLookPathFn
    origDir, _ := os.Getwd()
    t.Cleanup(func() {
        CurrentGOOS = origGOOS
        ShLookPathFn = origFn
        _ = os.Chdir(origDir)
    })

    root := makeShFixture(t, "scripts/trackfw-validate.sh")
    if err := os.Chdir(root); err != nil {
        t.Skipf("cannot chdir to fixture: %v", err)
    }
    CurrentGOOS = "windows"
    ShLookPathFn = shAbsent

    // Sítio 1: ValidateUnfiltered
    _, warnings, err := ValidateUnfiltered()
    if err != nil {
        t.Fatalf("ValidateUnfiltered: %v", err)
    }
    var found bool
    for _, w := range warnings {
        if strings.Contains(w, "windows_sh_missing") {
            found = true
            break
        }
    }
    if !found {
        t.Errorf("ValidateUnfiltered must include windows_sh_missing warning; got: %v", warnings)
    }

    // Sítio 2: validateUnfilteredTagged (--json path)
    _, warningsTagged, err2 := ValidateUnfilteredTagged()
    if err2 != nil {
        t.Fatalf("ValidateUnfilteredTagged: %v", err2)
    }
    var foundTagged bool
    for _, w := range warningsTagged {
        if strings.Contains(w.Rule, "windows_sh_missing") {
            foundTagged = true
            break
        }
    }
    if !foundTagged {
        t.Errorf("ValidateUnfilteredTagged must include windows_sh_missing; got: %v", warningsTagged)
    }
}
```

Nota: `ValidateUnfilteredTagged` pode não estar exportada. Checar `export_test.go` ou
expor via `export_test.go` seguindo o padrão existente. O TaggedMsg tem campo `Rule`
(verificar struct). Se não existir export, usar a CLI via `exec.Command` com
`--json` e assertar no JSON (como `TestADRReqHasContent_VetorJ`).

---

### F4 — INFO | Q5: REQ-IMP-9 cobertura parcial (conformance gap)

**Evidência:** REQ-IMP-9 (threat model §580): "regra que detecta ausência de sh no
PATH e emite aviso (não erro) com o texto do shMissingMsg. Esta é uma verificação
proativa: o consumidor não precisa rodar barrier para saber que sh está faltando."

A implementação só avisa quando `sh` está ausente E há artefatos presentes.
`TestWindowsShMissing_NoArtifacts_Silent` asserta explicitamente silêncio sem artefatos.

O barrier falha com sh ausente mesmo sem artefatos (barrier.go:311,1462). O aviso
proativo descrito na REQ não cobre esse cenário. O texto emitido também difere do
`shMissingMsg` do barrier ("gates not evaluated: sh not found in PATH — install a POSIX
shell (e.g. Git Bash, WSL) to evaluate gates").

**Por que não bloqueia:** o ML-3D (roadmap) definiu escopo mais restrito que a REQ —
seus critérios de aceite só mencionam artefatos. A implementação atende os critérios do
ML. Esta é uma dívida técnica documentada, não uma regressão do ML.

**Encaminhamento:** abrir novo ML na REQ vigente para: (a) emitir aviso mesmo sem
artefatos quando sh ausente no Windows (cobre o barrier); (b) alinhar o texto com
`shMissingMsg` do barrier.go.

---

### F5 — INFO | Q3: Root path diverge entre validate e doctor

**Evidência:**
- `validateWindowsShMissing()`: usa `os.Getwd()` (linha 44 de
  validator_windows_sh_missing_rule.go)
- doctor: usa `manager.ProjectRoot` resolvido via `git rev-parse --show-toplevel`
  (doctor.go:126)

Se o usuário roda `trackfw validate` de um subdiretório, validate não encontra
`.husky/pre-commit` na raiz e fica silencioso; doctor encontra via ProjectRoot e avisa.

**Padrão existente:** `validateProjectForeignWriteWindows` (validator.go:98) também usa
`os.Getwd()`. Esse comportamento é consistente em toda a camada validate. Não é um
defeito novo do ML-3D — é o comportamento esperado de todas as regras de arquivo no
validate. Registrado como informação, sem corretivo para ML-3D.

---

### F6 — INFO | Q3: `rules: windows_sh_missing: off` silencia validate mas não doctor

**Evidência:** `internal/commands/doctor.go` e `internal/integrations/doctor.go` nunca
leem o bloco `rules:` do `trackfw.yaml`. Nenhum outro finding do doctor respeita `rules:`.
Comportamento pré-existente e consistente. Sem corretivo para ML-3D.

---

## Q2: Design — `ShLookPathFn` e `CurrentGOOS` como seams exportados

**Padrão existente:** `CurrentGOOS` já existe como `var` mutável em `internal/validator/goos.go:21`
e é usado em testes de outros arquivos do pacote (validator_credential_guard_test.go:61,69,
validator_git_branch_guard_test.go:76). O novo `ShLookPathFn` segue o mesmo padrão.

**Necessidade de exportação:** `ShLookPathFn` precisa ser exportado porque
`doctor_windows_sh_missing_test.go` está em `package commands` (pacote diferente) e precisa
injetar o seam. Mesmo com o corretivo do F2 (chamar `runDoctor`), a injeção ainda é
necessária porque `runDoctor` chama `validator.CheckWindowsShMissingForDoctor` que usa
`ShLookPathFn` internamente.

**home_divergence_windows** usa um padrão diferente: injeta via
`agentconfig.SetPlatformForTest` (interface no pacote agentconfig). Não é melhor nem
pior — depende de onde a dependência mora. Para `ShLookPathFn`, a dependência é
`exec.LookPath` (stdlib), sem camada de abstração intermediária. O padrão de var mutável
é justificado.

**Risco de corrida:** nenhum dos novos testes usa `t.Parallel()`. Consistente com todos
os testes existentes que mutam `CurrentGOOS`. Sem risco adicional.

---

## Q3: Paridade validate × doctor (simetria dos dois sítios)

**Ambos os sítios de `validator.go` estão simétricos:**

```go
// ValidateUnfiltered (linha 1326):
applyRuleWarnOnly("windows_sh_missing", validateWindowsShMissing(), &warnings)

// validateUnfilteredTagged (linha 1884):
applyRuleWarnOnlyTagged("windows_sh_missing", validateWindowsShMissing(), &warnings)
```

Mesma função (`validateWindowsShMissing()`), mesmo rule id, mesmo routing
(`applyRuleWarnOnly`/`applyRuleWarnOnlyTagged`). Ambos têm comentário de mirror-site
com alerta 🔴. `ruleDefaults["windows_sh_missing"] = "warning"` presente (linha 256).

**Texto/remédio:** a frase-chave "sh not found in PATH — install a POSIX shell (e.g. Git
Bash, WSL)" é consistente entre validate warning, doctor Remedy, e cli-parity.md. O
sufixo difere por contexto ("to run .sh convenience scripts" vs "to execute this script"),
o que é aceitável.

---

## Resumo dos corretivos obrigatórios

| ID | Severidade | Arquivo | Corretivo |
|----|-----------|---------|-----------|
| F2 | MEDIUM | `internal/commands/doctor_windows_sh_missing_test.go` | Reescrever `TestDoctorWindowsShMissing_FindingsEmitted` para chamar `runDoctor(false)` em fixture com `git init` e assertar nos findings reais |
| F3 | MEDIUM | `internal/validator/validator_windows_sh_missing_rule_test.go` | Adicionar `TestWindowsShMissing_FiacaoValidateUnfiltered` que chama `ValidateUnfiltered()` e o caminho JSON com seams injetados |

Depois dos corretivos: re-executar `go test ./internal/validator/ ./internal/commands/ -count=1`
e confirmar que os novos testes passam (não apenas não regridem).
