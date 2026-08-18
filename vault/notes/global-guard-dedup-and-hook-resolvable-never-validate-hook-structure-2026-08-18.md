# O dedup e o `hook_resolvable` do guard global comparam string, nunca a forma estrutural do hook

> Data: 2026-08-18 | Autor: `hades-tf` (ML-4A, barreira de
> `ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`)

## O sintoma que custaria >10min para redescobrir

`internal/generators/agentfiles.go`'s `globalGitBranchGuardInstalledClaude()`/
`globalCredentialGuardInstalledClaude()` (a mesma família, `hookArrayHasCommand`) e
`internal/validator/validator_git_branch_guard.go`'s `validateGuardGlobalHookResolvable`/
`collectCommandsWithMarker` **só comparam o campo `command`** (via `samePathCommand`, ou por
`strings.Contains` do marker em qualquer string da árvore JSON). Nenhum dos dois olha para o campo
`"type"` do objeto hook — mesmo que o **escritor** (`mergeClaudeHookArray`, mesma arquivo, linha
~1497) sempre grave `{"type": "command", "command": <path>}`.

O Claude Code real (https://code.claude.com/docs/en/hooks) documenta `"type":"command"` como parte
da forma válida de um hook de comando. Uma entrada `{"command": <path-correto>}` **sem** `"type"` é
inerte no harness real — mas passa despercebida por TODO o mecanismo de verificação do trackfw:

```
globalGitBranchGuardInstalledClaude()   -> true   (dedup acredita que está instalado)
InjectClaudeHooks (projeto)             -> PULA a entrada de projeto (por causa do dedup)
trackfw validate                        -> ZERO mensagens (hook_resolvable só confere existência
                                            do script no path, nunca a forma do objeto hook)
```

Resultado: nem a fiação de projeto nem a global protegem o repositório, e `trackfw validate` fica
verde. Reproduzido por execução real (teste Go descartável, não commitado — ver
`docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md`, seção B).

## Por que não é óbvio de ler o código isoladamente

`hookArrayHasCommand`/`globalCredentialGuardInstalledClaude` têm comentários extensos sobre
fail-open e normalização de caminho (`normalizeGuardPath`, ML-2C) — a atenção do leitor vai para
"o caminho compara certo?", não para "o objeto inteiro é um hook válido?". O padrão se repete nos
6 CLIs e nos dois guards (credential-guard tem exatamente o mesmo formato de bug, não é exclusivo do
git-branch-guard) — corrigir só um lugar deixaria o defeito vivo nos outros 11 call-sites
equivalentes.

## O que fazer se for corrigir

Espelhar no lado de leitura (`hookArrayHasCommand`, `collectCommandsWithMarker`/
`validateGuardGlobalHookResolvable`) a mesma checagem de forma que o escritor já aplica — para
Claude/Codex, confirmar `hObj["type"] == "command"` antes de aceitar o match; para Gemini/Cursor/
Copilot/Kiro, o campo equivalente do schema de cada um. Mesmo custo de implementação em cada um dos
3 stacks (Go/Node/Python) que já pagou pela normalização de caminho do ML-2C.

## Não é o mesmo achado que...

- [validate-global-guard-integrity-by-existence-makes-unisolated-home-systemic-2026-08-18](validate-global-guard-integrity-by-existence-makes-unisolated-home-systemic-2026-08-18.md)
  — aquele é sobre `$HOME` não isolado quebrando testes/gates; este é sobre a SEMÂNTICA da
  comparação em si, independente de isolamento.
- O débito residual #1 do ML-2A (dedup ausente) e #1 do ML-3A (resolvability condicionada à
  fiação) — ambos já corrigidos por ML-2B/ML-3A; este é um terceiro gap, nunca nomeado em nenhum
  ML da série, que sobrevive mesmo depois de todos os 6 MLs do roadmap.

## Severidade

Exige `$HOME` já gravável pelo agente/atacante — mesmo pré-requisito que o
`ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-...` já declara fora do modelo de defesa.
Não bloqueei o roadmap por causa disso (não é regressão nem escalada nova), mas recomendei nomear
como débito residual explícito no ADR em vez de deixar implícito.
