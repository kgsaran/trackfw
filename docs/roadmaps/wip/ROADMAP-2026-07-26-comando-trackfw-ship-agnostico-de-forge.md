---
status: wip
date: 2026-07-26
req: "docs/req/REQ-2026-07-26-comando-trackfw-ship-agnostico-de-forge.md"
squad: ""
---

# Roadmap: comando trackfw ship agnostico de forge

> Created: 2026-07-26 | Status: wip

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
**Status:** pending
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
**Status:** pending
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
**Status:** pending
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

---

## Wave 3 — Integração e captura da escolha (2 MLs em paralelo)
> Dependências: **barrier** — Wave 2 concluída. Arquivos disjuntos: `commands/ship` × `generators/`+`discover/`.

### ML-3A — Abertura de PR/MR no `ship`
**Status:** pending
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
**Status:** pending
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
**Status:** pending
**Files affected:** testes nos 3 CLIs, `docs/cli-parity.md`, `README.md`, `site/`

**Actions:**
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

## Acceptance Criteria

- [ ] Todas as waves concluídas
- [ ] `trackfw ship` idêntico nos 3 CLIs
- [ ] Todos os critérios de aceite da REQ atendidos
- [ ] Escopo negativo respeitado (sem merge automático, sem `--force`, sem `git add .`, sem forge além das 4)
- [ ] `make quality` verde e `trackfw validate` sem violações
