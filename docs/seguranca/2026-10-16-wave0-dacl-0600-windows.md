# Wave 0 — Modelo de ameaça: DACL explícita para 0o600/0o700 no Windows (ML-W2b)

> Produzido por: hades-tf | Data: 2026-10-16
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

**Adversário:** co-inquilino local — usuário autenticado na mesma máquina Windows, sem privilégios de administrador, membro de `NT AUTHORITY\Authenticated Users` ou `BUILTIN\Users`. Não precisa ter acesso ao projeto (só à máquina). Não é necessário controle do repositório, do código-fonte ou de qualquer outra conta.

**Pré-condição confirmada por medição (run 38902064559):** em projetos criados diretamente sob `C:\` ou sob `D:\a\` (runner-temp), a ACL herdada concede `Modify` ou `WriteData` a esse adversário. Em `%USERPROFILE%\.trackfw\` a herança já é restritiva — nenhuma ação necessária para esse braço.

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

Nota: o ataque exige conhecer o `checksum` esperado (ou substituir por um cujo checksum seja recalculado pelo binário na hora). A dificuldade é modulada pela largura da janela e pela capacidade do adversário de recalcular SHA-256 — não é trivial, mas é realizável.

**T-3 — `manifest.go:92` (`integrations-manifest.json`)** — Severidade: MÉDIA-ALTA

O manifesto lista os agentes instalados e suas propriedades (modelo, escopo, targets). Com escrita alheia:

1. O adversário modifica o modelo de um agente existente (`model: malicious-endpoint`) ou injeta um novo agente no manifesto.
2. `trackfw serve` exibe o manifesto modificado; `trackfw validate` pode não detectar a adulteração (não há verificação de integridade criptográfica do manifesto hoje).

Resultado: desinformação sobre o estado de governança; potencial redirecionamento de modelo.

**T-4 — `render.go:730` (artefato de agente renderizado)** — Severidade: MÉDIA

Os arquivos renderizados são as instruções dos agentes (`.claude/agents/`, etc.). Com escrita alheia, o adversário pode modificar as instruções após o `render` e antes do próximo uso do agente. O impacto depende de como o host de agente consome o arquivo; em Claude Code, o arquivo é lido por sessão.

**T-5 — `identity.go:93` (`identity.json`)** — Severidade: BAIXA-MÉDIA

O `identity.json` contém a identidade do projeto. Adulteração pode causar confusão de identidade em contextos onde o trackfw usa esse arquivo para resolver o projeto. Não foi identificado vetor de escalada direta a partir desta adulteração.

**T-6 — `MkdirAll` (pathguard.go:150, manager.go:918)** — Severidade: INDIRETA

Se o diretório pai (`directory`) for gravável por Authenticated Users, um adversário pode criar arquivos com nomes que conflitem com o `.trackfw-tmp-*` que o trackfw vai criar, ou inserir arquivos no diretório `.trackfw/` antes do primeiro uso. A ameaça direta é limitada (o `RejectSymlinks` / `RejectAndReport` do `GuardedWrite` já bloqueia substituição por symlink). A proteção do diretório com DACL explícita fecha o vetor residual de criação de arquivo no diretório.

---

## 3. Targets de falsificação nas duas direções

Para cada sítio da superfície, o gate correto e a direção de falha:

| sítio | gate que deve pegar | falso negativo (sem fix) | falso positivo (com fix) |
|---|---|---|---|
| provenance (T-1) | `thirdparty_artifact_has_provenance` | adversário forja proveniência; validate diz "aprovado" | fix excessivamente restritivo impede que o próprio processo escreva a proveniência (erro ao criar o arquivo) |
| quarantine (T-2) | `SetNamedSecurityInfo` chamado antes da escrita dos dados | adversário substitui durante janela fetch→install | fix bloqueia escrita em FS sem suporte a ACL (FAT/SMB), tornando o fetch inutilizável |
| manifest (T-3) | ausência de ACE de escrita para Authenticated Users/Users no DACL lido por `GetNamedSecurityInfo` | manifesto adulterado não detectado | DACL com Owner SID errado bloqueia o próprio processo |
| render (T-4) | idem manifest | instruções adulteradas consumidas pelo agente | idem |
| identity (T-5) | idem | identidade forjada confunde resolução de projeto | idem |
| diretório (T-6) | DACL de diretório com OI/CI na ACE do Owner | arquivo criado no `.trackfw/` por adversário | herança OI/CI excessivamente restritiva bloqueia subcomandos que criam arquivos sob o diretório |

**Direção de falso negativo que o teste deve cobrir (prioritária):** um arquivo escrito via `GuardedWrite`/`atomicWrite` sob um diretório pai com `Authenticated Users:(OI)(CI)(M)` artificialmente adicionado (sem depender de `C:\`) deve resultar, após o fix, em DACL sem ACE de escrita para Authenticated Users — verificado por `GetNamedSecurityInfo`.

**Direção de falso positivo que o teste deve cobrir:** sem o fix, o mesmo arquivo deve ter o ACE de Authenticated Users presente (baseline do defeito). Isso confirma que o teste não é vacuoso.

---

## 4. Desenho recomendado

### 4.1 Ponto de aplicação

Novo pacote `internal/fsperm` com dois arquivos:

- `internal/fsperm/fsperm_windows.go` (build tag implícito pelo nome `_windows.go`) — implementação real
- `internal/fsperm/fsperm_other.go` (`//go:build !windows`) — no-ops que retornam `nil`

Função exportada: `SetRestrictiveACL(path string) error`

Ambos os pontos de escrita atômicos chamam `SetRestrictiveACL`:

1. `GuardedWrite` (pathguard.go) — logo após `os.CreateTemp` retornar (antes de escrever dados)
2. `atomicWrite` (manager.go) — idem, logo após `os.CreateTemp`
3. `MkdirAll` (os dois sítios) — logo após `os.MkdirAll` (no sítio de pathguard.go:150 e no de manager.go:918), com ACEs `OI|CI` para que os arquivos criados dentro herdem o DACL restritivo

Justificativa para `internal/fsperm` em vez de inlinar nos dois sítios: os dois sítios implementam o mesmo padrão (MkdirAll + CreateTemp + Chmod + Write + Sync + Close + Rename); o `fsperm` evita terceiro sítio divergente quando um quarto uso surgir, e isola os imports de `golang.org/x/sys/windows` em um único arquivo de build.

### 4.2 Composição do DACL

ACEs no DACL explícito:

| Trustee | Direitos | Flags |
|---|---|---|
| Owner (SID do token do processo) | `GENERIC_ALL` | — (ACE de arquivo); `OI\|CI` (ACE de diretório) |
| `NT AUTHORITY\SYSTEM` | `GENERIC_ALL` | — (arquivo); `OI\|CI` (diretório) |
| `BUILTIN\Administrators` | `GENERIC_ALL` | — (arquivo); `OI\|CI` (diretório) |

DACL protegida (`SE_DACL_PROTECTED`) — desativa herança de ACEs do pai. Esta flag é obrigatória: sem ela, uma propagação posterior de permissões do pai (e.g., `icacls /reset` no pai) re-adiciona a ACE permissiva.

**Não incluir:** `NT AUTHORITY\Authenticated Users`, `BUILTIN\Users`, `Everyone`.

**Justificativa para incluir SYSTEM:** processos do SO (backup VSS, antivírus, indexador) rodam como SYSTEM; removê-lo pode causar falhas operacionais silenciosas.

**Justificativa para incluir Administrators:** o processo pode rodar como administrador; e operações de administração legítimas (inspeção, backup, limpeza) exigem acesso.

**Obtenção do Owner SID:** via `windows.OpenCurrentProcessToken()` + `windows.GetTokenInformation(TOKEN_USER)`. Não hardcodar nenhum SID. Se a obtenção falhar (cenário extremamente improvável — o processo tem um token), emitir aviso e retornar `nil` (non-fatal), NÃO usar um SID estático como fallback.

### 4.3 Sequência correta dentro de `atomicWrite` / `GuardedWrite`

```
os.CreateTemp(directory, ".trackfw-tmp-*")   // cria o arquivo; herda ACL do pai
fsperm.SetRestrictiveACL(temporaryName)      // ← AQUI, antes de Write
temporary.Write(data)
temporary.Sync()
temporary.Close()
os.Rename(temporaryName, filename)           // preserva o DACL (mesmo volume, documentado)
```

**`os.Rename` preserva o security descriptor?** Sim. `os.Rename` no Windows chama `MoveFileExW`. Dentro do mesmo volume, `MoveFileExW` move o objeto de arquivo com seu security descriptor intacto — o arquivo não é recriado, apenas o diretório é atualizado. Referência: MSDN "MoveFileExW" e "Security Considerations: Windows File Management" ("The security descriptor of a file is not changed when you move the file"). Consequência: aplicar DACL no arquivo temporário antes do rename é suficiente; o arquivo final tem o mesmo DACL.

**Invariante que garante same-volume:** `os.CreateTemp(directory, ...)` onde `directory = filepath.Dir(filename)`. O temporário e o destino estão sempre no mesmo diretório, portanto no mesmo volume. Essa invariante está presente em ambos `GuardedWrite` (pathguard.go:154) e `atomicWrite` (manager.go:922). Documentá-la como comentário nos dois sítios.

### 4.4 Tratamento de falha de `SetNamedSecurityInfo`

**Não falhar-fechado em FS sem suporte a ACL.** `SetNamedSecurityInfo` retorna `ERROR_NOT_SUPPORTED` em FAT/exFAT e alguns shares SMB sem ACL. Falhar-fechado tornaria trackfw inutilizável em drives removíveis e em alguns ambientes CI — é uma regressão funcional para usuários legítimos.

Política: `SetRestrictiveACL` emite `fmt.Fprintf(os.Stderr, "trackfw: aviso: não foi possível definir DACL restritiva em %s (%v) — verifique se o sistema de arquivos suporta ACLs\n", path, err)` e retorna `nil`.

**Erros a tratar como "FS sem suporte":** `syscall.ERROR_NOT_SUPPORTED`, `syscall.ERROR_INVALID_FUNCTION`, `windows.ERROR_INVALID_SECURITY_DESCR`. Para `ERROR_ACCESS_DENIED` (pouco provável, o processo criou o arquivo), idem — avisar e continuar.

**Para proveniência e quarentena (T-1, T-2):** documentar na mensagem de aviso que o controle de acesso do FS não está ativo, mas não bloquear a escrita. A alternativa (fail-closed) seria pior: um adversário em FAT poderia simplesmente colocar o projeto em FAT para contornar o trackfw inteiro.

### 4.5 Isolamento fora do Windows

`internal/fsperm/fsperm_other.go` com `//go:build !windows`:

```go
func SetRestrictiveACL(_ string) error { return nil }
```

Nenhum import de `golang.org/x/sys/windows` fora do arquivo `_windows.go`. A chamada em `GuardedWrite` e em `atomicWrite` compila normalmente em todos os sistemas operacionais e é no-op em não-Windows. O `os.Chmod(mode)` atual permanece em ambos os caminhos — em Linux/macOS é o mecanismo correto; em Windows é inerte mas inofensivo.

---

## 5. Testes que mordem no Windows

### 5.1 `TestSave_WritesAtomicallyWithPermissions` (identity)

Situação atual: afirma `Mode().Perm() == 0o600`. No Windows, `os.File.Mode().Perm()` reflete o bit `READONLY` do atributo, não a ACL — o assert sempre falha ou passa pelo motivo errado.

**Destino:** substituir (não skip, não guard de plataforma que elimine o assert) por:

- Em `_windows_test.go`: lê o DACL com `GetNamedSecurityInfo`; afirma ausência de ACE com bits de escrita (`WRITE_DAC`, `WRITE_OWNER`, `GENERIC_WRITE`, `FILE_WRITE_DATA`, `DELETE`, `MODIFY`) para Authenticated Users, BUILTIN\Users e Everyone. Afirma presença de ACE FullControl para o owner.
- Em `_test.go` (ou com `//go:build !windows`): mantém `Mode().Perm() == 0o600`.

### 5.2 Testes para todos os demais sítios (quarantine, provenance, manifest, render)

O mesmo padrão de assert de DACL deve ser aplicado nos testes de cada sítio que chama `GuardedWrite` ou `atomicWrite`. Se não existir teste específico de permissão para esses sítios, criar.

### 5.3 Como criar a raiz "ruim" no teste sem depender de `C:\`

**Não usar `C:\` nem `%TEMP%` como raiz do teste.** Em vez disso:

1. Criar um diretório temporário com `os.MkdirTemp("", "trackfw-dacl-test-*")`.
2. Chamar `fsperm_test_helper.AddPermissiveACE(dir, "NT AUTHORITY\\Authenticated Users", windows.GENERIC_WRITE|windows.GENERIC_READ)` (implementado no pacote de teste) para adicionar o ACE permissivo ao diretório recém-criado, simulando a herança de `C:\`.
3. Escrever um arquivo através de `GuardedWrite` / `atomicWrite` nesse diretório.
4. Ler o DACL do arquivo resultante com `GetNamedSecurityInfo` e afirmar ausência do ACE de Authenticated Users.

Isso elimina qualquer dependência da configuração da máquina de CI e reproduz o defeito em qualquer runner Windows, independente de onde esteja montado.

### 5.4 Baseline do defeito (falsificação na direção "sem fix")

Antes de afirmar que o fix funciona, o teste deve afirmar que SEM o fix o ACE estaria presente (ou seria herdado). Isso pode ser feito:

- No mesmo teste, antes de aplicar a correção: verificar que o arquivo criado diretamente com `os.WriteFile(path, data, 0o600)` no diretório "ruim" TEM o ACE de Authenticated Users. Isso prova que o ambiente de teste reproduz o defeito.
- Só então escrever via `GuardedWrite` e verificar que o ACE está ausente.

---

## 6. Riscos numerados (R-n)

**R-1 — Janela de exposição do arquivo temporário.**
Entre `os.CreateTemp` e a chamada de `SetRestrictiveACL`, o arquivo temporário tem a ACL herdada do pai (permissiva em root-c/runner-temp). A janela de exploração exige que o adversário saiba o nome aleatório (`.trackfw-tmp-*` + sufixo aleatório do Go — 10 caracteres hexadecimais) e abra o arquivo nesse intervalo. Mitigação: chamar `SetRestrictiveACL` como primeira operação após `CreateTemp`, antes de qualquer `Write`. O risco residual é baixo (nome imprevisível, janela de microssegundos), mas deve ser documentado.

**R-2 — FAT/exFAT/SMB sem ACL: falha silenciosa.**
`SetNamedSecurityInfo` falha em silêncio (ou com `ERROR_NOT_SUPPORTED`) em volumes sem suporte a ACL. O comportamento resultante é idêntico ao atual (sem proteção), não uma regressão. Política: emitir aviso em stderr, continuar. Não é aceitável fail-closed porque isso quebra usos legítimos. O implementador DEVE testar este ramo simulando o erro com uma stub/wrapper de `SetNamedSecurityInfo` que retorne `ERROR_NOT_SUPPORTED`.

**R-3 — Owner SID obtido em runtime, não hardcoded.**
O processo pode rodar como qualquer usuário (desenvolvedor, conta de domínio, conta de serviço). Usar `OpenCurrentProcessToken` + `GetTokenInformation(TOKEN_USER)` para obter o SID real. Não usar `WinCreatorOwnerSid` (S-1-3-0) porque em alguns contextos não resolve corretamente em runtime. Testar com conta que não seja a conta de builtin do runner.

**R-4 — SE_DACL_PROTECTED é obrigatório.**
Sem este flag, uma operação posterior de "reset" de ACL no diretório pai (e.g., `icacls /reset /T`) pode re-herdar a ACE permissiva no arquivo. O flag deve ser passado em `SetNamedSecurityInfo` como `windows.PROTECTED_DACL_SECURITY_INFORMATION`.

**R-5 — `os.Rename` preserva o DACL apenas dentro do mesmo volume.**
Invariante atual: `os.CreateTemp(directory, ...)` onde `directory = filepath.Dir(filename)` garante same-volume. Documentar esse invariante como comentário nos dois sítios. Se o invariante for quebrado no futuro (e.g., criando temp em `%TEMP%` diferente do destino), o DACL seria perdido no rename cross-volume — o file seria recriado pelo fallback de copy+delete do Go sem a ACL explícita. O implementador deve adicionar um comentário de advertência.

**R-6 — MkdirAll cria apenas o diretório folha com DACL explícito.**
`os.MkdirAll` pode criar múltiplos níveis. Apenas o diretório passado para `SetRestrictiveACL` recebe a DACL explícita. Diretórios intermediários criados na mesma chamada herdam do pai. Aceitável porque: (a) o diretório folha (`.trackfw/`) é quem recebe a proteção, que propaga via OI/CI para os filhos; (b) os diretórios intermediários (e.g., o próprio diretório do projeto) são de responsabilidade do usuário e do FS, não do trackfw.

**R-7 — `golang.org/x/sys/windows` já é dependência indireta — promover a direta é mudança de go.mod.**
O `go.mod` lista `golang.org/x/sys` como dependência indireta. Para importar `golang.org/x/sys/windows` em código de produto, a dependência deve ser promovida a direta (`go get golang.org/x/sys`). Isso é uma mudança pequena mas deve ser declarada no commit.

**R-8 — Testes `TestSave_WritesAtomicallyWithPermissions` e correlatos não devem usar `t.Skip` nem guard de plataforma que elimine o assert.**
Um guard `if runtime.GOOS == "windows" { t.Skip() }` ou `if runtime.GOOS != "windows" { t.Skip() }` que elimine o assert de DACL é equivalente a não ter o teste. O padrão correto é arquivo `_windows_test.go` separado (build tag implícito) com o assert de DACL, coexistindo com o arquivo sem sufixo que afirma o modo POSIX.

**R-9 — `BUILTIN\Administrators` no DACL pressupõe que o processo rodando como admin pode sempre ler/escrever seus próprios arquivos.**
Em ambientes de contêiner Windows sem o grupo Administrators (e.g., Windows Server Core com usuário não-admin puro), a ACE de Administrators pode ser desnecessária mas inofensiva. Se a conta atual não for membro de Administrators, a ACE ainda é inserida (o DACL é uma política, não exige que a conta exista no contexto atual). Inofensivo.

---

## Veredito

**APROVA COM RESSALVAS (R-1 a R-9).**

O desenho é tecnicamente correto e resolve o defeito medido. As ressalvas são implementáveis sem mudança de arquitetura e nenhuma é bloqueante para o início da codificação, desde que o implementador as atenda todas antes de submeter o PR.

Ressalvas prioritárias (bloqueiam aprovação do PR se não atendidas):

- **R-1**: `SetRestrictiveACL` chamado antes de qualquer `Write` (não depois)
- **R-4**: `SE_DACL_PROTECTED` presente na chamada de `SetNamedSecurityInfo`
- **R-8**: testes de DACL em `_windows_test.go`, sem skip nem guard que elimine o assert
- **R-3**: Owner SID obtido em runtime, não hardcoded

Ressalvas documentais (devem estar registradas em comentário de código ou commit message):

- **R-2**: política de fail-soft em FAT/SMB documentada no código
- **R-5**: invariante same-volume documentado nos dois sítios de `os.CreateTemp`
- **R-7**: `go.mod` atualizado com `golang.org/x/sys` como dependência direta

Ressalva de teste de produto (requer CI Windows para fechar):

- A queda de `TestSave_WritesAtomicallyWithPermissions` do estado atual (impossível em Windows) para o novo assert de DACL só é verificável no runner x64. A aprovação do PR deve aguardar o CI Windows verde neste teste.
