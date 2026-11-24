# Revisão de Qualidade — ML-2C (REQ-2026-11-23, Wave 2)

**Veredito: APROVA COM RESSALVAS**

Data: 2026-11-24  
Revisor: hefesto-tf  
Branch: `fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado`  
Delta: `git diff 28c03d8c 249804fb`  
Contrato: `docs/adr/ADR-2026-11-24-guards-resolvem-trackfw-yaml-de-forma-monotonica-subagente-nao-cria-raiz-nem-afrouxa-regra.md` (D1–D6, incluindo emendas da Wave 2)  
Revisão anterior referenciada: `docs/seguranca/2026-11-24-revisao-write-scope-raiz-aninhada.md`

Build e testes passam sem erros:

```
ok  github.com/kgsaran/trackfw/internal/guard      3.354s
ok  github.com/kgsaran/trackfw/internal/generators 90.625s
```

Razão do veredito APROVA COM RESSALVAS: todos os achados bloqueantes da revisão anterior foram
fechados. Seis achados novos de severidade BAIXO são identificados abaixo; nenhum bloqueia o merge.

---

## 1. Verificação dos achados anteriores

| Achado | Arquivo:linha | Status |
|---|---|---|
| MÉDIO-1 — `credFindNearestProjectRoot` violates D3 | `credential.go` | ✅ FECHADO |
| MÉDIO-2 — 5 frases de reconciliação incorretas | vários testes | ✅ FECHADO |
| BAIXO-1 — `cli-parity.md` global scope attention signal | `cli-parity.md` | ✅ FECHADO |
| BAIXO-2 — Windows ADS contorna D1 | `writescope.go` | ✅ FECHADO |
| BAIXO-3 — comentário impreciso em `TestD3_DirectoryNamedTrackfwYAML_Deny` | `writescope_nested_root_test.go` | ✅ FECHADO |
| BAIXO-4 — `findProjectRoot` wrapper em código de produção | `writescope.go` | ✅ FECHADO |

### MÉDIO-1

`credFindNearestProjectRoot` (`credential.go:1492`) agora retorna `(string, error)`. Um erro
não-IsNotExist durante a detecção de raiz retorna `(dir, fmt.Errorf(...))`. O chamador `RunCredential`
(`credential.go:238–243`) nega (rc=2) ao receber esse erro.

Teste novo `TestCredNested_DirectoryYamlDenies` (`credential_nested_mode_test.go`) e contra-braço
`TestCredNested_NoYamlAllows`: ambos passam. Cenário B (chmod 000) não foi coberto por teste — ver
item de não-bloco abaixo.

### MÉDIO-2

Todas as 5 frases corrigidas:
1. `TestCredNested_GlobalPlantedWarnBlocks` — nova sentença: "RunCredentialGlobal hardcodes block
   without calling credWalkStrictestMode." ✓
2. `TestCredNested_GlobalNoYamlBlocks` — nova sentença: "RunCredentialGlobal hardcodes block
   without reading any yaml." ✓
3. `TestReadCredentialGuardMode_EndToEnd_GlobalScope_WarnCRLFStillBlocks` — renomeado para
   `TestReadCredentialGuardMode_EndToEnd_ProjectScope_CRLFBlockWorks`; migrado para exercitar
   `RunCredential` com `mode: block` em CRLF. ✓ (ver BAIXO-9 abaixo sobre a frase do teste)
4. `TestGlobalCredentialGuardScript_RespectsExplicitProjectMode` — comentário corrigido. ✓
5. `TestGlobalCredentialGuardScript_ProjectModeWarnDoesNotDowngradeGlobalBlock` — comentário
   corrigido. ✓

### BAIXO-1

`cli-parity.md` agora descreve: "Escopo global nunca grava attention JSON." ✓

### BAIXO-2

`normalizePathComponent` (`writescope.go:917`) agora corta no primeiro `:` antes de remover
ponto/espaço finais. `hasTrackfwYAMLComponent` (`writescope.go:941`) testa também
`trackfwYAML8dot3RE` (`writescope.go:961`). D6 (`frozenHit`, `writescope.go:774`) adiciona
comparação com grafia normalizada para a lista de congelados.

### BAIXO-3

Bloco de comentário em `writescope_nested_root_test.go` reescrito para descrever corretamente
o mecanismo D3: `findProjectRootSafe` encontra `ErrNotRegularFile` → retorna erro → `RunWriteScope`
nega. ✓

### BAIXO-4

`findProjectRoot` (wrapper de 2 retornos) removido de `writescope.go` e movido para
`writescope_ml2c_test.go:19–30`. Nenhuma referência ao wrapper sobreviveu em código de produção:

```
$ grep -rn "findProjectRoot\b" internal/ --include="*.go" | grep -v "_test.go"
internal/guard/writescope.go:602:  return dir, true, fmt.Errorf("findProjectRoot: ..."  ← só em string de erro
```

A string em 602 é o nome antigo numa mensagem de erro — ver BAIXO-5 abaixo.

---

## 2. Regra de Reconciliação — testes novos e alterados

### `writescope_ml2c_test.go` (arquivo novo — 16 testes)

| Teste | Sentença | Correta? | Passa sem a correção? |
|---|---|---|---|
| `TestD1_NormalizePathComponent_Unit` | "cutting at the first ':' and stripping trailing '.' and ' ' converts ADS/trailing-dot variants to the base filename; .bak suffix survives." | ✓ | Falha (ADS case) |
| `TestD1_HasTrackfwYAMLComponent_Unit` | "ADS variants, 8dot3, and uppercase are recognized as trackfw.yaml components; .bak suffix and 8dot3-of-.yaml are not." | Parcial — ver BAIXO-10 | Falha (ADS/8dot3 cases) |
| `TestD1_ADSTrackfwYAML_Deny` | "D1 — normalizePathComponent strips the ADS suffix before EqualFold." | ✓ | Falha (rc=0) |
| `TestD1_PartialADS_Deny` | "D1 — normalizePathComponent cuts at the first colon." | ✓ | Falha |
| `TestD1_ADSWithStreamName_Deny` | "D1 — only the part before the first colon is kept." | ✓ | Falha |
| `TestD1_8dot3TrackfwYAML_Deny` | "D1 — hasTrackfwYAMLComponent recognises the 8dot3 pattern TRACKF~<n>.YAM." | ✓ | Falha (rc=0) |
| `TestD1_YAMLBak_Allow` | "D1 only denies exact trackfw.yaml match; .bak suffix is not caught." | ✓ | Passa (contra-braço) |
| `TestD6_TrailingDotSettings_Deny` | "D6 — frozenHit compares also the path with each component normalized." | ✓ | Falha (rc=0) |
| `TestD6_ADSSettings_Deny` | "D6 — normalizePathComponent strips ADS suffix, matching frozen '.claude/settings.json'." | ✓ | Falha (rc=0) |
| `TestD6_TrailingSpaceSettings_Deny` | "D6 — same normalization strips trailing spaces." | ✓ | Falha (rc=0) |
| `TestD6_ExactSettingsJson_Deny` | "exact frozen path is denied before and after the fix; confirms fixture is not broken." | ✓ | Passa (baseline arm) |
| `TestD6_MainSession_TrailingDot_NoFreeze` | "D6 normalized frozen check applies to subagents; main session allows normalized spelling." | ✓ | Passa (contra-braço) |
| `TestD5_AncestorLeaseConsulted_Deny` | "D5 — collectLeaseRoots includes every ancestor with trackfw.yaml; outer lease gates access." | ✓ | Falha (rc=0 sem D5) |
| `TestD5_AncestorLeaseConsulted_AllowCounterArm` | "D5 — ancestor lease is consulted AND its allow list is honoured." | Incorreta — ver BAIXO-7 | Passa (rc=0 nos dois casos) |
| `TestD3_CollectLeaseRoots_NonRegularAncestorYAML_Error` | "D3 — a non-regular trackfw.yaml in an ancestor causes collectLeaseRoots to return error." | ✓ | Falha (err=nil) |
| `TestD1_8dot3Regex_CompileAndMatch` | "(?i)^trackf~[0-9]+\\.yam$ matches TRACKF~1.YAM and trackf~999.yam but not trackfw.yaml." | Incorreta — ver BAIXO-6 | Passa nos dois casos |

### `credential_nested_mode_test.go` (alterado)

| Teste | Sentença | Correta? | Sensível? |
|---|---|---|---|
| `TestCredNested_GlobalPlantedWarnBlocks` | Hardcodes block, não chama credWalkStrictestMode. | ✓ | Mesmo comportamento nos dois casos |
| `TestCredNested_GlobalNoYamlBlocks` | Unconditional block. | ✓ | Mesmo comportamento |
| `TestCredNested_DirectoryYamlDenies` | D3 — directory yaml → rc=2. | ✓ | Falha antes (rc=0) |
| `TestCredNested_NoYamlAllows` | No yaml → rc=0. | ✓ | Passa nos dois casos (contra-braço) |

### `credential_mode_test.go` (alterado)

`TestReadCredentialGuardMode_EndToEnd_ProjectScope_CRLFBlockWorks` — ver BAIXO-9.

### `credential_guard_test.go` (alterado)

| Teste | Sentença | Correta? |
|---|---|---|
| `TestGlobalCredentialGuardScript_RespectsExplicitProjectMode` | Script calls `trackfw guard credential`; mode:block → rc=2. | ✓ |
| `TestGlobalCredentialGuardScript_ProjectModeWarnDoesNotDowngradeGlobalBlock` | RunCredentialGlobal hardcodes block; yaml não é lido. | ✓ |
| `TestCredentialGuardScript_DirectoryYaml_CallsTrackfw` | D3 wrapper fix — `[ -e ] \|\| [ -L ]` replaces `[ -f ]`; stub exits 2. | ✓ |

---

## 3. D3 no delta — `err != nil` tratado como ausente

| Localização | Padrão | Conforme D3? |
|---|---|---|
| `credential.go:1500–1510` | `if !os.IsNotExist(err) { return dir, fmt.Errorf(...) }` | ✓ |
| `writescope.go:390–395` (`collectLeaseRoots`) | `if !os.IsNotExist(readErr) { return nil, fmt.Errorf(...) }` | ✓ |
| `writescope.go:697–703` (`buildFrozenList`) | `if !os.IsNotExist(readErr) { return nil, fmt.Errorf(...) }` | ✓ preexistente |
| Shell wrapper (3 cópias) | `[ -e ... ] \|\| [ -L ... ]` | ✓ |

Nenhum `err != nil → continue/fall-through` novo.

---

## 4. `normalizePathComponents` e `frozenHit`

**Volume Windows (`C:`, UNC):**

- `filepath.VolumeName("C:\\foo\\bar")` = `"C:"` → preservado; apenas os componentes de caminho normalizados. ✓
- UNC (`\\\\server\\share\\foo`): `VolumeName` = `"\\\\server\\share"`, resultado: `"\\\\server\\share\\foo"`. ✓
- POSIX: resultado `"/foo/bar"`. ✓

**Separador:** `FieldsFunc` divide em `filepath.Separator`; `strings.Join` reconstrói com o mesmo separador. ✓

**Falso positivo aceito pelo ADR (arquivo com `:`):** no POSIX, `settings.json:x` é normalizado para
`settings.json` e nega se `settings.json` for congelado. ADR D6 aceita explicitamente. ✓

**Falso positivo não coberto pelo ADR (diretório com `:`):** ver BAIXO-8.

**`norm != resolved` em `frozenHit`:** guarda de curto-circuito correta — se a normalização não
altera o caminho, o primeiro loop já cobriu. ✓

---

## 5. `collectLeaseRoots` — ordem, duplicação, interação com "todo lease autoriza"

**Ordem:** `[targetRoot, ancestor₁ (mais próximo), ..., ancestorN, cwdRoot se distinto]`. ✓

**Deduplicação:** mapa `seen` via igualdade de string. Nota: em filesystems case-insensitivos,
duas representações do mesmo caminho não são deduplicadas. Isso é inofensivo (leases duplicados
são validados e ambos devem autorizar — idempotente). O doc da função diz "not path-equal", o que
descreve o comportamento novo após a remoção do `pathEqual`; a precisão terminológica é uma
observação, não um bloco.

**Interação com "todo lease encontrado precisa autorizar":** Step 11f de `RunWriteScope`
(`writescope.go:338–367`) nega se qualquer lease não autorizar o alvo. ✓

**Alcançabilidade de `lrErr`:** `buildFrozenList` (linha 206) percorre os mesmos ancestrais com a
mesma verificação D3. Se `collectLeaseRoots` fosse retornar erro, `buildFrozenList` já teria negado
antes. A verificação D3 em `collectLeaseRoots` é defense-in-depth (válida, não um defeito), mas
unreachable end-to-end salvo TOCTOU. Ver item de não-bloco abaixo.

---

## 6. Byte-identidade das três cópias do wrapper

Verificado por comparação direta com `sed` + `diff`:

```
$ sed -n '/^const credentialGuardScript = `/,/^`$/p' internal/generators/scaffold.go \
    | sed '1s/^const credentialGuardScript = `//; $d; s/` + "`" + `/`/g' \
    | diff - scripts/trackfw-credential-guard.sh && echo IDENTICAL_SCAFFOLD
IDENTICAL_SCAFFOLD

$ sed -n '/^const credentialGuardScriptReference = `/,/^`$/p' \
    internal/validator/validator_credential_guard_integrity_reference.go \
    | sed '1s/^const credentialGuardScriptReference = `//; $d; s/` + "`" + `/`/g' \
    | diff - scripts/trackfw-credential-guard.sh && echo IDENTICAL_REFERENCE
IDENTICAL_REFERENCE
```

As três cópias são byte-idênticas. ✓

---

## 7. Código morto e comentários incorretos

**`findProjectRoot` em produção:** removido. ✓

**`regexp` import comment:** correto — ambas as variáveis `agentTypeSlugRE` (linha 26) e
`trackfwYAML8dot3RE` (linha 961) existem em `writescope.go`. ✓

**Comentário de ordem em `TestD1_NormalizePathComponent_Unit`:** linha 48:
`"// trailing dot + ADS: strip dot first, then cut at colon"`. O código corta no `:` primeiro
(step 1), depois remove `.` e espaço finais (step 2). A ordem descrita no comentário é a inversa
da ordem do código. Cosmético; o resultado do teste está correto.

---

## Achados novos

### BAIXO-5 — Stale function name em error message (não bloqueia)

**Arquivo:** `internal/guard/writescope.go`, linha 602  

```go
return dir, true, fmt.Errorf("findProjectRoot: trackfw.yaml at %q: %w", dir, readErr)
```

A mensagem cita `"findProjectRoot"` mas a função de produção é `findProjectRootSafe`. O wrapper
`findProjectRoot` foi movido para código de teste. Um diagnóstico de stderr nomeia uma função
inexistente em produção. Sem impacto semântico.

**Correção sugerida:** substituir a string por `"findProjectRootSafe:"`. Uma linha.

---

### BAIXO-6 — `TestD1_8dot3Regex_CompileAndMatch` decorativo (Regra de Reconciliação) (não bloqueia)

**Arquivo:** `internal/guard/writescope_ml2c_test.go`, linhas 422–436  

O teste compila sua **própria** literal de regex (`regexp.MustCompile(...)` na linha 423) e nunca
referencia a variável de produção `trackfwYAML8dot3RE` (`writescope.go:961`). Se a variável de
produção fosse alterada (padrão diferente, bug introduzido), este teste continuaria passando.

A sentença do teste diz: "the regexp `(?i)^trackf~[0-9]+\\.yam$` matches TRACKF~1.YAM and
trackf~999.yam but not trackfw.yaml." Isso é verdade para a literal embutida no teste, mas não
afirma nada sobre `trackfwYAML8dot3RE`. O teste é decorativo em relação à variável de produção.

**Cobertura existente:** `TestD1_8dot3TrackfwYAML_Deny` e `TestD1_HasTrackfwYAMLComponent_Unit`
exercitam `hasTrackfwYAMLComponent` → `trackfwYAML8dot3RE` via caminho completo. O teste de
compilação é redundante para a proteção real.

**Correção sugerida (uma de duas):**  
(a) Substituir a literal por `trackfwYAML8dot3RE.MatchString(...)`, tornando o teste sensível à
variável de produção; ou  
(b) Remover o teste, pois `TestD1_HasTrackfwYAMLComponent_Unit` já cobre as mesmas condições de
match/no-match via código de produção.

---

### BAIXO-7 — `TestD5_AncestorLeaseConsulted_AllowCounterArm` não exercita o walk de D5 (Regra de Reconciliação) (não bloqueia)

**Arquivo:** `internal/guard/writescope_ml2c_test.go`, linhas 338–374

O alvo do contra-braço é `allowedPath = filepath.Join(outer, "docs", "x.md")`. O projeto mais
próximo desse alvo é `outer/` (que tem `trackfw.yaml`). Portanto `targetRoot = outer`.

`collectLeaseRoots(outer, svc)`:
- **Antes do D5 (código antigo):** `targetRoot = outer`, `cwdRoot = svc` → retorna `[outer, svc]`
- **Após o D5 (código novo):** walk a partir de `Dir(outer)` não encontra mais yamls; adiciona
  `svc` → retorna `[outer, svc]`

Resultado idêntico nos dois casos. O walk ancestral que D5 acrescentou nunca é ativado neste teste
(o alvo já pertence ao projeto mais externo). A sentença "D5 — the ancestor lease is consulted AND
its allow list is honoured" descreve o comportamento pós-D5, mas o mecanismo exercitado é o mesmo
que o pré-D5 (cwdRoot distinto de targetRoot, já coberto pelo código anterior ao ML-2C).

**Uma versão discriminante:** target = `svc/main.go`, lease de `outer/` explicitamente
`allow:[svc/main.go]`, sem lease em `svc/`. Antes do D5: nenhum lease encontrado → rc=0 (Step 11d:
no leases → allow). Após D5: lease de `outer/` consultado, permite → rc=0. O contra-braço provaria
que D5 não nega indevidamente um caminho explicitamente autorizado pelo lease externo.

---

### BAIXO-8 — Falso positivo POSIX para componente de diretório com `:` não declarado no ADR D6 (não bloqueia)

**Arquivo:** `docs/adr/ADR-2026-11-24-*.md`, seção D6, e `internal/guard/writescope.go:793`

`normalizePathComponents` aplica `normalizePathComponent` a **todos** os componentes do caminho,
incluindo diretórios. No POSIX, `:` é legal em nome de diretório. Um caminho como
`/proj/dir:old/settings.json` seria normalizado para `/proj/dir/settings.json`. Se
`dir/settings.json` estiver na lista de congelados, o alvo `dir:old/settings.json` seria negado —
falso positivo.

O ADR D6 aceita o falso positivo para nomes de **arquivo** (`settings.json:x`). A extensão ao
caso de **diretório** (`dir:x/settings.json`) não está declarada como residual aceito.

A remoção do corte em `:` nos componentes de diretório quebraria o caso Windows de NTFS streams em
diretórios (`.claude::$INDEX_ALLOCATION`), então a correção correta é declarar o residual no ADR,
não alterar o código.

**Correção sugerida:** acrescentar ao ADR D6 (seção "Falso positivo aceito"): "O mesmo corte em `:`
se aplica a todos os componentes do caminho, incluindo diretórios. No POSIX, um diretório com `:`
no nome é raro; a negação de `dir:x/file.json` quando `dir/file.json` está congelado é aceita pela
mesma razão que o caso de arquivo."

---

### BAIXO-9 — Sentença de `TestReadCredentialGuardMode_EndToEnd_ProjectScope_CRLFBlockWorks` imprecisa sobre qual passo remove `\r` (não bloqueia)

**Arquivo:** `internal/guard/credential_mode_test.go`

A sentença diz: "the CRLF strip in `ReadCredentialGuardMode` is essential and exercised here."
No fixture do teste, o yaml tem `"  mode: block\r\n"`. O split por `\n` produz a linha
`"  mode: block\r"`. O passo `trimmed := strings.TrimSpace(line)` (linha 1448 de `credential.go`)
remove o `\r` antes que o Step 7 (`TrimRight(val, "\r")`) seja alcançado. O teste passaria mesmo
se o Step 7 fosse removido, porque `TrimSpace` já lida com o caso simples.

O Step 7 é essencial para o caso onde `\r` aparece após o valor mas antes de um comentário inline
(e.g., `"  mode: block\r  # comment"`) — situação que `TrimSpace` não alcança. O teste não cobre
esse caso.

Comportamento do código está correto. A sentença descreve parcialmente o mecanismo e poderia
induzir um mantenedor a pensar que apenas o Step 7 importa.

---

## Itens de não-bloco registrados

| Item | Natureza |
|---|---|
| Cenário B de MÉDIO-1 (chmod 000 / EACCES) sem teste | Sugerido na revisão anterior; não exigido pelos critérios de aceite do roadmap |
| `lrErr` em `RunWriteScope` unreachable salvo TOCTOU | Defense-in-depth; `buildFrozenList` já nega antes. Não é um defeito |
| `cli-parity.md` linha 6648: "yaml não-regular conta como `block` (D3)" | Pós-ML-2C, yaml não-regular na detecção de raiz nega via rc=2 antes da seleção de modo — a descrição "conta como block" é imprecisa para o novo comportamento de `credFindNearestProjectRoot`. Não bloqueia; atualização do doc é de baixo custo |
| Comentário em `TestD1_NormalizePathComponent_Unit` linha 48 inverte a ordem dos passos | Cosmético |
| `collectLeaseRoots` doc: "not path-equal" vs. igualdade de string | Duplicatas inofensivas; mais uma observação do que um defeito |

---

## Bloqueante para merge

Nenhum.

---

## `git status --porcelain`

```
?? docs/seguranca/2026-11-24-revisao-ml-2c-write-scope.md
```
