---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: Débitos de higiene — `package-lock` desatualizado, árvore de build poluindo `grep`, e validate script sem bit de execução

> Date: 2026-08-30 | Status: Open

## Motivation

Quatro resíduos acumulados em ciclos anteriores, agrupados porque compartilham a característica de
**não quebrarem nada até quebrarem uma investigação**.

**1. `npm/package-lock.json` travado em `6.1.0`** enquanto `package.json` está em `7.3.0` — doze
versões de defasagem. Não afeta o runtime (o CLI Node não tem dependências de produção), mas o
`package-smoke` do CI instala a partir dele.

**2. `pypi/build/lib/trackfw/` é uma cópia velha do CLI Python na árvore.** Não é rastreada pelo git,
mas **polui todo `grep -r`** — foi ela que produziu a contagem 19-versus-18 na auditoria do gate da
Wave 0 do pin de CI, e custou tempo até eu isolar. Um `.gitignore` não resolve: o `grep` não o
consulta.

**3. `internal/commands/discover.go` (`InstallGates`) escreve `scripts/trackfw-validate.sh` com
`0755` mas sem `os.Chmod`.** `os.WriteFile` só aplica `perm` no `O_CREATE` — se o arquivo já existe
com outro modo, o bit não é corrigido. É exatamente o defeito que a REQ do exec-bit corrigiu nos
outros cinco pontos de escrita; este ficou de fora porque o remédio do `doctor` aponta
`trackfw update`, não `discover --init`.

**4. `vunknown` na mensagem do `doctor` do CLI Python** quando rodado via `PYTHONPATH=pypi` —
`importlib.metadata` não acha o pacote instalado e cai no fallback. Cosmético, mas aparece em toda
saída de quem roda do fonte.

## Acceptance Criteria

- [ ] **AC1** — `npm/package-lock.json` sincronizado com a versão corrente, e o protocolo de release
      passa a **verificá-lo**: hoje as 5 ocorrências verificadas não o incluem.
- [ ] **AC2** — `pypi/build/` deixa de existir na árvore de trabalho após `make clean`, e o alvo de
      build não a recria em lugar que atrapalhe varredura. Se for inerente ao `setuptools`,
      **documentar** e ensinar a excluí-la nas buscas.
- [ ] **AC3** — `InstallGates` aplica `os.Chmod` após escrever, como os outros cinco pontos.
      Verificável: arquivo pré-existente com `0644` fica `0755` depois de `discover --init`.
- [ ] **AC4** — `doctor` do Python reporta a versão real quando rodado do fonte, ou uma mensagem que
      não pareça defeito.
- [ ] **AC5** — Paridade nos 3 onde aplicável; gate para AC3, que é o único com risco de regressão.
- [ ] **AC6** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** trocar o sistema de build do Python nem o gerenciador do Node.
- **Não** alterar o protocolo de release além de acrescentar o `package-lock` à verificação.

## Linked ADR
<!-- none -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
