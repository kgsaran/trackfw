# Revisão de segurança — âncora de confiança de `validateGuardGlobalHookResolvable` vira `$HOME`-first (Grupo A do PR #222)

> Autor: `hades-tf` | Data: 2026-08-31 (Wave 0, ML-0A)
> Escopo: apenas o Grupo A do `gh pr diff 222` (resolução de home) — `internal/homedir/homedir.go`,
> `npm/src/homedir.js`, `pypi/trackfw/homedir.py`, e os ~30 call-sites que passam a usá-los, com foco
> em `validateGuardGlobalHookResolvable`/`validateGuardGlobalScriptIntegrity`
> (`internal/validator/validator_git_branch_guard.go`, equivalentes Node/Python) e em `UpdateHarness`
> (`internal/generators/update.go`, `globalCredentialGuardScriptPath`/`globalGitBranchGuardScriptPath`
> em `agentfiles.go`). **Não escopo:** Grupo B (bit de execução NTFS), #223/#224/#225 — já avaliados
> por `hefesto-tf`.
> Método: leitura de `gh pr diff 222` (sem checkout, sem fetch de ref) + leitura de `gh issue view 216`
> (fonte primária do reporter) + leitura do código atual em `internal/`, `npm/src/`, `pypi/trackfw/` +
> leitura direta da implementação de `os.UserHomeDir()` (`$(go env GOROOT)/src/os/file.go`, local,
> Go 1.x instalado nesta máquina) e de `ntpath.expanduser` (`python3 -c "import ntpath, inspect;
> print(inspect.getsource(ntpath.expanduser))"`, também local — `ntpath` roda em qualquer SO porque é
> só o módulo de parsing de caminho, não faz syscall) + verificação externa do `os.homedir()` do
> Node.js no Windows (fonte citada, Node não tem módulo `ntpath`-equivalente inspecionável localmente
> em macOS). **Nenhuma medição em runner Windows real** — não disponível neste ambiente (macOS). Onde
> nem código-fonte nem literatura fecham a pergunta, digo isso explicitamente em vez de assumir.

## Veredito

**(b) Segura como está no eixo que a tarefa nomeou (processo pai hostil), mas com uma ressalva
concreta e diferente do eixo perguntado, que a Wave 2 precisa incorporar como critério de aceite
mensurável, não como suposição.** Não é um vetor de escalada de privilégio novo. É um vetor de
**divergência silenciosa entre o que o trackfw grava/audita e o que o CLI de agente real (Claude
Code, Codex, Gemini — não-patcheado) efetivamente lê** em ambientes Windows onde `$HOME` existe mas
não é garantidamente o mesmo caminho que `%USERPROFILE%`. Ver "O que a Wave 2 deve fazer" no fim.

---

## 0. Correção de premissa — a troca não é "API nativa do SO → env var". As duas pontas já eram env var.

O enunciado do handoff (e a análise do `hefesto-tf`) descreve a mudança como indo de "API nativa do
SO" para "`$HOME` env-var-first". **Verifiquei e essa framing não é precisa nos três runtimes**, e a
imprecisão muda o cálculo de risco:

- **Go — verificado lendo o fonte real da stdlib instalada nesta máquina**
  (`$(go env GOROOT)/src/os/file.go`, `func UserHomeDir()`):
  ```go
  func UserHomeDir() (string, error) {
      env, enverr := "HOME", "$HOME"
      switch runtime.GOOS {
      case "windows":
          env, enverr = "USERPROFILE", "%userprofile%"
      ...
      if v := Getenv(env); v != "" { return v, nil }
      ...
  ```
  No Windows, `env` vira literalmente `"USERPROFILE"` e a função devolve `os.Getenv("USERPROFILE")`.
  **Não há nenhuma chamada a API do Windows** (`SHGetKnownFolderPath` ou equivalente) em nenhum ramo.
  É env var contra env var, confirmado no código-fonte, não de memória.
- **Python — verificado lendo o fonte real de `ntpath.expanduser` instalado nesta máquina**
  (`ntpath` é só o módulo de parsing de caminho estilo-Windows do CPython; roda em qualquer SO
  porque não faz syscall, só string handling — por isso dá para inspecionar em macOS):
  ```python
  if 'USERPROFILE' in os.environ:
      userhome = os.environ['USERPROFILE']
  elif 'HOMEPATH' not in os.environ:
      return path
  else:
      drive = os.environ.get('HOMEDRIVE', '')
      userhome = join(drive, os.environ['HOMEPATH'])
  ```
  Confirmado: `USERPROFILE` primeiro, fallback `HOMEDRIVE`+`HOMEPATH`. **Não há fallback para `HOME`
  em nenhum ramo** — se nem `USERPROFILE` nem `HOMEPATH` estiverem setados, a função devolve o `path`
  inalterado (não expande `~`), silenciosamente.
- **Node.js**: não tenho um módulo local inspecionável equivalente a `ntpath` para o `os.homedir()`
  do Node no Windows (implementação em C++/libuv, não Python puro) — apoiei-me em fonte externa (ver
  Sources), que indica `USERPROFILE` primeiro, fallback `HOMEDRIVE`+`HOMEPATH`, e só depois `HOME`
  como terceiro nível. Diferente dos outros dois runtimes, não verifiquei isso lendo código-fonte
  diretamente — fica marcado como a única perna desta seção apoiada em fonte secundária, não em
  leitura direta.

**O que muda de fato, então, é só a *prioridade* entre duas variáveis de ambiente que já existiam,
e só no Windows** (no POSIX, `os.UserHomeDir()`/`os.homedir()`/`expanduser` já preferem `$HOME` hoje —
o Grupo A é no-op comportamental lá, confirmado pela análise do `hefesto-tf` e por mim
independentemente). Isso **não invalida** a pergunta do handoff — "quem controla `$HOME` e o que
ganha" continua válida — mas muda a resposta: não existe hoje, e não existirá depois do PR, nenhum
mundo em que a resolução de home do trackfw no Windows **não** seja uma variável de ambiente lida do
processo. A superfície "processo pai controla o env" já era 100% a âncora de confiança antes deste
PR, só que sob o nome `USERPROFILE`.

## 1. Pergunta do handoff — processo pai hostil controlando `$HOME`

**Não é uma capacidade nova.** Um processo pai que já consegue setar `$HOME` para o processo filho
(`trackfw`) já conseguia, antes deste PR, setar `USERPROFILE` para o mesmo processo filho e obter
exatamente o mesmo efeito — redirecionar onde `globalCredentialGuardScriptPath`,
`globalGitBranchGuardScriptPath`, `validateGuardGlobalHookResolvable`,
`validateGuardGlobalScriptIntegrity` e `UpdateHarness` leem/escrevem. O PR não abre uma porta nova
para esse ator; troca qual variável especificamente ele precisa setar, no Windows. No POSIX, zero
mudança — `$HOME` já era a âncora, nos dois lados, antes e depois.

**Consequência prática desse ator (inalterada pelo PR):** um processo pai que controla o ambiente do
filho pode fazer `trackfw validate`/`update harness` ler/escrever um `.trackfw/scripts/` e
`~/.claude/settings.json` forjados em vez dos reais — mas isso já é reconhecido e aceito pelo
`ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita...` e pela minha
própria revisão de 2026-08-18 (`docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md`,
seção B): "controle que mora onde o agente escreve não é controle" pressupõe que o agente não escreve
onde o controle mora — se ele controla o ambiente inteiro do processo, a garantia sempre foi
condicional, e o modelo de ameaça já trata "processo pai controla env" como fora do que este guard se
propõe a impedir. **Não há degradação nova neste eixo.**

`$HOME` ausente ou vazio: os três runtimes caem corretamente para o mecanismo nativo
(`os.UserHomeDir()`/`os.homedir()`/`expanduser("~")`) — verificado lendo `internal/homedir/homedir.go`
(`if h := os.Getenv("HOME"); h != "" {...}; return os.UserHomeDir()`), `npm/src/homedir.js`
(`if (fromEnv) return fromEnv; return os.homedir()`) e `pypi/trackfw/homedir.py` (guard
`sys.platform == "win32"`, e mesmo lá só substitui se `from_env` for truthy). Zero regressão nesse
caso — é exatamente o comportamento pré-PR.

## 2. Direção simétrica — a validação fica estrita demais e recusa ambiente legítimo?

**Não pelo mecanismo óbvio** (ausência de `$HOME` continua caindo no fallback nativo, confirmado
acima) — CI, contêiner, Windows sem `$HOME` setado: comportamento idêntico ao pré-PR.

**Mas há uma variante inversa do mesmo problema, que é o achado central desta revisão: em vez de
"recusar ambiente legítimo", o risco real é "aceitar silenciosamente um `$HOME` que não é o
`%USERPROFILE%` real, produzindo um estado autoconsistente e falso-positivo-de-saúde".** Não é a
validação recusando algo legítimo — é a validação e o `update harness` concordando entre si sobre um
caminho errado, e reportando saudável.

### O mecanismo concreto

`UpdateHarness` (o **escritor** — grava `~/.claude/settings.json`, `~/.trackfw/scripts/*`) e
`validateGuardGlobalHookResolvable`/`validateGuardGlobalScriptIntegrity` (o **leitor/auditor**) agora
usam a **mesma** `homedir.Dir()`/`homedir()`/`home_dir()`. Isso os mantém consistentes *entre si*
— confirmei isso lendo `internal/generators/update.go` (`UpdateHarness`, linha ~489),
`internal/generators/agentfiles.go` (`globalCredentialGuardScriptPath`,
`globalGitBranchGuardScriptPath`, `readGlobalHookJSON`) e `internal/validator/validator_git_branch_guard.go`
— todos chamam `homedir.Dir()`. **Mas nenhum dos dois lê o mesmo mecanismo que o CLI de agente real
usa** para localizar `~/.claude/settings.json`/`~/.codex/config.toml`/etc. Claude Code, Codex e
Gemini CLI são binários de terceiro, não código deste repositório — eles continuam resolvendo home
pelo mecanismo nativo deles (para um app Node.js não-patcheado, isso é `os.homedir()` puro, que no
Windows prioriza `USERPROFILE`, confirmado na seção 0). O trackfw, depois deste PR, passa a preferir
`$HOME` quando presente.

**Se `$HOME` e `%USERPROFILE%` apontarem para diretórios diferentes no mesmo ambiente Windows**
(cenário plausível, não exótico — variáveis desse tipo são setadas por Git Bash/MSYS2, integração
WSL, `direnv`-like tooling, ou por convenção corporativa que redireciona home para um drive de rede,
sem qualquer ator hostil envolvido), o resultado é:

1. `trackfw update harness` escreve o hook global e o script de guard sob `$HOME/.claude/settings.json`
   e `$HOME/.trackfw/scripts/...`.
2. O Claude Code real (ou Codex/Gemini) nunca olha para lá — ele resolve home pelo mecanismo dele
   próprio, que prioriza `%USERPROFILE%`. O hook **nunca é carregado pelo agente real**.
3. `trackfw validate` roda depois, no mesmo shell (mesmo `$HOME`), encontra o hook exatamente onde
   `update harness` o deixou, com conteúdo íntegro (`validateGuardGlobalScriptIntegrity` bate
   template) e resolvível (`validateGuardGlobalHookResolvable` acha o caminho, existe, é executável).
   **Zero warning.** `trackfw validate` reporta o guard global como saudável.
4. Resultado líquido: o guard está **genuinamente instalado em disco, íntegro, e 100% inerte** — o
   agente real nunca o invoca — e a própria ferramenta que deveria detectar isso (`trackfw validate`)
   não detecta, porque ela e o escritor concordam entre si sobre o caminho errado.

Isto é **a mesma classe de achado** que registrei em 2026-08-18
(`docs/seguranca/2026-08-17-revisao-do-guard-em-escopo-global.md`, seção B: "existe um mecanismo que
ativamente remove a proteção de projeto com base numa entrada global que nunca dispara") — lá a causa
era um campo `"type"` ausente no JSON do hook; aqui a causa seria um diretório-base divergente do que
o consumidor real usa. Mecanismo diferente, mesma forma de dano: **"pior do que nunca ter cabeado"**,
porque cria falso sinal de saúde em vez de silêncio neutro.

### Isto é fix de teste travestido de fix de produção, ou o item 2 é genuinamente de produção?

Verifiquei lendo `gh issue view 216` (fonte primária do reporter, não só a análise de `hefesto-tf`).
**É genuinamente de produção, não só isolação de teste** — a narrativa de "os testes não isolam"
está lá, mas não é o item inteiro:

- O reporter enumera **21 sites em `internal/`, 25 em `npm/src`, 28 em `pypi/trackfw`** que chamam
  `os.UserHomeDir()`/`os.homedir()`/`os.path.expanduser` — todos código de produção, nenhum arquivo
  de teste — e diz textualmente: "É bem mais barato que os 97 [call sites de teste], que conflitariam
  em todo merge futuro". Ou seja, o autor deliberadamente escolheu corrigir a origem (produção) em
  vez do sintoma (isolação de teste) porque é o fix mais barato, não porque o defeito seja só de
  teste.
- O cabeçalho da própria issue diz que o ambiente de reprodução é **"Windows 11 (Git Bash, ...)"** —
  o reporter mede e corrige a partir do mesmo terminal (Git Bash) que é o cenário de divergência
  `$HOME`/`%USERPROFILE%` que esta seção levanta. Isso não resolve a pergunta da divergência (não há,
  na issue, medição do valor literal de `$HOME` fora do teste, nem comparação com o que o Claude
  Code/Codex real leria) — mas confirma que o ambiente onde a correção nasceu **é** o ambiente onde a
  pergunta desta revisão importa, não um cenário hipotético.
- **Uma alternativa mais estreita existia e o PR não a escolheu**: corrigir só os fixtures de teste
  (fazer `t.Setenv`/`monkeypatch`/`process.env.HOME =` também setar `USERPROFILE`/`HOMEPATH` no
  Windows) resolveria a mesma dor de "teste escreve na home real" sem mover a âncora de confiança de
  nenhum código de produção — inclusive sem tocar `validateGuardGlobalHookResolvable`/`UpdateHarness`
  nem introduzir a superfície da seção 2. O PR não faz isso porque o reporter também quer que
  **usuários reais** do Windows com `$HOME` setado (o próprio caso dele, Git Bash) tenham `trackfw`
  respeitando essa variável como qualquer outra ferramenta Unix-like — objetivo de produto legítimo,
  não só de teste. **Registro isto para a Wave 2 avaliar como opção, não como recomendação minha**:
  dado que o objetivo de produto é real, a alternativa estreita provavelmente não deveria substituir
  o Grupo A inteiro — mas o fato de existir mostra que o raio da correção (produção, não só teste) foi
  uma escolha deliberada do autor, e portanto a divergência da seção 2 é um custo aceito de propósito,
  não um acidente de implementação — o que reforça, não enfraquece, a necessidade de medir e nomear o
  residual explicitamente em vez de tratá-lo como thread lateral.

### Cenário concreto de instalação fantasma (sem precisar de máquina Windows nova para visualizar)

Não preciso de um ambiente novo para tornar isto tangível — o mesmo runner Windows que já existe
(`ROADMAP-2026-08-30-job-de-windows-largo...`) reproduz em duas chamadas:

```
1. cmd.exe (sem $HOME)         > trackfw update harness
   -> escreve em %USERPROFILE%\.claude\settings.json  (instalação real, o Claude Code lê daqui)

2. Git Bash (com $HOME setado) > trackfw update harness
   -> escreve em $HOME\.claude\settings.json           (instalação fantasma, se $HOME != %USERPROFILE%)

3. Git Bash (mesmo $HOME)      > trackfw validate
   -> audita SÓ a instalação fantasma (item 2) — reporta saudável
   -> nunca olha para a instalação real (item 1), que pode estar desatualizada ou nem existir
```

Nenhum dos três passos exige um atacante — só um desenvolvedor real rodando `trackfw` a partir de
dois terminais diferentes na mesma máquina, o que é comportamento normal, não anômalo, para quem usa
Git Bash para `git` e às vezes PowerShell/cmd.exe para outra coisa.

### O que não consegui fechar sozinho, e por quê

Não tenho runner Windows disponível neste ambiente (macOS) para medir o valor literal de `$HOME` sob
Git Bash/MSYS2/WSL-interop/CI Windows, nem se ele aponta para o mesmo diretório físico que
`%USERPROFILE%` nesses ambientes, nem se um caminho no estilo `/c/Users/nome` (formato POSIX que o
Git Bash tradicionalmente usa) chega a um binário nativo Go/Node/Python já traduzido para forma
Windows ou como string literal `/c/Users/nome` (que as APIs de arquivo do Go/Node/Python tratam como
caminho relativo à raiz da drive atual, não como o perfil do usuário — resultando em diretório
**inexistente**, não em diretório errado-mas-existente). A busca externa que fiz (ver Sources) indica
que a tradução de caminho do MSYS2 se aplica com mais confiança a **argumentos de linha de comando**
do que a **variáveis de ambiente arbitrárias herdadas por um processo filho**, e as fontes se
contradizem sobre o comportamento exato dependendo da instalação (Git for Windows vs MSYS2 puro, e se
`HOME` foi deixado no default ou setado manualmente para `%USERPROFILE%`). **Não vou inferir essa
semântica** — é exatamente o tipo de afirmação que este projeto já decidiu, em revisões anteriores
(`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, Q3), não fazer sem medir.

Duas sub-hipóteses ficam em aberto, com efeitos de segurança opostos, e só medição no runner Windows
real (`ROADMAP-2026-08-30-job-de-windows-largo...`, já existe na wip) resolve qual vale:

- **Se `$HOME` sob Git Bash resolver para o mesmo diretório físico que `%USERPROFILE%`** (forma
  Windows válida, ex. `C:/Users/nome`, que APIs de arquivo aceitam com `/`): o Grupo A é estritamente
  benigno nesse ambiente — mesmo resultado de antes, só por variável diferente. **Veredito (a) nesse
  sub-caso.**
- **Se `$HOME` sob Git Bash (ou WSL-interop, ou CI) resolver para um diretório físico diferente de
  `%USERPROFILE%`, ou para um caminho POSIX não traduzido que aponta para um diretório inexistente**:
  o cenário da seção acima se aplica — falso-saudável silencioso em (o caso mais provável, dado que o
  diretório resultante nem existiria) ou proteção fantasma em (o caso onde o caminho POSIX resolve,
  por acaso, para algum diretório real diferente do perfil). **Veredito (b)/(c) conforme o subcaso.**

## 3. Falsificação nas duas direções (o que o próprio PR testa, e o que ele não testa)

**O que o PR #222 falsifica, verificado lendo os testes no diff:**
- `scripts/check-homedir-parity.sh`: roda os três binários reais com `HOME` apontado para um tempdir
  e compara a saída — prova que os três runtimes concordam entre si sob a mesma variável, e que o Go
  (que já estava "certo" por acidente no POSIX) não regride.
- Testes Go/Node/Python de `validateGuardGlobalHookResolvable`/`validateGuardGlobalScriptIntegrity`
  (fixtures com `HOME=t.TempDir()`) — provam que a função **encontra corretamente** um guard instalado
  sob o `$HOME` de teste.

**O que nenhum teste do PR falsifica, e é exatamente a lacuna da seção 2:** nenhum teste cobre
`$HOME != %USERPROFILE%` no mesmo processo, nem o caso onde o escritor (`UpdateHarness`) e um
consumidor externo não-patcheado (simulando o CLI de agente real) resolveriam home para diretórios
diferentes. Isso é esperado — é uma pergunta sobre **interação com um binário de terceiro fora do
repositório**, não algo que um teste unitário do trackfw consiga expressar sem simular o
comportamento nativo do Node.js/Windows do lado de fora do helper `homedir()` — mas por isso mesmo
precisa virar critério de aceite explícito da Wave 2, não ficar implícito.

### Nota de paridade — assimetria Python (guard `win32`-only) vs Go/Node (incondicional)

`pypi/trackfw/homedir.py` só prefere `$HOME` quando `sys.platform == "win32"`; `internal/homedir/homedir.go`
e `npm/src/homedir.js` preferem `$HOME` incondicionalmente, em qualquer SO. Confirmado (seção 0) que
isso é **comportamentalmente um no-op** — no POSIX, `os.UserHomeDir()`/`os.homedir()` já preferem
`$HOME` antes de chegar no fallback nativo, então "preferir incondicionalmente" e "preferir só no
Windows" produzem o mesmo resultado em Linux/macOS. Não é um defeito. Registro só para quem for rodar
`scripts/check-homedir-parity.sh` ou revisar o diff não confundir a forma assimétrica do código com
divergência de comportamento — são shapes de código diferentes por necessidade documentada no
docstring do próprio PR (evitar quebrar os três testes que fazem `monkeypatch.setattr("os.path.expanduser", ...)`),
não por descuido.

## 4. Residual declarado

- **Aceito, não é regressão:** um processo pai que já controla o ambiente do `trackfw` (CI de
  terceiro, container compartilhado, agente com escrita irrestrita) continua podendo redirecionar a
  resolução de home — antes via `USERPROFILE`, depois via `$HOME` também. Mesmo modelo de ameaça já
  aceito por `ADR-2026-08-12` e pela minha revisão de 2026-08-18.
- **Aceito, no-op confirmado:** no POSIX, o Grupo A não muda nada — `$HOME` já era a única fonte,
  antes e depois, nos três runtimes.
- **Não fechado por este parecer, nomeado para a Wave 2:** se `$HOME` e `%USERPROFILE%` podem
  divergir fisicamente num ambiente Windows legítimo e comum (Git Bash é o caso mais provável, dado
  que é o terminal onde a maioria dos usuários deste projeto roda ferramentas de linha de comando em
  Windows), e se essa divergência faz `trackfw` escrever/auditar um guard global que o CLI de agente
  real nunca carrega — sem nenhum warning, porque leitor e escritor concordam entre si sobre o
  caminho errado. Não medi porque não tenho runner Windows disponível aqui; é medível e barato de
  medir no job de Windows que este próprio roadmap referencia como já existente.

## 5. O que a Wave 2 deve incorporar (concreto, para `apolo-tf`)

1. **Medir antes de portar**, no runner Windows real (mesmo mecanismo do
   `ROADMAP-2026-08-30-job-de-windows-largo...`): valor literal de `$HOME` vs `%USERPROFILE%` sob (i)
   `cmd.exe`/PowerShell puro (esperado: `$HOME` ausente, sem mudança de comportamento — só confirmar),
   (ii) Git Bash (o caso que mais importa — é o terminal mais comum para quem usa `trackfw` em
   Windows, e é o ambiente do próprio reporter da issue #216), (iii) se disponível, um CI Windows
   genérico (`windows-latest` do GitHub Actions ou equivalente). Registrar se os dois caminhos
   coincidem fisicamente. **Não parar aí — medir também o consumidor real**: instalar um Claude Code
   (ou Codex/Gemini CLI) de verdade nesse mesmo runner e confirmar, por observação direta (ex.:
   `Get-Item`/`ls -la` no caminho resultante, ou um hook de teste que o próprio agente dispare),
   **onde ele de fato lê `settings.json`** a partir de cada um dos dois terminais — a divergência só
   é um achado de segurança se o consumidor real também divergir; comparar só as duas variáveis de
   ambiente entre si não prova isso, só é o primeiro passo.
2. **Critério de aceite explícito e testável, não implícito**: se os valores divergirem em algum dos
   ambientes medidos, `homedir.Dir()`/`homedir()`/`home_dir()` (ou o call-site específico de
   `UpdateHarness`/`validateGuardGlobalHookResolvable`) deve, no mínimo, emitir um finding
   informativo (não bloqueante — mesmo padrão fail-open já usado no resto do arquivo) quando `$HOME`
   e o resultado do mecanismo nativo (`os.UserHomeDir()`/`os.homedir()`/`expanduser` chamado sem o
   guard) **ambos resolvem e diferem** — transformando o split-brain silencioso em sinal visível, sem
   quebrar o fix do item 2 da issue (que continua precisando de `$HOME`-first para a isolação de
   teste funcionar). Custo de implementação baixo: os dois valores já estão disponíveis com uma
   segunda chamada ao mecanismo nativo puro.
3. **Não portar a alegação de "API nativa → env var" para a REQ/roadmap da Wave 2 sem correção** — é
   imprecisa (seção 0) e, se citada como a justificativa de segurança do ML, obscurece a pergunta que
   de fato importa (divergência `$HOME`/`%USERPROFILE%`, não "existência de controle via env var",
   que já existia).
4. Se a medição do item 1 confirmar que Git Bash / os ambientes comuns deste projeto sempre
   resolvem `$HOME` para o mesmo diretório físico que `%USERPROFILE%`, o item 2 pode ser rebaixado a
   nota de documentação (`docs/cli-parity.md`) em vez de mudança de código — mas essa é uma decisão
   que depende do dado medido, não deve ser assumida antes de medir.
5. **Opção alternativa a avaliar, não recomendação fechada**: existe um fix mais estreito — só corrigir
   a isolação dos fixtures de teste (setar `USERPROFILE`/`HOMEPATH` também, ao lado de `HOME`, nos
   ~97 pontos de teste que fazem `t.Setenv("HOME", ...)`/equivalente) — que resolveria a dor original
   ("`go test ./...` escreve na home real") sem mover a âncora de confiança de nenhum código de
   produção, e portanto sem abrir a superfície da seção 2. **Não é a recomendação desta revisão** —
   o próprio reporter da issue #216 documenta que escolheu deliberadamente o fix de produção porque
   também quer que usuários reais com `$HOME` setado no Windows tenham o comportamento respeitado
   (objetivo de produto, não só de teste) — mas a Wave 2 deve pesar essa opção explicitamente contra
   o resultado da medição do item 1, em vez de portar o Grupo A inteiro por padrão só porque o PR
   já vem pronto.

Sources:
- [Node.js os.homedir() Windows/POSIX behavior — GeeksforGeeks](https://www.geeksforgeeks.org/node-js/node-js-os-homedir-method/)
- [Node.js OS documentation (os.homedir())](https://nodejs.org/api/os.html)
- [MSYS2 — Filesystem Paths (path conversion behavior for native executables)](https://www.msys2.org/docs/filesystem-paths/)
- [Git Bash on Windows: The MSYS Path Conversion Pitfall](https://www.todzhang.com/blogs/tech/en/minikube-windows-path-pitfall)
- [Setting up Git Bash / MINGW / MSYS2 on Windows — HOME default vs %USERPROFILE%](https://www.pascallandau.com/blog/setting-up-git-bash-mingw-msys2-on-windows/)
