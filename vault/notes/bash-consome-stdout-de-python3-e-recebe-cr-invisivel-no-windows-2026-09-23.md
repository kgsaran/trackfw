
## Limite do ponto único, medido depois (2026-09-24, PR #416)

🔴 **Nem todo gate pode fazer `source` do lib** — e a exceção é estrutural, não preguiça.

O Cenário **182** do `check-gates-falsify.sh` (`:6522-6539`) escreve uma cópia sabotada do
`check-pr-closing-keyword.sh` **fora de `scripts/`**:

```bash
sed 's/if num not in english:/if not english:/' "$S182_REAL" > "$WORK/s182-sabotado.sh"
```

Nessa cópia, `SCRIPT_DIR` é `$WORK` e `. "$SCRIPT_DIR/lib-crlf-normalize.sh"` **não resolve**. Trocar
para `ROOT_DIR` não salva: cinco gates reapontam `ROOT_DIR` no `--self-test` — é o defeito que o
ML-1A-bis corrigiu.

**A saída certa é resolver na origem, do lado do Python:**

```python
with open(destino, "w", encoding="utf-8", newline="\n") as out:
    out.write(str(n))
```

O valor vai para **arquivo**, com newline fixado — não há `\r` para remover, e não há `source` para
resolver. Vale quando o dado é pequeno e o gate é copiável.

**Consequência para o gate anti-reintrodução:** um gate que só procura `$(python3 …)` sem `strip_cr`
**não vê** esta forma, e está certo em não ver — aqui ela é a correção, não o defeito. É por isso
que o cabeçalho declara as formas não cobertas em vez de tentar cobrir tudo.
