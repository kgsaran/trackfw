---
status: Accepted
date: 2026-08-18
author: "Zeus (Arquiteto)"
---

# ADR: a ordem de persistência inverte — manifesto antes dos artefatos

> Date: 2026-08-18 | Status: Accepted

## Context

`Manager.mutate()` (`internal/integrations/manager.go`) grava em duas fases:

```
laço 1 (:255)  applyMutation  -> escreve os bytes de TODOS os artefatos do lote em disco
laço 2 (:266)  writeManifest  -> só então persiste cada manifesto, um por escopo
:270           committed = true
```

Entre o fim do laço 1 e o fim do laço 2 existe uma janela em que N arquivos estão no disco,
corretos, e **nenhum** registro foi gravado. O `defer` de rollback cobre erro retornado
normalmente — **não** cobre interrupção: `SIGKILL`, crash, falta de energia. O `defer` nunca roda
nesses casos.

Bug real que originou a investigação (KG, projeto CMDB): 12 arquivos em disco com o mesmo
timestamp, 10 no manifesto. O `agents update --force` então recusava com
`unmanaged artifact ... does not match a trackfw template`.

### Dois fatos medidos que decidem a questão

**1. Cada gravação individual já é atômica.** `atomicWrite` faz `CreateTemp` → `Chmod` → `Write` →
`Sync` → `Close` → `Rename`, e `writeManifest` também passa por ele. **Não há arquivo parcial.**
A janela é **puramente de ordem** entre dois conjuntos de escritas atômicas.

**2. A direção da inconsistência decide o custo do reparo.** Medido em `inspectResolved`
(`manager.go:494`): arquivo **registrado no manifesto e ausente do disco** resolve para
`StateNotInstalled` — estado que `install`/`update --install-missing` conserta **sozinho**.

| ordem | interrupção deixa | quem repara |
|---|---|---|
| **hoje** — artefatos → manifesto | disco à frente do manifesto | `unmanaged`: recusa **até com `--force`**; exige `install --force` **manual** |
| **invertida** — manifesto → artefatos | manifesto à frente do disco | `StateNotInstalled`: **o próprio produto** repara na próxima execução |

## Decision

**Inverter a ordem dos dois laços: persistir os manifestos antes de escrever os bytes dos
artefatos.**

A janela **não desaparece** — sem WAL cross-file não há como eliminá-la, e construir um WAL para
isto seria desproporcional. O que muda é **para que lado ela falha**: de um estado que exige
intervenção humana e emite mensagem de recusa, para um estado que o produto reconhece e conserta.

### Por que não as alternativas

- **Persistir por item** (manifesto após cada artefato): encurta a janela, mas multiplica escritas
  atômicas do manifesto por N e **mantém a direção ruim** — a cada item ainda há um instante com
  disco à frente do registro. Trocar custo de I/O por uma melhoria que não muda a natureza da falha
  é mau negócio.
- **WAL / journal cross-file**: eliminaria a janela, e é a única opção que realmente resolve.
  Rejeitada por desproporção: complexidade de recuperação, formato próprio e superfície de bug nova,
  para um evento que exige interrupção não-tratável no meio de um lote.
- **Não mexer, só detectar**: era a hipótese inicial da REQ. Rejeitada depois de medir o fato 2 — se
  a inversão é barata e converte a falha em auto-reparável, detectar sem inverter deixa o usuário
  convivendo com o caso caro por escolha.

### Consequência aceita, declarada

Durante a operação normal existe um instante em que o manifesto **declara** artefatos que ainda não
estão no disco. Aceito porque:

- `inspectResolved` **lê o arquivo** antes de classificar; ausência vira `StateNotInstalled`, nunca
  `StateCurrent` falso. O manifesto adiantado não produz diagnóstico errado.
- O `defer` de rollback já fotografa **ambos**, arquivos e manifestos; a inversão exige que ele
  continue restaurando os dois — é requisito da implementação, não efeito colateral.
- Concorrência entre dois `trackfw` simultâneos não é cenário suportado hoje, nem antes nem depois
  desta mudança.

## Consequences

**Positivas**
- O caso que motivou a REQ deixa de exigir intervenção humana.
- A mensagem de recusa por `unmanaged` passa a indicar de fato adulteração externa, e não um
  acidente do próprio produto — o diagnóstico fica honesto.

**Negativas / riscos**
- **É o caminho de escrita de todo `install`/`update`.** Qualquer regressão aqui afeta tudo. Exige
  não-regressão explícita do rollback em erro normal.
- A inversão **não** dispensa a detecção (frente 1 da REQ): instalações já feitas com a ordem antiga
  seguem podendo ter o estado ruim, e o `doctor` é o que revela.
