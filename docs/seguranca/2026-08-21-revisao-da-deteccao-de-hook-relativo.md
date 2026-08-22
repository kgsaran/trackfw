# Revisao de Seguranca — Deteccao de Hook de Guard na Forma Relativa Antiga

> Data: 2026-08-21
> Agente: hades-tf
> ML: 3A (Wave 3 — Barreira)
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-21-validate-detecta-hook-de-guard-na-forma-relativa-antiga.md`
> REQ: `docs/req/REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz.md`

---

## VEREDITO: APROVADO

A deteccao nao e contornavel pela forma-alvo da REQ. O falso-positivo foi evitado por construcao,
nao por valor. A tabela dos 6 CLIs e consistente entre geradores e validador. Ha um gap residual em
formas nao geradas pelo trackfw; e nomeado aqui e fora do escopo declarado da REQ.

---

## A. A deteccao e contornavel?

A regra dispara quando: `hf.requiresVarOrShellPrefix == true` E `isRelativePureForGuard(raw) == true`.

`isRelativePureForGuard(raw)` retorna true quando `raw` nao comeca com `$`, nao comeca com `"` e nao
e caminho absoluto.

### Candidatos a bypass medidos

Todos os testes abaixo foram executados com `./bin/trackfw validate` (binario compilado nesta sessao
com `make build`). Fixture: subdiretorio isolado de scratchpad com `scripts/trackfw-credential-guard.sh`
presente e executavel. JSON valido confirmado antes de cada medicao.

| Forma | Captada? | Observacao |
|---|---|---|
| `scripts/trackfw-credential-guard.sh` (forma alvo) | **SIM** (MEDIDO) | Mensagem nomeia o remedio (`trackfw update`) |
| `./scripts/trackfw-credential-guard.sh` (dot-slash) | **SIM** (MEDIDO) | Mesma classe — `isRelativePureForGuard` = true |
| `sh scripts/trackfw-credential-guard.sh` (interprete) | **SIM** (MEDIDO) | String contem o marker; `isRelativePureForGuard` = true |
| `../scripts/trackfw-credential-guard.sh` (dotdot) | **SIM** (MEDIDO) | Mesma classe |
| `$CLAUDE_PROJECT_DIR/scripts/...` (forma correta) | **SILENCIO** (MEDIDO) | Comportamento esperado — nao e violacao |
| `$PWD/scripts/...` | **NAO captada** (MEDIDO) | Descrito em D.1 abaixo |
| `$UNDEFINED/scripts/...` | **NAO captada** (MEDIDO) | Descrito em D.2 abaixo |
| `"scripts/..."` (aspas literais no valor) | **NAO captada** (MEDIDO) | Descrito em D.3 abaixo |

**Conclusao sobre A:** a forma-alvo da REQ (`scripts/...` como relativo puro) e todas as suas
variantes de caminho relativo (dot-slash, dotdot, com interprete prefixo) sao capturadas. Formas que
comecam com `$` ou `"` saem pela ramificacao `ok=false` de `resolveCredentialGuardHookPath` e sao
puladas em silencio — comportamento documentado na funcao ("Nao e funcao desta regra adivinhar wiring
proprio de um usuario fora dos formatos que o trackfw gera").

---

## B. O falso-positivo foi evitado por construcao?

A guarda relevante e a linha:

```go
if hf.requiresVarOrShellPrefix && isRelativePureForGuard(m.raw) {
```

Para Cursor (`requiresVarOrShellPrefix=false`), Copilot (`false`) e Kiro (`false`): a condicao curto-
circuita no primeiro operando, independente do valor de `m.raw`. Isso e eliminacao por construcao, nao
por valor.

### Medicoes diretas (B1-B3)

- **B1 — Cursor com relativo:** `.cursor/hooks.json` com `"command":"scripts/trackfw-credential-guard.sh"` → **SILENCIO** (MEDIDO)
- **B2 — Copilot com relativo:** `.github/hooks/trackfw-attention.json` com `"bash":"scripts/trackfw-credential-guard.sh"` → **SILENCIO** (MEDIDO)
- **B3 — Kiro com relativo:** `.kiro/hooks/trackfw-attention.json` com `action.command:"scripts/trackfw-credential-guard.sh"` → **SILENCIO** (MEDIDO)

### A guarda e load-bearing?

Sim, por evidencia do ML-2A (Cenario 160): ao remover `hf.requiresVarOrShellPrefix &&` da condicao
e substituir por `isRelativePureForGuard(m.raw)` diretamente, o gate `check-validate-parity.sh`
detectou a acusacao indevida do Cursor. A remocao da guarda e capturada antes de chegar ao
produto. Isso confirma que a guarda nao e apenas defensiva — e o que manteria o falso-positivo fora.

### Haveria outra regra que acusasse Cursor/Copilot/Kiro pelo relativo?

Nao identificada. A outra condicao relevante e `requiresCommandType && !m.typeIsCommand`. Para
Cursor, `requiresCommandType=false` — pulada. Para Copilot e Kiro, `requiresCommandType=true`, mas
a forma correta dos geradores ja inclui `"type":"command"` como campo irmao, entao
`typeIsCommand=true` e a condicao nao dispara. Nenhum caminho alternativo acusa esses tres CLIs pelo
relativo.

---

## C. A tabela dos 6 CLIs esta certa?

A tabela em `credentialGuardHookFiles` (Go) foi verificada contra as constantes dos 3 geradores:

| CLI | Forma gerada (Go / Node / Python) | requiresVarOrShellPrefix |
|---|---|---|
| Claude Code | `$CLAUDE_PROJECT_DIR/scripts/...` (todos os 3) | `true` — consistente |
| Codex CLI | `"$(git rev-parse --show-toplevel)/scripts/..."` (todos os 3) | `true` — consistente |
| Gemini CLI | `$GEMINI_PROJECT_DIR/scripts/...` (todos os 3) | `true` — consistente |
| Cursor | `scripts/...` (todos os 3) | `false` — consistente |
| GitHub Copilot CLI | `scripts/...` (todos os 3) | `false` — consistente |
| Kiro | `scripts/...` (todos os 3) | `false` — consistente |

Verificacao por constante de gerador (Go: `agentfiles.go` linhas 253-275, 355, 367, 762-800; Node:
`hooks.js` linhas 1173-1178; Python: `hooks.py` linhas 445, 576, 587, 837-875, 944, 1095-1125).

A paridade de saida em runtime foi fechada pelo ML-2A: `check-validate-parity.sh` — 8 casos CG + 2
casos GBG, byte-identicos nos 3 CLIs — com Cenarios 159/160 provando as duas direcoes (acusar de
menos e acusar de mais). Nao ha divergencia entre a tabela e o que os geradores emitem.

**Kiro — o caso "indeterminado":** a escolha de `false` nao e comprovadamente segura — e
conservadora. Se o Kiro invocar hooks a partir de subdiretorios (como o Claude), o guard com
relativo falha em runtime e o `validate` nao acusa. Mas o oposto (accusar) quebraria o validate de
usuarios Kiro que estao usando a forma que o gerador emite hoje. Pelo principio do `ADR-2026-08-17`
("guard que atrapalha e guard que o usuario desliga"), a escolha de `false` e a correta dada a
indeterminancia. O risco residual e documentado como gap nomeado em D.4.

---

## D. Achados novos

### D.1 — Gap: `$PWD/scripts/...` nao e captado (NOMEADO, FORA DE ESCOPO)

**Medido:** `"command":"$PWD/scripts/trackfw-credential-guard.sh"` em `.claude/settings.json` →
`validate` silencioso. `resolveCredentialGuardHookPath` retorna `ok=false` (comeca com `$` mas nao
casa com `$CLAUDE_PROJECT_DIR/` nem `$GEMINI_PROJECT_DIR/` nem o prefixo Codex), entao a entrada e
pulada.

**Impacto em runtime:** se um usuario substitui manualmente `scripts/...` por `$PWD/scripts/...`
acreditando que `$PWD` ancora a raiz, o hook pode falhar em subdiretorios (o `$PWD` em contexto de
hook reflete o cwd corrente do agente, nao a raiz do projeto).

**Por que nao e um blocker aqui:** (a) `$PWD/...` nao e gerado pelo trackfw em nenhum ponto —
nenhuma migracao o produz; (b) a REQ e o escopo declarado da regra e a forma-alvo `scripts/...`; (c)
`resolveCredentialGuardHookPath` documenta explicitamente que nao tenta cobrir formas fora do que o
trackfw gera. O gap e consistente com o desenho.

**Recomendacao:** registrar como debito tecnico nomeado para uma REQ futura, se o caso aparecer em
campo.

### D.2 — Gap: variavel indefinida `$UNDEFINED/scripts/...` nao e captada (NOMEADO, FORA DE ESCOPO)

Mesmo mecanismo do D.1: `ok=false` por nao casar nenhum prefixo conhecido. Fora de escopo da REQ
por razao identica. Em runtime expande para `/scripts/...` (absoluto, nao existe). Debito tecnico,
nao blocker.

### D.3 — Gap: aspas literais no valor JSON nao sao captadas (INFORMACAO)

`"scripts/..."` (com aspas literais como parte do valor apos parse JSON) inicia com `"`, entao
`isRelativePureForGuard` retorna false e `resolveCredentialGuardHookPath` retorna `ok=false`. Forma
nao produzida pelo trackfw, improvavel em campo. Registrado por completude.

### D.4 — Risco residual Kiro: cwd indeterminado (NOMEADO, JA DOCUMENTADO NO ROADMAP)

Se o Kiro invocar hooks a partir de subdiretorios, o guard com `scripts/...` falha silenciosamente e
o `validate` nao avisa (porque `requiresVarOrShellPrefix=false`). O roadmap e o ML-1A ja registram
esse indeterminismo. A escolha e conservadora (nao acusar) e defensavel dado o ADR-2026-08-17, mas o
risco persiste ate que o comportamento do Kiro seja documentado.

---

## Postura ADR

- `ADR-2026-08-12`: sem prevencion contra agente induzido; deteccao ancorada. A regra nao muda
  o que e considerado correto para cada CLI — apenas detecta o desvio.
- `ADR-2026-08-17`: falso-positivo e risco de primeira ordem. A eliminacao por construcao
  (`requiresVarOrShellPrefix=false` para Cursor/Copilot/Kiro) e o mecanismo correto.

Nao ha recomendacao de mudar qual forma cada CLI usa. A regra detecta, nao redefine.
