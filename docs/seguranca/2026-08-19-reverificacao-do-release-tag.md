---
status: reviewed
date: 2026-08-19
reviewer: "hades-tf"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
adr: "docs/adr/ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
req: "docs/req/REQ-2026-08-19-ship-nao-cobre-push-forcado-nem-tag-e-o-guard-bloqueia-o-caminho-bruto.md"
supersedes: "docs/seguranca/2026-08-19-revisao-do-push-forcado-e-do-release-tag.md"
verdict: "LEVANTADO COM RESSALVAS — o achado B.2 (ML-4A, commit-alvo) não reproduz mais; os itens 1-2 de 'Dano real' do ML-4A (versão sequestrada, texto da tag forjado) seguem intactos, sem destino atribuído em nenhum ML"
---

# Reverificação de segurança: `release tag` após ML-4B/ML-4C

## Veredito

**LEVANTADO COM RESSALVAS.** O critério de levantamento que o handoff declara — "tag apontando
para commit que não é o tip no forge" — está fechado, medido, na forma mais forte do achado B.2.
Mas o bloqueio original citava dois danos concretos ("Dano real", itens 1 e 2 do ML-4A) que **não
dependiam** do mecanismo de commit-alvo e **continuam intactos** — ver seção "Ressalvas" abaixo.
Não são achado novo (já estavam descritos no ML-4A, seção B.1); a ressalva é registrar que nenhum
ML desta série os endereçou, e que o texto do ADR/Emenda 1 não os menciona.

Reproduzi, com fixture nova e independente da do gate (`git init --bare`
descartável, `bin/trackfw` recompilado nesta branch via `make build`, stub de `gh` que nunca toca
rede), o exato exploit que sustentou o bloqueio no ML-4A: commit local forjado, nunca empurrado a
lugar nenhum, com o symref `origin/HEAD` repontado para uma branch antiga e alheia
(`old-stale-branch`), sem nenhum push do atacante. Contra o binário atual, o comando publica a tag
apontando para o sha real de `origin/main` (`cc90355f...`), reportado pelo `gh` stub — não para o
commit local forjado (`2b934f12...`), nem para `old-stale-branch` (`b16cfb80...`), para o qual o
symref foi deliberadamente desviado. O desvio do symref foi neutralizado, não usado.

Os 3 CLIs fecham igual (código lido, não só o Go) e o gate (`check-release-tag-parity.sh`, 18/18
cenários, incluindo os 4 novos de seleção adversarial do commit-alvo) passa limpo contra o binário
desta branch.

---

## O que mudou e o que medi

### 1. O achado B.2 do ML-4A não reproduz mais

Medido diretamente, não inferido do gate. Fixture em
`/private/tmp/.../scratchpad/reverify/` (script `setup.sh` + `fixinit.sh`, `HOME`/`GIT_CONFIG_GLOBAL`
isolados, remoto `bare` local, nunca o repositório real do projeto):

```
origin/main real:              cc90355fe1218d1557eddef73aa1aa9f6bd3bebd
origin/old-stale-branch:       b16cfb803752334554e4b9fc68a56355a35edcd1  (pré-existente, alheia)
attacker HEAD (local, forjado): 2b934f12e5df2ef76508f50c5a4074d1688ba9df  (nunca em nenhum ref remoto)

$ git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/old-stale-branch
$ ./bin/trackfw release tag 4.4.4
Tag published: v4.4.4
  tag object: faketagobjectsha0000000000000000000000
  commit:     cc90355fe1218d1557eddef73aa1aa9f6bd3bebd

payload publicado em git/tags (object): cc90355fe1218d1557eddef73aa1aa9f6bd3bebd
```

`object` no payload publicado é o sha real de `origin/main`, não o commit local forjado, nem
`old-stale-branch`. O symref repontado foi completamente ignorado — o nome da branch veio do `gh`
stub (`default_branch: "main"`), incondicionalmente, exatamente como a Emenda 1 declara.

Isto fecha a forma mais forte do achado B.2 (a variante "sem push", que não dependia de contornar o
guard) e, por extensão arquitetural, a variante mais fraca (symref apontado para uma branch própria
empurrada sem PR) — nas duas o commit-alvo não é mais lido de nenhum ref local, é lido do `gh api
repos/{owner}/{repo}/commits/{branch}`.

### 2. A remoção da comparação de nomes não abriu janela nova

A pré-condição 2 antiga recusava *apenas quando* existia `refs/heads/<base>` local com valor
diferente de `origin/<base>` — ou seja, cobria só o subconjunto de ataques em que o atacante tinha
uma branch local com o mesmo nome do symref forjado. A versão atual não compara mais nome local
contra nome do forge, mas **também não precisa mais**: o nome vem de
`GET repos/{owner}/{repo}` → `.default_branch`, e o commit vem de
`GET repos/{owner}/{repo}/commits/{branch}` → `.sha` — os dois direto do forge, nunca do symref
local. A única forma de influenciar esses dois valores é controlar a resposta real da API do
GitHub (comprometer a credencial `gh` ou fazer MITM na chamada) — fora do modelo de ameaça desta
REQ (agente com escrita local no worktree, sem controle sobre o forge), e uma classe estritamente
mais difícil do que manipular um symref local com `git symbolic-ref`. Não é uma regressão: é
estritamente mais forte do que a checagem que ela substituiu, porque a checagem antiga nunca
validava nome nenhum contra o forge — só local contra local.

A referência local (`forgeLocalSHA`, resolvida contra `origin/<nome-do-forge>`) segue sendo
comparada, mas só como cross-check não-fatal: ausência dela não bloqueia (Cenário 14 do gate,
`forge-local-ref-absent-success`, confirma isso com um sha sintético inexistente no clone,
discriminando proveniência e não só valor) e divergência dela bloqueia (Cenários 12 e 13). Não achei
caminho para o atacante forçar esse cross-check a mascarar o valor do forge — o valor publicado é
sempre `commitObj.SHA`, nunca `forgeLocalSHA`.

Há uma terceira forma de influenciar o que "o forge responde", além de comprometer a credencial
`gh` ou fazer MITM na chamada real: um `gh` forjado, sombreando o binário real mais cedo no `PATH`
(`execForgeAPI`/`deps.execForgeAPI` resolvem por nome, não por caminho absoluto). Isso não muda a
conclusão — um atacante capaz de sombrear `gh` já publica tags diretamente via API, sem precisar do
`trackfw` nenhum, e o ML-4A já concede ao atacante a credencial `gh` como pré-condição — mas fica
dito por completude da enumeração.

**Fechando a própria reclamação do ML-4A sobre o gate:** o ML-4A dizia "o P4 desta propriedade
específica não existe ainda" e que, sem um cenário adversarial, "a regressão pode voltar
silenciosamente". O Cenário 11 (`forge-symref-repoint-neutralized`) é esse P4: se `objectSHA`
fosse revertido para `rev-parse origin/<base>` (a fonte antiga), o symref repontado para
`chore/other` faria `base` resolver para `"chore/other"`, e a asserção do cenário
(`commit:     $commit_sha_s11`, o sha de `main`) falharia — o gate pegaria a regressão. Não
reexecutei essa reversão (seria sabotar código de produto, fora do meu escopo); a leitura da
asserção contra o mecanismo do bug antigo é suficiente para fechar a reclamação original.

### 3. Os 3 CLIs fecham igual

Lidos diretamente (não só o Go, como na revisão anterior fiz por inferência de espelho
intencional): `internal/commands/release.go:365-410`, `npm/src/release/runner.js:311-366`,
`pypi/trackfw/release/runner.py:400-466`. A estrutura é idêntica nos três — `default_branch` do
forge incondicional, `commits/{branch}` como fonte do sha, `forgeLocalSHA` como cross-check
não-fatal, `objectSHA`/`object_sha` sempre igual a `commitObj.sha`/`forge_sha`, nunca a uma
variável derivada de ref local. O `defaultBaseBranch`/`_default_base_branch` também mudou nos três
de forma idêntica (ver item 5).

Rodei o gate completo (`bash scripts/check-release-tag-parity.sh`) contra o binário recompilado
desta branch — **18/18 OK**, incluindo os 4 cenários novos que exercitam Go, Node e Python em
sequência para cada um: `forge-symref-repoint-neutralized`, `forge-commit-diverges-update-ref`,
`forge-commit-diverges-narrowed-fetch`, `forge-local-ref-absent-success`.

### 4. `update-ref`/`worktree remove --force`/`rm -f` no guard

Lido em `scripts/trackfw-git-branch-guard.sh:479-490` (match) e `545-553` (mensagem): os três
entraram como blocos **incondicionais** — sem exceção de token, ao contrário de `branch`/`restore`
que têm lógica condicional. `update-ref` em particular bloqueia a subcommand inteira, com a razão
declarada no próprio código citando este ADR. Confirmado **ao vivo**, contra o hook real instalado
neste repositório (não só lendo o script-fonte):

```
$ git update-ref refs/heads/hades-test-noop HEAD
trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*)
sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que
`trackfw release tag` publicaria. [...]
```

Isso cobre exatamente o mecanismo nomeado no ML-4A. Não ficou "meio caminho" — os três (`update-ref`,
`worktree remove --force`, `rm -f`) são blocos independentes e completos, mesma classe dos já
existentes (`reset --hard`, `clean -f`).

### 5. Achado menor do ML-4A (branch com `/` no nome) — corrigido, confirmado nos 3 CLIs

`defaultBaseBranch`/`_default_base_branch` trocou `LastIndexByte`/`rfind` (último `/`) por
`strings.HasPrefix` + corte de prefixo fixo (`refs/remotes/origin/`) — preserva `/` internos do
nome da branch corretamente (`release/7.2` não vira mais `7.2`). Confirmado idêntico em
`ship.go:603-617`, `npm/src/release/runner.js:434-440` e
`pypi/trackfw/release/runner.py:248-262`, todos com a mesma constante de prefixo e o mesmo
comentário cruzado entre os três.

---

## Ressalvas — o que o bloqueio cobria e não fechou

O ML-4A listava três consequências em "Dano real" (seção B.2). O ML-4B fechou o mecanismo — o
commit-alvo — mas dois dos três danos **não dependiam desse mecanismo** e nenhum ML desta série,
nem a Emenda 1 do ADR, os menciona. Confirmado por leitura do código atual, não é achado novo:

1. **Sequestro de número de versão — intacto.** Precondição 3 (`release.go:302-314`) só compara os
   5 valores extraídos dos arquivos locais (`internal/version/version.go`, `npm/package.json`,
   `pypi/pyproject.toml`, os 2 fallbacks de `pypi/trackfw/__init__.py`) contra a versão pedida na
   CLI — todos via `deps.readFile`, todos locais, sem comparação com nenhum estado do forge. Um
   atacante que edite os 4 arquivos localmente para `vX.Y.Z` e rode `release tag X.Y.Z` continua
   conseguindo publicar essa tag primeiro, bloqueando a release legítima até alguém apagá-la
   manualmente — como medi na minha própria fixture (seção "O que medi") usando `4.4.4`, versão que
   nunca existiu em nenhum commit de `main`.
2. **Texto da tag arbitrário, sob a identidade real — intacto, e mais crível depois da correção.**
   Precondição 4 (`release.go:317-329`) lê `CHANGELOG.md` local sem qualquer comparação com o
   conteúdo real de `origin/<default_branch>`, e `tagMessage` vira o `message` do payload
   (`release.go:439`), publicado sob `tagger` = `git config user.name/email` local
   (`release.go:413-419`), também não verificado contra nada externo. Medido nesta rodada: o
   payload publicado na minha reprodução trazia o texto forjado ("Local-only commit, NEVER pushed
   anywhere...") associado ao sha real de `origin/main` — o próprio ML-4A já havia previsto esse
   efeito ("o commit... 'parece' legítimo a quem olha o commit, mas o texto da tag é forjado"). A
   correção do ML-4B, ao ancorar o commit no tip real, torna essa mensagem forjada **mais**
   crível, não menos: quem audita a tag vê um commit genuíno de `main` com uma mensagem que nunca
   passou por revisão nenhuma.
3. **PR em rascunho satisfazendo o `--force-with-lease`** — já classificado como "achado menor, não
   bloqueante" no ML-4A e como "declarar, não bloquear" na tabela da Emenda 1; segue no mesmo
   estado, sem mudança.

Nenhum destes dois primeiros itens depende do mecanismo que motivou o bloqueio (seleção do
commit-alvo) — por isso não reabrem o veredito de levantamento sobre esse critério específico. Mas
como o handoff original os registrou como "dano real" do mesmo comando, e nenhum artefato desta
série lhes atribuiu destino, registro aqui como débito nomeado, não corrigido: recomendo REQ
futura dedicada (comparar `CHANGELOG.md`/versão local contra o conteúdo real de
`origin/<default_branch>` antes de aceitá-los como fonte da mensagem/precondição de versão) — fora
do escopo desta reverificação, que é sobre o critério específico que o handoff pediu para
reavaliar.

## Achado novo: nenhum além das ressalvas acima

Considerei um vetor adicional não coberto pelo gate — atacante deleta a ref de tracking (variante do
Cenário 14, que eu já tinha visto no gate) *e* forja o config `remote.origin.fetch` para um branch
decoy que nem existe no remoto, esperando que a falha do `git fetch` interno degradasse para um
caminho de sucesso silencioso. Por leitura do código (`release.go:273-275`): uma falha no
`fetch origin --prune` já retorna erro fatal (`releaseTagFetchFailedFmt`) antes de qualquer outra
pré-condição rodar — não há caminho de degradação. Não medi isso à parte porque é exatamente o
comportamento que o Cenário 14 do gate já valida como pré-requisito (usa um decoy que *existe*, por
essa mesma razão declarada no comentário do cenário) — não há vetor novo aqui, só confirmação por
leitura de um caso adjacente.

Não encontrei imprecisão a corrigir no parecer do ML-4A sobre o achado B.2 — a descrição da variante
"sem push" como a mais forte, e da variante com push como mais fraca por já depender do guard,
permanece precisa à luz do código atual; o que mudou é que ambas deixaram de funcionar, pela mesma
correção (fonte do commit-alvo movida para o forge). A imprecisão que havia — tratar os itens 1-2 de
"Dano real" como resolvidos pela mesma correção — está corrigida na seção "Ressalvas" acima: eles
nunca dependeram do commit-alvo, e o ML-4B não os tocou.

---

## O que medi vs. o que inferi

**Medido, com fixture nova, binário recompilado desta branch (`make build`, nunca `make install`,
nunca o do PATH):**
- Reprodução direta do exploit B.2 (variante "sem push") contra o binário atual — falha em
  publicar o commit forjado; publica o sha real de `origin/main` via `gh` stub.
- `bash scripts/check-release-tag-parity.sh` completo — 18/18 `OK`, incluindo os 4 cenários novos.
- `git update-ref` bloqueado ao vivo pelo hook real deste repositório (não simulado).

**Inferido por leitura, não remedido nesta rodada:**
- Paridade estrutural Node/Python do trecho que resolve `default_branch`/commit-alvo (lido
  diretamente linha a linha, não recompilado/executado fora do gate — o gate já roda os 3
  runtimes por cenário, o que supre a lacuna que a revisão anterior tinha aqui).
- `update-ref`/`worktree remove --force`/`rm -f` como blocos incondicionais — lido no script-fonte
  e confirmado ao vivo apenas para `update-ref` (o mecanismo central do exploit); os outros dois não
  foram reexecutados nesta rodada por serem estruturalmente idênticos e já terem sido medidos pela
  auditoria de `apolo-tf` referenciada no ML-4A.
- Ausência de janela nova na remoção da comparação de nomes — argumento por modelo de ameaça
  (controlar a resposta real da API do GitHub está fora do escopo desta REQ), não uma tentativa de
  MITM medida.

Nenhum comando desta reverificação tocou o repositório real do projeto, publicou tag real, ou fez
push para um remoto de verdade — todo o trabalho ficou em
`/private/tmp/claude-501/.../scratchpad/reverify/`, com stub de `gh` que nunca chama a rede.
