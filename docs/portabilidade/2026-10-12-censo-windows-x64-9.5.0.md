# Censo Windows x64 — trackfw 9.5.0

> Medição: 2026-10-12 | Run CI: [38319939952](https://github.com/kgsaran/trackfw/actions/runs/38319939952) | Commit: a1cc6e93 | Tag: v9.5.0
> Base x64: [36036473391](https://github.com/kgsaran/trackfw/actions/runs/36036473391) (2026-09-24)
> Base ARM64: VM privada 2026-09-08 (hardcoded no job apuracao)

---

## 1. Validade do instrumento

**Commit:** a1cc6e93 (v9.5.0) — confirmado no header do job `falsificacao` (Windows Server 2025, runner v2.337.0).

**Células de falsificação:**

| Célula | Expectativa | Resultado |
|--------|-------------|-----------|
| A1 — BASE_PATH (RUNTIME_BIN:GIT_BIN_DIR:/usr/bin:/bin) | PASS | A1 PASS — git resolve em BASE_PATH para processo filho nativo (py e go) |
| A2 — NO_FORGE_PATH (RUNTIME_BIN:GIT_BIN_DIR) | PASS | A2 PASS — git resolve em NO_FORGE_PATH para processo filho nativo (py e go) |
| B1 — Forma 1 revertida (sem GIT_BIN_DIR) | FAIL esperado | B1 PASS (probe falhou como esperado) — Forma 1 (GIT_BIN_DIR) é load-bearing |
| B2 — Forma 2 revertida (git sem .exe) | FAIL esperado | B2 PASS (probe falhou como esperado) — arquivo 'git' sem .exe não é resolvido via PATHEXT |

**Working tree:** limpo (`git status --porcelain` saiu vazio).

**Shards:** 8/8 executados. Shards 0, 2 e 5 saíram com `failure=true` porque contêm FAIL labels; com `TRACKFW_FALSIFY_ENUMERATE=1` isso é o comportamento correto — o shard não aborta na primeira falha.

**Totais:**

| Origem | OK | FAIL |
|--------|----|------|
| ARM64 (base 2026-09-08, hardcoded) | 440 | 512 |
| x64 base (36036473391, 2026-09-24, lido de logs de shard) | 347 | 11 |
| x64 hoje (38319939952, 2026-10-12) | 343 | 4 |
| Queda de FAIL ARM64→hoje | — | 508 (esperado ~442; diferença de 66 atribuída a fixes entre 2026-09-08 e hoje além do ML-R2c) |

Por shard (apuracao):

```
shard 0: OK=66  FAIL=1   (base ARM64 FAIL=110 delta=-109 ▼)
shard 1: OK=86  FAIL=0   (base ARM64 FAIL=25  delta=-25  ▼)
shard 2: OK=122 FAIL=1   (base ARM64 FAIL=5   delta=-4   ▼)
shard 3: OK=37  FAIL=0   (base ARM64 FAIL=113 delta=-113 ▼)
shard 4: OK=22  FAIL=0   (base ARM64 FAIL=5   delta=-5   ▼)
shard 5: OK=4   FAIL=2   (base ARM64 FAIL=17  delta=-15  ▼)
shard 6: OK=5   FAIL=0   (base ARM64 FAIL=234 delta=-234 ▼)
shard 7: OK=1   FAIL=0   (base ARM64 FAIL=3   delta=-3   ▼)
```

Nota contagem: `apuracao` usa `grep -ac '^OK'` — inclui linhas unlabeled como `OK   vacuidade: 1 >= 1`. O total de rótulos `^OK   [` é 313. A unidade deste censo é o rótulo; as 30 linhas extras não são rótulos.

---

## 2. FAIL diff por rótulo

### Base x64 (36036473391) — 11 rótulos FAIL

Extraídos dos logs dos 8 shards de censo (artefatos expirados — lidos via `gh run view --job <id> --log`):

```
falsify/git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload
falsify/git-branch-guard-dedup/baseline-skips-project-entry
falsify/git-branch-guard-dedup/double-slash-tolerance
falsify/sandbox-gap-e/direction-a-detected
falsify/sandbox-walkdir-reintroduced/direction-b-detected
falsify/scaffold-update-chmod-removed/direction-c-detected
falsify/setup-s75
falsify/setup-s76
falsify/setup-s87-baseline
falsify/setup-s158-baseline
falsify/setup-s175-baseline
```

### Hoje x64 (38319939952) — 4 rótulos FAIL

```
falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline
falsify/roadmap-req-frontmatter-path/go/from-req-baseline
falsify/scaffold-update-chmod-removed/direction-c-baseline
falsify/scaffold-update-chmod-removed/direction-c-detected
```

### Persisting FAIL (base ∩ hoje) — 1 rótulo

```
falsify/scaffold-update-chmod-removed/direction-c-detected
```

### Turned OK (em base, ausente em hoje) — 10 rótulos

```
falsify/git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload  → resolvido
falsify/git-branch-guard-dedup/baseline-skips-project-entry                           → resolvido
falsify/git-branch-guard-dedup/double-slash-tolerance                                 → resolvido
falsify/sandbox-gap-e/direction-a-detected                                            → resolvido
falsify/sandbox-walkdir-reintroduced/direction-b-detected                             → resolvido
falsify/setup-s75       → P4 check-release-tag-parity.sh agora passa com binário real
falsify/setup-s76       → idem
falsify/setup-s87-baseline  → idem
falsify/setup-s158-baseline → idem
falsify/setup-s175-baseline → P4 check-update-parity.sh agora passa com binário real
```

Os rótulos `setup-sXX` são rótulos FAIL-only (gen-falsify-chunks.py documenta 36 rótulos da família setup-* como invisíveis em cenário de sucesso). Sua ausência hoje significa que a precondição P4 passou — o script de gate não reprova com o binário real — não que o cenário foi removido.

### New FAILs (em hoje, ausente em base) — 3 rótulos

```
falsify/git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline
falsify/roadmap-req-frontmatter-path/go/from-req-baseline
falsify/scaffold-update-chmod-removed/direction-c-baseline
```

---

## 3. FAILs de hoje agrupados por causa provável

### Grupo A — NTFS noacl (#421) — 2 rótulos

**Mecanismo:** `os.Chmod` é silenciosamente ignorado em NTFS sem ACL. `chmod 0644` não remove o bit de execução, portanto a fixture "arquivo sem bit x" não pode ser construída nesta plataforma.

**Rótulos:**

`scaffold-update-chmod-removed/direction-c-baseline`
: Mensagem: "fixture inconstruivel neste sistema de arquivos — chmod 0644 nao RETIROU o bit (mecanismo da #421: NTFS noacl). GARANTIA NAO EXERCITADA: que `trackfw update --targets validate-script` RESTAURA o bit de execucao de scripts/trackfw-validate.sh previamente rebaixado (os.Chmod em generateValidateScript). Ela continua exercitada de verdade em FS POSIX; aqui NAO foi exercitada — e o 'OK' que este rotulo emitia antes desta sonda era vacuo, porque 'test -x' era verdadeiro independentemente de os.Chmod ter rodado."

`scaffold-update-chmod-removed/direction-c-detected`
: Mensagem: "nao executado — mesma fixture inconstruivel do braco de baseline acima. GARANTIA NAO EXERCITADA: que a asercao do braco de baseline DISCRIMINA."

**Novo em relação à base:** `direction-c-baseline` era ausente na base (não emitia como FAIL); `direction-c-detected` já era FAIL na base. A sonda de hoje diferencia os dois braços — o baseline agora declara explicitamente que a garantia não foi exercitada, em vez de emitir OK vacuo.

### Grupo B — Novo defeito de fiação (kiro-dedicated-file) — 1 rótulo

**Rótulo:** `git-branch-guard-global-hook-resolvable/kiro-dedicated-file/baseline`
: Mensagem: "fiação Kiro íntegra deveria passar, saiu com 1"

**Causa:** Não foi investigada nesta medição. O rótulo é novo — ausente na base x64. O cenário testa que a fiação global do git-branch-guard instalada para o arquivo dedicado Kiro resolve corretamente. Classificação ML-R2b2: "git-branch-guard-global-hook-resolvable" — subgrupo kiro-dedicated-file.

### Grupo C — Regressão de ciclo limpo — 1 rótulo

**Rótulo:** `roadmap-req-frontmatter-path/go/from-req-baseline`
: Mensagem: "ciclo limpo saiu com 1, esperava 0"

**Causa:** Não foi investigada nesta medição. O rótulo existia na base com peso calibrado (log da base shard 2 mostra "AVISO rótulo sem peso calibrado — usando peso pessimista 54.1778s" — o rótulo era novo/não-calibrado na base). Na base emitia OK; hoje emite FAIL, indicando regressão: um ciclo que deveria sair limpo sai com código 1.

---

## 4. ML-R2b1b — {owner}/{repo} braces através do shim gh

**Contexto:** ML-R2b1b do roadmap ROADMAP-2026-09-03 mediu que a conversão MSYS remove chaves `{owner}/{repo}` ao cruzar a fronteira shim→bash. O fix adicionou `MSYS=noglob`, `MSYS_NO_PATHCONV=1`, `MSYS2_ARG_CONV_EXCL=*`.

**Observação na base (36036473391):**

Os 5 setup-sXX que falhavam (`setup-s75`, `s76`, `s87-baseline`, `s158-baseline`, `s175-baseline`) indicavam que `check-release-tag-parity.sh` falhava com o binário real. No shard 4 da base, dentro de um bloco PROOF, `release-tag-parity/no-forge-cli/go` falhou com:
```
stderr: Error: trackfw release tag refuses to run: could not fetch origin
(git fetch origin --prune exited with 3221225477). Check your network/credentials and retry.
```
Exit 3221225477 = 0xC0000005 (exceção Windows, tipicamente timeout/acesso de rede). A causa dos setup-sXX era falha de rede — não conversão MSYS.

**Observação hoje (38319939952):**

8 rótulos `release-tag-parity/*` emitem OK (os da lista `baseline-clean`, `false-negative`, `bypass-baseline`, `bypass-false-negative`, `success/baseline-clean`, `lightweight-tag-false-negative`, `content-from-commit-baseline`, `content-from-commit-false-negative`). Nenhum setup-sXX FAIL para release-tag-parity. Rótulo `no-forge-cli` ausente como rótulo top-level (sem ^FAIL nem ^OK ao nível de rótulo).

**Conclusão:** A melhora nos P4 de release-tag-parity é atribuída ao fix de prazo de rede (REQ-2026-10-12 / PR #559), não ao MSYS brace fix. Os cenários que exercitariam diretamente `gh pr list --repo {owner}/{repo}` são invisíveis neste instrumento: estão dentro de blocos PROOF ou bloqueados por `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` (definido no job censo). ML-R2b1b permanece não verificável por esta sonda.

---

## 5. O que o verde de windows-full-suites esconde

**Comando:** `go test -v -p 1 -parallel 1 -timeout 20m ./...` (confirmado em `.github/workflows/quality.yml`, job `windows-full-suites`, run 38316881566, job 115035521261).

**Total de SKIPs ao nível de função:** 20.

### SKIPs que protegem critérios de aceite

| Teste | Razão (netexec_test.go / arquivo) | Critério de aceite afetado |
|-------|-----------------------------------|---------------------------|
| `TestGitExec_AC2_Bite_NoWaitDelay` | "bite test skipped on Windows: pipe hang behaviour not confirmed there" (netexec_test.go:389) | AC2 — comportamento bite/prazo de rede: pipe hang não confirmado no Windows |
| `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` | "pre-push hook test skipped on Windows: shell hooks not supported" (netexec_test.go:603) | Shell hooks em pre-push não suportados pelo Windows cmd.exe |

### Demais 18 SKIPs (POSIX/NTFS/plataforma)

| Teste | Razão |
|-------|-------|
| `TestBarrierReentry_T1_DirectReentrance` | shell gate com fusível requer sh POSIX; Windows coberto por windows-known-failures.json |
| `TestBarrierReentry_T2_IndirectReentrance` | idem |
| `TestBarrierReentry_T3_LegitimateNested` | shell gate requer sh POSIX; Windows coberto por windows-known-files.json |
| `TestBarrierReentry_T7_Canonicalization` | idem |
| `TestBarrierReentry_T7e_APFSCaseInsensitive` | os.Symlink pode falhar no Windows sem permissão de admin |
| `TestCredentialGuardScript_GeneratedScript_WindowsPathRedirectDetectsJWTInFile` | ':' is not valid in a filename on Windows — fixture cannot be created |
| `TestCredentialGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar` | bits de permissão POSIX não se aplicam no Windows |
| `TestCredResolveArg_GitBashPath_POSIXUnchanged` | POSIX test — skipped on Windows |
| `TestD11FailClosed_POSIX_TabelaRC` | linha D11 PS/POSIX não se aplica diretamente ao cmd.exe — skip em Windows |
| `TestD11FailClosed_PowerShell_TabelaRC` | teste usa stub #!/bin/sh — não aplicável em cmd.exe/PS no Windows desta forma |
| `TestGitBranchGuardAwk_ProvaDeMordida` | git show 3b2eff09: não disponível (exit status 128) — pulando prova de mordida |
| `TestGitBranchGuardScriptIntegrity_ScriptIlegivel_ViolationSemAbortar` | bits de permissão POSIX não se aplicam no Windows |
| `TestGuardGlobalScriptIntegrity_ScriptIlegivel_ViolationSemSilencio` | bits de permissão POSIX não se aplicam no Windows |
| `TestGuardHookResolvable_ProjectPermissionDenied_DisparaViolationSemCrash` | bits de permissão POSIX não se aplicam no Windows |
| `TestMatchSubcommand_BackslashPrefixed_PosixNotGit` | POSIX-only: on Windows filepath.Base('\\git') returns 'git' — divergence is intentional |
| `TestMdBasenamesInGitTree_LiteralPathspecDirName` | NTFS reserva ':' em nome de arquivo — o diretório ':(exclude)rm' não pode existir, então o vetor RN1 não existe nesta plataforma |
| `TestPush_ForceWithLease_NoPROpen_Blocks` | force-with-lease+no-PR path has no end-to-end coverage; see docs/cli-parity.md partial= annotation |
| `TestReqDoneOpenCriteria_DoubleQuoteBranchName` | NTFS não permite `"` em nomes de arquivo; ataque não é montável nessa plataforma |

Adicionalmente, `compare_baseline_test.go` emite SKIPs de sub-teste para roadmaps com `RC=SKIP` ou `empty stdout (usage error in baseline)` — esses não aparecem como `--- SKIP:` de função e não são contados nos 20 acima.

---

## 6. ARM64 VM vs x64 — TestADRPrefixE2E_*

**Contexto:** Na VM privada ARM64 (medição 2026-09-08), os testes `TestADRPrefixE2E_*` falhavam com "unable to access 'NUL'" — 28 FAILs reportados.

**Resultado no x64 GitHub-hosted (run 38316881566, job 115035521261):**

```
TestADRPrefixE2E_AC2  PASS
TestADRPrefixE2E_AC3  PASS
TestADRPrefixE2E_AC5  PASS
TestADRPrefixE2E_AC6  PASS
TestADRPrefixE2E_AC7  PASS
```

**Conclusão:** Todos os 5 testes PASS no x64. A falha "unable to access 'NUL'" é artefato da VM privada ARM64 — não reproduz no runner `windows-latest` (Windows Server 2025) hospedado pelo GitHub. A causa provável na VM privada é a incompatibilidade de `git init -q -b main .` com a configuração Git daquela VM; no x64 o Git 2.x GitHub-hosted resolve NUL corretamente.

---

## Tabela final

| Causa | Nº rótulos | Produto ou gate | Sítio | Segurança? | Fecha o quê |
|-------|-----------|-----------------|-------|------------|-------------|
| NTFS noacl — os.Chmod silencioso em NTFS sem ACL (#421); garantia `trackfw update --targets validate-script` não exercitada | 2 | produto: `trackfw update` (generateValidateScript) | scaffold-update-chmod-removed/direction-c-baseline + direction-c-detected | Não | #421 (aberto) |
| Novo defeito de fiação kiro-dedicated-file — fiação Kiro global deveria passar, saiu com 1 | 1 | gate: falsify git-branch-guard-global-hook-resolvable | kiro-dedicated-file/baseline | Não | a determinar (causa não investigada) |
| Regressão de ciclo limpo roadmap-req-frontmatter-path — ciclo limpo saiu com 1, esperava 0 | 1 | gate: falsify roadmap-req-frontmatter-path/go | from-req-baseline | Não | a determinar (causa não investigada) |

---

## Limites desta medição

1. **Base x64 lida de logs de job, não de artefatos.** Os artefatos da run 36036473391 expiraram (retenção de 7 dias; data do run: 2026-09-24, 18 dias antes desta medição). Os 11 FAILs da base foram extraídos via `gh run view --job <id> --log`. Erros de extração não podem ser descartados — o OK total de 347 foi reportado pelo job apuracao da base (job 107760932675) e não foi reverificado linha a linha.

2. **ARM64 base para apuracao é hardcoded.** Os valores OK=440 FAIL=512 foram medidos na VM privada em 2026-09-08 e estão fixos no workflow. Qualquer variação de comportamento da VM nesse período não é capturada.

3. **ML-R2b1b não verificável.** Os cenários que exercitariam `gh pr list --repo {owner}/{repo}` estão dentro de blocos PROOF ou bloqueados por `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`. A melhora nos P4 de release-tag-parity é atribuída ao fix de rede (REQ-2026-10-12), não ao MSYS brace fix.

4. **Grupos B e C (kiro-dedicated-file, roadmap-req-frontmatter-path) sem investigação de causa raiz.** Esta medição classifica os rótulos como novos FAILs e descreve as mensagens; não rastreia a causa até o código.

5. **Rótulo direction-c-baseline novo em relação à base.** Na base x64 (2026-09-24) o rótulo não existia como FAIL — ou o cenário não foi gerado naquele chunk, ou emitia OK vacuo. A sonda foi atualizada entre as duas datas para declarar explicitamente "garantia não exercitada" em vez de emitir OK.

6. **SKIP de `TestGitExec_AC2_Bite_NoWaitDelay` e `TestGitExec_WaitDelay_ExitZeroWithHangingGrandchild` persistem.** Esses testes protegem comportamentos de rede/shell que não têm cobertura equivalente no Windows; o skip é correto, mas a cobertura desses ACs no Windows permanece zero.

7. **Contagem OK inclui linhas unlabeled.** O `apuracao` usa `grep -ac '^OK'` — 30 das 343 linhas OK são `OK   vacuidade: N >= N` e similares, não rótulos. A contagem de rótulos nomeados é 313.
