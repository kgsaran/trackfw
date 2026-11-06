---
status: Accepted
date: 2026-11-06
author: "Zeus (trackfw_architect)"
---

# ADR: o guard de credencial diz de onde veio o achado, e a segunda camada não lê diretório recursivamente

> Date: 2026-11-06 | Status: Accepted

## Context

A segunda camada do guard de credencial (REQ-2026-08-08) escaneia o conteúdo dos arquivos que o comando referencia, inclusive os que vêm da expansão de glob. Quando ela acha um padrão, a mensagem é a mesma do achado no payload: `possible <tipo> detected in tool payload`. Em 2026-11-06, isso fez o arquiteto procurar a credencial no comando, não achá-la e diagnosticar um falso positivo do regex. A reprodução está na REQ-2026-11-06 (guard de credencial). Pela mesma medição, `grep -n foo internal/guard/*.go` é bloqueado e `grep -rn foo internal/guard` passa.

## Decision

- **D1. A mensagem diz a origem.** Um achado vindo de arquivo referenciado é reportado como `possible <tipo> detected in file <caminho> referenced by the command`. O caminho é sanitizado: caractere de controle vira `?`, porque o caminho vem do comando e pode ser forjado. A forma `in tool payload` fica reservada ao achado no próprio payload. Vale nos dois escopos, nos dois modos e no sinal de atenção.
- **D2. A segunda camada não lê diretório recursivamente.** Ler `grep -r` ou `rg` num diretório teria custo sem limite: na raiz, seria o repositório inteiro a cada chamada de ferramenta. A camada continua cobrindo arquivo nomeado e glob. A limitação fica escrita no `--help` do `trackfw guard credential` e em `docs/cli-parity.md`, com o exemplo medido. A segunda camada é uma lombada para o caso óbvio, não uma fronteira contra quem quer contorná-la; é a mesma postura do checker de marcadores de terceiros.
- **D3. As fixtures do trackfw montam o token em tempo de execução.** O repositório que desenvolve o guard não pode bloquear quem trabalha nele. Os testes continuam afirmando a detecção sobre o valor montado.

## Consequences

- **Positivas:**
  - quem é bloqueado sabe qual arquivo abrir;
  - a limitação deixa de ser surpresa;
  - agentes podem fazer `grep` no código do guard.
- **Negativas:**
  - continua sendo possível ler um arquivo com segredo via `grep -r` sem bloqueio, agora de forma documentada;
  - o caminho do arquivo aparece no stderr.

## Alternatives Considered

- **Cobrir diretório com limite de bytes ou de arquivos.** Um limite qualquer cria um ponto de corte que o atacante conhece, e o custo cai em toda chamada. Rejeitada.
- **Allowlist de tokens falsos conhecidos.** Um token falso conhecido é o disfarce ideal. Rejeitada.
- **Não mudar a mensagem e só documentar.** O diagnóstico errado de 2026-11-06 aconteceu com quem conhece o guard. Rejeitada.
