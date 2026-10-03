# Parecer de Qualidade — Extrator JSON do Guard (ML-2B)

> Data: 2026-10-03 | Branch: `fix/guard-de-branch-falha-aberto-sem-jq` | Agente: hefesto-tf

---

## 1. Escopo

Diff analisado: `git diff 3b2eff09..HEAD -- internal/ scripts/ docs/cli-parity.md`

Focos:
- Byte-identidade do extrator `awk` nas cópias ativas
- Legibilidade e comentários
- Tabela de testes C01–C22 + N01–N09 em modo duplo (com e sem `jq`)
- `make quality` completo

---

## 2. Byte-identidade das cópias do extrator awk

A tarefa refere "4 cópias". A inspeção encontrou **3 cópias** ativas:

| # | Arquivo | Linhas awk |
|---|---------|-----------|
| 1 | `scripts/trackfw-git-branch-guard.sh` | linhas 178–264 |
| 2 | `internal/generators/scaffold.go` | linhas 1888–1976 |
| 3 | `internal/validator/validator_git_branch_guard_reference.go` | linhas 202–290 |

O gate `check-git-branch-guard-hook-schema` confirma exatamente esses 3 sítios (log linha 304: "3 sítio(s) derivado(s)... reconciliação ok"). Não há 4.ª cópia.

Comparação SHA256 do bloco `awk '…'` extraído dos 3 arquivos:

```
24b35991c3efe73e6d133e4ef53556ec135957dd9da34a78db3f6f247601b326  scaffold.go
24b35991c3efe73e6d133e4ef53556ec135957dd9da34a78db3f6f247601b326  validator_reference.go
24b35991c3efe73e6d133e4ef53556ec135957dd9da34a78db3f6f247601b326  trackfw-git-branch-guard.sh
```

Byte-idênticas. O bloco do jq NUL check (`# D2-bis`) também é idêntico nos 3 arquivos (881 chars cada).

---

## 3. Legibilidade e comentários

### 3.1 Comentário que mente — BLOQUEANTE (médio)

**Arquivo:** `internal/generators/git_branch_guard_test.go`, linha 959  
**Observado:**

```go
// guardCasesC01C22 retorna os 22 casos obrigatórios da Wave 0 + C22 (NUL).
func guardCasesC01C22() []struct {
```

O nome e o comentário dizem "22 casos (C01–C22)". A função retorna **31 casos**: C01–C22 (22) + N01–N09 (9). As linhas 1018–1034 adicionam os 9 casos negativos sem nenhuma menção no comentário nem no nome da função. Um leitor que busca "onde estão os N01–N09" não encontra pelo nome.

### 3.2 Duplicação de helper — Médio

`runGitBranchGuardWithEnv` (linha 926) reimplementa `runGitBranchGuardImpl` (linha 143) para `args=nil`, com **ordem de parâmetros invertida** (`env, stdin` vs `stdin, env`). O corpo é idêntico exceto pelos parâmetros:

- `runGitBranchGuardImpl(t, dir, scriptPath, args, stdin, env)` — args primeiro
- `runGitBranchGuardWithEnv(t, dir, scriptPath, env, stdin)` — env primeiro

Qualquer refatoração futura que altere um e não o outro quebra silenciosamente. Poderia ser `return runGitBranchGuardImpl(t, dir, scriptPath, nil, stdin, env)`.

### 3.3 Legibilidade do bloco awk — OK

O bloco é denso mas adequado para o contexto (body de here-doc shell). O comentário de cabeçalho `# D1/D2/D2-bis/D2-ter: extrator JSON em awk...` identificar as propriedades-chave (acumulação no END, hex2dec sem strtonum, last-wins, mesma prioridade do jq) é suficiente para orientação. O `_UMARK=sprintf("%c",2)` não tem comentário explicando a escolha do STX, mas o comportamento está documentado implicitamente pelo uso em `decode_str`.

---

## 4. Tabela de testes C01–C22 + N01–N09

### 4.1 Inventário

A função `guardCasesC01C22()` retorna 31 entradas:
- C01–C22: 22 casos da Wave 0 (baseline + Formas A–D + prioridade + limites)
- N01–N09: 9 casos negativos/limites (escape incompleto, unicode inválido, chave unicode, tipos não-string, string não-terminada, contrabarras)

Todos os casos N01–N09 foram adicionados pelo diff em análise.

### 4.2 Dois modos

| Função de teste | Modo |
|---|---|
| `TestGitBranchGuardAwk_C01C22_WithJQ` | PATH do sistema (jq disponível) |
| `TestGitBranchGuardAwk_C01C22_WithoutJQ` | PATH curado sem jq (extrator awk) |

Ambas iteram os 31 casos. Pré-condição de isolamento verificada por `assertJQAbsentInPath` antes de cada rodada sem jq.

### 4.3 Modos duplos nos 44 testes antigos

`runGitBranchGuard` (linha 212) agora chama `runGitBranchGuardBothModes` internamente — executa (a) com PATH do sistema e (b) com PATH curado sem jq, reportando divergência de rc se houver. Os 44 testes pré-existentes adquiriram cobertura do extrator awk sem alteração de assinatura.

O `TestGitBranchGuard_EnvVarFallback_Blocks` foi refatorado para usar `runGitBranchGuardBothModes` com `extraEnv`, preservando o `TRACKFW_GIT_COMMAND` em ambos os modos.

### 4.4 Prova de mordida

`TestGitBranchGuardAwk_ProvaDeMordida` extrai o script do commit `3b2eff09` via `git show` e verifica que os 6 casos (C02–C05, C12, C22) retornam rc=0 (fail-open) com o sed antigo. Afirmação do teste: o fallback por sed estava quebrado nestes casos e os novos testes não são vacuosos.

---

## 5. `make quality` — EXIT=2

**Pré-condição verificada:** `ps -axo pid,command | grep -E "chunk_|go test|make quality" | grep -v grep` saiu vazio antes da execução.

**Resultado:** EXIT=2. O `make` parou em `parity-rest` antes de atingir `parity-falsify`.

**Linha de falha (log q507.log, linha 1148):**

```
check-symlink-privilege-guard: FALHA — sitios sem guarda de capacidade:
  internal/generators/git_branch_guard_test.go:889: symlink/fifo sem guarda de capacidade
```

**Causa:** `makeCuratedPathWithoutJQ` (linha 889) chama `os.Symlink` diretamente. O gate `check-symlink-privilege-guard` exige `symlinkOrSkip` — que distingue "sem privilégio (skip)" de "falhou por outro motivo (fail)". O código atual tem fallback para cópia (`copyExecutableFile`), mas não usa a guarda de capacidade que o gate exige.

**A suíte de falsificação (`run-gates-falsify-parallel.sh`) não rodou.** O make parou antes de `parity-falsify`. A linha esperada pelo task:

```
run-gates-falsify-parallel: suite completa -- N chunks, X OK, Y FAIL, ...
```

não está no log. Por instrução da tarefa: não invento números.

---

## 6. Achados por severidade

| # | Severidade | Arquivo / Linha | Descrição |
|---|-----------|-----------------|-----------|
| A1 | **BLOQUEANTE** | `internal/generators/git_branch_guard_test.go:889` | `os.Symlink` sem guarda `symlinkOrSkip` — quebra `make quality` (EXIT=2) |
| A2 | Médio | `internal/generators/git_branch_guard_test.go:959` | `guardCasesC01C22` — nome e comentário mentem: retorna 31 casos (C01–C22 + N01–N09), não 22 |
| A3 | Baixo | `internal/generators/git_branch_guard_test.go:926` | `runGitBranchGuardWithEnv` duplica `runGitBranchGuardImpl` com ordem de parâmetros invertida; poderia ser um wrapper de uma linha |

---

## 7. Correção necessária para aprovação

**A1** é o único bloqueante. O fix é substituir `os.Symlink(binPath, dest)` por `symlinkOrSkip(t, binPath, dest)` em `makeCuratedPathWithoutJQ`. Se `symlinkOrSkip` não existir no pacote, criar o wrapper seguindo o padrão do próprio gate (distinguir `EPERM/ERROR_PRIVILEGE_NOT_HELD` de outros erros).

**A2** e **A3** são ajustes de qualidade, não blocantes de merge: renomear `guardCasesC01C22` para `guardAllCases` (ou similar) e atualizar o comentário; eliminar a duplicação de `runGitBranchGuardWithEnv`.

---

## Veredito: REPROVA

EXIT=2. `make quality` parou em `check-symlink-privilege-guard` (A1). A suíte de falsificação não rodou. Sem a correção de A1, o gate bloqueia merge.
