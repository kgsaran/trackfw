# Barreira de Segurança — porte das correções do reporter da issue #216 (PRs #222–#225)

> Produzido por: `hades-tf` | Data: 2026-09-01
> REQ: `docs/req/REQ-2026-08-31-portar-as-correcoes-dos-prs-222-225-do-reporter-da-issue-216-defeitos-1-2-3-5-e-6-de-windows.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-31-portar-as-correcoes-do-reporter-da-issue-216.md`
> Escopo: diff completo da branch `fix/portar-as-correcoes-do-reporter-da-issue-216` (PR #229) contra `origin/main`. Barreira final do roadmap.

---

**VEREDITO: APROVA.**

Nenhum achado bloqueante. Nenhum achado de acompanhamento novo de segurança — os dois já nomeados
pelo próprio roadmap (instalação fantasma na Wave 0; falta de detecção de divergência) continuam
corretamente fora deste diff e corretamente destinados a REQ própria. Risco residual reafirmado
abaixo, não retrabalhado.

---

## Metodologia

Li o diff completo (`git diff origin/main...HEAD`, 78 arquivos, +2670/-182) — **linha a linha em
todos os arquivos de produto** (Go, Node.js, Python, scripts, `Makefile`, `docs/cli-parity.md`); os
únicos arquivos que não abri byte a byte foram os artefatos de governança em prosa
(`docs/req/`, o próprio roadmap, `docs/analises/`, `docs/agents-working-context.md`) e o
`docs/roadmaps/.trackfw-log`, que não carregam lógica. Não me apoiei no relato do roadmap para
nenhum dos achados centrais:

- **Ponto 5 (bit de execução) foi verificado por execução, não só por leitura.** Rodei
  `go test ./internal/validator/... -run 'TestCredentialGuardHookResolvable|TestGitBranchGuard'`
  (30 testes, todos verdes, incluindo `TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao`),
  `node --test` sobre `npm/tests/validator.test.js` e
  `npm/tests/git_branch_guard_hook_integrity.test.js` (99 verdes), e
  `pytest -k test_windows_nao_dispara_pelo_bit_de_execucao` sobre `test_validator.py` e
  `test_git_branch_guard_validator.py` (2 verdes) — as duas direções, nos três runtimes, não só o
  grep dos números de linha. Depois **reverti o guard de plataforma manualmente**
  (`sed` trocando `case CurrentGOOS != "windows" && info.Mode()&0111 == 0:` por
  `case info.Mode()&0111 == 0:`) e confirmei que
  `TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao` **falha** sem o guard —
  restaurando o arquivo original em seguida (`git diff --stat` limpo depois). Isso fecha a lição do
  próprio histórico desta sessão (`[[feedback_execute_all_named_vectors_before_verdict]]`,
  vault): não bastava ler os números de linha citados pelo roadmap, era preciso rodar cada vetor
  citado.
- **Confirmei por grep que `CurrentGOOS`/`_platform`/`_current_platform` nunca são atribuídos fora
  de código de teste** nos três runtimes — o único ponto de mutação em código de produto é a linha
  de seed (`= runtime.GOOS` / `= process.platform` / `= sys.platform`); os únicos outros
  atribuidores são as funções `_setPlatformForTest`/`_set_platform_for_test`, chamadas apenas por
  arquivos `*.test.js`/`test_*.py`. Nenhum caminho de entrada governada pelo usuário alcança essa
  variável.
- **Reli `pypi/tests/__init__.py` e confirmei, via `git ls-tree origin/main`, que o arquivo já
  existia (vazio, blob `e69de29b`) antes deste diff** — `pypi/tests/` já era um pacote real, não um
  namespace package implícito; este diff só adiciona conteúdo ao arquivo que já existia como
  marcador. `pytest pypi/tests --collect-only` na branch coleta 1566 testes sem erro de coleção.
- **Reli `pypi/trackfw/commands/update_harness.py` inteiro** (o lado escritor do par
  escritor/auditor que a Wave 0 modelou) — as 43 linhas são port mecânico de `newline="\n"` mais a
  única troca de `os.path.expanduser("~")` por `home_dir()` em `_run()`; nenhum caminho de escrita
  novo, nenhuma mudança de permissão/modo no `Path.write_text`.
- **Reli `pypi/trackfw/validator.py` inteiro** (as 60 linhas do diff, não só os dois `elif` do
  Ponto 5) — ver achado adicional na seção "Restante do diff" abaixo: o próprio docstring do port
  revela que a divergência cross-runtime **já existia antes desta REQ**, e nas duas direções
  diferentes por runtime (Ponto 5 explica).
- Amostrei os 25 arquivos restantes de call site (`internal/config/config.go`,
  `internal/generators/*.go`, `internal/commands/*.go`, `npm/src/config/index.js`,
  `npm/src/generators/hooks.js`, `npm/src/commands/*.js`, `npm/src/integrations/*.js`,
  `pypi/trackfw/commands/*.py`, `pypi/trackfw/generators/hooks.py`,
  `pypi/trackfw/integrations/*.py`) lendo o diff completo de cada um, não só o `--stat`: confirmei
  que são 100% troca mecânica de `os.UserHomeDir()`/`os.homedir()`/`os.path.expanduser("~")` pelo
  helper, sem mudança de lógica, mais dois achados de reforço reportados abaixo
  (`stdin_is_interactive()` no portão de terceiro, alias `_user_home_dir` para evitar sombra de
  parâmetro).

Priorizei os quatro pontos que o handoff pediu de propósito (superfície de FFI nova,
`errors="replace"`, os três gates novos, e a detecção de instalação fantasma já auditada na
Wave 0), depois o resto do diff, na ordem acima.

---

## Ponto 1 — `pypi/trackfw/tty.py`: `GetConsoleMode` via `ctypes`, superfície de FFI nova

`pypi/trackfw/tty.py:27-47` (`_windows_is_console`):

```python
try:
    import ctypes
    import msvcrt
    from ctypes import wintypes
    handle = msvcrt.get_osfhandle(stream.fileno())
    mode = wintypes.DWORD()
    return bool(
        ctypes.windll.kernel32.GetConsoleMode(
            wintypes.HANDLE(handle), ctypes.byref(mode)
        )
    )
except Exception:  # noqa: BLE001 — sem handle utilizável não há console
    return False
```

**Falha para `False` em todo caminho, verificado, não assumido:**

- `msvcrt.get_osfhandle(stream.fileno())` levanta se `stream` não tiver `fileno()` (stream
  substituído por objeto de teste/captura de pipeline) — capturado pelo `except Exception` que
  envolve a função inteira.
- Handle inválido (`INVALID_HANDLE_VALUE`, `-1`): `GetConsoleMode` retorna `0` (falha) sem levantar
  exceção Python — o `bool(...)` do retorno já cobre isso, sem depender do `except`.
- Falha de carregamento da DLL (`ctypes.windll.kernel32` inexistente — cenário de Wine, contêiner
  Linux rodando um binário empacotado incorretamente, ambiente sem `kernel32.dll`): `ctypes.windll`
  só existe no atributo do módulo em builds Windows do CPython; num ambiente não-Windows a própria
  chamada é inalcançável, porque `_windows_is_console` só é invocado a partir de `_is_interactive`
  (`tty.py:56-58`) **atrás de `if sys.platform == "win32":`** — não há caminho onde este código roda
  fora do Windows. Dentro do Windows, se `kernel32.dll` não puder ser resolvido, `ctypes.windll`
  levanta `AttributeError`/`OSError` no acesso ao atributo, capturado pelo `except Exception` global
  da função.
- Rodar sob Wine: `GetConsoleMode` é reimplementado pelo Wine para consoles reais e falha (retorna
  `0`) para handles que não são console — mesmo contrato do Windows nativo, sem exceção não tratada
  esperada.

**Confirmei o consumidor:** `stdin_is_interactive()`/`stdout_is_interactive()` (`tty.py:61-68`) são
usados para decidir se o `init` entra no wizard interativo. `False` nesse predicado significa **não
promptar** — a direção seguramente conservadora. Um `init` que deixa de entrar no wizard num
Windows/Wine/contêiner de borda é uma degradação de UX (o usuário precisa passar flags em vez de
usar o wizard); um `init` que entrasse no wizard sob stdin não interativo é o que já causava o bug
original (`EOF when reading a line`, morte do processo). **Concordo com a direção do handoff: fail
`False` é a escolha segura**, e o código a implementa em todos os três caminhos de falha
identificados, não só no `except` óbvio.

**Achado: nenhum.** Superfície de FFI nova, mas de leitura (`GetConsoleMode` não escreve estado, não
aceita entrada externa — o único argumento variável é o handle do próprio processo, nunca dado
controlável por terceiro), com fail-safe verificado em três frentes independentes, e espelhando
deliberadamente a mesma API que o Go já usa em produção (`charmbracelet/x/term`) em vez de inventar
heurística paralela — reduz divergência de borda, não aumenta.

---

## Ponto 2 — `_force_utf8_output()`: `errors="replace"` pode mascarar mensagem de segurança?

`pypi/trackfw/cli.py:47-76`, chamada em `main()` (`:80`), antes de qualquer parsing de argumento:

```python
reconfigure(encoding="utf-8", errors="replace", newline="\n")
```

**`errors="replace"` aqui não mascara nada, porque a direção do erro que ele trata nunca ocorre na
prática.** `reconfigure()` está configurando os fluxos de **saída** (`stdout`/`stderr`) para
**escrever** UTF-8. O caminho de erro que `errors=` controla é `str.encode('utf-8', errors=...)` —
e UTF-8 consegue representar **qualquer** ponto de código Unicode válido sem erro. A única forma de
uma escrita para um stream UTF-8 falhar por `UnicodeEncodeError` é a `str` conter um **surrogate
solto** (`\ud800`–`\udfff` sem par, produzido por decodificação `surrogateescape` de bytes
inválidos vindos de outro lugar) — cenário que não existe no caminho de mensagens de recusa de guard
ou de violação de `validate`: essas mensagens são construídas por `f"..."`/`.format()` sobre strings
literais e caminhos de arquivo já decodificados pelo próprio interpretador, nunca por
`surrogateescape` de entrada binária arbitrária. **Não há mensagem de segurança que dependa de
`errors="replace"` para aparecer nem para ser suprimida** — na prática este `errors=` está
protegendo um caminho que a codificação de destino (UTF-8) torna quase inatingível, não abrindo uma
via de mascaramento.

**Reconfigurar globalmente afeta subprocessos ou código que já capturou o stream? Não, nos dois
casos, verificado:**

- **Subprocessos:** `sys.stdout`/`sys.stderr` reconfigurados são objetos Python (`io.TextIOWrapper`)
  do processo `trackfw` em si. Um subprocesso lançado com `subprocess.run`/`Popen` sem
  `stdout=subprocess.PIPE` herda o **descritor de arquivo do SO** por duplicação, não o objeto
  Python — escreve direto no mesmo FD, sem passar pelo wrapper reconfigurado. `reconfigure()` não
  altera o FD subjacente nem a codificação do console do SO, só a camada de texto do objeto Python.
  Nenhum efeito sobre saída de subprocesso.
- **Referência já capturada:** `io.TextIOWrapper.reconfigure()` muta o objeto **in-place** — não
  cria nem atribui um objeto novo a `sys.stdout`. Qualquer código que tenha feito `self._out =
  sys.stdout` em tempo de import, antes de `_force_utf8_output()` rodar em `main()` (`:80`, primeira
  linha da função), continua com a mesma referência de objeto e portanto herda a reconfiguração —
  não há divergência entre "quem pegou a referência antes" e "depois".

**Achado: nenhum.**

---

## Ponto 3 — Os três gates novos ampliam superfície ao rodar?

`scripts/check-python-writes-lf.sh`, `scripts/check-tty-detection.sh`: puramente estáticos
(`grep`/`find`/parsing Python de código-fonte próprio) ou, no caso do `tty-detection`, chamam
`python3 -c` e `node -e` com literais de código fixos do próprio script — nenhum input externo,
nenhuma leitura de variável de ambiente do chamador além de `</dev/null` como stdin. Sem achado.

`scripts/check-homedir-parity.sh` é o único que **executa os três binários reais do CLI** com `HOME`
redirecionado para um `mktemp -d` (`:14-41`):

```bash
FAKE="$(mktemp -d)"
...
out=$(cd "$FAKE" && HOME="$expected" "$ROOT/bin/trackfw" adr list --scope global 2>&1 || true)
```

- **Escopo de execução:** roda dentro de `make quality`/`make parity`, no ambiente de quem já tem
  permissão de escrita no repositório (dev local ou CI do próprio repo) — não é caminho alcançável
  por PR de fork sem merge prévio nem por entrada externa; os únicos "argumentos" são literais do
  próprio script.
- **`cd "$FAKE"` + `HOME="$expected"`:** os três CLIs são invocados com `cwd` e `$HOME` redirecionados
  para o mesmo diretório temporário isolado, criado e destruído pelo `trap 'rm -rf "$FAKE"' EXIT`
  (`:16`) no início do script — nenhuma escrita fora dele é esperada, e o subcomando escolhido
  (`adr list --scope global`) é de leitura.
- **Não é uma execução privilegiada nem amplia o que o script já podia fazer:** quem roda
  `make quality` já executa dezenas de outros gates que chamam os três binários reais
  (`check-artifact-parity.sh`, `check-barrier.sh`, etc.) — este não introduz uma classe de execução
  nova, só mais uma invocação dentro da mesma superfície de confiança que `make parity` já assume
  (repositório confiável, executado por quem já tem controle da máquina).

**Achado: nenhum.**

---

## Ponto 4 — o `homedir` nos 3 runtimes bate com o que a Wave 0 liberou, e a detecção de divergência continua ausente

Reverifiquei, não aceitei por relato do roadmap:

- **Zero call site cru fora do helper**, confirmado por grep independente nos três runtimes:
  `os.UserHomeDir()` fora de `internal/homedir/homedir.go` só aparece em
  `scripts/windows-repro/go/checks.go` (instrumento de medição da REQ anterior, deliberadamente cru
  para medir o comportamento de produção sem mock — não é código de produto); `os.homedir()` fora de
  `npm/src/homedir.js`: zero ocorrências; `os.path.expanduser`/`Path.home()` fora de
  `pypi/trackfw/homedir.py`: zero ocorrências.
- **Nenhuma detecção de divergência entre `$HOME` e o mecanismo do agente real (`%USERPROFILE%`)
  foi implementada** — confirmado por leitura de `internal/homedir/homedir.go`, `npm/src/homedir.js`
  e `pypi/trackfw/homedir.py` inteiros: os três só resolvem a home, nenhum compara contra outra
  fonte nem emite aviso. Correto: implementar essa detecção aqui seria *feature*, violaria a regra
  de porte fiel que governa a REQ, e a Wave 0 já decidiu explicitamente que ela vira REQ própria.
- **A alegação banida ("API nativa do SO → env var") não aparece** em nenhum arquivo do diff — grep
  por `API nativa`/`native API` nos arquivos novos: zero ocorrências.
- O vetor real identificado na Wave 0 — **instalação fantasma**: `homedir.Dir()` consistente entre
  escritor (`UpdateHarness`) e auditor (`validateGuardGlobalHookResolvable`), mas potencialmente
  divergente do agente de terceiro real sob Git Bash com `$HOME`≠`%USERPROFILE%` — **permanece sem
  mitigação neste diff**, como o roadmap já declarou. Não é um achado novo: é o mesmo risco
  residual, reafirmado, não escondido.

**Achado: nenhum novo.** Risco residual reafirmado (ver seção própria abaixo).

---

## Ponto 5 — bit de execução NTFS: o guard de segurança realmente enfraquece no Windows?

`internal/validator/validator_credential_guard.go` e `validator_git_branch_guard.go`, e os
equivalentes Node (`npm/src/validator/index.js:1607`, `:2713`) e Python
(`pypi/trackfw/validator.py:1983`, `:3184`), mudam:

```go
case info.Mode()&0111 == 0:
```
para
```go
case CurrentGOOS != "windows" && info.Mode()&0111 == 0:
```

Isto **suprime** a detecção de "script do guard existe mas não é executável" no Windows, nos três
runtimes — à primeira vista, exatamente o tipo de enfraquecimento de controle que devo recusar.
Verifiquei se é isso:

- **O bit POSIX de execução não é representável em NTFS.** `os.Stat().Mode()` do Go, `fs.statSync()`
  do Node e `os.access(X_OK)` do Python relatam **sempre** ausência do bit `0111` em arquivo regular
  no Windows, **inclusive imediatamente após `os.Chmod(path, 0o755)`** — a checagem antiga era
  `True` (dispara) em **todo** arquivo, em **toda** máquina Windows, **independente** de o guard
  estar de fato instalado, funcional e sendo invocado. Não é um discriminante ali: é sempre-falso
  travestido de achado.
- **O escopo do guard é exato, não "todo Windows".** `CurrentGOOS` vem de `runtime.GOOS` (Go),
  seedado no processo — reflete o SO de **compilação/execução do binário**, não do host físico. Um
  binário Linux rodando em WSL (que usa ext4 via VHD, onde o bit de execução **é** representável e
  **é** preservado por `chmod`) reporta `runtime.GOOS == "linux"` e a checagem **continua ativa**.
  O guard só desarma quando o binário é genuinamente Windows-nativo, exatamente onde o bit é
  inobservável. Mesmo raciocínio nos outros dois runtimes (`process.platform`/`sys.platform`).
- **Nenhuma outra checagem de resolvabilidade é afetada.** Só o ramo `case ...Mode()&0111 == 0` é
  tocado — o ramo "script não existe" (`does not exist`) continua ativo em todos os SOs, nos três
  runtimes, sem alteração. Um script de guard ausente ou apagado continua sendo detectado no
  Windows; só a checagem de permissão (que nunca discrimina nada ali) foi desligada.
- **Falsificado nas duas direções, nos três runtimes, e eu executei os testes em vez de confiar no
  relato:**

  | verificação | Go | Node | Python |
  |---|---|---|---|
  | dispara quando não-executável, plataforma ≠ windows | `TestCredentialGuardHookResolvable_DisparaScriptNaoExecutavel` (`CurrentGOOS="linux"`) | equivalente em `validator.test.js` | equivalente em `test_validator.py` |
  | **não** dispara quando não-executável, plataforma == windows | `internal/validator/validator_credential_guard_test.go:918` (`CurrentGOOS="windows"`) e `validator_git_branch_guard_test.go:851` | `npm/tests/validator.test.js:415` (`_setPlatformForTest('win32')`) | `pypi/tests/test_validator.py:1086`, `test_git_branch_guard_validator.py:115` (`_set_platform_for_test('win32')`) |

  Rodei `go test ./internal/validator/... -run TestCredentialGuardHookResolvable` e
  `TestGitBranchGuard.*NaoExecutavel` na branch: verde, e o teste "não dispara" falha se eu reverto
  manualmente o guard de plataforma (verificado por falsificação local, não só lido).
- **`scripts/check-gates-falsify.sh` Cenário 81** (o gate que prova que o Go regredir nesta checagem
  quebra a paridade cross-CLI) foi **retargetado**, não removido, para continuar mirando o
  substring correto depois do guard de plataforma ser prefixado — confirmei o `sed` atualizado
  (`case info\.Mode()&0111 == 0:` → `info\.Mode()&0111 == 0:`) e a nota de vault que documenta o
  porquê (mesmo precedente do Cenário 179, `scaffold_doctor.go`).

**Achado: nenhum.** Isto não é enfraquecimento de controle — é remoção de uma checagem que nunca foi
um discriminante no Windows, preservando a checagem de existência (a que de fato importa contra
script apagado/nunca instalado), com parity de teste nas duas direções e nos três runtimes,
**verificado por execução e por reversão manual do guard** (ver Metodologia). Já era o mesmo padrão
aplicado em `scaffold_doctor.go` (`REQ-2026-08-28`, revisado então).

**Achado de contexto que fortalece o veredito, não o enfraquece:** o docstring novo de
`pypi/trackfw/validator.py:9-19` revela que a divergência cross-runtime **já existia antes desta
REQ**, e nas **duas direções opostas por runtime** — algo que eu não teria descoberto sem ler o
arquivo inteiro (não estava nas duas linhas que o roadmap cita):

- **Go/Node (`stat.Mode()`/`fs.statSync().mode`):** sempre reportavam bit ausente no Windows →
  a checagem antiga **sempre disparava** "não executável" ali, mesmo em script recém-regenerado.
  Ruído (falso positivo), não silêncio.
- **Python (`os.access(path, os.X_OK)`):** o próprio docstring documenta que essa chamada
  **sempre retorna `True`** para qualquer arquivo existente no Windows — a checagem antiga
  **nunca disparava** ali. Silêncio (falso negativo), a direção oposta de Go/Node.

Ou seja: antes deste port, o Python já estava **inerte** para esta checagem no Windows — igual ao
estado pós-fix, só que sem dizer isso em lugar nenhum e sem paridade documentada com os outros dois
runtimes (que, ao contrário, alarmavam sempre). O port não introduz uma capacidade nova de "passar
despercebido no Windows" no Python: só torna esse comportamento pré-existente **explícito, intencional
e igual aos outros dois runtimes** (que passaram a se calar também), em vez de dependente de um
efeito colateral de API não documentado no próprio código.

---

## Achado adicional (positivo) — `stdin_is_interactive()` também é o portão de confiança do `thirdparty install`

Lendo `pypi/trackfw/integrations/command.py` e `pypi/trackfw/commands/thirdparty.py` inteiros (não
só os call sites de home), o consumidor de `tty.py` não é só o wizard de `init` — é também o
`resolve_scope()` (`command.py:126-131`) e o portão de confiança de
`trackfw thirdparty install` (`thirdparty.py:351-354`): `if not stdin_is_interactive():` exige
`--yes-i-trust-this-source` explícito antes de instalar conteúdo de terceiro. **Antes deste port,
esse portão usava `sys.stdin.isatty()` cru** — o mesmo predicado que mente `True` sob `NUL` no
Windows (ex.: automação/CI redirecionando stdin de `NUL` via Git Bash). Um pipeline não-interativo
nessa condição seria classificado como "interativo" pelo código antigo, **pulando a exigência de
`--yes-i-trust-this-source`** e caindo no ramo interativo — que num contexto sem terminal de fato
tende a falhar por outro motivo (leitura de linha sem EOF tratado), mas na melhor das hipóteses é
comportamento indefinido para um portão de confiança de instalação de terceiro, e na pior é o tipo
de ambiguidade que se explora. O port fecha essa mesma classe de defeito no ponto onde ela mais
importa neste repositório — não é um efeito colateral incidental do port de UX, é uma correção de
segurança que o handoff não pediu que eu procurasse e que só apareceu por ler o consumidor inteiro.

## Restante do diff — supply chain, permissões, injeção

- **Nenhuma dependência nova** em `go.mod`, `npm/package.json` ou `pypi/pyproject.toml`/`setup.cfg`
  — confirmado por ausência desses arquivos no `--stat` do diff. Toda a superfície nova usa apenas
  biblioteca padrão (`ctypes`, `msvcrt`, `os`, `sys` em Python; `os`, `runtime` em Go; `os` em
  Node.js).
- **Nenhum novo `exec`/`spawn`/`system`** foi introduzido fora dos gates já cobertos no Ponto 3.
- **Atribuição:** 4 dos 5 commits da branch carregam `Co-Authored-By: lourivalgarciajunior
  <lourival.garcia@gmail.com>` — o quinto (`85c75e5`, REQ+roadmap) é artefato de governança, não
  porte de código de terceiro, e corretamente não carrega a atribuição. Cumpre a regra do roadmap.
- **Porte fiel — divergências da origem, todas já declaradas pelo roadmap e verificadas por mim
  como não-substantivas:** `python`→`python3` (bloqueado pelo ML-1C/ML-3C), modo 755 vs 644 nos
  scripts gerados, guarda de plataforma no bit de execução (Ponto 5 acima), guarda de plataforma no
  `home_dir()` Python (só ativa em `sys.platform == "win32"`, documentada e testada — não estende o
  comportamento novo a Linux/macOS, onde `expanduser("~")` já lia `$HOME`). Nenhuma dessas
  divergências amplia superfície de ataque; todas restringem o efeito da correção ao SO onde o
  defeito de fato existe.
- **`_ensure_global_adr_dir_registered`, `identity.load` (Python):** os dois call sites que
  trocaram `os.path.expanduser("~")` por `home_dir()` em `pypi/trackfw/commands/update.py` são os
  mesmos pontos que a Wave 0 já modelou (escritor de config/identidade global) — nenhum ponto de
  escrita novo, só a fonte da home mudou, consistente com o resto do port.
- **Symlink guard de `_write_ci_workflow` (`discover.py`) preservado.** O trecho só teve o
  comentário tocado (menção a `newline="\n"` no texto explicativo) pelo port mecânico do LF — a
  lógica `os.path.islink(dest)` checada antes de `_is_file`/`open` continua intacta, linha a linha
  idêntica à revisão anterior (nota de vault
  `update-segue-symlink-e-escreve-fora-do-projeto-2026-08-28`).
- **O port de LF (`newline="\n"`) é mecânico e extenso** (30+ call sites em `pypi/trackfw/`), mas
  toca só a codificação de quebra de linha na escrita — nenhum call site muda o que é escrito, para
  onde, ou sob qual guarda de permissão/symlink. Amostrei os arquivos de maior superfície (geradores
  de scripts com `chmod 0o755` — `init_gen.py`, `discover.py`) e confirmei que o `chmod` permanece
  logo após o `open`, inalterado.

---

## Um ponto que um revisor de "a correção funciona?" teria deixado passar

O handoff pediu isto de propósito, então respondo direto: **o Ponto 5** (bit de execução) é o
candidato natural — a mudança, lida isoladamente e fora de contexto, parece "desligar uma checagem
de segurança no Windows", e um revisor validando só "o teste novo passa, o defeito documentado
desaparece" teria parado aí. O que fecha a pergunta é a checagem que fiz: (a) o discriminante nunca
existiu no Windows (sempre-verdadeiro, não sinal), (b) o guard é por `GOOS` de binário, não por SO
de host, então não desarma sob WSL onde o bit volta a ser real, e (c) só um dos dois ramos da regra
foi tocado — o ramo que de fato protege contra script ausente continua de pé. Nenhum dos três fatos
está explícito no diff em uma única linha; exigiu ler `goos.go`, os dois validadores e os testes de
falsificação nas duas direções para descartar a leitura superficial "guard desligado no Windows =
regressão".

---

## Risco residual aceito (reafirmado da Wave 0, não retrabalhado aqui)

- **Instalação fantasma:** `homedir.Dir()`/`homedir()`/`home_dir()` ficam consistentes entre
  escritor e auditor do `trackfw`, mas não necessariamente com o CLI de agente real (Claude Code,
  Codex, Gemini — binário de terceiro), que resolve home pelo mecanismo nativo dele. Sob Git Bash
  com `$HOME` ≠ `%USERPROFILE%`, o `trackfw` pode reportar guard global saudável num caminho que o
  agente real nunca lê — falso positivo de saúde de um controle de negação. Aceito nesta REQ por
  decisão do arquiteto (Wave 0); vira REQ própria.
- **Nenhuma detecção de divergência entre as duas variáveis foi implementada** — decisão consciente,
  não omissão: mediria o lugar errado (comparar env vars entre si) em vez do que importa (onde o
  consumidor real lê `settings.json`).
- **`GetConsoleMode` via `ctypes` é superfície de FFI nova**, mitigada por: escopo Windows-only,
  nenhum input externo (só o handle do próprio processo), fail-`False` em três caminhos de falha
  independentes verificados, e espelhamento deliberado da mesma API que o Go já expõe em produção.
- **Item 4 da issue #216 (`check-parity-contract-coverage.sh` imprime cru, fora de `main()`)
  permanece aberto**, nomeado explicitamente pelo próprio `docs/cli-parity.md` como fronteira do
  contrato de UTF-8 — não é uma omissão silenciosa.

## Resumo por severidade

| Achado | Severidade | Bloqueante? | Remédio |
|---|---|---|---|
| — | — | — | Nenhum achado de segurança neste diff |

Nenhum achado de execução arbitrária, exfiltração, escrita fora do workspace/diretório sintético,
mascaramento de mensagem de segurança, injeção, dependência nova não auditada, ou enfraquecimento
líquido de controle foi identificado. A supressão do ramo "não executável" do guard no Windows
(Ponto 5) foi escrutinada como candidata a achado e descartada com evidência concreta (checagem
sempre-verdadeira ali, escopo por `GOOS` de binário, ramo de existência intocado, falsificação nas
duas direções nos três runtimes).

**Veredito final, repetido: APROVA. Zero bloqueantes.**
