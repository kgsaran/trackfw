# Plano de Convergência — Harness pessoal do KG → trackfw

> Análise doc-only. Data: 2026-07-26. Autor: Zeus.
> **Objetivo do KG:** usar **somente** o trackfw, aposentando `~/.claude/CLAUDE.md` +
> `~/.claude/agents/*.md` + `~/.claude/skills/*`.
> **Premissa:** o framework nasceu no projeto CMDB + Panteão Grego — essa é a fonte de verdade
> em kanban, git flow, coordenação e ferramentas.

> ⚠️ **Governança:** documento de análise/decisão, não implementação (exceção §7).
> **Ao aprovar as respostas dos questionários, a sequência obrigatória antes de qualquer edição é:**
> `trackfw req new` → `trackfw roadmap new` → `trackfw roadmap move <nome> wip` → `git checkout -b feat/<slug>` → só então código.

---

## Parte 0 — Mecânica: o que é barato e o que é caro

| Mecanismo | Estado nos fontes | Consequência |
|---|---|---|
| **Sync de assets 3 CLIs** | `scripts/sync-integration-assets.sh` gera npm+pypi do canônico `internal/integrations/assets/`; `check-integration-assets.sh` valida byte-identity dentro de `make quality` | ✅ **Enriquecer agents é barato** — edita-se **1 árvore** e roda o sync. Não são 30 arquivos à mão. |
| **Alcance** | 9 targets: claude, codex, gemini, antigravity, cursor, copilot, windsurf, amazonq, kiro | ✅ Cada melhoria propaga para 9 ferramentas |
| **Presets de identidade** | **10 presets** + teste `TestPreset_EveryPresetCoversExactlyKnownAgentIDs` exigindo cobertura exata | 🔴 **Cada agente novo = 10 nomes × 3 CLIs = 30 entradas** + catalog.json + asset + tool set |
| **`agentTools()` serve para `tools:` do Claude?** | ❌ **Não.** SET_IMPL/SET_ARCH usam IDs do **agy/Windsurf** (`view_file`, `grep_search`, `run_command`) | 🔶 Correção da minha afirmação anterior: exige **novo mapeamento** para Claude Code |
| **Corpo variável por perfil** | ❌ Inexistente. `catalog.json` tem **um** `asset` por item; `Render()` só reescreve `name:`/`description:` + saudação | 🔴 "Base neutra + perfil opt-in" é **capacidade nova nos 3 CLIs** |
| **Estado `analyzing`** | Existe no scaffold, no CLAUDE.md gerado, no board e no codex — **mas o validator só conhece 5 estados** | 🔴 Inconsistência **interna do trackfw** → Q7 |

---

## Parte 1 — O que trazer, por camada

### 1.1 — Camada UNIVERSAL (vale para os 10 + N agentes)

| # | Item | Origem / evidência | Custo | Recomendação |
|:-:|---|---|---|---|
| U1 | **LOCK DE MODO** — "pinnado como X; este arquivo é sua única autoridade; em violação responda 'LOCK VIOLADO'" | **14/14** do panteão, texto quase idêntico | Baixo — 1 bloco × 10 assets | ✅ Trazer |
| U2 | **Registro em `docs/agents-working-context.md`** ao INICIAR e CONCLUIR | **14/14** | Baixo. trackfw já usa o arquivo (`trackfw context`) | ✅ Trazer |
| U3 | **`memory: project`** no frontmatter | **14/14** | Trivial | ✅ Trazer |
| U4 | **`tools:` explícito** por papel | **14/14** (Hades/Hefesto/Athena/Cronos sem Edit/Write — read-only por design) | 🔶 Médio — exige mapeamento novo (agy≠Claude) | ✅ Trazer + definir 3 conjuntos: ARCH / IMPL / REVIEW |
| U5 | **Anti-alucinação** — "análise estática antes de qualquer sugestão/edição" | Só **3/14** (afrodite, artemis, zeus) com 3 headings diferentes | Baixo | ✅ Normalizar para 10/10 |
| U6 | **Restrição de escopo / handoff** — "fora do meu domínio, handoff" | 4 formas distintas (`Restrição de escopo`, `Skill Boundary`, bullet de Workflow) | Baixo | ✅ Normalizar nome e texto |
| U7 | **Assinatura obrigatória** | Inconsistente: **5 agentes declaram `Assine:` e não assinam** | Baixo | ✅ **Trazer em EN** (Q2b) — sem preset usa o papel; com preset usa o DisplayName |
| U8 | ~~Mandato de idioma PT-BR~~ | 14/14, em 5 redações | — | ❌ **Descartado por Q2** (assets em EN) |

### 1.2 — Adendo ORQUESTRADOR (`architect`)

| # | Item | Origem | Observação |
|:-:|---|---|---|
| O1 | **Permissões git exclusivas** — única entidade autorizada a `git checkout -b`; commit restrito a `docs/`+`vault/`; `git push`; `gh pr create` sob pedido | `zeus.md` (1/14) | Núcleo do modelo. trackfw hoje não diz nada sobre quem cria branch |
| O2 | **Paralelização** — critério, padrão de wave, **barrier**, 4 regras de spawn (prompt autocontido, isolamento, barrier explícita, label), 3 anti-padrões | `zeus.md` (1/14) | Nenhum equivalente no trackfw |
| O3 | **Workflow arquiteto (10 passos)** — Análise→ADR→Roadmap→Branch→Commit docs→Handoffs→Auditoria→Commit final→PR→Monitor | `zeus.md` | Amarra O1+O2 numa sequência |
| O4 | **Auditoria de conformidade pós-ML** — validar critérios de aceite antes de liberar a próxima wave | `CLAUDE.md §4` | Ausente em todo o panteão e no trackfw |
| O5 | **Proibição de codificar** — "NÃO escreve lógica de negócio, componentes, migrações ou scripts de infra" | `zeus.md` + `CLAUDE.md §3` | trackfw diz só "do not implement product code" |
| O6 | **Tools de orquestração** — `Agent`, `Task*`, `Monitor`, `PushNotification`, `Enter/ExitPlanMode` | `zeus.md` (único) | Depende de U4 |

### 1.3 — Adendo IMPLEMENTADOR (os 9 demais)

> **Esta é a maior lacuna.** Hoje existe em **2/14** (afrodite, artemis) + 1 versão divergente (dedalo).
> Seis implementadores com `Edit/Write` — **apolo, ares, hermes, metis, poseidon, prometheus** — não têm
> **nenhuma** regra de kanban. "Trazer do panteão" aqui é, na verdade, **definir uma vez e aplicar aos 10**.

| # | Item | Origem | Observação |
|:-:|---|---|---|
| I1 | **KANBAN como pré-requisito** — não editar código sem REQ+ROADMAP em `wip` | afrodite+artemis (2/14) | É o que `branch_has_wip_roadmap` já valida — **o produto valida algo que seus próprios agentes não ensinam** |
| I2 | **GIT_FLOW do implementador** — "você NÃO pode criar branch/PR" | afrodite+artemis; dedalo/prometheus em outra redação | Complemento de O1 |
| I3 | **Protocolo de conclusão de ML** — build → teste → gate → commit → push → atualizar roadmap | `CLAUDE.md §2.1` | trackfw tem o gate (`validate`), não a sequência |
| I4 | **Atualizar o ROADMAP ao fim de cada microlote** — "não pode ser pulado" | `kanban-flow` #43 | trackfw já pede status ⬜→🔄→✅ no CLAUDE.md gerado |
| I5 | **Build obrigatório antes de concluir** | 4/14 | Combina com o `Pre-commit checklist` já gerado |
| I6 | **VIBE CODING AUTONOMOUS** — loop autônomo até verde, sem pedir confirmação | 2/14 | 🟡 sobrepõe o **Autopilot** do `CLAUDE.md §6` — **Q8** |
| I7 | **Vault de conhecimento** | 3/14 (afrodite, artemis, hermes) | 🟡 trackfw não tem conceito de vault — **Q9** |

### 1.4 — Camada CLAUDE.md gerado (`internal/generators/claudemd.go`)

| # | Bloco a trazer | Origem | Por que importa |
|:-:|---|---|---|
| C1 | **Branch strategy completa** — uma branch ativa; protocolo de 3 passos + **Passo 3-bis** (branches defasadas, `touched`/`diverg`) | `CLAUDE.md §1` | 60 linhas sem equivalente. Resolve squash-merge, que o `git branch --no-merged` não detecta |
| C2 | **Protocolo pós-aprovação de plano** — REQ→Roadmap→wip→branch, com proibições nomeadas | `CLAUDE.md §1` | Já parcialmente no CLAUDE.md gerado (regra 1) |
| C3 | **Definition of Done "sem rabo"** — 5 critérios; "build/test verdes **não** encerram o ML, o fluxo Kanban encerra" | `kanban-flow` #49-56 | Conceito mais forte do harness e ausente do trackfw |
| C4 | **Escopo negativo obrigatório na REQ** — "toda REQ declara o que o agente NÃO pode implementar" | `kanban-flow` #18 | Ausente no template de REQ do trackfw |
| C5 | **Alto detalhamento anti-alucinação** — roadmap executável por modelo simples, com snippet copy-paste em todo ML | `kanban-flow` #19, #41, #106-107 | Alinhado ao "decision-complete" da skill `plan` |
| C6 | **Exigências por estado** — `blocked` exige motivo+responsável; `abandoned` exige motivo+sucessor | `kanban-flow` #13-15 | trackfw tem as pastas, não as exigências |
| C7 | **Exceção de trivialidade** (lista fechada) | `CLAUDE.md §7` | 🟡 **Q4** — colide com "never code without REQ+ROADMAP" |
| C8 | **Prototipagem iterativa A→B→C** | `CLAUDE.md §9` | Sem equivalente |
| C9 | **Bug de produção: inspecionar ambiente live antes** | `CLAUDE.md §10` | Sem equivalente |
| C10 | **Bug concreto: fix direto sem expansão** | `CLAUDE.md §11` | 🟡 **Q4** — colide com C3/I1 |
| C11 | **Autopilot** — perguntar tudo antes, não interromper depois | `CLAUDE.md §6` | Sem equivalente |
| C12 | **Formato de roadmap** (waves + MLs + critérios) | `CLAUDE.md §5` | 🟡 **Q5** — colide com o template de `kanban-flow` |

---

## Parte 2 — Questionários de conflito

> Conflitos **reais e verificados**. Vários são internos ao seu próprio harness — a convergência
> obriga a resolvê-los de qualquer forma.

> ## ✅ DECISÕES TOMADAS (2026-07-26)
>
> | # | Decisão | Consequência registrada |
> |---|---|---|
> | **Q1** | **Default para todos** — harness completo vira o comportamento padrão | Perfis opt-in fora de escopo → **não** é preciso criar corpo variável no catálogo. Economia grande. |
> | **Q2** | **Inglês** nos assets e no CLAUDE.md gerado | **U8 cai** (sem mandato "100% PT-BR"). Idioma/persona voltam via `--preset` e via o CLAUDE.md do projeto do usuário. |
> | **Q2b** | **Assinatura mantida, em EN** | **U7 permanece**, reescrito: sem preset → `— Architect, Principal Software Architect`; com `--preset greek` → `— Zeus, …`. Exige o slug da identidade chegar ao corpo (hoje `Render()` já injeta a saudação — mesmo ponto de extensão). |
> | **Q3** | **Flat puro** — `docs/roadmaps/<estado>/` + `docs/req/` | **Validator, board e scaffold inalterados.** Migração fica no CMDB: subir `claude/*` um nível + realocar 45 legados. Descartar `kanban-flow` #8 (pastas por agente). |
> | **Q4** | **Manter exceção de trivialidade** (§7 + §11) | ⚠️ **Descartar** `kanban-flow` #21 ("o agente deve recusar mesmo se KG pedir"). Exige cláusula de precedência explícita §7/§11 > regra dura. Adotar a lista de **5** itens (§7), não a de 4. |
> | **Q5** | **Waves + MLs** como vocabulário único | Casa com a paralelização (O2): wave = MLs paralelos + barrier. **Adotar o frontmatter YAML** do `kanban-flow` (`req_id`, `status`, `responsavel`) para o gate funcionar. Descartar "FASES/MICROLOTES". |
> | **Q7** | **6 estados** — `analyzing` entra no validator | `backlog → analyzing → wip → blocked/done/abandoned`. **Custo: adicionar o estado ao validator nos 3 CLIs.** Ganho: `wip` passa a significar "codando com branch ativa", tornando `wip_limit` honesto. Alinha validator ↔ board ↔ scaffold ↔ codex. |
> | **Q9** | **Vault completo — scaffold + comando + gate** | `trackfw init` cria `vault/notes/index.md`; regra na camada universal; **`trackfw note new`** ×3 CLIs; regra **`note_orphan`** no validator. Converte a convenção em gate, coerente com o resto do produto. |
> | **Q14** | **`iac` como papel canônico novo** (Dédalo) | Custo: **30 entradas de preset** (10 universos × 3 CLIs) + `catalog.json` + `KnownAgentIDs` + tool set + fixtures. ⚠️ Exige **declarar a fronteira `infra` × `iac`** — hoje Ares e Dédalo se sobrepõem sem regra. |
> | **Q15** | **Cronos e Hermes ficam fora** | Verticais de domínio (ITIL/CMDB e NetSuite) não entram no produto. ⚠️ **Resíduo assumido:** `cronos.md` e `hermes.md` continuam locais — o objetivo "somente trackfw" fica em **11 de 13** papéis. Ver Parte 6. |
>
> **Decisões menores adotadas por default** (registradas; podem ser revertidas a qualquer momento):
> **Q6** branch = `feat|fix|refactor/<slug>` (o padrão data-`YYYY-MM-DD` do `git-rules` é legado do Copilot e **quebra** o `branch_has_wip_roadmap`) ·
> **Q10** Conventional Commits sem sufixo de agente e sem trailer de modelo hardcoded ·
> **Q11** `trackfw validate` **substitui** o `validate-kanban-gate.mjs` ·
> **Q12** quem cria branch é o **agente orquestrador** (`architect`) ·
> **Q13** artefatos sempre via **CLI** (`trackfw req|roadmap|adr new`), nunca à mão.

### 🔴 Q1 — Produto: default para todos ou perfil opt-in? **(decide todas as demais)**
O trackfw é open-source. Trazer seu harness significa qual das duas?
- **(a) Default** — todo usuário do trackfw recebe seu modelo (opinativo, PT-BR, kanban duro).
- **(b) Base neutra + perfil opt-in** (`--profile governed`).
> 💰 **Custo de (b):** `catalog.json` tem **um** `asset` por item e `Render()` não suporta corpo variável → **capacidade nova nos 3 CLIs** (catálogo, plano, render, testes). (a) é muito mais barato.

### 🔴 Q2 — Idioma dos assets
Panteão = 100% PT-BR com assinatura; assets = 100% EN.
- **(a) EN-only** — perde persona/assinatura, mantém alcance.
- **(b) PT-BR-only** — fecha o produto para não-lusófonos.
- **(c) i18n de assets** — escopo novo ×3 CLIs, sem mecanismo hoje.

### 🔴 Q3 — Layout de pastas do kanban
- `kanban-flow` #8: `docs/roadmaps/${AGENT_NAME}/{...}` e `docs/requisições/${AGENT_NAME}/`
- `CLAUDE.md §2`: `docs/roadmaps/claude/{...}` (hardcoded)
- **trackfw (validator)**: **flat** `docs/roadmaps/<estado>/` + `docs/req/`
#### 📊 Evidência empírica do CMDB (investigado em 2026-07-26)

Criações reais (`git log --diff-filter=A`), por janela:

| Janela | Total criado | Sob `claude/` | Sob agente nomeado | Flat na raiz |
|---|:-:|:-:|:-:|:-:|
| **Últimos 7 dias** | 63 | 52 | **0** | **11** |
| Últimos 14 dias | 117 | 104 | 2 | 11 |
| Últimos 30 dias | 287 | 274 | **2** (ambos `artemis/done` — movimentação, não criação) | 11 |

Acervo histórico total:

| Árvore | `claude/` | Agentes nomeados | Flat na raiz |
|---|:-:|:-:|:-:|
| `docs/requisições/` | **186** | 19 (afrodite 7, artemis 8, apolo 4) | 11 |
| `docs/roadmaps/` | **275** | 26 (artemis 10, apolo 8, ares 5, afrodite 2, atena 1) | 0 |

**Conclusão:** a dimensão por agente está **morta na prática**. Nos últimos 7 dias, **nenhum**
artefato foi criado sob agente nomeado. `claude/` virou balde único — ou seja, já é *flat com um
nível a mais que não carrega informação*. E 11 REQs já nasceram **flat na raiz** nos últimos 7 dias:
a migração para flat já está acontecendo organicamente.

> 💰 **Custo real de migrar para flat:** muito menor do que eu estimei. Não é reconciliar 6 árvores
> de agentes — é **subir `claude/*` um nível** e realocar 45 arquivos legados de agentes nomeados.
> Escolher **por agente** = mudar validator, board e scaffold nos 3 CLIs para sustentar uma
> dimensão que ninguém usa.
>
> ✅ **Recomendação: flat** — opcionalmente com `responsavel:` no frontmatter, se quiser preservar
> a rastreabilidade por dono sem pagar por diretórios.

### 🔴 Q4 — A exceção de trivialidade sobrevive?
- `kanban-flow` #21: *"mesmo que KG peça para implementar direto, o agente **deve recusar**"*
- `CLAUDE.md §7`: lista fechada de 5 dispensas · `kanban-flow` #23: lista fechada de **4** (sem "respostas a perguntas")
- `CLAUDE.md §11`: *"bug concreto → corrigir diretamente, NÃO iniciar ADR/roadmap"* — **sem cláusula de precedência**
> Hoje as três regras se contradizem. Qual prevalece?

### 🟠 Q5 — Vocabulário e template de roadmap
- `kanban-flow`: **FASES → MICROLOTES**, frontmatter YAML obrigatório (`req_id`, `microlotes`, `fases`, `status`), `Plano vigente` + `Log de execução` + `Validação Final`
- `CLAUDE.md §5` + `zeus.md`: **WAVES → MLs**, sem frontmatter, legenda ⬜/🔄/✅/❌
> Um roadmap no formato §5 **reprova** na DoD-2 do kanban-flow. Precisa de um vocabulário único.

### 🟠 Q6 — Nome de branch: três regras mutuamente excludentes
- `git-rules` #6: branch **tem que se chamar `YYYY-MM-DD`**, senão `exit 1`
- `CLAUDE.md §1`: `feat/…`, `fix/…`, `refactor/…`
- `git-ship`: só exige ≠ `main`
> ⚠️ Consequência atual: **toda** branch conforme o CLAUDE.md aborta o script do `git-rules`. E `branch_has_wip_roadmap` só dispara em `feat|fix|refactor` — exatamente as que o `git-rules` proíbe.

### 🟠 Q7 — Estado `analyzing`: 5 ou 6 estados?
- `kanban-flow` + skill `trackfw` + validator: **5**
- scaffold + CLAUDE.md gerado + board + codex: **6** (com `analyzing`)
> Inconsistência **interna do trackfw** — corrigir para um dos dois.
>
> ⚠️ **Alerta a confirmar antes da W1b:** `analyzing` tem **0 artefatos no CMDB em 30 dias** —
> exatamente o mesmo critério de vacuidade que derrubou a dimensão por agente na Q3. E é a única
> decisão que **obriga a mexer no validator dos 3 CLIs**. A decisão se sustenta **se o uso pretendido
> for prospectivo** (plan mode aterrissa em `analyzing`; ao começar a codar move para `wip`, tornando
> o `wip_limit` honesto) — e não uma tentativa de descrever um uso passado que não existiu.

### 🟠 Q8 — Autonomia do implementador
- `VIBE CODING AUTONOMOUS` (2/14): loop autônomo até verde, "não pergunte confirmações"
- `CLAUDE.md §6` Autopilot: perguntar tudo **antes**, não interromper depois
> São compatíveis, mas hoje coexistem com nomes/redações distintas. Unificar sob qual nome?

### 🟠 Q9 — Vault de conhecimento entra no trackfw?

#### 🔍 Investigação do vault real no CMDB (2026-07-26)

| Fato | Evidência |
|---|---|
| **209 notas** em `vault/notes/`, markdown puro | `find vault -name "*.md" \| wc -l` = 209 |
| Formato: frontmatter (`title`, `tags`, `date`, `related`) + wikilinks `[[...]]` + `index.md` como MOC | `vault/notes/index.md` + notas |
| Template de fato usado: **Problema → Causa Raiz → Solução Adotada** | `globalsearch-results-null-crash-2026-06-28.md` |
| **Nenhum plugin de vault instalado** | `installed_plugins.json` = apenas `gemini`, `frontend-design`, `codex`, `infracost` |
| `vault/skills/` = 4 skills **de formato Obsidian** (defuddle, json-canvas, obsidian-bases, obsidian-markdown), copiadas para dentro do projeto | `vault/skills/*/SKILL.md` |

**Conclusão: o vault NÃO depende de plugin.** As 4 skills em `vault/skills/` servem para *você*
ler/editar no Obsidian (canvas, bases, wikilinks) — os agentes só leem e escrevem markdown comum.
O que faz o vault funcionar são três coisas simples: **uma pasta, um template e a disciplina**
("consultar antes de investigar, escrever após causa-raiz não óbvia").

> 🔶 **Correção da minha recomendação anterior.** Eu havia sugerido "mapear para ADR" — está errado
> diante da evidência. Uma nota de vault documenta **causa-raiz de bug** (`results: null` em vez de `[]`),
> não uma decisão arquitetural com trade-off. Jogar isso em ADR polui a cadeia de governança e perde
> justamente o índice temático que dá valor. **São artefatos de naturezas diferentes.**

**Proposta:** trazer o vault como conceito próprio, **sem plugin**:
1. `trackfw init` cria `vault/notes/index.md`
2. Regra na **camada universal** dos assets: consultar o índice antes de investigar; escrever após constatação não óbvia (critério: "se outro agente perderia >10 min amanhã sem a nota, ela deve existir")
3. Opcional: `trackfw note new "<título>"` gerando o template e linkando no índice
4. Opcional: regra de validação `note_orphan` (nota não linkada no índice)
5. Fora de escopo: as skills de formato Obsidian (nicety do lado humano)

### 🟡 Q10 — Formato da mensagem de commit
- `git-rules`: `feat(scope): desc (NomeDoAgente)` — sufixo com nome do agente
- `git-ship`: Conventional + `Co-Authored-By: Claude Sonnet 4.6` (**modelo hardcoded, defasado**)
- `CLAUDE.md §2.1`: `fix(escopo): desc` — sem sufixo, sem trailer
- `CLAUDE.md §6`: exige registrar decisões autônomas na mensagem — nenhum formato prevê o espaço

### 🟡 Q11 — Dois gates de validação
`kanban-flow` DoD-5 exige `node scripts/validate-kanban-gate.mjs --strict` (bloqueante em CI e pre-push);
trackfw exige `trackfw validate`. **Nenhum documento declara precedência.**
> Presumo que `trackfw validate` **substitui** o `.mjs` — confirmar.

### 🟡 Q12 — Quem cria a branch: três donos
`git-rules` diz "KG cria"; `CLAUDE.md §1` diz "somente Claude orquestrador"; `zeus.md` diz "Zeus é a única entidade autorizada".
> Recomendação: **o agente orquestrador** (`architect`), consolidando O1.

### 🟡 Q13 — Criação de artefatos: arquivo manual ou CLI?
`kanban-flow` manda criar `.md` à mão pelo template; `CLAUDE.md §1` manda `trackfw req new` / `roadmap new`.
E a skill `trackfw/SKILL.md` **não lista** esses dois comandos.
> Recomendação: **CLI sempre** (garante `req_id`, frontmatter e pareamento) — e corrigir a skill.

---

## Parte 3 — Agentes extras a portar

> 💰 **Preço unitário:** 1 papel novo = 10 entradas de preset × 3 CLIs (**30**) + asset + `catalog.json` + `KnownAgentIDs` + tool set + fixtures. Os 4 juntos ≈ **120 entradas de preset**.
> Além disso, é preciso inventar nome para o papel em **10 universos** (grego, nórdico, potter, thrones, chaves, pioneers, starwars, tolkien, turma, egyptian).

| Agente | Papel | Genérico vs específico | Veredito sugerido |
|---|---|---|---|
| **Dédalo** | IaC (Terraform/OpenTofu multi-cloud + Ansible on-prem) | **~65% genérico.** Tabelas de provedores, protocolo de scaffolding e `Segurança por padrão` (7 regras: sem segredo inline, least privilege, Checkov/tfsec, aprovação humana p/ prod) são reaproveitáveis. Específico: contexto CMDB e a seção Temporal/ADR-014 | ✅ **Portar** — mas ⚠️ **sobrepõe `infra`** (Ares também declara Terraform). Papel novo ou especialização? → **Q14** |
| **Prometheus** | Tooling de agentes/IA (Copilot, MCP, prompts, design de agents) | **~55% genérico.** Mandato de checar docs oficiais e citar páginas, trade-offs speed/accuracy/cost, fluxo master. Específico: `AGENT_TEMPLATE.md` e "nomes de deuses gregos — convenção obrigatória" | ✅ **Portar** — é o agente que **define o formato dos outros**. Cabe bem num produto cujo negócio é governar agentes |
| **Cronos** | CMDB Business Analyst (ITIL, qualidade de dados de CI) | **~90% específico.** Removido o CMDB/ITIL, sobram ~6 linhas | ❌ **Não portar como papel canônico.** O que é reaproveitável é o **arquétipo** "analista read-only que audita e faz handoff" |
| **Hermes** | NetSuite | **~70% específico de plataforma** — mas **não** do CMDB do KG | ❌ **Não portar como canônico** — é vertical de produto SaaS. Candidato a "pacote de domínio" opcional |

### ⚠️ Q14 — Dédalo: papel novo ou especialização de `infra`?
Ares e Dédalo hoje se sobrepõem em Terraform **sem fronteira declarada** em nenhum dos dois arquivos.

### ⚠️ Q15 — Verticais de domínio (Cronos/Hermes)
Se não entram como canônicos, entram como quê?
- (a) fora do trackfw (ficam locais no KG)
- (b) "pacote de domínio" opcional — exige a mesma capacidade de perfil da **Q1(b)**
- (c) entram como canônicos mesmo sendo verticais

---

## Parte 4 — Defeitos a corrigir no caminho (independem de decisão)

Achados nos inventários, todos verificados:

1. `docs/roadmaps/zeus/` (plural) vs `docs/roadmap/<agente>/` (singular) — **um dos dois está errado**
2. `SKILL_GIT_RULES.md` é citado por **2 agentes** e **não** por Zeus, que é quem commita
3. `dedalo` cita REQ mas **não** ROADMAP — quebra o par
4. 5 agentes declaram `Assine:` e **não têm assinatura no rodapé**
5. `metis.description` tem **"Senior Specialist Senior Specialist"** duplicado
6. `req_id` tem **dois formatos** dentro do próprio `kanban-flow` (com e sem `_HH-MM`) — quebra o pareamento
7. `CLAUDE.md §0` manda invocar a skill **`zeus`, que não existe** (o nome é `arquiteto`)
8. `git-rules` proíbe "mencionar criação de branch" e **na regra seguinte imprime** "KG, crie branch…"
9. `git-ship` tem `Co-Authored-By: Claude Sonnet 4.6` **hardcoded e defasado**
10. Regras de projeto vivendo em arquivos de agente (ArangoDB, Entra ID, `API_SPECIFICATION.md`) — deveriam estar no CLAUDE.md do projeto
11. `analyzing` reconhecido pelo board e não pelo validator (**Q7**)

---

## Parte 5 — Sequência de execução (decisões fechadas)

| Onda | Conteúdo | Paralelizável? |
|---|---|---|
| **W0** | **ADR de convergência** consolidando Q1–Q15 + REQ + roadmap | sequencial — pré-requisito de tudo |
| **W1** | Camada universal (U1–U7) nos 10 assets, em EN + novo mapeamento `tools:` (ARCH/IMPL/REVIEW para Claude Code) | ✅ paralelo com W1b |
| **W1b** | `analyzing` no validator (Q7) + regra `note_orphan` + `trackfw note new` (Q9) — 3 CLIs | ✅ paralelo com W1 |
| **W2** | Adendo orquestrador (O1–O6) + adendo implementador (I1–I5) nos assets | depende de W1 |
| **W3** | CLAUDE.md gerado: C1–C6, C8–C11 + formato Waves/MLs (C12/Q5) + cláusula de precedência da exceção (Q4) | depende de W1b |
| **W4** | Papel `iac` (Q14): 30 entradas de preset + asset + catálogo + fronteira `infra`×`iac` | ✅ paralelo com W3 |
| **W5** | Papel `prometheus`/tooling de agentes: mais 30 entradas de preset | ✅ paralelo com W3 |
| **W4b** | **Skills técnicas por papel (Q16)** — 10 arquivos agnósticos de stack em `assets/skills/` | ✅ paralelo com W3/W4/W5 |
| **W6** | Correções da Parte 4 (defeitos) + Q6/Q10–Q13 | depende de W3 |
| **W7** | Aposentar `generators/agents.go` preservando `legacyHashes` | ortogonal — a qualquer momento |
| **W8** | **`trackfw ship` agnóstico de forge (Q17)** — campo `forge:` no config + detecção no `discover` + pergunta no `init` + comando × 3 CLIs | ortogonal — pode correr desde o início |

> **Paralelismo real:** W1‖W1b → W2 → (W3 ‖ W4 ‖ W4b ‖ W5) → W6. W7 e W8 a qualquer momento.
> Tempo de parede ≈ onda mais lenta, não a soma.
>
> 📌 **W8 é a candidata natural a REQ separada** — não depende de nenhuma decisão de harness
> (Q1–Q16) e toca uma área distinta do código (`config`, `discover`, comando novo). Separá-la
> mantém o roadmap da convergência focado e permite entregá-la antes ou depois, sem barrier.

---

## Parte 6 — Resíduo assumido: o que sobra fora do trackfw

Com **Q15 = ficam fora**, o objetivo "usar somente o trackfw" fica em **11 de 13 papéis**.
Permanecem locais em `~/.claude/agents/`:

| Arquivo | Motivo | Consequência |
|---|---|---|
| `cronos.md` | ~90% CMDB/ITIL — shipar para todo usuário do trackfw seria ruído | Você mantém 1 agente local |
| `hermes.md` | ~70% NetSuite — vertical de produto SaaS | Você mantém 1 agente local |
| `skills/cmdb-expert` · `cmdb-terminology` · `cmdb-business-rules` · `netsuite` · `copilot-expert` | Verticais de domínio (650 linhas) | Coerente com Q15 |
| **~50–60% do conteúdo das 11 skills técnicas** | Específico da sua stack (Q16) | ➡️ **não fica órfão** — migra para o `CLAUDE.md` do CMDB |

⚠️ **Atenção:** esses 2 arquivos ficam **órfãos de harness** — hoje o `hermes.md` já é um implementador
com `Edit/Write` e **zero regras de git e de kanban**, e o `cronos.md` não tem nenhuma. Ao aposentar o
`~/.claude/CLAUDE.md`, eles perdem também a cobertura que vinha de lá.

**Mitigação sugerida:** após W2, reaplicar manualmente neles a camada universal + o adendo
implementador que forem gerados para os assets do trackfw — ou reavaliar a Q15 com a opção
"analyst canônico + NetSuite fora", que reduziria o resíduo a 1 arquivo.

---

## Parte 7 — 🔴 LACUNA ABERTA: as 21 skills pessoais

Q1–Q15 cobriram **agents** e **CLAUDE.md**. Mas o objetivo é aposentar também
`~/.claude/skills/*` — **2.518 linhas em 21 skills** — e **nada no plano atual as absorve**.

### O que já está resolvido

| Skill pessoal | Linhas | Destino no plano |
|---|:-:|---|
| `kanban-flow` | 355 | ✅ Absorvida (Q3, Q5, C3–C6) |
| `git-rules` | 76 | ✅ Absorvida (Q6, Q10, Q12) |
| `arquiteto` | 60 | ✅ Vira o asset `architect` (é só expertise; o harness do Zeus já está em O1–O6) |
| `trackfw` | 24 | ✅ Redundante com o produto |

### O que **não** tem destino

| Grupo | Skills | Linhas | Problema |
|---|---|:-:|---|
| **Expertise técnica** | backend 73 · frontend 93 · dba 152 · infra 111 · security 76 · data 65 · qa 45 · ux-ui 53 · code-quality 78 · **iac 250** · **ai-integration 235** | **~1.231** | As 5 skills do trackfw (`governance`, `plan`, `implement`, `review`, `release`) são todas de **processo**, ~310 bytes cada. **Nenhuma carrega expertise técnica.** |
| **Fluxo git** | `git-ship` | 122 | O **único fluxo git internamente coerente** do seu harness (valida branch, detecta squash-merge, commit → push → PR). Candidato natural a **`trackfw ship`**. Hoje sem destino. |
| **Verticais** | cmdb-expert 254 · cmdb-terminology 301 · cmdb-business-rules 18 · netsuite 47 · copilot-expert 30 | 650 | Coerente com Q15 (ficam fora) |

### ✅ Q16 — RESOLVIDA: família nova de skills por papel, agnóstica de stack

**Estrutura:** as skills técnicas entram como **arquivos próprios**, um por papel, ao lado das 5 de
processo. **Não** são apensadas às existentes.

> **Razão:** as 5 skills atuais são organizadas por **verbo/fase** (`plan`, `implement`, `review`,
> `release`, `governance`) e são transversais. As técnicas são por **domínio/papel**. São eixos
> diferentes — fundir faria `implement` conter Go + React + ArangoDB + Kafka + Playwright ao mesmo tempo.

**Por que skill e não corpo do agente:**
1. **Contexto** — o corpo do agente carrega sempre; a skill carrega sob demanda (250 linhas de IaC não devem custar tokens num `terraform fmt`)
2. **Reuso cross-agent** — o próprio Hefesto diz *"pode carregar skills de outros agentes apenas para entender melhor o problema"*; isso só funciona com skill
3. **Thread principal** — o orquestrador consulta expertise sem precisar spawnar o especialista

**Curadoria: agnóstica de stack.** Atravessa o que vale em qualquer projeto — RFC 7807, SOLID,
12-Factor, WCAG 2.2 AA, web-first assertions, planos de execução e índices, least privilege,
idempotência, zero in-memory como fonte de verdade. Estimativa: **40–50% das ~1.231 linhas**.

**O específico de stack sai das skills** e vai para o CLAUDE.md do projeto — que é exatamente onde o
**defeito #10** da Parte 4 já dizia que deveria estar (`ArangoDB`, `Entra ID`, `API_SPECIFICATION.md`,
`Uber Fx`, `Module Federation`).

```
skills/backend.md   (trackfw, todos)   │  CLAUDE.md do CMDB   (só KG)
· erros RFC 7807                       │  · ArangoDB via driver.Database
· idempotência, wrap com contexto      │  · Uber Fx · Gin
· zero in-memory como fonte de verdade │  · Entra ID + storageState
```

**Camadas finais da expertise:**

| Camada | Carrega | Muda | Consumido por |
|---|---|---|---|
| Asset do agente | Fronteira, proibições, workflow, handoff | Raro | Só aquele agente, sempre |
| **Skill por papel** (novo) | Princípios agnósticos de stack | Médio | **Qualquer** agente, sob demanda |
| CLAUDE.md gerado | Comandos da stack detectada (`go`/`java`/`node`/`python`, `react`/`vue`/`angular`) | Por projeto | Todos, no projeto |
| CLAUDE.md do projeto | Escolhas de produto (ArangoDB, Entra ID) | Por projeto | Só o CMDB |

> 💰 **Custo mecânico baixo:** `sync-integration-assets.sh` já propaga skills para npm/pypi e
> `check-integration-assets.sh` valida byte-identity em `make quality`. São **10 arquivos numa árvore só**.
> O custo real é **curadoria**, não mecânica.

### ✅ Q17 — RESOLVIDA: `git-ship` vira `trackfw ship`, **agnóstico de forge**

Requisito do KG: a abertura de PR/MR **não pode ser amarrada ao GitHub**. Deve seguir o flavor
escolhido no `init` ou descoberto pelo `discover`.

#### Resolução do flavor (ordem de precedência)

| # | Fonte | Observação |
|:-:|---|---|
| 1 | Flag explícita `--forge github\|gitlab\|bitbucket\|azure` | Override pontual |
| 2 | Campo **`forge:`** no `trackfw.yaml` | Novo campo em `config.ProjectConfig`; definido no `init` e pelo `discover` |
| 3 | **`git remote get-url origin`** → parse do host | **Fonte mais autoritativa** — `github.com`, `gitlab.*`, `bitbucket.org`, `dev.azure.com` |
| 4 | CI já detectado pelo `discover` | `.github/workflows` → github · `.gitlab-ci.yml` → gitlab. **Proxy, não prova** |
| 5 | Nada resolvido | **Modo manual**: imprime a URL de criação e não falha |

> ⚠️ **Armadilha do self-hosted:** GitLab/Bitbucket on-premise têm host arbitrário
> (`git.empresa.com.br`) — o passo 3 sozinho **não** identifica. Por isso o passo 2 (`forge:` explícito)
> existe e o passo 4 (presença de `.gitlab-ci.yml`) serve de desempate.

#### Mapeamento por flavor

| Forge | CLI | Comando | Nomenclatura na saída |
|---|---|---|---|
| `github` | `gh` | `gh pr create` | **Pull Request** |
| `gitlab` | `glab` | `glab mr create` | **Merge Request** |
| `azure` | `az` | `az repos pr create` | Pull Request |
| `bitbucket` | — (sem CLI oficial estável) | fallback para URL | Pull Request |

> O comando deve **falar o substantivo certo** — "MR" no GitLab, "PR" nos demais.

#### Degradação graciosa (obrigatória)
Se o CLI do forge não estiver instalado, **não falhar**: fazer o push e imprimir a URL de criação
(todas as forges têm padrão de URL de compare). O `discover` já tem o helper `externalCommandAvailable`
(via `exec.LookPath`) — reaproveitar.

#### Passos do `trackfw ship` (do `git-ship`, já alinhados às decisões)
1. Valida branch ≠ `main`/`master` **e** conforme Q6 (`feat|fix|refactor/<slug>`)
2. Valida **REQ + roadmap em `wip`** (integra `trackfw validate`) — amarra o ship à governança
3. Detecta squash-merges pendentes (protocolo de 3 passos + 3-bis do `CLAUDE.md §1`)
4. Revisa staged — nunca `git add .`
5. Commit Conventional Commits — **sem** sufixo de agente e **sem** trailer de modelo hardcoded (Q10)
6. `git push origin <branch>`
7. Abre PR/MR conforme o flavor, com corpo referenciando REQ + roadmap + critérios de aceite

> 💰 **Custo:** comando novo × 3 CLIs + campo `forge:` no `config.ProjectConfig` + pergunta no wizard
> do `init` + detecção no `discover`. É a onda mais pesada depois da W4/W5.

---

## Parte 8 — Notas de implementação a resolver antes das ondas

1. **`note_orphan` deve nascer como `warning`, não `error`.** O validator já suporta severidade
   configurável (`ruleSeverity`, fallback `error`; `adr_orphan` já é warning por default). Uma regra
   bloqueante fecharia `trackfw validate` no CMDB no dia 1, com **209 notas pré-existentes**.
2. **Q2b exige código novo, não ajuste.** Verificado em `render.go`: só existe **prefixo**
   (`greeting + "\n\n" + body` e `insertBodyPrefix`). **Não há mecanismo que anexe ao fim do corpo** —
   a assinatura em rodapé precisa de uma função nova.
3. **`iac` e o papel de tooling precisam de nome em 10 universos.** Hoje só existe "Dédalo" (grego) e
   "Prometheus" (grego). `TestPreset_EveryPresetCoversExactlyKnownAgentIDs` **falha** até que os 10
   presets tenham o novo id — são 10 nomes a inventar por papel, × 3 CLIs.
