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
| `InjectClaudeHooks` | `CLAUDE_CONFIG_DIR/.claude/settings.json` | projeto | sim (todos os matchers) |
| `InjectCodexHooks` | `.codex/hooks.json` | projeto | sim (Bash, apply_patch) |
| `InjectGeminiHooks` | `.gemini/settings.json` | projeto | sim (todos os matchers) |
| `InjectCursorHooks` | `.cursor/hooks.json` | projeto | parcial — só `beforeShellExecution`/`afterShellExecution` |
| `InjectWindsurfHooks` | `.codeium/windsurf/hooks.json` | projeto | git-branch migra; credential guard não migra |
| `InjectAmazonQHooks` | `.amazonq/cli-agents/q_cli_default.json` | projeto | git-branch migra; credential guard não migra |
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
