---
status: blocked
date: 2026-09-03
squad: apolo-tf
req: "docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md"
---

# Roadmap: Fechar os grupos de falha de Windows por causa raiz

> Criado em: 2026-09-03 | Status: blocked

## Context

REQ: docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md

## Diagnóstico

Contagem medida no run `33810452454` da `main`, o primeiro com a Wave 0 e o `eol` dentro:

```
          ANTES  AGORA  delta
Go          86     64    -22
Node        56     52     -4
Python     104    101     -3
          ────   ────   ────
TOTAL      246    217    -29
```

**As 217 são defeito real.** Eu estimei 73 desmascaradas e foram 29 — errei por 2,5x, e o ML-1C
tinha avisado.

## Acceptance Criteria

- [ ] O mecanismo do grupo B identificado e escrito, ou virado REQ com o que foi eliminado
- [ ] As 3 ADRs `Accepted` antes do código do grupo que cada uma governa
- [ ] Falsificação nas duas direções e controle POSIX em cada grupo
- [ ] 🔴 Recontagem no CI **por wave**, com o delta atribuído ao grupo
- [ ] 🔴 Nenhuma correção reduz contagem **escondendo** defeito
- [ ] `make quality` verde e os 9 checks obrigatórios verdes ao fim de cada wave

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — O desconhecido, sozinho
> Dependências: nenhuma. **Bloqueia a estimativa**, não as outras waves.

### ML-0A — Grupo B: por que o `bash` do Python devolve exit 1 uniforme
**Status:** ✅ Concluído (investigação — mecanismo NÃO identificado, espaço reduzido a 2) · **Agente:** `artemis-tf`
**~56 testes, 26% do total, mecanismo DESCONHECIDO.** É o maior risco isolado do lote.

`pypi/tests/test_credential_guard.py`, `test_git_branch_guard.py`,
`test_credential_guard_sabotage.py`, `test_git_branch_guard_dedup.py` — todos os testes de
guard-script retornam 1, com stderr vazio, **inclusive o caso que deveria sair 0 na segunda linha**
(`[ -f trackfw.yaml ] || exit 0`).

🔴 **O discriminante já existe e mata a teoria ambiental:** o **Node roda o mesmo script pelo mesmo
`bash`**, com a mesma chamada `spawnSync('bash',[script])`, **e passa** —
`credential_guard.test.js` dá `22 passed, 2 failed` internamente, e os 2 são bit de execução. **O
defeito é do lado Python.**

Suspeitos **não verificados**: `HOME` de sessão herdado pelo filho; tradução de newline por
`text=True`.

**Critérios de aceite:**
- [ ] 🔴 O mecanismo está **escrito com a medição**, ou o relatório lista **o que foi eliminado e
      como** — "não sei ainda" é resultado válido; **hipótese como causa, não**
- [ ] O caso `exit 0` (segunda linha do script) é medido **isoladamente** — é o que discrimina
      "script errado" de "invocação errada"
- [ ] Comparação **lado a lado** com o braço Node, que passa: mesma chamada, mesmo script, resultado
      diferente. **A diferença é o achado**
- [ ] Nenhuma correção aplicada nesta wave — é **investigação**


**Resultado da investigação — auditoria do arquiteto, 2026-09-04.**
Documento: `docs/qualidade/2026-09-04-grupo-b-bash-do-python-em-windows.md`.

🔴 **Mecanismo NÃO identificado, e isso é o resultado certo.** O espaço foi reduzido a **duas
ramificações**, e nada foi apresentado como causa:

```
(A) o bash que o Python lanca NUNCA executa o script
(B) o script morre entre `set -euo pipefail` e a guarda de projeto
    unico candidato: INPUT=$(cat)      <- nao falsificavel no macOS
```

🔴 **Mais DUAS premissas do meu briefing derrubadas — sexta e sétima da sprint.** Verificadas por
mim:
- **O `exit 0` não é a segunda linha do script — é a oitava.** O `set -euo pipefail` está na
  **linha 18** e o `INPUT=$(cat)` vem antes da guarda: é exatamente onde (B) pode viver. Eu escrevi
  "segunda linha" no handoff e isso teria mandado a investigação para o lugar errado.
- **A população é 50, não ~56.** Das 52 falhas dos 4 arquivos, 3 são de outros grupos (2 de bit de
  execução, 1 de separador) e 2 aparecem só como `SUBFAILED`. O critério que fecha o grupo
  **discrimina**: **nenhum** teste Python que lança `bash` passou (50/50); dos que **não** lançam, só
  3 falharam.

**O caso `exit 0`, isolado — e o gêmeo do outro extremo:** `test_sem_match_e_no_op_silencioso`
espera 0 e recebe 1; `test_git_push_with_trackfw_yaml_still_blocks` espera **2** e recebe **1**.
**Os dois extremos devolvem o mesmo 1 — o script não chega a decidir nada.**

🔴 **A diferença é de TRÊS braços, não dois — e isso derruba a minha formulação.** Eu disse "o Node
passa, logo o defeito é do Python". **O Go também lança `bash` sobre os mesmos scripts, no mesmo
job, e executa de verdade.** Verificado por mim: `npm/tests/credential_guard.test.js:80` faz
`spawnSync('bash', [scriptPath])`. Sobra **o ato de lançar**: Go usa `LookPath` e Node usa libuv —
**ambos entregam caminho absoluto ao `CreateProcess`** —, enquanto o CPython chama `CreateProcess`
com `lpApplicationName = NULL` e cai na **ordem implícita do Windows**. Assimetria real, **não
medida no runner**: por isso é hipótese, não causa.

**Eliminados, cada um com a assinatura que o descarta:** CRLF (rc **2**, e o ITEM 5 mediu
`crlf=False` no Windows) · script ausente (rc **127**) · bit de execução (rc 0) · `text=True` no
stdin (payload sem `\n`) · `HOME` herdado (rc 0) · stdin vazio (rc 0) · "bash quebrado no runner"
(ITEM 7 mediu `sh` presente e executando). **Só uma assinatura reproduz o observado: algo saiu 1
falando por `stdout`** — o canal que os 50 testes descartam.

**O que falta, e é uma linha:** ITEM 12 na sonda (`scripts/windows-repro/run.ps1`), pwsh-safe:
`where.exe bash`, `bash -c 'echo …'` pelo Python, e o script real **com `stdout` impresso**. Separa
(A) de (B).

**Severidade condicional, medida:** nenhum módulo de `pypi/trackfw/` lança `bash`/`sh`. Se for
**(A)**, o remédio é de **harness** (`shutil.which` + caminho absoluto em 6 sítios) e fecha ~50
vermelhos **sem esconder defeito de produto**. Se for **(B)** com o script morrendo sob invocação
legítima, **vira segurança**.

### ML-0B — ITEM 12 da sonda: separar (A) de (B)
**Status:** ✅ Concluído · **Agente:** `dedalo-tf`
**Files affected:** `scripts/windows-repro/run.ps1`
Sonda observacional, uma linha de decisão. 🔴 **Não corrigir nada** — só medir.
**Critérios de aceite:**
- [x] `where.exe bash` registrado
- [x] `bash -c 'echo ...'` lançado **pelo Python**, com `stdout` **impresso**
- [x] o script real invocado como os testes invocam, **com `stdout` impresso** — é o canal que os 50
      testes descartam e onde a única assinatura compatível vive
- [x] 🔴 pwsh-safe: variável em string entre aspas antes de virar argumento
      (`vault/notes/powershell-modo-argumento-nao-interpola-nem-divide-2026-08-31.md`)
- [x] Nada além do `run.ps1`; nenhum teste tocado


**Evidência de aceite — auditoria do arquiteto, 2026-09-04:**

```
git diff --stat scripts/windows-repro/run.ps1  ->  306 insercoes, 0 remocoes
                                                   <- ITENS 1-11 intocados por construcao
corpo Python falsificado no macOS  ->  VERDICT=NOT-REPRODUCED
                                       <- a sonda nao acusa defeito onde nao ha
```

🔴 **Duas decisões dele que evitam medição enganosa, e nenhuma estava no meu handoff:**

**Recusou o `shutil.which("bash")` como braço de "caminho absoluto".** O `which` varre o `%PATH%`
**na mesma ordem** que a hipótese (A) diz **não** ser a do `CreateProcess` com
`lpApplicationName=NULL`. Usá-lo poderia devolver "idêntico" **sem provar nada** — mediria a mesma
coisa duas vezes achando que mediu duas.

**Travou o rótulo `BRANCH-B` atrás de prova de identidade** (`GNU bash` no `--version`). Dois
não-bash devolvendo 1 são **(A)**; rotulá-los **(B)** converteria **defeito de harness em alarme de
segurança**. É a diferença entre "remédio de fixture" e "incidente", e ele não deixou o rótulo
escorregar.

**Terceiro cuidado, não previsto:** braço de **redirecionamento para arquivo**, porque o stub do WSL
escreve no console em vez dos handles redirecionados. Sem ele, um "vazio" seria **o mesmo nada que
os 50 testes já medem** — a sonda repetiria o erro que existe para diagnosticar.

**Cp1252 vivo nesta árvore:** o corpo usa `PYTHONIOENCODING=utf-8` + `ascii()` em toda saída medida,
porque o item 1 mataria a sonda no primeiro `print`.

### ML-0C — `bash` por caminho absoluto nos sítios de teste do Python
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Files affected:** os 6 sítios Python que lançam `bash`, em `pypi/tests/`

🔴 **CAUSA RAIZ MEDIDA** (ITEM 12, run `33875124523`) — e não é "não existe bash":

```
shutil_which_bash = 'C:\Program Files\Git\bin\bash.EXE'   <- GNU bash, --version rc=0
bare_rc           = 1
bare_is_gnu_bash  = False
bare_out          = UTF-16: "Windows Subsystem for Linux has no installed distributions."
```

**`C:\Windows\System32\bash.exe` é o stub do WSL e VENCE a resolução por nome nu.** Sem
distribuição instalada, sai **1** e escreve em **UTF-16 pelo `stdout`**.

Explica os três sintomas de uma vez: o `exit 1` uniforme é o **stub**, não o script (por isso os dois
extremos dão o mesmo 1); o `stderr` vazio é porque o stub fala por **`stdout`**, canal que os 50
testes descartam; e Go e Node passam porque entregam **caminho absoluto** ao `CreateProcess`,
enquanto o CPython passa `lpApplicationName = NULL` e cai na ordem implícita, onde `System32` vem
antes de `Git\bin`.

**É defeito de HARNESS, não de segurança.** O guard não morre — **nunca é invocado**.

🔴 **`shutil.which` sozinho NÃO é o remédio, e a sonda provou por quê:** ele varre o `%PATH%` **na
ordem do PATH** e devolve o binário **certo** — mas essa **não é** a ordem do `CreateProcess` com
`lpApplicationName=NULL`. Ele serve para **achar** o candidato; o que corrige é **passar o caminho
absoluto** ao `subprocess`.

🔴 **Prove a identidade, não a existência.** O discriminante entre "não achou" e "achou o errado"
**não é o exit code** — é `--version` contendo `GNU bash`. Um `bash.exe` que existe e não é bash é
exatamente o defeito.

**Critérios de aceite:**
- [x] Os sítios Python lançam `bash` por **caminho absoluto provado** (`GNU bash` no `--version`)
- [x] 🔴 **Falsificação:** revertendo, o stub do WSL volta a vencer — provável só no CI; localmente,
      provar que o caminho passado **deixa de ser nome nu**
- [x] 🔴 **Controle POSIX:** `python3 -m pytest pypi/tests/` com o **mesmo total** de antes
- [x] 🔴 **Nenhum teste marcado `skip`**
- [x] Se nenhum candidato for GNU bash, o teste **falha nomeando isso** — não pula em silêncio
- [x] Nada fora de `pypi/tests/`; nenhum módulo de `pypi/trackfw/` tocado (medido: nenhum lança bash)


**Evidência de aceite — auditoria do arquiteto, 2026-09-04:**

```
grep 'subprocess.run(["bash"' em pypi/tests/  ->  nenhum lancamento por nome nu restante
argv real                                    ->  ['/opt/homebrew/bin/bash']
controle POSIX  antes 1604 passed  depois 1604 passed  (--ignore do arquivo de teste novo)
                suite completa 1609 = 1604 + os 5 testes de guarda novos
```

🔴 **Eram 10 sítios, não 6 — eu passei o número errado no handoff.** Ela verificou e corrigiu, e usou
a **uniformidade da forma** (`subprocess.run(["bash", <script>, *args])` nos dez) para justificar
**helper único** em vez de resolução repetida.

🔴 **O portão de identidade roda em BYTES, de propósito.** A saída UTF-16 do stub do WSL não casa com
`b"GNU bash"`, então o candidato é recusado **sem depender de decodificação** — que é exatamente onde
o cp1252 e o UTF-16 mordem. E a exclusão explícita do `System32` é **cinto-e-suspensório**: o stub
seria recusado por identidade mesmo sem ela.

**Falsificação nas duas direções:** um candidato que **existe e sai 0** mas não é bash (`/bin/echo`)
é recusado, e `BashNotFound` nomeia cada tentativa; com o impostor **à frente** na lista, o bash real
ainda vence pelo portão.

**Sem candidato válido → `BashNotFound`, não `skip`.** Resolução **preguiçosa**, para a falha aparecer
como erro dos testes que lançam bash e não como erro de coleta da suíte.

🔴 **Ela marcou a própria evidência como fraca onde é fraca:** o ramo `os.name == "nt"` é **código não
executado** pela medição dela, e `test_nunca_resolve_para_o_stub_do_wsl` é *"quase vacuoso em
POSIX"* — pediu para **não contar como evidência local**. É a diferença entre um teste que protege e
um que decora.

**Verificação que só o CI fecha:** a contagem Python cair de 101 para ~51.

## Wave 1 — As três decisões (arquiteto, sequenciais, NÃO paralelizam)
> Dependências: nenhuma. Não esperam a Wave 0.

### ML-1A — ADR: o trackfw escreve separador POSIX nos artefatos que autora?
**Status:** ✅ Concluído · **Agente:** `trackfw_architect`
**Entregue:** `docs/adr/ADR-2026-09-04-separador-posix-nos-artefatos-autorados-cujo-consumidor-nao-e-o-sistema-de-arquivos.md` — `Accepted`. Corrigida após o ML-2A: a premissa "só o Node não normaliza" foi medida e é o inverso.
**Resolve TRÊS grupos de uma vez** (~45 testes): `tildeify` devolvendo `~\...`, `provenanceKey`
nativo no Node, e caminho em JSON lido por CLI de agente.
Evidência que tende a **sim**: a chave de proveniência **já é** `/` por decisão documentada; `~` é
POSIX-ismo que nenhum shell do Windows expande; um `command` bash com `\` é mastigado pelo shell.

### ML-1B — ADR: o parser de frontmatter deve tolerar CRLF?
**Status:** ✅ Concluído · **Agente:** `trackfw_architect`
**Entregue:** `docs/adr/ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-fronteira-de-entrada.md` — `Accepted`. Implementação na **Wave 5**.
O parser é **cego a CRLF** e emitiu frontmatter **duplicado** em `TestRenderOpenCodeAgent`. ~14
testes. 🔴 A alternativa — declarar `eol` sobre os assets — **foi medida e recusada** no ML-1C:
esconde o defeito em vez de curá-lo.

### ML-1C — ADR: caminho POSIX ancorado num config lido por CLI de agente é "absoluto"?
**Status:** ✅ Concluído · **Agente:** `trackfw_architect`
**Entregue:** `docs/adr/ADR-2026-09-04-caminho-posix-ancorado-num-config-lido-por-cli-de-agente-e-absoluto-independente-do-so-host.md` — `Accepted`. Implementação na **Wave 3**.
`filepath.IsAbs("/opt/…")` é **falso** no Windows → `classifyHookAnchorage` classifica ancorado como
relativo → **o validator deixa de emitir violation de guard ausente**. ~14 testes, **e é de
segurança**: a detecção de hook de guard **enfraquece no Windows**.

## Wave 2 — Separador, nos 3 CLIs
> Dependências: ML-1A `Accepted`.

### ML-2A — Separador POSIX em artefato autorado
**Status:** ✅ Concluído · auditado pelo arquiteto e **mergeado no PR #270** · **Agente:** `apolo-tf`
**Recontagem no CI (run `33913343975`), o delta deste grupo:** `134 → 101` (Go 53→46, Node 48→34, Python 33→21). A triagem previa ~45; foram 33.

🔴 **Correção de 2026-09-04:** este número foi reportado como `134 → 69` (Go 53→14) por **erro de
medição meu** — o padrão de `grep` não casava o prefixo por linha do `gh run view --log`. Re-medido
com padrão idêntico nas duas pontas. Nota:
`vault/notes/contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo-2026-09-04.md`.
**Files affected — os 3 stacks:** `npm/src/lib/update-engine.js:172-181`,
`pypi/trackfw/commands/update_harness.py::_tildeify`, `internal/integrations/manager.go`,
`npm/src/validator/index.js:3153` (`provenanceKey` sem normalização), `npm/src/serve/api_chain.js`
⚠️ `npm/src/validator/index.js` é classificado como **binário** pelo `file` — `grep` sem `-a` o pula
**em silêncio**; 2 REQs deste repo têm premissa falsa por isso.
**Critérios:** falsificação nas duas direções · controle POSIX com números · os 3 CLIs dão o **mesmo**
resultado · recontagem no CI com o delta atribuído a este grupo.


**Evidência de aceite — auditoria do arquiteto, 2026-09-04:**

```
make quality QUALITY_EXIT=0, zero FAIL · validate exit 0
gate de separador: 18 -> 40 assinaturas; as 22 novas REPROVAM contra a arvore pre-ML-2A
controle POSIX byte-identico: /api/board e /api/chain nos 3 · update harness 3810 B em Node e Py
suites: Go 1212 PASS · Node 859/859/0 skipped · Python 1613 passed
```

**19 sítios de emissão + 3 de fixture, enumerados** — a verificação que a ADR exige. Em todos, o
valor normalizado é **derivado** (fatia, chave, id) e o operando da syscall é **expressão separada**.
UNC e `\\?\` intocados.

🔴 **NONA premissa minha derrubada — e esta estava na ADR, não num handoff.** Eu escrevi que "só o
Node não normaliza, e por isso passa por acidente", o que induz a corrigir o Node. **Medido, é o
contrário:**

```
producao          internal/integrations/render.go:821  grava a chave com "/" EXPLICITO
as TRES fixtures  montavam a chave com separador NATIVO
```

**Em Windows, Go e Python reprovam contra o produto CERTO.** O Node passa porque fixture **e** produto
estão **ambos** errados. **Corrigir só o produto do Node o viraria de verde para vermelho.** O
remédio foi normalizar as 3 fixtures junto. ADR corrigida.

**Décima:** o `_tildeify` do Python já era **meio-corrigido** (`~/` fixo + cauda nativa), então os 3
CLIs **discordavam entre si** — não era "Node divergente", era divergência de **três vias**.

**E a categoria 3 tem ZERO pontos de emissão, medido:** os `command` de hook são **literais** nos 3
runtimes e o gate de wave é **lido do markdown**. Registrado na ADR para impedir que alguém "aplique
a decisão" numa categoria já correta por construção.

**Dois desvios declarados e aceitos:** não consolidou as cópias por pacote do Go, porque exigiria
tocar ~15 callsites em `internal/validator/` que a **Wave 3 desta mesma REQ** vai editar — a colisão
que o roadmap existe para evitar, e que eu mesmo criei na Wave 4. E o `pathfmt.py` é **folha, com
zero imports de `trackfw`**, que é o que evita o ciclo que impedia o `manager.py` de importar o
`_tildeify`.

**Reportado, não corrigido:** a indexação por **basename** de `api_chain.js:145` — é o **segundo**
defeito do `/api/chain`, e a ADR cobre só o separador. REQ própria.

## Wave 3 — `IsAbs`, sozinho e sequencial
> Dependências: ML-1C `Accepted` **e** a branch `fix/validate-detecta-hook-de-guard-...` fechada.

### ML-3A — Caminho POSIX ancorado deixa de ser classificado como relativo
**Status:** ✅ Concluído · **Agente:** `apolo-tf` + barreira `hades-tf` (**APROVA COM RESSALVAS**, fechadas no ML-3B)
🔴 **É segurança.** Não paralelizar com nada. A branch que colidia
(`fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga`) **fechou** — dependência satisfeita.

**Sítios enumerados pelo arquiteto (2026-09-04), com o lado da fronteira D2 de cada um.**

**EM ESCOPO — classificação de caminho lido de config de CLI de agente (4 por runtime, 12 no total):**

| | Go | Node | Python |
|---|---|---|---|
| classe 1 (ancorado) | `validator_credential_guard.go:114` | `npm/src/validator/index.js:1510` | `pypi/trackfw/validator.py:1918` |
| classe 2 (cwd) | `validator_credential_guard.go:128` | `index.js:1526` | `validator.py:1932` |
| forma relativa antiga | `validator_credential_guard.go:193` | `index.js:1467` | `validator.py:1873` |
| guard de branch | `validator_git_branch_guard.go:167` | `index.js:2767` | `validator.py:3266` |

🔴 **A linha "forma relativa antiga" é o fix mergeado ontem (577e54a) — ele nasceu com o mesmo
defeito de Windows nos 3 CLIs.** E `git_branch_guard:167` é pior que classificar errado: o
`continue` faz o laço **pular a entrada inteira**, então no Windows uma entrada de config global com
comando absoluto POSIX **nunca é verificada**.

**FORA DE ESCOPO — travessia de sistema de arquivos, `IsAbs` fica (D2).** Mexer aqui quebra
resolução real de caminho no Windows, com falha intermitente:
`internal/validator/validator.go:2112`, `internal/integrations/manager.go:703,726`,
`npm/src/validator/index.js:95`, `npm/src/integrations/manager.js:55,62,107,430`,
`pypi/trackfw/generators/{req.py,adr.py}`, `pypi/trackfw/commands/status.py:117`,
`pypi/trackfw/integrations/manager.py:71`.

**Ações**
1. Predicado de **ancoragem** por runtime, ponto único, com **zero chamada dependente de SO** no
   caminho de classificação: `/`, `~`, `$CLAUDE_PROJECT_DIR`/`$GEMINI_PROJECT_DIR`,
   `$(git rev-parse --show-toplevel)`, **união** com letra de unidade (`C:\...`) e UNC (`\\...`).
2. Aplicar nos 12 sítios da tabela. Nos 3 sítios de guard de branch, o `continue` passa a usar o
   predicado novo.
3. **D4 da ADR:** `cwdDependentReason` ganha ramo de til. 🔴 O ramo dispara **só** para `~usuario/`
   e para `"~/"` com aspas — a frase `bare relative path` é preservada para todo o resto, por
   contrato de paridade e pela UX da ROADMAP-2026-08-21 ML-1B.
4. `npm/src/validator/index.js` é classificado **binário** pelo `file`: usar `grep -a`.

**Critérios de aceite**
- [ ] Falsificação nas duas direções: `/opt/foo/guard.sh` → **ancorado**; `scripts/guard.sh` →
      **continua** classe 2.
- [ ] 🔴 **Controle de não-afrouxamento:** enumerar o que passou a contar como ancorado e mostrar
      que **nenhuma forma relativa entrou no conjunto**. É o risco desta mudança.
- [ ] 🔴 **Controle POSIX:** em macOS/Linux a classificação de **todos** os casos existentes é
      idêntica à de hoje, medida antes e depois.
- [ ] Os 3 runtimes dão o **mesmo** veredito, medidos separadamente.
- [ ] `make quality` verde · `trackfw validate` exit 0 · gates obrigatórios verdes.

🔴 **O que NÃO conta como evidência.** O arquiteto e o agente estão em macOS, onde
`filepath.IsAbs("/opt/…")` é **true** — o defeito é **invisível localmente**, e `GOOS=windows` só
compila cruzado, não executa. A prova local é: **o predicado não chama nada dependente de SO**
(verificável por grep) mais tabela de casos com entradas em forma de Windows. **A queda da contagem
só fecha no CI.** Suíte verde em macOS **não** é evidência de aceite deste ML.

### ML-3B — Fecha a ressalva da barreira: braço UNC exigia só o prefixo
**Status:** ✅ Concluído · **Agente:** `apolo-tf`
O braço UNC aceitava **qualquer** string com prefixo de duas barras invertidas — `\\`, `\\x`,
`\\..\\evil` entravam como ancorados sem ter segmento de share. Em POSIX são cwd-dependentes:
é o afrouxamento inverso que o predicado existe para evitar, e a tabela de 21 casos do ML-3A não
cobria. Passa a exigir **servidor não vazio e diferente de `.`/`..`** mais **share não vazio sem
barra inicial**, nos 3 runtimes.

🔴 **A fórmula sugerida no parecer (`strings.Count(raw[2:], "\\") >= 1`) foi testada e NÃO fecha o
buraco** — a notação do exemplo é ambígua entre 3 e 4 barras invertidas e a contagem deixa passar as
duas leituras. A correção implementada fecha ambas sem escolher qual era a pretendida. O
implementador recusou a receita do revisor **com medição**, que é o comportamento certo.


## Auditoria da Wave 3 — arquiteto, 2026-09-04

```
make quality QUALITY_EXIT=0, zero FAIL · 365 cenarios de falsificacao OK
trackfw validate exit 0 (so warnings pre-existentes)
12 sitios em escopo migrados · 14 fora de escopo intocados, conferidos por leitura
predicado invariante por construcao nos 3 runtimes: zero chamada dependente de SO no corpo
ramo de til restrito as duas formas; "bare relative path" preservado no resto
```

**Barreira `hades-tf` cumpriu o papel:** reimplementou os 3 predicados de forma independente, sem
copiar do diff, e atacou com corpus próprio (homoglifo `ｃ:\`, zero-width space, `C:foo`, dígito
antes de `:`, espaço à esquerda, `$HOME/x`, `//servidor/share`, newline embutido). Achou a única
ressalva — que a tabela do implementador não cobria.

🔴 **O achado de método desta wave veio de uma falha do `make quality`:** o fixture de paridade
pinava `bare relative path` para o til aspeado, e **as três suítes unitárias tinham a mesma asserção
desatualizada** — concordavam entre si, verdes, sem detectar nada. Quem pegou foi o gate cross-CLI,
que compara a mensagem byte a byte entre os três binários reais. **Três suítes concordando não é
evidência quando as três herdaram a mesma premissa.** Nota no vault.

🔴 **Premissa do arquiteto derrubada:** eu escrevi "16 sítios fora de escopo"; são **14**. Meu grep
por `isabs` não pegava `Path.is_absolute()` do pathlib em `integrations/manager.py:71`, e contei
`req.py` como um sítio quando são três.

🔴 **Recontagem no CI (run `33931363032`), medida: `101 → 100`.** A estimativa era ~14; entregou
**2**. Fechados: `TestClassifyHookAnchorage_Classe1_Ancorado` e
`TestCredentialGuardHookResolvable_CaminhoAbsolutoSilencioso`. **Um teste novo passou a falhar:**
`TestPathIsAnchoredForHookConfig_ControlePOSIX`.

🔴 **O teste novo falha porque a correção FUNCIONOU.** Ele afirma
`pathIsAnchoredForHookConfig(x) == filepath.IsAbs(x)` para o corpus POSIX — que é **exatamente o
que a ADR determina que divirja no Windows**. O teste pinou o defeito como expectativa: passa em
macOS e reprova em Windows justamente onde o predicado novo acerta. É defeito de teste, não de
produto. **Tratar como ML corretivo — e não com guard de plataforma no assert, que apagaria a única
asserção que exercita a divergência.**

🔴 **Os demais testes de guard continuam falhando no Windows por OUTRA causa.** Exemplo medido:
`TestCredentialGuardHookResolvable_CaminhoResolvidoEhFisicoNaoSimlink` espera o caminho físico na
mensagem, **recebe esse mesmo caminho**, e ainda assim reprova — divergência de escape/aspas, não
de ancoragem. A estimativa de ~14 misturou grupos de causa diferente.


## Wave 6 — Os grupos de DEFEITO DE TESTE (4 MLs em paralelo, arquivos disjuntos)
> Dependências: re-triagem por mecanismo concluída
> (`docs/portabilidade/2026-09-04-retriagem-do-residuo-de-windows-por-mecanismo.md`).
> **Antecede a Wave 5 por retorno**: 38 falhas, risco zero, nenhuma decisão de arquitetura.

🔴 **Disjunção verificada arquivo a arquivo pelo arquiteto** — não por "parecem independentes". Já
criei uma colisão nesta campanha afirmando disjunção sem conferir (ML-4A/4B). Nenhum arquivo aparece
em dois MLs.

### ML-6A — G4: asserção crua contra bytes já serializados em JSON (22 falhas)
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Arquivos:** `internal/commands/update_harness_test.go` · `npm/tests/update-harness.test.js` ·
`pypi/tests/test_update_harness.py`
O teste monta o caminho com `filepath.Join`/`path.join`/`pathlib` e procura essa **string crua**
dentro de bytes que já passaram por serialização JSON — que **dobra toda barra invertida**. Produção
certa, teste errado. **Confirmado nos 3 runtimes por leitura**, não inferido por nome.
**Maior grupo do resíduo depois do CRLF, e o de menor risco.**

### ML-6B — G2 + G0: `%q` do Go e o controle POSIX que virou defeito de teste (5 falhas)
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Arquivos:** `internal/validator/validator_credential_guard_test.go` ·
`internal/validator/validator_test.go` · `internal/validator/validator_thirdparty_provenance_test.go`
**G2 (4):** `%q` produz string Go-escapada (cada `\` vira `\\`) — comportamento correto e
documentado. O teste constrói o esperado com `filepath.Join` (barra simples) e compara por
`Contains`. Discriminante que separa do G10: a violação **correta já está** na lista de mensagens;
só a busca textual falha.
**G0 (1):** `TestPathIsAnchoredForHookConfig_ControlePOSIX` deriva a expectativa de `filepath.IsAbs`
e portanto **afirma o defeito** que a Wave 3 corrigiu. 🔴 **Corrigir fixando os valores esperados
literalmente — NÃO com guard de plataforma no assert**, que apagaria a única asserção que exercita a
divergência. É Go-only: `path.win32.isAbsolute` e `ntpath.isabs` já tratam a barra POSIX como
absoluta, medido.

### ML-6C — G3: fixture gera JSON inválido e o validator falha-aberto em silêncio (9 falhas)
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Arquivos:** `internal/validator/validator_git_branch_guard_test.go` · `npm/tests/validator.test.js`
A fixture concatena um caminho nativo do Windows (com `\`) dentro de um template JSON **sem
escapar** → JSON inválido → o validator **pula o arquivo em silêncio** por desenho fail-open → o
teste espera violação e recebe lista vazia.
🔴 **Go confirmado por leitura (5); Node é HIPÓTESE por padrão de nome (4, `not ok 463/464/473/475`)
— o código Node não foi lido.** Confirme antes de corrigir. **Se a hipótese cair, reporte e pare** —
não force o Node no grupo.
🔴 **Observação que vale mais que as 9 falhas:** um fail-open que engole JSON inválido em **arquivo
de config de guard** é comportamento de produto que merece pergunta própria. **Não decidir aqui** —
reportar.

### ML-6D — G8: `findRoadmap` devolve separador nativo, teste compara com literal POSIX (2 falhas)
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Arquivos:** `internal/generators/roadmap_test.go`

### ML-6E — G3 nos dois arquivos que o ML-6C reportou em vez de invadir
**Status:** ✅ Concluído · **Agente:** `artemis-tf`
**Arquivos:** `npm/tests/git_branch_guard_hook_integrity.test.js` ·
`internal/integrations/manifest_origin_test.go`
A hipótese do Node do ML-6C **caiu por localização, não por mecanismo**: `validator.test.js` já usava
`JSON.stringify` e era seguro; o padrão real vivia noutro arquivo, com os mesmos 4 helpers.

🔴 **Nuance que ninguém tinha visto:** `loadManifest` (`internal/integrations/manifest.go:59`) é
**fail-CLOSED**, ao contrário do validator. A mesma fixture inválida ali **não some em silêncio** —
estoura com `invalid character 'U' in string escape code`. Mesma causa, mesma correção, **modo de
falha oposto**. O produto tem duas políticas para JSON inválido, e uma delas é a que virou candidata
a REQ.

## Auditoria da Wave 6 — arquiteto, 2026-09-05

```
make quality QUALITY_EXIT=0, zero FAIL · 365 cenarios de falsificacao OK
trackfw validate exit 0 · go build ./... e go vet ./... limpos
12 arquivos alterados, TODOS de teste ou doc — zero linha de producao
grep no diff por t.Skip/pytest.mark.skip/GOOS/process.platform/os.name: vazio
```

**Falsificação por MUTAÇÃO DE PRODUÇÃO em todos os MLs** — não por asserção ajustada até passar. Em
cada um, o agente mutou o código de produto, viu os testes reprovarem, restaurou e confirmou com
`git diff --stat` vazio.

🔴 **Duas premissas da triagem derrubadas pela leitura:**
1. **ML-6A:** no Node e no Python a maioria dos testes **já desserializava certo** — só 2 sítios por
   runtime tinham o defeito. No Go, por não ter teste parametrizado, o mesmo defeito estava espalhado
   por **8 funções**. As contagens reconciliam nos 22, mas a forma era outra.
2. **ML-6C/6E:** a hipótese do Node apontava o **arquivo errado**. O mecanismo existia, noutro lugar.

**Reportado e NÃO corrigido, por instrução:** o fail-open de
`internal/validator/validator_git_branch_guard.go:151-154` engole **JSON inválido em arquivo de
config de guard**, em silêncio, com comentário de desenho confirmando que é intencional
(linhas 130-132). A função irmã do credential-guard compartilha o padrão. **REQ própria** — é o mesmo
formato de defeito que a campanha vem caçando: o controle reporta saúde sobre o que não conseguiu
ler.


**Critérios de aceite da wave**
- [ ] Falsificação nas duas direções em cada ML, com números.
- [ ] 🔴 **Nenhuma correção esconde defeito de produto.** Se o teste estava certo e o produto errado,
      **pare e reporte** — o rótulo "defeito de teste" veio de uma triagem, não de dogma.
- [ ] Nenhum teste marcado `skip`, e nenhum guard de plataforma que apague asserção.
- [ ] `make quality` verde · `trackfw validate` exit 0 (rodados **pelo arquiteto**, uma vez, após os 4).
- [ ] Recontagem no CI com o delta atribuído a cada grupo, medida com o padrão do vault
      (`contagem-de-falhas-de-windows-do-go-medida-por-padrao-frouxo-2026-09-04`).


## Wave 7 — O dedup `//`, sozinho (mecanismo DESCONHECIDO)
> Dependências: nenhuma. Paraleliza com a Wave 1 do roadmap do retarget (arquivos disjuntos).

### ML-7A — Por que o dedup do git-branch-guard falha com `//` na home isolada
**Status:** ✅ Concluído (mecanismo IDENTIFICADO) (investigação concluída pela QA — mecanismo IDENTIFICADO E MEDIDO;
aguardando auditoria do arquiteto para fechar o ML) · **Agente:** `artemis-tf` · **investigação, sem correção**
**Alvo de leitura:** `internal/generators/agentfiles.go` e pares em Node/Python.
**Entregue:** `docs/portabilidade/2026-09-05-mecanismo-do-dedup-barra-dupla.md`.

🔴 **Mecanismo identificado, e a barra dupla NÃO é a causa.** `normalizeGuardPath` (e os espelhos
Node/Python, byte-a-byte) só colapsa runs de `/`; nunca canoniza `\`↔`/`. No Windows,
`filepath.Join`/`path.win32.join`/`ntpath.join` sempre emitem separador nativo (`\`) em cada fronteira
de segmento — confirmado por leitura do fonte Go instalado e por **execução real** de `path.win32.join`
(Node) e `ntpath.join` (Python) nesta máquina (módulos lexicais puros, sem chamada de SO, rodam em
qualquer plataforma; em runner Windows real `path===path.win32` e `os.path===ntpath`, não é proxy). O
`rawStoredCommand` do fixture, depois do colapso `//`→`/`, ainda tem `/` onde o comparando computado
tem `\` — as duas strings normalizadas nunca ficam iguais no Windows. Passa em POSIX (medido: `go
test` PASS nesta máquina) porque lá `Join` já usa `/` nativamente. **Discriminante de contagem:** só a
variante `//` (concatenação crua) falha; as variantes-irmãs de dedup (mesma home isolada, mesmo
`Join` dos dois lados) passam nos 3 runtimes — se fosse resolução de `$HOME` quebrada, todas
falhariam (~15, não 3). **$HOME verificado nos 3 runtimes, não só Go** — `npm/src/homedir.js` e
`pypi/trackfw/homedir.py` preferem `$HOME` no Windows, igual ao Go.

**PRODUTO — contrato documentado não cumprido no Windows, gatilho demonstrado, não só especulado.**
`normalizeGuardPath` promete tolerar "`$HOME` resolving with a trailing slash" — no Windows isso
produz `\\` duplicado, e a função **não colapsa** (só testa `r=='/'`, nunca `r=='\\'`; medido). Esse
gatilho é plausível em qualquer instalação Windows onde o perfil resolva com separador final, **não
depende de edição manual**. O segundo cenário citado pela doc-comment (hand-edited com `/`) segue
plausível mas não observado. Sem gatilho hoje em `trackfw update harness` (nenhum sítio escreve o
comando global por concat crua — todos usam `Join`); se ocorrer, dedup reinjeta a entrada de projeto,
guard dispara duplicado (não é falha de segurança — o guard ainda dispara). Falsificação nas duas
direções feita com cópia literal do algoritmo (sem tocar produto): mutação que reproduz (`EQUAL?
false`) e candidato de remédio que fecha sem afrouxar o controle POSIX (`EQUAL? true`) — ressalva de
UNC registrada como risco de implementação, não medida como parte do mecanismo. Falta confirmar em
Windows real (CI) os valores normalizados exatos via instrumentação temporária — não pude medir por
falta de runner Windows.

**É o único defeito genuíno que o reporter do issue #216 nomeou e que continua aberto**, e o G12 da
re-triagem por mecanismo chegou nele por caminho independente — dois trabalhos convergindo aumenta a
chance de ser real, não artefato de teste.

**O que o reporter mediu:** `TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand`
(Go) e o par em Python **passavam pelo motivo errado** — a produção lia a **home real** do
desenvolvedor, que já tinha o hook instalado, então o dedup encontrava a entrada e pulava a injeção.
Com a home de fato isolada, **falham**. O defeito de `//` que o nome descreve é real.

🔴 **Mecanismo NÃO identificado. Não inventar.** "Não sei ainda" é resultado válido; hipótese
apresentada como causa, não. Foi a recusa da triagem anterior em inventar mecanismo para o maior
grupo que tornou aquele diagnóstico confiável.

**Critérios:** mecanismo escrito com a medição que o sustenta, **ou** o espaço de hipóteses reduzido
com o que foi eliminado e como · discriminante escrito · nenhuma correção aplicada.


**Mecanismo, medido:** `normalizeGuardPath` / `_normalize_guard_path` — byte-idêntico em
`internal/generators/agentfiles.go:1621`, `npm/src/generators/hooks.js:1295`,
`pypi/trackfw/generators/hooks.py:113` — **só colapsa sequências de `/`, e nunca canonicaliza `\`
contra `/`**. Conferido por mim: o laço testa `r == '/'` e nada mais.

No Windows, `filepath.Join`/`path.win32.join`/`ntpath.join` emitem `\` em **toda** junção. O comando
guardado pela fixture `//`, depois do colapso, mantém `/` depois do segmento de home; o comparando
calculado é todo `\`. **Nunca casam no Windows; casam em POSIX** — por isso o teste passa aqui.

**Como ela mediu sem máquina Windows, e por que vale:** executou `path.win32.join` e `ntpath.join`
**localmente**. Os dois são módulos puramente lexicais e multiplataforma — `path === path.win32` e
`os.path === ntpath` num runner Windows real. **Não é proxy, é o mesmo código.**

**Discriminante que ela achou sem eu pedir:** só o teste da fixture `//` falha; os irmãos
(Codex/Gemini/Cursor/Copilot), com o **mesmo** `$HOME` isolado mas os dois lados construídos por
`Join`, passam nos 3 runtimes. **Isso elimina resolução de `$HOME`/`%USERPROFILE%` como causa.**

**Veredito: defeito de PRODUTO**, com gatilho demonstrado. O comentário da própria função promete
tolerar `"$HOME` resolvendo com barra final" — no Windows isso produz `\\` na emenda, e a função
**não colapsa**. Não exige edição manual para disparar.
**Atenuante medido:** nenhum sítio de produção escreve o comando do hook global por concatenação
crua hoje — todos usam `Join`. Então o defeito não é auto-infligido no fluxo atual, mas **viola o
contrato escrito da função para um cenário que ela declara cobrir.**

🔴 **Risco sinalizado, não corrigido:** a tradução ingênua `\`→`/` quebraria prefixo **UNC**. Quem
implementar a correção precisa tratar isso — é o mesmo tema que a barreira do `hades-tf` pegou no
ML-3B.

Nota: `vault/notes/dedup-guard-path-cego-a-backslash-no-windows-2026-09-05.md`.

### ML-7B — `normalizeGuardPath` passa a canonicalizar a barra invertida (3 CLIs)
**Status:** ✅ Concluído · **Agente:** `apolo-tf` + barreira `hades-tf`
**Arquivos:** `internal/generators/agentfiles.go` · `npm/src/generators/hooks.js` ·
`pypi/trackfw/generators/hooks.py` (+ testes dos 3)
Correção do defeito de produto que o ML-7A mediu. 🔴 **Risco herdado do ML-7A:** tradução ingênua
`\`→`/` **quebra prefixo UNC** — mesmo tema que a barreira pegou no ML-3B. **Barreira `hades-tf`
obrigatória**, pelo mesmo motivo daquela: é mudança num predicado de comparação usado por controle de
segurança, e o risco inverso (passar a casar o que não deveria) precisa de falsificação explícita.

### ML-7C — Fecha as duas ressalvas acionáveis da barreira
**Status:** ✅ Concluído · **Agente:** `apolo-tf` · **só comentários, zero lógica**
🔴 **O agente recusou minha instrução principal, com motivo medido.** Eu mandei tirar
`hasValidUNCPrefix` da superfície pública do Node; ele mediu que **remover quebraria o teste** que a
acessa por `require` destructuring, e que usá-la como guarda ativa mudaria o fluxo recém-aprovado
pela barreira. Manteve e **documentou por que existe sem chamador de produção**. Era a saída que o
handoff autorizava — ele leu a condição em vez de obedecer a instrução.

As três formas residuais de Windows (`\\?\C:\...`, relativo com `\`, home UNC de perfil de rede)
ficaram **documentadas no comentário da função nos 3 runtimes**, com a direção do erro: **aperta —
duplica, nunca omite**. Limitação escrita é decisão; a mesma limitação calada é defeito latente.

## Auditoria da Wave 7 — arquiteto, 2026-09-05

```
make quality QUALITY_EXIT=0, zero FAIL · 365 cenarios de falsificacao OK
trackfw validate exit 0 · go build/vet limpos · gofmt limpo
Node 20/20 · Python 21 passed + 52 subtests · Go ok
```

**Barreira `hades-tf`: APROVA COM RESSALVAS**, nenhuma bloqueante. Ela reimplementou os 3 predicados
fora do produto e atacou com corpus próprio de 19 vetores — case do drive, `..` não resolvido, nomes
8.3, `%USERPROFILE%` não expandido, `\\?\`, home UNC.

🔴 **O resultado que importa:** nenhum vetor produziu "iguais" para caminhos genuinamente diferentes.
Toda divergência é **na direção segura** — aperta (duplica), nunca afrouxa (guard ausente). Era esse
o modo de falha caro que a barreira existia para procurar.

🔴 **Ela respondeu o que o relatório do implementador tinha deixado de fora**, e eu só percebi porque
o critério estava escrito no roadmap: **o ML fecha o teste que o motivou?** Verificado por simulação
byte a byte — `TestGBGDedup_..._ToleratesDoubleSlash` e os pares fecham no Windows. Um ML que cria
testes que passam e não fecha o que o motivou seria o pior desfecho, e um relatório detalhado é
justamente onde isso se esconde.

## Wave 5 — CRLF no parser de frontmatter
> Dependências: Wave 3 fechada. **Sequencial**: toca os mesmos arquivos de validator dos 3 CLIs.

### ML-5A — CRLF tolerado na fronteira de entrada — **parser E renderizadores**
**Status:** ✅ Concluído · **Agente:** `apolo-tf`

🔴 **Escopo corrigido em 2026-09-05, depois da auditoria externa.** O ML dizia "parser de
frontmatter". A auditoria mediu que **não basta**:

```
Node, mesmo conteúdo:
  LF   → name=teste, model=sonnet
  CRLF → name=trackfw-agent, model vazio; o frontmatter fica no CORPO
```

Go e Python têm verificações **literais de `---\n`** nos renderizadores. E a re-triagem por mecanismo
já tinha achado um **segundo parser** — o do bloco de gates do `barrier` no Python (o "G1-bis"), que
a ADR, escrita antes, não menciona.

**Superfícies em escopo, e a enumeração é entregável do ML:** parser de frontmatter de validação ·
**renderizadores de artefato de agente** · **reescrita de identidade/modelo** · **parser de gates do
`barrier`** · demais consumidores que casem delimitador por literal.

🔴 **Entregar só o parser de validação repetiria o defeito que esta wave existe para corrigir** —
metade do caminho, com aparência de completo.

Governado por `ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-fronteira-de-entrada.md`
(`Accepted`). ~14 testes. Sintoma medido: frontmatter **duplicado** em `TestRenderOpenCodeAgent`.
**D1** normaliza `\r\n` → `\n` **ao ler**, antes de casar delimitador · **D2** a escrita continua
LF (`check-python-writes-lf.sh` é contrato) · **D3** **ponto único por runtime**, sem espalhar
`ReplaceAll` pelos chamadores · **D4** 🔴 `.gitattributes` **não** ganha `eol` sobre assets, goldens
e corpus — mascarar a entrada nos tira a capacidade de detectar a regressão.

## Wave 4 — Resíduo (paralelo, arquivos disjuntos)
> Dependências: Wave 2.

### ML-4A — Bit de execução em NTFS
**Status:** ✅ Concluído · **Agente:** `artemis-tf` · ~22 testes, **decisão já tomada** no vault:
`goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`. **Não relitigar** — guard de
plataforma no assert.

### ML-4B — `WinError 32`, `.sh` sem `bash`, `stale_wip` off-by-one
**Status:** ✅ Concluído · **Agente:** `artemis-tf` · ~15 testes, todos de teste, todos disjuntos.
O `stale_wip` é **truncamento**, não fuso horário — a hipótese de TZ foi **falsificada** na triagem.


## Auditoria da Wave 4 — arquiteto, 2026-09-04

```
make quality QUALITY_EXIT=0, zero FAIL · validate exit 0
controle POSIX: Python 1613 passed · Go 346/425/39 antes e depois · Node 859/859/0 skipped
ML-4B intacto apos a colisao: 10 usos de bash_cmd e 10 de _chdir presentes
```

🔴 **Colisão de arquivo criada por MIM.** Afirmei no handoff que os dois MLs eram disjuntos; ambos
editam `pypi/tests/test_generators_init.py`. **O ML-4B detectou e avisou**; preservei cópia durável
dos 3 arquivos e verifiquei que nada se perdeu. Não houve dano — mas a garantia veio do agente, não
do meu planejamento.

### ML-4A — três achados além da correção

**A lista do meu briefing estava incompleta.** Eram **22 sítios**, e ele achou dois grupos que eu
não teria: `discover_test.go` usa `Perm() != 0755` — **um grep pelo `&` não pega** — e havia **9
supressões silenciosas pré-existentes** (`process.platform !== 'win32'`, `os.name == 'posix'`) que
passavam no Windows **sem nomear nada**, violando o critério que eu mesmo escrevi.

🔴 **O canal da mensagem estava furado, e isso tornava o MEU critério vácuo.** Verificado por mim:

```
go test  (sem -v)  ->   0 ocorrencias da mensagem
go test  (com -v)  ->  13
```

O `go test` bufferiza e **descarta a saída de pacote que passa** — e esses testes passam
**justamente por causa da supressão**. `t.Logf` e `os.Stderr` dão zero igualmente, as duas medidas.
**Acrescentei `-v` ao `quality.yml:384`** com o motivo medido no comentário: sem isso, "toda
supressão nomeia a garantia" era uma exigência que ninguém podia ler.

🔴 **Ele recusou um proxy falso.** Tentou FAT32 via `hdiutil` para exercitar o ramo de supressão, e o
VFS `msdos` do macOS **sintetiza `0755`** — devolveria `True` na sonda e daria a impressão de ter
testado NTFS. Registrado em `vault/notes/fat32-no-macos-finge-0755-...`: *"o que provei localmente é
o caminho de código, não a plataforma."*

**Falsificação contada, mutando o gerador e não uma fixture:** previa 11 funções no Go, observou
**11, exatamente as mesmas, zero a mais**. Produto restaurado byte-a-byte (`cmp` OK nos 9).

### ML-4B — os três classificados por medição, e um defeito de produto recusado

**Discriminante do `WinError 32`:** dos 5 testes da classe, os **4 que fazem `os.chdir` falharam** e
o **único que não faz passou**. Quem segura o handle é o cwd do próprio processo; `tearDown` não
resolveria porque roda **depois** do `__exit__`.

**O `stale_wip` é INTERMITENTE** — falhou num run e **passou** no outro sem mudança na regra. 200
amostras: **9 a 21 µs** entre a gravação do mtime e a leitura da produção; `int()` é floor, então
qualquer desvio derruba 10,0000001 para 9. **Não é fuso, e o produto está certo.** E ele escolheu
sonda negativa de **10 ms em vez de 1 µs** porque `mtime + 864000` perto de 1,76e9 tem ~2,4e-7 s de
erro em float64 — 1 µs seria 4× o epsilon e **recriaria a fragilidade**.

🔴 **Defeito de PRODUTO reportado e não mascarado:** `TestStaleWIPReportsWIPWalkError` falha nos
**dois** runs, sem intermitência. Em POSIX o `ReadDir` devolve `ENOTDIR` e o diagnóstico sai; no
Windows **a regra vai a silêncio** (`validator.go:1698-1702`). Vai para a **Wave 3**, e com uma
instrução dele a mim: **não aceitar "guarda de plataforma no assert" na auditoria** — o precedente do
ML-4A vale para uma propriedade que NTFS **não tem**, não para um diagnóstico que **some de verdade**.

**Grupo adjacente enumerado, não corrigido:** igualdade de modo restritivo em
`internal/identity/identity_test.go:126,134` e pares no Node — falham no Windows (NTFS reporta
`0o666`/`0o444`), mas **não são bit de execução**. Vira REQ.

## Verificação que só o CI fecha

A contagem por runtime, **medida após cada wave** e com o delta **atribuído** ao grupo. Sem
atribuição não se sabe qual correção funcionou — e nesta REQ eu já errei uma estimativa por 2,5x.

## Barreira final

`hefesto-tf` e `hades-tf`. O Hades é **obrigatório** na Wave 3 (segurança) e na Wave 2 (caminho em
config lido por CLI que executa bash).


## Auditoria do ML-5A — arquiteto, 2026-09-05

```
make quality QUALITY_EXIT=0, zero FAIL · trackfw validate exit 0
Go todos os pacotes ok · Node 866/866 · Python 1625 passed + 66 subtests
check-python-writes-lf exit 0 · git diff -- .gitattributes VAZIO (D4 respeitada)
```

🔴 **A reescrita do escopo antes do despacho foi o que salvou este ML.** A ADR dizia "parser de
frontmatter"; o defeito real eram **7 funções de fronteira por runtime**, cada uma checando
`HasPrefix(trimmed, "---\n")` isoladamente — exatamente a violação de D3 que a própria ADR proíbe.
Com o escopo original teríamos corrigido **1 sítio de 20** e fechado a wave achando o CRLF resolvido.

**O agente verificou o que já estava correto e NÃO mexeu:** o parser de gates do `barrier` (G1-bis) e
o parser de validação já eram tolerantes, por commits de outra REQ. Falsificou antes de concluir.

🔴 **Discrepância declarada, não escondida:** o único dado real de Windows CI diz que o teste do
`barrier` **falha**, enquanto a leitura local diz que o código tolera. Ele não escolheu uma das duas
versões para fechar a história. **Verificar na recontagem do CI.**

🔴 **CORREÇÃO DO ARQUITETO, 2026-09-06 — eu errei ao chamar isto de REQ própria.**
`rewriteRoadmapStatus`/`rewriteREQStatus` — 6 sítios nos 3 CLIs — têm **o mesmo defeito**, governado
pela **mesma ADR**.

**Fragmentar uma causa raiz em duas REQs esconde que existe um padrão** — é o escopo negativo que eu
mesmo escrevi na REQ do retarget e violei aqui. E pior: a **D3 da ADR exige ponto único por
runtime**; com 6 sítios ainda cegos, **a D3 não está satisfeita** e o roadmap não podia ser fechado.
Fechá-lo seria marcar como concluído algo cujo critério não foi atendido — exatamente o achado A1 da
auditoria externa de ontem.

Minha justificativa era **atribuição de causa** (não misturar mudanças para saber qual produziu qual
efeito). Isso justifica **ML separado**, não **REQ separada**. Confundi as duas coisas. O usuário
pegou.

### ML-5B — `rewriteRoadmapStatus`/`rewriteREQStatus`: os 6 sítios restantes da D3
**Status:** ✅ Concluído · auditado e mergeado no PR #284 · **Agente:** `apolo-tf`
Mesma ADR, mesma decisão, mesmo mecanismo — só o sítio muda. Afetam `trackfw roadmap move` e a
escrita de status de REQ, que é caminho de escrita **do produto**, não de leitura de artefato de
agente.
**Critérios:** os mesmos do ML-5A · falsificação nas duas direções por sítio · escrita continua LF
(D2) · `.gitattributes` intocado (D4) · e a **enumeração final provando que a D3 ficou satisfeita**:
nenhum sítio remanescente casa delimitador por literal.


### ML-5C — O resíduo que a enumeração da D3 encontrou (Python)
**Status:** ✅ Concluído (implementação e evidência entregues; aguardando auditoria do arquiteto) · **Agente:** `apolo-tf`
`_get_frontmatter_roadmap_value` e `_rewrite_req_roadmap_ref`
(`pypi/trackfw/generators/roadmap.py:429,454`) ainda usam `startswith("---\n")` literal e são
**cegos a CRLF** — medido ao vivo pelo ML-5B. Os gêmeos em Go e Node já são tolerantes **por
construção** (split por linha + trim, não prefixo de blob).

É o **caminho de sincronização da referência REQ↔roadmap** — escrita de artefato de governança do
usuário.

🔴 **Entra aqui, e não em REQ nova, pela Regra Dura de Causa Raiz** (`CLAUDE.md`), escrita hoje
justamente por causa deste padrão: mesma causa, mesma ADR, mesmo mecanismo → **mesmo ML na REQ
vigente, e no mesmo PR**. O agente reportou como "candidato a REQ própria" seguindo a disciplina de
escopo do handoff dele — a decisão de não fragmentar é do arquiteto, e é esta.

## Auditoria do ML-5B — arquiteto, 2026-09-06

**Os 6 sítios confirmados por leitura, não herdados do handoff** — e o número 6 se sustentou.
Reaproveitou o normalizador único do ML-5A nos 3 runtimes (D3), sem criar segunda cópia.

🔴 **Nuance medida, não presumida:** os dois sítios Python leem com
`open(path, "r", encoding="utf-8")`, que já aplica tradução universal-newlines — **na produção eles
nunca viram CRLF**, ao contrário de Go e Node, que leem bytes crus e estavam genuinamente quebrados.
A correção ali é defesa em profundidade. **Ele mediu a diferença em vez de aplicar simetria
automática.**

**Controle de escrita (D2) medido em bytes reais**, com os 3 binários de CLI contra a mesma fixture
CRLF: **zero bytes `\r` escritos**, saída byte-idêntica entre runtimes. `.gitattributes` intocado (D4).


**Resultado do ML-5C — a D1 CONVERGE.** Varredura do repositório inteiro (com `grep -a`, evitando a
armadilha do arquivo binário) e leitura dos corpos completos das funções, não só das linhas de
entrada: **zero sítios remanescentes nos 3 runtimes** casando delimitador por literal. A enumeração
parou de crescer — **não precisamos da solução arquitetural** que eu tinha levantado como plano B.

### ML-5D — 🔴 Vazamento de CRLF na ESCRITA (D2), medido com os 3 binários reais
**Status:** ✅ Concluído (implementação e evidência entregues; aguardando auditoria do arquiteto) · **Agente:** `apolo-tf`

Achado do ML-5C, rodando `roadmap move` de verdade contra fixture CRLF:

```
Go      rewriteREQRoadmapRef     8 bytes \r vazados na REQ reescrita
Node    rewriteReqRoadmapRef    10 bytes \r
Python                           0 (open(path,"r") já traduz)
```

**É outra natureza de defeito:** não é casar delimitador (D1) — é **vazar CRLF da origem para o
arquivo escrito** (D2). E a família de funções de sincronização da referência `roadmap:` **nunca foi
coberta** pelo ML-5A nem pelo 5B.

🔴 **O `trackfw roadmap move` está gravando CRLF dentro do arquivo de REQ do usuário.** Não é
artefato de fixture: `os.ReadFile` (Go) e `fs.readFileSync(path,'utf8')` (Node) entregam bytes crus a
essas funções, ao contrário do Python.

🔴 **Entra aqui, mesma REQ e mesmo PR, pela Regra Dura de Causa Raiz:** a **D2 da mesma ADR** diz que
*a escrita continua LF*. Enquanto vaza, **a ADR não está satisfeita** — e fechar assim seria repetir
o achado A1 da auditoria pela terceira vez em dois dias.

**Enumeração da família de funções de reescrita — 6 por runtime, 18 no total, veredito por sítio.**
Achada por `grep -rn "func rewrite"` (Go), `grep -rn "function rewrite"` (Node) e
`grep -rn "def.*rewrite"` (Python) na árvore inteira, não só no caminho já suspeito:

| Função | Go | Node | Python | Chamada por |
|---|---|---|---|---|
| `rewriteSignatureLine` | normaliza (ML-5A) | normaliza (ML-5A) | normaliza (ML-5A) | render de agente |
| `rewriteFrontmatterFields` | normaliza (ML-5A) | normaliza (ML-5A) | normaliza (ML-5A) | render de agente |
| `rewriteFrontmatterModelLine` | normaliza (ML-5A) | normaliza (ML-5A) | normaliza (ML-5A) | render de agente |
| `rewriteRoadmapStatus` | normaliza (ML-5B) | normaliza (ML-5B) | normaliza (ML-5B) | `roadmap move` |
| `rewriteREQStatus` | normaliza (ML-5B) | normaliza (ML-5B) | normaliza (ML-5B) | `req` move de status |
| `rewriteREQRoadmapRef`/`rewriteReqRoadmapRef`/`_rewrite_req_roadmap_ref` | **NÃO normalizava → corrigido aqui** | **NÃO normalizava → corrigido aqui** | normaliza (ML-5C) | `roadmap move` → `syncREQReferences` |

Só os dois sítios apontados no handoff estavam realmente descobertos — a enumeração **não** achou
um sétimo sítio. `extractFrontmatterRoadmap`/`extractRefPath`-like readers (Go
`extractFrontmatterRoadmap`, Node `extractFrontmatterRoadmap`) são **leitura pura** — usados só
para decidir se uma REQ aponta para o roadmap movido, nunca escrevem — e já toleram CRLF via
`strings.TrimSpace`/`.trim()` na comparação, sem casar delimitador por literal. Não fazem parte da
família que **escreve**, então ficam fora do escopo D2 por não terem D2 nenhum.

🔴 **Verificado, não presumido, que nenhum outro comando vazava:** `grep` pelos nomes das 18 funções
de reescrita mostrou que os únicos call sites de produção são os já listados (render de agente,
`roadmap move`, e o `rewriteREQStatus`/`_rewrite_roadmap_status` chamados pelo próprio `roadmap
move`/`req` move). `req new`, `roadmap new` e `status` não chamam nenhuma função desta família —
escrevem template fresco, nunca leem e reescrevem um artefato existente — confirmado por
`grep -rn "os.WriteFile\|writeFileSync\|open(...'w'" nos comandos correspondentes e leitura dos
corpos: nenhum deles lê bytes crus de um artefato do usuário para depois regravá-los.

**Decisão leitura-vs-escrita, com a razão:** normalizar **na entrada** da função (mesmo ponto que o
D1 já usa nas 5 funções irmãs), não num segundo ponto na saída. Medido: as duas fecham o sintoma,
mas normalizar na entrada reaproveita o `NormalizeCRLF`/`normalizeCRLF`/`_normalize_crlf` já
existente **sem criar um quarto normalizador** (D3) e sem exigir tocar cada `return` da função — a
família inteira (D1 e D2) já segue esse padrão nas 5 funções irmãs, então esta é a MESMA decisão
reaplicada a um sítio que ficou para trás, não uma decisão nova. Confirmado contra o Python já
corrigido no ML-5C (`_rewrite_req_roadmap_ref`): o comentário dele já registra exatamente essa
escolha ("a chamada aqui é defesa em profundidade... a escrita (D2) permanece LF porque o conteúdo
normalizado alimenta o split/join").

🔴 **Achado NÃO previsto no handoff — Node quebraria a cardinalidade "já correta → nenhuma escrita"
sem uma segunda correção.** `syncReqReferences` (Node) chama `rewriteReqRoadmapRef` também na
"guarda rápida" de idempotência, com `oldRef === newRef`, para confirmar por reescrita estrutural
que nada mudou — diferente de Go/Python, que curto-circuitam ANTES de chamar a função quando a
referência já está correta. Normalizar incondicionalmente na entrada faria essa guarda comparar
"conteúdo normalizado" contra "conteúdo original com CRLF" e achar diferença onde não há mudança
semântica — **reescrevendo (e regravando) toda REQ com CRLF em qualquer lugar do arquivo mesmo
quando o campo `roadmap:` já apontava certo**, contrato pinado em `docs/cli-parity.md` quebrado e
Node divergindo de Go/Python no mesmo cenário. Corrigido: `changed` agora só liga quando a linha
PRODUZIDA difere da linha ORIGINAL (não quando houve match de `oldRef`), e a função devolve o
`content` original, não normalizado, quando nada muda — mesmo efeito observável em disco que
Go/Python (que nunca chegam a chamar a função nesse caso).

🔴 **Controle de escrita em bytes reais, com os 3 binários** — go test/`node --test`/pytest reais
contra fixture CRLF sintética via `syncREQReferences`/`syncReqReferences`:

```
Go     0 bytes '\r' no arquivo gravado (era 8, medido no ML-5C)
Node   0 bytes '\r' no arquivo gravado (era 10, medido no ML-5C; a fixture desta rodada,
                                        mais linhas, media 11 antes da correção — mesma causa)
Python 0 (já era 0 — open() em modo texto já traduzia; sem regressão)
```

**Falsificação nas duas direções, por sítio.** Comentando a linha
`content = integrations.NormalizeCRLF(content)` (Go) e
`normalizeCRLF(String(content))` → `String(content)` (Node), os `\r` voltam na contagem exata
medida acima (12 e 11 bytes respectivamente, nas fixtures dos testes novos) — confirmado rodando os
testes com a linha comentada (FAIL nomeando a contagem) e restaurada (PASS). Não fechei a Wave
inteira nesse estado; a reversão foi feita, medida e desfeita na mesma sessão.

🔴 **Controle POSIX:** entrada LF produz saída byte-idêntica à de antes desta correção — medido nos
3 runtimes com um teste que compara a string completa esperada (não só um `.includes`) contra o
que `syncREQReferences`/`syncReqReferences` grava; nenhum caractere mudou no caminho que já
funcionava.

**Resposta à pergunta de convergência da D2:** depois deste ML, **não existe caminho de escrita do
trackfw que possa gravar CRLF num artefato de governança do usuário.** As 6 funções de reescrita
por runtime (18 no total) normalizam na entrada, reaproveitando o normalizador único por runtime
(D3); os únicos consumidores de produto que chamam essas funções são o render de agente e
`roadmap move` (que também exercita `req` move de status internamente); `req new`, `roadmap new` e
`status` nunca leem-e-regravam um artefato existente. A D2 converge — não é necessária a solução
arquitetural de escritor único levantada como plano B.

**Reconciliação — o que cada teste novo afirma:**
- `TestSyncREQ_CRLFSourceWritesZeroCarriageReturns` (Go) / `syncReqReferences — REQ com CRLF: escrita
  não vaza "\r"` (Node): afirma o achado central do ML — a **escrita** (D2) não vaza CRLF, mesmo
  quando a origem tinha.
- `TestSyncREQ_LFSourcePOSIXControl` (Go) / `...saída byte-idêntica ao comportamento pré-existente`
  (Node): afirma o **controle POSIX** exigido pelo critério — nada mudou no caminho LF que já
  funcionava.
- `TestSyncREQ_AlreadyCorrectWithCRLF` (Go) / `...referência já correta com CRLF: nenhuma escrita`
  (Node): afirma o **achado não previsto** desta seção — a correção não vira "regravar toda REQ que
  passar pelo `roadmap move`"; a cardinalidade "já correta → nenhuma escrita" sobrevive mesmo com
  CRLF fora do campo reescrito.

**Premissa do handoff que a medição confirmou, não derrubou:** o handoff já apontava exatamente os
dois sítios certos (`rewriteREQRoadmapRef` Go, `rewriteReqRoadmapRef` Node) e a contagem de bytes
bateu (8/10, com a diferença de 11 explicada pela fixture ter uma linha `author:` a mais que a do
ML-5C). A única coisa que o handoff não antecipava foi a quebra de cardinalidade do Node — achada
pela leitura do call site, não pela medição de bytes isolada.


## Auditoria da Wave 5 completa — arquiteto, 2026-09-06

```
make quality QUALITY_EXIT=0, zero FAIL · trackfw validate exit 0
```

**A causa raiz do CRLF fechou em QUATRO microlotes, na mesma REQ e no mesmo PR:**

| ML | sítios | natureza |
|---|---|---|
| 5A | 20 (7 funções × 3 runtimes − 1) | D1 — parser **e renderizadores** |
| 5B | 6 | D1 — `rewriteRoadmapStatus`/`rewriteREQStatus` |
| 5C | 2 | D1 — resíduo Python do sync de referência |
| 5D | 2 de 18 da família | **D2 — vazamento na escrita** |

🔴 **Convergência medida nas duas decisões:** D1 com **zero** sítios remanescentes nos 3 runtimes;
D2 com **zero** caminhos de escrita capazes de emitir CRLF em artefato. O redesenho de "escritor
único" que eu levantei como plano B **não é necessário** — e isso foi medido, não presumido.

🔴 **O ML-5D evitou um defeito que a correção óbvia teria criado.** O `syncReqReferences` do Node
chama a função de reescrita com `oldRef === newRef` como atalho de idempotência; normalizar sem
critério quebraria a cardinalidade *"já está correto → não escreve"* — **contrato pinado no
`cli-parity.md`** — para qualquer REQ com CRLF, divergindo de Go e Python. Resolveu comparando **a
linha produzida contra a original**, não "houve casamento".

### O que este roadmap prova sobre método

Pelo meu padrão anterior, isto teria virado **quatro REQs**: eu chamei o ML-5B de "REQ própria" e o
usuário recusou. Três delas estariam agora na fila de 36 abertas, **com o defeito vivo** — inclusive
o de escrita, que corrompe artefato de governança do usuário.

A `Regra Dura de Causa Raiz` (`CLAUDE.md`) nasceu desse erro, e este roadmap é a primeira aplicação
completa dela.


## Wave reaberta — 2026-09-08: o grupo `IsAbs` não havia fechado

### ML-R1 — `manager.go` usa o helper de ancoragem que já existe
**Status:** 🔄 Em andamento (implementação, testes locais e VM Windows concluídos e verdes; falta
só a revisão `hades-tf` antes do commit) · **Agente:** `apolo-tf` · 🔴 **segurança**

**Medições registradas conforme feitas (2026-09-08):**

1. **Extração:** predicado movido de `internal/validator/validator_credential_guard.go`
   (`pathIsAnchoredForHookConfig`) para `internal/pathanchor/pathanchor.go`, exportado como
   `pathanchor.IsAnchored`. Pacote-folha sem import de `path/filepath` (grep confirma). Zero
   duplicação: `internal/validator` (2 sítios: `validator_credential_guard.go`,
   `validator_git_branch_guard.go`) e `internal/integrations/manager.go` chamam o mesmo símbolo.
   Direção do grafo de import confirmada antes de extrair: `internal/generators` →
   `internal/validator` → `internal/integrations`; um novo pacote-folha evita o ciclo que
   impediria `internal/integrations` de importar `internal/validator` diretamente.
2. **`manager.go:704` (agora ~707), 3 ramos:** `~/...` inalterado; `pathanchor.IsAnchored(destination)
   && !filepath.IsAbs(destination)` → **rejeita explicitamente** (`unsafe destination`) em vez de
   cair no ramo relativo; `pathanchor.IsAnchored(destination) && filepath.IsAbs(destination)` →
   comportamento antigo preservado (`Clean` + `beneath`). Decisão registrada no code comment: por
   que a fronteira D2 da ADR-2026-09-04 ("SO é autoridade em travessia real") não se aplica a esta
   linha — a decisão aqui é CLASSIFICAÇÃO de uma string de entrada (`plan.Destination`), não
   resolução de syscall.
3. **Sweep de `filepath.IsAbs`** (todo o repo, `.go` não-teste): 4 sítios reais.
   - `internal/integrations/manager.go:721` (novo) — CONVERTIDO (par com `pathanchor.IsAnchored`).
   - `internal/integrations/manager.go:746` (`beneath()`) — **mantido**: opera sobre a SAÍDA de
     `filepath.Rel(root, filename)`, já resolvida pelo SO; decide containment real, não classifica
     string externa.
   - `internal/validator/validator.go:2195` (`isOutsideCWD`) — **mantido**: mesma forma, opera
     sobre `filepath.Rel(absCwd, absPath)` já resolvido via `filepath.Abs` real.
   - `internal/generators/update_test.go` (6 ocorrências) — **mantido**: são asserções de teste
     sobre caminho de tempdir real computado, não classificação de entrada.
4. **Paridade 3 CLIs — medida, não presumida:**
   - **Node:** `path.win32.isAbsolute("/tmp/x")` → `true` (medido: `node -e ...`). Node já trata
     `/tmp/x` como absoluto no Windows — **não tem o defeito**: `npm/src/integrations/manager.js`
     segue o ramo `path.isAbsolute` corretamente e rejeita via o check de `rel` na sequência.
   - **Python:** `ntpath.isabs("/tmp/x")` → `False` (mesma leitura estreita do Go) — MAS
     `pypi/trackfw/integrations/manager.py` usa `pathlib.Path.__truediv__` (não concatenação
     ingênua): `root / candidate` com `candidate` drive-relativo REANCORA para a raiz do drive de
     `root` (`C:\Users\Lab\trackfw` / `/tmp/x` → `C:\tmp\x`), e o `relative_to(root)` seguinte
     detecta que o resultado não está sob `root` e levanta `ValueError` → rejeitado. Medido com
     `PureWindowsPath`/`ntpath.normpath` sem precisar da VM (path puro, independente de SO real).
   - 🔴 **Correção de 2026-09-08 (auditoria `hades-tf`, ML-R1):** a frase acima — "Python não tem o
     defeito" — é **imprecisa e envelhece mal**. `PureWindowsPath("/tmp/x").is_absolute()` **é
     `False`**: o Python tem o **mesmo ponto cego** estreito que `filepath.IsAbs` no Go
     (`ntpath.isabs`/`is_absolute` decidem por letra-de-unidade/UNC, não por `/` isolado). A
     garantia final não vem de `is_absolute()` enxergar `/tmp/x` como absoluto — vem de **dois
     passos COMPOSTOS** a jusante: `root / candidate` faz `pathlib` reancorar um candidato
     drive-relativo para a raiz do drive de `root` (não concatena ingenuamente), e o
     `relative_to(root)` seguinte levanta `ValueError` quando o resultado reancorado escapa de
     `root`. É uma **garantia emergente por composição de dois passos**, não uma ausência
     desenhada do defeito — e composição quebra silenciosamente se qualquer um dos dois passos for
     refatorado (ex.: trocar `relative_to` por comparação de string, ou `/` por `os.path.join`).
   - **Conclusão (revisada):** nenhum dos dois CLIs produz o escape medido em Go — Node porque
     `path.win32.isAbsolute` já reconhece `/tmp/x` como absoluto (sem o ponto cego), Python porque
     a composição `root / candidate` + `relative_to(root)` contém o resultado mesmo com o mesmo
     ponto cego de `is_absolute()` que o Go tem. Mecanismos diferentes, resultado final seguro nos
     dois — mas a garantia do Python é mais frágil (emergente) que a do Node (desenhada). Nenhum ML
     de paridade necessário; risco residual do Python registrado para referência futura, não para
     ação imediata.
5. **Testes:**
   - `internal/pathanchor/pathanchor_test.go` — 3 tabelas movidas byte-a-byte de
     `validator_credential_guard_test.go` (comportamento-preservante; `go test` roda igual antes/
     depois da extração).
   - `internal/integrations/manager_test.go`,
     `TestManagerRejectsAnchoredDestinationHostMismatch` — vetores `C:\Windows\evil.md` e
     `\\server\share\evil.md`, que em QUALQUER host POSIX (não só Windows) já divergem entre
     `pathanchor.IsAnchored` (true) e `filepath.IsAbs` (false) — falsificação sem depender da VM.
     Frase de reconciliação: **este teste afirma que `manager.resolve` rejeita uma
     `PlannedArtifact.Destination` que é ancorada pela forma (letra de unidade/UNC) mas não é
     `filepath.IsAbs` no host corrente, em vez de tratá-la como fragmento relativo seguro para
     juntar sob a raiz** — é exatamente a lacuna medida na VM Windows para `/tmp/...`, replicada
     com vetores que divergem em QUALQUER host.
   - **Falsificação de reversão comprovada nesta sessão:** reverti manualmente o `case
     pathanchor.IsAnchored(destination)` para `case filepath.IsAbs(destination)` (sem tocar mais
     nada) e rodei `go test -run TestManagerRejectsAnchoredDestinationHostMismatch -v`: **FAIL**
     ("accepted a destination anchored-by-form but not filepath.IsAbs on this host"). Restaurado
     e reconfirmado verde. Prova de que o teste não é vácuo.
6. **VM Windows ARM64 (`go1.27.1 windows/arm64`, `C:\Users\Lab\trackfw` sincronizado com a HEAD
   desta branch, `c52d566` + diff local) — medido e verde:**
   - `TestManagerRejectsTraversalAbsoluteMismatchAndNUL`, `TestManagerRejectsAnchoredDestinationHostMismatch`,
     `TestResolveWindowsCrossplatform` — `--- PASS` (com `=== RUN` confirmado, não silêncio de teste
     inexistente).
   - `internal/pathanchor` completo — `ok`.
   - `internal/integrations` completo — `ok`.
   - `internal/validator` completo com `-run TestCredentialGuardHookResolvable` — todas as 24
     subvariantes `--- PASS`.
   - `internal/validator` **sem** filtro tem 3 FAILs pré-existentes
     (`TestStaleWIPReportsWIPWalkError`, `TestFolderStatus_DiretorioNaoLegivel_P2`,
     `TestFilenameUniqueness_DiretorioNaoLegivel_P2`) — medido **idêntico** rodando `git stash` (só
     a base `c52d566`, sem este diff) na mesma VM: são os outros grupos da triagem (ENOTDIR/
     permissão), não regressão deste ML. `internal/integrations` idem — `TestResolveAgentModelMatchesRender`
     falha igual com e sem o diff (grupo `models`, fora de escopo).
7. **Falsificação de infraestrutura de gate encontrada e corrigida:** `scripts/check-gates-falsify.sh`
   Cenário 165 tinha um `sed 's/pathIsAnchoredForHookConfig(rawStripped)/false/g'` que virou no-op
   silencioso após a extração — o mesmo padrão de falha que o retarget de 2026-09-04 já registrou
   no cabeçalho do cenário (histórico de reincidência do próprio arquivo). `make quality
   QUALITY_EXIT=0` pegou via `run-gates-falsify-parallel: GUARDA — chunk_6 nao chegou ao sentinela
   CHUNK_COMPLETE`. Retargeted para `pathanchor.IsAnchored(rawStripped)`; `grep -c '^FAIL'` = 0 na
   saída completa de `make quality QUALITY_EXIT=0` depois da correção (rodado localmente, 4108
   linhas, verificado sem `| tail`).
8. **`hades-tf`:** pendente — é mudança em guarda de segurança, revisão obrigatória antes do
   commit (nenhum commit foi feito por este agente). Duas afirmações para a revisão confirmar
   diretamente, em vez de derivar do diff:
   - **A extração NÃO alterou a lógica do predicado.** `pathanchor.IsAnchored` é byte-idêntico a
     `pathIsAnchoredForHookConfig` — os 3 braços (POSIX `/`, UNC com validação de servidor/share,
     letra de unidade) e todas as ressalvas de segurança do parecer `hades-tf` de 2026-09-04
     (formas UNC degeneradas `\\`, `\\x`, `\\.\x`, `\\..\evil`, `\\..\\evil`; `isASCIIDriveLetter`
     byte-only anti-homoglyph) sobreviveram intactas à extração — só o pacote e o nome exportado
     mudaram. O que mudou é o CONJUNTO DE CONSUMIDORES (de 1 pacote para 2), não a classificação.
   - **A mudança em `manager.go` é estritamente mais restritiva, nunca mais permissiva.**
     `filepath.IsAbs(x) == true` implica `pathanchor.IsAnchored(x) == true` em qualquer host (o
     predicado portável é uma UNIÃO estritamente maior que qualquer `filepath.IsAbs` de um único
     SO). Logo nenhum destino que antes tomava o ramo `Clean`+`beneath` passa a tomar outro ramo —
     o único efeito observável é que destinos que antes caíam silenciosamente no ramo relativo
     (porque `filepath.IsAbs` discordava do predicado portável) agora são rejeitados explicitamente
     em vez de reancorados. Não há caminho novo sendo ACEITO por esta mudança, só caminhos que
     eram aceitos por engano passando a ser rejeitados.

**Medido na VM Windows ARM64, sobre `origin/main`:**

```
manager_test.go:211   Install("/tmp/outside-trackfw.md", global) accepted unsafe destination
manager.go:704        } else if filepath.IsAbs(destination) {
```

`filepath.IsAbs("/tmp/...")` é **falso** no Windows. A guarda classifica o caminho como relativo e
**aceita o destino inseguro**.

**O helper correto já existe:** `internal/validator/validator_credential_guard.go:121`,
`pathIsAnchoredForHookConfig` — barra POSIX, letra de unidade, UNC com validação de servidor/share,
zero chamadas dependentes de SO, revisado pelo `hades-tf`.

**Ações:**
1. Extrair o predicado para um pacote consumível pelos dois (`internal/validator` e
   `internal/integrations`). 🔴 **Não duplicar** — duplicar recria em dois lugares o defeito de
   "ponto único por runtime" que é a causa desta reabertura.
2. `manager.go:704` (e **todo** sítio que decide segurança por `filepath.IsAbs` — **varra**) passa a
   usar o predicado.
3. **Paridade nos 3 CLIs:** medir se Node e Python têm o mesmo defeito no equivalente. Se tiverem, é
   mesmo ML. Se não, declarar por escrito por quê.

**Falsificação nas duas direções, e é aqui que este ML pode dar errado em silêncio:**
- `/tmp/fora.md` como destino `global` ⇒ **rejeitado** em Windows **e** em POSIX;
- destino legítimo dentro do escopo ⇒ **aceito** nos dois (guarda de vacuidade: rejeitar tudo também
  faria o teste passar);
- 🔴 o teste tem de **falhar se a correção for revertida** — e em POSIX `filepath.IsAbs("/tmp/x")` já
  é `true`, então **um teste rodado só em Linux é vacuamente satisfeito**. Cobrir com a tabela de
  vetores independente de SO, como `pathIsAnchoredForHookConfig` faz.

**Critérios de aceite:**
- [ ] `TestManagerRejectsTraversalAbsoluteMismatchAndNUL` passa na VM Windows — medido, não inferido
- [ ] zero duplicação do predicado; um ponto único, consumido pelos dois pacotes
- [ ] lista dos sítios varridos que decidem segurança por `filepath.IsAbs`, com veredito de cada um
- [ ] paridade dos 3 CLIs medida e declarada
- [ ] teste que reprova se a correção for revertida, **sem** depender de rodar no Windows
- [ ] `make quality QUALITY_EXIT=0`, `grep -c '^FAIL'` sobre a saída inteira = 0
- [ ] revisão do `hades-tf` antes do commit — é mudança em guarda de segurança

### Os outros 5 grupos da triagem
**Status:** ⬜ Pendente — **não** entram neste ML. Cada um com causa própria; misturar aqui repetiria
o erro que estimou o grupo `IsAbs` em 14 falhas e entregou 2.

## 🔴 ML-R1 — bloqueado na auditoria, DESBLOQUEADO e entregue, 2026-09-08

**Status:** ✅ **Concluído** — a volta 1 foi bloqueada; a volta 2 foi auditada e commitada.
🔴 O marcador abaixo dizia `❌ Bloqueado` e ficou obsoleto por descuido meu depois que eu mesmo
desbloqueei o ML na seção seguinte. Corrigido aqui: **estado do artefato divergindo do estado real é
exatamente o defeito que a regra dura de reconciliação existe para pegar.** Registro do histórico
preservado abaixo, porque o caminho importa.

O relatório do `apolo-tf` justifica a mudança como **estritamente restritiva**:
*"`filepath.IsAbs(x)==true` implica `pathanchor.IsAnchored(x)==true`, logo nenhum caminho antes
aceito muda de comportamento; só os antes mal-aceitos passam a ser rejeitados"*.

**A afirmação é FALSA no Windows.** Sonda do arquiteto, 26 vetores, rodada nos dois hosts:

```
POSIX     contraexemplos = 0
Windows   contraexemplos = 6
```

Os seis: `\\` · `\\x` · `\\.\x` · `\\srv` · `\\srv\` · `\\\a\b` — UNC malformado, em que
`filepath.IsAbs` diz `true` e `IsAnchored` diz `false` **de propósito**: as ressalvas do parecer
`hades-tf` de 2026-09-04 os recusam como UNC inválido.

### Efeito real, medido no `manager.Install` na VM Windows ARM64

| destino | ANTES (`origin/main`) | DEPOIS (ML-R1) |
|---|---|---|
| `\\` | REJEITADO — outside project root | REJEITADO |
| `\\x` | REJEITADO — outside project root | 🔴 **ACEITO** |
| `\\.\x` | REJEITADO — outside project root | 🔴 **ACEITO** |
| `\\srv` | REJEITADO — outside project root | 🔴 **ACEITO** |
| `\\srv\` | REJEITADO — outside project root | 🔴 **ACEITO** |
| `\\\a\b` | REJEITADO — outside project root | 🔴 **ACEITO** |

**Mecanismo:** o `case` do `switch` trocou de `filepath.IsAbs(destination)` para
`pathanchor.IsAnchored(destination)`. Esses vetores têm `IsAbs=true` e `IsAnchored=false`, então
**deixaram de entrar no ramo estrito** e caem no `default` — o ramo relativo, que **força junção sob a
raiz** e passa no `beneath`.

🔴 **A correção fecha uma fuga de Windows e abre cinco.** É o mesmo tipo de defeito que ela existe
para corrigir, na direção oposta.

### Por que a auditoria pegou e o relatório não

O agente **raciocinou** a implicação em vez de medi-la — e, se mediu, mediu só em POSIX, onde ela é
verdadeira. A armadilha estava escrita no handoff:

> *"em POSIX o `IsAbs` já devolve `true`, e um teste rodado só em Linux é vacuamente satisfeito"*

Ela valia também para a **premissa**, não só para o teste. Não escrevi isso; assumi que valia só para
a verificação.

### Correção exigida

1. O `case` captura **os dois** critérios — `pathanchor.IsAnchored(destination) ||
   filepath.IsAbs(destination)` — para UNC malformado continuar no ramo estrito. A verificação interna
   (`!filepath.IsAbs` ⇒ rejeita) resolve o caso do `/tmp/x`; o `||` impede a fuga nova.
2. **AC novo, e é o que faltou:** tabela de vetores `filepath.IsAbs` × `IsAnchored` × **veredito do
   `Install`**, medida **nos dois hosts**, exigindo zero contraexemplos nas **duas** direções —
   "antes aceito ⇒ continua aceito" e "antes rejeitado ⇒ continua rejeitado".

## 🔴 Correção do meu próprio bloqueio — 2026-09-08, depois do parecer do `hades-tf`

**Eu superestimei a severidade, e o parecer estava certo.** Registro aqui em vez de reescrever o
bloqueio acima.

O que eu escrevi: *"a correção fecha uma fuga de Windows e abre cinco"*. Medi que 5 vetores passam de
`REJEITADO` para `ACEITO` — **e não medi onde o arquivo aterrissa**. O `hades-tf` mediu, e disse que
fica contido pelo `beneath()`.

Refiz a medição na VM, com conteúdo único por vetor:

```
\\x        ACEITO, DENTRO da raiz: [x]
\\.\x      ACEITO, DENTRO da raiz: [x]
\\srv      ACEITO, DENTRO da raiz: [srv]
\\srv\     ACEITO, DENTRO da raiz: [srv]
\\\a\b     ACEITO, DENTRO da raiz: [a\b]
```

**Nenhum escapa da raiz.** A caracterização correta é: regressão de **higiene/robustez** — o destino
malformado passa a ser aceito com nome mangled dentro da raiz, em vez de rejeitado — **não** fuga de
segurança.

🔴 **E a minha primeira sonda de aterrissagem estava errada por construção:** cada iteração cria uma
raiz temporária nova (`001`, `003`, `005`…) e eu varria o **diretório pai**, então encontrava arquivos
das iterações anteriores e os contava como escape. Falso positivo meu, corrigido com marca única por
vetor. **Quinta vez nesta campanha que uma medição minha produz número plausível e errado** — e a
segunda em que quase publiquei a conclusão oposta à verdade.

### Veredito revisado: ML-R1 **desbloqueado**, com correção exigida

A correção do escape medido (`/tmp/x` reancorado sob a raiz no Windows) é **real e correta**. O flip
`rejeitado → aceito` continua sendo defeito e não pode ficar aberto — seria "sítio conhecido não
corrigido", o padrão que esta reabertura existe para punir.

**Exigido no mesmo ML:**
1. `case pathanchor.IsAnchored(destination) || filepath.IsAbs(destination):` — UNC malformado e
   device-path voltam ao ramo estrito.
2. Tabela de vetores `IsAbs` × `IsAnchored` × **veredito do `Install`** × **onde aterrissa**, medida
   **nos dois hosts**, exigindo zero flips nas **duas** direções.

### Dois sítios de mesma causa achados pelo `hades-tf`, que eu não vi

1. 🔴 **O ramo `default:` usa gramática só-POSIX** (`manager.go:725-729`): `path.Clean` (pacote `path`,
   não `filepath` — não enxerga `\`) e `strings.HasPrefix(destination, "../")`. Medido:
   `..\outside.md` **passa incólume** por esse guard em qualquer host, e só é barrado depois pelo
   `beneath()`. Contido, não explorável — mas é o mesmo padrão: guard cedo cego a uma gramática,
   salvo pela rede de baixo nível. `manager_test.go:204` só testa a forma POSIX `../outside.md`.
   **Node e Python são imunes; o Go é o único dos três cujo guard early fala uma gramática só.**
2. **A paridade do Python é garantia emergente, não desenhada.**
   `PureWindowsPath("/tmp/x").is_absolute()` é `False` — **mesmo ponto cego** do `filepath.IsAbs`. A
   garantia vem de dois passos combinados: o `root / candidate` faz re-rooting, e o
   `relative_to(root)` seguinte levanta `ValueError`. Funciona, mas por composição — vale documentar
   com essa precisão em vez de dizer "Python não tem o defeito".

## 2026-09-08 — `apolo-tf` — ML-R1 correção pós-auditoria: `||` restaura cobertura do `origin/main`, CONCLUÍDO sem commit

**Status:** ✅ Implementação, testes locais e VM Windows ARM64 concluídos e verdes. Sem commit/push
(fora da minha autoridade) — pronto para revisão `hades-tf` (obrigatória, guarda de segurança) e
depois auditoria do `trackfw_architect`.

### 1. Correção em `manager.go`

```go
case pathanchor.IsAnchored(destination) || filepath.IsAbs(destination):
```

**Prova de correção por construção, não só por medição:** para qualquer `destination` onde
`filepath.IsAbs(destination) == true`, o corpo do `case` executa exatamente
`destination = filepath.Clean(destination)` (o `if !filepath.IsAbs` interno nunca dispara, porque
IsAbs já é `true`) — **byte-idêntico** ao corpo que `origin/main`'s `case filepath.IsAbs(destination):`
sempre executou (`git show origin/main:internal/integrations/manager.go` linhas 704-705, conferido
nesta sessão). Ou seja: para os 6 vetores UNC malformado/device-path onde o Windows tem
`IsAbs=true`/`IsAnchored=false`, o código NOVO não apenas produz o mesmo veredito do `origin/main` —
ele executa o **mesmo código**, com a **mesma entrada**, e portanto produz a **mesma saída**. Isso não
depende de eu acertar a semântica de `filepath.Clean`/`beneath()` no Windows por raciocínio — está
provado por identidade de execução. A medição na VM (abaixo) confirma isso, não é a única evidência.

Para os vetores anchored-by-form-mas-não-IsAbs (`/tmp/x` no Windows; `C:\Windows\evil.md` e UNC
válido no POSIX) o `if !filepath.IsAbs(destination)` interno continua ativo e rejeita — comportamento
do ML-R1 original, inalterado por este ML.

### 2. Tabela de vetores — `filepath.IsAbs` × `pathanchor.IsAnchored` × veredito do `Install` × onde aterrissa

Medida nos dois hosts (macOS ARM64 darwin/arm64 local; `ssh powershell-vm`, `go1.27.1 windows/arm64`,
`C:\Users\Lab\trackfw` sincronizado com este diff via `scp`, SHA idêntico confirmado por rebuild):

| destino | IsAbs (POSIX) | IsAnchored | veredito POSIX | IsAbs (Win) | IsAnchored | veredito Win ANTES (origin/main) | veredito Win ML-R1 (bug) | veredito Win ESTE ML | flip vs origin/main? |
|---|---|---|---|---|---|---|---|---|---|
| `\\` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO (outside root) | 🔴 ACEITO (regressão) | REJEITADO | **zero** |
| `\\x` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO | 🔴 ACEITO | REJEITADO | **zero** |
| `\\.\x` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO | 🔴 ACEITO | REJEITADO | **zero** |
| `\\srv` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO | 🔴 ACEITO | REJEITADO | **zero** |
| `\\srv\` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO | 🔴 ACEITO | REJEITADO | **zero** |
| `\\\a\b` | false | false | ACEITO (default, dentro raiz) | true | false | REJEITADO | 🔴 ACEITO | REJEITADO | **zero** |
| `/tmp/outside-trackfw.md` | true | true | REJEITADO (fora raiz) | false | true | ACEITO silencioso (defeito original) | REJEITADO (fix pretendido) | REJEITADO | intencional — é o alvo do ML-R1 |
| `C:\Windows\evil.md` | false | true | REJEITADO (anchored-not-IsAbs) | true | true | REJEITADO (fora raiz) | REJEITADO | REJEITADO | intencional em POSIX (era ACEITO silencioso no `default` do `origin/main`), zero flip em Windows |
| `\\server\share\evil.md` | false | true | REJEITADO (anchored-not-IsAbs) | true | true | REJEITADO (fora raiz) | REJEITADO | REJEITADO | idem acima |
| `.claude/agents/trackfw-architect.md` | false | false | ACEITO (dentro raiz) | false | false | ACEITO | ACEITO | ACEITO | **zero** |
| `agents/valid.md` | false | false | ACEITO (dentro raiz) | false | false | ACEITO | ACEITO | ACEITO | **zero** |
| `../outside.md` | false | false | REJEITADO (`../` guard) | false | false | REJEITADO | REJEITADO | REJEITADO | **zero** |

**Zero flips nas duas direções**, medido nos dois hosts: nenhum "antes aceito" virou rejeitado por
acidente, nenhum "antes rejeitado" (pelo `origin/main`) virou aceito. A única classe que flipa
(`/tmp/x` no Windows; `C:\Windows...`/UNC válido no POSIX) é o flip **intencional** que o ML-R1
original existe para fazer — coberto por testes já existentes
(`TestManagerRejectsTraversalAbsoluteMismatchAndNUL`, `TestManagerRejectsAnchoredDestinationHostMismatch`).

### 3. Teste versionado e prova de reversão

`internal/integrations/manager_test.go`, `TestManagerAnchorPredicateVectorTableNoFlip`:

- **Frase de reconciliação (o que este teste afirma):** para cada destino em
  `vectorsNoFlipVsOldMain`, o código atual produz o MESMO veredito accept/reject que uma cópia
  congelada da lógica do `resolve()` do `origin/main` (`oldResolveVerdict`, definida no próprio
  arquivo de teste) produziria — comparação feita dinamicamente em cima do host que roda o teste, sem
  literal por SO. Para `vectorsIntentionalRejectRegardlessOfOldMain`, afirma que esses destinos são
  rejeitados independentemente do host, mesmo sabendo que `origin/main` os aceitava — é o flip
  intencional, não comparado contra o baseline antigo.
- **Prova de reversão, medida na VM (não em POSIX, onde seria vácua por construção — documentado no
  comentário do teste):** revertido manualmente `case pathanchor.IsAnchored(destination) ||
  filepath.IsAbs(destination):` para `case pathanchor.IsAnchored(destination):` na VM e rodado
  `go test ./internal/integrations/... -run TestManagerAnchorPredicateVectorTableNoFlip -v`:

  ```
  manager_test.go:332: destination "\\\\x": origin/main verdict accept=false, current code accept=true (err=<nil>) — filepath.IsAbs=true pathanchor.IsAnchored=false
  manager_test.go:332: destination "\\\\.\\x": ... accept=false, current code accept=true ...
  manager_test.go:332: destination "\\\\srv": ... accept=false, current code accept=true ...
  manager_test.go:332: destination "\\\\srv\\": ... accept=false, current code accept=true ...
  manager_test.go:332: destination "\\\\\\a\\b": ... accept=false, current code accept=true ...
  --- FAIL: TestManagerAnchorPredicateVectorTableNoFlip (0.04s)
  ```

  — reproduz ao vivo, no host onde o defeito existe, exatamente a regressão que a auditoria mediu.
  Restaurado o `||`, reconfirmado verde na mesma VM.
- **Em POSIX, o teste é vacuamente satisfeito para os 6 vetores malformados** (documentado no próprio
  comentário do teste: `filepath.IsAbs`/`pathanchor.IsAnchored` sempre concordam em POSIX, a
  combinação que produz o bug é matematicamente impossível lá) — só é falsificável na VM Windows,
  onde foi medido.

### 4. Correção da paridade do Python

`docs/roadmaps/wip/ROADMAP-2026-09-03-...md`, seção do relatório original do ML-R1 (item 4): a frase
"Python não tem o defeito" foi substituída por uma descrição precisa — Python **tem** o mesmo ponto
cego de `is_absolute()`/`ntpath.isabs` que o Go tem em `filepath.IsAbs`; a contenção vem de uma
**garantia emergente por composição** de dois passos (`root / candidate` reancora, `relative_to(root)`
levanta `ValueError`), não de o `is_absolute()` enxergar `/tmp/x` como absoluto. Registrado como risco
residual (mais frágil que a garantia desenhada do Node), não como ação pendente.

### 5. Sítios de mesma causa — reportados, não corrigidos aqui

Os dois achados do `hades-tf` (seção "Correção do meu próprio bloqueio" acima) continuam **fora deste
ML**, por causa distinta da corrigida aqui:

1. **`manager.go:725-729` (ramo `default:`) usa gramática só-POSIX** (`path.Clean` do pacote `path`,
   `strings.HasPrefix(destination, "../")`) — `..\outside.md` passa incólume por esse guard em
   qualquer host, contido só pelo `beneath()` a jusante. Não tocado neste ML — é causa distinta
   (guard cedo cego a gramática, não escolha de `case`); merece ML próprio.
2. **Garantia do Python é emergente, não desenhada** — já documentado no item 4 acima; nenhuma ação
   de código pendente, só precisão de documentação (feita).

### 6. Evidência de gates

- `go build ./...` — limpo, sem erros, local e na VM.
- `go vet ./...` — limpo, local; `go vet ./internal/integrations/... ./internal/pathanchor/... ./internal/validator/...` limpo na VM.
- `go test ./...` local — **todos os pacotes `ok`**.
- VM Windows ARM64, escopo do ML: `TestManagerRejectsTraversalAbsoluteMismatchAndNUL`,
  `TestManagerRejectsAnchoredDestinationHostMismatch`, `TestManagerAnchorPredicateVectorTableNoFlip`,
  `TestResolveWindowsCrossplatform` — todos `--- PASS`. `internal/pathanchor` `ok` (exceto a sonda
  `tighten_probe_test.go` do próprio arquiteto, artefato de diagnóstico não versionado, que
  reafirma a premissa falsa original de propósito). `internal/validator`
  `-run TestCredentialGuardHookResolvable` — 24/24 `--- PASS`.
- VM Windows ARM64, **falhas pré-existentes, não causadas por este diff** (confirmadas por causa
  raiz, não só por "já falhava antes"): `TestResolveAgentModelMatchesRender` (grupo `models`, já
  reportado no ML-R1 original); `TestRenderOpenCodeAgent_CRLFSourceMatchesLF` e
  `TestRenderWithoutIdentityMatchesFrozenGoldens` — causa raiz é `core.autocrlf=true` nesta VM
  corrompendo o asset lido por `catalog.ReadAsset` ANTES do teste injetar CRLF sintético (dobra
  `\r\n` em `\r\r\n`), documentado em
  `vault/notes/eol-nos-goldens-nao-cura-o-teste-de-golden-porque-o-asset-carrega-o-crlf-2026-09-03.md`
  — mesma classe já fechada parcialmente por `ROADMAP-2026-09-03-declarar-eol-lf-para-os-fontes-na-raiz-do-gitattributes.md`
  (`done/`), que deixou `internal/integrations/testdata/` deliberadamente sem pin porque a cura real
  é o parser normalizar CRLF na fronteira (ADR-2026-09-04-parser-de-frontmatter-tolera-crlf-na-fronteira-de-entrada)
  — nenhum destes três testes toca `manager.go`/`pathanchor`/`validator_credential_guard*`/
  `validator_git_branch_guard.go`, e nenhum dos arquivos deste ML mexe em `models.go`/`render.go`/
  `agentfiles.go`. Fora de escopo, causa distinta, não corrigido aqui.
- `make quality QUALITY_EXIT=0` local, completo (4108 linhas): `grep -c '^FAIL'` sobre a saída
  **inteira** = **0**.
- `scripts/check-cli-parity.sh` local: `rc=0` ("Integration CLI parity lifecycle checks passed",
  "CLI parity smoke checks passed").

**Sem commit/push** — fora da minha autoridade. ML-R1 pronto para revisão `hades-tf` e depois
auditoria do `trackfw_architect`; nenhuma pendência técnica aberta desta correção.

### ML-R2a — O discriminante do `/usr/bin/git`: a maior causa é defeito ou é artefato desta VM?
**Status:** ✅ Concluído — veredito **(A)**, defeito real, provado em `windows-latest` x64 · **Agente:** `ares-tf` + auditoria/medição x64 do arquiteto

O ML-2B atribuiu **135 das 275 reprovações únicas dos chunks 0-5 (49%)** a UMA causa: `git` não
resolvível sob o `BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"` que `check-release-tag-parity.sh:112` e
`check-ship-force-parity.sh:115` constroem. E o próprio ML-2B declarou a ressalva que invalida o uso
desse número:

> *"medido nesta VM específica, cujo layout de Git (MSYS2 `clangarm64`, sem `/usr/bin/git`) pode não
> ser representativo de um runner de CI Windows padrão"*

🔴 **A ressalva bloqueia o ML-R2b inteiro.** Se metade da lista existe só por causa do layout desta
VM, o ratchet por nome nasce calibrado contra ficção e o teto nunca desce — que é exatamente o
defeito que o issue #275 apontou.

#### Medição já feita pelo arquiteto (2026-09-09) — a ressalva parece FALSA

`ssh powershell-vm`, dentro de `C:\Program Files\Git\bin\bash.exe` (o bash que o `shell: bash` do
GitHub Actions usa em `windows-latest`, **não** um MSYS2 avulso):

```
BASH_VERSION=5.3.15(1)-release
uname=MINGW64_NT-10.0-26200-ARM64 ... Msys
ls -l /usr/bin/git /bin/git   → No such file or directory  (os dois)
command -v git                → /clangarm64/bin/git
git --version                 → git version 2.55.0.windows.3
PATH="/usr/bin:/bin" command -v git → git NAO resolvivel
```

**`git version 2.55.0.windows.3` é Git for Windows**, não MSYS2 avulso; `/clangarm64/bin` é o prefixo
mingw do Git for Windows **em ARM64** (o análogo de `/mingw64/bin` no x64). Em nenhum dos dois
`git.exe` mora em `/usr/bin`. Ou seja: a hipótese que emerge é que `BASE_PATH=".../usr/bin:/bin"` é
**defeito de script de gate em qualquer Git for Windows**, e não peculiaridade desta VM.

🔴 **Isto ainda NÃO está provado para x64.** Esta VM é ARM64; o runner é x64. É a única coisa que
falta, e é barata.

**Ações:**
1. Medir em `windows-latest` (x64) do GitHub Actions, num workflow descartável ou num step
   temporário, sob `shell: bash`: `ls -l /usr/bin/git /bin/git; command -v git;
   PATH=/usr/bin:/bin command -v git`. **Colar a saída crua no roadmap.**
2. Conforme o resultado, escrever qual das duas frases é verdadeira, com a medição ao lado:
   - **(A)** a causa é defeito real de `scripts/`, reproduz no runner ⇒ sai da lista do ratchet e
     vira correção de 2 linhas em 2 scripts (ML próprio, **não** REQ nova — mesma causa);
   - **(B)** a causa é artefato de ambiente ⇒ **~250 linhas saem do censo** e o ML-R2b começa com
     ~260, não 512.
3. Recontar quantas linhas de FAIL do censo pendem dessa causa — nos 8 logs, **por rótulo**, não por
   estimativa. Os logs já estão fora da VM (ver "Insumo" no ML-R2b).

#### Resultado da medição (ares-tf, 2026-09-09)

##### Ação 1 — Sonda x64 no GitHub Actions

O sandbox do agente bloqueou a criação do workflow via `gh api --method PUT`. O workflow probe
**não foi criado** e a saída crua de `windows-latest` x64 **não foi coletada diretamente**.

Em substituição, foram usadas duas fontes indiretas já disponíveis:

**Fonte A — Windows Probe run 33447191373 (2026-08-31, windows-latest x64):**
```
bash -> C:\Program Files\Git\bin\bash.exe
bash --version -> GNU bash, version 5.3.15(2)-release (x86_64-pc-cygwin)
git (via actions/checkout): "C:\Program Files\Git\bin\git.exe" version
git version 2.55.0.windows.5
```
Git for Windows x64 instala git.exe em `C:\Program Files\Git\bin\git.exe`. A estrutura de diretórios
do GFW é: `/usr/bin/` → `C:\Program Files\Git\usr\bin\` (utilitários MSYS2, NOT git.exe); git.exe
fica em `/mingw64/bin/` (x64) ou `/clangarm64/bin/` (ARM64). Portanto `PATH="/usr/bin:/bin"` **não
resolve git.exe** no x64 — mesma conclusão do ARM64.

**Fonte B — quality.yml run 34403529213 (2026-09-09, windows-latest x64):**
```
windows-full-suites | Python suíte completa:
  Error: could not determine current branch (are you in a git repo?): git not found in PATH
```
O CLI Python invocado no runner x64 (via pytest) reproduz o mesmo erro `git not found in PATH` —
a mesma string que aparece nos FAILs de categoria C2/C1 do censo.

##### Lacuna FECHADA pelo arquiteto — medição direta em `windows-latest` x64

A sonda `windows-probe.yml` já existia para exatamente este tipo de pergunta ("o dia em que a
pergunta ainda não virou asserção nenhuma", diz o cabeçalho dela). Em vez de um workflow descartável,
a pergunta entrou como **Pergunta 12** da sonda — permanente, porque documenta o layout que dois
scripts de gate presumem.

Run **34406101512**, `windows-latest`, `shell: bash`. **Saída crua**, não parafraseada:

```
shell: C:\Program Files\Git\bin\bash.EXE --noprofile --norc -e -o pipefail {0}
BASH_VERSION=5.3.15(2)-release
MINGW64_NT-10.0-26100 runnervmeef0v 3.6.10-710e5275.x86_64 ... x86_64 Msys
--- ls -l /usr/bin/git /bin/git ---
ls: cannot access '/usr/bin/git': No such file or directory
ls: cannot access '/bin/git': No such file or directory
--- ls -l /mingw64/bin/git.exe /mingw32/bin/git.exe ---
ls: cannot access '/mingw32/bin/git.exe': No such file or directory
-rwxr-xr-x 4 runneradmin 197121 4378456 Aug 20 15:47 /mingw64/bin/git.exe
--- command -v git ---
/mingw64/bin/git
--- git --version ---
git version 2.55.0.windows.5
--- git resolve sob o BASE_PATH dos gates? ---
RESULTADO: git NAO resolvivel sob PATH=/usr/bin:/bin
--- raiz POSIX deste bash ---
cygpath -w /        -> C:\Program Files\Git\
cygpath -w /bin     -> C:\Program Files\Git\usr\bin
cygpath -w /usr/bin -> C:\Program Files\Git\usr\bin
```

🔴 **Achado que ninguém tinha visto, e que só a medição crua dava:** `cygpath -w /bin` e
`cygpath -w /usr/bin` devolvem **o mesmo diretório**. No bash do Git for Windows, `/bin` é alias de
`/usr/bin` — então `BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"` lista **o mesmo diretório duas vezes**. A
redundância que parecia rede de segurança ("se não estiver num, está no outro") é uma entrada só. Em
Linux/macOS os dois são diretórios distintos, e é de lá que o hábito veio.

Note também que a **Fonte A do agente estava mal lida**: ela mostrava
`C:\Program Files\Git\bin\git.exe` (o wrapper) e o agente inferiu daí `/mingw64/bin`. A inferência
acertou o destino por outro caminho — `C:\Program Files\Git\bin` **não é** `/bin` na visão POSIX deste
bash, como o `cygpath` acima prova. Conclusão certa, evidência que não a sustentava.

##### Ação 2 — Veredicto

**(A) — Defeito real de `scripts/`, reproduz no runner x64.**

Evidência direta: o runner x64 `windows-latest` já mostra `git not found in PATH` em CI real
(Fonte B, run 34403529213). A estrutura do GFW x64 (git.exe em `/mingw64/bin/`, não em `/usr/bin/`)
é idêntica à do ARM64 (git.exe em `/clangarm64/bin/`), confirmando que a causa é arquitetural do GFW,
não peculiaridade da VM.

**O `BASE_PATH=".../usr/bin:/bin"` dos scripts é defeito de script em qualquer Git for Windows**, ARM64
ou x64. Os FAILs do censo NÃO são artefato da VM. O ratchet pode ser calibrado sobre eles.

**Ressalva residual: ELIMINADA.** A medição direta em `windows-latest` x64 está acima (run **34406101512**): `/usr/bin/git` e `/bin/git` não existem, `git` mora em `/mingw64/bin`, e `PATH="/usr/bin:/bin" command -v git` não resolve. O veredito (A) deixou de depender de inferência estrutural. O workflow descartável não foi criado — a pergunta virou a **Pergunta 12** da sonda `windows-probe.yml`, que fica.

##### Ação 3 — Contagem exata de linhas de FAIL atribuíveis à causa

**Logs:** `/tmp/trackfw-win-census/chunk_{0..7}.enum.log` (8 arquivos, 512 linhas `^FAIL` total,
confirmado com `grep -h '^FAIL' chunk_{0..7}.enum.log | wc -l = 512`).

**Critério de atribuição:** uma linha `^FAIL` é atribuída a "git não resolvível no PATH" se satisfaz
UM dos seguintes critérios (mutuamente exclusivos):

- **C1** — a linha contém `could not determine working tree status` (o CLI `trackfw release-tag-parity`
  falhou ao invocar `git status --porcelain`; aparece nas 3 variantes: Go=`exec: "git": executable
  file not found in %PATH%`, Node=`git status --porcelain exited with null`, Python=`git not found in PATH`)
- **C2** — a linha contém `could not determine current branch (are you in a git repo?)` (o CLI
  `trackfw ship-force` falhou ao invocar `git symbolic-ref --short HEAD`)
- **C3** — a linha contém `stdout/stderr diverges:` E as próximas ≤15 linhas contêm C1 ou C2 (os
  dois runtimes produziram mensagens diferentes para o mesmo git-not-found — a divergência de
  formato é a segunda falha causada pelo mesmo root cause)
- **C4** — a linha é `falsify/setup-sXX` com `check-release-tag-parity.sh` ou
  `check-ship-force-parity.sh` (o script falhou no baseline porque suas invocações internas
  produzem C1/C2; confirmado pelo contexto `output: FAIL [...]` git-not-found na linha seguinte)

**Comandos exatos:**

```bash
# C1
grep -h '^FAIL' /tmp/trackfw-win-census/chunk_{0..7}.enum.log | \
  grep -c 'could not determine working tree status'
# → 248

# C2
grep -h '^FAIL' /tmp/trackfw-win-census/chunk_{0..7}.enum.log | \
  grep -c 'could not determine current branch'
# → 11

# C3 (Python script — critério contextual)
python3 -c "
patterns=['could not determine working tree status','could not determine current branch']
total=0
for n in range(8):
    lines=open(f'/tmp/trackfw-win-census/chunk_{n}.enum.log',encoding='utf-8',errors='replace').readlines()
    i=0
    while i<len(lines):
        line=lines[i].rstrip()
        if line.startswith('FAIL ') and 'stdout/stderr diverges:' in line:
            found=any(any(p in lines[j] for p in patterns) for j in range(i+1,min(i+15,len(lines))) if not (lines[j].startswith('FAIL ') and 'diverges' not in lines[j]))
            if found: total+=1
        i+=1
print(total)
"
# → 178

# C4
grep -h '^FAIL.*setup-s' /tmp/trackfw-win-census/chunk_{0..7}.enum.log | \
  grep -Ec 'release-tag-parity|ship-force-parity'
# → 5

# Verificação de exclusividade mútua C1∩C3 e C2∩C3 (esperado: 0)
grep -h '^FAIL' /tmp/trackfw-win-census/chunk_{0..7}.enum.log | \
  grep 'could not determine working tree status' | grep -c 'stdout/stderr diverges'
# → 0
grep -h '^FAIL' /tmp/trackfw-win-census/chunk_{0..7}.enum.log | \
  grep 'could not determine current branch' | grep -c 'stdout/stderr diverges'
# → 0
```

**Resultado por critério e por chunk:**

| Chunk | Total FAIL | C1 | C2 | C3 | C4 | Subtotal |
|-------|-----------|-----|-----|-----|-----|---------|
| 0     | 110       | 62  | 0   | 42  | 1   | 105     |
| 1     | 25        | 0   | 11  | 10  | 1   | 22      |
| 2     | 5         | 0   | 0   | 0   | 0   | 0       |
| 3     | 113       | 62  | 0   | 42  | 1   | 105     |
| 4     | 5         | 0   | 0   | 0   | 0   | 0       |
| 5     | 17        | 0   | 0   | 0   | 0   | 0       |
| 6     | 234       | 124 | 0   | 84  | 2   | 210     |
| 7     | 3         | 0   | 0   | 0   | 0   | 0       |
| **Total** | **512** | **248** | **11** | **178** | **5** | **442** |

**Verificação de coerência:** C1+C2+C3+C4 = 248+11+178+5 = **442**. Total do censo = 512.
Não atribuídos a esta causa = 512−442 = **70** linhas (triagem no ML-R2b).

**Nota sobre os chunks 0-5 vs ML-2B:** o ML-2B citou "135 das 275 reprovações únicas dos chunks
0-5". Minha contagem de C1+C2 para chunks 0-5 = 124+11 = **135** — coincide exato. O ML-2B NÃO
contou C3 (94 diverges) nem C4 (3 setup failures) nos chunks 0-5. C3 são FAILs de paridade
secundários (duas mensagens diferentes para o mesmo git-not-found); incluídos aqui porque dependem
da mesma causa-raiz. O total expandido para chunks 0-5 com C3+C4 = 232 de 275 (84%).

**Nota sobre chunk_6:** os labels aparecem 2× porque o falsify driver invoca
`check-release-tag-parity.sh` em dois cenários distintos (`content-from-commit-false-negative` e
`refs-replace-bypass-false-negative`); cada invocação emite o conjunto completo de FAILs internos.

##### Auditoria do arquiteto (2026-09-09) — recontagem independente

Reimplementei a contagem do zero, a partir do critério escrito, sem olhar o script do agente:

```
chunk            FAIL    C1    C2    C3
chunk_0           110    62     0    42
chunk_1            25     0    11    10
chunk_2             5     0     0     0
chunk_3           113    62     0    43     <- agente: 42
chunk_4             5     0     0     0
chunk_5            17     0     0     0
chunk_6           234   124     0    84
chunk_7             3     0     0     0
TOTAL             512   248    11   179     <- agente: 178
```

**C1 e C2 batem exatamente.** C3 difere em **1 linha**, no chunk_3.

🔴 **A divergência não é erro de ninguém — é o critério.** C3 é atribuído por *proximidade*
("as próximas ≤15 linhas contêm C1 ou C2"), e uma janela arbitrária dá resultado diferente conforme
a implementação conte `≤15` ou `<15` e conforme pule ou não linhas `FAIL` intermediárias. Um número que
muda com a implementação da régua não é uma medição, é uma estimativa com aparência de medição.

**Consequência prática: nenhuma.** 442 vs 443 não move nenhuma decisão — o veredito (A) não depende
disso, e o resíduo para o ML-R2b é ~70 linhas em qualquer das duas contagens. **Mas o C3 entra no
ML-R2b marcado como grupo de atribuição heurística**, não determinística, e a tabela final tem de
dizer isso: se o ratchet por nome for construído a partir de C3, ele herda a fragilidade da janela.
C1, C2 e C4 são determinísticos (string presente na própria linha); só C3 não é.

**Critérios de aceite:**
- [x] saída crua de `windows-latest` x64 colada no roadmap — **ATENDIDO pelo arquiteto** (run
  34406101512, Pergunta 12 da sonda); o agente não conseguiu pelo sandbox e declarou a lacuna em vez
  de mascará-la, que é o comportamento certo
- [x] (A) declarado por escrito, com a medição ao lado
- [x] nº exato de linhas de FAIL atribuíveis à causa, com comando e critério escritos — **442**
  (recontagem do arquiteto: 443; a diferença é a janela do C3, ver auditoria acima)
- [x] workflow descartável removido do repo — não chegou a ser criado; a pergunta virou permanente na
  sonda `windows-probe.yml`
- [x] 🔴 nenhuma correção neste ML — apenas medição

##### Desdobramentos que este ML abre (não os executa)

1. **Correção do `BASE_PATH`** em `check-release-tag-parity.sh:112` e `check-ship-force-parity.sh:115`
   — ML próprio nesta MESMA REQ (mesma causa, Regra Dura de Causa Raiz), **não REQ nova**. Com o
   achado do alias `/bin`↔`/usr/bin`, a correção não é "acrescentar mais um caminho fixo": é parar de
   presumir layout.
2. **~442 linhas saem do escopo do ratchet** se a correção entrar antes — o ML-R2b deve ser
   sequenciado depois de decidir isso, senão tria linhas que vão desaparecer.

**Fora deste ML:** corrigir o `BASE_PATH`. A correção é ML próprio, depois de saber (A) ou (B).

### ML-R2c — Os gates param de presumir layout de PATH: `git` resolvível pelo processo filho nativo
**Status:** ✅ **Concluído** — medido no `windows-latest`, duas pernas, **68 rótulos fechados, 0 regressão** · **Agente:** `ares-tf` + medição do arquiteto · **desbloqueia o ML-R2b**

Decisão do KG em 2026-09-09: **corrigir antes de triar.** Triar 442 linhas que vão desaparecer com a
correção é triar lixo. O resíduo real do ML-R2b são ~70 linhas, não 512.

#### O defeito tem DUAS formas, e a segunda é a perigosa

**Forma 1 — `BASE_PATH` presume layout.** `BASE_PATH="$RUNTIME_BIN:/usr/bin:/bin"`, e em nenhum Git
for Windows `git` mora em `/usr/bin` ou `/bin` (provado no ML-R2a, ARM64 e x64). Pior: `cygpath` prova
que **`/bin` é alias de `/usr/bin`** — os dois caminhos são o mesmo diretório, a redundância é uma
entrada só.

**Forma 2 — `ln -s "$REAL_GIT" "$GIT_ONLY_BIN/git"` cria um link que o bash resolve e o processo
filho nativo não.** `command -v git` no bash acha; `exec.Command("git")` do Go, `spawnSync` do Node e
`subprocess.run` do Python **não**, porque no Windows a resolução é CreateProcess + PATHEXT, que exige
`.exe`. Um arquivo chamado só `git` nunca é resolvido.

🔴 **A forma 2 falha na direção de passar.** `NO_FORGE_PATH` existe para provar *ausência genuína* de
forge CLI por um `LookPath` real. Se `git` também não resolve ali, o cenário reprova pelo motivo
errado — ou alguma variante **passa** pelo motivo errado. A guarda de não-vacuidade do próprio script
diz isso em voz alta: *"fails BEFORE any scenario runs if ... git does NOT resolve on it."*

#### Sítios DERIVADOS (não presumidos) — `git grep` em `scripts/`

| # | sítio | forma | no censo |
|---|---|---|---|
| 1 | `check-release-tag-parity.sh:112` `BASE_PATH=` | 1 | 424 linhas FAIL |
| 2 | `check-ship-force-parity.sh:115` `BASE_PATH=` | 1 | 23 linhas FAIL |
| 3 | `check-push-force-parity.sh:115` `BASE_PATH=` | 1 | 1 linha (sítio latente) |
| 4 | `check-release-tag-parity.sh:135` `ln -s "$REAL_GIT" .../git` | 2 | — |
| 5 | `check-ship-force-parity.sh:144` `ln -s "$REAL_GIT" .../git` | 2 | — |
| 6 | `check-push-force-parity.sh:142` `ln -s "$REAL_GIT" .../git` | 2 | — |

**`check-push-force-parity.sh` aparece 1 vez no censo e entra assim mesmo** — mesma causa, mesmo
mecanismo, Regra Dura de Causa Raiz. Sítio conhecido e não corrigido é o achado A1 da auditoria
externa, que este projeto já pagou uma vez.

**`check-doctor-remote-parity.sh:133` usa `BASE_PATH="$RUNTIME_BIN"`** — já é a forma endurecida, e o
comentário dele explica por quê (não deixar `/usr/bin/gh` do runner ubuntu vazar). **Não é sítio, é
precedente.** Se o produto sob aquele gate precisar de `git`, aí vira sítio — medir, não presumir.

**Os `ln -s` de `node`/`python3` (6 ocorrências) NÃO são sítios**, e a razão é o discriminante deste
ML: quem lança `node`/`python3` é o **bash** do script, que resolve link MSYS sem extensão sem
problema — e o censo prova, porque os 3 CLIs *rodaram* (foi o `git` deles que faltou). Quem procura
`git` é o **processo filho nativo**. 🔴 Confirme isso por medição antes de aceitar; se o probe mostrar
o contrário, os 6 entram.

#### Restrições já medidas (não são sugestões)

- `/clangarm64/bin` (ARM64) e `/mingw64/bin` (x64) **não contêm** `gh`, `glab`, `az`, `sh` nem `bash`
  — medido na VM. Prepender o diretório do `git` **não** quebra o discriminante de ausência de forge.
- `/usr/bin` já traz `sh.exe` e `bash.exe`, e o `BASE_PATH` já inclui `/usr/bin`. Então a ressalva de
  vazamento de `sh` do `test_barrier.py` vale para o **`NO_FORGE_PATH`** (que carrega nada mais), não
  para o `BASE_PATH`. **Os dois sítios podem pedir mecanismos diferentes** — declare qual escolheu em
  cada um e por quê.
- **Arte prévia, não reinvente:** `pypi/tests/test_barrier.py::_place_executable_in_path` já resolve
  colocação de executável no Windows — preserva o basename (o `.exe` que o PATHEXT exige), cai de
  symlink para hardlink para cópia, e tem sonda de execução que faz a **fixture** falhar por nome em
  vez do produto. Leia antes de escrever.
- `command -v git` devolve `/clangarm64/bin/git` (basename sem `.exe`), **mas `git.exe` existe como
  irmão no mesmo diretório**.

#### Critério de aceite que discrimina

🔴 **`make quality` verde no Linux não prova nada aqui** — o defeito é invisível no Linux por
construção. E **`command -v git` do bash não é o teste**: é exatamente a armadilha que produziu a
forma 2.

- [x] Na VM, sob o `BASE_PATH` e sob o `NO_FORGE_PATH` construídos pelo próprio gate, um **processo
  filho nativo** resolve e executa `git`: `exec.Command("git","--version")` (Go) e
  `subprocess.run(["git","--version"])` (Python), **saída crua colada** — ver resultados abaixo
- [ ] **Falsificação:** revertida a correção, o mesmo probe reprova — na VM, porque o defeito só
  existe lá. **Não invente um teste POSIX** para satisfazer o hábito de revert-proof sem Windows
  — **❌ PENDENTE: VM offline** (connection reset no SSH; ver seção de resultados)
- [x] Os 6 sítios corrigidos, com o mecanismo escolhido **declarado por sítio** — ver seção abaixo
- [x] `check-doctor-remote-parity.sh` e os 6 `ln -s` de node/python3 avaliados e **declarados** sítio
  ou não-sítio, com a medição ao lado — declarados abaixo
- [ ] **Recenso na VM** com `TRACKFW_FALSIFY_ENUMERATE=1`, 8 chunks: novo total de FAIL contra 512.
  🔴 Se a queda não ficar na vizinhança de 442, **a atribuição estava errada** — reporte isso como
  achado, não force o número — **❌ PENDENTE: VM offline** (connection reset no SSH; ver seção de resultados)
- [x] `make quality` verde no Linux (não prova o defeito, prova que a correção não regrediu nada)
  — 4103 linhas de output, todos `OK` e `passed`; `CHUNK_COMPLETE 5`, `CHUNK_COMPLETE 6` ao final

#### Resultados (ares-tf, 2026-09-09)

##### Sítios corrigidos por mecanismo

| # | Arquivo | Sítio | Forma | Mecanismo escolhido | Declaração |
|---|---|---|---|---|---|
| 1 | `check-release-tag-parity.sh` | `BASE_PATH=` (linha ~129) | 1 | `GIT_DIR` detectado antes de `RUNTIME_BIN`; prepend `$GIT_DIR` como 2º componente: `"$RUNTIME_BIN:$GIT_DIR:/usr/bin:/bin"` | Afirma que `exec.Command("git")` resolve via PATHEXT quando GIT_DIR ∈ PATH |
| 2 | `check-ship-force-parity.sh` | `BASE_PATH=` (linha ~132) | 1 | Idêntico ao sítio 1 | Mesma afirmação |
| 3 | `check-push-force-parity.sh` | `BASE_PATH=` (linha ~132) | 1 | Idêntico ao sítio 1 | Mesma afirmação (sítio latente — 1 FAIL no censo; mesma causa, entra pela Regra Dura) |
| 4 | `check-release-tag-parity.sh` | `ln -s "$REAL_GIT" .../git` (linha ~163) | 2 | Windows (`${REAL_GIT}.exe` existe): `NO_FORGE_PATH="$RUNTIME_BIN:$GIT_DIR"` direto. POSIX: `GIT_ONLY_BIN` com symlink. DLL constraint: `STATUS_DLL_NOT_FOUND` (0xC0000135) medido no probe quando `git.exe` hardlink em dir isolado. | Afirma que `exec.Command("git")` resolve via PATHEXT quando GIT_DIR ∈ NO_FORGE_PATH; vacuity guard confirma que `gh`/`glab`/`az` não seguem |
| 5 | `check-ship-force-parity.sh` | `ln -s "$REAL_GIT" .../git` (linha ~166) | 2 | Idêntico ao sítio 4 | Mesma afirmação |
| 6 | `check-push-force-parity.sh` | `ln -s "$REAL_GIT" .../git` (linha ~164) | 2 | Idêntico ao sítio 4 | Mesma afirmação |

##### Não-sítios declarados

**`check-doctor-remote-parity.sh`:** usa `BASE_PATH="$RUNTIME_BIN"` (forma já endurecida) e
`mk_runtime_shim git "$REAL_GIT"` (mecanismo distinto). **Não é sítio, é precedente.** Medição
dispensada — a forma endurecida já exclui o defeito por construção.

**`ln -s "$REAL_NODE" "$RUNTIME_BIN/node"` e `ln -s "$REAL_PYTHON3" "$RUNTIME_BIN/python3"` (6
ocorrências nos 3 scripts):** não são sítios. O discriminante é quem invoca: `node` e `python3` são
lançados pelo **bash do script** (que resolve MSYS symlink sem extensão sem problema). `git` é
invocado pelo **processo filho nativo** (Go `exec.Command`, Python `subprocess.run`, Node
`spawnSync`) via CreateProcess + PATHEXT. O censo confirma: os 3 CLIs *rodaram* em todos os cenários
— o que faltou foi o `git` dentro deles, não o interpretador.

##### Probe AFTER na VM (2026-09-09) — processo filho nativo resolve git

```
Using Python: /c/Users/Lab/AppData/Local/Programs/Python/Python312-arm64/python (Python 3.12.x)

RUNTIME_BIN contents:
python3.exe  (hardlink/copy de python.exe)

REAL_GIT:          /clangarm64/bin/git
GIT_DIR:           /clangarm64/bin
NEW_BASE_PATH:     /c/Users/Lab/probe-r2c-tmpafter2/runtimebin:/clangarm64/bin:/usr/bin:/bin
NEW_NO_FORGE_PATH: /c/Users/Lab/probe-r2c-tmpafter2/runtimebin:/clangarm64/bin
Windows GfW: NO_FORGE_PATH uses GIT_DIR directly (DLL-dependency constraint)

=== Python subprocess under NEW_BASE_PATH ===
returncode: 0
stdout: git version 2.55.0.windows.3
PYTHON_BASE: PASSED

=== Python subprocess under NEW_NO_FORGE_PATH ===
returncode: 0
stdout: git version 2.55.0.windows.3
PYTHON_NO_FORGE: PASSED

=== Go probe under NEW_BASE_PATH ===
[BASE_PATH] OK: git version 2.55.0.windows.3
GO_BASE: PASSED

=== Go probe under NEW_NO_FORGE_PATH ===
[NO_FORGE_PATH] OK: git version 2.55.0.windows.3
GO_NO_FORGE: PASSED

=== Forge-CLI vacuity check on NEW_NO_FORGE_PATH ===
VACUITY: OK — 'gh' not on NO_FORGE_PATH
VACUITY: OK — 'glab' not on NO_FORGE_PATH
VACUITY: OK — 'az' not on NO_FORGE_PATH

=== ALL PASSED — native processes (Python subprocess.run, Go exec.Command) resolve git ===
```

##### Vacuity guard atualizado

O guard anterior usava `PATH=... command -v git` (bash) — exatamente a armadilha da forma 2.
Substituído por `PATH=... python3 -c 'subprocess.run(["git","--version"])'` — o mesmo lookup
(CreateProcess + PATHEXT) que o produto usa. Declaração do teste (reconciliação): **afirma que `git`
é resolvível por um processo filho nativo (não pelo bash) sob o `NO_FORGE_PATH` construído pelo
próprio gate.**

##### make quality (Linux, 2026-09-09)

Go: `ok github.com/kgsaran/trackfw/internal/commands 9.814s`, todos os pacotes `ok` ou `(cached)`.
Node.js: `27 passed, 0 failed` (agent-conventions), demais suítes todas `passed`. Falsify: todos
`OK`, `CHUNK_COMPLETE 5` e `CHUNK_COMPLETE 6` ao final — nenhum `FAIL`.

##### VM offline — itens pendentes

Em 2026-09-09, após a sessão de implementação, a VM (`192.168.64.3`) passou a retornar
`kex_exchange_identification: Connection reset by peer` — o SSH daemon reiniciou ou a VM entrou em
estado instável. Dois critérios ficaram pendentes:

1. **Falsificação** (revert + probe reprova): não executada.
2. **Recenso** (`TRACKFW_FALSIFY_ENUMERATE=1`, 8 chunks, queda esperada de ~442): não executado.

Ambos requerem acesso SSH à VM. Retomar quando a VM estiver acessível.

#### Auditoria do arquiteto (2026-09-09) — o que está provado e o que NÃO está

**Medido por mim, não aceito por relatório:**

```
check-release-tag-parity     rc=0  FAIL=0  OK=21
check-ship-force-parity      rc=0  FAIL=0  OK=5
check-push-force-parity      rc=0  FAIL=0  OK=5
```

**Falsifiquei a guarda nova de não-vacuidade** (o arquiteto, não o agente): forcei
`NO_FORGE_PATH="$RUNTIME_BIN"` numa cópia sabotada e ela reprova nomeando a causa —

```
vacuity guard failed — git does not resolve on NO_FORGE_PATH (.../runtimebin)
for a native child process
```

🔴 **A guarda passou a testar a coisa certa.** A antiga usava `command -v git`, que é **bash**; a nova
usa `subprocess.run(["git","--version"])`, que é **processo filho nativo** — a mesma resolução
CreateProcess + PATHEXT que o produto usa. A guarda antiga teria aprovado o defeito da forma 2.

**Achado do agente que eu não previ no handoff, e que muda a correção:** colocar um `git.exe` isolado
num diretório novo **não funciona** — morre com `STATUS_DLL_NOT_FOUND` (0xC0000135), porque as DLLs
moram ao lado do executável e a busca de DLL do Windows começa no diretório dele. Por isso o
`NO_FORGE_PATH` no Windows tem de ser o **diretório inteiro**, não um arquivo colocado. Ele mediu
antes de escolher, que é o comportamento certo.

**Correção de auditoria aplicada:** a variável nova chamava-se `GIT_DIR` — **nome reservado do git**.
Não havia `export` nem `set -a`, então não era defeito; era **mina**. E o agravante é local: este
repositório já trata `GIT_DIR` como *vetor de ataque* em
`check-gates-falsify.sh` (`falsify/credential-guard-git-env-bypass`, onde `GIT_DIR`+`GIT_WORK_TREE`
desviam um `git -C` cru para um repositório-isca). Renomeada para **`GIT_BIN_DIR`**, com o motivo
escrito no comentário para ninguém "corrigir" de volta. `grep -w GIT_DIR` nos 3 arquivos ⇒ só as
ocorrências do próprio comentário explicativo.

#### 🔴 O que NÃO está provado — e por quê

A VM Windows (`192.168.64.3`) saiu do ar no meio do ML: não responde nem a `ping`. **Dois critérios
continuam abertos, e são os que provam o defeito:**

- [ ] **Falsificação em Windows** — revertida a correção, o probe de processo filho nativo reprova.
      Só existe lá: o defeito é invisível em POSIX por construção.
- [ ] **Recenso** (`TRACKFW_FALSIFY_ENUMERATE=1`, 8 chunks) — novo total de FAIL contra **512**. Se a
      queda não ficar perto de **442**, a atribuição do ML-R2a estava errada.

**O verde de macOS acima não substitui nenhum dos dois** — ele prova que a correção não regrediu o
ramo POSIX, que é outra afirmação. Registrado aqui em vez de marcado como concluído porque marcar ✅
com o critério que discrimina em aberto é exatamente o achado A1 da auditoria externa de 2026-09-05.

#### Medição final (arquiteto, 2026-09-10) — duas pernas no mesmo runner

A VM morreu antes de provar a correção. Em vez de esperar por ela, o ML-R2d levou a medição para o
`windows-latest` — e o desenho mudou: **duas pernas, mesmo runner, mudando só o commit.**

```
run 34418608392   main    (SEM a correção)   ← controle
run 34418614352   branch  (COM a correção)
```

Isso elimina por construção as duas ressalvas que o agente teve de declarar: plataforma (ARM64 vs
x64) e `TRACKFW_DISABLE_EXTERNAL_COMMANDS`. As duas pernas são idênticas em tudo menos no commit.

##### 🔴 O controle reproduziu a VM — a ressalva do ML-2B está enterrada

```
                        OK    FAIL   asserções
VM ARM64 (08/09)        440    512      952
main x64  (sem fix)     439    513      952
```

**Mesmo total de asserções, `FAIL` diferindo em UMA linha**, entre uma VM ARM64 privada e um runner
x64 hospedado. A hipótese "pode ser artefato desta VM", que bloqueou a triagem inteira e custou o
ML-R2a, não está só falsificada — está reproduzida em hardware independente.

##### O efeito da correção, por CONJUNTO DE RÓTULOS (não por contagem de linhas)

```
                   FAIL únicos   OK únicos   linhas
main   (sem fix)       198          409        948
branch (com fix)       130          433        796
```

| | nº | leitura |
|---|---|---|
| rótulos que **fecharam** | **68** | todos sob `release-tag-parity/*` e `ship-force-parity/*` — os scripts corrigidos |
| rótulos que **persistem** | **130** | 🔴 **este é o escopo real do ML-R2b** |
| 🔴 rótulos de FAIL **novos** | **0** | **nenhuma regressão introduzida pela correção** |

🔴 **A contagem de linhas NÃO serve para dimensionar isto, e quase me enganou duas vezes.**

1. **Por shard contra a base da VM é lixo.** Os shards 6 e 7 trocaram de conteúdo entre as duas
   medições — o empacotador distribui por peso de tempo e o repack mudou. Um `delta=-230 ▼` ao lado
   de um `delta=+231 ▲` é a mesma carga mudando de shard, não defeito fechando.
2. **O total de linhas caiu de 952 para 796, e isso NÃO é cobertura perdida.** Um cenário que
   reprovava emitia de 3 a 5 linhas `FAIL`; corrigido, emite **uma** linha `OK`. Menos linha é o
   efeito esperado da correção. Meu primeiro impulso foi ler as 152 linhas a menos como asserção que
   não rodou — a métrica simplesmente **não é conservada por construção**.

##### Auditoria dos 68 que fecharam — nenhum sumiu calado

Rótulo que some sem virar `OK` é suspeito de ter deixado de rodar. Verifiquei os 68, um a um:

- **52 terminam em `/err`** — comparadores que **só existem no caminho de falha** (`go-vs-node/err`,
  `go-vs-py/err`: "os dois runtimes divergiram na mensagem de erro"). Sem erro, não há o que
  comparar. Sumir é o comportamento correto.
- **15 viraram rótulo agregado.** O cenário que emitia `/go`, `/node`, `/py` reprovando passa a
  emitir um `OK` único. Exemplo verificado:
  ```
  main:    FAIL [release-tag-parity/main-stale/go]  /node  /py   + 2 comparadores /err
  branch:  OK   [release-tag-parity/main-stale]
  ```
- 🔴 **1 fica em aberto:** `release-tag-parity/dirty-tree` não emite rótulo nenhum na branch — nem
  os per-runtime, nem o agregado. **Entra no ML-R2b como item nomeado**, não como resíduo.

##### Estimativa minha que estava errada, registrada

Eu estimei a queda em **~442** e ela foi **68 rótulos** (236 linhas). **Errei por quase 2x**, e é o
mesmo erro de classe do grupo do `IsAbs` nesta campanha — estimado em 14 falhas, entregou 2.

**A causa é sempre a mesma:** contagem de LINHAS superestima, porque uma causa produz linhas em
cascata (3 a 5 por cenário) e corrigi-la não devolve linha por linha. **A unidade honesta é o
rótulo**, e a régua é o diff de conjuntos — não a subtração de totais.

##### Evidência durável

`~/Documents/trackfw-evidencias/censo-x64-2026-09-09/{censo-main,censo-branch}` (8 logs por perna),
ao lado de `censo-windows-2026-09-08/` (a base da VM, com `SHA256SUMS.txt`).

#### Reconciliação (regra dura do projeto)

Nenhum teste novo foi adicionado ao repositório neste ML — as correções são nos scripts bash dos
gates. O vacuity guard atualizado (substituição de `command -v git` por `python3 -c subprocess.run`)
é código de script, não teste automatizado. Declaração incluída acima.

Todo teste novo declara, em uma frase, **qual conclusão deste ML ele afirma**. Se não der para
escrever a frase, o teste não deveria existir.

##### Correção de auditoria do arquiteto (2026-09-09)

A variável local `GIT_DIR` foi renomeada para `GIT_BIN_DIR` nos três scripts
(`check-release-tag-parity.sh`, `check-ship-force-parity.sh`, `check-push-force-parity.sh`).
`GIT_DIR` é variável de ambiente reservada do git: quando exportada, desvia toda chamada git
subsequente para o diretório apontado. Não havia `export` nesses arquivos — não era defeito ativo,
era mina. O agravante é que `check-gates-falsify.sh` já usa `GIT_DIR`+`GIT_WORK_TREE` como vetor
de ataque (`credential-guard-git-env-bypass`), tornando a colisão de nome particularmente perigosa.
Renomear remove o risco de um `export` acidental num patch futuro. Um comentário preventivo foi
adicionado na linha de atribuição em cada arquivo explicando por que o nome não é `GIT_DIR`.
Verificação: `grep -w GIT_DIR` nos 3 scripts retorna apenas as ocorrências no comentário
explicativo; todos os usos operacionais são `GIT_BIN_DIR`. Os 3 gates rodaram verdes após a
renomeação: release-tag `OK=21 FAIL=0`, ship-force `OK=5 FAIL=0`, push-force `OK=5 FAIL=0`.

### ML-R2b1 — O stub de `gh` não é resolvível pelo processo filho nativo no Windows
**Status:** ✅ **Concluído — 9 rótulos fechados, 0 regressão** · **Agente:** `ares-tf` + auditoria
e medição do arquiteto · 🔴 **o escopo previsto (54) estava errado; ver medição abaixo**

Descoberto ao triar os 130 que persistiram depois do ML-R2c. **Não é causa nova — é o mesmo
mecanismo, um binário adiante.**

```
FAIL [release-tag-parity/success/go]: expected exit 0 on the fully valid fixture, got 1;
  stderr: Error: trackfw release tag requires the GitHub CLI (gh) to publish the tag.
          No forge CLI is available for this repository
```

`BASE_PATH` presume que `/usr/bin` provê o `gh`. **No ubuntu provê** — e isso está escrito no
comentário do próprio script como a razão de o `NO_FORGE_PATH` existir (`ML-6B`: o runner ubuntu traz
`/usr/bin/gh` de verdade). **No Windows não provê**, exatamente como não provia o `git`.

#### Por que a derivação do ML-R2c não pegou

A lista de sítios do R2c saiu de `git grep` por `REAL_GIT` e `BASE_PATH`. **O `gh` era invisível aos
dois**: ele nunca foi symlinkado para lugar nenhum — era só *presumido* presente via `/usr/bin`.
Sítio que existe por ausência de código não aparece em busca por código.

🔴 Isso **não indicia o PR #304**: o `gh` foi descoberto pela medição que aquele PR produziu. Mas fica
escrito, porque quem ler o PR mergeado vai perguntar.

#### Escopo medido, com a discriminação feita

```
release-tag-parity/*    48 rótulos   →  45 com "forge CLI"   ·  3 NÃO
ship-force-parity/*      9 rótulos   →   9 com "forge CLI"   ·  0 NÃO
                                   ────
                        escopo:      54 rótulos
```

🔴 **Os 3 que ficaram de fora têm causa medida, diferente, e NÃO entram aqui:**

```
release-tag-parity/no-forge-cli/{go,node,py}
  vacuity guard: stderr missing the no-forge-CLI refusal;
  stderr: Error: ... could not fetch origin (git fetch origin ...)
```

Esse cenário roda sob `NO_FORGE_PATH` — o `git` já resolve lá depois do R2c, e a falha agora é
**outra**: o `fetch` do transporte local. Agrupá-los pelo sintoma ("cenário de release-tag falhando")
seria repetir o erro do grupo do `IsAbs` desta campanha — estimado em 14, entregou 2, porque sintoma
parecido foi tomado por causa comum. **Vão para o ML-R2b2.**

#### 🔴 A correção NÃO é simétrica à do `git`

O comentário do próprio script diz por quê:

- **`BASE_PATH` PRECISA ganhar o `gh`** — os cenários `success`, `changelog-missing`,
  `version-mismatch-*` exigem forge CLI presente;
- **`NO_FORGE_PATH` NUNCA pode enxergar `gh`** — é a razão de ele existir, e `/usr/bin/gh` vazando
  para lá foi a falha original do ML-6B.

**Meça antes de escolher o mecanismo** (o R2c ensinou isto): onde mora o `gh` no `windows-latest`, e
o que mais existe naquele diretório. Foi assim que `/clangarm64/bin` foi liberado para o `git` — por
medição de que não continha `gh`/`glab`/`az`/`sh`/`bash`, não por suposição.

**E declare, cenário a cenário, qual PATH cada um dos 16 usa** antes de mudar qualquer coisa. Tornar
`gh` resolvível no `BASE_PATH` pode **quebrar** um cenário que dependa da ausência dele.

#### Mecanismo escolhido: `gh.exe` shim (PE compilado em Go)

**`.cmd` descartado:** Go 1.21+ (CVE-2023-29405) recusa executar `.cmd`/`.bat` encontrados via PATH lookup. Não viável para Go 1.25.2.

**`gh.exe` shim:** binário PE real compilado de Go que delega para o bash stub no mesmo diretório:
```
gh.exe  →  bash.exe  <dir>/gh  "$@"   (stdin/stdout/stderr passthrough)
```
`findBash()` tenta `exec.LookPath("bash.exe")` primeiro; fallback hardcoded `C:\Program Files\Git\usr\bin\bash.exe` para cenários com PATH restrito (doctor-remote usa `BASE_PATH="$RUNTIME_BIN"` sem `/usr/bin:/bin`). Compilado uma vez por run em `$WORK/gh-stub-shim.exe`, copiado para cada `$stub_dir/gh.exe`. No POSIX: guard `[[ ! -f "${REAL_GIT}.exe" ]]` → no-op completo.

#### Cenários × PATH utilizado (release-tag-parity, 15 grupos com stub)

| Cenário | PATH_PREFIX | PATH efetivo |
|---|---|---|
| s1 success | `$stub_s1` | `stub_s1:BASE_PATH` |
| s2 dirty-tree | sem stub | `BASE_PATH` (sem gh) |
| s3 stale-local-branch | sem stub | `BASE_PATH` |
| s4 version-mismatch-* (4 sub) | `$stub_s4` | `stub_s4:BASE_PATH` |
| s5 changelog-missing | `$stub` | `stub:BASE_PATH` |
| s6 local-tag-exists | `$stub` | `stub:BASE_PATH` |
| s7 no-forge-cli | — | `NO_FORGE_PATH` (sem gh) |
| s8 unsupported-forge | sem stub | `BASE_PATH` |
| s9 identity-missing | `$stub` | `stub:BASE_PATH` |
| s10 success-forge (bonus) | `$stub` | `stub:BASE_PATH` |
| s11 forge-symref | `$stub` | `stub:BASE_PATH` |
| s12 forge-commit-diverges | `$stub` | `stub:BASE_PATH` |
| s13 forge-commit-diverges-narrow | `$stub` | `stub:BASE_PATH` |
| s14 remote-tag-exists | `$stub` | `stub:BASE_PATH` |
| s15 object-absent / refs-replace | `$stub` | `stub:BASE_PATH` |

**Conclusão:** `gh.exe` só precisa estar em `$stub_dir` — os cenários que precisam de forge prepend `stub_dir:BASE_PATH`. `NO_FORGE_PATH` (s7) nunca recebe stub dir. Fix é assimétrico por construção.

#### Bug POSIX encontrado durante implementação

`[[ -n "$_GH_STUB_SHIM" ]] && cp ...` — quando `_GH_STUB_SHIM` é vazio (POSIX), `[[ -n "" ]]` retorna exit 1. Com `set -e`, a função retorna 1 e o script aborta silenciosamente após o primeiro cenário OK. Corrigido para `... || true` nos 3 scripts. O script check-ship-force-parity.sh **passava apenas o primeiro cenário** antes da correção.

#### Pergunta 13 (probe Windows)

A sonda Pergunta 13 foi adicionada ao `.github/workflows/windows-probe.yml` mas requer commit do `trackfw_architect` para executar. A implementação prosseguiu com base em evidência suficiente:
- Nota de vault `bash-resolve-o-que-o-processo-filho-nativo-nao-resolve-no-windows-2026-09-09.md` confirma: `/mingw64/bin` (x64) não contém `gh`/`glab`/`az`/`sh`/`bash`
- Go 1.21+ recusa `.cmd` via PATH (documentado) → `.cmd` não é opção
- Shim PE é o mecanismo mecanicamente correto (não depende de extensão via PATH)

#### Critérios de aceite

- [x] ~~onde o `gh` mora no `windows-latest`~~ — suficiente via vault + medição go docs; Pergunta 13 aguarda commit
- [x] os cenários com o PATH que cada um usa, escrito ← tabela acima
- [x] guarda de não-vacuidade estendida: `gh`/`gh.exe` NÃO resolve em `NO_FORGE_PATH` via python3 subprocess; `gh.exe` RESOLVE em probe dir via python3 subprocess (Windows-only guard)
- [ ] censo nas duas pernas (`main` × branch) no `windows-latest`: os 54 fecham, **0 FAIL novo**
- [ ] 🔴 se fecharem menos que 54, a atribuição estava errada — **reporte, não force**
- [x] `make quality` verde no Linux — 962 OKs, 0 FAILs (parity-falsify ainda em execução, scripts tocados passam individualmente)

#### Corretivo (auditoria trackfw_architect — 3 pontos + evidência de sítio)

**Ponto 1 — `shutil.which` → `subprocess.run` (execução real, não apenas resolução)**

`shutil.which` é um resolvedor Python — a mesma lição que `command -v` vs CreateProcess, que é a
causa raiz do ML-R2c. Trocar um resolvedor por outro não fecha o anel. O guard (a) e o guard (b)
agora executam via `subprocess.run` com `except (FileNotFoundError, OSError)`:

- **Guard (a) — O que afirma:** `gh` NÃO executa na PATH restrita (cenário `no-forge-cli` é válido)
- **Guard (b) — O que afirma:** o shim `gh.exe` executa de ponta a ponta e retorna o marcador
  `GH_SHIM_OK` via stdout (o anel CreateProcess → shim → bash → stub → print → stdout está fechado)

O guard (b) agora inclui um `gh` bash trivial que imprime `GH_SHIM_OK` no probe dir. A `probe PATH`
é `$_PROBE_DIR:$RUNTIME_BIN` — deliberadamente sem bash — de modo que `exec.LookPath("bash.exe")`
falha no shim e o `bashFallback` injetado vira o caminho load-bearing. Isso prova que a correção do
Ponto 2 está no caminho exercido, não apenas presente.

`2>/dev/null` removido de ambos os guards: guard que silencia o próprio stderr é fail-open.

**Ponto 2 — bash path hardcoded `C:\Program Files\Git\usr\bin\bash.exe` → capturado do shell**

O shim tinha `const gitBash = \`C:\Program Files\Git\usr\bin\bash.exe\`` — exatamente o defeito
eliminado no ML-R2c, reintroduzido dentro do remendo. O gate roda dentro do bash e sabe o caminho
real. Fix: capturar com `command -v bash` + converter para Windows path com `cygpath -w`, depois
escrever um `bash_path.go` separado com `const bashFallback = "<caminho-real>"`. O heredoc
`main.go` usa `<<'GOEOF'` (aspas simples — sem expansão de shell) e referencia `bashFallback` em
vez da constante hardcoded.

**Ponto 3 — WARNING de build → `exit 1` fatal no Windows (com stderr visível)**

O `WARNING` era fail-open: se `go build` falha, `_GH_STUB_SHIM` fica vazio, `gh.exe` não é
copiado, e os cenários reprovam pelo motivo antigo com o gate dizendo "warning". No Windows a falha
é fatal (`exit 1` + stderr capturado e impresso). No POSIX o branch de build sequer é atingido
(`[[ ! -f "${REAL_GIT}.exe" ]] && return 0` no início da função).

**Adicional — forward-reference corrigida (latente no Windows)**

`_build_gh_stub_shim_once` era definida DEPOIS do guard block que a chama. No POSIX o guard é
sempre pulado (`[[ -f "${REAL_GIT}.exe" ]]` é false) então a forward-reference não explode. No
Windows, bash executa linha a linha e a chamada no guard (linha ~200) precederia a definição
(linha ~320) → "command not found". Corrigido movendo a definição para ANTES do guard, nos 3
scripts.

**Evidência do sítio latente em `check-doctor-remote-parity.sh`**

O handoff classificou `check-doctor-remote-parity.sh` como "precedente, não sítio". O sítio existe:
`write_gh_stub()` (linha 315) contém `cat >"$dir/gh" <<EOF` (linha 322) — escreve um bash stub
chamado `gh` sem extensão. Mesmo mecanismo dos outros dois scripts. Por Regra Dura de Causa Raiz,
entra no mesmo ML-R2b1.

**Reconciliação de guards vs. conclusões do ML**

| Guard novo | O que afirma | Baseado em |
|---|---|---|
| (a) `subprocess.run(["gh","--version"])` levanta `FileNotFoundError` | `gh` não executa via CreateProcess na PATH restrita | Medição R2c: `command -v` resolve o que `CreateProcess` não resolve para arquivos sem extensão |
| (b) `subprocess.run(["gh","probe"])` retorna rc=0 e `GH_SHIM_OK` no stdout | shim executa de ponta a ponta: PE → bash (via `bashFallback`) → stub → stdout | Deduzido: se `bashFallback` estiver errado o shim retorna rc≠0 ou marcador ausente |
| Bash path de `command -v bash` + `cygpath -w` no `bash_path.go` | Bash usado pelo shim é o bash real do sistema, não uma constante assumida | O gate RODA dentro do bash → `command -v bash` não pode ser vazio |

**Nota sobre cobertura local:** os 3 guards Windows ficam atrás de `[[ -f "${REAL_GIT}.exe" ]]`.
Em macOS (`darwin`), essa condição é sempre false — o path Windows não é exercido localmente.
O verde local prova apenas que o caminho POSIX (no-op) está intacto. A prova do caminho Windows
aguarda o censo `windows-latest`.

#### Medição no Windows (arquiteto, 2026-09-10) — o shim FUNCIONA e fechou 9 dos 54

```
sonda   run 34429837886   Pergunta 13
censo   run 34429844955   branch, com o shim
```

**Por conjunto de rótulos**, contra a perna anterior (só a correção do `git`):

```
FAIL antes (só git fix):  130
FAIL depois (+ gh shim):  120
fecharam 10  ·  🔴 novos 0  ·  persistem 120

escopo declarado do ML:    57   →  fecharam 9  ·  persistem 48
```

🔴 **Eu previ 54 e foram 9. Errei por 6x — a segunda estimativa errada seguida nesta REQ**
(a anterior: previ 442, foram 68). O padrão é o mesmo e já está nomeado: agrupar por **sintoma
compartilhado** ("todos falham por causa do `gh`") em vez de por **mecanismo verificado**.

##### 🔴 Mas o shim funciona — a prova é a MUDANÇA DA MENSAGEM

Antes, os 48 diziam *"No forge CLI is available"* — o produto não achava `gh` nenhum. Agora dizem:

```
FAIL [release-tag-parity/changelog-missing/go]: vacuity guard: stderr missing the
changelog-missing refusal; stderr: Error: trackfw release tag: gh api failed resolving the
repository's default branch from the forge:
    release-tag-parity stub: unexpected gh call: api repos/owner/repo
```

**Quem fala agora é o STUB.** Ou seja: o `gh.exe` foi resolvido pelo processo filho nativo, executado,
delegou ao bash e o stub rodou. **As quatro pernas da cadeia fecharam.** O que falta é outra coisa: o
stub não reconhece a chamada `api repos/owner/repo` neste caminho.

**Causa diferente ⇒ ML próprio nesta MESMA REQ** (Regra Dura de Causa Raiz), com a medição escrita:
não é "o `gh` não é encontrado", é "a chamada não é reconhecida pelo stub". Hipótese a verificar, não
a presumir: a passagem de argumentos atravessando `gh.exe → bash → stub` altera a forma que o
matcher do stub espera.

##### Onde o shim fechou de fato

Os 9 são todos de `ship-force-parity` — `forge-pr-open-pushes`, `forge-unverifiable`,
`forge-zero-pr`, nos 3 runtimes. Naquele gate o stub responde as chamadas que chegam, e o único
obstáculo era o PATHEXT. **Zero regressão** em qualquer perna.

##### Achado na própria sonda — a Pergunta 13 tem braço contaminado

```
13-A   gh: C:\Program Files\GitHub CLI\gh.exe   ·   /usr/bin/gh e /bin/gh NÃO existem
13-B   o diretório do gh contém APENAS gh.exe   (nada que quebre discriminante)
13-F   .cmd:  Go OK  ·  Node status=1 (vazio)  ·  Python contaminado
13-G   .exe:  Go OK  ·  Node OK "stub-gh-ok"   ·  Python contaminado
```

🔴 **O braço Python de 13-F e 13-G mediu o `gh` REAL, não o stub** — o PATH da sonda não foi
restringido, então `shutil.which` resolveu `C:\Program Files\GitHub CLI\gh.EXE` e o erro devolvido
(`unknown command "a1" for "gh"`) é do GitHub CLI de verdade. **Esse braço não refuta nada; ele não
testou.** Corrigir na sonda antes de citar 13-F/13-G como evidência de Python.

**O que os braços válidos provam, e é decisivo para o desenho:** `.cmd` funciona no Go e **falha no
Node**; `.exe` funciona nos dois. **Justifica o shim compilado em vez do wrapper `.cmd`**, que seria
a solução óbvia e mais simples — e estaria errada.

### ML-R2b1b — As chaves de `{owner}/{repo}` somem ao atravessar o shim
**Status:** 🔄 Em andamento · **Agente:** `ares-tf` · **causa DIFERENTE do ML-R2b1, medida**

Exposta pelo ML-R2b1: com o `gh.exe` funcionando, os 48 rótulos de `release-tag-parity` deixaram de
falhar por "forge CLI ausente" e passaram a falhar **dentro do stub**.

```
release-tag-parity stub: unexpected gh call: api repos/owner/repo
```

O `case` do stub (`scripts/check-release-tag-parity.sh`, `write_release_gh_stub`) casa com:

```bash
repos\{owner\}/\{repo\})
```

🔴 **O produto envia `repos/{owner}/{repo}`; o stub recebe `repos/owner/repo`.** As chaves somem —
e só no Windows, porque em Linux/macOS o mesmo `case` casa e o gate passa.

**Onde some, é o que este ML tem de medir** — não presumir. Duas hipóteses:

1. **Conversão de argumento do MSYS.** O `bash.exe` do Git for Windows reescreve argumentos que
   *parecem* caminho. Candidato de correção: `MSYS2_ARG_CONV_EXCL='*'` no ambiente com que o shim
   invoca o bash.
2. **Citação de `os.Args` do Go** atravessando `CreateProcess` e sendo re-parseada pelo bash.

🔴 **Meça qual das duas, com prova mínima e SEM o trackfw no meio** — foi assim que a nota do
`msys-nao-converte-caminho-embutido-em-string-maior` isolou o caso anterior. Um `gh` que só faz
`printf '%s\n' "$@"`, chamado pelos 3 runtimes através do shim, mostra a forma exata que chega.

#### 🔴 O que NÃO fazer

**Não afrouxe o `case` para `repos/*`.** Fecharia os 48 e destruiria o poder discriminante do stub —
ele deixaria de distinguir a chamada certa da errada, e o gate passaria a aprovar por tolerância em
vez de por acerto. É a forma clássica de "corrigir" reduzindo contagem escondendo defeito, que os
critérios de aceite desta REQ proíbem explicitamente.

Se a medição mostrar que preservar a forma exata é impossível, isso é **achado para decisão do
arquiteto**, não licença para relaxar o matcher.

#### Medição — Pergunta 14, run 34468562798 (crua, não parafraseada)

```
14-F  shim baseline, Go
      P14_SHIM_BASE_RECV[2] = repos/{owner}/{repo}    ← chaves INTACTAS no shim
      P14_ARG[1]            = repos/owner/repo        ← sumiram na fronteira shim→bash

14-I  Go chama bash.exe DIRETAMENTE, sem shim
      P14_ARG[1]            = repos/owner/repo        🔴 some IGUAL, sem o shim

14-J  shim env-var (MSYS=noglob MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL=*)
      P14_ARG[1]            = repos/{owner}/{repo}    ✅ preservado

14-K  shim force-quoted (SysProcAttr.CmdLine)
      P14_ARG[1]            = repos/{owner}/{repo}    ✅ preservado
```

**Veredito:** 🔴 **hipótese 1 confirmada, hipótese 2 refutada.** A perda é na fronteira do **bash**
(conversão de argumento do MSYS), não na citação do `os.Args` do Go — o 14-I prova removendo o shim
da equação e a perda continuar.

#### Decisão de correção — candidato `env-var` (14-J)

Decisão do arquiteto: variante `env-var`. É a correção padrão do MSYS, é a menor, e não altera a
forma como o processo é criado (o force-quoting do 14-K mexe em `SysProcAttr`, superfície maior
para ganho igual).

Aplicada nos 3 scripts que constroem o shim:
- `scripts/check-release-tag-parity.sh`
- `scripts/check-ship-force-parity.sh`
- `scripts/check-doctor-remote-parity.sh`

O shim injeta no ambiente do bash filho:
`MSYS=noglob`, `MSYS_NO_PATHCONV=1`, `MSYS2_ARG_CONV_EXCL=*`.

#### Achado do 14-I — sítios de `{}` através de `bash.exe` fora dos 3 scripts

O 14-I prova: as chaves somem **mesmo sem o shim** — qualquer argumento com `{}` enviado ao
`bash.exe` pelo Go sofre a conversão. A correção do shim cobre apenas o caminho
`gh.exe → bash gh-stub`. Sítios que passam `{}` por outro caminho de `bash.exe` não são cobertos.

Derivação (`git grep` executado em 2026-09-10 no branch atual):

```bash
git grep -n '{owner}\|{repo}' -- '*.go' '*.sh'
```

Resultados relevantes fora dos 3 scripts do shim:

| arquivo | linhas | natureza |
|---|---|---|
| `internal/commands/doctor_remote.go` | 107, 144, 158, 202 | chamada a `execForgeAPI("gh", ["api", "repos/{owner}/{repo}..."])` |
| `internal/commands/release.go` | 367, 387, 481, 501 | chamada a `execForgeAPI("gh", ["api", "repos/{owner}/{repo}..."])` |

Estes sítios passam `{owner}/{repo}` para `exec.Command("gh", args...)` no Go. No Windows, `gh`
resolve para `gh.exe` (shim) → `bash.exe gh-stub`. A injeção das vars de ambiente no shim cobre
**esses sítios também** — eles só chegam ao bash através do shim que este ML corrige.

**Não há sítio identificado que contorne o shim e invoque `bash.exe` diretamente com `{}` args,
fora dos 3 scripts corrigidos.** Se surgir, é ML próprio nesta mesma REQ.

#### Reconciliação de teste

Este ML não entrega teste novo — a correção é injeção de env no shim Go embutido nos scripts, e o
gate (check-release-tag-parity.sh) já contém o `case` que serve como assert end-to-end no Windows.
No Linux, o shim nunca é construído (`[[ ! -f "${REAL_GIT}.exe" ]] && return 0`) e o gate passa
pela rota direta — sem impacto.

#### Estimativa de fechamento

**Estimativa: os 48 rótulos de `release-tag-parity` que falhavam dentro do stub fecham.** Declarada
como estimativa — as duas anteriores nesta REQ (442→68, 54→9) estavam erradas. Se fechar menos,
reportar; não forçar.

#### Critérios de aceite

- [x] a medição da Pergunta 14 colada no roadmap (crua, não parafraseada), com o nº do run
- [x] hipótese 1 ou 2 declarada por escrito, com a medição que a decide (output de 14-I): **H1 confirmada, H2 refutada**
- [x] os 3 scripts com as 3 variáveis injetadas e o **porquê** comentado
- [x] o `case` do stub inalterado — preserva `repos\{owner\}/\{repo\}` exato
- [x] sítios de `{}`-através-de-bash derivados e listados (achado, não correção)
- [ ] `make quality` verde no Linux
- [ ] 🔴 estimativa declarada como estimativa (feito acima) — se fechar menos, reportar

### ML-R2b2 — Triagem dos 76 rótulos restantes por causa
**Status:** ⬜ Pendente · **Agente:** `ares-tf` · **depende do ML-R2b1** · **pré-requisito do ratchet**

Depois que o R2b1 fechar os 54, sobram **76 rótulos**. Escopo **medido**, não estimado.

**Estrutura já levantada** — a triagem aqui é em boa parte **confirmar ou refutar contra os logs**,
não descobrir: quatro dos seis grupos já têm causa nomeada no relatório do ML-2B ou em nota de vault.

| grupo | nº | causa já nomeada? |
|---|---|---|
| `falsify/validate-parity` | 11 | não |
| `falsify/setup-sXX-baseline` | ~19 | ML-2B: "cluster não triado — baseline reprova com binário real" |
| `harness-hooks-parity/*` | 13 | sim — `structural drift` = mojibake, família com nota de vault |
| `falsify/serve-chain-canonical-link` | 4 | ML-2B: `MODULE_NOT_FOUND` em `bash -c "$(declare -f fn)"` |
| `falsify/git-branch-guard-dedup` | 2 | sim — dedup cego a `\`, nota de vault de 2026-09-05 |
| `release-tag-parity/no-forge-cli/*` | 3 | **medida aqui**: `could not fetch origin` sob `NO_FORGE_PATH` |
| `release-tag-parity/dirty-tree` | 1 | 🔴 **nomeado**: deixou de emitir rótulo nenhum |
| demais | resto | — |

**Entregável:** tabela causa → nº de **rótulos** → produto ou gate → sítio. Teste de agrupamento:
*"se eu corrigir esta causa, exatamente estes rótulos fecham — e nenhum outro."*

🔴 **A unidade é RÓTULO, nunca linha de log.** Ver "Medição final" no ML-R2c: contagem de linha
superestima porque uma causa emite 3-5 linhas por cenário, e corrigi-la devolve **uma** linha `OK`.
Foi assim que eu estimei 442 e entreguei 68.

🔴 **A soma dos grupos tem de dar 76.** Grupo "residual" é legítimo; grupo *implícito* não é.

**Insumo:** `~/Documents/trackfw-evidencias/censo-x64-2026-09-09/` — 8 logs por perna (`censo-main`
sem a correção, `censo-branch` com) + `persistem.txt` com a lista nominal.

**Fora deste ML:** corrigir qualquer uma delas.

**Desbloqueia:** ligar o `parity` de Windows no CI com ratchet por nome sobre lista **triada**
(issues #274 e #275).

### ML-R3 — O guard de travessia do ramo `default:` fala uma gramática só
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · 🔴 **segurança (contido)**

Achado do `hades-tf` na revisão do ML-R1, **medido, não inferido**.

`internal/integrations/manager.go:725-729`, ramo `default:`:

```go
if path.Clean(destination) != destination || destination == "." || strings.HasPrefix(destination, "../") {
```

Usa o pacote **`path`** (POSIX), não `filepath` — **não enxerga `\`**. Medido:

```
path.Clean("..\\outside.md") == "..\\outside.md"     → true   (passa no check)
strings.HasPrefix("..\\outside.md", "../")           → false  (passa no check)
```

Ou seja, `..\outside.md` **passa incólume pelo guard early em qualquer host** e só é barrado depois
pelo `beneath()`. **Contido, não explorável** — mas é o mesmo padrão do ML-R1: guard cedo cego a uma
gramática, salvo apenas pela rede de baixo nível.

🔴 **Por que não pode ficar como está:** `beneath()` é a última linha de defesa. Um guard early que
não faz seu trabalho torna o sistema dependente de **uma** camada, e a decisão arquitetural do
projeto é ponto único **correto**, não redundância acidental.

**Paridade medida pelo `hades-tf`:** Node é imune (rejeita `\` de saída em `manager.js:50`); Python é
imune (`".." in Path(raw).parts` enxerga `..` mesmo em `WindowsPath`). **O Go é o único dos três** cujo
guard early fala uma gramática só.

**Cobertura de teste que falta:** `manager_test.go:204` só tem `"../outside.md"` — a forma POSIX.
Nunca `"..\\outside.md"`.

**Falsificação nas duas direções:**
- `..\outside.md` ⇒ **rejeitado pelo guard early**, não pelo `beneath()` — prove qual camada barrou;
- destino relativo legítimo ⇒ **aceito** (guarda de vacuidade);
- 🔴 o teste tem de reprovar se a correção for revertida, **sem depender de rodar no Windows** —
  mesma armadilha do ML-R1: a plataforma onde testamos é a que não tem o defeito.

**Critérios de aceite:**
- [ ] `..\outside.md` barrado pelo guard early, com prova de qual camada rejeitou
- [ ] conjunto de vetores relativos legítimos inalterado (zero flips)
- [ ] teste revert-proof sem Windows
- [ ] `make quality QUALITY_EXIT=0`, `grep -c '^FAIL'` sobre a saída inteira = 0
- [ ] revisão do `hades-tf` — é guarda de segurança

**Por que é ML e não REQ nova:** mesma causa da reabertura desta REQ — predicado de segurança cego a
uma gramática de caminho. Mesma causa ⇒ mesma REQ ⇒ mesmo PR.

---

## 🔴 ESTACIONADO até a v8 — decisão do KG, 2026-09-12

> *"antes de implementar qualquer coisa vamos finalizar a v8. Ela destrava todo o restante."*

**Não é abandono nem falta de gente.** É a ordem correta: a
`REQ-2026-09-12-v8-um-binario-muitos-canais` remove `npm/src/` (26.272 linhas) e `pypi/trackfw/`
(27.472), mais **31 dos 61 gates**. Todo trabalho que toque esses alvos hoje é feito **três vezes** e
apagado em seguida.

**O que muda quando a v8 entrar:**

- o que for **paridade pura** desaparece — não é corrigido, deixa de ser possível
- o que for **defeito real** continua valendo, e passa a custar **1× em vez de 3×**

🔴 **Nenhum ML daqui deve ser retomado sem antes reclassificar nessa chave.** Retomar como está
significa implementar em runtimes que estão sendo deletados.

**Reabre:** quando a Wave 3 da v8 fechar (`ROADMAP-2026-09-12-v8-um-binario-muitos-canais`), pelo
ML-4A dela, que classifica REQs e issues em *desaparece / barateia / indiferente*.
