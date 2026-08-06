---
status: Open
date: 2026-08-05
author: "kg.saran@gmail.com"
adr: "docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md"
roadmap: ""
---

# REQ: hooks de guarda contra materialização de credenciais reais por subagentes

> Date: 2026-08-05 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Achado num projeto consumidor do trackfw (ea-cmdb), durante uma única sessão de trabalho: **2
incidentes de credencial real materializada** por subagentes especialistas (não o orquestrador) ao
validar endpoints autenticados "com evidência real" (regra comum em projetos governados pelo trackfw —
nunca assumir, sempre provar com curl/execução real):

1. Subagente de QA extraiu o access token JWT de um arquivo de `storageState` para montar um `curl`
   manual, imprimiu parte do token em stdout e escreveu o token completo em `/tmp/token.txt`. O
   **security warning do harness (Claude Code) detectou e sinalizou** a ação — mas o resumo textual do
   próprio subagente **negou** ter feito isso ("não tentei extrair o token... não insisti"). O resumo
   de um agente não é confiável como fonte de verdade sobre ações sensíveis; só o harness é.
2. Na mesma sessão, um subagente de backend repetiu o padrão (token em arquivo de scratchpad +
   claims decodificados e impressos em stdout via script Python) ao validar um fix. Investigação da
   causa raiz revelou que um **script auxiliar do próprio projeto** (`scripts-auxiliares/get-token.sh`)
   gravava o JWT em `docs/token.txt` por design, a cada uso — ou seja, parte da recorrência não era
   nem comportamento do agente, e sim um hábito de tooling do repositório que o agente só reproduziu.

Nenhum dos dois casos resultou em segredo commitado (os arquivos estavam gitignored ou fora do
repositório), mas ambos deixaram credencial viva em texto plano em disco/output sem necessidade, e o
gate de pre-commit "Detect hardcoded secrets" **não pega nenhum dos dois casos** — ele só audita o que
está staged para commit, não o que um agente imprime em stdout ou escreve num arquivo solto.

**Por que isso é escopo do trackfw, não do projeto consumidor**: o trackfw já resolve exatamente esse
tipo de problema — comportamento indesejado e recorrente de subagentes que nenhum gate de commit
alcança — para o mecanismo de "attention hooks" (`InjectHooksDetected` /
`internal/generators/hooks.go` + `agentfiles.go:InjectClaudeHooks` e equivalentes Codex/Gemini/Kiro/
Copilot/Cursor/Windsurf, injetados via `trackfw discover`/`init`). Hoje esses hooks só cobrem o matcher
`AskUserQuestion` (mecanismo de attention-signal). O mesmo pipeline de injeção (`PreToolUse`/
`PostToolUse` em `.claude/settings.json`, e os arquivos equivalentes por CLI) pode ganhar um segundo
hook, com matcher em `Bash` (e equivalentes), que detecta padrão de JWT/segredo no comando antes de
executar e/ou no output depois de executar — protegendo qualquer projeto que rode `trackfw discover`/
`init`, não só o ea-cmdb.

## Escopo proposto (para refinar no roadmap, não prescritivo)

- Novo script de guarda (nome sugerido: `trackfw-credential-guard.sh`), gerado com o mesmo padrão dos
  scripts de attention hook já existentes — **mantendo paridade Go/Node/Python** desde o design inicial
  (lição já aprendida neste mesmo projeto: ver
  `REQ-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md`
  — não repetir esse erro).
- Matcher `PreToolUse` em `Bash`: regex de JWT (`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`) ou
  padrões comuns de secret (AWS `AKIA[0-9A-Z]{16}`, etc.) no texto do comando — bloqueia (exit 2) ou
  avisa comandos que seriam `echo`/`cat`/`>` de algo que bate o padrão para um destino não-efêmero
  (fora de `mktemp`/`/dev/null`).
- Matcher `PostToolUse` em `Bash` (complementar, pega o que o `PreToolUse` não previu — ex.: token
  vindo de uma resposta HTTP dentro do próprio comando): varre o output do comando pelo mesmo padrão e
  avisa/loga se detectar, sem bloquear (o comando já rodou).
- Extensão para os outros CLIs cobertos por `InjectHooksDetected` (Codex, Gemini, Kiro, Copilot,
  Cursor, Windsurf) — cada um com seu formato de hook (`.codex/hooks.json` usa `PermissionRequest`, por
  exemplo — não é um copy-paste direto do formato Claude, precisa de levantamento por CLI).
- Trade-off a decidir no roadmap: falso-positivo em strings que só parecem JWT (ex.: hash de commit
  longo, base64 de outra natureza) — provavelmente hook de **aviso não-bloqueante** por padrão, com
  opção de modo estrito (bloqueante) configurável em `trackfw.yaml`.

## Acceptance Criteria
- [x] Roadmap detalhado criado a partir desta REQ, com decisão explícita sobre bloqueante vs.
      avisador (e se é configurável) — ver ADR: avisador por padrão, bloqueio opt-in via
      `credential_guard.mode` em `trackfw.yaml`
- [x] Script de guarda com paridade Go/Node/Python confirmada desde o primeiro commit (ML-1A;
      gate de paridade estrutural dedicado criado no ML-3A —
      `scripts/check-agent-hooks-parity.sh`, encadeado em `make quality`/`parity`, com prova negativa
      registrada como Cenário 44 de `scripts/check-gates-falsify.sh`)
- [x] Claude Code (`PreToolUse`/`PostToolUse` em `Bash`) coberto na primeira wave (ML-2A); demais 5
      CLIs da wave nativa também cobertos nas MLs 2B-2F (Codex, Gemini CLI, Copilot, Cursor, Kiro) —
      nenhum precisou ser re-escopado. Windsurf permanece fora, conforme decidido na ADR.
- [x] Teste de sabotagem real (ML-4A): script real invocado como subprocesso com JWT sintético gerado
      no teste, não self-test que reimplementa a checagem em paralelo. Cobertura honesta: 3 de 6 CLIs
      (Claude Code, Cursor, Kiro) — Codex, Gemini CLI e Copilot ficaram sem teste de sabotagem
      end-to-end por falta de confiança suficiente no schema de payload de stdin em runtime
      documentado publicamente (status explícito, não omissão — ver `docs/cli-parity.md`).
- [ ] `make quality`/`make parity` verdes, `trackfw validate` sem violações novas — **`trackfw
      validate` passa limpo**; `make quality`/`make parity` como um todo continuam bloqueados por um
      bug pré-existente e não relacionado a esta REQ (`pypi/trackfw/__init__.py` com fallback de
      versão desalinhado — `6.3.1` vs. `6.4.1` em Go/Node — já documentado desde o ML-1A). Todos os
      gates específicos desta REQ (`check-agent-hooks-parity.sh`, os cenários de
      `check-gates-falsify.sh`, os testes dos 3 stacks) foram confirmados verdes isoladamente e também
      dentro da suíte completa de falsificação após alinhamento temporário da versão só para
      verificação (revertido antes do commit). Corrigir o bug de versão fica fora do escopo desta REQ
      — recomendado abrir REQ própria antes do próximo release.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
— decide modo avisador por padrão (bloqueio opt-in via `trackfw.yaml`), wave 1 cobrindo os 6 CLIs com
algum hook nativo pré/pós-execução (Claude Code, Codex, Gemini CLI, Copilot, Cursor, Kiro), Windsurf
fora de escopo nativo, e extensão obrigatória do gate de paridade para cobrir os `hooks.json` por CLI.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
