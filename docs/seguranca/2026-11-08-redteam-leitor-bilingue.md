# Red Team — Leitor bilíngue de marcadores de roadmap

**Alvo:** branch `feat/idioma-do-produto-ingles-primeiro`
**Data:** 2026-11-08
**Revisor:** Hades (hades-tf)
**ADR de referência:** `docs/adr/ADR-2026-11-08-contrato-de-marcadores-do-roadmap-bilingue-parser-aceita-ingles-e-portugues-antes-de-o-gerador-emitir-ingles.md`
**Wave 0:** `docs/seguranca/2026-11-08-wave0-idioma-ingles-primeiro.md` §5

---

## Veredito

**REPROVA**

Dois motivos de bloqueio:

1. **A2 — `ParseGatesLines` j-loop aceita ` ```bash ` dentro de cercas de exemplo e comentários HTML, executando comandos do exemplo em vez do gate real.** Mecanismo pré-existente em `origin/main` para o cabeçalho PT; a branch adicionou o alias EN sem aplicar os guards de máscara que o pré-scan de D3 (mesma função) já usa. Wave 0 §5 não listou esta superfície.

2. **F — `Lapsed: ` (NBSP) e `Lapsed:​` (ZWSP) são contados como critério lapsed com razão vazia, fazendo `acceptance_evidence` passar indevidamente.** Mecanismo pré-existente em `origin/main` para `Caducou:`; a branch adicionou o alias `Lapsed:` sem corrigir o controle `\S`. Wave 0 §5 enumerou "over-recognition → critério Unmet passa como lapsed" como ameaça e disse que `\S` a controla — o controle falha para Unicode separators (`\p{Z}`) e format chars (`\p{Cf}`).

Ambos expandidos sem ser declarados como residual. Os demais achados são baixo risco, pré-existentes em mecanismo e não-expandidos, ou gap documentado.

---

## Método

Binários compilados:
- `$SCRATCH/trackfw-branch` — `go build ./cmd/trackfw` sobre a árvore de trabalho
- `$SCRATCH/trackfw-main` — `git archive origin/main | tar -x` seguido de `go build`

Ambos reportam versão `11.5.1`; a versão reflete a mesma tag base.

**Interações com o repositório real (disclosure):**

O processo de background `bsxue1qp9` foi lançado durante esta sessão (`cd "$REPO" && trackfw-branch barrier ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador --wave 1 --trust-local-gates --json`). O gate daquele roadmap é `make quality` + `bin/trackfw validate --json`. Ambos executaram com exit 0 — `make quality` reconstruiu `bin/trackfw`. Verificações pós-execução:

- `git check-ignore -v bin/trackfw` → `.gitignore:1:/bin/ bin/trackfw` — `bin/` é gitignored; `bin/trackfw` não está rastreado.
- `bin/trackfw --version` → `trackfw 11.5.1` — versão consistente; nenhum artefato rastreado foi alterado.
- `pgrep -fl 'make quality|go test'` → retornou apenas binários do Go build cache do sistema, não processos orphans do build.
- O processo `bsxue1qp9` foi terminado com `kill 33112` antes da escrita deste parecer; `ml_scope` bloqueou aquele barrier por `zz_hades_status_test_11081.go` estar out-of-scope.

Adicionalmente, o arquivo `zz_hades_status_test_11081.go` foi criado acidentalmente na raiz do repositório durante a sessão; foi descoberto via `git status` e removido antes do `git add`. Nunca foi staged.

Todos os testes de fixture usam diretórios em `$SCRATCH/` sem `git init` (confirmado: `ml_scope: skipped — not a git repository`). Os comandos `barrier` rodam com `--trust-local-gates`.

Contrato derivado antes de qualquer teste:
- `GatesHeaderRe`: `^\*\*(?:Gates da wave|Wave gates):\*\*` — conjunto fechado, case-sensitive
- `LapsedContinuationRe`: `^ {2,}(?:Caducou|Lapsed):\s*\S` — Go RE2 `\s` é ASCII-only
- `terminatedVocabulary`: agora inclui `"abandoned"` (novo EN para `"abandonado"`)
- `isCanceledToken`: correspondência exata — `"cancelado" || "canceled" || "cancelled"`
- j-loop (`ParseGatesLines` linha 1237): busca ` ```bash ` **sem** consultar `fenced[j]` nem `htmlMask[j]` (confirmado idêntico em `origin/main` linha 1201)

---

## Achados

### A2 — j-loop aceita ` ```bash ` dentro de cerca de exemplo (BLOQUEANTE · ALTO · pré-existente em PT; nova superfície EN não declarada)

**Localização:** `internal/roadmapdoc/roadmapdoc.go` linhas 1237–1248 (j-loop)

**Mecanismo:** após identificar o cabeçalho de gate válido, o j-loop varre as linhas seguintes procurando `t == "```bash"`. A verificação não consulta `fenced[j]` nem `htmlMask[j]`. Uma cerca de quatro crases contendo ` ```bash ` como interior faz o j-loop parar nessa linha interior em vez de prosseguir até o gate real. O mecanismo é idêntico em `origin/main`:

```
$ git show origin/main:internal/roadmapdoc/roadmapdoc.go | grep -n '"```bash"'
1201:  if t == "```bash" {
```

A branch modificou `ParseGatesLines` para adicionar o pré-scan de D3 — e nesse pré-scan aplicou corretamente `!fenced[i] && !htmlMask[i]`. O j-loop, na mesma função, ficou sem o guard equivalente.

**Medição — PT (confirma pré-existência em main via `--json`):**

Fixture `ROADMAP-a2-pt-example.md`, Wave 0, `**Gates da wave:**` (PT):
```
main:  barrier --wave 0 --json → gates.commands=['echo PICKED_EXAMPLE']  status=passed
```

**Medição — EN (nova superfície na branch via `--json`):**

Fixture `ROADMAP-a2-example-fence.md`, Wave 1, `**Wave gates:**` (EN):
```
branch: barrier --wave 1 --json → gates.commands=['echo PICKED_EXAMPLE']  status=passed
```

O gate real (`echo PICKED_REAL_GATE\nexit 1`) está depois da cerca de exemplo. `echo PICKED_EXAMPLE` substituiu o real gate; `exit 1` não executou. Contraprova — reference fixture `ROADMAP-arm-en.md` (mesmo `**Wave gates:**` + gate direto com `exit 1`):

```
branch: barrier --wave 1 --json → gates.commands=['exit 1']  status=blocked
```

**Por que é BLOQUEANTE:** o Wave 0 §5 não listou "cerca de exemplo antes do gate" como superfície (gap de enumeração). A branch expandiu a superfície sem a declarar como residual. A correção estava disponível na mesma função e na mesma sessão de edição — o pré-scan de D3 (linhas 1205–1209) já tinha o modelo correto.

**Correção proposta:** no j-loop (`roadmapdoc.go` linha 1239), guardar com `!fenced[j] && !htmlMask[j]`:

```go
// antes:
if t == "```bash" {
    break
}
// depois:
if t == "```bash" && !fenced[j] && !htmlMask[j] {
    break
}
```

`fenced` e `htmlMask` já são calculados nas linhas 1198–1199. Esta correção também fecha A1-EN abaixo.

---

### F — `Lapsed: ` e `Lapsed:​` passam `acceptance_evidence` como lapsed com razão vazia (BLOQUEANTE · MÉDIO · pré-existente em PT; nova superfície EN; controle `\S` falha para Unicode separators e format chars)

**Localização:** `internal/roadmapdoc/roadmapdoc.go` — `LapsedContinuationRe` (linha 64) e `extractLapsedReason` (linha 1029)

**Mecanismo:**

`LapsedContinuationRe` = `^ {2,}(?:Caducou|Lapsed):\s*\S`

Go RE2 `\s` é ASCII-only. NBSP (`U+00A0`, categoria `Zs`) satisfaz `\S` — não é ASCII whitespace. ZWSP (`U+200B`, categoria `Cf`) também satisfaz `\S`. A regex combina para `  Lapsed: ` e `  Lapsed:​`. `extractLapsedReason` usa `strings.TrimSpace` que, em Go, usa `unicode.IsSpace` — NBSP está em `unicode.IsSpace`, então é removido e a razão fica vazia. ZWSP NÃO está em `unicode.IsSpace`, então permanece (1 rune Unicode). O critério `- [ ]` é contado como Lapsed mesmo sem razão válida, e `acceptance_evidence` passa.

Harmonia com Wave 0 §5: a linha G3-2 da tabela §5.1 diz "over-recognition → critério Unmet passa como lapsed → barrier passa indevidamente. Ex.: `  Lapsed:` sem texto depois dos dois-pontos se regex omitir o `\S`." O controle nomeado (`\S`) não funciona para Unicode separators.

**Medição — NBSP (pre-existing em PT; nova superfície EN):**

Fixture `ROADMAP-f-nbsp.md`, Wave 0, ML-0A com `✅`, um `[x]` e um `- [ ] unmet criterion` seguido de `  Lapsed: `:

```
branch:  barrier --wave 0 → ✓ acceptance_evidence: passed
                              ~ ML-0A: 1 lapsed acceptance criteria
                              line 22: Caducou: 
main:    barrier --wave 0 → ✓ acceptance_evidence: passed
         (mesmo com Caducou:  — pré-existente)
```

**Medição — ZWSP:**

Fixture `ROADMAP-f-zwsp.md`, mesma estrutura com `  Lapsed:​`:

```
branch:  barrier --wave 0 → ✓ acceptance_evidence: passed
                              ~ ML-0A: 1 lapsed acceptance criteria
                              line 22: Caducou: ​
```

**Contra-braço (controle):**

(a) `- [ ] unmet criterion` seguido de `  Lapsed:` sem caractere trailing → gate que a Wave 0 nomeou como caso negativo:
```
branch:  barrier --wave 0 → ✗ acceptance_evidence: blocked  (ML-0A: 1 unmet acceptance criteria)
```

(b) `- [ ] unmet criterion` sem nenhuma linha de continuação:
```
branch:  barrier --wave 0 → ✗ acceptance_evidence: blocked  (ML-0A: 1 unmet acceptance criteria)
```

Ambos bloqueiam corretamente. O NBSP/ZWSP é o caminho acidental: a marker incompleta que o contrato mantém como Unmet vira Lapsed. É exatamente a direção G3-2 do adversário apressado descrita no Wave 0.

**Verificação do fix proposto `[^\s\p{Z}]`:**

```
$ go run test_lapsed2.go
Lapsed-NBSP  | orig=true  | fixed=false  (NBSP rejeitado — correto)
Lapsed-ZWSP  | orig=true  | fixed=true   (ZWSP aceito — fix INSUFICIENTE)
Lapsed-text  | orig=true  | fixed=true   (texto real aceito — correto)
```

O fix do regex não cobre ZWSP porque `U+200B` é `\p{Cf}` (format char), não `\p{Z}` (separator). O fix correto deve validar no extrator: exigir que `extractLapsedReason` retorne razão com pelo menos um `\p{L}`, `\p{N}` ou `\p{P}` — cobrindo NBSP, ZWSP e qualquer outro separador/formatador Unicode que `\S` aceita mas que não constitui razão legível.

---

### A1-EN — Comentário HTML mascara gate EN (MÉDIO · pré-existente em PT por leitura de código; nova superfície; mesma causa que A2)

**Localização:** mesmo j-loop, `htmlMask[j]` não consultado

**Mecanismo:** idêntico a A2: o j-loop para no primeiro ` ```bash ` que encontrar, que pode estar dentro de `<!-- ... -->`. Pré-existência verificada por leitura de código: o j-loop em `origin/main` é idêntico ao da branch.

**Medição (branch, EN, via `--json`):**

Fixture `ROADMAP-a1-comment-gate.md`, Wave 1, `**Wave gates:**`:
```
branch: barrier --wave 1 --json → gates.commands=['echo PICKED_OLD_GATE']  status=passed
```

O gate real (`echo PICKED_REAL_GATE\nexit 1`) está depois do comentário HTML. O comando do comentário executou. A correção de A2 (`!htmlMask[j]`) também fecha A1-EN.

---

### D1-EN — Comentário HTML não fechado mascara todo o gate EN (MÉDIO · pré-existente; nova superfície)

**Localização:** `htmlCommentMask` — comportamento documentado; não alterado nesta branch

**Mecanismo:** um `<!--` não fechado na prosa da wave faz `htmlCommentMask` marcar todas as linhas seguintes como masked — incluindo o cabeçalho do gate, que torna-se invisível para o pré-scan (D3) e para o j-loop. A correção de A2 não resolve D1 porque D1 mascara o próprio cabeçalho (antes do j-loop).

**Medição (branch, EN, via `--json`):**

Fixture `ROADMAP-d1-unclosed-comment.md`, Wave 1:
```
branch: barrier --wave 1 --json → gates.commands=[]  status=passed
```

Nenhum comando executou; o gate foi silenciosamente ignorado. Deve receber ML adicional nesta REQ, pois a causa (j-loop sem guard de máscara) é a mesma que A2/A1-EN. Ver §Residual.

---

### E3-EN — Cerca de três crases mascara cabeçalho por interação CommonMark (BAIXO · pré-existente; nova superfície EN)

**Localização:** `FenceMask`/`scanFences` — fechamento antecipado de cerca exterior (causa distinta de A2/D1)

**Mecanismo:** quando uma cerca de três crases contém um bloco ` ```bash ` interior sem info-string, `scanFences` fecha a cerca no primeiro ` ``` ` interior. A cerca exterior de fechamento abre uma nova cerca que mascara o cabeçalho do gate real. Causa distinta das acima: está no pré-scan via `fenced[]`, não no j-loop.

**Medição (branch, EN, via `--json`):**

Fixture `ROADMAP-e3-d3-fence-real.md`, Wave 1 (`**Wave gates:**`, gate `exit 1`):
```
branch: barrier --wave 1 --json → gates.commands=[]  status=passed
```

Nenhum comando executou. Pré-existência confirmada: `validate` com main binary para fixture PT equivalente emite "Wave 0 has no gate block; add '**Gates da wave:**' ..." — confirma falha aberta no main para mesma estrutura com PT header. Deve receber REQ própria (causa distinta: `scanFences`, não j-loop).

---

### G — `abandoñed` Terminated em `StatusCategory`, bloqueado em `StatusIsComplete` (BAIXO · nova superfície do alias `abandoned`)

**Localização:** `roadmapdoc.go` linhas 230–255 (`StatusCategory`) vs. linhas 167–177 (`StatusIsComplete`)

**Mecanismo:** `StatusIsComplete` guarda com `hasDisallowedCombiningMark(first)` → rejeita token com marca combinatória antes de normalizar. `StatusCategory` não faz essa guarda (comentário linha 239–241: "falls through to Pending" — factualmente incorreto: `normalizeStatusToken` a seguir strip a marca e encontra `"abandoned"` em `terminatedVocabulary`). Resultado: `move-to-done` aceita o ML como terminado enquanto `barrier mls_complete` o bloqueia.

**Medição:**

Fixture `ROADMAP-g-bare2.md`, status de ML-0A: `abandoñed` (bare, sem emoji):

```
branch:  barrier --wave 0  → ✗ mls_complete: blocked  (status: abandoñed)
         roadmap move done  → ✓ moved  (RC=0)
```

**Contra-braço — controle em branch e main:**

Fixture `ROADMAP-g-ctrl.md`, status de ML-0A: `⬜ Pendente`:
```
branch:  roadmap move done → Error: cannot move: 1 blocker(s): ML-0A  (RC=1)
main:    roadmap move done → Error: cannot move: 1 blocker(s): ML-0A  (RC=1)
```

O gate `move-to-done` FUNCIONA para Pending em ambos; a falha é específica ao combining mark em `abandoned`.

**Pré-existência em main:** fixture com `abandoñado` (PT + combining mark U+0303):
```
main:    roadmap move done → ✓ moved  (RC=0)
```

O mecanismo é pré-existente para PT. A branch abre nova rota via `"abandoned"` em `terminatedVocabulary`. O comentário em linha 239–241 está factualmente errado sobre o comportamento.

Severidade BAIXO — requer edição manual com marca combinatória Unicode e execução de `move-to-done` sem executar `barrier` primeiro.

---

### Oracle — script não executa `barrier` (MÉDIO · gap entre D7 original e emenda da Wave 1)

**Localização:** `scripts/check-marker-corpus-parity.sh`

**Mecanismo:** ADR D7 define o oracle como "compara a saída de `trackfw barrier` e de `trackfw validate`". A emenda da Wave 1 (linhas 93–95 do ADR) documenta que a verificação do acervo usou apenas `validate` e `roadmap show --json` porque os 14 roadmaps `done/` com dois cabeçalhos passariam a reprovar com D3. O script implementa a versão da emenda:

```
$ grep -c "barrier" scripts/check-marker-corpus-parity.sh
0
```

O schema de `roadmap show --json` não tem campo `gates`:

```
$ trackfw-branch roadmap show ROADMAP-arm-en --json | python3 -c "..."
['roadmap', 'path', 'state', 'malformed_waves', 'waves']
# cada wave: ['label', 'line', 'mls']  — sem campo gates
```

Resultado: mudanças em `ParseGatesLines` — incluindo A2 — não são detectadas pelo oracle de corpus. D7 original e emenda ficaram em tensão sem resolução explícita.

**Correção proposta:** expor comandos de gate por wave no schema de `roadmap show --json` (`waves[N].gate_cmds: []string`). Isso permite o oracle existente de 243 roadmaps comparar gate parsing sem executar shell — exatamente o motivo pelo qual a emenda trocou barrier por show.

---

## Achados corretos (confirmados sem anomalia)

| Alvo | Resultado branch | Observação |
|---|---|---|
| B — cerca de 4+ crases como fechador do gate | RC=2 "unterminated code fence" | Fail-closed correto; k-loop fecha só em ```` ``` ```` exato; validate emite "gate block is malformed" |
| C — cabeçalhos próximos (`**Wave Gates:**`, `**wave gates:**`, etc.) | `gates: passed` silencioso (commands=[]) | Conjunto fechado por design; nenhum near-miss entra |
| D3/E1 — PT + EN no mesmo wave | RC=2 "duplicate gates header: lines 29 and 35" | D3 dispara corretamente |
| E2 — comentário + real (D3 não deve disparar) | `gates: blocked` (exit 1 executado) | D3 não dispara; gate real executado |
| T6 absent (validate) | "Wave 0 has no gate block; add '**Wave gates:**' or '**Gates da wave:**' ..." | Ambas as formas EN e PT presentes |
| T6 malformed (validate) | "'**Wave gates:**' or '**Gates da wave:**' is not followed by a ```bash fence..." | Ambas as formas EN e PT presentes |
| T6 duplicate (validate) | "duplicate gates header: lines 11 and 17 both declare a gates block..." | Mensagem cita apenas linhas, não formas — correto (D3 mensagem) |

---

## Residual declarado

Este parecer aceita não cobrir como bloqueante:

1. **D1-EN (HTML não fechado) como fix completo nesta batch.** O mecanismo de mascaramento do `htmlCommentMask` não tem solução sem mudança de semântica mais ampla. Deve receber ML adicional nesta REQ (mesma causa de A2/A1-EN: j-loop sem guarda — mas D1 mascara o cabeçalho antes do j-loop, exigindo fix no pré-scan também).

2. **E3-EN (cerca 3 crases — interação CommonMark) como fix nesta batch.** Causa distinta (pré-scan via `scanFences`, não j-loop). Deve receber REQ própria com evidência de que a causa é distinta de A2.

3. **Oracle gap como bloqueante.** A emenda de D7 justificou a troca. O gap merece um novo campo no schema mas não bloqueia esta merge sozinho.

4. **Near-miss de cabeçalho (C) silenciosamente fail-open em wave 1+.** Por design (conjunto fechado). Fonte: Wave 0 §5.2 linha "Gates header (G3-1), wave 1+ — Under-recognition: sem controle compensador". Esta condição é permanente (não fecha com instalação de tag) e aceita como tal.

---

## Saída dos comandos git (repo `feat/idioma-do-produto-ingles-primeiro`)

```
$ git status --short
?? docs/seguranca/2026-11-08-redteam-leitor-bilingue.md

$ git diff --cached --name-only
(vazio)
```
