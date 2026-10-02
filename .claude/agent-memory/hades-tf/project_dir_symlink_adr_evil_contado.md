---
name: project_dir_symlink_adr_evil_contado
description: Symlink de diretório nomeado ADR-*.md é contado como ADR por WalkDir (isDir=false com ModeSymlink) — pré-existente, não corrigido
metadata:
  type: project
---

`filepath.WalkDir` reporta symlinks de diretório como `isDir=false` com `mode=Lrwxr-xr-x`. Um symlink chamado `ADR-evil.md` apontando para um diretório fora de `adr_dirs` passa o filtro `!d.IsDir() && HasPrefix("ADR-") && HasSuffix(".md")` e é incluído na contagem de ADRs.

Medido em 2026-10-02 com `scratchpad/symlinktest2.go` no macOS.

Consequências:
- Contagem inflada (ADRCount += 1, Governance Score +20)
- Leitura de conteúdo: `readRegularFile` usa `openRegularFileNonblock` que verifica `Mode().IsRegular()` — retorna `errNotRegularFile` para o symlink de diretório. Conteúdo falso não é lido.

**Why:** `walkADRFilePathsForRule` (validator.go:3117) usa `!d.IsDir()` que não distingue symlinks de diretório de arquivos regulares. Correção: adicionar `d.Type().IsRegular()` ao filtro.

**How to apply:** ao auditar o primitivo de enumeração, verificar que há proteção contra symlinks de diretório. Este é comportamento pré-existente antes do ROADMAP-2026-10-02; não é regressão da mudança D1. Fix declarado como ajuste #3 no parecer wave0.

[[project_findadrfile_nao_aplica_criterio_prefixo]]
