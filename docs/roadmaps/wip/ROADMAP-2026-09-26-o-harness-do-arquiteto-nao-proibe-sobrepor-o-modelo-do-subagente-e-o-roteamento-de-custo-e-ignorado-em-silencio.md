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

### ML-1C — Corretivo da 2ª rodada (bloco HTML)
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/render_test.go`, `vault/notes/dispatch-section-substring-match-bypass-2026-10-22.md`
**Acceptance criteria:**
- [x] O D-7 foi fechado: o `checkPhrases` recusa linha da seção que comece com `<` depois de no máximo 3 espaços. O braço `negative/html-block-wrapped` tem guarda de vacuidade.
- [x] Medido pelo arquiteto: 145 PASS. Numa cópia sem a checagem, exatamente os 12 `html-block-wrapped` reprovam.
- [x] Resíduo ampliado (D-8): fence ou comentário HTML que abre numa seção anterior e fecha numa posterior, envolvendo a seção inteira. Em todo vetor que sobra, quem esvazia a proibição é um texto ativo novo de exceção (M3), e a barreira contra isso é a auditoria do diff.
- [x] O arquiteto não abriu uma 3ª rodada de red team: o D-7 era o único defeito aberto, e a correção dele foi conferida por mordida medida.

## Wave 2 — Revisão
> Dependencies: Wave 1 auditada.

### ML-2A — Qualidade
**Status:** ✅ Concluído
**Agente:** hefesto-tf
**Files affected:** `docs/seguranca/2026-10-22-quality-model-override.md` (novo)
**Acceptance criteria:**
- [x] Parecer sobre o teste (vacuidade, braço negativo real, cobertura de todas as representações). Veredito: APROVADO COM RESSALVA; A1–A5 foram corrigidos no ML-1B.

### ML-2B — Red team
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-red-team-model-override.md` (novo)
**Acceptance criteria:**
- [x] Tentativa de esvaziar a proibição sem reprovar o teste, com o resultado registrado. 1ª rodada REPROVADO (D-1 a D-6, corrigidos no ML-1B); 2ª rodada REPROVADO (D-7, corrigido no ML-1C; D-8 virou resíduo declarado).

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-22-quality-model-override.md
test -s docs/seguranca/2026-10-22-red-team-model-override.md
```

## Wave 4 — Escape TOML do destino Codex (AC7, ampliação decidida pelo KG em 2026-10-22)
> Dependencies: Wave 2 concluída. A Wave 3 é refeita depois desta wave.
>
> **2026-10-22:** o KG mergeou o PR #598 (ed910c22) antes desta wave. As Waves 0–3 do model-override foram entregues nele, e o título do PR já cita o TOML, mas o `strconv.Quote` ainda está na main. A Wave 4 segue na branch `fix/harness-arquiteto-escape-toml-codex`, na mesma REQ (Regra Dura), e o roadmap fica em wip até ela fechar.

### ML-4.0 — Threat model do encoder TOML
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-22-wave0-toml-escape.md`
**Acceptance criteria:**
- [x] Alcançabilidade medida. O DEL (0x7F) passa por `containsControlCharIdent` e chega à `description` (T-2). O slug não é validado no `identity.Load` (só `identity.Validate`, usado pelo wizard e pelas flags, aplica o `slugPattern`), e um 0x07 nele chega ao `name` (T-3).
- [x] Versão do TOML: o Codex 0.163.0 aceita TOML 1.1. O piso adotado é TOML 1.0 (Python 3.12 serve de oráculo de 1.0).
- [x] Falsificação e resíduo declarados. Novos sítios: o YAML (`yamlEscapeDoubleQuoted` emite C0 cru; `containsControlChar` não cobre 0x7F nem C1) e o T-4 (`ApplyThirdPartyReferences` anexa um bloco HTML/markdown ao TOML e ao JSON).

**Auditoria do arquiteto (2026-10-22):** aprovada. Conferido no código: `identity.go:159-166` (só `< 0x20` e os separadores), `identity.go:64-76` (o Load valida apenas o nickname e o display_name), `render.go:914-934` (o bloco é anexado sem olhar a representação, e `plan.go:79` chama a função para todo KindAgents). **Decisão:** UTF-8 inválido é **recusado com erro**, sem escrever arquivo, em linha com `render.go:665` ("callers must reject, not sanitize"). Por isso o AC7 foi ajustado. O YAML, a identidade e o T-4 entram nesta REQ, pela regra do KG de corrigir agora o defeito conhecido.

### ML-4A — Encoder de basic string TOML 1.0 (Codex)
**Status:** ✅ Concluído
**Agente:** prometeu-tf (mesmo despacho do ML-4B, porque os dois tocam `render.go`)
**Files affected:** `internal/integrations/render.go`, `internal/integrations/render_test.go`
**Acceptance criteria:**
- [x] As 4 chamadas de `strconv.Quote` do ramo `custom-agent-toml` passam por `tomlBasicString(s) (string, error)`. Escapa `\b \t \n \f \r \" \\` e `\uXXXX` para os demais C0, o DEL e os runes com `!strconv.IsPrint`. UTF-8 inválido devolve erro (`r == utf8.RuneError && size == 1`).
- [x] O teste tem um decodificador mínimo de basic string TOML 1.0, sem dependência nova. A saída do encoder decodifica de volta à entrada para 0x00–0x1F, 0x7F, C1, NBSP e acentos. A saída do `strconv.Quote` para 0x07 e 0x0B é recusada pelo mesmo decodificador (braço contrário). UTF-8 inválido vira erro, com o `Render` propagando esse erro.
- [x] Os goldens do Codex não mudam.
**Auditoria do arquiteto:** 174 PASS em `TestTOMLBasicString` e `TestYAMLEscapeDoubleQuotedNoRawControl`. Mordida: com `strconv.Quote`, reprovam `arm_a_round_trip` (todo C0 e o DEL) e `arm_c_invalid_utf8_error`. Goldens sem diff. Pacotes integrations, identity, thirdparty, commands e generators verdes com HOME isolado.

### ML-4B — Escape YAML double-quoted
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/render.go`, `internal/integrations/render_test.go`
**Acceptance criteria:**
- [x] `containsControlChar` passa a cobrir 0x7F e C1 (U+0080–U+009F). `yamlEscapeDoubleQuoted` emite escape YAML (`\xHH`, `\uXXXX` ou os escapes nomeados do YAML 1.2) para todo C0, DEL e C1, sem byte de controle cru.
- [x] O teste prova que nenhum byte < 0x20, 0x7F ou C1 sai cru numa scalar double-quoted e que o valor original pode ser recuperado. Antes da mudança, o teste reprova.
- [x] Os goldens atuais não mudam.
**Auditoria do arquiteto:** com o escape antigo, `new_encoder_no_raw_control_and_round_trips` reprova para todo C0 (exceto o tab), o DEL e o C1. Agora `containsControlChar` também recusa o U+0085; antes ele era excluído porque o yaml.v3 o normaliza, então nada muda na prática.

### ML-4C — `identity.Load` valida o slug e recusa DEL/C1
**Status:** ✅ Concluído
**Agente:** apolo-tf (em paralelo com 4A/4B, porque os arquivos são disjuntos)
**Files affected:** `internal/identity/identity.go`, `internal/identity/identity_test.go`
**Acceptance criteria:**
- [x] O `Load` recusa slug não vazio que não case com `slugPattern`, com mensagem no padrão das existentes.
- [x] `containsControlCharIdent` passa a recusar 0x7F e C1.
- [x] Os testes reprovam no código antigo (com slug contendo 0x07, display_name com 0x7F e com U+0085) e passam no novo. Os presets continuam carregando.
**Auditoria do arquiteto:** diff mínimo em `identity.go`. Os 8 subtestes de recusa reprovam no código antigo. O slug vazio continua aceito no Load (o `Validate` o recusa).

### ML-4D — Referências de terceiro respeitam a representação (T-4)
**Status:** ✅ Concluído
**Agente:** prometeu-tf (depois de 4A/4B, porque também toca `render.go` e `plan.go`)
**Files affected:** `internal/integrations/plan.go`, `internal/integrations/thirdparty_refs_repr_test.go`, `internal/commands/integrations_thirdparty.go`, `internal/commands/integrations_thirdparty_test.go`
**Acceptance criteria:**
- [x] Para `custom-agent-toml`, `agent-json` e `cli-agent-json`, o artefato final continua TOML ou JSON válido quando existem referências de terceiro. Ou o bloco entra no campo de instruções, ou a referência é recusada com erro explícito. Nunca é anexada ao arquivo serializado.
- [x] O teste reprova no código atual.
**Auditoria do arquiteto:** a 1ª entrega foi reprovada. Ela recusava com erro dentro do `BuildPlans`, o que derrubava o plano de todos os destinos: um registro antigo com entrada para o codex quebrava `agents update` e `update harness`, e o `--apply-to codex` deixava o registro envenenado. No corretivo, o `--apply-to` recusa representação estruturada antes de qualquer escrita, e o `BuildPlans` só deixa de anexar o bloco nesses destinos, sem falhar. Os testes `SkipBlockKeepValid`, `StaleRegistryDoesNotBreakOtherTargets` e `ApplyToRejectsStructuredRepresentationBeforeWrite` têm as 3 mordidas medidas. **Resíduo:** o plano não tem canal de aviso, então a omissão do bloco num registro antigo é silenciosa e só fica documentada no comentário.

### ML-4E — Revisão
**Status:** ⬜ Pendente
**Agente:** hefesto-tf e hades-tf
**Acceptance criteria:**
- [ ] Parecer de qualidade e red team dos MLs 4A–4D.

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf (execução de gate, sem código)
**Acceptance criteria:**
- [x] `make quality` com rc=0, com a máquina ociosa e o HOME isolado. Medido em 2026-10-22 no HEAD da branch: falsificação com 8 chunks, 368 OK / 0 FAIL, `packaging` e `pyyaml` por venv.
- [ ] CI do PR verde

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Vivo**: o "Dispatch contract" de `assets/agents/architect.md` e do golden só fala de `subagent_type`.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
