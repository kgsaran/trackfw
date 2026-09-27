# filepath.Glob devolve separador nativo; o produto escreve POSIX — combinação que faz testes passarem em macOS/Linux e reprovarem no Windows

> Data: 2026-09-26 | REQ: REQ-2026-09-09 | ML: ML-6B

## Sintoma

4 testes do vínculo REQ↔roadmap reprovaram exclusivamente no `windows-full-suites` do PR #446.
No macOS/Linux os mesmos testes passavam. O ratchet acusou-os como NEW:

```
TestNewRoadmapFromContent_BacklinkWithNonCanonicalAbsoluteREQPath
TestNewRoadmapFromContent_REQPathAlsoGetsBacklink
TestNewRoadmapFromREQ_WritesBacklinkIntoREQ
TestRunReqNew_IntegratedPathHonorsCustomDirs
```

## Causa raiz medida (não deduzida)

O CI imprimiu os dois lados da comparação para todos os 4:

```
# Exemplo do TestRunReqNew_IntegratedPathHonorsCustomDirs (CI Windows):
esperado: "docs\\planos\\backlog\\ROADMAP-2026-09-27-catalogo-de-produtos.md"
obteve:   roadmap: "docs/planos/backlog/ROADMAP-2026-09-27-catalogo-de-produtos.md"
```

**O produto está certo**: `backlogDir = cfg.RoadmapDir + "/backlog"` usa concatenação de string
com `/` literal, e `filename := fmt.Sprintf("%s/ROADMAP-...", backlogDir, ...)` também. Logo o
valor gravado no frontmatter `roadmap:` da REQ é sempre POSIX.

**O teste está errado**: `filepath.Glob("docs/roadmaps/backlog/*.md")` no Windows devolve
`docs\roadmaps\backlog\ROADMAP-xxx.md` (separador nativo do SO). Usar esse valor diretamente
como expected falha a comparação porque o produto escreve `/`.

Em macOS/Linux, `filepath.Glob` já devolve `/`, então a comparação passava nos dois lados. O
defeito estava invisível desde a Wave 1 porque o `windows-full-suites` só roda em `pull_request`
e esta branch não tinha PR até recentemente.

## Experimento natural já disponível (sem custo adicional)

No mesmo job Windows:
- `TestNewRoadmapFromREQ_BacklinkIsIdempotent` — usa string literal hardcoded `roadmap: "docs/roadmaps/backlog/` → **PASS**
- `TestNewRoadmapFromREQ_DoesNotOverwriteExistingDifferentLink` — usa hardcoded `docs/roadmaps/wip/...` → **PASS**
- Os 4 que usam `filepath.Glob` → **FAIL**

Cardinalidade de `filepath.Glob` como causa: 4/4 testes com `filepath.Glob` falham, 0/N sem ele falham.

## Por que NÃO é defeito de produto (decisão (a))

O `✓ linked` no stdout do CI para TODOS os 4 testes confirma que o backlink **foi escrito
com sucesso**. Não é caso de `linkREQToRoadmap` sendo silenciado por `pathguard.RejectAndReport`
ou por `filepath.EvalSymlinks` divergindo — o produto funcionou; só o valor gravado usa `/`.

A ADR-2026-09-04 D1 define explicitamente: "O trackfw emite `/` nos artefatos que ele mesmo
autora e cujo consumidor não é o sistema de arquivos." O frontmatter `roadmap:` é um desses
artefatos — consumido pelo `validate` (que já faz `normalizeRefSeparator`) e por humanos/git.
Gravar `\` aqui seria o defeito.

## Correção aplicada (ML-6B, 2026-09-26)

Em todos os 4 testes: substituir `roadmapRel` (ou `rms[0]`) diretamente como expected por
`filepath.ToSlash(roadmapRel)` (ou `filepath.ToSlash(rms[0])`). No macOS/Linux `filepath.ToSlash`
é no-op; no Windows converte `\` → `/`.

🔴 **Por que não `normalizeRefSeparator`**: é uma função unexported em `package generators`.
Os testes de `package commands` não podem acessá-la. `filepath.ToSlash` é o equivalente
idiomático na stdlib e não tem a ressalva de "YAML malformado com `\`" que motivou a escolha
de `strings.ReplaceAll` no produto — em testes o valor vem exclusivamente de `filepath.Glob`,
que segue a API do SO.

Adicionalmente, cada teste ganhou uma assertion de "contrato ADR" (verifica ausência de `\` no
campo `roadmap:` gravado), exercitável em macOS/Linux, que pega uma regressão de produto antes
que ela chegue ao CI do Windows.

## Armadilha de método

O fato de que "os testes passavam em macOS/Linux" é exatamente o padrão que mascara defeitos
de separador. Quando um teste usa `filepath.Glob` ou `filepath.Join` para construir o **expected**
e o produto usa string literals ou `normalizeRefSeparator` para o **actual**, os dois convergem
no POSIX e o teste não distingue "produto correto" de "produto escrevendo separador nativo".

A prova diferenciadora é: o CI de Windows (onde os dois divergem) ou um teste local que force
o separador com `filepath.FromSlash` / `t.Setenv("GOOS", ...)` — **nenhum** dos dois estava
disponível antes desta REQ ter PR.

## ADRs relacionadas

- ADR-2026-09-04 — separador POSIX em artefatos autorados
- Nota de vault: `o-gerador-perdia-a-req-dentro-do-body-e-o-cenario-24-fixa-o-bloco-de-acs-2026-09-26.md` (contexto do ML-1B)
