---
title: "Auditoria de qualidade — gate de duas fases para artefatos de terceiro"
date: 2026-08-15
author: hefesto-tf
status: final
scope: "internal/thirdparty/, internal/commands/integrations_thirdparty.go, internal/validator/validator_thirdparty_provenance.go, internal/integrations/{manifest,plan,render}.go, npm/src/thirdparty/, npm/src/commands/thirdparty.js, npm/src/validator/index.js, pypi/trackfw/thirdparty/, pypi/trackfw/commands/thirdparty.py, pypi/trackfw/validator.py, scripts/check-thirdparty-parity.sh"
related_adr: "docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md"
---

# Auditoria de qualidade — ML-4B

Barreira de revisão final sobre os 15 commits de `feat/instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas`
(Waves 0–3, `make quality` verde no `HEAD` atual). Este documento é apenas achados; nenhuma correção foi aplicada aqui —
cada item aponta arquivo:linha e o ML a que deveria ser endereçado.

## Resumo por severidade

| Severidade | Qtde |
|---|---|
| 🔴 Bloqueante | 0 |
| 🟠 Alto | 3 |
| 🟡 Médio | 4 |
| 🔵 Baixo | 2 |
| ⚪ Observação | 3 |

Nenhum achado é bloqueante para merge desta branch — nada aqui contradiz uma decisão normativa já tomada
(D1–D11), e o código implementa fielmente o que os ADRs decidiram. Os achados **Alto** são gaps que o
próprio desenho aceito deixou abertos sem estarem plenamente declarados onde um operador ou um futuro
mantenedor os veria a tempo, e merecem um ML de acompanhamento antes que o recurso seja usado em produção
com `--scope global`.

## Os 3 achados mais importantes

1. **[ALTO-1] Instalação `--scope global` é completamente invisível para `thirdparty_artifact_has_provenance`, para sempre, sem qualquer aviso disso no momento em que o usuário escolhe `--scope global`.** O próprio ADR (D4) explica *por que* o default é `project` — mas o código permite `global` com só uma confirmação genérica, sem declarar essa consequência específica no runtime.
2. **[ALTO-2] Fence não fechada (ou closer mais curto que o opener) descarta o restante do documento inteiro até EOF do critério de marcadores — não apenas o bloco.** Isso é testado e portanto conhecido, mas não está listado entre as lacunas do D3 no ADR nem no `docs/cli-parity.md`; é um bypass mais barato que paráfrase (um único ` ``` ` sem fechamento antes do heading malicioso).
3. **[MÉDIO-1] `executeThirdPartyInstall` (Go) é uma função de ~285 linhas com validação de precondições, I/O de rede zero mas múltiplos I/Os de disco intercalados, dois loops aninhados e nove pontos de retorno de erro** — replicada em Node e Python na mesma forma monolítica. Funciona e é bem comentada, mas qualquer próxima mudança (ex.: um novo modo de aprovação) precisa navegar essa função inteira nos 3 CLIs.

---

## 🟠 Alto

### ALTO-1 — Escopo `global` é um ponto cego permanente da regra de proveniência

**Arquivos:**
- `internal/validator/validator_thirdparty_provenance.go:84-124` (só chama `integrations.LoadManifest(root)` com `root = os.Getwd()`, nunca `HomeDir`)
- `internal/commands/integrations_thirdparty.go:387-389` (mensagem de confirmação de `--scope global` não menciona a consequência)
- `internal/integrations/manager.go:520-534` (confirma que `Claim.Scope == "global"` grava no manifest de `HomeDir`, não no de `ProjectRoot`)

**O problema:** `third-party install --scope global` é permitido (D4 permite explicitamente, com confirmação
extra). Quando usado, o `Claim{Origin: "thirdparty"}` correspondente é gravado em
`~/.trackfw/integrations-manifest.json` (`manager.go:527-529`), **não** no manifest do projeto. A regra
`thirdparty_artifact_has_provenance` só carrega `integrations.LoadManifest(root)` com `root` derivado de
`os.Getwd()` (sempre o projeto) — o comentário em `validator_thirdparty_provenance.go:138-141` até declara
isso explicitamente ("a global-scope claim would live in the home manifest instead, which this rule
intentionally never reads"), mas essa é uma decisão de arquitetura tomada dentro de um comentário de
código, não algo que o ADR declara nas seções "Limite herdado" (D2) ou "Limite honesto" (D11) — que só
falam do limite de "instalação nunca commitada" e do limite de "sniffing de path", nunca do limite de
escopo.

**Impacto:** a defesa real do ADR-2026-08-12 (detecção git-ancorada, já que não há prevenção técnica) **não
existe** para instalação `--scope global`. Um artefato de terceiro instalado em `--scope global` nunca
aparece em `git status`/diff/PR (é a própria razão dada no ADR para D4 preferir `project` por default — "Em
`~/.trackfw/`, não aparecem em lugar nenhum") **e** nunca é pego por `trackfw validate` mesmo que o usuário
rode `validate` no projeto certo. É o único dos dois mecanismos de defesa descritos no ADR falhando
silenciosamente, e o usuário que escolhe `--scope global` não recebe nenhum aviso disso — só a mensagem
genérica em `integrations_thirdparty.go:388`: `"install to --scope global requires
--yes-i-trust-this-source as the additional explicit confirmation (D4)"`.

**Evidência de que é um gap não coberto por teste:** nenhuma ocorrência de `"global"` em
`internal/validator/validator_thirdparty_provenance_test.go` (confirmado por grep) nem em
`internal/commands/integrations_thirdparty_test.go` além de um teste que verifica que o default **não** é
global (`integrations_thirdparty_test.go:432`). Não há teste, em nenhum dos 3 CLIs, que instale com
`--scope global` e confirme (ou negue) que `validate` continua mudo depois.

**Correção sugerida (para um ML de segurança/arquitetura, não deste ML):**
- Curto prazo — decisão documental: acrescentar este limite explicitamente ao D2/D11 do ADR e ao
  `docs/cli-parity.md`, e imprimir um aviso runtime específico ("this install cannot be detected by
  `trackfw validate` because --scope global lives outside the repository") no momento da confirmação de
  `--scope global`, nos 3 CLIs.
- Médio prazo — decisão de produto: considerar se `--scope global` deveria sequer existir para
  `third-party install`, dado que o próprio ADR já argumenta contra ele.

### ALTO-2 — Fence sem fechamento (ou closer mais curto) descarta o resto do documento inteiro da checagem de marcadores, não documentado como limite do D3

**Arquivos:**
- `internal/thirdparty/markers.go:44-67` (`removeFencedBlocks`)
- Testes que provam o comportamento (não são bug de teste — comportamento real e intencional):
  `internal/thirdparty/markers_test.go:85-104` (`TestCheckMarkers_UnclosedFenceDropsRestOfDocument`,
  `TestCheckMarkers_CloserShorterThanOpenerDoesNotClose`)
- Réplicas idênticas: `npm/src/thirdparty/markers.js:58-80`, `pypi/trackfw/thirdparty/markers.py:53-78`

**O problema:** o scanner de linha implementa corretamente a regra do CommonMark (fechamento precisa do
mesmo caractere delimitador com pelo menos tantas repetições quanto o abridor). Mas quando uma fence **nunca
fecha** — ou fecha com um delimitador mais curto que o abridor —, `closer` nunca volta a `""`, e **todo o
restante do documento até EOF** é tratado como "dentro da fence" e descartado da checagem de heading, não
só o trecho realmente destinado a ser um bloco de código.

Isso significa que um único ` ``` ` sem fechamento no início do conteúdo (ou um abridor de 4 crases seguido
de um "fechamento" de 3) faz com que **qualquer heading malicioso escrito depois dele**, mesmo formatado
exatamente como um dos 6 marcadores literais, nunca seja avaliado. É um bypass mais barato e mais óbvio do
que os já listados no D3 do ADR ("paráfrase", "indireção", "fragmentação", "homoglifos") — não exige
reescrever o marcador, só abrir uma fence e não fechá-la.

**Por que isso é diferente de um bug de teste:** o comportamento é intencional e testado corretamente nos 3
CLIs (`TestCheckMarkers_UnclosedFenceDropsRestOfDocument` afirma explicitamente que **nenhum** marcador deve
ser encontrado nesse caso, incluindo o `# Git authority` que está no meio do conteúdo de teste). O código
está fazendo exatamente o que o teste — e a implementação — dizem que deveria fazer. O problema é que essa
consequência de segurança não está registrada como um limite conhecido do D3 em lugar nenhum (nem na seção
"Limitação do critério de markers (D3)" do `docs/cli-parity.md:3442-3468`, nem nas seções correspondentes
do ADR), enquanto limites de exploração muito mais sutis (homoglifo cirílico, NFKC) estão.

**Impacto:** como D3 já é declarado como "tripwire, não filtro contra adversário competente", este achado
não muda a postura geral do gate — mas é o caminho de evasão **mais barato** disponível hoje, e vale estar
no mesmo lugar onde os outros limites do D3 já estão documentados, para que quem lê o ADR não avalie mal o
piso de segurança real do checker.

**Correção sugerida:** acrescentar este caso à lista "NÃO cobre" do D3 (ADR e `docs/cli-parity.md`) como
achado desta auditoria, sem necessariamente mudar o comportamento (mudar o comportamento — por exemplo,
tratando `unclosed fence` como erro fatal do fetch — é uma decisão de produto/segurança, fora do escopo
deste ML).

### ALTO-3 — Casefold diverge entre os 3 CLIs no passo 4 do D3 (Go/Node usam lowercase simples, Python usa casefold Unicode completo)

**Arquivos:**
- `internal/thirdparty/markers.go:104` — `text = strings.ToLower(text)`
- `npm/src/thirdparty/markers.js:118` — `text = text.toLowerCase()`, com comentário explícito nas linhas
  96-99: "not a true Unicode casefold, deliberately, to stay byte-identical to the Go reference"
- `pypi/trackfw/thirdparty/markers.py:110` — `text = text.casefold()`, com docstring nas linhas 89-92: "not
  `str.lower()` — the ADR-mandated normalization step"

**O problema:** o ADR (D3) manda literalmente "casefold" como passo 4. Node seguiu deliberadamente o
comportamento *real* do Go (`strings.ToLower`, um mapeamento de caixa simples) para ficar byte-idêntico,
e o comentário registra isso com precisão. O port Python seguiu o **texto do ADR** ao pé da letra
(`str.casefold()`, um full case folding Unicode com expansões multi-caractere como `ß → "ss"`) sem checar o
que o Go de fato faz. O resultado são **2 comportamentos diferentes para o mesmo passo nominal em 3 CLIs**:
Go e Node concordam entre si, Python diverge de ambos.

**Por que não é bloqueante hoje:** os 6 marcadores literais (`git authority`, `mode lock`, `governance
prerequisite`, `reporting boundary`, `scope boundary`, `dispatch contract`) são ASCII puro, e nenhum contém
substrings onde as expansões de casefold completo (`ß→ss`, `ﬀ→ff`, `ﬁ→fi`, etc.) produziriam uma correspondência
diferente da que `ToLower` já produz — a maioria dessas ligaturas, aliás, já é decomposta pelo NFKC no passo
3, que roda **antes** do casefold. Não encontrei um vetor de conteúdo que hoje explora essa divergência para
passar em um CLI e ser pego em outro.

**Por que ainda vale reportar:** é uma inconsistência real, silenciosa e não coberta pelo gate de paridade
— nenhum dos casos do corpus da Parte A do `scripts/check-thirdparty-parity.sh` (fullwidth NFKC, homoglifo
cirílico) exercita caracteres com expansão de casefold específica. Se um sétimo marcador for adicionado no
futuro contendo, por exemplo, `ss` em sua forma canônica, o Python passaria a capturar conteúdo com `ß` que
Go/Node deixariam passar — uma regressão de paridade silenciosa em um gate de segurança, exatamente a classe
de bug que este projeto já documentou duas vezes noutros lugares (`v` prefix, mensagens de erro).

**Correção sugerida:** trocar `str.casefold()` por um equivalente ASCII-only (`str.lower()`) em
`pypi/trackfw/thirdparty/markers.py:110` para ficar byte-comportamentalmente idêntico a Go/Node, e corrigir
o comentário que hoje justifica a escolha citando o ADR — o ADR deveria, por sua vez, dizer "lowercase
simples (ASCII case mapping), não full Unicode casefold" para não induzir a mesma divergência de novo.

---

## 🟡 Médio

### MÉDIO-1 — `executeThirdPartyInstall` é um monólito de ~285 linhas replicado nos 3 CLIs

**Arquivos:** `internal/commands/integrations_thirdparty.go:228-512`, `npm/src/commands/thirdparty.js:132-329`,
`pypi/trackfw/commands/thirdparty.py` (função equivalente).

Uma única função concentra: validação de flags, leitura de quarentena, checagem TOCTOU, derivação de slug,
resolução de N destinos, precondição de `--apply-to` (dois loops aninhados com `BuildPlans`/`Inspect` por
combinação target×agente e 3 saídas de erro diferentes), impressão de confirmação AC1, confirmação
interativa/TTY, confirmação extra de `--scope global`, verificação de aprovação D8c por destino, verificação
de `marker_override` por destino, escrita do artefato via `Manager.Install`, um segundo loop que
recarrega+regrava a proveniência por destino (`LoadProvenance`+`UpsertProvenanceEntry`, cada um fazendo seu
próprio I/O — nenhum é batelado), e um terceiro loop (`--apply-to`) que grava referência e re-renderiza o
agente por combinação target×agente. É funcional e cuidadosamente comentada, mas nada aqui é testável em
isolamento — todo teste precisa montar o fluxo completo (fetch → aprovação externa → install) para exercitar
qualquer trecho interno, o que já se reflete no tamanho de `internal/commands/integrations_thirdparty_test.go`
(578 linhas para 11 testes).

**Impacto:** próxima mudança de comportamento (ex.: aprovação assinada criptograficamente, D8(e) do
`plugins install`) precisa ser pensada e implementada dentro dessa mesma função monolítica em 3 linguagens,
em vez de contra uma interface menor e testável isoladamente.

**Correção sugerida (ML de refino, não bloqueante):** extrair ao menos 3 sub-passos com assinatura própria
e teste unitário direto — `validateApplyToPreconditions`, `verifyApprovalForAllDestinations`, e
`recordInstalledSHA256` — mantendo `executeThirdPartyInstall` como orquestrador fino. Replicar a mesma
extração nos 3 CLIs para não regredir a disciplina de paridade já observada (ver "Observação-1" abaixo).

### MÉDIO-2 — `UpsertProvenanceEntry`/`upsertProvenanceEntry` fazem load-then-write não atômico entre múltiplas chamadas sequenciais no mesmo comando

**Arquivos:** `internal/thirdparty/provenance.go:140-147` (`UpsertProvenanceEntry`), chamado em loop por
`internal/commands/integrations_thirdparty.go:466-476`; réplicas em `npm/src/thirdparty/provenance.js` e
`pypi/trackfw/thirdparty/provenance.py`.

Cada chamada de `UpsertProvenanceEntry` faz `LoadProvenance` + `WriteProvenance` completos (não um patch
atômico de uma única entrada). Quando `--targets` tem mais de um destino, o loop em
`integrations_thirdparty.go:466-476` chama isso N vezes sequencialmente **dentro do mesmo processo** — sem
risco de corrida interna, mas se dois processos `third-party install` diferentes rodarem concorrentemente
(dois agentes instalando artefatos de terceiro distintos ao mesmo tempo no mesmo projeto, algo plausível em
uso orquestrado), o último a escrever vence e o outro perde silenciosamente sua entrada de proveniência —
sem erro, sem log. Dado que a "falha de escrita da proveniência é fatal" é uma garantia central de D6, uma
perda silenciosa por corrida é uma lacuna na mesma garantia que o código em outro lugar trata com tanto
cuidado (comentário em `provenance.go:110-119` explicitamente contrasta isso com o padrão best-effort do
log de transição).

**Correção sugerida:** não bloqueante para uso single-agent-at-a-time; documentar a suposição de execução
serializada, ou (melhor) adicionar um lock de arquivo (`flock`/equivalente) ao redor do
load-modify-write, coerente com o atomicWrite já usado para a escrita em si.

### MÉDIO-3 — Cobertura de falha assimétrica: status HTTP não-200 só é testado em Python, não em Go nem Node

**Arquivos:**
- `internal/thirdparty/fetch.go:72-74` (`if resp.StatusCode != http.StatusOK { ... }`) — **sem teste**
  correspondente em `internal/thirdparty/fetch_test.go` (11 testes cobrem HTTP downgrade, redirects,
  content-type, tamanho — nenhum cobre status 4xx/5xx).
- `npm/src/thirdparty/fetch.js:100-101` (`if (res.statusCode !== 200) throw ...`) — sem teste
  correspondente encontrado em `npm/tests/thirdparty.test.js`.
- `pypi/tests/test_thirdparty.py:595-597` — **tem** o teste (`HTTPError 404` → `ThirdPartyFetchError` com
  "404" na mensagem).

**Impacto:** é um caminho de falha de rede documentado no D7 (implícito: qualquer resposta não-200 deve
falhar), presente no código dos 3 CLIs, mas coberto por teste automatizado em apenas 1 dos 3 — exatamente o
tipo de lacuna que `scripts/check-thirdparty-parity.sh` não pega, porque ele compara comportamento
observável entre os CLIs quando os 3 são exercitados da mesma forma, não audita se cada CLI tem cobertura
própria de cada branch.

**Correção sugerida:** portar `pypi/tests/test_thirdparty.py:590-598`'s cenário 404 para Go (`httptest`
servidor devolvendo 404) e Node (mock de resposta 404), fixando a mensagem esperada (`"HTTP 404"` /
`"fetch failed: HTTP 404"`) como asserção, não só "erro não nulo".

### MÉDIO-4 — `ApplyThirdPartyReferences` não valida `end > start` ao localizar o marcador de fechamento

**Arquivo:** `internal/integrations/render.go:649-666`.

`start := strings.Index(text, thirdPartyRefStart)` e `end := strings.Index(text, thirdPartyRefEnd)` são
buscados independentemente. Se, por alguma razão externa ao fluxo normal (edição manual do arquivo
renderizado, conteúdo de terceiro cujo `slug` produza texto que acidentalmente contenha o marcador de
fechamento antes do de abertura), `end < start`, o slice `text[:start] + block + text[end+len(...):]`
produz uma reordenação silenciosa do conteúdo, não um erro. O caso "malformado (start sem end)" já é tratado
explicitamente (linhas 658-664, mirror do `injectOrUpdateRules`), mas "end antes de start" não é. Baixo risco
prático (os marcadores são strings HTML-comment fixas que o próprio código escreve), mas é o tipo de
assunção não verificada que este mesmo arquivo trata com cuidado em outros pontos (o comentário da linha 658
mostra que o autor já pensou em um caso de malformação e deliberadamente não tratou este outro).

**Correção sugerida:** adicionar `if end != -1 && end < start` ao mesmo ramo de "malformado", tratando como
o caso de append-fresh-block já existente.

---

## 🔵 Baixo

### BAIXO-1 — Nome do teste Go já registrado como débito no próprio ADR, ainda não corrigido

**Arquivo:** `internal/thirdparty/fetch_test.go:64` — `TestFetch_RefusesThirdRedirect`.

O ADR (D7-bis, linha 268-269) registra explicitamente: "⚠️ Débito menor registrado: o teste Go
`TestFetch_RefusesFourthRedirect` tem nome que descreve mal o próprio comportamento (recusa o 3º, não o 4º).
Renomear no ML-3A." O nome já **foi** corrigido para `TestFetch_RefusesThirdRedirect` neste código (não é
mais `RefusesFourthRedirect`) — o débito registrado no ADR já foi pago. Não é um achado novo, é uma
confirmação de que o débito descrito no ADR está fechado; incluído aqui só para não deixar a auditoria
silenciosa sobre um item que o próprio ADR pede para verificar.

### BAIXO-2 — `deriveSlug` trunca a extensão pela última ocorrência de `.` de forma ligeiramente diferente entre Go e Node/Python para nomes sem extensão com ponto no meio

**Arquivos:** `internal/commands/integrations_thirdparty.go:196-220` (usa `filepath.Ext`, que localiza o
último `.`), `npm/src/commands/thirdparty.js:72-91` (usa `base.lastIndexOf('.')`, com guarda `dot > 0`).

Ambos removem a extensão pelo último ponto — comportamento equivalente na prática (`filepath.Ext` também é
"a partir do último ponto"). Não encontrei um caso de entrada onde os dois produzem slugs diferentes; incluído
como observação de baixo risco porque a lógica é reimplementada char-a-char em 3 linguagens (não é uma
função compartilhada), e é exatamente o tipo de função pequena e "óbvia" onde divergências sutis de edge
case tendem a se esconder sem que `scripts/check-thirdparty-parity.sh` tenha um caso de teste dedicado a
nomes de arquivo com múltiplos pontos (ex.: `meu.skill.v2.md`). Não vale um ML dedicado; vale um caso a mais
no corpus de paridade na próxima vez que esse arquivo for tocado.

---

## ⚪ Observação

### OBS-1 — Duplicação está disciplinada, não é o problema aqui

Comparando `internal/commands/integrations_thirdparty.go`, `npm/src/commands/thirdparty.js` e
`pypi/trackfw/commands/thirdparty.py` linha a linha: mesma ordem de validação, mesmos nomes de função (com
convenção de casing por linguagem), comentários que citam explicitamente o arquivo Go de referência em cada
porte, e a mesma ordem de campos do `ProvenanceEntry` mantida deliberadamente (não via spread/dict
comprehension ingênuo) nos 3 lugares. A triplicação exigida pela regra de paridade do projeto está sendo
paga do jeito disciplinado — isso reduz o risco dos achados de Médio/Alto acima (a correção, quando vier,
tem um padrão claro a seguir nos 3 CLIs), mas não elimina a necessidade de portar cada correção.

### OBS-2 — Débitos já documentados em `docs/cli-parity.md`, confirmados ainda abertos, não repetidos como achados novos

Dois débitos que a Parte D do `scripts/check-thirdparty-parity.sh` já registra como conhecidos e fora de
escopo (`docs/cli-parity.md:3486-3505`) continuam exatamente como descritos, e não deveriam ser reabertos
como achados desta auditoria:

- **Mensagem D10.1 do caso `StateModified`** não é comparada entre os 3 CLIs pela Parte D (só o caso
  `StateNotInstalled` é). Confirmado ainda válido: `internal/commands/integrations_thirdparty_test.go`
  tem `TestThirdPartyInstall_ApplyToRejectsHandModifiedAgentBeforeAnyWrite` (linha 440) testando o caso
  Go isoladamente, mas não há verificação cruzada de que a mensagem seja byte-idêntica entre os 3 CLIs
  para esse ramo específico.
- **Divergência de wrapper de erro top-level Go/cobra vs Node/Python** (stack trace estruturalmente
  diferente) — pré-existente, de escopo do projeto inteiro, não específica de `third-party`.

### OBS-3 — `internal/integrations/manifest_origin_test.go` e a retrocompatibilidade de `Claim.Origin` estão bem cobertas

`Claim.Origin == ""` (manifests legados) é tratado como catálogo e nunca sinalizado — confirmado testado
nos 3 CLIs (D11). Não há achado aqui; registrado porque era um ponto explícito de risco de regressão
(retrocompatibilidade de manifests existentes) e a auditoria não encontrou problema.

---

## Metodologia

Leitura completa de: `internal/thirdparty/{fetch,markers,provenance,quarantine}.go` e seus testes;
`internal/commands/integrations_thirdparty.go` e seu teste; `internal/validator/validator_thirdparty_provenance.go`
e seu teste; `internal/integrations/{manager,plan,render}.go` (trechos D5/D9/D11); os portes Node
(`npm/src/thirdparty/markers.js`, `npm/src/thirdparty/fetch.js`, `npm/src/commands/thirdparty.js`,
`npm/src/thirdparty/references.js`) e Python (`pypi/trackfw/thirdparty/markers.py`,
`pypi/trackfw/thirdparty/provenance.py`, `pypi/trackfw/thirdparty/references.py`) equivalentes; o ADR completo
(D1–D11, D2-bis, D7-bis, D9, D10) e a seção `third-party` de `docs/cli-parity.md` (linhas 3311-3505). Grep
dirigido para confirmar presença/ausência de cobertura de teste em cada ramo de erro citado. Nenhum código
de produto foi executado além de leitura estática — não foi rodado `go test`/`npm test`/`pytest` nesta
sessão (o roadmap já registra `make quality` verde no `HEAD` atual, fora do escopo desta auditoria repetir).
