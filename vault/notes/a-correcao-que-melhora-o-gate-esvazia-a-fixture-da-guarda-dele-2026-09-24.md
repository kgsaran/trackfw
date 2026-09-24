# A correção que **melhora** o gate esvazia a **única fixture** da guarda dele — e a guarda passa a examinar zero e reportar verde

> 2026-09-24 · ML-2L, REQ-2026-09-23 (a apuração do censo morre no shard limpo) ·
> medido em `bash 5.3.20` (macOS/ARM64) · gate: `scripts/check-unguarded-capture-rc.sh` ·
> cenário: `scripts/check-gates-falsify.sh` Cenário 199, braço **P**

## O que aconteceu, em três passos

1. O **ML-2H** criou o gate com uma tabela `ALLEGATIONS` de *bootstrap* (2 entradas:
   `check-agent-namespace-union.sh` `alfa_ln`/`zulu_ln`) e uma **guarda de obsolescência**: entrada
   que não casa nenhum sítio **reprova**. O braço **P** do Cenário 199 falsificava essa guarda
   rodando o gate contra uma árvore **sintética**, onde as 2 entradas reais não casam nada.
   🔴 **A tabela era a única fixture do braço P.**
2. O **ML-2K** migrou as 2 entradas para a forma **preferida** — o marcador inline
   `# unguarded-capture-rc-allowed:` no próprio sítio. Correto: é o que o cabeçalho do gate
   recomenda, e a tabela só existia porque o ML-2H estava proibido de editar arquivo de produto.
3. Efeito: a tabela **esvaziou**. O laço da guarda passou a iterar **zero** entradas e a cair no
   `echo` final de sucesso. E `assert_fails_with "alegacao obsoleta"` do braço P passou a receber
   **rc=0**.

```
UNGUARDED_RC_GATE_FORCE_ALLEGATION_GUARD=1 bash scripts/check-unguarded-capture-rc.sh   -> rc=0
=== guarda de obsolescencia das alegacoes (classe 6) ===
(nada embaixo)
```

🔴 É **literalmente a classe de defeito que o cabeçalho do próprio gate nomeia como razão de
existir** — guarda que examina zero e reporta sucesso —, reproduzida por acidente, pela correção que
tornava o gate melhor.

## A lição transferível, que é maior que este gate

> 🔴 **Uma guarda cuja fixture é o dado real de produção é falsificável por ACIDENTE: ela só tem
> teste enquanto sobrar dado. No dia em que o dado acabar — e acabar costuma ser *progresso* — ela
> vira vácua em silêncio, e nada avisa.**

O corolário operacional é o que o arquiteto escolheu aqui (opção 1 de duas): **fixture injetável**,
não "mantenha a tabela não-vazia". Manter dado real vivo só para o teste existir é custo zero hoje e
armadilha amanhã — e este roadmap já mediu **três vezes** que defesa vácua é pior que ausência de
defesa (quem vê verde não olha).

Segundo corolário, este sobre revisão: **ao esvaziar uma estrutura, pergunte quem a usava como
fixture.** O ML-2K fez a coisa certa no produto e **detectou** o efeito — está escrito no relatório
dele —, mas o conserto caía fora da sua fronteira de escrita. A detecção só aconteceu porque ele
rodou o gate; um `git diff` limpo não mostraria nada.

## O conserto: três estados, não dois

O erro fácil aqui é mandar a guarda **reprovar quando não há o que verificar**. Isso a tornaria
inutilizável: toda árvore sintética do Cenário 199, e todo repositório consumidor sem sítio de
classe 6, reprovariam sem defeito nenhum. A distinção medida:

| estado | condição | veredito |
|---|---|---|
| **exercitada** | ≥1 alegação examinada (tabela, fixture injetada, ou marcador inline) | verifica cada uma; obsoleta → `FAIL` |
| **nada a verificar** | zero alegações **e** zero isenções de classe 6 concedidas | `NOTA nada a verificar`, rc **inalterado** — não há afirmação que possa envelhecer |
| 🔴 **não fui exercitada** | zero alegações examinadas **mas** isenção de classe 6 **concedida** | `FAIL guarda de obsolescencia nao foi exercitada` |

O terceiro estado é a **testemunha de medição** do ML-2K aplicada aqui: *a existência de algo a
medir é pré-condição, não resultado.* Ele é exatamente o estado que o ML-2K criou — 2 sítios
isentados por alegação, zero alegações examinadas — e a contabilidade que o detecta é o que faltava.

## O que deu à guarda algo a examinar na árvore real: **obsolescência do marcador inline**

A forma inline parece imune a envelhecer, porque mora colada ao sítio. **Não é.** Se alguém corrige
o sítio para `v=$( { grep … || true; } )` e esquece o marcador, ele passa a afirmar sobre um sítio
que **não precisa dele** — comentário travestido de afirmação, o mesmo defeito da tabela. O gate
agora censa todo marcador e reprova o que **não isentou nenhum sítio na classe 6**
(`alegacao inline obsoleta`). Na árvore real isso dá 2 alegações examinadas — a guarda deixa de ser
vácua **sem fixture nenhuma**.

🔴 **Armadilha ao censar marcadores:** o padrão frouxo `unguarded-capture-rc-allowed:` em qualquer
posição da linha casa **prosa e código de teste**, e o falso marcador nasce "obsoleto":

```
scripts/check-gates-falsify.sh:7725:mk199 arm-l 'set -euo pipefail\n# unguarded-capture-rc-allowed: …'
scripts/check-gates-falsify.sh:7626:#   L unguarded-rc/alleged-inline  … `# unguarded-capture-rc-allowed:`
```

A âncora `^[[:space:]]*#[[:space:]]*unguarded-capture-rc-allowed:` exclui as duas, e é a **mesma
regex** usada pelo censo **e** pelo consumo. Se divergissem, um sítio poderia ser isentado por
marcador que o censo nunca registrou — e a guarda de "não fui exercitada" reprovaria por **razão
falsa**. (É a lição §4-bis da nota irmã, do outro lado: ali o ônus era do *texto entre* marcador e
sítio; aqui é de **uma** definição para dois consumidores.)

## Forma da fixture: **arquivo**, não variável com separador

`UNGUARDED_RC_GATE_ALLEGATIONS_FILE` aponta para um arquivo com uma entrada por linha, no mesmo
formato da tabela (`<basename>|<var>|<razão>`). Não é variável de ambiente com entradas separadas
por newline: a razão é **texto livre** num script cheio de `printf`, e esta árvore já pagou por
casamento vácuo com newline embutida (`bash-grep-F-embedded-newline-vacuous-match-2026-08-16`). E a
injeção é sempre **anunciada** na saída — fixture silenciosa que muda veredito é a classe de defeito
desta própria REQ.

## Como cada peça foi falsificada

| peça | como | resultado |
|---|---|---|
| braço **P** (tabela/fixture obsoleta) | fixture injetada com `(check-synth.sh, variavel_que_nao_existe)` | `rc=1`, `alegacao obsoleta` — e **rc=0 sem a fixture**, que é a prova de que a fixture é o que o faz falsificar |
| braço **S** (contra-braço de P) | fixture que **casa** o sítio | `rc=0`, `alegacao viva` — P reprova por **obsolescência**, não por "injetar reprova" |
| braço **R** (marcador órfão) | marcador acima de sítio já isento por classe anterior | `rc=1`, `alegacao inline obsoleta` |
| braço **T** (nada a verificar) | mesma árvore de P, **sem** fixture | `rc=0`, `NOTA nada a verificar` |
| **"não fui exercitada"** | 🔴 **mutação**: cópia do gate com o censo de marcadores neutralizado, rodada na árvore real | `rc=1`, `2 isencao(oes) … mas ZERO alegacoes foram examinadas` — reproduz o estado do ML-2K |

O último não tem braço permanente **de propósito**: com a contabilidade íntegra ele é inalcançável
(toda isenção registra alegação). É defesa contra **regressão da contabilidade** — e o §5 da nota
irmã mostra que ela já quebrou uma vez, quando `allegation_reason` era chamada dentro de `$( )` e a
contagem morria na fronteira do subshell. Por isso a contabilidade nova (censo de marcadores) mora
em **laço direto** no corpo de `scan_file`: sem `$( )`, sem estágio de cano.

## Ver também

- `cinco-defeitos-do-instrumento-ao-transformar-as-6-classes-em-gate-2026-09-24.md` — nota irmã;
  §5 é a primeira vez que o subshell mordeu esta mesma contabilidade, e §7 é a contraprova das
  classes 4/5
- `bash-grep-F-embedded-newline-vacuous-match-2026-08-16.md` — por que a fixture é arquivo
- `carga-de-cpu-vem-da-suite-de-falsificacao-vezes-agentes-paralelos-2026-09-22.md` — antes de
  pensar em rodar a suíte inteira para conferir o piso
