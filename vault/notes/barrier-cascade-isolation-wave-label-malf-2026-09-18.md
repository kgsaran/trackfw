# barrier: wave malformada abortava documento inteiro (cascade isolation — ML-1D #392)

**Data:** 2026-09-18
**Revoga:** ADR-2026-07-29 decisão 16 ("heading outside grammar aborts entire document — feature, not defect")
**Substitui por:** ADR-2026-09-18 decisão 12 (cascade isolation)

## Causa raiz

`ParseWaves` devolvia `([]WaveBlock, error)`. Ao encontrar qualquer heading `## Wave X` com rótulo
fora da gramática (ex.: `## Wave 1b`, `## Wave reaberta`), retornava o erro imediatamente e
descartava todos os blocos já parseados.

O chamador em `barrier.go` propagava esse erro como exit 2 (usage error), mesmo que a wave alvo
pedida com `--wave N` fosse completamente válida e estivesse presente no documento.

**Efeito medido:** dois roadmaps reais (`ROADMAP-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md`
e `ROADMAP-2026-08-16-serve-amarra-em-loopback-por-padrao-com-opt-in-explicito-para-exposicao.md`)
eram **completamente invisíveis** a `barrier --wave N`, com N=1,2,3,4,5 — todos os comandos
saíam exit 2 por causa de `## Wave 1b` na linha 58 do primeiro e em posição similar no segundo.

## Por que não era óbvio

A decisão 16 do ADR-2026-07-29 documentava este comportamento explicitamente como intenção
("heading outside grammar aborts entire document — feature, not defect"). Qualquer agente
lendo apenas a ADR concluiria que estava correto.

Só ao medir `barrier <convergencia-harness> --wave 2` diretamente (rótulo válido, mas o documento
contém `1b`) é que o efeito real ficou visível: exit 2 reclamando do `1b` para um comando que
pedia a wave `2`.

## Solução aplicada (ML-1D)

1. `WaveLabelRe` ampliado: `^\d+(?:-[a-zA-Z0-9]+)?$` → `^\d+(?:-?[a-zA-Z0-9]+)?$`
   (hífen antes do sufixo opcional; `1b` agora é rótulo válido).

2. `ParseWaves` novo contrato: `([]WaveBlock, []MalformedWave)`.
   Heading malformado → registra `MalformedWave{Line, Token}` e **continua**.

3. `barrier.go`: para cada `MalformedWave`, emite warning no stderr e continua.
   Exit 0/1 baseado nas waves válidas encontradas.

4. Fail-safe fechado: `HasUnfinishedMLs` retorna `true` quando `len(malformed) > 0`.
   Wave malformada nunca conta como "concluída".

5. `--wave X` com rótulo inválido ainda sai exit 2 (flag validation, antes do parser).

## Cobertura de casos limítrofes

- `abc`, `reaberta`, `X` (sem dígito inicial) — continuam inválidos pelo `WaveLabelRe`.
- `## Wave reaberta` nos 2 arquivos do corpus: NÃO estão no snapshot de 144 arquivos; EXIT2=0
  no corpus é correto. Correção desses arquivos no ML-4A (renomear o rótulo, não mexer na gramática).
- `SplitWaveLabel("1b") == (1, "b")` — idêntico a `SplitWaveLabel("1-b")`.
- `CompareWaveLabels("1b", "1-b") == 0` — `--wave 1-b` resolve `## Wave 1b`.

## Arquivos afetados

- `internal/roadmapdoc/roadmapdoc.go` — `WaveLabelRe`, `MalformedWave`, `ParseWaves`, `SplitWaveLabel`
- `internal/commands/barrier.go` — warnings, lookup via `CompareWaveLabels`
- `internal/roadmapdoc/roadmapdoc_test.go` — 6 novos testes
- `internal/commands/barrier_test.go` — `TestParseWaves_MalformedHeadingIsCascadeIsolated_ML1D`
- `scripts/check-barrier.sh` — Cenários 8 e 9 reescritos
- `scripts/check-gates-falsify.sh` — Cenário 19 atualizado
- `scripts/testdata/roadmap-barrier-corpus-verdicts.tsv` — +33 linhas, append puro
- `scripts/check-roadmap-barrier-contract.sh` — pins atualizados
- `docs/adr/ADR-2026-09-18-...md` — decisão 11 emendada, decisão 12 adicionada
- `docs/cli-parity.md` — gramática e tabelas atualizadas
