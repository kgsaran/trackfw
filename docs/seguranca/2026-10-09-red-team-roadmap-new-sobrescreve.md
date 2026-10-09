# ML-6C Red-Team: `roadmap new` — Proteção contra Sobrescrita

> Data: 2026-10-09 | REQ: REQ-2026-09-09 (AC8) | Branch: `fix/req-nasce-orfa-roadmap-new-sobrescreve`
> Commit auditado: `b8b3c4e0` ("fix(roadmap): roadmap new não sobrescreve roadmap existente…")
> Binário: compilado do HEAD da branch; comparação pontual com main quando relevante.

---

## Superfícies atacadas

1. `findRoadmapByBasename`: `by_agent` com agente diferente, `roadmap_dir` customizado, diretórios de estado ausentes, symlink, case-insensitive FS (macOS).
2. Reparo de vínculo: roadmap existente com `req: ""`, roadmap existente pertencente a outra REQ.
3. `--force`: apaga `req:` na sobrescrita; `--force` sobre wip.
4. `--from-req` e caminho do wizard.
5. Texto de protocolo: ausência de `roadmap new` no passo 1 gerado.
6. Exit code 0 em "já existe" — algum script/gate interpreta como "criada"?

---

## Achados

### A1 — `by_agent`, `--agent` diferente cria roadmap órfã em namespace errado (BAIXO)

**Superfície:** `findRoadmapByBasename` pesquisa apenas o namespace do agente resolvido. Em modo `by_agent`, `roadmap new --agent hades-tf T` quando `req new --agent zeus-tf T` já criou o roadmap não detecta a colisão: os namespaces são distintos.

**Reprodução:**
```
projeto by_agent com agentes: [zeus-tf, hades-tf]
req new --agent zeus-tf "Cross Agent Feature"
  → docs/roadmaps/zeus-tf/backlog/ROADMAP-...-cross-agent-feature.md  req: "docs/req/zeus-tf/REQ-..."
roadmap new --agent hades-tf "Cross Agent Feature"
  → ✓ created docs/roadmaps/hades-tf/backlog/ROADMAP-...-cross-agent-feature.md  req: ""
```

**Antes/depois:** antes do commit, o resultado era sobrescrita silenciosa. Depois do commit, o resultado é criação de uma roadmap órfã em namespace diferente — sem sobrescrita do arquivo original (sem perda de dados do arquivo zeus-tf). A roadmap hades-tf fica com `req: ""` e sem vínculo.

**Severidade: BAIXO.** Requer que o operador especifique explicitamente `--agent` errado; o arquivo original é preservado; `validate` detecta a roadmap órfã pela ausência de `req:`; nenhuma REQ perde seu vínculo. O defeito é "criação de lixo" num caso de uso improvável, não perda de dados.

**Observação:** o comportamento anterior (antes do AC8) sobre esta superfície era idêntico — `findRoadmapByBasename` é novo no commit auditado. O defeito pré-existia e o AC8 não o fecha, mas também não o agrava.

---

### A2 — `linkREQToRoadmap` não repara o lado roadmap→REQ (BAIXO)

**Superfície:** Opção B (idempotente) chama `linkREQToRoadmap(content.REQPath, …)`. Quando `roadmap new T` é chamado sem `--req`, `content.REQPath = ""` → `linkREQToRoadmap` retorna imediatamente (primeira linha: `if reqPath == "" { return }`). O `req: ""` no roadmap existente nunca é reparado.

**Reprodução:**
```
roadmap existente com req: "" (criado por versão antiga do trackfw)
roadmap new "Orphan Repair"
  → prints "já existe… nada sobrescrito"
  → req: "" permanece intacto no roadmap
  → REQ.roadmap: atualizado SÓ SE --req for passado
```

**Antes/depois:** a mensagem afirma que "nada foi sobrescrito" e que o vínculo foi verificado/reparado. Na prática, o reparo do vínculo roadmap→REQ não ocorre sem `--req` explícito. O defeito original (sobrescrita) está fechado; mas a narrativa de "reparo bidirecional" não é verdadeira para o caminho sem `--req`.

**Severidade: BAIXO.** O `req: ""` persistente no roadmap é detectado por `validate` (`wip_has_req`) quando o roadmap for movido para `wip/`. Nenhuma perda de dados ocorre. O reparo é possível com `--force --req <REQ>` ou com edição manual.

---

### A3 — `--force` sem `--req` destrói o vínculo `req:` (BAIXO, documentado como esperado)

**Superfície:** `roadmap new --force T` sem `--req` sobrescreve o roadmap com template limpo (`req: ""`).

**Reprodução:**
```
req new "Alpha Feature"
  → ROADMAP-...-alpha-feature.md  req: "docs/req/REQ-...-alpha-feature.md"
roadmap new --force "Alpha Feature"
  → ✓ created  (sobrescreve)
  → req: ""  (vínculo destruído)
```

**Antes/depois:** o commit **declara** `--force` como opt-in de sobrescrita. O comportamento é conforme especificado. A mensagem de `--force` em `--help` não menciona que o vínculo `req:` será perdido.

**Severidade: BAIXO.** Opt-in explícito. Pode surpreender um agente que usa `--force` esperando apenas atualizar o conteúdo sem perder o link. Mitigação: `--force --req <REQ>` preserva o vínculo.

---

### A4 — `roadmap new --req REQ-B T` vincula REQ-B a roadmap de REQ-A (BAIXO)

**Superfície:** `linkREQToRoadmap` preenche `roadmap:` em REQ-B se estiver vazio, mesmo que a roadmap já pertença a REQ-A.

**Reprodução:**
```
REQ-A existe, vinculada a ROADMAP-...-shared-title-feature.md  (roadmap.req = REQ-A)
REQ-B existe, roadmap: ""
roadmap new --req REQ-B "Shared Title Feature"
  → roadmap ROADMAP-... já existe … nada sobrescrito
  → ✓ linked REQ-B → ROADMAP-...
  → ROADMAP.req: "REQ-A"  (inalterado)
  → REQ-B.roadmap: "ROADMAP-...-shared-title-feature.md"  ← NOVO
  → REQ-A.roadmap: inalterado
```

**Resultado:** duas REQs apontam para o mesmo roadmap. O roadmap aponta para REQ-A. `validate` detecta `req_roadmap_sync` (divergência em REQ-B: REQ-B.roadmap != roadmap.req).

**Antes/depois:** antes do commit, `roadmap new` sobrescrevia o roadmap, destruindo `req: REQ-A`. Pior. Depois: o roadmap é preservado, mas REQ-B ganha um ponteiro falso. O validate captura.

**Severidade: BAIXO.** Requer `--req` explícito apontando para REQ diferente com título igual; detectado pelo validate; nenhum dado destruído.

---

## Superfícies limpas (sem achado)

| Superfície | Vetor testado | Resultado |
|---|---|---|
| `by_agent` mesmo agente | `roadmap new --agent zeus-tf T` quando zeus-tf já tem o roadmap | Detecta, skip correto |
| `roadmap_dir` customizado | `roadmap_dir: my-roadmaps` | Detecção e skip corretos |
| Diretórios de estado ausentes | Apenas `backlog/` existe | `os.Stat` retorna erro para wip/done/etc, cai normalmente na escrita |
| Symlink folha em backlog/ | Symlink apontando para arquivo externo | `os.Stat` detecta (segue link), skip — target não é modificado |
| Case-insensitive FS (macOS) | Arquivo em disco com case misto (`Case-Feature`) vs slug (`case-feature`) | `os.Stat` detecta via FS case-insensitive, skip correto |
| `--force` sobre roadmap em wip/ | `roadmap new --force T` com roadmap em wip/ | Recusa com `exit 1` e mensagem clara |
| `--from-req` sobre roadmap existente | `roadmap new --from-req REQ` | Mesmo caminho (`NewRoadmapFromContent`), mesma proteção |
| Caminho do wizard | TTY=false, argumento posicional | `NewRoadmapFromContent` com `Force: forceFlag` — mesma proteção |
| Texto de protocolo nos arquivos gerados | `agentfiles.go`, `claudemd.go`, CLAUDE.md, README.md | `roadmap new` removido do passo 1; texto correto (`req new` → `roadmap move` → `branch new`) |
| Exit code 0 / Scenario 13 do check-barrier.sh | Script conta arquivos após `roadmap new` em projeto NOVO | Projeto novo = sem colisão, arquivo criado normalmente; vacuidade passa |

---

## Testes verificados

| Teste | Afirma | Passa |
|---|---|---|
| `TestRoadmapNew_SkipsWhenExistsInBacklog` | `req:` permanece após skip | SIM |
| `TestRoadmapNew_CreatesWhenNoExistingFile` | criação normal sem colisão | SIM |
| `TestRoadmapNew_SkipsWhenExistsInWip` | backlog/ vazio quando existe em wip/ | SIM |
| `TestRoadmapNew_ForceOverwritesSamePath` | `--force` sobrescreve com `req: ""` | SIM |
| `TestRoadmapNew_ForceRefusedWhenExistingInWip` | `--force` sobre wip/ retorna erro | SIM |
| `TestTrackfwRulesBlock_ByAgent2plus_Step1Block` | passo 1 não contém `roadmap new` | SIM |
| `TestInjectOrUpdateRules_ByAgent2plus_Step1Present` | step-1 contém só `req new` | SIM |

Todos os 21 testes relevantes: PASS. Nenhum FAIL.

---

## Veredito

**APROVA COM RESSALVAS.**

O defeito central (sobrescrita silenciosa via `os.WriteFile` sem checagem de existência, `roadmap.go:300`) está fechado. A proteção `findRoadmapByBasename` + `O_EXCL` funciona corretamente nos vetores testados: flat, `by_agent` mesmo agente, `roadmap_dir` customizado, symlink folha, case-insensitive FS, `--from-req`, wizard. O texto de protocolo gerado foi corrigido em todos os sítios enumerados no ML-6A.

As quatro ressalvas (A1–A4) são de severidade BAIXO, requerem uso incomum ou explicitamente errado (`--agent` errado, `--force` sem `--req`, `--req` apontando para REQ concorrente), e todas são detectadas pelo `validate` antes que o trabalho avance para PR. Nenhuma delas reproduz a perda de dados do defeito original.

**Residual aceito:** A1 (by_agent cross-namespace) é o único caso em que `findRoadmapByBasename` não detecta — por design da função, que não varre outros namespaces. Para fechar completamente, precisaria de scan global de todos os namespaces antes da criação, com custo de ambiguidade nova (mesma roadmap aparece em dois agentes: qual é a "existente"?). O `validate` é a barreira correta para este residual.
