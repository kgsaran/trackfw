---
title: Modelo de ameaça — Wave 0 no harness (a Wave 0 audita a própria Wave 0)
date: 2026-08-22
author: hades-tf
req: docs/req/REQ-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
roadmap: docs/roadmaps/wip/ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push.md
adr: docs/adr/ADR-2026-08-22-modelo-de-ameaca-no-desenho-wave-0-de-red-team-antes-da-implementacao-no-harness.md
ml: ML-0A
---

# Modelo de ameaça da Wave 0 — ML-0A

> Nenhuma linha de implementação foi escrita para produzir este parecer. Cada afirmação está marcada
> **[medido]** (lida no código/repo tal como está hoje) ou **[raciocinado]** (inferência sobre um
> artefato que ainda não existe).

## 1. Completude de enumeração

A lista da REQ/roadmap — gerador de roadmap (3 CLIs, `new` e `--from-req`), `barrier` (gramática +
parser), asset do arquiteto, asset de segurança, `CLAUDE.md` semeado — é a superfície certa, mas
**incompleta em cinco pontos**. Para cada um: o que falta e o veredito proposto.

### 1.1 `barrier` tem dois pontos de bloqueio para `--wave 0`, não um **[medido]**

A REQ (AC3) e o roadmap citam só `internal/commands/barrier.go:89` (validação do flag `--wave`). Mas
`parseWaves` (mesmo arquivo, linha ~203) faz a **mesma** checagem `intVal < 1` sobre o token do
cabeçalho `## Wave N` dentro do roadmap:

```go
// :89 — validação do flag
waveInt, _ := splitWaveLabel(waveLabel)
if waveInt < 1 { ... invalid --wave ... }

// :203 — validação do cabeçalho, dentro de parseWaves
intVal, _ := splitWaveLabel(token)
if intVal < 1 { ... malformed wave heading ... }
```

São dois guardas independentes com a mesma mensagem de erro. Corrigir só o do flag deixa `barrier
--wave 0` passar da CLI mas falhar ao **ler o próprio roadmap** assim que ele contiver `## Wave 0 —
…` — erro tardio, mesma classe do achado do `$PWD`/`~/` desta série (duas formas do mesmo problema,
só uma na lista). **Veredito:** ML-1A precisa tocar as duas linhas, nos 3 stacks; AC4 ("parser
reconhece o cabeçalho") não é satisfeito de graça pela correção de AC3 — são dois pontos de código.

O regex `waveLabelRe = ^\d+(?:-[a-z0-9]+)?$` já casa com `"0"` **[medido]** — o comentário no próprio
código já registra isso ("regex allows '0' since `\d+` matches any digits"); a rejeição de `intVal <
1` é deliberada, não um bug lateral. Isso é bom sinal: o autor original já havia previsto o caso e
optou por bloqueá-lo explicitamente, então estender para `intVal < 0` é uma mudança mínima e
localizada, não uma reescrita de gramática.

### 1.2 `roadmap new --from-req` não gera Wave 0 nem gate, e usa `ML-1x` fixo **[medido]**

`internal/generators/roadmap.go:153` (`NewRoadmapFromREQ`) monta a seção de implementação com string
literal `"## Wave 1 — Implementation (derived from REQ criteria)"` e rotula cada ML derivado de
critério de aceite como `fmt.Sprintf("ML-1%c", rune('A'+i))` — nunca emite `## Wave 0` nem qualquer
bloco `**Gates da wave:**`. A REQ já nomeia esse caminho como superfície (bom), mas não declara o
rótulo do ML da Wave 0 nesse fluxo. Como o ADR rejeitou renumerar (`## Wave 1` continua sendo a
primeira wave de implementação), a Wave 0 tem que ser **prependada** à seção derivada, com um `ML-0A`
próprio — nunca `ML-1A`, que já está reservado para o primeiro critério da REQ. Nada no parser impede
a colisão: `mlHeadingRe = ^### (ML-\S+)` não valida se o prefixo numérico do ML bate com o rótulo da
wave que o contém (§2 do `barrier.go`, `mlBlock` delimita por posição de linha, não por prefixo).
**Veredito:** ML-1A precisa declarar explicitamente, nos dois geradores (`new` e `--from-req`), que o
ML da Wave 0 é sempre `ML-0A` — e que o `--from-req` também precisa emitir um bloco `**Gates da
wave:**` vazio-mas-presente na Wave 0 (ver §2.3, é o único freio que o `barrier` de fato aplica).

### 1.3 Canal de propagação do asset ≠ canal citado na REQ **[medido]**

A REQ e o roadmap dizem "Aplicação: `trackfw update harness`". Mas `architect.md`/`security.md` são
**assets de agente**, propagados por `trackfw agents update` (ou `agents install`), não por `update
harness` — são pipelines distintos no CLI (`update harness` toca hooks/`CLAUDE.md`/config; `agents
update` toca `~/.claude/agents/*.md` e equivalentes). Confirmado nesta mesma série
(`docs/agents-working-context.md`, sessão de 2026-08-22): o KG precisou rodar `trackfw update`,
`update harness` **e** `agents update --force` separadamente para os três pontos convergirem. A boa
notícia, também medida na vault
(`asset-parity-gate-nao-cobre-o-caminho-de-geracao-mas-o-caminho-e-fiel-2026-08-13.md`): um `agents
update` **sem** `--force` já substitui o artefato normalmente — `--force` só é necessário se o usuário
tiver editado manualmente o arquivo instalado. **Veredito:** a REQ/roadmap deveriam nomear os dois
comandos (`update harness` + `agents update`), não só o primeiro; sem isso, um usuário que rode só
`update harness` conclui erroneamente que recebeu AC5/AC6.

### 1.4 Roadmaps existentes: comportamento é erro de uso, não passagem silenciosa **[medido]**

Testado por leitura de código: `barrier --wave 0` sobre um roadmap sem `## Wave 0` cai em
`resolveBarrierRoadmap`/`runBarrier` → `target == nil` → `usageExit(cmd, "wave %s not found in
roadmap %q", ...)`, que sai com código de uso (2), não com um "passed" vazio. **Veredito:** este item
da lista da REQ já está coberto pelo comportamento atual — nomear como verificado para que a Wave 3
(ML-3A) não reabra a investigação.

### 1.5 A separação "template plano vs. render multi-alvo" não está na REQ, e ela decide o que "6
runtimes" significa **[medido]**

O template de roadmap (`roadmap.go`) nunca passa por `internal/integrations/render.go` — é escrito
como arquivo Markdown puro, direto no disco, sem transformação por `Capability.Representation`. Só os
**assets de agente** (`architect.md`, `security.md`) atravessam os alvos com representações diferentes
(`custom-agent-toml` para Codex, `agent-directory`/`opencode-agent` reconstruindo frontmatter, `subagent`
para Claude/Gemini/Cursor/Copilot/Kiro/Windsurf copiando o corpo quase literal). Para o caso Codex, o
corpo inteiro vira uma string TOML citada (`developer_instructions = strconv.Quote(body)`) — o texto
de prosa sobre Wave 0/`trackfw push` sobrevive inalterado dentro da string, então não há divergência
de conteúdo entre alvos para esse ponto específico. **Veredito:** a pergunta "os 6 runtimes renderizam
igual?" (roadmap ML-0A, item da REQ) só se aplica a AC5/AC6 (assets); para AC1/AC2 (template de
roadmap) a pergunta não existe — é um único arquivo plano por CLI, não seis. A REQ/roadmap não separam
essas duas superfícies e isso pode levar o ML-1A a testar renderização de roadmap contra 6 alvos
inexistentes (desperdício) ou, pior, a assumir que testar 1 alvo cobre os assets nos outros 5.

### 1.6 `CLAUDE.md` customizado pelo usuário — sem novidade estrutural, mas sem gate dedicado

**[raciocinado, não medido neste ML]**: não abri `internal/generators/claudemd.go` além da linha 70
citada na REQ para confirmar se há detecção de customização (diff contra managed baseline) equivalente
à do manifesto de `agents update`. Não afirmo que falta — não medi. O que aponto é que a REQ trata
"`CLAUDE.md` semeado" como um único bullet (AC7) sem distinguir projeto-novo (recebe a diretiva nova
de fábrica) de projeto-existente-com-`CLAUDE.md`-customizado (só recebe via `trackfw update`, sujeito
às mesmas regras de "modified managed artifact" já conhecidas do manifesto). **Veredito:** nomear
como pendência de medição para ML-1A, não como achado fechado.

---

## 2. Modelo de ameaça

**Fronteira de confiança:** o adversário não está fora do processo de desenvolvimento — ele **é** o
processo. Duas personas, ambas de boa-fé e sem violar nenhuma regra escrita:

- **O agente com pressa** — otimiza para "barrier verde", não para "risco enumerado". Escreve o
  mínimo que satisfaz os checks mecânicos.
- **O arquiteto otimista** — decide que "esta REQ é óbvia, não precisa de Wave 0 de verdade" e
  despacha a Wave 1 sem esperar, ou aceita uma Wave 0 fraca porque o resto do plano parece bom.

A pergunta central: **quais dos quatro checks do `barrier` (`mls_complete`, `acceptance_evidence`,
`gates`, `validate`) cada via de esvaziamento precisa passar — e quantos ela de fato passa?**

### 2.1 O achado central, medido: os quatro checks do `barrier` são satisfazíveis editando só o
roadmap, sem o conteúdo do artefato de Wave 0 nunca ser lido

`runBarrier` (`internal/commands/barrier.go:410-560`) — **[medido, lendo a função inteira]**:

- `mls_complete` — verifica se `**Status:**` da ML contém `✅`. String no roadmap, escrita pelo agente.
- `acceptance_evidence` — verifica se os itens de `**Critérios de aceite:**` estão marcados `[x]`.
  String no roadmap, escrita pelo agente.
- `gates` — executa os comandos dentro do bloco `**Gates da wave:**` **se ele existir**. Se a wave não
  declarar nenhum gate, `parseGates` retorna lista vazia **sem erro** (`internal/commands/
  barrier.go:341-380`, caminho de saída `return []string{}, nil` quando o cabeçalho não é encontrado
  na wave); o loop de execução não roda nenhuma vez; `gatesOK` permanece `true`; o check é `passed`.
  **Uma wave sem gates declarados passa vacuamente.**
- `validate` — roda `validator.ValidateTagged()`, que não tem (e o ADR explicitamente recusa dar) uma
  regra sobre Wave 0 ou sobre `docs/seguranca/*.md`.

**Em nenhum lugar de `runBarrier` há leitura de `docs/seguranca/`.** O caminho do artefato nem aparece
como string no arquivo. Isso não é uma lacuna acidental — o comentário de topo do arquivo já declara o
design: "it never assumes a build tool... every executable check either comes from the roadmap itself
(gates) or from the in-process validator". A Wave 0 nasce dentro de um `barrier` desenhado para nunca
inventar um gate; ela é exatamente o tipo de coisa que esse desenho, por princípio, não teria como
verificar sozinho.

### 2.2 Cada via de esvaziamento listada pela tarefa, contra os quatro checks

| Via de esvaziamento | `mls_complete` | `acceptance_evidence` | `gates` | `validate` | O `barrier` pega? |
|---|---|---|---|---|---|
| Wave 0 escrita pelo próprio implementador (não pelo papel de segurança) | passa (agente marca ✅) | passa (agente marca [x]) | passa se não houver gate declarado | passa (nada a violar) | **Não.** Nada no `barrier`/`validate` distingue autor. |
| Parecer de uma linha ("nenhum risco identificado") | passa | passa (critérios são checkbox, não conteúdo) | passa | passa | **Não.** `acceptance_evidence` conta `[x]`, não lê o texto do artefato. |
| Wave 0 copiada da REQ anterior | passa | passa | passa | passa | **Não.** Nenhum check compara conteúdo entre REQs. |
| Wave marcada `✅ Concluído` sem artefato correspondente | passa (`mls_complete` só olha a string `✅`) | depende — se os 4 itens de aceite forem marcados `[x]` também, passa | passa | passa | **Não**, se o agente também marcar os critérios; `mls_complete`/`acceptance_evidence` nunca chamam `os.Stat` sobre `docs/seguranca/*.md`. |
| Arquiteto despacha Wave 1 sem esperar Wave 0 auditada | N/A (checagem de Wave 1, não de Wave 0) | N/A | N/A | passa | **Parcialmente.** `barrier --wave 1` roda independente do estado da Wave 0 — nada no `barrier` impõe ordem entre waves (nenhuma checagem de "wave anterior passou" em `runBarrier`). A dependência é só textual (`> Dependências: Wave 0 auditada`), lida por humano/agente. |

**Resumo honesto, correspondendo ao que a tarefa pediu ("qual desses o `barrier` pega, e qual
passa"): o `barrier` não pega nenhuma das cinco vias listadas.** Isso não é um bug a corrigir no
`barrier` em si — é uma consequência direta do desenho "nunca inventa gate" que o próprio `barrier.go`
declara como princípio, e que o ADR desta REQ reafirma ao rejeitar tornar Wave 0 bloqueante em
`validate`. **A única alavanca real está no gerador, não no `barrier`**: se o **template emitido**
já vier com um bloco `**Gates da wave:**` não-vazio na Wave 0 — por exemplo, um gate que verifica a
existência e o tamanho mínimo de `docs/seguranca/<data>-*.md`, ou que faz `grep` pelas quatro seções
obrigatórias dentro dele — então apagar ou esvaziar esse bloco **deixa um diff visível** no roadmap, e
rodar `barrier --wave 0` sem ele muda o resultado de `passed` para `blocked` (wave sem ML) ou expõe a
ausência do gate na leitura do `--json`. Isso não fecha as cinco vias sozinho (a Wave 0 pode ainda ser
rasa e passar no gate raso), mas é a única coisa que o `barrier` de fato pode aplicar mecanicamente, e
hoje **não está em nenhum AC da REQ nem em nenhuma ação do ML-1A**. Recomendo adicioná-la.

---

## 3. Alvos de falsificação, nas duas direções

Formato: onde a sabotagem entra (arquivo:símbolo) → o que deveria acusar → em qual direção.

| # | Sabotagem | Onde entra | Gate que deveria acusar | Direção |
|---|---|---|---|---|
| F1 | Gerador deixa de emitir `## Wave 0` (regressão de template) | `internal/generators/roadmap.go` (`NewRoadmapFromContent` e `NewRoadmapFromREQ`) + equivalentes `npm/src/generators/`, `pypi/trackfw/generators/` | `scripts/check-artifact-parity.sh` **[medido]** — já executa `roadmap new`, `roadmap new --title/--req` e `roadmap new --from-req` nos 3 binários (Go/Node/Python, linhas 58, 77-78, 83, 102-103, 108, 127-128) e faz `diff -q` **entre as 3 saídas**, não contra um conteúdo esperado. Uma regressão que remove `## Wave 0` **igualmente nos 3 stacks** produz 3 saídas idênticas entre si e o gate passa — mesma classe de lacuna do F3/F4. Precisa de uma asserção de conteúdo **adicionada** a este script (`grep -q "## Wave 0 — "` no arquivo Go gerado), não de um gate novo | acusar de menos (falta o que deveria existir) |
| F2 | `barrier --wave 0` volta a ser recusado | `internal/commands/barrier.go:89` (flag) **e** `:203` (`parseWaves`, ver §1.1 — são 2 pontos, um teste em cada) + equivalentes 3 CLIs | Teste de regressão: roadmap fixture com `## Wave 0`, `barrier --wave 0` deve sair 0 | acusar de menos |
| F3 | Asset (`architect.md`) perde a menção a `trackfw push` | `internal/integrations/assets/agents/architect.md` + `npm/src/`/`pypi/trackfw/` cópias | `scripts/check-integration-assets.sh` **[medido, lido inteiro]** só faz `find`+`sort`+`cmp` do diretório `internal/integrations/assets` contra as cópias — paridade de lista de arquivos e byte-a-byte entre stacks, nunca conteúdo esperado (confirma a vault `asset-parity-gate-nao-cobre-o-caminho-de-geracao...`); precisa de teste de **conteúdo** novo: `grep -c "trackfw push"` ≥ 1 no asset-fonte de `architect.md`, além do parity check existente | acusar de menos, e é o caso onde o gate atual **não** cobriria — precisa de teste novo, não reaproveitado |
| F4 | Templates divergem entre os 3 CLIs (Wave 0 com texto diferente em Go/Node/Python) | qualquer um dos 3 geradores, editado sem replicar nos outros 2 | `scripts/check-artifact-parity.sh` **[medido — fecha a lacuna que este ML deixara "não medida"]**: é o gate certo (não `check-integration-assets.sh`, que só cobre `internal/integrations/assets`), e cobre `generators/roadmap.go`, mas do mesmo jeito do F1/F3 — diff cross-stack, não contra conteúdo fixo. Uma divergência real entre stacks **é** pega; uma divergência idêntica-mas-errada nos 3 não é | acusar de menos |
| F5 | Bloco `**Gates da wave:**` da Wave 0 emitido pelo gerador é removido/esvaziado pelo implementador (a alavanca identificada em §2.1) | roadmap gerado, editado à mão antes do commit | `barrier --wave 0 --json`, verificando `checks[].name == "gates"` e `commands` não-vazio — se o AC9 adotar F5, este é o teste que fecha a lacuna central do §2. **Restrição obrigatória sobre o gate que o gerador emitir** (medido em `runGateCommand:385-394`): o comando dentro do bloco de código é executado literalmente via `exec.Command("sh", "-c", command)` — sem sanitização, e **sem respeitar `TRACKFW_DISABLE_EXTERNAL_COMMANDS`** (grep confirma zero ocorrências dessa env var em `internal/commands/barrier.go`; o Makefile só a define para o alvo `test`, não para `parity`/`barrier`). `NewRoadmapFromContent` interpola `content.Title` via `fmt.Sprintf` sem passar por `toSlug`, e `NewRoadmapFromREQ` interpola strings de critério de aceite da REQ nos cabeçalhos de ML — se o ML-1A interpolar título/slug/data no comando do gate, um título de REQ contendo `` ` ``, `$(...)` ou `;` vira comando shell executado pelo `barrier`. **O gate emitido tem que ser fixo, sem interpolação de string controlada pelo usuário/REQ**, e seu caminho não pode assumir `docs/seguranca/` como convenção universal — `ProjectConfig` (`internal/config/config.go:22`, campos medidos: `RoadmapDir` e afins, sem equivalente de diretório de segurança) não tem esse namespace; hardcodar o caminho quebra em todo projeto que não replicar esta convenção local | acusar de menos — único item desta tabela que fecha uma via real de esvaziamento do método, mas é também o único que **cria** superfície nova (execução de shell) se implementado sem esta restrição |
| F6 | `barrier --wave 0` passa a **aceitar** rótulos inválidos além de `0` (ex.: `-1`, `0.5`, regressão da gramática na direção oposta) | mesmos dois pontos de F2 | Teste de contrato negativo: `barrier --wave -1` e `--wave 0.5` devem continuar recusados nos 3 CLIs | acusar de mais — falsificação na direção oposta a F2, exigida pelo AC9 ("nas duas direções") e ausente da lista mínima do roadmap |
| F7 | Roadmap **existente** (sem Wave 0) passa a ser silenciosamente aceito por `barrier --wave 0` como se tivesse uma Wave 0 vazia-porém-válida | mudança futura em `resolveBarrierRoadmap`/`runBarrier` que trate "wave não encontrada" como sucesso em vez de erro de uso | Teste de regressão sobre §1.4: roadmap sem `## Wave 0`, `barrier --wave 0` deve continuar saindo com código de uso (2), nunca 0 | acusar de mais — hoje o comportamento é correto (medido), mas nada o trava como contrato |

F1, F2, F4 e F6 correspondem ao mínimo pedido pelo AC9/roadmap. F3, F5 e F7 são acréscimos deste
parecer: F3 porque o gate de paridade correto (`check-integration-assets.sh`) **não** cobre conteúdo
(medido, lido inteiro — confirma a vault); F1 e F4, medidos contra o gate correto
(`check-artifact-parity.sh`), revelam a **mesma** lacuna que F3 — os dois gates de paridade desta REQ
comparam stacks entre si, nunca contra conteúdo esperado; F5 porque é a única alavanca real contra o
achado do §2, mas exige a restrição de não-interpolação acima para não abrir uma superfície de
execução de shell nova; F7 porque fecha a falsificação "acusar de mais" que falta na lista mínima (o
roadmap tem duas entradas de "acusar de menos" e nenhuma de "acusar de mais" — F6 e F7 preenchem
isso).

---

## 4. Residual declarado

**O que este desenho aceita não cobrir, dito em duas partes — porque a resposta não é uma só.**

### 4.1 A classe de achado que uma Wave 0 pega: sim, plausivelmente, para o caso que motivou o ADR

O achado do `~/` (REQ do `$PWD`, 2026-08-21/22) era uma lacuna em uma **tabela de classes já fechada
no ADR** — exatamente o formato que a seção 1 deste parecer (completude de enumeração) foi desenhada
para forçar: "dada a lista fechada, o que falta?". Um revisor adversarial lendo a tabela de classes
antes de qualquer código **teria** perguntado "e `~/`?" pela mesma razão que, nesta seção 1, perguntei
"e o segundo ponto de bloqueio do `barrier`? e o `--from-req`? e o canal de propagação certo?" — são
perguntas sobre uma lista, respondidas por leitura, sem executar nada. **Isso o método pega.**

### 4.2 O resto da mesma reprovação: não — e isso é o limite estrutural que o ADR já nomeia

A mesma reprovação de 2026-08-21/22 trazia mais três achados que uma Wave 0 **não pegaria**:
`${PWD}/…` sendo classificado silenciosamente (comportamento errado só visível rodando o
classificador contra um input real), a mensagem de erro incorreta emitida em runtime, e
`resolveCredentialGuardHookPath` usando `m.raw` com aspas de um jeito que escapava checks de
existência/executabilidade. Nenhum desses é visível olhando uma tabela — todos exigem **executar** o
código contra um caso concreto. Tenho evidência própria e anterior disso, não só inferência: a nota de
memória `feedback_verify_by_execution` registra o mesmo padrão no bug de bind address do `trackfw
serve` — a leitura do código sozinha não achou o problema; só `lsof`/`curl` contra o processo real
acharam. **Wave 0 desloca a enumeração para a esquerda; ela não desloca a medição.** São famílias de
falha diferentes, e o ADR já declara isso explicitamente na seção "Consequences — Negativas": "o
revisor de desenho não mede — ele raciocina sobre um artefato que ainda não existe. É o limite
estrutural da Wave 0".

### 4.3 Residual novo, encontrado neste próprio parecer

- **O `barrier` não pega nenhuma das cinco vias de esvaziamento do §2**, por desenho (nunca inventa
  gate). A única correção estrutural possível é o gerador pré-carregar um gate não-vazio na Wave 0
  (F5) — e isso hoje não está em nenhum AC. Sem essa mudança, uma Wave 0 vazia, copiada ou escrita
  pelo implementador passa `barrier --wave 0` limpo, sempre.
- **`barrier` não impõe ordem entre waves.** "Wave 1 depende de Wave 0 auditada" é uma frase no
  roadmap, não uma checagem em código. O arquiteto que despacha Wave 1 cedo não recebe nenhum erro de
  ferramenta — só quebra uma convenção textual. Isso está fora do escopo desta REQ (o `barrier` avalia
  uma wave por vez, por design), mas é honesto nomear que **nada impede mecanicamente** a quinta via
  de esvaziamento listada na tarefa ("arquiteto que despacha Wave 1 sem esperar Wave 0").
- **A disciplina de medição (seis ocorrências de "verde" sem exit code, nesta mesma série) permanece
  fora de escopo**, como o próprio ADR declara. Este parecer não a resolve nem tenta.
- **Este parecer também não mede nada** — inclusive as duas classificações "não medido" acima
  (§1.6, cobertura de `check-integration-assets.sh` sobre `roadmap.go`). Isso é consistente com a
  natureza da Wave 0: é raciocínio sobre um artefato — o harness pós-ML-1A — que ainda não existe.

**Se a pergunta for "uma Wave 0 teria pego o achado do `~/`": sim.** Se a pergunta for "uma Wave 0
teria pego a reprovação inteira daquela REQ": não — três de quatro achados daquela reprovação exigiam
execução, e a Wave 0 estrutural e deliberadamente não executa nada.

---

## Veredito da Wave 0

**Prosseguir para a Wave 1**, com três adições recomendadas ao escopo do ML-1A além do que a REQ já
lista (nenhuma delas é bloqueante — são fechamentos de lacuna, não vetos):

1. Corrigir os **dois** pontos de bloqueio do `barrier` (`:89` e `:203`), não só o citado na REQ
   (§1.1) — nos 3 stacks.
2. O template da Wave 0 emitido pelo gerador (`new` e `--from-req`) deve incluir um bloco
   `**Gates da wave:**` não-vazio, ancorado no artefato desta própria wave (F5, §2.1/§3) — é a única
   alavanca mecânica encontrada contra o esvaziamento do método — **sujeito à restrição de F5**: o
   comando declarado tem que ser fixo e sem interpolação de string controlada por título/REQ/slug
   (`runGateCommand` executa via `sh -c` sem passar por `TRACKFW_DISABLE_EXTERNAL_COMMANDS`), e não
   pode assumir `docs/seguranca/` como caminho universal, já que `ProjectConfig` não tem esse
   namespace. Sem essa restrição, a mitigação do §2 introduz uma superfície de execução de shell nova.
3. Adicionar ao `scripts/check-artifact-parity.sh` uma asserção de conteúdo (`grep -q "## Wave 0 —
   "` no arquivo gerado pelo Go) — os gates de paridade existentes (`check-artifact-parity.sh` para o
   template de roadmap, `check-integration-assets.sh` para os assets) comparam os 3 stacks entre si,
   nunca contra um conteúdo esperado; uma regressão sincronizada nos 3 stacks (F1/F3/F4) passa hoje.

Nenhuma via de esvaziamento identificada aqui é fechável só por texto de asset ou por regra de
`validate` — o ADR já rejeitou a segunda opção, com motivo. O fechamento real depende de F5 (com a
restrição do item 2) e de F1/F3/F4 (item 3), e ambos dependem do ML-1A.
