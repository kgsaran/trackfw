---
status: Done
date: 2026-09-05
author: ""
adr: ""
roadmap: ""
---

# REQ: tres defeitos mecanicos medidos por consumidor externo: skips residuais, gate req_has_adr vacuo e ENOTDIR classificado como ausente

> Date: 2026-09-05 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O contribuidor externo `lourivalgarciajunior` abriu **sete issues** em 2026-09-05, medindo o trackfw
como consumidor num **Windows 11 real** — ambiente que o nosso CI não reproduz. A triagem
(`docs/portabilidade/2026-09-05-triagem-dos-sete-issues-do-lourival.md`) **confirmou os sete**, cada
um por medição própria, não por plausibilidade.

Esta REQ cobre os **três mecânicos** — os que têm mecanismo provado e **nenhuma decisão de
arquitetura pendente**. Os outros quatro seguem separados: `#275` é ADR de CI, `#273` tem REQ
própria já aberta, `#274` é passo de CI, `#277` é dívida de portabilidade fora do caminho crítico.

### `#279` — 9 `t.Skip` de classe plataforma sobraram do ML-4A

🔴 **É regressão do nosso próprio critério, e a mais desconfortável das três.** Em toda wave recente
o arquiteto escreveu em vermelho *"nenhum teste marcado `skip`, nenhum guard de plataforma que apague
asserção"* — e auditou **o diff de cada wave**, nunca o acervo. O critério valeu para o que cada wave
tocou; o resíduo ficou.

Confirmado por `git log -1` arquivo a arquivo: a `#269` tocou **só** `update_test.go` e converteu
**um** símbolo. Os 4 arquivos citados são resíduo genuíno.

### `#278` — `req_has_adr` detecta ADR vazio por literal

O gate compara literais, e **5 de 7 grafias de "vazio" escapam**. Medido no acervo real:
`trackfw validate` acusa **11** REQs; varredura independente achou **63** passando vacuosamente.
(Ele reportou 58 — diferença de método de classificação, não refutação.)

🔴 **É um gate vácuo dentro do produto que vende governança.** A mesma família de defeito que a
campanha de Windows vem caçando há uma semana, agora no artefato central do próprio trackfw: *um
veredito que não depende do estado real do alvo não é medição.*

### `#276` — `os.IsNotExist` classifica `ENOTDIR` como "não existe"

Sexto sítio de predicado de plataforma, em `internal/integrations/manager.go:477`, **fora do
validator** — por isso escapou das varreduras anteriores, que olhavam `internal/validator/`.
Medido em macOS: `os.IsNotExist(ENOTDIR) == false`, consistente com a tese.

## Acceptance Criteria

- [ ] **AC1** — Os 9 `skip` residuais tratados pelo **mesmo padrão do ML-4A**, não por invenção nova.
- [ ] **AC2** — 🔴 **Varredura do ACERVO, não do diff.** Entregar a contagem de `skip`/guard de
      plataforma em **todo** o repositório, com cada ocorrência classificada em *legítima* (o teste
      não faz sentido naquele SO) ou *apaga asserção*. Sem isso, fechamos 9 e o resíduo seguinte
      aparece do mesmo jeito — foi assim que estes 9 chegaram até aqui.
- [ ] **AC3** — `req_has_adr` passa a detectar as **7 grafias** de vazio. Falsificação nas duas
      direções: REQ com ADR real **não** é acusada; cada uma das 7 grafias **é**.
- [ ] **AC4** — 🔴 **A contagem do acervo é reportada antes e depois**, e o salto de 11 para dezenas
      é **esperado e declarado**, não tratado como regressão. Um gate que passa a acusar mais depois
      de deixar de ser vácuo está funcionando.
- [ ] **AC5** — 🔴 **Não corrigir as REQs do acervo nesta REQ.** Consertar o gate e consertar 60
      artefatos no mesmo diff torna impossível saber qual mudança produziu qual número. O acervo é
      trabalho seguinte, e provavelmente um baseline.
- [ ] **AC6** — `ENOTDIR` deixa de ser classificado como ausência. Falsificação: com o remendo
      revertido, o diagnóstico volta a sumir.
- [ ] **AC7** — 🔴 **Nenhuma correção esconde defeito.** Se em algum dos três o teste estiver certo e
      o produto errado, **parar e reportar**.

## Negative Scope

- ❌ **Não** decidir o ratchet por nome do `#275` — é ADR de CI, com armadilhas reais (lista virando
  cemitério, testes parametrizados, renomeação em refactor) e o próprio autor declara que o Windows
  dele **não é o runner**, logo a lista dele não serve como conteúdo inicial.
- ❌ **Não** tocar `#273` (tem REQ própria), `#274` (passo de CI) nem `#277` (dívida de
  portabilidade).
- ❌ **Não** relitigar a decisão do bit de execução em NTFS:
  `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`.
- ❌ **Não** corrigir o acervo de REQs sem ADR — ver AC5.

## Linked ADR
ADR: <!-- nenhum: os três têm mecanismo provado e nenhuma decisão de arquitetura pendente. -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-05-fechar-os-tres-defeitos-mecanicos-dos-issues-do-consumidor-externo.md


---

## Encerramento — 2026-09-05

Entregue no PR #281 (mergeado). Fecha os issues #278 e #279 automaticamente. O #276 NAO e fechado por decisao: a premissa foi refutada por medicao e a troca entrou como modernizacao — creditar como fix registraria causa que a medicao nao sustenta.
