---
status: Done
date: 2026-09-17
---

# REQ: leniência sem prazo é eterna, e a severidade por regra vem do arquivo que o PR edita

> Date: 2026-09-17 | Status: Done

**Issue:** #387
**ADR:** `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`
**Roadmap:** `docs/roadmaps/done/ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md`
**Parecer da Wave 0:** `docs/portabilidade/2026-09-17-threat-model-severidade-da-validacao.md`

> 🔴 **Esta REQ foi reescrita em 2026-09-17, depois da auditoria da Wave 0.** A redação original tinha
> três ACs que **não podiam ser satisfeitos**. O registro do que mudou está na seção final.

## Contexto

O `trackfw validate` lê a severidade e o **escopo** da verificação do `trackfw.yaml` **do próprio
projeto** — arquivo que, num PR, o PR edita.

**Três interruptores**, todos medidos nesta árvore em 2026-09-17:

| # | interruptor | sítio | efeito |
|---|---|---|---|
| 1 | `governance_mode: lenient` | `validator.go:626` **e** `:942` | todas as violations viram warnings, incondicional |
| 2 | `rules: {<regra>: off}` | `validator.go:207-212` → `diskRuleSeverity` (`:218`) | severidade do disco para toda regra, exceto 3 |
| 3 | 🔴 **`req_dir`/`roadmap_dir`/`adr_dirs` para diretório vazio** | config | **zera a governança inteira, mesmo em `strict`** |

O terceiro foi achado na Wave 0 e é o pior: silencioso, e **sobrevive à correção dos outros dois**.
Medido, com `governance_mode: strict` e uma REQ quebrada em `docs/req/`:

```
$ trackfw validate ; echo $?
✓ No violations found.
0
```

Estado atual desta árvore: `trackfw validate` → **177 warnings, exit 0**. `governance-go-install` e
`governance-install-script` — 2 dos 8 required checks — são **exit-0 por construção**.

### Composição das 177 (via `validate --json`)

| qtd | regra | natureza |
|---|---|---|
| 122 | `req_has_adr` | ausência histórica |
| 36 | `req_has_roadmap` | ausência histórica |
| 8 | `adr_orphan` | ausência histórica |
| 🔴 6 | `req_roadmap_lifecycle` | **contradição ativa** — REQ aberta com roadmap em `done/` |
| 🔴 2 | `ref_targets_exist` | **contradição ativa** — caminho de estado obsoleto |
| 1 | `wip_has_req` | ausência histórica |
| 1 | `wip_acceptance` | ausência histórica |
| 1 | *(sem rule)* | `frontmatter_presence`, `validator.go:2307`, fora de `applyRuleTagged` |

**8 ativas, 168 históricas.** As 8 são de 11 a 17/09 e **foram produzidas por nós**.
⚠️ A última linha importa: um warning **sem tag de regra** não pode ser endereçado por chave
`rules:` nenhuma.

### O defeito de produto

O mecanismo de prazo **já existe** (`Config.LenientUntil`, `config.go:430`; `validator.go:355-384`),
mas os pontos de entrada divergem: `init` brownfield escreve `lenient_until` (`scaffold.go:735`),
`discover` **não** escreve (`discover.go:497-499`), e `IsLenient()` (`:381-383`) trata ausência como
`true`. **Falha aberta**: o mecanismo de expiração é burlado por omitir um campo.

## Critérios de Aceite

- [ ] **AC1** — 🔴 **Ancoragem na `origin/main`, NÃO em HEAD.**
      A Wave 0 mediu que o padrão de `credentialGuardAnchoredRules` **não serve aqui**: o comentário
      em `validator.go:200-206` o delimita a edição **não commitada**, e em CI, no evento
      `pull_request`, `actions/checkout` faz checkout de `refs/pull/N/merge` — **HEAD é o merge
      commit, logo HEAD == disco** e a comparação é vácua. Um PR que **commita** o rebaixamento passa.
      A severidade de cada regra passa a comparar **`origin/main`** contra o disco, adotando **a mais
      estrita**.
      **Inclui a mudança de workflow, sem a qual o lado Go não tem o que ler:**
      `trackfw-gate.yml:11` e `trackfw-validate.yml:11` têm `actions/checkout@v7` **sem `with:`**, o
      que traz só o branch do PR — `origin/main` nunca vira ref. Reusar **verbatim** o precedente já
      resolvido neste repositório em `quality.yml:545-570` (fetch com refspec explícito, custo medido
      de 0,09 s), **inclusive a discriminação de dois estados**: *ref ausente* e *arquivo ausente* têm
      mensagens distintas, e a primeira é **fatal**. 🔴 Sem essa distinção, o gate repete o padrão
      "dois estados, um observável" que este projeto já pagou três vezes.
      ⚠️ Acrescentar `with:` **não pode** mudar nome de job: `--scope dw` RC=0 depois.
- [ ] **AC2** — `IsLenient()` exige `lenient_until`; ausência ⇒ **strict**. Cobrir os **dois** sítios
      (`validator.go:626` e `:942`).
      🔴 **E limitar o horizonte.** `lenient_until: 9999-12-31` derrota o AC2 a custo zero, e não há
      justificativa para recusar data ausente e aceitar data absurda — as duas são "leniência para
      sempre" escritas de formas diferentes. Prazo além de um teto (a definir na implementação, com o
      valor **escrito no código e no erro**) é recusado como se ausente fosse.
- [ ] **AC3** — `trackfw discover` escreve `lenient_until` com o **mesmo** default do `init`. Se
      divergirem, o motivo vai escrito.
- [ ] **AC4** — 🔴 **Pré-requisito descoberto na Wave 0, antes do carve-out:**
      `req_roadmap_lifecycle` é anexado direto a `warnings` (`validator.go:787`) e **nunca passa por
      `applyRuleTagged`** — `ruleSeverity()` não é consultado, então a regra **não pode virar violação
      sob configuração nenhuma**. A tag no `--json` é um `TaggedMsg` manual, não roteamento.
      Rotear pelo sistema de severidade com default **`error`** é **ação 1** do microlote; sem isso o
      carve-out é no-op para **6 dos 8** alvos.
      Só então: carve-out **nomeado e fechado**, com o critério escrito no código — contradição entre
      dois artefatos vivos, não ausência de artefato histórico.
- [ ] **AC5** — 🔴 **Novo AC, terceiro interruptor (Canal C da Wave 0).** Repontar
      `req_dir`/`roadmap_dir`/`adr_dirs` para diretório existente e vazio zera a governança **mesmo em
      `strict`** — medido acima. Mesma causa das outras duas (o objeto verificado governa a própria
      verificação), por isso **mesma REQ**, conforme a Regra Dura de Causa Raiz.
      O escopo configurado passa a ser ancorado em `origin/main` como a severidade, **ou** o
      esvaziamento do escopo é detectado e reprovado. A escolha é da implementação; o critério é que
      um PR que só repontar caminhos **não** consiga zerar a contagem.
      Com contra-braço: repositório legitimamente vazio (projeto novo) **não** pode reprovar por isso.
- [ ] **AC6** — `trackfw.yaml` deste repositório recebe `lenient_until`. **Medição que fecha a REQ:**
      `trackfw validate` sai **≠ 0** apontando **exatamente** as 8 ativas — **e não as 168
      históricas**. Se reprovar as históricas, o carve-out está largo demais.
- [ ] **AC7** — As 8 ativas corrigidas; `validate` volta a 0 **por consistência**, não por leniência.
- [ ] **AC8** — Falsificação em duas direções com contra-braço, por interruptor:
      (a) `lenient` sem prazo reprova como strict · com prazo futuro segue leniente fora do carve-out;
      (b) PR que commita `rules: {<regra>: off}` **não** rebaixa · mas **subir** a severidade no disco
      **é** respeitado (a regra é "a mais estrita vence", não "`origin/main` sempre vence");
      (c) repontar caminho para vazio reprova · projeto novo legítimo não reprova.
      🔴 Em cada um, o contra-braço: a versão **sem** a correção passa — provando que o cenário
      discrimina.
- [ ] **AC9** — 🔴 **CHANGELOG com nota de destaque no topo da seção.** É mudança de comportamento
      para todo consumidor onboardado por `discover`. A **decisão de versão** fica para o release.
- [ ] **AC10** — `doctor`, `make quality`, `--scope dw` e os 8 required checks verdes; nenhum job
      renomeado.

## Negative Scope

- ❌ **Não** saldar as 168 pendências históricas. Ficam sob `lenient` com prazo escrito, como dívida
  com data. Decisão de KG, 2026-09-17.
- ❌ **Não** alterar `.github/required-status-checks.txt` nem branch protection (`D = R = W = 8`).
- ❌ **Não** renomear job de workflow.
- ❌ **Não** decidir o número da próxima versão aqui.
- ❌ **Não** tratar o **#277** — mesma família, causa distinta (triagem de 2026-09-17).

### Residuais da Wave 0 declarados e NÃO cobertos — cada um com o motivo

- ❌ **`.trackfw-baseline.json` commitado à força** (driblando o `.gitignore`) suprime mensagens por
  igualdade exata. Fora: o baseline tem propósito legítimo e mudar sua semântica é decisão própria,
  não consequência desta causa. **Fica nomeado como residual, não como esquecimento.**
- ❌ **Manoplas comportamentais** (`stale_wip_days`, `wip_limit`) mudam o que conta como violação sem
  mexer em severidade. Fora: são parâmetros de política, e capturá-los exigiria decidir um valor
  canônico por projeto — escopo próprio.
- ❌ 🔴 **Falsificação de artefato homônimo** (medido por mim em 2026-09-17, ML-2C). O guard de escopo
  compara **identidade do artefato por basename**. Um PR que reponte `req_dir` **e** plante, no destino,
  um arquivo de **mesmo basename** e **integralmente válido** zera a contagem: medi baseline com 2
  violações → `RC=0, violações 0`.
  Fora de escopo, e o motivo é o custo, não o esquecimento: o ataque deixou de ser "só repontar
  caminhos" — que é o que o AC5 exige fechar — e passou a exigir **um artefato forjado e válido por
  artefato real** (122 REQs neste repositório), cada um com ADR e roadmap existentes para não reprovar
  por conta própria. É um diff grande e conspícuo, não uma linha de config.
  A curva de custo do atacante ao longo dos três microlotes: diretório vazio (1 linha) → fachada com
  outro nome (1 arquivo) → **N artefatos homônimos válidos + seus vínculos**. Fechar por conteúdo
  quebraria o contra-braço que importa: edição legítima de uma REQ durante um PR de reestruturação
  viraria falso-positivo.
- ❌ **`req_roadmap_sync` como braço de falso-negativo.** Pelo critério da ADR pertenceria ao
  carve-out (detecta contradição entre dois campos vivos), mas tem **zero ocorrências hoje**, então
  não entra na calibração. Fora por não ser calibrável agora; nomeado para não voltar como surpresa.

## O que mudou nesta reescrita (2026-09-17, pós-auditoria da Wave 0)

| antes | por que não podia ser satisfeito | agora |
|---|---|---|
| AC1 ancorava em **HEAD** | em CI, HEAD é o merge commit ⇒ HEAD == disco ⇒ comparação vácua; o padrão original cobre só edição **não commitada**, e um PR **commita** | ancora em `origin/main`, e inclui o fetch nos dois workflows |
| AC4 fazia carve-out direto | `req_roadmap_lifecycle` não passa por `applyRuleTagged`; não vira violação sob config nenhuma ⇒ no-op para 6 dos 8 | rotear a regra é **ação 1**, antes do carve-out |
| AC5 media "≠0 apontando as 6+2" | inatingível pelo motivo acima | virou AC6, depois do pré-requisito |
| corpus "172" | contagem do texto humano; `--json` dá **177** | 177, com a linha sem tag de regra nomeada |
| — | terceiro interruptor não estava na REQ | **AC5 novo** (repontar caminhos) |
| AC2 só exigia prazo | `lenient_until: 9999-12-31` derrota a custo zero | AC2 passa a limitar o horizonte |

## Linked ADR

ADR: `docs/adr/ADR-2026-09-17-severidade-da-validacao-nao-pode-vir-de-fonte-que-o-objeto-verificado-controla.md`

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-pr-edita.md`
