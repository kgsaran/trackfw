# Barreira final de qualidade — PR #229 (port das correções do reporter da issue #216)

> Autor: `hefesto-tf` | Data: 2026-09-01 | Tipo: barreira de qualidade, não implementação.

## Método

`git diff origin/main...HEAD` (4761 linhas, 78 arquivos) comparado programaticamente contra
`gh pr diff 222|223|224|225` (linha a linha, `+`/`-`, ignorando offsets de contexto), leitura de cada
arquivo tocado, falsificação própria das 3 guardas de vacuidade com fixtures isoladas em `/tmp` (sem
tocar nada versionado), execução de `python3 -m pytest`, `bash scripts/check-*.sh` e `make quality`
localmente. Nenhum arquivo de produto editado.

## Veredito

**APROVA.**

Nenhum bloqueante. Um achado de acompanhamento (duplicação leve entre os 3 gates novos, ver §6) e uma
ressalva de nomenclatura no Contrato 2 (ver §7). Fidelidade do port confirmada por comparação
programática, não por amostragem — nenhum arquivo dos 4 PRs ficou de fora do branch, nenhuma lógica
divergiu além do declarado.

---

## 1. Fidelidade do port

Extraí `+`/`-` de cada hunk de `gh pr diff 222|223|224|225` e de `git diff origin/main...HEAD`,
normalizei espaços/encoding, e comparei por arquivo (68 arquivos tocados pelos 4 PRs, todos presentes
no branch — zero arquivo ausente).

**Divergências encontradas, todas as 5 batendo exatamente com os desvios declarados no roadmap:**

| arquivo | divergência | causa |
|---|---|---|
| `scripts/check-homedir-parity.sh` | `python` → `python3` | desvio declarado (ML-1C/ML-2B/ML-3C) |
| `scripts/check-python-writes-lf.sh` | `python` → `python3` | idem |
| `scripts/check-tty-detection.sh` | `python` → `python3` (2 ocorrências) | idem |
| `pypi/trackfw/cli.py` (via #223 e #224) | `→` vs `→` | falso-positivo do meu extrator — `gh pr diff` escapa Unicode, o branch tem o char literal; mesmo byte semântico, confirmado por `grep` direto |

Nenhuma linha de lógica revisada nos PRs foi alterada em trânsito. Nenhuma adição no branch fica sem
explicação: toda linha extra em relação aos 4 PRs corresponde a uma das guardas de vacuidade
(ML-1C/ML-2B, comentadas e assinadas), ao retarget do Cenário 81 (assinado `RETARGETED 2026-08-31
ML-1A`), ou a realinhamento de `gofmt` num bloco de constantes existente em
`internal/commands/agents_models.go` (efeito colateral de uma constante mais longa ter sido
adicionada — não é lógica, é alinhamento de coluna).

**Nada do que os 4 PRs traziam ficou de fora sem justificativa.** O item 4 (gate de cobertura
`check-parity-contract-coverage.sh` morrendo em cp1252) permanece intocado — e isso é o esperado
segundo a própria análise prévia e o roadmap: nenhum dos 4 PRs o corrige.

## 2. Os desvios declarados são justificáveis?

| desvio | julgamento | evidência |
|---|---|---|
| `python`→`python3` nos 3 gates | ✅ correto, execução apenas | `which python` não existe neste PATH; `python3` sim. A lógica de detecção (parser Python, grep, comparação) é idêntica ao PR original — só o interpretador invocado muda |
| modo 755 nos 3 gates novos | ✅ correto | `git diff` confirma `new file mode 100755` para os 3; necessário porque `parity:` invoca os scripts sem `bash` na frente |
| par duplicado cosmético mantido verbatim no teste do #225 | ⚠️ **não localizei o par específico citado no roadmap.** Vasculhei `pypi/tests/test_generators_write_lf.py` (único arquivo de teste do #225) e o próprio `check-python-writes-lf.sh` linha a linha — não achei duplicação óbvia nem ocorrência que altere comportamento. Registro como **não verificado por mim**, não como confirmado: o julgamento original é de `apolo-tf`/arquiteto, não reproduzido aqui. Não é bloqueante — por definição é cosmético — mas não sustento um ✅ sobre algo que não localizei |
| retarget do Cenário 81 em `check-gates-falsify.sh` | ✅ correto — ver leitura linha a linha abaixo |
| ligar os 3 gates ao `Makefile` | ✅ correto, execução apenas — `make -n parity` expande os 3 |
| guardas de vacuidade (3x) | ✅ corretas — falsifiquei eu mesmo, ver §3 |

**Nenhum desvio alterou lógica revisada.** Verifiquei em particular o retarget do Cenário 81
(`scripts/check-gates-falsify.sh`, linha ~7738-7758): a âncora do `sed` mudou de
`case info\.Mode()&0111 == 0:` (cláusula completa) para `info\.Mode()&0111 == 0:` (substring), porque
o port do #222 Grupo B prefixou a condição com `CurrentGOOS != "windows" &&`. A mudança é puramente
sintática — o **mesmo delta semântico** (inverter `== 0` para nunca casar) continua sendo produzido,
só a âncora textual do `sed` foi ajustada para sobreviver ao guard de plataforma que o próprio port
introduziu. Comentário no diff (`RETARGETED 2026-08-31 ML-1A`) aponta a nota de vault correspondente
(`vault/notes/falsify-cenario-pina-linha-de-fonte-por-sed-guard-de-plataforma-quebra-2026-08-31.md`),
que existe e documenta o raciocínio.

## 3. As três guardas de vacuidade — falsificadas por mim, com fixture em `/tmp`

Não confiei no registro do roadmap. Extraí a lógica de cada guarda para um script standalone em
`/tmp/hefesto-fixture-*`, e testei os dois casos que o roadmap identifica como as formas historicamente
vácuas neste projeto: **diretório existente mas vazio** e **diretório ausente inteiramente**.

### `check-python-writes-lf.sh`

Fixture: cópia do script real em `/tmp/hefesto-fixture-lf`, `cwd` = raiz da fixture (replica o
ancoradouro relativo que o script usa de propósito — âncora absoluta destruiria a guarda porque o
`os.walk()` da varredura real é relativo ao cwd do chamador, não a `$ROOT`).

| cenário | resultado |
|---|---|
| `pypi/trackfw/` existe, vazio | `exit=1`, mensagem explícita da guarda: *"scan visited zero .py files ... refusing to pass silently"* |
| `pypi/trackfw/` ausente | `exit=1` — `set -euo pipefail` interrompe no próprio `find` (mensagem de erro do `find`, não a mensagem custom da guarda, mas o efeito é o exigido: falha alta, não passagem silenciosa) |
| normal (repo real) | `exit=0`, *"Escrita em LF: nenhuma chamada sem newline explicito."* |

**Nota:** no cenário "ausente", a guarda não chega a imprimir sua própria mensagem — quem barra é o
`set -e` sobre o `find` falhando dentro de `SCANNED=$(...)`. O efeito funcional exigido (não passar
silenciosamente) está garantido, mas o diagnóstico ao operador é menos claro que nos outros dois gates
(ver acompanhamento em §8).

### `check-homedir-parity.sh`

Fixture isolada em `/tmp/hefesto-fixture-homedir` reproduzindo só a seção de guarda de vacuidade (a
seção "por efeito" depende de binários reais dos 3 CLIs, verificada separadamente rodando o script
completo no repo real).

| cenário | resultado |
|---|---|
| todos os 3 diretórios populados | `exit=0` |
| `pypi/trackfw` esvaziado (arquivo removido) | `exit=1`, mensagem da guarda |
| `pypi/trackfw` ausente inteiramente | `exit=1`, mensagem da guarda — o `2>/dev/null \|\| true` no `find` evita o problema que o roadmap registra como "quase escapou pela segunda vez" |
| script completo no repo real | `exit=0`, *"Paridade de home: os 3 runtimes honram $HOME..."* |

Este gate é o único ancorado por `$ROOT` (variável absoluta), e por isso é o único cujo `find` precisa
do `2>/dev/null || true` explícito — sem ele, `find` num diretório ausente sob `set -e` mataria o
script sem a mensagem custom, exatamente o defeito que o roadmap registra como corrigido no ML-2B.
Confirmei que a correção está presente e funciona nos dois sentidos.

### `check-tty-detection.sh`

Mesmo padrão, fixture em `/tmp/hefesto-fixture-tty`.

| cenário | resultado |
|---|---|
| `pypi/trackfw` vazio | `exit=1`, mensagem da guarda |
| `pypi/trackfw` ausente | `exit=1`, mensagem da guarda (também usa `2>/dev/null \|\| true`) |
| populado (controle) | `exit=0` |
| script completo no repo real | `exit=0`, com a mensagem honesta *"efeito NAO exercitado — neste sistema sys.stdin.isatty() já devolve False..."* seguida de *"Deteccao de TTY: ..."* |

**As três guardas de vacuidade falsificam corretamente nas duas direções.** A única imperfeição é o
diagnóstico degradado (mas não a segurança) do primeiro gate no cenário "diretório ausente" — não é
bloqueante porque o efeito exigido (nunca passar em silêncio) está garantido.

## 4. Cobertura de teste — falsificação nas duas direções, honestidade Windows-only

Verifiquei os testes Go, Node e Python que acompanham os 3 defeitos que dependem de plataforma (item 3,
bit de execução) e o item 6 (isatty):

- **Go** (`validator_credential_guard_test.go`, `validator_git_branch_guard_test.go`): dois testes por
  arquivo, um força `CurrentGOOS = "linux"` e prova que a regra **dispara** sem bit +x, outro força
  `CurrentGOOS = "windows"` e prova que a regra **não dispara** — mas a checagem de existência do
  script **continua** disparando no Windows. Falsificação real nas duas direções, sem gate por SO.
- **Node** (`npm/tests/validator.test.js`): mesmo padrão via `validator._setPlatformForTest('linux'
  |'win32')`.
- **Python** (`pypi/tests/test_validator.py`, `test_git_branch_guard_validator.py`): mesmo padrão via
  `_set_platform_for_test`.

Nos três runtimes, os testes que dependem de comportamento *exclusivamente Windows* (CRLF, tty
`GetConsoleMode`) **se declaram honestamente como guarda de regressão fora do Windows**, não como
reprodução:

- `pypi/tests/test_generators_write_lf.py`: docstring diz *"Num Linux/macOS ... estes testes passam com
  ou sem a correção: ali eles valem como guarda de regressão, não como reprodução. Em Windows eles
  nascem vermelhos sem a correção."* — não há alegação de reprodução onde não há.
- `pypi/tests/test_cli_encoding.py::TestCliEmConsoleCp1252`: alega *reprodução determinística* via
  `PYTHONIOENCODING=cp1252`, o que é diferente de CRLF/isatty — este é o único dos itens onde a
  simulação cross-plataforma é genuína, porque a codepage é uma env var do interpretador, não uma
  API do SO. **Verifiquei isso empiricamente**, não por leitura: copiei `pypi/` para
  `/tmp/hefesto-cli-check`, desabilitei a chamada a `_force_utf8_output()` na cópia, e rodei
  `PYTHONIOENCODING=cp1252 python3 -m trackfw --help` — quebrou com
  `UnicodeEncodeError: 'charmap' codec can't encode character '→'`, a mesma causa citada no
  docstring. Com o código real (chamada presente), o mesmo comando sai limpo. A causalidade é real,
  não hipotética, e reproduzida **neste** SO (macOS), não presumida do Windows.

**Nenhum teste se apresenta como "reprodução" sem ser.** Os testes CRLF/isatty são explícitos sobre
serem guarda de regressão fora do Windows; o teste cp1252 é reprodução real, e eu confirmei a
causalidade rodando-o com e sem a correção.

## 5. `pypi/trackfw/tty.py`

Módulo novo, 69 linhas, autocontido. Pontos avaliados:

- **Legibilidade:** boa. Docstring do módulo explica o *porquê* (mentira do `isatty()` para `NUL` no
  Windows) e a *decisão de projeto* (usar o mesmo `GetConsoleMode` que `charmbracelet/x/term` já usa no
  Go, em vez de inventar heurística paralela) — citação verificável: `internal/commands/*.go` de fato
  importa `cbterm "github.com/charmbracelet/x/term"` e chama `cbterm.IsTerminal`.
- **Tratamento de erro:** dois `except Exception` amplos, ambos comentados com `# noqa: BLE001` e
  justificativa (stream substituído em teste/pipeline sem `fileno()`/`isatty()`; sem handle utilizável
  não há console). O padrão "falha para `False`" é o comportamento seguro correto para um predicado que
  decide se deve promptar — nunca promptar por engano é preferível a promptar indevidamente.
- **Comportamento em não-Windows — verificado, não presumido:** rodei
  `PYTHONPATH=pypi python3 -c "from trackfw.tty import stdin_is_interactive, stdout_is_interactive; ..."`
  neste macOS (`sys.platform == 'darwin'`) — ambas retornam `False` sob stdin não interativo, idêntico
  ao `isatty()` cru, confirmando que o branch `sys.platform == "win32"` nunca dispara fora do Windows e
  os imports de `ctypes`/`msvcrt`/`wintypes` (feitos *dentro* da função, não no topo do módulo) nunca
  são alcançados — decisão correta, porque `msvcrt` não existe fora do Windows e um import de topo
  quebraria a importação do módulo em qualquer outro SO.

Nenhum problema de legibilidade ou tratamento de erro encontrado no código do módulo.

**Achado de cobertura, não bloqueante mas relevante ao ponto 4 da tarefa:** `pypi/trackfw/tty.py`
**não tem teste unitário direto.** `ls pypi/tests/ | grep -i tty` não retorna nada, e confirmei em
`gh pr diff 224` que o PR original também não trouxe um (`grep -c test_tty /tmp/pr224.diff` → 0) — é
uma lacuna herdada do PR, não uma infidelidade do port. O único arquivo de teste que referencia o
módulo é `pypi/tests/test_scope_resolution.py`, e ele faz
`monkeypatch.setattr(integrations_command, "stdin_is_interactive", lambda: True/False)` — **substitui
a função inteira por um stub**, não exercita `_is_interactive`, `_windows_is_console`, nem os dois
caminhos `except Exception → False` que elogiei acima. A única coisa que roda o módulo de verdade é
`scripts/check-tty-detection.sh`, que neste macOS imprime *"efeito NAO exercitado"* — informativo, não
uma asserção. **Zero cobertura assertiva de `tty.py` fora do Windows.** Não é bloqueante (o gate de
efeito e o teste Go/Node equivalentes cobrem a garantia de produto por outra via, e a lacuna já
existia no PR original), mas é o fato de cobertura mais importante deste módulo e precisa estar
registrado, não só a leitura elogiosa do código.

## 6. Duplicação e manutenibilidade entre os gates novos e os existentes

**Convenção respeitada, não violada.** O padrão de "guarda de vacuidade" (`P2 vacuity guard`) já existe
em **30 dos 39** scripts `check-*.sh` do repositório antes deste PR (`check-agent-hooks-parity.sh`,
`check-static-assets.sh`, etc.) — os 3 gates novos seguem a mesma convenção textual e estrutural, não
inventam um mecanismo paralelo. Isso é reuso de idioma estabelecido, não duplicação problemática.

**Duplicação leve, de acompanhamento, não bloqueante:** os 3 gates novos repetem ~10 linhas de shell
(`find ... || true; if [ -z "$x" ]; then echo ...; fail=1; fi`) cada um, sem helper compartilhado. Isso
espelha o padrão pré-existente do projeto — cada um dos 39 `check-*.sh` é standalone, sem `source` de
lib comum — então **não é uma regressão introduzida por este PR**, é a convenção herdada. Extrair um
`scripts/lib/vacuity-guard.sh` sourceable seria uma melhoria de manutenibilidade real, mas tocaria os
39 scripts existentes para ser consistente, o que violaria a regra de porte fiel desta REQ (mudança de
escopo, não de execução). **Registro como débito técnico de baixo risco, não como bloqueante deste PR.**

## 7. O Contrato 3 do ML-4A — os 3 gates cumprem o próprio contrato que escrevi?

**Correção de escopo em relação à primeira versão desta seção:** o Contrato 3 não tem duas
propriedades, tem **quatro** (`docs/cli-parity.md`, seção "Princípios de design de gates (P1–P4)",
subseção "Gate ligado é o que revela os outros defeitos"). Reli o texto verbatim e verifiquei cada
uma individualmente para os 3 gates, em vez de checar só as duas mais óbvias:

| # | propriedade (texto exato do contrato) | evidência que verifiquei |
|---|---|---|
| 1 | **Ligado ao `Makefile`** — `make -n parity` deve expandir o script | ✅ `make -n parity` expande os 3 (linhas 25-27 do `Makefile`) |
| 2 | **Reprova sob vacuidade** — zero itens visitados ⇒ reprova, com mensagem nomeando o corpus | ✅ falsifiquei os 3 em fixtures `/tmp` (§3): todos reprovam com diretório vazio e com diretório ausente |
| 3 | **A guarda de vacuidade usa o mesmo cwd e os mesmos caminhos que a varredura real** | ✅ verifiquei por leitura + fixture: `check-python-writes-lf.sh` usa `pypi/trackfw` relativo ao cwd do chamador nos dois lados (guarda e varredura real via `os.walk`); `check-homedir-parity.sh` e `check-tty-detection.sh` ancoram os dois lados em `$ROOT` de forma consistente. Não achei nenhum dos dois modos de vacuidade-vácua que o próprio contrato cita como precedente |
| 4 | **`python3`, nunca `python`** | ✅ `grep -n '\bpython\b' ... \| grep -v python3` nos 3 scripts não retorna nenhuma invocação de comando — só ocorrências em string/comentário/label |

**Os três gates satisfazem as quatro propriedades — verificado individualmente, não presumido pela
tabela original (que só cobria as propriedades 1 e 2).**

A anotação `partial=` no Contrato 3 continua correta, e é uma alegação **diferente** da tabela acima:
ela não diz "os gates não cumprem as 4 propriedades", diz que **nenhum gate audita automaticamente,
para sempre, que eles continuam cumprindo** — `grep` confirma zero ocorrência de
`python-writes-lf`/`homedir-parity`/`tty-detection` em `check-gates-falsify.sh` (propriedades 1-3 sem
cenário registrado), e nenhum gate varre `check-*.sh` por invocação nua de `python` (propriedade 4).
As duas alegações não se confundem: eu, como revisor humano nesta barreira, cumpri o papel que o
próprio contrato reserva para "checklist de revisão humana"; nenhuma máquina fará isso automaticamente
da próxima vez que alguém tocar nesses 3 scripts. **Sem over-claim, e agora com as 4 propriedades
efetivamente checadas, não só as 2 mais visíveis.**

**Contrato 2 (UTF-8 na saída) — a categorização `gap` está correta, verificado no código do checker.**
`scripts/check-parity-contract-coverage.sh` distingue explicitamente 4 estados
(`gate_full`/`gate_partial`/`gap`/`none`), com `gap` reservado para "sem gate, com motivo declarado" —
exatamente o caso do Contrato 2 (`gap reason=... Go e Node.js já escrevem UTF-8 ... não há paridade de
3 runtimes a verificar aqui`), distinto de `partial=` (gate existe, cobertura incompleta). A anotação
usa a categoria certa do vocabulário do próprio checker, não uma aproximação. Ressalva anterior
resolvida.

## 8. `make quality` e testes — execução local

- `python3 -m pytest pypi/tests/test_cli_encoding.py -q` → **5 passed**.
- `bash scripts/check-python-writes-lf.sh` → `exit=0`.
- `bash scripts/check-homedir-parity.sh` → `exit=0`.
- `bash scripts/check-tty-detection.sh` → `exit=0`.
- `make quality` (sem pipe, exit code do `make` capturado diretamente, rodou em background por
  exceder 120s): **`MAKE_EXIT=0`**. Cobre `test` (Go), `test-node`, `test-python`, `lint` e `parity`
  — incluindo `check-gates-falsify.sh` (181 cenários, todos `OK`) e
  `check-roadmap-barrier-contract.sh` (53 cenários, todos `OK`, corpus com 144 arquivos/432 waves
  auditado por hash). `grep -c FAIL` no log bruto retorna 62 ocorrências, todas rótulos de cenário do
  corpus (`"113 failure"`, `"434 failure"` — contagem de casos negativos do próprio corpus de teste,
  não falha de execução); confirmei separadamente que não há nenhum `--- FAIL`/`FAIL:`/`FAILED` de Go
  test ou pytest no log — zero ocorrências.

## 8-bis. Atribuição — regra 🔴 do roadmap, checada nos 5 commits

O roadmap exige `Co-Authored-By: lourivalgarciajunior <lourival.garcia@gmail.com>` em **todo commit**.
Rodei `git log --format='%H %s' origin/main..HEAD` e inspecionei o corpo dos 5 commits:

| commit | traz código de PR do reporter? | tem `Co-Authored-By: lourivalgarciajunior` |
|---|---|---|
| `85c75e5` (REQ+roadmap) | não (governança) | ✅ |
| `4c3291d` (Wave 1) | sim (#222 Grupo B, #225) | ✅ |
| `5a01817` (Wave 2) | sim (#222 Grupo A) | ✅ |
| `0f0e1ad` (Wave 3) | sim (#223, #224) | ✅ |
| `ee8a735` (Wave 4, ML-4A) | **não** — só `docs/cli-parity.md`, escrito por `hefesto-tf` sintetizando os contratos que os gates *impõem*, zero linha copiada de qualquer PR | não tem, e **corretamente não tem** |

**Sem violação.** A regra existe para atribuir código dele; o commit `ee8a735` não porta nenhuma
linha do reporter — é documentação original produzida nesta REQ. Exigir a atribuição ali seria
atribuição falsa, o oposto do que a regra protege.

## 9. Limite — o que só o CI fecha

Os 5 defeitos corrigidos (itens 1, 2, 3, 5, 6) só se manifestam em Windows. Não há runner Windows neste
ambiente. Tudo o que verifiquei acima foi:

- **Por leitura**: fidelidade do port linha a linha contra os 4 PRs originais (comparação programática,
  não amostragem); leitura de `tty.py`; leitura dos desvios declarados.
- **Por execução local** (macOS/Linux): as 3 guardas de vacuidade falsificadas com fixtures reais em
  `/tmp`; o teste `TestCliEmConsoleCp1252` reproduzido com e sem a correção (causalidade real, não
  presumida); testes Go/Node/Python de plataforma forçada (`CurrentGOOS`/`_platform`/
  `_current_platform`), que são falsificação real por construção, independente de SO real.
- **Só o CI fecha**: a contagem da camada 2 caindo de 8 para 3 `REPRODUCED` em runner Windows real —
  isso não foi e não pode ser simulado aqui, e não fabrico essa evidência.

---

## Veredito (repetido)

**APROVA.** Nenhum bloqueante.

**Achados de acompanhamento (não bloqueantes):**
1. §6 — duplicação leve (~10 linhas × 3) do idioma de guarda de vacuidade entre os 3 gates novos, sem
   helper compartilhado. Convenção herdada do projeto (30/39 gates já assim), não regressão.
   Candidato a `scripts/lib/vacuity-guard.sh` numa REQ própria, não neste port.
2. §3 — diagnóstico degradado (mas não a segurança) de `check-python-writes-lf.sh` no cenário
   "diretório ausente": falha por `set -e` no `find` cru, sem a mensagem custom da guarda. Efeito
   exigido (não passar em silêncio) está garantido; a mensagem poderia ser mais clara com
   `2>/dev/null || true` no mesmo padrão dos outros dois gates.
3. §5 — `pypi/trackfw/tty.py` não tem teste unitário direto (lacuna herdada de #224, não introduzida
   pelo port); `test_scope_resolution.py` faz stub da função inteira em vez de exercitar o módulo, e
   `check-tty-detection.sh` roda em modo "efeito não exercitado" neste ambiente. Zero cobertura
   assertiva de `_is_interactive`/`_windows_is_console` fora do Windows — candidato a
   `pypi/tests/test_tty.py` (testável fora do Windows via mocks de `stream.isatty()`/`fileno()` e
   `sys.platform`), numa REQ própria de cobertura, não neste port.
4. §2 — não localizei o "par duplicado cosmético" citado no roadmap (ML-1B) dentro do escopo do #225;
   não é uma contradição do relato original, é uma verificação minha que não converge — reportado como
   não confirmado, não como erro alheio.
