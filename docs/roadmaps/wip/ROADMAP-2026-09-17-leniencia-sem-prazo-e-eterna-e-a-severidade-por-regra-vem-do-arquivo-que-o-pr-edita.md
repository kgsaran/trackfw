---
status: wip
date: 2026-09-17
---

# Roadmap: leniência sem prazo é eterna, e a severidade por regra vem do arquivo que o PR edita

> Created: 2026-09-17 | Status: wip

REQ: docs/req/REQ-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md
ADR: docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md

**Issue:** #387
**Parecer da Wave 0:** `docs/portabilidade/2026-09-17-threat-model-severidade-da-validacao.md`

## Diagnóstico

`trackfw validate` nesta árvore → **177 warnings, exit 0**. `governance-go-install` e
`governance-install-script` — 2 dos 8 required checks — são **exit-0 por construção**.

**Três interruptores**, o terceiro achado na Wave 0:

| # | interruptor | sítio |
|---|---|---|
| 1 | `governance_mode: lenient` | `validator.go:626` e `:942` |
| 2 | `rules: {<regra>: off}` | `validator.go:207-212` → `diskRuleSeverity` (`:218`) |
| 3 | 🔴 `req_dir`/`roadmap_dir`/`adr_dirs` para diretório vazio | zera a governança **mesmo em `strict`** |

Composição das 177: **8 contradições ativas** (6 `req_roadmap_lifecycle`, 2 `ref_targets_exist`),
**168 ausências históricas**. As 8 são de 11 a 17/09 e foram produzidas por nós.

---

## Wave 0 — Threat model
> Dependências: nenhuma.

### ML-0A — quem continua conseguindo afrouxar a própria verificação depois desta REQ
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-17) · **Papel:** `hades-tf`

**Critérios de aceite:**
- [x] Os cinco eixos respondidos com evidência de leitura
- [x] Veredito sobre o fetch do `actions/checkout` (eixo 3) — **achado bloqueante**
- [x] Veredito sobre caminhos de configuração fora de `governance_mode`/`rules` (eixo 1) — **achado bloqueante**
- [x] Vetores não fechados nomeados; residual declarado
- [x] Nenhuma linha de implementação escrita

**Veredito da auditoria — três achados, todos reverificados por mim no código:**

| achado | minha verificação | consequência |
|---|---|---|
| Ancoragem em HEAD é vácua em CI | `trackfw-gate.yml:11` e `trackfw-validate.yml:11` têm `actions/checkout@v7` **sem `with:`**; nenhum workflow de governança usa `fetch-depth`; o comentário em `validator.go:200-206` delimita o padrão a edição **não commitada** | **AC1 reescrito** — ancora em `origin/main` |
| `req_roadmap_lifecycle` nunca vira violação | `validator.go:787` anexa direto a `warnings`; `ruleSeverity()` não é consultado | **AC4 ganha pré-requisito** — rotear antes do carve-out |
| Repontar caminhos zera tudo em `strict` | fixture com `strict` + dirs vazios + REQ quebrada → `✓ No violations found`, RC=0 | **AC5 novo**, mesma REQ |

🔴 **Nenhum gate verde teria pego os três** — todos são defeitos **no remédio** que eu havia escrito,
não no código a corrigir. É o terceiro ciclo seguido em que a Wave 0 paga por si.

**Gate da wave 0:** executado, RC=0.

---

## Wave 1 — Ancoragem (precede tudo, por decisão da ADR)
> Dependências: **Wave 0 auditada.**

### ML-1A — severidade por regra ancorada em `origin/main`, e o fetch que a torna possível
**Status:** ✅ Concluído **via corretivo ML-1B** — a primeira entrega foi REPROVADA por mim (4 defeitos, registro preservado abaixo e no commit); o AC1 fecha com as duas entregas somadas · **Papel:** `apolo-tf`
Cobre **AC1** e o braço (b) do **AC8**.

🔴 **Vem primeiro por decisão normativa da ADR.** Apertar o `lenient` sem isto reabre a superfície
`rules: {<regra>: off}` para as ~21 regras não ancoradas.

**Arquivos afetados:**
- `internal/validator/validator.go` — `ruleSeverity` (`:207-212`), `diskRuleSeverity` (`:218`)
- `internal/validator/validator_credential_guard_integrity.go` — comparador existente (`:238`)
- `.github/workflows/trackfw-gate.yml` (`:11`) e `.github/workflows/trackfw-validate.yml` (`:11`)
- testes em `internal/validator/`

**Ações:**
1. A severidade de **toda** regra compara **`origin/main`** contra o disco e adota **a mais estrita**.
   ⚠️ **Não** use HEAD: em `pull_request`, HEAD é o merge commit e HEAD == disco — medido na Wave 0.
2. **Fetch nos dois workflows.** Sem ele o lado Go não tem `origin/main` para ler. Reusar **verbatim**
   o precedente de `quality.yml:545-570`: refspec explícito, custo medido de 0,09 s, e 🔴 **duas
   mensagens distintas** para *ref ausente* (fatal) e *arquivo ausente* (normal em PR que cria o
   arquivo). Sem essa distinção repete-se o padrão "dois estados, um observável" que este projeto já
   pagou três vezes.
3. Comportamento sem `origin/main` legível: **falhar fechada**, com a razão nomeada.
4. Reaproveite o comparador existente; se a lista `credentialGuardAnchoredRules` perder função, diga
   no relatório em vez de mantê-la por inércia.

**Critérios de aceite:**
- [x] Toda regra ancorada em `origin/main`; a mais estrita vence
- [x] PR que **commita** `rules: {<regra>: off}` não rebaixa a severidade
- [x] 🔴 Contra-braço: **subir** a severidade no disco **é** respeitado (a regra é "mais estrita
      vence", não "`origin/main` sempre vence")
- [x] `origin/main` ilegível ⇒ falha fechada, com mensagem própria, distinta de "arquivo ausente"
- [x] `--scope dw` RC=0 — acrescentar `with:` não mudou nome de job
- [x] Reconciliação: uma frase por teste novo
- [x] `go build ./...`, `make test`, `make quality` — RC sem pipe

---


### ML-1B — corretivo do ML-1A: quatro defeitos bloqueantes na ancoragem
**Status:** ✅ Concluído (make quality RC=0, 2026-09-17) · **Papel:** `apolo-tf`
Aberto pela auditoria do ML-1A em 2026-09-17. **O ML-1A está REPROVADO** — o código está commitado
para não se perder, não porque foi aceito.

O desenho central está certo e **fica**: ancorar em `origin/main`, stricter-wins, três braços de
discriminação no workflow. Os quatro defeitos abaixo foram medidos por mim.

#### 1. 🔴 `make quality` RC=2 — o ML foi marcado ✅ sem esperar o gate

O relatório diz "em background, não pôde ser aguardado". Rodei: **RC=2**. A falha que importa:

```
FAIL [falsify/credential-guard-anchoring-combined-edit/detected]: saiu com 0, esperava != 0
```

É o **cenário de falsificação da ancoragem de credential-guard** — a prova de que o mecanismo detecta
a edição combinada. Ao remover `credentialGuardRuleSeverity()` e trocar a semântica, o ML **derrubou
a prova de uma garantia de segurança existente**. Também reprovou a guarda de conjunto do
falsify-parallel (`rotulo esperado AUSENTE: integration-assets/direction-b-shim-absent`).
⚠️ Não "conserte" adaptando o cenário até passar: entenda se a garantia ainda vale e, se não valer,
diga **o que se perdeu**.

#### 2. 🔴 Regressão do #376, fechado hoje

```
$ ./bin/trackfw doctor
2 finding(s) -- ... 2 scaffold-divergent
[scaffold-divergent] .github/workflows/trackfw-gate.yml
[scaffold-divergent] .github/workflows/trackfw-validate.yml
```

Os dois workflows são **gerados por builders** (`buildGitHubActionsWorkflowContent` e
`BuildDiscoverGitHubActionsWorkflowContent`, braço de produtor). Editar o disco sem editar o builder
faz o `doctor` prescrever `trackfw update`, que **desfaz o fetch**. É exatamente o defeito do #376,
que fechamos hoje com "no mismatches found". **O passo de fetch tem de sair do builder.**

#### 3. 🔴 Quebra o CI de consumidor inocente — o mais grave

Medido, em fixture com remote `origin` mas **sem** ref `origin/main` (estado normal de checkout raso):

```
✗ severity anchor unavailable: origin/main ref could not be read — ...
Error: 1 violation(s) found
RC_REAL=1
```

Qualquer projeto nessa situação passa a **reprovar**. Não é hipótese: é todo CI com checkout raso que
não faça o fetch, e todo clone `--depth 1`. A ADR autorizou mudança de comportamento para quem usa
`lenient` sem prazo — **não** autorizou reprovar quem nunca configurou nada.

**Direção (decidida por mim, verifique antes de seguir):** separar as duas coisas que hoje estão
juntas. Âncora indisponível ⇒ **cair nos defaults embutidos** (que é o comportamento seguro, e a
mensagem já diz isso) **+ warning**. A **violação** só quando o `trackfw.yaml` do disco
**efetivamente enfraquece** alguma regra em relação ao default embutido. Assim o bypass continua
fechado — um PR que tente rebaixar sem âncora não consegue rebaixar — e quem não mexeu em nada não
reprova. Se você medir que essa separação não fecha o bypass, **diga e proponha outra**; não a adote
por obediência.

#### 4. 🔴 `origin/main` com o nome do branch fixo

Consumidor cujo branch default seja `master`, `trunk` ou `develop` cai permanentemente no braço
"ref ilegível". Derive o branch default (`origin/HEAD`, config do repositório) em vez de fixar `main`,
e **declare o fallback** quando não der para derivar.

**Critérios de aceite:**
- [x] `make quality` **RC=0**, medido sem pipe e **colado no relatório** — inclusive
      `falsify/credential-guard-anchoring-combined-edit` e a guarda de conjunto
- [x] Se alguma garantia de credential-guard mudou, **o que se perdeu está escrito**
- [x] `./bin/trackfw doctor` → **0 `scaffold-divergent`**; o fetch sai dos **builders**, não só do disco
- [x] Fixture com `origin` sem `origin/main` e `trackfw.yaml` **sem enfraquecimento** ⇒ **não reprova**
- [x] Contra-braço: fixture com `origin` sem `origin/main` e `rules: {<regra>: off}` no disco ⇒
      **a regra não é rebaixada** (o bypass continua fechado)
- [x] Branch default derivado, não fixo; fallback declarado
- [x] `--scope dw` RC=0 · `go build ./...` · `make test`
- [x] Reconciliação: uma frase por teste novo ou alterado
## Wave 2 — Prazo, carve-out e o terceiro interruptor
> Dependências: **Wave 1 auditada.** Os dois MLs tocam `validator.go` — **sequenciais, não paralelos.**

### ML-2A — leniência exige prazo com teto, e o carve-out estrutural
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
Cobre **AC2**, **AC3**, **AC4** e o braço (a) do **AC8**.

**Ação 0 — defeito achado por mim na auditoria do ML-1B, barato e do mesmo arquivo:**
`deriveOriginDefaultBranch` filtra `line == "origin/HEAD"`, **mas esse valor nunca aparece**. O
`%(refname:short)` de `refs/remotes/origin/HEAD` é **`origin`**, não `origin/HEAD` — medido:

```
$ git for-each-ref --format='%(refname) -> %(refname:short)' refs/remotes/origin/
refs/remotes/origin/HEAD -> origin
```

Consequência: num repositório com `origin/HEAD` definido e branch default de nome incomum
(`trunk`, `develop`), a lista fica com 2 entradas, o braço "ref único inequívoco" não dispara e a
derivação **falha fechada sem necessidade**. Direção segura — não é bypass —, mas é falso-positivo.
Corrija o filtro para `origin` e **acrescente um teste** que prove a derivação de branch único com
`origin/HEAD` presente. Nenhum gate atual pega isto.

**Ações — a 1 é pré-requisito das demais:**
1. 🔴 **Rotear `req_roadmap_lifecycle` por `applyRuleTagged`**, com default `error`.
   Hoje `validator.go:787` anexa direto a `warnings` e `ruleSeverity()` nunca é consultado: a regra
   **não pode virar violação sob configuração nenhuma**. Sem esta ação, o carve-out é no-op para
   **6 dos 8** alvos e o AC6 é inatingível.
2. **AC2** — `IsLenient()` exige `lenient_until`; ausência ⇒ strict. Cobrir os **dois** sítios
   (`:626` e `:942`). **E limitar o horizonte**: `lenient_until: 9999-12-31` derrota o AC2 a custo
   zero. O teto vai **escrito no código e na mensagem de erro**.
3. **AC3** — `discover` escreve `lenient_until` com o **mesmo** default do `init` (`scaffold.go:735`).
4. **AC4** — carve-out nomeado e fechado, critério escrito no código: contradição entre artefatos
   vivos, não ausência histórica. Calibrar: as 8 ativas entram, as 168 não.

**Critérios de aceite:**
- [x] Ação 0 — filtro `origin/HEAD` corrigido para `origin`, com teste de branch único
- [x] `req_roadmap_lifecycle` roteado; `rules: {req_roadmap_lifecycle: error}` passa a ter efeito
- [x] AC2 nos dois sítios, com teto de horizonte
- [x] AC3 — `discover` e `init` param de divergir
- [x] AC4 — lista fechada, critério no código
- [x] AC8 (a) com contra-braço: a versão sem a correção trata "sem prazo" como leniente
- [x] Reconciliação: uma frase por teste novo
- [x] `go build ./...`, `make test`, `make quality` — RC sem pipe

**Auditoria de Zeus (2026-09-17) — medido nesta árvore, não aceito do relatório:**

Com `lenient_until: 2027-06-01` aplicado temporariamente ao `trackfw.yaml` (restaurado byte a byte
em seguida), `validate --json` devolveu **exatamente**:

```
violations: 8   warnings: 168
  VIOL    6  req_roadmap_lifecycle
  VIOL    2  ref_targets_exist
```

Calibração exata: **zero** regra histórica vazou para violations. `make quality` **RC=0**,
`doctor` sem `scaffold-divergent`.

**Decisão de desenho que o agente expôs e eu valido:** violação **sem tag de regra**
(`frontmatter_presence`, `Rule: ""`) vai para warnings sob lenient. Correto — é uma das 168
históricas, e o carve-out é uma lista **nomeada**; item sem nome não pode pertencer a ela.

### ML-2B — o terceiro interruptor: repontar caminhos zera a governança
**Status:** ✅ Concluído **via corretivo ML-2C** — a primeira entrega foi REPROVADA por mim (discriminante `len(files) == 0`); o AC5 fecha com as duas somadas · **Papel:** `apolo-tf`
Cobre **AC5** e o braço (c) do **AC8**. Dependência: **ML-2A auditado** (mesmo arquivo).

**Medido na Wave 0 e por mim:** `governance_mode: strict`, `req_dir`/`roadmap_dir`/`adr_dirs`
apontando para diretórios existentes e vazios, REQ quebrada em `docs/req/` → `✓ No violations found`,
RC=0.

**Ações:**
1. Um PR que **só reponte caminhos** não pode zerar a contagem. Duas direções aceitáveis: ancorar o
   escopo configurado em `origin/main`, como o ML-1A fez com a severidade; **ou** detectar o
   esvaziamento e reprovar. Escolha uma e **escreva o porquê**.
2. 🔴 **Contra-braço obrigatório:** projeto legitimamente novo, com diretórios vazios de verdade,
   **não** pode reprovar por isso. Sem este braço a correção quebra todo `trackfw init`.

**Critérios de aceite:**
- [x] Repontar para vazio reprova, nomeando o caminho
- [x] Projeto novo legítimo não reprova (contra-braço demonstrado)
- [x] Decisão de abordagem escrita no código
- [x] Reconciliação: uma frase por teste novo
- [x] `go build ./...`, `make test`, `make quality` — RC sem pipe

---


### ML-2C — corretivo do ML-2B: o discriminante é perda de cobertura, não vazio
**Status:** ✅ Concluído (auditado por Zeus em 2026-09-17) · **Papel:** `apolo-tf`
Aberto pela auditoria do ML-2B em 2026-09-17. **Fecha o AC5 de verdade.**

O ML-2B está certo na direção (ancorar o escopo em `origin/main`, reusando o maquinário do ML-1A) e
**fica**. O defeito é o **discriminante**: `scopeRedirectViolations` só reprova quando o diretório
novo tem **zero** artefatos (`validator.go:298`, `:309`, `:325` — `len(files) == 0`).

**Medido por mim, com repositório real e `origin/main` fetchado:**

| passo | resultado |
|---|---|
| baseline, caminhos iguais ao `origin/main` | 3 violações, todas de `docs/req/REQ-quebrada.md` |
| `req_dir` repontado para `fachada/req` com **um** arquivo dentro | **as 3 somem**, e **nenhum `scope redirect` é emitido** |

Um arquivo de fachada custa uma linha e derrota o guard inteiro. O AC5 exige que *"um PR que só
repontar caminhos não consiga zerar a contagem"* — e ele consegue.

🔴 **O vazio é o sintoma, não o ataque.** O ataque é **deixar de enxergar artefato que antes se
enxergava**. Diretório vazio é apenas o caso em que isso acontece de forma mais grosseira.

**Ação — trocar o discriminante:**
1. Comparar a **cobertura**: o conjunto de artefatos de governança visíveis sob a config **ancorada**
   (`origin/main`) contra o visível sob a config **do disco**. Artefato que existia em `origin/main`
   dentro do escopo ancorado e **deixou de estar em qualquer escopo do disco** ⇒ violação,
   **nomeando os artefatos perdidos** (ou os primeiros N, com a contagem total).
2. **Reestruturação legítima continua passando**, e este é o contra-braço que decide: mover
   `docs/req/` para `requisicoes/` **levando os arquivos junto** não perde cobertura. Compare por
   **identidade do artefato** (basename, ou conteúdo), não por caminho — senão toda renomeação de
   pasta vira violação e ninguém mais reorganiza nada.
3. Manter o comportamento já estabelecido para os outros dois estados: **sem `origin`** ⇒ silencioso;
   **âncora ilegível** ⇒ warning, sem violação de redirect. Não regrida isso.
4. Reavaliar a lacuna que o ML-2B declarou (`adr_dirs: []` explicitamente vazio): sob o discriminante
   de cobertura ela provavelmente **fecha sozinha** — se os ADRs saíam do escopo, houve perda. Meça e
   diga.

**Critérios de aceite:**
- [x] 🔴 Fixture do arquiteto fecha: `req_dir` repontado para diretório com **um arquivo de fachada**
      ⇒ **violação**, nomeando o artefato que deixou de ser visto
- [x] Repontar para diretório **vazio** continua reprovando (não regredir o ML-2B)
- [x] 🔴 Contra-braço: reestruturação legítima **com os arquivos movidos junto** ⇒ **não reprova**
- [x] Projeto novo sem `origin` ⇒ silencioso · âncora ilegível ⇒ warning, sem violação de redirect
- [x] Veredito medido sobre `adr_dirs: []`
- [x] `go build ./...`, `make test`, **`make quality` RC=0**, `doctor` sem `scaffold-divergent`
- [x] Reconciliação: uma frase por teste novo ou alterado

**Auditoria de Zeus (2026-09-17) — medido por mim, não aceito do relatório:**

Rodei o meu próprio fixture de fachada: agora **reprova**, nomeando o artefato perdido —
*"1 artifact(s) committed in origin/main ... no longer visible ...: REQ-quebrada.md"*.
`make quality` **RC=0**, `doctor` sem `scaffold-divergent`, e **0** `scope redirect` neste
repositório — sem falso-positivo.

🔴 **Residual que medi e que fica NOMEADO na REQ:** fachada de **mesmo basename** e
integralmente válida zera a contagem (`RC=0`). Fora de escopo por custo, não por esquecimento —
deixou de ser "só repontar caminhos" e passou a exigir um artefato forjado e válido **por**
artefato real, com ADR e roadmap existentes. A curva de custo do atacante ao longo dos três
microlotes: diretório vazio (1 linha) → fachada com outro nome (1 arquivo) → **N homônimos
válidos + vínculos**.

**Crédito ao executor:** ele achou e fechou sozinho um bloqueador que eu não tinha visto —
`roadmap_dir: .` fazia o *walk* absorver os arquivos originais e derrotava a própria união de
basenames. Filtro de ancestral estrito, com teste próprio.
## Wave 3 — Postura deste repositório
> Dependências: **Wave 2 auditada.**

### ML-3A — nosso trackfw.yaml, a medição que fecha e a nota de comportamento
**Status:** ✅ Concluído · **Papel:** `apolo-tf`
Cobre **AC6**, **AC7**, **AC9**, **AC10**.

**Ações:**
1. **AC6** — `lenient_until` no `trackfw.yaml`. 🔴 **Medição que fecha a REQ:** `validate` sai **≠ 0**
   apontando **exatamente** as 8 ativas — **e não as 168 históricas**. Se reprovar históricas, o
   carve-out está largo e volta ao ML-2A.
2. **AC7** — corrigir as 8; `validate` volta a 0 **por consistência**, não por leniência.
3. **AC9** — CHANGELOG com nota **no topo da seção**: mudança de comportamento para todo consumidor
   onboardado por `discover`. ⚠️ Não proponha número de versão.

**Critérios de aceite:**
- [x] AC6 — saída colada, mostrando as 8 e nenhuma histórica
- [x] AC7 — as 8 corrigidas; `validate` RC=0 por consistência
- [x] AC9 — nota no topo da seção do CHANGELOG
- [x] AC10 — `doctor`, `make quality`, `--scope dw`, 8 required checks verdes; nenhum job renomeado

**Comandos de validação:**
```bash
go build ./... ; echo "RC=$?"
make test ; echo "RC=$?"
make build && ./bin/trackfw validate ; echo "VALIDATE_RC=$?"
./bin/trackfw doctor | head -1
python3 scripts/check-required-status-checks.py --scope dw ; echo "RC=$?"
make quality ; echo "RC=$?"
```

## Legenda de status

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado


**Auditoria de Zeus (2026-09-17) — medido por mim:**

`validate` RC=**0 por consistência**, 168 warnings, **0 violações**. `make quality` RC=0,
`doctor` sem `scaffold-divergent`, `--scope dw` RC=0.

Conferi as 8 uma a uma contra a realidade: a REQ-2026-09-03 continua **Open** porque o roadmap
dela está em `blocked/` — trabalho bloqueado, não concluído; as outras sete viraram `Done` e têm
roadmap em `done/`. Nenhuma foi fechada por conveniência.

🔴 **Corrigi um artefato que a entrega apenas declarou:** `ROADMAP-2026-09-16-run-capture`
estava em `done/` carregando o scaffold intocado do `roadmap new` — dois MLs `⬜ Pendente` e o
gate placeholder `exit 1`. Terceira ocorrência do padrão hoje; nenhuma regra do `validate` pega.

**Decisão que o executor tomou e eu aceito, com ressalva:** `lenient_until: 2027-12-31` são ~15
meses. É consequência lógica da decisão de KG de não saldar as 168 históricas — 30 dias as faria
bloquear em outubro. Dentro do teto de 730 dias. ⚠️ Mas é prazo longo, e este projeto tem
histórico de exceção temporária que vira permanente: **vale revisitar antes de 2027**.