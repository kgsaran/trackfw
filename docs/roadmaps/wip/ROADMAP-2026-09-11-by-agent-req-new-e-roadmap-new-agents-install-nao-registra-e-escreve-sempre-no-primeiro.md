---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md"
squad: "apolo-tf"
---

# Roadmap: `agents install` nao registra o agente, e `by_agent` escreve sempre no primeiro da lista

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md -->
REQ: docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md

REQ aberta em **2026-08-29**, sem roadmap ate hoje. Reconfirmada como **AINDA VALIDA (verificado)**
na triagem de 2026-09-05 (linha 7), e medida **por fora** pelo consumidor externo em 2026-09-11
(**issue #320**), que acrescentou tres superficies novas da **mesma causa**.

O mecanismo ja esta decidido pelo KG na propria REQ (secao "Mecanismo decidido"). 🔴 **Nao reabrir a
decisao** — implementar o que esta escrito.

**Dependencia satisfeita:** a REQ irma (uniao de leitura + `agent_namespace_undeclared`) esta **Done**
e a regra existe nos 3 CLIs — verificado em 2026-09-11.

## Acceptance Criteria
<!-- Detalhe por ML nas waves abaixo. Fonte de verdade: AC1-AC15 da REQ. -->
- [ ] AC1-AC3, AC8 — `agents install` registra o agente em `agents:`, so em `by_agent`, preservando o resto do arquivo
- [ ] AC4, AC5, AC5b, AC10, AC11, AC12, AC14 — resolucao de agente: flag, heranca, erro na ambiguidade, `flat` intacto
- [ ] AC6 — `roadmap move` continua funcionando entre namespaces
- [ ] AC7 — paridade exata nos 3 CLIs
- [ ] AC13 — emissores de `trackfw req new` ensinam `--agent` em `by_agent` multi-agente
- [ ] AC15 — `consumer-smoke-by-agent` VERDE e `continue-on-error` removido no mesmo PR
- [ ] AC9 — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Derivacao (bloqueia tudo)
> Dependencias: nenhuma. **Nenhuma linha de implementacao nesta wave.**

### ML-0A — Derivar o que o handoff ainda presume
**Status:** ✅ Concluído — relatório em `docs/qualidade/2026-09-11-derivacao-by-agent-ml0a.md`
**Arquivos afetados:** nenhum de codigo. Entrega um relatorio.
**Acoes:**
1. **Sitio do `req new` do Go.** O relator do #320 declarou que **nao o localizou**. Localizar e
   escrever arquivo:linha. Ponto de partida: `internal/generators/req.go:30` diz ser o "ponto unico de
   decisao de caminho de ESCRITA (ADR-2026-09-03, D2/D4)" — **confirmar ou refutar** que e ele.
2. 🔴 **`by_agent` vale para `req_dir`, ou so para `roadmap_dir`?** `internal/validator/validator.go:1458`
   descreve `req_dir/<agente>/` como canonico em `by_agent`, e `pypi/trackfw/validator.py:808` usa
   `agents[0] if agents else "default"`. **Se for so roadmap por desenho, AC10 esta mal posto** e a
   wave 1 muda de forma. Responder com evidencia dos 3 runtimes.
3. **Mecanismo de derivacao do `move`.** Ler `internal/generators/roadmap.go:437` e
   `npm/src/generators/roadmap.js:267-268`, e achar o equivalente Python. Escrever a assinatura que a
   wave 1 vai **reusar**. 🔴 Se nao houver equivalente Python, dizer — nao inventar.
4. **Re-derivar a lista de emissores** de `trackfw req new`/`roadmap new` sem `--agent`. A lista da REQ
   e de 2026-09-11 e pode ter crescido. Comando escrito no relatorio.
5. **Threat model:** quem esvazia esta wave 0 sem quebrar regra escrita? E o contra-braco de cada AC:
   o que quebra quando regride para cada lado?
**Criterios de aceite:**
- [ ] Os 5 itens respondidos com evidencia (arquivo:linha ou saida de comando), nao asserção de uma linha
- [ ] 🔴 Zero linhas de implementacao neste ML
- [ ] Se o item 2 refutar AC10, o ML **para e reporta** em vez de seguir

**Gate da wave:**
```bash
# Falha fechado ate ML-0A responder. O relatorio do ML-0A substitui este comando
# pelo gate real derivado no item 5.
f="docs/qualidade/2026-09-11-derivacao-by-agent-ml0a.md"
test -f "$f"                                        || { echo "FALTANDO: $f" >&2; exit 1; }
grep -q "req.go:"                            "$f"   || { echo "FALTA item 1: sitio req.go" >&2; exit 1; }
grep -q "req_dir"                            "$f"   || { echo "FALTA item 2: by_agent/req_dir" >&2; exit 1; }
grep -qE "roadmap\.(go|js|py):[0-9]"         "$f"   || { echo "FALTA item 3: mecanismo do move" >&2; exit 1; }
grep -q "agentfiles.go"                      "$f"   || { echo "FALTA item 4: emissores" >&2; exit 1; }
grep -q "init_gen.py"                        "$f"   || { echo "FALTA item 4: emissores novos" >&2; exit 1; }
grep -qi "threat\|esvazia\|contra.bra"      "$f"   || { echo "FALTA item 5: threat model" >&2; exit 1; }
echo "ML-0A gate: OK"
```

---

## Wave 1 — Resolucao de agente (3 MLs em PARALELO)
> Dependencias: **ML-0A aprovado.** Os tres tocam arvores disjuntas (`internal/`, `npm/src/`,
> `pypi/trackfw/`) e rodam juntos. 🔴 Nenhum deles toca `.github/` nem os emissores — isso e a wave 3.

> **Contrato comum aos tres.** Cada ML entrega, **no seu runtime**:
> - `--agent <nome>` em **`req new` e `roadmap new`**, alimentando caminho **e** frontmatter a partir
>   do **mesmo valor** (AC4, AC10, AC12).
> - Sem flag: **um** namespace em `agents:` ⇒ usa aquele. **Varios** ⇒ **erro nomeando as opcoes**.
>   O `Agents[0]` silencioso deixa de existir (AC5).
> - `--agent` com valor **fora** de `agents:` **funciona** e cria o namespace, produzindo a violacao
>   `agent_namespace_undeclared` da REQ irma (AC5b).
> - `roadmap new --req <caminho>` **herda o agente da REQ**, 🔴 **reusando o mecanismo do `roadmap move`
>   identificado no ML-0A**. Derivacao nova reprova a auditoria (AC11).
> - `roadmap move` entre namespaces continua funcionando (AC6).
> - **Testes, no proprio ML, nas duas direcoes** (AC14):
>   - `agents: [alpha, beta]` + `--agent beta` ⇒ artefato em `beta/`, frontmatter `beta`;
>   - `agents: [alpha, beta]` **sem** flag ⇒ **erro**, e a mensagem **nomeia `alpha` e `beta`**;
>   - `agents: [alpha]` **sem** flag ⇒ **cria em `alpha/`, sem erro** — 🔴 contra-braco: guarda que so
>     reprova e indistinguivel de guarda que reprova sempre;
>   - `flat` ⇒ comportamento **inalterado**.
> - 🔴 **Regra Dura de Reconciliacao:** para cada teste novo, uma frase no relatorio dizendo qual
>   conclusao do proprio ML ele afirma.
> - 🔴 **Nao rodar nada em background.**

### ML-1A — Go
**Status:** ✅ Concluído — auditado E2E nos 3 binários
**Arquivos afetados:** `internal/generators/req.go`, `internal/generators/roadmap.go`,
`internal/commands/req.go`, `internal/commands/roadmap.go`, e os `*_test.go` correspondentes.
**Acoes:** o contrato comum acima, no runtime Go. Sitio do `req new` conforme derivado no ML-0A.
**Criterios de aceite:**
- [ ] Contrato comum inteiro, com os 4 cenarios de teste
- [ ] `go build ./...` e `go test ./...` verdes
- [ ] 🔴 Nenhum arquivo fora de `internal/` tocado

### ML-1B — Node
**Status:** 🔄 Em andamento
**Arquivos afetados:** `npm/src/generators/req.js`, `npm/src/generators/roadmap.js`,
`npm/src/commands/req.js`, `npm/src/commands/roadmap.js`, e os testes em `npm/tests/`.
**Acoes:** o contrato comum acima, no runtime Node.
**Criterios de aceite:**
- [ ] Contrato comum inteiro, com os 4 cenarios de teste
- [ ] Suite do Node verde
- [ ] 🔴 Nenhum arquivo fora de `npm/` tocado

### ML-1C — Python
**Status:** 🔄 Em andamento
**Arquivos afetados:** `pypi/trackfw/generators/req.py`, `pypi/trackfw/generators/roadmap.py`,
`pypi/trackfw/commands/req.py`, `pypi/trackfw/commands/roadmap.py`, e os testes em `pypi/tests/`.
**Acoes:** o contrato comum acima, no runtime Python. **Atencao:** o `--agent` do `roadmap new` **ja
existe** aqui (`pypi/trackfw/commands/roadmap.py:211`) — estender para `req new` e alinhar o
comportamento sem-flag, nao reescrever o que ja funciona.
**Criterios de aceite:**
- [x] Contrato comum inteiro, com os 4 cenarios de teste
- [x] Suite do Python verde (1714 passed)
- [x] 🔴 Nenhum arquivo fora de `pypi/` tocado

---

## Wave 2 — `agents install` registra na governanca (3 MLs em PARALELO)
> Dependencias: **wave 1 auditada.** Arvores disjuntas, mesma regra de nao-sobreposicao.

> **Contrato comum.** No proprio runtime (AC1, AC2, AC3, AC8):
> - `trackfw agents install` num projeto `by_agent` **registra o agente em `agents:`** do
>   `trackfw.yaml` se ainda nao estiver la. **Idempotente**: instalar duas vezes nao duplica.
> - Em `flat`, **nao** escreve a chave — ali ela nao tem funcao.
> - Preserva ordem e formatacao do resto do arquivo: **verificavel por diff**, so `agents:` muda.
> - 🔴 **Falsificacao nas duas direcoes** (AC8): agente instalado **aparece** em `agents:`; instalar em
>   `flat` **nao** cria a chave.
> - 🔴 Regra Dura de Reconciliacao, uma frase por teste novo. **Nada em background.**

### ML-2A — Go · ### ML-2B — Node · ### ML-2C — Python
**Status:** ⬜ Pendente (os tres)
**Arquivos afetados:** o subsistema de integracoes/agents e o escritor de config de cada runtime,
mais os testes. 🔴 Cada ML fica **dentro da sua arvore**.
**Criterios de aceite (cada um):**
- [ ] Contrato comum inteiro, com a falsificacao nas duas direcoes
- [ ] Build e suite do runtime verdes
- [ ] 🔴 Nenhum arquivo fora da propria arvore tocado

---

## Wave 3 — Consequencias (SEQUENCIAL)
> Dependencias: waves 1 e 2 auditadas. 🔴 **3A e 3B sao sequenciais entre si** — o 3B mede o efeito do 3A.

### ML-3A — Os emissores param de ensinar um comando que falha (AC13)
**Status:** ⬜ Pendente
**Arquivos afetados:** os derivados no **item 4 do ML-0A**. A lista de 2026-09-11 era:
`internal/generators/agentfiles.go:59`, `internal/generators/claudemd.go:57-58`,
`internal/generators/scaffold.go:263`, `npm/src/generators/init.js:524,691-692,899`,
`npm/src/push/runner.js:130`, `npm/src/ship/runner.js:502`, `npm/src/commands/branch.js:33`,
`npm/src/commands/commit.js:37`, `pypi/trackfw/push/runner.py:148`, `pypi/trackfw/validator.py:1967`.
🔴 **Usar a lista re-derivada do ML-0A: 33 ARQUIVOS, nao 10 e nao 20.** E 🔴 **6 deles sao TESTES que
afirmam o texto dos emissores** (`npm/tests/{push,serve_chain,ship}.test.js`,
`pypi/tests/test_{push,serve_chain,ship}.py`) — mudam **em lockstep**, no mesmo ML. Classificar os 33 em
{orientacao exibida · teste que a afirma · comentario} e **justificar por escrito cada exclusao**;
contagem sozinha nao autoriza excluir nada.
**Acoes:** onde o texto orienta `trackfw req new "title"`, ensinar `--agent` quando o projeto for
`by_agent` com 2+ agentes. 🔴 **Um unico dono** — estes arquivos atravessam as tres arvores e nao
podem ser editados em paralelo com nada.
**Criterios de aceite:**
- [ ] Todo emissor da lista re-derivada coberto, ou a exclusao **justificada por escrito**
- [ ] Gate de paridade dos 3 CLIs verde
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (AC9)

### ML-3B-a — 🔴 O gate que "nasceu vermelho detectando o #320" NUNCA rodou o Go (issue #328)
**Status:** ⬜ Pendente · **Bloqueia o ML-3B-b**
**Arquivos afetados:** `scripts/check-consumer-smoke-by-agent.sh`, `.github/workflows/quality.yml`.
🔴 **Só esses dois.**

**Reportado pelo consumidor externo na issue #328 e REPRODUZIDO pelo arquiteto em 2026-09-11:**

```
guarda (da raiz):              ACHOU
uso apos cd "$PROJECT":        rc=127
uso apos cd, caminho absoluto: rc=0
```

`quality.yml:1280` passa `GO_BIN=bin/trackfw` — **relativo**. O default do script (linha 27) é
`$ROOT_DIR/bin/trackfw`, absoluto. A guarda da linha 98 roda **na raiz do repo** e acha; **todas** as
chamadas do Go (`153, 181, 188, 235, 239`) rodam depois de `cd "$PROJECT"` e dão **127**. `NODE_CLI` e
`PY_ROOT` partem de `$ROOT_DIR` e escapam.

🔴 **A guarda e o uso olham lugares diferentes — por isso a guarda passa.** É a nona instância da
classe de 2026-09-10, agora numa variante nova: não é "não consegui procurar", é **"procurei noutro
lugar"**.

#### O que isto custa, e é mais do que 5 falhas

1. **O Go não tem cobertura nenhuma no smoke.** Os "2 roadmaps em alpha" que supostamente detectam o
   #320 são do Node e do Python.
2. 🔴 **A linha 188 é a ÚNICA chamada `--req` do script inteiro — e é a do Go.** Ou seja: a metade do
   #320 que diz *"`roadmap new --req` ignora o agente da REQ"* — a que consumiu a Wave 1 inteira,
   incluindo o corretivo do ML-1A-fix — **não é exercitada em runtime nenhum**.
3. **Corrigir o #320 não deixaria o job verde**, e quem fosse remover o `continue-on-error` (nós,
   no ML-3B-b) encontraria vermelho alheio e não saberia disso.
4. O `--self-test` não pega: por desenho ele valida a lógica de detecção **sem invocar os CLIs reais**.

⚠️ **Nós criamos este gate e declaramos que ele nascia vermelho detectando o #320.** Ele nascia
vermelho por **seis** motivos, dos quais **um** era o #320 — e a metade do #320 que mais nos custou não
era medida por ele. **Escrevemos "nasce vermelho detectando X" sem verificar o que o vermelho dizia.**

**Acoes:**
1. Tornar `GO_BIN` absoluto **no próprio script**, logo após a guarda, para que guarda e uso vejam o
   mesmo caminho. Sugestão medida pelo relator em 3 formas de entrada (`bin/trackfw`, `./bin/trackfw`,
   absoluto): `GO_BIN="$(cd "$(dirname "$GO_BIN")" && pwd)/$(basename "$GO_BIN")"`.
   🔴 Passar `GO_BIN="$PWD/bin/trackfw"` no workflow **não** basta — a guarda continuaria sem garantir
   o que o uso precisa. Corrigir no script; opcionalmente **também** no workflow.
2. 🔴 **`--req` passa a ser exercitado nos 3 runtimes**, não só no Go. É o buraco que o item 2 acima
   expõe.
3. 🔴 **O `validate (Go)` da linha 235 termina em `|| true` e falha em silêncio.** Mesma classe. Decidir
   e escrever: ou ele conta, ou o motivo de não contar fica no script.
4. **Derivar** se outros gates recebem caminho relativo por env e o usam depois de `cd`. Comando escrito.
**Criterios de aceite:**
- [ ] 🔴 **Falsificação:** com o binário Go removido, o gate **reprova nomeando o Go** — hoje ele
      reprova nomeando o `#320`
- [ ] `--req` exercitado nos 3 runtimes, com asserção de que o roadmap nasce no agente da REQ
- [ ] Decisão do item 3 escrita no script
- [ ] Varredura do item 4 com comando escrito
- [ ] Fecha a issue **#328**

### ML-3B-b — 🔴 `consumer-smoke-by-agent` VERDE e `continue-on-error` REMOVIDO (AC15)
**Status:** ⬜ Pendente · **Dependência: ML-3B-a**
**Arquivos afetados:** o workflow que define `consumer-smoke-by-agent` (introduzido no PR #326).
**Acoes:**
1. Confirmar que o job passa a **VERDE** com as waves 1-3A aplicadas.
2. **Remover o `continue-on-error: true`.** Foi declarado **temporario** no PR #326.
3. Se o job exercita `req new` sem `--agent` com 2 agentes, ele agora **recebe o erro de ambiguidade
   por desenho** — ajustar o smoke para passar `--agent`, e **manter um cenario que prova o erro**.
**Criterios de aceite:**
- [ ] `continue-on-error` **ausente** do job — verificavel por `grep`
- [ ] Job verde no CI
- [ ] 🔴 Existe cenario no smoke que **reprova** se a resolucao de agente regredir
- [ ] 🔴 **Intencao declarada nao e gate.** Se o `continue-on-error` sobreviver a este ML, o ML **nao
      esta concluido** — "alguem lembra depois" e a classe que nos custou um dia inteiro

---

## Notas de governanca

- 🔴 **Esta REQ absorveu o #320 por `Regra Dura de Causa Raiz`.** A
  `REQ-2026-09-11-by-agent-req-new-...` foi aberta por engano antes de eu encontrar esta, e esta
  **superseded**. Mesma causa ⇒ mesma REQ ⇒ **mesmo PR**.
- A decisao de mecanismo e do KG, 2026-08-29, e esta na REQ. **Nao reabrir.**

---

## 🔴 Barreira da Wave 1 — 2026-09-11 — DOIS bloqueios medidos pelo arquiteto

### B1 — Go (ML-1A): AC11 **não entregue**, e o teste afirma outra coisa

Medido com binário recém-compilado, projeto descartável `agents: [alpha, beta]`:

```
$ trackfw req new "t" --agent beta
created docs/req/beta/REQ-2026-09-11-t.md

$ trackfw roadmap new "rm" --req docs/req/beta/REQ-2026-09-11-t.md
Error: by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one
rc = 1 · roadmaps criados: NENHUM
```

**O AC11 diz:** `roadmap new --req <caminho>` **herda o agente da REQ**. O binário **erra**.

🔴 **E o ML escreveu, no próprio teste, que não ia entregar:**

```go
// Nota: NewRoadmapFromContent com --req direto exige --agent explícito (sem herança).
// A herança é exclusiva do caminho --from-req (NewRoadmapFromREQ).
```

O `TestRoadmapFromREQ_InheritsAgentFromREQPath` **passa** — porque exercita `NewRoadmapFromREQ`
(`--from-req`), que **não é o caminho que o AC11 nomeia**. É a `Regra Dura de Reconciliação` no seu
caso exato: **teste verde afirmando conclusão diferente da do AC**, com a divergência declarada em
comentário e ninguém confrontando.

**Causa provável:** em `NewRoadmapFromContent` (`internal/generators/roadmap.go:193`) a guarda
`ResolveWriteAgent` roda **antes** de qualquer derivação a partir de `content.REQPath`. Em
`NewRoadmapFromREQ` (`:298`) a ordem está certa — `agentFromPath` primeiro, `ResolveWriteAgent`
depois. **Os dois caminhos do mesmo comando têm ordens diferentes.**

⚠️ **Também não pegou porque os testes unitários chamam o gerador direto**, pulando a camada de
comando. O E2E com binário real é o que separou.

### B2 — Node (ML-1B): `squad:` no frontmatter da **REQ** — não autorizado, e quebra paridade

O ML-1B acrescentou `squad: "<agente>"` ao frontmatter da REQ (`npm/src/generators/req.js:300,308`),
e ele mesmo avisou que `scripts/check-artifact-parity.sh` faz **diff byte-a-byte** entre os 3 CLIs.
Medido: **Go não tem, Python não tem, Node tem.** O gate vai reprovar.

**Decisão do arquiteto: REVERTER no Node. A REQ NÃO ganha `squad:`.**

🔴 O agente de uma REQ **já está no caminho** (`req_dir/<agente>/`). Acrescentar `squad:` cria uma
**segunda fonte de verdade que pode divergir da pasta** — que é exatamente a classe de defeito que esta
REQ existe para fechar (`agents:` virou fotografia e divergiu do disco). Uma REQ movida entre
namespaces passaria a mentir no frontmatter.

O "um valor, dois efeitos" do AC4 vale para o **roadmap**, cujo frontmatter **já tem** `squad:`. O
template da REQ nunca teve, e acrescentar chave ao schema é decisão de template — não efeito colateral
de um ML de resolução de agente.

### Consequência de governança

A Wave 2 **não é liberada** até B1 e B2 fecharem. Os dois são microlotes corretivos na wave vigente,
não REQ nova: **mesma causa, mesma REQ, mesmo PR.**

### ✅ Barreira da Wave 1 — LEVANTADA em 2026-09-11

B1 e B2 fechados. Auditoria do arquiteto **contra os 3 binários construídos**, projeto descartável,
não contra relatório:

```
C1  roadmap new --req <REQ em beta/>          GO rc=0 beta/   NODE rc=0 beta/   PY rc=0 beta/
C2  1 agente, sem flag  (contra-braço)        GO rc=0 alpha/  NODE rc=0 alpha/  PY rc=0 alpha/
C3  2 agentes, sem flag (braço de erro)       GO "alpha, beta"  NODE idem  PY idem
```

`scripts/check-artifact-parity.sh` → verde (9 tipos × 3 runtimes). `squad:` ausente da REQ nos três.
O comentário que declarava a não-entrega do AC11 foi removido do teste do Go.

**Wave 2 liberada.**

---

## 🔴 A barreira reabriu — `make quality` VERMELHO (2026-09-11)

A barreira tinha sido levantada com base em **três cenários E2E verdes nos 3 binários**. O
`make quality` reprovou depois:

```
artifact parity cycle failed: go/by_agent — .trackfw-log não registrou backlog → analyzing
```

### B3 — Go: derivação de caminho executada DEPOIS de mover o arquivo

```
obtido:    2026-09-11 18:43  /ROADMAP-...-teste-log.md    backlog → analyzing
esperado:  2026-09-11 18:43  alpha/ROADMAP-...-teste-log.md  backlog → analyzing
```

`internal/generators/roadmap.go:581` chama `agentFromPath(cfg.RoadmapDir, src)` **depois** do
`os.Rename(src, dst)` da linha ~563. `src` já não existe, e a função resolve symlink só no que existe:
`absRoot` vira `/private/var/...`, `absFile` fica `/var/...`, o `Rel` devolve `".."`, e o guard —
correto — devolve `""`.

A `main` usava a variável `agent` **computada antes do rename**. 🔴 **A extração trocou uma variável já
calculada por uma derivação nova executada tarde demais.** O `MoveRoadmap` já tem essa variável na
linha ~541; basta reusá-la.

### O que isto ensina sobre a auditoria — e é a parte que vale

🔴 **Meu E2E de três cenários passou e a regressão estava lá.** Os três mediam **onde o artefato foi
parar**; nenhum mediu o **efeito colateral** (o registro da transição). Os 16 testes unitários do ML
também não — pela mesma razão.

**Extrair uma expressão inline para função nomeada muda QUANDO ela é avaliada**, e efeito colateral sem
dono é o que fica sem cobertura. `make quality` pegou porque exercita o **ciclo**, não o ponto.

**Corretivo:** ML-1A-fix2. **Wave 2 continua bloqueada.**

---

## Achado lateral — medido, causa DIFERENTE, não entra nesta REQ

`roadmap new "<título>" --req <caminho>` — o **Go ignora o título posicional**; Node e Python o usam:

```
GO    → ROADMAP-2026-09-11-2026-01-01-pagamentos.md   (derivou do nome da REQ)
NODE  → ROADMAP-2026-09-11-titulo-escolhido.md
PY    → ROADMAP-2026-09-11-titulo-escolhido.md
```

**Causa:** `internal/commands/roadmap.go:30` declara `Args: cobra.MaximumNArgs(1)`, mas **`args[0]`
nunca é atribuído a `title`** — a variável só é alimentada por flag. O `if title == ""` da linha 44
sempre dispara.

**Medido como causa diferente, não presumido:**
- reproduz em **`flat`**, sem nenhum agente configurado → **independente de `by_agent`**;
- o código idêntico está em `origin/main` → **pré-existente**, não regressão desta wave;
- teste da triagem por mecanismo: corrigir a resolução de agente **não fecha** este defeito.

Por `Regra Dura de Causa Raiz`, causa diferente autoriza REQ própria — e a diferença de mecanismo fica
escrita acima. ✅ **Decisão do KG, 2026-09-11: "vamos de ML aqui mesmo."** Vira o **ML-1D** desta wave, com a
diferença de mecanismo escrita acima — não REQ nova.

🔴 **A regra continua valendo; o que mudou foi o custo relativo.** Causa diferente **autoriza** REQ
própria, não **obriga**. Com ~3 linhas, num arquivo já aberto neste PR, medido e reproduzido, mandar
para uma fila de 36 REQs abertas seria o defeito que a análise de 2026-09-11 nomeou: **"registrado"
não é "corrigido"**. O ônus de escrever a diferença de mecanismo foi pago acima; ele é o que permite
decidir, e não o que obriga a separar.

### ML-1D — Go: `roadmap new "<titulo>" --req` ignora o titulo posicional
**Status:** ✅ Concluído — os dois braços verificados pelo arquiteto com o binário:
`"titulo escolhido"` → `ROADMAP-...-titulo-escolhido.md`; **sem** título → fallback pelo nome da REQ
sobrevive. Varredura reconferida com régua própria (nenhum arquivo de `internal/commands/` declara
`Args:` com posicional opcional e nunca lê `args[`). ⚠️ Resíduo declarado: a varredura cobre
`MaximumNArgs` por sítio e `Args:` por arquivo; um arquivo com **dois** comandos, um lendo `args[` e
outro não, escaparia das duas réguas.
**Arquivos afetados:** `internal/commands/roadmap.go` e o teste correspondente. 🔴 **So `internal/`.**
**Medicao (arquiteto, 2026-09-11):** ver "Achado lateral" acima. Reproduz em `flat`, existe em
`origin/main`, e **nao fecha** com a correcao de resolucao de agente — causa diferente, mesmo PR por
decisao do KG.
**Acoes:**
1. `internal/commands/roadmap.go:30` declara `Args: cobra.MaximumNArgs(1)`, mas **`args[0]` nunca e
   atribuido a `title`**. Atribuir quando houver argumento posicional, antes do `if title == ""` da
   linha ~44. O fallback que deriva do nome da REQ **fica**, para quando nao houver titulo.
2. 🔴 **Derivar se o mesmo esquecimento existe em OUTROS comandos do Go** que declarem
   `cobra.*NArgs` e leiam de variavel de flag. **Escrever o comando** usado. Mesma causa ⇒ mesmo ML.
**Criterios de aceite:**
- [ ] `roadmap new "titulo escolhido" --req <REQ>` produz `ROADMAP-<data>-titulo-escolhido.md` nos
      **3 runtimes** — colar a saida dos tres binarios lado a lado
- [ ] **Contra-braco:** **sem** titulo posicional, `--req` continua derivando do nome da REQ
- [ ] Varredura do item 2 respondida com comando escrito
- [ ] `go build ./... && go vet ./... && go test ./...` verdes e
      `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0
- [ ] Frase de reconciliacao por teste novo

### ML-1A-fix2 — `.trackfw-log` sem o segmento do agente
**Status:** ✅ Concluído — log volta a gravar `alpha/ROADMAP-*.md`, verificado pelo arquiteto com binário recompilado

---

## ⚠️ Achado a triar — gate de mutação possivelmente VÁCUO

Com o `check-artifact-parity.sh` verde, o `make quality` passou a alcançar um cenário que antes nem
rodava:

```
direction-b2/node/detects-symlink-regression — corrupted binary did not escape through the symlink
  → "checagem vácua" (a própria mensagem do gate)
```

`scripts/check-agent-namespace-union.sh:907-922` é um **teste de mutação**: corrompe o binário Node
para seguir symlink e exige que ele **escape** — provando que a checagem detectaria a regressão. Se o
binário mutado **não** escapa, o cenário não prova nada.

**Medido:** o diff desta branch em `npm/src/validator/index.js` toca **apenas** `resolveAgentForWrite`
e `reqWriteDir` (21 linhas) — **nada de travessia de diretório nem de symlink**. A linha que a mutação
alveja (`statSync(...).isDirectory()`) não foi tocada.

🔴 **É a classe do `#309`, e a mais desconfortável:** gate correto no dia 1 que vira vácuo no dia 30
por mudança adjacente, **e nada percebe** — porque um gate vácuo passa. Este só apareceu porque outra
falha deixou de mascará-lo.

⚠️ **Não confirmado como pré-existente pelo arquiteto** — o ML anterior afirmou que era, mas usando
como evidência que o arquivo "não foi alterado nesta branch", o que **é falso** (`9d042b8e` o alterou).
A conclusão pode estar certa e a evidência errada. **Triar antes de afirmar qualquer coisa.**

### 🔴 O gate vácuo NÃO é pré-existente — é NOSSO. Medido em A/B

O ML anterior afirmou "pré-existente", com a evidência de que `npm/src/validator/index.js` não fora
alterado nesta branch — **evidência falsa**, o commit `9d042b8e` o alterou. Medi em A/B, com árvore
`npm/` completa de cada lado e `node_modules` ligado:

```
MAIN    → ✓ moved ROADMAP-leak.md → docs/roadmaps/evil/done     ESCAPOU  → o gate detectaria
BRANCH  → ✓ moved ROADMAP-leak.md → docs/roadmaps/alice/done    contido  → checagem VÁCUA
```

**Causa:** o `agentFromPath` do ML-1B resolve symlink (`realpathSync`) e depois exige que o caminho
fique **dentro** do `roadmapDir`. Pelo symlink `evil → /fora`, o relativo começa com `..`, o guard
devolve `""`, e o `move` cai em `alice`. **A fuga não acontece mais.**

### Por que isto NÃO é "o gate está errado"

🔴 **Defesa em profundidade quebra teste de mutação.** O cenário `direction-b2` mutila **um** guard (o
`.filter(e => e.isDirectory())` do AC12) e conclui, pela fuga, que aquele guard era o que segurava.
Agora existe um **segundo** guard, independente, que **absorve a mutação** — e o cenário perde o poder
de falar sobre o primeiro.

A premissa do gate ("só o filtro de `isDirectory` impede a fuga") virou **falsa**. O gate está
medindo certo e concluindo sobre um mundo que mudou.

⚠️ **E é ambíguo se `alice/done` é bom.** O roadmap foi **contido**, mas foi parar num namespace que
não é o dele. Contenção não é o mesmo que correção — o ML precisa decidir e escrever qual dos dois
comportamentos é o contrato.

### 🔴 O contrato — decisão do arquiteto, NÃO do implementador

O item 2 do rascunho anterior mandava o ML "decidir e escrever" o contrato. **Errado.** A decisão já
existe, e é do KG, na própria REQ (2026-08-29):

> *"controle que não reconhece **rejeita e avisa**, em vez de adivinhar"*

Contenção silenciosa em `alice/` **é o adivinhar**: o roadmap vai parar num namespace que não é o dele
e nada conta a ninguém. É a mesma classe que esta REQ inteira existe para fechar.

**Contrato:** em `MoveRoadmap` modo `by_agent`, `agentFromPath` devolver `""` é **erro explícito**,
não fallback. Entregar decisão de fronteira de segurança a um implementador é como se obtém uma
terceira resposta.

### ML-1E-a — o contrato nos 3 runtimes, e a paridade do teste de log
**Status:** 🔄 Em andamento — aguarda auditoria Zeus
**Arquivos afetados:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py` e os testes dos três. 🔴 **Não toque em `scripts/`** — é o ML-1E-b.
**Acoes:**
1. **O contrato:** em `by_agent`, agente derivado vazio em `MoveRoadmap` ⇒ **erro nomeando o caminho
   recusado**. Nada de cair em `agents[0]`, nada de string vazia virando caminho.
2. 🔴 **Reproduza o A/B você mesmo** (`git archive origin/main npm | tar -x`, `node_modules` ligado por
   symlink, mutação do literal do `direction-b2`). Não aceite a medição de segunda mão.
3. 🔴 **Rode o A/B também em Go e Python.** O cenário `direction-b2/python` (linha ~934) **passa** hoje
   — ou o Python não tem o segundo guard, ou o cenário difere. **Isso discrimina.** O Go está
   **excluído** do cenário (ver comentário do cabeçalho na linha 894) — **leia o motivo antes** de
   assumir que ele entra no escopo.
4. 🔴 **Paridade do teste de regressão:** o `TestMoveRoadmap_ByAgent_LogPrefixHasAgent` existe **só no
   Go**. Mesma extração, mesmo defeito, três runtimes — **a regra dura de paridade vale para a guarda,
   não só para o comportamento.** Portar para Node e Python.
**Criterios de aceite:**
- [ ] Contrato do item 1 nos 3 runtimes, com mensagem que **nomeia o caminho recusado**
- [ ] **Contra-braço:** `move` legítimo entre namespaces continua funcionando nos 3
- [ ] A/B dos 3 runtimes colado no relatório
- [ ] Teste de prefixo do `.trackfw-log` nos 3 runtimes
- [ ] `go test ./...`, suíte Node e `pytest pypi/tests/` verdes
- [ ] Frase de reconciliação por teste novo

### ML-1E-b — restaurar o poder de falsificação do `direction-b2`
**Status:** ⬜ Pendente · **Dependência: ML-1E-a auditado** (b mede a)
**Arquivos afetados:** `scripts/check-agent-namespace-union.sh`. 🔴 **Só `scripts/`.**
**Acoes:**
1. Reescrever o cenário contra o **novo contrato**: com symlink para fora, o binário **não corrompido**
   deve **falhar explicitamente**. 🔴 Não basta deletar o cenário nem afrouxar a asserção.
2. Para continuar provando o guard do AC12 (`.filter(e => e.isDirectory())`), o cenário precisa
   **isolá-lo** — mutar os **dois** guards, já que agora há defesa em profundidade e um absorve a
   mutação do outro.
3. **Derivar** se outros cenários deste script sofrem do mesmo mascaramento. Comando escrito.
**Criterios de aceite:**
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0
- [ ] 🔴 O cenário **reprova** quando o guard alvo é revertido — falsificação **provada**, não afirmada
- [ ] Varredura do item 3 com comando escrito

### ML-1E-a3 — os 7 sítios remanescentes da classe #315 + gate da classe
**Status:** ✅ Concluído — auditado pelo arquiteto **por execução própria**

```
RC COM symlink cru plantado = 1     ← gate acusa
RC SEM symlink cru          = 0     ← e nao acusa sempre
varredura final             : 290 arquivos, zero sitios desguardados
```

🔴 **Falsifiquei o gate eu mesmo**, plantando `(tmp_path / "link").symlink_to(...)` num teste e
removendo depois. Não aceitei o `--self-test 3/3` do relatório: **self-test prova que o gate roda, não
que ele pega o caso real.** Foi essa exata distinção que deixou o `check-serve-api-file-security.sh`
existir sem alvo hoje.

**Contra-braço medido:** neste macOS os testes **executam de verdade** — `--- PASS` nos três Go, 160
passed no Python, Node verde, **zero SKIP**. Guarda que vira skip permanente é pior que o defeito
original, e essa é a forma mais provável de "corrigir" a classe #315 errado.

**Gate ligado ao `Makefile`** (linha 74) — 🔴 gate órfão foi a classe que reapareceu **duas vezes hoje**.

**Dois antipadrões distintos fechados, e eles não são a mesma coisa:**

- `regularfile_test.go` pulava **por plataforma** (`runtime.GOOS == "windows"`), abandonando cobertura
  que existiria com Developer Mode. Virou guarda **por capacidade**: tenta, e só pula se o privilégio
  faltar.
- `generators.test.js` e `test_validator.py` engoliam **todo** erro (`catch (_) {}`). Agora
  discriminam: `EPERM`/`EACCES`/`winerror 1314` ⇒ skip; **qualquer outro erro ⇒ falha**.
