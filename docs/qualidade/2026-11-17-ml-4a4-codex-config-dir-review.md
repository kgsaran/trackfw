# Revisão de Qualidade — ML-4A4: `CodexConfigDir` / guard global do Codex

**Data:** 2026-11-17
**Revisor:** hefesto-tf
**Branch:** `feat/cobertura-e-compatibilidade-completas-do-trackfw-no-windows`
**Veredito:** APROVA COM NOTAS

---

## 1. Escrita e auditoria usam o mesmo resolvedor

**Verificação:** `grep -rn '\.codex' internal --include='*.go' | grep -v _test | grep -v '^internal/agentconfig/'`

Resultado:

```
internal/validator/validator_credential_guard.go:76:  {".codex/hooks.json", "Codex CLI", ...}
internal/validator/validator_guard_wiring.go:92:  ".codex/hooks.json": hookCmdSchemaNested
internal/integrations/manager.go:815:  codexDirSet && strings.HasPrefix(destination, "~/.codex/")
internal/integrations/manager.go:880:  if codexDirSet && strings.HasPrefix(plan.Destination, "~/.codex/")
internal/generators/agentfiles.go:628,631:  filepath.Join(rootDir, ".codex", "hooks.json") / filepath.Join(guardRoot, ".codex")
internal/generators/update.go:87:  os.Stat(filepath.Join(cwd, ".codex"))
internal/generators/update.go:3091: os.Stat(filepath.Join(root, ".codex"))
```

Nenhum desses é uma construção do caminho global que deveria usar `CodexConfigDir` mas não usa:

- `validator_credential_guard.go` / `validator_guard_wiring.go` — usa `.codex/hooks.json` como
  string relativa (chave de lookup em map), não como caminho absoluto de acesso a arquivo.
- `manager.go` — usa o prefixo `"~/.codex/"` como discriminador de rotas de catálogo; a reescrita
  para `codexDestRoot` é feita imediatamente após a condição (linhas 815-816). Correto.
- `agentfiles.go:628-631` — constrói `rootDir/.codex/hooks.json` onde `rootDir` é o diretório do
  projeto (escopo project), não o home global. Não usa CODEX_HOME porque não escreve para o escopo
  global.
- `update.go:87, 3091` — `os.Stat(filepath.Join(cwd, ".codex"))` detecta se existe diretório
  `.codex` dentro do projeto (project-scope detection), não o dir global do Codex.

Critério de aceite do ML (`grep -rn 'Join(home, ".codex"' internal --include='*.go' | grep -v _test`):

```
internal/agentconfig/agentconfig.go:373:  dir = filepath.Join(home, ".codex")
internal/agentconfig/agentconfig.go:442:  oldDir := filepath.Join(home, ".codex")
```

Linha 373: interior de `CodexConfigDirWithSource` — o próprio resolvedor canônico, legítimo.
Linha 442: `InertCodexGuardPath` — constrói intencionalmente o path HOME-based para detectar guarda
inerte (D5). Legítimo.

**[NOTA-2] O critério de aceite literal falha: retorna 2 linhas.**
O ML declarou o critério como "output vazio", mas as duas linhas retornadas são o resolvedor
canônico e a sonda D5 — exatamente o mesmo formato que o Claude tem
(`internal/agentconfig/agentconfig.go:223: oldDir = filepath.Join(home, ".claude")`). O critério
precisaria de `| grep -v internal/agentconfig/` para ficar vazio. Isso é uma divergência entre o
artefato (critério) e a implementação correta — não é um bug de código, é um critério redigido de
forma estreita demais. **Corretivo:** o arquiteto deve atualizar o critério no roadmap para incluir
`| grep -v internal/agentconfig/`.

---

## 2. `manager.go resolve()`: contenção e rejeição de symlink

### 2a. Contenção (`beneath`)

O `destRoot` é atualizado condicionalmente:

```go
// internal/integrations/manager.go:880
if codexDirSet && strings.HasPrefix(plan.Destination, "~/.codex/") {
    destRoot = codexDestRoot
}
```

A condicionalidade é necessária e correta. O vault
(`vault/notes/integrations-manager-destroot-conditional-update-2026-11-17.md`) documenta o
defeito que `if codexDirSet` (sem o prefixo) causaria: `~/.agents/skills/` fica fora de
`codexDestRoot` e gera "destination is outside global root". **Testado — ver Mutação A.**

**Residual de confiança (trust-model):** quando `CODEX_HOME=/`, `codexDestRoot = "/"` e
`beneath("/", "/anywhere")` é sempre verdadeiro — a contenção não protege mais. O mesmo
comportamento existe para `CLAUDE_CONFIG_DIR=/`. Isso é uma residual de modelo de confiança
aceitada explicitamente na arquitetura: o usuário que seta CODEX_HOME tem controle sobre o
destino. Não é um defeito novo introduzido por ML-4A4.

**CODEX_HOME apontando dentro de outro root** (ex.: `CODEX_HOME=/home/user/.claude`): os
destinos de catálogo `~/.codex/agents/xxx` seriam reescritos para
`/home/user/.claude/agents/xxx`. A contenção `beneath("/home/user/.claude", ...)` passa. Não há
escapes — o usuário configurou explicitamente esse redirecionamento.

CODEX_HOME com `..` ou relativo: `filepath.Clean` + `filepath.IsAbs` em
`CodexConfigDirWithSource` rejeita caminhos relativos (retorna erro, R-1) e normaliza `..` antes
de qualquer uso. `filepath.Abs` em `manager.resolve()` adiciona uma segunda normalização.

### 2b. Rejeição de symlink

```
update.go:1262  rejectHarnessSymlink(filepath.Dir(codexDir), path, id, displayPath)
update.go:2237  rejectHarnessSymlink(filepath.Dir(codexDir), path, id, displayPath)
```

Caso controle: `codexDir = HOME/.codex`, `filepath.Dir(codexDir) = HOME`. Com CODEX_HOME:
`codexDir = CODEX_HOME`, `filepath.Dir(codexDir) = parent(CODEX_HOME)`. Espelha o padrão Claude.

---

## 3. Testes vacuosos — prova de mutação

Seis mutações executadas. Cada uma aplicada, confirmada como FAIL, revertida, e revert
verificado por `git diff` (hit counts em agentconfig.go) e re-run do teste.

### Mutação 1 — `OSCLIHomeResolver.HomeFor`: remove braço CLICodex

**Teste:** `TestOSCLIHomeResolver_WindowsBranch_Codex`
**Conclusão afirmada:** no Windows (simulado), `HomeFor(CLICodex)` retorna USERPROFILE.
**Resultado:** FAIL — `got HOME, want USERPROFILE`.

### Mutação 2 — `CodexConfigDirWithSource`: ignora CODEX_HOME

**Teste:** `TestCodexConfigDir_CODEX_HOME_Honoured`
**Conclusão afirmada:** CODEX_HOME é honrado como raiz.
**Resultado:** FAIL — `got ~/.codex, want CODEX_HOME`.

### Mutação 3 — `InertCodexGuardPath`: nunca reporta inerte

**Teste:** `TestInertCodexGuardPath_DifferentDirs_WithGuardMarker`
**Conclusão afirmada:** detecta guard em HOME/.codex quando codexDir aponta para outro lugar.
**Resultado:** FAIL — `want present=true, got false`.

### Mutação A — `manager.go`: `if codexDirSet` (sem prefixo)

**Teste:** `TestUpdateHarnessCmd_RunsOutsideProjectWithoutTrackfwYAML`
**Conclusão afirmada:** vault — `~/.agents/skills/` fica fora de `codexDestRoot`, gera erro.
**Resultado:** FAIL — `1 target(s) failed`.

### Mutação B — `manager.go`: remove atualização de destRoot

**Teste:** `TestHarnessCatalogTarget_CodexAgents_CODEX_HOME_WritesUnderCodexHome`
**Conclusão afirmada:** `~/.codex/agents/xxx` precisa de `codexDestRoot` para conter `beneath`.
**Resultado:** FAIL — `destination "~/.codex/agents/..." is outside global root`.

### Mutação C — Validator: reverte braço Codex para `filepath.Join(home, relPath)`

**Teste:** `TestGuardGlobalHookResolvable_CodexCODEX_HOME_ReadsFromRoot`
**Conclusão afirmada:** auditor lê de `$CODEX_HOME/hooks.json` quando CODEX_HOME está definido.
**Resultado:** FAIL — `expected violation about missing script, got []`.

### Mutação D — `withD5NoticeCodex`: troca braço condicional por "remove it by hand"

**Teste:** `TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage`
**Conclusão afirmada:** mensagem condicional não instrui remoção quando CODEX_HOME está sem
divergência Windows.
**Resultado:** FAIL — `Message contains "remove it by hand" — conditional form must not instruct deletion`.

---

## 4. Duplicação: `ClaudeConfigDirWithSource` / `CodexConfigDirWithSource`

Duas funções estruturalmente simétricas (~20 linhas de lógica cada). Diferenças intencionais:
env var, CLI, subdiretório, fonte. `CodexConfigDirSource*` e `ClaudeConfigDirSource*` são
definidas e usadas apenas dentro de `agentconfig.go` — padrão simétrico, não é sinal de acesso
externo faltando. `withD5NoticeCodex` re-lê `CODEX_HOME` diretamente, exatamente como
`withD5Notice` re-lê `CLAUDE_CONFIG_DIR` — padrão espelhado.

**Avaliação:** duplicação aceitável. Fatoração não recomendada agora: a simetria facilita
auditoria, e os dois CLIs têm comportamentos que tendem a divergir (Codex não tem `.claude`
subdir; hooks.json está na raiz de CODEX_HOME, não em subdir).

---

## 5. Isolamento: sítios com `CLAUDE_CONFIG_DIR=` sem `CODEX_HOME=`

**Verificação:** `comm -23 <(grep -rl 'CLAUDE_CONFIG_DIR' scripts internal .github Makefile | sort) <(grep -rl 'CODEX_HOME' scripts internal .github Makefile | sort)`

Resultado: 24 arquivos com CLAUDE_CONFIG_DIR sem CODEX_HOME. Análise:

**Pacote `internal/commands`:** tem TestMain em `netexec_test.go` com `os.Unsetenv("CODEX_HOME")`
(ML-4A4, diff confirmado). Todos os testes do pacote estão cobertos globalmente.

**Pacote `internal/generators`:** tem TestMain em `guard_parity_helper_test.go` com
`os.Unsetenv("CODEX_HOME")` (ML-4A4, diff confirmado). Todos os testes do pacote cobertos.

**Pacote `internal/validator`:** tem TestMain em `main_test.go` com `os.Unsetenv("CODEX_HOME")`
(ML-4A4, diff confirmado).

**Scripts de paridade (`check-rules-parity.sh`, `check-slash-parity.sh`, `check-thirdparty-parity.sh`, `check-agent-models-parity.sh`):** verificado que nenhum executa instalação de agente global Codex (todos usam `--scope project` para Codex, ou instalam apenas Claude no escopo global). A isolação por `CODEX_HOME=` não é necessária nesses scripts para o estado atual. Atenção futura: se qualquer desses scripts for estendido para instalar agentes Codex em escopo global, `CODEX_HOME=` deve ser adicionado.

**`internal/validator/validator_credential_guard.go`, `internal/generators/scaffold.go`:** código de produto que menciona CLAUDE_CONFIG_DIR apenas em comentários. Sem risco de isolamento.

**`internal/integrations/manager_claude_test.go`:** testes de comportamento Claude; não envolvem Codex global dir.

Isolamento: **completo** para todos os sítios que realmente escrevem em dirs globais do Codex.

---

## 6. Achados

### [NOTA-1] `harnessGitBranchGuardTargetCodex` — braço "remove it by hand" do D5 sem teste dedicado

`withD5NoticeCodex` é aplicado em todos os return sites de ambas as funções Codex
(`harnessCredentialGuardTargetCodex` e `harnessGitBranchGuardTargetCodex`). Os testes de D5
Codex (`TestHarnessD5NoticeCodex_*`) cobrem apenas o target `codex-credential-guard` e apenas o
braço condicional (CODEX_HOME sem divergência Windows). O braço "remove it by hand" (divergência
Windows ou sem CODEX_HOME com codexDir ≠ HOME/.codex) e o target `codex-git-branch-guard` não
têm teste positivo dedicado.

**Evidência:** Mutação D (troca do braço condicional por "remove it by hand") falha em
`TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage` — o braço condicional é guardado
negativamente. Mas nenhum teste afirma positivamente que o braço "remove it by hand" aparece
quando deveria.

**Impacto:** baixo — a lógica é compartilhada com `withD5Notice` (Claude), que tem a mesma
estrutura e foi testada em MLs anteriores. Regressão futura pode passar despercebida.

**Corretivo:** adicionar dois testes em `update_harness_d5_notice_ml2d_test.go`:
1. `TestHarnessD5NoticeCodex_WindowsDivergence_RemoveItByHand` — simula divergência Windows
   (HOME≠USERPROFILE), verifica que mensagem contém "remove it by hand" para credential guard.
2. `TestHarnessD5NoticeCodex_GitBranch_CODEX_HOME_ConditionalMessage` — mesmo cenário de
   `TestHarnessD5NoticeCodex_CODEX_HOME_ConditionalMessage` mas com `codex-git-branch-guard`.

### [NOTA-2] Critério de aceite literal não está em correspondência com implementação correta

O ML declara: `grep -rn 'Join(home, ".codex"' internal --include='*.go' | grep -v _test` deve
ser vazio. O resultado real tem 2 linhas (resolvedor canônico + sonda D5), ambas legítimas. O
critério deveria ser `| grep -v internal/agentconfig/` para ficar vazio.

**Corretivo:** atualizar o critério no roadmap. Não requer mudança de código.

---

## 7. Gates executados

```
go build ./...                                          exit=0
go vet  (agentconfig, generators, commands,
         validator, integrations)                       exit=0
go test -p 1 -count=1 ./internal/agentconfig/          exit=0  0.220s
go test -p 1 -count=1 ./internal/generators/           exit=0  87.383s
go test -p 1 -count=1 ./internal/commands/             exit=0  66.224s
go test -p 1 -count=1 ./internal/validator/            exit=0  39.455s
go test -p 1 -count=1 ./internal/integrations/         exit=0   7.424s
go test -p 1 -count=1 ./internal/pathguard/            exit=0   1.356s
```

---

## Veredito: APROVA COM NOTAS

Implementação correta. Dois achados de nota, nenhum bloqueante:
- NOTA-1: cobertura de D5 parcial para `codex-git-branch-guard` e braço "remove it by hand".
- NOTA-2: critério de aceite do roadmap redigido mais estrito que necessário; requer ajuste no
  artefato, não no código.

O corretivo de NOTA-1 é novo trabalho na REQ vigente (mesma causa, Regra Dura). O arquiteto
decide se entra neste ML ou no próximo.
