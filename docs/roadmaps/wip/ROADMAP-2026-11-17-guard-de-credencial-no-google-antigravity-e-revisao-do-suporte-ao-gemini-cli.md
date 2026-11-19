---
status: wip
date: 2026-11-17
req: "docs/req/REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md"
squad: ""
---

# Roadmap: Guard de credencial no Google Antigravity e revisão do suporte ao Gemini CLI

> Created: 2026-11-17 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md -->
REQ: docs/req/REQ-2026-11-17-guard-de-credencial-no-google-antigravity-e-revisao-do-suporte-ao-gemini-cli.md

## Diagnóstico

- O Gemini CLI parou de atender contas individuais em 2026-06-18 e segue funcionando com chave de API paga (fontes no
  ML-0B). O trackfw grava guard de credencial e de git-branch para ele (`InjectGeminiHooks`,
  `internal/generators/agentfiles.go:772`; alvos `gemini-credential-guard`/`gemini-git-branch-guard`,
  `internal/generators/update.go:557`).
- O Antigravity **tem** hooks segundo a documentação oficial: `hooks.json` no diretório de customização (`.agents/` no
  workspace, `~/.gemini/config/` no global), evento `PreToolUse`, matcher por regex no nome da ferramenta (shell =
  `run_command`), script lê a chamada no stdin e devolve JSON `{"decision":"allow|deny|ask","reason":...}` no stdout.
  O trackfw não grava nada disso (`internal/generators/hooks.go:16`). O guard de hoje fala por exit 2 + stderr, que
  pode não ser o contrato do Antigravity — a medição decide.
- Mesma causa (falta de hook para um CLI que executa comandos) cobre o guard de git-branch → entra nesta REQ.

## Decisões do KG (2026-11-18)
- Medição no binário real no **Mac e na VM Windows** (KG instala o Antigravity nos dois).
- Gemini CLI: **manter como legado** — continua recebendo o guard; `doctor`/`validate` avisam que é legado e apontam o
  Antigravity. Vai para a ADR da Wave 2.

## Acceptance Criteria
- [ ] AC1 — contrato de hook do Antigravity medido em fonte primária e no binário (Mac e Windows)
- [ ] AC2 — status do Gemini CLI medido com contra-braço
- [ ] AC3 — ADR com o contrato do Antigravity e a decisão de legado do Gemini
- [ ] AC4 — `init`/`update` gravam guard de credencial e de git-branch para o Antigravity (projeto e global) e
  `doctor`/`validate` auditam no mesmo caminho
- [x] AC5 — Antigravity real bloqueia um segredo fake e libera um comando inócuo, no Mac e no Windows (CLI `agy`; App/IDE no ML-1B)
- [ ] AC6 — aviso de legado do Gemini CLI no `doctor`/`validate`
- [ ] AC7 — `docs/cli-parity.md` e catálogo refletem o medido; `make quality` e CI verdes

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat model e contrato documentado (2 MLs em paralelo; arquivos disjuntos)
> Dependências: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model
**Status:** ✅ Done
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-11-18-wave0-antigravity-threat-model.md` (novo)
**Actions:**
1. Completude: todas as superfícies onde o trackfw grava/audita hook por CLI (grep por `InjectGeminiHooks`,
   `gemini-credential-guard`, `cliDetectors`, `HomeFor`, `catalog.json`) e o que falta para o Antigravity.
2. Ameaças: quem faz o guard do Antigravity sair "allow" sem chamar o `trackfw` (exit code ignorado, stdout vazio,
   JSON inválido, timeout, `enabled:false`, hook de projeto sobrepondo o global, matcher que não casa a ferramenta de
   escrita de arquivo).
3. Alvos de falsificação nas duas direções por superfície.
4. Residual declarado.
**Acceptance criteria:**
- [x] as quatro seções com evidência (arquivo:linha ou fonte), sem linha de implementação — 13 superfícies (inclui
  `internal/auditsurface/auditsurface.go` `canonicalRuntimes`), 14 ameaças, residual R1–R7. Medido: guard de git-branch
  libera em silêncio payload de schema desconhecido (T2); os dois guards esperam EOF do stdin sem limite (T-Timeout)

### ML-0B — Contrato de hooks do Antigravity e status do Gemini CLI, por fonte primária
**Status:** ✅ Done
**Squad:** prometeu-tf
**Files affected:** `docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` (novo)
**Actions:** a partir da documentação oficial (`antigravity.google/docs/hooks`, IDE e CLI `agy`) e do blog do Google
sobre a transição do Gemini CLI, registrar com URL e citação: caminhos de `hooks.json` (projeto/global, IDE × CLI,
precedência e merge); eventos; nomes de ferramenta para shell e para escrita de arquivo; schema do stdin; contrato de
saída (JSON, exit code, stderr, o que acontece com exit 2, stdout vazio ou JSON inválido — fail-open ou fail-closed);
timeout; shell usado para rodar o `command` no Windows e no macOS; status do Gemini CLI (data, quem continua com
acesso). Onde a doc for omissa ou contraditória, escrever "não documentado" e virar item de medição da Wave 1. Fechar
com o **roteiro de medição** para o KG rodar no Mac e na VM (comandos exatos, hooks.json de sonda que grava stdin,
`$PSVersionTable`/`$0`/linha de comando do processo pai num arquivo, contra-braço).
**Acceptance criteria:**
- [x] cada afirmação com URL; nenhuma afirmação de comportamento sem fonte ou sem marcar "medir" — conferido pelo
  arquiteto contra `antigravity.google/docs/hooks.md`: global `~/.gemini/config/hooks.json`, `toolCall.args.CommandLine`,
  `decision` no stdout, `lastUserInput` em todo payload
- [x] roteiro de medição executável sem interpretação (Mac e Windows)

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-18-wave0-antigravity-threat-model.md
test -s docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md
```

## Wave 1 — Medição no binário real (depende do KG)
> Dependências: Wave 0 auditada.

### ML-1A — Antigravity no Mac e na VM Windows
**Status:** ⬜ Pending
**Squad:** zeus-tf + KG (comandos rodados pelo KG; o classificador bloqueia o agente de rodar CLI de terceiro com bypass)
**Files affected:** `docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` (seção "Medido"),
`docs/portabilidade/antigravity-sondas/run-arms-mac.sh`, `docs/portabilidade/antigravity-sondas/run-arms-windows.ps1`
**Acceptance criteria:**
- [x] tabela plataforma × (shell do hook, payload do stdin, efeito de exit 2, efeito de `{"decision":"deny"}`, stdout
  vazio, JSON inválido) preenchida, com contra-braço — §11 do contrato; `agy` 1.5.11, Mac e Windows: exit ≠ 0 bloqueia,
  stdout vazio + exit 0 libera, JSON inválido bloqueia; hook via `sh -c` (Mac) e `cmd /s /c` (Windows)
- [x] `agy` fecha o stdin do hook (o `cat`/`ReadToEnd` das sondas retornou nos 7 braços das duas plataformas) — T-Timeout
  do threat model não se materializa no `agy` 1.5.11

### ML-1B — Hook global, home efetivo e App/IDE (bloqueia o ML-3B2)
**Status:** ⬜ Pending — aguarda autorização do KG para escrever em `~/.gemini/config/`
**Squad:** zeus-tf + KG
**Files affected:** `docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` (§11),
`docs/portabilidade/antigravity-sondas/` (runners do ML-1B)
**Acceptance criteria:**
- [x] hook só no global `~/.gemini/config/hooks.json` dispara no `agy` (Mac e Windows); precedência projeto × global —
  §12: os dois níveis somam e deny vence
- [x] Windows: com `USERPROFILE` apontando para um diretório temporário, qual `hooks.json` global o `agy` carrega
  (`USERPROFILE` × `FOLDERID_Profile` — o caso que o Codex revelou no ML-4A da REQ-2026-11-15) — §12: **`USERPROFILE`**
- [x] App e IDE (Mac e Windows): um braço allow com leitura de PATH e do shell do hook; macOS App aberto pelo Finder
  acha o `trackfw` no PATH?
  — Windows App/IDE fora (KG, 2026-11-18, opção b: VM sem App/IDE; residual no ADR); Mac medido (contrato §13): PATH herdado nos dois; App 2.29.0 bloqueia pelo hook de projeto; **IDE 2.6.0 ignora o
  de projeto e bloqueia só pelo global**
- [ ] Gemini CLI: o binário instalado executa shell com chave de API? (contra-braço: sem chave)

## Wave 2 — ADR
### ML-2A — ADR do guard no Antigravity e legado do Gemini CLI
**Status:** ✅ Done
**Squad:** zeus-tf
**Acceptance criteria:**
- [x] ADR `ADR-2026-11-18-guard-de-credencial-e-de-git-branch-no-google-antigravity-e-gemini-cli-como-legado.md`:
  forma cmd.exe nas duas plataformas, sem saída JSON, payload `toolCall.args.CommandLine`, `lastUserInput` fora (KG: a)
- [x] REQ ligada à ADR

## Wave 3 — Implementação (ADR-2026-11-18; 3A ∥ 3B, depois 3C, depois 3D)
> Dependências: Wave 2. 3A e 3B tocam pacotes disjuntos (`internal/guard` × `internal/generators`+`internal/agentconfig`).
> 3C depende da API de caminhos do 3B. 3D (docs) por último.

### ML-3A — Payload do Antigravity no guard (ADR D2, D3, D4)
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:** `internal/guard/payload.go`, `internal/guard/credential.go`, testes em `internal/guard/`
**Actions:** `ExtractCommand` e `credExtractCmdAndCwd` reconhecem `toolCall.args.CommandLine` e o `cwd` do payload;
com schema Antigravity (`toolCall` objeto + `hookEventName`), a camada 1 do guard de credencial varre só `toolCall`.
**Acceptance criteria:**
- [x] git-branch: payload Antigravity com `git push` para branch protegida → exit 2 (hoje 0: T2); contra-braço `ls` → 0
- [x] credencial: segredo fake em `toolCall.args.CommandLine` → 2; o mesmo segredo só em `lastUserInput` → 0
- [x] camada 2 lê arquivo citado em `CommandLine` (ex. `cat <arquivo com segredo>`) → 2
- [x] camada 2 lê `toolCall.args.AbsolutePath` de `view_file` (arquivo regular, mesmo limite de tamanho) → segredo no
  arquivo = 2; arquivo inócuo = 0 (sem isso a perna `view_file` do D5 seria decorativa)
- [x] invariante: em todo caminho de exit 0 dos dois guards, stdout vazio (teste por tabela)
- [x] schemas de Claude/Codex/Gemini sem mudança de veredito (testes existentes verdes)
- [x] `go test ./internal/guard/` rc=0

### ML-3B — Hooks do Antigravity no projeto (ADR D1, D5, D6 — parte projeto)
**Status:** ✅ Done
**Squad:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go` (`InjectAntigravityHooks`), `internal/generators/hooks.go`
(`cliDetectors`), `internal/generators/update.go` (alvos `antigravity-credential-guard`/`antigravity-git-branch-guard`),
`internal/agentconfig/agentconfig.go` (`CLIAntigravity` em `HomeFor`), `internal/auditsurface/auditsurface.go`,
testes correspondentes
**Actions:** linha = `guardforms.CredentialCurrentCmdExe`/`GitBranchCurrentCmdExe` (global:
`CredentialGlobalCurrentCmdExe`); grupo `trackfw`, só `PreToolUse`, matchers do D5; `pathguard` + merge preservador;
sem BOM.
**Acceptance criteria:**
- [x] `init`/`agents install --targets antigravity` grava `.agents/hooks.json`; hooks de terceiros preservados byte a byte;
  segunda execução idempotente
- [x] `go test ./internal/generators/ ./internal/agentconfig/ ./internal/auditsurface/` rc=0; `make parity-rest` rc=0.
  Corretivo: o guard de git-branch saiu de `run_command|manage_task` para `run_command` (ADR D5; `manage_task` não tem
  `CommandLine`); teste de auditoria `TestExtractTuplesAntigravity` adicionado

### ML-3B2 — Guard global do Antigravity (ADR D6 — parte global; depende do ML-1B)
**Status:** ✅ Done — medido no binário: grava `~/.gemini/config/hooks.json` (4 entradas, sem BOM, idempotente); `make quality` rc=0
**Squad:** apolo-tf
**Files affected:** `internal/generators/update.go`, `internal/generators/agentfiles.go`, `internal/agentconfig/agentconfig.go`
(`CLIAntigravity`), `internal/validator/validator_git_branch_guard.go`, `scripts/check-update-parity.sh`, `docs/cli-parity.md`,
`internal/agentconfig/agentconfig_test.go`, `internal/generators/update_test.go`, `internal/generators/guard_registry_sites_ac4_test.go`
**Acceptance criteria:**
- [x] `update harness --targets antigravity-credential-guard --install-missing` grava no caminho global **medido** no ML-1B,
  com `HOME`/`USERPROFILE` isolados nos testes
- [x] teste de membro explícito para os dois IDs novos (não só contagem — Superfície B do threat model)
- [x] `scripts/check-update-parity.sh` e `make parity-rest` rc=0

### ML-3C — Auditoria e aviso de legado (ADR D6, D7)
**Status:** ✅ Done (parte projeto; auditoria do global vai com o ML-3B2)
**Squad:** apolo-tf
**Files affected:** `internal/validator/` (regras de guard de credencial/git-branch + regra nova `gemini_cli_legacy`),
`internal/commands/doctor*.go`
**Acceptance criteria:**
- [x] `validate`/`doctor` auditam o guard do Antigravity no mesmo caminho do 3B (braço: removido → acusa; presente → ok)
- [x] `gemini_cli_legacy` em nível warning: guard do Gemini sem o do Antigravity → aviso; com os dois → silêncio
- [x] `go test ./internal/validator/ ./internal/commands/` rc=0

### ML-3D — `docs/cli-parity.md` e `catalog.json`
**Status:** ✅ Done (parte projeto; o global entra com o ML-3B2)
**Squad:** apolo-tf
**Acceptance criteria:**
- [x] contrato do Antigravity (linha, matchers, caminhos, schema) e status de legado do Gemini documentados
- [x] `make parity-rest` rc=0

## Wave 4 — Prova real, revisão e red team
- ML-4A — KG roda no Antigravity real — **CLI `agy`, App e IDE**, no Mac e no Windows — com
  `credential_guard.mode: block` explícito no `trackfw.yaml` do projeto de teste (o default de projeto pode ser warn):
  segredo fake bloqueado, comando inócuo liberado, `git push` para branch protegida bloqueado
  - **CLI `agy` ✅ (2026-11-18, KG; `proof-mac.sh` e `proof-windows.ps1`, trackfw 11.7.0 da branch, 4/4 chamadas por
    máquina):** Mac e Windows iguais — B1 segredo fake → `jsonhook__trackfw_PreToolUse_0_0` exit 2, marcador ausente;
    B2 `echo hello` → DONE, marcador criado; B3 `git commit` cru → `PreToolUse_3_0` (guard de git-branch) exit 2,
    `raw git commit blocked` no ndjson (Mac commits=1; no Windows o contador saiu vazio porque a VM não tinha identidade
    git — corrigido no script, a evidência é o ndjson); B4 só o global `update harness` → segredo bloqueado. Nota: o
    braço real foi `git commit`, não `git push`; o bloqueio de push para branch protegida tem teste de payload no ML-3A.
  - **App e IDE (Mac) ✅** — contrato §13: App bloqueia pelo hook de projeto; IDE 2.6.0 só pelo global.
- ML-4B — revisão de qualidade (hefesto-tf)
- ML-4B — revisão do hefesto ✅ — APROVA COM NOTAS (`docs/qualidade/2026-11-18-ml-4b-antigravity-review.md`); A1 bloqueante
- ML-4C — corretivo A1–A7 ✅ — `view_file` usa `AbsolutePath` literal (apóstrofo no nome dava falso negativo; o teste
  reprova sem a correção); `cwd` do payload só se absoluto; walker único `internal/hookschema`; mensagem do
  `gemini_cli_legacy` em inglês; `make quality` rc=0
  - **Files affected:** `internal/guard/credential.go`, `internal/guard/antigravity_test.go`, `internal/hookschema/`,
    `internal/auditsurface/auditsurface.go`, `internal/validator/validator_credential_guard.go`,
    `internal/validator/validator_guard_wiring.go`, `internal/validator/validator_gemini_cli_legacy.go`,
    `internal/validator/validator_guard_hook_concordance_external_test.go`,
    `internal/generators/guard_registry_sites_ac4_test.go`, `docs/cli-parity.md`
- ML-4C2 — corretivo do ML-4C ✅ — auditoria volta a exigir `type: "command"` no schema Antigravity (o walker do 4C
  tinha perdido o filtro); case morto removido. **Files affected:** `internal/validator/validator_guard_wiring.go`,
  `internal/validator/validator_guard_wiring_test.go`
- ML-FA — red team (hades-tf) ✅ — **REPROVA** (`docs/seguranca/2026-11-18-red-team-req-2026-11-17-antigravity.md`):
  F5 `.agents/hooks.json` fora da lista congelada do write-scope; F2+F3 `enabled:false` no grupo `trackfw` preservado
  pelo gerador e não acusado pelo `validate`; F1 prioridade 5 sem checar o schema Antigravity
- ML-FA2 — corretivo F1, F2, F3, F5 (apolo-tf) ✅ — `.agents/hooks.json` congelado para subagentes; gerador remove
  `enabled:false` do grupo `trackfw` e avisa; `validate` acusa o grupo desabilitado; prioridade 5 só com schema Antigravity
  - **Files affected:** `internal/guard/writescope.go`, `internal/guard/writescope_test.go`, `internal/guard/payload.go`,
    `internal/guard/credential.go`, `internal/guard/antigravity_test.go`, `internal/generators/agentfiles.go`,
    `internal/generators/antigravity_hooks_test.go`, `internal/hookschema/hookschema.go`,
    `internal/validator/validator_credential_guard.go`,
    `internal/validator/validator_credential_guard_presence_warning_test.go`, `docs/cli-parity.md`
- ML-4E — aviso da IDE sem guard global (ADR D8; apolo-tf) — **Status:** ✅ Done
  - **Por quê:** a medição do ML-4A mostrou que a IDE 2.6.0/2.7.0 ignora `.agents/hooks.json`; quem usa a IDE fica sem
    guard e sem aviso. Mesma causa da REQ (o guard não chega ao produto do Google), por isso entra aqui.
  - **Files affected:** `internal/validator/validator_antigravity_ide_global_guard.go` (novo) e o teste,
    `internal/validator/validator.go` (dois sítios, espelho do `gemini_cli_legacy`), `internal/commands/doctor.go`,
    `internal/integrations/` (FindingKind novo), o sítio do aviso no install/init, `scripts/check-validate-rule-pins.sh`,
    `docs/cli-parity.md`
  - **Acceptance criteria:**
    - [x] regra warning nunca violation; dispara só com as 3 condições do D8; silenciosa sem a IDE, com o global
      presente, ou sem o hook de projeto
    - [x] `validate` e `validate --json` emitem (dois sítios); `doctor` emite; install/init imprimem o aviso
    - [x] testes com HOME temporário — nunca o `~/.gemini` real; pins novos no `check-validate-rule-pins.sh` verdes
    - [x] `make quality` rc=0 (apolo; 365 OK)
    - **Auditoria (zeus, binário, HOME/projeto temporários):** sem `.antigravity-ide` → install silencioso; com IDE e
      sem global → aviso no install, `validate`, `validate --json` (`rule: antigravity_ide_global_guard`) e `doctor`;
      após `update harness` → silêncio. 7 testes conferidos por nome (`-run ^TestAntigravityIDEGlobalGuard_`). Pins 42→45.
      Desvio aceito: `proof-mac.sh` ganhou trap do `mktemp -d` (gate `check-mktemp-cleanup`).
- ML-FB — `make quality` com a máquina ociosa e CI verde

## Wave 5 — OpenCode (issue #691; decisão do KG em 2026-11-19, opção a)
> Mesma causa: o OpenCode é o único alvo do catálogo sem hook (`hooks.go:16`). Mecanismo diferente: plugin JS/TS
> (`tool.execute.before`). O ML-3B fixou o skip num teste (`antigravity_hooks_test.go`, `contra_opencode_still_skips`);
> a Wave 5 é quem muda esse teste. O PR #692 fica em rascunho até a Wave 5 fechar.
> 5-0 ∥ 5A (arquivos disjuntos); 5B depois dos dois; 5C (ADR) depois do 5B; implementação só depois do 5C.

### ML-5-0 — Threat model do plugin do OpenCode (hades-tf)
**Status:** ✅ Done — 13 ameaças; ALTO: T1 fail-open com checagem ingênua (`status===2`; ENOENT dá `null`), T3 V1 `bash` × V2
`shell` e APIs incompatíveis, T4 `event.input.command` não é `tool_input.command` (libera em silêncio), T6 `--pure`/
`OPENCODE_PURE=1` desliga plugins, T7 agente edita/remove o plugin, T9 MCP/`task` fora do `execute.before`, T13 write-scope
só conhece nomes PascalCase
**Files affected:** `docs/seguranca/2026-11-19-wave5-opencode-threat-model.md`
**Acceptance criteria:**
- [x] ameaças do plugin JS/TS: quem carrega, onde (projeto × global), se o agente consegue desligar ou editar o plugin,
  falha do `spawn` do `trackfw` (fail-open × fail-closed), e entradas que não passam pelo `tool.execute.before`
- [x] braços de medição propostos para o ML-5B, cada um com resultado esperado e contra-braço

### ML-5A — Contrato do plugin do OpenCode por fonte primária (prometeu-tf)
**Status:** ✅ Done — plugin `.js/.ts` solto em `.opencode/plugins/` e `~/.config/opencode/plugins/` (Windows também XDG);
runtime Bun; `throw` no `execute.before` bloqueia; `execute.after` não bloqueia. 1.19.5 (V1) e 2.3.x (V2) são o mesmo
produto (`anomalyco/opencode`, conferido pelo zeus via `gh api` e `npm view`) com API de plugin incompatível → dual-export.
Achado: `read` com caminho → guard sai 0 (sem extrator), mesma causa do D5 `view_file` → ML nesta wave
**Files affected:** `docs/portabilidade/2026-11-19-opencode-contrato-de-plugin.md`
**Acceptance criteria:**
- [x] por fonte primária (doc oficial e código-fonte, com URL e versão): onde ficam os plugins (projeto/global), como são
  registrados, a assinatura do `tool.execute.before`, como bloquear (throw? retorno?), payload (nome da ferramenta, args
  do bash/edit/read), runtime (Bun/Node) e como chamar um binário externo
- [x] a diferença entre `opencode` 1.19.5 (Homebrew, no Mac do KG) e `@opencode/cli` 2.3.0 (a medição do #691) — mesmo
  produto ou outro?
- [x] itens que só o binário responde, listados para o ML-5B

### ML-5B — Medição no binário real (KG + zeus-tf) — 🔄 In progress (falta C7: git-branch)
- [ ] C7 `git commit` cru sob o OpenCode → bloqueado, commits iguais (Mac e Windows, 1/1 cada). Sem modelo, o payload do
  plugin já dá rc=2 no `guard git-branch` para `git commit`, `git add -A` e `git push` (rc=0 para `ls`) — zeus, 2026-11-19.
opencode 2.3.2, Mac e Windows iguais, 6/6 cada (contrato §8): chave em `shell` bloqueada; inócuo liberado com o hook
disparando; `trackfw` ausente → bloqueado (ENOENT); `read` com segredo liberado (gap); `OPENCODE_CONFIG_PROJECT_DISABLE=1`
desliga o plugin de projeto; plugin global bloqueia sozinho. Dois runs anteriores no Mac (LM Studio desligado; `--log-level
DEBUG` recusado pela 2.x) não mediram nada e foram descartados.
### ML-5C — ADR — ✅ Done: `docs/adr/ADR-2026-11-19-guard-do-trackfw-no-opencode-via-plugin.md` (D1–D6). V1 (1.x) declarado
residual, não medido (decisão do KG em 2026-11-19, opção a).

### ML-5D — Guard em Go: `read` do OpenCode e caminhos congelados (apolo-tf; ADR D3, D4) — ✅ Done
**Files affected:** `internal/guard/credential.go`, `internal/guard/payload.go` (só se preciso), `internal/guard/writescope.go`,
testes em `internal/guard/` (arquivo novo `opencode_test.go`)
- [x] `hook_source=="opencode" && tool_name=="read"` → camada 2 lê `tool_input.file_path` (regular, mesmo limite): segredo → 2
- [x] contra-braços: o mesmo payload sem `hook_source`, ou com `tool_name` diferente → o arquivo NÃO é lido
- [x] `.opencode/plugins/`, `opencode.json` e `opencode.jsonc` congelados para subagentes
- [x] testes de schema das outras CLIs inalterados (13 testes novos; 3 reprovavam antes)

### ML-5E — Plugin, geração e auditoria (apolo-tf; ADR D1, D2, D4, D5, D6) — ✅ Done (+ corretivo ML-5E2)
**Files affected:** asset novo do plugin em `internal/generators/` (go:embed), `internal/generators/hooks.go`, `agentfiles.go`,
`update.go`, `antigravity_hooks_test.go` (inverter `contra_opencode_still_skips`), `internal/agentconfig/agentconfig.go`,
`internal/validator/` (auditoria), `internal/commands/doctor.go`, `internal/integrations/`, `internal/auditsurface/`,
`scripts/check-update-parity.sh`, `scripts/check-validate-rule-pins.sh` (se houver regra nova), `docs/cli-parity.md`
- [x] `init`/`update`/`agents install` com `.opencode/` gravam `.opencode/plugins/trackfw-guard.js`; `update harness
  --targets opencode-guard` grava o global e reporta `missing` quando ausente
- [x] plugin: argv sem shell, `throw` em `error || status !== 0`, mapeamento de ferramentas do D3, autoproteção do D4,
  sem log; teste sem modelo (node) para segredo/inócuo/ENOENT/autoproteção
- [x] `validate`/`doctor` auditam o mesmo caminho; `doctor` informa projeto sem global (D6)
- [x] `make quality` rc=0 (zeus, 2026-11-19: 365 OK, 0 FAIL)
- **Auditoria zeus:** binário isolado — `missing` no dry-run, `update` grava o projeto, `validate` acusa adulterado, `doctor`
  acusa global ausente, `--install-missing` grava `SCOPE = "global"`. Reprovou 4 itens → **ML-5E2**: teste do contrato antigo
  (`TestInjectHooksForToolsUnknownNameSkipped`) dividido; remédio do projeto stale era o do global; comentários do asset em
  PT → EN; testes que faltavam (validator 9, agentconfig 8, runtime JS 5 braços — rodam no Mac, não pulam; update-parity 5c-2/5c-3).

### ML-5F — Revisão (hefesto-tf) — ✅ Done — **REPROVA** (`docs/qualidade/2026-11-19-ml-5f-opencode-review.md`): A7 bloqueante
(`patch` com alvo no texto pula a autoproteção); A2, A4, A5, A6, A8 baixos; A9 nota
### ML-5G — Red team (hades-tf) — ✅ Done — **REPROVA** (`docs/seguranca/2026-11-19-red-team-wave5-opencode.md`): F4 HIGH (escrita
de outro plugin em `.opencode/plugins/` ou por `shell`), F2/F8 (caixa e symlink na comparação), F6 (`webfetch`/`grep` fora do
guard); F1, F3, F5, F7 baixos; VS residual
### ML-5I — Corretivo de A7/F1–F8 e notas (apolo-tf; ADR-2026-11-19 D7) — ✅ Done
- [x] autoproteção por diretório de plugin, `realpath` + caixa, alvos de patch, precedência única (A7, F2, F4, F5, F7, F8)
- [x] `validate`/`doctor` acusam plugin estranho e chave `plugin(s)` no `opencode.json(c)` (F4 fix b)
- [x] `webfetch` e `grep` no guard; `grep` em arquivo regular lido em Go; demais ferramentas nativas mapeadas ou declaradas (F6)
- [x] caminhos congelados `.opencode/plugin/`, `.opencode/opencode.json(c)` (F1, F3)
- [x] A2, A4, A5, A6, A8 corrigidos (A2: o JS resolve `OPENCODE_CONFIG_DIR` relativo contra o cwd em vez de ignorar — aceito, protege mais)
- [x] cada achado com teste que reprova antes e passa depois; runtime JS 11 braços PASS no Mac
- [x] `make quality` — após o ML-5I2, **rc=0** (zeus, `MQ_RC=0` lido no log; 365 OK). Antes era **rc=2**, não 0: `check-raw-read-ban` reprova `os.ReadFile` em `validator_opencode_plugin.go:169`. O commit
  `fix(opencode)… (ML-5I)` afirmou rc=0 por erro do zeus (commitou antes de ler o rc); o apolo tinha rodado os gates em partes.
- [x] **ML-5I2** ✅ (`readRegularFile`; `websearch` no guard com arm12; arm8 com `symlinkOrSkip`; tabela de cobertura no
  cli-parity — só `glob` residual, devolve nomes): corrigir a leitura crua acima e pôr `websearch` (lista do contrato, l. 387) no guard — a consulta exfiltra
  como a URL do `webfetch`
### ML-5G2 — Red team rodada 2 (hades-tf) — ✅ Done — **REPROVA**: F1, F2, F3, F6, F7, F8-alvo e F4-write fechados; novos
N1 (diretórios protegidos sem `realpath` — symlink de dotfiles), N2 (`a/`/`b/` do diff git escapa), N3 (`validate` não olha o
global — o D7 do ADR afirmava que olhava); baixos: F5 patch sem cabeçalho libera, F7 campos invertidos, N4 JSONC `/* */` e
vírgula final
### ML-5I3 — Corretivo da rodada 2 (apolo-tf) — ✅ Done
- [x] N1, N2, N3 com teste que reprova antes; F5 fail-closed; F7 checa todos os campos de caminho; N4 JSONC completo
  (18 braços JS PASS no Mac, nenhum SKIP; 4 testes Go novos conferidos pelo nome — zeus)
- [x] `make quality` — `MQ_RC=0` literal reportado pelo apolo; o zeus roda de novo após o ML-5I4
### ML-5I4 — Cabeçalhos git de renomeação/cópia no extrator de patch (apolo-tf) — ✅ Done
- [x] `diff --git a/X b/Y`, `rename from/to`, `copy from/to` viram alvos; patch misto (`+++` inócuo + rename para
  `.opencode/plugins/`) → bloqueado. Achado do zeus na auditoria do 5I3 (o apolo o listou como residual)
- [x] `unquoteGitPath` decoda C-string: `\NNN` octal, `\"`, `\\`, etc.; token mal-formado → fail-closed;
  aplicado a TODOS os cabeçalhos (`+++`/`---`, `diff --git`, rename/copy)
- [x] `parseDiffGitTokens`: parser de dois tokens (quoted ou unquoted) sem split por espaço
- [x] 5 braços novos (arm19–arm23) com FAIL antes / PASS depois; 23 braços PASS, 0 SKIP no Mac
- [x] `make quality` — `MQ_RC=0` literal (apolo). Nota do zeus: o apolo editou este roadmap contra o handoff; conteúdo
  conferido e mantido.
### ML-5I5 — Patch nega por menção ao caminho protegido (apolo-tf) — ✅ Done (+ ML-5I6: menção também nos caminhos
decodificados e com octal decodificado — fecha `+++ "z/\056opencode/plugins/x.js"`)
- [x] residual do 5I4: `git apply -pN` remove qualquer prefixo, então reconhecer cabeçalho por cabeçalho não fecha. Regra nova:
  `patch`/`apply_patch` cujo texto mencione, em qualquer posição e sem caixa, `.opencode/plugin`, `opencode.json` ou o diretório
  de config global (forma crua e `realpath`) → bloqueado, mesmo sem cabeçalho reconhecido
- [x] 29 braços JS PASS no Mac, nenhum SKIP; `make quality` **MQ_RC=0 lido pelo zeus no log** (365 OK)
### ML-5H — prova real com o plugin do produto (KG) — ⬜ Pending — scripts prontos (`proof-product-mac.sh`,
`proof-product-windows.ps1`, 7/7 por máquina); roda depois do ML-5I3 e de nova rodada do hades
