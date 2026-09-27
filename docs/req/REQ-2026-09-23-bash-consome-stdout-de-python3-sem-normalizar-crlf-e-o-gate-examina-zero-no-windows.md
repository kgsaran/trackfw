---
status: Done
date: 2026-09-23
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md"
---

# REQ: bash consome stdout de `python3` sem normalizar CRLF, e o gate examina zero no Windows

> Date: 2026-09-23 | Status: Done
| Linear Issue:
| Jira Issue:

Origem: **#353** (consumidor externo, com medição) e achado próprio de 2026-09-23 no
`windows-census.yml`. Candidatos a mesma causa, **a confirmar pela Wave 0**: #363, #307, #364, #308.

## Motivation

No Windows, `python3` em modo texto traduz `\n` → `\r\n` no stdout. Quando um script bash consome
essa saída — por pipe, substituição de processo ou redirecionamento para arquivo lido com
`while read` —, **cada valor carrega um `\r` invisível no fim**.

O efeito não é erro: é **silêncio**. O gate roda, não encontra nada, e reporta sucesso — ou reprova
por vacuidade sem explicar a razão verdadeira.

### Ocorrência 1 — medida por consumidor externo (#353)

`scripts/check-no-literal-nul-in-source.sh` monta a lista com
`git ls-files --eol | python3 -c ...` e consome com `while IFS= read -r rel`:

```
$ python3 -c 'print("Makefile")' | od -c
0000000   M   a   k   e   f   i   l   e  \r  \n
```

| | resultado |
|---|---|
| caminhos recebidos | 671 |
| terminando em `\r` | **671** |
| `[[ -f ]]` como o gate lê | **0** |
| `[[ -f ]]` com só o `\r` removido (controle) | 671 |

**O gate examinava 0 de 671 arquivos.**

### Ocorrência 2 — medida por mim em 2026-09-23, no instrumento de medição

`scripts/run-gates-falsify-shard.sh:85` redireciona o stdout do gerador Python para
`manifest.txt`, e lê com `done < "$WORKDIR/manifest.txt"` (linhas 122 e 131).

Resultado no censo de Windows (run `35856380122`): **8/8 shards reprovados**, **146 rótulos únicos
acusados ausentes** — e a apuração se recusou a dar total (`TOTAL INCOMPLETO — 2/8 shards`).

🔴 **O padrão que denuncia o mecanismo:** o mesmo rótulo aparece **emitido e acusado ausente**:
```
OK   [falsify/release-tag-parity/content-from-commit-false-negative]     ← emitido
AUSENTE: release-tag-parity/content-from-commit-false-negative           ← acusado
```

Porque a guarda compara de duas formas:
- **linha 118**, rótulo literal: `grep -qxF` — linha **exata**, não casa com o `\r` → acusa ausente;
- **linha 127**, glob: `grep -qF` — substring, casa mesmo com `\r` → passa.

Reproduzido localmente, sem VM:

| manifesto | comparação | resultado |
|---|---|---|
| LF | `grep -qxF` | casa |
| **CRLF** | `grep -qxF` | **AUSENTE** |
| CRLF | `grep -qF` | casa |

**Consequência:** o instrumento oficial de medição de Windows **não produz número**. Em 2026-09-10
ele produzia (8/8 shards com contagem); hoje, 2/8. Sem ele, qualquer triagem do cluster de Windows é
palpite — e este projeto já estimou 14 falhas num grupo que entregou 2.

### Medição inicial da população (aproximada — a Wave 0 refuta ou confirma)

```
$ grep -rln 'python3\|PY_BIN\|$PYTHON' scripts/*.sh | wc -l
32
$ # desses, normalizam \r:
1   (check-gates-falsify.sh)
$ ls scripts/*.sh | wc -l
57
```

**32 dos 57 scripts** tocam `python3`; **1** normaliza. Os 32 **não são todos defeito** — só os que
consomem o **stdout** do Python como dado. Separar isso é entregável da Wave 0.

## Acceptance Criteria

- [x] **Enumeração real** dos sítios em que bash consome saída de `python3` como dado, classificada
      em: **(a)** consome e **não** normaliza → defeito · **(b)** normaliza → correto · **(c)** invoca
      Python sem consumir saída → fora
      → ML-1A-0: **(a)=19 · (b)=0 · (c)=11**; o gate do ML-1B achou o 20º sítio que a enumeração perdeu
- [x] Todo sítio **(a)** corrigido, por **ponto único** — não por `tr -d '\r'` espalhado
      → ponto único `scripts/lib-crlf-normalize.sh` (`strip_cr`), sourceado por `SCRIPT_DIR`; ML-1C levou a normalização para **dentro** das 4 funções intermediárias (11+ call sites)
- [x] 🔴 **Gate que impede a reintrodução**, falsificável, e que **reprove** quando um consumo novo
      nascer sem normalização
      → `scripts/check-crlf-normalize-capture.sh`, 5 braços de falsificação; cobre 3 formas de captura e **declara** as 2 que não cobre; piso de 68 candidatos / floor 50
- [x] 🔴 **O censo de Windows volta a produzir número**: `windows-census.yml` com **8/8 shards** e
      apuração sem `TOTAL INCOMPLETO`
      → 🔴 **AC reescrito pela medição** (ver roadmap, Wave 2): rótulos ausentes caem de **146 → 19**; os 19 remanescentes ficam **enumerados**, entrada medida do próximo trabalho
- [x] Falsificação nas duas direções, **exercitada no Windows** — é a plataforma onde o defeito vive
      → 3 braços `crlf-normalize/*` colhidos pela guarda de conjunto; censo rodado na branch (run `35872779844`)
- [x] `make quality` e **CI** verdes
- [x] 🔴 **AC7 (2026-09-27)** — o `strip_cr` chega ao **literal embutido** (`scaffold.go:924,925`), a
      cópia versionada é **regenerada a partir dele** (e não o contrário), e o
      `check-crlf-normalize-capture.sh` passa a **varrer os literais embutidos** — com falsificação
      que reprove um literal novo sem normalização
      → local RC=0 (903 `^OK `, 0 `: FALHA`, falsificação 252 OK); **CI: 21 checks verdes** no PR #414, incluindo os 6 jobs de Windows

## 🔴 REABERTA em 2026-09-27 — o ponto único nunca alcançou o literal distribuído

**AC7** — o `strip_cr` chega ao **literal embutido**, que é o que o produto escreve na máquina de
quem adota, e o gate passa a **varrê-lo**.

### O que aconteceu

A correção de #414 (`1f80abb3`) foi aplicada **só na cópia versionada**
`scripts/trackfw-attention-signal.sh`. O **literal embutido** em `internal/generators/scaffold.go`
— origem do script que `trackfw init` e `discover --init` gravam no projeto do consumidor — **nunca
a recebeu**:

```
scripts/trackfw-attention-signal.sh   (cópia versionada)   strip_cr = 2
internal/generators/scaffold.go       (literal embutido)   strip_cr = 0
```

Em 2026-09-27 encontrei a cópia versionada **revertida na árvore** — `strip_cr = 0` —, regenerada a
partir do literal por um comando do próprio produto. O defeito voltou sozinho.

🔴 **A consequência que importa não é local.** Todo consumidor que rodar `init` ou `discover --init`
na **v9.0.0** recebe o script **sem** a normalização. O defeito do #353 **continua sendo
distribuído**, e a `v9.0.0` já está publicada nos três canais.

### Por que o gate desta REQ não pegou — a causa estrutural

```
scripts/check-crlf-normalize-capture.sh:232
    for f in "$SCAN_ROOT/scripts/"*.sh
```

O gate criado **por esta REQ** para impedir a reintrodução varre **apenas `scripts/*.sh`**. Ele
nunca olha `internal/generators/*.go`. A correção foi aplicada exatamente onde o gate enxerga, e o
sítio que **distribui** ficou fora do seu campo de visão.

É a mesma forma de defeito que a `REQ-2026-09-01-gate-anti-divergencia` nomeia: *"o controle roda,
mede corretamente, e é completo sobre o que conhece — mas o que ele conhece está congelado"*.

### Por que reabre em vez de virar REQ nova

**Mesma causa, mesmo mecanismo, mesma ADR.** O AC2 original diz: *"todo sítio (a) corrigido, por
**ponto único** — não por `tr -d '\r'` espalhado"*, e o AC3 pede *"gate que impede a reintrodução"*.
Nenhum dos dois estava satisfeito para o literal. Fechar esta REQ com esse sítio vivo foi o achado
**A1** da auditoria externa de 2026-09-05 repetido — ADR de ponto único marcada satisfeita com sítio
sobrando — e este projeto já pagou por isso uma vez.

### População medida em 2026-09-27

| sítio | consome stdout como dado? | veredito |
|---|---|---|
| `scaffold.go:924` (`TOOL=$(…)`) | sim — captura em variável | **defeito** |
| `scaffold.go:925` (`MSG=$(…)`) | sim — captura em variável | **defeito** |
| `scaffold.go:2202` (`py_compile`) | não — não captura saída | fora |

## Negative scope — o que esta REQ NÃO faz

- **Não** recontar as falhas de Windows nem triar o cluster. Esta REQ **conserta o instrumento**; a
  contagem vem depois, com ele funcionando. Misturar as duas é medir com régua quebrada.
- **Não** trata os defeitos de Windows cujo mecanismo seja outro. #363, #307, #364 e #308 são
  **candidatos**, não membros: entram se a Wave 0 medir mesma causa, e saem com a medição escrita se
  não for.
- **Não** remove `python3` dos gates nem migra para Go. Mudar o interpretador é outra decisão, com
  outro custo, e não é necessária para fechar esta causa.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md`
