# Threat Model — Guard de Credencial no Google Antigravity
> Wave 0 · ML-0A · hades-tf · 2026-11-18
> REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md

---

## 1. Completude — Enumeração das Superfícies

Esta seção enumera, com evidência (arquivo:linha), todas as superfícies onde o trackfw **grava ou
audita** guard por CLI, e o que falta para o Antigravity em cada uma.

**Método de enumeração:** grep recursivo por `hooks\.json|settings\.json|trackfw-credential-guard\.sh|trackfw-git-branch-guard\.sh` em `internal/` (excluindo `_test`), mais grep por `InjectGeminiHooks`, `gemini-credential-guard`, `gemini-git-branch-guard`, `cliDetectors`, `HomeFor`, `HarnessTargetIDs`, `writeScopeSubagentFrozenRelPaths`, `validator_credential_guard`, `validator_git_branch_guard`, `guard_hook_concordance`, `guard_registry_sites`, `catalog.json`. Cada arquivo do resultado foi mapeado a uma superfície:

- `internal/fileutil/fileutil.go`, `internal/validator/regularfile*.go`, `internal/integrations/manager.go` — infraestrutura/utilitários, sem superfície de guard autônoma.
- `internal/auditsurface/auditsurface.go` — encontrado pelo mapeamento; resulta na superfície 1.13 abaixo.
- Todos os demais mapeados para 1.1–1.12.

A enumeração é por mecanismo (todos os arquivos do resultado do grep foram inspecionados). Não é prova de completude: padrões diferentes dos usados na busca podem nomear superfícies adicionais não listadas.

---

### 1.1 `cliDetectors` — Dispatcher de injeção de hooks

**Arquivo:** `internal/generators/hooks.go:18`

O mapa `cliDetectors` associa cada CLI a uma função `inject` e uma função `detect`. A entrada é o
que aciona a escrita do hook em `trackfw init` (via `InjectHooksDetected`) e em `InjectHooksForTools`.

```
// hooks.go:15-17
// CLIs without an Inject*Hooks implementation (opencode, antigravity) are
// intentionally absent — callers that pass unknown names get a silent skip.
```

CLIs presentes: `claude`, `codex`, `gemini`, `kiro`, `copilot`, `cursor`, `windsurf`, `amazonq`
(linhas 22–93).

**Antigravity: ausente.** Nenhuma função `InjectAntigravityHooks` existe. Qualquer chamada a
`InjectHooksForTools("antigravity", ...)` ou detecção automática de `.agents/` é um silent skip.

---

### 1.2 `InjectGeminiHooks` — Escritor de hooks do Gemini CLI

**Arquivo:** `internal/generators/agentfiles.go:772`

Grava em `.gemini/settings.json`. Eventos e matchers usados:
- `BeforeTool` / `AfterTool`, matchers: `run_shell_command`, `read_file|read_many_files`,
  `write_file|replace` (linhas 825–848).
- Comando de guard: `CredentialCurrentPSPOSIX` (internamente `guardCredentialCmdPSPOSIX`,
  `internal/generators/agentfiles.go:514`).
- Análogo para git-branch: `GuardGitBranchCmdPSPOSIX` (linha 513).

**Antigravity:** nenhum `InjectAntigravityHooks` existe. O caminho `~/.gemini/config/hooks.json`
(global) e `.agents/hooks.json` (projeto) são citados na documentação oficial do Antigravity,
conforme o roadmap Diagnóstico (fonte primária ML-0B). O caminho
`~/.gemini/antigravity-cli/hooks.json` (global, superfície `legacy-cli` para o CLI `agy`)
é **inferido** a partir do padrão de agentes/skills no `catalog.json` — o `catalog.json` não
lista caminhos de hooks explicitamente. **Medir na Wave 1 (ML-1A).**
Nenhum desses caminhos é escrito pelo trackfw hoje.

---

### 1.3 `HarnessTargetIDs` / `harnessCatalogTargetOrder` — Alvo global do harness

**Arquivo:** `internal/generators/update.go:506–570`

A lista de 34 IDs inclui `gemini-credential-guard` e `gemini-git-branch-guard` (linhas 558–559),
inseridos antes de `gemini-agents`/`gemini-skills`. Implementados em
`harnessCredentialGuardTargetGemini` (linha 1384) e `harnessGitBranchGuardTargetGemini` (linha 2327),
que escrevem em `~/.gemini/settings.json` via `HomeFor(CLIGemini)`.

**Antigravity:** `antigravity-credential-guard` e `antigravity-git-branch-guard` **ausentes** da
lista. Nenhum target de harness escreve em `~/.gemini/config/hooks.json`. O `update harness` não
instala guard global para o Antigravity.

---

### 1.4 `agentconfig.go` — Resolução de home por CLI

**Arquivo:** `internal/agentconfig/agentconfig.go:40–48`

Constantes presentes: `CLIClaude`, `CLICodex`, `CLIGemini`, `CLICursor`, `CLICopilot`,
`CLIWindsurf`, `CLIKiro`. **`CLIAntigravity` ausente.**

A função `HomeFor` (linha 129) usa `effectiveUserHomeDir()` (Windows API profile path) **apenas
para Claude e Codex** (linha 131: `cli == CLIClaude || cli == CLICodex`). Todos os demais CLIs
usam `homedir.Dir()` (HOME primeiro). Se o Antigravity no Windows usa o Windows API profile path
(diferente de HOME) para resolver `~/.gemini/config/`, o guard seria gravado no lugar errado.
**Medição: Wave 1 (ML-1A).**

---

### 1.5 `InertClaudeGuardPath` / `InertCodexGuardPath` — Detector de rastro inerte D5

**Arquivo:** `internal/agentconfig/agentconfig.go:218, 438`

Existem funções para Claude (lê `HOME/.claude/settings.json`) e Codex (lê `HOME/.codex/hooks.json`)
para detectar guards gravados no HOME antigo quando `CLAUDE_CONFIG_DIR` ou `CODEX_HOME` aponta
outro diretório. **Nenhuma função `InertAntigravityGuardPath` existe.** Se `~/.gemini/config/` mudar
(por variável de ambiente ou futura versão do Antigravity), o guard gravado no caminho anterior
permanece inerte sem aviso.

---

### 1.6 `writeScopeSubagentFrozenRelPaths` — Proteção de escrita via write-scope guard

**Arquivo:** `internal/guard/writescope.go:60–80`

Caminhos protegidos (seleção):
```
".claude/settings.json"
".codex/hooks.json"
".gemini/settings.json"
".cursor/hooks.json"
".windsurf/hooks.json"
".github/hooks/"
".kiro/hooks/"
".amazonq/cli-agents/"
```

**Ausente:** `.agents/hooks.json` — o arquivo de hook de projeto do Antigravity. Um agente rodando
dentro do Antigravity pode sobrescrever `.agents/hooks.json` (desabilitar o guard com
`"enabled": false` ou remover a entrada) sem que o `trackfw guard write-scope` bloqueie a escrita.

Caminhos globais (`~/.gemini/config/hooks.json` confirmado por documentação oficial;
`~/.gemini/antigravity-cli/hooks.json` inferido) estão fora da raiz do projeto por definição —
o write-scope não os cobre por limitação arquitetural declarada (ver Seção 4, R2).

---

### 1.7 Validators — Auditoria de hooks instalados

**Arquivos:**
- `internal/validator/validator_credential_guard.go`
- `internal/validator/validator_git_branch_guard.go`

Os validators auditam guard em: `.claude/settings.json`, `.codex/hooks.json`,
`.gemini/settings.json`, `.cursor/hooks.json`, `.windsurf/hooks.json`, `.github/copilot-hooks/`,
`.kiro/hooks/`, `.amazonq/cli-agents/` (project scope) e os análogos globais via
`harnessCredentialGuardTargetXxx`.

**Antigravity:** nenhuma verificação de `.agents/hooks.json` nem `~/.gemini/config/hooks.json`.
O comando `trackfw validate` passa com verde mesmo com o guard do Antigravity ausente ou
desabilitado.

---

### 1.8 Testes de concordância e registro de sites

**Arquivos:**
- `internal/validator/validator_guard_hook_concordance_external_test.go`
- `internal/generators/guard_registry_sites_ac4_test.go`

Os testes de concordância invocam `InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`,
`InjectKiroHooks`, e os análogos de Cursor/Copilot/Windsurf/AmazonQ, e validam que o validator
subsequente reporta zero violações. **Nenhum teste cobre `InjectAntigravityHooks`** (que não
existe). A adição do Antigravity não é detectada automaticamente por nenhuma dessas suítes.

---

### 1.9 `catalog.json` — Entrega de agentes/skills e caminhos por superfície

**Arquivo:** `internal/integrations/assets/catalog.json` (linha com `"id":"antigravity"`)

A entrada do Antigravity define **duas** superfícies para agentes/skills:
- `current` (ID `current`): Antigravity IDE e CLI; global `~/.gemini/config/agents/`,
  projeto `.agents/agents/`.
- `legacy-cli` (ID `legacy-cli`): CLI legado `agy`; global `~/.gemini/antigravity-cli/agents/`,
  projeto `.agents/agents/`.

Nenhuma superfície de hook (`hooks`, `credential-guard`, `git-branch-guard`) é listada em nenhuma
das duas. O catálogo confirma que o Antigravity entrega apenas agentes, skills, rules e workflows.

**Inferência de segurança (não confirmada pelo catálogo):** o padrão de caminhos de agentes sugere
que hooks seguiriam a mesma divisão — `~/.gemini/config/` para o IDE/CLI moderno e
`~/.gemini/antigravity-cli/` para o CLI `agy`. Se essa inferência for correta, um guard instalado
apenas na superfície `current` é ghost install para usuários do `agy`. **Medir na Wave 1 (ML-1A).**
Ver T-IDEvsAgy (Seção 2).

---

### 1.10 `trackfw doctor` — Auditoria de saúde do projeto

**Arquivo:** `internal/commands/doctor.go`

O comando `doctor` verifica: arquivos não-registrados com escrita de agente, scaffold-divergente,
scaffold-missing, branch-protection no GitHub (`--remote`), `core.hooksPath` neutralizado e
presença de sh no Windows. Grep por `credential`, `guard`, `hooks.json`, `settings.json`,
`antigravity` em `doctor.go` retorna apenas referências ao `core.hooksPath` do git.

**Antigravity:** `doctor` **não audita** nenhum hook de agente (`.agents/hooks.json`,
`~/.gemini/config/hooks.json`, nem os análogos de outros CLIs). O REQ nomeia o `doctor` como alvo
dos ACs 4 e 6 (audit no mesmo caminho da escrita, aviso de legado do Gemini). Hoje não há nada
para regredir; a ausência é a condição de partida.

---

### 1.11 `docs/cli-parity.md` — Contrato de canal e comportamento

**Arquivo:** `docs/cli-parity.md`

Documenta o contrato de comportamento de cada canal de entrega do trackfw. Menciona Antigravity
em contextos de agentes/skills (`antigravity-agents`, `antigravity-skills`) mas não em guard de
credencial ou git-branch. AC7 do REQ exige que este arquivo reflita o estado medido após a
implementação. Hoje a ausência de guard do Antigravity não está declarada.

---

### 1.12 Wrapper scripts — `scripts/trackfw-credential-guard.sh` e `~/.trackfw/scripts/…`

**Arquivos:** `internal/generators/scaffold.go:1257–1297` (projeto),
`internal/generators/scaffold.go:1304–1340` (global)

Dois wrapper scripts shell existem como superfície de entrega do guard:
- Projeto: `scripts/trackfw-credential-guard.sh`, gerado por `GenerateCredentialGuardScript`.
  Contém `[ -f trackfw.yaml ] || exit 0` (cwd-only, R5) e `exit 2` se `trackfw` não encontrado.
- Global: `~/.trackfw/scripts/trackfw-credential-guard.sh`, gerado por
  `GenerateGlobalCredentialGuardScript`. Sem guarda de cwd; `exit 2` se `trackfw` não encontrado.

O global wrapper foi usado em versões anteriores como o comando do hook nos agent CLI files
(`legacyScriptPath` em `harnessCredentialGuardTargetGemini`, `update.go:1109`). O harness ainda
instala o script como alvo próprio (`credential-guard-script`, `update.go:980`,
`update_test.go:732`) — ou seja, o script ainda é gerado e entregue. Contudo, as linhas de hook
nos `settings.json`/`hooks.json` dos CLIs modernos escrevem o PSPOSIX inline command
(`CredentialGlobalCurrentPSPOSIX`) diretamente — não o caminho do wrapper. O wrapper não é mais
referenciado pela linha de hook de nenhum CLI moderno, mas continua sendo um artefato instalado.

**Para o Antigravity:** o ML-3B deve usar o inline PSPOSIX command (como todos os outros harness
targets modernos), não criar um novo target que referencie o wrapper script. Se o ML-3B reintroduz
o wrapper script como command, cria uma superfície de migração adicional.

---

### 1.13 `auditsurface.go` — `trackfw audit-surface`

**Arquivo:** `internal/auditsurface/auditsurface.go`

O comando `trackfw audit-surface` lê os arquivos de wiring de hooks por runtime via `git show
<ref>:<path>`, sem checkout da worktree. Usa duas constantes internas:
- `canonicalRuntimes` (linha ~32): `["claude", "codex", "gemini", "copilot", "cursor", "kiro", "windsurf", "amazonq"]` — Antigravity ausente.
- `runtimeWiringPath` (linha ~38): mapeia cada runtime ao seu arquivo de wiring de projeto (ex: `"gemini": ".gemini/settings.json"`) — nenhuma entrada para `.agents/hooks.json`.

**Antigravity:** ausente de `canonicalRuntimes` e de `runtimeWiringPath`. Um `trackfw audit-surface` executado num PR que toca o hook do Antigravity não reporta nenhuma anomalia relacionada ao Antigravity.

---

### Resumo de completude

| Superfície | Claude | Codex | Gemini | Antigravity |
|---|:---:|:---:|:---:|:---:|
| `cliDetectors` (hook injector) | ✅ | ✅ | ✅ | ❌ ausente |
| `Inject*Hooks` (project scope) | ✅ | ✅ | ✅ | ❌ ausente |
| Harness target global credential | ✅ | ✅ | ✅ | ❌ ausente |
| Harness target global git-branch | ✅ | ✅ | ✅ | ❌ ausente |
| `CLIAntigravity` / `HomeFor` | ✅ | ✅ | ✅ | ❌ ausente |
| `InertXxxGuardPath` (D5) | ✅ | ✅ | ❌ | ❌ ausente |
| `writeScopeSubagentFrozenRelPaths` | ✅ | ✅ | ✅ | ❌ `.agents/hooks.json` ausente |
| Validator credential audit | ✅ | ✅ | ✅ | ❌ ausente |
| Validator git-branch audit | ✅ | ✅ | ✅ | ❌ ausente |
| Teste de concordância | ✅ | ✅ | ✅ | ❌ ausente |
| `doctor` audit | — | — | — | ❌ ausente (todos os CLIs) |
| `docs/cli-parity.md` guard section | ✅ | ✅ | ✅ | ❌ ausente |
| Wrapper scripts (1.12) | ✅ instalado | ✅ instalado | ✅ instalado | — (ML-3B deve usar inline PSPOSIX) |
| `audit-surface` / `canonicalRuntimes` (1.13) | ✅ | ✅ | ✅ | ❌ ausente |

---

## 2. Modelo de Ameaças

O adversário aqui é o **implementador apressado** que copia `InjectGeminiHooks`, troca o caminho,
o nome do evento e o matcher, mantém a linha PSPOSIX e adiciona um validator que espelha o escritor.
Os testes de concordância passam e o build fica verde — mas o Antigravity pode não respeitar o
contrato de exit 2. Apenas o ML-4A no binário real captura isso. O segundo adversário é o
**arquiteto otimista** que herda o contrato exit-2 do Gemini CLI por linhagem, sem medir.

---

### T1 — Contrato de saída incompatível: exit 2 ignorado pelo Antigravity (ALTO)

**Mecanismo:**
O trackfw `guard credential` (global) bloqueia emitindo para **stderr** e saindo com rc=2. O
`guard git-branch` emite para **stdout** o JSON `{"hookSpecificOutput":{"hookEventName":
"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"..."}}` e sai com rc=2
(`internal/guard/gitbranch.go:216`).

O contrato documentado do Antigravity (roadmap contexto, fonte terceira `antigravity.google/docs/hooks`)
define que o hook responde com `{"decision":"allow|deny|ask","reason":"..."}` em stdout. O
Antigravity pode **ignorar o exit code** e ler apenas o JSON de stdout; ou pode respeitar apenas
o exit code e ignorar o JSON.

**Dois riscos distintos:**
- Se o Antigravity só lê o JSON de stdout: o `guard credential` não escreve nada em stdout →
  stdout vazio → comportamento indefinido (fail-open? fail-closed? medir Wave 1). Medido neste
  ML: `printf '{"tool_input":{"command":"echo AKIA..."}}' | env HOME="$H" "$TRACKFW" guard credential --global`
  retorna rc=2 mas stdout=0 bytes. O `guard git-branch` escreve JSON em stdout mas no schema
  errado (`hookSpecificOutput` vs `decision`) → Antigravity pode não reconhecer o deny.
- Se o Antigravity só respeita o exit code: o git-branch guard funciona (rc=2 = deny). A
  confirmação depende de medição no binário real.

**Falsificação:** Veja Seção 3, Superfície E.
**Medir na Wave 1 (ML-1A):** Exit 2 causa deny? Stdout vazio causa fail-open ou fail-closed?

---

### T2 — Schema de stdin desconhecido: extração de comando silent-allow (MÉDIO)

**Mecanismo:**
O `guard git-branch` usa `ExtractCommand` (`internal/guard/payload.go:148`) que tenta 4 campos de
prioridade: `tool_input.command` (1), `command` root-level (2), `tool_info.command_line` (3),
`hook_input.command` (4). Se o Antigravity passa o comando shell em outro campo (ex:
`input.args`, `parameters.command`), `shellCmd=""` → `cmdRaw=""` → guard é no-op com rc=0.

**Medido neste ML** (binário: trackfw 11.5.0; código citado = HEAD da branch;
`fddf957e` é o único commit pós-11.5.0 que toca `credential.go`: renomeia variável local
`match` → `layer1Match` nas chamadas a `credScanInput` — verificado em
`git diff v11.5.0..fddf957e -- internal/guard/credential.go | grep '^[-+].*credScanInput'`;
a função `credScanInput` em si, `credExtractCmdAndCwd`, `io.ReadAll` e BOM handling ficam
inalterados. `gitbranch.go` não mudou após 11.5.0.
Medições exercitam Layer 1 e git-branch; R7 cobre Layer 2 e está declarado como leitura de
código em HEAD, não medição contra 11.5.0.)
Correção de isolamento: primeira tentativa executou `HOME=X printf '...' | trackfw` (HOME só
aplicado ao printf) e usou cwd do repo (walk-up encontrou `trackfw.yaml`). Corrigido para
subshell `cd "$PROJ"` com `env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H"`.
Rodado a partir de `$PROJ` (diretório com `trackfw.yaml`) com `env HOME="$H" "$TRACKFW" guard git-branch`:
```
# campo Claude-schema (campo 1 — deve bloquear)
echo '{"tool_name":"run_command","tool_input":{"command":"git commit -m msg"}}' | env HOME="$H" "$TRACKFW" guard git-branch
  → rc=2, stdout=265 bytes (hookSpecificOutput deny), stderr=reason  [BLOQUEADO como esperado]

# campo desconhecido do Antigravity (input.args)
echo '{"tool_name":"run_command","input":{"args":["git","commit","-m","msg"]}}' | env HOME="$H" "$TRACKFW" guard git-branch
  → rc=0, stdout=0, stderr=0  [SILENT ALLOW — ameaça confirmada]

# campo desconhecido alternativo (input.command)
echo '{"tool_name":"run_command","input":{"command":"git commit -m msg"}}' | env HOME="$H" "$TRACKFW" guard git-branch
  → rc=0, stdout=0, stderr=0  [SILENT ALLOW]
```

Nota de medição: a primeira tentativa deste ML rodou `HOME=X printf '...' | trackfw`, que aplicava
HOME apenas ao `printf`, não ao `trackfw`. Corrigido para `env HOME="$H" "$TRACKFW"` em subshell
a partir de `$PROJ` (que tem `trackfw.yaml`) — sem isso, o guard retorna rc=0 pelo check do
`trackfw.yaml`, não pelo payload.

O `guard credential` tem duas camadas:
- Layer 1 (`RunCredentialGlobal`, linha 338): `credScanInput(inputStr)` — varre o **payload raw
  completo** como string. Um segredo em qualquer campo (incluindo `input.args`) é detectado.
  Medido: `echo '{"tool_name":"run_command","input":{"args":["echo AKIA..."]}}' | env HOME="$H" "$TRACKFW" guard credential --global`
  → rc=2 (bloqueado). Layer 1 cobre campos desconhecidos.
- Layer 2: análise de redirecionamentos e arquivos referenciados pelo comando extraído. Se `shellCmd=""`
  (campo desconhecido), Layer 2 é no-op — credenciais em arquivos apontados pela ferramenta
  (ex: `--env-file .env.secret`) não são rastreadas.

**Medir na Wave 1 (ML-1A):** Campo exato que o Antigravity usa para o comando shell no payload
do evento `PreToolUse`. Mesmo campo para a ferramenta de escrita de arquivo.

---

### T3 — cwd do hook ≠ raiz do workspace: guard é no-op (MÉDIO)

**Dois comportamentos distintos — credential vs git-branch:**

**T3-cred:** O `guard credential` (project scope, `RunCredential`, `internal/guard/credential.go:210–212`)
verifica `Stat(cwd/trackfw.yaml)` e retorna 0 (no-op) se ausente. Não há walk-up. Se o cwd do
hook for HOME ou o diretório do script de hooks, o guard de projeto é no-op — mas o guard global
(`--global`) não tem essa verificação e está sempre ativo.

**T3-branch:** O `guard git-branch` (passo 0b, `internal/guard/gitbranch.go:116–118`) **caminha**
a partir de `os.Getwd()` procurando `trackfw.yaml`. Se não encontrar em nenhum ancestral até `/`,
retorna 0 (no-op). Walk-up mitiga o risco de cwd = subdiretório, mas não mitiga cwd = HOME ou
diretório fora da árvore do workspace.

**Medido neste ML** (isolamento completo: `cd "$S"` onde S=`mktemp -d` sem trackfw.yaml em nenhum
ancestral; HOME/USERPROFILE/CODEX_HOME = tmpdir separado):
```
# git-branch guard sem trackfw.yaml no cwd ou nos ancestrais
(cd "$S" && echo '{"tool_input":{"command":"git commit -m msg"}}' \
    | env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H" trackfw guard git-branch)
  → rc=0, stdout=0, stderr=0  [no-op — trackfw.yaml ausente, walk-up falha]

# controle: benign também é no-op (não é bloqueio indiscriminado)
(cd "$S" && echo '{"tool_input":{"command":"echo hello"}}' \
    | env HOME="$H" USERPROFILE="$H" CODEX_HOME="$H" trackfw guard git-branch)
  → rc=0, stdout=0, stderr=0
```

O `guard credential --global` não tem essa verificação — mas o harness global ainda não existe
para o Antigravity (ver Seção 1.3).

**Medir na Wave 1 (ML-1A):** cwd efetivo do processo de hook invocado pelo Antigravity (gravar
`$PWD`/`Get-Location` no hook de sonda).

---

### T4 — Matcher não cobre a ferramenta de escrita de arquivo do Antigravity (MÉDIO)

**Mecanismo:**
O Gemini CLI usa matchers `run_shell_command`, `read_file|read_many_files`, `write_file|replace`.
Para o Antigravity, os nomes de ferramentas para operações de shell e escrita de arquivo são
desconhecidos. O roadmap cita `run_command` como possível nome de ferramenta de shell, mas não
documenta a ferramenta de escrita de arquivo.

Se o `InjectAntigravityHooks` for implementado copiando os matchers do Gemini CLI sem medir, o
hook só será invocado para ferramentas com esses nomes exatos — e será ignorado para qualquer
ferramenta de escrita de arquivo com nome diferente. O guard de credencial ficaria sem cobertura
para escritas de arquivo.

**Medir na Wave 1 (ML-1A):** `tool_name` nos payloads de stdin para: (a) comando shell, (b)
escrita de arquivo, (c) leitura de arquivo — via hook de sonda que grava o payload completo.

---

### T5 — `enabled: false` desabilita o grupo silenciosamente (MÉDIO)

**Mecanismo:**
O schema de hooks do Antigravity permite `"enabled": false` em um grupo de hooks. Um agente
Antigravity que pode escrever em `.agents/hooks.json` (que **não** está em
`writeScopeSubagentFrozenRelPaths`, ver Seção 1.6) pode adicionar `"enabled": false` ao grupo
do guard ou remover a entrada inteira. O `trackfw validate` não auditaria essa remoção (ausência
de validator para Antigravity). O guard fica off sem nenhum aviso.

**Cadeia:**
1. Matcher do write-scope não inclui `.agents/hooks.json`.
2. Agente escreve `{"enabled": false}` no grupo do guard.
3. Guard desabilitado; `validate` passa verde.

---

### T6 — Override de projeto sobrepõe o guard global; ou o contrário (ALTO se override)

**Mecanismo:**
O Antigravity pode usar semântica de **override** para o hooks.json de projeto (o arquivo de
projeto substitui o global, em vez de fazer merge). Nesse caso:
- O guard global instalado pelo `update harness` em `~/.gemini/config/hooks.json` fica inativo
  no contexto de qualquer projeto que tenha `.agents/hooks.json` próprio (mesmo vazio).
- O projeto pode não ter o guard instalado (nenhum `InjectAntigravityHooks` ainda) → guard
  global completamente anulado.

No sentido oposto: se o global tiver precedência e o projeto não tiver override, o D14
(`removeGuardFromNestedArrayInHooks` / `removeGuardFromFlatArrayInHooks`,
`internal/generators/guard_generations.go:244`) removerá as entradas de projeto ao instalar o
global. Se o Antigravity usar override de projeto, isso deixa o projeto sem guard algum.

**D14 não deve ser aplicado ao Antigravity** até que a semântica de precedência (merge vs
override) seja medida.

**Medir na Wave 1 (ML-1A):** Precedência efetiva entre `.agents/hooks.json` e
`~/.gemini/config/hooks.json` — instalar hook apenas globalmente, verificar se ele dispara num
workspace com `.agents/hooks.json` vazio.

---

### T7 — PATH sem `trackfw`: hook falha com rc=127, não rc=2 (BAIXO — mitigado pelo PSPOSIX)

**Mecanismo:**
O comando `trackfw guard credential` requer que `trackfw` esteja no PATH do processo de hook.
Se o Antigravity invoca o hook em um ambiente com PATH reduzido (sem `/usr/local/bin`), o shell
retorna rc=127 ("command not found"). Se o Antigravity trata rc≠2 como allow, o hook falha aberto.

**Mitigação atual na linha PSPOSIX** (`CredentialCurrentPSPOSIX`,
`internal/guardforms/guard_generations.go:67`):
- Na parte POSIX: `set +eu; ... trackfw guard credential; LASTEXITCODE=$((2*!!$?))` →
  se trackfw não for encontrado, `$?=127`, `2*!!127=2`; a linha termina com `exit $LASTEXITCODE=2`.
  Fail-closed em POSIX com trackfw ausente do PATH.
- Na parte PS: `$LASTEXITCODE=2 2>$null; trackfw guard credential; $LASTEXITCODE=2*!!$LASTEXITCODE` →
  se trackfw não for encontrado em PS, `$LASTEXITCODE` é não-zero, e a expressão produce 2.
  Fail-closed em PS com trackfw ausente do PATH.
- Para CmdExe (`CredentialCurrentCmdExe`: `trackfw guard credential || exit 2`): fail-closed.

O risco real de T7 está condicionado ao shell do hook: se o Antigravity usar um shell que a
linha PSPOSIX não cobre (ex: fish, nushell, ou uma shell custom), a garantia não se aplica.

**Medir na Wave 1 (ML-1A):** PATH efetivo no processo de hook; shell efetivo usado pelo Antigravity
para executar o `command` do hook.

---

### T8 — Windows: shell do hook desconhecido para o Antigravity (MÉDIO)

**Mecanismo:**
A linha `CredentialCurrentPSPOSIX` é um poliglota PS/POSIX que funciona no PowerShell e em
sh/bash. Se o Antigravity no Windows usa cmd.exe para executar o `command` do hook (como
documentado como hipótese para outros CLIs no memory note `project_powershell_command_converts_exit2_to_1`),
a linha PSPOSIX não é válida em cmd.exe — a correção seria usar `CredentialCurrentCmdExe`.

Adicionalmente, o PowerShell 5.1 e 7.x têm comportamentos diferentes para exit codes
(memory note: `project_powershell_command_converts_exit2_to_1`): PS7 converte exit 2 → 1 em
alguns contextos.

**Medir na Wave 1 (ML-1A):** Shell usado pelo Antigravity para executar o `command` do hook no
Windows (gravar `$PSVersionTable` ou `$0` no hook de sonda).

---

### T-Timeout — Stdin aberto indefinidamente: credential guard trava, git-branch nega (MÉDIO)

**Mecanismo:**
O `guard credential` lê stdin com `io.ReadAll` sem timeout (`internal/guard/credential.go:196,210`:
comentário explícito "no timeout — residual declared in ADR D9"). Se o Antigravity mantiver o
stdin aberto, o guard de credencial trava indefinidamente — o Antigravity nunca recebe resposta
e a ferramenta fica pendente. O que o Antigravity faz com um hook que nunca responde é
desconhecido (medir na Wave 1).

O `guard git-branch` usa `DrainStdin` com `StdInIdleTimeout = 2 * time.Second`
(`internal/guard/gitbranch.go:78–110`): o idle timeout dispara se **nenhum byte chegar dentro de
2 s**, não se o EOF demorar. Uma vez que algum dado flui, o guard aguarda o EOF sem limite de tempo.

**Medido neste ML** (binário: trackfw 11.5.0; código citado = HEAD da branch; desvio de
isolamento: cwd = raiz do repositório (`/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw`),
não tmpdir — a restrição do dispatch exige tmpdir mas o braço de git-branch requer cwd com
`trackfw.yaml` para não ser no-op; HOME/USERPROFILE/CODEX_HOME = tmpdir separado.
O braço de credencial **não foi medido**: ao enviar um payload com chave fake, o guard de
pré-commit do repositório interceptou o comando e escreveu
`docs/roadmaps/.trackfw-credential-guard.json` (timestamp 2026-11-18T12:13:44Z, nível
`action_required`). O arquivo é gitignored; a escrita ocorreu na árvore de trabalho do
repositório. A afirmação de `io.ReadAll` sem timeout vem de leitura direta de
`credential.go:196,210` — não de medição desta sessão.):
```
arm_a — control, stdin fecha imediatamente:
  printf '{"tool_input":{"command":"echo hi"}}' | guard git-branch
  → rc=0, elapsed=0s  [allow]

arm_b — payload enviado, stdin mantido aberto 3s:
  (printf '...'; sleep 3) | guard git-branch
  → rc=0, elapsed=3s  [allow — guard aguarda EOF sem limite; processa quando stdin fecha]

arm_c — 3s de silêncio antes do primeiro byte:
  (sleep 3; printf '...') | guard git-branch
  → rc=2, elapsed=3s, stderr="read timeout — no byte arrived within the 2s idle window"
  [DENY por timeout — sem byte por 2s, guard nega antes do payload chegar]
```

**Dois riscos distintos — ambos os guards:**
- **Stdin nunca fecha (arm_b extrapolado):** o credential guard (`io.ReadAll`) e o git-branch guard
  (arm_b: "aguarda EOF sem limite") **ambos travam** se o Antigravity mantiver stdin aberto após
  enviar o payload. Para o credential guard isso vem do código (`io.ReadAll` sem timeout,
  `credential.go:196,210`); para o git-branch guard foi confirmado por arm_b (3s de stdin aberto
  após o payload → rc=0 apenas quando stdin fechou, 3s de elapsed).
- **Latência antes do primeiro byte (arm_c — apenas git-branch):** se o Antigravity demora >2 s
  para enviar o **primeiro byte**, o guard nega com timeout antes de ler qualquer dado — falso
  positivo para qualquer ferramenta. O credential guard não tem idle timeout; para ele o único
  risco é o stdin que nunca fecha.

**Medir na Wave 1 (ML-1A):** O Antigravity fecha stdin após enviar o payload? O hook recebe o
payload em um único write ou em chunks com possível latência antes do primeiro byte?

---

### T-IDEvsAgy — Guard instalado na superfície `current` pode ser ghost para usuários do CLI `agy` (MÉDIO, condicionado à inferência)

**Mecanismo:**
O caminho global `~/.gemini/config/hooks.json` para a superfície `current` está confirmado por
documentação oficial (roadmap Diagnóstico, via ML-0B). O caminho global
`~/.gemini/antigravity-cli/hooks.json` para a superfície `legacy-cli` (CLI `agy`) é **inferido**
a partir do padrão de agentes/skills no `catalog.json` — o catalog.json não lista hooks diretamente.
**Medir na Wave 1 (ML-1A) se `agy` usa um caminho distinto para hooks.**

Se a inferência for correta e o `InjectAntigravityHooks` e os harness targets forem
implementados escrevendo apenas em `~/.gemini/config/hooks.json`:
- Usuários do IDE Antigravity e do CLI moderno: guard ativo.
- Usuários do CLI `agy` (legacy): guard nunca invocado — `~/.gemini/antigravity-cli/hooks.json`
  está vazio.

O `trackfw validate` reportaria "guard instalado" com base em `~/.gemini/config/hooks.json` —
falso positivo para usuários do `agy`. O harness de atualização precisaria de targets distintos
para as duas superfícies (como `antigravity-credential-guard-current` e
`antigravity-credential-guard-legacy-cli`), ou precisaria escrever nos dois caminhos.

**A decisão de como tratar as duas superfícies é entrada para a ADR da Wave 2 (ML-2A).**
**Medir na Wave 1 (ML-1A):** O CLI `agy` usa `~/.gemini/antigravity-cli/hooks.json` ou
`~/.gemini/config/hooks.json`?

---

### T9 — hooks.json com BOM: Antigravity pode falhar ao parsear (BAIXO)

**Mecanismo:**
No Windows, o PowerShell pode escrever o BOM UTF-8 (0xEF 0xBB 0xBF) em arquivos JSON. Se o
`hooks.json` escrito pelo `InjectAntigravityHooks` contiver BOM (porque foi escrito sem BOM mas o
usuário o editou com PowerShell), o Antigravity pode falhar ao parsear o arquivo — travando ou
aceitando silenciosamente.

**Nota sobre stdin:** o `credExtractCmdAndCwd` já strips BOM do payload de stdin antes do
`json.Unmarshal` (`internal/guard/credential.go:737–744`: `stripped := bytes.TrimPrefix(data, bom)`,
depois `json.Unmarshal(stripped, &root)`). O risco em T9 é sobre o arquivo hooks.json lido pelo
próprio Antigravity, não sobre o payload de stdin lido pelo guard.

O comportamento do Antigravity com um hooks.json com BOM é desconhecido.

**Medir na Wave 1 (ML-1A):** Antigravity aceita hooks.json com BOM UTF-8?

---

### T10 — Divergência de home no Windows: guard global gravado no lugar errado (MÉDIO)

**Mecanismo:**
`HomeFor("antigravity")` cai no branch padrão (`homedir.Dir()`, HOME primeiro) porque
`CLIAntigravity` não existe em `agentconfig.go`. Se no Windows HOME ≠ perfil do Windows API
e o Antigravity lê `~/.gemini/config/hooks.json` via Windows API (UserHomeDir), o harness
grava em `%HOME%/.gemini/config/hooks.json` mas o Antigravity lê de
`%USERPROFILE%/.gemini/config/hooks.json` — guard instalado mas nunca disparado.

Este é exatamente o defeito documentado para o Claude (ADR-2026-10-14, medido em
`docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md`) e Codex (ML-4A4). O
padrão se repete para qualquer CLI adicionado sem medição explícita.

**Medir na Wave 1 (ML-1A):** Caminho efetivo que o Antigravity usa para ler `~/.gemini/config/`
no Windows (gravar `$env:HOME` e `$env:USERPROFILE` e o cwd efetivo no hook de sonda).

---

### T11 — stdout JSON do git-branch guard: schema errado para o Antigravity (ALTO)

**Mecanismo:**
O `denyOut` do git-branch guard (`internal/guard/gitbranch.go:216`) emite:
```
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"..."}}
```
O contrato do Antigravity (citado no roadmap) espera:
```
{"decision":"deny","reason":"..."}
```

**Dois modos de falha:**
- **Fail-open (perde o deny):** O Antigravity lê o JSON de stdout e não reconhece `hookSpecificOutput.permissionDecision`
  como deny. Ausência de `decision:deny` é tratada como allow. Resultado: `git commit` com
  credencial bloqueado pelo guard passa sem interrupção.
- **Fail-closed (bloqueia tudo):** O Antigravity não consegue parsear o JSON de schema desconhecido
  e rejeita toda saída do hook. Resultado: todas as ferramentas ficam bloqueadas permanentemente —
  incluindo operações benignas.

Apenas o ML-4A no binário real decide qual modo ocorre; não há como inferir pelo spec hoje.

O `guard credential` não escreve nada em stdout (medido neste ML com binário 11.5.0: stdout=0 bytes em qualquer arm).
Para o credential guard, apenas o exit code é sinal de deny — risco cobre apenas T1.

**Nota para ML-4A:** ao testar o credential guard no Antigravity real, usar **`--global`** ou um
projeto com `credential_guard.mode: block` em `trackfw.yaml`. O project scope padrão é `warn`
(rc=0, stderr=aviso): medido neste ML (binário 11.5.0) — `echo '{"tool_input":{"command":"echo AKIA..."}}'` passado
ao guard de projeto sem `credential_guard.mode: block` retorna rc=0. Um braço de deny que retorna
rc=0 seria lido como "o Antigravity ignora exit 2" quando na verdade o guard nunca bloqueou.

---

### T12 — Decisão `ask` desvia o guard sem deny explícito (BAIXO)

**Mecanismo:**
O Antigravity aceita `{"decision":"ask","reason":"..."}` como terceiro valor. Se o guard
emite `ask` em vez de `deny`, o agente apresenta a decisão ao usuário, que pode clicar em
"allow". Para proteção de credenciais, `ask` é bypass.

Risco concreto no ML-3A: se a implementação usar `ask` como fallback para "não tenho certeza"
(ex: modo warn), o guard torna-se interativo em vez de automático. O guard deve emitir apenas
`allow` ou `deny`, nunca `ask`. Declarado como restrição de implementação do ML-3A, não ameaça
ativa hoje.

---

## 3. Alvos de Falsificação — Dois Sentidos por Superfície

Para cada superfície: o que quebra se regredir em cada direção, e qual gate deveria capturar.

**Nota sobre o papel dos testes de concordância:** os testes de concordância
(`validator_guard_hook_concordance_external_test.go`) verificam que o **escritor** e o
**validator** concordam — ou seja, que o que `InjectAntigravityHooks` escreve é o que
`credential_guard_hook_resolvable` / `git_branch_guard_hook_resolvable` consegue encontrar.
Eles **não** verificam se o Antigravity real aceita esse formato. Para isso, apenas o ML-4A no
binário real é gate efetivo.

---

### Superfície A — `InjectAntigravityHooks` (escritor de hook de projeto)

**Falso negativo (guard não instalado):** O agente escreve em `.agents/hooks.json` sem o guard.
Gate que deveria capturar: teste de concordância no `validator_guard_hook_concordance_external_test.go`.
Estado atual: **nenhum teste existe para Antigravity** — a falha não seria detectada.

**Falso positivo (guard recusado pelo Antigravity):** Guard escrito mas o Antigravity rejeita o
formato JSON e bloqueia **todas** as chamadas de ferramenta, incluindo as benignas.
Gate: braço "allow" do ML-4A no binário real (comando inócuo deve passar sem bloqueio).
Os testes de concordância **não** detectam este falso positivo — eles só verificam que o
validator lê o que o escritor escreveu.

---

### Superfície B — Harness targets `antigravity-credential-guard` / `antigravity-git-branch-guard`

**Falso negativo (target ausente):** `update harness` não instala guard global em
`~/.gemini/config/hooks.json`. Gate que deveria capturar: o pin de contagem de IDs
(`HarnessTargetIDs`). **Comportamento atual do pin** (verificado em `update_test.go:830–842`,
`TestUpdateHarnessTargetCountMatchesHarnessTargetIDs`): ele faz
`len(report.Targets) == len(HarnessTargetIDs)`.

O que o pin **captura**: se um ID está em `HarnessTargetIDs` mas o código de `UpdateHarness`
não o despacha (não wired), `report.Targets` fica com um item a menos → contagens diferem →
teste falha.

O que o pin **não captura**:
- Target nunca adicionado à slice `HarnessTargetIDs` → `len` inalterado → passa.
- Target removido tanto de `HarnessTargetIDs` quanto de `UpdateHarness` → `len` inalterado → passa.
- Target com nome errado adicionado a ambos → `len` igual → passa; IDs corretos ausentes.

Para fechar este gap, ML-3B deve adicionar um teste de **membros** explícito: que `HarnessTargetIDs`
contém `"antigravity-credential-guard"` e `"antigravity-git-branch-guard"`. O pin de contagem é
gate de "ID declarado mas não wired" — não gate de nomenclatura correta nem de ID jamais declarado.

**Falso positivo (caminho errado por divergência HOME/USERPROFILE):** `validate` reporta
"installed" mas o guard está em `%HOME%` enquanto o Antigravity lê de `%USERPROFILE%`.
Gate: `InertAntigravityGuardPath` (análogo ao Claude/Codex) — **não existe ainda**.

---

### Superfície C — Validators (`credential_guard_hook_resolvable`, `git_branch_guard_hook_resolvable`)

**Falso negativo (validator não audita o caminho Antigravity):** Guard removido de
`.agents/hooks.json` e `validate` passa verde.
Gate: se o validator for implementado sem o caminho correto, qualquer remoção do guard passa
silenciosamente. O teste de concordância capturaria — mas só se o teste existir.

**Falso positivo (validator audita caminho errado ou schema errado):** Validator reporta
`credential_guard_hook_resolvable` FAIL para um guard corretamente instalado.
Gate: teste de concordância produz zero violações após `InjectAntigravityHooks` — mas somente
se o teste existir. Mesmo com o teste, o falso positivo de schema errado vs Antigravity real
só é detectável pelo ML-4A.

---

### Superfície D — `writeScopeSubagentFrozenRelPaths`

**Falso negativo (`.agents/hooks.json` ausente da lista):** Agente Antigravity escreve
`"enabled": false` no grupo do guard; write-scope não bloqueia.
Gate: teste em `credential_guard_sabotage_test.go` ou análogo para Antigravity — **não existe**.
Entrada atual da lista: `.gemini/settings.json` protegida; `.agents/hooks.json` não.

**Falso positivo (`.agents/` inteiro congelado):** Agente não consegue gravar novos agentes/skills
em `.agents/agents/` ou `.agents/skills/`.
Gate: a proteção deve ser apenas para `.agents/hooks.json`, não para `.agents/` como diretório.
O padrão de outros CLIs (`.kiro/hooks/`, não `.kiro/`) serve de referência.

---

### Superfície E — Contrato de saída (exit code vs JSON stdout)

**Falso negativo (exit 2 ignorado; stdout JSON sem reconhecimento de `decision:deny`):**
Operação com credencial bloqueada pelo guard passa sem interrupção.
Gate: braço "deny" do ML-4A no Antigravity real — git commit com credencial fake deve ser
bloqueado. **Único gate efetivo; não há substituto em testes unitários.**

**Falso positivo (schema desconhecido em stdout bloqueia todas as respostas):** Qualquer
ferramenta falha, incluindo as benignas.
Gate: braço "allow" do ML-4A — `echo hello` deve passar sem bloqueio.

---

### Superfície F — Schema de stdin (extração do comando)

**Falso negativo (campo desconhecido → command="" → no-op no git-branch guard):**
`git commit` passa sem bloqueio porque o campo do comando está em `input.args`, não em
`tool_input.command`. Confirmado como real: medido neste ML (ver T2).
Gate: teste com payload real do Antigravity (disponível apenas após ML-1A). Sem o payload
real, não há como escrever esse teste hoje.

**Falso positivo (campo extraído incorretamente → bloqueio de comando benígno):**
O guard extrai o valor errado de um campo diferente e classifica um comando inócuo como
operação git bloqueada.
Gate: braço "allow" do ML-4A.

---

### Superfície G — Proteção de rastro inerte (D5)

**Falso negativo (guard inerte não reportado):** Usuário migra de `HOME/.gemini/config/` para
outro caminho; guard antigo permanece em `HOME/.gemini/config/hooks.json` mas nunca dispara;
`validate` não reporta.
Gate: `InertAntigravityGuardPath` — **não existe ainda**.

**Falso positivo (rastro inerte reportado incorretamente):** `validate` reporta D5 para um guard
que ainda está no caminho correto.
Gate: condição `strings.EqualFold` no path comparison (já testada para Claude/Codex).

---

### Superfície H — Timeout de stdin

**Falso negativo (ambos os guards travam quando stdin nunca fecha):** Se o Antigravity não
fechar stdin após enviar o payload, tanto o credential guard (`io.ReadAll` sem timeout,
`credential.go:196,210`) quanto o git-branch guard (arm_b confirmado: aguarda EOF sem limite
uma vez que algum dado flui) ficam travados esperando EOF. Nenhum dos dois retorna rc=2.
O que o Antigravity faz com um hook que nunca responde é desconhecido — pode tratar como
allow (fail-open) ou como deny (fail-closed); medir na Wave 1 (ver T-Timeout).
Gate: ML-4A com sonda que fecha stdin imediatamente após o payload (caso normal) e, num braço
separado, com sonda que mantém stdin aberto indefinidamente. Se o guard travar, o teste trava
junto. Não há teste unitário para este comportamento (ausência de timeout é residual declarado
em ADR D9).

**Falso positivo (git-branch guard nega por latência antes do primeiro byte):** O Antigravity
demora >2 s para enviar o **primeiro byte** (latência pré-payload); git-branch nega com
`reasonTruncated` (rc=2) antes de receber qualquer dado. Ferramentas benignas são negadas.
Confirmado por medição: arm_c em T-Timeout (3s antes do primeiro byte → rc=2 "no byte within 2s").
Gate: braço "allow" do ML-4A — `echo hello` deve passar sem o guard acionar. O teste ML-4A
deve ser feito com payload enviado imediatamente (sem delay artificial).

---

### Superfície I — IDE vs CLI `agy` (T-IDEvsAgy)

**Falso negativo (guard instalado apenas em `current`, `agy` não protegido):** Se a inferência de
caminhos confirmar que o `agy` lê `~/.gemini/antigravity-cli/hooks.json`, um guard instalado
apenas em `~/.gemini/config/hooks.json` nunca é invocado para o usuário do `agy`.
Gate: ML-4A com o CLI `agy` explicitamente testado (além do IDE), após confirmação de caminho
em ML-1A. Requer decisão da ADR (Wave 2) sobre como tratar as duas superfícies.

**Falso positivo (nenhum identificado com certeza):** Invocar o guard duas vezes para um
comando benígno resultaria em dois `allow` (rc=0), não em bloqueio. Candidato: se o formato
de hooks.json esperado pelo `agy` (superfície `legacy-cli`) difere do formato de `current` e a
implementação escreve apenas um formato para ambos os caminhos, o `agy` pode falhar ao parsear
e bloquear todas as ferramentas. Medir na Wave 1 (ML-1A).

---

### Superfície J — D14 override semantics

**Falso negativo (D14 remove hook de projeto ao instalar global; override suprime o global):**
Se o Antigravity usa semântica de override (projeto sobrepõe global), instalar o guard
globalmente via `update harness` e depois rodar `trackfw init` no projeto aplica D14
(`removeGuardFromNestedArrayInHooks`), que remove o guard do arquivo de projeto — e o projeto
fica sem guard porque o override do projeto (mesmo vazio) suprime o global.
Resultado: `validate` reporta "credential_guard installed (global)" e o guard nunca dispara.
Gate: ML-4A braço "deny" com sequência global→project init; confirmar se guard dispara no
projeto após D14. Requer conhecer a semântica de precedência (ML-1A, T6).

**Falso positivo (validate reporta ausente quando guard global cobre o projeto via merge):**
Se o Antigravity usa semântica de **merge** (global + projeto se somam), D14 remove o guard do
arquivo de projeto ao instalar globalmente, e o guard global continua ativo. Nesse caso:
- O projeto está protegido pelo guard global.
- Um validator de escopo projeto (que verifica `.agents/hooks.json`) reportaria o guard como
  ausente → `validate` vai vermelho num projeto efetivamente protegido.
Gate: depende da semântica de precedência medida em ML-1A. Medir antes de implementar o validator.

---

## 4. Residual Declarado

O que este design aceita não cobrir, dito claramente:

**R1 — Contrato do Antigravity completamente não medido.**
Toda a implementação das Waves 3–4 repousa sobre as medições da Wave 1 (ML-1A). Até que ML-1A
seja executado pelo KG no Mac e na VM Windows, as seguintes propriedades são desconhecidas:
- Se o Antigravity respeita exit 2 como deny, ou se requer JSON `{"decision":"deny"}` em stdout.
- O `tool_name` e os campos de comando no payload de stdin para ferramentas de shell e escrita.
- O cwd efetivo do processo de hook.
- O shell usado no Windows.
- A semântica de precedência global vs projeto (merge ou override).
- O comportamento com stdout vazio, JSON inválido, timeout, `enabled: false`.
- Se o CLI `agy` usa `~/.gemini/antigravity-cli/hooks.json` ou `~/.gemini/config/hooks.json`
  (os caminhos de hook são inferência a partir do padrão de agentes; não estão no `catalog.json`).

Aceito como residual: o guard do Antigravity não existe enquanto Wave 1 não for concluída.

**R2 — Caminhos globais não são cobertos por write-scope (limitação arquitetural).**
Os caminhos globais (`~/.gemini/config/hooks.json` confirmado por documentação oficial;
`~/.gemini/antigravity-cli/hooks.json` inferido) estão fora da raiz do projeto por definição. O `trackfw guard write-scope` só protege caminhos relativos dentro da
raiz. Um agente com acesso ao HOME pode modificar o guard global sem intervenção do write-scope.
Aceito como residual pré-existente (mesma limitação existe para `~/.claude/settings.json`,
`~/.codex/hooks.json`, etc.).

**R3 — Windows home resolution para Antigravity é INFERIDO, não medido.**
`HomeFor("antigravity")` atualmente retorna `homedir.Dir()` (HOME primeiro) por default. Se o
Antigravity no Windows usar o Windows API profile path, o guard global seria gravado no lugar
errado. Aceito como item de medição obrigatório na Wave 1 (ML-1A), não como defeito da Wave 0.
O padrão do defeito está documentado para Claude e Codex — não presumir que Antigravity tem o
mesmo comportamento sem medir.

**R4 — stdout JSON do `denyOut` usa `%s` não-escapado.**
Atualmente seguro porque todos os `reason` são constantes. Aceito como dívida técnica declarada:
qualquer adição de conteúdo dinâmico ao reason no ML-3A deve usar `json.Marshal` ou escaping
explícito antes de interpolação. Obrigação de implementação, não defeito hoje.

**R5 — Credential guard project scope verifica apenas o cwd (ADR D9, R-cred-2).**
O `guard credential` (project scope, `RunCredential`) faz `Stat(cwd/trackfw.yaml)` e retorna 0
se ausente — sem walk-up de diretórios. Este é o residual R-cred-2 pré-declarado (confirmado em
`docs/seguranca/2026-11-05-redteam-guards-primeiro-usuario-externo.md:88,570`): um hook Antigravity
invocado de qualquer diretório que não seja a raiz exata do projeto roda o guard em no-op
silencioso. O guard global (`--global`) não tem essa verificação — mas o harness global ainda não
existe para o Antigravity.

**R6 — Credential guard sem timeout (ADR D9, sem número de residual publicado).**
`io.ReadAll` em `RunCredential` e `RunCredentialGlobal` (`internal/guard/credential.go:196,210`:
comentário "no timeout — residual declared in ADR D9") bloqueia indefinidamente se stdin não
fechar. Para o Antigravity, o risco concreto é: se o Antigravity mantiver stdin aberto após enviar
o payload, o guard trava e o Antigravity nunca recebe a resposta. O que o Antigravity faz com um
hook que trava é desconhecido (medir ML-1A). ML-1A deve verificar se o Antigravity fecha stdin
imediatamente.

**R7 — Layer 1 do credential guard cobre campos desconhecidos como string bruta; Layer 2 não.**
Para payloads com schema desconhecido do Antigravity, Layer 1 (`credScanInput`) detecta
credenciais presentes como substrings no payload completo — confirmado em medição (ver T2).
Layer 2 (análise de redirecionamentos e arquivos referenciados pelo comando) não é executada
quando `shellCmd=""`. Credenciais em arquivos apontados por `--env-file` ou redirecionamentos
dentro de um payload de schema desconhecido não são detectadas pela Layer 2. Aceito como
cobertura parcial até que o campo de comando correto do Antigravity seja conhecido (ML-1A).
