# Parecer de Qualidade — write-scope (ML-4A)

> Agente: hefesto-tf | Data: 2026-10-23 | REQ: REQ-2026-10-23

## Escopo examinado

- `internal/guard/writescope.go` (702 linhas, 11 passos em `RunWriteScope`)
- `internal/commands/barrier.go` (seção `ml_scope`, linhas 603–1103)
- `internal/guard/writescope_test.go` (1276 linhas, 46 funções)
- `internal/commands/guard_write_scope_test.go` (678 linhas, 28 funções)
- `internal/commands/barrier_ml_scope_test.go` (758 linhas, 10 funções)
- `internal/config/config.go` (`ParseWriteScopeConfig`, linhas 280–287)
- `internal/guardforms/guard_generations.go` (`WriteScopeCurrentPSPOSIX`)
- `internal/validator/validator_write_scope_wiring.go`

## Resultado

**Veredito: BLOQUEADO.** 1 achado bloqueante (B1). Corretivo necessário antes de fechar a wave.

---

## B — Bloqueantes

### B1 — `barrier.go:997` — `origin/main` indisponível retorna `blocked`, o contrato exige `skipped`

**Arquivo:** `internal/commands/barrier.go`, linhas 995–1002

**Evidência:**

O doc-comment da própria função `checkMLScope` (linha 933–938) declara:

```
// "skipped" — the check is not applicable: not a git repo, roadmap not in HEAD,
//             pre-cutoff roadmap date, or origin/main not available. The skip
//             reason is recorded in Evidence. A skipped check does NOT block the
//             overall barrier verdict.
```

O `docs/cli-parity.md` (linha 3250) confirma:

```
| `skipped` | Not a git repo; roadmap not in HEAD; date before 2026-07-18; no `origin/main` ref |
```

O código produz:

```go
// Step 5: resolve the merge base with origin/main.
mergeBase, ok := barrierGitMergeBase(topLevel)
if !ok {
    check.Status = "blocked"          // ← contradiz o doc e o contrato
    check.Failures = append(check.Failures,
        "origin/main not available — cannot compute scope diff …")
    return check
}
```

**Impacto:** em um ambiente sem `origin` configurado (clone isolado, CI sem `fetch` prévio,
repositório local sem remote) a barreira bloqueia onde deveria pular. O `skipped` é
explicitamente não-bloqueante pelo contrato; o `blocked` bloqueia o `result` global.

**Cobertura:** nenhum teste em `barrier_ml_scope_test.go` cobre este caminho — todos os testes
chamam `mlScopeGitRun(t, dir, env, "update-ref", "refs/remotes/origin/main", "HEAD")` antes de
invocar o check.

**Correção esperada:** substituir `check.Status = "blocked"` por `check.Status = "skipped"` e
mover a mensagem de `check.Failures` para `check.Evidence` (padrão dos outros ramos de skip),
mais um teste que omite o `update-ref` de `origin/main` e valida `status == "skipped"`.

---

## R — Recomendações

### R1 — `config.go:283` — `ParseWriteScopeConfig` não retorna erro; YAML inválido é engolido silenciosamente

**Arquivo:** `internal/config/config.go`, linhas 280–287

O `parse()` interno chama `yaml.Unmarshal` e retorna na linha 436 sem propagar o erro. A função
`ParseWriteScopeConfig` reusa `parse()` e portanto herda o silêncio.

O único chamador atual (`writescope.go:readWriteScopeFromYAML`) contorna o problema com uma
pré-validação explícita antes de chamar `ParseWriteScopeConfig` (linhas 604–611), o que cria um
segundo `yaml.Unmarshal` no mesmo arquivo (R3). Não há bug ativo, mas a API é armadilha para
futuros chamadores: quem chamar `ParseWriteScopeConfig` diretamente com YAML inválido recebe
`WriteScopeConfig{}` vazio sem saber que o parse falhou.

**Sugestão:** adicionar retorno `error` a `ParseWriteScopeConfig` (e a `parse()` quando invocada
por ela), ou ao menos um `ParseWriteScopeConfigE(content string) (WriteScopeConfig, error)` que
propague o erro, deprecando o silent-variant.

### R2 — `writescope.go:604–612` — duplo `yaml.Unmarshal` em `readWriteScopeFromYAML`

**Arquivo:** `internal/guard/writescope.go`, linhas 598–612

```go
var rawYAML interface{}
if yamlErr := yaml.Unmarshal(data, &rawYAML); yamlErr != nil {   // 1ª parse
    return nil, nil, fmt.Errorf(…)
}
cfg := config.ParseWriteScopeConfig(string(data))  // 2ª parse (dentro de parse())
```

O `trackfw.yaml` é relido e parseado duas vezes a cada chamada de `guard write-scope` quando o
arquivo existe. Latência extra em todo hook de escrita. Consequência directa de R1.

### R3 — `pathMatchesFrozen` não resolve symlinks em entradas de diretório congelado

**Arquivo:** `internal/guard/writescope.go`, linhas 630–652

Para entradas de arquivo (match exato), o código chama `filepath.EvalSymlinks(frozenAbs)` no
caminho congelado antes de comparar. Para entradas de diretório (trailing `/`), usa apenas
`filepath.Clean(frozenAbs)` — sem resolução de symlinks no caminho congelado.

Se uma entrada como `.trackfw/leases/` fosse um symlink apontando para outro diretório, a
comparação com `prefix` falharia para um alvo que chegasse pelo caminho real. Na prática os
caminhos congelados são embutidos no código e não são symlinks, então o risco é baixo.

### R4 — `extractFrontmatterField` duplicada em três pacotes

**Arquivos:**
- `internal/commands/barrier.go:623` (como `barrierFrontmatterField`)
- `internal/validator/validator.go:3187`
- `internal/generators/context.go:278`

O comentário em `barrier.go` justifica a cópia local ("does not import validator"), mas a função
já está duplicada em `generators/` também. Os três algoritmos são idênticos. O pacote
`internal/roadmapdoc` já tem funções de parsing de roadmap e seria o lugar natural para uma cópia
exportada única.

### R5 — `barrierGitStatusFiles` sem aviso de custo em repositórios com muitos arquivos não rastreados

**Arquivo:** `internal/commands/barrier.go`, linhas 706–732

`--untracked-files=all` é exigido pelo contrato (cli-parity.md:3217) e está correto, mas o
comentário da função não menciona que a listagem de todos os untracked em um monorepo com 100k
arquivos é O(N). Não é um bug, mas a ausência de nota na função cria surpresa operacional.

---

## Testes — observações

| Arquivo | Camada | Sobreposição |
|---|---|---|
| `internal/guard/writescope_test.go` | Unitário (chama `RunWriteScope` direto) | Cobre lógica interna: `resolveWriteScopePath`, `pathMatchesFrozen`, `normalizeTargetPath`, seam de panic, D4 completo |
| `internal/commands/guard_write_scope_test.go` | Subprocesso (binário real) | Cobre integração: wiring, payload real do Claude Code, contract smoke |

A sobreposição entre os dois arquivos em D4 (expired, no-lease, allow) é esperada e saudável —
camadas diferentes do mesmo comportamento. Não há teste decorativo identificado.

`barrier_ml_scope_test.go` cobre 8 cenários de comportamento + parsing + match, todos com
repositório git real em `t.TempDir()`. Ausência: cenário onde `refs/remotes/origin/main` não existe
(B1).

---

## Build e testes executados

```
go build ./...                                              rc=0
go test ./internal/guard/ -count=1                         ok (2.9s)
go test ./internal/config/ -count=1                        ok (0.1s)
go test ./internal/commands/ -run 'TestMLScope|TestBarrierMLScope|TestBarrierContract' -count=1  ok (1.5s)
git status --short                                         (clean)
```

---

## Coerência D3 — guardforms / InjectClaudeHooks / validate

| Ponto | Valor |
|---|---|
| `guardforms.WriteScopeCurrentPSPOSIX` | `emulate -R sh …; trackfw guard write-scope; …` |
| Matcher em `InjectClaudeHooks` | `"Write\|Edit\|NotebookEdit"` (constante `writeScopeMatcher`, `agentfiles.go:519`) |
| Matcher em `validateWriteScopeWiring` | `"Write\|Edit\|NotebookEdit"` (constante `writeScopeWiringMatcher`, `validator_write_scope_wiring.go:25`) |
| Comando na validação | `guardforms.WriteScopeCurrentPSPOSIX` (linha 108) |

Os três pontos são consistentes. Nenhuma divergência encontrada.
