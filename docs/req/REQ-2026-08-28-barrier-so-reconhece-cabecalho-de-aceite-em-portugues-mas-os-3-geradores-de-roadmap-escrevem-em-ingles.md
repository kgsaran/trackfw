---
status: Open
date: 2026-08-28
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `barrier` só reconhece cabeçalho de aceite em português, mas os 3 geradores de roadmap escrevem em inglês

> Date: 2026-08-28 | Status: Open

## Motivation

**Todo roadmap que a própria ferramenta gera é reprovado pelo `trackfw barrier`**, com a mensagem
`ML-XX: no acceptance block` — num ML cujo bloco de aceite está preenchido e com todos os critérios
marcados.

Medido em 2026-08-28, durante a barreira da Wave 3 da
`REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar`:

| | escreve / procura | onde |
|---|---|---|
| `roadmap new` (Go) | escreve `**Acceptance criteria:**` | `internal/generators/roadmap.go:64,176,225` |
| `roadmap new` (Node) | escreve `**Acceptance criteria:**` | `npm/src/generators/roadmap.js:31,495,558` |
| `roadmap new` (Python) | escreve `**Acceptance criteria:**` | `pypi/trackfw/generators/roadmap.py:40` |
| `barrier` (Go) | procura `^\*\*Crit[eé]rios de aceite:\*\*` | `internal/commands/barrier.go:166` |
| `barrier` (Node) | procura `/^\*\*Crit[ée]rios de aceite:\*\*/` | `npm/src/commands/barrier.js:144` |
| `barrier` (Python) | procura `^\*\*Crit[ée]rios de aceite:\*\*` | `pypi/trackfw/commands/barrier.py:105` |

A **paridade entre os 3 CLIs está intacta** — os três erram exatamente igual. O que está quebrado é o
contrato entre o **gerador** e o **verificador**, nos três ao mesmo tempo. Nenhum gate cross-CLI pega
isso, porque todos os três concordam entre si.

**Classe do defeito:** é a mesma da release 7.3.0 inteira — um controle emitindo veredito
desconectado do que deveria medir. Só que na direção oposta à daquelas cinco: aqui ele **bloqueia
trabalho correto** em vez de liberar trabalho errado. Ainda é o pior tipo de sinal, porque treina o
usuário a desconfiar do gate — e `ADR-2026-08-17` já registrou o padrão *"guard que atrapalha é
guard que o usuário desliga"*.

**Impacto prático:** ninguém consegue passar a barreira sem editar à mão o cabeçalho que a ferramenta
acabou de escrever, e não há como adivinhar isso — a mensagem de erro diz que o bloco não existe.
Foi exatamente o que aconteceu aqui: converti os 6 cabeçalhos do roadmap para a forma portuguesa
como contorno.

## Acceptance Criteria

- [ ] **AC1** — `trackfw barrier` reconhece o bloco de aceite escrito pelo `roadmap new` **sem
      edição manual**, nos 3 CLIs. Verificável ponta a ponta: `roadmap new` → preencher os critérios
      → `barrier` → `acceptance_evidence: passed`.
- [ ] **AC2** — A forma portuguesa (`**Critérios de aceite:**`) **continua** sendo reconhecida.
      Existem roadmaps em `done/` e `wip/` com ela; quebrá-los seria trocar um bug por outro.
- [ ] **AC3** — O reconhecimento é idêntico nos 3 CLIs: mesmo conjunto de formas aceitas, mesma
      rejeição. Gate falsificável cobre — falha se um dos três aceitar uma forma que os outros não.
- [ ] **AC4** — Gate falsificável **nas duas direções**: (a) roadmap recém-gerado pelo `roadmap new`
      passa no `acceptance_evidence`; (b) roadmap sem bloco nenhum **continua** sendo reprovado —
      a correção não pode virar um "aceita qualquer coisa", que transformaria o check num no-op.
- [ ] **AC5** — Decidir e documentar qual é a forma **canônica** daqui pra frente. As duas serem
      aceitas é compatibilidade; uma delas tem que ser a que o gerador escreve, e o ADR precisa
      dizer qual e por quê (o projeto tem artefatos e mensagens em ambos os idiomas — a escolha não
      é óbvia e não deve ser tomada por acidente de implementação).
- [ ] **AC6** — `docs/cli-parity.md` documenta o contrato gerador↔`barrier` com anotação `gate=`.
- [ ] **AC7** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0.

## Negative Scope

- **Não** traduzir o resto do template de roadmap nem padronizar o idioma dos artefatos do projeto.
  O escopo é o **contrato entre o gerador e o `barrier`**, não uma política de i18n. (Política de
  i18n é `REQ-2026-08-16-conformidade-estrutural-e-comportamental-de-i18n-entre-os-tres-clis`.)
- **Não** mexer nos outros checks do `barrier` (`mls_complete`, `gates`, `validate`).
- **Não** migrar roadmaps existentes em `done/`, `wip/` ou `backlog/`.
- **Não** alterar o cabeçalho de status do ML (`**Status:**`) nem o parsing de wave.

## Observação de método

Este defeito foi encontrado **usando a ferramenta em trabalho real**, não por inspeção de código:
a barreira reprovou um ML que eu sabia estar completo. Vale registrar porque a suíte de 181 cenários
de falsificação não o pega — todos os três CLIs concordam entre si, e é exatamente isso que os
gates de paridade medem. Paridade entre implementações não é o mesmo que correção do contrato.

## Linked ADR
<!-- Um ADR é necessário para AC5: qual forma é canônica e por quê. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
