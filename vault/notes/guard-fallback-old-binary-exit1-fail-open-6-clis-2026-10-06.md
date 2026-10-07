# guard — binario antigo (sem `guard`) sai exit 1: fail-open em 6 de 8 CLIs

**Data:** 2026-10-06
**Contexto:** ML-3A corretivo, REQ-2026-09-05, branch `feat/hooks-de-guard-executam-no-windows`

## Achado

Quando o PATH resolve para um binario `trackfw` antigo (ex.: 8.0.0-rc2, sem o comando `guard`),
a linha de hook retorna **exit 1** — nao exit 0.

```
trackfw guard git-branch
  → STDERR: Error: unknown command "guard" for "trackfw"
  → EXIT: 1
```

Medido na VM Windows 11 ARM64: PS5, cmd, Git Bash — todos exit 1.

## Comportamento por CLI (exit 1 do binario antigo)

| CLI | Exit que bloqueia | Resultado de exit 1 | Classificacao |
|-----|-------------------|---------------------|---------------|
| Claude Code | exit 2 apenas | **fail-open** (exit ≠ 2 = permite) | ❌ inseguro |
| Codex | exit 2 apenas | **fail-open** | ❌ inseguro |
| Gemini | exit 2 apenas | **fail-open** | ❌ inseguro |
| Cursor | exit 2 apenas | **fail-open** | ❌ inseguro |
| Windsurf | exit 2 apenas | **fail-open** | ❌ inseguro |
| Amazon Q | exit 2 apenas | **fail-open** | ❌ inseguro |
| Kiro | QUALQUER exit ≠ 0 | bloqueia (FP: deny-all) | ⚠️ FP operacional |
| Copilot | QUALQUER exit ≠ 0 | bloqueia (FP: deny-all) | ⚠️ FP operacional |

Fonte: `docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`, tabela de exit codes.

**6 de 8 CLIs sao fail-open com o binario antigo.** O risco D5 da ADR-2026-10-04 esta
confirmado como risco real para a maioria dos CLIs de agente.

Kiro e Copilot bloqueiam, mas pela razao errada (erro de cobra "unknown command", nao decisao
do guard). O resultado e FP operacional: bloqueia ate o binario ser atualizado — deny-all.

## Por que exit 1 != fail-closed

**A armadilha:** exit ≠ 0 parece "falha fechada" por definicao, mas "fail-closed" neste contexto
significa que a ferramenta e BLOQUEADA. Para 6 dos 8 CLIs, o criterio de bloqueio e exit 2
especificamente. Exit 1 e tratado como "hook falhou por erro, mas segue" — fail-open.

**Regra geral:** "exit ≠ 0 nao e bloqueio; o que bloqueia depende do CLI".

## Contraste com o caso genuinamente fail-open de outro mecanismo

No caso Restricted + shim `.ps1` + sufixo `; exit $LASTEXITCODE`:
- A excecao `PSSecurityException` NAO atualiza `$LASTEXITCODE` (permanece 0 do estado anterior)
- A instrucao `exit $LASTEXITCODE` propaga exit 0 → fail-open verdadeiro
- Ver [[restricted-exit-lastexitcode-failopen]]

O caso do binario antigo e diferente: o processo externo termina normalmente com exit 1,
o shell propaga exit 1. Para os 6 CLIs fail-open (exit 2 = criterio), exit 1 = permite.

## Residual: sonda do `validate` nao ve o binario do Git Bash login

O `trackfw validate` rule `hook_guard_binary_version` usa Go `exec.LookPath` (PATH do Windows).
No Windows PATH desta VM, `C:\Users\Lab\bin` NAO esta listado.
O Git Bash (login, `-l`) prepende `/c/Users/Lab/bin` ao PATH via seu perfil — esse diretorio
contem o 8.0.0-rc2. O `exec.LookPath` do `validate` ve 9.2.0 (pip, de
`C:\...\Python312-arm64\Scripts\`), nao o binario antigo.

**Consequencia:** usuario que instala via GitHub release em `~/bin` e usa Git Bash (login)
pode ter hooks quebrados sem que `validate` os detecte. O `validate` ve o canal pip/npm (9.2.0)
e reporta "OK", mas o hook na pratica resolve o binario de `~/bin`.

## Mitigacao

O `trackfw validate` rule `hook_guard_binary_version` (introduzida no ML-2B) sonda `trackfw guard --help`
para detectar se o binario no PATH suporta `guard`. Para o caso do Git Bash login, a sonda pode
nao ver o mesmo binario que o hook usa — residual registrado.

## Links

- Tabela de exit codes por CLI: `docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`
- Secao de medicao: `docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md` (ML-3A, Caso 6)
- Nota relacionada: [[restricted-exit-lastexitcode-failopen]] (o caso genuinamente fail-open)
