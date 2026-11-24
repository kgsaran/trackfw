# Revisão de Qualidade — ML-2A (REQ-2026-11-23, Wave 1)

**Veredito: REPROVA**

Data: 2026-11-24  
Revisor: hefesto-tf  
Branch: `fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado`  
Commits da Wave 1: d25e3b02, 96b8e428, afba024a, 7613c035  
Contrato: ADR-2026-11-24 D1–D4  

Build e testes passam sem erros:

```
ok  github.com/kgsaran/trackfw/internal/guard      3.047s
ok  github.com/kgsaran/trackfw/internal/generators 90.602s
```

Razão do veredito REPROVA: `credFindNearestProjectRoot` viola D3 — trata todo `err != nil`
como ausente e caminha além, incluindo `ErrNotRegularFile` e EACCES. Sitio de mesma causa que
os demais guards; Regra Dura de Causa Raiz exige ML na REQ vigente antes do merge.
(Item 2 do escopo de revisão — ✗ no guard de credencial.)

Nota sobre evidência: MÉDIO-1 é confirmado por leitura de código (3ae8d29c e HEAD). Um teste
dinâmico com payload de credencial não foi executado — o hook desta sessão bloqueia esses comandos
de shell. O padrão `err != nil → fall-through` está visível no código e no comentário da função.

---

## Achados

### MÉDIO-1 — `credFindNearestProjectRoot` viola D3: todo erro tratado como ausente

**Arquivo:** `internal/guard/credential.go`, linhas 1482–1500  
**Item de escopo coberto:** item 2 ("nenhum `err != nil → ausente` sobrando nos guards")

O ADR-2026-11-24 D3, linhas 59–63, é incondicional:

> D3: só "não existe" conta como ausente.
> - Ausente: só `os.IsNotExist` é ausente.
> - **Nega:** `ErrNotRegularFile` (diretório, FIFO) e qualquer outro erro negam.

Não há carve-out para a detecção de raiz no guard de credencial.

O código da função:

```go
_, err := fileutil.ReadRegularFile(filepath.Join(dir, "trackfw.yaml"))
if err == nil {
    return dir
}
// comentário: "os.IsNotExist → absent, skip; ErrNotRegularFile → not a valid root, skip."
parent := filepath.Dir(dir)
```

O fall-through após `err == nil` não distingue `os.IsNotExist`, `ErrNotRegularFile` ou EACCES —
todos fazem o loop caminhar para o pai. D3 exige que apenas `os.IsNotExist` caminhe; os demais devem
negar.

**Consequências em dois cenários:**

*Cenário A — único yaml na cadeia é um diretório:*  
`credFindNearestProjectRoot` encontra `ErrNotRegularFile`, caminha, chega à raiz do FS, retorna `""`.  
`RunCredential` linha 240–242: `if projectRoot == "" { return 0 }` — guard desabilitado.  
`credWalkStrictestMode` nunca é chamada.

`TestCredNested_DirectoryYamlCountsAsBlock` **não cobre** este cenário: o setup planta um yaml
regular em `root/` com `mode: warn`; o diretório está em `sub/`. `credFindNearestProjectRoot`
caminha além de `sub/trackfw.yaml` (directory) e encontra `root/trackfw.yaml` (regular) → projeto
detectado. O teste passa porque `credWalkStrictestMode` ainda encontra o diretório e retorna
`"block"`. O caso "único yaml não-regular" fica sem cobertura.

*Cenário B — `chmod 000 trackfw.yaml`:*  
`ReadRegularFile` retorna `*PathError{EACCES}` → fall-through → walks past → retorna `""` →
guard off (rc=0 silencioso).

| Código | Mecanismo | Resultado com credencial detectada |
|---|---|---|
| Pré-Wave 1 (`3ae8d29c` linha 233) | `os.Stat` consegue em chmod 000 → projeto encontrado; linha 334: `credReadModeWithDefault(..., "warn")` tenta ler → EACCES → retorna padrão `"warn"` | rc=0, attention JSON escrito |
| Pré-Wave 1 com `mode: block` + chmod 000 | Mesmo caminho; EACCES silencia o `mode: block` → retorna `"warn"` | rc=0, attention JSON (regressão no sentido oposto: o código antigo também violava D3 silenciando EACCES) |
| Wave 1 — cenário B | `ReadRegularFile` → EACCES → walks past → `""` → `return 0` | rc=0, sem attention JSON, sem sinal |

Ambas as versões retornam rc=0 para credencial detectada com yaml inacessível. A diferença é
visibilidade (attention JSON vs. silêncio) e o fato de que a Wave 1 estende o mesmo silêncio para
`ErrNotRegularFile` (cenário A). `rm` ou `mv trackfw.yaml` já produz `os.IsNotExist`, que desativa
o guard legitimamente via D3. Exploitabilidade é baixa; o bloco fundamenta-se no contrato.

**Comportamento esperado:** um erro não-IsNotExist durante a detecção de raiz não deve desligar o
guard. O design da correção pertence ao dono do código; o invariante é: `credFindNearestProjectRoot`
com `ErrNotRegularFile` ou EACCES não pode resultar em `RunCredential` retornando 0 silenciosamente.

---

### MÉDIO-2 — Sentenças de reconciliação incorretas (escopo global)

**Item de escopo coberto:** item 1 (reconciliação de testes novos/reescritos)

Cinco testes descrevem `credWalkStrictestMode` ou leitura de yaml que não ocorrem em
`RunCredentialGlobal`.

**`internal/guard/credential_nested_mode_test.go`, linha ~43:**  
`TestCredNested_GlobalPlantedWarnBlocks` — sentença: *"credWalkStrictestMode with caller-default=block treats mode:warn as weaker than the default and returns block."*  
`RunCredentialGlobal` não chama `credWalkStrictestMode` (ramo warn removido como dead code,
comentado no próprio código nas linhas ~452–458).

**`internal/guard/credential_nested_mode_test.go`, linha ~58:**  
`TestCredNested_GlobalNoYamlBlocks` — sentença: *"credWalkStrictestMode with no yamls in the ancestor chain returns the caller default (block)."*  
Mesmo problema: `RunCredentialGlobal` hardcoda block sem chamar a função.

**`internal/guard/credential_mode_test.go`, linha 228:**  
`TestReadCredentialGuardMode_EndToEnd_GlobalScope_WarnCRLFStillBlocks` — sentença: *"CRLF mode:warn yaml → RunCredentialGlobal returns rc=2 (global block wins)."*  
`RunCredentialGlobal` não lê o yaml; hardcoda block diretamente. O teste é **decorativo** para o
que a sentença afirma: remover o CRLF-stripping de `ReadCredentialGuardMode` não alteraria este
resultado.

**`internal/generators/credential_guard_test.go`:**  
`TestGlobalCredentialGuardScript_ProjectModeWarnDoesNotDowngradeGlobalBlock` — comentário: *"block still wins via credWalkStrictestMode(cwd, 'block')."* `RunCredentialGlobal` não chama `credWalkStrictestMode`.  
`TestGlobalCredentialGuardScript_RespectsExplicitProjectMode` — implica leitura de yaml pelo escopo global; a leitura não ocorre.

Comportamento do código é correto em todos os casos — o escopo global sempre bloqueia. Apenas as
anotações estão erradas. Sem impacto funcional; viola a Regra Dura de Reconciliação do CLAUDE.md.

---

### BAIXO-1 — `docs/cli-parity.md`: parágrafo de attention signal no escopo global descreve caso impossível

**Arquivo:** `docs/cli-parity.md` (seção alterada pela Wave 1)  
**Item de escopo coberto:** item 5

O parágrafo sobre quando o attention JSON é gravado no escopo global descreve um cenário
condicional ("só é gravado quando não há yaml ou não há chave — e o padrão efetivo for o padrão de
projeto"). `RunCredentialGlobal` **nunca** grava attention JSON — o ramo warn foi removido como dead
code (ML-1D, ADR-2026-11-24). A descrição correta seria: "O escopo global nunca grava attention
JSON." Pode confundir mantenedores sobre quando o banner de atenção é exibido.

---

### BAIXO-2 — Windows ADS contorna D1 em `hasTrackfwYAMLComponent`

**Arquivo:** `internal/guard/writescope.go`, linhas 874–882  
**Item de escopo coberto:** item 4 (normalização Windows)

`normalizePathComponent` faz strip apenas de `.` e espaços finais (linhas 859–862). Uma chamada
Write com path `...trackfw.yaml::$DATA` ou `...trackfw.yaml:zone.identifier` não é normalizada para
`trackfw.yaml` — `:` é preservado, `EqualFold` retorna false, D1 não bloqueia. No Windows/NTFS,
`trackfw.yaml::$DATA` é o stream padrão do arquivo, logo escrever nele é escrever no conteúdo
efetivo de `trackfw.yaml`. Esta rota não exige shell: uma única chamada Write-tool é suficiente.

Classificado BAIXO porque D2 e D4 neutralizam o efeito de um yaml nested plantado pelo subagente
(qualquer yaml que o subagente consiga criar só pode ACRESCENTAR proteção, nunca remover). D1 é
defesa em profundidade neste cenário. Não coberto pelas Formas I–VI do threat model; residual
aceitável se documentado no ADR como limitação conhecida de `normalizePathComponent`.

---

### BAIXO-3 — `TestD3_DirectoryNamedTrackfwYAML_Deny` — comentário interno impreciso sobre caminho de execução

**Arquivo:** `internal/guard/writescope_nested_root_test.go`, linhas 300–313

A sentença do teste (linha 282–283) está correta: `findProjectRootSafe` encontra diretório →
retorna `ErrNotRegularFile` → `RunWriteScope` linha 190–192 nega (rc=2). O teste é sensível à
correção (por leitura de `3ae8d29c:findProjectRoot`, `os.Lstat` em diretório tem sucesso → retorna
`(outer/.claude, true)` → `readWriteScopeFromYAML` falha silenciosamente → lista vazia → rc=0).

No entanto, o bloco de comentário nas linhas 300–313 descreve caminhos alternativos via
"D2's ancestor walk of buildFrozenList" e afirma "rc=2 on HEAD too" para o código anterior — o que
é incorreto: o código pré-Wave 1 não tinha ancestor walk; rc=0 era o resultado. O comentário
excessivo contradiz a sentença do teste e pode confundir futuros mantenedores sobre qual mecanismo
dispara o deny.

---

### BAIXO-4 — `findProjectRoot` wrapper em código de produção

**Arquivo:** `internal/guard/writescope.go`, linhas 544–548

Wrapper de 2 retornos que delega para `findProjectRootSafe`. Existe para compatibilidade com testes
antigos. Comentário no código documenta o papel explicitamente. Sem urgência.

---

## O que foi verificado sem achados

**D1 (subagente não escreve trackfw.yaml)**
- `hasTrackfwYAMLComponent`: split em `/` e `\`, `EqualFold` incondicional, `normalizePathComponent`
  strip de `.` e espaço. ✓
- Ordem e path normalizado: linha 159 `rawFilePath = normalizeTargetPath(rawFilePath, runtime.GOOS)`
  REATRIBUI `rawFilePath` antes de linha 177 `hasTrackfwYAMLComponent(rawFilePath)`. D1 é aplicado
  ao caminho normalizado. ✓
- D1 caminho normalizado vs resolvido (`EvalSymlinks`): D1 checa o path lexical normalizado antes
  de `resolveWriteScopePath` (linha 183). Um symlink com nome diferente apontando para
  `trackfw.yaml` não seria capturado por D1 — mas criar tal symlink requer shell, fora do threat
  model (Formas I–VI). Residual aceitável. ✓
- Item 2 (write-scope): sem `err != nil → ausente` nos guards de write-scope. ✓
- Item 2 (credential guard): `credFindNearestProjectRoot` FALHA — ver MÉDIO-1. ✗

**D2 (lista de congelados monotônica)**
- `buildFrozenList` sobe do projectRoot até a raiz do FS via `buildFrozenListForRoot`. ✓
- Ancestral com yaml não-regular → erro → guard nega. ✓

**D3 (write-scope)**
- `readWriteScopeFromYAML`, `findProjectRootSafe`, `buildFrozenList`: todos seguem `IsNotExist →
  ausente; qualquer outro → nega`. ✓
- FIFO: `fileutil.ReadRegularFile` usa O_NONBLOCK. `TestD3_FIFONamedTrackfwYAML_DenyWithoutHanging`
  registrado em `.github/windows-skips.json` com build tag `!windows`. ✓

**D4 (modo mais estrito da cadeia)**
- `credWalkStrictestMode(startDir, callerDefault)` sem `"block"` fixo como piso interno. ✓
- `RunCredential` passa `"warn"`; `RunCredentialGlobal` hardcoda block sem chamar a função
  (otimização correta; comentado no código). ✓
- `TestCredNested_ProjectWarnOnlyCounterArm`: rc=0 com mode:warn no projeto → piso de projeto não
  foi elevado. ✓

**Shell script `scripts/trackfw-credential-guard.sh`**
- POSIX correto: `while :`, `dirname`, `${d%/}`, `-f`. ✓
- Loop termina: `[ "$d" = / ]` antes de `dirname` para evitar `dirname /` infinito. ✓
- Consistência com `scaffold.go` e `validator_credential_guard_integrity_reference.go`. ✓

**`docs/cli-parity.md`**
- Tabela de escopos (padrão por escopo, modo efetivo com yaml explícito) correta com o código. ✓
- Descrição de D4 (ratchet unidirecional block > warn) corresponde a `credWalkStrictestMode`. ✓
- Parágrafo de attention signal no global → BAIXO-1 acima.

**Cobertura Windows**
- `TestD3_FIFONamedTrackfwYAML_DenyWithoutHanging` em `.github/windows-skips.json`. ✓

**Testes de reconciliação corretos (demais)**
- `TestD1_*`, `TestD2_*`, `TestD3_SymlinkToRegularFile_DoesNotBreakFrozen`: batem. ✓
- `TestCredNested_ProjectSubdirNoYamlBlocks`, `TestCredNested_ProjectPlantedWarnUnderBlockRootBlocks`,
  `TestCredNested_ProjectWarnOnlyCounterArm`, `TestCredNested_DirectoryYamlCountsAsBlock`,
  `TestCredNested_AntigravityPayloadCwdUsed`: sentenças batem. ✓
- `TestRunCredentialGlobal_WarnYamlDoesNotLowerGlobalBlock`,
  `TestRunCredentialGlobal_WarnYamlBlocksEvenWithoutRoadmapDir` (renomeados): asserções corretas. ✓
- `TestGlobalCredentialGuardScript_AlwaysBlocksNeverWritesAttention`: sentença correta. ✓
- `TestD3_DirectoryNamedTrackfwYAML_Deny`: sentença do teste correta; comentário interno impreciso
  → BAIXO-3 acima.

---

## Bloqueante para merge

**MÉDIO-1 é bloqueante.** `credFindNearestProjectRoot` violates D3 (item 2 do escopo) —
sitio de mesma causa que os demais guards corrigidos. Regra Dura de Causa Raiz exige ML na REQ
vigente. O PR não pode ser mergeado parcialmente com este sítio aberto.

**ML de correção — requisitos:**
- Corrigir `credFindNearestProjectRoot` para que `ErrNotRegularFile` e qualquer erro não-IsNotExist
  não silenciem o guard.
- Teste para o cenário "único yaml na cadeia é um diretório" (sem yaml regular mais acima) deve
  confirmar rc=2. Este teste requer `chmod 000` ou `os.Mkdir` apenas — sem credencial no payload,
  portanto não é bloqueado pelo hook desta sessão.
- Teste de chmod 000: deve pular quando `os.Geteuid() == 0` (root ignora permissões) e no Windows;
  registrar em `.github/windows-skips.json` como o FIFO test.

**Pendente para o arquiteto:**
- Nota de vault: mecanismo "root detection silences errors" — `credFindNearestProjectRoot` é lookup,
  não gate; erros não-IsNotExist devem propagar de forma que o chamador possa negar, não only
  `credWalkStrictestMode` (que nunca é chamada quando o root não é detectado). Sem a nota, próximo
  agente pode repetir o mesmo padrão.
- Correções MÉDIO-2 e BAIXO-1–4 não são bloqueantes; recomenda-se no mesmo ML de correção (mínimo
  diff adicional). ADS (BAIXO-2): documentar como residual no ADR.
