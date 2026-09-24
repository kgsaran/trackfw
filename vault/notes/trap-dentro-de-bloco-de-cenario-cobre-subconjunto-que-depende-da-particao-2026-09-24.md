# Trap instalado dentro de bloco de cenário cobre um subconjunto de chunks que **depende da partição** — e `$(grep … | wc -l)` mata o chunk em silêncio

> 2026-09-24 · ML-2E, REQ-2026-09-23 (a apuração do censo morre no shard limpo) · medido em
> `bash 5.3` (macOS/ARM64) e `git 2.55.0.windows.3` (VM Windows 11)

Dois achados independentes, mesmo sintoma: **chunk do censo que morre sem produzir número**.

---

## 1. Onde um `trap` é escrito decide quantos chunks ele cobre — e o número muda com `n_chunks`

`scripts/gen-falsify-chunks.py` monta cada chunk como

```
prelude_lines = lines[:prelude_end] + <segmentos de SUPORTE>
chunk_i        = prelude_lines + <apenas os blocos de cenário sorteados para i>
```

Logo: **qualquer coisa escrita dentro de um bloco de cenário existe só no chunk que recebeu aquele
bloco.** O ML-2A instalou `trap … ERR` (a denúncia `CHUNK_ABORT`) dentro do bloco do Cenário 18, e
o relatório dele declarou o limite como *"cobre deste ponto até o fim do processo"* — verdadeiro
para o arquivo inteiro, **falso para o chunk**.

🔴 **O que torna isto difícil de enxergar:** a cobertura é função da partição. Medido gerando o
mesmo fonte com vários `n_chunks`, procurando em que chunk cai o Cenário 18 (trap) e em que chunk
cai o sítio que matou o `chunk_0` do censo (Cenário 69, `grep -oF … | wc -l`):

| `n_chunks` | chunks com trap | sítio do Cenário 69 tem trap no chunk? |
|---|---|---|
| 4  | 1 de 4  | **sim** (coincidem no `chunk_0`) |
| 8  | 1 de 8  | **sim** (coincidem no `chunk_0`) |
| 12 | 1 de 12 | **não** |
| 16 | 1 de 16 | **não** |
| 60 | 1 de 45 | **não** ← a partição usada na medição do ML-2D |

Comando (bash explícito):

```bash
for N in 4 8 12 16 60; do
  d=/tmp/ch$N; mkdir -p $d
  python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh $d $N >/dev/null 2>&1
  t=$(grep -l '__falsify_abort_report' $d/chunk_*.sh | wc -l)
  k=$(grep -l 's69bad_gbg_count' $d/chunk_*.sh)
  echo "N=$N com_trap=$t sitio_em=$(basename $k) trap_nesse_chunk=$(grep -c '__falsify_abort_report' $k)"
done
```

Quem reproduzisse com `N=8` veria trap e sítio no mesmo chunk e concluiria que **não há defeito**.
A auditoria do ML-2D mediu com `N=60` e viu o `chunk_0` morrer sem uma linha de `CHUNK_ABORT`. As
duas observações são corretas; o que as reconcilia é a partição.

**Correção (ML-2E):** o bloco do trap passou para `lines[:prelude_end]` — imediatamente depois do
`trap 'rm -rf "$WORK"' EXIT`. Verificação em dois caminhos: `prelude_end_line` 1431 → **1492**
(exatamente as 61 linhas inseridas), `total_segments` / `fused_units` / lista de 234 rótulos
**idênticos**; e `grep -l __falsify_abort_report chunk_*.sh` passa a casar **100% dos chunks** em
N=4/8/12/16/60.

🔴 **Duas decisões do ML-2A que não podem ser trocadas** (as duas foram medidas, e a nota existe
para que ninguém as "simplifique" depois):

- **`ERR`, nunca `EXIT`.** O preâmbulo já usa o slot de `EXIT` para `rm -rf "$WORK"`; um segundo
  `trap … EXIT` **substitui** o primeiro e vaza um diretório temporário por execução.
  Verificado imprimindo `trap -p EXIT` de dentro de um chunk gerado (o `rm -rf "$WORK"` continua
  lá) e confirmando, por segundo caminho, que o diretório não existe mais depois da saída.
- **Sem `set -E` (errtrace).** Com errtrace o trap é herdado por funções e subshells, e o laço de
  medição roda cada gate num subshell cuja saída vai para um `"$_log"`: **toda falha esperada de
  gate escreveria `CHUNK_ABORT` dentro do log**, poluindo diagnóstico legítimo. O preço aceito é
  que falha ocorrida dentro de função/subshell não é denunciada.

---

## 1-bis. 🔴 `trap … ERR` dispara **mesmo com `set +e`** — e o diagnóstico passa a mentir

Descoberto ao levar o trap para o preâmbulo, medindo o efeito colateral em vez de presumi-lo.

O manual do bash condiciona o `ERR` às mesmas exclusões do `errexit` (teste de `if`/`while`, elo
não-final de `&&`/`||`, negação por `!`) — mas **não** ao `errexit` estar ligado. Medido em
`bash 5.3`:

```bash
set -euo pipefail
trap 'echo "TRAP [$-] cmd=$BASH_COMMAND" >&2' ERR
set +e
false          # -> TRAP [huB]   (sem `e`)  ... e o script SEGUE
set -e
false          # -> TRAP [ehuB]  (com `e`)  ... e o script morre
```

`check-gates-falsify.sh` usa `set +e … set -e` em dezenas de blocos justamente para capturar a
saída de um comando que **deve** falhar. Consequência medida ao rodar `chunk_3` de N=8: **2 falsos
positivos de `CHUNK_ABORT`** num chunk que terminou `rc=0`, com 36 `OK` e 0 `FAIL` — os dois
`s68dup_out=$(… trackfw validate 2>&1)` do Cenário 68.

🔴 **Isso é pior que não ter diagnóstico:** a linha `CHUNK_ABORT` afirma *"o shell abortou por
`set -e`"*, e com errexit desligado a afirmação é falsa. Quem estivesse triando um shard mudo iria
atrás de um aborto que não houve.

**Correção:** o handler devolve cedo quando o errexit está desligado.

```bash
case "$-" in
  *e*) ;;          # errexit ligado: o aborto é real, reporta
  *)   return 0 ;; # errexit desligado: nada vai abortar, silêncio é o certo
esac
```

Depois da guarda: `chunk_3` volta a **0** `CHUNK_ABORT`, com rótulos byte-idênticos ao pristino; e
a denúncia continua saindo no caso real (injeção sob errexit ligado, em chunk sem o Cenário 18).

⚠️ **O falso positivo já existia antes desta REQ**, latente: qualquer chunk que recebesse o
Cenário 18 **antes** do 68 o produzia. A mudança de alcance só o tornou sistemático — e visível.

---

## 2. `$( grep … | wc -l )` sob `pipefail` transforma "zero ocorrências" em morte do chunk

```bash
set -euo pipefail
out="nada aqui"
echo antes
n=$(grep -oF ausente <<<"$out" | wc -l)   # grep não casa -> rc 1 -> pipefail -> set -e
echo "DEPOIS n=$n"                         # nunca executa
```
→ imprime `antes`, `rc=1`, e `DEPOIS` **não sai**.

🔴 **É a mesma raiz da Wave 1 desta REQ** — `grep` que sai **1** no caminho de não-casamento — em
outra forma. Lá o sintoma foi `$'0\n0'` (captura com `|| echo 0` somando o `0` que o `grep -c` já
emitia); aqui o `grep` não tem fallback nenhum, e o que morre é o processo. Zero ocorrências é
justamente o dado que a asserção seguinte quer medir.

**Forma correta** (a mesma que o ML-1A padronizou), preservando `-o … | wc -l` porque ele conta
**ocorrências**, não linhas — `grep -c` não é substituto:

```bash
n=$( { grep -oF 'padrao' <<<"$out" || true; } | wc -l | tr -d ' ')
```

Medido nas duas direções: sem casar → `0`; com 2 ocorrências → `2`; `rc=0` nos dois.
🔴 **Não** usar `|| echo 0` (é o defeito da Wave 1) nem `${VAR:-0}` (guarda sobre captura é o
padrão que produziu tudo isto).

**População medida** em `scripts/check-gates-falsify.sh` (varredura de substituições de comando com
parênteses balanceados, não orientada a linha):

```python
# varredura de $( … ) com parênteses BALANCEADOS — regex de linha não serve:
# uma substituição pode atravessar linhas e conter parênteses internos.
import re, glob, os
def spans(text):
    out=[]; i=0
    while True:
        j=text.find('$(', i)
        if j<0: break
        d=0; k=j+1
        while k<len(text):
            if text[k]=='(': d+=1
            elif text[k]==')':
                d-=1
                if d==0: break
            k+=1
        out.append((text.count('\n',0,j)+1, text[j+2:k])); i=j+2
    return out
for f in sorted(glob.glob('scripts/*.sh'))+sorted(glob.glob('.github/workflows/*.yml'))+['Makefile']:
    if not os.path.exists(f): continue
    t=open(f,encoding='utf-8',errors='replace').read()
    pipefail='pipefail' in t
    for ln,b in spans(t):
        if '|' not in b or not re.search(r'\bgrep\b', b): continue
        flat=' '.join(b.split())
        guarded='|| true' in flat
        comment=t.split('\n')[ln-1].lstrip().startswith('#')
        print(f"{f}:{ln} pipefail={pipefail} guarded={guarded} comment={comment} :: {flat[:130]}")
```

Rodado sobre o corpus inteiro em 2026-09-24: **73** ocorrências brutas. Filtrando `pipefail=True`,
`guarded=False`, `comment=False` e descartando as 13 da forma `|| echo 'no model line'` (o `||` no
topo da substituição já devolve rc 0, e `grep` sem `-c` **não** emite no caminho de falha — é a
forma correta segundo o gate do ML-1B), sobram **10 candidatos da mesma causa fora de
`check-gates-falsify.sh`**, todos com `grep` em posição NÃO-final de pipeline dentro de `$( … )`:
`check-agent-namespace-union.sh:632,900,901` · `check-ci-workflow-pin-parity.sh:192,211` ·
`check-manifest-version-gate.sh:113` · `check-parity-call-site-pins.sh:261` ·
`check-serve-address-parity.sh:229` · `check-serve-api-file-security.sh:76` ·
`.github/workflows/check-annotations.yml:80`. Em cada um a pergunta é a mesma: *"não casar é um
resultado válido da medição?"* — se for, falta `|| true`. Não corrigidos aqui (fora do escopo de
escrita do ML-2E); nomeados para a REQ vigente, não empurrados para REQ nova.
→ 10 substituições com `grep`, das quais 6 são prosa de comentário. Reais: **4**.
**3 defeituosas** (`:4961`, `:5003`, `:5109` no fonte pré-correção) — todas `grep -oF … | wc -l`
em atribuição de topo de script. **1 isenta** (`:1970`): vive dentro de `SIMPLE_REQ_FIELD_SCRIPT`,
que roda por `bash -c` — shell novo, com `set -e` mas **sem** `pipefail`, e `SHELLOPTS` não é
exportado em lugar nenhum do arquivo, então a opção não atravessa.

### O gate do ML-1B **não** cobre esta forma, e por um bom motivo

`scripts/check-emitting-capture-fallback.sh` exige, no discriminante, a **coexistência** de
(1) comando que emite no caminho de falha **e** (2) **fallback que também emite** (`|| echo` /
`|| printf`). Estes sítios **não têm fallback algum** — o modo de falha é *pipeline desguarnecido
sob `pipefail`*. É um discriminante **irmão**, não um alargamento de regex do existente: ampliar o
gate atual acusaria todo `$(a | b)` legítimo. Registrado como candidato a ML próprio.

---

## Como isto se detecta da próxima vez

- **Trap/handler em script que é fatiado por gerador:** pergunte ao gerador, não ao olho —
  `python3 scripts/gen-falsify-chunks.py <fonte> <dir> <N>` imprime `prelude_end_line`. Qualquer
  instalação que precise valer para **todo** chunk tem de estar **antes** dessa linha. E meça com
  mais de um `N`: coincidência de bucket esconde o defeito.
- **`grep` dentro de `$( … )` sob `pipefail`:** o caminho de não-casamento é rc **1**, não 0.
  Se "nenhuma ocorrência" é um resultado válido da medição, o `grep` precisa de `|| true`.
