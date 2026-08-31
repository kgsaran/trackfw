# Modelo de Ameaça — extensão da sonda de Windows (junction em Node/Python, pergunta 7)

> Produzido por: `hades-tf` | Data: 2026-08-30
> REQ: `docs/req/REQ-2026-08-30-sonda-nao-responde-a-pergunta-7-e-nao-mede-junction-em-node-e-python-a-guarda-de-symlink-pode-estar-furada-nos-3-clis-no-windows.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-a-pergunta-7-volta-a-responder.md`
> ML: ML-0A, Wave 0 (bloqueante — nenhuma linha de implementação escrita aqui)

---

## Veredito antecipado

Nenhum achado bloqueante **para a Wave 1 em si** (pergunta 7 corrigida, junction medida em
Node/Python, tabela comparativa) — o desenho não abre primitiva de escrita fora de
`RUNNER_TEMP`/workspace, não amplia o que já vaza em log público além de infraestrutura efêmera, e
não introduz veredito nem interpretação, desde que siga o padrão já usado por `probe.go`. Um achado
de **precisão de alvo de falsificação** sobre a alegação "todo link fica dentro de
`RUNNER_TEMP`/workspace" e um achado de **enumeração** (duas superfícies fora dos arquivos citados
pela REQ) são endereçáveis dentro do próprio ML-1A sem crescer de escopo. Um quarto achado **não é
de precisão** — é uma **correção à classificação que o KG me pediu para contestar ou confirmar**: a
classe 2 (guarda "salva por acidente") vale só para o Go; Node e Python não têm o mesmo freio, e
tratá-los como a mesma classe teria produzido o roadmap de correção errado para dois dos três CLIs.
Isto não bloqueia a Wave 1 (que não corrige guarda nenhuma), mas é a informação que a REQ de
correção subsequente precisa antes de decidir o remédio.

---

## 1. Completude da enumeração

Busquei nas duas direções pedidas: (a) todo ponto que cria reparse point/symlink/junction, (b) todo
ponto que decide por `ModeSymlink`/`isSymbolicLink`/`islink`/`S_ISLNK`, nos três CLIs.

### 1a. Pontos de criação (comandos executados, não conclusão)

```bash
grep -rn "os\.Symlink(" internal/ scripts/
grep -rln "mklink" . 2>/dev/null | grep -v .git/
grep -rn "symlinkSync\|\.symlink(" npm/src/
grep -rn "os\.symlink\|readlink" pypi/trackfw/
```

- **Produção** (`internal/`, `npm/src/`, `pypi/trackfw/`): **nenhum** ponto de produção cria
  symlink/junction. Toda criação em `internal/` é em `*_test.go` (fixtures de teste) ou em
  `scripts/windows-repro/go/probe.go`. `npm/src/` e `pypi/trackfw/` não criam symlink em nenhum
  ponto — só leem/decidem sobre eles. Confirma o que a REQ já assumia (Negative Scope): a sonda é a
  única coisa no repositório, fora de teste, que planta reparse points.
- **A sonda em si** (`scripts/windows-repro/go/probe.go`): cria symlink via `os.Symlink`
  (`cmdLstatSymlink`) e junction via `cmd /c mklink /J` (`cmdLstatJunction`), ambos dentro de
  `os.MkdirTemp("", ...)` — **não** dentro de `$env:RUNNER_TEMP\probe-work`. Ver achado da seção 2.
- **O workflow** (`.github/workflows/windows-probe.yml`, Pergunta 7): cria um symlink **versionado**
  via plumbing do git (`update-index --cacheinfo 120000,...` + `checkout`), dentro de
  `$env:RUNNER_TEMP\probe-work\symlink-checkout`.
- **A Wave 1 vai adicionar**: um `probe.js` e um `probe.py` novos em `scripts/windows-repro/node/` e
  `scripts/windows-repro/python/` (conforme roadmap — arquivos novos de sonda, distintos de
  `checks.js`/`checks.py`, que são camada 2/regressão). Presumo, por paridade com `probe.go`, que
  vão criar symlink/junction via `fs.symlinkSync`/`os.symlink` + `mklink /J` também. Isso **ainda
  não existe** — é a superfície que a Wave 1 introduz e que este documento modela preventivamente.

### 1b. Pontos de decisão por modo/atributo de link (nos 3 CLIs)

```bash
grep -rn "ModeSymlink" internal/ | grep -v _test.go
grep -rn "isSymbolicLink" npm/src/
grep -rn "islink\|S_ISLNK" pypi/trackfw/
```

Dezesseis pontos de decisão de produção — 3 na classe 1, 3 na classe 2, 10 na classe 3 (3 Go + 4
Node + 3 Python; um dos 4 do Node, `discover.js:603`, é a variante `writeCIWorkflowForce` morta,
sem chamador — ver item 2 da lista de superfícies fora da REQ, logo abaixo) — em três classes por
mecanismo, não por contagem simétrica entre CLIs. A tabela abaixo estende a classificação que o KG
pediu para eu contestar/confirmar na seção "Resposta à classificação das três guardas" mais abaixo.
**Ressalva que muda a classe 2**: a simetria de mecanismo entre os 3 CLIs não se sustenta — ver a
correção logo após a tabela.

| Classe | Go | Node | Python |
|---|---|---|---|
| Guarda de ancestral (percorre a cadeia) | `manager.go:702` `rejectSymlinks` | `manager.js:69` `assertNoSymlinks` | `manager.py:82` `_reject_symlinks` |
| Guarda de diretório salva por acidente (só Go — ver correção abaixo) | `manager.go:582` `removeEmptyAncestors` ✅ freio confirmado | `manager.js:423` `cleanEmpty` ⚠️ freio diferente, não medido | `manager.py:589` `_remove_empty` ⚠️ sem freio equivalente confirmado |
| Guarda de folha (só o último componente) | `update.go:1869`, `:1894`; `discover.go:268` | `update.js:197`, `:223`; `discover.js:360`, `:603` | `update.py:194`, `:220`; `discover.py:499` |

**Correção à própria tabela — a classe 2 não é simétrica, é só do Go.** Reli os três trechos linha a
linha antes de escrever a resposta da seção final, e o mecanismo que salva `removeEmptyAncestors`
(Go) — `!info.IsDir()` testado *depois* do teste de symlink, interrompendo o loop — **não existe**
em `manager.js:423` (`cleanEmpty`) nem em `manager.py:589` (`_remove_empty`). Detalhe na seção final;
a tabela acima lista os três por *função equivalente*, não por *mecanismo de proteção equivalente* —
e é o mecanismo, não a função, que decide se a garantia se sustenta.

Duas superfícies **fora** da lista da REQ, que a busca ampla trouxe:

1. **`internal/generators/update.go:2323` `copyPath`** (e seu par
   `pypi/trackfw/commands/update.py:667` `_copy_path`) — usados para popular o sandbox de `--dry-run`
   copiando o projeto real para um diretório descartável. Diferente das guardas de folha acima, este
   é **recursivo por nível**: cada entrada de diretório recebe seu próprio `Lstat`/`lstat`, não só o
   caminho final — então não herda o buraco de ancestral-nunca-olhado. Mas herda a **cegueira de
   junction** pela mesma via: no Windows, `Lstat` de uma junction devolve `ModeDir=false` e
   `ModeSymlink=false` (medido pela sonda), então `copyPath` cai no ramo "arquivo comum" e tenta
   `os.ReadFile`/`open()` sobre um caminho que é na verdade um diretório-por-redirecionamento —
   **inferência, não medição**: não testei isto em Windows real, mas ler um reparse point de
   diretório como arquivo comum é falha de I/O na maioria dos SOs, então presumo que aborta o
   `--dry-run` com erro em vez de seguir silenciosamente ou copiar conteúdo de fora do projeto. Se
   confirmado, **não é achado de escrita/exfiltração** — é, na pior hipótese, uma negação
   de serviço local do `--dry-run` diante de uma junction dentro da árvore do projeto. Não tem
   equivalente Node porque a arquitetura de `--dry-run` do Node não usa sandbox de filesystem
   (`dryRun` é um parâmetro que pula a escrita, não uma cópia prévia) — divergência arquitetural
   preexistente, não gap de paridade desta REQ.
2. **`npm/src/commands/discover.js:593` `writeCIWorkflowForce`** — variante "force" do escritor de
   `trackfw-validate.yml`, com o mesmo guard de symlink (`isSymbolicLink()`) que `writeCIWorkflow`.
   Exportada (`module.exports.writeCIWorkflowForce`) mas **sem nenhum chamador** em `npm/src/`
   (confirmado por grep) — não há flag `--force` equivalente nem em Go (`internal/discover/discover.go`
   só tem `writeCIWorkflow`, uma função) nem em Python (`_write_ci_workflow`, uma função). É código
   morto hoje, inalcançável a partir da CLI. Não é achado de segurança (o guard nela está correto),
   é uma nota de paridade para o dono do código — fora do escopo desta Wave 0 e desta REQ.

Considero a enumeração **fechada** para o que a REQ e o roadmap tocam: nenhum outro ponto de
criação ou decisão de link nos 3 CLIs além dos já mapeados pela REQ, mais os dois itens acima
(ambos informativos, nenhum bloqueante, nenhum no caminho da Wave 1).

---

## 2. Modelo de ameaça

**Quem esvazia esta Wave 0 sem quebrar regra escrita, e como?**

O ator relevante aqui não é externo — `workflow_dispatch` já exige permissão de escrita no
repositório (reafirmado da barreira anterior, `docs/seguranca/2026-08-30-barreira-do-instrumento-de-windows.md`,
Pergunta 1(a); a Wave 1 não muda essa análise porque não adiciona novo gatilho nem novo input
externo — só novos passos dentro do mesmo `workflow_dispatch` já existente). O ator que importa é
"um agente implementador que segue o roadmap ao pé da letra e comete um desvio pequeno" — é esse
quem esvazia a Wave 0 sem quebrar regra escrita nenhuma, porque a regra escrita (roadmap, REQ)
já cobre os desvios óbvios (AC6 explícito). O que ela não amarra sozinha:

### (a) A sonda pode ser induzida a criar link fora de `RUNNER_TEMP`/workspace?

**Já cria, tecnicamente — e é pré-existente, não novo desta Wave.** O fato que sustenta este achado
é de código, permanente, e não depende de qual caminho o SO resolve numa imagem específica de
runner: `probe.go` (`cmdLstatSymlink`, `cmdLstatJunction`) usa `os.MkdirTemp("", "trackfw-probe-*")`
— string vazia como diretório-pai — que **nunca referencia `$env:RUNNER_TEMP` em lugar nenhum do
código**. A alegação de contenção ("todo link fica dentro de `RUNNER_TEMP`/workspace") é, portanto,
**não verificável pela própria construção do código**, independentemente de qual diretório
`%TEMP%`/`GetTempPath()` resolve nesta ou naquela imagem de runner.

Quanto a *qual* diretório é esse na prática: no Windows, `os.MkdirTemp("", ...)` resolve via
`GetTempPath()`/`%TEMP%`. Não verifiquei isto ao vivo neste runner — é inferência a partir de relato de terceiro que mediu
`%TEMP%` em runner Windows hospedado do Actions para outro propósito (otimização de I/O, não
segurança: https://ichard26.github.io/blog/2025/03/faster-pip-ci-on-windows-d-drive/, "Speeding up
pip's Windows CI by setting TEMP to the D: drive") — não verificação minha, tratar como não
confirmado — mas **tipicamente**, `%TEMP%`/`%TMP%` no `windows-latest` apontam para o perfil do
usuário de serviço da VM (algo como `C:\Users\runneradmin\AppData\Local\Temp`), não para
`$env:RUNNER_TEMP` (tipicamente uma unidade separada, ex. `D:\a\_temp`). Se confirmado, são dois
diretórios diferentes, ambos dentro da mesma VM efêmera descartada ao fim do job — mas isso é o dado
secundário; o dado primário, e o único que não precisa de confirmação ao vivo, é que o código não
amarra a garantia a `RUNNER_TEMP` de forma alguma.

A consequência prática é **baixa, não nula**: não há segredo nem dado do projeto nesse diretório —
é vazio, criado e destruído pelo próprio probe (`defer os.RemoveAll(tmp)`) — mas a alegação textual
do Wave 1 ("Todo link criado fica dentro de `RUNNER_TEMP`/workspace", critério de aceite do
ML-1A) é **falsa pela letra**, quando aplicada ao braço Go que já está em produção. Se a Wave 1
escrever `probe.js`/`probe.py` por paridade com `probe.go` — `os.tmpdir()` em Node, sem prefixo,
segue a mesma cadeia de candidatos (`TEMP`, `TMP`, depois `USERPROFILE` só se as duas primeiras
estiverem ausentes); `tempfile.mkdtemp()` em Python idem (`TMPDIR`/`TEMP`/`TMP`, depois, **só no
Windows e só se nenhuma estiver setada**, cai para o diretório de usuário) — herdam a mesma
imprecisão, não um novo risco em espécie.

**Achado (precisão, não segurança):** o critério de aceite do ML-1A tal como escrito
("todo link fica dentro de `RUNNER_TEMP`/workspace") não é literalmente verificável pelo próprio
padrão que o desenho já usa. Três saídas para o `ares-tf` decidir dentro do ML-1A sem voltar à
Wave 0 — a terceira é a que prefiro, por ser a única que resolve a questão medindo em vez de
argumentando, mesma filosofia da sonda:
1. Corrigir a redação do critério para o que é realmente garantido: "todo link fica dentro do
   diretório temporário do processo (`%TEMP%`/`os.tmpdir()`/`tempfile.mkdtemp()`) ou de
   `$env:RUNNER_TEMP`, ambos efêmeros e privados a este runner" — e não pretender uma garantia mais
   forte do que o código entrega.
2. Fazer `probe.js`/`probe.py` (e, se tocado, `probe.go`) receberem o diretório-base explicitamente
   (`$env:RUNNER_TEMP\probe-work`, já passado a `probe.go` via `working-directory` em outros passos)
   em vez de usar o default do runtime, tornando a alegação literalmente verdadeira.
3. **Imprimir o diretório temporário resolvido ao lado de `$env:RUNNER_TEMP`** em cada pergunta que
   cria link (ex.: `fmt.Printf("tempdir=%s runner_temp=%s\n", tmp, os.Getenv("RUNNER_TEMP"))`) —
   transforma "%TEMP% é ou não é RUNNER_TEMP neste runner" de uma afirmação inferida de fontes de
   terceiros (como a que fiz acima) em um dado bruto do próprio dispatch, sem exigir decisão alguma
   agora sobre qual das duas primeiras saídas adotar.

Não bloqueio nenhuma — é uma decisão de precisão de linguagem/implementação que cabe ao ML-1A, não
um risco que precise voltar para cá.

### (b) `git update-index --cacheinfo` com caminho controlado — onde escreve?

O caminho (`mylink`) é **literal no workflow**, não construído a partir de `inputs.motivo` nem de
qualquer valor externo — confirmado por leitura de `windows-probe.yml:281-322` (reafirma a barreira
anterior, Pergunta 1(c)). A correção da AC1 (passar `120000,$blob,mylink` como string única em vez
de deixar a vírgula do PowerShell construir array) **não muda o que é escrito nem onde** — muda
apenas como o argumento chega ao processo `git`. Nenhuma superfície nova de escrita se abre aqui.
O que muda é a necessidade da **falsificação da AC2** (ver seção 3) — o risco real da correção da
pergunta 7 não é "escreve fora do lugar", é "parece corrigida sem provar que corrigiu".

### (c) A tabela comparativa nova passa a imprimir algo que hoje não vaza?

Não, pela mesma análise da barreira anterior (Pergunta 1(b)): os valores que a tabela vai agregar —
`isSymbolicLink()`/`isDirectory()`/`isFile()` brutos do Node, `islink()`/`st_mode`/`S_ISLNK()`/
`readlink()` (ou erro) do Python — são os mesmos tipos de dado que `probe.go` já imprime para o
braço Go (modo, bits, booleans). Nenhum deles é segredo, nenhum deriva de `inputs.motivo`, e os
caminhos envolvidos continuam sendo os mesmos diretórios efêmeros discutidos no item (a) — a tabela
A tabela muda o **formato** (agregação legível), não a **classe** de dado exposta. Único cuidado a nomear
para o ML-1A: se `readlink()`/`os.readlink()` do Python for chamado sobre um link **quebrado**
apontando para um caminho absoluto arbitrário, o valor impresso é o alvo do link, não um segredo —
mas convém que o alvo seja, como os demais, construído dentro do próprio probe (nunca de input
externo), o que já é o padrão que `probe.go` segue e que presumo mantido.

**Achado: nenhum bloqueante nesta subseção.**

---

## 3. Alvos de falsificação nas duas direções

Para cada superfície nova ou tocada pela Wave 1, o que quebra quando regride, e o que quebra quando
regride **ao contrário** (a sonda ganhar veredito, ou trocar valor cru por interpretação):

| Superfície | Regride (perde sinal) | Regride ao contrário (ganha o que não devia) |
|---|---|---|
| **Pergunta 7 — argumento do `cacheinfo`** | Fix cosmético: `git update-index` falha por outro motivo (ex.: blob mal formado, `-add` esquecido) mas o erro é engolido (`2>&1 \| Out-Null`, `-ErrorAction SilentlyContinue` sem log) — step "passa" sem checkout ter ocorrido. É literalmente a AC2 da REQ: **falsificação obrigatória** — a prova exigida não é "o step saiu verde", é "o conteúdo em disco após o checkout corresponde ao blob commitado" (ex.: `Get-Content mylink` bater com `target.txt`, ou o `LinkType`/`Attributes` mudarem de vazio para preenchido comparado a uma execução anterior sem o fix). | A pergunta passa a **decidir** se o resultado é "certo" (ex.: `if ($item.LinkType -ne "SymbolicLink") { exit 1 }`) — vira gate disfarçado, viola AC6/ADR. |
| **Junction em Node (`probe.js`)** | `lstatSync()` lançar exceção (ex.: permissão, caminho não resolvido) é capturado e o subcomando devolve silenciosamente "não medido" sem imprimir a exceção crua — perde-se exatamente o sinal que fez a Pergunta 2 do Go valiosa (o erro de criação do symlink sem Developer Mode também é dado, não ruído a esconder). | `probe.js` passa a comparar o resultado contra um valor esperado hardcoded (`if (info.isSymbolicLink() !== true) console.log("BUG")`) em vez de imprimir `isSymbolicLink()`/`isDirectory()`/`isFile()` crus — interpretação no lugar de medição, mesmo erro que o ADR nomeia como inviolável. |
| **Junction em Python (`probe.py`)** | `os.readlink()` sobre a junction lançar `OSError`/`NotADirectoryError` (comportamento plausível — junction não é link simbólico do ponto de vista POSIX-like da stdlib) e o subcomando tratar isso como falha do probe (`sys.exit(1)`) em vez de imprimir o erro como dado, quebrando o padrão "sonda não falha, sonda relata" que `probe.go` já segue (`cmdLstatJunction`: erro de criação é impresso, `return`, não `os.Exit(1)`). | `checks.py`/`probe.py` reaproveitarem a função de medição para alimentar um `assert` na suíte de regressão da camada 2 (`checks.py`, que **tem** veredito, ao contrário do probe) — confundir os dois arquivos é justamente o erro que o cabeçalho do YAML nomeia ("mesmo erro do gate de falsificação que não pegou o bypass de cerca"). Falsificação concreta: grep em `checks.py`/`checks.js` por qualquer chamada às novas funções de junction após a Wave 1 — não deveria haver nenhuma. |
| **Tabela comparativa final** | A tabela cita só um subconjunto dos 3 runtimes ou omite a coluna "junction" quando a medição falhou (silêncio em vez de célula "erro: <mensagem>") — reintroduz o problema original desta REQ (Node/Python nunca medidos) só que agora escondido dentro de uma tabela que parece completa. | A tabela imprime uma coluna de "veredito" (✅/❌ por célula) em vez de valor bruto — é a forma mais fácil de o AC6 ser violado sem que ninguém perceba, porque "tabela legível" soa exatamente como "cada célula diz se está certo". A tabela deve ser `runtime × pergunta = valor bruto`, nunca `runtime × pergunta = OK/FAIL`. |
| **`Todo link fica dentro de RUNNER_TEMP/workspace` (AC do Wave 1)** | N/A — não é uma medição, é uma garantia de contenção. | Ver seção 2(a) — não é "ganhar veredito", é uma alegação já imprecisa que a Wave 1 não deveria piorar herdando o mesmo padrão sem nomear a imprecisão. |

---

## 4. Residual declarado

O que este desenho aceita não cobrir, e por quê:

- **`%TEMP%` ≠ `$env:RUNNER_TEMP`, ambos aceitos.** Já detalhado na seção 2(a). Aceito porque ambos
  são efêmeros, privados ao runner, sem dado sensível — só a redação do critério de aceite precisa
  ser precisa sobre qual dos dois garante, não o comportamento em si.
- **O comportamento de `rmdirSync`/`rmdir()` (Node/Python) sobre uma junction vazia não está
  medido.** Esta Wave 0 não confirma nem descarta se `removeEmptyAncestors`-equivalente é seguro por
  acidente nos dois runtimes não-Go — ver correção à classificação na seção final. Fica como pergunta
  em aberto, não resolvida aqui nem decidida por leitura de código sozinha — mas **o custo marginal
  de medir é quase zero dentro da própria Wave 1**: ML-1A já vai criar uma junction em Node e em
  Python para as perguntas 3/4 (AC3/AC4 da REQ); a fixture já existe no momento certo, e um
  `rmdirSync(junction)`/`os.rmdir(junction)` a mais, com o resultado impresso cru, responderia isto
  sem precisar de um dispatch extra depois. Não é requisito desta Wave 0 nem amplia o escopo do
  ML-1A por si — é uma decisão de escopo do KG/`ares-tf`, não uma exigência deste documento.
- **libuv/CPython podem enxergar junction de forma diferente do Go — e é exatamente isso que a
  Wave 1 existe para medir, não para prever.** Não modelo aqui qual será o resultado (hipótese não
  decide nada, mesma frase da REQ) — só que, seja qual for o valor medido, ele deve ser impresso cru.
- **Verificação pós-merge, não pré-merge.** `workflow_dispatch` só roda a partir da branch default —
  a AC9 da REQ já registra isso como estruturalmente inverificável antes do merge. Este documento
  audita o **desenho**, não a **execução**; a tabela real só existe depois que o arquiteto disparar
  o workflow em `main`.
- **Runner hospedado padrão, sem Developer Mode, sem `core.symlinks` customizado, sem codepage
  alternativa** — mesmo resíduo já nomeado pelo ML-0A do roadmap anterior e reafirmado no rodapé do
  próprio `windows-probe.yml`. A Wave 1 não muda essa fronteira.
- **Este documento não versiona uma opinião sobre qual correção adotar** (`ModeSymlink|ModeIrregular`
  etc.) — decisão explicitamente fora de escopo desta REQ (Negative Scope, item 2) e desta Wave 0.
- **A checagem de que `checks.py`/`checks.js` (camada 2) não importam as novas funções de junction**
  não está automatizada por este documento — fica registrada como alvo de falsificação (seção 3) para
  o `ares-tf` verificar manualmente ou, melhor, para o `hefesto-tf`/barreira final grepar
  explicitamente antes do merge.

---

## Resposta à classificação das três guardas (pergunta do KG)

**Confirmo duas sem correção (1 e 3). A 2 preciso corrigir**: é verdade só para o Go — Node e Python
não têm o mesmo mecanismo de proteção, e a classificação "salva por acidente" aplicada aos três,
como a REQ e minha própria tabela acima insinuam, produziria o roadmap de correção errado nos outros
dois CLIs (a correção de "tornar intencional" pressupõe que existe algo a tornar intencional, e nos
outros dois não existe). Evidência lida diretamente no código, não relida da REQ:

1. **`internal/integrations/manager.go:702` `rejectSymlinks` — furada, confirmado.** Código lido:
   percorre `current := filename` até `root`, testando `info.Mode()&os.ModeSymlink != 0` a cada
   ancestral (`:701-717`). Junction tem `ModeSymlink=false` (medido pela sonda), então o loop
   completo passa por uma junction ancestral sem nunca recusar. Paridade confirmada em
   `npm/src/integrations/manager.js:44-73` (`assertNoSymlinks`, mesmo padrão `isSymbolicLink()`) e
   `pypi/trackfw/integrations/manager.py:81-95` (`_reject_symlinks`, mesmo padrão `S_ISLNK`) — a
   furada é simétrica nos 3 CLIs, não só no Go.
2. **`internal/integrations/manager.go:582` `removeEmptyAncestors` — salva por acidente, confirmado
   **só para o Go**. Código lido: `info.Mode()&os.ModeSymlink != 0` é testado **antes** de
   `!info.IsDir()` (`:590-596`). Para junction, o primeiro teste é falso (não entra no `return err`
   de symlink), mas o segundo (`!info.IsDir()`) é **verdadeiro** — porque `Lstat` de junction devolve
   `ModeDir=false` (medido) — e a função retorna `nil` **sem remover nada e sem continuar o loop**.
   O comentário de doc do próprio código (`"never removes or follows a symlink"`, `:578-579`)
   descreve a garantia pretendida, não o mecanismo real que a entrega no caso junction — é a
   característica de "salva pelo motivo errado" que fragiliza a garantia diante de qualquer refactor
   que reordene os dois `if`s.

   **Node e Python não têm esse freio — não é a mesma garantia com o mesmo motivo errado, é uma
   garantia diferente e mais fraca.** `npm/src/integrations/manager.js:419-427` (`cleanEmpty`):
   `if (!fs.existsSync(directory) || fs.lstatSync(directory).isSymbolicLink() ||
   fs.readdirSync(directory).length) return` — não há teste de `isDirectory()`. Para uma junction
   com o `readdirSync` bem-sucedido, a proteção depende inteiramente de o **alvo redirecionado** não
   estar vazio (`.length` truthy) — se `readdirSync` seguir a junction (não medido: se `lstatSync`
   já erra em não marcar junction como symlink, `readdirSync`/libuv podem perfeitamente segui-la
   listando o conteúdo do alvo) e o alvo redirecionado estiver **vazio**, o `if` não dispara e
   `fs.rmdirSync(directory)` roda sobre o ponto de junção — precisa de medição, não é dado por certo
   nenhum dos dois lados. `pypi/trackfw/integrations/manager.py:589-596` (`_remove_empty`): testa
   `S_ISLNK` (falso para junction) e chama `directory.rmdir()` direto, sem checar `IsDir()`/`S_ISDIR`
   antes — a única coisa que impede continuar subindo ancestrais removendo é o `except OSError:
   return` ao redor do `rmdir()`, contando com o SO recusar `rmdir()` sobre uma junction não-vazia
   (ou com sucesso silencioso removendo só o ponteiro, comportamento também não medido aqui). **Se
   `rmdir()` do Python tiver sucesso sobre uma junction "vazia" e devolver, o loop `directory =
   directory.parent` continua subindo e tentando remover o próximo ancestral** — diferente do Go,
   que para incondicionalmente no teste `IsDir()`. Isto não é achado de exploração confirmado (não
   testei em Windows real, é leitura de código, não medição), mas é a correção de classificação que
   o KG pediu: o remédio "tornar `IsDir()` intencional" (proposto para o Go) pressupõe um freio que
   Node e Python não têm — nos outros dois a correção pode precisar **adicionar** o equivalente de
   `IsDir()`/`S_ISDIR`, não só torná-lo explícito. Ver Residual — recomendo ao KG registrar isto como
   pergunta nova de sonda (`rmdir`/`rmdirSync` sobre uma junction vazia, Node e Python) antes de a
   REQ de correção decidir o remédio dos três CLIs como se fosse um só.
3. **`internal/generators/update.go:1869`, `:1894`, `internal/discover/discover.go:268` — furadas
   por outra via e não só no Windows, confirmado.** Código lido nas três funções
   (`discoverWorkflowPresent`, `refreshDiscoverGitHubActionsWorkflowIfPresent`, `writeCIWorkflow`):
   todas fazem `os.Lstat` **apenas no caminho final** (`dest`/dado de entrada), sem nenhum loop de
   ancestral. `os.Lstat` só deixa de seguir o **último componente** do caminho por contrato da
   syscall — todo componente intermediário é resolvido normalmente pelo kernel/SO antes de chegar
   ali, em qualquer plataforma. Um symlink (ou junction, no Windows) num diretório ancestral de
   `.github/workflows/trackfw-validate.yml` redireciona a escrita para fora do projeto sem que
   nenhuma das três funções perceba — confirmado idêntico nos 3 CLIs
   (`npm/src/commands/update.js:188-197`, `:212-224`; `npm/src/commands/discover.js:341-365`, mais a
   variante morta `writeCIWorkflowForce:593-615`; `pypi/trackfw/commands/update.py:174-224`;
   `pypi/trackfw/commands/discover.py:486-509`).

A classificação em três classes (em vez de "as nove/dezesseis guardas estão furadas") é a leitura
certa e continua sendo — mas a classe 2, tal como o KG a descreveu ("salva por acidente"), vale
**só para o Go**, não para os três CLIs simetricamente. Para o Go, o remédio é tornar a garantia
**intencional** (testar `IsDir()` explicitamente como critério positivo, não como efeito colateral
de ordem de `if`), não "consertar uma furada" que hoje não vaza dado nenhum. Para Node e Python, a
REQ de correção não pode presumir o mesmo remédio: **primeiro precisa medir** se `rmdirSync`/`rmdir()`
sobre uma junction se comporta como o freio acidental do Go ou não — se não se comportar, o remédio
lá é **adicionar** um teste equivalente a `IsDir()`, uma mudança de escopo maior que "explicitar o
que já existe". Tratar os três como a mesma classe teria produzido o roadmap de correção errado
exatamente nos dois CLIs onde a garantia não está demonstrada — o erro que o KG pediu para eu vigiar.

---

## Superfícies que o KG não enumerou (resumo para o relatório)

1. `internal/generators/update.go:2323` `copyPath` / `pypi/trackfw/commands/update.py:667`
   `_copy_path` — cegueira de junction no sandbox de `--dry-run`, classe DoS-local, não
   escrita/exfiltração (seção 1a).
2. `npm/src/commands/discover.js:593` `writeCIWorkflowForce` — código morto sem chamador, guard
   correto, nota de paridade para o dono do código, não achado de segurança (seção 1a).
3. `%TEMP%` (usado por `probe.go` hoje) ≠ `$env:RUNNER_TEMP` — imprecisão do critério de aceite do
   ML-1A tal como redigido, não risco novo (seção 2a e seção 3, última linha da tabela).

Nenhuma delas bloqueia a Wave 1. Recomendo ao `ares-tf`: (a) decidir entre as duas saídas da seção
2(a) para a redação/implementação da contenção de `RUNNER_TEMP`, (b) seguir o padrão
"erro é dado, não falha" de `probe.go` nos dois arquivos novos, (c) manter a tabela final como
`valor cru`, nunca `veredito`, e (d) não deixar `checks.py`/`checks.js` importarem as novas funções
de junction.
