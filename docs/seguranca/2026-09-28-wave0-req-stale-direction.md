# Wave 0 — ML-0A: medir o que NÃO precisa ser construído (REQ-2026-09-28)

**Data:** 2026-09-28  
**Revisor:** hades-tf  
**Branch:** `fix/roadmap-para-req-sem-tratamento-de-stale`

---

## 1. Completude da enumeração de superfícies

A REQ define três superfícies:

| Superfície | Descrita na REQ? | Medida aqui? |
|---|---|---|
| Direção Roadmap → REQ: `referenceExists` cru vs `resolveRoadmapRefStatus` com fallback | Sim | Sim |
| AC4: cobertura parcial de `ref_targets_exist` (só wip+blocked) | Sim | Sim |
| AC2: ramo de ambiguidade do `default:` | Sim | Sim |

**Superfície não listada identificada durante medição:**

- A função `FindRoadmapLinkingREQ`, usada em `chainRoadmapForREQ` (req.go:226), busca roadmap por basename ao criar uma segunda REQ com mesmo título para outro agente — e vincula a segunda REQ ao roadmap do primeiro agente. Isso é o mecanismo do #452 manifestando-se na **direção de criação**, não só de validação. Por Regra Dura de Causa Raiz, é ML desta REQ, não outra REQ.

- A Wave 0 gate do roadmap é cega no único modo onde colisões são possíveis (ver seção 2 — ameaças do implementador).

---

## 2. Modelo de ameaça — o adversário é o implementador otimista

### Ameaça A: literal mirror de `resolveRoadmapRefStatus` não fecha o #452

O predicado de entrada do mecanismo existente é `isStaleRoadmapStateRef(ref)`:

```go
// validator.go:3300
func isStaleRoadmapStateRef(ref string) bool {
    dir := filepath.Base(filepath.Dir(filepath.ToSlash(ref)))
    return agentNamespaceStateNames[dir]   // true só para {backlog,analyzing,wip,blocked,done,abandoned}
}
```

Para o ref do #452 — `docs/requisicoes/hefesto/REQ-X.md`:
- `filepath.Base(filepath.Dir(ref))` = `"hefesto"` → **não está em `agentNamespaceStateNames`**
- `isStaleRoadmapStateRef` retorna `false`
- **fallback bloqueado — resolve retorna nil — mesma mensagem "does not exist"**

Um implementador que escrever `resolveREQRefStatus` com o mesmo predicado `isStaleRoadmapStateRef` entregará código que:
1. Compila
2. Passa todos os testes atuais
3. Passa o Scenario 25 (`check-gates-falsify.sh`)
4. NÃO corrige o #452

**O que distingue correto de errado aqui:** o predicado para REQs deve ser `filepath.Dir(ref) != "."` (o ref tem qualquer componente de diretório), não `agentNamespaceStateNames[dir]`. A razão: REQs não "movem de estado", elas mudam de **localização dentro da árvore** — o que pode ser qualquer subdiretório, não necessariamente um nome de estado reservado.

O Scenario 25 é preservado com o predicado correto: `ref = "REQ-flag-source.md"` tem `filepath.Dir(ref) == "."` → fallback bloqueado → violação reportada como hoje.

### Ameaça B: implementador não escreve teste para o caso `hefesto/` vs `hefesto/backlog/`

A REQ especifica AC5 (falsificação nas duas direções), mas não nomeia o caso onde o ref gravado tem `agente/` como parent (não um estado). Um teste de AC5 que só exercita `docs/req/wip/REQ-X.md` como stale pode passar com o predicado errado (`isStaleRoadmapStateRef`). O teste correto usa `docs/req/hefesto/REQ-X.md` como stale.

### Ameaça C: Wave 0 gate passa vacuamente em `by_agent` — cobre a condição só no corpus errado

O gate do roadmap é:
```bash
n=$(git ls-files 'docs/req/*.md' | xargs -n1 basename | sort -u | wc -l | tr -d ' ')
t=$(git ls-files 'docs/req/*.md' | wc -l | tr -d ' ')
test "$n" = "$t" && echo "Gate W0: zero colisao"
```

O glob `docs/req/*.md` NÃO alcança subdiretórios. Medido:

```
# Projeto by_agent com docs/req/hades/REQ-X.md e docs/req/apolo/REQ-X.md
$ ls docs/req/*.md
 docs/req/REQ-2026-09-01-flat.md     ← só REQ flat é listada

$ git ls-files 'docs/req/*.md'       ← idem
```

Em projeto `by_agent` puro (sem REQs flat), o gate retorna `t=0, n=0` → `"0" = "0"` → **passa vacuamente**. A condição que o gate existe para vigiar é inalcançável com o glob atual no único modo onde é possível.

O gate foi escrito para este repo (que é `flat`). Para consumidores `by_agent`, o glob correto seria `docs/req/**/*.md`.

---

## 3. Pergunta 1 — o ramo de ambiguidade é alcançável?

### Veredito: SIM — em `roadmap_namespacing: by_agent` com 2+ agentes

**Medição realizada (não leitura de código):**

Projeto temporário em `mktemp -d` com `roadmap_namespacing: by_agent`, agentes `hades` e `apolo`. Binário `tf-0a` compilado da branch atual.

```
$ tf-0a req new "same title test" --agent hades
created docs/req/hades/REQ-2026-09-28-same-title-test.md

$ tf-0a req new "same title test" --agent apolo
created docs/req/apolo/REQ-2026-09-28-same-title-test.md
```

Dois arquivos com **basename idêntico** em namespaces distintos. O `req new` não verifica colisões de basename entre namespaces antes de criar.

**Condição de alcançabilidade:**

A alcançabilidade é **condicional ao predicado que Wave 1 escolher**:

- Com predicado `filepath.Dir(ref) != "."` (correto para #452): ambos os arquivos são encontrados em fallback quando o ref original falha → `len(found) > 1` → `default:` → "ambiguous".
- Com predicado `isStaleRoadmapStateRef` (mirror literal, errado para #452): o fallback nunca roda para o #452 — o ramo de ambiguidade não é alcançado, mas porque o mecanismo inteiro está inerte.

**O ramo `default:` entra na Wave 1**, mas sua especificação deve ser solidária ao predicado correto. Não é suficiente dizer "o ramo entra" sem especificar que o predicado tem de ser `dir != "."`.

**Gate W0 deste repo:**

```
Gate W0: 234 REQs, 234 basenames unicos — zero colisao
```

Passa. Este repo é `flat` — sem namespaces de agente, colisão impossível. O gate é de vigilância para este repo. **Não é evidência de que o ramo é inalcançável em geral** — é evidência de que não há colisão no corpus atual deste repo (que é flat).

**Afirmação do arquiteto refutada:** "Provavelmente inalcançável, implementar é dívida disfarçada de segurança." A medição mostra que dois `req new --agent <x>` com mesmo título no mesmo dia produzem o estado de colisão sem fabricação.

---

## 4. Pergunta 2 — AC4: cobertura parcial de `ref_targets_exist`

### Confirmação do silêncio

Projeto temporário com `ROADMAP-wip-broken.md` (wip, req: quebrado) e `ROADMAP-backlog-broken.md` (backlog, req: quebrado). Validate reportou o wip, silenciou o backlog. Confirmado.

### Distribuição por estado neste corpus

```
backlog:   20   ← não coberto hoje
analyzing:  0
wip:        3   ← coberto
blocked:    2   ← coberto
done:     201   ← não coberto hoje
abandoned:  5   ← não coberto hoje
```

### Custo medido (não presumido) para `done` + `abandoned`

Classificação dos 206 roadmaps done/abandoned com campo `req:`:

```
LITERAL_OK (nenhuma violação em nenhuma opção):      149
BASENAME_FOUND (viraria "stale" na Wave 1):            20
NOWHERE (violação real em opção A):                    14
NO_REF (sem campo req:):                               23
Total:                                                206
```

As 14 entradas NOWHERE são: 3 com `req: "~"` (valor inválido), e 11 refs com formato pré-padronização sem prefixo de diretório (ex.: `REQ-2026-07-29-barrier-aceita-wave-com-sufixo-bis`, sem `docs/req/`). São artefatos históricos de quando o campo `req:` não era obrigatório.

As 20 entradas BASENAME_FOUND são refs que apontam para caminhos antigos mas o arquivo existe por basename (REQs que mudaram de localização após o roadmap ser concluído). Na Wave 1, estas virariam "stale" — aviso, não violação.

### Decisão recomendada (baseada na medição)

**Opção B: ampliar para `backlog` (e `analyzing`), declarar `done`/`abandoned` fora**

- **`backlog`**: 20 roadmaps, trabalho futuro não iniciado — acionável. É o estado onde o #452 se reproduz.
- **`analyzing`**: 0 roadmaps agora, zero custo.
- **`done` + `abandoned`**: 14 violações reais seriam emitidas para artefatos históricos que ninguém vai corrigir. Os 14 refs são pré-padronização; emiti-los como violação é ruído permanente que dilui a cobertura do CI.
- **Declarar** que `done`/`abandoned` não são varridos é controle honesto; já a cobertura atual (só wip+blocked) não estava declarada em lugar nenhum.

**Opção A (ampliar para todos os estados)** adicionaria 14 violações de ruído histórico permanente. **Opção C (só declarar, não ampliar)** mantém o defeito silencioso para `backlog` — o estado do #452.

A decisão final é do arquiteto. A análise de custo com números reais está acima.

---

## 5. Alvos de falsificação nas duas direções

| Superfície | Onde entra o dano | Gate que deveria pegar | Direção |
|---|---|---|---|
| Predicado `isStaleRoadmapStateRef` (mirror literal) | `internal/validator/validator.go` — `resolveREQRefStatus` | Scenario 25 (`check-gates-falsify.sh`) | Falso negativo: #452 não é detectado como stale |
| Predicado `dir != "."` sem guarda de basename puro | mesma função | Scenario 25 | Falso positivo: `REQ-flag-source.md` resolveria por basename, tornando Sc25 vácuo |
| backlog silencioso (`ref_targets_exist`) | `validateRefTargetsExist` — `dirs` não inclui `backlog` | `ref_targets_exist` (a própria regra quando expandida) | Falso negativo: roadmap backlog com req: quebrado passa |
| Wave 0 gate com glob `*.md` (nível único) | gate no roadmap | Nenhum gate atual pega isso — é cego por construção | Falso negativo: colisão by_agent não é monitorada |
| `FindRoadmapLinkingREQ` busca por basename em criação | `generators/req.go:chainRoadmapForREQ` | Nenhum gate atual | Falso positivo + cross-namespace wrong link |

---

## 6. Residual declarado

1. A Wave 0 gate (`git ls-files 'docs/req/*.md'`) é cega para `by_agent`. Este repo é flat — o gate é válido aqui. Para consumidores `by_agent`, o glob correto é `docs/req/**/*.md`. Corrigir o gate está fora do escopo desta REQ (é documentação/harness, não produto), mas deve ser nomeado no commit da Wave 1 para rastreabilidade.

2. `done` e `abandoned` ficam fora da cobertura de `ref_targets_exist` intencionalmente (14 violações históricas de pré-padronização). Isso deve ser declarado no contrato (`docs/cli-parity.md`) junto com a ampliação para `backlog`.

3. `FindRoadmapLinkingREQ` cross-namespace é same-mechanism-#452, mas na direção de criação. Por Regra Dura de Causa Raiz, é ML desta REQ. Este Wave 0 não o mede em profundidade — Wave 1 deve inspecioná-lo.

---

## Critérios de aceite — status

- [x] Veredito escrito sobre alcançabilidade do ramo de ambiguidade, com a medição
- [x] Decisão do AC4 (ampliar × declarar), com o custo de cada lado — **medido, não presumido** (14 violações históricas em done/abandoned)
- [x] Nenhuma linha de implementação neste ML

---

## Resumo executivo para a Wave 1

1. **Predicado**: usar `filepath.Dir(ref) != "."`, não `isStaleRoadmapStateRef`. A diferença decide se o #452 é corrigido ou não.
2. **Ramo `default:`**: incluir. Alcançável em by_agent via `req new --agent X` + `req new --agent Y` com mesmo título no mesmo dia.
3. **AC4**: ampliar `ref_targets_exist` para `backlog` (e `analyzing`). Deixar `done`/`abandoned` fora com declaração.
4. **Scenario 25**: não vai vacuo com o predicado `dir != "."` — `REQ-flag-source.md` tem `dir == "."` → fallback bloqueado → violação mantida.
5. **Teste de AC5**: exercitar `docs/req/hefesto/REQ-X.md` como stale (não só `docs/req/wip/REQ-X.md`).
