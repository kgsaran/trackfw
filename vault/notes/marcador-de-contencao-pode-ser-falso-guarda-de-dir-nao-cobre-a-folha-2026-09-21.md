# Marcador de contenção pode ser FALSO: guarda de diretório não cobre a folha

> 2026-09-21 · REQ-2026-08-31 (contenção de escrita) · MLs 2A/3A · PR #397

## O sintoma

`scripts/check-write-containment.sh` verde, `make quality` verde, CI verde — sobre uma escrita
**sem guarda nenhuma**.

## As duas causas, e elas se compõem

### 1. `RejectSymlinks(root, dir)` nunca desce abaixo de `dir`

`pathguard.RejectSymlinks` faz `os.Lstat` em cada componente **de `absTarget` para cima**, até
`root` inclusive. Se você passa o **diretório** e depois escreve um **arquivo dentro dele**, a folha
nunca é inspecionada:

```go
rejectScaffoldPath(absHome, absSkillDir)   // guarda trackfw/
os.WriteFile(skillPath, ...)               // escreve trackfw/SKILL.md  ← folha não checada
```

Com `SKILL.md` já sendo symlink para fora, a escrita passa **através** dele. PoC do `hades-tf`:
`err=nil`, `✓` impresso, vítima sobrescrita.

🔴 **Medição:** de 18 chamadas de `rejectScaffoldPath` em `scaffold.go`, **13 tinham esse defeito**.
O revisor estimou ~6. **Estimativa não é medição** — a classificação linha a linha é obrigatória.

**Regra:** guarde **o caminho do arquivo que você vai escrever**. Guardar o diretório só basta
quando a operação é `MkdirAll` e nada mais.

### 2. O marcador afirma; ele não prova

O gate aceita um sítio pelo comentário `// write-containment-allowed: <razão>`. O gate é bash puro e
**não faz análise de fluxo** — ele não consegue verificar que a guarda citada existe, nem que cobre
aquele caminho.

Caso real, em `generateCommitMsgHook` (`scaffold.go`, ramo `lefthook`):

```go
// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
os.WriteFile(lefthookPath, ...)      // lefthook.yml, na RAIZ
```
A única guarda do bloco cobria `.lefthook/commit-msg` — **outro diretório**. `lefthook.yml` estava
fora dela. **O marcador era falso, e o gate estava verde por causa dele.**

## Por que a verificação do arquiteto não pegou

Eu cruzei os 17 arquivos marcados contra uso de `pathguard.` e concluí que o risco de "marcador
carimbo" estava controlado. **O cruzamento era por ARQUIVO.** O marcador falso estava num arquivo
que legitimamente usa `pathguard` em vários outros pontos — a verificação não podia pegá-lo, e eu
tratei o risco como controlado quando não estava.

🔴 **A lição de método é mais geral que o bug:** uma verificação por agregado (arquivo, pacote,
contagem) não falsifica uma afirmação feita no nível do **sítio**. Se a afirmação é por linha, a
verificação tem de ser por linha.

## O que fazer ao encontrar um marcador

1. **Não confie no texto.** Ache a guarda que ele cita e confirme que ela cobre **o caminho exato**
   que está sendo escrito — não o diretório pai, não um irmão.
2. Se a guarda cobre o diretório e a escrita é de um arquivo, é defeito.
3. O padrão correto está em `scaffold.go:824` (`writeTrackfwConfig`): guarda `absConfig`, o arquivo.

## Residual conhecido

`scaffold.go` foi classificado linha a linha no ML-3A. **Os demais arquivos com marcador não foram**
— `generators/update.go` (53 marcadores), `agentfiles.go` (21), `discover/discover.go` (13),
`roadmap.go`/`req.go` (7 cada), `note.go`/`adr.go` (4 cada). A mesma classe de defeito pode existir
lá, e o gate não a detecta por construção.

O fix estrutural é um analisador de AST — ambos os revisores (`hades-tf`, `hefesto-tf`) chegaram a
essa conclusão de forma independente.
