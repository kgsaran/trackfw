---
status: Accepted
date: 2026-10-12
author: "zeus-tf"
---

# ADR: operação de rede do trackfw tem prazo e o estouro nunca vira sucesso silencioso

> Date: 2026-10-12 | Status: Accepted

REQ: `docs/req/REQ-2026-10-12-operacoes-de-rede-do-trackfw-nao-tem-prazo-e-um-fetch-pendurado-trava-push-ship-prune-e-release-sem-limite.md`
Issue: #556

## Context

**Medido em 2026-10-10** (#556): um `trackfw push` ficou parado **70 minutos**. A árvore de processos
era `trackfw push` → `git fetch origin --prune` → `ssh … git-upload-pack`. O processo pendurado era o
`ssh`, **neto** do `trackfw`. Com o `ssh` encerrado à mão, o `fetch` falhou, o `push` emitiu o aviso
que já existia e concluiu normalmente. Era rede intermitente, não credencial.

Fatos do código que restringem o desenho:

- Os executores de subprocesso de produção usam `exec.Command` **sem contexto e sem prazo**:
  `defaultGitExec` (`internal/commands/ship.go:166`, usado por `push`, `ship`, `branch prune` e
  `release tag`), `defaultGhExec` (`branch_prune_forge.go:56`, usado por `branch prune`, `push` e
  `ship`), `defaultExecForgeCLI` (`ship.go:186`), `defaultCheckPROpen` (`ship.go:224`) e o executor
  de `gh api` do `release tag` (`release.go:229`).
- Pontos de chamada de rede conhecidos (lista **não fechada**: o Wave 0 a fecha):

| ponto | operação | caminho de falha hoje |
|---|---|---|
| `push.go:238`, `ship.go:392` | `git fetch origin --prune` | aviso, pula a detecção de squash-merge |
| `branch_prune.go:390` | `git fetch origin --prune` | aviso, avalia com dado possivelmente velho (seguro para o local) |
| `release.go:278` | `git fetch origin --prune` | falha fechada (`releaseTagFetchFailedFmt`) |
| `release.go:310` | `git ls-remote --tags origin` | 🔴 **erro descartado** (`remoteTagOut, _ :=`): falha vira "a tag não existe no remoto" |
| `branch_prune_forge.go:56` | `gh pr list` | degrada para a heurística de conteúdo (D2 do ADR-2026-10-03) |
| `release.go:229` | `gh api` | falha fechada |
| passo 4 do `push`/`ship` | `git push` | erro do push |

- **Precedente no repositório:** `internal/validator/validator_guard_binary_probe.go:138-147` usa
  `exec.CommandContext` + `cmd.WaitDelay`, com comentário explicando que o `WaitDelay` existe porque
  um **neto** pode segurar o pipe aberto e o `Wait` não retorna. É exatamente o mecanismo do #556.
- A nota do vault `guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24`
  mediu que **orçamento total** depende do tamanho do trabalho e mata trabalho legítimo, enquanto
  **orçamento ocioso** é independente do tamanho.

## Decision

**D1 — O estouro de prazo nunca vira sucesso silencioso.** Cada ponto da população segue o caminho de
falha **já declarado** quando ele existe (aviso no `push`/`ship`, dado velho declarado no `prune`,
falha fechada no `release`, degradação D2 no forge). Onde o erro é **descartado** hoje
(`release.go:310`), o estouro e qualquer outra falha passam a ter caminho próprio: **falha fechada
nomeada**. "Não consegui perguntar ao remoto" não pode ser lido como "o remoto disse que não".

**D2 — Mecanismo: contexto com prazo + `WaitDelay`, nos executores de produção.** O prazo mora nos
executores (`defaultGitExec`, `defaultGhExec` e equivalentes), não em cada ponto de chamada, para que
um ponto novo herde o limite por construção. Matar o filho não basta: o `WaitDelay` limita a espera
pelo pipe que o neto segura. Em Unix, o executor coloca o filho num **grupo de processos próprio** e
o estouro encerra o grupo, para que o `ssh` neto não fique órfão. Esse detalhe precisa ser **medido**,
não presumido.

**D3 — Natureza do limite por operação.** A decisão fica com o **Wave 0, por medição**:
- **leitura de metadados** (`fetch` de refs, `ls-remote`, `gh pr list`, `gh api` de leitura):
  orçamento **total** é aceitável, porque o volume é pequeno e limitado. O valor padrão é proposto
  pelo Wave 0 a partir de medição.
- **transferência de dados** (`git push`, e `git fetch` quando traz objetos): orçamento total mata
  transferência longa legítima. A hipótese é um limite **ocioso** (sem progresso por N s). Os
  candidatos são o `http.lowSpeedLimit`/`http.lowSpeedTime` do git para HTTPS e o keepalive do `ssh`
  para SSH. O Wave 0 mede se esses candidatos existem e funcionam **sem sobrescrever** a configuração
  de `ssh` do usuário (`core.sshCommand`, `GIT_SSH_COMMAND`).
- **interativo** (`gh pr create` com stdin herdado do usuário): sem prazo. Há um humano presente, e
  matar um prompt seria defeito.

**D4 — O valor é ajustável sem recompilar.** Uma variável de ambiente única
(`TRACKFW_NETWORK_TIMEOUT`, duração no formato do Go) sobrepõe o padrão. Valor inválido gera aviso e
usa o padrão, sem nunca desligar o prazo em silêncio. `0` só desliga o prazo se o Wave 0 justificar.

**D5 — A mensagem de estouro nomeia o prazo e a variável.** Por exemplo: `git fetch origin timed out after
60s (TRACKFW_NETWORK_TIMEOUT)`. Assim o usuário distingue estouro de prazo de "offline".

**D6 — O teste que prova o prazo exercita o executor real.** Um `execGit` falso **substitui**
justamente a função que contém o prazo, e por isso não morde. O teste roda o executor de produção
contra um **neto pendurado**, reproduzindo o mecanismo medido (por exemplo, `GIT_SSH_COMMAND` apontando
para um processo que dorme e um remoto `ssh://`). Ele termina dentro de prazo + `WaitDelay`, e
**trava/reprova** sem o `WaitDelay`. Precisa rodar no CI de Windows.

### D7 — Ajustes do Wave 0 (`docs/seguranca/2026-10-12-wave0-prazo-das-operacoes-de-rede.md`, aprovado com ajustes)

**Mecanismo, medido em quatro modos** (neto `ssh` que dorme, `GIT_SSH_VARIANT=ssh`):

| modo | resultado |
|---|---|
| A: só `CommandContext` | não retorna; o pipe preso é o do **stderr**, herdado pelo neto |
| B: + `WaitDelay` | retorna; deixa 1 órfão |
| C: + `Setpgid` sem `Cancel` | retorna; deixa 2 órfãos |
| D: + `Setpgid` + `cmd.Cancel` matando o grupo | retorna no prazo; 0 órfãos |

- **A1 (D3, D4) — `git push`:**
  - Recebe teto próprio, `TRACKFW_PUSH_TIMEOUT`, com padrão de 30 min. Sem `CommandContext` o
    `WaitDelay` não ativa, e um push SSH pendurado travaria para sempre.
  - Em HTTPS, quando `git config --get http.lowSpeedLimit` vier vazio, recebe também
    `GIT_HTTP_LOW_SPEED_LIMIT=1` e `GIT_HTTP_LOW_SPEED_TIME=30`, que limitam o tempo ocioso.
  - Em SSH não há limite ocioso sem sobrescrever o comando de `ssh` do usuário (RS-1); o teto total
    é o backstop.
  - **D4 passa a ter duas variáveis**, porque os contratos são distintos: metadados com teto curto,
    transferência com teto longo.
  - O `git push --delete` do prune remoto é classificado como `push` e herda esse teto.
- **A2 (D2) — Grupo de processos:** só é usado quando **não** há terminal de controle (`/dev/tty`
  não abre). Com terminal, o processo fora do grupo de primeiro plano receberia SIGTTIN ao pedir a
  passphrase do `ssh`, e o Ctrl-C deixaria de alcançá-lo. Nesse caso, e sempre no Windows, só o
  `WaitDelay` atua (RS-2, RS-6), e o neto pode sobreviver.
- **A3 (D4) — Valores inválidos:** `0`, negativos e valores acima de 24 h geram aviso e o padrão é
  usado. O prazo nunca é desligado.
- **A4 (D3) — Padrão de 60 s para metadados.** Medido: `git fetch` em 1,58 s e `gh pr list` (463 PRs)
  em 2,96 s, o que dá margem de 20×. Um fetch que traz objetos no `release tag` pode estourar; é
  falso positivo aceito, e a variável ajusta.
- **A5 (D1):** `release.go:310` → falha fechada `could not confirm tag … is absent from remote`.
- **A6 (D6) — Teste:** o neto é o próprio binário de teste reexecutado e dormindo, nunca um processo
  que lê stdin (`cat` sai no EOF e mascara a regressão). O teste exige `GIT_SSH_VARIANT=ssh`. Ele
  afirma o limite inferior (≥ prazo) e o superior (< prazo + `WaitDelay` + margem).
- **A7 (D5):** em `ship.go:801`, o motivo da degradação de `queryForgePRs` é descartado; o estouro
  passa a chegar ao usuário.
- **A8 (D5):** estouro numa **escrita** (`release.go:462,482`, POST de tag/ref) diz que o estado
  remoto é incerto e manda verificar antes de rodar de novo.
- **A9 (D2) — Um único executor de `git`, com classificador de subcomando:**
  - O classificador pula flags globais de dois tokens (`-C`, `-c`, `--git-dir`, `--work-tree`,
    `--namespace`, `--super-prefix`).
  - Classes: `fetch` e `ls-remote` são metadados; `push` é transferência; o resto é local, sem
    prazo.
  - Os executores de `gh` têm sempre prazo; `defaultExecForgeCLI` (interativo) nunca tem.
  - Subcomando de rede novo (`pull`, `clone`, `remote show`) precisa entrar no mapa (RS-7).
- **A10 — mesma causa, outro mecanismo:** `internal/sync/linear.go:76` usa `http.DefaultClient`
  sem prazo. Entra nesta REQ, com o mesmo prazo de metadados. Jira e `thirdparty/fetch.go` já têm
  prazo de 30 s.

## Consequences

- O `push` deixa de poder travar indefinidamente no passo opcional; no pior caso espera o prazo e
  segue com o aviso que já existe.
- O `release tag` passa a recusar quando não consegue confirmar que a tag não existe no remoto.
- Um link muito lento pode estourar o prazo de leitura; a variável de D4 é a saída.

### Resíduos (a confirmar no Wave 0)

- **Windows:** não há grupo de processos com a mesma semântica; o `ssh` neto pode sobreviver ao
  estouro. O `WaitDelay` ainda garante que o `trackfw` retorna.
- **Hooks do git disparados pelo `push`** (`pre-push`) herdam o limite da operação; um hook lento
  pode estourar. A medir.

## Alternatives Considered

- **Prazo em cada ponto de chamada:** ponto novo nasce sem prazo. Rejeitada em favor de D2.
- **Só `exec.CommandContext`:** não resolve o caso medido. O neto segura o pipe e o `Wait` não
  retorna. Rejeitada.
- **Forçar `GIT_SSH_COMMAND` com `ConnectTimeout`/`ServerAliveInterval`:** sobrescreve a configuração
  de `ssh` do usuário. Fica como candidato do D3 só se o Wave 0 mostrar que dá para compor sem
  sobrescrever.
- **Orçamento total também no `git push`:** mata push legítimo de repositório grande. Rejeitada em
  princípio; o Wave 0 confirma.
