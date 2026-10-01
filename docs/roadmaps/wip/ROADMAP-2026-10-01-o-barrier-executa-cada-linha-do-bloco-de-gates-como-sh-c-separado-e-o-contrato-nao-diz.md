---
status: wip
date: 2026-10-01
req: "docs/req/REQ-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md"
squad: "hades-tf"
---

# Roadmap: o barrier executa cada linha do bloco de gates como sh -c separado e o contrato não diz a consequência

> Created: 2026-10-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md
Issue: #491 · ADR: `ADR-2026-09-01` (gate é contrato POSIX shell) · Notas de vault:
`parsegates-per-line-isolation-fuse-same-line-2026-09-30.md`, `barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`

Varredura (2026-10-01): nenhuma issue ou REQ aberta com este mecanismo. **Premissa do #491
corrigida** (comentário no issue): a execução por linha **está** na regra 5 do `cli-parity.md`. O que
falta é a consequência, o aviso na superfície de autoria e a distinção entre "bloco malformado" e
"gate reprovou".

## Acceptance Criteria
- [x] **AC1** — decisão (a) documentar · (b) detectar fragmento · (c) bloco como script, com medição do acervo
- [x] **AC2** — regra 5 escreve a consequência, com exemplo
- [x] **AC3** — template e assets de autoria avisam no ponto em que o gate é escrito
- [x] **AC4** — comportamento do `barrier` conforme o AC1
- [x] **AC5** — 🔴 todo bloco do acervo que passa hoje continua passando
- [ ] **AC6** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — medir o acervo e decidir entre (a), (b) e (c)
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Arquivos de leitura:** `internal/roadmapdoc/roadmapdoc.go` (`ParseGates`) · `internal/commands/barrier.go`
(`runGateCommand`, `evalGateCommands`) · `docs/cli-parity.md` (regra 5, ~:2619, e § *Wave gates are a
portable POSIX-shell contract*) · ADR-2026-09-01 · o template de Wave 0 em `internal/generators/` ·
as duas notas de vault acima
**Entregável:** `docs/seguranca/2026-10-01-wave0-gate-por-linha.md`
**Método:** medir. 🔴 **Não escrever implementação.**

**Perguntas:**
1. **O acervo.** Extraia **todos** os blocos de gate do acervo (`docs/roadmaps/**`, as fixtures
   em `scripts/testdata/` e `internal/roadmapdoc/testdata/`) com a mesma regra do `ParseGates`. Quantos
   blocos: (i) têm alguma linha que **não é comando completo** sozinha (aspas desbalanceadas, `$(`/`(`/`{`
   sem fechar, `\` no fim, heredoc)? (ii) **dependem** de estado de uma linha anterior (`export`, `cd`,
   atribuição usada depois)? (iii) dependem de **todas** as linhas rodarem mesmo quando uma no meio
   falha? É isso que separa (c) de (b).
2. **Candidatos.** Para (a), (b) e (c), o que cada um quebra e o que deixa passar, com o contra-braço
   medido. Em (c), diga como fica a evidência por comando (`<cmd>: exit N`), que o JSON do barrier e o
   corpus do contrato (`scripts/check-roadmap-barrier-contract.sh`) já fixam, e o que acontece sem
   `set -e`: um script cujo **último** comando passa mascara uma falha anterior. Em (b), como detectar
   "comando incompleto" **sem** reimplementar um parser de shell (por exemplo `sh -n` por linha), qual o
   custo e quais os falsos positivos.
3. **Superfície de autoria.** Enumere todo lugar em que um autor aprende a escrever um gate: o template
   gerado, os assets de agente e skill gerados, o `cli-parity.md` e o que mais existir.
4. **Ameaça.** Alguma das opções abre um caminho novo para conteúdo do roadmap chegar ao `sh`
   (diferente de hoje)? Exemplo: em (c), o bloco inteiro passa a ser **um** argumento. Isso muda a
   relação com o trust check?

**Ataque as minhas premissas:**
- Afirmo que nenhum gate do acervo depende de estado entre linhas, porque isso nunca funcionou.
  **Meça**: pode haver gate que "funciona por acaso" (a linha 2 só lê arquivo, a variável não importa).
- Afirmo que (b) é o menor corte que protege o autor. Se (c) for mais simples e não quebrar o acervo
  nem a evidência, **diga**.

**Critérios de aceite:**
- [x] Contagens (i), (ii) e (iii) sobre o acervo, com o comando usado
- [x] (a), (b) e (c), cada um com o contra-braço medido e o veredito
- [x] Superfície de autoria enumerada com `arquivo:linha`
- [x] Recomendação, com o resíduo declarado
- [x] Nenhuma linha de implementação

**Gates da wave:**

> 🔴 Cada linha deste bloco é um comando separado: essa é a semântica que este roadmap documenta.

```bash
test -f docs/seguranca/2026-10-01-wave0-gate-por-linha.md || { echo 'GATE FALHOU: parecer da Wave 0 ausente' >&2; exit 1; }
grep -qi 'recomenda' docs/seguranca/2026-10-01-wave0-gate-por-linha.md || { echo 'GATE FALHOU: parecer sem recomendacao' >&2; exit 1; }
```

## Auditoria da Wave 0 (arquiteto, 2026-10-01)

Parecer aceito: **opção (b) + o componente documental de (a)**. Medi o acervo de forma independente
(87 blocos / 176 comandos em `docs/roadmaps`; 11 falham `sh -n`, **todos** no mesmo bloco de
`done/ROADMAP-2026-08-28-…`) e bate com o parecer.

**Decisões:**
1. Fragmento incompleto vira **falha do check `gates`**, não exit 2. Mensagem: `line <n>: incomplete
   command — each line of the gates block runs as a separate sh -c (rule 5): <cmd>`. **Nenhuma** linha
   do bloco executa se houver fragmento. O `sh -n` roda **depois** do trust check; se o `sh` não puder
   ser iniciado, `not_evaluated` com `shMissingMsg`. Exit 2 está fora porque um bloco malformado é
   avaliado e reprovado, e porque o corpus do contrato (que roda sem trust) não pode mudar.
2. **F1 entra nesta REQ, como ML próprio.** Causa distinta, medida: o `ParseGates` não consulta a
   máscara de cerca para o **marcador** `**Gates da wave:**`. Um marcador dentro de um bloco de exemplo
   vira gate real (`barrier --wave 2` do `done/ROADMAP-2026-08-22` extrai `exit 1 # placeholder…` de
   um exemplo). Vai junto porque mexe na mesma função e viola a mesma regra 5 ("the barrier never
   invents a gate").
3. **O bloco quebrado do acervo** (`done/ROADMAP-2026-08-28`) é reescrito em linhas completas no mesmo
   PR. Com (b), ele reprovaria com a mensagem nova, e corrigir o acervo junto com a detecção é o
   precedente do #476.

## Wave 1 — parser e documentação (2 MLs em paralelo)
> Dependências: Wave 0 auditada. Arquivos disjuntos: `internal/roadmapdoc/` × (`docs/`, `README.md`,
> `vault/`, `internal/generators/`).

### ML-1A — `ParseGates` com número de linha e o marcador fora de cerca (F1)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/roadmapdoc/roadmapdoc.go` · `internal/roadmapdoc/gates_lines_test.go` (novo) · `internal/roadmapdoc/testdata/barrier-baseline.txt` (baseline atualizada para o delta F1)
**Ações:**
1. `type GateCmd struct{ Line int; Text string }` (linha 1-based) e
   `func ParseGatesLines(lines []string, waveStart, waveEnd int) ([]GateCmd, error)`, com a **mesma**
   gramática de hoje. `ParseGates` vira um wrapper que devolve só os textos, sem mudar a assinatura e
   sem quebrar os chamadores `internal/commands/barrier.go:196` e `roadmapdoc.go:798`.
2. **F1:** um `**Gates da wave:**` em linha mascarada por `FenceMask` **não** é marcador. Use a máscara
   que já existe; não reimplemente a gramática de cerca.

**Testes:** as linhas dos comandos batem com o arquivo · comentário e linha vazia não entram · 🔴 F1:
marcador dentro de um bloco de exemplo cercado com 4 crases **não** gera gate, e o mesmo marcador fora
da cerca gera · medido no acervo real: `ParseGates` sobre a Wave 2 do `done/ROADMAP-2026-08-22-…` passa
a devolver `[]` · os testes existentes do `roadmapdoc` passam sem edição.

**Critérios de aceite:**
- [x] `ParseGatesLines` e o wrapper existem; os chamadores compilam sem mudança
- [x] F1 coberto nos dois braços, mais a medição no arquivo real
- [x] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/ -count=1` verde
- [x] Uma frase por teste novo

### ML-1B — a consequência escrita onde o autor aprende a escrever gate
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `docs/cli-parity.md` (regra 5, ~:2619) · `README.md` (~:480) ·
`vault/notes/barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md` (linhas ~49–52) ·
`docs/roadmaps/done/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md` ·
`internal/generators/roadmap.go` e `internal/generators/scaffold.go` **só se** o item 4 permitir
**Ações:**
1. **Regra 5:** escrever a consequência: cada linha é um `sh -c` próprio, sem estado entre linhas
   (`export`, `cd`, variável) e sem construção multilinha (aspas, `$(`, heredoc, `\` no fim). Dar um
   exemplo errado e o certo (`n=$(…); test "$n" = 0 || { …; exit 1; }` numa linha só), e o conselho da
   nota de 2026-08-29: lógica grande vai para um script em `scripts/`. Linkar
   `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md`.
2. **README** (~:480): uma frase com o mesmo aviso, linkando a regra 5.
3. **Nota de vault do fork bomb:** corrigir o "Molde que funciona" para uma linha só (F2).
4. **Template** (`wave0GateFence`, `roadmap.go:45`, e o mesmo bloco em `scaffold.go:467`): o código diz
   que a constante é **byte-idêntica em todo projeto que roda `trackfw update`**. 🔴 **Antes** de editar,
   meça o que muda para um consumidor que já tem o placeholder antigo (`update`/`doctor`, detecção de
   drift, testes golden e cenários de `check-gates-falsify.sh` que fixam o literal). Se a mudança for
   segura, acrescente dentro da cerca uma linha de **comentário** (`# each line runs as a separate sh -c
   — see rule 5`), que o `ParseGates` ignora. Se não for segura, **não mexa na constante**: ponha o aviso
   como prosa logo antes da cerca, no texto do ML-0A gerado, e reporte a medição.
5. **Acervo:** reescreva o bloco de gates do `done/ROADMAP-2026-08-28-…` em linhas completas e
   independentes, preservando o que cada gate afirmava. Depois, `sh -n -c` passa em todas as linhas.

**Critérios de aceite:**
- [x] Regra 5 e README com a consequência e o exemplo
- [x] Molde da nota de vault em uma linha
- [x] Item 4: a medição no relatório e a escolha justificada
- [x] O bloco do `ROADMAP-2026-08-28` passa no `sh -n` linha a linha (zero falhas no acervo)
- [x] `go test ./internal/generators/ -count=1` verde, se o item 4 tocar `generators`

**Gates da wave:**
```bash
go build ./...
go test ./internal/roadmapdoc/ ./internal/generators/ -count=1
```

## Wave 2 — o `barrier` reprova fragmento antes de executar (1 ML)
> Dependências: Wave 1 auditada (usa `ParseGatesLines`).

### ML-2A — `sh -n` por linha no caminho de avaliação dos gates
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/commands/barrier.go` · `internal/commands/barrier_fragment_test.go`
(novo) · `docs/cli-parity.md` (§ `trackfw barrier`: a mensagem nova)
**Ações:**
1. Nos dois caminhos que chamam `evalGateCommands` (com `--trust-local-gates` e com trust via
   `origin/main`), **antes** de executar: para cada `GateCmd`, `sh -n -c <Text>` e "número ímpar de `\`
   no fim". Se algum falhar: `gates` = `blocked`, uma falha por linha
   (`line <n>: incomplete command — each line of the gates block runs as a separate sh -c (rule 5): <cmd>`),
   e **nenhum** gate executa. Se o `sh` não puder ser iniciado: `not_evaluated` com `shMissingMsg`.
2. Roadmap **não confiável**: nada muda. O `sh -n` não roda (`not_evaluated` como hoje).

**Testes:** fragmento (`n=$(python3 -c "`) → `blocked` nomeando a linha, e uma linha-sentinela
(`touch <arquivo>`) do mesmo bloco **não** executa · `\` ímpar no fim → `blocked` · `\\` par → executa
normalmente · bloco válido de vários comandos → evidência `<cmd>: exit N` por linha, inalterada (AC5) ·
roadmap não confiável → `not_evaluated`, sem spawn de `sh -n`.

**Critérios de aceite:**
- [x] Os 5 braços acima, com uma frase por teste
- [x] `make quality` com `EXIT=0` (autorizado: frente única), incluindo `check-roadmap-barrier-contract.sh` **sem** re-pin
- [x] Barrier real (binário novo) sobre as waves deste roadmap: `passed`

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'Fragment' -count=1
```

## Wave 3 — revisão de segurança (1 ML)
> Dependências: Wave 2 auditada.

### ML-3A — revisão independente do novo spawn de `sh`
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Entregável:** `docs/seguranca/2026-10-01-wave3-revisao-sh-n.md`
**Ações:** a partir da leitura do código, sem olhar os testes primeiro: o `sh -n` roda **só** depois do
trust check, em todos os caminhos? `sh -n` executa algo em algum `sh` real (dash, bash 3.2, busybox)?
Existe linha que passe no `sh -n` e mude de significado (por exemplo, uma interação de `#` com `\`)? O F1
pode esconder um gate legítimo, um marcador real que agora seria lido como "dentro de cerca"?

**Critérios de aceite:**
- [x] Cada pergunta com o comando e a saída
- [x] Veredito: aprova, ou bloqueia com um ML corretivo

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-01-wave3-revisao-sh-n.md
```

## Wave 4 — o argv do Windows aprova linha malformada (1 ML, corretivo)
> Dependências: Wave 3 auditada. Origem: **medição externa** no PR #495 e no #491 (Lourival, Windows 11,
> MINGW64, bash 5.2.26, Go 1.25.2) e o CI do PR (`windows-full-suites`).

**Causa medida pelo consumidor, mesma REQ pela Regra Dura de Causa Raiz.** O texto do gate chega ao `sh`
por **argv**. No Windows, o `EscapeArg` do Go não envolve em aspas um argumento **sem espaço** e
escapa o `"` embutido; o reparse do MSYS devolve `\` no lugar da aspa:

```
enviado                 recebido pelo sh
esperado="scaffold.go   esperado=\scaffold.go     ← atribuição VÁLIDA, sai 0
x="ab                   x=\ab                     ← idem
echo "abre              echo "abre                ← com espaço, ida e volta fiel
```

Efeito nos dois sítios:
- `checkGateFragments` (`sh -n -c`, desta PR) **não vê** o fragmento que a tabela da Wave 0 lista
  como verdadeiro positivo;
- `runGateCommand` (`sh -c`, **anterior** a esta PR) **executa** a linha malformada e a aprova:
  gate verde.

🔴 **Por que só apareceu de fora:** todas as medições desta REQ rodaram em macOS (bash 3.2, `bash
--posix`, dash). A Wave 0 declarou "zero instâncias" num ambiente em que essa forma não existe.

**Classe de consequência, no acervo do consumidor** (fork do Lourival, 81 roadmaps fora dos nossos
três universos): 18 blocos de gate, **5** afetados (16 linhas que falham `sh -n` e 3 com `\` final
solto), todos de Wave 0, e 🔴 **quatro em `done/`**. Antes da correção, essas waves saíam `blocked` sem
que o gate tivesse sido executado como escrito; uma wave foi fechada como concluída com um gate que
nunca pôde passar.

**Reproduzido pelo arquiteto na VM Windows** (2026-10-01, Windows 11 ARM64, bash 5.3.15 do Git, Go
1.27, binário real de `1290231b`): `esperado="scaffold.go` e `x="ab` saem `gates: passed`;
`echo "abre` sai `blocked`. Sonda das três formas de transporte: stdin e env+eval mantêm o
comportamento de hoje em todos os outros vetores (incluindo `cat`, `read` e `$0`), e argv é a única que
erra. Em macOS o executor mediu que o env+eval **diverge** (o `eval` do bash 3.2 sai 1 onde o `sh -c` sai
2). **Escolha: stdin.**

**CI do PR:** `TestBarrierFragment_UntrustedRoadmap_ShNotCalled` falha no Windows porque o `sh` falso
do teste é um script que o Windows não executa (o braço "com trust, o marcador aparece" não encontra o
marcador). É defeito do teste, não do produto.

### ML-4A — o texto do gate deixa de passar por argv
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/commands/barrier.go` (`checkGateFragments`, `runGateCommand`) ·
`internal/commands/barrier_fragment_test.go` · um teste novo de transporte · `docs/cli-parity.md`
(regra 5 e § POSIX shell contract) · `vault/notes/` (nota nova)
**Ações:**
1. **`sh -n` por stdin:** `c := exec.Command("sh", "-n"); c.Stdin = strings.NewReader(text)`. Nada
   executa sob `-n`, então trocar o transporte não muda a semântica.
2. **`runGateCommand`:** escolher, **por medição**, entre:
   - (i) stdin: `exec.Command("sh")` com o texto no stdin;
   - (ii) variável de ambiente: `exec.Command("sh", "-c", "eval \"$TRACKFW_GATE_CMD\"")`, com o texto
     em `TRACKFW_GATE_CMD` no `c.Env` (variável de ambiente não passa pelo `EscapeArg`).

   Critério: **paridade com o comportamento de hoje fora do Windows.** Meça em macOS, sobre os vetores
   do comentário do Lourival na #491 (`esperado="scaffold.go`, `x="ab`, `echo "abre`, `a="x" b="y"`,
   `esperado="scaffold go`, `false`, `true`, `exit 7`, `nosuchtool`) **e** sobre um gate que lê stdin
   (`cat`, `read x; test -z "$x"`). Hoje o stdin do gate é o dispositivo nulo do Go. Escolha a forma
   com paridade total e registre a tabela no relatório. Se nenhuma tiver paridade total, **pare e
   reporte**.
3. **Regressão do Windows nos dois sítios:** um teste que roda em **todo SO** com o vetor
   `esperado="scaffold.go`: no `barrier`, `gates: blocked` com `incomplete command`, e nada executa.
   No macOS e no Linux ele já passa hoje. O que ele afirma é que, no Windows, o transporte não troca a
   aspa, e é o job `windows-full-suites` que mede isso.
4. **Teste do `sh` falso portável:** troque o script por um binário Go buildado no teste (um `main`
   mínimo que grava o marcador e sai 0), com o nome `sh` (`sh.exe` no Windows). Os dois braços valem
   em todo SO.
5. **Contrato:** a regra 5 e o § *POSIX shell contract* dizem que o texto do gate chega ao `sh` sem
   passar por argv, e por quê (link para a nota nova).
6. **Nota de vault:** `windows-argv-troca-aspa-por-contrabarra-sem-espaco-2026-10-01.md`, com o
   mecanismo, a tabela e o crédito da medição.

**Critérios de aceite:**
- [x] `sh -n` por stdin; a forma do `runGateCommand` escolhida com a tabela de paridade no relatório
- [x] O teste do vetor `esperado="scaffold.go` existe e roda em todo SO
- [x] O teste do `sh` falso usa um binário Go, e os dois braços valem em todo SO
- [x] `make quality` com `EXIT=0` (autorizado: frente única)
- [ ] 🔴 CI do PR #495: `windows-full-suites` verde **sem** acrescentar nome a `.github/windows-known-failures.json`
- [x] Uma frase por teste novo

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'Fragment|Transport' -count=1
```

### ML-4B — invariante "um gate = uma linha" em código
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/commands/barrier.go` (`runGateCommand`, `checkGateFragments`) ·
`internal/commands/barrier_fragment_test.go`
**Ações:**
1. **Guard em `runGateCommand`:** se o texto contiver `\n` ou `\r`, retorna código **2** sem chamar o `sh`. Comentário: o stdin é seguro porque a regra 5 garante uma linha por gate; com mais de uma linha, um gate que lê stdin leria a própria próxima linha do script (medição do Lourival, PR #495).
2. **Guard em `checkGateFragments`:** se `gc.Text` contiver `\n` ou `\r`, acrescenta `line <n>: gate text spans multiple lines — the transport reads one line per gate (rule 5)` às falhas e continua (`continue`). Mesmo comentário citando o PR #495.
3. **Testes (nomes com `Transport`):**
   - `TestBarrierFragment_TransportMultiLineRunGate`: `runGateCommand` com texto de duas linhas (segunda linha = `touch <sentinela>`) retorna código ≠ 0, `spawnFailed=false`, sentinela ausente.
   - `TestBarrierFragment_TransportMultiLineCheckFragments`: `checkGateFragments` com `GateCmd{Text: "...\ntouch <sentinela>", Line: 42}` retorna `"blocked"` com a mensagem exata, sentinela ausente.
   - `TestBarrierFragment_TransportSingleLineReadContra`: `read x; test -z "$x"` (uma linha) sai 0.

**Critérios de aceite:**
- [x] Guard em `runGateCommand`: texto com `\n`/`\r` → código 2, sem spawn de sh
- [x] Guard em `checkGateFragments`: texto com `\n`/`\r` → "blocked" com mensagem exata
- [x] Comentário nos dois sítios citando a medição do Lourival no PR #495
- [x] Três testes novos: `TransportMultiLineRunGate`, `TransportMultiLineCheckFragments`, `TransportSingleLineReadContra`
- [x] `go build ./...` limpo · `go vet ./internal/commands/` limpo · `go test ./internal/commands/ -count=1` verde
- [x] `make quality` EXIT=0

**Comandos de validação:**
```bash
go build ./...
go vet ./internal/commands/
go test ./internal/commands/ -count=1
```
