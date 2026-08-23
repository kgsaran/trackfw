---
status: Open
date: 2026-08-23
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md"
---

# REQ: título de `roadmap new` forja seção com gate que o `barrier` executa

> Date: 2026-08-23 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Achado da barreira de 2026-08-22
(`docs/seguranca/2026-08-23-barreira-da-wave-0-no-harness.md`), **reproduzido por mim**:

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

**O vetor plantável não é o vetor perigoso.** O título vem de quem digita o comando — quem já
controla a máquina. O que importa é o **roadmap que chega por PR de terceiro**: o mantenedor roda
`trackfw barrier` para avaliar a wave e executa shell escrito pelo contribuidor, sem nunca ter
aceitado esse comando. É o fluxo normal de um projeto open-source, e é o fluxo deste repositório.

Decisão de desenho em
`ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md`.

## Acceptance Criteria

- [ ] **AC1** — `roadmap new` e `roadmap new --from-req` rejeitam ou neutralizam **newline e retorno
      de carro** no título, nos **3 CLIs**, com mensagem byte-idêntica.
- [ ] **AC2** — O título forjado do exemplo acima **não** produz bloco `**Gates da wave:**` extra —
      provado por fixture.
- [ ] **AC3** — `barrier` **recusa executar** o gate quando o roadmap é **não confiável**, com
      mensagem que diz por quê e o que fazer.
- [ ] **AC4** — O discriminante de confiança é **git**, não heurística de conteúdo. A forma exata —
      comparação contra base, contra `HEAD`, flag de consentimento — é **decidida pela Wave 0**,
      com o motivo registrado.
- [ ] **AC5** — 🔴 **O fluxo normal de implementação não pode virar fricção que faz desligar o
      controle.** Roadmap modificado localmente durante a wave é o caso **dominante**; a solução
      precisa ser medida contra ele, não só contra o caso hostil. (`ADR-2026-08-17`: guard que
      atrapalha é guard que o usuário desliga.)
- [ ] **AC6** — Recusa **não é silêncio**: o `barrier` reporta o gate como não avaliado, distinguindo
      de `passed`.
- [ ] **AC7** — Paridade nos 3 CLIs, com gate comparando **saídas reais**.
- [ ] **AC8** — Falsificação em **duas direções**: (a) sanitização removida do título é detectada;
      (b) `barrier` voltando a executar gate não confiável é detectado.
- [ ] **AC9** — `docs/cli-parity.md` com o contrato de confiança e a anotação `trackfw-contract`;
      checker de cobertura exit 0.
- [ ] **AC10** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0, com **exit code medido**.

### Acrescentados pela Wave 0 e pela auditoria dela

- [ ] **AC11** — Discriminante: comparação contra **`origin/main`**. `HEAD` **não serve** — o roadmap
      do PR **está** commitado na branch do PR, então HEAD-comparison o marcaria como confiável:
      fecharia a usabilidade sem fechar o vetor.
- [ ] **AC12** — O consentimento do fluxo dominante vem do **slash command**, não de flag digitada:
      `/trackfw:barrier` inclui a flag; a CLI direta não. Flag obrigatória universal viraria costume
      de digitá-la sempre — o *"guard que o usuário desliga"* do `ADR-2026-08-17`.
- [ ] **AC13** — 🔴 **Achado da minha auditoria da Wave 0:** o slash command vive **no repositório**
      (`.claude/commands/trackfw/barrier.md`). Um PR hostil pode **editar o próprio slash command**
      para incluir a flag e recuperar a execução. A entrega precisa dizer o que impede isso — ou
      declarar o residual explicitamente, com o motivo.
- [ ] **AC14** — O gate da direção (b) verifica **ausência do arquivo** criado pelo gate hostil, não
      só o código de saída: o defeito original **executava e depois reportava `blocked`**
      (`barrier.go:506-525` compõe o veredito **depois** de rodar os comandos).
- [ ] **AC15** — Falso-positivo é critério: o slash command com a flag **passa** em roadmap WIP sem
      interação — o caso dominante medido pela Wave 0.

## Negative scope

- **Não** remove o mecanismo de gates nem para de executar comando de shell — o ADR rejeita.
- **Não** implementa allowlist de comandos nem sandbox.
- **Não** trata roadmap **commitado e mergeado** com gate hostil: a fronteira ali é a revisão de
  código, e isso está declarado no ADR.
- **Não** muda a gramática de waves nem o contrato dos quatro checks do `barrier`.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md`
