# Quality Review — Log com um arquivo por entrada (ML-3A)

> Date: 2026-10-22 | Agent: hefesto-tf | Scope: `git diff origin/main...HEAD -- internal/ scripts/ Makefile CLAUDE.md AGENTS.md GEMINI.md`

---

## 1. Ponto único: sobrou algum leitor que não passe por `translog.ReadAll`?

**Grep:** `grep -rn 'trackfw-log[^-]' internal/ --include='*.go' | grep -v '_test.go' | grep -v '//'`

Resultado relevante: `generators/roadmap.go:265` retorna `logPath()` (ver achado M1 abaixo). Todos os demais são comentários, literais no gitattributes gerado ou a constante `legacySuffix` dentro do próprio `translog`.

**Callers do log confirmados:**

| Arquivo | Caminho efetivo |
|---|---|
| `commands/log.go:27` | `translog.ReadAll` direto |
| `commands/metrics.go` | `metrics.ParseLog` → `translog.ReadAll` |
| `metrics/metrics.go:45` | `translog.ReadAll` direto |
| `serve/metrics_log.go:21` | `translog.ReadAll` direto |
| `serve/api_metrics.go` | `ParseLog` local → `translog.ReadAll` |
| `validator/validator.go:2882` | `translog.ReadAll` direto |
| `discover/discover.go:544` | `translog.HasLog` (presença, não leitura de entradas) |
| `commands/discover.go:246` | `translog.ReadAll` direto |

`scripts/` não tem leitor ativo do log. As referências em `scripts/testdata/` são corpus histórico pinado — documentos, não código. **Ponto único satisfeito.**

---

## 2. Vacuidade dos testes novos

### `TestTransitionLog_LegacyAppendConflicts` — VACUOUS (BLOQUEANTE)

**O teste não pode falhar.** O fluxo inteiro é:

```
merge falha (exit ≠ 0)  →  passa (! verifica nada)
merge ok + conflict markers  →  passa (! verifica nada)
merge ok sem markers  →  t.Skip
```

Não existe `t.Error` nem `t.Fatal` em nenhum caminho. O teste afirma ADR D3 ("o formato antigo conflita") mas não há nenhuma asserção que falhe se o git resolver limpamente. Verificado mutando o teste na cópia:

```bash
TZ=UTC go test ./internal/generators/ -run TestTransitionLog_LegacyAppendConflicts -v
# --- PASS: sempre, independentemente de conflito
```

O ADR D3 exige: "O braço contrário **exige** que o formato antigo, com append nas duas branches, conflite." Esta prova não existe.

### `TestTransitionLog_ParallelBranchesMergeWithoutConflict` — não vacuous

Usa `MoveRoadmap` real (não grava entradas à mão). `.gitattributes` desabilitado via `core.attributesFile=/dev/null`. `translog.ReadAll` confirma que as duas entradas sobrevivem ao merge. **Afirmação verificável.**

Ressalva menor: a verificação do "set equality" (AC7) checa só que `alphaFound && betaFound` — se houvesse uma terceira entrada espúria, passaria. Não é bloqueante mas não é "igualdade de conjunto" estrita.

### `TestEntriesDirNotIgnoredByCommonGitignorePatterns` — não vacuous

Usa `WriteEntry` real + `git check-ignore -v`. O contrário arm verifica que `.trackfw-log.d/x.entry` **é** ignorado por `*.d`. Controle presente.

### `TestAgentInstructionsUseWorkingContextDir` — afirmação parcialmente encoberta (LOW)

O loop de renders usa `continue` em qualquer `Render` error, sem contar quantos renders bem-sucedidos aconteceram. Se todos os renders falharem por motivo inesperado (ex.: mudança de API do catálogo), somente o subtest negativo roda e o teste passa verde. O arm negativo só verifica o predicado no texto antigo já renderizado — não protege os 12 × N renders positivos. Não é bloqueante isolado, mas combinado com o achado C1 abaixo reduz a cobertura real.

### Comentário fantasma (LOW)

`transition_log_entry_test.go` (cabeçalho, linhas 14-19) lista `TestTransitionLog_ByAgent_DotDNotNamespace` como um teste neste arquivo. A função não existe; a cobertura real fica em `validator_namespacing_test.go:TestValidate_ByAgent_TrackfwLogDirIsInfra`. O comentário promete uma função ausente.

---

## 3. Correção do `translog`

### C1 — Ordenação com fuso quando legacy + .entry coexistem (BLOQUEANTE)

**Prova:**

```bash
# probe: legacy line "2026-10-22 12:00 -0300" (= 15:00Z)
# .entry filename timestamp = 14:00Z (deve vir ANTES de 15:00Z)

TZ=America/Sao_Paulo go test ./internal/translog/ -run TestReadAll_TZOrderingProbe -v
# ts[0].Basename=ROADMAP-entry.md   ← correto

TZ=UTC go test ./internal/translog/ -run TestReadAll_TZOrderingProbe -v
# ts[0].Basename=ROADMAP-legacy.md  ← ERRADO — legacy aparece primeiro
# ORDER BROKEN: TZ=UTC: expected entry first (14:00Z before 15:00Z), got ROADMAP-legacy.md first
# --- FAIL
```

**Causa:** `parseLine` usa `time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)` e ignora o offset `-0300` da linha. Com `TZ=UTC`, a linha "12:00 -0300" é lida como 12:00 UTC em vez de 15:00 UTC. Como o sort key das entradas `.entry` vem do UTC do filename (14:00Z), a legacy entra antes da entry quando 12:00 UTC < 14:00 UTC. O ADR adendum diz explicitamente: "O leitor único usa o offset escrito na própria linha, que o formato atual já grava (-0300)." A implementação não faz isso.

**Impacto:** `trackfw log`, `trackfw metrics`, `trackfw serve` (metrics endpoint), e `trackfw validate` recebem `Transition.Timestamp` com timezone errado para linhas legadas quando o servidor/CI roda em UTC. A ordenação errada é observável via `trackfw log` imediatamente após um `trackfw roadmap move` num projeto com legacy log.

**Arquivo e linha:** `internal/translog/translog.go:88` — `parseLine`.

### C2 — `HasLog` usa `os.Stat` (segue symlinks), `ReadAll` usa `os.Lstat` (recusa) (LOW)

`HasLog:319` faz `os.Stat(dir + legacySuffix)` e `os.Stat(dPath)`. `ReadAll:151` faz `os.Lstat(dPath)`. Um symlink em lugar de `.trackfw-log-entries/` seria reportado como "tem log" por `HasLog` mas produziria erro em `ReadAll`. O `discover.go` usa `HasLog` para decidir se exibe um banner, então o banner poderia aparecer mesmo com uma configuração que `ReadAll` recusaria. Não é um path de segurança, mas é inconsistência de contrato.

### Escrita atômica, idempotência, erros por conteúdo diferente, recusa de symlink em WriteEntry, arquivos não-.entry

Todos corretos. Verificados pelos testes existentes em `translog_test.go`.

---

## 4. Manutenção

### M1 — `logPath()` em `roadmap.go:264` é função morta em produção (LOW)

Chamada por zero sítios de produção. Chamada por 2 arquivos de teste:
- `fail_closed_root_test.go:219`
- `mute_guards_speak_test.go:122`

Funções não exportadas são acessíveis em testes do mesmo pacote sem precisar viver no arquivo de produção. Esta função deveria estar em `roadmap_test.go` ou em um helper de teste.

### M2 — `.d/` como abreviação de `.trackfw-log-entries/` em comentários (LOW)

`translog.go:108,113,117,154,174,259`, `commands/discover.go:179,184,241`, `generators/roadmap.go:1183`, `generators/req.go:527`. `.d` foi o nome antigo renomeado pelo ML-1C exatamente por ser um padrão problemático de gitignore. A abreviação cria ambiguidade para leitores sem contexto do histórico.

### M3 — `EntriesDirSuffix[1:]` em 5 sítios de produção (LOW)

`validator.go:1976`, `commands/discover.go:177,218`, `generators/roadmap.go:1189`, `generators/req.go:533`. Uma mudança de formato da constante (ex.: remover o `/` inicial) quebraria os 5 sítios silenciosamente. Alternativa sem esse risco: exportar `EntriesDirName = ".trackfw-log-entries"` e derivar `EntriesDirSuffix = "/" + EntriesDirName`.

### M4 — `Transition` struct definida 3 vezes (LOW)

`translog.Transition`, `metrics.Transition`, `serve.Transition` — tipos idênticos, 3 definições. Cada leitor copia o slice via loop. Não é um bug, mas `serve.ParseLog` engole erros (retorna `[]Transition{}` silenciosamente) enquanto `metrics.ParseLog` os devolve — um symlink recusado dá zeros no board web e erro no CLI.

---

## 5. Instruções geradas

**`trackfw context` chama `recentWorkingContextEntries`:** `generators/context.go:259` confirma. A instrução "run `trackfw context`" nos 12 assets é válida.

**Assets:** todos os 12 (`internal/integrations/assets/agents/*.md`) têm `## Working context` idêntico apontando para `docs/agents-working-context/`. **OK.**

**`agentfiles.go:92-93`:** gera o mesmo texto. **OK.**

**`CLAUDE.md`, `AGENTS.md`, `GEMINI.md`:** consistentes. **OK.**

**Golden files** (architect.agent-directory, architect.subagent, backend.agent-directory, backend.codex-toml): refletem o novo texto. **OK.**

**`gitAttributesBlock` em `scaffold.go`:** o comentário gerado diz (linha 2067-2070):
> "forjas como GitHub ignoram merge=union definido em .gitattributes de repositório (medido em PR #595/#597); o atributo só é respeitado em merges locais via `git merge`."
ADR D1 satisfeito.

---

## 6. Resultado dos testes (evidência)

```
go test ./internal/generators/ ./internal/metrics/ ./internal/serve/ ./internal/validator/ \
  ./internal/discover/ ./internal/commands/ ./internal/translog/ -count=1

ok  github.com/kgsaran/trackfw/internal/generators  61.578s
ok  github.com/kgsaran/trackfw/internal/metrics      0.206s
ok  github.com/kgsaran/trackfw/internal/serve        0.323s
ok  github.com/kgsaran/trackfw/internal/validator   31.274s
ok  github.com/kgsaran/trackfw/internal/discover     0.537s
ok  github.com/kgsaran/trackfw/internal/commands    46.366s
ok  github.com/kgsaran/trackfw/internal/translog     0.112s

go vet: 0 issues
go build ./...: 0 errors
```

Nota: os testes acima passam porque o ambiente de CI roda com `TZ=America/Sao_Paulo` (ou com legacy e entry a menos de 1 minuto de distância no testbed existente). O bug C1 se manifesta com `TZ=UTC` medido diretamente:

```bash
TZ=UTC go test ./internal/translog/ -run TestReadAll_TZOrderingProbe -v
# --- FAIL (ver §3 acima)
```

---

## 7. Veredito: REPROVADO

Dois achados bloqueantes:

| # | Severidade | Achado |
|---|---|---|
| **B1** | **BLOQUEANTE** | `TestTransitionLog_LegacyAppendConflicts` é vacuous: não tem nenhum caminho de falha; ADR D3 ("o formato antigo conflita") não tem prova efetiva. `internal/generators/transition_log_entry_test.go:167-258`. |
| **B2** | **BLOQUEANTE** | `parseLine` ignora o offset `-0300` da linha legada e usa `time.Local`. Com `TZ=UTC`, uma linha legada "12:00 -0300" (= 15:00Z) ordena antes de uma `.entry` com filename 14:00Z. O ADR adendum promete "usa o offset escrito na linha". Probe confirma falha com `TZ=UTC`. `internal/translog/translog.go:88`. |

Achados não bloqueantes:

| # | Severidade | Local | Achado |
|---|---|---|---|
| M1 | LOW | `generators/roadmap.go:264` | `logPath()` é função morta em produção, chamada só por 2 test files. |
| M2 | LOW | `translog.go`, `discover.go`, `roadmap.go`, `req.go` (comentários) | `.d/` usado como abreviação para `.trackfw-log-entries/` em comentários de produção; nome foi o antigo. |
| M3 | LOW | 5 sítios | `EntriesDirSuffix[1:]` frágil: extraindo o nome da constante sem proteção de tipo. |
| M4 | LOW | `validator_test.go` (cabeçalho) | Comentário promete `TestTransitionLog_ByAgent_DotDNotNamespace` que não existe no arquivo. |
| M5 | LOW | `translog.go:319` vs `:151` | `HasLog` usa `os.Stat` (segue symlink); `ReadAll` usa `os.Lstat` (recusa symlink). Divergência de contrato. |
| M6 | LOW | 3 pacotes | `Transition` definida em `translog`, `metrics`, `serve`. `serve.ParseLog` engole erros silenciosamente. |
| M7 | LOW | `working_context_dir_test.go:84,90` | `continue` em erros de render sem contador de sucesso: se todos os renders falharem, o teste passa. |

### Ações requeridas antes do merge

**B1:** O teste precisa de uma asserção real. A forma mais simples: ao final do merge, verificar que o arquivo contém `<<<<<<<` e chamar `t.Fatal` caso não contenha — removendo o `t.Skip` que actualmente engole o caso de merge limpo.

**B2:** `parseLine` precisa ler o offset quando a linha o contiver. A regex `lineRe` atual não captura o offset — um grupo adicional capturando o sufixo da linha (e passando o offset para `time.FixedZone` quando presente) corrige o bug sem quebrar o parser de linhas sem offset. O fallback para `time.Local` continua válido quando o offset está ausente.

---

**Nota para working context (proibição de escrita direta per handoff):**
- hefesto-tf / INÍCIO / 2026-10-22T00:00:00Z — ML-3A qualidade REQ-2026-09-02
- hefesto-tf / FIM / 2026-10-22T00:00:00Z — ML-3A qualidade: REPROVADO, B1 (teste vacuous ADR D3) + B2 (TZ ordering bug)
