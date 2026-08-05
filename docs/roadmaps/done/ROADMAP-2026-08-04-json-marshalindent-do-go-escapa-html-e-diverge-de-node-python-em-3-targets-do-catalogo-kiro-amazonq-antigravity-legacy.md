---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md"
squad: "apolo-tf"
---

# Roadmap: json.MarshalIndent do Go escapa HTML e diverge de Node/Python em 3 targets do catalogo (kiro amazonq antigravity-legacy)

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-em-3-targets-do-catalogo-kiro-amazonq-antigravity-legacy.md`

`internal/integrations/render.go:57` usa `json.MarshalIndent` puro, que faz HTML-escaping por
padrão — Node.js (`JSON.stringify`) e Python (`json.dumps`) não escapam. Isso diverge nos 3 targets
que usam a representação `agent-json`/`cli-agent-json` (kiro/cli, amazonq/cli,
antigravity/legacy-cli) sempre que o conteúdo do prompt contém `<`, `>` ou `&` — hoje exposto pelo
placeholder literal `<slug>` no texto do "Dispatch contract" do Architect. Fix pontual +
auditoria dos demais pontos de serialização JSON do Go que têm contrato de paridade byte-a-byte com
Node/Python.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `render.go:57` corrigido, `check-identity-parity.sh` com 0 falhas
- [x] Teste de regressão com caractere `<`/`>`/`&` no conteúdo de origem
- [x] Auditoria dos demais `json.Marshal*` com contrato de paridade cross-runtime concluída — nenhum
      fix necessário (todos os gates existentes reparseiam JSON antes de comparar, o que mascara
      divergência de escaping; `context.go:185` é risco real mas sem gate, registrado como candidato
      a REQ futura em vez de fix especulativo)
- [x] `make quality` verde (100/100 cenários de falsificação, incluindo `check-identity-parity.sh` já corrigido), `trackfw validate` sem violações

## Wave 1 — Fix pontual + regressão
> Dependencies: none

### ML-1A — Corrigir render.go:57 e adicionar teste de regressão
**Status:** ✅ Concluído
**Files affected:**
- `internal/integrations/render.go` (linha ~57, `case "cli-agent-json", "agent-json":`)
- `internal/integrations/render_test.go` (novo teste, ou extensão de existente)
**Actions:**
1. Trocar `json.MarshalIndent(map[string]string{...}, "", "  ")` por um `json.Encoder` com
   `SetEscapeHTML(false)` e `SetIndent("", "  ")`, escrevendo num `bytes.Buffer`. Atenção:
   `Encoder.Encode` já adiciona `\n` ao final — conferir se o `append(data, '\n')` logo depois no
   código atual precisa ser removido para não duplicar a quebra de linha (comparar byte-a-byte com
   a saída anterior menos o caractere escapado, para garantir que só o escaping mudou).
2. Adicionar um teste (fixture ou golden) que injeta um valor com `<`, `>` e `&` no source antes de
   `Render()` e confirma que a saída para `cli-agent-json`/`agent-json` não contém `<`,
   `>` nem `&`.
**Acceptance criteria:**
- [x] `go build ./...`, `go test ./internal/integrations/...` verdes (e `go test ./...` completo também)
- [x] `GO_BIN=bin/trackfw scripts/check-identity-parity.sh` com 0 falhas (era 6 → "Identity parity
      verified across Go/Node/Python for 11 target/surface combinations")
- [x] `bin/trackfw agents list --targets kiro,amazonq,antigravity --json` inspecionado manualmente
      pelo orquestrador — 0 ocorrências de `<`/`>`/`&`

> Auditoria manual (trackfw_architect): fix mínimo e correto — troca `json.MarshalIndent` por
> `json.Encoder` com `SetEscapeHTML(false)`, com nota explícita no código sobre o `\n` que
> `Encoder.Encode` já adiciona (evita duplicar). Teste novo cobre não-escaping, validade do JSON e
> round-trip do valor decodificado. Nenhum golden existente precisou de ajuste — os 4 goldens
> congelados cobrem outras representações (`subagent`, `custom-agent-toml`, `agent-directory`), não
> `cli-agent-json`/`agent-json`. Revalidei tudo eu mesmo: `go test ./...` completo verde,
> `check-identity-parity.sh` 0 falhas, inspeção manual da saída `--json` real confirmando ausência
> de escaping.

## Wave 2 — Auditoria dos demais pontos de serialização
> Dependencies: Wave 1 completa

### ML-2A — Auditar os demais json.Marshal* com contrato de paridade cross-runtime
**Status:** ✅ Concluído
**Files affected:** nenhum a priori — auditoria primeiro, fix só se confirmado
**Actions:**
1. Revisar `internal/generators/agentfiles.go` (6 call sites de `json.MarshalIndent`, geram
   `.claude/settings.json` e equivalentes de outros CLIs) e a saída `--json` de `trackfw
   validate`/`barrier`/`update` (`internal/commands/validate.go`, `barrier.go`, `update.go`,
   `update_harness.go`) — para cada um, checar se o conteúdo de origem pode conter `<`/`>`/`&` na
   prática (ex: mensagens de commit, texto de roadmap/REQ do usuário) e se existe algum gate de
   paridade Go×Node×Python que exercitaria essa saída.
2. Onde houver risco real confirmado (não só teórico) e gate existente, aplicar o mesmo fix do
   ML-1A. Onde for só risco teórico sem gate cobrindo, documentar a decisão (corrigir preventivamente
   é opcional, mas registrar por quê) — não é obrigatório blindar tudo se não há evidência de uso
   real com esses caracteres.
3. Chamadas Go-only sem equivalente cross-runtime (`internal/serve/`, `internal/sync/`) ficam de
   fora — não há contrato de paridade byte-a-byte pra elas, confirmar isso e não tocar.
**Acceptance criteria:**
- [x] Auditoria registrada nesta seção do roadmap (atualizar com o resultado por arquivo)
- [x] Todo fix aplicado (se houver) com teste de regressão equivalente ao ML-1A — nenhum fix aplicado
      (nenhum ponto atendeu ao critério de risco real + gate existente + prova possível de regressão)
- [x] `make quality` verde
- [x] `trackfw validate` sem violações

> Auditoria manual (trackfw_architect): validei a alegação central de forma independente —
> `grep -n "json.loads" scripts/check-update-parity.sh scripts/check-barrier.sh
> scripts/check-validate-parity.sh` confirma que os três reparseiam antes de comparar, mascarando
> qualquer divergência de escaping. Confirmei também a ausência de gate para `trackfw context --json`
> (`grep -rn "context.*--format\|context.*--json" scripts/*.sh` vazio) e que `UserNickname` é campo
> de texto livre no wizard (`internal/commands/identity_wizard.go:167`). Revalidei tudo: `go build`,
> `go test ./...` completo, `check-identity-parity.sh` 0 falhas, `trackfw validate` limpo. Decisão de
> não corrigir especulativamente (sem gate que prove a regressão) está correta e bem fundamentada —
> `context.go:185` fica registrado como candidato a REQ futura (criar gate primeiro, fix depois).

> **Auditoria (prometeu-tf, 2026-08-04) — resultado: nenhum fix aplicado.** Revalidei a lista de
> `json.Marshal`/`json.MarshalIndent`/`json.NewEncoder` em `internal/` com o grep pedido (ficou
> maior do que a lista original da REQ — inclui achados novos abaixo). Para cada um, o critério
> aplicado foi: (1) existe gate de paridade cross-runtime que exercita essa saída? (2) esse gate
> compara **bytes crus** (como `check-identity-parity.sh`, que foi o que pegou o bug original do
> ML-1A) ou ele **reparseia o JSON antes de comparar** (`json.loads` em Python, ou dict-equality),
> o que **anula** qualquer divergência de escaping porque o parse já desfaz o escaping de ambos os
> lados antes da comparação? (3) é plausível que conteúdo real (não só fixture de teste) contenha
> `<`/`>`/`&`?
>
> | Arquivo:linha | Saída | Gate de paridade? | Método de comparação | Conteúdo pode ter `<`/`>`/`&` na prática? | Risco | Ação |
> |---|---|---|---|---|---|---|
> | `internal/generators/agentfiles.go` (6 sites: `InjectClaudeHooks` L222, `InjectCodexHooks` L271, `InjectGeminiHooks` L319, `InjectKiroHooks` L354, `InjectCopilotHooks` L383, `InjectCursorHooks` L427) | `.claude/settings.json` e equivalentes (hooks de atenção) | **Nenhum** — grep pelos nomes das funções e por `settings.json`/`hooks` em `scripts/*.sh` não retorna nenhum gate cross-runtime | n/a | Não — comandos/paths de hook são strings fixas no código (`"AskUserQuestion"`, `"scripts/trackfw-attention-signal.sh"`), não texto livre do usuário | Teórico | Nenhuma |
> | `internal/commands/validate.go:31`, `barrier.go:565`, `update.go:73`, `update_harness.go:94` (`json.Marshal`/`json.NewEncoder` das saídas `--json`) | `trackfw validate/barrier/update --json` | Sim — `check-validate-parity.sh`, `check-artifact-parity.sh`, `check-update-parity.sh`, `check-barrier.sh` | **Reparseado antes de comparar**: `check-validate-parity.sh` compara tuplas `(rule, file)` extraídas via `json.load` (Python); `check-artifact-parity.sh` só valida campos individuais via `json.loads`, não faz diff cross-runtime dessas respostas; `check-update-parity.sh`/`check-barrier.sh` fazem `json.loads` → `json.dumps(...)` antes do `diff -u` (comentário no script cita isso para normalizar espaçamento). O ponto crítico é o **`json.loads` de entrada**, não o `json.dumps` de saída: `json.loads` desfaz o escaping HTML (`<` → `<`) independentemente de qualquer flag de `dumps`, então a comparação passa a ser sobre o valor já unescapado dos dois lados — mesmo que o bug do ML-1A existisse aqui, nenhum desses gates o pegaria | Baixo — conteúdo é `rule` (identificador fixo) e `file` (slug ASCII normalizado, ver NFKD em `check-artifact-parity.sh`), não texto livre | Teórico, e adicionalmente mascarado pelo próprio gate | Nenhuma |
> | `internal/identity/identity.go:72` (`Save` → `~/.trackfw/identity.json`) | Config de identidade do agente | **Nenhum** — `check-identity-parity.sh` usa um `identity.json` **escrito à mão via heredoc** como fixture de entrada; nunca gera esse arquivo chamando `Save()` do Go/Node/Python nem faz diff dos 3 outputs de `Save()` entre si | `UserNickname` é texto livre do usuário via wizard (`identity_wizard.go:167`) — plausível conter `&` (ex. "Kg & Time") | Real na prática, mas **sem gate nenhum cobrindo**, nem teórico nem detectável hoje | Nenhuma (critério do ML exige gate existente para virar fix obrigatório; registrado como candidato a REQ futura, ver observação abaixo) |
> | `internal/integrations/manifest.go:66` (`writeManifest` → `.trackfw/integrations-manifest.json`) | Manifesto de ownership | **Nenhum** — `check-identity-parity.sh` **exclui explicitamente** esse arquivo do diff de artefatos (comentário na linha ~160: é indexado por sha256/paths, esperado divergir por motivo diferente) | Campos são `destination` (path absoluto), `sha256`, `catalog_version`, `claims` (target/surface/scope/kind/item, todos enums do catálogo) — nenhum texto livre | Teórico | Nenhuma |
> | `internal/validator/validator.go:50` (`SaveBaseline` → `.trackfw-baseline.json`) | Baseline de violations/warnings | **Nenhum** — grep por `baseline`/`SaveBaseline` em `scripts/*.sh` só bate em listagem de subcomandos e nomes de cenário de falsify não relacionados | Conteúdo é `rule` + `file` (mesmo formato slugificado do item anterior) | Teórico | Nenhuma |
>
> **Achados adicionais fora da lista original da REQ** (grep próprio pediu para confirmar a lista, e ela mudou):
>
> | Arquivo:linha | Saída | Gate de paridade? | Método de comparação | Risco | Ação |
> |---|---|---|---|---|---|
> | `internal/commands/integrations_flags.go:352` (`printLifecycleOutput`, `agents/skills list/install/update/uninstall --json`) | Catálogo de items (nome/descrição) + deployments | Sim — `check-integration-cli-parity.sh` (`compare_json`) | **Semântico**: `json.load` os 3 arquivos e faz `assert document == first` (igualdade de dict Python) — reparseia antes de comparar, então também desfaz escaping HTML antes da comparação | Baixo (nomes/descrições vêm do `catalog.json` estático, conteúdo curado pelos devs) + mascarado pelo gate | Nenhuma |
> | `internal/generators/context.go:185` (`trackfw context --format json`) | Contexto de governança (inclui **títulos reais de REQ/ADR/Roadmap**, texto livre do usuário) | **Nenhum** — nenhum script em `scripts/*.sh` referencia `context --format`/`context --json` | n/a | **Este é o vetor de risco mais plausível de todos os auditados** — títulos de REQ/roadmap são texto livre do usuário e podem conter `<`, `>`, `&` (ex.: "API <-> Frontend", "Bug A & B") — mas sem gate nenhum, não atende ao critério do ML para fix obrigatório | Nenhuma agora; **candidato a REQ separada** para (a) criar gate de paridade cross-runtime para `trackfw context --json` e (b) só então aplicar o fix de `SetEscapeHTML(false)` |
> | `internal/server/server.go:391` (`handleAPIData`, pacote `internal/server`) | JSON de dashboard antigo | N/A — dois greps (`grep -rn "internal/server\"" cmd/ internal/` e `grep -rln "internal/server" --include="*.go" . \| grep -v "^./internal/server/"`) não retornam nenhum import de fora do próprio pacote | n/a | Nenhum import encontrado no restante do código — risco não avaliado como ativo | Nenhuma |
>
> **Confirmação do item 4 do ML** (`internal/serve/*.go`, `internal/sync/jira.go`, `internal/sync/linear.go`): confirmado Go-only. `internal/serve/` é o servidor HTTP do `trackfw serve` (dashboard local, sem equivalente Node/Python com contrato de bytes); `internal/sync/jira.go`/`linear.go` são payloads de saída para APIs externas (Jira/Linear), sem dashboard/sync duplicado nos outros runtimes. Nenhum arquivo desses foi tocado.
>
> **Conclusão**: nenhum fix aplicado neste ML. Todo ponto com gate de paridade cross-runtime real
> usa comparação semântica (reparse JSON → comparação de estrutura), o que mascara completamente
> qualquer divergência de HTML-escaping — um fix nesses pontos seria tecnicamente possível de
> provar isoladamente (um teste Go local no estilo `TestRenderJSONRepresentationsDoNotHTMLEscape`
> provaria o não-escaping sem depender de gate cross-runtime), mas **nenhum gate existente
> detectaria uma regressão de paridade** se o fix fosse revertido no futuro — ou seja, o fix seria
> inverificável como garantia de paridade, que é o objetivo real desta REQ. Os pontos sem gate
> algum são risco teórico (conteúdo estruturado/enum) exceto `context.go:185`, que é risco real
> mas está fora do escopo deste ML por não ter gate — registrado como candidato a REQ futura em
> vez de fix especulativo sem prova.
