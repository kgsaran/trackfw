---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md"
squad: "ares-tf"
---

# Roadmap: o ciclo testa onde funciona: faltam cp1252, Windows sem privilegio e consumidor novo

> Created: 2026-09-11 | Status: 🔄 WIP

## Contexto
REQ: docs/req/REQ-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md

Raiz do problema medida: nosso CI roda 18 jobs ubuntu-latest + 6 windows-latest. Quatro issues
recentes (#314, #315, #319, #320) moravam em caminhos que nenhum desses jobs exercita. Este
roadmap entrega os ambientes que os teria detectado. Não corrige #315 e #320 — entrega o
ambiente que os detecta.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model
**Status:** ✅ Concluído
**Arquivos afetados:** (análise — nenhum arquivo de código)

**1. Completude da enumeração de superfícies**

O roadmap cobre quatro lacunas medidas de CI:

| superfície | issue | método de detecção neste roadmap |
|---|---|---|
| console cp1252 em Windows | #314 | job `windows-gates-cp1252` com PYTHONIOENCODING=cp1252 |
| Windows sem privilégio de symlink | #315 | job `windows-symlink-unprivileged` |
| projeto by_agent com 2+ agentes | #320 | script consumer smoke + job `consumer-smoke-by-agent` |
| job verde com anotação de erro | #319 | script `check-job-annotations.py` + workflow_run |

Adicionalmente: AC5 (re-mutação de gate antigo) e AC7 (custo declarado).

Resíduo declarado: não cobrimos Go/Node.js em cp1252 (AC1 só exercita o self-test Python),
não cobrimos ambientes Windows com UTF-8 Mode ativado explicitamente (PYTHONUTF8=1), não
cobrimos ARM64 Windows (runner hospedado é x64).

**2. Modelo de ameaça — quem esvazia esta wave sem quebrar regra escrita**

- Um agente que marca ML-0A ✅ sem preencher as quatro seções (já visto em sessões anteriores).
- Mitigação: a instrução de reconciliação exige que cada job/gate declare em uma frase o que
  afirma, confrontada com a medição. Se a frase não puder ser escrita, o job não deve existir.

**3. Falsificação em ambas as direções**

- cp1252 (AC1): PYTHONIOENCODING=cp1252 com fix → exit 0. Sem fix (plain print('→')) → UnicodeEncodeError.
- symlink sem privilégio (AC2): se os.Symlink falha → job exercita o caminho de erro. Se não falha
  (Developer Mode ativo) → guarda de vacuidade ativa e job REPROVA com mensagem "substitute did not take effect".
- consumidor novo (AC3): job detecta #320 (roadmap sempre em agents[0]) e reprova. Quando #320
  for corrigido, o job deve ficar verde.
- anotação de erro em job verde (AC4): script com --self-test prova as duas direções (success+failure→1,
  success+warning→0).

**4. Resíduo declarado**

- AC2: se o runner não permite desabilitar Developer Mode via registro, a cobertura é zero para
  o caminho sem privilégio. Declarado explicitamente no ML-1B.
- AC4: jobs com continue-on-error: true em step-level emitem ::error:: intencionalmente (ratchet
  julga). O gate precisa de allowlist para esses jobs.
- workflow_run só roda a partir da branch default → AC4 não roda no PR desta feature.
- AC5: a re-mutação cobre check-python-writes-lf.sh (valor errado); outros gates sem cenário de
  valor-errado permanecem como resíduo.

**Critérios de aceite:**
- [x] Quatro seções respondidas com evidência
- [x] Nenhuma linha de implementação neste ML

---

## Wave 1 — Implementação (derivada dos ACs da REQ)
> Dependencies: Wave 0 completa

### ML-1A — AC1: job cp1252 em windows-latest
**Status:** ✅ Concluído

**Arquivos afetados:**
- `.github/workflows/quality.yml` — job `windows-gates-cp1252` adicionado
- `docs/roadmaps/wip/` (este arquivo)

**Ações:**
1. Novo job `windows-gates-cp1252` em `quality.yml`, `runs-on: windows-latest`.
2. Step que roda `python scripts/check-windows-known-failures.py --self-test` com
   `PYTHONIOENCODING=cp1252` e `PYTHONUTF8=0`.
3. Contra-braço: step que verifica que o fix é load-bearing (sem ele, o job reprova).

**Medições (executadas localmente em darwin/arm64):**

- Com fix (PYTHONIOENCODING=cp1252): exit 0, 43 PASS, 0 FAIL ✓
  ```
  PYTHONIOENCODING=cp1252 PYTHONUTF8=0 python3 scripts/check-windows-known-failures.py --self-test
  Self-test summary: 43 PASS, 0 FAIL
  ```
- Contra-braço (simula revert — encode U+2192 em cp1252):
  ```
  UnicodeEncodeError: 'charmap' codec can't encode character '→' in position 4: character maps to <undefined>
  Counter-arm works: plain print would crash here
  ```

**Frase de reconciliação:** Este job afirma que _st_print codifica com replace em vez de crash quando
o codec é cp1252 e o caractere é U+2192. Medição local confirma: com fix → exit 0; sem fix → UnicodeEncodeError.

**Resíduo declarado:** O passo exercita o Python. Go e Node.js em cp1252 não são cobertos por este job.

**Critérios de aceite:**
- [x] Job `windows-gates-cp1252` existe em quality.yml
- [x] PYTHONIOENCODING=cp1252 e PYTHONUTF8=0 declarados
- [x] Contra-braço: medição local mostra que sem fix o job reprova
- [x] actionlint limpo

### ML-1B — AC2: job Windows sem Developer Mode
**Status:** ✅ Concluído

**Arquivos afetados:**
- `.github/workflows/quality.yml` — job `windows-symlink-unprivileged` adicionado

**Ações:**
1. Novo job `windows-symlink-unprivileged` em `quality.yml`, `runs-on: windows-latest`.
2. Step que tenta desabilitar Developer Mode via registro:
   `Set-ItemProperty -Path HKLM:\...\AppModelUnlock -Name AllowDevelopmentWithoutDevLicense -Value 0`.
3. Guarda de vacuidade: prova que os.Symlink falha após desabilitar.
4. Se a guarda de vacuidade falha (Developer Mode não pôde ser desabilitado): job REPROVA com
   mensagem explícita "substitute did not take effect — não há cobertura real do caminho sem privilégio".

**Medição — LIMITAÇÃO DECLARADA:**

Não é possível medir localmente (darwin/arm64). A medição depende de:
(a) O runner windows-latest rodar como usuário com permissão de modificar HKLM.
(b) A modificação de registro ser efetiva para o processo atual (pode exigir reboot ou nova sessão).
(c) O valor 0 em AllowDevelopmentWithoutDevLicense realmente revogar SeCreateSymbolicLinkPrivilege.

Nenhuma das três pode ser confirmada sem rodar no runner. A guarda de vacuidade é o contrato:
se as três condições não valerem, o job REPROVA com "substitute did not take effect" em vez de
passar falsamente. Isso garante que cobertura fingida é impossível — o pior caso é "job vermelho
permanentemente" (que é achado em si: CI não consegue exercitar este ambiente).

**Frase de reconciliação:** Este job afirma que o ambiente exercita o caminho de `os.Symlink` com
privilege negado. Se não conseguir negar o privilege (vacuidade), o job REPROVA — nunca dá falsa
cobertura.

**Critérios de aceite:**
- [x] Job `windows-symlink-unprivileged` existe em quality.yml
- [x] Vacuidade: se symlink ainda funciona após desabilitar, job REPROVA
- [x] Limitação declarada: medição local impossível; resultado depende do runner
- [x] actionlint limpo

### ML-1C — AC3: consumer smoke by_agent (2 agentes, 3 CLIs)
**Status:** ✅ Concluído

**Arquivos afetados:**
- `scripts/check-consumer-smoke-by-agent.sh` — novo script
- `.github/workflows/quality.yml` — job `consumer-smoke-by-agent` adicionado
- `Makefile` — gate adicionado a parity-rest (para satisfazer check-orphan-gates.sh)

**Ações:**
1. Script `check-consumer-smoke-by-agent.sh`: cria projeto descartável com `by_agent` + `agents: [alpha, beta]`,
   roda `req new` e `roadmap new` nos 3 CLIs, verifica onde os artefatos caem.
2. Detecção de #320: roadmap sempre vai para alpha/backlog/ (agents[0]), nunca beta.
   Script sai com exit 1 e mensagem `::error::AC3: #320 detectado`.
3. Job com `continue-on-error: true` TEMPORÁRIO — remover quando #320 for corrigido.
   Critério de remoção: roadmap new (por --req e por agent explícito) rota para o agente correto nos 3 CLIs.

**Frase de reconciliação:** Este job afirma que com projeto by_agent + 2 agentes, o roadmap new
sempre vai para agents[0]. Medição: agentStateDir("", "backlog") usa cfg.Agents[0] (generators/roadmap.go:108).
Quando #320 for corrigido (agent inference via --req), o job deve ficar verde — nesse momento o
continue-on-error deve ser removido.

**Critérios de aceite:**
- [x] Script `check-consumer-smoke-by-agent.sh` existe e tem --self-test
- [x] Job `consumer-smoke-by-agent` em quality.yml com continue-on-error: true + critério de remoção
- [x] Script em parity-rest (consumer declarado → check-orphan-gates.sh fica verde)
- [x] actionlint limpo

### ML-1D — AC4: gate job verde com anotação de erro
**Status:** ✅ Concluído

**Arquivos afetados:**
- `scripts/check-job-annotations.py` — novo script com --self-test
- `.github/workflows/check-annotations.yml` — novo workflow workflow_run
- `Makefile` — `python3 scripts/check-job-annotations.py --self-test` em parity-rest

**Ações:**
1. Script `check-job-annotations.py`: dado job_name + conclusion + annotations_json,
   verifica: success + failure annotation → exit 1; success + warning-only → exit 0.
2. Allowlist: jobs onde ::error:: é intencional via continue-on-error (windows-full-suites,
   windows-defect-reproduction). Declarado explicitamente no script.
3. Workflow `check-annotations.yml`: trigger `workflow_run: [Quality], types: [completed]`,
   `permissions: checks: read`. Lê conclusão e anotações via GitHub API.
4. LIMITAÇÃO DECLARADA: workflow_run só roda da branch default — não roda neste PR.

**Medição da situação atual — continue-on-error + ::error:::**

Jobs que podem estar success com failure annotations:
- `windows-full-suites`: steps de suite têm continue-on-error: true; emitem ::error:: para
  suite-load-failure, zero-test, ML-1D warning sobre git. O RATCHET passo final julga —
  se ratchet passa, job é success mas pode ter ::error:: dos steps de suite.
- `windows-defect-reproduction`: também tem continue-on-error: true.
- `parity-falsify-shard`: continue-on-error: true no nível de job.

Política: allowlist por job name para os três acima. Para outros jobs (go, node, python, package-smoke,
parity): nenhum continue-on-error → success + failure annotation = erro real → reprova.

**Frase de reconciliação:** Este gate afirma que job success com annotation level=failure (fora da allowlist)
indica vazamento de ::error:: não intencional, como o #319 (self-test do ratchet vazava para stdout).
Teste: success+failure→exit 1, success+warning→exit 0. A workflow_run não roda neste PR (branch não-default).

**Critérios de aceite:**
- [x] Script `check-job-annotations.py` com --self-test (ambas as direções)
- [x] Workflow `check-annotations.yml` existe (deferred: não roda neste PR)
- [x] Declaração explícita: policy de allowlist + limitation sobre workflow_run
- [x] Script em parity-rest

### ML-1E — AC5: re-mutação de gate antigo
**Status:** ✅ Concluído

**Arquivos afetados:**
- `scripts/check-gates-falsify.sh` — novo cenário para check-python-writes-lf.sh (valor errado)
- `Makefile` — target `check-gates-remutation` adicionado (pré-release, como check-required-full)

**Ações:**
1. Adicionar cenário a check-gates-falsify.sh: fixture com `open(path, "w", newline="\r\n")` →
   gate deve sair != 0. Este é o gap do #309 (gate verificava presença de newline=, não valor).
2. Target `check-gates-remutation`: roda `scripts/run-gates-falsify-parallel.sh` com flag de
   enumeração, amarrado ao release (não ao CI de PR, como check-required-full).

**Frase de reconciliação:** O novo cenário afirma que check-python-writes-lf.sh detecta `newline="\r\n"`
(valor errado) além de `open()` sem newline= (ausência). Medição: o gate atual usa regex que verifica
a PRESENÇA de `newline=` mas não o VALOR — o cenário vai REPROVAR até a regex ser fortalecida.
Isso é o comportamento correto: o cenário nasce vermelho (detecta o gap do gate), até o gate ser corrigido.

**Critérios de aceite:**
- [x] Cenário para check-python-writes-lf.sh (wrong-value) em check-gates-falsify.sh
- [x] Target `check-gates-remutation` no Makefile
- [x] Cenário declaradamente nasce vermelho (gap do gate exposto)

### ML-1F — AC6: contra-braço para cada ambiente novo
**Status:** ✅ Concluído (integrado nos MLs 1A–1E acima)

Contra-braços declarados por ambiente:
- AC1: medição local confirma crash sem fix (UnicodeEncodeError documentado acima)
- AC2: guarda de vacuidade integrada ao job — reprova se symlink não falhou
- AC3: job nasce vermelho por #320 (o error é o contra-braço: sem #320, ele estaria verde)
- AC4: --self-test prova as duas direções localmente
- AC5: novo cenário nasce vermelho (gap do gate)

### ML-1G — AC7: custo declarado
**Status:** ✅ Concluído

**Medição de custo (de runs anteriores de quality.yml):**

```
windows-full-suites:   ~30-45 min (já o mais lento; o AC12 soma e o ratchet adiciona overhead)
windows-integrations-resolve: ~5-8 min
```

Jobs novos adicionados por este roadmap:
- `windows-gates-cp1252`: estimativa ~4-6 min (apenas python --self-test, sem suite completa)
- `windows-symlink-unprivileged`: estimativa ~5-8 min (registry + probe go + go test)
- `consumer-smoke-by-agent`: ~5-8 min (build 3 CLIs + smoke), roda em ubuntu-latest

Todos rodam em PARALELO com os jobs existentes — custo incremental em wall-clock: próximo de zero
se o runner pool tiver capacidade (PR típico já tem 10+ jobs paralelos).

Decisão: amarrar ao PR (não ao release). Motivo: o consumer smoke e o cp1252 step são leves e
o valor de detectar precocemente é alto. Exceção: AC4 (workflow_run) só roda pós-merge por
limitação da plataforma. AC5 (re-mutação) amarrado ao release, como check-required-full.

**Critérios de aceite:**
- [x] Custo medido ou estimado com base declarada
- [x] Decisão de amarração (PR vs. release) escrita com motivo
