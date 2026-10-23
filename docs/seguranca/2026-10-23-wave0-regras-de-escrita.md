# Wave 0 — Threat model: regras de escrita dos agentes viram mecanismo no harness

> Data: 2026-10-23
> Roadmap: ROADMAP-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-...
> REQ: REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-...
> ADR: ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md
> Autor: hades-tf

---

## 1. Completude da enumeração

A ADR lista quatro camadas (D1–D4) e declara um resíduo para Bash. Esta seção fecha a enumeração
por grep e medição no corpus real do worktree `/private/tmp/trackfw-toml-escape`.

### 1.1 Superfícies de escrita por CLI — matchers reais

Medição: `grep -n` nos blocos canônicos de emissão de cada CLI em `internal/generators/agentfiles.go`.
Os números de linha são os da inserção canônica (mergeClaudeHookArray ou equivalente),
não dos blocos de migração de legado.

| CLI | Matcher(s) de escrita | Evento | Linha real | Campo de caminho no payload |
|---|---|---|---|---|
| Claude Code | `Write\|Edit` | PreToolUse | 401 | `tool_input.file_path` (absoluto; Claude expande `~` e relativos) |
| Codex | `apply_patch` | PreToolUse + PostToolUse | 693, 710 | body do patch; não tem campo de caminho único |
| Gemini | `write_file\|replace` | BeforeTool + AfterTool | 859, 877 | a verificar; `write_file` provavelmente `tool_input.file_path` |
| Kiro | `write` | PreToolUse | 1025 | a verificar |
| Copilot | `create\|edit` | preToolUse | 1180 | a verificar |
| Cursor | `Write` | preToolUse | 1398 | a verificar |
| Windsurf | `pre_write_code` | — (evento raiz) | 1724 | `edits[*].new_string`; sem campo de caminho único documentado |
| Amazon Q | `fs_write` | preToolUse | 1898 | `file_text`/`new_str` per comentário linha 1895; sem `file_path` confirmado |

**Lacunas identificadas:**

1. `NotebookEdit` (Claude Code): o matcher `Write|Edit` é lista exata (pipe = separador de lista, não
   regex), confirmado pela documentação oficial em https://code.claude.com/docs/en/hooks. `NotebookEdit`
   não é casado. O ADR lista `Write|Edit|MultiEdit|NotebookEdit` como intenção; a implementação deve
   usar essa lista completa.

2. `MultiEdit` (Claude Code): não encontrado no código nem na documentação oficial de tools do Claude Code.
   Verificar se existe antes de incluir no matcher; se não existir, é string inerte.

3. `apply_patch` (Codex, linhas 693 e 710): o payload contém o diff como texto; não há campo
   `file_path`. Para D3/D4, o guard precisaria extrair todos os caminhos de `--- a/path` e `+++ b/path`
   do corpo do patch, ou declarar Codex como resíduo de D3/D4. D2 ainda funciona (o arquivo comprometido
   aparece no diff de git).

4. `NotebookEdit` path field (Claude Code): a documentação não confirma explicitamente o nome do campo.
   Requer verificação antes de implementar; presume-se `notebook_path` por analogia com a API
   oficial, mas não confirmado nesta wave.

5. Windsurf e Amazon Q: o payload de escrita não tem um campo de caminho único (`file_path`). Para D3/D4,
   o guard receberia apenas o evento sem saber o destino. Declarado como resíduo para esses CLIs;
   D3/D4 aplicam apenas ao Claude Code nesta REQ.

### 1.2 Identidade do subagente no payload do Claude Code

Fonte: https://code.claude.com/docs/en/hooks (Common input fields e Sub-agents).

| Campo | Thread principal | Subagente |
|---|---|---|
| `agent_id` | ausente | presente (UUID único) |
| `agent_type` | ausente (exceto se `--agent` flag) | presente (nome do agente, e.g. "apolo-tf") |

**Decisão para D4:** "é subagente" = `agent_id` presente no payload.
`agent_type` sozinho não basta porque aparece na thread principal com `--agent`.
Na ausência de `agent_id`, o guard aplica apenas D3 (comportamento de hoje, não nega).

**Outros CLIs:** nenhum dos demais CLIs tem documentação confirmada de identidade de subagente no
payload de hook. Declarado como não verificado; D4 aplica apenas ao Claude Code.

**Frontmatter hooks vs settings.json:** a documentação oficial diz que plugin subagents ignoram o
campo `hooks` no frontmatter. Portanto, o hook write-scope deve ser gerado em `settings.json`
(Claude Code), não no frontmatter do subagente. Frontmatter hooks em subagentes normais
(não-plugin) funcionam, mas a garantia é apenas enquanto o subagente está ativo — não protege a
thread principal. Decisão: wiring apenas via `settings.json` (como o credential guard atual).

### 1.3 Cobertura do credential guard para Bash

Código: `internal/guard/credential.go`.

```go
credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,]+`)
```

Vetores medidos manualmente (o guard extrai o redirect target e depois testa contra regras de
credencial, NÃO contra caminhos arbitrários):

| Vetor | Redireção capturada? | Passa no guard de credencial? |
|---|---|---|
| `echo x > docs/agents-working-context.md` | sim (`> docs/agents-working-context.md`) | **sim** — credential guard só nega credenciais, não caminhos |
| `echo x >> docs/agents-working-context.md` | sim | **sim** — mesma razão |
| `tee docs/agents-working-context.md` | não | sim (ignora) |
| `sed -i '' 's/x/y/' docs/agents-working-context.md` | não | sim (ignora) |

**Conclusão crítica:** o credential guard extrai o alvo do redirecionamento mas testa apenas se ele
parece uma credencial (padrões como tokens, chaves). Ele **não** nega escrita em caminhos
arbitrários. `echo x >> docs/agents-working-context.md` passa. A escrita via Bash em caminhos
congelados é **totalmente não coberta** pela D3 e D4 desta REQ. Só a D2 pega na auditoria de commit.

**Consequência para a REQ:** a ADR declara esse resíduo. Esta Wave 0 confirma que o resíduo inclui o
caso que gerou a REQ (o append do 2026-10-23 pode ter sido feito via Bash), e o resíduo fica.

### 1.4 Gramática de `Files affected` — corpus real

Medição via script sobre 1545 MLs extraídos de `docs/roadmaps/**/*.md`:

```bash
cd /private/tmp/trackfw-toml-escape
awk '/^\*\*Files affected:\*\*/{found=1; line=$0; next}
     found && /^\*\*/{
       if(line ~ /`/) print "backtick"
       else if(line ~ /a definir/) print "tbd"
       else if(length($0)>20) print "prose"
       else print "empty"
       found=0}
     /^### ML-/ && !found { print "no-label" }' \
  $(find docs/roadmaps -name "*.md") | sort | uniq -c
```

Resultado:

| Forma | Contagem | % |
|---|---|---|
| Backtick paths | 724 | 47% |
| Sem rótulo | 602 | 39% |
| Vazio (rótulo presente, sem conteúdo) | 104 | 7% |
| Só prosa | 46 | 3% |
| Glob dentro de backtick | 67 | 4% |
| `a definir`/TBD | 2 | <1% |

**Data de corte:** o rótulo aparece pela primeira vez em roadmaps de 2026-07-18. Roadmaps
criados antes dessa data nunca terão o rótulo. A D2 deve exibir a isenção explicitamente por
ML (não silenciosa) quando o roadmap for anterior ao corte.

**Roadmaps ativos post-corte sem rótulo:** medição nos estados wip e backlog:
- `ROADMAP-2026-09-12...` (backlog, sem rótulo, após corte) — 1 roadmap
- `ROADMAP-2026-09-08...` (backlog, sem rótulo, após corte) — 1 roadmap
Ambos estão em `backlog`, não em `wip`. Decisão: D2 falha apenas para MLs em `wip` que são
post-corte sem rótulo; roadmaps em `backlog`/`analyzing` recebem aviso, não falha.

**MLs "a definir" em wip:** o roadmap atual tem ML-3A, ML-3B, ML-4A, ML-4B e ML-5A com `Files
affected` vazios ou `a definir`. Esses MLs estão em wip. Pela regra acima, eles **reprovam D2**
quando atingidos pela wave correspondente. A Wave 0 deve preencher ML-3A e ML-3B; ML-4A/4B/5A
podem ter `a definir` até a wave anterior ser auditada.

### 1.5 Decisão de `Files affected` para ML-3A e ML-3B

Baseado no modelo de implementação da ADR e nas superfícies medidas acima:

**ML-3A (D3 — caminhos congelados):**
- `internal/commands/guard.go` — novo subcomando `write-scope`
- `internal/guard/writescope.go` — implementação do guard
- `internal/guard/writescope_test.go` — testes
- `internal/generators/agentfiles.go` — injeção do hook nos CLIs cobertos
- `docs/cli-parity.md` — contrato do `guard write-scope`
- `trackfw.yaml` — nova chave `frozen_paths` (se o default não vier hardcoded)

**ML-3B (D4 — escopo por despacho):**
- `internal/guard/writescope.go` — extensão com leitura de lease
- `internal/guard/writescope_test.go` — testes de D4
- `docs/cli-parity.md` — contrato do formato de lease

**ML-4A (hefesto-tf):** `docs/seguranca/2026-10-23-quality-write-scope.md`
**ML-4B (hades-tf):** `docs/seguranca/2026-10-23-red-team-write-scope.md`
**ML-5A (zeus-tf):** artefatos de orquestração (roadmap, orquestração set)

---

## 2. Modelo de ameaça

O adversário é o **implementador apressado e o arquiteto otimista**: quem entrega o mecanismo com
a intenção certa mas deixa buracos que produção vai encontrar antes de qualquer atacante externo.

### 2.1 D1 — A instrução na origem

**Estado atual dos assets:** os 12 assets em `internal/integrations/assets/agents/` têm atualmente
a instrução "Write a new file in `docs/agents-working-context/<timestamp>-<slug>.md`" (verificado
em `security.md`). Isso ainda é **escrita** — no subdiretório, não no `.md` raiz, mas escrita.
A ADR D1 diz que os papéis não-arquiteto devem **não escrever**, e sim reportar no relatório final,
com o arquiteto registrando. ML-1A precisa mudar os 12 assets (não só global).

O `architect.md` mantém "Write a new file in `docs/agents-working-context/...`" — isso não muda.

**Ameaça D1-A — instrução do asset muda, global persiste:**
Os arquivos globais em `~/.claude/agents/` têm a instrução antiga. A instrução do sistema carregada
pelo Claude é a do arquivo global, não do repositório. ML-1A corrige os assets do repositório; os
globais permanecem divergentes até `trackfw agents update` (fora do escopo desta REQ, declarado em
Negative scope). As D2–D4 são o backstop necessário.

**Ameaça D1-B — golden desatualizado:**
O teste de golden compara o arquivo gerado contra um snapshot. Se o snapshot não for atualizado
junto com o asset, o CI falha e expõe a divergência. O risco inverso: um refator que regenera os
assets sem atualizar os goldens faz o CI falhar ruidosamente. AC1 exige que goldens e assets mudam
juntos, e um teste afirma a forma correta por papel (não só compilação).

**Ameaça D1-C — condição arquiteto/resto invertida:**
Um único template com condicional é mais testável que 12 arquivos divergentes. Se a condição
inverter num refator, o arquiteto perde a permissão de escrever. O teste deve afirmar explicitamente:
arquiteto → forma "Write"; os outros 11 → forma "report instead".

### 2.2 D2 — `ml_scope` no barrier

**Decisão sobre a base do diff (Wave 0 define):**
Usar `git diff --name-only $(git merge-base origin/main HEAD) HEAD` com a **união dos `Files
affected` de todos os MLs de waves ≤ N** como conjunto permitido. Não é necessário âncora de SHA
no frontmatter — o merge-base com a main é estável e não forgeable por um agente (não está na
árvore). Arquivos no diff que não estejam na união nem no conjunto de orquestração reprovam.

Vantagem: cobre arquivos staged/untracked implicitamente (aparecem no status, e a checagem deve
incluir `git status --porcelain` para untracked, além do diff de commits).

**Ameaça D2-A — roadmap `Files affected` editado pelo agente:**
O roadmap está no conjunto de orquestração (sempre permitido). Um agente pode editar o próprio
`**Files affected:**` para incluir o arquivo que quer tocar, e a checagem passa. Mitigação: o
barrier deve ler `Files affected` da **base ref** (`git show $(git merge-base origin/main HEAD):<roadmap>`),
não do arquivo em disco. Este é o mesmo padrão de `credentialGuardAnchoredRules` (lê as regras
do HEAD anterior, não do disco). Sem esta âncora, D2 é trivialmente contornável.

**Ameaça D2-B — `trackfw.yaml` ampliado pelo agente:**
Se `orchestration_paths` for lido do `trackfw.yaml` em disco, um agente pode adicionar `**` antes
de commitar, e todos os arquivos passam. O `trackfw.yaml` deve ser lido da base ref ou deve ser
incluído na lista de congelados (D3). Decisão: incluir `trackfw.yaml` na lista de congelados da D3
para agentes não-arquiteto. O barrier também pode lê-lo da base ref como segunda defesa.

**Ameaça D2-C — MLs sem rótulo pós-corte passam silenciosamente:**
Se "rótulo vazio = isento", 104 MLs vazio-rotulados mais 602 sem rótulo escapam. A regra:
post-corte em wip sem rótulo → reprova. Pre-corte → isento com mensagem explícita no output do
barrier (não silencioso). `a definir` em backlog/analyzing → aviso; em wip → reprova.

**Ameaça D2-D — diff não inclui untracked:**
`git diff --name-only` só vê commits e staged. Um agente que cria um arquivo sem commitar escapa.
O barrier deve combinar diff de commits com `git status --porcelain` (que inclui untracked `??`).

### 2.3 D3 — caminhos congelados no guard

**Decisão de CLIs cobertos nesta REQ:**
Apenas Claude Code recebe wiring de `write-scope` nesta REQ. Os demais CLIs (Codex, Gemini, Kiro,
Copilot, Cursor, Windsurf, Amazon Q) são declarados como resíduo de D3 por dois motivos:
(a) o payload de escrita de vários deles não expõe um campo de caminho único (`apply_patch`,
`pre_write_code`, `fs_write` com `file_text`/`new_str`); (b) o campo de caminho dos outros
(`write_file`, `write`, `create|edit`) não foi confirmado como `file_path` nesta wave.
`docs/cli-parity.md` deve documentar explicitamente a cobertura e a lacuna.

**Ameaça D3-A — matcher incompleto (`NotebookEdit` falta):**
`Write|Edit` é lista exata; `NotebookEdit` não é casado. Uma escrita de notebook passa pelo
PreToolUse sem disparar o guard. Usar a lista completa da ADR:
`Write|Edit|NotebookEdit` (sem `MultiEdit` até confirmar que existe).
Campo de caminho para `NotebookEdit`: presumidamente `notebook_path`, mas não confirmado;
se o guard tentar ler `file_path` e o campo se chamar `notebook_path`, cai no branch de
"caminho ausente" → decisão de comportamento obrigatória (DENY ou ALLOW).
Decisão conservadora: campo ausente ou não string → DENY (exit 2).

**Ameaça D3-B — normalização de caminho:**
`file_path` do Claude Code é sempre absoluto. Mesmo assim:
- `/tmp/x` e `/private/tmp/x` no macOS são o mesmo arquivo (`/tmp` é symlink).
- Hardlinks diferentes apontam para o mesmo inode.
- APFS é case-insensitive: `docs/Agents-Working-Context.md` casa `docs/agents-working-context.md`.
Usar `filepath.EvalSymlinks` no caminho alvo **e** no caminho congelado, depois comparar
canonicamente. `os.SameFile` funciona para targets que já existem; para escrita criando arquivo
novo, usar `filepath.EvalSymlinks` no diretório pai + `filepath.Base`, comparar com prefixo
canonical do congelado.
`filepath.Clean` em caminho absoluto elimina `..` lexicalmente, mas não resolve symlinks. A proteção
real vem de `EvalSymlinks`, não do `Clean`.

**Ameaça D3-C — Windows: backslash, drive letter, UNC, MSYS:**
No Windows, `file_path` pode vir como `C:\docs\agents-working-context.md`,
`c:\docs\agents-working-context.md`, `/c/docs/agents-working-context.md` (MSYS) ou
`\\?\C:\...` (UNC longo). Decisão de comportamento:
- Normalizar separadores (`filepath.FromSlash`) e fazer o path case-insensitive na comparação (`strings.EqualFold`).
- Caminhos MSYS (`/c/path`) devem ser reconhecidos e convertidos para `C:\path`.
- Caminhos UNC longo (`\\?\`) devem ter o prefixo removido antes da comparação.
- `normalizeGuardPath` do guard atual só converte `\`→`/` com letra de unidade; o gap MSYS já
  foi medido (vault note `project_normalize_guard_path_cobre_meio_espaco_de_gramatica.md`). D3 não
  deve repetir esse erro: definir o conjunto de gramáticas suportadas explicitamente e testar
  cada uma em AC5.

**Ameaça D3-D — o agente edita os arquivos de configuração de hook (auto-disable):**
Um agente pode fazer `Edit`/`Write` em `.claude/settings.json`, `.claude/settings.local.json`,
`.codex/hooks.json`, `.gemini/settings.json` ou qualquer arquivo de hook gerado pelo `trackfw init`.
Sobrescrever esses arquivos remove o hook write-scope — e a escrita seguinte passa sem interceptação.
Mitigação: os arquivos de configuração de hook de todos os CLIs instalados devem estar na lista
de congelados da D3. A detecção não depende de identidade (D3 é identity-free por design da ADR),
então vale para toda escrita, incluindo a do arquiteto.
**Exceção:** o arquiteto precisa poder atualizar esses arquivos via `trackfw init`/`update`. A
isenção não é de identidade (D3 não tem identidade) — é de contexto de invocação (`trackfw init`
não passa pelo hook PreToolUse do Claude Code; é um comando de terminal). O freeze aplica a
ferramentas de agente (Write/Edit tool), não ao binário trackfw em si.

**Ameaça D3-E — payload MCP:**
Ferramentas MCP que escrevem no filesystem não dispararam `PreToolUse` de `Write|Edit`. A D3 não
intercepta escrita via MCP. Declarado como resíduo permanente nesta REQ.

**Ameaça D3-F — fail-open por erro:**
Se o guard não consegue parsear o payload (stdin corrompido, encoding errado, campo ausente, BOM
não strippado, FIFO em vez de arquivo — que suspende indefinidamente), a política deve ser DENY
(exit 2), não ALLOW. O guard Go deve usar `defer func() { if r := recover(); r != nil { os.Exit(2) } }()`
mas isso cobre apenas panics. Outros caminhos de falha (timeout de leitura, erro de unmarshal,
campo ausente) também devem retornar exit 2 explicitamente. BOM: `ExtractCommand` já faz
`bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})`; o novo guard deve herdar esse comportamento.
Vetor de teste obrigatório em AC5: payload com BOM prefixado.

**Ameaça D3-G — lista congelada vem do disco (forgeable):**
A lista de caminhos congelados em `trackfw.yaml` é lida do disco em runtime. Um agente pode editar
o `trackfw.yaml` para remover um caminho antes de escrever nele. Mitigação: `trackfw.yaml` deve
estar na lista de congelados hardcoded (não extensível por ele mesmo); a lista lida do disco é um
**acréscimo**, nunca uma substituição da lista padrão.

### 2.4 D4 — escopo por despacho

**Ameaça D4-A — path traversal no `agent_type` como nome de arquivo:**
O lease é em `.trackfw/leases/<agent_type>.json`. Se `agent_type` for `../../../etc/passwd` ou
`foo/bar`, o `filepath.Join` pode escapar da pasta `leases/`. Validar `agent_type` como slug
`[a-zA-Z0-9_-]+` (sem ponto, sem barra) antes de montar o caminho. `agent_type` que não casa →
"sem lease" (aplica D3 apenas), nunca erro fatal.

**Ameaça D4-B — dois agentes do mesmo tipo em worktrees paralelos:**
Um único `leases/apolo-tf.json` governa todas as instâncias do `apolo-tf`. Uma escrita da instância
do wt2 em wt1 é autorizada se o lease citar wt1. Comparação: `file_path` contra os worktrees do
lease (não o cwd do processo — a ADR rejeitou cwd). O lease declara uma lista de worktrees
autorizados; comparação com `filepath.EvalSymlinks`. Resíduo para esta REQ: se dois apolo-tf
rodam em paralelo, a permissão mais permissiva vence (não há distinção por instância).

**Ameaça D4-C — lease corrompido → fail-open:**
Se `.trackfw/leases/<agent_type>.json` existe mas é ilegível (permissão 000, diretório, JSON
inválido), o guard deve DENY, não fazer fallback silencioso para D3-only. Um lease ilegível
pode ser resultado de adulteração; recompensar com ALLOW inverte o sentido do controle.

**Ameaça D4-D — agente edita o próprio lease:**
`.trackfw/leases/` deve estar na lista de congelados da D3. Sem isso, um agente escreve um lease
mais permissivo antes de realizar a escrita proibida. A proteção aplica-se a todo agente — não
depende de identidade (D3 é identity-free).

**Ameaça D4-E — `agent_type` ausente → comportamento:**
Quando `agent_id` está ausente (thread principal) ou `agent_type` está ausente (raro), o guard
não tem com quem comparar o lease. Comportamento: aplica D3 apenas (sem D4). Isso é o
comportamento correto declarado na ADR: "na ausência de permissão para o agente, vale só a D3."
Não é uma brecha, é o design.

---

## 3. Alvos de falsificação nos dois sentidos

### D1

| Direção | Sabotage | Gate esperado | Evidência |
|---|---|---|---|
| FN (viola, passa) | Asset corrigido no repo, global `~/.claude/agents/` ainda manda escrever | Nenhum gate automático; D2–D4 são o backstop | Global fora do repo; `trackfw agents update` fora do escopo |
| FN (viola, passa) | Golden não atualizado junto com o asset; teste só verifica compilação | `go test ./internal/integrations/` com teste por papel | AC1 exige teste por papel |
| FP (válido, reprova) | Arquiteto é bloqueado de escrever no working context | Teste do arquiteto afirma forma "Write"; 11 outros afirmam forma "report" | Não há gate que bloqueie o arquiteto hoje |

### D2

| Direção | Sabotage | Gate esperado | Evidência |
|---|---|---|---|
| FN (viola, passa) | Agente edita `Files affected` no roadmap em disco antes de commitar | Barrier lê `Files affected` da base ref (merge-base), não do disco | Sem âncora de leitura, D2 é trivialmente contornável |
| FN (viola, passa) | Agente amplia `orchestration_paths` no `trackfw.yaml` em disco | `trackfw.yaml` na lista de congelados D3; barrier lê da base ref | D2-B |
| FN (viola, passa) | Arquivo criado sem commit (untracked) escapa do diff | Barrier combina `git diff` + `git status --porcelain` | D2-D |
| FN (viola, passa) | ML pós-corte sem rótulo isento silenciosamente | "vazio pós-corte em wip" → reprova com mensagem explícita | Medição: 104 MLs com rótulo vazio, 602 sem rótulo |
| FP (válido, reprova) | Commit de roadmap/REQ/ADR reprova por não estar em `Files affected` | Conjunto de orquestração fixo sempre permitido | Sem a lista, barrier trava o ciclo de governança |
| FP (válido, reprova) | Roadmap pré-corte reprova por ausência de rótulo | Isenção pré-corte com mensagem explícita (não silenciosa) | Corte: 2026-07-18 |

### D3

| Direção | Sabotage | Gate esperado | Evidência |
|---|---|---|---|
| FN (viola, passa) | Escrita via `NotebookEdit` — não casado por `Write\|Edit` | Matcher completo `Write\|Edit\|NotebookEdit` | Docs oficiais: `\|` é separador de lista exata |
| FN (viola, passa) | Escrita em `/tmp/x` quando congelado é `/private/tmp/x` | `filepath.EvalSymlinks` em ambos antes de comparar | macOS: `/tmp` → `/private/tmp` |
| FN (viola, passa) | Agente sobrescreve `.claude/settings.json` e remove o hook | `.claude/settings*.json` na lista de congelados hardcoded | D3-D |
| FN (viola, passa) | `trackfw.yaml` editado para remover caminho congelado | `trackfw.yaml` hardcoded na lista; extensão pelo yaml é acréscimo | D3-G |
| FN (viola, passa) | Escrita via MCP — não passa por PreToolUse `Write\|Edit` | Nenhum gate — resíduo declarado | D3-E |
| FN (viola, passa) | JSON ilegível, campo ausente → fail-open | Exit 2 em todo caminho de erro; `recover` para panics; sem fallback | D3-F |
| FP (válido, reprova) | Escrita legítima em `docs/agents-working-context/<arquivo>.md` é bloqueada | Congelar apenas o arquivo `.md` raiz, não o diretório inteiro | Lista de congelados por caminho exato, não por prefixo de diretório |
| FP (válido, reprova) | Escrita via `trackfw init`/`update` nos arquivos de hook | `trackfw init` não passa pelo PreToolUse — é terminal, não tool do agente | Isenção por mecanismo de invocação, não por identidade |

### D4

| Direção | Sabotage | Gate esperado | Evidência |
|---|---|---|---|
| FN (viola, passa) | `agent_type` com `/` → path traversal no lease | Slug validation `[a-zA-Z0-9_-]+` antes de `filepath.Join` | D4-A |
| FN (viola, passa) | Agente edita `.trackfw/leases/<agent_type>.json` para ampliar escopo | `.trackfw/leases/` na lista de congelados D3 | D4-D |
| FN (viola, passa) | Lease corrompido → fallback para D3-only (liberado) | Lease ilegível → DENY | D4-C |
| FN (viola, passa) | `file_path` comparado contra cwd em vez de worktrees do lease | Comparar `file_path` contra `worktrees[]` do lease com `EvalSymlinks` | ADR rejeitou cwd; D4-B |
| FP (válido, reprova) | Agente sem lease ativo é negado globalmente | Sem lease → D3 apenas (não DENY global) | ADR: "na ausência, comportamento de hoje" |

---

## 4. Resíduo declarado

O que este design aceita não cobrir, declarado sem eufemismo:

**R1 — Bash e escrita indireta:** `tee`, `sed -i`, `cp`, Python/Node inline, qualquer escrita que não
passe por uma tool de escrita do CLI, inclusive `echo >>`. O credential guard captura o alvo do
redirecionamento mas não testa contra caminhos congelados — só testa contra padrões de credencial.
O caso exato que motivou esta REQ (append de 2026-10-23) pode ter sido Bash. Só a D2 captura na
auditoria de commit. Este resíduo não é fechado por nenhuma wave desta REQ.

**R2 — Arquivos globais (`~/.claude/agents/`):** a instrução corrigida está nos assets do repositório.
A instrução global divergente prevalece para agentes carregados por caminho global. Fecha com
`trackfw agents update` em release futura.

**R3 — CLIs sem `file_path` único (Codex, Windsurf, Amazon Q):** `apply_patch` (Codex) expõe
o patch body; `pre_write_code` (Windsurf) usa `edits[*].new_string`; `fs_write` (Amazon Q) usa
`file_text`/`new_str`. D3/D4 não se aplicam a esses CLIs nesta REQ.

**R4 — Gemini, Kiro, Copilot, Cursor:** campo de caminho não confirmado nesta wave. D3/D4 não
são implementados para esses CLIs até a confirmação.

**R5 — MCP de escrita:** não passa por PreToolUse de tool de agente. Permanente nesta arquitetura.

**R6 — Dois agentes do mesmo `agent_type` em paralelo:** o mesmo lease governa ambos; a instância
menos privilegiada herda a permissão da mais privilegiada.

**R7 — `NotebookEdit` campo de caminho:** presumidamente `notebook_path`, não confirmado. Se o
campo diferir de `file_path`, o guard cai no branch de "campo ausente → DENY" (comportamento
conservador) mas nunca no branch de "campo errado → ALLOW". O resíduo é que todos os notebook
edits serão negados até confirmação e fix.

**R8 — `MultiEdit` não existe:** se o nome da tool não existir no Claude Code, o matcher para ele
é string inerte. Sem custo de segurança; sem cobertura.

## §Z — Auditoria do arquiteto (zeus-tf, 2026-10-23)

**Aprovado, com três correções.**

1. **MCP (D3-E e R5) — a premissa estava errada.** A documentação oficial (<https://code.claude.com/docs/en/hooks>) diz:
   "MCP server tools appear as regular tools in tool events (`PreToolUse`, …)", com nome `mcp__<server>__<tool>`. A escrita
   via MCP **passa** pelo PreToolUse. O resíduo é outro: cada servidor tem o seu próprio `tool_input`, sem um campo de
   caminho padronizado. A D3 não sabe extrair o caminho de um servidor MCP genérico. O R5 continua resíduo, por esse motivo.
2. **Matcher, confirmado.** Pela mesma página, uma string com apenas letras, dígitos, `_`, `-`, espaço, `,` e `|` é lida
   como lista exata, e qualquer outro caractere a transforma em regex JavaScript sem âncora. `Write|Edit|NotebookEdit` é
   lista exata. Nunca usar `Edit.*`, que casaria também `NotebookEdit` por regex.
3. **Lista de congelados em dois níveis (decisão do arquiteto; adendo na ADR).** A lista proposta congela `trackfw.yaml`,
   `.claude/settings*.json`, `.codex/hooks.json` e outros para **todos**, inclusive a sessão principal. Isso muda a sessão
   principal além do que a REQ permite: o negative scope diz "não muda o comportamento da sessão principal fora dos
   caminhos congelados", e o KG edita esses arquivos pela sessão principal. A decisão fica assim:
   - **Congelado para todos:** `docs/agents-working-context.md`.
   - **Congelado só para subagente** (`agent_id` presente): `trackfw.yaml`, os arquivos de hook e de settings de todo CLI
     cabeado pelo trackfw, e `.trackfw/leases/`. A ameaça que a Wave 0 apontou é o agente desligando o próprio guard ou
     editando a própria permissão, e ela vem de subagente.
   - O `trackfw.yaml` pode acrescentar caminhos às duas listas, nunca remover.

A recomendação de cabeamento por `settings.json`, e não por frontmatter, fica aceita. A `Files affected` do ML-3A ganha os
sítios de fiação e de validação no handoff.
