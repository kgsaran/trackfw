# integrations.Manager.resolve() — destRoot deve ser atualizado condicionalmente ao prefixo do destino

**Data:** 2026-11-17
**REQ:** REQ-2026-11-15
**Branch:** feat/cobertura-e-compatibilidade-completas-do-trackfw-no-windows

---

## Causa raiz

Ao adicionar um novo `XxxDir` hook em `integrations.Manager` (`internal/integrations/manager.go`),
a variável `destRoot` usada em `beneath(destRoot, destination)` **deve ser atualizada
condicionalmente** — somente quando o plano de destino foi realmente re-roteado para o novo dir.

A armadilha: adicionar um novo hook `CodexDir` e computar `codexDestRoot`, mas esquecer de atualizar
`destRoot` para ele, resulta em `beneath(homeDir, $CODEX_HOME/agents/...)` → falha no check de
contenção ("destination is outside global root") — mesmo que o destino tenha sido re-roteado
corretamente.

## A correção errada (e por que parece óbvia)

```go
// ERRADO — muito amplo
if codexDirSet {
    destRoot = codexDestRoot   // quebra codex-skills (~/.agents/skills/)
}
```

Quando `codexDirSet = true`, o catálogo codex contém dois tipos de destino:
- `~/.codex/agents/...` → deve usar `codexDestRoot = $CODEX_HOME`
- `~/.agents/skills/...` → NÃO está sob `~/.codex/`; deve continuar com `destRoot = homeDir`

Atualizar `destRoot` incondicionalmente quebra o segundo grupo porque `$CODEX_HOME/skills/`
não existe e `$HOME/.agents/skills/` fica fora do `codexDestRoot`.

## A correção correta

```go
if codexDirSet && strings.HasPrefix(plan.Destination, "~/.codex/") {
    destRoot = codexDestRoot
}
```

Atualizar `destRoot` apenas quando o destino do plano começa com `~/.codex/` — ou seja,
somente para destinos que foram de fato re-roteados para o `codexDestRoot`.

## Padrão geral

**Regra:** ao adicionar `XxxDir` em `resolve()`, actualize `destRoot` SOMENTE para destinos
com o prefixo correspondente — não para todos os destinos do catálogo desse CLI.

```
Novo hook XxxDir → codexDestRoot = filepath.Abs(xxxDir)
Para cada plan.Destination:
  if strings.HasPrefix(plan.Destination, "~/.xxx/") {
      destRoot = xxxDestRoot
  }
```

Isso preserva o comportamento de outros catálogos do mesmo CLI que apontam para caminhos
fora do novo dir (ex.: `~/.agents/skills/` para Codex).

## Arquivos envolvidos

- `/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/internal/integrations/manager.go`
  — função `resolve()`, bloco de substituição de destino Codex (procure `codexDestRoot`)
- Testes que cobrem o comportamento: `TestHarnessCatalogTarget_CodexAgents_CODEX_HOME_WritesUnderCodexHome`
  e `TestUpdateHarnessCmd_RunsOutsideProjectWithoutTrackfwYAML` (ML-4A4, R6)

## Sinal de que a correção está incorreta

Se `TestUpdateHarnessCmd_RunsOutsideProjectWithoutTrackfwYAML` ou
`TestUpdateHarnessCmd_EmptyHarnessExitsZero` falharem com
`"destination is outside global root"` ou `"1 target(s) failed"`, o `destRoot` está sendo
atualizado para todos os destinos Codex em vez de apenas para os com prefixo `~/.codex/`.
