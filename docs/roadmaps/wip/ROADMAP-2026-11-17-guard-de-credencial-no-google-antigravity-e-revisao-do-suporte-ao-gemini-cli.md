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
- [ ] AC5 — Antigravity real bloqueia um segredo fake e libera um comando inócuo, no Mac e no Windows
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
- [ ] App e IDE (Mac e Windows): um braço allow com leitura de PATH e do shell do hook; macOS App aberto pelo Finder
  acha o `trackfw` no PATH?
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
- ML-FB — `make quality` com a máquina ociosa e CI verde
