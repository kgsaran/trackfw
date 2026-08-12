---
status: Accepted
date: 2026-08-11
author: "Zeus (Arquiteto)"
---

# ADR: Resolucao de caminho dos hooks de projeto por CLI — mecanismo especifico do fornecedor, sem caminho absoluto

> Date: 2026-08-11 | Status: Accepted

REQ: `docs/req/REQ-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd-attention-signal-cleanup-e-os-5-clis-nao-claude.md`
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd.md`
Pesquisa que sustenta esta decisão (ML-0A): `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`

## Context

O trackfw injeta comandos de hook nos arquivos de settings de 6 CLIs de agente. Historicamente todos
foram emitidos como **caminho relativo puro** (`"command": "scripts/trackfw-attention-signal.sh"`).

No Claude Code isso é comprovadamente quebrado — a doc primária afirma "Handlers run in the current
directory", e esse cwd acompanha os `cd` do agente durante a sessão. Bug reportado em produção
(projeto CMDB) e corrigido em `0c66ecb`, **mas apenas para o credential-guard do Claude**. A questão
em aberto era: os outros comandos e os outros 5 CLIs sofrem do mesmo problema, e com qual mecanismo
se corrige?

Duas restrições delimitam o espaço de solução:

1. **Caminho absoluto está vetado no escopo de projeto.** Os arquivos de settings
   (`.claude/settings.json`, `.codex/hooks.json`, …) são versionados no repositório do usuário.
   Gravar o path da máquina que rodou `trackfw init/update` quebraria o hook em qualquer outro
   checkout. (O credential-guard de **escopo global**, gravado em `~/.trackfw/`, é caso distinto e
   legitimamente usa caminho absoluto — fora do escopo deste ADR.)
2. **Não existe mecanismo uniforme entre os 6 CLIs.** A verificação em doc primária (ML-0A)
   confirmou isso: cada fornecedor resolve o problema de forma diferente, e dois deles não o têm.

## Decision

**O mecanismo de resolução é decidido por CLI, segundo o que a doc primária do próprio fornecedor
oferece. Não se força uniformidade, e não se altera CLI cujo comportamento não foi verificado.**

| CLI | Veredito (ML-0A) | Decisão | Mecanismo |
|---|---|---|---|
| Claude Code | QUEBRADO | **Alterar** attention-signal/cleanup | `$CLAUDE_PROJECT_DIR/scripts/...` (credential-guard já usa desde `0c66ecb`) |
| Codex CLI | QUEBRADO | **Alterar** todos os comandos | `"$(git rev-parse --show-toplevel)/scripts/..."` |
| Gemini CLI | QUEBRADO | **Alterar** todos os comandos | `$GEMINI_PROJECT_DIR/scripts/...` |
| Cursor | OK | **Não alterar** | cwd de hooks de projeto é fixo na raiz; relativo já resolve certo |
| GitHub Copilot CLI | OK | **Não alterar** | já emite o campo nativo `"cwd": "."` em todas as entradas |
| Kiro | INDETERMINADO | **Não alterar** | doc primária não responde; registrar como não verificável |

### Justificativa por CLI

**Claude Code — alterar.** Mecanismo já provado em produção pelo `0c66ecb`. O placeholder é expandido
em runtime pelo próprio CLI, portanto não viola a restrição de caminho absoluto. Estender aos hooks
de attention é aplicar a mesma correção ao resto da superfície.

**Gemini CLI — alterar, por argumento de assimetria.** A doc do Gemini **não** tem uma frase
explícita do tipo "hooks run in the current directory"; ela documenta `GEMINI_CWD` e
`GEMINI_PROJECT_DIR` como duas variáveis distintas e usa `$GEMINI_PROJECT_DIR` em 100% dos exemplos
oficiais de hook. Isso é evidência indireta de dinamismo, não prova.

A decisão **não depende** dessa prova. `$GEMINI_PROJECT_DIR` resolve para a raiz do projeto tanto se
o cwd derivar quanto se não derivar: a mudança **não pode piorar** o comportamento, e corrige o bug
caso ele exista. Ou seja, não é preciso provar que o Gemini está quebrado — basta que o mecanismo
seja confirmado (e é: documentado, com exemplos oficiais). Registrar esta assimetria é o ponto: a
justificativa é "mudança segura por construção", **não** "a doc provou dinamismo".

**Codex CLI — alterar, com dependência explícita de shell e git.** É o único caso em que o argumento
de assimetria **não** se aplica, porque o mecanismo tem pré-condições que podem falhar:

- O Codex **não expõe env var de raiz de projeto** para hooks de repositório (`PLUGIN_ROOT`/
  `PLUGIN_DATA` existem apenas para hooks de *plugin*). O único mecanismo documentado é substituição
  de shell.
- A própria doc reconhece o problema e recomenda a correção:
  *"For repo-local hooks, prefer resolving from the git root instead of using a relative path such as
  `.codex/hooks/...`. Codex may be started from a subdirectory, and a git-root-based path keeps the
  hook location stable."*
- **Pré-condição «executa via shell»:** a doc não afirma o modelo de execução. A evidência é que
  **todos** os exemplos oficiais de hook repo-local usam `$(git rev-parse --show-toplevel)` dentro
  do `command`, e um outro usa expansão de `~` — construções que só funcionam sob shell. Um
  fornecedor não documenta como recomendado um construto que seu próprio runtime não executa.
  Aceito como evidência suficiente **para implementar**, mas não para dispensar verificação: o ML
  do Codex fica com critério de aceite de **verificação empírica** do comando emitido (ver
  Consequences).
- **Pré-condição «dentro de repositório git»:** `git rev-parse --show-toplevel` falha fora de um
  repo. Aceitável — o trackfw governa repositórios por definição. Mas em **submódulo** ou
  **worktree** o comando retorna a raiz daquele submódulo/worktree, que pode não ser onde o
  `scripts/` do trackfw vive. Limitação conhecida e aceita, registrada aqui.
- **Nota de severidade:** o cwd do hook do Codex é o `cwd` da **sessão** ("Commands run with the
  session `cwd` as their working directory"), não um cwd que acompanha os `cd` do agente. O modo de
  falha é iniciar o Codex a partir de um subdiretório — mais raro que o do Claude, e reconhecido
  pelo próprio fornecedor.

### Emenda 1 (2026-08-11, descoberta empírica no ML-3A): Codex exige projeto *trusted*

A verificação empírica do modelo de execução do Codex (feita com `codex-cli 0.147.0` real, não em
shell) revelou uma pré-condição **não documentada na página de hooks do fornecedor**: o Codex só
carrega hooks de projeto se aquele projeto estiver marcado como confiável em `~/.codex/config.toml`
(`[projects."<path>"] trust_level`). Sem isso os hooks do repositório são ignorados **em silêncio**.

Isto **não altera a decisão** — sem trust nenhum hook roda, nem o antigo nem o novo, então a mudança
não piora nada. Mas altera o que o usuário precisa saber: o fix de caminho do Codex só produz efeito
em projeto trusted. Registrar em `docs/cli-parity.md` (ML-8A). Detalhe operacional e armadilha de
teste (usar `$HOME` isolado + `--dangerously-bypass-hook-trust`, nunca escrever no config real do
usuário) em `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`.

**Prova obtida:** com a string nova, rodando o `codex` a partir de um **subdiretório**, o script
disparou — provando execução via shell, expansão do `$(...)` e resolução correta da raiz. **Controle
negativo:** com o caminho relativo antigo, do mesmo subdiretório, falhou (`hook: PreToolUse Failed`,
sem marca) — reproduzindo exatamente a classe de bug corrigida. A pré-condição de shell, que era o
único risco real desta decisão, está confirmada empiricamente e não mais por inferência.

**Cursor — não alterar.** A doc é explícita: *"Project hooks (`.cursor/hooks.json` in a repository):
Run from the project root"*, e o exemplo canônico ensina justamente a usar caminho relativo à raiz
(`.cursor/hooks/script.sh`, não `./hooks/script.sh`). Emitir `$CURSOR_PROJECT_DIR/...` seria mudança
sem defeito correspondente. `CURSOR_PROJECT_DIR` existe e fica registrado aqui caso a semântica
mude no futuro.

**GitHub Copilot CLI — não alterar; já estava correto.** `InjectCopilotHooks` já emite o campo
nativo `"cwd": "."` em **todas** as entradas — verificado por Zeus nos 3 stacks
(`internal/generators/agentfiles.go:698–762`, `npm/src/generators/hooks.js:837–849`,
`pypi/trackfw/generators/hooks.py:610/618/631`). A doc define o campo como *"Working directory for
the command (relative to repository root or absolute)"*, o que pina a execução na raiz do repo. O
caminho relativo dentro de `bash` resolve corretamente **por causa** desse campo. Estava certo por
uso do mecanismo nativo do fornecedor, não por acidente.

**Kiro — não alterar (default de INDETERMINADO).** As 4 páginas oficiais de hooks nunca mencionam o
diretório de trabalho da "Shell Command action" nem expõem env var de raiz. Aplica-se o default já
definido no roadmap: **não alterar**, e registrar em `docs/cli-parity.md` como *"mecanismo de
resolução não verificável em doc primária — mantido relativo"*, com data e as URLs consultadas.
Sobrepor este default exige evidência empírica direta (teste reproduzível no CLI real), nunca
inferência a partir de outro CLI.

### Regra geral derivada

**Preferir o mecanismo nativo do fornecedor, na seguinte ordem:** (1) campo estruturado de working
directory, quando existir (Copilot); (2) placeholder/env var de raiz de projeto expandido em runtime
(Claude, Gemini); (3) substituição de shell (Codex) — último recurso, por introduzir pré-condições;
(4) nenhuma mudança, quando o CLI já resolve contra a raiz (Cursor) ou quando não se pode verificar
(Kiro). **Caminho absoluto materializado no arquivo é proibido em escopo de projeto, em todos os
casos.**

## Consequences

**Positivas**
- 3 dos 6 CLIs não são tocados — menos superfície de mudança, menos risco de regressão em wiring que
  a verificação provou correto.
- Cada mudança é sustentada por doc do próprio fornecedor, não por analogia entre CLIs.
- O caso Copilot vira precedente: **campo estruturado de cwd é melhor que placeholder em string**,
  porque não depende de expansão nem de shell.

**Negativas / riscos aceitos**
- **Heterogeneidade permanente.** Passamos a ter 4 formas diferentes de comando entre os CLIs
  (`$CLAUDE_PROJECT_DIR/…`, `$GEMINI_PROJECT_DIR/…`, `$(git rev-parse …)/…`, relativo puro). Isso
  precisa estar documentado em `docs/cli-parity.md`, senão vira "divergência" aos olhos de quem lê o
  código depois.
- **Codex depende de shell e de git.** Se a pré-condição de shell estiver errada, o hook do Codex
  passa a falhar **sempre**, não só sob cwd derivado — regressão pior que o defeito atual. Por isso
  o ML do Codex tem critério de aceite de **verificação empírica do comando emitido** (executar o
  comando gerado num shell, a partir de um subdiretório, e confirmar que o script roda). Se essa
  verificação não for possível, a decisão para o Codex reverte para **não alterar + registrar como
  não verificável**, mesmo default do Kiro.
- **Kiro fica com dívida conhecida.** Continua com caminho relativo puro e comportamento
  não verificado. Registrado, não resolvido.
- **Cursor e Copilot continuam usando as constantes compartilhadas do Node**
  (`SIGNAL_CMD`/`CLEANUP_CMD`/`GUARD_CMD`, `npm/src/generators/hooks.js:437–439`). Como agora são
  CLIs **verificados corretos**, mutar essas constantes durante os MLs de Claude/Codex/Gemini
  passaria a quebrar wiring que está certo. A divisão dessas constantes em constantes por CLI, já
  exigida pelo roadmap, deixa de ser higiene e vira **requisito de não-regressão**.

## Alternatives Considered

**Caminho absoluto resolvido na injeção** (o que o escopo global já faz). Rejeitado: os arquivos de
settings de projeto são versionados; o path da máquina que rodou `trackfw init` não vale para outro
checkout nem para outro desenvolvedor.

**Um único mecanismo para os 6 CLIs.** Rejeitado por impossibilidade factual: o Codex não expõe env
var de raiz para hooks de repositório, e Cursor/Copilot já resolvem corretamente por meios próprios.
Forçar uniformidade significaria introduzir `$(git rev-parse …)` em CLIs que não precisam — mais
pré-condições, sem defeito correspondente.

**Tornar o script auto-localizável** (o `.sh` descobrir a própria raiz). Rejeitado: o comando é
invocado **por caminho**; se o caminho não resolve, o script nunca chega a executar. A auto-
localização não ajuda no ponto exato em que a falha ocorre.

**Marcar Gemini como INDETERMINADO cautelar** até teste empírico. Rejeitado pelo argumento de
assimetria: o mecanismo está confirmado e a mudança não pode piorar o comportamento; adiar teria
custo sem reduzir risco.

**Adotar `$(git rev-parse --show-toplevel)` também no Claude e no Gemini**, por uniformidade.
Rejeitado: substituiria um mecanismo nativo e garantido por um que depende de shell e de git —
troca de um mecanismo melhor por um pior em nome de simetria estética.
