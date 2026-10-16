# ML-W3a — Causas INFER medidas (localmente) + P20 para confirmação CI

> Data: 2026-10-16 | Branch: `fix/fechar-os-grupos-de-falha-de-windows-ml-w3-infer`
> REQ: REQ-2026-09-03 | Agente: artemis-tf (QA)
> Método: leitura de código + simulação CRLF local (macOS) + P20 adicionada ao `windows-probe.yml`
> **Confirmado no x64 pelo arquiteto** — `windows-probe` run `38959494352` (antes da correção) e
> `38963653933` (depois). Ver "Confirmação no x64" logo abaixo.

## Confirmação no x64 (zeus-tf, 2026-10-16)

Run `38959494352` (P20, antes da correção), linhas brutas:

```
i/lf    w/crlf  attr/                 	internal/integrations/assets/agents/architect.md
i/lf    w/lf    attr/-text            	internal/integrations/testdata/architect.subagent.golden.md
PROBE w3 eol architect.md crlf=61 lf=0
PROBE w3 hex_diverge test=TestRenderWithoutIdentityMatchesFrozenGoldens byte=11 got_len=8074 want_len=8655
PROBE w3 hex_got_repr  b'--\r\n        name: trackfw-architect\r\n     '
PROBE w3 hex_want_repr b'--\n        name: trackfw-architect\n       '
PROBE w3 manifest_key_form=native_json_escaped
```

Os 5 testes: `exit=1`. A medição local abaixo **converge** com o x64 (causa A: assets CRLF no
checkout + `normalizeMarkdown` sem `NormalizeCRLF`; causa B: chave nativa escapada em JSON — a
`INFER` "separador no manifesto" estava errada).

Run `38963653933` (P20, depois do ML-W3b): os 5 testes `exit=0`; `go test ./...` falha só nas 6
`entries` restantes, nenhum nome novo.

---

## Resumo executivo

Cinco entradas marcadas INFER em `.github/windows-known-failures.json` foram investigadas por leitura
de código e simulação local. Foram encontradas **duas causas raiz distintas**:

| Causa | Testes afetados | Produto ou instrumento |
|-------|----------------|------------------------|
| **A** — `go:embed` bake CRLF + `normalizeMarkdown` sem `NormalizeCRLF` | 4 testes (1, 2, 4, 5) | produto (1, 2) + instrumento (4, 5) |
| **B** — `filepath.Join` gera `\` mas JSON serializa como `\\` | 1 teste (3) | instrumento |

---

## Causa A — `go:embed` bake CRLF + ausência de `NormalizeCRLF` em `normalizeMarkdown`

### Mecanismo

1. `core.autocrlf=true` (padrão no runner Windows) → `.md` assets sem regra `eol=` em
   `.gitattributes` chegam na árvore com `\r\n`.
2. `//go:embed assets` em `internal/integrations/catalog.go` bake os bytes como estão na árvore no
   momento da compilação/teste. Os assets `*.md` chegam CRLF ao binário de teste.
3. `normalizeMarkdown()` (`render.go`, linha 433) faz apenas `strings.TrimSpace + "\n"` — **não
   chama `NormalizeCRLF`**. No caminho sem identidade (Rota B default), o retorno é
   `normalizeMarkdown(source)` diretamente — os `\r\n` passam para a saída.
4. Os goldens têm `-text` em `.gitattributes` → chegam sempre LF → `got` ≠ `want`.

```
render.go:429  NormalizeCRLF  → bytes.ReplaceAll(src, "\r\n", "\n")
render.go:433  normalizeMarkdown → TrimSpace(string(source)) + "\n"  ← NÃO chama NormalizeCRLF
render.go:393  return normalizeMarkdown(source), nil                  ← caminho no-identity
```

### Confirmação local (macOS simulando Windows)

Ambiente criado com:

```bash
mkdir -p ~/tmp-crlf-test
git -c core.autocrlf=true checkout-index --prefix="$HOME/tmp-crlf-test/" -a
cp go.mod go.sum ~/tmp-crlf-test/
cd ~/tmp-crlf-test
go test -count=1 -run '^TestRenderWithoutIdentityMatchesFrozenGoldens$' ./internal/integrations/
```

Resultado:
- `architect.md`: crlf=61, lf_only=0 (asset chega CRLF)
- golden `architect.subagent.golden.md`: crlf=0, lf_only=61 (golden chega LF, attr=-text)
- Subtest `subagent/architect` → **FAIL**; subtests `backend`, `frontend`, `opencode` → **PASS**
  (porque architect usa `Representation:"subagent"`, rota sem identidade; os outros passam por
  caminhos que chamam `NormalizeCRLF` indiretamente via `rewriteFrontmatterFields` etc.)

Primeira divergência extraída do subtest:
```
got[9..13]:  2d2d0d0a  (--- CRLF)
want[9..11]: 2d2d0a    (--- LF)
```

### Testes afetados

**Teste 1 — `TestRenderWithoutIdentityMatchesFrozenGoldens` (INFER → PRODUTO)**

- Subteste afetado: `subagent/architect` (Rota B no-identity, `Representation:"subagent"`)
- Causa: asset CRLF → `normalizeMarkdown` sem strip → got=CRLF ≠ want=LF (golden)
- Classificação: **PRODUTO** — a função `normalizeMarkdown` deveria normalizar EOL

**Teste 2 — `TestResolveAgentModelMatchesRender` (INFER → PRODUTO)**

- `extractFrontmatterField` (`models_test.go`, linha 195):
  ```go
  if !strings.HasPrefix(content, "---\n") { return "", false }
  ```
  Falha quando `content` começa com `"---\r\n"` (CRLF). Afeta targets
  `gemini`, `copilot`, `windsurf`, `kiro` — representação `agent-markdown`, rota sem identidade.
- Causa: `Render()` devolve bytes CRLF → `extractFrontmatterField` não acha `"---\n"` →
  `present=false` quando deveria ser `present=true`
- Classificação: **PRODUTO** — `Render` deveria garantir saída LF; `extractFrontmatterField` é
  instrumento mas o defeito primário é no renderizador

**Teste 4 — `TestRenderOpenCodeAgent_CRLFSourceMatchesLF` (MEASURED → reclassificado INSTRUMENTO)**

- Injeta CRLF manual: `strings.ReplaceAll(string(lfSource), "\n", "\r\n")`
- Se `lfSource` já vem CRLF do embed (no Windows), então `crlfSource` = `\r\r\n` (double-CR)
- `NormalizeCRLF` faz apenas uma passagem: `\r\r\n` → `\r\n` (um `\r` residual)
- A hipótese "CRLF normalizer não cobre todos os caminhos" descreve o sintoma mas não a causa real:
  **o instrumento assume que `lfSource` é LF puro, mas no Windows já é CRLF**
- Classificação: **INSTRUMENTO** — o teste precisa ler os bytes brutos do asset e comparar; não pode
  assumir que o embed entregou LF

**Teste 5 — `TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF` (MEASURED → reclassificado INSTRUMENTO)**

- Mesmo mecanismo do Teste 4, mas rota Rota B com identidade (`zeusIdentity()`), representation `subagent`
- `lfSource` do embed já é CRLF → injeção manual produz `\r\r\n` → `NormalizeCRLF` deixa `\r` residual
- Classificação: **INSTRUMENTO** — mesmo diagnóstico do Teste 4

---

## Causa B — `filepath.Join` (`\`) vs. serialização JSON (`\\`)

**Teste 3 — `TestUpdateMigratesKnownCodexAndPreservesUnknown` (INFER → INSTRUMENTO)**

- `update_test.go` (linha ~120):
  ```go
  backendPath := filepath.Join(agentsDir, "trackfw-backend.toml")
  // ...
  if !strings.Contains(string(manifest), backendPath) { ... }
  ```
- No Windows: `filepath.Join` → `...\trackfw-backend.toml` (barra invertida nativa)
- O manifest é serializado como JSON: Go `encoding/json` escapa `\` como `\\`
- `strings.Contains(manifest_json, backendPath_native)` → falha porque `\` ≠ `\\`
- Confirmado por vault note:
  `vault/notes/windows-known-failures-14a-entrada-ausente-das-tabelas-wave0-2026-09-29.md`
  documenta: "causa inferida: strings.Contains(manifest, backendPath) falha porque filepath.Join
  emite `\` no Windows e o manifest armazena `\\`"
- Classificação: **INSTRUMENTO** — o teste deve normalizar ou usar a forma JSON-escaped no assert

---

## Grupamento das 5 entradas

| # | Teste | Causa | Grupo | Produto/Instrumento |
|---|-------|-------|-------|----------------------|
| 1 | TestRenderWithoutIdentityMatchesFrozenGoldens | A | B | PRODUTO |
| 2 | TestResolveAgentModelMatchesRender | A | B | PRODUTO |
| 3 | TestUpdateMigratesKnownCodexAndPreservesUnknown | B | D | INSTRUMENTO |
| 4 | TestRenderOpenCodeAgent_CRLFSourceMatchesLF | A | B | INSTRUMENTO |
| 5 | TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF | A | B | INSTRUMENTO |

Grupo B original (CRLF path): testes 1, 2, 4, 5 — mesma causa raiz, mesmo PR.
Grupo D (separador de path): teste 3 — causa distinta, pode entrar como ML separado no mesmo roadmap.

---

## Arquivo P20 adicionado

`.github/workflows/windows-probe.yml` — passo inserido antes de "Registrar duração":

- `if: always()`, `shell: pwsh`, sai 0 sempre
- Imprime `git config --show-origin --get core.autocrlf`
- `git ls-files --eol` dos 5 arquivos relevantes
- Python conta bytes CRLF/LF por asset e golden → `PROBE w3 eol <arquivo> crlf=<n> lf=<n>`
- Para cada um dos 5 testes: `go test -count=1 -json -run '^<test>$' <pkg>` → Python extrai
  primeira divergência hex → `PROBE w3 hex_diverge ...` / `PROBE w3 manifest_key_form=...`
- Saída grep-friendly com prefixo `PROBE w3`

A confirmação CI vai corroborar (ou refutar) esta medição local. Se os bytes de `architect.md` no
runner mostrarem `crlf > 0`, a Causa A está confirmada para os testes 1, 2, 4, 5.

---

## Pendências para ML-W3b (produto) e ML-W3c (instrumento)

**Produto (causa A):**
- `normalizeMarkdown()` deve chamar `NormalizeCRLF(source)` antes do `TrimSpace`
- Ou o caminho no-identity deve normalizar EOL antes de retornar

**Instrumento (causa A — testes 4 e 5):**
- `TestRenderOpenCodeAgent_CRLFSourceMatchesLF` e `TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF`
  devem ler os bytes brutos do asset e verificar se já são CRLF antes de injetar

**Instrumento (causa B — teste 3):**
- `TestUpdateMigratesKnownCodexAndPreservesUnknown` deve usar `strings.Contains` com path em forma
  JSON-escaped, ou desserializar o manifest antes de verificar
