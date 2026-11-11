# Red Team — Gerador em Inglês (ML-4A)

> REQ: REQ-2026-11-02-idioma-do-produto-ingles-primeiro  
> Roadmap: ROADMAP-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues  
> Branch: feat/idioma-do-produto-ingles-primeiro-gerador  
> Data: 2026-11-10  
> Revisor: Hades (hades-tf)

---

## Veredito

**Rodada 1 (2026-11-10): REPROVA**

Encontrado um achado que reprova AC2 verbatim (F1 — MEDIUM). Os demais achados são residuais, por design ou informativos. O merge ficou bloqueado até um ML de correção ser adicionado a esta REQ e concluído (Regra Dura de Causa Raiz — mesma causa, mesma REQ).

**Rodada 2 (2026-11-11): APROVA COM RESSALVAS**

F1 e F4 fechados por ML-3E e ML-3F. F2 e F3 sem alteração (residuais declarados). Novo achado F5 LOW pré-existente em `origin/main` — não bloqueia merge; requer REQ separada.

---

## Escopo testado

Cinco superfícies conforme o handoff:

1. Gerador → leitor ponta-a-ponta: `trackfw roadmap new` gera roadmap EN → `barrier` reconhece `**Wave gates:**` → gate `exit 1` falha fechado.
2. Meta-geradores: `claudemd.go`, `agentfiles.go`, `scaffold.go` (modelo roadmap.md), `.claude/commands/trackfw/*.md`.
3. AC3 (`trackfw update`): preserva edições locais fora do bloco gerenciado; consumidores com PT roadmaps continuam passando após update.
4. AC2 / i18nguard: a lista de permissão de `pt_literals_test.go` está fechada? Encontrar literal PT emitido que escape.
5. CHANGELOG advisory (ADR D2).

---

## Método

Binário da branch compilado em `/private/tmp/.../hades-ml4a/trackfw` via `go build -o ... ./cmd/trackfw`. Binário de `origin/main` compilado em `/private/tmp/.../hades-ml4a/trackfw_main` via `git archive $SHA | tar xf - -C $MAINDIR && go build`.

Todos os repositórios de teste criados com `mktemp -d` fora da árvore do repositório real. Todos os `init` sobre consumidores com `HOME=$(mktemp -d)` para o teste de S3.

**Desvios de método a declarar:**

**Desvio 1 (consumer3, sessão anterior):** uma chamada a `cd "$TESTDIR3"` falhou — a causa exata (sufixo diferente da variável em relação ao diretório criado; possível efeito de alias eza em `$(ls -d …)`) não pôde ser determinada a partir da evidência disponível. Dois comandos rodaram acidentalmente no repositório real (`/Users/kgsaran/…/trackfw`):

- `trackfw validate` → RC=0, 174 avisos em LENIENT MODE. Nenhuma escrita no git index; `validate` não persiste estado em disco.
- `trackfw barrier ROADMAP-2026-01-01-auth --wave 0` → RC=2, roadmap não encontrado em `wip/` nem `done/`. Nenhum gate executado. Nenhuma escrita.

Verificado com `git status --short --ignored` antes e depois: nenhum arquivo novo em `!!` ou `M` além dos que já existiam.

**Desvio 2 (consumer5, esta sessão):** `$(ls -d …)` capturou `<U+F115> /private/tmp/…/consumer5.7TFxdq` (alias eza). `mkdir -p "$TESTDIR5/docs/adr"` executado com cwd no repositório real criou o caminho `<repo>/<U+F115> /private/tmp/.../consumer5.7TFxdq/docs/adr` dentro da árvore real. Um `git init -q` rodou dentro dessa estrutura (sem commits). O diretório foi removido com `rm -rf` antes do git status final.

**Desvio 3 (consumer5 — HOME):** `trackfw init` com binário main e `trackfw update` com binário branch rodaram sem `HOME` isolado. Verificação post-facto:

```
$ find ~/.claude -newermt '2026-11-10 21:00' -type f | grep 'agents\|skills/trackfw'
(nenhuma saída)
```

Nenhum arquivo `~/.claude/agents/trackfw-*.md` ou `~/.claude/skills/trackfw-*/SKILL.md` modificado hoje. O conteúdo editado manualmente de `trackfw-architect.md` (mapeamento `subagent_type`, linhas 43-45) está intacto.

Nenhum `git add`, `commit`, `push`, `checkout` ou `stash` foi executado na árvore real.

---

## Resultados por superfície

### S1 — Gerador → leitor ponta-a-ponta

**Resultado: PASS com contra-braço medido**

Fluxo executado em consumer6, via `trackfw roadmap new` real:

```
$ trackfw roadmap new --from-req docs/req/REQ-2026-11-10-s1.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-11-10-s1-test.md

# Edições mínimas: ML-0A ✅ Done, critérios [x], gate substituído por
# echo "wave0_ok" true
$ trackfw roadmap move ROADMAP-2026-11-10-s1-test wip

$ trackfw barrier ROADMAP-2026-11-10-s1-test --wave 0 --trust-local-gates
trackfw barrier — ROADMAP-2026-11-10-s1-test.md — wave 0
✓ wave_headings: passed
✓ mls_complete: passed
✓ acceptance_evidence: passed
✓ gates: passed
✓ validate: passed
~ ml_scope: skipped

result: passed
RC=0
```

**Contra-braço — `exit 1` falha fechado:**

```
$ trackfw barrier ROADMAP-2026-11-10-s1-test --wave 0 --trust-local-gates
# (exit 1 restaurado)
...
✗ gates: blocked
    - exit 1  # placeholder gate fails closed...: exit 1
✗ validate: blocked
    - 1 violations, 1 warnings

result: blocked
RC=1
```

Confirmações:
- `**Wave gates:**` reconhecido; gate executado; resultado em `gates:` reflete saída do gate.
- `exit 1` falha fechado enquanto não substituído.
- `roadmap new` gera `## Acceptance Criteria`, `## Status Legend`, `**Status:** ⬜ Pending` e `**Wave gates:**` todos em EN.

### S2 — D5: acervo PT preservado

**Resultado: PASS**

PT roadmaps com `**Gates da wave:**`, `✅ Concluído`, `## Critérios de Aceite` continuam passando. Confirmado por `go test ./internal/...` RC=0 e `scripts/check-marker-corpus-parity.sh` RC=0 (243 roadmaps do acervo).

### S3 — Meta-geradores (init com HOME isolado)

**Resultado: PASS**

Fresh `init` com binário branch e `HOME=$(mktemp -d)` em isoconsumer (`/private/tmp/.../isoconsumer.9AoS2F`):

```bash
# Scan de caracteres acentuados (excluindo nomes próprios de presets):
$ grep -rn --include='*.md' '[À-ÿ]' "$ISODIR" "$ISOHOME"
(nenhuma saída)

# Scan de palavras PT não acentuadas:
$ grep -rn "Obrigatório|Inspecione|Usar.*trackfw|Pendente|Concluído|Em andamento" ...
(nenhuma saída)
```

| Arquivo | Marcadores emitidos | Resultado |
|---|---|---|
| `claudemd.go` linha 39 | `Use /trackfw:architect` (EN), `Mandatory: Inspect…` (EN) | EN |
| `agentfiles.go` linhas 112-113 | `**Status:** ⬜ Pending`, `**Status:** 🔄 In progress`, `**Status:** ✅ Done` | EN |
| `scaffold.go` modelo roadmap.md | `**Status:** ⬜ Pending`, `**Wave gates:**` | EN |
| `.claude/commands/trackfw/*.md` (9 arquivos) | ciclo ML em EN | EN |

**F4 relacionado:** o modelo de instrução em `scaffold.go` para o comando `/trackfw:roadmap` omite o bloco `## Acceptance Criteria` no nível do roadmap. O gerador real (`roadmap.go`) inclui o bloco corretamente. A omissão é pré-existente em `origin/main` e não é regressão desta branch; contudo, agente seguindo o modelo do scaffold geraria roadmap que `validate` reprova ao mover para wip:

```
$ trackfw validate  # roadmap em wip sem ## Acceptance Criteria
✗ roadmap "ROADMAP-no-ac.md" is in wip but has no acceptance criteria block
Error: 1 violation(s) found
RC=1
```

Ver F4 abaixo.

### S4 — AC3: `trackfw update` preserva edições locais + PT roadmaps continuam passando

**Resultado: PASS com residual LOW declarado (ver F2)**

**Parte 1 — edições locais preservadas:**

```
# Edição local inserida FORA do bloco gerenciado (linha 7, antes de <!-- trackfw:rules:start -->):
#    <!-- AC3-LOCAL-EDIT: ... -->

# Depois de trackfw update com binário branch:
# Bloco gerenciado (linhas 142-143):
#   ANTES: "⬜ Pendente → 🔄 Em andamento", "✅ Concluído"
#   DEPOIS: "⬜ Pending → 🔄 In progress", "✅ Done"
# Edição local: linha 7 intacta após update.
```

**Parte 2 — PT roadmap continua passando após update:**

PT roadmap com `**Gates da wave:**`, `✅ Concluído`, `## Critérios de Aceite`, legend PT em consumer5 (pós-update com binário branch):

```
$ trackfw barrier ROADMAP-pt-test --wave 0 --trust-local-gates
...
✓ wave_headings: passed
✓ mls_complete: passed
✓ acceptance_evidence: passed
✓ gates: passed
✓ validate: passed

result: passed
RC=0
```

**Nuance (F2):** a seção EXTERNA do CLAUDE.md gerada pelo `claudemd.go` do main — fora do bloco gerenciado — retém PT em linhas 23 (`⬜ Pendente`), 26 (`Usar`), 27 (`Obrigatório`). Por design: `update` trata tudo fora dos sentinels como conteúdo do usuário, conforme a cláusula de AC3 "o contrato atual do `update` decide o que é sobrescrito."

### S5 — AC2 / i18nguard

**Resultado: REPROVA**

**AC2 verbatim:** "Mensagens do CLI em inglês (grupo 1), com teste que reprova o retorno do português nas mensagens emitidas."

**Achado — escape medido:**

```bash
$ LANG=pt_BR.UTF-8 trackfw validate   # consumer4, branch binary
⚠  write_scope_not_wired: ...
1 aviso(s)       ← PT

$ LANG=en_US.UTF-8 trackfw validate
⚠  write_scope_not_wired: ...
1 warning(s)     ← EN
```

**Mecanismo:** `internal/i18n/i18n.go` detecta `LANG`/`LC_ALL`/`LANGUAGE` e seleciona `pt-BR.json`. Os chamadores `i18n.T("validate.ok")`, `i18n.T("validate.violations")`, `i18n.T("validate.warnings")`, `i18n.T("metrics.no_data")` etc. retornam strings PT. O `pt-BR.json` tem 11 chaves com mensagens PT emitidas ao usuário.

**Teste existente não cobre:** `TestNoPTLiteralsInProductGo` testa literais string no código-fonte Go. A afirmação "sem literal PT no código" é correta — `validate.go` tem zero literais PT. O teste não afirma e não testa "sem saída PT em runtime com `LANG=pt_BR`." Isso é exatamente o que AC2 exige.

**Gap de enumeração do Wave 0:** a tabela de inventário de `docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md` não lista `validate.go`, `i18n/`, nem o canal `pt-BR.json → i18n.T()`. A enumeração contou PT literais no fonte; não mapeou o canal de lookup JSON. Declarado como gap retroativo da Wave 0.

**Por que "pre-existing" não resolve:** `validate.go` não foi modificado nesta REQ, mas `metrics.go` foi (ML-3B2). ML-3B2 traduziu hardcoded literals mas deixou `i18n.T("metrics.no_data")` emitindo PT. Mesmo as linhas não modificadas: AC2 é sobre o que o CLI emite, não sobre o que estava antes. Regra Dura de Causa Raiz: mesma causa (emissão PT pelo CLI via `pt-BR.json`), mesma REQ, mesmo PR.

**Decisão escrita no AC1 / Wave 0 sobre manter catálogos:** nenhuma. O Wave 0 não mapeou o canal. A negative scope "não cria seleção de idioma (`--lang`) nem i18n" proíbe adicionar novos catálogos — não protege manter a detecção automática. Ausência de decisão sobre catálogos + AC2 contradito = blocking.

**Direção do fix (novo ML nesta REQ):**

Opção B (definitiva): em `internal/i18n/i18n.go`, `DetectLocale()` retorna sempre `"en-US"`, ou os catálogos `pt-BR.json`/`es-ES.json` são removidos dos embeds. Esta opção não "cria i18n" — remove um comportamento que contradiz AC2.

Junto com B: adicionar `TestNoRuntimePTOutput` que executa o binário real (ou chama diretamente `i18n.T` com locale forçado) para `LANG`, `LC_ALL` e `LANGUAGE=pt_BR.UTF-8` e verifica que a saída é EN. Este teste é VERMELHO agora e VERDE após B — é o teste que AC2 exige.

### S5b — ptUnaccentedWords: escape por palavra PT não acentuada

Verificado: os 7 padrões em `ptUnaccentedWords` cobrem os casos nos arquivos `.go` de produto. Nenhum caminho de escape adicional encontrado além do mecanismo i18n (F1).

### S6 — CHANGELOG advisory (ADR D2)

**Resultado: CONFIRMADO PENDENTE**

O ADR `ADR-2026-11-08-…bilingue-parser…` declara: "O aviso vai no CHANGELOG da segunda tag." `CHANGELOG.md` atual tem `[11.6.0]` (primeira tag). O advisory deve ser incluído antes da segunda tag.

Texto sugerido:

```
**Aviso de migração:** roadmaps gerados por esta versão contêm `**Wave gates:**`
como cabeçalho de gates. Binários anteriores a v11.6.0 não reconhecem esse cabeçalho
e contam zero gates — o barrier passa em silêncio. Faça upgrade para v11.6.0 ou
superior antes de executar `trackfw barrier` em roadmaps gerados pela versão atual.
```

---

## Achados

### F1 — MEDIUM: i18n emite PT quando LANG=pt_BR.UTF-8; AC2 não coberto pelo teste

**Severidade:** MEDIUM — bloqueia merge (AC2 não satisfeito)  
**Novo ML exigido nesta REQ** (Regra Dura — mesma causa, mesmo PR)

Ver S5 acima. Direção: Opção B + `TestNoRuntimePTOutput`.

### F2 — LOW: seção externa do CLAUDE.md retém PT após `update` para consumidores pré-branch

**Severidade:** LOW — residual por design; não bloqueia merge  

Comportamento esperado: AC3 diz "o contrato atual do `update` decide o que é sobrescrito." A seção fora dos sentinels é preservada. ADR D1 aceita PT dos agentes que lerem o CLAUDE.md misto.

**Direção do fix (opcional, ML adicional nesta REQ):** `update` como segundo sítio de inject para a seção de regras de agente do CLAUDE.md.

### F3 — INFORMAÇÃO: CHANGELOG advisory ainda não escrito

**Severidade:** INFORMAÇÃO — entregável do arquiteto, não defeito de código  

Exigência do ADR D2. Deve aparecer no `CHANGELOG.md` antes da segunda tag.

### F4 — LOW: scaffold `/trackfw:roadmap` omite `## Acceptance Criteria` no nível do roadmap

**Severidade:** LOW — pré-existente em `origin/main`; não é regressão desta branch; causa diferente de F1  

O modelo de instrução em `scaffold.go` para `/trackfw:roadmap` não tem `## Acceptance Criteria` no nível do roadmap. O gerador real (`roadmap.go`) inclui o bloco. Se um agente seguir o modelo do scaffold ao escrever o roadmap à mão, `validate` vai reprovar ao mover para wip. Medido: `✗ roadmap is in wip but has no acceptance criteria block` (RC=1). Esta é uma omissão de documentação do scaffold, não do gerador. Causa diferente de F1 (PT vs. bloco ausente) → não entra no mesmo ML.

---

## Residual declarado

Este parecer aceita não cobrir os seguintes riscos:

1. **ADR D2 — janela do binário antigo:** binário anterior a v11.6.0 lendo roadmap gerado por esta branch conta zero gates em waves com `**Wave gates:**`. D2 reduz a janela, não a elimina para quem pular a primeira tag. Advisory no CHANGELOG (F3) é o controle residual.

2. **F2 — seção externa do CLAUDE.md:** consumidores existentes ficam com texto PT na seção de cabeçalho (fora do bloco gerenciado) após `update`. Por design; bilingual reader aceita PT.

3. **Ofuscação deliberada (herdado do Wave 0):** o parser protege contra acidente, não contra adversário. Homoglifos, VS selectors, HTML `<strong>` e entidades `&#58;` em cabeçalhos de gates são residual aceito pelo KG no modelo de ameaça do Wave 0.

---

## Suíte completa de testes

```
$ go test ./internal/...            RC=0
$ go build ./...                    RC=0
$ scripts/check-marker-corpus-parity.sh   RC=0 (243 roadmaps do acervo)
```

---

## Estado do repositório

```
$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short --ignored
 M .claude/agent-memory/hades-tf/MEMORY.md
?? .claude/agent-memory/hades-tf/feedback_ls_alias_glyph_creates_wrong_path.md
?? docs/seguranca/2026-11-08-redteam-gerador-ingles.md
!! .DS_Store
!! .agents/
!! .claude/settings.local.json
!! .pytest_cache/
!! bin/
!! build/
!! dist/
!! docs/.DS_Store
!! docs/roadmaps/.DS_Store
!! docs/roadmaps/.trackfw-branch-links.json
!! npm/.pytest_cache/
!! npm/node_modules/
!! pypi/.pytest_cache/
!! pypi/tests/
!! scripts/__pycache__/
!! scripts/windows-repro/python/__pycache__/
!! site/.vitepress/dist/

$ git diff --cached --stat
(vazio)
```

Arquivos novos/modificados nesta sessão:
- `docs/seguranca/2026-11-08-redteam-gerador-ingles.md` — este documento (output do ML-4A)
- `.claude/agent-memory/hades-tf/MEMORY.md` — índice de memória do agente (entrada adicionada)
- `.claude/agent-memory/hades-tf/feedback_ls_alias_glyph_creates_wrong_path.md` — nota de memória (new)

Os dois arquivos de memória são artefatos do agente sob `.claude/agent-memory/hades-tf/`, não código de produto. O handoff declarava apenas o parecer; o arquiteto decide sobre commit dos dois.

---

## Rodada 2 (2026-11-11)

### Veredito da Rodada 2

**APROVA COM RESSALVAS**

F1 e F4 fechados. Um novo achado LOW pré-existente (F5) não bloqueia o merge mas requer REQ separada. F2 e F3 sem alteração.

---

### O que foi verificado

#### V1 — F1: canal i18n emitindo PT com LANG=pt_BR.UTF-8

**Resultado: FECHADO**

Mecanismo corrigido em ML-3E: `DetectLocale()` hardcoded a `"en-US"` em `internal/i18n/i18n.go`; catálogos `pt-BR.json` e `es-ES.json` removidos do embed; apenas `//go:embed locales/en-US.json` permanece no produto.

Busca exaustiva por outros caminhos de emissão PT:

- `grep -rn 'Getenv.*LANG\|Getenv.*LC_\|Getenv.*LANGUAGE\|embed.*\.json\|embed.*locales'` em `internal/` e `cmd/` (excluindo `_test.go`): único resultado é `internal/i18n/i18n.go:10` — o embed correto `en-US.json`. O segundo resultado é um comentário dentro de testdata de corpus, não produto.
- Nenhum outro `//go:embed` de arquivo `.json` em código de produto.

Medição com binário compilado da branch (`go build ./cmd/trackfw`), projeto descartável em `mktemp -d`:

```
$ LANG=pt_BR.UTF-8 LC_ALL=pt_BR.UTF-8 LANGUAGE=pt_BR.UTF-8 trackfw validate
...
2 warning(s)   ← EN

$ LANG=en_US.UTF-8 trackfw validate
...
2 warning(s)   ← EN (idêntico ao anterior)
```

Comparação anterior (Rodada 1, antes do fix):

```
$ LANG=pt_BR.UTF-8 trackfw validate
...
1 aviso(s)   ← PT  (era o achado F1)
```

Comandos adicionais testados com PT locale: `validate --help`, `status`, `context`, `barrier --wave 0 --trust-local-gates` (projeto descartável) — todos emitem EN.

Testes unitários e de integração:

```
TestDetectLocale_AlwaysEN          PASS (8 sub-testes: LANG/LC_ALL/LANGUAGE × pt_BR/es_ES/C/"")
TestI18nT_AlwaysEN_UnderPTLocale   PASS (validate.ok, validate.warnings, validate.description)
TestI18nT_AlwaysEN_UnderESLocale   PASS
TestI18n_BinaryOutput_AlwaysEN     PASS (binário real: validate + validate --help com LANG=pt_BR)
TestI18nLocalesOnlyEN              PASS (só en-US.json em internal/i18n/locales/)
```

#### V2 — F4: scaffold /trackfw:roadmap omite `## Acceptance Criteria`

**Resultado: FECHADO**

ML-3F adicionou `## Acceptance Criteria` ao template de `/trackfw:roadmap` em `.claude/commands/trackfw/roadmap.md` (linha 32) e ao modelo de `internal/generators/scaffold.go`.

Verificação por leitura direta:

```
$ grep -n "Acceptance Criteria" .claude/commands/trackfw/roadmap.md
32:## Acceptance Criteria
```

Teste de reconciliação (`TestScaffold_RoadmapMD_TemplatePassesWIPAcceptanceCriteriaValidation`, `internal/generators/scaffold_test.go`):
- Extrai o template do fence `````markdown` em `roadmap.md`
- Promove `status: backlog` para `status: wip`
- Chama `validator.Validate()` num projeto descartável
- Verifica que nenhuma violação `"has no acceptance criteria block"` é emitida
- Resultado: **PASS**

#### V3 — Regressão S1–S4

**Resultado: SEM REGRESSÃO**

Suíte completa `go test ./internal/...`: todos os pacotes PASS (nenhum FAIL).

Paridade de corpus: `bash scripts/check-marker-corpus-parity.sh` → 243 roadmaps, `validate identical` (S2 — acervo PT preservado).

---

### Achado novo

#### F5 — LOW: literal PT hardcoded em `manager.go:512`, fora de `i18n.T()` e fora do alcance do guard

**Severidade:** LOW — não bloqueia merge  
**Pré-existente em `origin/main`** (mesma linha, mesmo conteúdo)

`internal/integrations/manager.go:512`:

```go
fmt.Fprintf(os.Stderr, "aviso: %q declara o mesmo name %q que %q; prosseguindo por --force\n", ...)
```

**Por que o guard não detecta:** `ptUnaccentedWords` cobre marcadores de contrato (`Pendente`, `Caducou`, `Gates da wave`, `Arquivos afetados`, etc.) e `ptAccentChars` cobre caracteres acentuados. `"aviso"` e `"prosseguindo"` não são palavras acentuadas nem estão na lista de `ptUnaccentedWords`. O `TestNoPTLiteralsInProductGo` PASSA com este literal presente — gap do guard.

**Trigger:** `trackfw integrations install --force` com colisão de nome — caminho raro.

**Por que não bloqueia:** pré-data esta REQ; a branch não introduziu nem modificou esta linha. A causa é diferente de F1 (literal hardcoded, não roteamento de catálogo i18n) — pelo teste da Regra Dura: corrigir F1 não fecha F5.

**Ação recomendada:** REQ separada para (a) converter a mensagem de manager.go para `i18n.T()` e (b) adicionar `"aviso"` ao `ptUnaccentedWords` para fechar o gap do guard.

---

### Status dos achados pós-Rodada 2

| Achado | Rodada 1 | Rodada 2 | Notas |
|---|---|---|---|
| F1 — i18n PT runtime | MEDIUM / BLOQUEANTE | **FECHADO** | DetectLocale fixo; catálogos PT/ES removidos; testes PASS |
| F2 — CLAUDE.md seção externa retém PT | LOW / por design | **INALTERADO** | Residual aceito por AC3 |
| F3 — CHANGELOG advisory | INFO / pendente | **INALTERADO** | Exigência do ADR D2; ação do arquiteto na segunda tag |
| F4 — scaffold sem `## Acceptance Criteria` | LOW / pre-existing | **FECHADO** | Adicionado em ML-3F; teste PASS |
| F5 — literal PT hardcoded em manager.go:512 | (não identificado) | **NOVO LOW / não-bloqueante** | Pré-existente em main; guard gap; REQ separada |

---

### Desvios de método (Rodada 2)

Nenhum. Todos os projetos descartáveis criados com `mktemp -d` diretamente. Nenhum comando com escrita rodou no repositório real. Binário compilado para o scratchpad da sessão.

---

### Estado do repositório (Rodada 2)

```
$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short
(saída vazia — apenas este arquivo aparecerá após ser escrito)

$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw diff --cached --stat
(vazio)
```

---

## Resumo para o arquiteto

**Iniciado:** red team de ML-4A (5 superfícies: gerador→leitor, meta-geradores, AC3 update, AC2/i18nguard, CHANGELOG advisory).

**Concluído:**
- S1 PASS: `trackfw roadmap new` gera EN roadmap real; `**Wave gates:**` reconhecido; `exit 1` falha fechado; `barrier result: passed` RC=0.
- S2 PASS: acervo PT preservado (243 roadmaps, paridade confirmada).
- S3 PASS (init isolado): generated tree e HOME com `HOME=$(mktemp -d)` — sem resíduo PT.
- S4 PASS: `update` preserva edições locais; PT roadmap `result: passed` RC=0 após update.
- S5 **REPROVA**: `LANG=pt_BR.UTF-8` → PT output via `i18n.T()`; AC2 "teste que reprova o retorno do português nas mensagens emitidas" não satisfeito. Novo ML nesta REQ antes do merge.
- S6 INFORMAÇÃO: CHANGELOG advisory deve ser escrito na segunda tag.

**Blocking — ação necessária antes do merge:**
- **F1 (MEDIUM):** novo ML nesta REQ — Opção B: pin `DetectLocale()` a en-US ou remover catálogos pt-BR/es-ES + `TestNoRuntimePTOutput`. PR fica aberto.

**Não blocking — a cargo do arquiteto:**
- **F2 (LOW):** ML opcional para update da seção externa do CLAUDE.md.
- **F3 (INFO):** advisory no CHANGELOG quando criar a segunda tag.
- **F4 (LOW, pre-existing):** scaffold `/trackfw:roadmap` omite `## Acceptance Criteria`. Causa diferente de F1; REQ dedicada se priorizado.

**Desvios de método declarados:** validate+barrier acidental no repositório real (Desvio 1, causa indeterminada); diretório `" /"` com nested `git init` criado e removido da árvore real (Desvio 2, alias eza); init/update com HOME real — verificado que nenhum arquivo de agent/skill foi modificado hoje (Desvio 3).
