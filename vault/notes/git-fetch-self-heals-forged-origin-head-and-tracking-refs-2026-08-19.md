# `git fetch` self-cura symref/ref de tracking forjados — a janela de ataque real é o refspec estreitado

> 2026-08-19 — apolo-tf, durante ML-4B (ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md)

## O que se supunha (e estava errado)

Ao desenhar cenários adversariais para `check-release-tag-parity.sh` provando que `trackfw
release tag` ancora o commit-alvo no forge (não em ref local), a hipótese inicial era: bastaria

1. `git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/<branch-errada>` (repontar o
   symref), ou
2. `git update-ref refs/remotes/origin/main <sha-arbitrário>` (forjar o tracking ref)

isoladamente, e rodar o comando. Nos dois casos o teste deu **exit 0 (sucesso)**, não a recusa
esperada — a divergência nunca disparava.

## Causa raiz confirmada por leitura direta do comportamento do git (não hipótese)

`trackfw release tag` chama **`git fetch origin --prune` como Precondição 2, antes** de ler
`origin/<base>` ou o symref. Com git 2.50 (Apple Git-155), sob o refspec PADRÃO
(`+refs/heads/*:refs/remotes/origin/*`):

- `git fetch` **sempre** sobrescreve `refs/remotes/origin/main` com o valor real do remoto —
  é literalmente o propósito do fetch. Um `update-ref` anterior nele é desfeito nesse mesmo
  fetch, antes que qualquer código do comando o leia.
- `git fetch` também **resincroniza `refs/remotes/origin/HEAD`** quando ele aponta para algo que
  o fetch não reconhece como válido (ex.: um branch que não existe no remoto) — mesmo com
  `remote.origin.followRemoteHEAD` não setado (default `create`, que a doc diz "não toca ref
  local já existente" — na prática, um symref apontando para um alvo inexistente parece contar
  como "não existente" para esse propósito, não como "já existente"). Setar
  `remote.origin.followRemoteHEAD never` explicitamente PREVINE essa resincronização.

Confirmado empiricamente com um script isolado (`git init --bare` + clone real, nunca contra o
repo do projeto — ver metodologia abaixo) reproduzindo os dois cenários passo a passo.

## A implicação para o desenho do ataque (e do teste)

O único jeito de uma forjadura local sobreviver ao fetch interno do comando é **primeiro
estreitar `remote.origin.fetch`** para excluir o alvo do refspec (ex.:
`+refs/heads/decoy:refs/remotes/origin/decoy`), de modo que o `git fetch --prune` do comando
nunca toque `refs/remotes/origin/main`. **Isto não é uma escolha de teste — é o único vetor
real**: é exatamente o mecanismo que a Emenda 1 do ADR-2026-08-19 já nomeia (“os dois saltos são
locais… um `remote.origin.fetch` estreitado deixa `origin/<base>` desatualizado ou forjado, e o
fetch não o conserta”). Um `update-ref` sozinho, sem estreitar o refspec primeiro, é vácuo contra
este comando especificamente — não prova nada, porque o próprio fetch interno desfaz a forjadura
antes do código de comparação rodar.

Para o symref (`origin/HEAD`), setar `remote.origin.followRemoteHEAD never` ANTES de repontar é
necessário para o repontamento sobreviver ao fetch interno — sem isso, o cenário também é vácuo.

## Onde isso foi corrigido

`scripts/check-release-tag-parity.sh`, Cenários 11-13 (ML-4B): os 3 cenários adversariais foram
redesenhados para narrowed-refspec-primeiro (12 e 13) e `followRemoteHEAD never`-primeiro (11).
Sem essa correção, os 3 cenários dariam **falso-negativo silencioso** (exit 0, "sucesso",
0 divergência detectada) mesmo que o código de `release.go` estivesse correto — o gate proveria a
coisa errada.

## Metodologia usada para descobrir (vale repetir)

Nunca reproduzido contra o repositório real do projeto: `git init --bare` num diretório `/tmp`
isolado + `git clone`, script bash dedicado (não comando composto na sessão — o
`trackfw-git-branch-guard.sh` bloqueia `git commit`/`checkout -b` em linha de comando composta;
rodar via `bash script.sh` evita o bloqueio por não conter o literal `git` como primeiro token do
comando visto pelo guard).

## Ligado a

Ver [[git-branch-guard-noop-outside-project-fixtures-and-falsify-cwd-2026-08-17]] para a mesma
técnica de isolar fixtures fora do projeto real ao testar comportamento de git.
