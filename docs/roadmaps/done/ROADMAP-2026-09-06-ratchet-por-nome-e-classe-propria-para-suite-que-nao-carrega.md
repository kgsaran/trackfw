---
status: done
date: 2026-09-06
squad: ares-tf
req: "docs/req/REQ-2026-09-06-o-ci-de-windows-nao-bloqueia-regressao-e-nao-distingue-suite-que-nao-carregou-de-teste-que-reprovou.md"
---

# Roadmap: Ratchet por nome, e classe própria para suíte que não carrega

> Criado em: 2026-09-06 | Status: done

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
**Status:** ✅ Concluído · **Agente:** `ares-tf`

## Critérios de Aceite
- [x] `.github/required-status-checks.txt` criado com os 10 checks medidos da API
- [x] `scripts/check-required-status-checks.py` entrega exit 0/1/2 corretos sem passar em silêncio
- [x] 6 braços de falsificação (T1-T6): concordância → 0, D\R → 1, R\W → 1, falha → 2, arquivo ausente → 2, arquivo vazio → 2
- [x] `make parity-rest` inclui `--self-test` do gate e passa
- [x] `trackfw validate` sem erros novos
- [x] Job `check-required-checks` no `quality.yml` com `administration: read` isolado
- [x] Fork PR tratado com `if:`-gated (não `continue-on-error`)
- [x] `parity.needs` **NÃO inclui** `check-required-checks` até primeira run de CI confirmar o token (item aberto ao arquiteto — ver abaixo)

**Por que ninguém viu durante o dia inteiro:** a lista de required checks vive **fora do
repositório**, numa tela do GitHub. Nenhum `grep`, nenhuma derivação nossa, nenhum gate alcança
aquilo. O acordo entre *"o que o `quality.yml` roda"* e *"o que bloqueia merge"* é **tácito**.

Construímos a catraca inteira — 4 MLs, 3 corretivos, 27 testes de falsificação — e ela **não
bloqueava nada**. Não por defeito de implementação: por **ausência de consumidor**.

**O que fazer:** um gate que compare os `required_status_checks.contexts` da proteção com a lista de
jobs que o repositório **declara** bloqueantes, e reprove na divergência.

#### As 3 decisões — tomadas com justificativa (2026-09-11)

**Decisão 1 — Onde mora a declaração:**
Arquivo próprio `.github/required-status-checks.txt` (um nome por linha).

Motivo: o contrato é específico deste repositório, não config do produto CLI trackfw —
outros consumidores do trackfw não têm (nem devem ter) esta proteção de branch.
Separado do workflow YAML para tornar edições divergentes visíveis no diff sem exigir parsing;
trivial de auditar; trivial de testar sinteticamente. Não entra em `trackfw.yaml` (config do produto).
Não fica como anotação nos jobs (exigiria parser YAML para extrair, e o significado fica implícito).

**Decisão 2 — O que acontece quando o gate não consegue LER a proteção:**

Medições realizadas (2026-09-11):
```
gh api repos/kgsaran/trackfw --jq '.private'
→ false   (repo público)

gh api repos/kgsaran/trackfw/actions/permissions/workflow
→ {"default_workflow_permissions":"read","can_approve_pull_request_reviews":false}

gh api repos/kgsaran/trackfw/branches/main/protection --jq '.required_status_checks.contexts'
→ ["go","node","python (3.10)","python (3.12)","package-smoke",
   "windows-integrations-resolve","parity","governance-install-script",
   "governance-go-install","windows-full-suites"]
   (com credencial pessoal do KG — não com GITHUB_TOKEN de CI)
```

Nenhum job existente em quality.yml declara `administration: read`. A chamada funciona
localmente com credencial pessoal; **NÃO MEDIDO**: se GITHUB_TOKEN com `administration: read`
consegue ler em CI. Requer run real não autorizado por este ML — **item aberto declarado ao
arquiteto**.

**Comportamento escolhido:**
- Gate falha → exit 2 com `::error::` incluindo raw stderr do gh. **Nunca passa em silêncio.**
- Em fork PR: o step de verificação real é `if:`-gated (não `continue-on-error` — um step
  skippado é visível como "não rodou"; `continue-on-error` pintaria vermelho de verde, defeito
  do ML-3A reintroduzido na própria correção). Um step de aviso imprime a limitação declarada.
- Job dedicado `check-required-checks` com `permissions: contents: read, administration: read`.
  Essa permissão não é adicionada a `parity-other-gates` (que roda ~50 scripts) para não ampliar
  superfície desnecessariamente.

**Decisão 3 — Direção da verificação:**
**Ambas as direções reprovam com exit 1 nomeando os checks divergentes.** Três conjuntos:

- D = `.github/required-status-checks.txt` (declaração)
- R = `required_status_checks.contexts` da API
- W = check names derivados de todos os `.github/workflows/*.yml` (com expansão de matriz)

Checks:
- `D \ R` → exit 1 (job declarado bloqueante ausente dos required — defeito de ontem)
- `R \ W` → exit 1 (required aponta check que o CI nunca emitirá — PR pendente para sempre,
  vault/notes/matriz-em-job-required-por-nome-fica-pendente-para-sempre-2026-09-08.md)
- `D \ W` → exit 1 (declaração fantasma — job declarado que nenhum workflow define)

Motivação: a direção `R \ W` é igual ao segundo defeito registrado no vault (PR que fica pendente).
O custo foi medido e documentado. A terceira direção `D \ W` pega a declaração de um job que não
existe antes mesmo de ele entrar no required.

**Limitação declarada de W:** W é construído a partir de TODOS os jobs em TODOS os workflows, sem
filtrar por trigger (`on:`) ou por `if:` no nível de job. Um check required cujo job esteja num
workflow que NUNCA dispara em `pull_request` (ex: `release.yml`) satisfaz R\W = ∅ mas ainda deixa
o PR pendente para sempre. Estado atual: os 10 checks required pertencem a jobs que disparam em
`pull_request`; a limitação não afeta o gate hoje. Risco residual declarado — não requer mudança
de código agora.

#### Artefatos entregues

- `.github/required-status-checks.txt` — declaração D com os 10 checks medidos
- `scripts/check-required-status-checks.py` — gate Python com `--self-test` (6 braços)
- `Makefile` — `python3 scripts/check-required-status-checks.py --self-test` adicionado ao `parity-rest`
- `.github/workflows/quality.yml` — job `check-required-checks` adicionado com `administration: read`; `parity.needs` **não** inclui o job ainda (ver item aberto abaixo)
- YAML validado com `python3 -c "yaml.safe_load(...)"` → válido, 12 jobs

**Passo pendente (mesmo PR, mesma REQ — regra de mesma causa):** após a primeira run de CI
confirmar que `GITHUB_TOKEN` com `administration: read` lê a proteção da branch (push para main ou
PR interno verde), o arquiteto adiciona `check-required-checks` ao `parity.needs` e atualiza a
mensagem de erro. Esse passo segue o mesmo sequenciamento do ML-4A: *"ligar um check vermelho
como required bloquearia todos os PRs"* — aqui, wirear um job com token incerto no caminho
obrigatório produziria o mesmo efeito.

#### Falsificação — 6 braços (T1-T6)

- **T1** (contra-braço obrigatório): D==R, todos em W → exit 0. Afirma: gate passa quando todos os conjuntos concordam.
- **T2** (D\R): `'parity'` em D mas não em R → exit 1 nomeando `'parity'`. Afirma: D\R ≠ ∅ causa falha nomeando o check ausente dos required (defeito ML-4A).
- **T3** (R\W): `'ghost-job'` em R mas não em W → exit 1 nomeando `'ghost-job'`. Afirma: R\W ≠ ∅ causa falha nomeando o check fantasma (PR pendente para sempre).
- **T4** (leitura falha): `SELFTEST_REQUIRED_FAIL=1` → exit 2 com `::error::`. Afirma: falha do instrumento não passa em silêncio.
- **T5** (arquivo ausente): ROOT aponta dir sem `.github/required-status-checks.txt` → exit 2. Afirma: arquivo ausente dispara guarda de vacuidade — não pode aprovar qualquer configuração.
- **T6** (arquivo vazio): ROOT aponta dir com arquivo vazio → exit 2. Afirma: arquivo vazio dispara guarda de vacuidade — não pode aprovar qualquer configuração.

#### Reconciliação teste↔conclusão (regra dura do projeto)

- T1 afirma: quando D==R e ambos ⊆ W, o gate retorna 0 — nenhuma divergência detectada.
- T2 afirma: quando D\R ≠ ∅ (medido: job adicionado a D sem estar em R), o gate retorna 1 e nomeia o check.
- T3 afirma: quando R\W ≠ ∅ (medido: nome em required sem job correspondente em workflow), o gate retorna 1 e nomeia o check.
- T4 afirma: quando o instrumento falha (simulado via SELFTEST_REQUIRED_FAIL), o gate retorna 2 com `::error::` — nunca passa.
- T5 afirma: quando `.github/required-status-checks.txt` não existe, o gate retorna 2 — arquivo ausente = instrumento falho, não D vazia.
- T6 afirma: quando `.github/required-status-checks.txt` existe mas está vazio, o gate retorna 2 — D vazia aprovaria qualquer configuração, o que é semanticamente falso.

#### Gates — 2026-09-11, foreground

```
python3 scripts/check-required-status-checks.py --self-test
→ Self-test summary: 6 PASS, 0 FAIL

python3 scripts/check-required-status-checks.py
→ check-required-status-checks: [OK] declared=10, required=10, workflow_checks=34 — D\R=∅, R\W=∅, D\W=∅ — todos os conjuntos concordam.
→ EXIT: 0

make parity-rest
→ (tail) Self-test summary: 6 PASS, 0 FAIL
→ exit 0

python3 -c "yaml.safe_load(...quality.yml)"
→ YAML válido. 12 jobs.

trackfw validate
→ 0 errors (174 warnings)
```

🔴 **Item aberto declarado ao arquiteto:** se `GITHUB_TOKEN` com `administration: read` consegue
ler a proteção da branch em CI (PRs internos e push para main) — não medível neste ML, requer
run real. Se falhar em CI, a causa é o token insuficiente; a mensagem de diagnóstico do gate
inclui a instrução de correção. Após confirmação, wirear `check-required-checks` no `parity.needs`
(passo pendente descrito acima).

**Por que vale mais que o ML-4A:** o 4A corrige a instância. O 4B impede a classe — e a classe é
*"construímos o mecanismo e não o ligamos a quem age sobre ele"*, que hoje apareceu **duas vezes**:
aqui, e no baseline do D4 que nunca rodava porque `origin/main` não era fetchado.

#### Corretivo ML-4B — `administration: read` não é escopo de workflow; schema rejeitado → 0 jobs (2026-09-11)
**Status:** ✅ Concluído · **Agente:** `ares-tf`

**Defeito:** declarar `administration: read` em `permissions:` do job `check-required-checks`
fez o GitHub rejeitar o schema do `quality.yml` inteiro — 0 jobs criados, todos os
`required_status_checks` ficam pendentes para sempre. YAML é válido para `yaml.safe_load`
mas inválido para o schema do GitHub Actions.

**Causa raiz:** `administration` é escopo de fine-grained PAT, não de workflow `permissions:`.
Os escopos válidos de workflow não incluem `administration`. Confirmado por actionlint.

**Erro de método identificado pelo arquiteto:** no ML-4B, o agente declarou a permissão no YAML
em vez de medir se GITHUB_TOKEN consegue chamar o endpoint. São duas questões distintas —
e a declaração de um escopo inválido causou o modo de falha `R\W` que o gate foi construído para detectar.

**Medições realizadas (2026-09-11, corretivo):**
```
# FATO 1: actionlint confirma escopo inválido
actionlint .github/workflows/quality.yml
→ linha 1069: "unknown permission scope 'administration'. all available permission scopes
   are 'actions', 'artifact-metadata', 'attestations', 'checks', 'contents', ..."

# FATO 2: endpoint retorna 401 anônimo
curl -s -w '\nHTTP %{http_code}\n' https://api.github.com/repos/kgsaran/trackfw/branches/main/protection
→ {"message": "Requires authentication", "status": "401"}

Controle (/branches/main anônimo):
curl -s -w '\nHTTP %{http_code}\n' https://api.github.com/repos/kgsaran/trackfw/branches/main -o /dev/null
→ HTTP 200

# FATO 3: com token pessoal de KG (scope 'repo')
gh api repos/kgsaran/trackfw/branches/main/protection -i | head -1
→ HTTP/2.0 200 OK

# NÃO CONFIRMADO: GITHUB_TOKEN com 'contents: read' em CI
# Requer fine-grained PAT com 'metadata: read' only ou run real de CI.
```

**Decisão de design — R em CI:**
O endpoint requer autenticação (FATO 2). GITHUB_TOKEN é autenticado mas não pode receber
`administration` (escopo inválido). Se GITHUB_TOKEN com `contents: read` não conseguir chamar
o endpoint em CI, D\R e R\W não são verificadas.

🔴 **Limitação declarada:** o defeito que originou este ML (`windows-full-suites` ausente do
`required_status_checks.contexts`) é um defeito D\R e **NÃO seria detectado** por um gate sem R.
D\W continua funcionando sem token (só arquivos locais do checkout).

Solução para R em CI: `secrets.REPO_ADMIN_TOKEN` (PAT com `repo` scope) — decisão de KG.
Se o GITHUB_TOKEN conseguir (run real confirmará), nenhuma mudança adicional necessária.

**Correções neste corretivo:**
1. `quality.yml` — `administration: read` removido; comentário corrigido (documentando a
   medição e a limitação declarada de R)
2. `scripts/check-required-status-checks.py` — Decision 2 atualizada com medições brutas;
   Limitação de R adicionada ao topo do docstring com declaração explícita
3. `Makefile` — comentário sobre `administration:read` corrigido
4. `vault/notes/administration-nao-e-escopo-de-workflow-github-actions-schema-rejeitado-2026-09-11.md` — nota criada

**Reconciliação teste↔conclusão:**
- Mudança 1 (remoção de `administration: read`) afirma: escopo inválido removido → actionlint
  passa em quality.yml → workflow volta a criar jobs.
- Mudança 2 (Decision 2) afirma: a decisão documenta os três fatos medidos, não uma presunção;
  "NÃO CONFIRMADO" é honesto sobre o que não foi testado.

**Gates (corretivo, sequenciais, foreground, local, macOS arm64):**
```
actionlint .github/workflows/quality.yml
→ (sem saída, exit 0)

actionlint .github/workflows/*.yml
→ apenas avisos shellcheck pré-existentes em windows-census.yml e windows-probe.yml
  (não introduzidos por este corretivo)

python3 scripts/check-required-status-checks.py --self-test
→ 6 PASS, 0 FAIL

make parity-rest
→ 0 FAIL, 0 ERROR (verificado por grep '^FAIL\|^ERROR' no output)

trackfw validate
→ (não executado neste corretivo — sem mudanças em artefatos de governança trackfw)
```

---

## Corretivo de desenho ML-4B — 2026-09-11, PR #317 (ares-tf)

**Defeito:** run CI 34605163678 provou que `GITHUB_TOKEN` retorna 404 ao chamar
`/branches/main/protection`. O conjunto **R** não é obtível em CI.
O gate ficaria permanentemente vermelho em todo PR — reproduzindo o ruído do issue #275.

### A aritmética que decide

| Verificação | Depende de R? | Obtível em CI? |
|---|---|---|
| D\R (job declarado ausente do required — defeito ML-4A) | Sim | Não |
| R\W (required aponta check que CI não emite — PR pendente para sempre) | Sim | Não |
| D\W (declaração fantasma — job não existe em nenhum workflow) | Não | Sim |

As duas verificações que pegam os defeitos documentados dependem de R. D\W é o mais fraco e
não teria pegado o defeito de ontem (`windows-full-suites` ausente do required = D\R).

### Decisão

**1. Alvo `make check-required-full`** — D/R/W completo. Roda localmente onde a credencial
existe. Fatal se R não for legível (mantenedor deveria ter a credencial por definição).

**2. CI roda `--scope dw`** — apenas D\W. Sem token, sem vermelho permanente. A saída
declara explicitamente que R não foi verificado e o motivo — escopo declarado, não degradação
silenciosa (padrão proibido por `vault/notes/guarda-que-reporta-ausencia-precisa-distinguir-
nao-achei-de-nao-consegui-procurar-2026-09-10.md`).

**3. `check-required-checks` entra em `parity.needs`** — o bloqueador era "esperar CI
confirmar o token". Esse ponto está resolvido negativamente (token insuficiente). O escopo
dw não depende de token; o job pode ser consumido.

**4. `make check-required-full` amarrado à rotina de release** — pré-condição do `git tag -a`
(CLAUDE.md §Protocolo de Release, passo 3.5). A lista muda episodicamente; verificar em cada
push (hook) adicionaria latência constante por proteção episódica — caminho direto para
`--no-verify`. Release é o gatilho natural.

### Falsificação — 4 braços novos (T7-T10)

- **T7** (`--scope dw` + token indisponível → exit 0): afirma que a medição do run
  34605163678 (GITHUB_TOKEN retorna 404) não causa vermelho permanente em CI com --scope dw.
- **T8** (`--scope dw` + D\R divergente → exit 0): afirma que R não é consultado em scope dw
  (flag restringe, não é cosmético).
- **T9** (`--scope dw` + D\W divergente → exit 1): afirma que D\W ainda funciona em scope dw.
- **T10** (`--scope dw` + D==W → exit 0, declara "R não verificado"): contra-braço de T9;
  afirma que o gate passa quando D e W concordam, e que a saída inclui a declaração de
  escopo reduzido.

### Critérios de aceite — corretivo

- [x] `--self-test` → 10 PASS, 0 FAIL (T1-T10)
- [x] `--scope dw` com workflows reais → exit 0, declara "R não verificado"
- [x] `--scope full` local → exit 0 (declared=10, required=10, workflow_checks=35 — D\R=∅, R\W=∅, D\W=∅)
- [x] CI job `check-required-checks` roda `--scope dw` (sem token, sem if-gate, sem fork-warning)
- [x] `check-required-checks` em `parity.needs`
- [x] `make check-required-full` — alvo standalone, comentado como pré-condição de release
- [x] CLAUDE.md — passo 3.5 adicionado ao Protocolo de Release
- [x] `actionlint .github/workflows/quality.yml` → exit 0
- [x] `actionlint .github/workflows/*.yml` → apenas pre-existing shellcheck em windows-census/probe
- [x] `make parity-rest` → 0 FAIL, 0 ERROR
- [x] `trackfw validate` → 0 errors (173 warnings pré-existentes)

### Gates — 2026-09-11, foreground

```
python3 scripts/check-required-status-checks.py --self-test
→ Self-test summary: 10 PASS, 0 FAIL

python3 scripts/check-required-status-checks.py --scope dw
→ check-required-status-checks: [OK] [scope=dw] declared=10, workflow_checks=35 — D\\W=∅.
  R não verificado (GITHUB_TOKEN sem permissão de administrador em CI): D\\R e R\\W não
  foram verificados nesta execução. Use 'make check-required-full' para verificação
  completa (requer credencial de mantenedor).

python3 scripts/check-required-status-checks.py
→ check-required-status-checks: [OK] [scope=full] declared=10, required=10,
  workflow_checks=35 — D\\R=∅, R\\W=∅, D\\W=∅ — todos os conjuntos concordam.

actionlint .github/workflows/quality.yml
→ (sem saída, exit 0)

actionlint .github/workflows/*.yml
→ apenas avisos shellcheck pré-existentes em windows-census.yml e windows-probe.yml

make parity-rest
→ 0 FAIL, 0 ERROR

trackfw validate
→ 0 errors (173 warnings pré-existentes)
```
