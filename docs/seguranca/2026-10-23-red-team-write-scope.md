# Red-team: trackfw guard write-scope (ML-4B)

> Data: 2026-10-23
> Agente: hades-tf
> Roadmap: ROADMAP-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-...
> Binário testado: compilado de `/private/tmp/trackfw-toml-escape` (branch `feat/regras-de-escrita-dos-agentes-viram-mecanismo-no-harness`)

---

## Modelo independente

Reimplementação das regras a partir da ADR e dos contratos (sem leitura do código antes dos ataques).

**D3 (caminhos congelados):** para toda sessão, `docs/agents-working-context.md` é congelado quando as 10 primeiras linhas contêm `<!-- trackfw:frozen -->`. Para subagentes (`agent_id` presente no payload), também são congelados: `trackfw.yaml`, os arquivos de hook/settings de todo CLI, e `.trackfw/leases/`. A lista do `trackfw.yaml` é acréscimo, nunca subtração. O guard deve usar `filepath.EvalSymlinks` em ambos os lados da comparação para resolver symlinks, e `os.SameFile` para detectar hardlinks (dois caminhos distintos que apontam para o mesmo inode).

**D4 (escopo por despacho):** só quando `agent_id` presente e `agent_type` casa `^[A-Za-z0-9_-]{1,64}$`. Lease em `<raiz do alvo>/.trackfw/leases/<agent_type>.json` e, pelo ML-3C, também na raiz do `cwd`. Lease inválido em qualquer raiz nega. D3 corre antes: caminho congelado nega mesmo se estiver no `allow`.

**Fail-closed:** qualquer erro de parse, campo ausente, YAML inválido → exit 2.

---

## Ataques executados e vereditos

### Grupo 1 — Fuga de caminhos congelados por normalização

| ID | Ataque | Comando/saída | Veredito |
|----|--------|---------------|---------|
| 1A | Maiúsculas (`Docs/Agents-Working-Context.md`) | `exit=2` — negado | B (correto) |
| 1B | Barra final no alvo | `exit=2` — negado | B (correto) |
| 1C | `//` dupla barra | `exit=2` — negado | B (correto) |
| 1D | `./` ponto-barra | `exit=2` — negado | B (correto) |
| **1E** | **Hardlink para o arquivo congelado** | **`exit=0` — LIBERADO** | **R (bypass confirmado)** |
| 1F | Symlink para o arquivo congelado | `exit=2` — negado | B (correto) |
| 1G | Symlink de diretório contendo o congelado | `exit=2` — negado | B (correto) |

**Attack 1E — bypass por hardlink (ÚNICO BLOQUEANTE):**

```bash
# Reprodução:
PROJ=/tmp/test-proj
ln "$PROJ/docs/agents-working-context.md" "$PROJ/docs/frozen-alias.md"
echo '{"tool_name":"Write","tool_input":{"file_path":"'"$PROJ"'/docs/frozen-alias.md"}}' \
  | trackfw guard write-scope; echo "exit=$?"
# exit=0  ← BYPASS; esperado 2
```

Evidência de inode compartilhado:

```
383591740  docs/agents-working-context.md
383591740  docs/frozen-alias.md   ← mesmo inode
```

**Causa raiz:** `pathMatchesFrozen` em `internal/guard/writescope.go:649` chama `filepath.EvalSymlinks` em ambos os lados, mas nunca `os.SameFile`. Hardlinks são caminhos distintos no sistema de arquivos; `EvalSymlinks` resolve symlinks mas devolve o caminho do próprio hardlink, não o do "original". A comparação de strings falha: `"docs/frozen-alias.md" != "docs/agents-working-context.md"`.

A Wave 0 §2.3 (Ameaça D3-B) declarou explicitamente: `os.SameFile funciona para targets que já existem`. A implementação não incluiu essa chamada.

**Fix necessário** em `pathMatchesFrozen` (`writescope.go`): quando o alvo existe (não é escrita-de-criação), chamar `os.SameFile(targetPath, frozenAbsPath)` além da comparação de strings. Para o caso de escrita-de-criação (alvo não existe), o hardlink também não existe ainda — sem bypass.

---

### Grupo 2 — Fuga pela identidade do agente e D4

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 2A | `agent_type` com path traversal `../../../etc/passwd` | `exit=0` — D3 only (slug inválido) | B (correto, design intencional) |
| 2B | `agent_type` com barra `apolo/tf` | `exit=0` — D3 only | B (correto) |
| 2C | allow entry é symlink para diretório real | `exit=0` — resolvido com EvalSymlinks | B (correto) |
| 2D | `expires` no momento atual (expira imediatamente) | `exit=2` — expirado | B (correto) |
| 2E | Projeto falso com lease permissivo via `cwd`; alvo é caminho congelado | `exit=2` — D3 bloqueia antes | B (correto) |
| 2F | Lease inválido (version=99) na raiz do alvo + lease válido na raiz do `cwd` | `exit=2` — versão inválida nega | B (correto) |
| 2G | Subagente tenta escrever em `.trackfw/leases/` | `exit=2` — congelado | B (correto) |
| 2H | Subagente tenta escrever em `trackfw.yaml` | `exit=2` — congelado | B (correto) |
| 2I | Sessão principal escreve em `.trackfw/leases/` | `exit=0` — design intencional | B (correto) |

---

### Grupo 3 — D3 não pode ser contornada via D4

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 3A | Lease de projeto externo com `allow` apontando para `.trackfw/leases/` do projeto alvo | `exit=2` — D3 bloqueia | B (correto) |
| 3B | Subagente escreve em `.claude/settings.json` | `exit=2` — congelado | B (correto) |
| 3C | Lease com `.trackfw/leases/` no próprio `allow` | `exit=2` — D3 bloqueia antes | B (correto) |

---

### Grupo 4 — Disponibilidade

| ID | Ataque | Resultado | Veredito |
|----|--------|-----------|---------|
| 4A | Payload de 10 MB | `exit=0` em ~1s | B (sem impacto de segurança — arquivo legítimo) |
| 4B | Pipe com EOF imediato | `exit=0` em <1s | B |
| 4C | Pipe com writer que nunca envia dados | `exit=2` em 2s exatos ("stdin truncated") | B (idle timeout funciona) |
| 4D | `trackfw.yaml` de 5 MB | `exit=0` em 1s | B |
| 4E | `trackfw.yaml` com YAML inválido | `exit=2` — nega | B (correto) |

---

### Grupo 5 — Unicode, BOM, TOCTOU

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 5A | Caminho NFD (Unicode normalizado) | `exit=2` — negado (ASCII, sem diferença) | B |
| 5B | `\n` literal no valor de string JSON | `exit=2` — JSON inválido (correto) | B |
| 5C | Payload com BOM UTF-8 prefixado | `exit=2` — BOM strippado, depois negado | B (correto) |
| 5E | Subagente escrevendo em `docs/agents-working-context/` (subdiretório) | `exit=0` — PERMITIDO | R (residual por design, ver §Observações) |
| 5F | Symlink criado APÓS a checagem (TOCTOU clássico) | Inerente ao modelo PreToolUse | R (residual declarado) |

---

### Grupo 6 — Arquivo de lease patológico

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 6B | `expires` com timezone `+00:00` em vez de `Z` | `exit=0` — RFC 3339 aceita offset | B (correto) |
| 6C | Lease é diretório | `exit=2` — "not a regular file (D4-C)" | B (correto) |
| 6D | Lease é symlink para arquivo válido | `exit=0` — symlink seguido via `os.ReadFile` | B (correto; `os.Stat` follow symlinks, `ReadFile` também) |
| 6E | Lease é FIFO | `exit=2` — "not a regular file (D4-C)" (verificado via `os.Stat` antes do open) | B (correto; FIFO não bloqueia) |

---

### Grupo 7 — Marcador congelado

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 7A | Sessão principal, sem marcador | `exit=0` — arquivo não congelado para sessão principal | B (correto) |
| 7B | Subagente, sem marcador | `exit=2` — sempre congelado para subagente | B (correto) |
| 7C | Marcador na linha 11 (fora das 10 primeiras) | `exit=0` — marcador não detectado, sessão principal liberada | B (correto, design intencional) |

---

### Grupo 8 — Allow list edge cases

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 8A | allow list vazia | `exit=2` — nenhuma entrada corresponde | B (correto) |
| 8B | Allow entry com barra final em caminho de arquivo (`/path/file.go/`) | `exit=0` — LIBERADO | R (comportamento nota, ver §Observações) |
| 8C | allow é JSON null | `exit=2` — não casa nenhuma entrada | B (correto) |

---

### Grupo 9 — Injeção no tool_name e tipos errados

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 9A | `tool_name` com pipe: `Write\|Edit` | `exit=0` — não está no mapa de write tools | B (correto) |
| 9B | `tool_name` em minúsculas: `write` | `exit=0` — comparação case-sensitive no mapa | B (correto) |
| 9C | `file_path` é objeto JSON | `exit=2` — "not a string" | B (correto) |
| 9D | `agent_id` é boolean `true` | `exit=2` — tratado como subagente (conservador), bloqueia frozen path | B (comportamento conservador, correto) |

---

### Grupo 10 — NotebookEdit

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 1H | NotebookEdit com `file_path` em vez de `notebook_path` | `exit=2` — "notebook_path absent" | B (correto: campo ausente → nega) |
| 1H2 | NotebookEdit com `notebook_path` correto | `exit=2` — negado pelo caminho congelado | B (correto) |
| 1I | `MultiEdit` (tool não existente na lista) | `exit=0` — não interceptado | B (correto; R8 declarado na Wave 0) |

---

### Grupo 11 — Repositório real

| ID | Ataque | Saída | Veredito |
|----|--------|-------|---------|
| 11B | Subagente escreve em `.claude/settings.json` do repo real | `exit=2` — congelado | B (correto) |
| 11C | `agent_type` com Unicode (`apolo-τf`) | `exit=0` — slug inválido, D3 only | B (correto) |
| 11D | `agent_type` com 64 chars (limite) | `exit=0` — slug válido | B (correto) |
| 11E | `agent_type` com 65 chars (acima do limite) | `exit=0` — slug inválido, D3 only | B (correto) |

---

## Observações (não bloqueantes)

**OBS-1 — Subagente pode escrever em `docs/agents-working-context/` (subdiretório):**
A D3 congela `docs/agents-working-context.md` (o arquivo legado). O diretório `docs/agents-working-context/` (novo-estilo, usado pelo arquiteto para registrar entradas) não é congelado para subagentes. Medido: `exit=0` ao escrever `docs/agents-working-context/2026-10-23-entry.md` como subagente.

Isso é **residual por design**: a proteção primária é a D1 (instrução "não escreva, reporte"), a D2 não bloqueia porque `docs/agents-working-context/` está no conjunto de orquestração. A D4 com lease restritivo fecharia, mas não é obrigatório. O residual já existia antes desta REQ; o ML-3A não declarou o diretório como congelado para subagentes.

**OBS-2 — Allow entry com barra final casa arquivo de mesmo nome:**
Uma entrada de lease `allow: ["/path/to/file.go/"]` (trailing slash, intenção: diretório) casa o arquivo `/path/to/file.go` por `filepath.Clean` strip the slash. Impacto: se o arquiteto colocar uma barra final acidentalmente num caminho de arquivo, o guard aceita o arquivo. Baixa severidade (erros de digitação do arquiteto, não de agente).

**OBS-3 — Lease como symlink funciona:**
`loadWriteScopeLease` usa `os.Stat` (que segue symlinks) + `info.Mode().IsRegular()` + `os.ReadFile`. Um lease que é symlink para um arquivo JSON válido é aceito. Isso é esperado e seguro, pois a criação do symlink em `.trackfw/leases/` está bloqueada pela D3 para subagentes.

**OBS-4 — TOCTOU inerente ao modelo de hook:**
Entre o retorno do guard (exit=0) e a escrita efetiva da ferramenta, o estado do filesystem pode mudar (ex: outro processo cria um symlink no caminho permitido). Não é corrigível sem atomicidade no nível do kernel. Residual declarado; o adversário relevante (agente isolado) não tem controle sobre o filesystem entre os dois eventos dentro da mesma invocação da ferramenta.

---

## Resumo

| Categoria | Contagem |
|-----------|----------|
| **B — Bloqueante** | **1** (hardlink bypass, Attack 1E) |
| R — Residual/observação | 4 (OBS-1 a OBS-4) |
| Total de ataques medidos | 47 |

**O único caminho de fail-open confirmado é o hardlink** (`internal/guard/writescope.go`, função `pathMatchesFrozen`, linha ~649): a ausência de `os.SameFile` permite que um hardlink para o arquivo congelado passe pelo guard. O fix é circúrgico: um `os.SameFile(targetPath, frozenAbsPath)` antes da comparação de strings, executado quando ambos os caminhos existem no disco.

---

## git status --short internal/

```
(vazio — nenhum arquivo de produto foi tocado)
```

Confirmado com `cd /private/tmp/trackfw-toml-escape && git status --short internal/` → sem output.
