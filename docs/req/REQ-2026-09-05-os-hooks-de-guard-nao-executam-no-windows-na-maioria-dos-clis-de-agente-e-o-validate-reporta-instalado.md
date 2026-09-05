---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
---

# REQ: os hooks de guard nao executam no Windows na maioria dos CLIs de agente e o validate reporta instalado

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Governada por
`docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md`.
Medição completa em
`docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`.

**Medido, não inferido** — leitura do código-fonte dos fornecedores e documentação oficial:

| CLI | shell no Windows | `.sh` dispara? |
|---|---|---|
| Gemini CLI | PowerShell, sempre | **não** |
| Codex CLI | PowerShell no caminho comum | **não** |
| GitHub Copilot CLI | — | **não** — populamos `"bash"`, ele lê outro campo |
| Claude Code | Git Bash se instalado | **condicional** |
| Cursor · Kiro | — | **indeterminado** |

🔴 **O `trackfw validate` reporta esses hooks como instalados.** O controle que existe para impedir
`git push` bruto por subagente reporta saúde sobre o que nunca inspecionou — na plataforma inteira.

**Contexto de negócio, do usuário:** *"hoje só atendemos usuários de Linux e macOS. Se algum usuário
corporativo tentar usar o trackfw vai se deparar com essa enxurrada de erros e vai desistir do
framework."* Um produto de governança que não governa no Windows não tem segunda chance com esse
usuário.

## Acceptance Criteria

- [ ] **AC1** — **Copilot:** o hook passa a ser escrito no campo que o CLI lê no Windows.
      Falsificação: com o campo antigo, o hook **não** é lido; com o novo, é.
      🔴 **Verificável sem Windows** — é o item de maior retorno por unidade de risco da REQ inteira.
- [ ] **AC2** — **Gemini e Codex:** hook que **executa** sob PowerShell.
- [ ] **AC3** — 🔴 **A prova é o guard DISPARANDO e BLOQUEANDO**, não o arquivo existindo.
      Verificar que o arquivo foi escrito é exatamente o erro que originou esta REQ.
- [ ] **AC4** — 🔴 **Paridade comportamental `.sh` ↔ nativo, por gate.** O `git-branch-guard` tem 561
      linhas e decide se um `push` bruto passa; duas implementações divergem onde ninguém testa. Sem
      contrato verificado, a variante nativa é **dívida de segurança**, não correção.
- [ ] **AC5** — 🔴 **Controle POSIX:** Linux e macOS inalterados, medidos antes e depois.
- [ ] **AC6** — 🔴 **Cursor e Kiro: NENHUMA emissão nova antes de medir.** Emitir para CLI cujo
      mecanismo não conhecemos é inventar mecanismo. O entregável para eles é o **experimento**
      especificado, não código.
- [ ] **AC7** — 🔴 **O `validate` para de reportar como instalado o hook que não pode executar**
      naquele CLI/plataforma. Corrigir a emissão sem corrigir o relato deixaria o silêncio falso de
      pé — e o relato falso é o defeito, não o sintoma.
- [ ] **AC8** — Regra dura de paridade: a mudança de emissão vale nos **3 CLIs** do trackfw
      (Go, Node, Python).

## Negative Scope

- ❌ **Não** exigir Git Bash como remédio — D1 da ADR: resolve 1 CLI de 6 e transfere custo sem
  entregar controle.
- ❌ **Não** portar `attention-signal`/`attention-cleanup` antes dos guards. São conveniência; se o
  esforço estourar, o corte cai neles (D4).
- ❌ **Não** emitir nada para Cursor/Kiro antes da medição (AC6).
- ❌ **Não** reescrever a lógica dos guards ao portar. Portar comportamento **igual** é o que torna a
  paridade verificável; melhorar junto tornaria impossível atribuir divergência.
- ❌ **Não** tratar aqui a jornada de instalação em Windows (`install.sh`, README, ARM64) — REQ
  própria já aberta.

## Linked ADR
ADR: docs/adr/ADR-2026-09-05-hook-de-windows-roda-no-windows-geracao-nativa-por-cli-de-agente-em-vez-de-exigir-git-bash.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
