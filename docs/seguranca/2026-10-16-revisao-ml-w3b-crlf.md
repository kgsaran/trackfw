# Revisão de qualidade — ML-W3b (CRLF em normalizeMarkdown)

> Data: 2026-10-16 | Agente: hefesto-tf
> Branch: `fix/fechar-os-grupos-de-falha-de-windows-ml-w3-infer`
> REQ: REQ-2026-09-03
> Escopo: `internal/integrations/render.go`, `internal/integrations/render_test.go`,
>         `internal/generators/update_test.go`, `.github/windows-known-failures.json`

---

## Veredito

**APROVA COM RESSALVAS** — Q-3 é ajuste não-bloqueante; nenhuma ressalva bloqueia o PR.

---

## Q-1 — Sítios remanescentes de mesma causa

**Resultado: nenhum sítio remanescente.**

O único embed que carrega markdown sensível a CRLF é `//go:embed assets` em
`internal/integrations/catalog.go` (line 15). Os bytes brutos fluem por `ReadAsset` e chegam
aos consumidores em dois caminhos:

### Caminho 1 — `plan.go:65` → `Render()`

Todos os branches de `Render` agora estão cobertos:

| Branch | Função que normaliza | Linha |
|--------|---------------------|-------|
| `KindSkills` | `normalizeMarkdown` (fix deste ML) | render.go:94 |
| Rota A (`opencode-agent`, `cli-agent-json`, `agent-directory`, `custom-agent-toml`) | `markdownParts` (entry normalization já existia) | render.go:99 |
| Rota B default, sem identidade | `normalizeMarkdown` (fix deste ML) | render.go:233 |
| Rota B default, cursor/claude com reescrita de modelo | `rewriteFrontmatterModelLine` (já normalizava, render.go:599) → `normalizeMarkdown` | render.go:233–238 |
| Rota B default, com identidade | `insertBodyPrefix` → `rewriteFrontmatterFields` → `rewriteSignatureLine` (todos normalizam) → `normalizeMarkdown` | render.go:234–238 |

### Caminho 2 — `commands/agents_models.go:135` → `AgentTier(source)`

`AgentTier` chama `markdownParts` (render.go:82), que aplica `NormalizeCRLF` na entrada
(render.go:258). Coberto.

### Outros embeds

- `internal/i18n/i18n.go` — JSON de locale: parsing JSON é indiferente a CRLF.
- `internal/serve/serve.go` — assets estáticos (HTML/CSS/JS): não são parseados como frontmatter.

### Checagem em `internal/generators`

`generators/roadmap.go:700,704` e `generators/req.go:283,286` — normalizam via
`integrations.NormalizeCRLF` na própria entrada, antes de qualquer reescrita. Inalterados
e corretos.

`generators/update.go` só usa `integrations.BuildPlans` e `manager.Update`, que consomem
output de `Render` — já normalizado.

**Conclusão Q-1:** Nenhum sítio adicional encontrado. A Regra Dura de Causa Raiz está satisfeita
para esta causa.

---

## Q-2 — Coerência com a ADR (D1: normalizar na fronteira de entrada)

A ADR (D1) diz: "normalizar na fronteira de **entrada**, não na de saída". O comentário de
`normalizeMarkdown` a nomeia "output normalizer". Há uma tensão aparente.

**Não é violação de D1.** O raciocínio:

Para os dois paths que chegam a `normalizeMarkdown` sem passar por `markdownParts` — `KindSkills`
(render.go:94) e Rota B no-identity (render.go:233) — não existe "parser" a entrar. O source passa
diretamente para `normalizeMarkdown` sem nenhuma decomposição de frontmatter. A "fronteira de
entrada" desses paths É `normalizeMarkdown`; normalizar ali É normalizar na entrada.

A alternativa correta seria `ReadAsset` (normalizaria antes de chegar a qualquer consumidor). Mas:

1. A ADR-D4 proíbe declarar `eol` nos assets justamente para não mascarar o defeito. Normalizar em
   `ReadAsset` teria o mesmo efeito: os bytes da embed chegariam sempre LF, tornando impossível
   testar a tolerância CRLF sem simular via `ReplaceAll`.
2. As funções de frontmatter (`markdownParts` et al.) chamam `NormalizeCRLF` nas suas próprias
   entradas — que é a D3 do ADR. Remover essas chamadas para mover tudo para `ReadAsset` seria
   um refator de maior superfície sem benefício líquido.
3. A implementação atual aplica `NormalizeCRLF` na primeira linha de `normalizeMarkdown` — antes
   de qualquer processamento — portanto satisfaz D1 semanticamente (normaliza antes de usar).

**Recomendação:** manter a abordagem atual. Não há inconsistência de ADR que deva ser corrigida
antes do PR.

---

## Q-3 — Densidade do comentário de `normalizeMarkdown`

O comentário tem 13 linhas. Funções vizinhas de complexidade similar têm 4–7 linhas
(`insertBodyPrefix`: 6 linhas; `rewriteSignatureLine`: 8 linhas, incluindo 2 de header).

O comentário atual lista 4 nomes de teste e descreve 3 modos de falha — informação útil para
rastreabilidade, mas excessiva para um comentário de função. Versão enxuta equivalente:

```go
// normalizeMarkdown is the output normalizer: TrimSpace + single trailing LF.
// Calls NormalizeCRLF first so that a CRLF-embed on Windows (go:embed, no
// eol=lf on assets per ADR D4) emits LF-only output — covering KindSkills
// and the no-identity default branch, which bypass markdownParts et al.
```

**Q-3 é não-bloqueante.** O comentário atual não introduz incorreção; é mais longo que o
padrão do arquivo. Ajuste recomendado mas não obrigatório para este PR.

---

## Q-4 — Reconciliação de testes

### `TestNormalizeMarkdown_StripsCRLF` (novo)

Conclusão do ML afirmada: `normalizeMarkdown` com source CRLF deve emitir output idêntico ao
de source LF e não pode conter `\r`.

Reprova sem o fix: sim — o código anterior fazia `TrimSpace(string(source)) + "\n"`, deixando
`\r\n` internos intactos. O fixture `"---\r\nname: test\r\n---\r\n"` produzia `\r` no output.

Mede no macOS: sim — input é hardcoded; independe da plataforma.

### `TestRenderWithoutIdentityMatchesFrozenGoldens_CRLFSource` (novo)

Conclusão do ML afirmada: `Render` com source CRLF (Rota B no-identity, `subagent/architect`)
produz output byte-idêntico ao golden LF.

Reprova sem o fix: sim — `normalizeMarkdown` retornava CRLF no output, golden é LF (fixado por
`-text` em `.gitattributes`); `got ≠ want`.

Mede no macOS: sim — o embed no macOS chega LF, mas o teste injeta CRLF via `ReplaceAll` antes
de chamar `Render`. O `NormalizeCRLF(embedSource)` antes da injeção é um guarda de instrumento
(evita double-CR no Windows), não afeta o que o teste mede.

### `TestRenderOpenCodeAgent_CRLFSourceMatchesLF` (instrumento modificado)

O path "opencode-agent" passa por `markdownParts` (Rota A), que já normalizava CRLF. Este teste
**passava no macOS antes do fix** e continua passando. A modificação (`NormalizeCRLF(embedSource)`)
previne double-CR no Windows quando o embed chega CRLF.

Mede algo no macOS pós-fix: sim — continua verificando que CRLF injetado externamente não vaza
para o output de `Render` no path opencode-agent. É um teste de regressão válido para a
normalização via `markdownParts`.

A modificação do instrumento não introduz vacuidade: o `crlfSource` ainda é diferente de
`lfSource` (todos os `\n` são CRLF); a asserção `lfOut == crlfOut` ainda discrimina.

### `TestRenderSubagentRouteInjectsIdentity_CRLFSourceMatchesLF` (instrumento modificado)

Análise idêntica à de `TestRenderOpenCodeAgent_CRLFSourceMatchesLF` para o path Rota B com
identidade (`rewriteFrontmatterFields` et al. normalizam). Mede regressão válida no macOS.

### `TestUpdateMigratesKnownCodexAndPreservesUnknown` (instrumento alterado em `update_test.go`)

Conclusão do ML afirmada: a presença de `backendPath` no manifest e a ausência de
`frontendPath` devem ser verificadas por lookup de chave no struct decodificado
(`integrations.LoadManifest`), não por `strings.Contains` no JSON bruto.

Reprova sem o fix: sim — `filepath.Join` gera `\` no Windows, que `json.Marshal` serializa
como `\\`. `strings.Contains(manifest, backendPath)` falha quando `backendPath = "agents\zeus.md"`
(barra simples) e o JSON contém `"agents\\zeus.md"` (barra dupla).

Mede no macOS: sim — a mudança de `strings.Contains` para key-lookup no `Artifacts` map é
correta em qualquer plataforma; não há risco de vacuidade.

**Reconciliação geral:** todos os testes afirmam exatamente o que o ML entregou. Nenhum teste
novo é vacuoso ou contradiz as medições do ML-W3a.

---

## Q-5 — Formato dos 5 novos `removed[]`

As 5 entradas seguem o formato das entradas corrigidas já existentes em `removed[]`
(ex.: `TestSave_WritesAtomicallyWithPermissions`):

- Campos obrigatórios presentes: `name`, `runtime`, `class`, `removal_note: "corrected"`
- Campo `reason` preservado textualmente da entrada em `known[]`
- Campo `correction_note` adicionado (additive — presente nos outros `corrected` recentes)
- Nenhum campo ausente ou com tipo errado

Validação JSON: `python3 -c "import json; json.load(open('.github/windows-known-failures.json'))"` — PASS.

**Q-5: PASS.**

---

## Evidências de execução

- `git diff origin/main...HEAD -- internal/ .github/windows-known-failures.json` revisado na íntegra
- Todos os caminhos de `Render()` mapeados contra chamadas a `NormalizeCRLF` / `normalizeMarkdown`
- ADR `ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-fronteira-de-entrada.md` lida na íntegra
- Arquivo `docs/portabilidade/2026-10-16-causas-infer-windows-ci.md` lido para confirmar as medições do ML-W3a
- JSON validado via Python
