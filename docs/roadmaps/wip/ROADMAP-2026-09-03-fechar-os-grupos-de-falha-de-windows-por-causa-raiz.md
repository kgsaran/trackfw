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
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
**Resolve TRÊS grupos de uma vez** (~45 testes): `tildeify` devolvendo `~\...`, `provenanceKey`
nativo no Node, e caminho em JSON lido por CLI de agente.
Evidência que tende a **sim**: a chave de proveniência **já é** `/` por decisão documentada; `~` é
POSIX-ismo que nenhum shell do Windows expande; um `command` bash com `\` é mastigado pelo shell.

### ML-1B — ADR: o parser de frontmatter deve tolerar CRLF?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
O parser é **cego a CRLF** e emitiu frontmatter **duplicado** em `TestRenderOpenCodeAgent`. ~14
testes. 🔴 A alternativa — declarar `eol` sobre os assets — **foi medida e recusada** no ML-1C:
esconde o defeito em vez de curá-lo.

### ML-1C — ADR: caminho POSIX ancorado num config lido por CLI de agente é "absoluto"?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
`filepath.IsAbs("/opt/…")` é **falso** no Windows → `classifyHookAnchorage` classifica ancorado como
relativo → **o validator deixa de emitir violation de guard ausente**. ~14 testes, **e é de
segurança**: a detecção de hook de guard **enfraquece no Windows**.

## Wave 2 — Separador, nos 3 CLIs
> Dependências: ML-1A `Accepted`.

### ML-2A — Separador POSIX em artefato autorado
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Files affected — os 3 stacks:** `npm/src/lib/update-engine.js:172-181`,
`pypi/trackfw/commands/update_harness.py::_tildeify`, `internal/integrations/manager.go`,
`npm/src/validator/index.js:3153` (`provenanceKey` sem normalização), `npm/src/serve/api_chain.js`
⚠️ `npm/src/validator/index.js` é classificado como **binário** pelo `file` — `grep` sem `-a` o pula
**em silêncio**; 2 REQs deste repo têm premissa falsa por isso.
**Critérios:** falsificação nas duas direções · controle POSIX com números · os 3 CLIs dão o **mesmo**
resultado · recontagem no CI com o delta atribuído a este grupo.

## Wave 3 — `IsAbs`, sozinho e sequencial
> Dependências: ML-1C `Accepted` **e** a branch `fix/validate-detecta-hook-de-guard-...` fechada.

### ML-3A — Caminho POSIX ancorado deixa de ser classificado como relativo
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` + barreira `hades-tf`
🔴 **É segurança e COLIDE com outra branch.** Não paralelizar com nada.
`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`, e pares nos
outros 2 CLIs.

## Wave 4 — Resíduo (paralelo, arquivos disjuntos)
> Dependências: Wave 2.

### ML-4A — Bit de execução em NTFS
**Status:** ⬜ Pendente · **Agente:** `artemis-tf` · ~22 testes, **decisão já tomada** no vault:
`goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`. **Não relitigar** — guard de
plataforma no assert.

### ML-4B — `WinError 32`, `.sh` sem `bash`, `stale_wip` off-by-one
**Status:** ⬜ Pendente · **Agente:** `artemis-tf` · ~15 testes, todos de teste, todos disjuntos.
O `stale_wip` é **truncamento**, não fuso horário — a hipótese de TZ foi **falsificada** na triagem.

## Verificação que só o CI fecha

A contagem por runtime, **medida após cada wave** e com o delta **atribuído** ao grupo. Sem
atribuição não se sabe qual correção funcionou — e nesta REQ eu já errei uma estimativa por 2,5x.

## Barreira final

`hefesto-tf` e `hades-tf`. O Hades é **obrigatório** na Wave 3 (segurança) e na Wave 2 (caminho em
config lido por CLI que executa bash).
