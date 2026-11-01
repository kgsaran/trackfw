# Red Team — Wave 1: Memória de Agente em Subpasta

> Roadmap: ROADMAP-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
> REQ: REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
> Issue: #646 ML-2A
> Analista: hades-tf | 2026-11-01

---

## VEREDITO: REPROVADO

Três achados (A-1, A-2, A-3) introduzidos neste PR. Nenhum é pré-existente. A-2a e A-3 são bloqueantes: o usuário pode deletar dados por seguir um remédio emitido pelo `doctor`. A-1 é bloqueante por violar o contrato documentado da própria função. Todos os três pertencem a esta REQ (mesma causa — `scanStrayAgentMemory` e `hasGitIgnoreAgentMemoryRule` são código novo) e exigem novo ML na Wave 1 antes do merge.

---

## Prescrições P-1 a P-7 — Veredito

| Prescrição | Veredito | Evidência |
|---|---|---|
| **P-1** (AC1: `## Memory location` só para `targetID == "claude"`) | ✅ CUMPRIDA | `TestZZLeakToAllTargets`: 11 targets não-claude (kiro/ide, kiro/cli, amazonq, antigravity, gemini, cursor, copilot, windsurf, codex, opencode) — 0 ocorrências de `## Memory location`. Goldens: `architect.agent-directory.golden.md` = 0; `backend.codex-toml.golden.toml` = 0. |
| **P-2** (AC1 leitura: instrução de Read tool) | ✅ CUMPRIDA | `claudeMemoryRootAnchor` inclui "read `<repo root>/.claude/agent-memory/<your name>/MEMORY.md` explicitly at the start of the session using the Read tool." — presente no golden `architect.subagent.golden.md` e verificado por `TestClaudeMemoryLocationSectionPresence`. |
| **P-3** (AC2: `## Dispatch cwd` no body de `architect.md`) | ✅ PARCIAL | Presente em `architect.md` e flui para todos os 10 targets medidos. A Wave-0 aceitou que o texto é comportamental e "não menciona `memory:`". O texto real diz "the harness resolves and creates the subagent's memory directory" — isso é falso para codex, opencode e gemini, que não têm harness de memória. Não causa dano técnico mas afirma falso para esses destinos. |
| **P-4** (AC3: `hasGitIgnoreAgentMemoryRule` por linha exata; CRLF; sem `\n` final) | ✅ PARCIAL | Exatidão: correto. CRLF: `TrimSpace` remove `\r`; e `git check-ignore` também honra padrão CRLF (medido: `NOT IGNORED → IGNORED` com `printf '*/**/.claude/agent-memory/\r\n'`). Sem `\n` final: guard linha 2374 previne grudar. **Falha no tratamento de `!`**: veja A-1. A Wave-0 prescreveu "pular se começa com `!`"; o executor implementou exatamente isso; `continue` ≠ "reconhecer como opt-out". |
| **P-5** (AC4: raiz por `git rev-parse`; skip `.git`/`node_modules`; nested repo) | ✅ CUMPRIDA | `scripts/check-doctor-parity.sh` inexistente confirmado. Kind `stray-agent-memory` em `printDoctorReport`. `TestZZDoctorFromSubdir` PASS (stray em `pypi/` encontrado ao varrer de `scripts/`). 6000 dirs em 116ms; 1000 vendor+.venv (não skipados) em 23ms. |
| **P-6** (AC5: `.gitignore` trackfw; três dirs removidos) | ✅ CUMPRIDA | `find . -path ./node_modules -prune -o -type d -name agent-memory -print` → `./.claude/agent-memory` apenas. `git check-ignore`: root NÃO ignorada; `pypi/.claude/agent-memory/test` IGNORADO. |
| **P-7** (AC3: 4 vetores com `git check-ignore` + idempotência) | ✅ PARCIAL | `TestGenerateGitIgnore_AgentMemoryFourVectorsWithGitCheckIgnore` cobre os 4 vetores com `git check-ignore` apenas para o caso fresh-file. Casos `WrongPattern`, `CommentedPattern` e `CorrectPatternIsIdempotent` usam apenas string-count, sem `git check-ignore`. O caso `!`-negation não tem teste (veja A-1). |

---

## Achados (novos — código introduzido neste PR)

### A-1 — MEDIUM: `hasGitIgnoreAgentMemoryRule` skip de `!`-linha ≠ bloqueio de append; opt-out silenciosamente sobrescrito

**Mecanismo:** `hasGitIgnoreAgentMemoryRule` faz `continue` em linhas `!` (não as conta como presentes). `generateGitIgnore` recebe `false` e acrescenta `*/**/.claude/agent-memory/` **após** a negação. Gitignore é last-match-wins: o novo padrão vence o opt-out.

**Evidência (live):**
```
# .gitignore antes: !lib/.claude/agent-memory/
git check-ignore -v lib/.claude/agent-memory/zeus/test.md → NOT IGNORED

# após generateGitIgnore:
git check-ignore -v lib/.claude/agent-memory/zeus/test.md
  .gitignore:3:*/**/.claude/agent-memory/  lib/.claude/agent-memory/zeus/test.md  (IGNORED)
```

**Impacto:** projeto com memória versionada em `lib/.claude/` passa silenciosamente a ignorar novos arquivos ali após `trackfw init`/`update`. Arquivos já rastreados ficam imunes (`.gitignore` não afeta tracked files); novos arquivos caem do `git add` sem aviso.

**Origem na Wave-0:** P-4 prescreveu "pular se começa com `!`"; o executor implementou literalmente. A prescrição era ambígua; `continue` ≠ "tratar como presente (opt-out reconhecido)."

**Nota de vault:** `vault/notes/gitignore-negation-override-by-append-2026-11-01.md`

---

### A-2 — MEDIUM: remédio do `doctor` embute caminho sem aspas e sem indicação da base

**Sub-vetor 2a — espaço no caminho (reproduzido com binário real):**

Stray em `my dir/.claude/agent-memory/`, doctor real (`trackfw-new`) emite:
```
[stray-agent-memory] my dir/.claude/agent-memory
  remedy: ... remove with: rm -rf my dir/.claude/agent-memory
```
Copiando o comando: shell interpreta dois tokens (`my` e `dir/.claude/agent-memory`). Deleta o diretório `my` (que pode existir independentemente do stray) e tenta `dir/.claude/agent-memory`.

**Sub-vetor 2b — caminho relativo à raiz, não ao cwd do usuário (medido):**

Doctor invocado de `scripts/` emite o mesmo `rm -rf my dir/.claude/agent-memory` (relativo à raiz git). Se o usuário executa de `scripts/`, o shell resolve `scripts/my dir/.claude/agent-memory` — inexistente, `rm -rf` sai 0, stray permanece sem aviso.

**Sub-vetor 2c — metacaracteres (analítico):** diretório com `$()` no nome geraria `rm -rf a$(touch PWNED)/...`; pasted em bash executa o subshell.

**Fix:** colocar o caminho entre aspas simples e anotar que é relativo ao git root.

---

### A-3 — MEDIUM: `chmod 000` em subdir do stray → misreportado como "empty (harness residue)" → rm -rf de memória perdida

**Mecanismo:** o `WalkDir` interno que conta arquivos faz `return nil` em `innerErr`. Subdir `chmod 000` não é contado; `fileCount = 0`; remédio: "empty directory (harness residue); remove with: rm -rf".

**Evidência (`TestZZChmod000InsideStray` FAIL):**
```
stray: sub/.claude/agent-memory/zeus-tf/hidden/ (chmod 000, contém secret.md)
remedy: "empty directory (harness residue from dispatch in a subdirectory cwd); remove with: rm -rf sub/.claude/agent-memory"
```
O usuário é instruído a apagar memória perdida que deveria ser arquivada.

**Fix:** ao encontrar `innerErr != nil` numa entrada de diretório, tratar como "potencialmente contém arquivos" e emitir `"may contain files (permission denied reading subdirectory)"` em vez de assumir zero.

---

### A-4 — LOW (informacional): `## Dispatch cwd` afirma falso para codex/opencode/gemini

Medido: `memoryDir=true` em todos os 10 targets. O texto "the harness resolves and creates the subagent's memory directory" é verdadeiro apenas para Claude Code. Codex, opencode e gemini não têm harness de memória. Não causa dano funcional; a instrução é benignamente ignorada. Sinaliza para o arquiteto considerar qualificar o texto na próxima edição do asset.

---

## Ataques sem achado (passaram)

| Vetor | Resultado |
|---|---|
| Sufixo + signature rewrite com identity | PASS: sig reescrita em byte 8795, sufixo em byte 8837, 1 ocorrência |
| Idempotência de `agents update` (install + 2× update) | PASS: SHA `853c55d0...` estável após 2 updates |
| Upgrade: install v11.2.3, update com binary novo | PASS: 0 hand-modified; `## Memory location` aparece após update; byte-stable |
| Validator: nenhuma regra compara agente instalado a fresh render | CONFIRMADO: `Render()` só chamado com `catalog.ReadAsset()`, nunca com arquivo instalado |
| Doctor reports hand-modified após update | NÃO: 0 hand-modified |
| CRLF no `.gitignore` | PASS: `git check-ignore` honra padrão com `\r`; `TrimSpace` correto |
| Sem `\n` final no `.gitignore` | PASS: guard linha 2374 insere `\n` antes do bloco |
| Preexisting `**/.claude/agent-memory/` | PASS: não confundido com regra correta; dois padrões coexistem (residual documentado) |
| Doctor de subpasta | PASS: `git rev-parse --show-toplevel` resolve raiz; stray fora de scripts/ encontrado |
| Symlink loop | PASS: `WalkDir` usa Lstat; sem hang, 0 findings |
| Real git worktree (caso a): scan de WT encontra stray em sub/ | PASS: 1 finding |
| Real git worktree (caso b): scan de main ignora worktree aninhado | PASS: 0 findings |
| Custo 6000 dirs regulares | PASS: 116ms |
| Custo 1000 vendor+.venv (não skipados) | PASS: 23ms |
| Leak de `## Memory location` para 11 targets não-claude | PASS: 0 ocorrências |
| P-5: `scripts/check-doctor-parity.sh` inexistente | CONFIRMADO |
| AC5: único `agent-memory/` é o da raiz | CONFIRMADO |
| Doctor no repo real (de scripts/) | PASS: 1 finding scaffold-workflow-duplicated (pre-existente), 0 stray-agent-memory |

---

## Residual Declarado (o que o design aceita não cobrir)

1. **Lado de leitura do harness (AC1):** incontornável; emenda P-2 mitiga mas não elimina.
2. **Instrução AC1 não verificável em execução:** auditável apenas pela presença no golden.
3. **Nested repo com `.git`-arquivo + gitdir válido (real worktree):** medido e correto — `git rev-parse` funciona; os dois casos (a) e (b) passam.
4. **Migração de padrão errado `**/` pré-existente:** dois padrões coexistem; comportamento final correto; correção manual necessária.
5. **Windows junctions (`ModeIrregular`):** guard implementado (`ModeSymlink|ModeIrregular`), não testado nesta rodada por falta de VM Windows.
6. **Barrier nested path:** `.claude/agent-memory/zeus-tf/.claude/agent-memory/...` aceito como in-scope. Pré-existente.

---

## Arquivos escritos nesta rodada

- `docs/seguranca/2026-11-01-redteam-memoria-de-agente-em-subpasta.md` (este arquivo)
- `vault/notes/gitignore-negation-override-by-append-2026-11-01.md`
- `vault/notes/index.md` (linha adicionada)
