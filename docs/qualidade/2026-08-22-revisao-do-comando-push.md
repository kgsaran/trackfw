# Revisão de Qualidade — `trackfw push`

> Agente: Hefesto (Code Quality) · ML-3A · 2026-08-22
> Artefatos de governança: REQ-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md · ROADMAP-2026-08-22-trackfw-push-comando-proprio-para-empurrar-commits-ja-criados.md · ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md

---

## Veredito

**APROVADO COM RESSALVAS**

A implementação está funcionalmente correta e a barreira de paridade cobre os cenários declarados. Existem quatro ressalvas que precisam ser resolvidas antes do merge: um comentário factualmente incorreto que é base para a decisão de aceite do ML-1A (alta), duplicação não declarada de `buildPushArgs` no Node.js que viola a AC2 da REQ (alta), dois casos de teste ausentes nos stacks Node.js e Python que eram critério de rejeição na primeira rodada do ML-1A (média), e uma cópia de privados Python com risco mínimo de regressão silenciosa (baixa). O gap do `--force-with-lease` é dívida nomeada e declarada — não é ressalva nova.

**Responsável pelo fix:** `trackfw_architect` para determinar se os itens altos bloqueiam o merge; `apolo-tf` para corrigir o código e os testes sob instrução do arquiteto.

---

## 1. Reuso declarado × duplicação real (por stack)

### Go — `internal/commands/push.go` (238 linhas)

Reuso 100% via visibilidade de pacote. Nenhuma reimplementação local.

| Helper | Origem | Linha de chamada em push.go |
|--------|--------|-----------------------------|
| `isShipBranch` | `ship.go:729` | `push.go:134` |
| `isGatedShipBranch` | `ship.go:736` | `push.go:147` |
| `detectPendingSquashMerges` | `ship.go:771` | `push.go:217` |
| `buildPushArgs` | `ship.go:795` | `push.go:222` |
| `isGitWriteCmd` | `ship.go` | `push.go:112` (via wrapper) |
| `defaultGitExec` | `ship.go:153` | `push.go:89` (injetado em prod) |
| `defaultCheckGovernance` | `ship.go:236` | `push.go:90` (injetado em prod) |
| `defaultCheckPROpen` | `ship.go:186` | `push.go:197` |

Resultado: **zero duplicação**. Modelo Go é o de menor risco de divergência futura.

### Node.js — `npm/src/push/runner.js` (260 linhas)

`module.exports` de `npm/src/ship/runner.js` (verificado nas linhas 748–771) **não expõe**:
- `buildPushArgs` (definida em `ship/runner.js:234`)
- `defaultExecGit` (definida em `ship/runner.js:121`)

Resultado: **duas funções reimplementadas localmente**.

| Função | Linha em push/runner.js | Linha original em ship/runner.js | Lógica idêntica? |
|--------|-------------------------|----------------------------------|-----------------|
| `defaultExecGit` | `35–42` | `121–128` | Sim — wrapper `spawnSync` trivial |
| `buildPushArgs` | `61–67` | `234–239` | Sim — mas contém a decisão `@{u}` que pode evoluir |

As funções exportadas pelo ship e corretamente reusadas pelo push:
`isShipBranch`, `isGatedShipBranch`, `isGitWriteCmd`, `checkShipGovernance`, `detectPendingSquashMerges`, `defaultCheckPROpen`.

### Python — `pypi/trackfw/push/runner.py` (220 linhas)

`default_exec_git` é pública em `ship/runner.py:136` — reuso genuíno.
`_detect_pending_squash_merges` (`ship/runner.py:189`) e `_build_push_args` (`ship/runner.py:225`) são convenção-privadas (prefixo `_`) e importadas explicitamente por push/runner.py sem renomear.

| Símbolo | Linha de import em push/runner.py | Visibilidade no ship |
|---------|-----------------------------------|----------------------|
| `default_exec_git` | `push/runner.py:10` | pública |
| `is_ship_branch` | `push/runner.py:6` | pública |
| `is_gated_ship_branch` | `push/runner.py:7` | pública |
| `is_git_write_cmd` | `push/runner.py:8` | pública |
| `check_ship_governance` | `push/runner.py:9` | pública |
| `default_check_pr_open` | `push/runner.py:11` | pública |
| `_detect_pending_squash_merges` | `push/runner.py:12` | convenção-privada |
| `_build_push_args` | `push/runner.py:13` | convenção-privada |

**Nota ADR vs. código:** O ADR lista `_all_doc_only` entre os símbolos Python reusados pelo push. A função **não é importada** em push/runner.py — o push nunca lê o índice de commits (push.go:144 confirma: "push never reads the index"). O ADR sobrestima a superfície por um símbolo.

Resultado: **zero reimplementação**. Toda a lógica flui do ship/runner.py via import.

### Ranking de risco de divergência futura

1. **Go**: risco zero — mesma compilação, sem cópia.
2. **Python**: risco baixo — ImportError em startup se ship renomear `_build_push_args`; falha é ruidosa e imediata.
3. **Node.js**: **risco alto** — `buildPushArgs` em push/runner.js:61-67 é uma cópia silenciosa; mudança em ship/runner.js:234 não é detectada nem pelo lint nem pelo gate de paridade (que roda apenas com `--dry-run`).

---

## 2. Onde uma mudança futura no ship NÃO propagaria ao push

### Caso alto: `buildPushArgs` no Node.js

**Localização:** `npm/src/push/runner.js:61-67`

```js
function buildPushArgs(branch, execGit) {
  const { error } = execGit(['rev-parse', '--abbrev-ref', '--symbolic-full-name', '@{u}'])
  if (error) {
    return ['push', '-u', 'origin', branch]
  }
  return ['push', 'origin', branch]
}
```

Esta função contém a decisão de incluir `-u` baseada na detecção de upstream (`@{u}`). É plausível que ship evolua para:
- Adicionar suporte a remotes com nome diferente de `origin`
- Mudar a flag usada para detectar upstream
- Adicionar preflight de permissão

Qualquer uma dessas evoluções em `ship/runner.js:234` deixaria `push/runner.js:61-67` silenciosamente desatualizada. O gate `check-push-parity.sh` roda com `--dry-run`, o que exercita `buildPushArgs` indiretamente — mas apenas compara saída do processo inteiro, não o resultado isolado da função. Uma divergência sutil (ex: remote name) poderia passar no gate e falhar apenas em produção.

**AC2 da REQ** declara explicitamente: `"Reuso de buildPushArgs/_build_push_args, não reimplementação."` A implementação Node.js viola essa AC.

**Recomendação:** exportar `buildPushArgs` de `npm/src/ship/runner.js` e remover a cópia local de push/runner.js. Custo: adicionar uma linha ao `module.exports`. Benefício: eliminar o único ponto de divergência silenciosa no stack de maior risco.

### Caso baixo: `defaultExecGit` no Node.js

**Localização:** `npm/src/push/runner.js:35-42`

Esta é uma cópia do wrapper `spawnSync`. A função em si não contém lógica — é apenas uma ponte para o sistema operacional. Mudanças futuras em `ship/runner.js:121` (ex: encoding, timeout, redirecionamento de stderr) **não** propagariam, mas o gate de paridade detectaria divergência de saída porque o comportamento externo muda.

**Recomendação:** exportar `defaultExecGit` do ship também elimina a cópia, mas o custo/benefício é menor que o de `buildPushArgs`. Aceitável manter como está se o arquiteto preferir minimizar mudanças na superfície do ship.

### Python — risco mínimo

`_build_push_args` é importada diretamente de ship/runner.py. Uma mudança em ship propaga automaticamente ao push. O único risco é uma renomeação (ex: `_build_push_args` → `build_push_args`), que causaria `ImportError` na inicialização do módulo — falha ruidosa, não silenciosa. Sem ressalva adicional além da que já consta na seção 3.

---

## 3. Símbolos privados Python: import direto vs. tornar públicos

### Situação atual

`push/runner.py` importa `_build_push_args` e `_detect_pending_squash_merges` com prefixo `_`, que é a convenção de "interno ao módulo" em Python. Python não impede o import — o sublinhado é uma convenção, não uma barreira do runtime.

### Falha de cada abordagem

**Manter privados importados:**
- Pro: funciona agora sem mudança na API do ship.
- Con: nenhum linter (projeto usa apenas `go vet`; `ruff`/`pylint` não estão no CI) sinaliza o import. Se `_build_push_args` for renomeada para `build_push_args` (ex: para torná-la pública), o push quebra com `ImportError` na inicialização — falha ruidosa e rápida, mas é um risco de refatoração.
- Con: semântica errada — o push é um consumidor externo, não um colega de módulo. O contrato implícito é "não use isso fora do ship".

**Tornar públicos (`build_push_args`, `detect_pending_squash_merges`):**
- Pro: contrato explícito; push pode depender deles com garantia de estabilidade da API.
- Con: cresce a superfície pública do ship/runner.py (mais símbolos que o ADR precisaria documentar).
- Pro contextual: o Go já trata esses helpers como parte do pacote `commands` sem barreira — tornar os equivalentes Python públicos aproxima a semântica entre stacks.

**Recomendação:** tornar `_build_push_args` e `_detect_pending_squash_merges` públicos em ship/runner.py é a escolha correta, pois push é um consumidor legítimo e permanente dessas funções. O sublinhado transmite "use com cuidado" — mas aqui o consumidor é o próprio projeto, com rastreabilidade via ADR. A superfície do ship crescida é preferível ao contrato silencioso. Decisão final é do arquiteto.

---

## 4. Três estratégias de reuso entre stacks: dívida aceitável ou nomeada?

| Stack | Estratégia | Risco de divergência | Detectável pelo gate? |
|-------|------------|---------------------|----------------------|
| Go | Visibilidade de pacote (zero cópia) | Nenhum | N/A |
| Node.js | Exportação parcial + 2 cópias locais | Alto (`buildPushArgs`) / Baixo (`defaultExecGit`) | Parcialmente — gate cobre saída, não função isolada |
| Python | Import de privados (zero cópia) | Baixo (ImportError se renomear) | Sim — falha em import |

**Avaliação:** a assimetria é dívida nomeada. O ADR reconhece a situação como "estratégias distintas por restrição das linguagens," mas não nomeia o custo de manutenção nem define quando a dívida deve ser quitada. A AC2 da REQ nega a dívida Node.js ao declarar "reuso, não reimplementação."

**É aceitável?** Tecnicamente, sim — o gate de paridade compensa parte do risco. Como dívida não bloqueante: sim, mas apenas se o arquiteto reclassificar o item Node.js como exceção aceita (e corrigir a AC2 ou a implementação). Como está, a Node.js viola uma AC escrita.

---

## 5. Cobertura de testes — Go 14, Node 7, Python 7

### Inventário completo

| Teste | Go | Node.js | Python |
|-------|:--:|:-------:|:------:|
| `main` branch bloqueada | ✅ | ✅ | ✅ |
| `master` branch bloqueada | ✅ | ✅ | ✅ |
| Branch inválida (`wip/something`) | ✅ `push_test.go:60` | ❌ | ❌ |
| `feat/` sem roadmap → governance falha | ✅ | ✅ | ✅ |
| `chore/` → governance isenta | ✅ | ✅ | ✅ |
| `docs/` → governance isenta | ✅ | ✅ | ✅ |
| Sem upstream → args incluem `-u` (live) | ✅ `push_test.go:133` | ✅ | ✅ |
| Sem upstream → dry-run imprime `-u` | ✅ `push_test.go:153` | ❌ | ❌ |
| Com upstream → sem `-u` (dry-run) | ✅ `push_test.go:173` | ✅ (live) | ✅ (live) |
| `--force-with-lease` sem forge CLI → bloqueia | ✅ `push_test.go:196` | ❌ | ❌ |
| `--force-with-lease` sem PR aberto → bloqueia | t.Skip `push_test.go:237` | ❌ | ❌ |
| NeverCommits (invariante comportamental) | ✅ `push_test.go:258` | ❌ | ❌ |
| DryRun imprime fetch e push | ✅ `push_test.go:272` | ❌ | ❌ |
| GovernanceMessage diz "trackfw push" | ✅ `push_test.go:293` | ❌ | ❌ |

**Contagem efetiva:** Go = 13 executando + 1 skipado (t.Skip). Node = 7. Python = 7.

### Casos mais críticos ausentes

**`TestPush_GovernanceMessage_SaysPush` (`push_test.go:293`) — ausente em Node e Python**

Este teste verifica que a mensagem de governança cita "trackfw push" e não "trackfw ship." O ML-1A foi reprovado na primeira rodada exatamente por divergência de texto entre os stacks. O único stack que verifica essa invariante nos testes unitários é o Go. Qualquer refatoração que introduzir o texto errado nos stacks Node ou Python não seria detectada pelos testes — só pelo `check-push-parity.sh`.

**`TestPush_NeverCommits` (`push_test.go:258`) — ausente em Node e Python**

Invariante comportamental central do push: o comando nunca deve chamar `git commit`. O teste percorre todos os argumentos git chamados e falha se encontrar `commit`. Sem esse teste, um bug que introduzisse uma chamada de commit passaria nos 7 testes existentes do Node e do Python.

**`TestPush_ForceWithLease_NoForgeCLI_Blocks` (`push_test.go:196`) — ausente em Node e Python**

Gate de segurança. O push com `--force-with-lease` sem forge CLI deve ser bloqueado. Não há teste equivalente nos stacks Node e Python. O comportamento está implementado nos três stacks (verificado em push/runner.js:201-203 e push/runner.py), mas não é exercitado por testes unitários nesses dois.

---

## 6. Gap do `--force-with-lease` — avaliação de qualidade

### Evidência

`docs/cli-parity.md` linha 1209 declara explicitamente:

```
<!-- trackfw-contract: gate=scripts/check-push-parity.sh
     partial=roda com TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 e todos os cenários
     usam --dry-run; o push real (git push para o remoto), o caminho
     --force-with-lease e a detecção de squash-merges com fetch real não são
     exercitados ponta a ponta -->
```

O gap foi nomeado no ML-2A e deferido ao ML-3B (segurança). A ML-2A do arquiteto confirma: o gate de paridade roda todos os cenários com `--dry-run` e `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` — o que significa que a refusa do `--force-with-lease` (que acontece no Step 2.5, antes de qualquer write) é executada, mas o push real com `--force-with-lease` não é.

### Detalhe crítico: TestPush_ForceWithLease_NoPROpen_Blocks (`push_test.go:237`)

Este teste está marcado com `t.Skip("forge gate tested in check-ship-force-parity.sh / check-push-parity.sh")`. O justificativo aponta dois gates:
- `check-ship-force-parity.sh`: testa o **ship**, não o push.
- `check-push-parity.sh`: roda com `--dry-run`, conforme declarado no `partial=`.

O caminho "push com `--force-with-lease` + PR não aberto → bloqueia" tem **zero cobertura em todos os níveis** — sem teste unitário em nenhum dos 3 stacks, sem gate de integração. O skip tem justificativa circular.

### Avaliação de qualidade

Não é bloqueante para este ML porque:
1. O gap é declarado com `partial=` antes desta revisão existir.
2. O ML-3B (segurança) está em andamento e cobre exatamente esse ponto.
3. O Step 2.5 é exercitado pelo `--force-with-lease` via `check-push-parity.sh` (o dry-run passa pelo forge resolve + adapter + availability check — apenas o push real não ocorre).

**Mas:** a justificativa do `t.Skip` é inacurada. O arquiteto deve corrigir o comentário do skip para refletir que o gate existe mas não cobre esse caminho, e registrar que a cobertura real fica a cargo do ML-3B.

---

## Ressalvas (ordenadas por impacto)

### [ALTA] Comentário incorreto em `npm/src/push/runner.js:10-14`

**Evidência:** linhas 10–14 afirmam:
> "Note: defaultExecGit is not exported by ship/runner.js (it is package-private). push/runner.js defines its own production git executor using the same implementation — a thin spawnSync wrapper. **This is the only function not reused from ship/runner.js.**"

A afirmação está errada. `buildPushArgs` também não é reusada — é reimplementada em `push/runner.js:61-67`. O ML-1A aceitou a duplicação "porque documentada com comentário explícito." O comentário subestima a duplicação pela metade.

**Impacto:** a decisão de aceite do ML-1A está fundada em uma premissa incompleta.

**Proprietário do fix:** `apolo-tf` (npm/src/push/runner.js).

### [ALTA] `buildPushArgs` duplicada no Node.js viola AC2 da REQ

**Evidência:** AC2 declara "Reuso de `buildPushArgs`/`_build_push_args`, não reimplementação." `npm/src/push/runner.js:61-67` reimplementa `buildPushArgs` localmente.

**Opções:**
1. Exportar `buildPushArgs` de `npm/src/ship/runner.js` e importar em push → satisfaz AC2.
2. Reclassificar como exceção aceita e corrigir AC2 da REQ → deixa a dívida nomeada.

A opção 1 é preferível: custo baixo (uma linha em `module.exports`), elimina o único ponto de divergência silenciosa de alta consequência.

**Proprietário do fix:** `apolo-tf` (npm/src/ship/runner.js e npm/src/push/runner.js), sob instrução do arquiteto.

### [MÉDIA] `TestPush_GovernanceMessage_SaysPush` ausente em Node e Python

**Evidência:** `push_test.go:293` existe. Node e Python não têm equivalente. O texto da mensagem de governança foi critério de rejeição na primeira rodada do ML-1A.

**Proprietário do fix:** `apolo-tf` (npm/tests/push.test.js e pypi/tests/test_push.py).

### [MÉDIA] `TestPush_NeverCommits` ausente em Node e Python

**Evidência:** `push_test.go:258` existe. Node e Python não verificam a invariante "push nunca chama git commit."

**Proprietário do fix:** `apolo-tf` (npm/tests/push.test.js e pypi/tests/test_push.py).

### [BAIXA] Justificativa do t.Skip em `push_test.go:237` é inacurada

**Evidência:** o skip aponta para `check-ship-force-parity.sh` (que testa ship, não push) e `check-push-parity.sh` (que roda com `--dry-run`, não cobrindo o caminho NoPROpen). O skip é correto como decisão; o comentário é inacurado.

**Proprietário do fix:** `apolo-tf` (`internal/commands/push_test.go`).

### [BAIXA] Python importa privados `_build_push_args` e `_detect_pending_squash_merges`

**Evidência:** `push/runner.py:12-13`. Funciona; risco é `ImportError` na renomeação.

**Decisão deferida ao arquiteto:** tornar esses dois símbolos públicos em ship/runner.py é a solução correta a médio prazo, mas não é bloqueante para o merge.

### [INFORMAÇÃO] ADR sobrestima superfície Python em um símbolo

**Evidência:** ADR lista `_all_doc_only` entre os privados Python reusados. `push/runner.py` não importa esse símbolo — o push não lê o índice de commits (push.go:144 confirma). Divergência documental menor; não afeta comportamento.

**Proprietário do fix:** `trackfw_architect` (ADR).

---

## Síntese executiva para o arquiteto

O comando `push` está correto e a barreira de paridade cobre os cenários esperados. As duas ressalvas altas (comentário incorreto e violação da AC2) são corrigíveis sem mudança de arquitetura: exportar `buildPushArgs` de ship/runner.js resolve ambas. As duas ressalvas médias (testes ausentes) são adições diretas nos arquivos de teste. Nenhuma das cinco ressalvas requer alteração da lógica de negócio.

O merge pode ser desbloqueado se o arquiteto decidir que as ressalvas altas são exceções aceitas (e documentar essa decisão) ou após a correção pelos agentes responsáveis.
