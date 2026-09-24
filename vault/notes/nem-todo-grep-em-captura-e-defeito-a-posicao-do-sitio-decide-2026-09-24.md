# Nem todo `grep` dentro de `$( … )` é defeito — a **posição sintática do sítio** decide, e um filtro que só olha o corpo da substituição erra nas duas direções

> 2026-09-24 · ML-2G, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3` (macOS/ARM64), `grep` BSD, `actionlint 1.7.x`

🔴 **Coordenadas.** Os números de linha citados abaixo são **pré-correção** (os da enumeração do
ML-2E). A própria correção inseriu comentários acima de cada sítio e os deslocou — a tabela de
tradução está no fim da nota (§7). Use `grep -n '|| true; }'` nos cinco arquivos em vez de confiar
em qualquer dos dois conjuntos daqui a um mês; é o mesmo cuidado de §5.

O ML-2E enumerou **10 candidatos** de "`grep` em posição não-final de pipeline dentro de `$( … )`
sob `pipefail`, sem guarda". O ML-2G remediu e classificou um a um. O resultado é **5 defeituosos e
5 isentos** — e as duas listas *não* são sublistas da lista original: um sítio que estava fora
entrou, e a caracterização do sítio do workflow estava errada.

🔴 **A lição operacional:** o `grep` não é o que decide. Decide **onde a substituição está** e
**o que o código imediatamente seguinte faz com o valor vazio**.

---

## 1. Três classes de isenção **estrutural** — o `set -e` nem chega a disparar

Medidas em `bash 5.3`, não presumidas (`/tmp/p.sh` do ML-2G, três braços num só script):

| forma | rc do sítio | o `set -e` dispara? |
|---|---|---|
| `f "$(grep … \| head -5)"` — substituição em **posição de argumento** | descartado | **não** — o que conta é o rc de `f` |
| `rv=$(grep … \| sed …) \|\| true` — guarda **depois** da atribuição | 0 | **não** |
| corpo rodado por `bash -c "$SCRIPT"` sem `pipefail` | 1 | **não mata o pai**; `SHELLOPTS` não é exportado |

A segunda é a que mais confunde um filtro automático: o `|| true` **existe**, mas fora do corpo do
`$( … )`. Um filtro que procure `'|| true' in body` marca o sítio como desguarnecido.
`check-ci-workflow-pin-parity.sh:192` e `:211` caíram exatamente aí — e o comentário logo acima
deles *já explicava* a guarda.

A primeira é a menos intuitiva: `fail "rótulo" "$(grep -n … | head -5)"`
(`check-serve-api-file-security.sh:76`) **não pode** matar o script, porque o rc de uma substituição
em posição de argumento é descartado; só o rc do comando externo conta.

## 2. Uma isenção **semântica**: o não-casamento já foi excluído por uma checagem anterior

`check-agent-namespace-union.sh:900,901` fazem `grep -n -F -- "[alfa"` sobre uma saída que o laço
imediatamente anterior já validou com `grep -qF` para os mesmos três marcadores — e o `fail()` desse
arquivo **encerra com `exit 1`**. Não casar em `:900` é **impossível** quando o fluxo chega lá.

🔴 **Escreva a isenção na forma estreita, não "este sítio não pode morrer".** Sob `pipefail`,
`grep | head -1` tem um **segundo** caminho não-zero: `head` fecha o cano, o `grep` recebe `SIGPIPE`
e sai **141**. Aqui é inaplicável — a saída é de ~7 roadmaps, muito abaixo da capacidade do pipe, e
o `grep` termina de escrever antes do `head` sair —, mas a afirmação larga é falsificável com um
corpus grande o bastante. (Nos sítios **corrigidos** a questão some de graça: `|| true` engole 1 e
141 igualmente.)

## 3. O defeito, quando é defeito, tem sempre a mesma assinatura

Nos 5 defeituosos, a linha **imediatamente seguinte** é um `if [[ -z "$v" ]]` que emite diagnóstico
próprio para o caso vazio. Ou seja: **o autor já declarou que "não casar" é um resultado válido da
medição** — e o `pipefail` + `set -e` tornam esse ramo **inalcançável**, trocando o diagnóstico por
morte muda. É a mesma raiz da Wave 1 e do ML-2E, na terceira forma.

> **Teste de bolso:** a linha seguinte trata o vazio? Então falta `|| true`.
> Trata? — não, *existe*: se ela existe, o ramo está morto hoje.

### O sítio com **dois** greps letais, não um

`check-manifest-version-gate.sh:113`:

```bash
grep '^## \[' "$CHANGELOG" | grep -v '^\#\# \[Unreleased\]' | head -1 | sed …
```

Há **dois** estados de morte, e só o primeiro é óbvio:

1. nenhum `## [` no arquivo → `grep`#1 sai 1;
2. 🔴 **só `## [Unreleased]` presente** → `grep`#1 sai **0**, e o `grep -v` filtra tudo → sai 1.

O estado (2) é o estado normal de meio de ciclo de release. Guardar só o primeiro `grep` deixaria o
gate morrendo mudo exatamente na janela em que ele é mais consultado. **Os dois** levaram `|| true`,
e a falsificação tem **três** direções — o braço `SÓ-UNRELEASED` é o que discrimina se ambos foram
guardados.

## 4. O sítio do workflow: **posição final**, e o `pipefail` é irrelevante

`.github/workflows/check-annotations.yml:80` foi descrito pelo ML-2E como "`grep` em posição
não-final". **Não é** — o `grep` é o último elo:

```bash
CHECK_RUN_ID=$(echo "$CHECK_RUN_URL" | grep -oE '[0-9]+$')
```

Isso o torna **pior**, não melhor: em posição final o rc do `grep` **é** o rc da substituição, então
o `set -e` sozinho já mata — o `pipefail` não precisa participar. Duas medições, as duas escritas:

- o step **não** declara `shell:`, mas o corpo `run:` abre com `set -euo pipefail` na sua primeira
  linha (`:42`) — `pipefail` está ativo, ainda que dispensável aqui;
- o `while` que contém o sítio é o **elo final** de `gh api | jq -c '.' | while …`, logo roda em
  **subshell**: a morte interrompe o laço, os jobs restantes ficam **sem medição**, e o `::warning::`
  da linha seguinte nunca sai. É o tema desta REQ na íntegra — apuração que morre no meio sem
  diagnóstico.

## 5. Como remedir (o comando), e os dois defeitos do filtro

O filtro do ML-2E (varredura de `$( … )` com parênteses balanceados) continua certo no núcleo e
reproduz **73 brutos** hoje. Dois ajustes o tornam fiel:

- **`|| true` fora do corpo** — testar a linha inteira, não só o miolo de `$( … )`;
- **`pipefail in t` é heurística de arquivo, não de escopo** — foi ela que trouxe
  `check-gates-falsify.sh:2021` de volta à lista: o `pipefail` do arquivo não atravessa o
  `bash -c "$SIMPLE_REQ_FIELD_SCRIPT"` onde o sítio vive (o ML-2E já o tinha julgado isento em
  `:1970`, antes do deslocamento de linhas do próprio ML-2E).

🔴 **E o filtro exige `'|' in body`**, então a forma **irmã sem cano** — `v=$(grep … arquivo)`, que
mata pelo rc do `grep` sozinho, **sem precisar de `pipefail`** — é invisível para ele. Medidos
**5 sítios** vivos na árvore (`check-ci-workflow-pin-parity.sh:230` ·
`check-install-version-pin.sh:210,213,246,249`). Mesma causa, mesma REQ — nomeados para ML próprio,
não empurrados para REQ nova.

## 6. Consequência para o gate irmão (ML-2H)

Um discriminante que só procure `grep` dentro de `$( … )` sob `pipefail` nasce **ruidoso**. As
classes que ele **não** pode acusar, todas medidas aqui:

1. substituição em **posição de argumento** de um comando;
2. `v=$( … ) || true` — guarda depois do fecha-parênteses;
3. corpo em `bash -c`/`sh -c`, onde `pipefail` não atravessa a fronteira;
4. não-casamento pré-excluído por checagem anterior que **encerra** o script.

As classes 1–3 são **sintáticas** (decidíveis por um parser). A **4 é semântica** e não é decidível
— o caminho honesto é ela ser declarada no cabeçalho como não coberta, com uma alegação por sítio,
em vez de o gate fingir que a decide.

---

## 7. Coordenadas pré → pós correção (e a medição de fechamento)

| arquivo | pré (ML-2E) | pós (ML-2G) |
|---|---|---|
| `scripts/check-agent-namespace-union.sh` (corrigido) | `:632` | **`:636`** |
| `scripts/check-agent-namespace-union.sh` (isentos) | `:900,901` | **`:904,905`** |
| `scripts/check-manifest-version-gate.sh` | `:113` | **`:117`** |
| `scripts/check-parity-call-site-pins.sh` | `:261` | **`:264`** |
| `scripts/check-serve-address-parity.sh` | `:229` | **`:232`** |
| `.github/workflows/check-annotations.yml` | `:80` | **`:85`** |
| `scripts/check-ci-workflow-pin-parity.sh` (isentos) | `:192,211` | inalterados |
| `scripts/check-serve-api-file-security.sh` (isento) | `:76` | inalterado |

**Medição de fechamento**, com o mesmo filtro: **73 brutos** (inalterado — os comentários novos
citam `` `{ grep … || true; }` `` em crase, não abrem `$(`), e o conjunto filtrado cai de
**10 → 6**. Os 6 restantes são **exatamente** os isentos desta nota mais o sítio de
`check-gates-falsify.sh` fora do escopo de escrita. O balde de `grep` em posição **final** vai de
**1 → 0**.

🔴 **Gates irmãos que varrem o que foi editado** — rodados porque os comentários adicionados contêm
os próprios tokens que eles caçam (`pipefail`, `grep -oE`, `gh api | jq | while`), e porque um
comentário dentro de bloco `run:` é indistinguível de diretiva para um gate orientado a linha (é o
achado `"go install" em comentário YAML != run step` deste mesmo vault): `check-emitting-capture-fallback.sh`
(o gate do ML-1B, desta REQ) · `check-ci-workflow-pin-parity.sh` · `check-crlf-normalize-capture.sh` ·
`check-orphan-gates.sh` · `check-req-path-literals.sh` · `check-roadmap-barrier-contract.sh` —
**todos rc=0**. `check-falsify-shard-coverage.sh` exige artefatos baixados do CI e não roda local.
