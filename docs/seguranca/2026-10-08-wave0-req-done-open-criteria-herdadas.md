---
date: 2026-10-08
author: hades-tf
roadmap: ROADMAP-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md
wave: 0
ml: ML-0A
---

# Wave 0 — Threat Model: req_done_open_criteria — recorte de herdadas

REQ: `docs/req/REQ-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md`

Corpora de medição:
- `$S/fork` — clone local de `kgsaran/trackfw`, remote `upstream` apontando para o mesmo repo (upstream == origin)
- `$S/shallow2` — clone real shallow via `file://` (confirmado: `git rev-parse --is-shallow-repository` = `true`)
- `$S/reporter` — `gh repo clone lourivalgarciajunior/trackfw` (fork real; remote `upstream` adicionado pelo `gh`, aponta para `git@github.com:kgsaran/trackfw.git`)
- `git version 2.54.0`; todos os comandos usam `git -C <projectRoot>` para isolar o CWD.

---

## 1. Enumeração de superfícies

### 1.1 Onde a linha agregada nasce

Arquivo único: `internal/validator/validator_req_done_criteria.go`.

Função geradora: `reqDoneOpenCriteriaNotice(exempt, enforced, scanned int) string` (linha 193).
Formato atual:
```
req_done_open_criteria: N Done REQ(s) with open criteria exempt as created before cutoff 2026-10-04, M enforced, P Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)
```

A função é chamada por `reqDoneOpenCriteriaAlwaysWarn` (linha 203), que retorna `nil` quando `scanned == 0`.

### 1.2 Dois sítios de emissão — paralelos, não sequenciais

`validateREQDoneOpenCriteria()` é chamada em dois sítios:

| Sítio | Função hospedeira | Linha | Rota |
|---|---|---|---|
| A | `ValidateUnfiltered()` | 910 | aviso `[]string` — chamada exclusivamente por `baseline.go` |
| B | `validateUnfilteredTagged()` | 1364 | aviso `[]TaggedMsg` — chamada por `Validate()` e `ValidateTagged()` |

`Validate()` (texto) e `ValidateTagged()` (`--json`) ambos chamam `validateUnfilteredTagged()` — sítio B. Uma execução de `trackfw validate` invoca apenas sítio B. Uma execução de `trackfw baseline update` invoca apenas sítio A.

**Consequência para ML-1A:** modificar `validateREQDoneOpenCriteria()` e `reqDoneOpenCriteriaNotice()` propaga para ambos os sítios via uma única alteração. O git call dentro de `validateREQDoneOpenCriteria()` executa O(1) vezes por invocação. ✓

**Consequência para os testes de AC2/AC4:** os testes existentes usam `validateD4Fixture` → `ValidateUnfiltered()` (sítio A). Os novos testes de herança devem usar `ValidateTagged()` (sítio B), que é o caminho do CLI. Sem isso, o teste prova apenas que o `baseline update` propaga o recorte; não que `trackfw validate` o faça.

### 1.3 Quem consome a linha agregada

**Saída texto (`trackfw validate`):** warnings impressos no terminal. Nenhum gate de shell pina o texto (`grep -r "Done REQ(s) with open criteria" scripts/` = vazio; confirmado por `docs/cli-parity.md` linha 660: `partial=nenhum gate de shell exercita a regra pela superfície do CLI`).

**Saída JSON (`trackfw validate --json`):** `ValidateResult.Warnings []RuleItem`; cada item tem `rule`, `file`, `message`. `extractFile` (result.go:40) extrai o primeiro token `"…"` da mensagem como campo `file`. A linha atual não tem aspas duplas → `file: ""`. **Restrição de formato para ML-1A:** a parentética não pode conter aspas duplas.

**Filtro de baseline (`filterBaselineTagged`):** chamado por `Validate()` (linha 1162) e `ValidateTagged()` (linha 1628) depois que `validateUnfilteredTagged()` retorna. Filtra warnings por `warnSet[w.Msg]` — texto exato. A linha agregada de `req_done_open_criteria` passa por este filtro. Ao alterar o texto (adicionando parentética), um baseline atualizado num ambiente não casa no outro. Exit code permanece 0 (warnings não alteram exit code — confirmado em result.go). Ver §5.4.

**`trackfw serve`:** não lê resultados de `validate` (`grep "req_done_open_criteria" internal/commands/serve.go` = vazio).

**Testes que pinam o formato** (`validator_req_done_criteria_test.go`): todos usam `strings.Contains`. Substrings pinados: `"N Done REQ(s) with open criteria exempt"`, `"N enforced"`, `"N Done REQ(s) scanned"`, `reqDoneOpenCriteriaCutoff`, `reqDoneOpenCriteriaNoticeSubstr`. Nenhum invalidado pela adição de `(K inherited from upstream/main)` após `exempt`. ✓

**`docs/cli-parity.md` linhas 677–680:** pina o texto literal com valores `126/211`; atualizar em ML-1A (AC5).

### 1.4 Enumeração fechada

`grep "Done REQ(s) with open criteria"` retornou 3 arquivos (fonte, testes, cli-parity). `grep "req_done_open_criteria"` retornou 14 — todos documentação, ADRs, roadmaps ou sítios de chamada mapeados acima. Vault: nenhuma nota prévia sobre `req_done`, `herdad` ou `upstream fork`.

---

## 2. Discriminante de "herdada" — medição

### 2.1 Discriminante derivado (opção a da REQ)

**Mecanismo — 5 chamadas git locais:**

1. `git -C <root> config remote.upstream.url` — exit ≠ 0: sem parentética (AC3)
2. `git -C <root> config remote.origin.url` — para comparação T1
3. `git -C <root> rev-parse --verify --quiet <ref>^{commit}` — exit 0 = ref existe; exit 1 = não existe (sem stderr, sem rede)
4. `git -C <root> show <ref>:trackfw.yaml` — ler `req_dir` do upstream; fallback `"docs/req"` se erro
5. `git -C <root> ls-tree -r -z --literal-pathspecs --name-only <ref> -- <upstream-req-dir>` — basenames das REQs

Todas chamadas locais; nenhuma rede.

**Custo medido (sequência completa, fork do relator):**
```
real  0m0.037s   # 5 passos em sequência
```
Por passo: `git config` ~0ms; `git rev-parse` ~2ms; `git show` blob ~5ms; `git ls-tree` (251 arquivos) ~8ms.

**Por que basename e não caminho completo — medição no fork real:**

O fork do relator usa `req_dir: docs/requisições/` (com UTF-8) e o upstream usa `req_dir: docs/req/`.

```
git ls-tree upstream/main -- docs/requisições/  →  0 arquivos  (path match falha)
basename match (após ler req_dir do upstream)   →  28 arquivos correspondentes
```

Basenames eliminam a diferença de `req_dir`, de separadores Windows e de layouts por-estado.

**Concordância com discriminante declarado (`upstream_origin`):**

O fork do relator tem 28 arquivos com campo `upstream_origin`. Todos os 28 têm basename que aparece em `upstream/main` — **concordância 100%**. O discriminante derivado é equivalente ao declarado sobre o corpus real.

**K real no fork do relator (estado atual do clone):**

```
Done REQs: 53/84 totais
Exempt (Done + pré-cutoff + critério aberto): 4
K (herdadas de upstream, entre as 4 isentas): 3
```

O relator reportou 22 na issue; o fork avançou desde o relato (REQs fechadas). A diferença confirma que K é dinâmico. A correspondência 28/28 de `upstream_origin` vs basenames é o dado estrutural que valida o discriminante.

**Vetores medidos:**

| Vetor | Configuração | Resultado | exit | Comportamento |
|---|---|---|---|---|
| V1 | `refs/remotes/upstream/HEAD` → ramo de feature | N arquivos extra | 0 | Não usar HEAD diretamente |
| V2 | `refs/remotes/upstream/main` | 251 REQs, ~8ms | 0 | Correto ✓ |
| V3 | Sem remote `upstream` | `git config` falha | 128 | Sem parentética — AC3 ✓ |
| V4 | upstream == origin | 251/252 "herdadas" | 0 | Falso positivo — ver §3 T1 |
| V5a | Clone shallow (`file://`, `is-shallow=true`), sem fetch | `rev-parse` falha | 128 | Sem parentética ✓ |
| V5b | Clone shallow + `git fetch --depth 1 upstream` | 251 REQs, ~8ms | 0 | `ls-tree` opera sobre tree objects ✓ |
| V6 | Ref mais antigo (243 REQs) | 243 REQs | 0 | Erra para "é seu" — seguro ✓ |
| V7a | `upstream/HEAD` em clone `file://` | HEAD atual do source | 0 | Não confiável para repos locais |
| V7b | `upstream/HEAD` em remote GitHub | Ramo padrão | 0 | Confiável para GitHub; usar como fallback |

**Nota V5a:** `git clone --depth 1 /path/to/repo` ignora `--depth` em paths locais (aviso impresso no stderr: `--depth is ignored in local clones; use file:// instead`). Para clone shallow genuíno: `git clone --depth 1 "file:///path/to/repo"`. Confirmar com `git rev-parse --is-shallow-repository`.

**exit 0 + stdout vazio:** `git ls-tree` sobre ref válido mas path inexistente no upstream retorna exit 0 e stdout vazio. Implementação deve ler `req_dir` do upstream via `git show` antes de chamar `ls-tree` — não interpretar vazio como fallback de busca.

### 2.2 Discriminante declarado (opção b)

Frontmatter `upstream_origin`. Presente em 28/28 REQs do fork do relator — concordância 100% com os basenames. Exige campo novo no esquema + manutenção manual. Não recomendado.

### 2.3 Recomendação

**Opção (a) — derivado.** ~37ms total. Sem campo novo. Concordância 100% com declarado sobre corpus real. Degradação segura em todos os cenários testados.

---

## 3. Threat Model

### Adversário

O implementador apressado que usa "herdada" para mascarar dívida própria. O aviso nunca bloqueia. O risco é evasão de leitura, não de gate.

### T1 — Remote `upstream` apontando para o próprio repositório

**Como:** `git remote add upstream <origin-url>`. Custo zero.

**Efeito medido:** 251 de 252 REQs locais aparecem como "herdadas" (1 exclusiva do ramo de feature não estava em main).

**Detecção:** comparação de string `upstreamURL == originURL` antes de chamar `ls-tree`. Iguais → sem parentética.

**Residual:** URLs logicamente equivalentes mas textualmente diferentes. Ver §5.1.

### T2 — Colisão intencional de basename

**Como:** fork cria REQ com mesmo nome de uma do upstream.

**Efeito:** basename match a classifica como "herdada". Enforcement inalterado para REQs pós-cutoff.

### T3 — Fetch antigo

REQs novas no upstream após o último fetch não aparecem em K. Erra para "é seu" — direção segura.

---

## 4. Alvos de falsificação nas duas direções

### 4.1 K inflado (falso positivo de herança)

| Cenário | Mecanismo | Gate |
|---|---|---|
| upstream == origin (T1) | Mesma URL | Comparação de URL fecha caso direto |
| Colisão de basename (T2) | Mesmo nome, conteúdo diferente | Resíduo §5.2 |
| `upstream/HEAD` → ramo de feature (V1) | HEAD ≠ main | Cadeia main→master evita |
| REQ pós-cutoff com basename compartilhado | Contada em K; mas aparece em `enforced` também | Enforcement inalterado |

### 4.2 K deflado (falso negativo de herança)

| Cenário | Mecanismo | Consequência |
|---|---|---|
| Sem remote `upstream` (V3/V5a) | refs ausentes | Sem parentética — AC3 ✓ |
| Fetch antigo (V6) | REQs novas do upstream ausentes | Erra para "é seu" — seguro |
| `req_dir` do upstream não detectável (§5.3) | Fallback `docs/req` pode não casar | K=0 com parentética — seguro |

### 4.3 Baseline instável por fetch

Tratado em §5.4. Não altera exit code. Design constraint para ML-1A.

---

## 5. Residual declarado

**§5.1 — URLs logicamente equivalentes:** `git@github.com:org/repo.git` vs `https://github.com/org/repo` — mesmo repositório, comparação de string não detecta. Atacante precisa de acesso de escrita ao fork. Aceitável: o aviso não bloqueia.

**§5.2 — Colisão intencional de basename:** REQ local com mesmo basename que uma do upstream. Não detectável sem comparar blobs (custo proibitivo). Efeito visual; enforcement inalterado.

**§5.3 — Upstream sem `trackfw.yaml`:** `git show` falha; fallback `"docs/req"`. Se o upstream usa outro `req_dir`, K=0 com parentética. Erra para "é seu" (seguro).

**§5.4 — Baseline instável por fetch (design constraint):** K depende de `refs/remotes/upstream/*` local, não de conteúdo commitado. O mesmo commit produz textos diferentes em CI (sem upstream → sem parentética) e dev local (com upstream → com parentética e K). `filterBaselineTagged` usa match exato → a linha flutua entre "no baseline" e "net-new" a cada `git fetch upstream`, com exit code 0 em ambos os casos. **Instrução para ML-1A:** documentar em `docs/cli-parity.md` (AC5) que baselines são por-ambiente quando `upstream` está configurado. Decisão arquitetural opcional: strip da parentética ao salvar no baseline — adiar para ML-1A.

**§5.5 — `enforced` não decomposto:** REQs herdadas pós-cutoff com critério aberto continuam em `enforced` sem indicação de herança. Escopo negativo da REQ. Aceitável.

**§5.6 — `upstream/HEAD` em repositórios locais:** após `git fetch` sobre `file://`, `upstream/HEAD` reflete o HEAD atual do source (pode ser feature branch). Para remotes GitHub, é o ramo padrão. A cadeia `main → master` é mais portátil.

---

## 6. Especificação mandatória para ML-1A

### 6.1 Lógica de detecção

```
1.  upstreamURL = git -C <root> config remote.upstream.url
    → exit ≠ 0: sem parentética — AC3

2.  originURL = git -C <root> config remote.origin.url
    → erro: pular verificação T1
    → se upstreamURL == originURL: sem parentética (T1)

3.  Para ref em [refs/remotes/upstream/main, refs/remotes/upstream/master]:
    a. git -C <root> rev-parse --verify --quiet <ref>^{commit}
       → exit ≠ 0: próximo da lista
    b. git -C <root> show <ref>:trackfw.yaml  →  parse req_dir; fallback "docs/req"
    c. git -C <root> ls-tree -r -z --literal-pathspecs --name-only <ref> -- <upstream-req-dir>
    d. Basenames (NUL-delimitado); pode ser vazio → K=0 válido
    e. Usar este ref; parar

4.  Nenhum ref na lista resolveu:
    → tentar refs/remotes/upstream/HEAD
    → se resolve: passos b–d
    → se não: parentética "(upstream tried main, master: ref unresolvable)" (sem aspas duplas)
```

K = `len(intersection(upstreamBasenames, localExemptBasenames))` via `map[string]struct{}`. K exibido mesmo quando K=0. Nenhuma aspa dupla na parentética (restrição `extractFile`).

### 6.2 Formato da linha

**Sem upstream OU upstream == origin (AC3 — byte-idêntico):**
```
req_done_open_criteria: N Done REQ(s) with open criteria exempt as created before cutoff 2026-10-04, M enforced, P Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)
```

**Com upstream resolvido (K qualquer):**
```
req_done_open_criteria: N Done REQ(s) with open criteria exempt as created before cutoff 2026-10-04 (K inherited from upstream/main), M enforced, P Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)
```

onde `upstream/main` é o nome do ref efetivamente resolvido.

**Com upstream configurado mas ref não resolvível:**
```
req_done_open_criteria: N Done REQ(s) with open criteria exempt as created before cutoff 2026-10-04 (upstream tried main, master: ref unresolvable), M enforced, P Done REQ(s) scanned (cutoff declared in internal/validator/validator_req_done_criteria.go)
```

### 6.3 Campo `--json`

Sem mudança estrutural em `ValidateResult`. `warnings[].file` permanece `""`. AC2 satisfeito pela propagação via `applyRuleWarnOnlyTagged`.

### 6.4 Testes obrigatórios

Todos os novos testes de AC2/AC4 devem usar `ValidateTagged()` (sítio B — caminho do CLI), não `validateD4Fixture`.

**AC3:** fixture sem remote `upstream` → aviso byte-idêntico ao atual.
Reconciliação: "sem upstream, saída idêntica à atual".

**AC4a — herança por basename com req_dir divergente (caso principal):**
- Fixture upstream: `trackfw.yaml` com `req_dir: docs/req/`, 2 REQs pré-cutoff com critério aberto.
- Fixture fork: `req_dir: docs/requisições/` (diferente do upstream), 3 REQs (2 com mesmo basename do upstream, 1 nova local).
- Verificar: K=2. A REQ local (basename único) não aparece em K.
- Reconciliação: "basename discriminant conta herdadas ignorando diferença de req_dir".

**AC4b — T1 (upstream == origin):**
- Fixture com `remote.upstream.url == remote.origin.url`.
- Verificar: sem parentética.
- Reconciliação: "upstream==origin suprimido para evitar falso positivo total".

**AC4c — REQ herdada pós-cutoff não entra em K:**
- Fixture upstream: REQ com basename X, data 2026-11-01 (pós-cutoff).
- Fixture fork: mesma REQ com critério aberto.
- Verificar: K=0 (está em `enforced`, não em `exempt`).
- Reconciliação: "REQ herdada pós-cutoff fica em enforced; K é subconjunto de exempt".

---

## Veredito

**LIBERA Wave 1 — ML-1A com as restrições das seções 6.1–6.4.**

Nenhum achado bloqueia. O escopo negativo da REQ (não isenta herdadas, não muda enforcement) elimina evasão de gate.

**Discriminante recomendado:** derivado (`git ls-tree` + basename matching + `git show trackfw.yaml`). ~37ms. Concordância 100% com discriminante declarado sobre corpus do relator (28/28).

**Constraint de AC4:** fixture deve usar `req_dir` diferente entre fork e upstream. Path matching falha no caso real (0 vs 28) — teste com mesmo `req_dir` valida apenas o caminho ingênuo.
