# Comparativo — Agents do trackfw × Panteão Grego implantado

> Análise doc-only (sem mudanças de código). Data: 2026-07-26. Autor: Zeus.
> Objetivo: expor diferenças de harness, fluxo git e coordenação para decisão do KG.

## 1. Os três conjuntos existentes

| Conjunto | Local | Nº | Data | Status |
|---|---|---|---|---|
| **A — Panteão grego** | `~/.claude/agents/*.md` (hand-authored) | 14 | 02-jul / 25-jul | Ambiente pessoal do KG |
| **B — trackfw assets (live)** | `internal/integrations/assets/agents/` + `npm/src/...` + `pypi/trackfw/...` | 10 | 25-jul | **Enviado ao usuário final** |
| **C — trackfw templates (legado)** | `internal/generators/templates/agents/` | 10 | 11-jun | **Código morto** (ver §5) |

O que está instalado hoje no ambiente do KG como `trackfw-*.md` é o **conjunto B** renderizado com o preset **norse** (`odin-tf`, `thor-tf`, …), não o conjunto C.

> **Por que a lista de agents mostra `odin-tf` e o arquivo se chama `trackfw-architect.md`:** o `render.go` reescreve apenas o campo `name:` do frontmatter, não o nome do arquivo — e o Claude Code seleciona subagentes pelo frontmatter, nunca pelo filename.

## 2. Matriz de diferenças por eixo

| Eixo | A — Panteão grego | B — trackfw assets (live) | C — templates legado |
|---|---|---|---|
| **Idioma** | 100% PT-BR | 100% inglês, neutro | PT-BR |
| **Tamanho médio** | 2,2–6,7 KB | **~360 bytes** (1 parágrafo) | 2,1–6,0 KB |
| **LOCK DE MODO** | ✅ em 14/14 | ❌ | ✅ |
| **Assinatura obrigatória** | ✅ (emoji + papel) | ❌ | ✅ |
| **frontmatter `tools:`** | ✅ explícito | ❌ ausente (herda tudo) | ✅ explícito |
| **frontmatter `model:`** | `claude-sonnet-4-6` / opus | `sonnet` / `opus` (genérico) | idem C |
| **frontmatter `memory: project`** | ✅ 14/14 | ❌ | ✅ |
| **Fluxo git** | ⚠️ **inconsistente** — só `zeus` (permissões git de orquestrador) e `afrodite`/`artemis` (GIT_FLOW: proíbe criar branch/PR) | ❌ nada | ✅ igual a A (Architect com permissões git) |
| **Pré-requisito KANBAN (REQ/ROADMAP em wip)** | ⚠️ **só 2/14** — `afrodite` e `artemis` | ❌ nada além de "preserve trackfw traceability" | ❌ |
| **Coordenação (waves/barrier/spawn paralelo)** | ✅ só em `zeus` (seção completa) | ❌ apenas "delegate to the appropriate specialist" | ✅ igual a zeus |
| **Vault de conhecimento** | ⚠️ 3/14 (`afrodite`, `artemis`, `hermes`) | ❌ | ❌ |
| **`agents-working-context.md`** | ✅ 14/14 | ❌ | ✅ |
| **Acoplamento a projeto** | Alto (ArangoDB, Entra ID, CMDB, ADRs específicos) | Zero (project-agnostic) | Médio (herda A, despersonificado) |

### Achado que reverte a intuição
O harness do panteão **não é uniforme**. `LOCK`, `memory: project`, `tools:` e registro de contexto são universais (14/14), mas **KANBAN, GIT_FLOW e Vault existem em apenas 2–3 agentes**. Ou seja: não existe "o harness do panteão" pronto para portar — ele teria que ser **normalizado antes**.

## 3. Mapeamento de papéis

| Papel canônico trackfw | Panteão | Observação |
|---|---|---|
| architect | Zeus | |
| backend | Apolo | |
| frontend | Afrodite | |
| qa | Ártemis | |
| infra | Ares | |
| security | Hades | |
| dba | Poseidon | |
| ux | Atena | arquivo local é `athena.md`; preset trackfw usa slug `atena` |
| code-quality | Hefesto | arquivo local é `hephaestus.md`; preset usa `hefesto` |
| data | Métis | |
| — | **Cronos** (CMDB BA) | sem contrapartida no trackfw |
| — | **Dédalo** (IaC/Terraform) | sem contrapartida |
| — | **Hermes** (NetSuite) | sem contrapartida |
| — | **Prometeu** (Copilot/agents) | sem contrapartida |

**Sem risco de colisão:** `identity.AgentName()` sempre sufixa `-tf` (`internal/identity/identity.go:119`). Instalar o preset **greek** geraria `zeus-tf`, `apolo-tf` — convivendo com os `zeus.md`/`apolo.md` autorais. A decisão **não é "um ou outro"**.

## 4. A lacuna que importa para o produto

O bloco KANBAN do `artemis.md` ("pare se estiver implementando sem REQ/ROADMAP em `wip`") é **exatamente** o que a validação `branch_has_wip_roadmap` do trackfw impõe (v2.7.0+). Os assets enviados aos usuários **não mencionam isso** — dizem apenas "preserve trackfw traceability".

Um usuário que instala os agents do trackfw recebe personas genéricas que **não sabem operar a cadeia ADR→REQ→ROADMAP** que é a razão de existir do produto. Isso é uma lacuna de produto, não de estilo.

## 5. Achado ortogonal: conjunto C é código morto — **mas os bytes são load-bearing**

- `generators.InstallAgents()` tem **zero chamadores de produção** — só testes (`internal/generators/agents_test.go`).
- **Já está declarado como exceção** em `docs/cli-parity.md:58` ("has no production caller … dead code kept as is, not part of the contract") — portanto **não é violação de paridade**, apenas código morto (~33 KB).
- ⚠️ **Porém:** o SHA-256 de `trackfw-architect.md` (`d28ae507…`) e de `trackfw-qa.md` (`384283eb…`) **batem exatamente** com as entradas `claude\0cli\0global\0agents\0…` de `internal/integrations/legacy.go`. Esses templates são os artefatos históricos que o `update`/`adopt` reconhece para **adoção segura** de instalações antigas.
- Os hashes estão hardcoded no `legacy.go`, então apagar os arquivos **não quebra** a adoção — mas remove a proveniência verificável desses hashes.

**Paridade dos assets vivos (B): OK** — md5 idêntico nos 3 CLIs para os 10 arquivos.

## 6. Opções para decisão

### Opção 1 — Status quo (assets finos e neutros)
Manter B como está. Personas mínimas, o comportamento de governança vem do `CLAUDE.md` do projeto do usuário.
**Custo:** zero. **Risco:** produto não ensina o próprio fluxo.

### Opção 2 — Enriquecer os assets (project-agnostic)
Adicionar aos 10 assets: bloco de pré-requisito KANBAN referenciando comandos reais do trackfw (`trackfw req new`, `roadmap move ... wip`, `validate`), regra git (só architect cria branch), seção de waves/barrier no architect.
**Custo de paridade:** 10 arquivos × 3 CLIs = 30 arquivos + revisar `render_test.go`, que garante saída *byte-for-byte* idêntica quando não há identidade — os testes de fixture quebram e precisam ser regravados. Manter neutralidade de stack (sem ArangoDB/Entra ID/CMDB).
**Barato de graça:** declarar `tools:` já é quase gratuito — o `render.go` (branch `agent-directory`) **já tem** o mapeamento canônico `agentTools(item.ID)` com os conjuntos `SET_IMPL`/`SET_ARCH`; ele simplesmente não é emitido na representação `subagent` que o Claude Code consome.

### Opção 3 — Duas camadas: base + "harness pack" opt-in
Manter B como base fina e adicionar um pacote opcional (`trackfw agents install --harness governance`) com os blocos KANBAN/git/waves.
**Custo:** maior (novo conceito no catálogo, flag nos 3 CLIs, ADR), mas não quebra quem já instalou.

### Opção 4 — Aposentar o gerador legado, **preservando a detecção legada** *(ortogonal)*
Remover `generators/agents.go` + `templates/agents/` + testes, **mantendo intacto** o `legacyHashes` de `internal/integrations/legacy.go` (a adoção segura depende dele). Atualizar `docs/cli-parity.md:58` e documentar no `legacy.go` que os hashes correspondem a templates removidos (proveniência via git history / tag).
**Custo:** baixo–médio, 1 ML — porém **não é "isolada"**: exige nota de proveniência para não deixar hashes órfãos e sem rastreabilidade.

---

**Sequência sugerida, se houver decisão de mudar:** Opção 4 primeiro (limpeza contida), depois escolher entre 1/2/3 para os assets vivos. Sem veredito — a escolha é do KG.
