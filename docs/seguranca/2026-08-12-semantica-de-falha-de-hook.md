---
status: done
date: 2026-08-12
author: "Hades (Segurança)"
---

# Parecer de segurança — semântica de falha de hook (fail-open vs fail-closed) e o credential-guard como controle de negação (ML-2A)

> ML-2A do roadmap
> `docs/roadmaps/wip/ROADMAP-2026-08-12-semantica-de-falha-de-hook-fail-open-vs-fail-closed-nucleo-empirico-no-codex.md`.
> Insumos: `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md` (empírico, Codex 0.147.0),
> `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md` (doc primária, 5 CLIs),
> `vault/notes/hooks-de-agente-falham-abertos-quando-o-script-nao-resolve-2026-08-12.md`,
> `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (Q3, origem da pergunta),
> `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` (cwd de execução por CLI).
> Este parecer é **puramente de leitura**: nenhum arquivo de código foi tocado. Achados vão para
> Zeus, que decide se abre REQ de mitigação.

## Resumo do veredito consolidado (Barreira B1, entrada deste ML)

| CLI | Caso A (`command` não resolve) | Caso B (`exit != 0`) | Fonte |
|---|---|---|---|
| Claude Code | **FAIL-OPEN** | `exit 1` open · `exit 2` closed | doc primária |
| Codex CLI | **FAIL-OPEN** | `exit 1` open · `exit 2` closed | **empírico**, `codex-cli 0.147.0` |
| Cursor | **FAIL-OPEN** (padrão) | open salvo `exit 2`; opt-in `failClosed: true` | doc primária |
| Gemini | INDETERMINADO | fora de `{0,2}` é open | doc primária |
| Copilot | fail-closed em `preToolUse` | fail-closed | doc primária |
| Kiro | INDETERMINADO | depende da superfície (IDE fail-closed / CLI como Claude) | doc primária |

---

## Pergunta 1 — O credential-guard ainda é um controle de negação efetivo?

**Resposta: não uniformemente. É efetivo apenas onde duas condições coincidem — (a) o CLI trata
falha de execução do hook como fail-closed, e (b) o Caso A é de fato alcançável pela forma exata do
comando que o trackfw emite para aquele CLI.** Avaliar só o veredito documental do fornecedor (eixo
a) responde a pergunta errada — é preciso cruzar com a **alcançabilidade real** do Caso A dado o
comando emitido (eixo b), porque um CLI fail-open só é um risco concreto se algo plausível o levar ao
Caso A.

### Por CLI, cruzando os dois eixos

**Codex — pior caso, e o único onde a alcançabilidade foi verificada como trivial.** O comando emitido
é `"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh"`
(`internal/generators/agentfiles.go:317`). A doc primária do Codex (`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`,
seção 2) confirma que o **cwd de execução do hook é o cwd da sessão, dinâmico** ("Codex may be
started from a subdirectory, and a git-root-based path keeps the hook location stable" — a própria
doc do fornecedor recomenda a resolução via git-root exatamente porque o cwd pode variar). Isso
significa que **o próprio agente, com uma única chamada de `Bash` que não passa por nenhuma lógica de
bloqueio do guard** (`mkdir x && cd x && git init`), consegue colocar a sessão dentro de um novo
repositório git aninhado sem `scripts/`. Toda chamada de `Bash` subsequente, executada com esse cwd,
faz `git rev-parse --show-toplevel` resolver para a raiz aninhada — sem `scripts/trackfw-credential-guard.sh`
ali — reproduzindo exatamente o Caso A medido empiricamente no ML-1A como FAIL-OPEN. Isso não é um
cenário hipotético "e se o ambiente for hostil": é uma sequência de dois comandos de shell comuns,
sem privilégio nenhum, disparável pelo próprio agente sob indução (prompt injection ou tool-call
malicioso). **O guard não protege contra esse vetor hoje.**

**Claude / Gemini — efetivo na prática, mas por um argumento de inalcançabilidade, não por
fail-closed.** Ambos são fail-open no Caso A (Claude confirmado; Gemini `INDETERMINADO`, tratado como
pior caso), mas o comando emitido (`$CLAUDE_PROJECT_DIR/...`, `$GEMINI_PROJECT_DIR/...`) só cai no
Caso A se a env var vier vazia — o que produz `/scripts/trackfw-<script>.sh`, caminho absoluto na
raiz do sistema de arquivos, onde nenhuma parte sem privilégio de root planta um arquivo
(`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q2). O agente **não tem um comando análogo ao
`git init` do Codex** para forçar essa condição — a variável é setada pelo processo do CLI, não
derivada do cwd nem do conteúdo do repositório. Efetivo hoje, mas frágil: depende inteiramente da
garantia (não citada literalmente na doc, ver Q2 do parecer de 2026-08-11) de que a variável está
**sempre** presente.

**Cursor — efetivo por design de cwd, mas com um gap de configuração explícito.** O cwd de hooks de
projeto é fixo na raiz por design do fornecedor (`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`,
seção 4: "Project hooks... Run from the project root", não dinâmico por sessão) — o Caso A por
resolução de caminho é praticamente inalcançável. Mas o veredito documental é **fail-open por
padrão** e o gerador do trackfw (`agentfiles.go:951-966`) **não emite `failClosed: true`** nas
entradas do guard. Ou seja: o mecanismo de proteção aqui não é "o Cursor bloqueia por padrão", é "o
Caso A é difícil de alcançar por cwd fixo" — mas continua fail-open para *qualquer outra* forma de o
hook falhar (crash, timeout, JSON inválido), sem o opt-in nativo que existe e está documentado.

**Copilot — o único CLI onde o controle realmente se sustenta em fail-closed do próprio fornecedor.**
`preToolUse` é fail-closed por doc primária tanto para Caso A quanto Caso B (com a ressalva de
interpretação de que a doc usa "crash", não literalmente "script not found" — registrada no ML-1B).
Mecanismo nativo estruturado (`"cwd": "."`, campo `bash`, sem substituição de shell,
`agentfiles.go:789-824`, função `InjectCopilotHooks`) reduz ainda mais a superfície de Caso A por
resolução de caminho.

**Kiro — indeterminado nos dois eixos, tratado como pior caso por precaução.** Caso A `INDETERMINADO`
na doc; cwd de execução também `INDETERMINADO` (`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`,
seção 5); Caso B diverge entre as abas IDE (fail-closed) e CLI (fail-open em `exit 1`, como Claude) da
mesma página de doc — e qual superfície o trackfw mira no Kiro é, por si, uma pergunta em aberto
(não resolvida por este parecer nem pelo ML-1B).

**Severidade por CLI:** 🔴 Codex (alcançável, verificado, sem privilégio) · 🟡 Cursor (fail-open
default sem opt-in usado, mas Caso A por caminho é de baixa alcançabilidade) · 🟡 Kiro
(indeterminado nos dois eixos) · 🟢 Claude/Gemini (fail-open mas caso A inalcançável sem controle de
ambiente) · 🟢 Copilot (fail-closed nativo).

---

## Pergunta 2 — A severidade dos caminhos documentados em `cli-parity.md` muda?

**Sim, e a mudança é de classe, não de grau.** Os três caminhos ("fora de repo git", "submódulo/worktree",
"`GIT_DIR`/`GIT_WORK_TREE`") estavam registrados como degradação de **disponibilidade** — o
enquadramento implícito era "o guard às vezes não roda, o agente perde uma verificação, mas o pior
caso é ruído". Com o veredito FAIL-OPEN confirmado, esse enquadramento está errado: **o guard não
"deixar de rodar" não é neutro — é o controle de negação sendo desligado sem nenhum sinal que o
usuário normalmente observa.** Isso reclassifica os três caminhos de "indisponibilidade tolerável"
para "bypass silencioso de controle de segurança", exatamente como o vault já registrou para o
incidente original (`Failed with non-blocking status code`).

A consequência prática para a **remediação** também muda de classe: uma falha de disponibilidade se
resolve **documentando** (o usuário eventualmente percebe que algo não funciona e corrige). Um bypass
de controle de segurança silencioso não se resolve documentando — ele exige **detecção**, porque por
definição ninguém percebe sozinho. É esse deslocamento — de "documentar" para "detectar" — que
justifica considerar uma REQ nova, não apenas atualizar uma tabela.

**Achado adicional de detectabilidade, fora da pergunta literal mas relevante para a mesma
reclassificação:** o gate de *trust* do Codex (hooks de projeto só carregam em projeto `trusted`,
Emenda 1 do ADR-2026-08-11) é **pior** que o Caso A no eixo de sinal observável. O Caso A ao menos
produz `hook: PreToolUse Failed` nos logs do Codex — um sinal, mesmo que ignorado. Um projeto não
confiável não produz **nenhum** evento de hook — zero sinal. Isso não é "novo" (é pré-existente, já
registrado no ADR), mas pertence à mesma família de risco que este parecer está reclassificando e
deveria ser citado ao lado dos três caminhos, não separado deles.

---

## Pergunta 3 — Modelo de ameaça: quem consegue colocar o agente numa dessas condições?

Distinguindo acidente de ação deliberada, e incluindo o próprio agente induzido como atacante
relevante (é para conter subagentes que o guard existe):

**(a) Acidente de configuração/ambiente** — cobre a maioria dos três caminhos documentados: rodar o
CLI fora de um clone git (ex.: diretório extraído de um zip), abrir uma sessão dentro de um
submódulo/worktree por engano, ou herdar `GIT_DIR`/`GIT_WORK_TREE` de uma sessão de shell anterior
sem perceber. Nenhum desses exige intenção maliciosa — são erros operacionais plausíveis do próprio
usuário legítimo. Consequência: o guard desliga em silêncio sem que ninguém tenha "atacado" nada.

**(b) Quem controla o repositório** — pode versionar `.codex/hooks.json`/`.claude/settings.json`
apontando para um caminho que nunca resolve, ou estruturar o repositório com um submódulo/worktree de
forma que a raiz relevante nunca contenha `scripts/`. Isso já está dominado pelo argumento do Q1 do
parecer de 2026-08-11: quem controla o `hooks.json` versionado já pode escrever comandos arbitrários
ali — não precisa do truque de Caso A para isso. **Não é uma superfície nova.**

**(c) Quem controla o ambiente do processo (`GIT_DIR`/`GIT_WORK_TREE`, `CODEX_HOME`, etc.)** — exige
acesso à máquina/ambiente de execução, um nível de acesso que já está fora do modelo de ameaça "repo
hostil" que o ADR de 2026-08-11 declara cobrir. Risco real, mas de prioridade menor porque quem tem
esse acesso já tem alternativas mais diretas de comprometimento.

**(d) O próprio agente, induzido — o achado novo e mais importante deste parecer, específico do
Codex.** Diferente de (b) e (c), este vetor **não exige controle prévio do repositório nem do
ambiente**: qualquer sequência de tool-calls que o agente execute (por indução de prompt injection em
conteúdo lido, por uma tarefa mal especificada, ou por um subagente comprometido a jusante) pode
incluir `mkdir x && cd x && git init` antes de uma ação que materializaria credenciais. Como
demonstrado na Pergunta 1, isso é suficiente para reproduzir o Caso A no Codex sem tocar em
`hooks.json`, sem env var especial, e sem acesso à máquina — só duas chamadas de `Bash` comuns, do
tipo que um agente executa o tempo todo sem supervisão linha-a-linha. Este é exatamente o atacante
que o credential-guard existe para conter (subagente induzido), e é o único vetor dos quatro que o
guard, hoje, **não detecta nem impede**.

---

## Pergunta 4 — Mitigação: avaliação da hipótese `sh -c 'test -x <script> && exec <script> || exit 2'`

**Avaliado, não implementado. Veredito: a hipótese resolve parte do problema, tem um custo real de
"bricking" em estado legítimo, e não é a única opção — nem a de melhor custo/benefício isolada.**

### O que a hipótese resolve

Converte "comando não resolve" (Caso A) em `exit 2` explícito, no ponto de emissão do comando —
funciona textualmente nos CLIs cujo campo de hook é uma string de shell (`command`/`bash`): Claude,
Codex, Gemini, Cursor, Kiro-CLI. Como `exit 2` é o único sinal de bloqueio documentado-ou-provado em
**todos os seis** fornecedores (Claude `PreToolUse`, Codex empírico, Cursor, Gemini "System Block",
Copilot `preToolUse`, Kiro IDE bloqueia qualquer não-zero / Kiro CLI honra `exit 2`
especificamente), a mudança padroniza o Caso A no único primitivo universalmente honrado — inclusive
nos dois CLIs hoje `INDETERMINADO` (Gemini, Kiro), sem precisar provar a semântica deles
empiricamente primeiro (ver Pergunta 5).

### O que a hipótese não resolve

1. **Não cobre o caso de ameaça mais relevante (Pergunta 3-d) nem o "raiz errada" do
   submódulo/worktree.** Se `git rev-parse --show-toplevel` resolve para uma raiz aninhada que
   **também** tem um `scripts/trackfw-credential-guard.sh` (porque é outro repositório git legítimo,
   possivelmente de outro projeto do usuário, ou uma cópia desatualizada), `test -x` passa e o wrapper
   executa esse script — não o Caso A que ele foi desenhado para capturar, mas um "raiz errada, script
   presente" que o wrapper trata como sucesso silencioso. O wrapper resolve *ausência*, não *raiz
   errada com presença enganosa*.
2. **Não cobre crash, timeout nem JSON inválido** — só converte "não iniciou" em `exit 2`; um script
   que inicia e trava, ou que crasha no meio, continua fora do alcance do wrapper. O Cursor já tem
   `failClosed: true` nativo cobrindo exatamente esses três casos (crash, timeout, invalid JSON) de
   uma vez — para o Cursor, a mudança de configuração nativa é estritamente superior ao wrapper, e é
   **custo zero de execução** (não adiciona processo nenhum por chamada).
3. **Risco de "bricking" em ausência legítima do script — confirmado, não hipotético.** O script
   `trackfw-credential-guard.sh` não é parte do binário do trackfw: é **gerado** por
   `GenerateCredentialGuardScript`/`GenerateGlobalCredentialGuardScript`
   (`internal/generators/scaffold.go:779-837`) dentro de um repositório governado, via `trackfw init`
   (escopo de projeto) ou `trackfw update harness` (escopo global em `~/.trackfw/scripts/`). Um
   clone fresco de um repositório cujo `hooks.json`/`settings.json` já está versionado, mas antes de
   `trackfw init` rodar (ou entre o commit do config e o commit do script, ou num submódulo/worktree
   onde a raiz resolvida nunca teve `trackfw init` executado ali), **tem o hook cadastrado sem o
   script existir** — exatamente a condição que o wrapper trata como `exit 2`. Sem o wrapper, isso é
   fail-open (Caso A, degradação silenciosa). **Com** o wrapper, isso vira bloqueio de **toda**
   chamada de `Bash`/ferramenta coberta pelo matcher do guard — o agente fica travado, não degradado,
   até alguém rodar `trackfw init`/`update harness`. Não é um efeito colateral remoto: é o estado
   normal de um checkout novo antes do primeiro `init`.

### Custo por chamada de ferramenta

Marginal e dominado pelo próprio custo de execução do guard: `test -x` é builtin do shell (sem
`fork`/`exec` adicional), e o `exec` no caminho de sucesso substitui o processo do shell em vez de
empilhar um processo a mais. Para o Codex, que já invoca `git rev-parse` a cada chamada
(`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q1), o custo do wrapper é irrelevante frente ao
custo já aceito do `git rev-parse`. **Não é um argumento contra a hipótese.**

### Alternativas — melhor custo/benefício, avaliadas individualmente

- **`failClosed: true` no Cursor.** Config-only, cobre crash/timeout/JSON inválido de uma vez, custo
  de execução zero. Superior ao wrapper *para o Cursor especificamente*. Ressalva de escopo: deve ser
  aplicado **só** às entradas do guard, nunca às de attention-signal/cleanup que compartilham os
  mesmos arrays (`agentfiles.go:940-943` vs `:950-966`) — essas não são controle de segurança, e
  fail-closed nelas seria negação de serviço pura sem ganho de segurança.
- **Controle positivo em `trackfw validate`/`doctor`** (mencionado no próprio texto do roadmap: "o
  gate detectar ausência"). Em vez de (ou além de) mudar o que acontece em tempo de execução de cada
  chamada, o `validate` verifica, por CLI instalado, se o comando resolvido do guard aponta para um
  arquivo executável existente — e, para o Codex, se o projeto está marcado `trusted`. Custo zero por
  chamada de ferramenta (roda só quando o usuário chama `validate`/`doctor`, não a cada tool-call),
  sem risco de bricking, e captura exatamente a classe de erro que originou o incidente (caminho
  digitado errado, script ausente, projeto não confiável) — configuração estática, verificável fora do
  caminho quente. **Não cobre** manipulação de ambiente em tempo de sessão (Pergunta 3-c/d) — é
  complementar ao wrapper, não substituto dele.

### Recomendação sobre o Caso A do submódulo/worktree

Nenhuma das opções acima resolve "raiz errada com script presente" — isso exigiria comparar a raiz
resolvida contra alguma âncora conhecida (ex.: caminho absoluto do repositório em que `trackfw init`
rodou), o que é uma mudança de desenho maior e não estava no escopo da hipótese de Zeus. Registrar
como gap conhecido, não bloqueante.

---

## Pergunta 5 — Gemini e Kiro `INDETERMINADO` no Caso A: muda a recomendação? Vale prova empírica?

**Não muda a recomendação, e prova empírica dedicada não é necessária agora — com uma condição.**

O pior caso para os dois já está **assumido como fail-open** neste parecer (Pergunta 1, tratamento
conservador) e para a recomendação de mitigação (Pergunta 4), o wrapper que emite `exit 2` explícito
resolve o Caso A para **ambos** sem depender de prova prévia: `exit 2` é o único primitivo de bloqueio
documentado para os dois (Gemini "System Block"; Kiro CLI honra `exit 2` explicitamente, Kiro IDE
bloqueia qualquer não-zero — `exit 2` está coberto nos dois casos). Ou seja: a mitigação, se adotada,
**torna a pergunta "o Caso A é fail-open no Gemini/Kiro?" irrelevante**, porque deixa de existir um
Caso A puro para eles — o wrapper sempre entrega um Caso B com `exit 2`, que ambos honram.

A prova empírica só volta a valer a pena **se a REQ de mitigação for aberta e adotar o wrapper**: nesse
cenário, um braço empírico confirmando que o `exit 2` *emitido pelo wrapper* de fato bloqueia nos
eventos específicos que o trackfw conecta (não uma leitura genérica da doc) é um critério de aceite
razoável — barato porque já existe o método reprodutível do ML-1A, e mais barato ainda para o Gemini
(sem gate de trust, sem timeout de 2 min observado no Codex). Para o Kiro, resolver **qual superfície
(IDE vs CLI) o trackfw mira** é pré-requisito e mais barato que rodar o experimento — é uma pergunta
sobre o próprio gerador do trackfw, não sobre a doc do fornecedor (`docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md`,
seção 5, "Implicação para o `trackfw-credential-guard.sh`").

Se a mitigação **não** for adotada (risco aceito puro), a resposta muda: aí sim vale reavaliar prova
empírica para Gemini como item de follow-up de prioridade média (mesmo argumento do ADR de
2026-08-11 para o Codex), e Kiro permanece como dívida conhecida até a pergunta de superfície ser
resolvida.

---

## Recomendação explícita

**Risco aceito × REQ nova: recomendo abrir REQ nova de mitigação — mas com escopo mais estreito e
mais barato do que só "aplicar o wrapper".** A justificativa de não aceitar risco puro é a Pergunta
3-d: existe um vetor verificado, alcançável pelo próprio agente sob indução, com uma única sequência
de dois comandos de shell sem privilégio, no CLI de maior superfície (Codex). Isso não é mais um
"caminho documentado e aceito" — é um bypass ativo do controle que o guard existe para prover.

### Composição recomendada (prioridade decrescente de custo/benefício)

1. **Controle positivo em `trackfw validate`/`doctor`** — maior benefício, menor custo/risco. Detecta
   a classe de erro que causou o incidente original (caminho digitado errado, script ausente, projeto
   Codex não confiável) sem custo por chamada de ferramenta e sem risco de bricking.
2. **`failClosed: true` nas entradas do guard do Cursor** — config-only, cobre mais casos que o
   wrapper para esse CLI especificamente, custo zero.
3. **Wrapper `test -x ... || exit 2`** — avaliar como item **condicional**, não obrigatório: só se
   (1) e (2) forem julgados insuficientes para o vetor 3-d do Codex (que eles não cobrem, porque não
   atuam em tempo de execução da sessão). Se adotado, precisa resolver o problema de bricking (item
   abaixo) antes de ir para produção.

### Critérios de aceite a esboçar na REQ (para Zeus formalizar)

- **Paridade nos 3 CLIs de código é obrigatória** se a mitigação tocar geradores: `internal/`,
  `npm/src/`, `pypi/trackfw/`, com teste por stack — registrar explicitamente para a REQ não sair
  Go-only (regra dura do projeto).
- **Comportamento definido para ausência legítima do script** — regression test cobrindo: repositório
  com `hooks.json`/`settings.json` já commitado mas `trackfw init` ainda não rodado. Escolher um
  comportamento e testá-lo: ou (a) o wrapper não é emitido até `init` gerar o script (ordem de
  emissão), ou (b) o `validate`/`doctor` detecta e orienta o usuário antes que o wrapper trave a
  sessão. Não deixar em aberto — é exatamente a lacuna que causaria o próximo incidente.
- **`failClosed: true` do Cursor escopado só às entradas do guard**, nunca às de
  attention-signal/cleanup que compartilham os mesmos arrays de hooks — teste garantindo que a
  entrada certa recebe o campo.
- **Se o wrapper for adotado**, braço empírico confirmando que o `exit 2` emitido bloqueia
  especificamente nos eventos que o trackfw conecta em Gemini e na superfície do Kiro que o trackfw
  de fato usa (a determinar antes de testar).
- **Não cobre e deve documentar como gap aberto:** Caso A por raiz errada com script presente
  (submódulo/worktree apontando para outro repositório com `scripts/trackfw-credential-guard.sh`
  legítimo mas não relacionado) — nenhuma opção avaliada aqui resolve isso.

---

## Tabela para reuso no ML-3A (`docs/cli-parity.md`)

| CLI | Caso A | Caso B | Alcançabilidade do Caso A dado o comando emitido pelo trackfw | Controle efetivo hoje? |
|---|---|---|---|---|
| Claude Code | FAIL-OPEN (doc primária) | `exit 1` open · `exit 2` closed | Só se `$CLAUDE_PROJECT_DIR` vier vazio → degrada para `/scripts/...` (não plantável sem root) | Sim, por inalcançabilidade |
| Codex CLI | FAIL-OPEN (empírico, 0.147.0) | `exit 1` open · `exit 2` closed | **Alta — uma sequência `mkdir x && cd x && git init` do próprio agente** reproduz o Caso A sem privilégio | **Não** |
| Cursor | FAIL-OPEN por padrão (doc primária) | open salvo `exit 2`; `failClosed: true` nativo não usado pelo trackfw | Baixa (cwd fixo na raiz do projeto por design) | Parcial (protegido pelo cwd fixo, não pela config) |
| Gemini | INDETERMINADO (tratado como pior caso) | fora de `{0,2}` é open | Mesma condição de Claude (`$GEMINI_PROJECT_DIR` vazio) | Sim, por inalcançabilidade |
| Copilot | fail-closed em `preToolUse` (doc primária) | fail-closed | N/A — fail-closed nativo, mecanismo estruturado sem substituição de shell | Sim |
| Kiro | INDETERMINADO | Depende da superfície (IDE fail-closed / CLI como Claude) | INDETERMINADO — cwd também não verificável | Indeterminado |

## Relacionado

- `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md`
- `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md`
- `vault/notes/hooks-de-agente-falham-abertos-quando-o-script-nao-resolve-2026-08-12.md`
- `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` (cwd de execução por CLI, base da
  Pergunta 1/3)
- `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (Q2/Q3, origem da pergunta deste ciclo)
- `internal/generators/scaffold.go:779-837` (geração do script do guard, base da Pergunta 4)
- `internal/generators/agentfiles.go:317, 789-824, 940-966` (comando emitido por CLI, base das
  Perguntas 1 e 4)
