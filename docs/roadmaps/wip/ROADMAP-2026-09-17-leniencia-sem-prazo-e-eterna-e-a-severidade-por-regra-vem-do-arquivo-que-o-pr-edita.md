---
status: wip
date: 2026-09-17
---

# Roadmap: leniência sem prazo é eterna, e a severidade por regra vem do arquivo que o PR edita

> Created: 2026-09-17 | Status: wip

**Issue:** #387 · **REQ:** `docs/req/REQ-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md`
**ADR:** `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`
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
**169 ausências históricas**. As 8 são de 11 a 17/09 e foram produzidas por nós.

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
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
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
- [ ] Toda regra ancorada em `origin/main`; a mais estrita vence
- [ ] PR que **commita** `rules: {<regra>: off}` não rebaixa a severidade
- [ ] 🔴 Contra-braço: **subir** a severidade no disco **é** respeitado (a regra é "mais estrita
      vence", não "`origin/main` sempre vence")
- [ ] `origin/main` ilegível ⇒ falha fechada, com mensagem própria, distinta de "arquivo ausente"
- [ ] `--scope dw` RC=0 — acrescentar `with:` não mudou nome de job
- [ ] Reconciliação: uma frase por teste novo
- [ ] `go build ./...`, `make test`, `make quality` — RC sem pipe

---

## Wave 2 — Prazo, carve-out e o terceiro interruptor
> Dependências: **Wave 1 auditada.** Os dois MLs tocam `validator.go` — **sequenciais, não paralelos.**

### ML-2A — leniência exige prazo com teto, e o carve-out estrutural
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre **AC2**, **AC3**, **AC4** e o braço (a) do **AC8**.

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
   vivos, não ausência histórica. Calibrar: as 8 ativas entram, as 169 não.

**Critérios de aceite:**
- [ ] `req_roadmap_lifecycle` roteado; `rules: {req_roadmap_lifecycle: error}` passa a ter efeito
- [ ] AC2 nos dois sítios, com teto de horizonte
- [ ] AC3 — `discover` e `init` param de divergir
- [ ] AC4 — lista fechada, critério no código
- [ ] AC8 (a) com contra-braço: a versão sem a correção trata "sem prazo" como leniente
- [ ] Reconciliação: uma frase por teste novo
- [ ] `go build ./...`, `make test`, `make quality` — RC sem pipe

### ML-2B — o terceiro interruptor: repontar caminhos zera a governança
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
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
- [ ] Repontar para vazio reprova, nomeando o caminho
- [ ] Projeto novo legítimo não reprova (contra-braço demonstrado)
- [ ] Decisão de abordagem escrita no código
- [ ] Reconciliação: uma frase por teste novo
- [ ] `go build ./...`, `make test`, `make quality` — RC sem pipe

---

## Wave 3 — Postura deste repositório
> Dependências: **Wave 2 auditada.**

### ML-3A — nosso trackfw.yaml, a medição que fecha e a nota de comportamento
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre **AC6**, **AC7**, **AC9**, **AC10**.

**Ações:**
1. **AC6** — `lenient_until` no `trackfw.yaml`. 🔴 **Medição que fecha a REQ:** `validate` sai **≠ 0**
   apontando **exatamente** as 8 ativas — **e não as 169 históricas**. Se reprovar históricas, o
   carve-out está largo e volta ao ML-2A.
2. **AC7** — corrigir as 8; `validate` volta a 0 **por consistência**, não por leniência.
3. **AC9** — CHANGELOG com nota **no topo da seção**: mudança de comportamento para todo consumidor
   onboardado por `discover`. ⚠️ Não proponha número de versão.

**Critérios de aceite:**
- [ ] AC6 — saída colada, mostrando as 8 e nenhuma histórica
- [ ] AC7 — as 8 corrigidas; `validate` RC=0 por consistência
- [ ] AC9 — nota no topo da seção do CHANGELOG
- [ ] AC10 — `doctor`, `make quality`, `--scope dw`, 8 required checks verdes; nenhum job renomeado

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
