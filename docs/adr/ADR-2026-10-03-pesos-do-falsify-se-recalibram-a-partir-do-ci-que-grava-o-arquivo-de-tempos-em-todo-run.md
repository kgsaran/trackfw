---
status: Accepted
date: 2026-10-03
author: "trackfw_architect"
---

# ADR: os pesos do falsify se recalibram a partir do CI, que grava o arquivo de tempos em todo run

> Date: 2026-10-03 | Status: Accepted

REQ: `docs/req/REQ-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md`
Origem: **#403**, com duas medições de consumidor externo nos comentários.

## Context

O `gen-falsify-chunks.py` distribui os cenários do `check-gates-falsify.sh` em chunks por peso de tempo,
lido de `scripts/falsify-scenario-weights.json`. Esse arquivo só se refaz com o
`gen-falsify-scenario-weights.py`, sobre um arquivo de marcas gravado quando `FALSIFY_TIMING_FILE` está
setada. **Nenhum job seta essa variável.** A calibração é manual, e a última é de 2026-09-08 (#295).

Medido em 2026-10-03:

| | |
|---|---|
| rótulos referenciados sem peso calibrado | **106** (de 204; o consumidor mediu 51%) |
| peso usado para eles | o **máximo** calibrado, 54,18 s, cerca de 105× a mediana |
| duração real dos 4 shards no CI (3 runs) | 57–113 s |
| `parity-other-gates`, que roda em paralelo | 150–200 s, e é o caminho crítico |

**Consequência:** no CI, o desequilíbrio não custa tempo de parede hoje. Ele pesa no `make quality`
local (8 chunks), onde a #504 já registrou chunk pendurado. E o número que o balanceador distribui é
94% fantasma (medição do consumidor), então nenhuma afirmação de tempo sai dele.

O consumidor também mediu que trocar o fallback pelo valor da **mediana** conserta o número, mas
**não** o desequilíbrio (2,30× → 1,76× em linhas). Só a recalibração ataca o desequilíbrio.

## Decision

- **D1 — O CI grava o tempo em todo run.** Cada job `parity-falsify-shard` seta
  `FALSIFY_TIMING_FILE` para um arquivo **dentro do diretório que já sobe como artefato**
  (`falsify-shard-out/`). A instrumentação já existe nos chunks gerados e só acrescenta um `printf`
  por bloco. Nada no veredito do shard muda.
- **D2 — Recalibrar é um comando.** `make falsify-recalibrate RUN=<id>` baixa os artefatos
  `falsify-shard-*` do run indicado com `gh run download`, concatena os arquivos de marcas, roda o
  `gen-falsify-scenario-weights.py` e reescreve `scripts/falsify-scenario-weights.json`. Ele reprova se
  faltar o arquivo de marcas de algum shard. O run tem de ser do repositório upstream; o alvo usa
  `--repo` explícito.
- **D3 — Recalibrar agora**, nesta REQ, a partir do run de CI do próprio PR.
- **D4 — A fração aparece.** O `gen-falsify-chunks.py` emite, além do aviso por rótulo, **uma linha de
  resumo**: `N de M rótulos sem peso calibrado (X%)`. **Não reprova** (decisão do KG em 2026-10-03:
  um teto reprovaria PR que só acrescenta cenário).

### D5 — Ajustes do threat model (Wave 0, `docs/seguranca/2026-10-03-wave0-recalibracao-pesos-falsify.md`)

- **T1/T2 — cobertos, medidos.** Com a variável setada, o log do shard fica idêntico (207 linhas, nenhuma
  `FALSIFY_TIMING`). A variável chega ao chunk, porque não há `env -i`.
- **AJ-T3:** o `FALSIFY_TIMING_FILE` do workflow aponta para dentro do `$OUTPUT_DIR`, que o
  `run-gates-falsify-shard.sh:96` cria antes do chunk. O caminho tem de ser exatamente esse diretório.
- **AJ-T4:** `--repo` não exclui fork, porque runs de PR de fork ficam sob o repositório base (medido:
  `cli/cli` run 37123041664, `head_repository=bodapatisaikrishna/cli`). O script exige
  `gh api repos/kgsaran/trackfw/actions/runs/<id> --jq .head_repository.full_name` == `kgsaran/trackfw`
  antes de baixar. Se divergir, sai com exit 1 e mensagem.
- **AJ-T5:** a cadeia `ts` forjado → todos os pesos 0 → tudo num chunk foi medida. O impacto é de
  disponibilidade; a cobertura não muda. Regras:
  - `gen-falsify-scenario-weights.py` rejeita duração negativa ou não finita, e eleva duração 0 a
    0,001 s, com aviso;
  - escreve o JSON de forma atômica (arquivo temporário + rename);
  - `load_weights` do `gen-falsify-chunks.py` rejeita, com erro nomeado, peso não finito, negativo ou
    não numérico. Isso fecha o resíduo do NaN, que contaminaria o fallback pessimista inteiro.
- **AJ-T6:** o `ts` tem validação explícita de formato antes do `float()`, e o erro nomeia o arquivo, a
  linha e o valor.
- **AJ-T7:** os `timing_<n>.log` não vazios para todo n em `0..FALSIFY_SHARD_COUNT-1` são exigidos, com
  o valor lido do `quality.yml`. Quando falta algum, o JSON existente não é tocado.
- **AJ-T8:** a linha de resumo do D4 vai para **stderr**. O stdout do gerador é o manifest que o
  `run-gates-falsify-shard.sh:87` consome.

## Consequences

- A recalibração deixa de depender de lembrar uma receita: o dado está em todo run de CI dos últimos 7
  dias (retenção do artefato).
- **Resíduo declarado:** os pesos ainda envelhecem entre recalibrações, porque ninguém é obrigado a
  rodar o alvo. A linha de resumo torna o envelhecimento visível; não o impede.
- **Resíduo declarado:** os pesos são medidos na VM do CI (Linux, 4 vCPU). No `make quality` local, o
  tempo absoluto é outro. O que importa para o balanceamento é o peso **relativo**, e isso não está
  medido entre máquinas.

## Alternatives Considered

- **Fallback pela mediana** (ou uma chave de dados irmã de `_fallback_weight_for_unlabeled`).
  Rejeitada como remédio principal pela medição do consumidor: conserta o número, não o equilíbrio. Fica
  fora de escopo.
- **Gate com teto de fração.** Rejeitado pelo KG (D4).
- **Fechar a issue sem mudança.** Rejeitado: o impacto local existe, e o número inauditável continua
  enganando quem ler o balanceador.

## Linked REQ

`docs/req/REQ-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md`
