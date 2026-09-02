---
status: Accepted
date: 2026-09-01
author: "zeus-tf"
---

# ADR: O repositório do trackfw é governado pelo trackfw, com o mesmo rigor que o produto vende

> Date: 2026-09-01 | Status: Accepted

## Contexto

Ao pedir governança a um contribuidor externo (PRs #238 e #240) e ao investigar a falsificabilidade
dessa exigência, três medições mudaram a leitura do repositório:

### 1. A proteção da `main` não exige nada

```
required_status_checks           AUSENTE   → o CI é conselho, não portão
required_approving_review_count  0         → nenhuma revisão exigida
enforce_admins                   false     → admin ignora as regras que existem
```

**Qualquer PR pode ser mergeado com todo o CI vermelho e zero revisão.** Os jobs `governance` e
`trackfw-gate`, que rodam `trackfw validate`, são **informativos**. Lemos verde como portão; é
semáforo.

### 2. Os guards protegem agentes, não pessoas

```
.claude/settings.json    ← credential-guard e git-branch-guard vivem SÓ aqui
.git/hooks/              ← nenhum hook ativo
core.hooksPath           = /dev/null
```

Nesta sessão os guards me bloquearam **seis vezes** — `git stash` em worktree compartilhado,
`checkout --` destrutivo, `push` bruto. **Um humano com git normal não tem nenhuma dessas
proteções**, e o `trackfw` as instala em outros projetos.

### 3. A cadeia não é exigida de quem contribui

Não há `CONTRIBUTING.md` nem template de PR. A regra `ADR → REQ → ROADMAP` é descrita no `README`
como *o que o produto faz*, nunca como *o que este repositório exige*.

## A incoerência

**O trackfw vende rastreabilidade aplicada e não a aplica a si mesmo.** Cada uma das três lacunas é
exatamente o defeito que o produto existe para corrigir. Um framework de governança cujo próprio
repositório opera por convenção verbal é a demonstração mais forte **contra** o produto que se pode
construir.

E o padrão já apareceu quatro vezes nesta sessão, sempre com o mesmo formato: **o mecanismo dá sinal
verde enquanto o controle está inerte.** Gates que nunca rodavam, `VERDICT=ABSENT` por vacuidade,
`success()` implícito, contrato afirmando garantia que o gate não entrega. Aqui é o mesmo, no nível
do repositório.

## Decisão

**O repositório do trackfw passa a ser governado pelo trackfw, com o mesmo rigor exigido de qualquer
projeto que o adote.** O que o produto instala em projeto de terceiro, este repositório usa.

Consequências assumidas, cada uma com custo real:

1. **`required_status_checks` configurado.** Merge com CI vermelho deixa de ser possível.
2. **Guards ativos para humanos**, não só para agentes — `core.hooksPath` deixa de ser `/dev/null`.
3. **A cadeia publicada e exigida**, com a exceção de trivialidade também publicada.

## O que esta decisão NÃO significa

**Não** significa exigir REQ para typo — a exceção de trivialidade é parte da regra, não concessão.

**Não** significa tornar obrigatórios os jobs de Windows, que **nascem vermelhos por projeto** e são
`continue-on-error`. Torná-los exigidos travaria todo merge até os onze defeitos fecharem — o oposto
do que o instrumento existe para permitir. **A escolha de quais checks são exigidos é decisão de
desenho, não consequência automática**, e a REQ ligada a trata.

**Não** significa aplicar retroativamente ao que já foi mergeado.

## Alternativas descartadas

**Manter como está e confiar na disciplina.** É o estado atual, e ele falhou de forma mensurável:
nesta sessão eu mesmo mergeei com roadmap em `wip` cinco vezes, criei branch em paralelo violando
regra própria, e commitei resíduo de PoC duas vezes. **Disciplina não é controle** — e o produto
existe precisamente porque disciplina não escala.

**Aplicar só o `required_status_checks` e parar.** Fecharia o portão mais visível e deixaria as duas
outras lacunas, que são da mesma classe. Trataria o sintoma mais barulhento.

## Rastreamento

REQ: `REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md`
