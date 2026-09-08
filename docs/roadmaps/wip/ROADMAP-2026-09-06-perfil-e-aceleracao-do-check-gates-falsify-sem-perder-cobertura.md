---
status: wip
date: 2026-09-06
squad: ares-tf
req: "docs/req/REQ-2026-09-03-check-gates-falsify-e-610-dos-780-segundos-do-parity-e-o-gate-que-falsifica-os-outros.md"
---

# Roadmap: Perfil e aceleração do `check-gates-falsify`, sem perder cobertura

> Criado em: 2026-09-06 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-03-check-gates-falsify-e-610-dos-780-segundos-do-parity-e-o-gate-que-falsifica-os-outros.md`

## Diagnóstico — medido no CI em 2026-09-06

```
parity                        1204s   (20 min)
todos os outros jobs juntos    361s
```

**O `parity` é 3,3x tudo o mais somado.** E como os jobs rodam em paralelo, **ele É o tempo de ciclo**:
todo PR espera 20 minutos, independentemente do que mudou.

Combinando com a medição da REQ (`check-gates-falsify` = 610 de 780s do `parity`):

```
check-gates-falsify   ~78% do parity
os outros 45 gates    ~22%
```

**Um gate é ~4/5 do tempo de CI do projeto.**

🔴 **E ele NÃO é candidato a corte.** Só em 2026-09-05/06 ele pegou: o fixture que pinava a mensagem
antiga (que as **3 suítes internas** deixaram passar por terem a mesma asserção desatualizada), o
Cenário 80 sabotando const compartilhada por 3 consumidores, e o **bump de versão errado** no Python.
Encolher cobertura para ganhar tempo destruiria o instrumento que mais achou defeito na campanha.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Perfil antes de otimizar
> **Sozinha.** Nada é alterado nesta wave.

### ML-1A — Onde estão os 610 segundos
**Status:** ✅ Concluído · **Agente:** `ares-tf` · **investigação, sem alteração**

🔴 **Conflito de instrução, sinalizado sem resolver:** o CLAUDE.md do projeto instrui o agente a
marcar o próprio ML como Concluído; o role card de Infra (Ares) instrui atualizar o status do roadmap
só **depois** da auditoria do orquestrador. Marquei ✅ seguindo o CLAUDE.md do projeto (autoridade mais
específica para este repositório) — o arquiteto decide se isso fica ou reverte para 🔄 até auditar.

**Resultado:** `docs/portabilidade/2026-09-06-perfil-do-check-gates-falsify.md`. Resposta: **execução
domina, não compilação** — compilação foi 81,1s de 921,4s (8,8%) num run local instrumentado (cópia do
script em scratchpad, nunca o arquivo real). O `GOCACHE` fixo já é compartilhado entre as 93
compilações (mediana 0,84s após a 1ª build fria de 4,85s) — a premissa de que cópia para tmp invalida
cache **não se confirmou**. Os 4 cenários de `release-tag-parity` sozinhos somam 112,4s (mais que
todas as 93 compilações juntas); o cluster `validate-parity/*` roda ~9,7s cada (3 runtimes por
cenário). Caminho recomendado para a Wave 2: paralelizar execução (não compilação) — estimativa de
teto ~200-250s com fator conservador de 4x sobre a fração de execução, a confirmar com paralelismo
real (riscos: flakiness sob concorrência já documentado na REQ, e contenção de lock no
`go-build-cache` compartilhado sob compilações simultâneas).

🔴 **A hipótese óbvia (paralelizar cenários) pode estar errada, e o AC1 da REQ já avisa:** muitos
cenários **compilam um binário Go sabotado** para provar detecção (`run_go_guard_dump`, cópias de
`cmd/` e `internal/` para tmp + `go build`). **Se o custo dominante for compilação, paralelizar
resolve pouco** — o caminho seria reaproveitar builds entre cenários que sabotam o mesmo alvo.

**Entregar:** tempo por cenário; quanto de cada um é **compilação** vs **execução**; quantos builds
distintos existem de fato; e quantos cenários poderiam compartilhar um mesmo binário sabotado.

**Complemento (issue #288, dado de terceiro):** verificadas as 226 cópias amplas `cp -r
"$ROOT_DIR/..."` do script — só 1 (linha 543, Cenário 8) carrega conteúdo ignorado pelo git em
volume relevante (bin/+dist/+.git, medido **nesta máquina local**: 203MB). 🔴 **Ressalva de CI, não
reconciliada com o número acima sem checar o workflow:** `.git` no CI é clone raso (`actions/
checkout@v7` sem `fetch-depth` = default 1, `.github/workflows/quality.yml`), `dist/` não é gerado
no job `parity`, e `bin/` **é** gerado (`parity: build` no `Makefile`) — só a parte `bin/` (17-37M,
varia por máquina/toolchain, não é uma divergência a corrigir) se aplica ao CI. Custo agregado de
todas as cópias amplas medido localmente em ~15-20s de 921s (~2%, extrapolação por contagem, não
soma de 226 medições) — confirma "não é a causa, mas não ajuda", e mais ainda no CI (onde a carga
real é menor que a local). Essa mesma linha 543 é a que quebra no Windows/MSYS2 (colisão
`trackfw`/`trackfw.exe` no `cp -r`, falsificada nas duas direções pelo autor do issue) — correção
local a uma linha, não corrigida nesta wave, recomendada como primeiro ML da Wave 2 por destravar o
Windows, não por ganho de tempo. 🔴 **Risco para o ML de correção:** Cenário 8 compila um binário a
partir de `$T8_MOD` logo após essa cópia (linha 559) e as regras de guard são ancoradas em git
(ADR-2026-08-12) — "copiar menos" precisa primeiro determinar o que o Cenário 8 de fato usa, não só
reduzir o volume copiado. Detalhes: `docs/portabilidade/2026-09-06-perfil-do-check-gates-falsify.md`
§ "Complemento — cópias amplas".

**Critérios:** perfil com números reais · a resposta explícita "o gargalo é compilação ou execução?"
· 🔴 **"não vale a pena" é resultado válido** (AC5 da REQ) — se o ganho possível for pequeno, dizer.

## Re-medição no CI — arquiteto, 2026-09-07

O perfil do ML-1A foi feito **localmente** (921s). O job do CI é outro número, e a diferença
**cresceu** depois do perfil. Re-medido no run `34055694451` (`main`, 06/09), atribuindo o tempo por
segmento entre invocações de script:

```
check-gates-falsify.sh          876.5s   72.6%
check-parity-contract-coverage    94.3s    7.8%
check-agent-namespace-union       33.4s    2.8%
check-doctor-parity               27.0s    2.2%
os outros 43 gates                ~177s   ~14.6%
                                 ------
soma dos segmentos                1208s   (job inteiro: 1237s)
```

**O que muda:** o absoluto do `check-gates-falsify` era **610s** na REQ e agora é **876s** — +44% em
quatro dias, pelo acréscimo de cenários (só o PR #289 pôs 181 linhas novas). A tendência do job na
`main`: ~18m no início de setembro, **20-21m** em 06/09.

**O que não muda:** a premissa da Wave 2 continua válida. Um gate é ~3/4 do tempo, e o segundo
colocado é 9x menor. Atacar o `check-gates-falsify` continua sendo o alvo certo.

🔴 **Armadilha de atribuição, registrada porque quase me pegou:** a primeira passada mediu intervalo
**entre marcos** e apontou "507s depois do `check-serve-address-parity`" — número real, atribuição
errada, porque entre dois marcos filtrados cabem centenas de linhas de outro script. A atribuição
correta segmenta por **invocação de script**. Mesmo modo de falha do 69-vs-101.

## Wave 2 — A aceleração que o perfil indicar
> Dependências: ML-1A. **O caminho é escolhido pela medição, não por hipótese.**

### ML-2A — Acelerar, mantendo cobertura idêntica
**Status:** ✅ Concluído (parcial, declarado) · **Agente:** `ares-tf`

**Entregue:** o Passo 0 — a cópia ampla do Cenário 8 (`cp -r "$ROOT_DIR/."`, issue #288) trocada pelo
padrão mínimo já usado pelos Cenários 80+. Destrava o Windows, onde o `cp -r` abortava por colisão
`trackfw`/`trackfw.exe`. Auditado: todas as referências a `$T8_MOD` usam só `cmd/`, `internal/`,
`go.mod`, `go.sum`, e a determinação veio **antes** da redução, como exigido.

**Medido e não embarcado:** o split de 2 vias, ~1.8x. Vai para o ML-2D.

**Diagnóstico corrigido:** a conclusão de "cluster indivisível de 467s" **não se sustenta** — ver
ML-2C. O agente acertou que o arquivo é hostil a parsing por número de linha (tropeçou três vezes
nisso, e o arquiteto tropeçou na mesma pedra ao verificar); errou a causa da indivisibilidade.

🔴 **Crédito onde é devido:** ele reverteu uma afirmação estática própria — *"zero dependência entre
cenários"* — porque a **execução real** sob `set -u` a desmentiu, e recusou-se a embarcar automação
em que não confiava. Foi a recusa que preservou a medição.
🔴 **Cobertura verificada por CONJUNTO, não por contagem** (AC2 da REQ): o conjunto de cenários
executados antes e depois tem de ser **o mesmo**. Contagem igual com conjunto diferente é regressão
disfarçada — e este projeto já foi mordido por isso.
🔴 **Falsificação do próprio harness** (AC3): numa amostra, sabotar o alvo e confirmar que o cenário
**ainda reprova** depois da otimização.
🔴 **Gate paralelo instável é PIOR que gate lento** — ensina a re-rodar em vez de investigar. Se
aparecer flakiness, **parar e reportar**, não "re-rodar para confirmar".

### ML-2C — Içar as 31 funções espalhadas para o prelúdio
**Status:** ✅ Concluído · **Agente:** `ares-tf` · **pré-requisito do ML-2D**

🔴 **Corrige um diagnóstico do ML-2A.** O ML-2A concluiu que os Cenários 86–165 formam um bloco
**indivisível** de 467s (52,3%) por acoplamento de dados entre cenários, e derivou daí um teto de
1,91x. **Medido pelo arquiteto em 2026-09-07, o acoplamento não existe:**

```
blocos de cenário no cluster        29
arestas reais entre cenários         0    ← as 3 detectadas eram falso positivo:
                                            "$T86" dentro da string do echo final
variáveis herdadas do preâmbulo      2    ← $ROOT_DIR (261x) e $WORK (74x)
```

A causa real é outra:

```
funções auxiliares no arquivo       40
   definidas no preâmbulo            9
   espalhadas entre os cenários     31    ← a causa
```

Um trecho que começa no meio do arquivo não enxerga funções definidas antes dele. **Reproduzido:** um
chunk com preâmbulo + trecho do meio do cluster morre com `corrupt_literal: command not found` —
função definida na linha 1375, entre cenários.

**Ação:** mover as 31 definições espalhadas para junto das 9 do prelúdio. **Mudança de posição, não de
conteúdo** — o corpo de cada função é movido byte a byte.

⚠️ **O arquivo é hostil a parsing ingênuo, e isto não é teoria.** Ao extrair as 40 funções por
`^nome() {` até a primeira linha `}`, o arquiteto quebrou num heredoc que contém `}` — e o ML-2A
relatou os mesmos três tropeços (caminho relativo velho, heredoc mal detectado, contagem de linha
defasada). **Não confie em número de linha nem em fim-de-função por `}` na coluna zero.**

**Critérios de aceite:**
- [x] `diff` do conjunto de rótulos de cenário antes/depois: **vazio** (388 labels de cada lado,
      `diff <(sort -u before) <(sort -u after)` vazio)
- [x] tempo serial antes/depois **equivalente** — OLD 889s vs NEW 893s (+0,45%; NEW ligeiramente mais
      lento, o que descarta aquecimento de cache de build como explicação de qualquer diferença)
- [x] `make quality QUALITY_EXIT=0` para **arquivo**, `grep -c '^FAIL'` sobre a saída inteira = 0
      (log com 3830 linhas, 0 ocorrências de `^FAIL`)
- [x] o corpo de cada função movida é **idêntico** — provado por comparação byte a byte de old
      (`git show HEAD:...`) menos as 31 funções == new menos o bloco inserido (10244 == 10244 linhas,
      igualdade exata) e por comparação individual das 31 funções (0 mismatches)
- [x] nenhuma função passa a ser definida **depois** do primeiro uso — as 40 definições agora ficam
      todas entre as linhas 54–976, antes do primeiro `# Cenário` (linha 998)

**Entregue (relatório completo do agente no handoff):** heredoc-aware scanner (exclui `<<<` via
lookaround, evita a armadilha que quebrou o extrator do arquiteto) + `bash -n` isolado por função
(40/40 limpo) + toda linha de fechamento é `}` solitário + sem sobreposição de faixas — quatro
confirmações independentes da fronteira de cada função. Prova do objetivo: o mesmo trecho que morria
com `corrupt_literal: command not found` (Cenário 177, linhas antigas 9210–9296) roda limpo (4x `OK`)
como `preâmbulo (novas linhas 1–996) + trecho (novas linhas 9423–9509)`.

**Não é puro `git mv`:** 31 linhas em branco separadoras foram adicionadas (uma após cada função
realocada); os pontos de remoção mantêm as linhas em branco originais ao redor, então algumas ficam
adjacentes agora. Comentários de documentação que precediam funções específicas foram deixados nos
sítios originais (não são corpo de função) — parte deles agora descreve uma função definida ~8000
linhas antes; sinalizado para o arquiteto, não corrigido (fora do escopo "mudança de posição, não de
conteúdo").

**Site de mesma causa (ML-2D, não corrigido aqui):** `ROOT_DIR` (linha 22) deriva de
`${BASH_SOURCE[0]}` e quebra se um chunk for materializado fora de `$ROOT_DIR/scripts/` (reproduzido:
`cp: .../cmd/.: No such file or directory` ao rodar um chunk de scratch). Sugestão para o ML-2D:
`ROOT_DIR=${TRACKFW_ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}` — uma linha, destrava
geração de chunk a partir de qualquer tmpdir.

### ML-2D — Gerador de fronteiras e paralelismo em produção
**Status:** ✅ Concluído (reentrega pós-reprovação) · **Agente:** `ares-tf` · **Dependências: ML-2C**

## Reentrega do ML-2D — `ares-tf`, 2026-09-07

🔴 **Causa raiz corrigida — a hipótese anterior ("42 grafias de cabeçalho divergentes") não se
confirmou.** `HDR_PAT` já aceitava acento opcional, plural e os dois travessões; o gap entre "137
cabeçalhos" e "95 que casam o padrão estrito" era, na maior parte, **prosa** (comentários que
*mencionam* um cenário sem *ser* seu cabeçalho — ex. `# Cenários 14/16/17/20/21...`). Reproduzido:
`python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh /tmp/probe 8` dá
`prelude_end_line=912`, não ~1160 — a linha 913 (`# Cenario 166 -- Direcao A...`) casa o padrão e
vira fronteira, mas o trecho que ela introduz (913-1160) **não contém nenhuma asserção própria**: só
define 5 funções (`setup_s166_tree`, `write_req_adr_placeholder_fixture`,
`write_req_roadmap_prose_fixture`, `run_node_chain_probe`, `run_python_chain_probe`) consumidas por
cenários **quase 8000 linhas depois** (9015-10927). Confirmado por grep: `chunk_0.sh` definia,
`chunk_6.sh` chamava — `command not found` sob `set -euo pipefail`, matando tudo depois na chunk. Isso
explica **as duas pontas do relatório reprovado ao mesmo tempo**: os 77 rótulos perdidos (cauda da
chunk que abortou) e o único FAIL (`serve-chain-canonical-link/node/edge-baseline`) — reproduzido
isoladamente (`bash -c "$(declare -f run_node_chain_probe); run_node_chain_probe ...\`" com a função
indefinida sai com **exit 2**, exatamente o valor do relatório original, não hipótese).

**Correção:** `gen-falsify-chunks.py` agora classifica cada segmento fronteira em SUPORTE (zero
asserção própria E só definição de função, heredoc-aware, mesmo critério de fronteira `}` solitário
que o ML-2C validou) ou ASSERÇÃO. Segmentos de suporte são içados para um **preâmbulo estendido**
(idêntico em todo chunk — definição de função não tem ordem de execução, içar é sempre seguro);
segmentos de asserção seguem o pipeline de fusão por variável + LPT inalterado. Medido no arquivo
real: 2 segmentos de suporte (`166`: 248 linhas; `55/56/57`: 10 linhas, separador só-comentário,
inerte) — os outros 20 "zero-assert" que uma varredura ingênua acusaria são cenários reais com
`echo "OK/FAIL [falsify/...]"` manual em vez de `assert_*` (ex. Cenário 18), corretamente
NÃO-classificados como suporte por procurar o sinal por substring (não âncora em coluna 0 — muitas
chamadas vêm depois de `cd ... &&`). Um terceiro candidato (`60/61`, efeito colateral de `cd` até o
Cenário 64 restaurar) tem código de topo além de função — corretamente mantido como asserção; já era
coberto pela fusão por variável existente (`GBG_ORIGINAL_PWD` referenciado no Cenário 64), verificado
sem alteração de código.

**Guarda de completude no gerador** (substitui a checagem de contagem de cabeçalhos, que se mostrou a
diagnose errada): `accounted = prelude + Σ(suporte) + Σ(asserção)` tem de bater com `len(lines)` — sai
com `SystemExit` se não bater. Sabotagem: subtrair 3 do `accounted` manualmente → gerador recusa gerar
chunks (`guarda de completude falhou -- contabilizado=10927, arquivo tem 10930`).

**Guarda de conjunto no driver** (`run-gates-falsify-parallel.sh`), derivada do próprio texto de cada
chunk (nunca lista congelada, ver decisão abaixo): duas checagens independentes, rodam **sempre**,
mesmo se algum chunk já reprovou.
1. **Sentinela por chunk**: todo chunk materializado termina com `echo "CHUNK_COMPLETE $i"`; se o log
   não termina nela, o chunk morreu no meio.
2. **Rótulos esperados**: `gen-falsify-chunks.py` extrai, do texto do PRÓPRIO chunk, os rótulos que
   seus `assert_*` podem emitir (literal exato; ou prefixo glob quando o rótulo tem `$var` resolvido só
   em runtime — ex. o loop `roadmap-acceptance-heading/go/$path_name`). O driver confere cada um contra
   `OK|FAIL|PROOF [falsify/rótulo]` do log do MESMO chunk e **nomeia** o que faltar.

Decisão declarada (a escolha pedida no roadmap entre "derivar do fonte" vs "serial de referência"):
**derivar do próprio fonte de cada chunk**, não um serial de referência — rodar o serial toda vez para
validar o paralelo anularia o ganho. 🔴 **Escopo medido e limitado, não coberto pela guarda**: o
manifesto real emite **249 rótulos literais únicos + 3 templates glob** (`assert_*` apenas —
`sed -n 's/^chunk=[0-9]* label=//p' | sort -u | wc -l` sobre a saída do gerador, contra `sort -u` do
run serial: 249/386, sem duplicata entre eles), ou seja, **~65%** da suíte por rótulo único — número
reproduzível pelo comando acima, não estimado. Cenários com `echo "OK/FAIL [falsify/...]"` manual
(ex. `no-repo-mutation`, `validate-ok-message/*`, `status-inventory/*`) não entram no conjunto
esperado — muitos desses écos são **assimétricos por design** (ex. `setup-sN` só imprime em erro,
nunca em sucesso), então incluí-los ingenuamente geraria falso-positivo garantido em toda run limpa.
Fechar esse gap com segurança exige distinguir eco "sempre dispara" de "só no caminho de erro", o que
esta entrega não fez — reportado, não é achado de mesma causa (mecanismo de guarda, não de perda de
cobertura), fica para quem priorizar.

🔴 **Segunda limitação declarada do extrator de rótulos**: `extract_expected_labels` pula linha de
comentário (`#`), mas **não** rastreia heredoc — um `assert_fails_with "foo" ...` dentro do BODY de um
heredoc (texto de fixture, não código real) seria capturado como rótulo esperado sem nunca ser
emitido. Mesma família de fragilidade de parser já registrada em
`vault/notes/comentario-inline-com-heredoc-derruba-arquivo-da-populacao-do-gate-2026-09-02.md`, na
direção oposta (aquele falhava aberto — gate passava sobre regressão; este falharia fechado — GUARDA
dispararia sobre um chunk correto). Não medido nenhum caso real no arquivo atual (as 3 rodadas
passaram limpas); declarado como risco de regressão futura, não corrigido nesta entrega.

🔴 **Trailer do arquivo original agora enganoso sob paralelismo — corrigido no driver, não no
arquivo fonte**: `check-gates-falsify.sh:9763` ecoa `"Falsification checks passed (all 181
scenarios...)"` como última linha do último segmento de asserção. Sob chunking essa linha imprime
UMA vez, pelo chunk que ficou com esse segmento (tipicamente 20-30 dos 118 segmentos) — no
`make parity` real ela apareceu depois de só 6 `CHUNK_COMPLETE`, não 8, e passou a descrever só
aquela chunk. Não editado o arquivo fonte (risco desnecessário num arquivo de 10930 linhas já frágil
a parsing, e a frase é conteúdo estático, não lógica de teste) — em vez disso o driver agora emite seu
próprio resumo agregado depois da guarda de conjunto passar: `"suite completa -- N chunks, X OK, Y
FAIL, guarda de conjunto OK"`. Confirmado: `rc=0`, `412 OK`, `0 FAIL`, resumo do driver presente.

**Confirmado que nenhum outro workflow/gate invoca `check-gates-falsify.sh` diretamente** —
`grep -rn 'check-gates-falsify\.sh' .github/workflows/ scripts/ Makefile` só retorna comentários que
MENCIONAM o script (precedente de cenário, contexto de UTF-8), nunca uma invocação executável fora do
`Makefile` já trocado.

**Também gerado e testado a `JOBS=4`** (default de `detect_cpus()` no runner de 4 vCPUs do CI, nunca
antes testado neste roadmap — só 8 tinha sido gerado até aqui): `bash -n` limpo nos 4 chunks, mesma
guarda de completude passa.

🔴 **Teto previsto para o ML-3A, declarado agora para não parecer regressão depois**: o maior bloco
fundido tem `largest_fused_unit_lines=3487` (30 cenários indivisíveis, ver `cross_segment_edges=25`)
contra chunk mediano de ~830 linhas — 4,2x o mediano. A `JOBS=4` esse bloco é `chunk_3` sozinho; a
`JOBS=8` é `chunk_7` sozinho. Ou seja, o piso de tempo é plausivelmente ditado por ESTE bloco, não pelo
número de workers — o que prevê **`JOBS=4` no CI (4 vCPUs) ≈ `JOBS=8` local**, e falsifica a premissa
anterior do roadmap ("com fronteiras livres, o teto passa a ser o número de workers"). Não
re-medido por chunk (tempo por chunk não foi registrado nas 3 rodadas) — declarado como previsão, não
medição; se o ML-3A medir ~1,8x em 4 vCPUs, é o resultado esperado, não uma regressão, e os 25 edges
cross-segmento são a próxima alavanca se alguém quiser mais.

**Falso positivo pego e corrigido durante a própria medição**: a primeira rodada com a guarda nova
acusou `git-branch-guard-global-script-integrity` e `credential-guard-script-integrity` como
"ausentes" com rc=1, mesmo com 412 OK / 0 FAIL idêntico ao serial — não era perda de cobertura, era bug
de extração: `assert_would_now_fail` nunca ecoa o rótulo cru, sempre `$label/non-vacuity` (`PROOF` ou
`FAIL`). Corrigido em `extract_expected_labels` (sufixo aplicado só para essa função). Re-medido limpo
depois — ver abaixo.

**Duas sabotagens provadas** (scripts sintéticos em scratchpad, produção real via
`TRACKFW_FALSIFY_SCRIPT`/`TRACKFW_FALSIFY_GEN`, override só para teste, sem efeito em produção sem a
env var):
1. Rótulo removido de propósito de um chunk (gerador sabotado para pular `chunk_lines.extend` de um
   segmento, mantendo o rótulo na lista esperada) → driver: `GUARDA -- chunk_1: rotulo esperado
   AUSENTE: fake/two`, exit 1.
2. Cenário que crasha no meio da chunk (`comando_que_nao_existe_de_proposito`) → driver: sentinela
   ausente **e** rótulo ausente nomeados, exit 1 (a mesma classe do incidente real: rc já era != 0
   antes, a guarda nova é o que nomeia).
3. Completude do gerador sabotada (`accounted -= 3`) → `SystemExit` antes de gerar qualquer chunk.

**Sabotagem de ALVO, distinta das três acima** (as três testam o harness — gerador/driver; esta
testa se o cenário, dentro de uma chunk gerada normalmente do arquivo real, ainda **reprova quando o
gate que ele falsifica deixa de detectar** — o AC herdado do ML-2A, "sabotar o alvo e confirmar que
ainda reprova"). No gate real `scripts/check-cli-parity.sh`, o trecho que emite
`"${runtime}: missing command '${command}'"` (a mensagem que `assert_fails_with
"cli-parity/missing-command"` do Cenário 5 exige) foi trocado por um no-op (`:`), simulando o gate
deixando de detectar comando ausente. Chunk 0 (JOBS=8, gerado do arquivo real, sem edição) regenerado
para copiar o `check-cli-parity.sh` sabotado (a cópia acontece em runtime, via `$ROOT_DIR`) e rodado
isolado:
```
TRACKFW_ROOT_DIR="$(pwd)" bash /tmp/x_sab/chunk_0.sh
rc=1
FAIL [falsify/cli-parity/missing-command]: saiu com 0, esperava != 0
```
Gate revertido (`diff` vazio contra a cópia pré-sabotagem), `git status --porcelain
scripts/check-cli-parity.sh` limpo depois. Prova que a asserção dentro de uma chunk real não é
vácua: se o alvo parasse de detectar, o cenário pegaria — mesmo particionado.

**Re-medição, mesma máquina, mesma sessão, foreground (nenhum comando em background sem poll ativo em
primeiro plano):**

```
serial                          903s   rc=0   412 OK   0 FAIL
JOBS=8 r1                       478s   rc=0   412 OK   0 FAIL
JOBS=8 r2                       478s   rc=0   412 OK   0 FAIL
JOBS=8 r3                       482s   rc=0   412 OK   0 FAIL
razão                           903/480 ≈ 1.88x (10 vCPUs locais, JOBS travado em 8 por desenho)
```

🔴 **1.88x, não os 3.03x do relatório reprovado** — aquele número era inflado por trabalho
**omitido** (78 cenários faltando, alguns com `go build` caro). Este é o número real, com cobertura
integral provada.

`diff` de conjunto de rótulos, serial × cada rodada paralela — **vazio nas 3**:
```
serial.labels (386 únicos) vs parallel_r1.labels (386) → diff vazio
serial.labels (386 únicos) vs parallel_r2.labels (386) → diff vazio
serial.labels (386 únicos) vs parallel_r3.labels (386) → diff vazio
```

**O 1 FAIL do relatório reprovado — mesma causa, não hipótese**: `serve-chain-canonical-link/node/
edge-baseline` chama `run_node_chain_probe`, uma das 5 funções da "Cenário 166" — a mesma raiz dos 77
rótulos perdidos. Confirmado OK nas 3 rodadas (`OK   [falsify/serve-chain-canonical-link/node/
edge-baseline]`).

**Embarcado em produção**: `Makefile` linha do alvo `parity` trocada de `scripts/check-gates-falsify.sh`
para `scripts/run-gates-falsify-parallel.sh` (mesmo `GO_BIN=...` prefixo, herdado pelos processos
filho). `make parity` completo (todos os ~45 gates, não só falsify) rodado do zero via `make`:
`rc=0`, `712s` (era ~1204s), `0` ocorrências de `^FAIL` no log inteiro (1761 linhas, 1015 `^OK`), 8/8
`CHUNK_COMPLETE`, guarda sem disparo. **Confirmado que o CI passa por aqui, não invoca o script
antigo direto**: `.github/workflows/quality.yml:582` chama `make parity` (não
`scripts/check-gates-falsify.sh`) — o job real do CI vai exercitar a troca, não só o Makefile local.

🔴 **Decisão para o arquiteto, não tomada unilateralmente**: adicionei `TRACKFW_FALSIFY_SCRIPT` e
`TRACKFW_FALSIFY_GEN` (env vars, default = caminho real, sem efeito se não setadas) em
`run-gates-falsify-parallel.sh` para poder provar as sabotagens de harness (acima) contra o driver de
produção real, sem duplicar o script. Efeito colateral: **qualquer processo com essas env vars
setadas redireciona o gate de falsificação para um script arbitrário durante `make parity`/CI** — dado
os ADRs deste repo sobre "controle é onde o agente escreve" (ADR-2026-08-12), um env var que desvia o
que o gate mais caro do CI executa merece revisão explícita, não ficar como efeito colateral de
conveniência de teste. Se reprovado, a alternativa é um script de teste separado
(`scripts/gen-falsify-chunks-test.sh`) sem env var em produção.

**Sítios de mesma causa — reportados, nenhum artefato aberto**:
- O gap "assert_* apenas" da guarda de rótulos (acima) — mesmo mecanismo de guarda, não de perda de
  cobertura; próximo a fechar se alguém priorizar.
- ML-2B (os outros 45 gates) já era item separado deste roadmap, não tocado aqui.

## Medição do ML-2D — arquiteto, 2026-09-07, na mesma sessão e na mesma máquina

```
serial (re-medido)   952s   rc=0    412 OK   0 FAIL
JOBS=4  r1           412s   rc=1                        2.31x
JOBS=8  r1           314s   rc=1    334 OK   1 FAIL     3.03x
JOBS=8  r2           318s   rc=1    334 OK   1 FAIL
JOBS=8  r3           319s   rc=1    334 OK   1 FAIL
```

**O ganho é real e a estimativa de ~3,5x quase se confirma: 3,03x a J=8.** E não há flakiness — as
3 rodadas dão o mesmo tempo (±1,6%) e o mesmo conjunto. Determinístico.

🔴 **Mas a entrega REPROVA, por perda silenciosa de cobertura:**

```
rótulos únicos no serial     398
rótulos únicos no paralelo   321
PERDIDOS                      77    (19% da suíte nunca executa)
ganhos                         0
```

**Causa localizada.** `scripts/gen-falsify-chunks.py` casa fronteira com o padrão estrito
`# Cenário N — ...` (acento em "Cenário", travessão `—`). No arquivo real:

```
cabeçalhos que casam o padrão estrito    95
cabeçalhos de cenário existentes        137     ← 42 usam "Cenario" sem acento ou `--`
```

Os perdidos concentram-se no **fim do arquivo** (linhas 9023–10929, de 10930) — a cauda depois da
última fronteira reconhecida não entra em chunk nenhum.

🔴 **Por que isto é o achado e não um detalhe:** a suíte paralela sai com **1 FAIL**, o que *parece*
"quase passando". Ninguém olha para `334 OK` e pensa "faltam 78". Um gate 3x mais rápido que roda 81%
da suíte e se apresenta como quase verde é **pior que o gate lento** — foi exatamente o cenário que a
REQ (AC2) e este roadmap anteciparam por escrito.

**O FAIL restante** (`serve-chain-canonical-link/node/edge-baseline`, do ML-3D) é determinístico nas
3 rodadas e precisa de diagnóstico próprio: é isolamento entre chunks ou defeito real que o serial
mascara?

**Não embarcado no `Makefile`** — confirmado, `git diff --name-only` não lista `Makefile`. Nada em
produção depende disto hoje.

### Correção exigida antes de reconsiderar

1. Fronteira reconhecida por **todas** as grafias presentes, e o gerador **falha alto** se
   `len(boundaries)` divergir da contagem de cabeçalhos — nunca degrada em silêncio.
2. 🔴 **Guarda de conjunto DENTRO do driver**: ao fim, comparar o conjunto de rótulos emitidos com o
   esperado e **sair != 0 se faltar qualquer um**. Sem isso, esta classe de defeito embarca de novo —
   e desta vez sem alguém medindo à mão.
3. A cauda depois da última fronteira tem de entrar em algum chunk, com teste que prove.

Com o ML-2C feito, **qualquer fronteira `# Cenário` vira ponto de corte válido** — o problema difícil
(descobrir onde é seguro cortar) deixa de existir. Este ML entrega o que o ML-2A mediu mas não
embarcou: o harness de paralelização no `Makefile`/CI.

**Medido no ML-2A**, com 2 vias e o cluster ainda monolítico:

```
serial            894.66s
split 2 vias      493.2s · 511.2s · 507.8s   (3 rodadas, diff de conjunto vazio)   ≈ 1.8x
```

Com fronteiras livres, o teto passa a ser o número de workers (4 vCPUs no runner do CI), **não** os
467s. Estimativa de ~3,5x — 🔴 **estimativa, não medição**: falsificar é justamente a entrega.

**Restrições herdadas do ML-2A, todas mantidas:**
- 🔴 Cobertura por **conjunto**, não por contagem. `diff` vazio, anexado.
- 🔴 **3 rodadas** com o mesmo conjunto aprovando. Flakiness ⇒ **parar e reportar**, nunca re-rodar.
- 🔴 **Nomear o mecanismo de isolamento**; listar todo estado compartilhado encontrado.
- 🔴 Falsificação do harness: numa amostra, sabotar o alvo e confirmar que **ainda reprova**.
- 🔴 **Não** gerar o split por número de linha — usar as fronteiras, descobertas em runtime.

**Critérios de aceite:**
- [x] paralelismo **embarcado** no `Makefile`/CI, não só medido em scratchpad — `make parity` real,
      do zero, `rc=0`, `712s` (era ~1204s), `0` `^FAIL` no log inteiro
- [ ] ganho medido **no CI** — medido **localmente** (mesma máquina, mesmo método nas duas pontas,
      903s×480s≈1.88x), CI é Wave 3/ML-3A, dependência declarada, não medido aqui de propósito
- [x] os 5 pontos vermelhos acima, cada um com evidência no relatório (ver "Reentrega do ML-2D" acima)

### ML-2B — Os outros 45 gates do alvo `parity`
**Status:** 🚫 **Abandonado** — decisão do arquiteto, 2026-09-08, com motivo medido
Eles são ~22% do tempo e rodam **em sequência dentro de uma receita só** do `make` — paralelismo
nunca foi possível ali, não foi desabilitado.
**Antes de paralelizar, medir o compartilhamento:** quais escrevem em caminho fixo de `/tmp` ou tocam
a árvore. Paralelizar gates que compartilham estado corrompe silenciosamente.

## Wave 3 — Confirmar no CI
> Dependências: Wave 2.

### ML-3A — Ganho medido em run comparável
**Status:** ✅ Concluído · **Agente:** `trackfw_architect` · medido em 2026-09-07

```
06/09   20m41s   ← antes
07/09   15m00s   ← PR #291, com o paralelismo        −27%
```

🔴 **Marcador estava obsoleto:** a medição já estava escrita neste roadmap (seção *"Medição no CI"*)
e o `Status` continuava `⬜`. Estado do artefato divergindo do estado real — segundo caso hoje, e o
mesmo defeito que a regra dura de reconciliação existe para pegar. Corrigido na auditoria final.

**Ressalva que a medição obriga:** o ganho é **menor** que o 1,89x local porque o runner tem 4 vCPUs
contra 10 cores da máquina de medição. E o arco completo desmonta a comemoração — `13m23s` quando a
REQ abriu, `20m41s` depois de quatro dias somando cenários, `15m00s` agora. **A paralelização pagou a
dívida que nós criamos e sobrou pouco.**
🔴 **Medido no CI** (AC4), não somado do local. E com as duas pontas medidas pelo mesmo método —
`vault/notes/contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo-2026-09-04.md`.


## Auditoria do ML-2D (reentrega) — arquiteto, 2026-09-07

**Re-medido pelo arquiteto, do zero, sem usar os números do relatório:**

```
serial     901s  rc=0
paralelo   477s  rc=0                    1.89x   (JOBS=8)
rótulos    398 = 398 · perdidos 0 · inventados 0
make quality (driver paralelo cabeado)   777s · 0 FAIL em 4107 linhas · 1015 OK
```

O agente reportou 903s/478s — diferença é ruído de máquina. **A igualdade de conjunto é real.**

### O número pior é o resultado melhor

```
entrega reprovada    3.03x  sobre 321 rótulos (81% da suíte)   ← rápido e errado
reentrega            1.89x  sobre 398 rótulos (100%)           ← mais lento e certo
```

Aceitar os 3,03x teria embarcado um gate que roda 4/5 da suíte e se apresenta como quase verde. O
`parity` cairia de 20 para 7 minutos **com todo mundo achando que ganhou**.

### 🔴 Meu diagnóstico da reprovação estava errado

Eu afirmei que a causa era **grafia de cabeçalho** (95 de 137 casando o padrão estrito). O agente
mediu e achou outra: um comentário `# Cenario 166 -- ...` é fronteira **legítima**, mas abre um bloco
**só de funções** usadas por cenários ~8000 linhas depois. Sem içar esse bloco, ele cai num chunk
diferente do dos chamadores e `command not found` mata o resto do chunk em silêncio.

Isso explica **os 77 rótulos perdidos e o FAIL isolado de uma vez** — a minha hipótese explicava só a
primeira metade. E é a mesma família do ML-2C: o problema nunca foi acoplamento de dados, é **onde as
funções estão definidas**. Vault: `comentario-de-cenario-sobre-bloco-so-de-funcoes-vira-fronteira-de-corte-falsa-2026-09-07.md`.

### Parecer de segurança (`hades-tf`) sobre os overrides de env

**Inalcançáveis em CI, medido:** nenhum workflow seta `TRACKFW_FALSIFY_*`; o trigger é
`pull_request` (não `pull_request_target`), então PR de fork roda o workflow congelado da base. Quem
pudesse setar a env já poderia trocar a linha `run: make parity` inteira.

🔴 **Mas ele achou o que ninguém tinha visto: a guarda de conjunto criada por este ML é
auto-referencial.** Ela deriva os rótulos esperados do **mesmo `$SCRIPT`** que foi substituído.
Reproduzido: um stub com 1 cenário sai `"suite completa -- 1 chunks, 1 OK, 0 FAIL, guarda de conjunto
OK"`, exit 0. E o manifesto nunca imprime o caminho de `$SCRIPT` — **o redirecionamento não deixa
rastro**.

O risco real não é sabotagem (quem roda `make` na própria máquina já pode tudo): é **auditabilidade
zero**, inclusive sem má intenção — uma env do ML-2D esquecida num `.envrc` silencia o gate.

Ele **rejeitou** o controle "recusar quando `CI=true`" com o argumento certo: quem seta
`TRACKFW_FALSIFY_SCRIPT` no workflow seta `CI: ""` no mesmo edit. Defenderia o vetor já fechado e
nenhum dos reais.

**Critério destilado, que vale além deste caso:** o problema não é a env existir, é o **call site não
pinar o valor**. `GO_BIN` é seguro porque o `Makefile` o seta em toda invocação.

### ML-2E — Rastro do override e o `HASH_CMD_BIN`
**Status:** ✅ Concluído · **Agente:** `ares-tf`

1. **Feito.** `scripts/run-gates-falsify-parallel.sh` avisa em `stderr` sempre que
   `TRACKFW_FALSIFY_SCRIPT`/`_GEN`/`_JOBS` estiverem setadas (mesmo que o valor coincida com o
   default), com valor efetivo e default. Provado rodando: as 3 linhas aparecem com as 3 env vars
   setadas; nenhuma linha aparece sem override.
2. **Sítio de mesma causa, feito.** `scripts/check-roadmap-barrier-contract.sh:444-448` —
   `HASH_CMD_BIN` agora é pinado pelo `Makefile` (`HASH_CMD := $(shell command -v sha256sum ... ||
   echo "shasum -a 256")`, passado na linha de recipe, mesmo desenho de `GO_BIN`); o script faz split
   intencional da string do env em vez de tratá-la como nome de comando único. **Sabotagem provada
   nas duas pontas** (corpus do snapshot mutado — 1 status alterado, revertido depois): antes da
   correção, `HASH_CMD_BIN` forjado (script que sempre emite o hash pinado) mascarava a reclassificação
   real (`OK [corpus/non-reclassification]` indevido); com hash real, o mesmo corpus mutado reprovava
   corretamente. Depois da correção, invocando como o Makefile invoca (`HASH_CMD_BIN="sha256sum"`
   pinado na recipe) COM `HASH_CMD_BIN` forjado exportado no ambiente pai, o gate voltou a reprovar
   corretamente — o pin no call site derrota o override ambiente.
3. **Feito por ser barato.** `smoke-integration-packages.sh:34` (`PYTHON_BIN`) pinado
   (`PYTHON_BIN=python3` na recipe `package-smoke` do Makefile) por consistência — severidade já era
   menor (sem guarda anexa satisfazível vaziamente; não está em `make quality`).

**Validação:** `make quality` reproduzido em batches foreground (limite de 10 min por chamada de
ferramenta) — `test`/`test-node`/`test-python`/`lint` isolados + `parity` em 3 blocos na ordem exata
de `make -n parity`. Log combinado: `grep -c '^FAIL'` = 0 em 4060 linhas.
`run-gates-falsify-parallel.sh` sozinho: 412 OK, 0 FAIL, guarda de conjunto OK.
`scripts/check-cli-parity.sh` isolado: rc=0.


## Auditoria do ML-2E — arquiteto, 2026-09-07

**Verificado por mim, não pelo relatório.** Com `HASH_CMD_BIN=/tmp/fake-hash.sh` **exportado no meu
ambiente**:

```
make -n parity → GO_BIN=bin/trackfw HASH_CMD_BIN="sha256sum" scripts/check-roadmap-barrier-contract.sh
```

**O pin vence o ambiente.** É o critério do `hades-tf`: o problema não é a env existir, é o call site
não pinar. E o desenho escolhido é o certo — pinar no `Makefile`, como `GO_BIN`, em vez de validar
dentro do script (validação dentro do script seria mais uma regra sem gate).

```
make quality (invocação única)  771s · 0 FAIL em 4107 linhas · 1015 OK
rastro sem override             0 linhas — comportamento default idêntico
```

**Sutileza que o agente resolveu:** env var não carrega array bash. O guard `-z` antigo deixava
comando forjado de uma palavra passar como nome de comando único; agora há split intencional, que o
fallback `shasum -a 256` exige.

### 🔴 Débitos declarados desta entrega

1. **Nenhum teste automatizado novo.** As sabotagens foram medições manuais em foreground —
   reproduzíveis, mas **não versionadas**. Consequência concreta: **se alguém remover o pin do
   `Makefile` amanhã, nada acusa.** O controle existe; o gate do controle, não. É a forma exata do
   problema que esta campanha inteira combate — vira ML-2F.
2. O agente rodou `make quality` em **5 chamadas** (limite de tempo da ferramenta dele), não numa.
   Contorno legítimo, mas gate em pedaços ≠ gate inteiro. Rodei numa invocação só: verde.

### ML-2F — Gate para os pins de call site
**Status:** ✅ Concluído · **Agente:** `ares-tf`

Gate que reprova se `HASH_CMD_BIN`, `PYTHON_BIN` ou o par `TRACKFW_FALSIFY_*` deixarem de ser pinados
nas recipes que os consomem. Falsificação nas duas direções: remover o pin ⇒ reprova; pin presente ⇒
aprova. Guarda de vacuidade contra `Makefile` vazio ou alvo ausente.

**Por que é ML e não "fica para depois":** sem ele, os pins do ML-2E são convenção, não contrato — e
o parecer do `hades-tf` só vale enquanto ninguém editar o `Makefile` sem saber por que aquilo está lá.

## Entrega do ML-2F — `ares-tf`, 2026-09-08

**Arquivo novo:** `scripts/check-parity-call-site-pins.sh`. **Cabeado em:** `Makefile`, alvo `parity`,
logo após `check-output-encoding-declared.sh` (linha nova, sem alterar nenhuma linha existente).

### 1. O que o gate verifica, por variável, e por que o critério difere

**Duas listas fechadas** (`VARS_PIN`, `VARS_TRACE`) — o CRITÉRIO de projeto está escrito no cabeçalho
do script: uma derivação ingênua ("toda env var lida via `${VAR:-...}` em algum script chamado pelo
Makefile") produziria FAIL de dia zero, porque `scripts/check-validate-parity.sh:139` lê
`${GO_BIN:-}` e cai para um binário próprio em `tmp` quando ausente — um site legítimo, já auditado,
fora do escopo deste ML (não é a família de controle do ML-2E). Por isso o CONJUNTO DE NOMES é
congelado — exatamente os que o ML-2E criou — e a manutenção é: todo novo controle desta família
repete o comentário `"ML-2E, mesma família de HASH_CMD_BIN"` (convenção já em uso em `Makefile:82-84`
e `check-roadmap-barrier-contract.sh:444`) e seu nome entra em `VARS_PIN`/`VARS_TRACE` no mesmo PR
que o introduz.

O que **é** derivado em runtime, sem caminho nem número de linha hardcoded: (a) qual script em
`scripts/*.sh` consome cada variável (via grep no corpo do script, não no Makefile); (b) qual linha
de recipe do Makefile invoca esse script (via grep no nome-base do script, não uma linha fixa). Só o
NOME da variável é fixo — o restante é descoberto a cada execução.

- **`HASH_CMD_BIN`, `PYTHON_BIN` → exigem PIN** no Makefile: a linha de recipe que invoca o script
  consumidor precisa conter `VAR=` (regex de borda de palavra, não substring — evita casar
  `HASH_CMD_BIN` contra o `HASH_CMD` do topo do arquivo). Critério: são pinados por desenho (ML-2E),
  então a ausência do pin É a regressão.
- **`TRACKFW_FALSIFY_SCRIPT`/`_GEN`/`_JOBS` → exigem RASTRO**, não pin: o ML-2E decidiu, com aval do
  `hades-tf`, que pinar essas três destruiria a via de sabotagem controlada do harness (é para isso
  que existem). O gate confere que `run-gates-falsify-parallel.sh` ainda tem a guarda
  `if [[ -n "${VAR:-}" ]]` seguida (até 6 linhas depois — medido no arquivo real, o `echo` de
  `TRACKFW_FALSIFY_JOBS` fica na linha +5 da guarda, não +3 como as outras duas) por um `echo ...
  >&2` que menciona a variável.

### 2. As três sabotagens (+ uma quarta, achada durante a implementação), com saída real

Todas rodadas contra uma CÓPIA da árvore em `/private/tmp/.../scratchpad` — nunca o repositório real
foi mutado durante a falsificação; sem `TRACKFW_*` de redirecionamento (ponto do advisor: um env var
de desvio no próprio gate que audita desvios seria a mesma falha que o `hades-tf` achou no ML-2D). A
raiz é passada como `$1` posicional (`ROOT="${1:-.}"`, mesmo idioma de
`check-ci-workflow-job-id-collision.sh:27`); a linha do Makefile não passa argumento, então o
ambiente não redireciona a invocação real.

**Sabotagem 1 — remover o pin de `HASH_CMD_BIN` da linha de recipe:**
```
FAIL [call-site-pin/HASH_CMD_BIN/check-roadmap-barrier-contract.sh]: linha de recipe invoca
check-roadmap-barrier-contract.sh sem pinar HASH_CMD_BIN= -- pin removido: 	GO_BIN=$(BUILD_DIR)/$(BINARY) scripts/check-roadmap-barrier-contract.sh
rc=1
```

**Sabotagem 2 — pin presente (baseline real, sem alteração):**
```
OK   [call-site-pin/HASH_CMD_BIN/check-roadmap-barrier-contract.sh]
OK   [call-site-pin/PYTHON_BIN/smoke-integration-packages.sh]
OK   [call-site-trace/TRACKFW_FALSIFY_SCRIPT/run-gates-falsify-parallel.sh]
OK   [call-site-trace/TRACKFW_FALSIFY_GEN/run-gates-falsify-parallel.sh]
OK   [call-site-trace/TRACKFW_FALSIFY_JOBS/run-gates-falsify-parallel.sh]
rc=0
```

**Sabotagem 3 — guarda de vacuidade, três variantes:**
```
3a) Makefile vazio:
    check-parity-call-site-pins: Makefile ausente ou vazio em .../Makefile
    rc=1
3b) Makefile só com comentários (zero linhas de recipe após o filtro):
    check-parity-call-site-pins: Makefile não tem nenhuma linha de recipe (alvo ausente ou vazio)
    rc=1
3c) script consumidor de PYTHON_BIN removido de scripts/ (renomeado sem atualizar nada):
    FAIL [call-site-pin/PYTHON_BIN/consumer-found]: nenhum script em scripts/*.sh lê ${PYTHON_BIN:-...}
    ou ${PYTHON_BIN} -- o consumidor desapareceu, ou foi renomeado sem atualizar o gate
    rc=1
```
Nenhuma variante aprova por não achar nada — todas nomeiam o que sumiu.

**Sabotagem 4 — achada implementando a Sabotagem 1, não pedida no roadmap, mas é a mesma armadilha
que custou uma reentrega ao ML-2D (comentário vira fronteira falsa):** remover o pin de `PYTHON_BIN`
da linha de recipe real **mantendo** o comentário `Makefile:82-84` que literalmente contém a frase
"PYTHON_BIN pinado":
```
FAIL [call-site-pin/PYTHON_BIN/smoke-integration-packages.sh]: linha de recipe invoca
smoke-integration-packages.sh sem pinar PYTHON_BIN= -- pin removido: 	scripts/smoke-integration-packages.sh
rc=1
```
Confirma que `recipe_lines()` filtra linha de comentário tab-indentada (`grep -vE "^${TAB}[[:space:]]*#"`)
antes de procurar o pin — sem o filtro, o comentário sozinho teria convencido o gate de que o pin
ainda existia.

### 3. Como derivou a lista (frozen, com justificativa)

Ver seção 1 — congelada por desenho, não por atalho: a alternativa (derivar de todo uso de
`${VAR:-...}`) foi tentada mentalmente e descartada por produzir falso-positivo de dia zero contra
`check-validate-parity.sh` (site legítimo e não-relacionado à família ML-2E). O que se derivou de
fato foi o *script consumidor* e a *linha de recipe*, nunca hardcoded.

### 4. Como verificou que não há recursão

`grep -n "TRACKFW\|make \|\.sh\"\|check-.*\.sh\b" scripts/check-parity-call-site-pins.sh` só retorna a
própria linha `VARS_TRACE=(TRACKFW_FALSIFY_SCRIPT ...)` — o gate nunca invoca `make`, nenhum
`check-*.sh` nem `go build`; ele só lê texto (`grep`/`sed`) do `Makefile` e de `scripts/*.sh`. Rodado
isolado: `0.12s` (não os ~478-712s do gate que ele protege) — confirma que não reexecuta o harness de
falsificação. `make -n parity` mostra a linha nova na posição 47/52, sem repetição.

### 5. `make quality` — invocado em blocos, foreground, log combinado

Limite de 10 min por chamada de ferramenta obriga split (o `run-gates-falsify-parallel.sh` sozinho
levou `8:11.53` nesta máquina/sessão — mais lento que os 478-712s de rodadas anteriores, atribuído a
carga concorrente da máquina, não a este ML: nenhum arquivo do harness de falsify foi tocado). 7
blocos em sequência, foreground, cada saída redirecionada para arquivo em
`scratchpad/quality-batchN.log`, concatenados em `quality-combined.log`:

```
grep -c '^FAIL' quality-combined.log   → 0     (sobre as 4115 linhas INTEIRAS, nunca | tail)
grep -c '^OK'   quality-combined.log   → 1020
```

Blocos: (1) build + 19 `check-*-parity.sh` iniciais; (2) 16 `check-*-parity.sh` seguintes; (3)
`run-gates-falsify-parallel.sh` isolado — `rc=0`, `8 chunks, 412 OK, 0 FAIL, guarda de conjunto OK`;
(4) os 11 gates finais do alvo `parity`, **incluindo o gate novo rodando no próprio `parity` real**
(`scripts/check-parity-call-site-pins.sh` aparece como linha própria no bloco, `rc=0` agregado) e
`check-pr-closing-keyword.sh --self-test`; (5) `go test` + `go vet` — `ok` em todos os pacotes; (6)
`npm test` — `885 tests, 0 fail`; (7) `pytest` — `1667 passed, 66 subtests passed`.

`scripts/check-cli-parity.sh` isolado (AC explícito do roadmap): `rc=0`.

### Conjunto de rótulos de falsificação inalterado — método declarado

`git diff --name-only -- scripts/check-gates-falsify.sh scripts/gen-falsify-chunks.py
scripts/run-gates-falsify-parallel.sh` → vazio: nenhum dos três arquivos do harness foi tocado nesta
entrega. Combinado com a guarda interna do próprio driver (construída no ML-2D, auditada no ML-2E)
reportando `412 OK, 0 FAIL, guarda de conjunto OK (nenhum rotulo esperado ausente)` — o mesmo número
que a auditoria do ML-2D já havia confirmado (`412 OK`) — o `diff` vazio + o número idêntico são a
prova de que nada mudou no conjunto; não rodei uma segunda vez o gate de 8 minutos só para comparar
rótulo a rótulo, porque a fonte que gera o rótulo está comprovadamente intocada.

### Regra dura de reconciliação — uma frase por teste novo

Este ML não adiciona teste dentro da suíte `check-gates-falsify.sh` — adiciona um GATE NOVO
independente. A frase por sabotagem (o equivalente do "teste" aqui): Sabotagem 1 afirma "o gate
reprova, nomeando a variável, quando o pin de call site é removido da linha de recipe real do
Makefile" — confirmado pela saída `FAIL [call-site-pin/HASH_CMD_BIN/...]` acima. Sabotagem 3 afirma
"o gate nunca aprova por não achar nada" — confirmado pelas 3 variantes reprovando com `rc=1`,
cada uma nomeando o que sumiu. Sabotagem 4 afirma "um comentário que menciona a variável não é
confundido com o pin real" — confirmado pelo `FAIL` mesmo com o comentário presente.

### Regra dura de paridade — 3 CLIs: exceção explícita, escrita aqui

`scripts/check-parity-call-site-pins.sh` existe **só** em `scripts/`, sem contraparte em
`npm/src/`/`pypi/trackfw/` — não é violação. `docs/cli-parity.md:1-4` define o contrato como
"public commands" dos 3 CLIs (`init`, `req`, `roadmap`, `validate`, ...); um gate interno de CI que
audita o próprio `Makefile` do repositório não expõe superfície de CLI nenhuma. Precedente: os outros
~45 `check-*.sh` do alvo `parity` (`check-cli-parity.sh`, `check-ci-workflow-job-id-collision.sh`,
etc.) também vivem só em `scripts/`, nunca replicados por CLI — é a mesma categoria de
"infra"/tooling de CI que `CLAUDE.md` já lista como exceção explícita à regra de paridade. Nenhum
comportamento visível de `trackfw <comando>` mudou nesta entrega.

### Status ✅ — pendente da auditoria do arquiteto

Marcado ✅ seguindo o mesmo precedente já registrado pelo próprio ML-1A deste roadmap: o CLAUDE.md do
projeto instrui marcar o ML concluído ao entregar; o role card de Infra instrui atualizar só depois
da auditoria do orquestrador. Sigo o CLAUDE.md (autoridade mais específica deste repositório), mas o
✅ é condicional — reverte para 🔄 se a auditoria do `trackfw_architect` encontrar algo que a exija.

### Sítios de mesma causa — reportados, nenhum artefato aberto

- `check-validate-parity.sh:139` também lê um override (`${GO_BIN:-}`) sem pin no Makefile — mas é um
  site já auditado e intencional (fallback para binário próprio em tmp), não a mesma causa do ML-2E
  (não existe guarda que um `GO_BIN` forjado ali possa satisfazer vaziamente da mesma forma que
  `HASH_CMD_BIN` fazia contra `check-roadmap-barrier-contract.sh`). Não incluído na lista congelada;
  se algum dia alguém decidir que merece a mesma proteção, é uma linha nova em `VARS_PIN`, não uma
  REQ nova.
- ML-2G (shardar em jobs de matriz) segue como próximo item do roadmap, não tocado aqui.


## Medição no CI — arquiteto, 2026-09-07 (a que o ML-3A pedia)

```
06/09   20m41s   ← antes
07/09   15m00s   ← PR #291, com o paralelismo
```

**−5m41s por PR (−27%)**, dentro da faixa projetada (13–16 min) e perto do extremo pessimista — como
esperado, porque o runner tem **4 vCPUs** contra os 10 cores da máquina onde medi 1,89x.

🔴 **Mas o arco completo desmonta a comemoração:**

```
13m23s   quando a REQ foi aberta
20m41s   depois de quatro dias acrescentando cenários
15m00s   agora, com o paralelismo
```

**A paralelização não nos deixou mais rápidos que o ponto de partida** — pagou a dívida que nós
mesmos criamos e sobrou pouco. Enquanto cada campanha somar cenários, o número volta a subir.

### ML-2G — Shardar o gate em jobs de matriz (custo zero)
**Status:** 🔄 Em andamento — implementado e provado localmente; falta a medição no CI (ver "O que este
ML NÃO conseguiu fechar" abaixo) · **Agente:** `ares-tf` · **PR próprio** (o #291 já foi mergeado)

**Runner maior não é opção:** larger runners **nunca** entram no free tier, nem em repositório
público — e o `trackfw` é público. Verificado na política de preços de 2026.

**Mas não precisamos de máquina maior — precisamos de mais máquinas, e essas são grátis.** Repositório
público tem jobs concorrentes em runner padrão sem custo. Hoje paralelizamos **dentro** de 1 job × 4
vCPUs; a matriz distribui entre N jobs × 4 vCPUs.

```
hoje     1 job  × 4 vCPUs        →  15min
matriz   4 jobs × 4 vCPUs cada   →  ~4-5min estimado, custo zero
```

🔴 **Estimativa, não medição.** A peça difícil **já existe**: o gerador do ML-2D é parametrizável —
`gen-falsify-chunks.py <fonte> <saída> <n-chunks>`. O mesmo mecanismo que distribui entre processos
distribui entre jobs.

**Três coisas a medir antes de prometer os 4-5 min:**

1. **Custo fixo por job** — cada um paga checkout, setup de Go/Node/Python e build. Se for ~90s, 4
   jobs somam 6 min de overhead e o ganho real encolhe. **Medível no log atual.**
2. 🔴 **A guarda de conjunto atravessando jobs.** Hoje vive dentro do driver. Precisa virar **job de
   agregação** que reprova se faltar rótulo — senão perdemos a proteção construída no ML-2D, que é o
   que impediu o gate de 3,03x rodando 81% da suíte. **Efeito colateral bom:** com a agregação num job
   separado, ela deixa de ser **auto-referencial**, que foi o achado do `hades-tf`.
3. **Self-hosted está fora** — não por custo, mas porque o GitHub desaconselha em repositório público:
   PR de fork executaria código arbitrário na máquina do mantenedor. Não vale para economizar minutos
   que já são grátis.

**Sobre o PR:** o #291 já foi mergeado, então este ML precisa de PR próprio. Não é violação da regra
*"mesma causa ⇒ mesmo PR"* — o mecanismo é outro (topologia de jobs do CI vs. paralelismo em
processo), e o que a regra protege (a janela de atenção sobre a causa) segue aberto **pelo roadmap**,
que continua em `wip` com ML-2F e ML-3A pendentes. 🔴 O que **seria** violação é fechar este roadmap
antes de o ML-2G entrar.

## Entrega do ML-2G — `ares-tf`, 2026-09-08

### 0. O que este ML NÃO conseguiu fechar — declarado antes do resto

🔴 **O AC "tempo medido no CI, não projetado" não é alcançável por mim nesta entrega.** O handoff desta
sessão proíbe explicitamente qualquer operação de Git (branch/commit/push) — regra do meu role card,
não deste roadmap. Não há caminho para um número de CI sem um push. Em vez de queimar a sessão tentando
contornar isso, entrego o mecanismo inteiro provado **localmente** (build, sabotagem, falsificação nas
duas direções) e deixo, na seção 6, a receita exata para o arquiteto rodar no CI e colar os números.
Por isso o status é 🔄, não ✅ — o precedente do ML-2F ("marca ✅, reverte se a auditoria achar algo")
não se aplica quando o próprio agente já sabe, ao entregar, que um AC obrigatório ficou de fora.

### 1. A descoberta que reabre a nota histórica do arquiteto (2026-09-03)

O comentário em `.github/workflows/quality.yml` (então na linha ~574) já dizia "matriz renomeia o
check e bloquearia TODO PR" como razão para não shardar. Medido, não presumido:

```
gh api repos/kgsaran/trackfw/branches/main/protection --jq '.required_status_checks'
{"checks":[...,{"context":"parity"},...],"contexts":[...,"parity",...],"strict":false}
```

`parity` é `required_status_check` **por nome exato**. O efeito de virar matriz não é "bloqueia" —
é **pior de diagnosticar**: o GitHub reportaria `parity (0)`/`parity (1)`/... e nenhum check chamado
`parity` apareceria nunca — todo PR fica **pendente para sempre**, não vermelho. Nota completa em
`vault/notes/matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md`.

**Desenho consequente:** o job `parity` não vira matriz. Os workers viram matriz sob IDs novos
(`parity-falsify-shard`, `parity-other-gates`); `parity` vira um job fino de **agregação**,
`needs: [parity-falsify-shard, parity-other-gates]`, que preserva o nome. O comentário histórico foi
**mantido no arquivo** (não apagado), marcado como vencido nas contas mas correto na decisão que
motivou — mesmo precedente já em uso no comentário do ML-2D em `run-gates-falsify-parallel.sh`.

### 2. Duas armadilhas do job de agregação, das quais uma eu só vi depois de pedir revisão

- `if: always()` faz o job de agregação RODAR mesmo com `needs` falho — necessário, senão o check
  obrigatório também não reporta. Mas sozinho ele faz o job **passar** por padrão nesse caso. Primeiro
  step de `parity` reprova explicitamente se `contains(needs.*.result, 'failure') ||
  contains(needs.*.result, 'cancelled')`, antes de qualquer outra coisa rodar.
- Os rótulos **esperados** da guarda de conjunto são recalculados no PRÓPRIO job de agregação, a partir
  do checkout fresco dele (nunca de algo que um shard upload) — só os rótulos **emitidos** (fato
  observado no stdout real do chunk) vêm dos artefatos de shard. Sem isso seria o mesmo defeito
  auto-referencial que o `hades-tf` achou no ML-2D, em escala de job.

### 3. Arquivos novos e alterados

- **Novo** `scripts/run-gates-falsify-shard.sh` — roda UM chunk (`SHARD_INDEX`/`SHARD_COUNT` via env),
  reutilizando `gen-falsify-chunks.py` (mesmo gerador do ML-2D). Guarda LOCAL (sentinela + rótulos deste
  chunk) como defesa em profundidade; grava `shard_N.actual`/`shard_N.log`/`shard_N.rc` em `OUTPUT_DIR`.
- **Novo** `scripts/check-falsify-shard-coverage.sh` — guarda de CONJUNTO entre shards, rodada pelo job
  de agregação. Recalcula o esperado a partir de checkout fresco (seção 2); nomeia shard ausente, shard
  com rc != 0, e rótulo esperado ausente do conjunto emitido por aquele shard especificamente.
- **`Makefile`** — `parity` dividido em `parity-rest` (os ~45 gates que não são falsify) e
  `parity-falsify` (só `run-gates-falsify-parallel.sh`); `parity: build parity-rest parity-falsify`
  preserva `make parity`/`make quality` locais bit-a-bit equivalentes ao comportamento anterior (mesmas
  46 linhas de recipe, mesmo pin `GO_BIN=`; a ÚNICA diferença observável é a ORDEM — falsify passa a
  rodar por último em vez de ~posição 30 — sem efeito de cobertura, `make -n parity` confirma as 52
  linhas, incluindo comentários, idênticas ao `git diff` só reordenando).
- **`.github/workflows/quality.yml`** — `env.FALSIFY_SHARD_COUNT: "4"` no topo; jobs
  `parity-falsify-shard` (matriz `shard: [0,1,2,3]`, `needs` idêntico ao `parity` original:
  `[go, node, python, package-smoke, windows-integrations-resolve]`), `parity-other-gates` (mesmo
  `needs`, roda `make parity-rest`), e `parity` (agregação, seções 1-2). Grau da matriz cabeado
  (GitHub Actions não permite matriz dinâmica sem job prévio com `fromJSON`) com guarda de drift: um
  step compara `strategy.job-total` contra `env.FALSIFY_SHARD_COUNT` e reprova cedo se alguém mudar um
  sem o outro.

### 4. Mecanismo de isolamento entre shards + estado compartilhado (AC herdado do ML-2D)

**Isolamento:** cada shard é uma VM efêmera própria do GitHub Actions — isolamento de processo,
filesystem e ambiente **mais forte** que o dos processos-irmãos do ML-2D (que compartilhavam
`GOPATH`/`GOCACHE`/`GOMODCACHE` do runner). **Estado compartilhado encontrado:** nenhum deliberado —
nem cache do `actions/setup-go` (nem o job `parity` original nem os novos setam `cache: true`, herdado
sem alteração) nem artefato de build entre shards; cada um baixa suas próprias deps e builda seu
próprio `bin/trackfw` do zero. O único artefato que atravessa job é o de saída
(`falsify-shard-N` → baixado pelo job de agregação), com nome único por valor de matriz — confirmado
que `upload-artifact@v4` não colide (`name: falsify-shard-${{ matrix.shard }}`, um valor por shard).

### 5. Falsificação — provas, com saída real

**Sintaxe/estrutura do workflow:**
```
python3 -c "import yaml; yaml.safe_load(open('.github/workflows/quality.yml'))"  → sem erro, 11 jobs
actionlint .github/workflows/quality.yml                                         → rc=0, sem achado
```

**Gates de governança do próprio repositório, re-rodados após CADA edição de Makefile/workflow (não só
no final — pedido explícito da revisão desta entrega):**
```
scripts/check-parity-call-site-pins.sh        → rc=0, 7 verificações (achou run-gates-falsify-shard.sh
                                                  como novo consumidor de TRACKFW_FALSIFY_SCRIPT/_GEN
                                                  automaticamente, por reusar o mesmo idioma de guarda)
scripts/check-ci-workflow-job-id-collision.sh → rc=0 (audita templates gerados p/ OUTROS projetos, não
                                                  quality.yml -- confirmado sem interferência)
go build ./... && go vet ./...                → OK
```

**Predição escrita ANTES de qualquer execução real (o AC da regra dura de reconciliação, para a
medição de `gen-falsify-chunks.py`):** o ML-2D já mediu `largest_fused_unit_lines=3487` (30 cenários
indivisíveis). Gerado (sem executar) o manifesto real a `N=4`:
```
python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh <dir> 4
chunk=0  3536 linhas
chunk=1  3525 linhas
chunk=2  3527 linhas
chunk=3  4916 linhas   ← carrega o bloco fundido de 3487 linhas sozinho (fused_blocks=1)
```
**Confirma a predição do advisor por escrito antes de medir tempo:** a `N=4` o bloco indivisível é
`chunk_3` sozinho — ~1,4x o tamanho dos outros 3. Isso prevê que `N=8` **não** vai chegar perto de
2x mais rápido que `N=4`: o piso continua sendo esse mesmo bloco, que não se divide mais (é `chunk_7`
sozinho a N=8, mesmo tamanho absoluto). Se a medição do arquiteto no CI (seção 6) confirmar isso, é o
resultado esperado, não uma regressão — e os 25 `cross_segment_edges` já nomeados pelo ML-2D continuam
sendo a próxima alavanca, não este ML.

**Integração real, ponta a ponta, contra o script de produção (não fixture sintética):**
```
make build
SHARD_INDEX=1 SHARD_COUNT=4 OUTPUT_DIR=<dir> bash scripts/run-gates-falsify-shard.sh
→ 92 OK/rótulos emitidos, sentinela "CHUNK_COMPLETE 1" presente, guarda local sem disparo,
  rc=0, ~2min55s de parede (1 de 4 shards -- não é o número final de CI, é a prova de que
  o mecanismo roda contra o arquivo real e produz os 3 artefatos esperados:
  shard_1.actual (92 linhas), shard_1.log, shard_1.rc="0")
```

**Sabotagens do job de agregação (`check-falsify-shard-coverage.sh`), com o manifesto REAL a `N=4` e
os rótulos REAIS extraídos dele (shard_1 é o run real acima; shards 0/2/3 são os rótulos literais que
o próprio manifesto declara esperar deles, o que é dado observável do gerador, não hipótese) —
`ARTIFACTS_DIR` montado, script rodado sem alterar nenhuma outra variável entre as 4 rodadas:**

```
Caminho feliz (0,1,2,3 todos presentes, rc=0, todos os rótulos esperados presentes):
  rc=0, 8 verificações OK, 0 FAIL

Sabotagem A -- artefato do shard 2 removido (upload falhou / job não rodou):
  FAIL [shard-coverage/shard_2/present]: artefato ausente ... job da matriz nao rodou ou upload falhou
  rc=1

Sabotagem B -- shard_0.rc forjado para 1 (chunk falhou OU guarda local reprovou):
  FAIL [shard-coverage/shard_0/rc]: shard_0 reportou rc=1 (chunk falhou, ou a guarda local do
  shard reprovou -- ver log shard_0.log)
  rc=1

Sabotagem C -- 1 linha removida de shard_3.actual (rótulo real, silenciosamente "esquecido" no upload):
  FAIL [shard-coverage/shard_3/label/adr-not-accepted/go/adr_accepted_when_req_done-baseline]:
  rotulo esperado AUSENTE do conjunto emitido pelo shard 3: adr-not-accepted/go/...-baseline
  rc=1
```

Nenhuma das 3 sabotagens produz `rc=0` por não achar nada — todas nomeiam exatamente o que sumiu, a
mesma exigência que a guarda do ML-2D já satisfazia em escala de processo, agora replicada em escala
de job.

### 6. Regra dura de reconciliação — uma frase por artefato novo

- `scripts/run-gates-falsify-shard.sh` afirma "roda um único chunk e reprova (guarda local) se o
  sentinela ou algum rótulo esperado por ESTE chunk faltar" — confirmado pelo run real contra shard 1
  (rc=0, sentinela presente, guarda local sem disparo) e pela Sabotagem B acima (rc forjado propaga).
- `scripts/check-falsify-shard-coverage.sh` afirma "recalcula o esperado de um checkout fresco e
  reprova nomeando shard ausente, shard com rc!=0, ou rótulo ausente do CONJUNTO emitido por aquele
  shard" — confirmado pelas Sabotagens A, B e C, cada uma reprovando por um mecanismo diferente e
  nomeando exatamente a causa.
- O step "Grau da matriz bate com FALSIFY_SHARD_COUNT" afirma "reprova cedo se a lista `matrix.shard`
  e o env `FALSIFY_SHARD_COUNT` divergirem" — não falsificado nesta entrega (exigiria rodar o workflow
  no CI com um dos dois valores propositalmente errado); declarado como risco residual não coberto
  localmente, não hipótese apresentada como prova.

### 7. Receita exata para o arquiteto medir no CI (o que este ML não pôde fazer)

1. Fazer merge/push desta branch e observar o run do workflow `Quality` no PR.
2. Anotar, do run: tempo de parede de `parity-falsify-shard (0)`, `(1)`, `(2)`, `(3)` individualmente
   (confirma ou refuta a predição da seção 5 sobre `chunk_3`/`shard 3` ser o mais lento), tempo de
   `parity-other-gates`, e tempo do job `parity` (agregação — deve ser segundos, não minutos).
3. Tempo de PAREDE do pipeline inteiro (do primeiro job ao `parity` fechar) — comparável ao "15m00s"
   já registrado pelo arquiteto em 2026-09-07 para o PR #291. 🔴 Não reusar o `954s`/`980s` local desta
   entrega como baseline — o handoff já apontou que esses números não reconciliam aritmeticamente com
   os 15m00s do pipeline real; a comparação correta é CI-contra-CI, run comparável.
4. Confirmar `parity` aparece como check obrigatório reportando normalmente (a preocupação da seção 1).
5. Se quiser falsificar a guarda de conjunto ao vivo: forçar um chunk a falhar (ex. sabotar
   temporariamente um `assert_*` do arquivo real numa branch de teste) e confirmar que `parity`
   reprova — mesmo AC que o ML-2A já exigia para o harness original.

### 8. Regra dura de paridade — 3 CLIs: exceção explícita

`scripts/run-gates-falsify-shard.sh` e `scripts/check-falsify-shard-coverage.sh` existem só em
`scripts/`, sem contraparte em `npm/src/`/`pypi/trackfw/` — mesmo precedente já registrado pelo ML-2F:
são tooling interno de CI deste repositório, não superfície de `trackfw <comando>`.

### Sítios de mesma causa — reportados, nenhum artefato aberto

- Os jobs `windows-full-suites`/`windows-defect-reproduction` não passam por `check-gates-falsify.sh` e
  não foram tocados — fora do escopo deste ML (mecanismo diferente, Windows roda camada 1 completa via
  outro caminho, já documentado nas linhas desses jobs).
- Nenhum sítio novo de mesma causa encontrado durante a implementação.

## Correção pós-auditoria — `ares-tf`, 2026-09-08

O `make quality` completo (rodado numa invocação só pelo arquiteto) abortou antes de fechar:
`check-output-encoding-declared: FAIL` porque `scripts/check-falsify-shard-coverage.sh` invoca
`python3` (linha 44, dentro de `resolve_py_bin`) sem declarar `export PYTHONIOENCODING=utf-8` antes
da primeira invocação — o mesmo ALVO 1 que os outros 40 `scripts/check-*.sh` já cumprem (ML-1B do
ROADMAP-2026-09-02). `rc=2`, 577 `^OK` contra os ≥1020 esperados — o gate morreu no meio, não
reprovou de forma nomeada.

**Causa:** os dois scripts novos deste ML (`check-falsify-shard-coverage.sh` e
`run-gates-falsify-shard.sh`, ambos com `resolve_py_bin()` copiado do driver de processo) foram
escritos e provados isoladamente, fora de `make quality`, então o gate anti-reintrodução do ML-1B
nunca correu contra eles antes desta auditoria.

**Correção — mesma causa, mesmo ML:** adicionado
`export PYTHONIOENCODING=utf-8` logo após `set -euo pipefail`, em ambos os arquivos, antes de
`resolve_py_bin`:
- `scripts/check-falsify-shard-coverage.sh`
- `scripts/run-gates-falsify-shard.sh`

`run-gates-falsify-shard.sh` não aparecia ainda no `FAIL` do arquiteto (a enumeração do gate parou no
primeiro infrator), mas tem o mesmo padrão (`resolve_py_bin` idêntico, comentário próprio dizendo
"Mesma resolução de Python do driver de processo") — corrigido preventivamente na mesma passada, sem
esperar o gate nomear o segundo.

**Não recomendo allowlist:** a única entrada existente em `ALLOWLIST` (`check-roadmap-barrier-
contract.sh`) protege um sítio com PR externo aberto (#238) onde forçar UTF-8 mascararia o defeito de
fundo sob investigação. Nenhuma condição equivalente existe aqui — os dois scripts novos não têm
motivo para divergir do padrão dos outros 39 gates.

**As três medidas, sobre o `make quality QUALITY_EXIT=0` completo redirecionado para arquivo:**
```
rc (MAKE_RC)          = 0
grep -c '^FAIL'        = 0
grep -c 'Error 1'       = 0
grep -c '^OK'          = 1022   (era 577 no run abortado; ≥ 1020 exigido)
wc -l                  = 4117
cauda do log           = "...suite completa -- 8 chunks, 412 OK, 0 FAIL, guarda de conjunto OK..."
```
`scripts/check-output-encoding-declared.sh` isolado: `rc=0`. `actionlint
.github/workflows/quality.yml`: limpo. `go build ./...` e `go vet ./...`: OK.

**Reconciliação:** nenhum teste novo foi adicionado por esta correção — é uma declaração ausente em
duas linhas de shell, coberta pela asserção estática já existente em `check-output-encoding-
declared.sh` (ALVO 1), que passou a aprovar os dois arquivos após a mudança.


## ML-2B abandonado — arquiteto, 2026-09-08

**Motivo medido, não preferência.** Atribuição por segmento no CI (run `34055694451`):

```
check-gates-falsify.sh          876.5s   72.6%   ← atacado (ML-2D, ML-2G)
check-parity-contract-coverage    94.3s    7.8%
os outros 45 gates               ~237s   ~19.6%  ← escopo do ML-2B
```

O ML-2B mira **~20%** de um job que já caiu **27%** (20m41s → 15m00s) e que a matriz do ML-2G deve
levar a poucos minutos. Depois disso, os ~237s **passam a ser a maior fatia** — mas de um job pequeno,
onde economizar 2 minutos não muda o ciclo de ninguém.

🔴 **E o roadmap do ML-2B já registrava o risco que o torna caro:** *"eles rodam em sequência dentro
de uma receita só do `make` — paralelismo nunca foi possível ali, não foi desabilitado. Antes de
paralelizar, medir o compartilhamento: quais escrevem em caminho fixo de `/tmp` ou tocam a árvore.
Paralelizar gates que compartilham estado corrompe silenciosamente."*

Ou seja: **o trabalho barato já foi feito, e o que sobra é o caro** — 45 gates para auditar por estado
compartilhado, com risco de corrupção silenciosa, para ganhar minutos num job que já não é o gargalo.

### Por que abandonar em vez de deixar pendente

ML que ninguém vai fazer é o mesmo passivo das REQs órfãs — **só mais bem escondido**, porque um
roadmap com pendência parece trabalho planejado em vez de dívida. Este projeto tem regra dura contra
exatamente isso: *"registro não é correção"*.

Se o `parity` voltar a incomodar depois da matriz, este ML **reabre com número novo** — a medição por
segmento é reproduzível pelo mesmo método (atribuição por invocação de script no log do CI). O que
não vale é ele ficar `⬜` por anos como promessa.

## Medição no CI do ML-2G — arquiteto, 2026-09-08 (o AC que faltava)

Run `34277875332`, PR #294. **Esta era a medição que nenhum agente podia produzir**: o `Quality` só
roda em `pull_request` ou push na `main`, então o número só existe a partir do PR — não é limitação de
autoridade de push, é do gatilho do workflow. Meu diagnóstico anterior estava impreciso.

```
parity-falsify-shard (2)    10m08s   ← gargalo REAL
parity-other-gates           4m20s
parity-falsify-shard (0)     3m50s
parity-falsify-shard (1)     3m47s
parity-falsify-shard (3)     1m51s   ← o PREVISTO como gargalo
parity (agregação)           0m11s   success

wall-clock do grupo         ~10m20s
soma de CPU                  24m07s
```

### 1. O desenho funciona — e era o risco estrutural

O check obrigatório `parity` **reportou** (`success`, 11s). Se a agregação estivesse errada, ele
ficaria **pendente para sempre** e o PR travaria sem falhar. Não travou.

### 2. Ganho real, metade do projetado

```
15m00s  →  ~10m20s     −31%
projeção era ~4m23s    errei por fator 2,3
```

A causa do erro está no item 3, e não no custo fixo — que eu medi certo (~25s).

### 3. 🔴 A predição do agente foi FALSIFICADA, e é o achado mais útil

Ele escreveu, **antes de qualquer cronometragem**, que `chunk_3` seria o gargalo por carregar o bloco
fundido de 3487 linhas. **`shard 3` levou 1m51s — o mais rápido de todos.** O gargalo é o `shard 2`,
**5,5x mais lento**.

**Conclusão: o empacotador distribui por peso de LINHA, e linha não prevê TEMPO.**

Consistente com o que o ML-1A já havia medido — *execução* domina, não compilação, e alguns cenários
compilam binários Go inteiros. Um bloco grande de asserções baratas pesa muito e roda rápido; um bloco
pequeno que compila Go pesa pouco e roda devagar.

🔴 **Valor metodológico:** a predição estava escrita antes, então o run **falsificou um modelo** em vez
de ser explicado por ele. É o oposto de como nasceu o "cluster indivisível de 467s" — explicação
construída depois de ver o número, e que a medição derrubou depois.

### ML-2H — Rebalancear os shards por tempo medido
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

**O dado necessário já existe:** cada shard reporta seu próprio tempo no log do CI, e o
`gen-falsify-chunks.py` já é parametrizável.

```
hoje (peso por linha)     10m08s / 1m51s   →  desequilíbrio de 5,5x
teto se equilibrado       24m07s / 4       ≈  6m + setup
```

**Ações:**
1. Peso por **tempo medido por cenário**, não por contagem de linha.
2. 🔴 **O tempo por cenário tem de vir de medição, não de estimativa** — e precisa de fonte
   versionada que envelheça de forma visível. Um arquivo de pesos que envelhece em silêncio é o
   próximo defeito silencioso: cenário novo entra sem peso e a distribuição degrada sem aviso.
   **Decida e declare** como a fonte é mantida e o que acontece com cenário sem peso.
3. Rebalancear e **medir no CI** (só existe em PR).

**Critérios de aceite:**
- [ ] desequilíbrio entre o shard mais lento e o mais rápido **abaixo de 2x**, medido no CI
- [ ] `diff` de conjunto de rótulos **vazio**, método declarado
- [ ] cenário sem peso registrado **não** degrada em silêncio — comportamento declarado e provado
- [ ] as **três medidas** do gate local: `rc`, `grep -c '^FAIL'`, e `^OK` ≥ 1022
- [ ] 🔴 "não vale a pena" segue sendo resultado válido: se o rebalanceamento render pouco diante do
      custo de manter a fonte de pesos, **diga**
