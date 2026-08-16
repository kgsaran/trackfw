---
title: "integrations: artefatos são escritos ANTES do manifesto ser persistido — janela real de registro parcial"
date: 2026-08-16
autor: Apolo (ML-2C, ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0)
tags: [integrations, manager, manifest, atomicidade, unmanaged-artifact]
---

## Contexto

Bug reportado em uso real: `trackfw agents update --force` falhou com `unmanaged artifact
".../.codex/agents/trackfw-iac.toml" does not match a trackfw template`. O comportamento estava
correto (ver `unmanagedArtifactError` em `internal/integrations/manager.go`), mas o artefato
nunca deveria ter chegado a esse estado — o histórico mostrava 12 arquivos em disco com o MESMO
timestamp, dos quais só 10 constavam no `integrations-manifest.json`. Não é legado: os agentes
`iac`/`tooling` entraram no catálogo em 2026-07-26 (#72) e o manifesto existe desde 2026-07-19
(#50) — ou seja, os 12 arquivos deveriam ter sido escritos e registrados juntos, na mesma
chamada de `install`.

## O que foi investigado

Em `Manager.mutate()` (`internal/integrations/manager.go:145-272`), o fluxo por chamada é:

1. `preflight` roda para TODOS os itens do batch (valida, decide skip).
2. Snapshot de todos os arquivos ativos + manifestos (para rollback).
3. **Loop 1**: para cada item ativo, `applyMutation` grava os bytes do artefato em disco
   (`atomicWrite`, linha ~429) E atualiza o objeto `Manifest` **em memória**.
4. **Loop 2** (só começa depois que o Loop 1 termina inteiro): `writeManifest` persiste cada
   manifesto tocado em disco, um por scope (`project`/`global`), ordenados por nome de arquivo.
5. `committed = true` — só a partir daqui o `defer` de rollback deixa de reverter em caso de erro
   de retorno normal (`return err`).

**Achado**: os bytes de TODOS os artefatos do batch são gravados em disco no Loop 1 —
antes de QUALQUER manifesto ser persistido no Loop 2. Isso cria uma janela real (não hipotética)
em que N arquivos existem em disco, corretos, com o mesmo timestamp, mas nenhum manifesto ainda
foi escrito. Se o processo for interrompido nessa janela — Ctrl-C, `kill`, crash, falha de disco,
terminal fechado — os arquivos permanecem, mas nenhuma entrada de manifesto foi persistida.
Isso bate exatamente com o sintoma relatado (12 arquivos, só 10 registrados: plausivelmente 2
scopes — 10 itens de um scope + 2 de outro — e a interrupção caiu entre a persistência do
manifesto do primeiro scope e do segundo, no Loop 2, OU caiu em qualquer ponto após o Loop 1
completar e antes do Loop 2 terminar).

**Não é uma falha de lógica no sentido de "código errado que sempre reproduz"** — em execução
limpa (sem interrupção externa), o `defer` de rollback (linhas 239-251) reverte tanto arquivos
quanto manifestos caso `mutate()` retorne erro por qualquer motivo ANTES de `committed = true`.
O gap só se manifesta com interrupção não tratável em Go (SIGKILL, crash do processo, queda de
energia, disco cheio no meio da gravação) — o `defer` nunca roda nesses casos. Isso é inerente a
qualquer operação que grava múltiplos arquivos físicos sem um WAL/journal cross-file; não é
uma regressão introduzida por um commit específico.

## Por que isso importa (para quem for corrigir)

- **Não é dead code nem falso-positivo**: é uma janela real e alcançável por qualquer usuário que
  interrompa um `install`/`update` grande (muitos itens/targets) no momento errado.
- **Correção não é só mensagem**: se isso for endereçado, precisa de DETECÇÃO (ex.: um
  `trackfw validate` rule ou um passo de `agents list`/`doctor` que sinaliza "arquivo com hash
  batendo o template do catálogo, mas ausente do manifesto — provável escrita não registrada") e/ou
  mudar a ORDEM da gravação (persistir o manifesto por item, ou pelo menos por scope, span mais
  curto) — mudança de comportamento, não de texto. Ambas fora do escopo do ML-2C (que é
  estritamente mensagem de erro + help do `--force`).
- Mirror do mesmo padrão existe nos 3 CLIs (`npm/src/integrations/manager.js` função `mutate` e
  `pypi/trackfw/integrations/manager.py` método `_mutate`) — qualquer fix de detecção/ordenação
  precisaria ser replicado nos 3.

## Decisão tomada nesta sessão

Não implementado — reportado ao arquiteto (`trackfw_architect`) para decisão de escopo, conforme
instrução do ML. Only mensagem de erro (`unmanagedArtifactError`) e help do `--force`
(`forceHelp`) foram alterados neste ML.

## Ver também

- `internal/integrations/manager.go` — `mutate`, `applyMutation`, `unmanagedArtifactError`
- `npm/src/integrations/manager.js` — `mutate`, `apply`, `unmanagedArtifactError`
- `pypi/trackfw/integrations/manager.py` — `_mutate`, `_apply`, `_unmanaged_artifact_error`
