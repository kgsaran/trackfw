# `sed -n` não tem rc para propagar — o único discriminante é o **conteúdo**, e o conteúdo a guardar é a **expressão comparada**, não a captura

> 2026-09-24 · ML-2J, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3` (macOS/ARM64), `sed` BSD

Terceira nota da série. A primeira
(`nem-todo-grep-em-captura-e-defeito-a-posicao-do-sitio-decide-2026-09-24.md`) fechou a forma
**com cano**; a segunda
(`o-ou-true-que-vira-aprovacao-vacua-a-forma-sem-cano-e-o-que-vem-depois-da-atribuicao-2026-09-24.md`)
fechou a forma **sem cano** e corrigiu o teste de bolso. Esta fecha a forma em que **não existe rc
algum** — e mostra que a guarda de não-vacuidade, sozinha, ainda pode ser colocada no lugar errado.

---

## 1. A guarda de rc é inútil aqui **por construção**

```bash
DEST_BARE=$(sed -n 's/^DEST: //p' <<<"$OUT")     # nao casou -> rc=0, valor vazio
```

`sed -n` sem casamento **sai 0**. Não há rc não-zero para `set -e` matar nem para `|| true`
engolir. As três variantes da nota irmã colapsam em uma:

| variante | não casa |
|---|---|
| sem guarda nenhuma | **rc=0 e `OK`** — aprovação vácua |
| `{ … \|\| true; }` | **rc=0 e `OK`** — idêntico, o `\|\| true` é decorativo |
| guarda de **não-vacuidade** | rc=1 **com `FAIL` nomeado** |

🔴 **Consequência para a triagem:** nesta família o teste de bolso da nota irmã é a **única**
pergunta que resta — *"o código depois da atribuição distingue vazio de valor legitimamente
medido?"*. Perguntar "o rc propaga?" é perguntar por algo que não existe.

**Medido nos dois cenários AC5 de `check-install-version-pin.sh`**, com o seam de dryrun
suprimindo a linha `DEST: ` (harness que reproduz o bloco AC5 byte-a-byte, `bash` explícito,
rc lido de arquivo, sem cano):

```
antes  seam quebrado -> rc=0  "OK   [install-version-pin/ac5-same-asset]"
depois seam quebrado -> rc=1  "FAIL [...]: o seam de dryrun nao emitiu basename utilizavel ..."
```

## 2. 🔴 O achado desta nota: guarde a **expressão comparada**, não a captura

A comparação do sítio **não** é sobre a captura. É sobre o basename:

```bash
if [ "${DEST_BARE##*/}" != "${DEST_PREFIXED##*/}" ]; then
```

`[ -z "$DEST_BARE" ]` **não** garante `${DEST_BARE##*/}` não-vazio. Medido:

| `DEST_BARE` | guarda na captura crua | guarda no basename | comparação |
|---|---|---|---|
| `""` | dispara | dispara | iguais |
| 🔴 `"/tmp/work/"` | **não dispara** | **dispara** | **iguais (dois vazios) → `OK`** |

Um `DEST` terminado em `/` tem captura **não-vazia** e basename **vazio** — a guarda copiada do
padrão do `URL` passaria batido e o cenário voltaria a emitir `OK` sobre comparação de dois vazios.
No `URL` o par não aparece porque ali a expressão guardada e a comparada são **a mesma**; aqui não
são, e a transferência do padrão por analogia é o que falha.

> **Regra que sai daqui:** a guarda de não-vacuidade incide sobre **o valor que o `if` lê**. Se o
> `if` lê uma expansão de parâmetro, materialize a expansão numa variável e guarde **ela**.

## 3. A forma perigosa é estreita: **duas capturas do mesmo produtor**

O que torna o vazio indistinguível não é o comando — é a **comparação entre duas capturas que
compartilham o produtor**. Um seam quebrado esvazia as duas de uma vez e elas concordam.

Comparar contra **literal não-vazio** (`!= "$EXPECTED_HASH"`, `== '"passed"'`,
`!= "CHUNK_COMPLETE $idx"`) é **auto-discriminante**: o vazio reprova alto. Não precisa de guarda.

Censo de `scripts/*.sh` + `.github/**` + `Makefile` pelo par captura↔comparação: **67 sítios
brutos**, dos quais **4 corrigidos** (os dois pares de `DEST` dos cenários AC5) e **63 isentos**,
em cinco classes com razão escrita — a maior delas (18) é "terminado em `wc`, que emite número
mesmo com entrada vazia", e a segunda (20) é "guarda `-z` explícita já existe na linha seguinte".

## 4. Isenção semântica que quase virou correção

`check-agent-namespace-union.sh:904,905` compara **duas capturas do mesmo `$dirc_go_out`** —
é a forma perigosa — e a comparação é `(( zulu_ln < alfa_ln ))`, onde operando vazio vale **0** e
`0 < 0` é falso, isto é, **passa**. O que o isenta é o laço **imediatamente acima**, que já reprova
(`fail`, que faz `exit 1`) se qualquer marcador estiver ausente. É a classe *"não-casamento
pré-excluído por checagem anterior que encerra o script"* — **semântica, não decidível por gate**.
Um gate linha-a-linha acusaria este sítio; a alegação tem de ficar escrita aqui.

## 5. Limites do varredor desta nota — escritos, porque ele erra

O varredor fixa a forma (atribuição + substituição) ∩ família de comandos que saem 0 sem produzir
saída ∩ uso do valor numa comparação em janela de 14 linhas. **Três cegueiras medidas:**

1. **Janela de 14 linhas** — `DEST_BARE` (linha 216) só é comparado na 233. O sítio só entrou no
   censo pelo **parceiro** (`DEST_PREFIXED`, linha 219). Par separado por >14 linhas **some**.
2. **`(( ))` sem `$`** — `(( zulu_ln < alfa_ln ))` não casa o padrão de comparação. Achado
   manualmente, não pelo varredor.
3. **Expansão de parâmetro na comparação** — a primeira passada do varredor **não** viu
   `"${DEST_BARE##*/}" != …`, que é exatamente o sítio que este ML existe para corrigir. Corrigido
   na segunda passada acrescentando `\$\{var(##|%%|#|%|:-|:=)` ao padrão. 🔴 **O instrumento errou
   primeiro contra o alvo conhecido** — mesma lição do §3 da nota irmã, uma camada acima.

Antes de confiar num varredor deste tipo: rode-o contra um sítio que você **já sabe** ser defeito.
Se ele não achar, o problema é o instrumento.
