# Wave 0 — Modelo de ameaça: DACL explícita para 0o600/0o700 no Windows (ML-W2b)

> Produzido por: hades-tf | Data: 2026-10-16 (revisado 2026-10-16 — auditoria do arquiteto)
> REQ: REQ-2026-09-03 | Fato base: run 38902064559 (Pergunta 17, x64)
> Branch: `fix/fechar-os-grupos-de-falha-de-windows-ml-w2-permissao`

---

## 1. Enumeração de superfícies — completude

O roadmap e o documento de medição listam cinco sítios de escrita com `0o600`:

| sítio | arquivo | via |
|---|---|---|
| `internal/identity/identity.go:93` | `identity.json` | `pathguard.GuardedWrite` |
| `internal/integrations/manifest.go:92` | `integrations-manifest.json` | `atomicWrite` (manager.go) |
| `internal/integrations/render.go:730` | artefato de agente renderizado | `atomicWrite` (manager.go) |
| `internal/thirdparty/quarantine.go:114` | `thirdparty-quarantine/<hash>.json` | `pathguard.GuardedWrite` |
| `internal/thirdparty/provenance.go:155` | `thirdparty-provenance.json` | `pathguard.GuardedWrite` |

E dois sítios de `MkdirAll` com `0o700`:

| sítio | via |
|---|---|
| `internal/pathguard/pathguard.go:150` | `GuardedWrite` — cria o diretório pai antes do `CreateTemp` |
| `internal/integrations/manager.go:918` | `atomicWrite` — cria o diretório pai antes do `CreateTemp` |

**Enumeração fechada?** Sim, dentro do escopo declarado. O ML-W2a mediu que todos os sítios de produção que pedem `0o600` passam por `GuardedWrite` ou por `atomicWrite` em manager.go (a docstring de `GuardedWrite` registra a consolidação dos três `atomicWrite` anteriores). Não há terceiro caminho de escrita de arquivo com modo restritivo documentado no produto.

**Superfície ausente não listada:** os arquivos de configuração gerados por `trackfw init` (scripts de hook, `lefthook.yml`, etc.) não usam `0o600` — eles têm permissão executável por desenho e não guardam estado de segurança. Não são parte desta superfície.

**Limitação da medição:** apenas x64 foi medido (run 38902064559). ARM64 não foi medido; pelo mecanismo de herança NTFS (configuração de ACL da raiz `C:\`, não da arquitetura), espera-se o mesmo comportamento, mas é suposição não verificada.

---

## 2. Modelo de ameaça

**Adversário:** co-inquilino local — usuário autenticado na mesma máquina Windows, sem privilégios de administrador, membro de `NT AUTHORITY\Authenticated Users`. Não precisa ter acesso ao projeto (só à máquina). Não é necessário controle do repositório, do código-fonte ou de qualquer outra conta.

**Pré-condição confirmada por medição (run 38902064559) — apenas o braço `C:\`:**
Em projetos criados diretamente sob `C:\` (braço `root-c`), a ACL herdada concede `Modify` a Authenticated Users: `NT AUTHORITY\Authenticated Users:(I)(M)`. O braço `D:\a\_temp` (runner-temp) **não** tem escrita alheia em arquivos — o `icacls` bruto do arquivo mostra `BUILTIN\Users:(I)(RX)` (leitura apenas); o ACE `(CI)(IO)(WD)` de `D:\` propaga-se só a contêineres (subdiretórios), não a arquivos (ausência de `OI`). O braço `%USERPROFILE%\.trackfw\` já é restritivo — nenhuma ação necessária para esses dois braços. A decisão de DACL explícita é sustentada pelo braço `C:\` sozinho.

### Ameaça por sítio

**T-1 — `provenance.go:155` (`thirdparty-provenance.json`)** — Severidade: ALTA

Este é o arquivo que `trackfw validate` lê como autoridade sobre se uma skill de terceiro recebeu aprovação humana (`thirdparty_artifact_has_provenance`). Com `Authenticated Users:(I)(M)` herdado:

1. O adversário lê o esquema da proveniência legítima existente (o arquivo é readable por Authenticated Users também).
2. Forja um registro com `checksum` de uma skill maliciosa e `approved_by` preenchido com qualquer valor.
3. `trackfw validate` passa: a regra lê o JSON, encontra o checksum, e conclui que foi aprovado.
4. `trackfw agents third-party install --checksum <forjado>` instala a skill sem disparar o gate de proveniência.

Resultado: bypass completo do fluxo de revisão humana de terceiros. A skill maliciosa chega ao ambiente como se tivesse sido aprovada.

**Relação com o restante do projeto exposto:** se o diretório raiz do projeto é amplamente gravável por Authenticated Users, um adversário também poderia modificar os scripts de hook, o `trackfw.yaml`, etc. Mas esses arquivos não são o que `trackfw validate` lê como prova de aprovação — `thirdparty-provenance.json` é. Proteger só a proveniência fecha o vetor de bypass do gate sem exigir que o diretório inteiro seja restrito.

**T-2 — `quarantine.go:114` (`thirdparty-quarantine/<hash>.json`)** — Severidade: ALTA

O registro de quarentena é o que liga o `fetch` ao `install`. Com escrita alheia:

1. Entre `trackfw agents third-party fetch <url>` e o passo de revisão humana (janela de tempo), o adversário substitui o conteúdo em quarentena por uma skill maliciosa com o mesmo `checksum` de uma legítima.
2. O operador aprova o que acreditou ter baixado; o `install` instala o substituto.

**T-3 — `manifest.go:92` (`integrations-manifest.json`)** — Severidade: MÉDIA-ALTA

O manifesto lista os agentes instalados e suas propriedades. Com escrita alheia, o adversário pode modificar o modelo de um agente existente ou injetar novo agente no manifesto; `trackfw validate` pode não detectar a adulteração (sem verificação de integridade criptográfica do manifesto).

**T-4 — `render.go:730` (artefato de agente renderizado)** — Severidade: MÉDIA

Os arquivos renderizados são as instruções dos agentes (`.claude/agents/`, etc.). Com escrita alheia, o adversário pode modificar as instruções após o `render` e antes do próximo uso do agente.

**T-5 — `identity.go:93` (`identity.json`)** — Severidade: BAIXA-MÉDIA

Adulteração do `identity.json` pode causar confusão de identidade. Não foi identificado vetor de escalada direta.

**T-6 — `MkdirAll` (pathguard.go:150, manager.go:918)** — Severidade: INDIRETA

Diretório `.trackfw/` gravável por adversário permite inserção de arquivos com nomes controlados. A proteção do diretório com DACL explícita fecha esse vetor.

---

## 3. Targets de falsificação nas duas direções

| sítio | falso negativo (sem fix) | falso positivo (com fix) |
|---|---|---|
| provenance (T-1) | adversário forja proveniência; validate diz "aprovado" | file born with restrictive DACL but owner SID errado bloqueia o próprio processo |
| quarantine (T-2) | adversário substitui durante janela fetch→install | DACL em volume FAT fail-closed impede o fetch inteiramente |
| manifest, render, identity (T-3/4/5) | arquivo adulterado não detectado | idem owner SID |
| diretório (T-6) | arquivo criado no `.trackfw/` por adversário | herança OI/CI restritiva bloqueia subcomandos que escrevem na pasta |

**Falsificação prioritária (falso negativo):** arquivo escrito via `GuardedWrite`/`atomicWrite` sob diretório pai com `Authenticated Users:(OI)(CI)(M)` manufaturado no teste (sem depender de `C:\`) deve resultar, após o fix, em DACL sem ACE de escrita para Authenticated Users — verificado por `GetNamedSecurityInfo`.

**Baseline do defeito (falsificação inversa):** sem o fix, `os.WriteFile` no mesmo diretório deve ter o ACE de Authenticated Users presente. Isso valida que o ambiente de teste reproduz o defeito antes de afirmar que o fix funciona.

---

## 4. Desenho recomendado

### 4.1 Ponto de aplicação

Novo pacote `internal/fsperm` com dois arquivos:

- `internal/fsperm/fsperm_windows.go` (build tag implícito pelo sufixo `_windows.go`) — implementação real
- `internal/fsperm/fsperm_other.go` (`//go:build !windows`) — no-ops que retornam `nil`

Funções exportadas:
- `CreateRestrictedFile(dir, pattern string) (*os.File, string, error)` — substitui `os.CreateTemp` nos dois sítios de escrita atômica; cria o arquivo já com DACL restritiva e share mode exclusivo
- `SetRestrictiveDirACL(path string) error` — aplica DACL restritiva a um diretório (chamado após `MkdirAll`)
- `IsACLSupported(path string) (bool, error)` — detecta suporte a ACL por `GetVolumeInformation`; usado por callers para decidir se emitem aviso

### 4.2 Criação do arquivo temporário sem janela de corrida

**Problema com `os.CreateTemp` + `SetRestrictiveACL` posterior:**
A verificação de acesso no Windows ocorre **no momento de abrir o handle** (MSDN "Access Checks", "CreateFile" — *"access to the object is determined by checking the security descriptor of the object"*). Um handle aberto retém os direitos concedidos na abertura mesmo que a DACL seja alterada depois. Além disso, `os.CreateTemp` abre o arquivo com `FILE_SHARE_READ | FILE_SHARE_WRITE` (confirmado em `syscall/syscall_windows.go:396`), o que permite que outro processo abra o mesmo arquivo para escrita enquanto nossa janela de herança permissiva ainda está ativa. Chamar `SetRestrictiveACL` depois não fecha esse handle já aberto.

**Solução — `CreateRestrictedFile`:**
Criar o arquivo temporário diretamente via `windows.CreateFile` com:
- `lpSecurityAttributes`: aponta para `SECURITY_ATTRIBUTES` cujo `SecurityDescriptor` já carrega o DACL restritivo (Owner+SYSTEM+Administrators, protegido)
- `dwShareMode`: `FILE_SHARE_READ` (sem WRITE) — impede que qualquer outro processo abra o arquivo para escrita enquanto o nosso handle está aberto
- `dwCreationDisposition`: `CREATE_NEW` — falha se o arquivo existir (necessário para o loop de nome aleatório)
- Nome: gerado por laço com sufixo aleatório (hex de 10 chars, igual ao `os.CreateTemp`), retry em `ERROR_FILE_EXISTS`

Após a criação: `os.NewFile(uintptr(handle), temporaryName)` para obter o `*os.File` compatível com o restante do código.

**Fecha a corrida por completo?** Sim:
1. O arquivo nasce com o DACL restritivo → não há janela de ACL permissiva
2. O share mode restringe a escrita concorrente → nenhum handle de adversário pode ser aberto para escrita enquanto o arquivo existe com o nome temporário
3. Após `os.Rename`, o arquivo final tem o mesmo DACL (mesma identidade de objeto, mesmo volume)

### 4.3 Composição do DACL

ACEs no DACL explícito:

| Trustee | Direitos | Flags para arquivo | Flags para diretório |
|---|---|---|---|
| Owner (SID do token do processo) | `GENERIC_ALL` | — | `OI\|CI` |
| `NT AUTHORITY\SYSTEM` | `GENERIC_ALL` | — | `OI\|CI` |
| `BUILTIN\Administrators` | `GENERIC_ALL` | — | `OI\|CI` |

DACL protegida (`SE_DACL_PROTECTED`, passado em `SetNamedSecurityInfo` como `windows.PROTECTED_DACL_SECURITY_INFORMATION`) — desativa herança de ACEs do pai. Esta flag é obrigatória: sem ela, propagação posterior (`icacls /reset`) re-adiciona a ACE permissiva.

**Não incluir:** `NT AUTHORITY\Authenticated Users`, `BUILTIN\Users`, `Everyone`.

**Obtenção do Owner SID:** via `windows.OpenCurrentProcessToken()` + `windows.GetTokenInformation(TOKEN_USER)`. Não hardcodar nenhum SID. Se a obtenção falhar (cenário extremamente improvável — o processo tem um token), retornar erro (fail-closed), NÃO usar um SID estático como fallback.

### 4.4 Sequência correta dentro de `atomicWrite` / `GuardedWrite`

```
fsperm.CreateRestrictedFile(directory, ".trackfw-tmp-*")
  // ↳ windows.CreateFile(randomName, GENERIC_WRITE, FILE_SHARE_READ,
  //                       &saRestrictive, CREATE_NEW, NORMAL, 0)
  // ↳ os.NewFile(handle, name)
temporary.Write(data)
temporary.Sync()
temporary.Close()
os.Rename(temporaryName, filename)
  // rename mesmo volume → preserva DACL (documentado: MSDN MoveFileExW,
  // "Security Considerations: Windows File Management")
```

O `os.MkdirAll` existente permanece; após a chamada, invocar `fsperm.SetRestrictiveDirACL(directory)` para proteger o diretório folha com ACEs `OI|CI`.

### 4.5 Limite de aplicação do DACL em diretórios

**Regra:** DACL explícita em diretório SOMENTE quando o trackfw criou esse diretório NESTA chamada de `MkdirAll`. Não aplicar em diretórios preexistentes.

**Motivação:** `render.go` escreve em diretórios de agente (`.claude/agents/`, `.cursor/`, etc.) que podem ter sido criados por ferramentas externas (Claude Code, Windsurf). Aplicar `SE_DACL_PROTECTED` nesses diretórios alteraria a configuração de acesso de recursos que não pertencem ao trackfw — desfaria permissões que outros usuários ou ferramentas esperam encontrar.

**Implementação:** antes de chamar `MkdirAll`, verificar se o diretório folha existe (`os.Stat`). Se já existia: não aplicar DACL ao diretório (a DACL do arquivo ainda é aplicada via `CreateRestrictedFile`). Se foi criado nesta chamada: aplicar `SetRestrictiveDirACL`.

**Todos os cinco sítios de arquivo:** DACL aplicada via `CreateRestrictedFile` em TODOS os sítios, independente do diretório. A proteção do arquivo é incondicional; a do diretório é condicional à criação pelo trackfw.

### 4.6 Tratamento de volume sem suporte a ACL

**Detecção proativa por `GetVolumeInformation`:**
Antes de criar o arquivo, verificar se `FILE_PERSISTENT_ACLS` (0x00000008) está presente nos `FileSystemFlags` retornados por `GetVolumeInformation` para a raiz do volume. FAT/exFAT não setam este flag.

```
IsACLSupported(path) → (false, nil) se FILE_PERSISTENT_ACLS ausente
```

Se o volume não suporta ACL:
- `CreateRestrictedFile` retorna erro tipado `ErrNoACLSupport{Path: path}` em vez de nil
- O **caller** (`GuardedWrite`, `atomicWrite`) faz o fall-through para `os.CreateTemp` convencional e RETORNA o `ErrNoACLSupport` para cima — não emite aviso aqui
- O handler do **comando** (identity.go caller, provenance.go caller, etc.) recebe `ErrNoACLSupport` e decide como comunicar ao usuário (stderr interativo vs. campo de aviso em JSON)

**Por que o aviso não pode vir de dentro de `pathguard`:** `pathguard.GuardedWrite` é chamado de contextos que podem estar no modo `--json`; emitir texto para stderr de dentro do pathguard polui a saída. O `ErrNoACLSupport` como tipo explícito permite ao caller emitir `{"warning": "filesystem does not support ACLs; file protection not enforced on this volume"}` no lugar apropriado.

**Erros que implicam fail-closed (não são FAT/SMB):**
Qualquer outro erro de `CreateFile` com `SECURITY_ATTRIBUTES` — em particular `ERROR_ACCESS_DENIED`, `ERROR_INVALID_PARAMETER`, falhas de alocação do security descriptor — retorna esse erro diretamente para cima (fail-closed). Esses erros indicam programador ou configuração, não limitação de sistema de arquivos.

**Erro específico de `SetNamedSecurityInfo` (para `SetRestrictiveDirACL`):**
- `ERROR_NOT_SUPPORTED` (50) ou `ERROR_INVALID_FUNCTION` (1): volume sem ACL — fail-soft com `ErrNoACLSupport` (mesmo tratamento)
- `ERROR_NO_SECURITY_ON_OBJECT` (0x546): objeto não suporta segurança — idem
- Outros: fail-closed

### 4.7 Arquivos legados (escritos antes do fix)

Arquivos de proveniência e quarentena escritos ANTES da correção mantêm o DACL permissivo herdado até serem regravados. Quando o trackfw reescreve o arquivo (próxima execução de `fetch`, `install --force`, etc.), `CreateRestrictedFile` aplica o DACL restritivo.

**Declaração como resíduo:** arquivos que nunca são regravados (e.g., proveniência de uma skill instalada e nunca atualizada) retêm o DACL permissivo indefinidamente. Este é o resíduo declarado desta implementação. A opção (b) — regra de `validate` que leia o DACL — está fora do escopo deste ML; pode ser adicionada em REQ futura. O commit do ML deve documentar este resíduo explicitamente.

### 4.8 Isolamento fora do Windows

`internal/fsperm/fsperm_other.go` com `//go:build !windows`:

```go
func CreateRestrictedFile(dir, pattern string) (*os.File, string, error) {
    return os.CreateTemp(dir, pattern)  // delegação direta; sem DACL
}
func SetRestrictiveDirACL(_ string) error { return nil }
func IsACLSupported(_ string) (bool, error) { return true, nil }
```

O `os.Chmod(mode)` existente em `GuardedWrite` e `atomicWrite` permanece em ambos os caminhos — em Linux/macOS é o mecanismo correto; em Windows é inerte mas inofensivo.

### 4.9 Exige ADR?

Sim. Critérios que se encaixam: novo pacote de produto (`internal/fsperm`), mudança de comportamento em tempo de execução (o arquivo passa a nascer com DACL restritiva, não herdada), promoção de dependência (`golang.org/x/sys` de indireta a direta), e decisão com trade-off de segurança documentado (fail-soft em FAT vs. fail-closed). A decisão afeta contribuidores futuros que adicionarem novos sítios de escrita.

ADR rascunhado em: `docs/adr/ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md` (status: Proposed).

---

## 5. Testes que mordem no Windows

### 5.1 `TestSave_WritesAtomicallyWithPermissions` (identity)

Situação atual: afirma `Mode().Perm() == 0o600`. No Windows, `os.File.Mode().Perm()` reflete o bit `READONLY` do atributo, não a ACL — o assert falha ou passa pelo motivo errado.

**Destino:** dois arquivos separados:

- `identity_windows_test.go`: chama `CreateRestrictedFile` diretamente; lê o DACL com `GetNamedSecurityInfo`; afirma ausência de ACE de escrita para Authenticated Users/Users/Everyone; afirma presença de ACE FullControl para o owner.
- `identity_test.go` (sem sufixo ou `//go:build !windows`): mantém `Mode().Perm() == 0o600`.

### 5.2 Como criar a raiz "ruim" no teste

**Não usar `C:\` nem `%TEMP%` do runner.** Em vez disso:

1. `os.MkdirTemp("", "trackfw-dacl-test-*")`
2. Usar `SetNamedSecurityInfo` via `golang.org/x/sys/windows` para adicionar `Authenticated Users:(OI)(CI)(M)` ao diretório — simula a herança de `C:\`
3. Escrever via `GuardedWrite` / `atomicWrite` nesse diretório
4. Ler o DACL do arquivo com `GetNamedSecurityInfo`; afirmar ausência do ACE de Authenticated Users

Isso elimina dependência da configuração do runner e reproduz o defeito em qualquer runner Windows.

### 5.3 Baseline do defeito

Antes de afirmar que o fix funciona, o teste verifica que `os.WriteFile` no mesmo diretório "ruim" produz arquivo COM o ACE de Authenticated Users. Isso prova que o ambiente de teste reproduz o defeito — o teste não é vacuoso.

---

## 6. Riscos numerados (R-n)

**R-1 — A corrida está no `os.CreateTemp`, não no `SetRestrictiveACL` posterior.**
Windows avalia acesso no `CreateFile`; handle aberto retém os direitos concedidos na abertura independentemente de mudanças posteriores na DACL. `os.CreateTemp` usa `FILE_SHARE_READ | FILE_SHARE_WRITE` (`syscall/syscall_windows.go:396`). Chamar `SetRestrictiveACL` depois do `CreateTemp` NÃO fecha a janela — um adversário que abriu o file para escrita durante a janela de ACL herdada mantém o handle válido. **Solução fechada: `CreateRestrictedFile` cria o arquivo com o DACL restritivo no `CreateFile` e com `FILE_SHARE_READ` apenas.** Nenhuma janela de ACL permissiva existe; nenhum handle concurrent de escrita pode ser aberto.

**R-2 — Fail-soft estritamente delimitado à classe "volume sem ACL".**
Fail-soft aplica-se APENAS quando `GetVolumeInformation` não retorna `FILE_PERSISTENT_ACLS` (detecção proativa) ou quando `SetNamedSecurityInfo`/`CreateFile`-com-SA retorna `ERROR_NOT_SUPPORTED` (50), `ERROR_INVALID_FUNCTION` (1) ou `ERROR_NO_SECURITY_ON_OBJECT` (0x546). Qualquer outro erro é fail-closed (retorna o erro para o caller). Fail-soft silencia dentro de `fsperm`; o CALLER recebe `ErrNoACLSupport` e decide se emite aviso em stderr ou como campo JSON `"warning"` — `pathguard` nunca escreve em stderr diretamente por este motivo (poluiria saída `--json`).

**R-3 — Owner SID obtido em runtime, não hardcoded.**
`OpenCurrentProcessToken()` + `GetTokenInformation(TOKEN_USER)`. Se falhar: fail-closed (retorna erro) — NÃO usar SID estático como fallback (um fallback errado cria DACL que bloqueia o próprio processo).

**R-4 — `SE_DACL_PROTECTED` obrigatório.**
Sem este flag, operação posterior de reset de ACL no pai pode re-herdar a ACE permissiva. Passado como `windows.PROTECTED_DACL_SECURITY_INFORMATION` em toda chamada de `SetNamedSecurityInfo`.

**R-5 — `os.Rename` preserva o DACL apenas dentro do mesmo volume.**
Invariante atual garantido por construção: `os.CreateTemp(directory, ...)` onde `directory = filepath.Dir(filename)` — temporário e destino no mesmo diretório, logo no mesmo volume. `CreateRestrictedFile` herda este invariante. Documentar como comentário nos dois sítios. Se quebrado no futuro, a DACL seria perdida numa cópia+delete cross-volume.

**R-6 — DACL de diretório apenas quando o trackfw o criou nesta chamada.**
Aplicar `SE_DACL_PROTECTED` em diretório preexistente (e.g., `.claude/` para `render.go`) altera configuração de outros usuários ou ferramentas. A regra: verificar existência ANTES de `MkdirAll`; aplicar `SetRestrictiveDirACL` somente ao diretório folha criado por esta chamada. Arquivos: DACL via `CreateRestrictedFile` é sempre aplicada, independente do diretório.

**R-7 — `golang.org/x/sys/windows` promovido de indireta a direta em `go.mod`.**
`go get golang.org/x/sys` antes de importar em código de produto. Deve estar no commit do ML.

**R-8 — Testes não usam `t.Skip` nem guard de plataforma que elimine o assert.**
O padrão correto é `_windows_test.go` separado com o assert de DACL, coexistindo com o arquivo `!windows` que afirma o modo POSIX.

**R-9 — Arquivos legados permanecem com DACL permissiva até serem regravados.**
Proveniência de skill instalada e nunca atualizada retém o ACL herdado. Resíduo declarado. Documentar no commit e no ADR. Regra de `validate` para detectar DACL permissiva é trabalho futuro (REQ separada).

**R-10 — `BUILTIN\Administrators` no DACL em contêiner sem Administrators.**
Em Windows Server Core com usuário não-admin puro, a ACE de Administrators é desnecessária mas inofensiva. Inofensivo.

---

## Veredito

**APROVA COM RESSALVAS (R-1 a R-10).**

As ressalvas R-1, R-3, R-4 e R-8 são bloqueantes para o PR. As demais são requisitos de documentação ou residuais declarados.

**Mudança crítica em relação à versão anterior:** `os.CreateTemp` + `SetRestrictiveACL` posterior **não fecha a corrida** — substituído por `CreateRestrictedFile` que cria o arquivo com DACL e share mode restritivos desde o `CreateFile`. Esta é a única abordagem que fecha a janela completamente.

**Exige ADR:** sim — `docs/adr/ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md` (Proposed).
