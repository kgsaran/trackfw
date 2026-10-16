# Red-team ML-W2b' — regra `project_foreign_write_windows` (gate de PR)

**Data:** 2026-10-16
**Revisor:** hades-tf
**Branch:** `fix/fechar-os-grupos-de-falha-de-windows-ml-w2-permissao`
**REQ:** REQ-2026-09-03
**Prova Windows:** run P18, ID 38923063026

---

## Veredito

**LIBERA COM RESSALVAS.**

Nenhuma das ressalvas bloqueia individualmente: a regra é `applyRuleWarnOnly` (nunca bloqueia validate) e os vetores de ataque medidos (P17/P19) são detectados. As ressalvas são gaps de cobertura (R-1), violações de D3 no código (R-2, R-3) e qualidade de testes e mensagem (R-4, R-5). Cada uma deve ser endereçada em follow-up ML ou na própria ADR antes de PR merged.

---

## Itens

### Item 1 — Falso negativo

**Vetor principal (P19 swap-root via AU Modify herdado de C:\\):** FECHA.
`DELETE` (0x10000) está em `d2WriteMask`; `AU` (S-1-5-11) não está no conjunto confiado;
`GetNamedSecurityInfo` retorna ACEs herdadas sem `INHERIT_ONLY_ACE` → `appliesToObject = true` →
regra emite aviso para a raiz. O run P17 confirmou que `C:\p17-proj` herda `AU: Modify` de `C:\`;
o run P18 (a) confirmou que o aviso nomeia `S-1-5-11`. Vetor medido coberto.

**ACCESS_ALLOWED_OBJECT_ACE / callback ACEs (tipos 5, 9, 11):** silenciosamente ignorados pelo
`continue` (código filtra apenas `ACCESS_ALLOWED_ACE_TYPE = 0`). Não aparecem em objetos NTFS
comuns. Resíduo não declarado na ADR. Nomear em ADR D2 como resíduo aceito.

**Group SIDs que incluem o usuário atual (INTERACTIVE S-1-5-4, AU S-1-5-11):** tratados como
alheios → aviso emitido. Correto per D2 ("SID fora do conjunto confiado"). FECHA.

**ACE com apenas `CONTAINER_INHERIT_ACE | INHERIT_ONLY_ACE` na raiz do projeto** — **R-1:**
`appliesToObject = false` (IO set), `inheritableToFile = false` (OI ausente) → `continue`.
Se o atacante obtém `FILE_ADD_SUBDIRECTORY` na raiz via exatamente esse padrão de flags, pode
pré-criar `.trackfw` sem que a regra detecte o direito na raiz. Após a pré-criação, o atacante
é owner de `.trackfw` e tem `WRITE_DAC` implícito. Se `.trackfw` tiver DACL "limpa" (sem AU),
a regra também não avisa para `.trackfw`. ADR D2 encerra a condição de cobertura em "objeto" ou
"herdável para arquivo" — CI|IO está fora por design, mas o resíduo não é nomeado.

**NULL DACL:** emite aviso explícito ("everyone full control"). FECHA.

**DACL vazia (zero ACEs):** loop não itera, `foreignSIDs` vazio, retorna `""`. Semântica correta
(deny-all). FECHA.

**GENERIC_WRITE (0x40000000) e GENERIC_ALL (0x10000000):** estão em `d2WriteMask`. FECHA.

**Rename via `FILE_DELETE_CHILD` no pai da raiz:** coberto indiretamente — nos cenários medidos
(C:\, D:\a\) o pai concede `Modify` que inclui `DELETE` à raiz; esse direito é herdado para a raiz
e detectado. O caso estrito onde apenas `FILE_DELETE_CHILD` no pai (sem ACE herdada para a raiz)
é um sub-caso de R-1 (CI|IO na raiz sem propagação de arquivo).

---

### Item 2 — Falso positivo / ruído

**Projeto em `%USERPROFILE%`:** P17 mediu `foreign_write=(none)` em `home-userprofile`. Regra
não dispara. FECHA.

**Runner D:\a\...:** P17 mediu `D:\ foreign_write_inheritable=BUILTIN\Users`. BUILTIN\Users
(S-1-5-32-545) não está no conjunto confiado; herda para subdiretórios sem `INHERIT_ONLY_ACE`.
`trackfw validate` num projeto checked-out em `D:\a\trackfw\trackfw` **emite aviso**. É verdadeiro
positivo técnico (a escrita alheia existe de fato), mas pode surpreender em pipelines CI normais.
ADR D2 não nomeia este caso explicitamente. **R-5 (observação):** nomear no ADR.

**OneDrive:** não medido. Não confirmado.

**Dono `Administrators` rodando elevado:** `Administrators (S-1-5-32-544)` está no conjunto
confiado → sem aviso. FECHA.

---

### Item 3 — D3: todo erro vira aviso?

**`ownerErr != nil`:** `checkDirForeignWrite` retorna aviso imediato. Se `.trackfw` existir, dois
avisos são emitidos com a mesma mensagem de erro (um para raiz, um para `.trackfw`). Redundante
mas tecnicamente D3-correto.

**`windows.GetAce` com `break`** — **R-2 (D3 violação):**
```go
if err := windows.GetAce(dacl, i, &ace); err != nil {
    break
}
```
Falha em ACE `i` abandona silenciosamente as ACEs restantes. Nenhum aviso D3 é emitido. Um ACL
malformado ou truncado silencia ACEs posteriores. Violação de D3 ("qualquer erro vira aviso").

**`os.Lstat(trackfwDir)` engolindo access-denied** — **R-3 (D3 violação):**
```go
if _, statErr := os.Lstat(trackfwDir); statErr == nil {
    ...
}
```
Qualquer erro (inclusive `access-denied`, `EACCES`) é tratado como "`.trackfw` não existe" →
verificação silenciosa ignorada. Um atacante pode criar `.trackfw` com DACL deny-all para o
processo-vítima: `Lstat` falha, a regra silencia o check de `.trackfw` sem aviso. Violação
concreta de D3 para `.trackfw`.

---

### Item 4 — Os 5 testes mordem?

**(a) `TestProjectForeignWrite_ForeignWriteACE`:** affirms "rule fires when non-trusted SID has
a write ACE on the root". PASS medido (run P18, SID S-1-5-11 nomeado no aviso). Morde se
`d2WriteMask` excluir todos os bits em `0x1301bf` ou se AU for adicionado ao conjunto confiado.
**Imprecisão de reconciliação (R-4):** o comentário diz "fails if d2WriteMask excluded
FILE_WRITE_DATA (0x2)"; mas `0x1301bf` também tem `FILE_APPEND_DATA (0x4)` e `DELETE (0x10000)` —
remover apenas 0x2 ainda faz o teste passar (0x4 ainda bate a mask). A frase de reconciliação
subestima a falsificação real.

**(b) `TestProjectForeignWrite_CleanDACL`:** affirms "no warning when only trusted SIDs hold write
ACEs". PASS medido. Mordera se o check de trusted SID for removido. FECHA.

**(c) `TestProjectForeignWrite_TrackfwDirForeignACE`:** affirms "the .trackfw check fires
independently of the root". PASS medido. Morde se apenas a raiz for verificada. FECHA.

**(d) `TestProjectForeignWrite_ReadOnlyACENoWarning`:** affirms "only D2 write rights trigger the
rule; read/execute rights do not". PASS medido. Morde se `d2WriteMask` for alargado para incluir
`FILE_READ_DATA (0x1)` ou `READ_CONTROL (0x20000)`. FECHA.

**(e) `TestProjectForeignWrite_DACLReadError`:** affirms "D3: errors produce a warning, not nil".
PASS medido. Morde se erros de `GetNamedSecurityInfo` forem silenciados. FECHA.

Testes não cobertos pelos 5: remoção individual de `WRITE_DAC`, `WRITE_OWNER`,
`FILE_DELETE_CHILD`, `GENERIC_WRITE`, `GENERIC_ALL` da mask passa em todos os 5 testes — nenhum
fixture usa exclusivamente esses bits. Nenhum teste cobre DACL com NULL-ACE ou ACE CI|IO-only
(vetor R-1). Não é bloqueante (regra warnOnly), mas é gap de cobertura de testes.

---

### Item 5 — `identity_windows_test.go`

O teste afirma `perm != 0o600` (D1 ADR-2026-10-16: `os.Chmod(0o600)` é inerte no NTFS).
PASS medido: run P18 reportou `perm=0666`.

**Vacuidade estrutural:** `$(go env GOROOT)/src/os/types_windows.go` constrói `Mode()` apenas de
`FILE_ATTRIBUTE_READONLY` → retorna 0o444 ou 0o666 SEMPRE, jamais 0o600 independentemente de ACL.
A condição `perm == 0o600` é estruturalmente inalcançável. O teste passa vacuamente — nunca falha
mesmo que Go adicione `Chmod` com ACL sem mudar `Mode()`.

A afirmação correta seria `perm == 0o666` (valor medido). D1 é provado por P17/icacls, não por
este teste. Se Go um dia adicionar ACL-aware `Mode()` E `Chmod()` simultaneamente, o teste falharia
conforme pretendido — mas a falha seria por `Mode()` mudado, não pelo `Chmod`. A reconciliação
declarada no comentário ("tests that Go's Chmod doesn't write ACL-based 0o600") é indemonstrável
pelo caminho escolhido. **R-4b (qualidade de teste):** não bloqueante.

---

### Item 6 — A mensagem diz a verdade e dá o remédio certo?

**Mensagem de aviso:** `"grants write access to <SID> — move the project inside your user
profile or remove inheritance from <root>"` — verdadeira: nomeia o caminho e o SID. A ressalva:

**R-5 (não bloqueante):** "remove inheritance" é o remédio errado quando o aviso disparou por
**ACE explícita** (não herdada). O teste (a) usa `PROTECTED_DACL_SECURITY_INFORMATION` — herança
já removida — com ACE explícita para AU. O remédio correto nesses casos é
`icacls <dir> /remove:g <SID>`, não remover herança. A mensagem é imprecisa para fixtures de
teste e para DACLs com ACEs explícitas.

**Mensagens D3** (`"cannot read DACL"`, `"cannot determine current user SID"`): carregam o mesmo
sufixo "move the project... or remove inheritance" que não tem relação semântica com a causa do
erro. Semanticamente incorreto, não bloqueante.

---

### Item 7 — Resíduo do fsperm ou go.mod direto sem uso?

`internal/fsperm/` **não existe.** `grep -rn fsperm internal/ .github/ docs/cli-parity.md go.sum`
retorna zero resultados. FECHA.

`go.mod`: `golang.org/x/sys v0.35.0` foi promovido de `// indirect` para dependência direta.
Correto: `validator_project_foreign_write_windows.go` importa `golang.org/x/sys/windows`
diretamente. FECHA.

`internal/pathguard/containment_live_test.go` e `internal/integrations/manager.go`: zero diff
contra `origin/main`. FECHA.

`.github/windows-known-failures.json`: `TestSave_WritesAtomicallyWithPermissions` removido da
lista de falhas ativas (entrada movida para seção `removal_note: corrected`). Consistente com P18
onde o teste passou. FECHA.

---

## Ressalvas (R-n)

| # | Localização | Tipo | Ação recomendada |
|---|---|---|---|
| R-1 | ADR D2 + código | FN: `(CI)(IO)` ACE silenciada | Nomear no ADR como resíduo declarado; considerar ML de follow-up para checar `CONTAINER_INHERIT` sem `INHERIT_ONLY` separadamente |
| R-2 | `validator_project_foreign_write_windows.go:166` | D3: `GetAce break` sem aviso | Substituir `break` por `continue` com warning D3 ao `msgs` |
| R-3 | `validator_project_foreign_write_windows.go:110` | D3: `Lstat` engole access-denied | Emitir aviso D3 quando `statErr != nil && !os.IsNotExist(statErr)` |
| R-4 | `validator_project_foreign_write_windows_test.go` + `identity_windows_test.go` | Qualidade de reconciliação | Corrigir frase de (a); alterar `perm != 0o600` para `perm == 0o666` em identity |
| R-5 | Mensagem de aviso + ADR | Remédio impreciso + FP no CI runner | ADR: nomear runner `D:\a\...` como FP declarado; mensagem: distinguir ACE explícita de herança |

R-2 e R-3 são as violações mais concretas de D3 declarado. Nenhuma bloqueia porque a regra é
`applyRuleWarnOnly` — a consequência máxima é falso negativo (usuário não avisado), nunca
bloqueio indevido.

---

## Frases de reconciliação (5 testes)

1. **(a)** "O teste afirma que a regra emite aviso nomeando o SID quando um SID não confiado tem
   ACE write na raiz; reprovaria se `d2WriteMask` excluísse todos os bits write presentes em
   `0x1301bf` ou se `S-1-5-11` fosse adicionado ao conjunto confiado."

2. **(b)** "O teste afirma que nenhum aviso é emitido quando apenas SIDs confiados (owner,
   SYSTEM, Administrators) têm ACEs; reprovaria se o check `isTrustedSID` fosse removido."

3. **(c)** "O teste afirma que a verificação de `.trackfw\` é independente da raiz; reprovaria se
   `checkProjectForeignWrite` inspecionasse apenas a raiz e não o subdiretório `.trackfw`."

4. **(d)** "O teste afirma que ACEs com apenas direitos de leitura/execução (`0x1200a9`) não
   disparam aviso; reprovaria se `d2WriteMask` fosse alargado para incluir `FILE_READ_DATA
   (0x1)` ou `READ_CONTROL (0x20000)`."

5. **(e)** "O teste afirma que D3 é satisfeito: erros de `GetNamedSecurityInfo` produzem aviso,
   nunca nil; reprovaria se `checkDirForeignWrite` silenciasse erros da chamada de API."
