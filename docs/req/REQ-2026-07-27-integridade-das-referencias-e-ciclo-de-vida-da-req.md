---
status: Done
date: 2026-07-27
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md"
---

# REQ: integridade das referencias e ciclo de vida da REQ

> Date: 2026-07-27 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

Dois defeitos que são a mesma coisa vista de ângulos diferentes: **o estado real da governança
diverge do que o validator enxerga, e nada acusa.**

### Defeito 1 — 79% das REQs têm referência inválida, com `validate` verde

**38 de 48 REQs** declaram no frontmatter `roadmap:` um caminho que não existe no filesystem. Medido
com os artefatos já migrados (PR #77):

| Formato encontrado | Qtd |
|---|---:|
| basename puro, sem diretório | 32 |
| caminho legado `claude/` | 2 |
| aponta `wip/`, arquivo está em `done/` | 2 |
| sem extensão `.md` | 1 |
| outro caminho inválido | 1 |
| **caminho válido** | **9** |

**37 das 38 apontam para um arquivo que existe** — o problema não é rastreabilidade perdida, é
**ausência de formato canônico**. O template grava `roadmap: ""` e nunca se definiu como preencher,
então cada agente inventou um formato.

O `validate` fica verde por **três escapes independentes**, cada um suficiente sozinho:

1. **O frontmatter nunca é lido.** O extrator procura o marcador `Roadmap:` (maiúsculo) no *corpo*;
   o frontmatter grava `roadmap:` (minúsculo, aspeado). Nenhuma regra do validator lê o campo do
   frontmatter — quem lê é só o `serve` (`internal/serve/api_chain.go:109-115`), que o usa como **ID
   de nó do grafo** e gera aresta órfã quando o valor não corresponde a um nó real.
2. **Fallback permissivo por basename.** `referenceExists`
   (`internal/validator/validator.go:1356-1377`, `npm/src/validator/index.js:715-724`,
   `pypi/trackfw/validator.py:864-872`) tenta o caminho; se falhar, faz **walk recursivo** procurando
   só o basename. Como o arquivo existe em `done/`, o caminho errado em `wip/` valida. É um permissivo
   disfarçado de validação.
3. **Severidade `warning`** (`internal/config/config.go:88` e equivalentes) — não reprova nem se 1 e 2
   forem corrigidos.

`req_has_roadmap` também não ajuda: verifica apenas se a **substring** `Roadmap:` existe no arquivo
(`validator.go:889-904`). O template sempre a insere → a regra é estruturalmente inviolável.

### Defeito 2 — nada fecha a REQ, e isso produz falso positivo em regra `error`

Não existe `req move`, `req close` nem qualquer lógica que atualize o status da REQ em nenhum dos 3
CLIs. Hoje há **6 REQs com `Status: Open`** cujo roadmap está em `done/`.

Consequências verificadas:

- **`blocked_by_draft_adr` é `error`** (`internal/config/config.go:91`). Uma REQ entregue mas ainda
  `Open` continua elegível: se qualquer ADR da sua seção `## Blocked by ADRs` for rebaixado a `Draft`,
  **o gate reprova trabalho que já terminou**. O inverso é pior — uma REQ marcada `Done` à mão sem
  estar é *excluída* do check (`if !strings.Contains(s, "Status: Open") { continue }`), um falso
  negativo silencioso.
- **`sync`** (`internal/sync/sync.go:90-101`) usa `isStatusOpen` para decidir o que empurrar para
  Linear/Jira. REQs concluídas são sincronizadas indefinidamente.
- **`serve`** sobrescreve o estado inferido do path com o `status` do frontmatter
  (`api_chain.go:105-107`): o board mostra a cadeia perpetuamente aberta.

Agravante de design: a REQ tem **dois** lugares de status — frontmatter `status:` e header
`> Date: … | Status: …` — e nada garante que concordem. É o mesmo padrão corrigido no roadmap pela
REQ-2026-07-27-roadmap-move, e a solução deve espelhar aquela.

### Por que os dois juntos

Fechar a REQ automaticamente exige saber **qual roadmap ela referencia** — que é exatamente o link
quebrado do Defeito 1. Separá-los faria a REQ do ciclo de vida depender de um contrato inexistente.

Ambos são **P2 — degradação silenciosa** do
`ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`, e sobreviveram pelo mesmo motivo dos
últimos três ciclos: a verificação existente não exercita o caso real.

## Acceptance Criteria

- [ ] Formato canônico do campo `roadmap:`/`adr:` do frontmatter **definido e documentado**
- [ ] `req new` e `roadmap new --from-req` preenchem o campo no formato canônico
- [ ] Regra que valida o link do **frontmatter** por caminho, nos 3 CLIs
- [ ] Fallback permissivo por basename removido ou tornado explícito e opt-in
- [ ] Severidade de `ref_targets_exist` elevada para `error` (ou justificativa registrada se mantida)
- [ ] As 38 referências inválidas normalizadas para o formato canônico
- [ ] Comando que fecha a REQ sincronizando **frontmatter e header**, nos 3 CLIs
- [ ] Teste negativo que prova a reprovação de cada caminho corrigido (P4)
- [ ] Higiene: `trackfw log` grava no mesmo arquivo nos 3 CLIs · `forge`/`trace_id_field` com strip de
      aspas no Go · `blocked` namespace-aware
- [ ] `make quality` verde, sem variável de ambiente auxiliar

## Escopo negativo — registrado, não corrigido

1. **Slash-command `/trackfw:roadmap` gera roadmap SEM frontmatter**, idêntico nos 3 `init`
   (`internal/generators/scaffold.go:278`, `npm/src/generators/init.js:790`,
   `pypi/trackfw/generators/init_gen.py:507`). Terceiro formato concorrente; mexer nele muda o que todo
   projeto instalado recebe. **Candidato ao próximo ciclo.**
2. **`stale_wip` é inócuo**: `staleWIPDays = 7` hardcoded e a regra usa **mtime** (com preferência por
   `git log`), então qualquer edição zera o contador. Tornar o campo configurável sem trocar a
   referência de tempo não resolve — exige decisão de design própria.
3. **`docs/schema/*.json` é schema morto** e `site/guide/ai-agents.md:68` afirma falsamente que
   `validate` valida contra ele.
4. `--from-req`, `--req`, `--title` e wizard TTY ausentes no Python — paridade de funcionalidade.
5. `adr_orphan` silencia erros de walk · padrão sistêmico `os.ReadFile → continue` (~30 sites × 3
   CLIs) · `check-identity-parity.sh` com `TARGETS` hardcoded · `parse_frontmatter` do Python sem strip
   de aspas · `findRoadmap` do Go sem erro de ambiguidade e sem filtro `.md` · log `by_agent` sem
   prefixo de agente no Python.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` — os dois defeitos são
casos de P2, e a correção só é aceita com a prova negativa exigida por P4.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md`
