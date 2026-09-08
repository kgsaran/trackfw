# Transformar um job required-by-name em matriz não fica vermelho — fica pendente para sempre

**ML-2G** (ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-sem-perder-cobertura.md).

## O que foi medido

`gh api repos/kgsaran/trackfw/branches/main/protection` mostra
`required_status_checks.contexts` incluindo `"parity"` **por nome exato**, com
`strict: false`. O comentário histórico em `.github/workflows/quality.yml`
(escrito em 2026-09-03) já registrava isso como razão para não shardar o job
`parity` — mas descrevia o sintoma como "matriz renomeia o check e bloqueia
todo PR", o que é impreciso de um jeito que importa para quem for debugar:

**Não bloqueia — nunca reporta.** Se o job `parity` virasse
`strategy.matrix`, o GitHub passaria a reportar checks `parity (0)`,
`parity (1)`, `parity (2)`... Nenhum check chamado exatamente `parity`
apareceria nunca. O PR não fica com um X vermelho — fica **pendente para
sempre** no check obrigatório, porque o check que a branch protection espera
literalmente não existe mais sob esse nome. É mais difícil de diagnosticar
que um vermelho: parece um problema de infra do GitHub, não do workflow.

## A correção que generaliza

Nunca colocar `strategy.matrix` no job que carrega o nome required. Desenhar
como **job de agregação**: os workers viram matriz sob IDs novos
(`parity-falsify-shard`, `parity-other-gates`), e o job antigo (`parity`)
vira um job fino com `needs: [<os workers>]` que só resume o resultado —
preservando o nome que a branch protection espera.

Duas armadilhas do desenho de agregação em si, não específicas deste caso:

1. `if: always()` é necessário para o job de agregação RODAR mesmo quando um
   `needs` falhou (senão o GitHub pula o job inteiro e o check obrigatório
   também não reporta — mesmo sintoma, causa diferente). Mas `if: always()`
   sozinho faz o job **passar** por padrão nesse caso — precisa de um step
   explícito checando `contains(needs.*.result, 'failure') ||
   contains(needs.*.result, 'cancelled')` antes de qualquer outra coisa.
2. Se o job de agregação reconstrói alguma garantia a partir do que os
   workers alegam ter feito (ex.: uma guarda de cobertura de conjunto), os
   valores ESPERADOS têm que vir de uma fonte independente dos workers (ex.:
   checkout fresco do próprio job de agregação), nunca de algo que o worker
   também uploadou — senão é o mesmo defeito auto-referencial que o
   `hades-tf` achou no ML-2D (guarda que deriva do mesmo fonte que pode ter
   sido sabotado), só que em escala de job em vez de processo.

## Como verificar antes de shardar qualquer job

```bash
gh api repos/<owner>/<repo>/branches/<branch>/protection --jq '.required_status_checks.contexts'
```

Se o job-alvo aparecer na lista, o nome é contrato — trate como API estável,
não como rótulo cosmético.
