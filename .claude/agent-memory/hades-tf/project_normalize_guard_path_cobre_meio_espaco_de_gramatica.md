---
name: normalize-guard-path-cobre-meio-espaco-de-gramatica
description: normalizeGuardPath só troca \ por / quando há letra de unidade; um HOME de grafia MSYS produz \tmp\... que nunca casa, e o fail-open responde "guard global não instalado"
metadata:
  type: project
---

Medido em 2026-09-25 (ML-0A, censo `36036473391`, shard 4). Rótulos
`git-branch-guard-dedup/baseline-skips-project-entry` e `/double-slash-tolerance` reprovam no Windows.
Mecanismo, por construção em `internal/generators/agentfiles.go` — **produto, não harness**:

- `globalGitBranchGuardScriptPath` (`:2098`) reconstrói o caminho com `filepath.Join(home, …)`, que no
  Windows emite `\`;
- `normalizeGuardPath` só faz `strings.ReplaceAll(p, "\\", "/")` **se `hasWindowsDriveLetterPrefix`**
  — isto é, `X:/` ou `X:\` nos 3 primeiros bytes;
- um `HOME` de grafia MSYS (`/tmp/…`) **não tem letra de unidade**, então `\tmp\…` fica com `\` e
  nunca casa com o `/tmp/…` gravado no `settings.json`;
- `readGlobalHookJSON` (`:2096`) é **fail-open** por desenho — qualquer erro de leitura/parse vira
  `false`, que significa *"não instalado"*: a resposta **permissiva**.

**Why:** é o mesmo padrão que já medi duas vezes neste repositório — *o escritor grava numa gramática,
o leitor reconstrói noutra, e a comparação falha em silêncio* (ver
[[provenance-key-filepath-rel-read-mismatch]]). O sintoma imediato é benigno (hook **duplicado**, não
ausente), mas 🔴 a direção perigosa é a **correção**: relaxar o predicado para comparar só o
*basename* faria os dois rótulos passarem e transformaria o defeito em **guard ausente** (dois scripts
homônimos em diretórios diferentes virariam o mesmo, e o dedup suprimiria a entrada legítima).

**How to apply:** a falsificação desse sítio precisa das **duas direções**: caminho igual em grafias
diferentes → casa; caminho **diferente** com mesmo basename → **não** casa. E os dois braços
(`baseline-skips-project-entry`, sem `//`, e `double-slash-tolerance`, com `//`) falharem juntos é a
prova de que a divergência é de **separador**, a montante de qualquer tratamento de barra dupla — não
gaste tempo no `//`.

⚠️ Não medido: se os testes Go de `agentfiles` cobrem essa fronteira. Num runner POSIX
`filepath.Join` emite `/` e a divergência **não pode** aparecer — presunção de ausência de cobertura,
a confirmar antes de corrigir.
