# Wave 0 — Threat Model: home por CLI no Windows (REQ-2026-09-01)

> Data: 2026-10-14 | Autor: hades-tf | Roadmap: ROADMAP-2026-09-22 ML-0A
> ADR: `docs/adr/ADR-2026-10-14-no-windows-o-guard-global-de-cada-cli-de-agente-e-gravado-e-auditado-no-home-que-aquele-cli-le.md`
> REQ: `docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md`

---

## Seção 1 — Mapa CLI → mecanismo de home no Windows

Os caminhos gravados hoje provêm de `internal/generators/update.go` — todas as funções
`harness*Target*` recebem `home` de `homedir.Dir()` (linha 575), que lê `$HOME` primeiro e
`os.UserHomeDir()` como fallback.

`os.UserHomeDir()` no Windows (confirmado em `$(go env GOROOT)/src/os/file.go`):

```go
case "windows":
    env, enverr = "USERPROFILE", "%userprofile%"
// ...
if v := Getenv(env); v != "" { return v, nil }
return "", errors.New(enverr + " is not defined")
```

No Windows, `os.UserHomeDir()` lê **somente a variável de ambiente `USERPROFILE`**. Sem fallback
para Shell API nem registry. Se `USERPROFILE` não estiver definida, retorna erro.

### Medições e fontes

**Claude Code (medido — evidência direta, três terminais)**

- Arquivo gravado: `$HOME/.claude/settings.json` (`update.go:891`)
- Mecanismo no Windows: debug file (`--debug-file`) em todos os três terminais (Git Bash, PowerShell,
  cmd.exe) com `HOME≠USERPROFILE` mostra que Claude Code 2.1.299 lê `settings.json` sempre em
  `C:\Users\Lab\.claude\settings.json` (USERPROFILE), nunca em `HOME`.
  Evidência: `docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md`, Passo 3a.
- O teste com `USERPROFILE` sobrescrito via Git Bash produziu resultado ambíguo
  (`C:\Users\Lab\sers\Lab\homeX_up_3721`) — MSYS2 converteu o path POSIX de modo parcialmente
  inesperado, não permitindo distinguir se Claude lê a env var `USERPROFILE` ou a Windows API
  (`GetUserProfileDirectoryW`). A hipótese mais provável (env var, dado que Node/libuv também usa
  env var primeiro) não está confirmada por medição limpa. O resultado funcional — Claude Code lê
  caminho com USERPROFILE como base — é inequívoco pelo Passo 3a.
- `CLAUDE_CONFIG_DIR` medido nesta sessão: com
  `CLAUDE_CONFIG_DIR=/c/Users/Lab/homeX_ccdir_test_9999`, Claude buscou
  `C:\Users\Lab\homeX_ccdir_test_9999\settings.json`. **O `settings.json` fica na raiz de
  `CLAUDE_CONFIG_DIR`, sem subpasta `.claude/`**. Este comportamento é cross-platform (ver A1).
- Fonte oficial: https://code.claude.com/docs/en/settings — "On Windows, paths shown as `~/.claude`
  resolve to `%USERPROFILE%\.claude`"
- Override: `CLAUDE_CONFIG_DIR` — cross-platform; fora do escopo negativo atual desta REQ (ver A1)
- **Confiança: Medido** (Passo 3a, três terminais)

**Gemini CLI (inferido — proxy Node.js medido)**

- Arquivo gravado: `$HOME/.gemini/settings.json` (`update.go:1136`)
- Mecanismo no Windows: CLI em Node.js. `os.homedir()` do Node.js ignora `HOME` no Windows, retorna
  `USERPROFILE` via `uv_os_homedir()` (libuv), que lê a env var `USERPROFILE` e cai para
  `GetUserProfileDirectoryW` se ausente. Medição proxy: `node -e "console.log(require('os').homedir())"` retorna
  `C:\Users\Lab` em todos os terminais e todos os valores de `HOME` testados (Passo 3c,
  portabilidade 2026-10-14). Proxy não substitui medição do binário Gemini diretamente.
- Fonte oficial: geminicli.com/docs — caminho `~/.gemini/settings.json`; sem override documentado
  para diretório do usuário
- Override: nenhum documentado para diretório do usuário
- **Confiança: Inferido** (proxy Node.js medido; D2 da ADR se aplica)

**Codex CLI (inferido com fonte secundária)**

- Arquivo gravado: `$HOME/.codex/hooks.json` (`update.go:1025`)
- Mecanismo no Windows: CLI em Rust (`codex-rs`). Fontes secundárias documentam o caminho padrão
  como `%USERPROFILE%\.codex` no Windows. Sem acesso ao código-fonte Rust para confirmar a função
  de resolução de home.
- Override: `CODEX_HOME` (https://developers.openai.com/codex/config-reference)
- **Confiança: Inferido** (fonte secundária; D2 se aplica)

**Cursor (inferido)**

- Arquivo gravado: `$HOME/.cursor/hooks.json` (`update.go:1258`)
- Mecanismo no Windows: app Electron/Node.js. Fórum documenta `C:\Users\<user>\.cursor\hooks.json`
  como user global (forum.cursor.com). Override: não documentado.
- **Confiança: Inferido** (fórum; D2 se aplica)

**GitHub Copilot CLI (inferido com fonte oficial parcial)**

- Arquivo gravado: `$HOME/.copilot/settings.json` (`update.go:1421`)
- Mecanismo no Windows: documentação oficial usa `$HOME/.copilot` sem especificar o mecanismo de
  resolução no Windows
  (https://docs.github.com/en/copilot/reference/copilot-cli-reference/cli-config-dir-reference)
- Override: `COPILOT_HOME`
  (https://docs.github.com/en/copilot/how-tos/copilot-cli/set-up-copilot-cli/configure-copilot-cli)
- **Confiança: Inferido** (fonte oficial cita `$HOME` sem precisar o mecanismo Windows; D2 se aplica)

**Windsurf (inferido)**

- Arquivo gravado (credential only): `$HOME/.codeium/windsurf/hooks.json` (`update.go:1531`)
- Não há `windsurf-git-branch-guard` global (`update.go:1521-1522`)
- Mecanismo no Windows: app Electron/Node.js. Fontes secundárias indicam
  `%USERPROFILE%\.codeium\windsurf\` como caminho global. Sem fonte oficial para `hooks.json`.
- Override: não documentado
- **Confiança: Inferido** (D2 se aplica; ver conflito D2×D3, Seção 3.8 e Veredito A8)

**Kiro (inferido com fonte oficial parcial)**

- Arquivos gravados: credential: `$HOME/.kiro/hooks/trackfw-credential-guard.json` (`update.go:1676`);
  git-branch: `$HOME/.kiro/hooks/trackfw-git-branch-guard.json` (`update.go:2225`)
- Mecanismo no Windows: IDE baseado em Electron (fork do VS Code, produto da AWS). Documentação
  oficial (https://kiro.dev/docs/configuration/) cita `~/.kiro/hooks/` sem precisar o mecanismo
  Windows. Override: `KIRO_HOME`
- **Confiança: Inferido** (D2 se aplica)

### Tabela resumida

| CLI | Arquivo global gravado hoje | Override documentado | Confiança |
|-----|----------------------------|-----------------------|-----------|
| Claude Code | `$HOME/.claude/settings.json` | `CLAUDE_CONFIG_DIR` (cross-platform; ver A1) | **Medido** |
| Gemini CLI | `$HOME/.gemini/settings.json` | nenhum p/ user dir | **Inferido** (proxy Node) |
| Codex CLI | `$HOME/.codex/hooks.json` | `CODEX_HOME` | **Inferido** |
| Cursor | `$HOME/.cursor/hooks.json` | nenhum documentado | **Inferido** |
| Copilot CLI | `$HOME/.copilot/settings.json` | `COPILOT_HOME` | **Inferido** |
| Windsurf | `$HOME/.codeium/windsurf/hooks.json` | nenhum documentado | **Inferido** |
| Kiro | `$HOME/.kiro/hooks/trackfw-*.json` | `KIRO_HOME` | **Inferido** |

---

## Seção 2 — População completa dos sítios de home em produção

Resultado de `git grep -n 'homedir\.Dir()\|os\.UserHomeDir()' -- 'internal/*.go'`
(excluindo `_test.go`):

### 2a — Sítios com `homedir.Dir()` (HOME primeiro, USERPROFILE como fallback)

| Arquivo:Linha | Função/Contexto | Classificação |
|---|---|---|
| `internal/commands/adr.go:111` | ADR global dir (`~/.trackfw/adr`) | **Artefato trackfw** — fica como está |
| `internal/commands/agents_models.go:80` | `config.LoadGlobalAgentModels` → `~/.trackfw/trackfw.yaml` | **Artefato trackfw** — fica como está |
| `internal/commands/init.go:60` | `isTrackfwInstalled` → `~/.trackfw/identity.json` | **Artefato trackfw** — fica como está |
| `internal/commands/init.go:407` | `runInitInstallAITools` → instala em `~/.claude/agents/`, `~/.gemini/settings.json` etc. | **Fiação de CLI** — precisa de resolvedor por CLI |
| `internal/commands/integrations_flags.go:338` | `identity.Save(home, cfg)` → `~/.trackfw/identity.json` | **Artefato trackfw** — fica como está |
| `internal/commands/integrations_flags.go:440` | `integrations.Manager{HomeDir: home}` — grava agentes/skills globais em `~/.claude/agents/` etc. | **Fiação de CLI** (agentes/skills de CLIs) |
| `internal/config/config.go:660,667` | `ExpandPath` — expande `~` em paths de config do trackfw | **Artefato trackfw** — fica como está |
| `internal/generators/agentfiles.go:2077` | `globalCredentialGuardScriptPath()` → `~/.trackfw/scripts/...` | **Artefato trackfw** — fica como está |
| `internal/generators/agentfiles.go:2089` | `readGlobalHookJSON(relParts...)` — lê JSON de configs globais de CLI | **Fiação de CLI** — terceiro resolvedor (dedup/check) |
| `internal/generators/agentfiles.go:2482` | `globalCredentialGuardInstalledKiro()` → `~/.kiro/hooks/...` | **Fiação de CLI** (Kiro dedup) |
| `internal/generators/agentfiles.go:2546` | `globalGitBranchGuardScriptPath()` → `~/.trackfw/scripts/...` | **Artefato trackfw** — fica como está |
| `internal/generators/scaffold.go:225` | Scaffold global (`~/.trackfw/`) | **Artefato trackfw** — fica como está |
| `internal/generators/update.go:163` | `identity.Load(home)` → `~/.trackfw/identity.json` | **Artefato trackfw** — fica como está |
| `internal/generators/update.go:295` | `ensureGlobalADRDirRegistered` → `~/.trackfw/adr` | **Artefato trackfw** — fica como está |
| `internal/generators/update.go:575` | **Raiz de `UpdateHarness`** — `home` passado a todos os `harness*Target*` | **Fiação de CLI** — ponto central de injeção |
| `internal/generators/update.go:2648` | `identity.Load(home)` → `~/.trackfw/identity.json` | **Artefato trackfw** — fica como está |
| `internal/validator/validator_git_branch_guard.go:162` | `validateGuardGlobalHookResolvable` — auditor principal global (6 CLIs) | **Fiação de CLI** — ponto de injeção crítico |
| `internal/validator/validator_git_branch_guard.go:356` | `validateGuardGlobalHookShLegacyWarnings` | **Fiação de CLI** |
| `internal/validator/validator_git_branch_guard.go:399` | `validateGuardGlobalHookD2InlineWarnings` | **Fiação de CLI** |
| `internal/validator/validator_git_branch_guard.go:450` | `validateGuardGlobalHookD11LegacyWarnings` | **Fiação de CLI** |
| `internal/validator/validator_git_branch_guard.go:523` | `validateGuardGlobalScriptIntegrity` → `~/.trackfw/scripts/...` | **Artefato trackfw** — fica como está |

### 2b — Sítios com `os.UserHomeDir()` direto (lê USERPROFILE env var no Windows)

| Arquivo:Linha | Função | Classificação | Observação |
|---|---|---|---|
| `internal/validator/validator_credential_guard.go:821` | `credentialGuardGlobalInstalledWindsurf` → `~/.codeium/windsurf/hooks.json` | **Fiação de CLI** (Windsurf) | Lê USERPROFILE; ver conflito D2×D3 em A8 |
| `internal/validator/validator_credential_guard.go:1175` | `validateClaudeGuardHookMatcherGlobalWarnings` → `~/.claude/settings.json` | **Fiação de CLI** (Claude, só warnings) | Lê USERPROFILE; escritor usa HOME → divergência atual |

### 2c — Divergências escritor/auditor hoje

**Caso 1 — Claude Code (violações):** escritor (`update.go:575` → `homedir.Dir()`) grava em
`HOME/.claude/settings.json`; auditor de violações (`validator_git_branch_guard.go:162` →
`homedir.Dir()`) audita no mesmo lugar. Consistentes entre si, **ambos errados** com `HOME≠USERPROFILE`.
Resultado: **falso positivo de saúde** (medido e confirmado).

**Caso 2 — Claude Code (warnings):** escritor grava em HOME; auditor de warnings
(`validator_credential_guard.go:1175` → `os.UserHomeDir()`) lê de USERPROFILE. Com `HOME≠USERPROFILE`:
warnings veem o caminho certo, regra de violações (Caso 1) não vê.

**Caso 3 — Windsurf (três resolvedores divergentes):**
- Escritor (`update.go:1531` → `homedir.Dir()`): grava em `HOME/.codeium/windsurf/hooks.json`
- Dedup checker (`agentfiles.go:2506` → `readGlobalHookJSON` → `homedir.Dir()`): lê de HOME
- Auditor de violações (`validator_credential_guard.go:821` → `os.UserHomeDir()`): lê de USERPROFILE

Com `HOME≠USERPROFILE`: guard em HOME, auditor procura em USERPROFILE → não encontra → emite
**violação** (falso negativo — direção oposta ao Claude). O auditor é acidentalmente correto em
apontar onde Windsurf leria, mas por razão não intencional. Ver A8.

### 2d — Segundo sítio de verdade: scripts

`scripts/check-update-parity.sh` usa `HOME="$home_dir"` para isolar o harness (linhas 110, 160, 197
etc.) mas não define `USERPROFILE`. Após ML-1A, funções de fiação de CLI que usarem `os.UserHomeDir()`
escreverão em `USERPROFILE` real do runner no Windows. Esses scripts precisarão definir `USERPROFILE`
no mesmo wrapper de execução.

`scripts/windows-repro/run.ps1` (linhas 271-283): já define `HOME` e `USERPROFILE` para valores
diferentes — isolamento correto para o script de reprodução.

`scripts/check-agent-models-parity.sh` usa `$WORK/caseN-go/.claude/agents/` — paths sob `HOME`
isolado. Não há referência a `USERPROFILE`. Mesmo risco após ML-1A.

---

## Seção 3 — Threat Model

O adversário aqui é o implementador apressado e o arquiteto otimista.

### 3.1 Override de diretório de config (`CLAUDE_CONFIG_DIR` e equivalentes)

**Medido nesta sessão:** com `CLAUDE_CONFIG_DIR=/c/Users/Lab/homeX_ccdir_test_9999`, Claude buscou
`C:\Users\Lab\homeX_ccdir_test_9999\settings.json` — `settings.json` na **raiz** do
`CLAUDE_CONFIG_DIR`, sem `.claude/` intermediário.

**Escopo:** `CLAUDE_CONFIG_DIR` é cross-platform. Ignorá-lo cria falso positivo de saúde em qualquer
plataforma para usuários que o usam. A REQ atual tem escopo negativo "não muda comportamento fora do
Windows". Ver A1 — decisão de escopo necessária antes do ML-1A.

**Análogos documentados:** `CODEX_HOME`, `COPILOT_HOME`, `KIRO_HOME`. Sem medição para esses; ficam
como resíduo declarado (Seção 5c.2).

### 3.2 `USERPROFILE` ausente no ambiente

**Mecanismo:** `os.UserHomeDir()` no Windows lê somente a env var `USERPROFILE`. Se ausente, retorna
`"", errors.New("%userprofile% is not defined")` (confirmado no Go source). **Sem fallback para
`GetUserProfileDirectoryW`.**

Node.js (libuv) difere: `uv_os_homedir()` usa `GetUserProfileDirectoryW` se `USERPROFILE` estiver
ausente. Claude Code (Node) continuaria funcionando onde o resolvedor Go falharia.

**Impacto para o auditor:** uma função auditora de controle de negação que retorna nil silenciosamente
diante de erro de resolução de home é o mesmo padrão de defeito desta REQ — controle reportado
saudável sem ter verificado. **O auditor deve emitir finding (violação ou aviso), não silêncio**,
quando a resolução de home falha. Ver A7.

**Impacto para o resolvedor Go:** em container/CI sem `USERPROFILE` definida, o resolvedor por CLI
retorna erro enquanto Claude ainda funciona via Windows API. O ML-1A deve declarar este limite e
testá-lo com resolvedor mockado para erro.

### 3.3 Perfil redirecionado ou em roaming

Em ambientes corporativos, `USERPROFILE` aponta para `\\servidor\perfis\<user>`. O trackfw segue
onde `USERPROFILE` aponta. Declarado como resíduo aceitável (Seção 5c.3).

### 3.4 Git Bash sem `HOME`

Com `HOME` ausente, `homedir.Dir()` cai para `os.UserHomeDir()` → USERPROFILE. Sem divergência antes
ou depois da correção.

### 3.5 MSYS2 converte HOME de POSIX para Windows ao spawnar processos nativos

**Medido nesta sessão:** quando Git Bash tem `HOME=/c/Users/Lab/homeX_posix_test`, um filho nativo
vê `HOME=C:\Users\Lab\homeX_posix_test` (Windows form). MSYS2 converte POSIX → Windows form para
variáveis de ambiente ao spawnar processos nativos.

**Impacto para D4:** Go nativo lê `HOME` já em Windows form quando lançado do Git Bash padrão
(`HOME=/c/Users/<user>` → `HOME=C:\Users\<user>`). O predicado de D4 deve usar `filepath.Clean` +
`strings.EqualFold` para comparação robusta.

**Limite:** `EqualFold(Clean(...))` não detecta quando dois paths distintos apontam para o mesmo
diretório via junction, symlink de dir, ou short name 8.3 do NTFS. Declarado como resíduo (Seção
5c.5).

### 3.6 Aviso D4 disparando em ambiente correto (falso positivo)

`HOME=/c/Users/Lab` (Git Bash padrão) e `USERPROFILE=C:\Users\Lab`: strings aparentemente diferentes
mas MSYS2 entrega ao Go `HOME=C:\Users\Lab`. Após `EqualFold(Clean)`: iguais → silêncio correto.

Caso em que o aviso deve disparar: `HOME` definido para diretório diferente de `USERPROFILE`
fisicamente (issue #216 — o cenário original desta REQ).

### 3.7 Resolvedor duplicado — risco de D3 não ser fechado

Se ML-1A cria o resolvedor no gerador mas não o usa no validador, o defeito original se reproduz.
Padrão documentado para Kiro em ROADMAP-2026-08-17 ML-2A. O ciclo de importação atual
(validator não pode importar generators) é a causa técnica. A2 exige pacote compartilhado.

### 3.8 Conflito D2 × D3 em Windsurf

D2: CLIs sem medição mantêm `homedir.Dir()`. D3: escritor e auditor usam mesmo resolvedor.

O auditor atual do Windsurf (`:821`) usa `os.UserHomeDir()` — acidentalmente alinhado com onde o
Windsurf provavelmente lê. Se D3 for aplicado sem medição, o auditor seria movido para
`homedir.Dir()`. Se Windsurf lê USERPROFILE, isso criaria um novo falso positivo de saúde — regressão
em relação ao comportamento atual.

O caminho correto é **medir Windsurf antes de qualquer decisão**. Ver A8. Enquanto A8 não for
resolvida, os três sítios do Windsurf ficam como estão.

### 3.9 CI Windows com HOME não isolado de USERPROFILE

Runners GitHub Actions (`windows-latest`) definem `HOME` e `USERPROFILE` para o mesmo diretório por
padrão. Para reproduzir o defeito (AC2), o job `windows-probe.yml` deve definir `HOME` explicitamente
para valor diferente de `USERPROFILE`. `scripts/windows-repro/run.ps1` já faz isso; deve ser
integrado ou replicado no job de CI.

---

## Seção 4 — Testes afetados

### 4a — Contagem

- **127 lugares** em testes de produção definem `HOME` via `t.Setenv` ou `os.Setenv`
- **6 lugares** em 5 arquivos já definem `USERPROFILE` também:
  `internal/validator/main_test.go:70` (TestMain da suite do validador — isola ambas as variáveis
  para todos os testes do pacote via `os.Setenv`),
  `internal/validator/validator_guard_hook_concordance_external_test.go:381`,
  `internal/validator/validator_credential_guard_test.go:1141`,
  `internal/generators/credential_guard_windsurf_amazonq_ml1b_test.go:375,392,411`

**Nota sobre `internal/validator/main_test.go:70`:** o TestMain do pacote validator define `USERPROFILE`
igual ao `HOME` isolado para todos os testes do pacote. Os sítios `:821` e `:1175` do validador já
estão protegidos contra poluição do runner — não precisam de ação adicional.

### 4b — Arquivos que precisam isolar USERPROFILE após ML-1A

Todos os arquivos abaixo definem `HOME` mas não `USERPROFILE`. Funções de fiação de CLI que após
ML-1A usarem `os.UserHomeDir()` (para CLIs medidos) escreverão em `USERPROFILE` real do runner
Windows se não isolado.

| Arquivo de teste | Risco |
|---|---|
| `internal/commands/update_harness_test.go` | Todos os targets de CLI; risco alto |
| `internal/generators/agentfiles_test.go` | `readGlobalHookJSON`, `globalCredentialGuardInstalled*` |
| `internal/generators/credential_guard_dedup_test.go` | Dedup checks de credential guard global |
| `internal/generators/credential_guard_sabotage_test.go` | Paths CLI globais em testes de falsificação |
| `internal/generators/identity_wiring_test.go` | `~/.claude/agents/`, `~/.claude/skills/` |
| `internal/generators/update_test.go` | Harness update paths |
| `internal/commands/init_test.go` | `init --ai-tools` → paths CLI globais |
| `internal/commands/agents_skills_test.go` | `agents install --scope global` → `~/.claude/agents/` |
| `scripts/check-update-parity.sh` (script) | `HOME=...` sem `USERPROFILE`; validate pós-ML-1A escreve em USERPROFILE real |
| `scripts/check-agent-models-parity.sh` (script) | `$WORK/caseN-go/.claude/agents/` sem `USERPROFILE` |

### 4c — Resolvedor injetável (AC5)

Interface proposta em `internal/agentconfig` (ver A2):

```go
type CLIHomeResolver interface {
    HomeFor(cli string) (string, error)  // cli: "claude", "gemini", "codex", etc.
}
```

Implementação de produção: usa `os.UserHomeDir()` para Claude Code (medido); `homedir.Dir()` para
os demais até medição. Implementação de teste: `FixedCLIHomeResolver{home: t.TempDir()}` — não toca
USERPROFILE real.

---

## Seção 5 — Alvos de falsificação nas duas direções e resíduo declarado

### 5a — Falso-positivo de saúde (guard ausente, validate reporta saudável)

| Superfície | Onde a sabotagem entra | Portão |
|---|---|---|
| Resolvedor por CLI ausente para um CLI específico | Escritor e auditor continuam em `homedir.Dir()` | AC3: teste write + validate com HOME≠USERPROFILE reprova antes da correção |
| Resolvedor duplicado — gerador ≠ validador (D3 não fechado) | Escritor usa resolvedor novo; auditor usa lista antiga | AC3 + AC5: teste reprova antes da correção |
| `CLAUDE_CONFIG_DIR` ignorado pelo resolvedor Claude | Guard em USERPROFILE, Claude lê em `CLAUDE_CONFIG_DIR` | Questão de escopo — ver A1 |
| Auditor retorna nil quando resolução de home falha | Guard ausente, validate saudável sem verificar | AC5: mockado para erro → finding esperado; A7 |
| Windsurf: mover auditor para `homedir.Dir()` sem medição — se Windsurf lê USERPROFILE | Guard em HOME, Windsurf lê em USERPROFILE → falso positivo de saúde | Medir Windsurf primeiro (A8) |

### 5b — Falso-negativo/violação (guard presente e correto, validate reporta violação)

| Superfície | Onde a sabotagem entra | Portão |
|---|---|---|
| Predicado D4 sem case-insensitive | `C:\Users\Lab` vs `c:\users\lab` dispara aviso espúrio | AC4: `strings.EqualFold(filepath.Clean(...))` |
| Windsurf: mover escritor para `os.UserHomeDir()` sem medição — se Windsurf lê HOME | Guard em USERPROFILE, dedup checker (HOME) não encontra → escreve duplicata | A8: medir Windsurf antes de qualquer decisão |
| `USERPROFILE` env var ≠ diretório via junction/subst | Comparação de strings passa, diretório físico difere | Resíduo declarado (5c.5) |

### 5c — Resíduo declarado

1. **CLIs não medidos (Codex, Cursor, Copilot, Windsurf, Kiro):** mantêm `homedir.Dir()` após a
   correção. O aviso de D4 sinaliza `HOME≠USERPROFILE` sem discriminar por CLI.
2. **Overrides de CLIs não medidos (`CODEX_HOME`, `COPILOT_HOME`, `KIRO_HOME`):** comportamento do
   override no Windows não medido.
3. **Perfis em roaming/redirected:** o trackfw segue onde `USERPROFILE` aponta.
4. **`USERPROFILE` ausente:** `os.UserHomeDir()` Go retorna erro; Claude Code (Node/libuv) usa
   `GetUserProfileDirectoryW` e continua funcionando. O resolvedor Go falha onde Claude funciona.
5. **Junction, symlink de dir, nome 8.3 NTFS:** `EqualFold(Clean(...))` pode falhar para caminhos
   fisicamente idênticos expressos de formas diferentes.
6. **`CLAUDE_CONFIG_DIR`:** cross-platform; tratamento pendente de decisão de escopo (A1).

---

## Seção 6 — Veredito

**APROVA COM RESSALVAS.** O ADR está correto nas decisões centrais (D1–D5) e resolve o defeito
principal (falso positivo de saúde no Claude Code, medido e confirmado). Os ajustes A1–A8 são
obrigatórios: A1 e A8 requerem decisão do arquiteto sobre escopo antes do ML-1A; A2–A7 são
diretivas técnicas para a implementação.

---

**A1 — `CLAUDE_CONFIG_DIR`: decisão de escopo antes do ML-1A (bloqueante para o resolvedor Claude)**

`CLAUDE_CONFIG_DIR` é cross-platform — honrá-lo afeta todas as plataformas. A REQ atual tem escopo
negativo "não muda comportamento fora do Windows". O arquiteto deve escolher antes do ML-1A:

- **Opção A:** emenda ao escopo negativo desta REQ para incluir `CLAUDE_CONFIG_DIR` cross-platform.
- **Opção B:** `CLAUDE_CONFIG_DIR` entra em REQ separada; nesta REQ o resolvedor usa
  `os.UserHomeDir()` no Windows e comportamento atual fora do Windows. Usuários com
  `CLAUDE_CONFIG_DIR` ficam com guard inerte em qualquer plataforma até a REQ separada.
- **Opção C:** declarar `CLAUDE_CONFIG_DIR` como resíduo desta REQ com registro explícito.

**A2 — Pacote único com conjunto fechado de sítios de fiação de CLI (obrigatório)**

Criar `internal/agentconfig` com `CLIHomeResolver`. Conjunto fechado obrigatório — qualquer sítio
omitido reintroduz divergência:

- `internal/generators/update.go:575` (escritor principal)
- `internal/validator/validator_git_branch_guard.go:162, 356, 399, 450` (auditor principal + warnings)
- `internal/validator/validator_credential_guard.go:821` (Windsurf — sujeito a A8)
- `internal/validator/validator_credential_guard.go:1175` (Claude warnings — + escopo A1)
- `internal/generators/agentfiles.go:2089` (`readGlobalHookJSON` — dedup de todos os CLIs)
- `internal/generators/agentfiles.go:2482` (Kiro dedup)
- `internal/commands/init.go:407` (`runInitInstallAITools`)
- `internal/commands/integrations_flags.go:440` (agentes/skills globais — `~/.claude/agents/` etc.)

D1 da ADR cobre `~/.claude/agents/` e `~/.claude/skills/` explicitamente como fiação de CLI. O
conjunto A2 já inclui os dois sítios de instalação global.

**A3 — Predicado de D4: `strings.EqualFold(filepath.Clean(...))` (obrigatório)**

MSYS2 converte HOME POSIX para Windows form antes de passar a processos nativos (medido). O predicado:

```go
strings.EqualFold(filepath.Clean(homeEnv), filepath.Clean(osUserHomeDir))
```

**A4 — Aviso de D4 nomeia CLIs afetados (obrigatório)**

O aviso deve distinguir: CLIs com resolvedor por CLI (medidos) vs. CLIs que continuam com
`homedir.Dir()` (não medidos). Torna o diagnóstico acionável.

**A5 — Resolvedor injetável em testes e em `check-update-parity.sh` (obrigatório)**

Testes da Seção 4b devem injetar `FixedCLIHomeResolver{home: t.TempDir()}`. O script
`check-update-parity.sh` deve passar `USERPROFILE="$home_dir"` ao lado de `HOME="$home_dir"` em
todos os wrappers de execução.

**A6 — Mapa de CLIs medidos vs inferidos declarado no código (obrigatório)**

`internal/agentconfig` deve conter comentário listando CLIs medidos, CLIs inferidos, data e
referência à medição (`docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md` e este
arquivo).

**A7 — Auditor com erro de resolução emite finding, não silêncio (obrigatório)**

Para funções auditoras de controle de negação, retornar nil quando o resolvedor falha é o mesmo
padrão de defeito desta REQ. Sítios afetados: `validator_git_branch_guard.go:162, 356, 399, 450` e
`validator_credential_guard.go:821` (sujeito a A8). Quando o resolvedor retorna erro, o auditor
emite finding indicando que o home não pôde ser resolvido (violação ou warning conforme a severidade
do controle).

**A8 — Windsurf: medir antes de qualquer decisão sobre D2×D3 (bloqueante para Windsurf)**

O conflito D2×D3 (Seção 3.8) não pode ser resolvido sem medição. O ML-1A não altera os três sítios
do Windsurf. O arquiteto escolhe:

- **Opção A:** incluir medição do Windsurf antes ou durante o ML-1A. Com medição, Windsurf entra
  no conjunto medido de A2.
- **Opção B:** declarar Windsurf como resíduo desta REQ no ADR D2; REQ separada após medição.

---

## Medições realizadas nesta sessão

| Teste | Resultado |
|---|---|
| `CLAUDE_CONFIG_DIR=… claude --debug-file … --print test` (VM Windows) | `settings.json` na raiz de `CLAUDE_CONFIG_DIR`, sem `.claude/` intermediário |
| Git Bash `HOME=/c/Users/Lab/homeX_posix_test`; cmd.exe: `echo %HOME%` | `C:\Users\Lab\homeX_posix_test` — MSYS2 converte POSIX para Windows form |
| `USERPROFILE='C:\Users\Lab\homeX_up_3721' claude …` via Git Bash | Resultado ambíguo — não conclusivo para env var vs Windows API |
| Go source `$(go env GOROOT)/src/os/file.go` | `os.UserHomeDir()` no Windows: somente env var `USERPROFILE`; sem fallback para Shell API |
