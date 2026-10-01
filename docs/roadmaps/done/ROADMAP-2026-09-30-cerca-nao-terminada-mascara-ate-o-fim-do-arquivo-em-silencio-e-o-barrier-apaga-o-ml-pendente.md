---
status: done
date: 2026-09-30
req: "docs/req/REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md"
squad: ""
---

# Roadmap: cerca nao terminada mascara ate o fim do arquivo em silencio e o barrier apaga o ML pendente

> Created: 2026-09-30 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md -->
REQ: docs/req/REQ-2026-09-30-cerca-nao-terminada-mascara-ate-o-fim-do-arquivo-em-silencio-e-o-barrier-apaga-o-ml-pendente.md

## Acceptance Criteria
- [x] **AC1** — comportamento correto **por superfície**, nos 10 call sites (Wave 0)
- [x] **AC2/AC3** — CLI: exit 2 nomeando a linha · **bem-formado continua passando**
- [x] **AC4** — o braço D da sonda deixa de sair `passed`
- [x] **AC5** — os 2 arquivos do acervo corrigidos **no mesmo PR**
- [x] **AC6** — `serve` não quebra
- [x] **AC7** — `cli-parity.md` descreve o real, por superfície
- [x] **AC8** — `make quality` e CI verdes — local `EXIT=0` (347 OK / 0 FAIL); CI do PR #492: 20/20 `pass`, mergeado em `e7595c4e`

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — o comportamento correto por superfície
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — mapear os 10 call sites e decidir o que cada um faz
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Arquivos de leitura:** `internal/roadmapdoc/roadmapdoc.go` (`FenceMask`:311, `ParseGates`:~674) ·
`internal/commands/barrier.go:164` · `internal/generators/roadmap.go:617` ·
`internal/generators/roadmap_show_json.go:128` · `internal/serve/api_board.go:168` ·
`docs/cli-parity.md` (§ *Roadmap parsing rules*, regra 6)
**Método:** medir. 🔴 **Não escrever implementação.** A Wave 0 tem autoridade para **bloquear**.

**Perguntas, em ordem de peso:**

1. 🔴 **O que cada uma das 10 superfícies deve fazer?** A regra 6 diz *exit 2*, mas `serve` é
   **servidor HTTP** — derrubá-lo por um roadmap malformado é pior que o defeito. Proponha o
   comportamento de cada uma, **com o motivo**. Candidatos para o `serve`: marcar o card como
   malformado no board · omitir o arquivo e registrar · servir o que der e sinalizar. **Meça o que o
   `serve` já faz hoje com o arquivo de cerca aberta que existe no acervo** — ele está lá, é o
   `ROADMAP-2026-08-22-wave-0-...` (abre na linha 460).
2. **`ParseGates` já detecta** e emite `unterminated gates fence starting at line %d`. Por que o
   `FenceMask` não? É esquecimento, ou há razão — `FenceMask` devolve `[]bool` e mudar a assinatura
   toca 10 sítios. **Qual é o menor corte que cumpre a regra 6 sem quebrar os 10?**
3. 🔴 **Os 2 arquivos do acervo: a cauda mascarada é REALMENTE só prosa?** Eu medi `### ML-`,
   `**Status:**`, `- [ ]` e `## Wave ` — **meça também** `**Critérios`, `**Gates`, `**Owner`,
   `- [x]`, e qualquer marcador que o parser leia. Se houver conteúdo governante escondido ali, o
   defeito **já** está agindo e a REQ muda de severidade.
4. **O que mais o `FenceMask` mascara em silêncio hoje?** Cerca aberta é um caso de entrada
   malformada; existem outros nessa família que a regra 6 promete e o produto não cumpre?

**Ataque as minhas premissas — duas Waves 0 desta sessão derrubaram premissas minhas e estavam certas:**

- Afirmo que o **#470/PR #475 não fecha isto**, porque rodei a sonda com o binário da `main` (que já
  tem o #475) e o braço D saiu `mls_complete: passed`. **Verifique por conta própria.** Se a causa for
  a mesma, esta REQ tem de ser absorvida, não aberta.
- Afirmo **2** arquivos com cerca aberta em 235. Reimplemente a contagem e diga se bate.
- Afirmo que **leniência não se aplica** porque isto é *usage error*, não *violation*. Se o projeto
  tiver precedente de leniência para erro de entrada, **diga** — muda o desenho.

**Critérios de aceite:**
- [x] Tabela das 10 superfícies com o comportamento proposto e o **motivo** de cada um
- [x] Resposta à pergunta 2 com o **menor corte** identificado
- [x] Recontagem independente dos arquivos de cerca aberta, e o conteúdo real das caudas
- [x] Veredito sobre o #470: mesma causa ou não, **com a medição**
- [x] 🔴 Se a medição refutar qualquer premissa da REQ, **diga**
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**

> 🔴 **O gate de uma wave NUNCA pode ser `trackfw barrier` sobre a própria wave.** Eu escrevi isso
> aqui em 2026-09-30 e o produto recursou sem limite: **3469 processos** em ~6 min, load 12.9.
> O gate tem de ser um comando de **verificação**, não o executor que o invoca. Defeito de produto
> registrado em issue própria; o erro de escrita era meu.

> ⚠️ **Gate reescrito em 2026-09-30, por dois defeitos meus.** (1) Era um `python3 -c` de várias
> linhas: o `barrier` executa **cada linha** como `sh -c` separado, e sob o barrier real ele reprovava.
> (2) Afirmava `5` sítios de cerca aberta — número que a Wave 1 existe para **baixar**: ficaria
> vermelho por construção assim que a correção entrasse. A contagem passou para o gate da Wave 1; o
> gate da Wave 0 afirma só o que continua verdadeiro: o parecer existe e tem a tabela por superfície.

```bash
test -f docs/seguranca/2026-09-30-wave0-cerca-nao-terminada.md || { echo 'GATE FALHOU: parecer da Wave 0 ausente' >&2; exit 1; }
grep -q '### Tabela de comportamento proposto' docs/seguranca/2026-09-30-wave0-cerca-nao-terminada.md || { echo 'GATE FALHOU: parecer sem a tabela por superficie' >&2; exit 1; }
```

## ✅ Desbloqueado em 2026-09-30 — o #485 foi mergeado

O bloqueio era por compartilhar `internal/commands/barrier.go` com o #485 (gate reentrante). O PR
**#486** foi mergeado em `255f1384` e a `main` foi trazida para esta branch (merge `218beb3c`). O
`barrier.go` daqui já tem a pilha de reentrada.

Histórico: o #485 apareceu **ao escrever o gate da Wave 0 deste roadmap** — `trackfw barrier <este
roadmap> --wave 0` dentro do próprio bloco de gates; 3469 processos em ~6 min.

## Auditoria da Wave 0 — correções do arquiteto ao parecer

O parecer foi aceito com **três correções medidas**, que os MLs abaixo já incorporam:

1. **Sítio 3 não é `roadmap show`.** O único chamador de `pendingMLsForDone` é
   `internal/generators/roadmap.go:739`, no caminho do **`roadmap move ... done`**. Cerca aberta ali
   significa **recusar a transição para `done`**, e não exit 2 de leitura.
2. **O board não mostra `malformed_waves`.** Nenhum arquivo em `internal/serve/static/` lê esse
   campo; o parecer presumiu que sim. **Decisão do KG (2026-09-30):** o card ganha um **selo de
   "roadmap malformado"**, que cobre tanto a cerca aberta quanto o `malformed_waves` já existente.
   Isso cria o ML-4A, de UI.
3. **Dos 5 sítios, 2 são acervo e 3 são fixture congelada:**
   `scripts/testdata/roadmap-barrier-corpus-snapshot/…2026-08-22…` e as 2 cópias em
   `internal/roadmapdoc/testdata/corpus/docs/roadmaps/done/`. A recomendação do parecer de "fechar a
   cerca no snapshot" é opinião, não medição. **As fixtures ficam abertas**, e passam a servir de
   regressão real. O que um consumidor delas mudar é medido e escrito no ML-1A.

**Mensagem canônica** (molde do `ParseGates`, regra 6): `unterminated code fence starting at line <n>`.
Cada superfície a prefixa do seu jeito, e o número da linha é 1-based.

## Wave 1 — detecção no `roadmapdoc` e conserto do acervo (1 ML)
> Dependências: Wave 0 auditada. Sozinha: todas as superfícies dependem do `FenceMaskCheck`.

### ML-1A — `FenceMaskCheck`, predicados fail-closed e os 2 arquivos do acervo
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/roadmapdoc/roadmapdoc.go` · `internal/roadmapdoc/fencecheck_test.go`
(novo) · `docs/roadmaps/done/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md`
· `docs/roadmaps/done/ROADMAP-2026-08-29-dialeto-canonico-do-roadmap-e-vocabulario-de-status-do-barrier.md`
**Ações:**
1. `func FenceMaskCheck(lines []string) (openLine int, err error)`, com a **mesma** gramática do
   `FenceMask` (reuse `DetectFenceMarker` e a regra de fechamento; não duplique a lógica, extraia um
   helper comum se precisar). Se a cerca fechar: `(0, nil)`. Se não:
   `(n, fmt.Errorf("unterminated code fence starting at line %d", n))`.
   **O `FenceMask` não muda de assinatura nem de comportamento.**
2. Predicados **fail-closed** quando `FenceMaskCheck` falha, com o mesmo padrão que eles já usam para
   `len(malformed) > 0`: `HasUnfinishedMLs` → `true` · `hasAnyNonPendingML` → `true` · `HasWave0` →
   `false` · `Wave0GateDiagnosis` → não-OK, com a mensagem canônica. `DuplicateWaveOrMLLabels`
   **não muda**, porque a regra nova do `validate` (ML-3B) cobre o caso.
3. **Acervo (AC5):** feche a cerca dos 2 arquivos reais. Leia a cauda e ponha o fechador **onde o
   bloco de código acaba de fato**, não no fim do arquivo. Depois disso, `FenceMaskCheck` tem de dar
   `(0, nil)` nos dois. Registre no relatório a linha de abertura, onde você fechou e por quê.
4. 🔴 **Consumidores das fixtures:** enumere todo teste ou script que lê as 3 cópias congeladas
   (`compare_baseline_test.go`, `parsewaves_fence_test.go`, `wave0_gate_cause_test.go`,
   `scripts/check-roadmap-barrier-contract.sh`, qualquer MANIFEST/sha256). **Não edite as
   fixtures.** Se a mudança do item 2 alterar o resultado esperado de algum teste, atualize a
   expectativa **com a medição antes e depois escrita no relatório**.

**Testes** (`fencecheck_test.go`): cerca fechada → `(0,nil)` · aberta com crase → linha certa ·
aberta com til · fechador **mais curto** não fecha · fechador de **outro caractere** não fecha ·
🔴 **as 3 fixtures congeladas** dão exatamente as linhas `460`, `460` e `1044` · cada predicado do
item 2 fail-closed com cerca aberta **e** inalterado com o mesmo documento fechado.
🔴 **`TestAcervoSemCercaAberta`**: percorre `docs/roadmaps/**/*.md` a partir da raiz do módulo e
falha nomeando arquivo e linha se `FenceMaskCheck` acusar cerca aberta. É o gate de acervo das
Waves 1 e 4; **o nome é contrato**.

**Critérios de aceite:**
- [x] `FenceMaskCheck` existe; `FenceMask` com assinatura e comportamento intactos (os testes existentes passam sem edição)
- [x] Os 4 predicados são fail-closed, com teste nos dois braços
- [x] Os 2 arquivos do acervo fecham, e o relatório diz onde e por quê

> 🔴 **Auditoria (arquiteto, 2026-10-01): o fechador do executor estava no lugar errado, e corrigi eu
> mesmo.** Ele pôs um ```` ``` ```` perto da linha acusada (455 / 1034): a contagem fechava, mas os pares
> continuavam deslocados, com prosa lida como código. A causa real era um bloco externo que transcreve
> outra cerca (linhas **223–236** e **948–963**): o ```` ``` ```` interno fechava o externo. Remédio:
> ```` ```` ```` no bloco externo. Todos os MLs dos dois roadmaps continuam `complete`. Nota:
> `vault/notes/cerca-aberta-acusada-longe-da-causa-bloco-externo-sem-crase-extra-2026-10-01.md`.
- [x] Consumidores das fixtures enumerados; toda expectativa alterada vem com a medição antes/depois
- [x] `go build ./...` · `go test ./internal/roadmapdoc/ ./internal/generators/ ./internal/validator/ -count=1` verdes
- [x] Uma frase por teste novo dizendo o que ele afirma

**Gates da wave:**
```bash
go build ./...
go test ./internal/roadmapdoc/ -count=1
n=$(go test ./internal/roadmapdoc/ -run '^TestAcervoSemCercaAberta$' -count=1 -v 2>&1 | grep -c '^--- PASS: TestAcervoSemCercaAberta'); test "$n" = "1" || { echo "GATE FALHOU: TestAcervoSemCercaAberta nao passou (ou nao existe)" >&2; exit 1; }
```

## Wave 2 — CLI (2 MLs em paralelo)
> Dependências: Wave 1 auditada. Arquivos disjuntos: `internal/commands/` × `internal/generators/`.

### ML-2A — `barrier`: exit 2 pela cerca, antes de resolver a wave
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/commands/barrier.go` · `internal/commands/barrier_fence_test.go` (novo)
**Ações:**
1. Em `runBarrier`, logo **depois** de `splitRoadmapLines` e **antes** de `fenceMask`/`parseWaves`:
   `if _, err := roadmapdoc.FenceMaskCheck(lines); err != nil { usageExit(cmd, "%s", err.Error()) }`.
   🔴 **A posição é obrigatória:** uma cerca aberta antes do cabeçalho da wave pedida esconde a wave,
   e o usuário receberia `wave X not found`, que é a mensagem **errada** com o mesmo exit 2. É o
   oposto da precedência dada à checagem de reentrada no #485, e é de propósito.
2. A mensagem final no stderr é `trackfw barrier: unterminated code fence starting at line <n>`.

**Testes:** braço D da sonda (cerca aberta, ML pendente na cauda) → exit 2 e a mensagem com a linha
certa · 🔴 **cerca abrindo ANTES do cabeçalho da wave pedida → mensagem da cerca, não `wave not
found`** · roadmap bem-formado → comportamento inalterado (AC3) · `--json` também sai 2, sem documento.

**Critérios de aceite:**
- [x] AC2/AC3/AC4 cobertos pelos testes acima; o AC4 medido com `barrier --json` sobre a fixture do braço D
- [x] `go test ./internal/commands/ -count=1` verde
- [x] Uma frase por teste novo

### ML-2B — `roadmap move ... done` e `roadmap show`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/generators/roadmap.go` · `internal/generators/roadmap_show_json.go` ·
`internal/generators/roadmap_fence_test.go` (novo)
**Ações:**
1. `roadmap move <x> done` (bloco em `roadmap.go` ~:732): antes de `pendingMLsForDone`, chame
   `FenceMaskCheck`; se falhar, **recuse a transição** com a mensagem canônica, no mesmo formato de
   recusa que os outros bloqueadores já usam. O arquivo não se move.
2. `roadmap show` e `roadmap show --json` (`buildRoadmapShowDoc` ~:128): com cerca aberta, **exit 2**
   e a mensagem canônica no stderr, sem documento parcial.
3. Verifique se `pendingMLsForDone` e `buildRoadmapShowDoc` têm outros chamadores (`grep`, fora de
   `testdata/`) e registre no relatório.

**Testes:** move→done recusado com cerca aberta e aceito com o mesmo arquivo fechado · show e show
`--json` saem 2 com cerca aberta · roadmap bem-formado inalterado.

**Critérios de aceite:**
- [x] Os 3 comportamentos cobertos nos dois braços
- [x] `go test ./internal/generators/ -count=1` verde
- [x] Uma frase por teste novo

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ ./internal/generators/ -count=1
```

### ML-2C — o exit 2 do `show` sai do `generators` e vai para `commands`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Origem:** auditoria do ML-2B. O comportamento está certo, mas `roadmap_show_json.go` chama
`os.Exit(2)` (`fenceExitUsage`), o que torna `generators` o único pacote fora de `commands` a sair
do processo. E os testes precisam de um `init()` que reexecuta o binário de teste do pacote inteiro.
**Arquivos afetados:** `internal/generators/roadmap_show_json.go` · `internal/generators/roadmap_fence_test.go` ·
`internal/commands/roadmap.go` · um teste em `internal/commands/`
**Ações:**
1. Em `generators`: `type UsageError struct{ Msg string }` com `Error()`. `ShowRoadmap`/`ShowRoadmapJSON`
   **retornam** `&UsageError{...}` com a mensagem canônica, sem imprimir no stdout. Remova `fenceExitUsage`
   e todo `os.Exit` do pacote.
2. Em `commands/roadmap.go` (~:136-138): `var ue *generators.UsageError; if errors.As(err, &ue)` →
   stderr `trackfw roadmap: <msg>` e `os.Exit(2)`. Os outros erros seguem o caminho de hoje (exit 1).
3. Testes do `generators` **em processo**: `errors.As(err, &ue)` com a mensagem e o stdout vazio. **Remova o
   `init()` e o `TRACKFW_TEST_FENCE_HELPER`.**
4. Um teste no nível do binário em `internal/commands/` (helper `barrierBinary(t)`, que já builda o
   `trackfw`): `roadmap show <cerca aberta>` e `--json` saem **2**; com a cerca fechada saem 0.
**Critérios de aceite:**
- [x] `grep -n 'os.Exit' internal/generators/*.go | grep -v _test` vazio
- [x] `grep -rn 'TRACKFW_TEST_FENCE_HELPER\|func init()' internal/generators/roadmap_fence_test.go` vazio
- [x] exit 2 medido no binário (teste do item 4) · `go test ./internal/commands/ ./internal/generators/ -count=1` verde
- [x] Uma frase por teste novo ou alterado

## Wave 3 — superfícies que não podem sair 2 (2 MLs em paralelo)
> Dependências: Wave 2 auditada. Arquivos disjuntos: `internal/serve/api_board.go` × `internal/validator/` + `scripts/check-validate-rule-pins.sh`.

### ML-3A — `serve`: campo na API, sem derrubar o servidor
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/serve/api_board.go` · `internal/serve/api_board_test.go`
**Ações:** no item do board, o campo **`UnterminatedFenceLine int` com `json:"unterminated_fence_line,omitempty"`**,
preenchido por `FenceMaskCheck` em `parseMLProgressFull`. **O nome JSON é contrato com o ML-4A: não
renomeie.** O servidor loga uma linha e continua. `ml_total`/`ml_done` **não** são "corrigidos" por
heurística; o selo é o sinal.

**Critérios de aceite:**
- [x] Teste: `/api/board` com fixture de cerca aberta → 200, e o item traz `unterminated_fence_line` = linha certa · com fixture bem-formada o campo está ausente (AC6)
- [x] `go test ./internal/serve/ -count=1` verde
- [x] Uma frase por teste novo

**Testes novos (3 testes — todos PASS):**
- `TestBoardHandler_UnterminatedFence_FieldPresent` — afirma que quando um roadmap tem cerca aberta, o item no JSON do `/api/board` traz `unterminated_fence_line` com o número da linha onde a cerca foi aberta, e o servidor retorna 200.
- `TestBoardHandler_WellFormedRoadmap_FenceFieldAbsent` — afirma que quando todas as cercas estão fechadas, o campo `unterminated_fence_line` está ausente do JSON (omitempty).
- `TestBoardHandler_MalformedAndWellFormed_BothListed` — afirma que quando o diretório contém um roadmap malformado e um bem-formado, o servidor retorna 200 e lista os dois (AC6: nunca derruba nem omite itens por causa de cerca aberta).

**Resultados:**
- `go build ./...`: ok
- `go vet ./internal/serve/`: ok
- `go test ./internal/serve/ -count=1`: ok

### ML-3B — `validate`: regra `roadmap_unterminated_fence`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/validator/validator_roadmap_gates.go` (ou arquivo vizinho no mesmo
padrão) · teste em `internal/validator/` · `scripts/check-validate-rule-pins.sh`
**Ações:** regra nova `roadmap_unterminated_fence`, aplicada a **todos** os estados de roadmap, com a
mensagem `<arquivo>: unterminated code fence starting at line <n>` e os mesmos helpers
`readFileForRule`/`inspectionDiagnostic` das regras vizinhas. Em `governance_mode: lenient` ela vira
warning, **como toda violation**. A premissa 4 da Wave 0 (sem leniência) vale para o usage error da
CLI, não para o `validate`, e essa diferença vai para o `cli-parity` no ML-4B. Acrescente um pin em
`check-validate-rule-pins.sh`. ⚠️ **Armadilha registrada no vault:** o script reusa `pin6`/`pin7`/`pin8`
entre blocos, e um rótulo duplicado passa **sem detecção**. Use um rótulo inédito e confira com `grep`.

**Critérios de aceite:**
- [x] Teste nos dois braços (cerca aberta → violation com a linha · fechada → nada)
- [x] `bash scripts/check-validate-rule-pins.sh` verde, com o pin novo contado
- [x] `go test ./internal/validator/ -count=1` verde
- [x] Uma frase por teste novo

**Testes novos (3 testes — todos PASS):**
- `TestRoadmapUnterminatedFence_OpenFence` — afirma que um roadmap em wip/ com cerca aberta na linha 5 produz uma violation da regra `roadmap_unterminated_fence` nomeando o arquivo e a linha.
- `TestRoadmapUnterminatedFence_ClosedFence` — afirma que um roadmap com todas as cercas fechadas não produz nenhuma violation da regra `roadmap_unterminated_fence`.
- `TestRoadmapUnterminatedFence_DoneState` — afirma que a regra cobre o estado done/ (não só wip/), provando cobertura universal de estados.

**Pins (Block 5):** `pin26` (open fence → violation com linha) · `pin27` (closed fence → silêncio).
`grep -c 'pin26' scripts/check-validate-rule-pins.sh` → 2 (header + OK print; sem duplicata entre blocos).

**Acervo real:** `tf3b validate 2>&1 | grep -c roadmap_unterminated_fence` → 0 (ML-1A fechou os 2 arquivos).

**Gates da wave:**
```bash
go build ./...
go test ./internal/serve/ ./internal/validator/ -count=1
bash scripts/check-validate-rule-pins.sh
```

## Wave 4 — selo no board e contrato (2 MLs em paralelo)
> Dependências: Wave 3 auditada. Arquivos disjuntos: `internal/serve/static/` × `docs/cli-parity.md`.

### ML-4A — selo de "roadmap malformado" no card
**Owner:** `afrodite-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/serve/static/app.js` · `internal/serve/static/style.css`
**Ações:** quando `card.unterminated_fence_line > 0` **ou** `card.malformed_waves > 0`, o card mostra
um selo visível ("roadmap malformado"), com texto ou tooltip dizendo a causa e a linha. A barra de
progresso **não** pode parecer completa quando o selo está presente. O board é **light-only**: sem
`prefers-color-scheme`. Os estáticos são `go:embed`, e esta é a fonte canônica.

**Critérios de aceite:**
- [x] 🔴 **Verificação visual em navegador real**, feita pelo arquiteto: `trackfw serve` sobre uma
  árvore com uma fixture de cerca aberta, uma com `malformed_waves` e uma bem-formada; selo nas duas
  primeiras, ausente na terceira
- [x] `go build ./...` (o embed compila)

> ✅ **Verificação visual (arquiteto, 2026-10-01)**, com Chrome real em modo headless contra o `serve` de um
> binário recém-buildado, sobre 4 roadmaps: wave inválida → selo · cerca aberta 0/1 → selo, `0/?` ·
> 🔴 **cerca aberta escondendo ML pendente com os visíveis 1/1** → selo, `1/?` e barra listrada âmbar no
> lugar da verde cheia (era o sintoma do #476) · bem-formado → sem selo, `1/1` verde. O 4º caso não
> estava na fixture da executora; acrescentei porque é o que a REQ existe para pegar.

### ML-4B — `cli-parity.md` por superfície e `make quality`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/cli-parity.md` (regra 6 em § *Roadmap parsing rules*, § `trackfw barrier`,
§ `roadmap move`/`show`, § `serve`, § `validate`)
**Ações:** a regra 6, cláusula 3, passa de promessa a descrição **por superfície**: `barrier`/`show`
saem com exit 2 · `move done` recusa · `serve` usa campo e selo, nunca sai 2 · `validate` gera
violation, warning em lenient. Cláusula 2 ("ML body cannot be delimited"): declare que é **letra
morta** no parser atual. Atualize o comentário `trackfw-contract` da linha ~2548, que diz que a regra
6 não tem cenário. Rode `make quality` (autorizado: ML final).

**Critérios de aceite:**
- [x] AC7: cada superfície descrita com a mensagem literal
- [x] `make quality` verde (a última linha real no relatório)

### ML-4C — re-pinar o corpus do contrato do barrier (a fixture aberta agora sai 2)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Origem:** `make quality` do ML-4B, `EXIT=2` em `scripts/check-roadmap-barrier-contract.sh`. A fixture
congelada `scripts/testdata/roadmap-barrier-corpus-snapshot/ROADMAP-2026-08-22-…`, que a auditoria da
Wave 0 decidiu **manter aberta**, agora sai com exit 2 nas suas 3 waves, que era o objetivo. Medido
pelo arquiteto: os números fecham exatamente com 3 waves (e/e, e/f, e/f) saindo do veredito
individual: `EXIT2` 3→6 · `MLS_COMPLETE_EVIDENCE` 656→653 · `ACCEPTANCE_EVIDENCE_EVIDENCE` 318→317 ·
`ACCEPTANCE_EVIDENCE_FAILURE` 452→450.
🔴 **Por que só apareceu agora:** eu proibi o `make quality` nos MLs paralelos da Wave 2, que foi onde o
`barrier` mudou. O ML-1A reportou o script como "sem efeito" porque, naquele ML, o `barrier` ainda
não chamava `FenceMaskCheck`.
**Arquivos afetados:** `scripts/check-roadmap-barrier-contract.sh` ·
`scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`
**Ações:** seguir o padrão "Re-pinado em ML-x" que já existe no script (comentários ~:381-470):
atualize `PINNED_CORPUS_EXIT2`, as contagens e o `PINNED_CORPUS_HASH`, regenere o TSV de vereditos
pelo mesmo procedimento documentado no script e escreva o comentário "Re-pinado em ML-4C (#476)" com
a causa e os números antes/depois. Atualize o comentário ~:571-581 ("Daí PINNED_CORPUS_EXIT2=3"): o
exit 2 agora tem **duas** causas, cabeçalho malformado (3) e cerca não terminada (3). **Não edite a
fixture.**
**Critérios de aceite:**
- [x] `bash scripts/check-roadmap-barrier-contract.sh` verde
- [x] O diff do TSV mostra só as linhas do `ROADMAP-2026-08-22-…`; nenhum outro arquivo do corpus muda de veredito
- [x] `make quality` com `EXIT=0`

**Gates da wave:**
```bash
go build ./...
n=$(go test ./internal/roadmapdoc/ -run '^TestAcervoSemCercaAberta$' -count=1 -v 2>&1 | grep -c '^--- PASS: TestAcervoSemCercaAberta'); test "$n" = "1" || { echo "GATE FALHOU: TestAcervoSemCercaAberta nao passou (ou nao existe)" >&2; exit 1; }
```
