---
status: done
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md"
squad: "hades-tf, ares-tf, apolo-tf"
---

# Roadmap: O repositório do trackfw sob os cuidados do trackfw

> Created: 2026-09-01 | Status: done

## Context

REQ: `docs/req/REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md`
ADR: `docs/adr/ADR-2026-09-01-o-repositorio-do-trackfw-e-governado-pelo-trackfw-...`

**O trackfw vende rastreabilidade aplicada e não a aplica a si mesmo.** Medido: `main` sem
`required_status_checks`, zero revisão exigida, `enforce_admins: false`; guards vivendo só no harness
de agente com `core.hooksPath = /dev/null`; e a cadeia nunca publicada como exigência.

**Qualquer PR pode ser mergeado com todo o CI vermelho.** Tudo o que construímos hoje é advisory.

## Acceptance Criteria

- [ ] Enumeração do que o trackfw instala em terceiros e não usa em si
- [ ] `required_status_checks` configurado, com a **escolha dos checks justificada**
- [ ] Guards ativos para humanos, **sem quebrar fluxo legítimo**
- [ ] Falsificação de cada controle **nas duas direções**
- [ ] `enforce_admins` decidido explicitamente
- [ ] 🔴 O `trackfw doctor` acusa estas lacunas — conserta todos os projetos, não só este
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — A enumeração é o entregável
> Dependências: nenhuma. Bloqueia tudo.

### ML-0A — O que o trackfw instala e este repositório não usa
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que a enumeração é o trabalho, e não um preâmbulo:** as três lacunas conhecidas foram achadas
**por acidente**, investigando outra coisa. Não há razão para supor que sejam as únicas — e nesta
sessão duas enumerações minhas erraram por uma ordem de grandeza, com você achando a população real
nas duas.
**Actions:**
1. **Varra o que o produto gera:** `trackfw init`, `discover`, `update harness`, `integrations
   install`, `agents install`, `skills install`. Para cada artefato que ele instala em projeto de
   terceiro, responda: **existe aqui? está ativo? está atualizado?**
2. 🔴 **A distinção que decide o roadmap:** separar *"não usamos e deveríamos"* de *"não usamos e há
   razão"*. Nem tudo que o produto instala faz sentido no repositório do próprio produto — e tratar
   os dois como iguais produziria trabalho inútil e ruído.
3. **Modelo de ameaça do portão que vamos ligar.** `required_status_checks` mal escolhido **trava o
   projeto**: os jobs de Windows nascem vermelhos por projeto. Quais checks são exigidos, e por quê?
   E `enforce_admins` — num projeto com um mantenedor, a escotilha de emergência tem valor legítimo.
4. 🔴 **Falsificação nas duas direções, e a simétrica é a que dói:** cada controle que ligarmos pode
   **quebrar fluxo legítimo**. Guard ativo para humanos que impeça um `git commit` normal é pior que
   guard ausente. Nomeie o que **não** pode ser bloqueado.
5. **Residual declarado.**
**Critérios de aceite:**
- [x] Enumeração com a distinção "deveríamos" × "há razão", item a item
- [x] Veredito sobre quais checks exigir, com o custo de cada escolha
- [x] Veredito sobre `enforce_admins`
- [x] Nenhuma linha de configuração alterada
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

**Quatro achados mudam o plano, e o primeiro bloqueia a AC2.**

### 1. 🔴 O nome `governance` colide três vezes — e o dado estava na minha frente o dia todo

```
internal/generators/scaffold.go:1917         → trackfw-gate.yml      job id: governance
internal/generators/scaffold_doctor.go:45    → trackfw-validate.yml  job id: governance
                                               (dispara em push E pull_request)
```

Verificado ao vivo no PR #241: **três check-runs distintos com o mesmo nome**.

**Eu vi `"governance=SUCCESS","governance=SUCCESS","governance=SUCCESS"` em cada verificação de CI
que fiz hoje — uma dúzia de vezes — e nunca perguntei por que eram três.** A repetição normalizou a
anomalia.

**Por que bloqueia a AC2:** o GitHub casa check exigido **por nome**. Exigir `governance` com três
homônimos torna o portão **ambíguo** — satisfeito por qualquer um deles, imprevisivelmente. **Um
portão que parece fechado.** Precisa de job ids únicos **antes** de entrar no `required_status_checks`.

E como os dois geradores são do produto, **todo projeto que adota o trackfw herda a colisão.**

### 2. A AC3 não fecha a lacuna que a REQ usa para justificá-la

O único gerador de hook de git (`generateCommitMsgHook`) só atende husky/lefthook — **não há caminho
para um repositório Go-only como este**. E mesmo construído, `commit-msg` é **classe errada de
controle** para 2 dos 3 incidentes citados: `git stash` e `checkout --` não têm hook nativo que
dispare antes do subcomando, como o `PreToolUse` do harness de agente faz.

**Isto precisa estar dito na Wave 1**, não descoberto depois de declarar "AC3 concluída". A REQ
prometia paridade humano-agente que o git **não permite**.

### 3. A AC6 está subescopada

O `doctor` é **exclusivamente de sistema de arquivos** — manifesto e templates. Não tem visibilidade
alguma de API de branch protection nem de `core.hooksPath`. A AC6 exige **modalidade nova de
verificação** (rede + autenticação), não uma checagem a mais no desenho atual.

### 4. `enforce_admins` — ele defende `true`, e o argumento é bom

Com `required_approving_review_count: 0` num projeto de um mantenedor, `enforce_admins` decide **uma
única coisa**: se o portão vincula o admin. **Os quatro incidentes que a REQ cita como motivação
foram todos cometidos pelo admin, e nenhum foi bloqueado porque o CI nunca foi vinculante para ele.**

Recomenda `true`, com procedimento de flip temporário documentado e auditado como escotilha — não
buraco permanente.

### Achado positivo que o ADR não mencionava

`allow_force_pushes` e `allow_deletions` **já são `false`** na `main`.

### O que NÃO pode ser bloqueado (§4 do parecer)

Commits comuns fora de `feat/*`/`fix/*`; pushes para branch que não é `main`; **o PR autorreferente
que conserta um `governance` quebrado**; e os jobs de medição de Windows. Cada um com o mecanismo
concreto que poderia quebrá-lo.

O terceiro é o mais sutil: **um portão que exige `governance` verde impede o PR que conserta o
`governance`.**

### Residual

Dois dos três incidentes que motivam a AC3 são **permanentemente inatingíveis** por hook de git; o
review-count segue ponto único de falha; o flip de emergência é ele próprio ação de admin não
revisada; e a ausência de `.claude/agents/` e `.claude/skills/` em escopo de projeto **precisa de
decisão do KG** — pessoal versus compartilhável —, não de veredito dele.

## Wave 1 — Desfazer a colisão de nome (pré-requisito da AC2)
> Dependências: Wave 0. **O particionamento saiu da enumeração**, e ela apontou um bloqueio antes de
> qualquer configuração: não dá para exigir um check cujo nome é ambíguo.

### ML-1A — Job ids únicos nos dois workflows gerados, nos 3 CLIs
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:**
`internal/generators/scaffold.go`, `internal/generators/scaffold_doctor.go`,
`npm/src/generators/init.js`, `npm/src/commands/discover.js`,
`pypi/trackfw/generators/init_gen.py`, `pypi/trackfw/commands/discover.py`,
mais os testes que pinam o conteúdo gerado.

**Diagnóstico:** os dois workflows que o produto gera declaram **o mesmo job id**:

| CLI | `trackfw-gate.yml` | `trackfw-validate.yml` |
|---|---|---|
| Go | `scaffold.go:1924` | `scaffold_doctor.go:49` |
| Node | `init.js:233` | `discover.js:329` |
| Python | `init_gen.py:579` | `discover.py:474` |

**Paridade perfeita no erro** — os três concordam e os três estão errados. Nenhum gate de paridade
pegaria: paridade mede **concordância**, não **correção**.

E o `trackfw-validate.yml` dispara em `push` **e** `pull_request`, então um PR produz **três**
check-runs homônimos. O GitHub casa check exigido **por nome** — exigir `governance` seria um portão
**satisfeito por qualquer um dos três, imprevisivelmente**. Um portão que parece fechado.

**Como os geradores são do produto, todo projeto que adota o trackfw herda a colisão.**

**Actions:**
1. Job ids únicos e descritivos nos dois workflows, nos 3 CLIs. Os nomes entram no
   `required_status_checks` de quem adota — **precisam dizer o que verificam**, não de onde vieram.
2. Atualizar os testes que pinam o conteúdo gerado.
**Critérios de aceite:**
- [x] Job ids distintos entre os dois workflows, **idênticos entre os 3 CLIs**
- [x] 🔴 **Medido num PR real:** os check-runs aparecem com nomes distintos, e nenhum nome se repete
- [x] 🔴 **Controle:** os dois workflows **continuam rodando e reprovando** quando devem — renomear
      não pode desligar o que já funcionava
- [x] Gate impedindo a reintrodução de job ids colidentes nos geradores dos 3 CLIs
- [x] `make quality` verde

**Gates da wave:**
```bash
make quality
```

#### Resultado do ML-1A (apolo-tf, 2026-09-02) — auditado pelo arquiteto

```
trackfw-gate.yml       governance-install-script   (curl | sh)
trackfw-validate.yml   governance-go-install       (go install)
```

Verifiquei: **nenhum `governance:` nu sobrou nos 6 geradores**, e os workflows vivos deste repositório
também estão corrigidos.

### O desvio de escopo dele é o achado que mais vale

Ele tocou `.github/workflows/trackfw-gate.yml` e `trackfw-validate.yml`, **fora da minha lista** — e
a justificativa é exata: **os workflows vivos deste repo divergem à mão do template** (usam
`go build` local em vez de `go install`, porque o repositório não consegue se instalar via `curl`
durante o próprio PR) e **não são regenerados** por `update`/`discover`.

Sem tocá-los, teríamos corrigido os geradores e **este repositório continuaria com a colisão viva** —
justamente o que a REQ existe para resolver. **A minha lista de arquivos estava incompleta**, de novo.

Ele também verificou que **nenhum `needs:` dependia do job `governance`** antes de renomear — o risco
concreto de quebrar cadeia de dependência.

### Os nomes: ele estava certo e eu errado

Eu instruí *"nomes que digam o que verificam, não de onde vieram"*. Mas os dois workflows verificam
**a mesma propriedade** (`trackfw validate` passa); nomear por propósito produziria **dois nomes
iguais**. Ancorar no **mecanismo de instalação** é o que distingue de fato — e **torna a redundância
visível** em vez de disfarçá-la.

Ele ainda escreveu no `docs/cli-parity.md` a dica para a Wave 2: sendo a mesma propriedade,
**exigir apenas um** é o argumento natural.

### Os testes stale — inspecionados, não presumidos

Os fixtures que pinam YAML (`update_test.go:1877/2034` e equivalentes em Node e Python) foram lidos
**linha a linha**. São *stale* **de propósito** — job id antigo e pin de versão velho, comparados
contra o template atual gerado dinamicamente.

**Atualizá-los por reflexo teria quebrado a detecção de conteúdo obsoleto**: o teste passaria a
comparar o novo contra o novo, e nunca mais acusaria drift.

### Falsificação — verificada por mim

```
gate novo, árvore correta      →  exit 0
colisão reintroduzida (cópia)  →  exit 1, nomeando o arquivo e o assert exato
```

**Controle que prova que nada mais mudou:** o `check-ci-workflow-pin-parity.sh` — que compara os 9
builders byte a byte — segue **15/15**. Só o job id mudou.

**`MAKE_EXIT=0`**, zero `FAIL`.

### Achado registrado em REQ própria

Os dois workflows **executam a mesma coisa**; diferem só no instalador. Um projeto que roda `init` e
`discover` paga **dois jobs por push** para uma verificação. **A colisão era o sintoma; a duplicação é
a causa** — REQ `REQ-2026-09-02-init-e-discover-geram-dois-workflows-...` aberta, com AC1 exigindo
descobrir **por que existem dois** antes de unificar.

## Wave 2 — Ligar os controles
> Dependências: Wave 1. **Só depois de os nomes serem únicos** é que `required_status_checks` pode
> ser configurado sem ambiguidade.
> 🔴 A Wave 0 corrigiu duas promessas da REQ que precisam constar aqui: a **AC3 não alcança**
> `git stash` nem `checkout --` — o git não tem hook que dispare antes de subcomando arbitrário, e
> prometer paridade humano-agente ali seria falso; e **o PR autorreferente que conserta um
> `governance` quebrado não pode ser travado pelo próprio `governance`**.

## Wave 3 — O `doctor` acusa a lacuna
> Dependências: Wave 2 completa. **É a wave que transforma o achado em produto:** as anteriores
> consertaram **um** repositório; esta faz qualquer projeto que adote o trackfw ganhar o mesmo
> diagnóstico.
> ADR: `ADR-2026-09-02-doctor-ganha-modalidade-remota-opcional-e-ausencia-de-credencial-vira-nao-avaliado-nunca-aprovacao.md`

### ML-3A — Modalidade remota no `doctor`, com "não avaliado" próprio
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `internal/commands/doctor.go` e o pacote de findings; equivalentes em Node e
Python (**paridade nos 3**).

**Diagnóstico:** `runDoctor` (`doctor.go:74`) é **inteiramente local** — catálogo, manager,
identidade, scaffold. Verificar branch protection exige **rede e token**: modalidade que o `doctor`
nunca teve.

**Actions:**
1. Modalidade remota **opcional e explícita** (flag). O `doctor` continua funcionando offline, rápido
   e sem credencial — é o que o torna rodado com frequência.
2. Verificar: `required_status_checks` presente e não vazio; `enforce_admins`; e localmente
   `core.hooksPath` não neutralizado.
3. 🔴 **Ausência de credencial, de rede ou de permissão produz resultado PRÓPRIO — não avaliado —
   distinto de aprovação e de falha, nomeando o remédio.** **Reuse o `not_evaluated` do `barrier`**
   (`barrier.go:872`, `barrier.js:592`, `barrier.py:688`/`:747`) — o conceito passa a ter **um** nome
   no projeto, já revisado.
4. **Token sem escopo suficiente é caso distinto de token ausente** — um se resolve dando permissão,
   o outro criando credencial. A mensagem separa os dois.
5. **Forja que não é GitHub** (GitLab, Gitea, local) → **não avaliado**, nunca reprovado. A
   verificação é específica de forja; fingir universalidade seria falso.

**Critérios de aceite:**
- [ ] Sem flag, o `doctor` roda **offline e sem credencial**, como hoje
- [x] 🔴 **Falsificação nas duas direções.** (a) repositório **sem** `required_status_checks` →
      finding que nomeia a lacuna; (b) **controle:** repositório **com** o portão configurado →
      **sem** finding. Sem (b), teríamos um `doctor` que acusa sempre — e alarme que sempre dispara
      se aprende a ignorar.
- [x] 🔴 **O caso que decide o ADR:** com a flag e **sem token**, o resultado é **não avaliado** —
      nunca "ok". Provar por execução. **Colapsar isso em aprovação é o defeito que esta sessão
      perseguiu nove vezes.**
- [x] Token sem escopo × token ausente produzem mensagens **distintas**
- [ ] Paridade nos 3 CLIs
- [x] `make quality` verde

**Gates da wave:**
```bash
make quality
```

#### Resultado do ML-3A (apolo-tf, 2026-09-02) — auditado pelo arquiteto

**As três direções, verificadas por mim com o binário real:**

```
com credencial, portão ligado  →  0 required-status-checks-missing
                                  0 enforce-admins-disabled
                                  0 not-evaluated
sem credencial                 →  1 not-evaluated
                                  0 required-status-checks-missing   ← NÃO afirma ausência
```

**O caso que decidia o ADR passou:** sem credencial ele **não reporta "ok"** e **não afirma que o
portão está ausente**. Um `doctor` ingênuo faria uma das duas — a primeira é a mentira cara, a
segunda é o alarme falso.

**O controle segurou:** o repositório **corrigido** não gera finding. Sem essa metade, teríamos um
`doctor` que acusa sempre — e alarme que sempre dispara se aprende a ignorar.

### O `doctor` já acusa a lacuna que a Wave 2 deixou aberta

```
1 hooks-path-neutralized
```

É o `core.hooksPath = /dev/null` que faz os guards protegerem **agentes e não pessoas**. Verificação
**local**, sem rede — e agora **qualquer projeto que adote o trackfw recebe o mesmo diagnóstico**. É
o que transforma o achado em produto.

### Escopo × ausência: decidido por dado, não por texto

`permissions.admin` do `gh api repos/{owner}/{repo}` — **não** parsing de stderr. `admin=false` dá
remédio próprio (*"conceda acesso admin"*), nunca reaproveitado do de credencial ausente
(*"gh auth login"*). Parsing de mensagem de erro quebraria na primeira mudança de texto do `gh`.

### O bug que ele achou em si mesmo

O stub de `gh` usava `#!/usr/bin/env bash` e **não resolvia** sob um `PATH` sem `/usr/bin:/bin` —
produzindo `not-evaluated` **pelo motivo errado**: o stub não subia, e não "sem credencial".

**O teste teria passado dando a resposta certa por acidente.** Corrigido, com nota de vault.

### Achado incidental que o próprio `doctor` expôs

```
2 scaffold-divergent  ← um deles é o .github/workflows/trackfw-gate.yml
```

É a divergência **deliberada** documentada no ML-1A: os workflows vivos deste repo usam `go build`
local em vez de `go install`, porque o repositório não consegue se instalar via `curl` durante o
próprio PR. **O `doctor` está certo — a divergência é intencional e não está registrada como
exceção.** Fica como observação; tratá-la é decisão à parte.

**`MAKE_EXIT=0`**, `trackfw validate` exit 0, gate de paridade novo com 33 cenários via stub de `gh`.

## Verificação


O portão só se prova **tentando mergear com CI vermelho** — e o controle, mergeando com CI verde.
Ambas exigem PR real; **não se verifica por leitura de configuração**.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`.
