# Red Team — Corretivos ML-4C a ML-4E (REQ-2026-11-22)

**Agente:** hades-tf  
**Data:** 2026-11-22  
**HEAD:** 7a38fada  
**Branch:** fix/onboarding-verificavel  
**Binário testado:** `$TMPDIR/tf` (go build ./cmd/trackfw HEAD)  
**Binário baseline:** /opt/homebrew/bin/trackfw 11.8.0  
**HOME isolado:** todos os comandos `trackfw` usaram `HOME=/tmp/tmp.Qo68WbDvHu`; nenhum tocou `~`

---

## Veredito

**APROVA COM RESSALVAS.**

Os corretivos ML-4C, ML-4D e ML-4E são funcionalmente corretos: a normalização de chave (ML-4C) e a expansão de merge key (ML-4D) convergem em leitor e escritor para todos os vetores testados; o guard de ciclo (ML-4E) protege contra recursão infinita (ciclos verdadeiros). Nenhum dos três MLs introduz novo vetor bloqueante.

A DoS por alias bomb é **pré-existente** (11.8.0 trava com as mesmas fixtures, confirmado abaixo). ML-4C introduz um segundo caminho de fan-out no escritor (`expandTopLevelKeys`), mas o leitor já era o gargalo — a janela do utilizador não muda. Duas ressalvas abertas precisam de ML corretivo próprio antes de qualquer release que inclua esta branch.

---

## Ressalva R-A — MÉDIO (pré-existente): alias bomb de sequência — DoS em validate e init

### Mecanismo

A expansão de merge key em `normalizeMapping` (leitor, `internal/config/config.go`) e em `expandTopLevelKeys` (escritor, `internal/generators/scaffold.go`) usa semântica de **path** para ciclos: um nó já na pilha de chamadas atual é bloqueado pelo `visiting map[*yaml.Node]bool`. Isso protege contra ciclos verdadeiros (a → b → a), mas **não** contra fan-out exponencial: um nó visitado, retornado e removido do conjunto `visiting` pode ser re-visitado um número ilimitado de vezes a partir de chamadas subsequentes independentes.

A forma explorável é o merge sequencial com um único `<<:` e valor de sequência:

```yaml
n00: &n00
  wip_limit: 1
n01: &n01
  <<: [*n00, *n00]   # um único <<: key; dois *n00 no valor — fan-out 2×
n02: &n02
  <<: [*n01, *n01]
# ... 30 camadas
req_dir: docs/req
adr_dirs:
  - docs/adr
<<: [*n29, *n29]
```

Cada camada duplica o trabalho: 30 camadas = 2^30 ≈ 10^9 visitas ao nó folha.

### Medições — HEAD e baseline

| Níveis | Visitas (est.) | validate HEAD | validate 11.8.0 | init HEAD | init 11.8.0 |
|--------|---------------|---------------|-----------------|-----------|-------------|
| 20     | ~1 M          | 0,13 s        | 0,14 s          | —         | —           |
| 25     | ~33 M         | 2,89 s        | 2,91 s          | —         | —           |
| 30     | ~1 B          | >30 s (rc=124)| >31 s (rc=124)  | >30 s     | >32 s       |

**A baseline 11.8.0 trava com as mesmas fixtures.** O DoS é pré-existente: confirmado com a fixture de sequência (`<<: [*n, *n]`) e com a fixture de valor (alias sem merge key, apontando para a mesma raiz). A DoS no path de valor estava em `normalizeNode` (que percorre a árvore sem limite), pré-ML-4D. Depois de ML-4D, o merge key path (`applyMergeMapping` → `applyMergeNode`) acrescenta um segundo caminho para o mesmo fan-out.

### Escritor — regressão secundária introduzida por ML-4C

Antes de ML-4C, `expandTopLevelKeys` ignorava `<<:` → a bomb de sequência era um no-op no escritor. Depois de ML-4C, a expansão correta de merge key torna a mesma bomb operacional no path do escritor. O leitor (`normalizeMapping`) já era vulnerável antes (pré-existente), então a regressão do escritor é redundante do ponto de vista do utilizador (`init` já travava no baseline via leitor). Contudo, a superfície de fan-out agora é dois caminhos em vez de um.

### Superfícies afetadas e não afetadas

| Superfície | Afetada | Por quê |
|---|---|---|
| `trackfw validate` | Sim (pré-existente) | `parse()` → `normalizeMapping()` → `applyMergeMapping()`: yaml.Node; sem orçamento de visitas |
| `trackfw init` | Sim (pré-existente) | leitor já era vulnerável em 11.8.0; writer path agora também (ML-4C) |
| guard write-scope | NÃO | `ParseWriteScopeConfig()` chama `yaml.Unmarshal(_, &interface{})` como primeiro passo; yaml.v3 nesse modo detecta "document contains excessive aliasing" e retorna erro em <15 ms |
| guard credential | NÃO | `ReadCredentialGuardMode()` usa parsing por texto (varredura de linhas); não usa yaml.Node nem chama parse() |
| guard git-branch | NÃO | `FindProjectRoot()` chama apenas `os.Stat` para verificar existência do arquivo; nunca interpreta o conteúdo |

Saída do write-scope guard com o bomb de 30 camadas (sequência):

```
trackfw guard write-scope: denied: trackfw.yaml: invalid YAML: yaml: document contains excessive aliasing
rc=2, elapsed=11ms
```

Saída do git-branch guard com o bomb de 30 camadas:

```
# guard não lê o conteúdo; retorna resultado em 11ms sem travar
```

### Cadeia linear profunda (1000 camadas `<<: *n`)

| Camadas | Tempo | RSS |
|---------|-------|-----|
| 1000    | 0,11 s | 69 MB |

Cadeia linear é O(n²) no pior caso mas segura na prática até pelo menos 1000 níveis.

### Correção sugerida

**Memoização por ponteiro de nó** — o resultado de expandir um nó é cacheado no primeiro uso:

```go
// expandTopLevelKeys (scaffold.go) e normalizeMapping (config.go):
func expandTopLevelKeys(node *yaml.Node, keys map[string]bool,
    visiting map[*yaml.Node]bool, memo map[*yaml.Node][]string) {
    if visiting[node] { return }
    if cached, ok := memo[node]; ok {
        for _, k := range cached { keys[k] = true }
        return
    }
    visiting[node] = true
    defer delete(visiting, node)
    var added []string
    // ... expandir normalmente; acrescentar cada chave a added ...
    memo[node] = added
}
```

Alternativa mais simples: orçamento de visitas `*int budget = 50_000` passado por referência; quando zerado, retorna sem expandir. Aceitável para YAMLs legítimos (nenhum config real tem >50 k nós alcançáveis via merge).

---

## Ressalva R-B — BAIXO (novo, pré-existente ADR D9): credential guard ignora `mode:` via âncora

### Mecanismo

`ReadCredentialGuardMode` percorre linhas do arquivo texto buscando `credential_guard:` e depois `mode:`. Um campo `mode: block` fornecido via âncora (não escrito literalmente no bloco `credential_guard:`) não é detectado pelo scanner de texto.

### Medição

```yaml
strict: &strict
  mode: block
credential_guard:
  <<: *strict
```

Resultado HEAD:

```
# guard usa modo padrão "warn" — não ativa block
# credencial JWT: rc=1 (warn), não rc=2 (block)
```

Resultado 11.8.0: idêntico. Comportamento pré-existente; declarado como residual no ADR D9.

### Decisão

Pré-declarado em ADR D9. Não é regressão introduzida por esta branch. Registrado como residual confirmado.

---

## Target 1 — Divergência leitor × escritor

### Vetores testados

| Vetor | Leitor (validate) | Escritor (init não adiciona) | Divergência |
|-------|------------------|------------------------------|-------------|
| T1-A: `<<: *s` onde `s` é escalar | ignora merge (ScalarNode cai fora do switch) | ignora merge | Nenhuma |
| T1-B: `<<: *s` onde `s` é sequência de escalares | ignora (cada elem é ScalarNode) | ignora | Nenhuma |
| T1-C: `!!merge <<: *a` (tag explícita) | lê como !!merge, expande | lê como !!merge, expande | Nenhuma |
| T1-D: `<<: *a` repetido (dois `<<:`) | ambas expandidas, primeira vence (duas passagens) | ambas expandidas | Nenhuma |
| T1-H: merge dentro de merge (anchor com `<<` próprio) | expandido recursivamente | expandido recursivamente | Nenhuma |
| T1-I: `<<:` com mapping inline (sem alias) | expandido (kind=MappingNode) | expandido | Nenhuma |
| T1-J: chave com escape (`"wip_\x6cimit"`) | decodifica via yaml.Node.Value | normalizeYAMLTopLevelKey usa parser → decodifica | Nenhuma |
| T1-G: YAML multi-documento (`---`) | erro "malformed YAML" | detecta erro antes de escrever, não modifica o arquivo | Consistente (ambos falham) |
| T1-E: chave número (`42:`) | `result["42"] = ...` | `keys["42"] = true` | Nenhuma |
| T1-F: chave complexa (`? [a,b]:`) | `result[""] = ...` (kind=SequenceNode, Value="") | `keys[""] = true` | Nenhuma (ambos silenciosamente usam "") |

**Chave complexa (T1-F)**: tanto o leitor quanto o escritor usam `k.Value` em nós que não são escalares; para uma SequenceNode isso retorna `""`. Essa key vazia não conflita com nenhuma chave de governança. Comportamento pré-existente; não alterado pelos MLs.

**Multi-documento (T1-G)**: o leitor retorna erro explícito ("malformed YAML") e rc≠0. O init detecta o mesmo erro antes de modificar o arquivo. Nenhum rebaixamento de governança possível porque o config não é lido.

**Resultado**: nenhuma divergência leitor × escritor encontrada.

---

## Target 2 — Relaxamento de governança via âncoras

### Teste inicial — data fora do horizonte (vacuous)

A primeira iteração usou `lenient_until: "2099-12-31"`. Essa data ultrapassa `LenientHorizonDays` e o validador trata a governança como strict para ambas as comparações. As saídas eram idênticas porque o modo lenient nunca era ativado — o teste não media o que pretendia.

### Redo com data próxima (lenient_until: 2026-12-02)

```yaml
lenient: &lenient
  governance_mode: lenient
  lenient_until: "2026-12-02"
  wip_limit: 100
req_dir: docs/req
adr_dirs:
  - docs/adr
<<: *lenient
```

| Versão | Saída | rc |
|--------|-------|----|
| HEAD (7a38fada) | `[LENIENT MODE] ... 7 warning(s)` | 0 |
| Baseline 11.8.0 | `✗ ... Error: 6 violation(s) found` | 1 |

HEAD activa corretamente o modo lenient a partir do `governance_mode` fornecido via âncora. A baseline ignora o campo porque não expandia `<<:`.

### Chave explícita vence sobre chave herdada (duas passagens de normalizeMapping)

```yaml
lenient: &lenient
  governance_mode: lenient
  lenient_until: "2026-12-02"
  wip_limit: 100
req_dir: docs/req
adr_dirs:
  - docs/adr
governance_mode: strict      # explícita — deve vencer
<<: *lenient
```

Resultado HEAD: rc=1 (strict), violações como erros. A passagem 1 de `normalizeMapping` recolhe as chaves explícitas antes que a passagem 2 preencha lacunas via merge. `governance_mode: strict` é explícita → vence sobre `governance_mode: lenient` herdada.

### T3-E — ciclo via sequência dentro de mapping dentro de merge

```yaml
a: &a
  list:
    - *a
<<: *a
```

Resultado HEAD: validate 0,04 s, 16 MB RSS, rc=0, sem trava.

`applyMergeNode(*a)` resolve o alias → MappingNode `a` → `applyMergeMapping(a)`: visiting[*a]=true; ao normalizar o valor de `list`, `normalizeNode` encontra `*a` → visiting[*a] = true → retorna nil. Ciclo capturado corretamente pelo guard de ML-4E.

### Veredito T2

**Sem bypass não intencional.** O comportamento novo (ler `governance_mode` via âncora) é mais correto que a baseline (que ignorava). A precedência explícita-sobre-herdada está correta.

---

## Target 3 — Ciclos e alias bomb

### T3-A: ciclo via sequência direta em merge

```yaml
x: &x
  - *x
<<: *x
```

Resultado: validate rc=0, sem trava. `expandMergeValue` detecta o SequenceNode, entra no visiting, itera o elemento `*x` → resolve para o mesmo SequenceNode → `visiting[resolved]` = true → retorna. Protegido.

### T3-B: bomb com `<<:` duplicado

Dois `<<:` keys duplicados. `ParseWriteScopeConfig` detecta "mapping key '<<' already defined" em 11 ms e retorna erro (fail-closed para write-scope). `validate` trava no mesmo mecanismo de fan-out (R-A).

### T3-C: bomb de sequência 30 camadas — resultado principal

Ver Ressalva R-A. Pré-existente em 11.8.0; ambas as versões travam em >30 s com 30 camadas.

### T3-D: `init` com bomb de 30 camadas

Mesmo mecanismo; pré-existente em 11.8.0 (medido: wall=32 s baseline, 30 s HEAD).

### T3-E: ciclo via sequência dentro de mapping dentro de merge

Ver Target 2 secção T3-E acima: 0,04 s, protegido.

---

## Target 4 — Fail-open em guard

### T4-A: write-scope com `<<:` duplicado

```
rc=2, denied: "mapping key '<<' already defined"
elapsed=11ms
```

Fail-closed.

### T4-B: write-scope com bomb de sequência 30 camadas

```
rc=2, denied: "yaml: document contains excessive aliasing"
elapsed=11ms
```

`yaml.Unmarshal(_, &interface{})` tem limite interno de aliasing no yaml.v3. Fail-closed.

### T4-C: write-scope — condições de erro diversas

| trackfw.yaml | write-scope | Comportamento |
|---|---|---|
| Sequência no topo (não mapping) | rc=0, permite | Pré-existente; sem frozen paths declarados, write é permitido |
| Vazio | rc=0, permite | Pré-existente; idem |
| `frozen_paths` via âncora em `write_scope` | rc=2, **denied** | Merge key lida corretamente pelo `parse()` → `normalizeMapping()` — ganho de ML-4D |

### T4-D: guard credential com bomb de sequência

| Arm | Configuração | Resultado | Tempo |
|-----|-------------|-----------|-------|
| A | `mode: block` literal | rc=2 (bloqueado) | 11 ms |
| B | `mode: block` via âncora (`<<: *strict`) | rc=1 (warn, default) | 11 ms |
| Bomb | `<<: [*n29, *n29]` 30 camadas | rc=1 (warn, default) | 11 ms |

O guard credential usa varredura de texto; não invoca yaml.Node. A bomb é inerte. Arm B (R-B) é pré-existente.

### T4-E: guard git-branch com bomb de sequência

Resultado: 11 ms, retorno normal. `FindProjectRoot` chama apenas `os.Stat`; nunca lê o conteúdo do arquivo.

---

## Tabela de residual declarado

| ID | Descrição | Severidade | Decisão |
|---|---|---|---|
| R-A | Alias bomb de sequência em `validate` e `init` — DoS; pré-existente em 11.8.0; writer path agora também afetado (ML-4C) | Médio | Pré-existente; ML corretivo (memoização ou orçamento de visitas) necessário antes de release |
| R-B | `ReadCredentialGuardMode` não vê `mode:` fornecido via âncora (`<<:`) | Baixo | Declarado em ADR D9; comportamento pré-existente; confirmado idêntico em HEAD e baseline |
| R-C | Chave complexa (`? [a, b]:`) silenciosamente vira chave vazia `""` | Baixo | Pré-existente; nenhuma chave de governança tem esse nome; comportamento consistente em leitor e escritor |
| R-D | Multi-documento YAML rejeitado por ambos; init não modifica o arquivo | Info | Comportamento defensivo correto |

---

## Checklist de segurança HOME

Todos os comandos `trackfw` foram executados com:

```bash
env HOME=/var/folders/.../tmp.Qo68WbDvHu \
    USERPROFILE=/var/folders/.../tmp.Qo68WbDvHu \
    CODEX_HOME=/var/folders/.../tmp.Qo68WbDvHu/.x \
    XDG_CONFIG_HOME=/var/folders/.../tmp.Qo68WbDvHu/.c \
    <binário>
```

Nenhum comando foi executado com `HOME=/Users/kgsaran`.

---

## Artefato de teste Go

Arquivo `internal/generators/zz_redteam4f_test.go` criado (7 testes: T1-A/B/J, T1-I, T2, T3-self-ref, T3-bomb20), executado com:

```bash
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m -run 'TestRedTeam4F' ./internal/generators/
# → ok 1.039s
```

Arquivo **apagado** após a execução. Confirmação:

```
$ git status --porcelain
(saída vazia — árvore limpa exceto pelo relatório untracked)
```
