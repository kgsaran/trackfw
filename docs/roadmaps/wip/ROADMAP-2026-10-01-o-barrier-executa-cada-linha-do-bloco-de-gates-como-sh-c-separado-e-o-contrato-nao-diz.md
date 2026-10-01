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
- [ ] **AC1** — decisão (a) documentar · (b) detectar fragmento · (c) bloco como script, com medição do acervo
- [ ] **AC2** — regra 5 escreve a consequência, com exemplo
- [ ] **AC3** — template e assets de autoria avisam no ponto em que o gate é escrito
- [ ] **AC4** — comportamento do `barrier` conforme o AC1
- [ ] **AC5** — 🔴 todo bloco do acervo que passa hoje continua passando
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
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/roadmapdoc/roadmapdoc.go` · `internal/roadmapdoc/gates_lines_test.go` (novo)
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
- [ ] `ParseGatesLines` e o wrapper existem; os chamadores compilam sem mudança
- [ ] F1 coberto nos dois braços, mais a medição no arquivo real
- [ ] `go test ./internal/roadmapdoc/ ./internal/commands/ ./internal/validator/ -count=1` verde
- [ ] Uma frase por teste novo

### ML-1B — a consequência escrita onde o autor aprende a escrever gate
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
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
- [ ] Regra 5 e README com a consequência e o exemplo
- [ ] Molde da nota de vault em uma linha
- [ ] Item 4: a medição no relatório e a escolha justificada
- [ ] O bloco do `ROADMAP-2026-08-28` passa no `sh -n` linha a linha (zero falhas no acervo)
- [ ] `go test ./internal/generators/ -count=1` verde, se o item 4 tocar `generators`

**Gates da wave:**
```bash
go build ./...
go test ./internal/roadmapdoc/ ./internal/generators/ -count=1
```

## Wave 2 — o `barrier` reprova fragmento antes de executar (1 ML)
> Dependências: Wave 1 auditada (usa `ParseGatesLines`).

### ML-2A — `sh -n` por linha no caminho de avaliação dos gates
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
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
- [ ] Os 5 braços acima, com uma frase por teste
- [ ] `make quality` com `EXIT=0` (autorizado: frente única), incluindo `check-roadmap-barrier-contract.sh` **sem** re-pin
- [ ] Barrier real (binário novo) sobre as waves deste roadmap: `passed`

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'Fragment' -count=1
```

## Wave 3 — revisão de segurança (1 ML)
> Dependências: Wave 2 auditada.

### ML-3A — revisão independente do novo spawn de `sh`
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Entregável:** `docs/seguranca/2026-10-01-wave3-revisao-sh-n.md`
**Ações:** a partir da leitura do código, sem olhar os testes primeiro: o `sh -n` roda **só** depois do
trust check, em todos os caminhos? `sh -n` executa algo em algum `sh` real (dash, bash 3.2, busybox)?
Existe linha que passe no `sh -n` e mude de significado (por exemplo, uma interação de `#` com `\`)? O F1
pode esconder um gate legítimo, um marcador real que agora seria lido como "dentro de cerca"?

**Critérios de aceite:**
- [ ] Cada pergunta com o comando e a saída
- [ ] Veredito: aprova, ou bloqueia com um ML corretivo

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-01-wave3-revisao-sh-n.md
```
