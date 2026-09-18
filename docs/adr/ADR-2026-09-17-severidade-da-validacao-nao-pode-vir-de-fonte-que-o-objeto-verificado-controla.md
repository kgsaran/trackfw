---
status: Accepted
date: 2026-09-17
author: "Zeus (arquiteto) / KG (decisão)"
---

# ADR: severidade da validação não pode vir de fonte que o objeto verificado controla

> Date: 2026-09-17 | Status: Accepted

## Context

O `trackfw validate` decide **se uma violação bloqueia** lendo o `trackfw.yaml` do próprio projeto.
Num PR, esse arquivo é editável pelo PR. O objeto verificado governa a severidade da sua própria
verificação.

Isso tem hoje dois interruptores, medidos em 2026-09-17 nesta árvore:

| interruptor | sítio | efeito |
|---|---|---|
| `governance_mode: lenient` | `internal/validator/validator.go:626` e `:942` | `warnings = append(warnings, violations...); violations = nil` — **incondicional** |
| `rules: {<regra>: off}` | `validator.go:207-212` → `diskRuleSeverity` (`:218`) | severidade lida do disco para **toda** regra, exceto 3 |

As três exceções (`credentialGuardAnchoredRules`, ancoradas em
`validator_credential_guard_integrity.go:238`) comparam HEAD contra disco e adotam a mais estrita.
O comentário no código diz por quê: *essas 3 regras poderiam ser silenciadas pela mesma edição não
commitada que elas existem para pegar*. **O padrão correto já existe no produto, aplicado a 3 de ~24
regras.**

Medição nesta árvore, com binário compilado do fonte:

```
$ trackfw validate ; echo $?
177 warning(s)   # via --json; a saida humana colapsa algumas linhas
0
```

Zero violações, exit 0. Consequência: `governance-go-install` e `governance-install-script` — 2 dos 8
required status checks — **são exit-0 por construção**. Prova contrafactual involuntária de
2026-09-16: quando um gate defeituoso destruiu o `trackfw.yaml` e removeu o `lenient`, o CI reprovou
com 26 REQs.

### O achado que muda a natureza do problema

O mecanismo de prazo **já existe e está implementado** (`Config.LenientUntil`, `config.go:430`,
`validator.go:355-384`). Mas os dois pontos de entrada do produto divergem:

| comando | escreve | efeito de `IsLenient()` |
|---|---|---|
| `trackfw init` (brownfield) | `governance_mode: lenient` **+ `lenient_until: <data>`** (`scaffold.go:735`) | expira na data |
| `trackfw discover` | `governance_mode: lenient`, **sem prazo** (`discover.go:497-499`) | 🔴 **leniência eterna** |

`IsLenient()` (`validator.go:381-383`) trata prazo ausente como `true`. **Falha aberta.** O mecanismo
de expiração existe e é burlado apenas por omitir um campo — e quem entrou pelo `discover` está nesse
estado sem ter escolhido. Isto **não é higiene de config deste repositório; é defeito de produto que
atinge todo consumidor onboardado por `discover`.**

## Decision

**A severidade de uma verificação não pode ser determinada exclusivamente por uma fonte que o objeto
verificado controla.** Três consequências normativas:

1. **Leniência sem prazo não é leniência.** `IsLenient()` passa a **exigir** `lenient_until`. Config
   com `governance_mode: lenient` e sem data é tratada como **strict**. O `discover` passa a escrever
   um prazo default, como o `init` já faz. 🔴 **Falha fechada**: a ausência de um campo nunca concede
   permissão.
2. **Um conjunto nomeado de regras estruturais não é coberto pelo `lenient`.** São as que detectam
   **inconsistência ativa** — estado que o repositório está criando agora, não dívida herdada.
   Critério de pertencimento: a regra aponta contradição entre dois artefatos vivos (ex.: REQ aberta
   cujo roadmap está em `done/`), e não ausência de artefato histórico.
3. **A severidade por regra é ancorada em HEAD.** O padrão de `credentialGuardAnchoredRules`
   generaliza para todas as regras: compara-se a severidade declarada em HEAD com a do disco e adota-se
   **a mais estrita**. Sem isto, sair do `lenient` apenas troca o interruptor geral pelo interruptor
   por regra.

🔴 **As decisões 1 e 3 são ordenadas: 3 antes de qualquer aperto de 1.** Apertar o `lenient` sem
ancorar a severidade por regra reabre a superfície `rules: {<regra>: off}` para as ~21 regras que não
estão na lista de credential-guard — troca-se um buraco por outro.

### Postura deste repositório

Carve-out estrutural + prazo declarado. As regras da decisão 2 passam a bloquear imediatamente; as
168 pendências históricas (122 `req_has_adr`, 36 `req_has_roadmap`, 8 `adr_orphan`, 1 `wip_has_req`, 1 `wip_acceptance`, 1 sem tag de regra) permanecem sob `lenient` com
`lenient_until` **escrito**. Justificativa: das 177 pendências, **8 são inconsistência ativa e
recente** — 6 REQs abertas cujo roadmap já está em `done/`, datadas de 11 a 17/09, **todas produzidas
por nós**, incluindo a do trabalho fechado hoje. O `lenient` está escondendo defeito que estamos
criando agora, não só dívida de junho.

## Consequences

**Positivas**

- Os dois checks de governança obrigatórios passam a poder reprovar — hoje não podem.
- O mecanismo de expiração deixa de ser burlável por omissão.
- O padrão de ancoragem em HEAD, já validado em 3 regras, cobre todas.
- A inconsistência ativa que produzimos volta a ser visível no PR que a produz.

**Negativas, e assumidas**

- 🔴 **Mudança de comportamento para consumidores.** Quem entrou por `discover` tem `lenient` sem
  prazo; após esta decisão, o `validate` desses projetos passa a reprovar. Exige nota de destaque no
  CHANGELOG e decisão de versionamento no momento do release — provavelmente **minor**, não patch,
  porque patch não sinaliza mudança de comportamento. A decisão de versão fica para o release, não
  para esta ADR.
- O carve-out é uma exceção a mais na configuração, e exceção tende a virar permanente. Mitigado por
  ser **lista nomeada e fechada**, com critério escrito de pertencimento.
- As 168 pendências históricas continuam invisíveis até o prazo. Registrado como dívida com data.

## Alternatives Considered

- **Alinhar só o `discover` ao `init`** (escrever o prazo, manter `IsLenient()` tolerante à ausência).
  Rejeitado: não quebra ninguém, mas deixa a fresta aberta — quem apagar a linha volta à leniência
  infinita, e as configs já instaladas não mudam. Trata o sintoma no gerador e deixa a decisão
  fail-open onde ela é tomada.
- **Janela de migração** (avisar agora, virar strict numa versão anunciada). Rejeitado por KG: é mais
  gentil, mas exige carregar um estado intermediário e lembrar de fechá-lo — e este projeto tem
  histórico documentado de exceção temporária que virou permanente, sendo o próprio `lenient` deste
  repositório o exemplo (comentário diz "durante onboarding"; o onboarding terminou há meses).
- **Sair do `lenient` de vez**, saldando as 176. Rejeitado por custo/benefício: 168 são dívida
  histórica de escrita, não de código, e consumiriam a sessão sem entregar produto. O carve-out
  captura os 8 casos que importam agora.
- **Manter como está.** Rejeitado: dois required checks que não podem reprovar são pior que dois
  checks ausentes, porque aparentam cobertura.

## Adendo de 2026-09-17 — corrigido pela auditoria da Wave 0

Três correções a esta ADR, todas medidas. **Elas mudam decisões, não só números.**

1. 🔴 **A ancoragem é em `origin/main`, não em HEAD — a decisão 3 acima está errada como escrita.**
   O padrão de `credentialGuardAnchoredRules` cobre edição **não commitada**, e o próprio comentário
   em `validator.go:200-206` diz isso com todas as letras. Em CI, no evento `pull_request`,
   `actions/checkout` traz `refs/pull/N/merge`: **HEAD é o merge commit, logo HEAD == disco**, e a
   comparação é vácua. Um PR que **commita** o rebaixamento passaria — e um PR commita, por
   definição. A decisão 3 passa a ler **`origin/main`**, o que exige fetch explícito nos workflows.
   Precedente já resolvido neste repositório, a reusar verbatim: `quality.yml:545-570`, inclusive a
   discriminação entre *ref ausente* e *arquivo ausente*, com a primeira fatal.

2. 🔴 **Existe um terceiro interruptor**, que esta ADR não previa: repontar
   `req_dir`/`roadmap_dir`/`adr_dirs` para diretório existente e vazio zera a governança **mesmo em
   `strict`**. Medido, com uma REQ quebrada no disco: `✓ No violations found`, RC=0. É a mesma causa
   — o objeto verificado governa a própria verificação — e entra na **mesma REQ** pela Regra Dura.
   **Consequência para esta ADR: a decisão vale para o `escopo` da verificação, não só para a
   `severidade`.** É o pior dos três, porque é silencioso e sobrevive à correção dos outros dois.

3. **Prazo absurdo é o mesmo que prazo ausente.** `lenient_until: 9999-12-31` derrota a decisão 1 a
   custo zero. Recusar data ausente e aceitar data absurda seria incoerente — as duas escrevem
   "leniência para sempre". A decisão 1 passa a incluir um **teto de horizonte**.

**Residuais nomeados e não cobertos** (detalhe e motivo na REQ): `.trackfw-baseline.json` commitado à
força; manoplas comportamentais (`stale_wip_days`, `wip_limit`); `req_roadmap_sync` como braço de
falso-negativo, hoje com zero ocorrências. Ficam **nomeados** porque residual sem nome volta como
surpresa.

## Relacionadas

- **#376** — o gerador aplicava o template de consumidor ao produtor; os mesmos dois checks validavam
  o binário publicado. Consertou *qual binário* eles executam. Esta ADR trata de *poderem bloquear*.
- **#366** — gate escrevendo na árvore que audita.

Mesma **família** — conteúdo do repositório influenciando o que o CI verifica — e **causas
distintas**, por isso REQs distintas, conforme a Regra Dura de Causa Raiz.
