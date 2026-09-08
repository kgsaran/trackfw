# `return 1` e contador em variável de shell reintroduzem o próprio `exit 1` que o modo de enumeração existe para evitar

**Contexto:** ML-2B, `ROADMAP-2026-09-07-gates-rodam-no-windows-resolucao-de-interpretador-e-binario.md`
— modo `TRACKFW_FALSIFY_ENUMERATE=1` em `scripts/check-gates-falsify.sh` (continua após reprovação em
vez de abortar no primeiro `exit 1`). Duas falsificações desta sessão pegaram dois defeitos
**estruturais** (não de um cenário específico) na primeira versão da implementação — nenhum dos dois
apareceu num run limpo, só sob sabotagem deliberada.

## Defeito 1 — `return 1` num helper chamado nu, sob `set -euo pipefail`, é `exit 1` disfarçado

Os ~15 helpers reutilizáveis (`assert_fails_with` e as 9 funções irmãs) são quase sempre chamados como
comando NU de topo de script (`assert_fails_with "label" ... `, sem `if`/`&&`/`||` em volta). Sob
`set -e`, um comando nu que retorna `!= 0` aborta o shell **imediatamente** — não importa se o "comando"
é um binário externo ou uma função bash com `return 1`.

A primeira versão do ML-2B fazia, dentro de cada helper, no ramo habilitado:
```bash
FALSIFY_ENUM_FAILURES=$((FALSIFY_ENUM_FAILURES + 1))
return 1
```
Isso conta corretamente — e então `set -e` mata o processo no call site, **antes** do próximo cenário
rodar. Resultado observado (sabotagem do Cenário 1, `static-assets/byte-drift`, dentro de
`assert_fails_with`): o chunk morre depois de emitir só o `FAIL` daquele cenário — 0 cenários depois
dele, exatamente como no modo desligado. O modo de enumeração não enumerava nada.

**Correção:** o valor de retorno do HELPER em si tem que ser `0` no ramo habilitado — a contagem de
falha já foi feita antes do `return`; o `return 0` só existe para não disparar `set -e` no call site.
```bash
falsify_count_failure   # grava a falha
[[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]] && return 0
exit 1
```
Generalização: **qualquer refatoração de "abortar" para "sinalizar e continuar" sob `set -e`, quando o
sinalizador é uma função bash chamada nua, precisa terminar em `return 0`** — `return 1`/`return $N`
(N≠0) delega a decisão de abortar de volta para o `set -e` do chamador, silenciosamente.

## Defeito 2 — contador em VARIÁVEL DE SHELL não sobrevive subshell; a contagem tem que ser um arquivo

Boa parte dos ~199 pontos de reprovação deste script roda dentro de `( ... )`, `$( ... )` ou como
estágio de pipeline — subshell, em bash. `FALSIFY_ENUM_FAILURES=$((FALSIFY_ENUM_FAILURES + 1))` dentro
de uma subshell muta a CÓPIA da subshell; a atribuição desaparece quando ela termina. O processo pai
(ou o processo do chunk inteiro, no driver paralelo) nunca vê o incremento — a checagem final encontra
`FALSIFY_ENUM_FAILURES=0`, não imprime nada, e sai com `exit 0`, **com uma linha `FAIL` no log**. É
"transformar vermelho em verde" pela definição exata da guarda 1 do ML-2B — só que por um mecanismo de
escopo de shell, não por lógica de negócio errada.

Reproduzido isolando o Cenário 64 (`git-branch-guard/no-op-outside-project/baseline-noop-without-
trackfw-yaml`), que roda dentro de `( cd "$T64_NO_YAML_DIR" && assert_guard_exit ... )`: sabotar o
resultado esperado e rodar o chunk correspondente com o contador em variável mostrou o sintoma exato
acima (esperado: FAIL contado, exit != 0; sem a correção, o defeito não chegou a se manifestar aqui
porque o Defeito 1 já matava o chunk mais cedo — os dois se mascaravam mutuamente até o Defeito 1 ser
corrigido primeiro).

**Correção:** contagem em ARQUIVO dentro de `$WORK` (já existe desde o topo do script, um `mktemp -d`
por processo — cada chunk do driver paralelo tem o seu, então chunks nunca cruzam contagem):
```bash
FALSIFY_ENUM_TALLY="$WORK/enum-failures"
# no ponto de falha:
printf 'x\n' >> "$FALSIFY_ENUM_TALLY"
# no fechamento:
n=$(wc -l < "$FALSIFY_ENUM_TALLY" 2>/dev/null || echo 0)
```
Escrita em arquivo atravessa fronteira de subshell (é I/O, não estado de processo) — funciona
independentemente de quantos níveis de `( ... )`/`$( ... )` envolvem o ponto de falha, sem precisar
auditar os ~199 sites individualmente para provar que nenhum está numa subshell.

## Por que as duas falsificações importavam

Nenhum dos dois defeitos aparece rodando o gate limpo (0 reprovações — a checagem final nunca dispara
de verdade). Só aparecem sob sabotagem deliberada — exatamente o motivo de este projeto exigir prova
por falsificação (`P4`, ver cabeçalho de `check-gates-falsify.sh`) em vez de só "os testes passam". Um
modo de enumeração cuja PRÓPRIA guarda de correção (nunca torna o gate verde) só é auditável por
sabotagem é, em si, uma instância do padrão que a regra dura de reconciliação deste projeto existe
para pegar — por isso os dois vieram para nota de vault, não só comentário de código.

## Terceira armadilha do driver paralelo, correlata

O sentinela `CHUNK_COMPLETE $i` (`scripts/run-gates-falsify-parallel.sh`) exige ser a **última linha**
do log do chunk. A checagem de tally-arquivo tem que rodar **antes** do sentinela (senão nunca sai)
mas só emitir/abortar quando `n > 0` — silenciosa quando `n == 0`, para não virar a "última linha" no
caminho feliz. E como o fechamento do script original só existe fisicamente na fatia de linhas do
ÚLTIMO cenário do arquivo-fonte, `gen-falsify-chunks.py` precisa injetar sua PRÓPRIA cópia da checagem
em TODO chunk gerado — sem isso, só o chunk que por acaso herda essa fatia final teria alguma checagem
de fechamento; os outros N-1 nunca converteriam tally>0 em exit != 0.

Ver: [falsify-gate-windows-python-stub-e-go-bin-de-fixture-sem-go-mod-2026-09-07](falsify-gate-windows-python-stub-e-go-bin-de-fixture-sem-go-mod-2026-09-07.md) (mesma REQ, MLs
anteriores) · [comentario-de-cenario-sobre-bloco-so-de-funcoes-vira-fronteira-de-corte-falsa-2026-09-07](comentario-de-cenario-sobre-bloco-so-de-funcoes-vira-fronteira-de-corte-falsa-2026-09-07.md)
(mesma classe de risco: comportamento correto em serial, quebrado só sob chunking paralelo).
