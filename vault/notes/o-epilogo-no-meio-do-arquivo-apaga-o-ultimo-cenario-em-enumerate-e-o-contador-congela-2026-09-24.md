---
title: O epílogo no meio do arquivo apaga o último cenário em modo enumerate — e o contador de sucesso congela junto
date: 2026-09-24
agente: artemis-tf
req: ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro (ML-4A)
---

# O epílogo no meio do arquivo apaga o último cenário em modo enumerate

## O fato

Em `scripts/check-gates-falsify.sh` (pré-ML-4A) o **bloco de saída do modo de enumeração** —
`if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then … exit 1; fi` — ficava na **linha 8109**, e o
**Cenário 200** começava na **linha 8124**. 15 linhas de distância, na direção errada.

Consequência, medida:

- O censo de Windows roda **justamente** em `TRACKFW_FALSIFY_ENUMERATE=1` e tem reprovações **por
  construção** (é um censo, não um gate).
- Qualquer reprovação no mesmo chunk faz o chunk executar `exit 1` **antes** do Cenário 200.
- Os **9 rótulos `interp-path/*`** nunca são emitidos.
- O driver os relata como *"rótulo esperado AUSENTE"* — 🔴 **que lê como chunk morto**, e não como
  *"a execução parou aqui, de propósito, 15 linhas antes"*. O diagnóstico aponta para o lugar errado.

Reprodução direta, nos chunks materializados com N=8 (falha injetada com
`printf 'x\n' >> "$FALSIFY_ENUM_TALLY"` antes do primeiro bloco):

| fonte | posição relativa | rótulos `interp-path/*` no log | RC |
|---|---|---|---|
| pré-fix (`chunk_7.sh`) | saída :1826 · Cenário 200 :1841 | **0** | 1 |
| pós-fix (`chunk_4.sh`) | Cenário 200 :2378 · epílogo :2500 | **9** | 1 |

🔴 **`CHUNK_COMPLETE` ausente nos dois é CORRETO, não sintoma.** O `gen-falsify-chunks.py` anexa a
cada chunk a mesma checagem de tally e sai `1` **antes** do sentinela, de propósito — está escrito no
comentário dele. Não perca tempo caçando o sentinela num braço de enumerate-com-reprovação.

## O segundo defeito, mesma causa, que ninguém tinha visto

O `falsify_success_n=$(wc -l < "$FALSIFY_SUCCESS_TALLY")` também morava **antes** do Cenário 200
(linha 8121), e o seu **único consumidor** — a guarda de vacuidade e o `echo "Falsification checks
passed (N scenarios)"` — mora **depois** dele.

Ou seja: o número que o script imprime e compara contra `FALSIFY_SUCCESS_FLOOR` era um **retrato
tirado antes do último cenário**. Os 9 `OK` do Cenário 200 entravam no arquivo de tally e **nunca**
entravam na conta. É por isso que o comentário do piso registra "249" numa árvore que emite mais.

**Regra transferível:** num script que conta a própria execução, **a leitura do contador é epílogo**.
Se ela não for a última coisa antes do consumidor, ela mede uma árvore que não existe.

## Por que o arquivo permitia isso

Razão **estrutural**, não descuido isolado: o `gen-falsify-chunks.py` particiona o arquivo em
segmentos cujas fronteiras são os cabeçalhos `# Cenário N — …`. O **último** segmento vai do último
cabeçalho até o EOF. Logo:

- Tudo que estiver **entre** o penúltimo cabeçalho e o último **pertence ao segmento do penúltimo
  cenário** — foi onde o epílogo caiu.
- Não havia **nenhuma marca** separando "cenários" de "epílogo". Quem acrescenta um cenário novo faz
  o que é natural — **append no fim do arquivo** — e aterrissa **depois** do epílogo, sem nenhum
  sinal.
- Pior: `falsify_success_n` atribuído no segmento 199 e lido no segmento 200 **fundia** os dois num
  bloco indivisível pelo fechamento de dependência por variável. Os dois sempre caíam no mesmo chunk,
  então o `exit 1` sempre chegava primeiro. Não era intermitente: era **determinístico**.

## A correção — e por que ela impede a reincidência

Mover as 15 linhas resolve **hoje**. O que impede **amanhã** é a marca + a guarda:

1. `# FALSIFY-EPILOGUE-BEGIN` em `check-gates-falsify.sh`, abrindo o epílogo.
2. `check_epilogue_after_all_scenarios()` em `gen-falsify-chunks.py`, chamada em `main()` antes de
   `build_segments`. **Recusa gerar chunks** em quatro direções:
   - cabeçalho de cenário **depois** da marca → nomeia a linha;
   - marca **ausente** → fail-closed;
   - marca **duplicada** → fail-closed;
   - marca presente mas o **bloco de saída não está depois dela** → *"marca sem o bloco que ela
     delimita não prova nada"*.

🔴 **Por que a guarda mora no gerador e não num gate novo:** o gerador já é o **único** lugar que
conhece a gramática de fronteira (`HDR_PAT`), e essa gramática já quebrou **3×** por ser
reimplementada em outro lugar. Um gate em bash seria o quarto sítio da mesma regex. E o gerador está
no **caminho obrigatório**: `make quality`/`make parity` (via `run-gates-falsify-parallel.sh`),
`quality.yml` e `windows-census.yml` (via `run-gates-falsify-shard.sh`) e
`check-falsify-shard-coverage.sh` **todos** passam por ele. Nenhum deles invoca
`check-gates-falsify.sh` direto.

**Limite declarado:** a guarda **não** cobre quem rodar `bash scripts/check-gates-falsify.sh` na mão.
Nesse caminho não há chunk, então o defeito também não existe — o script inteiro roda num processo só
e o `exit 1` do enumerate é o fim legítimo. O dano é exclusivo do caminho particionado.

## Efeito no piso (`FALSIFY_SUCCESS_FLOOR`)

O piso é `-lt`, isto é, um **mínimo**. A correção só pode fazer a contagem **subir** (o retrato deixa
de ser tirado antes do Cenário 200). Por isso o piso **não precisa mudar** — mas a folga muda, e o
número que o comentário do preâmbulo registra fica **desatualizado por construção**.

🔴 **Não some 9.** Este arquivo já carrega duas cicatrizes de reconciliação aritmética inflada. Meça.

## Armadilha de ambiente ao reproduzir

Chunk materializado **exige** `TRACKFW_ROOT_DIR`. Sem ela o chunk deriva `ROOT_DIR` do próprio
caminho em `/tmp` e morre na **linha 32** com
`.../scratchpad/scripts/lib-crlf-normalize.sh: No such file or directory` — **1 linha de log**, e um
`grep` de rótulo devolve **0**, que é indistinguível de *"os rótulos sumiram"*. Perdi um par A/B
inteiro assim: os dois braços deram `0`. O driver exporta a variável (`run-gates-falsify-parallel.sh`
linha 145); a invocação manual, não.
