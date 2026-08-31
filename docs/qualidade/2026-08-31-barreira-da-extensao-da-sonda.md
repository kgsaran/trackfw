# Barreira de Qualidade — Extensão da sonda de junction (Windows, 3 runtimes)

> Autor: `hefesto-tf` | Data: 2026-08-31
> Branch: `fix/sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder`
> Diff auditado: `git diff origin/main...HEAD`
> Parecer de segurança de referência (não repetido aqui): `docs/seguranca/2026-08-31-barreira-da-extensao-da-sonda.md` (`hades-tf`, APROVA COM RESSALVAS, sem bloqueantes)

---

## Veredito

**APROVA COM RESSALVAS.**

Nenhum achado bloqueante. Os cinco pontos de maior valor pedidos na revisão foram verificados por
leitura linha a linha dos três braços (`probe.go`, `probe.js`, `probe.py`) e do workflow, **mais
execução real** do que roda fora do Windows: os três probes (`table`, `rmdir-junction`) rodados de
fato em macOS (não só compilados/checados por sintaxe), `probe.go` compilado explicitamente por
nome de arquivo (o `go build ./...` da raiz **não** cobre `probe.go` — ver nota abaixo), `go vet`
sobre o arquivo, e `make quality` do repositório completo. Quatro achados de acompanhamento,
nenhum bloqueante: um já havia sido levantado pelo `hades-tf` sob a lente de segurança (vazamento
de tempdir em `probe.py`, confirmado aqui também por execução, não só por grep); um é achado próprio
(Node não mede o discriminante de produção que a Pergunta 10 existe para medir); dois são
observações de manutenibilidade.

**Nada nesta branch foi executado no Windows.** Junction não existe fora dele e
`workflow_dispatch` só dispara da branch default — a tabela real (AC9) é estruturalmente
inverificável antes do merge, como o próprio roadmap já registra na seção "Verificação diferida
para pós-merge". Separação explícita do que verifiquei:

- **Por leitura**: os três probes linha a linha, o workflow completo, o roadmap, a REQ, o parecer
  de segurança.
- **Por execução em macOS**:
  - `actionlint .github/workflows/windows-probe.yml` (limpo).
  - `git diff --quiet origin/main -- .github/workflows/quality.yml` (byte-idêntico).
  - `go build ./...` e `GOOS=windows GOARCH=amd64 go build ./...` da **raiz do módulo** — ambos OK,
    mas **não exercitam `probe.go`**: a tag `//go:build ignore` (linha 1) torna o arquivo invisível
    para `go build ./...`/`go vet ./...`, exatamente como o próprio comentário do arquivo documenta
    (linhas 16-19) — comportamento pretendido, não bug, mas significa que essas duas execuções
    sozinhas não provariam nada sobre as ~120 linhas novas (`cmdRmdirJunction`, `cmdTable`,
    `printTableRow`, `printTempDirInfo`). Corrigido com compilação explícita abaixo.
  - `GOOS=windows GOARCH=amd64 go build -o /tmp/probe-win.exe scripts/windows-repro/go/probe.go` —
    **OK**, cross-compila para Windows de verdade (nomear o arquivo contorna a tag `ignore`).
  - `go vet scripts/windows-repro/go/probe.go` — **OK**, limpo.
  - `go build -o /tmp/probe-native scripts/windows-repro/go/probe.go` (build nativo macOS) +
    **execução real** de `probe-native table` e `probe-native rmdir-junction` — ambos saem com
    `exit=0`, imprimindo `err_create=exec: "cmd": executable file not found in $PATH` cru, sem
    disfarçar como sucesso.
  - `node --check scripts/windows-repro/node/probe.js` (OK) + **execução real** de
    `node probe.js table` e `node probe.js rmdir-junction` — `exit=0`, erro
    `mklink_error=spawnSync cmd ENOENT` impresso cru — a checagem `status !== 0 || error` do item 1
    é exercitada de verdade aqui (`error` não-nulo, `status` `null`; o `||` cobre o caso `null`).
  - `python3 -c "ast.parse(...)"` sobre `probe.py` (OK, só sintaxe) + **execução real** de
    `python3 probe.py table` e `python3 probe.py rmdir-junction` — `exit=0`, erro sintetizado por
    `_mklink_junction` (`err_spawn_cmd=[Errno 2] No such file or directory: 'cmd'`) impresso cru.
    Confirma empiricamente o achado 6.1 (abaixo): os tempdirs desta execução **não** foram removidos
    pelo processo Python — precisei limpá-los manualmente depois, enquanto o run nativo do Go e a
    execução do Node não deixaram resíduo.
  - `make quality` — ver seção "Gate do repositório" abaixo.
  - greps para `exit 1`/`PASS`/`FAIL`/`ABSENT`/`REPRODUCED` no workflow e nos três probes.
- **Estruturalmente inverificável aqui**: qualquer valor real de `lstat`/`mklink /J`/`rmdir` em
  `windows-latest` — `cmd.exe`/`mklink.exe` não existem nesta máquina, então toda execução local
  acima exercita o caminho de **erro de infraestrutura**, nunca o caminho de sucesso que só existe
  no Windows. Nenhuma junction foi de fato criada ou medida por esta revisão.

---

## 1. Vacuidade — nenhum caminho encontrado onde uma pergunta pareça respondida sem medir

Verifiquei os três braços exatamente pelos exemplos citados na revisão (`spawnSync`/`subprocess`
que falha sem checar `error`/`returncode`; `try/except` largo que engole; `mklink` falho seguido de
`lstat` sobre caminho inexistente):

- **Node (`probe.js:105-115`, `cmdLstatJunction`)**: `spawnSync` é checado explicitamente —
  `if (mklink.status !== 0 || mklink.error)` — **antes** de qualquer `lstatSync` sobre a junction.
  Mesma checagem em `cmdRmdirJunction` (`probe.js:144`) e `cmdTable` (`probe.js:223`).
- **Python (`probe.py:114-121`, `cmd_lstat_junction`)**: `subprocess.run` é checado por
  `proc.returncode != 0` **antes** do `_print_lstat`. Mesma checagem em `cmd_rmdir_junction`
  (`probe.py:149`) e `cmd_table` (`probe.py:208`). O `except OSError` em `_mklink_junction`
  (linha 92-101) não engole o erro — sintetiza um `CompletedProcess` com `returncode=127` e a
  mensagem crua em `stderr`, que o chamador então checa da mesma forma que checaria uma falha real
  do `mklink.exe`. Não há caminho em que um `cmd.exe` ausente produza um `returncode == 0` falso.
- **Go (`probe.go:177-182`, `cmdLstatJunction`)**: `exec.Command(...).CombinedOutput()` tem seu
  `err` checado (`if err != nil { ...; return }`) antes do `os.Lstat`. Mesma checagem em
  `cmdRmdirJunction` (linha 234) e `cmdTable` (linha 306).

Em nenhum dos três a saída do `mklink` é assumida bem-sucedida. `printMode`/`printLstat`/
`_print_lstat` também tratam erro de `lstat` como valor de primeira classe (impresso, não
descartado) em todos os pontos de chamada — não há `try/except`/`try/catch` que descarte a exceção
silenciosamente em nenhum dos subcomandos novos.

**Conclusão do item 1: nenhum achado.** O agente aplicou a mesma disciplina de checagem nos três
runtimes, incluindo no caminho sintético de `_mklink_junction` em Python, que é exatamente o tipo
de lugar onde essa disciplina costuma vazar.

## 2. Comparabilidade dos três braços — confirmada, sem caso residual

```bash
grep -n "mklink" scripts/windows-repro/go/probe.go scripts/windows-repro/node/probe.js scripts/windows-repro/python/probe.py
```

Os três chamam `cmd /c mklink /J <link> <targetDir>` — mesma primitiva, mesmo binário
(`mklink.exe` builtin do `cmd.exe`), nenhum recurso nativo do runtime (`fs.symlinkSync(...,
'junction')` não aparece em lugar nenhum do diff; o comentário em `probe.js:30-37` documenta
explicitamente por que essa troca foi evitada — `SubstituteName`/`PrintName` do
`REPARSE_DATA_BUFFER` divergem entre `mklink.exe` e libuv, e são exatamente os campos que
`readlink()`/`LinkType` leem). Confirmei também:

- **Mesmos três alvos** em todo braço que produz a tabela final: arquivo comum, symlink real,
  junction — nessa ordem, nos três (`probe.go:278-311`, `probe.js:205-231`,
  `probe.py:189-211`).
- **Mesma ordem de criação** dentro de cada tempdir isolado por subcomando (arquivo → symlink →
  targetDir → junction), o que elimina reaproveitamento de fixture entre perguntas.

Os **nomes de campo** impressos por runtime são diferentes por necessidade, não por descuido —
`ModeSymlink`/`ModeDir`/`ModeIrregular` (Go) vs. `isSymbolicLink`/`isDirectory`/`isFile` (Node) vs.
`islink`/`S_ISLNK`/`S_ISDIR` (Python) são as primitivas nativas de cada linguagem, e são exatamente
o que esta sonda existe para comparar — uniformizar os nomes de campo esconderia a própria
divergência sob teste. Não é um defeito de comparabilidade; é o objeto medido.

**Conclusão do item 2: nenhum achado.** Não sobrou nenhum braço usando fixture diferente.

## 3. AC6 — a sonda continua sem veredito

```bash
grep -nE "^\s*exit 1" .github/workflows/windows-probe.yml        # vazio
grep -nE "PASS|FAIL|OK\b|ABSENT|REPRODUCED|✓|✗" .github/workflows/windows-probe.yml scripts/windows-repro/{go/probe.go,node/probe.js,python/probe.py}
  # únicas ocorrências são comentários que EXPLICAM a distinção sonda/regressão — não código
```

Os únicos `os.Exit(1)`/`sys.exit`/`process.exit` em todo o diff são falhas de **infraestrutura da
própria sonda** (`MkdirTemp`/`mkdir`/`chmod` falhando antes de haver qualquer coisa para medir) —
mesmo padrão que já existia em `probe.go` antes deste ML, e que a própria orientação da revisão
qualifica como correto. Nenhuma linha decide `pass`/`fail` a partir do *valor* de `ModeSymlink`,
`isSymbolicLink` ou `islink`. A tabela final imprime booleanos crus, nunca uma coluna de veredito.

**Conclusão do item 3: nenhum achado.**

## 4. Integridade da pergunta 7 (AC2)

`.github/workflows/windows-probe.yml:341-357`: `$cacheinfo = "120000,$blob,mylink"` é montado numa
variável com aspas duplas (contexto de interpolação real do PowerShell) antes de ser passado ao
`git update-index`. Logo em seguida, **antes do `checkout`**, `git ls-files --stage mylink` lê de
volta a entrada do índice e imprime o valor cru ao lado do valor esperado
(`esperado_mode=120000 esperado_blob=$blob esperado_path=mylink`). Isso é uma prova real de
integridade, não cosmética: se o `update-index` tivesse aceitado um argumento malformado e falhado
silenciosamente por outro motivo, ou gravado uma entrada divergente, a linha impressa aqui
divergiria do valor esperado — o teste discrimina "comando retornou 0" de "o índice tem a entrada
certa", que são coisas diferentes. Não há caminho em que `ls-files` imprima algo plausível sem que o
índice de fato contenha a entrada, porque a leitura é feita do próprio objeto que o comando anterior
escreveu, sem cache intermediário nem reinterpretação.

**Conclusão do item 4: nenhum achado.**

## 5. Pergunta 10 (`rmdir` sobre junction)

Os três braços (`probe.go:216-247`, `probe.js:137-182`, `probe.py:142-165`) reportam **os três
dados separadamente**, como pedido:

1. Resultado cru do `rmdir` — `os.Remove(junction)_err=`, `fs.rmdirSync(junction)_err=`,
   `Path(junction).rmdir()_err=`.
2. Se a **junction** sumiu — via `os.Lstat`/`fs.lstatSync`/`os.path.lexists` (não segue o link, não
   confunde "sumiu" com "o alvo redirecionado não existe mais").
3. Se o **alvo** sobreviveu — via `os.Lstat(targetDir)`/`fs.lstatSync(targetDir)`/
   `os.path.isdir(target_dir)`.

Confirma-se também o pedido mais específico da revisão — **a primitiva exata de cada CLI**: Python
usa `Path(junction).rmdir()` (não `os.rmdir()`), documentado no próprio comentário
(`probe.py:133-141`) como escolha deliberada porque `pypi/trackfw/integrations/manager.py:589`
chama `directory.rmdir()` sobre um `pathlib.Path`, não `os.rmdir()` sobre uma string — mesmo objeto
que a produção usa, não um primo próximo. Go usa `os.Remove` e Node usa `fs.rmdirSync` sem
`recursive`, que são os primitivos não-recursivos equivalentes usados pelas respectivas guardas de
produção citadas no ML-0A.

**Achado de acompanhamento (não bloqueante): o braço Node mede um primitivo diferente do
discriminante de produção do Node.** A tabela do ML-0A (reproduzida no roadmap) identifica o freio
de cada CLI contra subir removendo ancestrais: Go usa `info.IsDir()` (medido nas Perguntas 2/8/9/11
via `ModeDir`; a própria Pergunta 10 não reimprime `ModeDir` sobre a junction, mas o dado já está
estabelecido pelas perguntas anteriores no mesmo run), Python usa `except OSError` ao redor de
`rmdir()` (medido — é
exatamente o `Path(junction).rmdir()` da Pergunta 10), mas **Node usa
`fs.readdirSync(dir).length`** em `npm/src/integrations/manager.js:420` (`cleanEmpty`), não
`rmdirSync`. A Pergunta 10 mede `fs.rmdirSync(junction)` em vez de `fs.readdirSync(junction)`
— o próprio comentário do arquivo (`probe.js:131-136`) admite a substituição em palavras: *"medimos
diretamente se rmdirSync tem sucesso... em vez de inferir do comportamento do readdirSync."* Isso é
uma substituição deliberada e documentada, não um descuido silencioso — por isso não é bloqueante —
mas significa que **nenhuma pergunta desta sonda mede `readdirSync(junction).length`**, que é o
dado que de fato decide se `cleanEmpty` do Node trata uma junction vazia como diretório vazio (e
sobe removendo) ou não. Como `workflow_dispatch` só dispara da branch default, se a REQ de correção
precisar desse número específico para o Node, é mais um ciclo de merge+dispatch — custo concreto,
não hipotético. **Remédio sugerido, não deste PR**: acrescentar em `probe.js` uma leitura crua de
`fs.readdirSync(junction)` (comprimento e conteúdo) ao lado da medição de `rmdirSync` já existente,
no mesmo formato sem veredito.

**Conclusão do item 5: quatro dos cinco alvos de rmdir plenamente cobertos; um achado de
acompanhamento (Node mede o primitivo errado para o discriminante de produção do próprio Node).**

## 6. Manutenibilidade e duplicação

### 6.1 — Achado real, não bloqueante: `probe.py` não limpa nenhum tempdir

Confirmado independentemente (já apontado pelo `hades-tf` sob a lente de segurança/contenção; eu
confirmo sob a lente de manutenibilidade/higiene de recurso), por leitura **e por execução**:

```bash
grep -n "shutil\|rmtree\|TemporaryDirectory\|finally" scripts/windows-repro/python/probe.py
# nenhuma ocorrência
```

**Confirmação empírica** (macOS, `cmd`/`mklink` ausentes — só o caminho de erro de infraestrutura é
exercitável aqui, mas a limpeza do tempdir independe disso): rodei `python3 probe.py table` e
`python3 probe.py rmdir-junction` de verdade. Os dois tempdirs criados
(`trackfw-probe-table-*`, `trackfw-probe-rmdir-*`) ficaram em `$TMPDIR` depois do processo terminar
— precisei removê-los manualmente. Rodei os mesmos dois subcomandos em `probe.go` (build nativo) e
`probe.js`: nenhum resíduo ficou para trás nos dois. A assimetria não é só leitura de código — é
comportamento observado.

`probe.go` usa `defer os.RemoveAll(tmp)` logo após cada `MkdirTemp` (4 ocorrências —
`cmdLstatSymlink`, `cmdLstatJunction`, `cmdRmdirJunction`, `cmdTable`; os três `CreateTemp` de
arquivo comum usam `defer os.Remove(path)` no arquivo, equivalente para esse caso); `probe.js`
chama `fs.rmSync(tmp, { recursive: true, force: true })` em todo caminho de saída dos 5
subcomandos que criam tempdir. `probe.py` chama `tempfile.mkdtemp()` 5 vezes e não importa
`shutil`, não usa `tempfile.TemporaryDirectory()`, não tem `try/finally` — nenhum dos 5 subcomandos
limpa o próprio tempdir. Efeito concreto: `cmd_lstat_junction` e `cmd_table` deixam uma **junction
viva** em disco a cada execução (`cmd_rmdir_junction` remove a sua por definição do próprio teste).

Isso é uma **assimetria de padrão dentro do mesmo diff** — os três braços foram escritos para
espelhar o mesmo comportamento e um deles não segue a mesma disciplina de limpeza que os outros
dois já demonstram. Em `windows-latest` efêmero (CI) o runner é destruído ao fim do job, então não é
persistência real; mas quem rodar `python probe.py table` (ou qualquer subcomando de junction)
localmente numa máquina Windows acumula lixo em `%TEMP%`, e a próxima pessoa que tocar este arquivo
herda um padrão inconsistente sem sinal de que é inconsistente — os comentários dos outros dois
arquivos justificam a limpeza; o de Python não menciona a ausência dela.

**Remédio concreto**: envolver o corpo de cada subcomando em
`try: ... finally: shutil.rmtree(tmp, ignore_errors=True)`, ou trocar `tempfile.mkdtemp()` por
`tempfile.TemporaryDirectory()` como context manager. Baixo custo, resolve a assimetria com os
outros dois braços.
**Severidade**: acompanhamento (não bloqueante) — vira item da REQ de correção subsequente ou um
ML avulso antes do próximo uso local dos probes.

### 6.2 — Observação de acompanhamento: fixture de junction repetida 3x dentro de cada probe, e vai colidir com `checks.*` na próxima REQ

`mklinkJunction`/`_mklink_junction`/o bloco `exec.Command("cmd","/c","mklink","/J",...)` é chamado
uma vez por função auxiliar em Go (repetido inline 3x — `cmdLstatJunction`, `cmdRmdirJunction`,
`cmdTable` — não há uma função `mklinkJunction` em Go equivalente à de Node/Python, é o único dos
três sem essa extração) e uma vez auxiliar em Node/Python (`mklinkJunction`/`_mklink_junction`,
reaproveitada 3x). Isso é aceitável para código de sonda descartável — o próprio cabeçalho do
arquivo o distingue de `checks.*` — mas a REQ de correção que este roadmap prepara vai
provavelmente precisar da mesma fixture de junction dentro de `checks.go`/`checks.js`/`checks.py`
(camada 2, com veredito) para testar o freio corrigido. Se a extração para função nomeada não for
feita uniformemente em Go também nesse momento, a próxima pessoa terá 4 cópias do mesmo bloco
`mklink /J` por linguagem em vez de 1 fixture reaproveitada entre sonda e regressão — ou, pior,
duas fixtures ligeiramente diferentes que voltam a medir objetos diferentes (o próprio risco que
o item 2 desta revisão descartou para a sonda, mas que reaparece se `checks.*` reinventar a fixture
do zero). **Não é uma correção deste PR** — é um aviso para quem escrever a REQ de correção: citar
`_mklink_junction`/`mklinkJunction`/o bloco Go como referência de implementação, não recriar a
lógica.
**Severidade**: acompanhamento (informativo) — não bloqueia este PR; relevante para o próximo.

### 6.3 — Observação menor: `if: always()` com múltiplas invocações por step mascara falha de infraestrutura no meio do step

Várias steps do workflow (ex.: Pergunta 2, 8, 9, 10, 11) rodam duas ou três invocações
`go run`/`node`/`python` na mesma `run:` de `pwsh` sob `if: always()`. O PowerShell, por padrão,
não interrompe a execução de linhas subsequentes quando um comando nativo anterior sai com código
não-zero (a menos que `$LASTEXITCODE` seja checado explicitamente, o que o workflow não faz aqui de
propósito — coerente com "sem veredito"). Consequência prática: se a **primeira** invocação de um
step falhar por erro de infraestrutura do próprio probe (ex.: `MkdirTemp` falhar em
`lstat-junction`, saindo com `os.Exit(1)`) e a **segunda** tiver sucesso, o status reportado do
*step* no Actions tende a refletir o código de saída do **último** comando executado — ou seja, o
step pode aparecer verde mesmo com uma falha de infraestrutura registrada no meio do log. Isso não
viola o AC6 (não há veredito sobre o *valor medido*; o status do step nunca decidiu nada sobre
`ModeSymlink`) e não é um caminho de vacuidade (o erro continua impresso, cru, no log) — mas é uma
armadilha de leitura: "step verde" não significa "nenhuma pergunta daquele step falhou por
infraestrutura". Quem monitorar só o ícone verde/vermelho dos steps, sem ler o texto, pode perder um
`err_mkdtemp` real.
**Severidade**: informativo — nenhuma mudança necessária no design (a filosofia de "sem veredito,
leia o log" é deliberada e correta); vale como nota de operação para quem interpretar o run
pós-merge.

---

## Resumo por severidade

| # | Achado | Severidade | Arquivo:linha | Confirmado por |
|---|---|---|---|---|
| 1 | Node (Pergunta 10) mede `fs.rmdirSync` em vez de `fs.readdirSync(junction).length`, que é o discriminante real de `cleanEmpty` (`npm/src/integrations/manager.js:420`) | Acompanhamento (não bloqueante) | `scripts/windows-repro/node/probe.js:131-182` | Leitura + comentário do próprio arquivo |
| 2 | `probe.py` não limpa tempdir/junction em nenhum dos 5 subcomandos (assimetria com Go/Node) | Acompanhamento (não bloqueante) | `scripts/windows-repro/python/probe.py` (arquivo inteiro — ausência de `shutil`/`finally`) | Leitura (grep) + **execução real** (tempdir sobrou em `$TMPDIR`) |
| 3 | Fixture de junction (`mklink /J`) será provavelmente reimplementada em `checks.*` na próxima REQ; extrair/reaproveitar em vez de recriar | Informativo | `scripts/windows-repro/{go,node,python}/probe.go/js/py` | Leitura |
| 4 | `if: always()` + múltiplas invocações por step pode mostrar step verde mesmo com falha de infraestrutura registrada no meio do log | Informativo | `.github/workflows/windows-probe.yml` (steps das Perguntas 2, 8, 9, 10, 11) | Leitura |

Nenhum dos quatro é bloqueante. Nenhum compromete AC6, comparabilidade entre runtimes, ou a
integridade da pergunta 7. O achado 1 é o único que toca diretamente o "coração do ML" (comparabilidade/cobertura da Pergunta 10) e por isso está registrado como achado formal, não só nota —
mas é uma substituição documentada pelo próprio autor, não um descuido silencioso, e não bloqueia
porque a Pergunta 10 ainda produz dado real e utilizável para Go e Python, que é a maioria dos casos
já cobertos pelo ML-0A.

## Gate do repositório

```bash
make quality
```

rodado até o fim nesta sessão, sobre o worktree completo (Go + Node + Python + contratos de
paridade, incluindo `check-roadmap-barrier-contract` com seus 53 cenários e o corpus de 144
roadmaps/432 waves). **Código de saída medido de verdade — `MAKE_EXIT=0`.** A primeira tentativa
capturou o exit code do pipeline (`make quality 2>&1 | tail -200`), que em zsh sem `pipefail` é o
exit code do `tail`, não do `make` — quase sempre 0 independentemente do resultado do `make`. Refeito
sem pipe: `make quality > mq.log 2>&1; echo "MAKE_EXIT=$?"`, capturando o exit code real do `make`
diretamente. Log completo (3300+ linhas) sem nenhuma linha `make: *** [...] Error` (que `make`
sempre emite ao abortar por falha de target) e sem `FAIL` real — as únicas ocorrências de
"erro"/"fail" no texto são nomes de fixture de teste (`reson=erro de digitacao`) ou o próprio texto
`Falsification checks passed`. Última linha do log: `check-roadmap-barrier-contract: 53 cenários
OK`. Cobre AC10 da REQ (`actionlint` limpo; `make quality` verde).

## Veredito final (repetido)

**APROVA COM RESSALVAS.** Sem bloqueantes. Os cinco pontos de maior valor pedidos — vacuidade,
comparabilidade, AC6, integridade da pergunta 7, pergunta 10 — foram verificados linha a linha **e**
por execução real (não só leitura/compilação) dos três probes em macOS, incluindo o caminho de erro
de infraestrutura (`cmd`/`mklink` ausentes) que é o único exercitável fora do Windows. Quatro
achados de acompanhamento/informativos (tabela acima) não impedem o merge: nenhum abre superfície de
escrita fora do tempdir, nenhum interpreta valor medido, nenhum introduz veredito na sonda.
