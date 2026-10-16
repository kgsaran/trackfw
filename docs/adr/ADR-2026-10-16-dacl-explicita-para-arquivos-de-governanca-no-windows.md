---
status: Proposed
date: 2026-10-16
author: "Hades-tf (parecer) / Zeus (decisão pendente)"
---

# ADR: DACL explícita para arquivos de governança no Windows

> Date: 2026-10-16 | Status: Proposed

**REQ:** `docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md`

## Context

No Windows, `os.Chmod(0o600)` e `os.WriteFile(path, data, 0o600)` apenas alternam o bit `READONLY`
do atributo de arquivo — não modificam a ACL (DACL) NTFS. A ACL real de um arquivo criado por
`os.CreateTemp` é herdada do diretório pai no momento da criação.

Medição (run 38902064559, Pergunta 17, x64):

- `%USERPROFILE%\.trackfw\`: apenas dono/SYSTEM/Administrators — OK, nenhuma ação necessária.
- Projetos sob `C:\` diretamente: `NT AUTHORITY\Authenticated Users:(I)(M)` herdado de `C:\`
  (que tem `Authenticated Users:(OI)(CI)(IO)(M)` por padrão). Qualquer usuário logado
  localmente pode sobrescrever arquivos com esta herança.

Os cinco sítios que pedem `0o600` (`identity.json`, `integrations-manifest.json`, artefato de
render, `thirdparty-quarantine/<hash>.json`, `thirdparty-provenance.json`) ficam expostos à escrita
por co-inquilinos quando o projeto está diretamente sob `C:\`. O vetor de maior severidade é a
proveniência: um co-inquilino pode forjar `thirdparty-provenance.json` para fazer `trackfw validate`
reportar aprovação humana de uma skill maliciosa.

## Decision

Criar o pacote `internal/fsperm` com implementação build-tagged para Windows e no-op para os
demais sistemas operacionais.

### D1 — Criação do arquivo temporário já com DACL restritiva

`CreateRestrictedFile(dir, pattern string) (*os.File, string, error)` substitui `os.CreateTemp`
nos dois sítios de escrita atômica (`GuardedWrite` em pathguard.go, `atomicWrite` em manager.go).
Internamente usa `windows.CreateFile` com:

- `lpSecurityAttributes`: DACL restritiva pré-construída (Owner+SYSTEM+Administrators,
  `SE_DACL_PROTECTED`)
- `dwShareMode`: `FILE_SHARE_READ` — sem escrita concorrente enquanto o handle está aberto
- `dwCreationDisposition`: `CREATE_NEW` — loop de nome aleatório em `ERROR_FILE_EXISTS`

**Motivação:** Windows avalia acesso no `CreateFile`. Um handle aberto retém os direitos concedidos
na abertura mesmo após mudança de DACL. `os.CreateTemp` usa `FILE_SHARE_READ | FILE_SHARE_WRITE`
(`syscall/syscall_windows.go:396`). Chamar `SetRestrictiveACL` depois de `os.CreateTemp` não fecha
a janela. `CreateRestrictedFile` elimina a janela por construção.

### D2 — Composição da DACL

ACEs: Owner (SID obtido via `OpenCurrentProcessToken`+`GetTokenInformation(TOKEN_USER)`),
`NT AUTHORITY\SYSTEM`, `BUILTIN\Administrators` — todos com `GENERIC_ALL`. DACL protegida
(`SE_DACL_PROTECTED`). Nenhuma ACE para Authenticated Users, Users ou Everyone.

Owner SID em runtime obrigatório — não hardcodar — porque o processo pode rodar como qualquer
conta (desenvolvedor, domínio, serviço).

### D3 — DACL em diretório: apenas quando o trackfw criou o diretório nesta chamada

`SetRestrictiveDirACL(path string) error` é chamado após `MkdirAll` SOMENTE se o diretório folha
não existia antes desta chamada (verificado por `os.Stat` antes do `MkdirAll`). ACEs do diretório
incluem `OI|CI` para propagação a novos filhos.

**Motivação:** `render.go` escreve em diretórios de agente (`.claude/agents/`, etc.) que podem ter
sido criados por ferramentas externas. Aplicar `SE_DACL_PROTECTED` em diretório preexistente
alteraria configurações de terceiros — fora da propriedade do trackfw.

### D4 — Detecção de volume sem ACL: fail-soft delimitado

`IsACLSupported(path string) (bool, error)` chama `GetVolumeInformation` e testa
`FILE_PERSISTENT_ACLS` (0x00000008). FAT/exFAT/SMB sem ACL retornam false.

Quando o volume não suporta ACL: `CreateRestrictedFile` retorna `ErrNoACLSupport` (tipo sentinela);
o caller faz fallthrough para `os.CreateTemp` convencional e propaga `ErrNoACLSupport` para o
handler do comando. **Pathguard nunca emite aviso em stderr diretamente** — o caller decide
(stderr interativo ou campo `"warning"` em JSON).

Para `SetNamedSecurityInfo` (usado em `SetRestrictiveDirACL`): fail-soft somente em
`ERROR_NOT_SUPPORTED` (50), `ERROR_INVALID_FUNCTION` (1), `ERROR_NO_SECURITY_ON_OBJECT` (0x546).
Todo o resto: fail-closed.

### D5 — Arquivos legados: resíduo declarado

Arquivos escritos antes da correção mantêm o DACL herdado até serem regravados pelo trackfw.
A reescrita re-protege. Arquivos que nunca são regravados retêm o DACL permissivo — resíduo
declarado, não defeito silenciado. Uma regra de `validate` que leia DACLs está fora do escopo
deste ML; pode ser adicionada em REQ futura.

### D6 — Dependência promovida

`golang.org/x/sys` promovido de indireta a direta em `go.mod`. A dependência já existia no módulo
(indireta); a promoção não introduz nova transitividade.

## Consequences

**Positivas:**
- Arquivos de governança criados após o fix são protegidos contra escrita por co-inquilinos em
  qualquer volume NTFS, independente da ACL do diretório pai.
- A janela de corrida entre criação do temp e aplicação de DACL é eliminada por construção.
- Falha em volume sem ACL é explícita (tipo sentinela) em vez de silenciosa.

**Negativas e resíduos:**
- Arquivos existentes (pré-fix) não são retroativamente protegidos.
- Volumes FAT/exFAT/SMB sem ACL permanecem desprotegidos — resíduo declarado.
- O pacote `internal/fsperm` introduz código Windows-only; futuros sítios de escrita devem usar
  `CreateRestrictedFile` em vez de `os.CreateTemp` para herdar a proteção.
- `golang.org/x/sys` passa a ser dependência direta.

## Alternatives considered

**`os.CreateTemp` + `SetRestrictiveACL` posterior:** descartado. Windows avalia acesso no open;
handle aberto retém direitos concedidos na abertura independente de mudanças posteriores na DACL.
`FILE_SHARE_WRITE` no `os.CreateTemp` permite que o adversário abra o file durante a janela de
ACL herdada e retenha o handle.

**Declarar limite sem correção:** descartado para proveniência e quarentena. O vetor de bypass do
gate `thirdparty_artifact_has_provenance` é real e exploitável sem privilégio de admin.

**Regra de `validate` que leia DACL:** complementar, não substituta. Não corrige o problema na
escrita; trata-se de detecção pós-fato. Reservada para REQ futura.
