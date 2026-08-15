---
status: Open
date: 2026-08-15
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md"
---

# REQ: instalacao de skills de terceiro via URL para agentes especialistas

> Date: 2026-08-15 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Ideia do usuário, complementar à REQ irmã
`REQ-2026-08-15-agentes-especialistas-aceitam-contexto-...`: em vez de só declarar
convenções como texto no `trackfw.yaml`, permitir que o usuário aponte uma URL de
"skill" de terceiro (conhecimento de linguagem, stack, design pattern, padrão
arquitetural) para o `trackfw` baixar e decidir onde ela deve residir (qual agente/
escopo recebe esse conhecimento).

**Risco de segurança, levantado nesta análise, não trivial e não deve ser subestimado:**
uma skill baixada de URL se torna, na prática, **instrução de sistema** carregada por um
agente com `Bash`/`Edit`/`Write` — diferente de instalar uma dependência de código (que
roda em sandbox de execução, com superfície limitada), aqui o conteúdo baixado pode
tentar **reescrever o comportamento do próprio agente**: instruir a ignorar a fronteira
de Git authority (`trackfw_architect` como único que commita/dá push), pular a checagem
de governança (`wip` roadmap), ou vazar segredos. É a mesma classe de risco de
prompt-injection/supply-chain que motivou o `credential-guard`/`git-branch-guard` deste
mesmo projeto — não é hipotético, é o padrão de ataque que o projeto já trata como sério
o suficiente para justificar hooks técnicos dedicados.

Esta REQ propõe o mecanismo, mas com o desenho de segurança como parte central do
escopo, não um adendo — **`hades-tf` (Security) deve revisar o design antes de qualquer
ML de implementação começar**, dado o histórico deste projeto de tratar esse tipo de
vetor como P0.

**Restrição adicional, definida explicitamente pelo usuário (2026-08-15):** o comando de
instalação de skill **não é um comando CLI de uso geral** — ele só pode ser executado
dentro do contexto de uma sessão com LLM (agente), e **exclusivamente pelo orquestrador/
arquiteto** (`trackfw_architect`/Zeus), nunca diretamente pelo usuário via shell nem por
um agente especialista. O fluxo obrigatório é: usuário aponta a URL → Zeus invoca
`hades-tf` para analisar a skill quanto a segurança (prompt injection, "agent
kidnapping" — tentativa da skill de sequestrar o comportamento/autoridade de um agente,
categoria mais ampla que só "redefinir Git authority/mode lock" citada acima) → só após
parecer favorável do `hades-tf` o Zeus prossegue com a instalação. Isso implica: o
comando técnico (`trackfw skill add <url>` ou equivalente) deve, por natureza, recusar
execução fora de um contexto de sessão de agente (ex.: exigir uma env var/flag que só o
harness do orquestrador injeta, nunca disponível em invocação humana direta de terminal)
— decisão de design exata de como impor essa restrição tecnicamente cabe ao ML, mas o
requisito em si (só o arquiteto invoca, só após revisão do `hades-tf`, nunca uso
direto/manual) é inegociável, no mesmo espírito do resto desta REQ.

**Emenda de escopo do usuário (2026-08-15, posterior à redação acima):** a revisão do
`hades-tf` **não é um evento único de desenho** — é um **gate de runtime, recorrente,
disparado a cada instalação de artefato de terceiro**. Dois esclarecimentos que ampliam o
escopo original:

1. **Vale para skill E para agent/plugin de terceiro**, não só skill. Qualquer artefato de
   terceiro que vire instrução carregada por um agente (`skill`, `agent`, `plugin`) entra
   no mesmo fluxo. A REQ nasceu com título só de "skills" — leia-se "artefato de terceiro"
   em todos os critérios abaixo.
2. **Os dois caminhos de entrada disparam o gate:**
   - usuário executa o comando do `trackfw` (`skill add` / `agent add` / equivalente para
     third-party);
   - usuário pede em linguagem natural, dentro da sessão, "instala essa skill/agent pra mim".

   Em ambos, a sequência é a mesma e é inegociável: **baixar → quarentena → `hades-tf`
   analisa → só com parecer favorável instala**. Nunca instalar e revisar depois.

**Consequência de design que decorre desta emenda:** um comando de CLI não consegue, por si,
invocar um subagente. Logo o comando precisa ser desenhado para **parar** no meio do fluxo —
baixar em quarentena, emitir um artefato de revisão legível por máquina, e exigir referência
ao parecer do `hades-tf` para consumar a instalação. O desenho exato desse handshake em duas
fases é decisão da Wave 0 do roadmap (Q8), mas a propriedade "não existe caminho de código que
instale artefato de terceiro sem parecer prévio" é o requisito, não uma sugestão.

## Acceptance Criteria
- [ ] Novo comando (ex.: `trackfw skill add <url>`) baixa o conteúdo, mas **nunca o
      carrega automaticamente** sem confirmação — mostra o conteúdo completo ao usuário
      (ou um resumo com diff do que seria adicionado) antes de instalar, mesmo em modo
      não-interativo/CI (onde deve recusar por padrão, exigindo uma flag explícita tipo
      `--yes-i-trust-this-source`).
- [ ] Validação de que o conteúdo baixado NÃO contém instruções que tentam sobrescrever
      as fronteiras já estabelecidas nos agentes do catálogo (Git authority, governance
      prerequisite, mode lock) — critério objetivo mínimo: recusar instalação se o
      conteúdo contiver os marcadores literais dessas seções tentando redefini-las (ex.:
      texto que se apresenta como "## Git authority" ou "## Mode lock" dentro da skill
      baixada) — decisão de design exata do critério de detecção cabe ao ML, mas o
      objetivo (skill não pode se auto-conceder poderes de commit/push nem desligar o
      gate de governança) é inegociável.
- [ ] "Decidir onde a skill deve residir": a skill nunca SUBSTITUI o arquivo de um
      agente do catálogo — é sempre uma seção suplementar, apensada/referenciada (mesmo
      padrão de composição da REQ irmã de convenções de projeto), nunca uma
      sobrescrita. O usuário confirma explicitamente em qual(is) agente(s) a skill se
      aplica antes de instalar — o `trackfw` pode sugerir (por palavra-chave/nome da
      skill) mas não decide sozinho e silenciosamente.
- [ ] Escopo de instalação: local ao projeto por padrão (não escopo global —
      `~/.trackfw/...` — sem confirmação extra, dado que afeta todos os projetos do
      usuário).
- [ ] Registrar a proveniência (URL, hash/checksum do conteúdo baixado, data) em algum
      lugar auditável do projeto — para que `trackfw validate`/auditoria futura consiga
      responder "de onde veio isso" sem depender de memória do usuário.
- [ ] Comportamento idêntico nos 3 CLIs.
- [ ] `make quality` passa sem novas divergências de paridade.
- [ ] Revisão de `hades-tf` documentada (ADR ou seção de segurança dedicada no roadmap)
      antes do primeiro ML de implementação — não é opcional, é pré-requisito de
      sequenciamento deste roadmap.
- [ ] Comando de instalação só executa dentro de contexto de sessão de agente e apenas
      quando invocado pelo orquestrador/arquiteto (`trackfw_architect`) — recusa
      execução em invocação humana direta de terminal. Fluxo obrigatório: usuário aponta
      URL → Zeus invoca `hades-tf` para análise de segurança (prompt injection, agent
      kidnapping) → só com parecer favorável do `hades-tf` a instalação prossegue.
- [ ] **(Emenda 2026-08-15)** O gate do `hades-tf` é **de runtime e recorrente**, disparado a
      cada instalação de artefato de terceiro — **skill, agent ou plugin** — e pelos dois
      caminhos de entrada (comando explícito do `trackfw` e pedido em linguagem natural na
      sessão). Não existe caminho de código que instale artefato de terceiro sem parecer
      prévio: o comando baixa para quarentena, para, e só consuma a instalação mediante
      referência ao parecer favorável.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas.md
