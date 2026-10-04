# Recalibrar os pesos do falsify exige um run de PR, e blocos sem rótulo dividem um peso só

> Data: 2026-10-04 | REQ-2026-10-03 (#403), PR #517

## Como recalibrar

```bash
make falsify-recalibrate RUN=<id do run do quality.yml>
```

- O `quality.yml` roda **só em `pull_request` e em push na `main`**. Push numa branch não gera run. Para
  recalibrar a partir de uma branch, ela precisa de PR aberto, que pode ser rascunho.
- Os artefatos `falsify-shard-<n>` têm 7 dias de retenção. Run mais velho não recalibra.
- O script recusa run de PR de fork pelo `head_repository.full_name`. O `--repo` **não** basta: um run
  de fork fica registrado no repositório base (medido no `cli/cli`, run 37123041664).
- Ao terminar, o gerador imprime `N de M rotulos sem peso calibrado (X%)`. Se o N crescer, é hora de
  recalibrar.

## Resíduo: blocos sem rótulo

O peso é por **rótulo**. Bloco que não emite rótulo cai em `_fallback_weight_for_unlabeled`, **um valor
para todos**. Medido no run 37198827365:

| bloco (linha do `check-gates-falsify.sh`) | tempo real |
|---|---|
| 1814 | 69,11 s (o maior bloco; 25% dos 275 s totais) |
| 4977 | 2,17 s |
| 6925 | 1,95 s |

Os três recebem 69,11 s. Com N=8 (`make quality` local), cada bloco de 2 s ocupa um worker sozinho.
**Se o desequilíbrio local incomodar, a correção é dar rótulo a esses blocos**, e não mexer no fallback.

## Script novo em `scripts/` passa pelos gates de forma

O autoteste da recalibração reprovou duas vezes seguidas, por dois gates diferentes:

- `check-crlf-normalize-capture`: captura `$(... python3 ...)` sem `| strip_cr`
  (`. scripts/lib-crlf-normalize.sh`);
- `check-interpolated-path-in-python`: caminho dentro do texto do `python3 -c`. Passe por `sys.argv`;
- `check-output-encoding-declared`: `check-*.sh` que chama `python3` precisa de
  `export PYTHONIOENCODING=utf-8`.

Rode `make parity-rest` (cerca de 2 min, sem Go) antes de entregar script novo.
