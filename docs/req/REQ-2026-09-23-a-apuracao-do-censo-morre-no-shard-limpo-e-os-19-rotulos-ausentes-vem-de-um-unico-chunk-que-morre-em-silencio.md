---
status: Done
date: 2026-09-23
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md"
---

# REQ: a apuração do censo morre no shard limpo, e os 19 rótulos ausentes vêm de um único chunk que morre em silêncio

> Date: 2026-09-23 | Status: Done
| Linear Issue:
| Jira Issue:

Origem: medição própria de 2026-09-23, ao preparar a entrada do próximo trabalho depois do
merge do **#414** (REQ do CRLF). Corpus: run `35872779844` e seus 8 artefatos.

## Motivation

O `windows-census.yml` é o **instrumento oficial de medição de Windows** deste projeto. A REQ do
CRLF removeu **87%** dos rótulos ausentes (146 → 19) e mesmo assim o censo **continua terminando com
`TOTAL INCOMPLETO — 2/8 shards`**. Sem total, toda triagem do cluster de Windows é palpite — e este
projeto já estimou 14 falhas num grupo que entregou 2.

Ao recontar do artefato em vez da prosa, **as duas causas apareceram, e nenhuma delas é o que o
relatório do censo diz**.

### 🔴 Causa A — a apuração morre por uma linha, e o shard que passa inteiro é o que a mata

A mensagem `2/8 shards` se lê naturalmente como *"6 shards não subiram log"*. **Está errada:** os 8
artefatos existem, no layout exato que a apuração testa.

```
$ gh api .../runs/35872779844/artifacts --jq '.artifacts[].name'
falsify-shard-0 … falsify-shard-7           ← os 8
$ gh run download 35872779844
falsify-shard-1/shard_1.log                 ← o caminho que a apuração procura
```

O defeito está em `.github/workflows/windows-census.yml:485`:

```bash
CNT_FAIL_GREP=$(grep -ac '^FAIL' "$LOG_FILE" 2>/dev/null || echo 0)
```

`grep -c` **já imprime `0`** quando não casa nada — e **sai com 1**, o que dispara o `|| echo 0` e
acrescenta um segundo `0`. Medido nas duas direções, em `bash`:

| entrada | valor capturado | `$(( ))` |
|---|---|---|
| log **sem** `FAIL` (shard limpo) | `$'0\n0'` | 🔴 `arithmetic syntax error` |
| log **com** `FAIL` (controle) | `2` | ok |
| forma correta `{ grep -ac … \|\| true; }` | `0` | ok |

**Replay do laço de apuração contra os 8 artefatos reais, sem runner de Windows:**

| forma | resultado |
|---|---|
| como está hoje | `SHARDS_FOUND=2` · morre no shard 1 — **reproduz o `2/8` do CI** |
| com a captura corrigida | `SHARDS_FOUND=8` · `OK=225` · `FAIL=9` |

🔴 **A perversidade:** o shard 1 é o **primeiro sem nenhuma falha**. Quanto melhor o resultado, mais
cedo o total morre — e a tabela acusa *"artefato não baixado"*, mandando quem investiga para o lado
errado. É o mesmo padrão que a REQ do CRLF corrigiu: **o instrumento denuncia a causa errada**.

**E a defesa que existia foi derrotada pelo próprio defeito.** O workflow tem uma contagem cruzada
com `awk` justamente para pegar contador mentiroso. Ela disparou — e mentiu:

```
DISCREPÂNCIA FAIL shard 1: grep=0
0 awk=0
```

`grep=0\n0` contra `awk=0`: uma divergência **que não existe**, porque o valor do grep tem duas
linhas. Comparar dois contadores não protege contra uma **forma de captura** quebrada. O que se
corrige é a captura, não a quantidade de comparações.

### Causa B — os 19 rótulos ausentes são UM chunk que morre em silêncio, não 19 defeitos

A REQ do CRLF deixou os 19 enumerados como entrada deste trabalho, com a decomposição por família
(`git-branch-guard/*` dominante). **A decomposição é irrelevante:** medido por shard,

```
$ grep ... 'rotulo esperado AUSENTE' | atribuir ao shard emissor
   19  censo (1)          ← TODOS do shard 1. Nenhum outro shard tem ausente.
```

E o shard 1 é o único que **não emite nem `CHUNK_COMPLETE` nem a linha
`N cenário(s) reprovaram`** — os outros 7 emitem um ou outro. Ele simplesmente para, logo após:

```
OK   [falsify/barrier/blocked-not-detected]      ← última linha do log
```

Os 10 rótulos `git-branch-guard/*` são os cenários **62a** e **62b**, adjacentes no arquivo e ambos
dependentes de `run_go_guard_dump`; o rótulo `setup-s62*-go-baseline-build` **não aparece em log
nenhum** — os cenários nunca começaram.

🔴 **Consequência para o escopo:** o teste de fechamento deste projeto (*"se eu corrigir esta causa,
exatamente estes rótulos fecham, e nenhum outro"*) é satisfazível com **uma** causa. Tratar os 19
como 19 defeitos seria repetir o erro do `IsAbs`.

### Por que isto vem antes do resto do cluster de Windows

Dos 19 rótulos, **15 são falsificação de controle de segurança** (a Wave 0 refutou o `12` que eu
tinha escrito, por aritmética: 19 − `integration-assets/*` 2 − `roadmap-req-frontmatter-path/*` 2 =
**15**) — `git-branch-guard/*` (10), `git-branch-guard-global-script-integrity/*` (2),
`credential-guard-hook-resolvable/detected`, `trust-check/direction-b-detected` e
`barrier/wave-zero-flag-guard-rejected-again-detected`. **Onze deles estão SEM PROVA equivalente**
no Windows, medido por leitura do corpo dos testes Go — 3 cobertos, 1 parcial, 11 sem prova. Enquanto o chunk morre, a **detecção de bypass dos
guards não é exercitada no Windows** — a plataforma onde as escapadas de `git` bruto mais divergem.
Não é higiene de instrumento: é controle de segurança sem prova na plataforma de maior risco.

### População da causa A — varredura da família, não do sítio

```
$ grep -rn '|| echo "\?0"\?' .github/workflows/ scripts/ Makefile | wc -l
9
```

| classe | sítios |
|---|---|
| **(a)** comando que **já imprime** no caminho de falha (`grep -c`) → defeito | **4** — `windows-census.yml:485,486,564,565` |
| **(b)** comando que **não** imprime ao falhar (`wc -l <`, `jq`) → correto | 3 |
| **(c)** fixture/corpus, não é código | 2 |

🔴 **A Wave 0 refutou esta tabela em duas frentes, e o (a) é o único número que sobreviveu:**

- **População estreita — e a varredura corretiva também.** `grep -rnE '\$\([^)]*grep +-[a-z]*c[a-z]* '`
  acha **3 sítios** que a forma `|| echo` não pega. 🔴 **Mas ela própria não casa `--count`**
  (medido: `$(grep --count x f || echo 0)` → não casa; `$(grep -ac x f || echo 0)` → casa). A forma
  longa ficaria fora das duas enumerações; quem fecha esse buraco é o gate do ML-1B, que trata
  `--count` como flag de contagem — `check-git-branch-guard-hook-schema.sh:528`,
  `run-gates-falsify-parallel.sh:225,226`. A família tem **12 sítios, não 9**. Os 3 já estão na
  **forma correta** (`|| true`) — são a prova de que a correção pedida é a que o repo já pratica.
- **Classificação errada.** `scripts/gen-falsify-chunks.py:559` estava em (c) *"fixture/corpus"* e é
  **(b)**: é o template do epílogo que roda em **todo chunk de todo shard**. Partição correta:
  **4 (a) · 4 (b) · 1 (c)**.

## Acceptance Criteria

- [x] **AC1 — Enumeração real da causa A**, pelo critério *"o comando já emite saída no caminho de
      → ML-0A + ML-2E: 4 sítios (a) confirmados; a enumeração foi **refutada duas vezes** — população 12 e não 9, e um (c) que era (b)
      falha"*, classificando cada sítio em **(a)** defeito · **(b)** correto · **(c)** fora. O
      critério é aplicável por terceiro, não julgamento do revisor
- [x] **AC2 — Todo sítio (a) corrigido**, e a correção é da **forma de captura** — não uma
      → forma de captura (`|| true`), não comparação empilhada — o `awk` cruzado já existia e **foi derrotado** pelo próprio defeito
      comparação adicional por cima de uma captura quebrada
- [x] **AC3 — Mecanismo da causa B identificado e escrito**, com a medição que o sustenta. 🔴 Se não
      → causa B **desdobrada em duas**: silêncio (ML-2A/2E) medido; `rc=128` = **MAX_PATH**, falsificado nas duas direções na VM
      for identificável, fica escrito o que foi **eliminado** — hipótese apresentada como causa, não
- [x] **AC4 — Teste de fechamento declarado por causa:** *"corrijo esta causa, exatamente estes
      → frase de fechamento por causa; a de B2 foi **recusada** por falta de medição, e escrita depois do ML-2B
      rótulos fecham, e nenhum outro"*. Sem isso, não se separa nem se agrupa
- [x] **AC5 — Falsificação nas duas direções para a causa A**, exercitada contra os **artefatos
      → replay contra os 8 artefatos reais: forma atual `2/8`, corrigida `8/8 · OK=225 · FAIL=9`
      reais** do run `35872779844` (estão baixáveis; não exige runner de Windows): a forma atual
      reproduz `2/8`, a corrigida dá `8/8`
- [x] **AC6 — 🔴 O censo produz número**: um run pós-correção em `main` termina **sem**
      → 🔴 **run `36017761462` em `main`: 8/8 shards, SEM `TOTAL INCOMPLETO`** · OK=321 · FAIL=11 · ausentes 19 → 4
      `TOTAL INCOMPLETO`, com total por shard. **Este número é a linha de base pós-v8** — e não é
      usado para triar o cluster aqui
- [x] **AC7 — Gate que impede a reintrodução** da forma `$(cmd || echo N)` quando `cmd` já emite no
      → **dois** gates: `check-emitting-capture-fallback.sh` e `check-unguarded-capture-rc.sh`, com piso e formas não cobertas declaradas
      caminho de falha, falsificável, com guarda de não-vacuidade
- [x] **AC8** — `make quality` e **CI** verdes
      → `make quality` RC=0 · 1195 `^OK ` · 0 `: FALHA`; CI verde no PR #419 (mergeado em `fbf1636b`)

## Negative scope — o que esta REQ NÃO faz

- **Não** tria o cluster de Windows nem reconta falhas. Esta REQ **conserta o instrumento**; a
  contagem vem depois, com ele funcionando. É a mesma fronteira da REQ do CRLF, e pela mesma razão:
  medir com régua quebrada produziu o erro do `IsAbs`.
- **Não** corrige os cenários `git-branch-guard/*` em si. Se a Wave 0 mostrar que o chunk morre por
  defeito **dentro** de um cenário, o conserto do cenário entra aqui — mas o **comportamento de
  guard** medido por ele não é revisado nesta REQ.
- **Não** reabre a REQ do CRLF. O AC do censo lá já foi **reescrito pela medição** e o original já
  está declarado não atendido; este trabalho é a continuação registrada, não uma correção dela.
- **Não** toca a REQ-2026-09-03 (as 217 falhas de Windows). Medida e descartada como parente: é
  pré-v8, o roadmap está em `blocked/`, e o corpus é o de testes Go/Node/Python — não o censo de
  falsificação.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md`
