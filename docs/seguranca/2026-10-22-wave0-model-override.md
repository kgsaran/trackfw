# Wave 0 — Threat Model: ML-0A
# REQ-2026-09-26 — o harness do arquiteto não proíbe sobrepor o modelo do subagente

> Produzido por: hades-tf | 2026-10-22
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md`
> Branch: fix/arquiteto-nao-sobrepoe-modelo-do-subagente

---

## 1. Completude da enumeração

### 1.1 Sítio fonte

O único ponto de emissão da instrução de despacho de subagente é:

```
internal/integrations/assets/agents/architect.md:40-41   ## Dispatch contract
```

Evidência:

```
grep -rn 'subagent_type\|dispatch\|Agent tool' internal/ --include='*.md' \
    (excluindo testdata e corpus)
```

Resultado relevante: apenas `architect.md:41` (corpo do ativo) e os dois goldens congelados
(`architect.subagent.golden.md:41`, `architect.agent-directory.golden.md:53`), que são derivados
do mesmo fonte. Nenhum outro ativo de agente ou skill menciona `subagent_type` ou "Agent tool" fora
do contexto de exemplo/corpus.

**Único agente com ferramenta `Agent`** (capaz de despachar outros agentes):

```
internal/integrations/assets/agents/architect.md:6  tools: Agent, Read, Edit, …
```

Todos os outros 11 agentes (`backend`, `frontend`, `qa`, `infra`, `security`, `code-quality`, `dba`,
`ux`, `data`, `iac`, `tooling`) declaram `tools:` sem `Agent`. A proibição não precisa ser replicada
em nenhum outro ativo de agente.

Skills (19 arquivos em `assets/skills/`) não mencionam `subagent_type`, "dispatch" ou "Agent tool":
confirmado por `grep -rn 'subagent_type\|dispatch\|Agent tool' internal/integrations/assets/skills/`.

Geradores (`internal/generators/`): os templates de CLAUDE.md/AGENTS.md semeados por `scaffold.go`
(incluindo o slash command `barrier.md`, `plan.md`, `architect.md`) não contêm "Dispatch contract",
`subagent_type` nem instrução de campo. Confirmado por:
`grep -rn 'subagent_type\|Dispatch contract' internal/generators/ | grep -v '_test.go'` — saída vazia.

Cópias renderizadas versionadas: `grep -rln 'Dispatch contract'` (excluindo `.git`, `roadmapdoc/`,
`pathguard/` — corpus congelados) encontra apenas:
- o fonte (`architect.md`),
- os dois goldens congelados,
- referências em ADRs, REQs, roadmaps e `docs/` (documentação),
- `vault/notes/check-identity-parity-json-html-escaping-pre-existing-2026-08-04.md`.

**Não há cópia instalada do arquiteto em `.claude/agents/`, `.codex/agents/` ou `.gemini/agents/`
dentro do worktree.** O arquivo `.claude/commands/trackfw/architect.md` é um slash command em
português (texto de orientação de stack), não um render do agente; contém zero instâncias de
"Dispatch contract" (confirmado por `grep -c 'Dispatch contract'`).

Cópias em HOME do usuário (`~/.claude/agents/trackfw-architect.md`, etc.) ficam fora do escopo do
worktree e são tratadas como resíduo declarado (§5).

### 1.2 Representações e destinos para o architect

O catálogo (`internal/integrations/assets/catalog.json`) define 12 agentes e 9 targets com 13
superfícies. Para o agente `architect`, as representações emitidas são:

| Representação | Targets/superfícies | Rota em render.go |
|---|---|---|
| `subagent` | claude | Rota B (default) |
| `agent-markdown` | cursor (IDE), gemini (CLI), kiro (IDE), copilot (IDE) | Rota B (default) |
| `custom-agent` | copilot (IDE) | Rota B (default) |
| `skill` | windsurf (fallback) | Rota B (default) |
| `custom-agent-toml` | codex (CLI) | Rota A: body → `developer_instructions` |
| `agent-directory` | antigravity (current) | Rota A: frontmatter reconstruído + body |
| `agent-json` | antigravity (legacy-cli), kiro (CLI) | Rota A: body → campo `"prompt"` JSON |
| `cli-agent-json` | amazonq (CLI) | Rota A: body → campo `"prompt"` JSON |
| `opencode-agent` | opencode (CLI) | Rota A: frontmatter reconstruído + body (sem `model:`) |

**Em todas as representações, o body (onde vive o `## Dispatch contract`) é preservado:**
- Rota B: `normalizeMarkdown(source)` retorna a fonte normalizada inteira.
- `custom-agent-toml`: `fmt.Sprintf("developer_instructions = %s", strconv.Quote(body))` (render.go:223).
- `agent-json`/`cli-agent-json`: body → `"prompt"` no mapa JSON (render.go:233).
- `agent-directory`: `sb.WriteString(body + "\n")` após frontmatter reconstruído (render.go:259-261).
- `opencode-agent`: `sb.WriteString(body + "\n")` após frontmatter reconstruído (render.go:296-298).

A instrução chega ao modelo em todos os 9 caminhos de renderização.

**Subconjunto coberto pelos goldens congelados (ML-1A deve expandir):**
- `architect.subagent.golden.md`: representa Rota B / `subagent` (claude)
- `architect.agent-directory.golden.md`: representa `agent-directory` (antigravity)

As demais 7 representações não têm golden. O teste semântico `TestArchitectDispatchContractForbidsModelOverride`
deve iterar todas as representações via catálogo, não via lista hardcoded.

### 1.3 Campo `model:` ausente em algumas representações

Em três caminhos de renderização, o campo `model:` não é emitido no arquivo instalado:

- **`agent-json`/`cli-agent-json`**: JSON com apenas `name`, `description`, `prompt` (render.go:230-234).
- **`opencode-agent`**: deliberadamente omite `model:` (render.go:280-289, comentário extenso).
- **`cursor` (agent-markdown)**: `model:` removido quando sem mapeamento disponível (render.go:309).

O novo parágrafo diz: *"Read two fields from the installed agent file before every dispatch: `name:`…
`model:`…"*. Para esses destinos, o arquivo instalado não tem `model:`, o que contradiz a instrução
e enfraquece a racionalização. O leitor pode concluir que não há roteamento. **Este é um ajuste
necessário no texto do ML-1A** (§3.d), dentro do escopo negativo da REQ (não muda o `model:` de
nenhum agente; ajusta apenas a instrução). Reportar ao arquiteto com o caminho exato:
`internal/integrations/assets/agents/architect.md:81` (texto planejado do roadmap).

---

## 2. Mesma classe (AC4)

**Pergunta:** existe outro contrato deste produto que manda ler um artefato e nomeia só um subconjunto
dos campos que importam?

**Como procurei:** `grep -rn 'name:\|read.*field\|from the frontmatter\|installed agent' internal/integrations/assets/` — sem resultado fora do `architect.md:41`. Os cinco skills de processo (governance, plan, implement, review, release) foram lidos na íntegra: nenhum enumera campos específicos de artefatos. A skill de tooling não recomenda seleção de modelo.

**Candidatos avaliados:**

| Candidato | Arquivo:linha | Veredito |
|---|---|---|
| `## Dispatch contract` no architect.md | `architect.md:41` | É o defeito que esta REQ corrige. |
| Skill `trackfw-governance`: `Run trackfw context --format=json` | `assets/skills/governance.md:3` | Usa a CLI com saída JSON completa. Nenhum campo nomeado. Não é a mesma classe. |
| Skill `trackfw-plan`: `Inspect existing governance artifacts` | `assets/skills/plan.md:3` | Nenhum campo enumerado. Não é a mesma classe. |
| Skill `trackfw-implement`: `Identify the matching REQ and WIP roadmap` | `assets/skills/implement.md:3` | Nenhum campo enumerado. Não é a mesma classe. |

**Resultado: nenhum.** Não há outro contrato no produto que instrua o modelo a ler um artefato e
enumere um subconjunto incompleto dos campos com impacto operacional.

---

## 3. Threat model

O adversário aqui é o implementador apressado e o arquiteto otimista — não um atacante externo.

### 3.a Golden atualizado junto com a fonte

**Mecanismo:** o implementador altera `architect.md`, regenera os dois goldens (`architect.subagent.golden.md`,
`architect.agent-directory.golden.md`) corretamente, e submete o PR. A única verificação existente é
`TestRenderWithoutIdentityMatchesFrozenGoldens` (`render_test.go:765`), que compara a saída de
`Render` byte a byte com o golden. Se golden e fonte mudam juntos, o teste passa sem nenhuma
verificação semântica sobre o que mudou.

**Gate que deve capturar:** o teste semântico `TestArchitectDispatchContractForbidsModelOverride`
(ML-1A), que asserta a presença de strings semânticas específicas no output e exige que a remoção
do parágrafo cause falha — independentemente do golden.

**Dependência que cria risco:** o teste semântico e o `architect.md` ficam no mesmo PR. Um implementador
que altera ambos (fonte + teste) pode eliminar a cobertura. A Wave 2 (red team do hades-tf) é a
barreira independente para isso.

### 3.b Identidade customizada (identity.Config)

**Mecanismo hipotético:** a identidade poderia reescrever a seção `## Dispatch contract`.

**Refutação:** `identity.Config` (internal/identity/identity.go:29-33) tem apenas três campos:
`SchemaVersion int`, `UserNickname string`, `Agents map[string]AgentIdentity`. Cada `AgentIdentity`
tem apenas `DisplayName string` e `Slug string`. Não há campo de conteúdo do corpo ou override de
seção.

O `Render` com identidade (render.go:203-354) injeta apenas: (a) uma linha de saudação preposta ao
body (render.go:211), (b) reescrita de `name:` e `description:` no frontmatter, (c) reescrita da
linha de assinatura (`^— <name>, <title>$`) no final do body. A seção `## Dispatch contract` no
meio do body é preservada byte a byte.

**Veredito:** identidade customizada não esvazia a proibição.

### 3.c Render por destino que trunca ou transforma o texto

**Rota B (subagent, agent-markdown, custom-agent, skill):** retorna `normalizeMarkdown(source)` —
toda a fonte normalizada. Sem truncamento. A seção `## Dispatch contract` chega intacta.

**custom-agent-toml (codex):** o body vai para `developer_instructions = strconv.Quote(body)`.
`strconv.Quote` escapa `\`, `"` e caracteres de controle, mas **não backtick**. O novo parágrafo
contém `` `model` `` e `` `model:` `` entre backticks; eles chegam literalmente no valor TOML.
O arquivo instalado fica:
```toml
developer_instructions = "...Never pass the Agent tool's `model` parameter. Read two fields..."
```
O modelo lê o texto e os backticks chegam como estão. Sem truncamento relevante.

**agent-directory (antigravity):** corpo escrito depois do frontmatter reconstruído (render.go:259-261).
Seção `## Dispatch contract` presente no body. Sem truncamento.

**agent-json / cli-agent-json:** corpo no campo `"prompt"` JSON. Backticks não são escapados por
`json.Marshal` (apenas `"`, `\`, `<`, `>`, `&`). A seção chega intacta no JSON.

**opencode-agent:** corpo após frontmatter mínimo. Presente. Sem truncamento.

**Cursor (removeFrontmatterModelLine):** remove a linha `model:` do **frontmatter** somente.
O body é preservado em `rest = trimmed[4+end:]` (render.go:763). O novo parágrafo no body não é
afetado.

**rewriteFrontmatterModelLine:** opera exclusivamente dentro de `trimmed[4:4+end]` (frontmatter).
O body começa em `trimmed[4+end+4:]`. A presença de `` `model:` `` no texto do body não é capturada
por `markdownParts` (render.go:571-584: varre apenas linhas do frontmatter pelo padrão
`key: value` com `strings.Cut`). Nenhuma colisão.

### 3.d Parágrafo com premissa inconsistente para destinos sem `model:`

**Mecanismo:** em `agent-json`, `cli-agent-json`, `opencode-agent` e `cursor`, o arquivo instalado
não contém `model:`. O novo parágrafo (texto planejado no roadmap, ML-1A linha 80) instrui o
arquiteto a ler `model:` do arquivo instalado. Se o arquivo não tem o campo, o arquiteto pode
concluir que não há roteamento de custo e continuar passando `model` no despacho.

A proibição em si ("Never pass the Agent tool's `model` parameter") permanece válida em todos os
destinos. O que fica enfraquecido é o _motivo_: "o projeto configurou o modelo e está na linha ao
lado" não se sustenta quando a linha não existe.

**Ajuste sugerido para ML-1A:** adicionar uma cláusula como: *"If the installed file has no
`model:` field, the CLI's configured default is the cost routing; the prohibition still applies —
never pass `model` in the call."* O arquiteto decide se este ajuste cabe no escopo negativo da REQ.
O texto deve ser apontado explicitamente na instrução do handoff para o prometeu-tf.

### 3.e Marcador thirdparty e conflito de nominação de seção

O marcador `"dispatch contract"` está em `literalMarkers` (markers.go:18-25). Qualquer artefato de
terceiro com `## Dispatch contract` como cabeçalho é recusado por `CheckMarkers`. Isso protege
contra um ativo de terceiro que substitua a seção.

**Mas:** o marcador defende contra o cabeçalho, não contra o conteúdo. Um ativo de terceiro com
o parágrafo de proibição sob um cabeçalho diferente (ex: `## Agent dispatch policy`) não seria
recusado. Este é o limite documentado do D3 ("tripwire, não filtro contra adversário competente").
Não é novidade desta REQ.

O ML-1A **não renomeia** a seção (roadmap:81: "Não renomeie a seção nem altere o parágrafo
existente") — o marcador continua funcionando.

---

## 4. Alvos de falsificação nas duas direções

Para cada representação que o catálogo emite para o architect, o teste
`TestArchitectDispatchContractForbidsModelOverride` (ML-1A) deve:

### 4.1 Braço positivo

Asserta que a renderização contém a proibição. A asserção deve verificar:

1. A string `Never pass the Agent tool's \`model\` parameter` está presente no output.
2. A string `` `model:` is the project's cost routing `` está presente no output.
3. **Localização (seção correta):** o texto está entre a linha `## Dispatch contract` e o próximo
   cabeçalho `## ` — não num comentário HTML, não numa seção diferente. Substrings soltos permitem
   que o parágrafo seja movido e o teste ainda passe.

Por representação:

| Representação | Campo a inspecionar | Observação |
|---|---|---|
| `subagent` (Rota B) | String completa do output | Inspeção direta. |
| `custom-agent-toml` | Valor de `developer_instructions` após unmarshal do TOML | `strconv.Unquote` ou parser TOML; backtick não é escapado. |
| `agent-directory` | Corpo após o segundo `---` | Inspeção direta. |
| `agent-json` / `cli-agent-json` | Campo `"prompt"` após `json.Unmarshal` | Backtick não é escapado em JSON. |
| `opencode-agent` | Corpo após o segundo `---` | Idêntico a agent-directory. |
| Rota B / `agent-markdown`, `custom-agent`, `skill` | String completa | Idêntico a `subagent`. |

O teste deve iterar `catalog.Targets()` (ou equivalente), não uma lista hardcoded, para que um
target futuro não escape silenciosamente. Deve asserta que o conjunto coberto é não-vazio.

Deve incluir subcasos `zeusIdentity()` para Rota A e Rota B: a saudação injetada antes do body e
a reescrita de assinatura no final não devem destruir a detecção da seção do meio.

### 4.2 Braço negativo

Asserta que a mesma lógica reprova quando o parágrafo não está presente.

**Risco crítico: braço negativo vácuo.** Se `strings.Replace(source, paragraphNeedle, "", 1)` não
encontrar a agulha (por diferença de CRLF, espaço extra, ou agulha com newline interno), a fonte
fica inalterada, o Render produz o output completo com o parágrafo, o teste passa — e o braço
negativo afirmou nada.

**Defesa obrigatória no código do teste:**

```go
modified := strings.Replace(source, paragraphNeedle, "", 1)
if bytes.Equal(modified, source) {
    t.Fatal("strings.Replace não encontrou a agulha — o braço negativo seria vácuo; verificar CRLF e espaços")
}
```

Adicionalmente:
- A agulha não deve conter newlines. Usar apenas a primeira linha do parágrafo (ex:
  `Never pass the Agent tool's \`model\` parameter.`) como `paragraphNeedle`.
- Normalizar CRLF da fonte antes do Replace: `integrations.NormalizeCRLF(source)` (já usada por
  `markdownParts`). O ativo embedado pode ter CRLF sob `core.autocrlf=true` (evidência:
  `TestRenderWithoutIdentityMatchesFrozenGoldens_CRLFSource`, render_test.go:637-670).
- O braço negativo deve chamar `Render(...)` com a fonte modificada, não comparar strings na fonte.

### 4.3 Risco de falso negativo semântico

O teste existente `TestRenderWithoutIdentityMatchesFrozenGoldens` só cobre `subagent/claude` e
`agent-directory/antigravity`. As 5 representações restantes não têm golden. Um regressão nelas
passa silenciosamente hoje. O teste novo fecha esse gap para o parágrafo da proibição,
mas não para outras mudanças futuras. A Wave 2 (hefesto-tf ML-2A) deve avaliar a estratégia de
cobertura dessas representações.

### 4.4 Risco de falso positivo

Um futuro `strings.Replace` que remova mais do que o esperado (ou que altere a seção de outra
forma) deve ser detectado pela asserção de presença (braço positivo) da mesma suíte.

---

## 5. Resíduo declarado

O harness do trackfw gera **instrução**, não intercepta chamada. Cada item abaixo é coberto pelo
escopo negativo da REQ ("Não introduz mecanismo técnico de bloqueio no Agent tool").

1. **Instrução sem interceptação:** uma sessão que ignora a instrução ainda pode passar `model` no
   Agent tool. Nenhuma barreira técnica impede. Confirmado pelo escopo negativo da REQ (REQ.md:76-79).

2. **Cópias instaladas em HOME ficam obsoletas:** arquivos já instalados em `~/.claude/agents/trackfw-architect.md`,
   `~/.codex/agents/trackfw-architect.toml`, etc., continuam com o contrato antigo até que
   `trackfw agents update` seja executado. Nada nesta REQ regenera essas cópias.

3. **Destinos sem `model:` no arquivo instalado** (agent-json, cli-agent-json, opencode-agent, cursor):
   a racionalização do parágrafo é enfraquecida. A proibição permanece válida, mas o motivo
   declarado ("o campo está na linha ao lado") não se sustenta. Ajuste de texto sugerido em §3.d.
   Julgamento do arquiteto sobre adequação ao escopo negativo da REQ.

4. **Destinos sem Agent tool:** CLIs como Codex, Gemini, Kiro e OpenCode podem não ter um parâmetro
   `model` no equivalente à Agent tool. O parágrafo é irrelevante nesses contextos, mas inofensivo.

5. **Braço semântico e fonte no mesmo PR:** o teste `TestArchitectDispatchContractForbidsModelOverride`
   e `architect.md` estarão no mesmo PR. Um implementador que altere os dois em conjunto pode
   esvaziar a proteção. Barreira independente: Wave 2 red team (ML-2B, hades-tf) e auditoria do
   arquiteto.

6. **Marcador thirdparty não cobre conteúdo sob cabeçalho diferente:** `literalMarkers` (markers.go:24)
   recusa `## Dispatch contract` como cabeçalho de terceiros, mas não detecta paráfrases do
   parágrafo de proibição sob outro cabeçalho.

---

## Veredito

O ML-1A pode seguir, com um ajuste obrigatório antes do despacho:

**Ajuste necessário** (arquivo e linha):
`internal/integrations/assets/agents/architect.md:81` (texto planejado, roadmap ML-1A linha 80) —
adicionar uma cláusula para o caso em que o arquivo instalado não tem `model:` (§3.d). Texto
sugerido no final do parágrafo:
> *"If the installed file has no `model:` field, the target CLI uses its configured default as cost
> routing; the prohibition still applies — never pass `model` in the call."*

**Adicionais para o handoff do prometeu-tf:**
- Iterar `catalog.Targets()` no teste, não lista hardcoded.
- Normalizar CRLF antes de `strings.Replace` na agulha do braço negativo.
- Verificar que a fonte foi modificada (`bytes.Equal` antes/depois) — sem isso o braço negativo é vácuo.
- Asserta localização (entre `## Dispatch contract` e o próximo `##`), não apenas substring.
- Incluir subcasos com `zeusIdentity()`.

Não há arquivo adicional necessário além dos listados no roadmap ML-1A.
