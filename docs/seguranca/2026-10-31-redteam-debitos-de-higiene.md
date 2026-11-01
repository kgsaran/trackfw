# Red Team — Débitos de higiene (ML-2A)

**Roadmap:** ROADMAP-2026-09-22-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md
**Commit auditado:** `72aac048` ("fix: release tag confere o package-lock, scripts e hooks do husky recebem o bit de execução, aviso de agent_models sai uma vez")
**Data:** 2026-10-31 | **Revisor:** hades-tf

---

## Metodologia

Reimplementação a partir da leitura dos arquivos, não do diff. Achados verificados com:
- mutações aplicadas via `python3` e revertidas (confirmadas por `git diff --exit-code -- internal/ scripts/`);
- testes `zz_*` em `internal/discover/` para medir o comportamento real das funções (não do diff); removidos ao fim;
- testes `zz_*` em `internal/commands/` para a shape matrix dos extratores do lockfile (funções reais, não cópia); removidos ao fim;
- binário HEAD compilado em scratchpad isolado da árvore;
- `env -i HOME=<tmpdir>` para isolar o HOME em medições de AC3.

Verificação de ausência de artefatos temporários: `git diff --exit-code -- internal/ scripts/` vazio; nenhum arquivo `zz_*` remanescente.

---

## Achados

### A-1 (LOW) — `cli-parity.md` lista `pypi/trackfw/__init__.py` como arquivo verificado e diz "5 version files / 7 checks"

**Introduzido por:** 72aac048

**Evidência:**

```
grep -n 'version files\|5 checks\|7 checks\|4 files' docs/cli-parity.md
→ linha 1675: "The 5 version files disagree"
→ linha 1679: "7 checks in total (AC1, REQ-2026-08-30)"
→ linha 1781: "version files (the 5 checks in P3 above)"
```

`cli-parity.md:1676–1679` lista `pypi/trackfw/__init__.py` (dois subchecks) como um dos cinco arquivos. Esse arquivo foi excluído de `releaseVersionFiles` por decision anterior (ML-1A, `TestReleaseVersionFiles_NoWave3DeletedPaths` confirma). O estado corrente de `releaseVersionFiles` (verificado em `release.go:120–126`) é **4 arquivos distintos, 5 verificações**:

| Entrada | Arquivo | Checks |
|---|---|---|
| `internal/version/version.go` | version.go | 1 |
| `npm/package.json` | package.json | 1 |
| `pypi/pyproject.toml` | pyproject.toml | 1 |
| `npm/package-lock.json (root version)` | package-lock.json | 1 |
| `npm/package-lock.json (packages[""].version)` | package-lock.json | 1 |
| **Total** | 4 arquivos distintos | **5 checks** |

Inconsistência com os outros sítios de documentação:

| Sítio | O que diz | Correto? |
|---|---|---|
| `release.go` help text (linha 228) | "The 4 version files must all match" | Sim |
| `release.go` comentário (linha 101) | "All 5 checks (4 files; npm/package-lock.json contributes 2)" | Sim |
| `docs/cli-parity.md:1675` | "The 5 version files disagree" | **Não** (4 arquivos) |
| `docs/cli-parity.md:1679` | "7 checks in total" | **Não** (5 checks) |
| `docs/cli-parity.md:1781` | "the 5 checks in P3 above" | Sim (5 checks é correto) |
| `scripts/check-release-tag-parity.sh` comentários (linhas 628, 1511, 1513) | "5 version files" | Desatualizado (comentário de fixture, não asserção funcional) |

Nota: linha 1781 está correta porque o count de 5 *checks* é o correto: versão atual é version.go(1) + package.json(1) + pyproject.toml(1) + lockfile raiz(1) + lockfile packages[""] (1) = 5 checks em 4 arquivos. O doc foi escrito como se `pypi/__init__.py` ainda estivesse na lista (+2 checks) junto com o lockfile novo (+2 checks), somando 7 — mas `__init__.py` já tinha sido removido antes de 72aac048 (confirmado por `TestReleaseVersionFiles_NoWave3DeletedPaths`). A inconsistência está nas linhas 1675 (conta arquivos) e 1679 (conta checks), e na listagem textual que ainda menciona `pypi/trackfw/__init__.py`. O roadmap ML-1A também cita "5 arquivos / 7 conferências" no registro de auditoria do arquiteto.

**Correção sugerida:** em `cli-parity.md` linha 1675, substituir "The 5 version files" por "The 4 version files". Na linha 1679, substituir "7 checks in total" por "5 checks in total". Remover a listagem de `pypi/trackfw/__init__.py` e seus dois subchecks. Atualizar comentários em `scripts/check-release-tag-parity.sh` (linhas 628, 1511, 1513).

---

### A-2 (LOW) — Extratores do lockfile herdam o differential de case-insensitive struct tags pré-existente

**Introduzido por:** 72aac048 (propaga padrão pré-existente de `extractNpmVersion`)

**Evidência:**

```go
// scratchpad, não na árvore:
input: {"version":"0.0.1","Version":"9.9.9"}
Go reads: "9.9.9"   ← struct tag json:"version" é case-insensitive; último match vence
npm reads: "0.0.1"  ← JSON.parse é case-sensitive
```

Go's `encoding/json` faz correspondência de tag de campo **insensível a maiúsculas**; com chaves duplicadas de caso diferente, o último valor vence. Vetor: lockfile editado com `"version":"errada","Version":"certa"` — `release tag` aceitaria a versão "certa"; o npm real leria a "errada".

**Mitigação do contexto:** `npm version` nunca gera chaves com V maiúsculo; requer adulteração manual. O padrão era pré-existente em `extractNpmVersion` antes deste commit.

**Correção sugerida (se adotada):** trocar struct unmarshal por decode em `map[string]json.RawMessage` e indexar pela chave exata `"version"` — a lookup de map em Go é case-sensitive, sem dependências externas. Ou declarar como residual aceito (vetor requer adulteração manual e não é gerado por `npm version`).

---

### A-3 (LOW) — Teste negativo de AC3 checa `GlobalAgentModelsNoneMessage` em vez de `errBuf.Len()==0`

**Introduzido por:** 72aac048 (novo teste)

**Evidência — código:**

```go
// update_test.go:2925–2928
if strings.Contains(got, config.GlobalAgentModelsNoneMessage) {
    t.Errorf("advisory must not be emitted for non-catalog targets, but got:\n%s", got)
}
```

**Evidência — Cenário C (caminho de falso-negativo concreto):**

```bash
# cwd tem trackfw.yaml com agent_models; HOME vazio
$ env -i HOME="$EMPTY_HOME" ... trackfw update harness --targets codex-agents 2>&1 1>/dev/null
trackfw: agents global: agent_models configurado em trackfw.yaml do projeto mas não vale...
```

`ResolveAgentModels` tem três mensagens possíveis: `GlobalAgentModelsNoneMessage`, `GlobalAgentModelsProjectOnlyMessage`, `MalformedGlobalConfigMessage`. O teste negativo atual verifica apenas a primeira. Se uma regressão reintroduzir a emissão do advisory para destinos não-catálogo num ambiente Cenário C (cwd com `agent_models`, HOME vazio), o teste passaria silenciosamente.

**Correção sugerida:** substituir o `strings.Contains` por `errBuf.Len() == 0` — captura qualquer mensagem independente do tipo.

---

### A-4 (LOW) — M-8b compila e os dois testes de AC3 passam com `nil` passado a `harnessCatalogTarget`

**Introduzido por:** 72aac048 (novo parâmetro `agentModels`, mas sem teste que verifique que o valor chega aos arquivos gerados)

**Evidência:**

Mutação M-8b: `harnessCatalogTarget(…, nil, opts)` + `_ = harnessAgentModels` para suprimir erro de compilação. Resultado com suite completa dos dois pacotes:
```
go test ./internal/generators/ ./internal/commands/ -count=1
ok  github.com/kgsaran/trackfw/internal/generators  69s
ok  github.com/kgsaran/trackfw/internal/commands    63s
```

Os dois testes de AC3 verificam apenas o stderr — se o advisory foi emitido. Não verificam que `agentModels` chega aos arquivos gerados (ex.: que `model: "claude-sonnet-4-6"` aparece no `.claude/agents/*.md` gerado). Com `nil`, `harnessCatalogTarget` usa a lógica de tier canônico sem erro — os arquivos são gerados com um modelo, não com o pin configurado.

A medição end-to-end (Scenario D, config válida) mostrou que o comportamento correto funciona hoje: conteúdo dos arquivos é idêntico entre pré-fix e HEAD com config válida. A fragilidade é de robustez de teste, não de comportamento atual.

**Correção sugerida:** adicionar um subtest ao teste existente que verifique que o arquivo de agente gerado contém o tier correto quando `agent_models` está configurado (ou que `harnessCatalogTarget` é chamado com map não-nil quando hasCatalogTarget é true).

---

## AC1 — Shape matrix dos extratores do lockfile

Shapes testados nas funções reais `extractPackageLockRootVersion` e `extractPackageLockPackagesVersion` via `TestZZ_LockfileShapes` em `internal/commands/` (removido após medição). 22 sub-testes, todos PASS. Confirmações especiais: (a) `packages[""] = null` → `json.RawMessage("null")` → `json.Unmarshal(null, &pkg)` retorna nil, `pkg.Version = ""` → fail-closed (sem pânico); (b) `packages[""] = {}` → pkg.Version = "" → fail-closed. Sem divergência entre funções reais e modelo de análise.

| Shape | Extrator | Resultado | Comportamento |
|---|---|---|---|
| `{"version":1}` (numérico) | root | erro: cannot unmarshal number | fail-closed |
| `{"version":null}` | root | erro: no version field | fail-closed (null→""→empty check) |
| `{"version":""}` | root | erro: no version field | fail-closed |
| BOM + JSON | root | erro: invalid character | fail-closed |
| `{"version":"1.0.0","version":"2.0.0"}` (dup mesmo caso) | root | aceita "2.0.0" | last-wins (mesmo comportamento do npm para mesmo caso) |
| `{"packages":[]}` | packages | erro: cannot unmarshal array | fail-closed |
| `{"packages":{"other":{...}}}` (sem chave `""`) | packages | erro: packages[""] missing | fail-closed |
| `{"packages":{"":""}}` | packages | erro: cannot unmarshal string | fail-closed |
| `{"packages":{"":null}}` | packages | erro: version empty | fail-closed |
| `{"packages":{"":{} }}` | packages | erro: version empty | fail-closed |
| BOM + JSON com packages | packages | erro: invalid character | fail-closed |
| dup mesmo caso em `version` | packages | aceita "2.0.0" | last-wins (ambos Go e npm) |

Único caso fail-open identificado: **duplicate keys de mesmo caso** — Go e npm ambos tomam o último valor, comportamento idêntico. A-2 (case misto) é a diferença real.

---

## Mutações aplicadas e resultado

Todas as mutações foram aplicadas, testadas e revertidas. `git diff --exit-code -- internal/ scripts/` confirmou a reversão em cada caso.

| Mutação | Arquivo | Teste | Resultado |
|---|---|---|---|
| M-1: `extractPackageLockPackagesVersion` faz fallback ao root quando `packages[""]` ausente | `release.go` | `lockfile-no-packages-entry` | **FAIL — capturada** |
| M-2: labels trocados entre root e packages[""] em `releaseVersionFiles` | `release.go` | `lockfile-root`, `lockfile-packages`, `lockfile-no-packages-entry` | **FAIL — capturada** |
| M-3: `extractPackageLockPackagesVersion` retorna valor do root | `release.go` | `lockfile-packages` | **FAIL — capturada** |
| M-4: `os.Chmod` removido de `writeValidateScript` | `discover.go` | `TestInstallGates_ValidateScript_ExecBitPromotedWhenFileExistsWith0644` | **FAIL — capturada** |
| M-5: `os.Chmod` removido de `generateCommitMsgHook` | `scaffold.go` | `TestGenerateCommitMsgHook_HuskyExecBitPromotedWhenFileExistsWith0644` | **FAIL — capturada** |
| M-6: `os.Chmod` removido de `generateHuskyHook` | `scaffold.go` | `TestGenerateHuskyHook_ExecBitPromotedWhenFileExistsWith0644` | **FAIL — capturada** |
| M-7: `harnessCatalogTarget` chama `ResolveAgentModels` localmente | `update.go` | `TestUpdateHarness_AgentModelsAdvisory_EmittedOnceForMultipleCatalogTargets` | **FAIL — capturada** |
| M-8a: `nil` passado diretamente sem supressor | `update.go` | — | **Não compila** (`declared and not used`) |
| M-8b: `nil` passado + `_ = harnessAgentModels` (supressor) | `update.go` | ambos testes de AC3 | **PASS — lacuna de teste** (ver A-4) |

**M-8b** é o caso que importa para segurança: a mutação compila e os testes de advisory passam, porque os testes verificam apenas o stderr. Nenhum teste verifica que o `agentModels` correto chega aos arquivos gerados.

---

## Veredito por prescrição (Wave 0)

### P-1 — AC1: `release tag` confere o `package-lock` (dois campos independentes)

**Veredito: APROVADO** com ressalvas documentais (A-1).

- Real lockfile `npm/package-lock.json` aceito por ambos os extratores sem falso positivo.
- Três mutações de extrator capturadas (M-1, M-2, M-3).
- Shape matrix: todas as 12 shapes fail-closed confirmadas; único fail-open é duplicate keys de mesmo caso (comportamento idêntico ao npm).
- A-1: `cli-parity.md` diz "5 version files / 7 checks" mas o código tem 4 arquivos / 5 verificações. Não bloqueia a AC1 funcional.
- A-2: differential de case-insensitive struct tags. LOW; requer adulteração manual.

### P-2 — AC2: `os.Chmod` em `writeValidateScript` e nos hooks do husky

**Veredito: APROVADO** com residuais pré-existentes confirmados por medição na função real.

- Medido: `os.WriteFile(0755)` sob `umask 0111` produz modo `0644`; `writeValidateScript` com o fix produz `0755`.
- Leaf guards presentes em todos os três sítios novos (via `pathguard.RejectSymlinks`).

**Hardlink (R-4) — medido via `TestZZ_WriteValidateScript_HardlinkFollowsInode` (`internal/discover/`):**

```
before: otherFile mode=-rw------- dest mode=-rw-------  (dest é hardlink para otherFile)
após InstallGates:
  otherFile mode=-rwxr-xr-x content_len=55
RESULT: hardlink content OVERWRITTEN — validate script written through hardlink to otherFile
RESULT: otherFile mode promoted to -rwxr-xr-x via Chmod through hardlink
```

`pathguard.RejectSymlinks` usa `os.Lstat` e verifica `os.ModeSymlink|os.ModeIrregular` — hardlinks não têm nenhum desses bits, o guard **passa**. `os.WriteFile` então grava no inode compartilhado; `os.Chmod(0755)` eleva o modo no mesmo inode. Ambas as operações afetam o arquivo-alvo do hardlink.

**Symlink TOCTOU — medido via `TestZZ_WriteValidateScript_SymlinkRace` (`internal/discover/`, 5.000 iterações com racer):**

```
iterations=5000
WriteFile-through-symlink(TOCTOU)=1912  (38%; atribuível ao WriteFile de InstallGates — guard passa, racer planta symlink no TOCTOU window)
Chmod-through-symlink(TOCTOU)=171       (3.4%; exclusivamente do Chmod de InstallGates — racer não faz Chmod)
```

O `RejectSymlinks` guard recusa o symlink quando ele já está em dest no momento da verificação. O race win ocorre no TOCTOU: guard vê arquivo regular, racer planta symlink, então WriteFile ou Chmod segue o symlink. O racer faz `Remove(dest)` antes de cada `WriteFile(dest,"racer")` — portanto dest nunca é symlink quando o racer escreve; os 1912 são inteiramente do WriteFile de InstallGates. O `Chmod-through-symlink=171/5000` (3.4%) é atribuível exclusivamente ao `os.Chmod` adicionado por 72aac048 — confirma a adição de risco de promoção de modo.

**Não fechados por este commit (escopo negativo da REQ):**
- `scaffold.go:2004` (lefthook `runner: sh`) e `update.go:267` (O_APPEND hooks) — sem `Chmod`; fora do escopo por decisão do arquiteto.
- Windows: `os.Chmod` retorna nil sem efeito (sem bit de execução POSIX). Testes pulam.

### P-3 — AC3: Advisory de `agent_models` emitido uma vez por execução

**Veredito: APROVADO** com ressalva de robustez de teste (A-3, A-4).

**Cenários medidos com binário HEAD (`go build ./cmd/trackfw`, `env -i HOME=<tmpdir>`):**

| Cenário | Config | Alvo | Stderr | Esperado |
|---|---|---|---|---|
| A — sem config global | HOME vazio | `codex-agents` (não instalado) | 1 linha (NoneMessage) | 1 ✓ |
| C — projeto tem `agent_models`, HOME vazio | trackfw.yaml com `agent_models` | `codex-agents` | 1 linha (ProjectOnlyMessage) | 1 ✓ |
| D — config global válida | `~/.trackfw/trackfw.yaml` com `agent_models` | `codex-agents` | 0 linhas | 0 ✓ |
| F — YAML global malformado | `~/.trackfw/trackfw.yaml` malformado | `codex-agents` | 1 linha (MalformedGlobalConfigMessage) | 1 ✓ |
| não-catálogo | HOME vazio | `credential-guard-script` | 0 linhas | 0 ✓ |
| misto | HOME vazio | `codex-agents,credential-guard-script` | 1 linha | 1 ✓ |
| duplicado | HOME vazio | `codex-agents,codex-agents` | 1 linha | 1 ✓ |

Cenário A confirma R-5 original: advisory é emitido mesmo quando o target de catálogo está ausente (não instalado) — comportamento correto e decidido pelo arquiteto.

**Caller scan:** `UpdateHarness` tem exatamente um caller de produção (`commands/update_harness.go:87`); `ResolveAgentModels` em outros paths (`integrations_flags.go`, `init.go`, `thirdparty.go`) são caminhos de execução disjuntos. Sem double-call.

**M-7 (reintroduz bug):** capturada — teste vê 3 linhas em vez de 1.
**M-8a (nil sem supressor):** não compila.
**M-8b (nil + supressor):** ambos os testes AC3 PASSAM — lacuna de teste (A-4).

---

## Residual declarado

| ID | Descrição | Severidade | Mudança em relação à Wave 0 |
|---|---|---|---|
| R-1 | Scripts harness `~/.trackfw/scripts/*.sh` sem `os.Chmod` (scaffold.go 1244/1622, update.go 877/916) | BAIXO | Inalterado |
| R-2 | husky commit-msg/pre-commit (scaffold.go 1974/2042) — **corrigido** em 72aac048; lefthook (scaffold.go:2004) e O_APPEND (update.go:267) fora do escopo da REQ | BAIXO (escopo negativo) | Parcialmente fechado |
| R-3 | `discover.go` 179/293/336: O_APPEND hooks sem Chmod | BAIXO | Inalterado |
| R-4 | Hardlink em `scripts/trackfw-validate.sh` ou `.husky/*`: **medido via função real** — guard não detecta hardlinks (sem verificação de Nlink), WriteFile sobrescreve inode, Chmod(0755) eleva modo do inode para 0755. | BAIXO | Medido com função real; Chmod adiciona mode-promotion |
| R-5 | Advisory emitido mesmo quando target de catálogo está ausente (não instalado) — Cenário A: 1 linha com HOME vazio e codex-agents não instalado. Comportamento correto (decisão do arquiteto: avisar quando agent_models não está configurado globalmente, independente do target estar instalado). | DECIDIDO | Confirmado por medição (Cenário A) |
| TOCTOU | Janela entre WriteFile e Chmod: Chmod segue symlink plantado no TOCTOU window. **Medido via função real:** `Chmod-through-symlink=171/5000` (3.4%) — atribuível ao Chmod de 72aac048. | BAIXO | Medido com função real |
| A-1 | `cli-parity.md` "5 files / 7 checks" — deveria ser "4 files / 5 checks"; lista `pypi/__init__.py` | BAIXO | Novo — introduzido por 72aac048 |
| A-2 | Case-insensitive struct tags nos extratores do lockfile (padrão pré-existente propagado) | BAIXO | Novo — requer adulteração manual |
| A-3 | Teste negativo de AC3 checa `GlobalAgentModelsNoneMessage` em vez de `errBuf.Len()==0` | BAIXO | Novo — Scenario C é falso-negativo concreto |
| A-4 | M-8b compila e ambos os testes AC3 passam com `nil` como `agentModels` — sem teste que verifique que os pins chegam aos arquivos gerados | BAIXO | Novo |

---

## Veredito final

**APROVA COM RESSALVAS**

Nenhum achado bloqueia o merge. Os achados A-1 a A-4 são LOW. As prescrições P-1, P-2 e P-3 da Wave 0 estão implementadas com cobertura de teste discriminante para os casos diretos.

As medições P-2 (hardlink e TOCTOU) foram feitas na função real via `TestZZ_*` (não em proxy). O risco de hardlink é confirmado. O risco de TOCTOU via Chmod é real mas estreito (3.4% com racer ativo). O `make quality` não foi executado nesta sessão — responsabilidade da barreira de CI e do arquiteto na auditoria.

**Ações recomendadas antes do próximo milestone:**
1. Corrigir `cli-parity.md` para "4 version files / 5 checks" e remover `pypi/trackfw/__init__.py` (A-1).
2. Fortalecer `TestUpdateHarness_AgentModelsAdvisory_NotEmittedForNonCatalogTargets` com `errBuf.Len()==0` (A-3).
3. Adicionar subtest que verifique que os model-pins chegam ao arquivo de agente gerado (A-4).
4. A-2 (case-insensitive JSON): declarar como residual aceito ou migrar para `map[string]json.RawMessage` com indexação case-sensitive.
5. R-4 / TOCTOU: migrar os 3 sítios de `WriteFile+Chmod` para `pathguard.GuardedWrite(dir, filepath.Join(dir, filename), content, 0755)`. `GuardedWrite` (a) chama `RejectAndReport` como guard, (b) chama `temporary.Chmod(mode)` no fd aberto (não re-resolve o nome), (c) fecha o fd e faz `os.Rename(tmp, dest)` — rename não segue symlink-folha em dest e entrega novo inode, rompendo o vínculo de hardlink. Fecha R-4 e ambas as janelas TOCTOU (WriteFile 38% e Chmod 3.4%) com uma única primitiva já presente em `internal/pathguard`.
