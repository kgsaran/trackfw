# Wave 0 — Threat Model: `trackfw context` reporta ADRs (0) onde `status` reporta 145

> Data: 2026-09-29 | Hades | REQ-2026-09-29 | ADR-2026-09-29
> Branch: `fix/context-reporta-zero-adrs-onde-status-reporta-145`

---

## Seção 1 — Enumeração de completude

A ADR lista dois comandos afetados (`context` e `status`) e propõe um ponto único. Antes de
aceitar essa lista como fechada, enumerei todos os sítios do produto que leem ou contam ADRs.

### Sítios identificados

| # | Arquivo : Linha | Função | Mecanismo | Classe |
|---|---|---|---|---|
| S1 | `internal/generators/context.go:38-53` | `GetContext` (loop ADR) | `os.ReadDir(adrDir)` — lê **apenas a raiz** | **(iii) própria, ERRADA** |
| S2 | `internal/validator/validator.go:1419-1420` | `inventoryBlock` (`status`) | `walkADRFilePaths` → `filepath.WalkDir` recursivo | **(ii) própria, CORRETA** |
| S3 | `internal/validator/validator.go:2418-2419` | regra `adr_orphan` | `walkADRFilePathsForRule` → `filepath.WalkDir` recursivo | **(ii) própria, CORRETA** |
| S4 | `internal/validator/validator.go:3037-3053` | `findADRFile` | `filepath.WalkDir` recursivo | **(ii) própria, CORRETA** |
| S5 | `internal/serve/api_chain.go:88` | `chainHandler` (grafo) | `scanChainDir` → `filepath.WalkDir` recursivo | **(ii) própria, CORRETA** |
| S6 | `internal/generators/adr.go:189` | `ListADRs` (`adr list`) | `filepath.Glob(dir + "/*.md")` — lê **apenas a raiz** | **(iii) própria, ERRADA** |
| S7 | `internal/generators/adr.go:316` | `NewADRDraft` (dedup check) | `filepath.Glob(adrDir + "/ADR-*-slug.md")` — lê **apenas a raiz** | **(iii) própria, ERRADA** |
| S8 | `internal/serve/api_file.go:104,117` | `buildAllowedDirs` / `buildRealAllowedDirs` | usa `cfg.ADRDirs` para construir fronteira de contenção de acesso a arquivos — **não é enumerador** | fora de escopo |
| S9 | `internal/discover/discover.go:432,437` | `Scan` (`discover`) | `countMDFiles` → `filepath.WalkDir` recursivo | **(ii) própria, CORRETA** (pré-config, não consome `adr_dirs`) |

**Veredito de completude:** a lista da ADR (context + status) é incompleta. Há **três** sítios
classe (iii) — S1, S6 e S7 — não apenas um. S6 (`adr list`) produz zero ADRs para qualquer
usuário de layout com subpastas. S7 (`req new`) não consegue detectar rascunho duplicado quando
o twin vive numa subpasta de estado, e vai criar o arquivo novamente. A Regra Dura de Causa Raiz
(mesma causa, mesmo mecanismo) implica que S6 e S7 entram no ML-1A desta REQ, não em REQ nova.
A REQ e a ADR precisam ser atualizadas para cobrir os três sítios.

---

## Seção 2 — Modelo de ameaça

O adversário aqui não é um atacante externo: é o **implementador apressado** e o **arquiteto
otimista**. As ameaças emergem dos quatro falsos-negativo que os sítios errados já produzem ao vivo.

### T1 — Agente inicia trabalho sem conhecer as ADRs

**Superfície:** `trackfw context` (S1 — os.ReadDir raiz).
**Mecanismo:** o agente executa `trackfw context` (comando que a documentação instalada manda
rodar primeiro — "always run first"). Recebe `## ADRs (0) - (none)`. Conclui que o projeto não tem
decisões arquiteturais registradas. Propõe e implementa mudanças que violam ADRs existentes — sem
nenhum alerta, porque o dado faltante é apresentado como "ausência confirmada", não como erro.

**Evidência:**
```
$ cd fixture && trackfw context
## ADRs (0)
- (none)
...
## Warnings (4)
- adr "ADR-2026-09-01-teste.md" is not referenced by any REQ
- adr "ADR-2026-09-02-teste-b.md" is not referenced by any REQ
- adr "ADR-2026-09-04-teste-d.md" is not referenced by any REQ
- adr "ADR-2026-09-03-teste-c.md" is not referenced by any REQ
```

O mesmo comando, na mesma execução, **declara zero ADRs e nomeia quatro ADRs nos warnings**.

**Impacto:** violação arquitetural invisível. A pior forma de dado errado é aquela que induz
confiança na ausência.

### T2 — Score deflacionado passa gate que não deveria passar

**Superfície:** `GetContext` linha 122 — `if len(adrs) > 0 { score += 20 }` onde `adrs` vem do
enumerador errado de S1.

**Mecanismo:** qualquer repositório com ADRs exclusivamente em subpastas de estado reporta
`Governance score: X/100` onde X está deflacionado em exatamente 20 pontos. Um gate de CI que
verifica `score >= 80` passa para um repositório com score real 80 que aparece como 60.

**Evidência (fixture, 4 ADRs, status clean):**
- Antes do fix: `Governance score: 40/100` (sem ADRs = sem +20; REQs=0; roadmaps=0; clean=+40)
- Após o fix: `Governance score: 60/100`
- Delta: **exatamente 20 pontos**, nem mais nem menos

### T3 — `adr list` silencia subpastas, usuário presume ausência

**Superfície:** `ListADRs` (S6 — filepath.Glob raiz).

**Mecanismo:** `trackfw adr list` retorna "No ADRs found in docs/adr/zeus" mesmo com 4 ADRs no
disco. Um operador que invoca `adr list` para auditar decisões de um projeto com layout subpastas
conclui que não há ADRs registradas.

**Evidência:**
```
$ cd fixture && trackfw adr list
No ADRs found in docs/adr/zeus
```

### T4 — `NewADRDraft` cria rascunho duplicado invisível

**Superfície:** `NewADRDraft` (S7 — filepath.Glob raiz).

**Mecanismo:** `req new` chama `NewADRDraft(slug, adrDir)` que verifica existência de rascunho
via `filepath.Glob(adrDir + "/ADR-*-slug.md")`. Se o rascunho existente vive em `adrDir/wip/` (ou
qualquer subpasta), o Glob não o encontra, e o arquivo é criado de novo na raiz. Resultado: dois
ADR de rascunho para a mesma decisão — o original na subpasta (invisível), o novo na raiz (vazio).

**Linha:** `internal/generators/adr.go:316` — `matches, err := filepath.Glob(pattern)` onde
`pattern = filepath.Join(adrDir, "ADR-*-"+slug+".md")`.

---

## Seção 3 — Alvos de falsificação em ambas as direções

Para cada superfície: onde o sabotage entra, qual gate deveria pegar, e a qual direção —
falso-negativo (defeito escapa) e falso-positivo (gate reprova o que está certo).

### Superfície A — Enumerador de ADR em `context` (S1)

**Falso-negativo (defeito escapa):** um novo enumerador nasce com `os.ReadDir(adrDir)` ou
`filepath.Glob(adrDir+"/*.md")`. O gate é um script que varre `internal/` buscando chamadas
root-only dessas duas formas nos arquivos que consomem `adr_dirs`. O gate deve manter uma lista
de ocorrências permitidas **fixada por contagem**, não por offset. As três condições de
envelhecimento que devem acionar reavaliação: (a) arquivo de lista ausente, (b) contagem=0, (c)
contagem diverge do número real de ocorrências na varredura.

**Falso-positivo (gate reprova o certo):** o gate não deve sinalizar o resolvedor compartilhado
`ResolveADRFiles` (que usa `walkADRFilePaths`/WalkDir), nem `buildAllowedDirs`/`buildRealAllowedDirs`
em `api_file.go:104,117` — estes iteram `cfg.ADRDirs` para construir fronteira de segurança de
acesso a arquivos, não para enumerar conteúdo. A lista de permissão deve ter justificativa escrita
para cada entrada.

### Superfície B — `adr list` (S6) e `NewADRDraft` (S7)

**Falso-negativo:** estas funções continuam usando Glob raiz após a correção de S1/S2. O gate
de S1 deve cobrir S6 e S7 também — a mesma varredura por `filepath.Glob(filepath.Join(dir,` em
`internal/generators/adr.go` detecta os dois.

**Falso-positivo:** `filepath.Glob` em `adr.go` pode ter usos legítimos que não são enumeração
de ADRs existentes (ex.: verificar nomes de arquivos ao escrever). O gate deve ser preciso o
suficiente para não sinalizar usos não-enumeradores. Alternativa: o gate verifica o padrão
`"*.md"` ou `"ADR-*"` como argumento de Glob dentro das funções de leitura, não todo Glob.

### Superfície C — Double-count em `adr_dirs` aninhadas

**Falso-negativo:** após o fix de S1 (usar WalkDir para context), `adr_dirs: [docs/adr/zeus,
docs/adr/zeus/done]` passa a contar ADRs que estão em `zeus/done` **duas vezes** — uma pelo
WalkDir de `zeus` (que desce), uma pelo WalkDir de `zeus/done`. O mesmo double-count já existe
em `status` (S2) hoje — **medido na fixture**:

```
adr_dirs: [docs/adr/zeus, docs/adr/zeus/done]
status: ADRs 7  (4 do zeus + 3 do zeus/done = 7; zeus/done tem 3 arquivos)
```

O resolvedor `ResolveADRFiles(cfg)` deve deduplicar por caminho absoluto após `config.ExpandPath`.
Sem dedup, o fix de context herda o double-count já presente em status e os dois passam a concordar
no número errado. O AC da REQ "diferença tem que ser exatamente 20 pontos" pode passar mesmo com
double-count se o repositório de teste não tiver `adr_dirs` aninhadas.

**Falso-positivo:** a dedup por caminho absoluto não deve remover ADRs com nomes iguais em
diretórios distintos e não-aninhados — dois arquivos `ADR-2026-09-01-foo.md` em `docs/adr/zeus`
e `docs/adr/athena` são entidades distintas e devem ambos aparecer.

---

## Seção 4 — Residual declarado

Este Wave 0 **não cobre**:

1. **S8 (`api_file.go` — buildAllowedDirs):** consome `cfg.ADRDirs` exclusivamente para construir
   fronteira de contenção de acesso a arquivos no endpoint `serve`. Não é enumerador de ADRs. A
   semântica correta é "qualquer arquivo dentro de um dos `adr_dirs` está acessível via API de
   arquivo". Nenhuma mudança necessária neste sítio — e mudar seria risco de segurança.

2. **S9 (`discover/discover.go` — Scan):** ferramenta pré-config que varre o filesystem para
   sugerir `adr_dirs`. Hardcoda `docs/adr` e itera subpastas de forma correta (`countMDFiles`
   via WalkDir). Não consome `adr_dirs` configurado. Comportamento correto para seu propósito —
   sem mudança necessária.

3. **Dedup não implementado em `status` hoje:** o double-count com `adr_dirs` aninhadas já existe
   em S2 (`inventoryBlock`) antes desta REQ. O fix desta REQ não piora a situação — apenas não
   melhora. O dedup no resolvedor compartilhado fecha ambos; se o resolvedor for implementado sem
   dedup, `status` e `context` concordarão no número (ambos errado) para o caso aninhado.

4. **`adr list` e `NewADRDraft` (S6, S7) não entram no D3 do ADR tal como escrito:** o ADR nomeia
   `context` e `status` como os dois consumidores do ponto único. S6 e S7 consomem `ADRDirs[0]`
   (primeiro dir) via `resolveADRDir`, não o conjunto `cfg.ADRDirs`. O fix deles é aplicar o
   **primitivo** `walkADRFilePaths(dir)` (S2-class) no lugar de Glob — não o resolvedor nível-cfg.
   Este é um segundo sítio sobrando da mesma causa; a Regra Dura exige que entre no ML-1A, não
   em REQ nova, e o ADR precisa ser atualizado para nomear os três sítios.

---

## Análise de D3 — ponto único é viável, e a forma correta é dois estratos

**O que cada sítio precisa:**

| Sítio | Dado necessário | Usa `status:` do frontmatter? | Usa estado de pasta? |
|---|---|---|---|
| `status` (S2) | `len(paths)` — só contagem | Não | Não |
| `context` (S1) | `[]string{fullPaths}` + leitura de frontmatter para `status:` | Sim (por arquivo) | Não (ContextEntry.State é "somente ROADMAPs") |
| `validate` (S3) | `[]string{fullPaths}` para regra de órfão | Não | Não |
| `adr list` (S6) | `[]string{fullPaths}` — basta iterar | Sim (para exibição) | Não |

Os requisitos não divergem. A precedência já existe: `context.go:56` e `context.go:60` usam
`validator.ResolveREQFiles(cfg)` — o mesmo padrão aplicado a ADRs resolve D3.

**A forma correta (dois estratos):**

```
walkADRFilePaths(dir) []string          ← primitivo, um dir, já existe em S2-class
ResolveADRFiles(cfg)  []string          ← wrapper cfg: loop ADRDirs + dedup por caminho absoluto
```

- `status` usa `ResolveADRFiles(cfg)` — `len()` do resultado
- `context` usa `ResolveADRFiles(cfg)` — itera, lê frontmatter `status:` por arquivo
- `adr list` e `NewADRDraft` usam `walkADRFilePaths(resolveADRDir(scope))` — primitivo,
  diretório único, escopo resolvido separadamente (inclui `--scope global`)

Não há flags na interface. O `ContextEntry.State` não precisa de mudança — continua sendo
"somente ROADMAPs". **D3 é CONFIRMADA.**

---

## Incoerência encontrada na ADR — seção Consequências

A ADR afirma: *"o filtro por prefixo `ADR-` já é o usado pelo `status`"*.

Medido em `internal/validator/validator.go:3007-3022`:

```go
if !d.IsDir() && strings.HasSuffix(path, ".md") {
    paths = append(paths, path)
}
```

Não há filtro por prefixo `ADR-`. O mesmo vale para `context.go:43-44`. **Nenhum sítio filtra
por prefixo** — todos os arquivos `.md` na árvore de `adr_dirs` são contados.

Na prática, este repositório tem 0 arquivos não-`ADR-*` em `docs/adr` (medido: `find docs/adr
-maxdepth 1 -name "*.md" | grep -v "/ADR-" | wc -l` = 0), então o filtro ausente não é
observável aqui. Mas um repositório com `docs/adr/templates/template.md` teria esse arquivo
contado como ADR.

**Consequência para ML-1A:** o implementador não deve adicionar filtro por prefixo acreditando
que está preservando comportamento existente — o comportamento existente é **sem filtro**. Adicionar
um filtro mudaria contagens em repositórios com arquivos não-ADR em `adr_dirs` e seria invisível
nos testes deste projeto. O resolvedor compartilhado deve usar `strings.HasSuffix(path, ".md")`
sem prefixo, igual ao `walkADRFilePaths` de hoje.

A ADR deve ser corrigida nesse ponto antes da implementação.

---

## Resumo de divergência medida (fixture canônica)

Fixture: `adr_dirs: [docs/adr/zeus]`, ADRs em `docs/adr/zeus/done/` (3) e `docs/adr/zeus/wip/` (1).
ADRs no disco: 4.

| Comando | Contagem reportada | Mecanismo |
|---|---|---|
| `trackfw status` | **4** | `walkADRFilePaths` (WalkDir) — CORRETO |
| `trackfw context` | **0** | `os.ReadDir` (raiz apenas) — ERRADO |
| `trackfw adr list` | **0** ("No ADRs found") | `filepath.Glob("*.md")` (raiz apenas) — ERRADO |
| `trackfw validate` warnings | **4** (nomeados) | `walkADRFilePathsForRule` (WalkDir) — CORRETO |

Contradição interna da mesma execução de `context`: reporta `ADRs (0)` e simultâneamente nomeia
4 ADRs nos warnings — duas implementações da mesma pergunta, uma errada.

Score: 40/100 (atual) → 60/100 (após fix). Delta = 20 pontos, exatamente a categoria ADR.

---

## Layout plano (este repositório): não afetado

`adr_dirs: [docs/adr]`. `find docs/adr -type d` retorna apenas `docs/adr` — sem subpastas.
Ambos os comandos reportam 74 corretamente hoje. WalkDir num diretório sem subpastas é idêntico
a ReadDir — a correção não quebra o layout plano.

Porém: layout plano **com** subpasta não-ADR (ex. `docs/adr/templates/README.md`) veria a
contagem subir após o fix porque WalkDir desceria na subpasta. Sem filtro de prefixo, esse arquivo
seria contado como ADR. O residual é aceito — é o mesmo comportamento que `status` já tem — e
deve constar nas release notes.
