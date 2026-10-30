# Red Team — Fonte ls-remote na Resolução da Branch Padrão (#633)

> Roadmap: ROADMAP-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada...md
> Data: 2026-10-30 | Revisor: hades-tf (ML-2A)

---

## Tabela de Mutações

| ID | Alvo | Mutação | Resultado | Gate que matou |
|----|------|---------|-----------|----------------|
| M-1 | `isValidBranchLocalName` | Remove verificação de `..` | **MORTA** | `TestIsValidBranchLocalName/double_dot` |
| M-2 | `parseLsRemoteSymref` | Troca `second != "HEAD"` por `!strings.HasSuffix(second, "HEAD")` | **SOBREVIVE** | Nenhum — vetor `\trefs/heads/HEAD` não testado |
| M-3 | `parseLsRemoteSymref` | Aceita qualquer `ref: refs/` (não só `refs/heads/`) | **MORTA** | `TestParseLsRemoteSymref/refs/tags_not_refs/heads` |
| M-4 | `resolveDefaultBranchWithSource` | ls-remote ANTES do symref (ordem trocada) | **MORTA** | `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` |
| M-4b | `defaultBaseBranch` (ship.go) | ls-remote ANTES do symref no `defaultBaseBranch` | **SOBREVIVE** | Nenhum — `defaultBaseBranch` não tem pin equivalente |
| M-4c | `resolveDefaultBranchWithSource` | ls-remote também no caminho happy-path (symref ok) | **MORTA** | `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` |
| M-5 | `resolveDefaultBranchWithSource` | ls-remote também nos caminhos b/c (symref malformado) | **MORTA** | `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` |
| M-5b | `defaultBaseBranch` (ship.go) | ls-remote nos caminhos b/c do `defaultBaseBranch` | **SOBREVIVE** | Nenhum — `defaultBaseBranch` não tem pin equivalente |
| M-6 | `gitCommitsSince` (R-5) | Sempre usa `origin/<base>` (remove verificação local-first) | **SOBREVIVE** | Nenhum — sem teste que pina local-first |
| M-7 | `release_test.go` P-4 | ls-remote retorna `"ref: refs/heads/master\tHEAD"` no release test | **MORTA** (teste reprova) | Staleness check dispara: `localBranchSHA = ""` ≠ `releaseTestSHA` |

---

## Vereditos por Prescrição (Wave 0)

### P-1 — Nova fonte ls-remote em `resolveDefaultBranchWithSource`

**APROVADO com ressalva.**

A implementação segue a prescrição: ls-remote entra apenas no ramo `err != nil` (caminho a), nunca nos caminhos b/c (symref retornou output mas malformado). M-4 e M-5 morrem nos testes. M-5b (veja abaixo) mostra que `defaultBaseBranch` não tem um pin equivalente nos seus caminhos b/c.

**Ressalva documentada:** `defaultBaseBranch` em `ship.go` aceita ls-remote antes do symref (M-4b) e nos caminhos b/c (M-5b) sem que nenhum teste rejeite essas mutações. Os caminhos b/c retornam "main" diretamente sem tentar ls-remote; a inversão (M-5b) passa porque os fakes de `ship_test.go` têm `default: return "", nil`, então `parseLsRemoteSymref("")` = false e o resultado é idêntico. M-4c (ls-remote no happy-path de `resolveDefaultBranchWithSource`) é morta por `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled`; o análogo em `defaultBaseBranch` (M-4b) sobrevive.

**Vetor ausente:** `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` existe para `resolveDefaultBranchWithSource`; não existe equivalente para `defaultBaseBranch`. Impacto: se um contribuidor futuramente mover o bloco ls-remote para fora do `else` em `defaultBaseBranch`, a cobertura não vai capturar.

**Severidade: BAIXA.** Não é exploração hoje; é gap de cobertura que permite drift futuro.

### P-2 — `isValidBranchLocalName` com lista de permissão

**APROVADO.**

M-1 (remove `..`) morreu em `TestIsValidBranchLocalName/double_dot`. Todos os vetores de P-2 estão cobrindo casos de rejeição e aceitação. A lista de permissão `[a-zA-Z0-9._/-]` é implementada por loop de caractere, não regex.

**Achado M-2 — vetor de segundo campo ausente (BAIXA).**

O teste `TestParseLsRemoteSymref/second_field_is_refs/heads/HEAD_not_HEAD` tem o nome enganoso: o input é `ref: refs/heads/HEAD\tHEAD` (segundo campo = `"HEAD"`, não `"refs/heads/HEAD"`). O vetor que o nome sugere — segundo campo `refs/heads/HEAD` (termina com HEAD mas não é igual a HEAD) — não existe no suite.

A mutação M-2 troca `second != "HEAD"` por `!strings.HasSuffix(second, "HEAD")`, permitindo aceitar linhas cujo segundo campo termine em `HEAD` mas não seja `HEAD` (ex: `refs/heads/HEAD`, `some/HEAD`). Nenhum teste reprova. Resultado: linha `ref: refs/heads/main\trefs/heads/HEAD` seria aceita pela versão mutada, retornando "main" como branch padrão.

Impacto prático: baixo, porque um remoto hostil que controla a saída do ls-remote pode servir qualquer nome de branch diretamente na primeira coluna. A verificação do segundo campo é de correção semântica, não de barreira de segurança. Mas o teste deveria cobrir esse vetor para documentar a intenção.

**Vetor faltante:** `{"ref: refs/heads/main\trefs/heads/HEAD", wantOK: false}` em `TestParseLsRemoteSymref`.

### P-3 — `defaultBaseBranch` usa helper compartilhado, sem forge

**APROVADO.**

`defaultBaseBranch` em `ship.go:611–629` chama `parseLsRemoteSymref` e `isValidBranchLocalName` (via o helper no `branch_prune_forge.go`). Não chama a forge. Os três comentários foram atualizados conforme prescrito (verificado na leitura do diff). `release.go:270–274` também foi atualizado.

`defaultBaseBranch` não foi alargado com parâmetro de forge — a assinatura permanece `func defaultBaseBranch(execGit func(...string) (string, error)) string`.

### P-4 — Testes que declaram o case ls-remote explicitamente

**APROVADO.**

Três testes foram atualizados/renomeados:

1. `TestDefaultBranch_NeitherAvailable_FallsBackToMain` (`branch_prune_default_branch_test.go`): case `ls-remote` declarado com `return "", fmt.Errorf(...)`. Gate: `default: return "", fmt.Errorf("unexpected gitExec")` — ls-remote inesperado causaria falha. Frase de reconciliação correta.

2. `TestDefaultBaseBranch_SymrefAndLsRemoteFail_FallsBackToMain` (`ship_test.go`): renomeado de `SymbolicRefFails` para `SymrefAndLsRemoteFail`. Case `ls-remote` declarado com `return "", errors.New("ls-remote: offline")`. Gate: `default: return "", errors.New("unexpected...")`. Frase de reconciliação correta.

3. `TestReleaseTag_NoLocalSymref_ForgeBranchNameUsedWithoutFalseRefusal` (`release_test.go`): case `ls-remote` declarado como `""` (vazio). M-7 confirma que declarar `"ref: refs/heads/master\tHEAD"` causa falha no test (staleness check dispara porque `localBranchSHA = ""` ≠ `releaseTestSHA`). **Nota:** o comentário do teste diz "absent in the mock → skipped" (staleness check seria pulado), mas a execução mostra o contrário: o mock retorna `("", nil)` para chaves desconhecidas, então `verr == nil` (não é erro) e o staleness check procede, comparando `"" != releaseTestSHA`. O comentário está factualmente errado, mas o comportamento do teste é correto — a declaração de `""` é a escolha correta. Impacto: documento, não segurança.

---

## Achados

### A-1 — Teste `TestParseLsRemoteSymref/second_field_is_refs/heads/HEAD_not_HEAD` testa vetor diferente do nome (BAIXA)

**Arquivo/linha:** `internal/commands/branch_default_ls_remote_test.go`, caso com nome `"second field is refs/heads/HEAD not HEAD"`.

**Comportamento observado:** o input do caso é `"ref: refs/heads/HEAD\tHEAD"` (segundo campo é `"HEAD"` exato, não `"refs/heads/HEAD"`). O nome do caso afirma uma coisa; o input testa outra.

**Mutação que sobrevive:** M-2 — `!strings.HasSuffix(second, "HEAD")` em vez de `second != "HEAD"`. Com esse mutante, a linha `ref: refs/heads/main\trefs/heads/HEAD` seria aceita (segundo campo `refs/heads/HEAD` tem sufixo `HEAD`), retornando "main" incorretamente.

**Correção sugerida:** renomear o caso para `"second_field_is_exact_HEAD_as_branch_name"` e adicionar novo caso `{"second_field_ends_with_HEAD_but_is_not_HEAD", "ref: refs/heads/main\trefs/heads/HEAD", false}`.

**Evidência:** M-2 executado com `go test ./internal/commands/ -count=1 -run TestParseLsRemoteSymref` — todos os testes PASSARAM com a mutação.

### A-2 — `defaultBaseBranch` sem pin "ls-remote não chamado" (BAIXA)

**Arquivo/linha:** `internal/commands/ship.go:611–628` + `internal/commands/ship_test.go`.

**Comportamento observado:** M-4b (ls-remote ANTES do symref em `defaultBaseBranch`) e M-5b (ls-remote nos caminhos b/c de `defaultBaseBranch`) ambos sobrevivem — todos os `TestDefaultBaseBranch_*` passam porque o fake padrão retorna `("", nil)` e `parseLsRemoteSymref("")` = false, resultando em "main" igualmente. Nenhum teste verifica que ls-remote NÃO é chamado na ordem incorreta, nem que ls-remote NÃO é chamado quando symref retornou output malformado.

`resolveDefaultBranchWithSource` tem `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` que mata M-4 e M-5. `defaultBaseBranch` não tem equivalente para nenhum dos dois.

**Correção sugerida:** adicionar teste `TestDefaultBaseBranch_SymrefMalformed_LsRemoteNotCalled` em `ship_test.go` (análogo ao existente em `branch_default_ls_remote_test.go`) usando fake com `t.Errorf` no case ls-remote.

**Evidência:** M-5b executado — `go test ./internal/commands/ -count=1 -run TestDefaultBaseBranch` todos PASS.

### A-3 — `gitCommitsSince` local-first não pinado; nenhuma das duas estratégias é correta universalmente (BAIXA)

**Arquivo/linha:** `internal/commands/ship.go:560–564`.

**Comportamento observado:** M-6 (sempre usa `origin/<base>`) sobrevive — `TestGitCommitsSince_FallsBackToOriginWhenLocalRefAbsent` passa porque esse teste só cobre o caso onde a ref local está ausente. Não existe teste para o caminho local-first.

**Medição — as duas estratégias divergem em pelo menos um caso:**

Cenário (a-extended): feature branch `feat/x2` cortada de `origin/master` após um commit `extra` ser empurrado ao remoto. Local `master` ainda aponta para o commit anterior (nunca atualizado).

```
origin/master: A -- B (extra) -- (HEAD remoto)
local master:  A               (nunca atualizado)
feat/x2:       A -- C (feat)   (cortada após pull, antes de local master atualizar)
```

Comandos executados com git real:
- `git log master..HEAD` (local-first): **2 commits** — `feat: commit on x2` E `extra commit on origin`
- `git log origin/master..HEAD` (sempre origin/): **1 commit** — `feat: commit on x2` somente

Local-first sobre-reporta quando o local está atrás do remoto E a feature foi cortada da versão mais nova do origin. "Sempre origin/" dá resultado mais preciso nesse caso.

O oposto também existe: se local `master` estiver à frente do remoto (commits locais não empurrados) e a feature for cortada do local, "sempre origin/" incluiria commits locais não-feature no range — mas esse caso não foi medido nesta sessão.

**Conclusão:** as duas estratégias produzem resultados diferentes em casos reais medidos. O comportamento desejado para o PR body não está especificado. Adicionar `TestGitCommitsSince_PrefersLocalRefWhenPresent` sem antes decidir qual estratégia é correta pinaria o comportamento errado em ao menos um dos casos medidos.

**Ação para o arquiteto:** declarar qual estratégia é a intenção — local-first (comportamento atual), sempre origin/, ou uma regra mais sofisticada. Só então adicionar o teste que pina a decisão.

### A-5 — `isValidBranchLocalName` admite nomes que `git check-ref-format` rejeitaria; `HEAD` afeta branch prune (BAIXA)

**Arquivo/linha:** `internal/commands/branch_prune_forge.go:68–85` e `branch_default_ls_remote_test.go:176–186`.

**O que o código faz:** o permit list `[a-zA-Z0-9._/-]` não implementa todas as regras de `git check-ref-format`. Os seguintes valores passam:

| Valor | Por que o git rejeita como branch | Impacto downstream |
|-------|-----------------------------------|--------------------|
| `HEAD` | ref especial, não é branch | ver análise abaixo |
| `ORIG_HEAD`, `FETCH_HEAD` | refs especiais | `rev-parse` → SHA de ref especial; git log erraria |
| `.` | componente de path começa com `.` | `rev-parse .` → provavelmente falha |
| `/x` | branch não pode começar com `/` | `rev-parse /x` → provavelmente falha |
| `x/` | branch não pode terminar com `/` | `rev-parse x/` → provavelmente falha |
| `x.lock` | branch não pode terminar com `.lock` | `rev-parse x.lock` → provavelmente falha |

**Evidência — o teste documenta `HEAD` como aceito:**

`branch_default_ls_remote_test.go:176–186`: o caso `"second field is refs/heads/HEAD not HEAD"` usa input `"ref: refs/heads/HEAD\tHEAD"` com `wantOK: true, wantName: "HEAD"`. O comentário no código diz explicitamente "edge case — git servers do not normally name a branch 'HEAD'" e "this path is technically accepted by the parser." Rejeitar `HEAD` quebraria esse teste documentado — qualquer correção futura deve decidir primeiro se a aceitação é intencional.

**Para `.`, `x/`, `x.lock`:** o git downstream (em `resolveDefaultBranchWithSource` ou `defaultBaseBranch`) receberia o nome e chamaria `gitExec(...)` com ele. Essas chamadas provavelmente errariam, e o caller (que não valida o retorno de `resolveDefaultBranchWithSource` antes do `rev-parse --verify`) cairia para o comportamento de "ref não encontrada". Classifico como hipótese — não confirmada com binário real por limitação de tempo.

**Análise de impacto para `HEAD` em branch prune (path tracado):**

`resolveDefaultBranchWithSource` retorna `("HEAD", "origin/HEAD", false)`.

1. `branch_prune.go:426`: `git rev-parse --verify -q "origin/HEAD"`. `refs/remotes/origin/HEAD` existe quando foi criado pelo fetch (como symref ou ref direta). Se não existe → prune recusa ("not found") → **seguro, nada deletado**.

2. Se `origin/HEAD` existe e o prune prossegue: `defaultLocalName = "HEAD"`. A verificação `b == defaultLocalName` (linha 456) compara cada branch local contra `"HEAD"`. Nenhum branch em `refs/heads/` se chama `"HEAD"` — a verificação nunca dispara. O branch real padrão (ex: `"main"`) é avaliado como branch regular.

3. `evaluateBranchIntegration("main", "origin/HEAD", ...)`: git resolve `origin/HEAD` via symref para `refs/remotes/origin/main` (ou target da symref). `git merge-base "origin/HEAD" main` e `git diff --name-only "origin/HEAD" main` funcionam porque git dereferencia. Com symref válida: `main` tem conteúdo idêntico a `origin/HEAD` → classificado como "squash-merged" → com `--apply`, **`main` seria deletado**.

4. **Pré-condições para o ataque:**
   - Remoto hostil serve `ref: refs/heads/HEAD\tHEAD`
   - `symbolic-ref refs/remotes/origin/HEAD` falha (sem symref local → ls-remote é ativado)
   - `rev-parse --verify -q "origin/HEAD"` tem sucesso (ref existe localmente como symref ou direta)

   Condições 2 e 3 são compatíveis: `symbolic-ref` falha se a ref não existe; `rev-parse` também falha. Mas se `origin/HEAD` existe como ref direta (não symref) — criada por `git update-ref refs/remotes/origin/HEAD <SHA>` — `symbolic-ref` falha e `rev-parse` tem sucesso. Essa combinação não é criada pelo fluxo normal de fetch, mas é tecnicamente possível.

**Injeção:** nula — os chars perigosos (`~`, `^`, `@{`, `:`, `*`, espaço, `\0`) estão todos bloqueados pelo permit list.

**Severidade:** BAIXA. Vetor de deleção de branch padrão requer remoto hostil + estado local incomum. Não bloqueia merge.

**Correção sugerida (não bloqueia merge):** rejeitar `HEAD` explicitamente em `isValidBranchLocalName` (requer atualizar o teste documentado que aceita). Alternativamente, em `branch_prune.go`, após resolver `defaultLocalName`, verificar se ele é `"HEAD"` e recusar com mensagem explicativa.

### A-4 — Comentário errado em `release_test.go` P-4 (INFORMATIVO)

**Arquivo/linha:** `internal/commands/release_test.go`, comentário pós `g.responses["ls-remote --symref origin HEAD"] = ""`.

**Comportamento observado:** o comentário diz "absent in the mock → skipped" para o staleness check. M-7 mostra que o mock retorna `("", nil)` para chaves desconhecidas, o que significa `verr == nil` (branch "exists") e o staleness check PROCEDE e falha com `localBranchSHA = "" ≠ releaseTestSHA`. A explicação correta é: "mock default retorna nil → staleness check procede → `localBranchSHA = ""` ≠ `releaseTestSHA` → test falha".

**Correção sugerida:** atualizar o comentário no teste para refletir o comportamento real do mock.

---

## R-5 — Veredito separado (decisão do arquiteto)

O R-5 (`gitCommitsSince` com fallback para `origin/<base>`) entrou no ML-1A por decisão do arquiteto, fora do escopo original da Wave 0. A implementação está correta para o caso prescrito: quando `refs/heads/<base>` ausente, usa `origin/<base>`. M-6 revela que o oposto (sempre `origin/<base>`) não é testado.

**Exposição DWIM:** `gitCommitsSince` usa nomes curtos (`base` / `origin/<base>`) no git log. Se existir uma tag `origin/main`, o git DWIM resolveria `origin/main` para `refs/tags/origin/main` antes de `refs/remotes/origin/main`. Na prática, criar uma tag chamada `origin/main` é possível mas improvável. Não bloqueia o merge; declarado como residual.

---

## Medições Diretas

### Item 3 — Deadline de rede em `ship`

`defaultBaseBranch` e `resolveDefaultBranchWithSource` são os dois pontos de chamada ls-remote em `ship` sem forge. Pior caso: 2 × NetworkTimeout (padrão 60 s, configurável via `TRACKFW_NETWORK_TIMEOUT`).

Medido com `zz_hades_netdeadline_test.go` (arquivo temporário, apagado após; `git status --short` confirmado limpo): `net.Listen` em loopback aceitando mas nunca respondendo, `TRACKFW_NETWORK_TIMEOUT=2s`. `defaultBaseBranch(defaultGitExec)` com remoto inalcançável retornou `"main"` em **2.010 s**. Deadline aplicado corretamente.

### Item 4b — Ambiguidade DWIM com tag `origin/main`

`gitCommitsSince` usa nomes curtos (`base` / `origin/<base>`) no range do log. Medido com git real: tag `origin/main` criada apontando para commit base antigo; `refs/remotes/origin/master` (remoto mais novo) presente.

`git rev-parse --symbolic-full-name origin/main` → `refs/tags/origin/main` (DWIM prefere tag). `git log origin/main..HEAD` usa a tag, não o tracking branch — lista de commits diferente do esperado.

Impacto: PR body pode omitir ou incluir commits inesperados quando uma tag chamada `origin/<base>` existe. Requer controle do repositório para criar a tag — não é um vetor de ataque externo. Declarado em RD (R-5 section).

---

## Residual Declarado

**RD-1:** Gap de cobertura em `TestParseLsRemoteSymref` — vetor `\trefs/heads/HEAD` ausente (A-1).

**RD-2:** `defaultBaseBranch` sem pin "ls-remote não chamado" para ordem (M-4b) nem para caminhos b/c (M-5b) — ambos os survivors de A-2. Equivalente a `TestResolveDefaultBranch_SymrefMalformed_LsRemoteNotCalled` ausente em `ship_test.go`.

**RD-3:** `gitCommitsSince` — nenhuma das duas estratégias (local-first, sempre origin/) é correta universalmente. Decisão de intenção pendente para o arquiteto antes de adicionar pin (A-3).

**RD-4:** `GIT_TERMINAL_PROMPT=0` ausente em todas as chamadas de rede (pré-existente, R-1 da Wave 0).

**RD-5:** Gate `make quality` do ML-2A não executado (proibido pelo handoff). Pendente para o arquiteto.

---

## `git status --short`

```
 M .claude/agent-memory/hades-tf/MEMORY.md
?? .claude/agent-memory/hades-tf/project_parselsremotesymref_second_field_vector_gap.md
?? docs/seguranca/2026-10-30-redteam-branch-padrao-ls-remote.md
?? docs/seguranca/2026-10-30-redteam-teste-atencao-sem-symlink.md
```

Nenhum arquivo de produto ou teste modificado.
