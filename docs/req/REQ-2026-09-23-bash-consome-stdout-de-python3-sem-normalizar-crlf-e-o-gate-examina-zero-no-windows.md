---
status: Open
date: 2026-09-23
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md"
---

# REQ: bash consome stdout de `python3` sem normalizar CRLF, e o gate examina zero no Windows

> Date: 2026-09-23 | Status: Open
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

- [ ] **Enumeração real** dos sítios em que bash consome saída de `python3` como dado, classificada
      em: **(a)** consome e **não** normaliza → defeito · **(b)** normaliza → correto · **(c)** invoca
      Python sem consumir saída → fora
- [ ] Todo sítio **(a)** corrigido, por **ponto único** — não por `tr -d '\r'` espalhado
- [ ] 🔴 **Gate que impede a reintrodução**, falsificável, e que **reprove** quando um consumo novo
      nascer sem normalização
- [ ] 🔴 **O censo de Windows volta a produzir número**: `windows-census.yml` com **8/8 shards** e
      apuração sem `TOTAL INCOMPLETO`
- [ ] Falsificação nas duas direções, **exercitada no Windows** — é a plataforma onde o defeito vive
- [ ] `make quality` e **CI** verdes

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
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-23-bash-consome-stdout-de-python3-sem-normalizar-crlf-e-o-gate-examina-zero-no-windows.md`
