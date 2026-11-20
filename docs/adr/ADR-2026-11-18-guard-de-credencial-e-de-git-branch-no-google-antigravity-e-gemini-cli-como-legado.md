---
status: Accepted
date: 2026-11-18
author: "zeus-tf (decisões do KG em 2026-11-18)"
---

# ADR: guard de credencial e de git-branch no Google Antigravity e Gemini CLI como legado

> Date: 2026-11-18 | Status: Accepted

## Context

O Gemini CLI parou de atender contas individuais em 2026-06-18, e o fluxo do Google passou para o Antigravity
(App, IDE e CLI `agy`). O trackfw grava os guards para o Gemini CLI e nenhum hook para o Antigravity
(`internal/generators/hooks.go:16`). Quem migrou ficou sem o guard de credencial e sem aviso.

Medido no `agy` 1.5.11, no macOS e no Windows 11 ARM64 (REQ-2026-11-17, ML-1A;
`docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md` §11):

- Os hooks ficam em `hooks.json`: `.agents/hooks.json` no projeto e `~/.gemini/config/hooks.json` no global (doc
  oficial; o global ainda não foi medido). O evento que pode bloquear é o `PreToolUse`, com matcher por regex sobre
  `toolCall.name`. O `PostToolUse` não bloqueia.
- **Qualquer exit ≠ 0 bloqueia; exit 0 com stdout vazio libera**; stdout que não é JSON válido bloqueia.
  `{"decision":"deny"}` também bloqueia.
- O `command` roda em `sh -c` no macOS e em `cmd /s /c` no Windows. O cwd é `.agents/`, e no Windows `HOME` chega
  vazio.
- O payload traz o comando em `toolCall.args.CommandLine`, mais `cwd`, `workspacePaths` e `lastUserInput` (o prompt
  do usuário).

Threat model: `docs/seguranca/2026-11-18-wave0-antigravity-threat-model.md`. Dois achados medidos lá pesam aqui: o
guard de git-branch libera em silêncio payload de schema desconhecido (T2), e os dois guards esperam EOF do stdin.

## Decision

**D1 — Linha de hook.** O Antigravity usa a forma cmd.exe que já existe:
`guardforms.CredentialCurrentCmdExe` (`trackfw guard credential || exit 2`), `GitBranchCurrentCmdExe` e
`CredentialGlobalCurrentCmdExe` para o global. Ela é válida em `sh -c` e em `cmd /s /c`, que são os dois shells
medidos. Um `hooks.json` versionado no projeto funciona então nas duas plataformas. Se o `trackfw` faltar no PATH,
a linha sai com 127 (sh) ou 9009 (cmd) e vira `exit 2`: fail-closed. A forma PSPOSIX não serve, porque `cmd` não a
interpreta.

**D2 — Contrato de saída sem mudança.** Exit 2 bloqueia e exit 0 silencioso libera. Não há `--format antigravity`.
Invariante nova, com teste: **o guard nunca escreve no stdout quando libera** (texto não-JSON com exit 0 bloquearia
todo comando).

**D3 — Payload.** `guard.ExtractCommand` (`internal/guard/payload.go`) e `credExtractCmdAndCwd`
(`internal/guard/credential.go`) passam a reconhecer `toolCall.args.CommandLine` como comando e o `cwd` do payload
como cwd. Isso fecha o T2 para o Antigravity e liga a camada 2 do guard de credencial.

**D4 — `lastUserInput` fora da varredura (decisão do KG, opção a).** Quando o payload tem o schema do Antigravity
(objeto `toolCall` + `hookEventName`), a camada 1 do guard de credencial varre `toolCall` e não o payload inteiro.
O prompt já foi entregue ao modelo; bloquear o comando seguinte não protege o segredo e gera falso positivo.

**D5 — Matchers (só `PreToolUse`).**
- Guard de credencial: `run_command|manage_task`, `write_to_file|replace_file_content|multi_replace_file_content` e
  `view_file`. É o equivalente das três pernas do Gemini; a perna `AfterTool` não tem equivalente porque o
  `PostToolUse` não bloqueia.
- Para a perna `view_file` não ser decorativa, a camada 2 lê o arquivo de `toolCall.args.AbsolutePath`: só arquivo
  regular, com o mesmo limite das leituras de hoje. No Gemini, a perna `read_file` só pegava segredo na saída
  (`AfterTool`), e isso não existe no Antigravity.
- Guard de git-branch: `run_command`.

**D6 — Onde gravar e auditar.**
- Projeto: `InjectAntigravityHooks(rootDir)` grava `.agents/hooks.json` com o mesmo `pathguard` e o mesmo merge
  preservador das outras CLIs, num grupo nomeado `trackfw` (o formato agrupa por nome).
- Detecção em `cliDetectors`: diretório `.agents/`.
- Global: alvos `antigravity-credential-guard` e `antigravity-git-branch-guard` no `update harness`. 🔴 O caminho
  e o home efetivo (`USERPROFILE` × `FOLDERID_Profile` no Windows) só entram depois de medidos no ML-1B; até lá a
  doc diz `~/.gemini/config/hooks.json`. Não repetir o erro do Codex, cujo home foi presumido pelo fonte e
  desmentido pelo binário.
- `validate`/`doctor` auditam no mesmo caminho em que a escrita grava.
- `auditsurface.canonicalRuntimes` inclui o Antigravity.

**D7 — Gemini CLI como legado (decisão do KG).** O trackfw continua gravando e auditando os guards do Gemini CLI,
porque quem tem chave de API paga segue usando. Uma regra do `validate` em nível **warning** (nunca violation)
avisa quando há guard do Gemini CLI e nenhum do Antigravity no projeto: diz que o Gemini CLI é legado para contas
individuais desde 2026-06-18 e aponta `trackfw agents install --targets antigravity`. O `doctor` repete o aviso.
Medido em 2026-11-20 (contrato §14): o `gemini` 0.53.0 com login Google de conta individual é recusado com
`IneligibleTierError`, e o contra-braço sem credencial dá erro de autenticação. O caminho por chave de API não foi medido.

**D8 — A IDE do Antigravity só respeita o guard global (decisão do KG em 2026-11-19, opção b).** Medido no macOS
(contrato §13): a IDE 2.6.0 e a 2.7.0 ignoram `.agents/hooks.json` e bloqueiam pelo `~/.gemini/config/hooks.json`.
Uma regra nova do `validate`, `antigravity_ide_global_guard`, em nível **warning** (nunca violation, via
`applyRuleWarnOnly`), dispara quando as três condições valem:
1. o projeto tem guard de credencial em `.agents/hooks.json`;
2. a IDE está instalada: `<HomeFor(CLIAntigravity)>/.antigravity-ide` é diretório. No macOS isso foi medido
   (`~/.antigravity-ide/extensions`); no Windows o caminho não foi medido;
3. o `hooks.json` global (`AntigravityConfigDir`) não tem guard de credencial.

A mensagem diz que a IDE ignora o hook do projeto e manda rodar
`trackfw update harness --targets antigravity-credential-guard --install-missing`. A mesma função alimenta o `doctor`
e o aviso impresso pelo `agents install`/`init` quando gravam o hook do Antigravity no projeto. Uma função só, três
consumidores.

## Consequences

- Quem usa Antigravity passa a ter guard de credencial e de git-branch, no projeto e no global.
- O `hooks.json` do projeto é o mesmo para macOS e Windows. Não há forma por plataforma.
- A extração de payload ganha um schema. Os testes de schema das outras CLIs fixam que nada muda para elas.
- O hook de atenção do board não tem equivalente no Antigravity (não existe evento `Notification`). Fica como
  residual declarado.

**Escopo da medição:** os resultados do Context são do CLI `agy` lançado por terminal. App e IDE leem o mesmo
`hooks.json` segundo a doc, mas o shell do hook e o PATH (app de macOS aberto pelo Finder não herda o PATH do shell)
são medidos no ML-1B antes do guard global ser entregue. Se o `trackfw` faltar no PATH do App, a linha do D1
fecha (exit 2) e **bloqueia todo comando**. Isso é seguro, mas inutilizável, e precisa de aviso no `doctor`.

**Detecção por `.agents/`:** o Codex também usa `.agents/skills/`. Num projeto só de Codex, o `trackfw update` grava
um `.agents/hooks.json` que só o Antigravity lê. É inofensivo e idempotente, e foi aceito para não tornar a detecção
frágil.

**Residual:**
- `manage_task` com `send_input` carrega texto, não `CommandLine`. A camada 1 vê o segredo, mas o guard de
  git-branch não vê o comando.
- **IDE do Antigravity 2.6.0 e 2.7.0 (macOS) não lê `.agents/hooks.json`; só o global protege** (medido, contrato §13). O App
  2.29.0 lê o do projeto. Quem usa a IDE precisa do `update harness --targets antigravity-credential-guard`.
- Um `trackfw` anterior a esta REQ primeiro no PATH sai 0 para o payload do Antigravity: o hook existe e não protege
  (medido no App com 11.5.0).
- App e IDE do Antigravity no **Windows** não foram medidos (decisão do KG em 2026-11-18, opção b): a VM não tem
  App/IDE e o `trackfw` não está no PATH do usuário. Sem `trackfw` no PATH, a linha do D1 fecha (9009 → exit 2) e
  bloqueia todo comando; o `doctor` deve ser o aviso. Só o CLI `agy` tem prova real no Windows.
- Precedência projeto × global, `enabled:false` e timeout ainda não foram medidos. A Wave 4 mede no binário real
  antes do merge.

## Alternatives Considered

- **Saída JSON (`--format antigravity`)**: rejeitada, porque o exit 2 já bloqueia nas duas plataformas (medido).
  Seria um contrato a mais sem ganho.
- **Forma PSPOSIX**: rejeitada, porque o Windows roda o hook em `cmd /s /c`.
- **Uma forma por plataforma**: rejeitada, porque quebraria o `hooks.json` versionado entre máquinas.
- **Varrer `lastUserInput`** (opção b): rejeitada pelo KG, porque é falso positivo sem proteção real.
- **Remover os hooks do Gemini CLI**: rejeitada pelo KG, porque desprotege quem usa chave paga.
