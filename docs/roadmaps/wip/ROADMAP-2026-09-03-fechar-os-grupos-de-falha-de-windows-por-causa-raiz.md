---
status: wip
date: 2026-09-03
squad: apolo-tf
req: "docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md"
---

# Roadmap: Fechar os grupos de falha de Windows por causa raiz

> Criado em: 2026-09-03 | Status: wip

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


## Wave 5 — CRLF no parser de frontmatter
> Dependências: Wave 3 fechada. **Sequencial**: toca os mesmos arquivos de validator dos 3 CLIs.

### ML-5A — Parser tolera CRLF na fronteira de entrada
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
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
