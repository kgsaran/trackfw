# done < <(...) esconde o exit status de set -e e pipefail — e origin/main como fixture de falsify quebra pós-merge

## Causa raiz

`done < <(find ... 2>/dev/null | sort)` esconde o exit code do `find` em dois níveis:
1. **Substituição de processo `<(...)`:** o comando dentro roda assincroneamente; `set -e` e `pipefail` não vêem o seu exit code.
2. **`2>/dev/null`:** silencia erros de permissão e de diretório inexistente; `find` retorna zero itens sem avisar.

Com isso, se `req_dir` não existir (yaml com typo, renomeação, primeiro clone), o laço não processa nada, `status=0`, e o script sai com "Referential integrity OK" — **exit 0 falso positivo**.

## Correção

Substituir o `< <(...)` por captura com verificação de exit code:

```bash
if ! req_list=$(find "$ROOT_DIR/$REQ_DIR" -maxdepth 2 -name "*.md" | sort); then
  echo "FAIL: find em '...' falhou" >&2; exit 1
fi
```

`pipefail` é herdado dentro de `$(...)` (medido: `bash -c 'set -euo pipefail; if ! out=$(false|sort); then echo captured; fi'` → `captured`). Então `if !` captura falhas no `find` sem precisar de `cmd; rc=$?` (que viria depois do `set -e` e exigiria outro cuidado).

## Armadilha pós-merge: origin/main como fixture de falsify

**Não usar `git show origin/main:scripts/<gate>` como fixture "versão velha" em check-gates-falsify.sh.**

Depois que o PR que contém o fix é mergeado, `origin/main` aponta para a versão CORRIGIDA. O braço que afirmava "versão velha passa com o bug" passa a executar a versão nova, que reprovaria — e o cenário fica permanentemente vermelho na main.

**Solução:** embutir a versão pré-fix como heredoc `<<'SABEOF'` no próprio cenário de falsificação. O heredoc é uma cópia explícita e estável. Para verificar que o fix foi realmente aplicado, usar `cmp -s` entre o script atual e o heredoc — se forem idênticos, o fix não foi aplicado e o `falsify_fail_point` acusa.

## Awk com `'` em heredoc `<<'SABEOF'`

Em um heredoc `<<'SABEOF'`, o conteúdo é literal — nenhuma expansão de shell ocorre. Para que o arquivo resultante tenha o truque de quoting `'"'"'` (necessário para embutir `'` dentro de um awk delimitado por `'...'`), escreva `'"'"'` literalmente no heredoc. O bash que EXECUTA o arquivo depois vai interpretar o truque corretamente.

Escrever `["']` diretamente no heredoc (ao invés de `["'"'"']`) resulta em:
- O arquivo contém `["']` 
- Bash parse-time: `'` dentro de `["` fecha o string simples do awk
- SyntaxError: "unexpected EOF while looking for matching `\"'"

## Arquivos afetados

- `scripts/check-referential-integrity.sh` — Wave 1, REQ-2026-09-03
- `scripts/check-gates-falsify.sh` — cenário 12 (continuação), guarda de vacuidade
