---
status: wip
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-move-do-roadmap-sincroniza-status-sem-cobertura-de-teste.md"
---

# ROADMAP: cobrir a sincronia de status do move, com falsificação

> Date: 2026-09-02 | Status: wip

REQ: docs/req/REQ-2026-09-02-move-do-roadmap-sincroniza-status-sem-cobertura-de-teste.md
ADR:

## ML-1A — Sete testes sobre `MoveRoadmap`

**Status:** ✅ Concluído

`internal/generators/roadmap_move_test.go`, 161 linhas, em modo `flat`:

| Teste | O que fixa |
|---|---|
| `SincronizaStatusDoFrontmatter` | `done/` não fica declarando `status: wip` |
| `SincronizaLinhaHumana` | a linha `> … \| Status: …` acompanha a pasta |
| `LinhaHumanaComEmoji` | formato herdado — o trecho inteiro após o marcador é substituído |
| `SemFrontmatterNaoModifica` | sai **byte a byte** idêntico, inclusive com `status:` no corpo |
| `FrontmatterSemStatusNaoGanhaCampo` | não inventamos a chave |
| `StatusNoCorpoNaoEhTocado` | só a chave dentro do bloco de frontmatter é reescrita |
| `SemLinhaHumanaNaoCria` | a linha nunca é inventada |

Nenhuma linha de produção é tocada.

## ML-1B — Falsificação

**Status:** ✅ Concluído

**(a) Direção verde** — contra `main` (1054181), os 7 passam:

```
ok  github.com/kgsaran/trackfw/internal/generators
--- PASS: TestMoveRoadmap_SincronizaStatusDoFrontmatter
--- PASS: TestMoveRoadmap_SemFrontmatterNaoModifica
--- PASS: TestMoveRoadmap_FrontmatterSemStatusNaoGanhaCampo
--- PASS: TestMoveRoadmap_StatusNoCorpoNaoEhTocado
--- PASS: TestMoveRoadmap_SincronizaLinhaHumana
--- PASS: TestMoveRoadmap_LinhaHumanaComEmoji
--- PASS: TestMoveRoadmap_SemLinhaHumanaNaoCria
```

**(b) Controle** — desligando a chamada a `rewriteRoadmapStatus` dentro de `MoveRoadmap` e mantendo
todo o resto, **5 dos 7 acendem**:

```
--- FAIL: TestMoveRoadmap_SincronizaStatusDoFrontmatter
--- FAIL: TestMoveRoadmap_StatusNoCorpoNaoEhTocado
--- FAIL: TestMoveRoadmap_SincronizaLinhaHumana
--- FAIL: TestMoveRoadmap_LinhaHumanaComEmoji
--- FAIL: TestMoveRoadmap_SemLinhaHumanaNaoCria
```

Os **2 que continuam verdes** são `SemFrontmatterNaoModifica` e `FrontmatterSemStatusNaoGanhaCampo`
— e isso está certo: os dois afirmam que **nada muda**, e com a sincronia desligada nada muda mesmo.
São as guardas da direção negativa, e por construção não podem acender neste controle. Elas acendem
na direção oposta, contra uma substituição global.

## ML-1C — Estado do pacote, medido e declarado

**Status:** ✅ Concluído

`go test ./internal/generators/` já está **vermelho em `main` no Windows**, antes desta mudança:
`TestMoveRoadmap_AnalyzingFlat` e `_AnalyzingByAgent` reprovam. Medido em worktree limpo do
`upstream/main`, **sem** o arquivo desta PR, para não atribuir a ela o que já estava lá.

Causa aparente: `findRoadmap` monta o diretório concatenando com `"/"` e devolve com
`filepath.Join`, que no Windows normaliza para `\`; o teste compara com um literal de barras
normais. Não medi vazamento de `\` para dentro de artefato — `roadmap move` grava caminho portável
no arquivo. Fica como achado separado, não desta PR.
