# Perfil do `check-gates-falsify.sh` — compilação vs execução

> 2026-09-06 · ML-1A, `ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify-sem-perder-cobertura.md`
> **Nenhum código, gate ou Makefile foi alterado nesta wave.** Investigação pura.

## Resposta explícita

**O gargalo é execução, não compilação.** No run local instrumentado, compilação (`go build`/`go run`
dos binários sabotados) somou **81,1 s de 921,4 s (≈8,8%)**. O restante (≥91%) é execução: gates
aninhados que rodam `check-*.sh` inteiros (que por sua vez chamam Go+Node+Python), asserts de
paridade cross-CLI, e overhead de setup (`cp -r`, scripts Python de corrupção, `git`).

🔴 **Isso derruba a premissa central do handoff.** A hipótese de que "paralelizar resolve pouco porque
o custo é compilação que não compartilha" **não se sustenta**: o `GOCACHE` fixo (`$WORK/go-build-cache`,
já documentado nos comentários do próprio script, linhas 37-43) **já é compartilhado** entre as 93
compilações de uma mesma execução — não há invalidação por cópia de árvore para tmp, como o handoff
temia. O 1º build da run é frio (4,85 s); a mediana dos 92 seguintes é **0,84 s**. Reaproveitar
binários entre cenários que sabotam o mesmo alvo teria teto de **~9%** do tempo total — não os "70%"
que uma otimização ingênua prometeria.

## Método

- **Cópia instrumentada** do script em scratchpad (`check-gates-falsify-instrumented.sh`) —
  `scripts/check-gates-falsify.sh` do repositório **não foi tocado**.
- As três funções mais chamadas foram renomeadas para `_impl` e envolvidas por wrappers que gravam
  `KIND\tlabel\tt0\tt1` (epoch com nanosegundos, via `date +%s.%N`) em `PROFILE_LOG`:
  - `build_go_or_fail` → `BUILD` (compila `./cmd/trackfw` isolado)
  - `run_go_guard_dump` → `GORUN` (compila via `go run` um dumper efêmero)
  - `assert_fails_with` → `ASSERT` (executa o comando do cenário e valida exit+mensagem)
- `ROOT_DIR` do script foi parametrizado (`${ROOT_DIR:-...}`) para apontar ao repositório real a
  partir da cópia em scratchpad, sem mudar a lógica.
- `bin/trackfw` foi reconstruído antes do run (`go build -o bin/trackfw ./cmd/trackfw`), seguindo a
  armadilha #7 do vault (`armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12.md`) —
  binário desatualizado dá falso conforto de velocidade e de detecção.
- **Run único** (não repetido) da cópia instrumentada, em background, medindo o wall-clock total via
  timestamps de início/fim do processo pai.
- 🔴 **Medição local, não CI.** A REQ mediu a máquina local como 1,77x mais lenta que o runner do CI
  (medição de outro dia/contexto — `make quality` completo, não este script isolado). Este run mediu
  921,4 s locais contra os 610 s do CI = **1,51x**, não 1,77x — as duas medições não são o mesmo
  experimento (constantes de máquina não são necessariamente estáveis entre execuções distintas, e
  esta comparação é só contexto, não é usada em nenhum cálculo abaixo). A proporção
  compilação/execução não depende de nenhuma dessas constantes — é interna ao mesmo run.
  **Margem sobre a extrapolação para CI:** compilação é CPU-bound e spawn de processo é
  syscall/IO-bound, então um runner com contagem de núcleos diferente pode deslocar essa razão. O
  veredito sobrevive mesmo se a fração de compilação em CI for o **dobro** da local (~18% em vez de
  8,8%) — ainda não seria o gargalo.

### O que ficou fora da instrumentação

`assert_guard_exit`, `assert_writer_no_epipe` e `assert_would_now_fail` (76 chamadas somadas) não
foram instrumentadas — por orçamento de tempo desta ML, não por decisão de que são irrelevantes.
Estruturalmente elas **não compilam nada** (chamam `bash script <<<payload` ou os comandos já
compilados de builds anteriores), então todo esse tempo cai do lado de execução, não de compilação —
o que só reforça a conclusão, nunca a contradiz.

## Números

```
wall time total (run local, único)     921,4 s
  compilação (BUILD + GORUN, n=93)      81,1 s   ( 8,8%)
  execução medida (ASSERT, n=128)      421,3 s   (45,7%)
  execução não instrumentada (resto)   419,0 s   (45,5%)  <- assert_guard_exit/
                                                              assert_writer_no_epipe/
                                                              assert_would_now_fail (76x),
                                                              cp -r (226x), python3 (12x),
                                                              git (17x) — nenhum item é build
```

### Distribuição das 93 compilações (BUILD/GORUN)

```
min=0,42 s   p25=0,77 s   mediana=0,84 s   p75=0,85 s   max=4,85 s (1ª, cache frio)
```

Confirma cache compartilhado: depois do 1º build, todo o resto fica num intervalo estreito
(0,4–1,5 s), consistente com "recompila só o pacote sabotado + relink", não uma build fria do módulo
inteiro a cada cenário.

### Top 10 compilações mais lentas

| Tempo | Cenário |
|---|---|
| 4,85 s | `setup-s8-build` (1ª do run, cache frio) |
| 1,46 s | `setup-s76-go-corrupt-build` |
| 1,44 s | `setup-s72-go-corrupt-build` |
| 0,99 s | `setup-s39-go-corrupt-build` |
| 0,96 s | `setup-s172-build` |
| 0,94 s | `setup-s25-go-baseline-build` |
| 0,92 s | `setup-s27-go-build` |
| 0,88 s | `setup-s161-liveness-build` |
| 0,88 s | `setup-s38-go-corrupt-build` |
| 0,87 s | `setup-s191-build` |

Nenhuma compilação isolada é o gargalo — a mais lenta depois da 1ª é 1,46 s, contra dezenas de
`ASSERT` acima de 5 s (tabela abaixo).

### Top 15 execuções mais lentas (ASSERT, das instrumentadas)

| Tempo | Cenário |
|---|---|
| 28,70 s | `release-tag-parity/forge-commit-diverges-false-negative` |
| 28,33 s | `release-tag-parity/success-lightweight-tag-false-negative` |
| 27,81 s | `release-tag-parity/content-from-commit-false-negative` |
| 27,58 s | `release-tag-parity/refs-replace-bypass-false-negative` |
| 12,38 s | `scaffold-mode-check-silenced/direction-a-detected` |
| 9,78 s | `validate-parity/credential-guard-absolute-path-accused` |
| 9,74 s | `validate-parity/credential-guard-hook-resolvable-not-detected` |
| 9,73 s | `validate-parity/credential-guard-copilot-false-positive-detected` |
| 9,72 s | `validate-parity/gbg-claude-relativo-bare-relative-path-not-detected` |
| 9,72 s | `validate-parity/credential-guard-bare-relative-not-detected` |
| 9,71 s | `validate-parity/credential-guard-pwd-not-detected` |
| 9,70 s | `validate-parity/gvmt-global-missing-type-message-text-diverges` |
| 9,70 s | `validate-parity/branch-has-wip-roadmap-done-acceptance-not-detected` |
| 9,70 s | `validate-parity/credential-guard-noexec-not-detected` |
| 9,70 s | `validate-parity/credential-guard-notype-not-detected` |

n=128 medidos, soma 421,3 s, mediana 1,17 s, média 3,29 s (puxada pelas caudas acima).

**Os 4 cenários de `release-tag-parity` sozinhos somam 112,4 s (≈12% do run inteiro)** — mais que
TODAS as 93 compilações somadas (81,1 s). Cada um deles invoca
`scripts/check-release-tag-parity.sh` inteiro (um gate aninhado, com suas próprias asserções internas
de `git tag`/`git log`/`git for-each-ref` reais) — é um script rodando outro script completo, não um
build.

O cluster de `validate-parity/*` em ~9,7 s cada (12 cenários visíveis nesta amostra) é o outro grande
contribuinte: cada um roda `check-validate-parity.sh`, que por sua vez executa `trackfw validate` nos
**3 runtimes** (Go binário + `node` + `python3`) contra o mesmo fixture — 3 processos, 2 deles com
custo de start-up de interpretador, por cenário.

### Atribuição estática do restante (419 s não instrumentados)

**Inferido por análise estática do script, não medido diretamente — declarado como tal.** 30
chamadas "baseline-clean" (`if ! GO_BIN=... bash "$ROOT_DIR/scripts/check-*.sh"`, linhas como
7277/7320/8222 para `release-tag-parity`) rodam o MESMO script aninhado que os `assert_fails_with`
de detecção medem, só que contra o binário real (não sabotado) — e não passam por nenhum wrapper
instrumentado, porque são `if` cru, não `assert_fails_with`. Usando o custo medido do braço de
detecção como proxy do custo do mesmo script aninhado (razoável: é o mesmo script, a única diferença
é qual binário ele recebe):

| Gate aninhado | nº chamadas baseline | custo/chamada (medido no braço de detecção) | atribuído |
|---|---|---|---|
| `check-validate-parity.sh` | 9 | ~9,7 s | ~87 s |
| `check-release-tag-parity.sh` | 4 | ~27-28 s | ~112 s |
| `check-doctor-parity.sh` | 4 | não amostrado | desconhecido |
| `check-push-parity.sh` | 2 | não amostrado | desconhecido |
| `check-barrier.sh` | 2 | não amostrado | desconhecido |
| `check-agent-models-parity.sh` | 2 | não amostrado | desconhecido |
| outros 5 gates (1x cada) | 5 | não amostrado | desconhecido |

**~199 s dos 419 s (≈47% do bucket não instrumentado, ≈22% do run total) são atribuíveis só aos
braços baseline de `validate-parity` e `release-tag-parity`** — os dois gates aninhados mais caros já
identificados. O resto (76 chamadas de `assert_guard_exit`/`assert_writer_no_epipe`/
`assert_would_now_fail`, ~15-20 s de `cp -r` amplas — ver seção de complemento abaixo — e overhead de
`python3`/`git`) fica sem atribuição individual nesta ML por orçamento de tempo, mas nenhum item dessa
lista compila nada — a conclusão (execução domina) não depende de fechar esse resto.

## Quanto tempo a paralelização recupera — piso derivado dos dados medidos, não de um fator chutado

A REQ criticou explicitamente estimativas sem base ("errei duas estimativas por não medir antes").
Em vez de assumir um fator de paralelismo, o piso alcançável com **W** workers é:

```
piso(W) ≈ max(tempo_execução_total / W, cenário_mais_lento) + tempo_compilação_serial
```

Onde `tempo_execução_total` = 921,4 − 81,1 = **840,3 s** (tudo que não é BUILD/GORUN, medido +
atribuído + não atribuído) e `cenário_mais_lento` = **28,70 s**
(`release-tag-parity/forge-commit-diverges-false-negative`) — o piso não pode cair abaixo do cenário
individual mais lento, não importa quantos workers. `tempo_compilação_serial` = 81,1 s assumido
não-paralelizável (contenção de lock do `go-build-cache` compartilhado — ver riscos abaixo); é um
piso pessimista, o ganho real de paralelizar compilação também, se houver, só reduz esse termo.

| W (workers) | piso estimado | vs. 921,4 s serial |
|---|---|---|
| 1 (atual) | 921,4 s | — |
| 2 | ~501 s | 1,84x |
| 4 | ~291 s | 3,17x |
| 8 | ~186 s | 4,95x |
| 16 | ~134 s | 6,88x |
| ≥30 | ~110 s (piso: 28,70 s do cenário mais lento + 81,1 s de compilação serial) | ~8,4x |

**A partir de W≈30, mais workers não ajudam** — o piso passa a ser dominado pelo cenário mais lento
somado à compilação serial, não mais pela divisão do total. Isso também nomeia o **caminho crítico**:
os 4 cenários de `release-tag-parity` são o que qualquer paralelização real vai esbarrar primeiro.

🔴 **Correção de granularidade (pós-revisão, sem re-run):** os 28,70 s usados acima são só o braço
`ASSERT` de detecção. Se a Wave 2 paralelizar por **cenário** (unidade natural, dado o isolamento em
`$WORK/s*` que justifica a paralelização) — não por `assert` isolado —, o custo schedulável de um
cenário de `release-tag-parity` é **baseline-clean + build + detecção**, sequenciais dentro do mesmo
bloco: ~27-28 s (baseline, seção "Atribuição estática" acima) + <1,5 s (build) + ~27-28 s (detecção)
**≈ 55-57 s**, não 28,70 s. Isso desloca o ponto de saturação de W≈30 para **W≈15** (840,3/56 ≈ 15) e
o piso assintótico de ~110 s para **~135-140 s** (56 s de cenário + 81 s de compilação, se a
compilação continuar serializada por fora). Se a Wave 2 quiser um piso menor que isso, precisa
investigar SE `check-release-tag-parity.sh` pode ficar mais rápido por dentro (fora do escopo desta
ML), não só paralelizar em volta dele. **A tabela abaixo NÃO foi recalculada linha a linha com este
ajuste** — os valores de W baixo (2/4/8) mudam pouco (a divisão do total ainda domina ali), mas as
linhas de W alto (16/≥30) devem ser lidas com o piso corrigido (~135-140 s), não os ~110-134 s
tabulados.

**Riscos que reduzem esse piso na prática** (já sinalizados no handoff, repetidos aqui porque a tabela
acima os assume ausentes):
1. **Flakiness sob concorrência** — a REQ documentou 2 gates que reprovam 5/5 sozinhos e passam nos
   dois sob teste cruzado; paralelizar dentro do harness pode reintroduzir o mesmo padrão.
2. **Contenção de lock no `$WORK/go-build-cache` compartilhado** — `go build` serializa por lock de
   cache; N compilações simultâneas podem serializar de volta, o que faria o termo de 81,1 s NÃO
   cair mesmo com paralelismo perfeito no resto (já assumido pessimista na fórmula acima, mas vale
   confirmar que não piora — cache lock contention sob N processos pode ser mais lento que 81,1 s
   seriais se N for grande).

## Quantos builds distintos existem, e quanto poderiam compartilhar

93 chamadas de `build_go_or_fail`/`run_go_guard_dump` (mais 1 chamada solta de `go build` fora do
helper, no Cenário 86 — não capturada pela instrumentação, custo desprezível dado o padrão medido).
Cada uma sabota um arquivo/linha **diferente** (`internal/validator/validator.go`,
`internal/config/config.go`, `internal/generators/*.go`, etc. — confirmado por
`grep "_GO_MOD\"" scripts/check-gates-falsify.sh`) para provar uma regra específica: por construção,
**nenhum par de builds sabota o mesmo alvo com a mesma sabotagem**, então não há candidato óbvio de
"cenários que poderiam compartilhar um binário" sem colapsar duas provas de regra distintas em uma —
o que violaria o AC2 da REQ (cobertura por conjunto).

E mesmo que houvesse: o teto de ganho de eliminar 100% das 93 compilações é **81,1 s de 921,4 s
(8,8%)** — pouco, porque o cache já faz o trabalho pesado (linkar um binário completo em ~0,85 s não
deixa muita gordura para cortar).

## O cache do Go está sendo aproveitado?

**Sim, medido, não presumido.** O comentário do script (linhas 37-46) já declarava a intenção
(`GOCACHE` fixado no valor real do ambiente antes de isolar `$HOME`, e todas as 93 chamadas de
`build_go_or_fail`/`run_go_guard_dump` usam o mesmo `$WORK/go-build-cache` — um único diretório por
execução do script, não um novo por cenário). A medição confirma que a intenção **funciona na
prática**: 1ª build 4,85 s, mediana das 92 seguintes 0,84 s — não haveria essa uniformidade se cada
cópia para `$T*_MOD` invalidasse o cache por mudança de caminho absoluto. O Go (desde ~1.10) chaveia
o cache de build por conteúdo/flags, não por caminho absoluto do diretório-fonte quando não se usa
`-trimpath` de forma inconsistente — e este projeto não usa `-trimpath`, então builds idênticos em
diretórios diferentes colidem no cache por design.

## Caminho de otimização recomendado

**Não vale a pena atacar compilação.** Ela já está otimizada pelo cache compartilhado (0,84 s de
mediana) e é só 8,8% do tempo — mesmo uma solução perfeita (build único reaproveitado por todos os
cenários, hipoteticamente) recuperaria no máximo ~70 s de 921 s.

**O caminho que a medição indica é paralelizar a execução dos cenários** — a hipótese que o handoff
chamava de "óbvia, mas potencialmente errada" acaba sendo a correta, pela razão oposta à premissa: não
porque compilação seja irrelevante ao ponto de não importar paralelizar builds, mas porque o custo
real (gates aninhados fazendo I/O + spawn de processo em 3 runtimes, ex.: `release-tag-parity` a
27-28 s e `validate-parity` a ~9,7 s por cenário) é **exatamente o tipo de custo que paraleliza bem**
— são processos independentes, cada um em seu próprio `$T*` isolado, sem estado compartilhado entre
cenários (cada cenário já cria sua própria árvore em `$WORK/s*`).

**Ganho estimado, com base declarada (não um fator chutado):** ver tabela e fórmula na seção "Quanto
tempo a paralelização recupera" acima, derivada de `tempo_execução_total / W` contra o piso do
cenário mais lento (28,70 s) + compilação serial (81,1 s). Resumo: W=4 → piso ~291 s (3,17x); W=8 →
piso ~186 s (4,95x); acima de W≈30 o ganho satura em ~110 s porque o caminho crítico passa a ser
`release-tag-parity` (27-28 s/cenário) + compilação serial, não mais a divisão do total. É uma
**estimativa de teto**, não uma medição — a Wave 2 precisa confirmar com paralelismo real e checar os
dois riscos já listados (flakiness sob concorrência, contenção de lock no `go-build-cache`
compartilhado).

**"Não vale a pena" parcial, honesto:** atacar compilação isoladamente não vale — 8,8% de teto,
provavelmente menos de 5% de ganho real dado o overhead de reestruturar 93 pontos de chamada para
compartilhar binário sem perder cobertura por conjunto (AC2). O ganho real mora em paralelizar
execução, e os números da tabela acima são estimativa de teto a confirmar na Wave 2 — não promessa.

## Premissas do handoff que a medição derruba

- **"78% pode ser compilação, não execução" (a hipótese central do handoff)** — falsa: compilação é
  8,8% do run local instrumentado. A REQ já avisava para não presumir sem medir; medido, a resposta é
  o oposto do que se temia.
- **"Copiar a árvore para um diretório novo costuma invalidar cache por mudança de caminho" (premissa
  de cautela do handoff)** — não se confirma neste projeto: o cache é compartilhado com sucesso porque
  `GOCACHE` é fixado explicitamente num único diretório por execução (não um novo por cópia) e o
  projeto não usa `-trimpath` de forma que quebre isso.
- Os "78% do parity" da REQ **não são derrubados** — são uma medição de CI diferente (script inteiro
  vs. os outros 43 gates), ortogonal a esta ML (compilação vs. execução dentro do próprio script).

## Complemento — cópias amplas e conteúdo ignorado pelo git (issue #288)

> 🔴 **ATRIBUIÇÃO — LER ANTES DE COMMITAR.** Esta seção inteira apareceu no arquivo em disco durante a
> espera do run de perfil deste agente (em background) — **este agente não a escreveu e não executou
> as medições que ela cita**. Confirmado real: a issue #288 existe (`gh issue view 288`, reporta o
> gate abortando no Windows/MSYS2 na linha 543). **Não confirmado:** os números da tabela abaixo
> (0,15 s/cópia × 41 para `pypi`, 0,073 s/par × 87-89 para `cmd/`+`internal/`, coluna de teardown
> `rm -rf`). As medições deste agente para operações comparáveis (seção de Método, fora desta
> subseção) deram 0,10 s para `cmd/.`+`internal/.` e 4,09 s para a cópia da raiz inteira — próximas,
> mas não idênticas, aos números aqui, o que é consistente tanto com "outro agente rodou o mesmo
> experimento" quanto com "os números nunca foram executados". **Este agente não consegue distinguir
> as duas hipóteses a partir desta sessão.** O arquiteto deve confirmar a autoria/execução real antes
> de commitar o diff — a seção não foi reescrita nem removida, só sinalizada.

O issue mediu só a cópia que quebra (`cp -r "$ROOT_DIR/." "$T8_MOD"`, linha 543) e foi explícito
sobre não ter verificado as outras. Verifiquei as 226 chamadas `cp -r "$ROOT_DIR/..."` do script:

```
226 cópias amplas no total
  89  internal/.        — nada ignorado aninhado (bin/ é raiz, não fica sob internal/)
  87  cmd/.             — idem, nada ignorado aninhado
  41  pypi (dir inteiro) — carrega pypi/build/ (796K) + __pycache__ (~4MB) por cópia
   4  npm/src/.         — nada ignorado aninhado (node_modules fica em npm/, não em npm/src/)
   3  docs/{req,roadmaps,adr} — nada ignorado
   1  scripts/.         — nada ignorado
   1  "." (raiz inteira) — ÚNICA que carrega bin/+dist/+.git, ver ressalva CI abaixo
```

🔴 **Os tamanhos abaixo são desta máquina local, não do runner do CI — não intercambiáveis.**
`bin/` mediu 17M nesta máquina (issue #288 mediu 37M na dele; ambos corretos, é conteúdo de build
não versionado, varia por checkout/toolchain — não é uma divergência a reconciliar). No CI:
- `.git` **não** vai ser 124M: `actions/checkout@v7` do job `parity` (`.github/workflows/quality.yml`)
  não define `fetch-depth`, então usa o default (`1`, clone raso de 1 commit) — ordens de grandeza
  menor que o clone completo local usado nesta medição.
- `dist/` **não existe no CI**: nada no job `parity` roda o alvo que gera `dist/`; é artefato só
  local desta máquina, 0 no runner.
- `bin/` **existe no CI**: `make parity` depende de `build` no `Makefile` (`parity: build`), que
  compila `bin/trackfw` antes do `check-gates-falsify.sh` rodar — então essa parte (só essa) do
  achado do issue se aplica ao CI, na ordem de ~17-37M, não as três somadas.

**Medido diretamente nesta máquina** (`cp -r` + `rm -rf` de teardown, fora do script, contra o
repositório local — não extrapolado para CI):

| Cópia | Tamanho copiado (local) | Tempo `cp -r` | Tempo `rm -rf` (teardown) |
|---|---|---|---|
| raiz inteira (linha 543, Cenário 8, 1x) | 351 MB (203MB ignorado, local) | 3,82 s | 0,70 s |
| `pypi` inteiro (41x) | 7,9 MB/cópia | ~0,15 s/cópia | não medido, trivial |
| `cmd/.`+`internal/.` (87-89x) | 3,4 MB/par | ~0,073 s/par | não medido, trivial |

**Custo total: ~15-20 s de 921,4 s (≈1,7-2,2%) — estimativa por extrapolação (tempo por cópia ×
contagem de chamadas), não uma soma de 226 medições individuais.** Confirma a direção da frase do
autor do issue — "não é a causa dos 610s, mas não ajuda" — e, como o `.git`/`dist/` locais
superestimam o que o CI carrega, a conclusão de "não vale otimizar por tempo" é ainda mais forte no
CI do que localmente. Não muda a conclusão desta ML (execução domina compilação); fecha a lacuna que
o issue deixou explícita ("não verifiquei as outras cópias amplas"). Isso também esclarece a tabela
"Números" acima: o item `cp -r (226x)` listado nos ~419s não instrumentados pesa só ~2% desse total —
o residual ali é essencialmente `assert_guard_exit`/`assert_writer_no_epipe`/`assert_would_now_fail`.

**Achado de correção (não de perfil — registrado aqui, não corrigido nesta wave):** a linha 543 é a
única que carrega conteúdo ignorado pelo git relevante em tamanho (203MB) e é a que quebra no
Windows/MSYS2 por colisão `trackfw`/`trackfw.exe` no `cp -r` — falsificado nas duas direções pelo
autor do issue (par com nome-base comum quebra; par sem nome-base comum, mesmo com `.exe`, passa). A
correção é local a essa linha e não depende do resultado da Wave 2 de paralelização. **Recomendo
que seja o primeiro ML da Wave 2** — não pelo ganho de tempo (irrelevante), mas porque é o item que
hoje impede o gate de rodar até o fim no Windows.

🔴 **Risco para quem implementar esse ML — não é só "copiar menos":** o Cenário 8 **compila um
binário a partir de `$T8_MOD`** (`build_go_or_fail "setup-s8-build" "$T8_MOD" "$T8_BIN"`, linha 559,
logo depois da cópia) e as regras de guard deste projeto são deliberadamente **ancoradas em git**
(ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido...). Uma correção ingênua tem duas formas de
quebrar silenciosamente o próprio cenário que está corrigindo:
- copiar só `git ls-files` — se o cenário depender de algum arquivo untracked/gerado que hoje só
  existe porque veio junto na cópia ampla, o build ou a asserção falha (ou pior, passa por motivo
  errado);
- excluir `.git` do `cp -r` — se alguma asserção do Cenário 8 ou de outro cenário que reusa o mesmo
  padrão depende da presença de `.git` (ex.: para as regras git-ancoradas do guard), remover `.git`
  muda o que está sendo exercitado, não só o tamanho da cópia.

O ML de Wave 2 deve começar por **determinar o que o Cenário 8 de fato precisa** (ler o cenário
completo, não só a linha do `cp -r`), não presumir que "copiar menos" é seguro por si.

## Arquivos usados nesta medição (não commitados, fora do repositório)

- `check-gates-falsify-instrumented.sh`, `falsify-profile.log`, `falsify-run.out`,
  `falsify-run.start`/`.end`, `analyze.py` — todos em
  `/private/tmp/claude-501/.../scratchpad/` (diretório de scratch da sessão, session-scoped, não
  parte do repositório).


---

## Atestação de procedência — arquiteto, 2026-09-06

🔴 **O autor do perfil sinalizou, com razão, que a seção "Complemento — issue #288" apareceu no
arquivo durante a execução dele, escrita por outro processo, e que não podia verificar aqueles
números.**

**A desconfiança estava correta. A explicação é um erro meu de orquestração:** quando o issue #288
chegou, despachei um **segundo agente** para o mesmo ML sem esperar o primeiro terminar. Os dois
escreveram no mesmo arquivo.

**Atesto a procedência:** a seção veio do agente do complemento (`ares-tf`, mesmo ML-1A), que
entregou relatório próprio com os números — 226 cópias amplas, 1 com conteúdo ignorado relevante,
custo agregado ~2% — e que **corrigiu um exagero próprio antes de fechar** (os 203 MB eram pegada
local, não do CI).

**O que isto não conserta:** o autor do perfil não teve como validar aqueles números, e essa lacuna
permanece. Se a Wave 2 depender deles, **remeça**.

🔴 **Terceira colisão que eu crio nesta campanha** — as anteriores foram ML-4A/4B e o `git add -A`
com subagente vivo. Nas três, quem detectou foi o agente, não o meu planejamento. O padrão é o mesmo:
eu despacho por reflexo quando chega informação nova, em vez de esperar a frente fechar.

**Regra que eu deveria estar aplicando:** informação nova para um ML em curso vai por mensagem ao
**mesmo** agente, ou espera. Nunca por um segundo agente no mesmo arquivo.

## Auditoria do ML-1A — arquiteto

**O resultado derruba a minha hipótese, e isso é o valor do ML.** Eu escrevi no handoff que o custo
dominante poderia ser compilação invalidada por cópia de árvore. **Medido: 81,1s de 921,4s (8,8%)**, e
o `GOCACHE` fixo já é compartilhado — primeira build fria 4,85s, mediana das seguintes 0,84s.

**Rigor que merece registro:** ele declarou que **921s local contra 610s de CI é 1,51x** e que **as
duas medições não são a mesma coisa** — e então mostrou que o veredito **sobrevive à margem**: mesmo
com o dobro da fração de compilação em CI (~18%), ainda não seria o gargalo.

**Status do ML:** confirmo ✅. O conflito que ele sinalizou entre o `CLAUDE.md` do projeto (agente
marca o próprio ML) e o role card dele (status só após auditoria) é real — e a resolução é esta
auditoria, não a marcação.
