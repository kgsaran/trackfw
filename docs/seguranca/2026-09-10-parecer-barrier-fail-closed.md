# Parecer de Segurança — `roadmapTrustForGates` fail-open → fail-closed

**Data:** 2026-09-10
**Revisor:** Hades (Security Reviewer)
**Branch auditada:** `fix/barrier-executa-gate-de-roadmap-nao-confiavel` (commit `a6205aa`)
**Arquivos auditados:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`,
`pypi/trackfw/commands/barrier.py`, `scripts/check-barrier.sh`,
`docs/req/REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-*.md`
**Premissa do revisor independente:** raciocínio construído a partir da leitura do código, sem
consultar o diff — premissas compartilhadas não passam pela barreira.

---

## Veredicto: APROVA COM RESSALVAS

A inversão eliminou todos os caminhos fail-open dentro de `roadmapTrustForGates` /
`_roadmap_trust_for_gates`. Cada função tem exatamente um `return trusted: true`, em posição de
saída feliz após os 7 passos de verificação. Nenhum dos achados abaixo cria um caminho exploitável
dentro do threat model declarado (adversário = revisora de PR que clona repositório externo). Os
achados são assimetrias estruturais que aumentam a superfície de regressão futura.

---

## Vetor 1 — Exatamente um `trusted: true` dentro da função de confiança?

**Confirmado: exatamente um em cada CLI.**

| CLI | Localização | Linha |
|-----|-------------|-------|
| Go | `return gatesTrustVerdict{trusted: true}` | `barrier.go:759` |
| Node.js | `return { trusted: true }` | `barrier.js:585` |
| Python | `return {"trusted": True}` | `barrier.py:607` |

Todos os caminhos de erro dos 7 passos retornam `trusted: false` com mensagem nomeada. Nenhuma
bifurcação condicional produz um segundo `trusted: true` dentro do corpo da função. Os outros
literais `{ trusted: true }` que aparecem nos arquivos estão fora da função de confiança (ternária
`--trust-local-gates` e parâmetro default de `evalGates`) — ver Achados F2 e F3.

Nenhum passo usa `strings.Contains` / regex de substring em stderr do git — todos os discriminantes
são exit codes de `git rev-parse --verify` e `git cat-file -e` (AC3 atendido).

---

## Vetor 2 — 35 ocorrências de `--trust-local-gates` em check-barrier.sh são legítimas?

**Veredicto: LIMPO.**

`check-barrier.sh` usa dois builders de fixture:

- `common_dirs()` (linha 106): cria apenas a estrutura de diretórios, sem `git init`. Usado pelos
  cenários S1–S13. Nesses cenários, `git rev-parse --git-dir` falha (não é um repositório git) →
  `trusted: false`. O flag é obrigatório para que o gate execute — os cenários testam comportamentos
  ortogonais ao mecanismo de confiança.
- `make_barrier_git_fixture()` (linha 948): cria clone com origin bare, mas commita apenas
  `trackfw.yaml` no base commit — o roadmap fica não-rastreado. Usado por S14 e S15. Idem:
  `git cat-file -e refs/remotes/origin/main:<relPath>` falha (roadmap ausente da origin) → o flag é
  necessário.

Os cenários S14–S18 testam o mecanismo de confiança em si: S14 sem flag → sentinel NÃO criado
(fail-closed confirmado); S15 com flag → sentinel criado (vacuidade confirmada); S16 usa
`commit_roadmap_to_origin` e executa SEM `--trust-local-gates` → sentinel criado, `gates.status:
passed` (caminho positivo confirmado); S17 adiciona byte local → sentinel NÃO criado,
`content differs` (caminho negativo confirmado).

Todas as 35 ocorrências são legitimamente necessárias. Nenhuma mascara falha de confiança.

---

## Vetor 3 — `TestRoadmapTrustForGates_TrustedCountIsOne` (Go only) é gap de paridade?

**Veredicto: GAP PRESENTE — ver Achado F3.**

---

## Vetor 4 — Prova de identidade byte-a-byte: TOCTOU entre prova e execução?

**Veredicto: INVARIANTE QUEBRADO por construção — ver Achado F1.**

---

## Vetor 5 — Chamadas git novas aceitam input controlado? Path chega ao shell?

**Veredicto: LIMPO.**

Todas as chamadas git nos 3 CLIs usam execução baseada em array de argumentos:
- Go: `exec.Command("git", "rev-parse", "--git-dir")` com `Dir = roadmapDir`
- Node.js: `spawnSync("git", ["rev-parse", "--git-dir"], { cwd: roadmapDir })`
- Python: `subprocess.run(["git", "rev-parse", "--git-dir"], cwd=roadmap_dir)`

O path do roadmap chega como `Dir`/`cwd` (diretório de trabalho do processo), não como argumento
expandido por shell. O `relPath` derivado é passado como elemento único de array (`git cat-file -e
refs/remotes/origin/main:relPath`). Newlines no path causariam falha de lookup do git (exit nonzero
→ fail-closed). Nenhum caminho chega a um shell via interpolação.

---

## Achados

### F1 — Invariante quebrado: prova cobre B1, execução deriva de B0 [MÉDIO]

**Localização:**
- Go: primeira leitura `barrier.go:839` (`os.ReadFile`), segunda leitura `barrier.go:744`
  (`os.ReadFile` dentro de `roadmapTrustForGates`)
- Node.js: primeira leitura `barrier.js:716` (`fs.readFileSync`), segunda leitura `barrier.js:570`
  (`fs.readFileSync` dentro de `roadmapTrustForGates`)
- Python: primeira leitura `barrier.py:755` (`open(..., "r")`), segunda leitura `barrier.py:592`
  (`open(roadmap_path, "rb")` dentro de `_roadmap_trust_for_gates`)

**Comportamento observado:** os comandos de gate são parseados do buffer B0 (primeira leitura). A
prova de confiança compara o buffer B1 (segunda leitura) contra `origin/main`. O que é provado é uma
propriedade de B1; o que executa é derivado de B0. Isso não é uma condição de corrida — a prova
simplesmente não cobre o payload por construção.

**Vetor concreto:** um atacante com acesso de escrita local ao arquivo do roadmap posiciona comandos
de gate maliciosos em B0, aguarda que o barrier faça a primeira leitura (parseando os gates
maliciosos), então substitui o arquivo pela versão limpa (idêntica a `origin/main`) antes da segunda
leitura. A prova de confiança passa (B1 = origin/main), os gates executam com o conteúdo de B0.

**Exploitabilidade no threat model declarado:** BAIXA. O adversário declarado é a revisora de PR em
repositório externo clonado — sem acesso de escrita concorrente ao sistema de arquivos do revisor.
Fora do threat model declarado mas exploitável com acesso local.

**Nota:** a Regra Dura de Reconciliação deste projeto captura exatamente este padrão: o comentário
no código afirma `"PROVE that the roadmap is present in refs/remotes/origin/main byte-for-byte"` —
a prova é feita, mas sobre B1, não sobre B0 que executa.

**Fix:** passar o buffer já lido para `roadmapTrustForGates` em vez de relê-lo. A função retorna o
buffer verificado e os gates são parseados desse buffer. Uma linha de plumbing.
**Dono:** `apolo-tf`

---

### F2 — Defaults latentes de fail-open no consumidor (Node.js, Python) [BAIXO]

**Localização:**
- Python `barrier.py:626`: `if trust_result is None: trust_result = {"trusted": True}` —
  `None` é tratado como confiável
- Python `barrier.py:631`: `if not trust_result.get("trusted", True):` — dict sem chave `"trusted"`
  é tratado como confiável (default `True`)
- Node.js `barrier.js:595`: `function evalGates(commands, cwd, trustResult = { trusted: true })` —
  argumento omitido → gates executam

**Go:** nenhum default equivalente existe. A estrutura `gatesTrustVerdict` tem `trusted: false` como
zero-value, e o branch de execução em `runBarrier` não tem default.

**Exploitabilidade atual:** NULA. Todos os call sites passam o terceiro argumento explicitamente
(Node.js `barrier.js:727`; Python `barrier.py:766`). `_roadmap_trust_for_gates` sempre retorna um
dict com a chave `"trusted"` e nunca retorna `None`. Nenhum caminho atual atinge os defaults.

**Risco residual:** qualquer refatoração futura que adicione um call site omitindo o argumento (Node)
ou que retorne `None`/dict sem chave (Python) introduziria fail-open silencioso, assimétrico ao Go.
O teste estrutural do Go (Achado F3) não detecta esse padrão.

**Fix:** Go deve ser o modelo. Python: remover o `None`-guard e mudar o default de `get` para
`False`. Node.js: remover o default parameter ou torná-lo `{ trusted: false }`.
**Dono:** `apolo-tf`

---

### F3 — Guarda estrutural de contagem existe somente em Go [BAIXO]

**Localização:** `internal/commands/barrier_test.go:1145-1160`
(`TestRoadmapTrustForGates_TrustedCountIsOne`)

**Node.js e Python:** sem teste equivalente. O teste Go usa grep de literal `return
gatesTrustVerdict{trusted: true}` no texto fonte e afirma count == 1. Em Node.js, o literal
`{ trusted: true }` aparece 3 vezes no arquivo (linha 585 dentro da função, linha 595 como
parâmetro default, linha 726 como resultado de ternária de `--trust-local-gates`) — o mesmo grep
portado ingenuamente falharia com count == 3, não 1.

**Limitação do controle existente no Go:** o grep não captura construção via variável, inversão do
campo, ou remoção de return que faz o controle cair no próximo bloco. O mesmo padrão de
bypass-por-equivalência-sintática documentado no vault se aplica aqui.

**Recomendação:** em vez de clonar o grep para Node/Python, implementar guarda comportamental
(fixture por mensagem nomeada de cada razão de not_evaluated, assertando ausência de sentinel). Isso
fecha o risco em todos os CLIs e não propaga um controle fraco. Manter o teste Go como camada extra,
não como único mecanismo.
**Dono:** `apolo-tf`

---

### F4 — Node.js compara strings UTF-8, não bytes [INFORMACIONAL]

**Localização:** `barrier.js:570-577`

```js
const show = spawnSync("git", ["show", refPath], { encoding: "utf8", ... })
// ...
const localContent = fs.readFileSync(roadmapPath, "utf8")
if (show.stdout !== localContent) { /* content differs */ }
```

Go e Python fazem comparação binária (Go: `string([]byte)` é conversão zero-copy que preserva todos
os bytes; Python: `subprocess.run` sem `text=True` → `r.stdout` é bytes; `open(roadmap_path, "rb")`
→ bytes). Node.js decodifica ambos os lados como UTF-8 antes de comparar.

**Exploit path conhecido:** nenhum. Bytes inválidos em UTF-8 são substituídos por U+FFFD em ambos
os lados de forma simétrica. O payload de gate é ASCII e decodifica fielmente. Não é possível
contrabandear `touch /tmp/pwned` por uma colisão de caractere de substituição.

**Divergência declarável:** a garantia "byte-for-byte" documentada no comentário do código
(`barrier.js:574`) não é cumprida para arquivos com bytes inválidos em UTF-8. É uma divergência de
paridade, sem vetor concreto para roadmaps Markdown válidos em UTF-8.
**Dono:** `apolo-tf` (se decidirem corrigir para paridade completa)

---

### F5 — Python conflation de FileNotFoundError: instância 9 do padrão vault [INFORMACIONAL]

**Localização:** `barrier.py:504-520` (Step 1 de `_roadmap_trust_for_gates`)

```python
except FileNotFoundError:
    return {"trusted": False, "failureMsg": "not a git repository"}
```

Quando o binário `git` não está no PATH, Python lança `FileNotFoundError` na chamada a
`subprocess.run`. O código captura isso e retorna a mensagem "not a git repository" — que é
tecnicamente incorreta (o problema é o binário ausente, não a ausência de repo).

**Resultado de segurança:** correto (fail-closed). Mas a mensagem viola a regra documentada em
`vault/notes/guarda-que-reporta-ausencia-*.md`: "cada resposta tem mensagem própria" — "não achei"
e "não consegui procurar" devem produzir mensagens distintas.

**Relação com vault:** a nota documenta que a ausência de separação "pode levar a diagnósticos
falsos e a operadores ignorando erros reais." O passo 5 também sofre do mesmo padrão: `git cat-file
-e` pode falhar por (a) roadmap genuinamente ausente de origin/main, (b) `relPath` computado errado
por divergência de symlink, ou (c) path fora do repositório — todas as três rotas produzem a mesma
mensagem "roadmap is not committed in origin/main." Esta é a instância 9 do padrão, ironicamente
dentro da correção que fechou a instância 8.

**Go e Node.js:** tratam spawn failure via `err != nil` / `|| revParse.error` com mensagens
distintas para spawn vs. exit-nonzero. Melhor que Python, mas nenhum separa os três estados do
passo 5.
**Dono:** `apolo-tf` (opcional, sem impacto de segurança)

---

### F6 — Comentário stale em check-barrier.sh descreve comportamento pré-fix [DOCUMENTAÇÃO]

**Localização:** `scripts/check-barrier.sh:940-941`

```
# → barrier fails-open (trusted). Fix: resolve $WORK to its physical path (WORK_PHYS)
```

O comentário descreve o comportamento do código anterior à inversão. No código atual, a divergência
de symlink (`filepath.Abs` vs `--show-toplevel`) produz `relPath` errado → `git cat-file -e` falha
→ `trusted: false` (fail-CLOSED, não fail-open). O script corretamente usa `WORK_PHYS=$(cd "$WORK"
&& pwd -P)` como workaround, mas o comentário que descreve a consequência de não fazer isso está
desatualizado.

O comentário de residual declarado no código (`barrier.go` e equivalentes) menciona apenas o caso
CRLF. O caso de symlink (usuários macOS com `$TMPDIR` simbólico e roadmap fora de `$HOME`) produz
`not_evaluated` sem mensagem de orientação adequada. Não é um defeito de segurança mas é debt de
documentação.
**Dono:** `apolo-tf` (atualização de comentário)

---

## Superfícies confirmadas limpas

| Superfície | Veredicto | Evidência |
|------------|-----------|-----------|
| Parsing de stderr do git | LIMPO | Exit codes somente em todos os 7 passos dos 3 CLIs |
| Injeção de shell via path | LIMPO | Arrays de argumento; `roadmapPath` só vai como `cwd:` |
| Caminhos fail-open na função de confiança | LIMPO | 1 retorno `trusted: true` por função; todos os demais são `false` com mensagem |
| `--trust-local-gates` mascarando falha | LIMPO | 35 usos todos em fixtures sem git repo ou sem roadmap commitado |
| Janela `git cat-file → git show` | LIMPO | Objetos git são imutáveis; conteúdo de origem/main não pode ser trocado entre os dois comandos |

---

## Prioridade de handoff para apolo-tf

| # | Achado | Severidade | Fix |
|---|--------|------------|-----|
| F1 | Invariante quebrado: prova ≠ payload | Médio | Passar bytes já lidos para `roadmapTrustForGates` (1 linha por CLI) |
| F2 | Defaults latentes fail-open no consumidor | Baixo | Remover None-guard Python, mudar defaults para `false` |
| F3 | Guarda estrutural Go-only | Baixo | Guarda comportamental nos 3 CLIs (fixtures por mensagem nomeada) |
| F4 | Node.js: strings UTF-8, não bytes | Informacional | Usar `Buffer` em vez de `encoding: 'utf8'` no spawnSync |
| F5 | Python FileNotFoundError conflation | Informacional | Captura separada de `FileNotFoundError` com mensagem "git not found in PATH" |
| F6 | Comentário stale no script | Documentação | Atualizar linha 941 do check-barrier.sh |

---

## Verificação de Fechamento — 2026-09-10

**Revisor:** Hades (Security Reviewer)
**Método:** leitura direta do código nas 3 implementações + análise de falsificabilidade dos testes.
**Premissa:** premissas compartilhadas não passam pela barreira — nenhum relatório do apolo-tf foi consultado.

---

### F1 — Invariante de buffer: FECHADO (com gap de cobertura de teste em Node/Python)

**Evidência de fechamento:**

Go (`barrier.go`): `data` lido uma vez em linha 863 via `os.ReadFile`. Gates parseados de
`string(data)` via `lines`. Chamada a `roadmapTrustForGates(roadmapPath, data)` em linha 964 passa
esse mesmo buffer. Dentro da função, Step 7 compara `string(mainContent) != string(localContent)`
onde `localContent` é o parâmetro recebido — sem `os.ReadFile` adicional. Contagem de `os.ReadFile`
em `barrier.go` passou de 2 para 1 (a segunda era a releitura interna, agora eliminada).

Node (`barrier.js`): `contentBuf = fs.readFileSync(resolved.path)` em linha 740 (sem encoding, tipo
`Buffer`). Gates parseados de `contentBuf.toString('utf8')`. Chamada a
`roadmapTrustForGates(resolved.path, contentBuf)` em linha 753 passa o mesmo `Buffer`. Step 7 usa
`localContentBuf.equals(show.stdout)` — comparação de Buffer, sem releitura.

Python (`barrier.py`): `raw_bytes = open(roadmap_path, "rb").read()` em linha 774. Gates parseados
de `raw_bytes.decode("utf-8")`. Chamada a `_roadmap_trust_for_gates(roadmap_path, raw_bytes)` em
linha 787 passa o mesmo buffer. Step 7 usa `main_content != local_content` — comparação de bytes,
sem releitura.

**Path `cannot read local roadmap file` era alcançável?** Sim — era a releitura interna que falhava
se o arquivo fosse deletado entre a primeira leitura do caller e a segunda leitura da função. Era uma
condição de corrida, não um defeito lógico. A remoção é correta: se o caller não consegue ler (linha
865 Go / equivalentes), ele sai via `usageExit` antes de chamar a função de confiança. A função
nunca mais lê o disco.

**Teste `TestRoadmapTrustForGates_VerifiesPassedBuffer` é falsificável?** Sim. O teste
(`barrier_test.go:1185`) passa `[]byte("# Roadmap: altered-in-memory\n")` como buffer enquanto o
disco e origin/main têm `"# Roadmap: clean\n"`. Se alguém reintroduzir `os.ReadFile(roadmapPath)`
dentro da função (ignorando o parâmetro), a função leria o conteúdo limpo do disco, compararia
contra origin/main (igual), e retornaria `trusted=true`. O teste espera `trusted=false` — quebraria.
O teste captura a regressão.

**Gap residual de cobertura — dois meios, dois remédios distintos:**

*Metade callee:* `VerifiesPassedBuffer` (Go) prova que a função honra o parâmetro — se alguém
reintroduzir `os.ReadFile(roadmapPath)` internamente, o teste quebra. Node e Python não têm
equivalente. Os testes F3 de "content-differs" não isolam o invariante parâmetro-vs-disco: a fixture
faz com que arquivo local E parâmetro difiram de origin/main ao mesmo tempo — se uma segunda leitura
for reintroduzida, a função ainda compara o arquivo local (correto) e continua bloqueando. Nenhum
teste detectaria a regressão em Node/Python. Remédio: portar `VerifiesPassedBuffer` para Node e Python.

*Metade caller:* `VerifiesPassedBuffer` prova o callee, não o caller. O invariante do caller
("o buffer passado ao verificador é o mesmo que o parser de gates consumiu") não é testável por teste
comportamental determinístico: se o caller fizesse duas leituras sem escritor concorrente, ambas
retornam bytes idênticos e nenhuma fixture consegue distinguir. Esse lado é fechável apenas
estruturalmente — um guard de leitura única sobre o read-path do caller, verificado via análise
estática ou shell-ban (análogo ao `check-raw-read-ban.sh`). Remédio: análise estática ou script de
ban de `readFile`/`open` duplicado no caller. Ausente hoje nos 3 CLIs.

---

### F2 — Defaults latentes fail-open: PARCIAL

**Get defaults: FECHADO nos 3 CLIs.**

- Go: zero-value de `gatesTrustVerdict.trusted` é `false`. Sem default implícito.
- Node: `evalGates(commands, cwd, trustResult = { trusted: false })` em linha 615 — default mudou
  de `{ trusted: true }` para `{ trusted: false }`.
- Python: `trust_result.get("trusted", False)` em linha 645 — default mudou de `True` para `False`.

**None-guard em Python: PARCIAL.**

Código atual (`barrier.py:640`): `if trust_result is None: trust_result = {}`. O docstring
(`barrier.py:636`) promete "produces not_evaluated with failureMsg=None — fail-closed". O que o
código entrega: `trust_result = {}` → `{}.get("trusted", False)` = `False` → entra no ramo
not-trusted → tenta `trust_result["failure_msg"]` → `KeyError: 'failure_msg'`.

Resultado: crash de `KeyError`, não `not_evaluated` limpo. A promessa do docstring não é cumprida.

**Impacto de segurança: NULO.** A exceção propaga para cima de `_build_result_document` e a
barreira termina com erro, sem executar gates. É fail-closed. O crash só é atingível se alguém
chamar `_check_gates(commands)` sem o segundo argumento — o call site de produção em linha 792
sempre passa `trust_result` explicitamente.

**Node.js — análogo: `[null]` em vez de KeyError.**

`evalGates` default: `{ trusted: false }` (sem chave `failureMsg`). Linha 627:
`failures: [trustResult.failureMsg]` → `undefined` → JSON serializa como `[null]` em vez de string.
O call site de produção (linha 758) sempre passa `trustResult` explícito de `roadmapTrustForGates`,
que sempre inclui `failureMsg`. Path morto — nunca atingível em produção. `grep` em Node/Python tests
confirmou: nenhum teste chama `evalGates`/`_check_gates` com o argumento omitido.

**Impacto de segurança: NULO nos dois casos.** Python crashe fail-closed; Node emite `[null]` em
campo de failures informacional (não altera o `status`). Ambos são caminhos mortos em produção.

**Classificação: PARCIAL** — o fail-open está fechado, mas os comportamentos declarados nos docstrings
divergem do código em Python e Node. Manter como dívida técnica, sem impacto de segurança.

---

### F3 — Guardas comportamentais nos 3 CLIs: FECHADO para as razões cobertas

**Go:** 7 funções `TestRoadmapTrustForGates_*` confirmadas via `grep -c` (não contagem estimada):
`NotGitRepo`, `NoRemoteOrigin`, `NotCommittedInOrigin`, `IdenticalToOriginMain`, `ContentDiffers`,
`TrustedCountIsOne`, `VerifiesPassedBuffer`. Executadas com `go test -run TestRoadmapTrustForGates
-v`: todas PASS.

**Node.js:** 3 testes sentinel confirmados executados: `node --test npm/tests/barrier.test.js` —
`F3 sentinel: not a git repository`, `roadmap not committed in origin/main`, `local content differs`.
Todos PASS. Cada um verifica `gates.status === 'not_evaluated'` AND sentinelPath ausente.

**Python:** 3 testes sentinel confirmados executados: `pytest -k "sentinel"` —
`test_f3_sentinel_not_git_repository_prevents_gate_execution`,
`test_f3_sentinel_roadmap_not_committed_in_origin_prevents_gate_execution`,
`test_f3_sentinel_content_differs_from_origin_prevents_gate_execution`. Todos PASS.

**Gap herdado de F1:** os testes de "content-differs" (Node e Python) provam que um arquivo local
modificado não executa gates — não provam o invariante parâmetro-vs-disco (VerifiesPassedBuffer).
Esse gap é de cobertura de F1, não de F3.

---

### F4 — Comparação binária em Node.js: FECHADO

**Evidência:** `barrier.js:740` — `fs.readFileSync(resolved.path)` sem parâmetro `encoding`,
retorna `Buffer`. `barrier.js:596` — `localContentBuf.equals(show.stdout)` onde `show.stdout`
também é `Buffer` (spawnSync sem `encoding`). Comparação é byte-a-byte via `Buffer.prototype.equals`.
A string UTF-8 (`content`) é usada apenas para parsear gates, não para comparação de confiança.

---

### F5 — Separação de FileNotFoundError vs returncode: FECHADO nos 3 CLIs

**Evidência:**

- Go (`barrier.go:683-695`): `errors.As(err, &exitErr)` — spawn failure retorna "git not found in
  PATH", exit non-zero retorna "not a git repository". Dois paths, dois `return` distintos.
- Node (`barrier.js:508-523`): `revParse.error` distingue spawn failure de `revParse.status !== 0`.
  Dois `return` com mensagens distintas.
- Python (`barrier.py:515-528`): `except FileNotFoundError` captura spawn failure com mensagem "git
  not found in PATH". `if r.returncode != 0` captura o repositório inexistente.

**Impacto nos verditos de confiança:** nenhum. Ambos os caminhos em cada CLI retornam
`trusted: False`. As mudanças afetaram apenas os campos de mensagem. Nenhum novo caminho fail-open
foi introduzido. O Step 5 (cat-file) continua com três causas colapsadas em uma mensagem — declarado
não separável por design, conforme comentário em `barrier.go:768-773`.

---

### F6 — Comentário stale em check-barrier.sh: FECHADO

**Evidência:** `scripts/check-barrier.sh` (região de `make_barrier_git_fixture`) — o comentário
anterior descrevia "barrier fails-open (trusted)". O texto atual diz "barrier fails-closed
(not_evaluated)". O fato correto (fail-closed, não fail-open) está agora documentado. O fix continua
necessário (`WORK_PHYS`) mas a consequência de não fazê-lo está corretamente descrita.

---

## Veredito Final de Fechamento

| # | Achado original | Status |
|---|-----------------|--------|
| F1 | Invariante quebrado: prova ≠ payload | **FECHADO** — mesmo buffer em produção nos 3 CLIs; gap de cobertura de teste em Node/Python (VerifiesPassedBuffer ausente) |
| F2 | Defaults latentes fail-open | **PARCIAL** — get defaults fechados nos 3 CLIs; Python None-guard crashe com KeyError em vez de retornar not_evaluated limpo; sem impacto de segurança |
| F3 | Guarda estrutural Go-only | **FECHADO** — 3 sentinelas comportamentais em Node e Python; gap de cobertura F1 é residual declarado de F1, não de F3 |
| F4 | Node.js: strings UTF-8, não bytes | **FECHADO** — Buffer.equals() nos 3 lados da comparação |
| F5 | Python FileNotFoundError conflation | **FECHADO** — mensagens distintas nos 3 CLIs; nenhum impacto em verditos de confiança |
| F6 | Comentário stale no script | **FECHADO** — "fails-open" substituído por "fails-closed (not_evaluated)" |

**Veredito final: APROVA.**

F2 PARCIAL não bloqueia: o comportamento de segurança está correto (fail-closed), apenas o docstring
diverge do código. O gap de VerifiesPassedBuffer (F1/F3) é residual de cobertura, não de correção —
a produção está correta nos 3 CLIs. Nenhum caminho exploitável dentro do threat model declarado
permanece aberto.
