---
status: Open
date: 2026-09-17
---

# REQ: leniência sem prazo é eterna, e a severidade por regra vem do arquivo que o PR edita

> Date: 2026-09-17 | Status: Open

**Issue:** #387
**ADR:** `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md`

## Contexto

O `trackfw validate` lê a severidade da verificação do `trackfw.yaml` **do próprio projeto** — arquivo
que, num PR, o PR edita. Dois interruptores, ambos medidos nesta árvore em 2026-09-17:

| interruptor | sítio | efeito |
|---|---|---|
| `governance_mode: lenient` | `validator.go:626` **e** `:942` | move todas as violations para warnings, **incondicional** |
| `rules: {<regra>: off}` | `validator.go:207-212` → `diskRuleSeverity` (`:218`) | severidade do disco para toda regra, exceto 3 |

```
$ trackfw validate ; echo $?      # binário compilado desta árvore
172 warning(s)
0
```

**`governance-go-install` e `governance-install-script` — 2 dos 8 required checks — são exit-0 por
construção.** Não podem reprovar PR nenhum.

### O defeito de produto, que é maior que a config deste repositório

O mecanismo de prazo **já existe** (`Config.LenientUntil`, `config.go:430`; `validator.go:355-384`).
Os dois pontos de entrada divergem:

| comando | escreve | `IsLenient()` |
|---|---|---|
| `trackfw init` brownfield | `lenient` + `lenient_until` (`scaffold.go:735`) | expira |
| `trackfw discover` | `lenient` **sem prazo** (`discover.go:497-499`) | 🔴 **eterno** |

`validator.go:381-383`: prazo ausente ⇒ `return true`. **Falha aberta.** Quem entrou pelo `discover`
está em leniência infinita sem ter escolhido.

## Critérios de Aceite

- [ ] **AC1** — 🔴 **Ancoragem em HEAD da severidade por regra, ANTES de qualquer aperto do lenient.**
      `ruleSeverity` (`validator.go:207`) generaliza o padrão de `credentialGuardAnchoredRules`:
      compara a severidade declarada em **HEAD** com a do **disco** e adota **a mais estrita**, para
      **todas** as regras. ⚠️ **Ordem é normativa** (ADR): apertar o `lenient` sem isto apenas troca o
      interruptor geral pelo interruptor por regra, para as ~21 regras hoje não ancoradas.
- [ ] **AC2** — `IsLenient()` passa a **exigir** `lenient_until`. `governance_mode: lenient` sem data
      ⇒ tratado como **strict**. Falha fechada: ausência de campo nunca concede permissão.
      🔴 Cobrir os **dois** sítios de `IsLenient()` — `validator.go:626` e `:942`. O issue cita um só.
- [ ] **AC3** — `trackfw discover` passa a escrever `lenient_until` com prazo default, como o `init`
      já faz (`scaffold.go:735`). Os dois pontos de entrada param de divergir.
- [ ] **AC4** — **Carve-out estrutural.** Conjunto **nomeado e fechado** de regras que o `lenient`
      não cobre. Critério de pertencimento, escrito no código: a regra aponta **contradição entre dois
      artefatos vivos** (ex.: REQ aberta cujo roadmap está em `done/`), não ausência de artefato
      histórico. As 8 ocorrências ativas medidas hoje entram; as 157 históricas, não.
- [ ] **AC5** — `trackfw.yaml` deste repositório recebe `lenient_until` declarado, e o `validate`
      passa a **reprovar** as 8 inconsistências ativas. 🔴 **Medição que fecha:** `trackfw validate`
      sai **≠ 0** apontando exatamente as 6 REQs abertas com roadmap em `done/` e os 2 caminhos de
      estado obsoletos — **e não as 157 históricas**.
- [ ] **AC6** — Falsificação em duas direções, com contra-braço:
      (a) config com `lenient` **sem** prazo reprova como strict;
      (b) config com `lenient` **e** prazo futuro continua leniente nas regras **não** carve-out;
      (c) 🔴 contra-braço: a versão **sem** a correção trata (a) como leniente — provando que o
      cenário discrimina.
- [ ] **AC7** — Falsificação da ancoragem: um PR que commite `rules: {<regra>: off}` **não** rebaixa
      a severidade dessa regra; a de HEAD prevalece por ser mais estrita. Com contra-braço na direção
      oposta: subir a severidade no disco **é** respeitado (a mais estrita vence, não "HEAD sempre").
- [ ] **AC8** — 🔴 **CHANGELOG com nota de destaque**, no topo da seção: é **mudança de
      comportamento** para todo consumidor onboardado por `discover`, cujo `validate` passa a reprovar.
      A decisão de versão (minor, não patch) fica para o release — mas a nota é entregável desta REQ.
- [ ] **AC9** — `trackfw doctor`, `make quality` e os 8 required checks verdes ao fim; nenhum job
      renomeado.

## Negative Scope

- ❌ **Não** saldar as 157 pendências históricas (122 REQ sem ADR, 35 REQ sem roadmap). Ficam sob
  `lenient` com prazo escrito, registradas como dívida com data. Decisão de KG em 2026-09-17.
- ❌ **Não** alterar `.github/required-status-checks.txt` nem branch protection. `D = R = W = 8`,
  fechado em 2026-09-16.
- ❌ **Não** renomear job de workflow — contrato com `required_status_checks`.
- ❌ **Não** decidir o número da próxima versão aqui. A ADR registra que é mudança de comportamento;
  a versão sai no protocolo de release.
- ❌ **Não** tratar o #277 (corpus do barrier acoplado a `docs/roadmaps/**` do disco). Mesma família,
  **causa distinta** — triagem de 2026-09-17: corrigir um não move o outro.

## Linked ADR

ADR: `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`
