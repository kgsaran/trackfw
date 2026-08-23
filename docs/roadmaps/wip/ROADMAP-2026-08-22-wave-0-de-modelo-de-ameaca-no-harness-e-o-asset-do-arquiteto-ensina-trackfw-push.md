---
status: wip
date: 2026-08-22
req: "docs/req/REQ-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md"
adr: "docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md"
squad: "hades-tf, apolo-tf, prometeu-tf"
---

# Roadmap: Wave 0 de modelo de ameaça no harness, e o asset do arquiteto ensina `trackfw push`

> Created: 2026-08-22 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md`
ADR: `docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md`

Duas lacunas do harness: a revisão de segurança só chega no fim, e o asset do arquiteto não sabe que
`trackfw push` existe.

**Esta é a primeira REQ a nascer sob a regra nova — e a Wave 0 dela audita a própria Wave 0.** Se o
método não sobreviver à aplicação sobre si mesmo, é melhor descobrir agora.

## Acceptance Criteria

- [ ] AC1–AC11 da REQ, integralmente
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido, não "0 FAILs")
- [ ] `./bin/trackfw validate` sem violations novas

## Descoberta de desenho que já mudou o escopo

`trackfw barrier` **recusa `--wave 0`** hoje: `internal/commands/barrier.go:89` exige `waveInt >= 1`.
Chamar a wave nova de "Wave 0" sem mexer nisso a tornaria **inavaliável pela própria ferramenta** —
uma wave que o `barrier` não consegue abrir.

Decisão: **estender a gramática para aceitar 0**, em vez de renomear a wave. O rótulo carrega o
sentido (antes da implementação), o ADR já o usa, e renumerar empurraria implementação para Wave 2 em
todo roadmap futuro.

## Riscos que valem para todos os MLs

1. **Paridade de assets e de templates é byte-a-byte** entre `internal/integrations/assets/`,
   `npm/src/integrations/assets/` e `pypi/trackfw/integrations/assets/`. Editar um e esquecer os
   outros quebra o gate de artefatos.
2. **Isto é o harness — o que sair errado se instala em todo projeto que roda `trackfw update`.**
   Erro aqui não fica contido neste repositório.
3. `pypi/build/lib/trackfw/…` é árvore de build, **não** fonte. Não editar.
4. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality`, com **exit code**
   reportado — `grep FAIL` não vê abort por variável não ligada.
5. Commits, branch e PR são exclusivos do `trackfw_architect`.

---

## Wave 0 — Modelo de ameaça

> Dependências: nenhuma. **Bloqueia toda a implementação.**

### ML-0A — Modelo de ameaça do próprio método
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md`

Quatro seções, conforme o ADR:

1. **Completude de enumeração** — a lista de superfícies do harness está completa? Gerador de roadmap
   (3 CLIs, dois caminhos: `new` e `--from-req`), `barrier` (gramática + parser de waves), asset do
   arquiteto, asset de segurança, `CLAUDE.md` semeado. **O que falta nessa lista?** Considere:
   `roadmap new --from-req` derivando MLs de critérios de aceite; roadmaps **existentes** sem Wave 0;
   `trackfw update` em projeto que já tem `CLAUDE.md` customizado; e os 6 runtimes de agente com
   formatos distintos (Claude, Codex TOML, Cursor frontmatter, Gemini, Copilot, Kiro).
2. **Modelo de ameaça** — o adversário aqui **não** é um atacante externo: é o **agente com pressa** e
   o **arquiteto otimista**. Como cada um esvazia a Wave 0 sem violar nenhuma regra? Wave 0 escrita
   pelo próprio implementador; parecer de uma linha; Wave 0 copiada da REQ anterior; wave marcada
   `✅ Concluído` sem artefato. Qual desses o `barrier` pega, e qual passa?
3. **Alvos de falsificação nas duas direções** — enumere as sabotagens que o gate terá de detectar:
   (a) gerador deixa de emitir Wave 0; (b) `barrier --wave 0` volta a ser recusado; (c) asset perde a
   menção a `trackfw push`; (d) templates divergem entre os 3 CLIs. Para cada uma, diga **onde** a
   sabotagem entra e **qual gate** deveria acusar.
4. **Residual declarado** — o que este desenho aceita não cobrir. Em especial: a Wave 0 **raciocina
   sobre um artefato que ainda não existe** e não pode medir. O que isso deixa passar, estruturalmente?

**Critérios de aceite:**
- [x] As quatro seções, com a enumeração de superfícies **fechada** ou o que falta nomeado
- [x] Pelo menos uma via de esvaziamento do método que o `barrier` **não** pega, ou a prova de que
      não há
- [x] Alvos de falsificação com arquivo e gate correspondente — insumo direto do ML de gate
- [x] Nenhuma linha de implementação escrita (`git status --short` confirma 3 arquivos: roadmap,
      `agents-working-context.md`, `docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md`)

---

### Auditoria do ML-0A — **aprovada**, e a Wave 0 pagou o próprio custo antes de existir código

Parecer: `docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md`.

**Achados que mudaram o escopo, todos medidos e confirmados por mim:**

1. **`barrier` tem DOIS guardas contra `--wave 0`, não um.** O AC3 citava só a validação do flag
   (`:89`); `parseWaves` (~`:203`) repete `intVal < 1` sobre o cabeçalho do roadmap. Corrigir só o
   primeiro faria o comando passar da CLI e **falhar ao ler o próprio roadmap**. É literalmente a
   mesma classe do achado do `$PWD`/`~/`: duas formas do mesmo problema, só uma na lista.
2. **`roadmap new --from-req` não gera Wave 0** e usa rótulo `ML-1x` fixo — colidiria com os MLs
   derivados dos critérios de aceite. Virou **AC12**.
3. **Os quatro checks do `barrier` são satisfazíveis editando só o roadmap.** `gates` reporta
   `passed` quando a wave **não declara nenhum gate** (`parseGates` devolve lista vazia sem erro).
   Resultado: as cinco vias de esvaziamento — Wave 0 vazia, de uma linha, copiada da REQ anterior,
   escrita pelo implementador, ou marcada ✅ sem artefato — **passam limpas, sempre**. Virou **AC13**:
   o template pré-carrega um gate não-vazio.
4. 🔴 **E ele mesmo achou o perigo da própria sugestão.** `runGateCommand` (`:385`) executa via
   `exec.Command("sh","-c", …)` **sem sanitização**. Se o AC13 fosse implementado interpolando o
   título da REQ — como o gerador já faz com outros campos —, um título com backticks ou `$(...)`
   viraria **execução de shell dentro do harness**, instalada em todo projeto que roda
   `trackfw update`. A restrição "gate fixo, não interpolado" entrou no AC13 por causa disso.
5. **`check-artifact-parity.sh` só compara `go×node` e `go×python`** — nunca contra conteúdo
   esperado. Uma regressão **sincronizada** que remova `## Wave 0` dos três stacks passa em silêncio.
   Virou **AC14**.

**A resposta honesta que eu pedi, e ele deu:** uma Wave 0 **teria** pego o `~/` — era lacuna numa
tabela fechada, respondível por leitura. **Não** teria pego os outros três achados da mesma
reprovação (`${PWD}` silencioso, mensagem errada em runtime, aspas escapando dos checks), porque
exigem executar o código contra um caso concreto. A frase dele fecha a questão:

> **Wave 0 desloca a enumeração para a esquerda; ela não desloca a medição.**

**Residual que ele nomeia e fica fora desta REQ:** `barrier` não impõe ordem entre waves — "Wave 1
depende de Wave 0 auditada" é frase no roadmap, não checagem em código.

---

## Wave 1 — Harness

> Dependências: Wave 0 auditada. **ML único e não paralelizável:** os 3 stacks precisam sair
> byte-idênticos, e dividir por linguagem é o que produziu as divergências das séries anteriores.

### ML-1A — Wave 0 no gerador, no `barrier` e nos assets
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

**Arquivos afetados (âncoras medidas):**
- Gerador: `internal/generators/roadmap.go:113` (template `new`) e `:153` (`--from-req`) + os
  equivalentes em `npm/src/generators/` e `pypi/trackfw/generators/`
- Barrier: `internal/commands/barrier.go:87-92` (constraint `waveInt < 1`) e `parseWaves:186` +
  equivalentes nos 3 CLIs
- Assets: `internal/integrations/assets/agents/architect.md`,
  `internal/integrations/assets/agents/security.md` + cópias em `npm/src/integrations/assets/agents/`
  e `pypi/trackfw/integrations/assets/agents/`
- `CLAUDE.md` semeado: `internal/generators/claudemd.go:70` + equivalentes
- **Proibido:** `pypi/build/lib/…` (árvore de build), regras de `validate`, semântica de
  `commit`/`push`/`ship`

**Ações:**
1. Template de roadmap emite `## Wave 0 — Modelo de ameaça` com as quatro seções, nos dois caminhos.
2. `barrier` aceita `--wave 0`; parser reconhece o cabeçalho.
3. Asset do arquiteto: Wave 0 obrigatória antes de despachar implementação **e** autoridade de Git
   nomeando os três comandos (`commit` commita · `push` empurra · `ship` compõe). Hoje o arquivo tem
   **zero** ocorrências de `trackfw push`.
4. Asset de segurança: entregável da Wave 0.
5. `CLAUDE.md` semeado: diretiva *Security wave* cobrindo as duas pontas.

**Critérios de aceite:** AC1–AC8 da REQ · `make quality` exit 0 · assets byte-idênticos nos 3 CLIs

---

## Wave 2 — Gate

> Dependências: ML-1A auditado.

### ML-2A — Falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

Implementa os alvos enumerados pela Wave 0. Mínimo: gerador que deixa de emitir Wave 0 é detectado;
`barrier --wave 0` recusado é detectado. Baseline + braço de detecção, `cli-parity.md` atualizado.

**Critérios de aceite:** AC9, AC10, AC11 da REQ

---

## Wave 3 — Barreira

> Dependências: Wave 2 auditada.

### ML-3A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem escreveu a Wave 0 verifica se a implementação honra o que ela enumerou — e se as vias de
esvaziamento que ele apontou foram fechadas ou declaradas. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
