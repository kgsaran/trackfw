# Análise — Aproveitamento dos PRs #222–#225 (fechados)

> Autor: `hefesto-tf` | Data: 2026-08-31 | Tipo: parecer, não implementação.

## Contexto

`@lourivalgarciajunior` reportou 11 defeitos de Windows (issue #216). Nossa linha de base medida
em runner Windows real (`docs/roadmaps/wip/ROADMAP-2026-08-30-job-de-windows-largo-...md`, ML-1E)
confirma **8/8 REPRODUCED** no escopo coberto (itens 1, 2, 3, 4, 5, 6, 7, 10). KG fechou os quatro
PRs dele por conflito de governança (ciclo próprio já em curso, mesmos arquivos), não por mérito,
e avisou o autor. Esta análise responde apenas: **o que dá para aproveitar de cada um**.

Método: leitura de `gh pr diff <n>` (sem checkout, sem fetch de ref), cruzada contra o código atual
em `pypi/trackfw/`, `internal/`, `npm/src/`, contra a linha de base medida no roadmap `wip/` e contra
o mecanismo de `scripts/windows-repro/python/checks.py`.

**Nota de precisão sobre o enunciado da tarefa:** o instrumento neutraliza o item 1 via
`PYTHONIOENCODING=utf-8`, não `PYTHONUTF8=utf-8` — `checks.py` explicitamente faz
`env.pop("PYTHONUTF8", None)` nos checks de item 1/4, junto com `PYTHONIOENCODING`, para não
mascarar a codepage cp1252 nativa. Isso muda a leitura do achado sobre #223 — ver seção dedicada.

---

## #222 — `$HOME` nos 3 runtimes e o bit de execução em NTFS

**46 arquivos, mas DUAS correções coesas e disjuntas, não uma mistura confusa.** Separei por
conjunto de arquivos:

- **Grupo A — resolução de home (item 2):** `internal/homedir/homedir.go` (novo),
  `npm/src/homedir.js` (novo), `pypi/trackfw/homedir.py` (novo), mais ~30 call-sites trocando
  `os.UserHomeDir()`/`os.homedir()`/`os.path.expanduser("~")` cru pelo helper, `internal/config/config.go`
  e `pypi/trackfw/config.py` (`ExpandPath`/`expand_path`), e `scripts/check-homedir-parity.sh` (gate novo).
- **Grupo B — bit de execução em NTFS na camada do *validator* (item 3):** `internal/validator/goos.go`
  (novo), `validator_credential_guard.go`/`_test.go`, `validator_git_branch_guard.go`/`_test.go`,
  `validator_test.go`, o equivalente em `npm/src/validator/index.js` + testes, e
  `pypi/trackfw/validator.py` + testes.

**Zero overlap de arquivo entre os dois grupos.** São dois MLs independentes e paralelizáveis — a
única razão para estarem no mesmo PR é terem sido descobertos na mesma sessão de investigação.
**Recomendação de aproveitamento: dividir em dois MLs** (ou duas REQs), não pela qualidade — que é
alta nos dois — mas porque cada um tem critério de aceite e gate próprios, e misturá-los num commit
só dificulta a auditoria (`git diff --stat` de um ML devia mostrar só um assunto).

### 1. Diagnóstico bate com o medido?

**Sim, para os dois.** Item 2: a linha de base (ML-1D, "Residual 2") documenta que a mitigação de
`AC12` (igualar `HOME`/`USERPROFILE` para tornar a Camada 1 segura) **mascara o item 2 dentro do
próprio job** — quem mediu o defeito de verdade foi a Camada 2 (`REPRODUCED`). O PR #222 vai direto
na causa que a Camada 2 mede: `os.UserHomeDir()` (Go), `os.homedir()` (Node) e
`os.path.expanduser("~")` (Python) leem `%USERPROFILE%` no Windows e ignoram `$HOME`, então qualquer
suíte que isole via `HOME=<tempdir>` (é o que as nossas fazem) não isola nada lá — produção continua
lendo/escrevendo a home real.

Item 3: **bate exatamente**, arquivo e linha citados no Wave 0 (`hades-tf`) —
`internal/validator/validator_credential_guard.go:377` e `validator_git_branch_guard.go:193`,
`info.Mode()&0111 == 0` sempre verdadeiro em NTFS.

### 2. A correção é correta?

**Sim, correção na origem, não força bruta, em ambos os grupos.**

- **Item 2:** `homedir.Dir()`/`homedir()` preferem `$HOME` e caem para o mecanismo nativo
  (`os.UserHomeDir()`/`os.homedir()`) — string vazia **não** conta como setada (evita todo caminho
  derivado virar relativo silenciosamente). Documentado, testado, e com um detalhe que eu não
  esperava e que é o ponto mais forte do PR: **o Python tem um guard de plataforma que Go e Node
  não têm** (`if sys.platform == "win32": ... else: os.path.expanduser("~")`). A justificativa está
  no próprio docstring e é verificável: em POSIX, `os.path.expanduser("~")` **já** lê `$HOME`, então
  aplicar a preferência incondicionalmente ali não corrigiria nada — e **quebraria** três testes
  existentes que fazem `monkeypatch.setattr("os.path.expanduser", ...)` em vez de mexer na variável
  de ambiente (ele nomeia os três testes e a exceção real que um deles produz —
  `OSError: pytest: reading from stdin while output is captured!` — evidência de medição, não
  suposição). Isto é o padrão certo: **generalizar só onde a generalização é comportamentalmente
  idêntica**, e ele mediu antes de decidir, não assumiu.
- **Item 3:** mesmo padrão já estabelecido pela nossa própria REQ concluída
  (`docs/req/REQ-2026-08-28-modo-de-execucao-perdido...md`, que criou `generators.CurrentGOOS` para
  o `scaffold doctor`). O PR replica o seam (`validator.CurrentGOOS` em Go, `_platform`/`_setPlatformForTest`
  em Node, `_current_platform`/`_set_platform_for_test` em Python) — mesmo nome de conceito, mesmo
  protocolo de override em teste, citado explicitamente no comentário como precedente. **Achado
  lateral que vale registrar:** o comentário em `pypi/trackfw/validator.py` revela que o Python
  usava `os.access(path, os.X_OK)`, não `stat().st_mode & 0o111` como Go/Node — ou seja, **havia uma
  divergência de paridade pré-existente entre os 3 CLIs nesse ponto específico**, anterior a
  qualquer coisa relacionada a Windows, e o PR a corrige de passagem ao unificar o mecanismo.

**Efeitos colaterais em Linux/macOS:** nenhum identificado. O guard do item 3
(`CurrentGOOS != "windows"`/`_platform !== 'win32'`) é estritamente aditivo — em qualquer outro SO o
branch morto nunca dispara, e há teste explícito (`CurrentGOOS = "linux"` forçado) provando que a
regra **continua** disparando fora do Windows. O guard do item 2 é comportamentalmente no-op em
POSIX pelas razões acima.

**Trocou um defeito por outro?** Não encontrei evidência disso em nenhum dos dois grupos.

**Ponto que exige handoff de segurança, não é motivo para rejeitar.** Os dois grupos tocam caminho
de controle de segurança, não só conveniência:

- **Grupo B** suprime a checagem de bit de execução dentro de `validateGuardHookResolvable` —
  a regra que garante que os hooks de *credential guard* e *git branch guard* referenciados
  realmente existem e são executáveis. **Verifiquei que o padrão de supressão silenciosa já é o
  precedente aceito**: `internal/generators/scaffold_doctor.go:322,377` (REQ-2026-08-28, AC5) já
  suprime a mesma checagem no Windows **sem emitir nenhum finding nem mensagem ao usuário** — a
  "declaração" exigida pelo AC5 foi satisfeita no nível do código/REQ, não de saída visível. O PR
  #222 segue exatamente esse mesmo padrão já auditado e aceito neste repositório — **não é uma
  divergência nova**, é consistência com uma decisão já tomada.
- **Grupo A** muda o **âncora de confiança** de `validateGuardGlobalHookResolvable` (Node:
  `os.homedir()` → `homedir()`, i.e., `$HOME` passa a ganhar de `%USERPROFILE%`/API nativa do SO) —
  essa é a função que decide **onde procurar** os hooks globais de credential/git-branch guard para
  auditar. Confirmei que a troca é sistemática: todo `os.UserHomeDir()`/`os.homedir()`/
  `os.path.expanduser("~")` do repositório passa a resolver por variável de ambiente primeiro. Isto
  não é um defeito — é o fix correto do item 2 — mas é uma mudança de superfície de confiança
  (quem controla `$HOME` no ambiente passa a controlar onde a validação de segurança procura),
  merece revisão dedicada de `hades-tf` antes de aceitar, não só a minha leitura de qualidade. Não
  encontrei, na minha revisão, nenhum caminho onde isso amplia um privilégio hoje — mas não é decisão
  que este parecer deva fechar sozinho.

**Handoff nomeado:** `hades-tf` — revisar se a mudança de âncora de confiança em
`validateGuardGlobalHookResolvable`/`LoadGlobalAgentModels` (Grupo A) abre alguma superfície nova em
ambiente onde `$HOME` é controlável por quem não controla o SO (ex.: CI de terceiro, container
compartilhado) — antes de portar o Grupo A para produção.

### 3. Paridade dos 3 CLIs?

**Respeitada nos dois grupos**, e de forma verificada, não assumida — cada grupo vem com gate
estático que varre por usos crus fora do helper (`scripts/check-homedir-parity.sh` grepa
`os.homedir()`, `os.path.expanduser`/`Path.home()`, `os.UserHomeDir()` fora dos arquivos-helper) e
um teste dinâmico rodando os três binários reais com `HOME` apontado para um tempdir, comparando a
saída. Título do PR diz só `fix(windows)` sem menção a runtime único — corretamente, porque
**os dois grupos tocam os três CLIs**.

### 4. Tem teste? Falsifica nas duas direções?

**Sim, e bem — no padrão que este próprio projeto adota** (`ML-2A`/`ML-1B` no histórico recente
fazem exatamente esse tipo de falsificação simétrica). Para o item 3, o teste Go novo
(`TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao`) força `CurrentGOOS = "windows"`
e prova que a regra **não** dispara para script sem bit +x, **mas continua disparando** para script
inexistente (a checagem de existência não é enfraquecida) — e o teste vizinho já existente foi
alterado para fixar `CurrentGOOS = "linux"` explicitamente, provando que a regra **ainda** dispara
fora do Windows. Falsificação nas duas direções, literal. `check-homedir-parity.sh` tem uma nota
que também é metodologicamente correta: "a falsificação que ninguém vigia é o **Go** regredir,
porque ele já está certo" — cobre os três runtimes de propósito, não só os dois que precisavam de
correção.

### 5. O que aproveitar

**Grupo A (home): correção inteira**, como dois MLs (um por área de arquivo tocada, ou um ML só —
não há razão técnica para separar mais). **Grupo B (exec bit no validator): correção inteira**,
ML próprio.

**Achado que muda o roteamento — precisa de REQ nova, não é a mesma REQ já fechada.** A
`REQ-2026-08-28-modo-de-execucao-perdido...` está `status: done` e seu Negative Scope diz
literalmente *"Não adiciona verificação de modo aos artefatos do manifesto — é outra superfície,
com outro mecanismo"*. O escopo dela foi só `internal/generators/scaffold_doctor.go` (o `doctor`
comparando artefato gerado contra template). `validator_credential_guard.go`/`validator_git_branch_guard.go`
são a checagem que roda dentro de `trackfw validate` sobre hooks **já commitados** — superfície
diferente, nunca coberta. Confirmei lendo o código atual: as duas linhas citadas pelo Wave 0
continuam com `info.Mode()&0111 == 0` sem guard nenhum hoje. **Este é o gap real e vivo que produz
o item 3 REPRODUCED na nossa linha de base.**

---

## #223 — Força UTF-8 na saída do CLI Python, elimina dependência de `PYTHONUTF8`

### 1. Diagnóstico bate?

**Sim, na origem exata.** `pypi/trackfw/cli.py:207` tem `description="trackfw — governed software
delivery framework\nADR → REQ → ROADMAP → kanban"` — o caractere `→` (U+2192) não é representável em
cp1252. `parser.print_help()` roda quando `args.command is None`. É literalmente o caminho de
produção que o Wave 0 (`hades-tf`) identificou como não exercitado por nenhum teste existente
(o único teste de `--help` cobria um subparser, que não renderiza a `description=` do parser raiz).

### 2. A correção é correta?

**Sim — correção na origem, não neutralização de ambiente.** `_force_utf8_output()`, chamada logo no
início de `main()`, faz `stream.reconfigure(encoding="utf-8", errors="replace", newline="\n")` em
`sys.stdout`/`sys.stderr`, **incondicionalmente**, sem depender de nenhuma variável de ambiente.
Isso é estritamente melhor do que a alternativa óbvia (documentar `PYTHONUTF8=1` como pré-requisito):
resolve dentro do processo, funciona mesmo se quem invoca esquecer a env var, e traz o Python para o
mesmo comportamento que Go e Node.js já têm (escrevem bytes UTF-8 direto, sem consultar codepage —
verificado que é assim: nenhum dos dois consulta `chcp`/codepage do console em nenhum lugar do
código). `errors="replace"` degrada em vez de abortar; o `getattr(stream, "reconfigure", None)` com
checagem prévia evita levantar em testes/pipelines que substituem `sys.stdout` por objeto sem esse
método — cobre exatamente o caso do `TestForceUtf8Output` novo. Não há força bruta nem efeito
colateral identificado em Linux/macOS: `reconfigure(encoding="utf-8", ...)` num terminal que já é
UTF-8 é um no-op semântico.

**Item 4 continua sem correção.** O mecanismo do item 4 (`scripts/check-parity-contract-coverage.sh`,
gate de shell que lê `docs/cli-parity.md` e imprime `→` via `print()` isolado, sem passar por
`cli.py`/`main()`) não é tocado por este PR — é um `print()` cru fora do CLI. Confirmei que
`checks.py:cmd_cp1252_print` testa exatamente esse `print()` isolado, não o caminho de `main()`. Item
4 permanece um gap real após #223.

### 3. Paridade dos 3 CLIs?

**Correta como está — é um fix Python-only porque o defeito é Python-only.** O próprio docstring
declara que Go/Node já escrevem UTF-8 sem consultar codepage; não encontrei nenhuma lógica de
detecção/reconfiguração de encoding em `internal/` ou `npm/src/` — não há nada para eles corrigirem
aqui. Isto **não** é uma lacuna de paridade, é o caso correto de correção single-runtime porque o
defeito é single-runtime (mesmo raciocínio que a Wave 0 aplicou ao item 6 sobre `isatty`).

### 4. Tem teste? Falsifica nas duas direções?

**Sim, e com um mecanismo elegante de reprodução determinística cross-plataforma.**
`TestCliEmConsoleCp1252` roda o binário real via subprocess com `PYTHONIOENCODING=cp1252` forçado —
simula o console cp1252 do Windows **em qualquer SO**, sem precisar de runner Windows para rodar o
teste em CI Linux todo dia. Testa `--help`, `status`, `validate`, cada um checando
`returncode == 0` e ausência de `UnicodeEncodeError` no stderr. Isso falsifica na direção "sem a
correção, quebra": sem `_force_utf8_output()` no `main()`, os três comandos morreriam com
`UnicodeEncodeError` sob esse env forçado — eu não rodei o teste antes/depois do patch (proibido
fazer checkout), mas a leitura do código antes da mudança (`main()` sem a chamada) e a mensagem
`→` na `description=` deixam a causalidade direta, não hipotética. A direção "não quebra
operação legítima" está coberta por `test_reconfigura_com_utf8_e_replace` (chamadas corretas de
`reconfigure`) e por `test_stream_sem_reconfigure_nao_levanta` (stream sem o método não derruba nada
— cobre exatamente o cenário de teste/pipeline). `test_barrier_contract.py`/`test_changelog.py`
ganham `encoding="utf-8"` explícito no `subprocess.run(..., text=True)` — correção correlata e bem
diagnosticada: sem isso, o **teste** (não o CLI) decodifica a saída do filho pelo locale do SO
(cp1252 no Windows), produzindo mojibake do travessão `U+2014` numa mensagem pinada, mesmo com o
filho já escrevendo UTF-8 corretamente. É um bug de harness de teste separado do defeito de produto,
corretamente isolado num diff mínimo.

### 5. Impacto sobre o instrumento `windows-repro` e a linha de base 8/8 — ACHADO PRINCIPAL

**Não há conflito. #223 não muda a mecânica de medição — ele torna a neutralização em `checks.py`
redundante, mas inofensiva, e o item 1 passa a medir corretamente "corrigido".**

Cadeia verificada:

- `checks.py` mede o item 1 (`cmd_help`) e o item 4 (`cmd_cp1252_print`) fazendo
  `env.pop("PYTHONUTF8", None); env.pop("PYTHONIOENCODING", None)` — **para preservar** a codepage
  nativa do console e observar o crash. Depois de #223, isso **continua funcionando exatamente
  igual**: o subprocesso spawnado por `cmd_help` chama `main()`, que agora reconfigura o stream
  **dentro do processo**, via `.reconfigure()`, **independente de qualquer variável de ambiente**.
  Ou seja: mesmo com as env vars removidas pelo instrumento, o processo filho não crasha mais —
  porque o código foi corrigido, não porque o instrumento parou de testar a condição certa. O
  veredito de `cmd_help` viraria `ABSENT` (defeito corrigido), que é exatamente o resultado
  desejado — **prova de correção pelo próprio instrumento que mediu o defeito**, sem precisar mudar
  uma linha do instrumento.
- Os itens 5 e 6 (`cmd_crlf`, `cmd_isatty`) hoje **precisam** neutralizar o item 1 via
  `env["PYTHONIOENCODING"] = "utf-8"` **só nesses dois subprocessos**, porque o `init` chamado por
  eles passa por `init_gen.py`, que também imprime `→`/checkmarks e morre em cascata antes de
  alcançar CRLF/isatty (documentado no ML-1C do roadmap: *"Um defeito está mascarando a medição de
  outros dois"*). Depois de #223, essa neutralização **deixa de ser necessária** — `main()` já
  reconfigura os streams antes de qualquer print — mas a linha continua no `checks.py` e não quebra
  nada: setar `PYTHONIOENCODING=utf-8` num processo que já reconfigura via API é redundante, não
  conflitante. **Não requer alteração no instrumento para continuar correto.**
- **Efeito líquido na linha de base 8/8:** depois de #223 mergeado (sozinho, sem #224/#225), o item
  1 mudaria de `REPRODUCED` para `ABSENT` — a contagem cairia para **7/8** no escopo medido, e essa
  queda é o **sinal correto de que a correção funcionou**, não uma regressão do instrumento. O item
  4 continuaria `REPRODUCED` (mecanismo diferente, não tocado). Isto precisa estar documentado no
  roadmap **antes** de mergear qualquer correção de item 1, porque o critério atual do roadmap
  (`ML-1E`, AC) é *"a contagem segue 8 REPRODUCED"* — esse critério é para a fase de **instrumento**,
  não sobrevive à primeira correção de defeito real, e não deveria: a queda de contagem citando o
  item exato é o formato correto de reportar avanço, mirando o padrão que o próprio roadmap já usa
  (ex.: "Reproduzidos: 8 | Inconclusivos: 0 | ...").

**Conclusão sobre a preocupação levantada:** #223 **não invalida nem contamina** a medição — ao
contrário, é o primeiro caso real onde a aplicação de uma correção de defeito muda o veredito do
próprio instrumento de forma **rastreável e esperada**, exercitando exatamente o papel que a camada
2 foi desenhada para cumprir (REQ-2026-08-30, "a camada 2... prova que não voltou").

**Residual declarado, não escondido:** `pypi/tests/__init__.py` chama `_force_utf8_output()` **na
importação do pacote de testes inteiro**, por design — o próprio docstring explica que sem isso 16
testes que chamam funções de biblioteca direto (fora de `main()`) quebrariam em cp1252. Efeito
colateral aceito por essa escolha: **a suíte inteira fica permanentemente cega a falhas de encoding
in-process** depois deste PR — nenhum teste Python vai mais crashar por `UnicodeEncodeError` de
verdade, porque o processo de teste sempre reconfigura os streams antes de qualquer coisa rodar. É
a mesma forma de residual que a própria Wave 0 (`hades-tf`) apontou na fixture `HOME` do
`conftest.py` (isolamento que também é global e também apaga um sinal que só apareceria sem ele) —
não é um defeito do PR, mas precisa ficar registrado como residual explícito no roadmap que herdar
esta correção, não descoberto depois por quem for investigar por que um teste de encoding não pega
mais nada.

---

## #224 — `isatty()` devolve `True` para `NUL` no Windows

**Nota de dependência:** o diff deste PR **inclui** as mesmas mudanças de `pypi/trackfw/cli.py`,
`pypi/tests/__init__.py`, `test_barrier_contract.py`, `test_changelog.py` e `test_cli_encoding.py`
de #223 — é uma branch empilhada sobre #223, não independente. Ao aproveitar, tratar como **#223 +
este diff incremental** (`pypi/tests/test_scope_resolution.py`, `pypi/trackfw/commands/init.py`,
`pypi/trackfw/commands/thirdparty.py`, `pypi/trackfw/commands/validate.py`,
`pypi/trackfw/integrations/command.py`, `pypi/trackfw/tty.py` novo, `scripts/check-tty-detection.sh`
novo).

### 1. Diagnóstico bate?

**Sim, exatamente.** `NUL` no Windows é um character device, e o Windows classifica character device
como TTY — `sys.stdin.isatty()` mente `True`. É o item 6 da issue, e a causa raiz descrita no
`tty.py` novo é a mesma da Wave 0 e da linha de base (`REPRODUCED`, medido via `init` sem
`--identity-preset` morrendo com `EOF when reading a line` sob `stdin=DEVNULL`).

### 2. A correção é correta?

**Sim, e é a melhor solução das quatro que li — porque replica exatamente o mecanismo que o Go já
usa, em vez de inventar um novo.** `trackfw.tty._windows_is_console()` chama
`GetConsoleMode` via `ctypes`/`msvcrt.get_osfhandle`, citando `charmbracelet/x/term` (a lib que o
nosso Go usa) como referência de comportamento. **Verifiquei a citação**: `internal/commands/*.go`
importa `cbterm "github.com/charmbracelet/x/term"` e chama `cbterm.IsTerminal(uintptr(os.Stdin.Fd()))`
em cinco arquivos (`roadmap.go`, `integrations_flags.go`, `adr.go`, `req.go`, `init.go`) — confirmado
no código atual, `go.mod` tem `v0.2.1` (o PR cita `v0.2.2` no comentário — drift de versão trivial no
comentário, não no comportamento). A ideia central — "`isatty()` é a base, `GetConsoleMode` só
**estreita** o resultado, e só no Windows" — é semanticamente idêntica ao que o Go já faz
internamente naquela lib. **Node não precisa de correção**: `process.stdin.isTTY` já embute a
checagem equivalente via libuv (`uv_guess_handle`) e não reproduz o defeito — não encontrei nenhum
report do autor da issue nem evidência no código apontando o contrário.

Falha sempre para `False` em qualquer exceção (`stream` sem `fileno()`, típico de teste/pipeline) —
comportamento seguro: nunca promptar por engano é preferível a promptar indevidamente. Sem efeito
colateral identificado em Linux/macOS (o branch `sys.platform == "win32"` nunca dispara lá; o
`isatty()` original continua sendo a primeira checagem em qualquer plataforma).

### 3. Paridade dos 3 CLIs?

**Respeitada — Python estava divergindo, ficou igual.** Go e Node já tinham a garantia; só o Python
não. Não é fix Python-only por preguiça, é fix Python-only porque **era o único que precisava**.

### 4. Tem teste? Falsifica nas duas direções?

**Sim, e com uma auto-crítica que eu valorizo especificamente**: `scripts/check-tty-detection.sh`
documenta no próprio comentário que a **primeira versão do gate era vácua** — rodava
`trackfw init </dev/null` e exigia `exit 0` nos três, "e passava com e sem a correção, porque o
`init` sem `--ai-tools` não chega a alcançar o wizard". A versão final "pergunta direto ao predicado
que a produção consulta" (`MEDE O DISCRIMINANTE, NAO O COMANDO", no comentário — quase textualmente o
mesmo princípio que orientou nosso próprio `ML-1A→ML-2A`, "regra do hook relativo falsificada nas
duas direções", nos commits recentes deste repositório). O script:
- Reproduz a mentira via `</dev/null` (equivalente a `NUL` sob Git Bash) e verifica que
  `stdin_is_interactive()` corrige (`False`) onde `sys.stdin.isatty()` cru mentia (`True`).
- **Tem um caminho de "efeito não exercitado" com mensagem explícita**, em vez de dar falso-positivo,
  quando roda num sistema onde o `isatty()` cru já não mente (mesmo padrão de honestidade do
  `symlinkOrSkip` que nosso `ML-2A` implementou).
- Tem checagem estática (`grep -rn 'isatty()'` fora do helper) para impedir regressão por um novo
  call-site cru.
- `test_scope_resolution.py` — os 6 monkeypatches trocados de `sys.stdin.isatty` para
  `integrations_command.stdin_is_interactive` são coerentes com a mudança de seam: sem essa
  atualização os testes existentes quebrariam (mockavam o método errado após a refatoração), o PR
  os corrigiu junto — nada ficou quebrado por causa da migração.

### 5. O que aproveitar

**Correção inteira**, como ML separado de #223 (mas sequenciado depois, já que herda o diff dele).
`pypi/trackfw/tty.py` é um módulo novo, autocontido, sem necessidade de mudança em Go/Node.

---

## #225 — CRLF nos geradores Python

### 1. Diagnóstico bate?

**Sim.** `open(path, "w")` sem `newline=` usa `newline=None`, que traduz `\n` para `os.linesep` —
CRLF no Windows. Go e Node escrevem bytes direto (sem essa tradução). Bate com a Wave 0: "38+ sites
sem `newline=`", e com a linha de base (`REPRODUCED`).

### 2. A correção é correta?

**Sim — e o achado sobre o próprio teste existente é o mais valioso do PR.** O docstring do teste
novo (`test_generators_write_lf.py`) aponta que `test_generators_roadmap.py:774-836` **já** lê os
artefatos gerados em modo `"rb"` (bytes crus), mas a asserção é só de idempotência
(`bytes_before == bytes_after`) — "o que nunca acusa CRLF, porque as duas leituras saem igualmente
erradas". **Verifiquei o raciocínio e ele é correto**: um teste de idempotência não é um oráculo de
conteúdo — CRLF vs LF em ambas as leituras dá `bytes_before == bytes_after` de qualquer jeito. É
exatamente o tipo de "teste verde que não prova nada" que este projeto já persegue em outras partes
do roadmap (ex.: item 5 estava `INCONCLUSIVE` por cascata do item 1 — aqui o problema é diferente,
mas da mesma família: sinal aparentemente presente, mas sem oráculo). A correção em si é
`newline="\n"` explícito em **todo** `open(..., 'w'/'a', ...)` e `.write_text(...)` de texto em
`pypi/trackfw/` — busca sistemática, não pontual (o PR toca `generators/`, `commands/`, `validator.py`,
`integrations/scaffold_doctor.py` — coerente com "38+ sites").

Em Linux/macOS: **no-op absoluto**, porque `os.linesep` já é `\n` lá — `newline="\n"` explícito
produz byte a byte o mesmo resultado que `newline=None` produzia antes. Nenhum efeito colateral
possível fora do Windows, e o próprio arquivo de teste declara isso: nos testes CRLF, "em
Linux/macOS estes testes passam com ou sem a correção... valem como guarda de regressão, não como
reprodução" — honestidade de escopo, exatamente o padrão que o `ROADMAP-2026-08-30` cobra dos
próprios agentes.

**Gate estático novo (`scripts/check-python-writes-lf.sh`) é o ponto mais forte do PR**, porque
ataca o motivo estrutural do defeito nunca ter sido pego antes: *"a CI do upstream é Linux e nunca vê
o defeito, então todo arquivo novo que vier de lá chega sem `newline=`"*. Em vez de depender do job
de Windows (caro,~25min pela nossa própria medição) para pegar regressão futura, ele varre
estaticamente todo `open()`/`.write_text()` em modo texto de escrita sem `newline=` explícito — roda
em qualquer SO, inclusive Linux, e pega no merge. O parser é um contador de profundidade de
parênteses respeitando aspas/escapes (não regex ingênuo) — funcional para o estilo de código deste
repositório, com exclusão correta de modos binários (`rb`/`wb`/`ab`).

### 3. Paridade dos 3 CLIs?

**Correta como está.** Fix Python-only porque Go/Node já escrevem bytes crus sem tradução de
newline — não há nada a corrigir lá. Confirmei que o PR não toca `internal/` nem `npm/src/`.

### 4. Tem teste? Falsifica nas duas direções?

**Sim, com o mesmo padrão honesto de declarar onde falsifica de verdade.** `test_generators_write_lf.py`
lê bytes crus (`_bytes()`), afirma **ausência** de `\r\n` **e presença** de pelo menos um `\n` — a
segunda asserção evita que um arquivo vazio ou sem quebra de linha alguma passasse trivialmente.
Cobre `generate_adr`, `generate_req`, `generate_roadmap`, `new_note` (+ o índice do vault) —
caminhos de produção reais, sem mock. A falsificação "nasce vermelho sem a correção" só é real em
Windows (declarado explicitamente, não escondido); em Linux/macOS os mesmos testes funcionam como
guarda de regressão. O gate estático tem sua própria falsificação implícita: reverter qualquer um
dos ~40 `newline="\n"` faria o script acusar a linha exata — não testei isso na prática (proibido
tocar código), mas a lógica do parser é direta o suficiente para confiar sem rodar.

### 5. O que aproveitar

**Correção inteira**, ML próprio, sem dependência dos outros três PRs (não toca `cli.py`, `tty.py`
nem os arquivos do #222).

---

## Tabela de decisão

| PR | Item(ns) da issue #216 | Aproveitar | Onde entra | Esforço estimado |
|---|---|---|---|---|
| #222 (Grupo A — home) | 2 | **Inteiro** | REQ/ML novo — não há REQ existente cobrindo isto | Baixo (portar diff, revalidar gate `check-homedir-parity.sh` nos 3 CLIs) |
| #222 (Grupo B — bit de execução no *validator*) | 3 | **Inteiro** | REQ/ML novo, **distinto** de `REQ-2026-08-28` (que só cobriu `scaffold_doctor.go`) | Baixo (padrão já estabelecido, só replicar) |
| #223 | 1 | **Inteiro**, incl. testes | REQ/ML novo | Baixo — mas **documentar no roadmap wip** que o item 1 passa de REPRODUCED→ABSENT após merge, e que isso é esperado, antes de reabrir a barreira |
| #224 | 6 | **Inteiro**, incl. testes | REQ/ML novo, **sequenciado depois de #223** (diff empilhado) | Baixo-médio (módulo novo `tty.py`, 6 call-sites a migrar) |
| #225 | 5 | **Inteiro**, incl. testes e gate estático | REQ/ML novo | Baixo (mecânico, ~40 sites + 1 gate) |

**Nenhum dos quatro PRs merece "nada".** Todos batem o diagnóstico com a linha de base medida,
corrigem na origem (não força bruta), preservam paridade — corrigindo, aliás, uma divergência de
paridade pré-existente e não relacionada a Windows (item 3, Python usava `os.access(X_OK)` em vez do
mecanismo de bits usado por Go/Node) — e vêm com teste que falsifica nas duas direções, com o mesmo
nível de rigor metodológico que este projeto já pratica internamente (medir o discriminante, não o
comando; declarar residual em vez de fingir cobertura).

## Achado mais importante: `PYTHONUTF8`/`PYTHONIOENCODING` e a linha de base 8/8

**#223 não elimina nem contamina o mecanismo de medição.** A neutralização em `checks.py` usa
`PYTHONIOENCODING=utf-8` (não `PYTHONUTF8`, que aliás o script explicitamente **remove** do ambiente
para o item 1/4). `_force_utf8_output()` reconfigura os streams **dentro do processo**, via
`.reconfigure()`, sem depender de nenhuma env var — então:

- O item 1 passaria de `REPRODUCED` para `ABSENT` **corretamente**, refletindo a correção real.
- O item 4 **não muda** (mecanismo diferente — gate `.sh`, não CLI).
- Os itens 5/6, que hoje neutralizam o item 1 via `PYTHONIOENCODING=utf-8` só para conseguir medir
  em isolamento, continuam funcionando sem alteração — a neutralização vira redundante, não
  conflitante.
- A contagem "8/8 REPRODUCED" cai para 7/8 **depois** de #223 mergear, e essa queda precisa ser
  documentada no roadmap como sinal de correção, não como regressão do instrumento — o critério de
  aceite atual do `ML-1E` ("a contagem segue 8 REPRODUCED") é válido só enquanto nenhum defeito real
  for corrigido; ele deveria evoluir para "cada item que sair de REPRODUCED é explicado", que é
  exatamente a frase que o próprio `ML-1E` já usa para mudança de contagem.

## Pré-condição dura, antes de qualquer merge destes quatro

A Barreira final do `ROADMAP-2026-08-30-job-de-windows-largo...md` (wip) exige explicitamente
**"o job de Windows reprovando pelos motivos esperados e mapeados"** para fechar aquela REQ. Isso
não é compatível, por construção, com mergear qualquer um dos quatro PRs enquanto essa REQ segue
`wip` — cada correção que entra muda um veredito de `REPRODUCED` para `ABSENT`, e a barreira daquela
REQ ainda lê "reprovar pelos motivos mapeados" como critério de fechamento.

**Portanto, antes do primeiro merge:** ou a `REQ-2026-08-30` fecha (`done`) primeiro com a linha de
base 8/8 citável como estava, ou seu critério de aceite muda explicitamente de "a contagem segue 8
REPRODUCED" para "cada item que sair de REPRODUCED é explicado" — a mesma frase que o próprio
`ML-1E` já usa para mudança de contagem. Sem essa decisão registrada primeiro, o primeiro PR
aproveitado quebra a barreira daquela REQ silenciosamente, e a próxima pessoa a rodar o job largo
vê uma queda de contagem sem saber se é regressão do instrumento ou correção de produto.

## Gap a fechar em paralelo: `docs/cli-parity.md` não documenta os contratos que #223 e #225 passam a impor

Busquei `newline`/`CRLF`/`UTF-8`/`cp1252` em `docs/cli-parity.md` — a única ocorrência é sobre
**leitura** de conteúdo com CRLF ao fazer parse de roadmap (ML-3C, REQ-2026-08-28), não sobre
**escrita**. Não há contrato documentado dizendo "os três runtimes escrevem artefato em LF" nem
"os três runtimes escrevem UTF-8 na saída, independente da codepage do console". #225 introduz um
gate (`check-python-writes-lf.sh`) que **impõe** o primeiro contrato e #223 corrige o CLI para
cumprir o segundo — nenhum dos dois está registrado em `docs/cli-parity.md` hoje. Isto é exatamente
o tipo de drift que aquele documento existe para impedir (ver a seção "Gate assertion" do próprio
arquivo, sobre a divergência de versão que sobreviveu por causa de contrato frouxo). **Vale um ML
próprio, pequeno**: acrescentar as duas linhas de contrato a `docs/cli-parity.md` junto com o merge
de #223/#225, não depois. Nota lateral que vale registrar como ironia funcional: o item 4 **é** o
próprio `check-parity-contract-coverage.sh` — o gate que audita cobertura de `cli-parity.md` —
morrendo em cp1252 ao ler esse mesmo arquivo; estamos adicionando contrato novo a um documento cujo
gate de cobertura está, ele mesmo, cego no Windows até o item 4 ser corrigido (nenhum dos quatro PRs
corrige o item 4).

## Ordem de ataque recomendada

O item 1 (cp1252) bloqueia a medição limpa dos itens 5 e 6 hoje (mascarado via neutralização em
`checks.py`, não por acaso). Ordem sugerida, **depois** da pré-condição acima estar satisfeita:

1. **#222 Grupo B (bit de execução no validator)** e **#225 (CRLF)** — independentes entre si e de
   tudo mais, sem dependência de ordem, podem ir em paralelo.
2. **#222 Grupo A (home)** — independente dos outros três, pode entrar em paralelo com o passo 1,
   mas só depois do handoff de `hades-tf` sobre a mudança de âncora de confiança (ver seção #222).
3. **#223 (cp1252/UTF-8)** — depois dos passos acima (ou em paralelo, já que não compartilha
   arquivo com nenhum).
4. **#224 (isatty/NUL)** — depois de #223, porque o diff dele **inclui** o de #223 (branch
   empilhada). Fazer isso na ordem errada (ex.: portar #224 sem antes ter #223 no branch) exige
   separar manualmente os dois conjuntos de arquivo, o que é mecânico mas evitável mudando a ordem.

Com os quatro portados, a linha de base do `ROADMAP-2026-08-30` sai de "8/8 REPRODUCED" (instrumento
provando que os defeitos existem) para **3 REPRODUCED remanescentes: item 4 (gate de cobertura,
ainda sem fix em nenhum destes PRs), item 7 (semântica de shell divergente, `sh -c` vs `cmd.exe`,
mecanismo não coberto por nenhum dos quatro) e item 10 (separador de SO no frontmatter da REQ,
confirmado independentemente em Go e Node pelo `ML-1E`, também não coberto)** — mais os itens 8/9,
já fora de escopo declarado desde a Wave 0. Itens 1, 2, 3, 5 e 6 saem de `REPRODUCED`. Essa é
exatamente a evolução que a `REQ-2026-08-30` previu: "as correções vêm depois, cada uma com sua REQ,
usando o job como evidência".
