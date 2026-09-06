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

## Wave 2 — A aceleração que o perfil indicar
> Dependências: ML-1A. **O caminho é escolhido pela medição, não por hipótese.**

### ML-2A — Acelerar, mantendo cobertura idêntica
**Status:** ⬜ Pendente · **Agente:** `ares-tf`
🔴 **Cobertura verificada por CONJUNTO, não por contagem** (AC2 da REQ): o conjunto de cenários
executados antes e depois tem de ser **o mesmo**. Contagem igual com conjunto diferente é regressão
disfarçada — e este projeto já foi mordido por isso.
🔴 **Falsificação do próprio harness** (AC3): numa amostra, sabotar o alvo e confirmar que o cenário
**ainda reprova** depois da otimização.
🔴 **Gate paralelo instável é PIOR que gate lento** — ensina a re-rodar em vez de investigar. Se
aparecer flakiness, **parar e reportar**, não "re-rodar para confirmar".

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
