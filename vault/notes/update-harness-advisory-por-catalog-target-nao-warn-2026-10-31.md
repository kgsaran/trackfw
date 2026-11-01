# update harness — advisory de agent_models emitido por catalog target; token distinto de `agents models`

**Data:** 2026-10-31  
**REQ/Roadmap:** `REQ-2026-08-30-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md` / ROADMAP-2026-09-22

---

## Fato

`trackfw update harness` emite uma linha de advisory no stderr **por catalog target** (aqueles com
sufixo `-agents` ou `-skills` no id), não uma linha por run. Com 20 targets agents/skills, o
resultado é 20 linhas idênticas.

**Função responsável:** `harnessCatalogTarget` em `internal/generators/update.go:2669`

```go
harnessAgentModels, harnessWarnMsg := config.ResolveAgentModels("global", home, harnessCwd)
if harnessWarnMsg != "" {
    fmt.Fprintln(os.Stderr, harnessWarnMsg)
}
```

`ResolveAgentModels` é chamada dentro de `harnessCatalogTarget`, que é invocada uma vez por target
no loop de `UpdateHarness` (linha ~707). Logo, N targets → N chamadas → N linhas de advisory.

---

## Texto do advisory (Aviso 2)

O advisory **não** contém o token `WARN: agent_models.<tier>`. Ele é uma das três mensagens de
configuração global definidas em `internal/config/config.go`:

| Condição | Texto da linha |
|---|---|
| Sem `~/.trackfw/trackfw.yaml` | `GlobalAgentModelsNoneMessage` |
| Arquivo existe, sem `agent_models` | `GlobalAgentModelsNoneMessage` |
| `agent_models` só no projeto | `GlobalAgentModelsProjectOnlyMessage` |
| YAML global malformado | `MalformedGlobalConfigMessage` |
| `agent_models` global válido | — (0 linhas) |
| `agent_models` com valores suspeitos | — (0 linhas; aviso fica em `trackfw agents models`) |

---

## Distinção crítica: Aviso 2 vs Aviso 1

**Aviso 1** — `WARN: agent_models.<tier> = "..." — not a version string...`
- Fonte: `internal/commands/agents_models.go:122`
- Disparado por: `integrations.LooksLikeSuspectModelValue(value)` retornar `true`
- Aparece apenas em: `trackfw agents models`
- **NUNCA aparece em `trackfw update harness`**

**Aviso 2** — mensagens de configuração global (veja tabela acima)
- Fonte: `internal/generators/update.go:2671`
- Aparece apenas em: `trackfw update harness` (e apenas quando há catalog targets)
- **NUNCA contém o token `WARN:`**

## Armadilha do implementador

A REQ (AC3) descreve "20 linhas de `WARN: agent_models.<tier>`" — formato do Aviso 1. Um
implementador que busca `WARN: agent_models` em `agents_models.go` não encontrará nada relevante para
o problema das 20 linhas. A função a corrigir é `harnessCatalogTarget` em `update.go`.

---

## Fix

Mover `config.ResolveAgentModels` para fora de `harnessCatalogTarget`. O resultado
(`harnessAgentModels, harnessWarnMsg`) deve ser computado **uma vez** em `UpdateHarness`, antes do
loop de targets, e passado como parâmetros. A emissão do warning ocorre uma vez, antes do loop, se
e somente se pelo menos um catalog target está nos targets selecionados. Isso evita também que o
advisory seja emitido em runs com `--targets credential-guard-script` (sem catalog targets).

---

## Medição (binário 11.2.3, env -i, cwd fora do projeto)

```
# Cenário A — sem ~/.trackfw/trackfw.yaml
wc -l out.txt err.txt
# out.txt: 39 linhas  ← relatório texto (stdout, não stderr)
# err.txt: 20 linhas  ← advisory × 20 targets
```

O relatório texto do `update harness` vai para **stdout** (`cmd.OutOrStdout()`), confirmado por
medição. `cli-parity.md:804` descreve os advisories como "may still appear on stderr", não o
relatório.
