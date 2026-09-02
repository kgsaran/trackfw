---
status: done
date: 2026-09-01
req: "docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: Os 3 CLIs executam gate de wave com `sh -c`

> Created: 2026-09-01 | Status: done

## Context

REQ: `docs/req/REQ-2026-09-01-mesmo-gate-de-wave-da-vereditos-diferentes-conforme-o-cli-que-executa-o-barrier.md`
ADR: `docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md`

**Item 7 do issue #216** — o mais grave da lista, e o único que quebra a correção da **própria
ferramenta de governança**: `trackfw barrier` pode aprovar uma wave para quem usa um CLI e reprová-la
para quem usa outro, no mesmo repositório e no mesmo commit.

Medição que decidiu o ADR: **83 comandos** de gate em todos os roadmaps, e **nenhum idioma existe no
`cmd.exe`** (35 `grep`/`sed`/`awk`, 14 `test`/`[`, 8 negações com `!`, 3 `&&`/`||`, 3 `$( )`).
No Windows, Node e Python **não avaliam diferente — falham em avaliar**.

## Acceptance Criteria

- [ ] Os 3 CLIs executam gate com `sh -c`
- [ ] Mesmo gate, mesmo veredito nos 3 — **e o controle**: gate que deve reprovar continua reprovando
- [ ] `sh` ausente falha nomeando o remédio, com mensagem byte-idêntica nos 3
- [ ] **"Não pôde ser avaliado" é distinto de "reprovou"**
- [ ] Gate contra regressão para `shell: true`/`shell=True`
- [ ] Item 7 sai de `REPRODUCED` (camada 2 de 4 → 3), com a transição explicada
- [ ] `make quality` e **CI** verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Modelo de ameaça da troca de shell
> Dependências: nenhuma. Bloqueia a implementação.

### ML-0A — Superfície de execução e a semântica de "não pôde medir"
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** nenhum (documento em `docs/seguranca/`)
**Por que esta Wave 0 importa mais que a média:** estamos mudando **como conteúdo de artefato
versionado vira processo**. O gate já executava comando arbitrário — não é superfície nova — mas o
**shell muda**, e com ele o parsing, o quoting e o tratamento de erro.
**Actions:**
1. **A troca amplia superfície?** `sh -c` versus shell do SO: um gate malicioso num roadmap de PR de
   terceiro ganha capacidade nova? (Lembrar: o `barrier` já executa esses comandos hoje no Go.)
   Considere o `roadmapTrustForGates`, que **já tem REQ aberta por fail-open**.
2. 🔴 **A semântica de "não pôde ser avaliado".** A AC4 exige distinguir isso de "reprovou". **Julgue
   qual é o lado seguro** e por quê: um `sh` ausente que resulte em *"wave não passou"* é falso
   negativo que bloqueia trabalho legítimo; que resulte em *"passou"* é falso positivo que libera
   trabalho não verificado. **Nenhum dos dois é obviamente certo — quero o argumento.**
3. **Falsificação nas duas direções**, com atenção ao simétrico: um `barrier` que passe a recusar
   ambiente legítimo (contêiner mínimo sem `sh`? CI de terceiro?) troca um defeito por outro.
4. **Enumeração:** só os dois pontos (`barrier.js:561`, `barrier.py:582`)? Varra por outros lugares
   nos 3 CLIs onde conteúdo de artefato vira processo — `shell: true`, `shell=True`, `exec`, `spawn`,
   `system`. **Assuma que minha lista de dois está incompleta**; nesta sessão isso se confirmou
   repetidamente.
5. **Residual declarado.**
**Critérios de aceite:**
- [x] Veredito sobre ampliação de superfície, com vetor concreto se houver
- [x] Argumento explícito sobre o lado seguro de "não pôde medir"
- [x] Enumeração de pontos onde artefato vira processo, nos 3 CLIs
- [x] Nenhuma linha de implementação escrita
- [x] Parecer em `docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md`

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
! grep -qi "placeholder" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
grep -q "Residual" docs/seguranca/2026-09-01-modelo-de-ameaca-do-shell-de-gate.md
```

#### Resultado do ML-0A (hades-tf, 2026-09-01) — auditado pelo arquiteto

### 1. Ele corrigiu a própria primeira conclusão, e o achado inverte minha premissa

A primeira passagem concluiu *"no-op em POSIX"* — por **leitura de código**. Uma PoC com `sh` falso
no `$PATH` provou o contrário:

| forma | resolve `sh` como |
|---|---|
| `shell: true` / `shell=True` (hoje) | **preso a `/bin/sh`** |
| `spawnSync('sh', [...])` / `subprocess.run(["sh", ...])` (Wave 1) | **via `$PATH`**, como o Go sempre fez |

**A correção amplia superfície — pequena, mas real.** Node e Python saem de interpretador fixo para
interpretador resolvido por `$PATH`. É **necessário** para Windows, onde o `sh.exe` do Git Bash nunca
está em `/bin/sh`; mas quem controla o `$PATH` do processo passa a controlar o interpretador do gate
nos três, e não só no Go.

Eu teria assumido no-op. **Ele mediu.**

E foi honesto sobre o limite: no Windows a premissa do ADR é **inferida, não medida** — não há máquina
disponível. Nomeou como inferência em vez de afirmar como fato.

### 2. A resposta sobre "não pôde medir" veio com precedente interno

Eu pedi o argumento, não a conclusão, e sugeri considerar um terceiro estado. A resposta é melhor que
a sugestão: **o projeto já resolveu este exato problema.**

O `roadmapTrustForGates` tem um terceiro status — **`not_evaluated`**, distinto de `passed` e
`blocked` — nos três CLIs (`barrier.go:872`, `barrier.js:592`, `barrier.py:688`/`:747`). Ele
**reprova a wave** e **nomeia o remédio**.

**Fail-closed, reusando padrão existente**, em vez de inventar código de saída. E ele mediu algo que
teria quebrado a implementação: `sh -c 'nosuchtool'` devolve **exit 127** — *ferramenta interna
ausente*, **não** *`sh` ausente*. A AC4 **não pode** se apoiar em 127; o sinal de `sh` ausente é
falha no nível do spawn.

### 3. A enumeração confirmou o escopo do ADR — e achou uma vulnerabilidade fora dele

Só os três pontos do `barrier` levam conteúdo de **artefato versionado** para um shell. O ADR está
correto.

**Mas apareceu outra coisa**, porque pedi *todo* ponto onde conteúdo vira processo:

```
--host  'x" ; id > /tmp/INJETADO ; echo "'
   ↓
exec:   open "http://x" ; id > /tmp/INJETADO ; echo ":4080"
```

`serve.js:46-56` devolve `http://${host}:${port}` **sem sanitização** para qualquer string que não
seja `localhost` nem IP, e o resultado é interpolado numa string de shell passada a `exec()`.
**Reproduzido pelo arquiteto.** `serve.py:196` tem a variante Windows (`shell=True`); os ramos Darwin
e Linux do Python usam argv e **estão corretos** — precedente interno para a correção.

**REQ própria aberta.** Fora do escopo desta; não entra na Wave 1.

### Residual declarado

Composição com a REQ aberta de fail-open do `roadmapTrustForGates`; o vetor de adulteração de
`$PATH` de §1a; e a recomendação de que AC2/AC3/AC7 sejam verificadas no job `windows-full-suites`,
já que runner POSIX **não consegue falsificar** um defeito específico de Windows.

## Wave 1 — A correção (ML único)
> Dependências: Wave 0. ML único: os dois arquivos são pequenos e a semântica de erro tem de ser
> idêntica entre eles — separar convidaria à divergência que a REQ existe para fechar.

### ML-1A — `sh -c` nos dois CLIs, com `not_evaluated` para `sh` ausente
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`
**Actions:**
1. Trocar `spawnSync(cmd, { shell: true })` e `subprocess.run(cmd, shell=True)` por invocação
   explícita de `sh -c`, resolvida por `$PATH` — como o Go já faz.
2. 🔴 **`sh` ausente vira `not_evaluated`, não falha de gate.** **Reuse o padrão que já existe** no
   `roadmapTrustForGates` (`barrier.go:872`, `barrier.js:592`, `barrier.py:688`/`:747`): terceiro
   status, distinto de `passed`/`blocked`, que **reprova a wave** e **nomeia o remédio**. Não invente
   código de saída novo.
3. 🔴 **Não use `exit 127` como sinal de `sh` ausente.** Medido no ML-0A: `sh -c 'nosuchtool'`
   devolve 127 — *ferramenta interna ausente*, não *`sh` ausente*. O sinal correto é falha no nível
   do **spawn**.
**Critérios de aceite:**
- [x] Gate com idioma POSIX (`! grep -q`, `test -f`, `$( )`) dá **o mesmo veredito nos 3 CLIs**
- [x] 🔴 **Controle:** gate que **deve reprovar** continua reprovando nos três — uniformizar para
      "passa" seria pior que o defeito
- [x] `sh` ausente → `not_evaluated`, mensagem **byte-idêntica** nos 3, nomeando o remédio
- [x] `make quality` verde

#### Resultado do ML-1A (apolo-tf, 2026-09-01) — auditado pelo arquiteto

### O desvio de escopo foi dele, e a lista errada era minha

`barrier.go` **não estava** em *Files affected*. Ele o tocou e declarou por quê: a **AC3 exige
mensagem byte-idêntica nos 3**, e o Go colapsava falha de spawn em `"<cmd>: exit 1"` genérico —
**não satisfaria a AC mesmo já usando `sh -c`**. Eu escrevi a lista assumindo que o Go estava
pronto porque acertava o *shell*; a AC era sobre a **mensagem**.

`runGateCommand` passou a devolver `(exitCode, spawnFailed)`, com `evalGateCommands` extraído para
eliminar duplicação entre os dois ramos de trust. Não tocou o `roadmapTrustForGates`.

### Verificação por execução, não por `grep` no fonte

O `grep` mostrava o Go quebrado em concatenação — comparar fonte não provaria nada. **Rodei os três
binários** com `$PATH` curado sem `sh`:

```
gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates
```

**Byte-idêntica nos três.** E nomeia o remédio, em vez de vazar `exec: "sh": executable file not
found` — que era o ponto da AC3.

### O controle, que é a metade que impede o falso sucesso

Gate que **deve** reprovar, com `$PATH` normal:

```
go       ✗ gates: blocked
python   ✗ gates: blocked
node     • gates: grep -q "notpresent" trackfw.yaml: exit 1
```

**A uniformização não virou "tudo passa"** — que seria pior que o defeito, porque provaria
consistência sem provar correção. E os três distinguem `blocked` de `not_evaluated`: a AC4.

### O padrão reusado, não inventado

`not_evaluated` já existia no `roadmapTrustForGates` nos três CLIs. Reusar significa que o conceito
tem **um** vocabulário no projeto, já revisado — em vez de dois nomes para a mesma ideia.

E ele respeitou a medição do ML-0A: **`exit 127` não é sinal de `sh` ausente**. Verificado —
`sh -c 'nosuchtool-xyz'` dá `exit 127` e `status: blocked` nos três, nunca `not_evaluated`.

**`MAKE_EXIT=0`**, zero `FAIL`, `go test` verde, Node 70/70, Python 60/60.

**Limite declarado:** o defeito original é específico de Windows e não há runner aqui. A mudança de
pino fixo para resolução por `$PATH` **é real em POSIX** — confirmada pelo próprio teste de `$PATH`
curado. O veredito Windows só o CI fecha.

## Wave 2 — Gate e contrato
> Dependências: Wave 1 completa.

### ML-2A — Gate contra regressão e contrato
**Status:** ✅ Concluído
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-shell-posix-portability.sh` (novo), `Makefile`, `docs/cli-parity.md`
**Actions:**
1. Gate impedindo regressão de `barrier.js`/`barrier.py` para `shell: true`/`shell=True` na execução
   de gate.
2. Contrato do ADR em `docs/cli-parity.md`, **registrando também o custo** medido no ML-0A.
**Critérios de aceite:**
- [x] O gate reprova **regressão em apenas um** dos dois CLIs, **nomeando qual** — o caso que
      `assert_has` deixaria passar
- [x] Guarda de vacuidade: `ROOT` vazio faz as 10 checagens reprovarem individualmente
- [x] 🔴 O gate **não reprova por menção legítima** — os comentários do ML-1A citam `shell: true` em
      prosa, e um `grep` ingênuo reprovaria a árvore correta
- [x] Nasce **ligado** ao `parity:`; `make -n parity` expande
- [x] O contrato declara que a mudança **não é no-op em POSIX**
- [x] `make quality` verde

**Entregue:** `scripts/check-shell-posix-portability.sh` — **10 assinaturas** em `barrier.js` e
`barrier.py`, ligado ao `parity:`, contrato anotado com **cobertura plena** (`gate=`, não `partial=`).

#### A armadilha auto-referente, pela terceira vez nesta sessão

**Os comentários do ML-1A citam `shell: true` e `shell=True` em prosa**, para documentar o que não
repetir. Um `grep` no arquivo inteiro **reprovaria a árvore correta, em cima da própria
documentação**. Ela adicionou `assert_no_code_match`, que exclui linhas de comentário antes de
grepar.

É o mesmo padrão do gate do separador, que teria reprovado sobre os documentos que descrevem o
defeito. **Um artefato que documenta um antipadrão contém o antipadrão** — e gate ingênuo não
distingue menção de uso.

#### `assert_count` onde a presença não basta

O literal `not_evaluated` ocorre **legitimamente duas vezes por arquivo** — ramo de trust e ramo de
`sh` ausente. `assert_count(2)` distingue *"os dois ramos ainda reportam o terceiro estado"* de
*"um colapsou em `blocked`"*. `assert_has` não distinguiria.

#### Falsificação — verifiquei o caso decisivo

```
árvore correta          →  exit 0, "10 assinaturas confirmadas"
regressão SÓ no Node    →  exit 1, nomeando barrier.js — Python não mencionado
```

**Detecta regressão em um só CLI e diz qual.** Ela também falsificou o simétrico (só Python) e a
vacuidade (`ROOT` vazio → as 10 reprovam individualmente, cada uma nomeando o arquivo ausente).

#### O contrato registra o custo, não só o benefício

Anotação de **cobertura plena**, e o texto declara o que era tentador omitir: a mudança troca
**interpretador fixo em `/bin/sh`** por **resolvido via `$PATH`** — necessário para Windows, mas
**não é no-op em POSIX**. Quem controla o `$PATH` controla o interpretador do gate, e isso **compõe**
com a REQ aberta de fail-open do `roadmapTrustForGates`.

Um contrato que descreve só o lado bom é meio-caminho para o contrato falso que quase publiquei no
ciclo do `fchmod`.

**`MAKE_EXIT=0`**, `go test` nos 17 pacotes, Node 842 testes, cobertura de contrato limpa.

## Verificação que só o CI fecha

Item 7 saindo de `REPRODUCED`: camada 2 de **4 para 3**. O check compara o veredito do mesmo gate nos
3 runtimes — **comportamento de produto**, então deve genuinamente virar. Verificado o que ele mede
**antes** de fixar o número.

## Barreira final

`hefesto-tf` e `hades-tf`, auditoria do arquiteto, `barrier`. **CI verde**, não só verde local.


## MEDIÇÃO NO CI — a contagem NÃO caiu, e o motivo é o instrumento

PR #236, `windows-defect-reproduction`:

```
Reproduzidos: 4   (esperado: 3)
## ITEM 7 — Go (sh POSIX) vs Node/Python (cmd.exe) avaliam o MESMO gate de wave diferente
RESULT: REPRODUCED
```

**A correção está certa e o check não a enxerga.** O item 7 é medido por
`scripts/windows-repro/go/checks.go:135`:

```go
func cmdGateQuote() {
    c := exec.Command("sh", "-c", gateQuoteCommand)   // réplica, não o barrier
```

O check roda **réplicas dentro do próprio harness de reprodução** — `checks.go`, `checks.js`,
`checks.py`, cada uma com sua invocação de shell. A correção mudou `barrier.js` e `barrier.py`; **o
check exercita os `checks.*`**.

**É a mesma classe dos itens 2 e 3** — o check mede um substituto, não o produto. Agora com três
instâncias, é padrão do instrumento e não coincidência.

### O erro é meu, e é o que eu tinha escrito para evitar

Escrevi neste roadmap: *"verificado o que ele mede antes de fixar o número"*. **Verifiquei a
descrição, não a implementação.** O comentário do `run.ps1` diz *"compara o veredito do mesmo gate
nos 3 runtimes"* — o que descreve a **intenção**. O `cmdGateQuote` compara a saída de **três scripts
réplica**.

**A documentação do check descrevia o que ele deveria medir, não o que mede.** Terceira variante da
mesma armadilha nesta sessão, e a mais sutil das três: nas anteriores o texto estava certo e a
implementação furada; aqui o texto **descreve corretamente uma coisa que o código não faz**.

### Consequência

A **correção do item 7 permanece válida** — verificada por execução dos 3 binários reais: mensagem
byte-idêntica, `not_evaluated` distinto de `blocked`, e o controle reprovando nos três. O que falha é
**a capacidade da camada 2 de observar isso**.

O retarget entra na REQ que já existe para os itens 2 e 3
(`REQ-2026-09-01-camada-2-mede-a-plataforma-e-nao-o-produto-...`), que passa a ter **três** itens em
vez de dois. **Não abro REQ nova**: é a mesma causa raiz, e fragmentar esconderia que o instrumento
tem um padrão, não três acidentes.
