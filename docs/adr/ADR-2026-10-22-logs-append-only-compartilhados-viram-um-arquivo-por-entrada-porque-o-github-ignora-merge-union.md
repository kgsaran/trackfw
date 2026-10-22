---
status: Accepted
date: 2026-10-22
author: "zeus-tf (desenho) / KG (decisão)"
---

# ADR: Logs append-only compartilhados viram um arquivo por entrada, porque o GitHub ignora merge=union

> Date: 2026-10-22 | Status: Accepted

**REQ:** `docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md`

## Context

Dois arquivos que o trackfw manda escrever recebem toda escrita na **última linha**:

- `<roadmap_dir>/.trackfw-log`, escrito por `trackfw roadmap move` e por `req` (`internal/generators/roadmap.go:264`,
  `internal/generators/req.go:525`, que usa `<req_dir>/.trackfw-log`);
- `docs/agents-working-context.md`, que as instruções geradas para os agentes mandam atualizar
  (`internal/generators/agentfiles.go:92-93` e a seção `## Working context` dos 12 assets em
  `internal/integrations/assets/agents/`).

Duas branches que escrevem nos dois conflitam **sempre**. Isso é propriedade do formato, não azar.

O `.gitattributes` com `.trackfw-log merge=union` (#251, gerado pelo `trackfw init` em
`internal/generators/scaffold.go:2053`) resolve **só o merge local**. Medido nesta sessão:

- no PR #595, o GitHub acusou conflito em `docs/roadmaps/.trackfw-log`, mas o `git merge origin/main` local o
  resolveu sozinho; só o `docs/agents-working-context.md` precisou de resolução manual;
- no #597, de novo o `docs/agents-working-context.md`;
- histórico público: o merge pela web do GitHub não aplica os drivers do `.gitattributes`
  ([rubocop#3594](https://github.com/rubocop-hq/rubocop/pull/3594)). O GitLab e o Bitbucket também não
  ([gitlab-foss#17325](https://gitlab.com/gitlab-org/gitlab-foss/-/issues/17325),
  [BCLOUD-20234](https://jira.atlassian.com/browse/BCLOUD-20234)).

Para o adotante, o resultado é que o botão de merge do PR fica bloqueado a cada PR paralelo, e o
`.gitattributes` que geramos dá uma falsa sensação de resolvido.

## Decision

**Cada entrada passa a ser um arquivo próprio.** Duas branches nunca escrevem no mesmo arquivo, e o
conflito some em qualquer forge, sem depender de driver de merge. (Decisão do KG em 2026-10-22,
entre três opções: arquivo por entrada, manter `union` e documentar, ou inserção ordenada.)

### D1. Log de transições

- **Escrita:** cada transição cria `<dir>/.trackfw-log.d/<AAAAMMDDTHHMMSSZ>-<h8>.log`, onde `<dir>` é o
  mesmo diretório de hoje (`roadmap_dir` ou `req_dir`). O timestamp é em UTC, sem `:` (que o Windows não
  aceita), e `<h8>` são os 8 primeiros hex do SHA-256 da linha. O conteúdo é **uma linha**, no formato
  atual, para reaproveitar o parser. Duas branches que gerem a mesma entrada criam o mesmo arquivo com o
  mesmo conteúdo, e isso não conflita.
- **Leitura, por um ponto único:** o `.trackfw-log` legado (se existir) e todos os `*.log` de
  `.trackfw-log.d/`, ordenados pelo timestamp da linha já parseado com o fuso, com desempate pelo nome do
  arquivo. Os dois parsers atuais (`internal/metrics/metrics.go` e `internal/serve/metrics_log.go`) viram
  um só. Pela Regra Dura, esta ADR não está satisfeita enquanto sobrar um leitor que leia só o arquivo.
- **Legado:** o `.trackfw-log` existente fica **congelado**. Ele é lido, mas nunca reescrito nem migrado.
  A leitura dupla é requisito, e não conveniência: até a próxima release, e em times com versões
  misturadas, um binário antigo continua anexando ao arquivo.
- **`.gitattributes`:** a regra `merge=union` continua, porque protege o legado durante a transição. O
  comentário dela (`scaffold.go:2053`) passa a dizer que ela não resolve no forge, e por quê.

### D2. Contexto de trabalho dos agentes

- **Escrita:** cada entrada vira `docs/agents-working-context.d/<AAAAMMDDTHHMMSSZ>-<agente>-<h8>.md`, com o
  mesmo formato de bloco de hoje (`## <data> — <agente> — <INÍCIO|FIM>: <assunto>` e os bullets).
- **`docs/agents-working-context.md`** fica congelado como histórico, com um cabeçalho de
  redirecionamento no topo. Ele deixa de ser o lugar de escrita.
- **Leitura:** a instrução gerada passa a ser "leia as entradas mais recentes de
  `docs/agents-working-context.d/`". Se um subcomando de leitura for criado (por exemplo, `trackfw context`
  mostrando as N últimas), quem decide é a Wave 0 da REQ, com o caminho escrito no roadmap.
- **Instruções geradas:** `agentfiles.go`, os 12 assets e os goldens mudam juntos.

### D3. Verificação

O teste que prova a ausência de conflito roda **sem `.gitattributes`**, num repositório em `t.TempDir()`:
duas branches fazem transições pelo código real e depois fazem `git merge`. O braço positivo exige merge
limpo e igualdade de conjunto das entradas. O braço contrário exige que o formato antigo, com append nas
duas branches, conflite. Com `.gitattributes`, o teste passaria até no formato antigo, e foi exatamente
assim que o defeito ficou invisível.

## Consequences

- O conflito desaparece no GitHub, no GitLab e no Bitbucket, para os dois arquivos e para todo adotante.
- Os leitores do log passam a ler um diretório. Todo código que percorre `roadmap_dir` ou `req_dir`
  precisa ignorar `.trackfw-log.d/`, como já ignora o `.trackfw-log` (veja `roadmap.go:1040-1046`).
- O repositório ganha muitos arquivos pequenos: um por transição e um por entrada de contexto.
- **Transição:** as cópias instaladas das instruções (os agentes em `~/.claude/agents`, o
  `~/.claude/CLAUDE.md` do usuário) continuam mandando anexar no `.md` até o `trackfw agents update`
  depois da release. Durante essa janela, os conflitos no contexto continuam, e isso não é regressão.
- O histórico antigo fica nos arquivos congelados, sem migração e sem diff gigante.

## Alternatives Considered

- **Manter `merge=union` e documentar o merge local.** Rejeitada: o botão de merge do forge segue
  bloqueado a cada PR paralelo, e o custo recai sobre todo adotante.
- **Inserção ordenada por timestamp e hash, no meio do arquivo.** Rejeitada: reduz, mas não elimina, a
  chance de as duas branches tocarem a mesma região, e é frágil.
- **Migrar o legado para o diretório.** Rejeitada: gera um diff enorme, quebra binários antigos que
  ainda escrevem no arquivo e não traz ganho, já que a leitura dupla é necessária de qualquer forma.
