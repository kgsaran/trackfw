---
status: Done
date: 2026-08-15
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados.md"
---

# REQ: trackfw validate deve detectar scripts de hook ausentes ou desatualizados

> Date: 2026-08-15 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Achado ao vivo nesta sessão (discussão sobre gitignorar `scripts/trackfw-*.sh`): existe
uma regra de validação, `credential_guard_hook_resolvable`
(`internal/validator/validator_credential_guard.go`), cujo propósito documentado é
detectar quando um hook registrado referencia um script que não existe ou não é
executável. Testado ao vivo, 2 vezes, com causa raiz confirmada nos dois casos:

1. **Removi `scripts/trackfw-credential-guard.sh`** e rodei `trackfw validate --json` —
   zero violação/aviso. Causa raiz: NÃO é bug nesse caso — `.claude/settings.json` deste
   repo não tem entrada de credential-guard em escopo de projeto (instalado em escopo
   **global**, `~/.trackfw/scripts/trackfw-credential-guard.sh`, via
   `trackfw update harness`; o dedup `globalCredentialGuardInstalledClaude()` já omite a
   entrada de projeto de propósito). A regra corretamente não tinha nada para checar
   nesse arquivo específico — comportamento certo, não gap.
2. **Removi `scripts/trackfw-git-branch-guard.sh`** (que `.claude/settings.json` FAZ
   referenciar, em escopo de projeto, `$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh`)
   e rodei `trackfw validate --json` de novo — zero violação/aviso de novo. Causa raiz
   confirmada desta vez: **gap real**. `credential_guard_hook_resolvable` tem o nome do
   script hardcoded (`credentialGuardScriptMarker = "trackfw-credential-guard.sh"`) —
   ela nunca examina entradas que mencionam `trackfw-git-branch-guard.sh`. A regra foi
   escrita antes do guard de git existir e nunca foi estendida.

Além da checagem de **existência/executabilidade** (que já existe para
credential-guard, precisa ser estendida para git-branch-guard), o usuário pediu
explicitamente uma segunda dimensão: **desatualização por versão**. Todo hook/gate que o
`trackfw` gera deve ser preservado consistente com a versão do binário que o implantou —
se o `trackfw` for atualizado (nova versão do binário) e o conteúdo do script no disco do
projeto não bater mais com o que essa versão geraria (drift), `trackfw validate` deve
detectar isso, não silenciar. Já existe precedente para essa segunda dimensão, mas só
para credential-guard: `credential_guard_script_integrity`
(`internal/validator/validator_credential_guard_integrity.go`) compara o script no disco
byte-a-byte contra o template que o binário atual geraria. O git-branch-guard não tem
equivalente.

Em ambos os casos (ausente ou desatualizado), a mensagem de violação já usa o padrão
correto ("run `trackfw update` to regenerate it") — manter esse padrão, generalizado
para cobrir os dois scripts.

**Achado adicional (escopo global — pedido explícito do usuário, "contemple tb o
harness global"):** `globalCredentialGuardInstalledClaude()`
(`internal/generators/agentfiles.go`) — a função que decide se a entrada de PROJETO deve
ser omitida por já existir instalação global — vive no pacote `generators`, não no
`validator`, e só responde "está instalado" (dedup, side de geração). **Não existe hoje
nenhuma checagem, em `trackfw validate`, de que o script em
`~/.trackfw/scripts/trackfw-credential-guard.sh` (ou o equivalente de git-branch-guard,
uma vez implementado) realmente existe e está íntegro.** Ou seja: se o harness global
for instalado uma vez e depois o script sumir/ficar desatualizado no `$HOME` do usuário
(fora do repositório, `trackfw validate` nunca teria como saber pelo estado do projeto
sozinho), hoje nada acusa isso — o gap é maior no escopo global que no de projeto,
porque não há nem o esqueleto de regra que existe para o projeto.

## Acceptance Criteria
- [x] `credential_guard_hook_resolvable` (ou uma regra irmã nova,
      `git_branch_guard_hook_resolvable` — decisão de design do ML: generalizar a regra
      existente para aceitar uma lista de markers, ou duplicar como regra própria;
      preferir generalizar, já que a lógica de resolução de caminho por CLI é idêntica)
      passa a cobrir também `trackfw-git-branch-guard.sh`: hook registrado que referencia
      esse script, com o script ausente ou não-executável, gera violação/aviso
      instruindo `trackfw update`.
- [x] Nova regra (ou extensão de `credential_guard_script_integrity`) cobre
      `trackfw-git-branch-guard.sh`: script presente no disco mas com conteúdo diferente
      do que a versão atual do binário geraria (drift/desatualização) gera
      violação/aviso instruindo `trackfw update` — mesmo padrão já usado para
      credential-guard.
- [x] As duas checagens (resolvable + integrity) para git-branch-guard respeitam o
      mesmo escopo de severidade/configuração via `trackfw.yaml` (`rules:`) que
      credential-guard já usa — não hardcodar severidade nova sem seguir o padrão
      existente.
- [x] Cobertura para escopo GLOBAL também — dois sub-critérios distintos, não confundir:
      (a) **dedup (já existe, preservar)**: quando o global está instalado, `validate`
      não deve reportar ausência de projeto para o hook correspondente;
      (b) **checagem real do global (NÃO existe hoje, é o gap principal deste
      critério)**: `trackfw validate` (rodado em qualquer diretório de projeto, ou um
      modo/flag novo se fizer mais sentido arquiteturalmente — decisão do ML) passa a
      checar que `~/.trackfw/scripts/trackfw-credential-guard.sh` e
      `~/.trackfw/scripts/trackfw-git-branch-guard.sh` existem, são executáveis e têm
      conteúdo íntegro (mesmo binário atual), sempre que o hook de PROJETO delega para o
      global (dedup ativo). Se `trackfw validate` roda num projeto sem nenhum hook de
      projeto NEM instalação global detectável para aquele CLI, não é violação — só é
      violação quando existe uma dependência real (projeto delega pro global, ou global
      está registrado em algum arquivo de config do CLI) e o alvo dessa dependência
      falha.
- [x] Comportamento idêntico nos 3 CLIs (Go/Node/Python) — mensagens byte-idênticas.
- [x] Teste de regressão cobrindo os 2 casos reais desta sessão: (1) hook de
      git-branch-guard registrado + script ausente → violação/aviso; (2) hook + script
      presente mas com conteúdo desatualizado (simular alterando 1 byte) → violação/aviso;
      (3) script presente e íntegro → silêncio, sem falso positivo.
- [x] `make quality` passa sem novas divergências de paridade.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados.md
