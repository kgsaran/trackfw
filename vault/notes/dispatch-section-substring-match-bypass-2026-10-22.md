# extractDispatchSection — exact heading match closes substring-bypass and inert-content vectors

> Data: 2026-10-22 | REQ: REQ-2026-09-26 | ML: 1B (corretivo da Wave 2) + ML-1C (corretivo da 2ª rodada)

## Causa raiz

`extractDispatchSection` em `render_test.go` usava `strings.Index(body, "## Dispatch contract")`.
Essa é uma busca de **substring**, não de linha exata. Quatro classes de bypass eram possíveis:

1. **Heading com sufixo ou prefixo** (`## Old Dispatch contract`, `## Dispatch contract (legacy)`) — a substring casa e retorna a seção original com as frases.
2. **H3 heading** (`### Dispatch contract`) — `##` é substring de `###` no offset 1, e o cabeçalho H3 bate.
3. **Segundo heading idêntico** — `strings.Index` encontra o PRIMEIRO (com as frases); o segundo (com instrução de substituição) nunca é inspecionado.
4. **Conteúdo inativo** — parágrafo em comentário HTML multilinha ou bloco de código: sem stripping, `strings.Contains` encontra as frases dentro do conteúdo inativo.

## Fix implementado

Duas alterações em conjunto:

### 1 — `extractDispatchSection`: needle exato + unicidade

```go
const needle = "\n## Dispatch contract\n"
full := "\n" + body
count := strings.Count(full, needle)
if count != 1 {
    return ""   // 0 = não encontrado; 2+ = duplicata
}
```

- Needle com `\n` antes e depois: exige linha exata. `## Old Dispatch contract`, `### Dispatch contract` e `## Dispatch contract (legacy)` não casam.
- `strings.Count == 1`: exige unicidade. Segundo heading idêntico → `count == 2` → retorna `""`.

### 2 — `checkPhrases`: recusa marcadores de conteúdo inativo + exige col 0

```go
if strings.Contains(section, "```") { return "fenced-block marker" }
if strings.Contains(section, "~~~") { return "fenced-block marker (~~~)" }
if strings.Contains(section, "<!--") { return "HTML comment marker" }
// phrase1 deve iniciar na coluna 0
foundAtCol0 := false
for _, line := range strings.Split(section, "\n") {
    if strings.HasPrefix(line, phrase1) { foundAtCol0 = true; break }
}
if !foundAtCol0 { return "phrase1 not at column 0" }
```

Essas verificações garantem que as frases obrigatórias não estejam apenas presentes como substring,
mas em texto ativo e não indentado.

## Por que este fix foi escolhido em vez do parser proposto pelo red team

O red team (Hades) propôs `stripInactiveMarkdown` com `parseFenceRun` (CommonMark §4.5: char + run + whitespace-only closer), o que exigiria ~130 linhas de lógica de parser. A regra implementada aqui — needle exato + count == 1 + recusa de marcadores na seção — fecha os mesmos vetores com muito menos código:

- Sufixo/H3: fechado pelo needle exato (`\n## Dispatch contract\n`).
- Duplicata: fechado por `count != 1`.
- HTML comment: fechado por `strings.Contains(section, "<!--")`.
- Fenced block: fechado por `strings.Contains(section, "```")` e `"~~~"`.
- Indentação: fechado pela verificação de coluna 0.

O parser completo de CommonMark seria necessário se o teste precisasse distinguir fences abertas em outra seção que englobassem a seção inteira — esse vetor é um resíduo declarado (ver abaixo).

## D-7 — HTML block tag não detectado (2ª rodada, 2026-10-22)

A 2ª rodada do red team (Hades) descobriu que blocos HTML na forma `<div hidden>`, `<pre>` e `<script type="text/x-deprecated">` passavam pelo `checkPhrases` sem rejeição: nenhum dos marcadores existentes (`<!--`, ` ``` `, `~~~`) detecta uma tag HTML de abertura.

### Fix implementado (ML-1C)

Adicionado ao loop de linhas em `checkPhrases`, após o check de `<!--`:

```go
for _, line := range strings.Split(section, "\n") {
    trimmed := strings.TrimLeft(line, " ")
    if strings.HasPrefix(trimmed, "<") && len(line)-len(trimmed) <= 3 {
        return "## Dispatch contract section contains HTML block tag at start of line (D-7)"
    }
}
```

**Lógica:** CommonMark define HTML block quando uma linha começa com `<` após no máximo 3 espaços de recuo (spec §4.6). O check usa exatamente esse critério. O `<` no meio da linha (ex.: `` `<slug>-tf` ``) não é afetado porque `strings.HasPrefix` opera sobre a linha após TrimLeft.

Novo braço `negative/html-block-wrapped`: fonte com o parágrafo entre `<div hidden>\n` e `\n</div>`. Vacuidade garantida por `bytes.Equal`. O braço afirma que `checkPhrases` rejeita a saída renderizada de qualquer representação quando o parágrafo de proibição está dentro de um bloco HTML.

## Mordida verificada (133 PASS baseline → 145 PASS pós-ML-1C)

| Braço | O que afirma | Resultado |
|---|---|---|
| `negative/wrong-section` | `extractDispatchSection` retorna seção sem o parágrafo quando ele foi movido para outra seção | PASS (arm FAIL quando extractDispatchSection retorna body) |
| `negative/suffix-heading` | Needle exato rejeita `## Old Dispatch contract` | PASS |
| `negative/h3-heading` | Needle exato rejeita `### Dispatch contract` | PASS |
| `negative/duplicate-heading` | `count == 2` → seção não encontrada | PASS |
| `negative/comment-wrapped` | Detecção de `<!--` rejeita parágrafo em comentário HTML | PASS |
| `negative/fence-wrapped` | Detecção de ` ``` ` rejeita parágrafo em bloco de código | PASS |
| `negative/indented` | Verificação col 0 rejeita parágrafo indentado com 4 espaços | PASS |
| `negative/html-block-wrapped` | Detecção de linha começando com `<` rejeita parágrafo em `<div hidden>` (D-7) | PASS |

Provas de mordida (bite):
- Bite (a) — `extractDispatchSection = return body`: wrong-section, suffix-heading, h3-heading, duplicate-heading FAIL ×12; comment-wrapped, fence-wrapped, indented PASS.
- Bite (b) — recusa de fence/comment/col0 removida: comment-wrapped, fence-wrapped, indented FAIL ×12; demais PASS.
- Bite (c, ML-1C) — check de HTML tag removido: html-block-wrapped FAIL ×12; demais PASS.

## Resíduos declarados

**M3** — uma cláusula de exceção na MESMA seção que preserve as frases (ex.: `"Exception: this rule does not apply when..."`) passa. O teste não exige que o conteúdo da seção seja apenas o parágrafo proibido. A barreira que cobre esse vetor é a auditoria do diff pelo arquiteto. O que esvazia a proibição em TODOS os vetores que restam é um texto ativo novo de exceção (M3); o teste é uma tripwire de texto, e a barreira real contra o M3 é a auditoria do diff pelo arquiteto.

**D-8 — Fence ou comentário HTML externo** — uma fence (ou comentário HTML) que abre em uma seção anterior e fecha em uma seção posterior à `## Dispatch contract`, envolvendo a seção inteira, não é detectada: a verificação de ` ``` ` / `<!--` é somente sobre o conteúdo da seção já extraída por `extractDispatchSection`. O parser `parseFenceRun` do red team fecharia esse vetor; decidiu-se não implementá-lo por ser de complexidade desproporcional ao risco real (exigiria editar o asset de forma que o próprio `extractDispatchSection` não detectaria o cabeçalho — o que tornaria o asset ilegível para o próprio CLI).

## Regra a derivar

Ao testar presença de seção markdown: usar needle com `\n` antes e depois + `strings.Count == 1` para garantir localização exata e unicidade. Combinar com recusa explícita de marcadores de conteúdo inativo (fences e HTML comments) e verificação de coluna 0 para as frases críticas. `strings.Index` sozinho é insuficiente para qualquer dessas garantias.
