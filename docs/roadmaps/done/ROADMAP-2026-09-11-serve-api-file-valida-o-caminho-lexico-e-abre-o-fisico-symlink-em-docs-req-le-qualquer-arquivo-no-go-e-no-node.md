---
status: done
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo-no-go-e-no-node.md"
squad: ""
---

# Roadmap: serve /api/file valida o caminho lexico e abre o fisico: symlink em docs/req/ le qualquer arquivo no Go e no Node

> Created: 2026-09-11 | Status: done

## Context

H-01: `/api/file` aceita symlinks que apontam para fora das raízes autorizadas em Go e Node.
A verificação de segurança ocorria ANTES da resolução física do symlink: o código validava o
nome léxico (que está dentro da raiz), mas o SO abria o destino físico (que pode estar em
qualquer lugar do filesystem). Python já usava `os.path.realpath` nos dois lados — referência.

M-03 (mesma causa): `serveStatic` do Node também usava `path.resolve()` sem `realpathSync`.

REQ: docs/req/REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo-no-go-e-no-node.md

## Acceptance Criteria

- [x] AC1 — Go e Node canonicalizam raiz E arquivo antes de comparar (dois estágios: léxico + físico)
- [x] AC2 — M-03: serveStatic do Node usa o mesmo tratamento
- [x] AC3 — resposta 403 E ausência do conteúdo no corpo — assertos nos dois
- [x] AC4 — contra-braço: arquivo legítimo dentro da raiz continua 200
- [x] AC5 — teste do vetor nos 3 runtimes, inclusive Python
- [x] AC6 — gate em scripts/ com falsificação dinâmica nos 3 runtimes (Go via overlay, Node via sed+require, Python via importlib)
- [x] AC7 — varredura: lista de sítios fechada e documentada no gate

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model (incorporado ao Wave 1 — achado pré-reproduzido pelo arquiteto)

### ML-0A — Threat model (entregue pelo arquiteto como handoff)
**Status:** ✅ Concluído
**Files affected:** (análise, sem código)
**Actions:**
1. Superfícies com input controlado pelo usuário: `/api/file?path=` (3 runtimes) + `/static/...` (Node e Python)
2. Vetor: symlink cujo nome está dentro da raiz autorizada → destino físico fora
3. Falsificação: revertendo EvalSymlinks/realpathSync → handler retorna 200 com segredo (reproduzido)
4. Residual declarado: autenticação/CORS (M-04, escopo diferente)
**Acceptance criteria:**
- [x] Causa documentada, reprodução gravada, referência Python identificada

**Gates da wave:**
```bash
bash scripts/check-serve-api-file-security.sh
```

## Wave 1 — Implementação (todos os ACs cobertos em um único ciclo)

### ML-1A + ML-1B — AC1 + AC2: contenção física nos 3 runtimes
**Status:** ✅ Concluído
**Files affected:**
- `internal/serve/api_file.go` — Go: adiciona EvalSymlinks na raiz e no arquivo; dois estágios léxico+físico
- `npm/src/serve/api_file.js` — Node: adiciona realpathSync.native na raiz e no arquivo; dois estágios
- `npm/src/commands/serve.js` — Node M-03: REAL_STATIC_DIR pré-computado + contenção física em serveStatic
- `pypi/trackfw/serve/api_file.py` — Python: já defendido (referência); sem alteração funcional
**Actions:**
1. Go: `filepath.EvalSymlinks(workDir)` → realWorkDir; `EvalSymlinks(absPath)` → realAbsPath; falha → 404; verificar contra raízes canônicas
2. Node api_file: `fs.realpathSync.native(resolved)` → realResolved; fallback → 404; verificar contra `realAllowedDirs`
3. Node serveStatic: `REAL_STATIC_DIR = realpathSync.native(STATIC_DIR)` no load; realpathSync do arquivo → 404 ou verificar
4. Ordem correta: léxico → 403 (sem tocar disco) → canonicalizar → 404 (se não existe) → físico → 403
**Acceptance criteria:**
- [x] AC1 — Go e Node canonicalizam ambos os lados
- [x] AC2 — serveStatic Node idem
- [x] build Go: `go build ./...` → ok
- [x] testes Go: `go test ./...` → ok (todos os pacotes)

### ML-1C + ML-1D + ML-1E — AC3 + AC4 + AC5: testes nos 3 runtimes
**Status:** ✅ Concluído
**Files affected:**
- `internal/serve/api_file_test.go` — Go: TestFileHandler_SymlinkEscape (AC3) + TestFileHandler_SymlinkInsideRoot (AC4)
- `npm/tests/serve_api.test.js` — Node: symlink escape 403+sem corpo (AC3) + symlink legítimo 200 (AC4)
- `pypi/tests/test_serve_api.py` — Python: test_symlink_escape_blocked_403_no_body (AC3+AC5) + test_symlink_inside_root_allowed (AC4)
**Actions:**
1. Go: dois testes novos com frases de reconciliação explícitas
2. Node: dois testes novos com realpathSync.native para garantir pwd -P no setup
3. Python: dois testes novos + `handler.wfile.write.assert_not_called()` para AC3
**Acceptance criteria:**
- [x] AC3 — 403 asserted; ausência de corpo asserted nos 3 runtimes
- [x] AC4 — 200 com conteúdo asserted nos 3 runtimes
- [x] AC5 — Python incluso nos testes do vetor
- [x] testes Node: 12 passed, 0 failed
- [x] testes Python: 1722 passed, 66 subtests

**Frases de reconciliação por teste novo:**
- `TestFileHandler_SymlinkEscape`: afirma que EvalSymlinks bloqueia symlink externo com 403 — conclusão direta do H-01
- `TestFileHandler_SymlinkInsideRoot`: afirma que symlink interno legítimo continua retornando 200 — contra-braço de AC1
- Node `symlink para fora da raiz retorna 403 sem vazar conteúdo`: afirma que realpathSync.native bloqueia symlink externo e o corpo não vaza segredo — conclusão do H-01
- Node `symlink legítimo dentro da raiz retorna 200 com conteúdo`: contra-braço — realpathSync não bloqueia symlinks internos legítimos
- Python `test_symlink_escape_blocked_403_no_body`: afirma que _is_safe_path com realpath bloqueia symlink externo e wfile.write não é chamado — prova a defesa existente do Python (AC5: defesa sem teste era a próxima a ser "simplificada")
- Python `test_symlink_inside_root_allowed`: contra-braço — symlink interno retorna 200

### ML-1F — AC6: gate com falsificação + AC2: serveStatic testado
**Status:** ✅ Concluído
**Files affected:**
- `scripts/check-serve-api-file-security.sh` — gate reescrito com falsificação dinâmica nos 3 runtimes; wired ao Makefile (parity-rest)
- `npm/src/commands/serve.js` — serveStatic adicionado a module.exports (permite testes diretos)
- `npm/tests/serve_api.test.js` — 2 testes AC2: attack arm (symlink → 403) + counter-arm (app.js → 200)
- `internal/serve/api_file_test.go` — contains/containsRune substituídos por strings.Contains; import adicionado
- `Makefile` — gate adicionado a parity-rest
**Actions:**
1. AC6 Go dinâmico: `go test -overlay` com cópia vulnerável (filePathAllowed(realAbsPath) desabilitado via `if false &&`); teste FALHA na versão vulnerável e PASSA na correta
2. AC6 Python dinâmico: sed substitui os.path.realpath → os.path.abspath; importlib carrega módulo vulnerável; confirma wfile.write recebe segredo
3. AC6 Node dinâmico: sed remove realpathSync.native (não-op); require do módulo vulnerável; confirma 200+segredo
4. AC2 attack arm: serve_api.test.js — cópia temporária de npm/src com symlink no static dir → 403 sem corpo
5. AC2 counter-arm: serve_api.test.js — serveStatic('/static/app.js') → 200 com conteúdo
6. Gate wired ao parity-rest (Makefile linha após check-serve-browser-security.sh)
**Acceptance criteria:**
- [x] AC6 — falsificação dinâmica em todos os 3 runtimes executada; gate 15 ok, 0 falhou
- [x] AC2 — attack arm E counter-arm executados em serve_api.test.js
- [x] AC7 — lista de sítios fechada: Go (1), Node (2), Python (2); documentada no gate

**Frases de reconciliação por teste novo (AC2):**
- Node `serveStatic — arquivo legítimo em STATIC_DIR retorna 200`: afirma que REAL_STATIC_DIR está correto e serveStatic continua servindo arquivos legítimos — contra-braço de M-03
- Node `serveStatic — symlink para fora do STATIC_DIR retorna 403`: afirma que realpathSync.native em serveStatic bloqueia symlink externo sem vazar conteúdo — conclusão direta do M-03

### ML-1G — AC7: varredura de sítios
**Status:** ✅ Concluído
**Files affected:** (análise, sem código)
**Sítios com input controlado pelo usuário — lista fechada:**
- Go: `internal/serve/api_file.go` (?path=) — **corrigido** — Go usa embed.FS para assets estáticos (sem ReadFile de disco)
- Node: `npm/src/serve/api_file.js` (?path=) — **corrigido**
- Node: `npm/src/commands/serve.js` serveStatic (/static/...) — **corrigido** (M-03)
- Python: `pypi/trackfw/serve/api_file.py` (?path=) — **já defendido** (referência)
- Python: `pypi/trackfw/commands/serve.py` _serve_static_file (/static/...) — **já defendido**

**Comando da varredura:**
```bash
grep -rn "path\.resolve\|realpathSync\|EvalSymlinks\|os\.ReadFile\|readFileSync" \
  internal/serve/ npm/src/serve/ npm/src/commands/serve.js pypi/trackfw/serve/ pypi/trackfw/commands/serve.py \
  | grep -v "_test\.\|#\|^\(Binary\)"
```
**Acceptance criteria:**
- [x] AC7 — varredura executada; lista fechada; comando registrado; Go usa embed.FS verificado

---

## ✅ H-01 fechado — 2026-09-11 · auditado pelo arquiteto contra os 3 binários

Reprodução do arquiteto, **três braços, três runtimes**:

```
            ataque(symlink p/ fora)   vazou?   legitimo   symlink INTERNO legitimo
GO          403                       nao      200        200
NODE        403                       nao      200        200
PY          403                       nao      200        200
```

🔴 **Os três braços importam, e cada um sozinho engana:**

- só o **403** não prova nada — pode vir com o corpo vazando. Por isso `vazou=0` é asserção própria.
- só o **ataque** não prova nada — guarda que recusa tudo também dá 403. Por isso o `legitimo=200`.
- e o **symlink legítimo dentro da raiz** é o que separa *contenção* de *recusa cega*. Sem ele, o fix
  poderia ter quebrado um caso de uso real e passado nos outros dois.

Antes: `GO 200 / NODE 200` com `HADES_SECRET_TOKEN_ABC123` no corpo.

### Três bloqueios que a auditoria do próprio ML levantou antes de me entregar

1. **AC2 sem evidência executada** — o `serveStatic` (M-03) estava corrigido mas nenhum teste o
   exercitava. Passou a ser exportado e ganhou dois testes com setup dinâmico real.
2. 🔴 **O gate não estava ligado a alvo nenhum.** `check-serve-api-file-security.sh` existia e **nada o
   executava** — exatamente a classe que motivou o `check-orphan-gates.sh` em 2026-09-10. Ligado ao
   `parity-rest`.
3. **AC6 sem falsificação em Go e Python** — só o Node era dinâmico. Agora Go usa `go test -overlay`
   com cópia vulnerável, e Python carrega módulo mutilado via `importlib`. **A versão vulnerável
   reprova; a corrigida passa.**

`scripts/check-serve-api-file-security.sh` → **15 cenários, 0 falhas.**
