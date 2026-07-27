---
status: Done
date: 2026-07-26
author: "KG"
adr: "docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md"
---

# REQ: comando trackfw ship agnostico de forge

> Date: 2026-07-26 | Status: Done

## Motivation

O trackfw sabe **validar** (`trackfw validate`) mas não sabe **entregar**. O fluxo de entrega vive
hoje numa skill pessoal do KG (`git-ship`, 122 linhas) — a única peça de harness git internamente
coerente entre os artefatos pessoais, e a última sem destino na convergência.

Portá-la como comando fecha o ciclo `validate → ship` e amarra a entrega à cadeia
ADR→REQ→ROADMAP: sem roadmap em `wip`, não há entrega.

O requisito central é que a abertura de PR/MR seja **agnóstica de forge** — o trackfw é open-source
e não pode assumir GitHub. Decisões e mapeamento completo no ADR vinculado.

## Escopo

1. Comando `trackfw ship` nos 3 CLIs (Go, Node.js, Python).
2. Campo `forge:` em `config.ProjectConfig`, lido de `trackfw.yaml`.
3. Pergunta de forge no wizard do `trackfw init`.
4. Detecção de forge no `trackfw discover` (host do remote + desempate por CI detectado).
5. Resolução por precedência: flag → config → remote → CI → modo manual.
6. Suporte a `github` (`gh pr create`), `gitlab` (`glab mr create`), `azure` (`az repos pr create`)
   e `bitbucket` (fallback para URL).
7. Degradação graciosa quando o CLI da forge não está instalado: push concluído + URL impressa.
8. Validação de governança antes do commit: REQ e roadmap em `wip`.

## Escopo negativo (o que NÃO fazer)

- **Não** fazer merge de PR/MR — a decisão de merge é sempre do usuário.
- **Não** fazer `git push --force` em nenhuma circunstância.
- **Não** usar `git add .` — apenas arquivos revisados explicitamente.
- **Não** falhar quando o CLI da forge estiver ausente — degradar para URL.
- **Não** criar branch: o comando opera na branch atual (criação é do orquestrador).
- **Não** implementar suporte a forges além das 4 listadas nesta REQ.
- **Não** alterar `trackfw validate` — apenas consumi-lo.
- **Não** incluir trailer de modelo de IA nem sufixo de nome de agente na mensagem de commit.

## Acceptance Criteria

- [x] `trackfw ship` existe e se comporta de forma idêntica nos 3 CLIs
- [x] `--forge` sobrepõe o valor de `trackfw.yaml`, que sobrepõe a detecção pelo remote
- [x] Remote `github.com/...` resolve `github`; `gitlab.com/...` resolve `gitlab`
- [x] Host desconhecido com `.gitlab-ci.yml` presente resolve `gitlab` (caso self-hosted)
- [x] Saída usa "Merge Request" quando a forge é `gitlab` e "Pull Request" nas demais
- [x] Com o CLI da forge ausente: exit 0, push concluído e URL de criação impressa
- [x] Em branch `main`/`master`: aborta com mensagem clara e exit não-zero
- [x] Sem roadmap em `wip`: aborta e orienta a criar REQ e roadmap
- [x] `trackfw init` pergunta a forge e persiste em `trackfw.yaml`
- [x] `trackfw discover` preenche `forge:` quando consegue detectar
- [x] `docs/cli-parity.md` atualizado com o comando novo
- [x] `make quality` verde nos 3 CLIs

## Linked ADR

ADR: docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Roadmap será criado quando esta REQ entrar em execução -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md
