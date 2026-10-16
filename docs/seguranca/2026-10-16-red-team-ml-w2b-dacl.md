# Red-team ML-W2b — DACL explícita para arquivos de governança no Windows

> Produzido por: hades-tf | Data: 2026-10-16
> REQ: REQ-2026-09-03 | Branch: `fix/fechar-os-grupos-de-falha-de-windows-ml-w2-permissao`
> Evidência de CI: run 38910438658 (Pergunta 18, x64)
> Wave 0 de referência: `docs/seguranca/2026-10-16-wave0-dacl-0600-windows.md`

---

## Veredito

**BLOQUEIA.**

Dois achados bloqueantes independentes (R-A e R-B). Nenhum dos dois requer nenhuma das etapas que o ML implementou bem — o problema está no que ficou fora. Os demais itens (C–I) são residuais declarados ou não-bloqueantes.

---

## Itens 1–7: linha a linha

**Item 1 — `os.Rename` sobre destino pré-criado pelo atacante preserva qual DACL?**
FECHA. `MoveFileEx` com `MOVEFILE_REPLACE_EXISTING` no mesmo volume é uma operação de renomeação de entrada de diretório, não de cópia: o security descriptor viaja com o arquivo fonte. O destino criado pelo atacante é excluído; o arquivo temporário — com DACL restritiva criada por `CreateRestrictedFile` — toma o seu lugar. Medido por documentação (MSDN MoveFileExW §Security Considerations) e confirmado por construção (mesmo volume por invariante R-5 do ADR). DoS residual: atacante segura um handle sem `FILE_SHARE_DELETE` sobre o destino final, `os.Rename` falha, avaliado em R-H abaixo.

**Item 2 — TOCTOU entre `MkdirAll` e `SetRestrictiveDirACL`: existe corrida real?**
NÃO FECHA — achado bloqueante R-B (ver abaixo). `SetNamedSecurityInfo` é chamado pelo nome e segue reparse points por documentação de API (MSDN: "If the named object is a symbolic link, the security descriptor is set on the file that the symbolic link points to"; o mesmo se aplica a junctions). No intervalo entre o retorno de `MkdirAll` e a chamada de `SetNamedSecurityInfo`, o diretório recém-criado herda `AU:(M)` do pai (cenário `C:\` medido) — o atacante tem `DELETE` sobre ele e pode renomeá-lo e plantar uma junction. `SetNamedSecurityInfo` aplica então o DACL protegido `(Owner+SY+BA)` sobre o destino da junction, mutando o ACL tree de um diretório não relacionado. Este dano é novo em relação à escrita desviada pré-existente.

**Item 3 — `temporary.Chmod(mode)` no Windows altera a DACL?**
FECHA. Confirmado por leitura do fonte da stdlib (`$(go env GOROOT)/src/internal/poll/fd_windows.go:1141`): `FD.Fchmod` chama `GetFileInformationByHandle` para ler atributos, condiciona `FILE_ATTRIBUTE_READONLY`, e chama `SetFileInformationByHandle` com `FILE_BASIC_INFO`. Não toca a DACL. O DACL restritivo criado por `CreateRestrictedFile` é preservado intacto após `Chmod`.

**Item 4 — Heurística de fallback induzível via junction SMB?**
FECHA por motivo diferente do esperado — e expõe pré-existente. O lado da escrita: a junction substituindo `.trackfw` seria capturada pelo guard (`RejectSymlinks` detecta `ModeIrregular|ModeSymlink`) antes de `CreateRestrictedFile` ser chamado. O lado da leitura é o ponto real: `LoadProvenance` (provenance.go:92) usa `os.ReadFile(filename)` sem guard de reparse point. Em um projeto fresh sob `C:\`, o atacante cria `.trackfw` como junction para armazenamento sob controle dele antes de `trackfw init` correr — static, sem corrida. `trackfw validate` lê proveniência forjada. Este é pré-existente ao ML-W2b: o ML não introduz nem piora o lado de leitura. Declarado em R-C.

**Item 5 — `IsTrackfwOwned` re-protege diretório de terceiro?**
NÃO FECHA completamente — achado não-bloqueante R-D + R-E. `IsTrackfwOwned` caminha até a raiz do volume (não para na raiz do projeto), portanto qualquer caminho sob um ancestral chamado `.trackfw` dispara a re-proteção mesmo sem relação com o trackfw. Em projetos sob `C:\.trackfw\myproject`, os diretórios de render (`.claude/agents/`) seriam re-protegidos com DACL restritiva. Caso patológico mas não testado. Adicionalmente: o docstring de `TestIsTrackfwOwned_Examples` (fsperm_windows_test.go:312) promete "integration tests below" que verificariam a não-proteção em diretórios preexistentes não-trackfw — esses testes não existem. Violação da Regra Dura de Reconciliação (R-E).

**Item 6 — ACE explícita pré-plantada sobrevive à propagação; ML-W2c é necessário?**
NÃO FECHA — achado bloqueante R-A (ver abaixo). A questão vai além da ACE explícita: é de ownership. O atacante pode criar `.trackfw` antes da primeira execução do trackfw (tem `FILE_ADD_SUBDIRECTORY` via `AU:(M)` em `C:\`). `SetRestrictiveDirACL` define apenas o DACL — `SetNamedSecurityInfo` recebe `nil` para owner e `secInfo` sem `OWNER_SECURITY_INFORMATION`. O owner do diretório permanece o atacante. MSDN: "The owner of an object always has READ_CONTROL and WRITE_DAC, whether or not those rights are explicitly granted." Com `WRITE_DAC` implícito, o atacante modifica o DACL, concede-se `FA`, e pode excluir e forjar a proveniência. Não requer corrida. ML-W2c (regra de validate) não fecha isso sem ler também o OWNER.

**Item 7 — Mudança nos blind spots (13→15) mascara sítio novo sem guard?**
LEGÍTIMO. O `atomicWrite|directory` passou de `{2}` para `{1}` porque um dos dois sítios que apontavam para `os.CreateTemp(directory,...)` foi migrado para `fsperm.CreateRestrictedFile(directory,...)`; o analisador passa a enxergar dois sítios dentro de `fsperm` (fsperm_other.go `{1}` e fsperm_windows.go `{2}`). Justificativas verificadas: ambos os callers (`GuardedWrite`, `atomicWrite`) executam `RejectAndReport` antes de chamar `CreateRestrictedFile`; o guard é upstream. O analisador não enumera `windows.CreateFile` nem `SetNamedSecurityInfo` como primitivas de escrita — esse blind spot de instrumentação existe mas é não-bloqueante (R-F).

---

## Achados

### R-A — BLOQUEIA: implicit owner WRITE_DAC contorna DACL em `.trackfw` pré-criado

**Evidência:** `SetRestrictiveDirACL` (`fsperm_windows.go:143-158`) passa `nil` como owner em `SetNamedSecurityInfo` com `secInfo = DACL_SECURITY_INFORMATION | PROTECTED_DACL_SECURITY_INFORMATION` — não inclui `OWNER_SECURITY_INFORMATION`. O owner existente é preservado. O SDDL do diretório (`D:P(A;OICI;FA;;;ownerSID)...`) não contém `OWNER RIGHTS` ACE (`S-1-3-4`). MSDN garante `WRITE_DAC` implícito ao owner independentemente do DACL. No cenário `C:\` medido, `AU` tem `FILE_ADD_SUBDIRECTORY` — qualquer co-inquilino pode criar `.trackfw` antes de `trackfw init`, tornando-se owner. Com `WRITE_DAC` implícito após `SetRestrictiveDirACL`, o atacante re-escreve o DACL do diretório, concede-se `FILE_DELETE_CHILD` e `FILE_ADD_FILE`, exclui `thirdparty-provenance.json`, e deposita proveniência forjada. Não requer corrida. Bypass completo de T-1 (Wave 0 §2).

**Direção do fix:** incluir `OWNER RIGHTS` ACE no SDDL do diretório para neutralizar o `WRITE_DAC` implícito: `D:P(A;OICI;RC;;;OW)(A;OICI;FA;;;ownerSID)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)` — onde `OW = S-1-3-4` (Owner Rights), `RC = READ_CONTROL`. Alternativamente, incluir `OWNER_SECURITY_INFORMATION` no `SetNamedSecurityInfo` para transferir ownership ao owner do processo (requer verificação de privilégio). A primeira opção é independente de privilégio.

**Classificação:** por leitura de documentação (MSDN "Object Owner" + "OWNER RIGHTS ACE") e inspeção do código (`fsperm_windows.go:143-148` + `SetRestrictiveDirACL` SDDL em linha 134).

---

### R-B — BLOQUEIA: `SetNamedSecurityInfo` segue junctions — dano novo introduzido pelo ML-W2b

**Evidência:** `SetRestrictiveDirACL` (`fsperm_windows.go:143`) chama `windows.SetNamedSecurityInfo(path, ...)` por nome. MSDN documenta que a função segue symbolic links e reparse points para aplicar a segurança ao alvo. No intervalo entre `MkdirAll` (cria `.trackfw` com DACL herdada que inclui `AU:(M)`, logo `DELETE` para o atacante) e `SetNamedSecurityInfo`, o atacante renomeia `.trackfw` e planta uma junction. Resultado: (a) `SetNamedSecurityInfo` aplica DACL `D:P(Owner+SY+BA)` OI/CI ao alvo da junction — mutação de ACL tree em diretório não relacionado ao projeto; (b) `CreateRestrictedFile` subsequentemente cria a proveniência no alvo da junction (armazenamento do atacante). Este dano (mutação externa + desvio de escrita) não existia antes de ML-W2b, que introduziu a chamada de `SetNamedSecurityInfo`.

**Direção do fix:** abrir o diretório com handle usando `CreateFile(..., FILE_FLAG_BACKUP_SEMANTICS | FILE_FLAG_OPEN_REPARSE_POINT)`, verificar que `GetFileInformationByHandle` não reporta `FILE_ATTRIBUTE_REPARSE_POINT`, e usar `SetSecurityInfo` (handle-based) em vez de `SetNamedSecurityInfo`. Alternativamente: `CreateDirectory` com `SECURITY_ATTRIBUTES` (análogo a D1 para arquivos) — cria o diretório com DACL restritiva desde a abertura, sem janela de ACL permissiva.

**Classificação:** por leitura de documentação (MSDN `SetNamedSecurityInfo` §Remarks) e análise do código (`fsperm_windows.go:143`, ordenação `MkdirAll`→`SetRestrictiveDirACL` em pathguard.go:158-167). Janela é estreita porém o dano é determinístico após o atacante ganhar a corrida.

---

### R-C — não-bloqueante: leitura de proveniência usa `os.ReadFile` sem guard de reparse point

**Evidência:** `provenance.go:92` — `os.ReadFile(filename)` direto. Em projeto fresh sob `C:\` (atacante tem `FILE_ADD_SUBDIRECTORY`), atacante cria `.trackfw` como junction para local sob controle antes de `trackfw init`. `trackfw validate` lê proveniência do destino da junction. Pré-existente ao ML-W2b: o ML não introduz nem piora. Resíduo declarado. Não há PR a bloquear — mas deve ser documentado como trabalho futuro desta REQ (ML-W2c ou seguinte).

---

### R-D — não-bloqueante: `IsTrackfwOwned` caminha até raiz do volume

**Evidência:** `fsperm.go:30-41` — loop `filepath.Dir` sem limite de raiz de projeto. Projeto em `C:\.trackfw\myproject` faria `IsTrackfwOwned(".claude/agents")` retornar `true` e dispararia `SetRestrictiveDirACL` em diretório de terceiro. Caso patológico, baixa probabilidade. Não altera o vetor de ameaça no cenário medido.

---

### R-E — não-bloqueante: violação da Regra Dura de Reconciliação no docstring

**Evidência:** `fsperm_windows_test.go:312-322` — docstring de `TestIsTrackfwOwned_Examples` afirma "The actual non-protection is enforced by the caller's IsTrackfwOwned guard — tested implicitly by the integration tests below." Não existem testes de integração a seguir no arquivo. O comportamento de não-proteção de diretórios preexistentes não-trackfw pelo caller (`atomicWrite`/`GuardedWrite`) não tem teste. Regra Dura de Reconciliação: o artefato entregue (docstring) afirma cobertura que o diff não contém. Não bloqueia o PR pelo defeito de segurança, bloqueia pelo processo.

---

### R-F — não-bloqueante: analisador de contenção não cobre `windows.CreateFile` / `SetNamedSecurityInfo`

**Evidência:** grep no `containment_live_test.go` — nenhuma entrada para `windows.CreateFile` ou `SetNamedSecurityInfo` como primitiva de escrita. O caminho principal de escrita (`CreateRestrictedFile` → `windows.CreateFile`) e a mutação de DACL (`SetNamedSecurityInfo`) estão fora da população do instrumento. O blindspot é nomeado e justificado como upstream-guarded — mas a DACL mutation em R-B ficaria invisível para o instrumento mesmo com o fix aplicado. Limitação de instrumentação, não de produto.

---

### R-G — não-bloqueante: DoS via handle sem `FILE_SHARE_DELETE` sobre o destino final

**Evidência:** `os.Rename` → `MoveFileEx(MOVEFILE_REPLACE_EXISTING)` falha se outro processo tem handle aberto sobre o destino final sem `FILE_SHARE_DELETE`. Atacante abre `thirdparty-provenance.json` em modo que exclui `FILE_SHARE_DELETE`, impede a atualização pelo trackfw. Disponibilidade, não integridade. Não bloqueia.

---

### R-H — não-bloqueante: resíduo declarado de legado e leitura-side

Arquivo de proveniência pré-fix (D5 do ADR): a medição do run 38910438658 (`TestGuardedWrite_LegacyFileRetainsDACL`) confirma que `SetRestrictiveDirACL` com DACL protegida propaga para filhos legados (inherited ACE removida) — `legacy.json` passou de `AU write ACE = true` para `false` após um `GuardedWrite` de outro arquivo no mesmo `.trackfw`. Sobra ACE explícita (não herdada) pré-existente, que a propagação não remove. Resíduo declarado no ADR D5. ML-W2c (validate DACL) não fechariam o R-A sem ler também o OWNER.

---

## Evidência medida do run 38910438658 (Pergunta 18, x64)

| teste | resultado | o que afirma |
|---|---|---|
| `TestCreateRestrictedFile_HasRestrictiveDACL` | PASS | `CreateRestrictedFile` cria arquivo sem ACE de escrita para AU, Users, Everyone, com `SE_DACL_PROTECTED` |
| `TestCreateRestrictedFile_DetectorError_FailsClosed` | PASS | falha do detector retorna erro, não fallback silencioso |
| `TestSetRestrictiveDirACL_ProtectsPreexistingTrackfwDir` | PASS | `SetRestrictiveDirACL` remove ACE de AU de `.trackfw` preexistente |
| `TestGuardedWrite_FinalFileHasRestrictiveDACL` | PASS | arquivo final de `GuardedWrite` não tem ACE de AU/Users/Everyone |
| `TestGuardedWrite_LegacyFileRetainsDACL` | PASS | propagação remove inherited AU ACE de arquivo legado após GuardedWrite no mesmo dir |

Nenhum destes testes cobre os vetores de R-A (owner implícito) nem R-B (junction entre MkdirAll e SetNamedSecurityInfo).

---

## Itens que esta análise NÃO cobre

- Comportamento em ARM64 (não medido; mecanismo NTFS é independente de ISA, mas não verificado).
- Volumes SMB com ACL (cenário de teste não disponível localmente).
- PowerShell 5.1 vs 7 para paths de junction (não relevante para esta análise).
