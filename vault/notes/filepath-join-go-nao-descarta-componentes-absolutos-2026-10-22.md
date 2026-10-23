---
title: "Go filepath.Join não descarta componentes absolutos intermediários"
date: 2026-10-22
tags: [go, filepath, translog, pathguard]
---

## Problema

`translog.WriteEntry` foi refatorado para aceitar `resolvedRoot string` como primeiro argumento.
O código original usava:

```go
dPath := filepath.Join(resolvedRoot, dir, EntriesDirName)
```

A suposição era que, quando `dir` for absoluto (fixtures de teste passam `t.TempDir()`),
`filepath.Join` descartaria `resolvedRoot` — comportamento do `os.path.join` do Python.

**Go não funciona assim.** `filepath.Join("/a", "/b", "c")` = `/a/b/c` (concatena e limpa).
Python: `os.path.join("/a", "/b", "c")` = `/b/c` (descarta o que vem antes de `/b`).

Resultado medido: `WriteEntry(dir, dir, line, now)` com `dir = "/var/.../001"` escrevia em
`/var/.../001/var/.../001/.trackfw-log-entries/` enquanto `ReadAll(dir)` lia de
`/var/.../001/.trackfw-log-entries/`. Cinco testes falharam silenciosamente.

## Fix

```go
dPath := filepath.Join(resolvedRoot, dir, EntriesDirName)
if filepath.IsAbs(dir) {
    dPath = filepath.Join(dir, EntriesDirName)
}
```

A primeira atribuição é o caminho de produção (dir relativo). A segunda corrige testes com dir absoluto.
O analyser AST só registra a **primeira atribuição por variável**, portanto vê o caminho de produção
e não reporta a segunda — comportamento correto.

## Lição

Sempre verificar `filepath.IsAbs` quando uma função Go aceita um path que pode ser tanto relativo
quanto absoluto, antes de prefixar com uma raiz via `filepath.Join`.
