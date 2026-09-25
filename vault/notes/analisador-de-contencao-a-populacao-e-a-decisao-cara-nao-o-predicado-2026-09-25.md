# O caro num analisador de contenção é a POPULAÇÃO, não o predicado

> 2026-09-25 · `apolo-tf` · ML-7C da REQ-2026-08-31 · issues #400 / #402

## O problema, em uma frase

Escrever *"toda escrita é precedida pela guarda"* em AST é meia tarde. Decidir **sobre quais
escritas a frase vale** é o ML inteiro — e é onde um instrumento correto vira um gate vazio ou um
gate insuportável.

## As três populações que testei, e por que duas estão erradas

| população | escritas alcançadas (árvore de hoje) | o que acontece |
|---|---|---|
| **todo primitivo de escrita em `internal/`** | 154 | ~130 são caminho fixo/temporário. Exigiria **lista de exceção com ~130 entradas** — e uma lista desse tamanho **é** o T3 do threat model realizado, não uma defesa contra ele |
| **só quem tem taint de root** | 9 (!) | 🔴 **Perde o caso-bandeira do #400**: o ramo lefthook escreve o literal `"lefthook.yml"`, relativo, sem root nenhum na expressão. Taint puro não o vê |
| **taint ∪ "a função tem alguma guarda"** | 151 | É a que ficou |

A terceira regra é a que importa e não é óbvia: **uma função que guarda alguma coisa já declarou
que opera sobre caminhos de root do usuário; então toda escrita dentro dela está em população,
qualquer que seja a grafia do caminho.** É exatamente a forma do defeito do #400 — o bloco guarda
`.husky` e `.lefthook/commit-msg`, e escreve `lefthook.yml` na raiz.

## O segundo achado: multi-valor mata o taint em silêncio

`reqFiles, err := scanREQFiles(cfg)` tem `len(Lhs)=2` e `len(Rhs)=1`. Um `buildEnv` que só trata
`len(Lhs)==len(Rhs)` **não registra `reqFiles`**, e a partir daí a função inteira fica invisível.

Foi o que manteve `syncREQReferences` — **o marcador falso mais puro do #400, com zero chamadas a
`pathguard` na função inteira** — fora de todo relatório até eu tratar a forma. 🔴 O sintoma é
traiçoeiro: o analisador não erra, ele **cala**.

## Assimetria que não pode ser negociada

`RejectSymlinks` **sobe** a cadeia e **nunca desce**. Logo:

- guarda em `G`, escrita no **pai** de `G` → contido (o pai foi caminhado)
- guarda em `G`, escrita num **filho** de `G` → **não contido** ← é o gap de folha, 22 sítios

Um analisador que aceita as duas direções aprova os 22 defeitos que ele existe para pegar. A regra
vive em `coversComponents`, e a direção proibida tem braço de mutação próprio.

## Duas grafias do mesmo caminho: três formas legítimas

Recusar qualquer uma delas gera ruído que **empurra alguém a criar exceção** — e aí o T3 se realiza
pelo caminho honesto:

1. `RejectAndReport(root, filepath.Join(root, X))` + `os.WriteFile(X, …)` — o sítio escreve relativo
   à raiz e guarda a forma absoluta. É **a forma dominante da correção da Wave 4**.
2. `filepath.Dir(x)` é o **pai** de `x`, e `filepath.Base(x)` é o **último componente** de `x` —
   achatar os dois em componentes resolve a maioria dos falsos positivos.
3. `os.MkdirAll(".github/workflows")` tem **dois** componentes, não um: literal com `/` dentro se
   quebra antes de comparar.

## O que sobrou, e por que é honesto sobrar

20 achados na árvore de hoje, em **duas listas separadas de propósito**:

- **15 pontos cegos** — a guarda está lá e está certa; o analisador não junta as duas expressões.
- 🔴 **5 fail-open de verdade** — guarda dentro de `if root, err := projectRoot(); err == nil`, e a
  escrita **fora** do bloco. Resolvedor falha → escreve sem guarda. `RefuseUnverifiableRoot` existe
  desde o ML-7B exatamente para isso.

**Misturar as duas listas seria o erro.** *"Isto é aceitável"* e *"isto é um defeito medido que
ainda não corrigi"* são afirmações diferentes; unificá-las é como um gate vira tautologia.

## O que AST não alcança (T5) — declarado, não fingido

`projectRoot()` e `resolveRoot()` **caem para o caminho não resolvido** quando `EvalSymlinks` falha.
A provenância é estática, a degradação é dinâmica: o analisador marca as duas como produtoras
aprovadas e **nunca vê**. Fechar isso é fail-closed + teste de runtime. Nada neste analisador pode
ser lido como cobertura de T5.

## Corpus: por que ele mora em `testdata/`, e não num commit

O estado pré-fix (`87fe4915`) era alcançável por **um único ref local**, numa branch que o remoto já
apagou — que é como o `trackfw branch prune --apply` classifica *"seguro apagar"*, e como este
projeto já perdeu trabalho em 2026-09-12. `git show` também não roda num clone novo de CI.

Corpus versionado em `.go.txt` (a extensão o mantém fora do build **e** fora do
`find internal -name '*.go'` do gate bash), com `MANIFEST.sha256`. **Ausência = FAIL, nunca skip** —
um skip é indistinguível de verde, que é a falha que o ML inteiro existe para impedir.
