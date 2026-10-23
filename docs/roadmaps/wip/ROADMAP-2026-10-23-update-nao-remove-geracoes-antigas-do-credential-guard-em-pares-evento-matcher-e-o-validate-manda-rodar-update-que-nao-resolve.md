---
status: wip
date: 2026-10-23
req: "docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: update não remove gerações antigas do credential guard em pares evento×matcher

> Created: 2026-10-23 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
ADR: ADR-2026-10-04, adendo de 2026-10-23 (D12–D14). Issue: #599.

Causa (lida no código pelo arquiteto): `InjectCursorHooks` migra as formas legadas só em `beforeShellExecution` e
`afterShellExecution`. Os pares `preToolUse`/`postToolUse` × `Read`/`Write` recebem apenas `mergeCursorGuardMatcherEntry`,
que acrescenta a forma atual e deixa as legadas. A lista de gerações está copiada à mão em cada sítio, e as cópias
divergem entre os CLIs.

## Acceptance Criteria
- [ ] AC1–AC8 da REQ, cada um com a evidência registrada no ML que o fecha.
- [ ] `make quality` e o CI ficam verdes.

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
**Acceptance criteria:**
- [x] `InjectAmazonQHooks` migra também `guardforms.CredentialLegacyPSPOSIX` para a forma atual de cmd.exe, nos pares de credential que ele emite
- [x] A linha do Amazon Q de credential no `TestAC4_RegistrySites` passa a iterar `CredentialLegacyPSPOSIX`, e o comentário de lacuna sai
- [x] Contra-braço: a linha nova reprova no HEAD anterior a este ML


## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Acceptance criteria:**
- [x] Parecer: REPROVADO com B1 (literal do Gemini) e R1–R5 — `docs/qualidade/2026-10-23-revisao-geracoes-do-guard.md`; corretivo no ML-2C

### ML-2B — Red-team da remoção
**Status:** ✅ Concluído
**Agente:** hades-tf
**Acceptance criteria:**
- [x] Parecer: 15 ataques, 14 PASS; B1 (Copilot: `view` e `create|edit` somem com o global instalado) — `docs/seguranca/2026-10-23-redteam-geracoes-do-guard.md`; corretivo no ML-2C. Nenhum hook do usuário removido

### ML-2C — Corretivo das revisões
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Por que existe:** o Hefesto achou o B1, em que o `InjectGeminiHooks` passa o literal `"scripts/trackfw-credential-guard.sh"`
em 6 chamadas. O Hades achou o B1 do Copilot, e o arquiteto mediu o mesmo defeito no Kiro: o `InjectKiroHooks` deixa de
emitir `read` e `write` com o global instalado, e o global do Kiro só cobre `shell`. Copilot e Kiro ficaram fora da matriz
da Wave 0 porque são sobrescrita total, mas a D14 por par vale para eles também. A auditoria do arquiteto não conferiu isso.
**Acceptance criteria:**
- [ ] Gemini: as 6 chamadas com literal saem (a deduplicação pelo registro já cobre a forma) ou passam a usar o alias; grep pelo literal em `internal/generators/*.go` fora de testes devolve só `internal/guardforms`
- [ ] Copilot: `view` e `create|edit` emitidos com ou sem global; `bash` pulado com o global. Teste com os dois braços, que reprova no HEAD anterior
- [ ] Kiro: `read` e `write` emitidos com ou sem global; `shell` pulado com o global. Teste com os dois braços, que reprova no HEAD anterior
- [ ] R1: `migrateDedupGuardInFlatArray` delega para `dedupFlatGuardEntries`, sem duplicação dos passes 1–2
- [ ] R2: testes `*_ML1C_*` renomeados por comportamento
- [ ] R3: `TestNestedMigrate_Claude_AC4_EveryRegistryFormMigrates` removido, porque o `TestAC4_RegistrySites` já cobre o caso
- [ ] R4: `append` sobre slice do registro sempre com cópia (`append(append([]string{}, a...), b...)`)
- [ ] R5 (Hefesto), aceito como resíduo: os marcadores de substring do validador servem a outro propósito (detecção por `Contains`) e não são gerações
- [ ] Resíduos R1–R4 do Hades aceitos, conforme o parecer


## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde; PR com `Closes #599`
