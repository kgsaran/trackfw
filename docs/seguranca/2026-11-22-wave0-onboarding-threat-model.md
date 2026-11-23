# Threat Model — Wave 0: Onboarding verificável

**Data:** 2026-11-22  
**Revisor:** Hades (segurança)  
**Binário auditado:** `bin/trackfw` (v11.8.1)  
**REQ:** `docs/req/REQ-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md`  
**Isolamento:** todos os comandos em diretório temporário isolado com `HOME`, `USERPROFILE`, `CODEX_HOME`, `XDG_CONFIG_HOME` apontando para subdiretórios do temporário. Shell: `/bin/bash --noprofile --norc`.

**Governança:** `trackfw validate` medido no repositório atual → RC=0, 176 warnings (modo lenient `governance_mode: lenient`), 0 violations. `trackfw context` → 106 ADRs, score 100/100. Pré-condição de governança confirmada.

---

## Seção 1 — Completude da enumeração

### 1.1 Tabela flag × caminho

**`bin/trackfw init --help` (medido no binário):**

```
Flags:
      --ai-tools strings         AI tools to configure (...)
      --brownfield               Adopt governance gradually (lenient mode for 30 days)
      --forge string             Forge platform: github, gitlab, bitbucket, azure
  -h, --help                     help for init
      --identity-preset string   Agent identity preset: ...
      --scope string             Scope for AI tools installation: project or global (default "global")
```

Não há "Global Flags" separado — apenas `-h/--help` herdado pelo cobra. Lista fechada. Flags registrados em `internal/commands/init.go:184-188`.

| Flag | Tipo | Caminho não-TTY (retorno :309) | Caminho TTY (wizard) |
|---|---|---|---|
| `--brownfield` | bool | **IGNORADO** — só lido em :531, após o `return nil` em :309 | **FUNCIONA** — lido em :531; `cfg.BrownfieldMode = true` em :550; `cfg.LenientUntil` setado em :551 |
| `--ai-tools` | []string | **FUNCIONA** — `cmd.Flags().GetStringSlice("ai-tools")` em :304; passado para `executeScaffoldAndInstall` em :305 | **IGNORADO** — `var aiTools []string` em :321 (vazio); wizard preenche via `Value(&aiTools)` em :453; flag nunca lido no caminho TTY |
| `--identity-preset` | string | **FUNCIONA** — lido em :229, **antes** do check de TTY em :266 | **FUNCIONA** — mesmo caminho :229 |
| `--forge` | string | **FUNCIONA** — validado em :254; aplicado em :287 | **FUNCIONA** — validado em :254; default do wizard em :341-346 |
| `--scope` | string | **FUNCIONA** — aplicado em :294-298 | **FUNCIONA** — aplicado em :561-569 |

**Conclusão D1:** `--brownfield` e `--ai-tools` são simetricamente invertidos entre os dois caminhos. Ambos são violações de D1.

### 1.2 Medição de `--ai-tools` num TTY real

**Medição 1 — caminho não-TTY (confirmado):**
```
$ HOME="$H" bin/trackfw init --ai-tools claude --forge github < /dev/null
rc=0; 12 agentes em $H/.claude/agents/
```
Código: `:304` lê o flag, `:305` passa para `executeScaffoldAndInstall`.

**Medição 2 — caminho TTY tentado (não completada):**
Tentativa via `script -q /dev/null` + Enter spam abortou por timeout (14 s). O wizard abriu (capturou sequências de escape `[?25l` = cursor oculto, `[?2004h` = bracketed paste), mas o processo foi morto antes de completar instalação. "0 agentes instalados" nesta tentativa não é evidência de comportamento do flag — é evidência de que a instalação não atingiu esse passo.

**Evidência de código (definitiva para a TTY path):**
- `internal/commands/init.go:321`: `var aiTools []string` — inicializado **vazio** no caminho TTY
- `internal/commands/init.go:453`: `huh.NewSelect[string]().Value(&aiTools)` — **única** fonte do valor no caminho TTY
- Não há `cmd.Flags().Changed("ai-tools")` nem pré-preenchimento do wizard a partir do flag
- `internal/commands/init.go:573`: `executeScaffoldAndInstall(cfg, aiTools, ...)` usa o resultado do wizard

O flag é ignorado; o wizard sempre coleta o valor do zero. A medição TTY completa está designada para ML-4B (red team após fix).

### 1.3 Links de entrada que a troca de idioma (D3) pode quebrar

**Grep de fechamento:**
```bash
git grep -lE 'github\.io/trackfw|/en/guide|/guide/(getting-started|commands|ai-agents)|README\.md#' \
  -- . ':!internal/roadmapdoc/testdata' ':!scripts/testdata' ':!site/.vitepress/dist'
```

**Arquivos retornados (verbatim, medidos em 2026-11-22):**

```
docs/adr/ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md
docs/agents-working-context.md
docs/cli-parity.md
docs/qualidade/2026-08-16-remocao-de-plugins.md
docs/qualidade/2026-11-22-auditoria-onboarding.md
docs/req/REQ-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md
docs/req/REQ-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md
docs/req/REQ-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md
docs/req/REQ-2026-08-16-higiene-debitos-acumulados-na-entrega-de-remocao-de-plugins-e-release-7-0-0.md
docs/req/REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
docs/roadmaps/done/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md
docs/roadmaps/done/ROADMAP-2026-07-27-bloqueadores-de-release-de-paridade-e-precisao-contratual.md
docs/roadmaps/done/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md
docs/roadmaps/done/ROADMAP-2026-07-27-debitos-tecnicos-pos-release-de-robustez-e-manutenibilidade.md
docs/roadmaps/done/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md
docs/roadmaps/done/ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md
docs/roadmaps/done/ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md
docs/roadmaps/done/ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md
docs/roadmaps/done/docs-site-vitepress-2026-06-13.md
docs/roadmaps/wip/ROADMAP-2026-11-22-onboarding-verificavel-init-honra-as-flags-sem-tty-e-readme-e-site-batem-com-o-binario.md
docs/seguranca/2026-10-28-redteam-docs-e-install-ps1.md
docs/seguranca/2026-10-30-wave0-instalacao-de-consumidor-pela-tag.md
site/.vitepress/config.mts
site/en/guide/getting-started.md
site/en/index.md
site/guide/getting-started.md
site/index.md
vault/notes/listas-de-targets-hardcoded-divergem-do-catalog-json-2026-08-05.md
```

**Classificação:**

*Arquivos ao vivo (links que mudam com D3):*
- `site/.vitepress/config.mts`, `site/en/guide/getting-started.md`, `site/en/index.md`, `site/guide/getting-started.md`, `site/index.md` — ver tabela abaixo
- `docs/cli-parity.md` — referência viva (linha 385), deve ser atualizada em ML-1B

*Arquivados (menções históricas a caminhos de arquivo, não links ao vivo):*
- `docs/adr/ADR-2026-10-30-...` (1), `docs/agents-working-context.md` (1), `docs/qualidade/` (2), `docs/req/` (5), `docs/roadmaps/done/` (9), `docs/roadmaps/wip/ROADMAP-2026-11-22-...` (caminhos de arquivo a tocar, não links), `docs/seguranca/` (2), `vault/notes/` (1)

*Fora do escopo — zero ocorrências:* `internal/`, `npm/`, `pypi/`, `scripts/`, `.github/`. Lista fechada.

**D3** move inglês de `/en/` para `/` e PT-BR de `/` para `/pt/`. Links D3-específicos (vivos, rastreados):

| Arquivo | Localização | Link atual | Muda para | Gate automático |
|---|---|---|---|---|
| `site/.vitepress/config.mts` | linhas 22–24 | nav `/guide/getting-started`, `/commands`, `/ai-agents` (PT-BR) | `/pt/guide/*` | Não — config.mts é JS, não markdown body |
| `site/.vitepress/config.mts` | linhas 29–31 | sidebar PT-BR `/guide/*` | `/pt/guide/*` | Não |
| `site/.vitepress/config.mts` | linha 43 | locale root `link: '/en/'` | `link: '/'` | Não |
| `site/.vitepress/config.mts` | linha 46 | Home nav `link: '/en/'` | `link: '/'` | Não |
| `site/.vitepress/config.mts` | linhas 47–49 | nav `/en/guide/*` (EN) | `/guide/*` | Não |
| `site/.vitepress/config.mts` | linha 53 | sidebar key `/en/guide/` | `/guide/` | Não |
| `site/.vitepress/config.mts` | linhas 54–56 | sidebar items `/en/guide/*` | `/guide/*` | Não |
| `site/guide/getting-started.md` | linhas 310–311 | `/guide/commands`, `/guide/ai-agents` | `/pt/guide/*` | VitePress build (body) |
| `site/en/guide/getting-started.md` | linhas 310–311 | `/en/guide/commands`, `/en/guide/ai-agents` | `/guide/*` | VitePress build (body) |
| `site/index.md` | linha 11 | `/guide/getting-started` (frontmatter) | `/pt/guide/getting-started` | Incerto (frontmatter) |
| `site/en/index.md` | linha 11 | `/en/guide/getting-started` (frontmatter) | `/guide/getting-started` | Incerto |
| `docs/cli-parity.md` | linha 385 | referência a `site/guide/commands.md` e `site/en/guide/commands.md` | caminhos mudam | Nenhum |

**Total D3-específico: 22 localizações em 6 arquivos.** Os 15 matches em `config.mts` incluem `link: '/en/'` nas linhas 43/46.

**Gate gap:** 15 de 22 links estão em `config.mts` — `npm run build` não os verifica. Gate proposto para ML-1B (dentro de D6): após `npm run build`:
```bash
grep -rlE 'href="/trackfw/en/' site/.vitepress/dist           # → vazio
grep -rlE 'href="/trackfw/guide/' site/.vitepress/dist/pt     # → vazio (PT-BR não aponta para raiz EN)
```
Nota: `grep -rlE 'https?://[^"]*/en/' site` medido no repositório → 0 hits. Não há URLs externas com `/en/` no site, portanto o primeiro grep não tem falso-positivo externo hoje. ML-1B deve calibrar o segundo grep na primeira build — o locale switcher pode gerar hits que precisam de escopo mais estreito.

**Pré-existente, não causado por D3:** `kgsaran/trackfw#windows` em `site/guide/getting-started.md:304` e `site/en/guide/getting-started.md:304` (achado B.15). ML-1B deve corrigir.

**D5** (README): âncoras internas não quebram se os headings manterem o texto. `README.md:195, 223, 226` apontam para parágrafos em negrito (já quebradas, achado B.19 — não introduzidas por D5).

### 1.4 Scripts e testes que fixam conteúdo do README ou do site

| Artefato | Natureza do pin | Impacto de D1/D5 |
|---|---|---|
| `scripts/check-wheel-metadata-readme.sh` | Reconstrói a wheel PyPI do zero a cada execução (a partir do README.md atual). Edições no README não quebram este gate. | Não bloqueia edições de conteúdo em D5 |
| `scripts/check-init-preserves-user-config.sh` | **Análise estática de código:** verifica que `os.ReadFile` precede `os.WriteFile` nos corpos de `writeTrackfwConfig` e `generateLefthookHook`. Não executa testes Go. | Relevante para D2: a correção não deve introduzir overwrite incondicional. ML-1C deve manter este gate. |
| `internal/generators/trackfw_config_merge_test.go` | **6 testes unitários Go** (`TestWriteTrackfwConfig_*`). Medido: `go test -run TestWriteTrackfwConfig ./internal/generators/` → PASS. Cobre merge com chave ausente, presente, comentada, multiblock. | Relevante para D2 e ML-1C |
| `scripts/check-channels-content.sh` (linhas 672, 800, 924) | `package/README.md: b'# trackfw\n'` — stub do npm (1 linha), não o README raiz | Não afetado por D5 |
| `scripts/check-shim-diff-applies.sh` | `README.md` é "inerte" (file top-level); não verifica conteúdo | Não afetado por conteúdo |
| `internal/guardforms/readme_line_test.go` (linhas 124, 157, 161, 177) | Verifica linhas específicas dentro de **blocos fenced** no README | Move de seções não quebra; mudança de conteúdo em blocos fenced sim |
| `internal/commands/quickstart_e2e_test.go` (linhas 3, 100) | Exercita sequência do Quick Start mas **não lê o README** | ML-3A vai mudar este teste para ler README |
| `scripts/check-tty-detection.sh` (linha 55) | `init --ai-tools gemini </dev/null` não trava (non-TTY) | Pina comportamento não-TTY de `--ai-tools`; após fix D1, não muda |

**Site:** zero testes ou scripts fixam conteúdo de `site/`.

**Alertas para ML-1A:** `scripts/check-gates-falsify.sh` não tem cenário para `--brownfield` não-TTY nem para `--ai-tools` TTY. ML-1A deve verificar com o arquiteto se a adição é "cenário novo" (gates congelados, D6) ou "atualização de existente" antes de confirmar o escopo.

---

## Seção 2 — Threat model do D2

### 2.1 O que `init --brownfield` faz num projeto com `trackfw.yaml`

**Leitura relevante:** `internal/generators/scaffold.go:1033-1138`.

A função `writeTrackfwConfig` constrói o template, acrescenta `governance_mode: lenient\nlenient_until: <data>` ao final se `cfg.BrownfieldMode` (`:1084`), depois aplica merge textual: `presentTopLevelKeys(existing)` mapeia chaves já presentes; apenas chaves **ausentes** do arquivo existente são acrescentadas.

**Chaves de topo no template** (`:1049-1084`):
`frontend`, `backend`, `backend_framework`, `pkg_manager`, `hooks`, `ci`, `wip_limit`, `wip_by_squad`, `require_req_in_commit`, `rules`, `adr_dirs`, `req_dir`, `roadmap_dir`, `roadmap_namespacing`, `forge` (condicional), `agent_conventions` (condicional), `governance_mode` + `lenient_until` (condicional: BrownfieldMode).

**Contexto do repositório atual:** `trackfw.yaml` tem `governance_mode: lenient` + `lenient_until: "2027-12-31"`. Ambas as chaves presentes → merge seria NO-OP neste projeto.

**Estados finais do merge (derivados do código + validator medido com YAMLs pré-escritos):**

| Caso | YAML antes | Resultado do merge | Validate — discriminante |
|---|---|---|---|
| **A** | `governance_mode: strict` (sem `lenient_until`) | Merge acrescenta `lenient_until: 2026-12-22`; `governance_mode: strict` não muda | Sem `[LENIENT MODE]`, RC=1 |
| **B** | Sem `governance_mode` (todos os projetos do `trackfw init` padrão) | Merge acrescenta `governance_mode: lenient` + `lenient_until: 2026-12-22` | `[LENIENT MODE] until 2026-12-22`, RC=0 |
| **C** | `governance_mode: lenient` + `lenient_until: 2020-01-01` (expirado) | Ambas as chaves presentes → NO-OP | Sem `[LENIENT MODE]`, RC=1 |
| **D** | `governance_mode: lenient` (sem `lenient_until`) | Merge acrescenta `lenient_until: 2026-12-22`; `governance_mode` preservado | `[LENIENT MODE] until 2026-12-22`, RC=0 |

Derivação do merge: leitura de `scaffold.go:1083-1138`. Resultados do validator medidos com YAMLs pré-escritos no binário:

```
[A_strict]       Error: 3 violation(s) found  → RC=1 (sem banner)
[B_lenient_date] [LENIENT MODE] until 2026-12-22  → RC=0
[C_expired]      Error: 3 violation(s) found  → RC=1 (sem banner)
[D_lenient_new]  [LENIENT MODE] until 2026-12-22  → RC=0
```

**Risco concreto do Caso B:** `trackfw init` padrão não escreve `governance_mode`. Todo projeto inicializado com `trackfw init` está no Caso B. Depois do fix D1, qualquer agente pode chamar `init --brownfield </dev/null` nesses projetos via Bash — sem nenhum guard que bloqueie (ver §2.3). Resultado: `governance_mode: lenient` adicionado, projeto rebaixado de strict-por-omissão para lenient.

**Achado BLOQUEANTE — três grafias de chave bypassam `presentTopLevelKeys` e provocam downgrade de governance:**

`presentTopLevelKeys` (`scaffold.go:937`) extrai o trecho antes do primeiro `:` como texto bruto. Três grafias produzem uma chave textual distinta, fazendo o merge acrescentar `governance_mode: lenient`:

| Grafia | Chave extraída por `presentTopLevelKeys` | Chave lida por `yaml.Node` |
|---|---|---|
| `\xef\xbb\xbfgovernance_mode: strict` (BOM inicial) | `\xef\xbb\xbfgovernance_mode` | `governance_mode` |
| `"governance_mode": strict` (aspas YAML) | `"governance_mode"` | `governance_mode` |
| `governance_mode : strict` (espaço antes de `:`) | `governance_mode ` (com espaço) | `governance_mode` |

Nos três casos, `normalizeMapping` (`config.go:659`) é **last-wins** em duplicatas: `governance_mode: lenient` acrescentado no fim vence.

**Medição (YAMLs pré-escritos no binário):** para cada grafia, o arquivo de teste continha `<grafia>: strict` + `governance_mode: lenient\nlenient_until: 2027-11-22` (data 2027-11-22 pré-escrita manualmente para distinguir da data hoje+30d que o init geraria em produção). O discriminante é a presença do banner `[LENIENT MODE]`, não a data.

```
BOM case:   [LENIENT MODE] until 2027-11-22 | RC=0  ← downgrade de strict para lenient
Quoted:     [LENIENT MODE] until 2027-11-22 | RC=0  ← idem
Space:      [LENIENT MODE] until 2027-11-22 | RC=0  ← idem
Baseline:   sem [LENIENT MODE]              | RC=0  ← strict preservado (sem duplicate key)
```

**Escopo do BOM:** BOM fica no byte 0 do arquivo, portanto afeta apenas a chave que é a primeira linha do arquivo (antes de qualquer comentário). No template do `trackfw init`, a primeira linha é `# trackfw configuration` (comentário). Em yamls gerados pelo template, BOM é inatingível para `governance_mode`. Em yamls criados manualmente com `governance_mode` como primeira linha: reachable. As grafias de aspas e espaço são reachable em qualquer chave editada manualmente, independente de posição.

**Reachable hoje:** o caminho TTY de `--brownfield` funciona em 11.8.1.

### 2.2 Novo achado — mesmo mecanismo em chaves de diretório (qualquer init)

`presentTopLevelKeys` é chamado em `writeTrackfwConfig` para **todo init** (plain ou brownfield) em projetos com `trackfw.yaml` existente. As chaves de diretório (`adr_dirs`, `req_dir`, `roadmap_dir`) estão no template incondicional. Se qualquer uma dessas chaves estiver grafada com BOM, aspas ou espaço no arquivo existente, o merge a trata como ausente e acrescenta o default do template. `normalizeMapping` last-wins → o valor do usuário é silenciosamente sobrescrito.

**Medição end-to-end (binário + init real):**

Arquivo pré-init (BOM antes de `adr_dirs`, como primeira linha do arquivo):
```
\xef\xbb\xbfadr_dirs:    ← chave BOM-prefixed; presentTopLevelKeys vê "\xef\xbb\xbfadr_dirs"
  - custom-adr
req_dir: custom-req
roadmap_dir: custom-rm
```

Rodada: `HOME="$H" bin/trackfw init </dev/null` (sem brownfield) no diretório do projeto.

```
  ✓ docs/adr          ← init criou docs/adr (default do template), não custom-adr
  ✓ trackfw.yaml
  ...
  rc=0
```

Arquivo pós-init:
```
﻿adr_dirs:                 ← original BOM-prefixed preservado (presentTopLevelKeys não o reconheceu)
  - custom-adr
req_dir: custom-req        ← preservados porque foram reconhecidos
roadmap_dir: custom-rm
[... chaves do template ...]
adr_dirs:                  ← acrescentado pelo merge (presentTopLevelKeys não viu "adr_dirs")
  - docs/adr
```

`trackfw validate --json` → `{"warnings":[],"violations":[]}` (0 warnings, RC=0) — validate varre `docs/adr` (last-wins, existe), não `custom-adr`. ADRs existentes em `custom-adr` ficam invisíveis para o validator.

**Escopo do BOM neste achado:** BOM só alcança a chave na primeira linha do arquivo. Para yamls gerados pelo template, `adr_dirs` não é a primeira linha (está após comentários e 9 chaves). Este achado é reachable quando o usuário cria manualmente um yaml mínimo com `adr_dirs` como primeira linha, ou quando cola um yaml de editor Windows que adicionou BOM. As grafias de aspas e espaço são reachable para qualquer chave em qualquer posição.

**Causa raiz:** idêntica ao bypass de governance — `presentTopLevelKeys` extrai por texto, `yaml.Node` normaliza. Mesma causa, mesma REQ, novo ML.

**Mitigação necessária (ML-1C nesta REQ):** `presentTopLevelKeys` deve derivar as chaves presentes do mesmo parse `yaml.Node` que `config.go:478` usa. Isso cobre os três casos (BOM, aspas, espaço) por construção, sem lista de stripping.

### 2.3 Os guards impedem um agente de rodar `trackfw init`?

**Guards disponíveis** (`bin/trackfw guard --help`): `git-branch`, `write-scope`, `credential`. Nota: `guard` sem subcomando exibe a mensagem desatualizada "(git-branch or credential)" omitindo `write-scope` — string pré-existente.

**Medições diretas:**

```
write-scope: Write em trackfw.yaml SEM agent_id   → RC=0 — PERMITE
write-scope: Write em trackfw.yaml COM agent_id   → RC=2 — NEGA: "frozen path (trackfw.yaml)"
write-scope: Bash+init COM agent_id               → RC=0 — PERMITE (write-scope filtra por tool_name)
git-branch:  Bash com trackfw init (com/sem id)   → RC=0 — PERMITE
credential:  Bash com trackfw init                → RC=0 — PERMITE
```

**Conclusão:** o guard `write-scope` bloqueia subagentes de usar a ferramenta `Write` em `trackfw.yaml`. Não bloqueia chamadas `Bash` que invoquem `trackfw init`.

### 2.4 D2 recomendado

**Por que opção (b):** o Caso B (§2.1) mostra que qualquer agente pode relaxar um projeto strict-por-omissão com uma chamada `init --brownfield </dev/null` via Bash, sem nenhum guard bloqueando. ML-1C (correção de `presentTopLevelKeys`) é mandatório independente da escolha entre (a) e (b) — porque §2.2 afeta qualquer init, não só brownfield. Portanto ML-1C não é o diferenciador entre (a) e (b). O diferenciador é o Caso B e a ausência de guard. Com opção (b), o merge de governance não ocorre em yaml existente, e o Caso B fecha por construção.

**Recomendação: opção (b)** — `init --brownfield` procede normalmente (cria diretórios, instala agentes, faz merge de chaves não-governance), mas **não escreve** `governance_mode` nem `lenient_until` se `trackfw.yaml` já existir.

**Comportamento de referência para o precedente "warn + return":** `discover --init` (`internal/commands/discover.go:137-143`), quando `trackfw.yaml` já existe:
```go
if _, statErr := os.Stat(yamlPath); statErr == nil {
    fmt.Fprintln(out, "\n⚠ trackfw.yaml already exists — remove it first if you want to regenerate")
    return nil  // rc=0, sem escrita
}
```

**Comportamento escolhido para D2 (proceed-and-skip governance keys):** `init --brownfield` em yaml existente:
- Prossegue normalmente (merge de chaves não-governance, instalação de agentes, etc.)
- Não escreve `governance_mode` nem `lenient_until` (mesmo se ausentes no yaml existente)
- Emite "⚠ governance keys not modified — edit trackfw.yaml directly to enable lenient mode" em stdout
- rc=0

Isso é diferente do `discover --init` (que retorna completamente). A analogia com o `discover --init` vale apenas para o padrão "warn, não escreva, rc=0 para a parte de governance" — não para o comportamento completo do comando.

**Fixture discriminante para ML-1A:** `init --brownfield --ai-tools claude </dev/null` num projeto com yaml existente (sem `governance_mode`):
- Esperado: 12 agentes instalados (init prosseguiu); `governance_mode` **não** acrescentado ao yaml; stdout contém a mensagem de aviso; validate sem `[LENIENT MODE]`.

**Roteamento:** ML-1A implementa em `init.go`: em `writeTrackfwConfig`, quando `BrownfieldMode` e yaml já existe, pula a adição de `governance_mode`/`lenient_until` e emite o aviso. ML-1C corrige `presentTopLevelKeys` em `scaffold.go` (separado — afeta todo init, não só brownfield).

**ADR D2 (formulação para escrever antes da Wave 1):**

> `init --brownfield` cria `governance_mode: lenient` + `lenient_until: <hoje+30d>` somente em projetos sem `trackfw.yaml` existente. Na presença de `trackfw.yaml`, o comando prossegue (instala agentes, merge de chaves não-governance), mas não escreve `governance_mode` nem `lenient_until` — mesmo se ausentes — e emite "⚠ governance keys not modified — edit trackfw.yaml directly to enable lenient mode" em stdout (rc=0). Para projetos existentes que precisam de lenient mode, o usuário adiciona as chaves manualmente.

---

## Seção 3 — Alvos de falsificação nas duas direções

### 3.1 D1 — flags nos dois caminhos

**Regressão de D1 (flag ignorado num caminho):**

**R-1 — `--brownfield` ignorado em modo não-TTY (bug B.9, confirmado):**  
`"$BIN" init --brownfield < /dev/null` → `trackfw.yaml` sem `governance_mode` nem `lenient_until`. Validate sem banner.  
Gate ML-1A: `init --brownfield </dev/null` em projeto sem yaml → validate com `[LENIENT MODE]`.

**R-2 — `--ai-tools` ignorado no caminho TTY:**  
Código: `:321` aiTools vazio; `:453` wizard única fonte; flag nunca lido no TTY path.  
Gate ML-1A: no caminho TTY, `--ai-tools claude` deve pular a pergunta do wizard ou pré-selecionar (medição pelo ML-4B após fix).

**Comportamento indesejado (flag ativo onde não deveria):**

**U-1 — `--brownfield` ativa lenient em projeto strict com `governance_mode: strict` explícito?**  
Fixtures que discriminam (a grafia normal não é problema — apenas as três grafias de bypass):

| Fixture | Conteúdo do `trackfw.yaml` existente | Esperado após fix (opção b) |
|---|---|---|
| Normal strict | `governance_mode: strict` | `governance_mode` count não muda; stdout contém o aviso; validate sem `[LENIENT MODE]` |
| BOM strict | `\xef\xbb\xbfgovernance_mode: strict` | Idem |
| Quoted strict | `"governance_mode": strict` | Idem |
| Space strict | `governance_mode : strict` | Idem |

Sob opção (b): todos os quatro passam por construção (governance keys não são escritas em yaml existente). Gate ML-1A: `grep -c 'governance_mode' trackfw.yaml` antes e depois deve ser idêntico.

**U-2 — `--ai-tools` instala agentes quando não selecionados no wizard TTY após fix?**  
Gate ML-1A: `--ai-tools claude </dev/null` → 12 agentes instalados (medido hoje); deve-se manter após o fix.

### 3.2 D2 — limites do brownfield (após opção b)

**FN — `init --brownfield` não escreve governance em yaml existente?**  
Gate ML-1A: `init --brownfield --ai-tools claude </dev/null` em projeto com yaml existente sem `governance_mode`:
- `grep -c 'governance_mode' trackfw.yaml` antes e depois: idêntico (0)
- stdout contém "governance keys not modified"
- rc=0
- validate sem `[LENIENT MODE]`
- 12 agentes instalados (init prosseguiu)

**FN — `init --brownfield` cria lenient em projeto novo (sem yaml)?**  
Gate ML-1A: `init --brownfield </dev/null` em diretório sem `trackfw.yaml` → yaml criado com `governance_mode: lenient` + `lenient_until: <data>`; validate com `[LENIENT MODE]`; rc=0.

### 3.3 D3 — inglês na raiz do site

**FN (gate não pega link quebrado em config.mts):**  
15 dos 22 links estão em `config.mts` — não verificados por `npm run build`. Gate proposto para ML-1B:
```bash
grep -rlE 'href="/trackfw/en/' site/.vitepress/dist           # → vazio
grep -rlE 'href="/trackfw/guide/' site/.vitepress/dist/pt     # → vazio
```
ML-1B deve calibrar o segundo grep na primeira build — o locale switcher pode gerar hits que precisam de escopo mais estreito.

**FP (build quebra por link ainda válido):**  
Os 4 links em markdown body têm gate automático (VitePress build). Uma atualização parcial que deixa 1 dos 4 links antigos causa falha de build — intencional, o gate está funcionando.

### 3.4 D4 — documentação pinada pelo teste

**Pin vacuoso — comandos frequentes no README:**  
Se ML-3A usar prefixos (como `trackfw validate`), que aparecem 16x no README, remover o comando do Quick start não quebra o teste. Medidas das linhas **exatas** do Quick start:

| Linha exata | Ocorrências em README.md | Ocorrências em site/en/guide/getting-started.md |
|---|---|---|
| `trackfw init --ai-tools claude --forge github` | 1 | 0 |
| `trackfw adr new "User Authentication Strategy"` | 1 | 0 |
| `trackfw req new "User Authentication" --adr ADR-$(date +%F)-...` | 1 | 0 |
| `trackfw roadmap move ROADMAP-$(date +%F)-... wip` | 1 | 0 |
| `trackfw validate                  # ✓ No violations found.` | 1 | 0 |
| `trackfw status` (Quick start, continued) | 5 | 2 |
| `trackfw ship -m "feat(auth): add login flow"` | 1 | 0 |
| `trackfw serve` (Quick start, continued) | 4 | 0 |

Usando as linhas exatas, `trackfw init`, `adr new`, `req new`, `roadmap move`, `validate`, e `ship` são únicas e podem ser pinadas. `trackfw status` (5×) e `trackfw serve` (4×) **não são únicas** — sob D4's regra de sem-parsing, ML-3A não pode distinguir a ocorrência do Quick start das demais sem contexto de seção. Declarados em §4 como residual.

O site guide atual tem 0 correspondências para todas as linhas — ML-2A deve alinhar o conteúdo primeiro.

**FP — reformatação inocente do README quebra o teste:**  
Conferência literal — um espaço extra quebra. O contra-braço de ML-3A deve validar que a reformatação inocente do Quick start de fato falha o teste.

### 3.5 D5 — README contíguo

**FN — nenhum gate de âncora:**  
D5 reestrutura o README mas não há gate que verifique âncoras. Âncoras B.19 (`README.md:195, 223, 226`, já quebradas) não serão detectadas.

---

## Seção 4 — Residual declarado

1. **Nenhum guard bloqueia `trackfw init` via Bash.** Um agente pode chamar `init --brownfield` como Bash sem nenhum guard bloqueando. Se D2 escolher opção (b), a proteção é a ausência de merge de governance em yaml existente — não os guards. Projetos novos (Caso B) continuam vulneráveis se `--brownfield` for chamado antes de qualquer governance key existir.

2. **Achado BLOQUEANTE — três grafias bypassam `presentTopLevelKeys` para governance downgrade.** BOM inicial (apenas quando `governance_mode` está na linha 1 do arquivo), aspas YAML e espaço antes do `:` cada um causam downgrade de strict para lenient. Medido no binário. **Sob opção (b):** fecha por construção (governance keys não são escritas em yaml existente). **Sob opção (a):** permanece BLOQUEANTE — ML-1C é pré-requisito de merge antes de qualquer PR de D1/D2.

3. **Achado BLOQUEANTE — mesmo mecanismo em chaves de diretório (qualquer init, independente de brownfield).** BOM/aspas/espaço em `adr_dirs`, `req_dir` ou `roadmap_dir` fazem com que re-init sobrescreva o valor do usuário com o default do template. Medido end-to-end com binário real: `adr_dirs` com BOM → init criou `docs/adr`, validate varre `docs/adr` (não `custom-adr`), ADRs do usuário ficam invisíveis. **Não fechado por opção (b)** — afeta qualquer init. Requer ML-1C nesta REQ. ML-1C deve manter `scripts/check-init-preserves-user-config.sh` passando.

4. **`trackfw status` e `trackfw serve` não são únicos no README.** Aparecem 5× e 4× respectivamente. D4 (sem markdown parsing) não pode pinár essas linhas especificamente. Aceito como limitação do design D4.

5. **D5: nenhum gate de âncora.** Âncoras B.19 (já quebradas) não serão detectadas.

6. **Links `kgsaran/trackfw#windows` no site (B.15).** Pré-existente. Não detectável pelo VitePress build (link externo). ML-1B deve corrigir.

7. **Config.mts: 15 de 22 links D3 sem gate automático de build.** Gate proposto: dois greps no dist após build. ML-1B deve adotar e calibrar o segundo grep.

8. **`--ai-tools` no caminho TTY não pode ser testado por teste unitário sem injeção de TTY.** O fix de ML-1A deve criar um ponto de injeção ou mock de wizard. Aceito como débito de testabilidade.

9. **Medição TTY de `--ai-tools` não completada.** Evidência de código (:321 vazio, :453 wizard-only) sustenta a conclusão; medição direta designada para ML-4B.

10. **Medição de `--brownfield` no caminho TTY designada para ML-4B.** Comportamento TTY inferido do código (:531-:550), não medido com wizard completo.

11. **Stale error string em `guard` sem subcomando.** Exibe "(git-branch or credential)" omitindo `write-scope`. Pré-existente; não novo nesta REQ.

12. **Roadmap não tem ML-1C.** O achado §2.2 requer ML-1C (`scaffold.go:presentTopLevelKeys`). Nenhum ML atual tem `scaffold.go` em sua lista de arquivos. Esta é uma decisão do arquiteto — ML-1C deve ser adicionado ao roadmap antes de iniciar a Wave 1.
