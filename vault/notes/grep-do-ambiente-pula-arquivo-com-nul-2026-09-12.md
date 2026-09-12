# `grep` do ambiente (`ugrep -I`) pula `npm/src/validator/index.js` em silêncio

> 2026-09-12 · Hefesto (Code Quality) · medido com `type grep`, `grep -c` vs `/usr/bin/grep -c`

## Causa raiz desta nota

Duas REQs foram abertas com a premissa "regra ausente no Node", baseadas em `grep` que retornou 0
ocorrências em `npm/src/validator/index.js`. A triagem medida de 2026-09-12 (ML-1A) revelou que a
premissa era falsa: o código estava lá desde a origem. A ferramenta de busca, não o código, era o
problema.

## O mecanismo medido

```bash
type grep
# → grep is a shell function from /Users/kgsaran/.claude/shell-snapshots/snapshot-zsh-*.sh
# (resposta do ugrep 7.8.4)

grep -c note_orphan npm/src/validator/index.js; echo "RC=$?"
# → (vazio)
# → RC=1   ← ugrep com -I ativo trata o arquivo como binário e o pula em silêncio

/usr/bin/grep -c note_orphan npm/src/validator/index.js; echo "RC=$?"
# → 3
# → RC=0   ← grep nativo do sistema encontra 3 ocorrências
```

**Por que o ugrep pula:** `npm/src/validator/index.js` contém um byte NUL literal (offset 83123,
linha 1855: `const seenKey = \`${m.raw}<NUL>${m.typeIsCommand}\``). O flag `-I` do ugrep significa
"tratar arquivos binários como se não contivessem correspondência" — o arquivo inteiro é silenciado,
e o comando sai com RC=1 como se o padrão simplesmente não existisse.

O `file` classifica o arquivo como "Unicode text, UTF-8 text", então a inspeção visual não revela a
armadilha. Ver também: [[index-js-tem-um-byte-nul-no-fonte-file-diz-texto-e-grep-diz-binario-2026-09-06]]

## As duas REQs afetadas

| REQ | Premissa falsa | Evidência medida (ML-1A) |
|---|---|---|
| `note_orphan` (REQ-2026-08-20) | "ausente no Node" | `npm/src/validator/index.js:1621,3469,3758` |
| `thirdparty_artifact_has_provenance` (REQ-2026-09-01) | "ausente no Node" | `npm/src/validator/index.js:3614+` (8 ocorrências) |

Ambas as REQs permanecem **abertas** porque um AC de gate cross-CLI ainda não foi satisfeito — a
premissa era falsa, mas o requisito não era. As REQs foram reclassificadas como PARCIAL.

## O que fazer antes de declarar ausência no Node

1. **Usar `/usr/bin/grep` ou `git grep`**, nunca o `grep` do shell neste ambiente.
2. Verificar com o binário, não só com a fonte — presença no fonte não prova comportamento.
3. Se o `grep` retornar 0 em `npm/src/validator/index.js`, re-testar com `/usr/bin/grep -a` antes
   de declarar ausência.

## Nota de sobreposição

A nota [[index-js-tem-um-byte-nul-no-fonte-file-diz-texto-e-grep-diz-binario-2026-09-06]] cobre o
mecanismo do byte NUL e o paradoxo da medição (grep sem `-a` sendo derrotado pelo próprio objeto).
Esta nota cobre o ângulo de **investigação de REQs**: como duas REQs foram abertas com evidência
falsa, como confirmar, e qual o resultado da triagem. O arquiteto pode decidir fundir as duas.
