# Parecer de Qualidade — Wave 2 / ML-2B
## Reentrada no barrier — commit 231b2f4c (#485)

**Data:** 2026-09-30
**Revisor:** Hefesto (hefesto-tf)
**Escopo:** diff do commit `231b2f4c` — `internal/commands/barrier.go`,
`internal/commands/barrier_reentry_test.go` (novo), `internal/commands/barrier_test.go`,
`docs/cli-parity.md` (§ Reentrance detection)

---

## Veredito

**APROVA.** Nenhum achado bloqueante. Os dois achados de severidade "deveria" são de
manutenibilidade; os dois opcionais são cosméticos. A corretude funcional, a cobertura de testes
e a documentação de contrato estão adequadas para merge.

---

## Medições realizadas

```
go vet ./internal/commands/          → limpo (sem saída)
gofmt -l barrier.go barrier_reentry_test.go → limpo (sem saída)
go test ./internal/commands/ -run Reentry -count=1 -v
  T1 PASS (1.14s) · T2 PASS (0.08s) · T3 PASS (0.12s) · T4 PASS (0.14s)
  T5 PASS (0.02s) · T6 PASS (0.01s) · T7/symlink PASS · T7/wave PASS
  Total: 7 "^--- PASS" · ok 1.898s
```

---

## Achados

### A1 — Literal `4` sem constante nomeada
**Arquivo:** `internal/commands/barrier.go:581`
**Severidade:** deveria

```go
if len(stack) >= 4 {
```

O número `4` é o "backstop depth" documentado em `cli-parity.md` (seção "Backstop depth (N=4)")
e replicado implicitamente no T5 (que monta pilha com exatamente 4 entradas). Os três pontos de
atualização — código, test, doc — estão desacoplados por um literal solto.

**Correção proposta:** extrair `const barrierMaxDepth = 4` junto com as outras constantes do
bloco de reentrada e substituir o `4` na linha 581 e o comentário em `cli-parity.md` por uma
referência à constante. O T5 já usa `[][2]string{...}` com 4 entradas literais: acrescentar um
`assert.Equal(t, 4, ...)` ou pelo menos um comentário vinculando ao `barrierMaxDepth` seria
suficiente para fechar o acoplamento.

---

### A2 — Assinatura `runGateCommand(command, env)`: `nil` é contrato implícito
**Arquivo:** `internal/commands/barrier.go:376` (nova assinatura)
**Severidade:** deveria

```go
func runGateCommand(command string, env []string) (exitCode int, spawnFailed bool) {
    c := exec.Command("sh", "-c", command)
    c.Env = env   // nil → herda env do processo pai
```

A convenção de Go para `exec.Cmd.Env = nil` ("herdar pai") é correta e documentada no
comentário da função. O risco é do próximo call site dentro de `runBarrier` que, ao esquecer
de passar `childEnv`, vai compilar sem erro e silenciosamente bypassar o stack propagation.

O T1/T2 detectariam a regressão na branch do barrier, mas um call site novo (por ex. para
gate de outro subcomando) passaria nil sem os testes cobrirem aquele caminho.

**Correção proposta:** acrescentar ao comentário da função uma frase explícita sobre quando
`nil` é correto (unit tests diretos de `runGateCommand`, onde não há pilha) vs quando é
proibido (qualquer chamada dentro de `runBarrier` após a pilha ser construída). Alternativa
mais forte: wrapper `runGateCommandWithStack(command string, env []string)` que panics se env
é nil (já que dentro de `runBarrier` nunca deve ser nil pós-ML-1A). Decisão de custo/benefício
— o comentário é suficiente como mínimo.

---

### A3 — T5: `roadmapPath` criado e descartado
**Arquivo:** `internal/commands/barrier_reentry_test.go:~T5`
**Severidade:** opcional

```go
dir, roadmapPath := setupReentryFixture(t, nil)
// ...
_ = dir
_ = roadmapPath
```

`dir` é usado (para construir `sentinel`), mas `roadmapPath` é genuinamente inutilizado.
O `setupReentryFixture` cria toda a estrutura de diretórios e escreve dois arquivos, sendo que
o único valor usado do retorno é `dir`. Leve desperdício de I/O no diretório temporário.

**Correção proposta:** substituir por `dir := t.TempDir()` simples (sem `setupReentryFixture`
para a primeira fixture), ou ao menos renomear para `_ = roadmapPath` e acrescentar comentário
"só dir é necessário aqui, o roadmapPath é do `roadmapPath2`". Nenhum impacto em corretude.

---

### A4 — `runBarrierReentry` vs `runBarrierCLI`: diferença é legítima
**Arquivo:** `internal/commands/barrier_reentry_test.go:runBarrierReentry`
**Severidade:** opcional (observação, não achado negativo)

`runBarrierReentry` adiciona: env explícito, `context.WithTimeout(60s)`, `cmd.WaitDelay = 5s`,
e `cmd.Dir` configurável. `runBarrierCLI` não tem nenhum desses. A separação é deliberada e
correta — os testes de reentrada precisam controlar o env para não herdar `TRACKFW_BARRIER_STACK`
externo. Não há duplicação prejudicial.

---

## Pontos verificados pelo arquiteto — resposta direta

**1. Assinatura `runGateCommand(command, env)` — `nil` é claro ou armadilha?**
O comportamento de `nil` segue a convenção Go stdlib (`exec.Cmd.Env = nil` = herdar pai). Está
documentado no comentário. Os call sites em `barrier_test.go` que passam `nil` são corretos
(unit tests de `runGateCommand` sem pilha). O único risco é um call site futuro dentro de
`runBarrier` — mitigável com comentário (ver A2).

**2. `barrierReentryKey` usa `waveLabel` do argumento, não `target.Label` — divergência possível?**
Não. O `waveLabel` é validado contra `waveLabelRe` antes de entrar em `runBarrier` (linha 84),
então não pode ser uma string arbitrária como a heading completa. A normalização via
`SplitWaveLabel` é idempotente para os valores válidos ("1", "1b", "1-b" → "1" ou "1b").
`target.Label` é o label da wave no roadmap — para o propósito da chave de reentrada, o label
normalizado do argumento é equivalente, como demonstrado pelo T7/wave_label_normalization.

**3. Literal `4` do backstop aparece solto?**
Sim — ver achado A1. Deveria ser constante nomeada.

**4. Fragilidade de tempo ou ordem nos testes?**
Nenhuma. Cada teste usa `t.TempDir()` (isolamento), 60s de timeout (largura confortável vs
tempos medidos de 0.01–1.14s), sem ordem entre testes. T4 roda duas chamadas sequenciais ao
mesmo fixture sem shared state.

**5. `T_FUSE` pode mascarar falha real?**
Não. O fusível CRIA `fuse.txt` antes de sair 99. O teste verifica `os.Stat(fuse) != nil` (fuse
NÃO existe). Se a reentrada não for detida (barrier antigo), o gate recursará, T_FUSE dispara,
`fuse.txt` é criado, e o teste FALHA com "fusível disparou". O fusível só "passa" no teste se
NÃO disparar — o que exige que o barrier tenha recusado antes de rodar o gate. Estrutura
correta.

**6. `cli-parity.md` descreve o que o código faz, mensagem por mensagem?**
Sim. As 3 mensagens literais foram comparadas entre código e doc:
- `"TRACKFW_BARRIER_STACK is malformed: %s"` → `"TRACKFW_BARRIER_STACK is malformed: <json-error>"` ✓
- `"reentrant call — %s wave %s is already being evaluated by an enclosing barrier"` ✓
- `"evaluation depth limit exceeded (%d nested barriers) — possible reentrant call via indirection"` ✓
N=4, calibração documentada, resíduo `env -i` declarado. Contrato completo.

---

## Conclusão

Os 7 testes de reentrada passam. `go vet` e `gofmt` limpos. A documentação de contrato cobre
as 3 mensagens, o exit code 2, o N=4 calibrado e o resíduo de `env -i`. Os achados A1 e A2
são melhorias de manutenibilidade que não bloqueam o merge.
