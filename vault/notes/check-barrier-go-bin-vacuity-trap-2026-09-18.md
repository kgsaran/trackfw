# check-barrier.sh GO_BIN vacuity trap

**Data:** 2026-09-18 | **REQ:** #392 | **ML:** ML-4D/ML-4E

## O defeito

Quando `GO_BIN` não está definido, `check-barrier.sh` **constrói seu próprio binário a partir do
`$ROOT_DIR` atual**:

```bash
if [[ -z "${GO_BIN:-}" ]]; then
  GO_BIN="$WORK/trackfw-go"
  (
    cd "$ROOT_DIR" && GOCACHE="$WORK/go-build-cache" go build -o "$GO_BIN" ./cmd/trackfw
  )
fi
```

Isso significa: se você sabotar um arquivo em `/tmp/sab/` e construir o binário sabotado em
`/tmp/sab/bin/trackfw`, mas rodar `bash scripts/check-barrier.sh` sem `GO_BIN` apontando para o
binário sabotado, o script compilará e usará o binário **correto** da árvore atual — e reportará
"All scenarios passed" mesmo que o sabotado fosse detectado.

## O que aconteceu no ML-4D

O ML-4D reportou:
> `check-barrier.sh` reporta "All check-barrier.sh scenarios passed" com o binário sabotado
> (`intVal < 1` em `roadmapdoc.go`). S11 aceita exit 0 ou 1, portanto a sabotagem passa invisível.

**Isso era um falso positivo.** A medição mais provável: o sabotado estava em `/tmp` mas
`GO_BIN` não foi setado, então `check-barrier.sh` construiu o binário correto.

## O estado real medido no ML-4E

Construindo o binário sabotado corretamente e passando `GO_BIN=/tmp/sab/trackfw-sabotaged`:

```
FAIL [barrier/two-wave-flow/wave1-passed]: expected exit 0 for Wave 1, got 1; 
stderr: trackfw barrier: malformed wave heading at line 8: "0" is not a valid wave label
```

- **S1 detecta a sabotagem** (fixture de S1 tem Wave 0 desde ML-4C)
- **Cenários 167 e 168** do `check-gates-falsify.sh` também detectam (já existiam)
- **S11 não é alcançado** com o binário sabotado (S1 falha primeiro e o script sai)

## Por que S11 aceita exit 0 ou 1 (legítimo)

O fixture de S11 (`/tmp/sXX/s11-wave-zero/`) não é um repo governado. O `validate` check bloqueia
com "2 violations, 1 warnings", então o barrier sai 1 — um veredito, não uma malformação.

Distinção: `wave_headings.status == "passed"` quando exit=1 por validate.

Se Wave 0 fosse malformada: `parseWaves` a enviaria para `malformed`, não para `waves`; `target==nil`
→ `usageExit(2)` seria chamado. Exit 1 por malformação de Wave 0 é **impossível por construção** —
a assertiva `exit 0 or 1 (never 2)` já pega a sabotagem.

## Protocolo correto para testar sabotagem

```bash
# 1. Sabotar em /tmp (nunca em internal/)
sed 's/intVal < 0 {/intVal < 1 {/' internal/roadmapdoc/roadmapdoc.go > /tmp/sab/roadmapdoc.go

# 2. Verificar que o sed alterou o arquivo
cmp -s /tmp/sab/roadmapdoc.go internal/roadmapdoc/roadmapdoc.go && echo "VACUIDADE! sed não alterou nada" || echo "OK — alterado"

# 3. Construir o binário sabotado (em /tmp, nunca em bin/)
# (copiar a árvore completa, substituir o arquivo sabotado, compilar)

# 4. Passar GO_BIN explicitamente
GO_BIN=/tmp/sab/trackfw-sabotaged bash scripts/check-barrier.sh
```

**Sem `GO_BIN` explícito, `check-barrier.sh` usa o código correto e a sabotagem é invisível.**
