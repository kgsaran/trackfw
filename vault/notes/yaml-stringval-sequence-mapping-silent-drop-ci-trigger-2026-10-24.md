# go-yaml v3 `normalizeNode`/`stringVal`: SequenceNode e MappingNode caem silenciosamente para "ausente"

> Domínio: config parse | REQ: REQ-2026-10-24 (ci_trigger) | Agente: hades-tf | Data: 2026-10-24

## Código referenciado

`internal/config/config.go:594-623` (medido nesta sessão via `go run` com mirror exato):

- `normalizeNode(n *yaml.Node) interface{}` (linha 596): retorna `n.Value` (string) para ScalarNode, `[]interface{}` para SequenceNode, `map[string]interface{}` para MappingNode, `nil` para outros kinds. Doc: "using the **pre-coercion raw text** in Node.Value" — booleans/ints/nulls chegam como sua representação textual bruta, não como tipo Go.
- `stringVal(m map[string]interface{}, key string) (string, bool)` (linha 616): faz `v.(string)` — falha para `[]interface{}` e `map[string]interface{}`.

## Vetores medidos (mirror exato de normalizeNode/stringVal, 9 vetores)

| Input YAML | normalizeNode retorna | stringVal ok | Valor final |
|---|---|---|---|
| `ci: github-actions` | `"github-actions"` (string) | true | "github-actions" |
| `ci: true` | `"true"` (string — raw text) | true | "true" → enum check captura |
| `ci: 1` | `"1"` (string) | true | "1" → enum check captura |
| `ci: ~` | `"~"` (string) | true | "~" → enum check captura |
| `ci: null` | `"null"` (string) | true | "null" → enum check captura |
| `ci: [github-actions]` | `[]interface{}{"github-actions"}` | **false** | **campo ausente — violação AC4** |
| `ci: {a: b}` | `map[string]interface{}{"a":"b"}` | **false** | **campo ausente — violação AC4** |
| `ci:` (bare) | `""` (string — n.Value vazio) | true | "" → enum check captura |
| `ci: ''` | `""` (string) | true | "" → enum check captura |

Conclusão: **apenas SequenceNode e MappingNode escapam silenciosamente**. Null-tagged scalars (`~`, `null`, bare) retornam string via n.Value; `default: return nil` (linha 610) só dispara para kinds exóticos (DocumentNode, etc.) que nunca ocorrem em trackfw.yaml.

## Impacto em `ci_trigger:` (AC4 da REQ-2026-10-24)

`ci_trigger: [manual]` e `ci_trigger: {a: b}` → stringVal ok=false → campo silenciosamente ausente → AC4 "never silent fallback" violado.

O parser (config.go) lê silenciosamente por convenção. A violation de AC4 deve ser emitida pelo **validator** (validator.go) e a recusa pelo **update** (update.go — hoje sai rc=0, AC4 exige rc≠0).

Padrão correto no parser:
```go
if raw, present := m["ci_trigger"]; present {
    if s, ok := raw.(string); ok {
        if s == "pull_request" || s == "manual" {
            cfg.Update.CITrigger = s
        } else {
            cfg.Update.CITriggerInvalid = true
            cfg.Update.CITriggerRawValue = s
        }
    } else {
        cfg.Update.CITriggerInvalid = true // []interface{} ou map
    }
}
```

## Impacto na chave existente `ci:` (candidato Regra Dura)

Mesma falha em `ci:`. `ci: [github-actions]` → ok=false → cfg.Update.CI="" → platform detection silently defaults. Pré-existente. Arquiteto decide: entra na REQ-2026-10-24 como ML adicional ou nova REQ (diferença de mecanismo obrigatoriamente escrita).

## Armadilha de teste: `config.Load()` usa `sync.Once`

`config.Load()` (config.go, leitura do `sync.Once`) retorna singleton — por inspeção do código, não por teste medido nesta sessão. Testes que variem o input de parse devem chamar `parse()` diretamente com cada input como `[]byte`, nunca `config.Load()` em loop.
