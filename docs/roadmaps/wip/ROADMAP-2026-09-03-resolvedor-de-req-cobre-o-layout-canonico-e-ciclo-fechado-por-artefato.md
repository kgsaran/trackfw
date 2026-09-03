---
status: wip
date: 2026-09-03
squad: apolo-tf
req: "docs/req/REQ-2026-08-30-req-new-grava-flat-mas-resolvereqfiles-procura-namespaced-por-estado-e-as-regras-de-referencia-ficam-vacuas-em-by-agent.md"
adr: "docs/adr/ADR-2026-09-03-layout-canonico-de-req-em-by-agent-e-o-invariante-de-que-req-nao-tem-dimensao-de-estado.md"
---

# Roadmap: Resolvedor de REQ cobre o layout canônico, e ciclo fechado por artefato

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-30-req-new-grava-flat-mas-resolvereqfiles-procura-namespaced-por-estado-e-as-regras-de-referencia-ficam-vacuas-em-by-agent.md

ADR: docs/adr/ADR-2026-09-03-layout-canonico-de-req-em-by-agent-e-o-invariante-de-que-req-nao-tem-dimensao-de-estado.md

## Diagnóstico

Em todo projeto `by_agent`, as regras que consomem REQ ficam **vácuas** — passam sempre, sem olhar
nada. Medido: mesma REQ, mesma referência quebrada, `flat` → **2 violações**, `by_agent` → **0**.

O escritor grava flat, o leitor procura `<agente>/<estado>/`, e o campo usa `<agente>/`. **Nenhum dos
três concorda.**

🔴 **Correção de custo, feita na ADR:** a leitura de backlog afirmou que "a lógica correta já existe
— é fiação, não lógica nova", apontando `listREQFiles` (`internal/generators/req.go:119-152`).
**Verificado e parcialmente falso:** ele cobre flat, por-estado e `<agente>/<estado>`, e **não cobre
`<agente>/*.md`** — justamente o canônico da ADR. **Falta um caso.**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Acceptance Criteria

- [ ] O resolvedor de leitura cobre os 4 layouts, em união, nos 3 CLIs
- [ ] `req new` grava no canônico da ADR (`req_dir/<agente>/*.md` em `by_agent`)
- [ ] 🔴 Zero regras enxergando zero REQs em `by_agent` — forma mensurável, não impressão
- [ ] 🔴 Compatibilidade: REQ em qualquer layout continua encontrada; nenhum arquivo migrado
- [ ] Ciclo fechado por artefato nos 3 CLIs, em `flat` **e** `by_agent`
- [ ] `make quality` verde e CI verde

## Wave 1 — O resolvedor
> Dependências: nenhuma.

### ML-1A — Resolvedor de REQ em união, e `req new` no canônico
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected — os 3 stacks, regra dura de paridade sem exceção:**
`internal/validator/validator.go` (`resolveREQFiles`), `internal/generators/req.go` (`listREQFiles`,
`NewREQ`), `npm/src/validator/index.js`, `npm/src/generators/req.js`,
`pypi/trackfw/validator.py`, `pypi/trackfw/generators/req.py`, `docs/cli-parity.md`

⚠️ **`npm/src/validator/index.js` é classificado como BINÁRIO pelo `file`** — `grep` sem `-a` o pula
**em silêncio**. Duas REQs deste repositório têm premissa falsa por causa disso. Use sempre `grep -a`.

**Ações:**
1. Leitura em **união** dos 4 layouts (ADR D3). O caso ausente é `req_dir/<agente>/*.md`.
2. `req new` grava no canônico (ADR D2). Em `flat`, nada muda.
3. 🔴 **Um único ponto decide o caminho, consumido pelos dois lados** (ADR D4). Se hoje há duas
   noções de layout no mesmo runtime, unificar é parte do ML — é a causa das três ocorrências.

🔴 **`ResolveAgentNamespaces` já lê nome de agente do disco** e há nota registrando que
metacaracteres de glob no nome corrompiam contagem em silêncio (`ListMDFiles` em vez de `Glob`).
Ao acrescentar o 4º caso, **não reintroduza `Glob` sobre nome vindo do disco.**

**Critérios de aceite:**
- [x] A fixture do relato produz **2 violações** em `by_agent`, iguais às de `flat`
- [x] 🔴 **Forma mensurável do "não fica mais vácuo":** contar, num projeto `by_agent`, quantas
      regras enxergam **zero** REQs. Tem de ser **zero regras**. Lista a confirmar, não rederivar:
      `ref_targets_exist`, `req_has_adr`, `req_has_roadmap`, `blocked_by_draft_adr`,
      `adr_accepted_when_req_done`, traceid
- [x] 🔴 **Compatibilidade falsificada:** REQ em `req_dir/*.md` num projeto `by_agent` **continua
      encontrada**. Nenhum arquivo movido — `git status` limpo quanto a renomeações
- [x] 🔴 **Falsificação na direção oposta:** removendo o 4º caso do resolvedor, a fixture volta a
      dar **0 violações**. Um teste que passa nas duas árvores não mede nada
- [x] Paridade: os 3 CLIs dão o **mesmo** número de violações sobre a mesma fixture
- [x] `make quality` verde


**Evidência de aceite — auditoria do arquiteto (2026-09-03), reproduzida de forma independente:**

```
teste de uniao dos 4 layouts (Go)          -> PASS
SABOTADO (comentado o caso <agente>/*.md)  -> FAIL   <- discrimina
restaurado                                 -> PASS
git status: 0 linhas "R"                   <- nenhum arquivo movido
make quality QUALITY_EXIT=0, zero FAIL · validate exit 0
```

🔴 **A ADR subestimou o defeito, e o agente corrigiu com número.** Eu escrevi que faltava **um** caso
em `listREQFiles`. Medido antes de editar, nos 3 CLIs, sobre a fixture do relato:

```
flat req_dir/*.md            2 violacoes
by_agent <agente>/<estado>/  2 violacoes
os outros QUATRO layouts     0            <- vacuos
```

**Quatro dos seis eram vácuos, não um.** E a razão do meu erro: **`listREQFiles` não é a função que
as regras usam** — o resolvedor delas era `if/else`, não união. Inventário real: **9 implementações
de leitura** e 3 de escrita. E o `traceid` recebia um **diretório**, não a lista resolvida (Go e
Python): corrigir só o resolvedor não o alcançaria.

🔴 **Achado não previsto — a união colide com o namespace vindo do disco.** Como `agents:` é unido ao
disco, `req_dir/backlog/` também é lido como se fosse agente, e o caso 3 colide com o caso 2. **Sem
dedup por caminho normalizado, toda REQ por-estado contaria em dobro.** Coberto por teste nos 3
runtimes.

🔴 **A métrica óbvia era fraca, e o agente trocou.** "A regra apareceu na saída" dá **6/6 mesmo na
árvore sabotada**, porque `ref_targets_exist` e `traceid` disparam pelo lado do *roadmap*. A métrica
usada passou a ser *"a violação nomeia um `REQ-*.md` em `file` ou `message`"* — e aí a sabotagem cai
para **1/6** (Go, Python) e 2/6 (Node). Sem essa troca, a medição teria mentido a favor.

**Por que durou tanto:** **nenhum teste existente codificava o comportamento antigo** — as 3 suítes
passaram sem edição. O defeito nunca foi testado em nenhuma direção.

**Resíduos declarados, não feitos:** `npm/src/validator/traceid.js` segue fora do ponto único
(varredura recursiva, superconjunto, nunca vácuo — é por isso que o Node cai para 2/6 e não 1/6);
`trackfw sync` hardcoda `docs/req` nos 3 CLIs; `req move` ainda move para `<agente>/<estado>/`,
contra o invariante D1 da ADR. Os três viram REQ própria.

**Consequência que não é só de `by_agent`:** o caso por-estado passou a ser lido
incondicionalmente, então projeto **flat** com árvore legada `req_dir/<estado>/` também tem suas REQs
olhadas agora.

**Comandos de validação:** `make quality`, e a fixture do relato executada nos 3 CLIs.

## Wave 2 — A rede que impede a quarta ocorrência
> Dependências: Wave 1.

### ML-2A — Teste de ciclo fechado por artefato
**Status:** ✅ Concluído
**Agente:** `artemis-tf`

**Por que é microlote próprio, com barreira própria:** é a AC que impede a **quarta** ocorrência do
padrão *gerador e verificador discordando do contrato* — antes foram o cabeçalho de aceite e o
vocabulário de status. Como caixinha dentro do ML de correção, seria a primeira coisa cortada sob
pressão de escopo.

**Ações:** para cada artefato — `req new`, `adr new`, `note new` — criar pelo **gerador** e provar
que o **verificador enxerga**. Nos **3 CLIs**, em `flat` **e** `by_agent`. Mínimo 6 combinações por
artefato.

**Critérios de aceite:**
- [x] Ciclo fechado verde para os 3 artefatos × 3 CLIs × 2 layouts
- [x] 🔴 **Falsificação:** sabotando o resolvedor, o ciclo fechado **reprova**. Prove as duas direções
- [x] 🔴 O teste roda o **CLI**, não o módulo com mock — o defeito do `context` do Node sobreviveu
      desde a origem exatamente por o teste não executar o binário
- [x] Cenário em `scripts/check-gates-falsify.sh` se virar gate
- [x] `make quality` verde


**Evidência de aceite — auditoria do arquiteto (2026-09-03), reproduzida de forma independente:**

```
scripts/check-artifact-closed-cycle.sh   -> rc=0, 18 combinacoes, 36 assercoes
SABOTADO (caso <agente>/*.md fora do Go) -> rc=1   <- discrimina
restaurado                               -> rc=0
```

🔴 **O achado que vale mais que o gate: uma sabotagem não falsifica três fronteiras.** Medido, não
suposto — sabotar o resolvedor de REQ reprova só **9 das 36** asserções. O `adr_orphan` fica
*mais forte* (sem REQs, o ADR fica mais órfão) e o `note_orphan` é **insensível** (nunca toca
`req_dir`). Daí **três seams**, um por fronteira, cada um com sua sabotagem e seu cenário permanente
em `check-gates-falsify.sh` (183 REQ/verificador · 184 NOTE/gerador · 185 ADR/gerador):

```
A · resolvedor de REQ (verificador)   -> 9 reprovadas / 27 OK
B · note new escreve (notes/<arq>.md) -> 6 reprovadas / 30 OK
C · adr new emite Rascunho            -> 6 reprovadas / 30 OK
```

**Um único seam teria dado a impressão de cobrir os três artefatos cobrindo um.**

**A métrica, e por que discrimina:** *"a entrada do `validate --json` cita, em `file` ou `message`, o
basename exato do arquivo que o gerador acabou de escrever"*. O basename carrega data+slug — nenhum
outro artefato o satisfaz por acidente, e só há um caminho para ele chegar à saída: o verificador ter
resolvido o arquivo onde o gerador gravou. Deliberadamente **não** é "a regra apareceu na saída", que
deu 6/6 verde na árvore sabotada do ML-1A.

**Cobertura real declarada com a ressalva:** 18/18 combinações, mas o eixo de layout do braço de
**nota é degenerado por construção** — `vault/notes` é constante do gerador
(`internal/generators/note.go:12`), então as 2 execuções percorrem o mesmo caminho. Declarado em vez
de contado como cobertura que não existe.

🔴 **Achado novo, registrado e NÃO corrigido (escopo negativo):** uma regressão que derrube o campo
`status:` do frontmatter gerado por `adr new` é **invisível ao `validate`**. Trocando `status:` por
`state:` nos 3 geradores, o gate dá **EXIT=0, 36/36** — o verificador cai de volta na linha em prosa
`> Date: … | Status: …`, escrita pelo mesmo template. **O discriminante real é o vocabulário, não a
chave.** Vira REQ própria.

**Três observações reportadas, não corrigidas:** o `init` do Python semeia um ADR que Go e Node não
semeiam — o que **proíbe métrica por contagem** e é a razão de a métrica ser por basename; o
`validate --json` do Go imprime `N violation(s) found` **depois** do JSON, quebrando `json.load` puro;
e o resumo do `check-gates-falsify.sh` já estava defasado antes deste ML.

## Verificação que só o CI fecha

CI verde nos 3 runtimes. E a prova de campo é o repositório consumidor do relato voltar a reportar as
18 violações que existiam e nunca foram olhadas.

## Barreira final

`hefesto-tf` e `hades-tf` — há mudança de resolução de caminho a partir de nome de diretório vindo do
disco, então o Hades entra. Auditoria do arquiteto e `barrier`.
