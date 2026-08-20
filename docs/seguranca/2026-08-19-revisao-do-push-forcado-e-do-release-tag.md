---
status: reviewed
date: 2026-08-19
reviewer: "hades-tf"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
adr: "docs/adr/ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md"
req: "docs/req/REQ-2026-08-19-ship-nao-cobre-push-forcado-nem-tag-e-o-guard-bloqueia-o-caminho-bruto.md"
verdict: "BLOQUEAR — release tag pode ser induzido a publicar tag em commit não revisado; ship --force-with-lease aprovado"
---

# Revisão de segurança: push forçado (`ship --force-with-lease`) e `release tag` (ML-4A)

## Veredito

**BLOQUEAR `trackfw release tag`.** Medi, com fixture real (remoto bare local, sem tocar rede nem
publicar nada de verdade), que o comando pode ser induzido a publicar uma tag anotada apontando
para um commit que **nunca passou por `main`** — via um único comando git de leitura/config
(`git symbolic-ref`). Na forma mais forte que medi, isso não exige push nenhum do atacante (nem
forçado, nem normal), não exige PR, e não depende de burlar o guard de forma alguma — o commit
publicado é local, nunca chega a existir em nenhum ref remoto. Isso quebra diretamente a garantia
central que o ADR declara (`git/tags` → `object: <commit>`, sempre a partir de `origin/<default>`)
e que o REQ pergunta explicitamente ("pode ser induzido a publicar tag apontando para commit que
não é o da branch padrão?"). Não é o risco já aceito no `ADR-2026-08-12` (agente induzido rodando
comando fora do git, ou contornando o guard) — é um `git` plumbing command comum, de leitura/config,
sem sofisticação, dentro do próprio fluxo que o comando deveria fechar.

**`ship --force-with-lease` (Wave 1): APROVADO.** A amarração ao PR aberto é coerente com o que o
ADR declara, sem degradação silenciosa. Um achado menor (PR em rascunho satisfaz o portão) não
compromete a propriedade central (visibilidade/atribuição fora do repo).

**Bloqueio da classe destrutiva (Wave 3): APROVADO**, com dois riscos residuais nomeados, não
bloqueantes, fora do escopo declarado da REQ.

---

## Metodologia

Toda a evidência do achado bloqueante (seção B) foi **medida**, com um remoto `git init --bare`
local descartável e um stub de `gh` que loga cada chamada e nunca toca a rede real —
`/private/tmp/.../scratchpad/release-fixture/`. O binário usado foi recompilado com `make build`
nesta branch (`./bin/trackfw`), nunca o do `PATH`. Nenhum comando de setup de fixture foi digitado
diretamente nesta sessão — o hook do projeto bloqueia comandos git compostos pela string, então
todo o setup ficou em scripts (`fixture-release-tag.sh`, `fixture-scenario2.sh`,
`fixture-scenario3.sh` — este último é o que produz o achado que sustenta o veredito, a variante
sem push) executados via `bash <script>`, conforme a armadilha de ambiente já documentada no
handoff.

Onde digo "por leitura", é porque medir exigiria publicar de verdade (não fiz) ou porque a mesma
lógica está replicada byte-a-byte nos 3 CLIs por decisão de design documentada no próprio código
(`defaultBaseBranch`/`base` comentado explicitamente como espelho exato entre Go/Node/Python) — não
reexecutei o achado nos 3 runtimes porque a causa raiz é arquitetural, não de stack.

---

## A) `ship --force-with-lease` — amarração ao PR aberto

Não montei fixture nova para esta seção — a auditoria de `apolo-tf` já registrada no roadmap
(linhas 84-121) mediu, com remoto bare real, os quatro caminhos (sem forge CLI, forge sem PR,
forge não-verificável, forge com PR aberto) e confirmou reescrita de história só no último caso.
Revisei o código (`internal/commands/ship.go:328-364`) e concordo com o resultado: as três classes
de recusa são distintas e nenhuma cai silenciosamente em push permissivo.

**PR fechado e reaberto?** `defaultCheckPROpen` (ship.go:186-232) consulta `--state open` no
momento da chamada — um PR reaberto volta a contar como aberto. É o comportamento correto, não uma
brecha: o estado real no forge é o que importa.

**PR em rascunho?** — **achado menor, não bloqueante.** Verifiquei por leitura da documentação
oficial do `gh` (`gh pr list --help`, não a partir de busca web, que deu resposta errada): `--state`
e `--draft` são filtros independentes. `gh pr list --head <branch> --state open --json number`
(ship.go:190) **não** filtra por draft — PRs em rascunho contam como `state: open` e satisfazem o
portão. Isso baixa um pouco a barra do "caso real" que o ADR descreve (resolver conflito de PR em
revisão): um agente induzido pode abrir um PR em rascunho, sem pedir review de ninguém, e destravar
o force-push. Não invalida a propriedade central do ADR — o rascunho ainda é visível e atribuível
no forge, fora do alcance de escrita do agente — mas é uma barra mais baixa do que "PR em revisão"
sugere. Se a intenção for exigir PR pronto para review, adicionar `--draft=false` na query dos três
CLIs fecha isso sem custo.

**PR de outro repositório ou de fork?** Por leitura: `gh pr list --head <branch>` resolve o
repositório a partir do remote da working copy (contexto do `gh`), então só enxerga PRs abertos
*contra este mesmo repositório*. Não achei caminho para um PR de fork satisfazer o portão sem que o
próprio agente tenha aberto um PR contra este repo — que é o caso já aceito no ADR.

**Branch renomeada depois do PR aberto?** Por leitura + raciocínio: `checkPROpen` consulta pelo
nome da branch **atual** (`git symbolic-ref --short HEAD`, ship.go:264). Renomear localmente e
empurrar sob outro nome faz a consulta não achar PR nenhum sob o nome novo — recusa corretamente.
Não é uma evasão, é o comportamento correto.

**PR aberto pelo próprio agente segundos antes?** Confirmado no roadmap (ADR, seção "consequências
aceitas"): é o cenário que o próprio ADR já declara não prevenir — "o ganho é detecção ancorada
fora do repositório, não bloqueio". Não é achado novo; é a postura declarada.

**Degradação silenciosa sem forge?** Não. `ship.go:344-347`: `resolution.Forge == "manual" ||
!adapter.Available` retorna erro (`forceLeaseNoForgeCLIMsg`) antes de qualquer escrita — não há
caminho de código que caia para push sem `--force-with-lease` depois de pedir a flag.

---

## B) `release tag` — o achado bloqueante

### B.1 — O SHA do commit nunca é manipulável por conteúdo do working tree local

Testei primeiro a hipótese mais óbvia: forjar os 4 arquivos de versão + a seção do `CHANGELOG.md`
localmente, numa branch de feature nunca mesclada, com árvore limpa (committada), e rodar
`release tag 9.9.9`.

```
origin/main real:        Version = "1.0.0"
branch feat/whatever:    Version = "9.9.9"  (committado, árvore limpa)
$ ./bin/trackfw release tag 9.9.9
Tag published: v9.9.9
  commit: a295fea1...   ==  git rev-parse origin/main   (confirmado igual)
BODY publicado (gh api git/tags):
  {"tag":"v9.9.9",
   "message":"## [9.9.9] - 2026-08-19\n...TOTALLY FORGED entry never reviewed on main...",
   "object":"a295fea1...","type":"commit",
   "tagger":{"name":"Real User","email":"real@example.com", ...}}
```

Aqui o desenho do ADR se sustenta parcialmente: `objectSHA` é **sempre** `git rev-parse
origin/<base>` (release.go:263-267) — nunca o commit local, nunca `HEAD`. A tag aponta para o
commit real de `origin/main`. **Mas a mensagem publicada é inteiramente forjada** — o
`CHANGELOG.md` local, que a Precondição 4 lê (`deps.readFile`, sem qualquer comparação com o
conteúdo real de `origin/<base>`), vira o corpo da tag anotada, publicado sob a identidade do
tagger real (`git config user.name/email`, também lido localmente, não verificado). O dano aqui já
é real — texto arbitrário, atribuído ao usuário real, permanente num repositório público — mas é
apenas o primeiro degrau.

### B.2 — O achado que bloqueia: o próprio commit-alvo é manipulável, sem tocar em `main` e **sem nenhum push do atacante**

A Precondição 2 (release.go:256-277) resolve `base` via `defaultBaseBranch`
(`git symbolic-ref refs/remotes/origin/HEAD`, compartilhado com `ship.go:591-602`) e só valida "SE
existir uma branch local **com esse nome**, ela bate com `origin/<base>`". **Nada valida que
`base` é de fato `main`/`master`, nem que a branch atualmente em checkout é `base`.**

`refs/remotes/origin/HEAD` é um symref **local, mutável por um comando git comum**
(`git symbolic-ref`), não recalculado a cada `git fetch --prune` (a Precondição 2 já roda um fetch
e isso não o restaura).

**Causa raiz:** a Precondição 2 só compara alguma coisa quando existe `refs/heads/<base>` local —
ou seja, quando o nome que `base` resolve coincide com uma branch que o atacante decidiu ter
localmente. Em qualquer outro caso — inclusive o caso comum de nunca ter criado localmente uma
branch com esse nome — a checagem é **pulada por inteiro**, não satisfeita trivialmente. A
demonstração abaixo explora exatamente esse caso, e não depende de push nenhum do atacante:

```
# setup: uma branch já existia em origin ANTES do clone do "atacante" — nada que ele
# precise criar ou empurrar; é só uma branch antiga qualquer do repositório.
$ (num repo separado) git checkout -b old-stale-branch && ... && git push origin old-stale-branch
old-stale-branch sha (pré-existente em origin): f502df0f...

# clone novo do "atacante" — o fetch do clone já traz origin/old-stale-branch de graça
$ git clone <remote>
$ git rev-parse -q --verify refs/heads/old-stale-branch
  → falha: não existe branch local com esse nome (esperado)

$ git checkout -b work                       # branch local nova
$ echo 'Version = "4.4.4"' > internal/version/version.go   # + CHANGELOG/npm/pyproject
$ git commit -am "..."                       # COMMIT SÓ LOCAL — nunca empurrado, em lugar nenhum
$ git branch -r --contains HEAD              # (vazio) — este commit não existe em nenhum ref remoto

$ git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/old-stale-branch

$ ./bin/trackfw release tag 4.4.4
Tag published: v4.4.4
  commit: f502df0f...   ==  git rev-parse origin/old-stale-branch  (uma branch alheia, antiga,
                              sem qualquer relação com o commit local forjado)

BODY publicado (gh api git/tags):
  {"tag":"v4.4.4",
   "message":"## [4.4.4] - 2026-08-19\n\n### Added\n- Local-only commit, NEVER pushed
              anywhere, NEVER touched main.\n",
   "object":"f502df0f...","type":"commit","tagger":{"name":"Real User", ...}}

$ git branch -r | grep -i work
  (vazio) — confirmado: a branch 'work' nunca existiu em origin
```

Nenhum push do atacante, em nenhum momento. O único comando "incomum" é o `git symbolic-ref` — um
comando de leitura/config padrão, que nenhuma classe do guard cobre. A pré-condição 2 nunca teve
nada para invalidar, porque `refs/heads/old-stale-branch` nunca existiu no clone do atacante.

**Variante alternativa, mais fraca, registrada por completude:** apontar o symref para uma branch
própria, recém-empurrada (`rogue`), sem PR — aqui a Precondição 2 vira tautologia (local == origin,
porque o atacante acabou de empurrar os dois iguais), e o achado passa a depender de já ter
contornado o guard do `push` (aceito pelo `ADR-2026-08-12`, não é achado novo). A variante sem push
acima não tem essa dependência — é estritamente mais forte, e é a que sustenta o veredito.

```
origin/main:  a295fea1...   |   origin/rogue: 01342c4f...
Tag published: v6.6.6  →  commit: 01342c4f...  (== origin/rogue, NÃO origin/main)
```

**Por que isso é o achado central, e não uma repetição do que o `ADR-2026-08-12` já aceitou:**

- A variante sem push **não** depende de burlar o guard de forma alguma — não há push do atacante,
  logo não há comando de escrita remota para o guard interceptar. `git symbolic-ref` não está em
  nenhuma das classes bloqueadas (`case` em `scripts/trackfw-git-branch-guard.sh:281-472` cobre
  `commit`, `push`, `checkout -b/--`, `switch -c`, `stash`, `reset --hard`, `clean -f`,
  `restore <path>`, `worktree add -b` — não `symbolic-ref`). É um comando de leitura/config normal,
  sem qualquer sinal de "destrutivo".
- Não depende de credencial de forge nem de PR — as 9 pré-condições nunca exigem nenhum dos dois
  até a publicação em si (Precondição 6), e a publicação usa `gh`, que o atacante já teria de
  qualquer forma para rodar `release tag` legitimamente.
- Contradiz uma garantia que o ADR e o REQ afirmam ter resolvido: "a tag sempre aponta para
  `origin/<default>`" e a pergunta explícita do handoff ("pode ser induzido a publicar tag
  apontando para commit que não é o da branch padrão?") tem resposta **sim**, medida, na forma mais
  forte (zero push, zero contorno de guard, um único comando git de leitura/config).

**Dano real:** uma tag anotada, pública e permanente, com nome de versão e texto de release
inteiramente controlados por conteúdo nunca revisado (nem por humano, nem por CI, nem por merge em
`main`), publicada sob a identidade do usuário real via a API do GitHub autenticada por `gh`.
Consequências concretas:
1. **Sequestro de número de versão.** Publicar `vX.Y.Z` para a próxima versão real planejada
   bloqueia a release legítima (Precondição 5 recusa "tag já existe") até alguém apagar a tag
   manualmente — atrito e confusão em um repositório público.
2. **Conteúdo de release notes arbitrário**, atribuído ao mantenedor real, sem qualquer relação
   com o que está de fato no commit apontado (que continua sendo um commit real de alguma branch
   do próprio repo, então "parece" legítimo a quem olha o commit, mas o texto da tag é forjado).
3. **Nenhum rastro de PR** — ao contrário do `ship --force-with-lease`, que amarra o force-push a
   um PR aberto e visível, `release tag` não tem esse ancoradouro: as 9 pré-condições são
   verificadas contra estado **local**, nunca contra algo que exigisse revisão alheia.

**Correção mínima que fecharia o achado**, para registrar no ML corretivo (não implementei — fora
do meu escopo):
- Resolver `base` a partir do remoto **ao vivo**, não do symref local cacheado
  (`git ls-remote --symref origin HEAD`), e/ou restringir explicitamente `base` a `"main"` ou
  `"master"` literalmente, nunca ao que o symref local diz.
- Exigir que a branch **atualmente em checkout** seja `base` (ou que `HEAD` seja idêntico a
  `origin/<base>`) — hoje a Precondição 2 só compara refs, nunca o checkout atual, então rodar o
  comando de qualquer branch (inclusive uma nunca mesclada, e mesmo sem push nenhum) nunca é
  impedido por isso.
- Mesma correção vale nos 3 CLIs, com o caminho exato de cada um: `internal/commands/ship.go:591-602`
  (`defaultBaseBranch`, também usado por `release.go:261`), `npm/src/release/runner.js:347-353`,
  `pypi/trackfw/release/runner.py:229-241` (`_default_base_branch`) — os três são
  intencionalmente espelhados byte-a-byte (comentário explícito no Node e no Python citando o Go
  como referência), então o defeito é idêntico nos três, não é uma divergência de stack.

**O gate nomeado pelo AC8 não cobre esta propriedade — o ML corretivo precisa estender o gate, não
só o código.** `scripts/check-release-tag-parity.sh` tem exatamente 10 cenários: os 9 caminhos de
recusa (árvore suja, branch local desatualizada, 5 variantes de versão divergente, changelog
ausente, tag já existe local/remoto, sem forge CLI, forge não suportado, identidade git ausente) e
1 de sucesso — nenhum deles adultera `origin/HEAD`/`symbolic-ref` ou testa seleção adversarial do
commit-alvo (confirmado por grep no script: nenhuma ocorrência de `symbolic-ref` fora dos
comentários de cabeçalho). Ou seja, mesmo depois de corrigido o código, sem um cenário novo nesse
gate a regressão pode voltar silenciosamente — o P4 desta propriedade específica não existe ainda.

**Achado secundário, no mesmo helper, encontrado ao rastrear `defaultBaseBranch`:** a extração do
nome da branch a partir do symref usa o **último** `/` (`internal/commands/ship.go:597`,
`idx := strings.LastIndexByte(out, '/')`; idêntico em Node e no `rfind("/")` de
`pypi/trackfw/release/runner.py:238`). Um repositório cujo branch padrão tenha `/` no nome (ex.:
`release/main`, convenção comum em alguns projetos) faz `base` resolver para só o último segmento
(`"main"`, no exemplo — errado, ou pior, um nome que nem existe). Medido incidentalmente: ao
apontar o symref para `refs/remotes/origin/feat/rogue` no primeiro rascunho deste achado, `base`
virou `"rogue"` (perdeu o prefixo `feat/`) e `git rev-parse origin/rogue` falhou por não existir —
o comando recusou, mas por acaso (a branch de teste tinha `/` no nome por engano meu, não por
ataque). O mesmo helper alimenta `gitCommitsSince(base, ...)` em `ship.go:488`/`buildPRBody`, então
um projeto com branch padrão barrada afeta silenciosamente também o corpo do PR aberto pelo `ship`
normal (não só o `release tag`). Não é o achado bloqueante — é uma correção de baixo custo para
incluir no mesmo ML, já que o ML vai mexer exatamente nesse helper.

**Nota sobre o fixture:** o `trackfw.yaml` com `forge: github` que usei no clone do "atacante" é
um artefato do fixture — o remoto é um `bare` local, cuja URL não contém `github.com`, então
`forge.Resolve` não o identificaria sozinho. Isso não faz parte do que o achado explora: no
repositório real, a resolução de forge normalmente vem da URL do remote `origin` (que de fato é
GitHub), sem precisar de `trackfw.yaml` nenhum. O `trackfw.yaml` só está ali para que o `gh` stub
seja alcançado nesta fixture isolada; a Precondição 2 (o que o achado quebra) independe totalmente
disso.

---

## C) Bloqueio da classe destrutiva (Wave 3) — aprovado, dois riscos residuais nomeados

Não redigi fixture nova aqui — a auditoria de `apolo-tf` no roadmap (linhas 468-483) já mediu os 18
casos (9 bloqueados, 9 livres) direto contra o hook gerado pelo binário desta branch, incluindo
`git reset --soft HEAD~1` livre (o caso que mais importa, por ser o contorno padrão do `ship`) e
`checkout <branch>`/`switch <branch>` livres. Revisei o script (`scaffold.go`, literal
`gitBranchGuardScript`, trechos em `scripts/trackfw-git-branch-guard.sh:281-408`) e concordo com o
resultado — a lógica por token (não por posição fixa) cobre `env FOO=1 git stash` e flag fora da
primeira posição, como o AC3 exige.

**Nenhum falso-positivo novo encontrado** nas formas que tentei além das 18 já testadas:
`git checkout <branch> -- <path>` é bloqueado (por conter `--`) — não é falso-positivo: essa forma
*de fato* descarta o conteúdo não commitado do caminho indicado a partir de outra branch, exatamente
a classe que a regra pretende barrar.

**Dois riscos residuais, nomeados por já estarem no escopo da pergunta do handoff, não
bloqueantes:**

1. **`git update-ref`** pode reescrever qualquer ref local diretamente (ex.:
   `git update-ref refs/heads/main <sha-antigo>`) — mesmo efeito de `reset --hard`, ou pior
   (não exige estar naquela branch, pode corromper a branch de outro agente no worktree
   compartilhado sem precisar fazer checkout nela). Não coberto por nenhum `case` do guard.
2. **`git worktree remove --force`** e **`git rm -f`** também ficam de fora — mesma observação.

Isso **não** é reabertura do que a REQ já delimitou: a REQ lista explicitamente `git rm`,
`filter-branch`, `gc --prune` como "outra classe, sem incidente medido", e `update-ref` está na
mesma família não coberta. Registro aqui porque o handoff pediu explicitamente para verificar
"evasão óbvia dentro do próprio git" — `update-ref` é a mais preocupante das três porque atinge o
objetivo específico desta REQ (proteger o worktree compartilhado) por um caminho que nenhuma linha
do roadmap cogitou. Recomendo REQ futura dedicada, não bloqueio deste ML — é tripwire, não
fronteira, e o guard já declara essa postura.

---

## O que medi vs. o que inferi

**Medido, com fixture real e binário recompilado desta branch:**
- `release tag` publicando com `objectSHA` correto quando `base` não é adulterado, mas mensagem
  forjada (seção B.1).
- `release tag` publicando apontando para um commit fora de `main`, na variante com push de uma
  branch própria (seção B.2, "variante original") e na variante **sem nenhum push do atacante**
  (seção B.2, "variante sem push") — as duas contra o mesmo binário recompilado desta branch.
- 18 casos do guard destrutivo (bloqueio + liberação) — reusado da auditoria já registrada no
  roadmap, que eu revisei linha a linha contra o script gerado por este binário.
- Ausência de cobertura do achado B.2 no gate nomeado pelo AC8: `grep -n symbolic-ref
  scripts/check-release-tag-parity.sh` não retorna nada fora dos comentários de cabeçalho.
- O caminho exato do bug de `LastIndexByte`/`rfind` nos 3 CLIs: `ship.go:597`,
  `npm/src/release/runner.js` (mesma lógica), `pypi/trackfw/release/runner.py:229-241` — os três
  lidos diretamente, não só o Go.

**Inferido por leitura, não remedido:**
- Comportamento de `ship --force-with-lease` nos 4 caminhos (seção A) — reusei a medição já
  registrada no roadmap (linhas 84-121), que bate com a leitura do código atual.
- Paridade Node/Python do achado B.2 em si (a lógica de `defaultBaseBranch`/Precondição 2, não o
  bug de split) — a causa raiz é o design compartilhado, documentado como espelho intencional nos
  3 CLIs; não recompilei/testei Node e Python separadamente para o achado B.2, mas o código lido é
  idêntico em estrutura, incluindo o uso do mesmo helper para resolver `base`.
- Draft PR satisfazendo o portão do `ship --force-with-lease` — confirmado pela documentação
  oficial do `gh pr list --help` (`-d/--draft` é filtro independente de `-s/--state`), não por uma
  publicação real de PR em rascunho.

Nenhum comando desta revisão tocou o repositório real do projeto, publicou tag real, ou fez push
para um remoto de verdade — todo o trabalho ficou em `/private/tmp/.../scratchpad/release-fixture/`,
com stub de `gh` que nunca chama a rede.
