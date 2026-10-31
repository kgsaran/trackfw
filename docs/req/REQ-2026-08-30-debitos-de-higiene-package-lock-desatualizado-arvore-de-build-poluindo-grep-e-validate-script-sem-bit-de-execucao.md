---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md"
---

# REQ: Débitos de higiene — `package-lock` desatualizado, árvore de build poluindo `grep`, e validate script sem bit de execução

> Date: 2026-08-30 | Status: Open

## Revisão de 2026-10-31

A REQ original é anterior à v8 (implementação única em Go). Reescrita depois de medir cada item na
`main` (`a485c637`, 11.2.3), e ampliada com os débitos avulsos pedidos pelo KG.

| item original | medido em 2026-10-31 | destino |
|---|---|---|
| 1. `package-lock` defasado | versão em dia (11.2.3), mas `releaseVersionFiles` (`internal/commands/release.go`) confere só 3 arquivos (`version.go`, `npm/package.json`, `pypi/pyproject.toml`), e a ajuda do `release tag` diz "4 version files" | AC1 |
| 2. `pypi/build/` poluindo `grep` | `pypi/build/` não existe; em `pypi/trackfw/` restam só diretórios `__pycache__` locais, ignorados pelo `.gitignore` (0 arquivos rastreados). Não é conteúdo do repositório | sai do escopo; limpeza local feita pelo arquiteto |
| 3. `InstallGates` sem `os.Chmod` | continua: `internal/discover/discover.go` grava `scripts/trackfw-validate.sh` com `os.WriteFile(..., 0755)`, que só aplica o modo na criação | AC2 |
| 4. `vunknown` no `doctor` Python | o CLI Python não existe desde a v8 | cancelado |

## Motivation

Resíduos que não quebram nada até quebrarem uma investigação ou uma release.

- O `package-lock` ficou 12 versões atrás porque nada o confere: o `release tag` existe para
  recusar versões divergentes, e o lockfile está fora da lista.
- Um `trackfw-validate.sh` que já existia sem bit de execução continua sem ele depois de
  `discover --init`; o hook que o chama falha com "permission denied".
- **`agent_models` no `update harness`** (pedido do KG): em 2026-10-30 o arquiteto registrou 20 linhas
  de `WARN: agent_models.<tier> ...` no stderr de um `update harness` padrão. Em 2026-10-31, com a
  configuração atual, `trackfw update harness --dry-run` não emite nenhum `WARN`. De passagem: o
  relatório em texto do `update harness` sai inteiro no stderr.
- **Documentação pendente do #633:** o parecer
  `docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md` ainda descreve o residual R-5 como
  "REQ própria", mas ele foi corrigido no ML-1A daquele roadmap; e a nota de vault sugerida pelo
  `apolo-tf` (fixture com `remote.origin.followRemoteHEAD never` e origin em caminho local neutraliza
  a forge) não foi escrita.
- **`GIT_TERMINAL_PROMPT` (decisão do KG: desligar o pedido de credencial só sem terminal
  interativo):** medido pelo arquiteto em 2026-10-31, sem terminal de controle,
  `git -c credential.helper= ls-remote https://github.com/kgsaran/nao-existe-zz-641.git </dev/null`
  falha em 0 s com `fatal: could not read Username for 'https://github.com': Device not configured`
  (o git abre `/dev/tty`, não o stdin). O comportamento decidido já é o atual; nenhuma mudança de
  código. Fica a nota de vault com a medição.

## Acceptance Criteria

- [ ] **AC1** — `trackfw release tag` recusa quando a versão de `npm/package-lock.json` (o campo
      `version` da raiz e o de `packages[""]`) diverge da pedida, com mensagem que nomeia o arquivo; a
      ajuda do comando diz o número real de arquivos conferidos.
- [ ] **AC2** — `InstallGates` deixa `scripts/trackfw-validate.sh` com bit de execução também quando o
      arquivo já existia com `0644`. No Windows, onde o bit não existe, nada muda.
- [ ] **AC2b** (mesma causa, enumerada na Wave 0, `docs/seguranca/2026-10-31-wave0-debitos-de-higiene.md`) — os hooks do husky gravados por
      `internal/generators/scaffold.go` (`.husky/commit-msg` e o hook de pre-commit), que o git só
      executa com bit de execução, recebem o mesmo tratamento.
- [ ] **AC3** — `agent_models`: a Wave 0 reproduziu as 20 linhas de 2026-10-30. Não são o `WARN:` de
      `agents_models.go`: é a mensagem de configuração global de `agent_models` ausente ou parcial,
      emitida por `harnessCatalogTarget` (`internal/generators/update.go`) uma vez por destino do
      catálogo. Passa a ser emitida **uma vez por execução**, e só quando há pelo menos um destino do
      catálogo na execução (sem destino do catálogo continua sem a mensagem, como hoje). O stderr fica
      capturável em teste (`UpdateOptions` com escritor de erro), para o teste não ser vacuoso. O
      relatório em texto já vai para o stdout (medido na Wave 0; o "stderr" anotado era a mensagem).
- [ ] **AC4** — parecer do #633 corrigido quanto ao R-5 (nota do arquiteto, sem reescrever o
      parecer) e as duas notas de vault escritas (fixture `followRemoteHEAD never`;
      `GIT_TERMINAL_PROMPT` sem terminal), linkadas no `vault/notes/index.md`.
- [ ] **AC5** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** mexe em `GIT_TERMINAL_PROMPT`, credential helper nem SSH (a medição mostrou que não há o
  que mudar no caso sem terminal).
- **Não** altera o protocolo de release além de acrescentar o `package-lock` à verificação.
- **Não** muda os modos de escrita do hook do lefthook (o `runner: sh` não exige bit de execução) nem
  das escritas com `O_APPEND` em hook existente do usuário (`update.go`), que não devem alterar o modo de
  um arquivo que não é nosso. Medido e justificado na Wave 0 (R-2, R-3).

## Linked ADR
<!-- none -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-debitos-de-higiene-package-lock-desatualizado-arvore-de-build-poluindo-grep-e-validate-script-sem-bit-de-execucao.md`
