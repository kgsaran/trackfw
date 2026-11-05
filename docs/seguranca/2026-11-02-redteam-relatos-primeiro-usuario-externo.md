# Red Team — Wave 2: Relatos do Primeiro Usuário Externo

> Hades (hades-tf) | 2026-11-04 | REQ-2026-11-02 | issues #657–661
> Branch: fix/relatos-do-primeiro-usuario-externo
> Binários medidos: branch 11.3.0 compilado de `go build ./cmd/trackfw`; main por `git archive`

---

## Veredito geral

**REPROVA.**

T-12 (BLOQUEANTE): `preCheckAIToolDestinations` não cobre a escrita em
`<proj>/.claude/commands/trackfw` feita por `generateClaudeCommands()` dentro de `Scaffold`.
Com `<proj>/.claude` como symlink — com ou sem `--ai-tools` — Scaffold escreve 11 arquivos
antes de a guarda de escrita disparar. AC4 é violado para esse cenário. Main e branch
produzem resultado idêntico para (b) e (c): a branch não melhora esses casos.

A causa raiz — Scaffold verifica cada destino na hora da escrita, depois de outros já terem
sido escritos — se manifesta igualmente para symlinks em `scripts/` (5 arquivos) e `vault/`
(1 arquivo). Por CLAUDE.md Regra Dura de Causa Raiz: mesma causa → mesmo PR → não pode
mergear com sítio aberto.

T-8 (LOW): o gate prescrito na Wave 0 para o caminho TTY (T-8) nunca foi construído.
Mutação confirmada: remoção da chamada em `init.go:548` → suite passa (`ok 97.511s`).

---

## 1. Veredito por AC

| AC | Issue | Descrição | Resultado |
|----|-------|-----------|-----------|
| AC1 | #658 | Mensagem de binary probe com as duas cláusulas (incondicional) | PASS |
| AC2 | #660 | `wip_limit` respeita a severidade configurada (`warning/error/off`) | PASS |
| AC3 | #661 | `pkg_manager` detectado por presença de `package.json` | PASS |
| AC4 / parte 1 | #657 | Pre-check cobre HOME symlink (cenário a) | PASS |
| AC4 / parte 2 | #657 | Pre-check cobre `<proj>/.claude` symlink, scope=global | **FAIL** (T-12) |
| AC5 | #659 | Regra `adr_req_has_content` operacional | PASS |
| AC6 / barrier | — | `roadmap move` não bloqueia com `wip_limit: error` (medido empiricamente) | PASS |
| AC6 / PR | #657–661 | Comentário + `Closes` nos issues | PENDENTE (etapa de PR) |

---

## 2. Reprodução dos cinco issues (main × branch)

### #657 — Init pela metade

**Cenário (a): `~/.claude` é symlink, `--ai-tools claude`**

| Binário | Arquivos escritos em `$PROJ` | rc | Fonte do erro |
|---------|-----------------------------|----|---------------|
| main | 20 (Scaffold completo + `.claude/settings.json` + slash commands) | 1 | write-time guard em `installAITools` ao tentar `~/.claude/agents/...` |
| branch | 0 | 1 | pre-check Group 1 (`BuildPlans`/`manager.List`) antes de Scaffold |

Branch elimina completamente a partial-init para cenário (a). AC4 satisfeito para este caso.

**Cenário (c): `<proj>/.claude` é symlink, `--ai-tools claude`, scope=global**

| Binário | Arquivos escritos | rc | Fonte do erro |
|---------|------------------|----|---------------|
| main | 11 | 1 | write-time guard em `generateClaudeCommands` |
| branch | 11 | 1 | write-time guard em `generateClaudeCommands` |

Branch e main produzem resultado idêntico. O pre-check da branch não cobre
`.claude/commands/trackfw`. Os 11 arquivos são escritos antes de o guard disparar.

**Cenário (b): `<proj>/.claude` é symlink, sem `--ai-tools`**

| Binário | Arquivos escritos | rc |
|---------|------------------|----|
| main | 11 | 1 |
| branch | 11 | 1 |

`preCheckAIToolDestinations` retorna imediatamente quando `len(aiTools)==0`.
`generateClaudeCommands()` é chamado incondicionalmente em `Scaffold`, independente de
`--ai-tools`. Resultado idêntico.

**Cenário (c-proj): `<proj>/.claude` é symlink, `--scope project`**

| Binário | Arquivos escritos | rc | Hint emitido | Correto? |
|---------|------------------|----|--------------|----------|
| branch | 0 | 1 | "use --scope project" | Não — scope já é project |

Group 1 detecta o symlink porque com scope=project os BuildPlans incluem
`<proj>/.claude/agents/...`. Zero arquivos escritos. Mas o hint é enganoso: o usuário já
usa `--scope project`. O hint é emitido por `init.go:122` sem verificar o scope atual.

**Partial-init por posição na sequência de Scaffold:**

```
Ordem de chamadas em scaffold.go:
  121  MkdirAll govDirs[9]
  133  generateVaultIndex        → vault/notes/index.md
  137  generateGitAttributes     → .gitattributes
  145  generateGitIgnore         → .gitignore
  149  writeTrackfwConfig        → trackfw.yaml
  153  generateValidateScript    → scripts/trackfw-validate.sh
  157  GenerateAttentionScripts  → scripts/
  161  GenerateCredentialGuardScript → scripts/
  165  GenerateGitBranchGuardScript  → scripts/
  169  generateCIWorkflow        → .github/workflows/
  173  generateGitHooks          → .git/hooks/
  177  generateCommitMsgHook     → .git/hooks/commit-msg
  181  generateClaudeMD          → CLAUDE.md
  185  generateClaudeCommands    → .claude/commands/trackfw  ← guard dispara aqui para (b)/(c)
  206  InjectHooksDetected       → .claude/settings.json (não alcançado quando 185 erra)
```

Quanto mais tarde o symlink aparece na sequência, mais arquivos são escritos antes do erro:

| Symlink | Arquivos escritos antes do guard (branch) |
|---------|------------------------------------------|
| `<proj>/.claude` | 11 |
| `<proj>/scripts` | 5 |
| `<proj>/vault` | 1 |

---

### #658 — Mensagem de binary probe enganosa

| Situação | main | branch |
|----------|------|--------|
| Binário ausente | "fail open" apenas | ambas as cláusulas: "blocks (exit 2)" + "fail open" |
| Sem subcomando guard | "fail open" apenas | ambas as cláusulas: "blocks (exit 2)" + "fail open" |

Mensagens incondicionais em todos os 4 sítios (`validator_guard_binary_probe.go:308,321` e
`validator_guard_binary_probe_gitbash.go:512,547`).

**4 testes AmbasClausulas executados em macOS (sem build tag de Windows):**

```
go test ./internal/validator/ -run "AmbasClausulas" -count=1 -v
PASS: TestGuardBinaryProbeOnce_BinarioAusente_AmbasClausulas
PASS: TestGuardBinaryProbeOnce_BinarioSemGuard_AmbasClausulas
PASS: TestGuardGitBashBinaryProbe_Ausente_AmbasClausulas
PASS: TestGuardGitBashBinaryProbe_SemGuard_AmbasClausulas
ok  ...validator   0.290s
```

**Nota LOW-2:** main tinha `pip install -g trackfw` (inválido; pip não tem `-g`). Branch
corrige para `make install / npm install -g trackfw` mas omite o canal PyPI correto
(`pip install trackfw` / `pipx install trackfw`).

**Nota LOW-3:** a mensagem sem-guard sugere "run `trackfw update` to regenerate hook forms."
Medido por `trackfw update --help`: `trackfw update` (project-scope) atualiza CLAUDE.md rules
block, slash commands e Git hooks cirúrgicos, mas NOT `.claude/settings.json` (Claude AI
hooks). Para hooks globais, `trackfw update harness` ou `trackfw agents update` existem
(verificado por `--help`), mas o mapeamento exato entre esses comandos e a regeneração de
`.claude/settings.json` não foi medido nesta wave. A mensagem atual aponta para o comando
errado.

---

### #659 — ADR e REQ gerados com conteúdo só de modelo

**Medição no repositório real:**
```
82 ADRs com ## Context → 0 avisos adr_req_has_content
8 REQs com - [ ] nu ou só comentários → 8 avisos
0 falsos positivos
```

**Vetores Wave 0 (T-10 e T-11) medidos diretamente:**

| Vetor | Descrição | Resultado |
|-------|-----------|-----------|
| T-10 | Comentário HTML não fechado (`<!--` sem `-->`) | PASS: `## Motivation` detectada como model-only |
| T-11 | CRLF no modelo (`<!-- model\r\n`) | PASS: ambas as seções detectadas como model-only |

**Residuais medidos:**

| Residual | Comportamento |
|----------|--------------|
| R-7: char único | `x` em cada seção → sem aviso ✓ |
| R-7 estendido: `* [ ]` | passa como conteúdo real → LOW-1 |
| R-8: headings pt-BR (`## Contexto`) | silencia corretamente ✓ |
| R-9: seção ausente | ADR sem `## Context` → sem aviso ✓ |

Main não tem a regra `adr_req_has_content`. Branch: regra em ambos os caminhos
(`validator.go:958` e `:1496`, com comentário 🔴 sobre sítio espelho).

---

### #660 — `wip_limit` ignorava a severidade configurada

| Configuração | main | branch |
|-------------|------|--------|
| padrão | ⚠ warning | ⚠ warning |
| `rules.wip_limit: error` | ⚠ (não promovido) | ✗ violation |
| `rules.wip_limit: off` | — | silenciado |

Branch: `applyRule("wip_limit", ...)` em `validator.go:962` e `:1500`.

**Barrier / roadmap move:** `roadmap move` não chama `validateWIPLimit` → rc=0 mesmo com
`rules.wip_limit: error`. Medido empiricamente. Sem regressão.

---

### #661 — npm gravado em projetos Python

| Projeto | main `pkg_manager` | branch `pkg_manager` |
|---------|-------------------|---------------------|
| Python-only (sem `package.json`) | `npm` | `` (vazio) |
| Com `package.json` | `npm` | `npm` |

Branch: `os.Stat(filepath.Join(cwd, "package.json"))` em `init.go:243`.

---

## 3. Achados

### T-12 — BLOQUEANTE: Pre-check não cobre `generateClaudeCommands` nem govDirs

**Causa raiz:** Scaffold verifica cada destino individualmente na hora da escrita
(`rejectScaffoldPath`). `generateClaudeCommands()` escreve `ClaudeCommandsDirPath` =
`.claude/commands/trackfw` incondicionalmente (sem gating em `--ai-tools`).
`preCheckAIToolDestinations` retorna em `len(aiTools)==0`, não protegendo o caminho sem
`--ai-tools`. Mesmo com `--ai-tools`, Group 2 cobre `AgentFilePath("claude")` = `CLAUDE.md`,
não `.claude/commands/trackfw`.

A lacuna de prescrição está em P-7 (Wave 0), que mandou Group 2 cobrir
`filepath.Join(cwd, agentFiles[tool])`. Para `claude`, `agentFiles["claude"] = "CLAUDE.md"`,
que não está sob `.claude/`. O implementador seguiu P-7 corretamente. P-7 não contemplou
`generateClaudeCommands()`.

A causa é mais ampla: o padrão write-time-check de Scaffold produz partial-init para qualquer
symlink no projeto, independentemente do alvo. Os casos scripts e vault têm a mesma causa.

**Por que BLOQUEANTE:** branch não melhora (b) e (c) — main e branch são equivalentes.
Por CLAUDE.md: "Não se mergeia o PR 'parcial' prometendo o resto depois."

**Ausência de residual:** R-6 (declarado) cobre TOCTOU. T-12 é lacuna de cobertura — o
symlink existe, mas o caminho checado é errado ou a checagem não ocorre antes das escritas.

---

**Tabela de opções de fix vs. casos medidos:**

| Fix | (a) HOME/.claude | (b) .claude sem --ai-tools | (c) .claude + --ai-tools | scripts symlink | vault symlink |
|-----|-----------------|---------------------------|--------------------------|----------------|---------------|
| current branch | FECHADO | ABERTO | ABERTO | ABERTO | ABERTO |
| A: add ClaudeCommandsDirPath a Group 2 | FECHADO | ABERTO | FECHADO (só --ai-tools) | ABERTO | ABERTO |
| C: upfront check de ClaudeCommandsDirPath (não gateado) | FECHADO | FECHADO | FECHADO | ABERTO | ABERTO |
| **D: upfront check sobre todos os destinos de Scaffold** | **FECHADO** | **FECHADO** | **FECHADO** | **FECHADO** | **FECHADO** |

**Triagem (Regra Dura):** scripts e vault têm a mesma causa. Fix D fecha todos os casos e
é o único compatível com a Regra Dura.

**Fix recomendado (novo ML na REQ vigente):**

Adicionar loop de pre-check em `Scaffold` (scaffold.go), antes do primeiro `MkdirAll`,
cobrindo `govDirs` mais os principais destinos de arquivo:

```go
// Pre-check: verifica todos os destinos antes de qualquer escrita.
// pathguard.RejectAndReport tolera ENOENT — paths que não existem ainda são OK.
preCheckTargets := append(govDirs,
    "scripts/trackfw-validate.sh",
    ClaudeCommandsDirPath,  // = ".claude/commands/trackfw"
)
for _, rel := range preCheckTargets {
    abs := filepath.Join(cwd, rel)
    if err := pathguard.RejectAndReport(cwd, abs); err != nil {
        return fmt.Errorf("scaffold pre-check: %w", err)
    }
}
```

`pathguard` já é importado por `scaffold.go`. `govDirs` já é uma var no mesmo arquivo.

Correções adicionais no mesmo ML:
1. **Hint condicional por scope:** `init.go:122` emite o hint sem checar o scope atual. Com
   scope=project o hint "use --scope project" é enganoso. Fix: condicionar a `scope != "project"`
   (ou verificar que o path recusado está sob HOME).
2. Dois novos testes: `TestInitGlobalScopePreCheckAbortsOnSymlinkProjectClaudeDir` e
   `TestInitNoAIToolsAbortsOnSymlinkProjectClaudeDir` (rc=1, 0 arquivos escritos para ambos).

---

### T-8 — LOW: Gate Wave 0 "TTY pre-check" não construído

**Wave 0 §T-8 (verbatim):** "pré-checagem só no non-TTY path" — gate: "TTY com scope mockado
e HOME com symlink → Scaffold não chamado."

Branch chama `preCheckAIToolDestinations` em AMBOS os caminhos (TTY: `init.go:548`,
non-TTY: `init.go:274`). Mas nenhum teste cobre o caminho TTY.

**Mutação confirmada:**
```
# remoção de init.go:548 em cópia isolada do fonte
go test ./internal/commands/ -count=1 → ok  97.511s   (suite passa)
```

Remover a linha 548 não quebra nenhum teste.

---

### LOW-1: `* [ ]` e `1. [ ]` passam como conteúdo real

`adrReqBareCheckboxRe = "(?m)^\\s*-\\s*\\[\\s*\\]\\s*$"` captura apenas a forma com traço.
`* [ ]` e `1. [ ]` têm caracteres não-whitespace e passam sem aviso. Dentro de R-7 declarado.
Não bloqueia merge.

---

### LOW-2: PyPI ausente da mensagem de remediação

Branch omite `pip install trackfw` / `pipx install trackfw`. Não bloqueia merge.

---

### LOW-3: Sugestão `trackfw update` imprecisa na mensagem sem-guard

A mensagem "run `trackfw update` to regenerate hook forms" aponta para o comando errado.
`trackfw update --help` (medido): atualiza CLAUDE.md rules block, slash commands e Git hooks
cirúrgicos, mas não `.claude/settings.json`. Para hooks globais de CLI, `trackfw update harness`
ou `trackfw agents update` existem (verificado por `--help`); comportamento live sobre
`.claude/settings.json` não medido nesta wave. Não bloqueia merge.

---

## 4. Tabela de falsificação (títulos verbatim da Wave 0)

| ID Wave 0 | AC | Título (verbatim) | Resultado branch |
|-----------|----|--------------------|-----------------|
| T-1 (AC1) | AC1 | novo teste cobre 308/321 mas não 511/543 | PASS: 4 testes `AmbasClausulas` cobrem todos os 4 sítios |
| T-2 (AC1) | AC1 | nova mensagem overclaima "blocks" quando formas legadas presentes | PASS: mensagem incondicional por construção — overclaim declarado |
| T-3 (AC1) | AC1 | hasAnyUnverifiedSubcmd setado somente em branch D2-legacy, esquecendo D11/D11Rev | PASS: mensagem incondicional elimina o predicado |
| T-4 (AC2) | AC2 | mudar apenas ruleDefaults, sem mudar config.go | PASS: config.go medido — `wip_limit: warning` em ambas as fontes |
| T-5 (AC2) | AC2 | loops wipWarningsLimit permanecem após fix | PASS: `applyRule("wip_limit", wipWarningsLimit, ...)` em `:962` e `:1500`; dois sites de append removidos |
| T-6 (AC2)¹ | AC2 | cache da probe retorna mensagem errada em segunda chamada com flags diferentes | PASS: mensagem incondicional torna T-6 moot para AC1; segunda chamada com flags distintos não medida |
| T-7 (AC4) | AC4 | pré-checagem cobre apenas BuildPlans, não InjectRulesForTool | PASS: `AgentFilePath("claude")` = `CLAUDE.md` coberto por Group 2 |
| T-8 (AC4) | AC4 | pré-checagem só no non-TTY path | **FAIL** (LOW): gate não construído; mutação `init.go:548` sobrevive à suite |
| T-9 (AC4) | AC4 | TOCTOU — junction criada entre pré-checagem e escrita | Residual R-6 declarado; write-time guards confirmados ativos |
| T-10 (AC5) | AC5 | comentário HTML não fechado consome seção seguinte | PASS: vetor `<!--` não fechado → warning correto |
| T-11 (AC5) | AC5 | CRLF no modelo mascara detecção | PASS: vetor `<!-- model\r\n` → ambas as seções detectadas |
| **T-12 (novo)** | AC4 | pre-check não cobre `<proj>/.claude` symlink (todos os caminhos de Scaffold) | **FAIL** (BLOQUEANTE): 11 arquivos escritos; main=branch; causa ampla inclui scripts/vault |

¹ T-6 é rotulado "(AC2)" na Wave 0, mas seu conteúdo (cache da probe de binary) é AC1. Rótulo copiado verbatim.

---

## 5. Validação das prescrições da Wave 0

| Prescrição | Código | Resultado |
|-----------|--------|-----------|
| P-1/P-2 → emendado para mensagem incondicional | `validator_guard_binary_probe.go:308-311,320-325`, `_gitbash.go:512-518,547-553` | IMPLEMENTADO |
| P-3: `--scope project\|global` com validação | `init.go:171-174,235-237` | IMPLEMENTADO |
| P-4: `pkg_manager` por `package.json` | `init.go:243-247` | IMPLEMENTADO |
| P-5: `wip_limit` via `applyRule`, padrão warning | `validator.go:962,1500`, `config.go` | IMPLEMENTADO |
| P-6: `wip_limit` não em `roadmap move` | `roadmap move` não chama `validateWIPLimit` | IMPLEMENTADO |
| P-7: pre-check antes de Scaffold nos dois caminhos | `init.go:274,548` — lacuna: Group 2 cobre `CLAUDE.md`, não `ClaudeCommandsDirPath`; nenhum grupo cobre govDirs | **IMPLEMENTADO COM LACUNA** (T-12) |
| P-8: regra `adr_req_has_content`, padrão warning | `validator.go:4927-5015`, ambos os caminhos | IMPLEMENTADO |

---

## 6. Pendências e recomendações

### BLOQUEANTE — novo ML na REQ vigente antes do merge

**T-12 — ML a adicionar ao roadmap vigente:**

1. Loop de pre-check em `Scaffold` (antes do primeiro `MkdirAll`) sobre `govDirs` +
   `scripts/trackfw-validate.sh` + `ClaudeCommandsDirPath`. Fecha (b), (c), scripts, vault.
2. Condicionar hint `--scope project` em `init.go:122` a `scope != "project"`. Fecha c-proj hint.
3. Dois novos testes: `TestInitGlobalScopePreCheckAbortsOnSymlinkProjectClaudeDir` e
   `TestInitNoAIToolsAbortsOnSymlinkProjectClaudeDir`.

### LOW — próximo ML ou fix incremental

- **T-8:** adicionar seam para `cbterm.IsTerminal` em `init_test.go` para cobrir o caminho TTY.
- **LOW-1:** alargar `adrReqBareCheckboxRe` para `* [ ]` e `1. [ ]`.
- **LOW-2:** adicionar `pip install trackfw` / `pipx install trackfw` à mensagem.
- **LOW-3:** corrigir sugestão de comando na mensagem sem-guard.

---

## Nota para o arquiteto

**Vault (escopo diferido):** a causa do T-12 e o padrão devem ser registrados: causa = "Scaffold
verifica cada destino na hora da escrita, depois de outros já escritos; `preCheckAIToolDestinations`
retorna cedo sem `--ai-tools` e cobre apenas `agentFiles[tool]`, não `ClaudeCommandsDirPath`";
pattern = "para Scaffold, o pre-check correto é um loop sobre todos os destinos antes do primeiro
`MkdirAll`, independente de flags de runtime; `pathguard.RejectAndReport` tolera ENOENT e já é
importado por `scaffold.go`." Memória de agente também diferida por restrição de escopo.

**ML-2A** no roadmap: ainda `Pendente` — transição para `Concluído` é do arquiteto.

**AC6 (issue comments / `Closes`):** pendente até o PR.

**Regra Dura:** o PR não pode ser mergeado sem o ML de T-12. O sítio aberto (cenários b, c,
scripts, vault) tem a mesma causa da issue #657.
