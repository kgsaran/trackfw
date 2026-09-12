# Vault Note: `"<title>"` vs `"title"` parity trap in orientation helpers

**Domínio:** parity, orientation helpers, ML-3A  
**Data:** 2026-09-12  
**REQ:** REQ-2026-09-11-by-agent-req-new-e-roadmap-new

## Causa raiz

Ao implementar `ReqNewLine` e `RoadmapNewLine` em Go (`internal/validator/validator.go`),
o argumento placeholder foi escrito como `\"<title>\"` (com angle brackets):

```go
return "trackfw req new \"<title>\""
```

Node.js e Python usaram `"title"` (sem angle brackets) desde o início. O `check-validate-parity.sh`
exercitava apenas o caso **flat** (sem `--agent`), cujo texto não passa pelo comparador
cross-runtime de `bhr-byagent`. O resultado foi:

- `check-artifact-parity.sh` → verde (não testa essa string diretamente)
- `check-validate-parity.sh` → verde (só testa presença de `"wip/ nor done/"`, não o texto completo)
- `make parity-rest` → **vermelho** em `check-cli-parity.sh` ou `check-validate-parity.sh` quando
  executado com o script correto que compara byte-a-byte

## Como foi detectado

O script `check-validate-parity.sh` ao ser rodado com `env -u FORCE_COLOR make parity-rest`
divergiu:

```
go=['... trackfw req new "<title>"\n  trackfw roadmap new "<title>"\n...']
node=['... trackfw req new "title"\n  trackfw roadmap new "title"\n...']
py=['... trackfw req new "title"\n  trackfw roadmap new "title"\n...']
```

## Correção

Remover os angle brackets de `ReqNewLine` e `RoadmapNewLine` em Go:

```go
// Antes
return "trackfw req new \"<title>\""
// Depois
return "trackfw req new \"title\""
```

## Como evitar

1. **Não usar angle brackets** em placeholders que compõem strings de instrução comparadas cross-runtime.
   A convenção já estabelecida (Node e Python) é `"title"` puro.
2. **O gate parity-rest captura isso**, mas só se `check-validate-parity.sh` tiver um fixture
   que exercite a string afetada e compare byte-a-byte. O fixture `bhr-byagent` (adicionado em ML-3A)
   agora garante que a orientação para by_agent+2 é comparada cross-runtime.
3. **Testes unitários de igualdade exata** (`assert.strictEqual`, `self.assertEqual`, `go test`
   com `!=`) são o discriminante mais rápido — substring assert deixaria esse bug passar.
