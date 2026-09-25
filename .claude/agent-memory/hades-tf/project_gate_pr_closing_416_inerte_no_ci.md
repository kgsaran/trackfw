---
name: gate-pr-closing-416-inerte-no-ci
description: O PR #416 fez o gate de palavra-chave ler o corpo vivo pela API, mas quality.yml nao define GH_TOKEN — no CI o caminho da API nunca executa e a forma 1 segue viva
metadata:
  type: project
---

`scripts/check-pr-closing-keyword.sh` prefere o corpo vivo (`gh pr view`) ao payload imutável desde o
PR #416 (mergeado 2026-09-24). **No CI isso nunca executa:** `.github/workflows/quality.yml` não define
`GH_TOKEN` em nenhum lugar (outros workflows definem — `check-annotations.yml:39`, `release.yml:419`), então
`gh` falha e o gate cai no payload. Medido no log do run `36074650903` (PR #425, depois do merge):
`fonte: payload do evento (corpo de ABERTURA do PR #425)`.

A tabela de medição do #416 foi produzida **localmente**, com `gh` autenticado — ambiente que o CI não tem.

🔴 **Acoplamento não óbvio: esse defeito MASCARA o falso positivo de negação.** O gate da época de #293
acusa o corpo vivo de #293 (`**Não fecha #290**`) e o check foi `SUCCESS` — as linhas entraram por edição
posterior ao último evento com payload. Logo **corrigir `types: [… edited]` sozinho ativa falso positivo em
todo PR que declara escopo negativo**. Ordem obrigatória: negação + markdown no mesmo commit, `edited` depois.

**Why:** o código está correto e o diff engana o auditor; só o log do run diz a verdade.
**How to apply:** ao auditar qualquer gate que use `gh` em Actions, confira `GH_TOKEN` no step e leia a
linha de fonte no log do run — nunca conclua "corrigido" pelo diff. Ver
[[verify-by-execution]] e [[measure-the-real-code-path-not-an-isolated-proxy]].
