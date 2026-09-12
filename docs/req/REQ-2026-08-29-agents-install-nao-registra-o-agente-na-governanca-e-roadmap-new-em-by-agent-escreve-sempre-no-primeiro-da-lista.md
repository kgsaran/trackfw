---
status: Done
date: 2026-08-29
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-agents-install-nao-registra-e-escreve-sempre-no-primeiro.md"
---

# REQ: `agents install` não registra o agente na governança, e `roadmap new` em `by_agent` escreve sempre no primeiro da lista

> Date: 2026-08-29 | Status: Done

## Motivation

Surgiu de uma pergunta do KG durante a investigação da cegueira de namespace
(`REQ-2026-08-29-namespace-de-agente-nao-declarado-...`): *"o agente zeus foi implantado pelo
trackfw, ele não deveria ser visível mesmo sem união com o disco?"*

Investigando, três fatos medidos:

**1. `trackfw agents install` não toca o `trackfw.yaml`.** Ele instala as personas em
`.claude/agents/`, `.codex/agents/` e demais alvos. Nenhuma linha do subsistema de integrações
escreve na configuração de governança. **São dois conceitos homônimos que não conversam:** "agente"
como persona instalável, e "agente" como namespace de roadmap em `by_agent`.

**2. `roadmap new` em `by_agent` escreve sempre em `cfg.Agents[0]`** (`internal/generators/roadmap.go:101-114`):

```go
if agent == "" {
    if len(cfg.Agents) > 0 { agent = cfg.Agents[0] } else { agent = "default" }
}
```

Nunca pergunta, nunca detecta. Num projeto `by_agent`, **todo roadmap cai na pasta do primeiro
agente da lista**, independentemente de quem o criou — o que obriga a mover arquivo à mão e é a
provável origem da deriva observada no cmdb.

**3. O único ponto, nos 3 CLIs, que escreve a chave `agents:` é o `discover --init`**
(`internal/discover/discover.go:525`, `npm/src/commands/discover.js:164`,
`pypi/trackfw/commands/discover.py:274`). Ele varre o disco, encontra os diretórios existentes
**naquele instante** e escreve a lista.

**A raiz:** `agents:` é uma **fotografia tirada uma vez**, e nada a mantém sincronizada depois. O
projeto evolui, a lista não, e a ferramenta passa a confiar no retrato em vez do disco.

A `REQ-2026-08-29-namespace-de-agente-nao-declarado-...` trata a **leitura** — a união torna a deriva
inofensiva. Esta REQ trata a **escrita**: evitar que a deriva aconteça.

## Acceptance Criteria

- [ ] **AC1** — `trackfw agents install` num projeto em `by_agent` registra o agente instalado na
      chave `agents:` do `trackfw.yaml`, se ainda não estiver lá. Idempotente: instalar duas vezes
      não duplica.
- [ ] **AC2** — O registro **não** acontece em modo `flat` — ali a chave não tem função.
- [ ] **AC3** — O registro preserva ordem e formatação do resto do `trackfw.yaml`. Verificável por
      diff: só a chave `agents:` muda.
- [ ] **AC4** — `roadmap new --agent <nome>` em `by_agent` escreve o roadmap no namespace informado
      **e** registra o mesmo valor no frontmatter. Uma entrada, dois efeitos, sem possibilidade de
      divergirem.
- [ ] **AC5** — Sem `--agent`: com **um** namespace em `agents:`, usa aquele; com **vários**,
      **falha** nomeando as opções. O silêncio atual (`Agents[0]`) deixa de existir.
- [ ] **AC5b** — `--agent` com valor fora de `agents:` **funciona** (cria o namespace) e produz a
      violação de namespace não declarado da REQ irmã. Verificável nos 3 CLIs.
- [ ] **AC6** — `roadmap move` continua funcionando entre namespaces distintos.
- [ ] **AC7** — Paridade exata nos 3 CLIs.
- [ ] **AC8** — Gate falsificável nas duas direções: agente instalado aparece em `agents:`; e
      instalar em `flat` **não** cria a chave.
- [ ] **AC9** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0 **e o CI verde**.

## Mecanismo decidido (KG, 2026-08-29)

**Flag explícita `--agent`, que alimenta o frontmatter e o caminho a partir do MESMO valor.**

A REQ nascia tratando flag, variável de ambiente, frontmatter e inferência como quatro opções
concorrentes. Estava errado: `--agent` é *como se diz*, o frontmatter é *onde fica registrado*, e o
caminho é *derivado*. Uma entrada, três consequências. Tratá-los como mecanismos independentes
convida a divergência entre pasta e frontmatter — a mesma classe de defeito que a REQ irmã corrige
entre `agents:` e disco.

**A reformulação que decidiu:** a pergunta não é *"qual agente está executando o comando?"*, é
*"de quem é este trabalho?"*. Dono é propriedade da obra, não do processo que a criou. Na prática o
`roadmap new` é chamado pelo arquiteto em nome do especialista que vai executar — "quem digita"
nunca foi a informação relevante.

**Rejeitados, com motivo:**

- **Variável de ambiente** — some do comando e some da auditoria. Seis meses depois ninguém sabe por
  que aquele roadmap foi parar naquela pasta. Numa ferramenta cujo propósito é rastreabilidade, é o
  defeito mais caro possível.
- **Inferir do ambiente do assistente** — implícito, frágil, não reproduzível. Erra em silêncio.
- **Frontmatter como *entrada*** — a objeção original ("a pasta precisa ser decidida antes de o
  arquivo existir") **não** se sustenta: o `roadmap new` monta o conteúdo em memória antes de
  escrever. Mas como entrada ele é pior que a flag, porque exigiria o usuário editar o artefato
  depois de criado. Como **destino de registro**, é exatamente o certo — e é o que a decisão faz.

**Comportamento na ausência da flag — distinguir ausência de ambiguidade:**

- `agents:` com **um** namespace e sem `--agent` → usa aquele. Não há o que escolher.
- `agents:` com **vários** e sem `--agent` → **erro**, nomeando as opções disponíveis.

Falhar na ambiguidade, não na ausência. Hoje o comando escolhe `Agents[0]` em silêncio, que é
precisamente como a deriva do cmdb começou: a ferramenta decidiu e não contou a ninguém. A regra é a
mesma que adotamos no `barrier` (`ADR-2026-08-29`): controle que não reconhece **rejeita e avisa**,
em vez de adivinhar. E é precisa o bastante para não virar atrito — quem tem um agente só nunca vê o
erro.

**Encaixe com a REQ irmã:** `--agent` com valor **fora** de `agents:` cria namespace novo. Sob o
desenho da união isso é permitido — a enumeração enxerga — e gera a violação de namespace não
declarado, que instrui a registrar. As duas REQs se compõem sem contradição.

## Negative Scope

- **Não** implementar a união de leitura nem a violação de namespace não declarado — isso é
  `REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`,
  e esta REQ **depende** dela: a rede de segurança vem primeiro.
- **Não** alterar o `discover --init`, que já escreve `agents:` corretamente para o estado do
  momento.
- **Não** unificar os dois conceitos de "agente" (persona instalável × namespace de governança).
  Fazê-los conversar é o escopo; fundi-los é mudança de modelo e precisaria de ADR próprio.
- **Não** migrar `trackfw.yaml` de projeto existente automaticamente.

## Linked ADR
<!-- Necessário antes da implementação: como o trackfw sabe qual agente está agindo (AC4). -->
ADR: <!-- a criar: formaliza o mecanismo decidido acima antes da implementação -->

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-11-by-agent-req-new-e-roadmap-new-agents-install-nao-registra-e-escreve-sempre-no-primeiro.md

---

## Achados de 2026-09-11 — issue #320, **mesma causa**

Treze dias depois, o consumidor externo mediu o mesmo defeito por fora, num projeto descartável com
`agents: [alpha, beta]`, nos 3 CLIs (v7.5.1):

```
                                        Go      Node    Python
req new "x"                             alpha/  alpha/  alpha/
roadmap new --req .../beta/REQ-....md   alpha/  alpha/  alpha/   <- ignora o agente da REQ
req new ... --agent beta                erro    erro    erro
roadmap new ... --agent beta            erro    erro    OK       <- so o Python tem a flag
```

Entra **aqui**, como ML, e nao como REQ nova — `Regra Dura de Causa Raiz`. A causa e a que esta REQ
ja nomeia desde 2026-08-29 (`cfg.Agents[0]` em silencio); o que o #320 acrescenta e **superficie**:

**1. O `req new` tem o mesmo defeito, e a REQ original so falava do `roadmap new`.**
O escopo original estava **estreito demais** — nao e motivo para abrir outra REQ, e sim para corrigir
o escopo desta.

```
Go       internal/generators/req.go:30 (ponto unico de escrita, ADR-2026-09-03 D2/D4)
Node     npm/src/generators/req.js
Python   pypi/trackfw/generators/req.py:207
```

**2. `roadmap new --req <caminho>` nao herda o agente da REQ que recebe.** Com a REQ em `beta/`, o
roadmap nasce em `alpha/`. Superficie nova, mesmo mecanismo.

🔴 **O codigo ja sabe derivar agente de um caminho** — o `roadmap move` faz isso
(`internal/generators/roadmap.go:437`, `npm/src/generators/roadmap.js:267-268`). **So o `new` nao
usa.** Trazer o `new` para a forma que o `move` ja tem; **nao escrever uma terceira derivacao.**

**3. Divergencia de paridade:** o Python ja aceita `--agent` no `roadmap new`
(`pypi/trackfw/commands/roadmap.py:211`); Go e Node recusam. Merito do relator: **a recusa e
barulhenta nos tres** — nenhum aceita a flag e a ignora em silencio, que seria o pior caso.

### A decisao de AC5 ja estava tomada — e o #320 a confirma por fora

O mecanismo (**um namespace ⇒ usa; varios sem `--agent` ⇒ erro nomeando as opcoes**) foi decidido
pelo KG em 2026-08-29 e esta escrito acima, com os rejeitados e o motivo. **Nao reabrir.** O #320 e
evidencia independente de que o silencio do `Agents[0]` produz exatamente a deriva prevista.

### 🔴 Raio de alcance medido de AC5 — antes do handoff, nao depois

O erro de ambiguidade vai disparar em tudo que hoje emite `trackfw req new "title"` **sem** `--agent`.
Medido em 2026-09-11:

```
internal/generators/agentfiles.go:59       instrucoes geradas para o agente
internal/generators/claudemd.go:57-58      tabela de comandos do CLAUDE.md gerado
internal/generators/scaffold.go:263        slash-command req.md
npm/src/generators/init.js:524,691-692,899 os mesmos quatro, no Node
npm/src/push/runner.js:130 · ship/runner.js:502 · commands/branch.js:33 · commands/commit.js:37
pypi/trackfw/push/runner.py:148 · pypi/trackfw/validator.py:1967
```

Num projeto `by_agent` com 2+ agentes, **a propria orientacao que o trackfw gera passa a ensinar um
comando que falha.** Pela mesma regra de causa raiz, esses emissores entram **neste PR** — o erro
esta certo, a orientacao e que fica desatualizada.

### Por que nao tinhamos visto

🔴 **Zero testes com 2+ agentes** — derivado em 2026-09-11, nao estimado. Projeto com dois agentes e
configuracao **trivial**, nao caso exotico. Registrado em
`docs/qualidade/2026-09-11-por-que-o-consumidor-externo-acha-e-nos-nao.md`; o job
`consumer-smoke-by-agent` (PR #326) existe por isso e **nasce vermelho detectando este defeito**.

### Criterios acrescentados

- [ ] **AC10** — `req new` obedece a mesma regra de AC4/AC5 do `roadmap new`: `--agent` escreve no
      namespace informado; um namespace sem flag usa aquele; varios sem flag **falham nomeando**.
- [ ] **AC11** — `roadmap new --req <caminho>` **herda o agente da REQ**, reusando o mecanismo do
      `roadmap move`. 🔴 Derivacao nova reprova a auditoria.
- [ ] **AC12** — `--agent` existe e funciona nos **3 CLIs**, em `req new` **e** `roadmap new`. A
      divergencia so-Python some.
- [ ] **AC13** — os emissores da lista de raio de alcance ensinam `--agent` quando o projeto e
      `by_agent` com 2+ agentes. 🔴 **Derivar a lista de novo** antes de fechar; a de cima e de
      2026-09-11 e pode ter crescido.
- [ ] **AC14** — `flat` continua intacto, e `by_agent` com **um** agente **nunca** ve o erro de
      ambiguidade. Contra-braco obrigatorio: a guarda que so reprova e indistinguivel da que reprova
      sempre.
- [ ] **AC15** — 🔴 o `consumer-smoke-by-agent` passa a **VERDE** e o `continue-on-error: true` dele e
      **removido no mesmo PR**. Foi declarado temporario no PR #326; remover e parte de fechar esta
      REQ, e nao "alguem lembra depois".

### Sitio a derivar, nao presumir

O relator declarou que **nao localizou** o sitio do `req new` do Go. Nos localizamos e escrevemos o
caminho — declarar o que nao se achou e o comportamento certo; herdar a lacuna, nao.

### Absorve

`REQ-2026-09-11-by-agent-req-new-e-roadmap-new-sempre-criam-no-primeiro-agente-e-so-o-python-aceita-agent.md`
— aberta por engano em 2026-09-11 antes de eu encontrar esta, e **superseded** no mesmo dia. O
`docs/portabilidade/2026-09-05-triagem-das-reqs-abertas.md` linha 7 ja marcava esta REQ como
**AINDA VALIDA (verificado)**, citando `roadmap.go:109` e **"AC5 nao implementado"**.
