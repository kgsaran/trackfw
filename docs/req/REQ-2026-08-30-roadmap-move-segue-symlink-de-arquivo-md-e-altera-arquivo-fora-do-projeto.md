---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: `roadmap move` segue symlink de arquivo `.md` e altera arquivo fora do projeto

> Date: 2026-08-30 | Status: Open

## Motivation

Achado pelo `hades-tf` na barreira final da
`REQ-2026-08-29-namespace-de-agente-nao-declarado-...`, e **reproduzido por mim em modo `flat`** —
que é o modo deste repositório e o **padrão de todo projeto novo**:

```bash
printf -- '---\nstatus: PRESERVAR\n---\n# ARQUIVO DA VITIMA\n' > /fora/do/projeto/vitima.md
ln -s /fora/do/projeto/vitima.md docs/roadmaps/backlog/ROADMAP-2026-08-30-isca.md

trackfw roadmap move ROADMAP-2026-08-30-isca wip

# antes:   status: PRESERVAR
# depois:  status: wip        ← arquivo FORA do projeto, sobrescrito pelo link
```

O `move` renomeia o link e depois **reescreve o `status:` do alvo**, alterando um arquivo que nunca
esteve na árvore do projeto. Confirmado em Go e Node; no Python o conteúdo da vítima é
**desreferenciado e copiado** para um arquivo novo e rastreado — exfiltração em vez de mutação, mesma
raiz.

**Por que não está coberto pelo que acabamos de entregar.** O AC12 da REQ irmã fechou symlink no
nível de **diretório** (namespace que é link). Este é no nível de **arquivo**, dentro de um namespace
perfeitamente legítimo — e reproduz em `flat`, onde não há namespace nenhum. São superfícies
diferentes com a mesma causa: escrita que segue link.

**É a terceira fuga por symlink em três dias.** As duas anteriores:
`vault/notes/update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28.md` (`update`/`discover`) e o
AC12 acima. O padrão já não é coincidência: **este projeto escreve em caminhos derivados de conteúdo
do repositório, e nunca verificou se o destino é o que aparenta ser.**

**Cenário de dano real:** um roadmap chegado por PR de terceiro traz o symlink no diff. O mantenedor
roda `trackfw roadmap move` — operação cotidiana e aparentemente inócua — e altera um arquivo
arbitrário da própria máquina, escolhido pelo autor do PR.

## Acceptance Criteria

- [ ] **AC1** — `roadmap move` **não** escreve através de symlink de arquivo. Verificável nos 3 CLIs
      com a reprodução acima: o conteúdo da vítima fica **intacto**.
- [ ] **AC2** — A recusa é **explícita**: mensagem em stderr nomeando o caminho e dizendo que é um
      symlink. Silêncio aqui vira *"o move não fez nada e não falou"* — o mesmo erro que o ML-3E da
      REQ do `barrier` teve de corrigir no Node.
- [ ] **AC3** — Vale em `flat` **e** em `by_agent`.
- [ ] **AC4** — Vale para as três formas medidas: mutação do alvo (Go, Node) e cópia desreferenciada
      (Python). O comportamento final é idêntico nos 3: recusa.
- [ ] **AC5** — **Enumeração:** um `.md` que é symlink dentro de um namespace legítimo aparece no
      `status`/`list` ou é ignorado? Decidir e documentar. Ignorar em silêncio recria a cegueira que
      a REQ irmã acabou de fechar; enumerar e recusar a escrita é o comportamento coerente.
- [ ] **AC6** — **Varredura das outras escritas.** `move` foi o caminho reproduzido, mas o projeto
      escreve em caminhos derivados de conteúdo em vários lugares. Enumerar e cobrir todos, não só o
      `move`.
- [ ] **AC7** — Hardlink: **declarar** o comportamento. Hardlink não é distinguível por `lstat`, e o
      `hades-tf` registrou isso. Se não for coberto, tem que estar escrito.
- [ ] **AC8** — Paridade exata nos 3 CLIs; gate falsificável nas duas direções.
- [ ] **AC9** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0 **e o CI verde**.

## Negative Scope

- **Não** reabrir o AC12 da REQ irmã (symlink de **diretório**), já entregue e coberto por gate.
- **Não** tratar junction/reparse point de Windows aqui — depende de máquina Windows e está
  rastreado na frente do Windows (issue #216).
- **Não** implementar sandbox geral de escrita. O escopo é recusar escrita através de link em
  caminho derivado de conteúdo do repositório.

## Observação de método

**Terceira fuga por symlink em três dias, e a primeira que reproduz no modo padrão.** As duas
anteriores exigiam configuração específica (`update` com o arquivo presente; namespace em `by_agent`);
esta funciona em qualquer projeto recém-criado. Sugere que a correção certa não é mais um `lstat`
pontual, e sim uma **rotina única de escrita governada** que todos os caminhos usem — o mesmo remédio
que o resolvedor canônico foi para a cegueira de namespace. Decisão para o ADR.

## Linked ADR
<!-- Necessário: rotina única de escrita governada versus `lstat` por call site. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
