---
status: Open
date: 2026-09-05
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-05-reconciliar-o-que-declaramos-com-o-que-medimos-apos-a-auditoria-externa.md"
---

# REQ: auditoria externa aponta que declaramos correcao onde a nossa propria medicao dizia o contrario

> Date: 2026-09-05 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Uma auditoria externa (GPT-6 Astra, 2026-09-05, referência `main` em `4c69289`) analisou a campanha
de Windows e os issues do consumidor externo. **Verifiquei os três achados mais duros e os três
procedem.** Registro completo, com o comando de cada verificação:
`docs/portabilidade/2026-09-05-auditoria-externa-astra-achados-e-verificacao.md`.

### A1 — Introduzimos um vermelho novo na `main`

O ML-1C **mediu e escreveu** que `ENOTDIR` é indistinguível de ausência no Windows
(`ENOTDIR = ERROR_PATH_NOT_FOUND`) — e **na mesma entrega** deixou um teste afirmando o contrário.
Ele reprova no CI de Windows:

```
manager_collision_enotdir_test.go:58: detectNameCollision(ENOTDIR) = nil,
want a reported error (ENOTDIR must not be classified as absence)
--- FAIL
```

O ML foi marcado ✅ e eu mergeei **sem cruzar o teste novo com a conclusão do próprio ML**.

### A2 — O `#278` foi fechado cobrindo menos do que parece

As 7 grafias de vazio, sim. **Interpretar o vínculo, não:** `ADR: <!-- preencher depois -->` e prosa
contendo `ADR:` continuam contando como valor. A issue está fechada.

### A3 — Afirmamos publicamente um discriminante que não discrimina

No comentário do `#274` eu escrevi que `pass 0 / fail 1` distingue "a suíte não carregou". Medido:
**um teste que roda e reprova dá exatamente os mesmos contadores.**

### 🔴 O padrão, que é o motivo real desta REQ

Nos três, **produzimos a evidência e não a cruzamos com a nossa própria declaração**:

| | medimos | e mesmo assim declaramos |
|---|---|---|
| A1 | `ENOTDIR` indistinguível no Windows | teste afirmando que é distinguível |
| A2 | 7 grafias de vazio | vínculo resolvido |
| A3 | — | discriminante que nunca testamos |

**Não é falta de medição — é falta de reconciliação.** A campanha inteira exigiu falsificação dos
agentes e recebeu; o que faltou foi o passo em que **eu**, ao auditar, confronto o artefato entregue
com a conclusão escrita no mesmo relatório.

## Acceptance Criteria

- [ ] **AC1** — O vermelho novo sai da `main`. 🔴 **Reconciliar teste e conclusão, não só apagar o
      teste:** ou o teste passa a afirmar o que foi medido (`ENOTDIR` **é** tratado como ausência no
      Windows, e isso é do SO), ou é removido com a razão escrita. **Marcar `skip` não é saída** —
      esconderia a contradição em vez de resolvê-la.
- [ ] **AC2** — 🔴 **Falsificação da correção do AC1 no Windows real (CI), não em macOS.** Foi
      confiar em suíte verde local que produziu o defeito.
- [ ] **AC3** — `req_has_adr` deixa de aceitar **placeholder** e **prosa** como vínculo. Falsificação
      nas duas direções, incluindo `<!-- ... -->` e frase contendo `ADR:` no meio.
- [ ] **AC4** — 🔴 **A contagem do acervo é remedida e declarada de novo.** Ela vai subir acima de 67,
      e isso continua sendo acerto. Sem AC4 a correção parece regressão.
- [ ] **AC5** — O comentário público no `#274` é **corrigido na própria issue**, com a medição que o
      derruba. 🔴 Não editar o original silenciosamente: um novo comentário, dizendo o que estava
      errado. Quem leu a afirmação errada precisa poder encontrar a correção.
- [ ] **AC6** — 🔴 **Guarda contra a repetição do padrão.** Todo ML cuja entrega inclua teste novo
      precisa declarar, no relatório, **qual conclusão do próprio ML aquele teste afirma** — e a
      auditoria do arquiteto verifica esse cruzamento. É o passo que faltou nos três achados, e sem
      ele os outros ACs corrigem instâncias, não a causa.
- [ ] **AC7** — As REQs com `adr:` vazio que apontam para ADRs **existentes e aceitos** são
      reconciliadas. Achado de governança da mesma auditoria.

## Negative Scope

- ❌ **Não** implementar o ratchet de CI aqui — tem ADR própria
  (`ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md`).
  Esta REQ conserta o que já quebrou; a ADR impede a próxima.
- ❌ **Não** reabrir `#278`. A parte fechada está de fato corrigida; a residual entra por AC3 com
  referência explícita, sem reescrever o histórico.
- ❌ **Não** corrigir o acervo de REQs sem ADR — segue sendo baseline em trabalho próprio.
- ❌ **Não** tratar a lacuna de CRLF em renderizadores nem a jornada de instalação em Windows: são
  achados legítimos da mesma auditoria, com superfície própria, e misturá-los aqui repetiria o erro
  que esta REQ existe para corrigir.

## Linked ADR
ADR: <!-- nenhum: correção de contradição interna e de método de auditoria. A decisão de CI que
     previne a recorrência está na ADR do ratchet, deliberadamente separada. -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-05-reconciliar-o-que-declaramos-com-o-que-medimos-apos-a-auditoria-externa.md
