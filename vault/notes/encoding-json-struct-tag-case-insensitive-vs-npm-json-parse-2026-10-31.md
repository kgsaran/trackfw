# `encoding/json` struct tag é case-insensitive — diferente do `JSON.parse`

> 2026-10-31 · hades-tf · red team ML-2A (REQ-2026-08-30, débitos de higiene)

## O fato não óbvio

`encoding/json` em Go faz correspondência de tag de campo **insensível a maiúsculas**. Com chaves
duplicadas de caso diferente, o **último valor vence**.

```go
type V struct { Version string `json:"version"` }
// input: {"version":"0.0.1","Version":"9.9.9"}
// resultado: v.Version = "9.9.9"  ← o campo maiúsculo vence
```

`JSON.parse` em JavaScript/Node.js é **case-sensitive**: no mesmo input, `obj.version = "0.0.1"`.

## Onde isso aparece neste repositório

`extractPackageLockRootVersion` e `extractPackageLockPackagesVersion` em `internal/commands/release.go`
(adicionados em `72aac048`) usam o mesmo padrão de struct unmarshal. `extractNpmVersion` (pré-existente)
também usa struct unmarshal — o padrão é pré-existente, os novos extratores o propagam.

## Vetor exploitável (severidade LOW)

Lockfile editado manualmente com `"version":"1.0.0-wrong","Version":"1.0.0-right"`:
- `release tag` (Go) lê `"1.0.0-right"` e aceita.
- npm real lê `"1.0.0-wrong"` e enxergaria divergência.

O vetor requer adulteração manual do lockfile porque `npm version` nunca gera chaves com V
maiúsculo. Severidade LOW.

## Correção se necessária

Substituir struct unmarshal por decode em `map[string]json.RawMessage` e indexar pela chave exata:

```go
var raw map[string]json.RawMessage
if err := json.Unmarshal([]byte(content), &raw); err != nil {
    return "", err
}
v, ok := raw["version"]  // lookup de map é case-sensitive em Go
if !ok {
    return "", fmt.Errorf(`no "version" field`)
}
var s string
if err := json.Unmarshal(v, &s); err != nil {
    return "", err
}
return s, nil
```

Sem dependências externas. Para estruturas aninhadas (`packages[""].version`), fazer um segundo
unmarshal do valor de `raw["packages"]` para `map[string]json.RawMessage`.

## Regra geral

Toda extração de campo JSON de arquivo que pode ser editado por humanos (lockfile, package.json,
manifesto) deve usar lookup de map, não struct unmarshal — a menos que o campo nunca tenha
variante de caso e isso esteja documentado.
