---
status: Accepted
date: 2026-11-06
author: "Zeus (trackfw_architect)"
---

# ADR: CI vermelho por infraestrutura se distingue de vermelho por conteúdo

> Date: 2026-11-06 | Status: Accepted

## Context

Três vermelhos recentes não tinham relação com o conteúdo do PR:

1. **Fila do macOS (Discussion #664).** Em 03/11, no fork `lourivalgarciajunior/trackfw`, o braço `shim-byte-identity (macos-latest)` foi cancelado 13 vezes seguidas, sem runner, no limite de fila de cerca de 902 s, durante 8 h 46 min. No mesmo período, o braço `ubuntu-latest` passou. O `shim-byte-identity-gate` (required, ML-4E) só aceita `success`, então todo PR ficou vermelho, inclusive os que só mexiam em documentação. Os dois vermelhos esperados treinaram quem lia a ignorá-los e esconderam um achado real, o #665.
2. **Prazo do PowerShell 5.1 (#665).** O aquecimento do `powershell.exe` levou 8,00 s, dentro da faixa verde que o #652 mediu (2,91 a 9,86 s). Mesmo assim, a chamada afirmada de `RC0-allow` estourou o prazo fixo de 15 s. No re-run, com o mesmo `head`, o aquecimento levou 1,91 s e a chamada 1,50 s. O aquecimento mede a lentidão da máquina e não a absorve.
3. **Porta fixa (PR #666).** O `check-serve-address-parity.sh` usa portas fixas a partir de 46199. Em 04/11, `0.0.0.0:46202` estava ocupada no runner (`bind: address already in use`), e o cenário reprovou sem relação com o código.

## Decision

- **D1 — O gate do shim distingue "não aplicável" de "não verificado", por lista fechada de caminhos inertes.** Em evento `pull_request`, o `shim-byte-identity-gate` calcula o diff do PR contra a base com `git diff --name-only --no-renames <merge-base>..<head>` (um rename lista os dois caminhos). O gate só dispensa os braços quando **todos** os arquivos do diff estão num conjunto fechado de caminhos que não entram no build nem no shim. Nesse caso ele passa e escreve no log `não aplicável: o diff só toca caminhos inertes`, com a lista. Um arquivo fora do conjunto, mesmo novo ou desconhecido, torna o gate aplicável. Assim, se o build ganhar uma entrada nova, o gate continua exigindo os braços. Os candidatos ao conjunto inerte são `docs/`, `vault/`, `.claude/agent-memory/` e os `*.md` da raiz; a Wave 0 fecha o conjunto.
- **Emenda.** Este ADR emenda o ML-1D (AC9 da REQ-2026-09-12-v8, "byte-identidade nos três SOs") e o ML-4E (`shim-byte-identity` vira required) do `ROADMAP-2026-09-12-v8-um-binario-muitos-canais`. O AC9 passa a ser afirmado em todo PR que toca algo fora do conjunto inerte, em todo push para a `main` e em toda tag. Em PR só com caminhos inertes, ele não é medido, porque não há o que medir.
- **D2 — Fail-closed.** Se o diff não puder ser calculado (base ausente, clone raso, evento diferente de `pull_request`), o gate exige `success` dos dois braços, como hoje. No `push` para a `main` e na tag, o braço macOS continua obrigatório. O nome do check no conjunto R não muda.
- **D3 — O prazo da chamada afirmada do PowerShell deriva do aquecimento medido.** O prazo passa a ser `clamp(4 × aquecimento, 15 s, 60 s)`. A asserção continua sendo o `exit 0` e o stdin preservado; o prazo é orçamento do instrumento. A mensagem de estouro passa a citar o aquecimento e o prazo efetivo. **O fator 4 é palpite:** ele se apoia em dois pontos medidos (8,00 s com estouro e 1,91 s com 1,50 s) e no intervalo verde do #652. Também é preciso que o `-timeout` do `go test` no `windows-full-suites` comporte o novo teto multiplicado pelo número de chamadas afirmadas, senão o estouro só muda de lugar.
- **D4 — O `check-serve-address-parity.sh` não depende de porta fixa.** O `trackfw serve --port 0` não serve aqui, porque imprime `localhost:0` em vez da porta real (medido em 2026-11-06). Por isso, cada cenário escolhe uma porta livre fora da faixa fixa. Se o `serve` falhar com `address already in use`, o cenário tenta outra porta, até 3 vezes. Qualquer outro erro continua reprovando na primeira tentativa. A colisão do PR #666 foi no endereço curinga (`0.0.0.0`), então a porta livre tem de ser verificada no endereço do próprio cenário, e não só em `127.0.0.1`. Nenhum outro script ou teste do repositório usa porta fixa na faixa 46xxx; a varredura foi feita em `scripts/`, `.github/` e `internal/`.

## Consequences

- **Positivas:**
  - PR de documentação e governança deixa de ficar vermelho por falta de máquina macOS;
  - vermelho volta a significar defeito;
  - o teste do PowerShell 5.1 se adapta à máquina sem deixar de afirmar o resultado;
  - colisão de porta deixa de reprovar o `parity-rest`.
- **Negativas:**
  - num PR que não toca o shim, a byte-identidade no macOS só é medida no push para a `main`;
  - se a máquina estiver lenta de verdade, o prazo do PowerShell pode chegar a 60 s;
  - o novo retry de porta é código de teste a mais.

## Alternatives Considered

- **Não mudar nada (opção 1 da #664).** O custo de esperar volta a cada episódio de fila, e os vermelhos esperados escondem achado real. Rejeitada.
- **Tirar o macOS do PR (opção 3 da #664).** Muda o que a AC9 afirma em cada PR, inclusive nos que tocam o shim. Rejeitada.
- **`continue-on-error` ou aceitar `cancelled`.** Seria verde sobre nada verificado, exatamente o que o comentário do ML-4E recusa. Rejeitada.
- **Subir o prazo fixo do PowerShell.** É um número escolhido para a máquina de hoje, como foram os 15 s. Rejeitada.
- **Ratchet do Windows distinguir estouro de prazo de asserção (candidato 2 da #665).** O ratchet seguiria sem remédio para o consumidor. Fica fora deste ADR.
