---
status: Open
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-na-fronteira.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-02-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis.md"
---

# REQ: Substituir os parsers artesanais de config por biblioteca YAML nos tres CLIs

> Date: 2026-08-02 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Os três CLIs leem `trackfw.yaml` com parsers artesanais (~1085 linhas somadas) que produziram
**quatro** defeitos silenciosos em dois dias de trabalho. Cada um foi corrigido pontualmente, mas
o parser segue sendo **subconjunto** de YAML — listas aninhadas inline, mapas inline e âncoras
continuam sem suporte **e sem aviso**.

**A medição que redefine o trabalho:** adotar bibliotecas sem mais nada **troca** a divergência
artesanal por divergência de schema. Medido em 2026-08-02:

| Entrada | Go `yaml.v3` | Python `PyYAML` |
|---|---|---|
| `yes` | `"yes"` string | **`True` bool** |
| `010` | **`8` int** | **`8` int** |
| `2026-08-02` | **`time.Time`** | **`datetime.date`** |

`lenient_until` é `string // date string YYYY-MM-DD` e quebraria no dia 1.
`wip_limit: 010` viraria 8.

## Acceptance Criteria

- [ ] **AC1** — Os 3 CLIs carregam `trackfw.yaml` com biblioteca YAML: Go `gopkg.in/yaml.v3`
      (promover de indirect a direta), Node `js-yaml` ou `yaml`, Python `PyYAML`.
- [ ] **AC2** — **Normalização para string na fronteira.** Após o `load`, escalares chegam aos
      consumidores como **string**, listas como listas de string. Nenhum consumidor recebe
      `time.Time`, `date`, `int`, `float` ou `bool` da biblioteca.
- [ ] **AC3** — **Fidelidade textual**, verificada caso a caso nos 3 CLIs:
      | Entrada | Valor esperado no consumidor |
      |---|---|
      | `lenient_until: 2026-08-02` | `"2026-08-02"` — **não** timestamp formatado |
      | `wip_limit: 010` | `"010"` → `parseInt` → **10**, não 8 |
      | `wip_limit: 3` | `"3"` → 3 |
      | `wip_by_squad: true` | `"true"` → true |
      | `wip_by_squad: yes` | `"yes"` — os 3 **iguais**; hoje Go e Python divergem |
      | `k: 1.0` | `"1.0"` — não `"1"` |
      | `k: null` / `k: ~` | mesmo resultado nos 3 |
- [ ] **AC4** — **Comportamento do Node medido**, não presumido. A tabela do AC3 e a de tipos do
      ADR devem ser executadas contra a biblioteca escolhida e o resultado reportado. Se a
      biblioteca escolhida divergir de forma que a normalização não resolva, **reportar** antes de
      seguir.
- [ ] **AC5** — Todas as ~20 chaves de config seguem funcionando: `adr_dirs`, `agents`,
      `req_dir`, `roadmap_dir`, `roadmap_namespacing`, `acceptance_markers`, `link_fields`
      (e sub-chaves), `rules`, `wip_limit`, `wip_by_squad`, `stale_wip_days`, `lenient_until`,
      `governance_mode`, `require_req_in_commit`, `strict_ci_paths`, `trace_id_field`, `forge`,
      `squad`. Teste **por chave**.
- [ ] **AC6** — **Não regride nada dos quatro ciclos anteriores:** lista em bloco indentada e não
      indentada, lista inline (incluindo vírgula dentro de aspas), delimitador não pareado,
      ordenação do fallback de agentes.
- [ ] **AC7** — Formas antes **não suportadas** passam a funcionar: mapa inline (`{a: 1}`), lista
      aninhada inline, âncoras. Pelo menos uma de cada, testada.
- [ ] **AC8** — `config` ausente ou vazio continua caindo nos defaults, sem erro.
- [ ] **AC9** — `validate` e `status` verdes e **byte-idênticos** nos 3 no repositório real.
- [ ] **AC10** — `scripts/check-artifact-parity.sh` e `scripts/check-validate-parity.sh` passam.
- [ ] **AC11** — Cenário de falsificação com fixture contendo **`yes`, valor com cara de octal e
      data nua** — fixture de strings simples passa sob qualquer schema e não prova nada.
- [ ] **AC12** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **NÃO migrar o parser de frontmatter** (`parse_frontmatter`, `extractFrontmatterField`). É
  separado do config. Converter aplicaria coerção de data em **todo** campo `date:` de **todo**
  ADR e REQ — risco muito maior. Candidato a ADR próprio depois que o config estabilizar.
- **Não alterar o contrato de `ProjectConfig`** e equivalentes: mesmos campos, mesmos tipos.
- **Não alterar os consumidores** (`parseInt`, `== "true"`) — a normalização existe justamente
  para que eles não precisem mudar.
- Não alterar validadores, o comando `status`, nem mensagens de saída.
- Não adicionar chave de config nova.
- **Não configurar schema customizado** nas bibliotecas como alternativa à normalização — o ADR
  rejeitou por ser difícil de provar equivalente entre três APIs.
- Não alterar o status de nenhum ADR ou REQ do repositório.
- Não mexer em `pypi/build/lib/`.

## Notas de implementação

**Executor único nos 3 CLIs.** No ciclo anterior essa escolha produziu o primeiro trabalho
multi-CLI sem divergência nem ML de reconciliação. Aqui o alvo é mais difícil — semântica
idêntica entre três bibliotecas **diferentes**, com schemas diferentes.

O ponto de maior risco é o **AC3**: preservar a forma textual. Ler com a biblioteca e depois
"des-tipar" exige cuidado — `time.Time` de volta a `2026-08-02` e `8` de volta a `010` não são
reversões triviais. **Se a biblioteca perder a forma original, a normalização precisa acontecer
antes da coerção** (ex.: lendo o nó bruto em vez do valor tipado). Isso é decisão de
implementação, mas o resultado é contrato.

As medições completas de coerção de tipo estão reproduzidas no ADR pareado.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-na-fronteira.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-02-substituir-os-parsers-artesanais-de-config-por-biblioteca-yaml-nos-tres-clis.md
