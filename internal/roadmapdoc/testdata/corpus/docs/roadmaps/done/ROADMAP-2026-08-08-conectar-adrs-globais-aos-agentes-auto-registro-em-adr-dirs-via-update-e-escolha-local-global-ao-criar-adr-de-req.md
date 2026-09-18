---
status: done
date: 2026-08-08
req: ""
squad: ""
---

# Roadmap: conectar ADRs globais aos agentes: auto-registro em adr_dirs via update e escolha local-global ao criar ADR de REQ

> Created: 2026-08-08 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-08-08-conectar-adrs-globais-aos-agentes-auto-registro-em-adr-dirs-via-update-e-escolha-local-global-ao-criar-adr-de-req.md

Fecha dois dos três elos que faltam para ADRs globais (`~/.trackfw/adr`,
REQ-2026-08-08-adr-new-com-escopo-global-...) serem efetivamente
inspecionados: (1) o projeto precisa listar esse caminho em `adr_dirs` para
`trackfw context`/a diretiva do `CLAUDE.md` terem algo para apontar; (2) o
usuário precisa poder escolher, no momento de criar um ADR via `req new`,
se ele é local ou global. O terceiro elo (subagentes especialistas
recebendo contexto de ADR ao serem delegados) é comportamento do
orquestrador, não código — fora do escopo deste roadmap.

**Descoberta durante o design, relevante para a Wave 2:** o fluxo
interativo de `req new` que detecta domínios e gera ADR drafts (probes)
existe em Go (`internal/commands/req.go`) e Node.js (`npm/src/commands/req.js`),
mas **não existe em Python** — `pypi/trackfw/commands/req.py` só pede o
título, sem probes nem geração de ADR draft. Isso é um gap de paridade
pré-existente, não introduzido por este roadmap, e implementar o fluxo de
probes do zero em Python é um corpo de trabalho maior e não relacionado —
a Wave 2 abaixo é só Go+Node; Python fica de fora, documentado.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `trackfw update` (projeto, 3 stacks) registra `~/.trackfw/adr` em
      `adr_dirs` só se o diretório existir e tiver ≥1 `ADR-*.md`
- [ ] Escrita idempotente/cirúrgica no `trackfw.yaml`, preservando conteúdo
      existente do usuário
- [ ] `req new` (Go+Node, TTY) pergunta local/global antes de gerar ADR
      drafts das probes; default local sem TTY, sem mudança de comportamento
- [ ] Testes com `$HOME`/cwd de fixture verdes
- [ ] `docs/cli-parity.md` atualizado, incluindo a nota sobre Python não ter
      o fluxo de probes (gap pré-existente, não desta REQ)
- [ ] `trackfw validate` sem violações

## Wave 1 — Auto-registro de ~/.trackfw/adr em adr_dirs via `trackfw update` (3 MLs paralelos)
> Dependencies: none

### ML-1A — Go: registrar ~/.trackfw/adr em adr_dirs (surgical, condicional)
**Status:** ✅ Concluído
**Files affected:**
- `internal/generators/update.go` — nova função
  `ensureGlobalADRDirRegistered(cwd string) error`, chamada de dentro de
  `Update(cwd string) error` (a função de `trackfw update`, escopo projeto)
  logo após a leitura de `trackfw.yaml` já validada. Mesma categoria de
  merge cirúrgico de `updateHooksSurgical` (linha ~169 do mesmo arquivo) —
  ler o `trackfw.yaml` como texto, decidir se precisa inserir, escrever de
  volta preservando tudo o mais (não usar `config.Load()`+recriar do zero,
  que perderia comentários/formatação do usuário).
- Testes novos em `internal/generators/update_test.go` (ou arquivo
  equivalente já existente para testes de `Update()`).
**Actions:**
- Resolver `home, err := os.UserHomeDir()`; `globalDir := GlobalADRDir(home)`
  (função já criada na REQ anterior, `internal/generators/scaffold.go`).
- Condição de no-op (não escrever nada): `globalDir` não existe, OU existe
  mas não tem nenhum arquivo `ADR-*.md` dentro (glob), OU o `trackfw.yaml`
  já contém uma entrada `adr_dirs` que resolve para esse mesmo caminho
  (considerar `~/.trackfw/adr` E o caminho absoluto expandido como a mesma
  entrada — não duplicar por causa de forma textual diferente).
- Se precisar inserir: se `trackfw.yaml` já tem uma chave `adr_dirs:` com
  itens de lista (`- docs/adr`), inserir `  - ~/.trackfw/adr` como novo item
  dessa lista, sem tocar nas outras linhas. Se `trackfw.yaml` NÃO tem
  `adr_dirs:` (usando o default implícito `docs/adr`), acrescentar um bloco
  novo ao final do arquivo:
  ```
  adr_dirs:
    - docs/adr
    - ~/.trackfw/adr
  ```
  (preservar o default explícito, não silenciosamente trocar por só o
  global).
- Imprimir uma linha de feedback (`✓ adr_dirs: ~/.trackfw/adr registrado`)
  só quando efetivamente escrever — silencioso no caso no-op, seguindo o
  padrão dos outros passos de `Update()`.
**Acceptance criteria:**
- [ ] `go build ./... && go vet ./... && go test ./internal/generators/...` verde
- [ ] Fixture: `trackfw.yaml` sem `adr_dirs` + `~/.trackfw/adr` com 1 ADR →
      após `update`, arquivo ganha o bloco `adr_dirs` com os dois caminhos
- [ ] Fixture: `~/.trackfw/adr` inexistente/vazio → `trackfw.yaml` inalterado
      byte a byte
- [ ] Fixture: `adr_dirs` já contém `~/.trackfw/adr` → idempotente, nenhuma
      escrita/duplicação
- [ ] Fixture: `trackfw.yaml` com comentários/outras chaves → todas
      preservadas byte a byte, só a entrada nova é adicionada
**Comandos de validação:** `go build ./... && go vet ./... && go test ./internal/generators/...`

### ML-1B — Node.js: registrar ~/.trackfw/adr em adr_dirs (surgical, condicional)
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/commands/update.js` (lógica de `update` projeto vive aqui,
  confirmado — não em `generators/`) — mesma função equivalente a ML-1A,
  chamada do fluxo principal de `update` (projeto).
- `npm/tests/update.test.js` (já existe — estender).
**Actions:** mesmas regras de ML-1A, adaptadas ao estilo Node.js já usado
neste arquivo (edição de texto linha a linha do `trackfw.yaml`, sem
recriar/reserializar via um parser YAML que perderia formatação).
**Acceptance criteria:**
- [ ] `node --test npm/tests/*.test.js` verde
- [ ] Mesmas 4 fixtures de ML-1A (sem adr_dirs, sem diretório global, já
      registrado, com comentários preservados)
**Comandos de validação:** `node --test npm/tests/*.test.js`

### ML-1C — Python: registrar ~/.trackfw/adr em adr_dirs (surgical, condicional)
**Status:** ✅ Concluído
**Files affected:**
- `pypi/trackfw/commands/update.py` (função `_run_project` e vizinhas —
  mesma família de `_update_hooks_surgical`) — mesma função equivalente,
  chamada do fluxo de `update` (projeto).
- `pypi/tests/test_update.py` (ou arquivo equivalente já existente).
**Actions:** mesmas regras de ML-1A/1B, adaptadas ao estilo Python já usado
neste arquivo (edição de texto, sem re-serializar o YAML inteiro).
**Acceptance criteria:**
- [ ] `python3 -m pytest pypi/tests/` verde
- [ ] Mesmas 4 fixtures de ML-1A
**Comandos de validação:** `python3 -m pytest pypi/tests/`

## Wave 2 — Escolha local/global ao gerar ADR draft em `req new` (2 MLs paralelos — Go+Node; Python fora de escopo, ver Context)
> Dependencies: Independente da Wave 1 (arquivos disjuntos)

### ML-2A — Go: prompt de escopo (local/global) no fluxo de ADR draft de `req new`
**Status:** ✅ Concluído
**Files affected:**
- `internal/commands/req.go` — no bloco TTY de `runReqNew` (antes do loop
  que chama `generators.NewADRDraft(answer)` para cada probe respondida),
  adicionar UM prompt `huh.NewSelect` ("Escopo dos ADRs desta REQ: local
  (padrão) / global") cuja resposta vale para TODOS os ADR drafts gerados
  nessa sessão — não repetir a pergunta por probe.
- `internal/generators/adr.go` — `NewADRDraft(slug string) (string, error)`
  precisa aceitar o diretório de destino (mesmo padrão da mudança em
  `NewADR` na REQ anterior: `NewADRDraft(slug string, adrDir string)
  (string, error)`); resolver `adrDir` no comando (`config.Load().ADRDirs[0]`
  para local, `generators.GlobalADRDir(home)` para global) antes de chamar.
- Testes em `internal/commands/req_test.go`/`internal/generators/adr_test.go`.
**Actions:**
- Sem TTY (`cbterm.IsTerminal` falso): comportamento 100% inalterado — sem
  prompt novo, `NewADRDraft` sempre resolve para local (mesmo default de
  hoje).
- Não é preciso `trackfw.yaml`/projeto para escopo global aqui também,
  mesma regra das demais features desta família.
**Acceptance criteria:**
- [ ] `go build ./... && go vet ./... && go test ./internal/commands/... ./internal/generators/...` verde
- [ ] Sem TTY: nenhuma mudança de output/comportamento vs. antes desta ML
**Comandos de validação:** `go build ./... && go vet ./... && go test ./internal/commands/... ./internal/generators/...`

### ML-2B — Node.js: prompt de escopo (local/global) no fluxo de ADR draft de `req new`
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/commands/req.js` — mesma ideia de ML-2A: um único
  `select`/`@inquirer/prompts` antes do loop de probes que chama
  `adrGenerators.newADRDraft(answer)`.
- `npm/src/generators/adr.js` — `newADRDraft(slug)` passa a aceitar
  `newADRDraft(slug, adrDir)`, mesma resolução condicional do comando.
- `npm/tests/req.test.js` (criar se não existir, ou estender).
**Actions:** mesmas regras de ML-2A (sem TTY = comportamento inalterado).
**Acceptance criteria:**
- [ ] `node --test npm/tests/*.test.js` verde
- [ ] Sem TTY (`process.stdin.isTTY` falso em teste): nenhuma mudança de
      output/comportamento vs. antes desta ML
**Comandos de validação:** `node --test npm/tests/*.test.js`

## Wave 3 — Documentação e auditoria final (1 ML, orquestrador)
> Dependencies: Wave 1 + Wave 2 completas

### ML-3A — Atualizar cli-parity.md e confirmar paridade cross-stack
**Status:** ✅ Concluído
**Files affected:**
- `docs/cli-parity.md` — documentado o auto-registro condicional em
  `adr_dirs` via `update` (nota na seção `trackfw update` vs `update
  harness`, incluindo a exceção read-only de `~/.trackfw/adr`) e o prompt de
  escopo em `req new` (Go+Node), com a nota explícita de que Python não tem
  o fluxo de probes/ADR-draft (gap pré-existente, fora desta REQ).
**Actions:**
- Build+test completo dos 3 stacks — verde.
- Confirmado manualmente com `HOME`/cwd isolados por stack: os 3 CLIs
  registram `~/.trackfw/adr` em `adr_dirs` de forma **byte-idêntica**
  (mesmo bloco YAML gerado nos 3 stacks), idempotente (rodar `update` de
  novo não duplica), e sem tocar o arquivo quando o diretório global não
  existe.
- Auditado o desvio reportado pelo agente do ML-1C (Python): `update`
  chama `_ensure_global_adr_dir_registered` tanto em `_run` (invocação
  simples) quanto em `_run_project` (`--dry-run`/`--json`/`--targets`),
  contra `apply_root` neste último para respeitar o sandbox de dry-run —
  confirmado correto e necessário para paridade real com Go/Node (que
  chamam a função incondicionalmente em toda invocação de `update`).
**Acceptance criteria:**
- [x] `go build ./... && go vet ./... && go test ./...` verde
- [x] `node --test npm/tests/*.test.js` verde (446 pass)
- [x] `python3 -m pytest pypi/tests/` verde (985 passed)
- [x] `trackfw validate` sem violações (após mover roadmap/REQ para done)
**Comandos de validação:** `go build ./... && go vet ./... && go test ./... && node --test npm/tests/*.test.js && python3 -m pytest pypi/tests/ && trackfw validate`
