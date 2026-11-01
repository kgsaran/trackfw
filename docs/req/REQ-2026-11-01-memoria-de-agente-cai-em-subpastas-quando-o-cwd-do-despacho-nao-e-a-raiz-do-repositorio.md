---
status: Open
date: 2026-11-01
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md"
---

# REQ: Memória de agente cai em subpastas quando o cwd do despacho não é a raiz do repositório

> Date: 2026-11-01 | Status: Open

Issue: #646

## Motivation

Os 12 agentes gerados (`internal/integrations/assets/agents/*.md`) declaram `memory: project`. No
consumidor CMDB havia 30 diretórios `.claude/agent-memory/` fora da raiz, 9 deles com arquivos.
A memória gravada ali não volta a ser lida, e entra no commit quando o `.gitignore` não a cobre.

**Medido pelo arquiteto em 2026-11-01**, no trackfw, com Claude Code:

1. Com a sessão principal em `scripts/` e nenhum `scripts/.claude` existente, um despacho do
   `artemis-tf` com ordem de não escrever nada criou `scripts/.claude/agent-memory/artemis-tf/` (vazio)
   no ato do despacho. O caminho de memória injetado no prompt do agente foi
   `.../trackfw/scripts/.claude/agent-memory/artemis-tf/`, e o `pwd` dele era `.../trackfw/scripts`.
2. Na mesma mudança de cwd, a própria sessão principal ganhou `scripts/.claude/agent-memory/zeus-tf/`
   (vazio): o caminho de memória da sessão também segue o cwd.
3. Neste repositório já existiam três diretórios vazios do mesmo tipo: `pypi/.claude/agent-memory`
   (2026-09-02), `docs/roadmaps/wip/.claude/agent-memory` (2026-10-05) e
   `.claude/agent-memory/zeus-tf/.claude/agent-memory` (2026-10-22). A memória da raiz é versionada
   (143 arquivos em `git ls-files`).

**Consequência para a causa:** o issue atribui o desvio ao `cd` que o agente faz para buildar. A
medição mostra outro mecanismo, anterior a qualquer comando do agente: o harness resolve o diretório
de memória pelo **cwd de quem despacha** (ou da própria sessão) e o cria vazio na hora. O agente só
grava ali porque segue o caminho que recebeu. A resolução é do Claude Code, não do trackfw; o trackfw
mitiga no que gera.

## Acceptance Criteria

- [x] **AC1 — prompt dos agentes gerados.** Os agentes gerados para o Claude Code (os 12 assets, e o
      que o gerador acrescenta a eles) instruem: a memória do agente é sempre
      `<raiz do repositório>/.claude/agent-memory/<nome-do-agente>/`, com a raiz dada por
      `git rev-parse --show-toplevel`; se o caminho injetado pelo harness não estiver sob essa raiz, o
      agente grava na raiz e não no caminho injetado; nunca grava em `.claude/` de subpasta.
      **Lado de leitura** (P-2 da Wave 0, `docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md`): o harness carrega o `MEMORY.md` do caminho
      injetado; quando esse caminho não está sob a raiz, o agente lê explicitamente
      `<raiz>/.claude/agent-memory/<nome-do-agente>/MEMORY.md` no início. O texto só chega ao destino
      Claude Code (único que interpreta `memory:`), inserido pelo renderizador, não no corpo do asset.
- [x] **AC2 — prompt do orquestrador.** O agente arquiteto gerado instrui a despachar subagentes com o
      cwd na raiz do repositório (comandos em subpasta por caminho absoluto ou em subshell
      `(cd <subpasta> && ...)`), porque o cwd do despacho decide onde o harness cria a memória.
- [x] **AC3 — `.gitignore` do consumidor.** `trackfw init` e `trackfw update` garantem uma regra que
      ignora `.claude/agent-memory/` em qualquer subpasta sem ignorar a da raiz (padrão
      `*/**/.claude/agent-memory/`, medido na Wave 0 e pelo arquiteto). Medido com
      `git check-ignore -v` nos quatro casos: subpasta comum, subpasta profunda, aninhada dentro da
      própria memória (`.claude/agent-memory/<agente>/.claude/agent-memory/x`) → ignorados; arquivo da
      memória da raiz → não ignorado. Idempotente: a segunda execução não duplica a regra.
- [x] **AC4 — diagnóstico.** `trackfw doctor` lista, como aviso, os diretórios `.claude/agent-memory/`
      fora da raiz, dizendo quantos arquivos cada um tem (vazio é resíduo do harness; com arquivos é
      memória perdida). Sem diretório fora da raiz, nenhuma linha. O `doctor` não apaga nada.
- [x] **AC5 — este repositório.** O `.gitignore` do trackfw recebe a mesma regra do AC3 e os três
      diretórios vazios medidos são removidos.
- [x] **AC7** (red team, `docs/seguranca/2026-11-01-redteam-memoria-de-agente-em-subpasta.md`; defeito conhecido se corrige neste PR) — (a) negação `!` do
      consumidor para `.claude/agent-memory/` impede o append (A-1); (b) o remédio do `doctor` usa caminho
      absoluto entre aspas (A-2); (c) conteúdo ilegível num diretório fora da raiz nunca é declarado vazio
      nem recebe sugestão de apagar (A-3); (d) `## Dispatch cwd` diz que o comportamento é do Claude Code
      (A-4).
- [ ] **AC6** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** altera o Claude Code nem tenta mudar o caminho que ele injeta; o trackfw só mitiga no que gera.
- **Não** apaga nem move diretórios de memória do consumidor automaticamente; o `doctor` só aponta.
- **Não** toca os destinos sem `memory:` (OpenCode e os demais que descartam o campo).
- **Não** remove `memory: project` do frontmatter.

## Linked ADR
<!-- none -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md`
