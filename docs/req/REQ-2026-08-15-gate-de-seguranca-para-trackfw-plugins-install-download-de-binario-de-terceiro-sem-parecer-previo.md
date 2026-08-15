---
status: Open
date: 2026-08-15
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md"
roadmap: ""
---

# REQ: Gate de segurança para `trackfw plugins install` — download de binário de terceiro sem parecer prévio

> Date: 2026-08-15 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

Débito de segurança **consciente e datado**, criado deliberadamente pela decisão **D8(e)** do
`ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-...`. Não é achado novo nem
esquecimento: foi escopado para fora daquela REQ e registrado para virar esta.

**O estado atual, verificado no código em 2026-08-15** (`internal/plugins/plugins.go` e espelhos
`npm/src/commands/plugins.js`, `pypi/trackfw/commands/plugins.py`):

- `Install(repo)` resolve `name/repo[@tag]` para uma URL de GitHub Releases e **baixa um binário de
  terceiro**;
- grava em disco e faz **`chmod 0755`** — o artefato passa a ser diretamente executável;
- teto de 50 MiB (`maxPluginSize`) e `io.LimitReader`, mas **nenhuma validação de esquema de URL,
  nenhuma quarentena, nenhum parecer, nenhum registro de proveniência**;
- `grep -c "thirdparty|VerifyApproval|quarantine" internal/plugins/plugins.go` → **0**.

**A assimetria é o argumento central desta REQ.** O projeto acabou de construir um gate de duas
fases, com parecer obrigatório do `hades-tf` vinculado por checksum, para conteúdo **markdown** —
que apenas *influencia* o comportamento de um agente. Enquanto isso, o caminho que baixa um
**binário** e o torna **executável** segue sem gate algum. O parecer de segurança do ML-0A
(`docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md`, Q8e) classificou explicitamente este
vetor como de **severidade maior** que o caso markdown, pela razão óbvia: markdown influencia,
binário executa.

A separação em REQ própria foi decidida por escopo, **não por prioridade menor**: gate de binário é
superfície de ameaça materialmente distinta (assinatura, verificação de publisher, sandbox de
execução, permissão de arquivo) e teria arrastado de lado a Wave 2 daquele roadmap.

## Acceptance Criteria

- [ ] **AC1** — Nenhum caminho de código instala plugin de terceiro sem parecer prévio, na mesma
      propriedade estabelecida para artefatos markdown: baixar → quarentena → parecer → instalar.
      Nunca instalar e revisar depois.
- [ ] **AC2** — Reuso, e não reinvenção, do que já existe: handshake de duas fases (D8), quarentena
      por checksum, aprovação **vinculada por checksum** (fecha o TOCTOU), proveniência versionada
      e fail-closed (D6/D8f). Divergências em relação ao gate de markdown precisam ser justificadas
      no ADR desta REQ, não assumidas.
- [ ] **AC3** — Política de rede de binário definida explicitamente: HTTPS-only, limite de tamanho
      (o atual é 50 MiB — reavaliar), política de redirect e revalidação de esquema por hop, no
      mesmo rigor de D7/D7-bis.
- [ ] **AC4** — Decidir e documentar o tratamento de **permissão de execução**: o `chmod 0755` só
      pode acontecer **depois** da aprovação, nunca antes; avaliar se o artefato deve permanecer
      não-executável em quarentena.
- [ ] **AC5** — Avaliar verificação de **integridade e origem** que o caso markdown não tinha como
      exigir: checksum publicado pelo autor, assinatura, ou pinagem de release. Se a conclusão for
      "não é viável hoje", isso vai declarado no ADR, não omitido.
- [ ] **AC6** — Detecção equivalente à regra `thirdparty_artifact_has_provenance` (D2/D11),
      cobrindo plugins instalados sem proveniência e proveniência com checksum divergente.
- [ ] **AC7** — Revisão do `hades-tf` **antes** do primeiro ML de implementação, como na REQ irmã —
      pré-requisito de sequenciamento, não entregável.
- [ ] **AC8** — Comportamento idêntico nos 3 CLIs; `make quality` passa sem novas divergências de
      paridade.

## Escopo negativo (o que esta REQ NÃO faz)

- **Não** reabre nenhuma decisão do `ADR-2026-08-15` para artefatos markdown (D1–D11). O gate de
  skill/agent está entregue e auditado; esta REQ é aditiva.
- **Não** propõe sandbox de execução de plugin em runtime — o escopo é o **gate de instalação**.
  Sandbox é problema separado e maior.
- **Não** remove nem depreca o `trackfw plugins install`; o objetivo é gateá-lo, não matá-lo.
- **Não** trata do registry (`trackfw-plugins/registry.yaml`) como problema de confiança
  distribuída — só o ato de baixar e instalar.

## Linked ADR

ADR de origem (que criou este débito em D8e):
`docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md`

O ADR **próprio** desta REQ será escrito na Wave 0 do seu roadmap, após o parecer do `hades-tf`
(AC7), no mesmo padrão da REQ irmã.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: (a criar quando esta REQ sair do backlog — não iniciar sem REQ + roadmap em `wip`)

## Referências

- Parecer de segurança que classificou a severidade: `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` (Q8e)
- Código atual: `internal/plugins/plugins.go`, `npm/src/commands/plugins.js`, `pypi/trackfw/commands/plugins.py`
- Doutrina de detecção: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
