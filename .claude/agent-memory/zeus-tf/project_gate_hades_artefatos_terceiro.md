---
name: gate-hades-artefatos-terceiro
description: Instalação de skill/agent/plugin de terceiro exige parecer prévio do hades-tf como gate de RUNTIME recorrente, não revisão única de desenho
metadata:
  type: project
---

Todo artefato de terceiro instalado no trackfw (**skill, agent ou plugin**) passa por um gate
de segurança do `hades-tf` **a cada instalação** — é gate de runtime recorrente, não uma
revisão única na fase de desenho. Vale para os dois caminhos de entrada: o comando explícito
do `trackfw` e o pedido em linguagem natural dentro da sessão ("instala essa skill pra mim").
Sequência inegociável: **baixar → quarentena → `hades-tf` analisa → só com parecer favorável
instala**. Nunca instalar e revisar depois.

**Why:** KG definiu isso explicitamente em 2026-08-15, ampliando a REQ original que só falava
de skills e tratava a revisão como evento único. O raciocínio é o mesmo que já motivou o
`credential-guard` e o `git-branch-guard` neste projeto: conteúdo de terceiro carregado por um
agente com `Bash`/`Edit`/`Write` vira instrução de sistema e pode tentar sequestrar a
autoridade do agente (agent kidnapping) — auto-conceder Git authority, desligar o gate de
governança, vazar segredos.

**How to apply:** ao planejar ou revisar qualquer feature de instalação de artefato externo,
o desenho tem de satisfazer a propriedade *"não existe caminho de código que instale artefato
de terceiro sem parecer prévio"*. Como um comando de CLI não invoca subagente por si, isso
força um **handshake de duas fases**: a fase 1 baixa para quarentena e emite artefato de
revisão legível por máquina; a fase 2 só consuma mediante referência ao parecer, **vinculado
pelo checksum do conteúdo** (senão aprova-se um conteúdo e instala-se outro — TOCTOU).
Atenção: `trackfw plugins install` hoje baixa binário de terceiro e faz `chmod 0755` **sem
gate nenhum** — se for trazido para esse regime, vai em REQ separada, porque gate de binário
é superfície de ameaça distinta de gate de composição de markdown.

Relacionado: [[doutrina-deteccao-ancorada-no-git]] — "só o orquestrador invoca" é guardrail,
não controle, e a resposta real é detecção.
