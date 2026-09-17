# Parecer de Segurança — ML-0A  
## Roadmap: sync enumera REQ por caminho literal, ignora req_dir e escreve no provedor de PM  
**Data:** 2026-09-17 | **Autor:** hades-tf | **Branch:** fix/sync-enumera-req

---

## 1. Completude da enumeração

O roadmap nomeia um único sítio: `internal/sync/sync.go:43` com o literal `filepath.Glob("docs/req/*.md")`.

**Varredura independente em `internal/**` (exceto testes):**

| Arquivo | Ocorrência | Natureza |
|---|---|---|
| `internal/sync/sync.go:43` | `filepath.Glob("docs/req/*.md")` | Enumerador de REQ — o defeito |
| `internal/config/config.go:304` | `REQDir: "docs/req"` | Default da struct — legítimo |
| `internal/commands/configure.go:48,62,94` | `"docs/req"` | Valor padrão em wizard de setup — legítimo |
| `internal/commands/help.go:36` | `Default: "docs/req"` | Texto de ajuda — legítimo |
| `internal/generators/scaffold.go:38,339` | `"docs/req"` | Scaffold de diretório e template de comentário — legítimo |
| `internal/discover/discover.go:315` | `"docs/req"` em lista de candidatos | Heurística de descoberta automática — legítimo |

**Enumerador flat de REQ fora de `internal/`:**

`scripts/check-referential-integrity.sh:10` contém `for req in docs/req/*.md` — literal hardcoded em script de barreira. Fora do escopo do AC5 (que cobre apenas `internal/**`). Este sítio **não é endereçado por nenhum AC** da REQ vigente.

**Conclusão:** A lista de sítios em `internal/` está fechada. O único enumerador defeituoso em código de produto é `sync.go:43`. Existe um orphan literal em `scripts/check-referential-integrity.sh:10` fora do escopo declarado — nomeado como residual (ver seção 4).

---

## 2. Modelo de ameaça

O adversário aqui não é um atacante externo. É o **implementador apressado** que fecha os ACs como escritos sem examinar o que o fix abre, e o **arquiteto otimista** que marca a wave concluída quando os testes passam.

### 2.1 Alvo errado: vazamento de conteúdo para o provedor de PM

**Evidência:** `syncToProvider` (sync.go:43) nunca chama `config.Load()`. Com `req_dir: docs/requisições`, a glob encontra zero REQs reais. Se arquivos `.md` residuais existirem em `docs/req/` (de um layout anterior), cada um que contiver `| Status: Open` é publicado no Linear/Jira com o id injetado de volta no arquivo residual.

**`isStatusOpen` estreita a superfície:** o filtro usa `strings.Contains(line, "| Status: Open")` — apenas arquivos que contêm exatamente essa substring na primeira linha com `| Status:` qualificam. Uma linha `| Status:` sem `Open` retorna `false` imediatamente. No fallthrough (nenhuma linha com `| Status:`), retorna `false`. Portanto, arquivos sem esse campo não são publicados. Isso limita o vetor a arquivos que passam pelo filtro de status, não a qualquer `.md`.

**PM target:** verificado em `linear.go` e `jira.go`. `LinearTeamID` e `JiraProject` vêm de `config.Load().Sync.*` — chaves inteiramente separadas de `req_dir`. Uma misconfiguraçao de `req_dir` muda o **conteúdo** publicado (quais arquivos), não o **destino** (qual equipe ou projeto). O target permanece correto.

**O fix fecha este vetor?** Sim, substituindo a glob por `resolveREQFiles(cfg)` — desde que `cfg.REQDir` seja seguro (ver 2.2).

### 2.2 O vetor que o próprio fix abre (CRÍTICO)

**Evidência em `validator.go:1603`** (`ResolveREQFiles`):

```go
func ResolveREQFiles(cfg config.ProjectConfig) []string {
    reqDir := cfg.REQDir  // usado diretamente, sem verificação de contenção
    ...
    add(ListMDFiles(reqDir))        // lê reqDir diretamente
    addChild(reqDir, state)          // lê reqDir/<state>
    addChild(reqDir, agent)          // lê reqDir/<agent>
```

Não existe chamada a `isOutsideCWD` em nenhum ponto de `ResolveREQFiles`. `filepath.Clean` é chamado nos resultados individuais, mas `Clean` não reverte traversal quando a própria raiz já escapa: `filepath.Clean("../../etc")` → `../../etc`.

**Evidência em `config.go:413`:** `cfg.REQDir = v` — verbatim. `ExpandPath` não é chamado no `req_dir` no momento do load. Portanto `req_dir: ~/sensitive` fica literal (não expande), mas `req_dir: ../../team-secrets` ou `req_dir: /home/alice/docs` funcionam como rotas absolutas de leitura.

**O exploit concreto com o layout atual:** ambos os worktrees existem no mesmo host. Com `req_dir: ../trackfw/docs/req` no `trackfw.yaml` do worktree sync, o fix passaria a ler `../trackfw/docs/req/*.md` — arquivos do projeto principal, garantidamente com `| Status: Open` — e publicaria seus títulos e motivações no Linear/Jira do projeto sync. `extractTitle` não tem guarda de string não-vazia: um arquivo sem linha `# REQ: ` gera issue com título vazio mas ainda cria o ticket.

**A glob atual é acidentalmente imune a este ataque porque é literal** (`"docs/req/*.md"` ignora `cfg.REQDir` completamente). O fix correto troca esse comportamento errado por um comportamento perigoso se não adicionar contenção.

**`isOutsideCWD` não é a contenção correta como está:** usa `filepath.Rel` sem chamar `filepath.EvalSymlinks`. Um symlink em `docs/req` apontando para fora da árvore passaria na verificação lexical. O precedente no mesmo repositório: `REQ-2026-09-11-serve-api-file-valida-o-caminho-lexico-e-abre-o-fisico-symlink-em-docs-req-le-qualquer-arquivo`. Se a contenção for implementada via `isOutsideCWD`, ela deve primeiro resolver o caminho físico com `EvalSymlinks` — ou ser reimplementada. Isso é decisão do arquiteto.

**O AC1 como redigido é insuficiente:** diz "passa a resolver as REQ pelo ponto único honrando `req_dir` e `by_agent`", mas não exige que `req_dir` seja contido antes de ser usado. A fix sem contenção atende AC1 lexicalmente e abre este vetor.

### 2.3 Mensagem de erro expõe caminho (AC6)

**Evidência em `commands/sync.go:50`:** `fmt.Println("No REQs found in docs/req/")` — literal hoje. Após o fix, se a mensagem usar o caminho resolvido, um `req_dir: /home/alice/company/docs` seria impresso em logs de CI.

**`config.Load()` não chama `ExpandPath` para `req_dir`** (config.go:413-414). Portanto o valor configurado chega verbatim. Um `req_dir: ~/secret` imprime `~/secret` (seguro). Um caminho absoluto configura exatamente o que será impresso.

**Regra para AC6:** o texto da mensagem deve ecoar o valor `cfg.REQDir` verbatim — nunca passar por `filepath.Abs`, `ExpandPath` ou qualquer expansão antes de logar. Isso é safe-by-construction porque o valor já estava no `trackfw.yaml` commitado.

### 2.4 Redirecionamento do alvo Jira (pré-existente, não fechado por esta REQ)

**Evidência em `jira.go`:** `NewJiraClient()` usa `config.Load().Sync.JiraBaseURL` — lida do mesmo `trackfw.yaml` que contém `req_dir`. Um `trackfw.yaml` hostil pode apontar `jira_base_url` para um host controlado pelo atacante; o POST vai com `Authorization: Basic base64(email:token)`, entregando credenciais.

Linear não sofre este vetor: `https://api.linear.app/graphql` está hardcoded em `linear.go`. Apenas o API key é configurável.

**Este é um defeito pré-existente, independente do fix.** Não é introduzido nem fechado por esta REQ. É nomeado aqui porque a pergunta 2 perguntou se `req_dir` afeta o PM target — não afeta, mas a mesma superfície (`trackfw.yaml`) pode redirecionar o Jira target inteiramente. A REQ vigente não declara escopo sobre autenticação/alvos Jira.

### 2.5 Verificabilidade do AC4 e cache do `config.Load()`

**AC4 (nenhuma escrita real em provedor externo durante os testes):** atualmente satisfeito por arquitetura. `syncToProvider` recebe uma `create func` de stub; `NewLinearClient()`/`NewJiraClient()` nunca são chamados em `sync_test.go`. Não existe build tag `//go:build integration`, interface injetável para nil-in-tests, nem verificação de env var. O residual: um colaborador futuro adicionando teste que chame `SyncToLinear()` diretamente com token presente no ambiente criaria issue real. Verificável mecanicamente com um gate grep sobre `internal/sync/*_test.go` procurando `SyncToLinear(` ou `SyncToJira(` — análogo à família do AC5.

**`config.Load()` é `once.Do` (config.go:222-223):** após o fix, `syncToProvider` chama `config.Load()`. Em testes que fazem `os.Chdir` para `t.TempDir()`, o `Load()` pode retornar um valor cacheado do `Load()` anterior no mesmo processo — especialmente o valor default com `REQDir: "docs/req"`. `config.Reset()` existe (config.go:252-253) e está documentado como test-only, mas nenhum teste em `sync_test.go` o chama atualmente (os testes atuais não chamam `config.Load()`). Após o fix, AC2 ("falsificação com `req_dir` não-padrão") pode passar medindo o cache do default em vez do `req_dir` injetado — instrumento mentindo, não defeito corrigido. **O implementador deve chamar `config.Reset()` antes de cada caso de teste que injete um `trackfw.yaml` diferente.**

---

## 3. Alvos de falsificação em ambas as direções

Para cada superfície: onde o sabotador entra, qual gate deveria pegar, em qual direção.

### Superfície A — Enumerador literal (AC5)

| Direção | Entrada | Gate esperado | Modo de falha |
|---|---|---|---|
| Falso negativo (introduz sem pegar) | Novo literal em constante Go (`const reqGlob = "docs/req/*.md"`) expandida em chamada não coberta pelo grep | Grep do AC5 não casa nome de constante, só string literal | O gate passa, o literal vive |
| Falso positivo (rejeita legítimo) | O default em `config.go:304` (`REQDir: "docs/req"`) se o grep não tiver allowlist explícita | AC5 reprovaria o arquivo correto | Implementação bloqueada por gate defeituoso |

**Mitigação:** o grep do AC5 deve ter allowlist declarada (ex.: `config.go`, `configure.go`, `scaffold.go`, `help.go`, `discover.go`) e a allowlist deve ser auditada quando novos sítios legítimos forem adicionados. `scripts/check-referential-integrity.sh:10` deve ser declarado explicitamente como fora do escopo do AC5 ou o AC5 deve ser estendido para cobrir `scripts/`.

### Superfície B — Contenção de caminho (vetor 2.2, sem AC correspondente hoje)

| Direção | Entrada | Gate esperado | Modo de falha |
|---|---|---|---|
| Falso negativo (traversal passa) | `req_dir: ../../` em `trackfw.yaml` de teste | Contenção em `ResolveREQFiles` ou antes de chamá-la no sync | Sem contenção, o gate não existe — o vetor passa silenciosamente |
| Falso positivo (caminho legítimo rejeitado) | `req_dir: docs/requisições` rejeitado por contenção lexical que não tolera caracteres não-ASCII em `filepath.Rel` | Gate de contenção | Falsificação do AC2 falha por motivo errado |

**O AC que falta:** AC1 deve ser emendado ou um AC7 adicionado exigindo que `cfg.REQDir` seja verificado contra o CWD antes de ser passado a `resolveREQFiles` no caminho do sync. A verificação deve usar `EvalSymlinks` para não ser iludida por symlinks.

### Superfície C — Mensagem AC6 (informação exposta em log)

| Direção | Entrada | Gate esperado | Modo de falha |
|---|---|---|---|
| Falso negativo (expõe caminho absoluto) | Implementador usa `filepath.Abs(cfg.REQDir)` na mensagem | Revisão de código / teste que verifica a string logada não contém separador de path absoluto | Sem teste, passa na revisão |
| Falso positivo (mensagem muito genérica) | Mensagem diz só "No REQs found" sem o caminho — AC6 não especifica isso | AC6 como redigido não exige o caminho na mensagem | AC6 passa, mas o diagnóstico é inútil para o usuário |

**Regra:** a mensagem deve incluir `cfg.REQDir` verbatim, sem expansão. AC6 deve especificar isso.

### Superfície D — AC4 (sem escrita real em teste)

| Direção | Entrada | Gate esperado | Modo de falha |
|---|---|---|---|
| Falso negativo (cria issue real) | Teste novo chama `SyncToLinear()` diretamente com `LINEAR_API_KEY` presente | Gate grep por `SyncToLinear(` em `*_test.go` (análogo ao AC5) | Sem o gate, o teste passa localmente sem token, falha em CI ou cria issue em produção |
| Falso positivo | Nenhum identificado | — | — |

### Superfície E — Cache de `config.Load()` em AC2/AC3

| Direção | Entrada | Gate esperado | Modo de falha |
|---|---|---|---|
| Falso negativo (AC2 passa mas mede errado) | Teste de AC2 injeta `req_dir: docs/requisições` mas `config.Load()` retorna cache do default | `config.Reset()` antes de cada caso | Sem Reset, o teste mede `docs/req` (default cacheado) — AC2 "passa" mas o cenário discriminante não foi falsificado |
| Falso positivo | Nenhum identificado neste eixo | — | — |

---

## 4. Residual declarado

O que este design aceita não cobrir, dito claramente:

**R1 — Sem contenção de caminho em `ResolveREQFiles` (o mais importante).**  
`ResolveREQFiles` não chama `isOutsideCWD` e nenhum AC da REQ vigente a exige. O fix em AC1 como redigido introduz leitura fora da árvore para qualquer `req_dir` que aponte para fora. Este residual é **inadmissível sem emenda do AC**: o fix não pode ser entregue sem que AC1 (ou um AC7) exija contenção com `EvalSymlinks` antes de chamar `resolveREQFiles`. A decisão de onde implementar (wrapper no sync, guarda em `ResolveREQFiles`, ou novo AC sobre `config.Load()`) é do arquiteto — o parecer nomeia o requisito, não a implementação.

**R2 — `isOutsideCWD` é lexical (sem `EvalSymlinks`).**  
Pré-existente, compartilhado com outras superfícies no repositório. Mencionado porque qualquer contenção implementada para R1 que reutilize `isOutsideCWD` herda este limite. Não endereçado por esta REQ.

**R3 — Redirecionamento de host Jira via `jira_base_url`.**  
Pré-existente. Não introduzido nem fechado por esta REQ. Um `trackfw.yaml` controlado pelo atacante pode redirecionar POSTs autenticados para um host arbitrário. Linear não é afetado (URL hardcoded). Requer REQ própria se for endereçar.

**R4 — `scripts/check-referential-integrity.sh:10` hardcoda `docs/req/*.md`.**  
Fora do escopo do AC5 (`internal/**`). Não é um enumerador de produto, mas pode divergir de `req_dir` silenciosamente quando um consumidor usa path não-padrão. Não endereçado por esta REQ.

**R5 — AC4 é arquitetura, não mecânico.**  
Atualmente satisfeito pelo padrão de stub. Sem build tag de integração, sem interface injetável, sem env var gate. Um teste futuro pode criar issue real se chamar `SyncToLinear()`/`SyncToJira()` diretamente. O gate grep proposto na superfície D não é parte dos ACs atuais.

**R6 — Cache de `config.Load()` pode silenciar AC2/AC3.**  
`config.Reset()` existe mas não é chamado nos testes existentes. Após o fix, este é o vetor mais provável de falso positivo em AC2: o instrumento mede o default cacheado, não o cenário injetado. Deve ser AC explícito no ML de implementação.

---

## O que não consegui determinar

- **Se o arquiteto pretende emenda a AC1 ou criação de AC7** para a contenção de caminho — a evidência é clara (R1 acima), a decisão de onde colocar o requisito é de quem escreve os artefatos de governança.
- **Qual a implementação preferida da contenção:** wrapper no sync antes de chamar `resolveREQFiles`; guarda dentro de `ResolveREQFiles` (afetaria outros callers); ou validação em `config.Load()` no momento do parse (afetaria todos os consumidores de `cfg.REQDir`). Cada opção tem trade-offs de escopo, e a escolha entre elas é arquitetural.
- **Se `scripts/check-referential-integrity.sh:10` deve entrar no escopo do AC5** — o AC5 diz `internal/**`, mas o script de barreira que usa o literal errado é uma superfície real. Declarado como residual, não como bloqueio.
