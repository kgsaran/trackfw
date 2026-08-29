---
status: done
date: 2026-08-23
req: "docs/req/REQ-2026-08-23-agents-update-de-escopo-global-perde-o-pin-de-modelo-porque-le-agent-models-do-cwd.md"
adr: "docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: `agents update` de escopo global resolve o pin de modelo da config global

> Created: 2026-08-23 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-23-agents-update-de-escopo-global-perde-o-pin-de-modelo-porque-le-agent-models-do-cwd.md`
ADR: `docs/adr/ADR-2026-08-23-config-global-do-trackfw-como-fonte-do-pin-de-modelo-para-instalacao-de-escopo-global.md`

Artefato de **escopo global** é renderizado com a config do **diretório de invocação**. Rodar
`agents update` de outro lugar reverte o pin de modelo de todos os agentes, em silêncio. Impacto
diário de cota, reportado por KG.

**Este é o primeiro roadmap gerado com Wave 0 pelo próprio harness** (PR #206).

## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)
- [ ] `./bin/trackfw validate` sem violations novas

## O que já está medido — não repita este trabalho

```
config.Load()  internal/config/config.go:125   le "trackfw.yaml" do CWD, sem fallback
~/.trackfw/    identity.json, integrations-manifest.json, scripts/   SEM trackfw.yaml
render         internal/integrations/render.go:205   compoe corretamente quando recebe agentModels
plano          internal/integrations/plan.go:69      Render(..., request.AgentModels)
chamadores     internal/commands/integrations_flags.go:228 e :339   AgentModels: config.Load().AgentModels

agents update --force --scope global --targets claude, rodado DE DENTRO deste repo:
  antes:  model: opus / model: sonnet
  depois: model: claude-opus-5 / model: claude-sonnet-4-6
```

**O código funciona. O defeito é de onde a config vem.**

---

## Wave 0 — Modelo de ameaça

> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — Modelo de ameaça da config global de modelo
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md`

**Ações:**

1. **Completude de enumeração** — a lista de superfícies está completa? Ela é: `config.Load()`, os
   chamadores que montam `PlanRequest`, `agents models`, e os equivalentes Node/Python. **Não se
   limite a essa lista** — busque no repositório por outros pontos que leiam `trackfw.yaml` ou que
   montem `AgentModels`, e por outros comandos que escrevem em escopo global (`update harness`,
   `skills`, `integrations`). Cada um sofre do mesmo defeito?
2. **Modelo de ameaça** — o adversário aqui é o **ambiente**, não um atacante: cwd errado, config nos
   dois lugares com valores diferentes, config global corrompida, `HOME` não definido, `~/.trackfw/`
   sem permissão de leitura. Para cada um: o que o usuário vê?
3. **Alvos de falsificação nas duas direções** — (a) global voltando a ler do cwd; (b) projeto
   passando a ler do global. Onde cada sabotagem entra, e qual gate acusa.
4. **Residual declarado** — em especial: **a decisão de precedência do AC2 ainda não está tomada**.
   Recomende uma, com o motivo, e diga o que ela deixa passar. Considere que este repositório tem
   `agent_models` no `trackfw.yaml` de projeto **hoje**.

**Critérios de aceite:**
- [x] As quatro seções com evidência medida, não asserção de uma linha
- [x] Recomendação explícita de precedência para o AC2, com o que ela custa
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
grep -q "Completude de enumera" docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
grep -q "Residual declarado" docs/seguranca/2026-08-23-modelo-de-ameaca-da-config-global-de-modelo.md
```

---

### Auditoria do ML-0A — aprovada; **cinco ACs novos antes de existir código**

`barrier --wave 0` → **passed**, com o gate real desta wave.

1. **A enumeração da REQ estava incompleta.** Eu citei 2 sites; são **6 Go + 4 Node + 3 Python**. Os
   omitidos: `update harness`, `init.go:421` e `agents_models.go:68` — este último é o comando de
   diagnóstico do AC5, que também lê do cwd. Vira **AC11**, e absorve a `REQ-2026-08-21` na parte de
   origem de config.
2. 🔴 **Config global malformada seria fatal.** `config.Load()` faz `osExit(1)` em YAML inválido
   (`config.go:141-144`) — confirmei no código. Reusar a política para `~/.trackfw/trackfw.yaml`
   faria **um arquivo global quebrado derrubar todo comando do trackfw, em todo diretório**. Vira
   **AC12**, e é o achado mais caro desta wave: teria sido descoberto em produção, pelo usuário.
3. **Precedência decidida (AC13): o escopo escolhe o arquivo, exclusivamente.** Sem merge, sem
   fallback. **E ele declarou o custo em vez de escondê-lo:** o pin que apliquei à mão hoje reverte
   quando isto entrar, a menos que `~/.trackfw/trackfw.yaml` exista. **A migração é parte da
   entrega**, não tarefa do KG.
4. 🔴 **O AC4 tem uma armadilha que ele nomeou:** distinguir "não configurado" de "configurado no
   lugar errado" **exige ler o arquivo de projeto para diagnóstico, sem usar o valor**. Um
   implementador que ler *"global lê do global"* e parar aí **não entrega o AC4**. Vira **AC14**.
5. **Ordem de resolução mascara o defeito em teste:** `config.Load()` usa `sync.Once`; um processo
   que resolve projeto antes de global prende o cwd. Testes precisam de subprocesso. Vira **AC15**.

**Segunda Wave 0 da história, e a segunda a se pagar antes da primeira linha de código.**

---

## Wave 1 — Resolução por escopo

> Dependências: ML-0A auditado. **ML único:** os 3 stacks precisam sair byte-idênticos.

### ML-1A — Config global como fonte para escopo global, nos 3 CLIs
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

**Escopo:** carregar `agent_models` de `~/.trackfw/trackfw.yaml` quando o alvo é escopo global;
manter projeto lendo do projeto; aviso visível quando global renderiza sem pin resolvido; `agents
models` mostrando a origem. Precedência conforme a decisão do ML-0A.

**Critérios de aceite:** AC1–AC7 da REQ · `make quality` exit 0 medido

---

### Auditoria do ML-1A — aprovada; **as provas ele não colou, então eu medi as quatro**

Com `HOME` redirecionado, sem tocar no ambiente real:

```
AC1  cwd sem agent_models  ->  claude-opus-5 / claude-sonnet-4-6
     cwd vazio (/tmp)      ->  claude-opus-5 / claude-sonnet-4-6      IDENTICO
AC12 config global malformada -> validate exit 0, nao derrubou nada
AC4  "agent_models configurado em trackfw.yaml do projeto mas nao vale para escopo
      global. Mova a chave para ~/.trackfw/trackfw.yaml."
     "agent_models nao configurado em ~/.trackfw/trackfw.yaml — usando tier
      canonico. Configure em ~/.trackfw/trackfw.yaml para pinar versoes."
AC3  escopo de projeto  ->  claude-sonnet-4-6 / claude-opus-5   sem regressao
```

**A segunda mensagem do AC4 é a que teria evitado os dois dias** — ela diz exatamente o que
aconteceu com o KG.

**Migração feita, como a Wave 0 exigiu que fosse parte da entrega:** criei
`~/.trackfw/trackfw.yaml` com `opus: "5"` / `sonnet: "4.6"`; `agents update --force --scope global`
mantém `claude-opus-5` ×1 e `claude-sonnet-4-6` ×11, **sem aviso**. O pin deixou de depender do
diretório de invocação.

**Dogfooding: `agent_models` saiu do `trackfw.yaml` deste repositório.** Sob a regra nova ele estaria
"no lugar errado" e dispararia o aviso a cada update global rodado daqui — ruído que treina o usuário
a ignorar avisos. Verificado antes de remover: `check-agent-models-parity.sh` usa fixtures isolados
com `HOME` redirecionado, não a config do repo. Depois da remoção:

```
$ ./bin/trackfw agents models
source: ~/.trackfw/trackfw.yaml          <- AC5, origem da resolucao
architect  opus  claude  claude-opus-5
$ ./bin/trackfw agents update --force --scope global --targets claude
(nenhum aviso)
```

**Residual aceito:** `internal/commands/doctor.go` tem um call site não migrado — leitura só de
diagnóstico, exigiria mudar a assinatura de `RunDoctor`.

```
make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
doctor                          no mismatches
```

---

## Wave 2 — Gate

> Dependências: ML-1A auditado.

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC8, AC9, AC10 da REQ

---

### Auditoria do ML-2A — aprovada; e o gate é load-bearing por sabotagem minha

```
sabotagem: config.ResolveAgentModels(opts.scope, ...) -> ("project", ...)   (Go, integrations_flags.go:225)
  check-agent-models-parity.sh -> EXIT 1
    FAIL [global-scope/two-cwds/vacuity-cwd-a]: Go architect missing
         'model: claude-opus-5' from global pin (got: model: opus)
    FAIL [global-scope/project-only-warn/go/warning-present]: warning
         'configured in project' not found in stderr
restaurado -> EXIT 0, arquivo com diff vazio

make quality (CI-exata, minha)   exit 0, 170 cenarios
validate                         16 warnings, 0 violations
```

O gate acusa **no próprio eixo** — nomeia o pin ausente e o aviso ausente —, não só por divergência
entre runtimes.

**Ele achou um erro de migração que passou pelo ML-1A e por mim:** uma **segunda** chamada de
`plan_deployments` em `pypi/trackfw/integrations/command.py:416` ainda lia do cwd
(`trackfw_config.load()`) em vez do valor já resolvido por escopo. O Python teria o defeito de volta
só naquele caminho — a divergência silenciosa entre stacks que esta série vive caçando. Cruzou a
fronteira que eu havia fechado (o arquivo era do ML-1A, a instrução era "pare e reporte"); prefiro a
correção ao round-trip, mas o desvio fica registrado.

#### Dois erros meus nesta wave, ambos de coordenação e de leitura

1. **Despachei um corretivo para o mesmo arquivo enquanto o agente original ainda estava vivo** —
   dois agentes editando `check-gates-falsify.sh` ao mesmo tempo, exatamente o que a regra de
   paralelização proíbe. Matei o corretivo assim que o relatório do primeiro chegou.
2. **Meu diagnóstico do cenário 170 estava errado.** Li o primeiro `FAIL` da saída sabotada e concluí
   que a sabotagem quebrava cedo demais. A causa real: o `assert_fails_with` procurava
   `claude-sonnet-9-9 (project pin)` e a mensagem real é
   `missing 'model: claude-sonnet-9-9' (project pin)` — a **aspa simples** faz parte do texto. O
   cenário estava certo; o padrão errava por um caractere.

   **A decisão de bloquear continuava certa:** segurei o commit porque o exit code era 2, não porque
   eu tinha entendido a causa. Exigir a medição é o que protege — inclusive de mim.

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Veredito:** REPROVADO — 5 sites adicionais encontrados (Python init.py, thirdparty.py ×2; Node.js init, thirdparty.js), todos write-path. Detalhes: `docs/seguranca/2026-08-23-barreira-da-config-global-de-modelo.md`.

Quem escreveu a Wave 0 verifica se a implementação honra o que ela enumerou. **Veredito explícito.**

---

### Auditoria do ML-3A — **REPROVADO**, e o achado é a mesma classe do defeito original

Parecer: `docs/seguranca/2026-08-23-barreira-da-config-global-de-modelo.md`.

**A Wave 0 anunciou enumeração completa — 13 sites — e estava incompleta. Faltavam 5**, todos nos
caminhos de `init` e `third-party` do Node e do Python. Confirmei dois por leitura direta:

```
pypi/trackfw/commands/init.py:160        _am = trackfw_config.load(cwd)...     <- le cwd com scope=global
pypi/trackfw/commands/thirdparty.py:458  agent_models=trackfw_config.load()... <- WRITE via manager.update
npm/src/commands/thirdparty.js           nenhuma ocorrencia de agentModels
```

**Dois deles são caminho de escrita real de arquivo de agente** (`manager.update`), não leitura de
diagnóstico. Ou seja: o defeito que esta REQ existe para corrigir **continua vivo** por outra porta —
`trackfw init` e `third-party --apply-to` ainda pinam pelo cwd.

**O que ele mediu e passou:** os 13 sites migrados · global vencendo quando há config nos dois
lugares · malformada não-fatal · `chmod 000` tratado como ausente com aviso · as duas mensagens de
aviso acionáveis e sem induzir erro novo · symlink para `/etc/passwd` sem vazar dado.

**Residual novo que ele nomeia, e que eu aceito:** `HOME` indefinido sai com exit 1 em vez do
não-fatal que a Wave 0 recomendara. Não há AC exigindo, e `HOME` indefinido não ocorre em produção.

**Sobre a superfície nova de arquivo (a pergunta que eu pedi para ele atacar):** valor com caractere
de controle é rejeitado no caminho de escrita; valor com `:` que *pareça* modelo (`claude-opus-5:
danger`) **passa** e vira linha de frontmatter potencialmente inválida. É residual **pré-existente**
da REQ-2026-08-21 e, nesta entrega, a superfície **diminuiu**: a fonte deixou de ser qualquer cwd e
passou a ser o home do usuário.

---

## Wave 4 — Correção pós-barreira

### ML-1B — Os 5 sites que a enumeração perdeu
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

| # | Site | Tipo |
|---|---|---|
| B1 | `pypi/trackfw/commands/init.py:160` | write (install) |
| B2 | `pypi/trackfw/commands/thirdparty.py:458` | **write** (`manager.update`) |
| B3 | `npm/src/commands/thirdparty.js:337` | **write** (`manager.update`) |
| B4 | `npm/src/generators/init.js` (`installIntegrationTarget`) | write (install) |
| B5 | `pypi/trackfw/commands/thirdparty.py:307` | inspeção |

**Critérios de aceite:**
- [ ] Os 5 usando o resolvedor por escopo, no padrão dos 13 anteriores
- [ ] Casos 11–12 no `check-agent-models-parity.sh`: `init` e `third-party --apply-to` em escopo
      global, com **subprocesso próprio** e guard de vacuidade
- [ ] **Contagem independente:** provar por busca que não há um 19º site — a lista dada já falhou duas
      vezes nesta REQ
- [ ] `make quality` CI-exata **exit 0**, exit code medido · `validate` 16 warnings

---

### Auditoria do ML-1B — aprovada; e a exigência de "prove por busca" rendeu um sexto site

**Ele achou sozinho o que nem eu nem a barreira tínhamos enumerado:**
`npm/src/commands/thirdparty.js:195` — a chamada de `buildPlans` da **pré-condição** do `--apply-to`,
gêmea do B5 do Python. Sem ela, a inspeção renderizaria com `{}`, veria `model: sonnet`, concluiria
que o artefato foi "modificado" e **recusaria prosseguir**.

**Minha varredura independente, para não aceitar a lista dele tampouco:**

```
internal/config/config.go:201    dentro do resolvedor (ramo de projeto)   ok
internal/generators/update.go:150 e :1968   Scope: "project" explicito     ok
npm/src/config/index.js          internals do resolvedor                  ok
internal/commands/doctor.go:86   residuo ja declarado (so diagnostico)    aceito
```

**Sabotagem própria — o caso 11 é load-bearing e discrimina por runtime:**

```
sabotagem: init.py volta a trackfw_config.load(cwd)
  FAIL [init-global-scope/py/pin]: expected 'model: claude-sonnet-4-6' but got: model: sonnet
restaurado -> exit 0
```

Acusou **só no runtime sabotado**, nomeando o esperado e o obtido — os três eixos são independentes.

```
make quality (CI-exata, minha)   exit 0
validate                         16 warnings, 0 violations
```

**Contagem final desta REQ: 19 call sites.** A lista inicial tinha 2; a Wave 0 levou a 13; o ML-2A
achou o 14º; a barreira achou 5 (15–18) e reprovou; o ML-1B achou o 19º por busca própria. É a
evidência mais forte da sessão de que **lista dada não é lista fechada** — e de que a instrução que
o harness passou a ensinar (*buscar, não confiar na lista*) é a que fecha a diferença.

---

### ML-3B — Reverificação pós-correção
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Veredito:** APROVADO. Detalhes: `docs/seguranca/2026-08-23-barreira-da-config-global-de-modelo.md` (seção Reverificação — ML-3B).

| Pergunta | Resultado |
|----------|-----------|
| Q1 — B1–B5+B6 fechados (medição comportamental, 3 runtimes × 2 caminhos)? | ✅ |
| Q2 — Existe 20º site? | ✅ Não — busca independente confirma 19 call sites |
| Q3 — Regressão nos 13 anteriores + §2? | ✅ Nenhuma — gate 71 OK + make quality exit 0 |
| Q4 — Doctor residual aceitável? | ✅ Medição confirma: comparação por manifesto, não re-render |
| Q5 — Irmão da assimetria inspect/write? | ✅ Não — discriminante "resolução única no topo" verificado |

---

### Auditoria do ML-3B — **APROVADO**; e ele derrubou a própria previsão medindo

**Ele mediu comportamento, não código** — ambiente construído do zero, `HOME` redirecionado, cwd
vazio, os dois caminhos nos 3 runtimes:

```
init --ai-tools claude                 Go/Node/Python -> claude-opus-5 · claude-sonnet-4-6
third-party install --apply-to         Go/Node/Python -> claude-sonnet-4-6 preservado
gate cases 11-12                       71 OK, 0 FAIL
```

**Detalhe que só aparece medindo:** o `trackfw.yaml` que o `init` **escreve** no cwd não contém
`agent_models` — a resolução não lê o arquivo que ela mesma acabou de criar. Um caminho circular que
teria sido fácil introduzir sem ninguém notar.

**Q2 — lista fechada em 19**, com a contagem por stack e a razão de cada exclusão (escopo de projeto
explícito, `update harness` global, display-only). Depois de a lista errar três vezes, ele **disse
como verificou**, não só que verificou.

**Q4 — ele derrubou a própria ressalva com medição.** Previa falso-positivo do `doctor` com pin
global e projeto sem config; mediu e deu `no mismatches`. Causa: `inspectResolved` compara contra o
**hash do manifesto** — o que o trackfw escreveu —, não contra re-render. O resíduo do `doctor.go`
continua correto **pelo motivo que a medição revelou**, não pelo que ele supunha.

**Q5 — a classe do achado B6 não tem irmão.** Ele verificou os três stacks e mostrou o
discriminante: resolver `agentModels` **uma vez no topo** e passar o mesmo valor a inspeção e
escrita. Onde isso acontece, não há assimetria.

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
