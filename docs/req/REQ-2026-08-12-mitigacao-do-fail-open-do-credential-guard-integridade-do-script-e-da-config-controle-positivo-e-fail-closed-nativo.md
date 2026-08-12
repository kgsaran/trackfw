---
status: Open
date: 2026-08-12
author: "Zeus (Arquiteto)"
adr: ""
roadmap: ""
---

# REQ: Mitigacao do fail-open do credential-guard — integridade do script e da config, controle positivo e fail-closed nativo

> Date: 2026-08-12 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

O `ROADMAP-2026-08-12` **mediu** o que ninguém sabia: quando o `command` de um hook não resolve, a
ferramenta **prossegue**. O `scripts/trackfw-credential-guard.sh` é um **controle de negação** — se
não roda, nada bloqueia, e o agente não trata isso como erro fatal.

| CLI | Caso A (não resolve) | Caso B (`exit != 0`) | Como se soube |
|---|---|---|---|
| Claude Code | **FAIL-OPEN** | `exit 1` open · `exit 2` closed | doc primária |
| Codex CLI | **FAIL-OPEN** | `exit 1` open · `exit 2` closed | **empírico** |
| Cursor | **FAIL-OPEN** (padrão) | open salvo `exit 2`; opt-in `failClosed: true` | doc primária |
| Gemini | INDETERMINADO | fora de `{0,2}` é open | doc primária |
| Copilot | fail-closed | fail-closed | doc primária |
| Kiro | INDETERMINADO | depende da superfície (IDE × CLI) | doc primária |

Evidência: `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md`,
`docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md`.
Análise: `docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md` — **usar a seção "Revisão ML-2B"**,
a original está `[SUPERSEDIDA]`.

### O que sustenta a severidade (medido ou verificado no código)

- **Remoção do script.** Gerado com `os.WriteFile(..., 0755)`
  (`internal/generators/scaffold.go:797`), **sem verificação de integridade** em `validate`/`doctor`
  (que não existe hoje). Um agente induzido apaga o guard — o guard não bloqueia `rm` — e a partir
  daí toda chamada cai no Caso A, **já medido como fail-open**.
- **Sobrescrita é pior que remoção.** Substituir o conteúdo por `exit 0` produz
  `hook: PreToolUse Completed` — **o mesmo rótulo de sucesso normal**. Zero sinal observável, ao
  contrário do `Failed` que a remoção gera.
- **Downgrade por config.** `credential_guard.mode` é lido em **runtime** de `trackfw.yaml`
  (`scaffold.go:1005`). Uma linha de YAML derruba o controle **sem tocar no script** — derrotando
  qualquer checksum que cubra apenas o script.

### Hipóteses NÃO medidas (não usar para justificar decisão sem verificar)

- A cadeia do `rm` **ponta a ponta** num experimento único no Codex real.
- Redirecionamento de `.git` via gitfile + `core.worktree`.
- Sessão interativa com humano no loop (todas as medições foram em `codex exec` não-interativo).
- **Refutado e registrado:** o vetor "agente faz `cd` e o cwd do hook acompanha" **não se
  reproduz** — o cwd do hook do Codex é fixo na sessão (ML-1C). Não reabrir sem nova medição.

### Observação de arquitetura que a mitigação deve considerar

O credential-guard de **escopo global** (`trackfw update harness`) vive em `~/.trackfw/`, **fora do
repositório** em que o agente trabalha. Um agente restrito ao workspace **não alcança** esse arquivo.
Isso já é uma mitigação parcial existente e deve entrar na análise de custo/benefício — talvez
parte da resposta seja **preferir o escopo global** em vez de construir integridade no escopo de
projeto.

## Acceptance Criteria

Ordem de custo/benefício recomendada pelo parecer (Revisão ML-2B §5). Cada item é independente e
pode virar wave própria.

- [ ] **1. Controle positivo em `trackfw validate`/`doctor`** — verificar que os hooks registrados
      apontam para um script que **existe e é executável**. Custo zero, sem risco de *bricking*,
      cobre a classe do incidente original (caminho que não resolve). **Limitação a registrar:** só
      verifica no momento em que o comando roda, não no momento da invocação do hook.
- [ ] **2. `failClosed: true` nas entradas do guard no Cursor** — opt-in nativo do fornecedor,
      config-only, custo zero. **Escopo restrito ao guard**, nunca aos hooks de attention.
- [ ] **3. Wrapper que converte "não consegui rodar" em bloqueio** — ex.:
      `sh -c 'test -x <script> && exec <script> || exit 2'`. Deixou de ser condicional: é o único
      item que cobre a via de **deleção** no momento da invocação.
      🔴 **Bloqueador conhecido — resolver antes de implementar:** o script é **gerado** por
      `trackfw init`/`update harness`, **não faz parte do binário**. Um clone fresco com os hooks já
      commitados, **antes** do `init`, teria **toda chamada de ferramenta travada**. Precisa de
      resposta explícita para "projeto legitimamente sem o script".
- [ ] **4. Verificação de integridade de conteúdo** — do **script** *e* da config
      `credential_guard.mode`, com valor de referência guardado **fora do arquivo gerado**. É o único
      item que cobre **sobrescrita** e **downgrade por config**, que os itens 1–3 não cobrem.
- [ ] Se qualquer item tocar geradores: **paridade obrigatória nos 3 CLIs** (Go, Node.js, Python) —
      regra dura do projeto.
- [ ] Teste de comportamento para **ausência legítima do script** (projeto antes do `init`) em
      qualquer item que possa travar chamadas.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** altera o wiring de caminho dos hooks (encerrado no `ROADMAP-2026-08-11`).
- **Não** reabre a determinação da semântica (encerrada no `ROADMAP-2026-08-12`).
- **Não** implementa os 4 itens de uma vez sem avaliar custo/benefício de cada um — "documentar e
  aceitar o risco" continua sendo resposta válida para itens individuais, desde que **escrita**.

## Linked ADR
<!-- provável: a escolha entre integridade no escopo de projeto × preferir escopo global é decisão de arquitetura -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
