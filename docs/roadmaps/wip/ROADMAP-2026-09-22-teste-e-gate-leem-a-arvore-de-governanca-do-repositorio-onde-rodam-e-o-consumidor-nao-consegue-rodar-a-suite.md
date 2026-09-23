---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: teste e gate leem a árvore de governança do repositório onde rodam

> Created: 2026-09-22 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-22-teste-e-gate-leem-a-arvore-de-governanca-do-repositorio-onde-rodam-e-o-consumidor-nao-consegue-rodar-a-suite.md`
Origem: **#396** (consumidor externo, com medição). Mesma classe declarada: **#277**, e o achado 16
do #216 (fechado).

O produto se propõe a governar o projeto de quem o instala. Quando um teste ou gate lê a árvore de
governança **do repositório onde roda** e presume o layout do mantenedor, a suíte fica inalcançável
para o consumidor — a proposta de valor sendo negada pelo próprio produto.

## Acceptance Criteria

- [ ] Enumeração real da população, classificada em (a) exige do consumidor · (b) audita o upstream,
      legítimo · (c) já usa fixture própria
- [ ] Todo sítio (a) corrigido
- [ ] Nenhum artefato declara o que não sustenta (Regra Dura de Reconciliação)
- [ ] Falsificação nas duas direções, incluindo o controle
- [ ] Prova de execução num consumidor `by_agent`, sem importar governança alheia
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Enumeração e modelo de ameaça
> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — enumerar a população e classificar cada sítio
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-22)
**Files affected:** `docs/seguranca/2026-09-22-enumeracao-acoplamento-a-governanca-do-repo.md`

#### Resultado — **(a)=3 · (b)=4 · (c)=7 · fora de população=1**

🔴 **Minha medição inicial foi REFUTADA na população, não confirmada.** Eu medi `1` sítio de teste
porque procurei o **literal** `repoRoot(`. O segundo alcança a árvore real por
`filepath.Abs(filepath.Join("..", ".."))` — mecanismo diferente, mesmo defeito. Era exatamente o
risco que declarei no handoff, e por isso pedi refutação em vez de herança do número.

| # | sítio | evidência |
|---|---|---|
| 1 | `internal/roadmapdoc/roadmapdoc_test.go:250` | RC=1 contra árvore `by_agent` sem `docs/roadmaps/done/` — **#396** |
| 2 | `internal/validator/validator_test.go:2246` | lê **3 REQs reais por caminho literal** com `t.Fatalf`; RC=1 na mesma árvore — é o **achado 16 do #216** |
| 3 | `scripts/check-roadmap-barrier-contract.sh:512` | tripwire de disco **incondicional**; 144 basenames ausentes ⇒ 144 falhas — **#277** |

**#277 entra no ML-1B por medição, não por presunção**, como o roadmap exigia. O corpus congelado
já está em `scripts/testdata/` (mecanismo correto); o defeito é só a tripwire da linha 512.

🔴 **E a proposta do #277 tem um buraco medido:** a guarda `TRACKFW_SELF_GOVERNED=1` só fecha o
defeito se o CI do mantenedor setar a variável — **0 ocorrências** em `.github/`, `Makefile` e
`scripts/`. Confirmei. Sem isso, "desacoplar" vira "desligar".

#### O critério (a)/(b) — e a refutação do meu candidato

Eu propus *"rodaria num clone sem nenhum arquivo em `docs/`?"*. **Está errado, e ele mostrou por
quê:** em `by_agent` o `docs/` **tem** conteúdo (`docs/roadmaps/<agente>/done/`); o teste passaria e
o defeito persistiria. Além disso não cobre artefato fora de `docs/`.

O critério adotado é P1 (pertence ao domínio configurável `roadmap_dir`/`req_dir`/`adr_dirs`?) → P2
(o conteúdo varia entre mantenedor e consumidor?) → P3 (resolve via config **e** tolera qualquer
estado válido, incluindo vazio e `by_agent`?).

🔴 **Com árbitro executável contra o adversário:** quando P3 for arguível em prosa, copia-se o source
para árvore temporária com `roadmap_namespacing: by_agent` sem os diretórios planos e executa-se o
artefato. **RC=0 → (b). RC≠0 → (a). Prosa não substitui RC.** É o que impede esvaziar a wave
classificando tudo como "é do upstream, é legítimo".

**Contexto que você NÃO precisa remedir:** o #396 já está medido pelo reportante, e eu confirmei o
sítio. O seu trabalho é a **população**, não o caso individual.

**Actions:**
1. 🔴 **Enumeração pelo critério certo.** O critério é *"lê a árvore de governança do repositório
   onde roda"* — não *"menciona `docs/`"*. Minha medição inicial é **aproximada e você deve
   refutá-la ou confirmá-la**:
   ```
   $ grep -rn "repoRoot(" --include='*_test.go' internal/ | grep -v "func repoRoot" | wc -l
   1
   $ grep -rln 'docs/roadmaps\|docs/req\|docs/adr' scripts/*.sh | wc -l
   14
   ```
   ⚠️ O `14` **não** é a resposta — a maioria pode ser legítima. E o `1` pode estar subestimado:
   um teste pode alcançar a árvore real por `os.Getwd()`, por caminho relativo `../..`, ou por
   helper com outro nome. **Procure o mecanismo, não o literal.**
2. **Classifique cada sítio** em:
   - **(a)** exige a governança do consumidor → **defeito**
   - **(b)** audita o upstream e é legítimo → mantém, mas deve declarar isso
   - **(c)** já usa fixture própria → correto
3. **Modelo de ameaça.** Quem esvazia esta Wave 0 sem quebrar regra escrita? O caso óbvio: declarar
   tudo como (b) e não corrigir nada. Qual é o teste que distingue (a) de (b) sem depender de quem
   classifica?
4. **Falsificação nas duas direções.** Para cada superfície: o que quebra quando o produto regride,
   e o que quebra quando a correção vai longe demais — um gate que deixa de auditar a governança do
   upstream é regressão silenciosa, não melhoria.
5. **Residual declarado.**

**Acceptance criteria:**
- [x] Tabela completa, um veredito (a)/(b)/(c) por sítio, com `arquivo:linha` e o comando que
      produziu a lista — 15 sítios classificados, varredura por **5 mecanismos** de alcance
- [x] 🔴 O critério que distingue (a) de (b) está escrito e é aplicável por terceiro — P1/P2/P3 com
      **árbitro executável** (`RC=0 → (b)`, `RC≠0 → (a)`; prosa não substitui RC)
- [x] As quatro seções com evidência, não asserção de uma linha — 453 linhas, comando e saída
- [x] Nenhuma linha de implementação escrita neste ML — verificado: só `docs/seguranca/` foi criado

**Gate da wave:** `trackfw barrier <roadmap> --wave 0`, auditado por mim antes de qualquer despacho.

---

## Wave 1 — A correção
> Dependências: Wave 0 completa e **auditada**. Escopo definido pela tabela do ML-0A.

### ML-1A — `TestCorpusMeasurement_ReportOnly` cumpre o que declara (#396)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-22) · **Papel:** `artemis-tf`
**Files affected:** `internal/roadmapdoc/roadmapdoc_test.go` — e só este

**O defeito, já medido:** o cabeçalho da seção (linha 242) diz **"report-only, never fails"** e a
linha 253 faz `t.Fatalf`. Contradição interna literal — a **Regra Dura de Reconciliação** na forma
exata do achado A1 da auditoria externa de 2026-09-05.

Num consumidor `by_agent`, `docs/roadmaps/done` não existe e a cascata derruba `go`,
`windows-full-suites`, e pula `parity-falsify-shard` e `parity-other-gates`.

🔴 **Medição do reportante que decide o desenho:** com `docs/roadmaps/done/` **vazio** (só
`.gitkeep`), o teste **passa** e o `validate` segue com 0 violações. **Então ele não mede nada do
produto quando o diretório está vazio** — ele mede o corpus do mantenedor.

**Duas saídas possíveis, e a escolha precisa de justificativa escrita:**
- **skip declarado** quando o diretório não existe — cumpre o "never fails" do próprio docstring;
- **resolver o diretório pela config** (`roadmap_dir` + `roadmap_namespacing`) — mede o corpus de
  quem roda, seja qual for o layout.

O reportante sugere as duas e diz *"a escolha é sua"*. **Escolha uma, escreva por quê, e diga o que
a outra daria.** Se escolher skip: o teste passa a não medir nada no consumidor — isso é aceitável?
Se escolher config: ele passa a medir corpus alheio — o número resultante significa o quê?

**Acceptance criteria:**
- [x] O cabeçalho e o corpo **concordam** — se diz "never fails", não há `t.Fatalf`
- [x] Teste **load-bearing**: prove com um repositório sem `docs/roadmaps/done` que o comportamento
      novo difere do antigo. Cole as duas saídas
- [x] 🔴 **Braço (b):** no layout plano deste repositório, a medição **continua acontecendo** — a
      correção não pode transformar o teste em no-op para o upstream
- [x] `go test ./internal/roadmapdoc/` RC=0
- [x] 🔴 **NÃO rodar `make quality`** — barreira é do arquiteto
- [x] Uma frase declarando qual conclusão do ML o teste afirma

### ML-1B — os outros dois sítios (a)
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-22; sítio 3 corrigido no ML-1B-bis) · **Papel:** `apolo-tf`
**Files affected:** `internal/validator/validator_test.go`, `scripts/check-roadmap-barrier-contract.sh`
**Paralelo ao ML-1A** — arquivos disjuntos.

**Sítio 2 — `validator_test.go:2246`** (`TestExtractRefPath_TresREQsReaisDoRepositorio`): alcança a
árvore real por `filepath.Abs(filepath.Join("..",".."))` e lê **3 REQs por caminho literal**, com
`t.Fatalf` se faltarem. É o **achado 16 do #216**. As três REQs existem só neste repositório.

**Sítio 3 — `check-roadmap-barrier-contract.sh:512`**: a tripwire de disco é **incondicional** — para
cada basename do snapshot, exige `find $ROOT_DIR/docs/roadmaps`. Num fork, 108 de 144 ausentes.
Proposta do #277: corpus (já em `scripts/testdata/`, correto) separado da tripwire, que passa a
rodar só no upstream.

🔴 **O buraco medido na proposta do #277:** `TRACKFW_SELF_GOVERNED=1` tem **0 ocorrências** em
`.github/`, `Makefile` e `scripts/`. Se a tripwire ficar atrás dessa variável e ninguém a setar,
"desacoplar" vira **desligar** — e o upstream perde a auditoria sem que nada fique vermelho.
**Qualquer desenho aqui precisa provar que a tripwire continua rodando no upstream.**

**Acceptance criteria:**
- [x] Sítio 2: o teste não depende de REQ que só existe neste repositório; a conclusão que ele
      afirma (o extrator resolve ADR citado entre backticks) continua **afirmada e verificada**
- [x] Sítio 3: tripwire separada do corpus, e **prova de que ela continua reprovando no upstream**
      quando um roadmap some do disco
- [x] 🔴 Árbitro do ML-0A aplicado aos dois: árvore temporária com `roadmap_namespacing: by_agent`
      sem diretórios planos → **RC=0**. Cole a saída
- [x] Braço (b): no layout plano deste repositório, os dois continuam medindo o que mediam
- [x] `go test ./internal/validator/` RC=0 · `bash scripts/check-roadmap-barrier-contract.sh` RC=0
- [x] 🔴 **NÃO rodar `make quality`** e **NÃO** usar `go test ./...` — barreira é do arquiteto
- [x] Uma frase por teste novo (Regra Dura de Reconciliação)

Inclui, se a Wave 0 confirmar: o corpus do `check-roadmap-barrier-contract` (**#277**), cuja proposta
do reportante é separar as **fontes** — corpus vira fixture em `scripts/testdata/` (roda em qualquer
clone) e a tripwire de disco vira gate separado, executado só no upstream ou atrás de
`TRACKFW_SELF_GOVERNED=1`.

🔴 **Não presuma que #277 entra.** Ele entra se a Wave 0 medir que é a mesma causa. Se a medição
disser que a causa é outra, ele sai — **com a medição escrita**, como manda a Regra Dura.


---

#### Auditoria — resultado
**Veredito:** ✅ Concluído (auditado por Zeus em 2026-09-22). Escolha **(i) skip declarado**, com o
trade-off escrito. Braço (b) provado: no layout plano, `total=194, unfinished=26/25` — a medição
continua acontecendo. Árbitro `by_agent`: OLD `RC=1` → NEW `RC=0`.
Achado extra dele, da mesma classe: as notas do arquivo afirmavam baselines de **27/30/32** de
quando o corpus tinha ~27 itens; hoje são **194**. Corrigido.
🔴 **Falso alarme meu na auditoria:** contei um `t.Fatalf` remanescente na função com `grep -c` — era
o literal dentro de um **comentário**. Meu grep não distinguiu comentário de código, que é
exatamente o que o `check-symlink-privilege-guard` faz certo.

#### Auditoria — resultado: sítio 2 aprovado, **sítio 3 REPROVADO**
**Veredito:** 🔄 Em andamento — corretivo **ML-1B-bis** pendente

**Sítio 2 ✅** — fixture preserva o discriminante (`adr: ""` com aspas, ADR só em backtick no corpo),
e ele **acrescentou** `TestExtractRefPath_CorpusBacktickREF`: controle que ainda lê os 3 arquivos
reais, mas com `t.Logf`+`continue` quando ausentes e `t.Skip` declarado se nenhum for achado — evita
trocar acoplamento por vacuidade silenciosa. Upstream: 3/3 verificados.

**Sítio 3 ❌ — a correção não alcança o caminho que o consumidor usa.**
Ele mesmo declarou o residual, e a medição confirma:
```
Makefile:75   TRACKFW_SELF_GOVERNED=1 ... scripts/check-roadmap-barrier-contract.sh
Makefile:27   parity-rest: build          →  parity: build parity-rest parity-falsify
Makefile:156  quality: test lint parity
```
O pin é **incondicional** e está dentro de `parity-rest`. **Um fork que rode `make quality` continua
reprovando com os 144 basenames** — e `make quality` é exatamente o que o #277 relata como
inalcançável. O que ficou desacoplado foi a invocação avulsa do script, que ninguém usa.

🔴 **E a correção criou uma trava:** `check-parity-call-site-pins.sh` agora **exige** o pin
(`VARS_PIN=(... TRACKFW_SELF_GOVERNED)`), então removê-lo reprova outro gate. A saída fácil está
fechada por construção.

**Crédito ao executor:** ele **previu e declarou** exatamente isto no relatório, em vez de entregar
como concluído. Foi o que tornou a reprovação barata.

### ML-1B-bis — tirar a tripwire do caminho do consumidor
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
**Files affected:** `Makefile`, `.github/workflows/quality.yml`,
`scripts/check-parity-call-site-pins.sh`. **Não** tocar no `check-roadmap-barrier-contract.sh` — o
guard interno dele está correto.

**A regra:** o que o consumidor roda (`make quality` → `parity` → `parity-rest`) **não pode** exigir
a governança do mantenedor. A tripwire é auditoria do upstream e pertence a um alvo próprio,
invocado pelo **CI do upstream**, não pela suíte geral.

**Desenho pedido:**
1. Remover a linha 75 de `parity-rest`.
2. Criar alvo próprio (ex.: `self-governance:`) que invoca o script com `TRACKFW_SELF_GOVERNED=1`.
3. Invocar esse alvo em `.github/workflows/quality.yml`, num job do upstream.
4. Reapontar o pin de vacuidade em `check-parity-call-site-pins.sh` para o **novo call site**, de
   modo que ele continue reprovando se o pin sumir — 🔴 **sem** voltar a exigi-lo em `parity-rest`.

**Acceptance criteria:**
- [x] 🔴 **`make quality` numa árvore `by_agent` sem os roadmaps do mantenedor → RC=0.** É o AC que
      o ML-1B não atendeu. Prove rodando, e cole a saída
- [x] 🔴 **A tripwire continua reprovando no upstream:** apague um roadmap do disco, rode o alvo
      novo, veja reprovar. Cole a saída
- [x] O CI do upstream invoca o alvo novo — mostre a linha do workflow
- [x] O pin de vacuidade reprova se o `TRACKFW_SELF_GOVERNED=1` sumir do novo call site — prove
      removendo
- [x] 🔴 **NÃO rodar `make quality` completo** para validação de rotina — você é o único agente
      agora, mas a barreira é do arquiteto. A exceção é o AC 1, que **exige** `make quality` na
      árvore temporária `by_agent` — essa roda, porque é a prova

#### Auditoria — resultado: aprovado, e o desenho está certo
**Auditado por Zeus em 2026-09-22.** Verifiquei pelo `make -n`, que é a fonte de verdade do que
cada alvo executa:
```
$ make -n quality | grep check-roadmap-barrier-contract
GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh      ← SEM o pin

$ make -n self-governance | grep check-roadmap-barrier-contract
TRACKFW_SELF_GOVERNED=1 GO_BIN=... scripts/check-roadmap-barrier-contract.sh                ← COM o pin
```
O consumidor roda o gate **sem** a tripwire; o upstream roda **com**. O defeito do #277 no caminho
`make quality` está fechado, e o alvo novo entrou num job existente (`parity-other-gates`), sem
perturbar a lista de required checks.

### ML-1C — a regressão deste desacoplamento não é detectada por ninguém
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-22) · **Papel:** `artemis-tf`
**Files affected:** `scripts/check-parity-call-site-pins.sh` e seu cenário de falsificação em
`scripts/check-gates-falsify.sh`. **Não** tocar no `Makefile` nem no workflow — estão corretos.

🔴 **Achado da minha auditoria, por falsificação — não por leitura.** Reintroduzi o pin em
`parity-rest`, que é **exatamente a regressão** do defeito que o ML-1B-bis acabou de corrigir:

```
$ # pin TRACKFW_SELF_GOVERNED=1 reintroduzido na linha de parity-rest
$ bash scripts/check-parity-call-site-pins.sh > out 2> err; echo "RC=$?"
RC=0
check-parity-call-site-pins: 9 verificação(ões) -- 2 pin-all(s) + 1 pin-any(s) + 3 rastro(s)
```

**Nada reprova.** A política `pin-any` (linha 74: `VARS_PIN_ANY=(TRACKFW_SELF_GOVERNED)`) exige que
**ao menos uma** invocação pine — e `self-governance` pina. Ela é cega para uma invocação **a mais**
que também pine, que é precisamente como o defeito volta.

**Por que `pin-any` foi a escolha certa e mesmo assim insuficiente:** `pin-all` seria impossível aqui
— `parity-rest` **deve** invocar sem o pin. O executor acertou em trocar a política; o que falta é a
asserção **negativa** que a acompanha.

**Ação:** acrescentar ao gate uma verificação de que **nenhuma invocação alcançável por
`make quality`** pina `TRACKFW_SELF_GOVERNED`. O discriminante robusto é o próprio `make -n quality`,
não a leitura textual do Makefile — ele resolve as dependências entre alvos e imuniza contra alguém
mover a linha para outro alvo que `quality` alcance.

**Acceptance criteria:**
- [x] O gate **reprova** quando o pin é reintroduzido em `parity-rest` — prove injetando e colando a
      saída, e **restaure** depois
- [x] O gate **reprova** também se a linha for movida para outro alvo alcançável por `make quality`
      (ex.: `parity-falsify`) — é o caso que a leitura textual do Makefile não pega
- [x] O gate continua **RC=0** na árvore correta
- [x] Cenário de falsificação em `check-gates-falsify.sh` com **rótulo literal**, colhido pela guarda
      de conjunto. 🔴 Rótulo montado com variável vira **glob** em `gen-falsify-chunks.py` e a guarda
      para de checar o caso específico
- [x] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto
- [x] Uma frase por cenário novo (Regra Dura de Reconciliação)

#### Auditoria — resultado: aprovado, verificado por mim com a MESMA mutação
**Auditado por Zeus em 2026-09-22.** Repeti a falsificação que eu havia feito contra o gate antigo:

| | gate antes do ML-1C | gate depois |
|---|---|---|
| pin reintroduzido em `parity-rest` | **RC=0** (cego) | **RC=1** |

```
FAIL [call-site-pin/TRACKFW_SELF_GOVERNED/forbidden-in-quality]: make quality alcança ao menos uma
invocação de scripts/check-roadmap-barrier-contract.sh que pina TRACKFW_SELF_GOVERNED= -- o pin
pertence exclusivamente ao alvo upstream (self-governance), não ao caminho do consumidor:
TRACKFW_SELF_GOVERNED=1 GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
```

A mensagem **nomeia o defeito e mostra a linha ofensora** — não diz só "falhou". E o Makefile voltou
íntegro (`make -n quality` mostra a invocação sem o pin).

**O discriminante é `make -n quality`, como exigido** — por isso o braço `self-governed-moved` pega o
pin movido para `parity-falsify`, caso que varredura textual de `parity-rest` não pegaria.

**Decisão do executor que eu endosso:** ele **não** incrementou `FALSIFY_SUCCESS_FLOOR=205`, com a
razão escrita — é piso, verificado em modo serial, e incrementá-lo sem medir o total serial novo
seria **número fabricado**. Preferir não afirmar a afirmar sem medir é exatamente o comportamento que
esta campanha vem cobrando.

## Wave 2 — corretivos da barreira final
> Dependências: revisões `hades-tf` e `hefesto-tf` concluídas em 2026-09-22, ambas **aprovado com
> ressalvas**, nenhuma bloqueando. **ML único, `artemis-tf`** — os três itens são de teste/gate.

🔴 **Convergência independente entre os dois revisores:** Hades (R-C) e Hefesto (Q4) apontaram, sem
se falarem, que o step `make self-governance` pode ser **deletado do workflow sem que nada reprove**.
Dois pareceres convergindo num mesmo defeito é sinal forte — e é exatamente a causa desta REQ:
proteção que desliga em silêncio.

### ML-2A — fechar os três achados acionáveis
**Status:** ✅ Concluído · **Papel:** `artemis-tf`
**Files affected:** `internal/validator/validator_test.go`, `internal/roadmapdoc/roadmapdoc_test.go`,
e o gate que verificar o workflow (ver item 3). **Não** tocar no `Makefile`.

#### 1. Asserção fraca na fixture (Hefesto Q2)
As asserções do teste novo são só `got == ""` e `strings.HasSuffix(got, ".md")`. **Um parser quebrado
de outro jeito, que devolvesse qualquer outro caminho `.md`, passaria em silêncio.**

🔴 **E o vizinho já faz certo:** `validator_test.go:2222` usa `got != adrRel` — igualdade. O teste
novo ficou **mais fraco que o teste pré-existente que ele espelha**. Acrescentar a comparação de
igualdade contra o caminho esperado, nos dois (`TresREQsReaisDoRepositorio` e `CorpusBacktickREF`).

#### 2. Vacuidade: `done/` vazia mede zero e passa (Hades R-A, confirmado por mim)
```
$ # docs/roadmaps/done/ existe e está VAZIA
--- PASS: TestCorpusMeasurement_ReportOnly
    done/ corpus: total=0, unfinished=0, unfinished=0
RC=0
```
Não há guarda de vacuidade — medido: zero ocorrências de `total == 0` / piso na função.

⚠️ **O contrato do teste é "never fails", e ele deve continuar assim.** A saída correta **não** é
falhar: é **declarar** — `t.Skip` quando o corpus está vazio, como já faz quando o diretório não
existe. Silenciar com `total=0` é dizer "medi" sem ter medido, e é a classe que esta campanha inteira
vem corrigindo.

#### 3. O step do CI pode sumir sem nada reprovar (Hades R-C **e** Hefesto Q4)
Medido pelos dois: deletando o step `ML-1B-bis — self-governance` de `quality.yml`, `parity-rest`
passa, `parity-other-gates` passa, `parity` (**required check**) passa. **Nada reprova**, e a
tripwire do upstream desliga. `check-orphan-gates.sh` não pega, porque o script segue invocado por
`parity-rest`.

Acrescente a verificação de que **o workflow invoca `make self-governance`**. O gate natural é
`check-parity-call-site-pins.sh`, que já lê `Makefile` e já tem a noção de call site — mas a escolha
é sua, desde que o gate **reprove** quando o step sumir.

**Acceptance criteria:**
- [x] Asserção de igualdade nos dois testes; prove que um retorno `.md` **errado** agora reprova
- [x] `done/` vazia ⇒ **skip declarado**, não `total=0` silencioso. Prove com diretório vazio
- [x] `done/` com conteúdo ⇒ medição **continua acontecendo** (braço b: 194 no upstream)
- [x] 🔴 Remover o step do `quality.yml` ⇒ **gate reprova**. Injete, cole a saída, **restaure e prove
      que restaurou**
- [x] Gate continua RC=0 na árvore correta
- [x] Cenário de falsificação com **rótulo literal** para o item 3, colhido pela guarda de conjunto
- [x] 🔴 **NÃO rodar `make quality`** — a barreira é do arquiteto
- [x] Uma frase por teste/cenário novo (Regra Dura de Reconciliação)

### Fora de escopo — vira issue, com a razão
**Hefesto Q1:** `TestCorpusMeasurement_ReportOnly` promete *"surface the count in the CI log"*, mas o
job roda `go test -timeout 2m ./...` **sem `-v`** (`quality.yml:31`) e o `t.Logf` é descartado.
Confirmei. É de novo um artefato que não sustenta o que declara — **mas o mecanismo é outro**
(visibilidade de log em CI, não acoplamento a governança) e a correção exige uma **decisão de
produto**: virar step de CI com `-v`, ou deletar o teste. Issue própria, com a medição.

## Barreira final

Revisão `hefesto-tf` e `hades-tf`, auditoria do arquiteto, `trackfw barrier`.
🔴 **CI verde, não só verde local** — e, neste roadmap especificamente, a prova que importa é a
**execução num consumidor `by_agent`**, porque é exatamente o ambiente que o defeito quebra e que o
CI do mantenedor **não** exercita.

⚠️ Custo de CPU: ver `vault/notes/carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md`
— teste do pacote tocado nos handoffs, `TRACKFW_FALSIFY_JOBS=4` na barreira local.
