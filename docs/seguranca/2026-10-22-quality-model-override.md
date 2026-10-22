# Parecer de Qualidade — ML-2A: Proibição de `model` no dispatch do arquiteto

> REQ: REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio
> Data: 2026-10-22 | Revisor: hefesto-tf | Worktree: /private/tmp/trackfw-model-override
> Branch: fix/arquiteto-nao-sobrepoe-modelo-do-subagente

---

## Escopo revisado

Arquivos analisados (via `git diff origin/main -- internal/integrations/`):

- `internal/integrations/assets/agents/architect.md` — parágrafo novo em `## Dispatch contract`
- `internal/integrations/render_test.go` — helpers `extractBodyText` (linha 1535), `extractDispatchSection` (linha 1570) e teste `TestArchitectDispatchContractForbidsModelOverride` (linha 1588, ~240 linhas)
- `internal/integrations/testdata/architect.subagent.golden.md`
- `internal/integrations/testdata/architect.agent-directory.golden.md`

Comandos executados (todos no worktree /private/tmp/trackfw-model-override, mutações sobre cópias em scratchpad):

```
go vet ./internal/integrations/
  → PASS (zero saída)

go test ./internal/integrations/ -run TestArchitectDispatchContractForbidsModelOverride -v 2>&1 | grep -c "PASS:"
  → 49

go test ./internal/integrations/ -run TestRenderWithoutIdentityMatchesFrozenGoldens -v
  → PASS: 4 sub-testes

# python3 sobre assets/catalog.json — enumerou representações e confirmou chave support_level
```

49 linhas `PASS:` = 1 raiz + 12 pais (tgt/surf) + 12 zeus + 12 negative/no-paragraph + 12 negative/no-rationale = 49.

---

## Questão 1 — Vacuidade: algum braço pode passar sem afirmar nada?

### Guardas `bytes.Equal` para noParagraph e noRationale

**Mutation (d):** `paragraphNeedle` trocado por `"THIS_NEEDLE_DOES_NOT_EXIST_IN_SOURCE"`.

```
render_test.go:1637: paragraphNeedle not found in source; cannot build noParagraph needle — ...
FAIL
```

A guarda `t.Fatal` dispara imediatamente. Tem mordida real.

**noRationale:**
```go
noRationale := []byte(strings.Replace(string(source), rationalePhrase, "", 1))
if bytes.Equal(noRationale, source) {
    t.Fatal("noRationale vacuity guard...")
}
```
A frase `rationalePhrase` está no asset (confirmado pela execução). Se não estivesse, `t.Fatal` dispararia.

### Caso de `extractDispatchSection` devolver vazio

**Mutation (a):** `extractDispatchSection` substituída por `return body`.

```
go test ./internal/integrations/ -run TestArchitectDispatchContractForbidsModelOverride
→ PASS (49/49)
```

O teste passa sem qualquer alteração de comportamento observável. Isso significa:

- **A asserção de localização não tem mordida.** O teste afirma que "as frases devem estar dentro da seção ## Dispatch contract, não em outra seção ou em comentário" — mas nenhum braço falsifica esse claim. Os braços positivos encontram as frases no corpo completo porque phrase1/phrase2 só existem no parágrafo proibido; os braços negativos removem as frases do corpo inteiro, portanto elas ficam ausentes independentemente de onde `extractDispatchSection` corta.

- **O que falta:** um quinto braço que mova o parágrafo proibido para dentro de outra seção (ex.: após `## Post-microbatch audit`) e espere que `checkPhrases` rejeite. Com a implementação atual, esse braço passaria vacuosamente — com `extractDispatchSection` funcional ou com `return body`.

### Braços negativos quando `extractDispatchSection` retorna `""`

Se `extractDispatchSection` retornar `""`, `checkPhrases` retorna mensagem não-vazia → braços negativos passam vacuosamente. Risco teórico, não realizável com as mutações atuais (que só removem conteúdo dentro da seção, não o cabeçalho). Classificado abaixo em A4.

---

## Questão 2 — Cobertura de `requiredReprs`

Representações no `catalog.json` com `support_level != "unsupported"` (todas as 9 são `native` ou `fallback`, nenhuma é `unsupported`):

```
agent-directory    native   ← em requiredReprs ✓
agent-json         legacy   ← em requiredReprs ✓
agent-markdown     native   ← AUSENTE de requiredReprs
cli-agent-json     native   ← em requiredReprs ✓
custom-agent       native   ← AUSENTE de requiredReprs
custom-agent-toml  native   ← em requiredReprs ✓
opencode-agent     native   ← em requiredReprs ✓
skill              fallback ← AUSENTE de requiredReprs
subagent           native   ← em requiredReprs ✓
```

O loop itera sobre todos os alvos do catálogo e seta `coveredReprs[cap.Representation] = true` para qualquer `support_level != "unsupported"`. As três representações ausentes (`agent-markdown`, `custom-agent`, `skill`) **são exercitadas pelo loop** — os braços positive/zeus/negative/no-paragraph/negative/no-rationale rodam para elas. O floor check `requiredReprs` só garante que essas 6 nunca desapareçam silenciosamente.

**Conclusão:** se uma representação da lista de 6 sumir do catálogo, o check falha. Para as 3 não listadas, o check não detecta desaparecimento — mas o loop ainda as exercita enquanto existirem.

**Caso de representação que muda de nome:** se "subagent" virar "subagent-v2", o check falha (correto). Se "agent-markdown" virar "markdown-agent", o check não detecta (gap real, mas baixo impacto pois `agent-markdown` está no loop).

---

## Questão 3 — Manutenibilidade

### A1 (MÉDIA): asserção de localização sem mordida — achado principal

Detalhado na Questão 1. O docstring promete que `checkPhrases` verifica localização dentro da seção; a mutação prova que não.

### A2 (MINOR): bloco de comentário duplicado em `checkPhrases`

**Localização:** `render_test.go` linhas 1665–1668.

```go
// checkPhrases returns a non-empty message if either required phrase is
// absent from the ## Dispatch contract section of the decoded body.
// checkPhrases returns a non-empty message if either required phrase is
// absent from the ## Dispatch contract section of the decoded body.
checkPhrases := func(body string) string {
```

Quatro linhas, as duas primeiras idênticas às duas últimas. Artefato de copy-paste.

### A3 (MINOR): `tgt := tgt` e `surf := surf` desnecessários

`go.mod`: `go 1.25.2`. A partir de Go 1.22, a captura de variável de loop por closures foi corrigida. Essas atribuições locais são inócuas mas desnecessárias.

### A4 (MINOR): `cap` sombrea o builtin

```go
cap := surf.Capabilities.Agents
```

`cap` é builtin de slice. Não causa defeito aqui mas é conflito de nome de baixo risco.

### `extractBodyText` — helper novo, não duplicado

Existe lógica similar inline (linha ~82, decoding de `developer_instructions` embutido em outro teste), mas não como função nomeada reutilizável. O novo helper cria abstração correta. Não é duplicação.

**Mutation (c):** `extractBodyText` substituída por `return string(output)`.

```
render_test.go:1684: positive (no identity): ## Dispatch contract section not found in decoded body
  [codex/cli — custom-agent-toml]
render_test.go:1696: positive (zeusIdentity): phrase1 absent from ## Dispatch contract section: "}\n"
  [antigravity/legacy-cli — agent-json]
FAIL
```

A decodificação TOML (via `strconv.Unquote`) e JSON (via `json.Unmarshal`) é **load-bearing**: sem ela, o corpo de texto não é extraído corretamente e os braços positivos falham para `custom-agent-toml` e `agent-json`.

---

## Questão 4 — Goldens

Verificação:

```bash
diff <(grep "Never pass" internal/integrations/assets/agents/architect.md) \
     <(grep "Never pass" internal/integrations/testdata/architect.subagent.golden.md)
# → sem saída (idêntico)

diff <(grep "Never pass" internal/integrations/assets/agents/architect.md) \
     <(grep "Never pass" internal/integrations/testdata/architect.agent-directory.golden.md)
# → sem saída (idêntico)
```

`TestRenderWithoutIdentityMatchesFrozenGoldens` → PASS (4/4 sub-testes), confirmando byte-a-byte.

O diff é exclusivamente o parágrafo proibido. Nenhuma outra linha mudou nos goldens.

---

## Questão 5 — phrase2 check tem mordida?

**Mutation (b):** bloco `phrase2` removido de `checkPhrases`.

```
--- FAIL: TestArchitectDispatchContractForbidsModelOverride/claude/cli/negative/no-rationale
[mesma falha em todos os 12 targets]
FAIL
```

Todos os `negative/no-rationale` falham: sem o check de `phrase2`, `checkPhrases` retorna `""` quando phrase2 está ausente → o braço que espera rejeição não recebe. O check tem mordida real.

---

## Resumo de achados por severidade

| # | Severidade | Achado | Evidência |
|---|---|---|---|
| A1 | MÉDIA | A asserção de localização (`## Dispatch contract`) não tem mordida — `extractDispatchSection` pode ser substituída por `return body` e todos os 49 testes passam. O docstring promete verificação de seção; a mutação prova que não há. Falta um braço que mova o parágrafo para outra seção e espere rejeição. | mutation (a): PASS quando esperado FAIL |
| A2 | MINOR | Bloco de comentário de `checkPhrases` duplicado verbatim (4 linhas) | render_test.go:1665–1668 |
| A3 | MINOR | `requiredReprs` não inclui `agent-markdown`, `custom-agent`, `skill` — essas representações são exercitadas pelo loop mas não pelo floor check | catalog.json vs. render_test.go:1684–1687 |
| A4 | MINOR | `tgt := tgt` e `surf := surf` desnecessários com Go ≥ 1.22 (go.mod: 1.25.2) | go.mod:3 |
| A5 | MINOR | `cap := surf.Capabilities.Agents` sombrea o builtin `cap` | render_test.go:1698 |

Confirmado por mutação:
- **phrase2 check tem mordida real** (mutation b → FAIL em todos os `no-rationale`)
- **TOML/JSON decoding é load-bearing** (mutation c → FAIL em `codex/cli` e `antigravity/legacy-cli`)
- **vacuity guard de `paragraphNeedle` funciona** (mutation d → `t.Fatal` imediato)

---

## Veredito

**APROVADO COM RESSALVA**

O teste entrega o que a REQ exige: a proibição de `model` está presente no asset e em todos os renders de todas as representações, e isso é verificável com os braços positivos/negativos que têm mordida real (phrase1+phrase2 globalmente, phrase2 isolada via `no-rationale`, `extractBodyText` load-bearing para TOML/JSON).

A ressalva de média gravidade é A1: o claim de localização dentro da seção não é falsificável — o teste passa mesmo quando `extractDispatchSection` ignora os limites de seção. O defeito não é de segurança: a proibição está no lugar certo no asset e nos renders. É uma lacuna de teste: se um futuro commit mover o parágrafo para outra seção, o teste não detecta.

**Ressalva de média gravidade (A1):** adicionar braço `negative/wrong-section` que move o parágrafo para após `## Post-microbatch audit` na fonte mutada e espera que `checkPhrases` rejeite. Sem esse braço, `extractDispatchSection` é decorativa.

**Ressalvas sem urgência de bloqueio (A2–A5):** corrigíveis isoladamente, nenhuma bloqueia merge.

---

## O que iria para `docs/agents-working-context.md`

(Não escrito diretamente por instrução do Zeus — registro incluído aqui para o arquiteto)

```
2026-10-22 | hefesto-tf | ML-2A (revisão de qualidade) | fix/arquiteto-nao-sobrepoe-modelo-do-subagente
INÍCIO: revisão de TestArchitectDispatchContractForbidsModelOverride e helpers em render_test.go
FIM: veredito APROVADO COM RESSALVA.
  A1 MÉDIA: asserção de localização (extractDispatchSection) sem mordida — mutation (a) prova.
  A2-A5 MINOR: comment duplicado, requiredReprs incompleto, tgt:=tgt redundante, cap sombrea builtin.
  Goldens byte-a-byte corretos. phrase2 e decoding load-bearing confirmados por mutação.
  Parecer: docs/seguranca/2026-10-22-quality-model-override.md
```
