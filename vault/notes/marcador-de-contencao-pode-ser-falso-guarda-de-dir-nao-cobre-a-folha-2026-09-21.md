# Marcador de contenção pode ser FALSO: guarda de diretório não cobre a folha

> 2026-09-21 · REQ-2026-08-31 (contenção de escrita) · MLs 2A/3A · PR #397

## O sintoma

`scripts/check-write-containment.sh` verde, `make quality` verde, CI verde — sobre uma escrita
**sem guarda nenhuma**.

## As duas causas, e elas se compõem

### 1. `RejectSymlinks(root, dir)` nunca desce abaixo de `dir`

🔴 **Leia esta parte com cuidado — é a distinção que confunde.** `pathguard.RejectSymlinks` começa
com `current := filename` e faz `os.Lstat` **no próprio `filename` primeiro**, depois sobe pelos
ancestrais até `root`:

```go
current := filename
for {
    info, err := os.Lstat(current)      // ← a PRIMEIRA iteração é a folha
    ...
    current = filepath.Dir(current)
}
```

Portanto: **passar o ARQUIVO protege a folha.** O gap não é do `pathguard` — é de quem chama
passando o **diretório** e depois escreve um arquivo **abaixo** dele, que nunca entra no laço. Se você passa o **diretório** e depois escreve um **arquivo dentro dele**, a folha
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

## Resultado da auditoria completa (Wave 4, 2026-09-21)

Todos os 157 marcadores foram classificados. **22 gaps de folha e 7 marcadores falsos**, todos com o
gate **verde** por cima:

| arquivo | sítios | gap (B) | falso (D) |
|---|---|---|---|
| `scaffold.go` | 18 | **13** | 1 (`lefthook.yml`) |
| `generators/update.go` | 53 | 0 | 5 (`copyPath`) |
| `agentfiles`/`roadmap`/`req`/`note`/`adr`/`java` | 44 | **5** | 1 (`syncREQReferences`) |
| `discover/` + marcador único | 24 | **4** | 0 |

🔴 **A taxa não é uniforme.** `scaffold.go` deu 72%; `update.go` — o de **escopo global**, `$HOME` —
deu **zero**. Extrapolar de um arquivo teria errado nos dois sentidos. **Classifique linha a linha
ou não afirme nada.**

**O pior caso foi `syncREQReferences`** (`roadmap.go`): marcador presente e **zero** chamadas a
`pathguard` na função inteira. O teste contra o código antigo imprime `✓ synced REQ-...` e a vítima
é sobrescrita através do symlink.

## Terceira classe, descoberta na auditoria: guarda fail-OPEN

Nem sempre a guarda está ausente — às vezes ela é **pulada em silêncio**:

```go
if cwd, cwdErr := os.Getwd(); cwdErr == nil {
    ...guarda...
}
// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
return os.WriteFile(baselineFileName, data, 0644)   // ← executa MESMO se a guarda foi pulada
```

Sítios: `validator.go:58`, `metrics.go:199` (`os.Getwd()`), `config_agents_register.go:133`
(`if root != ""`). O marcador é **condicionalmente** verdadeiro — verdadeiro no caminho feliz, falso
no caminho de erro.

**O padrão correto já existe na própria REQ:** a correção de `syncREQReferences` declara
*"Fail closed: if projectRoot() fails we cannot verify containment"* e **aborta**. Se a precondição
da guarda falha, a escrita não acontece.

O fix estrutural é um analisador de AST — ambos os revisores (`hades-tf`, `hefesto-tf`) chegaram a
essa conclusão de forma independente.
