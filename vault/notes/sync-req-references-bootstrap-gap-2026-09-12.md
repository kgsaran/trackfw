# `syncREQReferences` não estabelece backlink para REQs com `roadmap: ""`

**Data:** 2026-09-12  
**Contexto:** ML-1A da REQ nasce órfã — AC9 (uma noção de "vinculada").

---

## Causa raiz

`syncREQReferences` (`internal/generators/roadmap.go`, linha ~957) só atualiza REQs cujo
frontmatter `roadmap:` já aponta para o roadmap que está sendo movido:

```go
fmVal := extractFrontmatterRoadmap(string(content))
if fmVal == "" || filepath.Base(normalizeRefSeparator(fmVal)) != roadmapBasename {
    continue // sem referência ou aponta para outro roadmap
}
```

`roadmap new --from-req` (`NewRoadmapFromREQ`, linha ~292) cria o roadmap com `req:` preenchido
corretamente, mas **não grava `roadmap:` de volta na REQ**. Quando `roadmap move` é chamado logo
após, `syncREQReferences` ignora a REQ porque `roadmap: ""` → condição `fmVal == ""` → skip.

Resultado: o ciclo `roadmap new --from-req` + `roadmap move` produz um par:
- roadmap: `req: "docs/req/REQ-x.md"` (correto)
- REQ:     `roadmap: ""` (nunca atualizado)

O ciclo "cria e move" NUNCA fecha o laço de volta.

---

## Por que `Roadmap: none` não dispara `req_has_roadmap`

O fixture da Falsify Cenário 25 usa `Roadmap: none` no corpo. O predicado de `validateREQsHaveRoadmap`
(ML-1A) usa `contentHasMarkerValue` — que aceita qualquer valor não-vazio e não-HTML-comment. "none"
é não-vazio e não-comment → sem violation → exit 0 → baseline passa.

Nota: `ref_targets_exist` usa `extractRefPath` (exige sufixo `.md`) → "none" não é extraído →
nenhum `ref_targets_exist` warning para o placeholder. Os dois predicados co-existem intencionalmente:
`req_has_roadmap` verifica PRESENÇA de intenção (qualquer valor); `ref_targets_exist` verifica
EXISTÊNCIA do arquivo apontado (exige `.md`).

---

## Onde corrigir (ML-1B)

O defeito raiz pertence ao AC7 (ML-1B): `roadmap new --from-req` deve gravar `roadmap:` de volta
na REQ imediatamente ao criar o roadmap, antes de qualquer `roadmap move`. `syncREQReferences` deve
então ser conservado para atualizar o caminho (backlog→wip) em moves subsequentes.

A correção precisa ser aplicada nos 3 CLIs:
- Go: `NewRoadmapFromREQ` em `internal/generators/roadmap.go`
- Node: `newRoadmapFromREQ` em `npm/src/generators/roadmap.js`
- Python: `new_roadmap_from_req` em `pypi/trackfw/generators/roadmap.py`

---

## Nota sobre falsify

O Cenário 25 (`roadmap-req-frontmatter-path`) prova que `req:` do frontmatter do roadmap usa o
caminho completo (não o basename). Ele NÃO prova que o backlink REQ→roadmap é estabelecido — esse
é o gap que o ML-1B fecha.
