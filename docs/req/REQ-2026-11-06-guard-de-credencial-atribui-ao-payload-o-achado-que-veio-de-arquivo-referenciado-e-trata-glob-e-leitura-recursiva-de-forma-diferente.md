---
status: Open
date: 2026-11-06
author: ""
adr: "docs/adr/ADR-2026-11-06-guard-de-credencial-diz-de-onde-veio-o-achado-e-nao-le-diretorio-recursivamente.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md"
---

# REQ: guard de credencial atribui ao payload o achado que veio de arquivo referenciado e trata glob e leitura recursiva de forma diferente

> Date: 2026-11-06 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Em 2026-11-06, o guard de credencial global (11.5.0) bloqueou duas vezes comandos de leitura inofensivos do arquiteto, com as mensagens `possible JWT detected in tool payload` e `possible PEM private key detected in tool payload`. Nenhum dos dois comandos continha credencial. Reprodução com payload sintético em `trackfw guard credential --global`, cwd na raiz do repositório:

| comando | rc | mensagem |
|---|---|---|
| `grep -n foo internal/guard/*.go` | 2 | possible PEM private key detected in tool payload |
| `grep -n foo internal/generators/*_test.go` | 2 | possible JWT detected in tool payload |
| `cat internal/generators/credential_guard_test.go` | 2 | possible JWT detected in tool payload |
| `grep -n foo internal/guard/gitbranch.go` | 0 | |
| `grep -rn foo internal/guard` | 0 | |

**Causa.** A segunda camada (REQ-2026-08-08) expande os globs dos argumentos e escaneia o **conteúdo** dos arquivos. Os achados vieram de fixtures de teste com token falso (`credential_guard_test.go`, `credential_patterns_test.go`, `credential.go`). Três defeitos:

1. **Atribuição errada.** A mensagem diz "in tool payload" quando o achado veio de um arquivo referenciado. Quem lê procura a credencial no comando, não acha, e conclui que é falso positivo do regex. Foi exatamente o que aconteceu.
2. **Cobertura inconsistente.** O mesmo conteúdo é bloqueado via glob e liberado via `grep -r` no diretório. A proteção depende da sintaxe do comando, não do que é lido, e nada documenta isso.
3. **O repositório do próprio trackfw bloqueia quem trabalha nele.** As fixtures têm tokens literais, então qualquer agente que faça `grep` em `internal/guard/*.go` é bloqueado.

A REQ-2026-08-08 (Done) previa: "se `block` quebrar fluxos legítimos, documentar o trade-off e a mitigação". Essa mitigação não foi feita, e esta REQ a completa. A varredura de issues abertas não achou o mesmo mecanismo.

## Acceptance Criteria
- [ ] **AC1.** Quando o achado vem de um arquivo referenciado, a mensagem cita o caminho do arquivo e diz que veio do conteúdo, não do payload. Isso vale nos dois escopos (projeto e global) e nos dois modos (`block` e `warn`), e também no sinal de atenção. Achado no payload continua dizendo "in tool payload". Um teste fixa as duas formas.
- [ ] **AC2.** A segunda camada não lê diretório recursivamente (ADR D2). A limitação fica documentada no `--help` do `trackfw guard credential` e em `docs/cli-parity.md`, com o exemplo medido de glob versus `grep -r`.
- [ ] **AC3.** As fixtures do repositório deixam de conter tokens literais que casem com os padrões: o token é montado em tempo de execução. Depois disso, `grep -n foo internal/guard/*.go` e `grep -n foo internal/generators/*_test.go` dão rc=0 no guard. Os testes continuam afirmando a detecção. As cópias congeladas de corpus (`internal/roadmapdoc/testdata/corpus/`, `scripts/testdata/roadmap-barrier-corpus-snapshot/`) e os documentos históricos (`docs/roadmaps/done/ROADMAP-2026-08-05-*`, `docs/seguranca/2026-11-05-wave0-guards-primeiro-usuario-externo.md`) ficam como estão: o corpus é fixture byte-a-byte de outros testes.
- [ ] **AC4.** `make quality` verde.

## Negative Scope

- Não muda os padrões de detecção nem o modo padrão (`block` no global).
- Não cria allowlist de tokens conhecidos: um token falso conhecido é exatamente o que um atacante usaria para se esconder.
- Não tenta entender a semântica de `grep` (se a linha casada seria impressa); o guard continua tratando a leitura do arquivo como exposição.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-06-guard-de-credencial-diz-de-onde-veio-o-achado-e-nao-le-diretorio-recursivamente.md

Origem: ADR-2026-08-06 e REQ-2026-08-08 (segunda camada).

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md
