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
**Files affected:** `docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` (seção "Medido")
**Acceptance criteria:**
- [ ] tabela plataforma × (shell do hook, payload do stdin, efeito de exit 2, efeito de `{"decision":"deny"}`, stdout
  vazio, JSON inválido, caminho global efetivo) preenchida, com contra-braço
- [ ] Gemini CLI: o binário instalado executa shell com chave de API? (contra-braço: sem chave)

## Wave 2 — ADR
### ML-2A — ADR do guard no Antigravity e legado do Gemini CLI
**Status:** ⬜ Pending
**Squad:** zeus-tf
**Acceptance criteria:**
- [ ] ADR em `docs/adr/` com: formato da linha de hook do Antigravity (e se o guard precisa de saída JSON —
  ex. `trackfw guard credential --format antigravity`), caminhos projeto/global, alvos de `update harness`, auditoria
  no `validate`/`doctor`, e o aviso de legado do Gemini
- [ ] REQ ligada à ADR

## Wave 3 — Implementação (MLs detalhados após a ADR, com arquivos e valores exatos)
> Dependências: Wave 2.
- ML-3A — saída no contrato do Antigravity no `trackfw guard credential`/`git-branch` (se a ADR exigir)
- ML-3B — `InjectAntigravityHooks` (projeto) + `cliDetectors`; alvos globais `antigravity-credential-guard`/
  `antigravity-git-branch-guard` no `update harness`
- ML-3C — auditoria no `validate`/`doctor` (mesmo caminho da escrita) + aviso de legado do Gemini
- ML-3D — `docs/cli-parity.md` e `catalog.json`

## Wave 4 — Prova real, revisão e red team
- ML-4A — KG roda no Antigravity real (Mac e Windows): segredo fake bloqueado, comando inócuo liberado
- ML-4B — revisão de qualidade (hefesto-tf)
- ML-FA — red team (hades-tf)
- ML-FB — `make quality` com a máquina ociosa e CI verde
