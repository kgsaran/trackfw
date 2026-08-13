---
status: Done
date: 2026-08-13
author: "Zeus (Arquiteto)"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-13-fronteira-de-escrita-dos-agentes-auditores.md"
---

# REQ: Fronteira de escrita dos agentes auditores e coerente com as ferramentas concedidas

> Date: 2026-08-13 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Em 2026-08-12, Hefesto (`code-quality`) **recusou** um microlote de documentação citando a própria
definição de papel — **depois de ter executado e tido aprovados MLs equivalentes em quatro PRs**
(#156, #158, #159, #160) na mesma sessão.

Não é indisciplina do agente. **O arquivo de definição é internamente contraditório**, e ele aplicou
uma das leituras possíveis:

| # | Contradição | Linhas em `assets/agents/code-quality.md` |
|---|---|---|
| 1 | `tools:` **não concede `Write`/`Edit`** — mas o arquivo **ordena** acrescentar entrada em `docs/agents-working-context.md` | 6 × 21 |
| 2 | *"You do not modify code"* × *"**Do not edit code** without a requirement and a roadmap in `wip`"* — a segunda pressupõe que ele **pode** editar **com** eles | 30 × 27 |
| 3 | *"You do not modify code"* × *"**refuse to implement** anything without [a handoff]"* — a segunda pressupõe que ele **implementa** | 30 × 33 |

**O arquivo exige escritas que não concede.** E o instalado (`~/.claude/agents/trackfw-code-quality.md`,
renomeado para `hefesto-tf` pelo preset de identidade) carrega as mesmas contradições — a correção
tem de ser **no gerador**, não no artefato gerado, que é sobrescrito a cada `trackfw update`.

### Alcance: são três agentes, não um

`security.md` (Hades) e `ux.md` (Atena) têm **a mesma linha** e o mesmo `tools:` sem `Write`/`Edit`.

Evidência de que a ambiguidade produz **roteamento imprevisível**: na mesma sessão, **Hades escreveu
pareceres em `docs/seguranca/` sem reclamar** enquanto **Hefesto recusou** — comportamentos opostos,
sob a mesma redação.

### Decisão de KG (2026-08-13): opção A

**Auditor que escreve os próprios artefatos.** Conceder `Write`/`Edit` e delimitar a proibição a
**código de produto**, reconciliando as três contradições. A opção B (auditor puro, sem escrita
nenhuma) foi rejeitada por contrariar o próprio desenho do trackfw, que **exige** de todo agente a
entrada em `docs/agents-working-context.md`.

## Acceptance Criteria

- [ ] Os **3** assets (`code-quality`, `security`, `ux`) concedem `Write` e `Edit` em `tools:`.
- [ ] A proibição passa a ser explicitamente de **código de produto**, e o que o papel **pode**
      escrever está **afirmado**, não deixado por omissão.
- [ ] As contradições #2 e #3 reconciliadas — nenhuma frase pressupõe que o papel implementa código.
- [ ] **Paridade byte-a-byte** entre os 3 stacks (`internal/`, `npm/src/`, `pypi/trackfw/`) — 9
      arquivos, e é o que o `check-integration-assets.sh` verifica.
- [ ] Nenhum **outro** agente é alterado — os que implementam código (Apolo, Ártemis, Poseidon,
      Afrodite, Ares, Dédalo, Métis, Prometeu) ficam intocados.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** altera o `~/.claude/agents/` instalado — é artefato gerado.
- **Não** altera agentes que implementam código de produto.
- **Não** altera o preset de identidade (`internal/identity/preset.go`) nem os nomes/slugs.
- **Não** muda o modelo (`model:`) nem o `memory:` de nenhum agente.

## Linked ADR
<!-- não requer ADR: é correção de inconsistência interna, não decisão de arquitetura nova -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-13-fronteira-de-escrita-dos-agentes-auditores.md
