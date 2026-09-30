# Triagem de Issues Abertas — 2026-09-28

> Binário usado: `./bin/trackfw` v9.0.1  
> Plataforma: macOS (darwin)  
> Metodologia: reprodução por execução, não por leitura de título  
> Restrições: nenhum commit, nenhuma branch criada

---

## #308 — MSYS brace expansion destrói `{owner}/{repo}` quando pai é nativo

**Veredito:** (D) — Não verificável em macOS; reproduzível apenas em Windows com Git for Windows  
**Gravidade:** — (n/a)

**Reprodução executada:**  
Não executada. O defeito requer que um processo não-MSYS (Python ou Go) lance um binário MSYS (bash.exe) e que o runtime MSYS reconstrua o `argv` a partir da linha de comando do Windows, aplicando brace expansion. Esse mecanismo não existe em macOS.

**O que falta para medir:** runner Windows ou VM com Git for Windows (MSYS x64). O reprodutor mínimo do reporter é `bash.exe -c 'printf "%s\n" "$@"' _ 'repos/{owner}/{repo}'` executado de um pai nativo.

**Estado conhecido (via comentários do issue):**  
Workaround com `MSYS=noglob` + 3 env vars foi medido no runner `windows-latest` (run 34470154546): 48 rótulos fechados, 0 regressões. O proprietário confirmou que o mecanismo é brace expansion pelo receptor (não conversão de caminho), e que o `noglob` funciona mas é frágil. ML aberto na REQ-2026-09-03 para migrar para rota stdin/env var independente de plataforma. Issue ainda aberta porque a solução durável não foi implementada.

---

## #364 — Sinal `[-1 resolvido]` do ratchet de Windows não reprova nem fecha

**Veredito:** (D) — Não verificável em macOS; requer Windows CI runner com `windows-known-failures.json`  
**Gravidade:** — (n/a)

**Reprodução executada:**  
Não executada. O defeito está no workflow do ratchet de CI do Windows: quando uma falha declarada passa, o sinal `[-1 resolvido]` não causa reprovação nem remove a entrada da lista. Requer execução de CI no runner Windows.

**O que falta para medir:** runner Windows para observar comportamento do ratchet. Nenhum commit encontrado que aborde especificamente este mecanismo.

---

## #403 — Pesos pessimistas (54 s) em 3 rótulos `falsify/write-containment`

**Veredito:** (D) — Não verificável em macOS; requer `FALSIFY_TIMING_FILE` gerado em CI  
**Gravidade:** — (n/a)

**Reprodução executada:**  
Não executada. `scripts/gen-falsify-scenario-weights.py` requer `FALSIFY_TIMING_FILE` gerado durante execução de CI. Localmente o arquivo não existe. O defeito é de balanceamento de shards (desempenho, não corretude), e nenhum commit foi encontrado recalibrando os pesos dos rótulos `write-containment/unguarded-write`, `write-containment/marker-accepted` e `write-containment/vacuous-scan`.

**O que falta para medir:** execução CI que produza o timing file + verificar se os avisos de "peso pessimista" somem para os 3 rótulos.

---

## #407 — `.trackfw-log` sem fuso/autor; status de ML sem saída legível por máquina

**Veredito:** (A) — Não reproduz. Corrigido em dois PRs distintos.  
**Gravidade:** — (n/a)

**Reprodução executada:**

```
$ tail -2 docs/roadmaps/.trackfw-log
2026-09-28 10:51  ROADMAP-2026-09-28-....md  backlog → wip  -0300  (kgsaran@gmail.com)
2026-09-28 18:13  ROADMAP-2026-09-28-....md  wip → done  -0300  (kgsaran@gmail.com)

$ ./bin/trackfw roadmap show ROADMAP-2026-09-22-init-e-discover... --json | python3 -m json.tool | grep '"status"'
    "status": "pending",
```

**Commit que corrigiu — Parte 1 (log):** `caa1f864` "feat(log): a linha de transicao ganha fuso e autor, no fim, sem mudar o parser (#457)". Novas entradas têm `-OFFSET (autor@email)` no fim; o `ParseLog` existente continua lendo entradas antigas sem mudança (sufixo não ancorado).

**Commit que corrigiu — Parte 2 (ML status):** `85bbee60` "roadmap show --json: 292 grafias de status viram 3 categorias, e o consumidor para de reimplementar o dialeto (parte 2 da #407) (#461)". `roadmap show --json` agora emite `"status": "pending"/"in_progress"/"complete"` para cada ML.

---

## #408 — `TestCorpusMeasurement_ReportOnly` usa `t.Logf`, mas o job `go` roda sem `-v`

**Veredito:** (B) — Reproduz integralmente  
**Gravidade:** BAIXA — visibilidade interna de CI; nenhum impacto ao consumidor

**Reprodução executada:**

```
# quality.yml job "go", linha 40:
- run: go test -timeout 2m ./...      ← sem -v

# O teste ainda existe:
$ grep -n "TestCorpusMeasurement_ReportOnly" internal/roadmapdoc/roadmapdoc_test.go
241:func TestCorpusMeasurement_ReportOnly(t *testing.T) {

# Com -v (prova que t.Logf emite):
$ go test -run TestCorpusMeasurement_ReportOnly -v ./internal/roadmapdoc/...
--- PASS: TestCorpusMeasurement_ReportOnly (0.07s)
    done/ corpus: total=194, unfinished(StatusIsComplete)=26, unfinished(HasUnfinishedMLs)=25

# Sem -v (o que o job faz): saída nula — t.Logf descartado em testes que passam
```

O job `go` (ubuntu-latest, linha 40 do quality.yml) roda `go test -timeout 2m ./...` sem `-v`. Os únicos jobs com `-v` são os de Windows (PowerShell), mas lá o teste faz `t.Skipf` porque `docs/roadmaps/done/` não existe naquele contexto. A contagem nunca aparece em nenhum job de CI.

---

## #421 — `chmod` não tira bit de execução em NTFS montado `noacl`

**Veredito:** (D) — Não verificável em macOS; requer Windows com NTFS montado `noacl`  
**Gravidade:** — (n/a)

**Reprodução executada:**  
Não executada. O defeito requer NTFS montado com a opção `noacl` (padrão no Git for Windows `TMPDIR`). Em macOS, `chmod` funciona normalmente. O reporter mediu que em Windows `chmod 644 f.sh` não remove o bit de execução, tornando a fixture `pin7-noexec` inconstruível.

**O que falta para medir:** VM Windows com Git Bash ou runner `windows-latest` com medição de `chmod 644` + verificação de `[ -x ]`. Nenhum commit encontrado abordando este cenário.

---

## #435 — `traceid_orphan_req` dispara para REQ em `backlog/` (estado correto sem roadmap)

**Veredito:** (B) — Reproduz integralmente  
**Gravidade:** MÉDIA — força uso do `.trackfw-baseline.json` como workaround; CI fica vermelho a cada nova REQ criada em backlog

**Reprodução executada:**

```bash
# Setup: projeto com trace_id_field configurado
$ cat trackfw.yaml | grep trace_id_field
trace_id_field: req_id

$ cat docs/req/backlog/REQ-2026-09-28-backlog.md | head -6
---
req_id: REQ-2026-09-28-backlog
status: backlog
roadmap: none
adr: ADR-001-test.md
---

$ ./bin/trackfw validate 2>&1 | grep traceid
✗ traceid_orphan_req: req "REQ-2026-09-28-backlog.md" has req_id="REQ-2026-09-28-backlog" but no Roadmap with same id
```

Diretório do temp: `/private/tmp/claude-501/.../scratchpad/issue435`

A implementação em `internal/validator/validator_traceid.go` itera sobre `reqIndex` sem filtrar por `e.state`. REQs em `backlog/` (cujo protocolo correto é não ter roadmap ainda) são indexadas e disparam a violação. O ADR-036 do consumidor declara explicitamente que roadmap só é criado quando a REQ sai de `backlog` para `wip`.

---

## #443 — Desde v8.0.1: 32 commits sem release; `internal/pathguard` não existe em versão publicada

**Veredito:** (A) — Não reproduz. v9.0.0 e v9.0.1 foram publicados; pathguard está em ambos.  
**Gravidade:** — (n/a)

**Reprodução executada:**

```
$ git tag --sort=-version:refname | sed -n '1,3p'
v9.0.1
v9.0.0
v8.0.1

$ git ls-tree -r v9.0.1 --name-only | grep -c pathguard
28

$ git ls-tree -r v9.0.0 --name-only | grep -c pathguard
27
```

A pergunta de planejamento foi respondida pelos releases v9.0.0 e v9.0.1. O `internal/pathguard` existe em ambas as tags. O `CHANGELOG.md` inclui as entradas `## [9.0.1]` e `## [9.0.0]`.

---

## #445 — `trackfw init` re-executado DESTRÓI `roadmap_dir` customizado

**Veredito:** (B) — Reproduz integralmente  
**Gravidade:** ALTA — destrói config customizada em silêncio; validate/status/serve passam a olhar para diretórios vazios e reportam estado errado

**Reprodução executada:**

```bash
# Diretório: /private/tmp/.../scratchpad/issue445
$ ./bin/trackfw init   # primeiro init
$ grep '^roadmap_dir' trackfw.yaml
roadmap_dir: docs/roadmaps

$ sed -i '' 's|^roadmap_dir: .*|roadmap_dir: governance/plans|' trackfw.yaml
$ grep '^roadmap_dir' trackfw.yaml
roadmap_dir: governance/plans

$ ./bin/trackfw init   # re-executado
  ✓ .claude/commands/trackfw (0 slash commands criados, 9 já existiam — não sobrescritos)
✓ trackfw initialized — run 'trackfw status' to see your governance state.

$ grep '^roadmap_dir' trackfw.yaml
roadmap_dir: docs/roadmaps          ← 🔴 DESTRUÍDO
```

`writeTrackfwConfig` (`internal/generators/scaffold.go`) chama `os.WriteFile("trackfw.yaml", …)` com o template fixo sem ler nem preservar valores existentes. Contraste com `generateGitAttributes`, que tem ramos criar/append/no-op. O defeito afeta todos os campos customizáveis, não só `roadmap_dir`.

---

## #450 — `trackfw context` reporta `ADRs (0)` quando ADRs estão em subpastas de `adr_dirs`

**Veredito:** (B) — Reproduz integralmente  
**Gravidade:** ALTA — `context` é o primeiro comando que agentes executam; zero ADRs quando existem 5 (ou 145) induz confiança na ausência de decisões arquiteturais

**Reprodução executada:**

```bash
# Config: adr_dirs: [docs/adr/zeus], ADRs em docs/adr/zeus/done/ e docs/adr/zeus/wip/
$ find docs/adr -name 'ADR-*.md' | wc -l
       5

$ ./bin/trackfw status | grep ADR
   ADRs        5       ← correto

$ ./bin/trackfw context | grep -A3 "^## ADR"
## ADRs (0)
- (none)              ← 🔴 zero
```

Diretório temp: `/private/tmp/claude-501/.../scratchpad/issue450`

`trackfw status` conta corretamente porque desce em subpastas. `trackfw context` procura ADRs apenas na raiz de cada diretório em `adr_dirs`, sem descer em subpastas de estado (done/, wip/). As duas implementações divergem. Conforme anotado no issue, o `Governance score` também é calculado a partir da leitura do `context`, logo é deflacionado sem motivo.

---

## #451 — v9.0.0 instala `trackfw-gate.yml` ao lado do `trackfw-validate.yml` existente

**Veredito:** (C) — Reproduz parcialmente; a coexistência dos dois arquivos é decidida (ADR-2026-08-28), o desperdício de 3x foi reduzido para 2x por #456  
> ⚠️ **RETRATAÇÃO 2026-09-29 (sítio 4 — ML-1C, ROADMAP-2026-09-22):** a afirmação *"a coexistência
> está decidida (ADR-2026-08-28)"* é **falsa**. Medido: aquela ADR tem zero ocorrências de
> `trackfw-validate.yml` — ela decide pino de versão e `TRACKFW_VERSION`. A decisão que agora existe
> é a `ADR-2026-09-29`, que resolve o contrário: o produto entrega **um** workflow por projeto e
> nunca instala um segundo ao lado do existente. O #451 foi absorvido na `REQ-2026-09-02` em
> 2026-09-29. O texto original é preservado acima como evidência de como a citação propagou.

**Gravidade:** MÉDIA — consumidor em plano free paga 2 execuções por push num PR (era 3)

**Reprodução executada:**

```bash
# Verificação via código-fonte e histórico, não por execução binária
# (o wizard sem TTY seleciona ci: none — não gera o gate.yml)

# PRs relevantes:
# 14bc5c9d (#456): mudou trigger de trackfw-validate.yml template de
#   on: [push, pull_request]  →  push: branches: [main] + pull_request
#   Reduzindo de 3x para 2x.
#
# b527bd91 (#459): alinhou .github/workflows/trackfw-validate.yml do repo
#   com o template corrigido no #456 (a cópia versionada tinha ficado).
#
# Sugestão 1 do issue (detectar workflow existente antes de instalar segundo)
# permanece aberta — declarada no commit b527bd91:
# "Continua aberto nesta REQ: a sugestão 1 da #451"
```

**O que fechou:** trigger `on: [push, pull_request]` gerava 2 execuções para 1 push numa branch com PR. Corrigido no template em #456 e na cópia versionada em #459. Resultado: 2x (uma de `trackfw-gate.yml`, uma de `trackfw-validate.yml`) em vez de 3x.

**O que sobrou:** coexistência dos dois workflows (decisão da ADR-2026-08-28, para cobrir dois métodos de instalação). A detecção de workflow pré-existente para evitar instalar um segundo não foi implementada.
> ⚠️ **RETRATAÇÃO 2026-09-29 (sítio 5 — ML-1C, ROADMAP-2026-09-22):** a afirmação *"decisão da
> ADR-2026-08-28, para cobrir dois métodos de instalação"* é **falsa**. Aquela ADR não decide
> coexistência. A `ADR-2026-09-29` decide o contrário: um workflow por projeto. A detecção de
> workflow pré-existente foi implementada no ML-1A desta mesma REQ. Texto original preservado acima.

---

## #460 — `Wave 0 gate is placeholder or absent` com prosa entre marcador e bloco bash

**Veredito:** (A) — Não reproduz. Corrigido em `c3ed28ba` (#462).  
**Gravidade:** — (n/a)

**Reprodução executada:**

```bash
# Diretório temp: /private/tmp/.../scratchpad/issue460
# Roadmap com prosa (blockquote) entre '**Gates da wave:**' e o bloco bash

$ ./bin/trackfw validate 2>&1 | grep -i "wave 0\|gate\|placeholder\|malform"
(no output)    ← 🔴 NÃO dispara

# Controle (sem prosa): também não dispara.
```

**Commit que corrigiu:** `c3ed28ba` "fix(roadmapdoc): o gate da Wave 0 aceita prosa, e a mensagem passa a dizer a causa (#462)".

`ParseGates` (`internal/roadmapdoc/roadmapdoc.go`) agora avança sobre prosa (blockquotes, parágrafos, linhas em branco) entre o marcador `**Gates da wave:**` e a cerca ` ```bash `, parando apenas no próximo heading. A busca é limitada ao heading seguinte para evitar capturar cercas de outras seções.

As mensagens também foram separadas em três causas distintas: `Wave0GatePlaceholder`, `Wave0GateAbsent`, `Wave0GateMalformed` — cada uma com instrução de correção específica.

---

## Tabela-resumo

| Issue | Título curto | Veredito | Gravidade |
|-------|-------------|---------|-----------|
| #308  | MSYS brace expansion destrói `{owner}/{repo}` | (D) Windows/MSYS | — |
| #364  | Sinal `[-1 resolvido]` do ratchet sem destino | (D) CI/Windows | — |
| #403  | Pesos pessimistas em 3 rótulos write-containment | (D) CI-only | — |
| #407  | `.trackfw-log` sem fuso/autor; ML status ilegível | (A) Corrigido | — |
| #408  | `TestCorpusMeasurement` sem `-v` no job `go` | (B) Reproduz | BAIXA |
| #421  | `chmod` ineficaz em NTFS `noacl` (pin7-noexec) | (D) Windows | — |
| #435  | `traceid_orphan_req` dispara para REQ em `backlog/` | (B) Reproduz | MÉDIA |
| #443  | 32 commits sem release; `pathguard` não publicado | (A) Corrigido | — |
| #445  | `trackfw init` destrói `roadmap_dir` customizado | (B) Reproduz | ALTA |
| #450  | `trackfw context` ADRs (0) com ADRs em subpastas | (B) Reproduz | ALTA |
| #451  | Dois workflows instalados; 3x → 2x por #456 | (C) Parcial | MÉDIA |
| #460  | Wave 0 gate com prosa entre marcador e bash | (A) Corrigido | — |

---

## Verificação `git diff trackfw.yaml`

```
(vazio — trackfw.yaml não foi modificado)
```
