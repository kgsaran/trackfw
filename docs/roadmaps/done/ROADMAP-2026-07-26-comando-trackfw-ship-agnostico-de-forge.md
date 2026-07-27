---
status: done
date: 2026-07-26
req: "docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md"
squad: ""
---

# Roadmap: comando trackfw ship agnostico de forge

> Created: 2026-07-26 | Status: done

## Context

REQ: docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md
ADR: docs/adr/ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md

O trackfw sabe **validar** mas não sabe **entregar**. Portar o fluxo `git-ship` como comando fecha o
ciclo `validate → ship` e amarra a entrega à cadeia ADR→REQ→ROADMAP: sem roadmap em `wip`, não há
entrega. A abertura de PR/MR deve ser **agnóstica de forge** — o trackfw é open-source e não pode
assumir GitHub.

### Regras invioláveis para todos os MLs

1. **Paridade 3 CLIs:** toda mudança de comportamento existe em Go, Node.js e Python. Ver `docs/cli-parity.md`.
2. **Degradação graciosa é requisito, não conveniência:** CLI de forge ausente ⇒ push concluído e URL
   impressa, exit 0. Nunca falhar por ausência de ferramenta externa.
3. **Self-hosted é caso de primeira classe**, não remendo: `git.empresa.com.br` não é identificável
   pelo host. O campo explícito e o desempate por CI existem desde a fundação.
4. **Nomenclatura correta:** "Merge Request" no GitLab, "Pull Request" nos demais.
5. **Nunca** `git add .`, `git push --force`, nem merge automático.

### Mapa de dependências

```
Wave 1 (1A, agente único) ─ barrier ─> Wave 2 (2A ‖ 2B) ─ barrier ─> Wave 3 (3A ‖ 3B) ─ barrier ─> Wave 4 (4A)
```

> Paralelismo avaliado pela regra aprendida na REQ anterior: MLs só correm juntos se **não
> compartilharem arquivo nem saída de build**. 2A e 2B são paralelos porque 2B vive em pacote novo
> (`internal/forge/`). Ambos rodam `make quality` — devem ser serializados no gate ou o orquestrador
> roda na barrier.

---

## Wave 1 — Fundação: configuração e resolução de forge (agente único)
> Dependências: nenhuma.

### ML-1A — Campo `forge:` e resolver de precedência
**Status:** done
**Files affected:** `internal/config/config.go`, `internal/discover/discover.go` (parse do remote),
novo `internal/forge/resolve.go`, equivalentes em `npm/src/` e `pypi/trackfw/`, mais testes

**Actions:**
1. Acrescentar `Forge string` a `ProjectConfig` (lido de `forge:` no `trackfw.yaml`). Default: vazio.
2. Criar o resolver com a precedência exata do ADR:
   flag `--forge` → campo `forge:` → host de `git remote get-url origin` → CI detectado → **manual**.
3. Parse de host: `github.com` → `github`; `gitlab.com` → `gitlab`; `bitbucket.org` → `bitbucket`;
   `dev.azure.com` e `*.visualstudio.com` → `azure`. Aceitar SSH (`git@host:org/repo.git`) e HTTPS.
4. Desempate self-hosted: host desconhecido + `.gitlab-ci.yml` presente → `gitlab`;
   host desconhecido + `.github/workflows/` presente → `github`.
5. Host desconhecido e sem sinal de CI → modo `manual` (não é erro).
6. O resolver deve reportar **qual fonte decidiu** (para o comando poder explicar ao usuário).

**Acceptance criteria:**
- [ ] `--forge` sobrepõe config; config sobrepõe remote; remote sobrepõe CI
- [ ] SSH e HTTPS resolvem igual para os 4 hosts conhecidos
- [ ] Host desconhecido + `.gitlab-ci.yml` → `gitlab`
- [ ] Host desconhecido sem sinal → `manual`, sem erro
- [ ] Paridade nos 3 CLIs, com os mesmos casos de teste
- [ ] `make quality` verde

---

## Wave 2 — Comando e adaptadores (2 MLs em paralelo)
> Dependências: **barrier** — ML-1A concluído. Arquivos disjuntos: `commands/` × `internal/forge/`.

### ML-2A — Comando `trackfw ship` (fluxo git, sem abrir PR)
**Status:** done
**Files affected:** `internal/commands/ship.go`, equivalentes em npm/pypi, testes

**Actions:** implementar os passos 1–6 do ADR:
1. Branch ≠ `main`/`master` e no padrão `feat|fix|refactor/<slug>` — aborta com mensagem clara.
2. **Validação de governança:** exigir REQ e roadmap em `wip` (reaproveitar o validator, não
   reimplementar). Sem isso, aborta e orienta os comandos a rodar.
3. Detectar squash-merges pendentes em outras branches, avisando sem bloquear.
4. Revisar o que está staged; **nunca** `git add .`.
5. Commit em Conventional Commits, sem sufixo de agente e sem trailer de modelo.
6. `git push origin <branch>`.

**Acceptance criteria:**
- [ ] Em `main`/`master`: aborta, exit ≠ 0
- [ ] Sem roadmap em `wip`: aborta com orientação
- [ ] Nunca executa `git add .` nem `--force` (verificável por teste)
- [ ] Paridade nos 3 CLIs
- [ ] `make quality` verde

### ML-2B — Pacote `internal/forge` — adaptadores
**Status:** done
**Files affected:** `internal/forge/` (novo), equivalentes em npm/pypi, testes

**Actions:**
1. Interface de adaptador com: nome do CLI, argumentos de criação, substantivo ("Pull Request" /
   "Merge Request") e construtor de URL de fallback.
2. Implementar: `github` (`gh pr create`), `gitlab` (`glab mr create`), `azure`
   (`az repos pr create`), `bitbucket` (somente URL — sem CLI oficial estável).
3. Detecção de disponibilidade via `exec.LookPath` (reaproveitar o padrão de
   `externalCommandAvailable` em `discover.go`).
4. **Degradação graciosa:** CLI ausente ⇒ retornar a URL de criação, nunca erro.

**Acceptance criteria:**
- [ ] Os 4 adaptadores implementados, com substantivo correto
- [ ] CLI ausente ⇒ URL retornada, sem erro
- [ ] URL de fallback correta para os 4 forges
- [ ] Paridade nos 3 CLIs
- [ ] `make quality` verde

### ML-2C — Correções pós-auditoria da Wave 2
**Status:** done
**Files affected:** `internal/commands/ship.go`, `npm/src/ship/runner.js`, `pypi/trackfw/ship/runner.py`,
`scripts/check-cli-parity.sh`, `docs/cli-parity.md`, `vault/notes/`

**Actions:**
1. **Explicitar o gate duro** (decisão do ADR, tomada depois do commit do ML-2A): o texto de
   `ship --help` e a **mensagem de erro** do passo 2 devem dizer que a exigência de REQ+roadmap em
   `wip` **não é afetada** pelo modo `lenient` nem pela severidade em `rules:`. Nos 3 CLIs.
2. **Corrigir `scripts/check-cli-parity.sh`** — defeito de gate descoberto na auditoria: em Python
   3.13+ o `argparse` colore a ajuda, e o `grep -E "(^|[[:space:]])<cmd>([[:space:]]|$)"` nunca casa
   com o nome colorido. Local com Python 3.14 reprova; CI com 3.10/3.12 passa.
3. Seção do comando `ship` em `docs/cli-parity.md`, incluindo o comportamento divergente do `validate`.
4. Nota de vault sobre o defeito do gate.

**Acceptance criteria:**
- [ ] `ship --help` e erro do passo 2 mencionam `lenient` explicitamente, nos 3 CLIs
- [ ] `make quality` verde **sem** `NO_COLOR=1`, com Python 3.14
- [ ] O gate continua reprovando comando realmente ausente (falsificação demonstrada)
- [ ] `docs/cli-parity.md` com a seção do `ship`

---

## Wave 3 — Integração e captura da escolha (2 MLs em paralelo)
> Dependências: **barrier** — Wave 2 concluída. Arquivos disjuntos: `commands/ship` × `generators/`+`discover/`.

### ML-3A — Abertura de PR/MR no `ship`
**Status:** done
**Files affected:** `internal/commands/ship.go`, equivalentes, testes

**Actions:**
1. Após o push, resolver o forge e abrir PR/MR pelo adaptador.
2. Corpo referenciando REQ, roadmap e critérios de aceite.
3. Falar o substantivo certo na saída.
4. Modo manual: imprimir a URL e encerrar com exit 0.
5. `--no-pr` para parar após o push.

**Acceptance criteria:**
- [ ] GitLab exibe "Merge Request"; demais, "Pull Request"
- [ ] Sem CLI instalado: exit 0, push feito, URL impressa
- [ ] `--no-pr` para após o push
- [ ] `make quality` verde

### ML-3B — Captura da forge no `init` e no `discover`
**Status:** done
**Files affected:** `internal/commands/init.go` (wizard), `internal/discover/discover.go`,
`internal/generators/` (escrita do `trackfw.yaml`), equivalentes, testes

**Actions:**
1. Pergunta de forge no wizard do `init`, com o valor detectado como default e opção "detectar automaticamente".
2. `discover` preenche `forge:` no `trackfw.yaml` gerado quando consegue detectar.
3. `--forge` não-interativo no `init`, seguindo o padrão de `--identity-preset`.

**Acceptance criteria:**
- [ ] `init` persiste `forge:` no `trackfw.yaml`
- [ ] `discover` detecta e preenche
- [ ] `make quality` verde

---

## Wave 4 — Contrato e documentação
> Dependências: **barrier** — Wave 3 concluída.

### ML-4A — Matriz de testes e documentação
**Status:** done
**Files affected:** testes nos 3 CLIs, `docs/cli-parity.md`, `README.md`, `site/`

**Actions:**
0. **Pendência da Wave 3:** `cmd.SilenceUsage` no `ship` (e demais comandos, se aplicável). Hoje um
   erro de runtime imprime o bloco `Usage:` + `Flags:` do cobra **depois** da mensagem de governança,
   empurrando a orientação acionável para fora da tela. A mensagem é boa; a apresentação a enterra.
   Verificar o padrão dos outros comandos antes de mudar — pode ser comportamento global.
1. **Matriz obrigatória:** 4 forges × (CLI presente / ausente) × (host conhecido / self-hosted).
2. Teste de degradação graciosa com o CLI **removido do `PATH`** — não apenas mock.
3. `docs/cli-parity.md`: comando `ship`, campo `forge:`, tabela de adaptadores.
4. README e `site/` (incluindo `site/en/`).

**Acceptance criteria:**
- [ ] Matriz completa coberta por testes nos 3 CLIs
- [ ] Degradação provada com `PATH` sem o CLI da forge
- [ ] Documentação atualizada
- [ ] `make quality` verde

---

## Log de execução

**2026-07-26 — ML-1A concluído e auditado.**

`make quality` verde; 28 testes de resolver em cada CLI.

**Isolamento dos testes confirmado:** `grep` por `exec.Command`, `git remote` e `os.Getwd` nos testes
de `internal/forge` retorna **vazio**. O resolver recebe `RemoteURL` como entrada em vez de executar
git — os testes não dependem do repositório real e não quebram em fork ou clone SSH. Era a armadilha
bloqueada no prompt.

**Match de host verificado contra falso-positivo por sufixo** — a falha clássica desse parser. A
implementação usa igualdade exata para `github.com`, `gitlab.com` e `bitbucket.org`, então
`github.evil.com` **não** resolve como GitHub. Para Azure usa sufixo **com ponto inicial**
(`.dev.azure.com`, `.visualstudio.com`), que casa `org.visualstudio.com` e rejeita
`evilvisualstudio.com`.

**Além da especificação:** o agente tratou `ssh.dev.azure.com`, forma SSH do Azure DevOps, que eu não
havia listado.

**Ambiguidade documentada, sem impacto prático:** com `.gitlab-ci.yml` e `.github/workflows/`
presentes ao mesmo tempo, o desempate escolhe GitLab. Só alcançável em host desconhecido — em host
conhecido a resolução termina no remote e nunca chega ao CI.

### ⚠️ Lição de processo — marcação de status em worktree compartilhado

A marcação `in progress` do ML-1A foi feita por mim **sem commit** e desapareceu durante a execução do
agente. Nenhum commit do agente tocou o roadmap (`git log -- <roadmap>` mostra só o meu), então a
edição foi descartada por alguma limpeza de working tree do lado dele.

**Regra adotada a partir daqui:** o orquestrador **commita a marcação de status ANTES do spawn**.
Edição não commitada em worktree compartilhado com agente é volátil — a proibição de o agente editar
o roadmap protege o conteúdo, mas não protege alterações que ainda não estão no índice.

**2026-07-26 — Wave 2 concluída e auditada (ML-2A, ML-2B, ML-2C).**

**Falso diagnóstico do orquestrador, registrado como lição.** Uma notificação de falha referente a
agentes antigos me levou a concluir que ML-2A e ML-2B haviam morrido. Vi trabalho não commitado e
nenhum adaptador, e spawnei um terceiro agente de "recuperação" — que teria colidido com o ML-2B,
vivo e escrevendo nos mesmos arquivos. Parei o agente ainda na fase de leitura, sem dano.
**Trabalho não commitado não distingue agente morto de agente ativo.** O sinal correto é o *mtime*
dos arquivos: os adaptadores tinham sido escritos minutos antes.

**Três defeitos de gate encontrados nesta REQ e na anterior** — nenhum pego por CI, todos por
auditoria em cenário real:

| Gate | Defeito |
|---|---|
| `check-integration-cli-parity.sh` | número mágico de itens do catálogo (corrigido na REQ anterior) |
| `branch_has_wip_roadmap` | pune a própria Definition of Done (registrado no vault, sem correção) |
| `check-cli-parity.sh` | quebra com `argparse` colorido do Python 3.13+ e validava menos do que aparentava |

**ML-2C — verificação independente do orquestrador:**
- Com `FORCE_COLOR=1`, o strip de ANSI faz `init` casar; comando inexistente continua reprovando.
- `make quality` verde **sem** `NO_COLOR=1`, em Python 3.14.6.
- A lista do gate ganhou `note` e `ship`, que não eram verificados.
- Mensagem de erro do passo 2 cita `lenient` nos 3 CLIs; `--help` idem.
- Duas notas de vault no repositório, ambas criadas com `trackfw note new`.

**Achado do agente que explica o defeito:** `agents` e `skills` passavam no gate **por acaso** —
aparecem sem cor em texto descritivo (`"List and manage trackfw agents"`), então o grep casava ali e
não no nome do subcomando. Node.js ainda ignora `NO_COLOR` quando `FORCE_COLOR` está setado, por isso
o strip inline foi mantido como segunda camada.

**2026-07-26 — Wave 3 concluída e auditada (ML-3A ‖ ML-3B).**

`make quality` verde **sem** `NO_COLOR` — a correção do ML-2C sustentou.

**Verificação empírica em repositório temporário**, com REQ+roadmap em `wip` e branch de feature:

| Cenário | Saída |
|---|---|
| remote `gitlab.com` | `Forge: gitlab (source: remote)` · **"Merge Request"** |
| remote `github.com` | `Forge: github (source: remote)` · **"Pull Request"** |
| `--forge azure` | `Forge: azure (source: flag)` — precedência da flag confirmada |
| self-hosted `git.empresa.com.br` + `.gitlab-ci.yml` | `Forge: gitlab (source: ci)` · **"Merge Request"** |
| `--no-pr` | para após o push, não tenta abrir nada |
| sem roadmap em `wip` | erro lista a violação **e** os 3 comandos de remediação |
| roadmap em `wip` sem REQ | erro lista a violação específica |

A nota do gate duro aparece na mensagem de erro, como pedido: *"not affected by lenient mode… if
'trackfw validate' passes but 'trackfw ship' aborts here, you likely have lenient mode configured"*.

**Reúso confirmado:** o `discover` chama `forge.ResolveFromRepo` e não duplica o parse de host — as
únicas ocorrências de `github.com` no arquivo são dois imports e o template do workflow de CI. Era o
risco principal da wave: duas lógicas de detecção divergindo.

**Acerto do agente além do pedido:** o `discover` só grava a chave quando `res.Source != "none"`,
evitando escrever `manual` no yaml — o que faria o resolver tratar como configurado.

**Achado de apresentação, não de lógica** → movido para o ML-4A: um erro de runtime dispara o bloco
`Usage:`+`Flags:` do cobra **depois** da mensagem, empurrando a orientação para fora da tela.
`cmd.SilenceUsage` resolve.

**Limitação de teste reconhecida:** a degradação graciosa com CLI ausente **não foi provada
empiricamente** — `--dry-run` encerra antes de consultar disponibilidade, e provar sem dry-run
exigiria um push real. Está coberta por teste unitário com injeção. O ML-4A deve fechar essa lacuna
com a matriz `PATH` sem o CLI.

**Erro do orquestrador, segundo da sessão:** rodei `discover` sem `--init` e conclui que a detecção
falhava nos 3 cenários; depois cortei a mensagem de erro do `ship` com `tail` e reportei defeito de
mensagem inexistente. Nos dois casos, verifiquei o comportamento antes de confirmar que sabia
invocá-lo. Mesma disciplina que exijo dos agentes.

**2026-07-27 — Wave 4 concluída. ROADMAP ENCERRADO.**

O ML-4A morreu duas vezes: primeiro por erro de rede (nada escrito), depois por limite de sessão —
esta com ~400 linhas de Go verificadas no working tree. **Commitei como checkpoint** (`6afbf5e`) em
vez de arriscar perder: três agentes morreram nesta REQ, e deixar trabalho verde solto seria repetir
o erro por escolha. O respawn recebeu o contexto e a instrução de replicar, não refazer.

Entregue: matriz 4 forges × CLI presente/ausente × host conhecido/self-hosted nos 3 runtimes,
`SilenceUsage` em erro de runtime (preservado em erro de uso, que ocorre antes do `RunE`), e o
`--dry-run` passou a **consultar a disponibilidade** do CLI da forge e imprimir a URL de fallback.
Isso fechou a lacuna declarada na Wave 3: a degradação graciosa deixou de ser garantida só por
injeção e virou observável de fora.

### ⚠️ ML-4B — bug de governança pego na auditoria, quase documentado como contrato

`npm/src/ship/runner.js` e `pypi/trackfw/ship/runner.py` resolviam `roadmap_dir` com default
`docs/roadmaps/claude` — o layout por-agente descartado na decisão D4 — enquanto o Go e os **próprios
validators** desses runtimes usam `docs/roadmaps`. Num projeto sem a chave configurada, o mesmo
repositório **entregava pelo Go e era bloqueado pelo npm**. O PyPI ficava incoerente consigo mesmo.

**O agravante foi a resolução proposta:** o agente documentou a divergência em `cli-parity.md` como
*"intentional and preserved"*. Um defeito documentado como contrato deixa de ser defeito aos olhos de
quem vier depois — e ninguém mais o corrige. Com `make quality` verde, isso iria para a `main` como
comportamento oficial de um produto que vende paridade de governança.

**Causa raiz apontada pelo ML-4B:** os runners de npm/PyPI reimplementaram a resolução da chave em vez
de usar o módulo de config, e os testes injetavam `checkGovernance` — nunca exercitando o caminho real.
Corrigido reutilizando o config de cada runtime, com teste de paridade travando o default nos três.

**Verificação empírica do orquestrador, nas duas direções:**

| Cenário (sem `roadmap_dir:` no yaml) | Go | Node | Python |
|---|:-:|:-:|:-:|
| roadmap em `docs/roadmaps/wip/` | Governance: OK | Governance: OK | Governance: OK |
| roadmap em `docs/roadmaps/claude/wip/` | reprova | reprova | reprova |

**Regra derivada:** divergência entre runtimes tem como hipótese padrão **bug**, não contrato.
Documentar divergência como intencional exige ADR — nunca uma nota de rodapé no `cli-parity.md`.

## Acceptance Criteria

- [x] Todas as waves concluídas (1, 2 + 2C, 3, 4 + 4B)
- [x] `trackfw ship` idêntico nos 3 CLIs
- [x] Todos os critérios de aceite da REQ atendidos
- [x] Escopo negativo respeitado (sem merge automático, sem `--force`, sem `git add .`, 4 forges)
- [x] `make quality` verde e `trackfw validate` sem violações
- [ ] `trackfw ship` idêntico nos 3 CLIs
- [ ] Todos os critérios de aceite da REQ atendidos
- [ ] Escopo negativo respeitado (sem merge automático, sem `--force`, sem `git add .`, sem forge além das 4)
- [ ] `make quality` verde e `trackfw validate` sem violações
