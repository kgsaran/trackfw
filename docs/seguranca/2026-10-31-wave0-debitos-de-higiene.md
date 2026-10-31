# Wave 0 — Threat Model: Débitos de Higiene
**Roadmap:** ROADMAP-2026-09-22-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md
**Data:** 2026-10-31 | **Revisor:** hades-tf

**Mapeamento de seções:**
| Requisito do papel (hades-tf Wave 0) | Ação do ML-0A | Seção neste documento |
|---|---|---|
| 1. Enumeração completa | Ações 1 e parte de 2 (liste) | § 1 |
| 2. Modelo de ameaça (implementador acelerado) | Transversal a AC1/AC2/AC3 | § 2 |
| 3. Alvos de falsificação em ambas as direções | Ações 2-4 (FN/FP por superfície) | § 3, § 4, § 5 (AC1/AC2/AC3) + § 6.1 |
| 4. Residual declarado | Ação 5 (aceito não cobrir) | § 6.2 |
| Prescrições numeradas | Ação 5 | § Prescrições |

---

## 1. Enumeração completa

### 1.1 Pontos que gravam script executável (modo 0755)

**Comando usado:**
```bash
grep -rnE --include='*.go' --exclude='*_test.go' 'WriteFile|OpenFile' \
  internal/ cmd/ | grep '0o*7' | grep -v 'MkdirAll\|Mkdir('
grep -rnE --include='*.go' --exclude='*_test.go' 'os\.Chmod' internal/ cmd/
```

| Arquivo | Linha | Função | Script alvo | os.Chmod? |
|---|---|---|---|---|
| `internal/generators/scaffold.go` | 1069/1077 | `generateValidateScript` | `scripts/trackfw-validate.sh` (projeto, init/update) | ✓ |
| `internal/generators/scaffold.go` | 1125/1129 | `GenerateAttentionScripts` | `scripts/trackfw-attention-signal.sh` | ✓ |
| `internal/generators/scaffold.go` | 1144/1148 | `GenerateAttentionScripts` | `scripts/trackfw-attention-cleanup.sh` | ✓ |
| `internal/generators/scaffold.go` | 1191/1195 | `GenerateCredentialGuardScript` | `scripts/trackfw-credential-guard.sh` (projeto) | ✓ |
| `internal/generators/scaffold.go` | 1569/1573 | `GenerateGitBranchGuardScript` | `scripts/trackfw-git-branch-guard.sh` (projeto) | ✓ |
| `internal/generators/scaffold.go` | 1244 | `GenerateGlobalCredentialGuardScript` | `~/.trackfw/scripts/trackfw-credential-guard.sh` | ✗ |
| `internal/generators/scaffold.go` | 1622 | `GenerateGlobalGitBranchGuardScript` | `~/.trackfw/scripts/trackfw-git-branch-guard.sh` | ✗ |
| `internal/generators/scaffold.go` | 1974 | `generateCommitMsgHook` (husky) | `.husky/commit-msg` | ✗ |
| `internal/generators/scaffold.go` | 2004 | `generateCommitMsgHook` (lefthook) | `.lefthook/commit-msg/trackfw-req-check.sh` | ✗ |
| `internal/generators/scaffold.go` | 2042 | `generateHuskyHook` | `.husky/pre-commit` | ✗ |
| `internal/generators/update.go` | 267 | `UpdateProject` (O_APPEND husky inject) | `.husky/pre-commit` | ✗ (O_APPEND, project scope) |
| `internal/generators/update.go` | 877 | `harnessGitBranchGuardScriptTarget` | `~/.trackfw/scripts/trackfw-git-branch-guard.sh` | ✗ |
| `internal/generators/update.go` | 916 | `harnessCredentialGuardScriptTarget` | `~/.trackfw/scripts/trackfw-credential-guard.sh` | ✗ |
| `internal/discover/discover.go` | 120 | `writeValidateScript` (via `InstallGates`) | `scripts/trackfw-validate.sh` | ✗ **(sujeito do AC2)** |
| `internal/discover/discover.go` | 179, 293, 336 | `installHook` (O_APPEND) | `.husky/pre-commit` (e variantes) | ✗ (O_APPEND, só cria se ausente) |

**Lista fechada.** Os cinco que já têm `os.Chmod` são os pares 1069/1077, 1125/1129, 1144/1148, 1191/1195, 1569/1573 — 5 sítios. Os demais **10 sítios** não têm `os.Chmod`: scaffold.go 1244, 1622, 1974, 2004, 2042; update.go 267, 877, 916; discover.go 120; discover.go 179/293/336 (contados como 1 sítio O_APPEND). O `update.go:267` usa `O_APPEND|O_CREATE` — cria o arquivo se ausente com modo `0755`, mas não reaplica `os.Chmod` sobre arquivo pré-existente.

**Contradição com a REQ (AC2):** A REQ diz "os outros cinco pontos de escrita já fazem isso". A contagem dos cinco está correta. Porém, existem **9 sítios sem Chmod além do sujeito do AC2** (`discover.go:120`). A REQ os excluiu via escopo negativo. Isso entra em conflito direto com a Regra Dura de Causa Raiz (mesmo mecanismo, mesma REQ). O arquiteto deve decidir se os outros sítios entram como novo ML nesta REQ ou se o escopo negativo prevalece com razão explícita (distinção de severidade). O presente threat model nomeia o conflito para que a decisão seja consciente — veja P-2 na seção de Prescrições.

### 1.2 Arquivos que portam a versão do projeto

**Comando usado:**
```bash
# Confirmação da lista em release.go
grep -n 'releaseVersionFiles\|3 files\|4 version' internal/commands/release.go

# Commit de bump como referência empírica (output abaixo)
git show --stat a485c637
```

**Saída de `git show --stat a485c637`:**
```
commit a485c637ad3fe25d33f103f6cb9623dc6976f0b0
Author: Kleber Saran <kg.saran@gmail.com>
Date:   Sat Oct 31 17:35:53 2026 -0300

    chore(release): bump version files to 11.2.3; governança do #641 fechada (#645)

 CHANGELOG.md                                           | 18 ++++++++++++++++++
 docs/.../20261031T202227Z-zeus-release-11-2-3.md       |  5 +++++
 ...                                                    |  4 ++--
 ...                                                    |  1 +
 docs/visao-projeto/VISION.md                           |  2 +-
 internal/version/version.go                            |  2 +-
 npm/package-lock.json                                  |  4 ++--
 npm/package.json                                       |  2 +-
 pypi/pyproject.toml                                    |  2 +-
 10 files changed, 36 insertions(+), 12 deletions(-)
```

`npm/package-lock.json` está presente no diff do bump commit. Confirmado: o lockfile é atualizado a cada bump. Os últimos 10 commits de bump foram verificados — todos incluem `npm/package-lock.json`.

| Arquivo | Conferido por `release tag`? |
|---|---|
| `internal/version/version.go` | ✓ (`releaseVersionFiles[0]`) |
| `npm/package.json` | ✓ (`releaseVersionFiles[1]`) |
| `pypi/pyproject.toml` | ✓ (`releaseVersionFiles[2]`) |
| `npm/package-lock.json` | ✗ (AC1 — não está na lista) |

**Lista fechada.** `check-manifest-version-gate.sh` confere os três primeiros mais CHANGELOG. Não confere `npm/package-lock.json`.

`npm/package-lock.json` está rastreado no git (`git ls-files --error-unmatch npm/package-lock.json` → exit 0). Lockfile version medida: `lockfileVersion: 3`; campo raiz `version: 11.2.3`; campo `packages[""].version: 11.2.3` (ambos alinhados com a versão corrente).

**Verificação dos últimos 10 bumps:**
```bash
for sha in a485c637 2e0f1817 45ed483e 8753441c 4a471465 \
           14d10dc3 8093aaaf 398a2716 3be3375c 090a4ebb; do
  git show --stat $sha | grep -q 'npm/package-lock.json' && echo "OK: $sha" || echo "MISSING: $sha"
done
# Todos os 10 commits retornam "OK: <sha>"
```
Resultado: 10/10 "OK". O lockfile é atualizado em todo bump desde a v10.0.0.

O comentário em `release.go:101` diz "All 3 checks (3 files)" — ficará obsoleto quando o lockfile for adicionado. A ajuda em `release.go:178` diz "4 version files" enquanto o código confere apenas 3 — estado atual inverso: help promete mais do que o código entrega.

O comando `npm version "$VERSION" --no-git-tag-version` no workflow `release.yml` (linhas 202, 245) atualiza `package.json` e `package-lock.json` automaticamente. Não é necessário passo adicional no protocolo de bump.

---

## 2. Modelo de ameaça — implementador acelerado

O adversário desta Wave 0 é o implementador acelerado que lê a REQ, a aceita literalmente e entrega
artefatos que passam nos gates sem fechar o defeito real. São três ameaças concretas, todas fundadas
em contradições medidas entre a REQ e o código.

### (a) AC1 — extrator de campo único passa sem detectar divergência no outro campo

O lockfile tem **dois** campos de versão independentes: `version` (raiz) e `packages[""].version`.
Um implementador que escreve o extrator para apenas um dos campos (o mais óbvio, `version` na raiz)
produz um teste que:

- Passa quando a raiz é correta mas `packages[""]` está errado — falso negativo vivo
- Reprova quando a raiz está errada — parece que o gate está funcionando

O teste de counter-arm ("sem a entrada nova, o teste de lockfile divergente reprova") ainda passa,
porque o counter-arm só verifica que a ENTRADA existe em `releaseVersionFiles` — não que ambos os
campos são checados. A sabotagem sobrevive ao gate.

**Falsificação:** testar com (1) raiz correta, `packages[""]` errado → deve reprovar; (2) raiz errada,
`packages[""]` correto → deve reprovar; (3) apenas raiz ausente (arquivo sem `packages[""]`) →
deve reprovar. Os três vetores devem ser casos de teste independentes.

### (b) AC2 — teste de arquivo novo passa sem o fix

`os.WriteFile("newfile", content, 0755)` cria o arquivo com modo 0755 na maioria dos sistemas (umask
típica de 022: `0755 & ~022 = 0755`). Um implementador que escreve:

```go
// teste ingênuo — PASSA mesmo sem os.Chmod
os.Remove("scripts/trackfw-validate.sh")
InstallGates(opts)
st, _ := os.Stat("scripts/trackfw-validate.sh")
assert(st.Mode().Perm() == 0755)  // verdadeiro — arquivo criado com os.WriteFile(..., 0755)
```

passa sem o fix porque o modo da criação já é 0755 para arquivo novo. O defeito a corrigir é o
arquivo **pré-existente** com 0644 não ter o bit promovido. O teste correto pré-cria o arquivo com
0644 antes de chamar `InstallGates`.

A sabotagem: remover `os.Chmod` do fix, mas o teste usa arquivo novo → gate verde → defeito vivo.

### (c) AC3 — implementador segue o formato errado da REQ para a fonte errada

A REQ diz "20 linhas de `WARN: agent_models.<tier>`". O formato `WARN: agent_models.<tier> = "..." —
not a version string...` é emitido exclusivamente por `internal/commands/agents_models.go:122`
(comando `trackfw agents models`). Este arquivo **não** é a fonte das 20 linhas do `update harness`.

Um implementador que:
1. Busca `WARN: agent_models` no código → acha `agents_models.go:122`
2. Examina a condição de disparo → percebe que não muda com `update harness`
3. Conclui "o aviso já está correto" ou "não reproduzo o problema"
4. Declara AC3 como "comportamento já correto" sem medir `stderr` de `update harness`

entrega um ML nulo. O defeito permanece ativo: 20 linhas por run no stderr do usuário.

A função real é `harnessCatalogTarget` em `internal/generators/update.go:2669`:
```go
harnessAgentModels, harnessWarnMsg := config.ResolveAgentModels("global", home, harnessCwd)
if harnessWarnMsg != "" {
    fmt.Fprintln(os.Stderr, harnessWarnMsg)
}
```

Chamada uma vez por catalog target (sufixo `-agents` ou `-skills`). Com 20 targets, 20 emissões.
`agents_models.go` não deve ser tocado pelo ML-1C.

### (d) P-3 — "mover para o topo de UpdateHarness" emite indevidamente em runs sem catalog targets

Um implementador que lê "deduplicar a emissão" e move `ResolveAgentModels` para o início de
`UpdateHarness` (antes do loop de targets) resolve o problema de 20 linhas, mas introduz um falso
positivo: `trackfw update harness --dry-run --targets credential-guard-script` (que não tem catalog
targets — `credential-guard-script` é tratado por função específica antes do `splitHarnessCatalogTargetID`)
passa a emitir o advisory mesmo sem nenhum target de agents/skills.

**Medição:** `env -i HOME=$tmp ... update harness --dry-run --targets credential-guard-script` → stderr: 0 linhas, stdout: 4 linhas. Confirmado: o target `credential-guard-script` existe (line 546 de `update.go`); sem catalog targets, nenhum advisory deve ser emitido.

O padrão correto: passar o resultado de `ResolveAgentModels` como parâmetros para
`harnessCatalogTarget` (return types reais: `harnessAgentModels map[string]string, harnessWarnMsg string`),
computá-los **uma vez em `UpdateHarness`** antes do loop, e emitir o warning uma vez — mas somente
se ao menos um catalog target está nos targets selecionados. A pré-varredura deve usar
`splitHarnessCatalogTargetID(id)` diretamente (não reimplementar a verificação de sufixo), para
evitar que pré-varredura e loop driftem.

### (e) AC3 — teste de deduplicação passa vacuamente se a emissão não for injetável

`update.go:2671` usa `fmt.Fprintln(os.Stderr, harnessWarnMsg)` diretamente no `os.Stderr` do
processo. `UpdateOptions` não tem campo `ErrWriter io.Writer`. Cobra's `cmd.SetErr()` não captura
essas linhas porque o cobra redireciona apenas os erros gerados via `cmd.ErrOrStderr()`, não writes
diretos em `os.Stderr`.

Um implementador que:
1. Move `ResolveAgentModels` para fora do loop (fix correto)
2. Não adiciona `ErrWriter` a `UpdateOptions`
3. Escreve o teste em `update_harness_test.go` usando `cmd.SetErr(buf)` e verifica `buf.Len() <= 1`

passa o teste independentemente de emitir 0 ou 20 linhas — porque `buf` fica vazio em ambos os
casos. O gate é vacuoso.

O counter-arm para AC3 requer capturar `os.Stderr` real via `os.Pipe()` ou exige que o fix modifique
`UpdateOptions` para receber um writer injetável.

---

## 3. AC1 — O que `release tag` pode passar a recusar indevidamente

O extrator para `npm/package-lock.json` precisa de duas extrações independentes: o campo raiz
`version` e o campo `packages[""].version`. Ambos devem ser conferidos contra o argumento `<version>`.

**Vetores de falso-positivo (recusa indevida após implementação do AC1):**

| Vetor | Estrutura do lockfile | Comportamento esperado |
|---|---|---|
| **v1 lockfile** (`lockfileVersion: 1`, npm 6) | Tem `version` na raiz; `packages[""]` não existe | Extrator de `packages[""]` falha → recusa. Fail-closed aceitável; npm 6 é anterior a 2020. |
| **Lockfile ausente no commit** | `readCommittedFile` retorna erro (objeto ausente em git) | Recusa com `releaseTagObjectAbsentFmt`. Força o commitador a incluir o lockfile antes do tag. |
| **v2 lockfile** (`lockfileVersion: 2`, npm 7) | `packages[""]` existe | Passa — mesma estrutura de v3. |
| **Campo raiz presente, `packages[""]` ausente** | Lockfile parcialmente corrompido | Extrator de `packages[""]` falha → recusa. Correto. |

**Ordem de verificação:** checar o campo raiz `version` primeiro (falha rápida com mensagem clara); depois o campo `packages[""].version`; depois comparar cada um contra `<version>`. Emitir mensagem que nome o campo divergente (mesma convenção de `vf.label` já usada nos demais extratores).

**Impacto no protocolo de release:** o `release.yml` usa `npm version` que atualiza ambos os campos automaticamente, portanto o protocolo existente já cobre o AC1. Nenhuma mudança de workflow necessária — a relação `npm version → lockfile atualizado` deve ser documentada no roadmap (veja P-1).

**Fixture debt (separado do counter-arm):** O stub `readCommittedFile` usa o mapa `files` populado por `validReleaseVersionFiles` (helper em `release_test.go:22`). Testes que chamam `makeReleaseDeps(t, nil)` — incluindo `TestReleaseTag_PostWave3Tree_Succeeds` e todos que usam o helper — herdam o mapa. Para que não quebrem: adicionar `npm/package-lock.json` ao retorno de `validReleaseVersionFiles`. O `makeReleaseDeps(t, nil)` then auto-inclui o lockfile em todos os testes existentes. Testes que passam `fileOverrides` com o lockfile faltante ainda falharão — isso é correto para o counter-arm. Esses dois artefatos — (a) contra-braço e (b) atualização de `validReleaseVersionFiles` — são prescrições distintas no P-1.

**Stale comment:** o comentário em `release.go:100-102` ("All 3 checks (3 files)") precisa ser atualizado para 4.

---

## 4. AC2 — `os.Chmod` sobre symlink, arquivo de outro dono, caminho fora do projeto, Windows

### 4.1 Symlink

`writeValidateScript` em `discover.go` guarda via `pathguard.RejectAndReport` tanto o diretório `scripts/` (linha 104) quanto o arquivo leaf `scripts/trackfw-validate.sh` (linha 116), antes de qualquer escrita. `pathguard.RejectAndReport` rejeita caminhos que passem por symlinks. Portanto, quando `os.Chmod(dest, 0755)` for chamado após `os.WriteFile`, o caminho é garantidamente um arquivo regular — nenhum symlink pode existir naquele inodo nesse ponto. O `os.Chmod` é seguro.

### 4.2 Arquivo de outro dono (EPERM)

`os.WriteFile` com `O_TRUNC` sobre um arquivo owned por outro usuário mas world-writable (umask permissiva, situação incomum) pode ter sucesso na escrita e falhar em `os.Chmod` com `EPERM`. Hoje `writeValidateScript` não chama `os.Chmod`, então o bug silencioso é o arquivo ficar com 0644. Após a correção, o `os.Chmod` falhará e `InstallGates` retornará erro — o `discover --init` errará com mensagem clara. Isso é uma **regressão na direção de falso-positivo**: antes silencioso (bit ausente), depois ruidoso (exit != 0). Design choice para o implementador: propagar o erro (paridade com scaffold.go) ou fazer `stat` antes e aplicar `os.Chmod` só se `mode&0111 != 0111`. O parecer não decide; o arquiteto escolhe. A opção "propagar" é mais simples e mantém paridade com o padrão AC9.

### 4.3 Caminho fora do projeto (pathguard)

Já bloqueado pelo `pathguard.RejectAndReport` antes de qualquer escrita. `os.Chmod` nunca chega a um caminho fora do projeto.

### 4.4 Windows

`os.Chmod` no Windows (`syscall_windows.go`) opera apenas no bit de somente-leitura derivado do bit de escrita POSIX (0200): modo `0755` e `0644` produzem o mesmo efeito (ambos têm 0200, nenhum é read-only). O bit de execução não existe no NTFS via `Chmod`. O `os.Chmod` retorna `nil` em ambos os casos. Portanto:
- No caminho **write** do AC2 fix: `os.Chmod(dest, 0755)` é chamado, retorna nil, sem efeito. Correto.
- No teste: o AC2 contra-braço (arquivo com 0644 deve ficar 0755) não pode ser verificado no Windows — o teste deve pular com `runtime.GOOS == "windows"` (padrão já usado nos testes de exec-bit no projeto, ver `vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md`).

### 4.5 Hardlink (residual)

`pathguard.RejectAndReport` detecta symlinks via `os.Lstat + ModeSymlink`; não detecta hardlinks. Um hardlink plantado no caminho `scripts/trackfw-validate.sh` antes do `InstallGates` não seria bloqueado. O `os.WriteFile` sobrescreveria o conteúdo do alvo do hardlink e o `os.Chmod` aplicaria 0755 ao mesmo inodo. O dano é limitado: o atacante precisa de write na mesma partição, e a consequência é `+x` no arquivo alvo do hardlink — não uma escalada de privilégio. Declarado como residual; ver seção 6.

### 4.6 Sítios fora do escopo do AC2 — severidade do mecanismo

Para os sítios **harness** (`scaffold.go` 1244, 1622; `update.go` 877, 916 caminho write): o formato de hook atual é um **comando inline** (`trackfw guard credential --global`), não uma referência ao `.sh`. O bit de execução dos scripts harness não é consultado pelos hooks Claude, Gemini, Codex, Cursor, Windsurf, Kiro (confirmado em `internal/guardforms/guard_generations.go:52`). Os scripts ficam para migração de configurações legadas. **Impacto de `+x` ausente: BAIXO** — o único cenário de falha é um usuário chamar o script diretamente pelo caminho absoluto.

Para os sítios **commit-hook** (`scaffold.go` 1974, 2004, 2042; `update.go` 267):
- `scaffold.go:1974` — husky `.husky/commit-msg`: git executa hooks por fork+exec; `+x` é requisito. Se criado em sistema com umask restritiva que mascara 0111, o commit falha com exit 126. **Impacto: MÉDIO.**
- `scaffold.go:2004` — lefthook `.lefthook/commit-msg/trackfw-req-check.sh` com `runner: sh` (linha 1982: `"runner: sh\n"`): lefthook invoca o script via `sh script.sh`, não por exec direto. `+x` não é necessário para este caso. **Impacto: BAIXO.**
- `scaffold.go:2042` — husky `.husky/pre-commit`: mesmo caso de 1974, git exec. **Impacto: MÉDIO.**
- `update.go:267` — `O_APPEND|O_CREATE` no `.husky/pre-commit` do projeto: criação com modo `0755`; sobre arquivo pré-existente, O_APPEND não altera permissões. **Impacto: BAIXO** (arquivo pré-existente já teve mode set por quem o criou).

---

## 5. AC3 — Condição exata que reproduz o aviso de `agent_models`

### 5.1 Identificação do aviso — dois mecanismos distintos

**Aviso 1 — formato `WARN: agent_models.<tier> = "..." — not a version string...`**
- Fonte: `internal/commands/agents_models.go:122` (comando `trackfw agents models`)
- Disparado por: `integrations.LooksLikeSuspectModelValue(version)` retornar `true` para um valor no `agent_models` resolvido
- Valores que disparam: `"gpt-5"`, `"latest"`, `"4.6-beta"` (não são versão pura nem `claude-*`); não dispara para `"4.6"`, `"5"`, `"claude-sonnet-4-5-20250929"`
- **NÃO emitido por `update harness`**

**Aviso 2 — mensagens advisory de configuração global**
- Fonte: `internal/generators/update.go:2671` (`fmt.Fprintln(os.Stderr, harnessWarnMsg)`)
- Emitido uma vez **por catalog target** na função `harnessCatalogTarget` (chamada no loop de todos os targets `-agents`/`-skills`)
- Disparado por `config.ResolveAgentModels("global", home, cwd)` retornar `warnMsg != ""`
- Texto: `GlobalAgentModelsNoneMessage` / `GlobalAgentModelsProjectOnlyMessage` / `MalformedGlobalConfigMessage`
- **Não contém o token `WARN:`**

**Contradição com a REQ:** A REQ diz "20 linhas de WARN: agent_models.\<tier\>". O formato Aviso 1 nunca vem de `update harness`. As 20 linhas reais são do Aviso 2 (advisory de configuração). A REQ misidentifica a fonte e o formato do aviso de 2026-10-30.

Nota de vault: [update-harness-advisory-por-catalog-target-nao-warn-2026-10-31](../../vault/notes/update-harness-advisory-por-catalog-target-nao-warn-2026-10-31.md)

### 5.2 Condições medidas (binário instalado 11.2.3, env -i, cwd fora do projeto)

```bash
SCRATCH=/private/tmp/...  # scratchpad da sessão
BIN=/opt/homebrew/bin/trackfw

# Cenário A — sem ~/.trackfw/trackfw.yaml
TMPH=$(mktemp -d); mkdir -p "$TMPH/.trackfw"
cd "$SCRATCH" && env -i HOME="$TMPH" PATH=/opt/homebrew/bin:/usr/bin:/bin \
  "$BIN" update harness --dry-run 1>out.txt 2>err.txt
wc -l out.txt err.txt
# out.txt: 39 linhas (relatório texto no STDOUT)
# err.txt: 20 linhas (GlobalAgentModelsNoneMessage × 20 targets)
# nenhuma linha contém "WARN: agent_models"
```

| Cenário | Condição | STDERR | Aviso 1? |
|---|---|---|---|
| A | Sem `~/.trackfw/trackfw.yaml` | 20× `GlobalAgentModelsNoneMessage` | Não |
| B | `~/.trackfw/trackfw.yaml` existe mas sem `agent_models` | 20× `GlobalAgentModelsNoneMessage` | Não |
| C | `agent_models` somente em `trackfw.yaml` do projeto | 20× `GlobalAgentModelsProjectOnlyMessage` | Não |
| D | `agent_models` válido em `~/.trackfw/trackfw.yaml` | 0 linhas | Não |
| E | `agent_models` suspeito (`gpt-5`, `latest`) em `~/.trackfw/trackfw.yaml` | 0 linhas (advisory) | Não |
| F | `~/.trackfw/trackfw.yaml` YAML malformado | 20× `MalformedGlobalConfigMessage` | Não |
| `trackfw agents models` com Cenário E | — | `WARN: agent_models.haiku = "latest" — not a version string...` | **Sim** |

**Causa raiz das 20 linhas:** `ResolveAgentModels("global", home, cwd)` é invocado dentro de `harnessCatalogTarget` (linha 2669 de `update.go`), que é chamada uma vez por target no loop de harness. Com 20 targets agent/skill, a mensagem é emitida 20 vezes. O padrão correto está descrito em P-3.

### 5.3 Canal do relatório em texto

`printUpdateReportText` em `update_harness.go:128` usa `cmd.OutOrStdout()` — stdout. Confirmado por medição: Cenário A com stdout separado de stderr produz 39 linhas no stdout (relatório texto) e 20 linhas no stderr (advisory).

**Contradição com a REQ:** A REQ diz "o relatório em texto do update harness sai inteiro no stderr". Isso é incorreto. O relatório texto vai para stdout. Apenas os advisories de configuração vão para stderr. `docs/cli-parity.md:804` documenta explicitamente: "agent_models warnings from update harness) may still appear on stderr" — refere-se aos advisories, não ao relatório.

Conclusão para AC3: o relatório texto vai para stdout e já está correto; não é necessário mover nada para stdout. O que precisa ser corrigido é a deduplicação do advisory (uma emissão por run, não por target).

---

## 6. Residual declarado e Prescrições

### 6.1 Alvos de falsificação em ambas as direções

**AC1 (lockfile no release tag):**
- **FN** (lockfile divergente aceito): Medir extrator com `packages[""].version` diferente de `<version>` e confirmando recusa; e com `version` na raiz diferente e confirmando recusa. Ambos os campos devem ser vetores independentes (ver Seção 2a).
- **FP** (lockfile válido recusado): Vetor principal: arquivo `npm/package-lock.json` ausente no commit — `readCommittedFile` falha. Documentar que `npm version` no workflow atualiza o lockfile automaticamente.

**AC2 (exec bit no InstallGates):**
- **FN** (script criado sem exec bit): pré-criar `scripts/trackfw-validate.sh` com `0644`, chamar `InstallGates`, verificar `mode == 0755`. (Arquivo novo não é vetor válido — ver Seção 2b.)
- **FP** (InstallGates falha em arquivo legítimo): arquivo owned por outro usuário com Chmod retornando EPERM. Escolha de design: propagar (paridade AC9) ou condicional (ver P-2).

**AC3 (deduplicação do advisory):**
- **FN** (advisory ainda emitido N vezes): contar linhas de stderr em Cenário A após fix; deve ser 1.
- **FP** (advisory silenciado quando deveria aparecer): verificar que Cenários A/B/C ainda emitem (uma vez) após fix; verificar que `--targets credential-guard-script` não emite (zero catalog targets).

### 6.2 Residual declarado

| ID | Descrição | Severidade | Motivo da aceitação |
|---|---|---|---|
| R-1 | Harness scripts `~/.trackfw/scripts/*.sh` sem `os.Chmod` nos caminhos de escrita (`scaffold.go` 1244/1622, `update.go` 877/916) | BAIXO | Hooks atuais usam comando inline, não o caminho `.sh`. Impacto: chamada direta ao script. |
| R-2 | `scaffold.go` 1974/2042: husky commit-msg/pre-commit sem Chmod (git exec, +x obrigatório) | MÉDIO | Excluído pelo escopo negativo da REQ; mesmo mecanismo. `scaffold.go:2004` (lefthook, `runner: sh`) é BAIXO — lefthook chama o script via `sh script.sh`, não exec direto. `update.go:267` é BAIXO — O_APPEND não altera permissão de arquivo pré-existente. |
| R-3 | `discover.go` 179/293/336: O_APPEND hooks sem Chmod | BAIXO | O_APPEND não altera permissões de arquivo existente; o bit só está errado se o arquivo foi criado originalmente com umask que mascara 0755. |
| R-4 | Hardlink em `scripts/trackfw-validate.sh` não detectado por pathguard | BAIXO | Requer acesso à partição; o dano se limita a `+x` no alvo do hardlink. |
| R-5 | Advisory de `agent_models` suprimido quando todos os targets estão como `missing` | ABERTO | O arquiteto deve decidir: suprimir advisory quando nada está instalado (ruído puro) ou manter (informa o usuário sobre ausência de config global). |

---

## Prescrições

**P-1 (ML-1A — AC1, `release tag`):**
(a) Adicionar `npm/package-lock.json` a `releaseVersionFiles` com extrator próprio que: (i) parse JSON do conteúdo; (ii) extraia e compare `version` da raiz; (iii) extraia e compare `packages[""].version`; (iv) produza mensagem de erro que nomeia o campo divergente. Atualizar o comentário em `release.go:101` de "3 files" para "4 files". Confirmar a ajuda em `release.go:178` ("4 version files" ficará correto após a correção — já era o texto esperado). Documentar que `npm version` atualiza ambos os campos automaticamente.

(b) **Counter-arm:** adicionar dois novos casos em `TestReleaseTag_VersionFileMismatch_NamesWhichFile`. O struct `cases` deve receber um campo `content string` para o conteúdo do arquivo divergente (evita que os dois casos de lockfile compartilhem o mesmo conteúdo via `switch tc.path`, o que permitiria que um extrator de campo único passasse ambos os casos). Dois casos:
- `"lockfile-root"`: raiz `version` = `"0.0.1"`, `packages[""]` = `releaseTestVersion` → deve reprovar com erro nomeando campo raiz.
- `"lockfile-packages"`: raiz `version` = `releaseTestVersion`, `packages[""]` = `"0.0.1"` → deve reprovar com erro nomeando campo `packages[""]`.
Sem a entrada nova em `releaseVersionFiles`, ambos os casos falham — essa é a prova que fecha AC1. Encerrando assim a ameaça 2(a): um extrator de campo único que verifica apenas a raiz passa o caso `"lockfile-packages"` e é detectado.

(c) **Fixture debt (separado do counter-arm):** adicionar `npm/package-lock.json` com versão correta ao retorno de `validReleaseVersionFiles` em `release_test.go:22`. O `makeReleaseDeps(t, nil)` propaga automaticamente para todos os testes existentes, inclusive `TestReleaseTag_PostWave3Tree_Succeeds`. Isso não é regressão — é acerto de fixture para refletir o novo contrato.

**P-2 (ML-1B — AC2, `discover.go`):**
Em `writeValidateScript` (`discover.go:120`), após `os.WriteFile(dest, []byte(content), 0755)`, adicionar `os.Chmod(dest, 0755)` com propagação de erro — mesma forma do padrão AC9 em `scaffold.go:1077`. Teste: pré-criar `scripts/trackfw-validate.sh` com modo `0644`; chamar `InstallGates`; verificar `mode.Perm() == 0755`. Pular no Windows (`runtime.GOOS == "windows"`). Nomear no roadmap a decisão sobre os outros sítios (R-2): ou abrir novo ML na mesma REQ (Regra Dura), ou documentar a razão de severidade que justifica o escopo negativo.

**P-3 (ML-1C — AC3, deduplicação):**
**Arquivo correto: `internal/generators/update.go`, função `harnessCatalogTarget` (linha 2659). NÃO tocar `internal/commands/agents_models.go` nem `internal/commands/update_harness.go` para este fix.**

Mover `config.ResolveAgentModels("global", home, harnessCwd)` para fora de `harnessCatalogTarget`:
adicionar parâmetros `agentModels map[string]string` à assinatura de `harnessCatalogTarget` ao invés de
recomputar dentro; remover a emissão de `harnessWarnMsg` de dentro da função. Em `UpdateHarness`,
antes do loop, pré-varrer `selectedTargets` com `splitHarnessCatalogTargetID(id)` — se algum catalog
target existir, computar `harnessAgentModels, harnessWarnMsg` uma vez via `ResolveAgentModels` e
emitir o warning uma vez antes do loop; se não houver, não computar nem emitir. Alternativa
equivalente: `sync.Once` dentro de `UpdateHarness`. A implementação "mover para o topo incondicional
de `UpdateHarness`" é INCORRETA — emite o advisory em runs sem catalog targets (ex: `--targets
credential-guard-script`, stderr medido: 0 linhas com comportamento atual correto).

**Testabilidade — requisito do fix:** `fmt.Fprintln(os.Stderr, harnessWarnMsg)` em `update.go:2671`
escreve no `os.Stderr` real, não em um writer injetável. A cobra `cmd.SetErr()` não captura essas
linhas. O fix deve adicionar `ErrWriter io.Writer` a `UpdateOptions` (ou equivalente) e substituir
`os.Stderr` pelo writer injetado. Sem isso, um teste em `update_harness_test.go` que verifique
"≤1 linha no stderr" passa com qualquer número de emissões (vacuidade). O counter-arm é:
reverter o fix de deduplicação e confirmar que o teste detecta N>1 linhas (ver ameaça (e) na seção 2).

Verificar: Cenário A passa a emitir exatamente 1 linha de advisory no writer. Cenário D (config global válida) emite 0 linhas. `--targets credential-guard-script` emite 0 linhas.

**Arquivo alvo confirmado pela medição:** `internal/generators/update.go:2659-2671` (função `harnessCatalogTarget`, linha de `ResolveAgentModels`). O arquivo `internal/commands/update_harness.go` e `internal/commands/agents_models.go` listados no ML-1C do roadmap estão **incorretos** como sítio do fix — o arquiteto deve atualizar a lista de arquivos do ML-1C antes de despachar o agente.

---

## Contradições declaradas com a REQ (para o arquiteto)

| Item | O que a REQ diz | O que a medição mostra |
|---|---|---|
| AC2 "os outros cinco" | "os outros cinco pontos de escrita já fazem isso" | Cinco corretos, mas outros **10** sítios não têm `os.Chmod` (9 na enumeração da REQ + `update.go:267` ausente). Escopo negativo da REQ não cobre o conflito com a Regra Dura. |
| AC3 formato do aviso | "20 linhas de WARN: agent_models.\<tier\>" | O `update harness` emite advisory de config (`GlobalAgentModelsNoneMessage`), não `WARN: agent_models.<tier>`. O formato citado na REQ vem de `trackfw agents models`. |
| AC3 canal do relatório | "relatório em texto do update harness sai inteiro no stderr" | Relatório texto vai para stdout (`cmd.OutOrStdout()`). Apenas advisories vão para stderr. `cli-parity.md:804` confirma o design. |
| ML-1C arquivos afetados | `agents_models.go`, `update_harness.go` | O sítio do fix é `internal/generators/update.go:harnessCatalogTarget`. `agents_models.go` não deve ser alterado. O roadmap precisa ser corrigido antes do handoff. |
| ML-1C critério de aceite e gates de Wave 1 | "pacote `internal/commands` verde"; gate roda `./internal/commands/ ./internal/discover/` | O sítio do fix é `internal/generators/`. Sem `./internal/generators/` nos gates, um teste que verifica AC3 não é exercitado pelo gate da wave. Adicionar `./internal/generators/` ao gate e ao critério de aceite do ML-1C. |
| AC3 testabilidade | (implícito: capture com cmd.SetErr) | `fmt.Fprintln(os.Stderr, ...)` em `update.go:2671` não é capturado por cmd.SetErr (não há ErrWriter em UpdateOptions). Fix deve adicionar writer injetável ou capturar via os.Pipe no teste; sem isso o counter-arm é vacuoso. |
