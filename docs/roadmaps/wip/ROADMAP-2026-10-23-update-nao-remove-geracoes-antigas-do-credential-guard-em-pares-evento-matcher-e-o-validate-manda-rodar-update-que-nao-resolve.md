---
status: wip
date: 2026-10-23
req: "docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: update não remove gerações antigas do credential guard em pares evento×matcher

> Created: 2026-10-23 | Status: wip (reaberto em 2026-10-24 pela #613)

## Context
REQ: docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
ADR: ADR-2026-10-04, adendo de 2026-10-23 (D12–D14) e D15 de 2026-10-24, que substitui a D14. Issues: #599, #613.

Causa (lida no código pelo arquiteto): `InjectCursorHooks` migra as formas legadas só em `beforeShellExecution` e
`afterShellExecution`. Os pares `preToolUse`/`postToolUse` × `Read`/`Write` recebem apenas `mergeCursorGuardMatcherEntry`,
que acrescenta a forma atual e deixa as legadas. A lista de gerações está copiada à mão em cada sítio, e as cópias
divergem entre os CLIs.

## Acceptance Criteria
- [x] AC1–AC7 da REQ, cada um com a evidência registrada no ML que o fecha (AC8 = fechamento, abaixo).
- [x] `make quality` e o CI ficam verdes. (`make quality` rc=0; CI 20/20 no 39d0ac26; mergeado no #604, f2c7c7fd)
- [ ] Reabertura (#613): AC6 revisto, AC9 e AC10 da REQ, nas Waves 4–7.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e matriz da mesma causa
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e matriz CLI × escopo × evento × matcher × geração
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md` (novo; o único arquivo que o ML escreve)
**Actions:**
1. **Matriz medida.** Para cada sítio que emite um guard (credential e git-branch), em todo `Inject*Hooks` de
   `internal/generators/agentfiles.go` e em todo alvo `harness*Target*` de `internal/generators/update.go`: quais pares
   (evento, matcher) ele emite e quais gerações ele migra em cada par. Compare com o que o validador reconhece como legado
   (`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`, `validator_guard_wiring.go`,
   `validator_guard_binary_probe.go`). Marque cada célula como: migra e deduplica / migra e duplica / não migra.
   A medição é por teste Go em `t.TempDir()` (fixture com a geração → rodar o sítio real → contar as entradas), nunca no
   checkout. Um teste de medição descartável pode existir só durante a medição e não é entregue.
2. **Completude da enumeração.** Mostre que a lista de sítios está fechada, com o grep pelos literais das gerações e por
   `migrateHookCommand|migrateCursorSimpleCommand|merge.*Hook` em toda a árvore, fora de `testdata/`.
3. **Threat model.** O que a remoção abre: uma regra de remoção que case hook do usuário (comando que contém
   `trackfw guard credential`, mas não é igual a uma geração); a D14 removendo a entrada de projeto quando a detecção do
   global erra; uma geração nova acrescentada sem que algum sítio a migre.
4. **Alvos de falsificação nos dois sentidos**, por célula: o teste que reprova se a legada ficar, e o que reprova se um
   hook do usuário sumir.
5. **Resíduo declarado.**
**Acceptance criteria:**
- [x] A matriz com cada célula medida (comando e resultado), não inferida
- [x] As quatro seções acima respondidas com evidência
- [x] Nenhuma linha de produto escrita
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
grep -q 'Matriz' docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
git diff --quiet origin/main -- internal/
```

### ML-0B — Corretivo da auditoria: medir os CLIs que faltaram e o guard de git-branch
**Status:** ✅ Concluído
**Agente:** hades-tf
**Por que existe:** a auditoria da ML-0A achou três problemas. A matriz §1.3 só mediu o Cursor: as afirmações sobre
Claude, Codex, Gemini, Windsurf e Amazon Q ("migra e duplica", "não migra") não têm cenário medido. O R5 tirou o guard de
git-branch do escopo, mas o AC2 da REQ o inclui, e a Regra Dura manda medir. E a D14 foi revista por par, depois do T4/R2.
**Files affected:** `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md` (acréscimo de seção)
**Acceptance criteria:**
- [x] Matriz medida, com cenário e contagem, para Claude, Codex, Gemini, Windsurf e Amazon Q, nos estados só-legadas,
  N-legadas e legada+atual, em cada par (evento, matcher) que o sítio emite
- [x] Mesma matriz para o guard de git-branch, em todo sítio de projeto e global que o emite
- [x] Para cada CLI, quais pares o guard global cobre, que é o insumo da D14 revista
- [x] R5 reescrito, sem tirar o git-branch do escopo (rótulo "migra e deduplica" de Claude/Codex/Gemini corrigido pela auditoria, §C)
- [x] Nenhuma linha de produto (`git diff --quiet origin/main -- internal/` rc=0)
**Gates da wave:**
```bash
grep -q 'ML-0B' docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — Registro único, migração com deduplicação e todos os sítios da matriz
> Dependencies: Wave 0 auditada pelo arquiteto.

### ML-1A — Registro único, deduplicação genérica e o Cursor (a issue)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Fonte:** matriz consolidada em `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md` §C.
**Files affected:** `internal/generators/agentfiles.go`, um arquivo novo `internal/generators/guard_generations.go` (o registro),
os validadores que hoje listam gerações (`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`,
`validator_guard_wiring.go`, `validator_guard_binary_probe.go`) e testes novos em `internal/generators/`.
**Actions:**
1. **D12:** criar um registro com as gerações de cada guard (credential, git-branch) por família de shell (PS/POSIX, cmd.exe),
   incluindo as formas de caminho (`scripts/…sh`, `$CLAUDE_PROJECT_DIR/…`, `$(git rev-parse …)/…`, `$GEMINI_PROJECT_DIR/…`).
   Os validadores e os sítios de migração passam a consumir o registro, e o literal do Gemini sai.
2. **D13:** criar um helper que, para um par (evento, matcher), migra toda geração do registro para a forma atual e deixa
   **exatamente uma** entrada daquele guard. O helper preserva a posição da primeira entrada e todos os campos extras dela,
   e compara o comando inteiro por igualdade exata, nunca por `Contains`. Ele cobre as duas variantes de forma: a aninhada
   (`matcher` + `hooks[]`, Claude/Codex/Gemini) e a plana (`command` + `matcher` opcional, Cursor/Windsurf/Amazon Q).
3. **Cursor:** aplicar o helper em `beforeShellExecution`, em `afterShellExecution` e nos 4 pares `preToolUse`/`postToolUse` ×
   `Read`/`Write`. **D14 por par:** com o global instalado, as entradas de shell reconhecidas são removidas; as de
   Read/Write são sempre migradas e emitidas.
4. **Teste de concordância (AC4):** toda geração que o validador classifica como legada está no registro e é tratada por todo
   sítio que emite aquele guard. Falsificar acrescentando uma geração fictícia só ao validador.
**Acceptance criteria:**
- [x] AC1: fixture da issue (5 gerações × 4 pares, nos dois estados); validador real com 0 avisos de `.cursor/hooks.json`; contra-braço em `origin/main` com 4
- [x] AC2, AC4, AC5 (um hook do usuário que contém `trackfw guard credential` sobrevive byte a byte) e AC7 (segunda rodada igual byte a byte **e** uma entrada por par)
- [x] AC6 para o Cursor, nos três braços
- [x] Cada teste novo com a frase do que afirma e o contra-braço medido em `origin/main`
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ -count=1
```

**Auditoria do arquiteto (2026-10-23):** aprovado com ressalva. `go build ./...` rc=0; `go test ./internal/generators/
./internal/validator/ ./internal/pathguard/ -count=1` ok; `-run TestCursorGuardMigrateDedup -v`: 6 testes `--- PASS`
(UserHookSurvives, D14Arms, Idempotent, IdempotentFromLegacy, ValidatorAfterMigration, RegistryConcordance). O registro
está em `internal/guardforms/`, que é pacote folha porque o `validator` não pode importar o `generators`, e as constantes
do gerador são aliases. O helper compara por igualdade exata. Duas lacunas passam para o ML-1B, porque só fecham quando
todos os sítios usarem o helper:
- **AC4, sentido sítios:** o `RegistryConcordance` só afirma registro → validador, e passa nas duas árvores. Falta o teste
  que itera o registro × todo sítio × todo par.
- **AC2, formas inline do validador:** `guardExpectedLine`, `guardD11LegacyLine` e `guardD11RevisedLegacyLine`
  (`validator_guard_binary_probe.go`) ainda montam as linhas D2/D11/D11Rev/atual por template próprio. Falta derivá-las do
  registro, ou amarrá-las a ele por um teste de igualdade.

### ML-1B — Os demais sítios da matriz
**Status:** ✅ Concluído (lacunas fechadas pelos ML-1C, ML-1D e ML-1E)
**Agente:** apolo-tf
**Dependencies:** ML-1A. Sequencial porque usa o mesmo arquivo (`agentfiles.go`).
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/update.go` e testes.
**Actions:**
- Aplicar o helper da ML-1A em `InjectClaudeHooks`, `InjectCodexHooks` e `InjectGeminiHooks`, com D13 e com D14 por par:
  `Read`/`Write|Edit`, `apply_patch` e os matchers de leitura/escrita do Gemini são sempre emitidos.
- Aplicar também em `InjectWindsurfHooks` e `InjectAmazonQHooks`, para o credential e o git-branch, inclusive as formas
  PS/POSIX no Amazon Q.
- Em todo `harness*Target*` global que migra no lugar: medir primeiro com N legadas e corrigir onde duplicar.
**Acceptance criteria:**
- [x] AC3: cada célula da matriz §C tem um teste que reprova em `origin/main` e passa no novo
- [x] AC6 para Claude, Codex e Gemini
- [x] AC4, sentido sítios (herdado da auditoria do ML-1A): um teste orientado por tabela itera cada forma de
  `guardforms.*Legacy*` × cada sítio de projeto e global × cada par que o sítio emite, e exige uma entrada atual por par.
  Falsificar acrescentando uma forma fictícia ao registro, e o teste tem de reprovar no primeiro sítio
- [x] AC2, formas inline (herdado): `guardExpectedLine`, `guardD11LegacyLine` e `guardD11RevisedLegacyLine` derivam do
  registro, ou um teste afirma igualdade com `guardforms.*` para os dois guards e as duas famílias
- [x] Os goldens e os configs versionados do repositório (`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`), se mudarem, mudam só por deduplicação
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ ./internal/commands/ -count=1
```

**Auditoria do arquiteto (2026-10-23): reprovado, com o corretivo no ML-1C.** Os gates passam (`go build` rc=0; 4 pacotes
ok; 27 `--- PASS` em `-run 'TestNested|TestDedup_'`). Faltam:
- o contra-braço em `origin/main`, que nenhum teste mediu;
- os testes do Windsurf, que não existem;
- o harness global: só os dois alvos do Claude foram tocados, e Codex, Gemini, Cursor, Copilot e Windsurf não foram medidos;
- o AC4 completo: ele cobre só Claude × `claudeShellMatcher`, sem falsificação;
- o AC2: não foi feito. O "garantido pela construção" é falso, porque o `guardExpectedLine` tem template próprio.

### ML-1C — Corretivo do ML-1B
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/update.go`, `internal/guardforms/guard_generations.go`, `internal/generators/nested_guard_migrate_dedup_test.go`, `internal/validator/guard_ac2_lines_ml1c_test.go` (fechado no #604)
**Acceptance criteria:**
- [x] Contra-braço medido em `origin/main` (via comentários de reconciliação em cada teste): 14 testes ML-1C e 13 testes AC2 reprovam em origin/main; 5 testes AC5/D14 passam nos dois lados.
- [x] Windsurf: credential e git-branch, N legadas → 1 por par; AC5; D14 com o global de credential — `TestWindsurf_ML1C_*` (5 testes verdes)
- [x] Amazon Q: git-branch com as formas PS/POSIX, N legadas → 1 por par — `TestAmazonQ_ML1C_GitBranch_PSPOSIXForms_Migrate` verde
- [x] Harness global: 9 testes de N legadas para Codex/Gemini/Cursor/Copilot/Windsurf × credential e git-branch — todos verdes; código corrigido em `harnessCredentialGuardTargetCodex`, `harnessGitBranchGuardTargetCodex`, `harnessCredentialGuardTargetGemini`, `harnessGitBranchGuardTargetGemini`, `harnessCredentialGuardTargetCursor`, `harnessGitBranchGuardTargetCursor`, `harnessCredentialGuardTargetCopilot`, `harnessGitBranchGuardTargetCopilot`, `harnessCredentialGuardTargetWindsurf`
- [x] AC4: todos os sítios referenciam `guardforms.*LegacyPSPOSIX` via append; adicionar forma fictícia em index 0 falharia nos testes correspondentes
- [x] AC2: 13 testes de igualdade byte-a-byte entre `guardExpectedLine`, `guardD11RevisedLegacyLine`, `guardD11LegacyLine`, `guardD2LegacyLine` e as constantes de `guardforms` — `TestAC2_*` todos verdes

**Gates:**
- `go build ./...` rc=0
- `go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ ./internal/commands/ -count=1` → ok (4 pacotes)
- `trackfw validate` exit=0 (163 warnings, 0 violations)

**Auditoria do arquiteto (2026-10-23):** aprovado com ressalva. Contra-braço medido pelo arquiteto: `origin/main` via
`git archive` + `internal/guardforms` + os 3 arquivos de teste novos de `internal/generators/`. Resultado:
- **27 testes reprovam lá:** todos os `*_NLegacies*`, `*D14*`, `*AC4*`, Windsurf e Harness ML1C;
- **passam lá, como esperado:** idempotência, AC5 aninhado e os `TestDedup_*Skips*` pré-existentes;
- **passa lá, e não deveria:** `TestNestedMigrate_AmazonQ_NLegacies_Deduplicates`. Ele só conta a forma atual, e o código
  antigo acrescenta uma entrada e deixa as legadas, o que também dá 1.

O AC2 está fechado por teste de igualdade (`TestAC2_*`, 13 subtestes). O AC4 orientado por tabela não foi entregue: o
"AC4 note" é argumento, não teste. As duas lacunas passam para o ML-1D.

### ML-1D — Corretivo do ML-1C
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/guard_registry_sites_ac4_test.go`, `internal/generators/nested_guard_migrate_dedup_test.go` (fechado no #604)
**Acceptance criteria:**
- [x] `TestNestedMigrate_AmazonQ_NLegacies_Deduplicates` também exige zero legadas no par, e reprova em `origin/main`
- [x] AC4: um teste orientado por tabela sobre (sítio, evento, matcher, guard) para todo sítio de projeto e global, iterando
  cada forma do registro; a falsificação, com uma forma fictícia acrescentada ao registro, reprova e a saída é relatada

**Auditoria do arquiteto (2026-10-23):** aprovado. O `TestAC4_RegistrySites` (23 linhas de tabela) passa na branch e tem
169 FAILs em `origin/main`. O teste do Amazon Q agora também exige zero legadas, e reprova em `origin/main`. Sobre a
falsificação pedida: acrescentar uma forma fictícia ao registro não pode reprovar, porque os sítios consomem a mesma
variável. O desenho do AC4 estava errado, e o erro é do arquiteto. A propriedade que importa, "todo sítio trata todo o
registro", fica provada pelo contra-braço, em que os sítios não usam o registro. Fica um defeito conhecido, que entra
agora: o credential guard do Amazon Q não migra `CredentialLegacyPSPOSIX`, enquanto o git-branch já migra (ML-1C).

### ML-1E — Amazon Q: credential com as formas PS/POSIX
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/guard_registry_sites_ac4_test.go` (fechado no #604)
**Acceptance criteria:**
- [x] `InjectAmazonQHooks` migra também `guardforms.CredentialLegacyPSPOSIX` para a forma atual de cmd.exe, nos pares de credential que ele emite
- [x] A linha do Amazon Q de credential no `TestAC4_RegistrySites` passa a iterar `CredentialLegacyPSPOSIX`, e o comentário de lacuna sai
- [x] Contra-braço: a linha nova reprova no HEAD anterior a este ML


## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-23-revisao-geracoes-do-guard.md` (fechado no #604)
**Acceptance criteria:**
- [x] Parecer: REPROVADO com B1 (literal do Gemini) e R1–R5 — `docs/qualidade/2026-10-23-revisao-geracoes-do-guard.md`; corretivo no ML-2C

### ML-2B — Red-team da remoção
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-redteam-geracoes-do-guard.md` (fechado no #604)
**Acceptance criteria:**
- [x] Parecer: 15 ataques, 14 PASS; B1 (Copilot: `view` e `create|edit` somem com o global instalado) — `docs/seguranca/2026-10-23-redteam-geracoes-do-guard.md`; corretivo no ML-2C. Nenhum hook do usuário removido

### ML-2C — Corretivo das revisões
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/copilot_kiro_d14_ml2c_test.go`, `internal/generators/credential_guard_dedup_test.go`, `internal/generators/cursor_guard_migrate_dedup_test.go`, `internal/generators/guard_harness_dedup_test.go` (fechado no #604)
**Por que existe:** o Hefesto achou o B1, em que o `InjectGeminiHooks` passa o literal `"scripts/trackfw-credential-guard.sh"`
em 6 chamadas. O Hades achou o B1 do Copilot, e o arquiteto mediu o mesmo defeito no Kiro: o `InjectKiroHooks` deixa de
emitir `read` e `write` com o global instalado, e o global do Kiro só cobre `shell`. Copilot e Kiro ficaram fora da matriz
da Wave 0 porque são sobrescrita total, mas a D14 por par vale para eles também. A auditoria do arquiteto não conferiu isso.
**Acceptance criteria:**
- [x] Gemini: as 6 chamadas com literal saem (a deduplicação pelo registro já cobre a forma) ou passam a usar o alias; grep pelo literal em `internal/generators/*.go` fora de testes devolve só `internal/guardforms`
- [x] Copilot: `view` e `create|edit` emitidos com ou sem global; `bash` pulado com o global. Teste com os dois braços, que reprova no HEAD anterior
- [x] Kiro: `read` e `write` emitidos com ou sem global; `shell` pulado com o global. Teste com os dois braços, que reprova no HEAD anterior
- [x] R1: `migrateDedupGuardInFlatArray` delega para `dedupFlatGuardEntries`, sem duplicação dos passes 1–2
- [x] R2: testes `*_ML1C_*` renomeados por comportamento
- [x] R3: `TestNestedMigrate_Claude_AC4_EveryRegistryFormMigrates` removido, porque o `TestAC4_RegistrySites` já cobre o caso
- [x] R4: `append` sobre slice do registro sempre com cópia (`append(append([]string{}, a...), b...)`)
- [x] R5 (Hefesto), aceito como resíduo: os marcadores de substring do validador servem a outro propósito (detecção por `Contains`) e não são gerações
- [x] Resíduos R1–R4 do Hades aceitos, conforme o parecer
**Gates evidence:**
```
go build ./...                                           rc=0
go test ./internal/generators/ ./internal/validators/ \
  ./internal/pathguard/ ./internal/commands/ -count=1
ok  github.com/kgsaran/trackfw/internal/generators    60.041s
ok  github.com/kgsaran/trackfw/internal/validator     32.473s
ok  github.com/kgsaran/trackfw/internal/pathguard      0.876s
ok  github.com/kgsaran/trackfw/internal/commands      46.648s
```
**Contra-braço:** `TestCopilotD14_ViewCreateEditAlwaysPresent/with-global` e
`TestKiroD14_ReadWriteAlwaysPresent/with-global` FAIL no HEAD anterior (pré-ML-2C),
confirmando que os novos testes reprovam a condição correta.


## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality e CI
**Status:** ✅ Concluído
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição (fechado no #604)
**Acceptance criteria:**
- [x] `make quality` rc=0 com a máquina ociosa e o HOME isolado (falsificação 368 OK / 0 FAIL; FAIL do log são só braços de autoteste)
- [x] CI do PR verde; PR com `Closes #599` — #604, 20/20 no 39d0ac26; #599 fechada


---

## Reabertura (2026-10-24, issue #613): a fiação de projeto não depende do guard global

**Por que o escopo original não previa:** o AC6 deste roadmap pedia a remoção, e ela foi entregue e testada. O defeito
está na própria decisão (D14 e decisão 4 do ADR-2026-08-06): ela grava estado da máquina num arquivo versionado. Nenhum
teste cruzou `update` com global e depois `validate`, e é a mesma falha de enumeração da Wave 1, porque a matriz cruzou
só a dimensão do defeito original.

**Medido:**
- ea-cmdb: 4 `guard_wiring_removed` (#613).
- Este repositório, PR #612: 9 `guard_wiring_removed`, `trackfw validate` rc=1 e CI vermelho em `governance-go-install` e
  `governance-install-script`.

**Decisão:** a D15 do ADR-2026-10-04.

## Wave 4 — Threat model e enumeração de sítios da D15
> Dependencies: nenhuma. Gate: `trackfw barrier <roadmap> --wave 4` e auditoria do arquiteto antes da Wave 5.

### ML-4A — Threat model da D15 e matriz de sítios
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-24-wave0-guard-global-versionado.md` (novo; o único arquivo que o ML escreve)
**Acceptance criteria:**
- [x] **Matriz por mecanismo.** Lista todo sítio que lê estado global para decidir sobre a fiação de **projeto**, com
  arquivo:linha:
  - os `globalCredentialGuardInstalled*` e `globalGitBranchGuardInstalled*` de `internal/generators/agentfiles.go`, e
    quem os chama (`Inject*Hooks`, `update.go`, `internal/guardforms`);
  - no validador, todo `globalInstalled:` e toda leitura de `$HOME` que aceite ausência de fiação de projeto
    (`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`, `validator_guard_wiring.go`).
  Para cada sítio, diz se ele **pula**, **remove** ou **aceita ausência**. Separa os que só tratam o escopo **global**
  (`update harness`, `validateGuardGlobalHookResolvable`), que ficam como estão.
- [x] **Testes que afirmam o contrato antigo,** com nome e arquivo. Eles vão inverter por contrato e não são
  pré-existentes. Inclui no mínimo os braços D14 do #604, `copilot_kiro_d14_ml2c_test.go`, `credential_guard_dedup_test.go`,
  `git_branch_guard_dedup_test.go`, `guard_harness_dedup_test.go` e o ML-1D da REQ-2026-10-06 (Windsurf).
- [x] **Idempotência do segundo disparo,** com o global e o projeto rodando no mesmo evento. Mede o efeito colateral de
  `credential` em `PreToolUse` e `PostToolUse` no modo `warn` (arquivo de atenção, log) e no modo `block`, e o de
  `git-branch`. Diz se a ordem e o paralelismo dos hooks do Claude Code mudam o veredito.
- [x] **Ameaças:** fail-open em algum caminho novo, um repositório limpo que passa a ter violation (aperto do validador →
  efeito em consumidores e na decisão de versão), e o `init` em máquina sem global.
- [x] Recomendação de `Files affected` exatos para o ML-5A.
**Auditoria do arquiteto (2026-10-24):** aprovado. Parecer em `docs/seguranca/2026-10-24-wave0-guard-global-versionado.md`.
- **Matriz:** 15 sítios de projeto (G1–G15) em `agentfiles.go` e 1 sítio do validador que aceita ausência (Windsurf,
  `validator_credential_guard.go:748`). Conferi por grep:
  - os 4 helpers só têm chamadores dentro dos `global*Installed*`;
  - as demais leituras de HOME no validador são de escopo global e só emitem aviso.
- **Idempotência:** a do segundo disparo foi derivada do código (`os.WriteFile` sobrescreve), e não executada. Aceito,
  porque o veredito vem do rc e não existe emissão de `allow`.
- **Versão:** o aperto do validador só muda o veredito **local** do Windsurf numa máquina com o global. O CI já reprovava
  esse estado, então nenhum repositório verde no CI fica vermelho. Recomendação: patch (11.0.1). A decisão é do KG na tag.
- **Divisão:** um ML só (5A), porque o AC9 cruza o gerador e o validador.

### ML-4B — Governança da reabertura (arquiteto)
**Status:** ✅ Concluído
**Agente:** zeus-tf
**Files affected:** `docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`, `docs/roadmaps/done/ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md`
**Por que existe:** o primeiro `trackfw barrier --wave 4` saiu `blocked` no `ml_scope` com estes dois arquivos.
- O ADR-2026-08-06 recebeu a marca de "substituída pela D15" na decisão 4, e o conjunto de orquestração só cobre o ADR
  ligado à REQ. A declaração aqui está correta.
- O caminho antigo do roadmap em `done/` aparece porque o `trackfw roadmap move` o apagou, e o conjunto de orquestração só
  conhece o caminho atual. Isso é defeito do produto, e o ML-5B corrige. A declaração aqui é o contorno até lá.
**Acceptance criteria:**
- [x] ADR-2026-08-06, decisão 4, marcada como substituída pela D15
- [x] Roadmap reaberto em `wip/`, com a REQ sincronizada pelo `trackfw roadmap move`

## Wave 5 — Implementação da D15
> Dependencies: Wave 4 auditada.

### ML-5A — Geradores e validador sem consulta ao global para a fiação de projeto
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/validator/validator_credential_guard.go`, `internal/generators/credential_guard_dedup_test.go`, `internal/generators/git_branch_guard_dedup_test.go`, `internal/generators/copilot_kiro_d14_ml2c_test.go`, `internal/generators/guard_harness_dedup_test.go`, `internal/generators/credential_guard_windsurf_amazonq_ml1b_test.go`, `internal/validator/validator_credential_guard_test.go`, `internal/commands/update_validate_global_guard_test.go` (novo), `docs/cli-parity.md`, `internal/generators/nested_guard_migrate_dedup_test.go`, `internal/generators/cursor_guard_migrate_dedup_test.go`, `internal/validator/export_test.go`, `internal/validator/validator_git_branch_guard.go` (os quatro últimos foram acrescentados na auditoria: testes que quebraram por contrato, o export de teste do validador e um comentário desatualizado)
**Acceptance criteria:**
- [x] AC6 revisto: todo sítio da matriz da Wave 4 que pula ou remove deixa de fazê-lo. Com e sem global, a fiação de
  projeto é emitida e migrada por par.
- [x] O validador não aceita mais ausência de fiação de projeto por causa do global.
- [x] AC9: teste de cruzamento por CLI (HOME falso com global, fixture git com `origin/main`, `init`/`update` → `validate`
  rc=0), com contra-braço que reprova em `origin/main`.
- [x] Os testes do contrato antigo passam a afirmar a D15. Cada teste novo ou invertido declara a conclusão que afirma
  (Regra de Reconciliação).
**Auditoria do arquiteto (2026-10-24):** aprovado depois do corretivo ML-5A2.
- **Primeira entrega, reprovada:** o AC9 cobria só o Windsurf, lia o arquivo em vez de rodar o `validate` e não tinha
  `origin/main`, então não exercitava o `guard_wiring_removed`.
- **ML-5A2:** `TestD15_UpdatePreservesProjectGuardWithGlobalInstalled` roda o binário compilado sobre os 7 CLIs
  (`init`, depois push para um `origin` bare, `update harness`, `update` e `validate --json`).
  `TestD15_GlobalOnlyDoesNotSatisfyProjectScope` cobre o lado do validador.
- **Medido pelo arquiteto:**
  - no HEAD, os dois testes passam (rc=0);
  - em `origin/main` (ec8f6c8d, via `git archive`), os 7 subtestes e o braço do validador reprovam (rc=1), com 33 linhas
    de `guard_wiring_removed` nos 7 arquivos de hook. É o defeito da #613.
- **Gerador:** os G1–G15 foram removidos, e o grep pelas funções mortas fora de comentários está vazio.
- **Validador:** o short-circuit do Windsurf foi removido.
- **Gates do executor:** `go build ./...` rc=0 e os 5 pacotes `ok`. O `trackfw validate` no repositório deu rc=0.

### ML-5B — `ml_scope` aceita o roadmap em qualquer estado do `roadmap_dir`
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Dependencies:** ML-5A, porque os dois compilam e testam o pacote `internal/commands`.
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Por que existe:** `buildMLScopeOrchestrationSet` (`internal/commands/barrier.go:899`) só inclui `roadmapRelPath`, que é o
caminho atual. Um roadmap reaberto (`done/` para `wip/`, procedimento da Regra Dura de Causa Raiz) ou movido de estado na
branch faz o `git diff` contra `origin/main` listar a remoção do caminho antigo, e o `ml_scope` sai `blocked`. Medido no
ML-4B. É outra causa, do `ml_scope` (REQ-2026-10-23 das regras de escrita, #606), e entra neste PR porque bloqueia o
barrier desta REQ e é defeito conhecido.
**Acceptance criteria:**
- [x] O conjunto de orquestração inclui o mesmo basename do roadmap em todo estado (`backlog`, `analyzing`, `wip`,
  `blocked`, `done`, `abandoned`) do `roadmap_dir`, nos dois modos de namespacing (`flat` e `by_agent`)
- [x] Um roadmap **diferente** com nome parecido continua fora do escopo (contra-braço por basename exato)
- [x] Teste com repositório git real em `t.TempDir()`: roadmap em `done/` no `origin/main`, movido para `wip/` na branch,
  barrier sem `out of scope` para o caminho antigo. O teste reprova no HEAD anterior a este ML.
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'MLScope' -count=1 -v
```
**Auditoria do arquiteto (2026-10-24):** aprovado.
- **Correção:** `buildMLScopeOrchestrationSet` acrescenta `<pai>/<estado>/<basename>` para os outros 5 estados, e só
  quando o diretório atual é um estado conhecido. No `by_agent`, o pai é a pasta do agente.
- **Testes novos:**
  - `TestMLScope_RoadmapMovedStatePasses_Flat`, com contra-braço por basename diferente;
  - `TestMLScope_RoadmapMovedStatePasses_ByAgent`, com contra-braço pelo mesmo basename em outro agente.
- **Medido pelo arquiteto:** no HEAD, os 13 testes `MLScope` passam (rc=0). No commit anterior (6b92eaeb, via
  `git archive`), os 2 testes novos reprovam (rc=1).
- **Ressalva para o ML-6A:** o executor criou a lista `roadmapAllStates`, em vez de reaproveitar uma existente. A mesma
  lista literal já existe em 9 sítios de 4 pacotes, sem constante compartilhada.

## Wave 6 — Revisões independentes
> Dependencies: Wave 5 auditada.

### ML-6A — Revisão de qualidade
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-24-revisao-guard-global-versionado.md`
**Acceptance criteria:**
- [x] Parecer REPROVADO: B1–B6 (nomes e comentários de teste afirmam a D14, enquanto o corpo afirma a D15) e R1–R9.
  Corretivo no ML-6C; o R4 vira o ML-6D.

### ML-6B — Red-team da D15
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-24-redteam-guard-global-versionado.md`, `.claude/agent-memory/hades-tf/feedback_measure_the_real_code_path_not_an_isolated_proxy.md`
**Acceptance criteria:**
- [x] Parecer APROVADO, sem bloqueantes. Resíduos: R-1 (lista de estados sem constante, que vira o ML-6D) e R-2/R-3
  (expansão por basename do ML-5B, aceitos).
- [x] A sabotagem fiel (`git diff origin/main..HEAD -- internal/generators/agentfiles.go | patch -R`) faz o AC9 reprovar
  nos 7 CLIs.

**Auditoria do arquiteto (2026-10-24):** parecer aceito, com um incidente.
- **O incidente:** o hades-tf montou o caminho de um diretório temporário com `ls -d`, que tem alias com ícone. O caminho
  virou relativo, e o `trackfw init`/`update` rodou dentro do repositório. O resultado foram 12 linhas acrescentadas ao
  `trackfw.yaml` e um diretório `<ícone> /private/...` com dois `settings.json`. Isso não estava no relatório.
- **A correção:** o arquiteto removeu as 12 linhas e o diretório antes do commit.
- **Efeito no parecer:** a medição de idempotência que usava esse caminho não entrou nele. O Ataque 3 se apoia nos
  testes de idempotência existentes, que cobrem o caminho "sem global", hoje o único caminho.

### ML-6C — Corretivo do ML-6A (nomes, comentários e documentação)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Files affected:** `internal/generators/nested_guard_migrate_dedup_test.go`, `internal/generators/git_branch_guard_dedup_test.go`, `internal/generators/copilot_kiro_d14_ml2c_test.go`, `internal/generators/cursor_guard_migrate_dedup_test.go`, `internal/generators/credential_guard_dedup_test.go`, `internal/generators/agentfiles.go`, `internal/validator/validator_git_branch_guard.go`, `internal/commands/update_validate_global_guard_test.go`, `docs/cli-parity.md`, `internal/generators/guard_generations.go` (só comentário, com o nome de teste renomeado)
**Acceptance criteria:**
- [x] B1–B6: nome e comentário de cada teste dizem o que o corpo afirma (D15)
- [x] R1, R2, R3, R5, R7, R8 e R9 do parecer `docs/qualidade/2026-10-24-revisao-guard-global-versionado.md` corrigidos
- [x] R6: o braço do validador do AC9 exige a violation específica de presença do credential guard, e não só rc≠0
- [x] Nenhuma mudança de lógica de produto. `git diff` em `agentfiles.go` e `validator_git_branch_guard.go` toca só comentários
**Auditoria do arquiteto (2026-10-24):** aprovado. Em `agentfiles.go`, `validator_git_branch_guard.go` e
`guard_generations.go`, o diff toca só comentários: medido por grep das linhas `[-+]` que não são comentário, com resultado
vazio nos três.

### ML-6D — Uma lista canônica de estados de roadmap (R4 do ML-6A, R-1 do ML-6B)
**Status:** ✅ Concluído
**Agente:** apolo-tf
**Dependencies:** ML-6C, pelo pacote `internal/generators`.
**Files affected:** `internal/config/config.go`, `internal/config/roadmap_states_test.go` (novo), `internal/commands/barrier.go`, `internal/discover/discover.go`, `internal/discover/discover_test.go`, `internal/generators/context.go`, `internal/generators/roadmap.go`, `internal/serve/api_board.go`, `internal/validator/validator.go`, `internal/validator/validator_roadmap_gates.go`, `internal/validator/validator_traceid.go`, `internal/generators/context_test.go`, `internal/generators/roadmap_order_states_test.go` (novo), `internal/validator/roadmap_order_states_test.go` (novo), `internal/serve/api_board_test.go`
**Por que existe:** a lista literal dos 6 estados está copiada à mão em 16 sítios de 6 pacotes, e o ML-5B criou mais uma.
As cópias já divergem: `internal/discover/discover.go` (~L520–L543) não tem `analyzing`, e por isso o `discover` deixa de
contar os roadmaps em `analyzing/` e de detectar o modo `by_agent` por essa pasta. É a mesma causa da cópia do ML-5B:
uma lista sem fonte única.
**Acceptance criteria:**
- [x] Uma lista exportada única (proposta: `config.RoadmapStates`, se não criar ciclo de import) é consumida por todo
  sítio que precisa do **conjunto**
- [x] Sítios em que a **ordem** tem semântica (prioridade de busca, como `roadmapStateOrder` em `generators/roadmap.go`)
  mantêm a ordem própria, mas um teste afirma que o conjunto é igual ao da lista canônica
- [x] O `discover` conta `analyzing/` e detecta `by_agent` por ela. O teste reprova no HEAD anterior
- [x] O grep pela lista literal fora de `config` e de testes devolve só os sítios de ordem semântica, cada um com o teste
  de conjunto
**Auditoria do arquiteto (2026-10-24):** aprovado depois do corretivo ML-6D2.
- **Primeira entrega, reprovada em dois pontos:**
  - `config.RoadmapStates()` devolvia a ordem **alfabética**. Os 5 sítios de conjunto mudaram de ordem de iteração, e
    um deles é a ordem que o `inventoryBlock` mostra na saída.
  - `context.go` ganhou `analyzing` sem nenhum teste.
- **ML-6D2:**
  - a ordem canônica é a do ciclo de vida, e `TestRoadmapStates_ExactLifecycleOrder` a fixa;
  - `contextRoadmapStateOrder` tem teste de conjunto;
  - `TestGetContext_AnalyzingRoadmapAppearsInOutput` cobre o `trackfw context` (outro sítio da mesma causa: omitia
    `analyzing/`).
- **Medido pelo arquiteto, no commit anterior (via `git archive`):**
  - os testes `TestScan_Analyzing*` do discover reprovam (rc=1), e passam no HEAD;
  - `TestGetContext_AnalyzingRoadmapAppearsInOutput` reprova (rc=1).
- **Defeito conhecido, que vira o ML-6D3:** os testes de conjunto de `serve` e `validator` comparam o tamanho e só a
  direção var→canônica. Uma lista com um estado duplicado e outro ausente passa.

### ML-6D3 — Testes de conjunto bidirecionais
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `internal/serve/api_board_test.go`, `internal/validator/roadmap_order_states_test.go`
**Acceptance criteria:**
- [ ] `TestBoardStates_SetEqualsCanonical`, `TestRoadmapFolderStates_SetEqualsCanonical` e
  `TestTraceIdStateDirs_SetEqualsCanonical` checam as duas direções e rejeitam duplicata
- [ ] Falsificação: numa cópia, uma lista com `wip` duplicado e sem `analyzing` faz cada um reprovar

## Wave 7 — Fechamento
> Dependencies: Wave 6 aprovada.

### ML-7A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] `trackfw update` neste repositório, numa máquina com global, seguido de `trackfw validate; echo rc=$?` sem pipe: rc=0
- [ ] CI do PR verde; PR com `Closes #613`
