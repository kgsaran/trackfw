---
status: Done
date: 2026-08-16
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-16-serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao.md"
---

# REQ: trackfw serve escuta em todas as interfaces sem autenticacao expondo a cadeia de governanca na rede

> Date: 2026-08-16 | Status: Open
| Linear Issue: 
| Jira Issue: 


## Motivation

**Exposição não autenticada de conteúdo em rede.** Encontrado pelo `hades-tf` ao varrer o `serve`
durante a barreira do vazamento de stack, e **verificado de forma independente pelo arquiteto**
antes de escalar.

### O que foi medido

```
$ trackfw serve --port 45871      (Go)
$ lsof -nP -iTCP:45871 -sTCP:LISTEN
trackfw 48350 kgsaran 3u IPv6 ... TCP *:45871 (LISTEN)      ← todas as interfaces

$ curl http://192.168.3.137:45871/     (IP da LAN da máquina)
HTTP 200
```

| CLI | endereço de escuta | onde |
|---|---|---|
| **Go** | **todas as interfaces** | `internal/serve/serve.go:59` — `addr := fmt.Sprintf(":%d", port)` |
| **Python** | **todas as interfaces** | `pypi/trackfw/commands/serve.py:150` — `HTTPServer(("", port), ...)` |
| Node | loopback | `npm/src/commands/serve.js:151` — `server.listen(port, '127.0.0.1', ...)` |

**Não há autenticação em nenhum ponto.** `/api/chain` devolve **105 KB** com a cadeia de governança
inteira — ADRs, REQs e roadmaps — para qualquer dispositivo na mesma rede.

### Alcance delimitado — para não exagerar a severidade

`/api/file` responde **403** tanto para um documento fora das pastas permitidas quanto para
tentativa de path traversal (`../../../etc/passwd`). **A restrição de caminho funciona.** O problema
é a **interface de escuta** somada à **ausência de autenticação** — não é leitura arbitrária de
arquivo do sistema.

### Severidade

**Moderada a alta para confidencialidade**, superior à do vazamento de stack que originou a varredura:
aquele expunha *metadados de ambiente*; este expõe **conteúdo** — decisões de arquitetura, requisitos
e, neste projeto especificamente, os ADRs que descrevem **controles de segurança, seus limites e
brechas conhecidas**.

**Atenuantes, declarados:** exige estar na mesma rede; o `serve` é ferramenta de desenvolvimento,
ligada sob demanda e normalmente efêmera; não há execução de código nem escrita.

**Cenários realistas de exposição:** Wi-Fi de café, coworking, hotel, LAN corporativa, e rede de
conferência — justamente onde se costuma trabalhar com o dashboard aberto.

### O indício de que foi descuido, não decisão

**O Node amarra em loopback e os outros dois não.** Se a exposição fosse intencional, seria
consistente nos três. É divergência de paridade que, neste caso, tem consequência de segurança.

## Acceptance Criteria

- [ ] **AC1** — Por padrão, os **3 CLIs** escutam em **loopback** (`127.0.0.1`). Verificação por
      `lsof`/equivalente mostrando o endereço, **não** apenas por leitura de código.
- [ ] **AC2** — Exposição em rede é **opt-in explícito** (ex.: `--host 0.0.0.0`), nunca o padrão.
- [ ] **AC3** — Ao expor, o comando emite **aviso claro** de que a cadeia de governança ficará
      legível sem autenticação por qualquer dispositivo alcançável.
- [ ] **AC4** — Comportamento idêntico nos 3 CLIs, incluindo o texto do aviso e o do `--help`.
- [ ] **AC5** — Gate de paridade cobrindo o endereço de escuta padrão, com **cenário de
      falsificação** conforme P4 do `ADR-2026-07-26-principios-de-design-de-gates-verificaveis`.
- [ ] **AC6** — Não-regressão: `serve` continua funcionando normalmente em `localhost`, e o
      dashboard segue acessível como hoje para quem usa a máquina.
- [ ] **AC7** — `make quality` verde.

## Escopo negativo

- **Não** adiciona autenticação ao `serve`. Se um dia for exposto de propósito, autenticação vira
  REQ própria — misturar as duas coisas atrasa a correção do padrão inseguro, que é o urgente.
- **Não** altera o que o `serve` serve, nem as rotas, nem o dashboard.
- **Não** mexe na restrição de caminho do `/api/file`, que **está funcionando** (403 verificado).

## Linked ADR

ADR: (a decidir — AC2 pode merecer registro por ser mudança de comportamento padrão)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-16-serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao.md`

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
