# yaml.Node bypassa o limitador de aliasing do yaml.v3; todo walker customizado é ilimitado

**Data:** 2026-11-22  
**Descoberto em:** ML-4F (red team ML-4C/4D/4E, REQ-2026-11-22)  
**Severidade:** MÉDIO  
**Estado:** pré-existente em 11.8.0; não corrigido; ML corretivo pendente

---

## O fato central

`yaml.Unmarshal(data, &interface{})` tem um limitador interno no yaml.v3 que rejeita documentos com aliasing excessivo ("document contains excessive aliasing"). Esse limitador **não existe** no modo `yaml.Node`: decodificar para `*yaml.Node` e depois percorrer a árvore manualmente significa que qualquer lógica de fan-out (alias bomb, merge bomb) roda sem nenhum limite automático.

Consequência direta: **todo walker Go que usa a API `yaml.Node` precisa de seu próprio orçamento de visitas**, ou é vulnerável a DoS por qualquer YAML com aliasing profundo.

---

## Onde isso aparece no trackfw

### Leitor — `internal/config/config.go`

Funções `normalizeMapping`, `normalizeNode`, `applyMergeNode`, `applyMergeMapping`: usam `yaml.Node`. Sem orçamento de visitas.

Bomb de sequência de 30 camadas (`<<: [*n, *n]`): `validate` trava em >30 s. Pré-existente em 11.8.0 (medido com `/usr/bin/time`).

### Escritor — `internal/generators/scaffold.go`

Funções `expandTopLevelKeys`, `expandMergeValue`: também usam `yaml.Node`. Introduzidas / expandidas em ML-4C.

Mesmo bomb: `init` trava em >30 s. Pré-existente em 11.8.0 via leitor; writer path é nova regressão de ML-4C.

### Guard write-scope — `internal/guard/writescope.go`

Protegido: `ParseWriteScopeConfig` chama `yaml.Unmarshal(_, &interface{})` como primeiro passo, que tem o limitador interno. Rejeita a bomb em <15 ms.

### Guard credential e guard git-branch

Não usam yaml.Node. Credential usa varredura de texto; git-branch usa apenas `os.Stat`. Ambos imunes.

---

## A armadilha de leitura

Código que faz:

```go
var root yaml.Node
if err := yaml.Unmarshal(data, &root); err != nil {
    return err
}
// percorrer root recursivamente...
```

...passa pela decodificação do yaml.v3 sem problema mesmo com um bomb, porque o decode em modo `*yaml.Node` **não** aplica o limitador. O limitador só entra quando o destino é `interface{}` ou uma struct.

---

## Correção

**Memoização por ponteiro de nó** (preferida): cache `map[*yaml.Node][]string` ou `map[*yaml.Node]interface{}` no nível de chamada de topo. Na segunda visita ao mesmo ponteiro, usa o cache em vez de re-expandir. Converte O(2^n) em O(n) sem perda de correção.

**Budget de visitas** (alternativa): `budget := 50_000` passado como `*int`. Cada nó visitado decrementa; quando zero, retorna sem expandir. Mais simples, mas pode cortar YAMLs legítimos muito ramificados.

---

## Regra geral

> Se o walker usa `yaml.Node` em vez de `interface{}`, adicione explicitamente um orçamento de visitas ou memoização. Não assuma que o yaml.v3 limitará o fan-out por você.

---

## Referências

- `docs/seguranca/2026-11-22-red-team-onboarding-corretivos.md` — evidências e medições
- `internal/config/config.go:656–793` — normalizeMapping/normalizeNode
- `internal/generators/scaffold.go:927–1046` — expandTopLevelKeys/expandMergeValue
