---
status: Open
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-09-01-publicar-a-exigencia-de-governanca-para-prs-no-contributing.md"
---

# REQ: O projeto não publica a exigência de governança para PRs, e não tem `CONTRIBUTING.md`

> Date: 2026-09-01 | Status: Open

## Motivation

**O trackfw é um framework de governança e rastreabilidade cujo próprio repositório não publica a
regra que ele existe para vender.** Medido:

| arquivo | existe? |
|---|---|
| `CONTRIBUTING.md` | **não** |
| `.github/CONTRIBUTING.md` | **não** |
| `.github/PULL_REQUEST_TEMPLATE.md` | **não** |

O `README.md:3` descreve `ADR → REQ → ROADMAP` como **o que o produto faz**. O `CLAUDE.md:167` tem a
sequência de comandos, mas é o **harness dos agentes**, não o guia de quem abre um PR. **Em lugar
nenhum está escrito que um PR para o trackfw precisa de REQ e roadmap.**

### O padrão já se repetiu três vezes, e esta é a quarta

É a mesma forma que registramos no `docs/cli-parity.md` sobre os gates de terceiro:

> *"Em lugar nenhum estava escrito que um gate, para contar como gate, precisa estar ligado e
> precisa reprovar quando não mede nada. Quem contribui de fora não adivinha um contrato que não
> existe."*

Dois gates chegaram inertes e custaram **um microlote corretivo cada**. O terceiro **nasceu correto**,
porque a regra passou a ser comunicada. A comunicação foi a correção — não a vigilância.

Agora repetimos o padrão num nível acima: pedimos governança a um contribuidor externo (PRs #238 e
#240) **por uma regra que nunca publicamos**. O pedido foi feito reconhecendo isso explicitamente,
mas a lacuna é nossa e precisa fechar.

### Por que isto não é burocracia

Um framework de rastreabilidade cujos mantenedores aceitam mudanças fora da cadeia é uma
**demonstração contra o produto**. A cadeia não é cerimônia: nesta sessão ela pegou dois critérios
meus que eram vácuos, um contrato que seria falso, um `skipif` que anularia a verificação inteira, e
uma afirmação factualmente errada em dois documentos — **nenhum deles apareceria no CI verde**.

## Acceptance Criteria

- [ ] **AC1** — `CONTRIBUTING.md` na raiz, publicando: a cadeia `ADR → REQ → ROADMAP`, os comandos
      exatos (`trackfw req new` / `roadmap new` / `roadmap move <nome> wip`), e **quando** cada um é
      exigido.
- [ ] **AC2** — 🔴 **A exceção de trivialidade é publicada junto.** Sem ela, a regra vira absurda —
      um typo não precisa de REQ. A lista fechada já existe internamente (`~/.claude/CLAUDE.md` §7);
      publicá-la é o que torna a exigência **defensável em vez de arbitrária**.
- [ ] **AC3** — O **contrato de gates** entra no `CONTRIBUTING.md`: um gate precisa estar **ligado**
      ao `Makefile` e **reprovar quando não mede nada**. Hoje está só no `docs/cli-parity.md`, que é
      documento interno de paridade — não é onde um contribuidor olha.
- [ ] **AC4** — `.github/PULL_REQUEST_TEMPLATE.md` com os campos mínimos: REQ ligada, roadmap ligado,
      e a **falsificação nas duas direções** — incluindo o controle. É o item que mais falta nas
      contribuições, e o que mais barato é pedir no template.
- [ ] **AC5** — 🔴 **Falsificação da própria regra:** um PR **sem** REQ ligada é detectável. Se a
      exigência não for verificável, ela é decorativa — e estaríamos publicando um contrato sem gate,
      exatamente o que acabamos de criticar. Avaliar `trackfw validate` ou o job `governance`.
- [ ] **AC6** — O `README.md` aponta para o `CONTRIBUTING.md` de forma visível.
- [ ] **AC7** — 🔴 **Coerência declarada:** o `CONTRIBUTING.md` diz que a regra **vale para os
      mantenedores também**, e cita que este próprio documento foi escrito sob a cadeia. Um guia de
      governança criado fora da governança se refutaria sozinho.

## Negative Scope

- **Não** endurecer o `validate` para bloquear PR sem REQ nesta REQ — a AC5 **avalia** a
  detectabilidade; implementar bloqueio é decisão que merece REQ própria.
- **Não** reescrever o `docs/cli-parity.md` — ele continua sendo o documento de contrato interno.
- **Não** aplicar retroativamente aos PRs já mergeados.

## Linked ADR

ADR: <!-- avaliar na análise: se a decisão for que a cadeia é obrigatória para PR externo com efeito
em runtime, isso é postura de projeto com consequência de comunidade, e merece ADR. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-09-01-publicar-a-exigencia-de-governanca-para-prs-no-contributing.md`

---

## 🔴 Recuperada em 2026-09-12 — este artefato foi apagado por engano

A branch `docs/publicar-a-exigencia-de-governanca-para-prs-no-contributing` (tip `507c54a2`,
2026-09-01) foi **apagada por engano** em 2026-09-12, junto com outras 35, por uma classificação
automática defeituosa. Ela era a **única das 36 sem PR** — logo, a única com trabalho não integrado.

**A causa foi um defeito de shell no teste prescrito pelo `CLAUDE.md`**, Passo 3-bis:

```bash
touched=$(git diff --name-only "$mb" <branch>)
diverg=$(git diff --name-only origin/main <branch> -- $touched)
```

🔴 **O `zsh` não faz word-splitting de variável não citada** — o `bash` faz. Em zsh, `-- $touched`
vira **um único pathspec** com quebras de linha dentro, que não casa arquivo nenhum. O resultado é
sempre vazio, e vazio é lido como *"integrada"*. **Toda** branch é classificada como integrada.

Medido no mesmo dia:

```
$touched sem aspas   →  0 arquivos diferem  →  "INTEGRADA"
array citado          → 17 arquivos diferem  →  ABERTA
```

Recuperada do object store. Roadmap volta em `backlog/`, não em `wip/`, porque não há trabalho em
curso — o estado `wip` do original refletia setembro.
