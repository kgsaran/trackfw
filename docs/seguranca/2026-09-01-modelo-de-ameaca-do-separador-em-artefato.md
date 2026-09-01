# Modelo de Ameaça — caminho dentro de artefato versionado usa sempre barra

> Produzido por: `hades-tf` | Data: 2026-09-01
> REQ: `docs/req/REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado-usa-sempre-barra.md`
> ML: ML-0A, Wave 0 (bloqueante — nenhuma linha de implementação escrita aqui)

---

## Veredito antecipado

Os 2 pontos nomeados pela REQ (sync do `roadmap move` na REQ pareada; `.trackfw-log` via
`os.path.join(agent, basename)`) **estão certos, mas incompletos por composição, não por
subestimação de ordem de grandeza** como nas duas Waves 0 anteriores desta sessão. A população real
de "caminho de junção cujo resultado é escrito dentro de conteúdo versionado" é pequena e
concentrada em duas famílias — a sincronização `roadmap:`/`Roadmap:` do `roadmap move` (3 runtimes,
já nomeada) e o `log_basename` do `.trackfw-log` (só existe furado em **um** dos 3 runtimes: Python).
Encontrei **um terceiro ponto que a REQ não previu**, de natureza diferente dos dois primeiros: não é
uma escrita nova quebrada, é uma **leitura já existente que nunca teve tolerância nenhuma** — nem
para o caso descrito pela REQ, nem para o caso que ela mesma pede na AC3. Reproduzi ao vivo, com o
binário Go real, os três sintomas de produto que a REQ e o roadmap preveem no modelo de ameaça:
`trackfw validate` recusando uma referência válida, o board do `serve` desenhando um grafo com aresta
solta, e a métrica de cycle time silenciosamente descartando um roadmap. **Nenhum dos três tem hoje
qualquer tentativa de tolerância — a AC3 não é um refinamento de código existente, é trabalho
inteiramente novo**, o que muda o cálculo de risco da Seção 3: não há comportamento correto para
proteger de regressão, só superfície nova para não superdimensionar.

---

## 1. Completude da enumeração

### 1.0 — Prova ao vivo antes da enumeração

Build real (`go build ./cmd/trackfw`), fora da árvore do projeto, em
`/private/tmp/.../scratchpad/poc216/`. Simulei o artefato "sujo" da forma que a REQ descreve como
reproduzível localmente: escrevendo `\` à mão no valor, não dependendo de rodar em Windows de
verdade (`filepath.Join` do Go em macOS sempre produz `/`, então **não dá para gerar o defeito
rodando `roadmap move` neste SO** — só simulando o resultado que o Windows produziria).

**PoC A — `trackfw validate` recusa uma referência que existe de verdade:**
```
$ cat docs/req/REQ-poc.md
---
status: Open
date: 2026-09-01
roadmap: "docs\roadmaps\wip\ROADMAP-poc.md"
---
# REQ: PoC
Roadmap: `docs\roadmaps\wip\ROADMAP-poc.md`

$ ls docs/roadmaps/wip/
ROADMAP-poc.md                                    ← o arquivo EXISTE, no caminho certo

$ ./trackfw validate
✗ roadmap "ROADMAP-poc.md" is in wip but has no linked REQ
✗ req "REQ-poc.md" has no linked ADR
✗ roadmap "ROADMAP-poc.md" is in wip but has no acceptance criteria block
✗ req "REQ-poc.md" links to Roadmap "docs\\roadmaps\\wip\\ROADMAP-poc.md" which does not exist
Error: 4 violation(s) found
```
`referenceExists` (`internal/validator/validator.go`) chama `os.Stat(expandedRef)` com o valor cru do
frontmatter. Em Linux/macOS, `\` não é separador — é um caractere literal do nome de arquivo — então
`os.Stat` procura por um arquivo cujo nome contém `\` de verdade, não encontra, e a regra
`ref_targets_exist` marca a referência como quebrada, embora o roadmap esteja exatamente onde deveria
estar. Isto reproduz a segunda violação nomeada no diagnóstico do roadmap ("Linux não resolve — e o
arquivo vai para o git") sem precisar de máquina Windows nenhuma, exatamente como o roadmap pediu.

**PoC B — o grafo do `serve` perde a aresta silenciosamente:**
```
$ ./trackfw serve --port 14099 &
$ curl -s http://localhost:14099/api/chain | python3 -m json.tool
"nodes": [
  { "id": "docs/roadmaps/wip/ROADMAP-poc.md", ... }          ← node ID usa "/" (filepath.Walk local)
],
"edges": [
  { "from": "docs/req/REQ-poc.md", "to": "docs\\roadmaps\\wip\\ROADMAP-poc.md" }   ← valor cru do frontmatter
]
```
`api_chain.go` usa o valor cru do campo `roadmap:` como `To` da aresta e o caminho devolvido por
`filepath.Walk` (sempre `/` na máquina que roda `serve`) como `ID` do node. Os dois nunca vão bater
por igualdade de string quando o valor gravado tem `\` — a aresta fica órfã e o frontend (que casa
`edge.To` contra `node.id`) simplesmente não desenha o link entre REQ e Roadmap. Sem erro, sem log,
degradação silenciosa de uma feature de visualização.

**PoC C — cycle time descarta o roadmap sem avisar (raciocínio a partir do código, não execução —
ver nota abaixo):** `internal/metrics/metrics.go:Calculate` agrupa transições por
`map[string][]stateEntry` chaveado no `Basename` **exato** de cada linha do `.trackfw-log`
(`byName[t.Basename] = append(...)`). Se a mesma REQ em modo `by_agent` transita `backlog → wip` numa
máquina que grava `zeus/ROADMAP-x.md` e depois `wip → done` numa que grava `zeus\ROADMAP-x.md` (o bug
do Python, seção 1.2), `Calculate` enxerga **dois artefatos diferentes**, nenhum dos dois com o par
`startTs`+`doneTs` completo — o roadmap desaparece do cálculo de `cycleTimeMean` sem erro, sem
warning, sem entrada zerada visível. Não executei isto ao vivo (exigiria dois runtimes de fato
divergindo em uma sessão real de `.trackfw-log`), mas o comportamento do agrupamento por igualdade de
string está confirmado por leitura direta do código, linha citada.

### 1.1 — Varredura pelos primitivos de junção

```bash
grep -rn "filepath\.Join(" internal/ | grep -v _test.go | wc -l     # → 240
grep -rn "path\.join(" npm/src/ | grep -v test | wc -l               # → 233
grep -rn "os\.path\.join(" pypi/trackfw/ | grep -v test | wc -l      # → 310
```
A imensa maioria (>95%) é acesso a sistema de arquivos — abrir, criar diretório, montar caminho para
`os.Stat`/`os.ReadFile`/`os.WriteFile` — e está **fora de escopo por definição da REQ** ("Não
normalizar caminho de sistema de arquivos em uso interno"). Filtrei manualmente pelo critério da REQ:
o resultado do `Join` tem que acabar **serializado como texto dentro de um artefato versionado**
(frontmatter, `.trackfw-log`, JSON de manifesto/quarentena/provenance), não usado para abrir/criar
arquivo. Não existe grep de sintoma que isole isso automaticamente — precisei seguir cada `dst`/
`newPath`/`logBasename` até o ponto de escrita, exatamente o aviso que a REQ e o roadmap deixaram
("grep por sintoma encontra quem já trata o problema, não quem o ignora" se aplica aqui na forma
inversa: grep pelo primitivo encontra os 780 sites; só a leitura dirigida separa os ~6 que importam).

### 1.2 — Classificação por site

| # | Runtime : arquivo : linha | O que constrói | Onde o valor acaba | Classe |
|---|---|---|---|---|
| 1 | Go `internal/generators/roadmap.go:452` | `dst := filepath.Join(targetDir, filepath.Base(src))` | passado como `newRoadmapPath` para `syncREQReferences` → escrito no `roadmap:` do frontmatter e na linha `Roadmap:` do corpo da REQ pareada (`rewriteREQRoadmapRef`, `os.WriteFile` em `roadmap.go:838`) | **(a) — o ponto nomeado pela REQ, confirmado** |
| 2 | Node `npm/src/generators/roadmap.js:283` | `const dst = path.join(targetDir, basename)` | passado como `newRoadmapPath` para `syncReqReferences` → `fs.writeFileSync` em `roadmap.js:~424` via `rewriteReqRoadmapRef` | **(a) — mesmo ponto, paridade confirmada** |
| 3 | Python `pypi/trackfw/generators/roadmap.py:622` | `dst = os.path.join(target_dir, basename)` | retornado como `new_path` de `move_roadmap` → `sync_paired_req_references(new_path, cfg)` → `_rewrite_req_roadmap_ref` → `open(req_path, "w").write(new_content)` | **(a) — mesmo ponto, paridade confirmada** |
| 4 | Python `pypi/trackfw/generators/roadmap.py:611` | `log_basename = os.path.join(agent, basename)` (só no ramo `by_agent`) | `_append_transition_log` → linha do `.trackfw-log` | **(a) — o segundo ponto nomeado pela REQ, confirmado, e só existe no Python** — Go (`roadmap.go:467`) e Node (`roadmap.js:269`) já usam `agent + "/" + basename` / `` `${agent}/${basename}` `` explicitamente, não `Join` |
| 5 | Go `internal/validator/validator_thirdparty_provenance.go:142` | `provenanceKey, relErr := filepath.Rel(root, destination)` | usado como chave de busca em `prov.Entries[provenanceKey]`, comparado contra chaves gravadas em `.trackfw/thirdparty-provenance.json` (que são sempre `/`, ver linha abaixo) | **NOVO — (a) pelo lado da leitura**: não é a escrita que está errada, é que a leitura nunca normaliza o separador nativo do `filepath.Rel` antes de comparar contra um valor de conteúdo que é sempre gravado com `/` |

### 1.3 — Sites que constroem `dst`/`logBasename` e são classe (b)/(c) — verificados para não sobrar dúvida

| Runtime : arquivo | Site | Uso do resultado | Classe |
|---|---|---|---|
| Go `internal/generators/req.go:351` | `dst := filepath.Join(targetDir, filepath.Base(path))` | só `os.WriteFile(dst, ...)` e `fmt.Printf` em stdout — a REQ nunca referencia o próprio path no seu frontmatter | (b) |
| Node `npm/src/generators/req.js:253` | idem | idem | (b) |
| Python `pypi/trackfw/generators/req.py:357` | idem | idem | (b) |
| Go `req.go:337`/Node `req.js:242`/Python `req.py:348` | `logBasename`/`log_basename` para `MoveREQ`/`_cmd_move_req` (`.trackfw-log` de REQ) | já `agent + "/" + basename` / `` `${agent}/${basename}` `` / `f"{agent}/{...}"` — **nenhum usa `Join`** | (c) — padrão já correto, mesmo padrão que falta no ponto 4 |
| Go/Node/Python `discover.*: GenerateBootstrapLog`/`generateBootstrapLog`/`generate_bootstrap_log` | linha retroativa do `.trackfw-log` a partir de `done/` | `agent + "/" + e.Name()` / `agent + '/' + entry` / `f"{agent}/{fname}"` — todos explícitos, nenhum `Join` | (c) |
| Go `internal/integrations/render.go:821` `ResolveThirdPartySkillDestination` | `baseDir + "/thirdparty/" + slug + ".md"` — a própria chave gravada em `thirdparty-provenance.json` | concatenação explícita, nunca `filepath.Join` | (c) — é por isso que o ponto 5 é só do lado da leitura: a escrita já está certa |
| Go `internal/thirdparty/quarantine.go:81`, `provenance.go:77` | `filepath.Join(root, ".trackfw", ...)` | só para abrir/escrever o arquivo de quarentena/provenance no disco — o valor não entra em nenhum campo JSON | (b) |
| Go `internal/integrations/manager.go:668` `Manager.resolve` | `destination = filepath.Join(root, destination)` (caminho **absoluto**) | vira a **chave** de `manifest.Artifacts[destination]`, serializada em `.trackfw/integrations-manifest.json` | **(c) ambíguo — ver 3.3**: tecnicamente escreve em conteúdo, mas a chave já é não-portável por ser absoluta (difere por máquina independente de SO); tratar separador aqui não resolve portabilidade, só risca um contrato de domínio de chave já documentado e pinado em `docs/cli-parity.md` ("domínio de chave absoluto vs. relativo") |
| Go `internal/generators/agentfiles.go`, `scaffold.go` | comandos de hook (`"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh"` etc.) | strings literais constantes, nunca `filepath.Join`/concatenação de variável de path | (b)/não-vetor — nada aqui deriva de `Join` |

---

## 2. Modelo de ameaça

**Quem grava, com que capacidade:** qualquer contribuidor rodando `trackfw roadmap move` no Windows
(qualquer um dos 3 CLIs — o defeito é de composição, não de plataforma privilegiada; não exige
symlink, não exige merge de PR malicioso, é o fluxo normal e documentado do comando). O commit
resultante entra no histórico normalmente — não há checagem hoje que rejeite `\` num valor de
frontmatter.

**O que quebra, para quem, e quando chega ao Linux — 4 manifestações medidas ou derivadas
diretamente do código, não hipotéticas:**

1. **`trackfw validate` (`ref_targets_exist`)** — PoC A. Falso positivo: recusa uma referência que
   aponta para um arquivo que existe de verdade. Bloqueia CI/`make quality` para todo contribuidor
   que der checkout do commit Windows em Linux/macOS, mesmo sem tocar no arquivo.
2. **`validateREQRoadmapLifecycle`** (mesma função de leitura, `internal/validator/validator.go:2096`)
   — pior que o caso 1 porque falha **fechado silenciosamente**: `os.Stat(expandedRef)` retorna erro,
   o código faz `continue`, e a checagem "REQ Open mas Roadmap linkado está em done/" simplesmente
   nunca dispara para nenhuma REQ cuja referência esteja suja. Não é um erro visível — é uma checagem
   de ciclo de vida inteira desarmada sem aviso.
3. **Board do `serve` (`api_chain.go`)** — PoC B. Degradação de UX silenciosa: o grafo REQ↔Roadmap
   perde a aresta sem erro no console, sem log — só "o link não aparece".
4. **Métrica de cycle time (`internal/metrics/metrics.go:Calculate`)** — PoC C (por leitura de
   código). Um roadmap `by_agent` cuja história de transições mistura `agent/file.md` e
   `agent\file.md` (produzido pelo ponto 4 da tabela 1.2, hoje só no Python) é tratado como dois
   artefatos diferentes; nenhum dos dois fecha o par backlog→done; o roadmap desaparece do
   `cycleTimeMean` sem qualquer sinal.

**Quem esvazia a Wave 0 sem quebrar regra escrita:** implementar a AC1 (escrita sempre com `/`) só
nos 2 pontos já nomeados pela REQ fecha os pontos 1–4 da tabela 1.2, mas deixa o ponto 5
(`filepath.Rel` na leitura de provenance) e a AC3 inteira (tolerância na leitura de `validate`,
`serve`, `metrics`) intocados — a letra da REQ nomeia "escrita sempre com /" e "leitura tolerante"
como ACs separadas (AC1 e AC3); cumprir só AC1 fecha a produção de conteúdo sujo novo, mas não repara
o conteúdo sujo que **já está no histórico** de qualquer projeto que já rodou `roadmap move` no
Windows antes deste fix — exatamente o cenário que a AC3 existe para cobrir.

---

## 3. Falsificação nas duas direções

### 3.1 — O que quebra quando a tolerância de leitura **não existe** (direção já provada)

PoC A e PoC B, seção 1.0 — reproduzidas ao vivo com o binário real, sem precisar de Windows.
PoC C, derivada por leitura de código com linha citada.

### 3.2 — O que quebra quando a normalização de leitura **supergeneraliza** (a direção simétrica que
KG pediu para nomear com prioridade)

**Não existe hoje nenhuma tentativa de normalização na leitura** — `referenceExists`,
`api_chain.go`, `metrics.Calculate` e o lookup de `provenanceKey` chamam `os.Stat`/comparam string
crua, sem qualquer `ReplaceAll("\\", "/")` em lugar nenhum dos 3 runtimes. Isto muda o risco de regra
para o roadmap: **não há comportamento correto pré-existente para uma normalização agressiva
quebrar por regressão** — o risco inteiro está em **onde** a normalização nova é aplicada, não em
remover algo que já funcionava. Três formas concretas de superdimensionar, todas evitáveis por
escopo, não por não fazer o fix:

**(i) Normalização de arquivo inteiro (`\` → `/` em todo o conteúdo lido) corromperia prosa e blocos
de código legítimos.** Roadmaps e REQs deste próprio projeto citam caminhos Windows, regex e escapes
em blocos de código como conteúdo de exemplo — não são hipotéticos, são o padrão real desta sessão e
das anteriores (ex.: `os.path.join(agent, basename)` citado *dentro* de uma REQ, comandos `sed`,
regex `\d{4}-\d{2}-\d{2}` no próprio `metrics.go` citado acima). Um `strings.ReplaceAll` sobre o
`[]byte` inteiro de um arquivo antes de qualquer parsing trocaria backslashes de exemplos didáticos e
de regex citada em prosa, corrompendo o próprio conteúdo do artefato de forma irreversível na
próxima escrita. **A normalização tem que ser escopada ao valor já extraído de um campo específico**
(`roadmap:`, `req:`, `adr:` do frontmatter; o token de basename de uma linha do `.trackfw-log`; a
chave de um mapa JSON de path) — nunca ao buffer bruto do arquivo.

**(ii) `content_base64` da quarentena de terceiros não pode ser tocado por normalização nenhuma.**
`internal/thirdparty/quarantine.go` documenta explicitamente (comentário de `NewQuarantineEntry`)
que o conteúdo é embutido "verbatim" e que qualquer indireção reabriria a janela de TOCTOU que o
registro existe para fechar (D8b/D8c). Um normalizador genérico que percorresse todo valor de string
dentro de todo JSON de `.trackfw/` sem lista de campos permitida quebraria a âncora de checksum
(`ChecksumSHA256`/`InstalledSHA256`) do fluxo de aprovação de terceiros — não é hipotético, é o
mecanismo de segurança mais crítico deste repositório sendo atingido por um efeito colateral de uma
REQ que não tem nada a ver com ele. **Este é o limite mais duro a nomear**: a Wave 1 precisa de uma
lista fechada de campos elegíveis para normalização (frontmatter `roadmap:`/`req:`/`adr:`, linha do
`.trackfw-log`, e — decisão do arquiteto — a chave de `thirdparty-provenance.json`), nunca uma
varredura genérica de string.

**(iii) A chave absoluta de `integrations-manifest.json` não deve ganhar normalização de separador
sob a bandeira desta REQ.** Já documentado como decisão pinada em `docs/cli-parity.md` ("domínio de
chave absoluto vs. relativo"): o manifesto usa destino absoluto como chave *deliberadamente*, e
`thirdparty-provenance.json` usa relativo *deliberadamente*, com uma nota de paridade já escrita para
essa divergência. Uma chave absoluta nunca é portável entre máquinas de qualquer forma (o prefixo do
usuário muda), então "consertar" o separador ali não entrega a propriedade que a REQ busca
(portabilidade real) — só corre o risco de tocar um contrato já pinado sem necessidade. Nomeio como
observação para o arquiteto decidir, não como item da Wave 1.

### 3.3 — O que NÃO pode ser normalizado (nomeado explicitamente)

- **O `content_base64` e qualquer campo coberto por checksum** (item 3.2-ii) — nunca.
- **Prosa e blocos de código dentro do corpo de ADR/REQ/Roadmap** — a normalização é por campo
  extraído, nunca por arquivo inteiro (item 3.2-i).
- **A chave absoluta de `integrations-manifest.json`** — fora do escopo desta REQ por já ser
  não-portável por design; tocar exigiria reabrir a decisão pinada de domínio de chave, não é um
  ajuste de separador trivial (item 3.2-iii).
- **Um nome de arquivo que legitimamente contenha `\` como caractere** (válido em nomes de arquivo
  Unix, embora não em Windows). Verificado que isto não ocorre com nada que o próprio trackfw gera:
  `toSlug` (`internal/generators/adr.go:151`, mesma função usada por `req`/`roadmap`/`note`)
  substitui toda sequência de não-`[a-z0-9]` por hífen — nenhum basename produzido pelos geradores do
  trackfw pode conter `\`. O risco é teórico (alguém renomeia manualmente um arquivo com `\` no
  nome, algo que só funciona em Unix) e não medido; registrado como residual, não como bloqueio.

---

## 4. Residual declarado

1. **Ponto 5 (leitura de `provenanceKey` via `filepath.Rel`) é Go-específico** — a regra
   `thirdparty_artifact_has_provenance` não está implementada em Node nem Python (gap de paridade já
   documentado e intencional em `docs/cli-parity.md`, "detecção ancorada em git"). Não há divergência
   cross-runtime a corrigir aqui, só o comportamento do Go isoladamente; decisão do arquiteto se entra
   na Wave 1 desta REQ ou vira achado separado, dado que toca um arquivo fora da lista original da REQ.
2. **PoC C não foi executado ao vivo** — depende de fazer duas máquinas (ou dois runtimes)
   divergirem numa mesma sequência de transições do `.trackfw-log`, o que exigiria simular o bug do
   Python (ponto 4) e depois rodar `trackfw metrics` sobre o log resultante. A conclusão vem de leitura
   direta do agrupamento por string exata em `Calculate`, não de execução — mais fraco que os PoCs A/B,
   mas o mecanismo (`map[string][]stateEntry` chaveado em `Basename` cru) é inequívoco por leitura.
3. **Não tentei enumerar exaustivamente os 240+233+310 sites brutos de `Join`** — segui cada `dst`/
   `logBasename`/chave de mapa que aparece perto de escrita de conteúdo versionado (frontmatter,
   `.trackfw-log`, JSON de `.trackfw/`), não cada ocorrência isolada. A Wave 1 deve rodar o mesmo
   filtro de novo ao particionar os MLs, não confiar cegamente nesta lista — mesmo aviso que a REQ e o
   roadmap já deixaram sobre a lista deles.
4. **PoCs rodados só em macOS** — a produção real do `\` só acontece em Windows de verdade; os PoCs A e
   B simulam o resultado à mão (como o roadmap autorizou explicitamente), não reproduzem o comando
   `roadmap move` rodando nativamente em Windows. Isto é suficiente para provar o *efeito* do lado da
   leitura (o que este documento cobre), mas não substitui o gate de CI real em runner Windows que a
   AC5/Wave 2 do roadmap já exige para provar o lado da *escrita*.
5. **Não investiguei `references.json` (`thirdparty-references.json`)** — é chaveado por id de
   agente-alvo, não por caminho, então a priori fora da classe desta REQ; não confirmei por leitura
   linha a linha, só por não ter aparecido em nenhuma das buscas direcionadas.
6. **`api_metrics.go`/`api_metrics.js`/`api_metrics.py` (métricas do board) não foram auditados com o
   mesmo nível de profundidade que `metrics.go`** — assumido equivalente por usar o mesmo `.trackfw-log`
   e regex de formato compatível (`internal/serve/metrics_log.go` cita o mesmo formato de linha), mas
   não confirmado byte a byte contra Node/Python.

---

## Resumo para quem só vai ler o veredito

- **Confirma os 2 pontos da REQ** nos 3 runtimes (achado 1–3 e 4 da tabela 1.2) — a REQ acertou no
  alvo principal, e o ponto do `.trackfw-log` (achado 4) já está corrigido "de graça" em Go e Node por
  um padrão de concatenação explícita que o Python não segue.
- **Achado novo, não previsto pela REQ:** `internal/validator/validator_thirdparty_provenance.go:142`
  usa `filepath.Rel` para gerar uma chave de busca contra um JSON de conteúdo que é sempre gravado com
  `/` — é um bug do lado da **leitura**, não da escrita, e é exatamente o tipo de coisa que a AC3
  (leitura tolerante) deveria cobrir, mas hoje ninguém tenta.
- **3 sintomas de produto reproduzidos** (2 ao vivo, 1 por leitura de código): `validate` recusando
  referência válida, board perdendo aresta do grafo, métrica de cycle time descartando um roadmap
  silenciosamente — os 3 já estão nomeados no modelo de ameaça do roadmap ("validate resolvendo
  referências, board do serve, .trackfw-log"), esta seção só entrega a evidência viva que faltava.
- **O limite mais duro para a Wave 1:** normalização tem que ser por campo extraído (lista fechada),
  nunca por arquivo inteiro nem por varredura genérica de JSON — o `content_base64` da quarentena de
  terceiros é o caso onde isso mais importa, porque é literalmente o mecanismo anti-TOCTOU do fluxo de
  aprovação de terceiros.
