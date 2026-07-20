---
status: done
date: 2026-07-19
req: "docs/requisições/afrodite/done/REQ-2026-07-19-global-adrs-governance-ML-1B.md"
---

# Roadmap: ML-1B — Node.js: Expansão de `~` em `adr_dirs`

## Status: ✅ Concluído
**Agente:** Afrodite (Frontend/Node Specialist)

## ML-1B Node.js `adr_dirs` tilde expansion
1. Criar helper `expandPath(filePath)` em Node.js usando `os.homedir()` e `path.resolve`/`path.join`. ✅
2. Aplicar expansão na leitura e validação de `adr_dirs`. ✅
3. Escrever testes em `npm/tests/config.test.js` e `npm/tests/validator.test.js`. ✅
4. Executar testes e garantir suite verde. ✅
