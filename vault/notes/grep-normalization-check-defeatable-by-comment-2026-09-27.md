---
date: 2026-09-27
domain: gate-design
severity: high
ml: ML-3C (ROADMAP-2026-09-23)
---

# grep-qF em bloco com comentários é derrotável por prosa

## O defeito

`scan_file()` em `check-crlf-normalize-capture.sh` coleta o bloco de lookahead com **todas as linhas** (incluindo comentários `#`) e depois faz:

```bash
grep -qF 'strip_cr' <<<"$block"
grep -qF 's/\r$//' <<<"$block"
```

Uma linha de comentário imediatamente após a captura como:

```bash
BAD_VAR=$(python3 -c 'print("hello")')
# sed $'s/\r$//': strips CRLF -- comment-only, not in pipeline
```

faz `grep -qF 's/\r$//'` retornar exit 0 e o gate **aprova** a captura sem normalização.

## Medido

`scaffold.go:939-940` tem exatamente este padrão:
```
  # sed $'s/\r$//' strips trailing \r added by python3 print() in Windows text mode.
  TOOL=$(echo "$INPUT" | ... python3 ... | sed $'s/\r$//' || echo "")
```

No caso real, o `sed` também está no pipeline (linha 941), então não há falso negativo no corpus real. Mas o defeito existe: qualquer arquivo que tenha o comentário antes mas o `sed` ausente do pipeline passaria.

## A correção (ML-3C)

Construir `block_code` em paralelo com `block`, filtrando linhas de comentário (`^[[:space:]]*#`), e checar normalização contra `block_code`:

```bash
while [ $j -le $limit ]; do
  local bline="${lines[$j]}"
  block+="$bline"$'\n'
  if ! [[ "$bline" =~ ^[[:space:]]*# ]]; then
    block_code+="$bline"$'\n'
  fi
  ((j++))
done
# ...
grep -qF 'strip_cr' <<<"$block_code" || grep -qF 's/\r$//' <<<"$block_code"
```

## Caveat: `strip_cr` em comentário

O mesmo defeito existe para `grep -qF 'strip_cr' <<<"$block"` (antes da correção). Na prática é menos provável porque `strip_cr` aparece quase exclusivamente em código ativo, mas o mesmo filtro agora protege os dois.

## Braço de falsificação

Braço H do Cenário 197 de `check-gates-falsify.sh` prova que o gate agora **reprova** quando `s/\r$//` aparece apenas em comentário.
