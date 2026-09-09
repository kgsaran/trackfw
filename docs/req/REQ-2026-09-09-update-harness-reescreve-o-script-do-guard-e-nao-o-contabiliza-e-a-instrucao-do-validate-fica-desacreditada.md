---
status: Open
date: 2026-09-09
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-09-o-alvo-do-manifesto-e-dono-do-caminho-do-script-do-guard.md"
---

# REQ: update harness reescreve o script do guard e nao o contabiliza, e a instrucao do validate fica desacreditada

> Date: 2026-09-09 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Origem

Issue [#300](https://github.com/kgsaran/trackfw/issues/300). **Reproduzido em três máquinas
diferentes** — o autor no Windows, o arquiteto no macOS, e o KG ao vivo na sessão.

## Motivation

`trackfw update harness` **reescreve** `~/.trackfw/scripts/trackfw-git-branch-guard.sh` e reporta
`updated=0`.

🔴 **Isto desacredita a instrução da própria release.** O CHANGELOG da **7.5.0**, publicado hoje, diz
em destaque: *"Quem já usa precisa rodar `trackfw update harness` depois de atualizar"* — e o comando
responde que não fez nada. E a regra do `validate` manda rodar exatamente esse comando.

### Três estados, relatórios indistinguíveis — medido

| estado | relatório | efeito real no script |
|---|---|---|
| desatualizado (macOS, arquiteto) | `updated=9 skipped=18 missing=6` | **conteúdo novo**: 561 → 580 linhas, sha `f2e80b0f` → `4507187c` |
| já em dia (macOS, KG, ao vivo) | `updated=0 skipped=27 missing=6` | **reescrito**: mesmo sha, **mtime +160s** |
| `--dry-run` (Windows, autor) | `updated=0 skipped=33` | nada |

🔴 **O número não correlaciona com o script.** O `updated=9` da máquina do arquiteto veio de arquivos
de **fiação** — o script mudou nos dois primeiros casos e não foi contado em nenhum.

### A causa, confirmada

```
alvos cujo path é o script do guard:  ZERO

claude-git-branch-guard   →  ~/.claude/settings.json
codex-git-branch-guard    →  ~/.codex/hooks.json
gemini-git-branch-guard   →  ~/.gemini/settings.json
cursor-git-branch-guard   →  ~/.cursor/hooks.json
copilot-git-branch-guard  →  ~/.copilot/settings.json
kiro-git-branch-guard     →  ~/.kiro/hooks/trackfw-git-branch-guard.json
```

**Nenhum alvo é dono de `~/.trackfw/scripts/trackfw-git-branch-guard.sh`.** O script é escrito como
**efeito colateral** do alvo de fiação, e a contagem só enxerga o arquivo de fiação — que, esse sim,
não mudou, e por isso saiu `skipped`.

**Não é contador que erra às vezes: é caminho escrito e nunca contabilizado.**

## Acceptance Criteria

- [ ] **AC1** — o script do guard é **alvo de primeira classe**, com `path` próprio no manifesto
- [ ] **AC2** — os três estados passam a ser **distinguíveis** pelo relatório: conteúdo novo ⇒
      `updated`; conteúdo idêntico ⇒ `skipped` **e sem reescrita**; `--dry-run` ⇒ nada escrito
- [ ] **AC3** — 🔴 **parar de reescrever quando o conteúdo é idêntico.** Reescrever com mesmo conteúdo
      muda `mtime` sem motivo e é o que torna o estado 2 indistinguível do 1
- [ ] **AC4** — 🔴 **controle:** o script **continua sendo escrito** quando precisa. Um alvo que conta
      certo e para de escrever seria pior que o defeito atual
- [ ] **AC5** — o mesmo vale para o `trackfw-credential-guard.sh`, se ele tiver o mesmo padrão —
      **medir**, não presumir
- [ ] **AC6** — paridade nos 3 CLIs

## Negative Scope

- **Não** altera o conteúdo gerado do guard — é contabilidade e idempotência, não geração.
- **Não** mexe nas regras de integridade do `validate` — lacuna **inversa**, coberta pela
  `REQ-2026-09-02-remover-a-entrada-pretooluse-...` (as regras cobrem script e modo, **não** a fiação).
- **Não** trata a resolução de escopo do `update harness`, que tem REQ própria
  (`REQ-2026-08-21-update-harness-le-trackfw-yaml-do-cwd-...`).
