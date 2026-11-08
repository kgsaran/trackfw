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

---

## Rodada 4 (2026-11-08)

**Binário branch:** `go build -o $SCRATCH/hades4/trackfw ./cmd/trackfw` sobre HEAD `51ada601` (ML-2J). Versão: `11.5.1`.
**Critério ADR itens 1–5:** near-miss só é erro se tiver cerca ` ```bash ` ligada (`boundGateFence`). APROVA quando nenhum vetor ACIDENTAL conta zero gates em silêncio (item 5). Residual (item 4): homóglifos fora da tabela fechada e ofuscação deliberada — esses NÃO reprovam.
**Scratchpad:** `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades4/`
**Instrumento:** `roadmap show --json` (wave 1 de cada fixture); barrier confirmado em N9 e ctrl-canonical.
**Disclosure:** durante esta rodada foi criado `/tmp/hades4-barrier-test/` fora do scratchpad. Continha `gates_script.sh` com `echo OK_BARRIER` + `exit 0` (teste rápido de fence durante diagnóstico de N8). Removido (`rm -rf`) nesta sessão antes do handback; nunca conteve roadmaps de produção.

---

### Reverificação R3-A a R3-H contra ML-2J

Critério novo para cada vetor: três braços — (a) com ` ```bash ` fence: deve emitir near-miss ERR; (b) sem fence: deve dar `cmds=[]` sem erro (zero gates legal); (c) cabeçalho canônico + fence: deve dar `cmds=['exit 1']`.

| Vetor | Arm (a) com fence | Arm (b) sem fence | Mecanismo do fix |
|---|---|---|---|
| R3-A `**Ԝave gates:**` U+051C | ERR near-miss | cmds=[] ok | ToLower antes de homoglyphs; U+051C→ԝ→w |
| R3-A `**Wave gΑtes:**` U+0391 | ERR near-miss | — | U+0391→α→a |
| R3-A `**Wave gatΕs:**` U+0395 | ERR near-miss | — | U+0395→ε→e |
| R3-B `**Wave︀ gates:**` VS1 U+FE00 | ERR near-miss | — | Mn removido em step 1a |
| R3-C `**Wave⠀gates:**` U+2800 | ERR near-miss | — | U+2800 removido explicitamente em step 1a |
| R3-D `**Wave gates**` (sem colon) | ERR near-miss | — | colonIdx=-1 → exact-equality path; skeleton=`wavegates` |
| R3-E `**Wave gates∶**` U+2236 | ERR near-miss | — | step 1e: `∶`→`:` |
| R3-F `<strong>Wave gates:</strong>` | ERR near-miss | — | step 0: tag removal; html.UnescapeString |
| R3-F `**Wave gates&#58;**` | ERR near-miss | — | step 0: `&#58;`→`:` (fixtura corrigida sem `\`) |
| R3-G `**Wave gɑtes:**` U+0251 | ERR near-miss | — | ɑ adicionado em nearMissHomoglyphs em ML-2J |
| R3-H prose antes de header válido | cmds=['echo R3H_REAL','exit 1'] | — | boundGateFence para em GatesHeaderRe; near-miss não dispara |

**Contra-braço ctrl-canonical:** `**Wave gates:**` + ` ```bash ` → `cmds=['echo REAL','exit 1']` (canônico, executado).

**Medição R3-H FP (correção do R3-H):** Wave 1 com prosa `Wave gates: see below` ANTES de `**Wave gates:**` válido + fence: near-miss loop encontra a prosa, chama `boundGateFence`; `boundGateFence` avança e encontra `GatesHeaderRe` → retorna false → near-miss não dispara. O header válido subsequente captura os gates → `cmds=['echo R3H_REAL','exit 1']`. Correto.

Todos os vetores R3-A a R3-H: **FECHADOS** com near-miss fence-anchored. Sem fence: `cmds=[]` sem erro — correto per item 2 (zero gates é legal quando nenhum cabeçalho válido existe).

---

### Vetores novos

#### N1 — `**Wave Gates:**` (G maiúsculo) com ` ```bash `

```
$ roadmap show n1-WaveGates-cap-fence --json (wave 1)
gates: ERR near-miss at line 23
```

Capturado. Classificação: **ACIDENTAL** (auto-capitalize, título Markdown). FECHADO pelo near-miss com fence.

Sem fence (`n1-WaveGates-cap-nofence`): `cmds=[]` sem erro. Correto per item 2.

---

#### N11 — `**wave gates:**` (tudo minúsculo) com ` ```bash `

```
$ roadmap show n11-allower-fence --json (wave 1)
gates: ERR near-miss at line 23
```

Capturado. Classificação: **ACIDENTAL** (Markdown editado sem capitalização). FECHADO.

---

#### N2 — Token numérico inserido: `**Wave 1 gates:**` / `**Gates da wave 0:**` (BLOQUEANTE)

```
$ roadmap show n2-Wave1gates-fence --json (wave 1)
gates: {'commands': []}
```

Sem erro. Zero gates silenciosamente. Contra-braço: `ctrl-canonical` → `cmds=['echo REAL','exit 1']`.

**Mecanismo:** skeleton de `**Wave 1 gates:**` = `wave1gates:` (após strip de `*`, espaço até `:`). `wave1gates:` não começa com nenhum target (`wavegates:`, `wavegate:`, `gatesdawave:`, `gatedawave:`). `colonIdx` encontra `:`, então a path sem colon não é usada. Nenhum match → near-miss não dispara → zero gates.

**Evidência real no acervo.** Dois roadmaps em `done/` têm `**Gates da wave 0:**` seguido de ` ```bash ` imediato:
- `docs/roadmaps/done/ROADMAP-2026-09-17-jira-...md`: wave 0: `cmds=[]`, sem erro
- `docs/roadmaps/done/ROADMAP-2026-09-17-gate-escreve-...md`: wave 0: `cmds=[]`, sem erro

Ambos medidos com `roadmap show --json` → `cmds=[]`. O binário atual conta zero para esses roadmaps. O mecanismo é o mesmo de N2. Mesmo padrão: N3 (`**Gates da wave 1:**` → skeleton `gatesdawave1:`, mesma causa).

**Classificação:** ACIDENTAL — inserir o número da wave no cabeçalho é o gesto natural de quem documenta wave por wave. Medido no acervo deste repositório. ALTO.

**Por que é BLOQUEANTE:** per ADR item 5, "APROVA quando nenhum vetor ACIDENTAL conta zero gates em silêncio". N2 é acidental e conta zero gates em silêncio, com fence explicitamente presente. Fora do residual do item 4 (não é homóglifo nem ofuscação deliberada).

---

#### N3 — `**Gates da wave 1:**` com ` ```bash `

```
$ roadmap show n3-GatesdaWave1-fence --json (wave 1)
gates: {'commands': []}
```

Mesma causa que N2: skeleton `gatesdawave1:` ≠ `gatesdawave:`. **ACIDENTAL**, ALTO. Mesma causa → mesmo ML.

---

#### N4 — `**Wave gates (blocking):**` com ` ```bash `

```
$ roadmap show n4-WavegatesBlocking-fence --json (wave 1)
gates: {'commands': []}
```

Skeleton: `wavegates(blocking):` — `(` inserido antes de `:`, bloqueia o prefix match de `wavegates:`. Classificação: **ACIDENTAL** — um LLM documentador pode anotar o cabeçalho como `**Wave gates (blocking):**`. Menos natural que N2 mas dentro do espaço acidental. Mesma causa de mecanismo que N2 (token inserido antes do colon). BAIXO. Per item 5: zero gates em silêncio + acidental = **BLOQUEANTE** (mesma causa e mesmo ML que N2, menor prioridade).

---

#### N5 — `**Gates:**` (paráfrase, palavra "Wave" omitida)

```
$ roadmap show n5-Gates-alone-fence --json (wave 1)
gates: {'commands': []}
```

Skeleton: `gates:`. Nenhum target começa com `gates:`. **Classificação: paráfrase** — uma palavra diferente do cabeçalho, equivalente a apagar o cabeçalho (ADR item 1). Não é near-miss. Não bloqueia.

---

#### N6 — `**Gates for this wave:**` (paráfrase)

```
$ roadmap show n6-GatesForThisWave-fence --json (wave 1)
gates: {'commands': []}
```

Skeleton: `gatesforthiswave:`. Sem match. **Classificação: paráfrase** — cabeçalho totalmente diferente. Não bloqueia.

---

#### N7 — `**Wave gates** (run by Zeus)` (forma sem colon + sufixo)

```
$ roadmap show n7-WavegatesSuffix-nocolon --json (wave 1)
gates: {'commands': []}
```

Skeleton no-colon path: `wavegates(runbyzeus)` ≠ `wavegates` (exact-equality falha). **Classificação:** tensão irreconciliável entre item 3 e item 5. Item 3 manda exact-equality na forma sem colon para evitar FPs de prosa — isso captura apenas `**Wave gates**` sozinho. Item 5 proíbe vetor acidental com zero gates em silêncio — `**Wave gates** (run by Zeus)` é variante acidental que um LLM pode gerar. Os dois itens não podem ser satisfeitos simultaneamente para esta forma: relaxar exact-equality para prefix-match introduz FPs de prosa; manter exact-equality deixa N7 aberto. Uma das duas ADR exigências precisa ser emendada pelo arquiteto. BLOQUEANTE-ADR (não é código, é conflito de especificação).

---

#### N8 — `**Wave Gates:**` + ` ```shell ` (assimetria de fence) (BLOQUEANTE)

```
$ roadmap show n8-WaveGates-shell-fence --json (wave 1)
gates: {'commands': []}
```

**Mecanismo:** skeleton de `**Wave Gates:**` = `wavegates:` → near-miss MATCH. `boundGateFence` busca `strings.TrimSpace(lines[j]) == "` ` ` `bash"` (literal). ` ```shell` → TrimSpace → ` ```shell` ≠ ` ```bash` → não ligado. Near-miss não dispara. Zero gates silenciosamente.

**Classificação:** ACIDENTAL — LLM que faz title-case nos cabeçalhos tende a emitir também ` ```shell` ou ` ```sh` (info-strings comuns para scripts shell). Os dois erros juntos resultam em zero gates sem diagnóstico.

**Por que é BLOQUEANTE:** per item 5. Vetor acidental, zero gates em silêncio. Fora do residual do item 4.

**Contra-braço A:** `fence-sh-only.md` (cabeçalho canônico `**Wave gates:**` + ` ```sh`, sem ` ```bash` posterior) → `roadmap show --json` retorna `ERR "gates block not found: ...not followed by a \`\`\`bash fence..."`. Ou seja: o cabeçalho canônico falha corretamente com ` ```sh` — fail-closed, explícito.

**Contra-braço B:** `n1-WaveGates-cap-fence.md` (near-miss `**Wave Gates:**` + ` ```bash`) → `ERR near-miss at line 23`. Ou seja: a near-miss com ` ```bash` falha corretamente.

Juntos os dois braços falsificam a premissa do item 2 ("nada se perde sem cerca ligada"): com near-miss + ` ```shell`, nenhuma cerca está ligada, mas zero gates são perdidos. O problema ocorre porque `boundGateFence` retorna not-ok (nenhum ` ```bash` ligado), a near-miss loop não dispara, e `ParseGatesLines` não emite erro.

**Mecanismo do fix candidato:** a premissa do item 2 ("um quase-cabeçalho só é erro se tiver ` ```bash` ligado") falha para N8: com ` ```sh`, nenhuma fence está ligada, mas zero gates são perdidos silenciosamente. O fix correto exige uma emenda ao item 2: qualquer fence opener após a near-miss deveria contar como ligação (não apenas ` ```bash`). A forma certa é inverter o critério: se `boundGateFence` encontrar QUALQUER fence opener antes do próximo cabeçalho válido ou heading, disparar o near-miss error. Isso tornaria N8 fail-closed. Esta é uma emenda ao ADR item 2 e deve ser declarada como tal pelo implementador.

---

#### N9 — Canonical header + ` ```sh ` gate + ` ```bash ` exemplo posterior (BLOQUEANTE D8)

```
$ roadmap show n9-canonical-sh-then-bash-example --json (wave 1)
gates: {'commands': ['echo EXAMPLE_PICKED']}

$ barrier n9-canonical-sh-then-bash-example --wave 1 --trust-local-gates --json
"checks":[...,{"name":"gates","status":"passed","commands":["echo EXAMPLE_PICKED"],"evidence":["echo EXAMPLE_PICKED: exit 0"],...}]
```

**Mecanismo:** `**Wave gates:**` é header válido. `boundGateFence` avança: linha ` ```sh` — TrimSpace → ` ```sh` ≠ ` ```bash` — skipped. As linhas DENTRO do bloco ` ```sh` estão marcadas como `fenced[j]=true` → skipped. A linha de fechamento ` ``` ` não é ` ```bash` → skipped. Depois da cerca, encontra ` ```bash` do exemplo → BOUND. O exemplo executa como gate.

**Contra-braço `ctrl-canonical`:**
```
barrier ctrl-canonical --wave 1 --trust-local-gates --json
"gates":{"status":"blocked","commands":["echo REAL","exit 1"],"evidence":["echo REAL: exit 0"],"failures":["exit 1: exit 1"]}
```
O gate real (`exit 1`) bloqueia corretamente.

**Classificação:** ACIDENTAL — ` ```sh` é info-string muito comum; qualquer pessoa ou LLM pode usar `sh` em vez de `bash`. O autor tem `**Wave gates:**` correto e um bloco ` ```sh` com os comandos reais, mas o barrier executa o exemplo seguinte.

**Por que é BLOQUEANTE:** D8 — o gate real "some em silêncio". O barrier executa `echo EXAMPLE_PICKED` e reporta `status=passed`; o gate real (`exit 1`) nunca corre. R1 classificou A2 (mesmo mecanismo via pre-scan mask: "exemplo executa em vez do gate real") como BLOQUEANTE-ALTO. N9 é A2 pelo caminho da info-string: a mesa é a mesma, a porta de entrada é diferente. Classificado **BLOQUEANTE-ALTO** consistentemente com A2.

**Mecanismo do fix candidato:** `boundGateFence` para quando encontra qualquer fence opener diferente de ` ```bash` em vez de continuar. Isso tornaria N9 fail-closed ("gates block not found").

---

#### N10 — CRLF line endings

```
$ roadmap show n10-crlf --json (wave 1)
gates: {'commands': ['echo CRLF_GATE', 'exit 1']}
```

Gate executado corretamente. CRLF não afeta o parsing. PASS.

---

#### Variantes de fence info-string após cabeçalho válido (sem exemplo posterior)

| Info-string | Resultado |
|---|---|
| ` ```sh` | ERR "gates block not found: ...not followed by a \`\`\`bash fence..." |
| ` ```shell` | ERR "gates block not found..." |
| `` ` ```Bash` `` | ERR "gates block not found..." |
| `` ` ``` bash` `` (espaço) | ERR "gates block not found..." |
| `` ` ```bash title=gate` `` | ERR "gates block not found..." |

Todos fail-closed quando NÃO há outro ` ```bash ` no mesmo wave. O problema N9 só ocorre quando há um ` ```bash ` posterior (exemplo).

---

### Falsos Positivos — Varredura do acervo

Instrumento: `roadmap show --json` em todos os roadmaps em `wip/` e `done/` (sem `abandoned/`, sem `analyzing/backlog/blocked/` — todos vazios, confirmado). Iteração por basename sem `.md` (evita "ambiguous match" do match por path relativo vs. busca por nome). Por wave, não por roadmap.

Estados existentes e contagem:
- `wip/`: 1 arquivo
- `done/`: 264 arquivos confirmados (`ls docs/roadmaps/done/ | grep -c '\.md$'`)
- `analyzing/`, `backlog/`, `blocked/`: vazios

Arquivos não resolvidos em `done/` (3 casos de match ambíguo):
- `ROADMAP-2026-06-19-architect-command-guidelines` (prefixo de dois arquivos): ambos resolvidos separadamente (ml-1b e o base) → waves=0 cada, sem near-miss
- `ROADMAP-2026-06-20-attention-hooks-agent-clis` (prefixo de dois arquivos): node separado → waves=0 sem near-miss; arquivo base (`…-clis.md`) não resolve — verificado diretamente: não tem `**Gate da wave`
- `ROADMAP-2026-07-19-global-adrs-governance` (prefixo de três arquivos): dois resolvem separadamente (ml-1b, ml-2b) → waves=0 sem near-miss; arquivo base (`…-governance.md`): não tem `**Gate da wave`

Mais 3 arquivos formato pré-wave (antigo): `agent-rules-inject`, `analyzing-state-ml-status-rules` etc. — `show --json` retorna estrutura com `waves=[]`, desconsiderados na contagem.

| Estado | Arquivos | Waves totais | Waves com near-miss |
|---|---|---|---|
| wip | 1 | 5 | **0** |
| done | 264 | 748 | **2** |
| analyzing/backlog/blocked | 0 | 0 | — |

Os 2 hits de near-miss em `done/` são:
- `ROADMAP-2026-09-11-by-agent-req-new-...`: wave 0, `**Gate da wave:**` (singular) + ` ```bash ` imediato
- `ROADMAP-2026-09-11-self-test-do-ratchet-...`: wave 0, idem

Skeleton = `gatedawave:` → matches target `gatedawave:`. Near-miss correto — a forma singular era usada antes da padronização. Ambas são wave 0; o binário atual conta zero para esses casos.

**Grep para os 3 roadmaps não-near-miss com `**Gate da wave:**` singular:** `leniencia` (linha 60: `**Gate da wave 0:**` + texto inline, sem fence), `conclusao` (linha 573: em prosa dentro de um parágrafo), `teste-e-gate` (linha 115: `**Gate da wave:**` + comando inline, sem fence). Todos confirmados: sem ` ```bash ` ligado → near-miss não dispara, correto per item 2.

**Reconciliação com R3 (que contou 3 hits):** R3 foi escrito no commit `a413988a` (ADR + rodada 3), anterior ao commit `51ada601` (ML-2J, que adicionou fence-anchoring). No código de R3, near-miss disparava independente de fence. O 3º hit de R3 é `ROADMAP-2026-09-22-teste-e-gate-...`: tem `**Gate da wave:**` (singular) na linha 115 mas sem ` ```bash ` fence seguinte — confirmado: R4 binary retorna `error=none`. Sob ML-2J, não é mais near-miss error (correto per item 2). R4: 2 near-miss hits — correto sob a semântica atual.

**FP1 construído** (confirmação de que a varredura funciona):

Fixture `fp1-prose-with-unrelated-bash.md`: linha de prosa `Gates da wave: rodados pelo Zeus` (sem `**`) + ` ```bash ` de exemplo não relacionado. Skeleton = `gatesdawave: rodados pelo zeus` → HasPrefix(`gatesdawave:`) = true → fence bound → near-miss dispara:

```
$ roadmap show fp1-prose-with-unrelated-bash --json (wave 1)
ERR: near-miss gates header at line 23...
```

FP1 é fail-closed por design do item 2: prosa com texto similar ao cabeçalho gera um erro explícito, não silência. 0 hits em wip/ → nenhuma regressão em CI. Nuisance documentado (item 2 declarou este tradeoff).

---

### Dead code — `nearMissSkeletonMatches` não chamada em produção

`nearMissSkeletonMatches` (linha 1540) é definida mas nenhum código de produção a chama. `boundGateFence` tem comentário "(or another near-miss) claims the fence from here on" (linha 1586) mas o código só para em `GatesHeaderRe.MatchString` — a parada em "outra near-miss" não foi implementada. Efeito: dois near-misses em sequência → ambos têm fence bound → o primeiro retorna erro (correto), o segundo nunca é checado. Fail-closed. Misatribuição de linha, não silenciamento.

```
$ roadmap show deadcode-two-nearmiss --json (wave 1)
ERR near-miss at line 23 (Wave Gates: linha 23) — correto, fail-closed
```

Severidade: LOW. Código morto, comentário diverge, mas comportamento é fail-closed.

---

### Tabela "residual, item 4"

Vetores abaixo são da classe do item 4 (ADR — "homóglifo fora da tabela fechada... e qualquer ofuscação deliberada"). NÃO reprovam.

Nenhum vetor desta classe foi medido nesta rodada. O modelo de ameaça do item 1 é acidente, não adversário; o item 4 declara o residual como aceito. As classes abaixo são definidas pelo ADR — não foi construída fixture para nenhuma.

| Classe | Descrição ADR item 4 |
|---|---|
| Homóglifo Cirílico/Grego fora da tabela fechada | nearMissHomoglyphs cobre um subconjunto; qualquer confusável não listado sobrevive à pipeline. Não medido. |
| Scripts não-latino/não-cirílico/não-grego | Sem mapa de homoglyphs: árabe, coreano, etc. Equivale a apagar o cabeçalho (ADR item 1). |
| Ofuscação deliberada | Zero-width splicing, fragmentação de bytes, homoglyphs raros. Defesa é revisão do diff. Fora do modelo de acidente. |

---

### Resumo tabular dos vetores R4

| ID | Vetor | Resultado com fence | Resultado sem fence | Classificação | Severidade |
|---|---|---|---|---|---|
| R3-A `**Ԝave gates:**` U+051C | ERR near-miss | cmds=[] ok | FECHADO | — |
| R3-A `**Wave gΑtes:**` U+0391 | ERR near-miss | — | FECHADO | — |
| R3-A `**Wave gatΕs:**` U+0395 | ERR near-miss | — | FECHADO | — |
| R3-B VS1 U+FE00 | ERR near-miss | — | FECHADO | — |
| R3-C U+2800 Braille | ERR near-miss | — | FECHADO | — |
| R3-D sem colon | ERR near-miss | — | FECHADO | — |
| R3-E ∶ U+2236 | ERR near-miss | — | FECHADO | — |
| R3-F HTML tags | ERR near-miss | — | FECHADO | — |
| R3-F &#58; entidade | ERR near-miss | — | FECHADO (fixture corrigida) | — |
| R3-G ɑ U+0251 | ERR near-miss | — | FECHADO | — |
| R3-H FP prosa + header válido | cmds corretos (sem FP) | — | FECHADO | — |
| N1 `**Wave Gates:**` G maiúsculo | ERR near-miss | cmds=[] ok | FECHADO (detectado) | — |
| N11 `**wave gates:**` minúsculo | ERR near-miss | — | FECHADO | — |
| **N2** `**Wave 1 gates:**` | **cmds=[] SILÊNCIO** | — | **ABERTO ACIDENTAL** | **ALTO BLOQUEANTE** |
| **N3** `**Gates da wave 1:**` | **cmds=[] SILÊNCIO** | — | **ABERTO ACIDENTAL** | **ALTO (mesma causa N2)** |
| **N4** `**Wave gates (blocking):**` | **cmds=[] SILÊNCIO** | — | **ABERTO ACIDENTAL (Mecanismo A)** | **BAIXO BLOQUEANTE** |
| N5 `**Gates:**` (paráfrase) | cmds=[] | — | Paráfrase — não é near-miss | — |
| N6 `**Gates for this wave:**` | cmds=[] | — | Paráfrase | — |
| **N7** `**Wave gates** (run by Zeus)` | cmds=[] | — | **ABERTO ACIDENTAL — conflito ADR item 3 × item 5** | **BLOQUEANTE-ADR** |
| **N8** `**Wave Gates:**` + ` ```shell` | **cmds=[] SILÊNCIO** | n/a | **ABERTO ACIDENTAL** | **MÉDIO BLOQUEANTE** |
| **N9** canonical + ` ```sh` + example | **cmds=['echo EXAMPLE_PICKED']** | n/a | **ABERTO ACIDENTAL (Mecanismo B)** | **ALTO BLOQUEANTE** |
| N10 CRLF | cmds corretos | — | PASS | — |
| Dead code nearMissSkeletonMatches | misatribuição, fail-closed | — | LOW | — |

---

### Veredito da Rodada 4

**REPROVA**

Motivos de bloqueio:

1. **N2/N3 — Inserção de token numérico: `**Wave 1 gates:**`, `**Gates da wave 0:**`** (ALTO, ACIDENTAL): skeleton `wave1gates:` / `gatesdawave0:` não casa nenhum target. Zero gates em silêncio com fence presente. Evidência no acervo: dois roadmaps em `done/` com `**Gates da wave 0:**` + ` ```bash ` imediato, ambos com `cmds=[]`. Fora do residual do item 4. Viola item 5.

2. **N8 — `**Wave Gates:**` + ` ```shell ` (assimetria de fence)** (MÉDIO, ACIDENTAL): near-miss skeleton `wavegates:` casa o target, mas `boundGateFence` só liga em ` ```bash ` → near-miss não dispara → zero gates em silêncio. Um LLM que faz title-case e usa ` ```shell` produz este resultado. Fora do residual do item 4. Viola item 5.

3. **N9 — Canonical header + ` ```sh ` gate + ` ```bash ` exemplo posterior** (ALTO, ACIDENTAL): `boundGateFence` skipa o bloco ` ```sh ` e liga no exemplo seguinte. Barrier reporta `gates.status=passed` executando `echo EXAMPLE_PICKED`; o gate real (`exit 1`) nunca corre. R1 classificou A2 (mesmo mecanismo via pre-scan mask) como BLOQUEANTE-ALTO; N9 é A2 pelo caminho da info-string. **BLOQUEANTE-ALTO.**

4. **N4 — `**Wave gates (blocking):**`** (BAIXO, ACIDENTAL): skeleton `wavegates(blocking):` não casa nenhum target. Zero gates em silêncio. Mesma causa que N2 (Mecanismo A), menor prioridade. Entra no mesmo ML que N2.

5. **N7 — `**Wave gates** (run by Zeus)` no-colon com sufixo** (BLOQUEANTE-ADR): item 3 manda exact-equality na forma no-colon; item 5 proíbe vetor acidental com zero gates em silêncio. Os dois não podem ser satisfeitos simultaneamente para esta forma. Requer emenda ao ADR — não é defeito de código isolado.

Achados não bloqueantes:
- Dead code `nearMissSkeletonMatches`: LOW, fail-closed.
- FP1 (prosa bound a exemplo): fail-closed por design do item 2, 0 hits em wip/ no acervo.

Os vetores se dividem em dois mecanismos distintos — ambos sob D8, mesma REQ, MLs separados:

**Mecanismo A — token extra no cabeçalho frustra a normalização** (N2, N3, N4, N7): `gatesNearMissSkeleton` reduz o cabeçalho a um skeleton que não casa nenhum target porque um token (número de wave, parêntese, sufixo) é inserido antes ou depois do colon. Fix site: `gatesNearMissSkeleton` / lista de targets. Teste: se eu corrigir A, N2+N3+N4 fecham; N7 fecha somente se exact-equality for relaxada (emenda de ADR, não só código). Se eu corrigir A sem emendar o ADR, N7 permanece aberto — portanto o ML deve declarar qual dos dois conflitos resolve.

**Mecanismo B — `boundGateFence` só liga ` ```bash`, skipa outras fences** (N8, N9): O loop de near-miss não dispara quando `boundGateFence` retorna not-ok; `ParseGatesLines` não emite erro. Para N9, `boundGateFence` retorna ok ligando a um ` ```bash` posterior (exemplo), executando-o. Fix site: `boundGateFence` (`internal/roadmapdoc/roadmapdoc.go:1578`). Regra unificada (emenda ao ADR item 2): **a primeira fence opener após um cabeçalho ou quase-cabeçalho é a cerca ligada, independente da info-string.** Consequências: (a) cabeçalho válido + fence não-bash → erro "gates block not found" (já é o comportamento atual); (b) quase-cabeçalho + qualquer fence → near-miss error (fix de N8); (c) quase-cabeçalho ou cabeçalho válido para no primeiro fence opener encontrado → nunca avança para exemplo posterior (fix de N9). Uma única regra fecha N8 e N9.

Por Regra Dura: mesma causa (D8) → mesma REQ → dois MLs novos (ML-A para mecanismo A; ML-B para mecanismo B).

---

### Saída dos comandos git (Rodada 4)

```
$ git status --short
 M docs/seguranca/2026-11-08-redteam-leitor-bilingue.md

$ git diff --cached --stat
(vazio)
```

---

## Rodada 5 (2026-11-08) — verificação

**Binário branch:** `go build -o $SCRATCH/hades5/trackfw ./cmd/trackfw` sobre HEAD `7758b8ce` (ML-2L). Versão: `11.5.1`.
**Escopo:** verificação fechada nos mecanismos A e B da Rodada 4, corrigidos no ML-2L. Critério ADR itens 1–5 (emenda rodada 4): APROVA quando nenhum vetor ACIDENTAL dos mecanismos A/B faz o gate sumir em silêncio.
**Scratchpad:** `/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/a247cc38-463b-4f99-8cb2-7400f9dec171/scratchpad/hades5/`
**Instrumento:** `roadmap show --json` (wave 1) + `barrier --wave 1 --trust-local-gates` (exit codes). Fixtures em `$SCRATCH/hades5/proj/docs/roadmaps/wip/` sem `git init` (ml_scope: skipped em todos).

---

### Reverificação N2, N3, N4, N7, N8, N9

| Vetor (Rodada 4) | Comando | Saída (wave 1 gates) | rc | Mecanismo do fix | Veredito R5 |
|---|---|---|---|---|---|
| N2 `**Wave 1 gates:**` + ` ```bash` | `barrier n2-wave1gates-fence --wave 1` | near-miss error at line 23 | 2 | step 4: `1` é dígito, removido do prefix → skeleton `wavegates:` → near-miss | **FECHADO** |
| N3 `**Gates da wave 0:**` + ` ```bash` | `barrier n3-gatesdawave0-fence --wave 1` | near-miss error at line 23 | 2 | step 4: `0` removido → `gatesdawave:` → near-miss | **FECHADO** |
| N4 `**Wave gates (blocking):**` + ` ```bash` | `barrier n4-wavegates-blocking --wave 1` | near-miss error at line 23 | 2 | step 4: `(blocking)` removido → `wavegates:` → near-miss | **FECHADO** |
| N7 `**Wave gates** (run by Zeus)` + ` ```bash` | `barrier n7-wavegates-suffix --wave 1` | near-miss error at line 23 | 2 | step 4 no-colon path: `(run by Zeus)` removido → skeleton `wavegates` = exact-equality target → near-miss | **FECHADO** |
| N8 `**Wave Gates:**` + ` ```shell` fence | `barrier n8-WaveGates-shell --wave 1` | near-miss error at line 23 | 2 | `boundGateFence` para no primeiro opener (` ```shell`): near-miss skeleton `wavegates:` → match → error | **FECHADO** |
| N9 canonical + ` ```sh` gate | `barrier n9-canonical-sh --wave 1` | gates fence error "found ` ```sh`" | 2 | `boundGateFence` para em ` ```sh`; caller verifica info-string ≠ `bash` → error; nenhum bloco executado | **FECHADO** |
| N9b canonical + ` ```sh` gate + ` ```bash` exemplo | `barrier n9b-canonical-sh-then-bash-example --wave 1` | gates fence error "found ` ```sh`" | 2 | boundGateFence para no ` ```sh` (primeiro opener) e nunca avança ao exemplo; `echo EXAMPLE_PICKED` não executado | **FECHADO** |

**Contra-braço ctrl-canonical:** `**Wave gates:**` + ` ```bash` + `exit 1` → barrier rc=1, `gates: blocked`, `commands=['exit 1']` — gate executado corretamente.

---

### Etiquetas de info-string (mecanismo B, cabeçalho canônico)

Todos via `barrier <fixture> --wave 1 --trust-local-gates`:

| Info-string | Linha no fixture | Saída barrier | rc | Classificação |
|---|---|---|---|---|
| ` ```sh` | 25 | "gates fence ... must open with exactly ` ```bash` (found ` ```sh`)" | 2 | fail-closed, erro visível |
| ` ```shell` | 25 | idem (found ` ```shell`) | 2 | fail-closed |
| ` ```Bash` | 25 | idem (found ` ```Bash`) | 2 | fail-closed — case-sensitive por design |
| ` ```bash ` (trailing space) | 25 | `gates: passed`, `commands=['echo TRAILSPACE']` | 1 (gate OK, outros bloqueiam) | ACEITO — TrimSpace remove espaço final; gate executa. Ficção, não silêncio. |
| `~~~bash` | 25 | "gates fence ... (found `~~~bash`)" | 2 | fail-closed |

Nenhuma dessas etiquetas faz o gate sumir em silêncio. Os rc=2 são erro explícito, não `status=passed` com `commands=[]`.

---

### Vetores acidentais adicionais dentro dos mecanismos A e B

Medidos com `roadmap show --json` (wave 1):

| Vetor | Mecanismo | Resultado wave 1 | Classificação |
|---|---|---|---|
| ` ```bash  # comment` (info-string com comentário) | B | error "found ` ```bash # a comment`" | fail-closed — TrimSpace não remove o comentário |
| ` ````bash` (quatro crases) | B | error "found ` ````bash`" | fail-closed — DetectFenceMarker detecta 4 crases como opener válido; comparação ≠ ` ```bash` |
| Cerca indentada 1–3 espaços (`   ```bash`) | B | `commands=['echo INDENTED_GATE']` — ACEITO | **FRICÇÃO** — TrimSpace remove o recuo; ` ```bash` indentado equivale ao raso por CommonMark. Gate executa corretamente. Sem silêncio. |
| `**Wave gates [ci]:**` + ` ```bash` | A | near-miss error at line 23 | FECHADO — step 4: `[ci]` removido → `wavegates:` |
| `**Wave gates (a (b)):**` + ` ```bash` | A | near-miss error at line 23 | FECHADO — step 4 rastreia profundidade; `(a (b))` inteiramente removido → `wavegates:` |

---

### Barrier ponta a ponta

**Caso 1 — cabeçalho válido + ` ```bash` + `exit 1`:**

```
$ barrier ctrl-canonical --wave 1 --trust-local-gates --json
{"checks":[...,{"name":"gates","status":"blocked","commands":["exit 1"],"evidence":[],"failures":["exit 1: exit 1"]},...]}
rc=1
```

`exit 1` foi executado; gates=blocked. Barrier REPROVA — correto.

**Caso 2 — cabeçalho válido + ` ```sh` gate:**

```
$ barrier ctrl-sh-only --wave 1 --trust-local-gates
trackfw barrier: gates fence at line 25 must open with exactly '```bash' (found "```sh") — the barrier never skips to a later fence (D8, ADR-2026-11-08)
rc=2
```

Barrier sai com rc=2 e erro explícito; nenhum bloco executado.

---

### Tabela de residual / fora do escopo

| Classe | ADR item | Status |
|---|---|---|
| Homóglifo fora da tabela fechada (scripts não-latino/cirílico/grego) | Item 4 | Residual declarado — equivale a apagar o cabeçalho |
| Ofuscação deliberada (zero-width splicing, fragmentação de bytes) | Item 4 | Residual declarado — defesa é revisão de diff |
| Cerca indentada 1–3 espaços: aceita sem aviso | B (by design) | Ficção — CommonMark válido; gate executa; sem silêncio |
| ` ```bash` com trailing space: aceita sem aviso | B (by design) | Ficção — TrimSpace intencional |
| rc=2 não emite JSON válido com `--json` | B infra | Ficção — barrier falha antes de avaliar; formato de erro é texto, não JSON |

Nenhum item desta tabela faz gate sumir em silêncio.

---

### Veredito da Rodada 5

**APROVA**

Todos os vetores dos mecanismos A e B da Rodada 4 (N2, N3, N4, N7, N8, N9) estão fechados no HEAD `7758b8ce`. Nenhum vetor acidental testado nesta rodada (dentro dos mecanismos A e B) faz o gate sumir em silêncio — todos produzem erro visível (rc=2) ou executam o gate corretamente.

Ficções documentadas (nenhuma bloqueia o merge):
1. ` ```bash ` com trailing space é aceito como gate válido sem aviso. TrimSpace intencional; o author não recebe diagnóstico, mas o gate executa.
2. Cerca indentada 1–3 espaços é aceita como gate válido sem aviso. CommonMark por design.
3. Etiquetas não-bash e ` ```bash  # comment` e ` ````bash` produzem rc=2 com erro legível em texto; o formato não é JSON mesmo com `--json`. Ficção de integração para orquestradores que dependem de `--json` em todos os casos de erro.

---

### Saída dos comandos git (Rodada 5)

```
$ git status --short
(nada — working tree clean)

$ git diff --cached --stat
(vazio)
```

