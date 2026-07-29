---
status: Accepted
date: 2026-07-29
author: "trackfw_architect"
---

# ADR: barrier governança e autoridade exclusiva do orquestrador

> Date: 2026-07-29 | Status: Accepted

## Contexto

Os agentes do trackfw já descrevem waves, microlotes e barriers, mas a barrier ainda é uma
instrução textual. Não existe uma operação verificável que confirme a conclusão da wave, execute
os gates declarados pelo projeto, solicite inspeções especializadas e controle a liberação da wave
seguinte.

Também existe risco operacional quando agentes especialistas executam operações Git concorrentes.
O fluxo seguro adotado é centralizar toda autoridade Git no agente arquiteto, exposto ao usuário
por sua identidade canônica `trackfw_architect`. Identidades personalizadas, como `zeus-tf`, são
apenas nomes de apresentação e não alteram o papel.

## Decisões

1. O trackfw terá uma operação nativa `trackfw barrier` como núcleo determinístico de validação.
2. O slash command `/trackfw:barrier` será a camada de orquestração: orientará o
   `trackfw_architect`, invocará os agentes de qualidade e segurança e chamará o CLI.
3. Uma barrier só libera a próxima wave quando todos os checks obrigatórios estiverem verdes.
4. Gates de stack, como paridade entre implementações, não fazem parte do contrato universal.
   Projetos declaram seus próprios comandos em configuração ou no roadmap.
5. Agentes especialistas nunca executam `git checkout`, `git branch`, `git commit`, `git push`,
   `git merge`, `git rebase` ou operações destrutivas de Git.
6. Apenas o `trackfw_architect` cria branches, audita o diff, consolida alterações, faz commit e
   push e sugere PR/MR.
7. O contrato público referencia sempre `trackfw_architect`; nomes personalizados não aparecem nas
   regras de coordenação.
8. A nova versão removerá os aliases de integração deprecated (`copilot`, `cursor`, `gemini`,
   `windsurf` e `amazonq`). O fluxo canônico será exclusivamente `agents|skills`.
9. A ajuda explícita será consolidada em uma única superfície `trackfw help`; as flags nativas
   `--help` permanecem como mecanismo de ajuda contextual dos frameworks CLI.
10. O ciclo de atualização será separado por escopo: `trackfw update` atualiza somente artefatos
    do projeto atual; `trackfw update harness` atualiza o harness global já instalado, sem depender
    de um repositório específico.

## Consequências

### Positivas

- Waves passam a ter uma condição de saída objetiva e reproduzível.
- O framework permanece agnóstico de stack e de estratégia de paridade.
- O risco de agentes sobrescreverem ou apagarem trabalho de outros agentes diminui.
- A camada de agente pode executar inspeções sem transformar o CLI em um executor de especialistas.
- A superfície pública fica menor, sem aliases deprecated e sem duplicação da ajuda explícita.
- O usuário atualiza o harness global uma única vez, sem repetir a operação ao visitar vários
  repositórios.

### Negativas e limites

- A inspeção semântica dos critérios de aceite continua exigindo julgamento do orquestrador e dos
  agentes especializados; o CLI pode verificar evidências, mas não substituir essa análise.
- Projetos precisam declarar comandos de build, testes e gates para obter validação automatizada.
- A restrição de Git em alguns targets depende tanto da superfície de ferramentas quanto das
  instruções renderizadas; ambas devem ser testadas.

## Fora de escopo

- Definir uma lista universal de stacks ou ferramentas de qualidade.
- Fazer o CLI invocar diretamente subagents de qualquer fornecedor.
- Exigir paridade entre runtimes em todos os projetos.
- Alterar o nome personalizado exibido para o arquiteto.
- Remover as flags nativas `--help` dos frameworks.
- Fazer `trackfw update` mutar artefatos globais.
- Instalar automaticamente agents ou skills ausentes durante um update sem uma opção explícita.
