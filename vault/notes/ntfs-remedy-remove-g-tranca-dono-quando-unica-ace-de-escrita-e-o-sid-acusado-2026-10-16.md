# NTFS: `icacls /remove:g` tranca o dono quando a única ACE de escrita é o SID acusado

**Data:** 2026-10-16 | **REQ:** REQ-2026-09-03 | **ML:** W2d | **Issue:** #569

## Causa raiz

No Windows, o **proprietário implícito** de um diretório detém apenas `READ_CONTROL` e `WRITE_DAC`
— não escrita de dados. Toda escrita de dados vem de ACEs explícitas ou herdadas.

Em instalações fora do perfil do usuário (ex.: `C:\dev\ferramentas\`), o padrão NTFS herda
`Authenticated Users:(OI)(CI)(IO)(M)` da raiz `C:\`. Para um usuário de conta **padrão** (sem
`Administradores` no token), essa é **a única ACE de escrita**.

O remédio impresso por `project_foreign_write_windows` antes da D2.2 era:
```
icacls "<dir>" /inheritance:d && icacls "<dir>" /remove:g *S-1-5-11
```

Ao executar essa sequência, toda escrita ao diretório é bloqueada para o usuário: `git`, `build`,
`trackfw` — tudo falha com "Acesso negado." O usuário fica preso pelo próprio remédio.

Confirmado por lourivalgarciajunior (#569) em Windows 11 Home, conta padrão sem `Administradores`
no token. Verificação por leitura da ACL + token (sem executar o remédio).

## Correcão (D2.2)

Incluir `icacls "<dir>" /grant *<ownerSID>:(OI)(CI)(F)` **antes** do `/remove:g` nos dois casos:

- **ACE explícita:** `/grant *<owner>:(OI)(CI)(F) && /remove:g *<SID>`
- **ACE herdada:** `/inheritance:d && /grant *<owner>:(OI)(CI)(F) && /remove:g *<SID>`

O SID do dono é computado por `currentUserSID()` — já disponível no código. Idempotente quando o
dono já tem ACE própria.

## Prova de efeito (CI Windows, run 38980416363)

No runner do CI (conta `RUNNER~1`, SID `S-1-5-21-4239583824-265399229-1323058055-500`):

**Braço de efeito (novo remédio com `/grant`):**
- `icacls ... /grant *S-1-5-21-...-500:(OI)(CI)(F)` → `processed file: ...`
- `icacls ... /remove:g *S-1-5-11` → `processed file: ...`
- `os.CreateTemp` depois → **sucesso** — dono conserva escrita

**Braço de controle (remédio antigo, sem `/grant`):**
- `icacls ... /remove:g *S-1-5-11` → `processed file: ...`
- `os.CreateTemp` depois → **"Access is denied."** — `/grant` é load-bearing

O runner do CI é conta padrão (não admin), o que reproduz o cenário real do #569.

## Arquivos afetados

- `internal/validator/validator_project_foreign_write_windows.go` — linhas 24–28, 260–268, 274–284
- `internal/validator/validator_project_foreign_write_windows_test.go` — TestProjectForeignWrite_*
- `docs/cli-parity.md` — seção `project_foreign_write_windows`, linhas ~8836–8862

## Armadilha ao escrever testes de efeito no Windows

**Cleanup de `t.TempDir` falha** se o DACL do diretório foi modificado para remover o acesso de
escrita. Registrar `t.Cleanup` (LIFO) que restaura full-control antes de `t.TempDir` tentar
`RemoveAll`:

```go
dir := t.TempDir()
t.Cleanup(func() { restoreFullControl(t, dir, ownerSIDStr) })
applyDACLFromSDDL(t, dir, sddl) // modifica após registrar cleanup
```

**Tokenizador de comando para `exec.Command`:** `exec.Command("cmd", "/c", fullCmd)` falha porque Go
escapa `"` como `\"` e `cmd.exe` não entende isso. Solução: dividir em ` && `, tokenizar cada
segmento (strip das aspas duplas, split em whitespace fora de aspas), e chamar
`exec.Command(args[0], args[1:]...)` por segmento.
