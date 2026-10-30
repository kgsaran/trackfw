# Red Team — Teste do Invólucro de Atenção sem Symlink (#634)

> Roadmap: ROADMAP-2026-10-30-teste-do-involucro-de-atencao-materializa-o-binario-por-symlink...md
> Data: 2026-10-30 | Revisor: hades-tf (ML-2A)

---

## Tabela de Mutações

| ID | Alvo | Mutação | Resultado | Gate que matou |
|----|------|---------|-----------|----------------|
| M-8 | `attentionSignalScript` | `exit 0` antes da chamada ao trackfw | **MORTA** | `scaffold_test.go:423` — `Arquivo de atenção não foi encontrado` |
| M-9 | `attentionSignalScript` | `trackfw attention signal "$@" </dev/null` (stdin descartado) | **MORTA** | `scaffold_test.go:423` — `Arquivo de atenção não foi encontrado` |
| M-10 | `attentionSignalScript` | `command -v jq >/dev/null || exit 0` adicionado (requer jq) | **MORTA** | `scaffold_test.go:423` — `Arquivo de atenção não foi encontrado` (jq ausente no PATH do filho) |
| M-11 | `scaffold_test.go` (P-2 contra-arm) | `trackfwBinDir = t.TempDir()` (diretório vazio) | **MORTA** | `scaffold_test.go:405` — `setup: trackfw não encontrado no PATH do filho` |

---

## Vereditos por Prescrição (Wave 0)

### P-1 — Remover os.Symlink; usar `filepath.Dir(compiledGuardBinary(t))` como PATH do filho

**APROVADO.**

A implementação usa `trackfwBinDir := filepath.Dir(compiledGuardBinary(t))` e `cmd.Env = []string{"PATH=" + trackfwBinDir}`. Nenhum `os.Symlink`, nenhum `exec.LookPath` para o binário. M-8 e M-9 morrem ao quebrar a chamada ao trackfw ou descartar o stdin — a presença do arquivo de atenção é o critério final.

### P-2 — Verificação de acessibilidade antes de rodar o script (AC3)

**APROVADO.**

A probe é `exec.Command("bash", "-c", "command -v trackfw")` com `probe.Env = []string{"PATH=" + trackfwBinDir}`. Falha com `t.Fatalf` antes da execução do script, nomeando o PATH e o diretório esperado.

M-11 (contra-arm): com `trackfwBinDir = t.TempDir()` (diretório vazio), o teste reprova em `scaffold_test.go:405` — `setup: trackfw não encontrado no PATH do filho` — e não em `scaffold_test.go:423` (ReadFile). A probe cumpre sua função: o diagnóstico aponta a causa real.

### P-3 — Declarar no relatório o que o teste afirma

**APROVADO por invariante.**

O teste afirma: "quando `trackfw` está no PATH do filho e `jq` não está, o invólucro de atenção escreve corretamente o arquivo de atenção."

M-10 confirma: com `jq` exigido pelo script e ausente no PATH do filho, o teste reprova porque o arquivo de atenção não é escrito — provando que o teste detecta a dependência não-declarada.

### P-4 — Declarar AC2 por invariante, não por código

**APROVADO.**

O diretório `filepath.Dir(compiledGuardBinary(t))` contém exatamente um arquivo: o binário `trackfw` produzido por `go build -o guardBinaryPath ./cmd/trackfw` em `TestMain`. Verificado: `jq` reside em `/opt/homebrew/bin/jq` (macOS), nunca no diretório temporário de compilação. O invólucro gerado (`attentionSignalScript`) não referencia `jq` em nenhuma linha — confirmado lendo `scaffold.go:1086–1091`.

AC2 é garantido por construção: PATH do filho contém só o diretório do binário compilado, que nunca inclui `jq`. Não é necessário código adicional.

---

## Achados

Nenhum achado de segurança. As quatro mutações morreram nos gates corretos.

**Nota sobre jq reachable:** o caso "jq no PATH do filho" é esperado passar pelo teste (o invólucro não chama jq, então o arquivo é escrito igualmente). Não é um survivor; é um comportamento correto. Explicitamente: M-10 não testa "jq no PATH do filho" — testa o oposto (script que REQUER jq). São vetores distintos. "jq presente no PATH" não alteraria o resultado porque o script nunca invoca jq.

---

## Residual Declarado

**RD-1 (R-1 da Wave 0):** AC4 — medição na conta Windows sem `SeCreateSymbolicLinkPrivilege` — é declarada como feita pelo ML-1B (zeus-tf) na `Windows-Lab` com a conta `tfnopriv`. O arquivo `docs/portabilidade/2026-10-30-teste-atencao-sem-symlink-medido-na-vm.md` registra o resultado. Fora do escopo desta barreira.

**RD-2 (R-2 da Wave 0):** o invariante de AC2 depende de `attentionSignalScript` não chamar `jq`. Se uma mudança futura adicionar `jq` ao script, o invariante deve ser reaplicado.

---

## `git status --short`

```
 M .claude/agent-memory/hades-tf/MEMORY.md
?? .claude/agent-memory/hades-tf/project_parselsremotesymref_second_field_vector_gap.md
?? docs/seguranca/2026-10-30-redteam-branch-padrao-ls-remote.md
?? docs/seguranca/2026-10-30-redteam-teste-atencao-sem-symlink.md
```

Nenhum arquivo de produto ou teste modificado.
