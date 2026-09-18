---
status: done
date: 2026-08-22
req: "docs/req/REQ-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md"
adr: "docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md"
squad: "hades-tf, apolo-tf, prometeu-tf"
---

# Roadmap: Wave 0 de modelo de ameaça no harness, e o asset do arquiteto ensina `trackfw push`

> Created: 2026-08-22 | Status: done

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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

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

**Critérios de aceite:**
- [x] AC1, AC2, AC12 — template emite `## Wave 0 — Threat Model` nos dois caminhos (`new` e
      `--from-req`), byte-idêntico nos 3 CLIs (provado por `diff`), ML sempre `ML-0A`
- [x] AC3, AC4 — `barrier --wave 0` aceito ponta a ponta nos 3 runtimes (prova real contra o
      roadmap desta REQ, não só fixture — ver `docs/agents-working-context.md`)
- [x] AC5 — asset do arquiteto: Wave 0 obrigatória + `trackfw push`/`commit`/`ship` nomeados
      (`grep -c "trackfw push"` = 2 nos 3 caminhos, era 0)
- [x] AC6 — asset de segurança: seção `## Wave 0 deliverable`
- [x] AC7 — `CLAUDE.md` semeado nas duas superfícies reais (`claudemd.go` **e**
      `agentfiles.go`/`trackfw:rules` block — a REQ só citava a primeira)
- [x] AC8 — assets byte-idênticos nos 3 CLIs (`diff` confirmado)
- [x] AC13 — gate não-vazio, fixo, sem interpolação (placeholder fail-closed, `exit 1`)
- [ ] `make quality` exit 0 — **exit 2**, isolado em `scripts/check-barrier.sh` Cenário 11
      (contrato antigo "Wave 0 = malformed"), antecipado e documentado; patch especificado para
      ML-2A. `go test`/`npm test`/`pytest` unitários: todos verdes (ver evidência abaixo)

---

## Wave 2 — Gate

> Dependências: ML-1A auditado.

### ML-2A — Falsificação nas duas direções
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

Implementa os alvos enumerados pela Wave 0. Mínimo: gerador que deixa de emitir Wave 0 é detectado;
`barrier --wave 0` recusado é detectado. Baseline + braço de detecção, `cli-parity.md` atualizado.

**Critérios de aceite:** AC9, AC10, AC11 da REQ

**Entregue (detalhe completo em `docs/agents-working-context.md`, sessão "FIM: ML-2A"):**
- `scripts/check-barrier.sh` Cenário 11 invertido: `## Wave 0` aceita e genuinamente avaliada (guard
  de vacuidade em evidence/commands, não só status), parity JSON byte-a-byte nos 3 runtimes.
- AC14: `scripts/check-artifact-parity.sh` ganhou asserção de conteúdo esperado (`## Wave 0 — Threat
  Model`, `**Gates da wave:**`, `ML-0A`) nos KINDS `roadmap`/`roadmap_flags`/`roadmap_from_req`,
  complementar ao diff cross-stack — provada load-bearing por sabotagem manual antes de escrever o
  cenário de falsificação. Achado fora de escopo reportado (não corrigido): `slash_roadmap`
  (`.claude/commands/trackfw/roadmap.md`) não ensina Wave 0 — sua fonte (`scaffold.go` +
  equivalentes npm/pypi) toca arquivos já marcados modificados pelo ML-1A, fora da fronteira deste ML.
- AC9: `scripts/check-gates-falsify.sh` Cenários 166 (Direção A — sabotagem sincronizada nos 3
  geradores via cópia isolada da árvore, detectada pela asserção do item acima, não pelo diff antigo),
  167 e 168 (Direção B — os **dois** guardas de `--wave 0`: 167 sabota `parseWaves`, 168 sabota a
  validação de flag em `newBarrierCmd`, ambos com `intVal/waveInt < 0` → `< 1`, ambos detectados pelo
  Cenário 11 invertido com mensagens de erro distintas — achado do próprio ML-0A, "barrier tem DOIS
  guardas", fechado no falsify só após revisão do advisor apontar que a primeira entrega cobria só
  um). Total 165→168, echo final. Achado colateral corrigido (fora do escopo Wave 0): Cenário 26
  (`s26-go`) pinava um literal de `roadmap.go` partido em dois `fmt.Sprintf` pelo ML-1A — só a
  referência do gate foi ajustada, não o arquivo.
- AC10: `docs/cli-parity.md`, seção `roadmap new <title>`, ganhou o bloco Wave 0 completo no exemplo
  de template + prosa (placeholder `exit 1` fail-closed, não-interpolação) e anotação
  `trackfw-contract` atualizada. Fence externo do exemplo alargado de 3 para 4 crases após revisão do
  advisor apontar que o fence interno (` ```bash `) de mesmo comprimento fechava o bloco externo
  prematuramente no CommonMark, vazando `## Wave 1`/`### ML-1A` como headings reais na renderização.

**Evidência medida:** `check-barrier.sh`/`check-artifact-parity.sh`/`check-gates-falsify.sh`/
`check-parity-contract-coverage.sh` todos exit 0; `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality`
exit 0 medido (`echo $? > arquivo`); `./bin/trackfw validate` exit 0, 16 warnings pré-existentes sem
relação com Wave 0.

---

### Auditoria do ML-1A e do ML-2A — aprovadas; e o teste de injeção foi o que eu mais queria ver

**ML-1A — auditoria por medição própria, incluindo o ataque que motivou o AC13:**

```
$ trackfw roadmap new 'teste $(touch /tmp/INJETADO) `id` fim'

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it
```
/tmp/INJETADO -> nao existe

barrier --wave 0   go_exit=0 · node_exit=0 · py_exit=0   (ponta a ponta, roadmap real)
asset do arquiteto  "trackfw push" x2 · "Wave 0" x2, nos 3 caminhos
```

**Ele entregou melhor do que o AC pedia:** eu escrevi "gate não-vazio"; ele fez `exit 1` —
**fail-closed**. Wave 0 gerada e não preenchida **reprova** no `barrier`, em vez de passar limpa. É o
que de fato fecha a via de esvaziamento que o ML-0A encontrou.

**E, pela primeira vez em sete entregas, ele mediu, achou vermelho e não escondeu:** `make quality`
saiu **exit 2** no cenário 11 do `check-barrier.sh`, que pinava o contrato antigo (*"Wave 0 é
malformada"*). Arquivo proibido para ele. **Reportou em vez de ajustar o gate para caber na própria
mudança**, que seria o pior desfecho possível.

Correção dele ao meu handoff: eu disse "dois guardas × 3 stacks = 6 pontos"; são **4** — só o Go tem
os dois.

**ML-2A — verifiquei a divulgação antes do resto.** Ele avisou que o cenário 168 reusa o braço de
baseline do 167 — e foi assim que o cenário 159 ficou vácuo nesta mesma série. Conferi: o braço de
**sabotagem** do 168 é independente e tem guard próprio (`sed 's/waveInt < 0 {/waveInt < 1 {/'` +
`cmp -s` + `FAIL [falsify/setup-s168]`). A prova não é herdada.

```
make quality (CI-exata, minha)   exit 0
check-barrier / artifact-parity / gates-falsify / contract-coverage   exit 0
validate                         16 warnings, 0 violations
falsificacao                     168 cenarios
```

---

## Wave 2-bis — A superfície que a Wave 0 não enumerou

> Dependências: ML-2A. **Bloqueia a Wave 3.**

### ML-1B — O slash command ainda ensina `## Wave 1`
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Achado do ML-2A, e é ele que decide se o método é real ou decorativo.**

`.claude/commands/trackfw/roadmap.md` — gerado de `internal/generators/scaffold.go:333` — ensina
`## Wave 1` como primeira wave, **sem nenhuma menção a Wave 0**.

**Por que bloqueia:** o gerador **não** é a superfície que governa a estrutura real dos roadmaps.
Todo roadmap desta sessão foi escrito à mão por cima do esqueleto. Quem ensina a estrutura é o slash
command. Somando ao resíduo já medido — Wave 0 manuscrita não traz bloco de gates, então `barrier`
reporta `gates: passed` —, o caminho **gerado** é fail-closed e o caminho **manuscrito** é
escancarado. E o slash command alimenta o segundo.

**Escopo:**
1. `internal/generators/scaffold.go:333` + equivalentes Node/Python: Wave 0 na estrutura ensinada,
   byte-idêntica nos 3 stacks.
2. Estender a asserção de conteúdo esperado (AC14) ao artefato `slash_roadmap` em
   `scripts/check-artifact-parity.sh`. O ML-2A o pulou **porque ele ainda não tinha Wave 0** —
   deixando sem pino justamente o artefato que mais ensina.
3. Decidir explicitamente o destino de `.claude/commands/trackfw/roadmap.md` **deste** repositório:
   regenerar, ou declarar o drift esperado do `doctor` como se fez com o `+1 warning` do guard.
   Surpresa de mismatch depois é o que não pode acontecer.

**Critérios de aceite:**
- [ ] Slash template com Wave 0, byte-idêntico nos 3 stacks
- [ ] `slash_roadmap` coberto pela asserção de conteúdo esperado
- [ ] `doctor` limpo **ou** drift declarado no roadmap com o motivo
- [ ] `make quality` CI-exata **exit 0**, exit code medido · `validate` sem violations novas

---

### Auditoria do ML-1B — aprovada, e a asserção nova é load-bearing por sabotagem minha

```
sabotagem: '## Wave 0 — Threat Model' -> '## Wave 1 — Implementation'  (scaffold.go, so Go)
  check-artifact-parity.sh -> EXIT 1
    artifact content drift: slash_roadmap (go) — arquivo gerado nao contem o
    literal esperado: ## Wave 0 — Threat Model
restaurado -> exit 0, scaffold.go IDENTICO ao entregue

make quality (CI-exata, minha)   exit 0
validate                         16 warnings, 0 violations
doctor                           no mismatches
```

**O que essa mensagem prova e o gate antigo não provava:** ela acusa **por conteúdo esperado**, não
por divergência entre stacks. Antes do AC14, uma regressão sincronizada nos três passaria calada — o
cenário realista de quem edita três arquivos com o mesmo `sed`.

**Honestidade dele que evitou um registro falso:** ele escolheu regenerar o
`.claude/commands/trackfw/roadmap.md` deste repo, mas declarou que o `doctor` estava limpo **antes e
depois**, porque ele cobre artefatos de agents/skills e **não** os assets do `scaffold.go`. A
regeneração foi higiene de dogfooding, não conserto de mismatch. Se tivesse escrito "regenerei para o
doctor ficar limpo", teríamos uma afirmação falsa no registro.

**Ponto cego novo, nomeado:** o `trackfw doctor` **não enxerga os assets do `scaffold.go`**. Se este
repositório tivesse ficado com o slash command defasado, nada acusaria.

---

### 🔴 O dado mais desconfortável desta entrega, e ele vai para a barreira

**A Wave 0 enumerou CINCO superfícies do harness e deixou passar a sexta — a mais importante.**

O slash command (`scaffold.go:333`) é o que realmente ensina a estrutura dos roadmaps, porque eles são
escritos à mão por cima do esqueleto. Ele não estava na enumeração do ML-0A. Quem tropeçou nele foi o
ML-2A, **escrevendo o gate** — ou seja, já na implementação.

Isso não invalida o método: a Wave 0 pagou o próprio custo com quatro ACs novos, dois deles
(injeção via `sh -c` e regressão sincronizada) de classe grave. Mas calibra a expectativa, e é
coerente com o que o próprio parecer declarou: **desloca a enumeração para a esquerda, não a torna
completa.**

---

## Wave 3 — Barreira

> Dependências: Wave 2 auditada.

### ML-3A — Reverificação
**Status:** ✅ Concluído — **APROVADO COM RESSALVAS** · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem escreveu a Wave 0 verifica se a implementação honra o que ela enumerou — e se as vias de
esvaziamento que ele apontou foram fechadas ou declaradas. **Veredito explícito.**

---

### Auditoria do ML-3A — **APROVADO COM RESSALVAS**, e a barreira achou o que ninguém tinha visto

Parecer: `docs/seguranca/2026-08-23-barreira-da-wave-0-no-harness.md`.

#### 🔴 Achado crítico — **reproduzi por medição própria**

O título de `roadmap new` é interpolado **sem sanitizar newlines**
(`internal/generators/roadmap.go:150`). Um título com quebras de linha **forja uma seção Markdown
inteira**, com bloco de gate próprio — e o `barrier` executa esse gate via `sh -c`:

```
titulo: "forjado\n\n## Wave 0 — Threat Model\n\n**Gates da wave:**\n```bash\ntouch /tmp/PWNED_TEST\n```"

roadmap gerado, linha 12:  **Gates da wave:**
                     13:  ```bash
                     14:  touch /tmp/PWNED_TEST

$ trackfw barrier <roadmap> --wave 0
  gates: passed
$ test -f /tmp/PWNED_TEST  ->  EXISTE      <- execucao confirmada
```

O `barrier` reportou `result: blocked` no conjunto — **e o comando forjado executou mesmo assim**,
porque os gates rodam antes de o veredito ser composto. "Bloqueado" não significa "não executou".

**É pré-existente** — o mecanismo de gates vem do `ADR-2026-07-26`, e a superfície é qualquer roadmap
com bloco de gates, não só o forjado por título. Esta REQ não o introduziu; estendeu a mesma
superfície para a Wave 0. Não bloqueia a entrega, **e precisa de REQ própria com urgência**.
Nota de vault escrita por ele:
`vault/notes/roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23.md`.

#### O diagnóstico que ele fez de si mesmo, e que vale mais que o veredito

Perguntei por que a Wave 0 dele deixou passar o slash command. Ele mediu e respondeu:

> Um `grep -rn "## Wave 1"` já retornava três ocorrências antes de qualquer código — duas na minha
> lista, **e o `scaffold.go:333` fora dela**. A seção 1 do meu parecer enumerou superfícies **a partir
> da lista que a própria REQ já nomeava** e perguntou "o que falta nessa lista". Isso achou lacunas
> **dentro** dos arquivos citados, mas nunca fez a pergunta inversa: que **outros** lugares do repo
> emitem esse artefato?

E propôs a correção no template: a seção "Completude de enumeração" pede *"a lista está completa?"* e
**não instrui como verificar**. Vira o **ML-1C** — o método aprendendo com a própria estreia.

#### Demais respostas
- **Via manuscrita:** continua aberta, medida ao vivo contra este próprio roadmap
  (`gates: passed` com `commands: []`). Residual já aceito pelo ADR.
- **AC13:** limpo nos vetores testados — `$(...)` e backtick nos 3 stacks, aspas e `;` em Go,
  `--from-req` em Go.
- **Sem regressão** em credential-guard nem git-branch-guard, conferido linha a linha nos 3 stacks.
- **`doctor` não cobre assets do `scaffold.go`:** confirmado no código, ponto cego real, REQ futura.

---

## Wave 4 — A lição da estreia entra no template

### ML-1C — "Completude de enumeração" passa a dizer COMO verificar
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

Acrescentar à Ação 1 do bloco de Wave 0 — em `internal/generators/roadmap.go` (`wave0Block`), no
slash command (`internal/generators/scaffold.go`) e nos equivalentes Node/Python — a instrução que
faltava: **não se limitar aos arquivos citados na REQ; buscar no repositório por outros pontos que
emitem o mesmo artefato ou padrão antes de declarar a lista fechada.**

**Critérios de aceite:**
- [x] Instrução presente nos dois templates, byte-idêntica nos 3 stacks
- [x] Asserção de conteúdo esperado do `check-artifact-parity.sh` cobre o texto novo
- [x] `make quality` CI-exata **exit 0**, exit code medido · `validate` sem violations novas

**Evidência:** o grep repositório-largo pelo próprio literal ("Enumeration completeness") — a
verificação que a instrução nova ensina a fazer — achou uma 7ª emissão fora da lista original do
handoff: `docs/cli-parity.md:3274`, uma cópia documental do bloco Wave 0 que havia ficado
desatualizada. Corrigida no mesmo ML (fora da lista de arquivos do handoff, mas dentro do escopo —
não está na lista de proibidos).

---

### Auditoria do ML-1C — aprovada na segunda medição; a primeira achou o pino pela metade

**O que ele entregou, e é o fecho do ciclo:** a instrução nova diz *"não se limite aos arquivos
citados; busque no repositório pelo literal do artefato final"*. Ele **rodou a própria instrução**
(`grep -rn "Enumeration completeness"`) e achou um **oitavo ponto de emissão** que nem o handoff nem
a barreira tinham enumerado — um espelho desatualizado do bloco de Wave 0 em `docs/cli-parity.md`.
O método encontrou uma superfície aplicando a lição da própria falha.

**O furo que a minha auditoria pegou:** o pino cobria só a **primeira metade** da frase.

```
sabotagem sincronizada nos 3 stacks:
  'search the repository for other places' -> SABOTADO   gate EXIT 0   <- passava
  'Do not limit the search to the files…'  -> SABOTADO   gate EXIT 1   <- pinada
```

A metade pinada era o *"não faça"*; a **metade operativa** — o *como* verificar, que é o valor
inteiro do ML — estava solta. Só apareceu porque sabotei os **três** stacks de uma vez: sabotar só o
Go dispara o eixo cross-stack antes e mascara a lacuna.

Corrigido no ML-1C-bis, e reverificado por mim:

```
sabotagem sincronizada da metade operativa -> EXIT 1
  artifact content drift: nao contem o literal esperado:
  search the repository for other places that emit the same artifact...
restaurado -> EXIT 0
```

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
