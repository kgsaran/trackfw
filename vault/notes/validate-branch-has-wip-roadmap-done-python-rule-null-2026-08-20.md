# Python's `validate --json` tags `branch_has_wip_roadmap` violations with `"rule": null`

**Data:** 2026-08-20
**Onde:** `pypi/trackfw/validator.py` (`validate_branch_has_wip_roadmap`, `_enrich_items`, `_apply_rule`)
**Achado por:** apolo-tf, ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco, ML-2A —
durante a montagem do fixture cross-CLI do bloco `branch_has_wip_roadmap` done/ acceptance em
`scripts/check-validate-parity.sh` (ainda não existia gate algum exercitando esta regra em
`trackfw validate` nos 3 CLIs antes deste ML — só via `trackfw branch new`).

## Sintoma

Rodando os 3 binários reais contra a mesma fixture (branch `feat/sem-roadmap-nenhum`, `wip/` e
`done/` vazios), `validate --json` reporta a violação nos 3, com o mesmo texto de mensagem
byte-idêntico — mas só Go e Node.js tagueiam `"rule": "branch_has_wip_roadmap"` /
`"file": "feat/sem-roadmap-nenhum"`. Python reporta `"rule": null, "file": null` para esta violação
específica, e só esta:

```json
// Go e Node.js
{"rule":"branch_has_wip_roadmap","file":"feat/sem-roadmap-nenhum","message":"branch \"feat/sem-roadmap-nenhum\" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ ..."}

// Python
{"rule": null, "file": null, "message": "branch \"feat/sem-roadmap-nenhum\" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ ..."}
```

## Causa raiz

`_enrich_items(items, rule_name)` (`pypi/trackfw/validator.py`) só tagueia `rule`/`file` em itens
que já são `dict` — um item que chega como `str` puro passa pelo `else: result.append(item)` sem
alteração nenhuma. Toda outra regra Python retorna `list[dict]` no formato
`{"type": "violation", "message": ...}` (ex. `validate_wip_has_req`), que `_enrich_items` enriquece
corretamente. `validate_branch_has_wip_roadmap(cfg)` é a única exceção: retorna `list[str]`
(`return [BranchGovernanceOrientation(branch)]` / `return [BranchNoMatchingRoadmapMessage(...)]`),
então `_apply_rule` a chama, `_enrich_items` não sabe o que fazer com uma string e a deixa passar
sem `rule`/`file` — e o serializador `--json` a converte para `{"message": ..., "rule": None,
"file": None}` a jusante.

## Por que isso importa

Qualquer consumidor externo do `validate --json` do Python (dashboards, scripts, o próprio
`trackfw serve`, se algum dia ler JSON em vez de invocar Go diretamente) que filtre violações por
`"rule": "branch_has_wip_roadmap"` nunca vai encontrar essa violação especificamente quando rodando
sob o CLI Python — mesmo ela existindo e bloqueando o `exit_code`. Não afeta o texto da mensagem
(que é byte-idêntico nos 3 CLIs, então humanos lendo `validate` sem `--json` não percebem nada), só
o campo estruturado.

## Não corrigido neste ML — deliberado

Corrigir seria mudar o formato de retorno de `validate_branch_has_wip_roadmap` (de `list[str]` para
`list[dict]`, no padrão das outras ~15 regras Python) — mudança de comportamento de produto, fora
do escopo do ML-2A (ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco), cujo objetivo
era só provar a aceitação de `done/` cross-CLI. `scripts/check-validate-parity.sh` filtra a
violação por substring de mensagem (não por `rule`) para não ficar vacuamente bloqueado por este
achado, e **pina** a divergência explicitamente (assert que Python é `[None]` e Go/Node são
`["branch_has_wip_roadmap"]`) — se algum dos dois lados mudar, o gate reprova e aponta aqui.

## Próximo passo, se alguém for atrás

Trocar os `return [...]` de `validate_branch_has_wip_roadmap` para o formato dict padrão
(`{"type": "violation", "message": ...}`), igual a `validate_wip_has_req` e as demais — mudança
pequena e local, mas é mudança de contrato de `--json` (campo `rule`/`file` passa a ser preenchido
onde hoje é `null`), então merece REQ própria, não um ajuste incidental dentro de um ML de gate.

## Ver também

- `docs/cli-parity.md`, seção "Regra `branch_has_wip_roadmap` — comportamento unificado nos 3
  runtimes" — anotação `trackfw-contract` registra o achado.
- `scripts/check-validate-parity.sh` — bloco `branch_has_wip_roadmap done/ acceptance` (ML-2A).
