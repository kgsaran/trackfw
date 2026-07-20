# Evolução do trackfw — ser "genérico" sem perder os dentes (baseline + catraca)

> **Origem:** discussão de campo após a v2.3.0, no contexto do CMDB (projeto legado real).
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Destinatário:** agente/mantenedor do trackfw · **Alvo sugerido:** v2.4.
> **Pré-requisito de leitura:** [`analise-comparativa-gates-governanca.md`](./analise-comparativa-gates-governanca.md)
> (Achado #4 — conflito de convenção — e a ressalva sobre `lenient` global).

---

## 1. Contexto e tese

Rodar a v2.3.0 no CMDB (monorepo legado) produziu **73 avisos** em modo `lenient`, dos quais a maioria
**não é dívida real**, e sim:

- **Divergência de convenção** (7 "wip sem REQ" porque o CMDB usa `req_id:` no frontmatter, não a linha
  inline `REQ:` que o trackfw casa por substring).
- **Passivo histórico** (59 "ADR órfão" — ADRs que precedem a adoção do fluxo REQ→ADR).

A tese desta nota: **o trackfw deve ser mais genérico** para conviver com projetos legados — **mas
"genérico" tem duas leituras, e só uma é saudável.** Confundi-las leva a um gate desdentado.

---

## 2. Separe dois eixos de "genérico"

### Eixo A — Gramática / convenção → **SEJA genérico** ✅

Impor uma única sintaxe de documento mata adoção. O trackfw casa marcadores textuais fixos
(`REQ:`, `ADR:`, `Roadmap:`, `## Acceptance Criteria` / `## Critérios de Aceite`, `Status: Open`,
`## Blocked by ADRs`). Projetos reais têm convenções próprias e versionadas (no CMDB, o ADR-036 manda
usar **frontmatter** `req_id:`/`roadmap:`/`status:`).

> Aqui, "genérico" = **adaptar-se à convenção do projeto** (field mapping, marcadores configuráveis,
> i18n de seções), não obrigar o projeto a reescrever centenas de documentos.

### Eixo B — Rigor de enforcement → **NÃO afrouxe** ⚠️

O risco de "ser mais genérico" é virar **toothless**: se tudo é opcional e tudo passa, o gate não valida
nada e perde a razão de existir. **Legado não deve ser resolvido baixando a régua global.**

---

## 3. O legado se resolve com *baseline + catraca*, não com leniência global

O `lenient`/`lenient_until` atual é um **interruptor binário e global**: ou tudo é bloqueante, ou nada é.
Isso não distingue **passivo histórico** (aceitável, congelar) de **trabalho novo** (deve seguir o padrão).

O padrão correto — já provado pelo gate interno do CMDB (`scripts/validate-kanban-gate.mjs`, ADR-036) e
pela indústria — é **grandfather por baseline com catraca (ratchet)**:

- **Congela o passivo:** artefatos anteriores a um corte (data ou snapshot) ficam **isentos**.
- **Cobra o novo integralmente:** a partir do corte, o padrão completo vale.

> Você nunca *floda* 59 avisos de ADR órfão legado; você impede **o 60º**.

### Como o gate interno do CMDB já faz (referência de implementação)

- Corte por data (`ADR036_DATE`, configurável via env): artefatos com data detectável `<` corte são
  *grandfathered*. A data é extraída em ordem: `Criado em:` no corpo → `date:` no frontmatter → sufixo
  `-YYYY-MM-DD` no nome do arquivo.
- Isenção por estado terminal: `done`/`abandoned` são registro histórico e não são cobrados por regras de
  processo (refs quebradas, pareamento), mas **continuam** sendo cobrados pela regra que é a razão de
  existir daquele estado (ex.: evidência em `done`).
- Resultado: o passivo pré-ADR não gera ruído; o trabalho novo é disciplinado desde a criação.

### Análogos de mercado (o mesmo princípio)

- **SonarQube** — foco em *new code* (Clean as You Code).
- **golangci-lint** — `--new-from-rev` / `--new-from-patch` (só o diff novo).
- **ESLint** — *suppressions baseline* / `--quiet`; ferramentas como `eslint-nibble`.
- **Git/blame-based gates** — "fail only on lines introduced após o baseline".

---

## 4. Recomendações para o trackfw (em ordem de impacto)

### R1 — Field mapping no `trackfw.yaml` (resolve o falso positivo de convenção)

Permitir mapear *como* o projeto expressa cada vínculo, satisfazendo a regra sem reescrever docs:

```yaml
# exemplo ilustrativo
link_fields:
  req:     [req_id, "REQ:"]        # aceita frontmatter req_id OU linha inline REQ:
  adr:     [adr, "ADR:"]
  roadmap: [roadmap, "Roadmap:"]
acceptance_markers:
  - "## Acceptance Criteria"
  - "## Critérios de Aceite"
```

Hoje, `validateWIPHasREQ` faz `strings.Contains(content, "REQ:")`. Com mapping, um roadmap que declara
`req_id:` no frontmatter satisfaz "tem REQ" — elimina os 7 falsos positivos do CMDB.

### R2 — Grandfather por baseline (transforma "genérico" em "adotável")

Duas formas, idealmente ambas:

- **Por data:** `baseline_date: 2026-06-13` no `trackfw.yaml` — artefatos com data detectável anterior
  ficam isentos das regras de processo (mantendo as estruturais críticas, se desejado).
- **Por snapshot:** comando `trackfw baseline` que grava o passivo atual (ex.: `.trackfw-baseline`),
  e `validate` só cobra **violações novas** em relação ao snapshot (modelo ratchet/diff).

Distinguir **isenção explícita** (baseline) de **tolerância silenciosa** (`lenient`) é o ponto central:
baseline **documenta** o passivo; leniência apenas **esconde** tudo.

### R3 — Severidade configurável por regra (adoção progressiva)

```yaml
rules:
  adr_orphan:           warning   # off | warning | error
  wip_has_req:          error
  wip_acceptance:       warning
  folder_status:        error
```

Permite ao time ligar o rigor regra a regra conforme amadurece, em vez do liga/desliga global.

---

## 5. Síntese

| Leitura de "genérico" | Veredito | Mecanismo |
|---|---|---|
| Adaptar-se à **convenção** do projeto | ✅ fazer | field mapping (R1), marcadores configuráveis, i18n |
| Tratar **legado** sem ruído | ✅ fazer | **baseline + catraca** (R2), não `lenient` global |
| Afrouxar enforcement para "passar" | ⚠️ evitar | severidade por regra (R3) p/ adoção progressiva, nunca diluição cega |

> "Genérico" deve significar **adaptável e calibrável**, não **permissivo**. A diferença em projeto
> legado deve ser **isentada como baseline explícito**, não tolerada em silêncio — senão troca-se
> ruído por cegueira.

Estas três mudanças (field mapping, baseline/ratchet, severidade por regra) deixariam o trackfw tão
adotável quanto necessário em legado **e** tão rigoroso quanto o gate interno do CMDB no trabalho novo —
abrindo caminho para o trackfw **substituir** o gate `.mjs` em vez de coexistir.
