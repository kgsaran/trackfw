# Erro de expansão aritmética mata o laço — e o script segue, verde, sob `set -euo pipefail`

> 2026-09-23 · medido na REQ do censo (apuração 2/8) · `bash 5.3.20`

## A suposição errada que custou dias

`set -euo pipefail` no topo de um step de CI passa a impressão de que **qualquer** erro derruba o
job. Para expansão aritmética, **não derruba**.

```bash
set -euo pipefail
T=0
for i in 1 2 3; do
  V=$'0\n0'          # valor com duas linhas
  T=$((T + V))       # <- erro aqui
  echo "iteracao $i ok"
done
echo "DEPOIS DO LAÇO — o script continuou"
```

```
line 5: 0
0: arithmetic syntax error in expression (error token is "0")
DEPOIS DO LAÇO — o script continuou
rc final=0
```

🔴 **Leia as três coisas que aconteceram:**

1. **Nenhuma** iteração imprimiu `iteracao N ok` — o erro não pula a linha, **mata o laço inteiro**.
2. O script **continuou** na linha seguinte ao laço.
3. **`rc=0`.** O step fica **verde**.

## Como isso se manifestou de verdade

No `windows-census.yml`, a apuração do censo de Windows:

```bash
CNT_FAIL_GREP=$(grep -ac '^FAIL' "$LOG_FILE" 2>/dev/null || echo 0)
```

`grep -c` **já imprime `0`** quando não casa e **sai 1** → o `|| echo 0` acrescenta um segundo `0` →
a captura vira `$'0\n0'` → o `$(( ))` seguinte quebra → o laço que percorre os 8 shards morre na
primeira iteração de um shard **sem falha nenhuma**.

O job publicou `TOTAL INCOMPLETO — 2/8 shards` e marcou 6 shards como ausentes — **no mesmo step
cujo próprio `ls` listou os 8 diretórios** — e o GitHub registrou:

```
$ gh run view 35872779844 --json jobs
apuracao → success
```

**O instrumento oficial de medição de Windows ficou mudo por dias, com o check verde.**

## Por que a defesa existente não pegou

O workflow tinha uma contagem cruzada com `awk`, escrita exatamente para pegar contador mentiroso.
Ela disparou — **e mentiu**:

```
DISCREPÂNCIA FAIL shard 1: grep=0
0 awk=0
```

`grep=0\n0` contra `awk=0`: uma divergência **que não existe**. 🔴 **Comparar dois contadores não
protege contra uma forma de captura quebrada** — os dois estavam certos; o que estava errado era o
valor ter duas linhas.

## O que fazer

**A forma correta é `|| true`, não `|| echo 0`:**

```bash
CNT=$(grep -ac '^FAIL' "$LOG" 2>/dev/null || true)     # true não escreve nada
```

| forma | sem match | com match |
|---|---|---|
| `\|\| echo 0` | `$'0\n0'` 🔴 | `2` |
| `\|\| true` | `0` | `2` |

Regra geral: **`$(cmd … || echo N)` só é seguro se `cmd` não escrever em stdout no caminho de
falha.** `wc -l < arquivo` e `jq` são seguros (não imprimem ao falhar); `grep -c` **não é**.

**Limitação medida e deixada sem guarda:** `|| true` devolve string **vazia** se o `grep` sair `2`
(arquivo ilegível). Ali o `[[ -f ]]` a montante já barra. Não empilhe `${VAR:-0}` por cima — isso é
guarda sobre captura quebrada, que é o padrão que produziu este defeito.

## O que esta nota NÃO diz

Não diz que `set -e` é inútil. Diz que ele **não cobre erro de expansão aritmética**, e que um step
verde não prova que o laço rodou. Se um laço produz um número, **o número precisa ser afirmado por
alguém** — a ausência de erro não basta.

Relacionado: `bash-consome-stdout-de-python3-e-recebe-cr-invisivel-no-windows-2026-09-23.md` —
mesma família (a captura carrega algo que ninguém vê), e o mesmo instrumento.

## Guarda que passou a existir (ML-1B, 2026-09-23)

`scripts/check-emitting-capture-fallback.sh` reprova, em `.github/workflows/*.yml`, `scripts/*.sh`,
`scripts/*.py` e `Makefile`, toda substituição de comando onde coexistem (1) `grep` com `-c`/`--count`
e (2) fallback `|| echo`/`|| printf`. Ele **não** reprova `|| true`, nem `grep` sem `-c`, nem
`wc -l <`/`jq` com `|| echo 0` — as formas corretas medidas acima.

Duas coisas que essa guarda ensina e que valem para a próxima do gênero:

- **`grep -c PAT /arquivo/ausente` sai `rc=2` com stdout VAZIO.** O `$'0\n0'` nasce só do caminho de
  **não-casamento** (`rc=1`, stdout `0`) — que é exatamente o caminho do shard limpo. Confundir os
  dois caminhos leva a concluir que a captura seria `$'\n0'`, e ela não é.
- **Um token que apenas CONTÉM a letra `c` não é flag de contagem** — `--color=never`. O
  discriminante do gate exige o pacote de flags como token inteiro; sem isso o gate reprovaria uma
  invocação legítima. Braço negativo `falsify/emitting-capture/color-flag`.
  🔴 Medido 2026-09-23, e vale para quem for enumerar a família de novo: a varredura usada pelo
  ML-0A (`grep -rnE 'grep +-[a-z]*c[a-z]* '`) **não casa `--color=never`** (rc=1, ok) **mas também
  não casa `--count`** (rc=1) — casa só `-ac`. Enumerar por ela deixa a forma longa de fora.

Formas que o gate declara **não** cobrir (crase, continuação de linha, captura aninhada, captura
indireta via função, `diff` como comando emissor) estão no cabeçalho do script, cada uma com o
comando de medição e a contagem no corpus.
