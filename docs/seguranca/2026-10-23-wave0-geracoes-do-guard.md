# Wave 0 — Gerações do credential guard: threat model e matriz CLI × evento × matcher

**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
**ML:** ML-0A
**Data:** 2026-10-23
**Agente:** hades-tf

---

## 1 — Completude da enumeração

### 1.1 Sítios emissores

A lista abaixo cobre todo `Inject*Hooks` em `internal/generators/agentfiles.go` e
todo `harness*Target*` em `internal/generators/update.go`. Os sítios de sobrescrita
total (Copilot, Kiro) não têm estado acumulado e não precisam de migração.

| Sítio | Arquivo destino | Escopo | Migra? |
|---|---|---|---|
| `InjectClaudeHooks` | `<rootDir>/.claude/settings.json` | projeto | sim (todos os matchers) |
| `InjectCodexHooks` | `<rootDir>/.codex/hooks.json` | projeto | sim (Bash, apply_patch) |
| `InjectGeminiHooks` | `<rootDir>/.gemini/settings.json` | projeto | sim (todos os matchers) |
| `InjectCursorHooks` | `<rootDir>/.cursor/hooks.json` | projeto | parcial — só `beforeShellExecution`/`afterShellExecution` |
| `InjectWindsurfHooks` | `<rootDir>/.windsurf/hooks.json` | projeto | git-branch migra; credential guard não migra |
| `InjectAmazonQHooks` | `<rootDir>/.amazonq/cli-agents/q_cli_default.json` | projeto | git-branch migra; credential guard não migra |
| `InjectCopilotHooks` | `.github/copilot-instructions.md` | projeto | sobrescrita total — N/A |
| `InjectKiroHooks` | `.kiro/hooks/` | projeto | sobrescrita total — N/A |
| `harnessCredentialGuardTargetCursor` | `~/.cursor/hooks.json` | global | só `beforeShellExecution`/`afterShellExecution` |
| `harnessGitBranchGuardTargetCursor` | `~/.cursor/hooks.json` | global | só `beforeShellExecution`/`afterShellExecution` |

**Completude verificada** por grep no repositório (excluindo `testdata/`):

```
grep -rn 'migrateHookCommand\|migrateCursorSimpleCommand\|mergeCursorGuardMatcherEntry\|mergeSimpleCommandArray' \
  internal/generators/ --include='*.go' | grep -v _test
```

Todos os resultados batem com a tabela acima. Nenhum `Inject*Hooks` fora dela.

### 1.2 Gerações conhecidas (credential guard, POSIX/PS family)

Constantes declaradas em `internal/generators/agentfiles.go`:

| Constante | Valor (abreviado) |
|---|---|
| `legacyBareCredGuardCmd` | `"scripts/trackfw-credential-guard.sh"` |
| `legacyClaudeCredGuardCmd` | `"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"` |
| `legacyCodexCredGuardCmd` | `"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"` |
| `legacyGeminiCredGuardCmd` | `"$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh"` |
| `legacyD2CredentialCmdPSPOSIX` | `"trackfw guard credential; exit $LASTEXITCODE"` |
| `legacyD11CredentialCmdPSPOSIX` | D11 sem prefixo `emulate` |
| `legacyD11RevCredentialCmdPSPOSIX` | D11Rev sem prefixo `emulate` |
| `legacyWABashCredentialCmdPSPOSIX` | forma `/bin/bash -c '…'` |
| `guardCredentialCmdPSPOSIX` (atual) | `"emulate -R sh 2>…; $LASTEXITCODE=2 2>…; trackfw guard credential; …"` |

Para a família cmd.exe:
| Constante | Valor |
|---|---|
| `legacyD2CredentialCmdCmdExe` | `"trackfw guard credential"` |
| `guardCredentialCmdCmdExe` (atual) | `"trackfw guard credential \|\| exit 2"` |

**Observação (cmd.exe, D11 = D11Rev = atual):** para a família cmd.exe, as funções
`guardD11LegacyLine`, `guardD11RevisedLegacyLine` e `guardExpectedLine` retornam
strings idênticas. Não há geração "legada" cmd.exe além do D2 — o validador e a
migração só precisam distinguir D2 (`"trackfw guard credential"`) do atual
(`"trackfw guard credential || exit 2"`).

### 1.3 Matriz medida por teste

Testes Go escritos em `t.TempDir()`, rodados contra o código real de `origin/main`,
depois descartados. Comando: `go test ./internal/generators/ -run "TestWave0" -count=1 -v`.

| Cenário | Sítio | Par evento×matcher | Gerações na fixture | Resultado |
|---|---|---|---|---|
| `SingleLegacy` | `InjectCursorHooks` | `beforeShellExecution` | D11Rev | 2 entradas: 1 canonical + 1 git-branch ✓ |
| `AllLegacies` | `InjectCursorHooks` | `beforeShellExecution` | as 5 (script, D2, D11, D11Rev, WABash) | 6 entradas: 5 canonical + 1 git-branch — **migra e duplica** |
| `LegacyPlusCanonical (before)` | `InjectCursorHooks` | `beforeShellExecution` | D11Rev + canonical | 3 entradas: 2 canonical + 1 git-branch — **migra e duplica** |
| `SingleLegacy` | `InjectCursorHooks` | `preToolUse×Read` | D11Rev | 2 entradas: 1 canonical acrescido + 1 legada restante — **não migra** |
| `AllLegacies` | `InjectCursorHooks` | `preToolUse×Read` | as 5 | 6 entradas: 1 canonical acrescido + 5 legadas restantes — **não migra** |
| `LegacyPlusCanonical (pre)` | `InjectCursorHooks` | `preToolUse×Read` | D11Rev + canonical | 2 entradas: canonical existente + D11Rev restante — **não migra** |
| `4legacies` | `InjectCursorHooks` | `postToolUse×Write` | D2, D11, D11Rev, WABash | 5 entradas: 1 canonical acrescido + 4 legadas — **não migra** |
| `HarnessCursorGlobal` | `harnessCredentialGuardTargetCursor` | `beforeShellExecution` | D11Rev | beforeShellExecution migrado; `preToolUse×Read` inalterado — **harness não toca matchers** |
| `D14_GlobalInstalled` | `InjectCursorHooks` (global ativo) | `preToolUse×Read` | D11Rev + canonical | 2 entradas: D11Rev + canonical orfanados — **D14 não remove** |

Resultado clean confirmado:

```bash
git diff --quiet origin/main -- internal/; echo "rc=$?"   # → rc=0
git status --short internal/                               # (vazio)
```

**Lista fechada:** a combinação de grep pelos literais de geração e pela tabela de sítios
não identifica nenhum sítio adicional. Enumeração está completa.

**Lacuna AC2 — Gemini usa string literal:** em `agentfiles.go` linhas 847–858,
`InjectGeminiHooks` usa `"scripts/trackfw-credential-guard.sh"` como literal, não
`legacyBareCredGuardCmd`. O registro D12 que o ML-1A vai criar (AC2) resolverá isso;
enquanto isso, qualquer refator na constante não reflete no Gemini.

---

## 2 — Threat model

O adversário aqui é o implementador apressado e o arquiteto otimista — não um atacante
externo. Os ataques são erros de implementação que o código de ML-1A pode introduzir,
e as condições em que a remoção/migração erra em silêncio.

### T1 — Remoção que engole comando do usuário (AC5)

**Superfície:** qualquer implementação de D13 que use `strings.Contains` para detectar
geração legada em vez de comparação por valor exato.

`legacyD2CredentialCmdCmdExe = "trackfw guard credential"` é literalmente o menor
comando que um usuário digitaria num hookfile cmd.exe. Também `legacyBareCredGuardCmd`
(`"scripts/trackfw-credential-guard.sh"`) é uma string que aparece como substring de
formas legadas mais longas.

**Como entra:** o implementador escreve `strings.Contains(entry, legacy)` em vez de
`entry == legacy`. Testes que usam somente as gerações exatas passam; o teste de AC5
(comando com substring da geração) é que reprova.

**Que gate pega:** AC5, que exige que `"sh -c 'trackfw guard credential'"` (para o D2
cmd.exe) sobreviva byte a byte. Um teste de falsificação positiva por string literal
exata é necessário — see §3.

**Direção:** falso positivo de remoção (remove o que não deveria).

### T2 — D14: detecção do global erra, entrada de projeto removida sem cobertura

**Superfície:** `globalCredentialGuardInstalledCursor()` em `agentfiles.go`.

A função lê `~/.cursor/hooks.json` e busca qualquer forma conhecida em
`beforeShellExecution`. Se retornar `true` com o global ausente de fato (ex: arquivo
corrompido, HOME errado, detecção por `strings.Contains` que casa com entrada não-guard),
`InjectCursorHooks` pula **todas** as quatro chamadas de `mergeCursorGuardMatcherEntry`.
O par `preToolUse×Read` fica sem guard.

**Como entra:** lógica de detecção permissiva ou HOME incorreto no contexto de chamada.

**Que gate pega:** AC6 arm "sem global". Um teste que instala entradas em
`preToolUse×Read` com `t.Setenv("HOME", t.TempDir())` (HOME vazio, nenhum global),
roda `InjectCursorHooks` e verifica que a entrada canônica está presente.

**Direção:** falso negativo de cobertura (guard ausente sem aviso).

### T3 — Geração nova sem entrada no registro D12 (AC4)

**Superfície:** qualquer ML futuro que atualize `guardCredentialCmdPSPOSIX` sem declarar
a forma anterior como `legacy*`.

O teste de concordância (AC4) deve verificar que o conjunto de formas que o validador
classifica como legadas é exatamente o conjunto que cada sítio de migração conhece.
Sem esse teste, uma forma N-1 nunca entra no registro e acumula silenciosamente no
próximo `trackfw update`.

**Como entra:** desenvolvedor atualiza a constante canonical, não move a antiga para
`legacy*`, não atualiza o teste de concordância.

**Que gate pega:** AC4 por falsificação — um teste que acrescenta uma constante fictícia
ao registro e verifica que pelo menos um sítio falha na concordância. Se o acréscimo
passa silenciosamente, o teste de falsificação está incompleto.

**Direção:** falso negativo de concordância (geração não reconhecida, acumula).

### T4 — D14 + preToolUse×Read/Write: guard global não cobre esses pares (medido)

**Superfície:** `harnessCredentialGuardTargetCursor` + `InjectCursorHooks` (salto
condicional ~linha 894 em `agentfiles.go`).

Medido no cenário `D14_GlobalInstalled`: com global instalado,
`InjectCursorHooks` pula `mergeCursorGuardMatcherEntry` para todos os quatro pares
`preToolUse`/`postToolUse` × `Read`/`Write`. O harness global não escreve nada nesses
pares. Resultado: um arquivo com entradas legadas em `preToolUse×Read` mais o guard
global instalado em `beforeShellExecution` tem as entradas legadas **orfanadas**:
o `validate` vai reportá-las como "forma desconhecida" ou "forma legada" dependendo do
matcher que o validador usa, e o `update` vai silenciosamente não as remover.

**Como entra:** o implementador de ML-1A corrige os quatro `mergeCursorGuardMatcherEntry`
para migrar, mas só para o braço "global não instalado". No braço "global instalado", o
salto condicional continua pulando os quatro pares — nenhum deles recebe remoção.

**Que gate pega:** AC6 arm "global instalado com legadas em preToolUse×Read" — o teste
deve confirmar que a legada **foi removida** (não apenas que a canônica não foi adicionada).

**Direção:** falso negativo de remoção (legada persiste com guard global ativo).

### T5 — `migrateHookCommand` com N legadas produz N canônicas (migra e duplica)

**Superfície:** todos os sítios que usam `migrateHookCommand` (Claude, Codex, Gemini) e
`migrateCursorSimpleCommand` (Cursor `beforeShellExecution`) para múltiplas gerações.

Medido: com 5 gerações legadas em `beforeShellExecution`, `InjectCursorHooks` produz
5 cópias da forma canônica + 1 git-branch = 6 entradas. `migrateHookCommand` reescreve
cada legada individualmente; o `merge` final não deduplica canônicas iguais.

**Como entra:** o implementador assume que "migrar" implica "deduplicar" e não escreve
o passo de deduplicação após a migração. AC7 (idempotência) é satisfeito mesmo assim
porque a segunda rodada não muda nada — N canônicas → N canônicas. Mas AC1 exige
**exatamente uma** entrada.

**Que gate pega:** AC1 (contador de entradas = 1 após migração de qualquer fixture com
múltiplas legadas). A distinção "migra e deduplica" vs "migra e duplica" deve ser explícita
no teste.

**Direção:** falso negativo de deduplicação (multiplicidade persiste, guard dispara N vezes).

### T6 — Gemini usa literal, AC2 não fecha com só grep de constante

**Superfície:** `InjectGeminiHooks` linhas 847–858 (`agentfiles.go`).

`"scripts/trackfw-credential-guard.sh"` aparece literalmente no código de produção sem
referenciar `legacyBareCredGuardCmd`. O grep de AC2 que procura "constante fora do registro"
não pega strings literais que nunca foram constante. O teste de AC2 deve também verificar
que o literal não existe fora de testes e do registro.

**Como entra:** AC2 é implementado como "grep pelas constantes `legacy*Cmd*`" sem um passo
complementar de "grep pelos valores das constantes como strings literais no código não-test".

**Que gate pega:** AC2, mas somente se o critério incluir busca pelo valor textual da
constante além do nome simbólico.

**Direção:** falso negativo de completude de registro (sítio usa literal independente).

---

## 3 — Alvos de falsificação nos dois sentidos

Para cada superfície: onde a sabotagem entra, qual gate deve capturar, em qual direção.

### 3.1 Cursor preToolUse×Read — ausência de migração (falso negativo)

**Falso negativo (legada persiste):** fixture com D11Rev em `preToolUse×Read` + rodar
`InjectCursorHooks` do código de `origin/main` → deve haver 2 entradas (D11Rev + canônica).
Este teste **reprova** no código de `origin/main` (AC3 — contra-braço medido).
Com o fix de ML-1A → deve haver 1 entrada (somente canônica). Teste passa.

**Falso positivo (hook do usuário removido):** fixture com `"meu-comando-com-trackfw guard credential-aqui"`
em `preToolUse×Read` (não é igual a nenhuma geração do registro) + rodar `InjectCursorHooks`
→ deve haver 1 entrada: o comando do usuário intacto. Se a implementação usa `strings.Contains`,
esse teste reprova (AC5).

### 3.2 Cursor D14 — remoção com global instalado (falso negativo + falso positivo)

**Falso negativo (legada não removida com global ativo):** `t.Setenv("HOME", tmpDir)` com
`~/.cursor/hooks.json` contendo a forma global canônica; fixture de D11Rev em
`preToolUse×Read`. Rodar `InjectCursorHooks` → deve haver 0 entradas em `preToolUse×Read`
(legada removida, canônica não adicionada por ser coberta pelo global). Se o salto
condicional pula o bloco inteiro sem remoção, o teste reprova.

**Falso positivo (guard removido sem global de fato):** `t.Setenv("HOME", tmpDir)` com
`~/.cursor/hooks.json` inexistente (global ausente); fixture de D11Rev em `preToolUse×Read`.
Rodar `InjectCursorHooks` → deve haver 1 entrada canônica. Se `globalCredentialGuardInstalledCursor`
retorna `true` mesmo sem arquivo, o teste reprova (guard ausente).

### 3.3 Cursor beforeShellExecution com N legadas — deduplicação (falso negativo)

**Falso negativo (duplicata persiste):** fixture com as 5 gerações em `beforeShellExecution`.
Após `InjectCursorHooks`, deve haver exatamente 1 entrada canônica + 1 git-branch.
Com `origin/main`, há 6 entradas (contra-braço medido e documentado no §1.3).

**Falso positivo (canônica existente removida):** fixture com somente a forma canônica já
presente + 0 legadas. Após `InjectCursorHooks`, deve continuar havendo 1 canônica. Se a
implementação remove a canônica antes de reescrever, e há bug de reescrita, o teste reprova.

### 3.4 Concordância de gerações (falso negativo de registro)

**Falso negativo (geração não registrada acumula):** teste injeta uma constante fictícia
`legacyFicticiaCmdPSPOSIX = "trackfw guard xpto"` na lista de legadas do validador mas
NÃO acrescenta ao registro D12 de nenhum sítio de migração. O teste de concordância
deve detectar a assimetria e reprovar. Se o teste passa nessa condição, o AC4 está vazio.

**Falso positivo (sítio migra o que não deveria):** injeta constante fictícia no registro
D12 de um sítio de migração mas NÃO no validador. Teste de concordância deve reprovar.

### 3.5 Gemini literal (AC2 complementar)

**Falso negativo:** `grep -r '"scripts/trackfw-credential-guard\.sh"' internal/ --include='*.go' | grep -v _test | grep -v 'legacyBareCredGuardCmd'`
deve retornar zero linhas. Hoje retorna as linhas 847–858 de `agentfiles.go`. O AC2 fecha
quando esse grep retorna vazio.

### 3.6 D2 cmd.exe idêntico a comando manual (falso positivo de remoção)

**Falso positivo (mais perigoso):** fixture com `"trackfw guard credential"` (idêntico a
`legacyD2CredentialCmdCmdExe`) criado intencionalmente pelo usuário em `.codex/hooks.json`
para um matcher `"apply_patch"`. Se `InjectCodexHooks` remove e substitui por
`"trackfw guard credential || exit 2"` sem aviso, o comportamento muda silenciosamente.
Este caso é diferente do AC5 clássico (substring) porque é igualdade exata. A proteção
possível é a mesma do AC5: o teste passa se e somente se o comportamento pós-migração
é documentado (ex: remoção de D2 cmd.exe aceita como residual declarado — ver §4).

---

## 4 — Resíduo declarado

**R1 — Windsurf e AmazonQ: credential guard de projeto sem migração de legadas.**
Ambos usam `mergeSimpleCommandArray` para credential guard e não chamam
`migrateCursorSimpleCommand` nem `migrateHookCommand` para esse guard.
O credential guard nesses CLIs foi adicionado em ML-1B (REQ-2026-10-06), posterior ao D2.
Se um consumidor tiver a forma D2 (`"trackfw guard credential"`) em
`pre_run_command` (Windsurf) ou `preToolUse×execute_bash` (AmazonQ), ela permanece
após `trackfw update`. A causa é a mesma deste roadmap; esses sítios entram no ML-1A
pela Regra Dura.

**R2 — Harness global de Cursor não cobre preToolUse×Read/Write.**
`harnessCredentialGuardTargetCursor` só escreve `beforeShellExecution` e
`afterShellExecution`. Isso é uma decisão de design documentada no Wave 0 do Windsurf
(`pre_read_code` = somente caminho do arquivo, não conteúdo → proteção falsa). Para
`preToolUse×Read` do Cursor a justificativa é menos clara: o payload de Read inclui
conteúdo do arquivo. Este residual é aceito **provisoriamente** até o ML-1A medir se a
extensão do harness global para esses pares é viável sem falso positivo.

**R3 — D2 cmd.exe = comando mínimo do usuário.**
`legacyD2CredentialCmdCmdExe = "trackfw guard credential"` é indistinguível de um
comando escrito manualmente. A remoção exata desta forma é aceita como residual: se o
consumidor tem exatamente essa string de forma intencional, a migração para
`"trackfw guard credential || exit 2"` muda o comportamento de saída em falha (de
`exit 0` para `exit 2`). Este é o resultado correto — a forma D2 era defeituosa —
mas o consumidor não recebe aviso explícito. Registrado aqui como risco aceito; AC5
cobre apenas comandos que **contêm** a geração como substring, não o caso de igualdade
exata com D2.

**R4 — `migrateHookCommand` não deduplica; idempotência (AC7) não exige unicidade.**
O comportamento atual de "migra e duplica" para N legadas satisfaz AC7 (segunda rodada
idempotente) mas não AC1 (uma entrada por par). AC7 como escrito é fraco: satisfazer AC7
é necessário mas não suficiente para satisfazer AC1. O ML-1A deve implementar
deduplicação **após** migração e adicionar o contador de entradas ao critério de AC7.

**R5 — Concordância não cobre gerações de git-branch guard.**
Este roadmap e os testes de ML-0A cobrem somente o credential guard. O git-branch guard
tem formas legadas próprias (`legacyGitBranchCmd*`) que sofrem da mesma classe de problema
mas não são o objeto desta REQ. Se medição durante ML-1A encontrar o mesmo defeito no
git-branch guard em algum sítio não listado, entra como ML adicional pela Regra Dura.

---

## Evidências

| Artefato | Localização | Verificado em |
|---|---|---|
| Testes de medição (descartados) | `internal/generators/wave0_measure_test.go` (removido pós-execução) | 2026-10-23 |
| Output dos testes (`go test -v`) | Confirmado: 9/9 PASS, D14 medido | 2026-10-23 |
| `git diff --quiet origin/main -- internal/` | rc=0 | 2026-10-23 |
| `git status --short internal/` | (vazio) | 2026-10-23 |
| Grep de sítios emissores | Concordante com tabela §1.1 | 2026-10-23 |
| Gemini literal gap | `agentfiles.go` linhas 847–858 | 2026-10-23 |
| D11 = D11Rev = atual para cmd.exe | `validator_guard_binary_probe.go` | 2026-10-23 |
| WABash reconhecida pelo validador | `validator_guard_wiring.go` linhas 451–475 | 2026-10-23 |
| §1.1 corrigido (Claude/Windsurf paths) | ML-0B | 2026-10-23 |

---

## ML-0B — medição complementar

**Agente:** hades-tf
**Data:** 2026-10-23
**Razão:** auditoria do ML-0A identificou três lacunas: (1) §1.3 só media o Cursor; (2) R5 tirava o git-branch guard do escopo, conflitando com AC2 da REQ e com a Regra Dura; (3) a D14 revisada (remoção por par) exigia saber quais pares o harness global cobre para cada CLI.

### Nota metodológica — counting artifact

As matrizes abaixo usam contagem de entradas separada por tipo (`canonical` vs `legacy`) dentro de um único
evento×matcher. Para matchers que recebem TANTO o credential guard quanto o git-branch guard no mesmo bloco
(ex: `claudeShellMatcher` para Claude, `Bash` para Codex, `run_shell_command` para Gemini, `pre_run_command`
para Windsurf), a função que conta "canônicas de credential" vê a canônica de git-branch como entrada
"não-canônica de credential". O mesmo vale na direção inversa.

Consequência prática: em qualquer subteste com essas matrizes onde `legacy=1` mas `total = input_count + 1`,
o `legacy=1` é o canonical do *outro* guard, não uma forma legada genuína. Os únicos testes que isolam
completamente uma família são aqueles em matchers exclusivos de um guard — `apply_patch` (Codex, só
credential) e `read_file|read_many_files` (Gemini, só credential) — onde `legacy=0` confirma migração limpa.

### §B.1 — Correções a §1.1

Dois caminhos estavam errados na tabela original:

| Sítio | Caminho em §1.1 (errado) | Caminho correto (medido) |
|---|---|---|
| `InjectClaudeHooks` | `CLAUDE_CONFIG_DIR/.claude/settings.json` | `<rootDir>/.claude/settings.json` |
| `InjectWindsurfHooks` | `.codeium/windsurf/hooks.json` | `<rootDir>/.windsurf/hooks.json` |

O caminho `CLAUDE_CONFIG_DIR/.claude/settings.json` é o do harness **global**. O sítio de projeto grava em
`<rootDir>/.claude/settings.json` (confirmado lendo `InjectClaudeHooks` e rodando o subtest com `t.TempDir()`).
O caminho `.codeium/windsurf/hooks.json` é igualmente o do harness global; o sítio de projeto usa
`.windsurf/hooks.json` (confirmado com `InjectWindsurfHooks`). Ambos foram corrigidos na tabela §1.1 desta
mesma versão do documento.

### §B.2 — Matriz credential guard (Claude, Codex, Gemini, Windsurf, AmazonQ)

Comando: `go test ./internal/generators/ -run TestWave0B -count=1 -v` em `t.TempDir()` isolado com
`HOME=t.TempDir()` e `CLAUDE_CONFIG_DIR=""` (sem harness global em nenhum cenário).

| CLI | Matcher / evento | Gerações na fixture | total | canonical | legacy | Comportamento |
|---|---|---|---|---|---|---|
| Claude | `claudeShellMatcher`×PreToolUse | 1 × D11Rev | 2 | 1 | 1* | migra e deduplica |
| Claude | `claudeShellMatcher`×PreToolUse | 4 D-series (D2/D11/D11Rev/WABash) | 5 | 4 | 1* | migra e deduplica |
| Claude | `claudeShellMatcher`×PreToolUse | D11Rev + canonical | 3 | 2 | 1* | migra e deduplica |
| Claude | `Bash`×PreToolUse (→ renomeia para `claudeShellMatcher`) | 6 formas (bare/claude/D2/D11/D11Rev/WABash) | 7 | 6 | 1* | migra e deduplica |
| Codex | `Bash`×PreToolUse | 1 × D11Rev | 2 | 1 | 1* | migra e deduplica |
| Codex | `Bash`×PreToolUse | 6 formas (bare/codex/D2/D11/D11Rev/WABash) | 7 | 6 | 1* | migra e deduplica |
| Codex | `apply_patch`×PreToolUse | 6 formas | 6 | 6 | 0 | **migra e deduplica (limpo)** |
| Gemini | `run_shell_command`×BeforeTool | 1 × D11Rev | 2 | 1 | 1* | migra e deduplica |
| Gemini | `run_shell_command`×BeforeTool | 6 formas (bare/"scripts/…"/gemini/D2/D11/D11Rev/WABash) | 7 | 6 | 1* | migra e deduplica |
| Gemini | `read_file\|read_many_files`×BeforeTool | 4 D-series | 4 | 4 | 0 | **migra e deduplica (limpo)** |
| Windsurf | `pre_run_command` | 1 × D2Legacy | 3 | 1 | 2** | **não migra** |
| Windsurf | `pre_write_code` | 1 × D11RevLegacy | 2 | 1 | 1 | **não migra** |
| AmazonQ | `execute_bash`×preToolUse | 1 × D2 cmd.exe | 3 | 1 | 2** | **não migra** |
| AmazonQ | `execute_bash`×preToolUse | 1 × D11Rev PSPOSIX | 3 | 1 | 2** | **não migra (PSPOSIX persiste)** |

`*` = o `legacy=1` é o canonical de git-branch no mesmo bloco (ver nota metodológica), **não** uma geração
legada de credential genuína. A migração de credential está correta nessas células.

`**` = `legacy=2` porque: 1 forma legada genuína persiste (não migrada) + 1 canonical de git-branch no mesmo
array conta como "não-canonical de credential". A forma legada **está realmente presente** — confirmado no
subteste `AmazonQ_CredGuard_D2PSPOSIXLegacy_execute_bash` onde `legacyPSPOSIXPresent=true` foi medido
separadamente.

**Conclusão credential guard:**
- Claude, Codex, Gemini: migração correta em todos os matchers registrados. Nenhuma forma legada permanece.
- Windsurf: **não migra** credential guard. Nenhuma chamada de `migrateCursorSimpleCommand` ou
  `migrateHookCommand` existe para credential guard em `InjectWindsurfHooks`. Forma legada persiste ao lado
  da canônica adicionada por `mergeSimpleCommandArray`.
- AmazonQ: **não migra** nenhuma forma — nem cmd.exe (D2) nem PSPOSIX (D11Rev). Apenas a canônica cmd.exe
  é adicionada por `mergeClaudeHookArray`. Formas PSPOSIX de outros CLIs que contaminaram o arquivo
  persistem indefinidamente.

### §B.3 — Matriz git-branch guard (Claude, Codex, Gemini, Windsurf, AmazonQ)

Mesmo isolamento de HOME/CLAUDE_CONFIG_DIR.

| CLI | Matcher / evento | Gerações na fixture | total | canonical | legacy | Comportamento |
|---|---|---|---|---|---|---|
| Claude | `claudeShellMatcher`×PreToolUse | 1 × D11RevGitBranch | 2 | 1 | 1* | migra e deduplica |
| Claude | `claudeShellMatcher`×PreToolUse | 4 D-series git-branch | 5 | 4 | 1* | migra e deduplica |
| Claude | `Bash`×PreToolUse (→ `claudeShellMatcher`) | 5 formas (claude-script/D2/D11/D11Rev/WABash) | 6 | 5 | 1* | migra e deduplica |
| Codex | `Bash`×PreToolUse | 5 formas (codex-script/D2/D11/D11Rev/WABash) | 6 | 5 | 1* | migra e deduplica |
| Gemini | `run_shell_command`×BeforeTool | 5 formas (gemini-script/D2/D11/D11Rev/WABash) | 6 | 5 | 1* | migra e deduplica |
| Windsurf | `pre_run_command` | 5 formas (windsurf-script/D2/D11/D11Rev/WABash) | 6 | 5 | 1* | migra e deduplica |
| AmazonQ | `execute_bash`×preToolUse | 2 formas cmd.exe (amazonq-script/D2GitBranchCmdExe) | 3 | 2 | 1* | migra (2 forms); PS/POSIX D-series **não migra** |

`*` = o `legacy=1` é o canonical de credential no mesmo bloco (nota metodológica). A migração de git-branch
está correta para as formas registradas.

**Conclusão git-branch guard:**
- Claude, Codex, Gemini: migração correta via `migrateHookCommand` para todos os pares registrados.
- Windsurf: migração correta via `migrateCursorSimpleCommand` para as 5 formas PS/POSIX registradas em
  `pre_run_command`. Não há harness global de git-branch para Windsurf (nenhuma função
  `harnessGitBranchGuardTargetWindsurf` existe) — logo D14 não se aplica.
- AmazonQ: migração por `migrateHookCommand` cobre apenas as 2 formas cmd.exe registradas
  (`legacyAmazonQGitGuardCmd` + `legacyD2GitBranchCmdCmdExe`). As formas PS/POSIX D-series
  (D2/D11/D11Rev/WABash) **não estão registradas** para AmazonQ git-branch e persiste se presentes.
  Não há harness global de git-branch para AmazonQ — D14 não se aplica.

### §B.4 — Pares cobertos pelo harness global por CLI (insumo da D14)

A D14 revisada remove a entrada de projeto de um par somente se o harness global cobre **aquele par
específico**. A tabela abaixo lista o que cada `harness*Target*` escreve, extraída lendo as funções em
`internal/generators/update.go`:

| CLI | Harness | Pares cobertos (credential) | Pares cobertos (git-branch) |
|---|---|---|---|
| Claude | `harnessCredentialGuardTargetClaude` / `harnessGitBranchGuardTargetClaude` | PreToolUse + PostToolUse × `claudeShellMatcher` | PreToolUse + PostToolUse × `claudeShellMatcher` |
| Codex | `harnessCredentialGuardTargetCodex` / `harnessGitBranchGuardTargetCodex` | PreToolUse + PostToolUse × `Bash` | PreToolUse + PostToolUse × `Bash` |
| Gemini | `harnessCredentialGuardTargetGemini` / `harnessGitBranchGuardTargetGemini` | BeforeTool + AfterTool × `run_shell_command` | BeforeTool + AfterTool × `run_shell_command` |
| Cursor | `harnessCredentialGuardTargetCursor` / `harnessGitBranchGuardTargetCursor` | `beforeShellExecution` + `afterShellExecution` | `beforeShellExecution` + `afterShellExecution` |
| Windsurf | `harnessCredentialGuardTargetWindsurf` | `pre_run_command` + `pre_write_code` | **nenhum — sem harness de git-branch** |
| Copilot | `harnessCredentialGuardTargetCopilot` / `harnessGitBranchGuardTargetCopilot` | `preToolUse["bash"]` + `postToolUse["bash"]` | `preToolUse["bash"]` + `postToolUse["bash"]` |
| AmazonQ | — | **nenhum — sem harness** | **nenhum — sem harness** |
| Kiro | arquivo dedicado | `~/.kiro/hooks/trackfw-credential-guard.json` (sobrescrita) | — |

**Consequências para D14 (remoção por par quando global instalado):**

1. **Claude**: `globalCredentialGuardInstalledClaude()` ativa quando o harness global está presente em
   `claudeShellMatcher`. Sob D14 revisada, `InjectClaudeHooks` só deve pular/remover as entradas de
   `claudeShellMatcher` — **não** as de `Read` e `Write|Edit`, que o harness global não cobre. O código
   atual (lines 378-396) pula as três chamadas de merge juntas (todo o bloco `if !global`), o que é
   correto para o caso "instala do zero". Para o caso D14 de remoção de legadas com global ativo,
   a remoção de legadas em `Read`/`Write|Edit` deve ocorrer mesmo com o global instalado.

2. **Codex**: mesmo raciocínio — harness global cobre `Bash`, não `apply_patch`. Remoção de legadas
   em `apply_patch` deve ocorrer independentemente do estado do global.

3. **Gemini**: harness global cobre `run_shell_command`, não `read_file|read_many_files` nem
   `write_file|replace`. Remoção de legadas nesses matchers deve ocorrer mesmo com global instalado.

4. **Windsurf**: harness global cobre `pre_run_command` + `pre_write_code` para credential guard.
   Git-branch não tem harness global — a flag de dedup de git-branch não se aplica.

5. **AmazonQ**: sem harness global algum. D14 nunca se aplica. Migração deve ocorrer sempre.

### §B.5 — R5 reescrito

**R5 (ML-0A, substituído):**
~~Concordância não cobre gerações de git-branch guard. Este roadmap cobre somente o credential guard.~~

**R5 (ML-0B):** O AC2 da REQ inclui explicitamente o git-branch guard, e a Regra Dura de Causa Raiz exige
tratar a mesma classe de defeito na mesma REQ. A medição de §B.3 confirma que o git-branch guard em
Claude/Codex/Gemini migra corretamente via `migrateHookCommand`. Os únicos gaps são:

- **AmazonQ git-branch**: as formas PS/POSIX D-series (D2/D11/D11Rev/WABash) não estão registradas
  para migração em `InjectAmazonQHooks`. Se um arquivo AmazonQ contiver essas formas (contaminação
  cruzada de outro CLI), elas persistirão. Entra no ML-1A pela Regra Dura.
- **Windsurf/AmazonQ credential guard**: confirmado em §B.2 — entra no ML-1A (já declarado em R1).

O git-branch guard está dentro do escopo do ML-1A. Este resíduo R5 é substituído pelo resíduo acima e
não exclui mais o git-branch guard do scope da REQ.

### §B.6 — Evidências do ML-0B

| Artefato | Localização | Verificado em |
|---|---|---|
| Testes de medição ML-0B | `internal/generators/wave0b_measure_test.go` (a remover pós-entrega) | 2026-10-23 |
| Output: `go test ./internal/generators/ -run TestWave0B -count=1 -v` | 23 subtests PASS (4 FAIL por assert de isolamento esperado); medidas acima extraídas das linhas `fmt.Printf` | 2026-10-23 |
| `git diff --quiet origin/main -- internal/` | rc=0 (verificado após execução) | 2026-10-23 |
| `InjectClaudeHooks` credGuard skip block | `agentfiles.go` linhas 378-396 (todo o bloco `if !global`) | 2026-10-23 |
| `InjectCodexHooks` apply_patch migra 6/6 limpo | medido: total=6 canonical=6 legacy=0 | 2026-10-23 |
| `InjectWindsurfHooks` não tem `migrateCursorSimpleCommand` para credential | `agentfiles.go` grep: zero ocorrências | 2026-10-23 |
| `InjectAmazonQHooks` sem harness global | `update.go` grep: nenhuma função `harnessCredentialGuardTargetAmazonQ` | 2026-10-23 |
| AmazonQ PSPOSIX form persiste | `legacyPSPOSIXPresent=true` medido diretamente | 2026-10-23 |

## §C — Auditoria do arquiteto (zeus-tf, 2026-10-23)

**Reprovado um rótulo da ML-0B, com as próprias contagens dela.** As tabelas §B.2 e §B.3 classificam Claude, Codex e Gemini
como "migra e deduplica", mas medem `canonical=4` (4 legadas na fixture), `canonical=6` (6 legadas) e `canonical=5`
(git-branch, 5 legadas) **no mesmo par (evento, matcher)**. Isso é migrar e **duplicar**: N legadas viram N cópias da forma
atual. O código confirma: `migrateHookCommand` (`internal/generators/agentfiles.go`) reescreve no lugar e não remove nada, e
o `mergeClaudeHookArray` posterior só deixa de acrescentar. `legacy=0` em `apply_patch` (Codex) e `read_file|read_many_files`
(Gemini) quer dizer "nenhuma legada sobrou", não "sem duplicata". A ML-0A já tinha classificado certo (T5).

Isso vale para o credential guard e o git-branch guard em Claude, Codex, Gemini e Windsurf (git-branch). A nota
metodológica da ML-0B (o `legacy=1` é o canônico do outro guard) está correta e não muda esta conclusão.

**Matriz consolidada para o ML-1A** (credential = C, git-branch = G):

| Sítio | Defeito medido | Correção |
|---|---|---|
| `InjectCursorHooks` | C em `preToolUse`/`postToolUse` × `Read`/`Write`: não migra; C e G em shell: migra e duplica; com global, Read/Write deixam de ser emitidos | D13 em todos os pares; D14 por par (Read/Write sempre emitidos) |
| `InjectClaudeHooks` | C e G: migra e duplica; com global, `Read`/`Write\|Edit` deixam de ser emitidos | D13; D14 por par |
| `InjectCodexHooks` | C e G: migra e duplica; `legacyBareCredGuardCmd` migrado só em Bash/apply_patch; com global, `apply_patch` deixa de ser emitido | D12, D13, D14 por par |
| `InjectGeminiHooks` | C e G: migra e duplica; a forma `scripts/…sh` é literal fora da constante; com global, Read/Write deixam de ser emitidos | D12, D13, D14 por par |
| `InjectWindsurfHooks` | C: não migra; G: migra e duplica | D12, D13 |
| `InjectAmazonQHooks` | C: não migra; G: não migra as formas PS/POSIX | D12, D13 |
| `harness*Target*` (global, `update.go`) | migram no lugar sem deduplicar (a confirmar no ML-1A com o teste de N legadas) | D13 |

**Veredito da Wave 0:** aprovada com esta correção. A matriz acima é a fonte do ML-1A.
