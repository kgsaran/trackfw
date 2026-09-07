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
**Status:** ❌ Bloqueado — reprovado na auditoria · **Agente:** `ares-tf` · **Dependências: ML-2C**

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
- [ ] paralelismo **embarcado** no `Makefile`/CI, não só medido em scratchpad
- [ ] ganho medido **no CI**, com as duas pontas pelo mesmo método e o método declarado
- [ ] os 5 pontos vermelhos acima, cada um com evidência no relatório

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
