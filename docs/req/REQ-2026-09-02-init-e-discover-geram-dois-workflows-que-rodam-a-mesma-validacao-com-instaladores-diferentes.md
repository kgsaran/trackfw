---
status: Open
date: 2026-09-02
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md"
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
- [ ] ~~**AC4** — Paridade nos 3 CLIs~~ → 🔴 **OBSOLETO pela v8.0.0**: existe **uma** implementação
      em Go, entregue por três canais. Não há paridade a manter. Substituído pelo **AC4-bis**.
- [ ] **AC4-bis** — 🔴 **O comentário do código que afirma decisão inexistente é corrigido.**
      `scaffold_doctor.go:333` afirma *"both can coexist in the same project (ADR-2026-08-28)"*;
      a `ADR-2026-08-28` tem **zero** ocorrências de `trackfw-validate.yml` — ela decide pino de
      versão e `TRACKFW_VERSION`. Medido: o único ADR que cita o arquivo é a `ADR-2026-09-18`, e lá
      como caminho de exemplo. **A coexistência nunca foi decidida**; o comentário fabrica a decisão
      e vinha travando a correção por engano.
- [ ] **AC5** — Migração para quem **já tem os dois** instalados: o `update` remove o obsoleto, ou o
      `doctor` acusa. **Deixar os dois em repositório existente e só corrigir o gerador resolveria
      apenas para projeto novo.**
- [ ] **AC6** — `make quality` e **CI** verdes.

## 🔴 AMPLIADA em 2026-09-29 — o #451 é esta causa, relatado de fora

Origem: **#451** (consumidor externo) e o comentário de `lourivalgarciajunior` na mesma issue.
Absorvido aqui por **mesma causa, mesmo mecanismo** — não vira REQ nova.

### O que o relato de fora acrescenta à medição

**1. A contagem caiu de 3 para 2, e a causa é nossa.** O PR **#456** trocou o gatilho de lista por
`push: branches:[main]` + `pull_request:` nas duas variantes do template. Medido pelo consumidor, no
**mesmo SHA**:

```
SHA 47d4a6b6 (ponta de branch com PR aberto)
  trackfw-gate.yml       pull_request   1
  trackfw-validate.yml   pull_request   1     → 2 execuções de `trackfw validate`
SHA 08b514d9 (main, pós-merge)
  trackfw-validate.yml   push           1
  trackfw-gate.yml       —              não dispara em push
```

Metade do defeito do #451 fechou sozinha. **O que resta é o núcleo desta REQ:** dois arquivos, mesmo
comando, mesmo veredito, no mesmo evento.

**2. O custo tem nome, e é restrição declarada.** O consumidor é repositório **privado em plano
free**: 2.000 min/mês. Este projeto já descartou job-splitting por medição, com o motivo escrito no
topo do `quality-gates.yml`: *"Cota é a causa-raiz desta família de defeitos."* Entregar duas
execuções do mesmo veredito vai contra a nossa própria decisão — e o custo não é o `validate`, é o
`checkout` + instalação do binário, duas vezes.

**3. A assimetria no gerador, verificada por mim em 2026-09-29:**

| sítio | comportamento |
|---|---|
| `scaffold.go:2553` `generateGitHubActionsWorkflow` | escreve `trackfw-gate.yml` **incondicionalmente** |
| `update.go:2189` `refreshDiscoverGitHubActionsWorkflowIfPresent` | **só atualiza** `trackfw-validate.yml` se já existir; nunca cria |

O caminho cuidadoso já existe no código. 🔴 **A assimetria é o defeito** — um lado pergunta, o outro não.

⚠️ E ela puxa o `doctor`: hoje ele cobra `trackfw-gate.yml` sempre que `ci: github-actions`. Pular a
escrita **sem** mexer no `doctor` troca uma duplicação por um achado falso. É uma dependência dura
entre MLs, não um detalhe.

### AC novo

- [ ] **AC7** — 🔴 **Um único `trackfw validate` por evento no repositório do consumidor**, medido
      por contagem de check-runs no mesmo SHA — não por leitura do template
- [ ] **AC8** — 🔴 **O `doctor` acompanha a decisão na mesma entrega.** Nenhum achado falso novo, e
      o contra-braço: o achado **verdadeiro** que ele já dá continua saindo

## Negative Scope

- **Não** reverter os job ids únicos do ML-1A. Eles corrigem a ambiguidade **independentemente** de
  haver um ou dois workflows.
- **Não** decidir quais checks entram no `required_status_checks` — é a Wave 2 da REQ irmã.

## Linked ADR

ADR: <!-- avaliar na Wave 0: se a conclusão for que os dois caminhos atendem públicos distintos, isso
é decisão de produto e merece registro. -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md`
