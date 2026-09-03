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


### ML-2B — Microlote corretivo da barreira final
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `docs/cli-parity.md`, `internal/validator/validator.go`
**Origem:** `docs/qualidade/2026-09-03-parecer-resolvedor-de-req.md` (B1) e
`docs/seguranca/2026-09-03-parecer-resolvedor-de-req.md` (S5, §4).

**B1 — BLOQUEIA. `docs/cli-parity.md:385` declara cobertura que não existe.** O texto diz que
*"every rule, generator and command calls them instead of rebuilding the tree"*. O `serve` é um
comando e reconstrói a árvore nos **3** runtimes. Medido pelo `hefesto-tf`, mesmo fixture `by_agent`
canônico, `GET /api/chain`, nós `type:"req"`:

```
go   -> 1 no      node -> 0 nos      py -> 0 nos
```

🔴 **A espécie do resíduo está invertida no documento.** O declarado (`npm/src/validator/traceid.js`)
é **superconjunto** — nunca vácuo, benigno. O **não** declarado (`serve/api_chain` em Node e Python)
é **vácuo no layout canônico**. Deixar o vácuo fora da lista transforma resíduo conhecido-por-ninguém
em "coberto" aos olhos do próximo auditor. **É a mesma falha que corrigimos hoje neste mesmo
documento** — contrato afirmando cobertura maior que a real.
**Remédio:** trocar a afirmação universal por enumeração honesta, e acrescentar `serve/api_chain` à
lista de resíduos declarados. **Doc-only, ~4 linhas.**

**S5 — paridade quebrada em código NOVO desta branch.** `internal/validator/validator.go:1391` testa
**só o índice 0**; Node e Python **filtram todos os vazios**:

```
agents: ["", "zeus"]   ->  Go: req_dir/default/   Node/Python: req_dir/zeus/
```

Uma linha. Deixar divergência conhecida em código novo é o oposto do que a barreira existe para
fazer.

**§4 — dedup lexical não vê `Backlog` ≡ `backlog`** em sistema de arquivos case-insensitive
(APFS, NTFS). Consequência que decide: **verde no CI Linux, vermelho na máquina do dev** — a AC
numérica prevê 2 violações e dá 4, nos 3 CLIs.

**B1-bis — SEGUNDO resíduo vácuo não declarado, achado no despacho do ML-2B e confirmado pelo
arquiteto.** `pypi/trackfw/commands/status.py:50` — `_count_reqs_by_status` chama `_list_files(req_dir)`
(linha 26), que é `os.listdir` de **um nível só** e **não** passa pelo ponto único. Em `by_agent`, o
`status` do Python conta **zero REQs**. O arquivo já importa `resolve_req_files` (2 ocorrências) e
usa em outro ponto — a divergência é interna ao mesmo arquivo. **Mesma espécie do `serve/api_chain`:
vácuo, não superconjunto.** Entra na lista de resíduos declarados junto com ele.

**Critérios de aceite:**
- [x] B1: a afirmação universal do `cli-parity.md` virou enumeração, e `serve/api_chain` está na
      lista de resíduos **com a medição** (1 nó no Go, 0 no Node/Python)
- [x] B1-bis: `pypi/.../status.py:50` também está na lista, com a medição do `by_agent`
- [x] S5: `agents: ["", "zeus"]` faz os **3** CLIs gravarem em `req_dir/zeus/`. Falsificado nas duas
      direções
- [x] §4: em FS case-insensitive, `req_dir/Backlog` ≡ `backlog` deixa de contar em dobro —
      **verificado por execução no macOS**, não por leitura
- [x] 🔴 **Controle:** o gate de ciclo fechado continua `rc=0` e as 3 sabotagens continuam reprovando
- [x] `make quality` verde e `trackfw validate` exit 0


**Evidência de aceite — auditoria do arquiteto (2026-09-03):**

```
check-artifact-closed-cycle.sh                       rc=0
dedup em APFS: 1 arquivo -> cada violacao UMA vez    (antes: duplicada)
grep -c 'addChild(reqDir, agent)' no validator.go    1   <- pin nao reproduz a ancora
```

🔴 **O agente rejeitou o mecanismo que EU sugeri para o §4, e a medição lhe dá razão.** Propus
identidade de inode. Ele mediu: o Go não tem chave hasheável portátil de `(dev,ino)`
(`syscall.Stat_t` não existe no Windows; `os.SameFile` é par-a-par, O(n²)), e um `ino` que repete ou
lê `0` em FS de rede **colapsa arquivos distintos** — exatamente a direção proibida. Escolheu
**filtro de existência verbatim**: o candidato só é enumerado se o nome aparecer literalmente no
`readdir` do pai. **Mede o disco em vez de presumir a propriedade do FS**, é idêntico nos 3 runtimes,
cobre de graça o eixo NFC/NFD, e o fallback é dupla contagem benigna — **nunca lista vazia**.

E não aceitou controle por argumento: **criou um volume APFS case-sensitive real com `hdiutil`**.

```
APFS case-INsensitive   antes 4/4/4   depois 2/2/2   filtro desligado 4/4/4  <- discrimina
volume case-SENSITIVE   Backlog/A.md + backlog/B.md -> 2 REQs, basenames distintos, 3/3 CLIs
                        filtro desligado: identico   <- prova que NAO houve supressao
```

**S5:** o discriminante veio do lado leitor — `resolveAgentNamespaces` já descarta `""` nos 3, então
filtrar devolve **uma** noção de agente ao par (D4). `["", "zeus"]` → `zeus/` nos 3.

**B1 + B1-bis:** enumeração **por runtime**, porque o conjunto não é uniforme e a lista do parecer
seria falsa em 3 células. Medido: `serve /api/chain` → `go=1 nó, node=0, py=0`; `status` → `go=1,
node=1, py=0` (**B1-bis é Python-only** — os 3 foram verificados antes de declarar).

### Auditoria cross-role do Cenário 183 — roteada ao `artemis-tf`, papel dono

O `apolo-tf` foi obrigado a retargetar o Cenário 183 (a renomeação do §4 quebrou a âncora literal, e
o gate reprovou **fail-closed**) e **pediu** que a auditoria fosse roteada. Foi.

**Seam provado idêntico, com desenho de 4 braços**: `npm/` e `pypi/` **fixos** na árvore de trabalho
— deixá-los variar misturaria o seam do Go com os fixes dos outros dois — e só o `GO_BIN` variando.
O braço `B0` (HEAD íntegro) foi necessário para provar que o `validator.go` pré-§4 não reprovava por
dupla contagem em APFS, sem o que `B1` seria ininterpretável. **Saídas idênticas**, as mesmas 3
asserções.

🔴 **Ela achou um defeito no próprio ML-2A dela:** o cabeçalho do Cenário 183 afirmava que sabotar o
resolvedor deixa `adr_orphan` **INTACTO**. Falso, e sempre foi — reprovam 3 asserções, não 1. Prosa
corrigida.

🔴 **E mediu uma armadilha ao consertar:** a primeira versão do pin **citava a âncora verbatim**, e o
`grep -c` foi de 1 para **2** — `corrupt_literal` conta o arquivo inteiro e não exclui comentário.
**O pin teria quebrado o cenário que existe para proteger.** Regra registrada no vault: *um pin de
âncora descreve a chamada, nunca a reproduz.* Verificado por mim: contagem em 1.

**Terceira ocorrência** de âncora literal deste harness quebrando por renomeação (Cenários 81 e 179
antes). O harness completo foi re-rodado: **359 OK, 0 FAIL, 13m33**.

**Sem paridade devida no pin:** ele descreve uma âncora que só existe no Go — replicá-lo em
Node/Python afirmaria um pin inexistente.

🔴 **Não corrigir aqui:** o symlink em `req_dir/default` que faz o `req new` escrever fora da árvore
(S2) — o furo **idêntico já existe na `main`** no `roadmap new`, medido, então é extensão de classe e
vira REQ com guarda compartilhada nos 3 escritores. Nem o `serve/api_chain` em si (A1), nem
`traceid.py:262` (A3), nem a degeneração do braço de ADR (A2).

## Verificação que só o CI fecha

CI verde nos 3 runtimes. E a prova de campo é o repositório consumidor do relato voltar a reportar as
18 violações que existiam e nunca foram olhadas.

## Barreira final

`hefesto-tf` e `hades-tf` — há mudança de resolução de caminho a partir de nome de diretório vindo do
disco, então o Hades entra. Auditoria do arquiteto e `barrier`.
