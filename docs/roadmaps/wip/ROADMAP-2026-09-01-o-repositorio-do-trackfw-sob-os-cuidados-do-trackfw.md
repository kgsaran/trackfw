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
**Status:** ✅ Concluído
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
- [x] Enumeração com a distinção "deveríamos" × "há razão", item a item
- [x] Veredito sobre quais checks exigir, com o custo de cada escolha
- [x] Veredito sobre `enforce_admins`
- [x] Nenhuma linha de configuração alterada
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-portao-do-repositorio.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

**Quatro achados mudam o plano, e o primeiro bloqueia a AC2.**

### 1. 🔴 O nome `governance` colide três vezes — e o dado estava na minha frente o dia todo

```
internal/generators/scaffold.go:1917         → trackfw-gate.yml      job id: governance
internal/generators/scaffold_doctor.go:45    → trackfw-validate.yml  job id: governance
                                               (dispara em push E pull_request)
```

Verificado ao vivo no PR #241: **três check-runs distintos com o mesmo nome**.

**Eu vi `"governance=SUCCESS","governance=SUCCESS","governance=SUCCESS"` em cada verificação de CI
que fiz hoje — uma dúzia de vezes — e nunca perguntei por que eram três.** A repetição normalizou a
anomalia.

**Por que bloqueia a AC2:** o GitHub casa check exigido **por nome**. Exigir `governance` com três
homônimos torna o portão **ambíguo** — satisfeito por qualquer um deles, imprevisivelmente. **Um
portão que parece fechado.** Precisa de job ids únicos **antes** de entrar no `required_status_checks`.

E como os dois geradores são do produto, **todo projeto que adota o trackfw herda a colisão.**

### 2. A AC3 não fecha a lacuna que a REQ usa para justificá-la

O único gerador de hook de git (`generateCommitMsgHook`) só atende husky/lefthook — **não há caminho
para um repositório Go-only como este**. E mesmo construído, `commit-msg` é **classe errada de
controle** para 2 dos 3 incidentes citados: `git stash` e `checkout --` não têm hook nativo que
dispare antes do subcomando, como o `PreToolUse` do harness de agente faz.

**Isto precisa estar dito na Wave 1**, não descoberto depois de declarar "AC3 concluída". A REQ
prometia paridade humano-agente que o git **não permite**.

### 3. A AC6 está subescopada

O `doctor` é **exclusivamente de sistema de arquivos** — manifesto e templates. Não tem visibilidade
alguma de API de branch protection nem de `core.hooksPath`. A AC6 exige **modalidade nova de
verificação** (rede + autenticação), não uma checagem a mais no desenho atual.

### 4. `enforce_admins` — ele defende `true`, e o argumento é bom

Com `required_approving_review_count: 0` num projeto de um mantenedor, `enforce_admins` decide **uma
única coisa**: se o portão vincula o admin. **Os quatro incidentes que a REQ cita como motivação
foram todos cometidos pelo admin, e nenhum foi bloqueado porque o CI nunca foi vinculante para ele.**

Recomenda `true`, com procedimento de flip temporário documentado e auditado como escotilha — não
buraco permanente.

### Achado positivo que o ADR não mencionava

`allow_force_pushes` e `allow_deletions` **já são `false`** na `main`.

### O que NÃO pode ser bloqueado (§4 do parecer)

Commits comuns fora de `feat/*`/`fix/*`; pushes para branch que não é `main`; **o PR autorreferente
que conserta um `governance` quebrado**; e os jobs de medição de Windows. Cada um com o mecanismo
concreto que poderia quebrá-lo.

O terceiro é o mais sutil: **um portão que exige `governance` verde impede o PR que conserta o
`governance`.**

### Residual

Dois dos três incidentes que motivam a AC3 são **permanentemente inatingíveis** por hook de git; o
review-count segue ponto único de falha; o flip de emergência é ele próprio ação de admin não
revisada; e a ausência de `.claude/agents/` e `.claude/skills/` em escopo de projeto **precisa de
decisão do KG** — pessoal versus compartilhável —, não de veredito dele.

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
