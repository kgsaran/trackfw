---
status: wip
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md"
squad: "hades-tf, ares-tf, apolo-tf"
---

# Roadmap: O repositório do trackfw sob os cuidados do trackfw

> Created: 2026-09-01 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw.md`
ADR: `docs/adr/ADR-2026-09-01-o-repositorio-do-trackfw-e-governado-pelo-trackfw-...`

**O trackfw vende rastreabilidade aplicada e não a aplica a si mesmo.** Medido: `main` sem
`required_status_checks`, zero revisão exigida, `enforce_admins: false`; guards vivendo só no harness
de agente com `core.hooksPath = /dev/null`; e a cadeia nunca publicada como exigência.

**Qualquer PR pode ser mergeado com todo o CI vermelho.** Tudo o que construímos hoje é advisory.

## Acceptance Criteria

- [ ] Enumeração do que o trackfw instala em terceiros e não usa em si
- [ ] `required_status_checks` configurado, com a **escolha dos checks justificada**
- [ ] Guards ativos para humanos, **sem quebrar fluxo legítimo**
- [ ] Falsificação de cada controle **nas duas direções**
- [ ] `enforce_admins` decidido explicitamente
- [ ] 🔴 O `trackfw doctor` acusa estas lacunas — conserta todos os projetos, não só este
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — A enumeração é o entregável
> Dependências: nenhuma. Bloqueia tudo.

### ML-0A — O que o trackfw instala e este repositório não usa
**Status:** ⬜ Pendente
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que a enumeração é o trabalho, e não um preâmbulo:** as três lacunas conhecidas foram achadas
**por acidente**, investigando outra coisa. Não há razão para supor que sejam as únicas — e nesta
sessão duas enumerações minhas erraram por uma ordem de grandeza, com você achando a população real
nas duas.
**Actions:**
1. **Varra o que o produto gera:** `trackfw init`, `discover`, `update harness`, `integrations
   install`, `agents install`, `skills install`. Para cada artefato que ele instala em projeto de
   terceiro, responda: **existe aqui? está ativo? está atualizado?**
2. 🔴 **A distinção que decide o roadmap:** separar *"não usamos e deveríamos"* de *"não usamos e há
   razão"*. Nem tudo que o produto instala faz sentido no repositório do próprio produto — e tratar
   os dois como iguais produziria trabalho inútil e ruído.
3. **Modelo de ameaça do portão que vamos ligar.** `required_status_checks` mal escolhido **trava o
   projeto**: os jobs de Windows nascem vermelhos por projeto. Quais checks são exigidos, e por quê?
   E `enforce_admins` — num projeto com um mantenedor, a escotilha de emergência tem valor legítimo.
4. 🔴 **Falsificação nas duas direções, e a simétrica é a que dói:** cada controle que ligarmos pode
   **quebrar fluxo legítimo**. Guard ativo para humanos que impeça um `git commit` normal é pior que
   guard ausente. Nomeie o que **não** pode ser bloqueado.
5. **Residual declarado.**
**Critérios de aceite:**
- [ ] Enumeração com a distinção "deveríamos" × "há razão", item a item
- [ ] Veredito sobre quais checks exigir, com o custo de cada escolha
- [ ] Veredito sobre `enforce_admins`
- [ ] Nenhuma linha de configuração alterada
- [ ] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
```

## Wave 1 — Ligar os controles
> Dependências: Wave 0. Particionamento sai da enumeração.

## Wave 2 — O `doctor` acusa a lacuna
> Dependências: Wave 1. **É a wave que transforma o achado em produto:** as anteriores consertam este
> repositório; esta faz qualquer projeto que adote o trackfw ganhar o mesmo diagnóstico.

## Verificação

O portão só se prova **tentando mergear com CI vermelho** — e o controle, mergeando com CI verde.
Ambas exigem PR real; **não se verifica por leitura de configuração**.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`.
