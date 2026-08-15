---
status: Closed
date: 2026-08-15
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-15-gate-de-plugins-binario-deteccao-de-adulteracao-sem-deteccao-de-instalacao-e-chmod-apos-aprovacao.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-15-gate-de-seguranca-para-trackfw-plugins-add-binario-de-terceiro.md"
---

# REQ: Gate de segurança para `trackfw plugins install` — download de binário de terceiro sem parecer prévio

> Date: 2026-08-15 | Status: **Closed — substituída**
> Substituída por `docs/req/REQ-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`:
> em vez de cercar o download de binário de terceiro, o subsistema de plugins é **removido**.
> Preservada como registro da análise que levou a essa decisão.
| Linear Issue:
| Jira Issue:

## Motivation

Débito de segurança **consciente e datado**, criado deliberadamente pela decisão **D8(e)** do
`ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-...`. Não é achado novo nem
esquecimento: foi escopado para fora daquela REQ e registrado para virar esta.

**O estado atual, verificado no código em 2026-08-15** (`internal/plugins/plugins.go` e espelhos
`npm/src/commands/plugins.js`, `pypi/trackfw/commands/plugins.py`):

- `plugins add` → `Install(repo)` resolve `name/repo[@tag]` para uma URL de GitHub Releases e **baixa um binário de
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
- [ ] **AC4** — ✅ **Decidido por KG em 2026-08-15:** o `chmod 0755` só ocorre **depois** da
      aprovação; em quarentena o binário fica **sem bit de execução** (`0600`). O escopo atual
      (`~/.trackfw/plugins`, global) é **mantido**, para não quebrar quem já usa plugins.
      ⚠️ Consequência a resolver na Wave 0: escopo global fica **fora do perímetro da detecção**
      (mesma situação de D4-bis) — onde a proveniência do plugin deve morar para que sobre alguma
      detecção é a pergunta central do parecer.
      Estado atual verificado: Go faz `os.Chmod(tmpPath, 0755)` **antes** do rename
      (`internal/plugins/plugins.go:246`); Node nasce executável no próprio `writeFileSync`
      (`npm/src/commands/plugins.js:108`).
- [ ] **AC5** — ✅ **Decidido por KG em 2026-08-15: apenas o handshake.** Verificação de origem
      (checksum publicado pelo autor, assinatura, pinagem de release) fica **fora de escopo** e o
      limite vai **declarado no ADR** com o rigor de D3: o checksum garante que *o binário
      instalado é o que foi revisado*, e **não** que o autor publicou aquilo. É gate de revisão,
      não gate de supply-chain — confundir os dois é pior que não ter o gate.
- [ ] **AC6** — ⚠️ **REESCRITO em 2026-08-15 — a versão original era INIMPLEMENTÁVEL.** Ela pedia
      detecção de "plugin instalado sem proveniência" (ramo i), que o parecer do `hades-tf`
      demonstrou ser **estruturalmente impossível** com escopo global: `~/.trackfw/plugins` é
      compartilhado entre todos os projetos da máquina, então um plugin ali pode ter sido aprovado
      em outro projeto — e uma regra que tentasse detectar dispararia falso-positivo entre projetos
      não relacionados. Fica valendo:
      - **ramo (ii), adulteração pós-aprovação: EXIGIDO** — índice versionado no projeto, chaveado
        por nome de plugin, guardando o checksum aprovado; regra portada nos 3 CLIs;
      - **ramo (i), instalação sem aprovação: DECLARADO AUSENTE** no ADR (D2), com a alternativa
        rejeitada (mover plugins para escopo de projeto) registrada.
- [ ] **AC7** — Revisão do `hades-tf` **antes** do primeiro ML de implementação, como na REQ irmã —
      pré-requisito de sequenciamento, não entregável.
- [ ] **AC8** — ⚠️ **Corrigido em 2026-08-15 após mapeamento do código.** O comando real é
      **`trackfw plugins add`** (não `install`), e **o Python NÃO possui caminho de instalação**:
      `pypi/trackfw/commands/plugins.py` só tem `list` e `run` sobre executáveis `trackfw-*` já no
      `PATH`. Portanto:
      - **o gate** (quarentena + parecer + `chmod` tardio) vale para **Go e Node**; Python entra
        como **exceção intencional documentada** em `docs/cli-parity.md`, porque não há caminho de
        download a gatear — e implementá-lo só para ter o que gatear **inverteria a REQ**,
        adicionando justamente o vetor que ela existe para fechar;
      - **a regra de detecção** do `validate`, essa sim, é portada nos **3 CLIs** (o validator
        Python já espelha o Go em `pypi/trackfw/validator.py`).
      `make quality` passa sem novas divergências de paridade.

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

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-15-gate-de-seguranca-para-trackfw-plugins-add-binario-de-terceiro.md`

## Referências

- Parecer de segurança que classificou a severidade: `docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` (Q8e)
- Código atual: `internal/plugins/plugins.go`, `npm/src/commands/plugins.js`, `pypi/trackfw/commands/plugins.py`
- Doutrina de detecção: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
