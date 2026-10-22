---
date: 2026-10-22
author: hades-tf
ml: ML-3B
roadmap: ROADMAP-2026-09-22-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md
---

# Red Team — log por entrada (ML-3B)

> Branch: `fix/trackfw-log-conflita-em-toda-branch-paralela`  
> HEAD: `e8014482d5f5da94c39acc51f99ecd515771af33`  
> Binário: `/private/tmp/claude-501/.../scratchpad/rt3b/tf` (`go build -buildvcs=false`)  
> Repositório temporário: `/private/tmp/claude-501/.../scratchpad/rt3b/repo1/`  
> (sem `.gitattributes`, `roadmap_dir: plan/rm`, `req_dir: plan/req`)

---

## Item 1 — Objetivo da REQ: merge limpo e leitores enxergam as duas branches

### Medição

**Setup:** repositório temporário com `trackfw.yaml` (`roadmap_dir: plan/rm`), sem `.gitattributes`.
Duas branches divergentes a partir do mesmo commit:

- `feat-a`: `tf roadmap move ROADMAP-2026-01-01-test-a.md wip` → cria
  `plan/rm/.trackfw-log-entries/20261022T222849Z-bef4cf64.entry`
- `feat-b`: `tf roadmap move ROADMAP-2026-01-02-test-b.md wip` → cria
  `plan/rm/.trackfw-log-entries/20261022T222849Z-5a5407f9.entry`
- Hashes diferentes → nomes diferentes → sem conflito.

```
# em feat-b, após commit:
GIT_CONFIG_NOSYSTEM=1 git merge feat-a -m "merge"
# → "Merge made by the 'ort' strategy."  rc=0
```

**Conjunto de entradas após o merge:**
```
20261022T222849Z-5a5407f9.entry   (feat-b, roadmap-b)
20261022T222849Z-bef4cf64.entry   (feat-a, roadmap-a)
```

**Leitores:**

| Comando | Resultado |
|---|---|
| `tf log` | Mostra as duas entradas. |
| `tf metrics` | Conta as duas como WIP (sem entries de done, cycle time = n/a). |
| `tf serve /api/metrics` | Retorna `{"state_distribution":{"wip":1,...}}` — serve usa `translog.ReadAll` via `serve/metrics_log.go:ParseLog`. |
| `tf validate` | Falha por regras de estrutura (sem REQ, sem AC) — regras de log não são o gatilho. |

**Discriminador de validate:** `latestWIPTransitionTime` (`validator.go:2879`) chama `translog.ReadAll(cfg.RoadmapDir)`. Com as duas entradas presentes, lê corretamente as duas. Com apenas uma, a outra não contribui para o cálculo de WIP age — o comportamento difere de forma mensurável.

**Braço negativo (formato antigo):**
```
Branch A: printf "..  ROADMAP-A.md  backlog → wip\n" >> .trackfw-log
Branch B: printf "..  ROADMAP-B.md  backlog → wip\n" >> .trackfw-log
git merge → CONFLICT (content): Merge conflict in plan/rm/.trackfw-log
```
Confirmado em repositório sem `.gitattributes` (mesmo setup do Teste 1 da Wave 0).

**Nota sobre REQ flat-layout:** `tf req move` em layout plano (`parentDir == reqDirClean`) retorna antes de chamar `appendREQTransitionLog` (`req.go:394-401`). Nenhuma entrada é gravada para REQ de layout plano. Comportamento pré-existente, não regressão.

**RESULTADO: PASS.** Merge limpo, conjunto A∪B intacto, todos os leitores enxergam as duas entradas.

---

## Item 2 — Versões misturadas (legado + novo)

### Medição

**Setup:** mesmo repositório, após merge. Inserido manualmente em `plan/rm/.trackfw-log`:
```
2026-10-22 15:00  ROADMAP-2026-01-01-test-a.md  backlog → wip  -0300  (legacy@test.com)
```

**`tf log` com legado + entradas:**
```
2026-10-22 15:00  ROADMAP-2026-01-01-test-a.md    backlog → wip   ← legado (sortKey: time.Local)
2026-10-22 19:29  ROADMAP-2026-01-03-done.md       backlog → done  ← entry (sortKey: UTC do nome)
2026-10-22 19:28  ROADMAP-2026-01-02-test-b.md     backlog → wip
2026-10-22 19:28  ROADMAP-2026-01-01-test-a.md     backlog → wip   ← duplicata do legado às 19:28
```

A transição do ROADMAP-A aparece duas vezes: uma no legado (15:00) e uma na entry (19:28). Sem dedup entre as duas fontes — comportamento correto conforme residual declarado D1/D2 da Wave 0.

**Ordenação:** legado às 15:00 local = 18:00 UTC. Entries às 19:28/19:29 UTC. Ordem cronológica correta.

**`discover --bootstrap-log` idempotente:**
- 1ª execução: "1 new entries" (entrada para `ROADMAP-2026-01-03-done.md`).
- 2ª execução: "0 new entries" — dedup via `readLogEntries` + reconstrução de chave por `t.Basename + t.From + t.To + t.Timestamp`. PASS.

**Fuso (residual declarado):** `parseLine` usa `time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)`.
O sufixo `-0300` na linha não é capturado por `lineRe`. Para projetos em fuso ≠ UTC, a ordem
relativa entre entradas legadas e entries pode ser imprecisa ao minuto, dependendo do TZ da máquina
leitora. Risco idêntico ao formato anterior; residual declarado no item 4 da Wave 0.

**Bug de bootstrap UTC — ordem de exibição no `tf log`:**
`discover.go:204` chama `time.ParseInLocation("2006-01-02 15:04", line[:16], time.UTC)` sobre uma
string no horário local (gerada por `GenerateBootstrapLog` que usa `time.Local`). O `entryTime`
passado a `translog.WriteEntry` é deslocado pelo offset de fuso — em `-0300`, o nome do arquivo
é 3h mais cedo do que deveria (ex: `20261022T150000Z` em vez de `20261022T180000Z`). O sort key
fica errado na exibição de `tf log`, fazendo entradas retroativas aparecerem fora de ordem.

**Escopo do bug:** apenas `tf log` (display order). `t.Timestamp` é sempre populado por `parseLine`
com `time.Local` — independente do nome do arquivo — e é o valor usado em `latestWIPTransitionTime`
(validator.go:2893) e `CycleTimeMean` (metrics.go:85). Medido: validate e metrics não são afetados.
O bug não flipa nenhum resultado de `tf validate` ou `tf metrics`.

**RESULTADO: PASS COM DUPLICATA ESPERADA E RESSALVA R4 (bootstrap sort key, ver veredito).**

---

## Item 2b — serve /api/metrics e chmod 000 no diretório de entradas

### 2b-1: serve ParseLog com symlink no diretório de entradas

`serve/metrics_log.go:ParseLog` delega a `translog.ReadAll`. Quando `ReadAll` retorna um erro
(ex: symlink no diretório), `ParseLog` captura o erro mas retorna `[]Transition{}` em vez de propagar:

```go
// serve/metrics_log.go:20
func ParseLog(dir string) []Transition {
    tls, err := translog.ReadAll(dir)
    if err != nil { return []Transition{} }   // silently empty
    ...
}
```

**Medição:**
```
# após ln -s /tmp plan/rm/.trackfw-log-entries
tf log          → Error: translog: ... is a symlink — rc=1  (fail-closed)
GET /api/metrics → {"lead_time_avg_days":0,"cycle_time_avg_days":0,...,"burndown":[]}  (rc=200)
```

`state_distribution` permanece correto (vem de varredura de diretórios de estado, não de translog).
`burndown` retorna `[]` e cycle time retorna 0 — dados parciais sem sinalização de erro.

Divergência: `tf log` é fail-closed; `tf serve /api/metrics` é fail-open silencioso. Operador
que usa apenas o board não percebe que o log de transições está inacessível.

### 2b-2: chmod 000 no diretório de entradas

```
tf log          → Error: ...permission denied  (rc=1)
tf validate     → ⚠ stale_wip: could not inspect "plan/rm": ...permission denied  (warning, não violação)
```

`latestWIPTransitionTime` propaga o erro como `stale_wip` warning via `inspectionDiagnostic`
(validator.go:2884→2862). Não causa pânico nem violação falsa; o operador vê o diagnóstico.
`tf serve /api/metrics` retorna estado parcial (mesmo comportamento do symlink acima).

**RESULTADO:**
- `tf log` e `tf validate`: fail-closed correto — erros visíveis.
- `tf serve /api/metrics`: fail-open silencioso — INFO, não bloqueante (mesmo operador pode monitorar via `tf validate`).

---

## Item 3 — Ataques ao diretório de entradas

**Matriz medida** — binário da árvore, repositório temporário.

### 3a — Symlink no lugar do diretório `.trackfw-log-entries/`

| Operação | Resultado |
|---|---|
| `tf log` (leitor) | `Error: translog: plan/rm/.trackfw-log-entries is a symlink — refusing to read` (rc=1) |
| `tf roadmap move` (escritor) | `trackfw: refusing write … (mode Lrwxr-xr-x)` via pathguard — roadmap é movido, entrada não é gravada |
| Arquivo criado fora da raiz | Não. O pathguard bloqueia antes do `WriteEntry`. |

Fail-closed para leitura; fail-open para o log de transição (a mudança de estado ocorre mas não é registrada). Pré-existente: a gravação do log já era não-fatal (`_ = appendTransitionLogEntry`). O reader retorna erro propagado — `tf log` falha com mensagem clara.

### 3b — Arquivo `.entry` que é symlink

Link simbólico apontando para arquivo com conteúdo válido fora do projeto:
```
de.Type()&os.ModeSymlink != 0  →  silently skipped
```
Conteúdo externo **não** aparece em `tf log`. PASS.

### 3c — Arquivo `.entry` gigante / sparse

| Variante | Resultado |
|---|---|
| 65 KB real (`\x00` após a linha válida) | Primeira linha válida extraída; nulos ignorados. rc=0. |
| 1 GB sparse (macOS APFS) | `io.ReadAll` lê os dados esparsamente — rc=0, tempo < 1s. |

`ReadAll` sem limite de tamanho: arquivo real não-sparse de 1GB alocaria 1GB de heap. Requer acesso de escrita ao diretório — superfície idêntica ao formato legado (sem limite também). Ressalva, não bloqueante (ver seção de veredito).

### 3d — Arquivo binário e multi-linha

| Variante | Resultado |
|---|---|
| NUL bytes após a linha válida | `lineRe` casa o prefixo; linha aparece em `tf log`. |
| Conteúdo puro NUL/binário sem linha válida | `parseLine` retorna false; arquivo silenciosamente ignorado. |
| Multi-linha (duas linhas válidas) | **Ambas as linhas são lidas e mostradas** — uma entry pode produzir N entradas no log. |

O comportamento multi-linha é consistente com o `strings.Split` em `ReadAll:192`. Uma entry com N linhas
válidas contribui N transições. É o mesmo comportamento do arquivo legado (multi-linha). Não é uma
vulnerabilidade de injeção nova, mas o formato nominal é "uma linha por arquivo" — qualquer arquivo
plantado com múltiplas linhas inflaciona o log.

### 3e — Nomes especiais (`..entry`, `.entry`)

`..entry` e `.entry` passam em `strings.HasSuffix(name, ".entry")`, são regulares, e são
processados. `entryNameRe` não os casa; `sortKey` cai para `t.Timestamp` (fallback). Conteúdo válido
**aparece no log**. Não há path traversal (os nomes são componentes de um único diretório já aberto
via `os.ReadDir`). Comportamento benign, mas `..entry` com conteúdo forjado inflaciona o log —
mesma classe de item 3d.

### 3f — Diretório nomeado `x.entry`

`de.Type().IsRegular()` = false para diretórios → silenciosamente ignorado. Nenhum crash. PASS.

### FIFO `.entry` dentro do diretório

`de.Type()` = `os.ModeNamedPipe` → `!de.Type().IsRegular()` → silently skipped. Sem hang. PASS.

### FIFO no caminho do log legado

`readRegularFileBytes` → `openRegularFileNonblock` → `errNotRegularFile` → tratado como "sem arquivo legado". Sem hang. PASS. (Correção pré-existente da família ML-1B de 2026-09-06.)

**RESULTADO DO ITEM 3: PASS** — nenhum vetor lê fora da raiz, nenhum causa hang. Ressalvas 3c e 3d registradas abaixo.

---

## Item 4 — gitignore

### Medição

Padrões testados com `git check-ignore -q` contra `plan/rm/.trackfw-log-entries/20261022T100000Z-aabbccdd.entry`:

| Padrão | entries silenciadas? | wctx silenciado? | Presente em templates padrão? |
|---|---|---|---|
| `*.log` | Não | Não | Node.js, Python — **resolvido pelo rename** |
| `*.d` | Não | Não | C++ (GitHub) — **resolvido pelo rename** |
| `*.entry` | **Sim** | Não | Não encontrado em nenhum template padrão |
| `*-entries` | **Sim** (via path component) | Não | Não encontrado |
| `.*` | **Sim** (via dir component) | Não | Não encontrado em templates padrão |
| `*.tmp`, `*.bak`, `*.swp` | Não | Não | Vários |

`docs/agents-working-context/` não é afetado por nenhum desses padrões.

### Gap no teste

`TestEntriesDirNotIgnoredByCommonGitignorePatterns` (`translog_test.go:246`) cobre apenas
`*.d`, `*.log`, `*.tmp`, `*.bak`, `*.swp`. Não cobre `*.entry` nem `.*`.

A afirmação do Wave 0 ("nenhum template conhecido ignora `*.entry`") era sobre templates de
linguagem padrão do GitHub — correta para Go, Node.js, Python, Java, C++. Porém o teste não
formaliza essa afirmação, e `.*` (padrão adotado por alguns projetos para ignorar todos os
dotfiles) silencia `.trackfw-log-entries` inteiro.

### Risco prático

- `*.entry`: não está em nenhum template de linguagem padrão, mas poderia aparecer em projetos
  que usam arquivos `.entry` para outros fins. Projetistas de trackfw que adicionarem esse
  padrão ao seu `.gitignore` perdem entradas silenciosamente.
- `.*`: autodestrutivo como padrão (também ignoraria `.gitattributes`, `.gitignore`, etc.),
  mas é tecnicamente possível. Risco menor que o `*.log`/`*.d` original.

**Regressão em arquivo legado rastreado:** testado com `.gitignore` contendo `.*` +
`!.gitignore` + `!.trackfw-log`. O `.trackfw-log` permanece rastreado (negação explícita
funciona). O `.trackfw-log-entries/` é ignorado pelo padrão `.*` — o mesmo achado de R1, não
uma nova regressão. Nenhuma entrada rastreada existente é perdida; entradas novas não são
comitadas, o que é o risco central já declarado.

**RESULTADO: RESSALVA MÉDIA** — `TestEntriesDirNotIgnoredByCommonGitignorePatterns` precisa ser
atualizado para incluir `*.entry` e `.*`. Sem mudança de implementação obrigatória; o test gap é
o ponto de ação.

---

## Item 5 — `trackfw context` com ataques ao diretório de contexto

Todos os vetores testados em `docs/agents-working-context/`.

| Vetor | Resultado |
|---|---|
| Symlink no lugar do diretório | `os.Lstat` → `!fi.IsDir()` → retorna nil silenciosamente. Conteúdo externo NÃO lido. |
| Arquivo não-`.md` | Sem `.md` suffix → silently skipped. |
| `.md` que é symlink | `de.Type()&os.ModeSymlink != 0` → silently skipped. Conteúdo externo NÃO lido. |
| Arquivo gigante (100MB sparse) | `bufio.Scanner` + `sc.Text()` para a primeira linha `## ` → sem hang, sem OOM em sparse. |
| Nome sem timestamp | Incluído, mas sortado lexicograficamente. Aparece antes de arquivos com timestamp se inicial > `2`. Sem falha. |

Nenhum vetor produz leitura fora do diretório configurado. PASS para todos os vetores.

**`extractFirstHeading`** usa `os.Open` (não `readRegularFileBytes`): no caso de um FIFO em
`docs/agents-working-context/foo.md`, o `os.Open` em modo leitura tentaria abrir o FIFO e
bloquearia. O guarda `de.Type().IsRegular()` em `recentWorkingContextEntries:42` previne isso:
FIFOs retornam `os.ModeNamedPipe` que não é regular → skipped antes de chegar ao `extractFirstHeading`.
PASS.

---

## Item 6 — Coexistência com instruções antigas

**Cenário:** agente com instrução antiga faz append em `docs/agents-working-context.md`.

- O append sucede (arquivo existe, é regular, permissão de escrita).
- `tf validate` não tem nenhuma regra sobre `agents-working-context.md` — nenhuma violação.
- `tf context` lê `docs/agents-working-context/` (diretório separado) — não afetado pelo `.md`.
- O projeto funciona com os dois mecanismos em paralelo até o `agents update`.

**Janela de transição:**

Arquivos instalados via `trackfw agents install` (v8+) têm entrada no manifest (`owned = true`).
`planArtifactWrite:589` (`actualHash != desiredHash → writeDesired = true`) → `agents update`
substitui o arquivo pela nova instrução. Janela é fechada por `agents update` pós-release.

Arquivos sem entrada no manifest (instalação pré-manifest ou manual): reconhecidos pela
`legacyHashes` para os 10 agentes pré-v8 (SHA-256 dos templates removidos em 664573f).
Para arquivos instalados no período v8–v10.1.0 sem manifest (hipotético — na prática a v8
introduziu o manifest junto com o catálogo), não haveria reconhecimento e o arquivo seria
tratado como "user-modified". Sem reprodução prática identificada; declarado como ressalva de
transição na Wave 0 D1.

**RESULTADO: PASS.** Coexistência correta; janela de transição é declarada e tecnicamente fechável via `agents update`.

---

## Arquivos de implementação a verificar pelo hefesto-tf

Os itens abaixo devem ir para o parecer de qualidade (`docs/seguranca/2026-10-22-quality-log-por-entrada.md`):

- `internal/translog/translog_test.go` — `TestEntriesDirNotIgnoredByCommonGitignorePatterns`: adicionar `*.entry` e `.*` ao conjunto de padrões (Ressalva 1).
- Nenhum outro arquivo de implementação requer mudança para endereçar os achados deste red team.

---

## Defeitos que persistem com pré-existência idêntica ao formato legado

| Defeito | Pré-existência | Dispositivo |
|---|---|---|
| Sem limite de tamanho em arquivos de entry | Sem limite no `.trackfw-log` legado | Requer acesso local ao FS |
| `..entry` com conteúdo válido é lido | Qualquer linha no legado pode ser injetada por quem tem escrita no diretório | Idem |
| Duplicata de evento na transição de versão | N/A (não havia duas fontes antes) | Residual declarado D1/D2 |

---

## O que teria ido para os artefatos de persistência (suprimido pelo escopo do handoff)

**`docs/agents-working-context/`** (suprimido — proibido por este handoff):
```
## 2026-10-22 — hades-tf — INÍCIO: ML-3B red team log-por-entrada
- Binário: e8014482 (fix/trackfw-log-conflita-em-toda-branch-paralela)
- Setup: repo1 isolado, sem .gitattributes, roadmap_dir: plan/rm

## 2026-10-22 — hades-tf — FIM: ML-3B red team log-por-entrada
- Veredito: APROVADO COM RESSALVA (R1 média, R2+R4 baixas, R3+R5 info)
- Bloqueante: nenhum
- Ressalva obrigatória: TestEntriesDirNotIgnoredByCommonGitignorePatterns não cobre *.entry e .*
```

**`vault/notes/`** (suprimido):
Se `*.entry` e `.*` silenciam entries, isso merece nota indexada. Chave sugerida:
`vault/notes/translog-gitignore-entry-pattern-2026-10-22.md`.

**`.claude/agent-memory/hades-tf/`** (suprimido):
Atualizar `project_gitignore_log_extension_silences_trackfw_log_d.md` para documentar
que a extensão `.entry` foi escolhida para evitar `*.log` e `*.d`, mas que `*.entry`
e `.*` ainda silenciam as entradas — e que o test gap não cobre esses padrões.

---

## Veredito

**APROVADO COM RESSALVA**

Nenhum critério bloqueante atingido:
- Merge limpo sem `.gitattributes`: confirmado (Item 1).
- Conjunto A∪B após merge: confirmado (Item 1).
- Nenhuma leitura ou escrita fora da raiz: confirmado (Item 3).
- Nenhum hang em nenhum leitor: confirmado (Item 3, FIFOs).
- Janela de transição fechável: confirmado (Item 6).
- Bootstrap UTC bug não afeta validate nem metrics: confirmado (Item 2).
- chmod 000 no diretório de entradas: stale_wip warning emitido (não silenciado): confirmado (Item 2b-2).
- Regressão em .trackfw-log rastreado sob `.*` gitignore: não ocorre (Item 4).

**Ressalvas:**

**R1 (MÉDIA — exige ação):** `TestEntriesDirNotIgnoredByCommonGitignorePatterns`
não cobre `*.entry` nem `.*`. Ambos silenciam `.trackfw-log-entries/*.entry` em
repositórios que usam esses padrões. A afirmação do Wave 0 ("nenhum template padrão
ignora `*.entry`") é correta para templates de linguagem do GitHub, mas o teste não
a formaliza. **Exigência:** adicionar `*.entry` e `.*` ao conjunto de padrões no braço
positivo do teste antes de liberar a Wave 4.

**R2 (BAIXA):** `io.ReadAll` sem limite de tamanho em arquivos `.entry`. Um arquivo
`.entry` não-sparse de 1 GB consumiria 1 GB de heap. Requer acesso de escrita local
ao diretório, mesma superfície do formato legado. Nenhuma mudança obrigatória, mas
um limite (ex: 1 MB) tornaria o comportamento mais previsível.

**R3 (INFO):** `..entry` com conteúdo válido é lido pelo `ReadAll`. Nomes não
correspondentes a `entryNameRe` usam `t.Timestamp` como sortKey (fallback). Sem
impacto de segurança; documenta um caso de borda do formato.

**R4 (BAIXA):** `discover.go:204` usa `time.UTC` para parsear string no horário local
→ nome do arquivo `.entry` gerado pelo `--bootstrap-log` tem sort key errado (deslocado
pelo offset de fuso). Afeta apenas a ordem de exibição de `tf log` para entradas
retroativas. `t.Timestamp` (usado por validate e metrics) não é afetado — vem de
`parseLine` com `time.Local` independente do nome do arquivo. Fix: usar
`time.ParseInLocation(..., time.Local)` em `discover.go:204`.

**R5 (INFO):** `serve/metrics_log.go:ParseLog` retorna slice vazio silenciosamente
quando `translog.ReadAll` falha (symlink no diretório, chmod 000). O board mostra
`burndown:[]` e `cycle_time:0` sem nenhum erro visível. Contraste: `tf log` falha com
rc=1 e mensagem clara. O operador que monitora apenas o board não percebe a degradação.
`state_distribution` permanece correto (não usa translog). Sem impacto de segurança;
recomenda-se um campo `log_error` na resposta JSON em versão futura.
