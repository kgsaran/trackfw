# Cinco defeitos do **instrumento** ao transformar as 6 classes de isenção num gate — e quatro deles falham para o lado do **falso negativo**

> 2026-09-24 · ML-2H, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3.20` (macOS/ARM64) · gate: `scripts/check-unguarded-capture-rc.sh`

As duas notas irmãs
(`nem-todo-grep-em-captura-e-defeito-a-posicao-do-sitio-decide-2026-09-24.md` e
`o-ou-true-que-vira-aprovacao-vacua-...-2026-09-24.md`) mediram **o que** o gate do ML-2H tinha de
decidir. Esta registra **o que quebrou ao decidir** — cinco defeitos do instrumento, todos medidos,
**quatro deles silenciosos**.

🔴 **Por que isso importa mais que o de costume:** um gate que erra para o lado do falso **positivo**
reclama e alguém olha. Um que erra para o lado do falso **negativo** fica verde e ninguém olha
nunca. Quatro dos cinco abaixo são dessa segunda espécie, e dois deles produziam **cobertura zero**
sobre a classe que o gate existe para pegar.

---

## 1. 🔴 Contagem de parênteses **não ciente de aspas** engole a substituição inteira, em silêncio

A nota irmã manda usar varredura por **parênteses balanceados** em vez de leitor linha-a-linha. Ela
está certa, e é insuficiente: um parêntese **literal dentro do padrão do `grep`** desequilibra o
contador.

Medido em `.github/workflows/quality.yml:1254`:

```bash
GO_BYPASS=$(grep -n "os\.Symlink(" \
    internal/generators/update_test.go \
  | grep -v 'err := os\.Symlink(target, link)' \
  | grep -v 't\.Fatalf("os\.Symlink' || true)
```

O contador ingênuo entra em `(`, encontra o `(` de `"os\.Symlink("`, **nunca volta a zero**, varre
até o teto de linhas e desiste. O sítio **não aparecia nem como candidato** — nem acusado, nem
isentado: **invisível**.

**Correção:** o contador rastreia aspas e **não conta parênteses dentro delas**, com barra invertida
escapando dentro de aspas duplas. O estado de citação começa **zerado** no `(` de abertura — um
`$( … )` abre contexto de citação próprio, e é isso que faz `f "$(grep …)"` ser varrido corretamente
mesmo estando dentro de aspas duplas no texto externo.

**Antes → depois na árvore real:** `quality.yml:1255` passa de **ausente** a
`exempt/guarded-inside-parens`.

## 2. 🔴 O ERE de `pipefail` que não casa a forma mais comum da árvore

```
set[[:space:]]+-o[[:space:]]+pipefail          # exige `-o` ISOLADO
```

Não casa `set -euo pipefail` — onde o `o` vem **empacotado** no pacote de flags, e que é a forma de
**todo** script desta árvore. Efeito medido: `pipefail` lido como **desligado** em toda parte, e a
classe 3 (elo não-final sem `pipefail`) isentava **13 sítios**, entre eles os dois de
`check-agent-namespace-union.sh` que são a razão de a classe 6 existir. **O gate nascia sem cobrir a
forma com cano.**

Forma correta: `-[A-Za-z]*o[[:space:]]+pipefail`. Com ela, a classe 3 cai de **13 → 3** e os sítios
certos voltam a ser avaliados.

> Mesmo cuidado vale para `errexit`: `-[A-Za-z]*e` casa `-e`, `-eu`, `-euo`.

## 3. `errexit`/`pipefail` são estado **sequencial**, não propriedade do arquivo

`grep -q 'set -e' arquivo` é a heurística óbvia e está errada nas duas pontas:

- linhas **antes** do `set -e` não estão sob errexit;
- 🔴 `check-install-version-pin.sh` faz `set +e` em `:465` e `set -e` em `:473` — no intervalo,
  errexit está **desligado**, e um gate que OR-eia o arquivo inteiro acusa ali por razão falsa.

A opção efetiva é calculada **por linha**, caminhando em ordem, com estado corrente **por escopo**
(arquivo · string entre aspas simples rodada por `bash -c` · heredoc · bloco `run:` de workflow).

**Sobre o bloco `run:`:** o default do GitHub Actions é `bash -e {0}` — **errexit sim, pipefail
não**; com `shell: bash` explícito é `bash -eo pipefail {0}` — **os dois**. Tratar o `run:` como
"arquivo sem `set -e`" isentaria o sítio real do defeito desta REQ.

## 4. `local s="$1" n=${#s}` aborta sob `set -u` — o builtin declara **todos** os nomes antes de atribuir

```
$ bash -c 'set -uo pipefail; f(){ local s="$1" out="" i=0 n=${#s}; echo "n=$n"; }; f hello'
bash: line 1: s: unbound variable
```

`local` cria `s` **unset** junto com todos os outros nomes e **só então** executa as atribuições,
então `${#s}` na mesma instrução vê `s` inexistente. Sob `set -u`, aborta. Separe em instruções.

Sintoma enganoso: a função morria, a varredura devolvia **0 candidatos**, e a guarda de vacuidade
com piso 0 (o modo de calibração) reportava **`OK   vacuidade: 0 >= 0`**. Gate verde examinando
nada — exatamente o defeito da REQ anterior, reproduzido por acidente.

## 4-bis. A isenção por "palavra-chave no prefixo" é larga demais — e o quinto silencioso

A classe 2 inclui a substituição usada como **condição** (`if v=$( … ); then`), onde o `if` consome
o rc. O teste óbvio — *"o prefixo contém `if`/`while`/`until`?"* — isenta também isto:

```bash
if [ -n "$x" ]; then v=$(grep PAT f.txt); fi     # o rc PROPAGA: a atribuição vem DEPOIS do `; then`
```

A palavra-chave está no prefixo, mas não governa a atribuição. **Medido na árvore: 0 sítios desta
forma hoje** — logo a correção é preventiva, não corretiva:

```bash
grep -rnE '(^|[[:space:]])(if|while|until)[[:space:]].*(;[[:space:]]*(then|do)|&&|\|\|)[[:space:]]*[A-Za-z_][A-Za-z0-9_]*=\$\(' \
  scripts/*.sh .github/workflows/*.yml | grep -vc check-unguarded-capture-rc.sh     # -> 0
```

O teste passa a exigir que **nenhum separador estrutural** (`; then` · `; do` · `&&` · `||`) se
interponha entre a palavra-chave e o `NAME=`. Falsificado nas duas direções (braços **Q** e **H** do
Cenário 199): `if [ -n "$x" ]; then v=$(grep …)` **reprova**; `if v=$(grep …); then` **passa**.

🔴 **A lição geral**, que vale para os cinco: numa isenção, o ônus é do *texto entre* o marcador e o
sítio. "Contém a palavra" é quase sempre largo demais, e largo demais numa **isenção** significa
**falso negativo** — o modo de erro que ninguém vai olhar.

---

## 5. O `$( )` que engole o efeito colateral — a raiz desta REQ mordendo o próprio gate

A tabela de alegações da classe 6 tem **guarda de obsolescência**: entrada que não casa nenhum sítio
**reprova**. Ela nascia sempre vermelha, porque a contagem era incrementada dentro de uma função
chamada assim:

```bash
reason="$(allegation_reason "$fname" "$varname")"     # SUBSHELL — a contagem morre no retorno
```

É a **mesma raiz desta REQ** (efeito perdido na fronteira do subshell), numa terceira roupa.
Correção: devolver a razão em **global** e usar o rc como sinal, sem cano e sem `$( )`.

> Regra de bolso que sai daqui: **função que tem efeito colateral em global não pode ser chamada
> dentro de `$( )`.** Se ela precisa devolver texto *e* contar, devolva o texto por global também.

---

## 6. Por que a família de **candidatos** é mais larga que a de **violação** — e não é frouxidão

O gate só **acusa** `grep`/`egrep`/`fgrep`. Mas **classifica e imprime** `command -v`, `find`,
`git rev-parse`, `jq -e`, `diff`, `cmp`, `which`, `type`. Dois motivos, o segundo não óbvio:

1. o piso de não-vacuidade não pode desabar quando os sítios forem corrigidos — a forma correta
   `{ grep … || true; }` continua **candidata**, só deixa de violar;
2. 🔴 **sem a família larga, as classes 4 e 5 ficariam sem testemunha impressa na árvore real.**
   `check-orphan-gates.sh:86,102` usa `find` — a família estreita o filtraria **antes** de a classe 4
   ser sequer avaliada, e nomeá-lo como testemunha da classe 4 seria veredito certo por razão errada.

**Por que não acusar os não-grep:** medido no ML-2I — `command -v` com rc não-zero significa
**ambiente inviável**, não "não casou" (e `|| true` ali fabrica um shim quebrado que ainda parece
funcionar); `find` sai não-zero em **erro de acesso**, não em "não encontrou nada". Acusá-los daria
falso positivo em **7 sítios já medidos e isentados**.

## 7. Classe 4 ≠ classe 5, e a árvore só tem testemunha de uma delas

| classe | testemunha real | como confirmei |
|---|---|---|
| 5 — `local v=$(cmd)` na **mesma** linha | 🔴 **nenhuma** | `grep -rnE 'local[[:space:]]+[A-Za-z_][A-Za-z0-9_]*=\$\(' --include='*.sh' . \| grep -vc '\$(('` → **0** |
| 4 — escopo sem `set -e` | `check-orphan-gates.sh:86,102` · `check-raw-read-ban.sh:96` | os três em arquivos `set -uo pipefail` |

Em `check-orphan-gates.sh` o `local` está em linha **separada** — ele **não** mascara nada. Medido:

```
local v=$(grep ZZZ f.txt)    sob set -e  -> rc=0, ALIVE
local v; v=$(grep ZZZ f.txt) sob set -e  -> rc=1, morre
```

Por isso o Cenário 199 tem o braço **F** (`local` em linha separada **deve reprovar**) ao lado do
**K** (mesma linha, isento): sem a contraprova, a classe 5 vira desculpa para isentar a classe 4.

---

## 8. Placar de fechamento

`bash scripts/check-unguarded-capture-rc.sh` na árvore real, 2026-09-24:

```
Candidatos: 108   Violações: 0   Arquivos varridos: 69   Piso: 60
  class1-argument-position 36 · guarded-inside-parens 47 · semantic-family-not-grep 14
  class2-guard-outside-parens 3 · class3-no-pipefail-in-scope 3 · class4-no-errexit 3
  class6-alleged-table 2
```

Segundo caminho, independente do gate (**109**, contra 106 medidos pelo gate antes de o Cenário 199
entrar — a diferença é de construção: o `grep` linha-a-linha conta a linha, o gate conta a
substituição):

```bash
grep -rnE '\$\(' scripts/*.sh .github/workflows/*.yml \
  | grep -v '/check-unguarded-capture-rc.sh:' | grep -vE ':[0-9]+:[[:space:]]*#' \
  | grep -E '(grep|egrep|fgrep|command -v|find |git rev-parse|jq -e|diff |cmp |which |type )' | wc -l
```

**Exatamente 2 sítios** chegam ao estágio de violação na árvore real:
`check-agent-namespace-union.sh` `alfa_ln` e `zulu_ln` — a previsão da nota irmã, confirmada. Os
dois são **classe 6 (semântica, não decidível)** e vivem como alegação com razão na forma **estreita**
(o laço anterior pré-valida **e** a saída é pequena demais para o `head` produzir `SIGPIPE 141`).

🔴 **A alegação está na tabela do próprio gate, não como marcador inline no sítio** — que é a forma
preferida. Não por escolha técnica: o ML-2H proibia editar outro `check-*.sh` de produto. Quem
retomar isto deve mover as duas para `# unguarded-capture-rc-allowed:` acima das linhas e apagar as
entradas de `ALLEGATIONS`; a guarda de obsolescência reprova se a tabela ficar para trás.
