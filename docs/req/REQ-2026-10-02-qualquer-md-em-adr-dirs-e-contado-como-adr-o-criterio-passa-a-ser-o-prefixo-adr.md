---
status: Done
date: 2026-10-02
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-02-o-criterio-de-identificacao-de-adr-e-o-prefixo-adr-no-nome-do-arquivo-aplicado-no-primitivo-unico-de-enumeracao.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md"
---

# REQ: qualquer .md em adr_dirs é contado como ADR — o critério passa a ser o prefixo ADR-

> Date: 2026-10-02 | Status: Done
| GitHub Issue: #471
| Mergeado: PR #503 em `2dd76b40` (2026-10-02); CI 20/20, inclusive `windows-full-suites`. #471 fechada pelo merge.

## Motivation

O critério de identificação de ADR é "termina em `.md`". Um `NOTAS.md` em `adr_dirs` é contado por
`status`/`context` e listado pelo `adr list`. O `validate` o acusa como "ADR sem frontmatter", e o
`discover` dá a ele os mesmos +20 de Governance Score de um ADR real. Medido no comentário da #471,
com fixture de três braços (vazio / só `NOTAS.md` / um ADR real).

Decisão do KG (ADR ligado): o critério passa a ser o **prefixo `ADR-`** no basename, sem distinção de
maiúsculas. O custo medido em 6 acervos é 0 de 133 ADRs. O critério vive no primitivo único, e os
dois sítios de fora (`serve` e a sonda do `discover`) passam a usá-lo.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model e completude: todo sítio que enumera ADR está na tabela
  do roadmap, ou o que falta é nomeado. Parecer em `docs/seguranca/`.
      ✅ Evidência: `docs/seguranca/2026-10-02-wave0-criterio-de-adr.md`, APROVA COM AJUSTES (A1–A5 absorvidos)
- [x] **AC2** — Fixture de três braços da #471 com o binário: `adr_dirs` vazio / só `NOTAS.md` / um
  ADR real. O braço `NOTAS.md` dá o **mesmo** resultado do vazio em `status` (ADRs 0), `context`
  (score), `discover` (score), `adr list` (não lista) e `validate` (sem violação de frontmatter).
  Reprova no binário de `44718ffc` e passa na branch.
      ✅ Evidência: `TestADRPrefixE2E_AC2_ThreeArms` (binário): reprova em `44718ffc`
- [x] **AC3** — `adr-001-x.md` (minúsculo) conta como ADR.
      ✅ Evidência: `TestADRPrefixE2E_AC3_LowercasePrefixCounts` + unitário `TestWalkADRFilePathsForRule_LowercaseADREnumerated`
- [x] **AC4** — `serve`: o endpoint do grafo (`/api/chain`) não inclui nó para `NOTAS.md` e inclui
  para `ADR-…`.
      ✅ Evidência: `TestChainHandler_ADRPrefixFilter_NoNotasNodeButADRReqRoadmapPresent`
- [x] **AC5** — `discover` sem `adr_dirs` declarado (sonda em `docs/adr`): `NOTAS.md` não credita a
  categoria ADR.
      ✅ Evidência: `TestADRPrefixE2E_AC5_DiscoverFallbackIgnoresNOTAS` + `TestScan_Fallback{Flat,Subdir}_NotasNotCounted`
- [x] **AC6** — Regra `adr_file_without_prefix` (warning): dispara para `.md` sem prefixo com
  frontmatter `status:`; **não** dispara para `README.md` sem frontmatter nem para `ADR-…`.
      ✅ Evidência: `TestADRPrefixE2E_AC6_*` + 5 unitários `TestADRFileWithoutPrefix_*`
- [x] **AC7** — O `adr new` não é afetado pelo critério, e o `NOTAS.md` não entra na contagem depois da criação.
  🔴 Premissa original corrigida pelo ML-2A: o `adr new` nomeia pela **data** (`ADR-YYYY-MM-DD-slug.md`),
  não por número sequencial. "Próximo número igual" não existia para medir.
      ✅ Evidência: `TestADRPrefixE2E_AC7_AdrNewUsesDateSlug` (reprova em `44718ffc` pela contagem do `NOTAS.md`)
- [x] **AC8** — `docs/cli-parity.md` descreve o critério e a regra nova; pinos/gates de conjunto de
  regras atualizados e verdes.
      ✅ Evidência: pinos 32/32 (pin28/pin29 novos); seção nova no `cli-parity.md`
- [x] **AC9** — Todo teste novo declara a conclusão que afirma; cada um reprova com o critério antigo.
      ✅ Evidência: prova de mordida por overlay em todos os ML; uma frase por teste
- [x] **AC10** — `make quality` EXIT=0 e CI do PR verde, inclusive `windows-full-suites`.
      ✅ CI do PR #503 em c61300ca: 20/20 SUCCESS, inclusive windows-full-suites.
      ⏳ Local: `make quality` passou todas as etapas até a falsificação; a falsificação, rodada em 2 grupos de 4 chunks (o paralelo de 8 pendurou 3x nesta máquina), deu 347 OK / 0 FAIL, 8/8 CHUNK_COMPLETE. Falta o CI do PR.

## Negative scope

- **Validade do frontmatter de ADR:** as regras atuais não mudam; o `ADR-001` sem frontmatter
  continua sendo acusado como hoje.
- **Critério de REQ e roadmap:** não muda (outra entidade, outro prefixo).
- **#481** (`branch prune`), **#421**, **#408**, **#403:** outras causas.
- **Migração de acervo de terceiros:** não renomeamos arquivos; o D4 avisa.

## Linked ADR
ADR: docs/adr/ADR-2026-10-02-o-criterio-de-identificacao-de-adr-e-o-prefixo-adr-no-nome-do-arquivo-aplicado-no-primitivo-unico-de-enumeracao.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md
