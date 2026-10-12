---
status: Accepted
date: 2026-10-12
author: "zeus-tf"
---

# ADR: branch prune apaga a branch remota só pelo sinal do forge e condicionada ao tip avaliado

> Date: 2026-10-12 | Status: Accepted

REQ: `docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md`
Estende: `ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md`
Depende de: `ADR-2026-10-12-operacao-de-rede-do-trackfw-tem-prazo-e-o-estouro-nunca-vira-sucesso-silencioso.md`

## Context

A REQ-2026-10-10 mediu que o guard bloqueia `git push origin --delete <branch>`, mas **nenhum**
comando do trackfw apaga ref remota. A mensagem do guard aponta para `trackfw push`/`ship`, que não
fazem a operação. A saída que sobra fica fora de qualquer trilho: a interface do forge ou uma chamada
de API.

Apagar no remoto é mais perigoso que apagar no local. A branch local apagada por engano ainda existe
no remoto e no object store; a remota apagada por engano pode ser o **único** lugar do trabalho, e
pode ser de outro autor.

O ADR-2026-10-03 já define o sinal forte para o local: um PR mergeado, não fork, cujo head contém o
tip. Também já define a degradação D2 (sem forge → heurística de conteúdo) e o tratamento da resposta
truncada (A7).

## Decision

**D1 — Opt-in explícito: `--remote`.** Sem `--remote`, o comportamento é idêntico ao de hoje: nenhuma
chamada escreve no remoto. `--remote` sem `--apply` só **relata** o veredito de cada branch remota.
O padrão continua sendo dry-run.

**D2 — População: `refs/remotes/origin/*`, menos `HEAD` e a branch padrão.** Só o remoto `origin`;
outros remotos degradam com mensagem. A branch padrão vem do forge (nota do vault
`forge-anchoring-branch-name-must-be-unconditional-not-cross-checked-2026-08-19`), nunca do symref
local.

**D3 — Único sinal que autoriza apagar no remoto: o D1 do ADR-2026-10-03 aplicado ao tip remoto.**
O sinal é um PR `MERGED`, não fork (`isCrossRepository == false`), cujo commit de head contém o tip
**remoto** recém-buscado. Todo o resto mantém a branch remota, nomeando o motivo:
- PR aberto de mesmo head;
- PR mergeado com commits **depois** do head (a hipótese da REQ para as duas remotas que
  sobreviveram ao auto-delete: `keep — commits after the merged PR`);
- tip divergente; head ausente (A3);
- PR fechado sem merge;
- nenhum PR.

A heurística de conteúdo **nunca** autoriza apagar no remoto: sem sinal do forge, não apaga.

**D4 — Pré-condições que proíbem toda remoção remota na execução.** Nenhuma remota é apagada se
qualquer uma delas falhar, e o relatório nomeia a causa:
- o `git fetch origin --prune` falhou ou estourou o prazo do ADR de rede;
- o snapshot do forge está degradado (D2) ou truncado (A7);
- a branch padrão do forge não pôde ser lida.

O local continua com o comportamento atual (dado velho declarado), porque para o local esse
comportamento é seguro.

**D5 — A remoção é condicionada ao SHA avaliado (TOCTOU).** TOCTOU é a janela entre verificar e usar:
o tip remoto pode mudar entre a avaliação e a remoção. A remoção usa
`git push --force-with-lease=refs/heads/<b>:<sha avaliado> origin :refs/heads/<b>`. São dois
controles, e nenhum substitui o outro:
- o head do PR contendo o tip cobre a **avaliação**;
- o lease cobre a **remoção**: se o remoto avançou depois do fetch, o push é recusado e a branch é
  relatada como `skip — remote advanced since evaluation`.

🔴 O Wave 0 **mede** os dois braços num repositório bare: tip igual apaga; tip avançado recusa e
mantém a ref. A medição local do arquiteto foi bloqueada pelo guard (`git commit` bruto), então este
ponto ainda **não está medido**.

**D6 — Branch protegida nunca é alvo.** Se o forge informar a branch como protegida, ela fica fora da
remoção, mesmo que o servidor fosse recusar de qualquer forma. A fonte exata é decisão do Wave 0.

**D7 — O guard aponta para o caminho governado.** Para `git push --delete <ref>`,
`git push <remote> --delete <ref>` e `git push <remote> :<ref>`, a mensagem do guard nomeia
`trackfw branch prune --remote --apply`. Ela não aponta mais para `push`/`ship`. O guard continua
bloqueando a forma bruta. A mensagem vive em mais de um sítio (`internal/guard/gitbranch.go` e a
referência `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh`, no mínimo); o
Wave 0 fecha a lista.

### D8 — Ajustes do Wave 0 (`docs/seguranca/2026-10-12-wave0-prune-remoto.md`, aprovado com ajustes)

- **A1 (D7):** a mensagem nova só vale para formas que **apagam branch**: `--delete`/`-d`/`:<ref>`, em
  que `<ref>` não é `refs/tags/`. `--mirror`, `--prune`, `:refs/tags/…` e `:` sozinho mantêm a
  mensagem atual.
- **A2 (D7):** o teste de paridade do guard recebe os vetores de remoção de branch.
- **A3 (D3):** branch que é `baseRefName` de um PR **aberto** fica de fora. Apagá-la fecharia o PR
  empilhado.
- **A4 (D6):** proteção via `gh api 'repos/<o>/<r>/branches?protected=true&per_page=100' --paginate`
  (medido: responde sem token de administrador). Se a consulta falhar, nenhuma remoção remota.
- **A5 (D2):** se a URL de fetch e a de push do `origin` divergirem, nenhuma remoção remota.
- **A6 (D3), crítico:** o caminho remoto exige `decision == merged_pr`. Nunca usa `deletable()`,
  que também aceita as decisões da heurística de conteúdo.
- **A7 (D2):** a branch padrão vem do forge (`default_branch`), não do literal `"main"`
  (`branch_prune.go:19,26`; filtro `baseRefName` em `branch_prune_forge.go:293`). O defeito também
  existe no prune **local** de hoje: num repositório `master`, o comando recusa tudo. Mesma causa,
  então entra como ML desta REQ.
- **A8 (D2):** `refs/remotes/origin/HEAD` é excluído pelo refname completo, porque o short name dele
  é `origin`.
- **A9 (D5):** o teste do lease tem três braços:
  - A: tip igual apaga;
  - B: o remoto avançou e a remoção é recusada por *stale info*;
  - C: houve refetch entre a avaliação e a remoção, e o lease usa o SHA **avaliado**.
  O teste tem mutações que precisam reprovar e confere a ref com `rev-parse --verify --quiet` no
  bare (`ls-remote` dá EXIT=0 com saída vazia). O lease na forma de remoção foi medido pelo Wave 0
  (braço igual apaga; braço avançado `! [rejected] (stale info)`, ref mantida).
- **C12 (corrige a hipótese da REQ):** as duas remotas que motivaram a REQ têm commits **depois** do
  head do PR mergeado (`git merge-base --is-ancestor 2d3a7dbb d3970eff` → 1;
  `99477f44 c61300ca` → 1; conferido pelo arquiteto). Por D3, elas são `keep`, e é isso que deve
  acontecer: o commit pós-merge nunca passou por PR, e a decisão sobre ele é humana.
- **Resíduos aceitos:** `git -c alias.p=push p …` não é reconhecido pelo guard (EXIT=0). É da mesma
  classe do script que chama `git`, e o ADR-2026-08-12 já declara que contra agente com shell não há
  prevenção. Remoção por configuração (`remote.origin.push`, `remote.origin.mirror`) também fica
  como resíduo.

## Consequences

- A operação que hoje acontece fora do trilho passa a ter caminho governado, auditável e
  conservador.
- Repositório sem `gh`, ou fora do GitHub, não ganha remoção remota (degrada como D2).
- Uma branch remota de outro autor, já mergeada por PR com o tip contido, **é** apagável. É o que o
  auto-delete do GitHub faz no merge, então isso não amplia o que o forge já faria.

### Resíduos (a confirmar no Wave 0)

- **Remotos além de `origin`** e **forges além do GitHub**: fora; degradam.
- **Ref recriada no remoto entre o fetch e o lease com o mesmo SHA** (apagada e recriada no mesmo
  commit): o lease aceita, e o resultado é o mesmo commit apagado. Não há perda de trabalho.

## Alternatives Considered

- **API do forge (`DELETE /git/refs/heads/<b>`):** não tem condição atômica de SHA, então a janela
  TOCTOU fica aberta. Rejeitada.
- **Heurística de conteúdo como sinal para o remoto:** o ADR-2026-10-03 mediu que, por conteúdo,
  0 de 120 branches se distinguem. Rejeitada.
- **Liberar `git push --delete` no guard:** a REQ excluiu. O caminho governado substitui, não
  convive. Rejeitada.
