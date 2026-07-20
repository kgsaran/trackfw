# Roadmap: Infraestrutura i18n — pacote npm trackfw

> Criado em: 2026-06-12 | Status: WIP

## Diagnóstico / Contexto

O pacote npm do trackfw distribui um CLI Node.js que wrappa o binário Go. Os comandos em `npm/src/commands/` contêm strings hardcoded em inglês. Para suporte multi-idioma (PT-BR, ES-ES, EN-US), precisamos de um módulo i18n leve sem dependências externas, baseado em arquivos JSON de tradução.

## Wave 1 — Criar infraestrutura i18n (independente)

### ML-1A — Módulo i18n e arquivos de locale
**Status:** CONCLUIDO
**Arquivos afetados:**
- `npm/src/i18n/index.js` (novo)
- `npm/src/i18n/locales/en-US.json` (novo)
- `npm/src/i18n/locales/pt-BR.json` (novo)
- `npm/src/i18n/locales/es-ES.json` (novo)

**Ações:**
- Criar módulo com detecção de locale via LANG/LC_ALL/LANGUAGE + fallback Intl
- Criar 3 arquivos JSON com todas as strings do CLI

**Critérios de aceite:**
- [x] Módulo carrega sem erros
- [x] `t('validate.ok')` retorna string correta
- [x] Locale detection funciona com LANG=pt_BR.UTF-8

## Wave 2 — Wire i18n nos comandos (dependente da Wave 1)

### ML-2A — validate.js + status.js
**Status:** CONCLUIDO
**Arquivos afetados:** `npm/src/commands/validate.js`, `npm/src/commands/status.js`

### ML-2B — log.js + roadmap.js
**Status:** CONCLUIDO
**Arquivos afetados:** `npm/src/commands/log.js`, `npm/src/commands/roadmap.js`

### ML-2C — plugins.js
**Status:** CONCLUIDO
**Arquivos afetados:** `npm/src/commands/plugins.js`

### ML-2D — init.js + adr.js + req.js
**Status:** CONCLUIDO
**Arquivos afetados:** `npm/src/commands/init.js`, `npm/src/commands/adr.js`, `npm/src/commands/req.js`

## Validação Final
**Status:** PENDENTE
- [ ] `node npm/bin/trackfw --help` sem erros
- [ ] `LANG=pt_BR.UTF-8 node npm/bin/trackfw validate 2>&1` exibe PT-BR
