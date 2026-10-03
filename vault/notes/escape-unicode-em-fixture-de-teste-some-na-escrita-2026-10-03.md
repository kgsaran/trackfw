# Escape `\uXXXX` em fixture de teste some na escrita: o caso vira cópia de outro

> 2026-10-03 | REQ-2026-10-02 (guard de branch falha aberto sem jq) | ML-1D/ML-1E

## O que aconteceu

O caso C14 da tabela do guard prometia, no comentário, "evasão unicode `g` = 'g'". O payload
gravado no arquivo era `git push` **sem escape**, porque o `g` foi decodificado para `g` no
caminho entre o agente e o disco. O teste passava, mas era uma cópia exata do C01 e **não exercitava**
a decodificação de `\u` no valor. Nenhum teste falhou: só uma leitura do relatório pegou.

## Onde o escape some

- a ferramenta de edição de arquivo do agente decodifica `\uXXXX` no texto a escrever;
- o `zsh` também, em `printf`/`echo` com aspas que interpretam escape;
- o próprio transporte do relatório do agente: no texto que chega ao arquiteto, `g` aparece como `g`.

O ML-1E gravou os bytes com `python3` (`chr(92) + 'u0067'`) e conferiu com `od -c`.

## Como conferir

```bash
L=$(grep -n 'id: "C14"' internal/generators/git_branch_guard_test.go | cut -d: -f1)
sed -n "${L}p" internal/generators/git_branch_guard_test.go | od -c
# tem de aparecer:  \   u   0   0   6   7
```

🔴 **Regra:** todo caso de teste cujo comentário promete um escape tem de ser conferido **nos bytes**,
não no texto do relatório nem no diff renderizado. E vale provar que o caso **mede algo novo**:
sabote a decodificação que ele alega exercitar e exija que ele reprove.
