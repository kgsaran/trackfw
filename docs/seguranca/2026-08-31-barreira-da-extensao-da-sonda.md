# Barreira final de segurança — extensão da sonda (junction em Node/Python, pergunta 7)

> Produzido por: `hades-tf` | Data: 2026-08-31
> Branch: `fix/sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder`
> Diff auditado: `git diff origin/main...HEAD`
> Modelo de ameaça de referência: `docs/seguranca/2026-08-30-modelo-de-ameaca-da-extensao-da-sonda.md` (ML-0A, mesmo autor)
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder.md`

---

## Veredito

**APROVA COM RESSALVAS.**

Nenhum achado bloqueante. Todos os alvos de falsificação que eu mesmo defini no ML-0A foram
respeitados pela implementação: a sonda continua sem veredito, continua imprimindo valor cru em vez
de interpretar, nenhuma primitiva de escrita sai do tempdir/workspace, `checks.{go,js,py}` não
importam as funções novas, `quality.yml` está byte-idêntico, nenhuma action nova foi introduzida.
Duas ressalvas de acompanhamento (não bloqueantes) e uma correção ao meu próprio ML-0A/vault, que
precisa entrar na REQ de correção antes de decidir o remédio — descritas abaixo.

Verificação por leitura + execução em macOS: `go build ./...`, `GOOS=windows go build ./...`
(cross-compile, sem executar), `node -c probe.js`, `python3 -m py_compile probe.py`, `actionlint
.github/workflows/windows-probe.yml` (exit 0), `git diff --quiet origin/main -- quality.yml` (sem
diferença). **Estruturalmente inverificável nesta branch**: o comportamento real do `lstat`/`mklink
/J`/`rmdir` em `windows-latest` — só existe depois do `workflow_dispatch` pós-merge, como o próprio
roadmap já registra. Não fabriquei execução alguma.

---

## 1. Alvos de falsificação — respeitados

Percorri os cinco alvos da minha própria tabela do ML-0A (seção 3 do modelo de ameaça) contra o
diff real:

- **Pergunta 7**: nenhum erro é engolido. `Write-Host "cacheinfo=$cacheinfo"` imprime o argumento
  montado antes da chamada; `git ls-files --stage mylink` é lido **antes** do `checkout`, com o
  valor cru ao lado do esperado (`.github/workflows/windows-probe.yml:349-356`). É falsificação de
  verdade (prova que o índice recebeu o valor, não só que o processo saiu com `exit 0`), não
  cosmética.
- **Junction em Node/Python**: exceções (`EPERM`, `OSError`) são impressas cruas, nunca engolidas
  nem convertidas em `sys.exit(1)`/`process.exit(1)` por causa do *valor* medido — confirmado lendo
  `probe.js:42-59`, `probe.py:31-46`. Nenhuma comparação contra "esperado" no caminho de medição.
- **`checks.py`/`checks.js` não importam as funções novas**: `grep -rn "probe" scripts/windows-repro/{go,node,python}/checks.*`
  não devolve nenhuma referência às funções de `probe.{go,js,py}` — as três ocorrências em
  `checks.py` são a variável local `encoding_probe`, não import.
- **Tabela final**: `printTableRow`/`_print_table_row` imprimem `ModeSymlink=`/`isSymbolicLink=`/
  `islink=` como booleano bruto, nunca uma coluna `OK`/`FAIL`. Confirmado nos três braços
  (`probe.go:254-264`, `probe.js:184-193`, `probe.py:168-177`).
- **`Todo link fica dentro de RUNNER_TEMP/workspace`**: a saída que eu recomendei no ML-0A (opção 3
  — medir em vez de afirmar) foi a adotada. `printTempDirInfo`/`_print_tempdir_info` imprime
  `tempdir_resolvido=... runner_temp=...` em cada subcomando que cria link — exceto
  `lstat-common`/`cmd_lstat_common`, que não cria link (correto, não precisa).

**Nenhum `exit 1` condicionado a valor medido em lugar algum do workflow ou dos três probes**
(`grep -n "exit 1" .github/workflows/windows-probe.yml` vazio; os únicos `os.Exit(1)`/`sys.exit(1)`
em `probe.{go,py}` são falhas de infraestrutura do próprio probe — `mkdtemp`/`mkdir` falhando —, o
mesmo padrão que `probe.go` original já usava antes deste ML, não uma decisão sobre o dado medido).

---

## 2. Criação de links — confinamento

Todo `MkdirTemp`/`mkdtempSync`/`tempfile.mkdtemp` das perguntas novas resolve para o diretório
temporário do processo (`%TEMP%`/`os.tmpdir()`/`tempfile.mkdtemp()`), **não** para
`$env:RUNNER_TEMP` — pré-existente ao ML-1A, já registrado como achado de precisão (não de
segurança) no meu próprio ML-0A e no ADR anterior. O ML-1A não piorou isso: em vez de reescrever a
alegação de contenção, escolheu a saída que eu recomendei (opção 3, medir e imprimir), o que é
estritamente melhor que herdar a alegação imprecisa em silêncio. Risco residual aceito, não novo.

Confirmado que **toda** criação/remoção de link nos três braços (Pergunta 8, 9, 10, 11) ocorre
dentro do tempdir próprio de cada subcomando — nenhum caminho vem de `inputs.motivo`, `github.event.*`,
nome de branch, ou qualquer valor externo. `mklinkJunction`/`_mklink_junction`/`mklink /J` recebem
sempre `junction`/`targetDir` construídos por `path.join(tmp, "...")`/`os.path.join(tmp, "...")`
com nomes literais, nunca concatenação de entrada externa.

---

## 3. Pergunta 10 — `rmdir` sobre junction: a única pergunta que destrói estado

Esta é a que exige o olhar mais duro, como pedido. Auditei os três braços linha a linha.

**Contenção estrutural**: em nenhum dos três braços o alvo do `rmdir`/`os.Remove`/`fs.rmdirSync`/
`Path.rmdir()` deriva de argv, env ou saída de `git` — é sempre `junction`, construído dentro do
próprio tempdir recém-criado pelo mesmo subcomando (`probe.go:230`, `probe.js:142`, `probe.py:147`).
E as três chamadas de remoção são **não-recursivas** (`os.Remove`, `fs.rmdirSync` sem `recursive`,
`Path.rmdir()`) — removem o *ponto de junção*, nunca o conteúdo do alvo redirecionado, que é
exatamente o que a pergunta mede em seguida (`alvo_ainda_existe`). O único delete recursivo em todo
o Pergunta 10 é a limpeza final (`fs.rmSync(tmp, {recursive:true})` em Node, `os.RemoveAll(tmp)` em
Go), sobre uma árvore cujo único reparse point aponta **para dentro da própria árvore** — não há
caminho para a remoção "vazar" para fora do tempdir de teste. Uma ressalva honesta: o
`fs.rmSync(recursive:true)` do Node, ao varrer uma árvore que ainda contém uma junction viva (se
`rmdirSync` sobre ela tiver falhado), depende do próprio comportamento de libuv que está sob
medição — contido aqui só porque o alvo apontado está dentro do mesmo tempdir, não por uma garantia
externa a essa medição.

**Se o alvo da junction não estiver onde se espera, o que é removido?** Não há esse caminho neste
desenho: o alvo (`targetDir`) é criado *pela própria pergunta*, dois nomes acima no mesmo tempdir,
imediatamente antes do `mklink /J` — não há reaproveitamento de fixture de uma pergunta anterior nem
qualquer forma do alvo apontar para fora do que a própria função acabou de criar.

**Ausência de cleanup em `probe.py` — achado real, não bloqueante.** `probe.go` usa
`defer os.RemoveAll(tmp)` em toda função que cria tempdir; `probe.js` chama
`fs.rmSync(tmp, {recursive:true, force:true})` em todo caminho de saída. **`probe.py` não limpa em
nenhum dos cinco subcomandos** — `tempfile.mkdtemp()` é chamado 5 vezes e o módulo `shutil` nem é
importado; não há `shutil.rmtree`, `tempfile.TemporaryDirectory()`, nem `try/finally`. Consequência:
`cmd_lstat_junction` e `cmd_table` deixam uma **junction viva** em disco ao sair (`cmd_rmdir_junction`
já a removeu por definição do próprio teste, então não se aplica a ela). Em `windows-latest`
efêmero isso não é escrita persistente nem exfiltração — o runner é destruído ao fim do job — mas é
a mesma classe de garantia que a seção acima defende ("nenhum link sobrevive fora do tempdir/o
runner"), e é uma assimetria real entre os três braços que alguém rodando `python probe.py table`
localmente vai notar como lixo acumulado em `%TEMP%`/`$TMPDIR`. **Remédio sugerido**: envolver o
corpo de cada subcomando de `probe.py` em `try/finally: shutil.rmtree(tmp, ignore_errors=True)`, ou
usar `tempfile.TemporaryDirectory()`. Não bloqueante — não abre superfície de escrita fora do
tempdir, só deixa de fechá-la.

---

## 4. Injeção em expressão de workflow

Todas as ocorrências de `${{ }}` em `run:` no arquivo (`grep -n '\${{' windows-probe.yml`) são
pré-existentes a este diff: `MOTIVO: ${{ inputs.motivo }}` (linha 68, já passado via `env:`, padrão
seguro já validado na barreira anterior) e `${{ steps.work.outputs.dir }}` (linhas 215, 237, 309,
382 — saída de um step do próprio workflow, não input externo). **Nenhuma das perguntas novas
(8, 9, 10, 11) contém `${{ }}` dentro de `run:`** — confirmado lendo o bloco inteiro
(`windows-probe.yml:386-449`): são chamadas diretas a `go run`/`node`/`python` com argumentos
literais, sem interpolação de workflow-expression.

A Pergunta 7 corrigida roda com `working-directory: ${{ steps.work.outputs.dir }}` (pré-existente,
não alterado neste ML) — confirma que o `git init`, `update-index` e `checkout` operam dentro de
`symlink-checkout` sob esse diretório de trabalho, não no checkout principal do workspace.

---

## 5. Injeção em PowerShell

`$cacheinfo = "120000,$blob,mylink"` — `$blob` vem de `(git hash-object -w link_target.tmp).Trim()`,
sobre um arquivo (`link_target.tmp`) cujo conteúdo é a string literal `"target.txt"` escrita duas
linhas acima pelo próprio step. `git hash-object` devolve um SHA-1/SHA-256 hex fixo (40 ou 64
caracteres hex) — não há forma de esse valor conter vírgula, aspas ou quebra que escape do
contexto de string dupla-aspas do PowerShell. Não é controlável por `inputs.motivo` nem por
qualquer valor externo. A montagem por concatenação aqui é segura porque a entrada é
estruturalmente um hash, não porque o código a sanitiza — vale registrar a distinção, mas não é um
achado.

Os steps das Perguntas 8–11 **não montam comando por concatenação a partir de valor de ambiente
nenhum** — são invocações fixas (`go run scripts/... <subcomando-literal>`), sem qualquer
`$env:` interpolado no `run:` desses blocos. Confirmado lendo `windows-probe.yml:386-449` linha a
linha.

---

## 6. Log público

Os valores novos impressos (`isSymbolicLink`/`isDirectory`/`isFile` do Node; `islink`/`S_ISLNK`/
`st_mode`/`readlink` do Python; `tempdir_resolvido`/`runner_temp`; a `index_entry_bruta` da Pergunta
7) são todos da mesma classe que `probe.go` já imprime desde antes deste ML: booleanos de modo de
arquivo, caminhos dentro de diretórios efêmeros do próprio runner, e o valor do índice git de um
arquivo de teste (`mylink`) criado pelo próprio step. Nenhum deriva de segredo, token, ou
`inputs.motivo`. `readlink()`/`os.readlink()` só é chamado sobre links criados pelo próprio probe,
nunca sobre um caminho externo — não há vetor para vazar um alvo arbitrário através do
`readlink`. Nada de novo vaza além do que a classe "infraestrutura efêmera do runner" já cobria.

---

## 7. Supply chain

`git diff origin/main...HEAD -- .github/workflows/windows-probe.yml | grep -n "uses:"` devolve
**vazio** — nenhuma `uses:` (action de terceiro) nova foi adicionada. Todos os passos novos são
`shell: pwsh` invocando `go run`/`node`/`python` sobre arquivos do próprio repositório. Sem
superfície de supply chain nova.

---

## 8. Correção à minha própria descrição do achado do ML-0A/vault — a REQ de correção precisa disto

O KG pediu para eu validar se a nota de vault `lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-
ancestral-2026-08-31.md` está tecnicamente correta antes de governar a próxima REQ. **Está correta
no mecanismo, mas duas frases superestimam o alcance e vão produzir remédio errado se citadas ao
pé da letra**:

**(a) O alcance da subida em Python não é "até o usuário" — é limitado ao `root` gerenciado.**
Confirmei lendo `pypi/trackfw/integrations/manager.py:589`:
```python
def _remove_empty(self, directory: Path, root: Path) -> None:
    while directory != root and root in directory.parents:
        try:
            if stat.S_ISLNK(directory.lstat().st_mode):
                raise IntegrationError(...)
            directory.rmdir()
        except FileNotFoundError:
            pass
        except OSError:
            return
        directory = directory.parent
```
O laço para quando `directory == root` **ou** quando `root` deixa de estar entre os ancestrais de
`directory` — a condição `root in directory.parents` é o freio de contenção. A nota de vault diz
"sobe removendo ancestrais" (correto) e o roadmap (ML-2A, linha ~301) diz "Python sobe removendo
diretórios do usuário" — **essa segunda frase é imprecisa**: a subida é limitada ao intervalo entre
a folha e `root`, não escapa para fora da árvore gerenciada. O ponto real e grave continua de pé —
**se uma junction estiver nesse intervalo e o `rmdir()` tiver sucesso sobre ela, o laço não para
onde deveria parar** — mas o remédio certo a escrever na REQ de correção é "adicionar teste de
`S_ISDIR`/`IsDir()` antes do `rmdir()`, ao lado do `S_ISLNK` que já existe", não algo que soe como
"delimitar o alcance", porque o alcance já é delimitado por `root`.

**(b) A tabela de Node no vault lista "sem teste de `isDirectory()`" corretamente, mas o texto ao
redor pode ser lido como "Node não tem contenção nenhuma" — falso.** Confirmei lendo
`npm/src/integrations/manager.js:419-427`:
```js
cleanEmpty(directory, root) {
  while (directory !== root) {
    const rel = path.relative(root, directory)
    if (!rel || rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) return
    if (!fs.existsSync(directory) || fs.lstatSync(directory).isSymbolicLink() || fs.readdirSync(directory).length) return
    fs.rmdirSync(directory)
    directory = path.dirname(directory)
  }
}
```
Há uma guarda de contenção explícita (`rel`/`isAbsolute`/`..`) que impede subir para fora de `root`
— isso é diferente do freio de junction que falta (`isDirectory()`). São duas garantias distintas:
Node tem a de contenção geográfica, falta a de tipo (diretório real vs. reparse point). A nota de
vault está tecnicamente certa sobre o mecanismo específico (não testa `isDirectory()`), mas o texto
não credita a guarda de `rel` que já existe — um leitor apressado da REQ de correção poderia propor
"adicionar contenção de root" em Node, que já existe, em vez de "adicionar teste de tipo", que é o
que falta.

**Recomendação concreta para a REQ de correção**: escrever a exposição como *"remove ancestrais
vazios dentro do `root` gerenciado e — se uma junction estiver nessa cadeia e o `rmdir` tiver
sucesso sobre ela — continua além de um reparse point que deveria ter sido recusado"*, não "sobe
removendo diretórios do usuário" nem "sem contenção nenhuma". A severidade real é a mesma (dado que
`root` normalmente é um diretório de projeto ou de artefatos governados, não o filesystem inteiro);
a imprecisão está no raio de alcance descrito, não na existência do problema.

---

## 9. Achados de acompanhamento (não bloqueantes)

1. **Pergunta 10 mede o discriminante completo só para Python.** O freio do Node
   (`readdirSync(directory).length`) depende de `readdirSync` **não** seguir a junction e listar o
   conteúdo do alvo redirecionado — nenhuma pergunta desta Wave mede isso; `probe.js` reconhece a
   lacuna no próprio comentário ("em vez de inferir do comportamento do readdirSync"). Sugestão
   para a REQ de correção: adicionar `readdir`/`listdir` sobre a junction como pergunta explícita
   antes de decidir o remédio de Node — sem isso, "adicionar `isDirectory()`" é a correção óbvia mas
   não a única possivelmente necessária, porque não se sabe se `readdirSync` já falha
   silenciosamente de outra forma sobre reparse points.
2. **`probe.py` não limpa tempdir em nenhum subcomando** (seção 3 acima) — remédio sugerido:
   `try/finally` com `shutil.rmtree(tmp, ignore_errors=True)` ou `tempfile.TemporaryDirectory()`.
3. **Falha parcial dentro de um step com múltiplos runtimes é engolida pelo `pwsh`.** As Perguntas
   8, 9, 10 e 11 rodam três comandos (`go run` / `node` / `python`) na mesma célula de `run: |`;
   `pwsh` propaga só o `$LASTEXITCODE` do último comando para decidir a cor do step — se o braço Go
   falhar por motivo de infraestrutura (não de medição) e Node/Python passarem, o step continua
   verde. É exatamente a célula "erro é engolido, step passa" da minha própria tabela de
   falsificação do ML-0A, agora alcançável por braço dentro do mesmo step. Não é regressão deste ML
   (o padrão de `if: always()` sem checagem de exit code por sub-comando já existia antes), mas a
   Wave 1 o replica em quatro steps novos em vez de restringi-lo. Não bloqueante — a saída de cada
   `go run`/`node`/`python` continua sendo impressa no log mesmo que o step "passe", então o dado
   não se perde, só o sinal de cor do step fica menos confiável do que parece.

---

## Risco residual aceito

- `%TEMP%`/`os.tmpdir()`/`tempfile.mkdtemp()` ≠ `$env:RUNNER_TEMP` — pré-existente, agora medido e
  impresso em vez de afirmado; ambos efêmeros e privados ao runner.
- Comportamento real de libuv/CPython sobre junction em `windows-latest` — estruturalmente
  inverificável antes do `workflow_dispatch` pós-merge; é o próprio propósito desta Wave.
- Runner hospedado padrão, sem Developer Mode, sem `core.symlinks` customizado — resíduo já
  nomeado por Waves anteriores, inalterado.
- `probe.py` sem cleanup de tempdir (item 9.2) — aceito como achado de acompanhamento, não
  bloqueante, por não abrir superfície de escrita fora do tempdir, só deixar de a fechar.
- Sinal de cor de step por sub-comando (item 9.3) — aceito, dado que o log continua imprimindo o
  valor cru de cada braço independentemente da cor do step.

---

## Resumo para o relatório

**Veredito: APROVA COM RESSALVAS. Nenhum bloqueante.**

Achados de acompanhamento (não bloqueantes):
1. `probe.py` não limpa nenhum dos cinco tempdirs que cria — assimetria com Go/Node, remédio:
   `try/finally` + `shutil.rmtree`.
2. Pergunta 10 mede o discriminante completo de `_remove_empty` (Python) mas só parcialmente o de
   `cleanEmpty` (Node) — falta medir `readdirSync`/`listdir` sobre a junction.
3. `pwsh` propaga só o exit code do último comando por step nas Perguntas 8/9/10/11 — falha parcial
   pode ser engolida pela cor do step (dado ainda visível no log).

Correção que precisa entrar na REQ de correção antes de decidir o remédio (não é achado novo, é
precisão sobre o achado do ML-0A/vault já registrado): a subida do `_remove_empty` do Python é
limitada ao `root` gerenciado, não "sobe removendo diretórios do usuário"; e `cleanEmpty` do Node
já tem contenção geográfica (`rel`/`isAbsolute`), falta apenas o teste de tipo (`isDirectory()`).
Escrever o remédio como "adicionar teste de tipo ao lado do teste de link já existente", não como
"adicionar contenção".
