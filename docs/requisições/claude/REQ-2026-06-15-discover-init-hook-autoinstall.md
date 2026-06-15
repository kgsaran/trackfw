---
name: REQ-2026-06-15-discover-init-hook-autoinstall
title: "feat: discover --init instala hook framework automaticamente quando nenhum é detectado"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/discover-init-hook-autoinstall-2026-06-15.md
created: 2026-06-15
author: zeus
---

# REQ — discover --init: auto-instalação de hook framework

## Contexto

Ao rodar `trackfw discover --init` em um projeto sem hook framework configurado,
o comportamento atual é exibir um aviso e pular a instalação do hook:

```
⚠ No hook framework detected — skipping hook installation
✓ governance gates installed
```

Isso cria um gap funcional crítico: o `trackfw.yaml` é gerado com `governance_mode: lenient`
(ou `strict` após configuração manual), mas o gate nunca dispara no commit porque
não há hook ativo. O principal ganho do framework — bloquear commits que violam
a cadeia ADR → REQ → ROADMAP — fica inoperante.

## Problema

Sem hook, `governance_mode: strict` é letra morta. O usuário que roda `--init`
espera ter governança ativa ao final do processo.

## Solução

Quando nenhum hook framework for detectado durante `--init`:

1. **Detectar se o projeto tem `package.json`** na raiz:
   - **Sim** → instalar e configurar **Husky** (ecossistema Node já presente)
   - **Não** → instalar e configurar **Lefthook** (binário estático, zero dependências)

2. **Husky** (quando `package.json` presente):
   - Executar `npm install --save-dev husky`
   - Executar `npx husky init`
   - Criar `.husky/pre-commit` com entrada `scripts/trackfw-validate.sh`

3. **Lefthook** (caso padrão):
   - Baixar binário via script oficial ou orientar instalação (`brew install lefthook` / `go install`)
   - Criar `lefthook.yml` com entrada do `trackfw-validate`
   - Executar `lefthook install`

## Paridade 3 CLIs

A lógica de detecção e instalação deve ser implementada nos três CLIs:

| CLI | Localização |
|-----|------------|
| Go | `internal/discover/discover.go` + `internal/commands/discover.go` |
| Node.js | `npm/src/commands/discover.js` |
| Python | `pypi/trackfw/commands/discover.py` |

## Critérios de Aceite

- [ ] `trackfw discover --init` em projeto sem hook e sem `package.json` → instala lefthook e cria `lefthook.yml`
- [ ] `trackfw discover --init` em projeto com `package.json` → instala husky e cria `.husky/pre-commit`
- [ ] Projetos que já têm framework detectado → comportamento atual mantido (idempotente)
- [ ] Paridade nos 3 CLIs
- [ ] `make test` verde
