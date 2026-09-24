# O `|| true` que vira **aprovação vácua** — na forma sem cano, o que decide é o que vem **depois** da atribuição, não o que vem na linha seguinte

> 2026-09-24 · ML-2I, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3` (macOS/ARM64), `grep` BSD

A nota irmã (`nem-todo-grep-em-captura-e-defeito-a-posicao-do-sitio-decide-2026-09-24.md`) fechou a
forma **com cano**. Esta fecha a forma **sem cano** — `v=$(cmd …)`, que mata pelo rc do comando
**sozinho**, sem depender de `pipefail` — e corrige um teste de bolso daquela nota que **inverte**
em um dos sítios.

---

## 1. O teste de bolso da nota irmã inverte quando a medição é uma **comparação**

A nota irmã diz: *"a linha seguinte trata o vazio? então falta guarda."* Nos 4 sítios de
`check-install-version-pin.sh` a linha seguinte **compara duas capturas**:

```bash
URL_BARE=$(grep '^URL: ' <<<"$OUT")
URL_PREFIXED=$(grep '^URL: ' <<<"$OUT")     # outra execução de install.sh
if [ "$URL_BARE" != "$URL_PREFIXED" ]; then … exit 1; fi
echo "OK   [install-version-pin/ac5-same-asset]"
```

Se o seam de dryrun parar de emitir `URL: `, as duas capturas ficam **vazias**, comparam **iguais**,
e o cenário emite **`OK`** sobre medição nenhuma. Medido, três variantes no mesmo harness:

| variante | não casa | casa |
|---|---|---|
| **antes** (sem guarda) | rc=1, **morte muda** — zero saída | `OK` |
| 🔴 **só a guarda de rc** | **rc=0 e `OK`** — aprovação vácua | `OK` |
| **depois** (guarda de rc **+** não-vacuidade) | rc=1 **com rótulo `FAIL [ac5-same-asset]`** | `OK`, valor byte-idêntico (`cmp` rc=0) |

🔴 **A linha do meio é o achado.** Trocar a morte muda pela guarda de rc **isolada** não corrige o
defeito — troca um falso-negativo barulhento por um **falso-positivo silencioso**, que é pior: o
gate passa a afirmar que mediu. É a mesma raiz do `|| echo 0` da Wave 1, com outra roupa.

> **Teste de bolso corrigido, para a forma sem cano:**
> *"o código **depois** da atribuição distingue **vazio** de valor legitimamente medido?"*
> Se distingue → guarda de rc basta. Se **não** distingue → guarda de rc **mais** guarda de
> não-vacuidade, ou o sítio fica como está e a razão vai escrita.

## 2. Cinco classes de isenção **estrutural** — duas novas, medidas aqui

Somando às três da nota irmã:

| # | forma | rc propaga? | medido |
|---|---|---|---|
| 1 | substituição em posição de **argumento** | não | nota irmã |
| 2 | `v=$( … ) \|\| true` — guarda fora do parêntese | não | nota irmã |
| 3 | corpo em `bash -c`/`sh -c` sem `pipefail` | não atravessa | nota irmã |
| 4 | 🔴 **arquivo sem `set -e`** (`set -uo pipefail` só) | **não mata** | aqui |
| 5 | 🔴 **`local v=$(cmd)` na mesma linha** — o rc é o do `local`, que é **0** | não | aqui |

A 5 é a razão do SC2155 e é **decidível sintaticamente** — mas só quando `local` e a atribuição
estão na **mesma linha**. Em `check-orphan-gates.sh:86,102` o `local` está em linha **separada**,
logo **não** mascara; ali quem isenta é a classe 4.

E a classe 4 tem um **segundo motivo independente** nesses dois sítios: o comando é `find`, que sai
não-zero em **erro de acesso**, não em "não encontrou nada" — não é sequer o mesmo mecanismo.

## 3. O varredor: por que o do ML-2E/2G não podia achar isto, e o que este acha a mais

O filtro anterior exigia `'|' in body` — a forma sem cano era **invisível por construção**. O
varredor desta nota fixa a **forma** (atribuição, substituição sem cano, sem `||` na linha) e abre
a **família** de comandos cujo rc não-zero é resultado legítimo da medição: `grep`, `git rev-parse`,
`command -v`, `test`, `jq -e`, `diff`, `cmp`, `find`, `which`, `type`.

Resultado sobre `scripts/*.sh` + `.github/**` + `Makefile`: **13 brutos** contra os **5** nomeados
pelo ML-2G — e a diferença é quase toda da **família**, não da forma: dos 8 novos, **4** são
`command -v` (`windows-census.yml:97,98` · `check-gates-falsify.sh:208` ·
`check-install-version-pin.sh:310`), **3** são `find` (`check-orphan-gates.sh:86,102` ·
`check-raw-read-ban.sh:96`) e **1** é `grep` — este último, o falso positivo de §3 abaixo. Nenhum
`grep` **genuíno** novo apareceu. 🔴 **O token estava certo; a família é que estava estreita.**

🔴 **Um dos 13 é falso positivo do varredor** (`quality.yml:1255`): a substituição é **multi-linha**
e o leitor linha-a-linha não enxerga o `|| true` que está três linhas abaixo. Antes de confiar num
varredor linha-a-linha, cheque as continuações: `grep -rn '=\$($' scripts/ .github/ Makefile` —
aqui deu **vazio**, então o falso **negativo** correspondente não existe nesta árvore.

## 4. `command -v` é isenção **semântica** — e o `|| true` ali é ativamente nocivo

Quatro dos seis sítios de `command -v` interpolam o resultado num **shim/stub** gerado:

```bash
REAL_UNAME=$(command -v uname)
cat > "$UNAME_STUB_BIN/uname" <<HEREDOC
  *) "${REAL_UNAME}" "$@" ;;
HEREDOC
```

Com guarda de rc, `uname` ausente produz `REAL_UNAME` vazio e o stub nasce com uma linha
`*) "" "$@"` — um **instrumento quebrado que ainda parece funcionar**. Sem guarda, o script morre
na linha do `command -v`, que é o comportamento correto: um sistema sem `uname`/`git`/`python3` não
é "medição que não casou", é ambiente inviável. **Isenção escrita, não omissão.**

## 5. Placar

**13 brutos = 5 corrigidos · 7 isentos com razão escrita · 1 falso positivo do varredor.** Dos 7
isentos, **3** vivem em arquivos fora do escopo de escrita deste ML
(`check-gates-falsify.sh:208` · `windows-census.yml:97,98`) — reportados com veredito, não tocados.
Coordenadas pós-correção: `check-ci-workflow-pin-parity.sh:235` ·
`check-install-version-pin.sh:215,218,259,262`. Use `grep -n '|| true; }'` em vez de confiar nos
números daqui a um mês — mesmo cuidado de §7 da nota irmã.
