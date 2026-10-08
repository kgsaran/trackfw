---
date: 2026-10-08
author: hades-tf
roadmap: ROADMAP-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md
wave: 2
ml: ML-2A
---

# Red-Team ML-2A: req_done_open_criteria — recorte de herdadas

REQ: `docs/req/REQ-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md`
Wave 0: `docs/seguranca/2026-10-08-wave0-req-done-open-criteria-herdadas.md`

Binários usados:
- Feature (`tf`): built de `feat/req-done-open-criteria-decompoe-herdadas`
- Main (`tf_main_2a`): built de `origin/main` via `git archive` no scratch

---

## Veredito

**NÃO LIBERA o PR como está.**

Achado F1 (MEDIUM) viola a restrição §6.2 da Wave 0 ("Nenhuma aspa dupla na parentética")
e exige corretivo de uma linha antes do merge. Pertence a esta PR/REQ pela Regra de Causa
Raiz (mesma superfície introduzida neste ML).

F2 (INFO) é aceito como residual — mesma classe de risco dos demais `.Output()` do pacote.
Todos os outros vetores nomeados na Wave 0 e no handoff estão encerrados.

---

## Achados

### F1 — MEDIUM | shortRef não sanitizado: `"` no nome do ramo polui `warnings[].file` no JSON

**Superfície:** `upstreamSymrefShort()` em `validator_req_done_criteria.go`, passo 4 (fallback
`refs/remotes/upstream/HEAD`). O retorno de `git symbolic-ref --short` é inserido diretamente
no format string `"(%d inherited from %s)"` sem sanitização.

**Condição de disparo:** `refs/remotes/upstream/main` e `refs/remotes/upstream/master` ausentes
(ou não resolvíveis) E `refs/remotes/upstream/HEAD` aponta para ramo cujo nome contém `"`.

**Evidência:**
```
git check-ref-format 'refs/heads/main"evil"'  →  exit=0   # git aceita " no nome do ramo
```

Criou-se o ref file `$UPSTREAM/.git/refs/heads/main"evil"` e o symref
`$FORK/.git/refs/remotes/upstream/HEAD → refs/remotes/upstream/main"evil"`.

Resultado do `validate --json`:
```
rule: req_done_open_criteria
FILE: 'evil'                          # ← extractFile regex "([^"]+)" extrai 'evil'
message: ...1 inherited from upstream/main"evil"...
```

Saída texto (legível, não afetada):
```
(1 inherited from upstream/main"evil")
```

**Impacto:**
- JSON `warnings[].file` exibe caminho falso extraído do nome do ramo.
- `warnings[].message` e saída texto: corretos.
- Violações, exit code, classificação exempt/enforced: **inalterados**.
- Restrição §6.2 da Wave 0 ("Nenhuma aspa dupla na parentética") violada.

**Fix mínimo (para apolo-tf):**
Sanitizar `shortRef` antes de formatar. Duas opções equivalentes:
```go
// Opção A — remover aspas duplas
shortRef = strings.ReplaceAll(shortRef, `"`, "")
// Opção B — trocar por aspas simples  
shortRef = strings.ReplaceAll(shortRef, `"`, "'")
```
Acrescentar teste `TestReqDoneOpenCriteria_DoubleQuoteBranchName` que cria o ref file
manualmente e verifica `file == ""` no JSON.

**Alcançabilidade:** somente via o fallback HEAD (step 4); o caminho normal
`upstream/main` / `upstream/master` usa strings hard-coded que não contêm `"`.

---

### F2 — INFO | trackfw.yaml de 47 MB no upstream: 510 ms vs. 67 ms de baseline

**Superfície:** `upstreamReqBasenames()` chama `.Output()` sem limite de tamanho em
`git show <ref>:trackfw.yaml`.

**Medição:**
```
trackfw.yaml = 47 MB (comment block)  →  510 ms  (7x baseline ~67 ms)
trackfw.yaml = alias-bomb YAML        →  120 ms  (yaml.v3 bem comportado)
req_dir com 500 arquivos              →  118 ms  (ls-tree 500 entradas)
fork/ normal (~251 arquivos)          →  189 ms  (vs. 157 ms no main — +32 ms)
```

**Caracterização:**
- Não há hang: `git show` + `gopkg.in/yaml.v3` processam 47 MB em ~500 ms e retornam.
- Limitado pelo object store local (todos os bytes do blob já foram fetched; zero rede).
- Padrão pre-existente: todos os `gitCommand(...).Output()` do pacote carecem de timeout.
- GitHub limita blobs a 100 MB; no pior caso ~1 s por validate.
- Alias-bomb não causa problema: yaml.v3 termina sem hang.

**Decisão:** aceitar como residual §5.x da Wave 0. Registrado; sem corretivo obrigatório neste ML.

---

## Vetores confirmados seguros

| Vetor | Resultado | Evidência |
|---|---|---|
| reqDir começando com `-` (injeção de opção) | K=0, exit=0 | `--` antes do pathspec; git ls-tree não interpreta como opção |
| reqDir como caminho absoluto `/etc/passwd` | K=0, exit=0 | git rejeita (outside repository, exit=128) → resultado vazio |
| reqDir com `../../etc` (path traversal) | K=0, exit=0 | git rejeita (outside repository, exit=128) → resultado vazio |
| reqDir `:(exclude)docs/req` (pathspec magic) | K=0, exit=0 | `--literal-pathspecs` bloqueia; saída vazia |
| GIT_DIR=/evil/path no ambiente do processo | output idêntico | cleanGitEnv() strip GIT_*; medido: diff entre com/sem GIT_DIR = vazio |
| GIT_WORK_TREE herdado | coberto | mesma lógica: cleanGitEnv() strip prefixo GIT_ |
| Byte-identidade sem upstream (git repo, origin only) | CONFIRMADO | diff full output + JSON = zero entre tf_main_2a e tf |
| Byte-identidade sem upstream (sem git) | CONFIRMADO | diff = zero |
| Exit code e violações inalterados | CONFIRMADO | feat exit=1, main exit=1 (ambos via req_has_adr); req_done: warning, não violation |
| K não muda decisão exempt/enforced/exit | CONFIRMADO | K adicionado após o loop; não re-entra na lógica de classificação |
| Custo em fork/ (10 medições) | +32 ms mediana | feat 189 ms vs. main 157 ms; dentro do plano Wave 0 |
| Git worktree | correto | (1 inherited from upstream/main) |
| Sem git repo | sem parentética | correto |
| Submodulo (sem upstream configurado no clone) | sem parentética | correto |
| Ref não resolvível (main/master ausentes, HEAD dangling) | `(upstream tried main, master: ref unresolvable)` | correto |
| Ramo com `/` no nome (feature/special-branch) | `(1 inherited from upstream/feature/special-branch)` | correto |
| T1 guard (upstream URL == origin URL) | sem parentética | confirmado em fork_t1_test e shallow2 |
| YAML alias-bomb | 120 ms, sem hang | yaml.v3 bem comportado |
| upstream/HEAD → refs/heads/main (ref local) | `(1 inherited from main)` | residual §5.1; K certo, label confuso; local-only, não afeta enforcement |

---

## Verificação dos testes (Wave 0 §6.4)

Todos os novos testes foram lidos e verificados manualmente:

| Teste | Usa ValidateTagged | req_dir divergente | Afirmação reconciliada |
|---|---|---|---|
| AC3_NoUpstreamByteIdentical | Sim | n/a (sem git) | Literal hard-coded `wantD4NoticeAC3Prefix`, não tautológico |
| AC4a_InheritedByBasename | Sim | Sim: fork=`docs/requisições`, upstream=`docs/req` | K=2, REQ local não conta |
| AC4b_T1_UpstreamEqualsOrigin | Sim | n/a | T1 suprime parentética |
| AC4c_InheritedPostCutoffInEnforced | Sim | n/a | REQ pós-cutoff fica em enforced; K=1 só do pré-cutoff |
| UpstreamRefUnresolvable | Sim | n/a | `(upstream tried main, master: ref unresolvable)` |

Resultado de `go test ./internal/validator/ -run "AC3|AC4|UpstreamRefUnresolvable"`: **PASS (1.148s)**.

Verificação de código: `grep -rn 'exec.Command("git"' internal/validator/` — único hit em
produção é `validator_git_exec.go:75` (`gitCommand()`). Todos os demais hits são em arquivos
`_test.go`. Refs usadas em `upstreamInheritedInfo` são hard-coded
(`refs/remotes/upstream/main`, `refs/remotes/upstream/master`, `refs/remotes/upstream/HEAD`);
somente `shortRef` (string display) origina-se de `symbolic-ref --short`, e é aí que F1 entra.

---

## Ações requeridas

1. **apolo-tf (mesmo PR):** sanitizar `shortRef` em `upstreamSymrefShort()` —
   `strings.ReplaceAll(shortRef, `"`, "")` — e acrescentar teste de regressão.
2. **Após corretivo:** re-rodar `make quality` e resubmeter para auditoria final.
3. **F2 (INFO):** documentar em §5.x da Wave 0 como residual aceito. Não requer código.
