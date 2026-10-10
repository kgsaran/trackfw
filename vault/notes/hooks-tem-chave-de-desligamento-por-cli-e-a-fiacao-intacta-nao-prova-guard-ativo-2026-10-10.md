# Hooks têm chave de desligamento por CLI — fiação intacta não prova guard ativo

**Data:** 2026-10-10  
**Causa raiz de:** REQ-2026-09-02 (Wave 0, ML-0A) — ADR-2026-10-10, D7  
**Branch:** `fix/remover-a-entrada-pretooluse-nao-e-detectado`

---

## O achado

Toda verificação de "o guard está instalado" que olhe só as entradas de hook (evento, matcher,
comando) é burlável sem tocar em nenhuma delas. Cada CLI tem uma chave que **desliga todos os
hooks** e deixa a fiação intacta:

| CLI | Onde | Chave | Fonte (citada no parecer) |
|---|---|---|---|
| Claude Code | `.claude/settings.json` e `.claude/settings.local.json` | `disableAllHooks: true` | docs.anthropic.com — settings |
| GitHub Copilot CLI | `.github/hooks/trackfw-attention.json` | `disableAllHooks: true` | docs.github.com — hooks reference |
| Gemini CLI | `.gemini/settings.json` | `hooksConfig.enabled: false` | geminicli.com — hooks reference |
| Kiro | `.kiro/hooks/*.json` | `"enabled": false` por entrada | kiro.dev — hooks |
| Codex CLI | `.codex/config.toml` (outro arquivo) | `[features] hooks = false` | developers.openai.com — codex hooks |
| Windsurf, Amazon Q | — | nenhuma encontrada | — |

`.claude/settings.local.json` é git-ignored (aqui, via `~/.config/git/ignore`): **nenhuma âncora no
git o vê**.

## Por que é armadilha

O `hades-tf` recomendou tratar isso como resíduo, com o argumento de que desligar para depurar é
indistinguível de adulteração. **O mesmo argumento vale para apagar a entrada do hook**, que a regra
já trata como violação. A decisão (ADR-2026-10-10, D7) foi tratar como a mesma causa: o gerador
nunca escreve essas chaves, então elas aparecerem no disco é contradição — ancorada em `origin/main`
quando o arquivo é rastreado, verificada só no disco quando não é.

## Como aplicar

Ao escrever ou revisar qualquer regra de integridade de guard: a pergunta não é "a entrada existe?",
é "o hook **executa**?". Liste as chaves de desligamento do CLI antes de declarar a regra completa.

Parecer completo: `docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md`, seção 2a.
