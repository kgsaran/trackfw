# Defeito de autorrelato em `check-gates-falsify.sh`: mensagem deslocada + contador estático

> 2026-09-16 · ML-3C-bis · Ártemis

## O defeito

`check-gates-falsify.sh` tinha, na linha 6272 do original (6641 linhas), a linha:

```bash
echo "Falsification checks passed (all 183 scenarios, 23 gates + ..."
```

Mas ~370 cenários **continuavam após essa linha**. O resultado: se qualquer um desses cenários falhava, o script imprimia a mensagem de sucesso e depois saía com exit 1 — o relatório de saída mentia sobre o resultado.

Além disso, o número "183" era hardcoded e desatualizado (o script executava bem mais cenários).

## A causa raiz

Não há automação que impeça inserir cenários após uma mensagem de resumo colocada no meio do script. O arquivo cresceu organicamente; a mensagem ficou parada enquanto novos cenários foram acumulados depois dela.

## O fix

1. **Remover** a mensagem hardcoded da linha 6272.
2. **Adicionar arquivo `$FALSIFY_SUCCESS_TALLY`** (padrão idêntico ao `$FALSIFY_ENUM_TALLY` já existente — arquivo, não variável shell, para sobreviver fronteiras de subshell).
3. **Adicionar `falsify_count_success()`** que incrementa o arquivo com `printf 'x\n'`.
4. **Instrumentar todos os 65 pontos `echo "OK   [falsify/..."`** no script:
   - 7 funções helper (`assert_fails_with`, `assert_output_contains`, etc.)
   - 39 `echo "OK   [falsify/..."` não-indentados no body
   - 19 `echo "OK   [falsify/..."` indentados (dentro de blocos `if`) no body
5. **Adicionar mensagem medida na última linha do script**:
   ```bash
   echo "Falsification checks passed (${falsify_success_n:-0} scenarios)"
   ```

## Armadilha de instrumentação: echoes indentados

A varredura inicial instrumentou apenas os echoes não-indentados (regex `^echo "OK   \[falsify/`). Os 19 echoes dentro de blocos `if` com indentação de 2 espaços não foram capturados na primeira passagem. Foram encontrados na segunda via análise Python: procurar todas as linhas `echo "OK   [falsify/"` que **não** têm `falsify_count_success` imediatamente antes.

## Echoes não contados (documentados)

Sub-scripts externos chamados bare (`bash "$ROOT_DIR/scripts/check-wheel-filename.sh" --falsify-raw`) emitem suas próprias linhas `OK   [falsify/...]` diretamente para stdout. Essas linhas **não incrementam** `$FALSIFY_SUCCESS_TALLY` — são ~2 linhas adicionais que explicam por que `make quality` reporta 212 OK enquanto o script reporta 201.

## Falsificações obrigatórias executadas

**Direção A** (mensagem não pode mentir após a linha 6272 original):
- Injeção: mudou string esperada de `assert_fails_with "integration-assets/direction-b-shim-absent"` para `"DIRECTION_A_FALSIFICATION_INJECTED_UNLIKELY_MATCH"`
- Resultado: `FAIL [falsify/integration-assets/direction-b-shim-absent]: saiu com 1 mas falta diagnóstico '...'`, exit 1, "Falsification checks passed" NÃO impresso
- Restauração: exit 0, "Falsification checks passed (201 scenarios)" como última linha ✓

**Direção B** (contador mede):
- Remoção: comentou `falsify_count_success` + `echo "OK   [falsify/integration-assets/baseline]"`
- Resultado: `Falsification checks passed (200 scenarios)` — 201 → 200 (exatamente -1) ✓
- Restauração: `Falsification checks passed (201 scenarios)` ✓

## Relacionado

- `scripts/check-gates-falsify.sh` — script corrigido
- `vault/notes/enum-mode-return-1-e-variavel-de-shell-reintroduzem-o-exit-1-2026-09-08.md` — mesma família: subshell boundary faz contador em variável shell silenciosamente zerar
