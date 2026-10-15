---
status: Proposed
date: 2026-10-14
author: "zeus-tf"
---

# ADR: no Windows, o guard global de cada CLI de agente é gravado e auditado no home que aquele CLI lê

> Date: 2026-10-14 | Status: Proposed

REQ: `docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md`
Medição: `docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md`

## Context

**Medido em 2026-10-14** na VM Windows ARM64, Claude Code 2.1.299 nativo, com `HOME` ≠
`USERPROFILE`, nos três terminais (Git Bash, PowerShell, cmd):

- `trackfw update harness` grava o guard em `HOME\.claude\settings.json`, porque
  `internal/homedir/homedir.go:31` lê `HOME` primeiro. O `validate` audita o mesmo caminho
  (`validator_git_branch_guard.go:162`) e **não acusa nada**.
- O Claude Code lê `C:\Users\<user>\.claude\settings.json`, o `USERPROFILE`. Evidência:
  `claude --debug-file` nunca referencia o `HOME`.
- O guard fica **inerte e reportado como saudável**: um falso positivo de saúde num controle de
  negação.
- O próprio código já usa as duas origens: `validator_credential_guard.go:1175` (avisos de matcher
  global) usa `os.UserHomeDir()`, isto é, `USERPROFILE`.

A REQ original declarava como escopo negativo "não altera o port do `homedir`" e pedia só um aviso.
Com a medição, aviso sozinho deixa o defeito vivo. **Decisão do usuário (2026-10-14): seguir o home
do agente e avisar.**

## Decision

**D1 — O home de cada CLI de agente é um atributo do alvo, não do trackfw.** Cada alvo global do
`update harness` (`harness*TargetClaude/Codex/Gemini/Cursor/Copilot/Windsurf/Kiro`,
`internal/generators/update.go:630-678`) e cada regra do `validate` que lê fiação global resolve o
home **do jeito que aquele CLI resolve**. O `homedir.Dir()` continua valendo para os artefatos do
próprio trackfw (`~/.trackfw/`). O port não é revertido; o que muda é **quem** usa cada função.

**D2 — Resolução por CLI, medida e não presumida.** O mapa CLI → mecanismo de home no Windows sai do
Wave 0, com evidência por CLI:
- Claude Code: `USERPROFILE` (medido);
- os demais: Codex (Rust), Gemini (Node), Cursor, Copilot, Windsurf, Kiro.

CLI sem medição fica **declarado** como não medido, e o comportamento atual (`HOME` primeiro) não
muda para ele. Fora do Windows nada muda: lá `HOME` e o home do sistema operacional coincidem.

**D3 — Escritor e auditor usam o mesmo resolvedor por alvo.** É uma função única por CLI, chamada
pelo `update harness` e pelo `validate`. Não pode haver dois resolvedores que divirjam, que é a causa
deste defeito.

**D4 — Aviso de divergência (AC3/AC4 da REQ).** Quando `HOME` e o home do sistema operacional
divergem no Windows, o `validate` emite um aviso **informativo, não bloqueante**, nomeando os dois
caminhos e dizendo qual cada CLI lê. Com os dois iguais, silêncio.

Reconciliação exigida pela auditoria externa de 2026-09-05 (REQ, item 11): o predicado que governa a regra é **o home que o CLI lê** (D1–D3); a divergência de D4 é diagnóstico auxiliar e **não** conta como correção.

**D5 — Migração.** Guard já gravado sob o `HOME` divergente é um resto inerte. O `update harness`
grava no lugar certo e **relata** o resto, mas **não o apaga**, porque é arquivo de configuração do
usuário. O aviso de D4 nomeia esse resto.

### D6 — Ajustes do Wave 0 (`docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md`, aprova com ressalvas)

- **A1 — `CLAUDE_CONFIG_DIR` entra (decisão do arquiteto):** é a mesma causa — o trackfw grava onde o
  agente não lê. Com a variável definida, o Claude Code lê `settings.json` **na raiz** dela (medido),
  em qualquer SO. O resolvedor do Claude honra a variável; **amplia o escopo para além do Windows**
  só neste ponto.
- **A2 — Pacote único `internal/agentconfig`** com o conjunto fechado de 8 sítios de fiação de CLI
  (`update.go:575`; `validator_git_branch_guard.go:162,356,399,450`; `validator_credential_guard.go:1175`;
  `agentfiles.go:2089,2482`; `init.go:407`; `integrations_flags.go:440` — o Wave 0 confere cada um).
  Os 11 sítios de artefato do trackfw ficam em `homedir.Dir()`.
- **A3 — Predicado de D4:** `strings.EqualFold(filepath.Clean(HOME), filepath.Clean(os.UserHomeDir()))`
  — o MSYS2 já converte `HOME` POSIX para a forma Windows (medido), e o caso padrão do Git Bash não
  avisa.
- **A4** — o aviso separa CLI medido de inferido. **A5** — resolvedor injetável nos testes e
  `USERPROFILE` definido em `scripts/check-update-parity.sh`. **A6** — comentário no código com CLIs
  medidos × inferidos, data e referência à medição.
- **A7 — Auditor com erro de resolução (ex.: `USERPROFILE` ausente) emite finding**, nunca `nil`:
  silêncio aqui é o defeito desta REQ.
- **A8 — Windsurf fica como está e é resíduo declarado (decisão do arquiteto):** hoje grava sob `HOME`
  (`update.go:1531`) e audita sob `USERPROFILE` (`validator_credential_guard.go:821`). Sem medição do
  que o Windsurf lê, unificar para qualquer lado pode **criar** falso positivo de saúde. O aviso de D4
  o nomeia como "não medido — grava e audita em homes diferentes". Medir o Windsurf é pré-requisito
  para tirá-lo do resíduo.
- **CLIs inferidos** (Codex, Gemini, Cursor, Copilot, Kiro): comportamento inalterado (D2) até medição.

### D7 — Resíduos do red-team (`docs/seguranca/2026-10-14-red-team-guard-global-home-por-cli.md`)

- **R-2:** symlink acima de `parent(CLAUDE_CONFIG_DIR)` não é verificado — extensão do T5 já declarado do `pathguard`.
- **R-5:** `CLAUDE_CONFIG_DIR` diferente entre o processo do trackfw e o do Claude Code não é detectável pelo trackfw; mitigado pela mensagem condicional do ML-2D.

## Consequences

- O guard global passa a proteger de fato, e o `validate` passa a auditar o arquivo que o agente lê.
- Teste que fixa `HOME` para isolar o home (`homedir.go:6`) precisa isolar também o home do sistema
  operacional (`USERPROFILE`) para os alvos de CLI. O Wave 0 levanta quantos testes isso afeta.

## Alternatives Considered

- **Só avisar (REQ original):** o defeito continua vivo. Rejeitada pelo usuário com a medição na mão.
- **Gravar nos dois lugares:** espalha configuração global e não diz qual vale. Rejeitada.
- **Trocar `homedir.Dir()` para `USERPROFILE`-first em tudo:** quebra o caso que motivou o port
  (artefatos do trackfw sob Git Bash) e decide por CLIs não medidos. Rejeitada.
