---
status: wip
date: 2026-09-30
req: "docs/req/REQ-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md"
squad: "hades-tf"
---

# Roadmap: gate de wave que reentra no barrier recursa sem limite e um roadmap vira fork bomb

> Created: 2026-09-30 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md
Issue: #485 · Nota de vault: `vault/notes/barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`

Varredura (2026-09-30): nenhuma issue ou REQ aberta com este mecanismo. A REQ-2026-09-11 (resíduos
dos pareceres do barrier) não trata reentrada nem profundidade. O #476 compartilha `barrier.go` e
está **bloqueado atrás deste** (ver o roadmap dele, § BLOQUEADO).

**Trazido da branch do #476 para cá, por ter a mesma causa:** o desarme da mina em
`done/ROADMAP-2026-09-22-...:73` e a nota de vault. Os hunks são idênticos nas duas branches.

**Medição de partida** (extração do 1º bloco cercado após `**Gates da wave:**`, linha que começa com
`^(trackfw|./bin/trackfw|bin/trackfw)\s+barrier\b`): **0** no acervo desta branch · **1** na versão
da `main` do ROADMAP-2026-09-22 (contra-braço: o detector acusa a mina quando ela existe).

## Acceptance Criteria
- [ ] **AC1** — reentrada direta recusada antes de executar, sem multiplicar
- [ ] **AC2** — reentrada por indireção contida
- [ ] **AC3** — 🔴 gate legítimo continua rodando, inclusive o que aninha `barrier` sobre **outro** roadmap
- [ ] **AC4** — duas invocações sequenciais continuam funcionando
- [ ] **AC5** — todo outro executor de gates tem a mesma contenção
- [ ] **AC6** — acervo com 0 gates reentrantes
- [ ] **AC7** — `cli-parity.md` descreve contenção, exit code e mensagem
- [ ] **AC8** — `make quality` e CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependências: nenhuma. 🔴 **BLOQUEIA toda implementação.**

### ML-0A — modelo de ameaça e escolha do discriminante
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Arquivos de leitura:** `internal/commands/barrier.go` (`runGateCommand`, `evalGateCommands`,
`runBarrier`, trust check) · `internal/commands/barrier_contract_test.go` (~:105, ~:258) ·
`scripts/check-barrier.sh` · `scripts/check-roadmap-barrier-contract.sh` · `docs/cli-parity.md`
(§ `trackfw barrier`) · o slash command `/trackfw:barrier` gerado · nota de vault acima · issue #485
**Entregável:** `docs/seguranca/2026-09-30-wave0-gate-reentrante-no-barrier.md`
**Método:** medir. 🔴 **Não escrever implementação.** A Wave 0 tem autoridade para **bloquear**.

**Perguntas:**

1. **Completude da enumeração.** Quais caminhos do produto **executam** conteúdo de roadmap? Não se
   limite ao `barrier`: procure `ship`, `commit`, `roadmap move ... done`, hooks gerados, o slash
   command e os scripts. Se algum outro executar gates, um gate que o chama também recursa, e corrigir
   só o `barrier` não fecha.
2. **Discriminante.** Avalie e **falsifique** cada candidato:
   - (a) contador de profundidade herdado por env (`TRACKFW_BARRIER_DEPTH`) que recusa acima de 1;
   - (b) inspeção estática do gate procurando o subcomando `barrier`;
   - (c) pilha herdada de chaves `(roadmap, wave)` que recusa só a reentrada na **mesma** chave, com
     teto de profundidade como backstop;
   - qualquer outro que a medição sugerir.
3. 🔴 **Contra-braço que derruba o (a) ingênuo.** O `barrier_contract_test.go` builda e **executa**
   `trackfw barrier` como processo filho, herdando o env se `cmd.Env` não for setado, e o
   `scripts/check-barrier.sh` roda `barrier` sobre fixtures. Então um gate legítimo como
   `go test ./internal/commands/...` ou `make quality` **aninha `barrier` sobre outro roadmap**, e com
   um contador cego vira erro de uso. **Meça quantos gates do acervo real caem nisso.**
4. **Exit code e mensagem.** O que é coerente com a convenção do `barrier` (exit 2 = não avaliável ≠
   `blocked`)? Reentrada é usage error ou `not_evaluated`?
5. **Resíduo declarado.** O que o desenho escolhido aceita não cobrir, dito explicitamente.

**Ataque as minhas premissas:**

- Afirmo que **nenhuma** correção de profundidade fecha "DoS por arquivo de texto", porque o gate é
  `sh -c` arbitrário e o trust check não protege clone hostil. Se isso estiver errado, **diga**: muda
  o escopo negativo da REQ.
- Afirmo que o `barrier_contract_test.go` herda o env do pai. **Verifique na linha**, não presuma.
- Afirmo **0** gates reentrantes no acervo desta branch (medição acima). Reimplemente a contagem.

**Critérios de aceite:**
- [x] Enumeração dos executores de gates, com `arquivo:linha`, declarada completa ou com o que falta
- [x] Cada candidato de discriminante com o contra-braço que o derruba ou o sustenta, **medido**
- [x] Contagem de gates do acervo que aninham `barrier` sobre outro roadmap
- [x] Exit code e mensagem propostos, com o motivo
- [x] Resíduo declarado
- [x] 🔴 Se a medição refutar qualquer premissa da REQ, **dizer**
- [x] Nenhuma linha de implementação escrita neste ML

**Gates da wave:**

> ⚠️ **Reescrito em uma linha em 2026-09-30.** A versão original era um `python3 -c "..."` de
> várias linhas e passava com `bash`, mas o `barrier` executa **cada linha** como um comando separado:
> sob o barrier real ela reprovava com 13 falhas. Achado do ML-2A (F4); o erro de escrita era meu.

> 🔴 **O gate de uma wave NUNCA pode ser `trackfw barrier` sobre a própria wave.** É o defeito que
> este roadmap corrige.

```bash
test -f docs/seguranca/2026-09-30-wave0-gate-reentrante-no-barrier.md || { echo "GATE FALHOU: parecer da Wave 0 ausente" >&2; exit 1; }
n=$(python3 -c "import glob,re;B=re.compile(r'^\s*(trackfw|\./bin/trackfw|bin/trackfw)\s+barrier\b',re.M);G=re.compile(r'\*\*Gates da wave:\*\*.*?\n[ \t]*(\`{3,}|~{3,})[^\n]*\n(.*?)\n[ \t]*\1[ \t]*(?:\n|\Z)',re.S);print(sum(len(B.findall(m.group(2))) for f in glob.glob('docs/roadmaps/**/*.md',recursive=True) for m in G.finditer(open(f,encoding='utf-8',errors='replace').read())))"); test "$n" = "0" && echo "Gate W0 OK: parecer presente, 0 gates reentrantes no acervo" || { echo "GATE FALHOU: $n gates que invocam barrier no acervo" >&2; exit 1; }
```

## Wave 1 — Implementação (1 ML)
> Dependências: Wave 0 auditada (✅, commit da Wave 0). Um só ML: código, testes, contrato e doc
> compartilham `barrier.go` e o índice do git — dividir só serializaria.

### ML-1A — pilha de chaves `(roadmap, wave)` no `barrier`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/commands/barrier.go` · `internal/commands/barrier_reentry_test.go`
(novo) · `docs/cli-parity.md` (§ `trackfw barrier`) · `scripts/check-barrier.sh` (só se o contrato
fixar mensagens de exit 2 ali)
**Desenho (do parecer `docs/seguranca/2026-09-30-wave0-gate-reentrante-no-barrier.md`):**

1. **Variável:** `TRACKFW_BARRIER_STACK`, valor = **array JSON** de objetos
   `{"roadmap":"<caminho>","wave":"<label canônico>"}`. JSON e não separador, porque caminho pode
   conter `|`, `:` (Windows) ou qualquer outro caractere.
2. **Chave:** `roadmap` = `filepath.EvalSymlinks(filepath.Abs(roadmapPath))` (se `EvalSymlinks`
   falhar, usar o `Abs`); `wave` = `fmt.Sprintf("%d%s", SplitWaveLabel(target.Label))`, assim
   `1b` e `1-b` viram a mesma chave, igual ao `CompareWaveLabels`.
3. **Onde:** em `runBarrier`, **logo depois** de resolver `target` (o erro "wave not found" continua
   com precedência) e **antes** de qualquer check. Ordem:
   - variável presente e JSON inválido → `usageExit(cmd, "TRACKFW_BARRIER_STACK is malformed: %s", err)`
   - chave atual já na pilha → `usageExit(cmd, "reentrant call — %s wave %s is already being evaluated by an enclosing barrier", filepath.Base(roadmapPath), waveLabel)`
   - `len(pilha) >= 4` → `usageExit(cmd, "evaluation depth limit exceeded (%d nested barriers) — possible reentrant call via indirection", len(pilha))`
4. **Propagação:** só para os **filhos dos gates**. `runGateCommand` passa a receber o ambiente
   (`c.Env`), formado por `os.Environ()` **sem** entradas `TRACKFW_BARRIER_STACK` anteriores, mais a
   pilha com a chave atual acrescentada. No Windows o nome de variável é case-insensitive: remova
   comparando com `strings.EqualFold` no nome. O processo do próprio `barrier` não chama `os.Setenv`.
5. **Resíduo, não implementar:** `env -i` apaga a pilha e o teto junto, e fica **sem** contenção
   (parecer, § Resíduo 1).

**🔴 Segurança operacional:** todo teste de reentrada usa um **fusível em shell** independente da
correção: o gate da fixture começa com
`[ "${T_FUSE:-0}" -lt 3 ] || exit 99; export T_FUSE=$(( ${T_FUSE:-0} + 1 ));`. Assim, contra o
binário antigo o teste **reprova** com exit 99 em vez de virar fork bomb. Rode o teste **antes** de
implementar e registre essa reprovação: é a prova de que o teste afirma algo. Cada `exec.Command`
dos testes tem `context.WithTimeout` de 60 s. Contenção, se algo escapar: `pkill -9 -x trackfw`.
Nunca use `pkill -f`.

**Testes** (`barrier_reentry_test.go`, com `barrierBinary(t)` e `cmd.Env` explícito, **sem** herdar
um `TRACKFW_BARRIER_STACK` de fora):
- T1 reentrada direta: o gate chama o próprio roadmap e a própria wave → a externa sai 1, o gate
  falha com `exit 2`, e o stderr do filho (redirecionado para arquivo) contém `reentrant call`
- T2 indireção: o gate roda `sh ./reenter.sh`, que chama o mesmo par → mesma recusa
- T3 🔴 legítimo aninhado: o gate chama `barrier` sobre **outro** roadmap que passa → a externa sai **0**
- T4 duas execuções sequenciais do mesmo par → ambas 0
- T5 backstop: `cmd.Env` com pilha de 4 chaves distintas → exit 2 e `evaluation depth limit exceeded`
- T6 pilha malformada (`not-json`) → exit 2 e `is malformed`
- T7 canonicalização: reentrada pelo **symlink** do roadmap, e pela wave `1-b` contra `1b` → recusada

**Critérios de aceite:**
- [x] T1–T7 existem e passam: `go test ./internal/commands/ -run 'Reentry' -v`, contando `^--- PASS`
  (7 ou mais). `ok` sozinho não prova nada
- [x] Contra o binário **antigo** (antes da mudança), T1 e T2 reprovam pelo fusível (exit 99). A
  saída é colada no relatório
- [x] `go build ./...` · `go test ./internal/commands/` verde
- [x] `docs/cli-parity.md` § `trackfw barrier`: a variável, o formato, as 3 mensagens literais, o
  exit 2 e o resíduo do `env -i`
- [x] Gate da Wave 0 continua passando (acervo com 0 gates reentrantes)
- [x] `make quality` verde (autorizado neste ML: é a única frente ativa)
- [x] Relatório com uma frase por teste novo dizendo **qual conclusão** ele afirma (Regra de Reconciliação)

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'Reentry' -count=1
n=$(go test ./internal/commands/ -run 'Reentry' -count=1 -v 2>&1 | grep -c '^--- PASS'); test "$n" -ge 7 || { echo "GATE FALHOU: $n testes de reentrada passaram, esperava >= 7" >&2; exit 1; }
grep -q 'TRACKFW_BARRIER_STACK' docs/cli-parity.md
```

## Wave 2 — Revisão independente (2 MLs em paralelo, só leitura de código)
> Dependências: Wave 1 auditada. Os dois escrevem **só** o próprio parecer: arquivos disjuntos.

### ML-2A — segurança: reimplementar a partir da leitura
**Owner:** `hades-tf`
**Status:** ✅ Concluído
**Entregável:** `docs/seguranca/2026-09-30-wave2-revisao-reentrada-barrier.md`
**Ações:** sem olhar os testes do ML-1A, derivar do código quais entradas contornam a pilha (fora o
`env -i`, já declarado) e **tentar**: caminho com `..`, hardlink, roadmap em `done/` vs `wip/`,
variável duplicada no env, JSON com chaves extras e Windows (case do nome da variável). Toda
reprodução com fusível e `timeout`.
**Critérios de aceite:**
- [x] Cada contorno tentado com o comando e a saída
- [x] Veredito: aprova, ou bloqueia com um ML corretivo proposto

### ML-2B — qualidade de código
**Owner:** `hefesto-tf`
**Status:** ✅ Concluído
**Entregável:** `docs/qualidade/2026-09-30-wave2-reentrada-barrier.md`
**Ações:** revisar o diff do ML-1A: assinatura do `runGateCommand`, duplicação, testes frágeis
(tempo, ordem) e se o fusível pode mascarar uma falha real.
**Critérios de aceite:**
- [x] Achados com `arquivo:linha` e severidade
- [x] Veredito: aprova ou bloqueia

**Gates da wave:**
```bash
test -f docs/seguranca/2026-09-30-wave2-revisao-reentrada-barrier.md
test -f docs/qualidade/2026-09-30-wave2-reentrada-barrier.md
```

## Wave 3 — Corretivo (bloqueador do ML-2A)
> Dependências: Wave 2 auditada (ML-2A ✅, ML-2B ✅). Bloqueia: dois achados do ML-2A (F1 desvio de spec, F2 identidade por `os.SameFile`).

### ML-2C — corrigir F1 (spec deviation) e F2 (os.SameFile) do ML-2A
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/commands/barrier.go` · `internal/commands/barrier_reentry_test.go` · `docs/cli-parity.md`

**Achados que fecha:**
- **F1 (spec deviation):** `barrier.go:573` usa `waveLabel` (CLI arg) em vez de `target.Label` (header parsed). `CompareWaveLabels` normaliza case para ENCONTRAR a wave, mas a chave usa `SplitWaveLabel(waveLabel)` que preserva case — `1b` e `1B` produzem chaves distintas. Fix: `barrierReentryKey(roadmapPath, target.Label)`.
- **F2 (identidade por string):** comparação `entry.Roadmap == currentKey.Roadmap` falha para hardlinks e APFS basename case (EvalSymlinks preserva o case dado, não o canônico do disco; `os.SameFile(lower, upper) = true`). Fix: `sameRoadmapFile(a, b string) bool` usando `os.SameFile`.
- **F3 (doc):** `docs/cli-parity.md` residual não lista `TRACKFW_BARRIER_STACK=""`, `=null`, `=[] cmd` como formas de override direto. Fix: documentação.

**Ações:**
1. `barrier.go:573`: mudar `barrierReentryKey(roadmapPath, waveLabel)` → `barrierReentryKey(roadmapPath, target.Label)`.
2. `barrier.go`: adicionar `func sameRoadmapFile(a, b string) bool` com `os.SameFile` (fast path `a == b`).
3. `barrier.go` loop (linhas ~574-578): mudar `entry.Roadmap == currentKey.Roadmap` → `sameRoadmapFile(entry.Roadmap, currentKey.Roadmap)`.
4. `barrier_reentry_test.go`: subtestes T7c (1b/1B), T7d (hardlink), T7e (APFS case — skip se FS case-sensitive). **Fusível e chamada na mesma linha de gate.** Para cada teste, registrar no relatório que o vetor REPROVA antes da correção.
5. `docs/cli-parity.md` § residual: acrescentar formas de override direto.
6. **(Hefesto A1)** `barrier.go`: `const barrierMaxDepth = 4`, usada no backstop; o T5 monta a pilha
   com `barrierMaxDepth` entradas; o `cli-parity.md` cita o nome da constante ao lado do número.
7. **(Hefesto A2)** comentário de `runGateCommand`/`evalGateCommands`: dentro de `runBarrier` é
   **proibido** passar `env == nil`, porque isso desliga a propagação da pilha em silêncio. `nil` só
   é aceitável em teste unitário direto.
8. 🔴 **Gate de uma linha só.** O `ParseGates` executa **cada linha** do bloco como um `sh -c`
   separado (nota `vault/notes/parsegates-per-line-isolation-fuse-same-line-2026-09-30.md`). Todo
   gate de fixture tem o fusível **na mesma linha** da chamada ao `barrier`.

**Critérios de aceite:**
- [ ] T7c/T7d/T7e existem e passam
- [ ] Contra o binário ANTES de (1): T7c reprova (bypass medido)
- [ ] `go test ./internal/commands/ -run 'Reentry' -count=1`: todos passam
- [ ] `make quality` verde
- [ ] Uma frase por teste novo afirmando a conclusão (Regra de Reconciliação)
- [ ] `grep -c 'barrierMaxDepth' internal/commands/barrier.go` ≥ 2 e nenhum `>= 4` literal no backstop

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -run 'Reentry' -count=1
grep -q 'barrierMaxDepth' internal/commands/barrier.go
grep -q 'os.SameFile' internal/commands/barrier.go
```
