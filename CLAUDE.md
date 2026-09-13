# trackfw — Instruções de Projeto (Claude Code)

> Regras globais de workflow estão em `~/.claude/CLAUDE.md` e se aplicam aqui.

## Visão geral

**trackfw** é um CLI de governança de entrega de software open-source.
Cadeia: `ADR → REQ → ROADMAP → backlog/wip/blocked/done/abandoned`

Leia `docs/visao-projeto/VISION.md` antes de qualquer tarefa.
Leia `docs/agents-working-context.md` para o estado atual de trabalho.

## Stack

- **Linguagem:** Go
- **CLI framework:** cobra (`github.com/spf13/cobra`)
- **Wizard:** huh (`github.com/charmbracelet/huh`)
- **Module:** `github.com/kgsaran/trackfw`

## Estrutura

```
cmd/trackfw/        → entry point
internal/commands/  → comandos CLI
internal/generators/→ geradores de artefatos por stack
internal/validator/ → validate + status
docs/               → visão, contexto de trabalho
scripts/            → install.sh
```

## Comandos

```bash
make build          # compila o binário em bin/trackfw
make test           # go test ./...
make lint           # go vet ./...
make quality        # Go + Node.js + Python + contratos de paridade
make install        # instala em /usr/local/bin
```

## Regra Dura — o Go é a implementação única (INVIOLÁVEL)

A partir da v8.0.0, o trackfw tem **uma implementação em Go** entregue por três canais
(`npm`, `PyPI`, `GitHub/Homebrew`). Não existem mais implementações separadas em Node.js e Python.

**Histórico e por quê isso importa:** o trackfw **nasceu em Go**. Os CLIs de Node.js e Python foram
criados **depois, por convenção**, para atender empresas com **restrição de segurança para baixar
executáveis** — elas já têm `npm` e `pip` liberados e um binário solto, não. Os dois existiam para
**entregar o mesmo produto por outro canal**, não para propor comportamento próprio. A v8 faz isso de
forma direta: o binário Go chega pelo canal que a política da empresa já autoriza. Não existem mais
dois artefatos adicionais para manter em paridade.

### 🔴 O Go é a expressão da verdade — por construção

**Na v8, divergência entre runtimes é impossível por construção** — existe uma única implementação.
Mas a regra continua vigente na sua forma original para qualquer código fora de `internal/`:
scripts, templates, documentação de contrato.

**Consequências práticas:**

- Toda mudança de comportamento do CLI vai em `internal/` (Go). Não há outro lugar.
- Scripts e templates em `scripts/` continuam sendo o segundo sítio de acerto — se o script diverge
  do que o binário faz, o binário é a referência.
- O contrato de comportamento continua documentado em `docs/cli-parity.md` — agora como contrato de
  canal (o que cada canal entrega) e contrato de comportamento (o que o binário faz).
- 🔴 **Nunca remova capacidade do Go para simplificar um canal.** O canal existe para entregar o
  produto; não é o produto que se adapta ao canal.

**Exemplo medido que levou à v8 (2026-09-12, issue #310):** os três `trackfw init` geravam
`scripts/trackfw-validate.sh` diferentes — Go/Node com mensagens ao usuário, Python silencioso e com
`set -euo pipefail`. O `set` do Python era objetivamente mais estrito. Convergiu para o Go mesmo
assim. A variante estrita, se for desejada, é mudança no Go primeiro. **Este é o último exemplo de
"divergência entre runtimes" — na v8 não há mais o que divergir.**

## Regra Dura de Reconciliação — todo teste novo declara o que afirma (INVIOLÁVEL)

**Todo ML que entregue teste novo declara, no relatório, qual conclusão do próprio ML aquele teste
afirma — em uma frase.** E a auditoria do arquiteto **verifica esse cruzamento**, não só se o teste
passa.

### Por que esta regra existe

Em 2026-09-05 um ML mediu, escreveu no relatório e registrou no vault que **`ENOTDIR` é
indistinguível de "ausente" no Windows** (`ENOTDIR = ERROR_PATH_NOT_FOUND`). **Na mesma entrega**, ele
criou um teste afirmando o contrário. O teste reprovou no CI de Windows, o ML foi marcado ✅, e o
arquiteto mergeou.

**Quem pegou foi uma auditoria externa**, um dia depois.

Não faltou medição — a medição estava certa e escrita. **Faltou reconciliar o artefato entregue com a
conclusão do próprio relatório.** O mesmo padrão apareceu em outros dois achados da mesma auditoria:

| | medimos | e mesmo assim declaramos |
|---|---|---|
| A1 | `ENOTDIR` indistinguível no Windows | teste afirmando que é distinguível |
| A2 | 7 grafias de vazio | "o vínculo está resolvido" |
| A3 | — | um discriminante que nunca testamos |

### Como aplicar

**No handoff**, o arquiteto inclui a exigência. **No relatório**, o agente escreve a frase, por teste
novo. **Na auditoria**, o arquiteto confronta cada frase com a seção de medição do mesmo relatório.

🔴 **Se não for possível escrever a frase para um teste, o teste não deveria existir.** Um teste que
não sustenta nenhuma conclusão do ML ou é decorativo, ou está afirmando outra coisa — e as duas
possibilidades são problema.

### O que esta regra NÃO cobre

Ela pega **contradição interna** — artefato contra conclusão do mesmo relatório. **Não pega premissa
errada compartilhada** pelos dois: se a medição estiver errada, o teste que a afirma passa na
reconciliação. Para isso serve a barreira independente (`hades-tf`), que reimplementa a partir da
leitura em vez de conferir o diff.

## Regra Dura de Causa Raiz — mesma causa, mesma REQ (INVIOLÁVEL)

**Achado de mesma causa de erro é tratado na MESMA REQ. Nunca vira REQ nova.**

Quando um microlote descobre que o defeito que ele corrige existe também em outros sítios — mesma
causa, mesmo mecanismo, mesma ADR governando —, esses sítios entram como **novo ML na REQ vigente**,
não como REQ nova.

### Por que esta regra existe

Medido em 2026-09-06: **59 ocorrências de "REQ própria" espalhadas por 30 roadmaps.** Cada uma é um
defeito **medido, localizado e não corrigido**, empurrado para uma REQ que entra numa fila de 36
abertas — onde espera priorização e **se perde**.

O efeito composto é o que o usuário nomeou:

> *"por isso temos essa enxurrada de REQs abertas, e pior: o erro permanece até priorizarmos a nova
> REQ gerada, e se perde na vastidão de REQs abertas."*

Duas consequências, e a segunda é pior:

1. **O backlog cresce por construção** — cada correção gera de uma a três REQs novas.
2. 🔴 **O defeito continua vivo em produção**, agora com a aparência tranquilizadora de estar
   "registrado". Registro não é correção.

### O que NÃO justifica abrir REQ nova

- **"Atribuição de causa"** — não misturar mudanças para saber qual produziu qual efeito. Isso
  justifica **ML separado** (ou commit separado), **não REQ separada**. Confundir os dois foi o erro
  que originou esta regra.
- **"Está fora do escopo declarado"** — se a causa é a mesma, o escopo estava **estreito demais**.
  Corrija o escopo da REQ vigente; não abra outra.
- **"É superfície diferente"** — parser vs. renderizador vs. escrita são superfícies diferentes do
  **mesmo** defeito. Mesma causa, mesma REQ.

### Mesmo SINTOMA também fica no mesmo roadmap — até a medição dizer o contrário

Acrescentado por decisão do usuário em 2026-09-06:

> *"Descobertas novas, se forem do mesmo sintoma ou mesma causa, devem ser fechadas dentro do mesmo
> roadmap."*

**Mesma causa → obrigatoriamente o mesmo roadmap.** Sem exceção.

**Mesmo sintoma → investiga no mesmo roadmap.** Se a medição mostrar que a causa é outra, pode
separar — 🔴 **mas só com a medição escrita**, nunca por presunção.

**Por que a distinção importa:** agrupar por sintoma foi o que produziu o erro mais caro desta
campanha. O grupo do `IsAbs` foi estimado em **14 falhas** e entregou **2**, porque falhas cuja causa
real era escape de aspas foram atribuídas a ele pelo sintoma parecido ("teste de guard falha com
caminho").

Então: o sintoma **inicia** a investigação junto; a **causa medida** é o que autoriza separar. O ônus
é de quem quer dividir, não de quem quer manter junto.

### Quando é legítimo abrir REQ nova

Somente quando a **causa é outra**. O teste é o mesmo da triagem por mecanismo:

> *"Se eu corrigir esta causa, exatamente estas falhas fecham — e nenhuma outra."*

Se o sítio novo **não fecha** com a correção da REQ vigente, é outra causa. Aí sim, REQ própria — e a
diferença de mecanismo fica **escrita**.

### Interação com as ADRs

🔴 **Uma ADR com decisão de "ponto único por runtime" não está satisfeita enquanto sobrar sítio.**
Fechar o roadmap com sítios conhecidos e não corrigidos marca como concluído algo cujo critério não
foi atendido — e é o achado A1 da auditoria externa de 2026-09-05, que este projeto já pagou uma vez.

### E o mesmo PR

**Mesma causa → mesma REQ → mesmo PR.**

A correção dos sítios descobertos entra **no PR que já está aberto** para aquela causa. Não se abre
PR novo, e não se mergeia o PR "parcial" prometendo o resto depois.

Motivo: um PR mergeado com a correção pela metade **fecha a janela de atenção**. O revisor viu, o
gate passou, a issue foi marcada — e os sítios restantes herdam a mesma fila onde as REQs órfãs se
perdem. O PR aberto é o que mantém a causa raiz visível até estar inteira.

Consequência prática: **um PR pode ficar aberto mais tempo, e isso é aceitável.** O custo de esperar
é menor que o de um defeito que sobrevive porque o PR que o fechava já foi mergeado.

### Como aplicar

Ao encontrar sítio de mesma causa: **acrescente um ML à REQ vigente**, mova o roadmap de volta para
`wip` se já tiver sido fechado, **mantenha o PR aberto** até todos os sítios entrarem, e registre no
roadmap **por que** o escopo original não os previa.

## Regras específicas

- **Nunca commitar na `main` sem PR** (mesmo sendo projeto novo)
- **Build obrigatório** após qualquer alteração: `go build ./...`
- **Atualizar `docs/agents-working-context.md`** ao iniciar e encerrar cada ciclo

## Instalação de skills de terceiro (`trackfw <skills|agents> third-party`)

Instala skills externas (via URL) em duas fases obrigatórias, com um ponto de revisão humana entre
elas — **nunca instale sem revisar o conteúdo em quarentena antes de aprovar**:

1. `trackfw <skills|agents> third-party fetch <url>` — baixa o conteúdo e grava um registro de quarentena em
   `.trackfw/thirdparty-quarantine/<checksum>.json`. Nada é instalado ainda.
2. **Revisão humana obrigatória** do conteúdo em quarentena, seguida da aprovação (que grava
   `.trackfw/thirdparty-provenance.json` — nenhum comando do CLI escreve essa aprovação sozinho).
3. `trackfw <skills|agents> third-party install --checksum <sha256> --targets <...>` — só instala se houver
   aprovação de provenance correspondente ao checksum.

O checker de markers usado em `fetch` é uma tripwire para o caso óbvio, não uma defesa contra um
adversário competente (não cobre paráfrase, indireção, fragmentação, homoglifos ou conteúdo
auto-modificável depois de aprovado). Detalhes completos, os 3 schemas JSON e as garantias/limites
da regra `trackfw validate` `thirdparty_artifact_has_provenance` estão em
`docs/cli-parity.md` (seção `trackfw <skills|agents> third-party`).

## Sinalização de Atenção para o Board (`trackfw serve`)

Quando um agente precisar de confirmação ou ação do usuário durante uma implementação,
**escreva o arquivo `.trackfw-attention.json`** na raiz do diretório de roadmaps
(ex: `docs/roadmaps/.trackfw-attention.json`).

O `trackfw serve` monitora esse arquivo a cada 8 s e exibe um banner de alerta no board.

### Formato obrigatório

```json
{
  "roadmap": "nome-exato-do-arquivo.md",
  "ml": "ML-2A — Título do microlote",
  "message": "Descreva objetivamente o que você precisa do usuário.",
  "level": "action_required",
  "timestamp": "2026-06-18T10:30:00Z"
}
```

| Campo | Obrigatório | Valores | Descrição |
|---|---|---|---|
| `message` | ✅ | string | Pergunta ou informação clara para o usuário |
| `level` | ✅ | `"action_required"` \| `"info"` | `action_required` = banner âmbar; `info` = banner azul |
| `timestamp` | ✅ | ISO 8601 UTC | Usado para deduplicar dismissals no browser |
| `roadmap` | recomendado | basename do `.md` | Marca o card correspondente no board |
| `ml` | opcional | string | Microlote em andamento |

### Quando usar

- Agente encontrou ambiguidade bloqueante que não pode resolver com o contexto disponível.
- Agente precisa escolher entre duas abordagens e o impacto é significativo.
- Agente gerou artefato que requer revisão antes de continuar.

### Quando NÃO usar

- Dúvidas que podem ser resolvidas lendo o roadmap, CLAUDE.md ou o código existente.
- Decisões de baixo risco (nomenclatura, formatação, ordem de campos).

### Limpeza após resolução

**Apague o arquivo** assim que a atenção não for mais necessária — o banner desaparece automaticamente.

```bash
rm docs/roadmaps/.trackfw-attention.json
```

---

## Protocolo de Release (tag)

Ao gerar uma nova tag, o fluxo obrigatório é:

1. **Determinar a próxima versão** com base no SemVer e nos commits desde a última tag:
   - `git tag --sort=-version:refname | head -1` — última tag
   - `git log <última-tag>..HEAD --oneline --no-merges` — commits incluídos

2. **Gerar o changelog** a partir dos commits desde a última tag, agrupando por tipo:
   - `feat` → What's New / `### Added`
   - `fix` → Fixes / `### Fixed`
   - `refactor/perf` → `### Changed`
   - `docs/chore/test/style/build/ci` → omitir ou agrupar em "Internal"
   - Indicar Breaking Changes explicitamente (ou "Nenhum" se retrocompatível)

3. **Atualizar `CHANGELOG.md`** (raiz do projeto, formato [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)):
   inserir uma nova seção `## [x.y.z] - YYYY-MM-DD` **no topo** do arquivo,
   com o mesmo agrupamento do passo 2. Este é o mesmo PR do bump de versão
   (`chore(release): bump version files to x.y.z`) — nunca um commit separado.
   `CHANGELOG.md` é arquivo único na raiz; não duplicar em `npm/` ou `pypi/`.

3.5. **Verificar concordância entre required checks, declaração e workflows** (pré-condição obrigatória da tag):
   ```bash
   make check-required-full
   ```
   Executa a verificação completa D/R/W (declared vs required_status_checks da API vs workflow checks).
   Requer credencial de mantenedor (`gh auth login` com scope `repo`). Falha fatal se R não for legível.
   Por que aqui: a tag afirma que o estado do repositório está íntegro. R (o que bloqueia merge) vive
   fora do repositório e muda episodicamente; o momento certo para verificar é imediatamente antes de
   selar o estado com a tag — o mantenedor tem a credencial por definição, e o custo é uma chamada de API.
   CI verifica apenas D\\W (sem token, via `check-required-checks`); D\\R e R\\W são responsabilidade deste passo.

4. **Criar a tag anotada** com o changelog no corpo da mensagem:
   ```bash
   git tag -a v<x.y.z> -m "<changelog>"
   git push origin v<x.y.z>
   ```

5. **Nunca criar tag diretamente na main sem PRs merged** — a tag representa o estado pós-merge.

> Critério de versão: feat breaking → major; feat não-breaking → minor; fix/patch → patch.

<!-- trackfw:rules:start -->
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: `ADR → REQ → ROADMAP` · States: `backlog / analyzing / wip / blocked / done / abandoned`

### Agent Protocol
1. **Before any implementation (mandatory):** create governance artifacts FIRST, then branch:
   `trackfw req new "title"` → `trackfw roadmap new "title"` → `trackfw roadmap move <name> wip` → `git checkout -b feat/<branch>`
   ❌ Never create a branch before REQ + ROADMAP are in wip/
   ❌ Never defer REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ✓ `trackfw validate` enforces this via `branch_has_wip_roadmap` rule (v2.7.0+)
2. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`
3. **After finishing:** update `docs/agents-working-context.md` with what changed
4. **Before PR:** `trackfw validate` must pass
5. **ML lifecycle — mandatory:**
   - Starting a ML: edit roadmap `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` + commit.
   - Completing a ML: edit roadmap → `**Status:** ✅ Concluído` + include in ML commit.
   - Analyzing a roadmap: move from `backlog/` to `analyzing/`; to `wip/` only when coding starts.
6. **Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.**

### Attention Signal (when you need user input during a task)
Write `docs/roadmaps/.trackfw-attention.json`:
```json
{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}
```
Delete the file when resolved. Visible as a live banner in `trackfw serve`.

> **Windsurf users:** before asking the user a question or requesting approval, write
> `<roadmap_dir>/.trackfw-attention.json` manually — there is no automatic hook for this.
> Delete the file after the user responds.

### Architecture Directives (mandatory)
- **3-layer separation:** frontend / backend / database — never mix concerns
- **No in-memory data:** always database + ORM (never arrays/globals for persistence)
- **Auth from day 1:** never defer — refactoring auth later is very costly
- **Docker + .env from day 1:** containerize early; all config via env vars
- **2-layer validation:** frontend (UX) + backend (security) — never only one
- **API-first:** define OpenAPI contract before coding frontend/backend integration
- **Security wave:** include a red-team review wave in every feature roadmap
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use `/trackfw:architect` to define stack before the first REQ

### Key Commands
- `trackfw context` — current governance state (always run first)
- `trackfw status` — all artifacts and states
- `trackfw validate` — governance consistency check
- `trackfw roadmap move <name> <state>` — transition roadmap state
- `trackfw serve` — live Kanban board at http://localhost:4080
<!-- trackfw:rules:end -->

---

## Este repositório é uma cópia consumidora, não o upstream

O produto vem de `kgsaran/trackfw`, adicionado como remote `upstream`. Este repo **consome** o
trackfw e o usa para governar a si mesmo; ele não é a linha principal do produto.

**Atualizar — use o script, não o `git merge` cru:**

```bash
git fetch upstream
./bin/trackfw branch new chore/<slug>
scripts/upstream-sync.sh
```

O `upstream-sync.sh` mescla, **retém `docs/` e `vault/`**, prova a retenção por efeito, reporta a
proporção produto/governança e verifica que o `validate` não mexeu. Não commita nem faz push por
padrão: o commit carrega a medição, e quem mede é quem escreve. Aborta e devolve a árvore se sobrar
conflito de produto ou se a retenção não puder ser provada.

**Por que não `git merge` direto.** Com `roadmap_namespacing: by_agent`, o git detecta os roadmaps
flat do upstream (`docs/roadmaps/wip/`) como **renomeação** dos nossos (`docs/roadmaps/claude/done/`)
e produz uma enxurrada de `rename/delete` — **23 conflitos** no merge de `4f0ad33`. Resolver um a um
é caro e erra fácil, e a `ADR-2026-08-29` já decidiu o resultado: não há julgamento a fazer.

A ancestralidade que torna o merge possível veio daquela ADR, com um merge de históricos. Antes
disso o repo era cópia por ZIP, sem ancestral comum, e `git merge` se recusava a rodar — o que
deixou este repo cinco majors atrás sem ninguém perceber.

**A proporção é o discriminante.** Um merge que fecha com muito mais que uma mão-cheia de arquivos
de produto indica que algo de `docs/` passou. Medido nos merges reais: `4f0ad33` trouxe 4 de produto
e reteve 37; `6b3ba49` trouxe 42 e reteve 10.

**Falsificação:** `scripts/check-upstream-sync-falsify.sh` exercita o script contra esses dois
merges históricos mais dois controles negativos. A propriedade verificada é a **invariante** —
retido ⊆ `docs/` ∪ `vault/`, e todo o resto trazido —, não a contagem: a contagem à mão errou nos
dois casos.

## Gate de layout de REQ (`scripts/check-req-layout.sh`)

A `ADR-2026-09-03` D1 decide que **REQ não tem dimensão de estado** — `backlog`/`wip`/`done` são
conceito de roadmap; REQ tem `status` no frontmatter. O layout canônico em `by_agent` é
`req_dir/<agente>/*.md`, **um nível**.

```bash
bash scripts/check-req-layout.sh
```

Lê `req_dir` do `trackfw.yaml` (nunca chumbado), varre **recursivamente**, imprime o denominador e
falha se varrer zero.

🔴 **`docs/req/` fica fora por decisão escrita.** Não é o nosso `req_dir`, e três daqueles arquivos
são **fixture de teste do produto**, lidas por caminho literal nos 3 runtimes — removê-las quebra
`go`, `node` e `python`, medido em 2026-09-05.

O gate existe porque a ADR foi aceita e, cinco dias depois, **7 REQs estavam em pasta de estado** com
o `validate` dizendo `✓ No violations found`. E o custo não era a desarrumação: aquele nível a mais
produziu um **ponto cego de medição** que fez um número publicado sair errado.

## `req new` e `roadmap new` exigem `--agent` aqui — são três namespaces

Desde o merge da [#330](https://github.com/kgsaran/trackfw/pull/330) do upstream (2026-09-12), em
projeto `by_agent` com **dois ou mais** agentes o comando **falha nomeando as opções** em vez de
escolher `agents[0]` em silêncio. O nosso `trackfw.yaml` declara `[apolo, artemis, claude]`, então
isto vale para todo uso diário:

```
trackfw req new "titulo"                  rc=1
  Error: by_agent project has multiple agent namespaces (apolo, artemis, claude): use --agent

trackfw req new "titulo" --agent claude   rc=0
  created docs/requisições/claude/REQ-2026-09-12-titulo.md
```

**Não é regressão — é a correção da nossa [#320](https://github.com/kgsaran/trackfw/issues/320).** O
comportamento antigo escrevia em `apolo/` sem avisar, que é exatamente o defeito que relatamos. A
flag passou a existir em `req new` **e** `roadmap new` nos três runtimes, e o `roadmap new --req`
agora **herda o agente da REQ** em vez de ignorá-lo.

Medido por efeito em projeto temporário com os mesmos três agentes, antes de mesclar.

## Branch `feat/fix/refactor` custa 3 jobs vermelhos por motivo falso

🔴 **Não leia estes três como regressão** — `python (3.10)`, `python (3.12)` e `windows-full-suites`.
Em branch `feat/`, `fix/` ou `refactor/` eles reprovam **sem que a árvore tenha defeito**. Medido em
2026-09-11, na PR #112 deste fork, com controle nas duas direções:

| cenário | resultado |
|---|---|
| `main`, arquivos da `main` | 30 passaram |
| `main`, **com os arquivos da PR por cima** | 30 passaram |
| branch `fix/…`, mesma árvore | **8 falharam** |
| branch `chore/…`, **mesma árvore, só o nome mudou** | 64 passaram |

A quarta linha é o discriminante: **nenhum byte mudou** entre ela e a terceira.

**O mecanismo.** `validate_branch_has_wip_roadmap` (`pypi/trackfw/validator.py:1990`) só se aplica a
branch `feat/fix/refactor` (2014) e, quando nenhum roadmap casa, devolve uma **`str` crua** (2025). O
`_apply_rule` (4205) documenta no docstring que recebe *"lista de dicts"*; o `_enrich_items` (429)
deixa o item passar intacto pelo `else` final. A `str` entra em `violations` no meio de dicionários, e
todo teste que faz `[item["message"] for item in items]` estoura com `TypeError`.

O que faz a regra disparar **dentro de um teste** é a assimetria de resolução:

```
cwd = raiz do repo  ->  wip_dirs = docs/roadmaps/<agente>/wip   matched=True   candidatos=68   (nao dispara)
cwd = tmpdir        ->  wip_dirs = docs/roadmaps/wip (default)  matched=False  candidatos=0    (dispara)
```

O nome da branch vem do git do repositório real; os roadmaps são procurados a partir do `cwd`, que na
fixture temporária não tem `trackfw.yaml` e cai no default flat.

**Os três runtimes devolvem string crua** nesta regra — Go `[]string`, Node array de string, Python
lista. O que é só do Python é ter teste que lê `item["message"]`: na mesma execução de CI, `go` e
`node` ficaram verdes e os dois de Python, vermelhos. Não é divergência de implementação.

**O `windows-full-suites` cai pelo mesmo motivo**: o ratchet acusa as 8 falhas novas de Python. 🔴 As
**10 falhas de Go** que aparecem nesse job são **idênticas por nome** às do último run verde da
`main` — ruído conhecido, tolerado, e não têm relação com isto.

**O que fazer.** Trabalho sem uma linha de produto — gate, script, governança — vai em `chore/` ou
`docs/`, que a regra não alcança, e que já é o tipo correto para esse trabalho. Se a branch precisar
mesmo ser `fix/`, conte com os três vermelhos e **compare por nome** contra o último run da `main`
antes de investigar qualquer um deles.

Reportado no upstream como sítio da mesma causa:
[#261](https://github.com/kgsaran/trackfw/issues/261#issuecomment-5641879945). O arquivo é o mesmo da
`main` dele (blob `26a5f6c854bd`): não há divergência local envolvida.

## Ponto cego local: os 3 gates de PATH curado não rodam nesta máquina

`check-ship-force-parity.sh`, `check-push-force-parity.sh` e `check-release-tag-parity.sh` **não
verificam nada no Windows daqui.** Não é "alguns cenários falham": é **zero cenário executado**.

```
gate                        linhas  asserts grep -qF  runtimes         executados
check-ship-force-parity        626              11    go8 node8 py6         0
check-push-force-parity        646              11    go8 node8 py6         0
check-release-tag-parity      1557              34    go8 node20 py9        0
                                                 56 afirmacoes  ·  0 rodam
```

🔴 **Não leia o `exit 1` deles como regressão.** Medido em 2026-09-09 com controle nas duas direções:
falham igual **antes e depois** de qualquer merge. A causa é ambiente, e são **dois** bloqueios
empilhados — descobrir o segundo exigiu destravar o primeiro:

1. **`ln -s` degrada para cópia** no Git Bash sem `winsymlinks`. O link do `node` passa (vira cópia);
   o do `python3` morre com `Permission denied`, porque aqui `python3` resolve para o *app execution
   alias* da Windows Store, que é ele próprio um symlink. Morre no setup, linha ~126.
2. Com um `python3` real, a **guarda de vacuidade** reprova dizendo `git does not resolve`. Não é o
   git: é o `python3` **copiado** que não carrega a `python312.dll`, porque o `NO_FORGE_PATH` exclui
   de propósito o diretório de instalação do Python. Mesmo mecanismo que o comentário do próprio
   script documenta para o `git.exe`.

**O ramo Windows desses gates não tem cobertura em CI nenhum** — `Makefile:58,60,61` os põe em
`parity-rest`, e `quality.yml:624` roda `make parity-rest` no job `parity-other-gates`, que é
`runs-on: ubuntu-latest`. Em POSIX o `ln -s` é symlink de verdade e o ramo é outro.

Reportado no upstream: [#307](https://github.com/kgsaran/trackfw/issues/307). Mesma causa do
[#304](https://github.com/kgsaran/trackfw/pull/304) — wrapper dependente de DLL no Windows —, outra
superfície, então é ML na `REQ-2026-09-03` dele, não REQ nova.

**O que fica sem cobertura local** são 56 afirmações, e as caras são as de segurança do
`release-tag`: `Scenario 11` (ataque ao ref derivado de symref), `15` (object-absent), `16`
(content-from-commit-provenance), `17` (refs-replace-bypass). Essas só rodam no CI dele.

**Mérito do gate, que é por que isto é nota e não alarme:** a guarda **recusa** em vez de rodar
cenário vacuo. O ponto cego é real, mas nunca vira verde falso.

🔴 **Duas armadilhas de medição, para não serem repetidas:**

- **Controle contaminado pelo PATH ambiente.** A primeira medição disse "a cópia do `python3`
  inicia" — e iniciava, porque o PATH ainda tinha o diretório do Python e o Windows achou a DLL por
  ali. Só isolando o PATH a falha aparece.
- **Script copiado para o scratchpad não vale como controle.** Estes scripts calculam `ROOT_DIR` a
  partir do próprio caminho; rodados de fora da árvore eles saem `1` por não achar o `trackfw.yaml`
  — **exit code certo pelo motivo errado**. Aconteceu três vezes em 2026-09-09. Rode de dentro de
  `scripts/`.

## Lint de predicado de SO em sítio de classificação (`scripts/check-os-predicate-classification.sh`)

Implementa o **AC2** da `REQ-2026-09-05-onda-2`, com o discriminante da
`ADR-2026-09-09-predicado-de-so-em-sitio-de-classificacao`: **a origem do argumento**, não a intenção
do autor.

```bash
bash scripts/check-os-predicate-classification.sh
```

```
240 sitios varridos
112 com arquivo de teste   (D5: reportado a parte, nunca somado)
 57 D1 travessia           (o predicado recebe um valor de ERRO)
  9 D3 costura             (constante nomeada OU plataforma passada como argumento)
 46 comentario             (inclui prosa dentro de docstring do Python)
 16 D2 CLASSIFICACAO       em 9 arquivos declarados
```

Os cinco somam 240 exatamente — o denominador reconcilia, não sobra resto.

**O ML-1H (2026-09-11) alargou o lint em três frentes, e a terceira corrigiu o próprio acervo:**

1. **Nove predicados, não seis.** Entraram `runtime.GOOS`, `sys.platform` e `platform.system()` —
   44 sítios que os dois instrumentos não viam, porque o lint e o `measure-os-predicate-sites.sh`
   compartilham a mesma lista.
2. **O D3 reconhece a costura pela ORIGEM, não pela forma.** Além da atribuição a constante nomeada,
   agora conta a plataforma **passada como argumento** (`openBrowser(process.platform, url)`). A #321
   do upstream reescreveu a mesma costura de uma forma para a outra, e o lint mudou de veredito sem o
   código mudar de natureza. Falsificado: no produto a regra nova casa **só** o `serve.js:280`, e
   `foo(runtime.GOOS == "windows")` não casa — comparar no próprio sítio é classificar.
   🔴 A sonda achou um furo **antes do merge**: `strings.Contains(runtime.GOOS, "win")` tem a forma
   de argumento e o ato de comparação, e a primeira versão da regra o aceitava como costura — seria
   fechar o ratchet trocando `==` por um ajudante de string. A regra passou a recusar a plataforma
   que entra num **comparador**, por lista literal e curta; comparador novo e não listado volta a ser
   aceito, e o remédio é acrescentá-lo à lista, nunca alargar a regra.
3. 🔴 **Prosa dentro de docstring não é sítio.** O classificador só olhava o início da linha, então
   contava como classificação sete linhas de texto que citam `os.path.isabs` e `os.name` ao explicar
   a ADR. Foi por essas linhas que `pypi/trackfw/validator.py` estava no baseline — **um arquivo
   declarado por um sítio que nunca existiu.** Ele saiu.

**Limite declarado:** o reconhecedor de docstring conta aspas triplas duplas. Medido em 2026-09-11:
zero ocorrências de `'''` no escopo varrido, e o dia em que houver, o sítio aparece como classificação
não declarada — que é o lado seguro de errar.

**Duas leituras inline entraram no baseline** (`pypi/trackfw/homedir.py`, `pypi/trackfw/tty.py`):
`if sys.platform == "win32"`, um sítio por arquivo. Não são classificação de string autorada; o lint
ainda não tem classe para leitura inline de plataforma, e as reporta sob D2 por falta de classe mais
fina. O motivo está escrito no baseline, como manda o D4.

🔴 **É um RATCHET, não uma varredura de limpeza.** Os 16 sítios de classificação estão **todos em
produto do upstream**; corrigi-los aqui criaria divergência, que hoje é zero. O escopo negativo da
REQ decide: achado vira **issue**, não correção local. O gate congela os conhecidos **com motivo por
arquivo** e reprova o próximo.

**Granularidade de arquivo, não de linha** — número de linha muda a cada merge do upstream e o
baseline viraria ruído.

**Duas guardas de vacuidade**, e a segunda é a que importa: se **zero** sítios saírem por D1, o
classificador parou de casar e **tudo** viraria classificação. O `os.IsNotExist` recebe `err` em
todos os sítios de código medidos — zero ali é derivação quebrada, não limpeza.

🔴 **O gate só vê conteúdo rastreado** (`git grep`). Um sítio novo em arquivo não commitado passa —
medido em 2026-09-10, quando a primeira falsificação deu `exit 0` por eu ter plantado sem `git add`.
Em CI isso não é limitação, porque lá tudo chega commitado.

## Gate de REQ `done` com critério aberto (`scripts/check-req-done-com-criterio-aberto.sh`)

Acusa REQ **nossa** marcada `done` que ainda tem critério de aceite em aberto.

```bash
git fetch upstream && bash scripts/check-req-done-com-criterio-aberto.sh
```

**Medido em 2026-09-10: eram dez, com 56 critérios substantivos em aberto.** Duas delas escreviam no
**próprio texto do critério** que ele *"não foi atingido"* — com a REQ em `status: Done`. É a Regra
Dura de Reconciliação aplicada ao acervo em vez de ao microlote.

**Três discriminantes, cada um por um erro medido:**

1. **Herdada fica fora por construção**, não por allowlist — mesma derivação do
   `check-inherited-req.sh`. Acusar as 28 dele seria pedir que marcássemos entrega que não é nossa.
2. **Só `status: done` entra no alvo.** REQ `Open` com critério aberto é trabalho em curso, não
   contradição. 🔴 A primeira derivação deu 64 ACs por não ter isto — e incluía a própria REQ que
   define o alvo.
3. **Só checkbox sob bloco de critério**, e **fora de cerca de código**. 🔴 `- [ ]` dentro de
   ``` é citação: são 3 no acervo, e contá-los inventa critério que ninguém escreveu.

O gate oferece **duas saídas e recusa a terceira**: marque nomeando o sítio que comprova, ou a REQ
deixa de ser `done`. Reescrever o critério para caber no estado atual não é saída — é fabricar
histórico.

**Vereditos possíveis, e o que cada um exige:** `(a) entregue` nomeia o sítio no produto;
`(b) não entregue` reabre a REQ; `(c) caducou` escreve o que mudou no mundo; `(d) não verificável
aqui` declara o que faltaria. 🔴 **`(d)` é resultado, não desculpa** — dos 56, **11** caíram nele, e
**cinco** por um motivo só — a maior causa isolada, não a maioria: os ACs exigiam comparação contra **lista nomeada** de corridas de agosto
que **nunca foram versionadas**. Uma REQ cujo critério depende de artefato não versionado é
inauditável por construção.

Como os outros, é **nosso**, e **não tem alvo no `Makefile`** pelo mesmo motivo.

## Gate de REQ herdada do upstream (`scripts/check-inherited-req.sh`)

A `ADR-2026-08-29` decide que **a governança do upstream não é importada**. Vinte e oito REQs do
`kgsaran/trackfw` já estão no nosso `req_dir` — entraram em 2026-06-28, quando este repo era cópia
por ZIP, antes daquela decisão.

```bash
git fetch upstream && bash scripts/check-inherited-req.sh
```

Deriva por `git cat-file -e upstream/main:docs/req/<basename>`, lê o `req_dir` do `trackfw.yaml`,
varre recursivamente e **congela o conjunto**: uma 29ª herdada reprova. Não resolve as 28 — a
`REQ-2026-09-09-governanca-do-upstream` trata o destino delas —, impede que o próximo merge
acrescente ao problema enquanto ela está aberta.

**Não faz `git fetch` sozinho, de propósito.** Um gate que muda o estado que ele mede deixa de ser
gate. Sem a ref ele **falha dizendo o motivo**, em vez de derivar zero e passar.

**O número que este gate existe para produzir já foi publicado errado três vezes** — 6·54, 8·70,
14·109 —, cada vez por uma varredura digitada na hora e mais estreita que o alvo. O valor derivado é
**23 REQs · 227 ACs**. A tabela das quatro medições, com o que estava estreito em cada uma, está na
REQ.

**A guarda de reconciliação é o que separa este gate de um relatório.** Ele conta checkbox aberto de
duas formas — sob bloco de critério, e no arquivo inteiro — e **nomeia a diferença** em vez de
escolher um número. Foi ela que pegou um defeito no próprio detector: `## Critérios de Aceite`
seguido de `### Bloco A` zerava o estado, e `[eé]` numa classe de caractere não casa UTF-8 no awk
desta máquina. As duas somadas davam `0 sob critério` — verde de aparência limpa.

Hoje o resíduo declarado é **2 checkboxes** em `REQ-roadmap-ai-generation-2026-06-11.md`, dentro de
um template de roadmap embutido na REQ. São placeholder, não critério — e o gate não adivinha isso,
ele aponta.

**Ele também exige a PROCEDÊNCIA no frontmatter de cada uma das 28** —
`upstream_origin: "kgsaran/trackfw:docs/req/<basename>"`. É a decisão do ML-2A da mesma REQ, tomada
depois que a medição fechou as saídas: remover está vetado em 28 de 28 (26 pelo snapshot congelado do
barrier, 2 por ADR nossa), e mover está proibido pelo escopo negativo. Sobrou declarar — e a
declaração tem gate, senão alguém apaga a linha e nada acusa.

🔴 **A declaração vive só no frontmatter, e isso foi medido.** Quando as 28 edições entraram, o
`req list` dos três runtimes lia o status de uma linha do **corpo** (`> Date: … | Status: <status>`),
não do frontmatter — e uma nota em blockquote abaixo do H1 poderia virar o "status" da REQ.
Falsificado por efeito na época: `req list` antes e depois das 28 edições saiu byte a byte idêntico.

**Isso mudou com o [#341](https://github.com/kgsaran/trackfw/pull/341) do upstream (2026-09-12):** o
`req list` passou a ler o `status:` do **frontmatter**, e só cai no corpo quando o frontmatter não tem
o campo. Medido com o binário antes e depois do merge, no mesmo acervo:

```
70 REQs listadas, casadas por nome   48 mudaram · 22 iguais
  unknown          -> status real      33
  texto do corpo   -> status real       6    (ex.: "<status>` (regex ou strings.Split)")
  status do corpo  -> status do frontmatter, que discordava   9
depois: 0 unknown · 0 texto de corpo tomado por status
```

🔴 **O limite que sobra é o fallback:** REQ **sem** `status:` no frontmatter continua lendo o corpo,
então o risco do blockquote vale para ela. Hoje nenhuma das 70 cai nisso — mas é propriedade do
acervo, não garantia do comando.

Como os outros, é **nosso**, e **não tem alvo no `Makefile`**, pelo mesmo motivo da seção abaixo.

## Ratchet de Windows do upstream: onde roda, e por que fica fora do agregador

O gate que confere as listas de falha **por nome** é o `scripts/check-windows-known-failures.py` do
**upstream**, lido inteiro no ML-0A da `REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados`.
Convergimos para ele em vez de construir o nosso: seria a segunda solução para um problema que já tem
uma.

| parte | onde roda | custo medido |
|---|---|---|
| o ratchet, sobre as três suítes | `windows-full-suites`, no `quality.yml` compartilhado | job de 488 a 604 s; o ratchet em si, 0 a 1 s |
| o self-test | `local-gates.yml`, só nosso, em `ubuntu-latest` | 0 s |

Custo por suíte, nos três runs medidos em 2026-09-11 (`34603239796`, `34598894601`, `34596059692`):
Go 136 a 183 s, Node 128 a 198 s, Python 110 a 132 s.

**Fica fora do `run-local-gates.sh`, e o motivo não é o custo:**

1. O veredito precisa dos artefatos das **três suítes completas rodadas no Windows**. Sem eles, a
   guarda de artefato do próprio ratchet reprova — de propósito.
2. A lista é de falhas **de Windows**. O agregador roda em `ubuntu-latest`, onde ela não diz nada.
3. O agregador conhece só os `.sh` **nossos**. O ratchet é `.py` do upstream: executado, nunca editado.

A parte que roda em Linux — o self-test — está no `local-gates.yml` (ML-1A), com guarda sobre o
denominador e com os comandos do GitHub desligados enquanto ele roda: os casos sintéticos imprimem
`::error::` de propósito, e sem isso um job verde aparecia com 10 erros.

🔴 **Dois pontos cegos, declarados:**

- **Os 8 mascarados.** O nosso `.gitattributes` faz 8 nomes da lista passarem no nosso CI; o ratchet os
  reporta como "sumidos" e não vê regressão neles. Causa e decisão estão na
  `REQ-2026-09-11-o-gitattributes-do-fork-mascara-um-defeito-de-produto-que-o-upstream-mantem-exposto-de-proposito`.
- **Sumida é aviso, não reprovação** — decisão escrita do mantenedor: consertar um teste não pode
  quebrar o CI. Aposentar um nome exige `removal_note` no `.github/windows-known-failures.json`, que é
  compartilhado: não se edita aqui.

## Gate de predicados de plataforma (`scripts/check-platform-predicates.sh`)

O `scripts/testdata/platform-predicates.tsv` deixou de ser tabela decorativa: o gate executa cada
caso contra o predicado **real** do runtime. 20 casos, 5 famílias.

```bash
bash scripts/check-platform-predicates.sh
```

**Roda no Windows, local, e NÃO entra no CI — decisão medida, não preferência.** 14 das 20 linhas
divergem entre `esperado` e `nativo_windows`, ou seja, só dizem algo no Windows; em `ubuntu-latest`
o gate consultaria a coluna `esperado` e passaria descrevendo o que já se sabe. Pior: a linha do
`execbit` reprovaria lá, porque `sem-bit` é fato de NTFS e não expectativa cross-SO.

**Três guardas, cada uma por um modo de falha medido:**

1. **Integridade do vetor** — cada linha declara o comprimento em bytes do campo `caso`, e o gate
   confere **antes** de comparar qualquer predicado. Existe porque em 2026-09-08 um heredoc comeu
   metade das barras invertidas de uma sonda e quase inverteu a conclusão. Num corpus de caminhos,
   barra perdida vira verde falso.
2. **Despacho por família** — a coluna `caso` não é homogênea: em `anchored` é string de entrada,
   nas outras é nome de cenário. Leitor uniforme daria verde por coincidência de tipo.
3. **Vacuidade** — casos executados têm de igualar as linhas não-comentário.

Família que não pode ser exercitada **declara-se N/A com a razão** (hoje: `bash`, porque
`System32ash.exe` não existe sem WSL), nunca em silêncio.

Como o `upstream-sync.sh`, ele é **nosso** e não tem alvo no `Makefile`, pelo mesmo motivo abaixo.

**Não há alvo no `Makefile` de propósito.** O `Makefile` é arquivo compartilhado com o upstream, e
modificá-lo criaria divergência de produto que todo merge futuro pagaria. Acrescentar arquivo novo
em `scripts/` não cria divergência — é o mesmo precedente dos outros três scripts só nossos.

**Governança local** (o `trackfw.yaml` daqui sobrescreve dois defaults do produto):

- `req_dir: docs/requisições` — não `docs/req`. O nome em português é histórico.
- `roadmap_namespacing: by_agent`, agentes `[apolo, artemis, claude]`, então os artefatos ficam em
  `docs/requisições/<agente>/` e `docs/roadmaps/<agente>/{backlog,wip,done}/`.

A governança do upstream **não** é importada: as 52 ADRs, 140 REQs e 142 roadmaps dele cairiam
dentro de `docs/adr/` e `docs/roadmaps/`, que é onde vive a governança daqui.

**Divergência local de produto: NENHUMA.** Medido em 2026-09-05:

```bash
git diff --name-only main upstream/main -- internal npm/src pypi/trackfw cmd .github Makefile
# (vazio)
```

Os únicos arquivos só nossos são adições que o upstream não tem — `scripts/check-slug-inventory.sh`,
`scripts/check-subcommand-parity.sh`, `scripts/check-upstream-content.sh`, `scripts/upstream-sync.sh`
e `scripts/check-upstream-sync-falsify.sh`. Adição não é divergência: nenhum arquivo compartilhado
difere.

> **Correção de 2026-09-05.** Esta seção afirmava que `_force_utf8_output` em `pypi/trackfw/cli.py`
> era divergência local deliberada. **Não é mais** — o upstream absorveu (2 ocorrências em
> `upstream/main:pypi/trackfw/cli.py`). A `REQ-2026-08-16-cli-python-utf8-windows` continua válida
> como registro do defeito e da correção; o que caducou foi a afirmação de que ela só existe aqui.
> Documentação que descreve divergência inexistente faz o próximo merge procurar conflito onde não
> há.
