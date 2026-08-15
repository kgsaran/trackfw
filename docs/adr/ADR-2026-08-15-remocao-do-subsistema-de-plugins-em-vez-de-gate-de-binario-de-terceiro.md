---
status: Accepted
date: 2026-08-15
author: "Zeus (Arquiteto)"
---

# ADR: Remoção do subsistema de plugins, em vez de gate de binário de terceiro

> Date: 2026-08-15 | Status: Accepted
> **Supersede:** `docs/adr/ADR-2026-08-15-gate-de-plugins-binario-deteccao-de-adulteracao-sem-deteccao-de-instalacao-e-chmod-apos-aprovacao.md`

REQ: `docs/req/REQ-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`
Parecer que sustenta a decisão: `docs/seguranca/2026-08-15-gate-de-plugins-binario.md`

## Context

O ADR anterior decidiu **cercar** o download de binário de terceiro com um gate de duas fases
(quarentena → parecer → `chmod` após aprovação), reusando o padrão do gate de artefato markdown.

Ao escrevê-lo, o parecer do `hades-tf` deixou explícito o quanto esse gate **não** entregaria:

| Propriedade | Gate markdown | Gate de plugins (projetado) |
|---|---|---|
| Barreira no momento da instalação | ✅ | ✅ |
| Detecção de adulteração pós-aprovação | ✅ | ✅ |
| **Detecção de instalação clandestina** | ✅ | ❌ **estruturalmente impossível** |
| Revisor consegue ler o conteúdo | ✅ | ❌ é binário |
| Verificação de origem | fora de escopo, declarado | fora de escopo, declarado |

O ramo (i) da detecção é impossível porque `~/.trackfw/plugins` é **por-máquina e compartilhado
entre todos os projetos**: um plugin ali pode ter sido aprovado em outro projeto, e uma regra que
tentasse detectar dispararia falso-positivo entre projetos não relacionados.

Somando: o gate exigiria uma superfície nova inteira — quarentena de binário, artefato de revisão
com sidecar, índice de checksums versionado, script de paridade dedicado, testes nos 3 stacks — para
entregar uma garantia **parcial** sobre um vetor que o próprio parecer classificou como de
severidade **maior** que o de markdown.

**KG propôs a alternativa que não estava na mesa: remover.**

## Decision

**O subsistema de plugins é removido do trackfw, por completo. O trackfw deixa de baixar, instalar,
gerenciar e executar código de terceiro.**

### D1 — Remoção total, não parcial

Saem: `plugins add`, `plugins search`, `plugins list`, `plugins remove`, o pacote
`internal/plugins/`, os equivalentes em `npm/src/commands/plugins.js` e
`pypi/trackfw/commands/plugins.py`, e a execução — `RunPlugin` mais o fallback de
`internal/commands/root.go:71-74`.

Remoção parcial foi **rejeitada**: manter `list`/`remove` sem `add` deixa gestão sem objeto, e
manter execução sem instalação preserva justamente a parte que **não** dá para controlar. Tirar o
que era controlável e conservar o incontrolável é o pior dos arranjos.

### D2 — A responsabilidade da instalação transfere; a da execução, **não** — por isso ela também sai

"Se o usuário instalar por fora, não é responsabilidade do trackfw" é verdadeiro para a
**instalação**. Não é verdadeiro para a **execução**: quem decide executar qualquer `trackfw-*`
encontrado é o trackfw, e essa é uma decisão de design dele, não do usuário.

Por isso a remoção inclui a execução. Depois dela, rodar uma ferramenta `trackfw-*` é invocar o
binário direto no shell — ato inteiramente do usuário, sem intermediação do trackfw.

### D3 — Fecha o débito D9 do ADR superseded

`root.go:71-74` fazia **qualquer argumento desconhecido** virar execução de plugin: `trackfw
vaildate` executava `trackfw-vaildate` se existisse. Era débito datado para REQ futura; a remoção
o elimina de graça. Argumento desconhecido passa a ser **erro de comando desconhecido**, com
mensagem idêntica nos 3 CLIs.

### D4 — Paridade obtida por remoção, não por adição

O Python nunca teve instalação de plugin — o ADR superseded ia registrar isso como **exceção
intencional documentada** em `docs/cli-parity.md`. Com a remoção, os três CLIs convergem para o
mesmo comportamento e a exceção deixa de ser necessária.

### D5 — Breaking change: `7.0.0`, em PR próprio

`plugins add` está documentado no `README.md`. A remoção é breaking. A versão vai de `6.10.0` para
**`7.0.0`**, com `CHANGELOG.md` declarando a Breaking Change — em **PR separado** do PR de remoção,
conforme o protocolo de release do projeto (bump atômico nos 4 arquivos de versão; tag após o merge).

### D6 — Nenhum substituto agora

Não nasce sistema de extensão novo junto. Se um dia fizer sentido, virá de REQ própria, com o gate
desenhado **desde o início** em vez de acrescentado depois — que é exatamente a lição desta.

## Consequences

**Positivas**
- O vetor de maior severidade do projeto deixa de existir, em vez de ser mitigado.
- Some superfície: um pacote inteiro, um registry externo não-pinado, um parser YAML caseiro, três
  caminhos de download.
- O débito D9 morre junto.
- Paridade dos 3 CLIs por convergência natural.
- O trabalho de gate que seria necessário (quarentena de binário, sidecar, índice, paridade) **não
  precisa existir**.

**Negativas / riscos aceitos**
- **Breaking change** para quem usa `trackfw plugins add` hoje. Mitigado por: projeto sem usuários
  downstream conhecidos, e `CHANGELOG` explícito.
- O trackfw perde extensibilidade. É deliberado — extensibilidade por execução de binário arbitrário
  é a própria fonte do risco, e a alternativa (gate) foi avaliada e considerada cara demais para o
  que entrega.
- Quem quiser as ferramentas `trackfw-*` passa a instalá-las e invocá-las por conta própria.
