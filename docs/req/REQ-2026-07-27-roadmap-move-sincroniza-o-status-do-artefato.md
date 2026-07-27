---
status: Done
date: 2026-07-27
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md"
---

# REQ: roadmap move sincroniza o status do artefato

> Date: 2026-07-27 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

`trackfw roadmap move <nome> <estado>` move o arquivo de pasta mas **não sincroniza o `status:` do
frontmatter**. O resultado é imediato e visível:

```
$ trackfw roadmap move ROADMAP-... done
✓ moved ROADMAP-....md → docs/roadmaps/done

$ trackfw validate
⚠  roadmap "ROADMAP-....md": folder is "done" but status declares "wip"
```

O comando que existe para cumprir a Definition of Done produz um estado que o próprio validador
acusa. Quem encerra um roadmap pelo caminho oficial recebe um warning e precisa editar o frontmatter
à mão — sem que nada avise que esse segundo passo existe.

É o **mesmo formato do defeito D4** da REQ-2026-07-26 (`branch_has_wip_roadmap` punindo a DoD): a
ferramenta pune quem segue o processo que ela mesma prega. Naquele caso o agente que cumpria a DoD
via o gate reprovar; aqui vê o validador reclamar. A saída natural, nos dois, é concluir que o
processo está errado — ou parar de mover roadmaps.

Descoberto ao encerrar o roadmap da própria REQ-2026-07-26. Documentado em
`vault/notes/roadmap-move-nao-atualiza-frontmatter-2026-07-27.md` e registrado como débito nº 5
daquele roadmap.

### Estado por CLI (levantado antes de abrir a REQ)

| CLI | Reescreve frontmatter? | Observação |
|---|:-:|---|
| Go (`internal/generators/roadmap.go:240`) | **não** | `os.Rename` + log, nada mais |
| Node.js (`npm/src/generators/roadmap.js:121`) | **não** | idem; **e não tem nenhum teste de `moveRoadmap`** |
| Python (`pypi/trackfw/generators/roadmap.py:159`) | parcial | `re.sub` **não escopado ao frontmatter** — casa o primeiro `status:` em qualquer lugar do arquivo, inclusive no corpo |

A implementação parcial do Python é um defeito próprio, não um modelo: além do regex solto, é no-op
silencioso quando a chave não existe e escreve label capitalizado (`WIP`), que só passa no
`folder_status` porque a comparação é case-insensitive — não por ser o formato correto.

## Acceptance Criteria

- [x] `roadmap move <nome> <estado>` sincroniza `status:` do frontmatter nos **3 CLIs**
- [x] A reescrita é **escopada ao bloco de frontmatter** (entre o `---` inicial e o `---` de
      fechamento); um `status:` no corpo do documento nunca é tocado
- [x] A reescrita **não inventa chave** que não existia, e devolve o arquivo intacto se não houver
      frontmatter reconhecível — mesma semântica de `rewriteFrontmatterFields`
- [x] A linha de cabeçalho `> Created: … | Status: …` é sincronizada junto, quando existir
- [x] O valor escrito é **idêntico byte a byte nos 3 CLIs** (casing canônico definido no roadmap)
- [x] Teste que roda `validate` **depois** do `move` e prova ausência de warning — nos 3 CLIs (P4)
- [x] Node.js ganha cobertura de `moveRoadmap`, hoje inexistente
- [x] `make quality` verde, sem variável de ambiente auxiliar

## Escopo negativo — achados adjacentes que NÃO entram

Levantados na investigação e conscientemente deixados de fora. Cada um é candidato a REQ própria;
registrá-los aqui é o que impede que virem dívida esquecida.

1. **Templates de roadmap divergem entre Python e Go/Node** — Python usa `name`/`title`/`created`/
   `author`, os outros usam `date`/`req`/`squad`; header em PT-BR com emoji; não há equivalente
   Python de `NewRoadmapFromREQ`. É a maior divergência de paridade do repositório hoje, mas mexer em
   `roadmap new` afeta todo roadmap existente e não tem relação com o `move`.
2. **Estado `analyzing` não é movível por nenhum CLI**, apesar de existir no scaffold, ser aceito
   pelo `folder_status` e ser citado nas instruções que o próprio trackfw gera.
3. **`parse_frontmatter` do Python não remove aspas** → `status: "wip"` gera warning em Python e
   passa em Go/Node.
4. **`findRoadmap` do Go retorna o primeiro match sem erro de ambiguidade** (Node e Python erram), e
   não filtra `.md`.
5. **Log de transição em modo `by_agent`**: Go e Node prefixam o agente no basename, Python não.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` (P1–P4; esta REQ é
governada por ele, em especial P4 — a correção só é aceita com teste que prova o cenário negativo)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md`
