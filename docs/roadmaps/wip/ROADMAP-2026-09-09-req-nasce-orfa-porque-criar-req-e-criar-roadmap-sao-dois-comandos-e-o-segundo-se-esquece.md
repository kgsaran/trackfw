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

**Gates da wave:**
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
**Status:** ✅ Concluído
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
- [x] Fonte de verdade escrita no artefato, com o motivo
- [x] Frontmatter preenchido + corpo vazio ⇒ comportamento decidido em (2)
- [x] Os dois preenchidos e **divergentes** ⇒ idem
- [x] Os dois iguais ⇒ passa (contra-braço)
- [x] Paridade nos 3 CLIs, com diff de saída real
**Reconciliação:** cada teste novo declara, em uma frase, qual conclusão deste ML ele afirma.

**Decisões implementadas:**
- **Fonte de verdade:** `extractRefPath(content, "roadmap")` — frontmatter `roadmap:` primeiro, fallback para corpo `Roadmap:`, exige sufixo `.md`. Motivo: consistência com `status:`, `adr:` e com o `serve`; eliminou 21 falsos positivos de órfã (REQs com frontmatter preenchido mas corpo vazio/placeholder).
- **Divergência (dois campos preenchidos e basenames distintos):** `req_roadmap_sync: "warning"` — não bloqueia o usuário; sinaliza drift para reparo.
- **`req list status:` (issue #306):** `parseREQMeta`/`parseREQStatus`/`parse_req_status` reescritos frontmatter-first em todos os 3 CLIs.
- **Falsify S25 (braço baseline):** `ROADMAP_CYCLE_SCRIPT_FROM_REQ_S25` suprime `req_has_roadmap` no sandbox S25 — o placeholder `Roadmap: none` era intencional para não disparar req_has_roadmap; extractRefPath o rejeita agora; a regra é suprimida no baseline para provar só a ausência de S25_PATTERN (ref_targets_exist), que é o seam do Cenário 25.

**Achado: `NewRoadmapFromREQ` não escreve backlink na REQ (ML-1B, não ML-1A):**
`roadmap new --from-req` cria o roadmap com `req:` preenchido mas NÃO grava `roadmap:` de volta na
REQ. `syncREQReferences` (em `roadmap move`) só atualiza REQs que já têm frontmatter `roadmap: != ""`
— portanto REQs com `roadmap: ""` nunca recebem o backlink via esse fluxo. Este é o defeito raiz
do AC7 (ML-1B), mesma causa, mesmo roadmap. Registrado no vault.

### ML-1B — **AC7** — `--from-req` fecha o laço
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · **e o AC do ML-1A não está implementado pela regra**
**Arquivos afetados:** `internal/generators/roadmap.go`.
⚠️ **Os caminhos `npm/src/…` e `pypi/trackfw/…` que este ML listava NÃO EXISTEM desde a v8.0.0** —
implementação única em Go. Corrigido em 2026-09-26.
**Contexto medido:** `roadmap new --from-req` gera os MLs a partir dos ACs, mas **não** grava
`roadmap:` na REQ e deixa o bloco "Acceptance Criteria" do roadmap **vazio**. O `roadmap move` já faz
o sync (`✓ synced REQ ... → roadmap`) — 🔴 **a capacidade existe num comando e falta no outro.**
**Ações:**
1. `--from-req` grava o vínculo de volta na REQ, no formato que o `validate` de fato lê (decidido no
   ML-1A). Uma operação, dois lados do elo.
2. O bloco consolidado de ACs do roadmap deixa de sair vazio quando a REQ tem ACs.
**Critérios de aceite:**
- [x] Criar REQ + roadmap pelo caminho integrado ⇒ `req_has_roadmap` **não** dispara (falsificação)
- [x] Bloco de ACs do roadmap reflete os ACs da REQ
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** — implementação única em Go

### ML-1C — **AC8 + AC12 (parte 1 de 2)** — recusar o nome vazio
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · 🔴 **o defeito era maior: o nome EXATO movia o arquivo errado**
**Arquivos afetados:** `internal/generators/roadmap.go` — `containsIgnoreCase` está em **`:814`** e
é usada em **`:791`** e **`:805`** (a linha `~632` do texto original está obsoleta; medido em 2026-09-26).
⚠️ **Os caminhos `npm/src/…` e `pypi/trackfw/…` NÃO EXISTEM desde a v8.0.0.**
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
**Sítios efetivamente fechados (entrega de 2026-09-26, pendente de auditoria):** `findRoadmap`
(ponto único de enumeração `roadmapCandidateFiles`, cobrindo `flat` **e** `by_agent`), `findREQ`
(`req.go` — Família 2 do censo, mesma causa, Regra Dura) e o nome vazio de `ShowRoadmap`. Contrato
documentado em `docs/cli-parity.md` (seção nova, anotada). `resolveBarrierRoadmap` medido como
negativo (resolve por filename exato). `BranchSlugMatchesRoadmap` permanece no `ML-3A` — e o braço
do **slug vazio** (casa qualquer roadmap) tem de estar na lista dele.
**Critérios de aceite:**
- [x] Nome vazio ⇒ erro, nenhum arquivo movido (falsificação)
- [x] Nome exato ⇒ move (contra-braço)
- [x] Nome parcial ambíguo ⇒ recusa nomeando os candidatos
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0**

---

#### 🔴 Auditoria do ML-1C — reproduzi o antes e o depois, e havia um TERCEIRO braço

Comparei o binário de `HEAD` contra o corrigido, sobre fixture com um par de prefixo:

```
ANTES (HEAD)
  roadmap move ""                          → ✓ moved …-alvo-ML-1B.md   rc=0
  roadmap move "ROADMAP-2026-07-19-alvo"   → ✓ moved …-alvo-ML-1B.md   rc=0   ← NOME EXATO, ARQUIVO ERRADO

DEPOIS
  ""                          → Error: roadmap name is required …            rc=1   nada movido
  "ROADMAP-2026-07-19-alvo"   → ✓ moved ROADMAP-2026-07-19-alvo.md           rc=0   ← o alvo
  "2026-09-26"  (ambíguo)     → Error: multiple roadmaps match … be specific rc=1   nada movido
  "um"          (único)       → ✓ moved ROADMAP-2026-09-26-um.md             rc=0
```

🔴 **O terceiro braço não estava no meu handoff e nenhuma inspeção de código o previa:** quando o
stem é **prefixo de um irmão maior** (`…-governance` ⊂ `…-governance-ML-1B.md`), o substring casa os
dois e **a ordem de varredura decide**. É a forma que o usuário digita **achando que é inequívoca** —
e este repositório tem **3 pares assim nos roadmaps e 3 nos REQs**. Confirmei:
`ROADMAP-2026-07-19-global-adrs-governance.md` convive com `-ML-1B.md` e `-ML-2B.md`.

⚠️ **E é o achado que quase transformou o fix numa paralisação:** recusar ambiguidade **sem**
precedência de casamento exato tornaria esses 3 nomes **inendereçáveis** — trocaria mover-o-errado
por não-mover-nenhum. A ordem entregue é **exato (com ou sem `.md`) → parcial único → recusa
nomeando candidatos**.

### A régua é cardinalidade, nunca comprimento — e os números mostram por quê

| query | roadmaps (228) | REQs (231) |
|---|---|---|
| nome completo sem `.md` | 225 únicos · **3 ambíguos** | 228 · **3** |
| 20 primeiros chars | 151 únicos · **77 ambíguos** | 206 · 25 |
| **vazio** | **228 casados** | **231 casados** |

Os **77** são casamentos que hoje **escolhem em silêncio**. E proibir "parcial" mataria os **151** —
o uso diário, o meu inclusive. A mudança **não acrescenta nenhuma recusa** para nome completo e
converte 3 escolhas arbitrárias em resolução correta.

### A varredura de mesma classe achou o sítio PIOR, e um ponto único que faltava

- **`findREQ`** (`req.go`) tem a mesma causa — e é **pior**, porque `MoveREQ` também **reescreve o
  `status:` dentro do arquivo**: com nome vazio, os **231** casavam e o conteúdo do errado era
  alterado. Entrou pela Regra Dura.
- 🔴 **Havia DUAS cópias do laço primeiro-vence** em `findRoadmap` (`flat` e `by_agent`). O corpus
  deste projeto é `flat`, logo **meia-correção ficaria verde no `make quality`** — exatamente a
  família do #396. Ele criou o ponto único `roadmapCandidateFiles` e testou o ramo `by_agent`
  separadamente.
- **`ShowRoadmap`** com nome vazio imprimia o arquivo.
- **Negativos medidos e escritos:** `resolveBarrierRoadmap`, o glob de `NewADR`, e os `List*` — nenhum
  seleciona por nome do usuário.

### Autorrefutação dele, pega antes do commit

A primeira redação da seção nova de `docs/cli-parity.md` afirmava as 4 regras também para
`roadmap show`. **Falso, e medido:** `show` recusa como ambíguo (glob próprio, sem precedência de
exato) e imprime candidatos em **stdout**. Contrato corrigido para dizer que `show` compartilha
**só** a recusa de nome vazio. 🔴 **Contradição interna pega antes do commit, não depois** — é a Regra
Dura de Reconciliação funcionando.

### O que fica para o ML-3A, e entra na lista dele

`validator.BranchSlugMatchesRoadmap` (`validator.go:3493`) é **mesma classe**: com `branchSlug`
**vazio**, casa **qualquer** roadmap (`matched = true` vaziamente). Não tocado aqui por escopo (D4 do
ADR manda o matcher vir depois, em modo aditivo) — **mas o braço do slug vazio tem de entrar no
`ML-3A`**.

⚠️ **Residual que é meu:** `check-symlink-privilege-guard` **não viu** o arquivo de teste novo
(enumera por `git ls-files`; medido `grep -c` = 0 no log, com rc=0 — **invisibilidade, não
segurança**). Ele fechou por inspeção direta (0 ocorrências de `os.Symlink`). **Segunda passada
pós-commit obrigatória.**

`make quality` → **rc 0**, 338 OK, 0 FAIL, `Error [0-9]` = 0.

#### 🔴 Auditoria do ML-1B — dois achados que são decisão minha, e eu decidi os dois

**O fix funciona, medido no ciclo inteiro:**

```
ANTES   roadmap new --from-req …    REQ: roadmap: ""      validate: ✗ has no linked Roadmap
DEPOIS  ✓ created … + ✓ linked REQ-….md → docs/roadmaps/backlog/ROADMAP-….md
        REQ: roadmap: "docs/roadmaps/backlog/…md"  e  Roadmap: docs/…md
        ciclo completo: move wip → ✓ synced → validate rc=0
```

`make quality` **rc 0**, 338 OK, 0 FAIL, `Error [0-9]` = 0.

### 🔴 R1 — a decisão do ML-1A existe, e a regra NÃO a implementa. Confirmei.

O `ML-1A` registrou a fonte de verdade como `extractRefPath` (frontmatter-first, **exige `.md`**). O
comentário de `validator.go:2217` **afirma isso**. O código de `:2236` usa `extractFrontmatterField`,
que aceita **qualquer valor não-vazio**. Medi:

```
REQ com  roadmap: none  →  validate: 0 violações de "no linked Roadmap"
```

🔴 **O comentário mente, e o AC do `ML-1A` foi marcado por uma decisão que o código não cumpre.** É a
classe exata que esta REQ persegue. **Decisão: ML novo NESTA REQ** (`ML-1D`), pela Regra Dura — não
issue, não REQ nova.

⚠️ **E ele não explorou a fraqueza:** o valor que o fix grava é caminho relativo **resolvível**, e o
teste afirma `os.Stat` sobre ele. Passar pela regra fraca seria fácil e teria sido invisível.

### 🔴 R3 — divergência com ADR `Accepted`, declarada em vez de assumida. Emendei a ADR.

A `ADR-2026-07-31` Decisão 3 diz: *"a seção consolidada é gerada como placeholder a preencher, **não**
como agregação automática"*. O AC7 pede o oposto. Ele **implementou conforme a REQ, restringiu ao
`--from-req`**, e **declarou a divergência** em vez de tomar precedência por conta própria.

**Auditei e emendei a ADR**, com a razão: a Decisão 3 fala de **reagregação** (*"o gerador não tem
como reagregar depois que o arquivo passa a ser editado à mão"*), e o `--from-req` **semeia uma vez,
na criação**, quando os MLs **são** os ACs da REQ — não há duas fontes a divergir. A emenda mantém o
placeholder no template simples e **proíbe** reagregação posterior.

🔴 **O comportamento certo aqui foi o dele, não o meu handoff:** eu não previ a ADR, e ele parou para
declarar em vez de seguir.

### R2 — `syncREQReferences` não servia, e a razão é estrutural

Ela **descobre** REQs cujo `roadmap:` **já aponta** para o basename movido
(`if fmVal == "" || basename != roadmapBasename { continue }`) — uma REQ com `roadmap: ""` cai no
**primeiro** ramo e é descartada **por construção**. Ele reusou o **escritor** (`rewriteREQRoadmapRefWith`),
não a descoberta, com o critério de sobrescrita como **predicado**. É o AC5 da REQ-2026-08-31
aplicado: uma implementação, dois consumidores.

### R4 — o fix ficou INERTE na primeira versão, e sem erro nenhum

`NewRoadmapFromREQ` montava o `req:` dentro da **string do Body** e chamava `NewRoadmapFromContent`
**sem `REQPath`**. A primeira versão compilou, rodou e produziu **saída byte-idêntica à de antes**.
🔴 **Fix inerte que não falha é pior que fix que quebra** — está na nota de vault.

### Duas armadilhas de instrumento, e a segunda ele não previu

O **Cenário 24** fixa o bloco de ACs como literal e exige **exatamente 2 ocorrências**. E o
**Cenário 25 fixa a LINHA DE ARGUMENTOS do `fmt.Sprintf`** — acrescentar `acBlock` matou o
`corrupt_literal`, o `chunk_1` morreu no meio, e o log cuspiu **~40 rótulos "AUSENTE" por UM
literal**. Primeira barreira **rc=2**. É a terceira vez que o `s182` morde nesta campanha, e ele
escreveu um checador dos 8 literais que `scripts/` fixa contra `roadmap.go`.

### Decisão minha sobre o #439, e é uma CORREÇÃO de atribuição

Ele propôs que `req move` deixando o `req:` do roadmap defasado é mesma causa **desta** REQ. 🔴 **Ele
está certo e eu estava errado.** Absorvi o #439 na `REQ-2026-09-25` (rastreabilidade/estado) por
**mesmo sintoma** — passivo no baseline. Mas a **causa** é esta: *"o comando conhece o vínculo e não
o escreve"*, e `syncREQReferences`, o sincronizador desta REQ, é literalmente a função que falta do
outro lado.

⚠️ **O PR #440 já está mergeado**, então a correção de atribuição é um PR próprio, não uma edição
silenciosa. Registro aqui e corrijo na REQ-2026-09-25 em seguida.

## Wave 1 (cont.) — os sítios que a varredura do ML-1B achou

### ML-1D — 🔴 A regra `req_has_roadmap` não implementa a decisão do ML-1A
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · **1 REQ mudou de veredito, e é genuína**

**Medido:** `roadmap: none` satisfaz a regra. `validator.go:2236` usa `extractFrontmatterField`
(qualquer valor não-vazio) enquanto o comentário de `:2217` afirma usar `extractRefPath`
(frontmatter-first, **exige `.md`**).

**Critérios de aceite:**
- [x] A regra passa a usar a fonte de verdade que o `ML-1A` decidiu, ou 🔴 **a decisão do `ML-1A` é
      corrigida com a razão escrita** — o que não pode continuar é comentário afirmando o que o
      código não faz
- [x] `roadmap: none` (e irmãos: `none`, `-`, `TBD`, comentário HTML) **passam a disparar**
- [x] 🔴 **Contra-braço:** vínculo legítimo continua **não** disparando, e o corpus real não ganha
      violação nova — medir **antes e depois** nos 231 REQs
- [x] O comentário de `:2217` passa a descrever o que o código faz

#### 🔴 Auditoria do ML-1D — medido por mim, com os dois binários

```
$ tf-before validate | grep -c "has no linked Roadmap"   →  12
$ tf-after  validate | grep -c "has no linked Roadmap"   →  13
$ diff (before | sort) (after | sort)
> ⚠ req "REQ-2026-08-16-conformidade-estrutural-…-tres-clis.md" has no linked Roadmap
```

**Exatamente 1 REQ mudou de veredito, e ela é genuína.** O valor dela, na linha 143:

```
Roadmap: (a criar quando esta REQ sair do backlog — não iniciar sem REQ + roadmap em `wip`)
```

🔴 **Prosa que diz literalmente que o roadmap não existe** — e passava pela regra. Sonda própria:
`roadmap: none` **dispara** (1) · vínculo legítimo **continua limpo** (0). `make quality` **rc 0**,
**340 OK** (338 + 2 da Direção C nova), 0 FAIL.

### R2 — o que decidiu entre (a) e (b) não foi o que eu mandei medir

Mandei escolher **pela medição do corpus**. Ele mediu, e a medição confirma — **mas o argumento que
fecha é outro, e é melhor:** o `docs/cli-parity.md` escrito pelo **próprio `ML-1B`** já declarava que
o gerador trata `none`, `-`, `<!-- … -->` como **placeholder a preencher**.

🔴 **Logo o gerador chamava `roadmap: none` de placeholder enquanto o validador chamava a mesma REQ
de vinculada.** Não era escolha de política entre duas saídas legítimas — era **divergência interna
do mesmo contrato**, e (b) exigiria reescrever o contrato do gerador junto. **Aritmética, não
amostra.**

### R1 — o comentário tinha DUAS claims falsas, não uma

Além da fonte de verdade, ele afirmava **case-insensitive**, e `extractFrontmatterField` faz
`HasPrefix(line, field+":")` — **sensível a caixa**. Só `extractRefPath` é `EqualFold`. As duas
viraram verdadeiras com a troca.

### 🔴 R3 — apertar a regra ESVAZIA o Cenário 192-B, e nenhuma composição salva

O Cenário 192 provava o achado A2 sabotando a ancoragem de `contentHasMarkerValue` e exigindo que a
violação **desaparecesse**. Com a regra migrada, a sabotagem deixa de mudar o veredito → a violação
**sobrevive** → `assert_lacks_pattern` reprova **sem defeito nenhum**.

E o detalhe que torna isso irrecuperável por remendo: com `.md` exigido, a prosa é recusada por
**duas razões independentes** (sem ancoragem **e** sem `.md`), então neutralizar só a ancoragem
**nunca** vira verde. Ele migrou a Direção B para o campo **ADR** (consumidor vivo de
`contentHasMarkerValue`) e criou a **Direção C** para medir a mesma propriedade no leitor novo.

⚠️ **E a Direção C só compila como disjunção** — substituir a condição deixaria `key` sem uso e o Go
recusa. É o tipo de detalhe que só aparece construindo.

### A população em risco não era 231, eram 7 — e a subtração é o teste

```
212 com .md · 17 com roadmap: "" · 2 sem o campo · 0 com none/não-.md
raio do aperto = 19 (frontmatter vazio) − 12 (já acusadas) = 7 REQs que passavam SÓ pelo corpo
```

Das 7: **4** têm caminho simples → passam · **2** têm caminho **entre backticks** → passam (e isso
fecha um achado da `REQ-2026-07-30` que registrava esse formato como *"ignorado"*) · **1** era a
prosa. 🔴 **Medir só o total esconde o raio** — foi ele que viu isso, não eu.

### Sítio de mesma causa fechado no mesmo ML

O lado frontmatter de `req_roadmap_sync` (`:2278`) ainda usava `extractFrontmatterField`. Deixá-lo
faria as duas regras **discordarem sobre o que é valor**: `roadmap: "none"` + caminho real no corpo
daria *"sem vínculo"* numa e *"divergent links"* na outra — divergência que **não existe**. Corrigido;
a saída ficou **byte-idêntica** à medição "depois", logo é correção de **mecanismo**, não de contagem.

### Varredura de mesma classe, com o critério certo

Ele não varreu "regras que leem frontmatter" — varreu **regras cujo comentário afirma uma fonte de
verdade diferente da que o código usa**. `req_has_adr`/`wip_has_req`/`blocked_has_req` são cegas ao
frontmatter **mas não têm comentário mentindo** → fora da classe, e viram o **`ML-1E`**.
`note_orphan` e `resolveAdrStatus` descrevem o código. E o doc de `extractRefPath` **omitia a
exigência de `.md`** — *"foi essa omissão que alimentou a confusão ML-1A/ML-1D"*.

⚠️ **Verrugas deliberadas, declaradas:** a mensagem da violação continua dizendo *"marker must start
the line"* (lê torto para o caminho de frontmatter) porque o literal **é barreira**; e um `roadmap:`
declarado e absolutamente vazio bloqueia o fallback para o corpo — **0 casos** nos 231.

### ML-1E — `req new` (wizard) cria ADR draft e não grava `adr:`
**Status:** ⬜ Pendente · ⚠️ **achado estrutural, sem instância medida no corpus**

Mesmo mecanismo, outro elo: o wizard cria drafts via `NewADRDraft`, lista em *"Blocked by ADRs"* e
**nunca** grava `adr:` — `content.LinkedADR` fica vazio.

**Critérios de aceite:**
- [ ] O elo é escrito na criação, ou a ausência é **declarada** no contrato
- [ ] 🔴 **A instância é construída antes de corrigir** — o corpus não tem nenhuma, e corrigir o que
      não se consegue reproduzir é como o gate que fica verde por não haver o que achar

## Wave 2 — A decisão arquitetural
> Dependências: Wave 1 (a fonte de verdade precisa estar decidida).

### ML-2A — **AC11** — ADR da precisão do vínculo branch↔roadmap
**Status:** ✅ Concluído em 2026-09-26 — `docs/adr/ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir.md`

**As cinco decisões, em uma linha cada:**

| | decisão |
|---|---|
| **D1** | 🔴 **O vínculo passa a ser ESCRITO; a inferência vira fallback.** O `branch new` **sabe** qual roadmap está em `wip/` no instante em que cria a branch — inferir depois é reconstruir informação que existia e foi jogada fora |
| **D2** | A relação da inferência é **sobreposição de tokens**, não substring nem fronteira. ⚠️ **O número mínimo NÃO é decidido no ADR** — é calibração do `ML-3A`, e o `AC14` a torna gate |
| **D3** | **`validator.go` é a FONTE**; `generators/roadmap.go` delega. Sem isso a Wave 3 entrega dois matchers concordando **por coincidência** — o defeito do AC5 da REQ-2026-08-31 em outra roupa |
| **D4** | 🔴 **Modo ADITIVO primeiro.** A etapa 1 aceita tudo que `Contains` aceitava **e mais**, logo nenhuma branch que passava passa a falhar — é o que torna o fix commitável pelo próprio `trackfw commit` |
| **D5** | A contenção do falso-positivo é **medida** (`AC14` vira gate), não prometida |

**Por que D4 é a decisão que mais importa:** o roadmap declara o *deadlock de bootstrap* como risco
terminal — matcher novo rejeita a branch do fix → `trackfw commit` falha → `git commit` cru é
bloqueado pelo guard → **o fix não pode ser mergeado**. Isso já acontece em ~9% dos casos medidos.
A ordem aditiva-primeiro é o que o torna **reversível sem intervenção manual de git**.

**Exceção à ordem, com razão escrita:** o nome vazio. `strings.Contains(x, "")` é sempre verdadeiro e
**não existe consumidor legítimo** de `roadmap move ""` — recusá-lo é aditivo em segurança e não
paralisa ninguém.

**Critérios de aceite:**
- [x] ADR escrita, com candidatos descartados e a medição citada
      → 4 alternativas, e a A1 (fronteira) **falsificada por aritmética**, não por amostra
- [x] Decisão explícita sobre escrever vs. inferir → **D1**: escrever é fonte, inferir é fallback
- [x] Contenção do risco de falso-positivo declarada → **D4** (ordem) + **D5** (gate)
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
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** — implementação única em Go

### ML-4B — **AC2 + AC3** — corte por data, grandfathering visível
**Status:** ⬜ Pendente
**Ações:** REQ criada a partir de `<corte>` sem roadmap ⇒ **error**; anterior ⇒ warning. Corte
declarado no artefato. O relatório diz **quantas** estão isentas e **desde quando**.
🔴 *Isenção que não se vê vira permanente.* E inverter a severidade sem corte faz o `validate` falhar
em 32 REQs — alguém configura `lenient` e perdemos a regra **e** o aviso.
**Critérios de aceite:**
- [ ] REQ pós-corte sem roadmap ⇒ error; pré-corte ⇒ warning (os dois braços)
- [ ] Relatório mostra contagem de isentas e a data de corte
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** — implementação única em Go

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

---

## 🔴 ESTACIONADO até a v8 — decisão do KG, 2026-09-12

> *"antes de implementar qualquer coisa vamos finalizar a v8. Ela destrava todo o restante."*

**Não é abandono nem falta de gente.** É a ordem correta: a
`REQ-2026-09-12-v8-um-binario-muitos-canais` remove `npm/src/` (26.272 linhas) e `pypi/trackfw/`
(27.472), mais **31 dos 61 gates**. Todo trabalho que toque esses alvos hoje é feito **três vezes** e
apagado em seguida.

**O que muda quando a v8 entrar:**

- o que for **paridade pura** desaparece — não é corrigido, deixa de ser possível
- o que for **defeito real** continua valendo, e passa a custar **1× em vez de 3×**

🔴 **Nenhum ML daqui deve ser retomado sem antes reclassificar nessa chave.** Retomar como está
significa implementar em runtimes que estão sendo deletados.

**Reabre:** quando a Wave 3 da v8 fechar (`ROADMAP-2026-09-12-v8-um-binario-muitos-canais`), pelo
ML-4A dela, que classifica REQs e issues em *desaparece / barateia / indiferente*.

**Nota específica:** o **AC7** desta REQ (o `roadmap new --from-req` não consolida os ACs nem
deriva MLs) mordeu **três vezes em 2026-09-12** — inclusive ao gerar o esqueleto do roadmap da
próprio v8, que precisou ser reescrito à mão. Ele **sobrevive à v8** e é o candidato mais forte a
retomada imediata quando ela fechar.

---

## Medição de 2026-09-22 — o defeito confirmado em execução, e a dimensão dele

Registrado por `trackfw_architect` ao organizar a governança. **Não altera o status deste roadmap**
(segue `blocked`); acrescenta a evidência que faltava.

### O comando existe, aceita a flag, e não faz o vínculo reverso

```
$ trackfw roadmap new --from-req docs/req/REQ-....md --req docs/req/REQ-....md
✓ created docs/roadmaps/backlog/ROADMAP-2026-09-22-....md

$ grep -n "^roadmap:" docs/req/REQ-....md
6:roadmap: ""
```

O roadmap gerado **declara** a REQ no próprio frontmatter (`req: "docs/req/..."`). A REQ **não**
recebe o ponteiro de volta. `--req` vincula roadmap→REQ; o sentido REQ→roadmap fica vazio, e é
justamente o que o `trackfw validate` cobra com `has no linked Roadmap`.

Ou seja: a informação existe e está correta de um lado. **O segundo comando não se esquece de
perguntar — ele se esquece de escrever de volta.**

### A dimensão, medida no corpus real

| | antes | depois de vincular à mão |
|---|---|---|
| REQs sem roadmap vinculado | **35** | **12** |
| `trackfw validate` warnings | **166** | **143** |

As **47** REQs que puderam ser vinculadas foram reconstruídas a partir do campo `req:` **do próprio
roadmap** — nenhum vínculo foi inventado. Verificação bidirecional: 47 consistentes, **0 cruzados**.

As 12 remanescentes são **9 `Superseded`** (paridade Node/Python, sem sentido pós-v8) e **3 `Done`**
históricas. **Nenhuma `Open`.**

### O que isto sugere para o escopo deste roadmap

O vínculo reverso é **derivável** — o roadmap já sabe de qual REQ nasceu. Então, além de fazer
`roadmap new --req` escrever nos dois lados, cabe avaliar um modo de reconciliação
(`trackfw validate --fix` ou `trackfw doctor`) que reconstrua o ponteiro a partir do `req:` do
roadmap, em vez de exigir edição manual em 47 arquivos.

🔴 **O ônus da prova é o de sempre:** um comando que escreve em 47 arquivos de governança precisa de
falsificação nas duas direções — vínculo correto criado **e** vínculo cruzado recusado.

---

## 🔴 Desbloqueio — 2026-09-26: a condição de reabertura foi satisfeita e ninguém a leu

Este roadmap foi para `blocked` em **2026-09-12 22:32**, e a razão estava **escrita** aqui:

> *"Reabre: quando a Wave 3 da v8 fechar (`ROADMAP-2026-09-12-v8-um-binario-muitos-canais`), pelo
> ML-4A dela, que classifica REQs e issues em desaparece / barateia / indiferente."*

**Medido em 2026-09-26:** `docs/roadmaps/done/ROADMAP-2026-09-12-v8-um-binario-muitos-canais.md` e a
REQ correspondente com `status: Done`. 🔴 **A condição foi satisfeita há duas semanas, e o roadmap
ficou parado** — o bloqueio expirou sem que nada o reabrisse.

⚠️ **Registro do que isso significa para o processo:** um `blocked` com condição de reabertura escrita
é bom; o que falta é **alguém reler a condição quando ela vence**. Não há gate para isso — e este é o
segundo artefato desta campanha que ficou parado por esse mecanismo (o outro foi a tabela de
residuais da REQ-2026-08-31, que virou as issues #400/#401/#402).

### Por que a retomada é agora, e o que o #273 acrescenta

Pedido do usuário, com a razão dele: *"o #273 já está causando problemas inclusive para nós"*.

O **#273** já estava absorvido aqui como **AC11–AC14** — mas medi que ele traz uma coisa que **nenhum
AC exigia**: a direção **restrito demais**. Virou o **AC15** da REQ.

### ⚠️ Uma tese minha, refutada pelo contra-braço ANTES de virar argumento

Medi que as branches governadas deste repositório têm slug de **63,6 chars** em média (máx. 109) e ia
usar isso como *"a regra deforma os nomes para caber"*. O contra-braço:

```
governadas (feat/fix/refactor):   média 63,6 chars   n=10
NÃO governadas (docs/, chore/):   média 58,9 chars   n=8
```

🔴 **Diferença de 4,7 caracteres — o comprimento NÃO acompanha a governança. É estilo desta casa.**
Declarado como **não-discriminante**; a medição do fork (~9% de rejeição real) continua sendo a
evidência, e a minha não a reforça.

### E o que eu quase reportei errado

Ao criar a branch desta retomada, `trackfw branch new` **recusou** — e a tentação era anunciar que
reproduzira o #273 ao vivo. Refeito depois do `roadmap move … wip`: **passou**. A causa da primeira
recusa era o roadmap estar em `blocked/`, que o guard não aceita, **não** o casamento de slug.
🔴 **Não afirmei o que não medi.**

