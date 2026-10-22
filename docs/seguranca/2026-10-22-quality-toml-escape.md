# Parecer de Qualidade — Wave 4 (MLs 4A–4D)

> Agente: hefesto-tf | Data: 2026-10-22 | REQ: REQ-2026-09-26 | Branch: fix/harness-arquiteto-escape-toml-codex

## Escopo

`internal/integrations/render.go` (tomlBasicString, yamlEscapeDoubleQuoted, containsControlChar),
`internal/integrations/render_escape_test.go`,
`internal/integrations/plan.go` (IsStructuredAgentRepresentation),
`internal/integrations/thirdparty_refs_repr_test.go`,
`internal/identity/identity.go`, `internal/identity/identity_test.go`,
`internal/commands/integrations_thirdparty.go` e teste.

## Comandos executados

```
go test ./internal/integrations/ -run TestTOMLBasicString -v        → PASS (174 subtests)
go test ./internal/integrations/ -run TestYAMLEscapeDoubleQuotedNoRawControl -v → PASS
go test ./internal/integrations/ -run TestThirdPartyRefs -v         → PASS (9 subtests)
go test ./internal/identity/ -run TestLoad -v                       → PASS (includes TestLoadRejectsDELAndC1, TestLoadRejectsInvalidSlug)
go test ./internal/commands/ -run TestThirdPartyInstall_ApplyToRejectsStructuredRepresentationBeforeWrite -v → PASS
go vet ./internal/integrations/ ./internal/identity/ ./internal/commands/  → limpo (rc=0)
```

## Medições de suporte (scratchpad)

### Strictness do decodeTomlBasicString

Executado em
`/private/tmp/claude-501/-Users-kgsaran-Sistemas-Desenvolvimento-workspace-trackfw/c0992db2-d0d4-47c0-89e6-254582848291/scratchpad/q4e/decoder_main.go`:

```
u_bad_hex_zz           → ACCEPTED, len=1, result="\x00"   # Sscanf para em 'z', aceita parcial "00"
u_short                → ERROR: \u escape too short        # correto
U_out_of_range         → ACCEPTED, result="<U+FFFD>"      # 0x110000 > U+10FFFF, aceita sem recusar
U_surrogate (0x0000D800 via \U) → ACCEPTED, result="<U+FFFD>"  # sem check de surrogate no branch \U
raw_nel_utf8 (0xC2 0x85) → ACCEPTED, result="\u0085"      # correto per TOML 1.0 spec
raw_ls_utf8 (LS U+2028) → ACCEPTED, result=" "       # correto per TOML 1.0 spec
```

Conclusão da medição: o decoder tem três lacunas (hex parcial, \U sem range-check, \U sem check de surrogate). O encoder (`tomlBasicString`) **nunca** emite essas sequências inválidas — ele só produz hex válido via `fmt.Fprintf` e Go nunca cria runes fora do range Unicode nem surrogates em strings. As lacunas do decoder não afetam a validade dos round-trips; elas afetariam um uso do decoder como validador geral de TOML, que não é o seu papel aqui.

---

## 1. Vacuidade dos contra-braços

### `arm_b_contra_arm_strconv_quote_rejected` (TOML)

**O que faz:** invoca `strconv.Quote` sobre BEL (0x07) e VT (0x0B) e confirma que `decodeTomlBasicString` rejeita o resultado.

**Papel real:** é um sanity check do *decodificador*, não do encoder. O braço não invoca `tomlBasicString`. A proteção contra regressão do encoder está em `arm_a_round_trip`: se `strconv.Quote` fosse substituído, `decodeTomlBasicString` rejeitaria `\a`/`\v` e todos os 32 subtests de C0+DEL reprovariam.

O arquiteto mediu exatamente isso: "com `strconv.Quote`, reprovam `arm_a_round_trip` (todo C0 e o DEL) e `arm_c_invalid_utf8_error`". A documentação está no roadmap (ML-4A, auditoria do arquiteto).

**Conclusão:** `arm_b` protege contra enfraquecimento do decodificador (ex: alguém adicionando `case 'a', 'v'` ao switch). Tem valor legítimo e complementar ao `arm_a`. Não é vacuidade.

**Observação menor (A1):** o comentário de `arm_b` diz "This proves the decoder bites" — correto, mas não fica explícito que o braço valida o decodificador, não o encoder. Poderia ser mais claro. Não afeta cobertura funcional.

### `contra_arm_old_encoder_leaves_raw_control` (YAML)

**O que faz:** aplica `oldYAMLEscapeDoubleQuoted` (encoder pré-fix, copiado localmente) a 4 inputs e confirma que `hasRawControlRuneInDoubleQuoted` os detecta.

**Papel real:** valida o verificador `hasRawControlRuneInDoubleQuoted`, não o encoder novo. Sem esse braço, o verificador poderia ser neutralizado sem reprovar o braço principal (`new_encoder_no_raw_control_and_round_trips`) — porque o encoder novo já produz saída sem controle cru, e um verificador sempre-false retornaria false corretamente para ela.

**Conclusão:** tem valor como guarda contra enfraquecimento do verificador. Não é vacuidade.

### Os decoders são estritos o suficiente?

**decodeTomlBasicString:** rejeita escape desconhecido e byte de controle cru (C0 exceto tab, DEL). Não rejeita raw C1 (correto per spec TOML 1.0, que inclui 0x80–0xD7FF em `non-ascii`). Três lacunas medidas (hex parcial, \U out-of-range, \U sem surrogate-check) — ver seção "Medições" acima. Essas lacunas não afetam a validade dos round-trips do encoder.

**decodeYAMLDoubleQuotedString:** permissivo por design (não rejeita chars crus). O contrato de "sem controle cru" é imposto pelo verificador `hasRawControlRuneInDoubleQuoted`. Design correto, mas significa que o contra-braço é a única barreira do verificador.

---

## 2. Correção do `tomlBasicString`

**`\uXXXX` com hex minúsculo:** `fmt.Fprintf(&sb, `\u%04x`, r)`. TOML 1.0 não exige maiúsculas; Python `tomllib` e o decoder do teste (`fmt.Sscanf(hex, "%04x", &cp)`) aceitam ambos. **Correto.**

**`\U` para > U+FFFF:** `fmt.Fprintf(&sb, `\U%08x`, r)`. Mesmo argumento. **Correto.**

**U+FFFD legítimo:** `utf8.DecodeRuneInString("�")` → `(0xFFFD, 3)`. A guarda `r == utf8.RuneError && size == 1` NÃO é acionada (size=3). Cai em `!strconv.IsPrint(r)` → emite `�`. Decoder recupera 0xFFFD. Caso `fffd_valid` em `arm_a_round_trip` cobre isso. **Correto.**

---

## 3. `containsControlChar` e `models.go:112`

A função ampliou de C0+U+2028/U+2029 para C0+DEL+C1+U+2028/U+2029. Os três chamadores:

| Chamador | Arquivo:linha | Efeito da ampliação |
|---|---|---|
| `needsYAMLQuoting` | render.go:147 | DEL e C1 agora forçam aspas duplas |
| `rewriteFrontmatterModelLine` | render.go:775 | DEL e C1 num model rejeitam com erro |
| `LooksLikeSuspectModelValue` | models.go:112 | DEL e C1 tornam model suspeito |

Nenhum model ID legítimo usa DEL ou C1. Mudança é conservadora na direção certa; nenhum model é falsamente rejeitado.

---

## 4. U+2028/U+2029 e `yamlEscapeDoubleQuoted` — LACUNA (R2)

**Medição:**

- `containsControlChar` (render.go:753) INCLUI U+2028/U+2029 via os dois checks literais: `r == ' ' || r == ' '` (os dois chars invisíveis ao final da linha são U+2028 e U+2029).
- Logo `needsYAMLQuoting` retorna `true` para qualquer valor com U+2028/U+2029 → `yamlSafeScalar` os coloca em aspas duplas.
- `yamlEscapeDoubleQuoted` (render.go:192–212) só escapa `r < 0x20 || r == 0x7F || (r >= 0x80 && r <= 0x9F)`. U+2028 = 0x2028 e U+2029 = 0x2029 **não caem em nenhum desses ranges** → emitidos crus (`sb.WriteRune(r)`).
- Resultado: um valor com U+2028 produz `"...a b..."` com o codepoint literal nos bytes do arquivo.

**Comportamento yaml.v3 com LS/PS crus em scalar double-quoted:**

libyaml (base do yaml.v3) trata U+2028 e U+2029 como line breaks (`is_break`). Em scalars double-quoted, um line break não escapado é "folded": uma quebra simples → espaço; múltiplas → menos uma. Portanto `"a b"` (bytes `"a` + 0xE2 0x80 0xA8 + `b"`) seria decodificado como `"a b"` — **round-trip falha**.

**Reachability:** BAIXA. Os dois caminhos principais que chegam a `yamlSafeScalar` com valores do usuário têm defesa de entrada anterior:
- `identity.Load` chama `containsControlCharIdent` que recusa U+2028/U+2029 antes de qualquer `Render`.
- `rewriteFrontmatterModelLine` (render.go:775) usa `containsControlChar` que recusa U+2028/U+2029 no valor do model.

Os valores de nome de item do catálogo são strings ASCII dos assets — sem U+2028/U+2029.

**Classificação:** lacuna de defense-in-depth no encoder. A camada de entrada já bloqueia; a camada de encoding não cobre. Por causa da mesma causa de ML-4B (escape YAML incompleto para caracteres que afetam a estrutura do frontmatter), a **Regra Dura** enquadra isso como ML adicional desta REQ, não como observação isolada.

**Fix:** em `yamlEscapeDoubleQuoted`, adicionar ao `default`:
```go
} else if r == 0x2028 || r == 0x2029 {
    fmt.Fprintf(&sb, `\u%04x`, r)
```
(YAML 1.2 aceita `\uXXXX` em double-quoted scalars.)

E adicionar ao `TestYAMLEscapeDoubleQuotedNoRawControl` os dois casos `LS_0x2028` e `PS_0x2029` no braço principal e no contra-braço.

---

## 5. `IsStructuredAgentRepresentation` — risco de drift (R3)

**Achado:** `IsStructuredAgentRepresentation` (`plan.go:45`) hardcoda `{custom-agent-toml, agent-json, cli-agent-json}`. O switch de `Render` (`render.go:280,306`) é a fonte primária de "quais representações emitem TOML/JSON". O teste `structuredRepTargetCases` (`thirdparty_refs_repr_test.go:34`) hardcoda uma terceira lista `[codex, kiro/cli, amazonq]`.

**Catálogo atual** (medido via `catalog.json`):

| Target:surface | Representation |
|---|---|
| codex:cli | custom-agent-toml |
| antigravity:legacy-cli | agent-json |
| kiro:cli | agent-json |
| amazonq:cli | cli-agent-json |

`IsStructuredAgentRepresentation` cobre exatamente esses 3 valores. Estado atual: correto.

**Risco:** se um novo target com `representation: custom-agent-yaml` (por exemplo) for adicionado ao catálogo, `Render` precisaria de um novo case, `IsStructuredAgentRepresentation` precisaria de atualização, e `structuredRepTargetCases` precisaria incluir o novo target. Não há gate que detecte automaticamente a divergência. O vault registra o mesmo padrão em `listas-de-targets-hardcoded-divergem-do-catalog-json-2026-08-05.md`.

**Fix proposto:** um teste que itera `catalog.Targets`, verifica para cada surface de cada target qual a representação de agente, e requer que `IsStructuredAgentRepresentation` retorne `true` para toda representação que o `Render` switch trata como TOML ou JSON (i.e., que não é markdown). Isso fecha o loop sem hardcoding.

**Classificação:** RESSALVA. O código atual está correto; o risco é de regressão futura. Não bloqueia merge, mas deve entrar como ML antes do fechamento da REQ.

---

## 6. Manutenção

- Nomes distintos e sem ambiguidade entre `containsControlChar` (integrations) e `containsControlCharIdent` (identity). Correto.
- Comentários de teste com "asserts CONCLUSION of..." satisfazem a Regra Dura de Reconciliação.
- `thirdparty_refs_repr_test.go` reutiliza `decodeTomlBasicString` do mesmo pacote com comentário documentando a não-duplicação.
- Contexto histórico do U+0085 ("yaml.v3 normaliza") foi removido do comentário de `containsControlChar`. O novo comentário descreve o comportamento atual ("includes NEL U+0085"). Não há nota de vault; o diff explica. TRIVIAL.

---

## 7. Veredito

**APROVADO COM RESSALVA**

| ID | Sev | Achado | Fix requerido |
|---|---|---|---|
| R2 | RESSALVA | `yamlEscapeDoubleQuoted` não escapa U+2028/U+2029 apesar de `containsControlChar` os sinalizar; yaml.v3 os faz fold para espaço em scalar double-quoted. Reachability baixa (identity.Load bloqueia na entrada), mas é lacuna de defense-in-depth no encoder. Mesma causa de ML-4B → Regra Dura exige ML adicional nesta REQ. | Adicionar `r == 0x2028 || r == 0x2029` ao escape em `yamlEscapeDoubleQuoted`. |
| R3 | RESSALVA | `structuredRepTargetCases` e `IsStructuredAgentRepresentation` são hardcodes não ligados ao catálogo. Risco de drift ao adicionar novo target estruturado. Vault: `listas-de-targets-hardcoded-divergem-do-catalog-json-2026-08-05.md`. | Teste que itera `catalog.Targets` e verifica `IsStructuredAgentRepresentation` para cada representação não-markdown. |
| A1 | Observação | Decoder `decodeTomlBasicString` aceita hex parcial (`\u00zz` → `\x00`), `\U` out-of-range (0x110000 aceito), `\U` sem surrogate-check. Não afeta round-trips do encoder (que emite hex sempre válido), mas o decoder não funciona como validador TOML geral. | Sem impacto prático atual; documentado. |
| A2 | Observação | `arm_b_contra_arm` valida o decodificador, não o encoder. Papel complementar válido, mas poderia ser mais explícito no comentário. | Clareza de comentário; sem impacto funcional. |

Os Rxx devem ser tratados como novos MLs desta REQ (Regra Dura de Causa Raiz). O merge pode prosseguir com nota para o arquiteto: R2 requer correção antes de fechar o roadmap; R3 requer cobertura adicional de teste.

---

## Nota para o parecer (substitui entradas que iriam para outros arquivos)

**Entrada para `docs/agents-working-context.md`** (não escrita por restrição do handoff):

```
2026-10-22 | hefesto-tf | START | ML-4E Revisão de qualidade Wave 4 (ROADMAP-2026-09-26)
  Escopo: render.go (tomlBasicString, yamlEscapeDoubleQuoted, containsControlChar),
  render_escape_test.go, plan.go (IsStructuredAgentRepresentation),
  thirdparty_refs_repr_test.go, identity.go, identity_test.go,
  integrations_thirdparty.go e teste.

2026-10-22 | hefesto-tf | DONE  | ML-4E concluído. Veredito: APROVADO COM RESSALVA.
  R2: yamlEscapeDoubleQuoted não escapa U+2028/U+2029 — lacuna de defense-in-depth,
      mesma causa ML-4B, Regra Dura → novo ML na REQ.
  R3: structuredRepTargetCases hardcoded — risco de drift do catálogo.
  A1: decoder TOML aceita hex parcial/out-of-range — não afeta round-trips do encoder.
  A2: arm_b valida decodificador, não encoder — observação de clareza.
```

**Nota de vault candidata** (não escrita por restrição do handoff — atinge limiar de >10 min de investigação):

Arquivo: `vault/notes/yaml-escape-ls-ps-gap-2026-10-22.md`

Conteúdo: `yamlEscapeDoubleQuoted` (render.go:192) escapa C0+DEL+C1 mas NÃO escapa U+2028 (LS) e U+2029 (PS). `containsControlChar` sinaliza esses chars → `needsYAMLQuoting` retorna true → `yamlSafeScalar` os coloca em aspas duplas, mas dentro das aspas eles saem crus. yaml.v3 trata LS/PS como line breaks (`is_break` em libyaml) e os faz fold para espaço em scalars double-quoted → round-trip falha. Reachability baixa (identity.Load bloqueia na entrada via `containsControlCharIdent`), mas é lacuna de encoding layer. Fix: `fmt.Fprintf(&sb, `\u%04x`, r)` para `r == 0x2028 || r == 0x2029` em `yamlEscapeDoubleQuoted`. Mesmo padrão que levou à adição de C1 no ML-4B.
