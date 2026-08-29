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
- [ ] **AC4** — `roadmap new` em `by_agent` passa a permitir **escolher o namespace de destino**, em
      vez de sempre usar `Agents[0]`. O mecanismo é decisão do ADR — ver "Questão em aberto".
- [ ] **AC5** — Comportamento atual preservado como **default explícito**: sem indicação de agente,
      continua indo para `Agents[0]`, e isso passa a estar **documentado**, não implícito.
- [ ] **AC6** — `roadmap move` continua funcionando entre namespaces distintos.
- [ ] **AC7** — Paridade exata nos 3 CLIs.
- [ ] **AC8** — Gate falsificável nas duas direções: agente instalado aparece em `agents:`; e
      instalar em `flat` **não** cria a chave.
- [ ] **AC9** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0 **e o CI verde**.

## Questão em aberto — precisa de ADR antes da implementação

**Como o trackfw sabe qual agente está agindo?** As opções não são equivalentes e a escolha muda o
desenho:

- **Flag explícita** (`roadmap new --agent zeus`): simples e auditável; obriga quem chama a saber.
- **Variável de ambiente** (`TRACKFW_AGENT`): conveniente para subagente que já roda com contexto
  próprio; invisível no comando, difícil de auditar depois.
- **Frontmatter do roadmap** (`squad:` já existe): o artefato declara o dono; mas a pasta precisa ser
  decidida **antes** de o arquivo existir.
- **Inferir do ambiente do assistente**: frágil e implícito — provavelmente a pior das quatro.

Não decidir isso antes de implementar seria repetir o erro de escolher mecanismo por acidente.

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
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
