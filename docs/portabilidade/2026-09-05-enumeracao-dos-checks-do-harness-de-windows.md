---
title: Enumeração e classificação de todos os checks do harness de Windows (ML-1A, AC7)
date: 2026-09-05
author: ares-tf (investigação pura — nenhuma correção aplicada)
status: investigação concluída
---

# Enumeração dos checks do harness de reprodução de Windows

> 🔴 Documento de investigação pura. Nenhum check foi corrigido, nenhuma correção de produto foi
> alterada, nenhuma operação de git foi executada. `internal/generators/agentfiles.go` e pares
> Node/Python não foram tocados (agente paralelo).

## Escopo lido

`scripts/windows-repro/` tem **dois instrumentos distintos**, e essa distinção é o primeiro achado
não óbvio — confundi-los inflaria ou desinflaria a contagem:

1. **`run.ps1` + `go/checks.go` + `node/checks.js` + `python/checks.py`** — a suíte de
   **reprodução de defeito** (camada 2), mapeada aos itens da issue #216, com veredito
   `REPRODUCED`/`ABSENT`/`INCONCLUSIVE`/outros rótulos declarados. Roda em **todo push/PR**
   (`quality.yml`, job `windows-defect-reproduction`) e **decide pass/fail**.
2. **`go/probe.go` + `node/probe.js` + `python/probe.py`** — a **sonda sob demanda**
   (`.github/workflows/windows-probe.yml`), que o próprio cabeçalho do YAML e de cada `probe.*`
   declara explicitamente **não ser** a suíte de regressão: sem veredito, `workflow_dispatch` apenas,
   nunca em push/PR, "`checks.*` nunca devem importar as funções deste arquivo." Confirmado por
   leitura: nenhuma referência a `probe.go/js/py` existe em `run.ps1` nem em `quality.yml`.

O AC7 pede "todos os checks do harness". Enumero os dois, mas só o instrumento (1) tem sentido de
ser classificado por "invoca o produto / replica o mecanismo / mede a plataforma" no critério de
**defeito**, porque só ele emite verdito sobre um estado do produto e afeta o gate. O instrumento (2)
está corretamente fora do gate — não é auto-declarado confirmatório escondendo um substituto; é
**outra ferramenta**, e a barreira que a mantém fora do CI de regressão está escrita no próprio YAML
("NÃO adicionar `push`/`pull_request` a este workflow").

## Parte 1 — Suíte de reprodução de defeito (`run.ps1`, camada 2, verdict-bearing)

11 itens mapeados da issue #216 (+ 1 sonda observacional injetada no meio, item 12 — ver achado
novo abaixo). Dos 11, 3 são sentinelas declaradas sem execução (itens 8, 9, 11) e não emitem
`REPRODUCED`/`ABSENT`/`INCONCLUSIVE` — não entram na tabela de classificação porque não medem nada
em runtime.

| item | mecanismo exato (arquivo:linha) | classificação | evidência |
|---|---|---|---|
| **1** | `python/checks.py:44-49` — `subprocess.run([sys.executable, "-c", "from trackfw.cli import main; ...; main()"])` | **INVOCA O PRODUTO** | Chama `trackfw.cli.main()` de verdade, sem argv (`args.command is None` → `parser.print_help()`), sob console sem `PYTHONUTF8`/`PYTHONIOENCODING` — o mesmo caminho que crasha em produção. |
| **2** (Go) | `go/checks.go:52-58` (`cmdHome`) — `os.UserHomeDir()` cru | 🔴 **MEDE A PLATAFORMA** (defeito, instância já conhecida) | Produção **nunca** chama `os.UserHomeDir()` direto: `homedir.Dir()` (`internal/homedir/homedir.go:31-36`) prefere `$HOME` antes de cair para `os.UserHomeDir()`. `grep -rn "homedir.Dir\|UserHomeDir" internal/` confirma **21 call sites de produção usam `homedir.Dir()`, zero usa `os.UserHomeDir()` cru fora do próprio helper**. O comentário do check (linhas 47-51) afirma "chama exatamente o que a produção chama" — **essa afirmação é falsa**, e é o mesmo padrão dos itens 3/4/7: o comentário descreve intenção, não o que o código faz. |
| **2** (Node) | `run.ps1:144` — `node -e "console.log(require('os').homedir())"` | **MEDE A PLATAFORMA** (mesma instância) | Chama `os.homedir()` do runtime Node cru; produção usa `homedir()` de `npm/src/homedir.js` (citado no docstring do pacote Go como par canônico), nunca importado aqui. |
| **2** (Python) | `run.ps1:146` — `python -c "import os; print(os.path.expanduser('~'))"` | **MEDE A PLATAFORMA** (mesma instância) | Chama `os.path.expanduser` cru; produção usa `home_dir()` de `pypi/trackfw/homedir.py`, nunca importado aqui. |
| **3** | `go/checks.go:67-88` (`cmdExecBit`) — `os.Chmod` + `os.Stat` num arquivo temp, sem tocar nenhum pacote `trackfw` | 🔴 **MEDE A PLATAFORMA**, declarado confirmatório em comentário, **mas não excluído do gate** | Comentário próprio diz "Confirmatório — evidência primária é camada 1" (linhas 61-66) e `run.ps1:169-173` repete "(confirmatório)" no título. **Apesar disso**, o veredito entra em `$results` (`Add-Result` linha 173) e participa de `$reproduced.Count` (linha 664) e da condição de saída do job (`exit 1` na linha 683) — a mesma pauta que decide pass/fail do CI. É exatamente o cenário que a AC2/ML-2B pede para resolver: "declarado confirmatório" no comentário **não** é o mesmo que "fora da contagem de `REPRODUCED` corrigíveis" no código. Hoje não está fora. |
| **4** | `python/checks.py:66-107` (`cmd_cp1252_print`) — `subprocess.run([sys.executable, "-c", "print('\\u2192')"])` | 🔴 **REPLICA O MECANISMO** (defeito, instância já conhecida) | Não invoca `scripts/check-parity-contract-coverage.sh` em nenhum ponto — o próprio docstring (linhas 70-75) admite "SEM invocar o wrapper .sh, evita confundir o item 4 com o item 7". Mede um `print()` cru de `→`, mecanismo que existia **antes** do fix de 2026-09-02 (que adicionou `PYTHONIOENCODING=utf-8` ao `.sh`, linha 54 do script real) e que o `.sh` real já neutraliza — a sonda mede uma pergunta que o artefato corrigido não deixa mais em aberto. |
| **5** | `python/checks.py:159-209` (`cmd_crlf`) — `subprocess.run([..., "from trackfw.cli import main; ...; main()"])` com `sys.argv=['trackfw','init','--identity-preset','none']`, depois varre bytes crus dos `.sh` gerados | **INVOCA O PRODUTO** | `trackfw init` real, sem mock, seguido de leitura de bytes do artefato real gerado. |
| **6** | `python/checks.py:212-263` (`cmd_isatty`) — mesmo padrão, `sys.argv=['trackfw','init']`, `stdin=subprocess.DEVNULL` | **INVOCA O PRODUTO** | `trackfw init` real sob `stdin=NUL`, sem monkeypatch de `isatty()` (ao contrário dos testes que a Wave 0 já achou vácuos). |
| **7** (Go) | `go/checks.go:134-144` (`cmdGateQuote`) — `exec.Command("sh", "-c", gateQuoteCommand)` sobre o literal `gateQuoteCommand`, não sobre `barrier.go` | 🔴 **REPLICA O MECANISMO** (defeito, instância já conhecida) | `internal/commands/barrier.go:729` é quem chama `exec.Command("sh","-c",...)` em produção — este check reimplementa a mesma chamada num arquivo isolado, nunca importa nem invoca `barrier.go`. |
| **7** (Node) | `node/checks.js:23-34` (`cmdGateQuote`) — `spawnSync(GATE_QUOTE_COMMAND, { shell: true })` sobre o mesmo literal, não sobre `barrier.js` | **REPLICA O MECANISMO** (mesma instância) | Comentário próprio (linhas 7-12) admite: "Replica o MESMO primitivo que `npm/src/commands/barrier.js:561` usa" — réplica declarada, nunca invoca `barrier.js`. |
| **7** (Python) | `python/checks.py:274-291` (`cmd_gatequote`) — `subprocess.run(GATE_QUOTE_COMMAND, shell=True)` sobre o mesmo literal, não sobre `barrier.py` | **REPLICA O MECANISMO** (mesma instância) | Mesmo padrão — docstring (linhas 275-280) admite ser réplica de `pypi/trackfw/commands/barrier.py`, nunca a invoca. |
| **7** (auxiliar `shc`) | `go/checks.go:97-109` (`cmdShC`) — `exec.Command("sh","-c","echo trackfw-sh-check-ok")` | **MEDE A PLATAFORMA**, mas não é verdict-bearing sozinho | Só alimenta `$item7Detail` como "evidência auxiliar" (`run.ps1:245`); o veredito do item 7 vem exclusivamente da comparação de tokens (`$item7Verdict`, linha 254), não deste sub-check. Não conta como instância adicional — é diagnóstico, não afirmação sobre o produto. |
| **10** (Go) | `run.ps1:274-278` (`go build -o $trackfwBinPath ./cmd/trackfw`) + linha 313 (`Run-Capture -Exe $trackfwBinPath -ArgList @("roadmap","move",...)`) | **INVOCA O PRODUTO** | Compila o binário real de `./cmd/trackfw` e roda `roadmap move` real sobre uma fixture. |
| **10** (Node) | `run.ps1:316` — `node (Join-Path $repoRoot "npm\bin\trackfw") roadmap move ...` | **INVOCA O PRODUTO** | `npm/bin/trackfw` é o entry point real (`require('../src/commands/index').createProgram()`), confirmado por leitura. |
| **10** (Python) | `run.ps1:318-320` — `python -c "from trackfw.cli import main; ...; main()"` com `sys.argv=[...'roadmap','move',...]` | **INVOCA O PRODUTO** | Mesmo padrão dos itens 1/5/6 — chama `trackfw.cli.main()` real. |
| 8 | não executado — `Add-Result` direto com `Verdict = "DECLARED-OUT-OF-SCOPE"` (`run.ps1:260-262`) | sentinela declarada, sem runtime | Não entra em `$reproduced`/`$inconclusive` (filtros por igualdade de string, linhas 664-666). Não é defeito sob o critério do AC7 — é presença documental, não medição disfarçada. |
| 9 | idem, `Verdict = "OUT-OF-SCOPE"` (`run.ps1:267-269`) | sentinela declarada, sem runtime | Idem. |
| 11 | idem, `Verdict = "COVERED-BY-CAMADA-1"` (`run.ps1:345-347`) | sentinela declarada, sem runtime | Idem. |

### Achado novo — item 12 é uma sonda declaradamente fora do escopo, mas contamina o mesmo gate

🔴 **Este é o achado que a campanha ainda não tinha catalogado, e não se encaixa nas três categorias
do AC7** (invoca produto / replica mecanismo / mede plataforma) porque não é sobre nenhum dos 11
itens da issue #216 — é sobre um problema **diferente e já rastreado** (grupo B, resolução ambígua
de `bash` por nome nu no Windows, `docs/qualidade/2026-09-04-grupo-b-bash-do-python-em-windows.md`).

`run.ps1:349-365` declara, no próprio comentário: *"item 12 — SONDA OBSERVACIONAL (ML-0B, NÃO é da
issue #216)... NÃO CORRIGE nada. Nenhum teste é tocado."* Isso é verdade sobre o que ela corrige.
**Não é verdade sobre o que ela afeta**: o veredito da sonda (`BRANCH-A`/`BRANCH-B`/`NOT-REPRODUCED`)
é mapeado para `REPRODUCED`/`ABSENT`/`INCONCLUSIVE` em `run.ps1:646-653`, entra em `Add-Result` como
qualquer outro item, e por isso participa de `$reproduced.Count` (linha 664) e da condição de saída
do job (`exit 1`, linha 683) — **o mesmo contador e o mesmo gate que decidem pass/fail para os itens
1-11 da issue #216**.

Consequência prática: se a ambiguidade de resolução de `bash` (que é um achado de **infra do
runner**, não um defeito do `trackfw` corrigível por um dos MLs deste roadmap) persistir, o job
`windows-defect-reproduction` continua reportando `Reproduzidos: N` incluindo o item 12 — e quem lê
"N reproduzidos" no dashboard/summary não tem como saber, sem ler o `Detail`, que um desses N não é
um dos 11 itens da issue e não pode ser corrigido pelo mesmo tipo de fix (o próprio comentário do
item diz "NÃO CORRIGE nada"). É a mesma classe de problema que motivou este roadmap — **o instrumento
mede algo que não é o que o número presume** — só que a direção é diferente das quatro instâncias
conhecidas: ali o check mede pouco (substituto do produto); aqui o gate conta demais (soma um item
que não é do domínio declarado do gate).

**Não classifico isto dentro da tabela do AC7** porque não é "check do item X medindo substituto do
produto" — é uma questão de **fronteira do instrumento**: um item declaradamente fora do escopo da
suíte de regressão está fisicamente dentro do mesmo arquivo/loop/contador que a suíte de regressão.
Registro como achado separado, para a Wave 3 (ML-3A, recontagem) decidir se isso deveria estar
excluído de `$reproduced`/`$inconclusive` (ex.: um `$results` paralelo para itens "observacionais
misturados por conveniência de execução") ou movido para fora de `run.ps1`.

### Consumidor externo ao harness — `quality.yml:280` reusa `checks.go home`, e não é uma quinta instância

`checks.go home` (a mesma chamada crua de `os.UserHomeDir()` que sustenta o item 2) tem um **segundo
chamador**, fora de `scripts/windows-repro/`: o job `windows-full-suites` em `.github/workflows/
quality.yml`, step "AC12 — confirmar que a isolação de HOME/USERPROFILE vale antes de rodar as
suítes" (`quality.yml:270-293`). O handoff pede para enumerar "qualquer outro arquivo de check" — e
o mesmo primitivo, citado como defeituoso na Parte 1, reaparece aqui como **precondição que decide
se a camada 1 inteira (as três suítes completas) roda ou é pulada**. Vale a pena examinar se é uma
quinta instância disfarçada.

**Não é.** A precondição faz algo estruturalmente diferente do item 2: ela seta `HOME` e
`USERPROFILE` para o **mesmo valor sintético** (`quality.yml:274-276`, ambos apontam para
`steps.home.outputs.home`), e depois confirma que `os.UserHomeDir()`/`os.homedir()`/
`os.path.expanduser` cru resolvem para esse valor (`$expected = $env:USERPROFILE`). Como as duas
variáveis são idênticas aqui, o resultado é o mesmo **independentemente** de o primitivo preferir
`$HOME` ou `%USERPROFILE%` — ao contrário do item 2, que deliberadamente usa `fakeHome ≠
fakeProfile` para expor a diferença. O que esta precondição mede é se a substituição de ambiente do
próprio job (usada para não deixar `go test`/`npm test`/`pytest` escreverem na home real do runner,
AC12) realmente colou — não se o `trackfw` respeita `$HOME`. Não há classificação de defeito aqui
porque não há afirmação sobre o produto: um `$failed = $true` só pode disparar se a mecânica de
`env:` do GitHub Actions falhar em aplicar o valor, não por causa do comportamento de `$HOME` vs
`%USERPROFILE%` que o item 2 investiga.

**Registrado por completude, não como instância nova.** Se este mesmo primitivo crescer um terceiro
consumidor que compare `$HOME ≠ %USERPROFILE%` (ao contrário deste, que usa valores iguais), aí sim
valeria reabrir a pergunta.

## Parte 2 — Sonda sob demanda (`probe.go`/`probe.js`/`probe.py`, `windows-probe.yml`)

19 subcomandos ao todo (Go: `statmode-common`, `statmode-chmod`, `lstat-common`, `lstat-symlink`,
`lstat-junction`, `lstat-path`, `rmdir-junction`, `table` = 8; Node: espelha 5 (`lstat-common`,
`lstat-symlink`, `lstat-junction`, `rmdir-junction`, `table`); Python: espelha os mesmos 5 do Node +
`crlf` reusado de `checks.py`, chamado nas Perguntas 5a/5b/5c). Todos imprimem valor **bruto** do SO
(`os.Stat`/`os.Lstat`/`fs.lstatSync`/`os.lstat`), sem comparação contra esperado, sem
`REPRODUCED`/`ABSENT`.

Classificação: **MEDE A PLATAFORMA por desenho, e declarado como tal** — o próprio cabeçalho de cada
arquivo (`probe.go:1-16`, `probe.js:1-14`, `probe.py:1-14`) e do workflow
(`windows-probe.yml:6-29`) diz explicitamente que não é a suíte de regressão, não tem veredito, roda
só sob `workflow_dispatch`, e proíbe adicionar `push`/`pull_request`. Isto **não é o defeito que o
AC7 pede para caçar** — o defeito é um check que **afirma medir o produto** (nome, comentário, ou
presença no gate) enquanto mede o substituto. A sonda nunca afirma nada sobre o produto; ela imprime
dado bruto para leitura humana. Não entra na contagem de `REPRODUCED` corrigíveis porque **nunca
gerou um `REPRODUCED`** — o vocabulário do dashboard nem existe aqui.

Uma ressalva, para não deixar a leitura incompleta: a Pergunta 5 do `windows-probe.yml`
(`5a`/`5b`/`5c`) reusa `checks.py:crlf` e roda `trackfw init`/`roadmap new` reais pelos 3 binários —
isto é a sonda **invocando o produto** para uma pergunta observacional (terminador de linha), não
platform-raw. Citado por completude; não muda a classificação de "sonda declarada, fora do gate".

## Perguntas do handoff, respondidas

**Quantos checks existem no total, por camada?**

- Camada 2 (`run.ps1`, verdict-bearing, afeta pass/fail): **12 itens numerados** (1 a 12), dos quais
  9 emitem `REPRODUCED`/`ABSENT`/`INCONCLUSIVE`/variantes (1, 2, 3, 4, 5, 6, 7, 10, 12) e 3 são
  sentinelas declaradas sem runtime (8, 9, 11). 🔴 Não publico uma contagem agregada de
  "sub-invocações de runtime" — uma primeira tentativa deu 17, uma recontagem manual linha a linha
  deu 19 (1+1+3+1+1+1+4+4+3, itens 1/4/2/3/5/6/7/10/12 respectivamente, contando o `go build` do
  item 10). A REQ que originou este roadmap existe exatamente para não herdar número não
  verificado — melhor não publicar um agregado que eu não consigo travar com confiança dentro do
  orçamento desta ML do que arriscar um terceiro valor errado. Os números que sustento com confiança
  são os 12/9/3 acima, verificados por leitura direta de cada `Add-Result`.
- Sonda sob demanda (`windows-probe.yml`, não afeta pass/fail, sem veredito): **18 subcomandos**
  entre `probe.go` (8: `statmode-common`, `statmode-chmod`, `lstat-common`, `lstat-symlink`,
  `lstat-junction`, `lstat-path`, `rmdir-junction`, `table`), `probe.js` (5: mesmos `lstat-*`/
  `rmdir-junction`/`table`, sem os dois `statmode-*`) e `probe.py` (5, idênticos a `probe.js`,
  confirmado por leitura do dict `COMMANDS` em `probe.py:214-220`), mais a reutilização de
  `checks.py:crlf` (não um subcomando de `probe.py`) nas Perguntas 5a/5b/5c — **19 invocações do
  workflow ao todo**. Cross-referenciei cada subcomando declarado nos três `COMMANDS`/`switch` contra
  as chamadas em `windows-probe.yml` (`grep -n "probe.go \|probe.js \|probe.py "`): **os 18
  subcomandos são todos invocados pelo menos uma vez** — nenhum existe sem ser chamado, então não há
  vácuo por "nunca executa" nesta camada.

**Quantos são defeito pelo critério do handoff — quantas instâncias além das quatro conhecidas?**

Dentro da Parte 1 (a única que o critério de defeito se aplica): **as quatro instâncias já
conhecidas são as únicas que se encaixam nas três categorias do AC7**, e todas se confirmam por
linha de código:
- item 2 (3 sub-checks: Go/Node/Python) — mede a plataforma.
- item 3 — mede a plataforma, declarado confirmatório em comentário mas **não excluído do gate**
  (a lacuna que ML-2B precisa fechar não é "declarar", é "excluir do contador").
- item 4 — replica o mecanismo.
- item 7 (3 sub-checks: Go/Node/Python) — replica o mecanismo.

**Dentro de `scripts/windows-repro/**`, nenhuma quinta instância dentro da taxonomia do AC7 foi
encontrada.** Itens 1, 5, 6 e 10 invocam o produto de verdade, com linha citada. Itens 8, 9, 11 são
sentinelas sem execução, corretamente fora do contador. A sonda sob demanda (Parte 2) está
corretamente fora do gate, por desenho e por declaração — não é uma quinta instância disfarçada.

**Fora de `scripts/windows-repro/**`, encontrei um consumidor do mesmo primitivo cru** —
`quality.yml:280`, dentro da precondição AC12 do job `windows-full-suites` — e determinei que
**não** é uma quinta instância (ver seção dedicada acima): ele compara `HOME`/`USERPROFILE`
setados para o **mesmo** valor, então o resultado independe de qual variável o primitivo prefere;
não afirma nada sobre o `trackfw` respeitando `$HOME`, só confirma que a substituição de ambiente do
job tomou efeito.

**Existe achado além da taxonomia de três categorias?** Sim — **um**, o item 12 (ver seção acima):
não é "mede plataforma"/"replica mecanismo" no sentido do AC7 (não afirma nada sobre um dos 11
itens), mas contamina `$reproduced.Count`/o exit code do job com o veredito de uma investigação que
o próprio comentário declara "NÃO é da issue #216" e "NÃO corrige nada". Recomendo que a Wave 3
(ML-3A, recontagem) trate isto explicitamente: ou o item 12 sai de `$results` para um sumário
paralelo, ou o roadmap documenta por que ele deve continuar contando (por exemplo, se o time decidir
que qualquer regressão de infra também deve manter o job vermelho — mas isso precisa ser uma decisão
escrita, não um efeito colateral de "ficava mais fácil somar no mesmo array").

**Existe check vácuo por outro motivo — sempre `ABSENT`, ou que nunca executa?**

Não encontrei nenhum item verdict-bearing que **sempre** dê `ABSENT` independentemente do estado do
produto (o padrão oposto do problema que originou esta REQ, mas igualmente vácuo). Os candidatos
mais próximos, descartados por leitura:
- **Item 3** não é vácuo no sentido "sempre ABSENT" — é o oposto: por medir `mode&0111` em NTFS, ele
  é **estruturalmente incapaz de ir a `ABSENT`**, e por isso sempre aparece `REPRODUCED` mesmo com o
  produto corrigido (é o problema já mapeado, não um vácuo novo).
- **Itens 8, 9, 11** nunca executam runtime — mas isso é **declarado e intencional** (residual
  documentado, coberto por outra camada ou fora de escopo), não um gate silenciosamente inerte: eles
  não entram no contador de `REPRODUCED`/`INCONCLUSIVE`, então não mascaram nada — um leitor que
  confia no contador não é enganado por eles.
- **O `RUNNER_TEMP` vazio** (`run.ps1:48-60`) é um vácuo **já corrigido**, documentado no próprio
  comentário do script: antes da guarda, `Join-Path $null "x"` devolvia string vazia sem erro, e o
  item 2 comparava contra `""`, emitindo `REPRODUCED` **incondicionalmente** mesmo com o defeito
  corrigido — o oposto exato do padrão desta REQ (aqui o vácuo produzia falso positivo, não falso
  negativo). Fica registrado porque é o mesmo gênero de achado ("gate vácuo"), mas já tem guarda de
  vacuidade (`$item2Medido`, linha 162) — não é uma instância aberta.
- **item 12 (achado novo, acima)** não é vácuo em si (mede algo real), mas seu efeito no contador é
  o mesmo gênero de risco: um número que não significa o que o dashboard implica que significa.
- **A precondição AC12 do job `windows-full-suites` (`quality.yml:270-293`) pode pular a camada 1
  inteira** (as três suítes completas) se a isolação de `HOME`/`USERPROFILE` falhar — mas isso é
  **declarado, não vácuo silencioso**: o job inteiro tem `continue-on-error: true` explicitamente
  documentado como temporário (`quality.yml:151-157`, AC4, "só remover depois que TODAS as REQs de
  correção dos 11 itens... fecharem"), e o caminho de skip emite `::warning::` nomeando a causa
  (`quality.yml:421-425`), não silencia. Mesmo raciocínio dos itens 8/9/11 e da sonda sob demanda:
  ausência de execução com contrato escrito e critério de remoção datado não é o defeito que este
  AC7 persegue — o defeito é a suposição não verificada de que algo mede o que o nome/comentário
  diz medir.

## Premissas do handoff que esta leitura confirmou ou corrigiu

- A tabela de 4 instâncias do handoff (itens 2, 3, 4, 7) **é precisa** — confirmada linha a linha,
  inclusive contra o código de produção (`internal/homedir/homedir.go`, `barrier.go:729`,
  `barrier.js:561`, `barrier.py:582` citados nos comentários dos próprios checks).
- Os caminhos citados no handoff (`js/checks.js`, `py/checks.py`) **não existem** — os diretórios
  reais são `node/checks.js` e `python/checks.py` (confirmado por `find`). Provavelmente um deslize
  de digitação do handoff; não muda a substância, mas registro porque o handoff pediu para não
  presumir estrutura.
- `internal/generators/agentfiles.go` e pares Node/Python **não foram tocados** por esta
  investigação, conforme restrição do agente paralelo.
- Nenhuma correção de produto ou de check foi alterada; nenhuma operação de git foi executada.
