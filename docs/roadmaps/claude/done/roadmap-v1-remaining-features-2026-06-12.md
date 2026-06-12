# Roadmap: v1.0.0 — 4 Itens Restantes

> Criado em: 2026-06-12 | Status: 🔄 WIP

## Contexto

Antes de taggear a v1.0.0, implementar os 4 itens listados no checklist de pré-release como "roadmap v0.2+", agora promovidos para o release final. Todos os recursos estão na branch `feat/v1-remaining-features`.

## Wave 1 — Itens independentes (3 em paralelo)

> Dependências: nenhuma — ML-1A, ML-1B e ML-1C podem rodar em paralelo

### ML-1A — `trackfw roadmap show <name>`

**Status:** ⬜ Pendente  
**Arquivo afetado:** `internal/commands/roadmap.go`, `internal/generators/roadmap.go`  
**Ações:**
- Adicionar subcomando `roadmap show <name>` em `newRoadmapCmd()`
- Implementar `ShowRoadmap(name string) error` em `internal/generators/roadmap.go`
- Busca por match parcial no nome do arquivo em todos os estados (`docs/roadmaps/*/ROADMAP-*<name>*.md`)
- Se zero matches: erro `"no roadmap found matching %q"`
- Se mais de um match: listar candidatos e retornar erro pedindo nome mais específico
- Renderização no terminal:
  - Linha de cabeçalho: `── <basename> ── [<STATE>] ──────────`
  - Conteúdo do arquivo impresso diretamente (markdown é legível no terminal)
  - Ao final: `Location: <path>`

**Critérios de aceite:**
- [ ] `trackfw roadmap show auth` encontra `ROADMAP-2026-06-12-auth-service.md`
- [ ] Exibe estado (pasta) + conteúdo
- [ ] Erro claro se não encontrar
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde

---

### ML-1B — Detecção de WIP stale

**Status:** ⬜ Pendente  
**Arquivos afetados:** `internal/validator/validator.go`, `internal/commands/status.go`  
**Ações:**
- Adicionar constante `staleWIPDays = 7` em `validator.go`
- Implementar `validateStaleWIP() ([]string, error)` que:
  - Lista arquivos em `docs/roadmaps/wip/`
  - Para cada arquivo, obtém `os.Stat(path).ModTime()`
  - Se `time.Since(modTime) > staleWIPDays * 24h` → warning `"roadmap/wip/<name> has been in WIP for N days (last modified YYYY-MM-DD)"`
- Integrar como warning (não violação) em `Validate()`
- Em `GetStatus()` em `validator.go`: adicionar seção `⚠  Stale WIP (N)` após a seção `❌ Blocked` quando houver stale — omitir quando vazia

**Critérios de aceite:**
- [ ] `trackfw validate` emite warning para roadmap stale
- [ ] `trackfw status` mostra seção stale quando aplicável
- [ ] Threshold de 7 dias via constante (fácil de alterar)
- [ ] Nenhum warning emitido se todos os WIPs têm menos de 7 dias
- [ ] `go build ./...` e `go test ./...` verdes

---

### ML-1C — `trackfw log` (histórico de transições de estado)

**Status:** ⬜ Pendente  
**Arquivos afetados:** `internal/generators/roadmap.go`, `internal/commands/root.go`, novo `internal/commands/log.go`  
**Ações:**
- Definir caminho do log: `docs/roadmaps/.trackfw-log` (arquivo de texto, uma linha por evento)
- Formato de cada linha: `YYYY-MM-DD HH:MM  <basename>  <from_state> → <to_state>`
- Em `MoveRoadmap()` em `internal/generators/roadmap.go`, ao final do move bem-sucedido, chamar `appendTransitionLog(basename, fromState, toState string)`
- `appendTransitionLog` abre o arquivo em modo append (cria se não existir), escreve a linha
- Novo arquivo `internal/commands/log.go`:
  - Comando `trackfw log [--tail N]` (default tail=20)
  - Lê `docs/roadmaps/.trackfw-log`, imprime as últimas N linhas no formato:
    ```
    ── trackfw log ─────────────────────────
    2026-06-12 14:30  roadmap-auth.md  backlog → wip
    2026-06-12 15:10  roadmap-auth.md  wip → done
    ```
  - Se arquivo não existe: `"No transitions recorded yet."`
- Registrar o comando em `root.go`

**Critérios de aceite:**
- [ ] `trackfw roadmap move <name> wip` registra linha no log
- [ ] `trackfw log` exibe as últimas 20 transições
- [ ] `trackfw log --tail 5` exibe as últimas 5
- [ ] Arquivo `.trackfw-log` criado automaticamente na primeira transição
- [ ] `go build ./...` e `go test ./...` verdes

---

## Wave 2 — Plugin system (depende de Wave 1 completa)

> Dependências: Wave 1 completa (evitar conflitos em `root.go`)

### ML-2A — `trackfw plugins list/add/remove`

**Status:** ⬜ Pendente  
**Arquivos afetados:** novo `internal/commands/plugins.go`, novo `internal/plugins/plugins.go`, `internal/commands/root.go`  
**Ações:**

**Estrutura de plugins:**
- Diretório: `~/.trackfw/plugins/`
- Um plugin = um executável ou script em `~/.trackfw/plugins/<name>`
- trackfw passa argumentos restantes: `trackfw <plugin-name> [args...]` chama `~/.trackfw/plugins/<name> [args...]`

**`trackfw plugins list`:**
- Lista arquivos em `~/.trackfw/plugins/`
- Para cada arquivo executável, imprime: `  <name>   <path>`
- Se vazio: `"No plugins installed. Add one with: trackfw plugins add <github-user/repo>"`

**`trackfw plugins add <user/repo[@tag]>`:**
- Resolve URL de download: `https://github.com/<user/repo>/releases/latest/download/trackfw-plugin-<name>-<os>-<arch>`
  - `<name>` = parte após `/` do repo (ex: `kgsaran/trackfw-plugin-ai` → `ai`)
  - `<os>` = `runtime.GOOS`, `<arch>` = `runtime.GOARCH`
- Download via `net/http` para `~/.trackfw/plugins/<name>`
- `os.Chmod` para tornar executável (0755)
- Mensagem: `✓ plugin <name> installed`

**`trackfw plugins remove <name>`:**
- Remove `~/.trackfw/plugins/<name>`
- Mensagem: `✓ plugin <name> removed`

**Dispatch automático (root.go):**
- Em `Execute()`, se o subcomando não for reconhecido pelo cobra, verificar se existe `~/.trackfw/plugins/<subcommand>`
- Se sim, executa via `os/exec` passando os args restantes, conectando stdout/stderr
- Se não: comportamento padrão do cobra (erro "unknown command")

**`internal/plugins/plugins.go`:**
- `Dir() string` — retorna `~/.trackfw/plugins/` (usando `os.UserHomeDir`)
- `List() ([]string, error)` — lista executáveis no diretório
- `Install(repo, tag string) error` — download + chmod
- `Remove(name string) error` — delete

**Critérios de aceite:**
- [ ] `trackfw plugins list` exibe plugins instalados (ou mensagem de vazio)
- [ ] `trackfw plugins add kgsaran/trackfw-plugin-demo` faz download (pode ser mockado no teste)
- [ ] `trackfw plugins remove demo` remove o arquivo
- [ ] Plugin instalado é executável via `trackfw <name>`
- [ ] `go build ./...` e `go test ./...` verdes

---

## Protocolo de conclusão geral

```
1. Build:  go build ./...
2. Testes: go test ./...
3. Lint:   go vet ./...
4. Commit: git commit -m "feat(<scope>): <descrição>"
5. Push:   git push origin feat/v1-remaining-features
6. Atualizar roadmap → marcar ML como ✅ Concluído
```
