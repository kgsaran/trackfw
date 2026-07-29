---
status: Open
date: 2026-07-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md"
---

# REQ: barrier de governança e autoridade exclusiva do orquestrador

> Date: 2026-07-29 | Status: Open

## Motivação

O conceito de barrier existe nos roadmaps e no agente arquiteto, porém não há um contrato
executável que impeça o avanço prematuro entre waves. A validação precisa reunir conclusão dos
MLs, testes, build, inspeção dos critérios de aceite, qualidade, segurança e gates definidos pelo
projeto.

O fluxo também precisa tornar explícita a separação de poderes: especialistas implementam e
reportam; somente o `trackfw_architect` orquestra, audita e executa operações Git. O nome exibido no
ambiente do usuário, como `zeus-tf`, não deve fazer parte do contrato.

## Escopo

1. Criar o comando nativo `trackfw barrier` nos CLIs suportados, com saída textual e JSON.
2. Criar o slash command `/trackfw:barrier` como instrução autocontida para o
   `trackfw_architect`.
3. Definir e validar o checklist universal da barrier:
   - todos os MLs da wave concluídos e marcados como concluídos;
   - testes unitários e E2E existentes para a wave executados;
   - build executado sem erros;
   - critérios de aceite inspecionados com evidências;
   - inspeção do agente de qualidade de código;
   - inspeção do agente de segurança, incluindo SAST, privilégios e controle de acesso aplicáveis;
   - gates pré-commit declarados pelo projeto executados;
   - diff e escopo auditados;
   - rastreabilidade e `trackfw validate` aprovados;
   - evidências registradas antes da liberação.
4. Definir estados de resultado `pending`, `running`, `passed` e `blocked`.
5. Em qualquer falha, bloquear a próxima wave e orientar o orquestrador a criar e despachar MLs
   corretivos; nenhuma nova wave pode ser liberada até uma nova barrier verde.
6. Atualizar os assets dos agentes para que especialistas não executem operações Git e só atuem por
   handoff do `trackfw_architect`.
7. Preservar a neutralidade de stack: gates específicos, como paridade entre runtimes, são
   configurados pelo projeto e não hardcoded no framework.
8. Remover os aliases deprecated `copilot`, `cursor`, `gemini`, `windsurf` e `amazonq` na nova
   versão, direcionando usuários para `agents|skills` antes da remoção.
9. Consolidar a ajuda explícita em um único `trackfw help`, preservando `--help` e
   `<comando> --help` como flags nativas de contexto.
10. Separar a atualização por escopo: manter `trackfw update` focado no projeto atual e criar
    `trackfw update harness` para atualizar o harness global já instalado em uma única execução.

## Fora de escopo

- Paridade Go/Node.js/Python como regra universal.
- Invocação automática de agentes externos pelo binário nativo.
- Commit ou push por subagents.
- Merge ou abertura automática de PR/MR sem autorização explícita do usuário.
- Alteração das identidades personalizadas; somente o papel canônico
  `trackfw_architect` será usado nas instruções.
- Manutenção de aliases deprecated de integração após a nova versão.
- Mutação de artefatos globais por `trackfw update` executado dentro de um projeto.
- Instalação silenciosa de agents ou skills que não existiam antes do update.

## Critérios de aceite

- [ ] `trackfw barrier <roadmap> --wave <n>` existe nos três CLIs e possui saída textual e `--json`.
- [ ] A barrier falha quando qualquer ML da wave não está marcado como concluído.
- [ ] A barrier falha quando falta evidência de build, testes, gates ou critérios de aceite exigidos.
- [ ] A barrier executa os comandos declarados pelo projeto sem assumir uma stack específica.
- [ ] `trackfw validate` é executado como parte da barrier e violações impedem a liberação.
- [ ] O resultado registra wave, checks, comandos, status, timestamps e mensagens de falha.
- [ ] Falha em qualquer check produz resultado `blocked` e impede a próxima wave.
- [ ] `/trackfw:barrier` instrui o `trackfw_architect` a invocar os agentes `code-quality` e `security`
      quando a mudança exigir análise especializada.
- [ ] Os agentes especialistas declaram que não executam operações Git e recusam trabalho direto
      sem handoff do `trackfw_architect`.
- [ ] Somente o `trackfw_architect` recebe autoridade documentada para branch, commit e push.
- [ ] O framework não contém regra universal de paridade entre implementações ou stacks.
- [ ] Os comandos `copilot`, `cursor`, `gemini`, `windsurf` e `amazonq` não existem mais no CLI Go.
- [ ] A documentação não orienta o uso dos aliases removidos e aponta para `agents|skills`.
- [ ] Existe uma única superfície explícita `trackfw help` nos três CLIs.
- [ ] `--help` continua funcionando no root e nos subcomandos.
- [ ] A documentação de configuração continua acessível pelo help consolidado.
- [ ] `trackfw update` atualiza somente regras, hooks, scripts, CI e comandos do projeto atual.
- [ ] `trackfw update` não altera agents, skills ou regras globais.
- [ ] `trackfw update harness` atualiza agents, skills e regras globais já instalados.
- [ ] `trackfw update harness` não exige `trackfw.yaml` nem estar dentro de um repositório.
- [ ] Nenhum dos dois comandos instala itens ausentes sem uma opção explícita.
- [ ] Ambos preservam artefatos desconhecidos ou não gerenciados e reportam-nos como ignorados.
- [ ] Go, Node.js e Python oferecem o mesmo contrato de update ou documentam uma exceção temporária
      com comportamento equivalente no fluxo suportado.
- [ ] `trackfw update --dry-run` e `trackfw update --json` permitem inspeção antes da alteração.
- [ ] `make quality` e `trackfw validate --json` passam no repositório trackfw.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
