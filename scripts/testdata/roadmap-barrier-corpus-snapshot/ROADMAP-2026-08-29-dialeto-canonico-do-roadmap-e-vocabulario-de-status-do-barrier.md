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
**Status:** ⬜ Pendente
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
- [ ] AC10, AC12, AC6 da REQ
- [ ] `bash scripts/check-roadmap-barrier-contract.sh` → exit 0 com contagem
- [ ] Guarda de vacuidade provada empiricamente
- [ ] `bash scripts/check-parity-contract-coverage.sh` → exit 0
- [ ] AC7: `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0

## Barreira final
Revisão `hefesto-tf` (qualidade) e `hades-tf` (segurança — o `barrier` é um check que **libera
wave**: falso positivo aqui é trabalho incompleto dado como pronto). Auditoria de diff pelo
arquiteto e `trackfw barrier --wave 3`.
