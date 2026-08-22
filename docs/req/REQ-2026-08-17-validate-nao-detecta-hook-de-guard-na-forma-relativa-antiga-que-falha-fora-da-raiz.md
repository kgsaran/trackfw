---
status: Open
date: 2026-08-17
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-21-validate-detecta-hook-de-guard-na-forma-relativa-antiga.md"
---

# REQ: `validate` não detecta hook de guard na forma relativa antiga, que falha fora da raiz

> Date: 2026-08-17 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

Bug reportado por KG em uso real no projeto CMDB (2026-08-17). Toda chamada de `Bash` cuspia:

```
PreToolUse:Bash hook error
Failed with non-blocking status code: /bin/sh: scripts/trackfw-credential-guard.sh: No such file or directory
PostToolUse:Bash hook error
Failed with non-blocking status code: /bin/sh: scripts/trackfw-credential-guard.sh: No such file or directory
```

**O script existia.** O `.claude/settings.json` do CMDB carregava os hooks do trackfw na forma
**relativa** (`scripts/trackfw-credential-guard.sh`), enquanto os hooks do próprio projeto, no mesmo
arquivo, usavam `$CLAUDE_PROJECT_DIR/...`. O comando que disparou tinha feito `cd` para
`react-cmdb/packages/shared-i18n/src/locales`, e daí o relativo não resolve.

Artefato velho, não defeito do gerador: o `ADR-2026-08-11` decidiu usar `$CLAUDE_PROJECT_DIR` e o
gerador já emite assim desde #156 (2026-08-12), com função de migração. O CMDB não rodava
`trackfw update` desde antes disso — a prova é que **não havia nenhum hook de `git-branch-guard`**,
que entrou em #169 (2026-08-14). `trackfw update` corrigiu.

### O defeito real: a regra existe e não pegou

Existem `credential_guard_hook_resolvable` e `git_branch_guard_hook_resolvable`, feitas exatamente
para "o hook aponta para script que não resolve". **Nenhuma disparou.**

`validateGuardHookResolvable` (`internal/validator/validator_credential_guard.go:130`) faz
`root, _ := os.Getwd()` e resolve o caminho do hook **relativo à raiz do projeto**. Da raiz,
`scripts/trackfw-credential-guard.sh` **resolve** — o arquivo está lá. A regra conclui "ok".

Mas o hook não roda sempre a partir da raiz. **O ponto cego é que resolvibilidade foi modelada como
propriedade do caminho, quando é propriedade do par (caminho, cwd).**

O `validate` avisou sobre divergência de **conteúdo** dos scripts, e nada sobre o **comando** do hook
estar na forma antiga — que era o que estava quebrado.

### Por que é sério

O guard de credenciais **não estava executando** no CMDB. A cada `Bash`, o hook falhava e o controle
não rodava. Uma regra de segurança dando verde enquanto o controle que ela guarda está inerte é pior
que não ter a regra: cria confiança falsa. É a mesma classe do que a barreira do `git-branch-guard`
bloqueou em 2026-08-16 — controle que parece ativo e não está.

Agravante: o hook falha com **status não-bloqueante**, então o trabalho segue normalmente. O único
sinal é ruído no terminal, fácil de ignorar por sessões inteiras.

## Escopo

Detectar que um hook de guard registrado está na **forma que não resolve fora da raiz**, e não
apenas que o arquivo apontado existe.

Sinais disponíveis, a avaliar por quem executar:
1. Comando do hook é **relativo** onde o CLI daquele fornecedor exige mecanismo próprio — a tabela
   do `ADR-2026-08-11` já diz, por CLI, qual é a forma correta. Para Cursor e Copilot o relativo
   **é** correto e não pode ser acusado.
2. Comando **diverge do que o gerador atual emitiria** para aquele CLI — o mesmo tipo de comparação
   que `credential_guard_script_integrity` já faz para o conteúdo do script, aplicada ao comando.

A opção 2 parece mais forte: não exige enumerar formas erradas e acompanha o gerador de graça.

### Escopo negativo — declarado

- **Não é mudar a resolução de caminho.** O `ADR-2026-08-11` já decidiu, por CLI, e o gerador já
  implementa. Isto é detecção do artefato desatualizado.
- **Não é migrar artefato automaticamente no `validate`.** `validate` diagnostica; `update` corrige.
- **Não acusar Cursor nem Copilot**, cujo relativo é correto por decisão registrada.

## Acceptance Criteria

- [x] AC1 — Repro fiel: fixture com hook de guard na forma relativa antiga em CLI cuja decisão é
      `$VAR/...`, com o script **presente** na raiz. Hoje o `validate` passa; deve acusar.
- [x] AC2 — Não-regressão: hook na forma correta **não** é acusado, em nenhum dos 6 CLIs.
- [x] AC3 — **Cursor e Copilot com caminho relativo continuam limpos** — falso-positivo aqui é o
      risco dominante, porque o relativo é a forma certa neles.
- [x] AC4 — Mensagem nomeia o remédio (`trackfw update`), como o ML-2C fez para o `update`.
- [x] AC5 — Paridade nos 3 CLIs, com gate comparando as saídas reais — **não** por leitura de fonte.
- [x] AC6 — Cenário de falsificação (P4), baseline + detecção, provando que a regra pega a forma
      antiga e que a prova não é vacuosa.
- [x] AC7 — `make quality` verde; `trackfw validate` sem novas violações. 160 cenários, CI-exata exit 0.

## Riscos para quem executar

- **Falso-positivo é o risco dominante**, por causa do AC3: acusar Cursor/Copilot indevidamente
  treina o usuário a ignorar a regra, que é exatamente a falha que esta REQ combate.
- **Não testar por leitura de fonte.** O critério é o `validate` acusando um fixture de fato
  desatualizado.
- **Cuidado com o binário do `PATH`:** medido em 2026-08-17, pode estar velho e emitir avisos falsos,
  e `--version` **não** distingue o build. Compilar antes de auditar.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-11-resolucao-de-caminho-dos-hooks-de-projeto-por-cli-mecanismo-especifico-do-fornecedor-sem-caminho-absoluto.md` (governa a forma correta por CLI; esta REQ detecta o desvio)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
