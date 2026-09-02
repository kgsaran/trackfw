---
status: Open
date: 2026-09-02
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: `init` e `discover` geram dois workflows que rodam a mesma validação, com instaladores diferentes

> Date: 2026-09-02 | Status: Open

## Motivation

Achado ao corrigir a colisão de job id (ML-1A da
`REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw`). Comparando o que cada
workflow gerado **executa**:

| workflow | gerado por | instala com | executa |
|---|---|---|---|
| `trackfw-gate.yml` | `init` / `update` | script de instalação | `trackfw validate` |
| `trackfw-validate.yml` | `discover` | `go install` | `trackfw validate` |

**São a mesma verificação.** A única diferença real é o método de instalação.

Um projeto que roda `init` **e** `discover` — o caminho normal de adoção — recebe **dois workflows
que validam a mesma coisa**. E o `trackfw-validate.yml` dispara em `push` **e** `pull_request`, então
um PR produz **três check-runs** para uma verificação.

## Por que isto importa mais do que "CI redundante"

**1. Custo em todo adotante.** Não é ineficiência deste repositório: é do que entregamos. Cada
projeto que adota o trackfw paga dois jobs por push.

**2. Foi o que escondeu a colisão de nome.** Os três check-runs homônimos que bloquearam o
`required_status_checks` existem **porque há dois workflows**. A colisão era o sintoma; a duplicação
é a causa.

**3. Ambiguidade de qual é o portão.** Com dois workflows equivalentes, qual entra no
`required_status_checks`? Exigir os dois dobra o custo sem dobrar a garantia; exigir um deixa o outro
como ruído verde que ninguém lê.

## A pergunta que a REQ precisa responder antes de corrigir

🔴 **Por que existem dois?** A resposta pode ser legítima e não é óbvia:

- O `discover` serve a repositório **que já existe** e talvez não tenha rodado `init`;
- Os instaladores diferentes podem atender públicos diferentes (`go install` exige Go; o script, não);
- Pode ser sedimentação histórica — dois caminhos que cresceram sem que ninguém comparasse.

**Descobrir isso é a Wave 0.** Unificar sem entender arriscaria remover um caminho que atende um caso
real de adoção.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
      de `discover` em repo sem `init`). **Se houver razão legítima, a REQ fecha documentando-a** —
      não force unificação.
- [ ] **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
- [ ] **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
      funcionando**. Remover CI de quem não rodou `init` seria trocar redundância por lacuna.
- [ ] **AC4** — Paridade nos 3 CLIs — a duplicação existe nos três.
- [ ] **AC5** — Migração para quem **já tem os dois** instalados: o `update` remove o obsoleto, ou o
      `doctor` acusa. **Deixar os dois em repositório existente e só corrigir o gerador resolveria
      apenas para projeto novo.**
- [ ] **AC6** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** reverter os job ids únicos do ML-1A. Eles corrigem a ambiguidade **independentemente** de
  haver um ou dois workflows.
- **Não** decidir quais checks entram no `required_status_checks` — é a Wave 2 da REQ irmã.

## Linked ADR

ADR: <!-- avaliar na Wave 0: se a conclusão for que os dois caminhos atendem públicos distintos, isso
é decisão de produto e merece registro. -->

## Linked Roadmap

Roadmap:
