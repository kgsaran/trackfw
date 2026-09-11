---
status: wip
date: 2026-09-06
squad: ares-tf
req: "docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md"
---

# Roadmap: Ratchet por nome, e classe própria para suíte que não carrega

> Criado em: 2026-09-06 | Status: wip

## Context

REQ: `docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md`
ADR: `docs/adr/ADR-2026-09-05-o-ci-de-windows-bloqueia-por-conjunto-de-nomes-e-por-tipo-de-evento-nunca-por-contagem.md` (`Accepted`)

> 🔴 **Correção de auditoria (arquiteto, 2026-09-10):** este roadmap afirmava `Accepted` desde
> 2026-09-06, mas a ADR estava **`Proposed`** — e a REQ estava **órfã** (`roadmap: ""`), apesar de o
> roadmap apontar para ela. Vínculo de mão única. As duas coisas foram corrigidas agora, **antes** do
> despacho: ADR lida e **aceita**, REQ vinculada. Trabalho sobre ADR não aceita é decisão
> arquitetural tomada por omissão.
Fecha: **#275** e **#274**

## Diagnóstico

`windows-full-suites` roda com `continue-on-error: true` — **nenhuma regressão de Windows reprova um
PR**. E a contagem esconde regressão: medido 3x pelo consumidor externo, e **uma 4ª vez conosco**, no
PR #285, que baixou o total e introduziu 6 falhas.

## ✅ PROVA FINAL — 2026-09-10, run 34522646868 (PR #312)

```
checks vermelhos: 0
ML-2A/2B: 38 observed / 38 active / 0 removed
          Go 14/14 · Node-assert 10/10 · Node-load 1/1 · Python 13/13
```

🔴 **O job está VERDE com os 38 vermelhos ainda na lista.** É a promessa da catraca cumprida e
medida: bloqueia **regressão** sem exigir que a dívida chegue a zero primeiro.

**O veredito mudou de dono.** As três suítes continuam reprovando — são os 38 — e são absorvidas no
nível do step. Quem decide o job é o **ratchet**.

### O que o PR aberto pagou

Três defeitos que **os 23 testes locais não pegavam**, todos achados por execução real no runner:

| defeito | achado por |
|---|---|
| `origin/main` não existe como ref (checkout raso) ⇒ a proteção do D4 **nunca rodaria** | run 34511651888 |
| a mensagem publicava *"arquivo não está na main"* para **ausência de ref** | idem |
| classe própria reprovava **sem consultar a lista** ⇒ job nunca ficaria verde | run 34519502920 |

Sem o PR de longa duração como instrumento, o `ML-3A` teria sido mergeado com a proteção do D4
desligada **e** com o job permanentemente vermelho.

### A lição de método que fica

O terceiro defeito passou porque **faltava o braço de "passa"** para a classe própria. Havia
contra-braço para asserção (`T17`), não para load-failure. 🔴 **Guarda sem braço positivo é guarda
que não se sabe se discrimina** — ela pode estar reprovando tudo e parecendo funcionar.

O `T19` foi acrescentado e nomeado no relatório como *"o braço cuja ausência causou o defeito"*.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — O discriminante, antes do ratchet
> Sequencial. A D3 da ADR exige medir antes de escrever.

### ML-1A — Distinguir "suíte não carregou" de "teste reprovou"
**Status:** ✅ Concluído · **Agente:** `ares-tf`
🔴 **Medir o discriminante nos DOIS cenários antes de escrevê-lo.** Foi pular esse passo que produziu
a nossa afirmação pública errada no `#274` — `pass 0 / fail 1` é idêntico nos dois casos.
Cobrir também `tests == 0`. **Falha de classe própria**, não linha na lista de nomes.

**Arquivos afetados:** `.github/workflows/quality.yml`

**Parity note:** a implementação é em CI script/YAML (infra), não em comando CLI.
O contrato de paridade dos 3 CLIs cobre COMANDOS públicos do trackfw, não scripts de CI.
Referência: `docs/cli-parity.md` (regra cobre `public commands`).

**Medições realizadas antes da implementação (2026-09-10):**

| Runtime | Cenário | Exit code | Sinal de evento | Observável |
|---|---|---|---|---|
| Go 1.25.2/macOS | (a) load failure — sintaxe | 1 | `[setup failed]` | Linha `FAIL\t<pkg> [setup failed]` |
| Go 1.25.2/macOS | (b) load + assert mistos | 1 | `[setup failed]` + `--- FAIL:` | Ambos na mesma saída |
| Go 1.25.2/macOS | (c) assertion failure | 1 | `--- FAIL: TestName` | Sem `[setup failed]` |
| Go 1.25.2/macOS | (d) zero tests | **0** | `[no test files]` | 🔴 exit 0 — indistinguível de sucesso |
| Python 3.14.7/macOS | (a) conftest error | **4** | exit code | Protocolo pytest estável |
| Python 3.14.7/macOS | (b) import error collection | **2** | exit code | Collection interrupted |
| Python 3.14.7/macOS | (c) assertion failure | 1 | exit code | Normal |
| Python 3.14.7/macOS | (d) zero tests | **5** | exit code | `no tests ran` |
| Node 26.8.1/macOS + TAP | (a) require quebrado | 1 | `exitCode:` presente no TAP | D3-bis confirmado |
| Node 26.8.1/macOS + TAP | (c) assertion failure | 1 | `exitCode:` **ausente** | `code: 'ERR_ASSERTION'` apenas |
| Node 26.8.1/macOS + TAP | (d) zero tests | **0** | `# tests 0`, `1..0` | 🔴 exit 0 — pior modo |

**Lacunas declaradas:**
- Go (d): `[no test files]` → exit 0. O discriminante `[setup failed]` não cobre isso; zero-test-Go é visível no log mas não bloqueia (exit 0).
- Node 20/windows-latest: não medido diretamente. Formato TAP estável entre Node 22 e 26; extrapolado.
- Node (d): `# tests 0` + exit 0 é o pior modo — o job parece ter passado. Emitido como `::error::` mesmo com exit 0.

**Discriminante implementado (tipo de evento, nunca contagem — ADR D3/D3-bis):**
- Go: `Select-String 'FAIL\t.*\[setup failed\]'` na saída capturada
- Python: exit code 2/3/4 → `suite-load-failure`; exit code 5 → `zero-test-failure`
- Node: `exitCode:` presente em bloco `not ok` do TAP → `suite-load-failure` (D3-bis)
- Node zero-test: exit 0 + `# tests 0` no TAP → `zero-test-failure`

**Falsificação (nas duas direções, 3 runtimes):**
Step `ML-1A — falsificação do discriminante` adicionado ANTES dos 3 runs de suíte.
Braço 1 (load failure → detector dispara) + Braço 2 (asserção → detector NÃO dispara).
Guarda de vacuidade por braço: verifica que a mutação foi aplicada antes de afirmar.

**Reconciliação teste↔conclusão (regra dura do projeto):**
O step de falsificação não contém testes de produto; contém PROBES que afirmam as conclusões desta seção de medição:
- `Assert-True ($goLoadFails.Count -gt 0) "..."` afirma: "Go: `[setup failed]` aparece na saída de load failure"
- `Assert-True ($goAssertFails.Count -eq 0) "..."` afirma: "Go: `[setup failed]` NÃO aparece na saída de assertion"
- `Assert-True ($rc_py_load -eq 2 -or ...-eq 4) "..."` afirma: "Python: exit code de load failure é 2 ou 4"
- `Assert-True ($rc_py_assert -eq 1) "..."` afirma: "Python: exit code de assertion é 1"
- `Assert-True ($tapLoadContent -match 'exitCode:') "..."` afirma: "Node: `exitCode:` aparece no TAP de load failure (D3-bis)"
- `Assert-True ($nodeAssertFails.Count -eq 0) "..."` afirma: "Node: `exitCode:` NÃO aparece no TAP de assertion"

## Wave 2 — O ratchet
> Dependências: Wave 1. Sem o discriminante, um estado sem nomes escapa do ratchet por construção.

### ML-2A — Lista versionada de vermelhos, por nome
**Status:** ✅ Concluído · **Agente:** `ares-tf`
🔴 **A lista nasce de um run do CI**, nunca de máquina — o autor do `#275` declara que o Windows dele
não é o runner. Reprova nome fora da lista; **avisa** quando um nome da lista deixa de falhar.
🔴 **Guarda de não-vacuidade:** com a lista vazia e a dívida atual, o job **tem** de reprovar.

**Medições (run 34478752778 · main · 2026-09-10T12:47 · job 102875922566):**

Comando: `gh run view 34478752778 --log --job=102875922566`

| Runtime | Classe | Contagem (informativa) |
|---|---|---|
| Go | assertion | 14 top-level (18 linhas `--- FAIL:` com subtests) |
| Node.js | assertion | 10 |
| Node.js | suite-load-failure | 1 (`validator.test.js`, `exitCode: 1` no TAP — D3-bis) |
| Python | assertion | 13 |

Reconciliação com sumários dos runners:
- Go: `6 pacotes FAIL`, `14 Z --- FAIL: Test` top-level — ✓ consistente
- Node: `# fail 11` no TAP (10 assertion + 1 suite-load) — ✓ consistente
- Python: `13 failed, 1647 passed` no sumário pytest — ✓ consistente

**D5 — decisões de estabilidade de nome:**
- Go subtests: nome top-level apenas (strip depois de `/`). Subtests são entradas de tabela e mudam.
- Python class methods: ID pytest completo com classe (`TestClass::method`). Nomes de função sozinhos não são únicos entre classes.
- Node suite-load-failure: basename do arquivo apenas. Path completo do runner (`D:\a\trackfw\...`) é instável.
- Node assertions: nome completo da string TAP (após `not ok N - `). Strings estáveis.

**Artefatos entregues:**
- `.github/windows-known-failures.json` — lista de 38 entradas com `_meta` (D2, D4, D5)
- `scripts/check-windows-known-failures.py` — verificador com `--self-test` (9 testes, 9 PASS)
- `Makefile` — `python3 scripts/check-windows-known-failures.py --self-test` adicionado a `parity-rest`
- `.github/workflows/quality.yml` — step Python captura saída para arquivo; step ML-2A adicionado

**Falsificação (nas duas direções + guardas de vacuidade):**
- T1: todos observados == lista → exit 0
- T2: nome novo não na lista → exit 1 (ADR D1)
- T3: entrada da lista não observada → aviso, exit 0 (corrigir não bloqueia CI)
- T4: lista vazia → guarda de vacuidade → SystemExit(1)
- T5: arquivo ausente → guarda de vacuidade → SystemExit(1)
- T6: artefato ausente (go-out) → guarda lado-observação → SystemExit(1)
- T7: path Windows no TAP → basename correto (`validator.test.js`)
- T8: backslash Python + método de classe → normalizado corretamente
- T9: nome não-ASCII (em-dash, acentos) → round-trip sem perda de encoding

**Gates executados (sequenciais, locais, macOS arm64):**
- `make build` → exit 0
- `make test` → exit 0 (cached)
- `make parity-rest` → exit 0 (9 PASS, 0 FAIL no self-test)
- `make quality` → exit 0
- `trackfw validate` → exit 0 (174 warnings pré-existentes, 0 errors)
- YAML: `python3 -c "yaml.safe_load(...)"` → válido, 11 jobs

### ML-2B — Remoção de nome exige justificativa
**Status:** ✅ Concluído · **Agente:** `ares-tf`
Corrigido, **renomeado** ou **deixou de executar** — o ratchet não distingue sozinho. Sem isto a
lista vira cemitério, que é a única forma de ele fracassar em silêncio.
**Falsificação obrigatória:** renomear um teste da lista **sem corrigi-lo** não pode virar verde.

**Medições do discriminante (2026-09-10, macOS arm64):**

| Runtime | Observável de pass | Discriminante corrigido-vs-não-executa? |
|---|---|---|
| Go 1.25.2 (`-v`) | `--- PASS: TestFoo (0.01s)` | ✅ SIM — presente=corrected, ausente=deleted |
| Node (TAP) | `ok N - test name` em coluna 0 | ✅ SIM — presente=corrected, ausente=deleted |
| Python (`-q -rA`) | `PASSED pypi/tests/...` em short summary | ✅ SIM com `-rA` (1 flag extra) |

Python sem `-rA`: sem discriminante (verificado medindo `pytest -q` vs `pytest -q -rA`). Mudança de 1 flag no step Windows supre. Se `-rA` ausente (vacuidade), checker emite aviso e pula verificação.

**Design do artefato:** seção `removed` separada de `entries` no JSON.
- Entradas retiradas: movidas para `removed` com `removal_note` obrigatório
- `--baseline`: diff contra `origin/main` detecta deleções silenciosas (bypassing `removed`)

**4 braços de falsificação implementados:**
- T10: entrada deletada sem `removed` record → **exit 1** (baseline check)
- T11: entrada em `removed` sem `removal_note` → **exit 1** (schema check)
- T12: `removal_note: corrected` mas teste não está em PASS output (Go não-vacuous) → **exit 1**
- T13: `removal_note: renamed` com `renamed_to` ausente das active entries → **exit 1**
- T14: remoção válida (corrected + teste em PASS) → **exit 0** (guarda de vacuidade)

**Arquivos afetados:**
- `.github/windows-known-failures.json` — seção `removed` adicionada; `_meta.d4_note` atualizado
- `scripts/check-windows-known-failures.py` — 3 extractors de pass, `validate_removed()`, `check_baseline_deletions()`, 5 novos self-tests (T10-T14), `--baseline` arg
- `.github/workflows/quality.yml` — `-rA` no Python step; novo step de baseline; ratchet step atualizado

**Gates (sequenciais, foreground, local, macOS arm64):**
- `python3 scripts/check-windows-known-failures.py --self-test` → 14 PASS, 0 FAIL
- `make parity-rest` → exit 0
- `trackfw validate` → exit 0 (174 warnings pré-existentes)
- YAML: `python3 -c "yaml.safe_load(...)"` → válido, 11 jobs
- `make quality` → em andamento (vide sessão 2026-09-10f)

---

### Corretivo ML-2B — baseline D4 nunca rodava em CI (2026-09-10, sessão g)

**Causa raiz medida:** `actions/checkout@v7` sem `with:` usa `fetch-depth: 1` e só busca o branch do PR. `origin/main` nunca é criado como ref remota. `git show origin/main:...` falha com *"unknown revision"* — não porque o arquivo está ausente na `main`, mas porque a **ref não existe**. Dois estados distintos (ref ausente / arquivo ausente) geravam o mesmo observável: silêncio + aviso genérico com mensagem errada *"normal em PRs que adicionam o arquivo pela primeira vez"*.

Terceira ocorrência do padrão dois-estados-um-observable registrado em `vault/notes/guarda-que-reporta-ausencia-precisa-distinguir-nao-achei-de-nao-consegui-procurar-2026-09-10.md` (entrada `ML-2B (CI)` na tabela já existente).

**O que foi corrigido (mesma REQ/roadmap — mesma causa):**
1. Step `ML-2B — extrair baseline` reescrito: fetch incondicional com refspec explícito (`+refs/heads/main:refs/remotes/origin/main`) — custo medido: 0.09 s / < 1 MB localmente.
2. Dois estados distinguíveis com mensagens próprias:
   - ref ausente (fetch falhou) → `::error::` + **exit 1** ("fatal" = step-level exit 1; não bloqueia PR enquanto `continue-on-error: true` do job windows-full-suites estiver ativo — ML-3A)
   - ref presente + arquivo ausente na main (este PR adiciona o arquivo) → `::warning::` + exit 0
   - ref presente + arquivo presente → baseline extraído, tamanho logado, D4 ativa
3. `check_baseline_deletions()` adicionou linha positiva de confirmação quando o baseline roda limpo (`ML-2B D4 (baseline): N entrada(s) comparadas — nenhuma deleção silenciosa`) — antes "clean" e "skipped" eram indistinguíveis no log.
4. T15 (a+b) nos self-tests: afirma que a linha aparece com baseline e está ausente sem ele.

**3 braços de falsificação (arm 3 ativo após merge para main):**
- Braço 1 (ref ausente → exit 1): refspec errado ou fetch falha → step falha com `::error::` e exitcode 1
- Braço 2 (ref ok + arquivo ausente → warning + exit 0): caso deste PR — arquivo adicionado aqui, ainda não mergeado
- Braço 3 (ref ok + arquivo presente → D4 ativa): após merge, próximo PR que modifica a lista recebe verificação real; evidência: checker emite `ML-2B D4 (baseline): N entrada(s) comparadas` (T15a verifica)

**Gates (corretivo, sequenciais, foreground, local, macOS arm64):**
- `python3 scripts/check-windows-known-failures.py --self-test` → **16 PASS, 0 FAIL**
- `make parity-rest` → exit 0
- `trackfw validate` → 0 errors (174 warnings pré-existentes)
- YAML: `python3 -c "yaml.safe_load(...)"` → válido

## Wave 3 — Tirar a rede
> Dependências: Waves 1 e 2 fechadas e verdes.

### ML-3A — Remover `continue-on-error` do `windows-full-suites`
**Status:** ✅ Concluído · **Agente:** `ares-tf`
🔴 **Só aqui.** Remover antes tornaria a `main` imergível com a dívida atual — o ponto do ratchet é
bloquear regressão **sem** exigir zero primeiro.

#### Corretivo ML-3A — Guarda de marcador consulta lista (CI run 34519502920)
**Status:** ✅ Concluído · **Agente:** `ares-tf`

**Defeito (auditoria do arquiteto):** a guarda de marcadores no `run_check` reprovava em
**qualquer** marcador de `suite-load-failure`, sem consultar a lista. `validator.test.js`
estava na lista como dívida conhecida (`runtime: node, class: suite-load-failure`), mas a
guarda retornava 1 antes que o ratchet de nomes pudesse absolver a entrada. Resultado: job
permanentemente não-verde enquanto houver qualquer entrada na lista dessa classe.

**Causa raiz (nota de método):** leitura do D3 como "falha de classe própria = sempre fatal"
quando D3 diz "falha de **classe** própria" — ou seja, precisa de **classificação** própria,
não que seja automaticamente fatal. O parágrafo seguinte do mesmo ADR distingue o caso perigoso
(sem nome, contagem some sem rastro) do caso catalogável (com nome, dívida conhecida).

**Correção aplicada (`scripts/check-windows-known-failures.py`):**
- `_parse_go_load_names(content)` — extrai pacotes de marcador Go (`FAIL\t<pkg> [setup failed]`).
- Passo 3 (early): apenas marcadores sem nome extraível (Python load, zero-test Node/Python) → `return 1`.
- Passo 3b (late, após extração): Go load (`known_go_load`) e Node load (`obs_node_load`):
  - Row 1 (nome na lista) → passa (dívida conhecida, job pode ser verde).
  - Row 2 (nome novo) → `has_new = True` (step-6 para Node; 3b para Go).
  - Row 3 (sem nome) → `has_new = True` + mensagem "sem nome".
- `known_go_load` adicionado ao `run_check`.

**Novos testes (T19–T22):**
- T19: Node load-failure marker + nome na lista → exit 0 (o braço cuja ausência causou o defeito).
- T20: Node load-failure marker + nome fora da lista → exit 1, nome na mensagem (row 2).
- T21: Node load-failure marker + `obs_node_load` vazio → exit 1, "sem nome" (row 3, D3).
- T22: `zero-test-failure.node.txt` → exit 1 (row 3, early check, sem nome por construção).

**Gates (macOS arm64, foreground):**
- `python3 scripts/check-windows-known-failures.py --self-test` → **23 PASS, 0 FAIL** (T1-T22)
- `make parity-rest` → exit 0
- `trackfw validate` → 0 errors (174 warnings pré-existentes)
- YAML: `python3 -c "yaml.safe_load(...)"` → válido

---

## Corretivo pós-fechamento — 2026-09-11, PR #316 (ares-tf)

**Defeito encontrado em CI run 34547480139:**

O sumário `ML-2A/2B: 38 observed / 38 active` parecia limpo, mas não estava. Node-assert tinha
**11/10** (+1 regressão nova) e Python tinha **12/13** (-1 dívida paga). Os dois se cancelavam no
total. O gate reprovou corretamente pela checagem por conjunto de nomes — mas **o número de manchete
mentiu**, e o usuário que leu só a primeira linha concluiu que "a catraca foi desligada".

É a **família exata** do defeito que a ADR existe para impedir:
> *"a contagem cai e a queda parece progresso" / "O discriminante é o tipo do evento, nunca a contagem."*

A ADR proibiu decidir por contagem — e decidimos por conjunto, corretamente, mas **reportamos** por
contagem. O relatório reintroduziu a fraqueza que o mecanismo eliminou.

**Segundo defeito (menor):** mensagem de erro "Add to .github/windows-known-failures.json or fix the
test" colocava a lista como primeira saída, convidando ao abuso. Falha nova de código novo se corrige.

### O que foi corrigido

**`scripts/check-windows-known-failures.py` — step 10 (sumário):**

Substituída a lógica de contagem (`len(obs_go) != len(known_go_assert)`) por detecção via **conjunto
de nomes** — os mesmos `obs_set - known_set` e `known_set - obs_set` que os steps 6 e 7 já usam para
a decisão. Invariante: se o gate falha (has_new=True), pelo menos uma classe tem surplus não-vazio →
pelo menos um `[+N NOVO]` aparece na linha de sumário. Um gate que falha nunca mais pode imprimir
manchete limpa.

Formato novo — direção explícita por classe na mesma primeira linha:

```
ML-2A/2B: 38 observed / 38 active / 0 removed — DESEQUILÍBRIO POR CLASSE.
Go 14/14, Node-assert 11/10 [+1 NOVO], Node-load 1/1, Python 12/13 [-1 resolvido].
```

Quando igual count mas nomes diferentes (ex: um nome substituído): `[+1 NOVO, -1 resolvido]`.

**Mensagens de erro (5 sítios):** invertida a ordem — "Fix the test." primeiro. A lista citada
como segunda saída, só para dívida herdada pré-existente ao PR, com source run id exigido.

### Falsificação — 4 braços (T23-T26)

- **T23** (caso do CI): Node-assert +1, Python -1, total igual → sumário acusa `DESEQUILÍBRIO POR
  CLASSE` + `NOVO` + `resolvido` na primeira linha.
- **T24** (contra-braço): todas as classes em equilíbrio → sem marcadores na primeira linha. Sem
  este braço, uma guarda que sempre flagra pareceria funcionar.
- **T25** (classe única): só Node-assert com surplus → `[+1 NOVO]` na primeira linha.
- **T26** (contagem igual, nomes diferentes): Node-assert known={A,B}, obs={A,C} → count 2/2 mas
  surplus={C} → `[+1 NOVO, -1 resolvido]`. Este braço é o que separa o fix correto (baseado em
  conjunto) do fix plausível-errado (baseado em contagem).

### Gates — 2026-09-11, foreground

```
python3 scripts/check-windows-known-failures.py --self-test
→ Self-test summary: 27 PASS, 0 FAIL  (T1-T26)

make parity-rest
→ exit 0

trackfw validate
→ 0 errors (173 warnings pré-existentes)
```

---

## 🔴 REABERTO — 2026-09-11: o ratchet reprovava e nada consumia o veredito

**Esta REQ foi fechada em 2026-09-10 afirmando que o CI bloqueia regressão de Windows. Ela não
bloqueava.** Fechamento prematuro, e é a classe do achado **A1** da auditoria externa de 2026-09-05,
que este projeto já pagou uma vez: marcar concluído algo cujo critério não foi atendido.

**Quem viu foi o usuário**, pelo sintoma certo: *"o gate do windows não está mais como required"*.

### ML-4A — O job reprova, mas não é `required` — medição sem consumidor
**Status:** ✅ Concluído (instância) · **Agente:** arquiteto

Medido em 2026-09-11:

```
gh api repos/kgsaran/trackfw/branches/main/protection --jq '.required_status_checks.contexts'

["go","node","python (3.10)","python (3.12)","package-smoke",
 "windows-integrations-resolve","parity",
 "governance-install-script","governance-go-install"]
```

🔴 **`windows-full-suites` não estava na lista.** O `ML-3A` tirou o `continue-on-error` e o job passou
a reprovar com honestidade — **num lugar onde ninguém agia sobre o resultado**. O PR #316 foi
mergeado com ele vermelho, o que prova o ponto empiricamente.

**A ADR D1 diz:** *"O job **falha** se aparecer um nome fora da lista."* Falhar só significa alguma
coisa se alguém **consome o veredito**.

**Corrigido** — `windows-full-suites` acrescentado aos required, depois de confirmar que estava
**verde** na `main` (run 34595061413) com os 38 vermelhos catalogados. Ligar um check vermelho como
required bloquearia todos os PRs.

**Efeitos declarados:**
- o job é o mais lento do run; todo PR passa a esperar por ele. Se incomodar, a saída é **encurtá-lo**,
  não retirá-lo do required;
- `strict: false` na proteção — PR não precisa estar atualizado com a `main`. PR aberto **antes** desta
  mudança pode ter sido avaliado sem o check.

### ML-4B — 🔴 A CLASSE: nada verifica se os `required` batem com os jobs bloqueantes
**Status:** ⬜ Pendente · **Agente:** `ares-tf`

**Por que ninguém viu durante o dia inteiro:** a lista de required checks vive **fora do
repositório**, numa tela do GitHub. Nenhum `grep`, nenhuma derivação nossa, nenhum gate alcança
aquilo. O acordo entre *"o que o `quality.yml` roda"* e *"o que bloqueia merge"* é **tácito**.

Construímos a catraca inteira — 4 MLs, 3 corretivos, 27 testes de falsificação — e ela **não
bloqueava nada**. Não por defeito de implementação: por **ausência de consumidor**.

**O que fazer:** um gate que compare os `required_status_checks.contexts` da proteção com a lista de
jobs que o repositório **declara** bloqueantes, e reprove na divergência.

**Decisões a tomar e registrar, não presumir:**

1. **Onde mora a declaração?** Não existe hoje. Candidatos: campo no `trackfw.yaml`, arquivo próprio,
   ou anotação nos jobs do workflow. Escolha justificada.
2. 🔴 **O gate consegue ler a proteção?** `gh api .../branches/main/protection` exige token com
   permissão de administração. Num fork, ou num CI sem esse escopo, **a leitura falha**. Se falhar, o
   gate **não pode passar em silêncio** — é a regra da nota de vault
   `guarda-que-reporta-ausencia-precisa-distinguir-nao-achei-de-nao-consegui-procurar-2026-09-10.md`:
   *"não consegui procurar"* é fatal, não aviso.
3. **Direção da verificação.** Required sem job declarado, e job declarado sem required, são dois
   defeitos distintos. Os dois reprovam? Um avisa? Declare.

**Falsificação:**
- required contém tudo que foi declarado ⇒ passa (🔴 contra-braço obrigatório);
- job declarado bloqueante ausente dos required ⇒ reprova, nomeando;
- required contendo check que não existe no workflow ⇒ reprova ou avisa, conforme a decisão 3;
- leitura da proteção indisponível ⇒ 🔴 **reprova**, nunca passa.

**Por que vale mais que o ML-4A:** o 4A corrige a instância. O 4B impede a classe — e a classe é
*"construímos o mecanismo e não o ligamos a quem age sobre ele"*, que hoje apareceu **duas vezes**:
aqui, e no baseline do D4 que nunca rodava porque `origin/main` não era fetchado.
