---
status: done
date: 2026-08-21
req: "docs/req/REQ-2026-08-21-versao-do-modelo-dos-agentes-configuravel-por-tier-no-trackfw-yaml.md"
adr: "docs/adr/ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: versão do modelo por tier, com composição por alvo

> Created: 2026-08-21 | Status: done

## Context

REQ: `docs/req/REQ-2026-08-21-versao-do-modelo-dos-agentes-configuravel-por-tier-no-trackfw-yaml.md`

O usuário não pode escolher a versão do modelo dos agentes. Pinar exige editar arquivo **gerado**, e
o próximo `agents update` reverte sem aviso. O conflito já é concreto: a regra de verbosidade do
#198 só chega ao arquivo local via update, e o update desfaz o pin.

## 🔴 Riscos que valem para todos os MLs

1. **Vazamento de namespace é o risco dominante.** `claude-sonnet-4-6` chegando ao mapeamento do
   Codex, Cursor ou Antigravity quebra os três — e quebra no **artefato gerado**, não no `trackfw`,
   então o usuário só descobre quando o agente não sobe. Precisa ser **gate**, não cuidado.
2. **Config ausente não pode mudar nada.** Sem `agent_models`, comportamento idêntico ao de hoje.
   Regressão aqui atinge todo usuário do trackfw.
3. **O motivo é cota, não custo.** Sonnet 4.6 consome ~30% menos tokens (tokenizador pré-4.7) e custa
   **mais** por token. Sem isso escrito, um leitor futuro "corrige" a escolha para o lado errado.
4. **Invocação CI-exata:** `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity`.

---

## Wave 1 — Decisão e composição

### ML-1A — ADR do formato e da composição
**Status:** ✅ Concluído · **Agente:** `zeus-tf` (arquiteto — **não delegar**)
`ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md`, com o formato, as três regras
de composição, o escape hatch, a fronteira de namespace, e o motivo (**cota, não custo**) registrado
com a medição da doc oficial.
Decisão material: formato de `agent_models`, as três regras de composição, o escape hatch, e a
fronteira de namespace. Decisão de formato é do arquiteto; o roadmap anterior atribuiu isso a
executor por engano e foi corrigido.

### ML-1B — Resolução e composição por alvo
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A
**Arquivos (3 stacks):** leitura de config, `internal/integrations/render.go` + espelhos, testes.

- `agent_models` por tier, guardando **versão**.
- Composição: ponto→traço; versão maior **omite o minor**; cada alvo compõe a **própria** forma.
- Escape hatch: valor que não parece versão é usado **literalmente**.

**Critérios de aceite:**
- [ ] As três regras de composição corretas, provadas por caso
- [ ] **Sem vazamento**: Codex, Cursor e Antigravity seguem com os próprios valores mesmo com
      `agent_models` configurado — é o AC que mais importa
- [ ] Config ausente → comportamento idêntico ao de hoje
- [ ] `make quality` verde

---

---

### Auditoria do ML-1B — aprovada, com uma lacuna pequena para o ML-2A

Verifiquei **end-to-end**, gerando artefatos reais com os 3 alvos, não pelos testes dele:

```
                        sem agent_models        com agent_models
Claude   model:         sonnet              ->  claude-sonnet-4-6     COMPOE
Codex    model =        "gpt-5.4-mini"      ->  "gpt-5.4-mini"        INTOCADO
Cursor   model:         composer-2.5[...]   ->  composer-2.5[...]     INTOCADO
```

**Zero vazamento de namespace** — o AC que mais importava do lote, provado no artefato gerado e não
só em teste unitário.

As três regras de composição, cada uma com caso próprio:

```
"5"                            -> claude-opus-5 / claude-sonnet-5      (maior sem minor)
"4.6"                          -> claude-opus-4-6 / claude-sonnet-4-6  (ponto vira traco)
"claude-sonnet-4-5-20250929"   -> literal                              (escape hatch)
"4.6-beta"                     -> literal                              (escape hatch)
```

**Escape hatch com critério explícito:** `^[0-9]+(\.[0-9]+)*$`. Trade-off documentado e coerente
com o que pedi — prefere falso-negativo (tratar como literal) a falso-positivo (compor errado a
partir de algo que não era versão).

#### 🔴 Lacuna que encontrei, e vai para o ML-2A

`"4.6-beta"` vira **literalmente** `model: 4.6-beta` no frontmatter. É o escape hatch funcionando
como especificado — mas o resultado é um valor de modelo **inválido**, escrito em silêncio. O agente
falha ao subir, e a causa fica a duas camadas de distância.

É a mesma classe do `reson=` que a REQ do contrato pinado tratou: entrada de forma desconhecida
aceita sem sinal. O AC da REQ pede que a resolução *"não falhe de forma obscura"*, e este caminho
falha.

**Não bloqueia**, e o remédio já tem lugar natural: o **ML-2A** entrega o comando de resolução
efetiva, que é exatamente onde isso deve aparecer. Acrescentado como critério lá — avisar quando o
valor não é versão **nem** parece ID de modelo (não começa com `claude-`).


## Wave 2 — Visibilidade e catálogo

### ML-2A — Comando de resolução efetiva
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-1B
Lista, por agente e por alvo, o modelo **efetivamente resolvido**. Sem isso o usuário configura e não
confirma — foi exatamente a situação em que ninguém sabia dizer qual modelo os agentes usavam.

**Critérios de aceite:**
- [x] Saída mostra agente · tier · alvo · valor resolvido
- [x] **Avisa** quando o valor configurado não é versão **nem** parece ID de modelo (não começa com
      `claude-`) — hoje `"4.6-beta"` vira `model: 4.6-beta` em silêncio, e o agente falha ao subir
      com a causa a duas camadas de distância (lacuna medida na auditoria do ML-1B)
- [x] Byte-idêntica nos 3 CLIs
- [x] `make quality` verde (Go/Node.js/Python todos verdes; parity harness passa)

**Superfície:** `trackfw agents models` (subcomando de `agents`, gate `kind == KindAgents`)
**Arquivos criados:**
- `internal/integrations/models.go` — `ResolveAgentModel`, `LooksLikeSuspectModelValue`, `AgentTier`, `DefaultAgentSurface`
- `internal/integrations/models_test.go` — 3 testes Go incluindo drift gate
- `internal/commands/agents_models.go` — command implementation
- `internal/commands/integrations_flags.go` — registro para KindAgents
- `npm/src/integrations/render.js` — `resolveAgentModel`, `looksLikeSuspectModelValue`
- `npm/src/commands/integrations.js` — subcomando `models` + `createAgentModelsCommand`
- `npm/tests/agents_models.test.js` — 17 testes Node.js
- `pypi/trackfw/integrations/renderers.py` — `resolve_agent_model`, `looks_like_suspect_model_value`
- `pypi/trackfw/integrations/command.py` — subcomando `models` + `_run_models`
- `pypi/tests/test_agents_models.py` — 30 testes Python incluindo drift gate

---

### Auditoria do ML-2A — aprovada; a lacuna do ML-1B ficou visível

Verifiquei os cinco casos eu mesmo, com o binário recém-compilado:

```
4.6                          warn=0  ->  claude-sonnet-4-6
5                            warn=0  ->  claude-sonnet-5
claude-sonnet-4-5-20250929   warn=0  ->  literal
4.6-beta                     warn=1  ->  literal          <- a lacuna, agora anunciada
<sem config>                 warn=0  ->  sonnet           <- comportamento de hoje preservado
```

O aviso nomeia o problema por extenso, em vez de apenas sinalizar: *"not a version string and not a
`claude-` model ID; will be written literally and may produce an invalid model identifier"*. Quem
receber isso sabe o que fazer sem abrir o código.

**Nenhum falso-positivo nos três valores legítimos** — era o risco que eu nomeei no handoff, porque
aviso barulhento treina o usuário a ignorar, e aí perde-se o aviso que importa.

**Decisão dele que eu não pedi, e que fecha um buraco que eu não tinha visto:** criou um *drift gate*
(`TestResolveAgentModelMatchesRender`) provando que o valor **relatado** pelo comando é o que o
`Render()` de fato **escreveria**. Sem isso, o comando poderia divergir do render em qualquer
mudança futura — e **um comando de inspeção que mente é pior que não ter comando**, porque o usuário
confia nele em vez de conferir o artefato.

Superfície escolhida: `trackfw agents models`, estendendo o grupo existente em vez de criar
superfície nova. Correto.

`make quality` (CI-exata) exit 0 · cobertura exit 0 · `validate` exit 0.


### ML-2B — Catálogo pina as versões
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-2A
`agents update` passa a **reforçar** o pin em vez de desfazê-lo.

**Critérios de aceite:**
- [x] Após `agents update`, os arquivos gerados trazem as versões pinadas
- [x] Provado end-to-end com os 3 binários, em fixture com `HOME` redirecionado
- [x] `make quality` verde

**Auditoria:**
Corrigidas 3 construções de `PlanRequest` que omitiam `AgentModels` em `update.go` (linhas 150 e
1961 project-scope; linha 1718 harness — alinhado ao espelho npm), 2 assinaturas em
`integrations/doctor.go` (`RunDoctor`/`doctorPlansForScope`), chamador em `commands/doctor.go`
(novo import `config`), teste em `doctor_test.go`, e espelhos npm (`integrations/doctor.js`,
`commands/doctor.js`). Corrigido também um bug adjacente de ML-1B em `config.ReadAgentConventions`
que não inicializava `AgentModels` antes de chamar `parse()`, causando panic com nil map quando
`agent_models` estava em `trackfw.yaml`.

**Prova end-to-end (3 binários, HOME redirecionado, fixture com `agent_models: sonnet: "4.6", opus: "5"`):**
```
Go:     trackfw-architect.md: model: claude-opus-5   | trackfw-backend.md: model: claude-sonnet-4-6
npm:    trackfw-architect.md: model: claude-opus-5   | trackfw-backend.md: model: claude-sonnet-4-6
Python: trackfw-architect.md: model: claude-opus-5   | trackfw-backend.md: model: claude-sonnet-4-6
```
`make build` exit 0 · `make test` exit 0 · `make quality` (458 OK, 0 FAIL) · `trackfw validate` 17 warnings pré-existentes (0 violations) · `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity` exit 0.

---

---

### Auditoria do ML-2B — aprovada no escopo, **com um defeito de gravidade alta subclassificado**

**O AC central passou**, verificado por mim end-to-end com os 3 binários e `HOME` redirecionado:

```
go    architect=claude-opus-5   backend=claude-sonnet-4-6
node  architect=claude-opus-5   backend=claude-sonnet-4-6
py    architect=claude-opus-5   backend=claude-sonnet-4-6
```

**O pin sobrevive ao `agents update`** — o defeito que originou a REQ está fechado.

#### 🔴 O panic, e duas falhas de processo

Ele corrigiu um `nil map` em `config.go` e chamou de *"bug adjacente ML-1B"*, num parágrafo do meio
do relatório. **Panic não é nota de rodapé.**

Reproduzi — precisou do caminho certo, com `CLAUDE.md` presente para que a geração de regras seja
alcançada em vez de pulada:

```
sem o fix:  trackfw update  ->  panic, stack ate root.go:94
com o fix:  normal
```

E **sobrou uma**. Confirmei com teste dedicado:

```
config.go:181  ReadAgentConventions   -> corrigido no ML-2B
config.go:165  ParseRulesFromContent  -> AINDA PANICA
   "PANIC com agent_models: assignment to entry in nil map"
```

Nona vez nesta série do padrão **"condição estreita demais"**: consertou-se a ocorrência, não a
classe.

**Falha minha, na auditoria do ML-1B:** exercitei `agents install` e `agents models` — nos dois, o
caminho da geração de regras é **pulado**. Testei o que a **feature nova** usa, não o que a **mudança
de struct** atinge. Campo novo em struct compartilhado tem raio de alcance maior que a feature que o
motivou, e eu não considerei isso.

**Falha do gate:** `make quality` ficou verde com o panic presente. Nenhum teste cobria `parse()`
com `agent_models` a partir das construções afetadas.

Aberta a `REQ-2026-08-21-nil-map-em-construcao-de-projectconfig-causa-panic-quando-agent-models-esta-configurado`,
com AC exigindo **fechar a classe** — construtor único ou init defensivo no `parse` — e não a
instância.

**Achado dele que vale registrar:** o `update-harness.js` do Node já passava `agentModels` enquanto o
Go não passava. Ele alinhou o Go ao espelho. Divergência silenciosa de comportamento entre CLIs, do
tipo que só aparece quando alguém compara linha a linha.


### ML-2C — Fechar a classe do nil map (corretivo, **antes do merge**)
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Bloqueia o ML-3A.**

**Decisão de sequenciamento, de KG:** resolver antes de seguir. E com uma correção minha — o panic
**ainda não está na `main`**: o campo `AgentModels` foi introduzido pelo ML-1B **nesta branch**.
Fixar aqui evita publicar um panic conhecido, então isto vira ML corretivo desta REQ em vez de
trabalho separado.

A `REQ-2026-08-21-nil-map-em-construcao-de-projectconfig-causa-panic-...` fica como registro da
classe e do processo que falhou; a correção acontece aqui.

**Critérios de aceite:** ver a REQ. O essencial: **fechar a classe, não a instância** — construtor
único ou init defensivo no `parse`, com a decisão registrada.


### Auditoria do ML-2C — aprovada; a classe **está** fechada, e provei

Meu critério era: *"se a solução exige que alguém lembre de algo, não fechou a classe"*. Testei
exatamente isso — acrescentei um campo de mapa **novo** ao struct, **sem tocar em nenhuma
construção**, e verifiquei se ele nasce inicializado:

```go
Rules            map[string]string
AgentModels      map[string]string
CampoNovoDeTeste map[string]string   <- adicionado so no teste

parse(...) -> cfg.CampoNovoDeTeste != nil
go test ... -run TestCampoNovoDeMapaEhInicializado  ->  ok
```

**Passou.** A varredura por reflexão em `initConfigMaps`, chamada na primeira linha do `parse()`,
inicializa qualquer campo de mapa nil — inclusive os que ainda não existem. O próximo campo não
reintroduz o defeito, e ninguém precisa lembrar de nada.

Confirmou também que a única construção quebrada era `ParseRulesFromContent`; a outra já tinha sido
corrigida no ML-2B.

**Node e Python imunes por construção** — todo literal de config já inicializa o mapa. Ele declarou
por escrito em vez de mexer sem necessidade, que era o pedido.

**Decisão de desenho do P4 que vale nota:** ele descartou a abordagem via `validate` + git HEAD
porque o subprocesso git não era determinístico no fixture, e passou a corromper o `config.go`
removendo a chamada e rodar o teste Go contra a cópia. Trocou uma prova instável por uma estável em
vez de conviver com intermitência — gate que falha às vezes é pior que gate ausente.

`make quality` (CI-exata) exit 0 · 155 cenários · `validate` exit 0.


## Wave 3 — Gate

### ML-3A — Gate de paridade + P4
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **Dep.:** ML-2B
**Antes de criar gate novo, verificar se algum existente cobre** — nesta série um comparador paralelo
quase foi criado sem necessidade.

**Critérios de aceite:**
- [ ] Gate compara as **três saídas reais**, incluindo o caso de não-vazamento
- [ ] Cenário P4 com baseline e detecção
- [ ] Anotação `trackfw-contract` atualizada; checker de cobertura exit 0
- [ ] `make quality` verde · **CI verde**

---

### Auditoria do ML-3A — aprovada; o P4 mira o defeito certo

Sabotei a fronteira de namespace eu mesmo — removi o `targetID == "claude" &&` do guard:

```
gate -> EXIT=1
  FAIL [no-namespace-leak/go/gemini]: trackfw-architect.md changed when agent_models
       was added (namespace leak!)
  ... e o mesmo para backend, code-quality e os demais
restaurado -> EXIT=0
156 cenarios · make quality (CI-exata) exit 0 · cobertura exit 0 · validate exit 0
```

**Ele escolheu o alvo de sabotagem que eu pedi, e melhor do que eu especifiquei.** Eu disse para
mirar o não-vazamento em vez da composição. Ele foi além: o alvo é o **Gemini**, que cai no `default:`
do switch e **não tem proteção por `targetID`** — ou seja, é o alvo onde o vazamento apareceria
primeiro se a fronteira cedesse. Sabotar contra o Codex teria sido mais óbvio e menos revelador.

**Guardas de vacuidade por runtime** (`vacuity-codex`, `vacuity-gemini`) garantem que o cenário não
passe comparando ausência com ausência — a mesma classe de furo que o `deniedCommands` teve.

**Gate criado, não estendido, com justificativa** — nenhum dos candidatos que apontei cobria a
combinação de composição e não-vazamento no artefato gerado. Aceito.

**Armadilha que ele documentou, e é sutil:** o braço de detecção precisa rodar o **script** a partir
do `ROOT_DIR` real, não da árvore sabotada — porque `NODE_CLI` e `PY_ROOT` derivam dele, e o `set -e`
mataria o script antes da comparação. Isolar o **binário** é correto; isolar também o **script que
dirige os CLIs** é erro, e o sintoma é um braço de detecção que passa por não chegar ao ponto.


## Wave 4 — Barreira

### ML-4A — `hades-tf`
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md`
Config do usuário passa a influenciar o que é escrito em arquivo de agente. Avaliar injeção via valor
de versão, e se o escape hatch permite escrever algo perigoso no frontmatter. **Veredito explícito.**

---

### Auditoria do ML-4A — **BLOQUEIO ACEITO**, e reproduzi as duas variantes

Parecer: `docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md`. Verdito **BLOQUEAR**.

**Reproduzi eu mesmo, com `HOME` redirecionado:**

```
agent_models.sonnet = "claude-sonnet-4-6\ntools: Bash"
  -> model: claude-sonnet-4-6
     tools: Bash                    <- CHAVE DUPLICADA, valor do atacante
     ...
     tools: Read, Edit, Write, ...

agent_models.sonnet = "claude-sonnet-4-6\n---\nINSTRUCAO INJETADA NO CORPO"
  -> model: claude-sonnet-4-6
     ---                            <- FRONTMATTER FECHADO
     INSTRUCAO INJETADA NO CORPO    <- conteudo no CORPO do agente
  aviso emitido: 0
```

**A segunda é a grave.** O arquivo de agente é **instrução executável** para um assistente. Fechar o
frontmatter e escrever no corpo é injeção de instrução, não config malformada — e sai **sem aviso**,
porque o `looksLikeSuspectModelValue` checa apenas o prefixo `claude-`. **O prefixo compra o
silêncio.**

**Eu apontei esta superfície no handoff** — *"o arquivo de agente é instrução executável; conteúdo
controlado que chega lá é superfície de prompt injection"*. Apontei porque desconfiei, não porque
sabia. Ele mediu e provou. É o valor da barreira: eu levanto a hipótese, ele decide se é real.

**Segundo achado, alto e não crítico:** `update harness` lê o `trackfw.yaml` do **cwd** e escreve em
`~/.claude/agents/`, que vale para todos os projetos da máquina. Um `trackfw.yaml` hostil num
diretório qualquer alcança escopo global. Medido nos 3 CLIs.

**A guarda de namespace foi verificada e aprovada sem ressalva** — Codex e Antigravity nunca leem
`agentModels`. O que eu mais temia não era o problema.

**Mitigação que ele propôs, e aceito:** rejeitar caracteres de controle (`\n`, `\r`, `\x00-\x1F`)
antes de escrever. Preserva o escape hatch, e o argumento é sólido: **ID de modelo nunca precisa de
newline**, qualquer que seja a mudança futura de formato. A objeção do ADR sobre formato não se
aplica a este controle.

---

## Wave 5 — Corretiva do bloqueio

### ML-5A — Rejeitar caracteres de controle no valor de modelo
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Bloqueia o merge.**

**Critérios de aceite:**
- [x] Valor com `\n`, `\r` ou controle é **recusado**, nos 3 CLIs — não sanitizado em silêncio
- [x] As duas variantes que reproduzi deixam de produzir artefato adulterado — provadas
- [x] Escape hatch legítimo (`claude-sonnet-4-5-20250929`) **continua funcionando**
- [x] Cenário P4 para as duas variantes (Case 5a + 5b em `check-agent-models-parity.sh`)
- [x] Decisão sobre o segundo achado: DEFERIDO com motivo registrado na nota de vault
- [x] `make quality` verde · CI-exata verde (pendente conclusão — executando em BG)

**Arquivos modificados (3 stacks):**
- Go: `internal/integrations/render.go` (helper `containsControlChar` + assinatura `([]byte, error)`),
  `internal/integrations/models.go` (`LooksLikeSuspectModelValue` flags controle),
  `internal/integrations/render_test.go` (+2 testes rejeição), `internal/integrations/models_test.go` (+2 casos)
- Node.js: `npm/src/integrations/render.js` (`containsControlChar` + throw), `npm/tests/agents_models.test.js` (+5 testes)
- Python: `pypi/trackfw/integrations/renderers.py` (`_contains_control_char` + ValueError + `looks_like_suspect_model_value`),
  `pypi/tests/test_agents_models.py` (+3 testes)
- Gate: `scripts/check-agent-models-parity.sh` (Case 5a + 5b, 2 variantes × 3 runtimes)
- Vault: `vault/notes/rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21.md` (decisão + correção)

**Contagem de cenários:** 156 → 163 (+ 7 novas OK lines no gate parity: 6 rejeições + 1 vacuity)

### Auditoria do ML-5A — aprovada; as duas injeções recusadas, os dois caminhos legítimos intactos

Verifiquei os quatro casos eu mesmo:

```
chave-duplicada   exit=1  <sem artefato>                 injecao=0
corpo-injetado    exit=1  <sem artefato>                 injecao=0
escape-legitimo   exit=0  claude-sonnet-4-5-20250929     injecao=0
versao-normal     exit=0  claude-sonnet-4-6              injecao=0
```

**Recusa, não sanitização** — nenhum artefato é gerado. Era o que eu tinha pedido: sanitizar
produziria um valor que o usuário não escreveu e não consegue explicar.

E o escape hatch **continua funcionando**, que era o risco de over-fix. A correção mira o caractere
de controle, não o mecanismo.

**A decisão sobre o segundo achado é dele, e endosso**, com o argumento que ele escreveu:
a correção do caractere de controle **elimina a classe de dano mais grave**. Depois dela, a pior
saída de um `trackfw.yaml` hostil é um ID de modelo arbitrário **de uma linha só** — não mais
injeção de instrução no corpo do agente. Uma ordem de magnitude menos severo, e o resto é mudança de
comportamento com raio amplo que merece ciclo próprio.

Virou `REQ-2026-08-21-update-harness-le-trackfw-yaml-do-cwd-e-escreve-em-escopo-global`, com o risco
dominante nomeado: **o caso legítimo e o hostil têm a mesma forma** — `trackfw.yaml` no cwd —, e a
distinção precisa de critério, não de heurística frouxa.

`make quality` (CI-exata) exit 0 · cobertura exit 0 · `validate` exit 0.


### ML-5B — Reverificação do `hades-tf`
**Status:** ✅ Concluído · **Agente:** `hades-tf` · **Dep.:** ML-5A. Quem bloqueou levanta.

### Auditoria do ML-5B — bloqueio levantado; a lacuna que eu previ é real

Veredito: **LEVANTADO COM RESSALVAS**
(`docs/seguranca/2026-08-21-reverificacao-da-configuracao-de-modelo.md`).

Os dois exploits dele foram reproduzidos contra o binário atual e **bloqueados**. Paridade nos 3
CLIs confirmada por ele, testando `rewriteFrontmatterModelLine` direto em cada runtime.

**A ressalva é a lacuna que eu levantei no handoff**, e confirmei por medição própria:

```
agent_models.sonnet = "claude-sonnet-4-6<U+2028>tools: Bash"
  exit=0
  model: claude-sonnet-4-6\u2028tools: Bash    <- ESCRITO
  contem U+2028: True
```

`containsControlChar` usa limite `< 0x20`; os bytes UTF-8 de U+2028 são todos `>= 0x80`, então
passam. O parser YAML do Go **preserva** o caractere em vez de normalizá-lo.

**Ele mediu o que dava para medir e marcou o resto como inferido** — o impacto depende do loader de
frontmatter: parser que divide por `\n` produz ID inválido (disponibilidade); parser YAML 1.2 que
trate U+2028 como separador produziria injeção estrutural. O loader real não é mensurável daqui, e
ele disse isso em vez de afirmar o pior caso.

**Decisão: corrigir agora, não virar REQ.** É extensão do mesmo check, com o **mesmo argumento já
aceito** — ID de modelo nunca precisa de separador de linha, qualquer que seja o formato futuro.
Abrir REQ para cinco linhas custaria mais em processo do que o trabalho.

**`CR` sozinho:** ele mediu que o parser normaliza para espaço — inócuo. Registro porque é o tipo de
caso que se presume perigoso por analogia e não é.

---

### ML-5C — Estender a recusa a separadores unicode
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)

**Critérios de aceite:**
- [x] U+2028 e U+2029 recusados nos 3 CLIs, com a mesma mensagem dos de controle
- [x] Provado: o caso que hoje sai `exit=0` passa a sair `exit != 0`, sem artefato
- [x] Valores legítimos e acentuação comum **não** afetados — provado com `claude-sonnet-4-6-café`
- [x] `LooksLikeSuspectModelValue` também sinaliza (delega a `containsControlChar`)
- [x] Cenário P4 para o caso novo (Case 5c no gate, U+2028 × 3 runtimes + vacuity)
- [x] `make quality` verde · CI-exata verde

**Arquivos modificados (3 stacks):**
- Go: `internal/integrations/render.go` (`containsControlChar` → rune loop + U+2028/U+2029),
  `internal/integrations/render_test.go` (+2 testes: rejeição U+2028/U+2029 + aceite acento),
  `internal/integrations/models_test.go` (+3 casos em `LooksLikeSuspectModelValue`)
- Node.js: `npm/src/integrations/render.js` (`containsControlChar` + `c === 0x2028 || c === 0x2029`),
  `npm/tests/agents_models.test.js` (+6 testes ML-5C)
- Python: `pypi/trackfw/integrations/renderers.py` (`_contains_control_char` + `ord(c) in (0x2028, 0x2029)`),
  `pypi/tests/test_agents_models.py` (+5 testes ML-5C)
- Gate: `scripts/check-agent-models-parity.sh` (Case 5c: U+2028 × 3 runtimes + vacuity guard)
- Vault: `vault/notes/rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21.md` (seção ML-5C adicionada)

**Pontos de código U+2028 e U+2029 — justificativa por medição:**
- U+2028/U+2029: yaml.v3 preserva verbatim no valor Go string (rune[17]=U+2028, len=31). Confirmado
  por `go run` com yaml.v3. Parsers de frontmatter baseados em linha tratam como separador → injeção
  estrutural. INCLUÍDOS.
- U+0085 (NEL): yaml.v3 normaliza para espaço antes de retornar o valor. O resultado é
  `claude-sonnet-4-6 tools: Bash` (espaço literal). Sem injeção estrutural. EXCLUÍDO com medição.

**Caso legítimo provado não afetado:** `claude-sonnet-4-6-café` aceito (U+00E9 não é separador).

**Evidência CI-exata:**
```
OK   [unicode-linesep/U+2028/go/install-rejected]
OK   [unicode-linesep/U+2028/node/install-rejected]
OK   [unicode-linesep/U+2028/py/install-rejected]
OK   [unicode-linesep/vacuity/yaml-fixture-contains-u2028-escape]
All check-agent-models-parity.sh scenarios passed (...unicode-separator rejection...)
TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make parity  →  exit 0, 0 FAILs
./bin/trackfw validate  →  17 warnings pré-existentes, 0 violations
```

### Auditoria do ML-5C — aprovada; sem over-fix

```
U+2028      exit=1  <sem artefato>
U+2029      exit=1  <sem artefato>
acentuado   exit=0  claude-sonnet-4-6-cafeé     <- legitimo, intacto
versao-ok   exit=0  claude-sonnet-4-6
```

O risco que eu nomeei era **recusar acentuação por paranoia**. Não aconteceu — o check é sobre
separadores de linha, não sobre não-ASCII.

**A decisão sobre U+0085 (NEL) é o melhor do lote.** Eu tinha citado o NEL no handoff como candidato,
e ele **excluiu — por medição**: o `yaml.v3` normaliza U+0085 para **espaço** antes de devolver o
valor, então não há injeção estrutural. Ampliar o bloqueio ali seria proteção nominal contra um vetor
que não existe, e cada codepoint bloqueado sem evidência é um falso-positivo esperando acontecer.

Ele fez o mesmo com o `CR` na reverificação anterior. Duas vezes seguidas medindo antes de bloquear,
contra uma sugestão minha nas duas. É a postura certa: **eu levanto a hipótese, quem mede decide**.

`make quality` (CI-exata) exit 0 · cobertura exit 0 · `validate` exit 0.

**Wave 5 fechada. REQ pronta para PR.**

## Notas
- **Fora de escopo:** trocar o tier de um agente; mudar mapeamento de Codex/Cursor/Antigravity;
  modelo por agente individual.
- Commits e branch são exclusivos do `trackfw_architect`.
