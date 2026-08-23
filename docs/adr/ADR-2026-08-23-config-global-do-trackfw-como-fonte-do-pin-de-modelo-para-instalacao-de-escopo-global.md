---
status: Accepted
date: 2026-08-23
author: "Zeus (Arquiteto)"
---

# ADR: Config global do trackfw como fonte do pin de modelo para instalação de escopo global

> Date: 2026-08-23 | Status: Accepted

## Context

A `REQ-2026-08-21` entregou `agent_models` no `trackfw.yaml`, compondo IDs versionados por alvo. A
motivação, no texto dela, era o Claude Code: *"os 11 implementadores rodavam no alias `sonnet`… pinar
exigiu editar `~/.claude/agents/*.md` à mão"*, e o **AC6** prometia que `agents update` **reforça** o
pin.

**Medido em 2026-08-23, na máquina do KG, com a 7.2.0 instalada e depois de
`trackfw agents update --force`:**

```
~/.claude/agents/*.md   ->  model: opus (1) · model: sonnet (11)     <- alias, nao pin
~/.trackfw/             ->  identity.json, integrations-manifest.json, scripts/
                            NAO existe ~/.trackfw/trackfw.yaml
```

E, rodando o mesmo comando **de dentro do repositório do trackfw**, que tem `agent_models`
configurado:

```
$ ./bin/trackfw agents update --force --scope global --targets claude
~/.claude/agents/*.md   ->  model: claude-opus-5 (1) · model: claude-sonnet-4-6 (11)
```

**O código funciona; a config é que não foi encontrada.** `config.Load()`
(`internal/config/config.go:125`) lê `trackfw.yaml` **do diretório corrente**, sem busca ascendente e
sem fallback global. Instalação de **escopo global** rodada de qualquer diretório sem
`trackfw.yaml` cai no tier canônico — **em silêncio**.

### A consequência que torna isso um defeito, e não uma limitação

**O pin de artefato global depende de onde o usuário estava quando rodou o comando.** Rodar
`agents update` a partir de outro projeto reverte o pin de todos os agentes, sem aviso — que é
exatamente o problema que a REQ-2026-08-21 dizia ter resolvido, por outra porta. O usuário levou dois
dias para perceber, e só percebeu perguntando.

### O erro de leitura que atrasou o diagnóstico, registrado de propósito

O arquiteto leu o `ADR-2026-08-14` (roteamento de modelo **para Codex e Cursor**), viu que Claude
Code não estava nele e concluiu *"fora de escopo, não é bug"*. A REQ que governa a feature dizia o
contrário. **ADR vizinho descreve uma decisão; a REQ descreve a intenção.** Quando divergem, a REQ
prevalece — e um AC marcado `[x]` que contradiz o comportamento medido é bug ou aceite falso, nunca
"fora de escopo".

## Decision

**1. Artefato de escopo global resolve `agent_models` a partir de config global, não do cwd.**

Fonte: `~/.trackfw/trackfw.yaml` — mesmo diretório que já guarda `identity.json`,
`integrations-manifest.json` e os scripts de guard, e que já é o namespace de escopo global do
produto.

**2. `agent_models` é chave de escopo global por natureza.** O modelo em que os agentes rodam é
propriedade do **ambiente do usuário**, não do repositório: os agentes são instalados uma vez e
usados em todos os projetos. Manter a chave no `trackfw.yaml` de projeto e aplicá-la a artefato
global é a inversão que produziu o defeito.

**3. Silêncio deixa de ser resposta aceitável.** Uma instalação de escopo global que renderiza sem
`agent_models` resolvido precisa **dizer isso**, de forma visível na saída. Hoje "não configurado" e
"configurado no lugar errado" são indistinguíveis, e foi essa indistinção que escondeu o defeito por
dois dias.

**4. `trackfw agents models` passa a mostrar a origem da resolução** — qual arquivo forneceu a
versão, ou a ausência dela. O comando existe justamente para responder *"pegou?"*, e responder sem
dizer *de onde* é meia resposta.

## Consequences

**Positivas**
- O pin para de depender do diretório de invocação.
- A promessa do AC6 da REQ-2026-08-21 passa a ser verdadeira para o alvo que a motivou.
- A separação fica coerente com o resto do produto: identidade já é global; modelo passa a ser.

**Negativas e riscos aceitos**
- **Chave em dois lugares durante a transição.** Projetos que hoje têm `agent_models` no
  `trackfw.yaml` — incluindo este repositório — precisam de uma regra de precedência explícita, ou de
  migração. A REQ decide qual, e o Wave 0 dela ataca essa escolha.
- **Um arquivo novo em `~/.trackfw/`** aumenta a superfície de config global. Mitigado por começar
  com **uma** chave, não por generalizar o carregador.
- **Instalação de escopo de projeto** continua lendo o `trackfw.yaml` do projeto — não é o caso que
  quebrou, e mudá-lo seria escopo alheio.

## Alternatives Considered

**Busca ascendente pelo `trackfw.yaml` a partir do cwd.** Rejeitada: continua fazendo artefato global
depender de onde o comando foi rodado, só que com um raio maior — o mesmo defeito, mais difícil de
prever.

**Manter no projeto e documentar que se deve rodar de lá.** Rejeitada: transforma uma propriedade do
ambiente em disciplina do usuário, e a evidência é que a disciplina falha em silêncio.

**Bloquear `agents update --scope global` quando não houver config global.** Rejeitada como
comportamento padrão: quem nunca configurou modelo tem hoje um comportamento válido (tier canônico) e
não pode ser impedido de instalar agentes. O caminho é **avisar alto**, não recusar.
