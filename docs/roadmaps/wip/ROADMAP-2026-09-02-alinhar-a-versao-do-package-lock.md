---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-package-lock-do-npm-parado-em-6-1-0.md"
---

# ROADMAP: alinhar a versão do package-lock

> Date: 2026-09-02 | Status: wip

REQ: docs/req/REQ-2026-09-02-package-lock-do-npm-parado-em-6-1-0.md
ADR:

## ML-1A — Corrigir os dois sítios de versão

**Status:** ✅ Concluído

`npm/package-lock.json`: `"version": "6.1.0"` → `"7.3.0"` na raiz e em `packages[""]`. Duas linhas.
O lock **não** é regenerado — as dependências resolvidas ficam byte a byte como estavam, para o diff
ser auditável.

## ML-1B — Medir antes de afirmar

**Status:** ✅ Concluído

Eu ia reportar "`npm ci` quebra". Medi antes, e **não quebra**:

```
$ npm ci    # com o lock em 6.1.0
added 42 packages, and audited 43 packages in 18s
```

`npm` só reclama quando as dependências divergem, e as três são idênticas entre os dois arquivos.
Procurei também quem lê a versão do lock — nenhum gate, nenhum workflow, nenhum código. **Não há
quebra medida.**

Registrado aqui porque a consequência falsa quase entrou no relatório. O achado que sobra é outro e
é honesto: o passo "bump version files" do protocolo de release pula este arquivo em silêncio, e vem
pulando desde a 6.1.0 — porque é a única declaração de versão do projeto que nenhum gate confere.

## ML-1C — Não-regressão

**Status:** ✅ Concluído

`npm ci` com o lock corrigido: `added 42 packages, and audited 43 packages`. Mesma árvore, mesma
contagem. A correção não mexe em dependência.
