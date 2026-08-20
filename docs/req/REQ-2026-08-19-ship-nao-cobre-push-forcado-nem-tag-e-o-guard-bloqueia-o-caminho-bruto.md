---
status: Done
date: 2026-08-19
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
---

# REQ: `ship` não cobre push forçado nem tag, e o guard bloqueia o caminho bruto

> Date: 2026-08-19 | Status: Open

## Motivação — medido duas vezes na mesma sessão, com uma hora de intervalo

Ao entregar a `7.1.0`, o caminho governado **acabou** em dois pontos distintos, e nos dois o único
recurso foi sair do trilho:

1. **Push após rebase.** O PR do bump conflitou com a `main` (apend no mesmo arquivo). Rebasear é a
   forma normal de resolver. Depois do rebase o push exige `--force-with-lease` — que o guard
   bloqueia e o `ship` não oferece. Contorno usado: republicar em branch nova e fechar o PR
   original, deixando um PR órfão.
2. **Push da tag de release.** `git push origin v7.1.0` bloqueado. O `ship` só empurra branch.
   Contorno usado: criar o objeto de tag e a ref pela API do GitHub, via `gh`.

O fato desconfortável, dito sem rodeio: **o protocolo de release documentado no `CLAUDE.md` deste
projeto é inexecutável dentro dos guardrails deste projeto.** O passo 4 manda `git push origin
v<x.y.z>`, e o guard recusa.

### O inventário completo, porque duas ocorrências não são o inventário

O `case push)` do guard (`scripts/trackfw-git-branch-guard.sh:392`) é **incondicional** — bloqueia
toda forma de `git push`. O `ship` cobre **exatamente uma**: `push [-u] origin <branch-atual>`
(`internal/commands/ship.go:595-603`, `buildPushArgs` — essa função é o inventário completo do que
está coberto). Tudo o mais é buraco:

| forma | guard | `ship` | consequência |
|---|---|---|---|
| `push origin <branch atual>` | bloqueia | ✅ cobre | ok |
| `push --force-with-lease` (pós-rebase) | bloqueia | ❌ | **medido** |
| `push origin <tag>` / `--tags` | bloqueia | ❌ | **medido** |
| `push origin --delete <branch>` | bloqueia | ❌ | sem caminho |
| `push` de branch que não é a atual | bloqueia | ❌ | sem caminho |
| `push` para remote que não é `origin` (fork) | bloqueia | ❌ | sem caminho |

Corrigir só as duas medidas devolveria a mesma classe de defeito daqui a pouco. É o padrão
"condição estreita demais" já nomeado nesta série.

### 🔴 Defeito de UX à parte, e ele custou um ciclo

O guard inspeciona a **string do comando**. Um comando composto cujo texto contenha `git push` é
bloqueado **por inteiro, antes de executar qualquer parte**. Na prática: um `cat > arquivo <<EOF ...
EOF && git tag -a ... && git push ...` não gravou o arquivo, não criou a tag, e devolveu só a
mensagem do push. O bloqueio está **correto**; o **raio de alcance não é óbvio**, e a mensagem não
diz que nada antes rodou.

## Escopo

1. **`ship` cobre push forçado** com `--force-with-lease` (nunca `--force` cru), para o caso de
   rebase — que é o desfecho normal de um conflito.
2. **Caminho governado para tag de release.** Ver a decisão pendente abaixo: provavelmente **não**
   é uma flag do `ship`.
3. **As demais formas da tabela** ganham cobertura **ou** recusa com orientação explícita — o que
   não pode continuar é a recusa **sem alternativa**.
4. **Mensagem do guard diz que o comando inteiro foi bloqueado**, para o caso composto.

### 🔴 Decisão que precisa ser tomada antes de projetar

Os dois casos **não são o mesmo defeito**:

- Push forçado é *"o push do `ship` é estreito demais"* → alargar o `ship`.
- Tag **não é operação de branch**. E o portão do `ship` é "REQ + roadmap em `wip` para
  feat/fix/refactor" — pendurar `--tag` num comando com esse portão é erro de categoria. Release é
  protocolo próprio, com regras próprias, como o `CLAUDE.md` já descreve.

**Leitura recomendada:** `trackfw ship --force-with-lease` **mais** um `trackfw release tag`
separado. A decisão fica registrada no ADR, não presumida no código.

### Implementação de referência já validada, para não ser redescoberta

A tag da `7.1.0` foi publicada por **duas chamadas** de API, e funciona com tag **anotada**:

```
POST /repos/{owner}/{repo}/git/tags   {tag, message, object: <commit>, type: "commit", tagger}
   -> devolve o sha do OBJETO de tag
POST /repos/{owner}/{repo}/git/refs   {ref: "refs/tags/v<x.y.z>", sha: <sha do objeto>}
```

Verificado: `refs/tags/v7.1.0` aponta para o objeto anotado, e este para o commit correto.

## O que **não** é escopo

- **Afrouxar o `case push)` do guard.** Ele ser incondicional é o que o torna honesto. A correção
  mora no `ship`/`release`, nunca em enfraquecer a tripwire.
- Merge de PR — segue sendo decisão humana.
- Mudar a estratégia de release ou o formato do `CHANGELOG.md`.

## 🔴 Consequência de segurança a declarar, não a esconder

Todo escape hatch adicionado ao `ship` é também um caminho para um agente induzido: em vez de
`git push --force`, ele roda `trackfw ship --force-with-lease`. Pelo `ADR-2026-08-12` **não se
pretende prevenir** isso — mas é obrigatório dizer, no ADR desta REQ, **se o push forçado via `ship`
deixa rastro auditável ancorado em lugar que o agente não reescreve**. Se a resposta for "não",
isso vai escrito como consequência aceita, do mesmo modo que o `rm trackfw.yaml` foi aceito no ADR
do no-op. Não deixar implícito.

## Acceptance Criteria

- [x] AC1 — Decisão registrada em **ADR**: `ship --force-with-lease` + `release tag` separado, ou
      um `ship` alargado. Com o motivo.
- [x] AC2 — Push pós-rebase tem caminho governado, usando **`--force-with-lease`**, nunca `--force`.
- [x] AC3 — Tag anotada de release tem caminho governado, com a mensagem íntegra no objeto de tag.
      ✅ **Refechado no ML-4B; bloqueio levantado pelo `hades-tf` no ML-4C**, que reproduziu o próprio
      exploit contra o binário atual e confirmou que a tag aponta para o tip real do forge.
      Histórico: bloqueado pela barreira do ML-4A — O commit-alvo vem de refs **locais**, não
      do forge — `symbolic-ref refs/remotes/origin/HEAD` e `rev-parse origin/<base>` são ambos
      artefatos locais e graváveis. A garantia "a tag sempre aponta para `origin/<default>`" **não
      é sustentada**. Fecha no ML-4B.
- [x] AC4 — As demais formas da tabela ou são cobertas, ou recusadas **com orientação**.
- [x] AC5 — Mensagem do guard deixa claro que o comando **inteiro** foi bloqueado.
- [x] AC6 — Paridade nos **3 CLIs**, com **gate comparando as saídas reais** — teste por stack não fecha.
- [x] AC7 — Cenário P4 para cada superfície nova, com braço de baseline e de detecção.
- [x] AC8 — Seção no `docs/cli-parity.md` **nomeando o gate** que a protege.
      ✅ Gate estendido nos ML-4B e ML-4D: 4 cenários sobre a origem do alvo (11-14).
      Histórico: era insuficiente — o gate nomeado não exercita seleção adversarial do commit-alvo, ou seja,
      não protege a própria garantia que o AC8 declara. Estender no ML-4B.
- [x] AC9 — Consequência de segurança declarada no ADR.
- [x] AC10 — `make quality` verde **e CI verde**.
      ✅ **CI verde** no PR #194, run 32318795207: `parity` (Linux, 6m14s), `go`, `node`,
      `python 3.10/3.12`, `package-smoke`, `windows-integrations-resolve`, `governance`.
      Foram **três** rodadas até fechar, e cada falha ensinou algo — está registrado no roadmap.
      Histórico: CI reprovou — `check-ship-force-parity.sh` verde no
      macOS, vermelho no Linux — o stub de `gh` não é encontrado. O gate passava localmente **pelo
      motivo errado**. Corrige no ML-6A.

## Riscos para quem executar

- **Nunca `--force` cru.** `--force-with-lease` recusa quando o remoto avançou; `--force` destrói.
- **Não testar por leitura.** Push e tag exigem fixture com remoto **de verdade** (bare local), como
  já fazem `check-branch-prune-parity.sh` e `check-doctor-parity.sh`.
- **Cuidado com o binário do `PATH`** — está desatualizado e `--version` não distingue o build.

## Linked ADR
ADR: <!-- a criar: forma do caminho governado para push forçado e tag -->

## Linked Roadmap
Roadmap: <!-- a criar -->

## Débito nomeado ao fechar (não é regressão, é escopo que nunca esteve aqui)

A reverificação do `hades-tf` levantou o bloqueio **com ressalvas**: as Pré-condições 3 e 4 do
`release tag` seguem lendo **conteúdo local** (arquivos de versão e `CHANGELOG.md`), sem âncora no
forge. Confirmei por leitura (`release.go:302-329`).

O ponto que torna isso digno de REQ própria, e é do revisor: **corrigir o commit-alvo tornou a
mensagem forjada mais crível**, porque agora ela aparece pendurada num commit real do tip da branch
padrão. A correção de um vetor ampliou a credibilidade do outro.

Aberta a `REQ-2026-08-19-release-tag-confia-em-conteudo-local-para-versao-e-mensagem-da-tag`
(backlog). **Não** é regressão desta REQ — é superfície que nunca esteve no escopo dela, e fechá-la
por dentro seria escopo inflado sem decisão de KG.
