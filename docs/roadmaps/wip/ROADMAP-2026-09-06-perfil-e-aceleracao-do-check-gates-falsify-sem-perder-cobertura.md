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
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
Eles são ~22% do tempo e rodam **em sequência dentro de uma receita só** do `make` — paralelismo
nunca foi possível ali, não foi desabilitado.
**Antes de paralelizar, medir o compartilhamento:** quais escrevem em caminho fixo de `/tmp` ou tocam
a árvore. Paralelizar gates que compartilham estado corrompe silenciosamente.

## Wave 3 — Confirmar no CI
> Dependências: Wave 2.

### ML-3A — Ganho medido em run comparável
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
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
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

Gate que reprova se `HASH_CMD_BIN`, `PYTHON_BIN` ou o par `TRACKFW_FALSIFY_*` deixarem de ser pinados
nas recipes que os consomem. Falsificação nas duas direções: remover o pin ⇒ reprova; pin presente ⇒
aprova. Guarda de vacuidade contra `Makefile` vazio ou alvo ausente.

**Por que é ML e não "fica para depois":** sem ele, os pins do ML-2E são convenção, não contrato — e
o parecer do `hades-tf` só vale enquanto ninguém editar o `Makefile` sem saber por que aquilo está lá.
