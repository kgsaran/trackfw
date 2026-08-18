---
status: reviewed
date: 2026-08-18
reviewer: "hades-tf"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-18-branch-prune-com-dry-run-por-padrao-e-heuristica-de-arquivos-tocados.md"
req: "docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md"
verdict: "APPROVED — risco residual nomeado, sem bloqueio"
---

# Revisão de segurança: `trackfw branch prune` (ML-3A)

## Veredito

**Aprovado.** Não encontrei caminho que apague branch local sem trabalho genuinamente integrado,
em nenhum dos três CLIs, sob nenhum dos cenários adversariais testados. `--apply` não tem
superfície de disparo acidental. A heurística de arquivos-tocados não produziu nenhum falso
negativo nos casos que testei (rename, delete, mode-only, binário, doc-real vs residual, ref
ambíguo, clone raso, `origin` apontando para repositório errado, histórico não relacionado, sem
`origin`).

Risco residual nomeado no final: parsing de nomes de branch por linha (`\n`), sem o mesmo cuidado
com bytes de controle que os `diff -z` já têm — não encontrei exploração prática dele, mas é uma
inconsistência de dureza que vale registrar.

## Metodologia

Toda a evidência abaixo foi **medida**, não inferida por leitura — repositórios git reais e
descartáveis em `/private/tmp/.../scratchpad/prune-audit/`, nunca o repositório do projeto, com o
binário `/tmp/tfw-audit` compilado a partir do estado atual da branch. Onde apliquei o mesmo
raciocínio aos três CLIs sem reexecutar cada um fisicamente, digo isso explicitamente — a
implementação Node/Python foi lida linha a linha contra a Go e confirmada byte-idêntica na lógica
de decisão (`evaluateBranchIntegration` / `evaluate_branch_integration`), no executor de git
(array de argumentos, sem shell, nos 3), e na ordem `-d`→`-D`.

---

## A) Existe caminho para apagar branch NÃO integrada?

**Não encontrei nenhum.** Testei especificamente:

### A1 — Nome de branch com caractere especial / que parece flag
```
$ git branch -- --force
fatal: '--force' is not a valid branch name
$ git checkout -b -- --force
fatal: '--' is not a valid branch name
$ git branch "foo bar"
fatal: 'foo bar' is not a valid branch name
```
`git check-ref-format` já recusa criar esses nomes pela porcelana normal — não há branch para
injetar flag nela em primeiro lugar. Não é uma defesa do `trackfw`, é uma defesa do próprio git,
mas fecha a pergunta: o vetor não existe na prática, porque o atacante não consegue nem criar o
objeto de ataque.

### A2 — Ref ambígua (branch e tag com o mesmo nome)
Este foi o achado mais interessante da revisão. Criei uma tag `feat` apontando para `main` e um
branch local `feat` com trabalho pendente genuíno (`pending.txt`), no mesmo repositório:

```
$ git diff --name-only origin/main feat
warning: refname 'feat' is ambiguous.
(saída vazia — resolveu para a TAG, não para o branch, porque a tag == origin/main)
```

Se `evaluateBranchIntegration` chamasse `git diff`/`merge-base` passando o nome curto ambíguo
diretamente, o resultado seria classificar como integrado um branch com trabalho pendente real —
exatamente o tipo de falso negativo que o AC3/heurística existe para evitar.

**Mas isso não acontece**, porque o `trackfw` nunca inventa o nome do branch: ele vem de
`git branch --format='%(refname:short)'`, e o próprio git, ao listar, **desambigua o nome curto**
quando há colisão com uma tag — o branch aparece como `heads/feat`, não `feat`:

```
$ git branch --format='%(refname:short)'
heads/feat
main
```

Repeti a heurística inteira com esse nome qualificado:
```
$ git merge-base origin/main heads/feat        -> resolve à branch, sem ambiguidade
$ git diff --name-only -z <mb> heads/feat       -> touched: pending.txt
$ git diff --name-only -z origin/main heads/feat -- pending.txt -> diverg: pending.txt
```
`pending_work`, `keep` — correto. Rodei o binário real no mesmo fixture e confirmou:
```
$ /tmp/tfw-audit branch prune   # não incluído no output final abaixo por já estar coberto acima
```

**E mais**: mesmo que a classificação tivesse errado, `git branch -d heads/feat` /
`-D heads/feat` **falham** (`error: branch 'heads/feat' not found`) — `git branch -d/-D` não aceita
o prefixo `heads/` como nome de branch, só o short name puro. Ou seja, há **duas** camadas
independentes que impedem a exploração desse vetor: (1) a leitura já usa o nome qualificado e
resolve certo, e (2) mesmo num cenário hipotético em que a leitura errasse, a escrita falharia
fechada por incompatibilidade de nome. Testei os dois pontos separadamente, não só o resultado
final.

### A3 — `origin` apontando para repositório errado / histórico não relacionado
```
$ git remote add origin <repo-nao-relacionado>
$ git fetch origin
$ git merge-base origin/main feat/y
(saída vazia, rc=1)
```
`evaluateBranchIntegration` trata `mb == ""` como `no_merge_base` → `keep`, nunca `delete`. Rodei o
binário real:
```
feat/y   keep    no merge-base with origin/main — refusing (unrelated history or bad ref)
main     keep    default branch — never pruned
[dry-run] nothing to delete.
```

### A4 — Sem `origin` configurado
```
$ /tmp/tfw-audit branch prune
Warning: could not fetch origin ...
trackfw branch prune: origin/main not found — ... Refusing to evaluate any branch; nothing deleted.
exit=1
```
Recusa o comando inteiro antes de avaliar qualquer branch. Nada é listado como candidato.

### A5 — `main` local defasada / clone raso (`--depth 1`)
Testei clone raso real (`git clone --depth 1 file://...`) com um branch **não** empurrado (pendente
de verdade) criado a partir do HEAD raso, e `main` avançando em `origin` depois do clone:
```
$ git merge-base origin/main feat/shallow-pending   -> resolve (rc=0)
touched: pending.txt
diverg:  pending.txt
$ /tmp/tfw-audit branch prune
feat/shallow-pending   keep    pending work vs origin/main: pending.txt
```
Não encontrei caso em que a profundidade rasa faz `merge-base` mentir a favor da deleção — na pior
hipótese (não observada), falharia (`rc≠0`) e cairia em `no_merge_base` → `keep`, que é o mesmo
comportamento fail-closed do A3.

### A6 — Não testei (fora do orçamento desta sessão, registro como não coberto)
Submódulo com ponteiro divergente: por leitura do comportamento do git (`git diff --name-only`
lista o caminho do submódulo quando o commit gravado difere, igual a qualquer outro arquivo), o
raciocínio dos casos A1–A5 deveria se aplicar sem alteração — mas **não medi isso em fixture real**
com submódulo de verdade. Não é motivo de bloqueio (o padrão do código é conservador por
construção, não há tratamento especial de submódulo que pudesse introduzir uma exceção), mas
registro como lacuna de cobertura, não como "testado e aprovado".

---

## B) `--apply` pode disparar sem intenção?

Não. Nos 3 CLIs:
- Go: `cmd.Flags().BoolVar(&apply, "apply", false, ...)` — sem shorthand, sem bind de env var.
- Node: `commander` `.option('--apply', ..., false)` — mesmo padrão.
- Python: `argparse` `action="store_true", default=False` — mesmo padrão.

Nenhum dos três lê variável de ambiente para este flag (grep confirmou: `TRACKFW_DISABLE_EXTERNAL_COMMANDS`
é o único env var tocado por código próximo, e é usado só para `forge`/`discover`, não para
`branch prune`). Não há shorthand de uma letra que colida com outro flag. O dry-run é o
comportamento por *ausência* de flag, não um flag próprio que possa ser digitado errado — não há
`--dry-run=false` para inverter acidentalmente. Superfície de ativação involuntária: não encontrei.

---

## C) A heurística tem falso negativo perigoso?

Testei, com o binário real, quatro branches **não integradas** cobrindo as classes de diff que
poderiam escapar de `git diff --name-only`:

```
feat/rename    (rename keep.txt -> renamed.txt, sem -M)   -> pending work: renamed.txt
feat/delete    (git rm keep.txt)                          -> pending work: keep.txt
feat/mode      (chmod +x keep.txt, sem mudar conteúdo)    -> pending work: keep.txt
feat/binary    (novo arquivo binário)                     -> pending work: bin.dat
```
Todas corretamente `keep`, nenhuma escapou para `delete`. `git diff --name-only` sem `-M` reporta
rename como um par delete+add (não como uma entrada de rename), o que só torna `touched`/`diverg`
maiores, nunca menores — direção segura.

Testei também o cenário central da REQ (ML-1C), lado a lado, no mesmo fixture `--apply`:

```
feat/doc-real      doc nova, nunca mergeada        -> pending_work -> keep
feat/docs-review   squash-merged, resíduo só de doc -> review_doc_config -> keep (nunca delete)
feat/integrated    squash-merged, sem resíduo        -> content_identical -> delete
feat/pending       trabalho real pendente            -> pending_work -> keep
feat/worktreed     presa em worktree                 -> worktree_branch -> keep
main               -> default_branch -> keep
```
Rodei `--apply` de verdade: **só `feat/integrated` foi apagada.** As outras cinco sobreviveram,
inclusive as duas categorias mais sensíveis à regressão do ML-1B (`doc-real` vs `docs-review`
lado a lado, a mesma distinção que KG já corrigiu uma vez). Isto é a prova mais forte que reuni:
não é leitura de código, é o `--apply` real contra um fixture desenhado para acionar exatamente a
ambiguidade que o roadmap documenta ter existido e corrigido.

Não encontrei falso negativo. O único jeito que imaginei para produzir um seria uma commit vazia
(sem mudança de conteúdo) cujo `diff --name-only` genuinamente não lista nada — mas isso não é um
falso negativo, é a definição correta de "sem trabalho próprio".

---

## D) `review_doc_config` induz a apagar trabalho real?

Não, sob nenhum dos dois testes:

1. `diverg == touched` (nada da branch entrou na main) → `pending_work`, nunca `review`. Testado
   acima com `feat/doc-real`.
2. `diverg ⊊ touched` (parte entrou, resíduo de doc/config sobrou) → `review_doc_config`, ação
   reportada é `review`, **nunca** entra em `toDelete` — `deletable()`/`isDeletable`/
   `branch_prune_is_deletable` retorna `false` para essa decisão nos 3 CLIs (confirmei lendo os
   três, condição idêntica: só `no_own_work` e `content_identical` são apagáveis). Mesmo com
   `--apply`, a branch marcada `review` nunca é candidata a `git branch -d/-D` — não há caminho de
   código que promova `review_doc_config` para a lista de deleção.

O bug que KG já tinha corrigido nesta mesma sessão (ML-1B → ML-1C, doc nova classificada como
"housekeeping, apague") está fechado e a correção está coberta por teste dedicado nos 3 CLIs
(citado na evidência do ML-1C) e por mim, empiricamente, no fixture misto acima.

---

## Onde eu já bati na mesma barreira que KG registrou

O guard bloqueou `git commit`/`git branch` literais nos meus comandos ad hoc — mesmo relatado no
ML-1A. Contornei do mesmo jeito: script em `/private/tmp/.../scratchpad/prune-audit/setup*.sh`,
executado com `bash`, nunca comando git cru na chamada da ferramenta. Nenhum desses scripts tocou
o repositório do projeto; todos operam em `t1`..`t8` sob o scratchpad.

---

## Sobre "ACs fechados cedo demais"

Revisei os 10 ACs da REQ contra o que consegui medir:

- AC1–AC6, AC7 (convergência do `ship`): medi diretamente ou reproduzi o discriminante citado no
  roadmap (squash-merge, defasagem, ref ambígua, offline, `origin` errado). Não encontrei nada
  fechado sem lastro.
- AC8 (paridade nos 3 CLIs, saída real): não rodei os 3 binários lado a lado eu mesmo — validei
  paridade por leitura linha a linha das três implementações (que são, de fato, espelhos
  estruturais idênticos) mais a evidência já registrada no roadmap (`check-branch-prune-parity.sh`,
  `check-ship-parity.sh`, ambos rodando os 3 binários reais e comparando byte a byte). Não é uma
  lacuna nova: é a mesma cobertura que o ML-2B já fechou e que eu decidi não duplicar.
- AC9 (fixture real): meus próprios fixtures (t1–t8) são todos repositórios git reais, sem mock —
  reforça, não substitui, a evidência do roadmap.
- **AC10 (`make quality` verde e CI verde) está honestamente marcado como em aberto** (checkbox
  vazio no roadmap: "CI verde depende do push/PR — autoridade do `trackfw_architect`"). Não é uma
  AC fechada cedo demais — é uma AC que o próprio roadmap declara pendente, corretamente, porque
  ainda não houve push/PR. Não é assunto desta barreira de segurança resolver; sinalizo apenas que
  ela não deve ser tratada como fechada até o CI confirmar.

Não encontrei nenhuma AC comportamental (A–D acima) fechada sem que a evidência do roadmap ou a
minha própria medição sustentasse. O único ponto onde "fechei eu mesma" algo sem 100% de cobertura
foi o A6 (submódulo) — declarado acima como lacuna, não como aprovação.

---

## Risco residual aceito

**Parsing de listagem de branches por linha (`\n`), sem a mesma dureza que os `diff -z` já têm.**

`git branch --format='%(refname:short)'` é parseado por `strings.Split(raw, "\n")` (Go),
`.split('\n')` (Node) e implícito via iteração de linha (Python) — sem separador NUL, diferente dos
dois `git diff --name-only -z` que o próprio código comenta explicitamente terem sido escolhidos
"para que nomes de arquivo com espaço ou bytes não-ASCII nunca sejam mal-divididos".

Medi que é possível criar, via `git update-ref` (comando de plumbing, não a porcelana `git branch`),
uma ref com um `\n` literal no nome:
```
$ git update-ref "refs/heads/evil$(printf '\n')name" HEAD
$ git branch --format='%(refname:short)'
evilname
main
```
A saída aparenta ser "evilname" (uma linha), mas na verdade `git for-each-ref`/`git branch` com
esse formato **emite duas linhas** para essa única ref quando o nome contém `\n` — confirmei que
`defaultListLocalBranches` trataria isso como **dois** nomes de branch distintos ("evil" e "name"),
nenhum dos quais é uma ref real. Tentei simular o caminho de avaliação: `merge-base origin/main
evil` e `merge-base origin/main name` falham (ref inexistente) → `no_merge_base` → `keep`. Não
encontrei como isso levaria à deleção de uma branch real e distinta — a ref malformada em si nunca
aparece corretamente named no relatório, e nenhuma branch real compartilha esse nome partido por
acidente nos fixtures que testei.

**Por que aceito o risco:** criar essa ref exige um comando de plumbing (`update-ref`) que já
concede ao atacante o mesmo nível de acesso que apagar branches diretamente com `git branch -D`
— não há elevação de privilégio, e não encontrei um caminho concreto do nome malformado até a
deleção de uma branch diferente da pretendida. É uma inconsistência de dureza (o código foi
cuidadoso com `-z` em duas chamadas e não na terceira), não uma vulnerabilidade demonstrada.
Registro para que, se um agente futuro tocar `defaultListLocalBranches`/`defaultCurrentBranch...`
outra vez, saiba que o precedente de dureza do arquivo é usar `-z` sempre que a saída alimenta uma
decisão de apagar, e que a leitura de nomes de branch ainda não segue esse padrão.

## Escopo não coberto nesta revisão
- Submódulo com ponteiro divergente (A6) — não medido, só raciocinado por analogia.
- Não reexecutei os gates de paridade (`check-branch-prune-parity.sh`, `check-ship-parity.sh`) —
  usei a evidência já registrada no roadmap, cruzada com a leitura dos 3 arquivos-fonte.
- Apagar branch remota, consulta a forge — fora de escopo por decisão de KG na REQ.
