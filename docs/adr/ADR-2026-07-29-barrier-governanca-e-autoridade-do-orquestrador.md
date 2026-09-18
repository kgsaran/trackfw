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

11. **Gates são declarados no roadmap, não em nova chave de configuração.** Cada wave declara
    seus gates em um bloco `**Gates da wave:**` seguido de fence ```bash. A alternativa —
    introduzir uma chave `gates:` em `trackfw.yaml` — foi rejeitada nesta entrega porque exigiria
    alterar os parsers de configuração dos três runtimes (`internal/config/config.go`,
    `npm/src/`, `pypi/trackfw/`), arquivos que não constam no escopo dos MLs 2A/2B/2C. O roadmap
    já é lido pelo comando e já é um artefato do projeto, satisfazendo "gates declarados pelo
    projeto" sem nova superfície pública.
12. **Exit code 2 é distinto de `blocked`.** Uma barrier que não pôde ser avaliada (roadmap ou
    wave inexistente, roadmap malformado) é um erro de uso, não uma reprovação. Confundir os dois
    permitiria que um roadmap malformado fosse lido como "wave reprovada" e mascararia o defeito
    real.
13. **Ausência de bloco de critérios de aceite em um ML reprova a wave.** A regra é
    deliberadamente não-vacuosa: um ML sem critérios não passa "por não ter o que falhar".

14. **A remoção dos aliases é breaking change e NÃO é registrada no `CHANGELOG.md` durante os MLs.**
    O protocolo de release do projeto edita o `CHANGELOG.md` exclusivamente no PR de release, junto
    com o bump de versão, nunca em commit separado. O ML que remove os aliases não toca o arquivo.
    Registro para o PR de release consumir:

    > **BREAKING CHANGE:** os aliases de integração `trackfw copilot`, `trackfw cursor`,
    > `trackfw gemini`, `trackfw windsurf` e `trackfw amazonq` foram removidos. Use
    > `trackfw agents` e `trackfw skills`. Os aliases existiam apenas no CLI Go; Node.js e Python
    > nunca os registraram. As superfícies de instalação marcadas como `legacy` no catálogo
    > **não** foram removidas — elas não são aliases de CLI e continuam necessárias para migração.

15. **Wave é identificada por um rótulo, não por um inteiro** (emenda de 2026-07-29,
    REQ-2026-07-29-barrier-aceita-wave-com-sufixo-bis). A decisão original tratava `--wave` como
    "inteiro ≥ 1". A prática desmentiu a suficiência disso: waves corretivas acrescentadas **depois**
    que uma wave já foi executada e commitada precisam de um rótulo que sinalize a correção sem
    renumerar as waves seguintes, já citadas em mensagens de commit. `Wave 2-bis` é a nomenclatura
    natural. Constatado empiricamente no roadmap
    `install-pula-artefato-desatualizado-em-vez-de-abortar` (PR #86), onde a auditoria cruzada da
    Wave 2 exigiu wave corretiva e o barrier reprovou **as quatro waves** com
    `malformed wave heading`.

    Gramática do rótulo: `<inteiro>[-<sufixo>]`, sufixo `[a-z0-9]+`. `--wave` aceita o rótulo
    verbatim. Rótulos são identidades distintas: `--wave 2` **não** casa com `Wave 2-bis`.

    **Emenda ML-1B (2026-09-18, REQ #392):** o sufixo passa a ser aceito **insensível a caixa**,
    alargando de `[a-z0-9]+` para `[a-zA-Z0-9]+`. Quatro roadmaps com `## Wave 3-Py` que eram
    rejeitados por inteiro passam a parsear corretamente. Motivo: mesma causa da decisão 11 desta
    ADR — dialeto que o verificador não aceitava. Gramática atualizada: `[a-zA-Z0-9]+`.

    **Emenda ML-1D (2026-09-18, REQ #392):** o hífen antes do sufixo torna-se **opcional**. Dois
    roadmaps reais usavam `## Wave 1b` (sufixo sem hífen) e permaneciam invisíveis mesmo após a
    emenda ML-1B. `WaveLabelRe` passa de `^\d+(?:-[a-zA-Z0-9]+)?$` para
    `^\d+(?:-?[a-zA-Z0-9]+)?$`. `SplitWaveLabel("1b")` devolve `(1,"b")`, idêntico a
    `SplitWaveLabel("1-b")`; portanto `CompareWaveLabels("1b","1-b") == 0` e `--wave 1-b` resolve
    `## Wave 1b`. Rótulos puramente alfabéticos (`reaberta`) continuam inválidos — prefixo inteiro
    (`^\d+`) obrigatório. Gramática final: `^\d+(?:-?[a-zA-Z0-9]+)?$`.

16. **Heading fora da gramática continua abortando o documento inteiro — é feature, não defeito.**
    Durante a análise da emenda 15 considerou-se escopar o erro à wave solicitada, tornando as
    demais headings malformadas inócuas. **Rejeitado.** Ignorar silenciosamente uma heading
    malformada faria os MLs contidos nela **deixarem de ser auditados**: um typo (`## Wave X — ...`)
    produziria barrier verde sobre trabalho não verificado. É a mesma vacuidade que a decisão 13
    proíbe — um ML "passar por não ter o que falhar". O parser não pode escolher entre "ignorar o que
    não entende" e "reprovar"; deve reprovar alto.

    **Emenda ML-1E (2026-09-18, REQ #392) — princípio preservado, remédio corrigido:** o remédio
    original ("abortar o documento inteiro") produzia o defeito que a própria decisão pretendia
    evitar: dois roadmaps reais (`convergencia-do-harness` e `serve-amarra`) com `## Wave 1b`
    ficavam **completamente invisíveis** ao `barrier --wave N` para qualquer `N` válido, pois o
    documento inteiro era descartado. Trabalho não auditado — mas por razão oposta ao cenário do
    `## Wave X`: não por ignorar, mas por descartar tudo.

    O princípio ("reprovar alto", nunca verde sobre trabalho não auditado) **está preservado e é
    inviolável**. O que muda é o remédio: heading malformada vira um **check próprio** (`wave_headings`)
    que entra no veredito e **bloqueia**. As waves válidas continuam sendo avaliadas — a cegueira
    não volta. O documento inteiro recebe veredito `blocked`, não `exit 2`, e cada heading
    malformada é nomeada com linha e token tanto em stderr quanto no campo `failures` do check.

    Resumo da propriedade preservada: `barrier` nunca emite `status: "passed"` enquanto houver
    heading malformada no documento. A diferença em relação ao remédio anterior é que as waves
    válidas *também* são avaliadas — o usuário sabe quais passaram e quais falharam, e nenhuma
    delas escapa à auditoria por estar no mesmo documento que uma heading com typo.

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
- Introduzir a chave `gates:` em `trackfw.yaml` (adiada — ver decisão 11). Se surgir demanda por
  gates repo-wide independentes de roadmap, será uma REQ própria, com os três parsers de
  configuração no escopo.
