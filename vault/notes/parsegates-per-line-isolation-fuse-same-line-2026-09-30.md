# `ParseGates` executa cada linha como `sh -c` isolado — fusível T_FUSE deve estar na mesma linha que a chamada

> Domínio: barrier/testes · Data: 2026-09-30 · Severidade: **operacional** (contenção ineficaz em testes de segurança)

## O que acontece

`ParseGates` (roadmapdoc.go:668) itera cada linha de um bloco de gate `\`\`\`bash ... \`\`\`` e
enfileira as linhas não-vazias e não-comentário como comandos individuais. Cada comando é
executado via `runGateCommand` → `sh -c "<linha>"` como processo SEPARADO.

**Consequência:** `export VAR=valor` em uma linha NÃO persiste para a linha seguinte. Cada linha
vê o env herdado do processo barrier — não o env modificado pelas linhas anteriores do bloco.

## Por que o fusível T_FUSE falha em gate blocks multi-linha

O padrão de fusível é:
```bash
[ "${T_FUSE:-0}" -lt 3 ] || exit 99; export T_FUSE=$(( ${T_FUSE:-0} + 1 ));
"$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates
```

Se o fusível está em uma linha e a chamada em outra linha no bloco de gates:
- Linha 1: `[ "${T_FUSE:-0}" -lt 3 ] || exit 99; export T_FUSE=1` — exporta no shell da linha 1, processo encerra.
- Linha 2: `"$T_BIN" barrier ...` — novo `sh -c`, T_FUSE não herdado do barrier (T_FUSE foi exportado pelo sh da linha 1, não pelo próprio barrier). O barrier filho herdaria T_FUSE do env do *seu* processo pai (o barrier), que não tem T_FUSE.

**Resultado:** a checagem `[ "${T_FUSE:-0}" -lt 3 ]` SEMPRE passa (T_FUSE sempre 0/unset). O fusível nunca dispara.

## Incidente medido (ML-2A, 2026-09-30)

Gate multi-linha escrito em fixture de roadmap. Variável testada: `TRACKFW_BARRIER_STACK=[] ./tf barrier` (reset da stack). Sem fusível funcional, a cadeia recursiva durou ~120 segundos até o timeout do sistema. O `pkill -9 -x trackfw` também não alvejou `trackfw_bin` nem `tf` — nomes errados para os binários usados nos testes. Nenhum orphan confirmado pelo check posterior, mas a contenção foi incerta.

## Regra

**O fusível T_FUSE E a chamada ao barrier devem estar na MESMA LINHA do gate block.**

Correto (como usado nos testes T1/T2 de `barrier_reentry_test.go`):
```bash
[ "${T_FUSE:-0}" -lt 3 ] || { echo fused >"$T_DIR/fuse.txt"; exit 99; }; export T_FUSE=$(( ${T_FUSE:-0} + 1 )); "$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates 2>>"$T_DIR/child.err"
```

Errado (fusível ineficaz):
```bash
[ "${T_FUSE:-0}" -lt 3 ] || exit 99; export T_FUSE=$(( ${T_FUSE:-0} + 1 ))
"$T_BIN" barrier "$T_ROADMAP" --wave 1 --trust-local-gates
```

## Nota de containment

Para matar processos de teste de barrier, use os NOMES CORRETOS dos binários usados (ex: `pkill -9 -x trackfw_bin` e `pkill -9 -x tf`). O `pkill -9 -x trackfw` só casa um binário nomeado exatamente `trackfw` — não `tf`, não `trackfw_bin`.

O comentário no topo de `barrier_reentry_test.go` (linhas 6-8) já documenta essa regra; este vault note existe para capturá-la para revisões externas ao pacote de testes.
