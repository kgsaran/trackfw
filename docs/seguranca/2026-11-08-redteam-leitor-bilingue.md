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

---

## Rodada 2 (2026-11-08)

**Binário:** `go build -o $SCRATCH/trackfw ./cmd/trackfw` sobre `feat/idioma-do-produto-ingles-primeiro` HEAD `7b2e30e9`. Versão reportada: `11.5.1`.
**Binário main:** `git archive origin/main | tar -x` seguido de `go build`, mesma versão reportada `11.5.1`.
**Fixtures:** `$SCRATCH/roadmaps/wip/` sem `git init` (ml_scope: skipped em todos).
**Scratchpad:** `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades/`

---

### Reverificação dos sete achados da Rodada 1

| Achado | Veredito R2 | Mecanismo do fix | Medição |
|---|---|---|---|
| A2 — j-loop aceita `\`\`\`bash\`` dentro de cerca de exemplo | FECHADO | `fenced[j] \|\| htmlMask[j]` guard adicionado ao j-loop (linha 1390) | barrier wave 1 fixture (4-tick outer + inner \`\`\`bash) → commands=["echo PICKED_REAL_GATE","exit 1"], status=blocked (exit 1 falhou) |
| A1-EN — comentário HTML mascara gate EN | FECHADO | mesmo guard, `htmlMask[j]` impede parada em \`\`\`bash\` dentro de `<!-- -->` | barrier wave 1 → commands=["echo PICKED_REAL_GATE","exit 1"], status=blocked |
| D1-EN — comentário HTML não fechado mascara cabeçalho | FECHADO | `HTMLCommentMaskCheck` detecta `<!--` não fechado antes do fim da wave → rc=2 | `trackfw barrier: unclosed HTML comment at line 13 may hide the gates header (D8, ADR-2026-11-08); rc=2` |
| E3-EN — cerca balanceada mascara cabeçalho (ML-2F) | FECHADO | quando nenhum cabeçalho válido existe mas há um mascarado → erro explícito | `trackfw barrier: gates header at line 20 is inside a code fence or HTML comment and no unmasked gates header exists in this wave; rc=2` |
| F — `Lapsed: ` NBSP / ZWSP passam como lapsed | PARCIAL → ver N-1 | `lapsedLineHasContent` foi adicionada: exige `\p{L}\|\p{N}\|\p{P}` | NBSP → `1 unmet acceptance criteria` (bloqueado ✓); ZWSP → `1 unmet acceptance criteria` (bloqueado ✓); mas Hangul Fillers (Lo) ainda bypassam — ver N-1 |
| G — `abandoñed` Terminated em StatusCategory, Pending em StatusIsComplete | FECHADO | `hasDisallowedCombiningMark` adicionado a StatusCategory (linha 247) | decomposed `abando`+`ñ`+`ed`: barrier mls_complete → blocked ✓; roadmap move done → `cannot move: 1 blocker(s): ML-0A` rc=1 ✓. Ambas as superfícies bloqueiam. Caso precomposto (U+00F1 → "abandonñed" não aplicável; `abandoñed` = abandoñed): barrier blocks (not complete, como esperado — abandoned ∉ statusVocabulary), move-to-done passa (terminated, design correto). Plain `abandoned`: mesmo padrão — intencional (terminated ≠ complete). |
| Oracle — script não executava `barrier` | FECHADO (parcial: inerte para este PR, fechado para runs futuros) | ML-2D adicionou `gates {commands, error}` ao schema de `roadmap show --json`; oracle compara quando base tem o campo | `TRACKFW_BASE_BIN=main bash scripts/check-marker-corpus-parity.sh` → `base lacked gates field` (old-binary mode); 243 roadmaps, rc=0. Com main como base, oracle está em modo strip — ParseGatesLines changes desta PR não são detectadas (documentado no cabeçalho do script). Inerte para este diff; fecha para futuros. |

**Detalhe de G:** o advisor apontou que o fixture original da Rodada 1 estava com `abanñoned` (typo). O fixture corrigido usou `abando`+`ñ`+`ed` = `abandoñed` (combinação real). Medição re-executada com fixture correto — G confirmado FECHADO.

---

### N-1 — Hangul Filler (Lo) bypassa `lapsedLineHasContent` (BLOQUEANTE · MÉDIO · mesma causa que F)

**Localização:** `internal/roadmapdoc/roadmapdoc.go` — `lapsedLineHasContent` (linha 1099)

**Mecanismo:** `lapsedLineHasContent` aceita um rune se `unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r)`. Hangul Fillers têm categoria Unicode `Lo` (Letter, Other) → `unicode.IsLetter` retorna `true`. Nenhum dos quatro tem glifo visível em contexto padrão. O NBSP/ZWSP corrigido pela ML-2C é `Zs`/`Cf` — fora de `\p{L}` → correto. Os Hangul Fillers estão dentro de `\p{L}` → passam.

**Classe medida:**

| Code Point | Nome | Categoria | `IsLetter` | `IsODIP` | Resultado `acceptance_evidence` |
|---|---|---|---|---|---|
| U+3164 | Hangul Filler | Lo | true | true | `passed` (1 lapsed) — BYPASSA |
| U+115F | Hangul Choseong Filler | Lo | true | true | `passed` (1 lapsed) — BYPASSA |
| U+1160 | Hangul Jungseong Filler | Lo | true | true | `passed` (1 lapsed) — BYPASSA |
| U+FFA0 | Halfwidth Hangul Filler | Lo | true | true | `passed` (1 lapsed) — BYPASSA |
| U+2800 | Braille Blank | So | false | false | `blocked` (1 unmet) — CORRETO |

Contra-braço: U+2800 (So, não letra) → `acceptance_evidence: blocked`. Texto real → `acceptance_evidence: passed`.

**Medição direta:**

```
Fixture: ML-0A ✅ Done, [x] met, [ ] unmet seguido de "  Lapsed:ㅤ"
branch: barrier --wave 0 --trust-local-gates --json
  → acceptance_evidence: status=passed, lapsed=["ML-0A: 1 lapsed acceptance criteria"]
  → lapsed_details: [{line:16, text:"ㅤ"}]
  → gates: passed (echo OK: exit 0)
  → overall: blocked apenas por validate (fixture sem governança — 86 violations)
```

Em repositório real com `validate` passando (como o próprio acervo desta branch), `acceptance_evidence` passaria e `barrier` passaria.

**Fix path verificado:** `unicode.Other_Default_Ignorable_Code_Point` (tabela nativa do Go) cobre todos os 4 Hangul Fillers. Fix correto em `lapsedLineHasContent`:
```go
// antes:
if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r) {
    return true
}
// depois:
if (unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsPunct(r)) &&
    !unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) {
    return true
}
```

**Por que é BLOQUEANTE:** mesma causa que F (ML-2C fechou Zs/Cf mas não Lo fillers). Regra Dura: mesma causa → mesma REQ → mesmo PR. ML-2C declarou no commit "F: razão de Lapsed:/Caducou: só com NBSP/ZWSP não conta como caducado" — o fix foi parcial. Adicionar alias EN sem fechar a classe completa expande a superfície sem declarar como residual.

---

### Probe (a) — Variantes de cabeçalho próximas ao fechado (PRÉ-EXISTENTES, não bloqueiam)

Medidos os vetores nomeados pelo revisor: 1-3 espaços ASCII antes de `**Wave gates:**`, lista (`- **Wave gates:**`), blockquote sem cabeçalho real, homóglifo cirílico (`gаtes`).

| Variante | Comportamento | Compensação wave 0 | Compensação wave 1+ |
|---|---|---|---|
| ` **Wave gates:**` (1 espaço) | gates: passed cmds=[] — gate ignorado silenciosamente | `validate roadmap_gate_coverage` dispara | NENHUMA |
| `  **Wave gates:**` (2 espaços) | idem | idem | NENHUMA |
| `   **Wave gates:**` (3 espaços) | idem | idem | NENHUMA |
| `- **Wave gates:**` (lista) | idem | idem | NENHUMA |
| blockquote-only | idem | idem | NENHUMA |
| `**Wave gаtes:**` (Cirílico) | idem | NENHUMA | NENHUMA |

1-3 espaços ASCII e homóglifo cirílico renderizam identicamente ao cabeçalho correto na maioria dos renderizadores Markdown — o controle de revisão visual que justifica o conjunto fechado (G3-1) não funciona para eles.

**Argumento de classificação:** estes são vetores da residual #4 da Rodada 1 ("near-miss de cabeçalho, fail-open em wave 1+, por design"). O mecanismo é `GatesHeaderRe` não corresponder — diferente do mecanismo D8 (cabeçalho mascarado dentro de cerca/comentário). A branch não introduziu este comportamento: `**Gates da wave:**` em `origin/main` tem a mesma falha para os mesmos vetores. A branch replica para `**Wave gates:**`.

**Por que não bloqueia:** pré-existente em PT, coberto pela residual declarada, mecanismo diferente do D8. Wave 0 tem controle compensador. Wave 1+ não tem — é o trade-off declarado como aceitável.

**Nota:** o vetor Cirílico não tem controle compensador em nenhuma wave. Para completar a postura D8 em wave 1+, o fix teria a forma "verificar se a linha satisfaz GatesHeaderRe após strip de whitespace/Cf/ODIP e emitir erro se a versão stripped matches mas a original não" — ML separado, REQ própria se a causa for considerada diferente do D8 mascarado.

---

### Probe (b) — Outros chars invisíveis em `Lapsed:` (resultado incorporado em N-1)

U+2060 (Cf), U+FEFF (Cf), U+180E (Cf): todos BLOQUEADOS corretamente (`1 unmet acceptance criteria`). Cf não é `\p{L}`, o fix da ML-2C cobre. Apenas a classe Lo (Hangul Fillers) escapa — documentado em N-1.

---

### Probe (c) — Concordância `barrier` / `roadmap show --json` por wave

Testados: (1) fixture normal (commands=[`echo wave0`]) — `show` retorna `gates.commands=["echo wave0"]`, barrier executa corretamente. (2) Fixture D1 (HTML não fechado) — `show` retorna `gates.error="unclosed HTML comment at line 13..."`, barrier retorna mesmo erro rc=2. (3) Fixture 2-wave: sed strip correto, `mls` não afetado. Ambas as superfícies concordam. PASS.

---

### Probe (d) — Oracle sed sem falso verde

`sed '/^      "gates": {/,/^      },/d'` sobre fixture de 2 waves com comandos reais:
```
wave 0: gates stripped, mls intacto
wave 1: gates stripped, mls intacto
```
Sem bleed entre waves. Sem remoção de `mls`. Sem falso verde. PASS.

Oracle com main como base (`TRACKFW_BASE_BIN=main`): `base lacked gates field` → modo strip. Comparação de 243 roadmaps, validate idêntico. rc=0. Confirma que o oracle é correto mas **não detecta** as mudanças em `ParseGatesLines` desta PR (ML-2C/2D) quando main é base — inerte para este diff. Para futuros PRs onde HEAD usa gates e base também, o campo é comparado diretamente (ST-P4 testado; rc=1 quando commands diferem). Declarado explicitamente no cabeçalho do script.

---

### Veredito da Rodada 2

**REPROVA**

Motivo único de bloqueio:

**N-1 — F parcial: Hangul Filler class (U+3164, U+115F, U+1160, U+FFA0) bypassa `lapsedLineHasContent` (BLOQUEANTE MÉDIO).** Todos os quatro são `IsLetter=true`, `IsODIP=true`. `acceptance_evidence` passa com `lapsed_details.text` = glifo invisível. Mesma causa que F (ML-2C). Regra Dura exige fechar no mesmo PR. Fix: `!unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r)` como guarda adicional.

Achados não bloqueantes:
- Probe (a) variantes de near-miss: pré-existentes em PT, cobertos pela residual #4, mecanismo diferente do D8.
- Probe (c) concordância barrier/show: PASS.
- Probe (d) oracle sed: sem falso verde; modo strip-base documenta a janela de indetectabilidade.

---

### Saída dos comandos git (Rodada 2)

```
$ git status --short
?? docs/seguranca/2026-11-08-redteam-leitor-bilingue.md

$ git diff --cached --stat
(vazio)
```

---

## Rodada 3 (2026-11-08)

**Binário branch:** `go build -o $SCRATCH/hades3/trackfw ./cmd/trackfw` sobre HEAD `f5e44ac9`. Versão: `11.5.1`.
**Scratchpad:** `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades3/`
**Acervo:** 1 wip, 264 done, 11 abandoned, 0 backlog/analyzing/blocked.

---

### Reverificação de N-1 e Probe (a) da Rodada 2

Após commits 4419fc0f (ML-2G) e f5e44ac9 (ML-2I):

| Vector | R2 | R3 | Mecanismo do fix |
|---|---|---|---|
| N-1 U+3164 Hangul Filler | BLOQUEANTE | **FECHADO** | `!unicode.Is(ODIP, r)` em `lapsedLineHasContent`; `acceptance_evidence: blocked` |
| N-1 U+115F Hangul Choseong | — | **FECHADO** | mesmo guard; `acceptance_evidence: blocked` |
| N-1 U+1160 Hangul Jungseong | — | **FECHADO** | idem |
| N-1 U+FFA0 Halfwidth Hangul | — | **FECHADO** | idem |
| Probe (a) 1–3 espaços antes | ABERTO | **FECHADO** | gatesNearMissSkeleton + step 2 (TrimLeftFunc IsSpace) |
| Probe (a) `- **Wave gates:**` | ABERTO | **FECHADO** | step 2 strip list marker |
| Probe (a) `> **Wave gates:**` | ABERTO | **FECHADO** | step 2 strip blockquote |
| Probe (a) cirílico `gаtes` | ABERTO | **FECHADO** | nearMissHomoglyphs `а`→`a` antes de ToLower |

Evidência de near-miss para Probe (a) ` **Wave gates:**` (1 espaço):

```
$ trackfw barrier ROADMAP-pa-1space --wave 0 --trust-local-gates
trackfw barrier: near-miss gates header at line 15: the line looks like a gates header but does not match the accepted form exactly (leading space, list/blockquote marker, invisible character, homoglyph or case) — write '**Wave gates:**' or '**Gates da wave:**' at column 0 (D8, ADR-2026-11-08)
rc=2
```

---

### R3-A — Homóglifos capitais: pipeline aplica homoglyphs antes de ToLower (ABERTO · ALTO · BLOQUEANTE)

**Mecanismo:** `gatesNearMissSkeleton` aplica `nearMissHomoglyphs` antes de `strings.ToLower`. A tabela cobre as formas minúsculas (ԝ→w, α→a, ε→e, τ→t) mas não os capitais correspondentes. O capital chega ao passo de homoglifos sem mapeamento, passa pela tabela inalterado, depois `ToLower` o converte para a forma minúscula — que a tabela de homoglyphs já aplicou. O rune minúsculo resultante (ԝ, α, ε) fica no skeleton sem mapeamento para o ASCII correspondente.

Verificação Go: `unicode.ToLower(0x051C)` → `0x051D` (ԝ). `unicode.ToLower(0x0391)` → `0x03B1` (α). `unicode.ToLower(0x0395)` → `0x03B5` (ε).

| Vector | Código | ToLower de | Skeleton | Resultado |
|---|---|---|---|---|
| `**Ԝave gates:**` | U+051C Cyrillic capital WE | U+051C→U+051D ԝ | `ԝavegates:` | ABERTO (cmds=[], rc=0) |
| `**Wave gΑtes:**` | U+0391 Greek capital Alpha | U+0391→U+03B1 α | `wavegαtes:` | ABERTO (cmds=[], rc=0) |
| `**Wave gatΕs:**` | U+0395 Greek capital Epsilon | U+0395→U+03B5 ε | `wavegateεs:` | ABERTO (cmds=[], rc=0) |

Contra-braço — Т (U+0422 Cyrillic capital TE, presente na tabela como `Т→T`) → `Ta` via ToLower → T mapeado na tabela → skeleton `wavegates:` → near-miss CAPTURADO (rc=2). Confirma que a tabela em si funciona quando o capital está incluído.

Contra-braço — `**Wave gates:**` canônica → `commands=["exit 1"]`, `status=blocked`, rc=1. Gate executado.

**Por que é BLOQUEANTE:** Ԝ, Α, Ε são scripts cirílico e grego, explicitamente fora do escopo de residual declarado (aprovação criterion: "Cyrillic and Greek cannot be declared residual").

---

### R3-B — Seletores de variação VS1–VS14: categoria Mn, não removidos em step 1a (ABERTO · ALTO · BLOQUEANTE)

**Mecanismo:** step 1a remove apenas `unicode.Cf`, `unicode.Other_Default_Ignorable_Code_Point` e VS16 (`U+FE0F`). Os seletores VS1–VS15 (`U+FE00–U+FE0E`) têm categoria `Mn` (Nonspacing Mark) com propriedade `Variation_Selector`. Não são `Cf` nem ODIP. NFKC não os remove. Não estão no strip set (`*_#|>-+~`) nem são `unicode.IsSpace`. Renderizam-se como invisíveis na maioria dos contextos.

| Vector | Código | Mecanismo | Resultado |
|---|---|---|---|
| `**Wave︀ gates:**` | VS1 U+FE00 após 'e' de Wave | byte 0xEF 0xB8 0x80 sobrevive ao skeleton; `wave︀gates:` ≠ `wavegates:` | ABERTO (cmds=[], rc=0) |
| `**Wave g︎ates:**` | VS14 U+FE0E dentro de gates | idem; `waveg︎ates:` ≠ `wavegates:` | ABERTO (cmds=[], rc=0) |

Contra-braço: NBSP (U+00A0, `unicode.IsSpace=true`) entre Wave e gates → near-miss CAPTURADO (rc=2). Confirma que a remoção de IsSpace em step 3 funciona, mas VS1–VS14 escapam por serem Mn.

**Por que é BLOQUEANTE:** o VS1–VS14 é invisível ao autor e ao revisor — contradiz a afirmação da emenda D8 "sem caracteres invisíveis". Um LLM que gere texto com seletor de variação por copy-paste ou troca de fonte produziria este bypass.

---

### R3-C — Braille Blank U+2800 como espaço (ABERTO · ALTO · BLOQUEANTE)

**Mecanismo:** U+2800 tem categoria `So` (Symbol, Other). `unicode.IsSpace` retorna `false` para `So` (apenas Zs/Zl/Zp e os whitespace ASCII especiais). Não está no strip set. Renderiza com largura de um espaço em contextos visuais.

```
**Wave⠀gates:**   (U+2800 entre Wave e gates)
→ skeleton: wave⠀gates:   (⠀ sobrevive, não-match)
→ ABERTO: commands=[], status=passed, rc=0
```

Contra-braço — NBSP U+00A0 (IsSpace=true) → near-miss capturado. Braille Blank escapa porque IsSpace não cobre categoria So.

**Por que é BLOQUEANTE:** Braille Blank renderiza identicamente ao espaço comum em todos os renderizadores Markdown testados. O revisor visual não distingue.

---

### R3-D — Formas sem dois-pontos ASCII: colonIdx = -1, retorno antecipado (ABERTO · ALTO · BLOQUEANTE)

**Mecanismo:** `gatesNearMissSkeleton` usa `strings.IndexByte(s, ':')` para localizar o dois-pontos. Quando o cabeçalho não contém `:` (ASCII 0x3A), `colonIdx < 0` e a função retorna `s` sem processar. O skeleton não começa com nenhum target → near-miss não dispara. `GatesHeaderRe` também não casa (exige `:` literal). Resultado: zero gates silenciosamente.

| Vector | GatesHeaderRe | Near-miss | Resultado |
|---|---|---|---|
| `**Wave gates**` | não casa | colonIdx=-1; não dispara | ABERTO (cmds=[], rc=0) |
| `### Wave gates` | não casa | colonIdx=-1; não dispara | ABERTO (cmds=[], rc=0) |
| `**Gates da wave**` | não casa | colonIdx=-1; não dispara | ABERTO (cmds=[], rc=0) |

Contra-braço — `**Wave gates:**` (com colon) → `commands=["exit 1"]`, status=blocked.

**Por que é BLOQUEANTE:** "**Wave gates**" (sem colon) é a forma mais provável de output de LLM ao gerar um roadmap incompleto. "### Wave gates" é natural em Markdown. São os vetores de maior probabilidade de ocorrência acidental.

---

### R3-E — Símbolos alternativos de dois-pontos (PARCIAL)

**U+FF1A ：(Fullwidth Colon) → FECHADO:** NFKC converte U+FF1A para `:` (ASCII). Após normalização, `colonIdx` encontra o colon e o skeleton se torna `wavegates:` → near-miss CAPTURADO (rc=2). Comportamento correto.

**U+2236 ∶ (Ratio Symbol) → ABERTO:** NFKC não converte U+2236 (categoria Sm). `strings.IndexByte` busca byte `0x3A`; U+2236 codifica como `\xE2\x88\xB6` — sem byte `0x3A`. `colonIdx=-1` → retorno antecipado → NOT caught.

```
**Wave gates∶**  (U+2236)
→ skeleton: **wave gates∶** (retorno antecipado)
→ ABERTO: commands=[], status=passed, rc=0
```

**U+A789 ꞉ (Modifier Letter Colon) → ABERTO:** idem. `colonIdx=-1` → NOT caught.

Dos 3 símbolos alternativos: 1 capturado, 2 ABERTO. ∶ e ꞉ renderizam identicamente ao `:` em muitas fontes.

---

### R3-F — Formas HTML e entity (PARCIAL)

| Vector | Mecanismo | Resultado |
|---|---|---|
| `<strong>Wave gates:</strong>` | `<` não está no strip set; skeleton = `<strong>wavegates:...` → não começa com `wavegates:` | ABERTO (cmds=[], rc=0) |
| `<b>Gates da wave:</b>` | idem; `<b>gatesdawave:` → não casa | ABERTO (cmds=[], rc=0) |
| `**Wave gates&#58;**` | entity `&#58;` não contém ASCII `:`; colonIdx=-1 → retorno antecipado | ABERTO (cmds=[], rc=0) |
| `**Wave gates:**` (NBSP) | NBSP é IsSpace → stripped em step 3 → skeleton `wavegates:` → CAPTURADO | FECHADO (near-miss rc=2) |

`<strong>Wave gates:</strong>` renderiza identicamente a `**Wave gates:**` em HTML. Um LLM pode gerar HTML raw em um roadmap. `**Wave gates&#58;**` renderiza como `**Wave gates:**` em todos os renderers que processam entidades HTML.

---

### R3-G — Latin ɑ (U+0251) em lugar de 'a' (ABERTO · MÉDIO · BLOQUEANTE pelo critério)

**Mecanismo:** U+0251 (Latin Small Letter Alpha, categoria Ll) não está em `nearMissHomoglyphs`. NFKC não o converte (já é forma canônica). `strings.ToLower` não altera (já é lowercase). Skeleton: `wavegɑtes:` ≠ `wavegates:` → NOT caught.

```
**Wave gɑtes:**  (ɑ = U+0251)
→ skeleton: wavegɑtes:
→ ABERTO: commands=[], rc=0
```

**Critério de residual:** o critério de aprovação do arquiteto exclui explicitamente scripts Latino/Cirílico/Grego do residual declarado. ɑ é script Latino. BLOQUEANTE pelo critério.

Contra-braço — ɡ (U+0261 Latin Small Letter Script G, presente na tabela como `ɡ→g`) → near-miss CAPTURADO. Confirma que a tabela captura o subset incluído.

---

### R3-H — Falsos positivos: near-miss dispara em prosa legítima

**Mecanismo:** o loop de near-miss escaneia todas as linhas não-mascaradas em [waveStart, waveEnd) que não casam com `GatesHeaderRe`. Uma linha de prosa "Wave gates: see below" não está em itálico/negrito e não casa `GatesHeaderRe`. Seu skeleton: `wave gates: see below` → strip (IsSpace, stripSet) → `wavegates:see below` → `strings.HasPrefix(_, "wavegates:")` = **true** → near-miss dispara rc=2.

O loop não é desabilitado pela presença de um cabeçalho válido (`validHeaderLines` é preenchido depois do loop de near-miss):

```go
// D8 near-miss check runs BEFORE D3 — before validHeaderLines is populated.
for i := waveStart; i < waveEnd; i++ {
    if fenced[i] || htmlMask[i] || GatesHeaderRe.MatchString(lines[i]) {
        continue
    }
    // prose line "Wave gates: see below" reaches here
```

**Medição:**

FP1 — wave com `**Wave gates:**` válido + prosa `Wave gates: see below` em corpo de ML:
```
$ trackfw barrier ROADMAP-fp1-prose-wavegates --wave 0 --trust-local-gates
trackfw barrier: near-miss gates header at line 16: the line looks like a gates header...
rc=2
```

FP2 — wave com `**Wave gates:**` válido + item de lista `- Gates da wave: rodados pelo Zeus`:
```
$ trackfw barrier ROADMAP-fp2-list-gatesdawave --wave 0 --trust-local-gates
trackfw barrier: near-miss gates header at line 28: the line looks like a gates header...
rc=2
```

**Varredura do acervo (`roadmap show --json` em 276 roadmaps):**

| Estado | Near-miss hits |
|---|---|
| wip (1) | **0** |
| done (264) | 3 (antigos `**Gate da wave:**` singulares em roadmaps pre-D8) |
| abandoned (11) | 0 |

0 hits em `wip/` → nenhuma regressão em CI na branch atual. Os 3 hits em `done/` são do form singular `**Gate da wave:**` que era usado antes da padronização; não são rodados pelo `validate`/`barrier` no CI.

**Prosa legítima com `**...**` em backtick é protegida:** `` `**Wave gates:**` `` tem o backtick preservado (passo 3 não remove backtick), então skeleton começa com `` ` `` e não casa nenhum target — correto.

**Severidade do FP:** MÉDIO. O padrão de risco é: um autor que escreve "Wave gates: see below" em texto simples (sem backtick) em qualquer lugar da wave teria o `barrier` reprovando mesmo com gate válido. A proteção via backtick é indireta e não documentada. 0 hits atuais em wip/ → nenhuma regressão imediata.

---

### Resumo tabular dos vetores R3

| ID | Vector | GatesHeaderRe | Near-miss | Veredito | Severidade |
|---|---|---|---|---|---|
| R3-A1 | `**Ԝave gates:**` U+051C | não casa | ABERTO (ԝ sobrevive) | ABERTO | ALTO |
| R3-A2 | `**Wave gΑtes:**` U+0391 | não casa | ABERTO (α sobrevive) | ABERTO | ALTO |
| R3-A3 | `**Wave gatΕs:**` U+0395 | não casa | ABERTO (ε sobrevive) | ABERTO | ALTO |
| R3-A* | `**Wave gaТes:**` U+0422 TE | não casa | CAPTURADO (Т→T na tabela) | FECHADO | — |
| R3-B1 | VS1 U+FE00 invisível | não casa | ABERTO (Mn, não-Cf/ODIP) | ABERTO | ALTO |
| R3-B2 | VS14 U+FE0E invisível | não casa | ABERTO | ABERTO | ALTO |
| R3-C | U+2800 Braille blank | não casa | ABERTO (So, não IsSpace) | ABERTO | ALTO |
| R3-D1 | `**Wave gates**` sem `:` | não casa | ABERTO (colonIdx=-1) | ABERTO | ALTO |
| R3-D2 | `### Wave gates` sem `:` | não casa | ABERTO | ABERTO | ALTO |
| R3-D3 | `**Gates da wave**` sem `:` | não casa | ABERTO | ABERTO | ALTO |
| R3-E1 | `**Wave gates：**` U+FF1A | não casa | CAPTURADO (NFKC→`:`) | FECHADO | — |
| R3-E2 | `**Wave gates∶**` U+2236 | não casa | ABERTO (colonIdx=-1) | ABERTO | MÉDIO |
| R3-E3 | `**Wave gates꞉**` U+A789 | não casa | ABERTO (colonIdx=-1) | ABERTO | MÉDIO |
| R3-F1 | `<strong>Wave gates:</strong>` | não casa | ABERTO (`<` sobrevive) | ABERTO | MÉDIO |
| R3-F2 | `<b>Gates da wave:</b>` | não casa | ABERTO | ABERTO | MÉDIO |
| R3-F3 | `**Wave gates&#58;**` | não casa | ABERTO (colonIdx=-1) | ABERTO | MÉDIO |
| R3-F* | `**Wave gates:**` NBSP | não casa | CAPTURADO (IsSpace stripped) | FECHADO | — |
| R3-G | `**Wave gɑtes:**` U+0251 | não casa | ABERTO (Ll, não na tabela) | ABERTO | MÉDIO |
| R3-H | FP: prosa "Wave gates: see below" | n/a | DISPARA rc=2 (FP) | FP | MÉDIO |

**ABERTO confirmado (D8 fail-open): 14 vetores.** FECHADO/CAPTURADO: 4 vetores. FP: 1 classe.

---

### Veredito da Rodada 3

**REPROVA**

Motivos de bloqueio:

1. **R3-A — Homóglifos capitais (Ԝ U+051C, Α U+0391, Ε U+0395):** pipeline aplica `nearMissHomoglyphs` antes de `ToLower`; capitais Cirílico/Grego não estão na tabela. Após `ToLower` a forma minúscula persiste no skeleton sem mapeamento para ASCII. Os três scripts são Cirílico e Grego — explicitamente fora do escopo de residual pelo critério de aprovação.

2. **R3-B — Variation selectors VS1–VS14 (U+FE00–U+FE0E):** categoria `Mn`, não removidos por step 1a (que cobre `Cf` e ODIP, não `Mn`). NFKC não os remove. Invisíveis em qualquer renderizador. Contrariam a afirmação de D8 "sem caracteres invisíveis".

3. **R3-C — Braille Blank U+2800:** categoria `So`, `unicode.IsSpace=false`. Renderiza como espaço. Não está no strip set. Near-miss bypassa.

4. **R3-D — Formas sem dois-pontos ASCII (`**Wave gates**`, `### Wave gates`, `**Gates da wave**`):** `strings.IndexByte` retorna -1; função retorna sem processar. `**Wave gates**` é a forma mais provável de LLM omitir o colon. Zero gates silenciosamente.

5. **R3-E2/R3-E3 — ∶ U+2236 e ꞉ U+A789:** colonIdx=-1 (nenhum byte `0x3A` no UTF-8 desses símbolos). Zero gates silenciosamente.

6. **R3-F — `<strong>`, `<b>`, `&#58;`:** `<` sobrevive ao skeleton ou colonIdx=-1. Zero gates.

7. **R3-G — Latin ɑ U+0251:** não na tabela de homoglyphs; script Latino, fora do escopo de residual.

8. **R3-H (falso positivo) — FP não bloqueia o merge por si só** (0 hits em wip/ no acervo real), mas o mecanismo é real: prosa não-backtick com "Wave gates:" ou lista com "Gates da wave:" em qualquer linha de uma wave dispara near-miss mesmo com cabeçalho válido. Deve ser corrigido junto com os ABERTO acima (mesma causa: lógica do loop de near-miss).

Todos os vetores ABERTO compartilham o mesmo sintoma D8: gate some em silêncio, `barrier` passa com `commands=[]`. Por Regra Dura: mesma causa → mesma REQ → mesmo ML (novo ML para o arquiteto na REQ vigente).

---

### Saída dos comandos git (Rodada 3)

```
$ git status --short
 M docs/seguranca/2026-11-08-redteam-leitor-bilingue.md

$ git diff --cached --stat
(vazio)
```
