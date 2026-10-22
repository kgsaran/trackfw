---
status: wip
date: 2026-09-26
req: "docs/req/REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md"
squad: "hades-tf, prometeu-tf, hefesto-tf"
---

# Roadmap: o harness do arquiteto nao proibe sobrepor o modelo do subagente e o roteamento de custo e ignorado em silencio

> Created: 2026-09-26 | Status: wip | Reescrito contra a main 3be3375c (v10.1.0) em 2026-10-22

## Context
REQ: docs/req/REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md

### Medido na main 3be3375c (2026-10-22)

- A fonte única é `internal/integrations/assets/agents/architect.md`. A seção `## Dispatch contract` (linhas 40-41) exige o
  `subagent_type` e manda ler só o `name:` do frontmatter. A palavra `model` não aparece fora do frontmatter (`:4 model: opus`).
- Os goldens `internal/integrations/testdata/architect.subagent.golden.md:40` e
  `architect.agent-directory.golden.md:52` congelam o mesmo texto. Hoje, o teste
  `TestRenderWithoutIdentityMatchesFrozenGoldens` (`render_test.go`) é a única verificação, e ela compara o texto
  inteiro. Basta atualizar o golden junto para uma regressão passar sem ninguém ver. Falta uma asserção semântica.
- O `model:` é traduzido por destino (Codex `gpt-5.4`, Cursor `composer-2.5`; `render_test.go:59,108`). Ou seja, o
  campo é configuração do projeto para cada CLI.
- `internal/thirdparty/markers_test.go:22` usa "Dispatch contract" como marcador de cabeçalho. Renomear a seção mexe
  nessa defesa. Por isso, **não renomear**.
- Não há issue aberta. A varredura de `gh issue list` em 2026-10-22 voltou vazia.
- Nenhuma ADR é necessária: o defeito é uma omissão num contrato existente, e não há decisão de desenho nova (escopo negativo da REQ).

## Acceptance Criteria
- [ ] AC1. O Dispatch contract proíbe explicitamente passar o parâmetro `model` do Agent tool, com a razão escrita (roteamento de custo).
- [ ] AC2. O contrato manda ler `name:` e `model:` do arquivo instalado e diz que o `model:` é decisão do projeto.
- [ ] AC3. Há falsificação nas duas direções: a renderização do architect **sem** o parágrafo reprova; **com** ele, passa. Vale em toda representação que o catálogo emite para o architect.
- [ ] AC4. A enumeração de outros contratos da mesma classe fica escrita, mesmo que dê "nenhum" (Wave 0).
- [ ] AC5. Os goldens acompanham a mudança, e ela aparece no diff.
- [ ] AC6. `make quality` e o CI ficam verdes.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e enumeração da classe
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-wave0-model-override.md` (novo, único arquivo)
**Actions:**
1. Completude da enumeração: liste todo lugar do repositório que emite instrução de despacho de subagente. No mínimo,
   procure com `grep -rn -i 'subagent_type\|dispatch\|Agent tool' internal/ scripts/` em assets de agentes, skills,
   templates de CLAUDE.md/AGENTS.md e em `internal/generators/`. Responda se outro sítio deveria carregar a mesma proibição.
2. Mesma classe (AC4): há outro contrato do produto que manda ler um artefato e nomeia só um subconjunto dos campos que
   importam? Liste os candidatos com arquivo:linha e um veredito para cada.
3. Threat model: quem esvazia a proibição sem violar nenhuma regra escrita? Por exemplo, um golden atualizado junto,
   uma identidade customizada que reescreve a seção, um render por destino que corta texto, ou o marcador do thirdparty.
4. Alvos de falsificação nas duas direções, por representação (subagent/claude, agent-directory/antigravity,
   custom-agent-toml/codex e as demais que o catálogo emitir para o architect).
5. Resíduo declarado: o harness gera instrução e não intercepta a chamada (escopo negativo da REQ).
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (arquivo:linha ou comando e saída)
- [x] Nenhuma linha de implementação escrita

**Auditoria do arquiteto (2026-10-22):** aprovada. Conferido no código: só `architect.md` tem `Agent` em `tools:`; os destinos `agent-json`/`cli-agent-json` (`render.go:230-234`), `opencode-agent` (`render.go:280-289`) e o `cursor` sem mapeamento (`render.go:309`) não emitem `model:`. Por isso o texto do ML-1A ganhou uma cláusula para arquivo sem `model:`; a cláusula fica dentro do escopo, porque não muda o modelo de nenhum agente. AC4: nenhum outro contrato da mesma classe.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-22-wave0-model-override.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada pelo arquiteto.

### ML-1A — Proibição no Dispatch contract, goldens e teste semântico
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/assets/agents/architect.md`, `internal/integrations/testdata/architect.subagent.golden.md`,
`internal/integrations/testdata/architect.agent-directory.golden.md`, `internal/integrations/render_test.go`
(outros só se a Wave 0 provar que são necessários, com o caminho escrito aqui antes do despacho)
**Actions:**
1. Em `architect.md`, dentro de `## Dispatch contract`, logo depois do parágrafo atual (linha 41), insira uma linha em
   branco e este parágrafo, exatamente como está:
   `Never pass the Agent tool's \`model\` parameter. Read two fields from the installed agent file before every dispatch: \`name:\` is the value you pass as \`subagent_type\`; \`model:\` is the project's cost routing for that role — the project's decision, not yours. The \`model\` parameter overrides that routing silently: no error, no warning, and the cost only shows on the bill. If the installed file has no \`model:\` field, the target CLI applies its own configured default, and that default is the project's routing too. If a task seems to need a different model, tell the user and let them change the agent's configuration; never override it in the call.`
   Não renomeie a seção nem altere o parágrafo existente.
2. Regenere os dois goldens a partir do render, sem editar à mão, e confira que o diff deles é só o parágrafo novo.
3. Em `render_test.go`, crie `TestArchitectDispatchContractForbidsModelOverride`, com um subteste por representação
   que o catálogo emite para o architect:
   - braço positivo: a saída contém `Never pass the Agent tool's \`model\` parameter` e `\`model:\` is the project's cost routing`;
   - braço negativo: o asset com o parágrafo removido. Use `strings.Replace` **e verifique que a fonte mudou** (sem isso, o
     braço negativo passa vazio). A mesma checagem tem de reprovar.
   Depois, acrescente ao comentário de re-congelamento (perto da linha 739) uma linha que cite a REQ-2026-09-26.
4. Defesas exigidas pela Wave 0 (`docs/seguranca/2026-10-22-wave0-model-override.md`):
   - percorra os destinos e representações do catálogo (`catalog.Targets`), sem lista fixa. Exija que o conjunto coberto
     não seja vazio e que inclua `subagent`, `custom-agent-toml`, `agent-directory`, `agent-json`, `cli-agent-json` e `opencode-agent`;
   - normalize o CRLF da fonte antes do `strings.Replace` (veja `TestRenderWithoutIdentityMatchesFrozenGoldens_CRLFSource`);
   - verifique que as frases estão **dentro** da seção, entre `## Dispatch contract` e o `##` seguinte;
   - no TOML do codex, decodifique `developer_instructions` antes de verificar; no JSON (`agent-json`/`cli-agent-json`), decodifique `prompt`;
   - inclua subcasos com `zeusIdentity()` (identidade customizada).
**Acceptance criteria:**
- [x] `go build ./...` com rc=0
- [x] `go test ./internal/integrations/ ./internal/thirdparty/` com rc=0
- [x] Prova de mordida: com o parágrafo removido de `architect.md`, o teste novo reprova; restaurado, passa (registrar as duas saídas)
- [x] O relatório declara, para cada teste novo, a conclusão do ML que ele afirma, em uma frase (Regra Dura de Reconciliação)

**Auditoria do arquiteto (2026-10-22):** a 1ª entrega foi reprovada: o braço negativo não chamava a mesma checagem do positivo e a frase do motivo nunca era exercida. O corretivo criou os braços `negative/no-paragraph` e `negative/no-rationale`, ambos exigindo `checkPhrases != ""`. Medido pelo arquiteto: 49 PASS (1 + 12 positivos + 12 com `zeusIdentity` + 24 negativos). Mordida: com `phrase2` inválida os positivos reprovam; com `checkPhrases` neutralizada os 24 negativos reprovam.

**Gates da wave:**
```bash
go build ./...
go test ./internal/integrations/ ./internal/thirdparty/
```

### ML-1B — Corretivo da Wave 2 (localização com mordida e conteúdo inerte)
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/render_test.go`, `vault/notes/dispatch-section-substring-match-bypass-2026-10-22.md`
**Origem:** o Hefesto apontou o A1 (a checagem de localização não tinha mordida) e A2–A5. O Hades, na primeira rodada, apontou D-1 a D-6 (sufixo, H3, duplicata, comentário, fence e recuo).
**Decisão do arquiteto:** o parser de markdown de 438 linhas que o Hades propôs não foi adotado. Em vez dele: o cabeçalho tem de casar exatamente e uma única vez, e a seção não pode ter ```` ``` ````, `~~~`, `<!--` nem a `phrase1` fora da coluna 0.
**Acceptance criteria:**
- [x] 7 braços negativos novos (wrong-section, suffix-heading, h3-heading, duplicate-heading, comment-wrapped, fence-wrapped, indented), cada um com guarda de vacuidade
- [x] Medido pelo arquiteto: 133 PASS (1 + 12 + 12×10); `go test ./internal/integrations/ ./internal/thirdparty/` ok
- [x] Mordida: com `extractDispatchSection` devolvendo `body`, os 4 braços de localização reprovam (×12); sem a recusa de conteúdo inerte, os 3 braços de inerte reprovam (×12)
- [x] Resíduos declarados: M3 (cláusula de exceção na mesma seção) e um fence aberto noutra seção que envolva a seção inteira

## Wave 2 — Revisão
> Dependencies: Wave 1 auditada.

### ML-2A — Qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/seguranca/2026-10-22-quality-model-override.md` (novo)
**Acceptance criteria:**
- [ ] Parecer sobre o teste (vacuidade, braço negativo real, cobertura de todas as representações)

### ML-2B — Red team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-red-team-model-override.md` (novo)
**Acceptance criteria:**
- [ ] Tentativa de esvaziar a proibição sem reprovar o teste, com o resultado registrado

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-22-quality-model-override.md
test -s docs/seguranca/2026-10-22-red-team-model-override.md
```

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf (execução de gate, sem código)
**Acceptance criteria:**
- [ ] `make quality` com rc=0, com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Vivo**: o "Dispatch contract" de `assets/agents/architect.md` e do golden só fala de `subagent_type`.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
