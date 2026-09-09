---
status: Open
date: 2026-09-09
author: ""
adr: ""
roadmap: ""
---

# REQ: REQ nasce orfa porque criar REQ e criar roadmap sao dois comandos e o segundo se esquece

> Date: 2026-09-09 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

**Seis REQs órfãs encontradas em três dias**, todas por acaso, durante outro trabalho. O
`req_has_roadmap` avisa — e o aviso não impediu nenhuma delas.

### Medição 1 — o tamanho e a natureza do passivo

```
warnings de req_has_roadmap          56
REQs Open sem roadmap                32
   das quais abertas em setembro     18
```

Das 32, por classificação **heurística** (não medição):

```
com critérios de aceite, pedem implementação    ~14
aparentam decisão/registro puro                 ~18
```

🔴 **Declaro o limite do classificador:** ele casa palavras-chave (*"decisão"*, *"ADR própria"*) e
testa esse ramo **primeiro** — então REQ que tenha as duas coisas cai em "decisão". O número real de
"pedem implementação" é **≥ 14**, provavelmente maior. Não use estes números como escopo sem
re-triagem.

**O que a medição sustenta com segurança:** **18 das 32 nasceram em setembro** — o passivo não é
histórico, está sendo produzido **agora**, no ritmo atual, com o aviso ativo.

### Medição 2 — 🔴 a ferramenta já existe e o fluxo não a usa

```
trackfw roadmap new --from-req <REQ>
  "Generate roadmap with ML stubs from REQ acceptance criteria"

trackfw req new
  (nenhuma opção de criar o roadmap junto)
```

**A capacidade existe. O caminho inverso não.** Quem cria a REQ precisa lembrar de um segundo comando
— e é esse segundo comando que se esquece.

🔴 **Mesmo padrão do achado A1 desta campanha:** o `pathIsAnchoredForHookConfig` existia e o sítio não
o usava; o `--from-req` existe e o fluxo não o usa. **O defeito não é ausência de capacidade, é
ausência de ligação.**

## Por que a inversão direta da severidade NÃO funciona

Virar `req_has_roadmap` para `error` hoje faz o `validate` falhar em **32 REQs**. Consequência
previsível: alguém configura `lenient` ou baixa a severidade — e aí perdemos **a regra e o aviso**.

É o que já acontece com o `windows-full-suites`: job que nasce vermelho vira ruído que se aprende a
ignorar. E é a crítica que o autor do issue #275 fez sobre contagem.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Prevenção antes de gate:** criar REQ e roadmap deixa de exigir dois comandos.
      Forma a decidir (flag em `req new`, prompt, ou `req new` chamando `--from-req`) — **medir o
      atrito de cada uma**, não escolher por gosto.
- [ ] **AC2** — severidade endurecida **por data de corte**, não por contagem: REQ criada a partir de
      `<corte>` sem roadmap ⇒ **error**; anterior ⇒ **warning**, grandfathered. Corte declarado no
      artefato.
- [ ] **AC3** — 🔴 **O grandfathering é visível, não silencioso.** O relatório diz quantas REQs estão
      isentas e desde quando. Isenção que não se vê vira permanente.
- [ ] **AC4** — decisão escrita sobre **onde bloqueia**: só `validate`, ou também `push`. 🔴 O `push`
      hoje exige REQ+roadmap **da branch**, não de toda REQ — são coisas diferentes e a decisão precisa
      dizer qual muda.
- [ ] **AC5** — re-triagem das 32: quantas **legitimamente** não têm roadmap (decisão pura, fechada sem
      implementação). O classificador heurístico acima **não serve** como escopo.
- [ ] **AC6** — paridade nos 3 CLIs.

## Negative Scope

- **Não** criar roadmap automático para as 32 existentes — roadmap vazio gerado em massa é pior que
  REQ órfã: parece cobertura e não é.
- **Não** endurecer `req_has_adr` no mesmo movimento — são **103** avisos, causa distinta, e existe
  ADR de ratchet própria para o acervo.
- **Não** alterar o gate de branch do `push` sem a decisão do AC4.
