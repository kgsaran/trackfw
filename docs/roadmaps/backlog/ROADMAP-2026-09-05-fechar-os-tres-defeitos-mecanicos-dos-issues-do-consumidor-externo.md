---
status: backlog
date: 2026-09-05
squad: artemis-tf
req: "docs/req/REQ-2026-09-05-tres-defeitos-mecanicos-medidos-por-consumidor-externo-skips-residuais-gate-req-has-adr-vacuo-e-enotdir-classificado-como-ausente.md"
---

# Roadmap: Fechar os três defeitos mecânicos dos issues do consumidor externo

> Criado em: 2026-09-05 | Status: backlog

## Context

REQ: `docs/req/REQ-2026-09-05-tres-defeitos-mecanicos-medidos-por-consumidor-externo-skips-residuais-gate-req-has-adr-vacuo-e-enotdir-classificado-como-ausente.md`
Triagem: `docs/portabilidade/2026-09-05-triagem-dos-sete-issues-do-lourival.md`

## Diagnóstico

Três dos sete issues do contribuidor externo, todos **confirmados por medição**, todos com mecanismo
provado e **nenhuma decisão de arquitetura pendente**.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — Os três, em paralelo (arquivos disjuntos)
> Dependências: PR #280 mergeado. 🔴 **Disjunção a CONFERIR arquivo a arquivo antes do despacho** —
> já criei uma colisão nesta campanha afirmando disjunção sem abrir os arquivos (ML-4A/4B).

### ML-1A — `#279`: os 9 `skip` residuais, e a varredura do ACERVO
**Status:** ⬜ Pendente · **Agente:** `artemis-tf`
Tratar os 9 pelo **mesmo padrão do ML-4A**, sem inventar padrão novo.

🔴 **O entregável principal NÃO são os 9 — é a varredura do acervo inteiro** (AC2), com cada
`skip`/guard de plataforma classificado em **legítimo** (o teste não faz sentido naquele SO) ou
**apaga asserção**. Fechar 9 sem varrer garante que o próximo resíduo apareça igual: foi exatamente
assim que estes 9 chegaram até aqui — o arquiteto auditou **o diff de cada wave**, nunca o acervo.

### ML-1B — `#278`: `req_has_adr` deixa de detectar vazio por literal
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (3 CLIs — regra dura de paridade)
5 de 7 grafias de vazio escapam. Falsificação nas duas direções: REQ com ADR real **não** acusada;
cada uma das 7 grafias **é**.

🔴 **A contagem do acervo salta de 11 para dezenas, e isso é ACERTO, não regressão** — declare antes
e depois. Um gate que passa a acusar mais depois de deixar de ser vácuo está funcionando.
🔴 **Não corrigir as REQs do acervo aqui** (AC5): consertar o gate e 60 artefatos no mesmo diff
torna impossível atribuir qual mudança produziu qual número.

### ML-1C — `#276`: `ENOTDIR` deixa de ser classificado como ausência
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
`internal/integrations/manager.go:477` — sexto sítio de predicado de plataforma, **fora do
validator**, que é por isso que escapou das varreduras anteriores. Trocar `os.IsNotExist` por
`errors.Is(err, fs.ErrNotExist)`. Verificar os pares Node/Python.
**Falsificação:** com o remendo revertido, o diagnóstico volta a sumir.

## Acceptance Criteria da wave
- [ ] Falsificação nas duas direções em cada ML, com números
- [ ] 🔴 Varredura do **acervo** entregue no ML-1A, não só os 9
- [ ] 🔴 Nenhuma correção esconde defeito — se o produto é que está errado, **parar e reportar**
- [ ] `make quality` verde e `trackfw validate` exit 0, rodados **pelo arquiteto**, uma vez, ao fim

## Fora deste roadmap
`#275` (ratchet por nome) → **ADR de CI**, com as armadilhas nomeadas.
`#273` → REQ própria já aberta. `#274` → passo de CI. `#277` → dívida de portabilidade.
