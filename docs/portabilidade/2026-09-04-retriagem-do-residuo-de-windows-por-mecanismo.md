---
title: Re-triagem do resíduo de falhas de Windows, por MECANISMO (não por sintoma)
date: 2026-09-04
author: artemis-tf (QA — investigação, nenhuma correção aplicada)
status: investigação concluída
---

# Re-triagem do resíduo de falhas de Windows por mecanismo

> 🔴 Este documento é investigação pura. Nenhum arquivo de produto, de teste ou de fixture foi
> alterado para produzi-lo. Nenhuma operação de git foi executada por este agente.

## 1. Contagem reconfirmada

Medida por mim, run `33931363032`, job `windows-full-suites` (databaseId `101212333073`), via
`gh api repos/kgsaran/trackfw/actions/jobs/101212333073/logs --allow-escape-sequences` (o `gh run
view --log` recusou o log porque o run inteiro ainda estava `in_progress` no momento da medição —
o job de Windows já tinha `conclusion: failure`, mas o job `parity` seguia rodando; a API de job-log
não tem essa dependência).

```bash
grep -cE 'Z --- FAIL' win-full.log        # Go, topo (subtestes não contam)
grep -oE '# fail [0-9]+' win-full.log     # Node (tap): "# fail 34"
grep -oE '[0-9]+ failed, [0-9]+ passed' win-full.log   # Python (pytest): "21 failed, 1588 passed"
```

```
Go     45
Node   34
Python 21
       ──
       100
```

**Confirma exatamente os números do handoff (45/34/21/100).** Nenhuma das minhas contagens
diverge desta vez — a nota do vault `contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo`
descreve o erro anterior (padrão de grep sem o timestamp do prefixo por linha), e o padrão usado
aqui já o inclui. As duas pontas desta contagem (esta seção) foram medidas por mim, na mesma sessão,
com os mesmos três comandos — não há comparação contra número lembrado.

Job `windows-defect-reproduction`: **failure** (mesmo run). Job `windows-integrations-resolve`:
**success** — restrição útil: resolução de integrações não está entre os mecanismos candidatos
abaixo, porque esse caminho já está provado correto em Windows real.

## 2. Metodologia

Para cada grupo abaixo, o critério de pertinência foi **"leio o código de produção e/ou de teste
que gera a mensagem, não só o texto da mensagem"**. Quando o código foi lido e a causa fica evidente
por construção, marco **CONFIRMADO**. Quando a evidência aponta fortemente mas não há como
reproduzir a plataforma (estou em macOS), marco **HIPÓTESE**, com o discriminante que a confirmaria
no CI. Quando não cheguei a uma explicação defensável, marco **DESCONHECIDO** e listo a evidência
crua.

Nenhum grupo abaixo foi aceito "de graça" a partir do handoff — os quatro grupos citados no handoff
(escape/aspas, CRLF, indexação por basename, `TestPathIsAnchoredForHookConfig_ControlePOSIX`) foram
lidos no código de novo, e o número de cada um está reconfirmado ou corrigido nesta seção.

## 3. Grupos, ordenados por retorno (falhas fechadas / tipo de correção)

### G4 — CONFIRMADO. Teste busca substring crua contra JSON corretamente escapado (produção certa, teste errado)
**~22 falhas — maior grupo de retorno, e o mais barato de fechar (só teste).**

**Mecanismo, em uma frase:** o produto escreve o caminho absoluto nativo do Windows (`C:\Users\...`)
dentro de um campo de string JSON via `encoding/json`, que **escapa cada `\` como `\\`** por
especificação; o teste constrói o caminho esperado com `filepath.Join` (nativo, `\` simples) e faz
`strings.Contains` contra os **bytes crus do arquivo já serializado** — nunca bate, porque o número
de barras invertidas diverge.

**Evidência (código lido):**
```go
// internal/generators/agentfiles.go:1476, mergeClaudeHookArray
hObj["command"] == command   // command = scriptPath, native separator, sem tratamento

// internal/commands/update_harness_test.go:195-202
wantScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
if !strings.Contains(string(data), wantScript) { t.Fatalf(...) }
```
`json.MarshalIndent` escreve `"command": "C:\\Users\\...\\trackfw-credential-guard.sh"` (cada `\`
dobrado). `wantScript` tem `\` simples. `strings.Contains` nunca casa.

**Falsificação:** trocar a asserção para decodificar o JSON e comparar o campo `command` já
decodificado (que Go automaticamente des-escapa) contra `wantScript` — deve passar. Ou: comparar
contra `strings.ReplaceAll(wantScript, `\`, `\\`)`. Não testado neste ML (seria correção).

**Discriminante (o que eu veria se a causa estivesse errada):** se a causa fosse "produto grava
caminho errado", o `Path`/`State` do JSON retornado por `--json` (checado nas linhas 174-193 do
mesmo teste, ANTES desta asserção) já teria falhado — e não falha. O produto escreve o hook
corretamente; só a comparação de string crua está errada.

**Lista completa — Go (8), confirmado por leitura de código:**
`TestUpdateHarnessCmd_CredentialGuardClaudeInstallsViaCLI`,
`TestUpdateHarnessCmd_CredentialGuardCodexInstallsViaCLI`,
`TestUpdateHarnessCmd_CredentialGuardGeminiInstallsViaCLI`,
`TestUpdateHarnessCmd_CredentialGuardCursorInstallsViaCLI`,
`TestUpdateHarnessCmd_CredentialGuardCopilotInstallsViaCLI`,
`TestUpdateHarnessCmd_CredentialGuardKiroInstallsViaCLI`,
`TestUpdateHarnessCmd_GitBranchGuardInstallsViaCLI`,
`TestUpdateHarnessCmd_GitBranchGuardAndCredentialGuardCoexistIdempotently`.

**Node (7) e Python (7) — CONFIRMADO por leitura de código, não mais hipótese:**
```js
// npm/tests/update-harness.test.js:797-799
const written = fs.readFileSync(path.join(homeRoot, ...relPath), 'utf8')
const wantScript = path.join(homeRoot, '.trackfw', 'scripts', 'trackfw-git-branch-guard.sh')
assert.ok(written.includes(wantScript), ...)   // written é o JSON já serializado, com "\" duplicado
```
```python
# pypi/tests/test_update_harness.py:1223-1225
written = home.joinpath(*rel_parts).read_text(encoding="utf-8")
want_script = str(home / ".trackfw" / "scripts" / "trackfw-git-branch-guard.sh")
assert want_script in written   # written é o JSON já serializado (json.dump escapa "\" também)
```
Byte a byte o MESMO mecanismo do Go: `path.join`/`pathlib` nativo (backslash) comparado por
substring cru contra JSON já serializado (`JSON.stringify`/`json.dump`, que escapam `\` do mesmo
jeito que `encoding/json`). **Lista: Node `not ok 793, 797, 801, 805, 809, 813, 816`; Python
`test_update_harness.py::test_git_branch_guard_installs_absolute_path_with_install_missing` (6
parâmetros) + `test_claude_credential_guard_and_git_branch_guard_coexist_idempotently`.**

**Grupo fechado: 22/22 confirmado por leitura de código nos 3 runtimes — não é mais estimativa por
paridade de nome.** Esta é a verificação que o handoff pediu explicitamente para não pular (o
precedente do `IsAbs`, estimado em ~14 por semelhança de sintoma e que entregou 2): aqui as 22
foram lidas uma a uma no código-fonte do teste, não inferidas pelo nome.

**Correção esperada:** só teste, nos 3 CLIs — nenhum risco de produto.

---

### G1 — CONFIRMADO. Parser de frontmatter cego a CRLF (Wave 5 da ADR, ainda NÃO implementada)
**29 falhas — ADR já `Accepted`, D1-D4 escritos, código pendente.**

**Mecanismo:** `core.autocrlf=true` no checkout do Windows entrega os assets embarcados
(`go:embed`) com `\r\n`; o parser de frontmatter procura o delimitador `---\n` e não casa com
`---\r\n`, conclui "sem frontmatter" e escreve um segundo frontmatter por cima do primeiro. Todo
teste que depende de reescrever/injetar campos no frontmatter (model, identity, tools) herda a
falha, porque o ponto de parsing é compartilhado.

**Evidência byte a byte (Node, `not ok 26`):**
```
'developer_instructions = "---\r\nname: trackfw-architect\r\n...model: opus\r\n...'
```
O `\r\n` está **literalmente dentro do valor capturado como corpo**, prova direta de que o parser
não reconheceu o delimitador.

**Evidência (Go, `TestRenderOpenCodeAgent`):** a saída final contém o frontmatter OpenCode válido
seguido do frontmatter Claude completo por baixo (`model:`, `tools:`, `memory:` aparecem — campos
que o OpenCode não deveria ter), mostrando concatenação em vez de substituição.

**Falsificação:** normalizar `\r\n`→`\n` na entrada do parser (D1 da ADR) deve fazer os 29 fecharem
juntos, sem tocar `.gitattributes` dos assets (D4 — mascarar a entrada apaga a evidência, medido
pelo ML-1C anterior).

**Discriminante:** um membro deste grupo que falhe por **comparação de bytes de golden** sem
qualquer `\r` visível no diff pertenceria à metade já fechada (`.gitattributes`/eol), não a este
grupo — nenhum dos 29 abaixo tem essa forma; todos mostram `\r\n` no valor ou duplicação de bloco.

**Lista completa — Go (13):** `TestRenderOpenCodeAgent`, `TestOpenCodeAgentsLifecycleEndToEnd`,
`TestRenderNativeAgentFormats`, `TestRenderCustomAgentTomlEmitsCodexModel`,
`TestRenderSubagentRouteEmitsCursorModel`, `TestRenderSubagentRouteCursorWithIdentityRewritesModelAndName`,
`TestRenderAgentDirectory`, `TestRenderWithoutIdentityMatchesFrozenGoldens`,
`TestRenderSubagentRouteInjectsIdentity`, `TestRenderAllRepresentationsRenderIdentityName`,
`TestResolveAgentModelMatchesRender`, `TestIdentityPropagatesThroughInstallMutationCaller`,
`TestIdentityPropagatesThroughInitInstallAITools`.

**Node (16), confirmado por trecho `\r\n` no próprio diff da falha:** `not ok 26, 27, 39, 492, 493,
495, 496, 497, 498, 499, 500, 501, 510, 520, 528, 593`.

**Python: 0 nesta forma** — o parser de frontmatter do Python usa `open()` com universal newlines
por padrão (a própria ADR já registra isso como risco de assimetria a medir).

🔴 **Achado NÃO previsto no handoff — CRLF cega um SEGUNDO parser, fora do escopo do Wave 5:**
```
pypi/tests/test_barrier.py::test_barrier_cli_crlf_roadmap_gates_da_wave_e_reconhecido_e_comando_roda_e2e
  AssertionError: esperava exit 1 (blocked pelo gate), stdout= stderr=trackfw barrier: malformed
  gates block at line 19: '**Gates da wave:**' must be immediately followed by a fenced code block
```
Este é o parser do **bloco de gates da wave** dentro do `barrier`, não o parser de frontmatter que a
ADR-2026-09-04 cobre. **A Wave 5, como está escrita, NÃO fecha este teste** — precisa de patch
próprio no reconhecedor de bloco cercado do `barrier` Python (e, por paridade, verificar se Go/Node
têm o mesmo teste e o mesmo problema — não verificado nesta sessão). Contá-lo separado do grupo
principal: **G1-bis, 1 falha, mecanismo irmão mas código diferente.**

**Correção esperada:** produto, nos 3 CLIs (D1-D4 já decididos), mais o patch G1-bis no `barrier`.

---

### G3 — CONFIRMADO. Fixture de teste grava path nativo (com `\`) direto num template JSON sem escapar → JSON inválido → validator falha-aberto em silêncio
**5 falhas confirmadas em Go, +4 hipótese em Node.**

**Mecanismo:** o helper de fixture concatena a string do caminho (via `+`, não via
`json.Marshal`/`encoding/json`) dentro de um literal JSON:
```go
// internal/validator/validator_git_branch_guard_test.go:264-276
func globalClaudeSettingsWithCommandNoType(scriptAbsPath string) string {
    return `{... "command": "` + scriptAbsPath + `"} ...}`
}
```
Em Windows, `scriptAbsPath` é `C:\Users\...\trackfw-git-branch-guard.sh` — os pares `\U`, `\A`,
`\T`, `\1` etc. **não são escapes JSON válidos**. `json.Unmarshal` retorna erro; a regra
(`validateGuardGlobalHookResolvable`, linha 152-153) captura o erro e faz `continue` — **pula o
arquivo inteiro em silêncio**, por design de fail-open ("Unreadable/invalid JSON all skip that file
in silence"). O teste espera violação; recebe lista vazia.

**A mesma classe explica a mensagem textual do handoff** ("invalid character 'U' in string escape
code") em `TestClaimOrigin_LegacyManifestReadsAsCatalog` — mesmo padrão, arquivo de manifesto
diferente.

**Falsificação:** trocar a concatenação de string por `json.Marshal`/um `map[string]interface{}`
serializado de verdade nos helpers de fixture; o JSON passa a ser válido em qualquer separador, e a
regra volta a inspecionar o arquivo.

**Discriminante:** se a causa fosse "regra não reconhece caminho ancorado no Windows", a mensagem de
erro seria "no violation about missing type" com uma lista **não vazia** contendo outra violação, ou
um erro explícito de parse propagado — em vez disso, `err == nil` no chamador do teste (o arquivo é
tratado como "não existe/inválido, pular"), e é exatamente esse silêncio que aparece como `obteve:
[]`.

**Lista completa — Go (5):** `TestGuardGlobalHookResolvable_GlobalInstaladoMasScriptAusente_Dispara`,
`TestGuardGlobalHookResolvable_MalformedTypeMissing_Dispara`,
`TestGitBranchGuardGlobalHookResolvable_KiroDedicatedFile_DisparaScriptAusente`,
`TestGuardGlobalHookResolvable_KiroDoisArquivosDedicados_NaoRegrideNaoDuplica`,
`TestClaimOrigin_LegacyManifestReadsAsCatalog`.

**Node (HIPÓTESE, não confirmado por leitura — mesmo padrão de nome):** `not ok 463, 464, 473, 475`
(`validator_git_branch_guard`-equivalentes). Código Node não lido nesta sessão.

**Correção esperada:** só teste (fixtures), nos runtimes onde o padrão se confirmar.

---

### G9 — CONFIRMADO (já diagnosticado pela própria roadmap, reconfirmado com +2 membros). ENOTDIR silencioso no Windows
**3 falhas, é defeito de PRODUTO e é o único deste lote com risco de segurança/observabilidade
adjacente (diagnóstico de erro que desaparece).**

**Mecanismo:** quando um estado (`wip`, `analyzing`, etc.) existe no disco como **arquivo regular**
em vez de diretório, `os.ReadDir` em POSIX devolve `ENOTDIR` e o validador emite o
warning/violation "could not read directory". No Windows, `internal/validator/validator.go:1698-
1702` não recebe o mesmo sinal e a regra vai a silêncio — sem produzir warning nem violation.

Isto já estava diagnosticado pelo ML-4B anterior (`TestStaleWIPReportsWIPWalkError`) como defeito
real, explicitamente marcado "não aceitar guarda de plataforma no assert" pelo arquiteto. Eu **medi
mais 2 ocorrências da mesma forma** que não estavam na lista original:

**Lista completa — Go (3):** `TestStaleWIPReportsWIPWalkError`, `TestFolderStatus_DiretorioNaoLegivel_P2`
(`validator_test.go:1499`, "esperado warning sobre diretório ilegível, obteve: []"),
`TestFilenameUniqueness_DiretorioNaoLegivel_P2` (mesma forma, violation em vez de warning).

**Falsificação:** tratar o erro de `os.ReadDir` no Windows de forma que reconheça "isto é um
arquivo, não um diretório" (`errors.Is`/checagem de `syscall.Errno` equivalente no Windows) deve
fazer os 3 fecharem juntos.

**Discriminante:** se a causa fosse outra, o Go em POSIX (que passa hoje) não estaria fazendo a
MESMA checagem `ENOTDIR` com sucesso — mas está (comprovado pelo próprio teste passar em macOS/CI
Linux).

**Correção esperada:** produto, `internal/validator/validator.go` (e paralelos Node/Python se
existirem — não verificado).

---

### G2 — CONFIRMADO. `%q` do Go dobra a barra invertida na mensagem; teste compara com string crua
**4 falhas — o exemplo citado no handoff, e mais 3 do MESMO padrão de código que eu achei.**

**Mecanismo:** `fmt.Sprintf(..., "%q", caminhoComBarraInvertida)` produz uma string Go-escapada
(cada `\` vira `\\`) — comportamento correto e documentado de `%q`. O teste constrói o "esperado"
com `filepath.Join` (barra simples) e faz `strings.Contains`/`hasViolation` contra a mensagem já
formatada — nunca bate.

**Evidência (3 sítios de produção lidos, todos `%q`):**
```go
// internal/validator/validator.go:2083
msg := fmt.Sprintf("adr_dir %q does not exist", adrDir)
// internal/validator/validator_credential_guard.go:457
"%s (%s) references %s resolved to %q, but the script does not exist..."
// internal/validator/validator_thirdparty_provenance.go:164
"thirdparty_artifact_has_provenance: %q is claimed as a third-party artifact but has no entry..."
```

🔴 **Correção do handoff — não é só `TestCredentialGuardHookResolvable_CaminhoResolvidoEhFisicoNao
Simlink`; é uma FAMÍLIA de 4, e eu separei `TestThirdPartyArtifactHasProvenance_BranchI` deste
mesmo teste-arquivo do grupo de baixo (G10) porque o mecanismo é diferente — ver a ressalva do
revisor abaixo.**

**Falsificação:** trocar `%q` por `%s` (ou o teste passar a comparar contra `fmt.Sprintf("%q", ...)`
do valor esperado) fecha os 4 sem tocar lógica de negócio.

**Discriminante — o que separa este grupo do G10:** aqui a violação **correta** já está na lista de
mensagens; só a busca textual falha. Prova: em `TestThirdPartyArtifactHasProvenance_BranchI`, `len(msgs)
== 1` E `strings.Contains(msgs[0], "D2 branch i")` **passam** — só o terceiro assert
(`strings.Contains(msgs[0], destination)`) falha, porque `destination` tem barra simples e a
mensagem (via `%q`) tem barra dupla. Se a causa fosse outra (ex.: branch errado disparando), o
segundo assert já teria falhado — e não falha.

**Lista completa — Go (4):** `TestCredentialGuardHookResolvable_CaminhoResolvidoEhFisicoNaoSimlink`,
`TestValidate_NonExistentADRDirs_WarningByDefault`, `TestValidate_NonExistentADRDirs_StrictCIPathsError`,
`TestThirdPartyArtifactHasProvenance_BranchI_MissingProvenanceEntry`.

**Correção esperada:** só teste (ou trocar `%q`→`%s` na mensagem — decisão de UX, não afeta
segurança: a mensagem já nomeia o caminho, só com escaping diferente).

---

### G10 — HIPÓTESE (não confirmado — precisa do CI). Chave de proveniência não encontrada num fluxo real de install+validate
**2 falhas — DIFERENTE de G2 apesar de estar no mesmo arquivo de regra.**

**Mecanismo hipotético:** `validateThirdPartyArtifactHasProvenance` reconstrói a chave de
proveniência via `filepath.Rel(root, destination)` (`root = os.Getwd()` + `EvalSymlinks`). Se
`root` e `destination` (este último gravado no manifesto por `Manager.Install` num momento
diferente do processo) usarem **formas distintas do mesmo diretório temporário do Windows**
(nome curto 8.3 `RUNNER~1` vs. nome longo `runneradmin`), `filepath.Rel` — que é manipulação de
string, não resolução de identidade — não consegue cortar o prefixo comum corretamente, e a chave
recalculada não bate com a chave gravada em `.trackfw/thirdparty-provenance.json`. Resultado:
branch i dispara **por engano** num artefato que foi corretamente aprovado e instalado.

**Evidência medida (não decisiva, mas real, do próprio log):** as duas formas aparecem no MESMO
job:
```
C:\\Users\\RUNNER~1\\...     (curta)
C:\\Users\\runneradmin\\...  (longa)
```
E especificamente: `TestThirdPartyInstall_PassesValidateEndToEnd` (fluxo real via comando) imprime
a forma **curta**; `TestThirdPartyArtifactHasProvenance_BranchI_MissingProvenanceEntry` (fixture
isolada, via `resolvedRootForTest`) imprime a forma **longa** — ou seja, dois HELPERS de teste
diferentes já produzem formas diferentes do mesmo host. Isso é consistente com a hipótese, mas não
a prova: não tenho acesso a Windows para instrumentar `os.Getwd()` vs. a chave gravada no manifesto
dentro do MESMO teste que falha.

**Por que não é o mesmo grupo que G2:** aqui a asserção que falha é "não deveria haver nenhuma
violação `thirdparty_artifact_has_provenance`", e ela aparece — não é uma questão de escaping de
mensagem, é o branch errado disparando de verdade.

**Discriminante que confirmaria/refutaria em CI:** instrumentar
`validateThirdPartyArtifactHasProvenance` para logar `root`, `destination` e `provenanceKey`
calculados, e comparar com a chave real em `prov.Entries` no momento da falha. Se as duas formas
(curta/longa) aparecerem uma de cada lado, a hipótese está confirmada; se ambas forem idênticas, o
mecanismo é outro e volta a "desconhecido".

**Lista completa — Go (2):** `TestThirdPartyInstall_PassesValidateEndToEnd`,
`TestThirdPartyInstall_TamperAfterInstallFailsValidateEndToEnd`.

**Correção esperada:** se confirmado, produto — provavelmente ancorar por `filepath.EvalSymlinks`
em AMBOS os lados (gravação e leitura) ou por identidade de arquivo em vez de string-prefixing.

---

### G5 — CONFIRMADO (Go), HIPÓTESE por nome (Python). NTFS não honra bits de modo restritivos
**Já enumerado como "vira REQ" pelo ML-4B anterior; não relitigado aqui, só recontado.**

**Evidência:** `identity_test.go:127: permissao do arquivo = 666, esperava 0600`. NTFS não expressa
`0600`/`0400` como POSIX; `os.Chmod(0600)` é praticamente no-op no filesystem do Windows nativo.

**Lista — Go (1):** `TestSave_WritesAtomicallyWithPermissions`. **Python (1, por nome, não
verificado):** `test_identity.py::test_save_is_atomic_and_mode_0600`.

**Correção esperada:** decisão de arquitetura — guarda de plataforma na ASSERÇÃO de modo (não do
comportamento), já que NTFS genuinamente não tem essa granularidade; precedente:
`vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`.

---

### G8 — CONFIRMADO. `findRoadmap` retorna separador nativo; teste compara com literal POSIX
**2 falhas.**

**Evidência:**
```go
// internal/generators/roadmap.go:512
return filepath.Join(dir, e.Name()), nil   // nativo: "\" no Windows
```
```go
// internal/generators/roadmap_test.go:330,352
dst := "docs/roadmaps/zeus/analyzing/ROADMAP-analyze-by-agent.md"   // literal POSIX
if found != dst { ... }                                             // igualdade de string
```
`findRoadmap` é consumido para **abrir arquivo de verdade** — nativo é a forma correta ali (não é
"artefato autorado" no sentido do ADR-2026-09-04, é uso de filesystem). O defeito é do teste: o
literal deveria ser construído com `filepath.Join` também, ou a comparação deveria usar
`filepath.ToSlash` dos dois lados.

**Falsificação:** trocar `dst` por `filepath.Join("docs","roadmaps","zeus","analyzing","ROADMAP-...")`
fecha os 2 sem tocar produto.

**Lista completa — Go (2):** `TestMoveRoadmap_AnalyzingFlat`, `TestMoveRoadmap_AnalyzingByAgent`.

**Correção esperada:** só teste.

---

### G6 — CONFIRMADO, e é a exceção que a própria Wave 3 já declarou fora de escopo (D2)
**1 falha — não deve ser corrigida por este REQ; citando aqui só para não deixá-la "sem grupo".**

**Evidência:** `manager_test.go:210: Install("/tmp/outside-trackfw.md", global) accepted unsafe
destination`. `/tmp/...` é um caminho POSIX absoluto; em Windows `filepath.IsAbs("/tmp/...")` é
`false`, então o guard de travessia de `internal/integrations/manager.go` (que usa `filepath.IsAbs`
de propósito, por decisão da própria ADR-2026-09-04 D2) não o reconhece como "fora do projeto" da
mesma forma que em POSIX.

Isso é **exatamente** o `manager.go:703,726` que a Wave 3 já listou como **FORA DE ESCOPO** — "mexer
aqui quebra resolução real de caminho no Windows, com falha intermitente". Confirmo essa decisão
por leitura direta do teste; não proponho tocar.

**Lista — Go (1):** `TestManagerRejectsTraversalAbsoluteMismatchAndNUL`.

**Correção esperada:** nenhuma, por decisão já tomada (D2). Se a arquitetura decidir revisitar isso
no futuro, é uma REQ de segurança própria, com análise de risco de travessia real primeiro.

---

### G12 — DESCONHECIDO (mecanismo não confirmado; fixture é válida, ao contrário de G3)
**3 falhas — Go, Node, Python, mesmo teste conceitual.**

**O que elimina a hipótese óbvia:** ao contrário de G3, esta fixture usa `helperWriteJSON` (que
serializa de verdade) — **não há JSON inválido aqui**:
```go
// internal/generators/git_branch_guard_dedup_test.go:222
rawStoredCommand := home + "//" + ".trackfw/scripts/trackfw-git-branch-guard.sh"
```
O teste verifica que a lógica de dedup tolera uma barra dupla (`//`) na forma POSIX do comando
armazenado. Em Windows, `home` já é nativo (`C:\Users\...`), então o valor final mistura separador
nativo com barra dupla POSIX explícita — mas eu não consegui, sem rodar em Windows, determinar SE
e ONDE a função de comparação de dedup trata isso de forma diferente do esperado. **Não apresento
hipótese como causa.**

**Lista — Go (1):** `TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand`.
**Node (1, mesmo propósito, não lido):** `not ok 443` — "injectClaudeHooks skips project-scope
git-branch-guard despite // formatting in stored global command". **Python (1, mesmo propósito, não
lido):** `test_git_branch_guard_dedup.py::test_claude_tolerates_double_slash_in_stored_command`.

**O que fecharia isto:** instrumentar a função de dedup (`internal/generators/agentfiles.go`, a
mesma família de `hookArrayHasCommand`) em Windows real com este valor exato.

---

### G13 — DESCONHECIDO. Scripts de attention (bash + fallback python3 sem jq) produzem mensagem/tool errados
**5 falhas — Go + Node.**

**Evidência:** `scaffold_test.go:430-433`: `Tool esperada 'fallback_tool', obteve ""` e `Message
esperada '...', obteve 'Agent needs attention'` — o script rodou (não deu erro de execução) mas
devolveu o payload **default/genérico** em vez de interpretar o JSON de entrada. Candidato de
família: os `.sh` embarcados também podem estar sujeitos a CRLF (G1) ou a uma diferença de
parsing do fallback python3 dentro do shell script — **não verificado**, não apresento como causa.

**Lista — Go (1):** `TestAttentionScripts_FallbackWithoutJQ`. **Node (4):** `not ok 400, 401, 402,
403` (mesmo arquivo de scripts, `generators.test.js`).

---

### G7 — HIPÓTESE. `shasum`/checksum externo escapa a linha inteira quando o caminho tem `\`
**1 falha.**

**Evidência:** `Checksum() = "6f0afa...5be", shasum -a 256 = "\\6f0afa...5be"` — a saída da
ferramenta externa vem com um `\` de prefixo na linha inteira. É o comportamento **documentado**
de `sha256sum`/`shasum` GNU: quando o nome do arquivo do argumento contém um caractere que a
ferramenta precisa citar (haveria whitespace ou — como aqui — a própria convenção de escaping do
formato BSD/GNU `checksum  filename`), a ferramenta prefixa a linha inteira com `\` e escapa o nome.
**Não confirmei o mecanismo exato lendo o código do teste** (o teste chama a ferramenta do SO via
`exec.Command`, fora do controle do produto) — por isso HIPÓTESE, não causa.

**Lista — Go (1):** `TestChecksum_StableAndMatchesSHA256Sum`.

---

### G11 — DESCONHECIDO. `git.exe` resolvido como stub de "fork bomb" em teste de degradação graciosa do `ship`
**2 falhas — Go + Python, mesmo teste conceitual.**

**Evidência:** `stderr: Error: could not determine current branch (are you in a git repo?): BUG
(fork bomb): C:\Users\runneradmin\...\bin\git.exe`. O teste monta um `PATH` contendo só um `git`
funcional para simular ausência de forge CLI; a mensagem sugere que **algum stub de proteção contra
recursão** (`git.exe` que se auto-invoca — provavelmente um wrapper de teste que existe para
detectar chamadas indevidas em cadeia) foi acionado. Não tive tempo de ler o fixture que cria esse
stub nesta sessão. **Candidato de família com o Grupo B (bash resolvido para o stub errado no
Windows), mas para `git` em vez de `bash` — não confirmado.**

**Lista — Go (1):** `TestShip_Integration_GracefulDegradation_RealBinary`. **Python (1):**
`test_ship_integration_graceful_degradation_clean_path`.

---

### Mecanismo desconhecido — residual sem hipótese sustentável (10 falhas)

Nenhuma hipótese é apresentada para os itens abaixo — só a evidência bruta que tenho.

- **Go — `TestUpdateMigratesKnownCodexAndPreservesUnknown`:** manifesto Codex mostra só
  `trackfw-backend.toml` registrado; a asserção completa que compara contra o esperado não coube na
  janela de log que li. Caminhos no manifesto usam separador nativo (esperado e correto — é chave
  de filesystem real), então não é candidato ao G4/G8.
- **Node — `not ok 22` ("Go manifest fixture is interoperable for inspect, update and uninstall")** —
  não localizado no tempo desta sessão.
- **Node — `not ok 78`, RE-LOCALIZADO (não é mais "YAML malformado" genérico).** Toda a sequência de
  subtestes de `validator.test.js` passa (✓), inclusive o controle POSIX do G5 acima e as 5
  variantes de `thirdparty_artifact_has_provenance` branch ii — a ÚLTIMA linha antes da falha é
  `✓ thirdparty_artifact_has_provenance: branch ii — apagar quarentena não impede detecção de
  adulteração`. Logo depois, sem nenhum `✓`/`✗` de subteste entre os dois: `# trackfw: erro ao
  carregar "trackfw.yaml": YAML malformado` seguido do `not ok 78` do ARQUIVO inteiro. Isso localiza
  a falha num teste que roda LOGO APÓS os branch-ii de proveniência e que aparentemente lança uma
  exceção não capturada pelo `test()` que a contém (o runner do Node reporta falha do arquivo, não
  de um `not ok` individual, quando isso acontece) — candidato a ser outra instância do G3 (fixture
  que grava YAML com caminho nativo cru, sem escapar) mas para `trackfw.yaml` em vez de um JSON de
  hook. **Não confirmei qual teste é nem li seu código-fonte — falta abrir `validator.test.js` no
  trecho logo após os testes de `thirdparty_artifact_has_provenance` branch ii.**
- **Python — `test_update_alias_converts_only_present_codex_artifacts`,
  `test_status_uses_real_handler`** (crash `TypeError: argument of type 'NoneType' is not
  iterable` — `result.stdout` veio `None`, sugerindo o subprocesso não capturou saída, mecanismo não
  investigado), **`test_parse_log_linhas_validas`** (docstring do próprio arquivo de teste chega
  corrompida — `transi\uFFFD\uFFFDes` — indício de problema de encoding no checkout ou no
  carregamento do arquivo de teste, não a lógica testada; não investigado a fundo),
  **`test_agents_install_with_existing_identity_and_no_flag_does_not_invoke`** e as outras 2
  variantes de `test_identity_wizard.py`, **`test_targets_flag_with_tty_and_no_scope_still_triggers_scope_resolver`**,
  **`test_install_global_scope_requires_its_own_confirmation`** — nenhuma tinha assertion de falha
  visível na janela de log que consultei (só o código-fonte do teste, cortado antes da falha real).
- **Python — `test_validator.py::TestResolveWipDirs` (2 variantes):** `Lists differ:
  ['.../docs/roadmaps/apolo/wip', ...] != ['.../docs\\roadmaps\\apolo\\wip', ...]` — mistura clara
  de separador, mas não determinei qual lado (produção ou fixture) está "certo" para este caso
  específico sem ler `pypi/trackfw/validator.py` (não lido nesta sessão). Pode ser um residual do
  ADR de separador (G4-adjacente) ou um caso legítimo de path-de-filesystem que não deveria ter sido
  normalizado — fica como desconhecido até essa leitura.

## 4. G0 — `TestPathIsAnchoredForHookConfig_ControlePOSIX` — reconfirmado, não é regressão, é só Go

Confirmo a leitura da Wave 3: este teste afirma
`pathIsAnchoredForHookConfig(x) == filepath.IsAbs(x)` para o corpus POSIX — exatamente a
divergência que a ADR-2026-09-04 determina que exista no Windows. **É defeito de teste (expectativa
derivada do comportamento antigo), não regressão de produto.** É 1 dos 45 do Go (rotulado **G0** na
tabela da seção 7, separado dos 14 grupos de mecanismo porque seu "fechamento" não é uma correção de
causa raiz, é reescrever o corpus do teste para não depender de `filepath.IsAbs`.

A seção 5 mostra POR QUE este defeito é exclusivo do Go: `path.win32.isAbsolute`/`ntpath.isabs`
(nas versões 3.10/3.12 da CI) já concordam com o predicado novo para o mesmo corpus — não há
`TestPathIsAnchoredForHookConfig_ControlePOSIX` equivalente falhando em Node/Python porque, nesses
dois runtimes, `before == after` continua verdadeiro depois da correção.

## 5. Controle POSIX (Node/Python) para a mesma armadilha — RESPONDIDO, não é lacuna

A primeira busca por nome de teste (`*_ControlePOSIX`) não achou nada porque os nomes são
diferentes por convenção de cada runtime. Buscando por **assinatura de asserção**
(`isAbsolute`/`isabs`) em vez de nome, os dois testes-irmãos existem e são byte-a-byte o mesmo
desenho do Go:

```js
// npm/tests/validator.test.js:963-976
test('pathIsAnchoredForHookConfig: controle POSIX — idêntico a path.isAbsolute para o conjunto existente', () => {
  const before = path.isAbsolute(raw)   // path.win32 no Windows
  const after = validator.pathIsAnchoredForHookConfig(raw)
  assert.strictEqual(before, after, ...)
```
```python
# pypi/tests/test_validator.py:1578-1591
def test_path_is_anchored_for_hook_config_controle_posix(self):
    before = os.path.isabs(raw)   # ntpath no Windows
    after = v._path_is_anchored_for_hook_config(raw)
    self.assertEqual(before, after, ...)
```

**A pergunta certa não é "o teste existe", é "o `before` no Windows concorda com `after`?" — medida
localmente, com os interpretadores reais:**

```
$ node -e "console.log(require('path').win32.isAbsolute('/opt/foo/guard.sh'))"
true
$ python3.12 -c "import ntpath; print(ntpath.isabs('/opt/foo/guard.sh'))"
True
```

**Node: `path.win32.isAbsolute('/opt/foo/guard.sh')` é `true`.** Ao contrário do `filepath.IsAbs` do
Go, o Node já trata uma barra POSIX isolada como absoluta no `win32`. `before == after == true` nos
dois lados — o controle **não reproduz** o afrouxamento que o Go tinha, e a evidência do próprio log
confirma: a linha do run mostra **`✓ pathIsAnchoredForHookConfig: controle POSIX — idêntico a
path.isAbsolute para o conjunto existente`** (passou), a poucas linhas do `not ok 78` deste mesmo
arquivo — não é a mesma falha, é um teste vizinho que passou de verdade.

**Python: `ntpath.isabs('/opt/foo/guard.sh')` — 🔴 sensível à versão, medido nas duas.** Em
**Python 3.14.7** (meu interpretador padrão), retorna `False` — reproduziria o MESMO afrouxamento
do Go. Mas medido em **Python 3.12.14** (uma das duas versões reais da matriz de CI, `python
(3.12)`/`python (3.10)` no `windows-full-suites`), retorna **`True`** — o comportamento de
`ntpath.isabs` para uma barra isolada sem letra de unidade mudou entre versões do CPython, e a CI
roda a versão onde ainda é `True`. Isso explica por que **este teste Python não está entre as 21
falhas**: na versão que a CI usa, `before == after == true`, o controle passa.

**Conclusão, não hipótese:** a assimetria de `IsAbs` para uma barra POSIX isolada é **específica do
Go** — `filepath.IsAbs` nunca trata `/foo` como absoluto no Windows, mas tanto o `path.win32` do
Node quanto o `ntpath` do Python (nas versões 3.10/3.12 da matriz) tratam. Os controles de Node e
Python não estão "não verificados"; estão **corretamente verdes**, por uma razão de linguagem, não
por acidente de teste. Só o Go precisou da correção da Wave 3 e só o Go carrega o teste de controle
que virou "defeito de teste" pós-correção (G0, abaixo).

## 6. Premissas do handoff que a minha medição confirmou ou derrubou

- **Contagem 45/34/21/100:** **CONFIRMADA**, medida por mim de forma independente, mesmos comandos
  nas duas pontas.
- **"Escape/aspas" é maior do que 1 teste:** **CONFIRMADO E EXPANDIDO** — é uma família de pelo
  menos 3 mecanismos de escaping distintos (G2 `%q`, G4 `encoding/json`, G3 JSON inválido por
  concatenação crua), não um só, e cada um fecha com um patch diferente. O handoff media "vários
  outros" sem número; agora há número e discriminante para cada sub-família.
- **CRLF ~14:** **DERRUBADA — o número real é 29** (13 Go + 16 Node), quase o dobro da estimativa
  herdada, e ainda existe um 30º membro (G1-bis) num parser diferente que a ADR como escrita não
  cobre.
- **`api_chain.js:145` indexação por basename — medido, não só "não investigado":** varri os
  títulos das 34 falhas de Node por `serve|chain|board` (case-insensitive) e **nenhuma bate** (o
  único hit textual, `TestUpdateMigratesKnownCodexAndPreservesUnknown`, é falso-positivo de
  substring — "Preserves" contém "serve"). **O defeito de indexação por basename não está coberto
  por nenhum teste hoje — não é que ele não apareça no resíduo por já estar fechado, é que nenhum
  teste o vigia.** Continua precisando de REQ própria, e essa REQ precisa **criar** o teste antes de
  poder fechar qualquer coisa — não vai "cair de graça" numa wave de correção deste resíduo.
- **`TestPathIsAnchoredForHookConfig_ControlePOSIX` é defeito de teste, não regressão:**
  **CONFIRMADO** por leitura do próprio corpo do teste.
- **Nona/décima premissas anteriores (fixture de proveniência, `_tildeify` triplamente divergente):**
  não relitigadas; fora do escopo desta re-triagem (já fechadas em waves anteriores).

## 7. Ordenação por retorno, para sequenciar as próximas waves

Tabela reconciliada — as três colunas de contagem somam exatamente 45/34/21/100 (nenhuma célula
"a determinar" escondida fora da soma; a linha final absorve tudo que não tem grupo).

| Grupo | Go | Node | Py | Total | Confiança | Tipo de correção | Risco |
|---|---|---|---|---|---|---|---|
| G4 — `includes`/`Contains`/`in` cru vs JSON escapado | 8 | 7 | 7 | **22** | **confirmado nos 3 runtimes** | só teste | nenhum |
| G1 — CRLF no parser de frontmatter | 13 | 16 | 0 | **29** | confirmado | produto, ADR já Accepted | baixo (D1-D4 escritos) |
| G3 — fixture gera JSON inválido (fail-open silencioso) | 5 | 4 (hip.) | 0 | **9** | confirmado (Go) / hipótese (Node) | só teste | nenhum |
| G13 — scripts de attention (bash/python3 fallback) | 1 | 4 | 0 | **5** | desconhecido | a determinar | a determinar |
| G2 — `%q` do Go dobra a barra | 4 | 0 | 0 | **4** | confirmado | só teste (ou UX de mensagem) | nenhum |
| G9 — ENOTDIR silencioso no Windows | 3 | 0 | 0 | **3** | confirmado | produto | médio (diagnóstico que desaparece) |
| G12 — dedup `//` + home nativo | 1 | 1 | 1 | **3** | desconhecido | a determinar | a determinar |
| G5 — bits de modo restritivo (NTFS) | 1 | 0 | 1 (hip.) | **2** | confirmado (Go) | decisão de arquitetura | nenhum se só o teste mudar |
| G8 — `findRoadmap` separador nativo vs literal POSIX | 2 | 0 | 0 | **2** | confirmado | só teste | nenhum |
| G10 — chave de proveniência pós-install (short/long name) | 2 | 0 | 0 | **2** | hipótese | produto, se confirmado | médio — toca aprovação de terceiros |
| G11 — `git.exe` stub "fork bomb" | 1 | 0 | 1 | **2** | desconhecido | a determinar | a determinar |
| G1-bis — CRLF cega o parser de gates do `barrier` | 0 | 0 | 1 | **1** | confirmado (sintoma), site não localizado | produto (parser distinto do G1) | baixo |
| G0 — controle POSIX vira defeito de teste após a Wave 3 | 1 | 0 | 0 | **1** | confirmado, **não é regressão** | só teste | nenhum |
| G7 — checksum externo escapa a linha | 1 | 0 | 0 | **1** | hipótese | só teste, provavelmente | nenhum |
| G6 — traversal POSIX no guard (`filepath.IsAbs`, D2) | 1 | 0 | 0 | **1** | confirmado, **fora de escopo por decisão D2** | nenhuma (não corrigir) | — |
| Desconhecido residual, sem hipótese sustentável | 1 | 2 | 10 | **13** | nenhuma hipótese | a determinar | a determinar |
| **Total** | **45** | **34** | **21** | **100** | | | |

🔴 **Correção sobre minha primeira versão desta tabela:** o total de "desconhecido residual" era
**10**, não **13** — eu tinha somado errado a coluna Python (a lista da seção 3 já enumerava 10
itens Python: `test_update_alias_converts_only_present_codex_artifacts`,
`test_status_uses_real_handler`, `test_parse_log_linhas_validas`, as 3 variantes de
`test_identity_wizard.py`, `test_targets_flag_with_tty_and_no_scope_still_triggers_scope_resolver`,
`test_install_global_scope_requires_its_own_confirmation`, e as 2 variantes de
`TestResolveWipDirs`). A tabela original também não tinha linha para G0 nem para G1-bis, o que
deixava a soma em ~96 em vez de 100. Encontrado ao reconciliar a tabela contra as listas completas
da seção 3, não pela auditoria externa sozinha — mas só depois que ela apontou que a soma não
fechava.

**Leitura para sequenciamento:**

1. **G4 (22, confirmado nos 3 runtimes)** é o candidato certo para o primeiro ML da próxima wave:
   maior retorno, risco zero, e — ao contrário do precedente do `IsAbs` (estimado ~14 por sintoma,
   entregou 2) — aqui as 22 foram lidas uma a uma no código-fonte de teste dos 3 CLIs, não inferidas
   por nome.
2. **G1 (29 + 1 G1-bis)** é o maior grupo isolado e já tem ADR `Accepted` — falta só o código (Wave
   5), mas **G1-bis precisa de patch adicional** no parser de gates do `barrier` (Python), que a ADR
   como escrita não cobre.
3. **G3 (9, confirmado em Go)** é o segundo melhor retorno por ser só-teste; a metade Node é
   hipótese e precisa da mesma leitura de código que fechou G4 antes de entrar no plano com número
   firme.
4. **G9 (3)** é o único grupo com peso de observabilidade/segurança (diagnóstico que desaparece em
   silêncio) — não é o maior, mas é o que mais importa se alguém depender da mensagem para agir.
5. **G10 (2, hipótese)** é o único candidato remanescente que mexe em lógica de aprovação de
   terceiros — se confirmado, deve receber revisão do `hades-tf` antes de qualquer patch, pelo
   precedente já estabelecido nesta REQ para as Waves 2 e 3.
6. **G13, G12, G11, G7 e o residual desconhecido (13) somam 24 falhas sem mecanismo sustentável.**
   Isso é quase um quarto do resíduo total ainda sem diagnóstico — maior que qualquer grupo
   individual confirmado exceto G1 e G4. Não recomendo estimar retorno para eles; recomendo uma
   wave de investigação dedicada (nos moldes do antigo "Grupo B") antes de comprometer código a
   qualquer um.
