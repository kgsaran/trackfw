---
status: Open
date: 2026-08-29
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `agents install` não registra o agente na governança, e `roadmap new` em `by_agent` escreve sempre no primeiro da lista

> Date: 2026-08-29 | Status: Open

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
Roadmap:
