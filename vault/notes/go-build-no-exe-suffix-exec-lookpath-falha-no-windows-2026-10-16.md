# go build -o bin/trackfw no Windows: arquivo sem .exe ≠ exec.LookPath

**Data:** 2026-10-16
**Contexto:** ML-W4 (REQ-2026-09-03) — censo Windows pós-remoção do pip install pypi/

## Achado

`go build -o bin/trackfw ./cmd/trackfw` no Windows produz `bin/trackfw` SEM `.exe`.

O `exec.LookPath("trackfw")` (Go runtime) procura extensões PATHEXT (`.COM`, `.EXE`, …) —
o arquivo nu sem extensão **nunca casa**.

Resultado: mesmo com `export PATH="$ROOT_DIR/bin:$PATH"` no bash, o processo nativo que roda
dentro de `trackfw validate` não encontra o binário — reporta "trackfw binary not found in PATH".

Medido no censo Windows (`windows-census.yml`) após remover o `pip install pypi/`:
- antes: `exec.LookPath` encontrava `trackfw.exe` instalado via pip
- depois: `exec.LookPath` não encontrava `bin/trackfw` (sem sufixo)

## Causa

`-o bin/trackfw` é literal. Go NÃO acrescenta `.exe` automaticamente quando o nome de saída
já está especificado em `-o`. `go env GOEXE` retorna `.exe` mas isso é só uma variável de
ambiente; ela não afeta o comportamento de `-o`.

Contraste: `go build ./cmd/trackfw` SEM `-o` coloca o binário com `.exe` no diretório corrente
porque Go usa o nome do módulo + GOEXE. Com `-o` explícito, o nome é tratado literalmente.

## Correção

```yaml
- run: go build -o "bin/trackfw$(go env GOEXE)" ./cmd/trackfw
```

- No Windows: produz `bin/trackfw.exe` → `exec.LookPath("trackfw")` encontra
- No Linux/Mac: `go env GOEXE` = `""` → produz `bin/trackfw` (igual ao que era antes)

## Verificação

O `check-gates-falsify.sh` já usava `GO_EXE_SUFFIX=$(go env GOEXE 2>/dev/null || true)` e
definia `FALSIFY_GO_BIN="$ROOT_DIR/bin/trackfw$GO_EXE_SUFFIX"`. Então ele JÁ esperava
`bin/trackfw.exe` e invocava o binário por caminho absoluto sem problema. O único lugar que
falhava era `exec.LookPath("trackfw")` dentro do validate chamado por cenários que precisam
de `trackfw` resolvível via PATH.

## Impacto do censo

Census 38941248253 (branch `fix/fechar-os-grupos-de-falha-de-windows-ml-w4-censo`):
- `git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline` → OK ✅
- `roadmap-req-frontmatter-path/go/from-req-baseline` → OK ✅

## Links

- Nota relacionada: [[guard-fallback-old-binary-exit1-fail-open-6-clis-2026-10-06]] (contexto original do LookPath)
- ML-W4, REQ-2026-09-03
