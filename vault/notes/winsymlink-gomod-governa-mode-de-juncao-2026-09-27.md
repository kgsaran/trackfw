# `winsymlink` GODEBUG: o `go.mod` governa como junção (`mklink /J`) reporta o mode no Go

> 2026-09-27 | REQ-2026-09-27 | issue #444 | hades-tf

## O Problema

`os.Lstat` em Go reporta o `FileMode` de juncões Windows (`IO_REPARSE_TAG_MOUNT_POINT`) de forma
**diferente dependendo do `go` directive no `go.mod`**, não da versão do Go instalado nem da
arquitetura (ARM64 vs x64):

| `go` no `go.mod` | winsymlink padrão | junção (`mklink /J`) |
|---|---|---|
| `< 1.23` | `0` (`modePreGo1_23`) | `ModeSymlink=true, ModeIrregular=false` |
| `>= 1.23` | `1` (`mode()`) | `ModeSymlink=false, ModeIrregular=true` |

trackfw usa `go 1.25.2` → `winsymlink=1` → junção = `ModeIrregular`.

## Fonte Primária

`$(go env GOROOT)/src/os/types_windows.go`, Go 1.27.0:

```go
// mode() — winsymlink=1
switch fs.ReparseTag {
case IO_REPARSE_TAG_SYMLINK:  m |= ModeSymlink
case IO_REPARSE_TAG_AF_UNIX:  m |= ModeSocket
case IO_REPARSE_TAG_DEDUP:    // tratado como regular (intencional)
default:                      m |= ModeIrregular  // ← MOUNT_POINT cai aqui
}

// modePreGo1_23() — winsymlink=0
if tag == SYMLINK || tag == MOUNT_POINT {
    return m | ModeSymlink  // ← junção = symlink (comportamento antigo)
}
```

## Armadilha

Um agente que crie uma sonda standalone com `go 1.21` no go.mod (go.mod mínimo usual) vai medir
`ModeSymlink=true` para junção e concluir que o guard já captura juncões — o oposto do verdadeiro.

A/B medido na VM (go1.27.0 windows/arm64):
- go.mod `go 1.21`: junção mode = `0x80001b6` (`Lrw-rw-rw-`, ModeSymlink)
- go.mod `go 1.25`: junção mode = `0x801b6` (`?rw-rw-rw-`, ModeIrregular)

**Sempre meça dentro do pacote do produto** (via `go test` dentro do módulo), nunca em sonda
standalone com go.mod separado.

## Consequência para o Predicado

O guard atual testa só `ModeSymlink`. No contexto do trackfw (go 1.25.2):
- `mklink /J` → `ModeIrregular` → guard PASSA → defeito ativo
- `mklink /D` → `ModeSymlink` → guard RECUSA → já funciona

Correção correta: testar `ModeSymlink | ModeIrregular`. Correto sob ambas as configurações de
`winsymlink` — nenhum bump de go.mod reabre o buraco.

## Tags e Modos (winsymlink=1, medido)

| Reparse Tag | Hex | Mode Go |
|---|---|---|
| `IO_REPARSE_TAG_SYMLINK` | `0xa000000c` | `ModeSymlink` |
| `IO_REPARSE_TAG_MOUNT_POINT` | `0xa0000003` | `ModeIrregular` |
| `IO_REPARSE_TAG_DEDUP` | `0x80000013` | (regular) |
| `IO_REPARSE_TAG_CLOUD_FILES` | `0x9000001a` | `ModeIrregular` (fonte, não medido) |
| outros | variado | `ModeIrregular` (default) |
