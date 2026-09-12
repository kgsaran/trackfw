---
status: wip
date: 2026-09-09
req: "docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md"
squad: ""
---

# Roadmap: REQ nasce orfa porque criar REQ e criar roadmap sao dois comandos e o segundo se esquece

> Created: 2026-09-09 | Reestruturado: 2026-09-12 (absorção da REQ-2026-08-20) | Status: wip

## Context
REQ: docs/req/REQ-2026-09-09-req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos-e-o-segundo-se-esquece.md

**A tese única deste roadmap:** o vínculo entre artefatos de governança é hoje **inferido** — por
comparação de string, por proximidade de nome, por leitura de um campo entre dois que discordam. Todo
defeito abaixo é uma face disso. A correção é **escrever o vínculo no instante em que ele existe sem
ambiguidade**, e recusar quando não existe.

> 🔴 O bloco "Acceptance Criteria" abaixo estava **vazio** (`- [ ]`, `- [ ]`) porque foi gerado por
> `roadmap new --from-req`, que não consolida os ACs da REQ. **Esse é o AC7 deste próprio roadmap.**
> Preenchido à mão em 2026-09-12; quando o AC7 fechar, deixa de precisar.

## Acceptance Criteria
- [ ] AC1 — criar REQ e roadmap deixa de exigir dois comandos (atrito medido, não escolhido por gosto)
- [ ] AC2 — severidade endurecida por **data de corte**, não por contagem
- [ ] AC3 — grandfathering **visível** no relatório
- [ ] AC4 — decisão escrita sobre onde bloqueia (`validate` e/ou `push`)
- [ ] AC5 — re-triagem das REQs sem roadmap (o classificador heurístico não serve como escopo)
- [ ] AC6 — paridade nos 3 CLIs
- [ ] AC7 — `roadmap new --from-req` escreve o vínculo de volta na REQ e consolida os ACs
- [ ] AC8 — `roadmap move` com nome vazio ou não-exato **recusa e nomeia**
- [ ] AC9 — **uma** noção de "vinculada": frontmatter ou corpo, escolher e escrever
- [ ] AC10 — contra-braço: criar REQ **sem** roadmap continua possível quando é deliberado
- [ ] AC11 — **ADR** da precisão do vínculo branch↔roadmap, com a medição que falsifica o candidato 1
- [ ] AC12 — `findRoadmap` e `BranchSlugMatchesRoadmap` param de aceitar vazio e de escolher por proximidade
- [ ] AC13 — retomada legítima de roadmap concluído continua funcionando
- [ ] AC14 — a medição das 111 branches históricas vira **gate**

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat model
> Dependências: nenhuma. **Bloqueia toda implementação.**

### ML-0A — Threat model deste roadmap
**Status:** ✅ Concluído
**Arquivos afetados:** somente este roadmap.

---

#### Seção 1 — Completude da enumeração

Busca feita com `git grep` (não `ugrep` — `npm/src/validator/index.js` contém NUL bytes que o
`ugrep -I` omite silenciosamente sem imprimir "Binary file matches"). Escopo: código de produto nos
3 runtimes, excluindo arquivos `_test.go`, `*.test.js`, `test_*.py`.

**Padrões buscados:** `containsIgnoreCase` (Go) · `.includes(` + `[Ll]ower` (Node) · `name_lower in`
/ `lowered in` / `in f\.lower` / `in fname\.lower` (Python).

Resultado: **10 sítios em 4 famílias funcionais**, listados abaixo.

---

**Família 1 — `findRoadmap` (`roadmap move`, `roadmap show`)**

| # | Arquivo | Linha(s) | Padrão | Comportamento |
|---|---|---|---|---|
| 1 | `internal/generators/roadmap.go` | 632, 646 | `containsIgnoreCase(e.Name(), name)` | Primeiro resultado, sem verificação de ambiguidade |
| 2 | `npm/src/generators/roadmap.js` | 789, 801 | `.toLowerCase().includes(nameLower)` | Retorna TODOS; `moveRoadmap` bloqueia se >1; `showRoadmap` pega o primeiro |
| 3 | `pypi/trackfw/generators/roadmap.py` | 279, 288 | `name_lower in f.lower()` | Retorna TODOS; caller (linha 723) filtra basename-exato primeiro, fallback para todos, bloqueia se >1 |
| 4 | `pypi/trackfw/commands/roadmap.py` | 72, 77 | `name_lower in fname.lower()` | `roadmap show` apenas; substring puro, primeiro-vence (sem mutação) |

**Divergência comportamental para `roadmap move ""` (nome vazio):**
- Go: move o PRIMEIRO roadmap encontrado **silenciosamente** — o incidente que originou este roadmap (AC8)
- Node: bloqueia com lista de candidatos, sai 1 (correto)
- Python (generators/move): bloqueia com `ValueError("Múltiplos roadmaps encontrados")` (correto, caminho diferente)

ML-1C é portanto Go-only para o caso vazio. Os três têm o problema de substring para nomes não-vazios.

---

**Família 2 — `findREQ` (`req move`)**

| # | Arquivo | Linha | Padrão | Comportamento |
|---|---|---|---|---|
| 5 | `internal/generators/req.go` | 353 | `containsIgnoreCase(filepath.Base(path), name)` | Primeiro-vence |
| 6 | `npm/src/generators/req.js` | 136 | `path.basename(f).toLowerCase().includes(lower)` | Primeiro-vence (`.find()`) |
| 7 | `pypi/trackfw/generators/req.py` | 244 | `lowered in os.path.basename(path).lower()` | Primeiro-vence |

🔴 Nenhum dos três runtimes tem proteção "bloqueia se >1 matches" para REQ move — diferente do Node/Python para `roadmap move`. Todos os três movem a REQ errada silenciosamente.

Esta família **não estava na lista "já conhecidos" do roadmap**, mas causa é idêntica (substring sobre basename, primeiro-vence). Regra Dura de Causa Raiz: mesma causa, mesmo roadmap.

---

**Família 3 — `BranchSlugMatchesRoadmap` (`branch new`, `commit`, `validate`)**

| # | Arquivo | Linha | Padrão | Comportamento |
|---|---|---|---|---|
| 8 | `internal/validator/validator.go` | 2871 | `strings.Contains(normalizeBranchSlug(name), branchSlug)` | Define `matched = true` |
| 9 | `npm/src/validator/index.js` | 1513 | `.includes(branchSlug)` | Define `matched = true` |
| 10 | `pypi/trackfw/validator.py` | 1986 | `branch_slug in normalize_branch_slug(f)` | Define flag |

---

**Sítio 11 — Serve title fallback (declarado residual)**

`npm/src/serve/api_chain.js:149-155, 188-191` — `resolveRef()` indexa por
`n.title.toLowerCase().trim()` (primeiro-vence em colisão) e faz fallback por título se o basename
não resolve. Sem paralelo em Go ou Python. Inferência por nome, mas exata (não substring de filename).
Declarado na Seção 4.

---

**Fechamento da enumeração:** varredura por `git grep` sobre os padrões acima retorna exatamente os
10 sítios listados. Vacuidade verificada: `git grep -q 'branchSlug' npm/src/validator/index.js`
→ presente (confirma que o arquivo NUL não foi omitido).

---

#### Seção 2 — Threat model

O adversário aqui é o implementador apressado e o arquiteto otimista. Quatro caminhos concretos
esvaziam esta Wave sem quebrar nenhuma regra escrita:

**Escape 1 (risco mais alto): corrigir apenas os dois sítios "já conhecidos".**
O roadmap nomeia `findRoadmap` e `BranchSlugMatchesRoadmap`. Um agente que lê literalmente para aí,
corrige esses dois (ou seis com os espelhos), marca ✅, e o `trackfw barrier` passa — ele verifica
`✅ + [x]`, não a qualidade da análise. Os 4 sítios restantes (`findREQ` em 3 runtimes + Python
`_find_file`) sobrevivem com o mesmo defeito. O escape entra na lacuna entre o texto das Ações
("🔴 Não parar nesses três") e os critérios de aceite — que não nomeiam `findREQ` explicitamente.
Pré-emissão: os MLs 1C e 3A devem listar `findREQ` (Family 2) nos arquivos afetados, não apenas
`findRoadmap`.

**Escape 2: declarar `findREQ` fora do escopo como "superfície diferente".**
Argumento: `findREQ` é lookup, não vínculo; `req move` é menos crítico. Fechado pela Regra Dura de
Causa Raiz (CLAUDE.md): "é superfície diferente" está listado explicitamente como fundamento inválido
para separar. A causa é idêntica em mecanismo e em ADR governante. Esta seção fecha o escape em
escrito — qualquer agente que tentar separar tem este parágrafo como evidência contrária.

**Escape 3: o fixture das 111 branches é regenerado no mesmo PR que muda o matcher (ML-3C).**
O corpus pina o comportamento atual. A movimentação sem atrito quando o matcher aperta é regenerar o
expected output — é uma escrita, não uma falha de teste. O roadmap diz "nunca afrouxar para caber",
mas o arquivo de fixture É gravável pelo mesmo ML que muda o matcher. O escape entra se o diff do
fixture não for revisado contra uma saída humano-legível. O gate de ML-0A âncora na contagem de sítios
ativos, não só no `make quality` verde.

**Escape 4: escolher boundary matching (candidato 1) e fechar o AC11 com um ADR.**
O AC11 é satisfeito estruturalmente se uma escolha estiver documentada. Um implementador que lê
apenas a REQ absorvida poderia escolher boundary matching (o único candidato nomeado ali) sem ler o
issue #273 nem a medição. Boundary matching é estritamente mais restritivo que `Contains` — por
aritmética, não amostra — e **não pode** corrigir nenhum falso-negativo; o issue #273 mostra que o
falso-negativo já está ativo em produção (~9% das branches governadas). Um ADR que documenta boundary
satisfaz AC11 sem satisfazer a intenção. Pré-emissão: o AC11 deve exigir que o ADR documente **as
duas direções** com evidência medida, não só o candidato escolhido.

---

#### Seção 3 — Falsificação nas duas direções

**Direção 1 (folgado demais — falso-positivo): matcher atual aceita par indevido**

*Família 1+2 (`findRoadmap` / `findREQ`), sítios 1-7:*
- Onde entra: slug curto (`gate`, `req`, `python`, `windows`) bate no primeiro de múltiplos roadmaps/REQs cujo filename contém esse token
- Qual gate captura: nenhum captura hoje — `roadmap move gate wip` move o primeiro roadmap silenciosamente em Go; em Node/Python para roadmap move bloqueia se >1; para req move NENHUM bloqueia
- Medido: 20 slugs genéricos curtos → 326 matches sob `Contains`, 266 sob boundary (−18%); `fix/roadmap` bate em 86% do corpus
- Falso-positivo em `roadmap move` / `req move` muta o artefato errado — corrupção silenciosa de dados de governança

*Família 3 (`BranchSlugMatchesRoadmap`), sítios 8-10:*
- Onde entra: branch `feat/gate` passa `validate` porque o token `gate` aparece no nome de qualquer roadmap que contenha "gate" — mesmo sem relação de governança real
- Qual gate captura: nenhum hoje; `branch_has_wip_roadmap` passa para branches órfãs que têm um slug curto genérico
- Escala: issue #273, 64 roadmaps — `req` bate em 9/64 sob `Contains` vs 8/64 sob boundary; os números absolutos NÃO são transferíveis entre corpora (nomes longos e descritivos no fork); o que transfere é a comparação entre relações

**Direção 2 (restrito demais — falso-negativo): matcher atual rejeita par legítimo**

*Família 3 (`BranchSlugMatchesRoadmap`), sítios 8-10:*
- **DEFEITO ATIVO, MEDIDO EM PRODUÇÃO** — não hipótese futura
- Onde entra: slug da branch NÃO aparece como substring do nome normalizado do roadmap, mesmo quando a branch governa legitimamente aquele roadmap. Os dois nomes descrevem o mesmo trabalho por caminhos diferentes (branch pelo trabalho; roadmap pelo título da REQ)
- Instância medida (issue #273, commit `c60a7f8`): `feat/adrs-retroativas-da-divida-do-acervo` reprovada contra `ROADMAP-2026-09-05-divida-de-governanca-do-acervo-...`; o slug `adrs-retroativas-da-divida-do-acervo` NÃO é substring de `divida-de-governanca-do-acervo`. Usuário teve de renomear a branch para o slug caber dentro do nome do roadmap
- Escala: ~9% das branches governadas (22 branches, 2 rejeitadas, excluída 1 mal-nomeada) no corpus do autor do issue
- Qual gate captura hoje: nenhum — a regra rejeita e o usuário não tem caminho de saída sem renomear ou intervenção manual de git
- O falso-negativo em Family 1+2 é menos severo: "não encontrado" é um erro visível; o usuário pode fornecer um nome mais preciso

**Corroboração cruzada de duas medições independentes:**
Duas equipes, dois corpora, dois métodos:
1. Este repo — 185 roadmaps + 111 branches históricas, medido sobre os binários e o corpus real: `Contains`=109/111, boundary=109/111 (0 regressão); 20 slugs curtos: Contains=326, boundary=266
2. Fork externo (issue #273) — 64 roadmaps, relações reimplementadas a partir de leitura de código (não binários): mesma conclusão qualitativa

Argumento decisivo do issue #273 que a medição 1 não dá diretamente: boundary é subconjunto estrito de `Contains` por definição — logo **não pode** corrigir nenhum falso-negativo, apenas criar mais. Isso é aritmética, não amostra. O problema não é o limiar de `Contains`; é a relação: substring exige que um nome esteja literalmente dentro do outro, enquanto os dois nomes descrevem o mesmo trabalho por perspectivas diferentes.

**Deadlock de bootstrap (risco terminal):**
Um falso-negativo em Family 3 bloqueia `trackfw commit` — o único caminho de commit neste repo
(`git commit` cru é bloqueado pelo guard). Consequência: nenhum novo trabalho pode ser mergeado,
incluindo o fix do próprio matcher. A cadeia é: branch nomeada corretamente para o trabalho →
validator rejeita → `trackfw commit` falha → `trackfw push` não existe → fix não pode ser mergeado
→ equipe bloqueada até intervenção manual de git. Este bloqueio já acontece (~9%), não é hipotético.

---

#### Seção 4 — Residual declarado

Este design aceita explicitamente não cobrir:

1. **Node `resolveRef` title fallback** (`npm/src/serve/api_chain.js:189-190`): inferência por título
   de roadmap (exata, não substring de filename), first-wins em colisão, somente Node, somente serve
   board. O serve é UI de leitura — nenhuma mutação de artefato. Declarado, não negligenciado.

2. **Assimetria de proteção Family 1 vs Family 2**: Node e Python bloqueiam `roadmap move` em >1
   matches; nenhum runtime bloqueia `req move` em >1 matches. Esta lacuna de protocolo é observada e
   registrada; equalizar o protocolo entre famílias é um ajuste independente da troca do matcher.

3. **Colisões em filesystem case-insensitive (macOS HFS+, Windows NTFS)**: dois roadmaps nomeados
   `REQ-A.md` e `req-a.md` seriam o mesmo inode. O matcher (qualquer candidato) não pode distingui-los
   — requer gate de lint independente, fora do escopo deste roadmap.

4. **Escritas concorrentes**: dois agentes chamando `roadmap move` / `req move` simultaneamente no
   mesmo arquivo. Entre o match e o `os.Rename`, o arquivo pode ter sido movido. É problema de
   lock/fencing, não de matcher.

5. **Calibração do candidato token-overlap**: sobreposição de tokens (≥ 2 tokens de ≥ 3 caracteres)
   foi a única das opções do issue #273 que fechou ambas as direções naquele corpus. Sem calibração
   contra os 127 roadmaps deste repo e sem gate. É alvo de medição para o AC11/ADR, não decisão
   declarada.

---

**Critérios de aceite:**
- [x] As quatro seções respondidas com evidência, não asserção de uma linha
- [x] Nenhuma linha de implementação escrita neste ML

**Gate da wave:**
```bash
# Gate ML-0A — Fechamento da enumeração de sítios de inferência por substring.
# Falha se um sítio DESAPARECER (função removida ou renomeada) sem que o ML
# correspondente seja marcado concluído e EXPECTED decrementado.
#
# 🔴 LIMITE MEDIDO deste gate (auditoria do arquiteto, 2026-09-12): 8 dos 10 checks
# rastreiam o NOME DA FUNÇÃO, não o padrão defeituoso. Corrigir um sítio EM LUGAR —
# manter `find_req`/`_find_file`/`branchSlugMatchesRoadmap` e trocar substring por
# casamento exato — deixa este gate VERDE. Ele prova que o sítio existe, não que ele
# ainda é defeituoso.
#
# Só `internal/generators/roadmap.go` e `internal/generators/req.go` rastreiam o padrão
# real (`containsIgnoreCase`) e detectariam a correção em lugar.
#
# Quem prova a correção são os critérios de aceite de cada ML (falsificação: nome vazio
# ⇒ erro; contra-braço: nome exato ⇒ move). Este gate é de ENUMERAÇÃO, não de correção —
# não o use como evidência de que um sítio foi consertado.
# Usar git grep (não ugrep): npm/src/validator/index.js tem NUL bytes que o ugrep omite.
cd "$(git rev-parse --show-toplevel)" || exit 1

# Vacuidade: confirma que git grep enxerga npm/src/validator/index.js (NUL não omitido).
if ! git grep -q 'branchSlug' npm/src/validator/index.js 2>/dev/null; then
  echo "GATE FALHOU: git grep não enxerga npm/src/validator/index.js" >&2; exit 1
fi

EXPECTED=10
fail=0
# Cada entrada: "arquivo:padrão" — um sítio documentado na Seção 1.
checks=(
  "internal/generators/roadmap.go:containsIgnoreCase"
  "internal/generators/req.go:containsIgnoreCase"
  "internal/validator/validator.go:BranchSlugMatchesRoadmap"
  "npm/src/generators/roadmap.js:findRoadmapMatches"
  "npm/src/generators/req.js:findREQ"
  "npm/src/validator/index.js:branchSlugMatchesRoadmap"
  "pypi/trackfw/generators/roadmap.py:_find_roadmap_matches"
  "pypi/trackfw/generators/req.py:find_req"
  "pypi/trackfw/commands/roadmap.py:_find_file"
  "pypi/trackfw/validator.py:branch_slug_matches_roadmap"
)
found=0
for check in "${checks[@]}"; do
  file="${check%%:*}"; pattern="${check##*:}"
  if git grep -q "$pattern" "$file" 2>/dev/null; then
    found=$((found + 1))
  else
    echo "  sítio removido ou renomeado: $file não contém '$pattern'" >&2
    echo "  → atualize EXPECTED e marque o ML que corrigiu este sítio como ✅" >&2
    fail=1
  fi
done
if [ "$found" -ne "$EXPECTED" ] || [ "$fail" -ne 0 ]; then
  echo "GATE FALHOU: $found/$EXPECTED sítios confirmados (esperado $EXPECTED)" >&2; exit 1
fi
echo "Gate ML-0A: $found/$EXPECTED sítios de inferência por substring confirmados — enumeração fechada."
```

---

## Wave 1 — A fonte de verdade do vínculo
> Dependências: Wave 0. 🔴 **Precede tudo**: enquanto houver duas noções de "vinculada", toda
> contagem e todo gate deste roadmap medem coisas diferentes.

### ML-1A — **AC9** — uma noção de "vinculada"
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/validator/validator.go`, `npm/src/validator/index.js`,
`pypi/trackfw/validator.py`, e todo comando que **escreve** REQ (`req new`, `roadmap new --from-req`,
`roadmap move`) nos 3 runtimes.
**Contexto medido:** `roadmap: ""` no frontmatter → 27 REQs; `"no linked Roadmap"` no `validate` →
57. Duas contagens, duas fontes. Vincular 4 REQs pelo frontmatter **não** as tirou da lista do
`validate`. Mesma causa do issue **#306** (`req list` lê `status:` do corpo, `validate` lê o
frontmatter).
**Ações:**
1. **Decidir e registrar** qual é a fonte de verdade. Presunção a confirmar: frontmatter, por
   consistência com `status:` e com o `serve`. 🔴 **Decisão, não inferência** — escrever o porquê.
2. Decidir o que fazer quando os dois divergem: silêncio, aviso ou violação. **Corrigir a leitura sem
   decidir isto troca um defeito por outro.**
3. Todo comando que escreve REQ escreve **os dois**, ou o gerador para de emitir o marcador de corpo
   como placeholder vazio — que é o que cria a órfã silenciosa.
**Critérios de aceite:**
- [ ] Fonte de verdade escrita no artefato, com o motivo
- [ ] Frontmatter preenchido + corpo vazio ⇒ comportamento decidido em (2)
- [ ] Os dois preenchidos e **divergentes** ⇒ idem
- [ ] Os dois iguais ⇒ passa (contra-braço)
- [ ] Paridade nos 3 CLIs, com diff de saída real
**Reconciliação:** cada teste novo declara, em uma frase, qual conclusão deste ML ele afirma.

### ML-1B — **AC7** — `--from-req` fecha o laço
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/roadmap.go`, `npm/src/generators/roadmap.js`,
`pypi/trackfw/generators/roadmap.py`.
**Contexto medido:** `roadmap new --from-req` gera os MLs a partir dos ACs, mas **não** grava
`roadmap:` na REQ e deixa o bloco "Acceptance Criteria" do roadmap **vazio**. O `roadmap move` já faz
o sync (`✓ synced REQ ... → roadmap`) — 🔴 **a capacidade existe num comando e falta no outro.**
**Ações:**
1. `--from-req` grava o vínculo de volta na REQ, no formato que o `validate` de fato lê (decidido no
   ML-1A). Uma operação, dois lados do elo.
2. O bloco consolidado de ACs do roadmap deixa de sair vazio quando a REQ tem ACs.
**Critérios de aceite:**
- [ ] Criar REQ + roadmap pelo caminho integrado ⇒ `req_has_roadmap` **não** dispara (falsificação)
- [ ] Bloco de ACs do roadmap reflete os ACs da REQ
- [ ] Paridade nos 3 CLIs

### ML-1C — **AC8 + AC12 (parte 1 de 2)** — recusar o nome vazio
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/roadmap.go` (`findRoadmap`, linha ~632),
`npm/src/generators/roadmap.js`, `pypi/trackfw/generators/roadmap.py`.
🔴 **Este ML vem ANTES do ML-3A por decisão de ordem:** recusar nome vazio é **estritamente aditivo**
— não existe consumidor legítimo de `roadmap move ""`. Apertar o matcher compartilhado (ML-3A) não é
aditivo e pode paralisar quem depende dele.
**Contexto medido:** `containsIgnoreCase(nome, "")` é sempre verdadeiro ⇒ `roadmap move ""` retorna o
**primeiro** roadmap do primeiro diretório de estado e o move. Aconteceu de verdade em 2026-09-12,
por variável de shell vazia.
**Ações:**
1. Nome vazio ⇒ **erro que nomeia o problema**, nos 3 CLIs.
2. Nome que não casa exatamente ⇒ recusar e **listar os candidatos**, em vez de escolher um.
   🔴 Decisão do KG de 2026-08-29: *"controle que não reconhece rejeita e avisa, em vez de adivinhar"*.
**Critérios de aceite:**
- [ ] Nome vazio ⇒ erro, nenhum arquivo movido (falsificação)
- [ ] Nome exato ⇒ move (contra-braço)
- [ ] Nome parcial ambíguo ⇒ recusa nomeando os candidatos
- [ ] Paridade nos 3 CLIs, mensagens byte-idênticas

---

## Wave 2 — A decisão arquitetural
> Dependências: Wave 1 (a fonte de verdade precisa estar decidida).

### ML-2A — **AC11** — ADR da precisão do vínculo branch↔roadmap
**Status:** ⬜ Pendente
**Arquivos afetados:** novo ADR em `docs/adr/`.
**Insumo obrigatório — a medição já feita em 2026-09-12, contra 185 roadmaps e 111 branches reais:**

| | substring | fronteira |
|---|---|---|
| casamentos das 111 branches históricas | 109 | **109** |
| regressão | — | **0** |
| 20 slugs curtos genéricos | 326 | 266 (−18%) |

```
fix/roadmap   substring 159   fronteira 159   ← 86% do corpus, INALTERADO
fix/guard              16              14
fix/ci                 28               2     ← só aqui funciona
```

🔴 **A medição falsifica o candidato 1 da REQ absorvida**, que o texto dela chamava de
*"provavelmente suficiente"*. Fronteira só remove palavra-dentro-de-palavra; quando o token curto é
uma palavra legítima do nome, ela não ajuda.
**Ações:**
1. Registrar a decisão **com os candidatos descartados e o porquê**, citando a medição.
2. Responder: o vínculo branch↔roadmap passa a ser **escrito** (o `branch new` sabe qual roadmap está
   em `wip` no instante em que cria a branch), inferido com regra mais estrita, ou os dois?
3. 🔴 **Risco dominante herdado da REQ absorvida:** este portão é atravessado por **todo** `branch
   new`, `commit` e `ship`. Falso-positivo aqui **paralisa**, não irrita. A ADR declara como o risco é
   contido.
**Critérios de aceite:**
- [ ] ADR escrita, com candidatos descartados e a medição citada
- [ ] Decisão explícita sobre escrever vs. inferir
- [ ] Contenção do risco de falso-positivo declarada

---

## Wave 3 — O matcher compartilhado
> Dependências: **ML-2A** (não mexer no matcher antes da ADR). 🔴 Enquanto esta wave estiver aberta,
> frente paralela deve usar o `trackfw` **instalado**, não `bin/trackfw` reconstruído desta árvore.

### ML-3A — **AC12 (parte 2 de 2)** — `BranchSlugMatchesRoadmap`
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/validator/validator.go:2864`, espelhos em `npm/src/validator/`,
`pypi/trackfw/validator.py`. Consumidores: `validate`, `branch new` (`commands/branch.go:100`),
`commit` (`commands/commit.go:103`).
**Ações:** implementar o que a ADR do ML-2A decidiu.
**Critérios de aceite:**
- [ ] Comportamento decidido na ADR, nos 3 CLIs
- [ ] 🔴 O gate do ML-2A da REQ antiga fixa o comportamento **atual** — atualizar junto, **nunca
      afrouxar para caber**

### ML-3B — **AC13** — retomada legítima continua funcionando
**Status:** ⬜ Pendente
**Ações:** cenário provando que retomar trabalho de um roadmap em `done/` continua permitido.
**Critérios de aceite:**
- [ ] Cenário de retomada legítima passa (contra-braço do ML-3A)
- [ ] Cenário de slug curto espúrio reprova (braço)

### ML-3C — **AC14** — a medição vira gate
**Status:** ⬜ Pendente
**Arquivos afetados:** novo `scripts/check-roadmap-slug-matching.sh`, wired no `Makefile`.
**Ações:** o corpus de 185 roadmaps e as 111 branches históricas viram fixture. Mudança no matcher que
altere o veredito de qualquer das 111 reprova.
**Critérios de aceite:**
- [ ] Gate roda nos 3 runtimes e compara saídas reais
- [ ] Falsificação: mutação no matcher ⇒ gate reprova
- [ ] Contra-braço: matcher correto ⇒ gate passa

---

## Wave 4 — Prevenção e severidade
> Dependências: Wave 1. Independente da Wave 3.

### ML-4A — **AC1 + AC10** — um comando, com contra-braço
**Status:** ⬜ Pendente
**Ações:** criar REQ e roadmap deixa de exigir dois comandos. 🔴 **Medir o atrito de cada forma**
(flag em `req new`, prompt, `req new` chamando `--from-req`) — não escolher por gosto. **E** criar
REQ sem roadmap continua possível quando é deliberado: *atrito onde é engano, caminho livre onde é
intenção.*
**Critérios de aceite:**
- [ ] Caminho integrado produz REQ **não órfã** (falsificação)
- [ ] Caminho deliberado sem roadmap continua disponível (contra-braço)
- [ ] Paridade nos 3 CLIs

### ML-4B — **AC2 + AC3** — corte por data, grandfathering visível
**Status:** ⬜ Pendente
**Ações:** REQ criada a partir de `<corte>` sem roadmap ⇒ **error**; anterior ⇒ warning. Corte
declarado no artefato. O relatório diz **quantas** estão isentas e **desde quando**.
🔴 *Isenção que não se vê vira permanente.* E inverter a severidade sem corte faz o `validate` falhar
em 32 REQs — alguém configura `lenient` e perdemos a regra **e** o aviso.
**Critérios de aceite:**
- [ ] REQ pós-corte sem roadmap ⇒ error; pré-corte ⇒ warning (os dois braços)
- [ ] Relatório mostra contagem de isentas e a data de corte
- [ ] Paridade nos 3 CLIs

### ML-4C — **AC4** — onde bloqueia
**Status:** ⬜ Pendente
**Ações:** decisão escrita: só `validate`, ou também `push`. 🔴 O `push` hoje exige REQ+roadmap **da
branch**, não de toda REQ — são coisas diferentes e a decisão precisa dizer **qual** muda.
**Critérios de aceite:**
- [ ] Decisão escrita, com o impacto de cada opção

---

## Wave 5 — Fechamento
> Dependências: Waves 1–4.

### ML-5A — **AC5** — re-triagem das REQs sem roadmap
**Status:** ⬜ Pendente
**Ações:** quantas **legitimamente** não têm roadmap (decisão pura, fechada sem implementação).
🔴 O classificador heurístico da REQ **não serve** como escopo: ele casa palavras-chave e testa esse
ramo primeiro, então REQ que tenha as duas coisas cai em "decisão".
**Critérios de aceite:**
- [ ] Contagem por leitura, não por heurística, com a lista

### ML-5B — **AC6** — paridade e fechamento
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [ ] `make quality` verde **e CI verde**
- [ ] `trackfw validate` sem violation nova

---

## Escopo negativo

- **Não** criar roadmap automático para as REQs órfãs existentes — roadmap vazio gerado em massa é
  pior que REQ órfã: **parece cobertura e não é.**
- **Não** endurecer `req_has_adr` no mesmo movimento — 103 avisos, causa distinta, ADR de ratchet
  própria para o acervo.
- **Não** remover a aceitação de `done/` no `branch_has_wip_roadmap`. Ela existe por um motivo
  (retomar trabalho concluído) e a `REQ-2026-07-26` a decidiu. Este roadmap ajusta a **precisão**, não
  a política.
