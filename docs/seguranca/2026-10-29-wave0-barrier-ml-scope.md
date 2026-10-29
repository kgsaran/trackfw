# Wave 0 — Threat model: barrier ml_scope lê declaração em lista e une os roadmaps da branch

> Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-29-barrier-ml-scope-le-a-declaracao-de-arquivos-afetados-em-formato-de-lista.md
> Autor: hades-tf | Data: 2026-10-29 | Branch: feat/documentacao-reflete-produto-11-windows-install-ps1

**Mapeamento de ações do ML-0A para seções:**
- Ação 1 (enumeração) → §1
- Ação 2 (regra de crases) → §2.1
- Ação 3 (modelo de ameaça) → §2.2–2.8
- Ação 4 (recorte de waves) → §2.9
- Ação 5 (alvos de falsificação) → §3
- Ação 6 (residual) → §4

---

## 1. Completude da enumeração

### 1.1 Busca ampla sobre a árvore inteira (excluindo docs/roadmaps)

```bash
git grep -l -e "Files affected" -e "Arquivos afetados" -- . ':!docs/roadmaps'
```

Saída produz 294 linhas cobrindo: `internal/commands/barrier.go`,
`internal/commands/barrier_ml_scope_test.go`, `internal/commands/barrier_test.go`,
`internal/generators/roadmap.go`, `internal/generators/scaffold.go`, `docs/cli-parity.md`,
`scripts/check-barrier.sh`, testdata (cópias em `pathguard/testdata`, `roadmapdoc/testdata`,
`scripts/testdata`), docs/adr, docs/req, docs/seguranca, vault/notes,
docs/agents-working-context.md, .claude/agent-memory/.

### 1.2 Lista fechada de leitores (código que implementa a gramática)

| Arquivo | Papel | Muda neste ML? |
|---|---|---|
| `internal/commands/barrier.go` | `parseMLScopeFilesAffected` — o defeito está aqui | Sim — é o fix |
| `internal/commands/barrier_ml_scope_test.go` | Testes da função e do gate de integração | Sim — testes novos |
| `internal/commands/barrier_test.go:508` | Usa `"**Files affected:**"` em `acceptanceEvaluate`; não é ml_scope | Não |
| `docs/cli-parity.md` §ml_scope | Contrato documental — pina a regra antiga (linha ~3227: "list marker stops the declaration") | Sim — seção ml_scope |
| `scripts/check-barrier.sh` cenário 18 | Usa declaração inline; sem acoplamento à regra de parada em `- ` | Não |
| `internal/generators/roadmap.go` | Emite rótulo vazio — não lê | Não |
| `internal/generators/scaffold.go` | Emite rótulo vazio — não lê | Não |
| Testdata | Cópias históricas — não parsers | Não |

Lista fechada: o `git grep` acima não encontrou nenhum arquivo de código fora dela que
implemente a gramática.

### 1.3 Contagem de declarações em lista (HEAD, todos os state dirs)

Script: `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/61b173ea-23ef-494e-b71b-e202101f3c46/scratchpad/hades-w0/count_decls2.py` (efêmero; `python3 -I` sobre `docs/roadmaps`, `date >= 2026-07-18`).

```
Roadmaps post-cutoff (todos os state dirs): 224
  done/: 208 | abandoned/: 11 | backlog/: 3 | wip/: 2
Roadmaps com ao menos uma declaração de Files affected: 101
Roadmaps SEM declaração:                               123
Total Files affected declarations:                     838
List-format declarations:                              194
Roadmaps com ao menos uma declaração em lista:          64
```

O arquiteto mediu 169 / 836 / 191 / 62 ao redigir a REQ (mesmo dia). Não foi possível reproduzir
169 a partir de uma contagem direta. A aritmética limita a discrepância: os 55 roadmaps extras
contribuem apenas 2 declarações adicionais (~0,04 cada). O sinal
de ~23% em lista é concordante; a diferença de +2 declarações não muda nenhuma conclusão.

### 1.4 Scan de Regra B sobre os 838 tokens

**(a) Tokens com stripped form vazio** (`strings.Trim(tok, "*?[]/") == ""`): **0**.
Zero falsos positivos neste corpus.

**(b) Tokens com caractere glob ou terminando em `/`**: **28**. Todos são prefixos de diretório
legítimos (`scripts/`, `generators/`), assinaturas com `[]string`, ou expressões de código.
Nenhum cobre o repositório inteiro.

**(c) Declarações inline seguidas de itens de lista**: **3**, todas em `done/`:
`ROADMAP-2026-10-08-push-e-ship-...:29`, `ROADMAP-2026-09-22-reconciliacao-...:136` e `:142`.
Todas existiam em `done/` antes do merge-base desta branch → somente orquestração na união.
Sob o novo parser, o comportamento seria aditivo (tokens inline + tokens dos itens de lista).
Ver p.9 e p.11 para cobertura de teste.

**(d) Com `.` adicionado ao cutset**: **1** (`.*` num roadmap de done/ — dotfiles legítimo).
`.` não deve ser adicionado. `**/*.*` está no Residual 4.

**(e) Sub-itens (itens com espaço à esquerda em blocos `Files affected`)**: **0** no corpus.
`TrimSpace` unifica qualquer item indentado com o item normal. Nenhuma mudança de comportamento.

### 1.5 Verificação ao vivo

```bash
$ bin/trackfw barrier docs/roadmaps/wip/ROADMAP-2026-10-28-documentacao-... --wave 1 --trust-local-gates
ml_scope: blocked — 13 arquivos out of scope
result: blocked  (rc=1)
```

Binário: `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/61b173ea-23ef-494e-b71b-e202101f3c46/scratchpad/hades-w0/trackfw` (efêmero — compilado de `./cmd/trackfw` com `go build` no commit `a48d2e26`). ML-2A deve recompilar a partir do binário da branch ao reimplementar.

> **Seção 1 concluída com evidência:** `git grep` executado (294 linhas), lista fechada com
> resultado da busca, corpus medido com script concreto (224 roadmaps, 838 declarações, 194 em
> lista), cinco variantes de scan executadas com resultado numérico, binário rodado ao vivo.

---

## 2. Modelo de ameaça

O adversário é o **arquiteto apressado e o implementador otimista**. O controle a proteger:
`barrier ml_scope` aprova apenas arquivos cobertos por declaração explícita nos roadmaps da branch.

### 2.1 Regra de crases por item (ação 2)

**Caso 1: npm/README.md e pypi/README.md** — ML-1B item 4 do roadmap de documentação:
```
- `npm/README.md` e `pypi/README.md`: removidos do git, porque a Wave 0 mediu que o release
  sobrescreve os dois (`release.yml:237` e `:309`)
```

`npm/README.md` está na primeira crase, `pypi/README.md` na segunda. Ambos estão no diff
(`git diff --name-only $(git merge-base origin/main HEAD) HEAD | grep README`).

**Regra A — só a primeira crase:** `pypi/README.md` não é extraído → barrier o bloqueia.
**Regra A viola AC8. Descartada.**

**Caso 2: `pypi/pyproject.toml` fica** — ML-1B item final (atenção à estrutura do item):
```
- **`pypi/pyproject.toml` fica.** A Wave 0 mediu consumidores vivos: `internal/commands/release.go`
  (`extractPyprojectVersion`), `scripts/check-manifest-version-gate.sh`,
  `scripts/check-integration-assets.sh` e `scripts/check-gates-falsify.sh`
```

Este item começa com `- ` (marcador de lista) seguido de `**` (bold de ênfase dentro do item).
A sequência `- **` é um item de lista cujo conteúdo começa com bold — não é um campo de parada
(que seria uma linha começando com `**Actions:**` ou `**Files affected:**` sem o `- ` anterior).
O parser correto NÃO para aqui.

Tokens extraídos por Regra B (item + wrap lines):
`pypi/pyproject.toml`, `internal/commands/release.go`, `extractPyprojectVersion`,
`scripts/check-manifest-version-gate.sh`, `scripts/check-integration-assets.sh`,
`scripts/check-gates-falsify.sh`.

Verificado no diff: **nenhum destes** está no diff. O over-grant é **inerte**. Este é o caso
que documenta o Residual 1: crase em texto de contexto.

**Regra B — todos os tokens entre crases (item + wrap lines):**
- Extrai `pypi/README.md` (caso 1) → AC8 passes. ✓
- Over-grant inerte no caso 2. ✓

**Itens com bold no início (`- **`):** 7 ocorrências no corpus (`git grep -c "^- \*\*" docs/roadmaps`). O parser deve tratar `- **` como item de lista (o `**` vem depois de `- `, portanto não é um campo de parada). `TestMLScopeFilesAffectedList_StopsAtBoldField` deve ter um contra-braço com o item do caso 2: o item `- **\`pypi/pyproject.toml\` fica.**...` deve ser extraído (não parar a lista).

**Sub-itens indentados:** 0 no corpus. `TrimSpace` no início de cada linha os trata como itens normais.

**Decisão: Regra B (todos os tokens do item e de suas wrap lines).**

### 2.2 Glob largo — quatro famílias cobrem o repositório inteiro

Medição: `mlScopeMatchPath` / `mlScopeMatchDoublestar` de `barrier.go`, executadas em
`broad_glob.go` (scratchpad efêmero). Confirmados contra `bin/trackfw` para `**` e `**/*`.

**Regra:** `strings.Trim(tok, "*?[]/") == ""` (sem parte literal).

| Padrão | stripped form | `internal/b.go` | `go.mod` | `README.md` | Ação |
|---|---|---|---|---|---|
| `**` | `""` | true | true | true | REJEITAR |
| `**/*` | `""` | true | true | true | REJEITAR |
| `*/**` | `""` | true | true | true | REJEITAR |
| `**/**` | `""` | true | true | true | REJEITAR |
| `**/` | `""` | false | false | false | REJEITAR (inócuo mas sem parte literal) |
| `*` | `""` | false | true | true | REJEITAR (top-level) |
| `/` | `""` | false | false | false | REJEITAR (inócuo mas sem parte literal) |
| `site/**` | `"site"` | false | false | false | Aceitar |
| `**/*.go` | `"go"` | true | false | false | Aceitar |
| `internal/**/*.go` | `"internalgo"` | true | false | false | Aceitar |

**`**/*.*`:** stripped form = `"."`. Não rejeitado. Casa qualquer arquivo com extensão. Residual 4.

**Corpus:** 0 tokens broad em 838 declarações (§1.4a). Zero falsos positivos.

**Prescrição (p.3):** se `strings.Trim(tok, "*?[]/") == ""`: status `blocked`, mensagem:
`"rejected: pattern without literal path segments (e.g. \"**\") matches every or no file without restriction"`.

### 2.3 Roadmap commitado só para ampliar escopo

Roadmap em `wip/` com `internal/` como directory prefix → barrier de outro roadmap da branch
passa para todos os arquivos sob `internal/`. Controle: K captura expansão acidental (waves ⬜
→ zero padrões). Residual 3: committer que flipa ML-0A para ✅ no mesmo commit derrota o filtro.

### 2.4 Roadmap tocado por rename trivial

`backlog/ROADMAP-2026-10-18-...` → `abandoned/ROADMAP-2026-10-18-...`. Com `--no-renames`:
origem ausente em HEAD → filtrada; destino → membro `abandoned/` → somente orquestração. Correto.

### 2.5 `req:` e `adr:` não validados contra prefixos — mesmo mecanismo, mesma função

`buildMLScopeOrchestrationSet` (`barrier.go:954-966`) adiciona `req:` e `adr:` sem verificação.
`req: "go.mod"` concede `go.mod` como orquestração (reproduzido ao vivo).

**Conjunto de orquestração fixo (hardcoded):** `barrier.go:909-919` adiciona ao set de CADA
roadmap: `docs/agents-working-context/`, `vault/notes/`, `.claude/agent-memory/`, e
`<rdDir>/.trackfw-log-entries/`, `<reqDir>/.trackfw-log-entries/`. Esses caminhos NÃO precisam
de validação de prefixo porque são hardcoded — não vêm de frontmatter.

**Corpus de req: fora de `docs/req/`:** 14 roadmaps (todos done/ ou abandoned/ no HEAD — zero
FPs nesta branch). Se reabertos (Regra Dura é rotina), perdem o grant de req: e adr: →
exigem correção manual.

**Correção prescrita (p.8):** para `req:` e `adr:`:
1. `cleaned = path.Clean(filepath.ToSlash(raw))`
2. Rejeitar se `path.IsAbs(cleaned)` ou `strings.HasPrefix(cleaned, "..")`
3. `req:`: aceitar só se `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(cfg.REQDir))+"/")`
4. `adr:`: aceitar só se `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(dir))+"/")`
   para algum `dir` em `cfg.ADRDirs`
5. Discard silencioso fora dos prefixos.

**Normalizar o config antes do `+"/"`:** se `cfg.REQDir = "docs/req/"` (trailing slash) então
`cfg.REQDir+"/"` = `"docs/req//"` → `HasPrefix("docs/req/REQ.md", "docs/req//")` = false —
discard de todos os REQs legítimos. O `path.Clean` no passo 3 previne isso.

Vetores: `docs/req-x/f.md` (sibling sem barra no prefixo), `docs/req/../../go.mod`
→ `path.Clean` → `go.mod` (fora do prefixo), `docs/req/../go.mod` → `docs/go.mod` (fora),
`docs/req/REQ-legit.md` (correto).

Campos reais: `cfg.REQDir = "docs/req"`, `cfg.ADRDirs = ["docs/adr"]`
(`internal/config/config.go:50-51`).

### 2.6 Lista que "vaza" para critérios de aceite

`**Acceptance criteria:**` como campo bold → stop em `**` bloqueia. Belt-and-suspenders:
stop em `- [ ]`/`- [x]`/`- [X]` ANTES do stop genérico em `- `.
Não usar `- [` — coincide com links Markdown `- [texto](url)`.
Corpus: 0 Markdown links em itens de `Files affected`.

### 2.7 Roadmaps uncommitted não entram na união

Três fatos separados:
- **Seleção de membros:** somente `barrierGitDiffFiles` (diff `mergeBase..HEAD --no-renames`). Roadmaps editados no disco mas ausentes do diff não entram. AC5.
- **Arquivos verificados:** diff (`barrierGitDiffFiles`) + working tree (`barrierGitStatusFiles` — `git status --porcelain=v1 -z --untracked-files=all --no-renames`, `barrier.go:717-722`). `docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md` chega por esta via (`??` em status).
- **Escopo:** lido de HEAD via `git show HEAD:<path>`. Roadmap presente no disco mas ausente do HEAD: skip.

### 2.8 Roadmap reaberto (done→wip) nesta branch

K = todas as waves históricas. Contribui escopo completo. Correto para Regra Dura. Residual 5.

### 2.9 Recorte de waves dos outros roadmaps da união (ação 4)

**Roadmap alvo:** waves 0..W (onde W = argumento `--wave`). **O roadmap alvo é excluído do loop de membros** — sua presença no diff não o torna membro; ele entra como alvo, não como contribuidor de K. Sem essa exclusão, após o próprio ML-2B do alvo ir para 🔄, K do alvo seria 2 como membro e arquivos de Wave 2 passariam num barrier `--wave 1`.

**Outros membros — K:**

K = número da wave mais alta com ao menos um ML onde:
```
roadmapdoc.StatusIsComplete(m)                       // ✅, done, concluido
  OR roadmapdoc.StatusCategory(m) == StatusTerminated  // ❌ Cancelado, 🚫 Abandonado, "abandonado"
  OR (roadmapdoc.StatusCategory(m) == StatusPending    // 🔄 Em andamento, ❌ Bloqueado
      AND firstToken ∈ {🔄, ❌})
```

Verificação em `internal/roadmapdoc/roadmapdoc.go:180-241`:
`StatusPending` cobre `{⬜, 🔄, ❌ Bloqueado}`; `StatusTerminated` cobre `{🚫, ❌ Cancelado, "abandonado"}`.
- `❌ Cancelado`: StatusTerminated → K.
- `🚫 Abandonado`: StatusTerminated → K.
- `🔄 Em andamento`: StatusPending, first token `🔄` → K.
- `❌ Bloqueado`: StatusPending, first token `❌` → K.
- `⬜ Pendente`: StatusPending, first token `⬜` → fora de `{🔄, ❌}` → não conta. K não avança.

K indefinido (nenhum ML iniciado) → somente orquestração.

**Membro pré-cutoff** (date: < 2026-07-18): somente orquestração. Usar o mesmo predicado do
`checkMLScope` que aplica ao roadmap alvo (`barrier.go:1035`) — não hardcodar uma segunda
constante `2026-07-18`.

**Caso desta branch:**

| Membro da união | Estado | K | Contribui |
|---|---|---|---|
| ROADMAP-2026-10-28 (wip) | Wave 0+1: ML-0A/1A/1B ✅; Wave 2+: ⬜ | K=1 | Padrões waves 0..1 |
| ROADMAP-2026-10-29 (wip, ALVO) | — | excluído do loop | waves 0..W (alvo) |
| ROADMAP-2026-10-18 (abandoned) | — | — | Somente orquestração |

### 2.10 Tabela de cobertura AC8 — duas invocações

**Dependência de ordem (crítica):** `barrier.go`, `barrier_ml_scope_test.go`, `docs/cli-parity.md`
e `scripts/check-barrier.sh` entram no escopo quando ROADMAP-2026-10-29 tiver ML-1A com K≥1
(status `🔄` ou melhor) commitado em HEAD. O gate AC8 pode rodar com esses arquivos ainda
uncommitted — `checkMLScope` usa `barrierGitStatusFiles` (status porcelain) para eles e os checa
contra o escopo de HEAD. **Nunca marcar ✅ para passar o AC8.** O status correto para rodar o
gate é `🔄 Em andamento`.

**Invocação 1:** `barrier ROADMAP-2026-10-28-documentacao --wave 1 --trust-local-gates`

(Prevista — a ser executada após o ML-1A implementado e commitado com K≥1; necessária para AC8.)

| Arquivo | Coberto por | Wave/ML | Formato |
|---|---|---|---|
| `scripts/install.ps1` | ROADMAP-2026-10-28 ML-1A | Wave 1 | Lista |
| `scripts/check-install-ps1.ps1` | ROADMAP-2026-10-28 ML-1A | Wave 1 | Lista |
| `.goreleaser.yaml` | ROADMAP-2026-10-28 ML-1A | Wave 1 | Lista |
| `.github/workflows/quality.yml` | ROADMAP-2026-10-28 ML-1A | Wave 1 | Lista |
| `pypi/scripts/build_wheel.py` | ROADMAP-2026-10-28 ML-1B | Wave 1 | Lista |
| `scripts/check-wheel-metadata-readme.sh` | ROADMAP-2026-10-28 ML-1B | Wave 1 | Lista |
| `Makefile` | ROADMAP-2026-10-28 ML-1B | Wave 1 | Lista |
| `npm/README.md` | ROADMAP-2026-10-28 ML-1B (1ª crase) | Wave 1 | Lista / Regra B |
| `pypi/README.md` | ROADMAP-2026-10-28 ML-1B (2ª crase) | Wave 1 | Lista / Regra B |
| `.gitignore` | ROADMAP-2026-10-28 ML-1B | Wave 1 | Lista |
| `docs/adr/ADR-2026-10-29-...` | ROADMAP-2026-10-29 orch. (adr:) | Wave 0 (orch.) | — |
| `docs/req/REQ-2026-10-29-...` | ROADMAP-2026-10-29 orch. (req:) | Wave 0 (orch.) | — |
| `docs/roadmaps/wip/ROADMAP-2026-10-29-...` | ROADMAP-2026-10-29 orch. (próprio) | Wave 0 (orch.) | — |
| `docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md` | ROADMAP-2026-10-29 ML-0A | Wave 0 | Inline |
| `internal/commands/barrier.go` | ROADMAP-2026-10-29 ML-1A | Wave 1 (K≥1) | Inline |
| `internal/commands/barrier_ml_scope_test.go` | ROADMAP-2026-10-29 ML-1A | Wave 1 (K≥1) | Inline |
| `docs/cli-parity.md` | ROADMAP-2026-10-29 ML-1A | Wave 1 (K≥1) | Inline |
| `scripts/check-barrier.sh` | ROADMAP-2026-10-29 ML-1A | Wave 1 (K≥1) | Inline |

`pypi/README.md` (linha 9) seria missed por Regra A — confirma necessidade de Regra B.

**Invocação 2 (análise complementar):** `barrier ROADMAP-2026-10-29-barrier-ml-scope --wave 1`

Diff: `git diff --no-renames --name-only $(git merge-base origin/main HEAD) HEAD` → 27 arquivos +
`docs/seguranca/2026-10-29-wave0-barrier-ml-scope.md` via status (`??`) = 28 verificados.

| Arquivo | Coberto por | Mecanismo |
|---|---|---|
| `.claude/agent-memory/zeus-tf/...` (2) | Hardcoded: `.claude/agent-memory/` | `barrier.go:914` |
| `.github/workflows/quality.yml` | ROADMAP-2026-10-28 ML-1A wave 1 | Lista |
| `.gitignore` | ROADMAP-2026-10-28 ML-1B wave 1 | Lista |
| `.goreleaser.yaml` | ROADMAP-2026-10-28 ML-1A wave 1 | Lista |
| `Makefile` | ROADMAP-2026-10-28 ML-1B wave 1 | Lista |
| `docs/adr/ADR-2026-10-28-...` | ROADMAP-2026-10-28 orch. (adr:) | req: → adr: |
| `docs/adr/ADR-2026-10-29-...` | ROADMAP-2026-10-29 orch. (adr:) | req: → adr: |
| `docs/agents-working-context/20261028T...` | Hardcoded: `docs/agents-working-context/` | `barrier.go:912` |
| `docs/req/REQ-2026-10-28-...` | ROADMAP-2026-10-28 orch. (req:) | req: |
| `docs/req/REQ-2026-10-29-...` | ROADMAP-2026-10-29 orch. (req:) | req: |
| `docs/roadmaps/.trackfw-log-entries/...` (3) | Hardcoded: `<rdDir>/.trackfw-log-entries/` | `barrier.go:916-917` |
| `docs/roadmaps/backlog/ROADMAP-2026-10-18-...` (deletado) | ROADMAP-2026-10-28 ML-0A inline + abandoned sibling | roadmap renomeado nesta branch |
| `docs/roadmaps/abandoned/ROADMAP-2026-10-18-...` | abandoned member orch. | sibling states |
| `docs/roadmaps/wip/ROADMAP-2026-10-28-...` | ROADMAP-2026-10-28 orch. (próprio) | roadmapRelPath |
| `docs/roadmaps/wip/ROADMAP-2026-10-29-...` | ROADMAP-2026-10-29 orch. (alvo próprio) | target path |
| `docs/seguranca/2026-10-28-wave0-...md` | ROADMAP-2026-10-28 ML-0A wave 0 | Inline |
| `docs/seguranca/2026-10-29-wave0-...md` | ROADMAP-2026-10-29 ML-0A wave 0 | Inline |
| `npm/README.md` | ROADMAP-2026-10-28 ML-1B wave 1 | Regra B |
| `pypi/README.md` | ROADMAP-2026-10-28 ML-1B wave 1 | Regra B |
| `pypi/scripts/build_wheel.py` | ROADMAP-2026-10-28 ML-1B wave 1 | Lista |
| `scripts/check-install-ps1.ps1` | ROADMAP-2026-10-28 ML-1A wave 1 | Lista |
| `scripts/check-wheel-metadata-readme.sh` | ROADMAP-2026-10-28 ML-1B wave 1 | Lista |
| `scripts/install.ps1` | ROADMAP-2026-10-28 ML-1A wave 1 | Lista |
| `vault/notes/index.md` | Hardcoded: `vault/notes/` | `barrier.go:913` |
| `vault/notes/powershell-funcao-...` | Hardcoded: `vault/notes/` | `barrier.go:913` |
| (após ML-1A) `internal/commands/barrier.go` | ROADMAP-2026-10-29 ML-1A wave 1 | Inline |
| (após ML-1A) `internal/commands/barrier_ml_scope_test.go` | ROADMAP-2026-10-29 ML-1A wave 1 | Inline |
| (após ML-1A) `docs/cli-parity.md` | ROADMAP-2026-10-29 ML-1A wave 1 | Inline |
| (após ML-1A) `scripts/check-barrier.sh` | ROADMAP-2026-10-29 ML-1A wave 1 | Inline |

**Invocação 2 não requer precondição de K≥1:** ROADMAP-2026-10-29 é o ALVO (não entra no loop de
membros); ROADMAP-2026-10-28 já tem K=1 (ML-1A e ML-1B ✅). Os 27 arquivos do diff + 1 untracked
= 28 verificados hoje estão cobertos; os 4 de implementação (linhas "(após ML-1A)") entrarão no
diff após ML-1A e estão cobertos via ROADMAP-2026-10-29 ML-1A wave 1 inline.

> **Seção 2 concluída com evidência:** Regra B decidida com dois casos reais (pypi/README.md e
> pyproject.toml medidos contra diff), 4 famílias broad medidas ao vivo (broad_glob.go + binário),
> req:/adr: bypass reproduzido ao vivo, K definido com vocabulário e enum lidos de
> `roadmapdoc.go:180-241` (StatusPending/StatusComplete/StatusTerminated verificados, ❌ Bloqueado
> = StatusPending confirmado, ❌ Cancelado = StatusTerminated confirmado), hardcoded set lido de
> `barrier.go:909-919`, tabelas de invocação 1 e 2 verificadas arquivo a arquivo com mecanismo concreto.

---

## 3. Alvos de falsificação nas duas direções

### Superfície A: parser de lista

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: lista = zero padrões | Reintroduzir `break` em `- ` | `pypi/README.md` fica out-of-scope com fixture de ML-1B | `TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect` |
| FP: lista engole o resto | Remover stop em `**` / blank | Tokens de `**Actions:**` viram padrões | `TestMLScopeFilesAffectedList_StopsAtBoldField` |
| FP: critério vaza | Remover stop em `- [x]`/`- [ ]` | Token de critério de aceite vira padrão | `TestMLScopeFilesAffectedList_CheckmarkItemStops` |

### Superfície B: glob largo

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: `**` aceito | Remover rejeição de stripped-form-vazio | Qualquer arquivo passa com `**` | `TestMLScopeGlobBroadPatternBlocked` |
| FN: `*/**` aceito | Cobrir só `**` e `**/*` | Arquivo nested passa com `*/**` | (vetor adicional no mesmo teste) |
| FP: `**/*.go` rejeitado | Rejeitar qualquer padrão com `**` | `site/**` produz `blocked` indevido | `TestMLScopeGlobPrefixedPatternAccepted` |

### Superfície C: união de roadmaps

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: só um roadmap | Remover lógica de união | Arquivo do roadmap 2 é out-of-scope no barrier do roadmap 1 | `TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope` |
| FP: roadmap uncommitted entra | Usar statusFiles em vez de diff | Roadmap editado no disco amplia escopo | `TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit` |

### Superfície C2: alvo excluído do loop de membros

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: alvo tratado como membro | Incluir alvo no loop | Após ML-2B do alvo ir para 🔄, arquivos de wave 2 passam em barrier de wave 1 | `TestMLScope_TargetRoadmapExcludedFromMemberLoop` |

### Superfície D: recorte de waves

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: todas as waves incluídas | Usar todas as waves sem K | Roadmap todo-⬜ expande escopo | `TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing` |
| FP: K muito restrito | K sempre = 0 | Wave 0 ✅ de union member não contribui | `TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns` |

### Superfície E: filtro done/-at-merge-base

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FP: done/ histórico contribui padrões | Não filtrar done/ pré-existente | Roadmaps históricos done/ entram em toda branch | `TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly` |
| FN: done/ novo nesta branch não contribui | Filtrar todos os done/ | Roadmap wip→done nesta branch não contribui | `TestMLScope_UnionDoneMovedInBranchContributesPatterns` |

### Superfície F: req:/adr: prefix check

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: `go.mod` entra | Não validar prefixo | `req: "go.mod"` concede `go.mod` | `TestMLScope_ReqOutsideReqDirDiscarded` |
| FN: traversal `../../go.mod` entra | Checar raw antes de limpar ("check raw, store cleaned") | `req: "docs/req/../../go.mod"`: raw passa HasPrefix `"docs/req"` mas cleaned = `go.mod` | (vetor adicional — clean ANTES de checar prefixo) |
| FN: sibling `req-x/` entra | HasPrefix sem `"/"` | `req: "docs/req-x/f.md"` passa com prefixo `"docs/req"` | (vetor adicional) |
| FN: adr: fora de adr_dirs | Não validar adr: | `adr: "go.mod"` concede `go.mod` | `TestMLScope_AdrOutsideADRDirsDiscarded` |
| FP: REQ legítimo descartado | `cfg.REQDir` com trailing slash → `"docs/req//"` | `docs/req/REQ-legit.md` rejeitado | (vetor adicional — normalizar config com `path.Clean`) |
| FP: `./docs/req/REQ-x.md` descartado | Clean produz path válido mas não reconhecido | `./docs/req/REQ-x.md` → `path.Clean` → `docs/req/REQ-x.md` → deve ser ACEITO | (vetor adicional — `./ ` prefix deve ser aceito após clean) |

### Superfície G: membro pré-cutoff

| Direção | O que regride | Teste | Nome |
|---|---|---|---|
| FN: pré-cutoff contribui padrões | Não checar date do membro | Roadmap de 2026-06-01 expande escopo | `TestMLScope_UnionPreCutoffMemberOrchestrationOnly` |
| FP: membro sem date: rejeitado | Tratar missing date como pré-cutoff | Roadmaps novos sem frontmatter date ficam com K=0 | `TestMLScope_UnionMissingDateTreatedAsPostCutoff` |

### Sabotagem obrigatória

Reverter `break` em `- ` → reprova `TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect`.
Remover a união → reprova `TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope`.

> **Seção 3 concluída com evidência:** 8 superfícies, 21 combinações FN/FP, mecanismo de
> regressão e nome de teste escritos para cada. As duas sabotagens obrigatórias identificadas
> por nome (exigidas no AC do ML-1A).

---

## 4. Residual declarado

1. **Itens de contexto expandem escopo (Regra B).** Crase em prosa ("consumidor: `a.sh`") adiciona
   `a.sh` ao escopo. Medido no caso `pypi/pyproject.toml` fica: 6 tokens extras, todos inertos
   (ausentes do diff). Sem NLP não há distinção automática. Aceito.

2. **req:/adr: concede qualquer path sob o prefixo.** Todo path dentro de `docs/req/` ou
   `docs/adr/` pode ser concedido como orquestração sem verificação de conteúdo. 14 roadmaps
   históricos com `req:` fora de `docs/req/` perdem o grant ao serem reabertos; exigem correção
   manual. Aceito.

3. **Filtro K não captura expansão deliberada.** Committer que flipa ML-0A de ⬜ para ✅ no mesmo
   commit derrota o filtro. K protege contra expansão acidental (waves futuras ⬜ → zero padrões).
   D2 confia em quem commita. Aceito.

4. **`**/*.*` e character classes não são rejeitados.** `**/*.*` tem stripped form `"."`.
   `[a-z]*/**` também sobrevive. A regra é tripwire, não closure. Aceito.

5. **Roadmap reaberto contribui escopo histórico completo.** K = todas as waves históricas. Aceito.

6. **done/ pré-existente no merge-base: somente orquestração.** Correto por construção. Aceito.

### 4.1 Prescrições para o ML-1A

1. **Parser em lista.** Em `parseMLScopeFilesAffected`: aceitar `- ` e `* ` como início de item.
   Extrair tokens por Regra B (todos entre crases no item + wrap lines). Parar em: linha em branco,
   linha que começa com `**` (sem `- ` precedente), linha que começa com `#`, cerca de código.
   Stop em `- [ ]`/`- [x]`/`- [X]` ANTES do stop genérico em `- `.
   Não usar `- [` — coincide com links Markdown.

2. **Regra B uniforme.** Parser de lista usa a mesma primitiva de extração de tokens que o parser
   inline existente.

3. **Rejeitar tokens com stripped form vazio.** Se `strings.Trim(tok, "*?[]/") == ""`: status
   `blocked`, mensagem:
   `"rejected: pattern without literal path segments (e.g. \"**\") matches every or no file without restriction"`.

4. **União de roadmaps.** Membros via `barrierGitDiffFiles` (diff `mergeBase..HEAD`). Para cada
   path: verificar existência em HEAD (`git show HEAD:<path>`); verificar state dir
   (`config.RoadmapStates()`). Path ausente em HEAD: skip.

5. **Escopo por estado do membro.**
   - `abandoned/` ou `backlog/`: somente orquestração.
   - `done/` E existia em done/ no merge-base: somente orquestração.
   - `date:` anterior a 2026-07-18 (usar predicado existente de `barrier.go:1035`): somente
     orquestração.
   - `date:` ausente: o predicado `barrier.go:1035` é `if date != "" && isPreMLScopeCutoff(date)` —
     condição false quando `date == ""` → tratado como pós-cutoff (scope permitido). Comportamento
     correto: roadmaps sem frontmatter date são pós-cutoff por omissão.
   - Demais: orquestração + padrões de waves 0..K.

6. **K — match positivo.** K = número da wave mais alta com ao menos um ML onde:
   ```
   roadmapdoc.StatusIsComplete(m)                         // ✅, done, concluido
     OR roadmapdoc.StatusCategory(m) == StatusTerminated  // ❌ Cancelado, 🚫 Abandonado, "abandonado"
     OR (roadmapdoc.StatusCategory(m) == StatusPending    // 🔄 Em andamento, ❌ Bloqueado
         AND firstToken ∈ {🔄, ❌})
   ```
   `⬜ Pendente` tem first token `⬜` → fora de `{🔄, ❌}` → não avança K.
   K indefinido → somente orquestração.

7. **Alvo excluído do loop de membros.** O roadmap alvo (passado a `checkMLScope`) não entra no
   loop de membros; contribui waves 0..W como alvo (W = argumento `--wave`).

8. **Orquestração com validação de prefixo (req: e adr:).** Para `req:` e `adr:`:
   (a) `cleaned = path.Clean(filepath.ToSlash(raw))`
   (b) Rejeitar se `path.IsAbs(cleaned)` ou `strings.HasPrefix(cleaned, "..")`
   (c) `req:`: aceitar só se `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(cfg.REQDir))+"/")`
   (d) `adr:`: aceitar só se `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(dir))+"/")`
       para algum `dir` em `cfg.ADRDirs`
   (e) Discard silencioso.

9. **`docs/cli-parity.md`** — somente a seção `ml_scope`: atualizar gramática (lista, paradas,
   Regra B, broad tokens, união). Incluir comportamento híbrido inline+lista (§1.4c). Nenhuma
   outra seção.

10. **`scripts/check-barrier.sh`** — nenhuma mudança neste ML.

11. **Testes obrigatórios em `barrier_ml_scope_test.go`** (repos via `t.TempDir()`):
    - `TestMLScopeFilesAffectedList_BreakAtListItemReproducesDefect`
    - `TestMLScopeFilesAffectedList_AllTokensExtracted`
    - `TestMLScopeFilesAffectedList_StopsAtBoldField`
    - `TestMLScopeFilesAffectedList_CheckmarkItemStops`
    - `TestMLScopeFilesAffectedHybridInlineAndList` ← §1.4c
    - `TestMLScopeGlobBroadPatternBlocked` (vetores: `**`, `**/*`, `*/**`, `**/**`, `*`, `**/`, `/`)
    - `TestMLScopeGlobPrefixedPatternAccepted` (vetores: `site/**`, `**/*.go`)
    - `TestMLScope_UnionTwoRoadmapsFilesFromOtherRoadmapInScope`
    - `TestMLScope_UnionIgnoresWorkingTreeRoadmapEdit`
    - `TestMLScope_TargetRoadmapExcludedFromMemberLoop`
    - `TestMLScope_UnionStartedWavesOnly_AllPendingContributesNothing`
    - `TestMLScope_UnionStartedWavesOnly_DoneWaveContributesPatterns`
    - `TestMLScope_UnionAbandonedContributesOrchestrationOnly`
    - `TestMLScope_UnionDoesNotIncludeUncommittedRoadmap`
    - `TestMLScope_UnionDoneAtMergeBaseOrchestrationOnly`
    - `TestMLScope_UnionDoneMovedInBranchContributesPatterns`
    - `TestMLScope_ReqOutsideReqDirDiscarded` (vetores: `go.mod`, `docs/req-x/f.md`, `docs/req/../../go.mod`, `./docs/req/REQ-x.md` deve ser ACEITO, trailing-slash config)
    - `TestMLScope_AdrOutsideADRDirsDiscarded`
    - `TestMLScope_UnionPreCutoffMemberOrchestrationOnly`
    - `TestMLScope_UnionMissingDateTreatedAsPostCutoff`

12. **AC5 preservado.** Roadmap presente no disco mas ausente do diff não entra na união.

> **Seção 4 concluída com evidência:** 6 residuais com aceitação explícita, 12 prescrições com
> código concreto (passos numerados, campos reais de config, linha de código), 20 testes nomeados
> cobrindo cada superfície de §3, dependência de ordem de commit documentada com K=🔄 como
> mínimo e proibição de marcar ✅ para passar AC8.
