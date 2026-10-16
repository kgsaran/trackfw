# P19 — Ataque por Segunda Identidade Não-Admin (ML-W2a-bis)

**Run ID:** 38918006930  
**Branch:** fix/fechar-os-grupos-de-falha-de-windows-ml-w2-permissao  
**Commits:** `0f568437` (script+workflow) · `36429ef6` (fix: helper inside module) · `85985334` (fix: New-LocalUser)  
**Data:** 2026-10-16  
**Status do step P19:** ✓ (green)

---

## Contexto

Mede se um usuário Windows local não-admin (`p19atk`) consegue comprometer
`thirdparty-provenance.json` protegido por DACL explícita (fsperm/pathguard),
em quatro braços independentes.

---

## Resultados por Braço

| Braço | Ação | Resultado | Detalhe |
|---|---|---|---|
| overwrite | sobreescrever arquivo diretamente | **OVERWRITE_DENIED** | `Access to the path … is denied.` |
| overwrite | deletar arquivo diretamente | **DELETE_DENIED** | `Access to the path … is denied.` |
| swap-root-rename | renomear `C:\p19-proj` (sem processo prendendo cwd) | **RENAME_SUCCEEDED** | — |
| swap-root-recreate | recriar raiz forjada após rename | **RECREATE_SUCCEEDED** | — |
| swap-root-cwd | renomear `C:\p19-proj` com processo vítima prendendo cwd | **RENAME_DENIED** | `The process cannot access the file because it is being used by another process.` |
| pre-create | vítima escreve provenance após atacante pré-criar `.trackfw` | **VICTIM_WRITE_SUCCEEDED** (unexpected_success) | SetNamedSecurityInfo da vítima ainda funciona |
| pre-create-dacl | DACL do dir `.trackfw` após escrita da vítima | **DACL_CHANGED** | vítima reapplica DACL restritiva |
| pre-create-overwrite | atacante tenta sobreescrever após DACL_CHANGED | **OVERWRITE_DENIED** | `Access to the path … is denied.` |
| junction | atacante remove `.trackfw` para plantar junction | **REMOVE_FAILED** | `attacker_could_not_remove_trackfw` |

---

## Linhas Brutas (PROBE w2x)

```
PROBE w2x overwrite result=OVERWRITE_DENIED detail=MethodInvocationException: Exception calling "WriteAllText" with "2" argument(s): "Access to the path 'C:\p19-proj\.trackfw\thirdparty-provenance.json' is denied."
PROBE w2x overwrite-delete result=DELETE_DENIED detail=UnauthorizedAccessException: Access to the path 'C:\p19-proj\.trackfw\thirdparty-provenance.json' is denied.
PROBE w2x swap-root-rename result=RENAME_SUCCEEDED
PROBE w2x swap-root-recreate result=RECREATE_SUCCEEDED
PROBE w2x swap-root-cwd result=RENAME_DENIED detail=IOException: The process cannot access the file because it is being used by another process.
PROBE w2x pre-create result=VICTIM_WRITE_SUCCEEDED detail=unexpected_success
PROBE w2x pre-create-dacl result=DACL_CHANGED
PROBE w2x pre-create-overwrite result=OVERWRITE_DENIED detail=MethodInvocationException: Exception calling "WriteAllText" with "2" argument(s): "Access to the path 'C:\p19-projb\.trackfw\thirdparty-provenance.json' is denied."
PROBE w2x junction result=REMOVE_FAILED detail=attacker_could_not_remove_trackfw
```

---

## Interpretação por Braço

**overwrite / overwrite-delete:** DACL explícita bloqueia acesso direto ao arquivo. Proteção funciona como esperado.

**swap-root-rename / swap-root-recreate:** Quando o processo vítima *não* está rodando (cwd livre), o atacante consegue renomear `C:\p19-proj` inteiro e recriar um diretório homônimo com conteúdo forjado. Trata-se de uma janela de ataque de substituição de raiz — não é coberta pela DACL do arquivo, que protege *dentro* da árvore.

**swap-root-cwd:** Quando o processo vítima mantém um handle aberto para o diretório raiz, a tentativa de rename falha com `ERROR_SHARING_VIOLATION`. A proteção depende do processo estar em execução.

**pre-create (R-A):** Ao contrário da hipótese inicial, a vítima consegue escrever provenance mesmo que o atacante tenha pré-criado `.trackfw`. Isso ocorre porque `SetNamedSecurityInfo` da vítima sobrescreve a DACL do diretório (`DACL_CHANGED`), restabelecendo a proteção. Após isso, a tentativa de overwrite do atacante é negada. O braço R-A não representa uma vulnerabilidade explorável pelo atacante.

**junction (R-B):** O atacante não conseguiu remover `.trackfw` para plantar uma junction. Como o diretório já foi criado pela vítima com DACL restritiva, o atacante não tem permissão para removê-lo. O ataque de junction não é realizável quando a vítima inicializou o diretório primeiro.

---

## Limites

1. **Ambiente de CI vs. produção:** O runner GitHub Actions usa `windows-latest` (Server 2022). Políticas de grupo corporativas podem alterar DACLs herdadas ou restringir `New-LocalUser` / `Start-Process -Credential`.

2. **Granularidade temporal no swap-root:** O braço swap-root-cwd é uma *aproximação sequencial* do ataque concorrente real. Não há garantia de que `RENAME_DENIED` se manteria sob load concorrente real — seria necessário um teste com threads paralelas para medir a janela de race.

3. **p19atk sem privilégios de SeBackupPrivilege / SeTakeOwnership:** O atacante simulado é estritamente não-admin. Um atacante que escalou privilégios (ex.: via UAC bypass) não está coberto por esta medição.

4. **Sem teste de sobrevivência de handle:** O braço swap-root-cwd não verifica se o handle de cwd persiste sob reinicialização do processo vítima. A proteção cessa se o processo for encerrado entre tentativas.

5. **Helper Go descartado:** `cmd/p19-helper-probe/` é criado e removido em tempo de CI. Se a limpeza falhar (ex.: crash do runner), o diretório pode ficar no workspace — inerte, mas visível em `git status`.

6. **Binário reutilizado de P17:** O step P19 primeiro procura o binário compilado por P17. Se P17 não rodou, compila `./cmd/trackfw` diretamente. Neste run, P17 estava disponível (✓).
