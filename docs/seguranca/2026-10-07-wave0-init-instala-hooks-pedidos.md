# Wave 0 — Threat Model: trackfw init instala hooks pedidos

> REQ: docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md
> Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md
> Branch: fix/trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro
> Binary commit: 398e75de — diff vs ae629bfd (main): empty em internal/, cmd/, go.mod, go.sum (apenas docs no commit). Stamp válido.
> Measurements: session scratchpad hades0710/, HOME isolado por dir (fakehome/)
> Date: 2026-10-07 | Author: hades-tf

---

## 1. Completude da Enumeracao

### Superficies enumeradas no roadmap

| Site | Arquivo | Chamado de | Tem o bug de ordenacao? |
|---|---|---|---|
| S1 | `internal/generators/scaffold.go:198` | `Scaffold()` — caminho do `trackfw init` | **SIM** |
| S2 | `internal/generators/update.go:69` | `Update()` — `trackfw update` simples | NAO (sinal pre-existe) |
| S3 | `internal/generators/update.go:2428` | `UpdateProject()`, `case "agent-hooks"` | NAO (sinal pre-existe) |
| S4 | `internal/commands/discover.go:161` | handler de `trackfw discover --init` | NAO |

### Superficie ausente do roadmap — double-call benigna

```
internal/discover/discover.go:88   (InstallGates -> InjectHooksDetected(rootDir))
```

No caminho `--init`, `commands/discover.go` chama `discover.InstallGates` (~linha 151), que por sua vez chama `InjectHooksDetected` em `discover.go:88`. Depois, `commands/discover.go:161` chama `InjectHooksDetected` diretamente — double-call. O efeito e nulo porque cada `InjectXxxHooks` tem dedup interno.

Medicao (discover --init, GEMINI.md pre-existente):
```
$ cd $SCRATCHPAD/m-discover && git init -q && touch GEMINI.md
$ $BIN discover --init 2>&1 | tail -2
  all gates installed
$ grep -o 'trackfw guard' .gemini/settings.json | wc -l
       7
```

Guards = 7, nao 14. Double-call e idempotente. Sem correcao necessaria.

`InjectRulesDetected` (discover.go:156) nao cria novos sinais de deteccao — apenas escreve DENTRO de arquivos ja detectados. Portanto discover nao tem o bug de ordenacao: opera em projetos brownfield onde os sinais pre-existem.

### Superficie com a MESMA CAUSA fora do roadmap (S5 — Regra Dura de Causa Raiz)

```
internal/commands/integrations_flags.go:254   (manager.Install)
internal/commands/integrations_flags.go:278   (InjectRulesForTool por target)
```

`trackfw agents install --targets gemini` e `trackfw skills install --targets gemini` executam `manager.Install` (cria `.gemini/agents/`) e `InjectRulesForTool` (cria GEMINI.md), mas **nunca chamam** `InjectHooksDetected`. Mecanismo identico ao do init.

Medicao (agents install, dir vazio, git init, HOME isolado):
```
$ cd $SCRATCHPAD/m-agents && git init -q
$ $BIN init --ai-tools claude --forge github 2>&1 | tail -1
  trackfw initialized ...
$ $BIN agents install --targets gemini --scope project 2>&1 | tail -2
  install complete: 12 agents artifact(s)
$ test -f GEMINI.md && echo exists || echo absent
  exists
$ grep -o 'trackfw guard' .gemini/settings.json 2>/dev/null | wc -l || echo absent
  absent
```

GEMINI.md criado por `agents install`, `.gemini/settings.json` ausente — mesma causa. Esta superficie deve entrar nesta REQ como ML adicional (ver Veredito, condicao 3).

### Update tem causa (b) para kiro

S2/S3 nao tem o bug de ordenacao (sinais pre-existem). Mas para kiro em global scope, update nao e caminho de reparo porque `.kiro/` nunca foi criado no projeto.

Medicao:
```
$ cd $SCRATCHPAD/m-kiro-update && git init -q
$ $BIN init --ai-tools kiro --forge github 2>&1 | tail -1
  trackfw initialized ...
$ $BIN update --targets agent-hooks 2>&1 | grep -E "updated|skipped"
  updated=0 skipped=1
```

`update --targets agent-hooks` reporta `skipped` para kiro porque `.kiro/` nao existe. Mesma causa (b). Um AC adicional de update entra no Wave 1 (ver Veredito, condicao 4).

### Superficies que NAO tem a mesma causa

- **opencode / antigravity:** sem `InjectXxxHooks` — causa estrutural diferente. Antigravity project scope usa `.agents/` (nao `.gemini/`) — sem colisao com gemini signal.
- **third-party skill install:** instala SKILL.md, nao cria sinais de deteccao de CLI.

### Tabela de medicao: 1a-2a execucao por CLI

Metodo: `grep -o 'trackfw guard' <hookfile> | wc -l`. HOME isolado em fakehome/ por medicao.
Hook file por CLI: claude=`.claude/settings.json`, codex=`.codex/hooks.json`, gemini=`.gemini/settings.json`, kiro=`.kiro/hooks/trackfw-attention.json`, copilot=`.github/hooks/trackfw-attention.json`, cursor=`.cursor/hooks.json`, windsurf=`.windsurf/hooks.json`, amazonq=`.amazonq/cli-agents/q_cli_default.json`.

| CLI | Sinal de deteccao | Sinal criado 1a exec | 1a exec guards | 2a exec guards |
|---|---|---|---|---|
| claude | `CLAUDE.md` ou `.claude/` | YES (por Scaffold, ANTES de InjectHooksDetected) | 7 | 7 |
| codex | `AGENTS.md` ou `.codex/` | YES (por installAITools) | 0 | 5 |
| gemini | `GEMINI.md` ou `.gemini/` | YES (por installAITools) | 0 | 7 |
| kiro | `.kiro/` apenas | NO (global scope nao cria `.kiro/` no projeto) | 0 | 0 |
| copilot | `.github/copilot-instructions.md` | YES (por installAITools) | 0 | 7 |
| cursor | `.cursor/` | YES (por installAITools) | 0 | 7 |
| windsurf | `.windsurfrules` | YES (por InjectRulesForTool dentro de InjectWindsurfHooks) | 0 | 1 |
| amazonq | `.amazonq/` | YES (por manager.Install) | 0 | 1 |
| opencode | nenhum sinal suportado | — | 0 | 0 |
| antigravity | nenhum sinal suportado | — | 0 | 0 |

Nota windsurf: sinal de deteccao = `.windsurfrules` (plaintext); arquivo de hook = `.windsurf/hooks.json` (JSON). Sao arquivos distintos. `InjectWindsurfHooks` chama `InjectRulesForTool("windsurf")` que cria `.windsurfrules`, e depois escreve `.windsurf/hooks.json`.

Nota amazonq: sinal de deteccao = `.amazonq/` (dir); arquivo de hook = `.amazonq/cli-agents/q_cli_default.json`. `manager.Install` cria `.amazonq/cli-agents/` e `.amazonq/developer/` na 1a run.

Reproducoes concretas:

Gemini 1a/2a exec:
```
$ cd $SCRATCHPAD/m1-gemini && git init -q
$ HOME=$SCRATCHPAD/m1-gemini/fakehome $BIN init --ai-tools gemini --forge github 2>&1 | tail -2
    gemini agents and skills
  trackfw initialized — run 'trackfw status' to see your governance state.
$ grep -o 'trackfw guard' .gemini/settings.json 2>/dev/null | wc -l || echo absent
       0
$ HOME=$SCRATCHPAD/m1-gemini/fakehome $BIN init --ai-tools gemini --forge github 2>&1 | tail -1
  trackfw initialized — run 'trackfw status' to see your governance state.
$ grep -o 'trackfw guard' .gemini/settings.json | wc -l
       7
```

Kiro 1a/2a/3a exec:
```
$ cd $SCRATCHPAD/m2-kiro && git init -q
$ HOME=$SCRATCHPAD/m2-kiro/fakehome $BIN init --ai-tools kiro --forge github 2>&1 | tail -2
    kiro agents and skills
  trackfw initialized — run 'trackfw status' to see your governance state.
$ ls .kiro/hooks/ 2>/dev/null | wc -l
       0
[runs 2 e 3: mesmo resultado, hooks present: 0]
```

Kiro controle (`.kiro/` pre-criado):
```
$ cd $SCRATCHPAD/m3-kiro-ctl && mkdir .kiro && git init -q
$ HOME=$SCRATCHPAD/m3-kiro-ctl/fakehome $BIN init --ai-tools kiro --forge github 2>&1 | tail -1
  trackfw initialized — run 'trackfw status' to see your governance state.
$ grep -o 'trackfw guard' .kiro/hooks/trackfw-attention.json | wc -l
       7
```

Windsurf e amazonq (1a run = 0; 2a run):
```
$ grep -o 'trackfw guard' .windsurf/hooks.json | wc -l
       1
$ grep -o 'trackfw guard' .amazonq/cli-agents/q_cli_default.json | wc -l
       1
```

Achados:

A. Claude funciona na 1a execucao por acidente de ordenacao. `generateClaudeMD` (scaffold.go:173) cria `CLAUDE.md` antes de `InjectHooksDetected` (scaffold.go:198). Claude nao depende de `installAITools`.

B. 7 CLIs com sinal de deteccao falham na 1a execucao (codex, gemini, kiro, copilot, cursor, windsurf, amazonq). Sinal criado por `installAITools` DEPOIS de Scaffold rodar InjectHooksDetected.

C. Kiro tem dois mecanismos de falha (secao 2, ameaca 2-2).

D. Achado de inconsistencia interna no codigo. `agentfiles.go:2422` comenta: "Kiro's project-scope injector (InjectKiroHooks) never wires git-branch-guard at all". Mas a implementacao (agentfiles.go:1028-1046) instala git-branch-guard explicitamente. O controle (`.kiro/` pre-criado) retornou 7 guards — confirmando que o codigo real diverge do comentario. O comportamento real e correto (protecao presente); o comentario e que esta errado. Correcao necessaria no Wave 1 (ver Veredito, condicao 7).

E. `trackfw validate` nao detecta guards ausentes pos-init. Confirmado: "No violations found." apos 1a init com `--ai-tools gemini`. Sem aviso; exit 0.

F. Kiro causa (a) isolada por controle: `mkdir .kiro && init --ai-tools kiro` -> 7 guards na 1a execucao. Confirma que a deteccao funciona quando o sinal pre-existe.

G. Caminho interativo (lido, nao executado). A ordem em `init.go` e identica para TTY e nao-TTY: `Scaffold` (linha 369) -> prompt de scope (apenas se TTY e aiTools nao-vazio) -> `installAITools` (linha 386). A unica diferenca e que TTY com scope=project faz `manager.Install` criar `.kiro/agents/` no projeto, o que fornece o sinal `.kiro/` — causa (b) nao se aplica em project scope. O bug de ordenacao (causa (a)) persiste em ambos os caminhos.

---

## 2. Threat Model

Adversario: O implementador apressado e o arquiteto otimista.

### Ameaca 2-1: Subestimacao de escopo — "so gemini e kiro"

A REQ foi aberta com o reprodutor `--ai-tools claude,gemini,kiro`. Sete CLIs sao afetados. Um fix que trate apenas gemini e kiro deixa codex, copilot, cursor, windsurf, amazonq com o mesmo gap. Um teste AC2 que so instancia gemini e kiro passa nessa interpretacao estreita.

Gate: AC2 para os 7 CLIs afetados explicitamente.

### Ameaca 2-2: Kiro tem dois mecanismos distintos de falha — o fix simples fecha apenas um

Causa (a) — ordenacao: `InjectHooksDetected` roda antes de `installAITools` criar `.kiro/`. Controle: `mkdir .kiro && init --ai-tools kiro` -> 7 guards na 1a exec. Corrigivel por reordenacao.

Causa (b) — sinal ausente em global scope: `installAITools` com scope="global" (padrao nao-TTY, init.go:120) instala em `~/.kiro/`, nao cria `.kiro/` no projeto. `InjectRulesForTool("kiro")` no-ops (kiro nao esta no mapa `agentFiles`). Apos a reordenacao, kiro continua sem hooks em CI e em qualquer init nao-interativo.

Discriminante para o Wave 1: Apenas reordenar fecha 6/7 CLIs. Fechar kiro exige despacho explicito por nome sobre `aiTools` (sem depender de deteccao) ou criacao explicita de `.kiro/`. O Wave 1 deve declarar qual abordagem adota.

O reprodutor da REQ e kiro sem TTY. "Kiro nunca" e o AC2 em CI. Nao e residuo aceito.

Gate: AC2 para kiro especificamente em modo nao-TTY (sem `.kiro/` pre-existente).

### Ameaca 2-3: Dedup com harness global — oracle de teste depende do HOME

Seis CLIs tem dedup de credential guard contra harness global: claude (agentfiles.go:363/406), codex (638), gemini (811), kiro (977), copilot (1141), cursor (1355). Windsurf e amazonq NAO tem `globalCredentialGuardInstalled<Tool>()` — nao existe credential guard de projeto para eles (agentfiles.go:2424-2428).

Impacto no teste: o count de guards depende do HOME. Um HOME que aponta para `~` real com harness instalado dara contagem menor para os 6 CLIs com dedup (credential entries omitidas). Windsurf/amazonq nao sao afetados por este mecanismo.

Gate: testes de AC2 devem usar HOME isolado (`t.Setenv("HOME", t.TempDir())`). Vault: `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`.

### Ameaca 2-4: InjectKiroHooks sobrescreve trackfw-attention.json existente

`InjectKiroHooks` usa `os.WriteFile` — overwrite total (agentfiles.go:1051). `InjectGeminiHooks` faz merge JSON.

Comportamento intencional: `trackfw-attention.json` e propriedade exclusiva do trackfw pelo nome dedicado. Sibling files (`.kiro/hooks/user-custom.json`) sobrevivem. Wave 1 nao deve alterar.

### Ameaca 2-5: Fix instala hook para CLI nao pedido (falso positivo)

`installAITools` chama `InjectRulesForTool(tool, cwd)` apenas para tools na lista. Antigravity project scope usa `.agents/` (nao `.gemini/`) — sem colisao com sinal de deteccao do gemini.

O unico FP pre-existente e claude (CLAUDE.md incondicional em qualquer init, Achado A). Nao e agravado pelo fix.

### Ameaca 2-6: Protecao ausente na janela init -> first AI session

Init emite exit 0 sem aviso. Nessa janela: 6 CLIs (codex, gemini, copilot, cursor, windsurf, amazonq) sem guards de projeto; kiro sem guards em qualquer numero de runs.

Para windsurf e amazonq: sem harness global de nenhum tipo. Missing project hook = sem guarda alguma para esses dois CLIs.

Para os 6 CLIs com dedup de credential guard: harness global compensa o credential guard quando instalado, mas nao o git-branch guard de projeto ate a 2a run.

### Ameaca 2-7: AC3 mal-direcionado para update/discover

Update/discover nao tem o bug de ordenacao. A superficie correta para AC3 e `integrations_flags.go` (agents/skills install), que tem a causa identica mas esta ausente do roadmap.

---

## 3. Alvos de Falsificacao — Duas Direcoes

### S1 — ordenacao no init

**Falso-negativo** (hook ausente apos correcao):

| Vetor | Onde a sabotagem entra | Gate |
|---|---|---|
| Remover a nova chamada ou mover antes de `installAITools` | init.go | AC2: guards >= 1 em hookfile para cada CLI pedido, 1a exec |
| `InjectHooksDetected` retorna nil sem instalar | hooks.go | idem |
| Restaurar ordem antiga | init.go | AC4: reprova para os 7 CLIs afetados |

AC4 exclusao de claude: reverter a ordem nao afeta claude (CLAUDE.md criado antes pelo Scaffold). Excluir claude do AC4.

**Falso-positivo** (hook instalado para CLI nao pedido):

| Vetor | Gate |
|---|---|
| `installAITools` cria GEMINI.md sem gemini na lista | `init --ai-tools codex` -> `.gemini/settings.json` ausente |
| Segunda chamada detecta sinal de outro CLI pre-existente | comportamento brownfield: a declarar no AC |

### S1b — kiro causa (b) — global scope

**Falso-negativo:**

| Vetor | Gate |
|---|---|
| Fix usa so reordenacao; kiro em non-TTY continua sem `.kiro/` | AC2 para kiro nao-TTY: guards >= 1 em `.kiro/hooks/trackfw-attention.json` |

**Falso-positivo:**

| Vetor | Gate |
|---|---|
| Fix cria `.kiro/` quando kiro NAO esta em `--ai-tools` | `init --ai-tools gemini` -> `.kiro/hooks/` NAO existe |

### S5 — agents/skills install (integrations_flags.go)

**Falso-negativo:**

| Vetor | Gate |
|---|---|
| `agents install --targets gemini` cria GEMINI.md sem InjectHooksDetected | guards em `.gemini/settings.json` >= 1 apos `agents install --targets gemini` em dir vazio |

**Falso-positivo:**

| Vetor | Gate |
|---|---|
| `agents install --targets codex` em dir com GEMINI.md pre-existente -> InjectHooksDetected instala tambem hooks do gemini | Comportamento a declarar: se GEMINI.md pre-existe, gemini hooks instalados. Se o fix usar name-dispatch, este FP desaparece. |

### S4 + discover.go:88 (double-call)

**Falso-negativo** (se segunda call for removida): guards = 7 apos `discover --init` (idempotencia da call restante).

**Falso-positivo (duplicacao)**: guards = 7, nao 14. Medicao: 7. A segunda call e redundante por idempotencia.

---

## 4. Residuo Declarado

**R1 — Falso positivo pre-existente para claude (CLAUDE.md incondicional).**
Claude hooks instalados em qualquer `trackfw init` mesmo sem `--ai-tools claude`. Comportamento anterior a esta REQ; nao e agravado pelo fix.

**R2 — InjectKiroHooks sobrescreve trackfw-attention.json por design.**
Arquivo de propriedade exclusiva do trackfw. Hooks customizados devem estar em arquivos separados. Wave 1 nao deve alterar.

**R3 — `trackfw validate` nao detecta guards ausentes pos-init.**
Nenhuma regra existente. Adicionar ao validate esta fora do escopo desta REQ.

**R4 — opencode e antigravity nao tem InjectXxxHooks.**
Causa estrutural diferente; necessita REQ propria.

**R5 — Windsurf e amazonq: 1 guard apenas (git-branch), sem global fallback.**
Pre-existente a esta REQ. Nao agravado pelo fix. Windsurf/amazonq nao tem credential guard em nenhum scope (design, agentfiles.go:2424-2428).

---

## Veredito

**LIBERA a Wave 1**, com as seguintes condicoes obrigatorias:

1. **AC2 para os 7 CLIs afetados** (codex, gemini, kiro, copilot, cursor, windsurf, amazonq), com HOME isolado por CLI.

2. **AC2 para kiro nao-TTY** (sem `.kiro/` pre-existente): guards >= 1 em `.kiro/hooks/trackfw-attention.json` apos 1a execucao. O Wave 1 deve declarar qual mecanismo fecha a causa (b).

3. **`integrations_flags.go` (agents/skills install) entra nesta REQ como ML adicional** antes do Wave 1. O arquiteto deve adicionar o ML ao roadmap.

4. **AC adicional de update para kiro:** apos o fix, `trackfw update --targets agent-hooks` em diretorio com init corrigido de kiro deve reportar `updated=1`, nao `skipped=1`.

5. **AC4 exclui claude.** Teste com ordem antiga deve reprovar para cada um dos 7 CLIs afetados.

6. **Gate da Wave 0** — o roadmap tem atualmente `test -s docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md`. Este documento satisfaz essa condicao. O arquiteto deve acrescentar:
   ```bash
   grep -q "Veredito" docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md
   ```
   Nota: `go test -run <NomeInexistente>` retorna exit 0 ("no tests to run") — nao usar como Wave 0 gate antes do Wave 1 existir.

7. **Comentario agentfiles.go:2422 deve ser corrigido no Wave 1.** O comentario diz "never wires git-branch-guard at all"; a implementacao (linhas 1028-1046) instala git-branch-guard. O controle mediu 7 guards. O comportamento real e correto; o comentario e que esta errado.

---

*Hades (Security), 2026-10-07. Binary: 398e75de.*
