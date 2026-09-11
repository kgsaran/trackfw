---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md"
squad: "apolo-tf"
---

# Roadmap: `agents install` nao registra o agente, e `by_agent` escreve sempre no primeiro da lista

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md -->
REQ: docs/req/REQ-2026-08-29-agents-install-nao-registra-o-agente-na-governanca-e-roadmap-new-em-by-agent-escreve-sempre-no-primeiro-da-lista.md

REQ aberta em **2026-08-29**, sem roadmap ate hoje. Reconfirmada como **AINDA VALIDA (verificado)**
na triagem de 2026-09-05 (linha 7), e medida **por fora** pelo consumidor externo em 2026-09-11
(**issue #320**), que acrescentou tres superficies novas da **mesma causa**.

O mecanismo ja esta decidido pelo KG na propria REQ (secao "Mecanismo decidido"). 🔴 **Nao reabrir a
decisao** — implementar o que esta escrito.

**Dependencia satisfeita:** a REQ irma (uniao de leitura + `agent_namespace_undeclared`) esta **Done**
e a regra existe nos 3 CLIs — verificado em 2026-09-11.

## Acceptance Criteria
<!-- Detalhe por ML nas waves abaixo. Fonte de verdade: AC1-AC15 da REQ. -->
- [ ] AC1-AC3, AC8 — `agents install` registra o agente em `agents:`, so em `by_agent`, preservando o resto do arquivo
- [ ] AC4, AC5, AC5b, AC10, AC11, AC12, AC14 — resolucao de agente: flag, heranca, erro na ambiguidade, `flat` intacto
- [ ] AC6 — `roadmap move` continua funcionando entre namespaces
- [ ] AC7 — paridade exata nos 3 CLIs
- [ ] AC13 — emissores de `trackfw req new` ensinam `--agent` em `by_agent` multi-agente
- [ ] AC15 — `consumer-smoke-by-agent` VERDE e `continue-on-error` removido no mesmo PR
- [ ] AC9 — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Derivacao (bloqueia tudo)
> Dependencias: nenhuma. **Nenhuma linha de implementacao nesta wave.**

### ML-0A — Derivar o que o handoff ainda presume
**Status:** ⬜ Pendente
**Arquivos afetados:** nenhum de codigo. Entrega um relatorio.
**Acoes:**
1. **Sitio do `req new` do Go.** O relator do #320 declarou que **nao o localizou**. Localizar e
   escrever arquivo:linha. Ponto de partida: `internal/generators/req.go:30` diz ser o "ponto unico de
   decisao de caminho de ESCRITA (ADR-2026-09-03, D2/D4)" — **confirmar ou refutar** que e ele.
2. 🔴 **`by_agent` vale para `req_dir`, ou so para `roadmap_dir`?** `internal/validator/validator.go:1458`
   descreve `req_dir/<agente>/` como canonico em `by_agent`, e `pypi/trackfw/validator.py:808` usa
   `agents[0] if agents else "default"`. **Se for so roadmap por desenho, AC10 esta mal posto** e a
   wave 1 muda de forma. Responder com evidencia dos 3 runtimes.
3. **Mecanismo de derivacao do `move`.** Ler `internal/generators/roadmap.go:437` e
   `npm/src/generators/roadmap.js:267-268`, e achar o equivalente Python. Escrever a assinatura que a
   wave 1 vai **reusar**. 🔴 Se nao houver equivalente Python, dizer — nao inventar.
4. **Re-derivar a lista de emissores** de `trackfw req new`/`roadmap new` sem `--agent`. A lista da REQ
   e de 2026-09-11 e pode ter crescido. Comando escrito no relatorio.
5. **Threat model:** quem esvazia esta wave 0 sem quebrar regra escrita? E o contra-braco de cada AC:
   o que quebra quando regride para cada lado?
**Criterios de aceite:**
- [ ] Os 5 itens respondidos com evidencia (arquivo:linha ou saida de comando), nao asserção de uma linha
- [ ] 🔴 Zero linhas de implementacao neste ML
- [ ] Se o item 2 refutar AC10, o ML **para e reporta** em vez de seguir

**Gate da wave:**
```bash
# Falha fechado ate ML-0A responder. O relatorio do ML-0A substitui este comando
# pelo gate real derivado no item 5.
test -f docs/qualidade/2026-09-11-derivacao-by-agent-ml0a.md || exit 1
```

---

## Wave 1 — Resolucao de agente (3 MLs em PARALELO)
> Dependencias: **ML-0A aprovado.** Os tres tocam arvores disjuntas (`internal/`, `npm/src/`,
> `pypi/trackfw/`) e rodam juntos. 🔴 Nenhum deles toca `.github/` nem os emissores — isso e a wave 3.

> **Contrato comum aos tres.** Cada ML entrega, **no seu runtime**:
> - `--agent <nome>` em **`req new` e `roadmap new`**, alimentando caminho **e** frontmatter a partir
>   do **mesmo valor** (AC4, AC10, AC12).
> - Sem flag: **um** namespace em `agents:` ⇒ usa aquele. **Varios** ⇒ **erro nomeando as opcoes**.
>   O `Agents[0]` silencioso deixa de existir (AC5).
> - `--agent` com valor **fora** de `agents:` **funciona** e cria o namespace, produzindo a violacao
>   `agent_namespace_undeclared` da REQ irma (AC5b).
> - `roadmap new --req <caminho>` **herda o agente da REQ**, 🔴 **reusando o mecanismo do `roadmap move`
>   identificado no ML-0A**. Derivacao nova reprova a auditoria (AC11).
> - `roadmap move` entre namespaces continua funcionando (AC6).
> - **Testes, no proprio ML, nas duas direcoes** (AC14):
>   - `agents: [alpha, beta]` + `--agent beta` ⇒ artefato em `beta/`, frontmatter `beta`;
>   - `agents: [alpha, beta]` **sem** flag ⇒ **erro**, e a mensagem **nomeia `alpha` e `beta`**;
>   - `agents: [alpha]` **sem** flag ⇒ **cria em `alpha/`, sem erro** — 🔴 contra-braco: guarda que so
>     reprova e indistinguivel de guarda que reprova sempre;
>   - `flat` ⇒ comportamento **inalterado**.
> - 🔴 **Regra Dura de Reconciliacao:** para cada teste novo, uma frase no relatorio dizendo qual
>   conclusao do proprio ML ele afirma.
> - 🔴 **Nao rodar nada em background.**

### ML-1A — Go
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/req.go`, `internal/generators/roadmap.go`,
`internal/commands/req.go`, `internal/commands/roadmap.go`, e os `*_test.go` correspondentes.
**Acoes:** o contrato comum acima, no runtime Go. Sitio do `req new` conforme derivado no ML-0A.
**Criterios de aceite:**
- [ ] Contrato comum inteiro, com os 4 cenarios de teste
- [ ] `go build ./...` e `go test ./...` verdes
- [ ] 🔴 Nenhum arquivo fora de `internal/` tocado

### ML-1B — Node
**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/generators/req.js`, `npm/src/generators/roadmap.js`,
`npm/src/commands/req.js`, `npm/src/commands/roadmap.js`, e os testes em `npm/tests/`.
**Acoes:** o contrato comum acima, no runtime Node.
**Criterios de aceite:**
- [ ] Contrato comum inteiro, com os 4 cenarios de teste
- [ ] Suite do Node verde
- [ ] 🔴 Nenhum arquivo fora de `npm/` tocado

### ML-1C — Python
**Status:** ⬜ Pendente
**Arquivos afetados:** `pypi/trackfw/generators/req.py`, `pypi/trackfw/generators/roadmap.py`,
`pypi/trackfw/commands/req.py`, `pypi/trackfw/commands/roadmap.py`, e os testes em `pypi/tests/`.
**Acoes:** o contrato comum acima, no runtime Python. **Atencao:** o `--agent` do `roadmap new` **ja
existe** aqui (`pypi/trackfw/commands/roadmap.py:211`) — estender para `req new` e alinhar o
comportamento sem-flag, nao reescrever o que ja funciona.
**Criterios de aceite:**
- [ ] Contrato comum inteiro, com os 4 cenarios de teste
- [ ] Suite do Python verde
- [ ] 🔴 Nenhum arquivo fora de `pypi/` tocado

---

## Wave 2 — `agents install` registra na governanca (3 MLs em PARALELO)
> Dependencias: **wave 1 auditada.** Arvores disjuntas, mesma regra de nao-sobreposicao.

> **Contrato comum.** No proprio runtime (AC1, AC2, AC3, AC8):
> - `trackfw agents install` num projeto `by_agent` **registra o agente em `agents:`** do
>   `trackfw.yaml` se ainda nao estiver la. **Idempotente**: instalar duas vezes nao duplica.
> - Em `flat`, **nao** escreve a chave — ali ela nao tem funcao.
> - Preserva ordem e formatacao do resto do arquivo: **verificavel por diff**, so `agents:` muda.
> - 🔴 **Falsificacao nas duas direcoes** (AC8): agente instalado **aparece** em `agents:`; instalar em
>   `flat` **nao** cria a chave.
> - 🔴 Regra Dura de Reconciliacao, uma frase por teste novo. **Nada em background.**

### ML-2A — Go · ### ML-2B — Node · ### ML-2C — Python
**Status:** ⬜ Pendente (os tres)
**Arquivos afetados:** o subsistema de integracoes/agents e o escritor de config de cada runtime,
mais os testes. 🔴 Cada ML fica **dentro da sua arvore**.
**Criterios de aceite (cada um):**
- [ ] Contrato comum inteiro, com a falsificacao nas duas direcoes
- [ ] Build e suite do runtime verdes
- [ ] 🔴 Nenhum arquivo fora da propria arvore tocado

---

## Wave 3 — Consequencias (SEQUENCIAL)
> Dependencias: waves 1 e 2 auditadas. 🔴 **3A e 3B sao sequenciais entre si** — o 3B mede o efeito do 3A.

### ML-3A — Os emissores param de ensinar um comando que falha (AC13)
**Status:** ⬜ Pendente
**Arquivos afetados:** os derivados no **item 4 do ML-0A**. A lista de 2026-09-11 era:
`internal/generators/agentfiles.go:59`, `internal/generators/claudemd.go:57-58`,
`internal/generators/scaffold.go:263`, `npm/src/generators/init.js:524,691-692,899`,
`npm/src/push/runner.js:130`, `npm/src/ship/runner.js:502`, `npm/src/commands/branch.js:33`,
`npm/src/commands/commit.js:37`, `pypi/trackfw/push/runner.py:148`, `pypi/trackfw/validator.py:1967`.
🔴 **Usar a lista re-derivada, nao esta.**
**Acoes:** onde o texto orienta `trackfw req new "title"`, ensinar `--agent` quando o projeto for
`by_agent` com 2+ agentes. 🔴 **Um unico dono** — estes arquivos atravessam as tres arvores e nao
podem ser editados em paralelo com nada.
**Criterios de aceite:**
- [ ] Todo emissor da lista re-derivada coberto, ou a exclusao **justificada por escrito**
- [ ] Gate de paridade dos 3 CLIs verde
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (AC9)

### ML-3B — 🔴 `consumer-smoke-by-agent` VERDE e `continue-on-error` REMOVIDO (AC15)
**Status:** ⬜ Pendente
**Arquivos afetados:** o workflow que define `consumer-smoke-by-agent` (introduzido no PR #326).
**Acoes:**
1. Confirmar que o job passa a **VERDE** com as waves 1-3A aplicadas.
2. **Remover o `continue-on-error: true`.** Foi declarado **temporario** no PR #326.
3. Se o job exercita `req new` sem `--agent` com 2 agentes, ele agora **recebe o erro de ambiguidade
   por desenho** — ajustar o smoke para passar `--agent`, e **manter um cenario que prova o erro**.
**Criterios de aceite:**
- [ ] `continue-on-error` **ausente** do job — verificavel por `grep`
- [ ] Job verde no CI
- [ ] 🔴 Existe cenario no smoke que **reprova** se a resolucao de agente regredir
- [ ] 🔴 **Intencao declarada nao e gate.** Se o `continue-on-error` sobreviver a este ML, o ML **nao
      esta concluido** — "alguem lembra depois" e a classe que nos custou um dia inteiro

---

## Notas de governanca

- 🔴 **Esta REQ absorveu o #320 por `Regra Dura de Causa Raiz`.** A
  `REQ-2026-09-11-by-agent-req-new-...` foi aberta por engano antes de eu encontrar esta, e esta
  **superseded**. Mesma causa ⇒ mesma REQ ⇒ **mesmo PR**.
- A decisao de mecanismo e do KG, 2026-08-29, e esta na REQ. **Nao reabrir.**
