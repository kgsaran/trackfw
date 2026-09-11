# Derivação ML-0A — `by_agent` req new / roadmap new

> Data: 2026-09-11 | Agente: apolo-tf | ML: ML-0A (Wave 0) | Zero linhas de implementação

---

## Item 1 — Sítio do `req new` do Go

**Evidência:**

O relator do #320 declarou não ter localizado o sítio. Localizado e confirmado:

- **`internal/generators/req.go:30-35`** — A função `NewREQ` contém o comentário que identifica o
  ponto único de escrita (linha 30) e a chamada real na linha 32:

  ```go
  // Ponto único de decisão de caminho de ESCRITA (ADR-2026-09-03, D2/D4): em by_agent grava no
  // canônico req_dir/<agente>/; em flat, req_dir/ — o mesmo ponto que alimenta a união de leitura.
  reqDir := validator.REQWriteDir(cfg)
  ```

- A **decisão** é delegada a `internal/validator/validator.go:1464` — função `REQWriteDir`:

  ```go
  func REQWriteDir(cfg config.ProjectConfig) string {
      reqDir := cfg.REQDir
      if cfg.RoadmapNamespacing == config.NamespacingByAgent {
          agent := "default"
          for _, a := range cfg.Agents {
              if a != "" { agent = a; break }
          }
          return filepath.Join(reqDir, agent)
      }
      return reqDir
  }
  ```

**Conclusão:** `internal/generators/req.go:32` é o sítio de escrita (uma linha); a lógica de
decisão de caminho está em `internal/validator/validator.go:1464`. O relator não estava errado em
não o ter localizado em `req.go` isoladamente — a lógica relevante está no validator.

---

## Item 2 — `by_agent` vale para `req_dir` ou só para `roadmap_dir`?

**Resposta: `by_agent` aplica-se a AMBOS — `req_dir` E `roadmap_dir`.**

Evidência nos 3 runtimes:

### Go — `internal/validator/validator.go:1456-1486`

Comentário do cabeçalho da função `REQWriteDir` (linha 1457-1459):
```
// REQWriteDir é o PONTO ÚNICO que decide ONDE uma REQ nova é gravada (ADR-2026-09-03, D2/D4):
//   - flat      → req_dir/
//   - by_agent  → req_dir/<agente>/
```
A checagem na linha 1469:
```go
if cfg.RoadmapNamespacing == config.NamespacingByAgent {
```
Retorna `filepath.Join(reqDir, agent)` — ou seja, `req_dir/<agente>/`.

O leitor `ResolveREQFiles` (linha 1507) também distingue by_agent para REQs (linha 1493):
```
//	req_dir/<agente>/*.md            CANÔNICO em by_agent
```

### Node — `npm/src/validator/index.js:417-431`

```js
function reqWriteDir(cfg) {
  const namespacing = cfg.roadmapNamespacing || cfg.roadmap_namespacing || ''
  if (namespacing === 'by_agent') {
    const agents = (cfg.agents || []).filter(a => a)
    const agent = agents.length > 0 ? agents[0] : 'default'
    return path.join(reqDir, agent)
  }
  return reqDir
}
```
Confirma: `by_agent` → `req_dir/<agente>/`.

### Python — `pypi/trackfw/validator.py:803-810`

```python
if cfg.get("roadmap_namespacing", "") == "by_agent":
    agents = [a for a in (cfg.get("agents") or []) if a]
    agent = agents[0] if agents else "default"
    return os.path.join(req_dir, agent)
```
Confirma: `by_agent` → `req_dir/<agente>/`.

**AC10 está corretamente posto.** `req new` deve obedecer a mesma regra de `--agent`/ambiguidade que
`roadmap new`. A Wave 1 não muda de forma.

**Nota sobre o defeito atual:** todas as 3 implementações ainda usam `agents[0]` (ou primeiro
não-vazio) sem expor `--agent`, sem detectar multi-agente e sem emitir erro de ambiguidade — que é
exatamente o que a Wave 1 corrige.

---

## Item 3 — Mecanismo de derivação de agente a partir de caminho (usado por `roadmap move`)

**Conclusão:** o mecanismo existe nos 3 runtimes, é INLINE em cada função `move_roadmap` /
`moveRoadmap` — NÃO existe como função nomeada separada em nenhum runtime.

### Go — `internal/generators/roadmap.go:436-442`

```go
if cfg.RoadmapNamespacing == config.NamespacingByAgent {
    // em by_agent: src = roadmapDir/agent/state/file → agentDir é a pasta avó
    agentDir := filepath.Dir(filepath.Dir(src))
    agent := filepath.Base(agentDir)
    fromState = filepath.Base(filepath.Dir(src))
    var ok bool
    targetDir, ok = agentStateDir(agent, state)
```

Derivação: `agent = filepath.Base(filepath.Dir(filepath.Dir(src)))`. Inline em `MoveRoadmap`.
Sem função auxiliar nomeada para extração de agente.

### Node — `npm/src/generators/roadmap.js:266-270`

```js
if (cfg.roadmapNamespacing === config.NAMESPACING_BY_AGENT) {
    const agentDir = path.dirname(path.dirname(src))
    const agent = path.basename(agentDir)
    fromState = path.basename(path.dirname(src))
    targetDir = agentStateDir(agent, state)
```

Derivação: `agent = path.basename(path.dirname(path.dirname(src)))`. Inline em `moveRoadmap`.

### Python — `pypi/trackfw/generators/roadmap.py:660-663`

```python
if cfg.get("roadmap_namespacing") == cfg_module.NAMESPACING_BY_AGENT:
    agent_dir = os.path.dirname(os.path.dirname(src))
    agent = os.path.basename(agent_dir)
    target_dir = _agent_state_dir(agent, to_state, cfg)
```

Derivação: `agent = os.path.basename(os.path.dirname(os.path.dirname(src)))`. Inline em `move_roadmap`.
**Equivalente Python EXISTE.**

### Implicação para Wave 1

A Wave 1 deve **reusar o padrão** (dois níveis de dirname + basename), NÃO extrair uma função nova.
No caso do `roadmap new --req <caminho>`, a derivação é feita a partir do caminho da REQ recebida:
`req_dir/<agente>/REQ-....md` → `agentDir = dirname(dirname(req_path))` → `agent = basename(agentDir)`.
Derivação nova reprova a auditoria (AC11).

---

## Item 4 — Lista re-derivada de emissores de `trackfw req new` / `roadmap new` sem `--agent`

**Comando usado:**

```bash
git ls-files | xargs grep -a -n "trackfw req new\|trackfw roadmap new" 2>/dev/null \
  | grep -v "_test\.\|\.md:\|\.yaml:\|\.sh:\|\.json:\|demo.tape\|CHANGELOG\|\.txt:"
```

Nota: `npm/src/validator/index.js` é classificado como binário por `file`; o `-a` é obrigatório
(vault: `serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29.md`).

### Emissores confirmados da lista de 2026-09-11 (posições verificadas)

| Arquivo | Linha(s) | Emite |
|---------|----------|-------|
| `internal/generators/agentfiles.go` | 59 | tabela de instruções para o agente |
| `internal/generators/claudemd.go` | 57-58 | tabela de comandos no CLAUDE.md gerado |
| `internal/generators/scaffold.go` | 263 | slash-command `req.md` |
| `npm/src/generators/init.js` | 524, 691-692, 899 | os mesmos 3 conteúdos no Node |
| `npm/src/push/runner.js` | 130 | saída de remediação do `push` |
| `npm/src/ship/runner.js` | 502 | saída de remediação do `ship` |
| `npm/src/commands/branch.js` | 33-34 | remediação do `branch` |
| `npm/src/commands/commit.js` | 37-38 | remediação do `commit` |
| `pypi/trackfw/push/runner.py` | 148 | remediação do `push` |
| `pypi/trackfw/validator.py` | 1967-1968 | mensagem da regra `branch_has_wip_roadmap` |

### Emissores NOVOS (não estavam na lista de 2026-09-11)

| Arquivo | Linha(s) | Emite |
|---------|----------|-------|
| `internal/commands/branch.go` | 84-85 | remediação do `branch` Go |
| `internal/commands/commit.go` | 87-88 | remediação do `commit` Go |
| `internal/commands/push.go` | 159-160 | remediação do `push` Go |
| `internal/commands/ship.go` | 305-306 | remediação do `ship` Go |
| `internal/validator/validator.go` | 2844 | mensagem `branch_has_wip_roadmap` Go |
| `npm/src/validator/index.js` | 1508 | mensagem `branch_has_wip_roadmap` Node |
| `pypi/trackfw/commands/branch.py` | 488-489 | remediação do `branch` Python |
| `pypi/trackfw/commands/commit.py` | 89-90 | remediação do `commit` Python |
| `pypi/trackfw/ship/runner.py` | 512-513 | remediação do `ship` Python |
| `pypi/trackfw/generators/init_gen.py` | 263, 409-410, 674 | os mesmos 3 conteúdos no Python |

**ML-3A deve usar esta lista re-derivada, não a de 2026-09-11.**

---

## Item 5 — Threat model + contra-braços

### 5(a) Quem esvazia este ML-0A sem quebrar nenhuma regra escrita?

**Vetor A — Resposta incorreta no item 2 com aparência de evidência.**
Um agente que responda item 2 como "by_agent não se aplica a req_dir, só a roadmap_dir" mas cite
linhas de código corretas (que, lidas rapidamente, não falsificam a afirmação) consegue remover
AC10 da Wave 1 sem violar a regra "evidência obrigatória" — pois cita arquivo:linha. O defeito
permanece em req new para sempre. Contra-braço: o auditor deve LER as linhas citadas, não só
verificar que foram citadas.

**Vetor B — Declaração técnica correta, mas incompleta na lista de emissores (item 4).**
Um agente que omita emissores novos (ex: `internal/commands/push.go:159-160`) da lista re-derivada
não viola nenhuma regra escrita — o gate do ML-0A verifica presença do arquivo, não completude da
lista. O ML-3A então "conclui" sem cobrir esses emissores, que ficam ensinando o comando errado.

**Vetor C — Gate de ML-0A substituído por placeholder permanente.**
Se o gate real (item 5c abaixo) não substituir o placeholder do roadmap antes do commit, a Wave 0
fica com `test -f docs/qualidade/...` como gate, que não verifica substância.

### 5(b) Contra-braços por AC

| AC | Regride para um lado | Regride para o outro lado |
|----|---------------------|--------------------------|
| **AC4** | `--agent` não atualiza frontmatter → pasta e frontmatter divergem; validator perde rastreio de dono | `--agent` não muda o path → arquivo vai para agents[0], frontmatter diz beta; divergência silenciosa |
| **AC5** | Erro de ambiguidade dispara com UM agente → todos os projetos single-agent by_agent quebram no req new/roadmap new | Silêncio com múltiplos agentes → `agents[0]` em silêncio, exatamente o bug atual do cmdb |
| **AC10** | Mesmo que AC4 lado 1, mas para req new | Mesmo que AC5 lado 2, mas para req new — req nasce no namespace errado |
| **AC11** | `roadmap new --req beta/REQ.md` gera roadmap em alpha/ → REQ e roadmap em namespaces diferentes | `roadmap new` deriva agente com lógica nova em vez da do `move` → dois caminhos de derivação que podem divergir; auditoria reprova por design |
| **AC12** | `--agent` some de Go/Node req new → Go e Node voltam ao estado atual quebrado | `--agent` some de Python roadmap new → regressão em runtime que hoje funciona |
| **AC14** | flat passa a criar chave `agents:` no trackfw.yaml → projetos flat corrompidos | "1 agente sem flag → usa aquele" vira "1 agente sem flag → erro" → atrito desnecessário em 100% dos projetos single-agent |

### 5(c) Gate concreto (substitui o placeholder do roadmap)

```bash
f="docs/qualidade/2026-09-11-derivacao-by-agent-ml0a.md"
test -f "$f"                                                         || { echo "FALTANDO: $f" >&2; exit 1; }
grep -q "req.go:"          "$f"                                      || { echo "FALTA: sitio req.go (item 1)" >&2; exit 1; }
grep -q "req_dir"          "$f"                                      || { echo "FALTA: by_agent/req_dir (item 2)" >&2; exit 1; }
grep -qE "(roadmap\.(go|js|py)):[0-9]" "$f"                         || { echo "FALTA: assinatura mecanismo move (item 3)" >&2; exit 1; }
grep -q "agentfiles.go"    "$f"                                      || { echo "FALTA: emissores — agentfiles.go (item 4)" >&2; exit 1; }
grep -q "init_gen.py"      "$f"                                      || { echo "FALTA: emissores novos — init_gen.py (item 4)" >&2; exit 1; }
grep -qi "threat\|esvazia\|contra.bra" "$f"                         || { echo "FALTA: threat model (item 5)" >&2; exit 1; }
echo "ML-0A gate: OK"
```

---

## Confirmação de passagem da dependência

O roadmap irmão está em
`docs/roadmaps/done/ROADMAP-2026-08-29-lista-de-agentes-complementa-o-disco-e-namespace-nao-declarado-vira-violacao.md`
— Done. A função `resolveAgentNamespaces` (união leitura+disco) existe nos 3 CLIs e a regra
`agent_namespace_undeclared` foi entregue. Dependência satisfeita.

---

## O que não foi possível derivar

Nada bloqueante. Todos os 5 itens têm evidência verificada por leitura direta de arquivo:linha.

---

## Correção da auditoria (Zeus, 2026-09-11) — item 4 mediu 20, a régua diz 33

O filtro do ML-0A usou `grep -v "_test\."`. **Ele não casa com os testes deste repositório**, que são
`npm/tests/*.test.js` e `pypi/tests/test_*.py`. Re-medido por **arquivo** (não por linha):

```bash
git ls-files | xargs grep -a -l "trackfw req new\|trackfw roadmap new" 2>/dev/null \
 | grep -vE "_test\.|\.md$|\.yaml$|\.sh$|\.json$|demo\.tape|CHANGELOG|\.txt$" | sort
```

→ **33 arquivos.** Os 6 que o filtro escondeu:

```
npm/tests/push.test.js · npm/tests/serve_chain.test.js · npm/tests/ship.test.js
pypi/tests/test_push.py · pypi/tests/test_serve_chain.py · pypi/tests/test_ship.py
```

🔴 **Não são ruído — são acoplamento.** Esses testes **afirmam o texto** que o ML-3A vai mudar. Se o
ML-3A alterar os emissores sem eles, a suíte quebra; se alterar os dois sem perceber, o teste deixa de
provar o que dizia provar. **Entram no mesmo ML, em lockstep.**

### Classificação obrigatória antes do ML-3A

Os 33 não são todos emissores de orientação. Ao menos `internal/serve/api_chain.go`,
`npm/src/serve/api_chain.js` e `pypi/trackfw/serve/api_chain.py` citam o comando em **comentário**, não
em texto exibido ao usuário. **O ML-3A classifica os 33 em {orientação exibida · teste que a afirma ·
comentário} e justifica por escrito cada exclusão** — contagem sozinha não autoriza excluir nada.

### A lição que vale além deste ML

O ML-0A **mediu**, escreveu o comando e mostrou a saída — foi isso que permitiu a auditoria pegar o
erro em minutos. O defeito não estava na medição, estava na **régua**: um `grep -v` cuja convenção de
nome não é a deste repositório. **Régua errada produz número confiante e falso** — a mesma classe que
já nos custou 4 contagens nesta campanha.

## Nota para a Wave 1 — item 3

A derivação do agente a partir do caminho é **inline e triplicada** (`dirname(dirname(src))` +
`basename`), sem função nomeada em nenhum runtime. 🔴 **"Reusar o mecanismo do `move`" (AC11) significa
EXTRAIR para uma função nomeada por runtime e chamá-la nos dois sítios** — não acrescentar uma quarta
cópia inline. Quarta cópia reprova a auditoria: seria exatamente a divergência que o AC11 existe para
impedir.

## Nota para a Wave 1 — item 1 e 2, confirmados por Zeus

O ponto único de escrita **já existe nos 3 runtimes** e já trata `by_agent` para `req_dir`:

```
Go      internal/validator/validator.go:1464  REQWriteDir   (consumido por generators/req.go:32)
Node    npm/src/validator/index.js:421        reqWriteDir
Python  pypi/trackfw/validator.py:~802        (equivalente)
```

Os três já **filtram nomes vazios** antes de escolher — correção do `hades-tf` em 2026-09-03,
documentada no próprio código do Go. **A Wave 1 altera essas três funções e a plumbing da flag; não
cria caminho novo de escrita.** AC10 está bem posto.
