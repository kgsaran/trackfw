# Barreira de segurança — vazamento de stack no CLI Node (ML-2A)

> Data: 2026-08-16 | Autor: Hades (Security) | Branch:
> `fix/handler-global-de-erro-nos-entrypoints-node-e-python` (sem commits deste agente)
> Escopo do ML-1A auditado: `npm/bin/trackfw`, `npm/src/lib/fatal-error.js`,
> `pypi/trackfw/fatal_error.py`, `pypi/trackfw/cli.py`

## Resumo executivo

O fix do ML-1A (handler global no entrypoint Node e no `args.func()` do Python) está **correto e
verificado empiricamente** no caminho para o qual foi desenhado: mensagem íntegra, exit code
preservado, `TRACKFW_DEBUG=1` restaura a stack sem regressão. **Libero o merge do ML-1A/ML-2A** —
mas encontrei, varrendo `trackfw serve` como o brief pediu, um achado **não relacionado ao ML-1A,
mais sério que o vazamento que motivou este REQ, e que exijo destacar antes de qualquer decisão de
release**: `trackfw serve` do Go e do Python escutam em **todas as interfaces de rede** (`0.0.0.0`),
não só em loopback — diferente do Node, que restringe corretamente a `127.0.0.1`. Confirmei por
execução real (bind + `curl` a partir do IP da LAN da máquina, resposta HTTP 200) nos dois CLIs. Ver
item 1-bis.

Encontrei também um gap residual real (não um exploit, um caminho dormente) no Python: a cobertura
do `fatal_error.py` não é simétrica com a do Node — `cli.py` só protege `args.func(args)`, não a
construção do parser nem o `register(subparsers)` de cada comando. Verifiquei isso executando (não
lendo), corrompendo uma cópia isolada do pacote.

Nenhum outro caminho testado (saída `--json` de `validate`, hooks gerados, `sync`/`thirdparty`)
vaza a mesma classe de informação de runtime/caminho/versão.

## 1. Varredura de outros caminhos — 3 CLIs

### Node
- `npm/src/commands/serve.js`: servidor HTTP local (`127.0.0.1` apenas — não escuta em `0.0.0.0`).
  Toda resposta de erro (`catch (_) { ... 500 ... }`) descarta o erro real e devolve texto genérico
  `Internal Server Error` — **não vaza stack nem caminho**. Erro de bind de porta
  (`server.on('error', ...)`) imprime só `err.message` (ex.: "Porta X já está em uso"), sem stack.
  Confirmado por leitura, linhas 39-173.
- `npm/src/commands/validate.js --json`: serializa apenas `{message, rule, file}` derivados de
  strings de violação já formatadas pelo validador — nunca um objeto `Error`/stack. Nenhum
  `JSON.stringify` em nenhum comando do repo serializa um objeto de erro (`grep` confirmou zero
  ocorrências combinando `JSON.stringify` com `err`/`stack`/`catch`).
- `npm/src/commands/sync.js`, `thirdparty.js` (caminhos de rede: API Linear/Jira, URLs
  third-party): todo `catch` usa `e.message`, nunca `e.stack`; erros não capturados propagam para o
  `.catch()` global do `bin/trackfw`, que já corta a stack. Testado empiricamente (ver §5).
- Hooks gerados pelo produto (`scripts/trackfw-*.sh`, templates em `npm/src/generators/hooks.js`):
  nenhum tem `set -x` nem `trap ... DEBUG`/ERR que ecoe comandos com caminho absoluto; os únicos
  `echo ... >&2` são mensagens estáticas com prefixo `trackfw-<hook>:` (`credential-guard.sh:128,132`,
  `git-branch-guard.sh:122`) — sem stack, sem `$0`/`BASH_SOURCE` no output.
- `.trackfw-attention.json` (`npm/src/serve/api_attention.js`): schema fixo
  (`roadmap/ml/message/level/timestamp`) escrito pelos próprios agentes, não por um handler de
  erro — fora da classe de vazamento em questão; o conteúdo de `message` é o que o agente decidir
  escrever (mesma superfície de qualquer texto livre gerado por LLM, não um vazamento de runtime).
- Primitivas adicionais de vazamento sugeridas pelo advisor, varridas em todo `npm/src`:
  `grep -rn "console.error(err)\|console.error(e)\b"` → zero ocorrências (nenhum `console.error`
  bruto de um objeto `Error`, que imprimiria a stack via `util.inspect`); `grep -rn "\.stack"
  npm/src` → única ocorrência é dentro do próprio `npm/src/lib/fatal-error.js`, sob
  `TRACKFW_DEBUG=1`. Confirma que não há um segundo caminho de impressão de stack no CLI Node.

### Python
- `pypi/trackfw/cli.py`: **gap residual confirmado por execução** (não é leitura de código) — ver
  item 5. `args.func(args)` é o único trecho dentro do `try/except Exception`; `parser.parse_args()`
  e os `N` chamadas `<cmd>.register(subparsers)` que rodam antes dele ficam **fora** do handler.
  Hoje nenhum `register()` real lança exceção em uso normal (são apenas `add_parser` +
  `set_defaults`), então não há caminho de entrada do usuário que dispare isso hoje — mas o
  comentário do próprio `fatal_error.py`/`cli.py` ("a per-entrypoint handler closes every future gap
  at once") **não é verdade para este caminho**, e é fácil um `register()` futuro ganhar lógica que
  falhe (ex.: leitura de config para decidir flags condicionais) sem ninguém perceber que caiu fora
  da rede de segurança.
- `pypi/trackfw/__main__.py` chama `main()` sem wrapper próprio — e o `console_script` real
  (`pypi/pyproject.toml:[project.scripts] trackfw = "trackfw.cli:main"`, o que `pip install` de fato
  registra) usa o mesmo `main()`, então o gap acima é o caminho de produção real, não um artefato
  de teste.
- Superfícies de rede/JSON Python (`sync`, `--json` equivalentes): mesma revisão de `e.message` vs
  `traceback` já coberta pelo trabalho do ML-1A (REQ registra explicitamente que o Python não
  vazava nos caminhos testados); não encontrei caminho novo.
- `grep -rn "format_exc\|print_exc" pypi/trackfw` fora de `fatal_error.py` → zero ocorrências.
  Confirma que não há um segundo caminho de traceback no CLI Python.
- **`pypi/trackfw/commands/serve.py:104` — vazamento ativo de caminho absoluto, achado novo, fora
  do escopo original do REQ.** `_serve_static_file()` faz `except OSError as e:
  self.send_error(500, f"Cannot read file: {e}")` — a representação em string de `OSError` em
  Python inclui o caminho do arquivo (ex.: `[Errno 2] No such file or directory:
  '/Users/.../site-packages/trackfw/serve/static/<arquivo>'`), e essa string vai **direto para o
  corpo da resposta HTTP 500**. O `_serve_static_file` restringe o alvo a `STATIC_DIR` (bloqueio de
  path traversal com `os.path.basename` + `realpath` — testei mentalmente contra `../../etc/passwd`
  e o `startswith(static_real + os.sep)` barra corretamente), então o que vaza é o **caminho de
  instalação do pacote Python** (path do `site-packages`/venv), não um arquivo arbitrário do
  usuário — mesma classe de dado que motivou o REQ original (item de menor valor sozinho, mas ver
  item 1-bis: neste caso é alcançável pela rede, não só localmente).

### Go
Ver item 1-bis e item 3. Os handlers HTTP do pacote realmente usado
(`internal/serve/api_file.go`, `api_board.go`, `serve.go`) usam só mensagens genéricas
(`"internal error"`, `"cannot read roadmap dir"`) — **não vazam** `err.Error()`. Achado colateral:
existe um `internal/server/server.go` com `http.Error(w, fmt.Sprintf("template error: %v", err),
...)` na linha 373 que ecoaria um erro de execução de template — mas `grep -rln
"internal/server\""` confirma que **nenhum arquivo do projeto importa esse pacote**; é código morto
não compilado no binário final via nenhum caminho de comando (`internal/commands/serve.go` usa
`internal/serve.Start`, não `internal/server`). Não é um vazamento ativo, mas é código morto que
convém remover em limpeza futura para não confundir a próxima auditoria.

## 1-bis. Achado principal — `trackfw serve` do Go e do Python expõe a rede (achado ativo, não dormente)

**Este é o item mais sério deste relatório e não estava no escopo original do REQ — apareceu ao
cumprir a instrução explícita de varrer `trackfw serve`.**

`grep -n "Listen\|127.0.0.1\|0.0.0.0" internal/serve/serve.go` mostra `addr := fmt.Sprintf(":%d",
port)` seguido de `http.ListenAndServe(addr, mux)` — endereço vazio antes do `:` significa **todas
as interfaces**. `pypi/trackfw/commands/serve.py:150` faz o mesmo com
`HTTPServer(("", port), handler_class)` — string vazia como host é `INADDR_ANY` em
`socketserver`/`http.server`, também todas as interfaces. Só o Node restringe explicitamente:
`npm/src/commands/serve.js:151` — `server.listen(port, '127.0.0.1', ...)`.

**Verificado por execução, não por leitura:**

```
$ /tmp/trackfw-go-test serve --port 18099
trackfw governance server running at http://localhost:18099
$ lsof -nP -iTCP:18099 -sTCP:LISTEN
trackfw-g ... TCP *:18099 (LISTEN)          # "*" = todas as interfaces
$ curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:18099/
200
$ curl -s -o /dev/null -w "%{http_code}" http://192.168.3.137:18099/   # IP da LAN desta máquina
200
```

Repeti o mesmo teste para o Python (`PYTHONPATH=pypi python3 -m trackfw serve --port 18098
--no-open`): `loopback=200`, `lan=200`, idêntico.

**Impacto:** qualquer dispositivo na mesma rede (Wi-Fi doméstico, rede corporativa, rede de
coworking, hotspot compartilhado) consegue, sem autenticação nenhuma, acessar `/api/board`,
`/api/chain`, `/api/metrics`, `/api/file` e ler o conteúdo de ADRs, REQs e roadmaps do repositório
— que podem conter contexto de negócio, decisões de arquitetura e (dependendo do que o time
documenta ali) informação sensível do projeto. Isso é **exposição de dados de governança do
projeto para a rede local**, não um vazamento de stack — categoria mais grave do que a que motivou
este REQ.

**Isso combina com o achado do item 1 (Python `send_error` com `OSError`)**: o mesmo servidor que
está exposto à rede tem, no Python, um caminho de erro que ecoa `str(OSError)` (inclui caminho
absoluto do arquivo) na resposta HTTP — tornando esse vazamento de caminho **remotamente
observável por qualquer um na LAN**, não só localmente.

**Relação com o merge deste ML:** este bug é **pré-existente e não foi introduzido nem tocado pelo
ML-1A/ML-2A** — nenhum arquivo de `internal/serve`, `npm/src/commands/serve.js` ou
`pypi/trackfw/commands/serve.py` está no diff desta branch. **Não bloqueia o merge do handler
global de erro.** Mas é grave o suficiente para exigir um REQ de correção próprio, com prioridade
pelo menos igual à do REQ que originou este ML — recomendo `serve.go`/`serve.py` passarem a bindar
`127.0.0.1`/`localhost` por padrão (com uma flag explícita tipo `--host 0.0.0.0` para quem
realmente precisa expor, opt-in e documentado), espelhando o que o Node já faz corretamente.

## 2. `TRACKFW_DEBUG` como superfície de ativação por terceiro

**Veredito: não é uma superfície de ataque nova, com uma ressalva a documentar.**

- `grep -rn "TRACKFW_DEBUG"` em todo o repo (`.yml`, `.yaml`, `.sh`, geradores de hooks Node/Python)
  não encontrou nenhum lugar onde o próprio produto exporta ou sugere exportar essa variável em
  templates de CI, hooks gerados (`.claude/`, `.codex/`, `scripts/trackfw-*.sh`) ou config
  versionada. A variável só é lida, nunca escrita, pelo código do produto.
- Para um terceiro ativá-la contra a vítima, ele precisaria escrever no ambiente dela — perfil de
  shell, variável de CI compartilhado, ou um wrapper/hook que ela executa. **Nesse ponto o atacante
  já tem a capacidade de rodar código arbitrário no ambiente da vítima**, o que é um problema
  estritamente mais grave do que fazer o `trackfw` imprimir uma stack — a stack deixaria de ser o
  vetor relevante.
- Ressalva a documentar (não bloqueante): se o `trackfw` algum dia ganhar um comando que **herda**
  env de um arquivo de config versionado no repositório do usuário (ex.: um `.env` lido
  automaticamente) em vez de só do processo do shell, isso mudaria o cálculo — um `TRACKFW_DEBUG=1`
  num `.env` commitado por engano voltaria a vazar para qualquer um que rodasse o CLI ali. Hoje isso
  não existe (`config.Load()`/`config.load()` não leem `.env`), mas é o tipo de regressão que este
  handler inteiro existe para evitar — vale um teste de não-regressão se essa feature for proposta.

## 3. Go — `panic` alcançável por entrada do usuário

**Veredito: não encontrei caminho de panic alcançável por entrada do usuário no estado atual.**

Não há nenhum `recover()` em `internal/` ou `cmd/` — se algum `panic` ocorrer em qualquer caminho,
o runtime do Go despeja goroutine trace e caminhos de arquivo `.go`, sem nenhuma rede de segurança.
Confirmei `grep -n "trimpath\|ldflags" Makefile` → **nenhuma ocorrência**: o `go build` do projeto
(`Makefile:7`, `go build -o $(BUILD_DIR)/$(BINARY) ./cmd/trackfw`) não usa `-trimpath`. **Correção
importante em relação à minha primeira leitura:** isso NÃO é "igual ao `unhandledRejection`" do
Node. Um panic do Go embute, em tempo de **compilação**, os caminhos absolutos de onde o binário
foi construído — ou seja, a máquina/CI de quem publicou o release (hoje, a máquina do KG ou o
runner de CI), não o ambiente da vítima que está rodando o binário já compilado. É um vazamento de
informação real (caminho de build, útil para reconhecimento sobre a infraestrutura de release), mas
de uma classe e de uma vítima diferentes do bug original — o REQ original vazava dados do ambiente
de quem RODA o CLI (usuário, home, versão de runtime local); um panic do Go vazaria dados de quem
CONSTRÓI o CLI. Severidade menor porque não expõe nada específico do usuário final.

`grep -rn "panic(" internal cmd` (excluindo testes) retornou 5 ocorrências, todas defensivas contra
**dessincronia interna de dados estáticos do binário**, não entrada do usuário:

- `internal/identity/preset.go:216-230` — roda em `func init()` do pacote, no boot do processo,
  comparando dois literais Go embutidos no binário (`presetOrder` vs `presets`). Não há entrada do
  usuário no caminho — dispara igual para todo mundo se algum dia os literais dessincronizarem.
- `internal/commands/identity_wizard.go:208,249` — comparam `identity.KnownAgentIDs()` (lista Go
  estática) contra `catalog.Item(...)`, onde `catalog` vem de `integrations.LoadCatalog()`
  (`internal/integrations/catalog.go:16` — `//go:embed assets`, `catalog.json` embutido no binário
  em tempo de compilação, não lido de disco/rede em runtime). Confirmado por leitura do
  `go:embed`: não há forma de um usuário ou config de projeto influenciar esse catálogo em runtime.

**Conclusão:** os 5 `panic()` existentes são invariantes de build, não vulnerabilidades — se um
disparar em produção é porque o binário foi construído com dados internos inconsistentes, e o
comportamento (mesmo stack) seria idêntico para qualquer usuário, não haveria vazamento
diferencial de segredo específico do ambiente da vítima.

**Achado colateral, fora do escopo de panic mas da mesma classe de risco:** não há `recover()`
nenhum no projeto. Não fiz varredura exaustiva de todo parser de arquivo do usuário (frontmatter de
roadmap/REQ, `trackfw.yaml`) atrás de index-out-of-range/nil-deref alcançável por um arquivo
malformado — os pontos que inspecionei (`extractFrontmatterField` em
`internal/validator/validator.go:1541-1560`) fazem bounds-check antes de indexar
(`strings.HasPrefix` antes de `content[3:]`) e não panicaram nos casos testados. Não é uma
varredura completa de todo `internal/` — recomendo, como follow-up e **não como bloqueio deste
ML**, um `recover()` no nível do `main()`/`Execute()` do cobra que espelhe o handler Node/Python,
fechando a classe inteira de uma vez em vez de depender de cada parser individual nunca panicar.

## 4. Severidade — confirmo "baixa a moderada", com uma nuance

**Confirmo a classificação do arquiteto: baixa a moderada, não crítica.** Concordo com a
justificativa (não é execução de código nem escalação de privilégio) e adiciono a razão que a torna
defensável mesmo no pior caso:

- O vetor de maior valor real — versão do runtime Node — é, na prática, **informação que qualquer
  atacante já pode obter de outra forma** contra a maioria dos alvos (banners HTTP, `node
  --version` se tiver shell, `package.json`/lockfile em CI logs). O vazamento economiza um passo de
  reconhecimento, não abre uma porta nova.
- Caminho absoluto de instalação e layout do home **têm valor real** para encadear com outra
  vulnerabilidade (ex.: um path traversal separado que precise saber onde o binário/config vive),
  mas sozinhos não são exploráveis.
- O gatilho exige que o usuário **já esteja rodando o CLI em uma condição de erro não tratada**
  (ex.: `agents update --force` contra um manifesto adulterado) — não é um vazamento passivo, é
  amplificação de um erro que já ia acontecer de qualquer forma.

Não subo para "moderada a alta" porque não há divulgação de segredo (token, credencial) nesse
caminho especificamente — **verificado, não hedge**: `grep -n "token\|Authorization" sync.js` mostra
que o token do Linear vai em `headers: { Authorization: apiKey }` (linha 86) e o do Jira em
`Authorization: Basic <base64(email:token)>` (linhas 130/138) — **sempre em header HTTP, nunca em
URL/query string**. O `fetch`/`https` do Node, quando falha (rede indisponível, DNS, TLS), produz
`TypeError`/`FetchError` cujo `.message` descreve a falha de conexão (ex.: "fetch failed",
"ECONNREFUSED") — não inclui o valor dos headers da requisição que falhou. Logo: **o token não
alcança nenhuma string de erro nesses dois caminhos, com ou sem este fix** — não é só "teoricamente
improvável", é estruturalmente impossível dado onde o token é colocado. Ressalva que mantenho: isso
vale para os dois provedores hoje implementados; qualquer integração futura que monte a URL com o
token embutido (`?token=...`) reabriria esse vetor via `e.message` contendo a URL — vale um teste de
não-regressão se isso for proposto.

Elevo a preocupação, porém, para o achado do item 1-bis: lá o dado que vaza (caminho de instalação
via `OSError`) fica **remotamente acessível pela rede local sem autenticação**, o que é uma mudança
de classe de severidade em relação ao vazamento original (que exigia acesso ao terminal onde o CLI
já roda). Isso é reportado separadamente porque é um bug distinto, não uma correção da nota
"baixa a moderada" acima — essa nota é sobre o caminho corrigido pelo ML-1A especificamente.

## 5. O fix introduziu risco? — verificado por execução

Executei os três cenários abaixo (não apenas li o diff), fora da árvore de trabalho (scratchpad),
sem alterar código de produto:

**a) Mensagem multi-linha e exit code — Node, caminho `unhandledRejection`:**
```
$ node repro.js   # Promise.reject(new Error("linha1\nlinha2 caminho /Users/segredo/instalacao"))
Error: linha1
linha2 caminho /Users/segredo/instalacao
$ echo $?
1
```
Mensagem íntegra byte a byte, exit code 1, sem stack — confirma o comportamento documentado no
ML-1A.

**b) `TRACKFW_DEBUG=1` restaura a stack sem regressão:**
```
$ TRACKFW_DEBUG=1 node repro.js
Error: linha1
linha2 caminho /Users/segredo/instalacao
    at Object.<anonymous> (.../repro.js:4:16)
    ... (stack completa)
```

**c) Caminho já limpo hoje (comando desconhecido) permanece inalterado:**
```
$ ./npm/bin/trackfw comando-inexistente
Error: unknown command "comando-inexistente" for "trackfw"
Run 'trackfw --help' for usage.
$ echo $?
1
```
Confirma que o handler global não interceptou/alterou um erro que já era tratado internamente pelo
commander (nenhuma mensagem duplicada, nenhum exit code diferente de antes do ML-1A).

**d) Python — `report_fatal_error` isolado:**
```
>>> report_fatal_error(ValueError('linha1\nlinha2 caminho /Users/segredo/instalacao'), command='roadmap list')
trackfw roadmap list: linha1
linha2 caminho /Users/segredo/instalacao
```
Mensagem íntegra; o exit code é responsabilidade do `sys.exit(1)` em `cli.py` (fora da função por
design), não regride.

**e) Gap residual do Python confirmado por execução (não bloqueante — ver §1):**
Corrompi `register()` de um comando numa **cópia isolada** do pacote (`/tmp`, nunca no repo) para
lançar antes de `args.func`:
```
$ python3 -m trackfw comando-qualquer
Traceback (most recent call last):
  ...
  File ".../cli.py", line 155, in main
    changelog_cmd.register(subparsers)
  File ".../commands/changelog.py", line 45, in register
    raise RuntimeError('corrupted register: ... /Users/segredo/pypi-copy')
RuntimeError: corrupted register: ...
```
Traceback completo com caminho absoluto vazou — confirma que o handler Python não é uma rede de
segurança total como o Node é (item abaixo).

**f) Assimetria Node vs Python confirmada por execução:** o mesmo tipo de corrupção (lançar
sincronamente antes do ponto onde o parsing normal do usuário começa) foi tentado contra o Node
(`createProgram()` corrompido para lançar antes do `parseAsync()`), numa cópia isolada:
```
$ node bin/trackfw comando-qualquer
Error: corrupted createProgram: caminho /Users/segredo/npm-copy
$ echo $?
1
```
**Não vazou** — porque `installGlobalHandlers()` é chamado ANTES do `require('../src/commands/index')`
em `npm/bin/trackfw:8-10`, então `uncaughtException` cobre até uma falha síncrona no carregamento
do módulo de comandos. O Python não tem esse equivalente global (`sys.excepthook` não é
sobrescrito) — só protege `args.func`. Essa é a lacuna relatada em §1.

**g) Preocupação do advisor sobre `trackfw serve` — processo derruba o servidor inteiro num throw
não tratado? Verificado: sim, mas isso NÃO é regressão do ML-1A — é comportamento idêntico
antes e depois do fix.** Testei com dois builds Node isolados: um com o handler do ML-1A
(`npm-serve-test/`) e um baseline com `installGlobalHandlers()` comentado
(`npm-serve-baseline/`), ambos com o mesmo `throw` sintético injetado em `handleAttention`
(`GET /api/attention`), fora do repo:

```
# BASELINE (sem installGlobalHandlers(), pré-ML-1A):
$ curl http://127.0.0.1:18086/                    → 200 (servidor de pé)
$ curl http://127.0.0.1:18086/api/attention        → conexão falha (processo já morreu)
$ curl http://127.0.0.1:18086/                     → conexão recusada (processo morto)
stderr:
Error: simulated bug baseline sem handler
    at handleAttention (.../api_attention.js:17:9)
    at Server.<anonymous> (.../serve.js:126:7)
    at Server.emit (node:events:514:20)
    ... + "Node.js v26.7.0"

# FIXED (com installGlobalHandlers() do ML-1A):
$ curl http://127.0.0.1:18087/                     → 200 (servidor de pé)
$ curl http://127.0.0.1:18087/api/attention         → conexão falha (processo já morreu)
stderr:
Error: simulated bug in request handler /Users/segredo/instalacao
```

Em **ambos os casos** o processo do servidor morre inteiro ao primeiro throw síncrono não
capturado num handler de request — esse é o comportamento **padrão do Node** para qualquer exceção
não tratada (não é algo que `installGlobalHandlers()` introduziu; é o próprio Node que já mataria o
processo mesmo sem nenhum listener de `uncaughtException`, só que imprimindo a stack antes). A
única diferença observável entre os dois é o conteúdo do stderr: baseline vaza stack completa +
versão do Node; fixed imprime só `Error: <mensagem>`. **Item 5 do advisor respondido com evidência
negativa: não há regressão de disponibilidade introduzida pelo ML-1A.** Acho importante registrar,
à parte, que `trackfw serve` não tem isolamento de falha por request (uma exceção não tratada em
qualquer handler futuro derruba o dashboard inteiro) — isso é um problema de robustez pré-existente,
ortogonal a este REQ, que vale nomear no mesmo follow-up do item 1-bis.

## Veredito final

1. **Item 1 (varredura):** nenhum caminho ativo de vazamento **na classe original do REQ** (stack
   Node) além do já corrigido no ML-1A. Porém a varredura de `trackfw serve` encontrou dois achados
   novos e reais, fora do escopo do REQ original: **(1-bis) Go e Python bindam `trackfw serve` em
   todas as interfaces de rede** (achado ativo, verificado por execução, exposição de dados de
   governança do projeto para a LAN sem autenticação) **e (1) o `OSError` do Python é ecoado na
   resposta HTTP 500**, o que — combinado com o 1-bis — torna esse vazamento de caminho remotamente
   observável. Também confirmei um gap dormente no Python (§1, §5e/f, `cli.py` sem cobertura de
   `parser.parse_args()`/`register()`) — sem exploração possível hoje, registrado como dívida.
2. **Item 2 (`TRACKFW_DEBUG`):** não é superfície nova; ativá-la remotamente já pressupõe
   comprometimento maior.
3. **Item 3 (Go):** sem panic alcançável por entrada do usuário hoje. Corrigido em relação à minha
   leitura inicial: um panic do Go (se algum dia ocorrer) vazaria caminhos de build da máquina de
   quem compilou o release, não do ambiente da vítima — severidade menor que o `unhandledRejection`
   original. Recomendo `recover()` central como follow-up defensivo, não bloqueante.
4. **Item 4 (severidade do ML-1A):** confirmo baixa a moderada para o caminho que o ML-1A corrigiu
   especificamente. O token de integração (Linear/Jira) nunca alcança uma string de erro — vai só
   em header HTTP, verificado no código, não é hedge. **Essa nota de severidade não se estende ao
   achado 1-bis**, que reporto com prioridade própria (ver abaixo).
5. **Item 5 (risco do fix):** nenhum — mensagem íntegra e exit code preservados, verificado por
   execução nos dois CLIs. Também verifiquei, por sugestão do advisor, que o handler global não
   introduz regressão de disponibilidade em `trackfw serve` — o processo já morria com o mesmo
   throw sintético antes do ML-1A (comportamento padrão do Node), só a stack no stderr mudou.

**LIBERO o merge do ML-1A/ML-2A.** Não há bloqueador no diff desta branch.

**Mas registro em destaque, para decisão do KG/arquiteto, o achado 1-bis — fora do escopo desta
branch mas descoberto ao cumprir a instrução de varrer `trackfw serve`:** `trackfw serve` do Go e
do Python expõe o dashboard de governança (ADRs/REQs/roadmaps) para qualquer dispositivo na mesma
rede local, sem autenticação, diferente do Node que já bloqueia corretamente em `127.0.0.1`.
Recomendo abrir um REQ de correção dedicado, com prioridade pelo menos igual à deste REQ (é uma
exposição de dados ativa e remotamente observável, não um vazamento condicionado a erro), para
`serve.go`/`serve.py` passarem a bindar `127.0.0.1` por padrão.

Follow-ups adicionais, não urgentes, não bloqueantes: (a) simetrizar `cli.py` com um
`sys.excepthook` global cobrindo `parser.parse_args()`/`register()`, espelhando o
`uncaughtException` do Node; (b) avaliar `recover()` central no Go como defesa em profundidade
equivalente ao Node/Python; (c) corrigir `pypi/trackfw/commands/serve.py:104` para não ecoar
`str(OSError)` na resposta HTTP (mensagem genérica, como já faz o Go/Node) — parte do mesmo REQ do
item 1-bis; (d) remover o código morto `internal/server/server.go`, não referenciado por nenhum
comando, para não confundir auditorias futuras.

---

## Apêndice — Barreira ML-2A: confirmação de que o achado 1-bis fechou

> Data: 2026-08-16 | Autor: Hades (Security) | Branch:
> `fix/serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao` (sem commits deste
> agente) | Roadmap:
> `docs/roadmaps/wip/ROADMAP-2026-08-16-serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao.md`
> | REQ: `docs/req/REQ-2026-08-16-trackfw-serve-escuta-em-todas-as-interfaces-sem-autenticacao-expondo-a-cadeia-de-governanca-na-rede.md`

Este apêndice cobre o ML-2A: confirmar por conexão real que o achado 1-bis acima (Go/Python
bindando `trackfw serve` em todas as interfaces) fechou nos 3 CLIs, avaliar se o `--host` opt-in
introduzido para corrigi-lo abre um caminho de exposição acidental, e varrer o produto por outro
componente que abra porta. **Tudo abaixo, salvo indicação contrária, foi medido subindo o processo
e conectando nele — não inferido do diff ou do relatório do arquiteto/agente de implementação.**

### 1. Padrão não é alcançável de fora da máquina — medido, não lido

Build usado: `go build -o /tmp/tfw ./cmd/trackfw` (nenhum código de produto alterado). IP de LAN
desta máquina: `192.168.3.137` (`ifconfig en0`).

**Rodada 1 — bind padrão, sem `--host`, portas 47001/47002/47003:**

```
$ lsof -nP -iTCP:47001 -sTCP:LISTEN   # Go
tfw  ... TCP 127.0.0.1:47001 (LISTEN)
$ lsof -nP -iTCP:47002 -sTCP:LISTEN   # Node
node ... TCP 127.0.0.1:47002 (LISTEN)
$ lsof -nP -iTCP:47003 -sTCP:LISTEN   # Python
Python ... TCP 127.0.0.1:47003 (LISTEN)

curl localhost   -> go 200 / node 200 / py 200
curl LAN (192.168.3.137) -> go 000 / node 000 / py 000
```

`lsof` é a asserção primária (mostra o endereço em que o socket está de fato ligado,
independentemente do caminho de rede); `curl` na LAN corrobora, mas **não é a única evidência** —
ver a ressalva sobre um falso-negativo transitório no item 1c abaixo, que é exatamente por que não
confio em `curl` sozinho.

**Rodada 2 — família IPv6, bind padrão, Go, porta 47031:** o padrão é um socket `AF_INET`
(`net.Listen("tcp", ...)` com host IPv4 `127.0.0.1`), então a inalcançabilidade por IPv6 é
esperada por construção — testei mesmo assim, para não deixar a alegação "inalcançável de fora"
restrita a IPv4:

```
$ lsof -nP -iTCP:47031 -sTCP:LISTEN
tfw ... TCP 127.0.0.1:47031 (LISTEN)          # nenhum listener IPv6
$ curl http://[fdc0:83c9:2141:7700:1c93:3ea9:a8cf:2618]:47031/   # endereço IPv6 global da própria máquina
-> 000
```

**Rodada 3 — controle positivo, `--host 0.0.0.0`, portas 47011/47012/47013:** prova que o harness
de medição *detecta* exposição quando ela existe — sem isso, um `000` na rodada 1 não distinguiria
"bind correto" de "meu curl/rede não está funcionando".

```
$ lsof -nP -iTCP:47011  # Go
tfw  ... TCP *:47011 (LISTEN)
$ lsof -nP -iTCP:47012  # Node
node ... TCP *:47012 (LISTEN)
$ lsof -nP -iTCP:47013  # Python
Python ... TCP *:47013 (LISTEN)

curl LAN (192.168.3.137) -> go 200 / node 200 (após reteste, ver 1c) / py 200
```

**1c — falso-negativo transitório encontrado e não escondido.** Na primeira rodada de exposição, o
processo Node especificamente devolveu `curl` `000` e `nc` sem resposta na LAN, **enquanto o
`lsof` já mostrava `*:47012 (LISTEN)`** — ou seja, um `curl` isolado teria me feito concluir (
errado) que o Node continuava seguro por padrão mesmo com `--host 0.0.0.0`. Descartei essa
hipótese porque (a) o mesmo processo respondia 200 via `127.0.0.1` no mesmo instante — não é o
processo travado — e (b) matar e subir um Node **novo** na mesma exposição (`--host 0.0.0.0`,
porta 47022) respondeu 200 tanto via loopback quanto via LAN de primeira. Não encontrei causa raiz
determinística (suspeito de firewall de aplicação do macOS negociando permissão de forma
assíncrona para um processo Node recém-lançado, mas não provei isso com certeza) — registro como
uma flakiness observada uma vez, não reproduzida de forma confiável em 3 tentativas subsequentes
com processos novos, e que **não muda o veredito**: quando reproduzível, o resultado foi sempre
"exposto quando `--host` não-loopback, bloqueado quando padrão". O ponto prático: **não confiar em
`curl` sozinho para provar ausência de exposição** — `lsof` (endereço do socket) é a fonte de
verdade; `curl` é corroboração, sujeita a ruído de rede/firewall que pode mascarar um positivo.

**Gate independente, rodado por mim:** `GO_BIN=/tmp/tfw scripts/check-serve-address-parity.sh` —
10/10 `OK`, cobrindo bind padrão, `--host ::1` e a URL/aviso de exposição com `--host 0.0.0.0` nos
3 CLIs, medido por execução real dentro do próprio script (não é reuso do relatório do ML-1C).

**Conclusão do item 1:** confirmado por conexão real — o padrão não é alcançável de fora da
máquina nos 3 CLIs, para os endereços testados (LAN IPv4, IPv6 global e link-local desta máquina).
AC1 e AC6 fecham.

### 2. `--host` cria caminho de exposição acidental?

**Varredura do próprio repositório por uso não-loopback de `--host` ou `0.0.0.0`:**
`grep -rn "\-\-host"` fora de `scripts/check-serve-address-parity.sh`,
`scripts/check-gates-falsify.sh`, `docs/agents-working-context.md`, o roadmap/REQ desta mudança e
os testes dos 3 CLIs (`npm/tests/serve_address.test.js`, `pypi/tests/test_serve_address.py`,
`internal/serve/serve_test.go`) — **nenhuma ocorrência**. Não há Makefile, Dockerfile, CI, doc de
onboarding ou template de artefato gerado (`discover --init`/`update`) com `--host 0.0.0.0` (ou
qualquer valor não-loopback) pronto para copy-paste. **Verificado, não inferido**: o `--host`
também não tem caminho de config/env por trás — `grep` por `TRACKFW_HOST`, `Getenv`,
`process.env`, `os.environ` em `internal/serve`, `internal/commands/serve.go`,
`npm/src/commands/serve.js`, `pypi/trackfw/commands/serve.py`, `internal/config`,
`pypi/trackfw/config.py`, `npm/src/lib/config.js`, e por `host` em `trackfw.yaml`/
`docs/cli-parity.md` — **zero ocorrências**. Isso importa porque o vetor perigoso de exposição
acidental não é o flag digitado interativamente (visível em qualquer diff), é um valor
`serve.host: 0.0.0.0` num `trackfw.yaml` versionado ou uma `TRACKFW_HOST` honrada silenciosamente
por CI/Docker — **nenhum dos dois existe hoje**. A única forma de expor é escrever `--host` com um
valor não-loopback, explicitamente, em algum lugar visível em `grep`/diff.

**O aviso em stderr é suficiente como mitigação?** Não sozinho, e digo isso sem enfraquecer o
veredito de aprovação — é uma nuance sobre o valor da mitigação, não um bloqueador, porque a REQ já
declara autenticação fora de escopo (ver Notas do roadmap) e o aviso nunca foi proposto como
controle técnico, só como sinalização. Concretamente: **stderr não protege nenhum dos cenários que
o brief pede para avaliar.** Um `--host 0.0.0.0` num alvo de `Makefile`, num `CMD`/`ENTRYPOINT` de
Dockerfile, num step de CI, ou num script chamado de forma não-interativa tem o stderr redirecionado
para um log que ninguém lê em tempo real — o aviso dispara, mas não há humano ali para vê-lo antes
do dano. O aviso é útil apenas para quem digita a flag manualmente num terminal interativo, que é
justamente o caso onde a pessoa **já sabe** que está expondo (o `--host` não é acidental nesse
caso, é intencional). Ou seja: o aviso mitiga bem o "typo mental" de quem esqueceu o que a flag faz,
mas não mitiga em nada o cenário de maior probabilidade real — alguém copiar `--host 0.0.0.0` para
um script versionado. **Isso não é motivo de bloqueio** porque (a) não há hoje nenhum caminho no
próprio repositório com esse valor (item acima), e (b) autenticação — o controle que resolveria
isso de verdade — está corretamente declarada fora de escopo desta REQ. É risco residual a
nomear, não a suavizar.

### 3. Outro componente do produto abre porta?

Varredura dos 3 stacks por primitivas de servidor (`net.Listen`/`ListenAndServe`/`http.Serve` no
Go; `.listen(`/`createServer` no Node; `HTTPServer`/`socketserver`/`socket.bind` no Python), fora
de arquivos de teste:

- **Go:** só `internal/serve/serve.go` (o `serve` real, já coberto acima) e
  `internal/server/server.go:401` — pacote **morto**: `grep -rln
  '"github.com/kgsaran/trackfw/internal/server"'` em todo o projeto retorna zero importadores, e
  `go tool nm /tmp/tfw` (binário recém-compilado) confirma por prova de não-vacuidade —
  `internal/serve\.` aparece 52 vezes no binário, `internal/server\.` aparece **0** vezes. Esse
  pacote morto ainda tem `addr := fmt.Sprintf(":%d", port)` — o exato padrão da regressão original
  desta REQ — mas não está linkado no binário publicado; não é um vazamento ativo, é uma armadilha
  para quem reativar o pacote sem saber que ele nunca foi corrigido. Reitero a recomendação já
  registrada no corpo principal deste documento: remover `internal/server/server.go` em limpeza
  futura, para que a próxima varredura não precise reprovar isso de novo.
- **Node:** único `http.createServer`/`.listen(` do produto é `npm/src/commands/serve.js`, já
  coberto. Nenhuma outra ocorrência fora de testes.
- **Python:** único `HTTPServer` do produto é `pypi/trackfw/commands/serve.py`, já coberto.
  Nenhuma outra ocorrência fora de testes.
- **Scripts (`scripts/`):** nenhum `nc -l`, `python -m http.server` ou servidor solto — as únicas
  ocorrências de "listen"/"bind" em `scripts/` são dentro de `check-serve-address-parity.sh` e
  `check-gates-falsify.sh`, que **invocam** o `serve` real para testá-lo, não abrem porta própria.
- **Hooks e geradores** (`internal/generators`, `npm/src/generators`, `pypi/trackfw/generators` e
  os hooks materializados por `discover --init`/`update` em `.claude/`, `.codex/`, etc.): nenhuma
  ocorrência de `listen`/`createServer`/`HTTPServer`. Os hooks são scripts de guarda
  (credential-guard, git-branch-guard) que escrevem só em stderr/exit code, não abrem socket.
- **`site/`** (documentação estática, VitePress): as únicas ocorrências de `createServer`/`listen`
  estão dentro de `site/.vitepress/dist/assets/chunks/*.js` — bundle de terceiro (framework
  VitePress), não código do projeto, e é o *client-side* framework bundle, não um servidor.

**Conclusão do item 3:** nenhum componente não óbvio do produto abre porta. O único achado é o
código morto Go, já reportado no corpo principal deste documento e não novo.

### Observação de definition-of-done, fora do meu escopo de correção

O roadmap marca **ML-1B como `✅ Concluído`**, mas as 6 caixas de critério de aceite do ML-1B (do
`--host ::1` até "evidência de `lsof`/`curl` colada") continuam **`- [ ]`** no arquivo — só as do
ML-1A estão marcadas. Meu ML-2A não depende disso (medi tudo de novo, de forma independente), mas
registro para o arquiteto fechar antes de mover o roadmap para `done/`: por §Definition of Done, a
pasta/status não fecha com critérios de aceite pendentes no próprio arquivo, mesmo que o trabalho
esteja de fato feito. Não editei o roadmap — não é meu escopo.

### Veredito

**APROVO.** Os 3 CLIs bindam em loopback por padrão, medido por `lsof` (assertiva primária) e
corroborado por `curl`, para IPv4 LAN e para os dois endereços IPv6 desta máquina. `--host` é
opt-in explícito, funciona, e o repositório não contém hoje nenhum caminho — flag, config ou env —
que exponha por acidente. O gate de paridade `check-serve-address-parity.sh`, rodado por mim de
forma independente, fecha 10/10.

**Risco residual aceito, explicitado:**
1. **O aviso de exposição não protege uso não-interativo** (Makefile/Dockerfile/CI) — só quem lê
   stderr em tempo real se beneficia dele. Aceito porque hoje não há nenhuma ocorrência desse
   padrão no repositório, e porque o controle que resolveria isso de fato (autenticação) está
   corretamente fora de escopo desta REQ, não por omissão, mas por decisão declarada.
2. **Código morto `internal/server/server.go`** ainda contém o padrão de bind exatamente igual à
   regressão original, sem estar linkado no binário — não é explorável hoje, mas é uma armadilha
   para reativação futura sem revisão de segurança. Recomendo remoção, não bloqueio.
3. **Flakiness observada uma vez** no bind exposto do Node via `curl`/`nc` (LAN), não reproduzida
   em processos novos, com `lsof` mostrando o bind correto o tempo todo — registrado para não
   esconder um resultado anômalo, não é evidência contra o veredito.

Nenhum dos três é motivo de bloqueio: os dois primeiros são recomendações de limpeza/hardening
fora do escopo declarado da REQ, o terceiro foi investigado e não reproduzido de forma que
contradiga a medição principal.

---

## Apêndice — Revisão de delta (`9314ae2..HEAD`), pós-abertura de PR

Escopo: apenas o delta entre a árvore revisada na Barreira ML-2A (`9314ae2`) e `HEAD`
(`b4697b8`), conforme solicitado. Não revalidei do zero a medição de bind/exposição dos 3 CLIs —
reproduzi as partes que o delta poderia ter afetado.

### A) Remoção de `internal/server/server.go` — limpa e completa

Confirmado por conta própria, não pela palavra de quem propôs a remoção:

- `git grep -rn "internal/server"` fora de `internal/serve` e do próprio parecer de segurança:
  zero ocorrências. Nenhuma referência órfã no código-fonte.
- `go build ./...` e `go vet ./...`: limpos, sem erros nem warnings.
- `go tool nm` no binário Go recompilado: `0` símbolos com prefixo `internal/server.` (era o
  próprio ponto que eu tinha usado como evidência de "código morto, não linkado" na Barreira
  ML-2A — a remoção elimina até a possibilidade de reativação futura sem revisão, que era minha
  recomendação nº 2).
- Nenhum arquivo de teste órfão (`*server_test*`, diretório `internal/server/`): busca vazia.
- `go test ./internal/serve/...`: verde.

A recomendação nº 2 do meu veredito original ("Recomendo remoção, não bloqueio") foi endereçada
integralmente. A superfície de ataque só diminuiu — a remoção é estritamente subtrativa, sem
código vivo levado junto.

### B) Prosa de segurança nova em `docs/cli-parity.md` — verificada linha a linha contra o código, não aceita por afirmação

**Ausência de autenticação:** confirmado. `grep -rni "authorization\|authenticate\|apikey\|bearer"`
em `internal/serve`, `npm/src/serve`, `pypi/trackfw/serve`: zero ocorrências nos 3.

**Aviso só em stderr:** confirmado nos 3 runtimes, lido no código-fonte real, não assumido:
- Go: `internal/serve/serve.go:125` — `fmt.Fprintln(os.Stderr, ExposureWarning(host, port))`.
- Node: `npm/src/commands/serve.js:196` — `console.error(exposureWarning(host, port))`
  (`console.error` vai para stderr no Node).
- Python: `pypi/trackfw/commands/serve.py:216` — `print(_exposure_warning(host, port), file=sys.stderr)`.

A caracterização "o aviso não protege uso não-interativo (Makefile/Dockerfile/CI)" é honesta e
não suavizada — é a mesma ressalva que eu próprio já tinha registrado como risco residual nº 1
na Barreira ML-2A, agora formalizada no contrato ao invés de ficar só no meu parecer.

**Exceções intencionais — avaliação item a item:**
- `::ffff:127.0.0.1` / `127.0.0.2` classificados como exceção porque "nenhum dos 3 consegue
  escutar nesses endereços, logo o impacto de segurança é nulo": aceitável — divergência de
  *mensagem de erro* sem superfície de exposição associada não é uma exceção de segurança
  disfarçada, é uma exceção de parceria de UX legítima. Não reexecutei o bind nesses endereços
  (fora do escopo do delta — comportamento herdado de `9314ae2`, não alterado aqui).
- Loopback dual-stack (`127.0.0.1` + `::1` simultâneos) declarado "não é objetivo": correto e é a
  postura mais conservadora, não uma concessão — um listener dual-stack ampliaria a superfície
  em vez de reduzi-la, e o documento é explícito que isso não é uma limitação aceita por
  conveniência, é a ausência do próprio defeito original (wildcard).
- Porta padrão divergente (`4080` Go vs `8080` Node/Python) e prefixo de linha de URL divergente:
  nenhum dos dois tem implicação de segurança — são convenções de apresentação pré-existentes ao
  REQ, corretamente fora de escopo. Concordo que não misturar a correção de segurança com uma
  mudança de interface não relacionada foi a decisão certa.

Nenhuma das exceções listadas esconde risco. A lista é defensável.

**Afirmação de autodenúncia do gate quando falta `lsof`:** verificada contra o código-fonte, não
aceita por afirmação. Em `scripts/check-serve-address-parity.sh` (linhas 74-81, 165-171), quando
`lsof` está ausente o braço de exclusão de wildcard é de fato pulado, degradando para um simples
`connect` TCP em `127.0.0.1:$port` — que também teria sucesso contra um bind wildcard (`0.0.0.0`
aceita conexões em `127.0.0.1`), então esse braço sozinho não provaria mais nada sobre o defeito
original. Em `scripts/check-gates-falsify.sh`, o Cenário 59 (linhas 5166-5189) exercita
exatamente essa lacuna: o braço de detecção corrompe `pypi/trackfw/commands/serve.py` para
reintroduzir o bind wildcard e usa `assert_fails_with "expected lsof to show 127.0.0.1:"` — uma
asserção que exige textualmente a mensagem de falha que só existe no braço `lsof`. Sem `lsof`, o
script degradado nunca emite essa string (o bind wildcard também aceita `127.0.0.1`, então o
`connect` de fallback passa), `assert_fails_with` falha por não encontrar o texto esperado, e o
Cenário 59 do `check-gates-falsify.sh` reporta falha — não passa em silêncio. A afirmação do
documento procede: é uma garantia real, não uma garantia falsa escrita num documento de contrato.

### Verificações adicionais feitas antes de fechar o veredito

- **Comentário repontado não esconde duplicata:** o heading antigo citado nos comentários
  pré-delta, `### Aviso ao usuário — string pinada` (linha 2114 de `docs/cli-parity.md`), continua
  existindo — mas é uma seção **diferente e não relacionada**, sobre o aviso de artefato
  desatualizado pulado em `update --install-missing`, não sobre o aviso de exposição do `serve`.
  Não é uma duplicata stale do mesmo contrato; o repoint estava correto e não deixou nada órfão.
- **A alegação central da seção nova — "prova por escuta real, nunca por leitura de fonte" —
  reexecutada, não só lida.** A leitura do script (feita acima) prova o *mecanismo*; só rodar o
  gate prova que ele continua passando **nesta árvore**, já que a última execução real fora da
  Barreira ML-2A foi contra `9314ae2`, antes da remoção de `internal/server/` e do reponte de
  comentários em `internal/serve/serve.go`. Reexecutado: `GO_BIN=<binário recompilado em HEAD>
  bash scripts/check-serve-address-parity.sh` → **10/10 OK**, todas as 4 sub-checagens
  (default-bind loopback, `--host ::1`, aviso de exposição byte-idêntico, URL impressa) com
  `lsof` disponível e usado (medição real de socket, não checagem de string em stdout). `go test
  ./internal/serve/...` sozinho não seria suficiente aqui — prova compilação/unidade, não bind
  real; o gate é o artefato que a prosa nova descreve, e é ele que precisava rodar contra `b4697b8`.

### Veredito do delta

**APROVADO.** A remoção de `internal/server/` é limpa, completa e estritamente subtrativa — a
recomendação nº 2 do parecer original está endereçada. Os 4 comentários repontados são só texto
e apontam para uma seção real, não para uma duplicata stale. A prosa nova em `docs/cli-parity.md`
foi verificada contra o código real (ausência de auth, destino stderr, mecanismo de autodenúncia
do gate) e o gate que ela descreve foi **reexecutado nesta árvore** (`b4697b8`), 10/10 — não
aceita por afirmação de quem a escreveu, nem só por leitura do script. Nenhuma redação exige
ajuste. Não reexecutei o bind em `::ffff:127.0.0.1`/`127.0.0.2` (fora do escopo do delta —
comportamento herdado de `9314ae2`, não alterado aqui).
