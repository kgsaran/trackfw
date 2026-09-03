---
status: Accepted
date: 2026-09-03
author: "trackfw_architect (Zeus)"
---

# ADR: Layout canônico de REQ em `by_agent`, e o invariante de que REQ não tem dimensão de estado

> Date: 2026-09-03 | Status: Accepted

## Contexto

Em todo projeto com `roadmap_namespacing: by_agent`, **as regras de validação que consomem REQ ficam
vácuas** — passam sempre, sem olhar nada. Medido na `main`:

```
mesma REQ, mesma referência quebrada, só mudando o layout:
flat      2 violações
by_agent  0            ← a regra não encontra nada para checar
```

A causa não é o resolvedor de namespace (PR #218, correto). É que **o escritor e o leitor discordam
sobre o layout**:

| | onde grava / procura |
|---|---|
| `req new` em `by_agent` | `req_dir/REQ-x.md` — **flat**, sem namespace |
| `resolveREQFiles` (validator) | `req_dir/<agente>/<estado>/*.md` — **três níveis** |
| repositório consumidor real | `req_dir/<agente>/*.md` — namespaced, **sem estado** |

**Nenhum dos três concorda**, e o leitor procura numa árvore que REQ nenhuma usa.

Reportado por `@lourivalgarciajunior` (issue #216) ao migrar um repositório consumidor de 2.12.4
para 7.3.0 e ver o `validate` sair de 0 para 18 violações — nenhuma nova, apenas nunca olhadas.

**É a terceira ocorrência do mesmo padrão em poucos dias**: gerador e verificador discordando de um
contrato. Antes foram o cabeçalho de aceite (inglês contra português) e o vocabulário de status
(`pending` contra emoji), ambos corrigidos no PR #217.

## Decisão

### D1 — Invariante: REQ **não** tem dimensão de estado

`backlog`/`analyzing`/`wip`/`blocked`/`done`/`abandoned` são conceito de **roadmap**. REQ tem
`status` no frontmatter (`Open`/`Done`), não pasta de estado.

**Esta é a parte que importa mais que a escolha de caminho.** O defeito nasceu de um leitor
*supondo* um nível que não existe. Enquanto o invariante não estiver escrito, o próximo leitor
inventa o mesmo nível de novo.

### D2 — Layout canônico de escrita em `by_agent`: `req_dir/<agente>/*.md`

É o que o campo já usa e a única forma compatível com D1. Em `flat`, permanece `req_dir/*.md`.

### D3 — Leitura é **união**, escrita é **única**

O resolvedor de leitura aceita **todos** os layouts, concatenados:

```
req_dir/*.md                        flat legado
req_dir/<estado>/*.md               por-estado (legado, apesar de D1)
req_dir/<agente>/*.md               ← CANÔNICO em by_agent — HOJE AUSENTE do resolvedor
req_dir/<agente>/<estado>/*.md      legado
```

🔴 **`req new` escolhe um caminho para escrever; o leitor nunca rejeita os outros.** "Layout
canônico" **não** significa "recusar os demais" — significa "é aqui que passamos a gravar". Nenhum
arquivo de ninguém é migrado.

### D4 — Um único ponto decide o caminho, e os dois lados o consomem

O par escritor/leitor não pode ter duas noções de layout. **Isto é invariante, não nota de
implementação** — é exatamente a divergência que produziu as três ocorrências.

## Consequências

**Custo maior que o estimado.** A leitura de backlog registrou que "a lógica correta já existe — é
fiação, não lógica nova", apontando `internal/generators/req.go:119-152` (`listREQFiles`).
**Verificado, e é parcialmente falso:** `listREQFiles` cobre flat, por-estado e `<agente>/<estado>`,
mas **não cobre `<agente>/*.md`** — justamente o layout de D2. Falta um caso, não só a fiação. A
correção é maior do que a estimativa otimista e menor do que a severidade sugere.

**Regra dura de paridade aplica-se integralmente.** É código de produto nos 3 CLIs — `internal/`,
`npm/src/` e `pypi/trackfw/` —, sem a exceção de infra de gate usada nos últimos PRs.

**Compatibilidade por construção.** Sendo a leitura uma união, REQ existente em qualquer layout
continua encontrada. D3 é o que torna a migração desnecessária.

## Fora desta decisão

🔴 **O template do `req new` gera `ADR:` e `Roadmap:` no corpo, e o validator lê só o frontmatter**
(AC7 da REQ). Decidir se o validator passa a ler os dois, ou se o corpo vira prosa declarada, é
**mudança de contrato que atinge toda REQ já existente em todo repositório adotante** — inclusive as
181 deste. Não cabe de carona: **vira REQ própria**. Deixar junto transformaria uma correção de
resolvedor numa migração.

**Não** introduzir partição por estado em REQ — D1 é o oposto disso.
**Não** migrar `req_dir` de projeto nenhum automaticamente.
**Não** reabrir o resolvedor de namespace do PR #218, que está correto.

## Verificação exigida de quem implementar

A forma mensurável de "as regras não ficam mais vácuas": **o número de regras que enxergam zero REQs
num projeto `by_agent` tem de ser zero**. As regras afetadas, já enumeradas pela auditoria de
2026-09-02 — confirmar a lista, não rederivá-la:

```
ref_targets_exist · req_has_adr · req_has_roadmap
blocked_by_draft_adr · adr_accepted_when_req_done · traceid
```

🔴 **E o teste de ciclo fechado por artefato** — `new` → `validate` enxerga —, nos 3 CLIs, em `flat`
**e** em `by_agent`. É a única verificação que impede a **quarta** ocorrência do padrão, e por isso
tem de ser microlote próprio com barreira própria, não caixinha dentro do ML de correção.
