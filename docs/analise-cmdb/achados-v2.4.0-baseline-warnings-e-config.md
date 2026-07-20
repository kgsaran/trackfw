# Achados da v2.4.0 — ratchet de warnings ausente + gotchas de config

> **Origem:** validação de campo da v2.4.0 no CMDB.
> **Autor:** 🌩️ Zeus — arquiteto do CMDB · **Data:** 2026-06-13
> **Destinatário:** agente/mantenedor do trackfw · **Alvo sugerido:** v2.4.1 (bugfix) / v2.5.
> **Contexto:** a v2.4.0 implementou as 3 recomendações de
> [`evolucao-generica-baseline-legado.md`](./evolucao-generica-baseline-legado.md): field mapping
> (`link_fields.*`), severidade por regra (`rules.*`) e baseline (`trackfw baseline`).

---

## Resumo da validação

| Pilar | Status | Evidência |
|---|---|---|
| Field mapping (`link_fields.req: ["REQ:", "req_id:"]`) | ✅ funciona | `req_id:` no frontmatter passou a satisfazer "tem REQ" |
| Severidade por regra (`rules.adr_orphan: off`) | ✅ funciona | aviso de ADR órfão silenciado |
| **Baseline + ratchet** | ⚠️ **parcial** | filtra `violations`, **não** `warnings` (Achado 1) |

Dois pilares estão sólidos. O baseline tem uma lacuna que **anula seu principal caso de uso em
legado**, mais um gotcha de parsing de config. Detalhes abaixo.

---

## Achado 1 (alto impacto) — o ratchet do baseline ignora `warnings`

### Sintoma

Após `trackfw baseline`, violations baselined desaparecem, mas **warnings baselined permanecem**.

**Reprodução mínima:**

```bash
# repo com 1 violation (wip sem REQ) + 1 warning (ADR órfão, severidade default "warning")
trackfw validate          # 1 violation + 1 warning
trackfw baseline          # "Baseline gravado: 1 violations, 1 warnings"
trackfw validate          # esperado: 0 ; observado: o WARNING ainda aparece
```

O warning de `ADR-001-legado.md is not referenced by any REQ` continua sendo reportado mesmo estando
no snapshot.

### Causa raiz (confirmada no código)

`internal/validator/validator.go`, função `Validate()`:

```go
baselineSet := make(map[string]struct{}, len(baseline.Violations))
for _, v := range baseline.Violations {        // ← só Violations
    baselineSet[v] = struct{}{}
}
var netNew []string
for _, v := range violations {                 // ← filtra só violations
    if _, exists := baselineSet[v]; !exists {
        netNew = append(netNew, v)
    }
}
violations = netNew
// baseline.Warnings é GRAVADO (SaveBaseline) mas NUNCA lido aqui
```

`BaselineFile` tem o campo `Warnings`, e `SaveBaseline` o persiste — mas o filtro do ratchet **nunca o
consome**. Warnings não são submetidos ao baseline.

### Por que isso anula o caso de uso de legado

O objetivo do baseline é **congelar o passivo e cobrar só o novo**. Mas, por design da v2.4.0, **a
maioria do passivo legado é warning**, não violation:

- `rules.adr_orphan` default = `"warning"` → os **59 ADR órfãos** do CMDB são warnings.
- `rules.stale_wip`, `rules.ref_targets_exist`, `rules.folder_status` também default `"warning"`.

Logo, `trackfw baseline` **não silencia** o ruído legado dominante. Para contornar hoje, o time
precisaria **promover** essas regras a `error` só para o baseline pegá-las — contraintuitivo e
perde a distinção semântica violation×warning.

Pior ainda em **modo `lenient`** (caso do CMDB): `Validate()` move as violations restantes para
warnings **depois** do filtro de baseline. Como os órfãos já nascem warning (via `applyRule`), eles
**nunca passam pelo filtro** — o baseline vira efetivamente um no-op para esse projeto.

### Correção sugerida

Aplicar o ratchet **também a warnings** — simétrico ao de violations:

```go
warnSet := make(map[string]struct{}, len(baseline.Warnings))
for _, w := range baseline.Warnings {
    warnSet[w] = struct{}{}
}
var netNewWarn []string
for _, w := range warnings {
    if _, exists := warnSet[w]; !exists {
        netNewWarn = append(netNewWarn, w)
    }
}
warnings = netNewWarn
```

Considerar também a ordem com o modo lenient: idealmente filtrar baseline **antes** de mover
violations→warnings (como hoje) **e** filtrar warnings pelo baseline — para que o lenient não
"recrie" ruído já baselined.

---

## Achado 2 (médio) — valor de regra entre aspas quebra o parsing

### Sintoma

```yaml
rules:
  adr_orphan: "off"     # COM aspas → NÃO reconhecido (regra continua ativa)
  adr_orphan: off       # SEM aspas → funciona
```

### Causa raiz

`internal/config/config.go`, `splitKV` **não remove aspas** do valor:

```go
func splitKV(line string) (key, val string, ok bool) {
    idx := strings.Index(line, ":")
    ...
    val = strings.TrimSpace(line[idx+1:])   // ← sem strings.Trim(val, `"'`)
    return key, val, key != ""
}
```

O comparador de severidade espera `off`/`warning`/`error`; recebe `"off"` (com aspas) e cai no default.

### Agravante de UX

O `trackfw help` **exibe os valores entre aspas** (`"error"`, `"warning"`, `"off"`). Um usuário que
copiar o formato exibido cai exatamente no bug.

### Correção sugerida

Em `splitKV`, remover aspas envolventes do valor: `val = strings.Trim(val, "\"'")`. Isso também
harmoniza com `extractFrontmatterField`, que **já** faz `strings.Trim(val, "\"'")`.

---

## Achado 3 (menor / fragilidade) — parser de YAML artesanal

O `trackfw.yaml` é parseado por um leitor manual baseado em flags de indentação
(`internal/config/config.go`), não por uma lib YAML. Consequências observadas:

- **Listas só em formato de bloco** (`- item`). Array inline (`req: ["A", "B"]`) é **silenciosamente
  ignorado** — o campo fica no default sem aviso.
- Sensível a indentação/variações que uma lib YAML toleraria.

**Sugestão:** migrar para `gopkg.in/yaml.v3` (ou similar) — elimina toda essa classe de bugs
(aspas, inline arrays, indentação) de uma vez e reduz superfície de manutenção. Se a escolha por
parser próprio for intencional (zero-dep), no mínimo: (a) aceitar inline arrays, (b) trim de aspas,
(c) **avisar** quando uma chave conhecida recebe valor em formato não-parseável.

---

## Prioridade sugerida

1. **Achado 1** (ratchet de warnings) — **bugfix v2.4.1**. Sem ele, o baseline não cumpre a promessa
   de legado para projetos cujo passivo é majoritariamente warning (o caso comum, e o do CMDB).
2. **Achado 2** (aspas) — bugfix barato, alto retorno de UX (alinhado ao que o `help` mostra).
3. **Achado 3** (lib YAML) — refator de robustez; resolve 1 e 2 como efeito colateral.

> Validação geral da v2.4.0: **muito boa** — field mapping e severidade por regra entregues e
> comprovados. Falta apenas fechar o ratchet de warnings para o baseline cumprir 100% do objetivo
> de "genérico para legado sem perder os dentes".
