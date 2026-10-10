# Parecer de qualidade — `guard_wiring_removed` (ML-2A Wave 2)

> Data: 2026-10-10 | Revisor: hefesto-tf | Branch: `fix/remover-a-entrada-pretooluse-nao-e-detectado`

REQ: `docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`
ADR: `docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`

Arquivos revisados:
- `internal/validator/validator_guard_wiring.go` (novo, 506 linhas)
- `internal/validator/validator_guard_wiring_test.go` (novo, 1207 linhas, 27 testes `TestGuardWiring*`)
- `internal/validator/validator_guard_binary_probe.go` (lido como referência de equivalência)
- `internal/generators/agentfiles.go` (lido como referência de `migrateHookCommand`)
- `internal/validator/validator_guard_hook_concordance_external_test.go` (lido)
- `docs/cli-parity.md` (seção `guard_wiring_removed`)

Metodologia:
- Todas as mutações foram executadas em cópia de scratch (`scratchpad/hefesto/`). O projeto permaneceu limpo.
- Comandos permitidos: `go build ./...`, `go vet ./internal/validator/`, `go test ./internal/validator/`.
- Medição de custo: baseline no binário da branch; overhead warm (~215 ms para 8 arquivos × 2 guards com repo local).

---

## 1. Cópia de verdade — divergência entre validador e gerador

**Achado:** ADR A2 afirma que a classe de equivalência do comando é "as mesmas que `migrateHookCommand` reconhece (`agentfiles.go:1491`)". A afirmação está incompleta.

`guardWiringTypeFromCmd` (`validator_guard_wiring.go`) reconhece 3 formas:
- D2-legacy: `trackfw guard credential; exit $LASTEXITCODE` (e variante PS)
- D11-legacy: forma sem o 4.º trecho de PS normalization
- D11-revised: forma canônica atual

`migrateHookCommand` em `agentfiles.go` reconhece adicionalmente:
- `legacyClaudeCredGuardCmd` = `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`
- `legacyBareCredGuardCmd` = `scripts/trackfw-credential-guard.sh`
- Equivalentes de git-branch

**Consequência medida:** se `origin/main` contiver uma das formas de script-path (instaladas por versões antigas do gerador antes da migração D11), o validador não as reconhece como fiação legítima. O resultado é silêncio (forma desconhecida → não é fiação → nenhuma tupla de origem extraída). `TestHefesto_ScriptPathOrigin_FalseNegative` (scratch) confirmou o comportamento: fixture com `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` em origin/main + arquivo ausente no disco → 0 violações.

**Evidência:** `agentfiles.go:1491` (definição de `migrateHookCommand`); `validator_guard_wiring.go` função `guardWiringTypeFromCmd`.

---

## 2. Leituras brutas e caminhos de falha de abertura

**Achado:** `guardWiringReadFromOrigin` (`validator_guard_wiring.go`) silencia falhas de leitura como "ausente".

Código observado:
```go
if lsErr != nil || len(strings.TrimSpace(string(lsOut))) == 0 {
    return nil, true, nil  // absent — SILENCIA erros git
}
```

O ramo `lsErr != nil` (falha do `git ls-tree`) devolve `(nil, true, nil)` — o chamador interpreta `absent=true` como "nunca instalado", fica em silêncio.

ADR D1 (explícito): "`origin/main` ilegível → **falha fechada**". O contrato documentado e a implementação divergem neste sub-caso.

Causas plausíveis cobertas indevidamente pelo silêncio: falha de rede ao buscar o remote, repositório corrompido, permissão negada no objeto pack, timeout de git. Em todos esses casos o validador não detecta remoção de fiação.

Caminho separado para ausência genuína (arquivo nunca existiu): `git show origin/main:<arquivo>` retorna exit ≠ 0 com stderr `"does not exist in"`. Este retorno também não está sendo distinguido de erros de infraestrutura — ambos chegam ao mesmo ramo `lsErr != nil`.

**Evidência:** `validator_guard_wiring.go` função `guardWiringReadFromOrigin`, corpo `if lsErr != nil`.

---

## 3. Duplicação com regras existentes

**Achado (menor):** `validateCredentialGuardPresenceRequired` (`validator_credential_guard.go:879`) cobre Windsurf e Amazon Q usando o próprio arquivo de disco como referência. A nova regra `guard_wiring_removed` usa `origin/main` como referência e cobre os mesmos dois arquivos + 6 outros.

Não há duplicação de lógica: as duas regras usam âncoras diferentes e detectam casos diferentes. `presenceRequired` falha se o arquivo não existe no disco; `guard_wiring_removed` falha se o arquivo existia em `origin/main` e sumiu do disco. Os dois podem disparar juntos ou separadamente.

**Conclusão:** sem duplicação funcional. Nenhum ajuste necessário.

---

## 4. Qualidade dos testes e mutações

Baseline: 27 testes `TestGuardWiring*`, todos PASS (`go test ./internal/validator/` em scratch limpo).

### Mutações executadas

**M1 — `guardWiringReadFromOrigin` devolve `(nil, true, nil)` sempre** (arquivo ausente em origin): 19/27 falham. Mordida adequada para os casos de fiação presente em origin.

**M2 — `tupleCovered` sempre `false`** (nenhuma tupla cobre): `TestGuardWiringRemoved_ArquivoIdentico_SemViolacao` e `TestGuardWiringRemoved_MigracaoLegitima_*` falham. Mordida adequada para equivalência de tuplas.

**M3 — `guardWiringTypeFromCmd` usa `Contains` em vez de igualdade de forma canônica**: testes de echo (`echo trackfw guard credential`) falham. Mordida adequada para a rejeição de formas não canônicas.

**M4 — `matcherCovers` sempre `true`**: testes de matcher estreitado (`Bash` não cobre `Bash|PowerShell`) falham. Mordida adequada.

**M5 — `guardWiringHasAnyFetchedOriginRefs` sempre `false`** (simula "0 refs buscados"): **todos os 27 testes passam**, incluindo `TestGuardWiringRemoved_RefIlegivel_FalhaFechada`.

O resultado de M5 revela que o ramo R4b (distinção entre "0 refs buscados → silêncio" e "refs presentes mas ilegíveis → falha fechada") não tem teste com mordida. `TestGuardWiringRemoved_RefIlegivel_FalhaFechada` não exercita o caminho que passa por `guardWiringHasAnyFetchedOriginRefs` — a falha fechada que o teste mede vem de outro caminho (ref indeterminado com origin presente).

**Achado adicional — Regra Dura de Reconciliação:** `TestGuardWiringRemoved_CategoriaApagada_Dispara` tem nome e comentário que dizem "apagar o evento PreToolUse inteiro", mas o fixture é `// Disk: hook file absent` (arquivo inteiro ausente, não apenas a chave `PreToolUse`). São casos distintos:
- Arquivo ausente no disco → regra dispara porque o arquivo existia em origin (D4).
- Arquivo presente no disco com a chave `PreToolUse` removida → regra deveria disparar (tupla em origin, tupla ausente no disco).

Não existe teste para o segundo caso. `TestHefesto_FilePresent_PreToolUseKeyDeleted_Dispara` (scratch) confirmou que o comportamento está correto — a regra dispara — mas o teste no suíte principal afirma o caso errado. Violação da Regra Dura de Reconciliação: o teste não sustenta a conclusão que o seu nome declara.

**Evidências:** saídas das mutações M1–M5 em scratch; `validator_guard_wiring_test.go` linhas de fixture do `TestGuardWiringRemoved_CategoriaApagada_Dispara`.

---

## 5. Custo — chamadas git e timing

**Modelo de custo:** a regra faz `git ls-tree` seguido de `git show origin/main:<arquivo>` por arquivo presente em origin, para cada um dos 8 arquivos de hook e os 2 guards. No pior caso (todos presentes): 16 chamadas `git show`.

**Medição:** baseline `trackfw validate` no binário da branch com repo local (warm): ~580 ms. Com os 8 arquivos de hook presentes e fiação intacta (sem violação): ~795 ms. Overhead incremental warm: ~215 ms.

**Comparação:** `git show` local em repo clonado é I/O de objeto pack — não há rede. O custo de 215 ms warm é aceitável para um comando de governança que roda em CI e em `pre-push`. Não é aceitável em `pre-commit` com repositório muito grande, mas a regra não está registrada em `pre-commit`.

**Conclusão:** custo dentro do orçamento declarado no ADR ("uma leitura `git show` por arquivo de hook existente, no mesmo carregamento do anchor"). Nenhum ajuste necessário.

---

## 6. Consistência de mensagens D1–D7 vs `docs/cli-parity.md`

**Achado A — `guardWiringDisableKeyMsg` hardcode "origin/main":**

```go
"this key was absent in origin/main; investigate before proceeding"
```

A função recebe o parâmetro `ref` mas não o usa na mensagem. Quando `deriveOriginDefaultBranch()` devolve `origin/master` ou outro ref, a mensagem cita `origin/main` incorretamente.

**Achado B — `guardWiringCheckCodexToml` hardcode "origin/main":**

```go
"this was absent in origin/main; investigate before proceeding"
```

Mesmo problema: `ref` recebido, não usado na string de mensagem.

**Achado C — comentário stale em `validator_guard_hook_concordance_external_test.go`:** o comentário `// deliberately no credential guard for Windsurf` refere-se ao estado anterior ao ML-1B, quando `InjectWindsurfHooks` não incluía `guardCredentialCmdPSPOSIX`. O ML-1B adicionou credencial ao Windsurf; o comentário não foi atualizado e induz leitura errada de cobertura intencional vs. cobertura acidental.

**Contrato `docs/cli-parity.md`:** a seção `guard_wiring_removed` declara: "NÃO cobre Cursor/Windsurf/Amazon Q com fixtures próprios (mesma lógica de extração, sem prova per-artefato)". A prova per-artefato existe para Windsurf: `TestHefesto_Windsurf_PopulationVacuity` (scratch) confirma que o extrator não é vacuoso — tuples extraídas corretamente, violação dispara quando arquivo ausente. A declaração de `cli-parity.md` ficou mais conservadora que o que foi medido; não é erro, mas é imprecisão declarada.

**Evidências:** `validator_guard_wiring.go` funções `guardWiringDisableKeyMsg` e `guardWiringCheckCodexToml`; `validator_guard_hook_concordance_external_test.go` comentário acima de `TestGuardHookConcordance_Windsurf_GitBranch`.

---

## Veredito

**APROVADO COM AJUSTES**

A lógica central (comparação de tuplas, equivalência, matcher coverage, D7, anchor 4-estados) está correta e tem mordida. Quatro itens precisam ser corrigidos antes de marcar o ML como concluído.

### Q1 — Classe de equivalência incompleta (ADR A2 x `migrateHookCommand`) — **não bloqueante**

`guardWiringTypeFromCmd` não reconhece as formas de script-path que `migrateHookCommand` reconhece. Falso negativo confirmado: origin com script-path + arquivo ausente no disco → silêncio. O ADR A2 afirma cobertura equivalente — a afirmação está errada.

Impacto: projetos que têm wiring de script-path em `origin/main` (instalado por versões pré-D11 do gerador e nunca migrado) não são protegidos. Não é regressão (nunca foi coberto), mas o contrato do ADR está incorreto. Correção: ampliar `guardWiringTypeFromCmd` com as formas `$CLAUDE_PROJECT_DIR/scripts/...` e `scripts/...`, ou corrigir o ADR A2 removendo a afirmação de equivalência com `migrateHookCommand`.

### Q2 — `guardWiringReadFromOrigin` silencia falha de `git ls-tree` — **bloqueante**

`lsErr != nil` devolve `(nil, true, nil)` (ausente/silêncio). ADR D1 exige falha fechada quando `origin/main` é ilegível. O código não distingue "arquivo nunca existiu" de "git falhou" — ambos chegam ao mesmo `return nil, true, nil`.

Correção mínima: separar o tratamento de `lsErr != nil` (falha de infraestrutura → falha fechada com mensagem) do tratamento de `len(out) == 0` (arquivo não rastreado → ausente → silêncio).

### Q3 — `guardWiringDisableKeyMsg` e `guardWiringCheckCodexToml` hardcode "origin/main" — **bloqueante**

Ambas as funções recebem `ref` como parâmetro e não o usam na mensagem de violação. Em repositórios cuja branch padrão é `master` ou outra, as mensagens citam `origin/main` incorretamente.

Correção: substituir o literal `"origin/main"` pelo valor de `ref` nas duas funções.

### Q4 — Duplication com `presenceRequired` — **não bloqueante**

Sem duplicação funcional. Documentado na seção 3. Nenhuma ação necessária.

### Q5 — Custo de chamadas git — **não bloqueante**

Dentro do orçamento do ADR (~215 ms warm overhead). Nenhuma ação necessária.

### Q6 — `TestGuardWiringRemoved_CategoriaApagada_Dispara` — Regra Dura de Reconciliação — **bloqueante**

Nome e comentário do teste afirmam "apagar o evento PreToolUse inteiro" mas o fixture testa arquivo inteiro ausente no disco. São casos distintos. O caso "arquivo presente, chave PreToolUse removida" não tem teste no suíte principal (comportamento correto confirmado em scratch, mas sem cobertura formal).

Correção: renomear o teste para refletir o que realmente testa (`ArquivoAusenteNoDisco_Dispara`) e adicionar um teste separado para `ArquivoPresenteChaveRemovidaDoDisco_Dispara`.

### Q7 — Comentário stale no teste de concordância — **não bloqueante**

`// deliberately no credential guard for Windsurf` está desatualizado (ML-1B adicionou). Correção: remover ou atualizar o comentário.

### Q8 — R4b (0 refs buscados → silêncio) sem teste com mordida — **bloqueante**

Mutação M5 (`guardWiringHasAnyFetchedOriginRefs` sempre `false`) não derruba nenhum dos 27 testes. O ramo que distingue "CI shallow sem fetch" (silêncio) de "refs presentes mas ref padrão ilegível" (falha fechada) está no produto mas sem cobertura verificável. Violação da Regra Dura de Reconciliação do ML: nenhum teste sustenta a conclusão "R4b: 0 refs → silêncio vs. refs presentes ilegíveis → falha fechada".

Correção: adicionar fixture que simule repo sem nenhum ref de `origin` buscado e verificar silêncio; verificar que o teste falha com M5 invertida (função sempre `true`).

---

### Resumo executivo

| # | Descrição | Severidade |
|---|---|---|
| Q1 | Formas script-path não reconhecidas (falso negativo) | não bloqueante |
| Q2 | `guardWiringReadFromOrigin` silencia `lsErr` como ausente | **bloqueante** |
| Q3 | Mensagens D7/Codex hardcode "origin/main" ignorando `ref` | **bloqueante** |
| Q4 | Duplicação com `presenceRequired` | não bloqueante (sem ação) |
| Q5 | Custo git calls | não bloqueante (dentro do orçamento) |
| Q6 | `CategoriaApagada` — Regra Dura de Reconciliação | **bloqueante** |
| Q7 | Comentário stale em concordância test | não bloqueante |
| Q8 | R4b sem mordida (M5 revela gap) | **bloqueante** |
