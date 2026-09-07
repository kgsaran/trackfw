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
**Status:** ⬜ Pendente · **Agente:** `ares-tf` · **pré-requisito do ML-2D**

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
- [ ] `diff` do conjunto de rótulos de cenário antes/depois: **vazio**
- [ ] tempo serial antes/depois **equivalente** (é refactor, não otimização — regressão de tempo aqui
      é sinal de que algo mudou de comportamento)
- [ ] `make quality QUALITY_EXIT=0` para **arquivo**, `grep -c '^FAIL'` sobre a saída inteira = 0
- [ ] o corpo de cada função movida é **idêntico** — provar com `git diff` mostrando só remoção num
      ponto e inserção idêntica no outro
- [ ] nenhuma função passa a ser definida **depois** do primeiro uso

### ML-2D — Gerador de fronteiras e paralelismo em produção
**Status:** ⬜ Pendente · **Agente:** `ares-tf` · **Dependências: ML-2C**

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
