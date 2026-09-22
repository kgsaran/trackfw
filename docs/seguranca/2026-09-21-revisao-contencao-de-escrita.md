# Revisao de Seguranca — Barreira Final da REQ de Contencao de Escrita

**Data:** 2026-09-21
**Agente:** hades-tf
**Branch:** `fix/afirma-contencao-antes-de-escrever`
**REQ:** `docs/req/REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente-e-nunca-inspeciona-ancestral-escrita-fora-do-projeto-em-todo-so-e-todo-runtime.md`

---

## Veredito

**APROVADO COM RESSALVAS**

Os caminhos CLI-alcancaveis estao corretamente guarded e os dois bracos principais da REQ
(ancestral symlink, leaf symlink nos caminhos reachable) estao fechados. Uma ressalva de
gravidade media existe: `installGlobalSkillInner` tem um gap de folha confirmado por PoC
executado, mas essa funcao nao e alcancavel por nenhum comando cobra — expoe apenas a API
Go programatica. Por regra de causa raiz, entra como ML adicional neste roadmap, nao como
REQ nova. Detalhe por questao abaixo.

---

## Q1 — A contencao e contornavel? (TOCTOU + componente inexistente + Beneath)

### TOCTOU

**Veredito:** janela existe, nao e pratica neste produto. RESIDUAL ACEITO.

A janela e: entre o ultimo `os.Lstat` no loop de `RejectSymlinks` e o primitivo de escrita.
No caminho mais critico (`harnessClaudeSkillTarget` em `update.go`), a janela e alargada
concretamente: existe um `os.MkdirAll(filepath.Dir(path), 0755)` entre o guard e o
`os.WriteFile`. `MkdirAll` segue symlinks por design. Se um atacante substituir o diretorio
pai por um symlink apos o guard mas antes do `MkdirAll`, o diretorio e criado fora da arvore.

Porem: este e um CLI de desenvolvedor, mono-usuario, sem daemon, sem setuid, sem processo
concorrente nao confiavel. Para explorar o TOCTOU, o atacante precisa de um processo
concorrente na mesma maquina com permissao de escrita no `$HOME`. Quem tem isso nao precisa
deste TOCTOU. Residual identiico ao que a Wave 0 ja declarou.

### Componente inexistente nao e erro

**Veredito:** decisao correta. Sem vetor especifico alem do TOCTOU geral.

Caso de uso dominante: criar arquivo novo (folha e, as vezes, diretorio pai nao existem).
`RejectSymlinks` caminha de `filename` ate `root`, e componentes inexistentes sao tolerados
via `os.IsNotExist`. Nao ha como criar um componente inexistente que seja simultaneamente
um symlink (symlinks existem por definicao). O unico vetor e TOCTOU (tratado acima).

### Beneath — edge cases

**APFS case-insensitivity** — MEDIDO em darwin.

```
Beneath("/Users/User/project", "/Users/user/project/file.md") = false
Beneath("/Users/user/project", "/Users/user/project/file.md") = true
filepath.Rel("/Users/User/project", "/Users/user/project/file.md") = "../../user/project/file.md"
```

Resultado: se root e target tiverem grafias de caso diferentes, `Beneath` retorna `false`
(falso-NEGATIVO da contencao — rejeita operacao legitima). Em uso normal, root e target
sao derivados do mesmo `os.Getwd()` + `filepath.Join`, entao casing e consistente.
Risco residual: `filepath.EvalSymlinks` num symlink que aponta para path com grafia diferente
poderia criar inconsistencia. Baixo impacto pratico.

**Separadores Windows** — raciocinio da stdlib. `filepath.Rel` normaliza via `filepath.Clean`
internamente, e `Beneath` usa `string(filepath.Separator)` — portavel.

**Reserved names Windows (CON, NUL)** — raciocinio: trackfw gera nomes fixos (`trackfw.yaml`,
`SKILL.md`). Nenhum nome reservado do Windows. Sem vetor pratico.

**UNC paths e Unicode NFC/NFD** — nao testavel em darwin/linux. Trackfw nao gera UNC paths.
NFD/NFC e teoricamente exploravel via EvalSymlinks num FS HFS+ legacy, mas o produto usa
`filepath.Join(root, ...)` com casing consistente. Classificado como residual teorico.

---

## Q2 — A guarda cobre o ancestral, mas e a FOLHA?

### Caminho CLI-alcancavel (`trackfw update harness --targets claude-skill`)

**PoC 1 — SKILL.md como symlink de folha:**

```bash
mkdir -p $SCRATCH/fakehome2/.claude/skills/trackfw
echo "PRESERVE" > $SCRATCH/victim2.txt
ln -sf $SCRATCH/victim2.txt $SCRATCH/fakehome2/.claude/skills/trackfw/SKILL.md
HOME=$SCRATCH/fakehome2 ./bin/trackfw update harness --targets claude-skill --install-missing
```

Resultado:
```
trackfw: refusing write to .../fakehome2/.claude/skills/trackfw/SKILL.md: refusing symlink path ".../SKILL.md"
trackfw update harness
x claude-skill: failed (~/.claude/skills/trackfw/SKILL.md)
updated=0 skipped=0 missing=0 failed=1
Error: trackfw update harness: 1 target(s) failed
RC=1
victim2.txt: PRESERVE (intacta)
```

**PoC 2 — ancestral como symlink (o defeito original da REQ):**

```bash
mkdir -p $SCRATCH/fakehome3/.claude/skills
ln -sf $SCRATCH/victim_dir $SCRATCH/fakehome3/.claude/skills/trackfw
echo "PRESERVE" > $SCRATCH/victim_dir/SKILL.md
HOME=$SCRATCH/fakehome3 ./bin/trackfw update harness --targets claude-skill
```

Resultado:
```
trackfw: refusing write to .../fakehome3/.claude/skills/trackfw/SKILL.md: refusing symlink path ".../trackfw"
RC=1, vitima intacta
```

**Mecanismo correto:** `harnessClaudeSkillTarget` chama
`rejectHarnessSymlink(home, path, ...)` → `pathguard.RejectSymlinks(home, path)` onde
`path` e o arquivo real de escrita (`SKILL.md`). O guard caminha de `path` ate `home`,
checando TODOS os componentes incluindo a folha.

### `installGlobalSkillInner` — GAP CONFIRMADO POR PoC

**PoC 3 — SKILL.md como symlink via `installGlobalSkillInner` (API programatica):**

Setup identico ao PoC 1. Chamada via test:
```go
err = installGlobalSkillInner(force=true)
```

Resultado:
```
Before: victim_leaf.txt = "PRESERVE\n"
installGlobalSkillInner result: <nil>
CLI print: "checkmark ~/.claude/skills/trackfw/SKILL.md"
After: victim_leaf.txt = "---\nname: trackfw\ndescription: ..." (conteudo do SKILL.md)
LEAF GAP CONFIRMED: victim was overwritten through symlink
```

**Causa:** `rejectScaffoldPath(absHome, absSkillDir)` guarda `absSkillDir` (diretorio
`trackfw/`). `os.WriteFile(skillPath, ...)` escreve em `skillPath` (`trackfw/SKILL.md`).
`RejectSymlinks(root, dir)` caminha de `dir` ate `root` — NUNCA desce abaixo de `dir`.
Uma folha dentro de `dir` que seja symlink nunca e checada.

**Alcancabilidade:** `installGlobalSkillInner` e chamada por `installSkillsInner`, que e
chamada pelas funcoes exportadas `InstallSkills()` e `ForceInstallSkills()`. Nenhum
comando cobra chama essas funcoes — verificado com:

```
grep -rn "InstallSkills\|ForceInstallSkills" --include="*.go" . | grep -v "_test.go" ...
```

Resultado: apenas declaracoes em `scaffold.go`. Sem call site de producao.

**Impacto:** gap na API Go programatica (importar o pacote). Nao exploravel via CLI.

**Padrao sistematico:** o mesmo gap existe em outros sites de `scaffold.go` que guardam
um DIRETORIO mas escrevem num ARQUIVO dentro dele (linhas 841, 940, 998, 1049, 1341).
O padrao correto — usado por `writeTrackfwConfig` (linha 824) e por `harnessClaudeSkillTarget`
— e passar o ARQUIVO ao guard, nao o diretorio pai.

**Recomendacao:** novo ML neste roadmap, mesma REQ (mesma causa — guard no diretorio
em vez de no arquivo). O padrao correto ja existe no proprio codebase como referencia.
Nao e REQ nova — e a Regra Dura de Causa Raiz aplicada.

---

## Q3 — O marcador e superficie de ataque de processo?

**Veredito:** SIM, e um risco de processo aceito. Nao ha discriminante bash puro sem
reintroduzir o custo do ML-1D-bis.

O gate verifica a presenca textual de `write-containment-allowed:` na linha do site ou
na linha imediatamente acima. Um desenvolvedor pode copiar o marcador sem criar guard
e o gate passa:

```go
// write-containment-allowed: TODO adicionar guard depois
os.WriteFile(caminhoNaoGuardado, data, 0644)  // gate: PASSA
```

O gate fornece protecao contra ACIDENTE (esquecer o guard), nao contra DESCUIDO DELIBERADO
(copiar o marcador sem entender).

**Por que nao tem discriminante melhor em bash:**

A historia e explicita no header do script — `check-symlink-privilege-guard.sh` usou janela
de +/-5 linhas e isso gerou o ML-1D-bis: um ML inteiro para mover um comentario para dentro
da janela. O custo de uma janela de proximidade e documentado e ja foi pago uma vez. Propor
janela e propor repagar.

O unico discriminante bash sem janela: verificar por ARQUIVO (nao por linha) se qualquer
arquivo que contenha um site marcado tambem contem `pathguard.` em algum lugar. Isso
mecanizaria a verificacao que o arquiteto ja fez manualmente em 17 arquivos. Limite:
nao pega marcador copiado para arquivo que ja importa `pathguard` por outro motivo.
Implementavel em bash puro; nao reintroduz janela; nao foi implementado por nao ser escopo
desta revisao.

A barreira independente correta para esse risco e um analisador de AST Go (staticcheck,
custom go vet pass), fora do alcance do bash.

---

## Q4 — O gate novo pode ser esvaziado?

**Veredito:** bypass LOCAL via `WRITE_CONTAINMENT_SCAN_DIR` confirmado. CI NAO e afetado.

### Bypass confirmado

```bash
mkdir -p /tmp/bypass_test2/internal/pkg
# Go file com 1 site marcado (< SITE_FLOOR=157)
cat > /tmp/bypass_test2/internal/pkg/safe.go << 'EOF'
package pkg
import "os"
func f(p string, d []byte) error {
    // write-containment-allowed: fixture
    return os.WriteFile(p, d, 0644)
}
EOF
WRITE_CONTAINMENT_SCAN_DIR=/tmp/bypass_test2 bash scripts/check-write-containment.sh
# Saida: OK — todos os sitios justificados (1 examinados)
# Exit: 0
```

Quando `WRITE_CONTAINMENT_SCAN_DIR` esta definido, `OVERRIDE_MODE=1` e o bloco de piso e
pulado. A verificacao de `FILE_COUNT == 0` ainda dispara se nao houver nenhum arquivo Go,
mas com qualquer arquivo Go presente (mesmo com 1 site) o gate passa.

### Por que o CI nao e afetado

O gate e invocado no Makefile como:
```makefile
scripts/check-write-containment.sh
```

Sem `WRITE_CONTAINMENT_SCAN_DIR`. A busca em `.github/workflows/` confirmou que nenhum
workflow define essa variavel. CI executa o gate sem override; piso de 157 e sempre aplicado.

### Vetor residual

Um PR malicioso poderia adicionar `WRITE_CONTAINMENT_SCAN_DIR` a um step de CI. Esse PR
mudaria um arquivo de workflow — mudanca visivel no diff e bloqueada pelo revisor.
Mitigacao de uma linha: `unset WRITE_CONTAINMENT_SCAN_DIR` no Makefile antes da chamada,
ou rejeitar `OVERRIDE_MODE` exceto quando `--self-test` tambem e passado. Nao e bloqueante
para merge; e uma hardening barata para um ML posterior.

### Arquivo untracked

O script usa `find` (nao `git ls-files`). Arquivos Go novos e untracked EM `internal/` SAO
varridos. O comentario no script documenta isso explicitamente. Esse vetor especifico
(arquivos untracked escapando) esta fechado.

---

## Q5 — Residual declarado

O que continua exposto apos este PR:

| # | Superficie | Motivo de residual | Aceitavel? |
|---|---|---|---|
| R1 | TOCTOU entre guard e escrita | Single-user local CLI; atacante com proc concorrente ja ganhou de outra forma | Sim — Wave 0 ja declarou |
| R2 | Leaf gap em `installGlobalSkillInner` e ~5 sites analogos em `scaffold.go` | Funcoes nao alcancaveis por CLI, somente por API Go | PARCIALMENTE — recomendo ML adicional neste roadmap |
| R3 | Marcador sem verificacao de presenca real do guard | Risco de processo; discriminante em bash tem custo (ML-1D-bis); fix correto exige AST analyzer | Sim — gate protege contra acidente, declaradamente |
| R4 | Bypass de `WRITE_CONTAINMENT_SCAN_DIR` em modo local | Requer definir env var deliberadamente; CI nao exposto | Sim — hardening barata para ML posterior |
| R5 | `Beneath` sensivel a casing em APFS case-insensitive | Trackfw usa casing consistente via `filepath.Join(root, ...)` | Sim — falso-negativo (rejeicao indevida) nao e vetor de escrita fora da arvore |

---

## O que eu bloquearia

Nao bloqueio o merge. Ha uma ressalva acionavel:

**Ressalva (media):** `installGlobalSkillInner` e funcoes analogas em `scaffold.go` guardam
o DIRETORIO mas escrevem num ARQUIVO dentro dele — gap de folha confirmado por PoC.
A funcao nao e CLI-alcancavel hoje, mas e uma API Go exportada com gap documentado. O
padrao correto ja existe no proprio codebase (`writeTrackfwConfig`, `harnessClaudeSkillTarget`).

**Acao recomendada:** novo ML neste roadmap (Regra Dura de Causa Raiz — mesma causa, mesma
REQ, mesmo PR se possivel). O ML deve substituir `rejectScaffoldPath(root, dir)` + `WriteFile(dir/file)`
por `rejectScaffoldPath(root, dir/file)` + `WriteFile(dir/file)` nos ~6 sites afetados
em `scaffold.go`, usando `writeTrackfwConfig` como referencia.

---

## Evidencias de execucao

Todas as PoCs e testes rodados em darwin 27.0.0, branch `fix/afirma-contencao-antes-de-escrever`.
Binario: `bin/trackfw` compilado de `make build` nesta sessao.
Arquivo de PoC de teste (`leaf_gap_poc_test.go`) removido apos execucao — nao commitado.
