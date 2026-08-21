# nil map em ProjectConfig: campo novo de mapa atinge construções antigas de parse()

> Criado em: 2026-08-21 | REQ: REQ-2026-08-21-nil-map-em-construcao-de-projectconfig-causa-panic-quando-agent-models-esta-configurado.md

## Raiz do defeito

Go inicializa campos de mapa em zero-value como `nil`. `parse()` escreve em mapas (`cfg.Rules[k]`, `cfg.AgentModels[k]`) sem verificar se são `nil`. Quando um novo campo de mapa é acrescentado ao struct `ProjectConfig` e `parse()` passa a escrever nele, TODA construção que não inicializou esse mapa explicitamente passa a ser uma bomba-relógio.

**O ML-1B acrescentou `AgentModels map[string]string` e `parse()` passou a escrever nele. Sobreviveram com AgentModels nil:**

| Função | Linha | Estado após ML-1B |
|---|---|---|
| `ParseRulesFromContent` | config.go:165 | `ProjectConfig{Rules: make(...)}` — AgentModels nil |
| `ReadAgentConventions` | config.go:181 | `ProjectConfig{Rules: make(...)}` — AgentModels nil |

O ML-2B corrigiu `ReadAgentConventions`. O ML-2C corrigiu `ParseRulesFromContent` **e fechou a classe**.

## O padrão de falha de auditoria

O caminho de repro exige `CLAUDE.md` presente no projeto. Sem ele, a geração de regras é pulada e o panic não aparece. O auditor do ML-1B exercitou `agents install` e `agents models` — nos dois, a geração de regras é pulada. **Testou o que a feature nova usa, não o que a mudança de struct atinge.** Campo novo em struct compartilhado tem raio de alcance maior que a feature que o motivou.

## O segundo campo de mapa latente: Rules

`cfg.Rules[k] = s` em `parse()` (linha ~332) é outra escrita em mapa. Não panicou ainda porque toda construção existente inicializava `Rules`. É a mesma classe de defeito, mascarada pela convenção acidental.

## A solução que fecha a classe

`initConfigMaps(cfg)` por reflexão no início de `parse()`:

```go
func initConfigMaps(cfg *ProjectConfig) {
    v := reflect.ValueOf(cfg).Elem()
    t := v.Type()
    for i := 0; i < t.NumField(); i++ {
        fv := v.Field(i)
        if fv.Kind() == reflect.Map && fv.IsNil() {
            fv.Set(reflect.MakeMap(t.Field(i).Type))
        }
    }
}
```

**Por que reflexão e não per-field nil checks?** Per-field exige que o desenvolvedor que adiciona um novo campo de mapa faça DUAS edições: a escrita em `parse()` e o nil-check no init. A reflexão faz o sweep de todos os campos de mapa do struct automaticamente — o próximo campo de mapa é coberto sem nenhuma ação humana.

**Gate de enforcement:** `TestAllMapFieldsInitializedAfterParse` usa reflexão para provar que todo campo de mapa de `ProjectConfig` é não-nil após qualquer chamada a `parse()`, mesmo com construção bare `ProjectConfig{}`.

## Node.js e Python: imunes por construção

Em ambos, toda construção de config object inclui `agentModels: {}` / `agent_models: {}` no literal do objeto. JavaScript e Python não têm nil maps — o problema é específico de Go (zero-value de map).

## Lição para futuros campos de mapa em ProjectConfig

**NÃO é necessário inicializar manualmente no caller.** `initConfigMaps(cfg)` no início de `parse()` garante que qualquer campo de mapa seja inicializado antes de qualquer escrita. O único requisito é que a escrita esteja em `parse()`.

## P4

Cenário 85 em `check-gates-falsify.sh`: corrompe `config.go` removendo a chamada `initConfigMaps(cfg)`, e roda `go test ./internal/config/ -run TestParseRulesFromContentWithAgentModels_NoPanic` na cópia corrompida (via `$T85/`). O teste panica com "assignment to entry in nil map" no código corrompido e passa no código real.

**Decisão de design:** abordagem via `trackfw validate` + git HEAD foi descartada porque `headTrackfwYAML()` envolve subprocess git cujo retorno depende do ambiente — mesmo com fixture, a chamada pode não chegar a `ParseRulesFromContent`. O `go test` exerce a função diretamente sem camada de subprocess, tornando a prova determinística.
