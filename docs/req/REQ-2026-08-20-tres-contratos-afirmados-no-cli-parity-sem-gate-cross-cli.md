---
status: Open
date: 2026-08-20
author: ""
adr: ""
roadmap: ""
---

# REQ: três contratos afirmados no `cli-parity.md` sem gate cross-CLI

> Date: 2026-08-20 | Status: Open

## Motivação

Primeiro consumo da lista produzida pela `REQ-2026-08-18-contrato-pinado-...`. A triagem dos 177
trechos devolveu **42 `gap` e 51 `partial`**; estes são os **três de maior risco**, escolhidos por
mim e confirmados por medição própria antes de abrir a REQ.

Uma REQ só, e não três: compartilham a natureza (contrato afirmado sem gate cross-CLI), saíram da
mesma triagem, e duas delas tocam os **mesmos scripts de gate** — separadas seriam sequenciais de
qualquer forma e pagariam três ciclos de governança.

---

### 1. Windsurf e Amazon Q — **alegação falsa de cobertura**, não lacuna silenciosa

É o caso mais grave dos quatro lotes, e o único do tipo. O documento lista os dois como cabeados
com `deny` global; os gates cobrem **seis de oito**:

```
check-agent-hooks-parity.sh:   CLIS="claude codex gemini copilot cursor kiro"
check-harness-hooks-parity.sh: CLIS="claude codex gemini cursor copilot kiro"
grep windsurf|amazonq nos dois -> 0
```

**Ausência silenciosa é ruim; afirmação falsa é pior** — quem lê o documento para decidir se pode
confiar no wiring recebe uma garantia que não existe.

**Correção de suposição minha, feita antes de abrir a REQ:** eu esperava encontrar ausência de
implementação. **Não é o caso** — os três CLIs implementam os dois alvos (Go em
`internal/generators/agentfiles.go:1197+`, mais `npm/src/generators/hooks.js` e
`pypi/trackfw/generators/hooks.py`), e há teste por stack. Falta **só** o gate cross-CLI.

E não é acrescentar dois nomes à lista: **Windsurf** usa arquivo único `.windsurf/hooks.json` com
`hooks.pre_run_command`; **Amazon Q** usa agente customizado em `.amazonq/cli-agents/*.json`. Os
formatos divergem dos outros seis — provavelmente foi por isso que ficaram de fora, e é o trabalho
real do lote.

### 2. `branch_has_wip_roadmap` aceitando `done/` — nunca exercitado

Desde a `REQ-2026-07-26` a regra aceita roadmap correspondente em `done/`, não só em `wip/`.
Medido: os cenários do `check-branch-new-parity.sh` dizem literalmente *"wip/ and done/ deliberately
left empty"*, e `check-validate-parity.sh` tem **zero** ocorrências da regra.

**O comportamento que define aquela REQ nunca foi testado entre os 3 CLIs** — e é a regra que
sustenta todo `branch`, `commit` e `ship` do projeto.

### 3. `credential_guard_hook_resolvable` — provado só em Go

O comentário do próprio Cenário 47 declara: *"prova P4 black-box de que a regra **Go** não…"*.
Go-only **por desenho**. Node e Python têm teste por runtime, nunca comparação cross-CLI.

É o controle que o `ADR-2026-08-12` aponta como o que resta mitigando o fail-open do
credential-guard. Um controle central com prova em um terço dos runtimes.

## Escopo

Para cada um: **gate comparando as três saídas reais** e cenário P4 com braço de baseline e de
detecção. Ao fechar, a anotação da seção correspondente passa de `gap`/`partial` para `gate=`.

## O que **não** é escopo

- **As outras 39 `gap` e 50 `partial`.** A lista é priorizável de propósito; fechar tudo não é meta,
  e provavelmente não vale para todas.
- Mudar comportamento de produto. Se algum gate revelar divergência real entre os CLIs — o que
  aconteceu **cinco vezes** na semana passada —, isso é **achado**, e a correção entra como microlote
  próprio, não silenciosamente.
- Afrouxar o checker de cobertura para acomodar qualquer coisa.

## Acceptance Criteria

- [x] AC1 — Windsurf e Amazon Q cobertos pelos gates de wiring, respeitando os formatos próprios.
- [x] AC2 — `branch_has_wip_roadmap` com roadmap em `done/` exercitado cross-CLI.
- [x] AC3 — `credential_guard_hook_resolvable` com prova cross-CLI, não só Go.
- [x] AC4 — Cenário P4 para cada um, com baseline e detecção.
- [x] AC5 — As anotações das seções correspondentes passam para `gate=`, e o texto que hoje **afirma
      falsamente** a cobertura de Windsurf/Amazon Q é corrigido ou passa a ser verdade.
- [x] AC6 — Divergência real encontrada é **registrada como achado**, não corrigida em silêncio.
- [ ] AC7 — `make quality` verde **e CI verde**. (local verde; **aguardando CI**)

## Riscos para quem executar

- **Windsurf e Amazon Q têm formato diferente dos outros seis.** Forçá-los no comparador estrutural
  existente provavelmente não funciona; avaliar antes de tentar.
- **Não afrouxar o gate para caber.** Se o comparador não serve, o comparador muda — não o critério.
- **A invocação CI-exata é `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.** Rodar o script direto
  não é a mesma coisa; três rodadas de CI se perderam por isso.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- nenhum; sao gates para contrato ja decidido -->

## Linked Roadmap
Roadmap: <!-- a criar -->
