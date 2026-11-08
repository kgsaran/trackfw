# Wave 0 — Threat Model: Idioma inglês primeiro (REQ-2026-11-02)

> Hades (hades-tf) · 2026-11-08 · ML-0A · ML-0B corretivo (2026-11-08)
> REQ: docs/req/REQ-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md
> ADR: docs/adr/ADR-2026-11-08-contrato-de-marcadores-do-roadmap-bilingue-parser-aceita-ingles-e-portugues-antes-de-o-gerador-emitir-ingles.md

---

## Veredito

**APROVA COM RESSALVAS.** A ADR D1–D7 está correta e a ordem das waves é a certa. O fail-open de D2 foi medido com o binário 11.5.1 e confirmado (ver §3). O ADR D3 postergou propositalmente a medição para a Wave 0: "se hoje não valer (a Wave 0 mede)" — medido: não vale hoje; ML-1A implementa. Os quatro achados do ML-0A permanecem válidos. ML-0B (corretivo de AC1) expandiu o inventário de 12 para 34 arquivos com literais PT em código executado (grupo 3, 2 e 1; tabela completa: 48 arquivos), adicionou `validator_roadmap_gates.go` (G1 mensagens com `**Gates da wave:**`) e encontrou três novos leitores G3 (G3-10 em `config.go`, G3-11 em `sync.go`, G3-12 em `roadmap.go`) mais uma correção de mecanismo (G3-9: `validator_req_done_criteria.go:87` usa regex próprio, não delegado) — todos já bilíngues, nenhum exige alias no ML-1A. Ver §1 e §2 atualizados. Há quatro achados:

1. **BLOQUEANTE para ML-1A — fail-open confirmado (wave 1+):** `**Wave gates:**` não reconhecido produz 0 gates e `gates: passed` — confirmado no binário 11.5.1 (Braço B e Braço wave-1-en, ver §3). Para waves 1+, não há controle compensador no validate; uma instância de roadmap válida com `**Wave gates:**` em wave 1 pode produzir RC=0 com `exit 1` não executado. Para wave 0 existe controle compensador parcial via `Wave0GateAbsent` (ver §3).

2. **IMPLEMENTAR em ML-1A:** dois `**Gates da wave:**` na mesma wave → primeiro bloco executa, segundo é silenciosamente ignorado. ADR D3 pede "erro que nomeia as duas linhas" — esse comportamento não existe hoje (medido: braços D1/D2 em §3). ML-1A implementa.

3. **ACRESCENTAR AO HANDOFF de ML-1A:** `docs/cli-parity.md` pina `**Gates da wave:**` como única forma aceita na seção "Wave gates contract". Deve ser atualizado na mesma wave que implementa o alias EN.

4. **ACRESCENTAR AO HANDOFF de ML-3C:** `internal/generators/claudemd.go:36` e `internal/generators/agentfiles.go:112-113` instruem o agente consumidor a escrever marcadores de grupo 3 em PT (`⬜ Pendente`, `🔄 Em andamento`, `**Status:**`). São "meta-geradores": geram instrução que por sua vez produz marcadores PT nos roadmaps. ADR D2 fala em "o gerador" sem nomeá-los. Devem mudar na Wave 3 (depois da tag do leitor bilíngue), não antes.

Binário instalado: **trackfw 11.5.0** (Homebrew). Binário de main compilado: **trackfw 11.5.1** (SHA `9fd2d84b`). Os hooks deste repo rodam o instalado (11.5.0).

---

## 1. Inventário completo (AC1)

### Critério de classificação

- **Grupo 1**: mensagem emitida pelo CLI ao usuário (stdout/stderr/UI). Nunca lida de volta pelo binário para decisão.
- **Grupo 2**: conteúdo gerado nos artefatos do consumidor por `trackfw init`/`update`/`new`. Sub-classe **G2-meta**: texto de instrução que diz ao agente consumidor *quais marcadores escrever* em roadmaps (claudemd.go, agentfiles.go).
- **Grupo 3**: marcador lido pelo binário de volta de um roadmap ou REQ para tomar decisão (parser de contrato).

### Varredura realizada

Comando executado (a partir de `git ls-files` para limitar ao versionado):

```bash
# Arquivos Go de produto (excluindo _test.go e testdata/)
git ls-files internal/ cmd/ | grep '\.go$' | grep -v '_test\.go' | grep -v testdata | \
  xargs grep -ln "Pendente\|Concluído\|Em andamento\|Cancelado\|abandonado\|Caducou\|Gates da wave\|Arquivos afetados\|Critérios de aceite"

# Scripts com marcadores de grupo 3
git ls-files scripts/ | xargs grep -l "Gates da wave\|Wave gates\|Caducou\|Lapsed\|abandonado\|abandoned\|cancelado\|canceled" 2>/dev/null

# npm/, pypi/, Makefile — resultado: zero hits de marcadores de grupo 3
git ls-files npm/ pypi/ Makefile | xargs grep -l "Gates da wave\|Wave gates\|Caducou\|Lapsed" 2>/dev/null
# → sem saída (zero hits)
```

**npm/:** nenhum arquivo versionado contém marcadores de grupo 3 (`npm/bin/trackfw.js`, `npm/package.json`, `npm/shim_packaging.test.js` inspecionados). `npm/node_modules/` não versionado.

**pypi/:** `pypi/AGENTS.md` contém `abandoned` apenas como nome de state folder no breadcrumb descritivo. Nenhum parser. `pypi/pyproject.toml`, `pypi/scripts/build_wheel.py`: sem marcadores de grupo 3.

**Makefile:** sem marcadores de grupo 3.

**internal/identity/preset.go:** grep de marcadores de grupo 3 retornou 0 hits. Escopo da ADR D6 (nomes e texto descritivo dos presets). Será editado na Wave 3 / ML-3C.

**JavaScript estático do `serve`** (`internal/serve/static/app.js`): `'Analisando'` em linhas 58 e 116. Grupo 1 (UI). Nenhum parser de grupo 3.

**Skills e agentes** (`.agents/skills/`): três arquivos SKILL.md presentes; nenhum contém marcadores de grupo 3 em PT.

**internal/generators/roadmap_show_json.go:75,171** e **internal/metrics/metrics.go:134**: os hits do grep são **comentários** (`//`), não literais de código executado.

### Tabela por arquivo e grupo (produto, não-teste) — ML-0B: inventário completo

**Metodologia de contagem (ML-0B):** o regex original do arquiteto pega qualquer linha com string entre aspas duplas ou backtick contendo acento — inclusive comentários `//` inline em linhas de código e strings dentro de comentários de doc. A contagem da coluna "Lits. PT exec." usa filtro de linha-de-comentário puro; complementado por verificação manual para backtick multi-linha e literais sem acento. Resultado: o regex do arquiteto inflou os contadores (ex.: `validator.go:31` → 3 exec., `roadmapdoc.go:7` → 1 acc.+5 unac. exec., `sync.go:3` → 1 exec.). A coluna "Arq. regex" preserva o valor original para rastreabilidade. Palavras portuguesas sem acento (ex.: `"abandonado"`, `"cancelado"`, `"concluido"`, `"Gates da wave"`, `"Caducou"`, `"**Arquivos afetados:**"`) são contadas separadamente em "+unac".

**Reconciliação:** varredura por acento do arquiteto: **45 arquivos**. ML-0B confirma 45 + 3 adições não capturadas pelo regex de acento: `barrier.go` (G3 unacento, 1 literal), `baseline.go` (G1 backtick multi-linha, 1 literal), `validator_roadmap_gates.go` (G1 mensagens com `**Gates da wave:**`, unacento). **13 dos 45** têm PT só em comentários `//` (executável = 0). **34 arquivos** têm pelo menos 1 literal PT em código executado. Total na tabela: **48 arquivos** (45 + 3 adições).

| Arquivo | Lits. PT exec. | Arq. regex | Grupo | Direção | ML destino |
|---------|---------------|------------|-------|---------|------------|
| **Grupo 3 — leitores (lê de arquivos do consumidor)** | | | | | |
| `internal/roadmapdoc/roadmapdoc.go` | 1 acc.+5 unac.¹ | 7 | 3 | lê | ML-1A (G3-1,2,3,4); bilíngue (G3-5,7,8) |
| `internal/commands/barrier.go` | 1 unac.² | 0 | 3 | lê | bilíngue (G3-6); sem ML-1A |
| `internal/validator/validator_req_done_criteria.go` | 1 acc.+1 unac.⁵ | 2 | 1+3 | emite+lê | bilíngue (G3-9); sem ML-1A; G1: linha 373 `Caducou:` em mensagem de erro |
| `internal/config/config.go` | 5 | 6 | 1+3 | emite+lê | ML-3B (G1); bilíngue (G3-10); sem ML-1A |
| `internal/sync/sync.go` | 1 | 3 | 3 | lê | bilíngue (G3-11); sem ML-1A |
| `internal/generators/roadmap.go` | 9 | 17 | 2+3 | emite+lê | ML-3A (G2); bilíngue (G3-12); sem ML-1A |
| **Grupo 2 — emissores de conteúdo para artefatos do consumidor** | | | | | |
| `internal/generators/scaffold.go` | 41 | 44 | 2+2-meta | emite | ML-3C |
| `internal/identity/preset.go` | 11 | 11 | 2 | emite | ML-3C (ADR D6) |
| `internal/generators/claudemd.go` | 2 | 2 | 2-meta | emite | ML-3C |
| `internal/generators/agentfiles.go` | 1 | 1 | 2-meta | emite | ML-3C |
| `internal/discover/discover.go` | 4 | 7 | 1+2 | emite | ML-3B (G1 msgs); ML-3C (G2 config); `"docs/requisições"` é sonda de caminho — deve sobreviver Wave 3 |
| `internal/generators/note.go` | 1 | 1 | 1+2 | emite | ML-3B |
| `internal/generators/nextsteps.go` | 1 | 1 | 1 | emite | ML-3B |
| **Grupo 1 — mensagens emitidas ao usuário (stdout/stderr/UI)** | | | | | |
| `internal/commands/help.go` | 45 | 45 | 1 | emite | ML-3B |
| `internal/generators/update.go` | 8 | 9 | 1 | emite | ML-3B |
| `internal/commands/identity_wizard.go` | 7 | 8 | 1 | emite | ML-3B |
| `internal/serve/static/app.js` | 5 | 7 | 1 | emite | ML-3B |
| `internal/commands/branch.go` | 5 | 5 | 1 | emite | ML-3B |
| `internal/commands/configure.go` | 6 | 6 | 1 | emite | ML-3B |
| `internal/commands/metrics.go` | 4 | 4 | 1 | emite | ML-3B |
| `internal/validator/validator.go` | 3 | 31 | 1 | emite | ML-3B; G3 via config.AcceptanceMarkers (G3-10) |
| `internal/validator/validator_git_branch_guard.go` | 3 | 3 | 1 | emite | ML-3B |
| `internal/identity/identity.go` | 3 | 3 | 1 | emite | ML-3B |
| `internal/generators/probes.go` | 3 | 3 | 1³ | emite | ML-3B; PT keywords (`"autenticação"`, `"migração"` etc.) devem sobreviver Wave 3 |
| `internal/commands/agents_models.go` | 3 | 3 | 1 | emite | ML-3B |
| `internal/validator/validator_credential_guard.go` | 2 | 5 | 1 | emite | ML-3B |
| `internal/commands/integrations_flags.go` | 2 | 2 | 1 | emite | ML-3B |
| `internal/commands/commit.go` | 2 | 2 | 1 | emite | ML-3B |
| `internal/metrics/metrics.go` | 1 | 1 | 1 | emite | ML-3B |
| `internal/integrations/models.go` | 1 | 1 | 1 | emite | ML-3B |
| `internal/commands/req.go` | 1 | 2 | 1 | emite | ML-3B |
| `internal/commands/adr.go` | 1 | 1 | 1 | emite | ML-3B |
| `internal/serve/static/index.html` | 9 | 9 | 1 | emite | ML-3B |
| `internal/commands/baseline.go` | 1⁴ | 0 | 1 | emite | ML-3B |
| `internal/validator/validator_roadmap_gates.go` | 2 unac.⁶ | 0 | 1 | emite | ML-3B; mensagens de erro com `**Gates da wave:**` (linhas 116, 121) |
| **Só comentário `//` — PT apenas em linhas de comentário, executável = 0** | | | | | |
| `internal/validator/validator_credential_guard_integrity.go` | 0 | 2 | — | — | — |
| `internal/thirdparty/markers.go` | 0 | 2 | — | — | — |
| `internal/integrations/render.go` | 0 | 2 | — | — | — |
| `internal/generators/req_chain_ml4a.go` | 0 | 2 | — | — | — |
| `internal/thirdparty/quarantine.go` | 0 | 1 | — | — | — |
| `internal/validator/validator_req_roadmap_cutoff.go` | 0 | 1 | — | — | — |
| `internal/validator/validator_git_exec.go` | 0 | 1 | — | — | — |
| `internal/integrations/manager.go` | 0 | 1 | — | — | — |
| `internal/guard/writescope.go` | 0 | 1 | — | — | — |
| `internal/commands/ship.go` | 0 | 1 | — | — | — |
| `internal/commands/branch_prune_remote.go` | 0 | 1 | — | — | — |
| `internal/generators/adr.go` | 0 | 1 | — | — | — |
| `internal/generators/req.go` | 0 | 1 | — | — | — |

¹ `roadmapdoc.go`: regex de acento encontra 7 mas 6 são matches em comentários inline. Executáveis com acento: linha 44 (`CriteriaHeaderRe`, acc.). Executáveis sem acento: linha 51 (`GatesHeaderRe`, G3-1), 58 (`LapsedContinuationRe`, G3-2), 81 (`"concluido"`, G3-7), 196 (`"abandonado"`, G3-3), 203 (`"cancelado"`, G3-4) — total 1 acc.+5 unac. Adicionais: linha 1011 (`const prefix = "Caducou:"`, extrator G3-2) e linha 1212 (mensagem de erro com `**Gates da wave:**`, G1); não contados na célula.
² `barrier.go`: linha 887 — `"**Arquivos afetados:**"` (PT sem acento, G3-6); linha 1842 — `"      line %d: Caducou: %s\n"` (G1 output). Zero hits no regex de acento do arquiteto.
³ `probes.go`: `DetectDomains` casa keywords contra `title + motivation` que o usuário digita na hora (`req new`), não contra arquivo versionado — G1 comportamental, não G3 de arquivo. Mas os PT keywords (`"autenticação"`, `"migração"`, `"senha"`, `"banco"`, `"notificação"`) são a cobertura PT-first do detector de domínio; se removidos na Wave 3, REQs em PT param de acionar as probes de threat model. D5-análogo: **os PT keywords devem permanecer mesmo após a Wave 3 mudar as mensagens para EN**.
⁴ `baseline.go`: linha 14-16 — `Long:` em backtick multi-linha; zero no regex de acento (não casa backtick multi-linha). G1.
⁵ `validator_req_done_criteria.go`: linha 87 `reqACHeadingRe` (acc., G3-9 bilíngue); linha 373 `"...Caducou:..."` em mensagem de erro (unac., G1). Arch regex=2: linha 11 é comentário `//`, linha 87 é executável.
⁶ `validator_roadmap_gates.go`: linhas 116 e 121 — `fmt.Sprintf` com `'**Gates da wave:**'` em mensagens de diagnóstico (G1, unac.). Não está nos 45 do arquiteto (zero chars acentuados).

**Segundo sítio do contrato de leitura — não na ADR:** `docs/cli-parity.md` documenta `**Gates da wave:**` como único marcador aceito na seção "Wave gates contract". Quando `GatesHeaderRe` ganhar o alias EN, o cli-parity.md deve ser atualizado na mesma wave.

**Scripts com marcadores de grupo 3 como INPUT (tipo ii):**

| Script | Natureza do uso |
|--------|----------------|
| `scripts/check-barrier.sh` | Fixtures com `**Gates da wave:**`, `**Critérios de aceite:**` como input ao parser — tipo ii |
| `scripts/check-gates-falsify.sh` | Idem |
| `scripts/check-roadmap-barrier-contract.sh:1282,1315` | Cenários CRLF e prefixo com `**Gates da wave:**` como input — tipo ii |

### Tabela linha a linha do Grupo 3

| # | Arquivo | Linha | Literal / Regex | Aceita hoje | Missing EN | Decisão ADR |
|---|---------|-------|-----------------|-------------|------------|-------------|
| G3-1 | `internal/roadmapdoc/roadmapdoc.go` | 51 | `GatesHeaderRe = regexp.MustCompile("^\*\*Gates da wave:\*\*")` | PT only | `**Wave gates:**` | D1 — adicionar alias |
| G3-2 | `internal/roadmapdoc/roadmapdoc.go` | 58 | `LapsedContinuationRe = regexp.MustCompile("^ {2,}Caducou:\s*\S")` | PT only | `  Lapsed: <text>` | D1 — adicionar alias |
| G3-3 | `internal/roadmapdoc/roadmapdoc.go` | 195–198 | `terminatedVocabulary = {"abandonado": true, "🚫": true}` | PT + emoji | `abandoned` | D1 — adicionar |
| G3-4 | `internal/roadmapdoc/roadmapdoc.go` | 203 | `canceladoNormalized = "cancelado"` (desempate do `❌`) | PT only | `canceled`, `cancelled` | D1 — adicionar |
| G3-5 | `internal/roadmapdoc/roadmapdoc.go` | 44 | `CriteriaHeaderRe` aceita `Acceptance criteria` e `Critérios de aceite` | EN + PT | — | já bilíngue |
| G3-6 | `internal/commands/barrier.go` | 886–887 | `strings.HasPrefix(line, "**Files affected:**") \|\| "**Arquivos afetados:**"` | EN + PT | — | já bilíngue |
| G3-7 | `internal/roadmapdoc/roadmapdoc.go` | 78–82 | `statusVocabulary = {"✅", "done", "concluido"}` (após fold de acento) | EN + PT | — | já bilíngue |
| G3-8 | `internal/roadmapdoc/roadmapdoc.go` | 180–215 | `StatusCategory`: pendente/ativo decidido por primeiro emoji `⬜`/`🔄`/`❌` | language-neutral | — | n/a |
| G3-9 | `internal/validator/validator_req_done_criteria.go` | 87 | `reqACHeadingRe = regexp.MustCompile("(?i)^## (?:Acceptance Criteria\|Crit[eé]rios de [Aa]ceite)")` | EN + PT | — | mecanismo corrigido (ML-0B: regex próprio, não delegado a `CriteriaHeaderRe`); bilíngue; sem ML-1A |
| G3-10 | `internal/config/config.go` | 398 | `AcceptanceMarkers: []string{"## Acceptance Criteria", "## Critérios de Aceite"}` (usado em `validator.go:2979`) | EN + PT | — | novo (ML-0B); bilíngue; sem ML-1A |
| G3-11 | `internal/sync/sync.go` | 202 | `strings.HasPrefix(line, "## Motivation") \|\| strings.HasPrefix(line, "## Motivação")` em `extractMotivation` | EN + PT | — | novo (ML-0B); bilíngue; sem ML-1A |
| G3-12 | `internal/generators/roadmap.go` | 661 | `lower == "## critérios de aceite" \|\| lower == "## acceptance criteria"` em `NewRoadmapFromREQ` | EN + PT | — | novo (ML-0B); bilíngue; sem ML-1A |

**Leitores fora de `internal/roadmapdoc`, `internal/commands/barrier.go` e `internal/validator`?**

`internal/serve/api_board.go` delega inteiramente a `roadmapdoc.StatusCategory` e `roadmapdoc.MLStatusMarker`. As comparações com `🔄` e `⬜` (linhas 221–228) são de exibição UI, language-neutral.

**ML-0B — G3-9 a G3-12:** G3-9 é correção de mecanismo (`validator_req_done_criteria.go:87` usa regex próprio, não delegado). G3-10 (`config.go`), G3-11 (`sync.go`) e G3-12 (`generators/roadmap.go`) são três novos readers. Todos bilíngues (EN + PT), nenhum exige alias no ML-1A. Inventário G3 passa de 8 para 12 entradas; seis zonas de leitura no total.

---

## 2. Grupo 3 fechado

### Tabela ADR (Context) vs inventário medido

| Marcador (ADR) | Onde é lido | Aceita hoje | Medido (hades) | Status |
|----------------|------------|-------------|----------------|--------|
| `**Acceptance criteria:**` / `**Critérios de aceite:**` | `roadmapdoc.CriteriaHeaderRe` | EN e PT | confirmado (linha 44) | ADR correto |
| `**Files affected:**` / `**Arquivos afetados:**` | `barrier.go` escopo do ML | EN e PT | confirmado (linhas 886–887) | ADR correto |
| `## Acceptance Criteria` / `## Critérios de aceite` | `validator_req_done_criteria.go:87` — regex próprio `reqACHeadingRe`; ML-0A tinha anotado "delegado a `CriteriaHeaderRe`" — incorreto, confirmado ML-0B: regex independente | EN e PT | confirmado via `reqACHeadingRe` linha 87 | ADR correto; mecanismo corrigido |
| Status completo (`✅`, `done`, `concluido`) | `roadmapdoc.statusVocabulary` | EN e PT | confirmado (linhas 78–82) | ADR correto |
| `**Gates da wave:**` | `roadmapdoc.GatesHeaderRe` | **só PT** | confirmado (linha 51) — `**Wave gates:**` não reconhecido (medido Braço B) | ADR correto; fix pendente |
| `  Caducou:` | `roadmapdoc.LapsedContinuationRe` | **só PT** | confirmado (linha 58) | ADR correto; fix pendente |
| Término `abandonado`, `🚫` | `roadmapdoc.terminatedVocabulary` | **só PT + emoji** | confirmado (linhas 195–198) | ADR correto; fix pendente |
| Término `❌ Cancelado` desempate | `roadmapdoc.canceladoNormalized` | **só PT** | confirmado (linha 203) | ADR correto; fix pendente |
| `## Acceptance Criteria` / `## Critérios de Aceite` | `config.go:398` — `AcceptanceMarkers` slice lido por `validator.go:2979 contentHasMarker()` | EN e PT | confirmado (linha 398) | **Novo** (ML-0B); bilíngue; sem ML-1A |
| `## Motivation` / `## Motivação` | `sync.go:202` — `extractMotivation` lê de arquivos REQ | EN e PT | confirmado (linha 202) | **Novo** (ML-0B); bilíngue; sem ML-1A |
| `## critérios de aceite` / `## acceptance criteria` | `roadmap.go:661` — `NewRoadmapFromREQ` lê AC de REQ | EN e PT | confirmado (linha 661) | **Novo** (ML-0B); bilíngue; sem ML-1A |

### Disposição por arquivo — arquivos citados no handoff do arquiteto

O arquiteto listou 11 arquivos para inspeção de G3. Resultado por arquivo:

| Arquivo | PT executável? | É G3? | Evidência |
|---------|---------------|-------|-----------|
| `internal/validator/validator.go` | sim (G1 msgs) | não direto | linha 2979 chama `contentHasMarker(config.AcceptanceMarkers, ...)` — delegado a `config.go:398` (G3-10); não mantém regex próprio |
| `internal/config/config.go` | sim (G1+G3) | **sim — G3-10** | linha 398: `AcceptanceMarkers: []string{"## Acceptance Criteria", "## Critérios de Aceite"}` lido por `contentHasMarker` em validator |
| `internal/discover/discover.go` | sim (G1+path) | não | linha 455: `"docs/requisições"` é sonda de caminho de filesystem (`dirExists(filepath.Join(rootDir, candidate))`) — não parseia marcador dentro de um arquivo; deve sobreviver Wave 3 pois o consumidor pode usar esse nome de diretório |
| `internal/sync/sync.go` | sim (G3) | **sim — G3-11** | linha 202: `extractMotivation` lê `## Motivação` de arquivos REQ; já bilíngue |
| `internal/thirdparty/markers.go` | não (só comentários) | não | confirmado por Python — zero literais executados |
| `internal/generators/update.go` | sim (G1 msgs) | não | idempotência verificada: sentinelas são `"trackfw validate"` (linha 261), `"trackfw-validate:"` (linha 285), `"commit-msg:"` (linha 1828 de scaffold) — todos language-neutral; nenhum PT marker como sentinela |
| `internal/generators/req_chain_ml4a.go` | não (só comentários) | não | confirmado por Python — zero literais executados |
| `internal/integrations/render.go` | não (só comentários) | não | confirmado por Python — zero literais executados |
| `internal/guard/writescope.go` | não (só comentários) | não | confirmado por Python — zero literais executados |
| `internal/validator/validator_req_roadmap_cutoff.go` | não (só comentários) | não | confirmado por Python — zero literais executados |
| `internal/metrics/metrics.go` | sim (G1) | não | linha 134 é comentário `//`; literal executável em G1 (mensagem de erro) — não parseia artefato do consumidor |

**Idempotência em `update.go`, `claudemd.go`, `agentfiles.go`, `scaffold.go`:** nenhum dos quatro usa marcador PT como sentinela de idempotência. O risco D5-análogo não se aplica aqui: quando a Wave 3 mudar os emissores para EN, `update` continuará encontrando o bloco existente pelo sentinel language-neutral. Sem alias necessário para esses arquivos.

**Conclusão (ML-0B):** a ADR declarava três zonas de leitura G3 (`roadmapdoc`, `barrier`, `validator`). ML-0B encontrou três zonas novas além das três originais: `config` (G3-10), `sync` (G3-11) e `generators/roadmap` (G3-12). G3-9 é correção de mecanismo (não novo reader): `validator_req_done_criteria.go` usa `reqACHeadingRe` próprio, não delegado a `CriteriaHeaderRe`. Inventário G3: 12 readers — 4 PT-only (G3-1,2,3,4) exigem ML-1A; 8 bilíngues (G3-5,6,7,8,9,10,11,12) não exigem. A ADR deve declarar as seis zonas de leitura em ML-1A.

---

## 3. Fail-open entre versões (ADR D2), medido

Binário: `trackfw 11.5.1` (compilado de `origin/main`, SHA `9fd2d84b`, em worktree temporário fora do repo). Repo de teste: `$SCRATCH/testrepo/` com `trackfw.yaml` apontando `roadmap_dir: docs/roadmaps` e `req_dir: docs/req`. Flag `--trust-local-gates` usada (roadmaps não commitados → ml_scope retorna skipped).

### Braço A — controle: `**Gates da wave:**` + `exit 1`

```
$ cd $SCRATCH/testrepo && $SCRATCH/trackfw-main barrier arm-a --wave 0 --trust-local-gates --json

{"checks":[
  {"name":"wave_headings","status":"passed"},
  {"name":"mls_complete","status":"passed","evidence":["ML-0A: ✅"]},
  {"name":"acceptance_evidence","status":"passed","evidence":["ML-0A: 1 criteria met"]},
  {"name":"gates","status":"blocked","commands":["exit 1"],"failures":["exit 1: exit 1"]},
  {"name":"validate","status":"blocked","failures":["13 violations, 3 warnings"]},
  {"name":"ml_scope","status":"skipped"}
],"status":"blocked"}
```

### Braço B — D2 fail-open: `**Wave gates:**` (EN) + `exit 1`, wave 0

```
$ $SCRATCH/trackfw-main barrier arm-b --wave 0 --trust-local-gates --json

{"checks":[
  {"name":"wave_headings","status":"passed"},
  {"name":"mls_complete","status":"passed","evidence":["ML-0A: ✅"]},
  {"name":"acceptance_evidence","status":"passed","evidence":["ML-0A: 1 criteria met"]},
  {"name":"gates","status":"passed","commands":[],"failures":[]},
  {"name":"validate","status":"blocked","failures":["13 violations, 3 warnings"]},
  {"name":"ml_scope","status":"skipped"}
],"status":"blocked"}
```

**`gates: passed` com 0 comandos confirmado.** Validate bloqueou com 13 violações — mesmo total do braço A. A diferença está dentro das violações: braço A tem `Wave0GatePlaceholder`; braço B tem `Wave0GateAbsent`. Ambos contam 1 violação, o total coincide.

### Por que validate bloqueia (wave 0) — controle compensador parcial

A `Wave0GateDiagnosis` chama `ParseGates` (que usa `GatesHeaderRe`). No braço B, `**Wave gates:**` não é reconhecido → 0 comandos. Como `mls_complete` exige ML-0A ✅ → `hasAnyNonPendingML` = true → `Wave0GateDiagnosis` retorna `Wave0GateAbsent` → violação `roadmap_gate_coverage` no validate.

Isso é um controle compensador **exclusivo de wave 0**: para `barrier --wave 0`, mls_complete exige MLs ✅, o que sempre ativa `hasAnyNonPendingML`. Para wave 0, o fail-open completo (RC=0) é bloqueado pelo validate mesmo que gates passe.

### Braço wave-1-en — fail-open real (wave 1): `**Wave gates:**` em wave 1, `**Gates da wave:**` com `true` em wave 0

```
$ $SCRATCH/trackfw-main barrier arm-wave1-en --wave 1 --trust-local-gates --json

{"checks":[
  {"name":"wave_headings","status":"passed"},
  {"name":"mls_complete","status":"passed","evidence":["ML-1A: ✅"]},
  {"name":"acceptance_evidence","status":"passed","evidence":["ML-1A: 1 criteria met"]},
  {"name":"gates","status":"passed","commands":[],"failures":[]},
  {"name":"validate","status":"blocked","failures":["16 violations, 3 warnings"]},
  {"name":"ml_scope","status":"skipped"}
],"status":"blocked"}
```

Validate inspecionado via `validate --json`: **nenhuma violação de tipo `roadmap_gate_coverage` para arm-wave1-en.md**. As 16 violações vêm de estrutura mínima do fixture (incluindo `wip_acceptance` pela ausência do bloco `## Acceptance Criteria` no roadmap). `Wave0GateDiagnosis` só verifica wave 0; para wave 1, não há diagnóstico análogo.

**Conclusão:** em um roadmap válido (validate-clean) com `**Wave gates:**` em wave N (N ≥ 1), o barrier retornaria RC=0 com `exit 1` não executado. Este é o fail-open que a ADR D2 descreve como residual aceito (reduzido pela tag sequencial, não eliminado). Para wave 0, o controle compensador via `Wave0GateAbsent` impede RC=0.

### Braço C — sem cabeçalho nenhum

```
gates: passed, commands: []
```

Braço B = Braço C no comportamento de gates. `**Wave gates:**` unrecognized ≡ absent.

### Braços D1/D2 — dois `**Gates da wave:**` em série (ADR D3)

**D1** (ordem: `true` primeiro, `exit 1` segundo):
```
gates: passed, commands do 1º bloco: ["echo GATE_A", "true"] — 2º bloco ignorado
```

**D2** (ordem invertida: `exit 1` primeiro, `true` segundo):
```
gates: blocked, commands do 1º bloco: ["echo GATE_B", "exit 1"] — 2º bloco ignorado
```

**Medido: dois cabeçalhos → primeiro bloco vence, segundo ignorado sem erro.** ADR D3 pede "erro que nomeia as duas linhas" — ML-1A implementa.

### Resumo da medição

| Braço | Cabeçalho da wave-alvo | Gates enc. | `gates:` | Controle compensador no validate |
|-------|------------------------|-----------|----------|----------------------------------|
| A (controle) | `**Gates da wave:**` em wave 0 | 1 (`exit 1`) | blocked | n/a (gates já bloqueou) |
| B (D2) | `**Wave gates:**` em wave 0 | 0 | passed | `Wave0GateAbsent` → validate bloqueia |
| C | nenhum em wave 0 | 0 | passed | `Wave0GateAbsent` → validate bloqueia |
| wave-1-en (D2 real) | `**Wave gates:**` em wave 1 | 0 | passed | **Nenhum** — validate não tem diagnóstico para wave 1+ |
| D1 | dois PT em wave 1 (`true` primeiro) | 2 cmds do 1º bloco | passed | n/a |
| D2 | dois PT em wave 1 (`exit 1` primeiro) | 2 cmds do 1º bloco | blocked | n/a |

---

## 4. Quem afirma a forma antiga

### (i) Testes que afirmam PT na SAÍDA do gerador — mudam na Wave 3

Estes verificam que o **gerador emite** a forma portuguesa. Devem ser atualizados junto com os geradores no ML-3A/3B/3C.

| Arquivo | Linha | Nome do teste | O que afirma |
|---------|-------|---------------|-------------|
| `internal/generators/roadmap_test.go` | 96 | `TestNewRoadmap_StatusLegendAndCanonicalForm` | Gerador emite legenda `"⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado"` |
| `internal/generators/roadmap_test.go` | 103–104 | `TestNewRoadmap_StatusLegendAndCanonicalForm` | Gerador emite `**Status:** ⬜ Pendente` |
| `internal/generators/roadmap_test.go` | 112–113 | `TestNewRoadmap_StatusLegendAndCanonicalForm` | Gerador emite `**Gates da wave:**` |
| `internal/generators/roadmap_test.go` | 199 | `TestMoveRoadmap_FrontmatterSync_ValidateAfterMove` | Substitui `**Status:** ⬜ Pendente` → `**Status:** ✅ Concluído` |
| `internal/generators/roadmap_test.go` | 679–681 | `TestNewRoadmapFromREQ_SingleGateBlockFromLegitREQ` | Conta exatamente 1 bloco `**Gates da wave:**` no roadmap gerado |
| `internal/generators/roadmap_test.go` | 727 | `TestNewRoadmapFromREQ_StatusLegendAndCanonicalForm` | Gerador (fromREQ) emite legenda PT |
| `internal/generators/roadmap_test.go` | 732–733 | `TestNewRoadmapFromREQ_StatusLegendAndCanonicalForm` | Gerador emite exatamente 3 ocorrências de `**Status:** ⬜ Pendente` |

Grep explícito em `internal/generators/scaffold_test.go`, `claudemd_test.go`, `agentfiles_test.go`, `update_test.go` e `help_test.go` (caminhos individuais, não glob): nenhum outro arquivo de teste em `internal/generators/` fixa marcadores de grupo 3 diretamente nos assertions de saída.

### (ii) Testes que alimentam PT como ENTRADA ao parser — devem permanecer (D5)

Alimentam a forma portuguesa como input para confirmar que o parser a aceita. São a garantia de D5. **Não devem ser removidos na Wave 3** — Wave 1 acrescenta braços EN, mas os braços PT permanecem.

| Arquivo | O que afirma |
|---------|-------------|
| `internal/roadmapdoc/gates_lines_test.go` | `ParseGatesLines` reconhece `**Gates da wave:**` como marcador válido |
| `internal/roadmapdoc/acceptance_lapsed_test.go` | `  Caducou: texto` aceito como lapsed |
| `internal/roadmapdoc/roadmapdoc_test.go` | `❌ Cancelado`, `ABANDONADO`, `🚫 Abandonado` → `StatusTerminated`; `❌ Bloqueado` → `StatusPending` |
| `scripts/check-barrier.sh` linhas 272, 276, 284, 292, 306, 369, 380, 406, 410, 418, 444, 448, 454, 484, 488, 496, 529, 533, 539, 578, 582, 590, 618, 658, 662, 668 | Alimenta `**Gates da wave:**`, `**Critérios de aceite:**`, `⬜ Pendente` como input ao barrier |
| `scripts/check-gates-falsify.sh` linhas 1166, 1170 | Alimenta `**Gates da wave:**` e `**Critérios de aceite:**` como input |
| `scripts/check-roadmap-barrier-contract.sh` linhas 1282, 1315 | Alimenta `**Gates da wave:**` em cenários CRLF e prefixo com texto extra |

O cenário `**Gates da wave:** (obrigatórios)` em linha 1315 é um teste de que casamento por prefixo AINDA é reconhecido. É tipo (ii): permanece como cobertura de D5 para a grafia PT.

---

## 5. Threat model

**Adversário:** o implementador apressado e o arquiteto otimista — quem esvazia esta Wave 0 sem quebrar nenhuma regra escrita, por pressa, por subestimar o impacto ou por cortar a ordem das waves.

### 5.1 Enumeração de superfícies

| Superfície | G3# | Ponto de entrada da sabotagem | Gate que deveria pegar | Falso negativo | Falso positivo |
|------------|-----|------------------------------|------------------------|----------------|----------------|
| `GatesHeaderRe` (cabeçalho de gates), wave 1+ | G3-1 | Gerador emite `**Wave gates:**` antes de reader bilíngue | Medição Braço wave-1-en | `**Wave gates:**` unrecognized → 0 gates → `barrier --wave N` (N≥1) passa sem executar gate | Texto de prosa com `**Wave gates:**` em position 0: improvável |
| `GatesHeaderRe`, wave 0 | G3-1 | Mesmo mecanismo em wave 0 | `Wave0GateAbsent` → validate bloqueia (controle compensador medido) | Mitigado pelo controle compensador | n/a |
| `LapsedContinuationRe` (critério caducado) | G3-2 | Alias EN muito largo: `  Lapsed:` sem texto seguinte conta como lapsed | Contagem de Lapsed vs Unmet | Under-recognition (PT falta) → mais itens Unmet → fail-closed, seguro | **Over-recognition** → critério Unmet passa como lapsed → barrier passa indevidamente. Ex.: `  Lapsed:` sem texto depois dos dois-pontos se regex omitir o `\S` |
| `terminatedVocabulary` (`abandoned`) | G3-3 | ML com `**Status:** abandoned-task` conta como terminado se implementado como substring | Barrier bloqueia done transition | Under-recognition → ML fica pendente → fail-closed | **Over-recognition** → ML com texto que contém `abandoned` como substring é terminado → wave avança; fix correto: comparação de token inteiro |
| `canceladoNormalized` (`canceled`/`cancelled`) | G3-4 | `❌ cancellation-pending` com `strings.HasPrefix(second, "cancel")` conta como terminado | Barrier bloqueia | Under-recognition → fail-closed | **Over-recognition** → `❌ cancellation-pending` termina ML indevidamente; fix correto: comparação exata (`second == "canceled" || second == "cancelled"`) |
| Dois cabeçalhos de gates | ADR D3 | Dois blocos; segundo ignorado; primeiro malicioso `true` | ADR D3 define erro — não implementado hoje | Segundo bloco real (`exit 1`) ignorado quando primeiro (`true`) passa | Dois cabeçalhos em roadmap legítimo (refactor que duplicou a seção) |
| `docs/cli-parity.md` desatualizado | meta | ML-1A adiciona alias no código mas não atualiza documento | Revisão de PR | Contrato documentado diverge do implementado | — |

### 5.2 Direções de falha por superfície

| Superfície | Falha segura (fail-closed) | Falha insegura (fail-open) |
|------------|---------------------------|---------------------------|
| Gates header (G3-1), wave 0 | Over-recognition (D3 dispara) | Under-recognition: **controlado** por `Wave0GateAbsent` |
| Gates header (G3-1), wave 1+ | Over-recognition (D3 dispara) | Under-recognition: **sem controle compensador** → barrier passa sem executar gate |
| Lapsed (G3-2) | Under-recognition → mais Unmet → fail-closed | **Over-recognition** → critério Unmet passa como lapsed → barrier passa indevidamente |
| Terminated (G3-3, G3-4) | Under-recognition → ML fica pendente → fail-closed | **Over-recognition** → ML terminado indevidamente → wave avança |
| CriteriaHeaderRe (G3-5) | Under-recognition → "no acceptance block" → barrier bloqueia | Over-recognition → prosa vira bloco de aceite |

A superfície com fail-open não mitigado é **gates em wave 1+** (G3-1). Lapsed e Terminated são fail-open na direção de over-recognition (alias muito largo) — o risco é o conjunto fechado de grafias. O roadmap já cita `blocked` como exemplo (ADR D1: "blocked continua pendente e um teste afirma isso").

### 5.3 Como o adversário esvazia esta Wave 0 sem quebrar regra escrita

**Via 1 — Pular a medição e declarar "confirmado" sem executar.** A seção §3 requer stdout literal com RC. Uma afirmação sem dados seria detectável na auditoria Zeus.

**Via 2 — Implementar ML-1A com regex muito ampla para `GatesHeaderRe`.** Exemplo: `^\*\*Wave.gates:\*\*` (ponto = qualquer char) reconhece `**Wave-gates:**`. Gate que pega: braço `**Wave-gates:**` → não reconhecido (testes de ML-1A devem incluir esse braço negativo).

**Via 3 — Não implementar o erro de D3 (dois cabeçalhos).** ML-1A deve incluir teste explícito para dois cabeçalhos → erro.

**Via 4 — Entregar Wave 3 (gerador EN) sem a tag da Wave 1 (leitor bilíngue) já instalada.** O binário instalado (11.5.0) leria `**Wave gates:**` em wave 1 como 0 gates → RC=0. Gate: ML-2B é pré-requisito da Wave 3 (ADR D2 + braço wave-1-en).

**Via 5 — Marcar ML-1A como ✅ antes de atualizar `docs/cli-parity.md`.** Critério de aceite de ML-1A deve incluir a atualização do cli-parity.md.

**Via 6 — Usar `strings.HasPrefix(second, "cancel")` para o alias EN de cancelado.** Resulta em `❌ cancellation-pending` sendo terminado. Gate: teste com esse braço negativo no ML-1A.

---

## 6. Residual declarado

1. **Binário instalado vs. main (11.5.0 vs. 11.5.1).** Para wave 0, o controle compensador (`Wave0GateAbsent`) existe. Para waves 1+, o fail-open permanece ativo até a instalação da primeira tag (Wave 2 / ML-2B). Residual declarado da ADR D2.

2. **Dois cabeçalhos → primeiro vence, sem erro.** ML-1A implementa D3.

3. **`docs/cli-parity.md` como segundo sítio do contrato.** ML-1A deve incluir a atualização.

4. **Sub-classe G2-meta não nomeada explicitamente na ADR D2.** O handoff de ML-3C deve incluir `claudemd.go` e `agentfiles.go`.

5. **`❌ Blocked` continua não reconhecido como terminado (by design).** Um teste em ML-1A deve afirmar que `❌ Blocked` → `StatusPending`, prevenindo regressão futura.

6. **Controle compensador de wave 0 depende de MLs não-pendentes.** Para o caso hipotético onde todos os MLs de wave 0 ainda estão pendentes E `**Wave gates:**` está presente, `Wave0GateAbsent` não dispararia — mas `mls_complete` bloquearia o barrier por ML não concluído. RC=0 permanece inalcançável por rota diferente.

---

## Verificação de escopo (mandatória)

```
$ git status --short
?? docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md

$ git diff --cached --name-only
(sem saída — índice vazio)
```

Único arquivo novo: `docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md`. Índice vazio.
