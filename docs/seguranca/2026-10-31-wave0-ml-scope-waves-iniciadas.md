# Wave 0 — Threat Model: ml_scope do alvo cobre as waves já iniciadas

> Parecer: Hades (Security) | 2026-10-31
> REQ: REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
> ADR: ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md
> Roadmap: ROADMAP-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md

---

## 1. Enumeração

**Objetivo:** identificar todos os sítios que afirmam ou dependem do recorte 0..N atual e que o ML-1A precisa atualizar.

**Comando de fechamento:**

```
git grep -n -E 'currentWaveInt|waveInt > |waves 0\.\.|0\.\.N|ml_scope|ml-scope' \
  -- internal/ scripts/ docs/cli-parity.md \
  | grep -v 'docs/roadmaps\|docs/req\|docs/adr\|docs/seguranca' \
  | grep -v '^Binary'
```

Resultado: 54 ocorrências analisadas. Sítios que afirmam o contrato 0..N e precisam mudar:

| Sítio | Linha | Afirmação | Ação no ML-1A |
|---|---|---|---|
| `internal/commands/barrier.go` | 606 | comentário do bloco `ml_scope check`: "waves 0..N" | atualizar para "waves 0..max(N, K)" |
| `internal/commands/barrier.go` | 1073 | comentário de `checkMLScope`: "MLs of waves 0..currentWaveInt" | atualizar |
| `internal/commands/barrier.go` | 1173 | comentário do Step 9: "collect scope patterns from all MLs in waves 0..currentWaveInt" | atualizar |
| `internal/commands/barrier.go` | 1192 | **código**: `if waveInt > currentWaveInt { continue }` — recorte 0..N do alvo | substituir pela lógica 0..max(N,K); manter declFailures só para 0..N |
| `internal/commands/barrier.go` | 1702 | comentário de `runBarrier`: "waves 0..N" | atualizar |
| `internal/commands/barrier_ml_scope_test.go` | 1735 | `TestMLScope_TargetRoadmapExcludedFromMemberLoop`: asserta que `wave2-file.go` está em `failures` para `--wave 1` com Wave 2 🔄 | reescrever — após o fix, wave2-file.go deve estar IN scope |
| `docs/cli-parity.md` | 3150 | tabela `ml_scope`: "waves 0..N" | atualizar para "waves 0..max(N, K)" |
| `docs/cli-parity.md` | 3199 | `**Wave N scope.**`: "waves 0..N of the target roadmap" | atualizar |

**Sítios que NÃO precisam mudar:**

- `scripts/check-barrier.sh` Cenário 18: usa roadmap com Wave 0 ✅ e Wave 1 ✅. Só essas duas waves existem. Após o fix: K = max(0,1) = 1, max(N=1, K=1) = 1. Nenhuma wave extra incluída. O cenário continua correto sem mudança. Evidência: fixture do Cenário 18 não declara Wave 2 ou superior.

- `scripts/check-gates-falsify.sh`: não tem nenhuma ocorrência de `currentWaveInt`, `waveInt >`, `ml_scope` nem `ml-scope`. Nenhuma sabotagem por linha literal que o ML-1A quebre.

- `scripts/check-roadmap-barrier-contract.sh`: zero ocorrências de `ml_scope` ou `0..N`.

- `scripts/testdata/roadmap-barrier-corpus-verdicts.tsv`: zero ocorrências de `ml_scope`.

- `internal/generators/`: não contém lógica de recorte de waves; gera apenas o template de `**Files affected:**`.

A lista está fechada. O único código de produto a mudar é `barrier.go:1192`; os demais sítios são comentários, testes e documentação.

---

## 2. Ameaça

**Quem amplia o escopo da wave N sem quebrar regra escrita — o implementador apressado e o arquiteto otimista.**

### 2.1 Marcar ML de wave futura como 🔄/❌/✅ só para liberar arquivos

O roadmap é lido de `HEAD` (`barrierGitShowHEAD`). Somente o arquiteto commita. Se o arquiteto commitar o roadmap com uma wave futura marcada como 🔄 (ou ❌ Bloqueado, que também satisfaz o predicado K, medido), K aumenta e max(N, K) cresce.

**Medição dos marcadores:**

```go
// TestHadesKMeasure_Variants (executado 2026-10-31):
[VARIANT blocked ❌] K = 1 MATCH        // ❌ Bloqueado → K match-positive
[VARIANT canceled ❌ Cancelado] K = 1    // StatusTerminated → K match-positive
[VARIANT abandoned 🚫 Abandonado] K = 1  // StatusTerminated → K match-positive
[VARIANT vs16 variant 🔄️] K = 1         // VS16 normalizado antes da comparação
[VARIANT pending ⬜] K = -1              // NÃO eleva K
[VARIANT no status line] K = -1          // NÃO eleva K
```

**Portanto:** qualquer ML marcado como ❌ Bloqueado (ou ❌ Cancelado, 🚫 Abandonado, 🔄) numa wave futura **eleva K**. O arquiteto que marcar precocemente um ML como "bloqueado" para documentar um impedimento real antes que a wave seja executada eleva K inadvertidamente.

**Existe gate mecânico que proíbe isso?** Não. O validator não tem regra que rejeite 🔄/❌/✅ em wave M quando waves anteriores ainda estão incompletas. A ausência de gate é documentada como residual no §4.

**Atenuante:** o `checkMLScope` lê de HEAD, não de disco. Um agente não pode elevar K editando o roadmap em disco. Apenas o arquiteto (que commita o roadmap) pode fazê-lo, e isso seria visível no diff do roadmap.

### 2.2 K vindo de ML ❌ Bloqueado e a escalada de um marcador

Medido: ❌ Bloqueado eleva K (K match-positive pela categoria StatusPending com primeiro token ❌). Isso é intencional — um ML bloqueado é um ML que começou. O risco está em usar ❌ Bloqueado como sinal de "impedimento antecipado numa wave futura", o que tem o efeito colateral de incluir os `Files affected` dessa wave no escopo.

**Escalada de um marcador — contradição com a ADR:**

A ADR rejeita explicitamente a "Opção 1 — todas as waves do alvo" dizendo que ela "deixa passar trabalho adiantado de wave ainda não iniciada." Mas qualquer único ML marcado como ❌ Cancelado, 🚫 Abandonado ou ❌ Bloqueado na wave mais alta faz `max(N, K)` equivaler ao índice dessa wave — e se existirem waves ⬜ entre N e K, elas ficam dentro do escopo. Na prática, sob Opção A, um único marcador K match-positive na última wave torna o comportamento de `--wave N` equivalente a "todas as waves" para os propósitos de scope. Isso é uma forma enfraquecida do que o ADR rejeitou.

Essa consequência está presente na ADR como consequência negativa implícita (a atribuição por commit está ausente), mas o texto de consequências positivas afirma que "trabalho adiantado de wave que ainda não começou continua reprovando" sem ressalvas. A ressalva necessária é: apenas para waves acima de K.

A decisão de aceitar ou não este gap pertence a KG. Veja P-5 e §3.

### 2.3 Waves com rótulo fracionário (`1b`) e `splitWaveLabel`

`splitWaveLabel("1b") = (1, "b")`. Para `mlScopeComputeK`: wave "1b" tem `waveInt = 1`. Para `checkMLScope`: `currentWaveInt` de `--wave 1b` também é 1. O `max(N=1, K)` usa a mesma base inteira. Não há superfície nova — "1b" e "1" são tratados identicamente pela comparação de inteiros. O `CompareWaveLabels` é usado só para buscar a wave alvo por nome (lookup), não no recorte numérico.

**Medição do fenced status:**

```
[FENCED-STATUS] K = -1 (status inside fence → expected -1)
```

`**Status:**` dentro de bloco de código não conta para K. A superfície de bypass via fenced content não existe.

### 2.4 Broad pattern em wave N+1..K

Um implementador que escreva `**` em `**Files affected:**` de uma wave > N (que eleva K por ter 🔄) tentará incluir todos os arquivos no escopo da wave N. O `addPatterns` atual rejeita padrões sem segmento literal e adiciona a mensagem de rejeição em `declFailures`. Isso precisa continuar sendo aplicado nas waves N+1..K. Risco: um implementador que, para evitar a rejeição, faça um segundo loop direto para `scopePatterns` sem passar por `addPatterns`. Prescrito em P-3.

### 2.5 Caminho de membros da união (inalterado)

O caminho de membros da união (Step 9.5) usa `mlScopeComputeK` dos membros com `waveInt > k` como filtro. Esse caminho não muda com este fix. O membro não pode ampliar o escopo do alvo além do que as waves 0..K do membro declaram, e `addPatterns` ainda rejeita broad patterns dos membros.

---

## 3. Alvos de falsificação nas duas direções

**Para cada AC, o que deve reprovar se o código regredir para 0..N (false negative), e o que deve reprovar se virar "todas as waves" (false positive).**

### AC1 — roadmap de 2 waves iniciadas, arquivos das duas commitados: `--wave 1` passa

| Direção | Condição | Resultado esperado | O teste |
|---|---|---|---|
| Regressão para 0..N | Wave 2 🔄 commitada, `waveInt > currentWaveInt → continue` ainda ativo | `--wave 1` reprova nomeando `wave2-file.go` | Teste de AC1 falha se wave2-file.go está em failures |
| Permissiva "todas as waves" | Waves com MLs ⬜ incluídas | Arquivo de wave ⬜ não iniciada também passa | AC3 captura este desvio (ver abaixo) |

**Gate de falsificação para AC1:** reintroduzir `if waveInt > currentWaveInt { continue }` deve fazer o teste de AC1 reprovar. O roadmap gates da Wave 1 do roadmap exigem isso explicitamente.

### AC2 — arquivo não declarado em wave nenhuma: `--wave 1` reprova nomeando só ele

| Direção | Resultado esperado |
|---|---|
| Regressão 0..N | Ainda reprova (conservative — nada muda para arquivos fora de qualquer wave) |
| Permissiva "todas as waves" | Ainda reprova (um arquivo sem declaração em nenhuma wave continua fora de escopo) |

AC2 não discrimina entre as duas direções. O seu valor está em ser um controle de sanidade independente: confirma que a ampliação do escopo não "apaga" a reprovação de arquivos genuinamente fora de qualquer ML.

### AC3 — arquivo declarado só em wave ⬜ > N: `--wave 1` reprova

**Vetor simples (wave ⬜ acima de K):** Wave 0 ✅, Wave 1 ✅, Wave 2 ⬜ → K = 1, max(1,1) = 1 → wave 2 não incluída → reprova. Correto.

**Vetor do gap (wave ⬜ entre N e K):**

```
TestHadesKMeasure_GapCase (2026-10-31):
Waves: 0✅, 1✅, 2⬜ (x.go), 3🔄 (y.go)
K = 3
max(N=1, K=3) = 3
→ wave 2 files (x.go) IN scope para --wave 1
```

**Contradição com o ADR:** a consequence positiva do ADR diz "trabalho adiantado de wave que ainda não começou (todos os MLs ⬜) continua reprovando." Medido: isso é verdade SOMENTE quando a ⬜ wave está acima de K. Se existe uma wave ≥ N+2 com ML iniciado (K ≥ N+2), uma wave ⬜ entre N+1 e K-1 fica DENTRO do escopo 0..max(N,K) e passa.

**Decisão necessária do KG (não tomo aqui):** duas opções:
- **Opção A — aceitar o gap:** usar 0..max(N,K) como está na ADR; atualizar o texto de consequences para dizer "trabalho adiantado de wave ainda não iniciada ACIMA de K continua reprovando"; AC3 precisa de vetor adicional que teste a ⬜ wave acima de K (não o gap).
- **Opção B — excluir waves ⬜ do intervalo:** usar {0..N} ∪ {waves com pelo menos um ML K match-positive}, pulando waves ⬜ entre N e K; AC3 continua como está e o ADR não precisa de correção textual.

**A escolha entre as opções pertence a KG.** P-5 prescreve que o vetor de gap seja incluído no teste de AC3 com o comportamento esperado deixado em aberto até essa decisão. O ML-1A está bloqueado neste ponto até KG deliberar. Veja P-5.

**Falsificação para AC3:**

| Direção | Resultado esperado |
|---|---|
| Regressão 0..N | `--wave 1` reprova (arquivo de wave ⬜ sem K intervening não está em 0..N) — este vetor ainda passa |
| Permissiva "todas as waves" (incluindo ⬜ waves) | `--wave 1` passa arquivo de wave ⬜ acima de K — AC3 captura com o vetor simples (K=-1) |

### AC4 — ML de wave > N sem `**Files affected:**` não reprova o barrier da wave N

| Direção | Resultado esperado |
|---|---|
| Regressão 0..N | Não há ML de wave > N sendo processado → AC4 não tem regressão para este caso |
| Erro de implementação: `declFailure` para ML sem declaração em wave > N | `--wave 1` reprova com "no **Files affected:** declaration" para ML de wave > N | 

O risco concreto: implementador que, ao ampliar o loop para 0..max(N,K), mantiver a lógica de `declFailures` sem distinguir `waveInt <= currentWaveInt`. Prescrito em P-2.

**Teste para AC4:** roadmap com Wave 0 ✅, Wave 1 ✅, Wave 2 🔄 MAS sem `**Files affected:**` no ML-2A. `--wave 1` deve passar sem mencionar ML-2A em failures.

---

## 4. Residual Declarado

Este design aceita explicitamente não cobrir:

**R1 — Gap de wave ⬜ entre N e K (decisão de KG pendente, Opção A).** Quando existe uma wave > N+1 com ML iniciado (K > N+1), uma wave ⬜ no intervalo N+1..K-1 tem seus `Files affected` incluídos no escopo da wave N sob a Opção A. Um único marcador ❌ Cancelado, 🚫 Abandonado ou ❌ Bloqueado na wave mais alta é suficiente para elevar K até esse nível e incluir todas as waves ⬜ abaixo. Não existe gate mecânico que proíba a marcação antecipada de um ML de wave futura. Esta é a forma enfraquecida da "Opção 1" que a ADR rejeitou; o gap só existe quando há waves ⬜ entre N+1 e K-1.

**R2 — Marcação antecipada de 🔄/❌ para elevar K.** Qualquer ML de wave futura marcado como 🔄 (iniciado), ❌ Bloqueado, ❌ Cancelado ou 🚫 Abandonado eleva K. O validator não tem regra de ordem de waves. O atenuante é que a leitura é de HEAD, e o diff do roadmap fica visível.

**R3 — Atribuição por commit.** `--wave N` não distingue se um arquivo da wave N+1 foi tocado em commits da wave N ou da wave N+1. Isso é consequence negativa explícita do ADR. O fix intenciona estabilidade do veredito, não atribuição precisa por commit.

**R4 — Broad pattern em wave > N gera rejeição e bloqueia `--wave N`.** O `addPatterns` rejeita padrões sem segmento literal com mensagem em `declFailures`, independente de a wave ser ≤ N ou > N. Isso é intencional e correto — um `**` em qualquer ML do alvo deve bloquear. O que NÃO é emitido para waves > N é o `declFailure` de "ML sem declaração": um ML de wave > N sem `**Files affected:**` não reprovará a wave N (AC4), mas um ML de wave > N COM `**` em `**Files affected:**` reprovará por rejeição de broad pattern.

---

## Prescrições

As prescrições abaixo são para o ML-1A (apolo-tf). Cada uma endereça uma superfície identificada acima.

**P-1 — Separar a coleta de padrões da verificação de declaração obrigatória.**

No Step 9 de `checkMLScope`, computar K do alvo com `mlScopeComputeK` antes do loop. Usar `kTarget = max(currentWaveInt, K)`. No loop: se `waveInt > kTarget`, pular. Se `waveInt > currentWaveInt` (mas `waveInt <= kTarget`), coletar padrões mas NÃO emitir `declFailure`. Se `waveInt <= currentWaveInt`, coletar padrões E emitir `declFailure` quando sem declaração. Isso implementa exatamente D1 + D2 da ADR.

**P-2 — Teste de AC4: ML sem `**Files affected:**` em wave > N não gera `declFailure`; ML sem declaração em wave ≤ N continua gerando.**

Escrever teste com duas asserções complementares:
- **Braço A (positivo):** Wave 0 ✅, Wave 1 🔄, Wave 2 🔄 SEM `**Files affected:**` no ML-2A. `--wave 1` deve resultar sem "no **Files affected:** declaration" de ML-2A em failures. Frase da Regra de Reconciliação: "o braço A afirma que declFailures não são emitidos para MLs de waves > N mesmo quando a wave tem K match-positive."
- **Braço B (contra-braço — evita falso verde por omissão de ML ausente):** mesma fixture mas com ML-1A SEM `**Files affected:**`. `--wave 1` DEVE reprovar com "no **Files affected:** declaration" nomeando ML-1A. Frase da Regra de Reconciliação: "o braço B afirma que MLs em waves ≤ N sem declaração ainda geram declFailure."

**P-3 — Broad pattern em wave > N ainda deve ser rejeitado via `addPatterns`.**

A segunda passagem do loop (waves N+1..kTarget) DEVE passar por `addPatterns` (não diretamente em `scopePatterns`). Isso garante que `**` numa wave futura 🔄 continua gerando mensagem de rejeição. Escrever teste: Wave 1 🔄, Wave 2 🔄 com `**` em `**Files affected:**`, `--wave 1` deve resultar em `blocked` com a mensagem de rejeição de broad pattern. Frase da Regra de Reconciliação: "este teste afirma que broad patterns em waves > N ainda são rejeitados pelo mecanismo addPatterns."

**P-4 — K lido de HEAD, não de disco: vetor adicional para `TestMLScope_DiskEditDoesNotAmplifyScope`.**

Adicionar vetor: editar o roadmap em disco para marcar Wave 3 como 🔄 (sem commitar) quando o HEAD tem Wave 3 ⬜. `--wave 1` deve tratar K como se Wave 3 estivesse ⬜ (K do HEAD). Frase da Regra de Reconciliação: "este vetor afirma que uma edição de disco que eleva K não altera o escopo da wave N."

**P-5 — AC3 deve incluir o vetor de gap (wave ⬜ entre N e K). BLOQUEANTE até decisão de KG.**

O vetor de gap é: Wave 0 ✅, Wave 1 ✅, Wave 2 ⬜ com `x.go`, Wave 3 🔄 com `y.go`. Commitar `x.go` e `y.go`. Medido: K = 3, max(N=1, K=3) = 3.

- Se **Opção A:** `--wave 1` passa (x.go in scope). Frase da Regra de Reconciliação: "este vetor afirma que arquivo de wave ⬜ entre N e K não reprova — gap aceito pela ADR atualizada."
- Se **Opção B:** `--wave 1` reprova nomeando x.go. Frase da Regra de Reconciliação: "este vetor afirma que arquivo de wave ⬜ entre N e K reprova — apenas waves com ML K match-positive contribuem."

**O comportamento esperado do vetor de gap está bloqueado até KG decidir entre Opção A e Opção B.** As opções:
- **Opção A** — range contíguo 0..max(N,K), como a ADR descreve estruturalmente; atualizar o texto de consequences positivas da ADR para dizer "trabalho adiantado de wave ⬜ acima de K continua reprovando".
- **Opção B** — {0..N} ∪ {waves com pelo menos um ML K match-positive}; a ADR não precisa de correção textual; o algoritmo do Step 9 precisa de um filtro adicional que pule waves cujo `mlScopeComputeK` retorne -1 (sem ML iniciado).

Nota: com Opção A, um único marcador ❌/🚫/🔄 na última wave eleva K até esse nível e inclui waves ⬜ abaixo — forma enfraquecida da "Opção 1" que a ADR rejeitou (ver §2.2 e R1).

**P-6 — Reescrever `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (AC5). Fixture atual é vacuosa.**

**Achado:** O teste atual (`barrier_ml_scope_test.go:1735`) usa `mlScopeSetupGitRepo`, que commita o roadmap e depois move `refs/remotes/origin/main` para HEAD. Em seguida o teste commita req/adr e re-move origin/main para o novo HEAD, e só então commita os arquivos de wave. O roadmap fica no merge-base (ou antes) e portanto NUNCA aparece em `diffFiles`. Remover `if diffPath == relPath { continue }` no Step 9.5 não altera nenhum veredito porque o target simplesmente não é um candidato de membro. O teste passa hoje e passaria sem a guarda — é vacuoso para a propriedade que afirma.

**Fix obrigatório na fixture:** após mover origin/main para o último commit pré-work, commitar uma edição no roadmap (ex: atualizar um status de ⬜ para 🔄 na Wave 2) para que o path do target apareça em `diffFiles`. Só então o `if diffPath == relPath { continue }` tem efeito mensurável.

**Discriminador:** usar `**` em `**Files affected:**` de Wave 2 🔄 no target. Com a fixture corrigida:
- Se a guarda `if diffPath == relPath { continue }` estiver presente: a mensagem de rejeição de broad pattern aparece **uma vez** (caminho direto do alvo).
- Se a guarda for removida: a mensagem aparece **duas vezes** (caminho direto + caminho do member loop).

O teste deve verificar a contagem exata. Frase da Regra de Reconciliação: "este teste afirma que o target é excluído do member loop — com fixture que o coloca em diffFiles, a mensagem de rejeição de broad pattern aparece exatamente uma vez."

*Aviso para apolo-tf: deduplicar `declFailures` antes de emitir eliminaria este discriminador. Não deduplicar.*

**P-7 — Atualizar comentários em `barrier.go` e `docs/cli-parity.md` (AC6).**

Atualizar os 4 comentários de `barrier.go` (linhas 606, 1073, 1173, 1702) e as 2 ocorrências em `docs/cli-parity.md` (linhas 3150, 3199) para descrever "waves 0..max(N, K)". A tabela de Union em `cli-parity.md` (~3260) já diz "waves 0..K" para membros — acrescentar nota de rodapé explicando que o alvo usa "waves 0..max(N, K)" para distinguir os dois caminhos.

---

*Verificação dos gates da Wave 0:*
```
test -f docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md   # ✅
grep -q 'Prescrições' docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md  # ✅
```
