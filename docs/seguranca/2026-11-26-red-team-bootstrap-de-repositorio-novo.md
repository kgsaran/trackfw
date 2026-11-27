# Red Team — Bootstrap de Repositório Novo
**REQ:** REQ-2026-11-26 | **ML:** 2B | **Binários:** 11.9.1 (before=1650d0c6) e HEAD (after=d6350cae)
**Data:** 2026-11-26 | **Revisor:** Hades

---

## 0. Escopo e Metodologia

Medição de todos os vetores do Wave 0 (`docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md`)
contra dois binários compilados em isolamento:

- **before** (`tf-before`): revisão 1650d0c6 — tag 11.9.1, antes da REQ
- **after** (`tf-after`): revisão d6350cae — HEAD da branch fix

Ambiente: `HOME` isolado no scratch (`$FH`); `CODEX_HOME`, `XDG_CONFIG_HOME`, `USERPROFILE` não
foram definidos explicitamente (desvio parcial do protocolo que pedia os quatro na mesma linha;
confirmado que HOME=$FH foi suficiente: `find $FH -maxdepth 2` mostra apenas o diretório vazio —
nenhum dotfile escrito; `grep user.Current` em `internal/` e `cmd/` não retornou nada — trackfw
não resolve home fora de `$HOME`). Remotos bare locais via `file://`; nenhuma escrita no
repositório do trackfw; nenhum acesso a rede ou GitHub real.

Scripts de medição:
`measure_d1.sh`, `measure_d2.sh`, `diag_d2.sh`, `diag_d2b.sh`, `measure_d3_d4.sh`,
`d3_6fix.sh`, `measure_d6.sh`, `d6_wrapper.sh`, `d6_wrapper2.sh`, `d6_4b.sh`,
`measure_gaps.sh`, `measure_gaps2.sh`, `measure_gaps3.sh`, `measure_gaps4.sh`,
`measure_gaps5.sh`, `measure_gaps6.sh`, `measure_toctou.sh`, `measure_toctou2.sh`,
`measure_toctou3.sh`, `measure_gaps7.sh`, `measure_gaps7b.sh`, `measure_gaps8.sh`

---

## 1. D1 — git init detection (runGitInitForBootstrap)

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D1-1a: before NÃO cria .git em dir fresco | 0 (não criou) | — | PASS |
| D1-1b: after CRIA .git em dir fresco | — | criou | PASS |
| D1-1c: after imprime instruções de bootstrap (PrintBootstrapSteps) | — | texto presente | PASS |
| D1-2: init dentro de .git/ recusado (rc!=0, sem nested .git) | qualquer | rc!=0, sem nested | PASS |
| D1-3: init dentro de repo bare recusado (rc!=0, sem nested .git) | qualquer | rc!=0, sem nested | PASS |
| D1-4: init em subdir de repo pai — absorção (rc=0, sem .git aninhado) | qualquer | rc=0 | PASS |
| D1-5: HOME=git-repo, cwd=fresco — HOME não interfere | — | rc=0, .git criado | PASS |

### Achado D1-6 (LOW — injeção em conselho impresso)

**Medido:** em `d1_inj_*` e confirmado em `d16_exec_*` e `d16v2_*`.

`PrintBootstrapSteps` imprime uma linha `git add` com nomes de arquivo sem escapamento de shell.
Com arquivos cujos nomes contêm `$(touch PWNED)` e `a;rm -rf .`:

```
git add "$(touch PWNED)" -A .claude .gitattributes ...
```

Executar essa linha em bash confirma injeção:
```bash
eval 'git add "$(touch PWNED)"'   # → arquivo PWNED criado no diretório corrente
```
Medido: `PWNED` foi criado em duas verificações independentes.

Notas adicionais:
- O arquivo nomeado `-A` torna-se a flag `--all` no `git add`, potencialmente adicionando
  conteúdo não intencionado. Fix: adicionar `--` antes dos pathspecs.
- **Mitigação:** `fmt.Sprintf("%q", path)` **NÃO é suficiente** — aspas duplas no bash não
  impedem a expansão `$(...)`. `git status --porcelain` (sem `-z`) não protege contra nomes de
  arquivo com `$(...)` porque git não C-quote `$`. Fix correto: usar `git status -z --porcelain`,
  NUL-separar os paths, single-quote-escapar cada path (`'` → `'\''`), e adicionar `--` antes dos
  pathspecs. Ou substituir o conselho por `git add .` (que não enumera nomes individualmente).

Localização: `internal/commands/init.go` (`parseGitStatusPorcelain`, `defaultInitGitStatus`);
`internal/generators/nextsteps.go` (`PrintBootstrapSteps`).

Severidade: **LOW**. O cenário típico de bootstrap tem um diretório vazio; os arquivos perigosos
precisariam ser plantados ANTES do `trackfw init`. Na prática do trackfw, o `init` é o primeiro
comando em um diretório recém-criado.

---

## 2. D2 — Bootstrap first commit + branch new

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D2-1a: branch new unborn HEAD (sem --from): after bloqueia | qualquer | rc!=0 | PASS |
| D2-1b: mensagem menciona bootstrap/unborn | — | presente | PASS |
| D2-2: branch new --from HEAD unborn: after bloqueia | qualquer | rc!=0 | PASS |
| D2-4: primeiro commit em main unborn: before bloqueia, after PERMITE | rc!=0 | rc=0 | PASS (exceção D2) |
| D2-5: segundo commit em main (HEAD born): after bloqueia | qualquer | rc!=0 | PASS |
| D2-7a: re-init permite commit (count=0 no repositório recriado) | — | rc=0 | PASS |
| D2-7b: re-init depois push — D3 bloqueia (bare já tem main) | rc=0 | rc!=0 | PASS |

### D2-3 — diagnóstico, sem achado de D2

`branch new feat/x --from HEAD` com HEAD born retorna rc=1 em ambos os binários. Causa: o gate de
governança (sem roadmap WIP para o tipo `feat/`) dispara antes do D2. Com tipo `chore/bootstrap`
(isento de roadmap), born HEAD + sem origin → rc=0 ✓. D2 não bloqueia `branch new` quando HEAD está
born.

### D2-6 — diagnóstico, comportamento correto

Commit em `orphan-branch` (nome explícito, não `main`/`master`) retorna rc=0. Correto:
`commitProtectedBranches` cobre apenas `main` e `master`. O guard D6 bloqueia a criação da branch
orphan; chegar a um commit nela já requer bypass do guard. Comportamento correto.

### M-X1 — D2 robusto; achado LOW de orientação sem passo de saída

Setup: repo com 1 commit em `main`; `git switch --orphan master` cria master como branch orphan
(unborn), com 1 commit em `main` já no repositório.

Precondições verificadas antes de qualquer assertion:
```
rev-parse --verify -q HEAD   → rc=1  (master unborn) ✓
rev-list --all --count        → 1    (main tem 1 commit) ✓
symbolic-ref HEAD             → refs/heads/master ✓
```

**D2 é robusto para esta forma de orphan-switch:**
- `trackfw commit` em master (unborn, count=1): rc=1,
  mensagem verbatim: "direct commit on master is not permitted. Bootstrap in progress? Use
  `trackfw branch new` to create a branch from the current HEAD." — `isBootstrapFirstCommit` retorna
  false (count≠0) → D2 bloqueia ✓
- Commit count após tentativa: 1 (invariante preservado) ✓
- `trackfw branch new chore/x` em master unborn: rc=1,
  mensagem verbatim: "cannot create a branch from an unborn HEAD. Complete the bootstrap first
  (run `trackfw push` from main or master)" ✓

### Achado M-X1 (LOW — sem passo de saída imediata)

`branch new` menciona "run `trackfw push` from main or master" mas o usuário está em `master`
(unborn) — `trackfw push` de master unborn falha porque não há commit para publicar. Nenhuma
mensagem indica `git switch main` para retornar à branch existente.

Fluxo bloqueado: `commit` → "use branch new" → `branch new` → "push from main or master" (master
é unborn, push falha) → sem saída via trackfw; exige `git switch main`.

Mitigação: adicionar à mensagem de `branch new` em estado unborn-com-commits-existentes:
"To return to an existing branch, run `git switch <branch>`."

Severidade: **LOW** (orientação; nenhum controle de segurança contornado).

---

## 3. D3 — Push bootstrap (branch default para remote vazio)

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D3-1a: main+1commit+remote_vazio: before bloqueia, after PERMITE | rc!=0 | rc=0 | PASS |
| D3-1b: commit foi realmente entregue ao bare após rc=0 | — | main no bare | PASS |
| D3-2: main+1commit+remote_tem_main: after bloqueia | rc!=0 | rc!=0 | PASS |
| D3-3: main+2commits+remote_vazio: after bloqueia (single-commit gate) | rc!=0 | rc!=0 | PASS |
| D3-4: remote ausente/inacessível: after fail-closed | rc!=0 | rc!=0 | PASS |
| D3-4 mensagem: "Bootstrap push failed: could not check if the remote is empty" | — | presente | PASS |
| D3-5: --force-with-lease recusado antes da consulta | rc!=0 | rc!=0 | PASS |
| D3-5 mensagem: "--force-with-lease is not allowed during bootstrap" | — | presente | PASS |
| D3-6: segundo push a remote que já tem main: bloqueado | rc!=0 | rc!=0 | PASS |
| D3-7a: pushurl=tem_main, url=vazia: after bloqueia (itera pushurls) | rc!=0 | rc!=0 | PASS |
| D3-7b: url=tem_main, pushurl=vazia: after PERMITE (FP counter-arm) | rc!=0 | rc=0 | PASS |
| D3-8: remote só com tags: after PERMITE (nenhuma head = "remoto vazio") | rc!=0 | rc=0 | PASS |
| D3-9: remote com 'develop' (não main): after bloqueia (não vazio) | rc!=0 | rc!=0 | PASS |
| Multi-pushurl B1: PP1=vazio, PP2=vazio: after PERMITE | — | rc=0 | PASS |
| Multi-pushurl B2: PP1=vazio, PP2=tem_main: after bloqueia | — | rc!=0 | PASS |
| Multi-pushurl B3: PP1=tem_main, PP2=vazio (ordem invertida): after bloqueia | — | rc!=0 | PASS |

Notas:
- **D3-6**: a medição em `measure_d3_d4.sh` reportou FAIL por reutilização de variável (`$BARE`
  apontava para um bare vazio). Confirmado correto via `d3_6fix.sh` com `bare_has_main`: rc=1 ✓.
- **D3-8**: ADR L67–69 é explícito: condição é "remoto vazio de heads". Tags-only = vazio. ✓
- **Multi-pushurl**: `checkRemoteEmpty` itera todas as pushurls; basta uma ter branches para
  bloquear; a ordem das pushurls não afeta o resultado.

### Achado D3-bypass (MEDIUM — insteadOf reescrita divergente em checkRemoteEmpty)

**Medido:** `measure_gaps3.sh` e confirmado em `measure_gaps5.sh`, `measure_gaps6.sh` e
`measure_gaps8.sh`.

**Mecanismo raiz:** `checkRemoteEmpty` obtém `push_url` via `git remote get-url --push --all origin`
e então chama `git ls-remote --heads push_url`. O git aplica as regras `insteadOf` à URL passada a
`ls-remote`, possivelmente redirecionando para um destino diferente — enquanto o push real vai para
`push_url`. A divergência permite que `checkRemoteEmpty` veja "remoto vazio" enquanto o push chega
em um remoto com branches.

**Duas formas de configuração de ataque:**

*Forma 1 — pushInsteadOf + insteadOf (`measure_gaps3.sh`):*
```ini
[remote "origin"]
    url = file:///bare_A
[url "file:///bare_B/"]
    pushInsteadOf = file:///bare_A   # push_url → bare_B (tem branch develop)
[url "file:///bare_C/"]
    insteadOf = file:///bare_B/      # ls-remote bare_B → bare_C (vazio)
```

*Forma 2 — insteadOf encadeado sem pushInsteadOf (`measure_gaps8.sh` Section A, precondições verificadas):*
```ini
[remote "origin"]
    url = file:///bare_A
[url "file:///bare_X"]
    insteadOf = file:///bare_A       # get-url --push: A → X (tem branch develop)
[url "file:///bare_Y"]
    insteadOf = file:///bare_X       # ls-remote X → Y (vazio)
```
Precondições confirmadas: `get-url --push origin` = X ✓; `ls-remote --get-url X` = Y ✓.
Resultado: rc=0, "Bootstrap push: main → origin/main", bare_X tem develop+main.

**O guard NÃO bloqueia `git config url.x.pushInsteadOf y` nem `git config url.x.insteadOf y`.**
Medido contra before e after: `before=0, after=0` em ambos. Guards PreToolUse gerados pelo
`trackfw init --ai-tools claude` (medido em `measure_gaps7.sh` Section B): `credential`,
`git-branch`, `write-scope`. Nenhum dos três bloqueia `git config url.*` — confirmado explicitamente
para cada um (`credential`: rc=0; `git-branch`: rc=0; `write-scope`: matcher é
"Write|Edit|NotebookEdit", não alcançado por chamadas Bash). Um agente com acesso à ferramenta Bash
pode auto-configurar o bypass em duas chamadas.

**Fluxo de execução em `checkRemoteEmpty`** (`internal/commands/push.go` ~L347–380):
1. `git remote get-url --push --all origin` → push_url (após pushInsteadOf se presente, ou após
   insteadOf se insteadOf se aplica ao push; para a Forma 2, insteadOf A→X faz push_url = X)
2. `git ls-remote --heads push_url` → git aplica insteadOf a push_url → redireciona para destino
   vazio → 0 linhas → `checkRemoteEmpty` retorna `true` ("remoto vazio")
3. `trackfw push` executa `git push -u origin main --force-with-lease=refs/heads/main:`
4. Push resolve origin → aplica pushInsteadOf (se presente) ou insteadOf → chega em push_url que
   tem branches (NÃO aplica insteadOf ao resultado já reescrito — assimetria medida)
5. main chega em push_url: "Bootstrap push: main → origin/main" rc=0

**Resultado medido** (`measure_gaps3.sh`):
- rc=0, mensagem "Bootstrap push: main → origin/main"
- `bare_B` recebeu refs/heads/main (além de refs/heads/develop preexistente)

**Impacto correto:** a ausência-lease (`--force-with-lease=refs/heads/main:`) ainda impede
sobrescrever uma main existente no remoto. O que falha: o controle D3 "remoto vazio" — `main`
chega em um remoto que já tem branches. D3 não restringe o conteúdo do commit — verifica apenas remotos vazios + count=1 (`push.go:177–188`);
o commit de bootstrap pode conter conteúdo arbitrário. O bypass publica um commit de `main`
arbitrário em um remoto que já tem branches, que é exatamente o estado que D3-9 mostra o trackfw
recusando quando recebe o comando diretamente.

**Por que `ls-remote origin` (remote name) não resolve:** medido em `measure_gaps5.sh`.
`ls-remote origin` usa a URL de fetch em operações de leitura. No setup de ambas as formas, o
remote de fetch (bare_A) é vazio por construção. Resultado identicamente vazio; o diagnóstico não
muda. Além disso, alterar `checkRemoteEmpty` para usar `ls-remote origin` em vez da push URL
regressaria o D3-7a: se `pushurl` explícito aponta para um bare com branches, `ls-remote origin`
(fetch URL) pode ver outro bare vazio, classificando erroneamente como "remoto vazio".

**Fix candidate medido e confirmado em `measure_gaps6.sh` Section A:**
`git ls-remote --get-url $PUSHURL` retorna a URL após aplicar insteadOf. Se o resultado diferir de
`$PUSHURL`, a URL de push seria duplamente reescrita — fail-closed.

| Config | PUSHURL | get-url result | Ação |
|--------|---------|---------------|------|
| Forma 1 (pushInsteadOf+insteadOf) | bare_B | bare_C (differ) | fail-closed ✓ |
| Forma 2 (chained insteadOf, medido gaps8 Section A) | bare_X | bare_Y (differ) | fail-closed ✓ |
| Normal (sem insteadOf) | bare_A | bare_A (same) | prossegue ✓ |
| D3-7a (pushurl explícito, sem insteadOf) | bare_B | bare_B (same) | prossegue ✓ |

Implementação sugerida em `checkRemoteEmpty` (`execGit` é o helper real, `push.go:19-20`):
```go
for _, u := range urls {
    resolved, _ := execGit("ls-remote", "--get-url", u)
    if strings.TrimSpace(resolved) != u {
        return false, fmt.Errorf("push URL %q rewritten to %q by insteadOf — refusing bootstrap push", u, resolved)
    }
    // proceed with ls-remote --heads u as before
}
```

**Falsos positivos medidos** (`measure_gaps7.sh` Section D; `measure_gaps7b.sh`):

| Config | push_url (get-url --push) | get-url result | Mesmo? | FP? |
|--------|--------------------------|----------------|--------|-----|
| (i) insteadOf HTTPS→SSH | `git@github.com:org/repo.git` | idem | YES | Não |
| (ii) pushInsteadOf HTTPS→SSH (forge padrão) | `git@github.com:org/repo.git` | idem | YES | Não |
| (iii) insteadOf `git@`→`ssh://` (normalização SSH) | `ssh://git@github.com/org/repo.git` | idem | YES | Não |
| **(i)+(iii) em cascata** | `git@github.com:org/repo.git` | `ssh://git@github.com/org/repo.git` | **NO** | **Sim** |

Em (i)–(iii) individualmente: `get-url --push` já retorna a URL pós-reescrita; `ls-remote --get-url`
sobre ela não dispara nova regra — sem falso positivo. Com (i)+(iii) juntos (medido em
`measure_gaps7b.sh`): a segunda regra casa com o resultado da primeira → FP confirmado para uma
configuração legítima.

Um quinto arm (`insteadOf X→Y` com sufixo `/` em X, origin sem trailing slash) foi descartado:
a regra não casou (precondição P2 `ls-remote --get-url X` = X, não Y), tornando o arm vazio.

O fix detecta o bypass em ambas as formas (Forma 1 e Forma 2, `differ=YES` em ambas) e produz
FP para a configuração (i)+(iii) em cascata, que é uma config legítima de normalização.

**Declaração de residual na ADR D3 e no Wave 0 (§4):** nenhuma. Nenhuma das fontes menciona
`insteadOf`, `pushInsteadOf` ou URL rewriting como vetor aceito.

**write-scope não fecha o caminho in-band:** medido em `measure_gaps8.sh` Section D.
`guard write-scope` com payload `{"tool_name":"Edit","tool_input":{"file_path":".../work/.git/config",...}}`
retorna rc=0. `~/.gitconfig` com mesmo payload também rc=0. **Apenas a Opção A (fix em
`checkRemoteEmpty`) fecha o vetor in-band.**

Severidade: **MEDIUM**. O guard não bloqueia os `git config` necessários, tornando o bypass
configurável em-banda por um agente.

### insteadOf simétrico — INFO (caso sem bypass)

Medido em `measure_gaps2.sh` (seção D): `url."file://bare_vazio/".insteadOf = "file://bare_has_main"`
sem `pushInsteadOf`. Resultado: ls-remote AND push vão para o mesmo destino reescrito. Sem
divergência — consistente. Classificado como **INFO**.

Nota: uma única regra insteadOf não produz divergência. O bypass requer uma segunda regra que
case com a URL já reescrita pela primeira — isso é a Forma 2 (insteadOf encadeado).

### TOCTOU — LOW (não-atomicidade estrutural; recomendo declarar como residual)

**Medido:** `measure_toctou.sh`, `measure_gaps6.sh` Section D, `measure_toctou2.sh`,
`measure_toctou3.sh`, `measure_gaps7.sh` Section C.

**Natureza:** `checkRemoteEmpty` → `git push` é check-then-act não-atômico. A
`--force-with-lease=refs/heads/main:` protege apenas `refs/heads/main`; qualquer ref com nome
diferente injetado na janela check→push passa pela verificação de receive-pack, pois a ausência-lease
não cobre outros refs. O check create-only de receive-pack ainda protege `main` de sobrescritas
(confirmado em toctou2: hook tentou sobrescrever `refs/heads/main` com SHA diferente → rc=1).

**Rodadas de medição:**

- **`measure_toctou.sh`** (hook usa `SHA=$(git rev-parse HEAD)` no repo de trabalho — obtém o SHA
  real do commit local — depois `git --git-dir=BARE update-ref refs/heads/main $SHA`): update-ref
  falhou com "nonexistent object" porque o objeto (commit do working repo) ainda não foi
  transferido para o bare (bare vazio; objetos chegam com o receive-pack, não antes). rc=0 foi da
  entrega normal, não da race.
- **`measure_gaps6.sh` Section D** (hook usa literal `HEAD` como valor de update-ref):
  `git --git-dir=BARE update-ref refs/heads/main HEAD` (linha 130 do script). No bare (unborn),
  `HEAD` é um ref simbólico não resolvível → update-ref falhou (erro suprimido via `2>/dev/null`).
  Race não disparou; rc=0 foi da entrega normal.
- **`measure_toctou2.sh`** (hook escreve SHA diferente em refs/heads/main): rc=1,
  "reference already exists" — `git-receive-pack` detectou conflito de ref. PASS.
- **`measure_toctou3.sh`** (hook copia objetos + escreve refs/heads/develop com SHA real):
  `RACE CONFIRMED` — rc=0, "Bootstrap push: main → origin/main", bare tem develop+main.
  A absence-lease protege `refs/heads/main` mas não outros refs; `refs/heads/develop` escrito
  pelo hook não é detectado; `git-receive-pack` vê main ausente → lease satisfeita → aceita.
- **`measure_gaps7.sh` Section C** (hook usa `git -C "$SEED" push "file://$BARE" develop` — receive-
  pack local, não rede; mesmo bare que o trackfw push vai usar):
  ```
  RACE CONFIRMED via git push (receive-pack path): develop+main, rc=0
  ```
  Confirma que a race dispara quando o escritor concorrente usa `git push` via receive-pack (não
  apenas via acesso de filesystem direto). Transporte SSH/HTTPS não foi medido (restrição offline).

**Análise de escopo:**
- **file://** (filesystem direto ou git push local via receive-pack): race determinística — medida
  e confirmada. Um hook `pre-push` pode usar `git push "file://$BARE" <branch>` ao mesmo bare,
  ou escrever refs diretamente via `git update-ref` (após copiar objetos).
- **SSH/HTTPS**: hook não pode escrever filesystem do servidor. A opção seria `git push origin <branch> --no-verify` a partir do hook (evita recursão via `--no-verify`, que suprime o hook no segundo push) —
  não medido por restrição offline. Contudo, a janela check-then-push existe estruturalmente para
  qualquer escritor concorrente externo (CI, colaborador, forge auto-init) independente de
  transport.
- Probabilidade SSH/HTTPS: a race requer um segundo escritor empurrando um ref não-default para o
  remoto dentro da janela check→push de um repositório recém-criado; frequência não medida.

**Achado adicional (`measure_gaps8.sh` Section B) — hook executa git push diretamente:**
Um hook `pre-push` que chame `git push` diretamente contorna todos os guards PreToolUse. Medido:
hook empurrou `refs/heads/feat/x` ao bare vazio logo antes que o `trackfw push` (de main) chegasse;
bare ficou com `refs/heads/feat/x` + `refs/heads/main` ao final. O trackfw push (de main) percorreu
o caminho D3 (bootstrap default branch), não D4 — D4 só é acionado quando se tenta empurrar uma
feature branch pelo `trackfw push`. O que o teste demonstra é: (a) uma feature branch pode chegar
ao remoto vazio antes da branch default, que é o estado que D4 existe para prevenir segundo o ADR;
(b) o push bootstrap de main em seguida foi para um remoto não mais vazio — mesma instância de
TOCTOU. Hooks executam `git` fora de qualquer PreToolUse guard.

Wave 0 §4 cita "**Aceito — classe gh/non-git push além de `gh repo create`**: outros binários com
push interno" (linha 479). Hooks executam `git` (não gh/non-git) — este gap é pre-existente à REQ
e não coberto pelo residual declarado. Registrado como pré-existente.

**Declaração de residual na ADR D3 e no Wave 0 §4:** nenhuma. TOCTOU não está listado em nenhuma
das duas fontes.

**Recomendação:** declarar como residual na ADR D3 e no Wave 0 §4. A não-atomicidade de
check-then-push é estrutural no git (nenhum mecanismo de atomic-lease sobre branches arbitrários
remotos); o impacto é a invariante "bootstrap vai para remoto limpo" podendo ser violada por um
escritor concorrente, sem dano aos dados existentes (absence-lease protege o ref sendo empurrado).
A decisão de aceitar ou tratar pertence ao arquiteto/KG.

**Pré-condição:** para a variante determinística (hook), write access a `.git/hooks/pre-push`.

Classificado como **LOW**.

---

## 4. D4 — Feature push para remote vazio

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D4-1: chore/ push para remote vazio: after bloqueia (D4-específico) | rc!=0 | rc!=0 | PASS |
| D4-counter-arm: chore/ push para remote não-vazio: PERMITE | rc!=0 | rc=0 | PASS |
| D4-3: chore/ via ship para remote vazio: after bloqueia | rc!=0 | rc!=0 | PASS |
| D4-fail-open: chore/ para remote inacessível: D4 não bloqueia, git falha | — | rc!=0 (git) | PASS |

Nota sobre feat/ branches: `push` de branch `feat/x` para remote vazio retorna rc=1 com mensagem
do gate de governança ("branch feat/x is a feat/fix/refactor branch but no roadmap is in wip/"),
não D4. D4 não é alcançado para tipos governados sem roadmap ativo.

Mensagem D4 medida (chore/ para remote vazio):
```
Branch: chore/d4chore
Governance: skipped (chore/docs branch)
Error: trackfw push: the remote has no branches yet.
Publish the default branch first:
  1. Create the repository on your forge
  2. git remote add origin <url>
  3. git status --short
  4. git add <each path listed by git status --short>
  5. trackfw commit -m "chore(scaffold): initial"
  6. trackfw push  (from main or master)
Then come back and push your feature branch.
```

D4 fail-open confirmado: remote inacessível → "Warning: could not fetch origin (offline or no
remote); skipping squash-merge check" → push tentado pelo git (que falha com rc!=0 por razão de
rede). D4 não bloqueia preventivamente quando não consegue consultar o remote.

---

## 5. D5 — Bloco de bootstrap nos arquivos gerados

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D5-1: arquivo gerado contém bloco bootstrap | ausente | presente | PASS |
| D5-2: texto bootstrap ausente nos arquivos gerados pelo before | ausente | — | PASS |

Medido via D1-1c (após `trackfw init`) e confirmado buscando nos arquivos gerados em `d1_fresh2_*`:
bootstrap text encontrado em `.claude/agents/trackfw-architect.md`, `.claude/commands/trackfw/implement.md`
e `CLAUDE.md` nos arquivos gerados pelo **after**. Busca nos mesmos arquivos gerados pelo **before**
não encontrou o texto — unicidade confirmada.

---

## 6. D6 — Guard additions (novos vetores bloqueados)

| Vetor | Before | After | Resultado |
|-------|--------|-------|-----------|
| D6-1a: git switch --orphan main | rc=0 | rc=2 | PASS (novo em D6) |
| D6-1b: git switch --orphan (sem nome) | rc=0 | rc=2 | PASS |
| D6-1c: git switch --orphan=newbranch | rc=0 | rc=2 | PASS |
| D6-2a: git checkout --orphan main | rc=2 | rc=2 | PASS (pré-existente) |
| D6-3a: gh repo create --push | rc=0 | rc=2 | PASS (novo em D6) |
| D6-3b: gh repo create --source=. --push | rc=0 | rc=2 | PASS |
| D6-3c: gh repo create --push=true | rc=0 | rc=2 | PASS |
| D6-3d: gh repo create sem --push (permitido) | rc=0 | rc=0 | PASS |
| **D6-3e: gh repo create --push=false (FP)** | rc=0 | **rc=2** | **FAIL** |
| D6-4a: git worktree add --orphan main /tmp/wt | rc=0 | rc=2 | PASS (novo em D6) |
| D6-4b: git worktree add --orphan /tmp/wt (1 arg) | rc=2 | rc=2 | PASS (pré-existente, motivo diferente) |
| D6-5a: git symbolic-ref HEAD refs/heads/x (write 2 pos) | rc=0 | rc=2 | PASS (novo) |
| D6-5b: git symbolic-ref --delete HEAD | rc=0 | rc=2 | PASS |
| D6-5c: git symbolic-ref -d HEAD | rc=0 | rc=2 | PASS |
| D6-5d: git symbolic-ref HEAD (read, 1 pos, permitido) | rc=0 | rc=0 | PASS |
| D6-5e: git symbolic-ref --short HEAD (permitido) | rc=0 | rc=0 | PASS |
| D6-5f: git symbolic-ref --verify HEAD (permitido) | rc=0 | rc=0 | PASS |
| D6-6a: bash -c 'gh repo create x --push' — ver achado | rc=0 | rc=0 | FAIL (gap) |
| D6-6b: env git switch --orphan main | **rc=0** | rc=2 | PASS (novo via matchSubcommandPOSIX) |
| D6-6c: command git switch --orphan main | **rc=0** | rc=2 | PASS (novo) |
| **D6-6d: git -c alias.sw=switch sw --orphan main** | **rc=0** | **rc=0** | **FAIL (gap pré-existente)** |
| D6-6e: \git switch --orphan main (backslash) | **rc=0** | rc=2 | PASS (novo) |
| D6-6f: gh alias set (gh verb!=repo, permitido) | rc=0 | rc=0 | PASS |
| D6-6g: gh repo view (verb!=create, permitido) | rc=0 | rc=0 | PASS |

**rc values D6-6b–e medidos em `measure_gaps6.sh` Section C** (correção: valores anteriores
before=rc=2 eram inventados; medição real: before=0 para todos).

Nota D6-4b: 1-arg form (`git worktree add --orphan /path`) já era bloqueada no before pelo guard
pré-existente de "worktree add sem branch flag" (motivo diferente do D6). D6 adiciona bloqueio para
a 2-arg form (`git worktree add --orphan <branch> <path>`) que antes não era bloqueada.

### Achado D6-FP (LOW — falso positivo)

`gh repo create --push=false` é bloqueado (rc=2) quando deveria ser permitido.

Localização: `internal/guard/gitbranch.go`, função `applyGhRule()`.

Causa raiz: `strings.HasPrefix(t, "--push=")` corresponde a `--push=false` além de valores truthy.

Mitigação: comparar `t == "--push"` OU restringir a `--push=true`, `--push=1`, `--push=yes`.

Severidade: **LOW** (falso positivo benigno — `--push=false` é inerte, o usuário pode omitir a flag).

### Achado D6-wrapper (LOW — lacuna de defesa em profundidade)

`bash -c 'gh repo create x --push'` retorna rc=0 (não bloqueado). Medido com JSON válido:

```bash
printf '{"command":"bash -c '"'"'gh repo create x --push'"'"'"}' | tf-after guard git-branch
→ rc=0
```

E `gh repo create --push` direto É bloqueado (rc=2). `bash -c 'git switch --orphan main'` É
bloqueado (rc=2).

Causa raiz: `matchSubcommandPOSIX` contém `if base != "git" { continue }` que pula comandos `gh`
dentro de wrappers. D6 adicionou `applyGhRule` ao `MatchSubcommand` (nível de topo) mas não ao
`matchSubcommandPOSIX` (nível recursivo).

Localização: `internal/guard/gitbranch.go`, `matchSubcommandPOSIX`, condição `base != "git"`.

Mitigação: em `matchSubcommandPOSIX`, para cada base extraída que seja `gh`, chamar
`applyGhRule` além de `applyGitRule`.

Severidade: **LOW**. D2/D3/D4 são os controles primários.

### Gap D6-6d (LOW pré-existente — alias via git -c)

`git -c alias.sw=switch sw --orphan main` retorna rc=0 em before e after. O guard tokeniza
`git -c <config> sw main` e vê `sw` como subcommand; `sw` não está em nenhuma regra → permitido.
Pré-existente; a nova implementação D6 não o fecha.

Cadeia hipotética (nunca executada end-to-end neste teste; GH não disponível):
`trackfw commit chore/x` → `git fetch . chore/x:main` (pré-existente) →
`bash -c 'gh repo create --push'` (D6-wrapper gap). Cada passo é LOW isoladamente.
`trackfw commit` é necessário (raw `git commit` é bloqueado pelo guard).

---

## 7. Sumário de Achados

| ID | Severidade | Tipo | Descrição |
|----|-----------|------|-----------|
| D3-bypass | **MEDIUM** | Bypass D3 | `checkRemoteEmpty`: ls-remote vê destino diferente do push (insteadOf divergente); duas formas confirmadas (pushInsteadOf+insteadOf; chained insteadOf); guard não bloqueia `git config` necessários; write-scope também não fecha; fix candidate disponível |
| D1-6 | LOW | Injeção em conselho impresso | `PrintBootstrapSteps`: pathnames sem shell-quote; `%q` não protege `$()`; fix: `-z`, single-quote POSIX, `--` |
| D6-FP | LOW | Falso positivo | `gh repo create --push=false` bloqueado incorretamente via HasPrefix |
| D6-wrapper | LOW | Lacuna defesa-em-profundidade | `bash -c 'gh repo create --push'` não bloqueado por `matchSubcommandPOSIX` |
| D6-6d | LOW pré-existente | Lacuna guard | `git -c alias.sw=switch sw --orphan main` não bloqueado (bypass via alias) |
| M-X1-guidance | LOW | Orientação sem saída | commit → "use branch new"; branch new → "push from main or master"; sem instruir `git switch <existing>` |
| TOCTOU | LOW | Residual estrutural | check-then-push não-atômico; absence-lease cobre apenas refs/heads/main; ref com outro nome injetado na janela passa; medido via hook (file://) e via git push (receive-pack); não declarado na ADR D3 nem no Wave 0 §4; recomendo declarar como residual |
| insteadOf-sym | INFO | Footgun de configuração | insteadOf simétrico: ls-remote + push vão para o mesmo destino; sem bypass real |

---

## 8. Veredito

**REPROVA** — um achado MEDIUM sem residual declarado.

O achado `D3-bypass` não está coberto pela ADR D3 (nenhuma menção a `insteadOf`, `pushInsteadOf`
ou URL rewriting) nem pelo Wave 0 §4 "Residual declarado" (que cita ref-movers, classe gh/non-git
push e recovery, mas não URL rewriting). Sem declaração de residual, o controle D3 está incompleto
para este vetor.

O achado **TOCTOU** (LOW) também não está declarado na ADR D3 nem no Wave 0 §4; recomendo
declarar como residual, mas não afeta o veredito REPROVA (que é exclusivamente pelo D3-bypass
MEDIUM).

**Para aprovar, o implementador deve escolher uma das opções:**
- **(A) Aplicar o fix:** em `checkRemoteEmpty`, antes de chamar `ls-remote --heads $url`, chamar
  `git ls-remote --get-url $url`; se o resultado diferir de `$url`, retornar erro (fail-closed).
  Medido como correto para ambas as formas (Forma 1 pushInsteadOf+insteadOf e Forma 2 insteadOf
  encadeado) e sem falso positivo para (i)–(iii) individualmente. **Falso positivo medido** para
  o par (i)+(iii) em cascata (`insteadOf HTTPS→SSH` + `insteadOf git@→ssh://`): push_url =
  `git@github.com:org/repo.git` → `ls-remote --get-url` = `ssh://git@github.com/org/repo.git` ≠
  push_url → fix bloqueia uma configuração legítima. Custo aceitável para um bootstrap único
  (o usuário remove uma das regras temporariamente).
- **(B) Declarar residual:** adicionar à ADR D3 e ao Wave 0 §4 que o vetor de `checkRemoteEmpty`
  passar a push_url para `ls-remote --heads` (que pode aplicar insteadOf e divergir do destino real
  do push) é residual aceito, com justificativa explícita. Esta declaração deve cobrir ambas as
  formas do mecanismo.

---

## 9. Vetores Limpos

- **D2-3**: governance gate para `feat/` antes de D2; `chore/` born HEAD permite branch new ✓
- **D2-6**: commit em `orphan-branch` (não main/master) permitido por design ✓
- **D3-6**: test harness bug (reutilização de variável); comportamento confirmado correto ✓
- **D3-8**: tags-only = vazio de heads; ADR L67–69 é explícito ✓
- **D3-9**: remote com develop (não main) → D3 bloqueia (qualquer head = não-vazio) ✓
- **D6-4b**: 1-arg form já bloqueada em before por guard pré-existente ✓
- **M-X1-D2**: D2 robusto — unborn master + count=1 bloqueia commit; absence-lease preservado ✓
- **E2E happy path**: `init → git add → commit → push → branch new chore/first` rc=0 ✓

### Gaps pré-existentes confirmados (não introduzidos por esta REQ)

| Gap | Before | After | Pré-existente? |
|-----|--------|-------|---------------|
| `git fetch . chore/x:main` não bloqueado (ref-mover) | rc=0 | rc=0 | Sim |
| `git -c alias.sw=switch sw --orphan main` não bloqueado | rc=0 | rc=0 | Sim |
| `git config url.x.pushInsteadOf y` não bloqueado (before e after) | rc=0 | rc=0 | Sim |
| `git config url.x.insteadOf y` não bloqueado (before e after) | rc=0 | rc=0 | Sim |
| Hook `pre-push` executa `git push` direto — bypassa todos os PreToolUse guards | n/a | n/a | Sim (classe arquitetural: hooks git rodam fora do PreToolUse; Wave 0 §4 "gh/non-git" cobre outros binários, não hooks git) |

---

## 10. Estado do Repositório

Verificado ao término:
```
?? docs/seguranca/2026-11-26-red-team-bootstrap-de-repositorio-novo.md  (este relatório)
?? docs/seguranca/2026-11-26-revisao-bootstrap-de-repositorio-novo.md  (pré-existente, untracked)
```
Nada staged. Nenhuma escrita fora de `docs/seguranca/`.
