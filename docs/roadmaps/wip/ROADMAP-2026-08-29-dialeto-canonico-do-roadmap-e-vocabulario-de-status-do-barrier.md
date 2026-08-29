---
status: wip
date: 2026-08-29
req: "docs/req/REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Dialeto canônico do roadmap e vocabulário de status do `barrier`

> Created: 2026-08-29 | Status: wip

## Context

REQ: `REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md`
ADR: `ADR-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-que-o-barrier-reconhece.md`

**Um roadmap gerado pelo `trackfw roadmap new` e preenchido exatamente como o próprio template
instrui é reprovado pelo `barrier` em dois checks.** Medido com o binário 7.3.0:

```
- ML-1A: not complete (status: done)      ← mls_complete
✗ acceptance_evidence: blocked
- ML-1A: no acceptance block              ← acceptance_evidence
```

Dois defeitos de natureza diferente: o cabeçalho é problema de **idioma** (gerador escreve
`**Acceptance criteria:**`, barrier procura `**Critérios de aceite:**`); o status é problema de
**representação** (gerador escreve `pending`, barrier exige que a linha contenha `✅`).

Nenhum gate pega porque a paridade entre os 3 CLIs está intacta — os três erram igual. O contrato
quebrado é gerador↔verificador.

## Acceptance Criteria

Consolidado — AC1 a AC12 da REQ. **AC12 é a que define a REQ:** ciclo `roadmap new` → preencher →
`barrier passed`, com CLI real, sem edição manual.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça deste roadmap
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap. Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração.** O contrato gerador↔`barrier` tem **quantos** tokens, não só os dois
   já achados? Enumere **todos** os cabeçalhos e marcadores que o `barrier` parseia
   (`internal/commands/barrier.go:160-171` e equivalentes Node/Python) e confronte, um a um, com o
   que os 3 geradores escrevem (`internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
   `pypi/trackfw/generators/roadmap.py`). Já sabidos: `**Acceptance criteria:**` vs
   `**Critérios de aceite:**` (diverge); `**Status:**` valor `pending` vs exigência de `✅`
   (diverge); `**Gates da wave:**` (concorda). Faltam: `^## Wave <label>`, `^### ML-\S+`,
   `^- \[ \]` / `^- \[.\]`, `^\*\*` como delimitador de bloco. **Para cada um, diga se o gerador
   produz forma que o parser aceita** — e não confie em que a lista acima esteja completa.
   > A Wave 0 da REQ anterior declarou enumeração fechada sobre um padrão de busca incompleto e
   > perdeu metade da superfície. Não repita: enumere pelo **parser**, não pela memória.
2. **Modelo de ameaça.** O vocabulário de status vai **crescer** (de `✅` para `✅|done|Concluído`) e
   o mecanismo vai mudar de `contains` para primeiro-token. Quem faz um ML **não** concluído passar
   por concluído sem quebrar nenhuma regra escrita? Cubra no mínimo: `não done`,
   `pending (era done)`, `notdone`, `done-not-really`, `**Status:** ` seguido de linha vazia,
   status com marcador dentro de código inline (`` `done` ``), status com caractere invisível ou
   zero-width antes do token, `✅` em posição não inicial (`⬜ Pendente ✅`), e status multilinha.
   Lembre que este é um check que **libera wave** — falso positivo aqui é trabalho incompleto sendo
   dado como pronto.
3. **Alvos de falsificação nas duas direções.** Para cada mudança: o que quebra se regredir (volta a
   exigir só `✅`, ou só o cabeçalho PT), **e** o que quebra se regredir para o lado oposto
   (aceita qualquer status não vazio; aceita `**Status:** não done`; o cabeçalho novo passa a casar
   dentro de bloco de código ou de prosa).
4. **Residual declarado.** O que este desenho aceita não cobrir. Inclua, no mínimo: roadmaps
   históricos com status fora do vocabulário fechado (`feito`, `ok`); a dupla forma de cabeçalho
   como superfície permanente; e o fato de o `barrier` passar a conhecer dois idiomas.
**Critérios de aceite:**
- [x] As quatro seções respondidas com evidência (comando + saída), não asserção de uma linha
- [x] A enumeração cobre **todos** os tokens do parser, não só os dois já conhecidos
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**
```bash
# Wave 0 gate — o conjunto de regexes de parsing do barrier tem que ser o que o ML-0A enumerou.
# Superfície nova no parser sem passar pela Wave 0 reabre a wave.
# Uma linha só: `parseGates` (barrier.go/js/py) executa cada linha não-comentário como um `sh -c`
# INDEPENDENTE — a versão original deste gate (4 linhas, `set -eu`/`n=$(...)`/`[ "$n" -eq 9 ]`
# separados) nunca funcionou, porque `$n` não sobrevive entre invocações separadas. Achado e
# corrigido pelo hades-tf no ML-0A, reproduzido ao vivo contra `./bin/trackfw barrier` real.
n=$(sed -n '/^var (/,/^)/p' internal/commands/barrier.go | grep -c 'regexp.MustCompile'); [ "$n" -eq 9 ] && echo "Wave 0 gate OK — 9 regexes de parsing enumeradas." || { echo "barrier.go tem $n regexes de parsing, ML-0A enumerou 9 — reabrir a Wave 0" >&2; exit 1; }
```

#### Resultado do ML-0A (hades-tf, 2026-08-29)

**Método:** enumeração pelo `var (...)` real de `internal/commands/barrier.go` (`grep -n "^var (" -A 40`),
leitura completa das 3 funções que os consomem (`mlStatusMarker`, `acceptanceEvaluate`, `parseGates`),
grep de todo `RegExp`/regex literal em `npm/src/commands/barrier.js` e `re.compile` em
`pypi/trackfw/commands/barrier.py`, e reprodução ao vivo contra o binário compilado
(`go build ./cmd/trackfw`) sobre roadmaps de sonda em `docs/roadmaps/wip/` de um projeto descartável —
não análise de código sozinha, conforme `feedback_verify_by_execution` do meu memory. Todos os
comandos abaixo rodaram de fato; a saída colada é saída real, não reconstruída.

##### 1. Completude de enumeração

`internal/commands/barrier.go:156-171` — bloco `var (...)` tem exatamente **9** `regexp.MustCompile`,
confirmando o gate da Wave 0:

```
$ grep -n "^var (" -A 40 internal/commands/barrier.go | head -18
156:var (
159:	waveHeadingRe = regexp.MustCompile(`^## Wave (\S+) `)
163:	waveLabelRe      = regexp.MustCompile(`^\d+(?:-[a-z0-9]+)?$`)
164:	mlHeadingRe      = regexp.MustCompile(`^### (ML-\S+)`)
165:	statusLineRe     = regexp.MustCompile(`^\*\*Status:\*\*(.*)$`)
166:	criteriaHeaderRe = regexp.MustCompile(`^\*\*Crit[eé]rios de aceite:\*\*`)
167:	unmetCriterionRe = regexp.MustCompile(`^- \[ \]`)
168:	criterionLineRe  = regexp.MustCompile(`^- \[.\]`)
169:	boldLineRe       = regexp.MustCompile(`^\*\*`)
170:	gatesHeaderRe    = regexp.MustCompile(`^\*\*Gates da wave:\*\*`)
171:)
```

| # | Regex Go | O que os 3 geradores escrevem | Casa? |
|---|---|---|---|
| 1 | `waveHeadingRe` `^## Wave (\S+) ` | `## Wave 0 — Threat Model`, `## Wave 1 — <name> (parallel MLs)` (`internal/generators/roadmap.go:53,169`; espelhado em `.js`/`.py`) | **Sim** — `\S+` captura `0`/`1`, exige espaço depois, presente |
| 2 | `waveLabelRe` `^\d+(?:-[a-z0-9]+)?$` | token capturado acima (`0`, `1`) | **Sim** — grafia numérica simples sempre válida |
| 3 | `mlHeadingRe` `^### (ML-\S+)` | `### ML-0A — Threat model for this roadmap`, `### ML-1A — %s` | **Sim** |
| 4 | `statusLineRe` `^\*\*Status:\*\*(.*)$` | `**Status:** pending` | **Sim, sintaticamente** — a linha casa; o *valor* capturado (`pending`) é o defeito já conhecido (não está no vocabulário atual `✅`/futuro `✅\|done\|Concluído` até a Wave 1 trocar o template) |
| 5 | `criteriaHeaderRe` `^\*\*Crit[eé]rios de aceite:\*\*` (só PT) | `**Acceptance criteria:**` (`roadmap.go:64,176,225`; `.js:31,495,558`; `.py:40`) | **Não** — diverge, já sabido, é o AC1–AC5 desta REQ |
| 6 | `unmetCriterionRe` `^- \[ \]` | `- [ ] <critério>` | **Sim** |
| 7 | `criterionLineRe` `^- \[.\]` | idem (cobre `- [x]` após marcado) | **Sim** |
| 8 | `boldLineRe` `^\*\*` | linha em branco seguida de `**Gates da wave:**` logo após a lista de critérios (`wave0Block`) | **Sim** — delimita corretamente o fim do bloco de aceite |
| 9 | `gatesHeaderRe` `^\*\*Gates da wave:\*\*` | `**Gates da wave:**` | **Sim** — concorda, fora de escopo desta REQ (Negative Scope) |

**Resultado:** confirma o que o roadmap já sabia (#4 status e #5 cabeçalho divergem) e fecha a
enumeração dos 7 tokens restantes — nenhum deles diverge do que os 3 geradores escrevem. **Não há
décimo token no parser Go**, verificado no arquivo inteiro, não só no bloco `var (...)`:

```
$ grep -c 'regexp.MustCompile' internal/commands/barrier.go
9
```

Whole-file count = block count = 9; não há `regexp.MustCompile` fora do bloco enumerado.

**Node e Python têm a MESMA cobertura semântica, mas NÃO têm literalmente "9 regexes" cada um — o
número 9 é um artefato de implementação do Go, não um invariante cross-CLI**, também verificado no
arquivo inteiro de cada um, não só por leitura:

```
$ grep -c 're.compile' pypi/trackfw/commands/barrier.py
11
$ grep -oE '/\^[^/]*/' npm/src/commands/barrier.js | sort -u | wc -l
11
```

- **Node** (`npm/src/commands/barrier.js`): 11 regex *literais* distintos (`WAVE_SCAN_RE`,
  `WAVE_LABEL_RE`, `/^## /` usado 2x em pontos de código diferentes, `/^### ML-/`, `/^### (\S+)/`
  — recaptura redundante do mesmo heading —, `/^### /`, `/^\*\*Status:\*\*(.*)$/`,
  `/^\*\*Crit[ée]rios de aceite:\*\*/`, `/^\*\*/`, `/^- \[/`, `/^- \[ \]/`), **e o cabeçalho de gates
  não é regex**: `barrier.js:169` usa `lines[i].trim() === '**Gates da wave:**'` — igualdade exata de
  string, não prefixo.
- **Python** (`pypi/trackfw/commands/barrier.py:97-109`): 11 constantes `re.compile` nomeadas
  (inclui `_ANY_WAVE_H2_RE` separado de `_WAVE_HEADING_RE`, e `_H2_BOUNDARY_RE` separado de
  `_H3_OR_H2_BOUNDARY_RE` — Go resolve os dois papéis reaproveitando `waveHeadingRe`/checagem de
  prefixo inline).
- **Divergência de parsing já existente HOJE, fora do escopo desta REQ mas achada pela enumeração
  pedida**: `**Gates da wave:** com sufixo` (ex.: `**Gates da wave:** (placeholder)`) **casaria** em
  Go e Python (`MatchString`/`.match` não ancoram `$`, então é prefixo) mas **não casaria** em Node
  (igualdade exata pós-`trim()`). Não é nova nem introduzida por este ADR — é pré-existente, e o
  Negative Scope da REQ proíbe mexer em `**Gates da wave:**`, então não é ação desta REQ; registro
  aqui porque a instrução era ir pelo parser e não pela memória, e a memória (a REQ/ADR) não citava
  isso. Recomendo abrir achado separado se algum roadmap real algum dia escrever um sufixo ali — hoje
  nenhum dos 143 do corpus o faz (`grep -rn "Gates da wave:\*\*." docs/roadmaps` não retorna sufixo).
- **Conclusão prática:** o gate da Wave 0 (`n -eq 9` sobre `barrier.go`) protege só o Go de crescer
  superfície silenciosamente. Ele **não** gate-ia Node/Python. Isso é aceitável porque a Wave 1 exige
  paridade comportamental nos 3 (AC3, `criteria: mesmo conjunto de formas aceitas`), verificada por
  teste, não por contagem de regex — mas o residual fica declarado na seção 4.

##### 2. Modelo de ameaça

Simulação executável do desenho do ADR (primeiro token do restante de `**Status:**`, `strip` de
acento via NFD, `casefold`, vocabulário fechado `{✅, done, concluido}`) — script em
`sim_first_token.py`, saída real:

```
'não done'                     tok='não'                -> complete=False
'pending (era done)'           tok='pending'            -> complete=False
'notdone'                      tok='notdone'            -> complete=False
'done-not-really'              tok='done-not-really'    -> complete=False
'empty after Status'           tok=None                 -> complete=False
'inline code `done`'           tok='`done`'             -> complete=False
'zero-width before'            tok='​done'         -> complete=False
'posicao nao inicial'          tok='⬜'                  -> complete=False
'DONE uppercase'                tok='DONE'               -> complete=True
'concluido sem acento'          tok='concluido'          -> complete=True
'tab separator'                 tok='done'               -> complete=True
'NBSP separator'                tok='done'               -> complete=True
'Concluido accent'              tok='Concluído'          -> complete=True
```

Cobertura dos 13 vetores pedidos, todos passam pelo desenho de primeiro-token **exceto os dois
achados abaixo, que não são deste script — são reproduzidos direto contra o binário real**:

- `não done`, `pending (era done)`, `notdone`, `done-not-really` → **rejeitados**, corretamente.
- `` `Status:` `` seguido de linha vazia → primeiro token é `None` → **rejeitado**, corretamente.
- marcador dentro de código inline (`` `done` ``) → o token inclui os backticks (`` `done` ``), não
  casa com `done` → **rejeitado**. Efeito colateral: se um autor *pretendesse* usar crase como ênfase
  em torno do marcador real, isso viraria falso-negativo (bloqueia trabalho concluído) — não é risco
  de segurança, é usabilidade; registrado no residual.
- caractere zero-width antes do token → o zero-width **não é whitespace Unicode**, então gruda no
  token (`​done` ≠ `done`) → **rejeitado**. Mesma classe de falso-negativo inofensivo.
- `✅` em posição não inicial (`⬜ Pendente ✅`) → primeiro token é `⬜` → **rejeitado pelo desenho
  novo**. **Isto já é explorável HOJE, contra o binário real, com o mecanismo atual (substring)** —
  reproduzido ao vivo:
  ```
  $ ./hades-barrier barrier docs/roadmaps/wip/posnaoinicial.md --wave 1 --trust-local-gates
  ✓ mls_complete: passed
  ✗ acceptance_evidence: blocked
      - ML-1A: no acceptance block
  ```
  Com `**Status:** ⬜ Pendente ✅`, o `Contains(marker, "✅")` de hoje (`barrier.go:554`) dá
  `mls_complete: passed` — falso positivo **já em produção**, não hipotético. É a prova concreta de
  por que a mudança para primeiro-token é necessária *já*, não só para o vocabulário novo.
- `DONE` maiúsculo, `concluido` sem acento, tab, NBSP → **aceitos**, conforme o ADR pede
  (case/acento-insensível).
- status multilinha → **não é vetor executável por construção**: `statusLineRe`/equivalentes usam
  `.` sem modo multilinha/DOTALL nos 3 runtimes (Go RE2 default, JS sem flag `s`, Python sem
  `re.DOTALL`), então o "restante da linha" nunca atravessa `\n`. Uma segunda linha só conta se
  **ela própria** começar com `**Status:**` — e `mlStatusMarker` já para no primeiro match. Risco
  nulo por desenho, não por mitigação adicional.

**Vetor não coberto pela lista de 13, achado durante a reprodução, e o mais grave dos quatro que
reporto no fechamento** — **sombreamento por bloco de código (`mlStatusMarker`/`acceptanceEvaluate`
não têm consciência de cerca ```` ``` ````, só `parseGates` tem)**. Reproduzido ao vivo, roadmap real,
binário real:

> ### ML-1A — probe
> Example of the bug we are documenting:
> ```
> **Status:** done
> ```
> **Status:** pending
> ...

*(bloco acima em blockquote de propósito — a versão sem `>` na frente de cada linha seria, ela
mesma, um `### ML-\S+` real para o parser do `barrier`, exatamente o defeito que esta seção
descreve. A reprodução real usou um arquivo `.md` separado, `forged.md`, fora deste roadmap; ver
`docs/agents-working-context.md` desta sessão para o caminho de sonda usado.)*

```
$ ./hades-barrier barrier forged.md --wave 1 --trust-local-gates
✗ mls_complete: blocked
    - ML-1A: not complete (status: done)
✓ acceptance_evidence: passed
```

Hoje (mecanismo `contains("✅")`), a linha `**Status:** done` dentro da cerca é lida **primeiro** (o
loop de `mlStatusMarker` para no primeiro `**Status:**` que encontra, cercado ou não) e vence sobre a
linha `**Status:** pending` real, que nunca é alcançada — hoje isso **falha fechado** (bloqueia um ML
que talvez estivesse `pending` mesmo, ou mascara o status real, mas nunca libera indevidamente,
porque `"done"` sem `✅` não casa a regra atual).

**Sob o desenho proposto (primeiro token = marcador válido), o mesmo roadmap passaria a reportar
`mls_complete: passed`** — a linha cercada `**Status:** done` teria primeiro-token `done`, que
**é** marcador válido no vocabulário novo. Isto inverte a direção de falha: de "bloqueia
indevidamente" para **"libera indevidamente"**, e o gatilho é justamente o tipo de prosa que este
próprio roadmap, a REQ e o ADR usam repetidamente para *documentar* o bug (citam
`**Status:** pending` e `**Status:** done` como literais, em blocos de código, várias vezes). Um ML
real cujo "Actions" inclua um trecho de exemplo assim — inclusive copiado desta própria REQ como
referência — passaria a fechar `mls_complete` sem estar concluído. Confirmei o mesmo padrão do lado
do cabeçalho de aceite: uma cerca contendo `- [x] fake evidence, nothing built` sob um
`**Critérios de aceite:**` de exemplo é lida como o bloco de aceite real quando não há nenhum outro
depois, dando `acceptance_evidence: passed` sem nenhum critério genuíno (`forged3.md`, reproduzido).

**Vetor de PR de terceiro:** sim, qualquer um que edite o `.md` do roadmap — incluindo por PR —
pode escrever `**Status:** done` sem ter feito o trabalho; isso não é um bug de parsing, é o limite
de confiança inerente ao desenho inteiro (o `barrier` lê o que o arquivo declara, não verifica a
realidade). Isso já é verdade hoje com `✅` (é só mais fácil de digitar `done` que copiar/colar um
emoji, o que **baixa a barreira de erro humano ou automação descuidada**, mesmo sem má-fé) — ver
seção 4.

##### 3. Alvos de falsificação nas duas direções

| Mudança | Regride para trás (volta ao antigo) | Regride para o lado oposto (super-permissivo) |
|---|---|---|
| Cabeçalho bilíngue (`criteriaHeaderRe` aceita EN+PT) | Só PT: os 43/143 roadmaps EN e todo `roadmap new` recém-gerado voltam a falhar `acceptance_evidence` com `no acceptance block` — é o próprio bug desta REQ, reintroduzido | Regex vira `\*\*(Acceptance criteria\|Crit[eé]rios de aceite):\*\*` **sem `^`** ou aplicada ao documento inteiro: casa dentro de prosa/cerca (como reproduzido no `forged2.md`/`forged3.md` acima) — critérios forjados por citação passam a "evidência" |
| Status por primeiro token (`✅\|done\|Concluído`) | Volta a `contains("✅")`: `⬜ Pendente ✅` volta a passar (falso positivo **já em produção hoje**, reproduzido acima) — regressão para um bug já conhecido e catalogado no vault | Token comparado por `contains`/regex solto em vez de igualdade exata pós-normalização: `**Status:** não done` passaria (`não` contém `nao`? não — mas um regex mal escrito tipo `done` sem `\b`/comparação exata casaria `notdone`, `done-not-really` e `pendingdone`) |
| Vocabulário fechado `{✅, done, Concluído}` | N/A (não há vocabulário "antes" a regredir aqui) | Aceitar **qualquer** primeiro token não vazio como conclusão — vira no-op, é exatamente a alternativa que o ADR já rejeita explicitamente ("Fazer o barrier aceitar qualquer status não vazio") |
| Fence-awareness em `mlStatusMarker`/`acceptanceEvaluate` (achado nesta Wave 0, ainda **não implementado**, nem no ADR) | Se a Wave 1 não adicionar isso: qualquer ML cujo corpo cite `**Status:** done`/`**Critérios de aceite:**` com `[x]` dentro de uma cerca de código (documentação, exemplo, citação desta própria REQ) passa a liberar wave indevidamente — cenário concreto de gate para a Wave 3 | Se a implementação de fence-awareness for feita errado e ignorar cercas *legítimas* de status real (o que não deveria existir, mas por exemplo se alguém formatar `**Status:**` dentro de um bloco por engano), o efeito é falso-negativo (bloqueia trabalho real) — direção segura, mas vale caso de teste |

##### 4. Residual declarado

1. **Vocabulário fechado deixa fora `feito`, `ok`, `finalizado`** — decisão explícita do ADR
   (Alternatives Considered), aceito.
2. **Dupla forma de cabeçalho é superfície permanente**, nos 3 runtimes, para sempre — aceito pelo
   ADR (Consequences).
3. **`barrier` passa a conhecer dois idiomas** — dívida conceitual aceita pelo ADR.
4. **O gate da Wave 0 (`n -eq 9`) só cobre Go.** Node e Python têm implementações com contagens de
   regex diferentes (11 e 11, respectivamente) para a mesma cobertura semântica — não há hoje um
   gate automatizado que trave "os 3 runtimes reconhecem exatamente os mesmos 9 tokens de sintaxe";
   a garantia real vem do teste comportamental de paridade da Wave 1 (AC3), não de contagem
   estrutural. Aceito como o desenho atual, mas registrado para não ser confundido com paridade
   estrutural que não existe.
5. **`barrier` é um verificador sintático, não semântico.** Nenhuma versão deste desenho (substring
   ou primeiro-token) verifica se o trabalho descrito foi de fato feito — ele confia no que o arquivo
   declara. Baixar a barreira de digitação (de um emoji para a palavra `done`) reduz o atrito para
   marcar algo concluído por engano ou automação descuidada; é um residual aceito pelo ADR
   implicitamente (não discutido lá), tornado explícito aqui.
6. **Sombreamento por bloco de código/prosa (`mlStatusMarker` e `acceptanceEvaluate` sem consciência
   de cerca) é um residual NOVO que a Wave 1, como especificada hoje no roadmap, não cobre** — ver
   tabela da seção 3. Recomendo à Wave 1 tratar isto como parte do "mecanismo muda de contains para
   primeiro-token" (mesma função, mesmo arquivo, sem exigir novo ML), e à Wave 3 incluir os cenários
   `forged.md`/`forged3.md` (reproduzidos aqui) na bateria de `assert_fails_with`. Não bloqueio a
   Wave 0 por isto porque é achado, não pré-requisito de enumeração — mas registro como ação
   obrigatória de escopo para quem executar o ML-1A.
7. **Falsos-negativos de usabilidade** (crase ao redor do marcador, caractere zero-width) —
   direção segura (bloqueiam em vez de liberar), não tratados, não é responsabilidade de segurança
   corrigir.

## Wave 1 — Parser do `barrier` nos 3 CLIs (ML único)
> Dependências: Wave 0 aprovada. **ML único e sequencial**: os 3 runtimes implementam a mesma regra
> de casamento, e três agentes em paralelo produziram divergência de comportamento na REQ anterior
> (ML-2C acrescentou uma linha; ML-3D deixou o Node mudo). Um agente, os 3 arquivos.

### ML-1A — Cabeçalho bilíngue e status por primeiro token
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`,
`pypi/trackfw/commands/barrier.py` e os testes correspondentes de cada runtime.
**Actions:**
1. `criteriaHeaderRe` (e equivalentes) passa a aceitar `**Acceptance criteria:**` **e**
   `**Critérios de aceite:**`. AC1, AC2, AC3.
2. A detecção de conclusão deixa de ser `contains(marker, "✅")` e passa a ser **primeiro token**:
   concluído quando o primeiro token do restante da linha é `✅`, `done` ou `Concluído` — insensível
   a caixa e a acento. AC8.
   > Os 3 CLIs hoje fazem substring: `barrier.go:554`, `barrier.js:134`, `barrier.py:207`.
   > Ampliar o vocabulário **sem** trocar o mecanismo faz `**Status:** não done` passar. Ver
   > `vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`.
3. Sufixos continuam válidos: `✅ Concluído · **Agente:** \`apolo-tf\`` e
   `✅ concluído (auditado 2026-08-02)` seguem sendo concluídos — são 48 ocorrências no corpus.
4. Paridade exata nos 3: mesmas formas aceitas, mesmas rejeitadas, mesma saída.
5. **[Adicionado pelo ML-0A/hades-tf — fence-awareness]** `mlStatusMarker` e `acceptanceEvaluate` (e
   equivalentes Node/Python) passam a ignorar linhas dentro de blocos de código cercado
   (` ``` `...` ``` `) ao procurar a linha `**Status:**`/`**Acceptance criteria:**`/
   `**Critérios de aceite:**` real do ML. Sem isso, um ML cujo corpo cite `**Status:** done` ou
   `**Critérios de aceite:**` com `- [x]` dentro de uma cerca (documentação, exemplo, citação de uma
   REQ como esta) é lido como o status/bloco de aceite real — reproduzido ao vivo em
   `docs/roadmaps/wip/ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md`
   §"Resultado do ML-0A", seção 2 (`forged.md`, `forged3.md`). Sob `contains("✅")` isso falha
   fechado (mecanismo atual); sob primeiro-token, sem fence-awareness, passa a **liberar wave
   indevidamente** — é regressão de segurança introduzida pela própria mudança desta Wave se não for
   tratada aqui.
**Critérios de aceite:**
- [x] AC1, AC2, AC3, AC8
- [x] **AC9 provado por teste**, com os 6 casos negativos nomeados na REQ
- [x] Caso de teste nomeado para o item 5: ML com `**Status:** done` dentro de cerca de código e
      `**Status:** pending` real fora da cerca → `mls_complete` reporta **não concluído** (usa o
      status real, ignora o cercado); ML com `**Critérios de aceite:**`/`- [x]` dentro de cerca e sem
      bloco real fora dela → `acceptance_evidence` reporta **sem bloco de aceite**, não `passed`
- [x] `go build ./...` → 0 · `go test ./...` → 0 · `npm test --prefix npm` → 0 ·
      `PYTHONPATH=pypi python3 -m pytest pypi/tests` → 0
- [x] `./bin/trackfw barrier` sobre este próprio roadmap continua `passed`


#### Resultado do ML-1A + ML-1B (apolo-tf, 2026-08-29)

**ML-1A** — cabeçalho bilíngue ancorado em `^`; status por primeiro token com vocabulário fechado
`{✅, done, concluido}` sob normalização NFD + strip de combining marks e variation selectors;
máscara de cerca aplicada nos 3 pontos (heading de ML, status, bloco de aceite). O agente achou e
corrigiu uma divergência de VS16 (`✅️` com U+FE0F) que Go aceitava e Node/Python rejeitavam.

**ML-1B — corretiva da minha auditoria.** Dois pontos que o relatório do ML-1A classificava como
residual e como pendência não corrigida, e que medidos bloqueavam:

1. **Evasão da própria proteção.** A máscara só conhecia três crases: `~~~` nunca era mascarado e
   cerca de 4+ crases tinha o interior desmascarado por aninhamento. Agora segue CommonMark —
   abertura com 3+ do mesmo caractere, fechamento com o mesmo caractere e comprimento ≥.
2. **Os 3 CLIs discordavam, e o Node era o permissivo.** Marcadores indentados: Go e Python
   bloqueavam, Node liberava. O check que autoriza o PR era mais fraco num runtime.

O agente ainda achou sozinho, durante o fix, que trocar `.trim()` por igualdade de linha inteira no
cabeçalho de gates do Node faria o Node **ignorar o bloco de gates em silêncio** (`gates: passed`
com zero comandos) quando houvesse prosa ou CRLF na linha. Corrigido para casamento por prefixo,
como Go e Python.

**Auditoria do arquiteto — 3 CLIs reais:**

| caso | Go | Node | Python |
|---|---|---|---|
| `**Status:** ⬜ Pendente ✅` | blocked | blocked | blocked |
| marcadores indentados | blocked | blocked | blocked |
| `### ML-9Z` em `~~~` | 0 fantasmas | 0 | 0 |
| `### ML-8Y` em cerca de 4 crases | 0 fantasmas | 0 | 0 |
| bloco de aceite vazio | blocked | blocked | blocked |
| roadmap gerado + preenchido | `mls_complete` ✓ e `acceptance_evidence` ✓ | | |

Corpus: 144 roadmaps / 788 MLs, **zero** mudanças de veredito pela máscara. A única reclassificação
do ciclo é o caso da AC14, previsto pelo ADR, num roadmap em `abandoned/`.

**Residual aceito:** esconder um `- [ ]` não atendido dentro de cerca faz `unmet == 0`. Vale desde o
ML-1A, para qualquer forma de cerca. **Não amplia poder de ataque**: quem escreve o roadmap pode
simplesmente marcar `- [x]`. É o limite de confiança que o próprio ML-0A declarou na seção 2 — o
`barrier` é verificador **sintático**, não semântico. Bloco de aceite vazio, esse sim, é rejeitado
nos 3.

## Wave 2 — Template e legenda (ML único)
> Dependências: Wave 1 concluída. Toca os 3 geradores; ML único pela mesma razão da Wave 1.

### ML-2A — `roadmap new` escreve a forma canônica e ensina a legenda
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py` e testes.
**Actions:**
1. O template passa a escrever a forma canônica de status e a incluir a **legenda dos quatro
   estados** (⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado). AC11.
2. **Byte-identidade entre os 3 geradores** para o mesmo input.
3. `**Acceptance criteria:**` **permanece** — é a forma canônica pelo ADR. Não traduzir.
4. `**Gates da wave:**` **não muda**. Está no escopo negativo da REQ.
**Critérios de aceite:**
- [x] AC11
- [x] Template gerado byte-idêntico nos 3, provado por `diff`
- [x] Testes dos 3 runtimes verdes


#### Resultado do ML-2A (apolo-tf, 2026-08-29)

Legenda colocada **uma vez**, antes da primeira wave, dentro do bloco compartilhado `wave0Block` —
então vale nos dois caminhos (`new` e `--from-req`) sem duplicação e sem repetir por ML. Todo
`**Status:** pending` virou `**Status:** ⬜ Pendente`.

`**Acceptance criteria:**` e `**Gates da wave:**` **intocados**, provado por
`git diff | grep -E "^[+-].*(Acceptance criteria|Gates da wave)"` → zero linhas.

**Auditoria do arquiteto, com os 3 CLIs reais:**

```
template gerado           diff go/node · diff go/py     IDÊNTICO nos 3
ocorrências de "pending"  0
legenda                   ⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

AC12 — ciclo fechado, preenchendo SÓ pelo que a legenda ensina:
  go    mls_complete ✓   acceptance_evidence ✓
  node  mls_complete ✓   acceptance_evidence ✓
  py    mls_complete ✓   acceptance_evidence ✓
```

`check-artifact-parity.sh` → exit 0, cobrindo também o caminho `--from-req`.

**Testes acrescentados por revisão do próprio agente**, fechando uma lacuna que ele mesmo notou: a
legenda e a forma canônica não tinham cobertura unitária nenhuma. Cada teste assere a legenda
aparecendo **uma vez**, `**Status:** ⬜ Pendente` presente e `**Status:** pending` presente **zero**
vezes — que é a direção de falsificação.

## Wave 3 — Gate de ciclo fechado e contrato
> Dependências: Waves 1 e 2 concluídas.

### ML-3A — Gate falsificável do contrato gerador↔`barrier`
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-roadmap-barrier-contract.sh` (novo), `docs/cli-parity.md`,
`Makefile`.
**Actions:**
1. Gate que executa o **ciclo fechado** com CLI real, nos 3 runtimes: `roadmap new` em sandbox →
   preencher status e critérios **seguindo apenas o que o template diz** → `roadmap move wip` →
   `barrier --wave N` → exigir `passed`. **AC12.** Nada de chamada de função interna: foi assim que
   o ML-2G da REQ anterior escapou da auditoria.
2. **AC10 — não reclassificação:** rodar o parser novo sobre os 143 roadmaps de `docs/roadmaps/**` e
   comparar ML a ML com o veredito atual. Emitir a tabela do antes/depois. A única diferença
   permitida é ML que dizia `done`/`Concluído` e passa a ser reconhecido.
3. Falsificação nas duas direções, com `assert_fails_with` mirando a razão que o **próprio gate**
   emite: cabeçalho PT deixa de ser aceito → reprova; `**Status:** não done` passa a ser aceito →
   reprova; template deixa de trazer a legenda → reprova. **[Adicionado pelo ML-0A/hades-tf]** incluir
   os dois cenários de sombreamento por cerca de código reproduzidos no ML-0A: ML com
   `**Status:** done` dentro de bloco cercado e `**Status:** pending` real fora dele → deve continuar
   reprovado; ML com `**Critérios de aceite:**`/`- [x]` só dentro de cerca, sem bloco real → deve
   continuar reprovado com `no acceptance block`.
4. Guarda de vacuidade obrigatória; contagem de cenários no fim.
5. Seção em `docs/cli-parity.md` documentando o contrato gerador↔`barrier`, anotada com `gate=`.
6. Registrar no `Makefile`.
**Critérios de aceite:**
- [x] AC10, AC12, AC6 da REQ
- [x] `bash scripts/check-roadmap-barrier-contract.sh` → exit 0 com contagem
- [x] Guarda de vacuidade provada empiricamente
- [x] `bash scripts/check-parity-contract-coverage.sh` → exit 0
- [x] AC7: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0


#### Resultado do ML-3A (artemis-tf, 2026-08-29)

`scripts/check-roadmap-barrier-contract.sh`, **31 cenários**, exit 0. Determinismo provado rodando
3× com hash de corpus idêntico. Guarda de vacuidade com auto-teste embutido: roda o próprio guard
com `SCENARIOS_RUN=0` num subshell e assere exit 1 **antes** de confiar na execução real.

**AC10 — não reclassificação.** A tabela de vereditos dos 144 roadmaps foi congelada por hash, com o
conteúdo lido de `git show a4e8f35:<path>` e **não** da árvore viva — decisão dela, e é a certa: um
hash sobre a árvore viva quebraria a cada roadmap novo, virando gate que todo mundo aprende a
atualizar sem olhar.

**AC12 — ciclo fechado** com CLI real nos 3 runtimes, preenchendo só pelo que o template ensina.

Ela também corrigiu as regras 2-4 de "Roadmap parsing rules" do `cli-parity.md`, que estavam
desatualizadas desde a Wave 1 e que eu não tinha mandado revisar.

**Auditoria do arquiteto:** falsifiquei revertendo o template para `**Status:** pending` — o gate
reprova em `closed-cycle/go/mls-complete-passed` citando o veredito real. Restaurado, volta a exit 0.
`check-parity-contract-coverage.sh` verde. Árvore sem resíduo.

**Achado dela, fora do escopo, e mais grave que esta REQ:** `roadmapTrustForGates`
(`internal/commands/barrier.go:568`) **falha aberto**. Confirmei com o binário: `barrier` sem
`--trust-local-gates`, roadmap nunca commitado, gate `touch /tmp/... && echo EXECUTOU` →
`gates: passed` e o arquivo **criado**. Lendo o código, todo caminho de erro devolve
`trusted: true`, e o comentário admite: *"Any other failure (origin not configured, ref not fetched)
→ fail-open"*. Fecha só quando o `git show` falha com uma de duas substrings em inglês.

Consequência: sem remote `origin`, com `origin/main` não fetchado, com remote de outro nome, ou —
hipótese não confirmada — com git em outro idioma, o gate de um roadmap não confiável **executa**.
É exatamente o cenário para o qual o `ADR-2026-08-23` criou o controle. Pré-existente, não
introduzido por esta REQ. **REQ própria, prioridade acima do Windows.**

### ML-3C — Normalização de CRLF na leitura (corretiva, achada na auditoria)
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`,
`pypi/trackfw/commands/barrier.py` e os três testes.
**Critérios de aceite:**
- [x] Roadmap CRLF: os 3 dão o mesmo veredito, e é o correto
- [x] Não regride ML-1A (`⬜ Pendente ✅`) nem ML-1B (indentado), em CRLF
- [x] Cerca e `**Gates da wave:**` reconhecidos em CRLF
- [x] Suítes dos 3 verdes; `check-roadmap-barrier-contract.sh` e `check-gates-falsify.sh` exit 0

#### Resultado do ML-3C (apolo-tf, 2026-08-29)

Defeito que **eu achei na auditoria** e que nasceu no nosso próprio diff. Três primitivas, três
comportamentos: o `.` do JS **exclui** `\r` (é `LineTerminator` na spec do ECMAScript), então
`/^\*\*Status:\*\*(.*)$/` não casava a linha CRLF; o `.` do RE2 **inclui**, e o Go ainda passa por
`TrimSpace`; o Python lê em modo texto com universal newlines e o `\r` nunca chega ao parser.

Corrigido **na fronteira de entrada**, onde o arquivo vira linhas — não remendando regex por regex,
que seriam nove marcadores por runtime e o próximo a ser acrescentado nasceria com o bug.

**Auditoria do arquiteto, 3 CLIs reais:**

```
roadmap CRLF completo      go ✓ passed   node ✓ passed   py ✓ passed
roadmap CRLF ⬜ Pendente ✅ go blocked    node blocked    py blocked
```

**A honestidade dele vale registro.** Ele reportou que a normalização **só é load-bearing no Node** —
em Go e Python é no-op hoje, porque `TrimSpace` e universal-newlines já absorvem o `\r`, e nenhuma
fixture CRLF consegue distinguir "chamou a função" de "não chamou". Em vez de forjar a prova com
asserção de call-site ou mock, documentou a lacuna no doc-comment do teste. Está certo, e é
exatamente a lição do ML-2G do ciclo anterior: teste que prova que uma função foi chamada não prova
que ela faz diferença.

Ele também **recusou** tornar load-bearing no Python via `open(..., newline="")`, porque regrediria
o tratamento de CR solto que o Python ganha de graça — mudança sem defeito que a force. Concordo e
mantive.

Registrado em `vault/notes/barrier-crlf-divergencia-node-regex-2026-08-29.md` e no `cli-parity.md`.

### ML-3D — Bypass de fechamento de cerca (corretiva de segurança, REPROVAÇÃO da barreira)
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Critérios de aceite:**
- [x] Bypass bloqueado nos 3, citando status e critério reais
- [x] Info string continua abrindo; fechamento com espaços continua fechando
- [x] Default de `fenced` no Node falha fechado
- [x] Gate 31 → 39 cenários

### ML-3E — Marca combinante rejeitada no primeiro token (decisão 9 do ADR)
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
**Critérios de aceite:**
- [x] AC15 nas duas direções, nos 3
- [x] `Concluído` acentuado continua concluído — o caso que quebra com a ordem errada
- [x] Gate 39 → 42 cenários; corpus sem reclassificação

#### Resultado dos ML-3D e ML-3E (apolo-tf, 2026-08-29)

**O `hades-tf` REPROVOU a barreira final com um bypass ao vivo nos 3 CLIs**, sobre a própria
proteção que esta REQ introduziu. Uma linha ` ```qualquer-coisa ` **fechava** a cerca antes da hora,
porque a implementação contava a corrida de caracteres e ignorava o que vinha à direita — correto
para **abertura** (o CommonMark permite info string), errado para **fechamento** (exige só os
caracteres da cerca e espaços). Resultado: o exemplo virava conteúdo real.

Reproduzido por mim antes e depois. Um ML com `**Status:** ⬜ Pendente` e `- [ ]` não atendido
liberava `mls_complete` **e** `acceptance_evidence` nos 3; agora bloqueia nos 3, citando o status e o
critério **reais**.

**A lição do gate:** os 31 cenários cobriam cerca de 3 crases, de 4, til e conteúdo forjado em cerca
"limpa" — **nenhum** usava linha de fechamento com sufixo. A suíte foi desenhada para essa classe de
ameaça e não a pegou. *Gate de falsificação prova o que alguém lembrou de falsificar.*

**Re-pin do corpus investigado, não aceito.** A correção reclassifica **1 de 144** roadmaps, e o
agente foi ver qual: um bloco de 3 crases aninhando um ` ```bash ` de mesmo comprimento sem escalar
para 4. Confirmou por diff binário dos dois parsers sobre os 144 arquivos que só aquelas 6 linhas
mudam.

**Decisão 9 do ADR (ML-3E).** Ele implementou o lado permissivo do achado Unicode e **escalou** a
direção, com o censo na mão: zero marcas `Mn` no primeiro token de status nos 144 roadmaps, VS16
incluído. Decidi **contra** o que ele tinha implementado — apertar, não alinhar por baixo. O motivo
não é custo, é engano: `d<U+1DC0>one` renderiza como algo que um revisor humano não lê como `done`.

**A ordem que ele escolheu é a parte fina do ML, e está certa:** remover VS16 → checar `Mn`
remanescente **no token bruto, antes do NFD** → só então dobrar diacríticos para comparar. Checar
depois do NFD rejeitaria `Concluído`, porque a decomposição **sintetiza** um `Mn` que o autor nunca
digitou. Ele viu isso sozinho.

**Auditoria do arquiteto — 4 casos × 3 CLIs:**

| status | go | node | py |
|---|---|---|---|
| `d<U+1DC0>one` (injetado) | rejeitado | rejeitado | rejeitado |
| `✅️` (VS16) | concluído | concluído | concluído |
| `Concluído` (acentuado) | concluído | concluído | concluído |
| `done` | concluído | concluído | concluído |

## Wave 4 — Fronteira de CI (reaberta em 2026-08-29)

> **Por que esta wave existe, e o erro é do arquiteto.** Eu movi este roadmap para `done/` e fechei a
> REQ **antes** de o CI estar verde. O `barrier --wave 3` e o `make quality` local passavam, e eu
> tratei isso como conclusão — mas o PR #217 subiu com o job `go` vermelho, e depois o `parity`.
> Dois MLs corretivos nasceram **depois** do fechamento. A `artemis-tf` recusou o handoff do ML-3G
> apontando exatamente isso: roadmap em `done/`, `wip/` vazio, e um ML que não existia em roadmap
> nenhum. Recusa correta — eu estava pedindo trabalho fora da cadeia de governança que este projeto
> existe para impor.
>
> **Lição:** verde local não é conclusão. A conclusão é o CI verde, porque é o CI que roda no
> ambiente mais pobre — e foi na diferença entre os dois ambientes que os dois defeitos moraram.

### ML-3F — Paridade cross-runtime sai do `go test` e vai para o gate
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Files affected:** `scripts/check-roadmap-barrier-contract.sh`, `internal/commands/barrier_test.go`
**Critérios de aceite:**
- [x] Gate 42 → 52 cenários, incluindo os 5 de CRLF e o de cabeçalho de gates com prosa
- [x] `go test ./internal/commands/...` com PATH sem `node` e sem `python3` → exit 0
- [x] Nenhum arquivo de produto tocado

#### Resultado do ML-3F (artemis-tf, 2026-08-29)

O job `go` do CI reprovou com 9 testes: os `TestBarrierParity_*` faziam shell-out para `node` e
`python3` de dentro do `go test`, e o job `go` é Go puro. Passava na minha máquina porque ela tem os
três runtimes. O job `parity`, onde os três existem, ficou `skipping` em cascata — o gate que de
fato cobre paridade nem rodou.

**Duas correções dela sobre mim, ambas materiais:**

1. Afirmei no despacho que os cenários `fence-phantom` do gate já cobriam cross-runtime. **Não
   cobriam** — chamavam só `run_cli go`. As únicas coberturas cross-runtime daquele achado eram
   dois dos nove testes que eu mandei remover. Ela verificou em vez de confiar, e acrescentou os
   cenários **antes** de remover.
2. O comando que sugeri para reproduzir o CI (`env PATH=/usr/bin:/bin`) **não** reproduz no macOS:
   `/usr/bin/python3` existe como stub das Xcode CLT. Ela montou um PATH com só `git`, `/bin` e o
   diretório do `go`, conferindo `command -v` vazio antes de rodar.

### ML-3G — Congelamento do corpus sem depender de história do git
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-roadmap-barrier-contract.sh` e um arquivo de snapshot versionado.
**Actions:**
1. O congelamento do AC10 lê o corpus via `git show a4e8f35:<path>` e o job `parity` reprova com
   `fatal: Not a valid object name a4e8f35`. Dois motivos independentes:
   **(a)** `actions/checkout@v7` usa `fetch-depth: 1` — nenhum SHA histórico é alcançável no CI;
   **(b)** `a4e8f35` é commit **desta branch**, confirmado não-ancestral de `origin/main`, e o
   projeto faz **squash-merge** — o SHA vira órfão no merge e o gate quebraria na `main`
   **permanentemente**. Por isso `fetch-depth: 0` não é a correção: adia a falha por horas.
2. Trocar por **snapshot versionado chaveado por `basename`**, lido da árvore de trabalho. Roadmap
   novo é ignorado (preserva a imunidade ao crescimento do corpus, que era a intenção original e
   está certa); basename do snapshot ausente do disco **reprova**.
3. **Por que basename e não caminho:** roadmap muda de pasta o tempo todo (`backlog` → `wip` →
   `done`); snapshot por caminho reprovaria a cada transição, que é operação diária.
4. Política de colisão de basename **explícita**. Medido pela agente: hoje não há colisão
   (`uniq -d` vazio), mas nada impede — declare o comportamento em vez de deixá-lo indefinido.
**Critérios de aceite:**
- [x] Gate exit 0, contagem ≥ 52
- [x] Nenhuma leitura de história do git no gate — `grep` prova
- [x] **Reprodução em clone raso**: `git clone --depth 1` e rodar o gate lá dentro
- [x] Falsificação: veredito alterado no snapshot → reprova nomeando qual; roadmap novo → **não**
      reprova; roadmap do snapshot removido → reprova
- [x] Guarda de vacuidade provada; `check-gates-falsify.sh` → 0
- [x] Nenhum arquivo de produto tocado

#### Resultado do ML-3G (artemis-tf, 2026-08-29)

Snapshot versionado em `scripts/testdata/` — 144 arquivos, chaveados por `basename`, extraídos
**uma vez** na autoria (fora do gate) e agora commitados como bytes. O gate faz **zero** chamadas a
`git show`/`cat-file`/`ls-tree`/`archive`; as 4 menções ao SHA que restam são comentário explicando
o histórico.

Ela foi além do meu desenho num ponto que melhora: o conteúdo para calcular veredito vem do
**snapshot**, não do disco. O disco é consultado só para **existência**. Isso preserva a semântica
de congelamento do AC10 de verdade — o gate testa o **parser** contra entrada fixa, e não fica
refém de alguém editar um roadmap em `done/`.

Acrescentou também um `.tsv` com a tabela de vereditos, hash-idêntico ao pin, para que uma
reclassificação **nomeie a linha divergente** via `diff` em vez de dizer só "o hash mudou".

**Prova de fidelidade:** rodar o gate migrado contra o snapshot reproduziu o `PINNED_CORPUS_HASH` e
as seis contagens **sem alteração** — nenhum pin foi re-pinado. Se o snapshot estivesse errado, ao
menos um teria se movido.

**Três falsificações, em cópias descartáveis:** veredito alterado → reprova nomeando a linha;
roadmap novo → **não** reprova; roadmap do snapshot removido do disco → reprova nomeando o basename.
Mais a guarda de vacuidade, onde ela achou e corrigiu um FAIL duplicado no próprio rascunho.

**Custo aceito:** 2,4 MB de corpus duplicado no repositório. É o preço de um pin de regressão de
parser que não depende de história do git — e história do git é justamente o que o clone raso do CI
não tem.

**Gates da wave:**
```bash
set -eu
grep -q "git show" scripts/check-roadmap-barrier-contract.sh && { echo "gate ainda lê história do git" >&2; exit 1; }
echo "Wave 4 gate OK — congelamento sem dependência de história."
```

## Barreira final
Revisão `hefesto-tf` (qualidade) e `hades-tf` (segurança — o `barrier` é um check que **libera
wave**: falso positivo aqui é trabalho incompleto dado como pronto). Auditoria de diff pelo
arquiteto e `trackfw barrier --wave 3`.

#### Parecer de qualidade da barreira final (hefesto-tf, 2026-08-29)

**Método:** leitura integral do `git diff origin/main...HEAD` dos 3 runtimes
(`internal/commands/barrier.go`, `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`
e geradores), da REQ (AC1–AC14), deste roadmap, e leitura direcionada dos testes (não só grep de
nomes): corpo de `TestBarrierParity_CRLFLineEndings_*` e do helper `assertParity`/
`runAllThreeRuntimes` (confirma shell-out real a `node`/`python3`, não simulação em Go), uso dos
símbolos exportados (`statusIsComplete`, `computeFenceMask`, `splitRoadmapLines`,
`CRITERIA_HEADER_RE`) nas suítes Node/Python, e a lógica de `scripts/check-roadmap-barrier-contract.sh`
(FREEZE_REF, hash pinado, guarda de vacuidade).

**Veredito: APROVA.**

---

**1. CRLF inerte em Go/Python (pergunta 1) — concordo em manter, não é peso morto.**

O doc-comment em cada `splitRoadmapLines`/`_split_roadmap_lines` já diz, explicitamente, que a
normalização é no-op nesses dois runtimes hoje, nomeia o mecanismo acidental que a torna no-op
(`TrimSpace`/universal-newlines) e nomeia o tipo exato de mudança futura que reintroduziria o
defeito (um marcador novo comparado por igualdade exata sem passar por `TrimSpace`, ou um `.` sob
modo mais estrito). Isso não é código que "mente sobre o que protege" — é o oposto: o comentário
se recusa a reivindicar proteção que não tem hoje. O teste unitário
(`TestSplitRoadmapLines_StripsTrailingCROnlyAtBoundary` em Go, equivalentes em JS/Python) documenta
a mesma lacuna em vez de escondê-la com um mock de call-site — é precisamente a lição do ML-2G
aplicada corretamente. A alternativa (normalizar por runtime, no ponto onde cada um "precisar") é
pior: o próximo marcador nasceria sem saber qual runtime absorve `\r` por acidente. Concordo com o
apolo-tf: fronteira única, mesmo sendo hoje inerte em dois terços dos runtimes, é a decisão certa.
Dívida aceita, não bloqueia.

**2. Duplicação de `statusIsComplete`/`_status_is_complete` (pergunta 2) — inevitável no mecanismo,
mas a mitigação real não é extrair código, é compartilhar a lista de vetores.**

Não há build step nem runtime compartilhado entre os 3 CLIs (regra dura de paridade do projeto);
extrair ~15 linhas atrás de um gerador de código custaria mais do que economiza. O padrão de erro
observado nesta própria REQ — VS16 (ML-1A) e o `.trim()` do Node (ML-1B) — não nasceu de "três
implementações da regra", nasceu de **três listas de vetores de teste escritas à mão de forma
independente**. Verifiquei: os vetores aceitos/rejeitados de `statusIsComplete` aparecem
hard-coded, separadamente, em `internal/commands/barrier_test.go`, `npm/tests/barrier.test.js` e
`pypi/tests/test_barrier.py` — não há um fixture de dados único que as 3 suítes leiam. Recomendo,
como dívida a registrar (não bloqueia esta REQ): um arquivo de vetores canônico (ex.:
`docs/cli-parity.md` já pinado, ou um `.json`/`.txt` versionado) que as 3 suítes carreguem, para que
adicionar um vetor de teste o adicione nos 3 runtimes por construção. É a mitigação que a paridade
comportamental (AC3) está tentando alcançar por convenção hoje.

**3. Testes que passam sem provar nada (pergunta 3) — não encontrei a forma ML-2G nesta REQ.**

Verifiquei especificamente os candidatos mais prováveis:

- `TestBarrierParity_CRLFLineEndings_*` (Go) chamam `assertParity` → `runAllThreeRuntimes`, que
  invoca `exec.Command("node", ...)` e `exec.Command("python3", ...)` de verdade
  (`internal/commands/barrier_test.go:1133,1159`) — não é simulação Go-only com nome enganoso.
- As suítes Node e Python usam os símbolos internos exportados (`statusIsComplete`,
  `computeFenceMask`, `_status_is_complete`, `_fence_mask` etc.) para testes unitários de vetor,
  **mas também** têm testes e2e reais via CLI (`pypi/tests/test_barrier.py:679` em diante,
  `test_barrier_cli_*_e2e`, subprocess real) cobrindo os mesmos casos (cabeçalho bilíngue, cerca
  forjada, marcador indentado, CRLF). Unitário + e2e, não unitário no lugar de e2e.
- O único gap reportado explicitamente é o do item 1 acima (CRLF inerte em Go/Python), e ele é
  **declarado** no doc-comment do teste, não maquiado como cobertura.

**Achado que reporto, não bloqueia:** o `AC10` (não reclassificação) pede comparar o parser novo
contra "o veredito do parser atual" — ou seja, uma comparação diferencial parser-velho vs.
parser-novo. O gate (`scripts/check-roadmap-barrier-contract.sh`, `FREEZE_REF=a4e8f35`) roda o
binário **atual** (já com o parser novo — `a4e8f35` é o commit do ML-2A, posterior ao ML-1A/1B)
contra o conteúdo do corpus congelado, e pina o hash resultante. Isso prova **determinismo daqui pra
frente** (nenhuma reclassificação futura passa despercebida) — mas não é, em si, a prova
diferencial "parser velho vs. parser novo" que o AC10 descreve; essa comparação foi feita ao vivo
pelo hades-tf/apolo-tf e está registrada como afirmação em prosa neste roadmap ("144 roadmaps / 788
MLs, zero mudanças de veredito"), não como artefato re-executável no repositório. É reproduzível
manualmente (checkout de um commit anterior ao ML-1A, rebuild, rodar contra o mesmo `FREEZE_REF`,
diff), mas não está automatizado. Não é uma fixture que "coincide por acaso" — é uma afirmação
correta, mas cujo re-teste depende de reconstrução manual em vez de comando único. Recomendo
registrar como dívida: um script (ou a mesma tabela) que compare explicitamente contra o binário
pré-ML-1A, preservado como artefato, não só como prosa de roadmap.

**Achado adicional, menor, fora das 4 perguntas — Node por omissão de default fica permissivo.**
`mlCompletionStatus(mlLines, fenced = [])` e `mlAcceptanceEvidence(mlLines, fenced = [])`
(`npm/src/commands/barrier.js:272,294`) usam default `[]` — chamado sem o segundo argumento, TODA
linha é tratada como fora de cerca, ou seja, a proteção de AC13 desaparece silenciosamente. O
call-site de produção (`evalMlsComplete`/`evalAcceptanceEvidence`, linhas 369/383) sempre passa
`ml.fenced`, então **não há exploração hoje**. Mas as duas funções são exportadas
(`module.exports.mlCompletionStatus`/`mlAcceptanceEvidence`, usadas diretamente pelos testes) e o
default silenciosamente permissivo é exatamente a forma do ML-1B achado 2 (Node era o runtime
permissivo por omissão) — já mordeu esta REQ uma vez. Não bloqueia porque não há call-site real
afetado, mas registro para correção de baixo custo: tornar `fenced` obrigatório (sem default), ou
default para `computeFenceMask(mlLines)` em vez de `[]`.

**4. Legibilidade (pergunta 4) — boa, com uma ressalva sobre onde a garantia mora.**

Os três mecanismos (máscara de cerca, normalização Unicode do primeiro token, leitura por primeiro
token) têm doc-comments que carregam o *porquê*, não só o *o quê* — cada um cita o ADR/AC específico
que o motiva e, nos pontos mais sutis (VS16 em `normalizeStatusToken`, CommonMark em
`detectFenceMarker`/`fenceMask`), explica o efeito colateral que a implementação ingênua teria
("Go's `runes.In(unicode.Mn)` folds VS16 too... without stripping it here both barrier.go and
barrier.py would accept the VS16 form while barrier.js rejected it"). Isso é o padrão certo: o
comentário não apenas descreve o código, ele preserva o raciocínio que motivou a escolha, então um
mantenedor que tentar "simplificar" a máscara de cerca ou o first-token tem, na própria função, o
motivo escrito de por que a versão simples já foi tentada e rejeitada. `docs/cli-parity.md` §2-bis
e §3/§4 espelham a mesma explicação em nível de contrato cross-runtime, não só em comentário de
código — reduz o risco de a garantia existir só na cabeça de quem escreveu.

---

**Resumo — bloqueia o PR:** nenhum item.

**Dívida aceita a registrar** (nenhuma bloqueia; ordem de valor):
1. AC10 — a comparação diferencial parser-velho vs. parser-novo existe como prosa de roadmap, não
   como artefato re-executável; recomendo script/tabela dedicados numa REQ futura de manutenção do
   gate.
2. Vetores de teste de `statusIsComplete`/`_status_is_complete` duplicados manualmente nos 3
   runtimes, sem fonte de dados única — é como VS16 e `.trim()` nasceram nesta própria REQ.
3. `mlCompletionStatus`/`mlAcceptanceEvidence` (Node) com default `fenced = []` silenciosamente
   permissivo; sem exploração hoje (call-site de produção sempre passa a máscara), mas é a mesma
   forma do ML-1B achado 2.
4. CRLF inerte em Go/Python — aceito como fronteira única e bem documentada, não como dívida a
   remover.

**Nota de processo:** por instrução explícita desta tarefa ("não toque em nenhum outro arquivo" —
`hades-tf` revisa em paralelo no mesmo roadmap), não escrevi a entrada correspondente em
`docs/agents-working-context.md`; fica para quem consolidar os dois pareceres.

#### Parecer de segurança da barreira final (hades-tf, 2026-08-29)

**Método:** leitura do `git diff origin/main...HEAD` completo dos 3 runtimes, confronto linha a
linha com o modelo de ameaça que eu mesmo escrevi no ML-0A, e reprodução ao vivo — binário Go
compilado deste branch (`go build ./cmd/trackfw`), `npm/bin/trackfw` e
`PYTHONPATH=pypi python3 -m pypi.trackfw.cli` — contra roadmaps de sonda num projeto descartável
fora deste repositório, seguindo `feedback_verify_by_execution` do meu memory: nenhum veredito
abaixo é dedução de leitura de código sem confirmação por execução real.

**Veredito: REPROVA.**

Achado #1 é um bypass completo de liberação de wave, ao vivo, nos **3 CLIs simultaneamente**,
sobre exatamente a proteção que esta REQ (AC13, ADR decisão 7) foi desenhada para fechar. Isto não
é hipótese nem leitura de código — é `mls_complete: passed` e `acceptance_evidence: passed`
reproduzidos para uma ML cujo conteúdo real é `**Status:** pending` e
`- [ ] critério real não atendido`.

---

**Achado #1 (Crítico, bloqueia) — fechamento prematuro de cerca por conteúdo à direita do
delimitador; bypass total de `mls_complete` + `acceptance_evidence` nos 3 CLIs.**

`detectFenceMarker` (Go `internal/commands/barrier.go:275-291`), `detectFenceMarker` (Node
`npm/src/commands/barrier.js:187-195`) e `_detect_fence_marker` (Python
`pypi/trackfw/commands/barrier.py:167-186`) contam apenas a corrida de caracteres idênticos no
início da linha (` ``` `/`~~~`) e ignoram **qualquer conteúdo depois dela**. Isso é correto para a
linha de **abertura** de uma cerca (CommonMark permite info string, ex.: ` ```bash `), mas está
errado para a linha de **fechamento**: o CommonMark exige que uma linha de fechamento contenha
**apenas** os caracteres da cerca, seguidos no máximo de espaço em branco — uma linha como
` ```qualquer-coisa ` encontrada **dentro** de uma cerca já aberta **não fecha** a cerca no
CommonMark real; ela continua sendo conteúdo interno do bloco.

`fenceMask`/`computeFenceMask`/`_fence_mask` tratam essa linha como fechamento válido mesmo assim
(`isFence && ch == fenceChar && length >= fenceLen`, sem checar o que vem depois), porque
reusam a mesma função de detecção para os dois papéis. O efeito: uma cerca aberta para ilustrar o
próprio defeito (documentação, exemplo, citação — exatamente o padrão que este roadmap, a REQ e o
ADR usam repetidamente, e que o próprio ML-0A já tinha sinalizado como o gatilho mais provável)
**fecha sozinha e prematuramente** assim que qualquer linha interna começar com 3+ do caractere da
cerca seguida de texto — e todo o conteúdo real do exemplo que viria depois (incluindo um
`**Status:** done` forjado e um `- [x]` forjado colocados deliberadamente pelo autor do exemplo,
ou por um PR hostil) passa a ser lido como **conteúdo real da ML**, não como interior de cerca.

**Reprodução ao vivo, os 3 CLIs, mesmo arquivo** (`sonda-fence-full-bypass.md`, roadmap de sonda
fora deste repositório):

```
### ML-1A — ML nao concluida, mas libera a wave
Prosa introduzindo um exemplo do defeito que documentamos:
```
notas de exemplo, sem relacao com o trabalho real
```trailing-junk-que-nao-fecha-a-cerca-no-commonmark-real
**Status:** done
**Acceptance criteria:**
- [x] evidencia forjada, nada foi feito
Mais texto ainda dentro do exemplo, por CommonMark de verdade:
```
Conteúdo real da ML, fora do exemplo:
**Status:** pending
**Acceptance criteria:**
- [ ] critério real não atendido
```

```
$ ./hades-barrier barrier sonda-fence-full-bypass --wave 1        # Go, binário deste branch
✓ mls_complete: passed
✓ acceptance_evidence: passed

$ node npm/bin/trackfw barrier sonda-fence-full-bypass --wave 1   # Node
[passed] mls_complete
[passed] acceptance_evidence

$ PYTHONPATH=pypi python3 -m pypi.trackfw.cli barrier sonda-fence-full-bypass --wave 1   # Python
Status: passed
  ✓ mls_complete: passed
  ✓ acceptance_evidence: passed
```

O `barrier` Python, isolado, dá **`Status: passed`** para essa wave inteira — o único motivo de Go
e Node mostrarem `blocked` no meu sandbox é o check `validate` acusando artefatos de governança
ausentes no projeto de sonda (REQ/ADR/git — irrelevante ao roadmap em si); `mls_complete` e
`acceptance_evidence`, que são os dois checks que esta REQ existe para corrigir, **passam nos 3**.

**Por que isto é exatamente o cenário que a Wave 0 e o ADR decisão 7 previram, e a Wave 1/3 não
fecharam:** o ML-0A já tinha identificado "sombreamento por bloco de código" como o vetor mais
grave e recomendado tratamento explícito; o ML-1A/1B implementou `fenceMask` e o ML-1B até corrigiu
a regra de abertura/fechamento para seguir CommonMark **na contagem de caracteres** (decisão
correta), mas não replicou a segunda metade da regra CommonMark — que a linha de fechamento não
pode ter conteúdo à direita do delimitador. Os 31 cenários do ML-3A
(`scripts/check-roadmap-barrier-contract.sh`) cobrem cerca de 3, til, 4+ crases aninhadas, status
forjado dentro de cerca "limpa" e critérios forjados dentro de cerca "limpa" — mas nenhum cenário
usa uma linha de fechamento com sufixo, então a suíte falsificável não pega este vetor apesar de
ter sido desenhada especificamente para a classe de ameaça que ele explora.

**Vetor de PR de terceiro:** sim, plenamente — qualquer PR que edite o `.md` do roadmap pode conter
esta construção dentro de uma seção "Actions" ou de documentação de exemplo (uso legítimo aparente:
ilustrar um trecho de código com um trecho de outro código dentro, prática comum em Markdown) e
liberar a wave sem que o ML tenha sido feito, em qualquer um dos 3 CLIs — inclusive o que roda em
CI/`make quality`/`trackfw serve`.

**Correção mínima recomendada:** distinguir as duas regras de fechamento — abertura aceita
conteúdo à direita (info string), fechamento não. Concretamente, em `fenceMask`/`computeFenceMask`/
`_fence_mask`, ao avaliar uma linha **enquanto já dentro de uma cerca** (`fenced == true`), exigir
que o restante da linha após a corrida de caracteres da cerca seja vazio ou só espaço em branco
antes de aceitá-la como fechamento; caso contrário a linha permanece como conteúdo interno (mascarada).
A regra de abertura (`fenced == false`) não muda. Não requer mudança de assinatura das funções.

---

**Achado #2 (Alto, deveria bloquear em conjunto com o #1) — divergência de normalização Unicode
entre os 3 CLIs quebra AC3/AC4 explicitamente, dois dos três liberam o que o terceiro bloqueia.**

`diacriticsFolder`/`normalizeStatusToken` em Go (`internal/commands/barrier.go:220-239`) usa
`golang.org/x/text/unicode/norm` + `runes.Remove(runes.In(unicode.Mn))` — remove **toda** marca
combinante da categoria Unicode Mn, em qualquer bloco (Combining Diacritical Marks, seu Supplement,
Extended, marcas para símbolos, pontos hebraicos, marcas árabes, devanágari etc.). `_normalize_status_token`
em Python (`pypi/trackfw/commands/barrier.py:126-140`) usa `unicodedata.combining(ch)` — remove
qualquer caractere com classe de combinação canônica não nula, cobertura equivalente na prática.
`normalizeStatusToken` em Node (`npm/src/commands/barrier.js:129`) usa a regex literal
`[̀-ͯ︀-️]` — **só** o bloco "Combining Diacritical Marks" mais os seletores de
variação; qualquer marca combinante Mn fora dessa faixa estreita **não é removida** pelo Node,
mas **é** removida por Go e Python.

O comentário do próprio código em Go/JS/Python já registra a motivação de cobrir VS16 (achado do
ML-1A) — mas a lista manual do Node ficou mais estreita que a categoria Unicode real que Go e
Python usam, reabrindo exatamente a classe de bug que o VS16 já tinha exposto: uma forma que dois
dos três runtimes aceitam e o terceiro rejeita, violando o próprio AC3 ("mesmo conjunto de formas
aceitas... falha se um dos três aceitar uma forma que os outros não") e o próprio texto do ADR.

**Reprodução ao vivo**, com `**Status:** d<COMBINING DOTTED GRAVE ACCENT U+1DC0>one` (`Mn`,
classe de combinação canônica 230 — fora do intervalo `̀-ͯ` do Node):

```
$ ./hades-barrier barrier sonda-divergencia --wave 1                                    # Go
✓ mls_complete: passed

$ node npm/bin/trackfw barrier sonda-divergencia --wave 1                               # Node
[blocked] mls_complete
  ✗ ML-1A: not complete (status: d᷀one)

$ PYTHONPATH=pypi python3 -m pypi.trackfw.cli barrier sonda-divergencia --wave 1         # Python
✓ mls_complete: passed
```

Direção do defeito: **Go e Python liberam o que Node bloqueia** — na prática, qualquer agente ou
processo que rode o `barrier` de Go/Python (que são, pelos MLs deste próprio roadmap, os dois
runtimes onde a normalização por categoria Unicode Mn foi a decisão consciente) libera uma wave
que o Node reprovaria com o mesmo arquivo. `check-roadmap-barrier-contract.sh` não tem nenhum
cenário com marca combinante fora de `̀-ͯ`/VS16, então a suíte falsificável de 31
cenários não pega esta divergência — ela testa VS16 e diacríticos comuns (o achado real do ML-1A),
não a categoria Unicode completa que o comentário do próprio Node cita como motivação.

**Correção mínima recomendada:** trocar a lista manual do Node por uma verificação de categoria
Unicode real — `token.normalize('NFD').replace(/\p{Mn}/gu, '')` (property escape `\p{Mn}`, suportado
por toda versão de Node usada neste projeto) em vez do intervalo fixo — para que os 3 runtimes
removam exatamente a mesma classe de caractere, e não uma aproximação enumerada à mão que já
divergiu uma vez (VS16) e voltou a divergir aqui.

---

**Confirmação dos residuais do ML-0A (item 4 do meu mandato):**

- Os 7 residuais que declarei no ML-0A (vocabulário fechado deixa `feito`/`ok`/`finalizado` de
  fora; dupla forma de cabeçalho permanente; `barrier` bilíngue; gate `n==9` só cobre Go;
  `barrier` sintático não semântico; falsos-negativos de crase/zero-width; sombreamento por cerca
  como residual identificado) seguem válidos como descritos — **exceto o item 6**, que eu havia
  registrado como "residual novo que a Wave 1 não cobre" e recomendado à Wave 1 tratar. A Wave 1
  tratou a *forma* do residual (implementou `fenceMask`) mas não a regra completa de fechamento do
  CommonMark — então o residual não foi fechado, **piorou de "não implementado" para "implementado
  incompletamente, com falsa sensação de cobertura"**: os 31 cenários do ML-3A dão a impressão de
  que o sombreamento por cerca está testado exaustivamente, e o Achado #1 mostra que não está.
- `roadmapTrustForGates` (fail-open) não foi tocado por este diff — confirmado por
  `git diff origin/main...HEAD -- internal/commands/barrier.go` não incluir a função
  `roadmapTrustForGates` em nenhum hunk. Não piorou; segue como REQ própria, fora desta revisão.

---

**Se REPROVAR — microlote corretivo mínimo:**

1. **`internal/commands/barrier.go`, `npm/src/commands/barrier.js`,
   `pypi/trackfw/commands/barrier.py`** — em `fenceMask`/`computeFenceMask`/`_fence_mask`, ao
   avaliar o candidato a **fechamento** (ramo `fenced == true`), exigir que o texto após a corrida
   de caracteres da cerca seja vazio ou só espaço em branco; manter a regra de **abertura**
   (`fenced == false`) aceitando conteúdo à direita (info string), sem mudar de assinatura.
2. **`npm/src/commands/barrier.js:129`** — trocar `[̀-ͯ︀-️]` por
   `\p{Mn}` via `/\p{Mn}/gu` (mantendo a remoção separada de VS16 se `\p{Mn}` não cobrir, mas
   `\u{FE0F}` já é categoria Mn — verificar e simplificar) para igualar a cobertura de Go/Python.
3. **`scripts/check-roadmap-barrier-contract.sh`** — dois cenários novos, falsificáveis nas duas
   direções: (a) cerca com linha de fechamento sufixada (` ```texto `) dentro de um bloco de ML,
   com status/critério real fora da cerca divergindo do forjado dentro — deve reprovar com o
   veredito real, não o forjado; (b) marca combinante Mn fora de `̀-ͯ`/VS16 no primeiro
   token de status — os 3 CLIs devem concordar (aceitar ou rejeitar juntos).
4. Rebuild + suíte de cada runtime + `bash scripts/check-roadmap-barrier-contract.sh` +
   `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` — os 4 comandos do protocolo padrão de ML.
5. Registrar nota de vault (`vault/notes/barrier-fence-closing-trailing-content-bypass-2026-08-29.md`
   ou similar) — é exatamente o tipo de achado não óbvio que outro agente perderia >10min
   reconstruindo, pela minha própria regra de vault.
