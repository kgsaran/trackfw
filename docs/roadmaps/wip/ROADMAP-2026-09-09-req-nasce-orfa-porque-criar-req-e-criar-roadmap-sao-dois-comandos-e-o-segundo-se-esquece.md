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
- [x] AC14 — a medição das **205** branches históricas vira **gate** ⚠️ *(o texto dizia 111 — corrigido em 2026-09-26; o número é load-bearing: é o que um leitor futuro usa para decidir o que regenerar)*

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

**Escape 3: o fixture das **205** branches é regenerado no mesmo PR que muda o matcher (ML-3C).**
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
1. Este repo — **201** roadmaps + **205** branches históricas ⚠️ *(o texto dizia 185+111)*, medido sobre os binários e o corpus real: `Contains`=109/111, boundary=109/111 (0 regressão); 20 slugs curtos: Contains=326, boundary=266
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
# ⚠️ RECONCILIADO em 2026-09-26. O gate original tinha DOIS defeitos independentes, e
# nenhum apareceu antes porque `barrier --wave 0` não foi reexecutado desde 2026-09-09:
#
#   1. a âncora de vacuidade era `npm/src/validator/index.js`, DELETADO pela v8 (2eae0a44),
#      e 7 dos 10 sítios enumerados moravam em npm/src e pypi/trackfw — insatisfazível;
#   2. 🔴 era um script MULTI-LINHA. O `barrier` executa UMA LINHA POR VEZ: o `if` abria
#      sem fechar (`exit 2`) e `checks=(` saía `127`. Ele nunca foi executável como gate,
#      em nenhum momento — o formato, não o conteúdo, é que estava errado.
#
# Dos 3 sítios Go originais, `internal/generators/req.go` foi CORRIGIDO pelo ML-1C
# (medido: 1 ocorrência em 97543eef, zero hoje). Sobram 2 — e a queda é evidência de
# entrega, não de perda. Falsificado nas duas direções antes de entrar aqui.
n=$(git grep -l -e containsIgnoreCase -e BranchSlugMatchesRoadmap -- internal/generators/roadmap.go internal/generators/req.go internal/validator/validator.go | wc -l | tr -d " "); test "$n" = 2 && echo "Gate ML-0A: $n/2 sitios remanescentes confirmados (1 dos 3 Go corrigido pelo ML-1C; 7 sairam com a v8)" || { echo "GATE FALHOU: $n/2 - a populacao mudou, reconcilie a enumeracao" >&2; exit 1; }
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

### Sítios que a varredura do ML-1B achou (ainda Wave 1)

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

### ML-1E — As regras de vínculo são cegas ao frontmatter — e há instância medida
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · **o delta não era −2: é −7 e +24**
**Critérios de aceite:**
- [x] As regras de vínculo passam a ler **frontmatter e corpo**, com precedência declarada
- [x] 🔴 **Delta medido no corpus real, não estimado** → **−7 e +24**, não os −2 que eu previ.
      Minha régua censava *"sem linha `ADR:`"*; o mecanismo era o frontmatter **invisível** a ela
- [x] Contra-braço: vínculo legítimo **deixa** de ser acusado, e ausência real **continua** sendo
- [x] Reconciliação: uma frase por teste novo
- [x] `make quality` verde

⚠️ **Bloco acrescentado em 2026-09-26**, na auditoria que reexecutou os barriers das waves 0–4. O ML
estava `✅` desde antes e **não tinha critérios escritos** — o `barrier --wave 1` recusava com
`no acceptance block`, e ninguém tinha rodado. Os critérios acima são a **reconstituição do que foi
de fato entregue e medido**, não invenção retroativa: cada linha aponta para medição já registrada
no corpo deste ML.

⚠️ **Este ML foi REESCRITO por mim em 2026-09-26.** A redação anterior dizia *"achado estrutural, sem
instância medida no corpus"* e falava só do wizard. 🔴 **Medi, e há instância — na regra, não no
wizard.**

### O achado: `req_has_adr` acusa REQ com vínculo legítimo

`req_has_adr` (`validator.go:2186`) lê **só o corpo**, por `contentHasMarkerValue`. O mesmo vale para
`wip_has_req` (`:2165`) e `blocked_has_req` (`:2204`). Medido nos 231 REQs:

```
4 REQs têm `adr:` no frontmatter e NENHUM `ADR:` no corpo
  2 com travessão (—)                          → acusação CORRETA, é placeholder
  2 com caminho .md REAL e resolvível no disco → 🔴 ACUSAÇÃO FALSA
```

E uma das duas falsas é **a REQ desta própria campanha**:

```
⚠ req "REQ-2026-09-09-req-nasce-orfa-….md" has no linked ADR
   frontmatter: adr: "docs/adr/ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-….md"   ← existe no disco
```

🔴 **Eu vinculei essa REQ ao ADR que escrevi hoje, pelo frontmatter, e o validador diz que ela não
tem ADR.** É exatamente o defeito que o `ML-1D` acabou de corrigir para `roadmap:`, **no campo ao
lado**, e que não foi tocado porque a régua da varredura era *"comentário que mente"* — e estas três
regras **não têm comentário nenhum**, logo ficaram fora da classe.

⚠️ **Contexto que dimensiona:** `no linked ADR` é o maior bloco do `validate` hoje — **128
ocorrências**. A correção **não** deve zerá-lo: a maioria é REQ genuinamente sem ADR. O alvo são as
que têm vínculo e são acusadas mesmo assim.

### Ações

1. As três regras passam a consultar **frontmatter e corpo**, pela mesma fonte de verdade que o
   `ML-1D` instalou (`contentHasStructuredRefValue`). 🔴 **Reúse o helper** — escrever um segundo é o
   defeito do AC5 da REQ-2026-08-31, que esta casa já pagou.
2. O **wizard** (`req new`) cria ADR drafts via `NewADRDraft`, lista em *"Blocked by ADRs"* e **nunca**
   grava `adr:`. ⚠️ **Sem instância no corpus** — construa uma antes de corrigir, ou declare a ausência
   no contrato.

**Critérios de aceite:**
- [x] As **2 acusações falsas** medidas desaparecem, **nomeadas uma a uma** no relatório
- [x] 🔴 **Contra-braço:** as **2 com travessão continuam acusadas** — se sumirem, a regra virou
      permissiva e trocamos falso positivo por falso negativo
- [x] Medir **antes e depois** as 128 de `no linked ADR`, com a lista das que mudaram de veredito.
      🔴 **Se o delta for maior que 2, cada uma extra é explicada** — número que muda sem explicação é
      o que esta REQ persegue
- [x] `wip_has_req` e `blocked_has_req` recebem o mesmo tratamento, com a medição própria de cada uma
- [x] O wizard: elo escrito **ou** ausência declarada no contrato, com a instância construída
- [x] `ML-1B`, `ML-1C` e `ML-1D` não são desfeitos
- [x] Reconciliação: uma frase por teste novo
- [x] `make quality` verde

#### 🔴 Auditoria do ML-1E — o delta que eu previ estava errado por um fator de 12

Medi com os dois binários:

```
              antes    depois
no linked ADR   128  →   145        ← −7 falsas, +24 novas e CORRETAS
no linked Roadmap 13 →    13        ← ML-1D intacto
wip_has_req        0 →     0        ← medido, não presumido
blocked_has_req    0 →     0
warnings         153 →   170
contra-braço: as 2 com `adr: —` continuam acusadas ✅
```

🔴 **Eu escrevi que o delta seria −2 e que "delta maior que 2 exige explicação".** Ele explicou, e a
explicação refuta a minha régua: eu censei *"REQs sem a linha `ADR:` no corpo"*, e **o mecanismo é o
frontmatter ser invisível ao casamento case-sensitive**. As outras 5 falsas **têm** linha `ADR:` — um
**comentário HTML** — e caminho `.md` real no frontmatter. Minha régua as excluiu **por construção**.

**E a direção que eu não previ é a maior:** **+24 acusações novas e corretas**, de REQs que
satisfaziam a regra com **placeholder em prosa** (`ADR: N/A — extensão de um gate…`,
`ADR: (a decidir — …`). Nenhuma tem ADR. 🔴 **O bloco CRESCER é o sinal certo** — o `ML-1D` produziu o
mesmo em `req_has_roadmap` (12 → 13). Zerá-lo é que seria o defeito.

Fora do bloco do ADR, a saída é **idêntica linha a linha**: nenhuma outra regra mudou de veredito.

### Ponto de ratificação 1 — o +24: **ratifico**

São genuínas, e `validate` continua **rc=0** nos dois lados (o acervo está `lenient` até 2027-12-31).
É **surfacing**, não gate: o passivo vira visível sem travar ninguém.

### 🔴 Ponto de ratificação 2 — o ID pelado deixa de ser vínculo: **ratifico, e vai para o CHANGELOG**

`REQ: REQ-2026-07-29-fixture` (sem diretório, sem `.md`) satisfazia `wip_has_req`/`blocked_has_req` e
**não satisfaz mais**. Medi: **zero** ocorrências nos artefatos governados — mas era a forma viva nas
**fixtures** (6 testes de barrier + 1 de `ship` + 12 sítios de `check-barrier.sh`).

**Ratifico pela consistência:** a mesma restrição já valia para `roadmap:` desde o `ML-1D`, e ter
`REQ:` aceitando ID pelado enquanto `roadmap:` exige `.md` **é exatamente a divergência que esta REQ
persegue**. O valor precisa ser resolvível para `ref_targets_exist` significar alguma coisa.

⚠️ **Mas é mudança de comportamento observável, e o usuário pediu o changelog explícito.** São
**três**, não duas:

| # | mudança | efeito |
|---|---|---|
| 1 | `lenient` sem `lenient_until` | `validate` que saía `rc=0` pode sair `rc≠0` |
| 2 | vínculo branch↔roadmap deixa de ser substring | **a medir na Wave 3** (ADR manda aditivo) |
| 3 | **`REQ:`/`ADR:` com ID pelado deixa de ser vínculo** | consumidor com `link_fields` não-caminho perde o casamento |

⚠️ `link_fields` **não tem superfície de contrato** em `cli-parity.md` (medido: 2 menções, nenhuma
normativa), então nada documentado quebra — **mas o consumidor que configurou `req_id` não leu
documentação nenhuma para fazê-lo.**

### O wizard: instância CONSTRUÍDA, e a não-correção é a decisão certa

Ele reproduziu sem TTY pelo caminho real (`NewADRDraft` → `DependsOnADRs` → `NewREQ`): a REQ nasce
com o ADR listado só em *"Blocked by ADRs"*, `adr: ""`, e **é acusada**.

🔴 **E não corrigiu, com razão melhor que a minha:** o ADR está em **`Draft`**, e aquela seção
significa **o oposto** de vinculado — o próprio wizard imprime *"Resolve these ADRs (set Status:
Accepted) before creating a roadmap"*. Preencher `adr:` chamaria de vinculada uma REQ cuja decisão
**não existe**, trocando o falso positivo pelo **falso negativo que o contra-braço proíbe**.
Declarado no contrato e **pinado por teste**, para a declaração não envelhecer calada.

### Instrumento: três braços esvaziados, e o segundo achado pelo QUALITY, não por leitura

As Direções **A e B** do Cenário 192 ficaram vácuas **de uma vez** — com as três regras migradas,
sabotar a ancoragem não move veredito nenhum. E o **Cenário 28** reprovou na primeira barreira:

🔴 *"A régua 'grepe a MENSAGEM da regra' acha quem **assevera** a mensagem; não acha quem passa a
**emiti-la como efeito colateral**."* O mesmo binário corrompido que escondia a referência de
`adr_accepted_when_req_done` escondia o vínculo de `req_has_adr`, que passou a emitir corretamente e
derrubou um `assert_lacks_pattern` **sem defeito nenhum**.

⚠️ **E o rc lido em linha separada foi decisivo:** a notificação do runner disse *exit code 0* na
primeira execução, e o rc real era **2** — era o `echo` do wrapper que saía 0. **Só o arquivo de rc
revelou a reprovação.**

### Resíduo

- `contentHasMarkerValue` ficou **sem chamador de produção** (só testes). Mantida: remover ampliaria o
  diff. A propriedade do achado A2 sobrevive **por construção** dentro de `extractRefPath`.
- `gen-falsify-chunks` avisa que os 7 rótulos novos estão **sem peso calibrado** — é o **#403**, que
  está declarado fora desta REQ. Coerente.

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
**Insumo obrigatório — a medição já feita em 2026-09-12, contra **201** roadmaps e **205** branches reais ⚠️ *(o texto dizia 185 e 111)*:**

| | substring | fronteira |
|---|---|---|
| casamentos das **205** branches históricas | 109 | **109** |
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

### ML-3A + ML-3B — **AC12 (parte 2) + AC13 + AC15** — o matcher, em modo ADITIVO
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · 🔴 **eram 29 de 205 rejeitadas, não 2 de 111**
**Arquivos afetados:** `internal/validator/validator.go` — `BranchSlugMatchesRoadmap` está em
**`:3594`** e `normalizeBranchSlug` em **`:3750`** (medido 2026-09-26; a linha `2864` do texto
original está obsoleta). Consumidores: `validate` · `branch new` (`commands/branch.go:100`) · `commit`.
⚠️ **Os espelhos `npm/src/validator/` e `pypi/trackfw/validator.py` NÃO EXISTEM desde a v8.0.0.**
**Ações:** implementar o que a **ADR do ML-2A** decidiu — `ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir.md`.

🔴 **A D4 é a decisão que governa este ML: modo ADITIVO.** A etapa 1 aceita **tudo que `Contains`
aceitava, e mais**. Nenhuma branch que passava pode passar a falhar — é o que torna o fix commitável
pelo próprio `trackfw commit` e evita o **deadlock de bootstrap** (matcher rejeita a branch do fix →
`commit` falha → `git commit` cru é bloqueado pelo guard → o fix não pode ser mergeado).

**Critérios de aceite:**
- [x] **AC12** — vínculo **escrito** (D1) é a fonte; a inferência por **sobreposição de tokens** (D2)
      é fallback para branch vinda de fora do `branch new`
- [x] 🔴 **AC13 (ex-ML-3B) — modo aditivo provado por MEDIÇÃO, não por intenção:** as **205 branches
      históricas** ⚠️ *(o texto dizia 111)* que `Contains` aceitava **continuam** aceitas. Zero regressão, e o número sai do
      corpus, não do raciocínio
- [x] 🔴 **AC15 — a direção RESTRITO DEMAIS fecha:** uma branch legitimamente governada cujo slug
      **não** é substring do roadmap passa a ser aceita. Caso do #273:
      `feat/adrs-retroativas-da-divida-do-acervo` × `ROADMAP-…-divida-de-governanca-do-acervo-…`
- [x] **Retomada legítima** de roadmap em `done/` continua funcionando (contra-braço)
- [x] 🔴 **`branchSlug` vazio** deixa de casar qualquer roadmap — herdado do `ML-1C`, que o mediu e
      declarou fora do escopo dele
- [x] 🔴 **O gate que fixa o comportamento atual é ATUALIZADO, nunca afrouxado para caber.**
      `scripts/check-barrier.sh` e `scripts/check-validate-rule-pins.sh` referenciam o matcher
- [x] ⚠️ **O número mínimo de tokens é CALIBRADO contra o corpus**, não escolhido. O #273 propôs ≥2 e
      **declarou que não era valor calibrado**
- [x] `ML-1B`…`ML-1E` não são desfeitos
- [x] Reconciliação: uma frase por teste novo
- [x] `make quality` verde

### ML-3C — **AC14** — a medição vira gate
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · 🔴 **e o braço substring é quase morto: só 2 de 205 dependem dele**
**Arquivos afetados:** novo `scripts/check-roadmap-slug-matching.sh`, wired no `Makefile`.
🔴 **Fronteira:** este ML **NÃO** toca `internal/generators/` — é do `ML-3D`, em paralelo.

⚠️ **Os números deste ML estavam errados** (corrigido em 2026-09-26 pela medição do `ML-3A+3B`):
não são 185 roadmaps e 111 branches, são **201** em `wip/`+`done/` e **205** branches governadas.
"Os 3 runtimes" ficou **sem objeto** desde a v8.

**Ações:** o corpus real vira **fixture versionada**, e mudança no matcher que altere o veredito de
qualquer branch **reprova**.

**Critérios de aceite:**
- [x] 🔴 **O corpus é FIXTURE VERSIONADA, não consulta ao `git`/`gh` em tempo de gate.** Lição do
      `ML-7C` da REQ-2026-08-31: corpus alcançável só por ref local é corpus que o CI não tem — e ali
      o objeto sobrevivia em **um único ref**, que o `branch prune` classificaria como seguro apagar
- [x] O gate fixa o veredito das **205** branches contra os **201** roadmaps, e a contagem é **exata**,
      não piso — regressão do matcher **muda o número** e reprova
- [x] 🔴 **Falsificação: mutação no matcher ⇒ gate reprova**, e o braço **nomeia** qual branch mudou
      de veredito. Gate que só diz "o número mudou" manda o próximo desenvolvedor adivinhar
- [x] **Contra-braço:** matcher correto ⇒ gate passa, com **não-vacuidade provada** (corpus ausente ou
      população zero **reprovam**, nunca passam em silêncio)
- [x] ⚠️ **O limiar `branchRoadmapMinSharedTokens=2` é fixado pelo gate** — alterá-lo sem atualizar o
      corpus reprova. É a calibração virando contrato
- [x] `make quality` verde

### ML-3D — `trackfw init` precisa ignorar o vínculo no consumidor
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26
🔴 **Fronteira:** este ML toca **`internal/generators/`** e seus testes. **NÃO** toca `scripts/`,
`Makefile` nem `docs/cli-parity.md` — são do `ML-3C`, em paralelo.

**Por que é entregável desta REQ, e não achado externo:** o `ML-3A` criou
`<roadmap_dir>/.trackfw-branch-links.json` e o adicionou ao `.gitignore` **deste** repositório. No
consumidor, o `trackfw init` não sabe dele — então **estado por checkout vaza para o repositório
dele**, e o vínculo de uma máquina passa a governar a de outra.

**Critérios de aceite:**
- [x] `trackfw init` acrescenta `<roadmap_dir>/.trackfw-branch-links.json` ao `.gitignore` gerado
- [x] 🔴 **Contra-braço:** verificado pelo **arquiteto** em 2026-09-26, em projeto de sonda: um
      `.gitignore` com `# meu gitignore` + `node_modules/` **sobrevive intacto** (o bloco é
      acrescentado ao fim), e a **2ª execução do `init`** deixa `grep -c trackfw-branch-links` em
      **1**. `.gitignore` já existente **não é sobrescrito**, e rodar duas vezes **não
      duplica** a linha
- [x] ⚠️ **Consumidor já onboardado não vai rodar `init` de novo** — o caminho para ele fica
      **declarado**, nem que seja uma linha dizendo que o arquivo é local e pode ser ignorado à mão
- [x] Reconciliação: uma frase por teste novo
- [x] `go test ./internal/generators/` verde (a barreira completa é do arquiteto)

---

#### 🔴 Auditoria do ML-3A+3B — o defeito era 7× maior do que o ADR dizia

Medi por caminho próprio:

```
branches governadas no histórico     205   (o ADR dizia 111)
bootstrap: TRACKFW_BRANCH=<esta branch> validate  →  0 violações, rc=0
os 6 testes do matcher                PASS
make quality  rc 0 · 342 OK · 0 FAIL · check-validate-rule-pins OK (28 pins, era 25)
```

🔴 **`strings.Contains` rejeita 29 das 205 branches governadas — 14%**, não os "2 de 111" do ADR nem
os ~9% do #273. Das 29, **14** casam o roadmap certo por ≥2 tokens, verificadas par a par.

⚠️ **E a ressalva dele é o que dá sentido ao número:** os 176 aceitos são população **sobrevivente** —
o autor do #273 **renomeou a branch para caber na regra**, e esta casa provavelmente fez o mesmo sem
registrar. **`Contains` acertar 86% é em parte seleção, não acerto.**

### R2 — a D3 do meu ADR é NO-OP, e eu citei linhas que não existem mais

Mandei `generators/roadmap.go` delegar, citando `:791,805`. **Essas linhas morreram no `ML-1C`.**
Confirmei: `containsIgnoreCase` está em `:939`, chamada de `:908`, dentro de `selectArtifactByName` —
que resolve **argumento do usuário**, não vínculo branch↔roadmap.

🔴 **Delegar ali reintroduziria seleção difusa no `roadmap move` e desfaria o `ML-1C`** — que é
critério de aceite deste próprio ML. **D3 satisfeita com zero linha**, e ele tratou isso como achado
a declarar, não como tarefa a pular. **Emendei o ADR.**

### R3 — "falso positivo" tem dois sentidos, e medir um só dá falso verde

A relação é um **OR** e a função responde *"algum roadmap casou?"*. Para branch que já passava por
substring, token extra **não muda veredito**. Ele reportou os dois níveis — **veredito** (o que este
portão aplica: 14 flips rejeita→aceita, **0** aceita→rejeita) e **cardinalidade** (o sinal da etapa 2).

### R4 — tokenizar o nome cru dá um token grátis a todo mundo

`normalizeBranchSlug("ROADMAP-2026-09-09-x.md")` = `roadmap-2026-09-09-x-md`: 🔴 **`roadmap` é token
de todo arquivo do acervo** — 175 de 201 crus contra **16** com prefixo removido. Remédio **posicional**,
nunca lista negra: o token `req` do mesmo par é **título** e sobrevive.

### O limiar 2 foi FORÇADO, não escolhido

| N | das 29 passam | regressão | genérico de 1 token |
|---|---|---|---|
| 1 | 29 (**205/205** — deixa de discriminar) | 0 | `req` 18 · `gate` 20 · `guard` 14 |
| **2** | **14** | **0** | **0** |
| 3 | 5 | 0 | 0 |

**Teto:** o par do #273 compartilha **exatamente 2** tokens. **Piso:** `N=1` aceita tudo.

### Zero regressão é ESTRUTURAL antes de empírica

Para todo slug não-vazio, `Contains(x,s) ⇒ aceita` **por construção** (OR com o braço substring
preservado verbatim), logo o conjunto aceito é **superconjunto universal**. As 205 confirmam:
`flip− = 0` em 4 limiares × 2 tokenizações. 🔴 **Prova de construção primeiro, medição como
confirmação** — é a ordem certa, e evita concluir "não regrediu" de uma amostra.

**Bootstrap testado no PRIMEIRO build**, não no fim — o deadlock é impossível nesta etapa por
construção.

### Armadilha de rótulo que ele achou e que teria passado

O Bloco 3 do `check-validate-rule-pins.sh` **já usava** `pin6`/`pin7`/`pin8`. Continuar a numeração
local do Bloco 2 fez o gate **passar com rótulos duplicados**, sem detecção, imprimindo
`Block 3: pin6-pin20` como se nada fosse. Renomeados para `pin2b`/`pin2c`/`pin2d`.

### As três decisões que ele devolveu — decidi as três

1. **Emendar o ADR** (censo 29/205 e D3 no-op) — ✅ **feito**, no mesmo dia. ADR que descreve mal o
   corpus é pior que ADR ausente, porque é citado.
2. **`SITE_FLOOR=157` defasado** (mede 158 **já em `HEAD`**) — ✅ **corrijo em commit separado**, e ele
   estava certo em não fazê-lo aqui: bumpar junto faria parecer que este ML criou o sítio.
3. 🔴 **`.gitignore` do `trackfw init`** — vira **`ML-3D`**. Se o consumidor não ignorar o arquivo de
   vínculo, **estado por checkout vaza para o repositório dele** — e é entregável desta REQ, não
   achado externo.

#### Registro — o `ML-3D` nasceu aqui

⚠️ **A definição e o status do `ML-3D` são os da seção própria dele**, acima. Este bloco é só o
registro de **onde o ML nasceu** (achado do ML-3A), e a cópia dos critérios que existia aqui foi
removida: ela marcava `⬜ Pendente` para um ML que está `✅ Concluído`, e o único motivo de o `barrier`
não a ter acusado é que este heading não começa com o rótulo. Critério em dois lugares é critério que
divergem.

#### 🔴 Auditoria do ML-3C e do ML-3D — e o achado do 3C reposiciona a etapa 2

**Medido por mim:**

```
check-roadmap-slug-matching             OK  (7 verificações; 205 branches × 201 roadmaps + 1 calibração)
check-roadmap-slug-matching --self-test OK  (9 braços)
corpus versionado                       5 arquivos, nenhum gitignored
distribuição dos arms                   substring 176 · tokens 14 · none 15
make quality                            rc 0 · 342 OK · 0 FAIL
```

### 🔴 R1 do ML-3C — o braço substring é PRATICAMENTE MORTO, e isso é o primeiro número da etapa 2

Eu pedi só o limiar. Ele mutou **o outro braço** da relação e mediu:

```
matcher atual (N=2)        190 accept / 15 block
braço SUBSTRING morto      188 accept / 17 block     ← só 2 flips em 205
flips: feat/v2.0-gaps · fix/v8-um-binario
```

**Em 203 de 205 casos a sobreposição de tokens já SUBSUME o substring.** 🔴 **O raio da etapa 2 da D4
— "remover o que só o substring aceita" — é exatamente essas duas branches**, ambas com menos de 2
tokens de 3+ caracteres (`gaps`; `binario`, porque `v8` e `um` têm 2 chars). Não é estimativa: é um
braço do `--self-test`.

⚠️ **E o R2 dele corrige o meu AC:** eu escrevi *"mutação no matcher ⇒ gate reprova"*, e os três
primeiros braços moviam **a mesma constante** — provariam o **limiar**, não a **relação**. O braço do
substring é que fecha isso.

### Corroboração por caminho independente

A linha `limiar 99` (braço de tokens morto = `Contains` puro) devolve **176 accept / 29 block** —
reproduzindo **exatamente** o *"`Contains` rejeita 29 das 205"* do `ML-3A`, por outro caminho. E fecha
a aritmética sem amostra: `29 − 14 reparadas = 15 block`; `205 − 29 = 176 substring`.

### O que o gate NÃO pina, declarado em vez de presumido

- **Vínculo escrito (D1):** `branch new --dry-run` chama `matchSlug` direto, nunca
  `ResolveBranchRoadmap`. Ele pina a **inferência** — que é o **único caminho em CI, clone e fork**,
  por o vínculo ser gitignored. 🔴 *"Quem auditar assumindo o contrário vai achar que deixei a D1 sem
  pino: não deixei, ela não é congelável em fixture."*
- **Cardinalidade:** a CLI só responde sim/não.
- **Se o corpus ainda espelha o acervo vivo:** nada prova. Regenerar é ato deliberado.

### O caso do #273 é sintético, e a honestidade está no fixture

`feat/adrs-retroativas-da-divida-do-acervo` **não existe neste acervo** — o autor renomeou a branch
para caber no `Contains`. 🔴 **Sem ele, o teto do limiar não fica pinado por branch nenhuma desta
casa.** O tail do roadmap é elidido no issue e no ADR; ele completou com `-de-decisoes`, que **não
acrescenta token compartilhado**, e escreveu isso como nota no próprio fixture.

### R3 — ele achou 6 citações defasadas no MEU roadmap e não as editou (fronteira)

`111 branches` / `185 roadmaps`, duas delas **load-bearing**: o texto do **AC14** e a **instrução de
regeneração** do corpus — *"as linhas que um leitor futuro usa para decidir o que regenerar"*.
**Corrigi todas**, marcando o valor antigo.

#### Auditoria do `ML-3D` — ratifico a decisão de NÃO parar

O handoff mandava parar se o `init` não gerasse `.gitignore`. Ele não gera (medido: 0 ocorrências em
`HEAD`) — mas a razão do gatilho era *"categoria nova de mutação de arquivo do projeto"*, e
`generateGitAttributes` (`scaffold.go:2614`) **já é essa categoria**, no mesmo arquivo, com três ramos
idempotentes. 🔴 **Ele disparou na letra e não na razão, verificou qual das duas valia, e ofereceu a
reversão de graça.** Sonda própria: o `.gitignore` nasce com `docs/roadmaps/.trackfw-branch-links.json`.

**E o R2 dele é o achado mais fino dos dois MLs:** num `t.TempDir()` sem `trackfw.yaml` o
`roadmap_dir` efetivo **é** `docs/roadmaps` — então o teste óbvio passa **idêntico** com o código
lendo a config ou **hardcodando** a string, e o AC anti-hardcode ficaria *não verificado com aparência
de coberto*.

#### 🔴 R4 do ML-3D — reproduzi, é grave, e virou a issue #445

```
roadmap_dir: governance/plans   →   trackfw init   →   roadmap_dir: docs/roadmaps
```

**`trackfw init` re-executado destrói a config do consumidor.** Não absorvi nesta REQ: o mecanismo é
**escrever por cima do layout declarado**, e não *"o comando conhece o vínculo e não o escreve"* —
diferença escrita na issue, inclusive por que **também não é** a família do #396 (lá era **ler**
presumindo layout).

⚠️ **Acoplamento que fica registrado:** o `ML-3D` roda `generateGitIgnore` **antes** de
`writeTrackfwConfig` **por causa do #445** — depois dele, o "roadmap_dir efetivo" seria sempre o
default e o AC estaria satisfeito **só na letra**. Quando a #445 for corrigida, a ordem deixa de
importar; se alguém mudá-la antes, **o AC quebra em silêncio**.

## Wave 4 — Prevenção e severidade
> Dependências: Wave 1. Independente da Wave 3.

### ML-4A — **AC1 + AC10** — um comando, com contra-braço
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · **o prompt caiu por medição, não por gosto**
**Ações:** criar REQ e roadmap deixa de exigir dois comandos. 🔴 **Medir o atrito de cada forma**
(flag em `req new`, prompt, `req new` chamando `--from-req`) — não escolher por gosto. **E** criar
REQ sem roadmap continua possível quando é deliberado: *atrito onde é engano, caminho livre onde é
intenção.*
**Critérios de aceite:**
- [x] Caminho integrado produz REQ **não órfã** (falsificação)
- [x] Caminho deliberado sem roadmap continua disponível (contra-braço)
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** — implementação única em Go

### ML-4B — **AC2 + AC3** — corte por data, grandfathering visível
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-09-26 · 🔴 **a regra JÁ era `error`; o ML não inverte severidade**
**Ações:** REQ criada a partir de `<corte>` sem roadmap ⇒ **error**; anterior ⇒ warning. Corte
declarado no artefato. O relatório diz **quantas** estão isentas e **desde quando**.
🔴 *Isenção que não se vê vira permanente.* E inverter a severidade sem corte faz o `validate` falhar
em 32 REQs — alguém configura `lenient` e perdemos a regra **e** o aviso.
**Critérios de aceite:**
- [x] REQ pós-corte sem roadmap ⇒ error; pré-corte ⇒ warning (os dois braços)
- [x] Relatório mostra contagem de isentas e a data de corte
- [x] ~~Paridade nos 3 CLIs~~ **SEM OBJETO desde a v8.0.0** — implementação única em Go

### ML-4C — **AC4** — onde bloqueia
**Owner:** `trackfw_architect` (eu)
**Status:** ✅ Concluído em 2026-09-26 — **decisão: só o `validate`. O `push` não muda.**
**Critérios de aceite:**
- [x] Decisão **escrita** sobre onde a regra bloqueia → **só o `validate`**; o `push` fica como está
- [x] 🔴 A distinção entre os **dois universos** (1 artefato × N artefatos) medida e registrada —
      é o erro que este ML existe para evitar
- [x] Nenhuma mudança de comportamento no `push` → `branch_has_wip_roadmap` intocado
- [x] `trackfw validate` sem violation nova

⚠️ **Bloco acrescentado em 2026-09-26**, mesma auditoria do `ML-1E`. Este ML é **decisão pura** — não
entrega código —, e por isso nasceu sem critérios. 🔴 **Decisão pura também precisa de bloco de
aceite:** sem ele o `barrier` não distingue *"decidido"* de *"esquecido"*, e foi exatamente isso que
ele recusou.

### A distinção que o AC4 pede, medida

🔴 **São dois universos diferentes, e confundi-los é o erro que este ML existe para evitar:**

| | o que pergunta | universo |
|---|---|---|
| `branch_has_wip_roadmap` | *"a branch em que estou tem roadmap em `wip/`?"* | **1** artefato — o da branch |
| `req_has_roadmap` | *"toda REQ do acervo tem roadmap?"* | **231** artefatos |

O `push` e o `commit` atravessam o **primeiro**. Medido: `commands/commit.go:24` referencia
`branch_has_wip_roadmap`, e **não** há referência a `req_has_roadmap` em `push.go`/`commit.go`.

### A decisão, e a razão

**`req_has_roadmap` bloqueia apenas no `validate`. O `push` continua exigindo só o da branch.**

Medição que fecha o argumento: **13 REQs** do acervo não têm roadmap — **8 de agosto, 5 de setembro**.
Se `req_has_roadmap` passasse a bloquear o `push`, 🔴 **ninguém conseguiria empurrar nada até que as
13 fossem resolvidas** — inclusive o PR que entrega esta própria REQ.

É a **mesma forma do deadlock de bootstrap** que a D4 do ADR evitou no matcher, e o roadmap já a
nomeia como *risco terminal*: o portão do `push` é atravessado por todo trabalho, e um passivo
histórico de 13 artefatos viraria trava para 100% das entregas.

⚠️ **E há uma assimetria que justifica a diferença, não só a conveniência:** o que a branch faz é
responsabilidade de quem empurra; o passivo do acervo **não é**. Bloquear alguém por uma REQ que
outra pessoa criou em agosto é cobrar o que ele não pode pagar — e o resultado previsível é
`governance_mode: lenient`, que perde a regra **e** o aviso.

**O que muda, então:** nada no `push`. O `ML-4B` cuida da severidade no `validate`, com corte por
data — que é o instrumento certo para passivo histórico.

**Critérios de aceite:**
- [x] Decisão escrita, com o impacto de cada opção
      → medido: 13 REQs afetadas, 8 de agosto e 5 de setembro; bloquear o `push` travaria 100% das
      entregas até que o passivo de terceiros fosse quitado

---

#### 🔴 Auditoria da Wave 4 — e os dois MLs refutaram a premissa do próprio ML

**Sondas próprias, com o binário compilado da árvore:**

```
req new "sonda integrada"   →  ✓ created ROADMAP-… + ✓ linked REQ-… → docs/roadmaps/backlog/…
validate na fixture          →  0 ocorrências de "no linked Roadmap"
grandfathering: 13 isentas (pré-corte 2026-09-03) · 0 cobradas · 232 varridas
make quality  rc 0 · 342 OK · 0 FAIL
```

### 🔴 ML-4B R1 — a regra JÁ era `error`. Este ML não inverte severidade.

O roadmap descrevia o ML como *"inverter a severidade"*. Medido: `req_has_roadmap` está **ausente de
`ruleDefaults`** e o `trackfw.yaml` daqui **não tem bloco `rules:`** → o fallback é `"error"`. E há
prova viva, não ausência de configuração: `TestReqHasRoadmapConfiguravel/default_error` **já
afirmava** isso.

🔴 **O que faz as 13 saírem como `⚠` neste repositório é `governance_mode: lenient` com
`lenient_until: 2027-12-31`.** Ou seja: **o modo de falha que o roadmap temia — *"alguém configura
`lenient` e perdemos a regra E o aviso"* — já está em vigor há tempo.** O aviso de contagem que ele
entrega é **warning**, e `applyLenientWithCarveout` só rebaixa violations — então **sobrevive ao
lenient**. Era o buraco exato.

E o *"32 REQs"* do texto estava desatualizado **duas vezes**: são **13**.

### A curva do corte satura, e o desempate NÃO é gosto

```
corte      isentas  cobradas
2026-08-16     0       13
2026-09-02    12        1
2026-09-03    13        0   ← satura
2026-09-26    13        0
```

A curva **restringe** a escolha a `≥ 09-03` e **não decide dentro** do intervalo. O desempate é
**direção de estritude**: o corte concede **anistia**, e a regra já é `error` — logo um corte mais
**tarde** anistiaria em silêncio REQ órfã datada entre 09-03 e hoje, que **hoje é erro**. **Anistia
mínima = a data mais antiga que zera o deadlock.**

⚠️ **E um efeito colateral medido a favor:** a fixture do Cenário 192 (`date: 2026-09-06`) continua
violation. **Um corte em "hoje" teria derrubado aquele braço em silêncio.**

**A régua de data, com o achado que eu pedi:** frontmatter-first com fallback para o nome — a mesma
precedência do `ML-1A`/`ML-1D`. As duas réguas **divergem em 13 dos 231** arquivos, mas 🔴 **nenhum
desses 13 é uma das 13 órfãs** — a escolha de régua **não muda veredito nenhum hoje**. E data
ilegível **falha fechado**, o que fecha o bypass de apagar `date:` e renomear.

#### Adendo R1 ao ML-4A — "caminho livre onde é intenção" é verdade no COMANDO e falso no `validate`

O próprio teste de controle dele prova: com `--no-roadmap`, o comando sai **exit 0** sem atrito, e
`req_has_roadmap` **acusa** a REQ. 🔴 **Ele não inventou marcador de isenção** (o escopo negativo
proíbe mudar a forma do vínculo) e **devolveu a decisão** — que o `ML-4B` respondeu declarando, no
residual 5, que **não há override por projeto**: é a direção restrito-demais do AC15, declarada em vez
de inferível.

### O prompt caiu por MEDIÇÃO, e a comparação está escrita

| forma | ao esquecer | não-interativo |
|---|---|---|
| flag de adesão | 🔴 **reproduz o defeito inteiro** — flag a lembrar é comando a lembrar, mais barato de digitar | ok |
| prompt | 🔴 morre no guard `cbterm.IsTerminal` — **não roda sem TTY**, ou seja, inerte em CI e sob agente; e se rodasse em pipeline, **travaria** | inerte ou travando |
| **encadeamento padrão + `--no-roadmap`** | quem esquece obtém o estado **governado** | determinístico, exit 0 |

**A assimetria que autoriza inverter o padrão:** roadmap sobrando é reversível (`roadmap move
abandoned`); **REQ órfã é irreversível no sentido que importa** — ninguém volta para consertá-la, que
é o achado desta REQ.

### Ele descartou o "pular por ADR Draft" que ia implementar, e mediu para isso

Com REQ `Open` bloqueada por ADR `Draft`: **com e sem** roadmap disparam `blocked_by_draft_adr` +
`req_has_adr`; sem roadmap dispara **também** `req_has_roadmap`. **Zero regra nova.** Manter o pular
implícito tornaria a frase de reconciliação *"o caminho integrado produz REQ não órfã"* **falsa no
caminho do wizard** — o padrão A1/A2/A3 que este projeto já pagou.

### Decisão minha sobre o residual 1 do ML-4B — ratifico o narrowing

Ele cobriu o caso **literal** do meu AC (zero cobradas ⇒ o aviso traz `13 isentas / 0 cobradas / 232
varridas`, que não é silêncio). Não cobriu **zero órfãs no total** — e a razão é medida: a alternativa
**quebra `TestValidate_Clean`**, que fixa *"estrutura vazia = zero ruído"*. 🔴 **Ratifico:** trocar um
contrato existente para satisfazer a leitura estrita de um AC meu seria pagar caro por simetria.

### Dois defeitos MEUS, corrigidos neste commit

1. 🔴 **A REQ que criei hoje era a única `1 enforced`** — porque usei o `trackfw` do **Homebrew
   (8.0.1)** em vez do binário da árvore, e o backlink do `ML-1B` não existe lá. Gravei o vínculo à
   mão; agora **`0 enforced`**. **O instrumento mente quando é o binário errado.**
2. **`duplicate ML label "ML-3D" at lines [902 1000 1068]`** — eu criei três headings `### ML-3D`. Os
   dois de auditoria viraram `####`. Agora **0**.

## Wave 5 — Fechamento
> Dependências: Waves 1–4.

### ML-5A — **AC5** — re-triagem das REQs sem roadmap
**Status:** ✅ Concluído — executado pelo `trackfw_architect` em 2026-09-26, **por leitura das 13**
**Ações:** quantas **legitimamente** não têm roadmap (decisão pura, fechada sem implementação).
🔴 O classificador heurístico da REQ **não serve** como escopo: ele casa palavras-chave e testa esse
ramo primeiro, então REQ que tenha as duas coisas cai em "decisão".
**Critérios de aceite:**
- [x] Contagem por leitura, não por heurística, com a lista

#### Resultado — **12 das 13 legitimamente sem roadmap · 1 com status errado**

Escopo: as **13** REQs que o `validate` acusa com `has no linked Roadmap` depois do ML-4B. Cada uma
lida na íntegra — título, Motivation, ACs e seção de fechamento. **Nenhuma classificada por
palavra-chave.**

| # | REQ | status | veredito | por quê — lido, não inferido |
|---|---|---|---|---|
| 1 | `2026-08-20-note-orphan-…-node` | Superseded | legítima | `npm/src/validator/index.js` deletado em `2eae0a44` |
| 2 | `2026-08-20-validate-json-do-python-…` | Superseded | legítima | `pypi/trackfw/validator.py` deletado em `2eae0a44` |
| 3 | `2026-08-28-cli-python-…-init` | Superseded | legítima | `pypi/trackfw/commands/init.py` deletado |
| 4 | `2026-08-30-fonte-unica-de-vetores` | Superseded | legítima | **premissa** eliminada: não há mais três suítes |
| 5 | `2026-08-30-roadmap-move-segue-symlink` | Superseded | legítima | 🔴 **absorvida** como **AC9** da `REQ-2026-08-31-guarda-de-folha…` — o roadmap dela **é** o da absorvedora. Regra Dura de Causa Raiz, não supersessão por v8 |
| 6 | `2026-09-01-api-chain-do-serve` | Superseded | legítima | braços Node/Python removidos; o `serve` Go responde 505 nodes / 531 edges |
| 7 | `2026-09-01-cli-node-chmodsync` | Superseded | legítima | alvo removido; o Go já usa descritor, e o **Negative Scope da própria REQ** isentava o Go |
| 8 | `2026-09-01-pypi-tty-py` | Superseded | legítima | `pypi/trackfw/tty.py` deletado |
| 9 | `2026-09-01-thirdparty-provenance-node` | Superseded | legítima | alvo removido; a regra vive em `internal/validator/validator_thirdparty_provenance.go` |
| 10 | `2026-08-21-nil-map-em-projectconfig` | Done | legítima | 🔴 corrigida **dentro do roadmap da REQ que a causou** — `ROADMAP-2026-08-21-versao-do-modelo-por-tier`, **ML-2C "Fechar a classe do nil map"**. Absorvida, não órfã |
| 11 | `2026-08-30-titulo-de-roadmap-com-newline` | Done | legítima | já corrigida pela `ADR-2026-08-23` quando foi triada — `roadmap new` responde *"title must be a single line"* |
| 12 | `2026-09-02-job-parity-13m23s` | Done | legítima | **decisão pura**: a medição **descartou** o ganho proposto e nomeou sucessora. Nada a implementar, por construção |
| 13 | `2026-08-16-conformidade-de-i18n-entre-os-tres-clis` | **Open** | 🔴 **não legítima** | ver abaixo |

#### 🔴 O achado: a única Open das 13 não precisava de roadmap — precisava ser fechada

Os três sujeitos da REQ **não existem**:

```
$ git ls-files npm/src                               → 0
$ git ls-files pypi/trackfw                          → 0
$ git ls-files pypi/trackfw/generators/roadmap.py    → (nada)   ← o sítio citado na Motivation
$ git ls-files '*locales*'
internal/i18n/locales/{en-US,es-ES,pt-BR}.json                 ← sobra só o Go
```

O AC1 dela — *"os 3 CLIs produzem saída **byte-idêntica** sob locale fixo"* — **perdeu o objeto**: não
há mais três saídas para comparar. Fechada como `Superseded` com o **mesmo motivo medido** das 8
irmãs. A seção de fechamento está na própria REQ.

🔴 **Isto é o oposto do que o `req_has_roadmap` sugeriria.** A regra diria *"dê um roadmap a esta
REQ"*; o correto era **fechá-la**. É exatamente o escopo negativo deste roadmap — *"roadmap vazio
gerado em massa parece cobertura e não é"* — visto do outro lado: **órfã não é sinônimo de trabalho
pendente**, e o grandfathering do ML-4B existe porque tratar as duas coisas igual produziria 13
roadmaps decorativos.

#### ⚠️ Uma afirmação deste mesmo roadmap, refutada

A seção *"Medição de 2026-09-22"* acima declara, sobre as remanescentes:

> *"São **9 `Superseded`** … e **3 `Done`** históricas. **Nenhuma `Open`.**"*

**Falso, e verificável no git:**

```
$ git show 596a694b:docs/req/REQ-2026-08-16-conformidade-…-clis.md | head -6
status: Open
roadmap: ""
```

A régua daquela contagem errou por **1**, e o erro caiu **justamente na única das 13 que exigia
decisão** — as 12 legítimas foram classificadas certo. Não é aleatório: aquela contagem deduziu o
grupo *"Superseded/Done"* das que **tinham seção de fechamento**, e a 08-16 não tinha porque nunca
havia sido triada. **O caso não coberto pela heurística era o único caso com trabalho.**

Causa a montante, medida: a triagem de 2026-09-18 cobriu **31 das 40** REQs abertas, e esta ficou nas
**9 não triadas**. Triagem parcial que não enumera o resto do universo deixa o resíduo indistinguível
de trabalho vivo. Consistente com o mecanismo que este roadmap já registrou duas vezes — `blocked`
com condição vencida, e a tabela de residuais que virou #400/#401/#402.

**Efeito no acervo:** REQs `Open` caem de **25 → 24**; as 13 órfãs ficam **13 isentas por
grandfathering, 0 cobradas** — e agora **13 decididas por leitura**, o que antes não era verdade.

### ML-5B — **AC6** — paridade e fechamento
**Status:** ✅ Concluído — **CI verde: 21/21**, `state=CLEAN` no PR #446 (head `a28b6461`)
**Critérios de aceite:**
- [x] `make quality` verde  → remedido após o `ML-6A`: `exit=0`, **1360 `^OK `**, **0 `: FALHA`**
- [x] `trackfw validate` sem violation nova  → **0 violations, rc=0**, **171 warnings**
- [x] **CI verde** — medido no PR, não aqui → **21 checks, 21 SUCCESS**, `mergeable=MERGEABLE state=CLEAN`

🔴 **E foi o CI que achou o que o local não achava.** Os 4 testes do `ML-6B` reprovavam no Windows
**desde a Wave 1** e ninguém tinha visto: o `windows-full-suites` só dispara em `pull_request`, e
esta branch não teve PR até o fim da Wave 6. O critério *"CI verde"* não era burocracia — era o único
instrumento que enxergava aquela plataforma. Ver `ML-6B`.

⚠️ **Este ML foi remedido.** A primeira medição (1359 OK) foi feita **antes** de a auditoria pré-PR
achar o `AC16`. Um `make quality` verde numa REQ com AC não entregue não afirma o que parece afirmar —
por isso a contagem que vale é a de **depois** da Wave 6.

#### Medição, com a ressalva

```
make quality                 exit=0   OK=1359   FALHA=0
trackfw validate             rc=0     violations=0   warnings=171
trackfw barrier … --wave 5   ✓ wave_headings  ✓ gates  ✓ validate
```

⚠️ **Um warning que eu declarei zerado na Wave 4 não estava zerado.** O `validate` acusou
`duplicate ML label "ML-4A" at lines [1100 1216]`, e verifiquei em `HEAD` que **já estava commitado** —
o heading `### ML-4A R1` do adendo é lido como um segundo ML com o mesmo rótulo. Rebaixado para
`#### Adendo R1 ao ML-4A — …`, e o warning foi a **0**.

🔴 **Registro o erro de método, não só o conserto:** na Wave 4 eu reportei *"`duplicate ML label` → 0"*
contando **depois** de corrigir três headings `### ML-3D`, e não **reexecutei a contagem no artefato
commitado**. A régua estava certa; a hora de aplicá-la estava errada. É a terceira vez nesta campanha
que uma contagem minha só sobrevive porque alguém a repetiu no artefato final — as duas anteriores
foram o `SITE_FLOOR` defasado e o censo de 185/111 que era 228/205.

---

## Wave 6 — o AC16, que a auditoria pré-PR encontrou sem ML nenhum (🔴 achada na auditoria)
> Dependências: Waves 1–5. **Bloqueia o PR** — não é wave opcional.

### Por que esta wave existe

Antes de abrir o PR eu confrontei as **16 ACs da REQ** com a evidência escrita, em vez de conferir os
checkboxes. Quinze têm ML `✅` auditado. **O AC16 não tem ML nenhum** — foi acrescentado à REQ em
2026-09-26, ao corrigir a atribuição do **#439**, e o roadmap **nunca** foi atualizado para cobri-lo.
`grep -n AC16 <roadmap>` → **vazio**.

**Reproduzido ao vivo em 2026-09-26**, com o binário desta branch, em layout com subpastas de estado
(que é o do consumidor):

```
$ trackfw req move REQ-2026-09-26-x done
✓ moved REQ-2026-09-26-x.md → docs/req/done

$ grep '^req:' docs/roadmaps/wip/ROADMAP-x.md
req: "docs/req/wip/REQ-2026-09-26-x.md"     ← caminho antigo

$ test -f docs/req/wip/REQ-2026-09-26-x.md
NÃO EXISTE — vínculo quebrado
```

Confirmado no código: `MoveREQ` (`internal/generators/req.go:358`, **115 linhas**) chama apenas
`rewriteREQStatus`. A contraparte `MoveRoadmap` chama `syncREQReferences` em `roadmap.go:803`. **A
assimetria está no fonte, não na interpretação.**

🔴 **Por que não dá para deferir:** é a **mesma causa** desta REQ — *"o comando conhece o vínculo e não
o escreve"* — e a Regra Dura de Causa Raiz é literal: *mesma causa → mesma REQ → **mesmo PR***, e
*"não se mergeia o PR parcial prometendo o resto depois"*. Fechar aqui produziria exatamente o achado
A1 da auditoria externa: ADR de ponto único marcada satisfeita com sítio conhecido sobrando.

⚠️ **E o efeito medido é do consumidor, não nosso:** neste repositório `req_dir` é **flat**, então
`req move` só reescreve o `status:` e o defeito **não aparece**. Ele só se manifesta em layout com
subpastas de estado — as **78 violações de `stale state path`** congeladas no baseline do consumidor.
É a quarta vez nesta campanha que o sinal vem de fora porque o upstream não exercita o layout do
consumidor.

### ML-6A — **AC16** — `req move` escreve o vínculo de volta, simétrico ao `roadmap move`
**Status:** ✅ Concluído — auditado em 2026-09-26 · e o corretivo R1 removeu um teste que afirmava o
nome errado
**Arquivos:** `internal/generators/req.go` · `internal/generators/roadmap.go` (só se o ponto único
exigir) · `internal/generators/req_test.go`
**Ações:**
1. Ao mover a REQ de pasta, atualizar o `req:` do **roadmap vinculado** para o novo caminho — pelo
   mesmo mecanismo de `syncREQReferences`, não por uma segunda implementação.
2. 🔴 **Decidir e escrever qual é o ponto único.** `syncREQReferences` descobre REQs cujo `roadmap:`
   já aponta para o roadmap; aqui o sentido é o inverso. Se a simetria exigir função nova, ela é
   **uma**, e o comentário diz por que não deu para reusar — `roadmap.go:1394` já tem a distinção
   escrita, e é o lugar certo para ancorar a decisão.
3. Anunciar a sincronização em stdout, como `roadmap move` já faz. Silêncio aqui reproduz o *"o
   trabalho é feito, mas o controle não aparece no relatório"* que esta casa já pagou.
**Critérios de aceite:**
- [x] Reprodução acima passa a sair com o `req:` **atualizado**, e o caminho existe
      → `✓ synced ROADMAP-x.md → docs/req/done/REQ-2026-09-26-x.md`, e o arquivo existe lá
- [x] 🔴 **Contra-braço:** `req move` em `req_dir` **flat** (este repositório) **não** altera nada além
      do `status:` → `git status --porcelain docs/` acusa só a REQ movida; nenhuma das 232 tocada.
      As branches in-place retornam antes da sincronização, porque `dst == path`
- [x] 🔴 **Recusa preservada:** vínculo **cruzado** não é criado
      → `TestMoveREQ_CrossLinkGuard_NotRewritten`; o guard compara o basename do `req:` do roadmap
- [x] Contenção de escrita: sítio novo agido, sem `_ = RefuseUnverifiableRoot(...)`
      → `check-write-containment: 161 sítio(s) em 109 arquivo(s) — OK`
- [x] A frase da Regra Dura de Reconciliação, por teste novo → 4 frases, uma por teste
- [x] `make quality` verde e `trackfw validate` sem violation nova
**Validação:** `go test ./internal/generators/ -run 'MoveREQ|ReqMove' -v` · `make quality`

#### O ponto único, e por que não deu para reusar `syncREQReferences`

`rewriteREQRoadmapRefWith` foi **generalizado** com `(fmKey, bodyKey string, bodyOnce bool)`. Os dois
callers que já existiam passam `"roadmap", "Roadmap", false` e mantêm o comportamento anterior; o
caller novo passa `"req", "REQ", true`. **Não** nasceu uma segunda implementação de reescrita de
frontmatter — que era o risco que o handoff nomeava.

`syncREQReferences` **descobre** REQs cujo `roadmap:` já aponta para o roadmap movido. Aqui o sentido é
o inverso: parte-se da REQ e chega-se ao roadmap. O que as duas compartilham é o ponto único acima.

#### 🔴 Auditoria do ML-6A — dois achados meus, e uma régua minha que caiu

**R1 — um teste afirmava o que o nome não dizia.** `TestMoveREQ_SyncIdempotent` criava uma **segunda
REQ diferente** e verificava que o roadmap não era reescrito — isto é o **cross-link guard**, predicado
que `TestMoveREQ_CrossLinkGuard_NotRewritten` já afirmava. E a **idempotência real ficou sem
cobertura**, apesar de o lado espelho (`syncREQReferences`) tê-la em `roadmap_test.go`.

🔴 **O nome de um teste é uma afirmação.** Quem lesse a lista concluiria que a idempotência estava
coberta. É a classe exata que originou a Regra Dura de Reconciliação nesta casa — o caso `ENOTDIR` de
2026-09-05, em que o relatório media uma coisa e o teste da mesma entrega afirmava o contrário.
Corrigido: removido (sobreposição total medida, uma única condição de guard) e substituído por
`TestMoveREQ_SyncRoadmapREQRef_Idempotent`, que compara **bytes** — não `mtime`, cuja granularidade
faria o teste passar pelo relógio em vez de pelo predicado.

**R2 — o `SITE_FLOOR` absorveu defasagem pré-existente.** Ele subiu 158 → 161 num movimento. Medi
contra `HEAD` limpo, com `git archive`:

```
HEAD limpo:  160 sítio(s) examinado(s), floor declarado 158   ← defasagem de 2, JÁ existia
ML-6A:       +1 sítio
```

O número final está certo; a **atribuição** não estava. Separado em dois commits, como na Wave 3.

**⚠️ E o número de `validate` que ele reportou não era desta branch.** Ele reportou *"152 warnings,
lenient mode"*; eu medi **171**. A diferença é o **binário**:

```
/tmp/tf-6b  (build desta branch)   → 171 warning(s)
trackfw     (Homebrew 8.0.1)       → 152 warning(s)
```

🔴 O handoff dizia, literalmente, para usar o binário compilado. Não muda a conclusão (0 violations
nas duas réguas), mas o número publicado era de **outro artefato** — e é o mesmo erro que eu cometi
no ML-1B desta campanha, quando quase reportei um ML como falho medindo com o binário do Homebrew.

**⚠️ Minha régua sobre `bodyOnce`, refutada pelo contra-braço.** Tentei falsificar a heurística *"a
primeira linha `REQ:` do corpo é sempre a linha de contexto"* varrendo os roadmaps e marcando como
suspeito o que tivesse backtick: **94 de 184**. Olhando o resultado, o backtick é o **formato do
próprio template** — minha régua marcou o padrão, não a anomalia. **Nenhum contra-exemplo encontrado**;
o risco residual é contido pelo cross-link guard, que só reescreve se o basename casar.

---
      Parecer (`docs/seguranca/2026-10-09-wave6-roadmap-new-sobrescreve.md`): `internal/generators/roadmap.go:300`
      `os.WriteFile` sem checagem de existência; medido: sobrescreve a vinculada (`req: ""`), destrói edição manual
      imprimindo `✓ created`, e com a roadmap já em wip cria uma segunda em backlog. Texto gerado que manda rodar os dois:
      `agentfiles.go` (AGENTS/GEMINI/copilot/windsurf/cursor), `claudemd.go`, mais `CLAUDE.md` e `README.md` deste repo.
      Decisão do arquiteto: opção B (idempotente — existe roadmap com o mesmo nome-base em QUALQUER estado → não
      escreve, avisa e repara o vínculo de volta) + C (`--force` só sobrescreve no mesmo caminho). Criação com
      `O_EXCL` em vez de Stat+Write (fecha o TOCTOU).

### ML-6B — os 4 testes do vínculo comparavam separador nativo com valor portável
**Status:** ✅ Concluído — auditado em 2026-09-26 · 🔴 **e refutou a minha hipótese**
**Arquivos afetados:** `internal/generators/roadmap_backlink_ml1b_test.go` · `internal/commands/req_chain_ml4a_test.go`
**Critérios de aceite:**
- [x] Os 4 testes passam no `windows-full-suites` do PR #446 — **medido no CI**
      → **21 checks, 21 SUCCESS**, `state=CLEAN`, head `a28b6461`
- [x] Para cada um: decisão **(a)** ou **(b)** escrita, com a linha do código e a do teste
      → **(b) nos 4**, mesma causa raiz, justificada pela `ADR-2026-09-04` D1
- [x] 🔴 **Contra-braço:** a correção **não** quebra POSIX → `go test ./internal/...` verde no macOS
- [x] `.github/windows-known-failures.json` **inalterado** → confirmado por `git diff --stat`
- [x] Se algum dos 4 tiver causa diferente, isso está escrito
      → **não tem**: a hipótese de segunda causa para `…NonCanonicalAbsoluteREQPath` foi
      **descartada por medição** — o CI mostra `✓ linked` nos 4, então o `pathguard` não rejeitou
- [x] A frase da Regra Dura de Reconciliação por teste alterado → 4 frases
- [x] `make quality` `exit=0` (1360 `^OK `, 0 `: FALHA`) e `validate` rc=0 sem violation nova
**Validação:** `gh pr checks 446` · `go test ./internal/generators/ ./internal/commands/`


Achado pelo **primeiro** CI desta branch: `windows-full-suites` vermelho, 4 falhas novas, todas do
`ML-1B` e do `ML-4A` — **Waves 1 e 4**. Estavam vivas desde a Wave 1, invisíveis porque a suíte de
Windows só roda em `pull_request`.

**A causa é o inverso do que eu escrevi no handoff.** Eu disse que *"(b) o defeito é do teste é a
resposta confortável e costuma ser a errada"*. A medição:

```
produto grava:   roadmap: "docs/planos/backlog/ROADMAP-….md"    ← com "/"
teste esperava:  "docs\planosacklog\ROADMAP-….md"             ← retorno de filepath.Glob
```

O produto **obedece** a `ADR-2026-09-04` D1 — separador portável em artefato autorado cujo consumidor
não é o sistema de arquivos. Os testes é que usavam o retorno de `filepath.Glob`, nativo no Windows,
como valor esperado. **Decisão (b) nos 4** — e aqui ela não é a saída preguiçosa, é o que a ADR exige.

**Convergência por dois caminhos independentes:** o executor mediu pelo **dump do CI**; eu reproduzi
na **VM de Windows** (worktree isolado — a VM investiga, o `windows-latest` mede). As duas leituras
batem, e nenhuma dependeu da outra.

**Correção:** `filepath.ToSlash` no valor vindo do `Glob`, mais uma assertion por teste afirmando que
o campo gravado **não contém** `\` — a direção que nenhum teste cobria.
🔴 **`.github/windows-known-failures.json` INALTERADO.** Silenciar o ratchet transformaria defeito
medido em ruído permanente — o padrão que esta casa mediu 59 vezes.

⚠️ **Ressalva da auditoria, registrada sem bloquear:** a assertion nova da ADR D1 é condicional
(`if strings.Index(…, "roadmap: \"") >= 0`), então **sozinha** passaria vacuamente se o campo
sumisse. Não é defeito porque a assertion vizinha, no mesmo teste, falha nesse caso — mas se alguém
mover uma das duas, a outra deixa de ser suficiente.

⚠️ **O executor reportou `151 warnings` do `validate`.** É o binário do Homebrew 8.0.1; a branch
reporta **170**. Conclusão idêntica (0 violations), procedência diferente. **Segunda ocorrência do
mesmo erro em dois MLs seguidos** — o handoff avisa, e ainda assim acontece.

---

## Auditoria pré-PR das 16 ACs da REQ — 2026-09-26

Feita **por confronto com a evidência escrita**, não pelos checkboxes. É o controle que o achado A1 da
auditoria externa de 2026-09-05 exige.

| AC | ML | veredito |
|---|---|---|
| AC1 · AC10 | ML-4A | ✅ auditado |
| AC2 · AC3 | ML-4B | ✅ auditado — com a correção de que a regra **já era** `error` |
| AC4 | ML-4C | ✅ decidido: só o `validate`; o `push` não muda |
| AC5 | ML-5A | ✅ 12 de 13 legítimas · 1 fechada |
| AC6 | ML-5B | ⚠️ **emendado** — ver abaixo |
| AC7 | ML-1C | ✅ auditado — o defeito era maior: o nome exato também movia errado |
| AC8 | ML-1B | ✅ auditado |
| AC9 | ML-1A + ML-1D | ✅ auditado — o ML-1D existe porque o AC do ML-1A não estava implementado |
| AC11 | ML-2A | ✅ ADR escrita, com emenda do mesmo dia |
| AC12 · AC13 · AC15 | ML-3A + ML-3B | ✅ auditado — 29 de 205, não 2 de 111 |
| AC14 | ML-3C | ✅ auditado — e o braço substring é praticamente morto |
| — | ML-3D | ✅ entregável descoberto, não previsto por AC |
| AC16 | ML-6A | ✅ auditado — entregue na Wave 6, com o corretivo R1 |

### ⚠️ Emenda ao AC6 — perdeu o objeto, e digo isso em vez de marcá-lo

O AC6 diz *"paridade nos 3 CLIs"*. A REQ é de **2026-09-09**; a **v8.0.0** (`2eae0a44`) removeu os CLIs
Node e Python. **Não há mais três CLIs para ter paridade.** Marcar este AC como atendido seria afirmar
uma verificação impossível; deixá-lo em branco sem explicação seria deixar a REQ eternamente aberta.

**Fica assim:** o AC6 é lido como *"`make quality` verde e CI verde na implementação única em Go"* —
que é o que o `ML-5B` mede. A emenda está escrita aqui e na própria REQ. 🔴 **É o mesmo mecanismo que
o ML-5A acabou de achar na `REQ-2026-08-16`**: AC redigida contra três runtimes, sobrevivendo à
remoção deles. Vale perguntar quantas outras REQs abertas têm AC nessa condição — e essa pergunta é
**causa diferente** desta REQ, logo não entra aqui.

---

## ⚠️ Resíduo desta wave, que NÃO é desta causa

O `ML-5A` mediu que a triagem de 2026-09-18 cobriu **31 das 40** REQs abertas, e a `REQ-2026-08-16`
estava entre as **9** não triadas. **Tentei enumerar as 9 e não consegui** — e o motivo importa:

```
régua tentada:   REQ Open, anterior a 2026-09-18, sem marca de triagem   → 21
contra-braço:    REQ Open COM marca de triagem                           → 0
```

🔴 **O contra-braço derruba a régua.** A marca de triagem só foi escrita nas REQs que a triagem
**fechou**; as que ela examinou e declarou *"VIVE"* não receberam marca nenhuma. Então o filtro não
distingue *triada-e-viva* de *não-triada*, e o `21` é o universo, não o resíduo. **A lista das 31 não
existe em nenhum artefato** — `docs/triagem-reqs-abertas-2026-09-18.md` registra só o agregado
(13 obsoletas / 18 vivem) e cita **1** REQ por nome.

**Não invento o número.** O que fica medido: **21 REQs `Open` anteriores à triagem, das quais no
máximo 18 foram declaradas vivas** — e nenhuma carrega prova de ter sido examinada.

Causa disto é *"triagem registra agregado e não lista nominal"*, **diferente** da causa desta REQ. Por
isso **não** entra aqui: vira issue, com a régua falsificada acima escrita, para não ser a quinta
ocorrência do mecanismo que este roadmap diagnosticou.

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

## Wave 6 — Reabertura (AC8): `roadmap new` sobrescreve a roadmap que o `req new` criou
> Reaberto em 2026-10-09. **Por que o escopo original não previa:** o AC1 fez o `req new` criar a roadmap, mas o
> `roadmap new` e o texto de protocolo gerado continuaram como antes — a sequência ensinada destrói o vínculo.

### ML-6A — Threat model e enumeração
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Medição com o binário da main: `req new` + `roadmap new` mesmo título; título diferente; `--from-req`; roadmap já em wip/done; o que o `roadmap new` faz hoje com arquivo existente (sobrescreve? em que caminho do código?)
- [x] Enumeração de todo texto gerado que manda rodar `req new` e depois `roadmap new` (templates de CLAUDE.md, AGENTS.md, GEMINI.md, regras de Cursor/Windsurf/Kiro/Copilot, skills, docs) — por grep, não por memória
- [x] Decisão recomendada (recusar com erro? vincular sem sobrescrever? `--force`?) e Veredito

### ML-6B — Correção
**Status:** 🔄 Em andamento
**Squad:** apolo-tf
- [x] `roadmap new` nunca sobrescreve roadmap existente sem opt-in explícito; teste nas duas direções com falsificação
- [x] Texto de protocolo gerado atualizado em todos os sítios do ML-6A; testes que pinam o texto ajustados
- [x] `make quality` (arquiteto)
      Auditoria (2026-10-09): 5 testes conferidos por nome. Binário real num projeto temporário: `req new "Teste T"` +
      edição à mão + `roadmap new "Teste T"` → aviso, arquivo byte-idêntico (hash igual), nota manual e `req:` preservados;
      com a roadmap em wip → nenhuma cópia em backlog; `--force` com a existente em wip → erro. `make quality` EXIT=0
      (executor), 347 OK. Texto de protocolo atualizado em agentfiles.go, claudemd.go, CLAUDE.md, AGENTS.md, GEMINI.md,
      pypi/AGENTS.md e README.

### ML-6C — Red-team
**Status:** ✅ Concluído
**Squad:** hades-tf
- [x] Parecer sobre o diff
      Veredito do hades-tf (`docs/seguranca/2026-10-09-red-team-roadmap-new-sobrescreve.md`): aprova com ressalvas, 4
      achados baixos. Decisão do arquiteto: A2 (roadmap deixada com `req: ""` pela versão antiga não é reparada sem
      `--req`) e A4 (`--req REQ-B` sobre roadmap de REQ-A cria vínculo falso) são a mesma causa — o caminho "já existe"
      escrevendo/deixando vínculo errado → ML-6D. A3 (`--force` sem `--req` zera o vínculo) → texto do help. A1 (by_agent
      com `--agent` diferente cria órfã noutro namespace, sem perda) → residual; o validate acusa.

### ML-6D — Corretivo do red-team: vínculo no caminho "já existe"
**Status:** ✅ Concluído
**Squad:** apolo-tf
- [x] A2: roadmap existente com `req: ""` e exatamente uma REQ apontando para ela → `req:` reparado sem `--req`
- [x] A4: `--req` diferente do `req:` já gravado → não vincula, avisa; nenhuma das duas REQs alterada
- [x] A3: help do `--force` diz que o vínculo é recriado só com `--req`
- [x] Testes nas duas direções, falsificação; `make quality` (arquiteto)
      Auditoria (2026-10-09): 6 testes conferidos por nome (A2 ×3, A4 ×3 incluindo `TestRoadmapNew_A4_OrphanRoadmap_GetsReqFilled`
      — complemento pedido pelo arquiteto: `--req R` sobre roadmap órfã também grava `req:` na roadmap); falsificações de A2,
      A4 e do complemento registradas; árvore só com os arquivos declarados. `make quality` (arquiteto) EXIT=0, 347 OK.
      Residual: by_agent com `--agent` diferente (A1) — o validate acusa a órfã.


### ML-6E — A1: by_agent com `--agent` diferente
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Por que:** decisão do KG (2026-10-09): o residual A1 vira erro em breve — corrigir aqui.
**Decisão do arquiteto:** em `roadmap_namespacing: by_agent`, a procura por roadmap de mesmo nome-base varre TODOS os
namespaces de agente (e as pastas de estado flat, se existirem). Achou noutro agente → mesmo tratamento de "já existe
em outro estado": não grava, avisa nomeando o caminho e o agente, repara vínculo pelas regras A2/A4; `--force` recusa.
- [x] `req new --agent A T` + `roadmap new --agent B T` → nenhuma roadmap nova em B; aviso; vínculo íntegro
- [x] Mesmo agente e flat continuam como no ML-6B/6D (sem regressão)
- [x] Testes, falsificação; `make quality` (arquiteto)
      Auditoria (2026-10-09): 5 testes A1; binário num projeto by_agent: `roadmap new --agent apolo-tf` com a roadmap em
      zeus-tf → aviso nomeando o dono, nada criado; `--force` → exit 1. Falsificação registrada. Bomba-relógio achada pelo
      executor nos testes A2/A4 do ML-6D (data do dia fixa no nome do arquivo — reprovariam a partir de 2026-10-10;
      falha de auditoria minha): corrigida com `ac8Today()`. `make quality` (arquiteto) EXIT=0, 347 OK.

