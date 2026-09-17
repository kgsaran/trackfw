---
status: Open
date: 2026-08-29
author: claude
adr: docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md
roadmap: "docs/roadmaps/claude\backlog\ROADMAP-2026-08-29-atualizar-para-a-upstream-main-com-o-fix-de-symlink.md"
---

# REQ: Atualizar para a upstream main com o fix de symlink

> Date: 2026-08-29 | Status: Open

## Motivation

Primeiro exercicio real da ancestralidade estabelecida em
`ADR-2026-08-29-adotar-upstream-como-base`: `git merge upstream/main` agora funciona sem flag.

Ha **2 commits** depois da `v7.3.0`, sem tag nova. Um e governanca do upstream (nao importamos). O
outro, `e0f8543` (PR #215 deles), traz tres coisas — e a primeira e o motivo desta REQ.

### 1. Escrita arbitraria por symlink — HIGH, reproduzida nos tres CLIs

Da nota do revisor deles
(`.claude/agent-memory/hades-tf/project_update_discover_symlink_follow_arbitrary_write.md`):

> As checagens de presenca (`os.Stat` / `fs.existsSync` / `os.path.isfile`) e as escritas
> (`os.WriteFile` / `fs.writeFileSync` / `open(...,'w')`) seguem symlink por padrao, sem nenhuma
> guarda de `lstat` em `internal/generators/update.go`, `npm/src/commands/update.js`,
> `pypi/trackfw/commands/update.py`, nem nos irmaos `discover.*`.

- Symlink vivo em `.github/workflows/trackfw-validate.yml` apontando para fora do projeto, mais um
  `trackfw update` — **mesmo com `ci: none` no `trackfw.yaml`** — sobrescreve o alvo com o template
  de CI do trackfw.
- Symlink pendurado no mesmo caminho, mais `trackfw discover --init`, **cria** arquivo no caminho
  escolhido pelo atacante, fora do projeto.

Classificado HIGH e nao CRITICAL porque o conteudo escrito e sempre o template fixo do trackfw,
nunca conteudo do atacante. Ainda assim e escrita fora do projeto, e **este repositorio roda
`update` e `discover`** ao fazer dogfooding.

### 2 e 3. Pin de versao

O gate de CI passa a nascer pinado na versao que o gerou, e o `install.sh` honra
`TRACKFW_VERSION`. Mais dois gates: `check-ci-workflow-pin-parity.sh` e
`check-install-version-pin.sh`.

## Risco: oito arquivos em colisao

O upstream e este repo tocaram os mesmos arquivos. Sao exatamente onde vivem os fixes locais de
CRLF, `homedir` e `tty`:

```
internal/generators/scaffold.go        pypi/trackfw/commands/discover.py
internal/generators/update.go          pypi/trackfw/commands/update.py
npm/src/commands/update.js             pypi/trackfw/generators/init_gen.py
npm/src/generators/init.js             pypi/trackfw/integrations/scaffold_doctor.py
```

Onde o upstream reescreveu funcao inteira, o fix local **some em silencio** — nao ha conflito para
avisar.

**E para exatamente isto que os quatro gates locais existem.** `check-python-writes-lf`,
`check-homedir-parity`, `check-tty-detection` e `check-slug-inventory` sao estaticos porque a CI do
upstream e Linux e nunca vera esses defeitos; eles pegam no merge, que e onde a regressao nasce.
**Este merge e o primeiro teste real deles**, e isso vale mais do que o resultado: se um fix sumir
sem gate reprovar, o gate e que esta errado.

## Acceptance Criteria

- [x] `git merge upstream/main` concluido, sem marcador de conflito em arquivo versionado
      → **(a) ENTREGUE.** `git grep '^<<<<<<<'` na árvore versionada: **0 ocorrências**. E a
      ancestralidade segue viva: `git merge-base main upstream/main` → `97543eef979a`.
- [x] Os **seis** gates verdes: `slug-inventory`, `python-writes-lf`, `homedir-parity`,
      `tty-detection`, `artifact-parity`, `subcommand-parity`
      → **(a) ENTREGUE.** Os **seis nomeados** saem `exit 0` em 2026-09-10.
      ```
      slug-inventory 0 · python-writes-lf 0 · homedir-parity 0 · artifact-parity 0
      tty-detection 0 · barrier 0 · subcommand-parity 0 · upstream-content 1  <- VERMELHO
      ```
      🔴 O `upstream-content` está vermelho, mas **não é um dos seis deste AC** — ele pertence à
      `REQ-2026-08-29-politica-de-conteudo-do-upstream-sem-gate`, onde recebeu veredito **(b)**.
- [ ] Para cada fix local que o merge derrubar, registrar **qual gate acusou** — ou, se nenhum
      acusou, tratar como buraco de cobertura e dizer isso
      → **(d) NAO VERIFICAVEL AQUI.** O AC é sobre o **processo daquele merge**, não sobre um estado
      do repositório. Não há artefato que registre quais fixes o merge de agosto derrubou nem qual
      gate acusou cada um; sem isso, não dá para dizer se foi feito.
      **O que faltaria:** o registro contemporâneo. Ele não existe.
- [x] `go build ./...` verde
      → **(a) ENTREGUE.** `exit 0`.
- [ ] Suite pypi sem regressao por **lista nomeada** contra 95 falhas, nunca so por contagem
      → **(b) NAO ENTREGUE** — convertido de `(d)` em 2026-09-11 pela
      `REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados` (AC5). A afirmação sobre agosto é
      **irrecuperável**: um baseline tirado hoje não prova "sem regressão desde agosto", só daqui
      para a frente — e daqui para a frente quem verifica é o ratchet do upstream, no nosso CI.
      Registro de 2026-09-10, que continua verdadeiro: A lista nomeada das 95 falhas **não foi versionada**. O próprio
      AC proíbe comparar por contagem, que é a única coisa que eu conseguiria produzir hoje.
- [ ] A vulnerabilidade de symlink verificada como corrigida em execucao real, nos tres runtimes
      → **(d) NAO VERIFICAVEL AQUI.** Verificar correção de symlink exige **criar** symlink, e nesta
      máquina o `ln -s` do Git Bash **degrada para cópia** (`MSYS=disable_pcon`, sem `winsymlinks`) —
      medido em 2026-09-10 e registrado no `CLAUDE.md`. Um teste que não consegue criar o vetor não
      prova que ele está fechado.
      **O que faltaria:** Developer Mode ou execução como administrador, ou uma máquina POSIX.
- [ ] Governanca do upstream continua fora, conforme a ADR
      → 🔴 **(b) NAO ENTREGUE.** `check-upstream-content.sh` acusa **7 arquivos** de governança do
      upstream em `docs/`, incluindo a `ADR-2026-09-03`. E a `REQ-2026-09-09-governanca` mediu
      **28 REQs** dele no nosso `req_dir`. A governança dele **não** continuou fora.

## Nao faz parte

Os dois gates novos do upstream (`ci-workflow-pin-parity`, `install-version-pin`) vem junto e serao
executados, mas fazer eles passarem no Windows nao entra: se falharem, viram registro, como os sete
ja levantados em kgsaran/trackfw#216.

## Linked ADR

ADR: docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-atualizar-para-a-upstream-main-com-o-fix-de-symlink.md
