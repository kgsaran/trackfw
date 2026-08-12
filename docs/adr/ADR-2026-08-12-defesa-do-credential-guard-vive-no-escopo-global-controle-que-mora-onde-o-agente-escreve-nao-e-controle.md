---
status: Accepted
date: 2026-08-12
author: "Zeus (Arquiteto), decisão de KG"
---

# ADR: Defesa do credential-guard vive no escopo global — controle que mora onde o agente escreve nao e controle

> Date: 2026-08-12 | Status: Accepted

REQ: `docs/req/REQ-2026-08-12-mitigacao-do-fail-open-do-credential-guard-integridade-do-script-e-da-config-controle-positivo-e-fail-closed-nativo.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard-wave-1-controle-positivo-e-failclosed.md`

## Context

O `scripts/trackfw-credential-guard.sh` é um **controle de negação**: bloqueia a materialização de
credenciais reais por subagentes. Foi **medido** (`ROADMAP-2026-08-12-semantica-de-falha-de-hook`)
que, quando o `command` de um hook não resolve, **a ferramenta prossegue** — fail-open em 4 dos 6
CLIs. Se o guard não roda, nada bloqueia, e o agente não trata isso como erro fatal.

O parecer de segurança (`docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`, Revisão ML-2B)
listou as vias pelas quais um agente induzido, **sem privilégio nenhum**, alcança esse estado:

- **apagar** `scripts/trackfw-credential-guard.sh` — o guard bloqueia materialização de credenciais,
  não `rm`;
- **sobrescrever** o script com `exit 0` — pior que apagar: produz `hook: PreToolUse Completed`, o
  **mesmo rótulo de sucesso normal**, sem sinal observável;
- **rebaixar `credential_guard.mode`** no `trackfw.yaml`, lido em **runtime**
  (`internal/generators/scaffold.go:1005`) — uma linha de YAML, sem tocar no script.

Três mitigações foram consideradas para o **escopo de projeto**:

1. **`failClosed: true`** (opt-in nativo do Cursor) — implementado nos ML-3A/3B desta branch;
2. **wrapper** `sh -c 'test -x <script> && exec <script> || exit 2'`, convertendo "não consegui
   rodar" em bloqueio;
3. **verificação de integridade** do conteúdo do script e da config.

E uma quarta, de natureza diferente:

4. **preferir o escopo global** — o guard instalado por `trackfw update harness` vive em
   `~/.trackfw/`, **fora do repositório** em que o agente trabalha.

## Decision

**A defesa real do credential-guard é o escopo global. O escopo de projeto passa a ser tratado como
conveniência de configuração, não como controle de segurança.**

Consequências diretas, decididas por KG em 2026-08-12:

- **Não enviar `failClosed`** (ML-3A/3B) — o código é **revertido** nesta branch.
- **Não implementar o wrapper** (item 3 da REQ).
- **Não construir verificação de integridade** de script/config (item 4 da REQ).
- **O trackfw passa a recomendar/instalar o guard global por padrão** — trabalho de REQ própria.
- **Mantido** o que já foi entregue e é útil independentemente: a regra
  `credential_guard_hook_resolvable` (ML-1A) e o Cenário 47 que prova que ela não é vácua (ML-2A).

### O argumento que decide

**Um controle que mora no mesmo diretório em que o agente escreve não é um controle.** As três
mitigações de escopo de projeto tentam proteger um arquivo que o próprio adversário pode apagar,
sobrescrever ou desligar por config. Elas elevam o custo do ataque; não o impedem. O guard global
está **fora do alcance** de um agente restrito ao workspace — o que ataca a causa em vez do sintoma.

### Por que as três alternativas foram rejeitadas, especificamente

**`failClosed` (item 1).** Cobre apenas o **Cursor** — é o único dos 6 CLIs com opt-in nativo. E
**brica clone fresco**: um repositório com `.cursor/hooks.json` já commitado, **antes** do `init`
gerar o script, teria toda chamada de shell bloqueada dentro do Cursor. Aceitar esse custo para
proteger 1 de 6 CLIs, contra um adversário que ainda pode apagar o script, não fecha.

**Wrapper (item 2).** Mesma classe de *bricking*, confirmada no código: o script é **gerado** por
`trackfw init`/`update harness` (`internal/generators/scaffold.go:779-837`), **não faz parte do
binário**. Cobre a via de **deleção**, mas **não** a de **sobrescrita** — um script substituído por
`exit 0` passa no `test -x` e sai `0`.

**Integridade de conteúdo (item 3).** É a única que cobriria sobrescrita **e** downgrade de config,
mas exige um valor de referência guardado fora do arquivo gerado — ou seja, **exige exatamente o
escopo global** para ter onde guardar a referência de forma confiável. Se o escopo global é
pré-requisito da solução, é mais simples e mais forte usá-lo como a solução.

### Incoerência corrigida por esta decisão

O roadmap adiava o wrapper **por risco de bricking** e, ao mesmo tempo, enviava o `failClosed`, que
**brica pelo mesmo mecanismo**. A incoerência foi detectada durante o ML-3A e é resolvida aqui: ambos
ficam de fora, pelo mesmo motivo.

## Consequences

**Positivas**
- Ataca a causa: o controle sai de onde o adversário escreve.
- Evita duas classes de *bricking* que travariam o trabalho de usuários legítimos.
- Evita construir integridade de conteúdo cujo pré-requisito seria o próprio escopo global.
- O que foi entregue (regra de validação + prova negativa) permanece e continua útil: detecta o
  **incidente real já observado** — hook apontando para script que não resolve.

**Negativas / riscos aceitos**
- **Usuário que não instala o guard global fica sem defesa** contra as vias de deleção e sobrescrita.
  Aceito, e **precisa estar documentado** — a regra do ML-1A **não** detecta sobrescrita, só ausência.
- O guard global tem **pré-condição de instalação** e não é o default hoje; até que passe a ser, a
  decisão é uma intenção, não uma proteção. **Esta é a lacuna mais importante deste ADR** e o motivo
  de a REQ de "global por padrão" ter prioridade.
- **Não foi medido** se o agente alcança `~/.trackfw/` nos ambientes reais em que o trackfw roda —
  a premissa "agente restrito ao workspace" vale para sandboxes que restringem escrita fora do
  projeto, **não universalmente**. Um agente sem sandbox alcança `$HOME`. Registrar como hipótese a
  verificar antes de tratar o escopo global como defesa forte.
- `credential_guard.mode` no `trackfw.yaml` **continua** rebaixável por uma linha de YAML, no
  repositório, independentemente do escopo do script. **Esta via não é fechada por esta decisão** e
  deve entrar na REQ de "global por padrão".

### Emenda 1 (2026-08-12, ML-4A) — a premissa central é provavelmente FALSA no ambiente padrão

Este ADR decidiu com base em: *"o guard global está **fora do alcance** de um agente restrito ao
workspace"*, e registrou como risco aceito que isso **não foi medido**.

O parecer do ML-4A (`docs/seguranca/2026-08-12-pos-barreira-estado-do-credential-guard.md`) avalia
essa probabilidade e conclui: **a premissa é provavelmente falsa no ambiente padrão — nenhum dos 6
CLIs roda sandboxed por default.** Um agente sem sandbox alcança `$HOME`, e portanto alcança
`~/.trackfw/` e os arquivos de settings globais.

**O que isso muda:**

- O escopo global deixa de ser "fora do alcance" e passa a ser, no melhor caso, **mais caro de
  alcançar** — o agente precisaria sair do diretório do projeto, o que é um sinal mais visível.
  Continua sendo melhor que o escopo de projeto, mas **por diferença de grau, não de natureza**.
- A rejeição das três mitigações de escopo de projeto continua válida pelos **motivos próprios de
  cada uma** (cobertura de 1 de 6 CLIs e *bricking* no `failClosed`; *bricking* e não cobrir
  sobrescrita no wrapper; dependência do próprio escopo global na integridade). **Nenhuma delas
  dependia exclusivamente da premissa agora abalada.** A decisão se sustenta; a **justificativa
  precisa ser lida com esta emenda junto.**
- **A medição vira a primeira coisa a fazer.** O eixo 2 da
  `REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-...` deixa de ser um
  pré-requisito entre outros e passa a ser o **primeiro trabalho, sem paralelizar** com consentimento
  ou com a via do `credential_guard.mode`. Se a medição confirmar que o agente alcança `$HOME`, este
  ADR **precisa ser reaberto** — não emendado de novo.

**Honestidade sobre o que temos hoje:** nenhum escopo protege contra um agente induzido com acesso de
escrita irrestrito. O que este ciclo entregou é **detecção** da classe do incidente real
(`credential_guard_hook_resolvable`), não **prevenção** contra adversário ativo.

## Alternatives Considered

**Endurecer o escopo de projeto** (enviar `failClosed`, implementar wrapper e integridade).
Rejeitado: mais superfície, duas classes de *bricking*, cobertura parcial por CLI, e ainda assim o
agente apaga o que está no workspace.

**Fazer os dois — global primeiro, projeto como defesa em profundidade.** Rejeitado por
custo/benefício: dobra o trabalho e adia o fechamento, para um ganho marginal sobre um adversário que
já é contido pelo escopo global. Pode ser reaberto se a hipótese "agente alcança `$HOME`" se
confirmar.

**Só documentar e aceitar o risco, sem mudar nada.** Rejeitado: deixaria o guard global como
opcional e desconhecido, e a decisão de arquitetura implícita. A diferença entre esta alternativa e a
decisão tomada é justamente **tornar o global o caminho padrão**.
