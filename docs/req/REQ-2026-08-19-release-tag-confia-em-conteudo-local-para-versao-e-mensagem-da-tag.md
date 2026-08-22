---
status: Open
date: 2026-08-19
author: ""
adr: "docs/adr/ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-21-release-tag-ancora-versao-e-mensagem-no-forge.md"
---

# REQ: `release tag` confia em conteúdo local para a versão e para a mensagem da tag

> Date: 2026-08-19 | Status: Open (backlog, sem roadmap)

## Motivação

Levantado pela **reverificação** do `hades-tf`
(`docs/seguranca/2026-08-19-reverificacao-do-release-tag.md`), ao levantar o bloqueio do ML-4A.
Débito nomeado com honestidade pelo próprio revisor: **dois dos três danos que o parecer original
listava nunca dependiam do mecanismo que o ML-4B corrigiu**, e nenhum ML da série os endereçou.

O ML-4B ancorou o **commit-alvo** no forge. Não ancorou o resto.

### Confirmado por leitura, não pelo relatório

`internal/commands/release.go` (+ espelhos Node/Python), todos via `deps.readFile`, sem qualquer
comparação com o remoto:

| pré-condição | fonte | consequência |
|---|---|---|
| **3** — os 4 arquivos de versão (`release.go:302-314`) | **local** | sequestro do número de versão: basta editar os arquivos locais para publicar a tag que se quiser |
| **4** — seção do `CHANGELOG.md` (`release.go:317-329`) | **local** | o **texto da tag** é conteúdo local arbitrário, publicado sob a identidade real do usuário |

### 🔴 O ponto que torna isto mais grave depois do ML-4B, não menos

A observação decisiva é do revisor, e é contraintuitiva: **corrigir o commit-alvo tornou a mensagem
forjada mais crível**. Antes, uma tag suspeita podia apontar para um commit estranho — um sinal.
Agora ela aparece pendurada num commit **real e legítimo** do tip da branch padrão, com a mensagem
que o atacante escreveu, assinada pela credencial do usuário.

A correção de um vetor **aumentou a superfície de credibilidade** do outro. Não invalida o ML-4B —
invalida a ideia de que o `release tag` esteja fechado.

## Escopo

1. **Versão ancorada fora do conteúdo local editável** — ou verificada contra o forge, ou derivada
   de fonte que o agente não reescreve.
2. **Mensagem da tag verificada contra o `CHANGELOG.md` de `origin/<default_branch>`**, não contra a
   cópia local. Divergência → recusa nomeando a divergência, no padrão que o ML-4B já estabeleceu
   para o sha.
3. Paridade nos 3 CLIs, com **gate comparando as saídas reais**, e cenário P4.

## O que **não** é escopo

- Reabrir o ancoramento do commit-alvo — está fechado e reverificado.
- Impedir agente induzido. Vale o `ADR-2026-08-12`: **detecção ancorada, não prevenção**. Aqui a
  âncora existe e é barata (o `CHANGELOG` do forge), então não usá-la é omissão, não postura.

## Acceptance Criteria

- [x] AC1 — A versão publicada não pode ser determinada apenas por conteúdo local editável.
- [x] AC2 — A mensagem da tag é verificada contra o `CHANGELOG.md` de `origin/<default_branch>`.
- [x] AC3 — Divergência recusa nomeando **o quê** divergiu, no padrão do ML-4B.
- [x] AC4 — Paridade nos 3 CLIs com **gate comparando saídas reais**.
- [x] AC5 — Cenário P4 com braço de baseline e de detecção.
- [x] AC6 — Seção do `docs/cli-parity.md` atualizada, **nomeando o gate**.
- [ ] AC7 — `make quality` verde **e CI verde**. (local verde; **aguardando CI**)

## Riscos para quem executar

- **Publica em repositório público.** Prefira recusar a adivinhar. Testar contra stub e remoto bare
  local, **nunca** contra o repositório real.
- **Falso-positivo tem custo alto aqui:** o `CHANGELOG` local legitimamente está à frente do forge
  durante o PR de bump. A verificação precisa acontecer **depois** do merge, ou tolerar esse caso
  de forma declarada — pensar nisso **antes** de escrever código, ou o comando fica inutilizável no
  próprio fluxo que ele existe para servir.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- avaliar: pode caber como Emenda 2 ao ADR-2026-08-19 -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
