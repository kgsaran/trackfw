# Guardrails de git e governança do trackfw — guia de reimplementação para outro harness

> Escrito em 2026-09-02 para um leitor que **não tem acesso a este repositório**: outra empresa
> está construindo um harness de agente próprio e quer portar as práticas do trackfw, sem usar o
> trackfw em si (restrição de compliance). Cada afirmação cita arquivo e linha — quem tem o
> repositório pode conferir; quem não tem, pode reimplementar a partir da descrição e dos trechos
> citados.
>
> Escopo: só o que o trackfw **instala ou provê como produto** — os comandos de git que ele
> implementa, os hooks que ele gera, as regras de `trackfw validate`. Gates de desenvolvimento
> deste próprio repositório (`quality.yml`, `scripts/check-*.sh`) ficam de fora — são meios de o
> trackfw se auto-verificar, não algo que ele entrega a um projeto que o instala.

---

## 1. A limitação que precisa vir primeiro — hook de agente ≠ hook de git

Antes de qualquer coisa: **os guardrails de git deste projeto não são hooks de git.** São hooks do
harness de agente (Claude Code), que interceptam a *ferramenta* que o agente usaria para rodar
`git`, não o `git` propriamente dito.

**Evidência medida neste repositório, hoje:**

```bash
$ ls -la .git/hooks/ | grep -v sample   # vazio — nenhum hook de git instalado
$ git config core.hooksPath
/dev/null                                # hooksPath aponta para o vazio
```

Os guardrails vivem em `.claude/settings.json`, sob `hooks.PreToolUse`, com matcher `"Bash"` —
confirmado em `internal/generators/agentfiles.go:294-306` (git-branch-guard) e
`internal/generators/agentfiles.go:253-283` (credential-guard). O gerador injeta o comando
`$CLAUDE_PROJECT_DIR/scripts/trackfw-git-branch-guard.sh` nesse array; é o Claude Code, não o git,
quem invoca o script — e só quando o próprio Claude Code decide chamar a ferramenta Bash.

**Consequência inescapável:** isto protege agentes, não pessoas. Um humano rodando `git push
--force` num terminal comum nunca passa por este script — não há absolutamente nada no lado do git
que o intercepte.

### O limite mais duro: git não tem hook pré-subcomando

Mesmo se o objetivo fosse migrar para hooks nativos de git, um limite estrutural permanece: **git
não expõe um hook que dispare antes de um subcomando arbitrário.** Os hooks nativos de git
(`pre-commit`, `pre-push`, `pre-receive`, etc.) são pontos fixos, um por operação — não existe
`pre-command` genérico. Comandos como `git stash`, `git checkout --`, `git reset --hard` **não têm
hook nativo correspondente**: não há gancho a instalar que rode antes deles.

`trackfw-git-branch-guard.sh` bloqueia exatamente essas operações (`stash`, `reset --hard`,
`clean -f`, `restore <path>`, `checkout -- <path>`, `update-ref`, `rm -f`, ver §3.2) — mas só
consegue fazê-lo porque intercepta a *chamada de ferramenta* antes do shell rodar, não porque git
oferece um ponto de interceptação para essas operações. **Isto só é alcançável com uma camada de
harness que envolve a chamada da ferramenta** (o `PreToolUse` do Claude Code, ou equivalente em
outro harness) — não com hooks de git, por mais que se tente.

Quem for portar precisa decidir, com essa distinção explícita:
- **Bloquear pessoas** → só é possível com hooks de git nativos (`pre-commit`, `pre-push`), e só
  cobre as operações para as quais git realmente expõe um gancho.
- **Bloquear agentes** → exige uma camada de harness (algo como `PreToolUse`), e aí sim cobre
  qualquer subcomando de git, porque a interceptação acontece na chamada da ferramenta, não no
  git.
- As duas coisas juntas exigem os dois mecanismos — nenhum substitui o outro.

O próprio projeto documenta essa decisão como deliberada:
`docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
— "não há prevenção contra agente induzido com escrita irrestrita; a detecção é ancorada no git".
O comentário de topo de `scripts/trackfw-git-branch-guard.sh:1-13` é explícito sobre ser uma
**tripwire, não uma fronteira de segurança**: detecta o caso óbvio (comando git literal, sem
indireção de shell); evasões que exigem tokenizar como o bash (`git${IFS}push`, `{git,push}`,
`g""it push`) permanecem abertas, por decisão documentada, não por descuido.

---

## 2. Vocabulário de comandos de git governados

O trackfw substitui a superfície crua de git por seis comandos compostos. Todos vivem em
`internal/commands/` e são replicados byte-a-byte em `npm/src/commands/` e
`pypi/trackfw/commands/` (regra de paridade do projeto — fora do escopo deste documento, mas
relevante para quem for portar: **decida se seu harness precisa da mesma superfície em múltiplas
linguagens ou só numa**).

### 2.1 `branch new <type>/<slug>` — `internal/commands/branch.go`

**O que faz:** cria uma branch (`git checkout -b <type>/<slug>`) só depois de confirmar que existe
governança para ela.

**Problema que resolve:** `git checkout -b` cru não impede ninguém de abrir uma branch de feature
sem nenhum artefato de planejamento associado — o trabalho começa "órfão", sem rastreabilidade.

**Mecanismo** (`branch.go:135-175`):
- Vocabulário fixo: `feat`, `fix`, `refactor`, `chore`, `docs` (`branch.go:19-25`).
- Só `feat`/`fix`/`refactor` são **gated** (`branchGatedTypes`, `branch.go:30-34`): exige que
  exista um roadmap cujo slug normalizado bata com o slug da branch, já em `wip/` ou `done/`
  (checado via `validator.BranchSlugMatchesRoadmap` — a mesma lógica que `trackfw validate` usa).
  Sem match, a branch nunca é criada — `git checkout -b` nem chega a rodar
  (`branch.go:153-166`) — e a mensagem de recusa nomeia o remédio (`branch.go:83-86`, reproduzido
  literalmente em runtime): `trackfw req new "title"` → `trackfw roadmap new "title"` →
  `trackfw roadmap move <name> wip`.
- `chore`/`docs` pulam esse gate — tipos de housekeeping tratados como isentos em todo o resto da
  cadeia (`branch.go:16-18`).
- Em falha do próprio `git checkout -b` (ex.: branch já existe), o processo herda o exit code real
  do git em vez de deixar o Go reformular o erro — ver `defaultGitCheckout`, `branch.go:111-131`,
  com o motivo documentado no comentário: evitar que a mensagem de erro do git seja duplicada por
  uma linha genérica "exit status 128" do runtime.

**Dependência:** um roadmap em `wip/` ou `done/` cujo nome normalizado contenha o slug da branch —
sem essa convenção de nomenclatura entre roadmap e branch, o mecanismo de match não tem o que
comparar.

### 2.2 `branch prune [--apply]` — `internal/commands/branch_prune.go`

**O que faz:** relatório (e, com `--apply`, deleção) de branches locais já integradas na branch
default, usando uma heurística que não é nem `git branch -d` nem um diff bidirecional ingênuo.

**Problema que resolve:** dois defeitos conhecidos de heurísticas mais simples, ambos documentados
em comentário no próprio código (`branch_prune.go:68-92`):
1. `git branch -d` recusa qualquer branch squash-merged, porque squash-merge nunca produz
   ancestralidade real — **falso negativo** (a branch está integrada, mas `-d` diz que não está).
2. `git diff origin/main <branch> --stat` ingênuo dá **falso positivo** quando a branch foi
   squash-merged mas está desatualizada em relação à `main` (que avançou depois) — o diff
   bidirecional mostra tudo que a `main` ganhou desde então como se fosse trabalho pendente da
   branch.

**Mecanismo** (`evaluateBranchIntegration`, `branch_prune.go:93-170`):
```
mb      = git merge-base origin/main <branch>
touched = git diff --name-only mb <branch>                       # o que a branch tocou
diverg  = git diff --name-only origin/main <branch> -- touched   # o que ainda diverge, só nesses arquivos
```
- `touched` vazio → `no_own_work`, deletável (o caso 1 acima).
- `touched` não vazio, `diverg` vazio → `content_identical`, deletável (o caso 2 acima).
- `diverg` não vazio, mas é subconjunto próprio de `touched` e só contém arquivos doc/config →
  `review_doc_config` — **nunca deletado automaticamente**, só sinalizado para revisão humana
  (`branch_prune.go:39-46`, decisão explícita: "housekeeping presumido não é housekeeping
  confirmado sem revisão").
- Caso contrário → `pending_work`, mantida.
- Sem `origin/main` resolvível (offline, sem remoto, nunca deu fetch) → o comando inteiro se recusa
  a avaliar qualquer branch e não deleta nada (`branch_prune.go:339-342`).
- Deleção real (`--apply`) tenta `git branch -d` primeiro e só cai para `-D` se `-d` recusar
  (`defaultDeleteBranch`, `branch_prune.go:482-488`) — reaproveita a checagem de ancestralidade do
  próprio git como confirmação extra, quando ela existe.
- Antes de cada deleção, re-checa se a branch virou a atual ou foi aberta em outro worktree entre o
  relatório e a execução (`branch_prune.go:401-413`) — proteção contra corrida com outro processo.

**Dependência:** um `origin/main` local resolvível (fetch prévio bem-sucedido, ou trabalho
recorrente que já fez fetch). Não faz nenhuma chamada de rede além do `git fetch` inicial (best
effort, não bloqueia se falhar) — nenhuma consulta à forja.

### 2.3 `commit -m "<msg>"` — `internal/commands/commit.go`

**O que faz:** roda `git commit -m <message>`, mas recusa antes de rodar quando a governança
está ausente, em vez de deixar o commit acontecer e só ser pego depois.

**Problema que resolve:** um `git commit` cru na branch default, ou numa branch de feature sem
roadmap correspondente, só é detectado depois — em `trackfw validate` ou em revisão de PR. Mover a
mesma checagem para antes do commit corta o ciclo de "código órfão descoberto tarde".

**Mecanismo** (`runCommit`, `commit.go:302-348`):
1. Em `main`/`master`: sempre bloqueado (`commit.go:309-317`).
2. Em branch `feat/`, `fix/`, `refactor/`: exige roadmap correspondente em `wip/` ou `done/` — a
   mesma lógica de match de `branch new` (`commit.go:319-339`).
3. Em qualquer outra branch (doc/housekeeping): permite, mas avisa (`commit.go:340-344`) — não
   bloqueia.
4. Passou: `git commit -m <message>`, propagando stdout/stderr/exit code do git literalmente
   (`defaultGitCommit`, `commit.go:136-150`).

**Recurso auxiliar, caminho totalmente separado:** `--suggest` nunca commita — lê
`git diff --cached --name-status`, classifica por uma heurística fixa de prioridade (todos os
arquivos são teste → `test`; todos são doc → `docs`; algum arquivo novo em um diretório de
comandos → `feat`; senão → `fix`; `commit.go:191-227`) e imprime um esqueleto de mensagem
Conventional Commits para revisão humana — nunca uma chamada de LLM, puramente estrutural
(`commit.go:80-84`).

**Dependência:** mesma de `branch new` — roadmap em `wip/`/`done/` com slug batendo o nome da
branch, para os tipos gated.

### 2.4 `push [--force-with-lease]` — `internal/commands/push.go`

**O que faz:** empurra commits já criados. Nunca commita, nunca abre PR.

**Problema que resolve:** separa a operação "empurrar" da operação "abrir PR" — permite reempurrar
depois de um rebase sem reabrir o fluxo de `ship` inteiro, e dá um lugar único para governar
`--force-with-lease` sem reexpor `--force` cru.

**Mecanismo** (`runPush`, `push.go:109-237`):
1. Valida o nome da branch (`feat|fix|refactor|chore|docs/<slug>`); bloqueia incondicionalmente em
   `main`/`master` (`push.go:126-140`).
2. Governança: mesmo gate hard de `commit`/`ship` para `feat/fix/refactor`; `chore/docs` pulam
   (`push.go:147-170`). É um **hard gate**: não é afetado por modo lenient nem por severidade
   por-regra configurada em `trackfw.yaml` — a mensagem de erro avisa explicitamente disso
   (`push.go:162-165`), porque `trackfw validate` sozinho pode passar (modo lenient) enquanto
   `push` ainda recusa.
3. `--force-with-lease` só roda se já existir PR/MR aberto na branch, confirmado via CLI da forja
   resolvida (`gh`/`glab`/`az`) — nunca aceita `--force` puro como flag (`push.go:172-207`,
   mensagens nomeadas em `push.go:49-55`). Sem CLI de forja disponível, ou sem PR aberto, ou sem
   conseguir verificar (erro do CLI): recusa, nunca assume "sem PR" por omissão de dado
   (`push.go:190-204` — a distinção entre "não tem PR" e "não consegui verificar" é preservada
   como erro explícito, nunca colapsada silenciosamente).
4. Detecção de squash-merge pendente em outras branches: avisa, não bloqueia (`push.go:209-219`).
5. Push, com `-u` automático se não houver upstream configurado (`buildPushArgs`, `ship.go:795-802`
   — compartilhado com `ship`).

**Dependência:** para `--force-with-lease`, um CLI de forja instalado e autenticado
(`gh`/`glab`/`az`); sem isso, o comando se recusa em vez de arriscar um force-push não confirmado.

### 2.5 `ship -m "<msg>" [--no-pr] [--force-with-lease]` — `internal/commands/ship.go`

**O que faz:** composição de commit + push + abertura de PR/MR, com todos os gates de `commit` e
`push` embutidos numa sequência única.

**Problema que resolve:** o fluxo mais comum ("terminei o ML, quero commitar, empurrar e abrir
PR") normalmente exige três comandos separados, cada um repetindo mentalmente as mesmas checagens.
`ship` fecha o ciclo de uma vez, sem tornar a governança opcional.

**Mecanismo** (`runShip`, `ship.go:246-546`, sete passos nomeados no próprio texto de ajuda do
comando, `ship.go:88-107`):
1. **Staged files** lidos uma vez no topo (`ship.go:259-261`) — determina se a mudança é
   "doc-only" (só `docs/`, `vault/`, `*.md`), reaproveitado nos passos seguintes.
2. **Validação de branch**: bloqueio incondicional em `main`/`master`; para as demais, aceita
   `feat|fix|refactor|chore|docs/<slug>` — com uma exceção: mudança doc-only é aceita mesmo fora
   desse padrão (`ship.go:278-284`).
3. **Governança**: doc-only pula o gate (mesma exceção do CLAUDE.md §7 do projeto, mapeada
   explicitamente em código — `ship.go:291-292`); `chore/docs` pulam; `feat/fix/refactor` exigem
   roadmap correspondente, hard gate (`ship.go:293-316`).
4. **`--force-with-lease` gate**: idêntico ao de `push`, roda antes de qualquer escrita
   (`ship.go:318-364`).
5. **Detecção de squash-merge pendente**: aviso, não bloqueio (`ship.go:366-376`).
6. **Revisão do staged**: imprime `git status --short` + `git diff --cached --stat` antes de
   commitar — nunca `git add .`/`git add -A` (proibido no texto de ajuda, `ship.go:101-102`, e o
   comando nunca invoca esses subcomandos em nenhum caminho do código).
7. **Commit**: exige `-m`, exceto no caso `--force-with-lease` sem nada staged (push-only pós-
   rebase, `ship.go:393-425`).
8. **Push**: mesma lógica de `push.go`.
9. **Abertura de PR/MR**: resolve a forja (flag → config → URL remota → arquivos de CI →
   `manual`), monta título (primeira linha da mensagem de commit) e corpo (agrega histórico de
   commits da branch quando há 2+, `buildPRBody`, `ship.go:619-665`); se o CLI da forja não está
   disponível, imprime a URL de fallback em vez de falhar (`ship.go:515-524`).

**Dependência:** as mesmas de `commit`+`push`; para abertura de PR, opcionalmente um CLI de forja
(sem ele, degrada para "abra manualmente" em vez de bloquear o fluxo inteiro).

### 2.6 `release tag <version>` — `internal/commands/release.go`

**O que faz:** cria e publica uma tag anotada de release via API da forja (hoje só GitHub),
preservando a anotação — algo que um `git push origin <tag>` simples perderia se a tag fosse criada
sem `-a`, e que o próprio git-branch-guard bloqueia de qualquer forma (§3.2, `push` bruto).

**Problema que resolve:** publicar uma tag errada num repositório público é um erro caro e
difícil de desfazer; o comando prefere recusar a adivinhar em qualquer das seis pré-condições
(texto de ajuda, `release.go:187-199`).

**Mecanismo** (`runReleaseTag`, `release.go:280-510`), seis pré-condições em sequência, cada
recusa nomeando o remédio:
1. Working tree limpo (`release.go:284-291`).
2. Branch default local, se existir, em dia com `origin` (`release.go:293-323`).
3. As 5 ocorrências de versão nos 3 stacks batem exatamente com a versão pedida
   (`internal/version/version.go`, `npm/package.json`, `pypi/pyproject.toml`, e as duas ocorrências
   de `pypi/trackfw/__init__.py` — `releaseVersionFiles`, `release.go:103-109`).
4. `CHANGELOG.md` tem uma seção `## [<version>] - YYYY-MM-DD`.
5. A tag não existe nem local nem remotamente (`release.go:325-332`).
6. CLI `gh` disponível e autenticado — outras forjas são recusadas com instrução de publicar
   manualmente (`release.go:334-357`).

**O padrão mais interessante deste comando: a origem da verdade do commit-alvo é a forja, nunca um
ref local.** O SHA que recebe a tag vem de duas chamadas `gh api` (`repos/{owner}/{repo}` para o
nome da branch default, `repos/{owner}/{repo}/commits/<branch>` para o SHA — `release.go:367-404`)
— um ref local (`origin/<base>`) é usado só como **cross-check opcional**, nunca como fonte: se
diverge do que a forja reporta, o comando recusa (`releaseTagCommitDivergesFmt`,
`release.go:398-400`), porque um ref local é gravável de dentro do próprio clone (`git
update-ref`) — daí o guard bloquear `update-ref` sem exceção (§3.2). O conteúdo lido para checar
versão/CHANGELOG vem de `git show <sha>:<path>` sobre esse SHA vindo da forja, nunca da working
tree (`release.go:406-439`) — um `git show` sobre objeto ausente falha nomeando exatamente o quê
está faltando (`releaseTagObjectAbsentFmt`, `release.go:86-91`), nunca cai de volta para o
arquivo local.

**Dependência:** `gh` autenticado; identidade de git configurada (`user.name`/`user.email`,
`release.go:441-448`, exigida porque a tag é anotada e precisa de tagger).

---

## 3. Os hooks gerados — mecanismo, escopo e modos de falha

### 3.1 Onde vivem e como são instalados

`internal/generators/hooks.go` detecta qual(is) dos 8 harnesses de agente suportados estão
presentes no diretório (por artefatos característicos — `.claude`/`CLAUDE.md` para Claude Code,
`AGENTS.md`/`.codex` para Codex, etc., `hooks.go:18-91`) e injeta os hooks correspondentes em cada
um. Cada harness tem sua própria convenção de arquivo e formato — a implementação real de cada
injeção vive em `internal/generators/agentfiles.go` (quase 2000 linhas, uma função `Inject*Hooks`
por harness).

Para Claude Code — o caso medido neste repositório — o destino é `.claude/settings.json`, chave
`hooks.PreToolUse`/`hooks.PostToolUse`, cada entrada com um `matcher` (nome de ferramenta:
`"Bash"`, `"Read"`, `"Write|Edit"`, `"AskUserQuestion"`) e um `command`
(`InjectClaudeHooks`, `agentfiles.go:198-340`). **Isto não é um mecanismo de git — é o formato de
configuração de hooks do harness de agente.** Outros harnesses (Codex, Gemini, Kiro, Copilot,
Cursor, Windsurf, Amazon Q) têm arquivos e formatos de payload diferentes — todos documentados em
comentário no próprio `agentfiles.go`, cada um com a mesma decisão de fundo.

Uma pegadinha específica de Claude Code, corrigida em produção
(`agentfiles.go:237-256`): o comando não pode ser um caminho relativo
(`scripts/trackfw-credential-guard.sh`) — Claude Code resolve isso contra o cwd *dinâmico* da
sessão, que muda quando o agente roda `cd`. O fix é usar `$CLAUDE_PROJECT_DIR`, uma variável de
ambiente que o harness garante fixa na raiz do projeto — e o gerador migra qualquer entrada antiga
na forma relativa antes de mesclar a nova, para não deixar duas entradas (uma quebrada, uma boa)
convivendo (`migrateHookCommand`). **Para quem for portar: se o harness alvo também injeta comando
via string de shell, confirme se ele expõe um equivalente de "raiz do projeto fixa" — a
alternativa (caminho relativo) quebra silenciosamente assim que o agente muda de diretório.**

### 3.2 `trackfw-git-branch-guard.sh` — o que intercepta e como decide

Script de 561 linhas (`scripts/trackfw-git-branch-guard.sh`), acionado como `PreToolUse` no
matcher `"Bash"`. Fluxo:

1. **Drena stdin primeiro, sempre**, antes de qualquer saída antecipada — sem isso, quem grava o
   payload no pipe recebe `EPIPE` (linhas 24-30, comentário explica que é reprodutível em 100% das
   chamadas fora de projeto trackfw, não é corrida).
2. **No-op fora de projeto trackfw**: sobe diretórios a partir do cwd físico (`pwd -P`, resolve
   symlink) até achar `trackfw.yaml`; sem ele em nenhum ancestral, sai com 0 sem fazer nada
   (linhas 32-49). Decisão documentada em
   `docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md`: fora de
   projeto trackfw não existe `trackfw ship` como alternativa, então bloquear ali não tem
   contrapartida.
3. **Extrai o comando bruto** do payload JSON do `PreToolUse` (campo `tool_input.command`, com
   vários fallbacks de parsing — `jq` se disponível, senão `sed`, linhas 55-79).
4. **Pré-processamento anti-falso-positivo**: neutraliza `;`, `&&`, `||`, `|` e quebras de linha
   quando estão dentro de aspas ou de corpo de heredoc, para que uma linha de mensagem de commit
   que comece com "git ..." não seja lida como um segundo comando (`strip_heredoc_bodies`,
   `quote_aware_split`, linhas 100-220).
5. **`match_subcommand`** (linhas 222-410): varre cada segmento resultante, resolve `env`/`command`
   como prefixo (sem flags — `env -i`/`command -p` continuam evadindo, declarado, não fechado),
   confirma que o primeiro token é `git` (por basename) e então casa contra uma lista fixa de
   subcomandos perigosos, cada um com sua própria lógica de exceção — por exemplo `git reset` só
   bloqueia com `--hard` (`--soft`/`--mixed` seguem liberados, é o contorno padrão para reempurrar
   trabalho staged); `git branch` só bloqueia criação/rename, leitura (`-a`, `-r`, `--list`, etc.)
   segue liberada; `git checkout <branch>` sem `--`/`.` segue liberado, só a forma que descarta
   caminho é bloqueada.
6. Em match: bloqueia com `exit 2` e um payload `{"decision":"block","reason":"..."}` (formato que
   Claude Code reconhece), nomeando **o comando trackfw equivalente** em cada mensagem — nunca só
   "bloqueado" (linhas 455-505, ex.: `git push` bruto → "use `trackfw push`/`trackfw ship`/`trackfw
   release tag`"). Sem match: `exit 0`, silencioso.

**Cobertura além de commit/push/checkout -b** (a superfície que dá nome ao script é maior do que o
nome sugere): `git switch -c`, `git stash` (qualquer forma exceto `list`/`show` — decisão
explícita: worktree compartilhado entre subagentes paralelos, um stash de um agente some com o
trabalho staged dos outros), `git reset --hard`, `git clean -f`/`-x` (exceto `--dry-run`), `git
restore <path>` sem `--staged`, `git checkout -- <path>`/`git checkout .`, `git update-ref`
(sempre, sem exceção — é o vetor que permitiria forjar o commit-alvo que `release tag` publicaria,
ver §2.6), `git worktree remove -f`, `git rm -f`.

**Modo de falha:** *fail-open por design* em dois eixos, ambos documentados:
- Fora de projeto trackfw → no-op (§ acima).
- Evasão por tokenização de shell (`git${IFS}push`, aspas quebradas propositalmente,
  `env -i git push`) → não detectada, por decisão do ADR-2026-08-12 já citado.

Se o **script sumir do disco** (apagado, ou o próprio comando de instalação nunca rodou), o
resultado depende inteiramente do harness: para Claude Code, um `command` que não resolve para um
arquivo existente falha ao ser invocado — mas isso só bloqueia se o harness tratar "hook não
resolvível" como recusa da ferramenta, não como no-op. **`trackfw validate` tem uma regra dedicada
para essa lacuna** — ver §4.2 (`git_branch_guard_hook_resolvable`,
`git_branch_guard_script_integrity`): a garantia de que o hook existe e não foi adulterado não vem
do próprio hook (que não pode se auto-verificar se sumiu), vem de uma checagem externa e separada.

### 3.3 `trackfw-credential-guard.sh` — o que intercepta e como decide

Script de 150 linhas (`scripts/trackfw-credential-guard.sh`), acionado em `PreToolUse` E
`PostToolUse`, matchers `Bash`, `Read`, `Write|Edit`. Fluxo:

1. No-op fora de projeto trackfw (`[ -f "trackfw.yaml" ] || exit 0`, linha 8) — mesma decisão do
   git-branch-guard, versão mais simples (não resolve symlink, porque o teste é só presença do
   arquivo no cwd, não uma comparação de caminho).
2. Varre o payload por padrão de JWT (`eyJ...`) ou de AWS access key (`AKIA...`, linhas 10-17).
3. **Ignora matches redirecionados só para destino efêmero** (`/dev/null`, ou um path derivado de
   `mktemp`) — um match sem redirect nenhum (ex.: impresso no stdout) ainda alerta; é exatamente o
   caso que o hook existe para pegar (linhas 26-46).
4. Segunda camada: se a varredura do payload não achou nada mas o comando tem `cat`/`head`/
   `tail`/`jq`/`grep` de um arquivo, lê o arquivo (até 1 MiB) procurando o mesmo padrão
   (`scan_file_for_pattern`, linhas 52-68) — cobre o caso onde a credencial não está no payload em
   si, mas o comando vai expô-la ao ler um arquivo que a contém.
5. **Modo configurável, padrão `warn`** (linhas 118-124): lê `credential_guard.mode` de
   `trackfw.yaml`; qualquer valor diferente de `warn`/`block` cai para `warn`. Em `block`: `exit 2`
   com mensagem em stderr — bloqueia a ferramenta. Em `warn` (o padrão): `exit 0` — a ferramenta
   roda normalmente — mas grava um arquivo de atenção
   (`<roadmap_dir>/.trackfw-credential-guard.json`) que o `trackfw serve` mostra como banner
   (linhas 126-150).

**Isto é o padrão "gate nasce desligado" no seu ponto mais explícito neste projeto**: por padrão,
o guard detecta e **avisa**, não bloqueia. Bloquear exige opt-in explícito
(`credential_guard: {mode: block}` em `trackfw.yaml`). Quem for portar esse controle precisa
decidir conscientemente qual padrão quer — um guard de segurança que nasce em modo "aviso apenas"
é uma escolha de produto (menos fricção por padrão), não uma obviedade, e vale documentar por que
foi essa a escolha aqui em vez do inverso.

---

## 4. Regras de `trackfw validate` — agrupadas por tema

`internal/validator/validator.go` (2596 linhas) mais 8 arquivos satélite implementam ~25 regras
nomeadas (algumas com subtipos, como `traceid_*`), todas registradas em
`validateUnfilteredTagged` (`validator.go:619-816`) sob um nome de regra (`applyRuleTagged`) que
`trackfw.yaml` pode reclassificar como violação (erro, bloqueia) ou aviso (não bloqueia) — modo
`lenient` do projeto rebaixa severidade globalmente, exceto para os hard gates explicitamente
isentos disso (`branch_has_wip_roadmap` em `ship`/`push`/`commit`/`branch new`, ver §2).

### 4.1 Cadeia de governança ADR → REQ → Roadmap

| Regra | O que verifica |
|---|---|
| `wip_has_req` | todo roadmap em `wip/` tem uma REQ vinculada |
| `req_has_adr` | toda REQ referencia um ADR |
| `blocked_has_req` | todo roadmap em `blocked/` tem uma REQ vinculada |
| `req_has_roadmap` | toda REQ tem um roadmap vinculado |
| `adr_orphan` | todo ADR é referenciado por ao menos uma REQ |
| `adr_accepted_when_req_done` | REQ concluída exige ADR em estado "Accepted", não rascunho |
| `blocked_by_draft_adr` | REQ não pode depender de ADR ainda em rascunho |
| `req_roadmap_lifecycle` | consistência de transições de estado entre REQ e Roadmap |
| `folder_status` | a pasta onde o artefato está bate com o `status:` do frontmatter |
| `filename_uniqueness` | nomes de arquivo não colidem entre namespaces |

### 4.2 Integridade dos hooks (a contraparte de §3)

| Regra | O que verifica |
|---|---|
| `credential_guard_hook_resolvable` / `git_branch_guard_hook_resolvable` | o hook está registrado no arquivo de config do harness (`.claude/settings.json` etc.) E o script que ele referencia existe e é executável — cobre tanto escopo de projeto quanto global |
| `credential_guard_script_integrity` / `git_branch_guard_script_integrity` | o conteúdo do script no disco bate com o conteúdo de referência que o trackfw instalou — detecta adulteração |
| `credential_guard_mode_downgrade` | detecta se o `mode` do credential guard foi rebaixado de `block` para `warn` (ou removido) entre commits — regressão silenciosa de postura de segurança |

Este grupo existe precisamente porque um hook não pode se auto-auditar se foi apagado ou
adulterado (§3.2) — a garantia vem de uma checagem externa, rodada por `trackfw validate`, não pelo
hook.

### 4.3 Rastreabilidade (`traceid_*`)

`req_id` explícito no frontmatter, ligando REQ e Roadmap além da convenção de nome de arquivo:
`traceid_duplicate_req`, `traceid_duplicate_roadmap` (mesmo id em mais de um artefato),
`traceid_orphan_roadmap`, `traceid_orphan_req` (id sem contraparte), `traceid_state_mismatch`
(REQ e Roadmap com o mesmo id em estados diferentes).

### 4.4 Higiene de conteúdo

`wip_acceptance` (todo ML em `wip/` tem critério de aceite mensurável — texto vazio ou header
presente sem nenhum item não conta, mesmo padrão de "guarda de vacuidade" do `barrier`, §5.2),
`stale_wip` (roadmap parado em `wip/` há tempo demais sem atividade), `wip_limit` (limite
configurável de roadmaps simultâneos em `wip/` — 1 por padrão), `ref_targets_exist` (links internos
entre documentos apontam para arquivos que existem), `note_orphan` (nota do vault não referenciada
em nenhum `index.md`), `adr_dir_exists` (os diretórios declarados em `adr_dirs` do
`trackfw.yaml` realmente existem), `agent_namespace_undeclared` / `agent_namespace_hidden`
(namespace de agente em disco não declarado em `agents:`, ou nome oculto/ambíguo — aviso de baixo
ruído, nunca erro).

### 4.5 Cadeia de suprimento de terceiros

`thirdparty_artifact_has_provenance`: um artefato de terceiro instalado
(`trackfw skills third-party install`) precisa ter um registro de aprovação de provenance
(`.trackfw/thirdparty-provenance.json`) vinculado ao checksum exato do conteúdo em quarentena —
sem isso, `validate` reprova. Documentado em profundidade em `docs/cli-parity.md`.

---

## 5. Padrões recorrentes que atravessam o projeto

### 5.1 Ausência de medição não é medição favorável

`trackfw barrier` (avaliação de gates declarados num roadmap, `internal/commands/barrier.go`)
distingue três estados, não dois: `passed`, `blocked`, e **`not_evaluated`** — este último quando
`sh` não está disponível no `$PATH` para sequer rodar o comando do gate
(`evalGateCommands`, `barrier.go:760-778`, mensagem fixa `shMissingMsg`, linha 730, igual nos 3
runtimes por contrato de paridade). `not_evaluated` sai com exit code distinto de `blocked`
(`usageExit`, exit 2, vs. `blocked`, exit 1 — comentário explícito: "um barrier que não pôde ser
avaliado não é o mesmo que um que avaliou para falha", linhas 784-786). O mesmo terceiro estado
existe em `trackfw doctor --remote` — quando não há credencial para consultar a forja
remotamente, o resultado é "não avaliado", nunca "aprovado" por omissão.

**A regra geral, para quem for portar:** um controle que não conseguiu rodar deve dizer "não sei",
nunca "ok" — colapsar os dois é a forma mais comum de um gate parecer verde sem ter medido nada.

### 5.2 Guarda de vacuidade

`acceptanceEvaluate` (`barrier.go:534-587`) trata um bloco de critérios de aceite presente mas
**vazio** (header "Critérios de aceite:" sem nenhuma linha `- [...]` abaixo) como
`hasBlock=false` — "não vacuamente aprovado", no comentário do próprio código (linha 537). Um ML
sem nenhum critério real não passa por omissão; ele é tratado como se não tivesse bloco de aceite
nenhum. O mesmo raciocínio aparece em `validateWIPHasAcceptanceCriteria` (§4.4) e, em espírito, no
próprio design do `thirdparty_artifact_has_provenance`: quarentena sem aprovação nunca é tratada
como aprovação por padrão.

Contraexemplo interessante, também no próprio código: `parseGates` documenta explicitamente que
**zero gates declarados numa wave é legal** — "o barrier nunca inventa um gate"
(`barrier.go:589-590`). A distinção entre os dois casos é o que importa: vacuidade em um campo que
deveria conter evidência (critérios de aceite) reprova; ausência de um campo opcional por natureza
(gates da wave) não é penalizada. Uma guarda de vacuidade mal desenhada trataria os dois iguais.

### 5.3 Falsificação nas duas direções

Cada controle relevante deste projeto é testado provando duas coisas, não uma: que ele **detecta**
o defeito que existe para pegar, e que ele **não dispara** no caso legítimo correspondente. O
padrão está espalhado pelos nomes de teste em `internal/generators/` (`credential_guard_test.go`,
`credential_guard_sabotage_test.go`, `git_branch_guard_test.go`,
`git_branch_guard_dedup_test.go`) e é citado explicitamente na convenção de nomeação de commits e
ADRs deste próprio repositório (ex.: `dc89d91 feat(gate): regra do hook relativo falsificada nas
duas direções`, no histórico git). Para quem for portar: um gate provado só no sentido positivo
("bloqueia quando deveria") é metade da prova — a outra metade ("não bloqueia quando não deveria")
é onde a maioria dos falsos positivos em produção realmente mora.

### 5.4 Mensagem que nomeia o remédio

Toda recusa deste projeto — nos 6 comandos de git (§2), no git-branch-guard (§3.2), nas regras de
`validate` (§4) — nomeia o comando exato que resolve a recusa, nunca só "bloqueado" ou "inválido".
Exemplos citados ao longo deste documento: `branch new` sem match aponta os 3 comandos exatos para
criar a governança (`branch.go:83-86`); `git push` bruto bloqueado pelo guard nomeia os 3
substitutos possíveis (`trackfw push`/`ship`/`release tag`); `release tag` recusa cada uma das 6
pré-condições nomeando o arquivo e o valor esperado, nunca "versão incorreta" genérico. É um
padrão consciente, não incidental — vale a pena portar como convenção de time, não só como
propriedade emergente de um comando específico.

---

## 6. O que eu faria diferente

Este documento não é só elogio — o próprio código e o vault de conhecimento do projeto registram
dívidas conhecidas que não valeria a pena reproduzir sem correção:

1. **`credential_guard` nasce em modo `warn`, não `block`** (§3.3, `trackfw-credential-guard.sh:
   118-124`). Um controle de segurança cujo padrão de fábrica é "avisar e deixar passar" é uma
   escolha de produto legítima (menos fricção), mas ela é fácil de esquecer que foi feita — quem
   portar deveria decidir esse padrão conscientemente, documentá-lo tão explicitamente quanto o
   resto do design, e considerar se o contexto do harness-alvo (não um projeto open-source com
   contribuidores externos, mas um ambiente corporativo interno) pede o inverso.

2. **O trust-check de `barrier` tem um fail-open real, não declarado, causado por resolução de
   caminho** (`vault/notes/barrier-trust-check-fail-open-em-tmpdir-simbolico-2026-08-29.md`):
   `roadmapTrustForGates` compara um caminho resolvido por `git rev-parse --show-toplevel`
   (sempre físico, resolve symlink) contra um caminho de `filepath.Abs` (não resolve symlink). Em
   qualquer ambiente onde o diretório de trabalho atravessa um componente simbólico — `$TMPDIR` no
   macOS por padrão, ou um `$HOME` montado via automounter/NFS em setups corporativos — a
   comparação falha de um jeito que o código não reconhece, e cai no branch genérico "qualquer
   outra falha → fail-open" (`return gatesTrustVerdict{trusted: true}`). Isto é exatamente o tipo
   de bug que o §5.1 deste documento existe para prevenir — mas o próprio mecanismo criado para
   aplicar esse princípio tem uma exceção não coberta. Não corrigido no repositório até a data
   deste documento (nota classifica como "deveria virar REQ própria"). **Para quem portar:**
   qualquer trust-check que compare dois caminhos calculados por rotinas diferentes precisa
   resolver symlinks dos dois lados antes de comparar — a lição não é específica deste bug, é
   genérica para qualquer código que misture `git rev-parse --show-toplevel` com `filepath.Abs`.

3. **`release tag` só suporta GitHub** (`release.go:347-351` — outras forjas são recusadas com
   instrução de publicar manualmente). Aceitável como decisão de escopo do trackfw em si (produto
   real, cobertura incremental), mas quem for portar para um harness corporativo precisa checar
   qual forja o ambiente realmente usa antes de assumir que o padrão de design (SHA vindo da forja,
   nunca de ref local — §2.6) já vem pronto para GitLab/Azure/Bitbucket. O padrão é portável; a
   implementação, hoje, não é.

4. **A tripwire do git-branch-guard é conhecidamente evadível por um agente adversarial
   competente** (§1, §3.2) — declarado no próprio ADR-2026-08-12, não uma surpresa. Mas vale
   repetir para quem for portar sem ler o ADR: um agente induzido a evadir (por prompt injection,
   por exemplo) pode compor o comando via `env -i`, `$IFS`, concatenação de string, ou qualquer
   tokenização que o shell aceite mas o regex do guard não reconheça. **A defesa real contra esse
   cenário não está no guard — está em nunca dar ao agente um caminho de escrita irrestrita sem
   revisão**, que é exatamente o argumento do ADR. Um harness que trate o guard como suficiente,
   sozinho, está reproduzindo a mesma lacuna que este projeto decidiu conscientemente deixar
   aberta — só que sem ter decidido conscientemente.

5. **A distinção entre "hook de agente" e "hook de git" (§1) não estava documentada em lugar
   nenhum deste projeto antes de hoje** — foi medida nesta sessão de documentação, não encontrada
   em um ADR pré-existente. Isso por si só é um sinal: um projeto pode construir uma superfície de
   guardrails sofisticada (tripwires falsificadas nas duas direções, integridade de script
   verificada por `validate`, mensagens que nomeiam o remédio) inteiramente dentro do escopo
   "protege agentes" sem que isso fique explícito em nenhum lugar — até alguém medir `.git/hooks/`
   e perceber que está vazio. **Para quem for portar: documente essa fronteira no primeiro ADR do
   sistema de guardrails, não como uma descoberta tardia.**
