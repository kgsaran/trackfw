# O censo de sítios mudos por NOME perde o que está dentro do próprio `pathguard` — e o adaptador que delega mas re-embrulha dobra a mensagem

> ML-7B · REQ-2026-08-31 · `apolo-tf` · 2026-09-25

## Contexto

O `ML-6A` mediu **53 implementações** do par *predicado + recusa audível* (4 helpers nomeados +
49 pares inline), **5 gramáticas** de mensagem e **3 sítios mudos**
(`manager.go:762`, `roadmap.go:829`, `req.go:474`). O `ML-7B` extraiu
`pathguard.RejectAndReport` como emissor único.

## Achado 1 🔴 — os sítios mudos eram **4**, e o quarto é `GuardedWrite`

A régua do censo foi *"chamada de `pathguard.RejectSymlinks` fora de `pathguard` sem
`Fprintf(os.Stderr, …)` por perto"*. Ela **exclui por construção** o próprio `pathguard`, e
`GuardedWrite` chamava `RejectSymlinks` e devolvia o erro **sem dizer nada**. Os três chamadores
(`internal/identity/identity.go:86`, `internal/thirdparty/quarantine.go:104`,
`internal/thirdparty/provenance.go:146`) apenas embrulham o erro — nenhum imprime.

Ou seja: uma recusa de contenção em qualquer escrita atômica do binário chegava ao usuário **muda**,
e o censo não podia vê-la porque a isenção do emissor foi aplicada **antes** da medição, não depois.

**Regra prática:** quando um censo isenta um pacote por ser "o lugar certo", medir esse pacote
**separadamente**. A isenção é sobre *onde a correção mora*, não sobre *onde o defeito pode estar*.

## Achado 2 🔴 — adaptador que delega **e** re-embrulha dobra a mensagem

`rejectScaffoldPath` (`scaffold.go`) e `rejectDiscoverPath` (`commands/discover.go`) eram cópias
byte-idênticas do par. A conversão "óbvia" é trocar só a chamada:

```go
if err := pathguard.RejectAndReport(root, absTarget); err != nil {
    return fmt.Errorf("refusing write to %s: %w", absTarget, err)   // ← ERRADO
}
```

`RejectAndReport` **já** devolve `refusing write to <alvo>: <causa>`. O resultado é
`refusing write to X: refusing write to X: refusing symlink path "…"`. O build passa, os testes
existentes passam (todos usam `strings.Contains`), e a duplicação só aparece para o usuário.

A forma correta do adaptador é `return pathguard.RejectAndReport(root, absTarget)` — sem `if`.
Mesma armadilha em `sync.go` (`fmt.Errorf("refusing write: %w", guardErr)`),
`configure.go` (`"refusing write to trackfw.yaml"`) e `java.go` (`"refusing write to pom.xml"`).

## Achado 3 ⚠️ — como um teste dos sítios mudos degenera e fica verde sem medir

Os dois logs de transição envolvem a guarda em:

```go
if root, err := projectRoot(); err == nil { … }
```

Se a fixture não faz a guarda ser **alcançável**, `stderr` fica vazio **antes e depois** da correção,
e o teste "afirma que a recusa é audível" passa por vacuidade. É a mesma classe do mutante do
`ML-7A` que degenerava porque o diretório da fixture não existia.

**Braço de controle obrigatório**: rodar o mesmo helper numa árvore **limpa** e exigir
(a) `stderr` vazio **e** (b) que o arquivo de log **tenha sido escrito**. O (b) é o que prova que o
fluxo chegou até a guarda. Sem ele, o braço barulhento não significa nada.

Detalhe de fixture: o caminho do log vem de `config.Load()`, que é `sync.Once` — não presumir
`docs/roadmaps`. Derivar o **primeiro segmento** de `logPath()` / `config.Load().REQDir` em tempo de
teste e plantar o symlink-isca nesse segmento.

## Achado 4 — "não fatal" é controle de fluxo, não licença para gramática própria

`discover.go:355` (`writeCIWorkflow`) mantinha a 5ª gramática (`aviso: … não escreve através de
symlinks`) **porque retorna `nil`**. As duas coisas são independentes: o contrato não fatal é
preservado devolvendo `nil` depois de `RejectAndReport`, que já falou.

Mesma coisa em `update.go`: o ramo `Lstat` de folha com `aviso: %s é um symlink` era uma 6ª
gramática para um resultado que `RejectSymlinks` **já produz** (ele faz `Lstat` na folha antes dos
ancestrais). Ramo removido, comportamento idêntico.

## Achado 5 — identidade "por construção" morre num `%w` descuidado

`fmt.Fprintf` não aceita `%w`; `fmt.Errorf` precisa dele para embrulhar. Escrever dois literais
completos recria a coincidência textual que o AC5 proíbe. A forma que sobrevive ao `go vet`
(que reprova format string não constante) é uma const **sem o verbo final** e a concatenação
constante no sítio:

```go
const refusalGrammar = "refusing write to %s: "
fmt.Fprintf(os.Stderr, stderrPrefix+refusalGrammar+"%v\n", absTarget, guardErr)
return fmt.Errorf(refusalGrammar+"%w", absTarget, guardErr)
```

## Limite do instrumento entregue

`internal/pathguard/single_emitter_test.go` prova que **nenhum sítio reimplementa o par**. Ele
**não** prova que o sítio ainda **age** sobre o erro devolvido — `if err != nil { }` com corpo vazio
passa. Essa é a propriedade *guarda-antes-de-escrever*, e é o `ML-7C`.
