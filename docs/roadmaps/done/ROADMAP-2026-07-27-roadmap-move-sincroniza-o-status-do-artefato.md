---
status: done
date: 2026-07-27
req: "docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md"
squad: ""
---

# Roadmap: roadmap move sincroniza o status do artefato

> Created: 2026-07-27 | Status: done

## Context

REQ: `docs/req/REQ-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato.md`
ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

`trackfw roadmap move` reposiciona o arquivo mas não sincroniza o `status:` do frontmatter. O comando
que existe para cumprir a DoD produz um estado que o próprio validador acusa.

### O defeito se reproduziu ao criar este roadmap

Não foi preciso fixture. A sequência do protocolo de governança — `req new` → `roadmap new` →
`roadmap move wip` — bastou:

```
$ trackfw roadmap move ROADMAP-2026-07-27-roadmap-move-sincroniza-o-status-do-artefato wip
✓ moved ROADMAP-....md → docs/roadmaps/wip

$ trackfw validate
⚠  roadmap "ROADMAP-....md": folder is "wip" but status declares "backlog"
```

**O artefato que autoriza o conserto nasceu com o defeito que vai consertar.** É a evidência mais
forte disponível: o caminho oficial de governança, executado sem desvio, gera warning na hora. O
`status: wip` no frontmatter deste arquivo foi escrito à mão — exatamente o passo manual que esta REQ
elimina.

### Estado por CLI

| CLI | Reescreve frontmatter? | Testes de move |
|---|:-:|---|
| Go `internal/generators/roadmap.go:240` | **não** | posicionamento apenas |
| Node `npm/src/generators/roadmap.js:121` | **não** | **nenhum** |
| Python `pypi/trackfw/generators/roadmap.py:159` | parcial, com bug próprio | posicionamento + `assertIn("status: WIP")` |

A implementação do Python **não é o modelo a copiar**: o `re.sub` (`roadmap.py:213`) não é escopado
ao bloco de frontmatter — casa o primeiro `status:` em qualquer lugar do arquivo, inclusive no corpo
de um roadmap que documente `status:` numa tabela ou bloco de código. Este roadmap é um desses.

### Precedente a espelhar

Já existe rewriter de frontmatter nos 3 CLIs, para campos de identidade de agente:
`internal/integrations/render.go:211`, `npm/src/integrations/render.js:165`,
`pypi/trackfw/integrations/renderers.py:173`. A semântica está documentada em
`render.go:195-210` e é exatamente a que falta aqui: escopo estrito ao bloco `---`/`---`, demais
linhas preservadas byte a byte, **nunca inventa chave inexistente**, source intacta se não houver
frontmatter reconhecível.

### Decisão de formato (tomada aqui para não virar três interpretações)

O valor escrito é **minúsculo**, igual ao nome da pasta e ao template Go/Node: `status: wip`,
`status: done`. O Python hoje escreve `WIP`/`Done` capitalizado e passa no `folder_status` apenas
porque a comparação é `EqualFold` — tolerância, não contrato. Os 3 CLIs devem produzir **bytes
idênticos**; um "fix de paridade" que grava três formatos diferentes não é fix.

## Wave 1 — Sincronização de status no move (agente único)

> Dependências: nenhuma. **Wave única, agente único** — os 3 CLIs compartilham o contrato de
> comportamento e a decisão de casing. Distribuir entre agentes paralelos produziria três
> interpretações de "formato canônico"; é o caso que a regra de paralelismo carve-out.

### ML-1A — `move` sincroniza frontmatter e cabeçalho nos 3 CLIs

**Status:** done

**Files affected:**
- `internal/generators/roadmap.go` (`MoveRoadmap`, ~linha 240) + `internal/generators/roadmap_test.go`
- `npm/src/generators/roadmap.js` (`moveRoadmap`, ~linha 121) + teste novo em `npm/tests/`
- `pypi/trackfw/generators/roadmap.py` (`move_roadmap`, ~linha 159, e o `re.sub` da linha ~213)
  + `pypi/tests/test_generators_roadmap.py`

**Actions:**

1. **Escrever a função de reescrita de status**, uma por CLI, espelhando a semântica de
   `rewriteFrontmatterFields` (`internal/integrations/render.go:195-210`) — **não inventar semântica
   nova, não generalizar em helper compartilhado**:
   - escopo estrito entre o `---` da primeira linha e o `---` de fechamento;
   - demais linhas preservadas byte a byte;
   - **não cria** a chave `status:` se ela não existir (roadmap sem frontmatter sai intacto);
   - sem frontmatter reconhecível → devolve a source inalterada, sem erro.
2. **Corrigir o Python**: substituir o `re.sub` não escopado pela função do item 1. O comportamento
   atual é um bug, não uma base.
3. **Sincronizar a linha de cabeçalho** `> Created: <data> | Status: <estado>` quando ela existir,
   com o mesmo valor minúsculo. Se não existir, não criar. Motivo de não ser cosmético: os comandos
   `context` de Node e Python usam essa linha como *fallback* quando o frontmatter não tem `status`
   (`npm/src/commands/context.js:78-80`, `pypi/trackfw/commands/context.py:78-82`).
4. **Valor gravado idêntico nos 3**: minúsculo, igual ao nome do estado (`backlog`, `wip`, `blocked`,
   `done`, `abandoned`).
5. **Preservar o resto do comportamento do `move`**: log de transição, modo `by_agent`, mensagens de
   erro de estado inválido / roadmap não encontrado. Esta REQ não muda nada disso.

**Testes (P4 — obrigatório, é o que a REQ anterior tornou mandatório):**

6. **Teste que roda `validate` depois do `move`**, nos 3 CLIs: mover um roadmap `backlog → wip → done`
   e afirmar **ausência do warning `folder_status`**. Hoje nenhum teste dos 3 valida após mover —
   é por isso que o defeito sobreviveu. Afirmar o estado final, não só que o arquivo trocou de pasta.
7. **Teste de escopo do frontmatter**: roadmap cujo **corpo** contém uma linha `status: backlog`
   (dentro de bloco de código ou tabela) → só o frontmatter muda; a linha do corpo fica intacta.
   Este é o teste que reprova a implementação atual do Python.
8. **Teste de arquivo sem frontmatter**: `move` funciona, não cria chave, não corrompe o arquivo.
9. **Node.js ganha suíte de `moveRoadmap`**, hoje inexistente: os casos que Go e Python já cobrem
   (move válido, estado inválido, não encontrado) mais os itens 6–8.

**Acceptance criteria:**
- [ ] `move` sincroniza `status:` do frontmatter nos 3 CLIs, com bytes idênticos e minúsculos
- [ ] Reescrita escopada ao bloco de frontmatter — `status:` no corpo nunca é tocado (com teste)
- [ ] Chave inexistente não é criada; arquivo sem frontmatter sai intacto (com teste)
- [ ] Cabeçalho `> Created: … | Status: …` sincronizado quando existir
- [ ] Teste que roda `validate` após `move` e prova ausência de warning, nos 3 CLIs
- [ ] Node.js com suíte de `moveRoadmap` cobrindo move válido, estado inválido e não encontrado
- [ ] `go build`, `go vet`, `go test` verdes; testes Node e Python verdes
- [ ] `make quality` verde, sem variável de ambiente auxiliar

**Comandos de validação:**
```bash
make quality
go test ./internal/generators/... -run Move -v
npm test --prefix npm
python3 -m pytest pypi/tests/test_generators_roadmap.py -q
```

---

## Wave 2 — Prova no próprio repositório (orquestrador)

> Dependências: **barrier** — ML-1A concluído e auditado.

### ML-2A — Encerramento como prova empírica

**Status:** done

**Actions:**
Encerrar este roadmap com o binário recém-compilado: `trackfw roadmap move <este-roadmap> done`
seguido de `trackfw validate`, **sem editar o frontmatter à mão**. O `status:` deve virar `done`
sozinho e o validate deve ficar limpo.

É o mesmo padrão de prova da REQ anterior, onde encerrar o roadmap na própria branch provou o ML-1A.
Aqui a prova é mais direta: este arquivo nasceu com o defeito (ver Context) e deve morrer sem ele.

**Acceptance criteria:**
- [ ] `roadmap move ... done` grava `status: done` sem intervenção manual
- [ ] `trackfw validate` verde logo após, sem warning de `folder_status`
- [ ] Nenhuma edição manual de frontmatter neste ciclo

## Log de execução

**2026-07-27 — ML-1A concluído e auditado.**

`rewriteRoadmapStatus` (Go) / `rewriteRoadmapStatus` (Node) / `_rewrite_roadmap_status` (Python) —
as três estruturalmente idênticas, espelhando `rewriteFrontmatterFields`. O `re.sub` não escopado do
Python foi removido. Preservam aspas quando presentes (`status: "wip"` continua com aspas), não criam
chave ausente, devolvem source intacta sem frontmatter reconhecível.

**Prova de paridade de bytes feita pelo orquestrador** — o critério que os testes de cada CLI, rodando
isolados, não conseguem verificar. Mesmo roadmap de entrada, `move ... done` executado pelos 3
binários em diretórios separados:

| Comparação | Resultado |
|---|---|
| Go × Node | **idênticos byte a byte** |
| Go × Python | **idênticos byte a byte** |

O fixture incluía a armadilha do escopo: uma linha `` `status: backlog` `` no corpo do documento.
Ficou intacta nos três — é o cenário que reprovava a implementação anterior do Python. `validate`
após o move: sem `folder_status`.

Cobertura: Go 3 testes novos (com `ValidateUnfiltered` e controle positivo), Python 9 novos em 2
classes, e **Node ganhou sua primeira suíte de `moveRoadmap`** — `npm/tests/roadmap_move.test.js`,
10 testes. A ausência total de teste no Node era o que permitia o defeito sobreviver ali.

`make quality` verde, sem variável auxiliar.

---

## Acceptance Criteria

- [x] Wave 1 e Wave 2 concluídas
- [x] Paridade real nos 3 CLIs — mesmo comportamento **e mesmos bytes gravados**
- [x] Todo comportamento novo tem teste que prova o cenário negativo (P4)
- [x] `make quality` verde, sem variável auxiliar
- [x] Escopo negativo da REQ respeitado — os 5 achados adjacentes ficam registrados, não corrigidos

**2026-07-27 — ML-2A: a prova.**

Encerramento executado com o binário corrigido, **sem nenhuma edição manual de frontmatter**:

```
$ trackfw roadmap move ROADMAP-2026-07-27-roadmap-move-... done
✓ moved → docs/roadmaps/done

$ head -2 docs/roadmaps/done/ROADMAP-2026-07-27-...
---
status: done          ← era "wip", virou "done" sozinho
$ grep "| Status:"
> Created: 2026-07-27 | Status: done   ← cabeçalho sincronizado junto
```

Este arquivo **nasceu com o defeito** (ver Context: o `move` para `wip` gerou warning na hora) e
**morreu sem ele**. O ciclo inteiro cabe num único artefato.

**Efeito colateral útil — o gate da REQ anterior pegou um erro meu.** No mesmo `validate` apareceu:

```
✗ branch "fix/roadmap-move-sincroniza-status" has no matching roadmap in wip/ nor done/
  (found: ..., e mais 13)
```

A branch tinha sido criada com nome abreviado (`...sincroniza-status`) enquanto o roadmap é
`...sincroniza-o-status-do-artefato` — o slug não casava. O `branch_has_wip_roadmap` **reprovou
corretamente**: era exatamente o trabalho órfão que a regra existe para prevenir, e o autor do
descuido fui eu. Branch renomeada para casar o slug; `validate` verde.

Duas confirmações de bônus nessa mensagem: a regra corrigida no ML-1A da REQ anterior continua
reprovando o que deve (não afrouxou ao aceitar `done/`), e o truncamento do ML-3A está em produção
— "e mais 13" em vez da parede de 16 nomes.

## Débito confirmado durante a execução

Os 5 achados do escopo negativo da REQ seguem válidos. O ML-1A confirmou dois deles na prática:

1. **Template Python gera `status: Backlog` capitalizado** — `roadmap new` no Python produz frontmatter
   divergente de Go/Node desde o nascimento. O `move` agora normaliza para minúsculo, mas o estado
   inicial ainda diverge entre CLIs. É o item 1 do escopo negativo, agora com evidência.
2. **Prefixo de agente no log de transição** ausente no Python (item 5).
