## 2026-10-23 — apolo-tf — FIM: ML-1E para REQ-2026-10-23 (Amazon Q credential PSPOSIX)

**Worktree:** `/private/tmp/trackfw-toml-escape`
**Branch:** `fix/update-nao-remove-geracoes-antigas-do-credential-guard`

### O que foi feito

`InjectAmazonQHooks` (`internal/generators/agentfiles.go`) agora migra também
`guardforms.CredentialLegacyPSPOSIX` para `guardCredentialCmdCmdExe` nos dois pares de
credential que emite (`execute_bash` e `fs_write`). O padrão segue o git-branch do mesmo
sítio (ML-1C): lista combinada construída uma vez como `credentialLegacyAll` e passada a
ambas as chamadas de `migrateDedupGuardInNestedArray`.

Em `internal/generators/guard_registry_sites_ac4_test.go`:
- `amazonQCredentialList` adicionada (CmdExe + PSPOSIX, análoga a `amazonQGitBranchList`).
- Linha `InjectAmazonQ-cred` atualizada para iterar a lista combinada.
- Nova linha `InjectAmazonQ-cred-fswrite` (mesma lista, matcher `fs_write`).
- Comentário de lacuna removido do corpo da tabela e do cabeçalho do arquivo.

### Gates

```
go build ./...                                              rc=0
go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ -count=1
  ok  github.com/kgsaran/trackfw/internal/generators  60.669s
  ok  github.com/kgsaran/trackfw/internal/validator   30.148s
  ok  github.com/kgsaran/trackfw/internal/pathguard    1.162s
                                                            rc=0
```

### Contra-braço

Snapshot de HEAD extraído em scratchpad; teste modificado copiado. 7 subtests PSPOSIX
reprovam em HEAD (legacy sobrevive, canonical duplicado). As 2 formas que passam em HEAD
estão corretas: `LegacyBareCredGuard` e `LegacyD2CredentialCmdExe` já eram tratadas por
`migrateHookCommand` nas linhas 1872-1873.

## 2026-10-23 — apolo-tf — INÍCIO: ML-1E para REQ-2026-10-23 (Amazon Q credential PSPOSIX)

- Escopo: migrar CredentialLegacyPSPOSIX no sítio AmazonQ, teste AC4 atualizado.
- Worktree: `/private/tmp/trackfw-toml-escape`. Branch: `fix/update-nao-remove-geracoes-antigas-do-credential-guard`.
