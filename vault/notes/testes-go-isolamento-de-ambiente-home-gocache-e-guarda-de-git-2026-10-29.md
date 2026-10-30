# Testes Go: isolar o HOME move o GOCACHE, `LookPath` não prova git funcional, e o `go test` colapsa o rc do TestMain

> Data: 2026-10-29 · Domínio: testes Go, ambiente · Origem: REQ-2026-09-12 + #624 (Waves 0 e 3, hades-tf)

## Sintoma
`go test ./internal/commands/` reprovava 15 testes na máquina do mantenedor e passava no CI. Causa:
os testes liam o `~/.claude/settings.json` real (guard global) e a sonda de binário reprovava sem
`trackfw` no PATH. Sem `git` no PATH, 233 `--- FAIL` em 4 pacotes pareciam defeito de produto.

## O que não é óbvio
1. **Trocar o `HOME` no `TestMain` muda o `GOCACHE` dos `go build` filhos.** No macOS o default é
   `$HOME/Library/Caches/go-build`, e `GOCACHE` normalmente não está no ambiente do shell. Com o
   HOME temporário, os 5 testes do `commands` que rodam `go build` compilam a frio (falham com
   `GOPROXY=off`), e o `os.RemoveAll` do HOME falha nos arquivos somente-leitura do module cache.
   Correção: capturar `go env GOCACHE GOMODCACHE GOPATH GOENV` **antes** de trocar o HOME e
   re-injetar depois (`internal/commands/netexec_test.go`).
2. **`exec.LookPath("git")` não executa.** Aceita um `git` que sai 127. Guarda de pré-requisito usa
   `exec.Command("git", "--version").Run()`.
3. **O `go test` devolve rc=1 mesmo quando o `TestMain` faz `os.Exit(2)`**; o binário de teste
   preserva o 2. O sentinela re-executa `os.Args[0]` para ver o código real.
4. **"Zero `--- FAIL`" é satisfeito por `os.Exit(0)`.** Critério de falha rápida precisa de rc≠0 +
   `FAIL <pkg>` + ausência de `PASS`/`ok`.
5. **Sentinela de "não vazou diretório temporário" com `filepath.Glob` em `$TMPDIR` dá falso
   positivo**: o HOME temporário do próprio TestMain existe durante `m.Run()`. Redirecione
   `TMPDIR`/`TMP`/`TEMP` do filho para um `t.TempDir()` e afirme que ficou vazio.
6. **O cache do `go test` não inclui variáveis lidas só por subprocessos**: um wrapper de `git` que
   muda de comportamento por env pode devolver `(cached) ok`. Use `-count=1` ao medir ambiente.

## Moldes no código
- isolamento: `internal/validator/main_test.go` e `internal/commands/netexec_test.go`
- sentinelas: `TestIsolatedHomeCoversBothChannels`, `TestGitGuardFailsFastWhenGitAbsent`
- pareceres: `docs/seguranca/2026-10-29-wave0-isolamento-ambiente-dos-testes.md`,
  `docs/seguranca/2026-10-29-redteam-isolamento-ambiente-dos-testes.md`
