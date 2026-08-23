---
status: Accepted
date: 2026-08-23
author: "Zeus (Arquiteto)"
---

# ADR: `barrier` não executa gate de roadmap não confiável, e `roadmap new` sanitiza o título

> Date: 2026-08-23 | Status: Accepted

## Context

`trackfw barrier` executa o bloco `**Gates da wave:**` de um roadmap via
`exec.Command("sh", "-c", …)` (`internal/commands/barrier.go:385`), **sem sanitização**. Isso é
deliberado desde o `ADR-2026-07-26`: o gate é o comando que o arquiteto escolheu para provar a wave, e
executá-lo é a função do comando.

**A barreira de 2026-08-22 encontrou que o título de `roadmap new` é interpolado sem sanitizar
newlines** (`internal/generators/roadmap.go:150`). Reproduzido por mim:

```
titulo: "forjado\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n```bash\ntouch /tmp/PWNED_TEST\n```"

roadmap gerado, linha 12:  **Gates da wave:**
                     14:  touch /tmp/PWNED_TEST

$ trackfw barrier <roadmap> --wave 0
  gates: passed
  result: blocked          <- e o comando forjado EXECUTOU assim mesmo
$ test -f /tmp/PWNED_TEST  ->  EXISTE
```

**"Bloqueado" não significa "não executou":** os gates rodam antes de o veredito ser composto.

### O vetor plantável não é o vetor perigoso

O título vem de quem digita o comando — quem já controla a máquina. **O vetor que importa é outro:**
um roadmap que chega por **PR de terceiro**. O mantenedor roda `trackfw barrier` para avaliar a wave
e executa shell escrito pelo contribuidor, sem nunca ter aceitado esse comando.

Isso não é hipótese de laboratório: é o fluxo normal de um projeto open-source, e é o fluxo que este
próprio repositório usa.

## Decision

**1. `roadmap new` (e `--from-req`) sanitiza o título.** Newline e retorno de carro são rejeitados ou
neutralizados, nos 3 CLIs. O título é dado de **uma linha** — qualquer coisa além disso é entrada
malformada, não conteúdo.

**2. `barrier` distingue roadmap confiável de não confiável, e não executa o gate do segundo.**

O discriminante é **git**, não heurística de conteúdo: um roadmap cujo conteúdo **difere do que está
commitado na branch base** (ou que não existe nela) é **não confiável** para efeito de execução de
gate. Nesse caso o `barrier` **recusa executar** e diz por quê, em vez de executar e reportar.

Consequência aceita e desejada: rodar `barrier` sobre um roadmap **modificado localmente e ainda não
commitado** — o caso normal durante a implementação — precisa de consentimento explícito. A forma
desse consentimento (flag, confirmação, ou comparação contra `HEAD` em vez da base) é decisão de
implementação, e a **Wave 0 desta REQ ataca a escolha**.

**3. O que NÃO muda:** o mecanismo de gates continua existindo e continua executando comando de
shell. A decisão não é "parar de executar gates" — é **parar de executar gate de origem que o
operador não aceitou**.

## Consequences

**Positivas**
- Fecha o caminho de execução por PR de terceiro, que é o realista.
- A sanitização do título fecha o vetor plantável por comando, que é o barato.
- O discriminante ancorado em git é verificável e não depende de adivinhar intenção.

**Negativas e riscos aceitos**
- **Atrito no fluxo normal.** Durante a implementação o roadmap está sempre modificado e não
  commitado; exigir consentimento a cada `barrier` é o tipo de fricção que faz o usuário desligar o
  controle — o padrão que o `ADR-2026-08-17` nomeia. **É o risco dominante desta REQ**, e a Wave 0
  precisa dimensioná-lo antes da implementação.
- **Não fecha a classe inteira.** Um roadmap **commitado e mergeado** com gate hostil continua sendo
  executado — é o mesmo que confiar em `Makefile` ou script de CI que veio no merge. A fronteira é a
  revisão de código, não o `barrier`.
- Mais estado de git na avaliação do `barrier`, que hoje é quase puro sobre o arquivo.

## Alternatives Considered

**Só sanitizar o título.** Rejeitada como solução completa: fecha o vetor plantável por comando e
deixa aberto o vetor por PR, que é o que realmente importa. Entra como parte 1 da decisão, não como
tudo.

**Sanitizar o conteúdo do gate (allowlist de comandos).** Rejeitada: transforma o `barrier` num
interpretador de política de comandos, com todas as fugas conhecidas de allowlist de shell — e o gate
existe justamente para rodar o comando arbitrário que o projeto escolheu.

**Nunca executar gates; só reportar o comando.** Rejeitada: destrói o valor do `barrier`, que é
avaliar a wave de forma determinística. A `Wave 0` do harness mostrou que gate declarado e não
executado vira decorativo.

**Sandbox para o gate.** Rejeitada nesta REQ por custo desproporcional e por não ser portável entre
os 3 runtimes. Registrada como caminho futuro se o modelo de ameaça mudar.
